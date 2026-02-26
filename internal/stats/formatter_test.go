package stats

import (
	"strings"
	"testing"
	"time"

	"github.com/user/leetcode-bot/internal/gamification"
	"github.com/user/leetcode-bot/internal/leetcode"
)

// ---------------------------------------------------------------------------
// FormatStats tests
// ---------------------------------------------------------------------------

func TestFormatStats_NilStats(t *testing.T) {
	got := FormatStats(nil)

	if !strings.Contains(got, "No stats available yet") {
		t.Errorf("expected nil-stats message, got %q", got)
	}
	if !strings.Contains(got, "/connect") {
		t.Errorf("expected /connect hint in nil-stats message, got %q", got)
	}
	assertValidHTML(t, got)
}

func TestFormatStats_ZeroState(t *testing.T) {
	stats := &UserStats{TotalSolved: 0}
	got := FormatStats(stats)

	// Zero-state delegates to gamification.ZeroStateMessage().
	want := gamification.ZeroStateMessage()
	if got != want {
		t.Errorf("FormatStats zero-state =\n%q\nwant\n%q", got, want)
	}
	assertValidHTML(t, got)
}

func TestFormatStats_SampleData(t *testing.T) {
	stats := &UserStats{
		TotalSolved:     150,
		EasySolved:      80,
		MediumSolved:    50,
		HardSolved:      20,
		AcceptanceRate:  65.3,
		Ranking:         42000,
		TotalEasy:       800,
		TotalMedium:     1600,
		TotalHard:       700,
		TotalActiveDays: 90,
		Delta: &StatsDelta{
			TotalSolved:    5,
			EasySolved:     2,
			MediumSolved:   2,
			HardSolved:     1,
			AcceptanceRate: 1.5,
			Ranking:        -100, // improved by 100 positions
		},
	}

	got := FormatStats(stats)

	// Verify header.
	if !strings.Contains(got, "<b>Your LeetCode Statistics</b>") {
		t.Error("missing statistics header")
	}

	// Verify total solved with delta.
	if !strings.Contains(got, "<b>Total Solved:</b> 150") {
		t.Error("missing total solved count")
	}
	if !strings.Contains(got, "(+5)") {
		t.Error("missing total solved delta")
	}

	// Verify difficulty breakdown: emojis and labels.
	if !strings.Contains(got, emojiEasy) {
		t.Error("missing easy emoji")
	}
	if !strings.Contains(got, "<b>Easy:</b> 80/800") {
		t.Error("missing easy difficulty line")
	}
	if !strings.Contains(got, "(+2)") {
		t.Error("missing easy delta")
	}
	if !strings.Contains(got, emojiMedium) {
		t.Error("missing medium emoji")
	}
	if !strings.Contains(got, "<b>Medium:</b> 50/1600") {
		t.Error("missing medium difficulty line")
	}
	if !strings.Contains(got, emojiHard) {
		t.Error("missing hard emoji")
	}
	if !strings.Contains(got, "<b>Hard:</b> 20/700") {
		t.Error("missing hard difficulty line")
	}

	// Verify acceptance rate with delta.
	if !strings.Contains(got, "<b>Acceptance Rate:</b> 65.3%") {
		t.Error("missing acceptance rate")
	}
	if !strings.Contains(got, "(+1.5%)") {
		t.Error("missing acceptance rate delta")
	}

	// Verify ranking with improvement indicator.
	if !strings.Contains(got, "<b>Ranking:</b> #42000") {
		t.Error("missing ranking")
	}
	// Negative ranking delta means improvement → ↑.
	if !strings.Contains(got, "\u2191100") {
		t.Error("missing ranking improvement indicator (↑100)")
	}

	// Verify active days.
	if !strings.Contains(got, "<b>Active Days:</b> 90") {
		t.Error("missing active days")
	}

	assertValidHTML(t, got)
}

