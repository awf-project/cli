package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWorkflowNamespace(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantPackName     string
		wantWorkflowName string
	}{
		{
			name:             "pack namespace with slash",
			input:            "speckit/specify",
			wantPackName:     "speckit",
			wantWorkflowName: "specify",
		},
		{
			name:             "local workflow without slash",
			input:            "my-workflow",
			wantPackName:     "",
			wantWorkflowName: "my-workflow",
		},
		{
			name:             "multiple slashes splits on first only",
			input:            "speckit/workflows/specify",
			wantPackName:     "speckit",
			wantWorkflowName: "workflows/specify",
		},
		{
			name:             "empty string returns empty pack and workflow",
			input:            "",
			wantPackName:     "",
			wantWorkflowName: "",
		},
		{
			name:             "single slash with content after",
			input:            "pack/",
			wantPackName:     "pack",
			wantWorkflowName: "",
		},
		{
			name:             "single slash with content before",
			input:            "/workflow",
			wantPackName:     "",
			wantWorkflowName: "workflow",
		},
		{
			name:             "simple pack name with hyphens and numbers",
			input:            "my-pack-123/my-workflow",
			wantPackName:     "my-pack-123",
			wantWorkflowName: "my-workflow",
		},
		{
			name:             "local workflow with hyphens and underscores",
			input:            "my_workflow-2024",
			wantPackName:     "",
			wantWorkflowName: "my_workflow-2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packName, workflowName := parseWorkflowNamespace(tt.input)

			assert.Equal(t, tt.wantPackName, packName)
			assert.Equal(t, tt.wantWorkflowName, workflowName)
		})
	}
}

func TestParseWorkflowNamespace_NamespacePresence(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		hasPackName bool
	}{
		{
			name:        "namespace present",
			input:       "speckit/specify",
			hasPackName: true,
		},
		{
			name:        "no namespace",
			input:       "my-workflow",
			hasPackName: false,
		},
		{
			name:        "empty input has no namespace",
			input:       "",
			hasPackName: false,
		},
		{
			name:        "slash only makes pack name empty",
			input:       "/workflow",
			hasPackName: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packName, _ := parseWorkflowNamespace(tt.input)

			if tt.hasPackName {
				assert.NotEmpty(t, packName)
			} else {
				assert.Empty(t, packName)
			}
		})
	}
}

func TestValidatePackWorkflow(t *testing.T) {
	tests := []struct {
		name         string
		setupDir     func(t *testing.T) string
		packDir      string
		workflowName string
		wantErr      bool
		errContains  string
	}{
		{
			name: "valid workflow in manifest",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				createTestManifest(t, dir, `
name: speckit
version: "1.0.0"
description: "Test pack"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - specify
  - clarify
  - plan
`)
				return dir
			},
			workflowName: "specify",
			wantErr:      false,
		},
		{
			name: "workflow not listed in manifest",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				createTestManifest(t, dir, `
name: speckit
version: "1.0.0"
description: "Test pack"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - specify
  - clarify
`)
				return dir
			},
			workflowName: "internal-wf",
			wantErr:      true,
			errContains:  "USER.INPUT.VALIDATION_FAILED",
		},
		{
			name:         "manifest.yaml missing",
			setupDir:     func(t *testing.T) string { return t.TempDir() },
			workflowName: "specify",
			wantErr:      true,
		},
		{
			name: "path traversal attempt with ..",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				createTestManifest(t, dir, `
name: speckit
version: "1.0.0"
description: "Test pack"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - specify
`)
				return dir
			},
			workflowName: "../etc/passwd",
			wantErr:      true,
		},
		{
			name: "path traversal attempt with ../ at start",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				createTestManifest(t, dir, `
name: speckit
version: "1.0.0"
description: "Test pack"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - specify
`)
				return dir
			},
			workflowName: "../../../../../../etc/passwd",
			wantErr:      true,
		},
		{
			name: "multiple workflows in manifest",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				createTestManifest(t, dir, `
name: speckit
version: "1.0.0"
description: "Test pack"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - specify
  - clarify
  - plan
  - validate
  - document
`)
				return dir
			},
			workflowName: "validate",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.packDir = tt.setupDir(t)
			err := validatePackWorkflow(tt.packDir, tt.workflowName)

			if tt.wantErr {
				assert.Error(t, err)
				if err != nil && tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResolvePackDir(t *testing.T) {
	tests := []struct {
		name        string
		setupDirs   func(t *testing.T) (local, global string)
		packName    string
		wantPackDir string
		wantErr     bool
		errContains string
	}{
		{
			name: "pack found in local directory",
			setupDirs: func(t *testing.T) (local, global string) {
				local = t.TempDir()
				global = t.TempDir()
				packLocalDir := filepath.Join(local, "speckit")
				os.Mkdir(packLocalDir, 0o755)
				return local, global
			},
			packName: "speckit",
			wantErr:  false,
		},
		{
			name: "pack found in global directory when not in local",
			setupDirs: func(t *testing.T) (local, global string) {
				local = t.TempDir()
				global = t.TempDir()
				packGlobalDir := filepath.Join(global, "speckit")
				os.Mkdir(packGlobalDir, 0o755)
				return local, global
			},
			packName: "speckit",
			wantErr:  false,
		},
		{
			name: "local pack takes precedence over global",
			setupDirs: func(t *testing.T) (local, global string) {
				local = t.TempDir()
				global = t.TempDir()
				packLocalDir := filepath.Join(local, "speckit")
				packGlobalDir := filepath.Join(global, "speckit")
				os.Mkdir(packLocalDir, 0o755)
				os.Mkdir(packGlobalDir, 0o755)
				return local, global
			},
			packName: "speckit",
			wantErr:  false,
		},
		{
			name: "pack not found in either directory",
			setupDirs: func(t *testing.T) (local, global string) {
				local = t.TempDir()
				global = t.TempDir()
				return local, global
			},
			packName: "nonexistent",
			wantErr:  true,
		},
		{
			name: "path traversal attempt rejected",
			setupDirs: func(t *testing.T) (local, global string) {
				local = t.TempDir()
				global = t.TempDir()
				return local, global
			},
			packName: "../etc/passwd",
			wantErr:  true,
		},
		{
			name: "path traversal with multiple dots rejected",
			setupDirs: func(t *testing.T) (local, global string) {
				local = t.TempDir()
				global = t.TempDir()
				return local, global
			},
			packName: "../../../../../../etc/passwd",
			wantErr:  true,
		},
		{
			name: "pack with hyphens and numbers resolved",
			setupDirs: func(t *testing.T) (local, global string) {
				local = t.TempDir()
				global = t.TempDir()
				packLocalDir := filepath.Join(local, "my-pack-123")
				os.Mkdir(packLocalDir, 0o755)
				return local, global
			},
			packName: "my-pack-123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local, global := tt.setupDirs(t)
			result, err := resolvePackDir(tt.packName, local, global)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
				absResult, _ := filepath.Abs(result)
				assert.Equal(t, absResult, result)
			}
		})
	}
}

