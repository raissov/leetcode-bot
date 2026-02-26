package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/user/leetcode-bot/internal/gamification"
	"github.com/user/leetcode-bot/internal/stats"
)

// handleStart welcomes the user and ensures they are registered in the database.
// The EnsureUser middleware already creates the user record, so this handler
// just sends a friendly welcome message with instructions.
func (b *Bot) handleStart(ctx *th.Context, message telego.Message) error {
	name := ""
	if message.From != nil {
		name = message.From.FirstName
	}

	greeting := "there"
	if name != "" {
		greeting = name
	}

	text := fmt.Sprintf(
		"\U0001F44B <b>Welcome, %s!</b>\n\n"+
			"I'm your personal LeetCode companion. I'll help you track your progress, "+
			"maintain streaks, and stay motivated!\n\n"+
			"<b>Get started:</b>\n"+
			"1. Link your account: <code>/connect &lt;username&gt;</code>\n"+
			"2. Check your stats: /stats\n"+
			"3. See today's challenge: /daily\n\n"+
			"Use /help to see all available commands.",
		greeting,
	)

	_, err := b.api.SendMessage(ctx, tu.Message(
		tu.ID(message.Chat.ID),
		text,
	).WithParseMode(telego.ModeHTML))
	return err
}

// handleHelp lists all available bot commands with brief descriptions.
func (b *Bot) handleHelp(ctx *th.Context, message telego.Message) error {
	text := "\U0001F4D6 <b>Available Commands</b>\n\n" +
		"/start \u2014 Welcome message\n" +
		"/connect &lt;username&gt; \u2014 Link LeetCode account\n" +
		"/stats \u2014 Show your statistics\n" +
		"/streak \u2014 Show your streak\n" +
		"/daily \u2014 Today's daily challenge\n" +
		"/achievements \u2014 Your achievements\n" +
		"/level \u2014 Your level &amp; points\n" +
		"/remind on|off \u2014 Toggle reminders\n" +
		"/remind &lt;0-23&gt; \u2014 Set reminder hour\n" +
		"/help \u2014 Show this message"

	_, err := b.api.SendMessage(ctx, tu.Message(
		tu.ID(message.Chat.ID),
		text,
	).WithParseMode(telego.ModeHTML))
	return err
}

