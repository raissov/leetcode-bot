package stats

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/user/leetcode-bot/internal/leetcode"
	"github.com/user/leetcode-bot/internal/storage"
)

// UserStats holds the combined statistics for a user, aggregated from the
// LeetCode API and stored snapshots. It is returned by GetUserStats and
// FetchAndSaveStats for use by formatters and handlers.
type UserStats struct {
	// Current totals from LeetCode.
	TotalSolved    int
	EasySolved     int
	MediumSolved   int
	HardSolved     int
	AcceptanceRate float64
	Ranking        int

	// Total question counts by difficulty (for progress bars).
	TotalEasy   int
	TotalMedium int
	TotalHard   int

	// Streak info computed from the submission calendar.
	CurrentStreak int
	BestStreak    int

	// Delta since the previous snapshot (nil if no previous snapshot exists).
	Delta *StatsDelta

	// Calendar data for display.
	SubmissionCalendar map[time.Time]int
	TotalActiveDays    int

	// Recent submissions from LeetCode.
	RecentSubmissions []leetcode.RecentSubmission
}

// StatsDelta represents the change in stats between two snapshots.
type StatsDelta struct {
	TotalSolved    int
	EasySolved     int
	MediumSolved   int
	HardSolved     int
	AcceptanceRate float64
	Ranking        int
}

// Collector orchestrates fetching statistics from the LeetCode API, saving
// snapshots to the database, and computing derived stats like streaks and
// deltas. It depends on a leetcode.Client for API access and a storage.DB
// for persistence.
type Collector struct {
	lc    *leetcode.Client
	store *storage.DB
}

// NewCollector creates a new stats Collector with the given dependencies.
func NewCollector(lc *leetcode.Client, store *storage.DB) *Collector {
	return &Collector{
		lc:    lc,
		store: store,
	}
}

// FetchAndSaveStats fetches the user's LeetCode profile and calendar, saves a
// daily stats snapshot to the database, and returns the computed UserStats.
// The telegramID is used to look up the internal user ID for DB operations.
func (c *Collector) FetchAndSaveStats(ctx context.Context, telegramID int64, username string) (*UserStats, error) {
	// Fetch profile and calendar concurrently would be nice, but the
	// leetcode.Client rate limiter serializes requests anyway. Fetch
	// sequentially for simplicity.
	profile, err := c.lc.GetUserProfile(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("fetch profile: %w", err)
	}
	if profile == nil || profile.MatchedUser == nil {
		return nil, fmt.Errorf("leetcode profile not found or private for %q", username)
	}

	year := time.Now().UTC().Year()
	calendar, err := c.lc.GetUserCalendar(ctx, username, year)
	if err != nil {
		return nil, fmt.Errorf("fetch calendar: %w", err)
	}

	submissions, err := c.lc.GetRecentSubmissions(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("fetch recent submissions: %w", err)
	}

	// Extract stats from the profile response.
	matched := profile.MatchedUser
	easySolved, mediumSolved, hardSolved, totalSolved := extractSolvedCounts(matched.SubmitStats)
	acceptanceRate := computeAcceptanceRate(matched.SubmitStats)
	ranking := matched.Profile.Ranking

	// Extract total question counts (for progress display).
	totalEasy, totalMedium, totalHard := extractTotalQuestions(profile.AllQuestionsCount)

	// Parse submission calendar for streak computation.
	calendarMap, err := leetcode.ParseSubmissionCalendar(matched.SubmissionCalendar)
	if err != nil {
		return nil, fmt.Errorf("parse submission calendar: %w", err)
	}

	// Compute streak from the parsed calendar.
	currentStreak := ComputeStreak(calendarMap)

	// Look up the internal user ID from the telegram ID.
	user, err := c.store.GetUserByTelegramID(telegramID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user with telegram_id %d not found", telegramID)
	}

	// Sync the user's complete solved problem history to the database.
	if err := c.SyncUserSolvedProblems(ctx, user.ID, username); err != nil {
		return nil, fmt.Errorf("sync solved problems: %w", err)
	}

	// Save today's snapshot.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if err := c.store.SaveSnapshot(
		user.ID,
		totalSolved, easySolved, mediumSolved, hardSolved,
		acceptanceRate, ranking, today,
	); err != nil {
		return nil, fmt.Errorf("save snapshot: %w", err)
	}

	// Compute delta against previous snapshot.
	var delta *StatsDelta
	prev, err := c.store.GetPreviousSnapshot(user.ID)
	if err != nil {
		return nil, fmt.Errorf("get previous snapshot: %w", err)
	}
	if prev != nil {
		current := &storage.StatsSnapshot{
			TotalSolved:    totalSolved,
			EasySolved:     easySolved,
			MediumSolved:   mediumSolved,
			HardSolved:     hardSolved,
			AcceptanceRate: acceptanceRate,
			Ranking:        ranking,
		}
		delta = ComputeDelta(current, prev)
	}

	// Determine total active days from calendar or API.
	totalActiveDays := 0
	if calendar != nil {
		totalActiveDays = calendar.TotalActiveDays
	}

	// Best streak: max of stored best and current.
	bestStreak := user.BestStreak
	if currentStreak > bestStreak {
		bestStreak = currentStreak
	}

	return &UserStats{
		TotalSolved:        totalSolved,
		EasySolved:         easySolved,
		MediumSolved:       mediumSolved,
		HardSolved:         hardSolved,
		AcceptanceRate:     acceptanceRate,
		Ranking:            ranking,
		TotalEasy:          totalEasy,
		TotalMedium:        totalMedium,
		TotalHard:          totalHard,
		CurrentStreak:      currentStreak,
		BestStreak:         bestStreak,
		Delta:              delta,
		SubmissionCalendar: calendarMap,
		TotalActiveDays:    totalActiveDays,
		RecentSubmissions:  submissions,
	}, nil
}

