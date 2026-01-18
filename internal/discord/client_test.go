package discord

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// newTestClientWithMock creates a Client for testing with a mock session
func newTestClientWithMock(mock *MockSession) *Client {
	return &Client{
		session:          mock,
		realSession:      nil,
		channelCache:     make(map[string]string),
		channelTypeCache: make(map[string]discordgo.ChannelType),
		userCache:        make(map[string]string),
	}
}

// findUserInMembers tests the matching logic without Discord API calls.
// This mirrors the logic in LookupUserByUsername but operates on a slice of members.
func findUserInMembers(username string, members []*discordgo.Member) (userID, matchType string) {
	// Skip empty usernames
	if username == "" {
		return "", ""
	}

	// Tier 1: Exact match (Username takes precedence over GlobalName, then Nick)
	for _, member := range members {
		if member.User.Username == username {
			return member.User.ID, "username"
		}
	}
	for _, member := range members {
		if member.User.GlobalName == username {
			return member.User.ID, "global_name"
		}
	}
	for _, member := range members {
		if member.Nick == username {
			return member.User.ID, "nick"
		}
	}

	// Tier 2: Case-insensitive match (Username takes precedence over GlobalName, then Nick)
	for _, member := range members {
		if strings.EqualFold(member.User.Username, username) {
			return member.User.ID, "username_case_insensitive"
		}
	}
	for _, member := range members {
		if strings.EqualFold(member.User.GlobalName, username) {
			return member.User.ID, "global_name_case_insensitive"
		}
	}
	for _, member := range members {
		if strings.EqualFold(member.Nick, username) {
			return member.User.ID, "nick_case_insensitive"
		}
	}

	lowerUsername := strings.ToLower(username)

	// Tier 3: Prefix match (only if unambiguous)
	type prefixMatch struct {
		member    *discordgo.Member
		matchType string
	}
	var matches []prefixMatch

	for _, member := range members {
		if strings.HasPrefix(strings.ToLower(member.User.Username), lowerUsername) {
			matches = append(matches, prefixMatch{member: member, matchType: "username_prefix"})
		} else if strings.HasPrefix(strings.ToLower(member.User.GlobalName), lowerUsername) {
			matches = append(matches, prefixMatch{member: member, matchType: "global_name_prefix"})
		} else if strings.HasPrefix(strings.ToLower(member.Nick), lowerUsername) {
			matches = append(matches, prefixMatch{member: member, matchType: "nick_prefix"})
		}
	}

	if len(matches) == 1 {
		return matches[0].member.User.ID, matches[0].matchType
	}

	return "", ""
}

