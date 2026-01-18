package state

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	defer store.Close() //nolint:errcheck // test cleanup

	t.Run("thread operations", func(t *testing.T) {
		// Initially no thread
		_, ok := store.Thread(ctx, "owner", "repo", 1, "chan1")
		if ok {
			t.Error("Thread() found non-existent thread")
		}

		// Save thread
		info := ThreadInfo{
			ThreadID:    "thread123",
			MessageID:   "msg456",
			ChannelID:   "chan1",
			ChannelType: "forum",
			LastState:   "needs_review",
		}
		if err := store.SaveThread(ctx, "owner", "repo", 1, "chan1", info); err != nil {
			t.Fatalf("SaveThread() error = %v", err)
		}

		// Retrieve thread
		got, ok := store.Thread(ctx, "owner", "repo", 1, "chan1")
		if !ok {
			t.Fatal("Thread() did not find saved thread")
		}
		if got.ThreadID != info.ThreadID {
			t.Errorf("Thread().ThreadID = %q, want %q", got.ThreadID, info.ThreadID)
		}
		if got.UpdatedAt.IsZero() {
			t.Error("Thread().UpdatedAt should be set")
		}

		// Different channel returns nothing
		_, ok = store.Thread(ctx, "owner", "repo", 1, "chan2")
		if ok {
			t.Error("Thread() should not find thread for different channel")
		}
	})

	t.Run("DM info operations", func(t *testing.T) {
		prURL := "https://github.com/owner/repo/pull/42"

		// Initially no DM info
		_, ok := store.DMInfo(ctx, "user1", prURL)
		if ok {
			t.Error("DMInfo() found non-existent info")
		}

		// Save DM info
		info := DMInfo{
			ChannelID:   "dmchan123",
			MessageID:   "dmmsg456",
			MessageText: "Hello",
			SentAt:      time.Now(),
		}
		if err := store.SaveDMInfo(ctx, "user1", prURL, info); err != nil {
			t.Fatalf("SaveDMInfo() error = %v", err)
		}

		// Retrieve DM info
		got, ok := store.DMInfo(ctx, "user1", prURL)
		if !ok {
			t.Fatal("DMInfo() did not find saved info")
		}
		if got.ChannelID != info.ChannelID {
			t.Errorf("DMInfo().ChannelID = %q, want %q", got.ChannelID, info.ChannelID)
		}

		// Different user returns nothing
		_, ok = store.DMInfo(ctx, "user2", prURL)
		if ok {
			t.Error("DMInfo() should not find info for different user")
		}
	})

	t.Run("event processing", func(t *testing.T) {
		eventKey := "event123"

		// Initially not processed
		if store.WasProcessed(ctx, eventKey) {
			t.Error("WasProcessed() returned true for unprocessed event")
		}

		// Mark processed
		if err := store.MarkProcessed(ctx, eventKey, time.Hour); err != nil {
			t.Fatalf("MarkProcessed() error = %v", err)
		}

		// Now processed
		if !store.WasProcessed(ctx, eventKey) {
			t.Error("WasProcessed() returned false for processed event")
		}
	})

	t.Run("pending DMs", func(t *testing.T) {
		now := time.Now()

		dm1 := &PendingDM{
			ID:          "dm1",
			UserID:      "user1",
			PRURL:       "https://github.com/o/r/pull/1",
			MessageText: "Hello",
			SendAt:      now.Add(-time.Hour), // Past
			GuildID:     "guild1",
		}
		dm2 := &PendingDM{
			ID:          "dm2",
			UserID:      "user2",
			PRURL:       "https://github.com/o/r/pull/2",
			MessageText: "World",
			SendAt:      now.Add(time.Hour), // Future
			GuildID:     "guild1",
		}

		if err := store.QueuePendingDM(ctx, dm1); err != nil {
			t.Fatalf("QueuePendingDM() error = %v", err)
		}
		if err := store.QueuePendingDM(ctx, dm2); err != nil {
			t.Fatalf("QueuePendingDM() error = %v", err)
		}

		// Get pending DMs due now
		pending, err := store.PendingDMs(ctx, now)
		if err != nil {
			t.Fatalf("PendingDMs() error = %v", err)
		}
		if len(pending) != 1 {
			t.Errorf("PendingDMs() returned %d, want 1", len(pending))
		}
		if len(pending) > 0 && pending[0].ID != "dm1" {
			t.Errorf("PendingDMs()[0].ID = %q, want dm1", pending[0].ID)
		}

		// Remove dm1
		if err := store.RemovePendingDM(ctx, "dm1"); err != nil {
			t.Fatalf("RemovePendingDM() error = %v", err)
		}

		// dm1 should be gone
		pending, err = store.PendingDMs(ctx, now)
		if err != nil {
			t.Fatalf("PendingDMs() error = %v", err)
		}
		if len(pending) != 0 {
			t.Errorf("PendingDMs() returned %d after removal, want 0", len(pending))
		}
	})

	t.Run("stats", func(t *testing.T) {
		stats := store.Stats()
		if stats.Threads < 1 {
			t.Errorf("Stats() threads = %d, want >= 1", stats.Threads)
		}
		if stats.DMs < 1 {
			t.Errorf("Stats() dms = %d, want >= 1", stats.DMs)
		}
		if stats.Events < 1 {
			t.Errorf("Stats() events = %d, want >= 1", stats.Events)
		}
		// pending could be 0 or 1 depending on previous test
		_ = stats.Pending
	})
}

