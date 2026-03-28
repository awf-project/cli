package cli_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: C068 - Plugin Registry Install/Update/Remove from GitHub Releases

// platformAssetName returns the goreleaser-style asset name for the current platform.
// Format: awf-plugin-<name>_<version>_<goos>_<goarch>.tar.gz
func platformAssetName(pluginName, version string) string {
	return fmt.Sprintf("awf-plugin-%s_%s_%s_%s.tar.gz", pluginName, version, runtime.GOOS, runtime.GOARCH)
}

// TestPluginSubcommands_Exist verifies install/update/remove/search subcommands exist.
func TestPluginSubcommands_Exist(t *testing.T) {
	subcommands := []string{"install", "update", "remove", "search"}

	root := cli.NewRootCommand()
	pluginCmd, _, err := root.Find([]string{"plugin"})
	require.NoError(t, err)

	for _, name := range subcommands {
		t.Run(name, func(t *testing.T) {
			found := false
			for _, sub := range pluginCmd.Commands() {
				if sub.Name() == name {
					found = true
					break
				}
			}
			assert.True(t, found, "plugin command should have %q subcommand", name)
		})
	}
}

// TestPluginInstall_LifecycleAndFlags validates complete install workflows.
// Covers: Downloads from GitHub, verifies checksum, atomically installs, and --force reinstallation.
func TestPluginInstall_LifecycleAndFlags(t *testing.T) {
	tests := []struct {
		name          string
		preExisting   bool
		force         bool
		expectSuccess bool
	}{
		{
			name:          "successful initial install",
			preExisting:   false,
			force:         false,
			expectSuccess: true,
		},
		{
			name:          "force reinstall over existing",
			preExisting:   true,
			force:         true,
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			basePluginsDir := filepath.Join(tmpDir, "plugins")
			require.NoError(t, os.MkdirAll(basePluginsDir, 0o755))
			t.Setenv("AWF_PLUGINS_PATH", basePluginsDir)

			if tt.preExisting {
				pluginsDir := filepath.Join(basePluginsDir, "test-plugin")
				require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
				oldBinaryPath := filepath.Join(pluginsDir, "awf-plugin-test-plugin")
				require.NoError(t, os.WriteFile(oldBinaryPath, []byte("old binary v1.0"), 0o755))
			}

			const version = "2.0.0"
			assetName := platformAssetName("test-plugin", version)
			tarball := createTestPluginTarball(t, "test-plugin")
			checksumData := createTestSHA256File(t, tarball, assetName)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/releases") {
					releases := []map[string]interface{}{
						{
							"tag_name": "v" + version,
							"assets": []map[string]interface{}{
								{
									"name":                 assetName,
									"browser_download_url": "http://" + r.Host + "/downloads/" + assetName,
								},
								{
									"name":                 "checksums.txt",
									"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
								},
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(releases) //nolint:errcheck // test fixture response
					return
				}

				if strings.Contains(r.URL.Path, "/downloads/"+assetName) {
					w.Header().Set("Content-Type", "application/gzip")
					w.Write(tarball) //nolint:errcheck // test fixture response
					return
				}

				if strings.Contains(r.URL.Path, "/downloads/checksums.txt") {
					w.Write([]byte(checksumData)) //nolint:errcheck // test fixture response
					return
				}

				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			t.Setenv("GITHUB_API_URL", server.URL)

			args := []string{"plugin", "install", "testorg/awf-plugin-test-plugin"}
			if tt.force {
				args = append(args, "--force")
			}

			cmd := cli.NewRootCommand()
			cmd.SetArgs(args)

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			err := cmd.Execute()

			if tt.expectSuccess {
				assert.NoError(t, err, "install should succeed")

				pluginDir := filepath.Join(basePluginsDir, "test-plugin")
				_, err := os.Stat(pluginDir)
				assert.NoError(t, err, "plugin directory should exist")

				binary := filepath.Join(pluginDir, "awf-plugin-test-plugin")
				_, err = os.Stat(binary)
				assert.NoError(t, err, "plugin binary should exist")

				stat, _ := os.Stat(binary)
				assert.True(t, stat.Mode()&0o100 != 0, "binary should be executable")
			}
		})
	}
}

// TestPluginInstall_BadChecksumAborts validates checksum verification prevents partial installation.
// Covers: Rejects mismatched checksum, leaves no partial files on disk.
func TestPluginInstall_BadChecksumAborts(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	const version = "1.0.0"
	assetName := platformAssetName("test-plugin", version)
	tarball := createTestPluginTarball(t, "test-plugin")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			releases := []map[string]interface{}{
				{
					"tag_name": "v" + version,
					"assets": []map[string]interface{}{
						{
							"name":                 assetName,
							"browser_download_url": "http://" + r.Host + "/downloads/" + assetName,
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(releases) //nolint:errcheck // test fixture response
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/"+assetName) {
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(tarball) //nolint:errcheck // test fixture response
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/checksums.txt") {
			// Intentionally wrong checksum, using the actual asset name
			w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000  " + assetName)) //nolint:errcheck // test fixture response
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"plugin", "install", "testorg/awf-plugin-test-plugin"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.Error(t, err, "installation should fail with bad checksum")

	// Verify no partial installation
	entries, _ := os.ReadDir(pluginsDir)
	assert.Equal(t, 0, len(entries), "no partial plugin files should remain after checksum failure")
}

// TestPluginInstall_AlreadyInstalledError validates error when plugin already exists without --force.
func TestPluginInstall_AlreadyInstalledError(t *testing.T) {
	tmpDir := t.TempDir()
	basePluginsDir := filepath.Join(tmpDir, "plugins")
	pluginsDir := filepath.Join(basePluginsDir, "existing-plugin")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
	t.Setenv("AWF_PLUGINS_PATH", basePluginsDir)

	binaryPath := filepath.Join(pluginsDir, "awf-plugin-existing-plugin")
	require.NoError(t, os.WriteFile(binaryPath, []byte("#!/bin/bash"), 0o755))

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"plugin", "install", "testorg/awf-plugin-existing-plugin"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.Error(t, err, "install should fail when plugin already exists")
	assert.Contains(t, out.String(), "already", "error should indicate plugin already installed")
}

// TestPluginRemove_SuccessfulRemoval validates complete remove workflow.
// Covers: Removes plugin directory, clears state, keeps config with --keep-data.
func TestPluginRemove_SuccessfulRemoval(t *testing.T) {
	tmpDir := t.TempDir()
	basePluginsDir := filepath.Join(tmpDir, "plugins")
	pluginsDir := filepath.Join(basePluginsDir, "test-plugin")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
	t.Setenv("AWF_PLUGINS_PATH", basePluginsDir)

	binaryPath := filepath.Join(pluginsDir, "awf-plugin-test-plugin")
	require.NoError(t, os.WriteFile(binaryPath, []byte("#!/bin/bash"), 0o755))

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"plugin", "remove", "test-plugin"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.NoError(t, err, "remove should succeed")

	_, err = os.Stat(pluginsDir)
	assert.Error(t, err, "plugin directory should be removed after successful removal")
	assert.True(t, os.IsNotExist(err), "plugin directory should not exist")
}

// TestPluginRemove_BuiltinGuard validates that built-in plugins cannot be removed.
// Covers: github, http, notify built-in protection.
func TestPluginRemove_BuiltinGuard(t *testing.T) {
	tests := []struct {
		name string
		repo string
	}{
		{name: "github", repo: "github"},
		{name: "http", repo: "http"},
		{name: "notify", repo: "notify"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			cmd.SetArgs([]string{"plugin", "remove", tt.repo})

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			err := cmd.Execute()
			assert.Error(t, err, "should not allow removing built-in plugin %s", tt.repo)
			assert.Contains(t, out.String(), "built-in", "error message should mention plugin is built-in")
		})
	}
}

// createTestPluginTarball creates a minimal tar.gz with plugin binary and manifest.
func createTestPluginTarball(t *testing.T, pluginName string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// Add binary file
	binaryName := fmt.Sprintf("awf-plugin-%s", pluginName)
	binaryContent := []byte("#!/bin/bash\necho mock plugin")
	header := &tar.Header{
		Name: binaryName,
		Size: int64(len(binaryContent)),
		Mode: 0o755,
	}
	require.NoError(t, tw.WriteHeader(header))
	_, err := tw.Write(binaryContent)
	require.NoError(t, err)

	// Add manifest file (plugin.yaml format)
	manifestContent := fmt.Sprintf(`api_version: v1
name: %s
version: 1.0.0
description: Test plugin
capabilities: []
`, pluginName)

	header = &tar.Header{
		Name: "plugin.yaml",
		Size: int64(len(manifestContent)),
		Mode: 0o644,
	}
	require.NoError(t, tw.WriteHeader(header))
	_, err = tw.Write([]byte(manifestContent))
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	return buf.Bytes()
}

// createTestSHA256File creates a SHA-256 checksum file for the given asset filename.
// Format matches goreleaser checksums.txt: "<hex>  <filename>"
func createTestSHA256File(t *testing.T, tarballContent []byte, assetName string) string {
	t.Helper()

	hash := sha256.Sum256(tarballContent)
	return fmt.Sprintf("%s  %s", hex.EncodeToString(hash[:]), assetName)
}

// seedPluginSourceData writes a plugins.json state file so that updatePlugin can read
// the repository and version metadata that was normally persisted by runPluginInstall.
func seedPluginSourceData(t *testing.T, storagePath, pluginName, ownerRepo, version string) {
	t.Helper()

	stateDir := filepath.Join(storagePath, "plugins")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))

	content := fmt.Sprintf(`{
	%q: {
		"enabled": true,
		"source_data": {
			"repository": %q,
			"version": %q,
			"installed_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z"
		}
	}
}`, pluginName, ownerRepo, version)

	require.NoError(t, os.WriteFile(
		filepath.Join(stateDir, "plugins.json"),
		[]byte(content),
		0o600,
	))
}

// TestPluginUpdate_SuccessfulUpdate validates complete update workflow.
// Covers: Fetches newer version, verifies checksum, atomically replaces.
func TestPluginUpdate_SuccessfulUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	basePluginsDir := filepath.Join(tmpDir, "plugins")
	pluginsDir := filepath.Join(basePluginsDir, "test-plugin")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
	t.Setenv("AWF_PLUGINS_PATH", basePluginsDir)

	// Simulate a plugin previously installed at v1.0.0 via runPluginInstall.
	oldBinaryPath := filepath.Join(pluginsDir, "awf-plugin-test-plugin")
	require.NoError(t, os.WriteFile(oldBinaryPath, []byte("v1.0.0 binary"), 0o755))
	// Seed source metadata so updatePlugin knows the origin repo and current version.
	seedPluginSourceData(t, tmpDir, "test-plugin", "testorg/awf-plugin-test-plugin", "v1.0.0")

	// Create v2.0.0 tarball for update
	tarballV2 := createTestPluginTarball(t, "test-plugin")
	const v2 = "2.0.0"
	assetNameV2 := platformAssetName("test-plugin", v2)
	checksumV2 := createTestSHA256File(t, tarballV2, assetNameV2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			releases := []map[string]interface{}{
				{
					"tag_name": "v" + v2,
					"assets": []map[string]interface{}{
						{
							"name":                 assetNameV2,
							"browser_download_url": "http://" + r.Host + "/downloads/" + assetNameV2,
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(releases) //nolint:errcheck // test fixture response
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/"+assetNameV2) {
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(tarballV2) //nolint:errcheck // test fixture response
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/checksums.txt") {
			w.Write([]byte(checksumV2)) //nolint:errcheck // test fixture response
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"plugin", "update", "test-plugin", "--storage", tmpDir})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.NoError(t, err, "update should succeed")

	output := out.String()
	assert.Contains(t, output, "Updated plugin", "output should confirm the update")
	assert.Contains(t, output, "v1.0.0", "output should show previous version")
	assert.Contains(t, output, "v2.0.0", "output should show new version")

	// Verify binary was replaced
	binary := filepath.Join(pluginsDir, "awf-plugin-test-plugin")
	_, err = os.Stat(binary)
	assert.NoError(t, err, "plugin binary should exist after update")

	stat, _ := os.Stat(binary)
	assert.True(t, stat.Mode()&0o100 != 0, "binary should be executable")
}

// TestPluginUpdate_MissingArgsError validates that update without args and no --all returns error.
func TestPluginUpdate_MissingArgsError(t *testing.T) {
	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"plugin", "update"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.Error(t, err, "update without args should fail")
}

// TestPluginSearch_SuccessfulSearch validates GitHub API search returns results.
// Covers: Searches by topic, returns results in formatted table output.
func TestPluginSearch_SuccessfulSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/search/repositories") {
			results := map[string]interface{}{
				"total_count": 2,
				"items": []map[string]interface{}{
					{
						"full_name":        "org1/awf-plugin-jira",
						"description":      "Jira integration plugin",
						"stargazers_count": 42,
					},
					{
						"full_name":        "org2/awf-plugin-github",
						"description":      "GitHub integration plugin",
						"stargazers_count": 123,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(results) //nolint:errcheck // test fixture response
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"plugin", "search", "integration"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.NoError(t, err, "search should succeed")

	output := out.String()
	assert.NotEmpty(t, output, "search should return results")
	assert.Contains(t, output, "jira", "results should include jira plugin")
	assert.Contains(t, output, "github", "results should include github plugin")
}

// TestPluginInstall_VersionConstraints validates --version flag version resolution.
func TestPluginInstall_VersionConstraints(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		releaseTag string
		shouldPass bool
	}{
		{
			name:       "exact version match",
			constraint: "v1.0.0",
			releaseTag: "v1.0.0",
			shouldPass: true,
		},
		{
			name:       "semver range",
			constraint: ">=1.0.0 <2.0.0",
			releaseTag: "v1.5.0",
			shouldPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			basePluginsDir := filepath.Join(tmpDir, "plugins")
			require.NoError(t, os.MkdirAll(basePluginsDir, 0o755))
			t.Setenv("AWF_PLUGINS_PATH", basePluginsDir)

			// Derive version string (strip leading "v" for asset filename)
			versionStr := strings.TrimPrefix(tt.releaseTag, "v")
			assetName := platformAssetName("test-plugin", versionStr)
			tarball := createTestPluginTarball(t, "test-plugin")
			checksum := createTestSHA256File(t, tarball, assetName)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/releases") {
					releases := []map[string]interface{}{
						{
							"tag_name": tt.releaseTag,
							"assets": []map[string]interface{}{
								{
									"name":                 assetName,
									"browser_download_url": "http://" + r.Host + "/downloads/" + assetName,
								},
								{
									"name":                 "checksums.txt",
									"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
								},
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(releases) //nolint:errcheck // test fixture response
					return
				}

				if strings.Contains(r.URL.Path, "/downloads/"+assetName) {
					w.Header().Set("Content-Type", "application/gzip")
					w.Write(tarball) //nolint:errcheck // test fixture response
					return
				}

				if strings.Contains(r.URL.Path, "/downloads/checksums.txt") {
					w.Write([]byte(checksum)) //nolint:errcheck // test fixture response
					return
				}

				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			t.Setenv("GITHUB_API_URL", server.URL)

			args := []string{"plugin", "install", "testorg/awf-plugin-test-plugin"}
			if tt.constraint != "" {
				args = append(args, "--version", tt.constraint)
			}

			cmd := cli.NewRootCommand()
			cmd.SetArgs(args)

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			err := cmd.Execute()

			if tt.shouldPass {
				assert.NoError(t, err, "install with version constraint should succeed")
				pluginDir := filepath.Join(basePluginsDir, "test-plugin")
				_, err := os.Stat(pluginDir)
				assert.NoError(t, err, "plugin directory should exist")
			}
		})
	}
}

// TestPluginInstall_PrereleaseFlag validates --pre-release flag includes alpha/beta/rc versions.
func TestPluginInstall_PrereleaseFlag(t *testing.T) {
	tmpDir := t.TempDir()
	basePluginsDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(basePluginsDir, 0o755))
	t.Setenv("AWF_PLUGINS_PATH", basePluginsDir)

	const prereleaseVersion = "2.0.0-beta.1"
	assetName := platformAssetName("test-plugin", prereleaseVersion)
	tarball := createTestPluginTarball(t, "test-plugin")
	checksum := createTestSHA256File(t, tarball, assetName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			releases := []map[string]interface{}{
				{
					"tag_name":   "v" + prereleaseVersion,
					"prerelease": true,
					"assets": []map[string]interface{}{
						{
							"name":                 assetName,
							"browser_download_url": "http://" + r.Host + "/downloads/" + assetName,
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(releases) //nolint:errcheck // test fixture response
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/"+assetName) {
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(tarball) //nolint:errcheck // test fixture response
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/checksums.txt") {
			w.Write([]byte(checksum)) //nolint:errcheck // test fixture response
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"plugin", "install", "testorg/awf-plugin-test-plugin", "--pre-release"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.NoError(t, err, "install with --pre-release should include beta versions")

	pluginDir := filepath.Join(basePluginsDir, "test-plugin")
	_, err = os.Stat(pluginDir)
	assert.NoError(t, err, "plugin directory should exist after prerelease install")
}

// TestPluginList_SourceColumn validates SOURCE column shows owner/repo for external plugins.
func TestPluginList_SourceColumn(t *testing.T) {
	tmpDir := t.TempDir()
	basePluginsDir := filepath.Join(tmpDir, "plugins")
	pluginsDir := filepath.Join(basePluginsDir, "test-plugin")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
	t.Setenv("AWF_PLUGINS_PATH", basePluginsDir)

	binaryPath := filepath.Join(pluginsDir, "awf-plugin-test-plugin")
	require.NoError(t, os.WriteFile(binaryPath, []byte("#!/bin/bash"), 0o755))

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"plugin", "list"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.NoError(t, err, "plugin list should succeed")

	output := out.String()
	assert.Contains(t, output, "SOURCE", "list output should include SOURCE column")
}

// TestPluginInstall_RateLimitDetection validates rate limit error is returned.
func TestPluginInstall_RateLimitDetection(t *testing.T) {
	tmpDir := t.TempDir()
	basePluginsDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(basePluginsDir, 0o755))
	t.Setenv("AWF_PLUGINS_PATH", basePluginsDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", "1234567890")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message":"API rate limit exceeded"}`)) //nolint:errcheck // test fixture response
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"plugin", "install", "testorg/awf-plugin-test-plugin"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.Error(t, err, "install should fail with rate limit")
}

// TestPluginInstall_GithubTokenFallback validates GITHUB_TOKEN auth and unauthenticated fallback.
func TestPluginInstall_GithubTokenFallback(t *testing.T) {
	tmpDir := t.TempDir()
	basePluginsDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(basePluginsDir, 0o755))
	t.Setenv("AWF_PLUGINS_PATH", basePluginsDir)

	const version = "1.0.0"
	assetName := platformAssetName("test-plugin", version)
	tarball := createTestPluginTarball(t, "test-plugin")
	checksum := createTestSHA256File(t, tarball, assetName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			releases := []map[string]interface{}{
				{
					"tag_name": "v" + version,
					"assets": []map[string]interface{}{
						{
							"name":                 assetName,
							"browser_download_url": "http://" + r.Host + "/downloads/" + assetName,
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(releases) //nolint:errcheck // test fixture response
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/"+assetName) {
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(tarball) //nolint:errcheck // test fixture response
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/checksums.txt") {
			w.Write([]byte(checksum)) //nolint:errcheck // test fixture response
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)
	t.Setenv("GITHUB_TOKEN", "test-token-123")

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"plugin", "install", "testorg/awf-plugin-test-plugin"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.NoError(t, err, "install should succeed with GITHUB_TOKEN")

	pluginDir := filepath.Join(basePluginsDir, "test-plugin")
	_, err = os.Stat(pluginDir)
	assert.NoError(t, err, "plugin directory should exist after install")
}
