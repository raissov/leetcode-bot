package leetcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	// graphqlEndpoint is the LeetCode GraphQL API URL.
	graphqlEndpoint = "https://leetcode.com/graphql/"

	// refererHeader is required by LeetCode to accept requests.
	refererHeader = "https://leetcode.com"

	// httpTimeout is the maximum duration for a single HTTP request.
	httpTimeout = 15 * time.Second
)

// Client is a rate-limited, caching HTTP client for the LeetCode GraphQL API.
// It enforces at most 1 request per second and caches responses with
// configurable TTLs to minimize API load.
type Client struct {
	http        *http.Client
	rateLimiter <-chan time.Time
	cache       *Cache
}

// NewClient creates a new LeetCode GraphQL API client with rate limiting
// (1 request/second) and an in-memory response cache.
func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: httpTimeout,
		},
		rateLimiter: time.Tick(time.Second), //nolint:staticcheck // simple rate limiter
		cache:       NewCache(),
	}
}

// query executes a GraphQL request against the LeetCode API. It blocks on the
// rate limiter, marshals the request body, sends the POST, and decodes the
// response into the provided result pointer. The result must be a pointer to
// the expected "data" shape (e.g., *userProfileResponse).
func (c *Client) query(ctx context.Context, opName, queryStr string, vars map[string]any, result any) error {
	// Block until rate limiter allows the next request.
	select {
	case <-c.rateLimiter:
	case <-ctx.Done():
		return ctx.Err()
	}

	body, err := json.Marshal(graphqlRequest{
		OperationName: opName,
		Query:         queryStr,
		Variables:     vars,
	})
	if err != nil {
		return fmt.Errorf("leetcode: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("leetcode: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", refererHeader)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("leetcode: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("leetcode: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	// Decode into a wrapper that captures both data and errors.
	var raw struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphqlError  `json:"errors,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("leetcode: decode response: %w", err)
	}

	if len(raw.Errors) > 0 {
		return fmt.Errorf("leetcode: graphql error: %s", raw.Errors[0].Message)
	}

	if raw.Data == nil {
		return fmt.Errorf("leetcode: empty data in response")
	}

	if err := json.Unmarshal(raw.Data, result); err != nil {
		return fmt.Errorf("leetcode: unmarshal data: %w", err)
	}

	return nil
}

// GetUserProfile fetches the comprehensive user profile including submission
// stats by difficulty, badges, contributions, recent submissions, and the
// submission calendar. Returns nil if the user profile is null (private or
// non-existent). Results are cached for ProfileCacheTTL.
func (c *Client) GetUserProfile(ctx context.Context, username string) (*userProfileResponse, error) {
	cacheKey := "profile:" + username
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.(*userProfileResponse), nil
	}

	var data userProfileResponse
	err := c.query(ctx, "getUserProfile", QueryUserProfile, map[string]any{
		"username": username,
	}, &data)
	if err != nil {
		return nil, fmt.Errorf("get user profile: %w", err)
	}

	// A null matchedUser means the profile doesn't exist or is private.
	if data.MatchedUser == nil {
		return nil, nil
	}

	c.cache.Set(cacheKey, &data, ProfileCacheTTL)
	return &data, nil
}

// GetUserCalendar fetches the user's activity calendar for a given year,
// including streak count, total active days, and the submission calendar.
// Returns nil if the user profile is null. Results are cached for StatsCacheTTL.
func (c *Client) GetUserCalendar(ctx context.Context, username string, year int) (*UserCalendar, error) {
	cacheKey := fmt.Sprintf("calendar:%s:%d", username, year)
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.(*UserCalendar), nil
	}

	var data userCalendarResponse
	err := c.query(ctx, "UserProfileCalendar", QueryUserCalendar, map[string]any{
		"username": username,
		"year":     year,
	}, &data)
	if err != nil {
		return nil, fmt.Errorf("get user calendar: %w", err)
	}

	// A null matchedUser means the profile doesn't exist or is private.
	if data.MatchedUser == nil {
		return nil, nil
	}

	cal := &data.MatchedUser.UserCalendar
	c.cache.Set(cacheKey, cal, StatsCacheTTL)
	return cal, nil
}

// GetDailyChallenge fetches today's daily coding challenge including the
// question title, difficulty, topic tags, and link. Results are cached for
// DailyChallengeCacheTTL.
func (c *Client) GetDailyChallenge(ctx context.Context) (*DailyChallenge, error) {
	cacheKey := "daily"
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.(*DailyChallenge), nil
	}

	var data dailyChallengeResponse
	err := c.query(ctx, "questionOfToday", QueryDailyChallenge, nil, &data)
	if err != nil {
		return nil, fmt.Errorf("get daily challenge: %w", err)
	}

	challenge := &data.ActiveDailyCodingChallengeQuestion
	c.cache.Set(cacheKey, challenge, DailyChallengeCacheTTL)
	return challenge, nil
}

// GetRecentSubmissions fetches a user's recent submissions (up to 20).
// Returns an empty slice if the user profile is null. Results are cached for
// StatsCacheTTL.
func (c *Client) GetRecentSubmissions(ctx context.Context, username string) ([]RecentSubmission, error) {
	cacheKey := "submissions:" + username
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.([]RecentSubmission), nil
	}

	var data recentSubmissionsResponse
	err := c.query(ctx, "getRecentSubmissionList", QueryRecentSubmissions, map[string]any{
		"username": username,
		"limit":    20,
	}, &data)
	if err != nil {
		return nil, fmt.Errorf("get recent submissions: %w", err)
	}

	// Return empty slice instead of nil for consistency.
	if data.RecentSubmissionList == nil {
		data.RecentSubmissionList = []RecentSubmission{}
	}

	c.cache.Set(cacheKey, data.RecentSubmissionList, StatsCacheTTL)
	return data.RecentSubmissionList, nil
}

// GetUserSolvedProblems fetches all problems the user has successfully solved
// (accepted submissions). Uses a high limit (5000) to capture the complete
// history. Returns an empty slice if no solved problems are found. Results
// are cached for StatsCacheTTL.
func (c *Client) GetUserSolvedProblems(ctx context.Context, username string) ([]SolvedProblem, error) {
	cacheKey := "solved:" + username
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.([]SolvedProblem), nil
	}

	var data solvedProblemsResponse
	err := c.query(ctx, "getUserSolvedProblems", QueryUserSolvedProblems, map[string]any{
		"username": username,
		"limit":    5000,
	}, &data)
	if err != nil {
		return nil, fmt.Errorf("get user solved problems: %w", err)
	}

	// Return empty slice instead of nil for consistency.
	if data.RecentACSubmissionList == nil {
		data.RecentACSubmissionList = []SolvedProblem{}
	}

	c.cache.Set(cacheKey, data.RecentACSubmissionList, StatsCacheTTL)
	return data.RecentACSubmissionList, nil
}

// GetProblemDetails fetches comprehensive metadata for a specific problem
// including difficulty level and topic tags. Returns nil if the problem
// doesn't exist. Results are cached for ProfileCacheTTL since problem
// metadata rarely changes.
func (c *Client) GetProblemDetails(ctx context.Context, titleSlug string) (*ProblemDetails, error) {
	cacheKey := "problem:" + titleSlug
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.(*ProblemDetails), nil
	}

	var data problemDetailsResponse
	err := c.query(ctx, "getProblemDetails", QueryProblemDetails, map[string]any{
		"titleSlug": titleSlug,
	}, &data)
	if err != nil {
		return nil, fmt.Errorf("get problem details: %w", err)
	}

	// A null question means the problem doesn't exist.
	if data.Question == nil {
		return nil, nil
	}

	c.cache.Set(cacheKey, data.Question, ProfileCacheTTL)
	return data.Question, nil
}

// ParseSubmissionCalendar double-parses the submissionCalendar field from
// LeetCode. The field is a JSON-encoded string containing a JSON object that
// maps unix timestamps (in seconds, as strings) to submission counts.
// Returns a map of time.Time (truncated to day) to count.
func ParseSubmissionCalendar(raw string) (map[time.Time]int, error) {
	if raw == "" || raw == "{}" {
		return map[time.Time]int{}, nil
	}

	// The submissionCalendar field is already a JSON string (not double-encoded
	// when it comes from the matchedUser.submissionCalendar field). However,
	// when it arrives inside a JSON response it may be a quoted string that
	// needs one extra round of unquoting.
	decoded := raw
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			return nil, fmt.Errorf("leetcode: unmarshal calendar string: %w", err)
		}
	}

	// Parse the JSON object: {"timestamp": count, ...}
	var tsMap map[string]int
	if err := json.Unmarshal([]byte(decoded), &tsMap); err != nil {
		return nil, fmt.Errorf("leetcode: parse calendar data: %w", err)
	}

	result := make(map[time.Time]int, len(tsMap))
	for tsStr, count := range tsMap {
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue // skip invalid timestamps
		}
		// LeetCode timestamps are in seconds, not milliseconds.
		t := time.Unix(ts, 0).UTC().Truncate(24 * time.Hour)
		result[t] = count
	}

	return result, nil
}