func TestMemoryStore_Cleanup(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	defer store.Close() //nolint:errcheck // test cleanup

	// Manually set short retention periods for testing
	store.eventRetain = time.Millisecond
	store.threadRetain = time.Millisecond
	store.dmRetain = time.Millisecond

	// Add data
	if err := store.MarkProcessed(ctx, "old-event", time.Millisecond); err != nil {
		t.Fatalf("MarkProcessed() error = %v", err)
	}

	info := ThreadInfo{ThreadID: "old-thread"}
	if err := store.SaveThread(ctx, "o", "r", 1, "c", info); err != nil {
		t.Fatalf("SaveThread() error = %v", err)
	}

	dmInfo := DMInfo{ChannelID: "dm-chan", SentAt: time.Now()}
	if err := store.SaveDMInfo(ctx, "user", "pr-url", dmInfo); err != nil {
		t.Fatalf("SaveDMInfo() error = %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Cleanup
	if err := store.Cleanup(ctx); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// Verify event was cleaned up
	if store.WasProcessed(ctx, "old-event") {
		t.Error("old event should have been cleaned up")
	}

	// Thread should be cleaned up too
	_, ok := store.Thread(ctx, "o", "r", 1, "c")
	if ok {
		t.Error("old thread should have been cleaned up")
	}

	// DM info should be cleaned up
	_, ok = store.DMInfo(ctx, "user", "pr-url")
	if ok {
		t.Error("old DM info should have been cleaned up")
	}
}

func TestThreadKey(t *testing.T) {
	key := threadKey("owner", "repo", 42, "chan123")
	expected := "owner/repo#42:chan123"
	if key != expected {
		t.Errorf("threadKey() = %q, want %q", key, expected)
	}
}

func TestDMKey(t *testing.T) {
	key := dmKey("user123", "https://github.com/o/r/pull/1")
	expected := "user123:https://github.com/o/r/pull/1"
	if key != expected {
		t.Errorf("dmKey() = %q, want %q", key, expected)
	}
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestMemoryStore_WasProcessed_Expired(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	store.eventRetain = time.Millisecond // Very short retention

	// Mark as processed
	if err := store.MarkProcessed(ctx, "expiring-event", time.Millisecond); err != nil {
		t.Fatalf("MarkProcessed() error = %v", err)
	}

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Should return false since event expired
	if store.WasProcessed(ctx, "expiring-event") {
		t.Error("WasProcessed() should return false for expired event")
	}
}

func TestMemoryStore_DailyReportInfo(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	defer store.Close() //nolint:errcheck // test cleanup

	userID := "user123"

	// Initially no report info
	_, ok := store.DailyReportInfo(ctx, userID)
	if ok {
		t.Error("DailyReportInfo() found non-existent info")
	}

	// Save report info
	info := DailyReportInfo{
		LastSentAt: time.Now(),
		GuildID:    "guild123",
	}
	if err := store.SaveDailyReportInfo(ctx, userID, info); err != nil {
		t.Fatalf("SaveDailyReportInfo() error = %v", err)
	}

	// Retrieve report info
	got, ok := store.DailyReportInfo(ctx, userID)
	if !ok {
		t.Fatal("DailyReportInfo() did not find saved info")
	}
	if got.GuildID != info.GuildID {
		t.Errorf("DailyReportInfo().GuildID = %q, want %q", got.GuildID, info.GuildID)
	}
	if got.LastSentAt.IsZero() {
		t.Error("DailyReportInfo().LastSentAt should not be zero")
	}

	// Different user returns nothing
	_, ok = store.DailyReportInfo(ctx, "other-user")
	if ok {
		t.Error("DailyReportInfo() should not find info for different user")
	}

	// Update existing
	newInfo := DailyReportInfo{
		LastSentAt: time.Now().Add(time.Hour),
		GuildID:    "guild456",
	}
	if err := store.SaveDailyReportInfo(ctx, userID, newInfo); err != nil {
		t.Fatalf("SaveDailyReportInfo() update error = %v", err)
	}

	got, ok = store.DailyReportInfo(ctx, userID)
	if !ok {
		t.Fatal("DailyReportInfo() did not find updated info")
	}
	if got.GuildID != newInfo.GuildID {
		t.Errorf("Updated DailyReportInfo().GuildID = %q, want %q", got.GuildID, newInfo.GuildID)
	}
}

// TestMemoryStore_ClaimThread tests thread claim locking.
func TestMemoryStore_ClaimThread(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	defer store.Close() //nolint:errcheck // test cleanup

	// First claim should succeed
	if !store.ClaimThread(ctx, "owner", "repo", 1, "chan1", time.Second) {
		t.Error("ClaimThread() should succeed on first attempt")
	}

	// Immediate second claim should fail (locked)
	if store.ClaimThread(ctx, "owner", "repo", 1, "chan1", time.Second) {
		t.Error("ClaimThread() should fail when already claimed")
	}

	// Different thread should succeed
	if !store.ClaimThread(ctx, "owner", "repo", 2, "chan1", time.Second) {
		t.Error("ClaimThread() should succeed for different PR")
	}

	// Wait for lock to expire
	time.Sleep(1100 * time.Millisecond)

	// Should be able to claim again after expiry
	if !store.ClaimThread(ctx, "owner", "repo", 1, "chan1", time.Second) {
		t.Error("ClaimThread() should succeed after lock expiry")
	}
}

// TestMemoryStore_ClaimDM tests DM claim locking.
func TestMemoryStore_ClaimDM(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	defer store.Close() //nolint:errcheck // test cleanup

	userID := "user123"
	prURL := "https://github.com/owner/repo/pull/1"

	// First claim should succeed
	if !store.ClaimDM(ctx, userID, prURL, time.Second) {
		t.Error("ClaimDM() should succeed on first attempt")
	}

	// Immediate second claim should fail (locked)
	if store.ClaimDM(ctx, userID, prURL, time.Second) {
		t.Error("ClaimDM() should fail when already claimed")
	}

	// Different PR should succeed
	prURL2 := "https://github.com/owner/repo/pull/2"
	if !store.ClaimDM(ctx, userID, prURL2, time.Second) {
		t.Error("ClaimDM() should succeed for different PR")
	}

	// Different user should succeed
	if !store.ClaimDM(ctx, "user456", prURL, time.Second) {
		t.Error("ClaimDM() should succeed for different user")
	}

	// Wait for lock to expire
	time.Sleep(1100 * time.Millisecond)

	// Should be able to claim again after expiry
	if !store.ClaimDM(ctx, userID, prURL, time.Second) {
		t.Error("ClaimDM() should succeed after lock expiry")
	}
}

// TestMemoryStore_ListDMUsers tests listing users with DMs for a PR.
func TestMemoryStore_ListDMUsers(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	defer store.Close() //nolint:errcheck // test cleanup

	prURL := "https://github.com/owner/repo/pull/1"

	// Initially empty
	users := store.ListDMUsers(ctx, prURL)
	if len(users) != 0 {
		t.Errorf("ListDMUsers() returned %d users, want 0", len(users))
	}

	// Save DM info for two users
	dm1 := DMInfo{ChannelID: "chan1", MessageID: "msg1", SentAt: time.Now()}
	if err := store.SaveDMInfo(ctx, "user1", prURL, dm1); err != nil {
		t.Fatalf("SaveDMInfo(user1) error = %v", err)
	}

	dm2 := DMInfo{ChannelID: "chan2", MessageID: "msg2", SentAt: time.Now()}
	if err := store.SaveDMInfo(ctx, "user2", prURL, dm2); err != nil {
		t.Fatalf("SaveDMInfo(user2) error = %v", err)
	}

	// Different PR for same user
	prURL2 := "https://github.com/owner/repo/pull/2"
	dm3 := DMInfo{ChannelID: "chan3", MessageID: "msg3", SentAt: time.Now()}
	if err := store.SaveDMInfo(ctx, "user1", prURL2, dm3); err != nil {
		t.Fatalf("SaveDMInfo(user1, pr2) error = %v", err)
	}

	// Should get two users for first PR
	users = store.ListDMUsers(ctx, prURL)
	if len(users) != 2 {
		t.Fatalf("ListDMUsers() returned %d users, want 2", len(users))
	}

	// Check both users are present
	userMap := make(map[string]bool)
	for _, u := range users {
		userMap[u] = true
	}
	if !userMap["user1"] {
		t.Error("ListDMUsers() should include user1")
	}
	if !userMap["user2"] {
		t.Error("ListDMUsers() should include user2")
	}

	// Only one user for second PR
	users = store.ListDMUsers(ctx, prURL2)
	if len(users) != 1 {
		t.Fatalf("ListDMUsers(pr2) returned %d users, want 1", len(users))
	}
	if users[0] != "user1" {
		t.Errorf("ListDMUsers(pr2)[0] = %q, want user1", users[0])
	}
}

// TestMemoryStore_UserMapping tests user mapping operations.
func TestMemoryStore_UserMapping(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	defer store.Close() //nolint:errcheck // test cleanup

	guildID := "guild-123"
	githubUser1 := "octocat"
	githubUser2 := "torvalds"

	// Initially no mapping
	_, ok := store.UserMapping(ctx, guildID, githubUser1)
	if ok {
		t.Error("UserMapping() found non-existent mapping")
	}

	// Save mapping
	info1 := UserMappingInfo{
		GitHubUsername: githubUser1,
		DiscordUserID:  "discord-123",
		GuildID:        guildID,
		CreatedAt:      time.Now(),
	}
	if err := store.SaveUserMapping(ctx, guildID, info1); err != nil {
		t.Fatalf("SaveUserMapping() error = %v", err)
	}

	// Retrieve mapping
	got, ok := store.UserMapping(ctx, guildID, githubUser1)
	if !ok {
		t.Fatal("UserMapping() did not find saved mapping")
	}
	if got.GitHubUsername != githubUser1 {
		t.Errorf("UserMapping().GitHubUsername = %q, want %q", got.GitHubUsername, githubUser1)
	}
	if got.DiscordUserID != "discord-123" {
		t.Errorf("UserMapping().DiscordUserID = %q, want %q", got.DiscordUserID, "discord-123")
	}
	if got.GuildID != guildID {
		t.Errorf("UserMapping().GuildID = %q, want %q", got.GuildID, guildID)
	}

	// Save second mapping
	info2 := UserMappingInfo{
		GitHubUsername: githubUser2,
		DiscordUserID:  "discord-456",
		GuildID:        guildID,
		CreatedAt:      time.Now(),
	}
	if err := store.SaveUserMapping(ctx, guildID, info2); err != nil {
		t.Fatalf("SaveUserMapping() error = %v", err)
	}

	// List mappings for guild
	mappings := store.ListUserMappings(ctx, guildID)
	if len(mappings) != 2 {
		t.Fatalf("ListUserMappings() returned %d mappings, want 2", len(mappings))
	}

	// Verify both mappings are present
	foundUsers := make(map[string]bool)
	for _, m := range mappings {
		foundUsers[m.GitHubUsername] = true
		if m.GuildID != guildID {
			t.Errorf("ListUserMappings() mapping has GuildID %q, want %q", m.GuildID, guildID)
		}
	}
	if !foundUsers[githubUser1] {
		t.Error("ListUserMappings() should include octocat")
	}
	if !foundUsers[githubUser2] {
		t.Error("ListUserMappings() should include torvalds")
	}

	// Different guild should have no mappings
	mappings2 := store.ListUserMappings(ctx, "different-guild")
	if len(mappings2) != 0 {
		t.Errorf("ListUserMappings(different-guild) returned %d mappings, want 0", len(mappings2))
	}

	// Update existing mapping
	info1Updated := UserMappingInfo{
		GitHubUsername: githubUser1,
		DiscordUserID:  "discord-789",
		GuildID:        guildID,
		CreatedAt:      time.Now(),
	}
	if err := store.SaveUserMapping(ctx, guildID, info1Updated); err != nil {
		t.Fatalf("SaveUserMapping() update error = %v", err)
	}

	// Verify update
	got, ok = store.UserMapping(ctx, guildID, githubUser1)
	if !ok {
		t.Fatal("UserMapping() did not find updated mapping")
	}
	if got.DiscordUserID != "discord-789" {
		t.Errorf("UserMapping().DiscordUserID = %q, want %q after update", got.DiscordUserID, "discord-789")
	}

	// Still only 2 mappings (update, not insert)
	mappings = store.ListUserMappings(ctx, guildID)
	if len(mappings) != 2 {
		t.Errorf("ListUserMappings() after update returned %d mappings, want 2", len(mappings))
	}
}