func TestFormatStats_NoDelta(t *testing.T) {
	stats := &UserStats{
		TotalSolved:    50,
		EasySolved:     30,
		MediumSolved:   15,
		HardSolved:     5,
		AcceptanceRate: 55.0,
		Ranking:        100000,
		TotalEasy:      800,
		TotalMedium:    1600,
		TotalHard:      700,
	}

	got := FormatStats(stats)

	// Should not contain any delta indicators.
	if strings.Contains(got, "(+") {
		t.Error("should not contain positive delta when Delta is nil")
	}
	if strings.Contains(got, "\u2191") || strings.Contains(got, "\u2193") {
		t.Error("should not contain ranking arrows when Delta is nil")
	}

	assertValidHTML(t, got)
}

func TestFormatStats_ZeroDelta(t *testing.T) {
	stats := &UserStats{
		TotalSolved:    50,
		EasySolved:     30,
		MediumSolved:   15,
		HardSolved:     5,
		AcceptanceRate: 55.0,
		Ranking:        100000,
		TotalEasy:      800,
		TotalMedium:    1600,
		TotalHard:      700,
		Delta: &StatsDelta{
			TotalSolved:    0,
			EasySolved:     0,
			MediumSolved:   0,
			HardSolved:     0,
			AcceptanceRate: 0,
			Ranking:        0,
		},
	}

	got := FormatStats(stats)

	// Zero deltas should not produce delta display.
	if strings.Contains(got, "(+0)") {
		t.Error("should not show (+0) delta")
	}
	if strings.Contains(got, "\u2191") || strings.Contains(got, "\u2193") {
		t.Error("should not contain ranking arrows when delta is zero")
	}

	assertValidHTML(t, got)
}

func TestFormatStats_RankingDecline(t *testing.T) {
	stats := &UserStats{
		TotalSolved:    10,
		EasySolved:     10,
		AcceptanceRate: 50.0,
		Ranking:        200000,
		TotalEasy:      800,
		Delta: &StatsDelta{
			Ranking: 500, // positive delta means rank number went up = worse
		},
	}

	got := FormatStats(stats)

	// Positive ranking delta → worse ranking → ↓ indicator.
	if !strings.Contains(got, "\u2193500") {
		t.Error("missing ranking decline indicator (↓500)")
	}

	assertValidHTML(t, got)
}

func TestFormatStats_NegativeAcceptanceRateDelta(t *testing.T) {
	stats := &UserStats{
		TotalSolved:    50,
		EasySolved:     50,
		AcceptanceRate: 48.5,
		TotalEasy:      800,
		Delta: &StatsDelta{
			AcceptanceRate: -2.3,
		},
	}

	got := FormatStats(stats)

	// Negative acceptance rate delta should not have a + sign.
	if !strings.Contains(got, "(-2.3%)") {
		t.Error("missing negative acceptance rate delta")
	}

	assertValidHTML(t, got)
}

func TestFormatStats_NoRanking(t *testing.T) {
	stats := &UserStats{
		TotalSolved:    10,
		EasySolved:     10,
		AcceptanceRate: 50.0,
		Ranking:        0, // no ranking
		TotalEasy:      800,
	}

	got := FormatStats(stats)

	if strings.Contains(got, "Ranking") {
		t.Error("should not display ranking when it is 0")
	}

	assertValidHTML(t, got)
}

func TestFormatStats_NoActiveDays(t *testing.T) {
	stats := &UserStats{
		TotalSolved:     10,
		EasySolved:      10,
		AcceptanceRate:  50.0,
		TotalEasy:       800,
		TotalActiveDays: 0,
	}

	got := FormatStats(stats)

	if strings.Contains(got, "Active Days") {
		t.Error("should not display active days when it is 0")
	}

	assertValidHTML(t, got)
}

// ---------------------------------------------------------------------------
// FormatStreak tests
// ---------------------------------------------------------------------------

