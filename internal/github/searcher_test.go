package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v50/github"
)

// TestParseRepoFromIssue tests repository parsing from GitHub issue.
func TestParseRepoFromIssue(t *testing.T) {
	tests := []struct {
		name      string
		issue     *github.Issue
		wantOwner string
		wantRepo  string
	}{
		{
			name: "valid repository URL",
			issue: &github.Issue{
				RepositoryURL: github.String("https://api.github.com/repos/testowner/testrepo"),
			},
			wantOwner: "testowner",
			wantRepo:  "testrepo",
		},
		{
			name:      "nil repository URL",
			issue:     &github.Issue{},
			wantOwner: "",
			wantRepo:  "",
		},
		{
			name: "empty repository URL",
			issue: &github.Issue{
				RepositoryURL: github.String(""),
			},
			wantOwner: "",
			wantRepo:  "",
		},
		{
			name: "too short URL",
			issue: &github.Issue{
				RepositoryURL: github.String("https://api.github.com/repos/"),
			},
			wantOwner: "",
			wantRepo:  "",
		},
		{
			name: "no slash in path",
			issue: &github.Issue{
				RepositoryURL: github.String("https://api.github.com/repos/owneronly"),
			},
			wantOwner: "",
			wantRepo:  "",
		},
		{
			name: "owner with hyphen",
			issue: &github.Issue{
				RepositoryURL: github.String("https://api.github.com/repos/test-owner/test-repo"),
			},
			wantOwner: "test-owner",
			wantRepo:  "test-repo",
		},
		{
			name: "owner with underscore",
			issue: &github.Issue{
				RepositoryURL: github.String("https://api.github.com/repos/test_owner/test_repo"),
			},
			wantOwner: "test_owner",
			wantRepo:  "test_repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo := parseRepoFromIssue(tt.issue)
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}

// TestNewSearcher tests Searcher construction.
func TestNewSearcher(t *testing.T) {
	t.Run("with nil logger", func(t *testing.T) {
		appClient := &AppClient{}
		searcher := NewSearcher(appClient, nil)
		if searcher.logger == nil {
			t.Error("logger should default to slog.Default()")
		}
		if searcher.appClient != appClient {
			t.Error("appClient should be set")
		}
	})

	t.Run("with provided logger", func(t *testing.T) {
		appClient := &AppClient{}
		logger := slog.Default()
		searcher := NewSearcher(appClient, logger)
		if searcher.logger != logger {
			t.Error("logger should be the provided logger")
		}
	})
}

// TestSearchPRs tests the searchPRs function with a mock GitHub API server.
// Helper function to create a GitHub client pointing to a test server
func setupTestGitHubClient(t *testing.T, serverURL string) *github.Client {
	t.Helper()
	client := github.NewClient(nil)
	parsedURL, err := client.BaseURL.Parse(serverURL + "/")
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}
	client.BaseURL = parsedURL
	return client
}

// Helper function to encode and write JSON response
func writeJSONResponse(t *testing.T, w http.ResponseWriter, response any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Errorf("Failed to encode response: %v", err)
	}
}

