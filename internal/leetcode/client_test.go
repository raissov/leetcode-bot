package leetcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Cache tests
// ---------------------------------------------------------------------------

func TestCacheSetAndGet(t *testing.T) {
	c := NewCache()

	c.Set("key1", "value1", 5*time.Minute)

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if val != "value1" {
		t.Errorf("value = %v, want %q", val, "value1")
	}
}

func TestCacheGetMiss(t *testing.T) {
	c := NewCache()

	val, ok := c.Get("nonexistent")
	if ok {
		t.Errorf("expected cache miss, got hit with value %v", val)
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	c := NewCache()

	// Set with very short TTL that expires immediately.
	c.Set("ephemeral", "gone", 1*time.Nanosecond)
	time.Sleep(5 * time.Millisecond)

	val, ok := c.Get("ephemeral")
	if ok {
		t.Errorf("expected cache miss after TTL expiry, got hit with value %v", val)
	}
}

func TestCacheOverwrite(t *testing.T) {
	c := NewCache()

	c.Set("key", "v1", 5*time.Minute)
	c.Set("key", "v2", 5*time.Minute)

	val, ok := c.Get("key")
	if !ok {
		t.Fatal("expected cache hit after overwrite")
	}
	if val != "v2" {
		t.Errorf("value = %v, want %q", val, "v2")
	}
}

func TestCacheThreadSafety(t *testing.T) {
	c := NewCache()

	var wg sync.WaitGroup
	const goroutines = 50
	const iterations = 100

	// Concurrent writers.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				c.Set("shared", id*iterations+j, 5*time.Minute)
			}
		}(i)
	}

	// Concurrent readers.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				c.Get("shared")
			}
		}()
	}

	wg.Wait()
	// If we get here without a race condition or panic, the test passes.
}

// ---------------------------------------------------------------------------
// GraphQL request marshaling tests
// ---------------------------------------------------------------------------

