package leetcode

// graphqlRequest is the request body sent to the LeetCode GraphQL endpoint.
type graphqlRequest struct {
	OperationName string         `json:"operationName"`
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
}

// graphqlResponse is the generic wrapper for all LeetCode GraphQL responses.
type graphqlResponse struct {
	Data   any            `json:"data"`
	Errors []graphqlError `json:"errors,omitempty"`
}

// graphqlError represents a single error in a GraphQL response.
type graphqlError struct {
	Message string `json:"message"`
}

// --- User Profile & Stats ---

// userProfileResponse wraps the data returned by the user profile query.
type userProfileResponse struct {
	AllQuestionsCount    []QuestionCount    `json:"allQuestionsCount"`
	MatchedUser          *MatchedUser       `json:"matchedUser"`
	RecentSubmissionList []RecentSubmission `json:"recentSubmissionList"`
}

// QuestionCount holds the total number of questions for a given difficulty.
type QuestionCount struct {
	Difficulty string `json:"difficulty"`
	Count      int    `json:"count"`
}

// MatchedUser contains the full user profile returned by the matchedUser query.
type MatchedUser struct {
	Username           string          `json:"username"`
	Contributions      Contributions   `json:"contributions"`
	Profile            UserProfile     `json:"profile"`
	SubmissionCalendar string          `json:"submissionCalendar"`
	SubmitStats        SubmitStats     `json:"submitStats"`
	Badges             []Badge         `json:"badges"`
	UpcomingBadges     []UpcomingBadge `json:"upcomingBadges"`
	ActiveBadge        *ActiveBadge    `json:"activeBadge"`
}

// Contributions holds the user's contribution stats.
type Contributions struct {
	Points        int `json:"points"`
	QuestionCount int `json:"questionCount"`
	TestcaseCount int `json:"testcaseCount"`
}

// UserProfile holds the user's profile information.
type UserProfile struct {
	RealName    string  `json:"realName"`
	CountryName string  `json:"countryName"`
	Company     string  `json:"company"`
	School      string  `json:"school"`
	StarRating  float64 `json:"starRating"`
	AboutMe     string  `json:"aboutMe"`
	UserAvatar  string  `json:"userAvatar"`
	Reputation  int     `json:"reputation"`
	Ranking     int     `json:"ranking"`
}

// SubmitStats wraps the accepted and total submission counts by difficulty.
type SubmitStats struct {
	ACSubmissionNum    []SubmissionNum `json:"acSubmissionNum"`
	TotalSubmissionNum []SubmissionNum `json:"totalSubmissionNum"`
}

// SubmissionNum holds submission counts for a specific difficulty level.
type SubmissionNum struct {
	Difficulty  string `json:"difficulty"`
	Count       int    `json:"count"`
	Submissions int    `json:"submissions"`
}

// Badge represents a LeetCode badge earned by the user.
type Badge struct {
	ID           string `json:"id"`
	DisplayName  string `json:"displayName"`
	Icon         string `json:"icon"`
	CreationDate string `json:"creationDate"`
}

// UpcomingBadge represents a badge the user is close to earning.
type UpcomingBadge struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// ActiveBadge references the user's currently displayed badge.
type ActiveBadge struct {
	ID string `json:"id"`
}

// --- Recent Submissions ---

// RecentSubmission represents a single recent submission.
type RecentSubmission struct {
	Title         string `json:"title"`
	TitleSlug     string `json:"titleSlug"`
	Timestamp     string `json:"timestamp"`
	StatusDisplay string `json:"statusDisplay"`
	Lang          string `json:"lang"`
}

// --- User Calendar ---

// userCalendarResponse wraps the data returned by the user calendar query.
type userCalendarResponse struct {
	MatchedUser *calendarMatchedUser `json:"matchedUser"`
}

// calendarMatchedUser is the matched user shape returned by the calendar query.
type calendarMatchedUser struct {
	UserCalendar UserCalendar `json:"userCalendar"`
}

// UserCalendar holds the user's activity calendar data.
type UserCalendar struct {
	ActiveYears        []int      `json:"activeYears"`
	Streak             int        `json:"streak"`
	TotalActiveDays    int        `json:"totalActiveDays"`
	DCCBadges          []DCCBadge `json:"dccBadges"`
	SubmissionCalendar string     `json:"submissionCalendar"`
}

// DCCBadge represents a daily coding challenge badge.
type DCCBadge struct {
	Timestamp string    `json:"timestamp"`
	Badge     BadgeInfo `json:"badge"`
}

// BadgeInfo holds badge metadata.
type BadgeInfo struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// --- Daily Challenge ---

// dailyChallengeResponse wraps the data returned by the daily challenge query.
type dailyChallengeResponse struct {
	ActiveDailyCodingChallengeQuestion DailyChallenge `json:"activeDailyCodingChallengeQuestion"`
}

// DailyChallenge represents today's daily coding challenge.
type DailyChallenge struct {
	Date     string        `json:"date"`
	Link     string        `json:"link"`
	Question DailyQuestion `json:"question"`
}

// DailyQuestion holds the question details for a daily challenge.
type DailyQuestion struct {
	QuestionID         string             `json:"questionId"`
	QuestionFrontendID string             `json:"questionFrontendId"`
	Title              string             `json:"title"`
	TitleSlug          string             `json:"titleSlug"`
	Difficulty         string             `json:"difficulty"`
	TopicTags          []TopicTag         `json:"topicTags"`
	Status             string             `json:"status"`
	ChallengeQuestion  *ChallengeQuestion `json:"challengeQuestion"`
}

// TopicTag represents a topic tag associated with a problem.
type TopicTag struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ChallengeQuestion holds metadata about the daily challenge question.
type ChallengeQuestion struct {
	ID                       string `json:"id"`
	Date                     string `json:"date"`
	IncompleteChallengeCount int    `json:"incompleteChallengeCount"`
	StreakCount              int    `json:"streakCount"`
	Type                     string `json:"type"`
}

// --- Recent Submissions (standalone query) ---

// recentSubmissionsResponse wraps the data returned by the recent submissions query.
type recentSubmissionsResponse struct {
	RecentSubmissionList []RecentSubmission `json:"recentSubmissionList"`
}