func TestResolvePackDir_AbsolutePath(t *testing.T) {
	tests := []struct {
		name     string
		setupDir func(t *testing.T) (local, global string)
		packName string
	}{
		{
			name: "returns absolute path for local pack",
			setupDir: func(t *testing.T) (local, global string) {
				local = t.TempDir()
				global = t.TempDir()
				packDir := filepath.Join(local, "speckit")
				os.Mkdir(packDir, 0o755)
				return local, global
			},
			packName: "speckit",
		},
		{
			name: "returns absolute path for global pack",
			setupDir: func(t *testing.T) (local, global string) {
				local = t.TempDir()
				global = t.TempDir()
				packDir := filepath.Join(global, "speckit")
				os.Mkdir(packDir, 0o755)
				return local, global
			},
			packName: "speckit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local, global := tt.setupDir(t)
			result, err := resolvePackDir(tt.packName, local, global)

			assert.NoError(t, err)
			assert.True(t, filepath.IsAbs(result))
		})
	}
}

func createTestManifest(t *testing.T, packDir, content string) {
	t.Helper()
	manifestPath := packDir + "/manifest.yaml"
	err := os.WriteFile(manifestPath, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("failed to create test manifest: %v", err)
	}
}

func TestResolvePackWorkflow_HappyPath(t *testing.T) {
	ctx := context.Background()

	// Setup pack directory with manifest and workflow file
	tempDir := t.TempDir()
	local := filepath.Join(tempDir, "local")
	global := filepath.Join(tempDir, "global")
	os.Mkdir(local, 0o755)
	os.Mkdir(global, 0o755)

	packDir := filepath.Join(local, "speckit")
	os.Mkdir(packDir, 0o755)

	// Create manifest
	createTestManifest(t, packDir, `
name: speckit
version: "1.0.0"
description: "Test pack"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - specify
  - clarify
`)

	// Create workflow file with proper YAML structure
	workflowsDir := filepath.Join(packDir, "workflows")
	os.Mkdir(workflowsDir, 0o755)
	workflowYAML := `name: specify
version: "1.0.0"
states:
  initial: agent_step
  agent_step:
    type: agent
    provider: claude
    prompt: "test"
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(workflowsDir, "specify.yaml"), []byte(workflowYAML), 0o644)
	require.NoError(t, err)

	// Execute
	wf, resolvedPackDir, err := resolvePackWorkflow(ctx, "speckit", "specify", local, global)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, wf)
	assert.Equal(t, "specify", wf.Name)
	assert.True(t, filepath.IsAbs(resolvedPackDir))
	absPackDir, _ := filepath.Abs(packDir)
	assert.Equal(t, absPackDir, resolvedPackDir)
}

func TestResolvePackWorkflow_PackNotFound(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	local := filepath.Join(tempDir, "local")
	global := filepath.Join(tempDir, "global")
	os.Mkdir(local, 0o755)
	os.Mkdir(global, 0o755)

	wf, packDir, err := resolvePackWorkflow(ctx, "nonexistent", "workflow", local, global)

	assert.Error(t, err)
	assert.Nil(t, wf)
	assert.Empty(t, packDir)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolvePackWorkflow_WorkflowNotListed(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	local := filepath.Join(tempDir, "local")
	global := filepath.Join(tempDir, "global")
	os.Mkdir(local, 0o755)
	os.Mkdir(global, 0o755)

	packDir := filepath.Join(local, "speckit")
	os.Mkdir(packDir, 0o755)

	// Create manifest without the requested workflow
	createTestManifest(t, packDir, `
name: speckit
version: "1.0.0"
description: "Test pack"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - clarify
  - plan
`)

	wf, _, err := resolvePackWorkflow(ctx, "speckit", "specify", local, global)

	assert.Error(t, err)
	assert.Nil(t, wf)
	assert.Contains(t, err.Error(), "VALIDATION_FAILED")
}

func TestResolvePackWorkflow_WorkflowFileNotFound(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	local := filepath.Join(tempDir, "local")
	global := filepath.Join(tempDir, "global")
	os.Mkdir(local, 0o755)
	os.Mkdir(global, 0o755)

	packDir := filepath.Join(local, "speckit")
	os.Mkdir(packDir, 0o755)

	// Create manifest that lists the workflow
	createTestManifest(t, packDir, `
name: speckit
version: "1.0.0"
description: "Test pack"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - specify
`)

	// Create workflows dir but don't create the specify.yaml file
	workflowsDir := filepath.Join(packDir, "workflows")
	os.Mkdir(workflowsDir, 0o755)

	wf, _, err := resolvePackWorkflow(ctx, "speckit", "specify", local, global)

	assert.Error(t, err)
	assert.Nil(t, wf)
}

func TestResolvePackWorkflow_GlobalPackFallback(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	local := filepath.Join(tempDir, "local")
	global := filepath.Join(tempDir, "global")
	os.Mkdir(local, 0o755)
	os.Mkdir(global, 0o755)

	// Create pack only in global directory
	packDir := filepath.Join(global, "speckit")
	os.Mkdir(packDir, 0o755)

	createTestManifest(t, packDir, `
name: speckit
version: "1.0.0"
description: "Test pack"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - specify
`)

	workflowsDir := filepath.Join(packDir, "workflows")
	os.Mkdir(workflowsDir, 0o755)
	workflowYAML := `name: specify
version: "1.0.0"
states:
  initial: agent_step
  agent_step:
    type: agent
    provider: claude
    prompt: "test"
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(workflowsDir, "specify.yaml"), []byte(workflowYAML), 0o644)
	require.NoError(t, err)

	wf, _, err := resolvePackWorkflow(ctx, "speckit", "specify", local, global)

	assert.NoError(t, err)
	assert.NotNil(t, wf)
	assert.Equal(t, "specify", wf.Name)
}

