package bot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/codeGROOVE-dev/discordian/internal/format"
	"github.com/codeGROOVE-dev/discordian/internal/state"
	"github.com/google/uuid"
)

const (
	eventDeduplicationTTL = time.Hour
	maxConcurrentEvents   = 10
)

// Coordinator orchestrates event processing for a GitHub organization.
type Coordinator struct {
	discord    DiscordClient
	config     ConfigManager
	store      StateStore
	turn       TurnClient
	userMapper UserMapper
	logger     *slog.Logger
	eventSem   chan struct{}
	tagTracker *tagTracker
	org        string
	wg         sync.WaitGroup
}

// CoordinatorConfig holds configuration for creating a coordinator.
type CoordinatorConfig struct {
	Discord    DiscordClient
	Config     ConfigManager
	Store      StateStore
	Turn       TurnClient
	UserMapper UserMapper
	Logger     *slog.Logger
	Org        string
}

// NewCoordinator creates a new coordinator for an organization.
func NewCoordinator(cfg CoordinatorConfig) *Coordinator {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Coordinator{
		org:        cfg.Org,
		discord:    cfg.Discord,
		config:     cfg.Config,
		store:      cfg.Store,
		turn:       cfg.Turn,
		userMapper: cfg.UserMapper,
		logger:     logger.With("org", cfg.Org),
		eventSem:   make(chan struct{}, maxConcurrentEvents),
		tagTracker: newTagTracker(),
	}
}

// ProcessEvent handles an incoming sprinkler event.
func (c *Coordinator) ProcessEvent(ctx context.Context, event SprinklerEvent) {
	// Acquire semaphore
	select {
	case c.eventSem <- struct{}{}:
	case <-ctx.Done():
		return
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer func() { <-c.eventSem }()

		if err := c.processEventSync(ctx, event); err != nil {
			c.logger.Error("failed to process event",
				"error", err,
				"url", event.URL,
				"type", event.Type)
		}
	}()
}

func (c *Coordinator) processEventSync(ctx context.Context, event SprinklerEvent) error {
	// Parse PR URL
	owner, repo, number, ok := ParsePRURL(event.URL)
	if !ok {
		return fmt.Errorf("invalid PR URL: %s", event.URL)
	}

	// Auto-reload config when .codeGROOVE repo is updated
	if repo == ".codeGROOVE" {
		c.logger.Info("config repo updated, reloading config", "org", c.org)
		if err := c.config.ReloadConfig(ctx, c.org); err != nil {
			c.logger.Warn("failed to reload config", "error", err)
		}
		return nil // Don't post notifications for config repo PRs
	}

	// Deduplicate
	eventKey := fmt.Sprintf("%s:%s", event.DeliveryID, event.URL)
	if c.store.WasProcessed(ctx, eventKey) {
		c.logger.Debug("skipping duplicate event", "delivery_id", event.DeliveryID)
		return nil
	}

	c.logger.Info("processing event",
		"type", event.Type,
		"repo", repo,
		"number", number)

	// Load config
	if err := c.config.LoadConfig(ctx, c.org); err != nil {
		c.logger.Warn("failed to load config, using defaults", "error", err)
	}

	// Call Turn API for PR analysis
	checkResp, err := c.turn.Check(ctx, event.URL, "", event.Timestamp)
	if err != nil {
		c.logger.Warn("turn API call failed", "error", err)
		// Continue with limited info
		checkResp = &CheckResponse{}
	}

	// Determine state
	prState := format.StateFromAnalysis(
		checkResp.PullRequest.Merged,
		checkResp.PullRequest.Closed,
		checkResp.PullRequest.Draft,
		checkResp.Analysis.WorkflowState,
		checkResp.Analysis.Checks.Failing,
		checkResp.Analysis.MergeConflict,
	)

	// Build action users
	actionUsers := c.buildActionUsers(ctx, checkResp)

	// Get channels for this repo
	channels := c.config.ChannelsForRepo(c.org, repo)
	if len(channels) == 0 {
		c.logger.Warn("no channels found for repo - check that a channel named the same as the repo exists in Discord",
			"repo", repo,
			"org", c.org)
		return nil
	}

	// Process each channel
	for _, channelName := range channels {
		if err := c.processChannel(ctx, channelName, owner, repo, number, checkResp, prState, actionUsers); err != nil {
			c.logger.Error("failed to process channel",
				"channel", channelName,
				"error", err)
		}
	}

	// Queue DM notifications
	c.queueDMNotifications(ctx, owner, repo, number, checkResp, prState, actionUsers)

	// Mark as processed
	if err := c.store.MarkProcessed(ctx, eventKey, eventDeduplicationTTL); err != nil {
		c.logger.Warn("failed to mark event processed", "error", err)
	}

	return nil
}

