package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadExternalFile_HappyPath(t *testing.T) {
	tests := []struct {
		name            string
		fileContent     string
		setupFile       func(t *testing.T, dir string) string
		awfContext      map[string]string
		expectedContent string
	}{
		{
			name:        "relative path to external file",
			fileContent: "#!/bin/bash\necho 'test script'",
			setupFile: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "scripts", "deploy.sh")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("#!/bin/bash\necho 'test script'"), 0o644))
				return "scripts/deploy.sh"
			},
			expectedContent: "#!/bin/bash\necho 'test script'",
		},
		{
			name:        "absolute path to external file",
			fileContent: "#!/bin/sh\nls -la",
			setupFile: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "absolute.sh")
				require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nls -la"), 0o644))
				return path
			},
			expectedContent: "#!/bin/sh\nls -la",
		},
		{
			name:        "nested directory structure",
			fileContent: "build commands here",
			setupFile: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "scripts", "ci", "build", "step-02.sh")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("build commands here"), 0o644))
				return "scripts/ci/build/step-02.sh"
			},
			expectedContent: "build commands here",
		},
		{
			name:        "file with unicode content",
			fileContent: "#!/bin/bash\n# 日本語コメント\necho 'テスト'",
			setupFile: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "unicode.sh")
				require.NoError(t, os.WriteFile(path, []byte("#!/bin/bash\n# 日本語コメント\necho 'テスト'"), 0o644))
				return "unicode.sh"
			},
			expectedContent: "#!/bin/bash\n# 日本語コメント\necho 'テスト'",
		},
		{
			name:        "template variable in path - awf.scripts_dir",
			fileContent: "Script from AWF scripts directory",
			setupFile: func(t *testing.T, dir string) string {
				scriptsDir := filepath.Join(dir, "awf-scripts")
				path := filepath.Join(scriptsDir, "checks", "lint.sh")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("Script from AWF scripts directory"), 0o644))
				return "{{.awf.scripts_dir}}/checks/lint.sh"
			},
			awfContext: map[string]string{
				"scripts_dir": "",
			},
			expectedContent: "Script from AWF scripts directory",
		},
		{
			name:        "large file under 1MB limit",
			fileContent: strings.Repeat("a", 1024*500),
			setupFile: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "large.sh")
				require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("a", 1024*500)), 0o644))
				return "large.sh"
			},
			expectedContent: strings.Repeat("a", 1024*500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			filePath := tt.setupFile(t, tmpDir)
			if tt.awfContext != nil && strings.Contains(filePath, "{{.awf.scripts_dir}}") {
				scriptsDir := filepath.Join(tmpDir, "awf-scripts")
				tt.awfContext["scripts_dir"] = scriptsDir
			}

			wf := &workflow.Workflow{
				Name:      "test-workflow",
				SourceDir: tmpDir,
			}

			intCtx := &interpolation.Context{
				AWF: tt.awfContext,
			}

			result, err := loadExternalFile(context.Background(), filePath, wf, intCtx)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedContent, result)
		})
	}
}

func TestLoadExternalFile_TildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "custom", "script.sh")
	require.NoError(t, os.MkdirAll(filepath.Dir(scriptPath), 0o755))
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'custom'"), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	result, err := loadExternalFile(context.Background(), "~/custom/script.sh", wf, &interpolation.Context{})

	require.NoError(t, err)
	assert.Contains(t, result, "echo 'custom'")
}

func TestLoadExternalFile_TemplateInterpolationInPath(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	scriptPath := filepath.Join(configDir, "scripts", "test-feature.sh")
	require.NoError(t, os.MkdirAll(filepath.Dir(scriptPath), 0o755))
	require.NoError(t, os.WriteFile(scriptPath, []byte("Feature-specific script"), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{
		Inputs: map[string]any{
			"feature": "test-feature",
		},
		AWF: map[string]string{
			"config_dir": configDir,
		},
	}

	scriptFile := "{{.awf.config_dir}}/scripts/{{.inputs.feature}}.sh"

	result, err := loadExternalFile(context.Background(), scriptFile, wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "Feature-specific script", result)
}

func TestLoadExternalFile_SizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	oversizedContent := strings.Repeat("a", 1024*1024+1)
	scriptPath := filepath.Join(tmpDir, "oversized.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(oversizedContent), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	_, err := loadExternalFile(context.Background(), "oversized.sh", wf, intCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds 1MB limit")
}

func TestLoadExternalFile_FileNotFound(t *testing.T) {
	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: t.TempDir(),
	}

	_, err := loadExternalFile(context.Background(), "nonexistent.sh", wf, &interpolation.Context{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "nonexistent.sh")
}

func TestLoadExternalFile_DirectoryInsteadOfFile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "scripts"), 0o755))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	_, err := loadExternalFile(context.Background(), "scripts", wf, &interpolation.Context{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")
}

func TestLoadExternalFile_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "unreadable.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("secret script"), 0o000))
	defer os.Chmod(scriptPath, 0o644)

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	_, err := loadExternalFile(context.Background(), "unreadable.sh", wf, &interpolation.Context{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestLoadExternalFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "empty.sh"), []byte(""), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	result, err := loadExternalFile(context.Background(), "empty.sh", wf, &interpolation.Context{})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestLoadExternalFile_WhitespaceOnlyFile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "whitespace.sh"), []byte("   \n\t\n   "), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	result, err := loadExternalFile(context.Background(), "whitespace.sh", wf, &interpolation.Context{})

	require.NoError(t, err)
	assert.Equal(t, "   \n\t\n   ", result)
}

