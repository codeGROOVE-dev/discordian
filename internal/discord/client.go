// Package discord provides Discord API client functionality.
package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"

	"github.com/codeGROOVE-dev/discordian/internal/format"
)

// Client wraps discordgo.Session with a clean interface for bot operations.
type Client struct {
	session          *discordgo.Session
	channelCache     map[string]string                // channel name -> ID
	channelTypeCache map[string]discordgo.ChannelType // channel ID -> type
	userCache        map[string]string                // username -> ID
	guildID          string
	mu               sync.RWMutex
}

// New creates a new Discord client for a specific guild.
func New(token string) (*Client, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	return &Client{
		session:          session,
		channelCache:     make(map[string]string),
		channelTypeCache: make(map[string]discordgo.ChannelType),
		userCache:        make(map[string]string),
	}, nil
}

// SetGuildID sets the guild ID for this client.
func (c *Client) SetGuildID(guildID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.guildID = guildID
}

// GuildID returns the current guild ID.
func (c *Client) GuildID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.guildID
}

// Open opens the WebSocket connection to Discord.
func (c *Client) Open() error {
	return c.session.Open()
}

// Close closes the WebSocket connection.
func (c *Client) Close() error {
	return c.session.Close()
}

// Session returns the underlying discordgo session.
func (c *Client) Session() *discordgo.Session {
	return c.session
}

// PostMessage sends a plain text message to a channel.
func (c *Client) PostMessage(ctx context.Context, channelID, text string) (string, error) {
	msg, err := c.session.ChannelMessageSend(channelID, text)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	slog.Debug("posted message",
		"channel_id", channelID,
		"message_id", msg.ID,
		"preview", format.Truncate(text, 50))

	return msg.ID, nil
}

// UpdateMessage edits an existing message.
func (c *Client) UpdateMessage(ctx context.Context, channelID, messageID, newText string) error {
	_, err := c.session.ChannelMessageEdit(channelID, messageID, newText)
	if err != nil {
		return fmt.Errorf("failed to edit message: %w", err)
	}

	slog.Debug("updated message",
		"channel_id", channelID,
		"message_id", messageID)

	return nil
}

// PostForumThread creates a forum post with title and content.
func (c *Client) PostForumThread(ctx context.Context, channelID, title, content string) (threadID, messageID string, err error) {
	thread, err := c.session.ForumThreadStartComplex(channelID, &discordgo.ThreadStart{
		Name: format.Truncate(title, 100), // Discord limits thread names
	}, &discordgo.MessageSend{Content: content})
	if err != nil {
		return "", "", fmt.Errorf("failed to create forum thread: %w", err)
	}

	c.mu.RLock()
	guildID := c.guildID
	c.mu.RUnlock()

	slog.Info("created forum thread",
		"guild_id", guildID,
		"channel_id", channelID,
		"thread_id", thread.ID,
		"title", title)

	// Get the first message in the thread to return its ID
	messages, err := c.session.ChannelMessages(thread.ID, 1, "", "", "")
	if err == nil && len(messages) > 0 {
		return thread.ID, messages[0].ID, nil
	}

	return thread.ID, "", nil
}

// UpdateForumPost updates both the thread title and starter message.
func (c *Client) UpdateForumPost(ctx context.Context, threadID, messageID, newTitle, newContent string) error {
	_, err := c.session.ChannelEdit(threadID, &discordgo.ChannelEdit{
		Name: format.Truncate(newTitle, 100),
	})
	if err != nil {
		return fmt.Errorf("failed to update thread title: %w", err)
	}

	if messageID != "" {
		_, err = c.session.ChannelMessageEdit(threadID, messageID, newContent)
		if err != nil {
			return fmt.Errorf("failed to update thread message: %w", err)
		}
	}

	slog.Debug("updated forum post",
		"thread_id", threadID,
		"new_title", newTitle)

	return nil
}

// ArchiveThread archives a forum thread.
func (c *Client) ArchiveThread(ctx context.Context, threadID string) error {
	archived := true
	_, err := c.session.ChannelEdit(threadID, &discordgo.ChannelEdit{
		Archived: &archived,
	})
	if err != nil {
		return fmt.Errorf("failed to archive thread: %w", err)
	}

	slog.Debug("archived thread", "thread_id", threadID)
	return nil
}

// SendDM sends a direct message to a user.
func (c *Client) SendDM(ctx context.Context, userID, text string) (channelID, messageID string, err error) {
	channel, err := c.session.UserChannelCreate(userID)
	if err != nil {
		return "", "", fmt.Errorf("failed to create DM channel: %w", err)
	}

	msg, err := c.session.ChannelMessageSend(channel.ID, text)
	if err != nil {
		return "", "", fmt.Errorf("failed to send DM: %w", err)
	}

	slog.Debug("sent DM",
		"user_id", userID,
		"channel_id", channel.ID,
		"message_id", msg.ID)

	return channel.ID, msg.ID, nil
}

// UpdateDM updates an existing DM message.
func (c *Client) UpdateDM(ctx context.Context, channelID, messageID, newText string) error {
	_, err := c.session.ChannelMessageEdit(channelID, messageID, newText)
	if err != nil {
		return fmt.Errorf("failed to update DM: %w", err)
	}
	return nil
}