func (c *Coordinator) buildActionUsers(ctx context.Context, checkResp *CheckResponse) []format.ActionUser {
	var users []format.ActionUser

	for username, action := range checkResp.Analysis.NextAction {
		mention := username
		if c.userMapper != nil {
			mention = c.userMapper.Mention(ctx, username)
		}

		users = append(users, format.ActionUser{
			Username: username,
			Mention:  mention,
			Action:   format.ActionLabel(action.Action),
		})
	}

	return users
}

func (c *Coordinator) processChannel(
	ctx context.Context,
	channelName string,
	owner, repo string,
	number int,
	checkResp *CheckResponse,
	prState format.PRState,
	actionUsers []format.ActionUser,
) error {
	// Resolve channel ID
	channelID := c.discord.ResolveChannelID(ctx, channelName)
	if channelID == channelName {
		// Resolution failed, channel doesn't exist
		c.logger.Debug("channel not found", "channel", channelName)
		return nil
	}

	// Check if bot can send to channel
	if !c.discord.IsBotInChannel(ctx, channelID) {
		c.logger.Debug("bot not in channel", "channel", channelName)
		return nil
	}

	// Build message params
	prURL := FormatPRURL(owner, repo, number)
	params := format.ChannelMessageParams{
		Owner:       owner,
		Repo:        repo,
		Number:      number,
		Title:       checkResp.PullRequest.Title,
		Author:      checkResp.PullRequest.Author,
		State:       prState,
		ActionUsers: actionUsers,
		PRURL:       prURL,
	}

	// Check for existing thread/message
	threadInfo, exists := c.store.Thread(ctx, owner, repo, number, channelID)

	// Auto-detect forum channels from Discord API
	if c.discord.IsForumChannel(ctx, channelID) {
		return c.processForumChannel(ctx, channelID, owner, repo, number, params, threadInfo, exists)
	}

	return c.processTextChannel(ctx, channelID, owner, repo, number, params, threadInfo, exists)
}

func (c *Coordinator) processForumChannel(
	ctx context.Context,
	channelID string,
	owner, repo string,
	number int,
	params format.ChannelMessageParams,
	threadInfo state.ThreadInfo,
	exists bool,
) error {
	title := format.ForumThreadTitle(params.Repo, params.Number, params.Title)
	content := format.ForumThreadContent(params)

	if exists && threadInfo.ThreadID != "" {
		// Update existing thread
		err := c.discord.UpdateForumPost(ctx, threadInfo.ThreadID, threadInfo.MessageID, title, content)
		if err == nil {
			// Update state
			threadInfo.MessageText = content
			threadInfo.LastState = string(params.State)
			if saveErr := c.store.SaveThread(ctx, owner, repo, number, channelID, threadInfo); saveErr != nil {
				c.logger.Warn("failed to save thread info", "error", saveErr)
			}

			// Archive if merged/closed
			if params.State == format.StateMerged || params.State == format.StateClosed {
				if archiveErr := c.discord.ArchiveThread(ctx, threadInfo.ThreadID); archiveErr != nil {
					c.logger.Warn("failed to archive thread", "error", archiveErr)
				}
			}

			c.trackTaggedUsers(params)
			return nil
		}
		c.logger.Warn("failed to update forum post, creating new", "error", err)
	}

	// Create new forum thread
	threadID, messageID, err := c.discord.PostForumThread(ctx, channelID, title, content)
	if err != nil {
		return fmt.Errorf("create forum thread: %w", err)
	}

	// Save thread info
	newInfo := state.ThreadInfo{
		ThreadID:    threadID,
		MessageID:   messageID,
		ChannelID:   channelID,
		ChannelType: "forum",
		LastState:   string(params.State),
		MessageText: content,
	}
	if err := c.store.SaveThread(ctx, owner, repo, number, channelID, newInfo); err != nil {
		c.logger.Warn("failed to save thread info", "error", err)
	}

	c.trackTaggedUsers(params)
	return nil
}

