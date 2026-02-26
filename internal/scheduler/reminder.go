package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
	"github.com/robfig/cron/v3"

	"github.com/user/leetcode-bot/internal/gamification"
	"github.com/user/leetcode-bot/internal/leetcode"
	"github.com/user/leetcode-bot/internal/stats"
	"github.com/user/leetcode-bot/internal/storage"
)

// Reminder manages per-user cron-based daily reminders using robfig/cron/v3.
// It sends motivational messages at each user's configured hour and timezone
// if they haven't already solved a problem that day.
type Reminder struct {
	cron    *cron.Cron
	bot     *telego.Bot
	store   *storage.DB
	lc      *leetcode.Client
	stats   *stats.Collector
	mu      sync.Mutex
	entries map[int64]cron.EntryID // telegramID → cron entry ID
}

// New creates a new Reminder scheduler with panic recovery enabled.
// The scheduler is not started until Start() is called.
func New(
	bot *telego.Bot,
	store *storage.DB,
	lc *leetcode.Client,
	statsCollector *stats.Collector,
) *Reminder {
	c := cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger)))
	return &Reminder{
		cron:    c,
		bot:     bot,
		store:   store,
		lc:      lc,
		stats:   statsCollector,
		entries: make(map[int64]cron.EntryID),
	}
}

// Start starts the cron scheduler. Scheduled jobs will fire after this call.
func (r *Reminder) Start() {
	r.cron.Start()
	log.Println("[scheduler] reminder scheduler started")
}

// Stop stops the cron scheduler gracefully, waiting for running jobs to finish.
func (r *Reminder) Stop() {
	ctx := r.cron.Stop()
	<-ctx.Done()
	log.Println("[scheduler] reminder scheduler stopped")
}

// ScheduleUser adds or replaces a daily reminder cron entry for the given user.
// The reminder fires at the specified hour in the user's timezone. If the user
// already has a scheduled entry, it is removed first to avoid duplicates.
func (r *Reminder) ScheduleUser(telegramID int64, hour int, timezone string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove existing entry if any.
	if entryID, ok := r.entries[telegramID]; ok {
		r.cron.Remove(entryID)
		delete(r.entries, telegramID)
	}

	// Build cron spec with per-user timezone: "CRON_TZ=<tz> 0 <hour> * * *"
	spec := fmt.Sprintf("CRON_TZ=%s 0 %d * * *", timezone, hour)

	tid := telegramID // capture for closure
	entryID, err := r.cron.AddFunc(spec, func() {
		r.sendReminder(tid)
	})
	if err != nil {
		return fmt.Errorf("schedule reminder for user %d: %w", telegramID, err)
	}

	r.entries[telegramID] = entryID
	log.Printf("[scheduler] scheduled reminder for user %d at %02d:00 %s", telegramID, hour, timezone)
	return nil
}

// UnscheduleUser removes the daily reminder for the given user. It is a no-op
// if the user has no scheduled entry.
func (r *Reminder) UnscheduleUser(telegramID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entryID, ok := r.entries[telegramID]; ok {
		r.cron.Remove(entryID)
		delete(r.entries, telegramID)
		log.Printf("[scheduler] unscheduled reminder for user %d", telegramID)
	}
}

// LoadAllReminders reads all users with reminders enabled from the database
// and schedules a cron entry for each. Call this on bot startup to restore
// reminder state after a restart.
func (r *Reminder) LoadAllReminders() error {
	users, err := r.store.GetAllUsersWithReminders()
	if err != nil {
		return fmt.Errorf("load reminders from DB: %w", err)
	}

	loaded := 0
	for _, u := range users {
		if err := r.ScheduleUser(u.TelegramID, u.RemindHour, u.Timezone); err != nil {
			log.Printf("[scheduler] failed to schedule reminder for user %d: %v", u.TelegramID, err)
			continue
		}
		loaded++
	}

	log.Printf("[scheduler] loaded %d/%d reminders from database", loaded, len(users))
	return nil
}

// sendReminder checks if the user has already solved a problem today via the
// calendar API, and if not, sends a motivational reminder with their current
// streak urgency and the daily challenge info.
func (r *Reminder) sendReminder(telegramID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Look up the user to get their LeetCode username and streak info.
	user, err := r.store.GetUserByTelegramID(telegramID)
	if err != nil || user == nil {
		log.Printf("[scheduler] skip reminder for user %d: not found: %v", telegramID, err)
		return
	}

	if user.LeetCodeUser == "" {
		log.Printf("[scheduler] skip reminder for user %d: no linked LeetCode account", telegramID)
		return
	}

	// Check if user already solved today via the calendar API.
	if r.hasSolvedToday(ctx, user.LeetCodeUser) {
		log.Printf("[scheduler] skip reminder for user %d: already solved today", telegramID)
		return
	}

	// Build and send the reminder message.
	text := r.buildReminderMessage(ctx, user)

	_, err = r.bot.SendMessage(ctx, tu.Message(
		tu.ID(telegramID),
		text,
	).WithParseMode(telego.ModeHTML))
	if err != nil {
		log.Printf("[scheduler] failed to send reminder to user %d: %v", telegramID, err)
	}
}

// hasSolvedToday checks whether the user has made at least one submission
// today (UTC) by fetching their calendar from the LeetCode API.
func (r *Reminder) hasSolvedToday(ctx context.Context, username string) bool {
	year := time.Now().UTC().Year()
	cal, err := r.lc.GetUserCalendar(ctx, username, year)
	if err != nil {
		log.Printf("[scheduler] failed to fetch calendar for %q: %v", username, err)
		return false // err on the side of sending the reminder
	}

	if cal == nil {
		return false
	}

	calMap, err := leetcode.ParseSubmissionCalendar(cal.SubmissionCalendar)
	if err != nil {
		log.Printf("[scheduler] failed to parse calendar for %q: %v", username, err)
		return false
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	return calMap[today] > 0
}

// buildReminderMessage constructs a motivational reminder including streak
// urgency, daily challenge info, and a motivational quote.
func (r *Reminder) buildReminderMessage(ctx context.Context, user *storage.User) string {
	msg := "\U0001F514 <b>Daily LeetCode Reminder</b>\n\n"

	// Streak urgency section.
	if user.CurrentStreak > 0 {
		msg += gamification.StreakMessage(user.CurrentStreak) + "\n"
		msg += fmt.Sprintf(
			"\u26A0\uFE0F <b>Don't break your %d-day streak!</b> Solve a problem today!\n\n",
			user.CurrentStreak,
		)
	} else {
		msg += "\U0001F31F Start a new streak today! Every journey begins with a single step.\n\n"
	}

	// Daily challenge info.
	daily, err := r.lc.GetDailyChallenge(ctx)
	if err == nil && daily != nil {
		link := daily.Link
		if len(link) > 0 && link[0] == '/' {
			link = "https://leetcode.com" + link
		}
		msg += fmt.Sprintf(
			"\U0001F4C5 <b>Today's Challenge:</b>\n"+
				"<b>%s</b> (%s)\n"+
				"\U0001F517 <a href=\"%s\">Solve it now!</a>\n\n",
			daily.Question.Title, daily.Question.Difficulty, link,
		)
	}

	// Motivational quote.
	msg += gamification.DailyMotivation()

	return msg
}
