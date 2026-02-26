package gamification

import "testing"

// ---------------------------------------------------------------------------
// CalculatePoints tests
// ---------------------------------------------------------------------------

func TestCalculatePoints_EasyOnly(t *testing.T) {
	got := CalculatePoints(5, 0, 0, 0)
	want := 5 * PointsEasy // 50
	if got != want {
		t.Errorf("CalculatePoints(5,0,0,0) = %d, want %d", got, want)
	}
}

func TestCalculatePoints_MediumOnly(t *testing.T) {
	got := CalculatePoints(0, 3, 0, 0)
	want := 3 * PointsMedium // 75
	if got != want {
		t.Errorf("CalculatePoints(0,3,0,0) = %d, want %d", got, want)
	}
}

func TestCalculatePoints_HardOnly(t *testing.T) {
	got := CalculatePoints(0, 0, 2, 0)
	want := 2 * PointsHard // 100
	if got != want {
		t.Errorf("CalculatePoints(0,0,2,0) = %d, want %d", got, want)
	}
}

func TestCalculatePoints_MixedDifficulty(t *testing.T) {
	got := CalculatePoints(10, 5, 2, 0)
	want := 10*PointsEasy + 5*PointsMedium + 2*PointsHard // 100+125+100 = 325
	if got != want {
		t.Errorf("CalculatePoints(10,5,2,0) = %d, want %d", got, want)
	}
}

func TestCalculatePoints_ZeroValues(t *testing.T) {
	got := CalculatePoints(0, 0, 0, 0)
	if got != 0 {
		t.Errorf("CalculatePoints(0,0,0,0) = %d, want 0", got)
	}
}

func TestCalculatePoints_StreakWithoutBonus(t *testing.T) {
	// 3-day streak: no bonus, just daily streak points.
	got := CalculatePoints(1, 0, 0, 3)
	want := 1*PointsEasy + 3*PointsDailyStreak // 10+15 = 25
	if got != want {
		t.Errorf("CalculatePoints(1,0,0,3) = %d, want %d", got, want)
	}
}

func TestCalculatePoints_Streak6Days(t *testing.T) {
	// 6-day streak: still below 7-day bonus threshold.
	got := CalculatePoints(0, 0, 0, 6)
	want := 6 * PointsDailyStreak // 30
	if got != want {
		t.Errorf("CalculatePoints(0,0,0,6) = %d, want %d", got, want)
	}
}

func TestCalculatePoints_Streak7DayBonus(t *testing.T) {
	// Exactly 7-day streak: earns 7-day bonus.
	got := CalculatePoints(0, 0, 0, 7)
	want := 7*PointsDailyStreak + Points7DayBonus // 35+50 = 85
	if got != want {
		t.Errorf("CalculatePoints(0,0,0,7) = %d, want %d", got, want)
	}
}

func TestCalculatePoints_Streak15Days(t *testing.T) {
	// 15-day streak: earns 7-day bonus but not 30-day bonus.
	got := CalculatePoints(0, 0, 0, 15)
	want := 15*PointsDailyStreak + Points7DayBonus // 75+50 = 125
	if got != want {
		t.Errorf("CalculatePoints(0,0,0,15) = %d, want %d", got, want)
	}
}

func TestCalculatePoints_Streak30DayBonus(t *testing.T) {
	// Exactly 30-day streak: earns both 7-day and 30-day bonuses.
	got := CalculatePoints(0, 0, 0, 30)
	want := 30*PointsDailyStreak + Points7DayBonus + Points30DayBonus // 150+50+300 = 500
	if got != want {
		t.Errorf("CalculatePoints(0,0,0,30) = %d, want %d", got, want)
	}
}

func TestCalculatePoints_Streak29Days(t *testing.T) {
	// 29-day streak: earns 7-day bonus but NOT 30-day bonus.
	got := CalculatePoints(0, 0, 0, 29)
	want := 29*PointsDailyStreak + Points7DayBonus // 145+50 = 195
	if got != want {
		t.Errorf("CalculatePoints(0,0,0,29) = %d, want %d", got, want)
	}
}

func TestCalculatePoints_FullCombo(t *testing.T) {
	// Mixed difficulty + 30-day streak: all bonuses.
	got := CalculatePoints(10, 5, 2, 30)
	want := 10*PointsEasy + 5*PointsMedium + 2*PointsHard +
		30*PointsDailyStreak + Points7DayBonus + Points30DayBonus
	// 100 + 125 + 100 + 150 + 50 + 300 = 825
	if got != want {
		t.Errorf("CalculatePoints(10,5,2,30) = %d, want %d", got, want)
	}
}

// ---------------------------------------------------------------------------
// DetermineLevel tests
// ---------------------------------------------------------------------------

