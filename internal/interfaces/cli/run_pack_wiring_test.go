package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T009 Tests: Pack context wiring in CLI run modes
//
// These tests verify that pack namespaces are properly detected and pack context
// is correctly injected into workflow execution across all 4 run modes:
// - runWorkflow
// - runDryRun
// - runInteractive
// - runSingleStep

// TestPackNamespaceDetection verifies namespace parsing for all run modes
func TestPackNamespaceDetection_PackWithSlash(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
		wantPack     string
		wantWF       string
	}{
		{
			name:         "simple pack namespace",
			workflowName: "speckit/specify",
			wantPack:     "speckit",
			wantWF:       "specify",
		},
		{
			name:         "pack with hyphens",
			workflowName: "my-pack/my-workflow",
			wantPack:     "my-pack",
			wantWF:       "my-workflow",
		},
		{
			name:         "multiple slashes splits on first",
			workflowName: "speckit/nested/path",
			wantPack:     "speckit",
			wantWF:       "nested/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pack, wf := parseWorkflowNamespace(tt.workflowName)
			assert.Equal(t, tt.wantPack, pack)
			assert.Equal(t, tt.wantWF, wf)
		})
	}
}

// TestPackNamespaceDetection_LocalWorkflow verifies local workflows have no namespace
func TestPackNamespaceDetection_LocalWorkflow(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
	}{
		{"simple local workflow", "my-workflow"},
		{"local workflow with hyphens", "my-local-workflow"},
		{"local workflow with underscores", "my_workflow"},
		{"local workflow with numbers", "workflow-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pack, wf := parseWorkflowNamespace(tt.workflowName)
			assert.Empty(t, pack)
			assert.Equal(t, tt.workflowName, wf)
		})
	}
}

// TestPackResolution_LocalPrecedence verifies local packs take priority over global
func TestPackResolution_LocalPrecedence(t *testing.T) {
	tmpDir := t.TempDir()

	localPacks := filepath.Join(tmpDir, "local")
	globalPacks := filepath.Join(tmpDir, "global")

	// Setup: create same pack in both locations
	require.NoError(t, os.MkdirAll(filepath.Join(localPacks, "test-pack"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(globalPacks, "test-pack"), 0o755))

	// Create manifests
	localManifest := "name: test-pack\nworkflows:\n  - local-wf\n"
	globalManifest := "name: test-pack\nworkflows:\n  - global-wf\n"

	require.NoError(t, os.WriteFile(
		filepath.Join(localPacks, "test-pack", "manifest.yaml"),
		[]byte(localManifest),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(globalPacks, "test-pack", "manifest.yaml"),
		[]byte(globalManifest),
		0o644,
	))

	// Test: Should resolve to local
	resolved, err := resolvePackDir("test-pack", localPacks, globalPacks)
	require.NoError(t, err)

	// Verify it resolved to local
	assert.Contains(t, resolved, "local")
	assert.NotContains(t, resolved, "global")
}

// TestPackResolution_GlobalFallback verifies global fallback when local unavailable
func TestPackResolution_GlobalFallback(t *testing.T) {
	tmpDir := t.TempDir()

	localPacks := filepath.Join(tmpDir, "local")
	globalPacks := filepath.Join(tmpDir, "global")

	// Setup: create pack only in global
	require.NoError(t, os.MkdirAll(filepath.Join(globalPacks, "test-pack"), 0o755))

	manifestData := "name: test-pack\nworkflows:\n  - test-wf\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(globalPacks, "test-pack", "manifest.yaml"),
		[]byte(manifestData),
		0o644,
	))

	// Test: Should fall back to global
	resolved, err := resolvePackDir("test-pack", localPacks, globalPacks)
	require.NoError(t, err)

	assert.Contains(t, resolved, "global")
}

// TestPackResolution_NotFound verifies error when pack not in either location
func TestPackResolution_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	localPacks := filepath.Join(tmpDir, "local")
	globalPacks := filepath.Join(tmpDir, "global")

	// Setup: create directories but no pack
	require.NoError(t, os.MkdirAll(localPacks, 0o755))
	require.NoError(t, os.MkdirAll(globalPacks, 0o755))

	// Test: Should fail with clear error
	_, err := resolvePackDir("nonexistent-pack", localPacks, globalPacks)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestPackValidation_ManifestChecks verifies workflow listed in manifest