func TestLoadExternalFile_PathWithSpaces(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "my script file.sh"), []byte("Content with spaces in path"), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	result, err := loadExternalFile(context.Background(), "my script file.sh", wf, &interpolation.Context{})

	require.NoError(t, err)
	assert.Equal(t, "Content with spaces in path", result)
}

func TestLoadExternalFile_RelativePathResolution(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, "workflows")
	scriptsDir := filepath.Join(tmpDir, "scripts")
	require.NoError(t, os.MkdirAll(workflowDir, 0o755))
	require.NoError(t, os.MkdirAll(scriptsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(scriptsDir, "build.sh"), []byte("Shared script"), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: workflowDir,
	}

	result, err := loadExternalFile(context.Background(), "../scripts/build.sh", wf, &interpolation.Context{})

	require.NoError(t, err)
	assert.Equal(t, "Shared script", result)
}

func TestLoadExternalFile_MultipleAWFVariables(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	dataDir := filepath.Join(tmpDir, "data")
	scriptPath := filepath.Join(configDir, dataDir, "script.sh")
	require.NoError(t, os.MkdirAll(filepath.Dir(scriptPath), 0o755))
	require.NoError(t, os.WriteFile(scriptPath, []byte("Multi-var script"), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{
		AWF: map[string]string{
			"config_dir": configDir,
			"data_dir":   dataDir,
		},
	}

	scriptFile := "{{.awf.config_dir}}/{{.awf.data_dir}}/script.sh"

	result, err := loadExternalFile(context.Background(), scriptFile, wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "Multi-var script", result)
}

func TestLoadExternalFile_BoundarySize(t *testing.T) {
	tmpDir := t.TempDir()
	exactContent := strings.Repeat("a", 1024*1024)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "exact-1mb.sh"), []byte(exactContent), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	result, err := loadExternalFile(context.Background(), "exact-1mb.sh", wf, &interpolation.Context{})

	require.NoError(t, err)
	assert.Equal(t, exactContent, result)
}

func TestLoadExternalFile_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.sh"), []byte("test content"), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := loadExternalFile(ctx, "test.sh", wf, &interpolation.Context{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestLoadExternalFile_InterpolationError(t *testing.T) {
	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: t.TempDir(),
	}

	intCtx := &interpolation.Context{
		Inputs: map[string]any{},
	}

	_, err := loadExternalFile(context.Background(), "{{.inputs.nonexistent}}/script.sh", wf, intCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "interpolate")
}

func TestLoadExternalFile_LocalOverridesGlobalScriptsDir(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()
	sourceDir := filepath.Join(projectDir, "workflows")
	require.NoError(t, os.MkdirAll(sourceDir, 0o755))

	globalScriptsDir := filepath.Join(globalDir, "scripts")
	globalFile := filepath.Join(globalScriptsDir, "deploy.sh")
	require.NoError(t, os.MkdirAll(globalScriptsDir, 0o755))
	require.NoError(t, os.WriteFile(globalFile, []byte("global content"), 0o644))

	localFile := filepath.Join(projectDir, "scripts", "deploy.sh")
	require.NoError(t, os.MkdirAll(filepath.Dir(localFile), 0o755))
	require.NoError(t, os.WriteFile(localFile, []byte("local content"), 0o644))

	wf := &workflow.Workflow{Name: "test", SourceDir: sourceDir}
	intCtx := &interpolation.Context{
		AWF: map[string]string{"scripts_dir": globalScriptsDir},
	}

	result, err := loadExternalFile(context.Background(), "{{.awf.scripts_dir}}/deploy.sh", wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "local content", result, "local script should override global")
}

