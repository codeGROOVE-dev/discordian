package discord

import (
	"strings"
	"testing"

	"github.com/codeGROOVE-dev/discordian/internal/format"
)

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
		{"unicode", "hello world", 8, "hello..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.Truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
			if len(got) > tt.maxLen {
				t.Errorf("Truncate(%q, %d) length = %d, want <= %d", tt.input, tt.maxLen, len(got), tt.maxLen)
			}
		})
	}
}

func TestFormatReportEmbed(t *testing.T) {
	handler := &SlashCommandHandler{}

	t.Run("empty report", func(t *testing.T) {
		report := &PRReport{
			GeneratedAt: "2024-01-15 10:00 UTC",
		}
		embed := handler.formatReportEmbed(report)

		if embed.Title != "Your PR Report" {
			t.Errorf("Title = %q, want 'Your PR Report'", embed.Title)
		}
		if len(embed.Fields) != 0 {
			t.Errorf("Fields = %d, want 0 for empty report", len(embed.Fields))
		}
	})

	t.Run("with incoming PRs", func(t *testing.T) {
		report := &PRReport{
			IncomingPRs: []PRSummary{
				{
					Repo:      "myrepo",
					Number:    42,
					Title:     "Fix the bug",
					URL:       "https://github.com/o/myrepo/pull/42",
					Action:    "needs review",
					IsBlocked: true,
				},
			},
			GeneratedAt: "2024-01-15 10:00 UTC",
		}
		embed := handler.formatReportEmbed(report)

		if len(embed.Fields) != 1 {
			t.Fatalf("Fields = %d, want 1", len(embed.Fields))
		}

		field := embed.Fields[0]
		if !strings.Contains(field.Name, "Incoming PRs") {
			t.Errorf("Field name = %q, want to contain 'Incoming PRs'", field.Name)
		}
		if !strings.Contains(field.Value, "🔴") {
			t.Errorf("Field value should contain blocked indicator 🔴")
		}
		if !strings.Contains(field.Value, "myrepo#42") {
			t.Errorf("Field value should contain PR reference")
		}
		if !strings.Contains(field.Value, "needs review") {
			t.Errorf("Field value should contain action")
		}
	})

	t.Run("with outgoing PRs", func(t *testing.T) {
		report := &PRReport{
			OutgoingPRs: []PRSummary{
				{
					Repo:      "myrepo",
					Number:    99,
					Title:     "New feature",
					URL:       "https://github.com/o/myrepo/pull/99",
					Action:    "waiting for review",
					IsBlocked: true,
				},
			},
			GeneratedAt: "2024-01-15 10:00 UTC",
		}
		embed := handler.formatReportEmbed(report)

		if len(embed.Fields) != 1 {
			t.Fatalf("Fields = %d, want 1", len(embed.Fields))
		}

		field := embed.Fields[0]
		if !strings.Contains(field.Name, "Your PRs") {
			t.Errorf("Field name = %q, want to contain 'Your PRs'", field.Name)
		}
		if !strings.Contains(field.Value, "🟢") {
			t.Errorf("Field value should contain blocked indicator 🟢 for outgoing")
		}
	})

	t.Run("with both sections", func(t *testing.T) {
		report := &PRReport{
			IncomingPRs: []PRSummary{
				{Repo: "repo1", Number: 1, Title: "PR1", URL: "url1"},
			},
			OutgoingPRs: []PRSummary{
				{Repo: "repo2", Number: 2, Title: "PR2", URL: "url2"},
			},
			GeneratedAt: "2024-01-15 10:00 UTC",
		}
		embed := handler.formatReportEmbed(report)

		if len(embed.Fields) != 2 {
			t.Fatalf("Fields = %d, want 2", len(embed.Fields))
		}
	})

	t.Run("long title gets truncated", func(t *testing.T) {
		longTitle := strings.Repeat("x", 100)
		report := &PRReport{
			IncomingPRs: []PRSummary{
				{Repo: "repo", Number: 1, Title: longTitle, URL: "url"},
			},
			GeneratedAt: "2024-01-15 10:00 UTC",
		}
		embed := handler.formatReportEmbed(report)

		// Title should be truncated to 40 chars + "..."
		if strings.Contains(embed.Fields[0].Value, longTitle) {
			t.Error("Long title should be truncated")
		}
		if !strings.Contains(embed.Fields[0].Value, "...") {
			t.Error("Truncated title should contain ellipsis")
		}
	})

	t.Run("footer shows generated time", func(t *testing.T) {
		report := &PRReport{
			IncomingPRs: []PRSummary{
				{Repo: "repo", Number: 1, Title: "Test", URL: "url"},
			},
			GeneratedAt: "2024-01-15 10:00 UTC",
		}
		embed := handler.formatReportEmbed(report)

		if embed.Footer == nil {
			t.Fatal("Footer should not be nil")
		}
		if !strings.Contains(embed.Footer.Text, "2024-01-15") {
			t.Errorf("Footer = %q, want to contain generated time", embed.Footer.Text)
		}
	})
}

