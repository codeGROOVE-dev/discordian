package bot

import (
	"context"
	"testing"
	"time"

	"github.com/codeGROOVE-dev/discordian/internal/config"
	"github.com/codeGROOVE-dev/discordian/internal/state"
)

// Mock implementations

type mockDiscordClient struct {
	postedMessages  []postedMessage
	updatedMessages []updatedMessage
	forumThreads    []forumThread
	sentDMs         []sentDM
	updatedDMs      []updatedDM
	channelIDs      map[string]string
	forumChannels   map[string]bool
	usersInGuild    map[string]bool
	botInChannel    map[string]bool
	guildID         string
}

type postedMessage struct {
	channelID string
	text      string
}

type updatedMessage struct {
	channelID string
	messageID string
	text      string
}

type forumThread struct {
	forumID string
	title   string
	content string
}

type sentDM struct {
	userID string
	text   string
}

type updatedDM struct {
	channelID string
	messageID string
	text      string
}

func newMockDiscordClient() *mockDiscordClient {
	return &mockDiscordClient{
		channelIDs:    make(map[string]string),
		forumChannels: make(map[string]bool),
		usersInGuild:  make(map[string]bool),
		botInChannel:  make(map[string]bool),
		guildID:       "test-guild",
	}
}

func (m *mockDiscordClient) PostMessage(_ context.Context, channelID, text string) (string, error) {
	m.postedMessages = append(m.postedMessages, postedMessage{channelID, text})
	return "msg-" + channelID, nil
}

func (m *mockDiscordClient) UpdateMessage(_ context.Context, channelID, messageID, text string) error {
	m.updatedMessages = append(m.updatedMessages, updatedMessage{channelID, messageID, text})
	return nil
}

func (m *mockDiscordClient) PostForumThread(_ context.Context, forumID, title, content string) (threadID, messageID string, err error) {
	m.forumThreads = append(m.forumThreads, forumThread{forumID, title, content})
	return "thread-" + forumID, "msg-" + forumID, nil
}

