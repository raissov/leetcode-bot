package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Problem represents a row in the problems table.
type Problem struct {
	ID         int64
	Slug       string
	Title      string
	Difficulty string
	Topics     []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UserSolvedProblem represents a row in the user_solved_problems table.
type UserSolvedProblem struct {
	ID          int64
	UserID      int64
	ProblemSlug string
	SolvedAt    time.Time
}

// SaveProblem inserts or updates a problem by slug.
// If a problem with the same slug already exists, it is updated.
func (d *DB) SaveProblem(slug, title, difficulty string, topics []string) error {
	topicsJSON, err := json.Marshal(topics)
	if err != nil {
		return fmt.Errorf("marshal topics: %w", err)
	}

	_, err = d.db.Exec(
		`INSERT INTO problems (slug, title, difficulty, topics_json)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET
		    title       = excluded.title,
		    difficulty  = excluded.difficulty,
		    topics_json = excluded.topics_json,
		    updated_at  = CURRENT_TIMESTAMP`,
		slug, title, difficulty, string(topicsJSON),
	)
	if err != nil {
		return fmt.Errorf("save problem: %w", err)
	}

	return nil
}

// GetProblem retrieves a problem by its slug.
// Returns (nil, nil) if the problem does not exist.
func (d *DB) GetProblem(slug string) (*Problem, error) {
	p := &Problem{}
	var topicsJSON string

	err := d.db.QueryRow(
		`SELECT id, slug, title, difficulty, topics_json, created_at, updated_at
		 FROM problems WHERE slug = ?`,
		slug,
	).Scan(
		&p.ID, &p.Slug, &p.Title, &p.Difficulty,
		&topicsJSON, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get problem: %w", err)
	}

	if err := json.Unmarshal([]byte(topicsJSON), &p.Topics); err != nil {
		return nil, fmt.Errorf("unmarshal topics: %w", err)
	}

	return p, nil
}

// GetProblemsByTopics retrieves all problems that include any of the specified topics.
// Topics are matched case-insensitively within the JSON array.
func (d *DB) GetProblemsByTopics(topics []string) ([]*Problem, error) {
	if len(topics) == 0 {
		return []*Problem{}, nil
	}

	// Build a query that checks if any topic appears in the topics_json field.
	// SQLite's json_each() is used to expand the JSON array and match topics.
	query := `SELECT DISTINCT p.id, p.slug, p.title, p.difficulty, p.topics_json, p.created_at, p.updated_at
		FROM problems p, json_each(p.topics_json) AS topic
		WHERE LOWER(topic.value) IN (`

	args := make([]interface{}, len(topics))
	placeholders := make([]string, len(topics))
	for i, topic := range topics {
		placeholders[i] = "?"
		args[i] = topic
	}

	query += fmt.Sprintf("%s)", fmt.Sprintf("?%s", ""))
	query = `SELECT DISTINCT p.id, p.slug, p.title, p.difficulty, p.topics_json, p.created_at, p.updated_at
		FROM problems p, json_each(p.topics_json) AS topic
		WHERE LOWER(topic.value) IN (` + fmt.Sprintf("%s", joinPlaceholders(len(topics))) + `)`

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get problems by topics: %w", err)
	}
	defer rows.Close()

	var problems []*Problem
	for rows.Next() {
		p := &Problem{}
		var topicsJSON string

		if err := rows.Scan(
			&p.ID, &p.Slug, &p.Title, &p.Difficulty,
			&topicsJSON, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan problem: %w", err)
		}

		if err := json.Unmarshal([]byte(topicsJSON), &p.Topics); err != nil {
			return nil, fmt.Errorf("unmarshal topics: %w", err)
		}

		problems = append(problems, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate problems: %w", err)
	}

	return problems, nil
}

// joinPlaceholders returns a string of N question marks separated by commas.
func joinPlaceholders(n int) string {
	if n == 0 {
		return ""
	}
	s := "?"
	for i := 1; i < n; i++ {
		s += ",?"
	}
	return s
}

// SaveUserSolvedProblem records that a user has solved a specific problem.
// If the user has already solved this problem, it is a no-op (INSERT OR IGNORE).
func (d *DB) SaveUserSolvedProblem(userID int64, problemSlug string) error {
	_, err := d.db.Exec(
		`INSERT OR IGNORE INTO user_solved_problems (user_id, problem_slug)
		 VALUES (?, ?)`,
		userID, problemSlug,
	)
	if err != nil {
		return fmt.Errorf("save user solved problem: %w", err)
	}

	return nil
}

// GetUserSolvedProblems retrieves all problems that a user has solved.
// Returns an empty slice if the user has not solved any problems.
func (d *DB) GetUserSolvedProblems(userID int64) ([]*UserSolvedProblem, error) {
	rows, err := d.db.Query(
		`SELECT id, user_id, problem_slug, solved_at
		 FROM user_solved_problems
		 WHERE user_id = ?
		 ORDER BY solved_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get user solved problems: %w", err)
	}
	defer rows.Close()

	var solvedProblems []*UserSolvedProblem
	for rows.Next() {
		sp := &UserSolvedProblem{}

		if err := rows.Scan(
			&sp.ID, &sp.UserID, &sp.ProblemSlug, &sp.SolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user solved problem: %w", err)
		}

		solvedProblems = append(solvedProblems, sp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user solved problems: %w", err)
	}

	return solvedProblems, nil
}