func TestNewSlashCommandHandler(t *testing.T) {
	t.Run("with nil logger", func(t *testing.T) {
		handler := NewSlashCommandHandler(nil, nil)
		if handler.logger == nil {
			t.Error("logger should default to slog.Default()")
		}
		if handler.dashboardURL != "https://reviewgoose.dev" {
			t.Errorf("dashboardURL = %q, want default", handler.dashboardURL)
		}
	})
}

func TestSlashCommandHandler_SetDashboardURL(t *testing.T) {
	handler := NewSlashCommandHandler(nil, nil)

	customURL := "https://custom.example.com"
	handler.SetDashboardURL(customURL)

	if handler.dashboardURL != customURL {
		t.Errorf("dashboardURL = %q, want %q", handler.dashboardURL, customURL)
	}
}

func TestSlashCommandHandler_SetStatusGetter(t *testing.T) {
	handler := NewSlashCommandHandler(nil, nil)

	if handler.statusGetter != nil {
		t.Error("statusGetter should be nil initially")
	}

	// We can't easily test this without a mock, but we can verify the method exists
	handler.SetStatusGetter(nil)
}

func TestSlashCommandHandler_SetReportGetter(t *testing.T) {
	handler := NewSlashCommandHandler(nil, nil)

	if handler.reportGetter != nil {
		t.Error("reportGetter should be nil initially")
	}

	handler.SetReportGetter(nil)
}

func TestPRSummary_Fields(t *testing.T) {
	// Test that PRSummary has all expected fields
	pr := PRSummary{
		Repo:      "testrepo",
		Number:    123,
		Title:     "Test PR",
		Author:    "testuser",
		State:     "open",
		URL:       "https://github.com/o/r/pull/123",
		Action:    "review",
		UpdatedAt: "2024-01-15",
		IsBlocked: true,
	}

	if pr.Repo != "testrepo" {
		t.Errorf("Repo = %q, want 'testrepo'", pr.Repo)
	}
	if pr.Number != 123 {
		t.Errorf("Number = %d, want 123", pr.Number)
	}
	if !pr.IsBlocked {
		t.Error("IsBlocked should be true")
	}
}

func TestPRReport_Fields(t *testing.T) {
	report := PRReport{
		IncomingPRs: []PRSummary{{Repo: "r1"}},
		OutgoingPRs: []PRSummary{{Repo: "r2"}, {Repo: "r3"}},
		GeneratedAt: "2024-01-15",
	}

	if len(report.IncomingPRs) != 1 {
		t.Errorf("IncomingPRs len = %d, want 1", len(report.IncomingPRs))
	}
	if len(report.OutgoingPRs) != 2 {
		t.Errorf("OutgoingPRs len = %d, want 2", len(report.OutgoingPRs))
	}
}

func TestBotStatus_Fields(t *testing.T) {
	status := BotStatus{
		Connected:       true,
		ActivePRs:       10,
		PendingDMs:      5,
		ConnectedOrgs:   []string{"org1", "org2"},
		UptimeSeconds:   3600,
		LastEventTime:   "10 minutes ago",
		ConfiguredRepos: []string{"repo1"},
		WatchedChannels: []string{"channel1", "channel2"},
	}

	if !status.Connected {
		t.Error("Connected should be true")
	}
	if status.ActivePRs != 10 {
		t.Errorf("ActivePRs = %d, want 10", status.ActivePRs)
	}
	if len(status.ConnectedOrgs) != 2 {
		t.Errorf("ConnectedOrgs len = %d, want 2", len(status.ConnectedOrgs))
	}
}