func TestFormatStreak_ZeroStreak(t *testing.T) {
	got := FormatStreak(0, 5, nil)

	if !strings.Contains(got, "<b>Streak Tracker</b>") {
		t.Error("missing streak header")
	}
	if !strings.Contains(got, "<b>Current Streak:</b> 0 days") {
		t.Error("missing zero current streak")
	}
	if !strings.Contains(got, "<b>Best Streak:</b> 5 days") {
		t.Error("missing best streak")
	}
	// Zero streak should show start-your-streak message.
	if !strings.Contains(got, "Solve a problem today to start your streak!") {
		t.Error("missing zero-streak motivational message")
	}
	// Zero streak should use snowflake emoji.
	if !strings.Contains(got, "\u2744\uFE0F") {
		t.Error("missing snowflake emoji for zero streak")
	}

	assertValidHTML(t, got)
}

func TestFormatStreak_SingleDay(t *testing.T) {
	got := FormatStreak(1, 1, nil)

	// Singular "day" (not "days") for current streak of 1.
	if !strings.Contains(got, "1 day\n") {
		t.Error("expected singular 'day' for streak of 1")
	}
	// Best streak is also 1.
	if !strings.Contains(got, "<b>Best Streak:</b> 1 day\n") {
		t.Error("expected singular 'day' for best streak of 1")
	}
	// 1-3 streak should show "Keep it going" message.
	if !strings.Contains(got, "Keep it going!") {
		t.Error("missing keep-going motivational message")
	}

	assertValidHTML(t, got)
}

func TestFormatStreak_MidStreak(t *testing.T) {
	got := FormatStreak(5, 10, nil)

	// 5-day streak: double fire.
	if !strings.Contains(got, emojiFire+emojiFire) {
		t.Error("expected double fire emoji for 5-day streak")
	}
	if !strings.Contains(got, "5 days") {
		t.Error("missing 5-day streak")
	}
	if !strings.Contains(got, "10 days") {
		t.Error("missing 10-day best streak")
	}
	// Mid streak (< 7) should show "Keep it going" message.
	if !strings.Contains(got, "Keep it going!") {
		t.Error("missing mid-streak motivational message")
	}

	assertValidHTML(t, got)
}

func TestFormatStreak_SevenDayStreak(t *testing.T) {
	got := FormatStreak(7, 7, nil)

	// 7-day streak: triple fire.
	if !strings.Contains(got, emojiFire+emojiFire+emojiFire) {
		t.Error("expected triple fire emoji for 7-day streak")
	}
	// >= 7 streak should show "Incredible consistency" message.
	if !strings.Contains(got, "Incredible consistency!") {
		t.Error("missing 7-day streak motivational message")
	}

	assertValidHTML(t, got)
}

func TestFormatStreak_LongStreak(t *testing.T) {
	got := FormatStreak(30, 30, nil)

	// 30-day streak: five fires (>= 30).
	fireCount := strings.Count(got[:strings.Index(got, "<b>Current Streak")], emojiFire)
	if fireCount < 4 {
		t.Errorf("expected at least 4 fire emojis for 30-day streak, got %d", fireCount)
	}

	assertValidHTML(t, got)
}

func TestFormatStreak_WithCalendar(t *testing.T) {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	calendar := map[time.Time]int{
		today:                   3,
		today.AddDate(0, 0, -1): 2,
		today.AddDate(0, 0, -2): 1,
		// days -3, -4, -5, -6 have no activity
	}

	got := FormatStreak(3, 10, calendar)

	// Calendar should contain green squares for active days.
	if !strings.Contains(got, squareGreen) {
		t.Error("missing green squares in calendar")
	}
	// Calendar should contain gray squares for inactive days.
	if !strings.Contains(got, squareGray) {
		t.Error("missing gray squares in calendar")
	}
	// Should have "Last 7 Days" header.
	if !strings.Contains(got, "<b>Last 7 Days:</b>") {
		t.Error("missing calendar header")
	}

	assertValidHTML(t, got)
}

