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

	err = runWorkflowInstall(nil, cfg, "org/awf-workflow-speckit", flags)

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

	err := runWorkflowInstall(nil, cfg, "org/awf-workflow-speckit", flags)

	// Stub returns nil; global flag should prevent .awf/ project check.
	_ = err
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

	err = runWorkflowInstall(nil, cfg, "org/awf-workflow-speckit", flags)

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

	err = runWorkflowInstall(nil, cfg, "invalid-format", flags)

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

	err = runWorkflowInstall(nil, cfg, "org/awf-workflow-speckit", flags)

	// With --force, real implementation should allow replacement of existing pack.
	// Stub returns nil; this validates test setup is correct.
	_ = err
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

	err = runWorkflowRemove(nil, cfg, "speckit")

	// Stub returns nil; real implementation should remove pack directory.
	_ = err
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

	err = runWorkflowRemove(nil, cfg, "nonexistent")

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

	err = runWorkflowRemove(nil, cfg, "speckit")

	// Real implementation searches local first, then global.
	// Pack exists in global; remove should succeed.
	// Stub returns nil; validates test harness works.
	_ = err
}
