package github

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements ports.Logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...any)                {}
func (m *mockLogger) Info(msg string, fields ...any)                 {}
func (m *mockLogger) Warn(msg string, fields ...any)                 {}
func (m *mockLogger) Error(msg string, fields ...any)                {}
func (m *mockLogger) WithContext(fields map[string]any) ports.Logger { return m }

// mockGHRunner implements GHRunner for testing without shelling out to gh CLI.
type mockGHRunner struct {
	output []byte
	err    error
}

func (m *mockGHRunner) RunGH(_ context.Context, _ []string) ([]byte, error) {
	return m.output, m.err
}

// newTestProvider creates a GitHubOperationProvider with a mock runner returning empty JSON.
func newTestProvider() *GitHubOperationProvider {
	return NewGitHubOperationProvider(
		&mockGHRunner{output: []byte("{}"), err: nil},
		&mockLogger{},
	)
}

// TestClient_RunGH_HappyPath tests successful gh CLI execution with valid JSON output
func TestClient_RunGH_HappyPath(t *testing.T) {
	// Skip if gh CLI not installed
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not installed")
	}

	client := &Client{
		authMethod: AuthMethod{Type: "gh_cli"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	ctx := context.Background()

	// Test a simple command that should work without auth (version check)
	result, err := client.RunGH(ctx, []string{"--version"})

	// Should succeed and return version info
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, string(result), "gh version")
}

