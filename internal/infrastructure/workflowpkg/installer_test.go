package workflowpkg_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
)

// createTestPackArchive creates a valid tar.gz archive with manifest and workflows directory.
func createTestPackArchive(t *testing.T, name, version string, awfVersion string, workflows []string) []byte { //nolint:gocritic // paramTypeCombine: signature kept for clarity
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// Write manifest.yaml
	manifestContent := fmt.Sprintf(`name: %s
version: "%s"
description: Test pack
author: Test Author
awf_version: "%s"
workflows:
`, name, version, awfVersion)
	for _, w := range workflows {
		manifestContent += fmt.Sprintf("  - %s\n", w)
	}

	header := &tar.Header{
		Name: "manifest.yaml",
		Size: int64(len(manifestContent)),
		Mode: 0o644,
	}
	require.NoError(t, tw.WriteHeader(header))
	_, err := tw.Write([]byte(manifestContent))
	require.NoError(t, err)

	// Write workflows directory with yaml files
	for _, wf := range workflows {
		filename := fmt.Sprintf("workflows/%s.yaml", wf)
		content := fmt.Sprintf("name: %s\n", wf)
		header := &tar.Header{
			Name: filename,
			Size: int64(len(content)),
			Mode: 0o644,
		}
		require.NoError(t, tw.WriteHeader(header))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	return buf.Bytes()
}

func TestPackInstaller_Install_HappyPath(t *testing.T) {
	archiveData := createTestPackArchive(t, "test-pack", "1.0.0", ">=0.1.0", []string{"workflow1", "workflow2"})
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveData))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	ctx := context.Background()

	installer := workflowpkg.NewPackInstaller("0.5.0")
	source := workflowpkg.PackSource{
		Repository:  "owner/awf-workflow-test-pack",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := installer.Install(ctx, server.URL+"/pack.tar.gz", checksum, targetDir, false, source)

	assert.NoError(t, err)
	assert.DirExists(t, targetDir)
	assert.FileExists(t, filepath.Join(targetDir, "manifest.yaml"))
	assert.FileExists(t, filepath.Join(targetDir, "workflows", "workflow1.yaml"))
	assert.FileExists(t, filepath.Join(targetDir, "workflows", "workflow2.yaml"))

	// Verify state.json was written
	stateFile := filepath.Join(targetDir, "state.json")
	assert.FileExists(t, stateFile)
}

func TestPackInstaller_Install_ChecksumMismatch(t *testing.T) {
	archiveData := createTestPackArchive(t, "test-pack", "1.0.0", ">=0.1.0", []string{"workflow1"})
	wrongChecksum := fmt.Sprintf("%x", sha256.Sum256([]byte("different-content")))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	ctx := context.Background()

	installer := workflowpkg.NewPackInstaller("0.5.0")
	source := workflowpkg.PackSource{
		Repository: "owner/awf-workflow-test-pack",
		Version:    "1.0.0",
	}

	err := installer.Install(ctx, server.URL+"/pack.tar.gz", wrongChecksum, targetDir, false, source)

	assert.Error(t, err)
	assert.NoDirExists(t, targetDir)
}

func TestPackInstaller_Install_InvalidManifest(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// Write only a manifest with missing required fields
	invalidManifest := "name: test-pack\n" // Missing version, description, author, awf_version, workflows
	header := &tar.Header{
		Name: "manifest.yaml",
		Size: int64(len(invalidManifest)),
		Mode: 0o644,
	}
	require.NoError(t, tw.WriteHeader(header))
	_, err := tw.Write([]byte(invalidManifest))
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	archiveData := buf.Bytes()
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveData))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	ctx := context.Background()

	installer := workflowpkg.NewPackInstaller("0.5.0")
	source := workflowpkg.PackSource{
		Repository: "owner/awf-workflow-test-pack",
		Version:    "1.0.0",
	}

	err = installer.Install(ctx, server.URL+"/pack.tar.gz", checksum, targetDir, false, source)

	assert.Error(t, err)
	assert.NoDirExists(t, targetDir)
}

