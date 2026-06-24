package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginInstallCommand_UsesSyntaxEquivalentToInstallOwnerRepoAtVersion(t *testing.T) {
	cmd := newPluginInstallCommand(&Config{})

	assert.Equal(t, "install <owner/repo[@version]>", cmd.Use)
	assert.Nil(t, cmd.Flags().Lookup("version"))
	assert.NotNil(t, cmd.Flags().Lookup("pre-release"))
	assert.NotNil(t, cmd.Flags().Lookup("force"))
}

func TestParsePluginInstallSource(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantRepo    string
		wantVersion bool
		wantTag     string
		wantErr     string
	}{
		{
			name:     "unversioned",
			source:   "owner/repo",
			wantRepo: "owner/repo",
		},
		{
			name:        "bare semver",
			source:      "owner/repo@1.2.3",
			wantRepo:    "owner/repo",
			wantVersion: true,
			wantTag:     "1.2.3",
		},
		{
			name:        "v-prefixed semver",
			source:      "owner/repo@v1.2.3",
			wantRepo:    "owner/repo",
			wantVersion: true,
			wantTag:     "1.2.3",
		},
		{
			name:    "colon version syntax",
			source:  "owner/repo:1.2.3",
			wantErr: "owner/repo:version syntax is not supported; use owner/repo@version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePluginInstallSource(tt.source)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantRepo, got.Repository)
			assert.Equal(t, tt.wantVersion, got.Target.HasVersion)
			assert.Equal(t, tt.wantTag, got.Target.Tag)
		})
	}
}

func TestPluginInstallCommand_VersionFlagFailsDuringCobraFlagParsing(t *testing.T) {
	var releaseHits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		releaseHits++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := newPluginInstallCommand(&Config{StoragePath: t.TempDir()})
	cmd.SetArgs([]string{"owner/repo", "--version", "1.2.3"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag: --version")
	assert.Zero(t, releaseHits)
}

func TestPluginInstallCommand_RejectsInvalidVersionTargetsBeforeReleaseLookup(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "latest",
			source:  "owner/repo@latest",
			wantErr: `invalid release version "latest": version: invalid format "latest"`,
		},
		{
			name:    "range",
			source:  "owner/repo@>=1.0.0",
			wantErr: `invalid release version ">=1.0.0": version: invalid format ">=1.0.0"`,
		},
		{
			name:    "empty",
			source:  "owner/repo@",
			wantErr: `invalid release version "": version: empty string`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var releaseHits int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				releaseHits++
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			t.Setenv("GITHUB_API_URL", server.URL)

			cfg := &Config{StoragePath: t.TempDir()}
			cmd := newPluginInstallCommand(cfg)
			cmd.SetArgs([]string{tt.source})

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			err := cmd.Execute()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Zero(t, releaseHits)
		})
	}
}

func TestPluginInstallCommand_ExistingOwnerRepoValidationRemainsInForce(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "missing slash separator",
			source:  "owneronly",
			wantErr: "invalid owner/repo format: missing slash separator",
		},
		{
			name:    "empty owner segment",
			source:  "/repo",
			wantErr: "invalid owner/repo format: empty owner segment",
		},
		{
			name:    "multiple slashes",
			source:  "owner/repo/extra",
			wantErr: "invalid owner/repo format: multiple slashes not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var releaseHits int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				releaseHits++
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			t.Setenv("GITHUB_API_URL", server.URL)

			cmd := newPluginInstallCommand(&Config{StoragePath: t.TempDir()})
			cmd.SetArgs([]string{tt.source})

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			err := cmd.Execute()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Zero(t, releaseHits)
		})
	}
}

func TestPluginInstallCommand_ValidSemverAbsentFromReleaseListReturnsNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	var releaseHits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		releaseHits++
		releases := []map[string]any{
			{"tag_name": "v2.0.0"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases) //nolint:errcheck // test fixture response
	}))
	defer server.Close()
	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := newPluginInstallCommand(&Config{StoragePath: tmpDir})
	cmd.SetArgs([]string{"owner/repo@1.2.3"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve version for owner/repo: release version 1.2.3 not found")
	assert.Equal(t, 1, releaseHits)
}

func TestSelectRelease_NoConstraint_ReturnsFirstNonPrerelease(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v1.0.0-alpha", Prerelease: true},
		{TagName: "v1.0.0", Prerelease: false},
		{TagName: "v1.1.0", Prerelease: false},
	}

	result, err := selectRelease(releases, false)

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.TagName)
}

func TestSelectRelease_IncludePrerelease_MatchesPrerelease(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v1.0.0-alpha", Prerelease: true},
		{TagName: "v1.1.0", Prerelease: false},
	}

	result, err := selectRelease(releases, true)

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0-alpha", result.TagName)
}

func TestSelectRelease_NoEligibleRelease_ReturnsError(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v1.0.0-alpha", Prerelease: true},
	}

	_, err := selectRelease(releases, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no eligible releases found")
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

	checksum := registry.ExtractChecksumForAsset(content, "plugin-linux")

	assert.Equal(t, "abc123def456", checksum)
}

func TestExtractChecksumForAsset_NotFound_ReturnsEmpty(t *testing.T) {
	content := `abc123def456  plugin-linux
def789ghi012  plugin-darwin`

	checksum := registry.ExtractChecksumForAsset(content, "plugin-windows")

	assert.Empty(t, checksum)
}

func TestExtractChecksumForAsset_MalformedLine_Skipped(t *testing.T) {
	content := `malformed line without two parts
abc123def456  plugin-linux`

	checksum := registry.ExtractChecksumForAsset(content, "plugin-linux")

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

func TestAPIBaseURLDoer_RewritesGitHubURL(t *testing.T) {
	doer := registry.NewGitHubAPIDoer("http://localhost:9999", &http.Client{Timeout: 1 * time.Second})

	req, err := http.NewRequestWithContext(t.Context(), "GET", "https://api.github.com/repos/owner/repo", http.NoBody)
	require.NoError(t, err)

	_, err = doer.Do(req)

	assert.Error(t, err, "expected connection error when calling non-existent server")
}

func TestAPIBaseURLDoer_PassthroughNonGitHubURL(t *testing.T) {
	doer := registry.NewGitHubAPIDoer("http://localhost:9999", &http.Client{Timeout: 1 * time.Second})

	// Use a non-routable address to guarantee connection failure without hitting a real server.
	req, err := http.NewRequestWithContext(t.Context(), "GET", "http://192.0.2.1/some/path", http.NoBody)
	require.NoError(t, err)

	_, err = doer.Do(req)

	assert.Error(t, err, "expected connection error for non-GitHub URL")
}

func TestAPIBaseURLDoer_InvalidAPIBase_ReturnsError(t *testing.T) {
	doer := registry.NewGitHubAPIDoer("://invalid", &http.Client{})

	req, err := http.NewRequestWithContext(t.Context(), "GET", "https://api.github.com/repos/owner/repo", http.NoBody)
	require.NoError(t, err)

	_, err = doer.Do(req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid GITHUB_API_URL")
}
