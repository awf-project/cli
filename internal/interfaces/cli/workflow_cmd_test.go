package cli

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

	"github.com/awf-project/cli/internal/application"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkflowInstall_ValidRepoWithVersion installs workflow pack with pinned version.
func TestWorkflowInstall_ValidRepoWithVersion(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	// Mock GitHub API
	tarball := createTestWorkflowTarballForUnit(t, "speckit")
	checksumData := createTestWorkflowSHA256FileForUnit(t, tarball, "awf-workflow-speckit_1.2.0.tar.gz")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			releases := []map[string]interface{}{
				{
					"tag_name": "v1.2.0",
					"assets": []map[string]interface{}{
						{
							"name":                 "awf-workflow-speckit_1.2.0.tar.gz",
							"browser_download_url": "http://" + r.Host + "/downloads/awf-workflow-speckit_1.2.0.tar.gz",
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(releases)
			return
		}
		if strings.Contains(r.URL.Path, "/downloads/awf-workflow-speckit") {
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(tarball)
			return
		}
		if strings.Contains(r.URL.Path, "/downloads/checksums.txt") {
			_, _ = w.Write([]byte(checksumData))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cfg := &Config{}
	flags := workflowInstallFlags{version: "1.2.0", global: false, force: false}

	err = runWorkflowInstall(&cobra.Command{}, cfg, "org/awf-workflow-speckit", flags)

	require.NoError(t, err, "should install workflow pack successfully")
}

// TestWorkflowInstall_GlobalFlag sets up conditions for global install.
func TestWorkflowInstall_GlobalFlag(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDataHome := filepath.Join(tmpDir, "data")
	require.NoError(t, os.Setenv("XDG_DATA_HOME", xdgDataHome))
	t.Cleanup(func() {
		os.Unsetenv("XDG_DATA_HOME")
	})

	cfg := &Config{}
	flags := workflowInstallFlags{version: "1.0.0", global: true, force: false}

	err := runWorkflowInstall(&cobra.Command{}, cfg, "org/awf-workflow-speckit", flags)

	// Global flag prevents .awf/ project check — error comes from GitHub client, not project validation.
	require.Error(t, err)
	assert.NotContains(t, err.Error(), ".awf", "error should come from GitHub client, not project validation")
}

// TestWorkflowInstall_MissingProjectDir fails when .awf/ doesn't exist and not global.
func TestWorkflowInstall_MissingProjectDir(t *testing.T) {
	tmpDir := t.TempDir()

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(tmpDir))

	cfg := &Config{}
	flags := workflowInstallFlags{version: "1.0.0", global: false, force: false}

	err = runWorkflowInstall(&cobra.Command{}, cfg, "org/awf-workflow-speckit", flags)

	require.Error(t, err, "should error when .awf/ missing and not global")
}

// TestWorkflowInstall_InvalidRepoFormat fails with malformed owner/repo.
func TestWorkflowInstall_InvalidRepoFormat(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	cfg := &Config{}
	flags := workflowInstallFlags{version: "", global: false, force: false}

	err = runWorkflowInstall(&cobra.Command{}, cfg, "invalid-format", flags)

	require.Error(t, err, "should error for invalid repo format")
}

// TestWorkflowInstall_ForceReplaces verifies --force flag handling.
func TestWorkflowInstall_ForceReplaces(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	packDir := filepath.Join(projDir, ".awf", "workflow-packs", "speckit")
	require.NoError(t, os.MkdirAll(packDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte("name: speckit\n"), 0o644))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	cfg := &Config{}
	flags := workflowInstallFlags{version: "1.0.0", global: false, force: true}

	err = runWorkflowInstall(&cobra.Command{}, cfg, "org/awf-workflow-speckit", flags)

	// With --force, the pack directory check is bypassed and the error comes from
	// the GitHub client (no mock server), not from the existing pack directory.
	require.Error(t, err, "expected network error from GitHub client")
}

// TestWorkflowRemove_DeletesPack removes workflow pack directory.
func TestWorkflowRemove_DeletesPack(t *testing.T) {
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

	cfg := &Config{}

	err = runWorkflowRemove(&cobra.Command{}, cfg, "speckit")

	require.NoError(t, err, "should remove local pack directory successfully")
	assert.NoDirExists(t, packDir, "pack directory should be deleted after remove")
}

// Helper functions for unit tests

func createTestWorkflowTarballForUnit(t *testing.T, packName string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	manifestContent := fmt.Sprintf(`name: %s
version: "1.0.0"
description: Test workflow pack
author: test-author
license: MIT
awf_version: ">=0.5.0"
workflows:
  - specify
`, packName)

	header := &tar.Header{
		Name: "manifest.yaml",
		Size: int64(len(manifestContent)),
		Mode: 0o644,
	}
	require.NoError(t, tw.WriteHeader(header))
	_, _ = tw.Write([]byte(manifestContent))

	workflowsDir := "workflows/"
	header = &tar.Header{
		Name:     workflowsDir,
		Mode:     0o755,
		Typeflag: tar.TypeDir,
	}
	require.NoError(t, tw.WriteHeader(header))

	workflowContent := `steps:
  - name: step1
    config:
      provider: github
`
	header = &tar.Header{
		Name: workflowsDir + "specify.yaml",
		Size: int64(len(workflowContent)),
		Mode: 0o644,
	}
	require.NoError(t, tw.WriteHeader(header))
	_, _ = tw.Write([]byte(workflowContent))

	_ = tw.Close()
	_ = gz.Close()

	return buf.Bytes()
}

func createTestWorkflowSHA256FileForUnit(t *testing.T, tarballContent []byte, assetName string) string {
	t.Helper()

	hash := sha256.Sum256(tarballContent)
	hexHash := hex.EncodeToString(hash[:])
	return fmt.Sprintf("%s  %s", hexHash, assetName)
}

// TestWorkflowRemove_PackNotFound fails when pack doesn't exist.
func TestWorkflowRemove_PackNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	cfg := &Config{}

	err = runWorkflowRemove(&cobra.Command{}, cfg, "nonexistent")

	require.Error(t, err, "should error when pack not found in local or global")
}

// TestWorkflowRemove_SearchesGlobal finds pack in global dir if not in local.
func TestWorkflowRemove_SearchesGlobal(t *testing.T) {
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

	cfg := &Config{}

	err = runWorkflowRemove(&cobra.Command{}, cfg, "speckit")

	// Pack exists only in global dir; remove should find and delete it.
	require.NoError(t, err, "should remove global pack directory successfully")
	assert.NoDirExists(t, globalPackDir, "global pack directory should be deleted after remove")
}

// TestWorkflowUpdate_SinglePack updates a single pack without --all flag.
func TestWorkflowUpdate_SinglePack(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	// Create local pack directory with state
	packDir := filepath.Join(projDir, ".awf", "workflow-packs", "speckit")
	require.NoError(t, os.MkdirAll(packDir, 0o755))

	// Create state.json with source data pointing to org/repo
	stateContent := `{
		"name": "speckit",
		"enabled": true,
		"source_data": {
			"repository": "org/awf-workflow-speckit",
			"version": "1.0.0",
			"installed_at": "2025-01-01T00:00:00Z",
			"updated_at": "2025-01-01T00:00:00Z"
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(stateContent), 0o644))

	// Create manifest.yaml
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte("name: speckit\nversion: 1.0.0\n"), 0o644))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	// Mock GitHub API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repos/org/awf-workflow-speckit/releases") {
			releases := []map[string]interface{}{
				{
					"tag_name": "v1.1.0",
					"assets": []map[string]interface{}{
						{
							"name":                 "awf-workflow-speckit_1.1.0.tar.gz",
							"browser_download_url": "http://" + r.Host + "/downloads/pack.tar.gz",
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(releases)
			return
		}
		if strings.Contains(r.URL.Path, "/downloads/pack.tar.gz") {
			tarball := createTestWorkflowTarballForUnit(t, "speckit")
			_, _ = w.Write(tarball)
			return
		}
		if strings.Contains(r.URL.Path, "/downloads/checksums.txt") {
			tarball := createTestWorkflowTarballForUnit(t, "speckit")
			checksum := createTestWorkflowSHA256FileForUnit(t, tarball, "awf-workflow-speckit_1.1.0.tar.gz")
			_, _ = w.Write([]byte(checksum))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cfg := &Config{}
	flags := workflowUpdateFlags{all: false}

	cmd := &cobra.Command{}
	cmd.SetOut(bytes.NewBuffer(nil))
	cmd.SetErr(bytes.NewBuffer(nil))

	err = runWorkflowUpdate(cmd, cfg, "speckit", flags)

	require.NoError(t, err, "should update single pack successfully")
}

// TestWorkflowUpdate_AllPacks updates all installed packs with --all flag.
func TestWorkflowUpdate_AllPacks(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	// Create two packs: speckit and other-pack
	for _, packName := range []string{"speckit", "other-pack"} {
		packDir := filepath.Join(projDir, ".awf", "workflow-packs", packName)
		require.NoError(t, os.MkdirAll(packDir, 0o755))

		stateContent := fmt.Sprintf(`{
			"name": "%s",
			"enabled": true,
			"source_data": {
				"repository": "org/awf-workflow-%s",
				"version": "1.0.0",
				"installed_at": "2025-01-01T00:00:00Z",
				"updated_at": "2025-01-01T00:00:00Z"
			}
		}`, packName, packName)
		require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(stateContent), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(fmt.Sprintf("name: %s\nversion: 1.0.0\n", packName)), 0o644))
	}

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	// Mock GitHub API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repos/org/awf-workflow-") && strings.Contains(r.URL.Path, "/releases") {
			releases := []map[string]interface{}{
				{
					"tag_name": "v1.1.0",
					"assets": []map[string]interface{}{
						{"name": "pack.tar.gz", "browser_download_url": "http://" + r.Host + "/downloads/pack.tar.gz"},
						{"name": "checksums.txt", "browser_download_url": "http://" + r.Host + "/downloads/checksums.txt"},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(releases)
			return
		}
		if strings.Contains(r.URL.Path, "/downloads/pack.tar.gz") {
			tarball := createTestWorkflowTarballForUnit(t, "test-pack")
			_, _ = w.Write(tarball)
			return
		}
		if strings.Contains(r.URL.Path, "/downloads/checksums.txt") {
			tarball := createTestWorkflowTarballForUnit(t, "test-pack")
			checksum := createTestWorkflowSHA256FileForUnit(t, tarball, "pack.tar.gz")
			_, _ = w.Write([]byte(checksum))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cfg := &Config{}
	flags := workflowUpdateFlags{all: true}

	cmd := &cobra.Command{}
	cmd.SetOut(bytes.NewBuffer(nil))
	cmd.SetErr(bytes.NewBuffer(nil))

	err = runWorkflowUpdate(cmd, cfg, "", flags)

	require.NoError(t, err, "should update all packs successfully")
}

// TestWorkflowUpdate_PackNotFound returns error when pack doesn't exist.
func TestWorkflowUpdate_PackNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	cfg := &Config{}
	flags := workflowUpdateFlags{all: false}

	cmd := &cobra.Command{}
	cmd.SetOut(bytes.NewBuffer(nil))
	cmd.SetErr(bytes.NewBuffer(nil))

	err = runWorkflowUpdate(cmd, cfg, "nonexistent", flags)

	require.Error(t, err, "should error when pack not found")
	require.Contains(t, err.Error(), "not found", "error should mention pack not found")
}

// TestWorkflowUpdate_NoPacksToUpdate returns success with message when --all used but no packs installed.
func TestWorkflowUpdate_NoPacksToUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	cfg := &Config{}
	flags := workflowUpdateFlags{all: true}

	cmd := &cobra.Command{}
	cmd.SetOut(bytes.NewBuffer(nil))
	cmd.SetErr(bytes.NewBuffer(nil))

	err = runWorkflowUpdate(cmd, cfg, "", flags)

	require.NoError(t, err, "should succeed when no packs to update")
}

// TestWorkflowUpdate_GitHubAPIError returns error when API call fails.
func TestWorkflowUpdate_GitHubAPIError(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.Mkdir(projDir, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(projDir, ".awf"), 0o755))

	// Create pack directory with state pointing to invalid repo
	packDir := filepath.Join(projDir, ".awf", "workflow-packs", "speckit")
	require.NoError(t, os.MkdirAll(packDir, 0o755))

	stateContent := `{
		"name": "speckit",
		"enabled": true,
		"source_data": {
			"repository": "org/awf-workflow-speckit",
			"version": "1.0.0",
			"installed_at": "2025-01-01T00:00:00Z",
			"updated_at": "2025-01-01T00:00:00Z"
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(stateContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte("name: speckit\nversion: 1.0.0\n"), 0o644))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	// Mock GitHub API that always returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cfg := &Config{}
	flags := workflowUpdateFlags{all: false}

	cmd := &cobra.Command{}
	cmd.SetOut(bytes.NewBuffer(nil))
	cmd.SetErr(bytes.NewBuffer(nil))

	err = runWorkflowUpdate(cmd, cfg, "speckit", flags)

	require.Error(t, err, "should error when GitHub API call fails")
}

// TestEmitPluginWarnings_NilService_AllWarned verifies all plugins are warned about when pluginSvc is nil.
func TestEmitPluginWarnings_NilService_AllWarned(t *testing.T) {
	packDir := t.TempDir()

	manifestContent := `name: test-pack
version: "1.0.0"
description: Test pack
author: test
license: MIT
awf_version: ">=0.5.0"
plugins:
  github-plugin: ">=1.0.0"
  api-plugin: ">=2.0.0"
workflows:
  - test
`

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifestContent), 0o644))

	cmd, errBuf := newTestCommand(t)
	emitPluginWarnings(cmd, packDir, nil)
	stderr := errBuf.String()

	assert.Contains(t, stderr, "github-plugin", "should warn about github-plugin")
	assert.Contains(t, stderr, "api-plugin", "should warn about api-plugin")
	assert.Contains(t, stderr, ">=1.0.0", "should include version constraint for github-plugin")
	assert.Contains(t, stderr, ">=2.0.0", "should include version constraint for api-plugin")
}

