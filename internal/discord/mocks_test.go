package discord

import (
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// MockSession is a programmable mock for discordgo.Session
type MockSession struct {
	// Programmable responses
	OpenError                      error
	CloseError                     error
	MessageSendError               error
	MessageEditError               error
	UserChannelError               error
	GuildMembersError              error
	GuildMemberError               error
	ChannelError                   error
	GuildChannelsError             error
	MessagesError                  error
	ThreadsActiveError             error
	ApplicationCommandsError       error
	InteractionResponseError       error
	ChannelMessageSendComplexError error
	ChannelMessageEditComplexError error
	ForumThreadStartComplexError   error
	ChannelEditError               error
	GuildError                     error
	UserChannelPermissionsError    error

	// Storage for tracking calls
	SentMessages    []*sentMessage
	EditedMessages  []*editedMessage
	CreatedChannels []string
	CreatedThreads  []*discordgo.Channel
	Interactions    []*discordgo.InteractionResponse

	// Mock data
	Channels      map[string]*discordgo.Channel
	Members       map[string][]*discordgo.Member
	Guilds        map[string]*discordgo.Guild
	Messages      map[string][]*discordgo.Message
	ActiveThreads []*discordgo.Channel
	Commands      []*discordgo.ApplicationCommand
	MockState     *discordgo.State

	mu sync.Mutex
}

type sentMessage struct {
	ChannelID string
	Content   string
	Embed     *discordgo.MessageEmbed
}

type editedMessage struct {
	ChannelID string
	MessageID string
	Content   string
	Embed     *discordgo.MessageEmbed
}

func NewMockSession() *MockSession {
	return &MockSession{
		SentMessages:   make([]*sentMessage, 0),
		EditedMessages: make([]*editedMessage, 0),
		CreatedThreads: make([]*discordgo.Channel, 0),
		Channels:       make(map[string]*discordgo.Channel),
		Members:        make(map[string][]*discordgo.Member),
		Guilds:         make(map[string]*discordgo.Guild),
		Messages:       make(map[string][]*discordgo.Message),
		Commands:       make([]*discordgo.ApplicationCommand, 0),
		MockState:      discordgo.NewState(),
	}
}

func (m *MockSession) Open() error {
	return m.OpenError
}

func (m *MockSession) Close() error {
	return m.CloseError
}

func (m *MockSession) ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.MessageSendError != nil {
		return nil, m.MessageSendError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.SentMessages = append(m.SentMessages, &sentMessage{
		ChannelID: channelID,
		Content:   content,
	})

	msgID := fmt.Sprintf("msg-%d", len(m.SentMessages))
	return &discordgo.Message{
		ID:        msgID,
		ChannelID: channelID,
		Content:   content,
	}, nil
}

func (m *MockSession) ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.MessageSendError != nil {
		return nil, m.MessageSendError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.SentMessages = append(m.SentMessages, &sentMessage{
		ChannelID: channelID,
		Embed:     embed,
	})

	msgID := fmt.Sprintf("msg-%d", len(m.SentMessages))
	return &discordgo.Message{
		ID:        msgID,
		ChannelID: channelID,
		Embeds:    []*discordgo.MessageEmbed{embed},
	}, nil
}

func (m *MockSession) ChannelMessageEdit(channelID, messageID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.MessageEditError != nil {
		return nil, m.MessageEditError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.EditedMessages = append(m.EditedMessages, &editedMessage{
		ChannelID: channelID,
		MessageID: messageID,
		Content:   content,
	})

	return &discordgo.Message{
		ID:        messageID,
		ChannelID: channelID,
		Content:   content,
	}, nil
}

func (m *MockSession) ChannelMessageEditEmbed(channelID, messageID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.MessageEditError != nil {
		return nil, m.MessageEditError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.EditedMessages = append(m.EditedMessages, &editedMessage{
		ChannelID: channelID,
		MessageID: messageID,
		Embed:     embed,
	})

	return &discordgo.Message{
		ID:        messageID,
		ChannelID: channelID,
		Embeds:    []*discordgo.MessageEmbed{embed},
	}, nil
}

func (m *MockSession) UserChannelCreate(recipientID string, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	if m.UserChannelError != nil {
		return nil, m.UserChannelError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	channelID := fmt.Sprintf("dm-%s", recipientID)
	m.CreatedChannels = append(m.CreatedChannels, channelID)

	return &discordgo.Channel{
		ID:   channelID,
		Type: discordgo.ChannelTypeDM,
	}, nil
}

func (m *MockSession) GuildMembers(guildID string, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
	if m.GuildMembersError != nil {
		return nil, m.GuildMembersError
	}

	if members, ok := m.Members[guildID]; ok {
		return members, nil
	}

	return []*discordgo.Member{}, nil
}

func (m *MockSession) Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	if m.ChannelError != nil {
		return nil, m.ChannelError
	}

	if channel, ok := m.Channels[channelID]; ok {
		return channel, nil
	}

	return nil, fmt.Errorf("channel not found")
}

func (m *MockSession) GuildChannels(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error) {
	if m.GuildChannelsError != nil {
		return nil, m.GuildChannelsError
	}

	channels := make([]*discordgo.Channel, 0)
	for _, ch := range m.Channels {
		if ch.GuildID == guildID {
			channels = append(channels, ch)
		}
	}

	return channels, nil
}

func (m *MockSession) ChannelMessages(channelID string, limit int, beforeID, afterID, aroundID string, options ...discordgo.RequestOption) ([]*discordgo.Message, error) {
	if m.MessagesError != nil {
		return nil, m.MessagesError
	}

	if messages, ok := m.Messages[channelID]; ok {
		return messages, nil
	}

	return []*discordgo.Message{}, nil
}

func (m *MockSession) ThreadsActive(guildID string, options ...discordgo.RequestOption) (*discordgo.ThreadsList, error) {
	if m.ThreadsActiveError != nil {
		return nil, m.ThreadsActiveError
	}

	return &discordgo.ThreadsList{
		Threads: m.ActiveThreads,
	}, nil
}

func (m *MockSession) ApplicationCommandBulkOverwrite(appID, guildID string, commands []*discordgo.ApplicationCommand, options ...discordgo.RequestOption) ([]*discordgo.ApplicationCommand, error) {
	if m.ApplicationCommandsError != nil {
		return nil, m.ApplicationCommandsError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.Commands = commands
	return commands, nil
}

func (m *MockSession) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse, options ...discordgo.RequestOption) error {
	if m.InteractionResponseError != nil {
		return m.InteractionResponseError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.Interactions = append(m.Interactions, resp)
	return nil
}

// Helper functions to set up mock data

func (m *MockSession) AddChannel(channel *discordgo.Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Channels[channel.ID] = channel
}

func (m *MockSession) AddMember(guildID string, member *discordgo.Member) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Members[guildID] = append(m.Members[guildID], member)
}

func (m *MockSession) AddMessage(channelID string, message *discordgo.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages[channelID] = append(m.Messages[channelID], message)
}

func (m *MockSession) AddActiveThread(thread *discordgo.Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveThreads = append(m.ActiveThreads, thread)
}

// NewMockChannel creates a mock Discord channel
func NewMockChannel(id, name, guildID string, channelType discordgo.ChannelType) *discordgo.Channel {
	return &discordgo.Channel{
		ID:      id,
		Name:    name,
		GuildID: guildID,
		Type:    channelType,
	}
}

// NewMockMember creates a mock Discord guild member
func NewMockMember(userID, username, globalName string) *discordgo.Member {
	return &discordgo.Member{
		User: &discordgo.User{
			ID:         userID,
			Username:   username,
			GlobalName: globalName,
		},
	}
}

// NewMockMessage creates a mock Discord message
func NewMockMessage(id, channelID, content, authorID string) *discordgo.Message {
	return &discordgo.Message{
		ID:        id,
		ChannelID: channelID,
		Content:   content,
		Author: &discordgo.User{
			ID: authorID,
		},
	}
}

// ChannelMessageSendComplex mocks sending a complex message
func (m *MockSession) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.ChannelMessageSendComplexError != nil {
		return nil, m.ChannelMessageSendComplexError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var embed *discordgo.MessageEmbed
	if len(data.Embeds) > 0 {
		embed = data.Embeds[0]
	}

	m.SentMessages = append(m.SentMessages, &sentMessage{
		ChannelID: channelID,
		Content:   data.Content,
		Embed:     embed,
	})

	msgID := fmt.Sprintf("msg-%d", len(m.SentMessages))
	return &discordgo.Message{
		ID:        msgID,
		ChannelID: channelID,
		Content:   data.Content,
		Embeds:    data.Embeds,
	}, nil
}

// ChannelMessageEditComplex mocks editing a complex message
func (m *MockSession) ChannelMessageEditComplex(data *discordgo.MessageEdit, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.ChannelMessageEditComplexError != nil {
		return nil, m.ChannelMessageEditComplexError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	content := ""
	if data.Content != nil {
		content = *data.Content
	}

	var embed *discordgo.MessageEmbed
	if data.Embeds != nil && len(*data.Embeds) > 0 {
		embed = (*data.Embeds)[0]
	}

	m.EditedMessages = append(m.EditedMessages, &editedMessage{
		ChannelID: data.Channel,
		MessageID: data.ID,
		Content:   content,
		Embed:     embed,
	})

	return &discordgo.Message{
		ID:        data.ID,
		ChannelID: data.Channel,
		Content:   content,
	}, nil
}

// ForumThreadStartComplex mocks creating a forum thread
func (m *MockSession) ForumThreadStartComplex(channelID string, threadData *discordgo.ThreadStart, messageData *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	if m.ForumThreadStartComplexError != nil {
		return nil, m.ForumThreadStartComplexError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	threadID := fmt.Sprintf("thread-%d", len(m.CreatedThreads)+1)
	thread := &discordgo.Channel{
		ID:       threadID,
		Name:     threadData.Name,
		Type:     discordgo.ChannelTypeGuildPublicThread,
		ParentID: channelID,
	}

	m.CreatedThreads = append(m.CreatedThreads, thread)

	// Also create the initial message in the thread
	msgID := fmt.Sprintf("msg-%d", len(m.SentMessages)+1)
	message := &discordgo.Message{
		ID:        msgID,
		ChannelID: threadID,
		Content:   messageData.Content,
	}
	m.Messages[threadID] = []*discordgo.Message{message}

	return thread, nil
}

// ChannelEdit mocks editing a channel
func (m *MockSession) ChannelEdit(channelID string, data *discordgo.ChannelEdit, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	if m.ChannelEditError != nil {
		return nil, m.ChannelEditError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Update or create channel
	channel, exists := m.Channels[channelID]
	if !exists {
		channel = &discordgo.Channel{
			ID: channelID,
		}
		m.Channels[channelID] = channel
	}

	if data.Name != "" {
		channel.Name = data.Name
	}
	if data.Archived != nil {
		channel.ThreadMetadata = &discordgo.ThreadMetadata{
			Archived: *data.Archived,
		}
	}

	return channel, nil
}

// ChannelMessage mocks retrieving a single message
func (m *MockSession) ChannelMessage(channelID, messageID string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.MessagesError != nil {
		return nil, m.MessagesError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if messages, ok := m.Messages[channelID]; ok {
		for _, msg := range messages {
			if msg.ID == messageID {
				return msg, nil
			}
		}
	}

	return nil, fmt.Errorf("message not found")
}

// GetState returns the mock state
func (m *MockSession) GetState() *discordgo.State {
	return m.MockState
}

// GuildMember mocks fetching a single guild member
func (m *MockSession) GuildMember(guildID, userID string, options ...discordgo.RequestOption) (*discordgo.Member, error) {
	if m.GuildMemberError != nil {
		return nil, m.GuildMemberError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if members, ok := m.Members[guildID]; ok {
		for _, member := range members {
			if member.User.ID == userID {
				return member, nil
			}
		}
	}

	return nil, fmt.Errorf("member not found")
}

// Guild mocks fetching guild information
func (m *MockSession) Guild(guildID string, options ...discordgo.RequestOption) (*discordgo.Guild, error) {
	if m.GuildError != nil {
		return nil, m.GuildError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if guild, ok := m.Guilds[guildID]; ok {
		return guild, nil
	}

	return nil, fmt.Errorf("guild not found")
}

// UserChannelPermissions mocks checking user permissions
func (m *MockSession) UserChannelPermissions(userID, channelID string, fetchOptions ...discordgo.RequestOption) (int64, error) {
	if m.UserChannelPermissionsError != nil {
		return 0, m.UserChannelPermissionsError
	}

	// Return full permissions for testing
	return discordgo.PermissionAll, nil
}

// GuildThreadsActive mocks fetching active threads
func (m *MockSession) GuildThreadsActive(guildID string, options ...discordgo.RequestOption) (*discordgo.ThreadsList, error) {
	if m.ThreadsActiveError != nil {
		return nil, m.ThreadsActiveError
	}

	return &discordgo.ThreadsList{
		Threads: m.ActiveThreads,
	}, nil
}