func TestLoadExternalFile_FallbackToGlobalScriptsDir(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()
	sourceDir := filepath.Join(projectDir, "workflows")
	require.NoError(t, os.MkdirAll(sourceDir, 0o755))

	globalScriptsDir := filepath.Join(globalDir, "scripts")
	globalFile := filepath.Join(globalScriptsDir, "deploy.sh")
	require.NoError(t, os.MkdirAll(globalScriptsDir, 0o755))
	require.NoError(t, os.WriteFile(globalFile, []byte("global content"), 0o644))

	wf := &workflow.Workflow{Name: "test", SourceDir: sourceDir}
	intCtx := &interpolation.Context{
		AWF: map[string]string{"scripts_dir": globalScriptsDir},
	}

	result, err := loadExternalFile(context.Background(), "{{.awf.scripts_dir}}/deploy.sh", wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "global content", result, "should fallback to global when no local exists")
}

func TestLoadExternalFile_LocalOverridesGlobalPromptsDir(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()
	sourceDir := filepath.Join(projectDir, "workflows")
	require.NoError(t, os.MkdirAll(sourceDir, 0o755))

	globalPromptsDir := filepath.Join(globalDir, "prompts")
	globalFile := filepath.Join(globalPromptsDir, "system.md")
	require.NoError(t, os.MkdirAll(globalPromptsDir, 0o755))
	require.NoError(t, os.WriteFile(globalFile, []byte("global prompt"), 0o644))

	localFile := filepath.Join(projectDir, "prompts", "system.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(localFile), 0o755))
	require.NoError(t, os.WriteFile(localFile, []byte("local prompt"), 0o644))

	wf := &workflow.Workflow{Name: "test", SourceDir: sourceDir}
	intCtx := &interpolation.Context{
		AWF: map[string]string{"prompts_dir": globalPromptsDir},
	}

	result, err := loadExternalFile(context.Background(), "{{.awf.prompts_dir}}/system.md", wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "local prompt", result, "local prompt should override global")
}

func TestResolveLocalOverGlobal(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, projectDir, globalDir string) (interpolatedPath string, awfMap map[string]string)
		wantPath string // "local" = under projectDir, "unchanged" = same as interpolatedPath
	}{
		{
			name: "local scripts_dir overrides global when local file exists",
			setup: func(t *testing.T, projectDir, globalDir string) (string, map[string]string) {
				globalFile := filepath.Join(globalDir, "scripts", "deploy.sh")
				require.NoError(t, os.MkdirAll(filepath.Dir(globalFile), 0o755))
				require.NoError(t, os.WriteFile(globalFile, []byte("global"), 0o644))

				localPath := filepath.Join(projectDir, "scripts", "deploy.sh")
				require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0o755))
				require.NoError(t, os.WriteFile(localPath, []byte("local"), 0o644))

				return globalFile, map[string]string{"scripts_dir": filepath.Join(globalDir, "scripts")}
			},
			wantPath: "local",
		},
		{
			name: "local prompts_dir overrides global when local file exists",
			setup: func(t *testing.T, projectDir, globalDir string) (string, map[string]string) {
				globalFile := filepath.Join(globalDir, "prompts", "greet.md")
				require.NoError(t, os.MkdirAll(filepath.Dir(globalFile), 0o755))
				require.NoError(t, os.WriteFile(globalFile, []byte("global"), 0o644))

				localPath := filepath.Join(projectDir, "prompts", "greet.md")
				require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0o755))
				require.NoError(t, os.WriteFile(localPath, []byte("local"), 0o644))

				return globalFile, map[string]string{"prompts_dir": filepath.Join(globalDir, "prompts")}
			},
			wantPath: "local",
		},
		{
			name: "fallback to global scripts_dir when no local file",
			setup: func(t *testing.T, projectDir, globalDir string) (string, map[string]string) {
				globalFile := filepath.Join(globalDir, "scripts", "deploy.sh")
				require.NoError(t, os.MkdirAll(filepath.Dir(globalFile), 0o755))
				require.NoError(t, os.WriteFile(globalFile, []byte("global"), 0o644))

				return globalFile, map[string]string{"scripts_dir": filepath.Join(globalDir, "scripts")}
			},
			wantPath: "unchanged",
		},
		{
			name: "absolute non-XDG path is unchanged",
			setup: func(t *testing.T, projectDir, globalDir string) (string, map[string]string) {
				otherFile := filepath.Join(globalDir, "other.sh")
				require.NoError(t, os.WriteFile(otherFile, []byte("other"), 0o644))

				return otherFile, map[string]string{"scripts_dir": filepath.Join(globalDir, "scripts")}
			},
			wantPath: "unchanged",
		},
		{
			name: "nested subpath preserves structure in local resolution",
			setup: func(t *testing.T, projectDir, globalDir string) (string, map[string]string) {
				globalFile := filepath.Join(globalDir, "scripts", "ci", "build.sh")
				require.NoError(t, os.MkdirAll(filepath.Dir(globalFile), 0o755))
				require.NoError(t, os.WriteFile(globalFile, []byte("global"), 0o644))

				localPath := filepath.Join(projectDir, "scripts", "ci", "build.sh")
				require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0o755))
				require.NoError(t, os.WriteFile(localPath, []byte("local"), 0o644))

				return filepath.Join(globalDir, "scripts", "ci", "build.sh"),
					map[string]string{"scripts_dir": filepath.Join(globalDir, "scripts")}
			},
			wantPath: "local",
		},
		{
			name: "empty AWF map returns path unchanged",
			setup: func(t *testing.T, projectDir, globalDir string) (string, map[string]string) {
				otherFile := filepath.Join(globalDir, "file.sh")
				require.NoError(t, os.WriteFile(otherFile, []byte("content"), 0o644))

				return otherFile, map[string]string{}
			},
			wantPath: "unchanged",
		},
		{
			name: "nil AWF map returns path unchanged",
			setup: func(t *testing.T, projectDir, globalDir string) (string, map[string]string) {
				otherFile := filepath.Join(globalDir, "file.sh")
				require.NoError(t, os.WriteFile(otherFile, []byte("content"), 0o644))

				return otherFile, nil
			},
			wantPath: "unchanged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			globalDir := t.TempDir()
			// sourceDir simulates .awf/workflows/ — parent is where local scripts/prompts live
			sourceDir := filepath.Join(projectDir, "workflows")
			require.NoError(t, os.MkdirAll(sourceDir, 0o755))

			interpolatedPath, awfMap := tt.setup(t, projectDir, globalDir)

			result := resolveLocalOverGlobal(interpolatedPath, sourceDir, awfMap)

			switch tt.wantPath {
			case "local":
				assert.NotEqual(t, interpolatedPath, result, "expected local path, got unchanged global path")
				assert.True(t, strings.HasPrefix(result, projectDir), "expected path under projectDir, got: %s", result)
				_, statErr := os.Stat(result)
				assert.NoError(t, statErr, "resolved local path must exist")
			case "unchanged":
				assert.Equal(t, interpolatedPath, result, "expected path unchanged")
			}
		})
	}
}