// TestEmitPluginWarnings_NoInstalledPlugins_AllWarned verifies all plugins are warned about when none are installed.
func TestEmitPluginWarnings_NoInstalledPlugins_AllWarned(t *testing.T) {
	packDir := t.TempDir()

	manifestContent := `name: test-pack
version: "1.0.0"
description: Test pack
author: test
license: MIT
awf_version: ">=0.5.0"
plugins:
  custom-plugin: ">=1.5.0"
workflows:
  - test
`

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifestContent), 0o644))

	pluginSvc := application.NewPluginService(nil, nil, nil)

	cmd, errBuf := newTestCommand(t)
	emitPluginWarnings(cmd, packDir, pluginSvc)
	stderr := errBuf.String()

	assert.Contains(t, stderr, "custom-plugin", "should warn about custom-plugin when not installed")
}

// TestEmitPluginWarnings_SomeInstalledPlugins_OnlyUninstalledWarned verifies only uninstalled plugins are warned about.
func TestEmitPluginWarnings_SomeInstalledPlugins_OnlyUninstalledWarned(t *testing.T) {
	packDir := t.TempDir()

	manifestContent := `name: test-pack
version: "1.0.0"
description: Test pack
author: test
license: MIT
awf_version: ">=0.5.0"
plugins:
  installed-plugin: ">=1.0.0"
  missing-plugin: ">=2.0.0"
workflows:
  - test
`

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifestContent), 0o644))

	pluginSvc := application.NewPluginService(nil, nil, nil)
	pluginSvc.RegisterBuiltin("installed-plugin", "Installed plugin", "1.0.0", []string{})

	cmd, errBuf := newTestCommand(t)
	emitPluginWarnings(cmd, packDir, pluginSvc)
	stderr := errBuf.String()

	assert.NotContains(t, stderr, "installed-plugin", "should NOT warn about installed-plugin")
	assert.Contains(t, stderr, "missing-plugin", "should warn about missing-plugin")
}

