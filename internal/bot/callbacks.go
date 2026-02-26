package bot

import (
	"fmt"
	"log"
	"strings"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/user/leetcode-bot/internal/gamification"
)

// Callback data prefixes used for routing inline keyboard interactions.
// Each prefix identifies the type of action, followed by optional payload
// separated by ":". Example: "ach_cat:streaks".
const (
	callbackPrefixAchCategory = "ach_cat"
	callbackPrefixNoop        = "noop"
)

// handleCallback is the main router for inline keyboard callback queries.
// It inspects the callback data prefix and dispatches to the appropriate
// handler. Every code path must call AnswerCallbackQuery to dismiss the
// loading indicator on the client.
func (b *Bot) handleCallback(ctx *th.Context, query telego.CallbackQuery) error {
	data := query.Data

	// Route based on the callback data prefix.
	switch {
	case strings.HasPrefix(data, callbackPrefixAchCategory+":"):
		return b.handleAchievementCategoryCallback(ctx, query)

	case data == callbackPrefixNoop:
		// No-op callback — just acknowledge silently.
		return b.api.AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID))

	default:
		// Unknown callback — acknowledge to avoid stuck loading indicator.
		log.Printf("[bot] callback: unknown data=%q from user %d", data, query.From.ID)
		return b.api.AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("Unknown action"))
	}
}

// handleAchievementCategoryCallback handles the "ach_cat:<category>" callbacks
// triggered when a user browses achievement categories via inline keyboards.
// It displays achievements filtered by the selected category.
func (b *Bot) handleAchievementCategoryCallback(ctx *th.Context, query telego.CallbackQuery) error {
	// Parse the category from callback data: "ach_cat:<category>"
	parts := strings.SplitN(query.Data, ":", 2)
	if len(parts) < 2 || parts[1] == "" {
		return b.api.AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("Invalid category"))
	}
	category := parts[1]

	telegramID := query.From.ID

	// Retrieve the user's unlocked achievements.
	user, err := b.store.GetUserByTelegramID(telegramID)
	if err != nil || user == nil {
		log.Printf("[bot] callback ach_cat: failed to get user %d: %v", telegramID, err)
		return b.api.AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("Failed to load data"))
	}

	dbAchievements, err := b.store.GetUserAchievements(user.ID)
	if err != nil {
		log.Printf("[bot] callback ach_cat: failed to get achievements for user %d: %v", telegramID, err)
		return b.api.AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("Failed to load achievements"))
	}

	unlocked := make(map[string]bool, len(dbAchievements))
	for _, a := range dbAchievements {
		unlocked[a.AchievementKey] = true
	}

	// Filter achievements by category and format the response.
	all := gamification.AllAchievements()
	filtered := filterAchievementsByCategory(all, category)

	if len(filtered) == 0 {
		return b.api.AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("No achievements in this category"))
	}

	text := formatAchievementCategory(category, filtered, unlocked)

	// Edit the original message with the filtered achievements.
	if query.Message != nil {
		chatID := query.Message.GetChat().ID
		messageID := query.Message.GetMessageID()

		_, err := b.api.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:    tu.ID(chatID),
			MessageID: messageID,
			Text:      text,
			ParseMode: telego.ModeHTML,
		})
		if err != nil {
			log.Printf("[bot] callback ach_cat: failed to edit message: %v", err)
		}
	}

	// Acknowledge the callback.
	return b.api.AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID))
}

// achievementCategories defines human-readable names for achievement
// categories used in inline keyboard labels.
var achievementCategories = map[string]string{
	"solving":    "\U0001F4CA Solving",
	"streaks":    "\U0001F525 Streaks",
	"milestones": "\U0001F3C6 Milestones",
}

// filterAchievementsByCategory returns the subset of achievements belonging
// to the given category. Categories are defined as:
//   - "solving": easy_10, medium_10, hard_5, all_easy
//   - "streaks": streak_7, streak_30, daily_solver
//   - "milestones": first_blood, century, half_k
func filterAchievementsByCategory(achievements []gamification.Achievement, category string) []gamification.Achievement {
	categoryMap := map[string][]string{
		"solving":    {"easy_10", "medium_10", "hard_5", "all_easy"},
		"streaks":    {"streak_7", "streak_30", "daily_solver"},
		"milestones": {"first_blood", "century", "half_k"},
	}

	keys, ok := categoryMap[category]
	if !ok {
		return nil
	}

	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}

	var result []gamification.Achievement
	for _, a := range achievements {
		if keySet[a.Key] {
			result = append(result, a)
		}
	}
	return result
}

// formatAchievementCategory formats a list of achievements in a specific
// category, showing locked/unlocked status for each.
func formatAchievementCategory(category string, achievements []gamification.Achievement, unlocked map[string]bool) string {
	title := category
	if name, ok := achievementCategories[category]; ok {
		title = name
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>%s Achievements</b>\n\n", title))

	for _, a := range achievements {
		if unlocked[a.Key] {
			sb.WriteString(fmt.Sprintf("%s <b>%s</b> \u2014 %s \u2705\n", a.Emoji, a.Name, a.Description))
		} else {
			sb.WriteString(fmt.Sprintf("\U0001F512 <b>%s</b> \u2014 %s\n", a.Name, a.Description))
		}
	}

	return sb.String()
}
