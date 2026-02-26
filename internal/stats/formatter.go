package stats

import (
	"fmt"
	"strings"
	"time"

	"github.com/user/leetcode-bot/internal/gamification"
	"github.com/user/leetcode-bot/internal/leetcode"
)

// --- Emoji constants ---

const (
	emojiEasy   = "\U0001F7E2" // 🟢
	emojiMedium = "\U0001F7E1" // 🟡
	emojiHard   = "\U0001F534" // 🔴
	emojiFire   = "\U0001F525" // 🔥
	emojiStar   = "\u2B50"     // ⭐
	emojiTrophy = "\U0001F3C6" // 🏆
	emojiChart  = "\U0001F4CA" // 📊
	emojiTarget = "\U0001F3AF" // 🎯
	emojiRocket = "\U0001F680" // 🚀
	emojiLock   = "\U0001F512" // 🔓
	emojiCheck  = "\u2705"     // ✅
	emojiCal    = "\U0001F4C5" // 📅
	emojiLink   = "\U0001F517" // 🔗

	squareGreen = "\U0001F7E9" // 🟩
	squareGray  = "\u2B1C"     // ⬜

	barFilled = "\u2588" // █
	barEmpty  = "\u2591" // ░
)

// FormatStats formats the user's comprehensive statistics as an HTML-formatted
// Telegram message. Includes total solved, difficulty breakdown with emoji
// bars, acceptance rate, ranking, and progress delta since last check.
func FormatStats(stats *UserStats) string {
	if stats == nil {
		return emojiChart + " <b>No stats available yet.</b>\nConnect your LeetCode account with /connect."
	}

	// Zero-state: no problems solved.
	if stats.TotalSolved == 0 {
		return gamification.ZeroStateMessage()
	}

	var b strings.Builder

	b.WriteString(emojiChart + " <b>Your LeetCode Statistics</b>\n\n")

	// Total solved.
	b.WriteString(fmt.Sprintf(emojiTarget+" <b>Total Solved:</b> %d", stats.TotalSolved))
	if stats.Delta != nil && stats.Delta.TotalSolved > 0 {
		b.WriteString(fmt.Sprintf("  <i>(+%d)</i>", stats.Delta.TotalSolved))
	}
	b.WriteString("\n\n")

	// Difficulty breakdown with bars.
	b.WriteString("<b>Difficulty Breakdown:</b>\n")
	b.WriteString(formatDifficultyLine(emojiEasy, "Easy", stats.EasySolved, stats.TotalEasy, deltaVal(stats.Delta, "easy")))
	b.WriteString(formatDifficultyLine(emojiMedium, "Medium", stats.MediumSolved, stats.TotalMedium, deltaVal(stats.Delta, "medium")))
	b.WriteString(formatDifficultyLine(emojiHard, "Hard", stats.HardSolved, stats.TotalHard, deltaVal(stats.Delta, "hard")))
	b.WriteString("\n")

	// Acceptance rate.
	b.WriteString(fmt.Sprintf(emojiStar+" <b>Acceptance Rate:</b> %.1f%%", stats.AcceptanceRate))
	if stats.Delta != nil && stats.Delta.AcceptanceRate != 0 {
		sign := "+"
		if stats.Delta.AcceptanceRate < 0 {
			sign = ""
		}
		b.WriteString(fmt.Sprintf("  <i>(%s%.1f%%)</i>", sign, stats.Delta.AcceptanceRate))
	}
	b.WriteString("\n")

	// Ranking.
	if stats.Ranking > 0 {
		b.WriteString(fmt.Sprintf(emojiTrophy+" <b>Ranking:</b> #%d", stats.Ranking))
		if stats.Delta != nil && stats.Delta.Ranking != 0 {
			// Negative ranking delta means improvement (lower rank = better).
			diff := -stats.Delta.Ranking
			if diff > 0 {
				b.WriteString(fmt.Sprintf("  <i>(\u2191%d)</i>", diff))
			} else if diff < 0 {
				b.WriteString(fmt.Sprintf("  <i>(\u2193%d)</i>", -diff))
			}
		}
		b.WriteString("\n")
	}

	// Active days.
	if stats.TotalActiveDays > 0 {
		b.WriteString(fmt.Sprintf(emojiCal+" <b>Active Days:</b> %d\n", stats.TotalActiveDays))
	}

	return b.String()
}

