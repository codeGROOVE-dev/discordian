// Package usermapping provides GitHub to Discord user mapping.
package usermapping

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// DiscordLookup defines the interface for Discord user lookup.
type DiscordLookup interface {
	LookupUserByUsername(ctx context.Context, username string) string
}

// ConfigLookup defines the interface for config-based user lookup.
type ConfigLookup interface {
	DiscordUserID(org, githubUsername string) string
}

// Mapper maps GitHub usernames to Discord user IDs.
type Mapper struct {
	configLookup  ConfigLookup
	discordLookup DiscordLookup
	cache         map[string]string
	org           string
	mu            sync.RWMutex
}

// New creates a new user mapper.
func New(org string, configLookup ConfigLookup, discordLookup DiscordLookup) *Mapper {
	return &Mapper{
		org:           org,
		configLookup:  configLookup,
		discordLookup: discordLookup,
		cache:         make(map[string]string),
	}
}

// DiscordID returns the Discord user ID for a GitHub username.
// Uses a 3-tier lookup:
// 1. YAML config mapping (explicit)
// 2. Discord guild username match
// 3. Empty string (fallback).
func (m *Mapper) DiscordID(ctx context.Context, githubUsername string) string {
	// Check cache first
	m.mu.RLock()
	if id, ok := m.cache[githubUsername]; ok {
		m.mu.RUnlock()
		return id
	}
	m.mu.RUnlock()

	// Tier 1: YAML config mapping
	if m.configLookup != nil {
		if id := m.configLookup.DiscordUserID(m.org, githubUsername); id != "" {
			m.cacheResult(githubUsername, id)
			slog.Debug("mapped user via config",
				"github", githubUsername,
				"discord_id", id)
			return id
		}
	}

	// Tier 2: Discord username match
	if m.discordLookup != nil {
		if id := m.discordLookup.LookupUserByUsername(ctx, githubUsername); id != "" {
			m.cacheResult(githubUsername, id)
			slog.Debug("mapped user via Discord lookup",
				"github", githubUsername,
				"discord_id", id)
			return id
		}
	}

	// Tier 3: No mapping found
	slog.Debug("no Discord mapping found",
		"github", githubUsername)
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
	m.cache[githubUsername] = discordID
}

// ClearCache clears the user mapping cache.
func (m *Mapper) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]string)
}