// T005: 3-tier pack path resolution tests

func TestResolvePackPathTiers_Tier1_UserOverride(t *testing.T) {
	tmpDir := t.TempDir()

	tier1Dir := filepath.Join(tmpDir, ".awf", "prompts", "speckit")
	require.NoError(t, os.MkdirAll(tier1Dir, 0o755))

	tier1File := filepath.Join(tier1Dir, "example.txt")
	require.NoError(t, os.WriteFile(tier1File, []byte("tier 1 - user override"), 0o644))

	sourceDir := filepath.Join(tmpDir, "workflow.yaml")

	resolved := resolvePackPathTiers("example.txt", "prompts_dir", "speckit", sourceDir)

	assert.Equal(t, tier1File, resolved, "should resolve to tier 1 user override")
}

func TestResolvePackPathTiers_Tier2_PackEmbedded(t *testing.T) {
	tmpDir := t.TempDir()

	packRoot := filepath.Join(tmpDir, "packs", "speckit")
	tier2Dir := filepath.Join(packRoot, "prompts")
	require.NoError(t, os.MkdirAll(tier2Dir, 0o755))

	tier2File := filepath.Join(tier2Dir, "example.txt")
	require.NoError(t, os.WriteFile(tier2File, []byte("tier 2 - pack embedded"), 0o644))

	sourceDir := filepath.Join(packRoot, "workflows", "spec.yaml")

	resolved := resolvePackPathTiers("example.txt", "prompts_dir", "speckit", sourceDir)

	assert.Equal(t, tier2File, resolved, "should resolve to tier 2 pack embedded")
}

func TestResolvePackPathTiers_Tier1PrecedeseTier2(t *testing.T) {
	tmpDir := t.TempDir()

	tier1Dir := filepath.Join(tmpDir, ".awf", "prompts", "speckit")
	require.NoError(t, os.MkdirAll(tier1Dir, 0o755))
	tier1File := filepath.Join(tier1Dir, "example.txt")
	require.NoError(t, os.WriteFile(tier1File, []byte("tier 1"), 0o644))

	packRoot := filepath.Join(tmpDir, "packs", "speckit")
	tier2Dir := filepath.Join(packRoot, "prompts")
	require.NoError(t, os.MkdirAll(tier2Dir, 0o755))
	tier2File := filepath.Join(tier2Dir, "example.txt")
	require.NoError(t, os.WriteFile(tier2File, []byte("tier 2"), 0o644))

	sourceDir := filepath.Join(packRoot, "workflows", "spec.yaml")

	resolved := resolvePackPathTiers("example.txt", "prompts_dir", "speckit", sourceDir)

	assert.Equal(t, tier1File, resolved, "tier 1 should take precedence over tier 2")
}

