package gamification

import (
	"fmt"
	"math/rand"
)

// motivationalQuotes is a curated list of motivational quotes for daily use.
var motivationalQuotes = []string{
	"Every problem you solve makes the next one easier. Keep going! 💪",
	"Consistency beats talent. One problem a day keeps imposter syndrome away. 🎯",
	"The best time to solve a LeetCode problem was yesterday. The second best time is now. ⏰",
	"You don't have to be great to start, but you have to start to be great. 🌟",
	"Small daily improvements are the key to staggering long-term results. 📈",
	"A journey of a thousand problems begins with a single AC. ✅",
	"The harder the problem, the sweeter the acceptance. Keep pushing! 🚀",
	"Your future self will thank you for the problems you solve today. 🙏",
	"Debugging is like being a detective in a crime movie where you are also the criminal. 🔍",
	"Code is like humor. When you have to explain it, it's bad. Write clean solutions! ✨",
	"It's not about how fast you solve it — it's about showing up every day. 🏃",
	"Think of each problem as a puzzle, not a test. Enjoy the process! 🧩",
	"The only way to learn data structures is to use them. Solve on! 📚",
}

// LevelUpMessage returns a congratulatory announcement for reaching a new level.
func LevelUpMessage(level int, title string) string {
	return fmt.Sprintf(
		"🎉 <b>LEVEL UP!</b>\n\n"+
			"You've reached <b>Level %d — %s</b>!\n"+
			"Keep solving to unlock even greater heights! 🚀",
		level, title,
	)
}

// StreakMessage returns an escalating encouragement message based on the
// current streak length. Longer streaks get more enthusiastic messages.
func StreakMessage(days int) string {
	switch {
	case days <= 0:
		return "Start your streak today! Solve one problem and the fire begins. 🔥"
	case days < 3:
		return fmt.Sprintf("🔥 <b>%d-day streak!</b> You're getting started — keep it up!", days)
	case days < 7:
		return fmt.Sprintf("🔥🔥 <b>%d-day streak!</b> You're building momentum! Don't stop now!", days)
	case days < 14:
		return fmt.Sprintf("🔥🔥🔥 <b>%d-day streak!</b> A whole week and counting — you're on fire!", days)
	case days < 30:
		return fmt.Sprintf("🔥🔥🔥🔥 <b>%d-day streak!</b> Incredible consistency! You're unstoppable!", days)
	case days < 100:
		return fmt.Sprintf("🔥🔥🔥🔥🔥 <b>%d-day streak!</b> Monthly master! You're in the elite now! 💎", days)
	default:
		return fmt.Sprintf("🔥🔥🔥🔥🔥🔥 <b>%d-day streak!</b> LEGENDARY! You are an absolute machine! 👑", days)
	}
}

// AchievementUnlockedMessage returns a celebratory message for a newly
// unlocked achievement, including its emoji and description.
func AchievementUnlockedMessage(achievement Achievement) string {
	return fmt.Sprintf(
		"🏅 <b>Achievement Unlocked!</b>\n\n"+
			"%s <b>%s</b>\n"+
			"<i>%s</i>\n\n"+
			"Congratulations! Keep going for more! 🎊",
		achievement.Emoji, achievement.Name, achievement.Description,
	)
}

// DailyMotivation returns a random motivational quote from the curated list.
func DailyMotivation() string {
	return motivationalQuotes[rand.Intn(len(motivationalQuotes))]
}

// ZeroStateMessage returns an encouraging message for first-time users
// who haven't solved any problems yet or just connected their account.
func ZeroStateMessage() string {
	return "🌱 <b>Welcome, future LeetCode champion!</b>\n\n" +
		"You haven't solved any problems yet — but every expert was once a beginner.\n" +
		"Start with an Easy problem today and watch your stats grow!\n\n" +
		"Use /daily to see today's challenge. You've got this! 💪"
}
