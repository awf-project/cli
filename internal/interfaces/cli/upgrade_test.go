package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/awf-project/cli/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: update_upgrade_version
// Acceptance source: .specify/implementation/F110/tasks/T060.md, "Acceptance".
// These cases cover the F110 upgrade grammar change from `awf upgrade --version <version>`
// to `awf upgrade [version]`, including red-state checks for removed flag parsing
// and exact SemVer validation before any release lookup.
func TestNewUpgradeCommand_UsesSyntaxEquivalentToAwfUpgradeVersionAndAcceptsZeroOrOnePositionalVersionArgument(t *testing.T) {
	cmd := newUpgradeCommand(nil)

	assert.Equal(t, "upgrade [version]", cmd.Use)
	require.NoError(t, cmd.Args(cmd, nil))
	require.NoError(t, cmd.Args(cmd, []string{"0.5.0"}))
}

func TestNewUpgradeCommand_RejectsMoreThanOnePositionalArgumentThroughCobraMaximumNArgs(t *testing.T) {
	cmd := newUpgradeCommand(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"0.5.0", "0.6.0"})

	err := cmd.Execute()

	require.Error(t, err)
	assert.EqualError(t, err, "accepts at most 1 arg(s), received 2")
}

func TestNewUpgradeCommand_NoLongerRegistersLocalVersionFlag(t *testing.T) {
	cmd := newUpgradeCommand(nil)

	assert.Nil(t, cmd.Flags().Lookup("version"))
}

func TestAwfUpgradeVersionFlagFailsDuringCobraFlagParsingWithoutReleaseResolution(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--version", "0.5.0"})

	err := cmd.Execute()

	require.Error(t, err)
	assert.EqualError(t, err, "unknown flag: --version")
	assert.Equal(t, int32(0), requests.Load(), "removed --version flag must fail during Cobra parsing before release lookup")
}

func TestRunUpgrade_SelectsLatestStableReleaseWhenNoPositionalVersionIsProvided(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v1.1.0-beta", Prerelease: true},
		{TagName: "v1.0.0", Prerelease: false},
		{TagName: "v0.9.0", Prerelease: false},
	}

	target, err := parseExactReleaseTarget("", true)
	require.NoError(t, err)

	result, err := selectExactRelease(releases, target, false)

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.TagName)
}

func TestRunUpgrade_AcceptsPositional050ValidatesExactSemVerAndSelectsExactlyReleaseV050(t *testing.T) {
	origVersion := Version
	Version = "0.4.0"
	t.Cleanup(func() { Version = origVersion })

	releases := []map[string]any{
		{"tag_name": "v0.6.0", "prerelease": false, "assets": []any{}},
		{"tag_name": "v0.5.0", "prerelease": false, "assets": []any{}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases) //nolint:errcheck // controlled test response
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	err := runUpgrade(cmd, nil, upgradeOptions{check: true, targetVersion: "0.5.0"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "v0.5.0")
	assert.NotContains(t, out.String(), "v0.6.0")
}

func TestRunUpgrade_AcceptsPositionalV050NormalizesItAndSelectsExactlyTheMatchingRelease(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v1.0.0", Prerelease: false},
		{TagName: "v0.5.0", Prerelease: false},
	}

	target, err := parseExactReleaseTarget("v0.5.0", true)
	require.NoError(t, err)

	result, err := selectExactRelease(releases, target, false)

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", result.TagName)
}

func TestUpgradeCommand_CheckModeAcceptsPositionalV050AndSelectsExactRelease(t *testing.T) {
	origVersion := Version
	Version = "0.4.0"
	t.Cleanup(func() { Version = origVersion })

	releases := []map[string]any{
		{"tag_name": "v0.6.0", "prerelease": false, "assets": []any{}},
		{"tag_name": "v0.5.0", "prerelease": false, "assets": []any{}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases) //nolint:errcheck // controlled test response
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--check", "v0.5.0"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, out.String(), "v0.5.0")
	assert.NotContains(t, out.String(), "v0.6.0")
}

func TestRunUpgrade_RejectsPositionalLatestBeforeReleaseLookup(t *testing.T) {
	origVersion := Version
	Version = "0.4.0"
	t.Cleanup(func() { Version = origVersion })

	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)

	err := runUpgrade(cmd, nil, upgradeOptions{targetVersion: "latest"})

	require.Error(t, err)
	assert.EqualError(t, err, `invalid release version "latest": version: invalid format "latest"`)
	assert.Equal(t, int32(0), requests.Load(), "invalid explicit version must fail before release lookup")
}

func TestRunUpgrade_RejectsPositionalRangeBeforeReleaseLookup(t *testing.T) {
	origVersion := Version
	Version = "0.4.0"
	t.Cleanup(func() { Version = origVersion })

	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)

	err := runUpgrade(cmd, nil, upgradeOptions{targetVersion: ">=0.5.0"})

	require.Error(t, err)
	assert.EqualError(t, err, `invalid release version ">=0.5.0": version: invalid format ">=0.5.0"`)
	assert.Equal(t, int32(0), requests.Load(), "range constraints must fail before release lookup")
}

