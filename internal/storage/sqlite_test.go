package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testDB creates a temporary file-based SQLite database for testing.
// It returns the opened DB and a cleanup function that closes the DB
// and removes the temporary file.
func testDB(t *testing.T) *DB {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	return db
}

// ---------------------------------------------------------------------------
// Open / Close / migration idempotency
// ---------------------------------------------------------------------------

func TestOpenAndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Database file should exist on disk.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestMigrationIdempotency(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	// Open → migrate → close, then re-open to run migrations again.
	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second open (migration idempotency): %v", err)
	}
	db2.Close()
}

// ---------------------------------------------------------------------------
// User CRUD
// ---------------------------------------------------------------------------

func TestCreateUser(t *testing.T) {
	db := testDB(t)

	user, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if user.TelegramID != 12345 {
		t.Errorf("telegram id = %d, want 12345", user.TelegramID)
	}
	if user.TelegramName != "alice" {
		t.Errorf("telegram name = %q, want %q", user.TelegramName, "alice")
	}
	if user.Timezone != "UTC" {
		t.Errorf("timezone = %q, want %q", user.Timezone, "UTC")
	}
	if user.RemindHour != 9 {
		t.Errorf("remind hour = %d, want 9", user.RemindHour)
	}
	if !user.RemindEnabled {
		t.Error("remind enabled = false, want true")
	}
	if user.Points != 0 {
		t.Errorf("points = %d, want 0", user.Points)
	}
	if user.Level != 1 {
		t.Errorf("level = %d, want 1", user.Level)
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	db := testDB(t)

	u1, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	// Duplicate INSERT OR IGNORE should return existing user.
	u2, err := db.CreateUser(12345, "alice-updated")
	if err != nil {
		t.Fatalf("duplicate create: %v", err)
	}

	if u1.ID != u2.ID {
		t.Errorf("duplicate create returned different id: %d vs %d", u1.ID, u2.ID)
	}
	// Name should stay as original because INSERT OR IGNORE is a no-op.
	if u2.TelegramName != "alice" {
		t.Errorf("telegram name = %q, want %q (original)", u2.TelegramName, "alice")
	}
}

func TestGetUserByTelegramID_NotFound(t *testing.T) {
	db := testDB(t)

	user, err := db.GetUserByTelegramID(99999)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user != nil {
		t.Errorf("expected nil user for non-existent telegram id, got %+v", user)
	}
}

func TestUpdateLeetCodeUser(t *testing.T) {
	db := testDB(t)

	_, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := db.UpdateLeetCodeUser(12345, "alice_lc"); err != nil {
		t.Fatalf("update leetcode user: %v", err)
	}

	user, err := db.GetUserByTelegramID(12345)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.LeetCodeUser != "alice_lc" {
		t.Errorf("leetcode user = %q, want %q", user.LeetCodeUser, "alice_lc")
	}
}

func TestUpdateLeetCodeUser_NotFound(t *testing.T) {
	db := testDB(t)

	err := db.UpdateLeetCodeUser(99999, "nobody")
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}
}

func TestUpdateUserGamification(t *testing.T) {
	db := testDB(t)

	_, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := db.UpdateUserGamification(12345, 100, 3, 5, 10); err != nil {
		t.Fatalf("update gamification: %v", err)
	}

	user, err := db.GetUserByTelegramID(12345)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.Points != 100 {
		t.Errorf("points = %d, want 100", user.Points)
	}
	if user.Level != 3 {
		t.Errorf("level = %d, want 3", user.Level)
	}
	if user.CurrentStreak != 5 {
		t.Errorf("current streak = %d, want 5", user.CurrentStreak)
	}
	if user.BestStreak != 10 {
		t.Errorf("best streak = %d, want 10", user.BestStreak)
	}
}

func TestUpdateUserGamification_NotFound(t *testing.T) {
	db := testDB(t)

	err := db.UpdateUserGamification(99999, 10, 1, 1, 1)
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}
}

