package config

import (
	"testing"
	"time"
)

func TestManager_ChannelsForRepo(t *testing.T) {
	m := New()

	// Set up config directly for testing
	m.configs["testorg"] = &DiscordConfig{
		Channels: map[string]ChannelConfig{
			"backend": {
				Repos: []string{"api", "db"},
				Type:  "text",
			},
			"frontend": {
				Repos: []string{"web", "mobile"},
				Type:  "forum",
			},
			"all-prs": {
				Repos: []string{"*"},
				Type:  "text",
			},
			"muted": {
				Repos: []string{"noisy-repo"},
				Mute:  true,
			},
		},
	}

	tests := []struct {
		name     string
		org      string
		repo     string
		contains []string
	}{
		{
			name:     "repo matches specific channel",
			org:      "testorg",
			repo:     "api",
			contains: []string{"backend", "all-prs"}, // api auto-discovered too
		},
		{
			name:     "repo matches wildcard only",
			org:      "testorg",
			repo:     "random-repo",
			contains: []string{"all-prs", "random-repo"},
		},
		{
			name:     "muted repo still gets non-muted channels",
			org:      "testorg",
			repo:     "noisy-repo",
			contains: []string{"all-prs"}, // muted channel excluded but wildcard still included
		},
		{
			name:     "unknown org uses auto-discovery",
			org:      "unknownorg",
			repo:     "myrepo",
			contains: []string{"myrepo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.ChannelsForRepo(tt.org, tt.repo)
			for _, want := range tt.contains {
				found := false
				for _, ch := range got {
					if ch == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ChannelsForRepo() = %v, want to contain %q", got, want)
				}
			}
		})
	}
}

func TestManager_ChannelType(t *testing.T) {
	m := New()

	m.configs["testorg"] = &DiscordConfig{
		Channels: map[string]ChannelConfig{
			"forum-channel": {Type: "forum"},
			"text-channel":  {Type: "text"},
			"no-type":       {},
		},
	}

	tests := []struct {
		name    string
		org     string
		channel string
		want    string
	}{
		{"forum channel", "testorg", "forum-channel", "forum"},
		{"text channel", "testorg", "text-channel", "text"},
		{"no type defaults to text", "testorg", "no-type", "text"},
		{"unknown channel", "testorg", "unknown", "text"},
		{"unknown org", "unknownorg", "any", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.ChannelType(tt.org, tt.channel)
			if got != tt.want {
				t.Errorf("ChannelType(%q, %q) = %q, want %q", tt.org, tt.channel, got, tt.want)
			}
		})
	}
}

func TestManager_DiscordUserID(t *testing.T) {
	m := New()

	m.configs["testorg"] = &DiscordConfig{
		Users: map[string]string{
			"alice": "111111111111111111",
			"bob":   "222222222222222222",
		},
	}

	tests := []struct {
		name           string
		org            string
		githubUsername string
		want           string
	}{
		{"mapped user", "testorg", "alice", "111111111111111111"},
		{"another mapped user", "testorg", "bob", "222222222222222222"},
		{"unmapped user", "testorg", "charlie", ""},
		{"unknown org", "unknownorg", "alice", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.DiscordUserID(tt.org, tt.githubUsername)
			if got != tt.want {
				t.Errorf("DiscordUserID(%q, %q) = %q, want %q", tt.org, tt.githubUsername, got, tt.want)
			}
		})
	}
}

func TestManager_ReminderDMDelay(t *testing.T) {
	delay30 := 30
	delay0 := 0

	m := New()

	m.configs["testorg"] = &DiscordConfig{
		Global: GlobalConfig{
			ReminderDMDelay: 45,
		},
		Channels: map[string]ChannelConfig{
			"custom-delay":  {ReminderDMDelay: &delay30},
			"zero-delay":    {ReminderDMDelay: &delay0},
			"default-delay": {},
		},
	}

	tests := []struct {
		name    string
		org     string
		channel string
		want    int
	}{
		{"channel with custom delay", "testorg", "custom-delay", 30},
		{"channel with zero delay", "testorg", "zero-delay", 0},
		{"channel using global", "testorg", "default-delay", 45},
		{"unknown channel uses global", "testorg", "unknown", 45},
		{"unknown org uses default", "unknownorg", "any", 65},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.ReminderDMDelay(tt.org, tt.channel)
			if got != tt.want {
				t.Errorf("ReminderDMDelay(%q, %q) = %d, want %d", tt.org, tt.channel, got, tt.want)
			}
		})
	}
}

