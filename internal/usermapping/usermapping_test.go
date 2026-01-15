package usermapping

import (
	"context"
	"testing"
)

type mockConfigLookup struct {
	users map[string]string
}

func (m *mockConfigLookup) DiscordUserID(_, githubUsername string) string {
	if m.users == nil {
		return ""
	}
	return m.users[githubUsername]
}

type mockDiscordLookup struct {
	users map[string]string
}

func (m *mockDiscordLookup) LookupUserByUsername(_ context.Context, username string) string {
	if m.users == nil {
		return ""
	}
	return m.users[username]
}

func TestMapper_DiscordID(t *testing.T) {
	ctx := context.Background()

	configLookup := &mockConfigLookup{
		users: map[string]string{
			"alice": "111111111111111111",
		},
	}

	discordLookup := &mockDiscordLookup{
		users: map[string]string{
			"bob":     "222222222222222222",
			"charlie": "333333333333333333",
		},
	}

	mapper := New("testorg", configLookup, discordLookup)

	tests := []struct {
		name           string
		githubUsername string
		want           string
		tier           string
	}{
		{
			name:           "config mapping (tier 1)",
			githubUsername: "alice",
			want:           "111111111111111111",
			tier:           "config",
		},
		{
			name:           "discord lookup (tier 2)",
			githubUsername: "bob",
			want:           "222222222222222222",
			tier:           "discord",
		},
		{
			name:           "no mapping (tier 3)",
			githubUsername: "unknown",
			want:           "",
			tier:           "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapper.DiscordID(ctx, tt.githubUsername)
			if got != tt.want {
				t.Errorf("DiscordID(%q) = %q, want %q", tt.githubUsername, got, tt.want)
			}
		})
	}
}

func TestMapper_DiscordID_ConfigOverridesDiscord(t *testing.T) {
	ctx := context.Background()

	// Same user in both config and Discord with different IDs
	configLookup := &mockConfigLookup{
		users: map[string]string{
			"alice": "111111111111111111",
		},
	}

	discordLookup := &mockDiscordLookup{
		users: map[string]string{
			"alice": "999999999999999999", // Different ID
		},
	}

	mapper := New("testorg", configLookup, discordLookup)

	// Config should take priority
	got := mapper.DiscordID(ctx, "alice")
	if got != "111111111111111111" {
		t.Errorf("DiscordID(alice) = %q, want config ID 111111111111111111", got)
	}
}

func TestMapper_DiscordID_Caching(t *testing.T) {
	ctx := context.Background()

	discordLookup := &mockDiscordLookup{
		users: map[string]string{
			"bob": "222222222222222222",
		},
	}

	mapper := New("testorg", nil, discordLookup)

	// First call - should hit Discord lookup
	id1 := mapper.DiscordID(ctx, "bob")
	if id1 != "222222222222222222" {
		t.Errorf("First DiscordID(bob) = %q, want 222222222222222222", id1)
	}

	// Change the underlying data
	discordLookup.users["bob"] = "999999999999999999"

	// Second call - should be cached (still returns old value)
	id2 := mapper.DiscordID(ctx, "bob")
	if id2 != id1 {
		t.Errorf("Second DiscordID(bob) = %q, want cached %q", id2, id1)
	}
}

func TestMapper_Mention(t *testing.T) {
	ctx := context.Background()

	configLookup := &mockConfigLookup{
		users: map[string]string{
			"alice": "111111111111111111",
		},
	}

	mapper := New("testorg", configLookup, nil)

	tests := []struct {
		name           string
		githubUsername string
		want           string
	}{
		{
			name:           "mapped user gets mention",
			githubUsername: "alice",
			want:           "<@111111111111111111>",
		},
		{
			name:           "unmapped user gets plain text",
			githubUsername: "unknown",
			want:           "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapper.Mention(ctx, tt.githubUsername)
			if got != tt.want {
				t.Errorf("Mention(%q) = %q, want %q", tt.githubUsername, got, tt.want)
			}
		})
	}
}

func TestMapper_ClearCache(t *testing.T) {
	ctx := context.Background()

	configLookup := &mockConfigLookup{
		users: map[string]string{
			"alice": "111111111111111111",
		},
	}

	mapper := New("testorg", configLookup, nil)

	// Populate cache
	mapper.DiscordID(ctx, "alice")

	// Change the underlying data
	configLookup.users["alice"] = "999999999999999999"

	// Still returns cached value
	if got := mapper.DiscordID(ctx, "alice"); got != "111111111111111111" {
		t.Errorf("Before ClearCache: DiscordID(alice) = %q, want cached 111111111111111111", got)
	}

	// Clear cache
	mapper.ClearCache()

	// Now returns new value
	if got := mapper.DiscordID(ctx, "alice"); got != "999999999999999999" {
		t.Errorf("After ClearCache: DiscordID(alice) = %q, want new 999999999999999999", got)
	}
}

func TestMapper_NilLookups(t *testing.T) {
	ctx := context.Background()

	// Both lookups nil
	mapper := New("testorg", nil, nil)

	got := mapper.DiscordID(ctx, "anyone")
	if got != "" {
		t.Errorf("DiscordID with nil lookups = %q, want empty", got)
	}

	mention := mapper.Mention(ctx, "anyone")
	if mention != "anyone" {
		t.Errorf("Mention with nil lookups = %q, want plain username", mention)
	}
}