func TestFindUserInMembers(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		members       []*discordgo.Member
		wantID        string
		wantMatchType string
	}{
		{
			name:     "exact username match",
			username: "alice",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "Alice Smith"}},
				{User: &discordgo.User{ID: "222", Username: "bob", GlobalName: "Bob Jones"}},
			},
			wantID:        "111",
			wantMatchType: "username",
		},
		{
			name:     "exact global name match",
			username: "Alice Smith",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "Alice Smith"}},
				{User: &discordgo.User{ID: "222", Username: "bob", GlobalName: "Bob Jones"}},
			},
			wantID:        "111",
			wantMatchType: "global_name",
		},
		{
			name:     "case-insensitive username match",
			username: "ALICE",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "Alice Smith"}},
				{User: &discordgo.User{ID: "222", Username: "bob", GlobalName: "Bob Jones"}},
			},
			wantID:        "111",
			wantMatchType: "username_case_insensitive",
		},
		{
			name:     "case-insensitive global name match",
			username: "alice smith",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "Alice Smith"}},
				{User: &discordgo.User{ID: "222", Username: "bob", GlobalName: "Bob Jones"}},
			},
			wantID:        "111",
			wantMatchType: "global_name_case_insensitive",
		},
		{
			name:     "prefix match - username - unambiguous",
			username: "ali",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "Alice Smith"}},
				{User: &discordgo.User{ID: "222", Username: "bob", GlobalName: "Bob Jones"}},
			},
			wantID:        "111",
			wantMatchType: "username_prefix",
		},
		{
			name:     "prefix match - global name - unambiguous",
			username: "Alice S",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "Alice Smith"}},
				{User: &discordgo.User{ID: "222", Username: "bob", GlobalName: "Bob Jones"}},
			},
			wantID:        "111",
			wantMatchType: "global_name_prefix",
		},
		{
			name:     "prefix match - ambiguous - should fail",
			username: "al",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "Alice Smith"}},
				{User: &discordgo.User{ID: "222", Username: "alex", GlobalName: "Alex Jones"}},
				{User: &discordgo.User{ID: "333", Username: "bob", GlobalName: "Bob Jones"}},
			},
			wantID:        "",
			wantMatchType: "",
		},
		{
			name:     "no match",
			username: "charlie",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "Alice Smith"}},
				{User: &discordgo.User{ID: "222", Username: "bob", GlobalName: "Bob Jones"}},
			},
			wantID:        "",
			wantMatchType: "",
		},
		{
			name:     "exact match takes precedence over prefix",
			username: "alice",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "Alice Smith"}},
				{User: &discordgo.User{ID: "222", Username: "alicejones", GlobalName: "Alice Jones"}},
			},
			wantID:        "111",
			wantMatchType: "username",
		},
		{
			name:     "case-insensitive takes precedence over prefix",
			username: "ALICE",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "Alice Smith"}},
				{User: &discordgo.User{ID: "222", Username: "ALICEJONES", GlobalName: "Alice Jones"}},
			},
			wantID:        "111",
			wantMatchType: "username_case_insensitive",
		},
		{
			name:     "empty username - should not match empty fields",
			username: "",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "", GlobalName: ""}},
				{User: &discordgo.User{ID: "222", Username: "bob", GlobalName: "Bob Jones"}},
			},
			wantID:        "",
			wantMatchType: "",
		},
		{
			name:     "prefix match with case insensitivity",
			username: "ALI",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "Alice Smith"}},
				{User: &discordgo.User{ID: "222", Username: "bob", GlobalName: "Bob Jones"}},
			},
			wantID:        "111",
			wantMatchType: "username_prefix",
		},
		{
			name:     "username match preferred over global name when both match",
			username: "bob",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "111", Username: "alice", GlobalName: "bob"}},
				{User: &discordgo.User{ID: "222", Username: "bob", GlobalName: "Robert"}},
			},
			wantID:        "222",
			wantMatchType: "username",
		},
		{
			name:     "real world case - thomstrom exact match",
			username: "thomstrom",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "123456", Username: "thomstrom", GlobalName: "thomstrom"}},
				{User: &discordgo.User{ID: "789012", Username: "alice", GlobalName: "Alice"}},
			},
			wantID:        "123456",
			wantMatchType: "username",
		},
		{
			name:     "real world case - THOMSTROM case insensitive",
			username: "THOMSTROM",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "123456", Username: "thomstrom", GlobalName: "thomstrom"}},
				{User: &discordgo.User{ID: "789012", Username: "alice", GlobalName: "Alice"}},
			},
			wantID:        "123456",
			wantMatchType: "username_case_insensitive",
		},
		{
			name:     "real world case - thom prefix",
			username: "thom",
			members: []*discordgo.Member{
				{User: &discordgo.User{ID: "123456", Username: "thomstrom", GlobalName: "Thomas S"}},
				{User: &discordgo.User{ID: "789012", Username: "alice", GlobalName: "Alice"}},
			},
			wantID:        "123456",
			wantMatchType: "username_prefix",
		},
		// Nickname matching tests
		{
			name:     "exact nickname match",
			username: "octocat",
			members: []*discordgo.Member{
				{Nick: "octocat", User: &discordgo.User{ID: "111", Username: "user1", GlobalName: "User One"}},
				{Nick: "cooldev", User: &discordgo.User{ID: "222", Username: "user2", GlobalName: "User Two"}},
			},
			wantID:        "111",
			wantMatchType: "nick",
		},
		{
			name:     "case-insensitive nickname match",
			username: "OCTOCAT",
			members: []*discordgo.Member{
				{Nick: "octocat", User: &discordgo.User{ID: "111", Username: "user1", GlobalName: "User One"}},
				{Nick: "cooldev", User: &discordgo.User{ID: "222", Username: "user2", GlobalName: "User Two"}},
			},
			wantID:        "111",
			wantMatchType: "nick_case_insensitive",
		},
		{
			name:     "prefix nickname match - unambiguous",
			username: "octo",
			members: []*discordgo.Member{
				{Nick: "octocat", User: &discordgo.User{ID: "111", Username: "user1", GlobalName: "User One"}},
				{Nick: "cooldev", User: &discordgo.User{ID: "222", Username: "user2", GlobalName: "User Two"}},
			},
			wantID:        "111",
			wantMatchType: "nick_prefix",
		},
		{
			name:     "username preferred over nickname",
			username: "octocat",
			members: []*discordgo.Member{
				{Nick: "different", User: &discordgo.User{ID: "111", Username: "octocat", GlobalName: "User One"}},
				{Nick: "octocat", User: &discordgo.User{ID: "222", Username: "other", GlobalName: "User Two"}},
			},
			wantID:        "111",
			wantMatchType: "username",
		},
		{
			name:     "global name preferred over nickname",
			username: "User One",
			members: []*discordgo.Member{
				{Nick: "User One", User: &discordgo.User{ID: "111", Username: "user1", GlobalName: "Different"}},
				{Nick: "other", User: &discordgo.User{ID: "222", Username: "user2", GlobalName: "User One"}},
			},
			wantID:        "222",
			wantMatchType: "global_name",
		},
		{
			name:     "nickname match when username and global name don't match",
			username: "mynickname",
			members: []*discordgo.Member{
				{Nick: "mynickname", User: &discordgo.User{ID: "111", Username: "user1", GlobalName: "User One"}},
				{Nick: "othernick", User: &discordgo.User{ID: "222", Username: "user2", GlobalName: "User Two"}},
			},
			wantID:        "111",
			wantMatchType: "nick",
		},
		{
			name:     "empty nickname should not match",
			username: "test",
			members: []*discordgo.Member{
				{Nick: "", User: &discordgo.User{ID: "111", Username: "test", GlobalName: "Test User"}},
			},
			wantID:        "111",
			wantMatchType: "username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotMatchType := findUserInMembers(tt.username, tt.members)

			if gotID != tt.wantID {
				t.Errorf("findUserInMembers() ID = %v, want %v", gotID, tt.wantID)
			}

			if gotMatchType != tt.wantMatchType {
				t.Errorf("findUserInMembers() matchType = %v, want %v", gotMatchType, tt.wantMatchType)
			}
		})
	}
}

// TestClient_SetGuildID tests setting the guild ID.
func TestClient_SetGuildID(t *testing.T) {
	client := &Client{}

	client.SetGuildID("test-guild-123")

	if client.guildID != "test-guild-123" {
		t.Errorf("SetGuildID() guildID = %q, want %q", client.guildID, "test-guild-123")
	}
}

// TestClient_GuildID tests getting the guild ID.
func TestClient_GuildID(t *testing.T) {
	client := &Client{guildID: "test-guild-456"}

	got := client.GuildID()
	if got != "test-guild-456" {
		t.Errorf("GuildID() = %q, want %q", got, "test-guild-456")
	}
}

// TestClient_Session tests getting the session.
func TestClient_Session(t *testing.T) {
	realSession := &discordgo.Session{}
	client := &Client{
		session:     &sessionAdapter{Session: realSession},
		realSession: realSession,
	}

	got := client.Session()
	if got != realSession {
		t.Error("Session() should return the same session")
	}
}

