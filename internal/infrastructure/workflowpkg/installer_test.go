package workflowpkg_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
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
