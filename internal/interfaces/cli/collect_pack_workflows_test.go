package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCollectPackWorkflows_EmptyPackDirectory verifies that when no packs
// are installed (empty workflow-packs directory), an empty list is returned without error.
func TestCollectPackWorkflows_EmptyPackDirectory(t *testing.T) {
	tmpHome := createTestPackHome(t)
	t.Setenv("HOME", tmpHome)

	workflows, err := collectPackWorkflows(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, workflows)
}

// TestCollectPackWorkflows_SinglePackSingleWorkflow verifies that a pack with
// one workflow returns a single WorkflowInfo entry with "pack/workflow" naming
// and "pack" source label.
func TestCollectPackWorkflows_SinglePackSingleWorkflow(t *testing.T) {
	tmpHome := createTestPackHome(t)
	t.Setenv("HOME", tmpHome)

	createTestPack(t, tmpHome, "example-pack", "1.0.0", map[string]string{
		"hello": `name: hello
version: "1.0.0"
states:
  initial: start
  start:
    type: terminal
`,
	})

	workflows, err := collectPackWorkflows(context.Background())

	require.NoError(t, err)
	require.Len(t, workflows, 1)

	wf := workflows[0]
	assert.Equal(t, "example-pack/hello", wf.Name)
	assert.Equal(t, "pack", wf.Source)
	assert.Equal(t, "1.0.0", wf.Version)
}

// TestCollectPackWorkflows_SinglePackMultipleWorkflows verifies that all
// workflows from a single pack are discovered and returned with correct naming.
func TestCollectPackWorkflows_SinglePackMultipleWorkflows(t *testing.T) {
	tmpHome := createTestPackHome(t)
	t.Setenv("HOME", tmpHome)

	createTestPack(t, tmpHome, "multi-pack", "2.0.0", map[string]string{
		"workflow1": `name: workflow1
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
		"workflow2": `name: workflow2
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
		"workflow3": `name: workflow3
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	})

	workflows, err := collectPackWorkflows(context.Background())

	require.NoError(t, err)
	require.Len(t, workflows, 3)

	names := make(map[string]bool)
	for _, wf := range workflows {
		assert.Equal(t, "pack", wf.Source)
		assert.Equal(t, "2.0.0", wf.Version)
		names[wf.Name] = true
	}

	assert.True(t, names["multi-pack/workflow1"])
	assert.True(t, names["multi-pack/workflow2"])
	assert.True(t, names["multi-pack/workflow3"])
}

// TestCollectPackWorkflows_MultiplePacksMultipleWorkflows verifies that
// multiple installed packs with multiple workflows are all discovered correctly.
func TestCollectPackWorkflows_MultiplePacksMultipleWorkflows(t *testing.T) {
	tmpHome := createTestPackHome(t)
	t.Setenv("HOME", tmpHome)

	createTestPack(t, tmpHome, "pack-a", "1.0.0", map[string]string{
		"wf-a1": `name: wf-a1
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
		"wf-a2": `name: wf-a2
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	})

	createTestPack(t, tmpHome, "pack-b", "3.0.0", map[string]string{
		"wf-b1": `name: wf-b1
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	})

	workflows, err := collectPackWorkflows(context.Background())

	require.NoError(t, err)
	require.Len(t, workflows, 3)

	names := make(map[string]bool)
	for _, wf := range workflows {
		assert.Equal(t, "pack", wf.Source)
		names[wf.Name] = true
	}

	assert.True(t, names["pack-a/wf-a1"])
	assert.True(t, names["pack-a/wf-a2"])
	assert.True(t, names["pack-b/wf-b1"])
}

// TestCollectPackWorkflows_DisabledPackIgnored verifies that packs with
// enabled=false in state.json are excluded from the results.
func TestCollectPackWorkflows_DisabledPackIgnored(t *testing.T) {
	tmpHome := createTestPackHome(t)
	t.Setenv("HOME", tmpHome)

	// Create disabled pack
	packDir := filepath.Join(tmpHome, ".awf", "workflow-packs", "disabled-pack")
	require.NoError(t, os.MkdirAll(packDir, 0o755))

	createPackManifest(t, packDir, "disabled-pack", "1.0.0", []string{"workflow"})

	workflowDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(workflowDir, "workflow.yaml"),
		[]byte(`name: workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`), 0o644,
	))

	createPackState(t, packDir, "disabled-pack", "1.0.0", false)

	workflows, err := collectPackWorkflows(context.Background())

	require.NoError(t, err)
	assert.Empty(t, workflows)
}

// TestCollectPackWorkflows_MissingManifest handles gracefully when a pack
// directory exists but manifest.yaml is missing (either returns error or skips pack).
func TestCollectPackWorkflows_MissingManifest(t *testing.T) {
	tmpHome := createTestPackHome(t)
	t.Setenv("HOME", tmpHome)

	packDir := filepath.Join(tmpHome, ".awf", "workflow-packs", "broken-pack")
	require.NoError(t, os.MkdirAll(packDir, 0o755))

	// Write state.json but no manifest.yaml
	createPackState(t, packDir, "broken-pack", "1.0.0", true)

	workflows, err := collectPackWorkflows(context.Background())

	// Should handle gracefully - either error or empty result
	if err == nil {
		assert.Empty(t, workflows)
	} else {
		assert.NotNil(t, err)
	}
}

