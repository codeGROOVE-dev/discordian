// Package notify handles DM notification scheduling and delivery.
package notify

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/codeGROOVE-dev/discordian/internal/state"
)

const (
	checkInterval     = time.Minute
	minDMInterval     = time.Minute // Minimum time between DMs to same user
	dmRetentionPeriod = 90 * 24 * time.Hour
)

// DiscordDMSender defines the interface for sending DMs.
type DiscordDMSender interface {
	SendDM(ctx context.Context, userID, text string) (channelID, messageID string, err error)
}

// Manager handles pending DM notifications.
type Manager struct {
	store      state.Store
	logger     *slog.Logger
	dmSenders  map[string]DiscordDMSender // guildID -> sender
	lastDMTime map[string]time.Time       // userID -> last DM time
	stopCh     chan struct{}
	mu         sync.RWMutex
	wg         sync.WaitGroup
}

// New creates a new notification manager.
func New(store state.Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}

	return &Manager{
		store:      store,
		dmSenders:  make(map[string]DiscordDMSender),
		logger:     logger,
		lastDMTime: make(map[string]time.Time),
		stopCh:     make(chan struct{}),
	}
}

// RegisterGuild registers a Discord client for a guild.
func (m *Manager) RegisterGuild(guildID string, sender DiscordDMSender) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dmSenders[guildID] = sender
}

// Start begins the notification processing loop.
func (m *Manager) Start(ctx context.Context) {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.run(ctx)
	}()
}

// Stop stops the notification manager.
func (m *Manager) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

func (m *Manager) run(ctx context.Context) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.processPendingDMs(ctx)
		}
	}
}

func (m *Manager) processPendingDMs(ctx context.Context) {
	// Get DMs ready to send
	dms, err := m.store.PendingDMs(ctx, time.Now())
	if err != nil {
		m.logger.Error("failed to fetch pending DMs", "error", err)
		return
	}

	if len(dms) == 0 {
		return
	}

	m.logger.Debug("processing pending DMs", "count", len(dms))

	for _, dm := range dms {
		if err := m.sendDM(ctx, dm); err != nil {
			m.logger.Error("failed to send DM",
				"error", err,
				"user_id", dm.UserID,
				"pr_url", dm.PRURL)
			// Keep in queue to retry later
			continue
		}

		// Remove from queue
		if err := m.store.RemovePendingDM(ctx, dm.ID); err != nil {
			m.logger.Warn("failed to remove pending DM", "error", err, "id", dm.ID)
		}
	}
}

func (m *Manager) sendDM(ctx context.Context, dm *state.PendingDM) error {
	// Check rate limit
	m.mu.RLock()
	lastTime := m.lastDMTime[dm.UserID]
	m.mu.RUnlock()

	if time.Since(lastTime) < minDMInterval {
		m.logger.Debug("rate limiting DM",
			"user_id", dm.UserID,
			"last_dm", lastTime)
		return nil // Will retry next cycle
	}

	// Get sender for guild
	m.mu.RLock()
	sender := m.dmSenders[dm.GuildID]
	m.mu.RUnlock()

	if sender == nil {
		m.logger.Warn("no sender for guild", "guild_id", dm.GuildID)
		return nil // Remove from queue
	}

	// Send DM
	channelID, messageID, err := sender.SendDM(ctx, dm.UserID, dm.MessageText)
	if err != nil {
		return err
	}

	// Update rate limit tracker
	m.mu.Lock()
	m.lastDMTime[dm.UserID] = time.Now()
	m.mu.Unlock()

	// Save DM info for potential updates
	dmInfo := state.DMInfo{
		ChannelID:   channelID,
		MessageID:   messageID,
		MessageText: dm.MessageText,
		SentAt:      time.Now(),
	}
	if err := m.store.SaveDMInfo(ctx, dm.UserID, dm.PRURL, dmInfo); err != nil {
		m.logger.Warn("failed to save DM info", "error", err)
	}

	m.logger.Info("sent DM notification",
		"user_id", dm.UserID,
		"pr_url", dm.PRURL)

	return nil
}

// Tracker tracks notifications for anti-spam and deduplication.
type Tracker struct {
	tagged     map[string]map[string]bool // prURL -> userID -> tagged
	lastDMTime map[string]time.Time       // userID:prURL -> time
	mu         sync.RWMutex
}

// NewTracker creates a new notification tracker.
func NewTracker() *Tracker {
	return &Tracker{
		tagged:     make(map[string]map[string]bool),
		lastDMTime: make(map[string]time.Time),
	}
}

// WasTaggedInChannel returns whether a user was tagged in channel for a PR.
func (t *Tracker) WasTaggedInChannel(prURL, userID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.tagged[prURL] == nil {
		return false
	}
	return t.tagged[prURL][userID]
}

// MarkTaggedInChannel marks a user as tagged in channel for a PR.
func (t *Tracker) MarkTaggedInChannel(prURL, userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tagged[prURL] == nil {
		t.tagged[prURL] = make(map[string]bool)
	}
	t.tagged[prURL][userID] = true
}

// LastDMTime returns when a user was last DM'd about a PR.
func (t *Tracker) LastDMTime(userID, prURL string) time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := userID + ":" + prURL
	return t.lastDMTime[key]
}

// MarkDMSent marks that a DM was sent.
func (t *Tracker) MarkDMSent(userID, prURL string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := userID + ":" + prURL
	t.lastDMTime[key] = time.Now()
}
