package registry_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testContext creates a context with timeout for tests.
func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// mockHTTPDoer implements httpx.HTTPDoer for testing.
type mockHTTPDoer struct {
	statusCode  int
	body        string
	headers     map[string]string
	err         error
	lastRequest *http.Request
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.lastRequest = req

	header := make(http.Header)
	for k, v := range m.headers {
		header.Set(k, v)
	}

	return &http.Response{
		StatusCode: m.statusCode,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

// ============================================================================
// ValidateOwnerRepo Tests
// ============================================================================

func TestValidateOwnerRepo_ValidFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple owner/repo", "owner/repo"},
		{"with dashes", "my-org/my-plugin"},
		{"with numbers", "org123/plugin456"},
		{"complex", "awf-project/awf-plugin-github"},
		{"single letter", "a/b"},
		{"exactly 200 chars", "a/" + strings.Repeat("x", 198)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.ValidateOwnerRepo(tt.input)
			assert.NoError(t, err)
		})
	}
}

func TestValidateOwnerRepo_InvalidFormats(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"no slash", "ownerepo", "missing slash"},
		{"multiple slashes", "owner/repo/extra", "multiple slashes"},
		{"empty owner", "/repo", "empty owner"},
		{"empty repo", "owner/", "empty repo"},
		{"https prefix", "https://github.com/owner/repo", "protocol"},
		{"http prefix", "http://example.com/owner/repo", "protocol"},
		{"exceeds 200 chars", "owner/" + strings.Repeat("x", 200), "exceeds 200"},
		{"empty string", "", "missing slash"},
		{"only slash", "/", "empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.ValidateOwnerRepo(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// ============================================================================
// NewGitHubReleaseClient Tests
// ============================================================================

func TestNewGitHubReleaseClient_WithValidDoer(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	require.NotNil(t, client)

	// Verify client can make requests
	_, err := client.ListReleases(testContext(t), "owner/repo")
	assert.NoError(t, err)
}

func TestNewGitHubReleaseClient_WithNilDoer(t *testing.T) {
	client := registry.NewGitHubReleaseClient(nil)
	require.NotNil(t, client, "client should be created even with nil doer")
}

// ============================================================================
// ListReleases Tests
// ============================================================================

func TestListReleases_HappyPath(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[{"tag_name":"v1.2.0","prerelease":false},{"tag_name":"v1.1.0","prerelease":false}]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	releases, err := client.ListReleases(testContext(t), "owner/repo")

	require.NoError(t, err)
	require.Len(t, releases, 2)
	assert.Equal(t, "v1.2.0", releases[0].TagName)
	assert.False(t, releases[0].Prerelease)
	assert.Equal(t, "v1.1.0", releases[1].TagName)
}

func TestListReleases_WithAssets(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body: `[{
			"tag_name":"v1.0.0",
			"prerelease":false,
			"assets":[
				{"name":"plugin_1.0.0_linux_amd64.tar.gz","browser_download_url":"https://example.com/linux.tar.gz","size":1024}
			]
		}]`,
		headers: map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	releases, err := client.ListReleases(testContext(t), "owner/repo")

	require.NoError(t, err)
	require.Len(t, releases, 1)
	require.Len(t, releases[0].Assets, 1)
	assert.Equal(t, "plugin_1.0.0_linux_amd64.tar.gz", releases[0].Assets[0].Name)
	assert.Equal(t, "https://example.com/linux.tar.gz", releases[0].Assets[0].DownloadURL)
	assert.Equal(t, 1024, releases[0].Assets[0].Size)
}

func TestListReleases_RateLimitedStatus429(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 429,
		body:       `{"message":"API rate limit exceeded"}`,
		headers:    map[string]string{"X-RateLimit-Remaining": "0"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "owner/repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestListReleases_RateLimitedHeader(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "0"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "owner/repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestListReleases_NetworkError(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		err: fmt.Errorf("connection refused"),
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "owner/repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list releases")
}

func TestListReleases_InvalidJSON(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `not valid json`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "owner/repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestListReleases_HTTPErrorStatus(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 500,
		body:       `{"message":"Internal Server Error"}`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "owner/repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub API returned status")
}

func TestListReleases_InvalidOwnerRepo(t *testing.T) {
	mockDoer := &mockHTTPDoer{}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "invalid-no-slash")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid owner/repo")
}