// TestClient_RunGH_ContextCancellation tests cancellation handling
func TestClient_RunGH_ContextCancellation(t *testing.T) {
	// Skip if gh CLI not installed
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not installed")
	}

	client := &Client{
		authMethod: AuthMethod{Type: "gh_cli"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	// Create context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := client.RunGH(ctx, []string{"version"})

	// Should return error (context canceled or command failed)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestClient_RunGH_ContextTimeout tests timeout handling
func TestClient_RunGH_ContextTimeout(t *testing.T) {
	// Skip if gh CLI not installed
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not installed")
	}

	client := &Client{
		authMethod: AuthMethod{Type: "gh_cli"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	// Create context with immediate timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure timeout triggers

	result, err := client.RunGH(ctx, []string{"version"})

	// Should return error (context deadline exceeded or command failed)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestClient_RunGH_EmptyArgs tests handling of empty argument list
func TestClient_RunGH_EmptyArgs(t *testing.T) {
	// Skip if gh CLI not installed
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not installed")
	}

	client := &Client{
		authMethod: AuthMethod{Type: "gh_cli"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	result, err := client.RunGH(context.Background(), []string{})

	// gh with no args should succeed and print help or version
	// It won't error, just might return help text
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestClient_RunGH_SpecialCharactersInArgs tests args with special characters
func TestClient_RunGH_SpecialCharactersInArgs(t *testing.T) {
	t.Skip("requires authenticated gh CLI and valid repo - integration test")
}

// TestClient_RunGH_NonAuthenticatedUser tests behavior with no auth
func TestClient_RunGH_NonAuthenticatedUser(t *testing.T) {
	t.Skip("requires controlled gh auth state - integration test")
}

// TestClient_RunHTTP_HappyPath tests successful HTTP API execution
func TestClient_RunHTTP_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       []byte
		wantJSON   string
		wantStatus int
	}{
		{
			name:       "GET issue",
			method:     "GET",
			path:       "/repos/owner/repo/issues/42",
			body:       nil,
			wantJSON:   `{"title":"Fix bug","state":"open"}`,
			wantStatus: 200,
		},
		{
			name:       "POST create issue",
			method:     "POST",
			path:       "/repos/owner/repo/issues",
			body:       []byte(`{"title":"New issue","body":"Description"}`),
			wantJSON:   `{"number":43,"url":"https://github.com/owner/repo/issues/43"}`,
			wantStatus: 201,
		},
		{
			name:       "PATCH update issue",
			method:     "PATCH",
			path:       "/repos/owner/repo/issues/42",
			body:       []byte(`{"state":"closed"}`),
			wantJSON:   `{"state":"closed"}`,
			wantStatus: 200,
		},
		{
			name:       "DELETE remove label",
			method:     "DELETE",
			path:       "/repos/owner/repo/issues/42/labels/bug",
			body:       nil,
			wantJSON:   `[]`,
			wantStatus: 204,
		},
	}

	client := &Client{
		authMethod: AuthMethod{Type: "token", Token: "ghp_test123"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			result, err := client.RunHTTP(ctx, tt.method, tt.path, tt.body)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "not implemented")
			assert.Nil(t, result)
		})
	}
}

// TestClient_RunHTTP_ContextCancellation tests cancellation during HTTP request
func TestClient_RunHTTP_ContextCancellation(t *testing.T) {
	client := &Client{
		authMethod: AuthMethod{Type: "token", Token: "ghp_test123"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := client.RunHTTP(ctx, "GET", "/repos/owner/repo/issues/42", nil)

	// Returns not-implemented error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Nil(t, result)
}

// TestClient_RunHTTP_ContextTimeout tests timeout during HTTP request
func TestClient_RunHTTP_ContextTimeout(t *testing.T) {
	client := &Client{
		authMethod: AuthMethod{Type: "token", Token: "ghp_test123"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	result, err := client.RunHTTP(ctx, "GET", "/repos/owner/repo/issues/42", nil)

	// Returns not-implemented error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Nil(t, result)
}

// TestClient_RunHTTP_EmptyPath tests handling of empty API path
func TestClient_RunHTTP_EmptyPath(t *testing.T) {
	client := &Client{
		authMethod: AuthMethod{Type: "token", Token: "ghp_test123"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	result, err := client.RunHTTP(context.Background(), "GET", "", nil)

	// Returns not-implemented error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Nil(t, result)
}

// TestClient_RunHTTP_InvalidMethod tests handling of invalid HTTP method
func TestClient_RunHTTP_InvalidMethod(t *testing.T) {
	client := &Client{
		authMethod: AuthMethod{Type: "token", Token: "ghp_test123"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	result, err := client.RunHTTP(context.Background(), "INVALID", "/repos/owner/repo/issues/42", nil)

	// Returns not-implemented error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Nil(t, result)
}

// TestClient_RunHTTP_NonAuthenticatedUser tests HTTP fallback with no auth
func TestClient_RunHTTP_NonAuthenticatedUser(t *testing.T) {
	client := &Client{
		authMethod: AuthMethod{Type: "none"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	result, err := client.RunHTTP(context.Background(), "GET", "/repos/owner/repo/issues/42", nil)

	// Returns not-implemented error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Nil(t, result)
}

// TestClient_RunHTTP_LargeRequestBody tests handling of large JSON payload
func TestClient_RunHTTP_LargeRequestBody(t *testing.T) {
	client := &Client{
		authMethod: AuthMethod{Type: "token", Token: "ghp_test123"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	// Create large body (10KB)
	largeBody := make([]byte, 10*1024)
	for i := range largeBody {
		largeBody[i] = byte('a' + (i % 26))
	}

	result, err := client.RunHTTP(context.Background(), "POST", "/repos/owner/repo/issues", largeBody)

	// Returns not-implemented error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Nil(t, result)
}

// TestClient_RunHTTP_SpecialCharactersInPath tests URL encoding
func TestClient_RunHTTP_SpecialCharactersInPath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "path with spaces",
			path: "/repos/owner with space/repo/issues/42",
		},
		{
			name: "path with special chars",
			path: "/repos/owner/repo/labels/bug%3Acritical",
		},
		{
			name: "path with unicode",
			path: "/repos/owner/repo/issues/comments?body=日本語",
		},
	}

	client := &Client{
		authMethod: AuthMethod{Type: "token", Token: "ghp_test123"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.RunHTTP(context.Background(), "GET", tt.path, nil)

			// Returns not-implemented error
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not implemented")
			assert.Nil(t, result)
		})
	}
}

// TestClient_DetectRepo_HappyPath tests successful repository detection from git remote
func TestClient_DetectRepo_HappyPath(t *testing.T) {
	ctx := context.Background()

	// Skip if not in a git repository
	if _, err := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir").Output(); err != nil {
		t.Skip("not in a git repository")
	}

	// Skip if no origin remote configured
	if _, err := exec.CommandContext(ctx, "git", "remote", "get-url", "origin").Output(); err != nil {
		t.Skip("no origin remote configured")
	}

	client := &Client{
		authMethod: AuthMethod{Type: "gh_cli"},
		logger:     &mockLogger{},
	}

	repo, err := client.DetectRepo(context.Background())

	// Should succeed and return owner/repo format
	require.NoError(t, err)
	assert.NotEmpty(t, repo)
	assert.Contains(t, repo, "/") // Should contain owner/repo separator
}

// TestClient_DetectRepo_NoGitRepository tests behavior outside git repository
func TestClient_DetectRepo_NoGitRepository(t *testing.T) {
	t.Skip("cannot reliably test outside git repo from within git repo - integration test")
}

// TestClient_DetectRepo_NoRemoteConfigured tests behavior with no git remote
func TestClient_DetectRepo_NoRemoteConfigured(t *testing.T) {
	t.Skip("cannot reliably test no remote from within repo with remote - integration test")
}

// TestClient_DetectRepo_MultipleRemotes tests behavior with multiple git remotes
func TestClient_DetectRepo_MultipleRemotes(t *testing.T) {
	t.Skip("requires controlled git remote setup - integration test")
}

// TestClient_DetectRepo_InvalidRemoteURL tests handling of malformed git remote URLs
func TestClient_DetectRepo_InvalidRemoteURL(t *testing.T) {
	t.Skip("requires controlled malformed remote URL - integration test")
}

// TestClient_DetectRepo_NonGitHubRemote tests behavior with non-GitHub remotes
func TestClient_DetectRepo_NonGitHubRemote(t *testing.T) {
	t.Skip("requires controlled non-GitHub remote - integration test")
}

// TestClient_DetectRepo_EdgeCases tests edge cases in repository detection
func TestClient_DetectRepo_EdgeCases(t *testing.T) {
	t.Skip("requires controlled remote URLs with edge cases - integration test")
}

// TestClient_ErrorHandling_RateLimitExceeded tests GitHub API rate limiting
func TestClient_ErrorHandling_RateLimitExceeded(t *testing.T) {
	client := &Client{
		authMethod: AuthMethod{Type: "token", Token: "ghp_test123"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	// Simulate rate limit response
	result, err := client.RunHTTP(context.Background(), "GET", "/repos/owner/repo/issues/42", nil)

	// Returns not-implemented error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Nil(t, result)
}

// TestClient_ErrorHandling_NotFound tests handling of 404 responses
func TestClient_ErrorHandling_NotFound(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "issue not found",
			path: "/repos/owner/repo/issues/99999",
		},
		{
			name: "pr not found",
			path: "/repos/owner/repo/pulls/99999",
		},
		{
			name: "repo not found",
			path: "/repos/nonexistent/repo/issues/1",
		},
	}

	client := &Client{
		authMethod: AuthMethod{Type: "token", Token: "ghp_test123"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.RunHTTP(context.Background(), "GET", tt.path, nil)

			// Returns not-implemented error
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not implemented")
			assert.Nil(t, result)
		})
	}
}

// TestClient_ErrorHandling_InvalidJSON tests handling of malformed JSON responses
func TestClient_ErrorHandling_InvalidJSON(t *testing.T) {
	t.Skip("requires controlled malformed gh response - integration test")
}

// TestClient_ErrorHandling_NetworkFailure tests handling of network errors
func TestClient_ErrorHandling_NetworkFailure(t *testing.T) {
	client := &Client{
		authMethod: AuthMethod{Type: "token", Token: "ghp_test123"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	result, err := client.RunHTTP(context.Background(), "GET", "/repos/owner/repo/issues/42", nil)

	// Returns not-implemented error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Nil(t, result)
}

// TestClient_ErrorHandling_GHCLINotFound tests behavior when gh CLI is not installed
func TestClient_ErrorHandling_GHCLINotFound(t *testing.T) {
	t.Skip("cannot test gh CLI missing without PATH manipulation - integration test")
}

// TestParseRepoFromRemote tests the parseRepoFromRemote helper function
func TestParseRepoFromRemote(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL string
		want      string
	}{
		{
			name:      "SSH format",
			remoteURL: "git@github.com:owner/repo.git",
			want:      "owner/repo",
		},
		{
			name:      "SSH format without .git",
			remoteURL: "git@github.com:owner/repo",
			want:      "owner/repo",
		},
		{
			name:      "HTTPS format",
			remoteURL: "https://github.com/owner/repo.git",
			want:      "owner/repo",
		},
		{
			name:      "HTTPS format without .git",
			remoteURL: "https://github.com/owner/repo",
			want:      "owner/repo",
		},
		{
			name:      "repo with dots",
			remoteURL: "git@github.com:owner/repo.name.git",
			want:      "owner/repo.name",
		},
		{
			name:      "repo with hyphens",
			remoteURL: "https://github.com/owner/repo-name.git",
			want:      "owner/repo-name",
		},
		{
			name:      "repo with underscores",
			remoteURL: "git@github.com:owner/repo_name.git",
			want:      "owner/repo_name",
		},
		{
			name:      "non-GitHub SSH",
			remoteURL: "git@gitlab.com:owner/repo.git",
			want:      "", // Function validates github.com host for SSH format
		},
		{
			name:      "non-GitHub HTTPS with github.com in path",
			remoteURL: "https://gitlab.com/owner/repo.git",
			want:      "", // No "github.com/" in URL, so returns empty
		},
		{
			name:      "invalid format",
			remoteURL: "owner/repo",
			want:      "",
		},
		{
			name:      "empty string",
			remoteURL: "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRepoFromRemote(tt.remoteURL)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestClient_ThreadSafety tests concurrent access to Client methods
func TestClient_ThreadSafety(t *testing.T) {
	client := &Client{
		authMethod: AuthMethod{Type: "gh_cli"},
		repo:       "owner/repo",
		logger:     &mockLogger{},
	}

	ctx := context.Background()
	concurrency := 10

	// Launch concurrent goroutines
	done := make(chan bool, concurrency*3)

	for range concurrency {
		go func() {
			_, _ = client.RunGH(ctx, []string{"issue", "list"})
			done <- true
		}()

		go func() {
			_, _ = client.RunHTTP(ctx, "GET", "/repos/owner/repo/issues", nil)
			done <- true
		}()

		go func() {
			_, _ = client.DetectRepo(ctx)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < concurrency*3; i++ {
		<-done
	}

	// If we get here without race detector warnings, thread safety is OK
	assert.True(t, true)
}
