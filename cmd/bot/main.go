package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mymmrac/telego"

	"github.com/user/leetcode-bot/internal/bot"
	"github.com/user/leetcode-bot/internal/gamification"
	"github.com/user/leetcode-bot/internal/leetcode"
	"github.com/user/leetcode-bot/internal/scheduler"
	"github.com/user/leetcode-bot/internal/stats"
	"github.com/user/leetcode-bot/internal/storage"
)

func main() {
	log.Println("LeetCode Bot starting...")

	// --- Read TOKEN from environment (required) ---
	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatal("TOKEN environment variable is required")
	}

	// --- Create Telegram bot API instance ---
	api, err := telego.NewBot(token)
	if err != nil {
		log.Fatalf("create telegram bot: %v", err)
	}
	log.Println("[main] telegram bot API initialized")

	// --- Open SQLite database ---
	db, err := storage.Open("leetcode_bot.db")
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	log.Println("[main] database opened and migrated")

	// --- Load curated lists from JSON files ---
	curatedListFiles := []string{"data/blind75.json", "data/neetcode150.json"}
	loadedCount := 0
	for _, filePath := range curatedListFiles {
		if err := db.LoadCuratedList(filePath); err != nil {
			log.Printf("[main] warning: failed to load curated list from %s: %v", filePath, err)
		} else {
			loadedCount++
		}
	}
	log.Printf("[main] loaded %d curated lists", loadedCount)

	// --- Create LeetCode client ---
	lcClient := leetcode.NewClient()
	log.Println("[main] leetcode client created")

	// --- Create gamification engine ---
	gamifyEngine := gamification.NewEngine(db)
	log.Println("[main] gamification engine created")

	// --- Create stats collector ---
	collector := stats.NewCollector(lcClient, db)
	log.Println("[main] stats collector created")

	// --- Create scheduler reminder ---
	reminder := scheduler.New(api, db, lcClient, collector)
	log.Println("[main] scheduler created")

	// --- Load existing reminders from DB ---
	if err := reminder.LoadAllReminders(); err != nil {
		log.Printf("[main] warning: failed to load reminders: %v", err)
	}

	// --- Start the reminder scheduler ---
	reminder.Start()

	// --- Create bot handler with all dependencies ---
	b, err := bot.New(api, db, lcClient, gamifyEngine, collector, reminder)
	if err != nil {
		log.Fatalf("create bot handler: %v", err)
	}
	log.Println("[main] bot handler created")

	// --- Setup graceful shutdown ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigChan:
			log.Printf("[main] received signal: %v, initiating shutdown...", sig)
			cancel()
			b.GracefulShutdown()
		case <-ctx.Done():
		}
	}()

	// --- Start bot (blocks until stopped) ---
	log.Println("[main] bot is now running. Press Ctrl+C to stop.")
	if err := b.Start(); err != nil {
		log.Printf("[main] bot stopped: %v", err)
	}

	log.Println("[main] LeetCode Bot shut down complete.")
}
