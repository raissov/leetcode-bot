package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// StatsSnapshot represents a row in the stats_snapshots table.
type StatsSnapshot struct {
	ID             int64
	UserID         int64
	TotalSolved    int
	EasySolved     int
	MediumSolved   int
	HardSolved     int
	AcceptanceRate float64
	Ranking        int
	SnapshotDate   time.Time
	CreatedAt      time.Time
}

// SaveSnapshot upserts a stats snapshot for the given user and date.
// If a snapshot already exists for that user+date pair, it is replaced.
func (d *DB) SaveSnapshot(userID int64, totalSolved, easySolved, mediumSolved, hardSolved int, acceptanceRate float64, ranking int, date time.Time) error {
	_, err := d.db.Exec(
		`INSERT INTO stats_snapshots
		    (user_id, total_solved, easy_solved, medium_solved, hard_solved,
		     acceptance_rate, ranking, snapshot_date)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, snapshot_date) DO UPDATE SET
		    total_solved    = excluded.total_solved,
		    easy_solved     = excluded.easy_solved,
		    medium_solved   = excluded.medium_solved,
		    hard_solved     = excluded.hard_solved,
		    acceptance_rate = excluded.acceptance_rate,
		    ranking         = excluded.ranking`,
		userID, totalSolved, easySolved, mediumSolved, hardSolved,
		acceptanceRate, ranking, date.Format("2006-01-02"),
	)
	if err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}

	return nil
}

// GetLatestSnapshot returns the most recent stats snapshot for the given user.
// Returns (nil, nil) if no snapshots exist.
func (d *DB) GetLatestSnapshot(userID int64) (*StatsSnapshot, error) {
	s := &StatsSnapshot{}

	err := d.db.QueryRow(
		`SELECT id, user_id, total_solved, easy_solved, medium_solved, hard_solved,
		        acceptance_rate, ranking, snapshot_date, created_at
		 FROM stats_snapshots
		 WHERE user_id = ?
		 ORDER BY snapshot_date DESC
		 LIMIT 1`,
		userID,
	).Scan(
		&s.ID, &s.UserID, &s.TotalSolved, &s.EasySolved,
		&s.MediumSolved, &s.HardSolved, &s.AcceptanceRate,
		&s.Ranking, &s.SnapshotDate, &s.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest snapshot: %w", err)
	}

	return s, nil
}

// GetPreviousSnapshot returns the second most recent stats snapshot for the
// given user, useful for computing deltas between the latest and previous state.
// Returns (nil, nil) if fewer than two snapshots exist.
func (d *DB) GetPreviousSnapshot(userID int64) (*StatsSnapshot, error) {
	s := &StatsSnapshot{}

	err := d.db.QueryRow(
		`SELECT id, user_id, total_solved, easy_solved, medium_solved, hard_solved,
		        acceptance_rate, ranking, snapshot_date, created_at
		 FROM stats_snapshots
		 WHERE user_id = ?
		 ORDER BY snapshot_date DESC
		 LIMIT 1 OFFSET 1`,
		userID,
	).Scan(
		&s.ID, &s.UserID, &s.TotalSolved, &s.EasySolved,
		&s.MediumSolved, &s.HardSolved, &s.AcceptanceRate,
		&s.Ranking, &s.SnapshotDate, &s.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get previous snapshot: %w", err)
	}

	return s, nil
}

// GetSnapshotHistory returns up to the last N days of stats snapshots for the
// given user, ordered from oldest to newest.
func (d *DB) GetSnapshotHistory(userID int64, days int) ([]*StatsSnapshot, error) {
	rows, err := d.db.Query(
		`SELECT id, user_id, total_solved, easy_solved, medium_solved, hard_solved,
		        acceptance_rate, ranking, snapshot_date, created_at
		 FROM stats_snapshots
		 WHERE user_id = ? AND snapshot_date >= DATE('now', ?)
		 ORDER BY snapshot_date ASC`,
		userID, fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return nil, fmt.Errorf("get snapshot history: %w", err)
	}
	defer rows.Close()

	var snapshots []*StatsSnapshot
	for rows.Next() {
		s := &StatsSnapshot{}

		if err := rows.Scan(
			&s.ID, &s.UserID, &s.TotalSolved, &s.EasySolved,
			&s.MediumSolved, &s.HardSolved, &s.AcceptanceRate,
			&s.Ranking, &s.SnapshotDate, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}

		snapshots = append(snapshots, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate snapshots: %w", err)
	}

	return snapshots, nil
}
