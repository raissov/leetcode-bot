package stats

import (
	"context"
	"testing"
	"time"
)

func TestWeeklyStats(t *testing.T) {
	db := testDB(t)

	// Create a test user.
	user, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create snapshots for the past 7 days with varying solve counts.
	// We'll simulate a user solving problems at different rates each day.
	baseDate := time.Now().UTC().Truncate(24 * time.Hour)

	snapshots := []struct {
		daysAgo     int
		totalSolved int
	}{
		{7, 100}, // 7 days ago: 100 total
		{6, 102}, // 6 days ago: 102 total (+2 on this day)
		{5, 105}, // 5 days ago: 105 total (+3 on this day)
		{4, 107}, // 4 days ago: 107 total (+2 on this day)
		{3, 111}, // 3 days ago: 111 total (+4 on this day)
		{2, 116}, // 2 days ago: 116 total (+5 on this day)
		{1, 122}, // 1 day ago: 122 total (+6 on this day)
		{0, 130}, // today: 130 total (+8 today)
	}

	for _, s := range snapshots {
		date := baseDate.AddDate(0, 0, -s.daysAgo)
		if err := db.SaveSnapshot(user.ID, s.totalSolved, 0, 0, 0, 0.0, 0, date); err != nil {
			t.Fatalf("save snapshot for %d days ago: %v", s.daysAgo, err)
		}
	}

	// Create a collector to compute weekly stats.
	collector := &Collector{
		lc:    nil, // Not needed for this test
		store: db,
	}

	// Compute weekly stats.
	stats, err := collector.ComputeWeeklyStats(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("compute weekly stats: %v", err)
	}

	// Verify total this week.
	// Total should be: 2 + 3 + 2 + 4 + 5 + 6 + 8 = 30
	expectedTotal := 30
	if stats.TotalThisWeek != expectedTotal {
		t.Errorf("total this week = %d, want %d", stats.TotalThisWeek, expectedTotal)
	}

	// Verify daily breakdown has correct number of entries.
	// We have 8 snapshots, so 7 daily deltas.
	if len(stats.DailyBreakdown) != 7 {
		t.Fatalf("daily breakdown length = %d, want 7", len(stats.DailyBreakdown))
	}

	// Verify the daily breakdown values.
	expectedDaily := []int{2, 3, 2, 4, 5, 6, 8}
	for i, expected := range expectedDaily {
		if stats.DailyBreakdown[i].Solved != expected {
			t.Errorf("day %d: solved = %d, want %d", i, stats.DailyBreakdown[i].Solved, expected)
		}
	}

	// Verify trend is "increasing" since the second half of the week has higher averages.
	// First half (3 days): 2, 3, 2 (avg = 2.33)
	// Second half (4 days): 4, 5, 6, 8 (avg = 5.75)
	// 5.75 > 2.33 * 1.2, so trend should be "increasing"
	if stats.Trend != "increasing" {
		t.Errorf("trend = %q, want %q", stats.Trend, "increasing")
	}
}

func TestWeeklyStats_NoSnapshots(t *testing.T) {
	db := testDB(t)

	// Create a test user with no snapshots.
	user, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	collector := &Collector{
		lc:    nil,
		store: db,
	}

	// Compute weekly stats.
	stats, err := collector.ComputeWeeklyStats(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("compute weekly stats: %v", err)
	}

	// Should return empty stats.
	if stats.TotalThisWeek != 0 {
		t.Errorf("total this week = %d, want 0", stats.TotalThisWeek)
	}
	if len(stats.DailyBreakdown) != 0 {
		t.Errorf("daily breakdown length = %d, want 0", len(stats.DailyBreakdown))
	}
	if stats.Trend != "stable" {
		t.Errorf("trend = %q, want %q", stats.Trend, "stable")
	}
}

func TestWeeklyStats_SingleSnapshot(t *testing.T) {
	db := testDB(t)

	// Create a test user.
	user, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create only one snapshot.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if err := db.SaveSnapshot(user.ID, 100, 0, 0, 0, 0.0, 0, today); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	collector := &Collector{
		lc:    nil,
		store: db,
	}

	// Compute weekly stats.
	stats, err := collector.ComputeWeeklyStats(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("compute weekly stats: %v", err)
	}

	// With only one snapshot, we can't compute deltas.
	if stats.TotalThisWeek != 0 {
		t.Errorf("total this week = %d, want 0", stats.TotalThisWeek)
	}
	if len(stats.DailyBreakdown) != 0 {
		t.Errorf("daily breakdown length = %d, want 0", len(stats.DailyBreakdown))
	}
}

func TestWeeklyStats_StableTrend(t *testing.T) {
	db := testDB(t)

	// Create a test user.
	user, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create snapshots with consistent daily progress.
	baseDate := time.Now().UTC().Truncate(24 * time.Hour)

	snapshots := []struct {
		daysAgo     int
		totalSolved int
	}{
		{6, 100},
		{5, 103},
		{4, 106},
		{3, 109},
		{2, 112},
		{1, 115},
		{0, 118},
	}

	for _, s := range snapshots {
		date := baseDate.AddDate(0, 0, -s.daysAgo)
		if err := db.SaveSnapshot(user.ID, s.totalSolved, 0, 0, 0, 0.0, 0, date); err != nil {
			t.Fatalf("save snapshot for %d days ago: %v", s.daysAgo, err)
		}
	}

	collector := &Collector{
		lc:    nil,
		store: db,
	}

	// Compute weekly stats.
	stats, err := collector.ComputeWeeklyStats(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("compute weekly stats: %v", err)
	}

	// Trend should be stable since the user is consistently solving 3 problems per day.
	if stats.Trend != "stable" {
		t.Errorf("trend = %q, want %q", stats.Trend, "stable")
	}
}

func TestWeeklyStats_DecreasingTrend(t *testing.T) {
	db := testDB(t)

	// Create a test user.
	user, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create snapshots with decreasing daily progress.
	baseDate := time.Now().UTC().Truncate(24 * time.Hour)

	snapshots := []struct {
		daysAgo     int
		totalSolved int
	}{
		{6, 100},
		{5, 108}, // +8
		{4, 115}, // +7
		{3, 121}, // +6
		{2, 124}, // +3
		{1, 126}, // +2
		{0, 127}, // +1
	}

	for _, s := range snapshots {
		date := baseDate.AddDate(0, 0, -s.daysAgo)
		if err := db.SaveSnapshot(user.ID, s.totalSolved, 0, 0, 0, 0.0, 0, date); err != nil {
			t.Fatalf("save snapshot for %d days ago: %v", s.daysAgo, err)
		}
	}

	collector := &Collector{
		lc:    nil,
		store: db,
	}

	// Compute weekly stats.
	stats, err := collector.ComputeWeeklyStats(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("compute weekly stats: %v", err)
	}

	// Trend should be decreasing since the solving rate is going down.
	// First half: 8, 7, 6 (avg = 7)
	// Second half: 3, 2, 1 (avg = 2)
	// 2 < 7 * 0.8, so trend should be "decreasing"
	if stats.Trend != "decreasing" {
		t.Errorf("trend = %q, want %q", stats.Trend, "decreasing")
	}
}