func TestGraphQLRequestMarshaling(t *testing.T) {
	req := graphqlRequest{
		OperationName: "getUserProfile",
		Query:         "query ($username: String!) { matchedUser(username: $username) { username } }",
		Variables: map[string]any{
			"username": "testuser",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded["operationName"] != "getUserProfile" {
		t.Errorf("operationName = %v, want %q", decoded["operationName"], "getUserProfile")
	}
	vars, ok := decoded["variables"].(map[string]any)
	if !ok {
		t.Fatal("variables is not a map")
	}
	if vars["username"] != "testuser" {
		t.Errorf("variables.username = %v, want %q", vars["username"], "testuser")
	}
}

func TestGraphQLRequestNilVariables(t *testing.T) {
	req := graphqlRequest{
		OperationName: "questionOfToday",
		Query:         "query { activeDailyCodingChallengeQuestion { date } }",
		Variables:     nil,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// nil map marshals as JSON null.
	if decoded["variables"] != nil {
		t.Errorf("variables = %v, want null", decoded["variables"])
	}
}

// ---------------------------------------------------------------------------
// Helper: newTestClient creates a Client pointing at a httptest.Server.
// The rate limiter is a closed-immediately channel so tests don't wait.
// ---------------------------------------------------------------------------

func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	srv := httptest.NewServer(handler)

	// Create an instantly-firing rate limiter for tests.
	rateCh := make(chan time.Time, 1)
	rateCh <- time.Now()

	client := &Client{
		http:        srv.Client(),
		rateLimiter: rateCh,
		cache:       NewCache(),
	}

	return client, srv
}

// overrideEndpoint temporarily patches the graphqlEndpoint package-level
// constant by using the Client's query method with a custom server URL.
// Since the endpoint is a const, we wrap the handler to intercept all
// requests and redirect the client via its HTTP client transport.

// queryWithServer performs a GraphQL query against a test server instead
// of the real LeetCode endpoint. It mimics Client.query but posts to the
// test server URL.
func queryWithServer(c *Client, srv *httptest.Server, ctx context.Context, opName, queryStr string, vars map[string]any, result any) error {
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
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", refererHeader)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("leetcode: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var raw struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphqlError  `json:"errors,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return err
	}

	if len(raw.Errors) > 0 {
		return fmt.Errorf("leetcode: graphql error: %s", raw.Errors[0].Message)
	}

	if raw.Data == nil {
		return fmt.Errorf("leetcode: empty data in response")
	}

	return json.Unmarshal(raw.Data, result)
}

// ---------------------------------------------------------------------------
// Response parsing tests — User Profile
// ---------------------------------------------------------------------------

const sampleUserProfileJSON = `{
	"data": {
		"allQuestionsCount": [
			{"difficulty": "All", "count": 3000},
			{"difficulty": "Easy", "count": 800},
			{"difficulty": "Medium", "count": 1500},
			{"difficulty": "Hard", "count": 700}
		],
		"matchedUser": {
			"username": "testuser",
			"contributions": {"points": 500, "questionCount": 10, "testcaseCount": 5},
			"profile": {
				"realName": "Test User",
				"countryName": "US",
				"company": "TestCorp",
				"school": "",
				"starRating": 4.5,
				"aboutMe": "I solve problems",
				"userAvatar": "https://example.com/avatar.png",
				"reputation": 100,
				"ranking": 5000
			},
			"submissionCalendar": "{\"1704067200\":3,\"1704153600\":5}",
			"submitStats": {
				"acSubmissionNum": [
					{"difficulty": "All", "count": 150, "submissions": 200},
					{"difficulty": "Easy", "count": 60, "submissions": 70},
					{"difficulty": "Medium", "count": 70, "submissions": 100},
					{"difficulty": "Hard", "count": 20, "submissions": 30}
				],
				"totalSubmissionNum": [
					{"difficulty": "All", "count": 150, "submissions": 400},
					{"difficulty": "Easy", "count": 60, "submissions": 120},
					{"difficulty": "Medium", "count": 70, "submissions": 200},
					{"difficulty": "Hard", "count": 20, "submissions": 80}
				]
			},
			"badges": [{"id": "1", "displayName": "Badge1", "icon": "icon1.png", "creationDate": "2024-01-01"}],
			"upcomingBadges": [{"name": "Upcoming1", "icon": "upcoming1.png"}],
			"activeBadge": {"id": "1"}
		},
		"recentSubmissionList": [
			{"title": "Two Sum", "titleSlug": "two-sum", "timestamp": "1704067200", "statusDisplay": "Accepted", "lang": "golang"}
		]
	}
}`

func TestParseUserProfileResponse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleUserProfileJSON))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data userProfileResponse
	err := queryWithServer(c, srv, ctx, "getUserProfile", QueryUserProfile, map[string]any{"username": "testuser"}, &data)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	// Verify allQuestionsCount.
	if len(data.AllQuestionsCount) != 4 {
		t.Fatalf("allQuestionsCount length = %d, want 4", len(data.AllQuestionsCount))
	}
	if data.AllQuestionsCount[0].Difficulty != "All" {
		t.Errorf("first difficulty = %q, want %q", data.AllQuestionsCount[0].Difficulty, "All")
	}
	if data.AllQuestionsCount[0].Count != 3000 {
		t.Errorf("first count = %d, want 3000", data.AllQuestionsCount[0].Count)
	}

	// Verify matchedUser.
	if data.MatchedUser == nil {
		t.Fatal("matchedUser is nil")
	}
	if data.MatchedUser.Username != "testuser" {
		t.Errorf("username = %q, want %q", data.MatchedUser.Username, "testuser")
	}
	if data.MatchedUser.Profile.Ranking != 5000 {
		t.Errorf("ranking = %d, want 5000", data.MatchedUser.Profile.Ranking)
	}
	if data.MatchedUser.Contributions.Points != 500 {
		t.Errorf("contribution points = %d, want 500", data.MatchedUser.Contributions.Points)
	}

	// Verify submitStats.
	if len(data.MatchedUser.SubmitStats.ACSubmissionNum) != 4 {
		t.Fatalf("acSubmissionNum length = %d, want 4", len(data.MatchedUser.SubmitStats.ACSubmissionNum))
	}

	// Verify badges.
	if len(data.MatchedUser.Badges) != 1 {
		t.Fatalf("badges length = %d, want 1", len(data.MatchedUser.Badges))
	}
	if data.MatchedUser.Badges[0].DisplayName != "Badge1" {
		t.Errorf("badge name = %q, want %q", data.MatchedUser.Badges[0].DisplayName, "Badge1")
	}

	// Verify recentSubmissionList.
	if len(data.RecentSubmissionList) != 1 {
		t.Fatalf("recentSubmissionList length = %d, want 1", len(data.RecentSubmissionList))
	}
	if data.RecentSubmissionList[0].Title != "Two Sum" {
		t.Errorf("submission title = %q, want %q", data.RecentSubmissionList[0].Title, "Two Sum")
	}
}

// ---------------------------------------------------------------------------
// Response parsing tests — Null / Private Profile
// ---------------------------------------------------------------------------

const sampleNullProfileJSON = `{
	"data": {
		"allQuestionsCount": [
			{"difficulty": "All", "count": 3000}
		],
		"matchedUser": null,
		"recentSubmissionList": null
	}
}`

func TestParseNullProfileResponse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleNullProfileJSON))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data userProfileResponse
	err := queryWithServer(c, srv, ctx, "getUserProfile", QueryUserProfile, map[string]any{"username": "privateuser"}, &data)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if data.MatchedUser != nil {
		t.Errorf("expected nil matchedUser for private profile, got %+v", data.MatchedUser)
	}
}

// ---------------------------------------------------------------------------
// Response parsing tests — User Calendar
// ---------------------------------------------------------------------------

const sampleUserCalendarJSON = `{
	"data": {
		"matchedUser": {
			"userCalendar": {
				"activeYears": [2023, 2024],
				"streak": 15,
				"totalActiveDays": 200,
				"dccBadges": [
					{"timestamp": "1704067200", "badge": {"name": "Jan 2024", "icon": "jan.png"}}
				],
				"submissionCalendar": "{\"1704067200\":3,\"1704153600\":5}"
			}
		}
	}
}`

func TestParseUserCalendarResponse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleUserCalendarJSON))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data userCalendarResponse
	err := queryWithServer(c, srv, ctx, "UserProfileCalendar", QueryUserCalendar, map[string]any{"username": "testuser", "year": 2024}, &data)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if data.MatchedUser == nil {
		t.Fatal("matchedUser is nil")
	}

	cal := data.MatchedUser.UserCalendar
	if cal.Streak != 15 {
		t.Errorf("streak = %d, want 15", cal.Streak)
	}
	if cal.TotalActiveDays != 200 {
		t.Errorf("totalActiveDays = %d, want 200", cal.TotalActiveDays)
	}
	if len(cal.ActiveYears) != 2 {
		t.Fatalf("activeYears length = %d, want 2", len(cal.ActiveYears))
	}
	if len(cal.DCCBadges) != 1 {
		t.Fatalf("dccBadges length = %d, want 1", len(cal.DCCBadges))
	}
	if cal.DCCBadges[0].Badge.Name != "Jan 2024" {
		t.Errorf("badge name = %q, want %q", cal.DCCBadges[0].Badge.Name, "Jan 2024")
	}
}

const sampleNullCalendarJSON = `{
	"data": {
		"matchedUser": null
	}
}`

func TestParseNullCalendarResponse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleNullCalendarJSON))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data userCalendarResponse
	err := queryWithServer(c, srv, ctx, "UserProfileCalendar", QueryUserCalendar, map[string]any{"username": "ghost", "year": 2024}, &data)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if data.MatchedUser != nil {
		t.Errorf("expected nil matchedUser, got %+v", data.MatchedUser)
	}
}

// ---------------------------------------------------------------------------
// Response parsing tests — Daily Challenge
// ---------------------------------------------------------------------------

const sampleDailyChallengeJSON = `{
	"data": {
		"activeDailyCodingChallengeQuestion": {
			"date": "2024-06-15",
			"link": "/problems/two-sum/",
			"question": {
				"questionId": "1",
				"questionFrontendId": "1",
				"title": "Two Sum",
				"titleSlug": "two-sum",
				"difficulty": "Easy",
				"topicTags": [
					{"name": "Array", "slug": "array"},
					{"name": "Hash Table", "slug": "hash-table"}
				],
				"status": null,
				"challengeQuestion": {
					"id": "123",
					"date": "2024-06-15",
					"incompleteChallengeCount": 0,
					"streakCount": 10,
					"type": "DAILY"
				}
			}
		}
	}
}`

func TestParseDailyChallengeResponse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleDailyChallengeJSON))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data dailyChallengeResponse
	err := queryWithServer(c, srv, ctx, "questionOfToday", QueryDailyChallenge, nil, &data)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	challenge := data.ActiveDailyCodingChallengeQuestion
	if challenge.Date != "2024-06-15" {
		t.Errorf("date = %q, want %q", challenge.Date, "2024-06-15")
	}
	if challenge.Link != "/problems/two-sum/" {
		t.Errorf("link = %q, want %q", challenge.Link, "/problems/two-sum/")
	}
	if challenge.Question.Title != "Two Sum" {
		t.Errorf("title = %q, want %q", challenge.Question.Title, "Two Sum")
	}
	if challenge.Question.Difficulty != "Easy" {
		t.Errorf("difficulty = %q, want %q", challenge.Question.Difficulty, "Easy")
	}
	if len(challenge.Question.TopicTags) != 2 {
		t.Fatalf("topicTags length = %d, want 2", len(challenge.Question.TopicTags))
	}
	if challenge.Question.TopicTags[0].Name != "Array" {
		t.Errorf("first tag = %q, want %q", challenge.Question.TopicTags[0].Name, "Array")
	}
	if challenge.Question.ChallengeQuestion == nil {
		t.Fatal("challengeQuestion is nil")
	}
	if challenge.Question.ChallengeQuestion.StreakCount != 10 {
		t.Errorf("streakCount = %d, want 10", challenge.Question.ChallengeQuestion.StreakCount)
	}
}

// ---------------------------------------------------------------------------
// Response parsing tests — Recent Submissions
// ---------------------------------------------------------------------------

const sampleRecentSubmissionsJSON = `{
	"data": {
		"recentSubmissionList": [
			{"title": "Two Sum", "titleSlug": "two-sum", "timestamp": "1704067200", "statusDisplay": "Accepted", "lang": "golang"},
			{"title": "Add Two Numbers", "titleSlug": "add-two-numbers", "timestamp": "1704060000", "statusDisplay": "Wrong Answer", "lang": "python3"}
		]
	}
}`

func TestParseRecentSubmissionsResponse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleRecentSubmissionsJSON))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data recentSubmissionsResponse
	err := queryWithServer(c, srv, ctx, "getRecentSubmissionList", QueryRecentSubmissions, map[string]any{"username": "testuser", "limit": 20}, &data)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if len(data.RecentSubmissionList) != 2 {
		t.Fatalf("submissions length = %d, want 2", len(data.RecentSubmissionList))
	}
	if data.RecentSubmissionList[0].Title != "Two Sum" {
		t.Errorf("first title = %q, want %q", data.RecentSubmissionList[0].Title, "Two Sum")
	}
	if data.RecentSubmissionList[0].StatusDisplay != "Accepted" {
		t.Errorf("first status = %q, want %q", data.RecentSubmissionList[0].StatusDisplay, "Accepted")
	}
	if data.RecentSubmissionList[1].Lang != "python3" {
		t.Errorf("second lang = %q, want %q", data.RecentSubmissionList[1].Lang, "python3")
	}
}

const sampleNullSubmissionsJSON = `{
	"data": {
		"recentSubmissionList": null
	}
}`

func TestParseNullSubmissionsResponse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleNullSubmissionsJSON))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data recentSubmissionsResponse
	err := queryWithServer(c, srv, ctx, "getRecentSubmissionList", QueryRecentSubmissions, map[string]any{"username": "ghost", "limit": 20}, &data)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if data.RecentSubmissionList != nil {
		t.Errorf("expected nil submissions for null response, got %+v", data.RecentSubmissionList)
	}
}

// ---------------------------------------------------------------------------
// HTTP error handling tests
// ---------------------------------------------------------------------------

func TestQueryHTTPError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data userProfileResponse
	err := queryWithServer(c, srv, ctx, "getUserProfile", QueryUserProfile, map[string]any{"username": "test"}, &data)
	if err == nil {
		t.Fatal("expected error for 429 response, got nil")
	}
}

func TestQueryGraphQLError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": null, "errors": [{"message": "user not found"}]}`))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data userProfileResponse
	err := queryWithServer(c, srv, ctx, "getUserProfile", QueryUserProfile, map[string]any{"username": "test"}, &data)
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
}

