package bot

import (
	"context"
	"log"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/user/leetcode-bot/internal/gamification"
	"github.com/user/leetcode-bot/internal/leetcode"
	"github.com/user/leetcode-bot/internal/scheduler"
	"github.com/user/leetcode-bot/internal/stats"
	"github.com/user/leetcode-bot/internal/storage"
)

// Bot is the main Telegram bot application. It holds all dependencies and
// manages the handler lifecycle. Created via New() and started via Start().
type Bot struct {
	api       *telego.Bot
	handler   *th.BotHandler
	store     *storage.DB
	lc        *leetcode.Client
	gamify    *gamification.Engine
	stats     *stats.Collector
	scheduler *scheduler.Reminder
}

// New creates a new Bot instance, sets up long-polling updates, registers
// middleware (panic recovery, logging, ensureUser), and registers all command
// and callback handlers. The bot is not started until Start() is called.
func New(
	api *telego.Bot,
	store *storage.DB,
	lc *leetcode.Client,
	gamify *gamification.Engine,
	statsCollector *stats.Collector,
	sched *scheduler.Reminder,
) (*Bot, error) {
	// Create long-polling updates channel.
	updates, err := api.UpdatesViaLongPolling(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	// Create bot handler from updates channel.
	bh, err := th.NewBotHandler(api, updates)
	if err != nil {
		return nil, err
	}

	b := &Bot{
		api:       api,
		handler:   bh,
		store:     store,
		lc:        lc,
		gamify:    gamify,
		stats:     statsCollector,
		scheduler: sched,
	}

	// --- Global Middleware ---
	// Order matters: PanicRecovery first, then logging, then ensureUser.
	bh.Use(
		th.PanicRecovery(),
		LoggingMiddleware(),
		EnsureUser(store),
	)

	// --- Command Handlers (no LeetCode account required) ---
	bh.HandleMessage(b.handleStart, th.CommandEqual("start"))
	bh.HandleMessage(b.handleHelp, th.CommandEqual("help"))
	bh.HandleMessage(b.handleConnect, th.CommandEqual("connect"))
	bh.HandleMessage(b.handleRemind, th.CommandEqual("remind"))

	// --- Command Handlers (LeetCode account required) ---
	// These commands need a connected LeetCode account. We use a handler
	// group with EnsureConnected middleware so users without a linked
	// account get a helpful error message instead of a crash.
	connected := bh.Group(th.Or(
		th.CommandEqual("stats"),
		th.CommandEqual("streak"),
		th.CommandEqual("daily"),
		th.CommandEqual("achievements"),
		th.CommandEqual("level"),
	))
	connected.Use(EnsureConnected(api, store))

	connected.HandleMessage(b.handleStats, th.CommandEqual("stats"))
	connected.HandleMessage(b.handleStreak, th.CommandEqual("streak"))
	connected.HandleMessage(b.handleDaily, th.CommandEqual("daily"))
	connected.HandleMessage(b.handleAchievements, th.CommandEqual("achievements"))
	connected.HandleMessage(b.handleLevel, th.CommandEqual("level"))

	// --- Callback Query Handlers ---
	bh.HandleCallbackQuery(b.handleCallback, th.AnyCallbackQueryWithMessage())

	return b, nil
}

// Start begins processing Telegram updates. This method blocks until Stop()
// is called or the bot handler is otherwise stopped.
func (b *Bot) Start() error {
	log.Println("[bot] starting bot handler")
	return b.handler.Start()
}

// Stop signals the bot handler to stop processing updates.
func (b *Bot) Stop() error {
	log.Println("[bot] stopping bot handler")
	return b.handler.Stop()
}

// GracefulShutdown performs a clean shutdown sequence: stops the bot handler,
// stops the cron scheduler, and closes the database connection. Call this
// on SIGINT/SIGTERM.
func (b *Bot) GracefulShutdown() {
	log.Println("[bot] initiating graceful shutdown")

	// 1. Stop accepting and processing updates.
	if err := b.handler.Stop(); err != nil {
		log.Printf("[bot] error stopping handler: %v", err)
	}

	// 2. Stop the reminder scheduler.
	if b.scheduler != nil {
		b.scheduler.Stop()
	}

	// 3. Close the database connection.
	if b.store != nil {
		if err := b.store.Close(); err != nil {
			log.Printf("[bot] error closing database: %v", err)
		}
	}

	log.Println("[bot] graceful shutdown complete")
}

// --- Stub handler methods ---
// Callback handler is a stub until replaced by callbacks.go (subtask-6-4).
// Command handlers (handleStart, handleHelp, etc.) are in handlers.go.

func (b *Bot) handleCallback(ctx *th.Context, query telego.CallbackQuery) error {
	// Acknowledge the callback query.
	return b.api.AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("OK"))
}
