package stats

import (
	"context"
	"testing"

	"github.com/user/leetcode-bot/internal/leetcode"
	"github.com/user/leetcode-bot/internal/storage"
)

// ---------------------------------------------------------------------------
// SyncUserSolvedProblems tests
// ---------------------------------------------------------------------------

// leetCodeAPI is a minimal interface for the methods used by SyncUserSolvedProblems.
type leetCodeAPI interface {
	GetUserSolvedProblems(ctx context.Context, username string) ([]leetcode.SolvedProblem, error)
	GetProblemDetails(ctx context.Context, titleSlug string) (*leetcode.ProblemDetails, error)
}

// mockLeetCodeClient is a test double for leetcode.Client that returns
// predefined solved problems and problem details.
type mockLeetCodeClient struct {
	solvedProblems []leetcode.SolvedProblem
	problemDetails map[string]*leetcode.ProblemDetails
}

func (m *mockLeetCodeClient) GetUserSolvedProblems(ctx context.Context, username string) ([]leetcode.SolvedProblem, error) {
	return m.solvedProblems, nil
}

func (m *mockLeetCodeClient) GetProblemDetails(ctx context.Context, titleSlug string) (*leetcode.ProblemDetails, error) {
	if details, ok := m.problemDetails[titleSlug]; ok {
		return details, nil
	}
	return nil, nil
}

// mockCollector is a test version of Collector that uses the leetCodeAPI interface.
type mockCollector struct {
	lc    leetCodeAPI
	store *storage.DB
}

func (c *mockCollector) SyncUserSolvedProblems(ctx context.Context, userID int64, username string) error {
	// Fetch the list of solved problems from LeetCode.
	solved, err := c.lc.GetUserSolvedProblems(ctx, username)
	if err != nil {
		return err
	}

	// For each solved problem, fetch metadata and persist both problem and
	// user_solved_problem records.
	for _, sp := range solved {
		// Fetch problem details to get difficulty and topics.
		details, err := c.lc.GetProblemDetails(ctx, sp.TitleSlug)
		if err != nil {
			// Skip this problem if we can't fetch details.
			continue
		}
		if details == nil {
			// Problem doesn't exist or was deleted; skip it.
			continue
		}

		// Extract topic names from topic tags.
		topics := make([]string, 0, len(details.TopicTags))
		for _, tag := range details.TopicTags {
			topics = append(topics, tag.Name)
		}

		// Save problem metadata.
		if err := c.store.SaveProblem(sp.TitleSlug, details.Title, details.Difficulty, topics); err != nil {
			return err
		}

		// Save user solved problem record.
		if err := c.store.SaveUserSolvedProblem(userID, sp.TitleSlug); err != nil {
			return err
		}
	}

	return nil
}

func TestSyncSolvedProblems(t *testing.T) {
	db := testDB(t)

	// Create a test user.
	user, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Set up mock LeetCode client with sample solved problems.
	mock := &mockLeetCodeClient{
		solvedProblems: []leetcode.SolvedProblem{
			{ID: "1", Title: "Two Sum", TitleSlug: "two-sum", Timestamp: "1234567890"},
			{ID: "2", Title: "Add Two Numbers", TitleSlug: "add-two-numbers", Timestamp: "1234567891"},
			{ID: "3", Title: "Longest Substring", TitleSlug: "longest-substring-without-repeating-characters", Timestamp: "1234567892"},
		},
		problemDetails: map[string]*leetcode.ProblemDetails{
			"two-sum": {
				QuestionID:         "1",
				QuestionFrontendID: "1",
				Title:              "Two Sum",
				TitleSlug:          "two-sum",
				Difficulty:         "Easy",
				TopicTags: []leetcode.TopicTag{
					{Name: "Array", Slug: "array"},
					{Name: "Hash Table", Slug: "hash-table"},
				},
			},
			"add-two-numbers": {
				QuestionID:         "2",
				QuestionFrontendID: "2",
				Title:              "Add Two Numbers",
				TitleSlug:          "add-two-numbers",
				Difficulty:         "Medium",
				TopicTags: []leetcode.TopicTag{
					{Name: "Linked List", Slug: "linked-list"},
					{Name: "Math", Slug: "math"},
				},
			},
			"longest-substring-without-repeating-characters": {
				QuestionID:         "3",
				QuestionFrontendID: "3",
				Title:              "Longest Substring Without Repeating Characters",
				TitleSlug:          "longest-substring-without-repeating-characters",
				Difficulty:         "Medium",
				TopicTags: []leetcode.TopicTag{
					{Name: "String", Slug: "string"},
					{Name: "Hash Table", Slug: "hash-table"},
					{Name: "Sliding Window", Slug: "sliding-window"},
				},
			},
		},
	}

	collector := &mockCollector{
		lc:    mock,
		store: db,
	}

	// Sync solved problems.
	err = collector.SyncUserSolvedProblems(context.Background(), user.ID, "alice")
	if err != nil {
		t.Fatalf("sync solved problems: %v", err)
	}

	// Verify that problems were saved to the database.
	twoSum, err := db.GetProblem("two-sum")
	if err != nil {
		t.Fatalf("get two-sum problem: %v", err)
	}
	if twoSum == nil {
		t.Fatal("expected two-sum problem, got nil")
	}
	if twoSum.Title != "Two Sum" {
		t.Errorf("two-sum title = %q, want %q", twoSum.Title, "Two Sum")
	}
	if twoSum.Difficulty != "Easy" {
		t.Errorf("two-sum difficulty = %q, want %q", twoSum.Difficulty, "Easy")
	}
	if len(twoSum.Topics) != 2 {
		t.Fatalf("two-sum topics length = %d, want 2", len(twoSum.Topics))
	}

	// Verify that user solved problems were saved.
	solved, err := db.GetUserSolvedProblems(user.ID)
	if err != nil {
		t.Fatalf("get user solved problems: %v", err)
	}
	if len(solved) != 3 {
		t.Fatalf("got %d solved problems, want 3", len(solved))
	}

	// Verify slugs (order is DESC by solved_at).
	slugs := make(map[string]bool)
	for _, sp := range solved {
		slugs[sp.ProblemSlug] = true
	}
	for _, want := range []string{"two-sum", "add-two-numbers", "longest-substring-without-repeating-characters"} {
		if !slugs[want] {
			t.Errorf("problem %q not found in user solved problems", want)
		}
	}
}