func TestUpdateReminder(t *testing.T) {
	db := testDB(t)

	_, err := db.CreateUser(12345, "alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := db.UpdateReminder(12345, false, 18, "America/New_York"); err != nil {
		t.Fatalf("update reminder: %v", err)
	}

	user, err := db.GetUserByTelegramID(12345)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.RemindEnabled {
		t.Error("remind enabled = true, want false")
	}
	if user.RemindHour != 18 {
		t.Errorf("remind hour = %d, want 18", user.RemindHour)
	}
	if user.Timezone != "America/New_York" {
		t.Errorf("timezone = %q, want %q", user.Timezone, "America/New_York")
	}
}

func TestUpdateReminder_NotFound(t *testing.T) {
	db := testDB(t)

	err := db.UpdateReminder(99999, true, 9, "UTC")
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}
}

func TestGetAllUsersWithReminders(t *testing.T) {
	db := testDB(t)

	// Create three users.
	db.CreateUser(1, "alice")
	db.CreateUser(2, "bob")
	db.CreateUser(3, "charlie")

	// Link LeetCode for alice and bob only.
	db.UpdateLeetCodeUser(1, "alice_lc")
	db.UpdateLeetCodeUser(2, "bob_lc")

	// Disable reminders for bob.
	db.UpdateReminder(2, false, 9, "UTC")

	users, err := db.GetAllUsersWithReminders()
	if err != nil {
		t.Fatalf("get all users with reminders: %v", err)
	}

	// Only alice should match (remind_enabled=1, leetcode_user != '').
	if len(users) != 1 {
		t.Fatalf("got %d users, want 1", len(users))
	}
	if users[0].TelegramID != 1 {
		t.Errorf("telegram id = %d, want 1", users[0].TelegramID)
	}
}

// ---------------------------------------------------------------------------
// Stats Snapshots
// ---------------------------------------------------------------------------

func TestSaveAndGetLatestSnapshot(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	date := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	err := db.SaveSnapshot(user.ID, 100, 40, 35, 25, 65.5, 50000, date)
	if err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	snap, err := db.GetLatestSnapshot(user.ID)
	if err != nil {
		t.Fatalf("get latest snapshot: %v", err)
	}
	if snap == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if snap.TotalSolved != 100 {
		t.Errorf("total solved = %d, want 100", snap.TotalSolved)
	}
	if snap.EasySolved != 40 {
		t.Errorf("easy solved = %d, want 40", snap.EasySolved)
	}
	if snap.MediumSolved != 35 {
		t.Errorf("medium solved = %d, want 35", snap.MediumSolved)
	}
	if snap.HardSolved != 25 {
		t.Errorf("hard solved = %d, want 25", snap.HardSolved)
	}
	if snap.AcceptanceRate != 65.5 {
		t.Errorf("acceptance rate = %f, want 65.5", snap.AcceptanceRate)
	}
	if snap.Ranking != 50000 {
		t.Errorf("ranking = %d, want 50000", snap.Ranking)
	}
}

func TestGetLatestSnapshot_NoSnapshots(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	snap, err := db.GetLatestSnapshot(user.ID)
	if err != nil {
		t.Fatalf("get latest snapshot: %v", err)
	}
	if snap != nil {
		t.Errorf("expected nil snapshot, got %+v", snap)
	}
}

func TestSaveSnapshotUpsert(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	date := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	// Save initial snapshot.
	db.SaveSnapshot(user.ID, 100, 40, 35, 25, 65.5, 50000, date)

	// Upsert with updated values on the same date.
	err := db.SaveSnapshot(user.ID, 105, 42, 37, 26, 66.0, 49000, date)
	if err != nil {
		t.Fatalf("upsert snapshot: %v", err)
	}

	snap, err := db.GetLatestSnapshot(user.ID)
	if err != nil {
		t.Fatalf("get latest after upsert: %v", err)
	}
	if snap.TotalSolved != 105 {
		t.Errorf("total solved after upsert = %d, want 105", snap.TotalSolved)
	}
	if snap.Ranking != 49000 {
		t.Errorf("ranking after upsert = %d, want 49000", snap.Ranking)
	}
}

