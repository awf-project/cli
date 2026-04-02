//go:build integration

package cli_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PackSourceForState represents serializable pack source metadata for state.json.
type PackSourceForState struct {
	Repository  string    `json:"repository"`
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// createPackStructureWithState creates a complete workflow pack structure with manifest and state.json.
// Extends createPackStructure by adding valid version/awf_version/description fields to manifest
// and writing state.json with source metadata needed by info/update commands.
func createPackStructureWithState(
	t *testing.T,
	baseDir string,
	packName string,
	workflows map[string]string,
	source PackSourceForState,
) {
	t.Helper()

	packDir := filepath.Join(baseDir, ".awf", "workflow-packs", packName)
	require.NoError(t, os.MkdirAll(packDir, 0o755))

	workflowsDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	for name, content := range workflows {
		workflowPath := filepath.Join(workflowsDir, name+".yaml")
		require.NoError(t, os.WriteFile(workflowPath, []byte(content), 0o644))
	}

	manifestContent := fmt.Sprintf(`name: %s
version: "%s"
description: Test workflow pack for %s
author: test-author
license: MIT
awf_version: ">=0.5.0"
workflows:
`, packName, source.Version, packName)

	for name := range workflows {
		manifestContent += fmt.Sprintf("  - %s\n", name)
	}

	manifestPath := filepath.Join(packDir, "manifest.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifestContent), 0o644))

	stateData := map[string]interface{}{
		"name":    packName,
		"enabled": true,
		"source_data": map[string]interface{}{
			"repository":   source.Repository,
			"version":      source.Version,
			"installed_at": source.InstalledAt,
			"updated_at":   source.UpdatedAt,
		},
	}

	stateBytes, err := json.MarshalIndent(stateData, "", "  ")
	require.NoError(t, err)

	statePath := filepath.Join(packDir, "state.json")
	require.NoError(t, os.WriteFile(statePath, stateBytes, 0o644))
}

// Tests for runWorkflowList command

func TestWorkflowList_EmptyDir(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	output, err := runCLI(t, "workflow", "list")

	assert.NoError(t, err)
	assert.NotNil(t, output)
}

func TestWorkflowList_SinglePack(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	source := PackSourceForState{
		Repository:  "https://github.com/test/pack",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "test-pack", map[string]string{
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
	}, source)

	output, err := runCLI(t, "workflow", "list")

	assert.NoError(t, err)
	assert.Contains(t, output, "test-pack")
	assert.Contains(t, output, "1.0.0")
	assert.Contains(t, output, "workflow1")
	assert.Contains(t, output, "workflow2")
}

func TestWorkflowList_MultiplePacks(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	source1 := PackSourceForState{
		Repository:  "https://github.com/test/pack1",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	source2 := PackSourceForState{
		Repository:  "https://github.com/test/pack2",
		Version:     "2.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "pack-one", map[string]string{
		"workflow1": `name: workflow1
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source1)

	createPackStructureWithState(t, tmpDir, "pack-two", map[string]string{
		"workflow2": `name: workflow2
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source2)

	output, err := runCLI(t, "workflow", "list")

	assert.NoError(t, err)
	assert.Contains(t, output, "pack-one")
	assert.Contains(t, output, "1.0.0")
	assert.Contains(t, output, "pack-two")
	assert.Contains(t, output, "2.0.0")
}

func TestWorkflowList_WithLocalWorkflows(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	createTestWorkflow(t, tmpDir, "local-workflow.yaml", `name: local-workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`)

	output, err := runCLI(t, "workflow", "list")

	assert.NoError(t, err)
	assert.Contains(t, output, "(local)")
	assert.Contains(t, output, "local-workflow")
}

func TestWorkflowList_LocalAndPackWorkflows(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	source := PackSourceForState{
		Repository:  "https://github.com/test/pack",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "test-pack", map[string]string{
		"pack-workflow": `name: pack-workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source)

	createTestWorkflow(t, tmpDir, "local-workflow.yaml", `name: local-workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`)

	output, err := runCLI(t, "workflow", "list")

	assert.NoError(t, err)
	assert.Contains(t, output, "test-pack")
	assert.Contains(t, output, "(local)")
	assert.Contains(t, output, "pack-workflow")
	assert.Contains(t, output, "local-workflow")
}

func TestWorkflowList_NoLocalWorkflows(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	source := PackSourceForState{
		Repository:  "https://github.com/test/pack",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "test-pack", map[string]string{
		"workflow": `name: workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source)

	output, err := runCLI(t, "workflow", "list")

	assert.NoError(t, err)
	assert.Contains(t, output, "test-pack")
	assert.NotContains(t, output, "(local)")
}

func TestWorkflowList_VersionDisplay(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	source := PackSourceForState{
		Repository:  "https://github.com/org/pack",
		Version:     "3.2.1",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "versioned-pack", map[string]string{
		"wf": `name: wf
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source)

	output, err := runCLI(t, "workflow", "list")

	assert.NoError(t, err)
	assert.Contains(t, output, "3.2.1")
}

// Tests for runWorkflowInfo command

func TestWorkflowInfo_ExistingPack_DisplaysManifest(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	source := PackSourceForState{
		Repository:  "https://github.com/test/awf-workflow-pack",
		Version:     "1.2.3",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "test-pack", map[string]string{
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
	}, source)

	output, err := runCLI(t, "workflow", "info", "test-pack")

	assert.NoError(t, err)
	assert.Contains(t, output, "test-pack")
	assert.Contains(t, output, "1.2.3")
	assert.Contains(t, output, "Test workflow pack for test-pack")
	assert.Contains(t, output, "workflow1")
	assert.Contains(t, output, "workflow2")
}

func TestWorkflowInfo_MissingPack_ReturnsError(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	output, err := runCLI(t, "workflow", "info", "nonexistent-pack")

	assert.Error(t, err)
	assert.NotEmpty(t, output)
}

func TestWorkflowInfo_DisplaysPluginDependencies(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create a pack with plugins in the manifest
	packName := "plugin-pack"
	packDir := filepath.Join(tmpDir, ".awf", "workflow-packs", packName)
	require.NoError(t, os.MkdirAll(filepath.Join(packDir, "workflows"), 0o755))

	// Create manifest with plugins
	manifestContent := fmt.Sprintf(`name: %s
version: "1.0.0"
description: Pack with plugins
author: test-author
license: MIT
awf_version: ">=0.5.0"
plugins:
  my-plugin: ">=1.0.0"
  another-plugin: "^2.0.0"
workflows:
  - test-workflow
`, packName)

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifestContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "workflows", "test-workflow.yaml"), []byte(`name: test-workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`), 0o644))

	// Write state.json
	stateData := map[string]interface{}{
		"name":    packName,
		"enabled": true,
		"source_data": map[string]interface{}{
			"repository":   "https://github.com/test/pack",
			"version":      "1.0.0",
			"installed_at": time.Now(),
			"updated_at":   time.Now(),
		},
	}
	stateBytes, _ := json.MarshalIndent(stateData, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), stateBytes, 0o644))

	output, err := runCLI(t, "workflow", "info", packName)

	assert.NoError(t, err)
	assert.Contains(t, output, "my-plugin")
	assert.Contains(t, output, "another-plugin")
}

func TestWorkflowInfo_WithReadme(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	source := PackSourceForState{
		Repository:  "https://github.com/test/pack",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "readme-pack", map[string]string{
		"workflow": `name: workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source)

	// Add README to the pack
	packDir := filepath.Join(tmpDir, ".awf", "workflow-packs", "readme-pack")
	readmeContent := `# README Pack

This is a test workflow pack.

## Features
- Feature 1
- Feature 2
`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "README.md"), []byte(readmeContent), 0o644))

	output, err := runCLI(t, "workflow", "info", "readme-pack")

	assert.NoError(t, err)
	assert.Contains(t, output, "README")
	assert.Contains(t, output, "This is a test workflow pack")
	assert.Contains(t, output, "Features")
}

// T005: TestWorkflowUpdate_NewerVersionAvailable
// Update command downloads and installs when newer version exists on GitHub.
// Covers: Version comparison, download, checksum verify, atomic install, state.json update.
func TestWorkflowUpdate_NewerVersionAvailable(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create pack with version 1.0.0
	source := PackSourceForState{
		Repository:  "testorg/awf-workflow-speckit",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "speckit", map[string]string{
		"workflow": `name: workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source)

	// Create mock GitHub server with version 1.1.0 available
	newVersion := "1.1.0"
	assetName := fmt.Sprintf("awf-workflow-speckit_%s.tar.gz", newVersion)
	tarball := createTestWorkflowTarball(t, "speckit")
	checksumData := createTestWorkflowSHA256File(t, tarball, assetName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			releases := []map[string]interface{}{
				{
					"tag_name": "v" + newVersion,
					"assets": []map[string]interface{}{
						{
							"name":                 assetName,
							"browser_download_url": "http://" + r.Host + "/downloads/" + assetName,
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(releases) //nolint:errcheck
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/"+assetName) {
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(tarball) //nolint:errcheck
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/checksums.txt") {
			w.Write([]byte(checksumData)) //nolint:errcheck
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	output, err := runCLI(t, "workflow", "update", "speckit")

	assert.NoError(t, err)
	assert.Contains(t, output, "1.1.0")

	// Verify state.json was updated with new version
	statePath := filepath.Join(tmpDir, ".awf", "workflow-packs", "speckit", "state.json")
	stateBytes, err := os.ReadFile(statePath)
	require.NoError(t, err)

	var stateData map[string]interface{}
	err = json.Unmarshal(stateBytes, &stateData)
	require.NoError(t, err)

	sourceData, ok := stateData["source_data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, newVersion, sourceData["version"])
}

// T005: TestWorkflowUpdate_AlreadyAtLatest
// Update command shows "already at latest" when no newer version available.
func TestWorkflowUpdate_AlreadyAtLatest(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	currentVersion := "1.0.0"
	source := PackSourceForState{
		Repository:  "testorg/awf-workflow-speckit",
		Version:     currentVersion,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "speckit", map[string]string{
		"workflow": `name: workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source)

	// Mock server returns only current version (no newer release)
	assetName := fmt.Sprintf("awf-workflow-speckit_%s.tar.gz", currentVersion)
	tarball := createTestWorkflowTarball(t, "speckit")
	checksumDataForAsset := createTestWorkflowSHA256File(t, tarball, assetName)
	_ = checksumDataForAsset // Used implicitly in release data

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			releases := []map[string]interface{}{
				{
					"tag_name": "v" + currentVersion,
					"assets": []map[string]interface{}{
						{
							"name":                 assetName,
							"browser_download_url": "http://" + r.Host + "/downloads/" + assetName,
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(releases) //nolint:errcheck
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	output, err := runCLI(t, "workflow", "update", "speckit")

	assert.NoError(t, err)
	assert.Contains(t, output, "already")
}

// T005: TestWorkflowUpdate_PackNotFound
// Update command returns user error when pack doesn't exist.
func TestWorkflowUpdate_PackNotFound(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Initialize .awf directory but no packs
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf"), 0o755))

	output, err := runCLI(t, "workflow", "update", "nonexistent-pack")

	assert.Error(t, err)
	assert.NotEmpty(t, output)
}

// T005: TestWorkflowUpdate_PreservesUserOverrides
// Update command preserves user-created overrides in prompts/<pack>/ and scripts/<pack>/.
func TestWorkflowUpdate_PreservesUserOverrides(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create pack with version 1.0.0
	source := PackSourceForState{
		Repository:  "testorg/awf-workflow-speckit",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "speckit", map[string]string{
		"workflow": `name: workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source)

	// Create user override directories with custom files
	awfDir := filepath.Join(tmpDir, ".awf")
	customPromptDir := filepath.Join(awfDir, "prompts", "speckit")
	customScriptDir := filepath.Join(awfDir, "scripts", "speckit")

	require.NoError(t, os.MkdirAll(customPromptDir, 0o755))
	require.NoError(t, os.MkdirAll(customScriptDir, 0o755))

	customPrompt := "# Custom Prompt\nThis is a user override"
	customScript := "#!/bin/bash\necho 'user script'"

	require.NoError(t, os.WriteFile(filepath.Join(customPromptDir, "custom.md"), []byte(customPrompt), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(customScriptDir, "custom.sh"), []byte(customScript), 0o755))

	// Create mock GitHub server with version 1.1.0 available
	newVersion := "1.1.0"
	assetName := fmt.Sprintf("awf-workflow-speckit_%s.tar.gz", newVersion)
	tarball := createTestWorkflowTarball(t, "speckit")
	checksumData := createTestWorkflowSHA256File(t, tarball, assetName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases") {
			releases := []map[string]interface{}{
				{
					"tag_name": "v" + newVersion,
					"assets": []map[string]interface{}{
						{
							"name":                 assetName,
							"browser_download_url": "http://" + r.Host + "/downloads/" + assetName,
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(releases) //nolint:errcheck
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/"+assetName) {
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(tarball) //nolint:errcheck
			return
		}

		if strings.Contains(r.URL.Path, "/downloads/checksums.txt") {
			w.Write([]byte(checksumData)) //nolint:errcheck
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	_, err := runCLI(t, "workflow", "update", "speckit")

	assert.NoError(t, err)

	// Verify state.json was updated with new version
	statePath := filepath.Join(tmpDir, ".awf", "workflow-packs", "speckit", "state.json")
	stateBytes, err := os.ReadFile(statePath)
	require.NoError(t, err)

	var stateData map[string]interface{}
	err = json.Unmarshal(stateBytes, &stateData)
	require.NoError(t, err)

	sourceData, ok := stateData["source_data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, newVersion, sourceData["version"])

	// Verify user overrides were preserved
	customPromptContent, err := os.ReadFile(filepath.Join(customPromptDir, "custom.md"))
	require.NoError(t, err)
	assert.Equal(t, customPrompt, string(customPromptContent))

	customScriptContent, err := os.ReadFile(filepath.Join(customScriptDir, "custom.sh"))
	require.NoError(t, err)
	assert.Equal(t, customScript, string(customScriptContent))
}

func TestWorkflowInfo_WithoutReadme_StillWorks(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	source := PackSourceForState{
		Repository:  "https://github.com/test/pack",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "no-readme-pack", map[string]string{
		"workflow": `name: workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source)

	output, err := runCLI(t, "workflow", "info", "no-readme-pack")

	assert.NoError(t, err)
	assert.Contains(t, output, "no-readme-pack")
	assert.Contains(t, output, "1.0.0")
}

func TestWorkflowInfo_DisplaysSourceMetadata(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	source := PackSourceForState{
		Repository:  "https://github.com/example/awf-workflow-test",
		Version:     "2.5.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "source-pack", map[string]string{
		"wf": `name: wf
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source)

	output, err := runCLI(t, "workflow", "info", "source-pack")

	assert.NoError(t, err)
	assert.Contains(t, output, "example/awf-workflow-test")
	assert.Contains(t, output, "2.5.0")
}

func TestWorkflowInfo_SearchesLocalAndGlobal(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	source := PackSourceForState{
		Repository:  "https://github.com/test/pack",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	createPackStructureWithState(t, tmpDir, "global-pack", map[string]string{
		"workflow": `name: workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}, source)

	output, err := runCLI(t, "workflow", "info", "global-pack")

	assert.NoError(t, err)
	assert.Contains(t, output, "global-pack")
	assert.Contains(t, output, "1.0.0")
}

// T009: TestPluginWarnings
// Verifies that awf workflow info emits non-blocking warnings for packs with plugin dependencies.
// The warning format must include the plugin name, version constraint, and an install suggestion.
// Warnings are non-blocking — the command succeeds even when plugins are missing.
func TestPluginWarnings_WorkflowInfo_EmitsWarningWithInstallSuggestion(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	packName := "plugin-dep-pack"
	packDir := filepath.Join(tmpDir, ".awf", "workflow-packs", packName)
	require.NoError(t, os.MkdirAll(filepath.Join(packDir, "workflows"), 0o755))

	manifestContent := fmt.Sprintf(`name: %s
version: "1.0.0"
description: Pack with plugin dependencies
author: test-author
license: MIT
awf_version: ">=0.5.0"
plugins:
  my-plugin: ">=1.0.0"
workflows:
  - test-workflow
`, packName)

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifestContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "workflows", "test-workflow.yaml"), []byte(`name: test-workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`), 0o644))

	stateData := map[string]interface{}{
		"name":    packName,
		"enabled": true,
		"source_data": map[string]interface{}{
			"repository":   "https://github.com/test/pack",
			"version":      "1.0.0",
			"installed_at": time.Now(),
			"updated_at":   time.Now(),
		},
	}
	stateBytes, _ := json.MarshalIndent(stateData, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), stateBytes, 0o644))

	output, err := runCLI(t, "workflow", "info", packName)

	// Plugin warnings are non-blocking — command succeeds even with missing plugins
	assert.NoError(t, err)
	// Warning format: "Warning: pack requires plugin "my-plugin" (>=1.0.0) — install with: awf plugin install <owner>/my-plugin"
	assert.Contains(t, output, "Warning:")
	assert.Contains(t, output, "my-plugin")
	assert.Contains(t, output, "awf plugin install")
}

// T009: TestPluginWarnings_WorkflowInstall_EmitsWarning
// Verifies that awf workflow install emits plugin dependency warnings after a successful install.
// Reuses the existing mock HTTP server pattern from TestWorkflowUpdate_NewerVersionAvailable.
// This test is RED: emitPluginWarnings writes to os.Stderr, not cmd.ErrOrStderr(), so warnings
// are not captured by runCLI which sets cmd.SetErr(&out).
func TestPluginWarnings_WorkflowInstall_EmitsWarning(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	require.NoError(t, os.Mkdir(".awf", 0o755))

	// Build a tarball containing a manifest with a plugin dependency
	packName := "awf-workflow-plugin-pack"
	tarball := createPackTarballWithPlugins(t, packName, map[string]string{
		"my-plugin": ">=1.0.0",
	})

	checksumHex := sha256HexOf(tarball)
	checksumData := checksumHex + "  " + packName + "_1.0.0.tar.gz"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases"):
			releases := []map[string]interface{}{
				{
					"tag_name": "v1.0.0",
					"assets": []map[string]interface{}{
						{
							"name":                 packName + "_1.0.0.tar.gz",
							"browser_download_url": "http://" + r.Host + "/downloads/" + packName + "_1.0.0.tar.gz",
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(releases)
		case strings.Contains(r.URL.Path, "/downloads/"+packName):
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(tarball)
		case strings.Contains(r.URL.Path, "/downloads/checksums.txt"):
			_, _ = w.Write([]byte(checksumData))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	// Requires .awf/ project directory (created by setupInitTestDir via awf init)
	output, err := runCLI(t, "workflow", "install", "test-org/"+packName)

	// Plugin warnings are non-blocking — install succeeds regardless
	assert.NoError(t, err)
	// Warning must appear in captured output (requires emitPluginWarnings to write to cmd.ErrOrStderr())
	assert.Contains(t, output, "Warning:")
	assert.Contains(t, output, "my-plugin")
	assert.Contains(t, output, "awf plugin install")
}

// createPackTarballWithPlugins creates a tar.gz containing a manifest with plugin dependencies.
// Used by plugin warning tests to install a pack that requires specific plugins.
// The manifest name uses the extracted pack name (without "awf-workflow-" prefix) to match
// what the installer derives from the GitHub repo path.
func createPackTarballWithPlugins(t *testing.T, packName string, plugins map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	pluginsYAML := ""
	for name, constraint := range plugins {
		pluginsYAML += fmt.Sprintf("  %s: \"%s\"\n", name, constraint)
	}

	// The manifest name must match what extractPackName() derives from the repo path
	shortName := strings.TrimPrefix(packName, "awf-workflow-")

	manifestContent := fmt.Sprintf(`name: %s
version: "1.0.0"
description: Test pack with plugin dependencies
author: test-author
license: MIT
awf_version: ">=0.5.0"
plugins:
%sworkflows:
  - test-workflow
`, shortName, pluginsYAML)

	header := &tar.Header{
		Name: "manifest.yaml",
		Size: int64(len(manifestContent)),
		Mode: 0o644,
	}
	require.NoError(t, tw.WriteHeader(header))
	_, err := tw.Write([]byte(manifestContent))
	require.NoError(t, err)

	workflowsDir := "workflows/"
	header = &tar.Header{
		Name:     workflowsDir,
		Mode:     0o755,
		Typeflag: tar.TypeDir,
	}
	require.NoError(t, tw.WriteHeader(header))

	workflowContent := `name: test-workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`
	header = &tar.Header{
		Name: workflowsDir + "test-workflow.yaml",
		Size: int64(len(workflowContent)),
		Mode: 0o644,
	}
	require.NoError(t, tw.WriteHeader(header))
	_, err = tw.Write([]byte(workflowContent))
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	return buf.Bytes()
}

// sha256HexOf returns the hex-encoded SHA-256 hash of data.
func sha256HexOf(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// T015: Workflow Search Tests
// Tests `awf workflow search [query]` command with GitHub Search API mocking,
// rate limit handling, and formatted output.

// TestWorkflowSearch_HappyPath tests searching for workflow packs without a query.
// Should return all repositories tagged with topic:awf-workflow in table format.
func TestWorkflowSearch_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify correct GitHub Search API endpoint and topic
		if !strings.Contains(r.URL.RawQuery, "topic%3Aawf-workflow") {
			http.Error(w, "expected topic:awf-workflow", http.StatusBadRequest)
			return
		}

		searchResp := map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"full_name":        "acme/awf-workflow-speckit",
					"description":      "SpecKit workflow automation pack",
					"stargazers_count": 42,
				},
				{
					"full_name":        "example/awf-workflow-data",
					"description":      "Data pipeline workflows",
					"stargazers_count": 15,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResp)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	output, err := runCLI(t, "workflow", "search")

	require.NoError(t, err)
	assert.Contains(t, output, "acme/awf-workflow-speckit")
	assert.Contains(t, output, "example/awf-workflow-data")
	assert.Contains(t, output, "SpecKit workflow automation pack")
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "STARS")
}

// TestWorkflowSearch_WithQuery filters search results by optional query parameter.
// Should append the query to the search string and return filtered results.
func TestWorkflowSearch_WithQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query parameter is included in search
		if !strings.Contains(r.URL.RawQuery, "speckit") {
			http.Error(w, "expected query parameter", http.StatusBadRequest)
			return
		}

		searchResp := map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"full_name":        "acme/awf-workflow-speckit",
					"description":      "SpecKit workflow automation pack",
					"stargazers_count": 42,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResp)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	output, err := runCLI(t, "workflow", "search", "speckit")

	require.NoError(t, err)
	assert.Contains(t, output, "acme/awf-workflow-speckit")
	assert.Contains(t, output, "SpecKit workflow automation pack")
}

// TestWorkflowSearch_NoResults handles case when no workflow packs match the query.
// Should return friendly "No workflows found" message.
func TestWorkflowSearch_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchResp := map[string]interface{}{
			"items": []interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResp)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	output, err := runCLI(t, "workflow", "search", "nonexistent-pack")

	require.NoError(t, err)
	assert.Contains(t, output, "No")
	assert.Contains(t, output, "found")
}

// TestWorkflowSearch_JSONOutput validates JSON formatted output.
// Should encode search results as JSON array when output format is json.
func TestWorkflowSearch_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchResp := map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"full_name":        "acme/awf-workflow-speckit",
					"description":      "SpecKit automation",
					"stargazers_count": 42,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResp)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	output, err := runCLI(t, "workflow", "search", "--output=json")

	require.NoError(t, err)

	var results []map[string]interface{}
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err, "output should be valid JSON")
	require.Len(t, results, 1)
	assert.Equal(t, "acme/awf-workflow-speckit", results[0]["full_name"])
}

// TestWorkflowSearch_RateLimitError returns actionable error when GitHub API rate limit is hit.
// Should check X-Ratelimit-Remaining header and provide guidance to set GITHUB_TOKEN.
func TestWorkflowSearch_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message": "API rate limit exceeded"}`))
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	output, err := runCLI(t, "workflow", "search")

	require.Error(t, err)
	assert.Contains(t, output, "rate limit")
	assert.Contains(t, output, "GITHUB_TOKEN")
}

// TestWorkflowSearch_InvalidResponse handles malformed GitHub API responses.
// Should return error when response body is not valid JSON.
func TestWorkflowSearch_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	output, err := runCLI(t, "workflow", "search")

	require.Error(t, err)
	assert.Contains(t, output, "parse")
}

// TestWorkflowSearch_APIError handles non-200 HTTP status codes from GitHub.
// Should return error indicating the HTTP status code received.
func TestWorkflowSearch_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "Internal Server Error"}`))
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	output, err := runCLI(t, "workflow", "search")

	require.Error(t, err)
	assert.Contains(t, output, "500")
}
