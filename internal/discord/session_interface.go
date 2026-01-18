package discord

import "github.com/bwmarrin/discordgo"

// session defines the interface for Discord session operations used by Client.
// This interface allows for mocking in tests while the production code uses *discordgo.Session.
type session interface {
	// Connection
	Open() error
	Close() error

	// Message operations
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageEditComplex(data *discordgo.MessageEdit, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessage(channelID, messageID string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessages(channelID string, limit int, beforeID, afterID, aroundID string, options ...discordgo.RequestOption) ([]*discordgo.Message, error)

	// Channel operations
	Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	ChannelEdit(channelID string, data *discordgo.ChannelEdit, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	GuildChannels(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error)
	ForumThreadStartComplex(channelID string, threadData *discordgo.ThreadStart, messageData *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	ThreadsActive(guildID string, options ...discordgo.RequestOption) (*discordgo.ThreadsList, error)
	GuildThreadsActive(guildID string, options ...discordgo.RequestOption) (*discordgo.ThreadsList, error)

	// User operations
	UserChannelCreate(recipientID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	UserChannelPermissions(userID, channelID string, fetchOptions ...discordgo.RequestOption) (perms int64, err error)
	GuildMembers(guildID string, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error)
	GuildMember(guildID, userID string, options ...discordgo.RequestOption) (*discordgo.Member, error)

	// Guild operations
	Guild(guildID string, options ...discordgo.RequestOption) (*discordgo.Guild, error)

	// GetState returns the session state for accessing bot user info, etc.
	GetState() *discordgo.State
}

// sessionAdapter wraps *discordgo.Session to implement the session interface.
type sessionAdapter struct {
	*discordgo.Session
}

// GetState returns the session state.
func (s *sessionAdapter) GetState() *discordgo.State {
	return s.State
}