// TestIsAllDigits tests the isAllDigits helper function.
func TestIsAllDigits(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"all digits", "123456", true},
		{"has letters", "123abc", false},
		{"has spaces", "123 456", false},
		{"has special chars", "123-456", false},
		{"empty string", "", false}, // Empty string is not all digits
		{"single digit", "5", true},
		{"single letter", "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAllDigits(tt.input)
			if got != tt.want {
				t.Errorf("isAllDigits(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestClient_ResolveChannelID_LooksLikeID tests ResolveChannelID when input looks like an ID.
func TestClient_ResolveChannelID_LooksLikeID(t *testing.T) {
	client := &Client{
		guildID:      "test-guild",
		channelCache: make(map[string]string),
	}

	// 20-character numeric string should be returned as-is (looks like a Discord ID)
	channelID := "12345678901234567890"
	got := client.ResolveChannelID(context.Background(), channelID)

	if got != channelID {
		t.Errorf("ResolveChannelID(%q) = %q, want %q", channelID, got, channelID)
	}
}

// TestClient_ResolveChannelID_CacheHit tests ResolveChannelID with cached channel.
func TestClient_ResolveChannelID_CacheHit(t *testing.T) {
	client := &Client{
		guildID: "test-guild",
		channelCache: map[string]string{
			"general": "111222333444555666",
		},
	}

	got := client.ResolveChannelID(context.Background(), "general")
	want := "111222333444555666"

	if got != want {
		t.Errorf("ResolveChannelID(\"general\") = %q, want %q", got, want)
	}
}

// TestClient_ResolveChannelID_NoGuildID tests ResolveChannelID when no guild ID is set.
func TestClient_ResolveChannelID_NoGuildID(t *testing.T) {
	client := &Client{
		guildID:      "",
		channelCache: make(map[string]string),
	}

	channelName := "general"
	got := client.ResolveChannelID(context.Background(), channelName)

	// Should return the input unchanged when no guild ID is set
	if got != channelName {
		t.Errorf("ResolveChannelID(%q) = %q, want %q", channelName, got, channelName)
	}
}

// TestClient_ChannelType_CacheHit tests ChannelType with cached channel type.
func TestClient_ChannelType_CacheHit(t *testing.T) {
	client := &Client{
		channelTypeCache: map[string]discordgo.ChannelType{
			"123456": discordgo.ChannelTypeGuildText,
		},
	}

	got, err := client.ChannelType(context.Background(), "123456")
	if err != nil {
		t.Fatalf("ChannelType() error = %v, want nil", err)
	}

	if got != discordgo.ChannelTypeGuildText {
		t.Errorf("ChannelType() = %v, want %v", got, discordgo.ChannelTypeGuildText)
	}
}

// TestClient_IsBotInChannel_NilState tests IsBotInChannel when session state is nil.
func TestClient_IsBotInChannel_NilState(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MockState = nil

	client := newTestClientWithMock(mockSession)

	got := client.IsBotInChannel(context.Background(), "some-channel-id")
	if got {
		t.Error("IsBotInChannel() = true, want false when session.State is nil")
	}
}

// TestClient_IsBotInChannel_NilUser tests IsBotInChannel when user is nil.
func TestClient_IsBotInChannel_NilUser(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MockState.User = nil

	client := newTestClientWithMock(mockSession)

	got := client.IsBotInChannel(context.Background(), "some-channel-id")
	if got {
		t.Error("IsBotInChannel() = true, want false when session.State.User is nil")
	}
}

// TestClient_IsUserInGuild_NoGuildID tests IsUserInGuild when no guild ID is set.
func TestClient_IsUserInGuild_NoGuildID(t *testing.T) {
	client := newTestClientWithMock(NewMockSession())
	// Don't set guildID

	got := client.IsUserInGuild(context.Background(), "user-123")
	if got {
		t.Error("IsUserInGuild() = true, want false when no guild ID is set")
	}
}

// TestClient_IsUserActive_NoGuildID tests IsUserActive when no guild ID is set.
func TestClient_IsUserActive_NoGuildID(t *testing.T) {
	client := newTestClientWithMock(NewMockSession())
	// Don't set guildID

	got := client.IsUserActive(context.Background(), "user-123")
	if got {
		t.Error("IsUserActive() = true, want false when no guild ID is set")
	}
}

// TestClient_GuildInfo_NoGuildID tests GuildInfo when no guild ID is set.
func TestClient_GuildInfo_NoGuildID(t *testing.T) {
	client := newTestClientWithMock(NewMockSession())
	// Don't set guildID

	_, err := client.GuildInfo(context.Background())
	if err == nil {
		t.Error("GuildInfo() error = nil, want error when no guild ID is set")
	}
	if err != nil && err.Error() != "no guild ID set" {
		t.Errorf("GuildInfo() error = %q, want %q", err.Error(), "no guild ID set")
	}
}

// TestClient_BotInfo_NilState tests BotInfo when session state is nil.
func TestClient_BotInfo_NilState(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MockState = nil

	client := newTestClientWithMock(mockSession)

	_, err := client.BotInfo(context.Background())
	if err == nil {
		t.Error("BotInfo() error = nil, want error when session.State is nil")
	}
	if err != nil && err.Error() != "bot user not available" {
		t.Errorf("BotInfo() error = %q, want %q", err.Error(), "bot user not available")
	}
}

// TestClient_BotInfo_NilUser tests BotInfo when user is nil.
func TestClient_BotInfo_NilUser(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MockState.User = nil

	client := newTestClientWithMock(mockSession)

	_, err := client.BotInfo(context.Background())
	if err == nil {
		t.Error("BotInfo() error = nil, want error when session.State.User is nil")
	}
	if err != nil && err.Error() != "bot user not available" {
		t.Errorf("BotInfo() error = %q, want %q", err.Error(), "bot user not available")
	}
}

// TestClient_PostMessage tests posting a message to a channel.
func TestClient_PostMessage(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	channelID := "channel-123"
	text := "Hello, world!"

	msgID, err := client.PostMessage(ctx, channelID, text)
	if err != nil {
		t.Fatalf("PostMessage() error = %v, want nil", err)
	}

	if msgID == "" {
		t.Error("PostMessage() returned empty message ID")
	}

	if len(mockSession.SentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockSession.SentMessages))
	}

	sentMsg := mockSession.SentMessages[0]
	if sentMsg.ChannelID != channelID {
		t.Errorf("Sent message channel ID = %q, want %q", sentMsg.ChannelID, channelID)
	}
	if sentMsg.Content != text {
		t.Errorf("Sent message content = %q, want %q", sentMsg.Content, text)
	}
}

// TestClient_PostMessage_Error tests PostMessage error handling.
func TestClient_PostMessage_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.ChannelMessageSendComplexError = fmt.Errorf("API error")

	client := newTestClientWithMock(mockSession)

	_, err := client.PostMessage(context.Background(), "channel-123", "test")
	if err == nil {
		t.Error("PostMessage() error = nil, want error")
	}
}

