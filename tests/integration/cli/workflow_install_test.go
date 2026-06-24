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
	"strings"
	"sync/atomic"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: C071 - Workflow Pack Format and Installation

// workflowAssetName returns the archive name for a workflow pack.
// Workflow packs are platform-independent (YAML/scripts only) — no os/arch suffix.
func workflowAssetName(packName, version string) string {
	return fmt.Sprintf("awf-workflow-%s_%s.tar.gz", packName, version)
}

// createTestWorkflowSHA256File creates a SHA-256 checksum file for the given asset filename.
// Format matches goreleaser checksums.txt: "<hex>  <filename>"
// This is workflow-pack specific (distinct from plugin test helpers).
func createTestWorkflowSHA256File(t *testing.T, tarballContent []byte, assetName string) string {
	t.Helper()

	hash := sha256.Sum256(tarballContent)
	hexHash := hex.EncodeToString(hash[:])
	return fmt.Sprintf("%s  %s", hexHash, assetName)
}

func readWorkflowPackSource(t *testing.T, packDir string) *workflowpkg.PackSource {
	t.Helper()

	stateData, err := os.ReadFile(filepath.Join(packDir, "state.json"))
	require.NoError(t, err)

	var state workflowpkg.PackState
	require.NoError(t, json.Unmarshal(stateData, &state))

	source, err := workflowpkg.PackSourceFromSourceData(state.SourceData)
	require.NoError(t, err)
	return source
}

// TestWorkflowSubcommands_Exist verifies install/remove subcommands exist.
func TestWorkflowSubcommands_Exist(t *testing.T) {
	subcommands := []string{"install", "remove"}

	root := cli.NewRootCommand()
	workflowCmd, _, err := root.Find([]string{"workflow"})
	require.NoError(t, err)

	for _, name := range subcommands {
		t.Run(name, func(t *testing.T) {
			found := false
			for _, sub := range workflowCmd.Commands() {
				if sub.Name() == name {
					found = true
					break
				}
			}
			assert.True(t, found, "workflow command should have %q subcommand", name)
		})
	}
}

func TestWorkflowInstall_CommandSyntaxAndFlags(t *testing.T) {
	root := cli.NewRootCommand()
	workflowCmd, _, err := root.Find([]string{"workflow"})
	require.NoError(t, err)

	var installCmdFound bool
	for _, sub := range workflowCmd.Commands() {
		if sub.Name() != "install" {
			continue
		}

		installCmdFound = true

		t.Run("newWorkflowInstallCommand keeps syntax equivalent to install owner repo version", func(t *testing.T) {
			assert.Equal(t, "install <owner/repo[@version]>", sub.Use)
		})

		t.Run("newWorkflowInstallCommand no longer registers a local version flag", func(t *testing.T) {
			assert.Nil(t, sub.Flags().Lookup("version"))
		})
	}

	require.True(t, installCmdFound, "workflow install command should exist")
}

