package storage

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Problems CRUD
// ---------------------------------------------------------------------------

func TestSaveAndGetProblem(t *testing.T) {
	db := testDB(t)

	err := db.SaveProblem("two-sum", "Two Sum", "Easy", []string{"Array", "Hash Table"})
	if err != nil {
		t.Fatalf("save problem: %v", err)
	}

	problem, err := db.GetProblem("two-sum")
	if err != nil {
		t.Fatalf("get problem: %v", err)
	}
	if problem == nil {
		t.Fatal("expected problem, got nil")
	}
	if problem.Slug != "two-sum" {
		t.Errorf("slug = %q, want %q", problem.Slug, "two-sum")
	}
	if problem.Title != "Two Sum" {
		t.Errorf("title = %q, want %q", problem.Title, "Two Sum")
	}
	if problem.Difficulty != "Easy" {
		t.Errorf("difficulty = %q, want %q", problem.Difficulty, "Easy")
	}
	if len(problem.Topics) != 2 {
		t.Fatalf("topics length = %d, want 2", len(problem.Topics))
	}
	if problem.Topics[0] != "Array" || problem.Topics[1] != "Hash Table" {
		t.Errorf("topics = %v, want [Array, Hash Table]", problem.Topics)
	}
}

func TestGetProblem_NotFound(t *testing.T) {
	db := testDB(t)

	problem, err := db.GetProblem("non-existent")
	if err != nil {
		t.Fatalf("get problem: %v", err)
	}
	if problem != nil {
		t.Errorf("expected nil problem for non-existent slug, got %+v", problem)
	}
}

func TestSaveProblemUpsert(t *testing.T) {
	db := testDB(t)

	// Save initial problem.
	db.SaveProblem("two-sum", "Two Sum", "Easy", []string{"Array"})

	// Upsert with updated values on the same slug.
	err := db.SaveProblem("two-sum", "Two Sum Updated", "Medium", []string{"Array", "Hash Table", "Math"})
	if err != nil {
		t.Fatalf("upsert problem: %v", err)
	}

	problem, err := db.GetProblem("two-sum")
	if err != nil {
		t.Fatalf("get problem after upsert: %v", err)
	}
	if problem.Title != "Two Sum Updated" {
		t.Errorf("title after upsert = %q, want %q", problem.Title, "Two Sum Updated")
	}
	if problem.Difficulty != "Medium" {
		t.Errorf("difficulty after upsert = %q, want %q", problem.Difficulty, "Medium")
	}
	if len(problem.Topics) != 3 {
		t.Errorf("topics length after upsert = %d, want 3", len(problem.Topics))
	}
}

func TestGetProblemsByTopics(t *testing.T) {
	db := testDB(t)

	// Create several problems with different topics.
	db.SaveProblem("two-sum", "Two Sum", "Easy", []string{"Array", "Hash Table"})
	db.SaveProblem("add-two-numbers", "Add Two Numbers", "Medium", []string{"Linked List", "Math"})
	db.SaveProblem("longest-substring", "Longest Substring", "Medium", []string{"String", "Hash Table"})
	db.SaveProblem("median-sorted-arrays", "Median of Two Sorted Arrays", "Hard", []string{"Array", "Binary Search"})

	// Search for problems with "Hash Table" or "Math" topics.
	problems, err := db.GetProblemsByTopics([]string{"hash table", "math"})
	if err != nil {
		t.Fatalf("get problems by topics: %v", err)
	}

	// Should return: two-sum, add-two-numbers, longest-substring (3 problems).
	if len(problems) != 3 {
		t.Fatalf("got %d problems, want 3", len(problems))
	}

	// Verify slugs.
	slugs := make(map[string]bool)
	for _, p := range problems {
		slugs[p.Slug] = true
	}
	for _, want := range []string{"two-sum", "add-two-numbers", "longest-substring"} {
		if !slugs[want] {
			t.Errorf("problem %q not found", want)
		}
	}
}

func TestGetProblemsByTopics_Empty(t *testing.T) {
	db := testDB(t)

	// Empty topics list should return empty result.
	problems, err := db.GetProblemsByTopics([]string{})
	if err != nil {
		t.Fatalf("get problems by topics: %v", err)
	}
	if len(problems) != 0 {
		t.Errorf("expected empty result, got %d problems", len(problems))
	}
}

func TestGetProblemsByTopics_NoMatches(t *testing.T) {
	db := testDB(t)

	db.SaveProblem("two-sum", "Two Sum", "Easy", []string{"Array", "Hash Table"})

	// Search for non-existent topic.
	problems, err := db.GetProblemsByTopics([]string{"nonexistent"})
	if err != nil {
		t.Fatalf("get problems by topics: %v", err)
	}
	if len(problems) != 0 {
		t.Errorf("expected no matches, got %d problems", len(problems))
	}
}

// ---------------------------------------------------------------------------
// User Solved Problems
// ---------------------------------------------------------------------------

func TestSaveAndGetUserSolvedProblems(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	// Save solved problems.
	err := db.SaveUserSolvedProblem(user.ID, "two-sum")
	if err != nil {
		t.Fatalf("save user solved problem: %v", err)
	}

	err = db.SaveUserSolvedProblem(user.ID, "add-two-numbers")
	if err != nil {
		t.Fatalf("save user solved problem: %v", err)
	}

	// Retrieve solved problems.
	solved, err := db.GetUserSolvedProblems(user.ID)
	if err != nil {
		t.Fatalf("get user solved problems: %v", err)
	}

	if len(solved) != 2 {
		t.Fatalf("got %d solved problems, want 2", len(solved))
	}

	// Verify slugs (order is DESC by solved_at, so newest first).
	if solved[0].ProblemSlug != "add-two-numbers" {
		t.Errorf("first solved problem = %q, want %q", solved[0].ProblemSlug, "add-two-numbers")
	}
	if solved[1].ProblemSlug != "two-sum" {
		t.Errorf("second solved problem = %q, want %q", solved[1].ProblemSlug, "two-sum")
	}
}

func TestSaveUserSolvedProblemDuplicate(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	// Save the same problem twice.
	db.SaveUserSolvedProblem(user.ID, "two-sum")
	err := db.SaveUserSolvedProblem(user.ID, "two-sum")
	if err != nil {
		t.Fatalf("save duplicate user solved problem: %v", err)
	}

	// Should only have one entry due to INSERT OR IGNORE.
	solved, err := db.GetUserSolvedProblems(user.ID)
	if err != nil {
		t.Fatalf("get user solved problems: %v", err)
	}
	if len(solved) != 1 {
		t.Fatalf("got %d solved problems, want 1", len(solved))
	}
}

func TestGetUserSolvedProblems_Empty(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	solved, err := db.GetUserSolvedProblems(user.ID)
	if err != nil {
		t.Fatalf("get user solved problems: %v", err)
	}
	if len(solved) != 0 {
		t.Errorf("expected empty solved problems, got %d", len(solved))
	}
}