func TestBuildPackAWFPaths_IncludesPackName(t *testing.T) {
	paths := xdg.PackAWFPaths("speckit")

	assert.NotNil(t, paths)
	assert.Equal(t, "speckit", paths["pack_name"])
}

func TestBuildPackAWFPaths_IncludesAllStandardPaths(t *testing.T) {
	paths := xdg.PackAWFPaths("speckit")

	standardKeys := []string{
		"prompts_dir",
		"scripts_dir",
		"config_dir",
		"data_dir",
		"workflows_dir",
		"plugins_dir",
		"pack_name",
	}

	for _, key := range standardKeys {
		assert.Contains(t, paths, key)
	}
}

func TestBuildPackAWFPaths_PathsNotEmpty(t *testing.T) {
	paths := xdg.PackAWFPaths("test-pack")

	// All paths except pack_name should be non-empty XDG paths
	assert.NotEmpty(t, paths["prompts_dir"])
	assert.NotEmpty(t, paths["scripts_dir"])
	assert.NotEmpty(t, paths["config_dir"])
	assert.NotEmpty(t, paths["data_dir"])
	assert.NotEmpty(t, paths["workflows_dir"])
	assert.NotEmpty(t, paths["plugins_dir"])
	assert.Equal(t, "test-pack", paths["pack_name"])
}

func TestBuildPackAWFPaths_DifferentPackNames(t *testing.T) {
	tests := []struct {
		name       string
		packName   string
		expectedPN string
	}{
		{
			name:       "simple pack name",
			packName:   "speckit",
			expectedPN: "speckit",
		},
		{
			name:       "pack with hyphens",
			packName:   "my-pack-123",
			expectedPN: "my-pack-123",
		},
		{
			name:       "empty pack name",
			packName:   "",
			expectedPN: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := xdg.PackAWFPaths(tt.packName)

			assert.Equal(t, tt.expectedPN, paths["pack_name"])
		})
	}
}