func TestQueryEmptyData(t *testing.T) {
	// When the server omits the "data" field entirely, the response wrapper's
	// Data field will be a nil json.RawMessage and the client should error.
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errors": []}`))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data userProfileResponse
	err := queryWithServer(c, srv, ctx, "getUserProfile", QueryUserProfile, map[string]any{"username": "test"}, &data)
	if err == nil {
		t.Fatal("expected error for empty data response, got nil")
	}
}

func TestQueryInvalidJSON(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json at all`))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data userProfileResponse
	err := queryWithServer(c, srv, ctx, "getUserProfile", QueryUserProfile, map[string]any{"username": "test"}, &data)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// Request verification tests
// ---------------------------------------------------------------------------

func TestQuerySendsCorrectHeaders(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Verify headers.
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}
		if r.Header.Get("Referer") != refererHeader {
			t.Errorf("Referer = %q, want %q", r.Header.Get("Referer"), refererHeader)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		// Return valid response.
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleNullProfileJSON))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data userProfileResponse
	queryWithServer(c, srv, ctx, "getUserProfile", QueryUserProfile, map[string]any{"username": "test"}, &data)
}

func TestQuerySendsCorrectBody(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var req graphqlRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}

		if req.OperationName != "getUserProfile" {
			t.Errorf("operationName = %q, want %q", req.OperationName, "getUserProfile")
		}
		if req.Query != QueryUserProfile {
			t.Error("query string does not match QueryUserProfile")
		}
		vars := req.Variables
		if vars["username"] != "testuser" {
			t.Errorf("variables.username = %v, want %q", vars["username"], "testuser")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleUserProfileJSON))
	}

	c, srv := newTestClient(handler)
	defer srv.Close()

	ctx := context.Background()
	var data userProfileResponse
	if err := queryWithServer(c, srv, ctx, "getUserProfile", QueryUserProfile, map[string]any{"username": "testuser"}, &data); err != nil {
		t.Fatalf("query: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Context cancellation test
// ---------------------------------------------------------------------------

func TestQueryContextCancellation(t *testing.T) {
	// Use a rate limiter that never fires to force context cancellation.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleUserProfileJSON))
	}))
	defer srv.Close()

	neverCh := make(chan time.Time) // never sends
	c := &Client{
		http:        srv.Client(),
		rateLimiter: neverCh,
		cache:       NewCache(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var data userProfileResponse
	err := queryWithServer(c, srv, ctx, "getUserProfile", QueryUserProfile, map[string]any{"username": "test"}, &data)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// ---------------------------------------------------------------------------
// ParseSubmissionCalendar tests
// ---------------------------------------------------------------------------

func TestParseSubmissionCalendar_Normal(t *testing.T) {
	raw := `{"1704067200":3,"1704153600":5}`

	result, err := ParseSubmissionCalendar(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("result length = %d, want 2", len(result))
	}

	// 1704067200 = 2024-01-01T00:00:00Z
	day1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if count, ok := result[day1]; !ok || count != 3 {
		t.Errorf("day1 count = %d (found=%t), want 3", count, ok)
	}

	// 1704153600 = 2024-01-02T00:00:00Z
	day2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	if count, ok := result[day2]; !ok || count != 5 {
		t.Errorf("day2 count = %d (found=%t), want 5", count, ok)
	}
}

func TestParseSubmissionCalendar_DoubleEncoded(t *testing.T) {
	// Simulate the double-encoded case where the string is JSON-quoted.
	raw := `"{\"1704067200\":3,\"1704153600\":5}"`

	result, err := ParseSubmissionCalendar(raw)
	if err != nil {
		t.Fatalf("parse double-encoded: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("result length = %d, want 2", len(result))
	}

	day1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if count, ok := result[day1]; !ok || count != 3 {
		t.Errorf("day1 count = %d (found=%t), want 3", count, ok)
	}
}

func TestParseSubmissionCalendar_Empty(t *testing.T) {
	result, err := ParseSubmissionCalendar("")
	if err != nil {
		t.Fatalf("parse empty: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestParseSubmissionCalendar_EmptyObject(t *testing.T) {
	result, err := ParseSubmissionCalendar("{}")
	if err != nil {
		t.Fatalf("parse empty object: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestParseSubmissionCalendar_InvalidJSON(t *testing.T) {
	_, err := ParseSubmissionCalendar("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseSubmissionCalendar_InvalidTimestamp(t *testing.T) {
	// Invalid timestamps are silently skipped.
	raw := `{"notanumber":3,"1704067200":5}`

	result, err := ParseSubmissionCalendar(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Only the valid timestamp should be in the result.
	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
}

// ---------------------------------------------------------------------------
// NewClient sanity test
// ---------------------------------------------------------------------------

func TestNewClient(t *testing.T) {
	client := NewClient()

	if client.http == nil {
		t.Error("http client is nil")
	}
	if client.rateLimiter == nil {
		t.Error("rate limiter is nil")
	}
	if client.cache == nil {
		t.Error("cache is nil")
	}
}

// ---------------------------------------------------------------------------
// Cache integration tests (typed values)
// ---------------------------------------------------------------------------

func TestCacheWithTypedValues(t *testing.T) {
	c := NewCache()

	profile := &userProfileResponse{
		AllQuestionsCount: []QuestionCount{{Difficulty: "Easy", Count: 100}},
		MatchedUser:       &MatchedUser{Username: "cached_user"},
	}

	c.Set("profile:cached_user", profile, 5*time.Minute)

	val, ok := c.Get("profile:cached_user")
	if !ok {
		t.Fatal("expected cache hit")
	}

	cached, ok := val.(*userProfileResponse)
	if !ok {
		t.Fatal("type assertion failed")
	}
	if cached.MatchedUser.Username != "cached_user" {
		t.Errorf("username = %q, want %q", cached.MatchedUser.Username, "cached_user")
	}
}

func TestCacheWithDailyChallenge(t *testing.T) {
	c := NewCache()

	challenge := &DailyChallenge{
		Date: "2024-06-15",
		Link: "/problems/two-sum/",
		Question: DailyQuestion{
			Title:      "Two Sum",
			Difficulty: "Easy",
		},
	}

	c.Set("daily", challenge, DailyChallengeCacheTTL)

	val, ok := c.Get("daily")
	if !ok {
		t.Fatal("expected cache hit")
	}

	cached := val.(*DailyChallenge)
	if cached.Question.Title != "Two Sum" {
		t.Errorf("title = %q, want %q", cached.Question.Title, "Two Sum")
	}
}

func TestCacheWithSubmissionSlice(t *testing.T) {
	c := NewCache()

	subs := []RecentSubmission{
		{Title: "Two Sum", StatusDisplay: "Accepted"},
		{Title: "Add Two Numbers", StatusDisplay: "Wrong Answer"},
	}

	c.Set("submissions:user", subs, StatsCacheTTL)

	val, ok := c.Get("submissions:user")
	if !ok {
		t.Fatal("expected cache hit")
	}

	cached := val.([]RecentSubmission)
	if len(cached) != 2 {
		t.Fatalf("length = %d, want 2", len(cached))
	}
}
