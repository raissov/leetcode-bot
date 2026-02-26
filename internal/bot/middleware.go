package bot

import (
	"log"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/user/leetcode-bot/internal/storage"
)

// extractUser returns the Telegram user from the update, checking
// Message, CallbackQuery, and InlineQuery in order.
// Returns nil if no user can be determined.
func extractUser(update telego.Update) *telego.User {
	if update.Message != nil && update.Message.From != nil {
		return update.Message.From
	}
	if update.CallbackQuery != nil {
		return &update.CallbackQuery.From
	}
	if update.InlineQuery != nil {
		return &update.InlineQuery.From
	}
	return nil
}

// extractChatID returns the chat ID from the update for sending replies.
// Returns 0 if no chat can be determined.
func extractChatID(update telego.Update) int64 {
	if update.Message != nil {
		return update.Message.Chat.ID
	}
	if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
		return update.CallbackQuery.Message.GetChat().ID
	}
	return 0
}

// extractCommand returns the command name from the update, if any.
func extractCommand(update telego.Update) string {
	if update.Message == nil || update.Message.Text == "" {
		return ""
	}
	text := update.Message.Text
	if len(text) == 0 || text[0] != '/' {
		return ""
	}
	// Extract command (up to first space or end of string).
	cmd := text[1:]
	for i, ch := range cmd {
		if ch == ' ' || ch == '@' {
			cmd = cmd[:i]
			break
		}
	}
	return cmd
}

// LoggingMiddleware logs each incoming update to stderr with the command name,
// user ID, and processing duration.
func LoggingMiddleware() th.Handler {
	return func(ctx *th.Context, update telego.Update) error {
		start := time.Now()

		user := extractUser(update)
		cmd := extractCommand(update)

		var userID int64
		if user != nil {
			userID = user.ID
		}

		err := ctx.Next(update)

		duration := time.Since(start)

		if cmd != "" {
			log.Printf("[bot] cmd=/%s user_id=%d duration=%s err=%v", cmd, userID, duration, err)
		} else {
			log.Printf("[bot] update_id=%d user_id=%d duration=%s err=%v", update.UpdateID, userID, duration, err)
		}

		return err
	}
}

// EnsureUser is a middleware that auto-creates a user record in the database
// if one does not already exist. It runs on every incoming update that has
// a detectable Telegram user.
func EnsureUser(db *storage.DB) th.Handler {
	return func(ctx *th.Context, update telego.Update) error {
		user := extractUser(update)
		if user == nil {
			// No user info available — pass through.
			return ctx.Next(update)
		}

		// Build a display name from the Telegram user.
		name := user.FirstName
		if user.LastName != "" {
			name += " " + user.LastName
		}
		if name == "" {
			name = user.Username
		}

		_, err := db.CreateUser(user.ID, name)
		if err != nil {
			log.Printf("[bot] ensure_user: failed to create user %d: %v", user.ID, err)
			// Don't block the handler — log and continue.
		}

		return ctx.Next(update)
	}
}

// EnsureConnected is a middleware that checks whether the user has linked
// a LeetCode account. If not, it sends an error message and stops the
// middleware chain. This middleware should be applied to handler groups
// that require a connected LeetCode account (e.g., /stats, /streak).
func EnsureConnected(bot *telego.Bot, db *storage.DB) th.Handler {
	return func(ctx *th.Context, update telego.Update) error {
		user := extractUser(update)
		if user == nil {
			return ctx.Next(update)
		}

		dbUser, err := db.GetUserByTelegramID(user.ID)
		if err != nil {
			log.Printf("[bot] ensure_connected: failed to get user %d: %v", user.ID, err)
			return ctx.Next(update)
		}

		if dbUser == nil || dbUser.LeetCodeUser == "" {
			chatID := extractChatID(update)
			if chatID != 0 {
				_, _ = bot.SendMessage(ctx, tu.Message(
					tu.ID(chatID),
					"⚠️ Please connect your LeetCode account first using <code>/connect &lt;username&gt;</code>.",
				).WithParseMode(telego.ModeHTML))
			}
			// Stop the middleware chain — don't call the handler.
			return nil
		}

		return ctx.Next(update)
	}
}