// TestClient_UpdateMessage tests updating an existing message.
func TestClient_UpdateMessage(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	channelID := "channel-123"
	messageID := "msg-456"
	newText := "Updated content"

	err := client.UpdateMessage(ctx, channelID, messageID, newText)
	if err != nil {
		t.Fatalf("UpdateMessage() error = %v, want nil", err)
	}

	if len(mockSession.EditedMessages) != 1 {
		t.Fatalf("Expected 1 edited message, got %d", len(mockSession.EditedMessages))
	}

	editedMsg := mockSession.EditedMessages[0]
	if editedMsg.ChannelID != channelID {
		t.Errorf("Edited message channel ID = %q, want %q", editedMsg.ChannelID, channelID)
	}
	if editedMsg.MessageID != messageID {
		t.Errorf("Edited message ID = %q, want %q", editedMsg.MessageID, messageID)
	}
	if editedMsg.Content != newText {
		t.Errorf("Edited message content = %q, want %q", editedMsg.Content, newText)
	}
}

// TestClient_UpdateMessage_Error tests UpdateMessage error handling.
func TestClient_UpdateMessage_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.ChannelMessageEditComplexError = fmt.Errorf("API error")

	client := newTestClientWithMock(mockSession)

	err := client.UpdateMessage(context.Background(), "channel-123", "msg-456", "test")
	if err == nil {
		t.Error("UpdateMessage() error = nil, want error")
	}
}

// TestClient_PostForumThread tests creating a forum thread.
func TestClient_PostForumThread(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("guild-123")

	ctx := context.Background()
	channelID := "forum-channel-789"
	title := "New PR Discussion"
	content := "Let's discuss this PR"

	threadID, messageID, err := client.PostForumThread(ctx, channelID, title, content)
	if err != nil {
		t.Fatalf("PostForumThread() error = %v, want nil", err)
	}

	if threadID == "" {
		t.Error("PostForumThread() returned empty thread ID")
	}
	if messageID == "" {
		t.Error("PostForumThread() returned empty message ID")
	}

	if len(mockSession.CreatedThreads) != 1 {
		t.Fatalf("Expected 1 created thread, got %d", len(mockSession.CreatedThreads))
	}

	createdThread := mockSession.CreatedThreads[0]
	if createdThread.Name != title {
		t.Errorf("Created thread name = %q, want %q", createdThread.Name, title)
	}
}

// TestClient_PostForumThread_Error tests PostForumThread error handling.
func TestClient_PostForumThread_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.ForumThreadStartComplexError = fmt.Errorf("API error")

	client := newTestClientWithMock(mockSession)
	client.SetGuildID("guild-123")

	_, _, err := client.PostForumThread(context.Background(), "channel-123", "title", "content")
	if err == nil {
		t.Error("PostForumThread() error = nil, want error")
	}
}

// TestClient_UpdateForumPost tests updating a forum post.
func TestClient_UpdateForumPost(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	threadID := "thread-123"
	messageID := "msg-456"
	newTitle := "Updated PR Discussion"
	newContent := "Updated content"

	err := client.UpdateForumPost(ctx, threadID, messageID, newTitle, newContent)
	if err != nil {
		t.Fatalf("UpdateForumPost() error = %v, want nil", err)
	}

	// Should have edited the title (via ChannelEdit)
	if len(mockSession.Channels) == 0 {
		t.Error("Expected channel to be edited for title update")
	}

	// Should have edited the message content
	if len(mockSession.EditedMessages) != 1 {
		t.Fatalf("Expected 1 edited message, got %d", len(mockSession.EditedMessages))
	}

	editedMsg := mockSession.EditedMessages[0]
	if editedMsg.Content != newContent {
		t.Errorf("Edited message content = %q, want %q", editedMsg.Content, newContent)
	}
}

// TestClient_UpdateForumPost_NoMessageID tests UpdateForumPost with empty message ID.
func TestClient_UpdateForumPost_NoMessageID(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	threadID := "thread-123"
	newTitle := "Updated PR Discussion"
	newContent := "Updated content"

	err := client.UpdateForumPost(ctx, threadID, "", newTitle, newContent)
	if err != nil {
		t.Fatalf("UpdateForumPost() error = %v, want nil", err)
	}

	// Should only have edited the title, not the message
	if len(mockSession.EditedMessages) != 0 {
		t.Errorf("Expected 0 edited messages, got %d", len(mockSession.EditedMessages))
	}
}

// TestClient_UpdateForumPost_TitleEditError tests UpdateForumPost when title edit fails.
func TestClient_UpdateForumPost_TitleEditError(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.ChannelEditError = fmt.Errorf("title edit failed")
	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	err := client.UpdateForumPost(ctx, "thread-123", "msg-456", "New Title", "New Content")
	if err == nil {
		t.Error("UpdateForumPost() error = nil, want error when title edit fails")
	}
}

// TestClient_UpdateForumPost_MessageEditError tests UpdateForumPost when message edit fails.
func TestClient_UpdateForumPost_MessageEditError(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.ChannelMessageEditComplexError = fmt.Errorf("message edit failed")
	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	err := client.UpdateForumPost(ctx, "thread-123", "msg-456", "New Title", "New Content")
	if err == nil {
		t.Error("UpdateForumPost() error = nil, want error when message edit fails")
	}
}

// TestClient_PostForumThread_MessagesFetchError tests when ChannelMessages fails.
func TestClient_PostForumThread_MessagesFetchError(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MessagesError = fmt.Errorf("messages fetch failed")
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("guild-123")

	ctx := context.Background()
	threadID, messageID, err := client.PostForumThread(ctx, "channel-123", "title", "content")

	// Should succeed but return empty messageID
	if err != nil {
		t.Errorf("PostForumThread() error = %v, want nil", err)
	}
	if threadID == "" {
		t.Error("PostForumThread() threadID = empty, want non-empty")
	}
	if messageID != "" {
		t.Errorf("PostForumThread() messageID = %q, want empty when messages fetch fails", messageID)
	}
}

// TestClient_ArchiveThread tests archiving a thread.
func TestClient_ArchiveThread(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	threadID := "thread-123"

	err := client.ArchiveThread(ctx, threadID)
	if err != nil {
		t.Fatalf("ArchiveThread() error = %v, want nil", err)
	}

	// Check that the channel was edited with Archived=true
	channel, exists := mockSession.Channels[threadID]
	if !exists {
		t.Fatal("Expected channel to be edited for archiving")
	}

	if channel.ThreadMetadata == nil || !channel.ThreadMetadata.Archived {
		t.Error("Expected thread to be archived")
	}
}