// ---------------------------------------------------------------------------
// FormatDaily tests
// ---------------------------------------------------------------------------

func TestFormatDaily_FullChallenge(t *testing.T) {
	challenge := leetcode.DailyChallenge{
		Date: "2024-01-15",
		Link: "/problems/two-sum/",
		Question: leetcode.DailyQuestion{
			QuestionID:         "1",
			QuestionFrontendID: "1",
			Title:              "Two Sum",
			TitleSlug:          "two-sum",
			Difficulty:         "Easy",
			TopicTags: []leetcode.TopicTag{
				{Name: "Array", Slug: "array"},
				{Name: "Hash Table", Slug: "hash-table"},
			},
			Status: "",
		},
	}

	got := FormatDaily(challenge)

	// Verify header.
	if !strings.Contains(got, "<b>Daily Challenge</b>") {
		t.Error("missing daily challenge header")
	}

	// Verify title with frontend ID.
	if !strings.Contains(got, "<b>1. Two Sum</b>") {
		t.Error("missing title with frontend ID")
	}

	// Verify difficulty badge with emoji.
	if !strings.Contains(got, emojiEasy) {
		t.Error("missing easy emoji in difficulty badge")
	}
	if !strings.Contains(got, "<b>Easy</b>") {
		t.Error("missing difficulty label")
	}

	// Verify topic tags.
	if !strings.Contains(got, "Array") {
		t.Error("missing Array topic tag")
	}
	if !strings.Contains(got, "Hash Table") {
		t.Error("missing Hash Table topic tag")
	}
	if !strings.Contains(got, "<i>Array, Hash Table</i>") {
		t.Error("missing formatted topic tags")
	}

	// Verify link.
	if !strings.Contains(got, "https://leetcode.com/problems/two-sum/") {
		t.Error("missing problem link")
	}
	if !strings.Contains(got, "Solve it now!") {
		t.Error("missing solve prompt")
	}

	// Should not show "already solved" for non-AC status.
	if strings.Contains(got, "already solved") {
		t.Error("should not show already-solved message")
	}

	assertValidHTML(t, got)
}

func TestFormatDaily_AlreadySolved(t *testing.T) {
	challenge := leetcode.DailyChallenge{
		Date: "2024-01-15",
		Link: "/problems/two-sum/",
		Question: leetcode.DailyQuestion{
			QuestionFrontendID: "1",
			Title:              "Two Sum",
			Difficulty:         "Easy",
			Status:             "ac",
		},
	}

	got := FormatDaily(challenge)

	if !strings.Contains(got, emojiCheck) {
		t.Error("missing check emoji for solved challenge")
	}
	if !strings.Contains(got, "<b>You already solved this one!</b>") {
		t.Error("missing already-solved message")
	}

	assertValidHTML(t, got)
}

func TestFormatDaily_MediumDifficulty(t *testing.T) {
	challenge := leetcode.DailyChallenge{
		Link: "/problems/some-medium/",
		Question: leetcode.DailyQuestion{
			QuestionFrontendID: "42",
			Title:              "Some Medium Problem",
			Difficulty:         "Medium",
		},
	}

	got := FormatDaily(challenge)

	if !strings.Contains(got, emojiMedium) {
		t.Error("missing medium emoji")
	}
	if !strings.Contains(got, "<b>Medium</b>") {
		t.Error("missing medium difficulty label")
	}

	assertValidHTML(t, got)
}

func TestFormatDaily_HardDifficulty(t *testing.T) {
	challenge := leetcode.DailyChallenge{
		Link: "/problems/some-hard/",
		Question: leetcode.DailyQuestion{
			QuestionFrontendID: "123",
			Title:              "Some Hard Problem",
			Difficulty:         "Hard",
		},
	}

	got := FormatDaily(challenge)

	if !strings.Contains(got, emojiHard) {
		t.Error("missing hard emoji")
	}
	if !strings.Contains(got, "<b>Hard</b>") {
		t.Error("missing hard difficulty label")
	}

	assertValidHTML(t, got)
}

