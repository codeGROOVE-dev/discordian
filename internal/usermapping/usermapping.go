// Package usermapping provides GitHub to Discord user mapping.
package usermapping

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/codeGROOVE-dev/discordian/internal/state"
)

const (
	// cacheTTL is how long cache entries are valid (same as slacker).
	cacheTTL = 24 * time.Hour
)

// DiscordLookup defines the interface for Discord user lookup.
type DiscordLookup interface {
	LookupUserByUsername(ctx context.Context, username string) string
}

// ConfigLookup defines the interface for config-based user lookup.
type ConfigLookup interface {
	DiscordUserID(org, githubUsername string) string
}

// cacheEntry stores a cached mapping with timestamp.
type cacheEntry struct {
	cachedAt  time.Time
	discordID string
}

// Mapper maps GitHub usernames to Discord user IDs.
type Mapper struct {
	configLookup  ConfigLookup
	discordLookup DiscordLookup
	store         state.Store
	guildID       string
	cache         map[string]cacheEntry
	org           string
	mu            sync.RWMutex
}

// New creates a new user mapper.
func New(org string, configLookup ConfigLookup, discordLookup DiscordLookup, store state.Store, guildID string) *Mapper {
	return &Mapper{
		org:           org,
		configLookup:  configLookup,
		discordLookup: discordLookup,
		store:         store,
		guildID:       guildID,
		cache:         make(map[string]cacheEntry),
	}
}

// DiscordID returns the Discord user ID for a GitHub username.
// Uses a 4-tier lookup:
// 1. YAML config mapping (explicit)
// 2. Fido storage (self-service via /goose github-user command)
// 3. Discord guild username match
// 4. Empty string (fallback).
// Results are cached for 24 hours.
func (m *Mapper) DiscordID(ctx context.Context, githubUsername string) string {
	// Check cache first (with TTL)
	m.mu.RLock()
	if entry, ok := m.cache[githubUsername]; ok {
		if time.Since(entry.cachedAt) < cacheTTL {
			m.mu.RUnlock()
			return entry.discordID
		}
		// Entry expired, will re-lookup below
		slog.Debug("cache entry expired, re-looking up",
			"github", githubUsername)
	}
	m.mu.RUnlock()

	// Tier 1: YAML config mapping
	if m.configLookup != nil {
		if configValue := m.configLookup.DiscordUserID(m.org, githubUsername); configValue != "" {
			// Check if config value is a numeric ID or a Discord username
			// Discord IDs are 17-20 digit snowflakes
			if len(configValue) >= 17 && len(configValue) <= 20 && isAllDigits(configValue) {
				// It's a numeric ID, use it directly
				m.cacheResult(githubUsername, configValue)
				slog.Info("mapped GitHub user to Discord via config (numeric ID)",
					"github_username", githubUsername,
					"discord_id", configValue,
					"org", m.org,
					"method", "config_numeric_id")
				return configValue
			}
			// It's a Discord username, resolve it to numeric ID
			if m.discordLookup != nil {
				if id := m.discordLookup.LookupUserByUsername(ctx, configValue); id != "" {
					m.cacheResult(githubUsername, id)
					slog.Info("mapped GitHub user to Discord via config (username resolved)",
						"github_username", githubUsername,
						"discord_username", configValue,
						"discord_id", id,
						"org", m.org,
						"method", "config_username_resolved")
					return id
				}
				slog.Warn("config specified Discord username not found in guild",
					"github_username", githubUsername,
					"discord_username", configValue,
					"org", m.org)
			}
		}
	}

	// Tier 2: Fido storage (self-service mappings)
	if m.store != nil && m.guildID != "" {
		if mapping, found := m.store.UserMapping(ctx, m.guildID, githubUsername); found {
			m.cacheResult(githubUsername, mapping.DiscordUserID)
			slog.Info("mapped GitHub user to Discord via Fido storage",
				"github_username", githubUsername,
				"discord_id", mapping.DiscordUserID,
				"guild_id", m.guildID,
				"org", m.org,
				"method", "fido_storage")
			return mapping.DiscordUserID
		}
	}

	// Tier 3: Discord username match
	if m.discordLookup != nil {
		if id := m.discordLookup.LookupUserByUsername(ctx, githubUsername); id != "" {
			m.cacheResult(githubUsername, id)
			slog.Info("mapped GitHub user to Discord via username match",
				"github_username", githubUsername,
				"discord_id", id,
				"org", m.org,
				"method", "discord_username_match")
			return id
		}
	}

	// Tier 4: No mapping found
	slog.Info("no Discord mapping found for GitHub user",
		"github_username", githubUsername,
		"org", m.org,
		"note", "user will not receive notifications")
	return ""
}

// Mention returns a Discord mention string for a GitHub username.
// Returns the username in plain text if no Discord ID is found.
func (m *Mapper) Mention(ctx context.Context, githubUsername string) string {
	if id := m.DiscordID(ctx, githubUsername); id != "" {
		return fmt.Sprintf("<@%s>", id)
	}
	return githubUsername
}

func (m *Mapper) cacheResult(githubUsername, discordID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache[githubUsername] = cacheEntry{
		discordID: discordID,
		cachedAt:  time.Now(),
	}
}

// isAllDigits returns true if the string is non-empty and contains only digit characters.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ClearCache clears the user mapping cache.
func (m *Mapper) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]cacheEntry)
}

// ExportCache returns a copy of the cache for inspection (githubUsername -> discordID).
func (m *Mapper) ExportCache() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(m.cache))
	for githubUsername, entry := range m.cache {
		result[githubUsername] = entry.discordID
	}
	return result
}
