// Package format provides PR message formatting for Discord.
package format

import (
	"fmt"
	"strings"
)

// PR state emoji mappings.
const (
	EmojiDraft        = "\U0001F4DD"   // Draft
	EmojiTestsRunning = "\u23F3"       // Tests running
	EmojiTestsBroken  = "\U0001F534"   // Tests failing
	EmojiNeedsReview  = "\U0001F7E1"   // Needs review
	EmojiChanges      = "\U0001F7E0"   // Changes requested
	EmojiApproved     = "\u2705"       // Approved
	EmojiMerged       = "\U0001F680"   // Merged
	EmojiClosed       = "\u274C"       // Closed
	EmojiConflict     = "\u26A0\uFE0F" // Merge conflict
	EmojiUnknown      = "\u2753"       // Unknown state
)

// PRState represents the simplified state of a PR for formatting.
type PRState string

// PR state constants.
const (
	StateDraft        PRState = "draft"
	StateTestsRunning PRState = "tests_running"
	StateTestsBroken  PRState = "tests_broken"
	StateNeedsReview  PRState = "needs_review"
	StateChanges      PRState = "changes_requested"
	StateApproved     PRState = "approved"
	StateMerged       PRState = "merged"
	StateClosed       PRState = "closed"
	StateConflict     PRState = "conflict"
	StateUnknown      PRState = "unknown"
)

// StateEmoji returns the emoji for a PR state.
func StateEmoji(state PRState) string {
	switch state {
	case StateDraft:
		return EmojiDraft
	case StateTestsRunning:
		return EmojiTestsRunning
	case StateTestsBroken:
		return EmojiTestsBroken
	case StateChanges:
		return EmojiChanges
	case StateApproved:
		return EmojiApproved
	case StateMerged:
		return EmojiMerged
	case StateClosed:
		return EmojiClosed
	case StateConflict:
		return EmojiConflict
	case StateNeedsReview:
		return EmojiNeedsReview
	case StateUnknown:
		return EmojiUnknown
	}
	return EmojiUnknown
}

// ChannelMessageParams contains parameters for formatting a channel message.
type ChannelMessageParams struct {
	ActionUsers []ActionUser // Users who need to take action
	Owner       string
	Repo        string
	Title       string
	Author      string
	State       PRState
	PRURL       string
	Number      int
}

// ActionUser represents a user who needs to take action.
type ActionUser struct {
	Username string
	Mention  string // Discord mention format or plain username
	Action   string // e.g., "review", "approve", "fix tests"
}

// ChannelMessage formats a PR notification for a text channel.
func ChannelMessage(p ChannelMessageParams) string {
	emoji := StateEmoji(p.State)

	// Format: emoji [repo#123](url) Title · author → @user needs to action
	var sb strings.Builder

	sb.WriteString(emoji)
	sb.WriteString(" ")

	// PR link
	sb.WriteString(fmt.Sprintf("[%s#%d](%s)", p.Repo, p.Number, p.PRURL))
	sb.WriteString(" ")
	sb.WriteString(Truncate(p.Title, 60))

	// Author
	sb.WriteString(" · ")
	sb.WriteString(p.Author)

	// Action users
	if len(p.ActionUsers) > 0 {
		sb.WriteString(" \u2192 ")
		for i, au := range p.ActionUsers {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(au.Mention)
			if au.Action != "" {
				sb.WriteString(" ")
				sb.WriteString(au.Action)
			}
		}
	}

	return sb.String()
}

// ForumThreadTitle formats the title for a forum thread.
func ForumThreadTitle(repo string, number int, title string) string {
	// [repo#123] Title (truncated to fit Discord's 100 char limit)
	prefix := fmt.Sprintf("[%s#%d] ", repo, number)
	maxTitleLen := 100 - len(prefix)
	return prefix + Truncate(title, maxTitleLen)
}

// ForumThreadContent formats the content for a forum thread starter message.
func ForumThreadContent(p ChannelMessageParams) string {
	return ChannelMessage(p)
}

// DMMessage formats a DM notification.
func DMMessage(p ChannelMessageParams, action string) string {
	emoji := StateEmoji(p.State)

	var sb strings.Builder
	sb.WriteString(emoji)
	sb.WriteString(" ")

	// Action prompt
	if action != "" {
		sb.WriteString("**")
		sb.WriteString(action)
		sb.WriteString("**: ")
	}

	// PR link
	sb.WriteString(fmt.Sprintf("[%s/%s#%d](%s)", p.Owner, p.Repo, p.Number, p.PRURL))
	sb.WriteString(" ")
	sb.WriteString(p.Title)

	// Author
	sb.WriteString(" by ")
	sb.WriteString(p.Author)

	return sb.String()
}

// StateFromAnalysis determines PRState from Turn API analysis.
func StateFromAnalysis(merged, closed, draft bool, workflowState string, checksFailng int, mergeConflict bool) PRState {
	if merged {
		return StateMerged
	}
	if closed {
		return StateClosed
	}
	if draft {
		return StateDraft
	}
	if mergeConflict {
		return StateConflict
	}
	if checksFailng > 0 {
		return StateTestsBroken
	}

	// Map workflow states
	switch workflowState {
	case "IN_DRAFT", "NEWLY_PUBLISHED":
		return StateDraft
	case "PUBLISHED_WAITING_FOR_TESTS":
		return StateTestsRunning
	case "TESTED_WAITING_FOR_FIXES":
		return StateTestsBroken
	case "REVIEWED_NEEDS_REFINEMENT":
		return StateChanges
	case "APPROVED_WAITING_FOR_MERGE":
		return StateApproved
	case "TESTED_WAITING_FOR_ASSIGNMENT", "ASSIGNED_WAITING_FOR_REVIEW", "REFINED_WAITING_FOR_APPROVAL":
		return StateNeedsReview
	default:
		return StateUnknown
	}
}

// ActionLabel returns a human-readable label for an action.
func ActionLabel(action string) string {
	switch action {
	case "review":
		return "needs to review"
	case "re_review":
		return "needs to re-review"
	case "approve":
		return "needs to approve"
	case "resolve_comments":
		return "needs to resolve comments"
	case "fix_tests":
		return "needs to fix tests"
	case "fix_conflict":
		return "needs to fix merge conflict"
	case "merge":
		return "ready to merge"
	case "publish_draft":
		return "needs to publish draft"
	case "request_reviewers":
		return "needs to request reviewers"
	default:
		return action
	}
}

// Truncate truncates a string to maxLen, adding "..." if truncated.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