func TestResolvePackPathTiers_NoTierExists(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "workflow.yaml")

	resolved := resolvePackPathTiers("missing.txt", "prompts_dir", "speckit", sourceDir)

	assert.Equal(t, "", resolved, "should return empty string when no tiers exist")
}

func TestResolvePackPathTiers_NestedPath(t *testing.T) {
	tmpDir := t.TempDir()

	tier1Dir := filepath.Join(tmpDir, ".awf", "scripts", "mypack")
	require.NoError(t, os.MkdirAll(tier1Dir, 0o755))

	nestedPath := filepath.Join(tier1Dir, "ci", "build.sh")
	require.NoError(t, os.MkdirAll(filepath.Dir(nestedPath), 0o755))
	require.NoError(t, os.WriteFile(nestedPath, []byte("build script"), 0o755))

	sourceDir := filepath.Join(tmpDir, "workflow.yaml")

	resolved := resolvePackPathTiers("ci/build.sh", "scripts_dir", "mypack", sourceDir)

	assert.Equal(t, nestedPath, resolved, "should resolve nested paths correctly")
}

func TestExtractFilePathSuffix_SimpleFile(t *testing.T) {
	suffix := extractFilePathSuffix("script.sh arg1 arg2")
	assert.Equal(t, "script.sh", suffix)
}

func TestExtractFilePathSuffix_PathWithDirs(t *testing.T) {
	suffix := extractFilePathSuffix("subdir/script.sh | other")
	assert.Equal(t, "subdir/script.sh", suffix)
}

func TestExtractFilePathSuffix_QuoteBoundary(t *testing.T) {
	suffix := extractFilePathSuffix(`script.sh" args`)
	assert.Equal(t, "script.sh", suffix)
}

func TestExtractFilePathSuffix_PipeBoundary(t *testing.T) {
	suffix := extractFilePathSuffix("script.sh | cat file")
	assert.Equal(t, "script.sh", suffix)
}

func TestExtractFilePathSuffix_AmpersandBoundary(t *testing.T) {
	suffix := extractFilePathSuffix("script.sh & other_cmd")
	assert.Equal(t, "script.sh", suffix)
}

func TestExtractFilePathSuffix_EmptyString(t *testing.T) {
	suffix := extractFilePathSuffix("")
	assert.Equal(t, "", suffix)
}

func TestExtractFilePathSuffix_NoDelimiter(t *testing.T) {
	suffix := extractFilePathSuffix("script.sh")
	assert.Equal(t, "script.sh", suffix)
}

func TestReplaceAWFPathsInString_SingleReplacement(t *testing.T) {
	tmpDir := t.TempDir()

	localDir := filepath.Join(tmpDir, "scripts")
	require.NoError(t, os.MkdirAll(localDir, 0o755))

	localFile := filepath.Join(localDir, "test.sh")
	require.NoError(t, os.WriteFile(localFile, []byte("#!/bin/bash"), 0o755))

	globalDir := "/global/scripts"
	cmdStr := globalDir + "/test.sh arg1"

	result := replaceAWFPathsInString(cmdStr, globalDir, localDir)

	assert.Equal(t, localFile+" arg1", result, "should replace single occurrence")
}

func TestReplaceAWFPathsInString_MultipleReplacements(t *testing.T) {
	tmpDir := t.TempDir()

	localDir := filepath.Join(tmpDir, "scripts")
	require.NoError(t, os.MkdirAll(localDir, 0o755))

	file1 := filepath.Join(localDir, "script1.sh")
	file2 := filepath.Join(localDir, "script2.sh")
	require.NoError(t, os.WriteFile(file1, []byte("#!/bin/bash"), 0o755))
	require.NoError(t, os.WriteFile(file2, []byte("#!/bin/bash"), 0o755))

	globalDir := "/global/scripts"
	cmdStr := globalDir + "/script1.sh && " + globalDir + "/script2.sh"

	result := replaceAWFPathsInString(cmdStr, globalDir, localDir)

	expected := file1 + " && " + file2
	assert.Equal(t, expected, result, "should replace all occurrences")
}