// ============================================================================
// ResolveVersion Tests
// ============================================================================

func TestResolveVersion_LatestStable(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body: `[
			{"tag_name":"v1.2.0","prerelease":false},
			{"tag_name":"v1.1.0","prerelease":false},
			{"tag_name":"v1.2.0-alpha","prerelease":true}
		]`,
		headers: map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	version, err := client.ResolveVersion(testContext(t), "owner/repo", "", false)

	require.NoError(t, err)
	assert.Equal(t, "1.2.0", version.String())
	assert.False(t, version.IsPrerelease())
}

func TestResolveVersion_WithPrerelease(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body: `[
			{"tag_name":"v1.2.0-beta","prerelease":true},
			{"tag_name":"v1.1.0","prerelease":false}
		]`,
		headers: map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	version, err := client.ResolveVersion(testContext(t), "owner/repo", "", true)

	require.NoError(t, err)
	assert.Equal(t, "1.2.0-beta", version.String())
	assert.True(t, version.IsPrerelease())
}

func TestResolveVersion_WithConstraint(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body: `[
			{"tag_name":"v2.0.0","prerelease":false},
			{"tag_name":"v1.5.0","prerelease":false},
			{"tag_name":"v1.0.0","prerelease":false}
		]`,
		headers: map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	version, err := client.ResolveVersion(testContext(t), "owner/repo", ">=1.0.0 <2.0.0", false)

	require.NoError(t, err)
	assert.Equal(t, "1.5.0", version.String())
}