func TestFormatDaily_NoFrontendID(t *testing.T) {
	challenge := leetcode.DailyChallenge{
		Link: "/problems/mystery/",
		Question: leetcode.DailyQuestion{
			QuestionFrontendID: "",
			Title:              "Mystery Problem",
			Difficulty:         "Easy",
		},
	}

	got := FormatDaily(challenge)

	// Without frontend ID, title should not have a number prefix.
	if !strings.Contains(got, "<b>Mystery Problem</b>") {
		t.Error("missing title without frontend ID")
	}
	// Should NOT have "." prefix when no ID.
	if strings.Contains(got, ". Mystery Problem") {
		t.Error("should not have number prefix when QuestionFrontendID is empty")
	}

	assertValidHTML(t, got)
}

func TestFormatDaily_NoTopicTags(t *testing.T) {
	challenge := leetcode.DailyChallenge{
		Link: "/problems/no-tags/",
		Question: leetcode.DailyQuestion{
			QuestionFrontendID: "99",
			Title:              "No Tags Problem",
			Difficulty:         "Easy",
			TopicTags:          nil,
		},
	}

	got := FormatDaily(challenge)

	if strings.Contains(got, "Topics:") {
		t.Error("should not show Topics when there are no tags")
	}

	assertValidHTML(t, got)
}

func TestFormatDaily_FullURL(t *testing.T) {
	challenge := leetcode.DailyChallenge{
		Link: "https://leetcode.com/problems/full-url/",
		Question: leetcode.DailyQuestion{
			QuestionFrontendID: "200",
			Title:              "Full URL Problem",
			Difficulty:         "Medium",
		},
	}

	got := FormatDaily(challenge)

	// Full URL should not be double-prefixed.
	if strings.Contains(got, "https://leetcode.comhttps://") {
		t.Error("link should not be double-prefixed with https://leetcode.com")
	}
	if !strings.Contains(got, "https://leetcode.com/problems/full-url/") {
		t.Error("missing full URL link")
	}

	assertValidHTML(t, got)
}

// ---------------------------------------------------------------------------
// FormatLevel tests
// ---------------------------------------------------------------------------

func TestFormatLevel_Newcomer(t *testing.T) {
	got := FormatLevel(1, "Newcomer", 0, 50)

	if !strings.Contains(got, "<b>Your Level</b>") {
		t.Error("missing level header")
	}
	if !strings.Contains(got, "<b>Level 1 — Newcomer</b>") {
		t.Error("missing level title")
	}
	if !strings.Contains(got, "Points: <b>0</b>") {
		t.Error("missing points display")
	}
	// Should show progress bar and points to next.
	if !strings.Contains(got, "50 points to Level 2") {
		t.Error("missing points-to-next-level message")
	}
	// Progress bar should have empty segments (0 points = 0% filled).
	if !strings.Contains(got, barEmpty) {
		t.Error("missing progress bar empty segments")
	}
	if !strings.Contains(got, "[") || !strings.Contains(got, "]") {
		t.Error("missing progress bar brackets")
	}

	assertValidHTML(t, got)
}

func TestFormatLevel_MidLevel(t *testing.T) {
	// Level 5 (Practitioner), 1000 points, 500 to next.
	got := FormatLevel(5, "Practitioner", 1000, 500)

	if !strings.Contains(got, "<b>Level 5 — Practitioner</b>") {
		t.Error("missing level 5 title")
	}
	if !strings.Contains(got, "Points: <b>1000</b>") {
		t.Error("missing points")
	}
	if !strings.Contains(got, "500 points to Level 6") {
		t.Error("missing points-to-next message")
	}

	assertValidHTML(t, got)
}