func TestDetermineLevel_Boundaries(t *testing.T) {
	tests := []struct {
		name      string
		points    int
		wantLevel int
		wantTitle string
	}{
		{"0 points → Newcomer", 0, 1, "Newcomer"},
		{"49 points → Newcomer", 49, 1, "Newcomer"},
		{"50 points → Beginner", 50, 2, "Beginner"},
		{"51 points → Beginner", 51, 2, "Beginner"},
		{"149 points → Beginner", 149, 2, "Beginner"},
		{"150 points → Apprentice", 150, 3, "Apprentice"},
		{"399 points → Apprentice", 399, 3, "Apprentice"},
		{"400 points → Solver", 400, 4, "Solver"},
		{"799 points → Solver", 799, 4, "Solver"},
		{"800 points → Practitioner", 800, 5, "Practitioner"},
		{"1499 points → Practitioner", 1499, 5, "Practitioner"},
		{"1500 points → Veteran", 1500, 6, "Veteran"},
		{"2999 points → Veteran", 2999, 6, "Veteran"},
		{"3000 points → Expert", 3000, 7, "Expert"},
		{"4999 points → Expert", 4999, 7, "Expert"},
		{"5000 points → Master", 5000, 8, "Master"},
		{"7999 points → Master", 7999, 8, "Master"},
		{"8000 points → Grandmaster", 8000, 9, "Grandmaster"},
		{"11999 points → Grandmaster", 11999, 9, "Grandmaster"},
		{"12000 points → Legend", 12000, 10, "Legend"},
		{"99999 points → Legend", 99999, 10, "Legend"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, title := DetermineLevel(tt.points)
			if level != tt.wantLevel {
				t.Errorf("DetermineLevel(%d) level = %d, want %d", tt.points, level, tt.wantLevel)
			}
			if title != tt.wantTitle {
				t.Errorf("DetermineLevel(%d) title = %q, want %q", tt.points, title, tt.wantTitle)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PointsToNextLevel tests
// ---------------------------------------------------------------------------

func TestPointsToNextLevel(t *testing.T) {
	tests := []struct {
		name   string
		points int
		want   int
	}{
		{"0 points → 50 to Beginner", 0, 50},
		{"25 points → 25 to Beginner", 25, 25},
		{"49 points → 1 to Beginner", 49, 1},
		{"50 points → 100 to Apprentice", 50, 100},
		{"100 points → 50 to Apprentice", 100, 50},
		{"150 points → 250 to Solver", 150, 250},
		{"8000 points → 4000 to Legend", 8000, 4000},
		{"11999 points → 1 to Legend", 11999, 1},
		{"12000 points → 0 (max level)", 12000, 0},
		{"99999 points → 0 (beyond max)", 99999, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PointsToNextLevel(tt.points)
			if got != tt.want {
				t.Errorf("PointsToNextLevel(%d) = %d, want %d", tt.points, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LevelTitle tests
// ---------------------------------------------------------------------------

func TestLevelTitle(t *testing.T) {
	tests := []struct {
		level int
		want  string
	}{
		{1, "Newcomer"},
		{2, "Beginner"},
		{3, "Apprentice"},
		{4, "Solver"},
		{5, "Practitioner"},
		{6, "Veteran"},
		{7, "Expert"},
		{8, "Master"},
		{9, "Grandmaster"},
		{10, "Legend"},
	}

	for _, tt := range tests {
		got := LevelTitle(tt.level)
		if got != tt.want {
			t.Errorf("LevelTitle(%d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestLevelTitle_OutOfRange(t *testing.T) {
	// Out-of-range levels should return "Newcomer".
	for _, level := range []int{0, -1, 11, 100} {
		got := LevelTitle(level)
		if got != "Newcomer" {
			t.Errorf("LevelTitle(%d) = %q, want %q", level, got, "Newcomer")
		}
	}
}

// ---------------------------------------------------------------------------
// MaxLevel tests
// ---------------------------------------------------------------------------

func TestMaxLevel(t *testing.T) {
	got := MaxLevel()
	if got != 10 {
		t.Errorf("MaxLevel() = %d, want 10", got)
	}
}

// ---------------------------------------------------------------------------
// Achievement eligibility tests
// ---------------------------------------------------------------------------

func TestCheckEligibility_FirstBlood(t *testing.T) {
	stats := UserStats{TotalSolved: 1}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "first_blood") {
		t.Errorf("expected first_blood in newly eligible, got %v", newly)
	}
}

func TestCheckEligibility_FirstBlood_NotEligible(t *testing.T) {
	stats := UserStats{TotalSolved: 0}
	newly := CheckEligibility(stats, nil)

	if contains(newly, "first_blood") {
		t.Errorf("first_blood should not be eligible with 0 solves, got %v", newly)
	}
}

func TestCheckEligibility_Easy10(t *testing.T) {
	stats := UserStats{TotalSolved: 10, EasySolved: 10}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "easy_10") {
		t.Errorf("expected easy_10 in newly eligible, got %v", newly)
	}
}

func TestCheckEligibility_Easy10_NotEnough(t *testing.T) {
	stats := UserStats{TotalSolved: 9, EasySolved: 9}
	newly := CheckEligibility(stats, nil)

	if contains(newly, "easy_10") {
		t.Errorf("easy_10 should not be eligible with 9 easy solves, got %v", newly)
	}
}

func TestCheckEligibility_Medium10(t *testing.T) {
	stats := UserStats{TotalSolved: 10, MediumSolved: 10}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "medium_10") {
		t.Errorf("expected medium_10 in newly eligible, got %v", newly)
	}
}

func TestCheckEligibility_Hard5(t *testing.T) {
	stats := UserStats{TotalSolved: 5, HardSolved: 5}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "hard_5") {
		t.Errorf("expected hard_5 in newly eligible, got %v", newly)
	}
}

func TestCheckEligibility_Hard5_NotEnough(t *testing.T) {
	stats := UserStats{TotalSolved: 4, HardSolved: 4}
	newly := CheckEligibility(stats, nil)

	if contains(newly, "hard_5") {
		t.Errorf("hard_5 should not be eligible with 4 hard solves, got %v", newly)
	}
}

func TestCheckEligibility_Streak7_CurrentStreak(t *testing.T) {
	stats := UserStats{CurrentStreak: 7}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "streak_7") {
		t.Errorf("expected streak_7 in newly eligible, got %v", newly)
	}
}

func TestCheckEligibility_Streak7_BestStreak(t *testing.T) {
	// streak_7 should trigger from BestStreak even if CurrentStreak < 7.
	stats := UserStats{CurrentStreak: 2, BestStreak: 7}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "streak_7") {
		t.Errorf("expected streak_7 via BestStreak, got %v", newly)
	}
}

func TestCheckEligibility_Streak7_NotEnough(t *testing.T) {
	stats := UserStats{CurrentStreak: 6, BestStreak: 6}
	newly := CheckEligibility(stats, nil)

	if contains(newly, "streak_7") {
		t.Errorf("streak_7 should not be eligible with 6-day streak, got %v", newly)
	}
}

func TestCheckEligibility_Streak30(t *testing.T) {
	stats := UserStats{CurrentStreak: 30}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "streak_30") {
		t.Errorf("expected streak_30 in newly eligible, got %v", newly)
	}
}

func TestCheckEligibility_Streak30_BestStreak(t *testing.T) {
	stats := UserStats{CurrentStreak: 5, BestStreak: 30}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "streak_30") {
		t.Errorf("expected streak_30 via BestStreak, got %v", newly)
	}
}

func TestCheckEligibility_Century(t *testing.T) {
	stats := UserStats{TotalSolved: 100}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "century") {
		t.Errorf("expected century in newly eligible, got %v", newly)
	}
}

func TestCheckEligibility_Century_NotEnough(t *testing.T) {
	stats := UserStats{TotalSolved: 99}
	newly := CheckEligibility(stats, nil)

	if contains(newly, "century") {
		t.Errorf("century should not be eligible with 99 solves, got %v", newly)
	}
}

func TestCheckEligibility_HalfK(t *testing.T) {
	stats := UserStats{TotalSolved: 500}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "half_k") {
		t.Errorf("expected half_k in newly eligible, got %v", newly)
	}
}

func TestCheckEligibility_DailySolver(t *testing.T) {
	stats := UserStats{SolvedDaily: true}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "daily_solver") {
		t.Errorf("expected daily_solver in newly eligible, got %v", newly)
	}
}

func TestCheckEligibility_DailySolver_NotSolved(t *testing.T) {
	stats := UserStats{SolvedDaily: false}
	newly := CheckEligibility(stats, nil)

	if contains(newly, "daily_solver") {
		t.Errorf("daily_solver should not be eligible when daily not solved, got %v", newly)
	}
}

func TestCheckEligibility_AllEasy(t *testing.T) {
	stats := UserStats{TotalSolved: 800, EasySolved: 800, TotalEasy: 800}
	newly := CheckEligibility(stats, nil)

	if !contains(newly, "all_easy") {
		t.Errorf("expected all_easy in newly eligible, got %v", newly)
	}
}

func TestCheckEligibility_AllEasy_UnknownTotal(t *testing.T) {
	// TotalEasy = 0 should prevent all_easy from triggering (avoids false positives).
	stats := UserStats{TotalSolved: 800, EasySolved: 800, TotalEasy: 0}
	newly := CheckEligibility(stats, nil)

	if contains(newly, "all_easy") {
		t.Errorf("all_easy should not be eligible when TotalEasy=0, got %v", newly)
	}
}

func TestCheckEligibility_AllEasy_NotComplete(t *testing.T) {
	stats := UserStats{TotalSolved: 500, EasySolved: 799, TotalEasy: 800}
	newly := CheckEligibility(stats, nil)

	if contains(newly, "all_easy") {
		t.Errorf("all_easy should not be eligible with 799/800 easy, got %v", newly)
	}
}

// ---------------------------------------------------------------------------
// Duplicate achievement detection
// ---------------------------------------------------------------------------

func TestCheckEligibility_NoDuplicates(t *testing.T) {
	// Stats that qualify for first_blood, easy_10, and century.
	stats := UserStats{
		TotalSolved: 100,
		EasySolved:  50,
	}

	alreadyUnlocked := map[string]bool{
		"first_blood": true,
		"easy_10":     true,
	}

	newly := CheckEligibility(stats, alreadyUnlocked)

	// first_blood and easy_10 should NOT appear because they are already unlocked.
	if contains(newly, "first_blood") {
		t.Error("first_blood should not appear in newly (already unlocked)")
	}
	if contains(newly, "easy_10") {
		t.Error("easy_10 should not appear in newly (already unlocked)")
	}

	// century should still appear as newly eligible.
	if !contains(newly, "century") {
		t.Errorf("expected century in newly eligible, got %v", newly)
	}
}

func TestCheckEligibility_AllAlreadyUnlocked(t *testing.T) {
	stats := UserStats{
		TotalSolved:   500,
		EasySolved:    200,
		MediumSolved:  200,
		HardSolved:    100,
		CurrentStreak: 30,
		BestStreak:    30,
		TotalEasy:     200,
		SolvedDaily:   true,
	}

	// Mark every achievement as already unlocked.
	alreadyUnlocked := map[string]bool{
		"first_blood":  true,
		"easy_10":      true,
		"medium_10":    true,
		"hard_5":       true,
		"streak_7":     true,
		"streak_30":    true,
		"century":      true,
		"half_k":       true,
		"daily_solver": true,
		"all_easy":     true,
	}

	newly := CheckEligibility(stats, alreadyUnlocked)
	if len(newly) != 0 {
		t.Errorf("expected no newly eligible achievements, got %v", newly)
	}
}

func TestCheckEligibility_EmptyAlreadyUnlocked(t *testing.T) {
	// With a high-stat user and no prior unlocks, all eligible should appear.
	stats := UserStats{
		TotalSolved:   500,
		EasySolved:    200,
		MediumSolved:  200,
		HardSolved:    100,
		CurrentStreak: 30,
		BestStreak:    30,
		TotalEasy:     200,
		SolvedDaily:   true,
	}

	newly := CheckEligibility(stats, nil)

	// All 10 achievements should be newly eligible.
	if len(newly) != 10 {
		t.Errorf("expected 10 newly eligible achievements, got %d: %v", len(newly), newly)
	}
}

// ---------------------------------------------------------------------------
// AchievementByKey tests
// ---------------------------------------------------------------------------

func TestAchievementByKey_Found(t *testing.T) {
	a := AchievementByKey("first_blood")
	if a == nil {
		t.Fatal("expected achievement, got nil")
	}
	if a.Name != "First Blood" {
		t.Errorf("name = %q, want %q", a.Name, "First Blood")
	}
}

func TestAchievementByKey_NotFound(t *testing.T) {
	a := AchievementByKey("nonexistent_key")
	if a != nil {
		t.Errorf("expected nil for unknown key, got %+v", a)
	}
}

// ---------------------------------------------------------------------------
// AllAchievements tests
// ---------------------------------------------------------------------------

func TestAllAchievements_Count(t *testing.T) {
	achievements := AllAchievements()
	if len(achievements) != 10 {
		t.Errorf("AllAchievements() returned %d items, want 10", len(achievements))
	}
}

func TestAllAchievements_UniqueKeys(t *testing.T) {
	achievements := AllAchievements()
	seen := make(map[string]bool)

	for _, a := range achievements {
		if seen[a.Key] {
			t.Errorf("duplicate achievement key: %q", a.Key)
		}
		seen[a.Key] = true
	}
}

// ---------------------------------------------------------------------------
// NewEngine tests
// ---------------------------------------------------------------------------

func TestNewEngine(t *testing.T) {
	engine := NewEngine(nil)
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// contains checks if a string slice contains the given value.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