func TestPackValidation_ManifestChecks(t *testing.T) {
	tmpDir := t.TempDir()

	packDir := filepath.Join(tmpDir, "test-pack")
	require.NoError(t, os.MkdirAll(packDir, 0o755))

	// Create manifest with specific workflows
	manifest := "name: test-pack\nworkflows:\n  - public-wf\n  - another-wf\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(packDir, "manifest.yaml"),
		[]byte(manifest),
		0o644,
	))

	// Test: Listed workflow passes
	err := validatePackWorkflow(packDir, "public-wf")
	assert.NoError(t, err)

	// Test: Another listed workflow passes
	err = validatePackWorkflow(packDir, "another-wf")
	assert.NoError(t, err)

	// Test: Unlisted workflow fails
	err = validatePackWorkflow(packDir, "private-wf")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not listed in pack manifest")
}

// TestPackValidation_PathTraversalRejection verifies security validation
func TestPackValidation_PathTraversalRejection(t *testing.T) {
	tmpDir := t.TempDir()

	localPacks := filepath.Join(tmpDir, "local")
	require.NoError(t, os.MkdirAll(localPacks, 0o755))

	// Setup: create a valid pack for workflow traversal test
	require.NoError(t, os.MkdirAll(filepath.Join(localPacks, "safe-pack"), 0o755))
	manifest := "name: safe-pack\nworkflows:\n  - test\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(localPacks, "safe-pack", "manifest.yaml"),
		[]byte(manifest),
		0o644,
	))

	// Test: Pack name with traversal should error
	_, err := resolvePackDir("../escape", localPacks, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")

	// Test: Workflow name with traversal should error
	packDir := filepath.Join(localPacks, "safe-pack")
	err = validatePackWorkflow(packDir, "../escape")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

// TestPackContextInjection_AWFPathsWithPack verifies pack_name in AWF context
func TestPackContextInjection_AWFPathsWithPack(t *testing.T) {
	awfPaths := buildPackAWFPaths("speckit")

	// Verify pack_name is set
	assert.Equal(t, "speckit", awfPaths["pack_name"])

	// Verify standard paths are present
	standardPaths := []string{"config_dir", "prompts_dir", "scripts_dir"}
	for _, path := range standardPaths {
		assert.Contains(t, awfPaths, path, "AWF paths should contain %s", path)
		assert.NotEmpty(t, awfPaths[path], "AWF path %s should not be empty", path)
	}
}

// TestPackContextInjection_LocalWorkflowsUnchanged verifies local behavior preserved
func TestPackContextInjection_LocalWorkflowsUnchanged(t *testing.T) {
	localAWFPaths := buildAWFPaths()

	// Verify pack_name is NOT set for local workflows
	_, hasPack := localAWFPaths["pack_name"]
	assert.False(t, hasPack, "local workflows should not have pack_name in AWF paths")

	// Verify standard paths are present
	standardPaths := []string{"config_dir", "prompts_dir", "scripts_dir"}
	for _, path := range standardPaths {
		assert.Contains(t, localAWFPaths, path)
		assert.NotEmpty(t, localAWFPaths[path])
	}
}

// TestPackContextInjection_PathDifference verifies pack paths differ from local
func TestPackContextInjection_PathDifference(t *testing.T) {
	localPaths := buildAWFPaths()
	packPaths := buildPackAWFPaths("test-pack")

	// Both should have config_dir (or similar)
	assert.Contains(t, localPaths, "config_dir")
	assert.Contains(t, packPaths, "config_dir")

	// pack_name should be present in pack paths only
	_, hasLocalPack := localPaths["pack_name"]
	_, hasPackPack := packPaths["pack_name"]

	assert.False(t, hasLocalPack)
	assert.True(t, hasPackPack)
}

// TestWorkflowResolution_FullPackPath verifies complete resolution flow
func TestWorkflowResolution_FullPackPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup: create complete pack structure
	packDir := filepath.Join(tmpDir, "workflow-packs", "complete-pack")
	workflowsDir := filepath.Join(packDir, "workflows")

	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	// Create manifest
	manifest := &workflowpkg.Manifest{
		Name:      "complete-pack",
		Workflows: []string{"complete-workflow"},
	}
	manifestData := fmt.Sprintf("name: %s\nworkflows:\n  - complete-workflow\n", manifest.Name)
	require.NoError(t, os.WriteFile(
		filepath.Join(packDir, "manifest.yaml"),
		[]byte(manifestData),
		0o644,
	))

	// Create workflow
	workflowYAML := `name: complete-workflow
description: Complete test workflow

states:
  initial: step1
  step1:
    type: step
    command: echo test
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(
		filepath.Join(workflowsDir, "complete-workflow.yaml"),
		[]byte(workflowYAML),
		0o644,
	))

	// Test: Full resolution pipeline
	packName, workflowName := parseWorkflowNamespace("complete-pack/complete-workflow")
	assert.Equal(t, "complete-pack", packName)
	assert.Equal(t, "complete-workflow", workflowName)

	// Resolve pack directory
	packRoot, err := resolvePackDir(packName, filepath.Join(tmpDir, "workflow-packs"), "")
	require.NoError(t, err)

	// Validate workflow in manifest
	err = validatePackWorkflow(packRoot, workflowName)
	require.NoError(t, err)

	// Resolve full workflow from pack
	ctx := context.Background()
	wf, resolvedPackRoot, err := resolvePackWorkflow(
		ctx,
		packName,
		workflowName,
		filepath.Join(tmpDir, "workflow-packs"),
		"",
	)
	require.NoError(t, err)
	require.NotNil(t, wf)

	// Verify workflow loaded correctly
	assert.Equal(t, "complete-workflow", wf.Name)
	assert.NotEmpty(t, resolvedPackRoot)
	assert.NotEmpty(t, wf.Steps)

	// Verify AWF paths injected with pack context
	awfPaths := buildPackAWFPaths(packName)
	assert.Equal(t, packName, awfPaths["pack_name"])
}

// TestWorkflowResolution_LocalWorkflowBypass verifies local workflows skip pack logic
func TestWorkflowResolution_LocalWorkflowBypass(t *testing.T) {
	// For local workflows (no namespace), pack resolution should be skipped
	pack, workflow := parseWorkflowNamespace("local-workflow")

	// Should have empty pack name
	assert.Empty(t, pack)
	assert.Equal(t, "local-workflow", workflow)

	// AWF paths should not include pack_name
	localPaths := buildAWFPaths()
	_, hasPack := localPaths["pack_name"]
	assert.False(t, hasPack)
}

// TestErrorHandling_InvalidPackName verifies proper error on bad pack name
func TestErrorHandling_InvalidPackName(t *testing.T) {
	tmpDir := t.TempDir()

	localPacks := filepath.Join(tmpDir, "packs")
	require.NoError(t, os.MkdirAll(localPacks, 0o755))

	// Test: Pack name with traversal patterns should error
	_, err := resolvePackDir("../../escape", localPacks, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

// TestErrorHandling_InvalidWorkflowName verifies proper error on bad workflow name
func TestErrorHandling_InvalidWorkflowName(t *testing.T) {
	tmpDir := t.TempDir()

	packDir := filepath.Join(tmpDir, "test-pack")
	require.NoError(t, os.MkdirAll(packDir, 0o755))

	manifest := "name: test-pack\nworkflows:\n  - test-wf\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(packDir, "manifest.yaml"),
		[]byte(manifest),
		0o644,
	))

	// Test: Workflow name with traversal patterns should error
	err := validatePackWorkflow(packDir, "../../escape")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

// TestErrorHandling_MissingManifest verifies error on missing manifest
func TestErrorHandling_MissingManifest(t *testing.T) {
	tmpDir := t.TempDir()

	packDir := filepath.Join(tmpDir, "no-manifest-pack")
	require.NoError(t, os.MkdirAll(packDir, 0o755))

	// Test: Missing manifest should error
	err := validatePackWorkflow(packDir, "any-workflow")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read manifest")
}

// TestBuildPackAWFPaths_ConsistentAcrossCalls verifies idempotency
func TestBuildPackAWFPaths_ConsistentAcrossCalls(t *testing.T) {
	// Multiple calls should produce same paths for same pack
	paths1 := buildPackAWFPaths("test-pack")
	paths2 := buildPackAWFPaths("test-pack")

	assert.Equal(t, paths1, paths2)

	// Different packs should have different pack_name but same structure
	paths3 := buildPackAWFPaths("other-pack")
	assert.Equal(t, "test-pack", paths1["pack_name"])
	assert.Equal(t, "other-pack", paths3["pack_name"])

	// Same keys (except pack_name value) should be present
	for key := range paths1 {
		assert.Contains(t, paths3, key)
	}
}
