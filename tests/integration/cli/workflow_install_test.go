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
	"testing"

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
func TestWorkflowInstall_WithVersionConstraint(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	const requestedVersion = "2.0.0"
	assetName := workflowAssetName("speckit", requestedVersion)
	tarball := createTestWorkflowTarball(t, "speckit")
	checksumData := createTestWorkflowSHA256File(t, tarball, assetName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			// API returns multiple versions
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
					"tag_name": "v" + requestedVersion,
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
	cmd.SetArgs([]string{"workflow", "install", "testorg/awf-workflow-speckit@" + requestedVersion})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.NoError(t, err, "install with version constraint should succeed")

	packDir := filepath.Join(projDir, ".awf", "workflow-packs", "speckit")
	_, err = os.Stat(packDir)
	assert.NoError(t, err, "pack directory should exist after versioned install")
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
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"workflow", "install", "invalid-format"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.Error(t, err, "install with invalid repo format should fail")
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
	cmd.SetArgs([]string{"workflow", "install", "testorg/awf-workflow-speckit@99.99.99"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.Error(t, err, "install with nonexistent version should fail")
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