func TestFormatLevel_MaxLevel(t *testing.T) {
	// Level 10 (Legend), max level, 0 points to next.
	got := FormatLevel(10, "Legend", 15000, 0)

	if !strings.Contains(got, "<b>Level 10 — Legend</b>") {
		t.Error("missing level 10 title")
	}
	if !strings.Contains(got, "Points: <b>15000</b>") {
		t.Error("missing points")
	}
	// Should show max level message.
	if !strings.Contains(got, "Maximum level reached!") {
		t.Error("missing max level message")
	}
	// Should show 100%.
	if !strings.Contains(got, "100%") {
		t.Error("missing 100% for max level")
	}
	// Should show trophy emoji.
	if !strings.Contains(got, emojiTrophy) {
		t.Error("missing trophy emoji for max level")
	}

	assertValidHTML(t, got)
}

func TestFormatLevel_ProgressBarPercentage(t *testing.T) {
	// Level 2 (Beginner), 100 points, 50 to next (Level 3 = 150 points).
	// Current level min = 50, next level min = 150, range = 100, progress = 50, pct = 50%.
	got := FormatLevel(2, "Beginner", 100, 50)

	if !strings.Contains(got, "50%") {
		t.Error("missing 50% progress")
	}
	if !strings.Contains(got, "50 points to Level 3") {
		t.Error("missing points to next level")
	}

	assertValidHTML(t, got)
}

// ---------------------------------------------------------------------------
// FormatAchievements tests
// ---------------------------------------------------------------------------

func TestFormatAchievements_MixedLockedUnlocked(t *testing.T) {
	all := gamification.AllAchievements()

	unlocked := map[string]bool{
		"first_blood": true,
		"easy_10":     true,
		"streak_7":    true,
	}

	got := FormatAchievements(unlocked, all)

	// Verify header.
	if !strings.Contains(got, "<b>Achievements</b>") {
		t.Error("missing achievements header")
	}

	// Verify count.
	if !strings.Contains(got, "Unlocked: <b>3 / 10</b>") {
		t.Error("missing or incorrect unlocked count")
	}

	// Verify unlocked achievements show check emoji.
	// First Blood should be unlocked.
	if !strings.Contains(got, emojiCheck+" "+gamification.AllAchievements()[0].Emoji+" <b>First Blood</b>") {
		t.Error("missing check emoji for unlocked First Blood")
	}

	// Verify locked achievements show lock emoji.
	// medium_10 should be locked.
	if !strings.Contains(got, emojiLock) {
		t.Error("missing lock emoji for locked achievements")
	}

	assertValidHTML(t, got)
}

func TestFormatAchievements_NoneUnlocked(t *testing.T) {
	all := gamification.AllAchievements()
	unlocked := map[string]bool{}

	got := FormatAchievements(unlocked, all)

	if !strings.Contains(got, "Unlocked: <b>0 / 10</b>") {
		t.Error("missing zero unlocked count")
	}

	// All should show lock emoji, none should show check emoji.
	checkCount := strings.Count(got, emojiCheck)
	if checkCount != 0 {
		t.Errorf("expected 0 check emojis for no unlocked, got %d", checkCount)
	}

	lockCount := strings.Count(got, emojiLock)
	if lockCount != 10 {
		t.Errorf("expected 10 lock emojis, got %d", lockCount)
	}

	assertValidHTML(t, got)
}

func TestFormatAchievements_AllUnlocked(t *testing.T) {
	all := gamification.AllAchievements()
	unlocked := make(map[string]bool)
	for _, a := range all {
		unlocked[a.Key] = true
	}

	got := FormatAchievements(unlocked, all)

	if !strings.Contains(got, "Unlocked: <b>10 / 10</b>") {
		t.Error("missing full unlocked count")
	}

	// All should show check emoji.
	checkCount := strings.Count(got, emojiCheck)
	if checkCount != 10 {
		t.Errorf("expected 10 check emojis, got %d", checkCount)
	}

	lockCount := strings.Count(got, emojiLock)
	if lockCount != 0 {
		t.Errorf("expected 0 lock emojis when all unlocked, got %d", lockCount)
	}

	assertValidHTML(t, got)
}

