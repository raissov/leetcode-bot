package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// User represents a row in the users table.
type User struct {
	ID             int64
	TelegramID     int64
	TelegramName   string
	LeetCodeUser   string
	Timezone       string
	RemindHour     int
	RemindEnabled  bool
	Points         int
	Level          int
	CurrentStreak  int
	BestStreak     int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CreateUser inserts a new user with the given Telegram ID and display name.
// If a user with the same Telegram ID already exists, the call is a no-op
// (INSERT OR IGNORE) and the existing user is returned.
func (d *DB) CreateUser(telegramID int64, telegramName string) (*User, error) {
	_, err := d.db.Exec(
		`INSERT OR IGNORE INTO users (telegram_id, telegram_name) VALUES (?, ?)`,
		telegramID, telegramName,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return d.GetUserByTelegramID(telegramID)
}

// GetUserByTelegramID retrieves a user by their Telegram ID.
// Returns (nil, nil) if the user does not exist.
func (d *DB) GetUserByTelegramID(telegramID int64) (*User, error) {
	u := &User{}
	var remindEnabled int

	err := d.db.QueryRow(
		`SELECT id, telegram_id, telegram_name, leetcode_user, timezone,
		        remind_hour, remind_enabled, points, level,
		        current_streak, best_streak, created_at, updated_at
		 FROM users WHERE telegram_id = ?`,
		telegramID,
	).Scan(
		&u.ID, &u.TelegramID, &u.TelegramName, &u.LeetCodeUser,
		&u.Timezone, &u.RemindHour, &remindEnabled, &u.Points,
		&u.Level, &u.CurrentStreak, &u.BestStreak,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by telegram id: %w", err)
	}

	u.RemindEnabled = remindEnabled == 1
	return u, nil
}

// UpdateLeetCodeUser links a LeetCode username to the user identified by telegramID.
func (d *DB) UpdateLeetCodeUser(telegramID int64, leetcodeUser string) error {
	result, err := d.db.Exec(
		`UPDATE users SET leetcode_user = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE telegram_id = ?`,
		leetcodeUser, telegramID,
	)
	if err != nil {
		return fmt.Errorf("update leetcode user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update leetcode user rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("update leetcode user: user with telegram_id %d not found", telegramID)
	}

	return nil
}

// UpdateUserGamification updates the gamification fields (points, level, streaks)
// for the user identified by telegramID.
func (d *DB) UpdateUserGamification(telegramID int64, points, level, currentStreak, bestStreak int) error {
	result, err := d.db.Exec(
		`UPDATE users
		 SET points = ?, level = ?, current_streak = ?, best_streak = ?,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE telegram_id = ?`,
		points, level, currentStreak, bestStreak, telegramID,
	)
	if err != nil {
		return fmt.Errorf("update user gamification: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update user gamification rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("update user gamification: user with telegram_id %d not found", telegramID)
	}

	return nil
}

// UpdateReminder updates the reminder settings (enabled, hour, timezone)
// for the user identified by telegramID.
func (d *DB) UpdateReminder(telegramID int64, enabled bool, hour int, timezone string) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	result, err := d.db.Exec(
		`UPDATE users
		 SET remind_enabled = ?, remind_hour = ?, timezone = ?,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE telegram_id = ?`,
		enabledInt, hour, timezone, telegramID,
	)
	if err != nil {
		return fmt.Errorf("update reminder: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update reminder rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("update reminder: user with telegram_id %d not found", telegramID)
	}

	return nil
}

// GetAllUsersWithReminders returns all users who have reminders enabled
// and have a linked LeetCode account.
func (d *DB) GetAllUsersWithReminders() ([]*User, error) {
	rows, err := d.db.Query(
		`SELECT id, telegram_id, telegram_name, leetcode_user, timezone,
		        remind_hour, remind_enabled, points, level,
		        current_streak, best_streak, created_at, updated_at
		 FROM users
		 WHERE remind_enabled = 1 AND leetcode_user != ''`,
	)
	if err != nil {
		return nil, fmt.Errorf("get all users with reminders: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		var remindEnabled int

		if err := rows.Scan(
			&u.ID, &u.TelegramID, &u.TelegramName, &u.LeetCodeUser,
			&u.Timezone, &u.RemindHour, &remindEnabled, &u.Points,
			&u.Level, &u.CurrentStreak, &u.BestStreak,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user with reminder: %w", err)
		}

		u.RemindEnabled = remindEnabled == 1
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users with reminders: %w", err)
	}

	return users, nil
}