// TestEmitPluginWarnings_AllPluginsInstalled_NoWarnings verifies no warnings when all plugins are installed.
func TestEmitPluginWarnings_AllPluginsInstalled_NoWarnings(t *testing.T) {
	packDir := t.TempDir()

	manifestContent := `name: test-pack
version: "1.0.0"
description: Test pack
author: test
license: MIT
awf_version: ">=0.5.0"
plugins:
  plugin-a: ">=1.0.0"
  plugin-b: ">=2.0.0"
workflows:
  - test
`

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifestContent), 0o644))

	pluginSvc := application.NewPluginService(nil, nil, nil)
	pluginSvc.RegisterBuiltin("plugin-a", "Plugin A", "1.0.0", []string{})
	pluginSvc.RegisterBuiltin("plugin-b", "Plugin B", "2.0.0", []string{})

	cmd, errBuf := newTestCommand(t)
	emitPluginWarnings(cmd, packDir, pluginSvc)
	stderr := errBuf.String()

	assert.Empty(t, stderr, "should emit no warnings when all plugins are installed")
}

// TestEmitPluginWarnings_MissingManifest_Silent verifies function returns silently when manifest is missing.
func TestEmitPluginWarnings_MissingManifest_Silent(t *testing.T) {
	packDir := t.TempDir()

	cmd, errBuf := newTestCommand(t)
	emitPluginWarnings(cmd, packDir, nil)
	stderr := errBuf.String()

	assert.Empty(t, stderr, "should emit no warnings when manifest is missing")
}

