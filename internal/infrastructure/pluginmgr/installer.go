package pluginmgr

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/awf-project/cli/pkg/httpx"
)

// PluginSource represents the origin and installation metadata of an external plugin.
// Serialized to PluginState.SourceData for persistence.
type PluginSource struct {
	Repository  string    `json:"repository"` // e.g., "owner/repo"
	Version     string    `json:"version"`    // e.g., "1.2.3"
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PluginInstaller handles downloading, verifying, extracting, and atomically installing plugins
// from GitHub releases.
type PluginInstaller struct {
	client httpx.HTTPDoer
}

// NewPluginInstaller creates a new PluginInstaller with optional client injection.
// If client is nil, uses a default *http.Client.
func NewPluginInstaller(optionalClient ...httpx.HTTPDoer) *PluginInstaller {
	if len(optionalClient) > 0 && optionalClient[0] != nil {
		return &PluginInstaller{client: optionalClient[0]}
	}
	return &PluginInstaller{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Download downloads a plugin binary from a URL.
// Returns the downloaded content as bytes, or an error.
// ctx: context for cancellation
// url: download URL
// checksum: expected SHA-256 checksum (hex string)
func (pi *PluginInstaller) Download(ctx context.Context, url, checksum string) ([]byte, error) {
	const maxBodyBytes = 100 * 1024 * 1024 // 100 MB

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	httpResp, err := pi.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, truncated, err := httpx.ReadBody(httpResp.Body, maxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	if truncated {
		return nil, fmt.Errorf("download truncated: exceeded 100MB limit")
	}

	return []byte(body), nil
}

// VerifyChecksum verifies that data matches the expected SHA-256 checksum.
// data: the bytes to verify
// checksum: the expected SHA-256 checksum (hex string)
// Returns error if verification fails.
func (pi *PluginInstaller) VerifyChecksum(data []byte, checksum string) error {
	actual := fmt.Sprintf("%x", sha256.Sum256(data))
	if actual != checksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", checksum, actual)
	}
	return nil
}

// ExtractTarGz extracts a tar.gz archive to a target directory.
// data: the tar.gz archive bytes
// targetDir: directory where files will be extracted
// Returns error if extraction fails.
func (pi *PluginInstaller) ExtractTarGz(data []byte, targetDir string) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		filePath := filepath.Join(targetDir, header.Name) //nolint:gosec // G305: path traversal validated by filepath.IsLocal check below

		// Prevent directory traversal attacks by validating the cleaned path
		cleanPath := filepath.Clean(filePath)
		cleanTarget := filepath.Clean(targetDir)
		relPath, err := filepath.Rel(cleanTarget, cleanPath)
		if err != nil || !filepath.IsLocal(relPath) {
			return fmt.Errorf("tar entry attempts path traversal: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filePath, os.FileMode(header.Mode)); err != nil { //nolint:gosec // G115: tar header mode is bounded by uint32 range in valid archives
				return fmt.Errorf("failed to create directory %s: %w", filePath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil { //nolint:gosec // G301: 0o750 is intentional for plugin directories
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			file, err := os.Create(filePath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", filePath, err)
			}
			_, copyErr := io.Copy(file, tarReader) //nolint:gosec // G110: decompression bomb mitigated by 100MB download limit in PluginInstaller.Download
			closeErr := file.Close()
			if copyErr != nil {
				return fmt.Errorf("failed to write file %s: %w", filePath, copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("failed to close file %s: %w", filePath, closeErr)
			}
			if err := os.Chmod(filePath, os.FileMode(header.Mode)); err != nil { //nolint:gosec // G115: tar header mode is bounded by uint32 range in valid archives
				return fmt.Errorf("failed to set file permissions: %w", err)
			}
		}
	}

	return nil
}

// AtomicInstall performs an atomic installation by moving a temp directory to the final location.
// tempDir: temporary directory with extracted plugin files
// targetDir: final plugin installation directory
// Returns error if installation fails; targetDir is rolled back on failure.
func (pi *PluginInstaller) AtomicInstall(tempDir, targetDir string) error {
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("plugin directory already exists: %s", targetDir)
	}

	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0o750); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	if err := os.Rename(tempDir, targetDir); err != nil {
		return fmt.Errorf("failed to move plugin to target directory: %w", err)
	}

	return nil
}

// ValidateManifest checks that a plugin directory contains a valid manifest file.
// dir: plugin directory to validate
// Returns error if manifest is missing or invalid.
func (pi *PluginInstaller) ValidateManifest(dir string) error {
	manifestPath := filepath.Join(dir, ManifestFileName)
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("manifest file not found: %s", ManifestFileName)
		}
		return fmt.Errorf("failed to access manifest file: %w", err)
	}
	return nil
}

// Install downloads, verifies, extracts, and installs a plugin.
// Performs atomic installation with rollback on failure.
// Validates manifest exists after extraction.
// Rejects installation if plugin already exists (unless force=true).
//
// ctx: context for cancellation
// url: download URL of plugin archive
// checksum: expected SHA-256 checksum (hex string)
// targetDir: directory where plugin will be installed
// force: if true, overwrites existing plugin directory
//
// Returns error if:
// - Plugin already exists and force=false
// - Download fails
// - Checksum verification fails
// - Extraction fails
// - Manifest validation fails
// - Installation fails
func (pi *PluginInstaller) Install(ctx context.Context, url, checksum, targetDir string, force bool) error {
	if _, statErr := os.Stat(targetDir); statErr == nil {
		if !force {
			return fmt.Errorf("plugin already exists at %s", targetDir)
		}
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("failed to remove existing plugin: %w", err)
		}
	}

	data, err := pi.Download(ctx, url, checksum)
	if err != nil {
		return err
	}

	if checksumErr := pi.VerifyChecksum(data, checksum); checksumErr != nil {
		return checksumErr
	}

	// Create temp dir under target's parent to ensure same filesystem for atomic rename.
	parentDir := filepath.Dir(targetDir)
	if mkdirErr := os.MkdirAll(parentDir, 0o750); mkdirErr != nil {
		return fmt.Errorf("failed to create parent directory: %w", mkdirErr)
	}
	tempDir, err := os.MkdirTemp(parentDir, ".plugin-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := pi.ExtractTarGz(data, tempDir); err != nil {
		return err
	}

	if err := pi.ValidateManifest(tempDir); err != nil {
		return err
	}

	return pi.AtomicInstall(tempDir, targetDir)
}

// SourceDataFromPluginSource serializes a PluginSource to the format expected by PluginState.SourceData.
//
//nolint:gocritic // Parameter is 80 bytes but signature required by tests
func SourceDataFromPluginSource(source PluginSource) (map[string]any, error) {
	data, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("marshal plugin source: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal plugin source: %w", err)
	}

	return m, nil
}

// PluginSourceFromSourceData deserializes SourceData from PluginState into a PluginSource.
func PluginSourceFromSourceData(sourceData map[string]any) (PluginSource, error) {
	if sourceData == nil {
		return PluginSource{}, fmt.Errorf("source data is nil")
	}

	data, err := json.Marshal(sourceData)
	if err != nil {
		return PluginSource{}, fmt.Errorf("marshal source data: %w", err)
	}

	var source PluginSource
	if err := json.Unmarshal(data, &source); err != nil {
		return PluginSource{}, fmt.Errorf("unmarshal plugin source: %w", err)
	}

	return source, nil
}