// GetUserStats returns combined stats from the DB (latest snapshot) and
// live LeetCode data. This is a convenience method that calls
// FetchAndSaveStats under the hood, ensuring the DB is always up to date.
func (c *Collector) GetUserStats(ctx context.Context, telegramID int64) (*UserStats, error) {
	user, err := c.store.GetUserByTelegramID(telegramID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user with telegram_id %d not found", telegramID)
	}
	if user.LeetCodeUser == "" {
		return nil, fmt.Errorf("user with telegram_id %d has no linked LeetCode account", telegramID)
	}

	return c.FetchAndSaveStats(ctx, telegramID, user.LeetCodeUser)
}

// SyncUserSolvedProblems fetches the user's complete solved problem history from
// the LeetCode API and persists it to the database. For each solved problem, it
// also fetches and stores the problem metadata (title, difficulty, topics).
// This method is idempotent; duplicate entries are automatically ignored by the DB.
func (c *Collector) SyncUserSolvedProblems(ctx context.Context, userID int64, username string) error {
	// Fetch the list of solved problems from LeetCode.
	solved, err := c.lc.GetUserSolvedProblems(ctx, username)
	if err != nil {
		return fmt.Errorf("fetch solved problems: %w", err)
	}

	// For each solved problem, fetch metadata and persist both problem and
	// user_solved_problem records.
	for _, sp := range solved {
		// Fetch problem details to get difficulty and topics.
		details, err := c.lc.GetProblemDetails(ctx, sp.TitleSlug)
		if err != nil {
			// Log and skip this problem if we can't fetch details, but continue
			// processing the rest.
			continue
		}
		if details == nil {
			// Problem doesn't exist or was deleted; skip it.
			continue
		}

		// Extract topic names from topic tags.
		topics := make([]string, 0, len(details.TopicTags))
		for _, tag := range details.TopicTags {
			topics = append(topics, tag.Name)
		}

		// Save problem metadata. SaveProblem upserts, so it's safe to call
		// multiple times.
		if err := c.store.SaveProblem(sp.TitleSlug, details.Title, details.Difficulty, topics); err != nil {
			return fmt.Errorf("save problem %s: %w", sp.TitleSlug, err)
		}

		// Save user solved problem record. INSERT OR IGNORE handles deduplication.
		if err := c.store.SaveUserSolvedProblem(userID, sp.TitleSlug); err != nil {
			return fmt.Errorf("save user solved problem %s: %w", sp.TitleSlug, err)
		}
	}

	return nil
}