// TestEmitPluginWarnings_InvalidManifest_Silent verifies function returns silently when manifest is invalid.
func TestEmitPluginWarnings_InvalidManifest_Silent(t *testing.T) {
	packDir := t.TempDir()

	invalidContent := `invalid: yaml: content: [
This is not valid YAML
`

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(invalidContent), 0o644))

	cmd, errBuf := newTestCommand(t)
	emitPluginWarnings(cmd, packDir, nil)
	stderr := errBuf.String()

	assert.Empty(t, stderr, "should emit no warnings when manifest is invalid")
}

// TestEmitPluginWarnings_NoPluginsInManifest_Silent verifies function returns silently when manifest has no plugins.
func TestEmitPluginWarnings_NoPluginsInManifest_Silent(t *testing.T) {
	packDir := t.TempDir()

	manifestContent := `name: test-pack
version: "1.0.0"
description: Test pack with no plugins
author: test
license: MIT
awf_version: ">=0.5.0"
workflows:
  - test
`

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifestContent), 0o644))

	cmd, errBuf := newTestCommand(t)
	emitPluginWarnings(cmd, packDir, nil)
	stderr := errBuf.String()

	assert.Empty(t, stderr, "should emit no warnings when manifest has no plugins")
}

// newTestCommand creates a cobra.Command with a custom error stream for testing.
func newTestCommand(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&out)
	return cmd, &out
}

