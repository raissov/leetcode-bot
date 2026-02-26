package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// CuratedList represents a row in the curated_lists table.
type CuratedList struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CuratedListProblem represents a row in the curated_list_problems table.
type CuratedListProblem struct {
	ID          int64
	ListID      int64
	ProblemSlug string
	Position    int
	CreatedAt   time.Time
}

// CuratedListWithProblems represents a curated list with its associated problems.
type CuratedListWithProblems struct {
	List         *CuratedList
	ProblemSlugs []string
}

// UserListProgress represents a user's progress on a curated list.
type UserListProgress struct {
	ListName     string
	TotalCount   int
	SolvedCount  int
	Percentage   float64
	SolvedSlugs  []string
}

// curatedListData is the JSON structure for loading curated list files.
type curatedListData struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Problems    []struct {
		Slug       string   `json:"slug"`
		Title      string   `json:"title"`
		Difficulty string   `json:"difficulty"`
		Topics     []string `json:"topics"`
	} `json:"problems"`
}

// LoadCuratedList loads a curated list from a JSON file and inserts it into the database.
// If the list already exists, it is replaced with the new data.
// This also saves all problems from the list into the problems table.
func (d *DB) LoadCuratedList(path string) error {
	// Read and parse the JSON file.
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var listData curatedListData
	if err := json.Unmarshal(data, &listData); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}

	// Begin transaction for atomic insert.
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert or update the curated list.
	_, err = tx.Exec(
		`INSERT INTO curated_lists (name, description)
		 VALUES (?, ?)
		 ON CONFLICT(name) DO UPDATE SET
		    description = excluded.description,
		    updated_at  = CURRENT_TIMESTAMP`,
		listData.Name, listData.Description,
	)
	if err != nil {
		return fmt.Errorf("insert curated list: %w", err)
	}

	// Get the list ID (either from INSERT or from existing row).
	var listID int64
	err = tx.QueryRow("SELECT id FROM curated_lists WHERE name = ?", listData.Name).Scan(&listID)
	if err != nil {
		return fmt.Errorf("get list id: %w", err)
	}

	// Delete existing problems for this list to avoid duplicates.
	if _, err := tx.Exec("DELETE FROM curated_list_problems WHERE list_id = ?", listID); err != nil {
		return fmt.Errorf("delete existing list problems: %w", err)
	}

	// Insert all problems from the list.
	for i, problem := range listData.Problems {
		// First, save the problem to the problems table.
		topicsJSON, err := json.Marshal(problem.Topics)
		if err != nil {
			return fmt.Errorf("marshal topics for problem %s: %w", problem.Slug, err)
		}

		_, err = tx.Exec(
			`INSERT INTO problems (slug, title, difficulty, topics_json)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(slug) DO UPDATE SET
			    title       = excluded.title,
			    difficulty  = excluded.difficulty,
			    topics_json = excluded.topics_json,
			    updated_at  = CURRENT_TIMESTAMP`,
			problem.Slug, problem.Title, problem.Difficulty, string(topicsJSON),
		)
		if err != nil {
			return fmt.Errorf("insert problem %s: %w", problem.Slug, err)
		}

		// Then, add it to the curated list.
		_, err = tx.Exec(
			`INSERT INTO curated_list_problems (list_id, problem_slug, position)
			 VALUES (?, ?, ?)`,
			listID, problem.Slug, i,
		)
		if err != nil {
			return fmt.Errorf("insert list problem %s: %w", problem.Slug, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetCuratedList retrieves a curated list by name along with its associated problem slugs.
// Returns (nil, nil) if the list does not exist.
func (d *DB) GetCuratedList(name string) (*CuratedListWithProblems, error) {
	list := &CuratedList{}

	err := d.db.QueryRow(
		`SELECT id, name, description, created_at, updated_at
		 FROM curated_lists
		 WHERE name = ?`,
		name,
	).Scan(&list.ID, &list.Name, &list.Description, &list.CreatedAt, &list.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get curated list: %w", err)
	}

	// Get all problems for this list, ordered by position.
	rows, err := d.db.Query(
		`SELECT problem_slug
		 FROM curated_list_problems
		 WHERE list_id = ?
		 ORDER BY position ASC`,
		list.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("get list problems: %w", err)
	}
	defer rows.Close()

	var problemSlugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("scan problem slug: %w", err)
		}
		problemSlugs = append(problemSlugs, slug)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate problem slugs: %w", err)
	}

	return &CuratedListWithProblems{
		List:         list,
		ProblemSlugs: problemSlugs,
	}, nil
}

// GetUserProgressOnList computes a user's progress on a specific curated list.
// Returns the total count, solved count, percentage, and list of solved slugs.
// Returns (nil, nil) if the list does not exist.
func (d *DB) GetUserProgressOnList(userID int64, listName string) (*UserListProgress, error) {
	// Get the curated list.
	listWithProblems, err := d.GetCuratedList(listName)
	if err != nil {
		return nil, fmt.Errorf("get curated list: %w", err)
	}
	if listWithProblems == nil {
		return nil, nil
	}

	totalCount := len(listWithProblems.ProblemSlugs)
	if totalCount == 0 {
		return &UserListProgress{
			ListName:    listName,
			TotalCount:  0,
			SolvedCount: 0,
			Percentage:  0,
			SolvedSlugs: []string{},
		}, nil
	}

	// Get all problems the user has solved.
	rows, err := d.db.Query(
		`SELECT problem_slug
		 FROM user_solved_problems
		 WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get user solved problems: %w", err)
	}
	defer rows.Close()

	// Build a set of solved slugs for fast lookup.
	solvedSet := make(map[string]bool)
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("scan solved problem: %w", err)
		}
		solvedSet[slug] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate solved problems: %w", err)
	}

	// Count how many problems from the list the user has solved.
	var solvedSlugs []string
	for _, slug := range listWithProblems.ProblemSlugs {
		if solvedSet[slug] {
			solvedSlugs = append(solvedSlugs, slug)
		}
	}

	solvedCount := len(solvedSlugs)
	percentage := 0.0
	if totalCount > 0 {
		percentage = (float64(solvedCount) / float64(totalCount)) * 100.0
	}

	return &UserListProgress{
		ListName:    listName,
		TotalCount:  totalCount,
		SolvedCount: solvedCount,
		Percentage:  percentage,
		SolvedSlugs: solvedSlugs,
	}, nil
}