// handleConnect parses the LeetCode username from the message text, validates
// it against the LeetCode GraphQL API, and saves it to the database if valid.
func (b *Bot) handleConnect(ctx *th.Context, message telego.Message) error {
	chatID := message.Chat.ID
	telegramID := int64(0)
	if message.From != nil {
		telegramID = message.From.ID
	}

	// Parse username from message: "/connect <username>"
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		_, err := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u26A0\uFE0F <b>Usage:</b> <code>/connect &lt;username&gt;</code>\n\n"+
				"Example: <code>/connect neal_wu</code>",
		).WithParseMode(telego.ModeHTML))
		return err
	}
	username := parts[1]

	// Send a "working" message.
	_, _ = b.api.SendMessage(ctx, tu.Message(
		tu.ID(chatID),
		"\u23F3 Validating LeetCode username...",
	))

	// Validate username via LeetCode API.
	profile, err := b.lc.GetUserProfile(ctx, username)
	if err != nil {
		log.Printf("[bot] connect: failed to fetch profile for %q: %v", username, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C LeetCode is temporarily unavailable. Please try again in a few minutes.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	if profile == nil || profile.MatchedUser == nil {
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Username not found on LeetCode. Please check and try again.\n\n"+
				"If your profile is private, please make it public to use stats tracking.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	// Save the username to the database.
	if err := b.store.UpdateLeetCodeUser(telegramID, username); err != nil {
		log.Printf("[bot] connect: failed to save username for user %d: %v", telegramID, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to save your username. Please try again.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	_, sendErr := b.api.SendMessage(ctx, tu.Message(
		tu.ID(chatID),
		fmt.Sprintf(
			"\u2705 <b>Connected!</b>\n\n"+
				"Your LeetCode account <b>%s</b> has been linked successfully.\n\n"+
				"Try /stats to see your statistics or /daily for today's challenge!",
			username,
		),
	).WithParseMode(telego.ModeHTML))
	return sendErr
}

// handleStats routes to the appropriate stats subcommand handler based on the message text.
// Supports: /stats (overview), /stats topics, /stats weekly
func (b *Bot) handleStats(ctx *th.Context, message telego.Message) error {
	// Parse subcommand from message: "/stats [subcommand]"
	parts := strings.Fields(message.Text)

	// Default to overview if no subcommand specified
	if len(parts) == 1 {
		return b.handleStatsOverview(ctx, message)
	}

	// Route to appropriate handler based on subcommand
	subcommand := parts[1]
	switch subcommand {
	case "topics":
		return b.handleStatsTopics(ctx, message)
	case "weekly":
		return b.handleStatsWeekly(ctx, message)
	default:
		// Unknown subcommand - show usage
		chatID := message.Chat.ID
		_, err := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u26A0\uFE0F <b>Unknown subcommand.</b>\n\n"+
				"<b>Available options:</b>\n"+
				"/stats \u2014 Show overview\n"+
				"/stats topics \u2014 Topic coverage breakdown\n"+
				"/stats weekly \u2014 Past 7 days breakdown",
		).WithParseMode(telego.ModeHTML))
		return err
	}
}

// handleStatsOverview fetches the user's LeetCode statistics, saves a snapshot,
// checks for new achievements, and sends a formatted response.
func (b *Bot) handleStatsOverview(ctx *th.Context, message telego.Message) error {
	chatID := message.Chat.ID
	telegramID := int64(0)
	if message.From != nil {
		telegramID = message.From.ID
	}

	// Fetch and save stats.
	userStats, err := b.stats.GetUserStats(ctx, telegramID)
	if err != nil {
		log.Printf("[bot] stats: failed to get stats for user %d: %v", telegramID, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to fetch your statistics. LeetCode may be temporarily unavailable. "+
				"Please try again in a few minutes.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	// Check and unlock any new achievements.
	newAchievements := b.checkAndUnlockAchievements(ctx, telegramID, userStats)

	// Update gamification state.
	b.updateGamification(ctx, telegramID, userStats)

	// Format and send the stats message.
	text := stats.FormatStats(userStats, nil)
	_, err = b.api.SendMessage(ctx, tu.Message(
		tu.ID(chatID),
		text,
	).WithParseMode(telego.ModeHTML))
	if err != nil {
		return err
	}

	// Send achievement unlock notifications.
	b.sendAchievementNotifications(ctx, chatID, newAchievements)

	return nil
}

// handleStatsTopics shows topic coverage breakdown for the user.
// This handler will be implemented in the next subtask.
func (b *Bot) handleStatsTopics(ctx *th.Context, message telego.Message) error {
	chatID := message.Chat.ID
	telegramID := int64(0)
	if message.From != nil {
		telegramID = message.From.ID
	}

	// Get the user from the database to retrieve the internal user ID.
	user, err := b.store.GetUserByTelegramID(telegramID)
	if err != nil {
		log.Printf("[bot] stats topics: failed to get user %d: %v", telegramID, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to retrieve your user information. Please try again.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}
	if user == nil {
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C User not found. Please use /start to register.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	// Compute topic coverage for this user.
	topicStats, err := b.stats.ComputeTopicCoverage(ctx, user.ID)
	if err != nil {
		log.Printf("[bot] stats topics: failed to compute topic coverage for user %d: %v", user.ID, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to compute topic coverage. Please try again.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	// Format and send the topic coverage message.
	text := stats.FormatTopicCoverage(topicStats)
	_, err = b.api.SendMessage(ctx, tu.Message(
		tu.ID(chatID),
		text,
	).WithParseMode(telego.ModeHTML))
	return err
}

// handleStatsWeekly shows the user's past 7 days breakdown.
func (b *Bot) handleStatsWeekly(ctx *th.Context, message telego.Message) error {
	chatID := message.Chat.ID
	telegramID := int64(0)
	if message.From != nil {
		telegramID = message.From.ID
	}

	// Get the user from the database to retrieve the internal user ID.
	user, err := b.store.GetUserByTelegramID(telegramID)
	if err != nil {
		log.Printf("[bot] stats weekly: failed to get user %d: %v", telegramID, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to retrieve your user information. Please try again.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}
	if user == nil {
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C User not found. Please use /start to register.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	// Compute weekly statistics for this user.
	weeklyStats, err := b.stats.ComputeWeeklyStats(ctx, user.ID)
	if err != nil {
		log.Printf("[bot] stats weekly: failed to compute weekly stats for user %d: %v", user.ID, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to compute weekly statistics. Please try again.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	// Convert DailyBreakdown to map[time.Time]int for formatting.
	dailyActivity := make(map[time.Time]int, len(weeklyStats.DailyBreakdown))
	for _, day := range weeklyStats.DailyBreakdown {
		dailyActivity[day.Date.Truncate(24*time.Hour)] = day.Solved
	}

	// Calculate week start and end dates.
	now := time.Now().UTC()
	weekEnd := now.Truncate(24 * time.Hour)
	weekStart := weekEnd.AddDate(0, 0, -6)

	// Format and send the weekly stats message.
	text := stats.FormatWeeklyStats(weekStart, weekEnd, dailyActivity, weeklyStats.TotalThisWeek)
	_, err = b.api.SendMessage(ctx, tu.Message(
		tu.ID(chatID),
		text,
	).WithParseMode(telego.ModeHTML))
	return err
}

// handleStreak fetches the user's submission calendar, computes the current
// and best streak, updates gamification state, and sends a formatted response.
func (b *Bot) handleStreak(ctx *th.Context, message telego.Message) error {
	chatID := message.Chat.ID
	telegramID := int64(0)
	if message.From != nil {
		telegramID = message.From.ID
	}

	// Fetch stats (which includes streak computation).
	userStats, err := b.stats.GetUserStats(ctx, telegramID)
	if err != nil {
		log.Printf("[bot] streak: failed to get stats for user %d: %v", telegramID, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to fetch your streak data. Please try again in a few minutes.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	// Update gamification with streak info.
	b.updateGamification(ctx, telegramID, userStats)

	// Format and send the streak message.
	text := stats.FormatStreak(userStats.CurrentStreak, userStats.BestStreak, userStats.SubmissionCalendar)
	_, err = b.api.SendMessage(ctx, tu.Message(
		tu.ID(chatID),
		text,
	).WithParseMode(telego.ModeHTML))
	return err
}

// handleDaily fetches today's daily coding challenge and sends a formatted
// response with the title, difficulty, topic tags, and a link.
func (b *Bot) handleDaily(ctx *th.Context, message telego.Message) error {
	chatID := message.Chat.ID

	daily, err := b.lc.GetDailyChallenge(ctx)
	if err != nil {
		log.Printf("[bot] daily: failed to fetch daily challenge: %v", err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to fetch today's daily challenge. Please try again in a few minutes.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	text := stats.FormatDaily(*daily)
	_, err = b.api.SendMessage(ctx, tu.Message(
		tu.ID(chatID),
		text,
	).WithParseMode(telego.ModeHTML))
	return err
}

// handleAchievements retrieves the user's unlocked achievements and sends a
// formatted list showing locked/unlocked status for each achievement.
func (b *Bot) handleAchievements(ctx *th.Context, message telego.Message) error {
	chatID := message.Chat.ID
	telegramID := int64(0)
	if message.From != nil {
		telegramID = message.From.ID
	}

	// Get the user's DB record to find the internal user ID.
	user, err := b.store.GetUserByTelegramID(telegramID)
	if err != nil || user == nil {
		log.Printf("[bot] achievements: failed to get user %d: %v", telegramID, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to load your achievements. Please try again.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	// Get unlocked achievements from DB.
	dbAchievements, err := b.store.GetUserAchievements(user.ID)
	if err != nil {
		log.Printf("[bot] achievements: failed to get achievements for user %d: %v", telegramID, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to load your achievements. Please try again.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	// Build the unlocked map.
	unlocked := make(map[string]bool, len(dbAchievements))
	for _, a := range dbAchievements {
		unlocked[a.AchievementKey] = true
	}

	// Format and send.
	all := gamification.AllAchievements()
	text := stats.FormatAchievements(unlocked, all)
	_, err = b.api.SendMessage(ctx, tu.Message(
		tu.ID(chatID),
		text,
	).WithParseMode(telego.ModeHTML))
	return err
}

// handleLevel retrieves the user's gamification data (points, level, progress)
// and sends a formatted response with a progress bar.
func (b *Bot) handleLevel(ctx *th.Context, message telego.Message) error {
	chatID := message.Chat.ID
	telegramID := int64(0)
	if message.From != nil {
		telegramID = message.From.ID
	}

	// Get the user's current gamification state from the DB.
	user, err := b.store.GetUserByTelegramID(telegramID)
	if err != nil || user == nil {
		log.Printf("[bot] level: failed to get user %d: %v", telegramID, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to load your level data. Please try again.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	// Recalculate level from current points to ensure consistency.
	level, title := gamification.DetermineLevel(user.Points)
	pointsToNext := gamification.PointsToNextLevel(user.Points)

	// Format and send.
	text := stats.FormatLevel(level, title, user.Points, pointsToNext)
	_, err = b.api.SendMessage(ctx, tu.Message(
		tu.ID(chatID),
		text,
	).WithParseMode(telego.ModeHTML))
	return err
}

// handleRemind parses the argument (on/off/<hour>) from the message text,
// updates the user's reminder settings in the database, and reschedules the
// cron job accordingly.
func (b *Bot) handleRemind(ctx *th.Context, message telego.Message) error {
	chatID := message.Chat.ID
	telegramID := int64(0)
	if message.From != nil {
		telegramID = message.From.ID
	}

	// Parse argument from message: "/remind <arg>"
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		_, err := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u2139\uFE0F <b>Reminder Settings</b>\n\n"+
				"<b>Usage:</b>\n"+
				"<code>/remind on</code> \u2014 Enable daily reminders\n"+
				"<code>/remind off</code> \u2014 Disable daily reminders\n"+
				"<code>/remind &lt;0-23&gt;</code> \u2014 Set reminder hour\n\n"+
				"Example: <code>/remind 9</code> (reminder at 9:00 AM UTC)",
		).WithParseMode(telego.ModeHTML))
		return err
	}
	arg := strings.ToLower(parts[1])

	// Get current user settings.
	user, err := b.store.GetUserByTelegramID(telegramID)
	if err != nil || user == nil {
		log.Printf("[bot] remind: failed to get user %d: %v", telegramID, err)
		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\u274C Failed to load your settings. Please try again.",
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}

	switch arg {
	case "on":
		// Enable reminders with current hour/timezone.
		if err := b.store.UpdateReminder(telegramID, true, user.RemindHour, user.Timezone); err != nil {
			log.Printf("[bot] remind: failed to enable reminder for user %d: %v", telegramID, err)
			_, sendErr := b.api.SendMessage(ctx, tu.Message(
				tu.ID(chatID),
				"\u274C Failed to enable reminders. Please try again.",
			).WithParseMode(telego.ModeHTML))
			return sendErr
		}

		// Schedule the cron job.
		if b.scheduler != nil {
			if err := b.scheduler.ScheduleUser(telegramID, user.RemindHour, user.Timezone); err != nil {
				log.Printf("[bot] remind: failed to schedule reminder for user %d: %v", telegramID, err)
			}
		}

		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			fmt.Sprintf(
				"\u2705 <b>Reminders enabled!</b>\n\nYou'll receive a daily reminder at <b>%02d:00 %s</b>.",
				user.RemindHour, user.Timezone,
			),
		).WithParseMode(telego.ModeHTML))
		return sendErr

	case "off":
		// Disable reminders.
		if err := b.store.UpdateReminder(telegramID, false, user.RemindHour, user.Timezone); err != nil {
			log.Printf("[bot] remind: failed to disable reminder for user %d: %v", telegramID, err)
			_, sendErr := b.api.SendMessage(ctx, tu.Message(
				tu.ID(chatID),
				"\u274C Failed to disable reminders. Please try again.",
			).WithParseMode(telego.ModeHTML))
			return sendErr
		}

		// Remove the cron job.
		if b.scheduler != nil {
			b.scheduler.UnscheduleUser(telegramID)
		}

		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			"\U0001F515 <b>Reminders disabled.</b>\n\nUse <code>/remind on</code> to re-enable.",
		).WithParseMode(telego.ModeHTML))
		return sendErr

	default:
		// Try to parse as an hour (0-23).
		hour, parseErr := strconv.Atoi(arg)
		if parseErr != nil || hour < 0 || hour > 23 {
			_, sendErr := b.api.SendMessage(ctx, tu.Message(
				tu.ID(chatID),
				"\u26A0\uFE0F Invalid argument. Use <code>on</code>, <code>off</code>, or an hour <code>0-23</code>.",
			).WithParseMode(telego.ModeHTML))
			return sendErr
		}

		// Update the reminder hour and enable reminders.
		if err := b.store.UpdateReminder(telegramID, true, hour, user.Timezone); err != nil {
			log.Printf("[bot] remind: failed to set hour for user %d: %v", telegramID, err)
			_, sendErr := b.api.SendMessage(ctx, tu.Message(
				tu.ID(chatID),
				"\u274C Failed to update reminder hour. Please try again.",
			).WithParseMode(telego.ModeHTML))
			return sendErr
		}

		// Reschedule the cron job with the new hour.
		if b.scheduler != nil {
			if err := b.scheduler.ScheduleUser(telegramID, hour, user.Timezone); err != nil {
				log.Printf("[bot] remind: failed to reschedule reminder for user %d: %v", telegramID, err)
			}
		}

		_, sendErr := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			fmt.Sprintf(
				"\u2705 <b>Reminder updated!</b>\n\nYou'll receive a daily reminder at <b>%02d:00 %s</b>.",
				hour, user.Timezone,
			),
		).WithParseMode(telego.ModeHTML))
		return sendErr
	}
}

// --- Internal helper methods ---

// checkAndUnlockAchievements evaluates the user's stats against all achievement
// definitions and unlocks any newly eligible achievements. Returns the list of
// newly unlocked achievement keys.
func (b *Bot) checkAndUnlockAchievements(ctx *th.Context, telegramID int64, userStats *stats.UserStats) []string {
	if userStats == nil {
		return nil
	}

	// Get the user's internal ID.
	user, err := b.store.GetUserByTelegramID(telegramID)
	if err != nil || user == nil {
		log.Printf("[bot] check_achievements: failed to get user %d: %v", telegramID, err)
		return nil
	}

	// Get already unlocked achievements.
	dbAchievements, err := b.store.GetUserAchievements(user.ID)
	if err != nil {
		log.Printf("[bot] check_achievements: failed to get achievements for user %d: %v", telegramID, err)
		return nil
	}

	alreadyUnlocked := make(map[string]bool, len(dbAchievements))
	for _, a := range dbAchievements {
		alreadyUnlocked[a.AchievementKey] = true
	}

	// Build gamification.UserStats for achievement checking.
	gStats := gamification.UserStats{
		TotalSolved:   userStats.TotalSolved,
		EasySolved:    userStats.EasySolved,
		MediumSolved:  userStats.MediumSolved,
		HardSolved:    userStats.HardSolved,
		CurrentStreak: userStats.CurrentStreak,
		BestStreak:    userStats.BestStreak,
		TotalEasy:     userStats.TotalEasy,
		SolvedDaily:   hasSolvedToday(userStats.SubmissionCalendar),
	}

	// Check eligibility for new achievements.
	newlyEligible := gamification.CheckEligibility(gStats, alreadyUnlocked)

	// Unlock each newly eligible achievement.
	var unlocked []string
	for _, key := range newlyEligible {
		newlyUnlocked, err := b.store.UnlockAchievement(user.ID, key)
		if err != nil {
			log.Printf("[bot] check_achievements: failed to unlock %q for user %d: %v", key, telegramID, err)
			continue
		}
		if newlyUnlocked {
			unlocked = append(unlocked, key)
		}
	}

	return unlocked
}

// updateGamification recalculates the user's points and level from current
// stats and persists the updated state. Also handles level-up notifications.
func (b *Bot) updateGamification(ctx *th.Context, telegramID int64, userStats *stats.UserStats) {
	if userStats == nil {
		return
	}

	solveStats := gamification.SolveStats{
		EasySolved:   userStats.EasySolved,
		MediumSolved: userStats.MediumSolved,
		HardSolved:   userStats.HardSolved,
		StreakDays:   userStats.CurrentStreak,
	}

	prevPoints, newPoints, err := b.gamify.UpdateUserStats(ctx, telegramID, solveStats)
	if err != nil {
		log.Printf("[bot] gamification: failed to update user %d: %v", telegramID, err)
		return
	}

	// Check for level-up.
	prevLevel, _ := gamification.DetermineLevel(prevPoints)
	newLevel, newTitle := gamification.DetermineLevel(newPoints)

	if newLevel > prevLevel {
		// Send level-up notification.
		user, err := b.store.GetUserByTelegramID(telegramID)
		if err != nil || user == nil {
			return
		}

		text := gamification.LevelUpMessage(newLevel, newTitle)
		_, _ = b.api.SendMessage(ctx, tu.Message(
			tu.ID(user.TelegramID),
			text,
		).WithParseMode(telego.ModeHTML))
	}
}

// sendAchievementNotifications sends a celebratory message for each newly
// unlocked achievement.
func (b *Bot) sendAchievementNotifications(ctx *th.Context, chatID int64, achievementKeys []string) {
	for _, key := range achievementKeys {
		achievement := gamification.AchievementByKey(key)
		if achievement == nil {
			continue
		}

		text := gamification.AchievementUnlockedMessage(*achievement)
		_, err := b.api.SendMessage(ctx, tu.Message(
			tu.ID(chatID),
			text,
		).WithParseMode(telego.ModeHTML))
		if err != nil {
			log.Printf("[bot] achievement notification: failed to send for key %q: %v", key, err)
		}
	}
}

// hasSolvedToday checks whether the submission calendar contains an entry for
// today (UTC).
func hasSolvedToday(calendar map[time.Time]int) bool {
	if calendar == nil {
		return false
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	return calendar[today] > 0
}
