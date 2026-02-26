package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Curated Lists
// ---------------------------------------------------------------------------

func TestLoadCuratedList(t *testing.T) {
	db := testDB(t)

	// Create a test JSON file.
	listData := curatedListData{
		Name:        "Test List",
		Description: "A test curated list",
		Problems: []struct {
			Slug       string   `json:"slug"`
			Title      string   `json:"title"`
			Difficulty string   `json:"difficulty"`
			Topics     []string `json:"topics"`
		}{
			{
				Slug:       "two-sum",
				Title:      "Two Sum",
				Difficulty: "Easy",
				Topics:     []string{"Array", "Hash Table"},
			},
			{
				Slug:       "add-two-numbers",
				Title:      "Add Two Numbers",
				Difficulty: "Medium",
				Topics:     []string{"Linked List", "Math"},
			},
		},
	}

	// Write JSON to temp file.
	dir := t.TempDir()
	path := filepath.Join(dir, "test_list.json")
	data, err := json.Marshal(listData)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Load the curated list.
	err = db.LoadCuratedList(path)
	if err != nil {
		t.Fatalf("load curated list: %v", err)
	}

	// Verify the list was loaded.
	list, err := db.GetCuratedList("Test List")
	if err != nil {
		t.Fatalf("get curated list: %v", err)
	}
	if list == nil {
		t.Fatal("expected list, got nil")
	}
	if list.List.Name != "Test List" {
		t.Errorf("name = %q, want %q", list.List.Name, "Test List")
	}
	if list.List.Description != "A test curated list" {
		t.Errorf("description = %q, want %q", list.List.Description, "A test curated list")
	}
	if len(list.ProblemSlugs) != 2 {
		t.Fatalf("problem slugs length = %d, want 2", len(list.ProblemSlugs))
	}
	if list.ProblemSlugs[0] != "two-sum" {
		t.Errorf("first problem = %q, want %q", list.ProblemSlugs[0], "two-sum")
	}
	if list.ProblemSlugs[1] != "add-two-numbers" {
		t.Errorf("second problem = %q, want %q", list.ProblemSlugs[1], "add-two-numbers")
	}

	// Verify problems were also saved to the problems table.
	problem, err := db.GetProblem("two-sum")
	if err != nil {
		t.Fatalf("get problem: %v", err)
	}
	if problem == nil {
		t.Fatal("expected problem, got nil")
	}
	if problem.Title != "Two Sum" {
		t.Errorf("problem title = %q, want %q", problem.Title, "Two Sum")
	}
}