func TestFormatAchievements_EmptyList(t *testing.T) {
	got := FormatAchievements(nil, nil)

	if !strings.Contains(got, "Unlocked: <b>0 / 0</b>") {
		t.Error("missing zero count for empty list")
	}

	assertValidHTML(t, got)
}

func TestFormatAchievements_DescriptionsPresent(t *testing.T) {
	all := gamification.AllAchievements()
	unlocked := map[string]bool{"first_blood": true}

	got := FormatAchievements(unlocked, all)

	// Every achievement should have its description in italic tags.
	for _, a := range all {
		if !strings.Contains(got, "<i>"+a.Description+"</i>") {
			t.Errorf("missing description for achievement %q", a.Key)
		}
	}

	assertValidHTML(t, got)
}

// ---------------------------------------------------------------------------
// Helper function tests (exported via formatter behavior)
// ---------------------------------------------------------------------------

func TestStreakFires_Escalation(t *testing.T) {
	tests := []struct {
		name        string
		streak      int
		wantContain string
		wantCount   int // expected number of fire emojis (0 for snowflake)
	}{
		{"zero streak → snowflake", 0, "\u2744\uFE0F", 0},
		{"negative streak → snowflake", -1, "\u2744\uFE0F", 0},
		{"1-day → single fire", 1, emojiFire, 1},
		{"2-day → single fire", 2, emojiFire, 1},
		{"3-day → double fire", 3, emojiFire + emojiFire, 2},
		{"6-day → double fire", 6, emojiFire + emojiFire, 2},
		{"7-day → triple fire", 7, emojiFire + emojiFire + emojiFire, 3},
		{"13-day → triple fire", 13, emojiFire + emojiFire + emojiFire, 3},
		{"14-day → quad fire", 14, emojiFire + emojiFire + emojiFire + emojiFire, 4},
		{"29-day → quad fire", 29, emojiFire + emojiFire + emojiFire + emojiFire, 4},
		{"30-day → five fires", 30, emojiFire + emojiFire + emojiFire + emojiFire + emojiFire, 5},
		{"100-day → five fires", 100, emojiFire + emojiFire + emojiFire + emojiFire + emojiFire, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := streakFires(tt.streak)
			if !strings.Contains(got, tt.wantContain) {
				t.Errorf("streakFires(%d) = %q, expected to contain %q", tt.streak, got, tt.wantContain)
			}
			if tt.wantCount > 0 {
				fireCount := strings.Count(got, emojiFire)
				if fireCount != tt.wantCount {
					t.Errorf("streakFires(%d) has %d fires, want %d", tt.streak, fireCount, tt.wantCount)
				}
			}
		})
	}
}

func TestDifficultyBadge(t *testing.T) {
	tests := []struct {
		difficulty string
		wantEmoji  string
		wantLabel  string
	}{
		{"Easy", emojiEasy, "<b>Easy</b>"},
		{"Medium", emojiMedium, "<b>Medium</b>"},
		{"Hard", emojiHard, "<b>Hard</b>"},
		{"Unknown", "", "<b>Unknown</b>"},
	}

	for _, tt := range tests {
		t.Run(tt.difficulty, func(t *testing.T) {
			got := difficultyBadge(tt.difficulty)
			if tt.wantEmoji != "" && !strings.Contains(got, tt.wantEmoji) {
				t.Errorf("difficultyBadge(%q) missing emoji %q, got %q", tt.difficulty, tt.wantEmoji, got)
			}
			if !strings.Contains(got, tt.wantLabel) {
				t.Errorf("difficultyBadge(%q) missing label %q, got %q", tt.difficulty, tt.wantLabel, got)
			}
		})
	}
}

