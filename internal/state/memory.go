package state

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// MemoryStore provides an in-memory implementation of Store.
type MemoryStore struct {
	threads      map[string]ThreadInfo
	dmInfo       map[string]DMInfo
	dmUserIndex  map[string]map[string]bool // prURL -> userIDs who received DMs
	processed    map[string]time.Time
	pendingDMs   map[string]*PendingDM
	dailyReports map[string]DailyReportInfo
	userMappings map[string]UserMappingInfo // guildID:gitHubUsername -> UserMappingInfo
	claims       map[string]time.Time       // claimKey -> expiry time
	mu           sync.RWMutex
	threadRetain time.Duration
	dmRetain     time.Duration
	eventRetain  time.Duration
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		threads:      make(map[string]ThreadInfo),
		dmInfo:       make(map[string]DMInfo),
		dmUserIndex:  make(map[string]map[string]bool),
		processed:    make(map[string]time.Time),
		pendingDMs:   make(map[string]*PendingDM),
		dailyReports: make(map[string]DailyReportInfo),
		userMappings: make(map[string]UserMappingInfo),
		claims:       make(map[string]time.Time),
		threadRetain: 30 * 24 * time.Hour, // 30 days
		dmRetain:     90 * 24 * time.Hour, // 90 days
		eventRetain:  24 * time.Hour,      // 1 day
	}
}

func threadKey(owner, repo string, number int, channelID string) string {
	return fmt.Sprintf("%s/%s#%d:%s", owner, repo, number, channelID)
}

func dmKey(userID, prURL string) string {
	return fmt.Sprintf("%s:%s", userID, prURL)
}

func userMappingKey(guildID, gitHubUsername string) string {
	return fmt.Sprintf("%s:%s", guildID, gitHubUsername)
}

// Thread returns thread info for a PR in a channel.
func (s *MemoryStore) Thread(ctx context.Context, owner, repo string, number int, channelID string) (ThreadInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.threads[threadKey(owner, repo, number, channelID)]
	return info, exists
}

// SaveThread saves thread info for a PR.
func (s *MemoryStore) SaveThread(ctx context.Context, owner, repo string, number int, channelID string, info ThreadInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info.UpdatedAt = time.Now()
	s.threads[threadKey(owner, repo, number, channelID)] = info

	slog.Debug("saved thread info",
		"owner", owner,
		"repo", repo,
		"number", number,
		"channel_id", channelID,
		"thread_id", info.ThreadID)

	return nil
}

// ClaimThread attempts to claim a thread for creation.
// Returns true if the claim was successful, false if another goroutine already claimed it.
func (s *MemoryStore) ClaimThread(ctx context.Context, owner, repo string, number int, channelID string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	claimKey := fmt.Sprintf("claim:thread:%s", threadKey(owner, repo, number, channelID))

	// Check if already claimed and not expired
	if expiry, exists := s.claims[claimKey]; exists && time.Now().Before(expiry) {
		slog.Debug("thread already claimed",
			"owner", owner,
			"repo", repo,
			"number", number,
			"channel_id", channelID)
		return false
	}

	// Claim it
	s.claims[claimKey] = time.Now().Add(ttl)
	slog.Debug("successfully claimed thread",
		"owner", owner,
		"repo", repo,
		"number", number,
		"channel_id", channelID,
		"ttl", ttl)
	return true
}

// DMInfo returns DM info for a user/PR combination.
func (s *MemoryStore) DMInfo(ctx context.Context, userID, prURL string) (DMInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.dmInfo[dmKey(userID, prURL)]
	return info, exists
}

// SaveDMInfo saves DM info.
func (s *MemoryStore) SaveDMInfo(_ context.Context, userID, prURL string, info DMInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.dmInfo[dmKey(userID, prURL)] = info

	// Update reverse index
	if s.dmUserIndex[prURL] == nil {
		s.dmUserIndex[prURL] = make(map[string]bool)
	}
	s.dmUserIndex[prURL][userID] = true

	slog.Debug("saved DM info",
		"user_id", userID,
		"pr_url", prURL,
		"channel_id", info.ChannelID)

	return nil
}

// ClaimDM attempts to claim a DM for sending.
// Returns true if the claim was successful, false if another goroutine already claimed it.
func (s *MemoryStore) ClaimDM(ctx context.Context, userID, prURL string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	claimKey := fmt.Sprintf("claim:dm:%s", dmKey(userID, prURL))

	// Check if already claimed and not expired
	if expiry, exists := s.claims[claimKey]; exists && time.Now().Before(expiry) {
		slog.Debug("DM already claimed",
			"user_id", userID,
			"pr_url", prURL)
		return false
	}

	// Claim it
	s.claims[claimKey] = time.Now().Add(ttl)
	slog.Debug("successfully claimed DM",
		"user_id", userID,
		"pr_url", prURL,
		"ttl", ttl)
	return true
}

