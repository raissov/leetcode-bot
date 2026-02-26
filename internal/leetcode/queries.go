package leetcode

// GraphQL query string constants for the LeetCode API.
// Each query matches the verified shapes from research.json and the
// response types defined in types.go.

// QueryUserProfile fetches the comprehensive user profile including submission
// stats (by difficulty), badges, contribution points, recent submissions, and
// the full question count breakdown. The submissionCalendar field returns a
// JSON-encoded string that requires double-parsing.
const QueryUserProfile = `query ($username: String!) {
  allQuestionsCount {
    difficulty
    count
  }
  matchedUser(username: $username) {
    username
    contributions {
      points
      questionCount
      testcaseCount
    }
    profile {
      realName
      countryName
      company
      school
      starRating
      aboutMe
      userAvatar
      reputation
      ranking
    }
    submissionCalendar
    submitStats {
      acSubmissionNum {
        difficulty
        count
        submissions
      }
      totalSubmissionNum {
        difficulty
        count
        submissions
      }
    }
    badges {
      id
      displayName
      icon
      creationDate
    }
    upcomingBadges {
      name
      icon
    }
    activeBadge {
      id
    }
  }
  recentSubmissionList(username: $username, limit: 20) {
    title
    titleSlug
    timestamp
    statusDisplay
    lang
  }
}`

// QueryUserCalendar fetches the user's activity calendar for a given year,
// including streak count, total active days, daily coding challenge badges,
// and the submission calendar (JSON string of unix_timestamp -> count).
const QueryUserCalendar = `query UserProfileCalendar($username: String!, $year: Int!) {
  matchedUser(username: $username) {
    userCalendar(year: $year) {
      activeYears
      streak
      totalActiveDays
      dccBadges {
        timestamp
        badge {
          name
          icon
        }
      }
      submissionCalendar
    }
  }
}`

// QueryDailyChallenge fetches today's daily coding challenge, including the
// question details (title, difficulty, topic tags, link) and challenge metadata
// (streak count, incomplete challenge count).
const QueryDailyChallenge = `query {
  activeDailyCodingChallengeQuestion {
    date
    link
    question {
      questionId
      questionFrontendId
      title
      titleSlug
      difficulty
      topicTags {
        name
        slug
      }
      status
      challengeQuestion {
        id
        date
        incompleteChallengeCount
        streakCount
        type
      }
    }
  }
}`

// QueryRecentSubmissions fetches a user's recent submissions with a
// configurable limit. Each submission includes the problem title, slug,
// unix timestamp (in seconds), acceptance status, and language used.
const QueryRecentSubmissions = `query ($username: String!, $limit: Int) {
  recentSubmissionList(username: $username, limit: $limit) {
    title
    titleSlug
    timestamp
    statusDisplay
    lang
  }
}`

// QueryUserSolvedProblems fetches all problems the user has successfully
// solved (accepted submissions). Returns problem metadata including title,
// slug, and submission timestamp. Uses a high limit to capture all solved
// problems (LeetCode API supports up to 5000 results).
const QueryUserSolvedProblems = `query ($username: String!, $limit: Int!) {
  recentAcSubmissionList(username: $username, limit: $limit) {
    id
    title
    titleSlug
    timestamp
  }
}`

// QueryProblemDetails fetches detailed metadata for a specific problem
// including difficulty level and topic tags. Used to enrich solved problem
// data with full metadata.
const QueryProblemDetails = `query ($titleSlug: String!) {
  question(titleSlug: $titleSlug) {
    questionId
    questionFrontendId
    title
    titleSlug
    difficulty
    topicTags {
      name
      slug
    }
  }
}`
