package workflowpkg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/awf-project/cli/pkg/httpx"
	"github.com/awf-project/cli/pkg/registry"
)

const ManifestFileName = "manifest.yaml"

// MaxManifestSize caps manifest.yaml reads at 1MB to prevent OOM from malicious packs.
const MaxManifestSize = 1 << 20

// readFileLimited reads a file up to maxBytes. Returns error if the file exceeds the limit.
func readFileLimited(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("file %s exceeds maximum size (%d bytes)", filepath.Base(path), maxBytes)
	}

	data, err := io.ReadAll(io.LimitReader(f, maxBytes))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return data, nil
}

// PackInstaller handles downloading, verifying, extracting, and atomically installing
// workflow packs from GitHub releases. Delegates all transport to pkg/registry.
type PackInstaller struct {
	client     httpx.HTTPDoer
	cliVersion string
}

// NewPackInstaller creates a new PackInstaller with the given CLI version and optional HTTP client.
func NewPackInstaller(cliVersion string, optionalClient ...httpx.HTTPDoer) *PackInstaller {
	var client httpx.HTTPDoer
	if len(optionalClient) > 0 && optionalClient[0] != nil {
		client = optionalClient[0]
	} else {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &PackInstaller{
		client:     client,
		cliVersion: cliVersion,
	}
}

// Install downloads, verifies, extracts, and atomically installs a workflow pack.
//
// url: download URL of the pack archive
// checksum: expected SHA-256 checksum (hex string)
// targetDir: final installation directory (workflow-packs/<name>/)
// force: if true, overwrites an existing pack
// source: source metadata written to state.json after installation
func (pi *PackInstaller) Install(ctx context.Context, url, checksum, targetDir string, force bool, source PackSource) error { //nolint:gocritic // hugeParam: signature required by test expectations
	// Check if target already exists
	if _, statErr := os.Stat(targetDir); statErr == nil {
		if !force {
			return fmt.Errorf("workflow pack already exists at %s", targetDir)
		}
		// Remove existing pack to replace it
		if rmErr := os.RemoveAll(targetDir); rmErr != nil {
			return fmt.Errorf("failed to remove existing pack: %w", rmErr)
		}
	}

	// Download the archive
	data, err := registry.Download(ctx, pi.client, url)
	if err != nil {
		return fmt.Errorf("download pack: %w", err)
	}

	// Verify checksum
	if checksumErr := registry.VerifyChecksum(data, checksum); checksumErr != nil {
		return fmt.Errorf("verify checksum: %w", checksumErr)
	}

	// Create parent directory for temp dir (ensures same filesystem for atomic rename)
	parentDir := filepath.Dir(targetDir)
	if mkdirErr := os.MkdirAll(parentDir, 0o750); mkdirErr != nil {
		return fmt.Errorf("failed to create parent directory: %w", mkdirErr)
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp(parentDir, ".pack-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract archive to temp directory
	if extractErr := registry.ExtractTarGz(data, tempDir); extractErr != nil {
		return fmt.Errorf("extract tar archive: %w", extractErr)
	}

	// Load and validate manifest (1MB cap to prevent OOM from malicious packs)
	manifestPath := filepath.Join(tempDir, ManifestFileName)
	manifestData, readErr := readFileLimited(manifestPath, MaxManifestSize)
	if readErr != nil {
		return fmt.Errorf("read manifest: %w", readErr)
	}

	manifest, parseErr := ParseManifest(manifestData)
	if parseErr != nil {
		return fmt.Errorf("parse manifest: %w", parseErr)
	}

	// Validate manifest
	if validateErr := manifest.Validate(tempDir); validateErr != nil {
		return validateErr
	}

	// Verify manifest name matches target directory name
	targetDirName := filepath.Base(targetDir)
	if manifest.Name != targetDirName {
		return fmt.Errorf("manifest name %q does not match target directory name %q", manifest.Name, targetDirName)
	}

	// Check AWF version constraint
	compatible, versionErr := registry.CheckVersionConstraint(manifest.AWFVersion, pi.cliVersion)
	if versionErr != nil {
		return fmt.Errorf("check awf_version constraint: %w", versionErr)
	}
	if !compatible {
		return fmt.Errorf("workflow pack requires AWF version %s, but CLI version is %s", manifest.AWFVersion, pi.cliVersion)
	}

	// Atomic rename to target directory
	if renameErr := os.Rename(tempDir, targetDir); renameErr != nil {
		return fmt.Errorf("failed to move pack to target directory: %w", renameErr)
	}

	// Write state.json
	sourceData, sourceErr := SourceDataFromPackSource(&source)
	if sourceErr != nil {
		return fmt.Errorf("convert source data: %w", sourceErr)
	}
	state := PackState{
		Name:       manifest.Name,
		Enabled:    true,
		SourceData: sourceData,
	}

	stateFile := filepath.Join(targetDir, "state.json")
	stateData, marshalErr := json.MarshalIndent(state, "", "  ")
	if marshalErr != nil {
		// Cleanup on error
		_ = os.RemoveAll(targetDir) //nolint:errcheck // cleanup attempt in error path
		return fmt.Errorf("marshal state: %w", marshalErr)
	}

	writeErr := os.WriteFile(stateFile, stateData, 0o644) //nolint:gosec // G306: state.json is installation metadata, not sensitive
	if writeErr != nil {
		// Cleanup on error
		_ = os.RemoveAll(targetDir) //nolint:errcheck // cleanup attempt in error path
		return fmt.Errorf("write state.json: %w", writeErr)
	}

	return nil
}
