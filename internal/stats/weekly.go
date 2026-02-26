package stats

import (
	"context"
	"fmt"
	"time"
)

// DailyStats holds the statistics for a single day.
type DailyStats struct {
	Date   time.Time // Date of the snapshot
	Solved int       // Number of problems solved on this day
}

// WeeklyStats holds the weekly statistics computed from daily snapshots.
type WeeklyStats struct {
	TotalThisWeek int          // Total problems solved in the past 7 days
	DailyBreakdown []DailyStats // Daily breakdown ordered by date
	Trend         string       // Trend indicator: "increasing", "decreasing", or "stable"
}

// ComputeWeeklyStats computes weekly statistics for the user based on the past
// 7 days of snapshots. It returns the total problems solved this week, a daily
// breakdown showing how many problems were solved each day, and a trend
// indicator showing whether the solving rate is increasing or decreasing.
func (c *Collector) ComputeWeeklyStats(ctx context.Context, userID int64) (*WeeklyStats, error) {
	// Fetch the last 7 days of snapshots.
	snapshots, err := c.store.GetSnapshotHistory(userID, 7)
	if err != nil {
		return nil, fmt.Errorf("get snapshot history: %w", err)
	}

	// If we have fewer than 2 snapshots, we can't compute deltas.
	if len(snapshots) < 2 {
		// Return empty stats if insufficient data.
		return &WeeklyStats{
			TotalThisWeek:  0,
			DailyBreakdown: []DailyStats{},
			Trend:          "stable",
		}, nil
	}

	// Compute daily deltas by comparing consecutive snapshots.
	var dailyBreakdown []DailyStats
	totalThisWeek := 0

	for i := 1; i < len(snapshots); i++ {
		prev := snapshots[i-1]
		curr := snapshots[i]

		// Delta is the difference in total solved between consecutive days.
		delta := curr.TotalSolved - prev.TotalSolved
		if delta < 0 {
			// This shouldn't happen in normal operation, but handle it gracefully.
			delta = 0
		}

		dailyBreakdown = append(dailyBreakdown, DailyStats{
			Date:   curr.SnapshotDate,
			Solved: delta,
		})

		totalThisWeek += delta
	}

	// Compute trend by comparing first half vs second half of the week.
	trend := computeTrend(dailyBreakdown)

	return &WeeklyStats{
		TotalThisWeek:  totalThisWeek,
		DailyBreakdown: dailyBreakdown,
		Trend:          trend,
	}, nil
}

// computeTrend determines the trend based on the daily breakdown.
// It compares the average of the first half of the week to the second half.
func computeTrend(dailyBreakdown []DailyStats) string {
	if len(dailyBreakdown) == 0 {
		return "stable"
	}

	// Split the week into two halves and compare averages.
	mid := len(dailyBreakdown) / 2
	if mid == 0 {
		return "stable"
	}

	firstHalf := dailyBreakdown[:mid]
	secondHalf := dailyBreakdown[mid:]

	firstAvg := 0.0
	for _, day := range firstHalf {
		firstAvg += float64(day.Solved)
	}
	firstAvg /= float64(len(firstHalf))

	secondAvg := 0.0
	for _, day := range secondHalf {
		secondAvg += float64(day.Solved)
	}
	secondAvg /= float64(len(secondHalf))

	// Use a threshold to determine if the trend is significant.
	const threshold = 0.2 // 20% difference is considered significant

	if secondAvg > firstAvg*(1+threshold) {
		return "increasing"
	} else if secondAvg < firstAvg*(1-threshold) {
		return "decreasing"
	}

	return "stable"
}
