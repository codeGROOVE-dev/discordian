package format

import (
	"strings"
	"testing"
)

func TestStateEmoji(t *testing.T) {
	tests := []struct {
		state PRState
		want  string
	}{
		{StateDraft, EmojiDraft},
		{StateTestsRunning, EmojiTestsRunning},
		{StateTestsBroken, EmojiTestsBroken},
		{StateNeedsReview, EmojiNeedsReview},
		{StateChanges, EmojiChanges},
		{StateApproved, EmojiApproved},
		{StateMerged, EmojiMerged},
		{StateClosed, EmojiClosed},
		{StateConflict, EmojiConflict},
		{StateUnknown, EmojiUnknown},
		{"invalid", EmojiUnknown},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			got := StateEmoji(tt.state)
			if got != tt.want {
				t.Errorf("StateEmoji(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestChannelMessage(t *testing.T) {
	tests := []struct {
		name   string
		params ChannelMessageParams
		want   []string // substrings that must be present
	}{
		{
			name: "basic message",
			params: ChannelMessageParams{
				Owner:  "org",
				Repo:   "repo",
				Number: 123,
				Title:  "Fix the bug",
				Author: "alice",
				State:  StateNeedsReview,
				PRURL:  "https://github.com/org/repo/pull/123",
			},
			want: []string{
				EmojiNeedsReview,
				"[repo#123]",
				"Fix the bug",
				"alice",
			},
		},
		{
			name: "with action users",
			params: ChannelMessageParams{
				Owner:  "org",
				Repo:   "repo",
				Number: 42,
				Title:  "New feature",
				Author: "bob",
				State:  StateChanges,
				PRURL:  "https://github.com/org/repo/pull/42",
				ActionUsers: []ActionUser{
					{Username: "charlie", Mention: "<@123>", Action: "needs to review"},
				},
			},
			want: []string{
				EmojiChanges,
				"<@123>",
				"needs to review",
			},
		},
		{
			name: "multiple action users",
			params: ChannelMessageParams{
				Owner:  "org",
				Repo:   "repo",
				Number: 1,
				Title:  "Test",
				Author: "author",
				State:  StateNeedsReview,
				PRURL:  "https://github.com/org/repo/pull/1",
				ActionUsers: []ActionUser{
					{Username: "a", Mention: "<@1>", Action: "review"},
					{Username: "b", Mention: "<@2>", Action: "approve"},
				},
			},
			want: []string{
				"<@1>",
				"<@2>",
				", ",
			},
		},
		{
			name: "long title truncated",
			params: ChannelMessageParams{
				Owner:  "org",
				Repo:   "repo",
				Number: 1,
				Title:  strings.Repeat("a", 100),
				Author: "author",
				State:  StateDraft,
				PRURL:  "https://github.com/org/repo/pull/1",
			},
			want: []string{
				"...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChannelMessage(tt.params)
			for _, substr := range tt.want {
				if !strings.Contains(got, substr) {
					t.Errorf("ChannelMessage() = %q, want to contain %q", got, substr)
				}
			}
		})
	}
}

func TestForumThreadTitle(t *testing.T) {
	tests := []struct {
		name   string
		repo   string
		number int
		title  string
		want   string
	}{
		{
			name:   "short title",
			repo:   "myrepo",
			number: 42,
			title:  "Fix bug",
			want:   "[myrepo#42] Fix bug",
		},
		{
			name:   "title at limit",
			repo:   "repo",
			number: 1,
			title:  strings.Repeat("a", 90),
			want:   "[repo#1] " + strings.Repeat("a", 90),
		},
		{
			name:   "very long title truncated",
			repo:   "repo",
			number: 123,
			title:  strings.Repeat("x", 200),
			want:   "[repo#123] ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ForumThreadTitle(tt.repo, tt.number, tt.title)
			if len(got) > 100 {
				t.Errorf("ForumThreadTitle() length = %d, want <= 100", len(got))
			}
			if !strings.HasPrefix(got, tt.want[:min(len(got), len(tt.want))]) {
				t.Errorf("ForumThreadTitle() = %q, want prefix %q", got, tt.want)
			}
		})
	}
}

func TestDMMessage(t *testing.T) {
	params := ChannelMessageParams{
		Owner:  "org",
		Repo:   "repo",
		Number: 99,
		Title:  "Important PR",
		Author: "dave",
		State:  StateApproved,
		PRURL:  "https://github.com/org/repo/pull/99",
	}

	t.Run("with action", func(t *testing.T) {
		got := DMMessage(params, "Review needed")
		if !strings.Contains(got, "**Review needed**") {
			t.Errorf("DMMessage() = %q, want to contain bold action", got)
		}
		if !strings.Contains(got, "org/repo#99") {
			t.Errorf("DMMessage() = %q, want to contain full PR ref", got)
		}
	})

	t.Run("without action", func(t *testing.T) {
		got := DMMessage(params, "")
		if strings.Contains(got, "**") {
			t.Errorf("DMMessage() = %q, should not contain bold markers", got)
		}
	})
}

func TestStateFromAnalysis(t *testing.T) {
	tests := []struct {
		name          string
		merged        bool
		closed        bool
		draft         bool
		workflowState string
		checksFailing int
		mergeConflict bool
		want          PRState
	}{
		{"merged PR", true, false, false, "", 0, false, StateMerged},
		{"closed PR", false, true, false, "", 0, false, StateClosed},
		{"draft PR", false, false, true, "", 0, false, StateDraft},
		{"merge conflict", false, false, false, "", 0, true, StateConflict},
		{"failing checks", false, false, false, "", 2, false, StateTestsBroken},
		{"in draft workflow", false, false, false, "IN_DRAFT", 0, false, StateDraft},
		{"newly published", false, false, false, "NEWLY_PUBLISHED", 0, false, StateDraft},
		{"waiting for tests", false, false, false, "PUBLISHED_WAITING_FOR_TESTS", 0, false, StateTestsRunning},
		{"tests failed workflow", false, false, false, "TESTED_WAITING_FOR_FIXES", 0, false, StateTestsBroken},
		{"needs refinement", false, false, false, "REVIEWED_NEEDS_REFINEMENT", 0, false, StateChanges},
		{"approved", false, false, false, "APPROVED_WAITING_FOR_MERGE", 0, false, StateApproved},
		{"waiting for assignment", false, false, false, "TESTED_WAITING_FOR_ASSIGNMENT", 0, false, StateNeedsReview},
		{"waiting for review", false, false, false, "ASSIGNED_WAITING_FOR_REVIEW", 0, false, StateNeedsReview},
		{"waiting for approval", false, false, false, "REFINED_WAITING_FOR_APPROVAL", 0, false, StateNeedsReview},
		{"unknown workflow", false, false, false, "SOMETHING_ELSE", 0, false, StateUnknown},
		{"empty workflow", false, false, false, "", 0, false, StateUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StateFromAnalysis(tt.merged, tt.closed, tt.draft, tt.workflowState, tt.checksFailing, tt.mergeConflict)
			if got != tt.want {
				t.Errorf("StateFromAnalysis() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActionLabel(t *testing.T) {
	tests := []struct {
		action string
		want   string
	}{
		{"review", "needs to review"},
		{"re_review", "needs to re-review"},
		{"approve", "needs to approve"},
		{"resolve_comments", "needs to resolve comments"},
		{"fix_tests", "needs to fix tests"},
		{"fix_conflict", "needs to fix merge conflict"},
		{"merge", "ready to merge"},
		{"publish_draft", "needs to publish draft"},
		{"request_reviewers", "needs to request reviewers"},
		{"unknown_action", "unknown_action"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := ActionLabel(tt.action)
			if got != tt.want {
				t.Errorf("ActionLabel(%q) = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"no truncation needed", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short max", "hello", 2, "he"},
		{"max 3", "hello", 3, "hel"},
		{"max 4", "hello", 4, "h..."},
		{"empty string", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
			if len(got) > tt.maxLen {
				t.Errorf("Truncate(%q, %d) length = %d, want <= %d", tt.input, tt.maxLen, len(got), tt.maxLen)
			}
		})
	}
}