// TestWorkflowInstall_SuccessfulLocalInstall validates basic local installation workflow.
// Covers: Downloads from GitHub, verifies checksum, extracts, validates manifest, and atomically installs.
func TestWorkflowInstall_SuccessfulLocalInstall(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	// Create workflow and plugin directories to verify they remain untouched (NFR-002)
	workflowsDir := filepath.Join(projDir, ".awf", "workflows")
	pluginsDir := filepath.Join(projDir, ".awf", "plugins")
	require.NoError(t, os.Mkdir(workflowsDir, 0o755))
	require.NoError(t, os.Mkdir(pluginsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "existing.yaml"), []byte("existing"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(pluginsDir, "existing-plugin"), []byte("existing"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	const version = "1.2.0"
	assetName := workflowAssetName("speckit", version)
	tarball := createTestWorkflowTarball(t, "speckit")
	checksumData := createTestWorkflowSHA256File(t, tarball, assetName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			releases := []map[string]interface{}{
				{
					"tag_name": "v0.9.0",
					"assets": []map[string]interface{}{
						{
							"name":                 workflowAssetName("speckit", "0.9.0"),
							"browser_download_url": "http://" + r.Host + "/downloads/" + workflowAssetName("speckit", "0.9.0"),
						},
					},
				},
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

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"workflow", "install", "testorg/awf-workflow-speckit"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.NoError(t, err, "install should succeed")

	// Verify pack was installed
	packDir := filepath.Join(projDir, ".awf", "workflow-packs", "speckit")
	_, err = os.Stat(packDir)
	assert.NoError(t, err, "pack directory should exist")

	manifestPath := filepath.Join(packDir, "manifest.yaml")
	_, err = os.Stat(manifestPath)
	assert.NoError(t, err, "manifest should exist in pack directory")

	// Verify state.json was written
	statePath := filepath.Join(packDir, "state.json")
	_, err = os.Stat(statePath)
	assert.NoError(t, err, "state.json should exist in pack directory")

	source := readWorkflowPackSource(t, packDir)
	assert.Equal(t, "testorg/awf-workflow-speckit", source.Repository)
	assert.Equal(t, version, source.Version)

	// Verify NFR-002: .awf/workflows and .awf/plugins remain untouched
	workflowContent, err := os.ReadFile(filepath.Join(workflowsDir, "existing.yaml"))
	require.NoError(t, err)
	assert.Equal(t, []byte("existing"), workflowContent, "existing workflow should not be modified")

	pluginBinary, err := os.ReadFile(filepath.Join(pluginsDir, "existing-plugin"))
	require.NoError(t, err)
	assert.Equal(t, []byte("existing"), pluginBinary, "existing plugin should not be modified")
}

// TestWorkflowInstall_SuccessfulGlobalInstall validates global installation workflow.
// Covers: Uses XDG_DATA_HOME path, installs to global workflow-packs directory.
func TestWorkflowInstall_SuccessfulGlobalInstall(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDataHome := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(xdgDataHome, 0o755))

	const version = "1.0.0"
	assetName := workflowAssetName("speckit", version)
	tarball := createTestWorkflowTarball(t, "speckit")
	checksumData := createTestWorkflowSHA256File(t, tarball, assetName)

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

	t.Setenv("XDG_DATA_HOME", xdgDataHome)
	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"workflow", "install", "testorg/awf-workflow-speckit", "--global"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.NoError(t, err, "global install should succeed")

	// Verify pack was installed to global location
	packDir := filepath.Join(xdgDataHome, "awf", "workflow-packs", "speckit")
	_, err = os.Stat(packDir)
	assert.NoError(t, err, "pack directory should exist in global XDG location")

	manifestPath := filepath.Join(packDir, "manifest.yaml")
	_, err = os.Stat(manifestPath)
	assert.NoError(t, err, "manifest should exist in global pack directory")
}

// TestWorkflowInstall_WithVersionConstraint validates installation with explicit version.
// Covers: Parses @version suffix, resolves correct release, installs exact version.
func TestWorkflowInstall_WithVersion(t *testing.T) {
	tests := []struct {
		name            string
		source          string
		expectedVersion string
	}{
		{
			name:            "Workflow install parses owner repo 1.2.3 validates exact SemVer before release lookup and selects exactly version v1.2.3 or equivalent normalized tag",
			source:          "testorg/awf-workflow-speckit@1.2.3",
			expectedVersion: "1.2.3",
		},
		{
			name:            "Workflow install parses owner repo v1.2.3 accepts the prefix and persists uses the normalized selected release version consistently with existing pack metadata behavior",
			source:          "testorg/awf-workflow-speckit@v1.2.3",
			expectedVersion: "1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			projDir := filepath.Join(tmpDir, "project")
			require.NoError(t, os.Mkdir(projDir, 0o755))
			require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

			origWd, err := os.Getwd()
			require.NoError(t, err)
			t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

			require.NoError(t, os.Chdir(projDir))

			assetName := workflowAssetName("speckit", tt.expectedVersion)
			tarball := createTestWorkflowTarball(t, "speckit")
			checksumData := createTestWorkflowSHA256File(t, tarball, assetName)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/releases") {
					releases := []map[string]interface{}{
						{
							"tag_name": "v3.0.0",
							"assets": []map[string]interface{}{
								{
									"name":                 workflowAssetName("speckit", "3.0.0"),
									"browser_download_url": "http://" + r.Host + "/downloads/" + workflowAssetName("speckit", "3.0.0"),
								},
							},
						},
						{
							"tag_name": "v" + tt.expectedVersion,
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
						{
							"tag_name": "v1.0.0",
							"assets": []map[string]interface{}{
								{
									"name":                 workflowAssetName("speckit", "1.0.0"),
									"browser_download_url": "http://" + r.Host + "/downloads/" + workflowAssetName("speckit", "1.0.0"),
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

			cmd := cli.NewRootCommand()
			cmd.SetArgs([]string{"workflow", "install", tt.source})

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			err = cmd.Execute()
			require.NoError(t, err, "install with exact version should succeed")

			packDir := filepath.Join(projDir, ".awf", "workflow-packs", "speckit")
			_, err = os.Stat(packDir)
			require.NoError(t, err, "pack directory should exist after versioned install")

			source := readWorkflowPackSource(t, packDir)
			assert.Equal(t, "testorg/awf-workflow-speckit", source.Repository)
			assert.Equal(t, tt.expectedVersion, source.Version)
		})
	}
}

// TestWorkflowInstall_ForceReinstall validates --force flag replaces existing pack.
// Covers: Allows overwriting existing pack when --force specified.
func TestWorkflowInstall_ForceReinstall(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	// Pre-install existing pack
	packDir := filepath.Join(projDir, ".awf", "workflow-packs", "speckit")
	require.NoError(t, os.MkdirAll(packDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte("name: speckit\nversion: 0.1.0\n"), 0o644))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	const version = "2.0.0"
	assetName := workflowAssetName("speckit", version)
	tarball := createTestWorkflowTarball(t, "speckit")
	checksumData := createTestWorkflowSHA256File(t, tarball, assetName)

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

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"workflow", "install", "testorg/awf-workflow-speckit", "--force"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.NoError(t, err, "force reinstall should succeed")

	// Verify pack directory still exists and was updated
	_, err = os.Stat(packDir)
	assert.NoError(t, err, "pack directory should exist after force reinstall")

	// Verify new state.json exists
	statePath := filepath.Join(packDir, "state.json")
	_, err = os.Stat(statePath)
	assert.NoError(t, err, "state.json should be updated after force reinstall")
}

func TestWorkflowInstall_RemovedVersionFlag(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"workflow", "install", "testorg/awf-workflow-speckit", "--version", "1.2.3"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag: --version")
	assert.Zero(t, hits.Load())
}

func TestWorkflowInstall_InvalidVersionSuffix(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		wantContains string
	}{
		{
			name:         `Workflow install rejects owner repo latest before release lookup with invalid release version latest`,
			source:       "testorg/awf-workflow-speckit@latest",
			wantContains: `invalid release version "latest": version: invalid format "latest"`,
		},
		{
			name:         `Workflow install rejects owner repo greater than or equal range before release lookup with invalid release version range`,
			source:       "testorg/awf-workflow-speckit@>=1.0.0",
			wantContains: `invalid release version ">=1.0.0": version: invalid format ">=1.0.0"`,
		},
		{
			name:         `Workflow install rejects owner repo empty suffix before release lookup with invalid release version empty string`,
			source:       "testorg/awf-workflow-speckit@",
			wantContains: `invalid release version "": version: empty string`,
		},
		{
			name:         `Workflow install rejects owner repo colon version before release lookup as unsupported syntax`,
			source:       "testorg/awf-workflow-speckit:1.2.3",
			wantContains: `owner/repo:version syntax is not supported; use owner/repo@version`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			t.Setenv("GITHUB_API_URL", server.URL)

			cmd := cli.NewRootCommand()
			cmd.SetArgs([]string{"workflow", "install", tt.source})

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantContains)
			assert.Zero(t, hits.Load())
		})
	}
}

// TestWorkflowRemove_SuccessfulRemoval validates complete remove workflow.
// Covers: Removes pack directory from local workflow-packs.
func TestWorkflowRemove_SuccessfulRemoval(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))

	packDir := filepath.Join(projDir, ".awf", "workflow-packs", "speckit")
	require.NoError(t, os.MkdirAll(packDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte("name: speckit\n"), 0o644))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"workflow", "remove", "speckit"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.NoError(t, err, "remove should succeed")

	_, err = os.Stat(packDir)
	assert.True(t, os.IsNotExist(err), "pack directory should be removed after successful removal")
}

// TestWorkflowRemove_SearchesGlobalFallback validates removal searches global if not found locally.
// Covers: Searches local first, then global ~user/.local/share/awf/workflow-packs.
func TestWorkflowRemove_SearchesGlobalFallback(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDataHome := filepath.Join(tmpDir, "data")

	globalPackDir := filepath.Join(xdgDataHome, "awf", "workflow-packs", "speckit")
	require.NoError(t, os.MkdirAll(globalPackDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(globalPackDir, "manifest.yaml"), []byte("name: speckit\n"), 0o644))

	require.NoError(t, os.Setenv("XDG_DATA_HOME", xdgDataHome))
	t.Cleanup(func() {
		os.Unsetenv("XDG_DATA_HOME")
	})

	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"workflow", "remove", "speckit"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.NoError(t, err, "remove should find and delete from global")

	_, err = os.Stat(globalPackDir)
	assert.True(t, os.IsNotExist(err), "pack directory should be removed from global")
}

// TestWorkflowInstall_InvalidRepoFormat validates error on malformed owner/repo.
// Covers: Rejects input without slash, returns user error (exit 1).
func TestWorkflowInstall_InvalidRepoFormat(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		wantContains string
	}{
		{
			name:         "Existing invalid repository format behavior remains unchanged and continues to use registry ValidateOwnerRepo errors missing slash separator",
			source:       "invalid-format",
			wantContains: "invalid owner/repo format: missing slash separator",
		},
		{
			name:         "Existing invalid repository format behavior remains unchanged and continues to use registry ValidateOwnerRepo errors empty owner segment",
			source:       "/awf-workflow-speckit",
			wantContains: "invalid owner/repo format: empty owner segment",
		},
		{
			name:         "Existing invalid repository format behavior remains unchanged and continues to use registry ValidateOwnerRepo errors multiple slashes not allowed",
			source:       "testorg/team/awf-workflow-speckit",
			wantContains: "invalid owner/repo format: multiple slashes not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			projDir := filepath.Join(tmpDir, "project")
			require.NoError(t, os.Mkdir(projDir, 0o755))
			require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

			origWd, err := os.Getwd()
			require.NoError(t, err)
			t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

			require.NoError(t, os.Chdir(projDir))

			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			t.Setenv("GITHUB_API_URL", server.URL)

			cmd := cli.NewRootCommand()
			cmd.SetArgs([]string{"workflow", "install", tt.source})

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			err = cmd.Execute()
			require.Error(t, err, "install with invalid repo format should fail")
			assert.Contains(t, err.Error(), tt.wantContains)
			assert.Zero(t, hits.Load())
		})
	}
}

// TestWorkflowInstall_VersionNotFound validates error when requested version doesn't exist.
// Covers: Queries releases API, fails when version not found.
func TestWorkflowInstall_VersionNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			// Only 1.0.0 and 2.0.0 available
			releases := []map[string]interface{}{
				{
					"tag_name": "v2.0.0",
					"assets": []map[string]interface{}{
						{
							"name":                 workflowAssetName("speckit", "2.0.0"),
							"browser_download_url": "http://" + r.Host + "/downloads/" + workflowAssetName("speckit", "2.0.0"),
						},
					},
				},
				{
					"tag_name": "v1.0.0",
					"assets": []map[string]interface{}{
						{
							"name":                 workflowAssetName("speckit", "1.0.0"),
							"browser_download_url": "http://" + r.Host + "/downloads/" + workflowAssetName("speckit", "1.0.0"),
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(releases) //nolint:errcheck // test fixture response
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"workflow", "install", "testorg/awf-workflow-speckit@1.2.3"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	require.Error(t, err, "install with nonexistent version should fail")
	assert.Contains(t, err.Error(), "release version 1.2.3 not found")
}

// TestWorkflowInstall_ChecksumMismatchAborts validates checksum verification prevents installation.
// Covers: Rejects mismatched checksum, leaves no partial files on disk.
func TestWorkflowInstall_ChecksumMismatchAborts(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	const version = "1.0.0"
	assetName := workflowAssetName("speckit", version)
	tarball := createTestWorkflowTarball(t, "speckit")

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
			// Intentionally wrong checksum
			w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000  " + assetName)) //nolint:errcheck // test fixture response
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"workflow", "install", "testorg/awf-workflow-speckit"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.Error(t, err, "installation should fail with bad checksum")

	// Verify no partial installation
	packDir := filepath.Join(projDir, ".awf", "workflow-packs")
	entries, _ := os.ReadDir(packDir)
	assert.Equal(t, 0, len(entries), "no partial pack files should remain after checksum failure")
}

// createTestWorkflowTarball creates a minimal tar.gz with manifest and workflow files.
func createTestWorkflowTarball(t *testing.T, packName string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// Add manifest file
	manifestContent := fmt.Sprintf(`name: %s
version: "1.0.0"
description: Test workflow pack
author: test-author
license: MIT
awf_version: ">=0.5.0"
workflows:
  - specify
  - clarify
`, packName)

	header := &tar.Header{
		Name: "manifest.yaml",
		Size: int64(len(manifestContent)),
		Mode: 0o644,
	}
	require.NoError(t, tw.WriteHeader(header))
	_, err := tw.Write([]byte(manifestContent))
	require.NoError(t, err)

	// Add workflow files directory
	workflowsDir := "workflows/"
	header = &tar.Header{
		Name:     workflowsDir,
		Mode:     0o755,
		Typeflag: tar.TypeDir,
	}
	require.NoError(t, tw.WriteHeader(header))

	// Add workflow files
	workflowContent := `steps:
  - name: step1
    config:
      provider: github
`
	for _, name := range []string{"specify", "clarify"} {
		header = &tar.Header{
			Name: workflowsDir + name + ".yaml",
			Size: int64(len(workflowContent)),
			Mode: 0o644,
		}
		require.NoError(t, tw.WriteHeader(header))
		_, err := tw.Write([]byte(workflowContent))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	return buf.Bytes()
}
