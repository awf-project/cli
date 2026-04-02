package workflowpkg_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiscoverPacks_ValidPacks tests discovering valid installed packs.
func TestDiscoverPacks_ValidPacks(t *testing.T) {
	// Setup: create temporary directory structure with valid packs
	packsDir := t.TempDir()

	// Create first pack (speckit)
	specPack := filepath.Join(packsDir, "speckit")
	require.NoError(t, os.Mkdir(specPack, 0o755))
	specWf := filepath.Join(specPack, "workflows")
	require.NoError(t, os.Mkdir(specWf, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(specWf, "specify.yaml"), []byte("test"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(specPack, "manifest.yaml"),
		[]byte(`name: speckit
version: "1.0.0"
description: "Spec-driven development"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - specify
`),
		0o644,
	))

	// Create second pack (docgen)
	docPack := filepath.Join(packsDir, "docgen")
	require.NoError(t, os.Mkdir(docPack, 0o755))
	docWf := filepath.Join(docPack, "workflows")
	require.NoError(t, os.Mkdir(docWf, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(docWf, "document.yaml"), []byte("test"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(docPack, "manifest.yaml"),
		[]byte(`name: docgen
version: "2.0.0"
description: "Documentation generator"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - document
`),
		0o644,
	))

	loader := workflowpkg.NewPackLoader()
	packs, err := loader.DiscoverPacks(context.Background(), packsDir)

	assert.NoError(t, err)
	assert.Len(t, packs, 2)

	// Verify both packs are discovered
	names := make(map[string]bool)
	for _, pack := range packs {
		names[pack.Name] = true
	}
	assert.True(t, names["speckit"])
	assert.True(t, names["docgen"])
}

// TestDiscoverPacks_NonexistentDirectory tests behavior with nonexistent directory.
func TestDiscoverPacks_NonexistentDirectory(t *testing.T) {
	loader := workflowpkg.NewPackLoader()
	packs, err := loader.DiscoverPacks(context.Background(), "/nonexistent/path/workflow-packs")

	assert.NoError(t, err)
	assert.Empty(t, packs)
}

// TestDiscoverPacks_EmptyDirectory tests behavior with empty directory.
func TestDiscoverPacks_EmptyDirectory(t *testing.T) {
	packsDir := t.TempDir()

	loader := workflowpkg.NewPackLoader()
	packs, err := loader.DiscoverPacks(context.Background(), packsDir)

	assert.NoError(t, err)
	assert.Empty(t, packs)
}

// TestDiscoverPacks_SkipsInvalidManifest tests that directories with invalid manifests are skipped.
func TestDiscoverPacks_SkipsInvalidManifest(t *testing.T) {
	packsDir := t.TempDir()

	// Create valid pack
	validPack := filepath.Join(packsDir, "valid")
	require.NoError(t, os.Mkdir(validPack, 0o755))
	validWf := filepath.Join(validPack, "workflows")
	require.NoError(t, os.Mkdir(validWf, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(validWf, "test.yaml"), []byte("test"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(validPack, "manifest.yaml"),
		[]byte(`name: valid
version: "1.0.0"
description: "Valid pack"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - test
`),
		0o644,
	))

	// Create pack with missing manifest
	noManifest := filepath.Join(packsDir, "no-manifest")
	require.NoError(t, os.Mkdir(noManifest, 0o755))

	// Create pack with invalid manifest YAML
	invalidYAML := filepath.Join(packsDir, "invalid-yaml")
	require.NoError(t, os.Mkdir(invalidYAML, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(invalidYAML, "manifest.yaml"),
		[]byte("invalid: [unclosed array"),
		0o644,
	))

	// Create pack with missing workflow file referenced in manifest
	missingWf := filepath.Join(packsDir, "missing-workflow")
	require.NoError(t, os.Mkdir(missingWf, 0o755))
	missingWfDir := filepath.Join(missingWf, "workflows")
	require.NoError(t, os.Mkdir(missingWfDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(missingWf, "manifest.yaml"),
		[]byte(`name: missing-workflow
version: "1.0.0"
description: "Missing workflow file"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - nonexistent
`),
		0o644,
	))

	loader := workflowpkg.NewPackLoader()
	packs, err := loader.DiscoverPacks(context.Background(), packsDir)

	// Should only find the valid pack; others should be skipped
	assert.NoError(t, err)
	require.Len(t, packs, 1)
	assert.Equal(t, "valid", packs[0].Name)
}

// TestDiscoverPacks_WithPluginDependencies tests discovering a pack with plugin dependencies.
func TestDiscoverPacks_WithPluginDependencies(t *testing.T) {
	packsDir := t.TempDir()

	// Create pack with plugins
	packDir := filepath.Join(packsDir, "with-plugins")
	require.NoError(t, os.Mkdir(packDir, 0o755))
	wfDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.Mkdir(wfDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "main.yaml"), []byte("test"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(packDir, "manifest.yaml"),
		[]byte(`name: with-plugins
version: "1.0.0"
description: "Pack with plugin dependencies"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - main
plugins:
  security-validator: ">=1.0.0"
  logger-plugin: ">=2.0.0"
`),
		0o644,
	))

	loader := workflowpkg.NewPackLoader()
	packs, err := loader.DiscoverPacks(context.Background(), packsDir)

	require.NoError(t, err)
	require.Len(t, packs, 1)
	assert.Equal(t, "with-plugins", packs[0].Name)
	assert.Len(t, packs[0].Plugins, 2)
	assert.Equal(t, ">=1.0.0", packs[0].Plugins["security-validator"])
	assert.Equal(t, ">=2.0.0", packs[0].Plugins["logger-plugin"])
}

// TestDiscoverPacks_MultipleWorkflowsInPack tests discovering a pack with multiple workflows.
func TestDiscoverPacks_MultipleWorkflowsInPack(t *testing.T) {
	packsDir := t.TempDir()

	// Create pack with multiple workflows
	packDir := filepath.Join(packsDir, "multi-workflow")
	require.NoError(t, os.Mkdir(packDir, 0o755))
	wfDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.Mkdir(wfDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "specify.yaml"), []byte("test"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "clarify.yaml"), []byte("test"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "plan.yaml"), []byte("test"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(packDir, "manifest.yaml"),
		[]byte(`name: multi-workflow
version: "1.5.0"
description: "Pack with multiple workflows"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - specify
  - clarify
  - plan
`),
		0o644,
	))

	loader := workflowpkg.NewPackLoader()
	packs, err := loader.DiscoverPacks(context.Background(), packsDir)

	require.NoError(t, err)
	require.Len(t, packs, 1)
	assert.Equal(t, "multi-workflow", packs[0].Name)
	assert.Len(t, packs[0].Workflows, 3)
}

// TestLoadPackState_HappyPath tests loading a valid state.json file.
func TestLoadPackState_HappyPath(t *testing.T) {
	packDir := t.TempDir()

	stateJSON := []byte(`{
  "name": "speckit",
  "enabled": true,
  "source_data": {
    "repository": "owner/speckit",
    "version": "1.0.0",
    "installed_at": "2026-04-01T10:00:00Z",
    "updated_at": "2026-04-01T11:00:00Z"
  }
}`)

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), stateJSON, 0o644))

	loader := workflowpkg.NewPackLoader()
	state, err := loader.LoadPackState(packDir)

	require.NoError(t, err, "LoadPackState should not return error for valid state.json")
	require.NotNil(t, state, "LoadPackState should return non-nil PackState")
	assert.Equal(t, "speckit", state.Name)
	assert.True(t, state.Enabled)
	assert.NotNil(t, state.SourceData)
	assert.Equal(t, "owner/speckit", state.SourceData["repository"])
	assert.Equal(t, "1.0.0", state.SourceData["version"])
}

// TestLoadPackState_MissingFile tests error when state.json is missing.
func TestLoadPackState_MissingFile(t *testing.T) {
	packDir := t.TempDir()

	loader := workflowpkg.NewPackLoader()
	state, err := loader.LoadPackState(packDir)

	assert.Error(t, err)
	assert.Nil(t, state)
}

// TestLoadPackState_InvalidJSON tests error when state.json contains invalid JSON.
func TestLoadPackState_InvalidJSON(t *testing.T) {
	packDir := t.TempDir()

	invalidJSON := []byte(`{
  "name": "speckit",
  "enabled": true,
  "source_data": {
    "repository": "owner/speckit"
    "version": "1.0.0"
  }
}`)

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), invalidJSON, 0o644))

	loader := workflowpkg.NewPackLoader()
	state, err := loader.LoadPackState(packDir)

	assert.Error(t, err)
	assert.Nil(t, state)
}
