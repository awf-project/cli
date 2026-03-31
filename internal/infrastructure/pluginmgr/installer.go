package pluginmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/awf-project/cli/pkg/httpx"
	"github.com/awf-project/cli/pkg/registry"
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

	data, err := registry.Download(ctx, pi.client, url)
	if err != nil {
		return fmt.Errorf("download plugin: %w", err)
	}

	if checksumErr := registry.VerifyChecksum(data, checksum); checksumErr != nil {
		return fmt.Errorf("verify checksum: %w", checksumErr)
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

	if err := registry.ExtractTarGz(data, tempDir); err != nil {
		return fmt.Errorf("extract tar archive: %w", err)
	}

	if err := pi.ValidateManifest(tempDir); err != nil {
		return err
	}

	return pi.AtomicInstall(tempDir, targetDir)
}

// roundTripJSON converts a value to another type via JSON marshal/unmarshal round-trip.
func roundTripJSON[From, To any](from From) (To, error) {
	var to To
	data, err := json.Marshal(from)
	if err != nil {
		return to, fmt.Errorf("marshal: %w", err)
	}
	if err := json.Unmarshal(data, &to); err != nil {
		return to, fmt.Errorf("unmarshal: %w", err)
	}
	return to, nil
}

// SourceDataFromPluginSource serializes a PluginSource to the format expected by PluginState.SourceData.
//
//nolint:gocritic // Parameter is 80 bytes but signature required by tests
func SourceDataFromPluginSource(source PluginSource) (map[string]any, error) {
	m, err := roundTripJSON[PluginSource, map[string]any](source)
	if err != nil {
		return nil, fmt.Errorf("convert plugin source to map: %w", err)
	}
	return m, nil
}

// PluginSourceFromSourceData deserializes SourceData from PluginState into a PluginSource.
func PluginSourceFromSourceData(sourceData map[string]any) (PluginSource, error) {
	if sourceData == nil {
		return PluginSource{}, fmt.Errorf("source data is nil")
	}
	source, err := roundTripJSON[map[string]any, PluginSource](sourceData)
	if err != nil {
		return PluginSource{}, fmt.Errorf("convert map to plugin source: %w", err)
	}
	return source, nil
}