// TestFindPackDir_ShortAndLongName verifies findPackDir resolves both the exact name
// and the short name (strips awf-workflow- prefix) from a local project pack directory.
func TestFindPackDir_ShortAndLongName(t *testing.T) {
	projDir := t.TempDir()
	localPacksDir := filepath.Join(projDir, ".awf", "workflow-packs", "hello")
	require.NoError(t, os.MkdirAll(localPacksDir, 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	// Short name should find the directory.
	assert.NotEmpty(t, findPackDir("hello"), "short name should resolve to pack directory")

	// Long name (awf-workflow-hello) should also find it via prefix stripping.
	assert.NotEmpty(t, findPackDir("awf-workflow-hello"), "long name should resolve via prefix stripping")

	// Non-existent pack should return empty string.
	assert.Empty(t, findPackDir("nonexistent"), "unknown pack should return empty string")
}

// TestEffectiveCLIVersion_DevPrefix verifies effectiveCLIVersion returns "0.5.0"
// for any version string starting with "dev" and the verbatim version otherwise.
func TestEffectiveCLIVersion_DevPrefix(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"exact dev", "dev", "0.5.0"},
		{"dev with hash", "dev-abc123", "0.5.0"},
		{"dev dirty", "dev-abc123-dirty", "0.5.0"},
		{"real version", "1.2.3", "1.2.3"},
		{"pre-release", "1.0.0-rc1", "1.0.0-rc1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origVersion := Version
			Version = tt.version
			t.Cleanup(func() { Version = origVersion })

			assert.Equal(t, tt.want, effectiveCLIVersion())
		})
	}
}

