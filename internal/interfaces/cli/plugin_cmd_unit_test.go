package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectRelease_NoConstraint_ReturnsFirstNonPrerelease(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v1.0.0-alpha", Prerelease: true},
		{TagName: "v1.0.0", Prerelease: false},
		{TagName: "v1.1.0", Prerelease: false},
	}

	result, err := selectRelease(releases, "", false)

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.TagName)
}

func TestSelectRelease_WithConstraint_MatchesVersion(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v0.9.0", Prerelease: false},
		{TagName: "v1.0.0", Prerelease: false},
		{TagName: "v1.1.0", Prerelease: false},
	}

	result, err := selectRelease(releases, ">=1.0.0", false)

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.TagName)
}

func TestSelectRelease_IncludePrerelease_MatchesPrerelease(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v1.0.0-alpha", Prerelease: true},
		{TagName: "v1.1.0", Prerelease: false},
	}

	result, err := selectRelease(releases, "", true)

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0-alpha", result.TagName)
}

func TestSelectRelease_InvalidConstraint_ReturnsError(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v1.0.0", Prerelease: false},
	}

	_, err := selectRelease(releases, "invalid constraint", false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid version constraint")
}

func TestSelectRelease_NoMatch_ReturnsError(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v0.5.0", Prerelease: false},
	}

	_, err := selectRelease(releases, ">=1.0.0", false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no release matches constraint")
}

func TestFindChecksumURL_ChecksumsFileExists(t *testing.T) {
	assets := []registry.Asset{
		{Name: "plugin-linux", DownloadURL: "https://example.com/plugin-linux"},
		{Name: "checksums.txt", DownloadURL: "https://example.com/checksums.txt"},
		{Name: "plugin-darwin", DownloadURL: "https://example.com/plugin-darwin"},
	}

	url := findChecksumURL(assets)

	assert.Equal(t, "https://example.com/checksums.txt", url)
}

func TestFindChecksumURL_SHA256SUMSFileExists(t *testing.T) {
	assets := []registry.Asset{
		{Name: "plugin-linux", DownloadURL: "https://example.com/plugin-linux"},
		{Name: "SHA256SUMS", DownloadURL: "https://example.com/SHA256SUMS"},
	}

	url := findChecksumURL(assets)

	assert.Equal(t, "https://example.com/SHA256SUMS", url)
}

func TestFindChecksumURL_NoChecksumFile_ReturnsEmpty(t *testing.T) {
	assets := []registry.Asset{
		{Name: "plugin-linux", DownloadURL: "https://example.com/plugin-linux"},
		{Name: "plugin-darwin", DownloadURL: "https://example.com/plugin-darwin"},
	}

	url := findChecksumURL(assets)

	assert.Empty(t, url)
}

func TestFindChecksumURL_EmptyAssetList_ReturnsEmpty(t *testing.T) {
	assets := []registry.Asset{}

	url := findChecksumURL(assets)

	assert.Empty(t, url)
}

func TestExtractChecksumForAsset_ExactMatch(t *testing.T) {
	content := `abc123def456  plugin-linux
def789ghi012  plugin-darwin`

	checksum := extractChecksumForAsset(content, "plugin-linux")

	assert.Equal(t, "abc123def456", checksum)
}

func TestExtractChecksumForAsset_NotFound_ReturnsEmpty(t *testing.T) {
	content := `abc123def456  plugin-linux
def789ghi012  plugin-darwin`

	checksum := extractChecksumForAsset(content, "plugin-windows")

	assert.Empty(t, checksum)
}

func TestExtractChecksumForAsset_MalformedLine_Skipped(t *testing.T) {
	content := `malformed line without two parts
abc123def456  plugin-linux`

	checksum := extractChecksumForAsset(content, "plugin-linux")

	assert.Equal(t, "abc123def456", checksum)
}

func TestExtractPluginName_WithAWFPrefix(t *testing.T) {
	repo := "awf-plugin-github"

	name := extractPluginName(repo)

	assert.Equal(t, "github", name)
}

func TestExtractPluginName_WithoutAWFPrefix(t *testing.T) {
	repo := "custom-plugin-name"

	name := extractPluginName(repo)

	assert.Equal(t, "custom-plugin-name", name)
}

func newTestAPIBaseURLDoer() *apiBaseURLDoer {
	return newAPIBaseURLDoer("http://localhost:9999", &http.Client{Timeout: 1 * time.Second})
}

func TestAPIBaseURLDoer_RewritesGitHubURL(t *testing.T) {
	doer := newTestAPIBaseURLDoer()

	req, err := http.NewRequestWithContext(t.Context(), "GET", "https://api.github.com/repos/owner/repo", http.NoBody)
	require.NoError(t, err)

	_, err = doer.Do(req)

	assert.Error(t, err, "expected connection error when calling non-existent server")
}

func TestAPIBaseURLDoer_PassthroughNonGitHubURL(t *testing.T) {
	doer := newTestAPIBaseURLDoer()

	// Use a non-routable address to guarantee connection failure without hitting a real server.
	req, err := http.NewRequestWithContext(t.Context(), "GET", "http://192.0.2.1/some/path", http.NoBody)
	require.NoError(t, err)

	_, err = doer.Do(req)

	assert.Error(t, err, "expected connection error for non-GitHub URL")
}

func TestAPIBaseURLDoer_InvalidAPIBase_ReturnsError(t *testing.T) {
	doer := newAPIBaseURLDoer("://invalid", &http.Client{})

	req, err := http.NewRequestWithContext(t.Context(), "GET", "https://api.github.com/repos/owner/repo", http.NoBody)
	require.NoError(t, err)

	_, err = doer.Do(req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid GITHUB_API_URL")
}

func TestDownloadTextFile_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "abc123  file1\ndef456  file2")
	}))
	defer server.Close()

	content, err := downloadTextFile(context.Background(), server.URL)

	require.NoError(t, err)
	assert.Contains(t, content, "abc123")
	assert.Contains(t, content, "file1")
}

func TestDownloadTextFile_HTTPError_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := downloadTextFile(context.Background(), server.URL)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")
}

func TestDownloadTextFile_InvalidURL_ReturnsError(t *testing.T) {
	_, err := downloadTextFile(context.Background(), "not a valid url")

	assert.Error(t, err)
}

func TestDownloadTextFile_ContextCancelled_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := downloadTextFile(ctx, server.URL)

	assert.Error(t, err)
}