// TestCollectPackWorkflows_NoWorkflowPacksDirectory handles gracefully when
// the workflow-packs directory doesn't exist (normal state for fresh installs).
func TestCollectPackWorkflows_NoWorkflowPacksDirectory(t *testing.T) {
	// Create temp home WITHOUT workflow-packs directory
	tmpHome := t.TempDir()
	awfDir := filepath.Join(tmpHome, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))
	// Deliberately don't create workflow-packs

	t.Setenv("HOME", tmpHome)

	workflows, err := collectPackWorkflows(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, workflows)
}

// TestCollectPackWorkflows_IncludesDescription verifies that workflow descriptions
// are loaded from the individual workflow YAML files inside the pack.
func TestCollectPackWorkflows_IncludesDescription(t *testing.T) {
	tmpHome := createTestPackHome(t)
	t.Setenv("HOME", tmpHome)

	createTestPack(t, tmpHome, "desc-pack", "1.0.0", map[string]string{
		"with-desc": `name: with-desc
description: "A workflow with a description"
states:
  initial: done
  done:
    type: terminal
`,
		"no-desc": `name: no-desc
states:
  initial: done
  done:
    type: terminal
`,
	})

	workflows, err := collectPackWorkflows(context.Background())

	require.NoError(t, err)
	require.Len(t, workflows, 2)

	byName := make(map[string]string)
	for _, wf := range workflows {
		byName[wf.Name] = wf.Description
	}

	assert.Equal(t, "A workflow with a description", byName["desc-pack/with-desc"])
	assert.Empty(t, byName["desc-pack/no-desc"])
}

// TestCollectPackWorkflows_DescriptionFromPackNotLocal verifies that pack
// workflow descriptions come from the pack's workflow files, not from
// identically-named workflows in local/global directories.
func TestCollectPackWorkflows_DescriptionFromPackNotLocal(t *testing.T) {
	tmpHome := createTestPackHome(t)
	t.Setenv("HOME", tmpHome)

	// Create pack workflow with specific description
	createTestPack(t, tmpHome, "mypack", "1.0.0", map[string]string{
		"shared": `name: shared
description: "Pack version of shared workflow"
states:
  initial: done
  done:
    type: terminal
`,
	})

	// Create a local workflow with the same name but different description
	localDir := filepath.Join(tmpHome, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(localDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(localDir, "shared.yaml"),
		[]byte(`name: shared
description: "Local version of shared workflow"
states:
  initial: done
  done:
    type: terminal
`), 0o644,
	))

	workflows, err := collectPackWorkflows(context.Background())

	require.NoError(t, err)
	require.Len(t, workflows, 1)
	assert.Equal(t, "Pack version of shared workflow", workflows[0].Description)
}

// ==================== Test Helpers ====================

// createTestPackHome creates a temporary directory with .awf/workflow-packs structure.
func createTestPackHome(t *testing.T) string {
	t.Helper()
	tmpHome := t.TempDir()
	awfDir := filepath.Join(tmpHome, ".awf")
	require.NoError(t, os.MkdirAll(filepath.Join(awfDir, "workflow-packs"), 0o755))
	return tmpHome
}

// createTestPack creates a complete workflow pack with manifest, workflows, and state.json.
// workflows map contains workflow name → YAML content.
func createTestPack(t *testing.T, home, packName, version string, workflows map[string]string) {
	t.Helper()

	packDir := filepath.Join(home, ".awf", "workflow-packs", packName)
	require.NoError(t, os.MkdirAll(packDir, 0o755))

	// Create workflows directory and files
	workflowDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowDir, 0o755))

	workflowNames := make([]string, 0, len(workflows))
	for name, content := range workflows {
		workflowNames = append(workflowNames, name)
		require.NoError(t, os.WriteFile(
			filepath.Join(workflowDir, name+".yaml"),
			[]byte(content),
			0o644,
		))
	}

	// Create manifest
	createPackManifest(t, packDir, packName, version, workflowNames)

	// Create state.json
	createPackState(t, packDir, packName, version, true)
}

// createPackManifest creates manifest.yaml for a test pack.
func createPackManifest(t *testing.T, packDir, packName, version string, workflowNames []string) {
	t.Helper()

	manifest := `name: ` + packName + `
version: "` + version + `"
description: Test workflow pack
author: test-author
license: MIT
awf_version: ">=0.5.0"
workflows:
`
	for _, name := range workflowNames {
		manifest += `  - ` + name + "\n"
	}

	require.NoError(t, os.WriteFile(
		filepath.Join(packDir, "manifest.yaml"),
		[]byte(manifest),
		0o644,
	))
}

// createPackState creates state.json for a test pack.
func createPackState(t *testing.T, packDir, packName, version string, enabled bool) {
	t.Helper()

	stateJSON := `{
  "name": "` + packName + `",
  "enabled": ` + boolToJSON(enabled) + `,
  "source_data": {
    "repository": "https://github.com/test/` + packName + `",
    "version": "` + version + `",
    "installed_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
`
	require.NoError(t, os.WriteFile(
		filepath.Join(packDir, "state.json"),
		[]byte(stateJSON),
		0o644,
	))
}

// boolToJSON converts a boolean to JSON string.
func boolToJSON(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