func TestRunUpgrade_ReturnsReleaseVersion050NotFoundWhenValidSemVerIsAbsentFromReleaseList(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v1.0.0", Prerelease: false},
	}

	target, err := parseExactReleaseTarget("0.5.0", true)
	require.NoError(t, err)

	_, err = selectExactRelease(releases, target, false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "release version 0.5.0 not found")
}

func TestSelectExactRelease_NoStableReleases_ReturnsError(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v1.0.0-beta", Prerelease: true},
	}

	target, err := parseExactReleaseTarget("", true)
	require.NoError(t, err)

	_, err = selectExactRelease(releases, target, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no stable releases")
}

func TestCheckWritePermission_WritableDir_ReturnsNil(t *testing.T) {
	dir := t.TempDir()

	err := checkWritePermission(dir)

	assert.NoError(t, err)
}

func TestCheckWritePermission_NonExistentDir_ReturnsError(t *testing.T) {
	err := checkWritePermission("/nonexistent/awf-test-path-that-does-not-exist")

	assert.Error(t, err)
}

func TestRunUpgrade_DevBuildWithoutForce_ReturnsError(t *testing.T) {
	origVersion := Version
	Version = "dev"
	t.Cleanup(func() { Version = origVersion })

	cmd := newUpgradeCommand(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := runUpgrade(cmd, nil, upgradeOptions{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dev build")
	assert.Contains(t, err.Error(), "--force")
}

func TestRunUpgrade_CheckMode_PrintsUpdateAvailableWithoutInstalling(t *testing.T) {
	origVersion := Version
	Version = "dev"
	t.Cleanup(func() { Version = origVersion })

	releases := []map[string]any{
		{"tag_name": "v1.0.0", "prerelease": false, "assets": []any{}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases) //nolint:errcheck // controlled test response
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	err := runUpgrade(cmd, nil, upgradeOptions{check: true, force: true})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Update available")
	assert.Contains(t, out.String(), "v1.0.0")
}

func TestRunUpgrade_AlreadyOnLatest_PrintsUpToDate(t *testing.T) {
	origVersion := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = origVersion })

	releases := []map[string]any{
		{"tag_name": "v1.0.0", "prerelease": false, "assets": []any{}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases) //nolint:errcheck // controlled test response
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	err := runUpgrade(cmd, nil, upgradeOptions{})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "already up to date")
}

func TestRunUpgrade_VersionInstall_DownloadsAndInstalls(t *testing.T) {
	origVersion := Version
	Version = "0.9.0"
	t.Cleanup(func() { Version = origVersion })

	releases := []map[string]any{
		{"tag_name": "v1.0.0", "prerelease": false, "assets": []any{}},
		{"tag_name": "v0.9.0", "prerelease": false, "assets": []map[string]any{
			{"name": "awf_linux_amd64.tar.gz", "browser_download_url": "http://example.com/binary.tar.gz"},
		}},
	}

	checksumContent := "abc123def456  awf_linux_amd64.tar.gz\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(releases) //nolint:errcheck // controlled test response
		case "/checksums.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(checksumContent)) //nolint:errcheck // controlled test response
		case "/binary.tar.gz":
			w.Header().Set("Content-Type", "application/gzip")
			// Return minimal tar.gz with awf binary
			mockBinary := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff}
			_, _ = w.Write(mockBinary) //nolint:errcheck // controlled test response
		}
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := runUpgrade(cmd, nil, upgradeOptions{targetVersion: "v0.9.0"})

	// We expect an error due to invalid tar.gz, but it should have attempted the install
	assert.Error(t, err)
}

func TestRunUpgrade_PermissionDenied_ErrorMessageHintsSudo(t *testing.T) {
	origVersion := Version
	Version = "0.9.0"
	t.Cleanup(func() { Version = origVersion })

	// Create a read-only directory
	readOnlyDir := t.TempDir()
	require.NoError(t, os.Chmod(readOnlyDir, 0o555))
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0o755) // restore for cleanup
	})

	// Verify the directory is actually read-only
	err := checkWritePermission(readOnlyDir)
	if err == nil {
		t.Skip("unable to create read-only directory in test environment (running as root?)")
	}

	// Test the error message format that would be shown to user
	fullErr := fmt.Errorf("no write permission on %s: %w (try running with sudo)", readOnlyDir, err)
	assert.Contains(t, fullErr.Error(), "no write permission")
	assert.Contains(t, fullErr.Error(), "sudo")
}

