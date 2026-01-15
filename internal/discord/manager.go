package discord

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// GuildManager manages Discord clients for multiple guilds.
type GuildManager struct {
	logger  *slog.Logger
	clients map[string]*Client // guildID -> client
	mu      sync.RWMutex
}

// NewGuildManager creates a new guild manager.
func NewGuildManager(logger *slog.Logger) *GuildManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &GuildManager{
		logger:  logger,
		clients: make(map[string]*Client),
	}
}

// RegisterClient registers a Discord client for a guild.
func (m *GuildManager) RegisterClient(guildID string, client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.clients[guildID] = client
	m.logger.Info("registered Discord client",
		"guild_id", guildID)
}

// Client returns the Discord client for a guild.
func (m *GuildManager) Client(guildID string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[guildID]
	return client, ok
}

// RemoveClient removes a Discord client for a guild.
func (m *GuildManager) RemoveClient(guildID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[guildID]; ok {
		if err := client.Close(); err != nil {
			m.logger.Warn("failed to close Discord client",
				"guild_id", guildID,
				"error", err)
		}
		delete(m.clients, guildID)
		m.logger.Info("removed Discord client",
			"guild_id", guildID)
	}
}

// GuildIDs returns all registered guild IDs.
func (m *GuildManager) GuildIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.clients))
	for id := range m.clients {
		ids = append(ids, id)
	}
	return ids
}

// Close closes all Discord clients.
func (m *GuildManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for guildID, client := range m.clients {
		if err := client.Close(); err != nil {
			m.logger.Warn("failed to close Discord client",
				"guild_id", guildID,
				"error", err)
			errs = append(errs, err)
		}
	}
	m.clients = make(map[string]*Client)

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// ForEach calls the given function for each registered client.
func (m *GuildManager) ForEach(fn func(guildID string, client *Client)) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for guildID, client := range m.clients {
		fn(guildID, client)
	}
}

// ClientFromToken creates and registers a new client from a bot token.
func (m *GuildManager) ClientFromToken(ctx context.Context, guildID, token string) (*Client, error) {
	client, err := New(token)
	if err != nil {
		return nil, err
	}

	client.SetGuildID(guildID)

	if err := client.Open(); err != nil {
		return nil, err
	}

	m.RegisterClient(guildID, client)
	return client, nil
}
