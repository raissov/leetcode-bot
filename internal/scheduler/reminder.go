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
	"github.com/user/leetcode-bot/internal/storage"
)

// Reminder manages per-user cron-based daily reminders using robfig/cron/v3.
// It sends motivational messages at each user's configured hour and timezone.
type Reminder struct {
	cron    *cron.Cron
	bot     *telego.Bot
	store   *storage.DB
	lc      *leetcode.Client
	mu      sync.Mutex
	entries map[int64]cron.EntryID // telegramID → cron entry ID
}

// New creates a new Reminder scheduler with panic recovery enabled.
func New(bot *telego.Bot, store *storage.DB, lc *leetcode.Client) *Reminder {
	c := cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger)))
	return &Reminder{
		cron:    c,
		bot:     bot,
		store:   store,
		lc:      lc,
		entries: make(map[int64]cron.EntryID),
	}
}

// Start starts the cron scheduler.
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
// The reminder fires at the specified hour in the user's timezone.
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
	return nil
}

// UnscheduleUser removes the daily reminder for the given user.
func (r *Reminder) UnscheduleUser(telegramID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entryID, ok := r.entries[telegramID]; ok {
		r.cron.Remove(entryID)
		delete(r.entries, telegramID)
	}
}

// LoadAllReminders reads all users with reminders enabled from the database
// and schedules them. Call this on bot startup to restore reminders.
func (r *Reminder) LoadAllReminders() error {
	users, err := r.store.GetAllUsersWithReminders()
	if err != nil {
		return fmt.Errorf("load reminders from DB: %w", err)
	}

	for _, u := range users {
		if err := r.ScheduleUser(u.TelegramID, u.RemindHour, u.Timezone); err != nil {
			log.Printf("[scheduler] failed to schedule reminder for user %d: %v", u.TelegramID, err)
		}
	}

	log.Printf("[scheduler] loaded %d reminders from DB", len(users))
	return nil
}

// sendReminder checks if the user has already solved a problem today, and if
// not, sends a motivational reminder with their current streak and the daily
// challenge info.
func (r *Reminder) sendReminder(telegramID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	user, err := r.store.GetUserByTelegramID(telegramID)
	if err != nil || user == nil || user.LeetCodeUser == "" {
		log.Printf("[scheduler] skip reminder for user %d: not found or no LC account", telegramID)
		return
	}

	// Check if user already solved today via calendar API.
	year := time.Now().UTC().Year()
	cal, err := r.lc.GetUserCalendar(ctx, user.LeetCodeUser, year)
	if err != nil {
		log.Printf("[scheduler] failed to fetch calendar for user %d: %v", telegramID, err)
		// Continue with reminder anyway — better to remind than not.
	}

	if cal != nil {
		// Parse calendar and check today's submissions.
		profile, _ := r.lc.GetUserProfile(ctx, user.LeetCodeUser)
		if profile != nil && profile.MatchedUser != nil {
			calMap, _ := leetcode.ParseSubmissionCalendar(profile.MatchedUser.SubmissionCalendar)
			today := time.Now().UTC().Truncate(24 * time.Hour)
			if calMap[today] > 0 {
				// User already solved today — skip reminder.
				return
			}
		}
	}

	// Build reminder message.
	var msg string
	msg = "🔔 <b>Daily Reminder</b>\n\n"

	// Streak urgency.
	if user.CurrentStreak > 0 {
		msg += fmt.Sprintf("🔥 You have a <b>%d-day streak</b>! Don't break it!\n\n", user.CurrentStreak)
	} else {
		msg += "Start a new streak today! Every journey begins with a single step.\n\n"
	}

	// Daily challenge info.
	daily, err := r.lc.GetDailyChallenge(ctx)
	if err == nil && daily != nil {
		link := daily.Link
		if len(link) > 0 && link[0] == '/' {
			link = "https://leetcode.com" + link
		}
		msg += fmt.Sprintf("📅 <b>Today's Challenge:</b> %s\n", daily.Question.Title)
		msg += fmt.Sprintf("Difficulty: <b>%s</b>\n", daily.Question.Difficulty)
		msg += fmt.Sprintf("🔗 <a href=\"%s\">Solve it now!</a>\n\n", link)
	}

	// Motivational quote.
	msg += gamification.DailyMotivation()

	_, err = r.bot.SendMessage(ctx, tu.Message(
		tu.ID(telegramID),
		msg,
	).WithParseMode(telego.ModeHTML))
	if err != nil {
		log.Printf("[scheduler] failed to send reminder to user %d: %v", telegramID, err)
	}
}
