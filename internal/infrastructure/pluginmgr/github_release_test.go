package pluginmgr_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/stretchr/testify/assert"
)

func TestValidateOwnerRepo(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid owner/repo",
			input:   "owner/repo",
			wantErr: false,
		},
		{
			name:    "valid with dashes",
			input:   "my-org/my-plugin",
			wantErr: false,
		},
		{
			name:    "valid with numbers",
			input:   "org123/plugin456",
			wantErr: false,
		},
		{
			name:    "valid complex",
			input:   "awf-project/awf-plugin-github",
			wantErr: false,
		},
		{
			name:    "no slash",
			input:   "ownerepo",
			wantErr: true,
			errMsg:  "missing slash separator",
		},
		{
			name:    "multiple slashes",
			input:   "owner/repo/extra",
			wantErr: true,
			errMsg:  "multiple slashes",
		},
		{
			name:    "empty owner",
			input:   "/repo",
			wantErr: true,
			errMsg:  "empty owner segment",
		},
		{
			name:    "empty repo",
			input:   "owner/",
			wantErr: true,
			errMsg:  "empty repo segment",
		},
		{
			name:    "https prefix",
			input:   "https://github.com/owner/repo",
			wantErr: true,
			errMsg:  "protocol",
		},
		{
			name:    "http prefix",
			input:   "http://github.com/owner/repo",
			wantErr: true,
			errMsg:  "protocol",
		},
		{
			name:    "exceeds 200 characters",
			input:   "owner/" + string(make([]byte, 200)),
			wantErr: true,
			errMsg:  "exceeds 200 characters",
		},
		{
			name:    "exactly 200 characters valid",
			input:   "a/" + string(make([]byte, 198)),
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "missing slash separator",
		},
		{
			name:    "only slash",
			input:   "/",
			wantErr: true,
			errMsg:  "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pluginmgr.ValidateOwnerRepo(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.ErrorContains(t, err, tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGitHubReleaseClient_ListReleases_HappyPath(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[{"tag_name":"v1.2.0","prerelease":false},{"tag_name":"v1.1.0","prerelease":false}]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	releases, err := client.ListReleases(testContext(t), "owner/repo")

	assert.NoError(t, err)
	assert.Len(t, releases, 2)
	assert.Equal(t, "v1.2.0", releases[0].TagName)
	assert.Equal(t, false, releases[0].Prerelease)
}

func TestGitHubReleaseClient_ListReleases_RateLimited(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 429,
		body:       `{"message":"API rate limit exceeded"}`,
		headers:    map[string]string{"X-RateLimit-Remaining": "0"},
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "owner/repo")

	assert.Error(t, err)
	assert.ErrorContains(t, err, "rate limit")
}

func TestGitHubReleaseClient_ListReleases_NetworkError(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		err: fmt.Errorf("connection refused"),
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "owner/repo")

	assert.Error(t, err)
}

func TestGitHubReleaseClient_ListReleases_InvalidJSON(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `not valid json`,
		headers:    map[string]string{},
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "owner/repo")

	assert.Error(t, err)
}

func TestGitHubReleaseClient_ResolveVersion_LatestStable(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body: `[
			{"tag_name":"v1.2.0","prerelease":false},
			{"tag_name":"v1.1.0","prerelease":false},
			{"tag_name":"v1.2.0-alpha","prerelease":true}
		]`,
		headers: map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	version, err := client.ResolveVersion(testContext(t), "owner/repo", "", false)

	assert.NoError(t, err)
	assert.Equal(t, "1.2.0", version.String())
	assert.False(t, version.IsPrerelease())
}

func TestGitHubReleaseClient_ResolveVersion_WithPrerelease(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body: `[
			{"tag_name":"v1.2.0-beta","prerelease":true},
			{"tag_name":"v1.1.0","prerelease":false}
		]`,
		headers: map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	version, err := client.ResolveVersion(testContext(t), "owner/repo", "", true)

	assert.NoError(t, err)
	assert.Equal(t, "1.2.0-beta", version.String())
	assert.True(t, version.IsPrerelease())
}

func TestGitHubReleaseClient_ResolveVersion_WithConstraint(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body: `[
			{"tag_name":"v2.0.0","prerelease":false},
			{"tag_name":"v1.5.0","prerelease":false},
			{"tag_name":"v1.0.0","prerelease":false}
		]`,
		headers: map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	version, err := client.ResolveVersion(testContext(t), "owner/repo", ">=1.0.0 <2.0.0", false)

	assert.NoError(t, err)
	assert.Equal(t, "1.5.0", version.String())
}

func TestGitHubReleaseClient_ResolveVersion_NoMatch(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[{"tag_name":"v1.0.0","prerelease":false}]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	_, err := client.ResolveVersion(testContext(t), "owner/repo", ">=2.0.0", false)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "no matching")
}