func TestReplaceAWFPathsInString_SkipMissing(t *testing.T) {
	tmpDir := t.TempDir()

	localDir := filepath.Join(tmpDir, "scripts")
	require.NoError(t, os.MkdirAll(localDir, 0o755))

	existingFile := filepath.Join(localDir, "exists.sh")
	require.NoError(t, os.WriteFile(existingFile, []byte("#!/bin/bash"), 0o755))

	globalDir := "/global/scripts"
	cmdStr := globalDir + "/missing.sh && " + globalDir + "/exists.sh"

	result := replaceAWFPathsInString(cmdStr, globalDir, localDir)

	expected := globalDir + "/missing.sh && " + existingFile
	assert.Equal(t, expected, result, "should keep unresolvable paths unchanged and replace resolvable ones")
}

func TestReplaceAWFPathsInString_NoOccurrences(t *testing.T) {
	cmdStr := "echo hello world"
	globalDir := "/global/scripts"
	localDir := "/local/scripts"

	result := replaceAWFPathsInString(cmdStr, globalDir, localDir)

	assert.Equal(t, cmdStr, result, "should return unchanged when no matches")
}

func TestReplaceAWFPathsInString_PartialMatch(t *testing.T) {
	tmpDir := t.TempDir()

	localDir := filepath.Join(tmpDir, "scripts")
	require.NoError(t, os.MkdirAll(localDir, 0o755))

	// /global/scripts is not followed by separator
	cmdStr := "/global/scripts-version/file.sh"
	result := replaceAWFPathsInString(cmdStr, "/global/scripts", localDir)

	assert.Equal(t, cmdStr, result, "should not replace partial matches")
}

func TestResolveCommandAWFPaths_LocalWorkflowResolution(t *testing.T) {
	tmpDir := t.TempDir()

	localDir := filepath.Join(tmpDir, "scripts")
	require.NoError(t, os.MkdirAll(localDir, 0o755))

	localFile := filepath.Join(localDir, "test.sh")
	require.NoError(t, os.WriteFile(localFile, []byte("#!/bin/bash"), 0o755))

	sourceDir := filepath.Join(tmpDir, "workflow.yaml")
	globalDir := "/global/scripts"

	cmd := globalDir + "/test.sh arg1"
	awfMap := map[string]string{
		"scripts_dir": globalDir,
	}

	result := resolveCommandAWFPaths(cmd, sourceDir, awfMap)

	assert.Equal(t, localFile+" arg1", result, "should resolve to local path")
}

func TestResolveCommandAWFPaths_EmptyCommand(t *testing.T) {
	sourceDir := "/some/workflow.yaml"
	awfMap := map[string]string{
		"scripts_dir": "/global/scripts",
	}

	result := resolveCommandAWFPaths("", sourceDir, awfMap)

	assert.Equal(t, "", result, "should return empty string for empty command")
}

func TestResolveCommandAWFPaths_EmptyAWFMap(t *testing.T) {
	cmd := "echo hello"
	sourceDir := "/some/workflow.yaml"

	result := resolveCommandAWFPaths(cmd, sourceDir, map[string]string{})

	assert.Equal(t, cmd, result, "should return command unchanged with empty awfMap")
}

func TestResolveCommandAWFPaths_DirectoryField(t *testing.T) {
	tmpDir := t.TempDir()

	localDir := filepath.Join(tmpDir, "scripts")
	require.NoError(t, os.MkdirAll(localDir, 0o755))

	sourceDir := filepath.Join(tmpDir, "workflow.yaml")
	globalDir := "/global/scripts"

	awfMap := map[string]string{
		"scripts_dir": globalDir,
	}

	result := resolveCommandAWFPaths(globalDir, sourceDir, awfMap)

	assert.Equal(t, localDir, result, "should resolve directory field to local path")
}

func TestResolveCommandAWFPaths_MultiplePrompts(t *testing.T) {
	tmpDir := t.TempDir()

	prompts := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(prompts, 0o755))

	file := filepath.Join(prompts, "system.md")
	require.NoError(t, os.WriteFile(file, []byte("prompt"), 0o644))

	sourceDir := filepath.Join(tmpDir, "workflow.yaml")
	globalDir := "/global/prompts"

	cmd := globalDir + "/system.md"
	awfMap := map[string]string{
		"prompts_dir": globalDir,
	}

	result := resolveCommandAWFPaths(cmd, sourceDir, awfMap)

	assert.Equal(t, file, result, "should resolve prompts_dir correctly")
}

// T006: Pack context tests for resolveCommandAWFPaths with 3-tier resolution
func TestResolveCommandAWFPaths_PackContext_Tier1UserOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup project structure with user override in tier 1
	tier1Dir := filepath.Join(tmpDir, ".awf", "scripts", "mypack")
	require.NoError(t, os.MkdirAll(tier1Dir, 0o755))

	tier1File := filepath.Join(tier1Dir, "deploy.sh")
	require.NoError(t, os.WriteFile(tier1File, []byte("#!/bin/bash\necho 'local override'"), 0o644))

	sourceDir := filepath.Join(tmpDir, "workflows", "myworkflow.yaml")
	globalDir := "/global/scripts"

	cmd := globalDir + "/deploy.sh"
	awfMap := map[string]string{
		"scripts_dir": globalDir,
		"pack_name":   "mypack",
	}

	result := resolveCommandAWFPaths(cmd, sourceDir, awfMap)

	assert.Equal(t, tier1File, result, "should resolve from tier 1 user override")
}

