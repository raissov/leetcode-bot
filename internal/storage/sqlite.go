package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps a *sql.DB connection to the SQLite database.
type DB struct {
	db *sql.DB
}

// Open creates a new SQLite database connection at the given path,
// configures it for WAL mode, foreign keys, and busy timeout,
// then runs auto-migrations to ensure all required tables exist.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?cache=shared&mode=rwc")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite single-writer limitation — only one connection allowed.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	// Enable WAL mode via PRAGMA (NOT via connection string — modernc.org/sqlite
	// does not support _journal_mode URI parameter, unlike mattn/go-sqlite3).
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Enable foreign key enforcement (OFF by default in SQLite).
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	// Set busy timeout for concurrent access.
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	store := &DB{db: db}

	// Run auto-migrations.
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return store, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// migrate runs CREATE TABLE IF NOT EXISTS statements for all required tables.
func (d *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_id     INTEGER UNIQUE NOT NULL,
			telegram_name   TEXT NOT NULL DEFAULT '',
			leetcode_user   TEXT NOT NULL DEFAULT '',
			timezone        TEXT NOT NULL DEFAULT 'UTC',
			remind_hour     INTEGER NOT NULL DEFAULT 9,
			remind_enabled  INTEGER NOT NULL DEFAULT 1,
			points          INTEGER NOT NULL DEFAULT 0,
			level           INTEGER NOT NULL DEFAULT 1,
			current_streak  INTEGER NOT NULL DEFAULT 0,
			best_streak     INTEGER NOT NULL DEFAULT 0,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS stats_snapshots (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id         INTEGER NOT NULL REFERENCES users(id),
			total_solved    INTEGER NOT NULL DEFAULT 0,
			easy_solved     INTEGER NOT NULL DEFAULT 0,
			medium_solved   INTEGER NOT NULL DEFAULT 0,
			hard_solved     INTEGER NOT NULL DEFAULT 0,
			acceptance_rate REAL NOT NULL DEFAULT 0,
			ranking         INTEGER NOT NULL DEFAULT 0,
			snapshot_date   DATE NOT NULL,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, snapshot_date)
		)`,
		`CREATE TABLE IF NOT EXISTS achievements (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id         INTEGER NOT NULL REFERENCES users(id),
			achievement_key TEXT NOT NULL,
			unlocked_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, achievement_key)
		)`,
	}

	for _, m := range migrations {
		if _, err := d.db.Exec(m); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	return nil
}