func TestGitHubReleaseClient_Auth_GitHubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token-123")

	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	_, _ = client.ListReleases(testContext(t), "owner/repo")

	assert.NotNil(t, mockDoer.lastRequest)
	authHeader := mockDoer.lastRequest.Header.Get("Authorization")
	assert.Equal(t, "Bearer test-token-123", authHeader)
}

func TestGitHubReleaseClient_Auth_Unauthenticated(t *testing.T) {
	// Force tier-3 (unauthenticated) by clearing GITHUB_TOKEN and removing gh from PATH.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("PATH", "")

	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "59"},
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	_, _ = client.ListReleases(testContext(t), "owner/repo")

	assert.NotNil(t, mockDoer.lastRequest)
	authHeader := mockDoer.lastRequest.Header.Get("Authorization")
	assert.Equal(t, "", authHeader)
}

func TestGitHubReleaseClient_RateLimit_HeaderDetection(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 200,
		body:       `[]`,
		headers:    map[string]string{"X-RateLimit-Remaining": "0"},
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "owner/repo")

	assert.Error(t, err)
	assert.ErrorContains(t, err, "rate limit")
}

func TestGitHubReleaseClient_RateLimit_HTTPStatus(t *testing.T) {
	mockDoer := &mockHTTPDoer{
		statusCode: 429,
		body:       `{"message":"API rate limit exceeded"}`,
		headers:    map[string]string{"Retry-After": "60"},
	}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "owner/repo")

	assert.Error(t, err)
	assert.ErrorContains(t, err, "rate limit")
}

func TestGitHubReleaseClient_InvalidOwnerRepo(t *testing.T) {
	mockDoer := &mockHTTPDoer{}

	client := pluginmgr.NewGitHubReleaseClient(mockDoer)
	_, err := client.ListReleases(testContext(t), "invalid-no-slash")

	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid owner/repo")
}

// Mock types for testing

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

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// FindPlatformAsset tests

func TestFindPlatformAsset_MatchesCurrentPlatform(t *testing.T) {
	assets := []pluginmgr.Asset{
		{Name: "awf-plugin-jira_1.0.0_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux-amd64.tar.gz"},
		{Name: "awf-plugin-jira_1.0.0_darwin_arm64.tar.gz", DownloadURL: "https://example.com/darwin-arm64.tar.gz"},
		{Name: "awf-plugin-jira_1.0.0_windows_amd64.tar.gz", DownloadURL: "https://example.com/windows-amd64.tar.gz"},
	}

	asset, err := pluginmgr.FindPlatformAsset(assets, "linux", "amd64")

	assert.NoError(t, err)
	assert.Equal(t, "awf-plugin-jira_1.0.0_linux_amd64.tar.gz", asset.Name)
	assert.Equal(t, "https://example.com/linux-amd64.tar.gz", asset.DownloadURL)
}

func TestFindPlatformAsset_NoMatchListsAvailable(t *testing.T) {
	assets := []pluginmgr.Asset{
		{Name: "awf-plugin-jira_1.0.0_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux-amd64.tar.gz"},
		{Name: "awf-plugin-jira_1.0.0_darwin_arm64.tar.gz", DownloadURL: "https://example.com/darwin-arm64.tar.gz"},
	}

	_, err := pluginmgr.FindPlatformAsset(assets, "freebsd", "amd64")

	assert.Error(t, err)
	assert.ErrorContains(t, err, "freebsd")
	assert.ErrorContains(t, err, "amd64")
	assert.ErrorContains(t, err, "linux_amd64")
	assert.ErrorContains(t, err, "darwin_arm64")
}

func TestFindPlatformAsset_EmptyAssetsReturnsError(t *testing.T) {
	_, err := pluginmgr.FindPlatformAsset([]pluginmgr.Asset{}, "linux", "amd64")

	assert.Error(t, err)
}

func TestFindPlatformAsset_IgnoresNonTarGzAssets(t *testing.T) {
	assets := []pluginmgr.Asset{
		{Name: "awf-plugin-jira_1.0.0_linux_amd64.zip", DownloadURL: "https://example.com/linux-amd64.zip"},
		{Name: "awf-plugin-jira_1.0.0_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux-amd64.tar.gz"},
	}

	asset, err := pluginmgr.FindPlatformAsset(assets, "linux", "amd64")

	assert.NoError(t, err)
	assert.Equal(t, "awf-plugin-jira_1.0.0_linux_amd64.tar.gz", asset.Name)
}

func TestFindPlatformAsset_DarwinArm64(t *testing.T) {
	assets := []pluginmgr.Asset{
		{Name: "awf-plugin-echo_2.1.0_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux.tar.gz"},
		{Name: "awf-plugin-echo_2.1.0_darwin_arm64.tar.gz", DownloadURL: "https://example.com/darwin.tar.gz"},
	}

	asset, err := pluginmgr.FindPlatformAsset(assets, "darwin", "arm64")

	assert.NoError(t, err)
	assert.Equal(t, "awf-plugin-echo_2.1.0_darwin_arm64.tar.gz", asset.Name)
}