//nolint:maintidx // Test complexity from multiple subtests with mock servers
func TestSearchPRs(t *testing.T) {
	ctx := context.Background()

	t.Run("successful search with results", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/search/issues" {
				t.Errorf("Expected path /search/issues, got %s", r.URL.Path)
			}

			response := &github.IssuesSearchResult{
				Total: github.Int(2),
				Issues: []*github.Issue{
					{
						Number:        github.Int(123),
						RepositoryURL: github.String("https://api.github.com/repos/testowner/testrepo"),
						UpdatedAt:     &github.Timestamp{Time: time.Now()},
						PullRequestLinks: &github.PullRequestLinks{
							HTMLURL: github.String("https://github.com/testowner/testrepo/pull/123"),
						},
					},
					{
						Number:        github.Int(456),
						RepositoryURL: github.String("https://api.github.com/repos/testowner/testrepo"),
						UpdatedAt:     &github.Timestamp{Time: time.Now()},
						PullRequestLinks: &github.PullRequestLinks{
							HTMLURL: github.String("https://github.com/testowner/testrepo/pull/456"),
						},
					},
				},
			}

			writeJSONResponse(t, w, response)
		}))
		defer server.Close()

		client := setupTestGitHubClient(t, server.URL)

		searcher := NewSearcher(&AppClient{}, nil)
		results, err := searcher.searchPRs(ctx, client, "test query")
		if err != nil {
			t.Errorf("searchPRs() error = %v, want nil", err)
		}

		if len(results) != 2 {
			t.Errorf("searchPRs() returned %d results, want 2", len(results))
		}

		if results[0].Number != 123 {
			t.Errorf("First result number = %d, want 123", results[0].Number)
		}

		if results[0].Owner != "testowner" {
			t.Errorf("First result owner = %s, want testowner", results[0].Owner)
		}

		if results[0].Repo != "testrepo" {
			t.Errorf("First result repo = %s, want testrepo", results[0].Repo)
		}
	})

	t.Run("search with no pull requests", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &github.IssuesSearchResult{
				Total: github.Int(1),
				Issues: []*github.Issue{
					{
						Number:           github.Int(789),
						RepositoryURL:    github.String("https://api.github.com/repos/testowner/testrepo"),
						PullRequestLinks: nil,
					},
				},
			}
			writeJSONResponse(t, w, response)
		}))
		defer server.Close()

		client := setupTestGitHubClient(t, server.URL)

		searcher := NewSearcher(&AppClient{}, nil)
		results, err := searcher.searchPRs(ctx, client, "test query")
		if err != nil {
			t.Errorf("searchPRs() error = %v, want nil", err)
		}

		if len(results) != 0 {
			t.Errorf("searchPRs() returned %d results, want 0 (no PRs)", len(results))
		}
	})

	t.Run("search with missing repository URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &github.IssuesSearchResult{
				Total: github.Int(1),
				Issues: []*github.Issue{
					{
						Number:           github.Int(123),
						RepositoryURL:    nil,
						PullRequestLinks: &github.PullRequestLinks{HTMLURL: github.String("url")},
					},
				},
			}
			writeJSONResponse(t, w, response)
		}))
		defer server.Close()

		client := setupTestGitHubClient(t, server.URL)

		searcher := NewSearcher(&AppClient{}, nil)
		results, err := searcher.searchPRs(ctx, client, "test query")
		if err != nil {
			t.Errorf("searchPRs() error = %v, want nil", err)
		}

		if len(results) != 0 {
			t.Errorf("searchPRs() returned %d results, want 0 (invalid repo URL)", len(results))
		}
	})

	t.Run("search with empty HTML URL constructs URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &github.IssuesSearchResult{
				Total: github.Int(1),
				Issues: []*github.Issue{
					{
						Number:        github.Int(123),
						RepositoryURL: github.String("https://api.github.com/repos/testowner/testrepo"),
						UpdatedAt:     &github.Timestamp{Time: time.Now()},
						PullRequestLinks: &github.PullRequestLinks{
							HTMLURL: nil,
						},
					},
				},
			}
			writeJSONResponse(t, w, response)
		}))
		defer server.Close()

		client := setupTestGitHubClient(t, server.URL)

		searcher := NewSearcher(&AppClient{}, nil)
		results, err := searcher.searchPRs(ctx, client, "test query")
		if err != nil {
			t.Errorf("searchPRs() error = %v, want nil", err)
		}

		if len(results) != 1 {
			t.Fatalf("searchPRs() returned %d results, want 1", len(results))
		}

		expectedURL := "https://github.com/testowner/testrepo/pull/123"
		if results[0].URL != expectedURL {
			t.Errorf("Result URL = %s, want %s", results[0].URL, expectedURL)
		}
	})

	t.Run("search with API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(`{"message": "Internal Server Error"}`)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		}))
		defer server.Close()

		client := setupTestGitHubClient(t, server.URL)

		searcher := NewSearcher(&AppClient{}, nil)
		_, err := searcher.searchPRs(ctx, client, "test query")

		if err == nil {
			t.Error("searchPRs() error = nil, want error")
		}
	})

	t.Run("search with pagination", func(t *testing.T) {
		pageCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pageCount++

			var response *github.IssuesSearchResult
			if pageCount == 1 {
				response = &github.IssuesSearchResult{
					Total: github.Int(3),
					Issues: []*github.Issue{
						{
							Number:        github.Int(1),
							RepositoryURL: github.String("https://api.github.com/repos/testowner/testrepo"),
							UpdatedAt:     &github.Timestamp{Time: time.Now()},
							PullRequestLinks: &github.PullRequestLinks{
								HTMLURL: github.String("https://github.com/testowner/testrepo/pull/1"),
							},
						},
					},
				}
				w.Header().Set("Link", `<`+r.URL.String()+`&page=2>; rel="next"`)
			} else {
				response = &github.IssuesSearchResult{
					Total: github.Int(3),
					Issues: []*github.Issue{
						{
							Number:        github.Int(2),
							RepositoryURL: github.String("https://api.github.com/repos/testowner/testrepo"),
							UpdatedAt:     &github.Timestamp{Time: time.Now()},
							PullRequestLinks: &github.PullRequestLinks{
								HTMLURL: github.String("https://github.com/testowner/testrepo/pull/2"),
							},
						},
					},
				}
			}

			writeJSONResponse(t, w, response)
		}))
		defer server.Close()

		client := setupTestGitHubClient(t, server.URL)

		searcher := NewSearcher(&AppClient{}, nil)
		results, err := searcher.searchPRs(ctx, client, "test query")
		if err != nil {
			t.Errorf("searchPRs() error = %v, want nil", err)
		}

		if len(results) != 2 {
			t.Errorf("searchPRs() returned %d results, want 2 (from 2 pages)", len(results))
		}
	})
}