func TestResolveCommandAWFPaths_PackContext_Tier2PackEmbedded(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup pack structure with tier 2 embedded file
	packRoot := filepath.Join(tmpDir, ".awf", "workflow-packs", "mypack")
	tier2Dir := filepath.Join(packRoot, "scripts")
	require.NoError(t, os.MkdirAll(tier2Dir, 0o755))

	tier2File := filepath.Join(tier2Dir, "test.sh")
	require.NoError(t, os.WriteFile(tier2File, []byte("#!/bin/bash\necho 'pack embedded'"), 0o644))

	sourceDir := filepath.Join(packRoot, "workflows", "myworkflow.yaml")
	globalDir := "/global/scripts"

	cmd := globalDir + "/test.sh"
	awfMap := map[string]string{
		"scripts_dir": globalDir,
		"pack_name":   "mypack",
	}

	result := resolveCommandAWFPaths(cmd, sourceDir, awfMap)

	assert.Equal(t, tier2File, result, "should resolve from tier 2 pack embedded")
}

func TestResolveCommandAWFPaths_PackContext_Tier1PrecedesTier2(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup both tier 1 and tier 2 with same file
	tier1Dir := filepath.Join(tmpDir, ".awf", "scripts", "mypack")
	require.NoError(t, os.MkdirAll(tier1Dir, 0o755))
	tier1File := filepath.Join(tier1Dir, "deploy.sh")
	require.NoError(t, os.WriteFile(tier1File, []byte("tier1"), 0o644))

	tier2Dir := filepath.Join(tmpDir, ".awf", "workflow-packs", "mypack", "scripts")
	require.NoError(t, os.MkdirAll(tier2Dir, 0o755))
	tier2File := filepath.Join(tier2Dir, "deploy.sh")
	require.NoError(t, os.WriteFile(tier2File, []byte("tier2"), 0o644))

	sourceDir := filepath.Join(tmpDir, ".awf", "workflow-packs", "mypack", "workflows", "wf.yaml")
	globalDir := "/global/scripts"

	cmd := globalDir + "/deploy.sh"
	awfMap := map[string]string{
		"scripts_dir": globalDir,
		"pack_name":   "mypack",
	}

	result := resolveCommandAWFPaths(cmd, sourceDir, awfMap)

	assert.Equal(t, tier1File, result, "tier 1 should take precedence over tier 2")
}

func TestResolveCommandAWFPaths_PackContext_NoTierExists(t *testing.T) {
	tmpDir := t.TempDir()

	sourceDir := filepath.Join(tmpDir, ".awf", "workflow-packs", "mypack", "workflows", "wf.yaml")
	globalDir := "/global/scripts"

	cmd := globalDir + "/nonexistent.sh"
	awfMap := map[string]string{
		"scripts_dir": globalDir,
		"pack_name":   "mypack",
	}

	result := resolveCommandAWFPaths(cmd, sourceDir, awfMap)

	assert.Equal(t, cmd, result, "should return original path when no tiers exist")
}

func TestResolveCommandAWFPaths_LocalWorkflow_NoPackContext(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup local workflow structure (no pack)
	localDir := filepath.Join(tmpDir, "scripts")
	require.NoError(t, os.MkdirAll(localDir, 0o755))

	localFile := filepath.Join(localDir, "setup.sh")
	require.NoError(t, os.WriteFile(localFile, []byte("#!/bin/bash\necho 'local'"), 0o644))

	sourceDir := filepath.Join(tmpDir, "myworkflow.yaml")
	globalDir := "/global/scripts"

	cmd := globalDir + "/setup.sh"
	awfMap := map[string]string{
		"scripts_dir": globalDir,
	}

	result := resolveCommandAWFPaths(cmd, sourceDir, awfMap)

	assert.Equal(t, localFile, result, "local workflows should use 2-tier resolution")
}