// FormatStreak formats the user's streak information as an HTML-formatted
// Telegram message. Shows fire emoji, streak count, best streak, and a mini
// activity calendar (green/gray squares for the last 7 days).
func FormatStreak(currentStreak, bestStreak int, recentCalendar map[time.Time]int) string {
	var b strings.Builder

	b.WriteString(emojiFire + " <b>Streak Tracker</b>\n\n")

	// Current streak with fire emoji escalation.
	fires := streakFires(currentStreak)
	b.WriteString(fmt.Sprintf("%s <b>Current Streak:</b> %d day", fires, currentStreak))
	if currentStreak != 1 {
		b.WriteString("s")
	}
	b.WriteString("\n")

	// Best streak.
	b.WriteString(fmt.Sprintf(emojiTrophy+" <b>Best Streak:</b> %d day", bestStreak))
	if bestStreak != 1 {
		b.WriteString("s")
	}
	b.WriteString("\n\n")

	// Mini activity calendar for the last 7 days.
	b.WriteString("<b>Last 7 Days:</b>\n")
	b.WriteString(formatMiniCalendar(recentCalendar))
	b.WriteString("\n")

	// Motivational message based on streak.
	if currentStreak == 0 {
		b.WriteString("\nSolve a problem today to start your streak! " + emojiRocket)
	} else if currentStreak >= 7 {
		b.WriteString("\nIncredible consistency! Keep the fire alive! " + emojiFire)
	} else {
		b.WriteString("\nKeep it going! Every day counts! " + emojiStar)
	}

	return b.String()
}

// FormatDaily formats today's daily coding challenge as an HTML-formatted
// Telegram message with title, difficulty badge, topic tags, and link.
func FormatDaily(challenge leetcode.DailyChallenge) string {
	var b strings.Builder

	b.WriteString(emojiCal + " <b>Daily Challenge</b>\n\n")

	// Title with question number.
	if challenge.Question.QuestionFrontendID != "" {
		b.WriteString(fmt.Sprintf("<b>%s. %s</b>\n", challenge.Question.QuestionFrontendID, challenge.Question.Title))
	} else {
		b.WriteString(fmt.Sprintf("<b>%s</b>\n", challenge.Question.Title))
	}

	// Difficulty badge.
	b.WriteString(fmt.Sprintf("Difficulty: %s\n", difficultyBadge(challenge.Question.Difficulty)))

	// Topic tags.
	if len(challenge.Question.TopicTags) > 0 {
		tags := make([]string, len(challenge.Question.TopicTags))
		for i, t := range challenge.Question.TopicTags {
			tags[i] = t.Name
		}
		b.WriteString(fmt.Sprintf("Topics: <i>%s</i>\n", strings.Join(tags, ", ")))
	}

	// Link.
	link := challenge.Link
	if !strings.HasPrefix(link, "http") {
		link = "https://leetcode.com" + link
	}
	b.WriteString(fmt.Sprintf("\n%s <a href=\"%s\">Solve it now!</a>", emojiLink, link))

	// Status.
	if challenge.Question.Status == "ac" {
		b.WriteString("\n\n" + emojiCheck + " <b>You already solved this one!</b>")
	}

	return b.String()
}

// FormatLevel formats the user's gamification level as an HTML-formatted
// Telegram message with level number, title, points, and a progress bar to
// the next level.
func FormatLevel(level int, title string, points, pointsToNext int) string {
	var b strings.Builder

	b.WriteString(emojiStar + " <b>Your Level</b>\n\n")

	b.WriteString(fmt.Sprintf("<b>Level %d — %s</b>\n", level, title))
	b.WriteString(fmt.Sprintf("Points: <b>%d</b>\n\n", points))

	// Progress bar.
	if pointsToNext > 0 {
		// Calculate progress within the current level bracket.
		currentLevelMin := levelMinPoints(level)
		nextLevelMin := currentLevelMin + pointsToNext + (points - currentLevelMin)
		// Actually: pointsToNext = nextLevelMin - points, so nextLevelMin = points + pointsToNext
		nextLevelMin = points + pointsToNext
		rangeTotal := nextLevelMin - currentLevelMin
		rangeProgress := points - currentLevelMin

		pct := 0
		if rangeTotal > 0 {
			pct = rangeProgress * 100 / rangeTotal
		}
		if pct > 100 {
			pct = 100
		}

		b.WriteString(formatProgressBar(pct))
		b.WriteString(fmt.Sprintf(" %d%%\n", pct))
		b.WriteString(fmt.Sprintf("<i>%d points to Level %d</i>\n", pointsToNext, level+1))
	} else {
		// Max level reached.
		b.WriteString(formatProgressBar(100))
		b.WriteString(" 100%\n")
		b.WriteString("<i>Maximum level reached!</i> " + emojiTrophy + "\n")
	}

	return b.String()
}