func TestPackInstaller_Install_AWFVersionIncompatible(t *testing.T) {
	// Archive requires AWF >= 1.0.0, but CLI is 0.5.0
	archiveData := createTestPackArchive(t, "test-pack", "1.0.0", ">=1.0.0", []string{"workflow1"})
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveData))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	ctx := context.Background()

	installer := workflowpkg.NewPackInstaller("0.5.0") // CLI version too old
	source := workflowpkg.PackSource{
		Repository: "owner/awf-workflow-test-pack",
		Version:    "1.0.0",
	}

	err := installer.Install(ctx, server.URL+"/pack.tar.gz", checksum, targetDir, false, source)

	assert.Error(t, err)
	assert.NoDirExists(t, targetDir)
}

func TestPackInstaller_Install_DirectoryExists_WithoutForce(t *testing.T) {
	archiveData := createTestPackArchive(t, "test-pack", "1.0.0", ">=0.1.0", []string{"workflow1"})
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveData))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	require.NoError(t, os.MkdirAll(targetDir, 0o750))
	existingFile := filepath.Join(targetDir, "manifest.yaml")
	require.NoError(t, os.WriteFile(existingFile, []byte("name: existing"), 0o644))

	ctx := context.Background()

	installer := workflowpkg.NewPackInstaller("0.5.0")
	source := workflowpkg.PackSource{
		Repository: "owner/awf-workflow-test-pack",
		Version:    "1.0.0",
	}

	err := installer.Install(ctx, server.URL+"/pack.tar.gz", checksum, targetDir, false, source)

	assert.Error(t, err)
}

func TestPackInstaller_Install_DirectoryExists_WithForce(t *testing.T) {
	archiveData := createTestPackArchive(t, "test-pack", "2.0.0", ">=0.1.0", []string{"new-workflow"})
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveData))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	require.NoError(t, os.MkdirAll(targetDir, 0o750))
	oldFile := filepath.Join(targetDir, "old-file.txt")
	require.NoError(t, os.WriteFile(oldFile, []byte("old content"), 0o644))

	ctx := context.Background()

	installer := workflowpkg.NewPackInstaller("0.5.0")
	source := workflowpkg.PackSource{
		Repository: "owner/awf-workflow-test-pack",
		Version:    "2.0.0",
	}

	err := installer.Install(ctx, server.URL+"/pack.tar.gz", checksum, targetDir, true, source)

	assert.NoError(t, err)
	assert.DirExists(t, targetDir)
	assert.FileExists(t, filepath.Join(targetDir, "manifest.yaml"))
	assert.NoFileExists(t, oldFile)
}

func TestPackInstaller_Install_NetworkError(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	ctx := context.Background()

	installer := workflowpkg.NewPackInstaller("0.5.0")
	source := workflowpkg.PackSource{
		Repository: "owner/awf-workflow-test-pack",
		Version:    "1.0.0",
	}

	// Use invalid URL that will fail
	err := installer.Install(ctx, "http://invalid-domain-that-does-not-exist.com/pack.tar.gz", "abc123", targetDir, false, source)

	assert.Error(t, err)
	assert.NoDirExists(t, targetDir)
}

func TestPackInstaller_Install_ContextCancellation(t *testing.T) {
	archiveData := createTestPackArchive(t, "test-pack", "1.0.0", ">=0.1.0", []string{"workflow1"})
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveData))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	installer := workflowpkg.NewPackInstaller("0.5.0")
	source := workflowpkg.PackSource{
		Repository: "owner/awf-workflow-test-pack",
		Version:    "1.0.0",
	}

	err := installer.Install(ctx, server.URL+"/pack.tar.gz", checksum, targetDir, false, source)

	assert.Error(t, err)
	assert.NoDirExists(t, targetDir)
}

func TestNewPackInstaller_WithOptionalClient(t *testing.T) {
	mockClient := &mockHTTPClient{}

	installer := workflowpkg.NewPackInstaller("0.5.0", mockClient)

	assert.NotNil(t, installer)
}

func TestNewPackInstaller_WithoutOptionalClient(t *testing.T) {
	installer := workflowpkg.NewPackInstaller("0.5.0")

	assert.NotNil(t, installer)
}

type mockHTTPClient struct{}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("mock error")
}