// TestRunWorkflowList_TableOutput verifies runWorkflowList prints table headers and
// pack entries when at least one pack is installed in the local project directory.
func TestRunWorkflowList_TableOutput(t *testing.T) {
	projDir := t.TempDir()

	// Isolate XDG directories so the real user environment does not bleed into results.
	xdgIsolated := filepath.Join(projDir, "xdg")
	require.NoError(t, os.MkdirAll(xdgIsolated, 0o755))
	t.Setenv("XDG_CONFIG_HOME", xdgIsolated)
	t.Setenv("XDG_DATA_HOME", xdgIsolated)

	packDir := filepath.Join(projDir, ".awf", "workflow-packs", "testpack")
	require.NoError(t, os.MkdirAll(packDir, 0o755))

	manifestContent := `name: testpack
version: "2.0.0"
description: Test pack
author: test
license: MIT
awf_version: ">=0.5.0"
workflows:
  - myworkflow
`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifestContent), 0o644))

	// Manifest validation requires the declared workflow file to exist inside workflows/.
	workflowsDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "myworkflow.yaml"), []byte("name: myworkflow\n"), 0o644))

	stateContent := `{
  "name": "testpack",
  "enabled": true,
  "source_data": {
    "repository": "org/awf-workflow-testpack",
    "version": "2.0.0",
    "installed_at": "2025-01-01T00:00:00Z",
    "updated_at": "2025-01-01T00:00:00Z"
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(stateContent), 0o644))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	var outBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&outBuf)
	cmd.SetErr(&bytes.Buffer{})

	cfg := &Config{}

	err = runWorkflowList(cmd, cfg)

	require.NoError(t, err)
	output := outBuf.String()
	assert.Contains(t, output, "NAME", "output should include NAME column header")
	assert.Contains(t, output, "VERSION", "output should include VERSION column header")
	assert.Contains(t, output, "ENABLED", "output should include ENABLED column header")
	assert.Contains(t, output, "testpack", "output should include installed pack name")
	assert.Contains(t, output, "2.0.0", "output should include pack version")
}

// TestRunWorkflowRemove_ShortAndLongName verifies runWorkflowRemove succeeds for
// a pack installed under the short name when called with both the short and long form.
func TestRunWorkflowRemove_ShortAndLongName(t *testing.T) {
	tests := []struct {
		name string
		arg  string // argument passed to runWorkflowRemove
	}{
		{"short name", "hello"},
		{"long name", "awf-workflow-hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projDir := t.TempDir()
			packDir := filepath.Join(projDir, ".awf", "workflow-packs", "hello")
			require.NoError(t, os.MkdirAll(packDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte("name: hello\n"), 0o644))

			origWd, err := os.Getwd()
			require.NoError(t, err)
			t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

			require.NoError(t, os.Chdir(projDir))

			var outBuf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&outBuf)
			cmd.SetErr(&bytes.Buffer{})
			cfg := &Config{}

			err = runWorkflowRemove(cmd, cfg, tt.arg)

			require.NoError(t, err, "remove should succeed for %q", tt.arg)
			assert.NoDirExists(t, packDir, "pack directory should be deleted after remove")
		})
	}
}

// TestRunWorkflowInstall_ProgressMessages verifies that runWorkflowInstall emits
// expected progress messages ("Fetching releases...", "Resolved version:", "Downloading...",
// "Installed workflow pack...") to stdout when the install succeeds.
func TestRunWorkflowInstall_ProgressMessages(t *testing.T) {
	projDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projDir, ".awf"), 0o755))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	tarball := createTestWorkflowTarballForUnit(t, "msgpack")
	checksumData := createTestWorkflowSHA256FileForUnit(t, tarball, "awf-workflow-msgpack_1.0.0.tar.gz")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases"):
			releases := []map[string]interface{}{
				{
					"tag_name": "v1.0.0",
					"assets": []map[string]interface{}{
						{
							"name":                 "awf-workflow-msgpack_1.0.0.tar.gz",
							"browser_download_url": "http://" + r.Host + "/downloads/awf-workflow-msgpack_1.0.0.tar.gz",
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(releases)
		case strings.Contains(r.URL.Path, "/downloads/awf-workflow-msgpack"):
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(tarball)
		case strings.Contains(r.URL.Path, "/downloads/checksums.txt"):
			_, _ = w.Write([]byte(checksumData))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	var outBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&outBuf)
	cmd.SetErr(&bytes.Buffer{})

	cfg := &Config{}
	flags := workflowInstallFlags{version: "1.0.0", global: false, force: false}

	err = runWorkflowInstall(cmd, cfg, "org/awf-workflow-msgpack", flags)

	require.NoError(t, err)
	output := outBuf.String()
	assert.Contains(t, output, "Fetching releases", "should output fetching releases message")
	assert.Contains(t, output, "Resolved version:", "should output resolved version message")
	assert.Contains(t, output, "Downloading", "should output downloading message")
	assert.Contains(t, output, "Installed workflow pack", "should output installed message")
}

// TestRunWorkflowList_EmptyShowsNoMessage verifies that runWorkflowList outputs
// "No workflow packs installed." when no packs exist and no loose workflows exist.
func TestRunWorkflowList_EmptyShowsNoMessage(t *testing.T) {
	projDir := t.TempDir()

	// Create .awf directory without workflow-packs or workflows subdirs.
	require.NoError(t, os.MkdirAll(filepath.Join(projDir, ".awf"), 0o755))

	// Isolate both XDG directories so no real user workflows or packs appear in results.
	xdgIsolated := filepath.Join(projDir, "xdg")
	require.NoError(t, os.MkdirAll(xdgIsolated, 0o755))
	t.Setenv("XDG_CONFIG_HOME", xdgIsolated)
	t.Setenv("XDG_DATA_HOME", xdgIsolated)

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore working directory in cleanup

	require.NoError(t, os.Chdir(projDir))

	var outBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&outBuf)
	cmd.SetErr(&bytes.Buffer{})

	cfg := &Config{}

	err = runWorkflowList(cmd, cfg)

	require.NoError(t, err)
	assert.Contains(t, outBuf.String(), "No workflow packs installed.", "empty state should show 'No workflow packs installed.'")
}