func (c *Coordinator) processTextChannel(
	ctx context.Context,
	channelID string,
	owner, repo string,
	number int,
	params format.ChannelMessageParams,
	threadInfo state.ThreadInfo,
	exists bool,
) error {
	content := format.ChannelMessage(params)

	if exists && threadInfo.MessageID != "" {
		// Update existing message
		err := c.discord.UpdateMessage(ctx, channelID, threadInfo.MessageID, content)
		if err == nil {
			threadInfo.MessageText = content
			threadInfo.LastState = string(params.State)
			if saveErr := c.store.SaveThread(ctx, owner, repo, number, channelID, threadInfo); saveErr != nil {
				c.logger.Warn("failed to save thread info", "error", saveErr)
			}

			c.trackTaggedUsers(params)
			return nil
		}
		c.logger.Warn("failed to update message, creating new", "error", err)
	}

	// Create new message
	messageID, err := c.discord.PostMessage(ctx, channelID, content)
	if err != nil {
		return fmt.Errorf("post message: %w", err)
	}

	// Save message info
	newInfo := state.ThreadInfo{
		MessageID:   messageID,
		ChannelID:   channelID,
		ChannelType: "text",
		LastState:   string(params.State),
		MessageText: content,
	}
	if err := c.store.SaveThread(ctx, owner, repo, number, channelID, newInfo); err != nil {
		c.logger.Warn("failed to save thread info", "error", err)
	}

	c.trackTaggedUsers(params)
	return nil
}

func (c *Coordinator) trackTaggedUsers(params format.ChannelMessageParams) {
	prURL := params.PRURL
	for _, au := range params.ActionUsers {
		// Only track if we have a Discord ID (mention contains <@)
		if au.Mention != "" && au.Mention[0] == '<' {
			c.tagTracker.mark(prURL, au.Username)
		}
	}
}

func (c *Coordinator) queueDMNotifications(
	ctx context.Context,
	owner, repo string,
	number int,
	checkResp *CheckResponse,
	prState format.PRState,
	_ []format.ActionUser,
) {
	// Skip DMs for merged/closed PRs
	if prState == format.StateMerged || prState == format.StateClosed {
		return
	}

	prURL := FormatPRURL(owner, repo, number)

	for username, action := range checkResp.Analysis.NextAction {
		// Get Discord ID
		var discordID string
		if c.userMapper != nil {
			discordID = c.userMapper.DiscordID(ctx, username)
		}
		if discordID == "" {
			c.logger.Debug("skipping DM - no Discord mapping",
				"github_user", username)
			continue
		}

		// Check if user is in guild
		if !c.discord.IsUserInGuild(ctx, discordID) {
			c.logger.Debug("skipping DM - user not in guild",
				"github_user", username,
				"discord_id", discordID)
			continue
		}

		// Get delay from config (any channel will do, use first one found)
		channels := c.config.ChannelsForRepo(c.org, repo)
		delay := 65 // default
		if len(channels) > 0 {
			delay = c.config.ReminderDMDelay(c.org, channels[0])
		}

		if delay == 0 {
			c.logger.Debug("skipping DM - notifications disabled",
				"github_user", username,
				"repo", repo)
			continue
		}

		// Build DM message
		params := format.ChannelMessageParams{
			Owner:  owner,
			Repo:   repo,
			Number: number,
			Title:  checkResp.PullRequest.Title,
			Author: checkResp.PullRequest.Author,
			State:  prState,
			PRURL:  prURL,
		}
		message := format.DMMessage(params, format.ActionLabel(action.Action))

		// Calculate send time
		sendAt := time.Now()
		if c.tagTracker.wasTagged(prURL, username) {
			// User was tagged in channel, delay DM
			sendAt = sendAt.Add(time.Duration(delay) * time.Minute)
		}
		// If not tagged, send immediately (sendAt is now)

		// Queue the DM
		dm := &state.PendingDM{
			ID:          uuid.New().String(),
			UserID:      discordID,
			PRURL:       prURL,
			MessageText: message,
			SendAt:      sendAt,
			GuildID:     c.discord.GuildID(),
			Org:         c.org,
		}

		if err := c.store.QueuePendingDM(ctx, dm); err != nil {
			c.logger.Warn("failed to queue DM",
				"error", err,
				"user", username)
		} else {
			c.logger.Debug("queued DM notification",
				"user", username,
				"discord_id", discordID,
				"send_at", sendAt)
		}
	}
}

// Wait waits for all pending event processing to complete.
func (c *Coordinator) Wait() {
	c.wg.Wait()
}

// tagTracker tracks which users were tagged in channel messages.
type tagTracker struct {
	tagged map[string]map[string]bool // prURL -> username -> tagged
	mu     sync.RWMutex
}

func newTagTracker() *tagTracker {
	return &tagTracker{
		tagged: make(map[string]map[string]bool),
	}
}

func (t *tagTracker) mark(prURL, username string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tagged[prURL] == nil {
		t.tagged[prURL] = make(map[string]bool)
	}
	t.tagged[prURL][username] = true
}

func (t *tagTracker) wasTagged(prURL, username string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.tagged[prURL] == nil {
		return false
	}
	return t.tagged[prURL][username]
}
