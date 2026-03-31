package pluginmgr_test

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

	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
)

func TestNewPluginInstaller(t *testing.T) {
	installer := pluginmgr.NewPluginInstaller()
	require.NotNil(t, installer)
}

func TestValidateManifest(t *testing.T) {
	t.Run("manifest exists", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, pluginmgr.ManifestFileName)
		require.NoError(t, os.WriteFile(manifestPath, []byte("name: test"), 0o644))

		installer := pluginmgr.NewPluginInstaller()
		err := installer.ValidateManifest(dir)
		assert.NoError(t, err)
	})

	t.Run("manifest missing", func(t *testing.T) {
		dir := t.TempDir()
		installer := pluginmgr.NewPluginInstaller()

		err := installer.ValidateManifest(dir)
		assert.Error(t, err)
	})
}

func TestAtomicInstall(t *testing.T) {
	t.Run("successful atomic install", func(t *testing.T) {
		tempDir := t.TempDir()
		manifestPath := filepath.Join(tempDir, pluginmgr.ManifestFileName)
		require.NoError(t, os.WriteFile(manifestPath, []byte("name: test"), 0o644))

		parentDir := t.TempDir()
		targetDir := filepath.Join(parentDir, "new-plugin")

		installer := pluginmgr.NewPluginInstaller()
		err := installer.AtomicInstall(tempDir, targetDir)
		assert.NoError(t, err)
		assert.DirExists(t, targetDir)
	})

	t.Run("target already exists without force", func(t *testing.T) {
		tempDir := t.TempDir()
		manifestPath := filepath.Join(tempDir, pluginmgr.ManifestFileName)
		require.NoError(t, os.WriteFile(manifestPath, []byte("name: test"), 0o644))

		targetDir := t.TempDir()
		existingManifest := filepath.Join(targetDir, pluginmgr.ManifestFileName)
		require.NoError(t, os.WriteFile(existingManifest, []byte("name: existing"), 0o644))

		installer := pluginmgr.NewPluginInstaller()
		err := installer.AtomicInstall(tempDir, targetDir)
		assert.Error(t, err)
	})
}

func TestInstall(t *testing.T) {
	t.Run("complete install workflow", func(t *testing.T) {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)

		manifest := []byte("name: test-plugin\nversion: 1.0.0\n")
		header := &tar.Header{
			Name: pluginmgr.ManifestFileName,
			Size: int64(len(manifest)),
			Mode: 0o644,
		}
		require.NoError(t, tw.WriteHeader(header))
		_, err := tw.Write(manifest)
		require.NoError(t, err)

		require.NoError(t, tw.Close())
		require.NoError(t, gz.Close())

		tarData := buf.Bytes()
		checksum := fmt.Sprintf("%x", sha256.Sum256(tarData))

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(tarData)
		}))
		defer server.Close()

		ctx := context.Background()
		targetDir := filepath.Join(t.TempDir(), "plugins", "test-plugin")

		installer := pluginmgr.NewPluginInstaller()
		err = installer.Install(ctx, server.URL+"/plugin.tar.gz", checksum, targetDir, false)
		assert.NoError(t, err)
		assert.DirExists(t, targetDir)
	})

	t.Run("plugin already exists without force", func(t *testing.T) {
		targetDir := t.TempDir()
		manifestPath := filepath.Join(targetDir, pluginmgr.ManifestFileName)
		require.NoError(t, os.WriteFile(manifestPath, []byte("name: existing"), 0o644))

		ctx := context.Background()
		installer := pluginmgr.NewPluginInstaller()
		err := installer.Install(ctx, "https://example.com/plugin.tar.gz", "checksum", targetDir, false)
		assert.Error(t, err)
	})

	t.Run("plugin already exists with force", func(t *testing.T) {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)

		manifest := []byte("name: updated-plugin\nversion: 2.0.0\n")
		header := &tar.Header{
			Name: pluginmgr.ManifestFileName,
			Size: int64(len(manifest)),
			Mode: 0o644,
		}
		require.NoError(t, tw.WriteHeader(header))
		_, err := tw.Write(manifest)
		require.NoError(t, err)

		require.NoError(t, tw.Close())
		require.NoError(t, gz.Close())

		targetDir := t.TempDir()
		existingManifest := filepath.Join(targetDir, pluginmgr.ManifestFileName)
		require.NoError(t, os.WriteFile(existingManifest, []byte("name: old"), 0o644))

		tarData := buf.Bytes()
		checksum := fmt.Sprintf("%x", sha256.Sum256(tarData))

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(tarData)
		}))
		defer server.Close()

		ctx := context.Background()
		installer := pluginmgr.NewPluginInstaller()
		err = installer.Install(ctx, server.URL+"/plugin.tar.gz", checksum, targetDir, true)
		assert.NoError(t, err)
	})

	t.Run("checksum mismatch aborts", func(t *testing.T) {
		ctx := context.Background()
		targetDir := filepath.Join(t.TempDir(), "plugins", "test-plugin")

		installer := pluginmgr.NewPluginInstaller()
		err := installer.Install(ctx, "https://example.com/plugin.tar.gz", "invalid-checksum", targetDir, false)
		assert.Error(t, err)
	})
}

func TestSourceDataFromPluginSource(t *testing.T) {
	source := pluginmgr.PluginSource{
		Repository:  "owner/repo",
		Version:     "1.2.3",
		InstalledAt: time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2024, 3, 16, 14, 45, 0, 0, time.UTC),
	}

	data, err := pluginmgr.SourceDataFromPluginSource(source)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Equal(t, "owner/repo", data["repository"])
	assert.Equal(t, "1.2.3", data["version"])
}

func TestPluginSourceFromSourceData(t *testing.T) {
	t.Run("valid source data", func(t *testing.T) {
		sourceData := map[string]any{
			"repository":   "owner/repo",
			"version":      "1.2.3",
			"installed_at": "2024-03-15T10:30:00Z",
			"updated_at":   "2024-03-16T14:45:00Z",
		}

		source, err := pluginmgr.PluginSourceFromSourceData(sourceData)
		assert.NoError(t, err)
		assert.Equal(t, "owner/repo", source.Repository)
		assert.Equal(t, "1.2.3", source.Version)
	})

	t.Run("nil source data", func(t *testing.T) {
		_, err := pluginmgr.PluginSourceFromSourceData(nil)
		assert.Error(t, err)
	})
}

func TestSourceDataRoundTrip(t *testing.T) {
	original := pluginmgr.PluginSource{
		Repository:  "myorg/awf-plugin-jira",
		Version:     "2.1.0",
		InstalledAt: time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2024, 3, 16, 14, 45, 0, 0, time.UTC),
	}

	data, err := pluginmgr.SourceDataFromPluginSource(original)
	require.NoError(t, err)

	restored, err := pluginmgr.PluginSourceFromSourceData(data)
	require.NoError(t, err)

	assert.Equal(t, original.Repository, restored.Repository)
	assert.Equal(t, original.Version, restored.Version)
	assert.Equal(t, original.InstalledAt.Format(time.RFC3339), restored.InstalledAt.Format(time.RFC3339))
	assert.Equal(t, original.UpdatedAt.Format(time.RFC3339), restored.UpdatedAt.Format(time.RFC3339))
}