// TestListOpenPRs tests listing open PRs
func TestListOpenPRs(t *testing.T) {
	ctx := context.Background()

	t.Run("successful search", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &github.IssuesSearchResult{
				Total: github.Int(1),
				Issues: []*github.Issue{
					NewMockPRIssue("testowner", "testrepo", 123, "Test PR"),
				},
			}
			writeJSONResponse(t, w, response)
		}))
		defer server.Close()

		client := setupTestGitHubClient(t, server.URL)
		mockAppClient := &MockAppClient{Client: client}
		searcher := NewSearcher(mockAppClient, nil)

		results, err := searcher.ListOpenPRs(ctx, "test-org", 24)
		if err != nil {
			t.Fatalf("ListOpenPRs() error = %v, want nil", err)
		}

		if len(results) != 1 {
			t.Errorf("ListOpenPRs() returned %d results, want 1", len(results))
		}
	})

	t.Run("client error", func(t *testing.T) {
		mockAppClient := &MockAppClient{
			ClientError: fmt.Errorf("no installation found"),
		}
		searcher := NewSearcher(mockAppClient, nil)

		_, err := searcher.ListOpenPRs(ctx, "test-org", 24)
		if err == nil {
			t.Error("ListOpenPRs() error = nil, want error")
		}
	})
}

// TestListClosedPRs tests listing closed PRs
func TestListClosedPRs(t *testing.T) {
	ctx := context.Background()

	t.Run("successful search", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &github.IssuesSearchResult{
				Total: github.Int(1),
				Issues: []*github.Issue{
					NewMockPRIssue("testowner", "testrepo", 456, "Closed PR"),
				},
			}
			writeJSONResponse(t, w, response)
		}))
		defer server.Close()

		client := setupTestGitHubClient(t, server.URL)
		mockAppClient := &MockAppClient{Client: client}
		searcher := NewSearcher(mockAppClient, nil)

		results, err := searcher.ListClosedPRs(ctx, "test-org", 24)
		if err != nil {
			t.Fatalf("ListClosedPRs() error = %v, want nil", err)
		}

		if len(results) != 1 {
			t.Errorf("ListClosedPRs() returned %d results, want 1", len(results))
		}
	})

	t.Run("client error", func(t *testing.T) {
		mockAppClient := &MockAppClient{
			ClientError: fmt.Errorf("no installation found"),
		}
		searcher := NewSearcher(mockAppClient, nil)

		_, err := searcher.ListClosedPRs(ctx, "test-org", 24)
		if err == nil {
			t.Error("ListClosedPRs() error = nil, want error")
		}
	})
}

// TestListAuthoredPRs tests listing authored PRs
func TestListAuthoredPRs(t *testing.T) {
	ctx := context.Background()

	t.Run("successful search", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &github.IssuesSearchResult{
				Total: github.Int(2),
				Issues: []*github.Issue{
					NewMockPRIssue("testowner", "testrepo", 111, "User's PR 1"),
					NewMockPRIssue("testowner", "testrepo", 222, "User's PR 2"),
				},
			}
			writeJSONResponse(t, w, response)
		}))
		defer server.Close()

		client := setupTestGitHubClient(t, server.URL)
		mockAppClient := &MockAppClient{Client: client}
		searcher := NewSearcher(mockAppClient, nil)

		results, err := searcher.ListAuthoredPRs(ctx, "test-org", "testuser")
		if err != nil {
			t.Fatalf("ListAuthoredPRs() error = %v, want nil", err)
		}

		if len(results) != 2 {
			t.Errorf("ListAuthoredPRs() returned %d results, want 2", len(results))
		}
	})

	t.Run("client error", func(t *testing.T) {
		mockAppClient := &MockAppClient{
			ClientError: fmt.Errorf("no installation found"),
		}
		searcher := NewSearcher(mockAppClient, nil)

		_, err := searcher.ListAuthoredPRs(ctx, "test-org", "testuser")
		if err == nil {
			t.Error("ListAuthoredPRs() error = nil, want error")
		}
	})
}

// TestListReviewRequestedPRs tests listing review-requested PRs
func TestListReviewRequestedPRs(t *testing.T) {
	ctx := context.Background()

	t.Run("successful search", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &github.IssuesSearchResult{
				Total: github.Int(1),
				Issues: []*github.Issue{
					NewMockPRIssue("testowner", "testrepo", 333, "Review Requested PR"),
				},
			}
			writeJSONResponse(t, w, response)
		}))
		defer server.Close()

		client := setupTestGitHubClient(t, server.URL)
		mockAppClient := &MockAppClient{Client: client}
		searcher := NewSearcher(mockAppClient, nil)

		results, err := searcher.ListReviewRequestedPRs(ctx, "test-org", "testuser")
		if err != nil {
			t.Fatalf("ListReviewRequestedPRs() error = %v, want nil", err)
		}

		if len(results) != 1 {
			t.Errorf("ListReviewRequestedPRs() returned %d results, want 1", len(results))
		}
	})

	t.Run("client error", func(t *testing.T) {
		mockAppClient := &MockAppClient{
			ClientError: fmt.Errorf("no installation found"),
		}
		searcher := NewSearcher(mockAppClient, nil)

		_, err := searcher.ListReviewRequestedPRs(ctx, "test-org", "testuser")
		if err == nil {
			t.Error("ListReviewRequestedPRs() error = nil, want error")
		}
	})
}