// TestClient_ArchiveThread_Error tests ArchiveThread error handling.
func TestClient_ArchiveThread_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.ChannelEditError = fmt.Errorf("API error")

	client := newTestClientWithMock(mockSession)

	err := client.ArchiveThread(context.Background(), "thread-123")
	if err == nil {
		t.Error("ArchiveThread() error = nil, want error")
	}
}

// TestClient_SendDM tests sending a direct message.
func TestClient_SendDM(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	userID := "user-123"
	text := "Hello via DM!"

	channelID, messageID, err := client.SendDM(ctx, userID, text)
	if err != nil {
		t.Fatalf("SendDM() error = %v, want nil", err)
	}

	if channelID == "" {
		t.Error("SendDM() returned empty channel ID")
	}
	if messageID == "" {
		t.Error("SendDM() returned empty message ID")
	}

	if len(mockSession.CreatedChannels) != 1 {
		t.Fatalf("Expected 1 created DM channel, got %d", len(mockSession.CreatedChannels))
	}

	if len(mockSession.SentMessages) != 1 {
		t.Fatalf("Expected 1 sent DM, got %d", len(mockSession.SentMessages))
	}

	sentMsg := mockSession.SentMessages[0]
	if sentMsg.Content != text {
		t.Errorf("Sent DM content = %q, want %q", sentMsg.Content, text)
	}
}

// TestClient_SendDM_UserChannelError tests SendDM error when creating DM channel.
func TestClient_SendDM_UserChannelError(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.UserChannelError = fmt.Errorf("failed to create DM channel")

	client := newTestClientWithMock(mockSession)

	_, _, err := client.SendDM(context.Background(), "user-123", "test")
	if err == nil {
		t.Error("SendDM() error = nil, want error")
	}
}

// TestClient_SendDM_MessageSendError tests SendDM error when sending message.
func TestClient_SendDM_MessageSendError(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.ChannelMessageSendComplexError = fmt.Errorf("failed to send message")

	client := newTestClientWithMock(mockSession)

	_, _, err := client.SendDM(context.Background(), "user-123", "test")
	if err == nil {
		t.Error("SendDM() error = nil, want error")
	}
}

// TestClient_UpdateDM tests updating a DM.
func TestClient_UpdateDM(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	channelID := "dm-channel-123"
	messageID := "msg-456"
	newText := "Updated DM content"

	err := client.UpdateDM(ctx, channelID, messageID, newText)
	if err != nil {
		t.Fatalf("UpdateDM() error = %v, want nil", err)
	}

	if len(mockSession.EditedMessages) != 1 {
		t.Fatalf("Expected 1 edited message, got %d", len(mockSession.EditedMessages))
	}

	editedMsg := mockSession.EditedMessages[0]
	if editedMsg.Content != newText {
		t.Errorf("Edited DM content = %q, want %q", editedMsg.Content, newText)
	}
}

// TestClient_UpdateDM_Error tests UpdateDM error handling.
func TestClient_UpdateDM_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.ChannelMessageEditComplexError = fmt.Errorf("API error")

	client := newTestClientWithMock(mockSession)

	err := client.UpdateDM(context.Background(), "dm-123", "msg-456", "test")
	if err == nil {
		t.Error("UpdateDM() error = nil, want error")
	}
}

// TestClient_LookupUserByUsername tests user lookup with cached value.
func TestClient_LookupUserByUsername(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.AddMember("guild-123", NewMockMember("user-111", "alice", "Alice Smith"))
	mockSession.AddMember("guild-123", NewMockMember("user-222", "bob", "Bob Jones"))

	client := newTestClientWithMock(mockSession)
	client.SetGuildID("guild-123")

	ctx := context.Background()
	userID := client.LookupUserByUsername(ctx, "alice")

	if userID != "user-111" {
		t.Errorf("LookupUserByUsername(\"alice\") = %q, want %q", userID, "user-111")
	}

	// Should now be cached
	if cachedID, ok := client.userCache["alice"]; !ok || cachedID != "user-111" {
		t.Error("Expected user to be cached after lookup")
	}
}

// TestClient_LookupUserByUsername_CacheHit tests user lookup with cached value.
func TestClient_LookupUserByUsername_CacheHit(t *testing.T) {
	client := newTestClientWithMock(NewMockSession())
	client.SetGuildID("guild-123")
	client.userCache["alice"] = "user-111"

	userID := client.LookupUserByUsername(context.Background(), "alice")
	if userID != "user-111" {
		t.Errorf("LookupUserByUsername(\"alice\") = %q, want %q", userID, "user-111")
	}
}

// TestClient_LookupUserByUsername_NoGuildID tests user lookup with no guild ID.
func TestClient_LookupUserByUsername_NoGuildID(t *testing.T) {
	client := newTestClientWithMock(NewMockSession())
	// Don't set guildID - test with empty guild ID

	userID := client.LookupUserByUsername(context.Background(), "alice")
	if userID != "" {
		t.Errorf("LookupUserByUsername() = %q, want empty string when no guild ID", userID)
	}
}

// TestClient_LookupUserByUsername_EmptyUsername tests user lookup with empty username.
func TestClient_LookupUserByUsername_EmptyUsername(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.AddMember("guild-123", NewMockMember("user-111", "alice", "Alice Smith"))

	client := newTestClientWithMock(mockSession)
	client.SetGuildID("guild-123")

	userID := client.LookupUserByUsername(context.Background(), "")
	if userID != "" {
		t.Errorf("LookupUserByUsername(\"\") = %q, want empty string", userID)
	}
}

// TestClient_IsForumChannel tests checking if a channel is a forum.
func TestClient_IsForumChannel(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.AddChannel(&discordgo.Channel{
		ID:   "forum-123",
		Type: discordgo.ChannelTypeGuildForum,
	})

	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	isForum := client.IsForumChannel(ctx, "forum-123")

	if !isForum {
		t.Error("IsForumChannel() = false, want true for forum channel")
	}
}