func (m *mockDiscordClient) UpdateForumPost(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (m *mockDiscordClient) ArchiveThread(_ context.Context, _ string) error {
	return nil
}

func (m *mockDiscordClient) SendDM(_ context.Context, userID, text string) (channelID, messageID string, err error) {
	m.sentDMs = append(m.sentDMs, sentDM{userID, text})
	return "dm-chan-" + userID, "dm-msg-" + userID, nil
}

func (m *mockDiscordClient) UpdateDM(_ context.Context, channelID, messageID, text string) error {
	m.updatedDMs = append(m.updatedDMs, updatedDM{channelID, messageID, text})
	return nil
}

func (m *mockDiscordClient) ResolveChannelID(_ context.Context, channelName string) string {
	if id, ok := m.channelIDs[channelName]; ok {
		return id
	}
	return channelName // Return name if not found (signals not found)
}

func (m *mockDiscordClient) LookupUserByUsername(_ context.Context, _ string) string {
	return ""
}

func (m *mockDiscordClient) IsBotInChannel(_ context.Context, channelID string) bool {
	return m.botInChannel[channelID]
}

func (m *mockDiscordClient) IsUserInGuild(_ context.Context, userID string) bool {
	return m.usersInGuild[userID]
}

func (m *mockDiscordClient) IsUserActive(_ context.Context, _ string) bool {
	return false
}

func (m *mockDiscordClient) IsForumChannel(_ context.Context, channelID string) bool {
	return m.forumChannels[channelID]
}

func (m *mockDiscordClient) GuildID() string {
	return m.guildID
}

func (m *mockDiscordClient) FindForumThread(_ context.Context, _, _ string) (string, string, bool) {
	return "", "", false
}

func (m *mockDiscordClient) FindChannelMessage(_ context.Context, _, _ string) (string, bool) {
	return "", false
}

func (m *mockDiscordClient) FindDMForPR(_ context.Context, _, _ string) (string, string, bool) {
	return "", "", false
}

func (m *mockDiscordClient) MessageContent(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

type mockConfigManager struct {
	configs  map[string]*config.DiscordConfig
	channels map[string][]string // org:repo -> channels
}

func newMockConfigManager() *mockConfigManager {
	return &mockConfigManager{
		configs:  make(map[string]*config.DiscordConfig),
		channels: make(map[string][]string),
	}
}

func (m *mockConfigManager) LoadConfig(_ context.Context, _ string) error {
	return nil
}

func (m *mockConfigManager) ReloadConfig(_ context.Context, _ string) error {
	return nil
}

func (m *mockConfigManager) Config(org string) (*config.DiscordConfig, bool) {
	cfg, ok := m.configs[org]
	return cfg, ok
}

func (m *mockConfigManager) ChannelsForRepo(org, repo string) []string {
	key := org + ":" + repo
	if channels, ok := m.channels[key]; ok {
		return channels
	}
	return []string{repo} // Default: use repo name as channel
}

func (m *mockConfigManager) ChannelType(_, _ string) string {
	return "text"
}

func (m *mockConfigManager) DiscordUserID(_, _ string) string {
	return ""
}

func (m *mockConfigManager) ReminderDMDelay(_, _ string) int {
	return 65
}

func (m *mockConfigManager) When(_, _ string) string {
	return "immediate"
}

func (m *mockConfigManager) GuildID(_ string) string {
	return "test-guild"
}

func (m *mockConfigManager) SetGitHubClient(_ string, _ any) {}

type mockTurnClient struct {
	responses map[string]*CheckResponse
}

func newMockTurnClient() *mockTurnClient {
	return &mockTurnClient{
		responses: make(map[string]*CheckResponse),
	}
}

func (m *mockTurnClient) Check(_ context.Context, prURL, _ string, _ time.Time) (*CheckResponse, error) {
	if resp, ok := m.responses[prURL]; ok {
		return resp, nil
	}
	return &CheckResponse{}, nil
}

type mockUserMapper struct {
	mappings map[string]string // GitHub username -> Discord ID
}

func newMockUserMapper() *mockUserMapper {
	return &mockUserMapper{
		mappings: make(map[string]string),
	}
}

func (m *mockUserMapper) DiscordID(_ context.Context, githubUsername string) string {
	return m.mappings[githubUsername]
}

func (m *mockUserMapper) Mention(_ context.Context, githubUsername string) string {
	if id := m.mappings[githubUsername]; id != "" {
		return "<@" + id + ">"
	}
	return githubUsername
}

func TestNewCoordinator(t *testing.T) {
	discord := newMockDiscordClient()
	configMgr := newMockConfigManager()
	store := state.NewMemoryStore()
	turn := newMockTurnClient()
	userMapper := newMockUserMapper()

	coord := NewCoordinator(CoordinatorConfig{
		Discord:    discord,
		Config:     configMgr,
		Store:      store,
		Turn:       turn,
		UserMapper: userMapper,
		Org:        "testorg",
	})

	if coord == nil {
		t.Fatal("expected non-nil coordinator")
	}
	if coord.org != "testorg" {
		t.Errorf("expected org 'testorg', got %s", coord.org)
	}
	if coord.discord != discord {
		t.Error("discord client not set")
	}
	if coord.config != configMgr {
		t.Error("config manager not set")
	}
}

func TestCoordinator_ProcessEvent_BasicFlow(t *testing.T) {
	ctx := context.Background()

	discord := newMockDiscordClient()
	discord.channelIDs["testrepo"] = "chan-testrepo"
	discord.botInChannel["chan-testrepo"] = true

	configMgr := newMockConfigManager()
	store := state.NewMemoryStore()
	turn := newMockTurnClient()
	turn.responses["https://github.com/testorg/testrepo/pull/42"] = &CheckResponse{
		PullRequest: PRInfo{
			Title:  "Test PR",
			Author: "alice",
			State:  "open",
		},
		Analysis: Analysis{
			NextAction: map[string]Action{
				"bob": {Kind: "review"},
			},
			WorkflowState: "awaiting_review",
		},
	}

	coord := NewCoordinator(CoordinatorConfig{
		Discord: discord,
		Config:  configMgr,
		Store:   store,
		Turn:    turn,
		Org:     "testorg",
	})

	event := SprinklerEvent{
		URL:        "https://github.com/testorg/testrepo/pull/42",
		Type:       "pull_request",
		DeliveryID: "delivery-1",
	}

	coord.ProcessEvent(ctx, event)
	coord.Wait()

	if len(discord.postedMessages) != 1 {
		t.Errorf("Expected 1 posted message, got %d", len(discord.postedMessages))
	}
}

func TestCoordinator_ProcessEvent_Deduplication(t *testing.T) {
	ctx := context.Background()

	discord := newMockDiscordClient()
	discord.channelIDs["testrepo"] = "chan-testrepo"
	discord.botInChannel["chan-testrepo"] = true

	configMgr := newMockConfigManager()
	store := state.NewMemoryStore()
	turn := newMockTurnClient()
	turn.responses["https://github.com/testorg/testrepo/pull/42"] = &CheckResponse{
		PullRequest: PRInfo{
			Title:  "Test PR",
			Author: "alice",
			State:  "open",
		},
		Analysis: Analysis{
			NextAction: map[string]Action{
				"bob": {Kind: "review"},
			},
		},
	}

	coord := NewCoordinator(CoordinatorConfig{
		Discord: discord,
		Config:  configMgr,
		Store:   store,
		Turn:    turn,
		Org:     "testorg",
	})

	event := SprinklerEvent{
		URL:        "https://github.com/testorg/testrepo/pull/42",
		Type:       "pull_request",
		DeliveryID: "delivery-same",
	}

	// Process same event twice
	coord.ProcessEvent(ctx, event)
	coord.Wait()

	coord.ProcessEvent(ctx, event)
	coord.Wait()

	// Should only create 1 message (deduplication works)
	if len(discord.postedMessages) != 1 {
		t.Errorf("Expected 1 posted message due to deduplication, got %d", len(discord.postedMessages))
	}
}

func TestCoordinator_ProcessForumChannel(t *testing.T) {
	ctx := context.Background()

	discord := newMockDiscordClient()
	discord.channelIDs["testrepo"] = "chan-testrepo"
	discord.botInChannel["chan-testrepo"] = true
	discord.forumChannels["chan-testrepo"] = true // Mark as forum

	configMgr := newMockConfigManager()
	store := state.NewMemoryStore()
	turn := newMockTurnClient()
	turn.responses["https://github.com/testorg/testrepo/pull/42"] = &CheckResponse{
		PullRequest: PRInfo{
			Title:  "Test PR",
			Author: "alice",
			State:  "open",
		},
		Analysis: Analysis{
			NextAction: map[string]Action{
				"bob": {Kind: "review"},
			},
		},
	}

	coord := NewCoordinator(CoordinatorConfig{
		Discord: discord,
		Config:  configMgr,
		Store:   store,
		Turn:    turn,
		Org:     "testorg",
	})

	event := SprinklerEvent{
		URL:        "https://github.com/testorg/testrepo/pull/42",
		Type:       "pull_request",
		DeliveryID: "delivery-forum",
	}

	coord.ProcessEvent(ctx, event)
	coord.Wait()

	if len(discord.forumThreads) != 1 {
		t.Errorf("Expected 1 forum thread, got %d", len(discord.forumThreads))
	}
}

func TestCoordinator_ConfigReload(t *testing.T) {
	ctx := context.Background()

	discord := newMockDiscordClient()
	configMgr := newMockConfigManager()
	store := state.NewMemoryStore()
	turn := newMockTurnClient()

	coord := NewCoordinator(CoordinatorConfig{
		Discord: discord,
		Config:  configMgr,
		Store:   store,
		Turn:    turn,
		Org:     "testorg",
	})

	// Test config repo PR (should trigger reload)
	event := SprinklerEvent{
		URL:        "https://github.com/testorg/.codeGROOVE/pull/1",
		Type:       "pull_request",
		DeliveryID: "delivery-config",
	}

	coord.ProcessEvent(ctx, event)
	coord.Wait()

	// Should not create any messages for config repo
	if len(discord.postedMessages) != 0 {
		t.Errorf("Expected 0 messages for config repo, got %d", len(discord.postedMessages))
	}
}

func TestCoordinator_QueueDMNotifications_Disabled(t *testing.T) {
	ctx := context.Background()

	discord := newMockDiscordClient()
	discord.channelIDs["testrepo"] = "chan-testrepo"
	discord.botInChannel["chan-testrepo"] = true
	discord.usersInGuild["discord-bob"] = true

	configMgr := &mockConfigManagerWithDelay{
		mockConfigManager: newMockConfigManager(),
		delay:             0, // DMs disabled
	}
	store := state.NewMemoryStore()
	turn := newMockTurnClient()
	turn.responses["https://github.com/testorg/testrepo/pull/42"] = &CheckResponse{
		PullRequest: PRInfo{
			Title:  "Test PR",
			Author: "alice",
			State:  "open",
		},
		Analysis: Analysis{
			NextAction: map[string]Action{
				"bob": {Kind: "review"},
			},
		},
	}

	mapper := newMockUserMapper()
	mapper.mappings["bob"] = "discord-bob"

	coord := NewCoordinator(CoordinatorConfig{
		Discord:    discord,
		Config:     configMgr,
		Store:      store,
		Turn:       turn,
		UserMapper: mapper,
		Org:        "testorg",
	})

	event := SprinklerEvent{
		URL:        "https://github.com/testorg/testrepo/pull/42",
		Type:       "pull_request",
		DeliveryID: "delivery-nodm",
	}

	coord.ProcessEvent(ctx, event)
	coord.Wait()

	// No DMs should be queued when delay=0
	pending, err := store.PendingDMs(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("PendingDMs() error = %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending DMs with delay=0, got %d", len(pending))
	}
}

type mockConfigManagerWithDelay struct {
	*mockConfigManager

	delay int
}

func (m *mockConfigManagerWithDelay) ReminderDMDelay(_, _ string) int {
	return m.delay
}

func TestCoordinator_QueueDMNotifications_Tagged(t *testing.T) {
	ctx := context.Background()

	discord := newMockDiscordClient()
	discord.channelIDs["testrepo"] = "chan-testrepo"
	discord.botInChannel["chan-testrepo"] = true
	discord.usersInGuild["discord-bob"] = true

	configMgr := &mockConfigManagerWithDelay{
		mockConfigManager: newMockConfigManager(),
		delay:             30, // 30 minute delay
	}
	store := state.NewMemoryStore()
	turn := newMockTurnClient()
	turn.responses["https://github.com/testorg/testrepo/pull/42"] = &CheckResponse{
		PullRequest: PRInfo{
			Title:  "Test PR",
			Author: "alice",
			State:  "open",
		},
		Analysis: Analysis{
			NextAction: map[string]Action{
				"bob": {Kind: "review"},
			},
		},
	}

	mapper := newMockUserMapper()
	mapper.mappings["bob"] = "discord-bob"

	coord := NewCoordinator(CoordinatorConfig{
		Discord:    discord,
		Config:     configMgr,
		Store:      store,
		Turn:       turn,
		UserMapper: mapper,
		Org:        "testorg",
	})

	event := SprinklerEvent{
		URL:        "https://github.com/testorg/testrepo/pull/42",
		Type:       "pull_request",
		DeliveryID: "delivery-tagged",
	}

	coord.ProcessEvent(ctx, event)
	coord.Wait()

	// DM should be queued with delay
	pending, err := store.PendingDMs(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("PendingDMs() error = %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending DM, got %d", len(pending))
	}

	// Check that sendAt is delayed by ~30 minutes
	dm := pending[0]
	delay := dm.SendAt.Sub(dm.CreatedAt)
	expectedDelay := 30 * time.Minute
	if delay < expectedDelay-time.Second || delay > expectedDelay+time.Second {
		t.Errorf("Expected delay ~%v, got %v", expectedDelay, delay)
	}
}

func TestCoordinator_QueueDMNotifications_NoMapper(t *testing.T) {
	ctx := context.Background()

	discord := newMockDiscordClient()
	discord.channelIDs["testrepo"] = "chan-testrepo"
	discord.botInChannel["chan-testrepo"] = true
	discord.usersInGuild["discord-bob"] = true

	configMgr := &mockConfigManagerWithDelay{
		mockConfigManager: newMockConfigManager(),
		delay:             30, // 30 minute delay
	}
	store := state.NewMemoryStore()
	turn := newMockTurnClient()
	turn.responses["https://github.com/testorg/testrepo/pull/42"] = &CheckResponse{
		PullRequest: PRInfo{
			Title:  "Test PR",
			Author: "alice",
			State:  "open",
		},
		Analysis: Analysis{
			NextAction: map[string]Action{
				"bob": {Kind: "review"},
			},
		},
	}

	// No user mapper - bob won't be mapped to Discord user
	coord := NewCoordinator(CoordinatorConfig{
		Discord:    discord,
		Config:     configMgr,
		Store:      store,
		Turn:       turn,
		UserMapper: nil, // No mapper
		Org:        "testorg",
	})

	event := SprinklerEvent{
		URL:        "https://github.com/testorg/testrepo/pull/42",
		Type:       "pull_request",
		DeliveryID: "delivery-nomapper",
	}

	coord.ProcessEvent(ctx, event)
	coord.Wait()

	// No DMs should be queued when user cannot be mapped
	pending, err := store.PendingDMs(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("PendingDMs() error = %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending DMs when user unmapped, got %d", len(pending))
	}
}

func TestCoordinator_ClosedPR_UpdatesAllDMs(t *testing.T) {
	ctx := context.Background()

	discord := newMockDiscordClient()
	discord.channelIDs["testrepo"] = "chan-testrepo"
	discord.botInChannel["chan-testrepo"] = true
	discord.usersInGuild["discord-alice"] = true
	discord.usersInGuild["discord-bob"] = true

	configMgr := &mockConfigManagerWithDelay{
		mockConfigManager: newMockConfigManager(),
		delay:             30,
	}
	store := state.NewMemoryStore()
	turn := newMockTurnClient()

	mapper := newMockUserMapper()
	mapper.mappings["alice"] = "discord-alice"
	mapper.mappings["bob"] = "discord-bob"

	coord := NewCoordinator(CoordinatorConfig{
		Discord:    discord,
		Config:     configMgr,
		Store:      store,
		Turn:       turn,
		UserMapper: mapper,
		Org:        "testorg",
	})

	prURL := "https://github.com/testorg/testrepo/pull/42"

	// First, simulate existing DMs for alice and bob
	if err := store.SaveDMInfo(ctx, "discord-alice", prURL, state.DMInfo{
		ChannelID:   "dm-alice",
		MessageID:   "msg-alice",
		MessageText: "Old message",
		LastState:   "awaiting_review",
	}); err != nil {
		t.Fatalf("SaveDMInfo() error = %v", err)
	}
	if err := store.SaveDMInfo(ctx, "discord-bob", prURL, state.DMInfo{
		ChannelID:   "dm-bob",
		MessageID:   "msg-bob",
		MessageText: "Old message",
		LastState:   "awaiting_review",
	}); err != nil {
		t.Fatalf("SaveDMInfo() error = %v", err)
	}

	// Now send merged event
	turn.responses[prURL] = &CheckResponse{
		PullRequest: PRInfo{
			Title:  "Test PR",
			Author: "author",
			State:  "closed",
			Merged: true,
		},
		Analysis: Analysis{
			NextAction: map[string]Action{}, // No actions for merged PR
		},
	}

	event := SprinklerEvent{
		URL:        prURL,
		Type:       "pull_request",
		DeliveryID: "delivery-merged",
	}

	coord.ProcessEvent(ctx, event)
	coord.Wait()

	// Both DMs should be updated
	if len(discord.updatedDMs) != 2 {
		t.Errorf("Expected 2 DM updates for merged PR, got %d", len(discord.updatedDMs))
	}
}

// Skipping TestCoordinator_DM_Idempotency_SameState as it tests existing functionality
// and requires deeper mocking of the format.DMMessage function

func TestShouldPostThread(t *testing.T) {
	coord := NewCoordinator(CoordinatorConfig{
		Org: "testorg",
	})

	tests := []struct {
		name           string
		when           string
		checkResult    *CheckResponse
		wantPost       bool
		wantReasonPart string // Part of the reason string to check for
	}{
		{
			name: "immediate mode always posts",
			when: "immediate",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State: "open",
				},
			},
			wantPost:       true,
			wantReasonPart: "immediate_mode",
		},
		{
			name: "merged PR always posts regardless of when",
			when: "passing",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State:  "closed",
					Merged: true,
				},
			},
			wantPost:       true,
			wantReasonPart: "pr_merged",
		},
		{
			name: "closed PR always posts regardless of when",
			when: "passing",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State:  "closed",
					Merged: false,
				},
			},
			wantPost:       true,
			wantReasonPart: "pr_closed",
		},
		{
			name: "assigned: posts when has assignees",
			when: "assigned",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State:     "open",
					Assignees: []string{"user1", "user2"},
				},
			},
			wantPost:       true,
			wantReasonPart: "has_2_assignees",
		},
		{
			name: "assigned: does not post when no assignees",
			when: "assigned",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State:     "open",
					Assignees: []string{},
				},
			},
			wantPost:       false,
			wantReasonPart: "no_assignees",
		},
		{
			name: "blocked: posts when users are blocked",
			when: "blocked",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State: "open",
				},
				Analysis: Analysis{
					NextAction: map[string]Action{
						"user1": {Kind: "review"},
						"user2": {Kind: "approve"},
					},
				},
			},
			wantPost:       true,
			wantReasonPart: "blocked_on_2_users",
		},
		{
			name: "blocked: does not post when no users blocked",
			when: "blocked",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State: "open",
				},
				Analysis: Analysis{
					NextAction: map[string]Action{},
				},
			},
			wantPost:       false,
			wantReasonPart: "not_blocked_yet",
		},
		{
			name: "blocked: ignores _system sentinel",
			when: "blocked",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State: "open",
				},
				Analysis: Analysis{
					NextAction: map[string]Action{
						"_system": {Kind: "processing"},
					},
				},
			},
			wantPost:       false,
			wantReasonPart: "not_blocked_yet",
		},
		{
			name: "passing: posts when in review state",
			when: "passing",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State: "open",
				},
				Analysis: Analysis{
					WorkflowState: "assigned_waiting_for_review",
				},
			},
			wantPost:       true,
			wantReasonPart: "workflow_state",
		},
		{
			name: "passing: does not post when tests pending",
			when: "passing",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State: "open",
				},
				Analysis: Analysis{
					WorkflowState: "published_waiting_for_tests",
				},
			},
			wantPost:       false,
			wantReasonPart: "waiting_for",
		},
		{
			name: "passing: uses fallback when workflow state unknown and tests passing",
			when: "passing",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State: "open",
				},
				Analysis: Analysis{
					WorkflowState: "unknown_state",
					Checks: Checks{
						Passing: 5,
						Failing: 0,
						Pending: 0,
						Waiting: 0,
					},
				},
			},
			wantPost:       true,
			wantReasonPart: "tests_passed_fallback",
		},
		{
			name: "passing: uses fallback when tests failing",
			when: "passing",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State: "open",
				},
				Analysis: Analysis{
					WorkflowState: "unknown_state",
					Checks: Checks{
						Passing: 3,
						Failing: 2,
					},
				},
			},
			wantPost:       false,
			wantReasonPart: "tests_failing",
		},
		{
			name:           "nil check result returns false",
			when:           "passing",
			checkResult:    nil,
			wantPost:       false,
			wantReasonPart: "no_check_result",
		},
		{
			name: "invalid when value defaults to immediate",
			when: "invalid_value",
			checkResult: &CheckResponse{
				PullRequest: PRInfo{
					State: "open",
				},
			},
			wantPost:       true,
			wantReasonPart: "invalid_config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPost, gotReason := coord.shouldPostThread(tt.checkResult, tt.when)

			if gotPost != tt.wantPost {
				t.Errorf("shouldPostThread() gotPost = %v, wantPost %v", gotPost, tt.wantPost)
			}

			if tt.wantReasonPart != "" && !contains(gotReason, tt.wantReasonPart) {
				t.Errorf("shouldPostThread() reason = %q, want to contain %q", gotReason, tt.wantReasonPart)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