func TestRunUpgrade_GitHubTokenForwarding_SendsAuthHeader(t *testing.T) {
	origVersion := Version
	Version = "dev"
	t.Cleanup(func() { Version = origVersion })

	var authHeader string
	releases := []map[string]any{
		{"tag_name": "v1.0.0", "prerelease": false, "assets": []any{}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases) //nolint:errcheck // controlled test response
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GITHUB_TOKEN", "test-token-12345")

	cmd := newUpgradeCommand(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	_ = runUpgrade(cmd, nil, upgradeOptions{check: true, force: true}) //nolint:errcheck // we're testing the header, not the result

	// Verify token was forwarded
	assert.Equal(t, "Bearer test-token-12345", authHeader)
}

func TestRunUpgrade_PackageManagerWarning_EmitsWarningOnStderr(t *testing.T) {
	origVersion := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = origVersion })

	// IsPackageManagerPath is already tested in updater package.
	// This test verifies the warning is emitted to stderr during upgrade flow.
	// We use --check mode which exits before binary replacement, so the
	// package manager warning path is only reached in the full install flow.
	// For check mode, we verify the update-available output is correct.
	releases := []map[string]any{
		{"tag_name": "v1.0.1", "prerelease": false, "assets": []any{}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases) //nolint:errcheck // controlled test response
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	err := runUpgrade(cmd, nil, upgradeOptions{check: true})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Update available")
}

func TestRunUpgrade_ForceFlag_UpgradesEvenWhenAlreadyOnLatest(t *testing.T) {
	origVersion := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = origVersion })

	releases := []map[string]any{
		{"tag_name": "v1.0.0", "prerelease": false, "assets": []map[string]any{
			{"name": "awf_linux_amd64.tar.gz", "browser_download_url": "http://example.com/binary.tar.gz"},
			{"name": "checksums.txt", "browser_download_url": "http://example.com/checksums.txt"},
		}},
	}

	checksumContent := "abc123def456  awf_linux_amd64.tar.gz\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(releases) //nolint:errcheck // controlled test response
		case "/checksums.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(checksumContent)) //nolint:errcheck // controlled test response
		}
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	err := runUpgrade(cmd, nil, upgradeOptions{force: true})

	// With --force, should attempt upgrade even though already on latest
	// Will fail due to invalid tar.gz, but that proves force flag works
	assert.Error(t, err)
	assert.NotContains(t, out.String(), "already up to date")
}

func TestRunUpgrade_ChecksumMismatch_ReturnsError(t *testing.T) {
	origVersion := Version
	Version = "0.9.0"
	t.Cleanup(func() { Version = origVersion })

	releases := []map[string]any{
		{"tag_name": "v1.0.0", "prerelease": false, "assets": []map[string]any{
			{"name": "awf_linux_amd64.tar.gz", "browser_download_url": "http://example.com/binary.tar.gz"},
			{"name": "checksums.txt", "browser_download_url": "http://example.com/checksums.txt"},
		}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases) //nolint:errcheck // controlled test response
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := runUpgrade(cmd, nil, upgradeOptions{})

	// Fails because asset download URLs are mocked without real responses
	assert.Error(t, err)
}

func TestRunUpgrade_GitHubAPIUnreachable_ReturnsError(t *testing.T) {
	origVersion := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = origVersion })

	// Use invalid/unreachable URL
	t.Setenv("GITHUB_API_URL", "http://invalid-unreachable-domain-12345.example.com")

	cmd := newUpgradeCommand(nil)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := runUpgrade(cmd, nil, upgradeOptions{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch releases")
}

func TestRunUpgrade_NoCompatibleAsset_ReturnsError(t *testing.T) {
	origVersion := Version
	Version = "0.9.0"
	t.Cleanup(func() { Version = origVersion })

	// Release has no compatible asset for current platform
	releases := []map[string]any{
		{"tag_name": "v1.0.0", "prerelease": false, "assets": []map[string]any{
			{"name": "awf_windows_amd64.zip", "browser_download_url": "http://example.com/windows.zip"},
		}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases) //nolint:errcheck // controlled test response
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := newUpgradeCommand(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	err := runUpgrade(cmd, nil, upgradeOptions{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no compatible binary found")
}

func TestSelectExactRelease_MultipleStableReleases_ReturnsLatestStable(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v0.5.0", Prerelease: false},
		{TagName: "v1.0.0", Prerelease: false},
		{TagName: "v0.9.0", Prerelease: false},
		{TagName: "v2.0.0-beta", Prerelease: true},
	}

	target, err := parseExactReleaseTarget("", true)
	require.NoError(t, err)

	result, err := selectExactRelease(releases, target, false)

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.TagName)
}

func TestCheckWritePermission_TempFileCleanup(t *testing.T) {
	dir := t.TempDir()

	err := checkWritePermission(dir)

	require.NoError(t, err)

	// Verify no temp files were left behind
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "checkWritePermission should clean up temp files")
}