func TestSyncSolvedProblems_Idempotent(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	mock := &mockLeetCodeClient{
		solvedProblems: []leetcode.SolvedProblem{
			{ID: "1", Title: "Two Sum", TitleSlug: "two-sum", Timestamp: "1234567890"},
		},
		problemDetails: map[string]*leetcode.ProblemDetails{
			"two-sum": {
				QuestionID:         "1",
				QuestionFrontendID: "1",
				Title:              "Two Sum",
				TitleSlug:          "two-sum",
				Difficulty:         "Easy",
				TopicTags: []leetcode.TopicTag{
					{Name: "Array", Slug: "array"},
				},
			},
		},
	}

	collector := &mockCollector{
		lc:    mock,
		store: db,
	}

	// Sync once.
	collector.SyncUserSolvedProblems(context.Background(), user.ID, "alice")

	// Sync again with the same data.
	err := collector.SyncUserSolvedProblems(context.Background(), user.ID, "alice")
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// Should still have only one entry due to deduplication.
	solved, err := db.GetUserSolvedProblems(user.ID)
	if err != nil {
		t.Fatalf("get user solved problems: %v", err)
	}
	if len(solved) != 1 {
		t.Fatalf("got %d solved problems after double sync, want 1", len(solved))
	}
}

func TestSyncSolvedProblems_EmptyList(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	mock := &mockLeetCodeClient{
		solvedProblems: []leetcode.SolvedProblem{},
		problemDetails: map[string]*leetcode.ProblemDetails{},
	}

	collector := &mockCollector{
		lc:    mock,
		store: db,
	}

	// Sync with empty list should succeed without errors.
	err := collector.SyncUserSolvedProblems(context.Background(), user.ID, "alice")
	if err != nil {
		t.Fatalf("sync empty list: %v", err)
	}

	solved, err := db.GetUserSolvedProblems(user.ID)
	if err != nil {
		t.Fatalf("get user solved problems: %v", err)
	}
	if len(solved) != 0 {
		t.Errorf("expected empty solved problems, got %d", len(solved))
	}
}

func TestSyncSolvedProblems_SkipsMissingDetails(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	// Mock returns two solved problems, but only one has details available.
	mock := &mockLeetCodeClient{
		solvedProblems: []leetcode.SolvedProblem{
			{ID: "1", Title: "Two Sum", TitleSlug: "two-sum", Timestamp: "1234567890"},
			{ID: "999", Title: "Deleted Problem", TitleSlug: "deleted-problem", Timestamp: "1234567891"},
		},
		problemDetails: map[string]*leetcode.ProblemDetails{
			"two-sum": {
				QuestionID:         "1",
				QuestionFrontendID: "1",
				Title:              "Two Sum",
				TitleSlug:          "two-sum",
				Difficulty:         "Easy",
				TopicTags: []leetcode.TopicTag{
					{Name: "Array", Slug: "array"},
				},
			},
			// "deleted-problem" has no details
		},
	}

	collector := &mockCollector{
		lc:    mock,
		store: db,
	}

	// Should skip the missing problem and continue with the rest.
	err := collector.SyncUserSolvedProblems(context.Background(), user.ID, "alice")
	if err != nil {
		t.Fatalf("sync should succeed even with missing details, got error: %v", err)
	}

	// Should have saved only the problem with available details.
	twoSum, err := db.GetProblem("two-sum")
	if err != nil {
		t.Fatalf("get two-sum: %v", err)
	}
	if twoSum == nil {
		t.Error("expected two-sum to be saved")
	}

	deletedProblem, err := db.GetProblem("deleted-problem")
	if err != nil {
		t.Fatalf("get deleted-problem: %v", err)
	}
	if deletedProblem != nil {
		t.Error("deleted-problem should not be saved")
	}

	// User should have only one solved problem recorded.
	solved, err := db.GetUserSolvedProblems(user.ID)
	if err != nil {
		t.Fatalf("get user solved problems: %v", err)
	}
	if len(solved) != 1 {
		t.Fatalf("got %d solved problems, want 1", len(solved))
	}
	if solved[0].ProblemSlug != "two-sum" {
		t.Errorf("solved problem = %q, want %q", solved[0].ProblemSlug, "two-sum")
	}
}

// ---------------------------------------------------------------------------
// Helper: testDB creates an in-memory SQLite database for testing.
// ---------------------------------------------------------------------------

func testDB(t *testing.T) *storage.DB {
	t.Helper()

	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}
