package workflowpkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// PackLoader discovers installed workflow packs from filesystem directories.
type PackLoader struct{}

// NewPackLoader creates a new PackLoader.
func NewPackLoader() *PackLoader {
	return &PackLoader{}
}

// DiscoverPacks scans packsDir for installed workflow packs.
// Each subdirectory containing a manifest.yaml is considered a pack.
// Returns empty slice for nonexistent or empty directories.
// Skips subdirectories with invalid or missing manifests.
func (l *PackLoader) DiscoverPacks(ctx context.Context, packsDir string) ([]PackInfo, error) {
	entries, err := os.ReadDir(packsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []PackInfo{}, nil
		}
		return nil, err
	}

	var packs []PackInfo

	for _, entry := range entries {
		// Respect context cancellation
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("discover packs: %w", ctx.Err())
		default:
		}

		// Only process directories
		if !entry.IsDir() {
			continue
		}

		packDir := filepath.Join(packsDir, entry.Name())

		// Try to read manifest (1MB cap)
		manifestPath := filepath.Join(packDir, "manifest.yaml")
		manifestData, err := readFileLimited(manifestPath, maxManifestSize)
		if err != nil {
			// Skip packs without manifest.yaml or oversized manifests
			continue
		}

		// Parse manifest
		manifest, err := ParseManifest(manifestData)
		if err != nil {
			// Skip packs with invalid manifest YAML
			continue
		}

		// Validate manifest (name, version, awf_version, workflow files)
		if err := manifest.Validate(packDir); err != nil {
			// Skip packs that fail validation
			continue
		}

		// Convert manifest to PackInfo
		workflowsMap := make(map[string]string)
		for _, wf := range manifest.Workflows {
			workflowsMap[wf] = wf + ".yaml"
		}

		packInfo := PackInfo{
			Name:        manifest.Name,
			Version:     manifest.Version,
			Description: manifest.Description,
			Author:      manifest.Author,
			License:     manifest.License,
			Workflows:   workflowsMap,
			Plugins:     manifest.Plugins,
		}

		packs = append(packs, packInfo)
	}

	return packs, nil
}