func TestFormatProgressBar(t *testing.T) {
	tests := []struct {
		name    string
		pct     int
		wantLen int // total length including brackets
	}{
		{"0%", 0, 12}, // [ + 10 chars + ]
		{"50%", 50, 12},
		{"100%", 100, 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatProgressBar(tt.pct)
			if len([]rune(got)) != tt.wantLen {
				t.Errorf("formatProgressBar(%d) rune len = %d, want %d (got %q)", tt.pct, len([]rune(got)), tt.wantLen, got)
			}
			if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
				t.Errorf("formatProgressBar(%d) should be bracketed, got %q", tt.pct, got)
			}
		})
	}

	// 0% should be all empty.
	bar0 := formatProgressBar(0)
	if strings.Contains(bar0, barFilled) {
		t.Error("0% bar should have no filled segments")
	}

	// 100% should be all filled.
	bar100 := formatProgressBar(100)
	if strings.Contains(bar100, barEmpty) {
		t.Error("100% bar should have no empty segments")
	}
}

func TestFormatMiniBar(t *testing.T) {
	// 0% → all empty.
	got0 := formatMiniBar(0)
	if strings.Contains(got0, barFilled) {
		t.Error("0% mini bar should have no filled segments")
	}

	// 100% → all filled.
	got100 := formatMiniBar(100)
	if strings.Contains(got100, barEmpty) {
		t.Error("100% mini bar should have no empty segments")
	}

	// 50% → mixed.
	got50 := formatMiniBar(50)
	if !strings.Contains(got50, barFilled) || !strings.Contains(got50, barEmpty) {
		t.Error("50% mini bar should have both filled and empty segments")
	}
}

func TestLevelMinPoints(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{1, 0},
		{2, 50},
		{3, 150},
		{4, 400},
		{5, 800},
		{6, 1500},
		{7, 3000},
		{8, 5000},
		{9, 8000},
		{10, 12000},
		{0, 0},  // out of range
		{11, 0}, // out of range
	}

	for _, tt := range tests {
		got := levelMinPoints(tt.level)
		if got != tt.want {
			t.Errorf("levelMinPoints(%d) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestDeltaVal(t *testing.T) {
	delta := &StatsDelta{
		EasySolved:   3,
		MediumSolved: 5,
		HardSolved:   1,
	}

	if got := deltaVal(delta, "easy"); got != 3 {
		t.Errorf("deltaVal easy = %d, want 3", got)
	}
	if got := deltaVal(delta, "medium"); got != 5 {
		t.Errorf("deltaVal medium = %d, want 5", got)
	}
	if got := deltaVal(delta, "hard"); got != 1 {
		t.Errorf("deltaVal hard = %d, want 1", got)
	}
	if got := deltaVal(delta, "unknown"); got != 0 {
		t.Errorf("deltaVal unknown = %d, want 0", got)
	}
	if got := deltaVal(nil, "easy"); got != 0 {
		t.Errorf("deltaVal nil easy = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// HTML tag validation
// ---------------------------------------------------------------------------

// assertValidHTML checks that HTML tags used by Telegram (b, i, a, code) are
// properly opened and closed. This is a basic check — not a full parser.
func assertValidHTML(t *testing.T, s string) {
	t.Helper()

	tags := []string{"b", "i", "code"}
	for _, tag := range tags {
		open := strings.Count(s, "<"+tag+">")
		close := strings.Count(s, "</"+tag+">")
		if open != close {
			t.Errorf("HTML tag <%s>: %d opens vs %d closes in:\n%s", tag, open, close, s)
		}
	}

	// Check <a href="..."> tags separately since they have attributes.
	aOpen := strings.Count(s, "<a ")
	aClose := strings.Count(s, "</a>")
	if aOpen != aClose {
		t.Errorf("HTML tag <a>: %d opens vs %d closes in:\n%s", aOpen, aClose, s)
	}
}