// ListDMUsers returns all user IDs who received DMs for a PR.
func (s *MemoryStore) ListDMUsers(_ context.Context, prURL string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := s.dmUserIndex[prURL]
	if users == nil {
		return nil
	}

	result := make([]string, 0, len(users))
	for userID := range users {
		result = append(result, userID)
	}
	return result
}

// WasProcessed checks if an event was already processed.
func (s *MemoryStore) WasProcessed(ctx context.Context, eventKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	processedAt, exists := s.processed[eventKey]
	if !exists {
		return false
	}

	// Check if entry has expired
	if time.Since(processedAt) > s.eventRetain {
		return false
	}

	return true
}

// MarkProcessed marks an event as processed.
func (s *MemoryStore) MarkProcessed(_ context.Context, eventKey string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.processed[eventKey] = time.Now()
	return nil
}

// QueuePendingDM adds a DM to the pending queue.
func (s *MemoryStore) QueuePendingDM(ctx context.Context, dm *PendingDM) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dm.CreatedAt = time.Now()
	s.pendingDMs[dm.ID] = dm

	slog.Debug("queued pending DM",
		"id", dm.ID,
		"user_id", dm.UserID,
		"pr_url", dm.PRURL,
		"send_at", dm.SendAt)

	return nil
}

// PendingDMs returns DMs scheduled before the given time.
func (s *MemoryStore) PendingDMs(ctx context.Context, before time.Time) ([]*PendingDM, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*PendingDM
	for _, dm := range s.pendingDMs {
		if dm.SendAt.Before(before) || dm.SendAt.Equal(before) {
			result = append(result, dm)
		}
	}
	return result, nil
}

// RemovePendingDM removes a DM from the queue.
func (s *MemoryStore) RemovePendingDM(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pendingDMs, id)
	return nil
}

// DailyReportInfo returns daily report info for a user.
func (s *MemoryStore) DailyReportInfo(_ context.Context, userID string) (DailyReportInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.dailyReports[userID]
	return info, exists
}

// SaveDailyReportInfo saves daily report info for a user.
func (s *MemoryStore) SaveDailyReportInfo(_ context.Context, userID string, info DailyReportInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.dailyReports[userID] = info
	return nil
}

// Cleanup removes old entries from the store.
func (s *MemoryStore) Cleanup(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var threadsCleaned, dmsCleaned, eventsCleaned int

	// Clean old threads
	for key, info := range s.threads {
		if now.Sub(info.UpdatedAt) > s.threadRetain {
			delete(s.threads, key)
			threadsCleaned++
		}
	}

	// Clean old DM info
	for key, info := range s.dmInfo {
		if now.Sub(info.SentAt) > s.dmRetain {
			delete(s.dmInfo, key)
			dmsCleaned++
		}
	}

	// Clean old processed events
	for key, processedAt := range s.processed {
		if now.Sub(processedAt) > s.eventRetain {
			delete(s.processed, key)
			eventsCleaned++
		}
	}

	// Clean expired claims
	var claimsCleaned int
	for key, expiry := range s.claims {
		if now.After(expiry) {
			delete(s.claims, key)
			claimsCleaned++
		}
	}

	if threadsCleaned > 0 || dmsCleaned > 0 || eventsCleaned > 0 || claimsCleaned > 0 {
		slog.Info("cleaned up old state entries",
			"threads", threadsCleaned,
			"dms", dmsCleaned,
			"events", eventsCleaned,
			"claims", claimsCleaned)
	}

	return nil
}

// UserMapping returns user mapping info for a GitHub username in a guild.
func (s *MemoryStore) UserMapping(ctx context.Context, guildID, gitHubUsername string) (UserMappingInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.userMappings[userMappingKey(guildID, gitHubUsername)]
	return info, exists
}

// SaveUserMapping saves user mapping info for a GitHub username.
func (s *MemoryStore) SaveUserMapping(ctx context.Context, guildID string, info UserMappingInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info.CreatedAt = time.Now()
	info.GuildID = guildID
	s.userMappings[userMappingKey(guildID, info.GitHubUsername)] = info

	slog.Info("saved user mapping",
		"guild_id", guildID,
		"github_username", info.GitHubUsername,
		"discord_user_id", info.DiscordUserID)

	return nil
}

// ListUserMappings returns all user mappings for a guild.
func (s *MemoryStore) ListUserMappings(ctx context.Context, guildID string) []UserMappingInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var mappings []UserMappingInfo
	for _, info := range s.userMappings {
		if info.GuildID == guildID {
			mappings = append(mappings, info)
		}
	}

	return mappings
}

// Close closes the store (no-op for memory store).
func (*MemoryStore) Close() error {
	return nil
}

// StoreStats contains store statistics.
type StoreStats struct {
	Threads int
	DMs     int
	Events  int
	Pending int
}

// Stats returns current store statistics.
func (s *MemoryStore) Stats() StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return StoreStats{
		Threads: len(s.threads),
		DMs:     len(s.dmInfo),
		Events:  len(s.processed),
		Pending: len(s.pendingDMs),
	}
}