func TestResolveCommandAWFPaths_PackContext_PromptsDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup pack structure with prompts_dir
	tier1Dir := filepath.Join(tmpDir, ".awf", "prompts", "speckit")
	require.NoError(t, os.MkdirAll(tier1Dir, 0o755))

	promptFile := filepath.Join(tier1Dir, "system.md")
	require.NoError(t, os.WriteFile(promptFile, []byte("# System Prompt"), 0o644))

	sourceDir := filepath.Join(tmpDir, "workflows", "specify.yaml")
	globalDir := "/global/prompts"

	cmd := globalDir + "/system.md"
	awfMap := map[string]string{
		"prompts_dir": globalDir,
		"pack_name":   "speckit",
	}

	result := resolveCommandAWFPaths(cmd, sourceDir, awfMap)

	assert.Equal(t, promptFile, result, "should resolve prompts_dir with pack context")
}

func TestResolveCommandAWFPaths_PackContext_DirectoryField(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup pack structure
	tier2Dir := filepath.Join(tmpDir, ".awf", "workflow-packs", "mypack", "scripts")
	require.NoError(t, os.MkdirAll(tier2Dir, 0o755))

	sourceDir := filepath.Join(tmpDir, ".awf", "workflow-packs", "mypack", "workflows", "wf.yaml")
	globalDir := "/global/scripts"

	awfMap := map[string]string{
		"scripts_dir": globalDir,
		"pack_name":   "mypack",
	}

	result := resolveCommandAWFPaths(globalDir, sourceDir, awfMap)

	assert.Equal(t, tier2Dir, result, "should resolve directory field with 3-tier resolution")
}

func TestResolveCommandAWFPaths_PackContext_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup pack with multiple script files
	tier2Dir := filepath.Join(tmpDir, ".awf", "workflow-packs", "mypack", "scripts")
	require.NoError(t, os.MkdirAll(tier2Dir, 0o755))

	file1 := filepath.Join(tier2Dir, "build.sh")
	file2 := filepath.Join(tier2Dir, "deploy.sh")
	require.NoError(t, os.WriteFile(file1, []byte("build"), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte("deploy"), 0o644))

	sourceDir := filepath.Join(tmpDir, ".awf", "workflow-packs", "mypack", "workflows", "wf.yaml")
	globalDir := "/global/scripts"

	awfMap := map[string]string{
		"scripts_dir": globalDir,
		"pack_name":   "mypack",
	}

	// Test first file
	cmd1 := globalDir + "/build.sh"
	result1 := resolveCommandAWFPaths(cmd1, sourceDir, awfMap)
	assert.Equal(t, file1, result1, "should resolve first file")

	// Test second file
	cmd2 := globalDir + "/deploy.sh"
	result2 := resolveCommandAWFPaths(cmd2, sourceDir, awfMap)
	assert.Equal(t, file2, result2, "should resolve second file")
}

func TestFindPackLocalDir_Tier1_UserOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup tier 1 user override
	tier1 := filepath.Join(tmpDir, ".awf", "scripts", "mypack")
	require.NoError(t, os.MkdirAll(tier1, 0o755))

	sourceDir := filepath.Join(tmpDir, "workflows", "wf.yaml")

	result := findPackLocalDir(sourceDir, "scripts", "mypack")

	assert.Equal(t, tier1, result, "should find tier 1 user override directory")
}

func TestFindPackLocalDir_Tier2_PackEmbedded(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup tier 2 pack embedded (no tier 1)
	tier2 := filepath.Join(tmpDir, ".awf", "workflow-packs", "mypack", "scripts")
	require.NoError(t, os.MkdirAll(tier2, 0o755))

	sourceDir := filepath.Join(tmpDir, ".awf", "workflow-packs", "mypack", "workflows", "wf.yaml")

	result := findPackLocalDir(sourceDir, "scripts", "mypack")

	assert.Equal(t, tier2, result, "should find tier 2 pack embedded directory")
}

func TestFindPackLocalDir_ParentDirectorySearch(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup tier 1 at project root level
	tier1 := filepath.Join(tmpDir, ".awf", "scripts", "mypack")
	require.NoError(t, os.MkdirAll(tier1, 0o755))

	// Source is nested deeper
	sourceDir := filepath.Join(tmpDir, "subdir", "workflows", "wf.yaml")

	result := findPackLocalDir(sourceDir, "scripts", "mypack")

	assert.Equal(t, tier1, result, "should find tier 1 by walking up parent directories")
}

func TestFindPackLocalDir_Fallback(t *testing.T) {
	tmpDir := t.TempDir()

	sourceDir := filepath.Join(tmpDir, "workflows", "wf.yaml")

	result := findPackLocalDir(sourceDir, "scripts", "mypack")

	// Fallback returns filepath.Join(filepath.Dir(sourceDir), localSubdir)
	// sourceDir parent is tmpDir/workflows, so fallback is tmpDir/workflows/scripts
	expected := filepath.Join(tmpDir, "workflows", "scripts")
	assert.Equal(t, expected, result, "should return fallback path when no tiers exist")
}
