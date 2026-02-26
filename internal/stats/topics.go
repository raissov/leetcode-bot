package stats

import (
	"context"
	"fmt"
	"strings"
)

// TopicStats holds statistics for a single topic.
type TopicStats struct {
	Topic      string  // Topic name
	Solved     int     // Number of problems solved with this topic
	Total      int     // Total number of problems with this topic
	Percentage float64 // Solved/Total * 100
}

// ComputeTopicCoverage computes statistics for each topic based on the user's
// solved problems. It returns a map of topic name to TopicStats, showing how
// many problems the user has solved per topic and the total available.
func (c *Collector) ComputeTopicCoverage(ctx context.Context, userID int64) (map[string]*TopicStats, error) {
	// Get all problems the user has solved.
	userSolved, err := c.store.GetUserSolvedProblems(userID)
	if err != nil {
		return nil, fmt.Errorf("get user solved problems: %w", err)
	}

	// Track which problem slugs the user has solved.
	solvedSlugs := make(map[string]bool)
	for _, sp := range userSolved {
		solvedSlugs[sp.ProblemSlug] = true
	}

	// Get all problems with their topics to compute totals and solved counts.
	// We need to iterate through all problems in the database to count totals.
	// For efficiency, we could add a GetAllProblems() method, but for now we'll
	// query problems by getting solved problem details.

	// First, get all problem details for solved problems to build topic counts.
	topicSolved := make(map[string]int)

	for slug := range solvedSlugs {
		problem, err := c.store.GetProblem(slug)
		if err != nil {
			return nil, fmt.Errorf("get problem %q: %w", slug, err)
		}
		if problem == nil {
			// Problem not found in DB (shouldn't happen if sync worked correctly).
			continue
		}

		// Increment solved count for each topic.
		for _, topic := range problem.Topics {
			topicSolved[topic]++
		}
	}

	// Now we need to compute totals per topic. To do this efficiently, we need
	// a method to get all problems or at least all topics with counts. Since
	// we don't have that yet, we'll use a workaround: query all unique topics
	// from the solved problems and count them.
	//
	// A more complete solution would be to add a GetAllProblems() method to
	// storage.DB, but for now we'll work with what we have.

	// For now, we'll compute stats only for topics the user has encountered.
	// This is a reasonable starting point, but ideally we'd show all topics
	// even if the user hasn't solved any problems in them yet.

	// Build the result map.
	result := make(map[string]*TopicStats)

	for topic, solvedCount := range topicSolved {
		// To get the total count of problems with this topic, we need to
		// query the database. We can use GetProblemsByTopics for this.
		// Note: GetProblemsByTopics uses LOWER() comparison, so we need to
		// pass the topic in lowercase.
		problems, err := c.store.GetProblemsByTopics([]string{strings.ToLower(topic)})
		if err != nil {
			return nil, fmt.Errorf("get problems for topic %q: %w", topic, err)
		}

		totalCount := len(problems)
		percentage := 0.0
		if totalCount > 0 {
			percentage = float64(solvedCount) / float64(totalCount) * 100.0
		}

		result[topic] = &TopicStats{
			Topic:      topic,
			Solved:     solvedCount,
			Total:      totalCount,
			Percentage: percentage,
		}
	}

	return result, nil
}