// TestClient_IsForumChannel_TextChannel tests checking if a text channel is a forum.
func TestClient_IsForumChannel_TextChannel(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.AddChannel(&discordgo.Channel{
		ID:   "text-123",
		Type: discordgo.ChannelTypeGuildText,
	})

	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	isForum := client.IsForumChannel(ctx, "text-123")

	if isForum {
		t.Error("IsForumChannel() = true, want false for text channel")
	}
}

// TestClient_IsForumChannel_Error tests IsForumChannel error handling.
func TestClient_IsForumChannel_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.ChannelError = fmt.Errorf("channel not found")

	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	isForum := client.IsForumChannel(ctx, "nonexistent")

	if isForum {
		t.Error("IsForumChannel() = true, want false on error")
	}
}

// TestClient_MessageContent tests retrieving message content.
func TestClient_MessageContent(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.AddMessage("channel-123", &discordgo.Message{
		ID:      "msg-456",
		Content: "Test message content",
	})

	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	content, err := client.MessageContent(ctx, "channel-123", "msg-456")
	if err != nil {
		t.Fatalf("MessageContent() error = %v, want nil", err)
	}

	if content != "Test message content" {
		t.Errorf("MessageContent() = %q, want %q", content, "Test message content")
	}
}

// TestClient_MessageContent_Error tests MessageContent error handling.
func TestClient_MessageContent_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MessagesError = fmt.Errorf("message not found")

	client := newTestClientWithMock(mockSession)

	_, err := client.MessageContent(context.Background(), "channel-123", "msg-456")
	if err == nil {
		t.Error("MessageContent() error = nil, want error")
	}
}

// TestClient_ChannelMessages tests retrieving channel messages.
func TestClient_ChannelMessages(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.AddMessage("channel-123", &discordgo.Message{
		ID:      "msg-1",
		Content: "Message 1",
	})
	mockSession.AddMessage("channel-123", &discordgo.Message{
		ID:      "msg-2",
		Content: "Message 2",
	})

	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	messages, err := client.ChannelMessages(ctx, "channel-123", 10)
	if err != nil {
		t.Fatalf("ChannelMessages() error = %v, want nil", err)
	}

	if len(messages) != 2 {
		t.Errorf("ChannelMessages() returned %d messages, want 2", len(messages))
	}
}

// TestClient_ChannelMessages_Error tests ChannelMessages error handling.
func TestClient_ChannelMessages_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MessagesError = fmt.Errorf("failed to fetch messages")

	client := newTestClientWithMock(mockSession)

	_, err := client.ChannelMessages(context.Background(), "channel-123", 10)
	if err == nil {
		t.Error("ChannelMessages() error = nil, want error")
	}
}

// TestClient_FindChannelMessage tests finding a message by PR URL.
func TestClient_FindChannelMessage(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.AddMessage("channel-123", &discordgo.Message{
		ID:      "msg-1",
		Content: "Check out this PR: https://github.com/owner/repo/pull/123",
		Author: &discordgo.User{
			ID: "user-123",
		},
	})
	mockSession.AddMessage("channel-123", &discordgo.Message{
		ID:      "msg-2",
		Content: "Another message",
		Author: &discordgo.User{
			ID: "user-456",
		},
	})

	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	messageID, found := client.FindChannelMessage(ctx, "channel-123", "https://github.com/owner/repo/pull/123")

	if !found {
		t.Error("FindChannelMessage() found = false, want true")
	}

	if messageID != "msg-1" {
		t.Errorf("FindChannelMessage() messageID = %q, want %q", messageID, "msg-1")
	}
}

// TestClient_FindChannelMessage_NotFound tests FindChannelMessage when message not found.
func TestClient_FindChannelMessage_NotFound(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.AddMessage("channel-123", &discordgo.Message{
		ID:      "msg-1",
		Content: "Some other content",
		Author: &discordgo.User{
			ID: "user-123",
		},
	})

	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	_, found := client.FindChannelMessage(ctx, "channel-123", "https://github.com/owner/repo/pull/999")

	if found {
		t.Error("FindChannelMessage() found = true, want false for non-existent PR")
	}
}

// TestClient_FindChannelMessage_Error tests FindChannelMessage error handling.
func TestClient_FindChannelMessage_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MessagesError = fmt.Errorf("API error")

	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	_, found := client.FindChannelMessage(ctx, "channel-123", "https://github.com/owner/repo/pull/123")

	if found {
		t.Error("FindChannelMessage() found = true, want false on error")
	}
}

// TestClient_FindDMForPR tests finding a DM for a specific PR.
func TestClient_FindDMForPR(t *testing.T) {
	mockSession := NewMockSession()

	// UserChannelCreate will create a channel with ID "dm-user-123"
	// So we need to add messages to that channel ID
	mockSession.AddMessage("dm-user-123", &discordgo.Message{
		ID:      "dm-msg-1",
		Content: "PR notification: https://github.com/owner/repo/pull/456",
		Author: &discordgo.User{
			ID: "bot-user-id",
		},
	})

	client := newTestClientWithMock(mockSession)
	client.session.GetState().User = &discordgo.User{
		ID:       "bot-user-id",
		Username: "testbot",
	}

	ctx := context.Background()
	channelID, messageID, found := client.FindDMForPR(ctx, "user-123", "https://github.com/owner/repo/pull/456")

	if !found {
		t.Error("FindDMForPR() found = false, want true")
	}

	if channelID != "dm-user-123" {
		t.Errorf("FindDMForPR() channelID = %q, want %q", channelID, "dm-user-123")
	}

	if messageID != "dm-msg-1" {
		t.Errorf("FindDMForPR() messageID = %q, want %q", messageID, "dm-msg-1")
	}
}

// TestClient_FindDMForPR_NotFound tests FindDMForPR when DM not found.
func TestClient_FindDMForPR_NotFound(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.session.GetState().User = &discordgo.User{
		ID:       "bot-user-id",
		Username: "testbot",
	}

	ctx := context.Background()
	_, _, found := client.FindDMForPR(ctx, "user-123", "https://github.com/owner/repo/pull/999")

	if found {
		t.Error("FindDMForPR() found = true, want false for non-existent PR")
	}
}

// TestClient_FindDMForPR_NoBotUser tests FindDMForPR when bot user not set.
func TestClient_FindDMForPR_NoBotUser(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MockState.User = nil

	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	_, _, found := client.FindDMForPR(ctx, "user-123", "https://github.com/owner/repo/pull/123")

	if found {
		t.Error("FindDMForPR() found = true, want false when bot user is nil")
	}
}

