package gamification

import (
	"context"
	"fmt"

	"github.com/user/leetcode-bot/internal/storage"
)

// Points awarded per action.
const (
	PointsEasy          = 10
	PointsMedium        = 25
	PointsHard          = 50
	PointsDailyStreak   = 5
	Points7DayBonus     = 50
	Points30DayBonus    = 300
)

// levelThreshold defines the minimum points required for a level and its title.
type levelThreshold struct {
	MinPoints int
	Title     string
}

// levels defines the 10 levels from Newcomer to Legend, ordered ascending.
var levels = []levelThreshold{
	{0, "Newcomer"},       // Level 1
	{50, "Beginner"},      // Level 2
	{150, "Apprentice"},   // Level 3
	{400, "Solver"},       // Level 4
	{800, "Practitioner"}, // Level 5
	{1500, "Veteran"},     // Level 6
	{3000, "Expert"},      // Level 7
	{5000, "Master"},      // Level 8
	{8000, "Grandmaster"}, // Level 9
	{12000, "Legend"},     // Level 10
}

// Engine provides gamification calculations and user stat updates.
type Engine struct {
	storage *storage.DB
}

// NewEngine creates a new gamification engine with the given storage dependency.
func NewEngine(store *storage.DB) *Engine {
	return &Engine{storage: store}
}

// SolveStats holds the number of problems solved by difficulty and streak info.
type SolveStats struct {
	EasySolved   int
	MediumSolved int
	HardSolved   int
	StreakDays   int
}

// CalculatePoints computes the total points earned based on problems solved
// by difficulty and current streak length, per the spec points table.
func CalculatePoints(easySolved, mediumSolved, hardSolved, streakDays int) int {
	points := easySolved*PointsEasy +
		mediumSolved*PointsMedium +
		hardSolved*PointsHard +
		streakDays*PointsDailyStreak

	// 7-day streak bonus (awarded once per qualifying streak).
	if streakDays >= 7 {
		points += Points7DayBonus
	}

	// 30-day streak bonus (awarded once per qualifying streak).
	if streakDays >= 30 {
		points += Points30DayBonus
	}

	return points
}

// DetermineLevel returns the level number (1-10) and title for the given
// point total, based on the spec level thresholds table.
func DetermineLevel(points int) (level int, title string) {
	// Walk backwards through levels to find the highest one the user qualifies for.
	for i := len(levels) - 1; i >= 0; i-- {
		if points >= levels[i].MinPoints {
			return i + 1, levels[i].Title
		}
	}

	// Fallback (should not happen since level 1 threshold is 0).
	return 1, levels[0].Title
}

// PointsToNextLevel returns the number of points remaining until the next
// level. Returns 0 if the user is already at the maximum level.
func PointsToNextLevel(points int) int {
	level, _ := DetermineLevel(points)

	// Already at max level.
	if level >= len(levels) {
		return 0
	}

	return levels[level].MinPoints - points
}

// LevelTitle returns the title for a given level number (1-indexed).
// Returns "Newcomer" for out-of-range levels.
func LevelTitle(level int) string {
	if level < 1 || level > len(levels) {
		return levels[0].Title
	}
	return levels[level-1].Title
}

// MaxLevel returns the maximum achievable level.
func MaxLevel() int {
	return len(levels)
}

// UpdateUserStats recalculates a user's gamification state from the new
// solve stats, updates points and level, and persists the changes.
// It returns the previous and new points so callers can determine deltas.
func (e *Engine) UpdateUserStats(ctx context.Context, telegramID int64, newStats SolveStats) (prevPoints, newPoints int, err error) {
	user, err := e.storage.GetUserByTelegramID(telegramID)
	if err != nil {
		return 0, 0, fmt.Errorf("get user for gamification update: %w", err)
	}
	if user == nil {
		return 0, 0, fmt.Errorf("user with telegram_id %d not found", telegramID)
	}

	prevPoints = user.Points

	// Calculate new total points from current solve stats.
	newPoints = CalculatePoints(
		newStats.EasySolved,
		newStats.MediumSolved,
		newStats.HardSolved,
		newStats.StreakDays,
	)

	// Determine new level from the computed points.
	newLevel, _ := DetermineLevel(newPoints)

	// Update streak: keep the best streak as the max of stored and current.
	bestStreak := user.BestStreak
	if newStats.StreakDays > bestStreak {
		bestStreak = newStats.StreakDays
	}

	// Persist updated gamification state.
	if err := e.storage.UpdateUserGamification(
		telegramID,
		newPoints,
		newLevel,
		newStats.StreakDays,
		bestStreak,
	); err != nil {
		return 0, 0, fmt.Errorf("persist gamification update: %w", err)
	}

	return prevPoints, newPoints, nil
}