func TestResolveVersion_NoMatchingVersions(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[{"tag_name":"v1.0.0","prerelease":false}]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, err := client.ResolveVersion(testContext(t), "owner/repo", ">=2.0.0", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching")
}

func TestResolveVersion_InvalidConstraint(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[{"tag_name":"v1.0.0","prerelease":false}]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, err := client.ResolveVersion(testContext(t), "owner/repo", "invalid_constraint", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid constraint")
}

func TestResolveVersion_NoReleases(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, err := client.ResolveVersion(testContext(t), "owner/repo", "", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching")
}

func TestResolveVersion_SkipsInvalidTags(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body: `[
			{"tag_name":"invalid-tag","prerelease":false},
			{"tag_name":"v1.0.0","prerelease":false},
			{"tag_name":"also-bad","prerelease":false}
		]`,
		headers: map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	version, err := client.ResolveVersion(testContext(t), "owner/repo", "", false)

	require.NoError(t, err)
	assert.Equal(t, "1.0.0", version.String())
}

func TestResolveVersion_InvalidOwnerRepo(t *testing.T) {
	mockDoer := &mockHTTPDoer{}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, err := client.ResolveVersion(testContext(t), "invalid", "", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid owner/repo")
}

// ============================================================================
// FindPlatformAsset Tests
// ============================================================================

func TestFindPlatformAsset_Matches(t *testing.T) {
	assets := []registry.Asset{
		{Name: "awf-plugin-jira_1.0.0_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux-amd64.tar.gz"},
		{Name: "awf-plugin-jira_1.0.0_darwin_arm64.tar.gz", DownloadURL: "https://example.com/darwin-arm64.tar.gz"},
		{Name: "awf-plugin-jira_1.0.0_windows_amd64.tar.gz", DownloadURL: "https://example.com/windows-amd64.tar.gz"},
	}

	asset, err := registry.FindPlatformAsset(assets, "linux", "amd64")

	require.NoError(t, err)
	assert.Equal(t, "awf-plugin-jira_1.0.0_linux_amd64.tar.gz", asset.Name)
	assert.Equal(t, "https://example.com/linux-amd64.tar.gz", asset.DownloadURL)
}

func TestFindPlatformAsset_NoMatchListsAvailable(t *testing.T) {
	assets := []registry.Asset{
		{Name: "awf-plugin-jira_1.0.0_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux-amd64.tar.gz"},
		{Name: "awf-plugin-jira_1.0.0_darwin_arm64.tar.gz", DownloadURL: "https://example.com/darwin-arm64.tar.gz"},
	}

	_, err := registry.FindPlatformAsset(assets, "freebsd", "amd64")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "freebsd")
	assert.Contains(t, err.Error(), "amd64")
	assert.Contains(t, err.Error(), "linux_amd64")
	assert.Contains(t, err.Error(), "darwin_arm64")
}

func TestFindPlatformAsset_EmptyAssets(t *testing.T) {
	_, err := registry.FindPlatformAsset([]registry.Asset{}, "linux", "amd64")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching")
}

func TestFindPlatformAsset_IgnoresNonTarGz(t *testing.T) {
	assets := []registry.Asset{
		{Name: "awf-plugin-jira_1.0.0_linux_amd64.zip", DownloadURL: "https://example.com/linux-amd64.zip"},
		{Name: "awf-plugin-jira_1.0.0_linux_amd64.tar", DownloadURL: "https://example.com/linux-amd64.tar"},
		{Name: "awf-plugin-jira_1.0.0_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux-amd64.tar.gz"},
	}

	asset, err := registry.FindPlatformAsset(assets, "linux", "amd64")

	require.NoError(t, err)
	assert.Equal(t, "awf-plugin-jira_1.0.0_linux_amd64.tar.gz", asset.Name)
}

func TestFindPlatformAsset_DarwinArm64(t *testing.T) {
	assets := []registry.Asset{
		{Name: "awf-plugin-echo_2.1.0_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux.tar.gz"},
		{Name: "awf-plugin-echo_2.1.0_darwin_arm64.tar.gz", DownloadURL: "https://example.com/darwin.tar.gz"},
	}

	asset, err := registry.FindPlatformAsset(assets, "darwin", "arm64")

	require.NoError(t, err)
	assert.Equal(t, "awf-plugin-echo_2.1.0_darwin_arm64.tar.gz", asset.Name)
}

func TestFindPlatformAsset_WindowsAmd64(t *testing.T) {
	assets := []registry.Asset{
		{Name: "tool_1.0.0_windows_amd64.tar.gz", DownloadURL: "https://example.com/windows.tar.gz"},
	}

	asset, err := registry.FindPlatformAsset(assets, "windows", "amd64")

	require.NoError(t, err)
	assert.Equal(t, "tool_1.0.0_windows_amd64.tar.gz", asset.Name)
}

// ============================================================================
// Authentication Tests
// ============================================================================

func TestListReleases_AuthWithGitHubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token-123")

	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, _ = client.ListReleases(testContext(t), "owner/repo")

	require.NotNil(t, mockDoer.lastRequest)
	authHeader := mockDoer.lastRequest.Header.Get("Authorization")
	assert.Equal(t, "Bearer test-token-123", authHeader)
}

func TestListReleases_AuthUnauthenticatedNoGHCLI(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("PATH", "") // Remove gh CLI from PATH

	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, _ = client.ListReleases(testContext(t), "owner/repo")

	require.NotNil(t, mockDoer.lastRequest)
	authHeader := mockDoer.lastRequest.Header.Get("Authorization")
	assert.Equal(t, "", authHeader)
}

func TestListReleases_AuthWithGHCLI(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake gh binary that outputs a token on stdout.
	ghScript := filepath.Join(tmpDir, "gh")
	err := os.WriteFile(ghScript, []byte("#!/bin/sh\necho test-gh-token"), 0o755)
	require.NoError(t, err)

	t.Setenv("PATH", tmpDir+":"+os.Getenv("PATH"))
	t.Setenv("GITHUB_TOKEN", "") // force fallback to gh CLI

	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, _ = client.ListReleases(testContext(t), "owner/repo")

	require.NotNil(t, mockDoer.lastRequest)
	authHeader := mockDoer.lastRequest.Header.Get("Authorization")
	assert.Equal(t, "Bearer test-gh-token", authHeader, "should use token from gh auth token")
}

func TestListReleases_AcceptHeader(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := registry.NewGitHubReleaseClient(mockDoer)
	_, _ = client.ListReleases(testContext(t), "owner/repo")

	require.NotNil(t, mockDoer.lastRequest)
	acceptHeader := mockDoer.lastRequest.Header.Get("Accept")
	assert.Equal(t, "application/vnd.github+json", acceptHeader)
}
