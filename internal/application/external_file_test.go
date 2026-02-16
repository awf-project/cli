package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/pkg/interpolation"
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