func TestPackInstaller_Update_WritesStateJsonWithVersion(t *testing.T) {
	archiveData := createTestPackArchive(t, "test-pack", "2.0.0", ">=0.1.0", []string{"workflow1"})
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveData))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	ctx := context.Background()

	installer := workflowpkg.NewPackInstaller("0.5.0")
	source := workflowpkg.PackSource{
		Repository:  "owner/awf-workflow-test-pack",
		Version:     "2.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := installer.Install(ctx, server.URL+"/pack.tar.gz", checksum, targetDir, true, source)

	require.NoError(t, err)
	assert.DirExists(t, targetDir)

	stateFile := filepath.Join(targetDir, "state.json")
	require.FileExists(t, stateFile)

	stateData, err := os.ReadFile(stateFile)
	require.NoError(t, err)

	var state workflowpkg.PackState
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	assert.Equal(t, "test-pack", state.Name)
	assert.True(t, state.Enabled)
	assert.NotNil(t, state.SourceData)

	recoveredSource, err := workflowpkg.PackSourceFromSourceData(state.SourceData)
	require.NoError(t, err)
	assert.Equal(t, "owner/awf-workflow-test-pack", recoveredSource.Repository)
	assert.Equal(t, "2.0.0", recoveredSource.Version)
}

func TestPackInstaller_Update_Force_ReplaceExistingPack(t *testing.T) {
	// Create initial archive version 1.0.0
	oldArchiveData := createTestPackArchive(t, "test-pack", "1.0.0", ">=0.1.0", []string{"old-workflow"})
	oldChecksum := fmt.Sprintf("%x", sha256.Sum256(oldArchiveData))

	// Create new archive version 2.0.0
	newArchiveData := createTestPackArchive(t, "test-pack", "2.0.0", ">=0.1.0", []string{"new-workflow"})
	newChecksum := fmt.Sprintf("%x", sha256.Sum256(newArchiveData))

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	ctx := context.Background()

	// Initial install with version 1.0.0
	oldServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(oldArchiveData)
	}))
	defer oldServer.Close()

	installer := workflowpkg.NewPackInstaller("0.5.0")
	source := workflowpkg.PackSource{
		Repository:  "owner/awf-workflow-test-pack",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := installer.Install(ctx, oldServer.URL+"/pack.tar.gz", oldChecksum, targetDir, false, source)
	require.NoError(t, err)

	oldState := filepath.Join(targetDir, "state.json")
	oldStateData, err := os.ReadFile(oldState)
	require.NoError(t, err)
	var oldPackState workflowpkg.PackState
	err = json.Unmarshal(oldStateData, &oldPackState)
	require.NoError(t, err)

	// Recover the original source to preserve installed_at timestamp
	oldSource, err := workflowpkg.PackSourceFromSourceData(oldPackState.SourceData)
	require.NoError(t, err)

	// Simulate update to version 2.0.0 with force=true
	newServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(newArchiveData)
	}))
	defer newServer.Close()

	updateTime := time.Now()
	newSource := workflowpkg.PackSource{
		Repository:  "owner/awf-workflow-test-pack",
		Version:     "2.0.0",
		InstalledAt: oldSource.InstalledAt,
		UpdatedAt:   updateTime,
	}

	err = installer.Install(ctx, newServer.URL+"/pack.tar.gz", newChecksum, targetDir, true, newSource)
	require.NoError(t, err)

	newStateFile := filepath.Join(targetDir, "state.json")
	require.FileExists(t, newStateFile)

	newStateData, err := os.ReadFile(newStateFile)
	require.NoError(t, err)

	var newPackState workflowpkg.PackState
	err = json.Unmarshal(newStateData, &newPackState)
	require.NoError(t, err)

	newSource2, err := workflowpkg.PackSourceFromSourceData(newPackState.SourceData)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", newSource2.Version)

	// Verify new workflow exists and old one doesn't
	assert.FileExists(t, filepath.Join(targetDir, "workflows", "new-workflow.yaml"))
	assert.NoFileExists(t, filepath.Join(targetDir, "workflows", "old-workflow.yaml"))
}