// FormatAchievements formats the user's achievement list as an HTML-formatted
// Telegram message showing locked/unlocked status for each achievement.
func FormatAchievements(unlocked map[string]bool, all []gamification.Achievement) string {
	var b strings.Builder

	b.WriteString(emojiTrophy + " <b>Achievements</b>\n\n")

	unlockedCount := 0
	for _, a := range all {
		if unlocked[a.Key] {
			unlockedCount++
		}
	}

	b.WriteString(fmt.Sprintf("Unlocked: <b>%d / %d</b>\n\n", unlockedCount, len(all)))

	for _, a := range all {
		if unlocked[a.Key] {
			b.WriteString(fmt.Sprintf("%s %s <b>%s</b>\n", emojiCheck, a.Emoji, a.Name))
			b.WriteString(fmt.Sprintf("    <i>%s</i>\n", a.Description))
		} else {
			b.WriteString(fmt.Sprintf("%s %s <b>%s</b>\n", emojiLock, a.Emoji, a.Name))
			b.WriteString(fmt.Sprintf("    <i>%s</i>\n", a.Description))
		}
	}

	return b.String()
}

// --- Helper functions ---

// formatDifficultyLine formats a single difficulty row with emoji, label,
// solved count, total, bar, and optional delta.
func formatDifficultyLine(emoji, label string, solved, total, delta int) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s <b>%s:</b> %d", emoji, label, solved))
	if total > 0 {
		b.WriteString(fmt.Sprintf("/%d", total))
	}
	if delta > 0 {
		b.WriteString(fmt.Sprintf("  <i>(+%d)</i>", delta))
	}

	// Mini progress bar if total is known.
	if total > 0 {
		pct := solved * 100 / total
		b.WriteString("  " + formatMiniBar(pct))
	}

	b.WriteString("\n")
	return b.String()
}

// deltaVal extracts the delta value for a given difficulty from the StatsDelta.
func deltaVal(delta *StatsDelta, difficulty string) int {
	if delta == nil {
		return 0
	}
	switch difficulty {
	case "easy":
		return delta.EasySolved
	case "medium":
		return delta.MediumSolved
	case "hard":
		return delta.HardSolved
	default:
		return 0
	}
}

// difficultyBadge returns a colored emoji + bold label for a difficulty string.
func difficultyBadge(difficulty string) string {
	switch difficulty {
	case "Easy":
		return emojiEasy + " <b>Easy</b>"
	case "Medium":
		return emojiMedium + " <b>Medium</b>"
	case "Hard":
		return emojiHard + " <b>Hard</b>"
	default:
		return "<b>" + difficulty + "</b>"
	}
}

// streakFires returns escalating fire emoji based on streak length.
func streakFires(streak int) string {
	switch {
	case streak <= 0:
		return "\u2744\uFE0F" // ❄️
	case streak < 3:
		return emojiFire
	case streak < 7:
		return emojiFire + emojiFire
	case streak < 14:
		return emojiFire + emojiFire + emojiFire
	case streak < 30:
		return emojiFire + emojiFire + emojiFire + emojiFire
	default:
		return emojiFire + emojiFire + emojiFire + emojiFire + emojiFire
	}
}

// formatMiniCalendar renders a 7-day activity calendar using green/gray
// squares. Each square represents one day, from 6 days ago to today (left
// to right). Below the squares, abbreviated day names are shown.
func formatMiniCalendar(calendarMap map[time.Time]int) string {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	var squares strings.Builder
	var labels strings.Builder

	for i := 6; i >= 0; i-- {
		day := today.AddDate(0, 0, -i)
		if calendarMap != nil && calendarMap[day] > 0 {
			squares.WriteString(squareGreen)
		} else {
			squares.WriteString(squareGray)
		}
		// Abbreviated day name (Mo, Tu, ...).
		labels.WriteString(day.Format("Mo")[0:2])
		if i > 0 {
			labels.WriteString(" ")
		}
	}

	return squares.String() + "\n<code>" + labels.String() + "</code>"
}

// formatProgressBar returns a text-based progress bar of fixed width using
// filled/empty block characters.
func formatProgressBar(pct int) string {
	const barWidth = 10

	filled := pct * barWidth / 100
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}

	return "[" + strings.Repeat(barFilled, filled) + strings.Repeat(barEmpty, barWidth-filled) + "]"
}

// formatMiniBar returns a short progress bar for inline use.
func formatMiniBar(pct int) string {
	const barWidth = 6

	filled := pct * barWidth / 100
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}

	return strings.Repeat(barFilled, filled) + strings.Repeat(barEmpty, barWidth-filled)
}

// levelMinPoints returns the minimum points required for the given level.
func levelMinPoints(level int) int {
	thresholds := []int{0, 50, 150, 400, 800, 1500, 3000, 5000, 8000, 12000}

	if level < 1 || level > len(thresholds) {
		return 0
	}
	return thresholds[level-1]
}