// ResolveChannelID resolves a channel name to its ID.
func (c *Client) ResolveChannelID(ctx context.Context, channelName string) string {
	// If it looks like an ID already (long numeric string), return it
	if len(channelName) > 15 {
		isID := true
		for _, r := range channelName {
			if r < '0' || r > '9' {
				isID = false
				break
			}
		}
		if isID {
			return channelName
		}
	}

	// Check cache
	c.mu.RLock()
	if id, ok := c.channelCache[channelName]; ok {
		c.mu.RUnlock()
		return id
	}
	guildID := c.guildID
	c.mu.RUnlock()

	if guildID == "" {
		return channelName
	}

	channels, err := c.session.GuildChannels(guildID)
	if err != nil {
		slog.Warn("failed to fetch guild channels",
			"guild_id", guildID,
			"error", err)
		return channelName
	}

	name := strings.TrimPrefix(channelName, "#")

	for _, ch := range channels {
		if ch.Name != name {
			continue
		}

		c.mu.Lock()
		c.channelCache[channelName] = ch.ID
		c.mu.Unlock()

		slog.Debug("resolved channel",
			"name", channelName,
			"id", ch.ID)

		return ch.ID
	}

	slog.Debug("channel not found",
		"name", channelName,
		"guild_id", guildID)

	return channelName
}

// ChannelType returns the type of a channel (forum, text, etc.).
func (c *Client) ChannelType(ctx context.Context, channelID string) (discordgo.ChannelType, error) {
	// Check cache first
	c.mu.RLock()
	if channelType, ok := c.channelTypeCache[channelID]; ok {
		c.mu.RUnlock()
		return channelType, nil
	}
	c.mu.RUnlock()

	channel, err := c.session.Channel(channelID)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch channel: %w", err)
	}

	// Cache the result
	c.mu.Lock()
	c.channelTypeCache[channelID] = channel.Type
	c.mu.Unlock()

	return channel.Type, nil
}

// IsForumChannel returns true if the channel is a forum channel.
func (c *Client) IsForumChannel(ctx context.Context, channelID string) bool {
	channelType, err := c.ChannelType(ctx, channelID)
	if err != nil {
		slog.Debug("failed to check channel type", "channel_id", channelID, "error", err)
		return false
	}
	return channelType == discordgo.ChannelTypeGuildForum
}

// LookupUserByUsername finds a Discord user ID by exact username match.
func (c *Client) LookupUserByUsername(ctx context.Context, username string) string {
	c.mu.RLock()
	if id, ok := c.userCache[username]; ok {
		c.mu.RUnlock()
		return id
	}
	guildID := c.guildID
	c.mu.RUnlock()

	if guildID == "" {
		return ""
	}

	members, err := c.session.GuildMembers(guildID, "", 1000)
	if err != nil {
		slog.Warn("failed to fetch guild members",
			"guild_id", guildID,
			"error", err)
		return ""
	}

	for _, member := range members {
		if member.User.Username != username {
			continue
		}

		c.mu.Lock()
		c.userCache[username] = member.User.ID
		c.mu.Unlock()

		slog.Debug("found user by username",
			"username", username,
			"user_id", member.User.ID)

		return member.User.ID
	}

	slog.Debug("user not found",
		"username", username,
		"guild_id", guildID)

	return ""
}

// IsBotInChannel checks if the bot has permission to send messages in a channel.
func (c *Client) IsBotInChannel(ctx context.Context, channelID string) bool {
	if c.session.State == nil || c.session.State.User == nil {
		return false
	}

	perms, err := c.session.UserChannelPermissions(c.session.State.User.ID, channelID)
	if err != nil {
		slog.Debug("failed to check channel permissions",
			"channel_id", channelID,
			"error", err)
		return false
	}

	return perms&discordgo.PermissionSendMessages != 0
}

// IsUserInGuild checks if a user is a member of the guild.
func (c *Client) IsUserInGuild(ctx context.Context, userID string) bool {
	c.mu.RLock()
	guildID := c.guildID
	c.mu.RUnlock()

	if guildID == "" {
		return false
	}

	_, err := c.session.GuildMember(guildID, userID)
	return err == nil
}

// GuildInfo holds basic guild information.
type GuildInfo struct {
	ID   string
	Name string
}

// GuildInfo returns information about the current guild.
func (c *Client) GuildInfo(ctx context.Context) (GuildInfo, error) {
	c.mu.RLock()
	guildID := c.guildID
	c.mu.RUnlock()

	if guildID == "" {
		return GuildInfo{}, errors.New("no guild ID set")
	}

	guild, err := c.session.Guild(guildID)
	if err != nil {
		return GuildInfo{}, fmt.Errorf("failed to fetch guild: %w", err)
	}

	return GuildInfo{
		ID:   guild.ID,
		Name: guild.Name,
	}, nil
}

// BotInfo holds basic bot user information.
type BotInfo struct {
	UserID   string
	Username string
}

// BotInfo returns the bot's user information.
func (c *Client) BotInfo(ctx context.Context) (BotInfo, error) {
	if c.session.State == nil || c.session.State.User == nil {
		return BotInfo{}, errors.New("bot user not available")
	}

	return BotInfo{
		UserID:   c.session.State.User.ID,
		Username: c.session.State.User.Username,
	}, nil
}

// ChannelMessages retrieves recent messages from a channel.
func (c *Client) ChannelMessages(ctx context.Context, channelID string, limit int) ([]*discordgo.Message, error) {
	messages, err := c.session.ChannelMessages(channelID, limit, "", "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel messages: %w", err)
	}
	return messages, nil
}