// TestClient_IsUserInGuild_Success tests IsUserInGuild when user is a member.
func TestClient_IsUserInGuild_Success(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	// Add a member to the guild
	mockSession.AddMember("test-guild", &discordgo.Member{
		User: &discordgo.User{
			ID:       "user-123",
			Username: "testuser",
		},
	})

	got := client.IsUserInGuild(context.Background(), "user-123")
	if !got {
		t.Error("IsUserInGuild() = false, want true when user is a guild member")
	}
}

// TestClient_ResolveChannelID_GuildLookup tests ResolveChannelID with guild channel lookup.
func TestClient_ResolveChannelID_GuildLookup(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	// Add a channel to the guild
	mockSession.AddChannel(&discordgo.Channel{
		ID:      "channel-456",
		Name:    "general",
		GuildID: "test-guild",
	})

	// Resolve by name
	got := client.ResolveChannelID(context.Background(), "general")
	if got != "channel-456" {
		t.Errorf("ResolveChannelID(\"general\") = %q, want \"channel-456\"", got)
	}

	// Verify it was cached
	if client.channelCache["general"] != "channel-456" {
		t.Error("Channel should be cached after resolution")
	}
}

// TestClient_ResolveChannelID_GuildLookupNotFound tests ResolveChannelID when channel not found.
func TestClient_ResolveChannelID_GuildLookupNotFound(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	// Don't add any channels

	// Resolve by name
	got := client.ResolveChannelID(context.Background(), "nonexistent")
	if got != "nonexistent" {
		t.Errorf("ResolveChannelID(\"nonexistent\") = %q, want \"nonexistent\"", got)
	}
}

// TestClient_ResolveChannelID_WithHashPrefix tests ResolveChannelID with # prefix.
func TestClient_ResolveChannelID_WithHashPrefix(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	// Add a channel
	mockSession.AddChannel(&discordgo.Channel{
		ID:      "channel-789",
		Name:    "announcements",
		GuildID: "test-guild",
	})

	// Resolve with # prefix
	got := client.ResolveChannelID(context.Background(), "#announcements")
	if got != "channel-789" {
		t.Errorf("ResolveChannelID(\"#announcements\") = %q, want \"channel-789\"", got)
	}
}

// Note: FindForumThread tests are skipped because they require realSession.ThreadsArchived()
// which is complex to mock. The method uses realSession directly for archived threads.

// TestClient_LookupUserByUsername_Success tests successful user lookup.
func TestClient_LookupUserByUsername_Success(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	// Add members to the guild
	mockSession.AddMember("test-guild", &discordgo.Member{
		User: &discordgo.User{
			ID:       "user-123",
			Username: "testuser",
		},
	})
	mockSession.AddMember("test-guild", &discordgo.Member{
		User: &discordgo.User{
			ID:         "user-456",
			Username:   "anotheruser",
			GlobalName: "Another User",
		},
	})

	ctx := context.Background()
	userID := client.LookupUserByUsername(ctx, "testuser")

	if userID != "user-123" {
		t.Errorf("LookupUserByUsername() userID = %q, want \"user-123\"", userID)
	}

	// Verify it was cached
	if client.userCache["testuser"] != "user-123" {
		t.Error("User should be cached after lookup")
	}
}

// TestClient_LookupUserByUsername_GlobalName tests lookup by global name.
func TestClient_LookupUserByUsername_GlobalName(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	mockSession.AddMember("test-guild", &discordgo.Member{
		User: &discordgo.User{
			ID:         "user-789",
			Username:   "someuser",
			GlobalName: "Display Name",
		},
	})

	ctx := context.Background()
	userID := client.LookupUserByUsername(ctx, "Display Name")

	if userID != "user-789" {
		t.Errorf("LookupUserByUsername() by global name userID = %q, want \"user-789\"", userID)
	}
}

// TestClient_LookupUserByUsername_Nick tests lookup by nickname.
func TestClient_LookupUserByUsername_Nick(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	mockSession.AddMember("test-guild", &discordgo.Member{
		Nick: "CoolNick",
		User: &discordgo.User{
			ID:       "user-999",
			Username: "plainuser",
		},
	})

	ctx := context.Background()
	userID := client.LookupUserByUsername(ctx, "CoolNick")

	if userID != "user-999" {
		t.Errorf("LookupUserByUsername() by nickname userID = %q, want \"user-999\"", userID)
	}
}

// TestClient_LookupUserByUsername_CaseInsensitive tests case-insensitive matching.
func TestClient_LookupUserByUsername_CaseInsensitive(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	mockSession.AddMember("test-guild", &discordgo.Member{
		User: &discordgo.User{
			ID:       "user-case",
			Username: "CamelCase",
		},
	})

	ctx := context.Background()
	userID := client.LookupUserByUsername(ctx, "camelcase")

	if userID != "user-case" {
		t.Errorf("LookupUserByUsername() case-insensitive userID = %q, want \"user-case\"", userID)
	}
}

// TestClient_LookupUserByUsername_PrefixMatch tests unambiguous prefix matching.
func TestClient_LookupUserByUsername_PrefixMatch(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	mockSession.AddMember("test-guild", &discordgo.Member{
		User: &discordgo.User{
			ID:       "user-prefix",
			Username: "PrefixedUser",
		},
	})

	ctx := context.Background()
	userID := client.LookupUserByUsername(ctx, "prefix")

	if userID != "user-prefix" {
		t.Errorf("LookupUserByUsername() prefix match userID = %q, want \"user-prefix\"", userID)
	}
}

// TestClient_LookupUserByUsername_AmbiguousPrefix tests ambiguous prefix matching.
func TestClient_LookupUserByUsername_AmbiguousPrefix(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	mockSession.AddMember("test-guild", &discordgo.Member{
		User: &discordgo.User{
			ID:       "user-1",
			Username: "TestUser1",
		},
	})
	mockSession.AddMember("test-guild", &discordgo.Member{
		User: &discordgo.User{
			ID:       "user-2",
			Username: "TestUser2",
		},
	})

	ctx := context.Background()
	userID := client.LookupUserByUsername(ctx, "test")

	// Should return empty for ambiguous match
	if userID != "" {
		t.Errorf("LookupUserByUsername() ambiguous prefix userID = %q, want empty", userID)
	}
}