// ComputeStreak calculates the current streak (consecutive days with at least
// one submission ending at today or yesterday) from a parsed submission
// calendar map. A streak is broken if a day has zero submissions.
func ComputeStreak(calendarMap map[time.Time]int) int {
	if len(calendarMap) == 0 {
		return 0
	}

	// Start from today (UTC) and walk backwards.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	streak := 0

	// Check if today has submissions. If not, start from yesterday
	// (the user might not have solved anything yet today).
	day := today
	if calendarMap[today] == 0 {
		day = today.AddDate(0, 0, -1)
	}

	for {
		if calendarMap[day] > 0 {
			streak++
			day = day.AddDate(0, 0, -1)
		} else {
			break
		}
	}

	return streak
}

// ComputeBestStreak calculates the longest streak from a parsed submission
// calendar map by sorting all active days and finding the longest consecutive
// run.
func ComputeBestStreak(calendarMap map[time.Time]int) int {
	if len(calendarMap) == 0 {
		return 0
	}

	// Collect all days with submissions.
	var days []time.Time
	for t, count := range calendarMap {
		if count > 0 {
			days = append(days, t)
		}
	}

	if len(days) == 0 {
		return 0
	}

	// Sort days ascending.
	sort.Slice(days, func(i, j int) bool {
		return days[i].Before(days[j])
	})

	best := 1
	current := 1

	for i := 1; i < len(days); i++ {
		// Check if this day is exactly 1 day after the previous.
		if days[i].Sub(days[i-1]) == 24*time.Hour {
			current++
			if current > best {
				best = current
			}
		} else {
			current = 1
		}
	}

	return best
}

// ComputeDelta calculates the difference between a current snapshot and a
// previous snapshot. Returns nil if previous is nil.
func ComputeDelta(current, previous *storage.StatsSnapshot) *StatsDelta {
	if previous == nil {
		return nil
	}

	return &StatsDelta{
		TotalSolved:    current.TotalSolved - previous.TotalSolved,
		EasySolved:     current.EasySolved - previous.EasySolved,
		MediumSolved:   current.MediumSolved - previous.MediumSolved,
		HardSolved:     current.HardSolved - previous.HardSolved,
		AcceptanceRate: current.AcceptanceRate - previous.AcceptanceRate,
		Ranking:        current.Ranking - previous.Ranking,
	}
}

// extractSolvedCounts extracts easy, medium, hard, and total solved counts
// from the LeetCode SubmitStats structure.
func extractSolvedCounts(stats leetcode.SubmitStats) (easy, medium, hard, total int) {
	for _, s := range stats.ACSubmissionNum {
		switch s.Difficulty {
		case "Easy":
			easy = s.Count
		case "Medium":
			medium = s.Count
		case "Hard":
			hard = s.Count
		case "All":
			total = s.Count
		}
	}

	// If "All" was not provided, sum the individual counts.
	if total == 0 {
		total = easy + medium + hard
	}

	return easy, medium, hard, total
}

// computeAcceptanceRate calculates the overall acceptance rate from submission
// stats. Returns 0 if there are no total submissions.
func computeAcceptanceRate(stats leetcode.SubmitStats) float64 {
	var totalAccepted, totalSubmissions int

	for _, s := range stats.TotalSubmissionNum {
		if s.Difficulty == "All" {
			totalSubmissions = s.Submissions
		}
	}

	for _, s := range stats.ACSubmissionNum {
		if s.Difficulty == "All" {
			totalAccepted = s.Submissions
		}
	}

	if totalSubmissions == 0 {
		return 0
	}

	return float64(totalAccepted) / float64(totalSubmissions) * 100
}

// extractTotalQuestions extracts total question counts by difficulty from the
// allQuestionsCount field.
func extractTotalQuestions(counts []leetcode.QuestionCount) (easy, medium, hard int) {
	for _, q := range counts {
		switch q.Difficulty {
		case "Easy":
			easy = q.Count
		case "Medium":
			medium = q.Count
		case "Hard":
			hard = q.Count
		}
	}

	return easy, medium, hard
}
