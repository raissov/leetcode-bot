package gamification

// UserStats holds the aggregated stats used for checking achievement eligibility.
// This is populated from the user's latest LeetCode data and stored state.
type UserStats struct {
	TotalSolved    int
	EasySolved     int
	MediumSolved   int
	HardSolved     int
	CurrentStreak  int
	BestStreak     int
	TotalEasy      int  // Total easy problems available on LeetCode.
	SolvedDaily    bool // Whether the user solved today's daily challenge.
}

// Achievement defines a single unlockable achievement with its check logic.
type Achievement struct {
	Key         string
	Name        string
	Description string
	Emoji       string
	CheckFunc   func(stats UserStats) bool
}

// AllAchievements returns the full list of defined achievements, per the spec.
// Order matches the spec table: first_blood, easy_10, medium_10, hard_5,
// streak_7, streak_30, century, half_k, daily_solver, all_easy.
func AllAchievements() []Achievement {
	return []Achievement{
		{
			Key:         "first_blood",
			Name:        "First Blood",
			Description: "Solve your first problem",
			Emoji:       "\U0001F3AF", // 🎯
			CheckFunc: func(stats UserStats) bool {
				return stats.TotalSolved >= 1
			},
		},
		{
			Key:         "easy_10",
			Name:        "Easy Peasy",
			Description: "Solve 10 easy problems",
			Emoji:       "\U0001F7E2", // 🟢
			CheckFunc: func(stats UserStats) bool {
				return stats.EasySolved >= 10
			},
		},
		{
			Key:         "medium_10",
			Name:        "Getting Serious",
			Description: "Solve 10 medium problems",
			Emoji:       "\U0001F7E1", // 🟡
			CheckFunc: func(stats UserStats) bool {
				return stats.MediumSolved >= 10
			},
		},
		{
			Key:         "hard_5",
			Name:        "Hard Hitter",
			Description: "Solve 5 hard problems",
			Emoji:       "\U0001F534", // 🔴
			CheckFunc: func(stats UserStats) bool {
				return stats.HardSolved >= 5
			},
		},
		{
			Key:         "streak_7",
			Name:        "Week Warrior",
			Description: "Achieve a 7-day streak",
			Emoji:       "\U0001F525", // 🔥
			CheckFunc: func(stats UserStats) bool {
				return stats.CurrentStreak >= 7 || stats.BestStreak >= 7
			},
		},
		{
			Key:         "streak_30",
			Name:        "Monthly Master",
			Description: "Achieve a 30-day streak",
			Emoji:       "\U0001F30D", // 🌍
			CheckFunc: func(stats UserStats) bool {
				return stats.CurrentStreak >= 30 || stats.BestStreak >= 30
			},
		},
		{
			Key:         "century",
			Name:        "Century",
			Description: "Solve 100 total problems",
			Emoji:       "\U0001F4AF", // 💯
			CheckFunc: func(stats UserStats) bool {
				return stats.TotalSolved >= 100
			},
		},
		{
			Key:         "half_k",
			Name:        "Half K",
			Description: "Solve 500 total problems",
			Emoji:       "\U0001F3C6", // 🏆
			CheckFunc: func(stats UserStats) bool {
				return stats.TotalSolved >= 500
			},
		},
		{
			Key:         "daily_solver",
			Name:        "Daily Devotee",
			Description: "Solve the daily challenge",
			Emoji:       "\U0001F4C5", // 📅
			CheckFunc: func(stats UserStats) bool {
				return stats.SolvedDaily
			},
		},
		{
			Key:         "all_easy",
			Name:        "Easy Sweep",
			Description: "Solve all easy problems",
			Emoji:       "\U0001F9F9", // 🧹
			CheckFunc: func(stats UserStats) bool {
				// TotalEasy must be known (> 0) to avoid false positives.
				return stats.TotalEasy > 0 && stats.EasySolved >= stats.TotalEasy
			},
		},
	}
}

// achievementsByKey is a lazily-cached map for fast key lookups.
var achievementsByKey map[string]Achievement

// AchievementByKey returns the Achievement definition for the given key,
// or nil if the key is not recognised.
func AchievementByKey(key string) *Achievement {
	if achievementsByKey == nil {
		achievementsByKey = make(map[string]Achievement, 10)
		for _, a := range AllAchievements() {
			achievementsByKey[a.Key] = a
		}
	}
	a, ok := achievementsByKey[key]
	if !ok {
		return nil
	}
	return &a
}

// CheckEligibility evaluates all defined achievements against the provided
// user stats and the set of already-unlocked achievement keys. It returns
// the keys of achievements that are newly eligible (i.e., the check passes
// and the key is not in the alreadyUnlocked set).
func CheckEligibility(stats UserStats, alreadyUnlocked map[string]bool) []string {
	var newly []string

	for _, a := range AllAchievements() {
		if alreadyUnlocked[a.Key] {
			continue
		}
		if a.CheckFunc(stats) {
			newly = append(newly, a.Key)
		}
	}

	return newly
}