// TestClient_LookupUserByUsername_NotFound tests when user is not found.
func TestClient_LookupUserByUsername_NotFound(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	mockSession.AddMember("test-guild", &discordgo.Member{
		User: &discordgo.User{
			ID:       "user-other",
			Username: "someuser",
		},
	})

	ctx := context.Background()
	userID := client.LookupUserByUsername(ctx, "nonexistent")

	if userID != "" {
		t.Errorf("LookupUserByUsername() not found userID = %q, want empty", userID)
	}
}

// TestClient_IsBotInChannel_Success tests IsBotInChannel when bot is in channel.
func TestClient_IsBotInChannel_Success(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MockState.User = &discordgo.User{
		ID:       "bot-id",
		Username: "testbot",
	}

	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	// MockSession's UserChannelPermissions returns PermissionAll by default
	ctx := context.Background()
	got := client.IsBotInChannel(ctx, "channel-123")
	if !got {
		t.Error("IsBotInChannel() = false, want true when bot has permissions")
	}
}

// TestClient_IsBotInChannel_PermissionError tests IsBotInChannel when permission check fails.
func TestClient_IsBotInChannel_PermissionError(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MockState.User = &discordgo.User{
		ID:       "bot-id",
		Username: "testbot",
	}
	mockSession.UserChannelPermissionsError = fmt.Errorf("permission check failed")

	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	got := client.IsBotInChannel(ctx, "channel-123")
	if got {
		t.Error("IsBotInChannel() = true, want false when permission check fails")
	}
}

// TestClient_IsUserInGuild_Error tests IsUserInGuild when GuildMember fails.
func TestClient_IsUserInGuild_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.GuildMemberError = fmt.Errorf("member not found")
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	got := client.IsUserInGuild(context.Background(), "user-123")
	if got {
		t.Error("IsUserInGuild() = true, want false when GuildMember fails")
	}
}

// TestClient_IsUserActive_Error tests IsUserActive when guild state is unavailable.
func TestClient_IsUserActive_Error(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	// Don't add guild to state, so state lookup will fail
	ctx := context.Background()
	got := client.IsUserActive(ctx, "user-123")
	if got {
		t.Error("IsUserActive() = true, want false when guild state unavailable")
	}
}

// TestClient_GuildInfo_Error tests GuildInfo when guild lookup fails.
func TestClient_GuildInfo_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.GuildError = fmt.Errorf("guild not found")
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	ctx := context.Background()
	_, err := client.GuildInfo(ctx)
	if err == nil {
		t.Error("GuildInfo() error = nil, want error when guild lookup fails")
	}
}

// TestClient_ResolveChannelID_Error tests ResolveChannelID when GuildChannels fails.
func TestClient_ResolveChannelID_Error(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.GuildChannelsError = fmt.Errorf("channels fetch failed")
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	ctx := context.Background()
	got := client.ResolveChannelID(ctx, "test-channel")

	// Should return input unchanged when fetch fails
	if got != "test-channel" {
		t.Errorf("ResolveChannelID() = %q, want %q when fetch fails", got, "test-channel")
	}
}

// TestClient_IsUserActive_Success tests IsUserActive when user is active.
func TestClient_IsUserActive_Success(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	// Add guild with presence to the state
	guild := &discordgo.Guild{
		ID:   "test-guild",
		Name: "Test Guild",
		Presences: []*discordgo.Presence{
			{
				User: &discordgo.User{
					ID: "user-123",
				},
				Status: discordgo.StatusOnline,
			},
		},
	}
	mockSession.MockState.GuildAdd(guild)

	ctx := context.Background()
	got := client.IsUserActive(ctx, "user-123")
	if !got {
		t.Error("IsUserActive() = false, want true when user is online")
	}
}

// TestClient_GuildInfo_Success tests successful guild info retrieval.
func TestClient_GuildInfo_Success(t *testing.T) {
	mockSession := NewMockSession()
	client := newTestClientWithMock(mockSession)
	client.SetGuildID("test-guild")

	// Add a guild
	mockSession.Guilds["test-guild"] = &discordgo.Guild{
		ID:   "test-guild",
		Name: "Test Guild",
	}

	ctx := context.Background()
	guildInfo, err := client.GuildInfo(ctx)
	if err != nil {
		t.Fatalf("GuildInfo() error = %v, want nil", err)
	}
	if guildInfo.ID != "test-guild" {
		t.Errorf("GuildInfo() guild.ID = %q, want \"test-guild\"", guildInfo.ID)
	}
	if guildInfo.Name != "Test Guild" {
		t.Errorf("GuildInfo() guild.Name = %q, want \"Test Guild\"", guildInfo.Name)
	}
}

// TestClient_BotInfo_Success tests successful bot info retrieval.
func TestClient_BotInfo_Success(t *testing.T) {
	mockSession := NewMockSession()
	mockSession.MockState.User = &discordgo.User{
		ID:       "bot-id",
		Username: "testbot",
	}

	client := newTestClientWithMock(mockSession)

	ctx := context.Background()
	botInfo, err := client.BotInfo(ctx)
	if err != nil {
		t.Fatalf("BotInfo() error = %v, want nil", err)
	}
	if botInfo.UserID != "bot-id" {
		t.Errorf("BotInfo() botInfo.UserID = %q, want \"bot-id\"", botInfo.UserID)
	}
	if botInfo.Username != "testbot" {
		t.Errorf("BotInfo() botInfo.Username = %q, want \"testbot\"", botInfo.Username)
	}
}

// TestSessionAdapter_GetState tests the sessionAdapter.GetState method
func TestSessionAdapter_GetState(t *testing.T) {
	state := discordgo.NewState()
	state.User = &discordgo.User{
		ID:       "bot-123",
		Username: "testbot",
	}

	session := &discordgo.Session{
		State: state,
	}

	adapter := &sessionAdapter{Session: session}

	gotState := adapter.GetState()
	if gotState != state {
		t.Errorf("GetState() = %v, want %v", gotState, state)
	}

	if gotState.User.ID != "bot-123" {
		t.Errorf("GetState().User.ID = %v, want bot-123", gotState.User.ID)
	}
}