func TestPackInstaller_Update_ChecksumFailure_OriginalPreserved(t *testing.T) {
	// Create initial valid archive
	initialArchiveData := createTestPackArchive(t, "test-pack", "1.0.0", ">=0.1.0", []string{"workflow1"})
	initialChecksum := fmt.Sprintf("%x", sha256.Sum256(initialArchiveData))

	// Create different archive for the "update"
	updateArchiveData := createTestPackArchive(t, "test-pack", "2.0.0", ">=0.1.0", []string{"workflow2"})
	wrongChecksum := fmt.Sprintf("%x", sha256.Sum256([]byte("totally-different")))

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	ctx := context.Background()

	// Initial install
	initialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(initialArchiveData)
	}))
	defer initialServer.Close()

	installer := workflowpkg.NewPackInstaller("0.5.0")
	initialSource := workflowpkg.PackSource{
		Repository:  "owner/awf-workflow-test-pack",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := installer.Install(ctx, initialServer.URL+"/pack.tar.gz", initialChecksum, targetDir, false, initialSource)
	require.NoError(t, err)

	// Attempt update without force, so directory isn't removed before checksum check
	updateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(updateArchiveData)
	}))
	defer updateServer.Close()

	updateSource := workflowpkg.PackSource{
		Repository:  "owner/awf-workflow-test-pack",
		Version:     "2.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Without force=true, Install() will reject the existing directory, preserving the original
	err = installer.Install(ctx, updateServer.URL+"/pack.tar.gz", wrongChecksum, targetDir, false, updateSource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Verify original installation is still intact
	assert.FileExists(t, filepath.Join(targetDir, "workflows", "workflow1.yaml"))
	assert.NoFileExists(t, filepath.Join(targetDir, "workflows", "workflow2.yaml"))

	// Verify state.json still has original version
	stateFile := filepath.Join(targetDir, "state.json")
	require.FileExists(t, stateFile)

	stateData, err := os.ReadFile(stateFile)
	require.NoError(t, err)

	var state workflowpkg.PackState
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	source, err := workflowpkg.PackSourceFromSourceData(state.SourceData)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", source.Version)
}

func TestPackInstaller_Update_WithForce_ChecksumFailure_DirectoryRemoved(t *testing.T) {
	// When force=true and checksum fails, the directory is lost (removed before checksum check)
	// This tests that the Install function properly removes the directory before verification
	archiveData := createTestPackArchive(t, "test-pack", "1.0.0", ">=0.1.0", []string{"workflow1"})
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveData))
	wrongChecksum := fmt.Sprintf("%x", sha256.Sum256([]byte("different")))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	ctx := context.Background()

	installer := workflowpkg.NewPackInstaller("0.5.0")
	source := workflowpkg.PackSource{
		Repository:  "owner/awf-workflow-test-pack",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Initial install
	err := installer.Install(ctx, server.URL+"/pack.tar.gz", checksum, targetDir, false, source)
	require.NoError(t, err)
	require.DirExists(t, targetDir)

	// Attempt force update with wrong checksum - directory will be removed before checksum check fails
	err = installer.Install(ctx, server.URL+"/pack.tar.gz", wrongChecksum, targetDir, true, source)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum")

	// Directory should not exist after failed force update (it was removed before checksum check)
	assert.NoDirExists(t, targetDir)
}

func TestPackInstaller_StateJsonPersistence_SourceMetadata(t *testing.T) {
	archiveData := createTestPackArchive(t, "test-pack", "1.5.0", ">=0.1.0", []string{"workflow1", "workflow2"})
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveData))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	targetDir := filepath.Join(t.TempDir(), "workflow-packs", "test-pack")
	ctx := context.Background()

	installer := workflowpkg.NewPackInstaller("0.5.0")
	installedAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2024, 1, 20, 14, 45, 0, 0, time.UTC)

	source := workflowpkg.PackSource{
		Repository:  "github.com/owner/my-pack",
		Version:     "1.5.0",
		InstalledAt: installedAt,
		UpdatedAt:   updatedAt,
	}

	err := installer.Install(ctx, server.URL+"/pack.tar.gz", checksum, targetDir, false, source)
	require.NoError(t, err)

	stateFile := filepath.Join(targetDir, "state.json")
	stateData, err := os.ReadFile(stateFile)
	require.NoError(t, err)

	var state workflowpkg.PackState
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	recovered, err := workflowpkg.PackSourceFromSourceData(state.SourceData)
	require.NoError(t, err)

	assert.Equal(t, "github.com/owner/my-pack", recovered.Repository)
	assert.Equal(t, "1.5.0", recovered.Version)
	assert.Equal(t, installedAt.Unix(), recovered.InstalledAt.Unix())
	assert.Equal(t, updatedAt.Unix(), recovered.UpdatedAt.Unix())
}