func TestGetPreviousSnapshot(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	day1 := time.Date(2024, 6, 14, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	db.SaveSnapshot(user.ID, 95, 38, 33, 24, 64.0, 52000, day1)
	db.SaveSnapshot(user.ID, 100, 40, 35, 25, 65.5, 50000, day2)

	prev, err := db.GetPreviousSnapshot(user.ID)
	if err != nil {
		t.Fatalf("get previous snapshot: %v", err)
	}
	if prev == nil {
		t.Fatal("expected previous snapshot, got nil")
	}
	if prev.TotalSolved != 95 {
		t.Errorf("previous total solved = %d, want 95", prev.TotalSolved)
	}
}

func TestGetPreviousSnapshot_OnlyOne(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	date := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	db.SaveSnapshot(user.ID, 100, 40, 35, 25, 65.5, 50000, date)

	prev, err := db.GetPreviousSnapshot(user.ID)
	if err != nil {
		t.Fatalf("get previous snapshot: %v", err)
	}
	if prev != nil {
		t.Errorf("expected nil when only one snapshot exists, got %+v", prev)
	}
}

func TestGetSnapshotHistory(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	now := time.Now().UTC()

	// Insert snapshots for the last 3 days (recent enough to appear).
	for i := 2; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		total := 100 + (2 - i)
		db.SaveSnapshot(user.ID, total, 40, 35, 25, 65.5, 50000, date)
	}

	history, err := db.GetSnapshotHistory(user.ID, 7)
	if err != nil {
		t.Fatalf("get snapshot history: %v", err)
	}

	if len(history) != 3 {
		t.Fatalf("history length = %d, want 3", len(history))
	}

	// History should be ordered oldest → newest (ASC).
	if history[0].TotalSolved > history[2].TotalSolved {
		t.Errorf("history not in ascending order: first=%d, last=%d",
			history[0].TotalSolved, history[2].TotalSolved)
	}
}

func TestGetSnapshotHistory_Empty(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	history, err := db.GetSnapshotHistory(user.ID, 7)
	if err != nil {
		t.Fatalf("get snapshot history: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d entries", len(history))
	}
}

// ---------------------------------------------------------------------------
// Achievements
// ---------------------------------------------------------------------------

func TestUnlockAchievement(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	newlyUnlocked, err := db.UnlockAchievement(user.ID, "first_solve")
	if err != nil {
		t.Fatalf("unlock achievement: %v", err)
	}
	if !newlyUnlocked {
		t.Error("expected newly unlocked = true, got false")
	}
}

func TestUnlockAchievementDuplicate(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	db.UnlockAchievement(user.ID, "first_solve")

	// Second unlock of the same achievement should be a no-op.
	newlyUnlocked, err := db.UnlockAchievement(user.ID, "first_solve")
	if err != nil {
		t.Fatalf("duplicate unlock: %v", err)
	}
	if newlyUnlocked {
		t.Error("expected newly unlocked = false for duplicate, got true")
	}
}

func TestGetUserAchievements(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	db.UnlockAchievement(user.ID, "first_solve")
	db.UnlockAchievement(user.ID, "streak_3")
	db.UnlockAchievement(user.ID, "streak_7")

	achievements, err := db.GetUserAchievements(user.ID)
	if err != nil {
		t.Fatalf("get achievements: %v", err)
	}
	if len(achievements) != 3 {
		t.Fatalf("got %d achievements, want 3", len(achievements))
	}

	// Verify keys are present.
	keys := make(map[string]bool)
	for _, a := range achievements {
		keys[a.AchievementKey] = true
	}
	for _, want := range []string{"first_solve", "streak_3", "streak_7"} {
		if !keys[want] {
			t.Errorf("achievement %q not found", want)
		}
	}
}

func TestGetUserAchievements_Empty(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	achievements, err := db.GetUserAchievements(user.ID)
	if err != nil {
		t.Fatalf("get achievements: %v", err)
	}
	if len(achievements) != 0 {
		t.Errorf("expected empty achievements, got %d", len(achievements))
	}
}

func TestHasAchievement(t *testing.T) {
	db := testDB(t)

	user, _ := db.CreateUser(12345, "alice")

	db.UnlockAchievement(user.ID, "first_solve")

	has, err := db.HasAchievement(user.ID, "first_solve")
	if err != nil {
		t.Fatalf("has achievement: %v", err)
	}
	if !has {
		t.Error("expected has = true, got false")
	}

	has, err = db.HasAchievement(user.ID, "non_existent")
	if err != nil {
		t.Fatalf("has achievement (non-existent): %v", err)
	}
	if has {
		t.Error("expected has = false for non-existent achievement, got true")
	}
}