func TestLoadCuratedListUpsert(t *testing.T) {
	db := testDB(t)

	// Create and load initial list.
	listData := curatedListData{
		Name:        "Test List",
		Description: "Original description",
		Problems: []struct {
			Slug       string   `json:"slug"`
			Title      string   `json:"title"`
			Difficulty string   `json:"difficulty"`
			Topics     []string `json:"topics"`
		}{
			{
				Slug:       "two-sum",
				Title:      "Two Sum",
				Difficulty: "Easy",
				Topics:     []string{"Array"},
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test_list.json")
	data, _ := json.Marshal(listData)
	os.WriteFile(path, data, 0644)
	db.LoadCuratedList(path)

	// Update with new data.
	listData.Description = "Updated description"
	listData.Problems = append(listData.Problems, struct {
		Slug       string   `json:"slug"`
		Title      string   `json:"title"`
		Difficulty string   `json:"difficulty"`
		Topics     []string `json:"topics"`
	}{
		Slug:       "add-two-numbers",
		Title:      "Add Two Numbers",
		Difficulty: "Medium",
		Topics:     []string{"Linked List"},
	})

	data, _ = json.Marshal(listData)
	os.WriteFile(path, data, 0644)
	err := db.LoadCuratedList(path)
	if err != nil {
		t.Fatalf("reload curated list: %v", err)
	}

	// Verify the list was updated.
	list, err := db.GetCuratedList("Test List")
	if err != nil {
		t.Fatalf("get curated list: %v", err)
	}
	if list.List.Description != "Updated description" {
		t.Errorf("description = %q, want %q", list.List.Description, "Updated description")
	}
	if len(list.ProblemSlugs) != 2 {
		t.Fatalf("problem slugs length = %d, want 2", len(list.ProblemSlugs))
	}
}

func TestGetCuratedList_NotFound(t *testing.T) {
	db := testDB(t)

	list, err := db.GetCuratedList("Non-Existent List")
	if err != nil {
		t.Fatalf("get curated list: %v", err)
	}
	if list != nil {
		t.Errorf("expected nil list for non-existent name, got %+v", list)
	}
}

func TestGetUserProgressOnList(t *testing.T) {
	db := testDB(t)

	// Create a user.
	user, _ := db.CreateUser(12345, "alice")

	// Create and load a curated list.
	listData := curatedListData{
		Name:        "Blind 75",
		Description: "Curated list of 75 problems",
		Problems: []struct {
			Slug       string   `json:"slug"`
			Title      string   `json:"title"`
			Difficulty string   `json:"difficulty"`
			Topics     []string `json:"topics"`
		}{
			{
				Slug:       "two-sum",
				Title:      "Two Sum",
				Difficulty: "Easy",
				Topics:     []string{"Array"},
			},
			{
				Slug:       "add-two-numbers",
				Title:      "Add Two Numbers",
				Difficulty: "Medium",
				Topics:     []string{"Linked List"},
			},
			{
				Slug:       "longest-substring",
				Title:      "Longest Substring",
				Difficulty: "Medium",
				Topics:     []string{"String"},
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "blind75.json")
	data, _ := json.Marshal(listData)
	os.WriteFile(path, data, 0644)
	db.LoadCuratedList(path)

	// Mark two problems as solved.
	db.SaveUserSolvedProblem(user.ID, "two-sum")
	db.SaveUserSolvedProblem(user.ID, "longest-substring")

	// Get progress on the list.
	progress, err := db.GetUserProgressOnList(user.ID, "Blind 75")
	if err != nil {
		t.Fatalf("get user progress on list: %v", err)
	}
	if progress == nil {
		t.Fatal("expected progress, got nil")
	}
	if progress.ListName != "Blind 75" {
		t.Errorf("list name = %q, want %q", progress.ListName, "Blind 75")
	}
	if progress.TotalCount != 3 {
		t.Errorf("total count = %d, want 3", progress.TotalCount)
	}
	if progress.SolvedCount != 2 {
		t.Errorf("solved count = %d, want 2", progress.SolvedCount)
	}
	// Percentage should be approximately 66.67%
	if progress.Percentage < 66.0 || progress.Percentage > 67.0 {
		t.Errorf("percentage = %f, want ~66.67", progress.Percentage)
	}
	if len(progress.SolvedSlugs) != 2 {
		t.Fatalf("solved slugs length = %d, want 2", len(progress.SolvedSlugs))
	}

	// Verify solved slugs contain the right problems.
	solvedSet := make(map[string]bool)
	for _, slug := range progress.SolvedSlugs {
		solvedSet[slug] = true
	}
	if !solvedSet["two-sum"] || !solvedSet["longest-substring"] {
		t.Errorf("solved slugs = %v, want [two-sum, longest-substring]", progress.SolvedSlugs)
	}
}

func TestGetUserProgressOnList_EmptyList(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	// Create an empty curated list.
	listData := curatedListData{
		Name:        "Empty List",
		Description: "A list with no problems",
		Problems:    []struct {
			Slug       string   `json:"slug"`
			Title      string   `json:"title"`
			Difficulty string   `json:"difficulty"`
			Topics     []string `json:"topics"`
		}{},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	data, _ := json.Marshal(listData)
	os.WriteFile(path, data, 0644)
	db.LoadCuratedList(path)

	progress, err := db.GetUserProgressOnList(user.ID, "Empty List")
	if err != nil {
		t.Fatalf("get user progress on empty list: %v", err)
	}
	if progress == nil {
		t.Fatal("expected progress, got nil")
	}
	if progress.TotalCount != 0 {
		t.Errorf("total count = %d, want 0", progress.TotalCount)
	}
	if progress.SolvedCount != 0 {
		t.Errorf("solved count = %d, want 0", progress.SolvedCount)
	}
	if progress.Percentage != 0 {
		t.Errorf("percentage = %f, want 0", progress.Percentage)
	}
}

func TestGetUserProgressOnList_NotFound(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	progress, err := db.GetUserProgressOnList(user.ID, "Non-Existent List")
	if err != nil {
		t.Fatalf("get user progress on list: %v", err)
	}
	if progress != nil {
		t.Errorf("expected nil progress for non-existent list, got %+v", progress)
	}
}

func TestGetUserProgressOnList_NoSolved(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	// Create a list but don't solve any problems.
	listData := curatedListData{
		Name:        "Unsolved List",
		Description: "A list with no progress",
		Problems: []struct {
			Slug       string   `json:"slug"`
			Title      string   `json:"title"`
			Difficulty string   `json:"difficulty"`
			Topics     []string `json:"topics"`
		}{
			{
				Slug:       "two-sum",
				Title:      "Two Sum",
				Difficulty: "Easy",
				Topics:     []string{"Array"},
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "unsolved.json")
	data, _ := json.Marshal(listData)
	os.WriteFile(path, data, 0644)
	db.LoadCuratedList(path)

	progress, err := db.GetUserProgressOnList(user.ID, "Unsolved List")
	if err != nil {
		t.Fatalf("get user progress on list: %v", err)
	}
	if progress == nil {
		t.Fatal("expected progress, got nil")
	}
	if progress.SolvedCount != 0 {
		t.Errorf("solved count = %d, want 0", progress.SolvedCount)
	}
	if progress.Percentage != 0 {
		t.Errorf("percentage = %f, want 0", progress.Percentage)
	}
	if len(progress.SolvedSlugs) != 0 {
		t.Errorf("expected empty solved slugs, got %v", progress.SolvedSlugs)
	}
}
