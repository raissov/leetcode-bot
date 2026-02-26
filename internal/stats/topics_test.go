package stats

import (
	"context"
	"math"
	"testing"
)

func TestTopicStats(t *testing.T) {
	db := testDB(t)

	// Create a test user.
	user, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create sample problems with various topics.
	problems := []struct {
		slug       string
		title      string
		difficulty string
		topics     []string
	}{
		{"two-sum", "Two Sum", "Easy", []string{"Array", "Hash Table"}},
		{"add-two-numbers", "Add Two Numbers", "Medium", []string{"Linked List", "Math"}},
		{"longest-substring", "Longest Substring", "Medium", []string{"String", "Hash Table", "Sliding Window"}},
		{"median-sorted-arrays", "Median of Two Sorted Arrays", "Hard", []string{"Array", "Binary Search", "Divide and Conquer"}},
		{"reverse-integer", "Reverse Integer", "Medium", []string{"Math"}},
		{"palindrome-number", "Palindrome Number", "Easy", []string{"Math"}},
	}

	// Save all problems to the database.
	for _, p := range problems {
		if err := db.SaveProblem(p.slug, p.title, p.difficulty, p.topics); err != nil {
			t.Fatalf("save problem %q: %v", p.slug, err)
		}
	}

	// Mark some problems as solved by the user.
	// User has solved: two-sum, add-two-numbers, longest-substring
	solvedSlugs := []string{"two-sum", "add-two-numbers", "longest-substring"}
	for _, slug := range solvedSlugs {
		if err := db.SaveUserSolvedProblem(user.ID, slug); err != nil {
			t.Fatalf("save user solved problem %q: %v", slug, err)
		}
	}

	// Create a collector to compute topic stats.
	collector := &Collector{
		lc:    nil, // Not needed for this test
		store: db,
	}

	// Compute topic coverage.
	stats, err := collector.ComputeTopicCoverage(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("compute topic coverage: %v", err)
	}

	// Verify the results.
	// Expected topic counts:
	// - Array: solved 1 (two-sum) out of 2 total (two-sum, median-sorted-arrays) = 50%
	// - Hash Table: solved 2 (two-sum, longest-substring) out of 2 total = 100%
	// - Linked List: solved 1 (add-two-numbers) out of 1 total = 100%
	// - Math: solved 1 (add-two-numbers) out of 3 total (add-two-numbers, reverse-integer, palindrome-number) = 33.33%
	// - String: solved 1 (longest-substring) out of 1 total = 100%
	// - Sliding Window: solved 1 (longest-substring) out of 1 total = 100%

	expectedStats := map[string]struct {
		solved int
		total  int
	}{
		"Array":          {solved: 1, total: 2},
		"Hash Table":     {solved: 2, total: 2},
		"Linked List":    {solved: 1, total: 1},
		"Math":           {solved: 1, total: 3},
		"String":         {solved: 1, total: 1},
		"Sliding Window": {solved: 1, total: 1},
	}

	if len(stats) != len(expectedStats) {
		t.Fatalf("got %d topics, want %d", len(stats), len(expectedStats))
	}

	for topic, expected := range expectedStats {
		stat, ok := stats[topic]
		if !ok {
			t.Errorf("topic %q not found in stats", topic)
			continue
		}

		expectedPercentage := float64(expected.solved) / float64(expected.total) * 100.0

		if stat.Topic != topic {
			t.Errorf("topic name = %q, want %q", stat.Topic, topic)
		}
		if stat.Solved != expected.solved {
			t.Errorf("topic %q: solved = %d, want %d", topic, stat.Solved, expected.solved)
		}
		if stat.Total != expected.total {
			t.Errorf("topic %q: total = %d, want %d", topic, stat.Total, expected.total)
		}
		// Use approximate comparison for floating point values.
		if math.Abs(stat.Percentage-expectedPercentage) > 0.0001 {
			t.Errorf("topic %q: percentage = %f, want %f", topic, stat.Percentage, expectedPercentage)
		}
	}
}

func TestTopicStats_NoSolvedProblems(t *testing.T) {
	db := testDB(t)

	// Create a test user with no solved problems.
	user, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create a collector.
	collector := &Collector{
		lc:    nil,
		store: db,
	}

	// Compute topic coverage.
	stats, err := collector.ComputeTopicCoverage(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("compute topic coverage: %v", err)
	}

	// Should return an empty map since user hasn't solved any problems.
	if len(stats) != 0 {
		t.Errorf("got %d topics, want 0 for user with no solved problems", len(stats))
	}
}

func TestTopicStats_PartialCoverage(t *testing.T) {
	db := testDB(t)

	// Create a test user.
	user, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create problems where user has only solved some problems in a topic.
	problems := []struct {
		slug       string
		title      string
		difficulty string
		topics     []string
	}{
		{"problem-1", "Problem 1", "Easy", []string{"Dynamic Programming"}},
		{"problem-2", "Problem 2", "Medium", []string{"Dynamic Programming"}},
		{"problem-3", "Problem 3", "Hard", []string{"Dynamic Programming"}},
		{"problem-4", "Problem 4", "Easy", []string{"Dynamic Programming"}},
	}

	for _, p := range problems {
		if err := db.SaveProblem(p.slug, p.title, p.difficulty, p.topics); err != nil {
			t.Fatalf("save problem %q: %v", p.slug, err)
		}
	}

	// User has solved 2 out of 4 Dynamic Programming problems.
	if err := db.SaveUserSolvedProblem(user.ID, "problem-1"); err != nil {
		t.Fatalf("save user solved problem: %v", err)
	}
	if err := db.SaveUserSolvedProblem(user.ID, "problem-3"); err != nil {
		t.Fatalf("save user solved problem: %v", err)
	}

	collector := &Collector{
		lc:    nil,
		store: db,
	}

	stats, err := collector.ComputeTopicCoverage(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("compute topic coverage: %v", err)
	}

	// Should have exactly 1 topic.
	if len(stats) != 1 {
		t.Fatalf("got %d topics, want 1", len(stats))
	}

	dpStats, ok := stats["Dynamic Programming"]
	if !ok {
		t.Fatal("Dynamic Programming topic not found")
	}

	if dpStats.Solved != 2 {
		t.Errorf("solved = %d, want 2", dpStats.Solved)
	}
	if dpStats.Total != 4 {
		t.Errorf("total = %d, want 4", dpStats.Total)
	}
	if dpStats.Percentage != 50.0 {
		t.Errorf("percentage = %f, want 50.0", dpStats.Percentage)
	}
}