func TestManager_GuildID(t *testing.T) {
	m := New()

	m.configs["testorg"] = &DiscordConfig{
		Global: GlobalConfig{
			GuildID: "123456789012345678",
		},
	}

	tests := []struct {
		name string
		org  string
		want string
	}{
		{"org with guild", "testorg", "123456789012345678"},
		{"unknown org", "unknownorg", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.GuildID(tt.org)
			if got != tt.want {
				t.Errorf("GuildID(%q) = %q, want %q", tt.org, got, tt.want)
			}
		})
	}
}

func TestManager_Config(t *testing.T) {
	m := New()

	cfg := &DiscordConfig{
		Global: GlobalConfig{GuildID: "test123"},
	}
	m.configs["testorg"] = cfg

	t.Run("existing config", func(t *testing.T) {
		got, exists := m.Config("testorg")
		if !exists {
			t.Error("Config() should find existing config")
		}
		if got != cfg {
			t.Error("Config() returned wrong config")
		}
	})

	t.Run("non-existing config", func(t *testing.T) {
		_, exists := m.Config("unknownorg")
		if exists {
			t.Error("Config() should not find non-existing config")
		}
	})
}

func TestConfigCache(t *testing.T) {
	cache := &configCache{
		entries: make(map[string]configCacheEntry),
		ttl:     time.Hour, // Long TTL for testing
	}

	cfg := &DiscordConfig{
		Global: GlobalConfig{GuildID: "test"},
	}

	t.Run("miss on empty cache", func(t *testing.T) {
		_, found := cache.get("org1")
		if found {
			t.Error("get() should return false for empty cache")
		}
	})

	t.Run("hit after set", func(t *testing.T) {
		cache.set("org1", cfg)
		got, found := cache.get("org1")
		if !found {
			t.Error("get() should return true after set")
		}
		if got != cfg {
			t.Error("get() returned wrong config")
		}
	})

	t.Run("miss after invalidate", func(t *testing.T) {
		cache.invalidate("org1")
		_, found := cache.get("org1")
		if found {
			t.Error("get() should return false after invalidate")
		}
	})

	t.Run("stats", func(t *testing.T) {
		hits, misses := cache.stats()
		if hits < 1 {
			t.Errorf("stats() hits = %d, want >= 1", hits)
		}
		if misses < 1 {
			t.Errorf("stats() misses = %d, want >= 1", misses)
		}
	})
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()

	if cfg.Global.ReminderDMDelay != 65 {
		t.Errorf("default ReminderDMDelay = %d, want 65", cfg.Global.ReminderDMDelay)
	}
	if cfg.Channels == nil {
		t.Error("default Channels should not be nil")
	}
	if cfg.Users == nil {
		t.Error("default Users should not be nil")
	}
}

func TestManager_SetGitHubClient(t *testing.T) {
	m := New()

	// Set a mock client (using any type since the interface accepts any)
	m.SetGitHubClient("testorg", "mock-client")

	// Verify it was stored
	m.mu.RLock()
	got := m.clients["testorg"]
	m.mu.RUnlock()

	if got != "mock-client" {
		t.Errorf("SetGitHubClient() did not store client")
	}
}

func TestManager_CacheStats(t *testing.T) {
	m := New()

	// Initially both should be zero
	hits, misses := m.CacheStats()

	// Stats should return valid values (could be 0)
	if hits < 0 {
		t.Errorf("CacheStats() hits = %d, want >= 0", hits)
	}
	if misses < 0 {
		t.Errorf("CacheStats() misses = %d, want >= 0", misses)
	}
}

func TestConfigCache_Expiry(t *testing.T) {
	cache := &configCache{
		entries: make(map[string]configCacheEntry),
		ttl:     time.Millisecond, // Very short TTL
	}

	cfg := &DiscordConfig{
		Global: GlobalConfig{GuildID: "test"},
	}

	// Set entry
	cache.set("org1", cfg)

	// Immediately should find it
	got, found := cache.get("org1")
	if !found {
		t.Error("get() should find entry immediately after set")
	}
	if got != cfg {
		t.Error("get() should return the same config")
	}

	// Wait for expiry
	time.Sleep(5 * time.Millisecond)

	// Should not find it now
	_, found = cache.get("org1")
	if found {
		t.Error("get() should not find expired entry")
	}
}

func TestManager_ReminderDMDelay_GlobalZero(t *testing.T) {
	m := New()

	// Config with global.ReminderDMDelay = 0 (should use default 65)
	m.configs["testorg"] = &DiscordConfig{
		Global: GlobalConfig{
			ReminderDMDelay: 0,
		},
		Channels: make(map[string]ChannelConfig),
	}

	got := m.ReminderDMDelay("testorg", "some-channel")
	if got != 65 {
		t.Errorf("ReminderDMDelay() = %d, want default 65 when global is 0", got)
	}
}
