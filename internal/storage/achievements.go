package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// Achievement represents a row in the achievements table.
type Achievement struct {
	ID             int64
	UserID         int64
	AchievementKey string
	UnlockedAt     time.Time
}

// UnlockAchievement records that a user has earned the given achievement.
// If the user already has the achievement, the call is a no-op (INSERT OR IGNORE)
// thanks to the UNIQUE(user_id, achievement_key) constraint.
// Returns true if the achievement was newly unlocked, false if it already existed.
func (d *DB) UnlockAchievement(userID int64, achievementKey string) (bool, error) {
	result, err := d.db.Exec(
		`INSERT OR IGNORE INTO achievements (user_id, achievement_key) VALUES (?, ?)`,
		userID, achievementKey,
	)
	if err != nil {
		return false, fmt.Errorf("unlock achievement: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("unlock achievement rows affected: %w", err)
	}

	return rows > 0, nil
}

// GetUserAchievements returns all achievements unlocked by the given user,
// ordered by unlock time ascending.
func (d *DB) GetUserAchievements(userID int64) ([]*Achievement, error) {
	rows, err := d.db.Query(
		`SELECT id, user_id, achievement_key, unlocked_at
		 FROM achievements
		 WHERE user_id = ?
		 ORDER BY unlocked_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get user achievements: %w", err)
	}
	defer rows.Close()

	var achievements []*Achievement
	for rows.Next() {
		a := &Achievement{}

		if err := rows.Scan(
			&a.ID, &a.UserID, &a.AchievementKey, &a.UnlockedAt,
		); err != nil {
			return nil, fmt.Errorf("scan achievement: %w", err)
		}

		achievements = append(achievements, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate achievements: %w", err)
	}

	return achievements, nil
}

// HasAchievement checks whether the given user has already unlocked the
// specified achievement. Returns (false, nil) if the achievement does not exist.
func (d *DB) HasAchievement(userID int64, achievementKey string) (bool, error) {
	var count int

	err := d.db.QueryRow(
		`SELECT COUNT(1) FROM achievements
		 WHERE user_id = ? AND achievement_key = ?`,
		userID, achievementKey,
	).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("has achievement: %w", err)
	}

	return count > 0, nil
}
