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

// TestExecutionService_loadPromptFile_HappyPath tests successful prompt file loading scenarios.
func TestExecutionService_loadPromptFile_HappyPath(t *testing.T) {
	tests := []struct {
		name            string
		fileContent     string
		promptFilePath  string
		setupFile       func(t *testing.T, dir string) string
		awfContext      map[string]string
		expectedContent string
	}{
		{
			name:        "relative path to prompt file",
			fileContent: "Analyze this code: {{.inputs.code}}",
			setupFile: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "prompts", "analyze.md")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("Analyze this code: {{.inputs.code}}"), 0o644))
				return "prompts/analyze.md"
			},
			expectedContent: "Analyze this code: {{.inputs.code}}",
		},
		{
			name:        "absolute path to prompt file",
			fileContent: "Review this PR: {{.inputs.pr_url}}",
			setupFile: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "absolute.md")
				require.NoError(t, os.WriteFile(path, []byte("Review this PR: {{.inputs.pr_url}}"), 0o644))
				return path
			},
			expectedContent: "Review this PR: {{.inputs.pr_url}}",
		},
		{
			name:        "nested directory structure",
			fileContent: "Feature analysis prompt",
			setupFile: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "prompts", "feature-001", "agent", "step-02.md")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("Feature analysis prompt"), 0o644))
				return "prompts/feature-001/agent/step-02.md"
			},
			expectedContent: "Feature analysis prompt",
		},
		{
			name:        "prompt file with unicode content",
			fileContent: "分析このコード: {{.inputs.code}}\n日本語プロンプト",
			setupFile: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "unicode.md")
				require.NoError(t, os.WriteFile(path, []byte("分析このコード: {{.inputs.code}}\n日本語プロンプト"), 0o644))
				return "unicode.md"
			},
			expectedContent: "分析このコード: {{.inputs.code}}\n日本語プロンプト",
		},
		{
			name:        "template variable in path - awf.prompts_dir",
			fileContent: "Prompt from AWF prompts directory",
			setupFile: func(t *testing.T, dir string) string {
				promptsDir := filepath.Join(dir, "awf-prompts")
				path := filepath.Join(promptsDir, "plan", "research.md")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("Prompt from AWF prompts directory"), 0o644))
				return "{{.awf.prompts_dir}}/plan/research.md"
			},
			awfContext: map[string]string{
				"prompts_dir": func(t *testing.T, dir string) string {
					return filepath.Join(dir, "awf-prompts")
				}(t, ""),
			},
			expectedContent: "Prompt from AWF prompts directory",
		},
		{
			name:        "large file under 1MB limit",
			fileContent: strings.Repeat("a", 1024*500), // 500KB
			setupFile: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "large.md")
				require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("a", 1024*500)), 0o644))
				return "large.md"
			},
			expectedContent: strings.Repeat("a", 1024*500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			promptFilePath := tt.setupFile(t, tmpDir)
			if tt.awfContext != nil && strings.Contains(promptFilePath, "{{.awf.prompts_dir}}") {
				promptsDir := filepath.Join(tmpDir, "awf-prompts")
				tt.awfContext["prompts_dir"] = promptsDir
				promptFilePath = strings.Replace(promptFilePath, "{{.awf.prompts_dir}}", promptsDir, 1)
			}

			wf := &workflow.Workflow{
				Name:      "test-workflow",
				SourceDir: tmpDir,
			}

			intCtx := &interpolation.Context{
				Inputs: map[string]any{
					"code":   "test-code",
					"pr_url": "https://github.com/org/repo/pull/1",
				},
				AWF: tt.awfContext,
			}

			result, err := loadPromptFile(context.Background(), promptFilePath, wf, intCtx)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedContent, result)
		})
	}
}

// TestExecutionService_loadPromptFile_TildeExpansion tests home directory expansion.
func TestExecutionService_loadPromptFile_TildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "custom", "prompt.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(promptPath), 0o755))
	require.NoError(t, os.WriteFile(promptPath, []byte("Custom prompt content"), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	tildePromptFile := "~/custom/prompt.md"

	result, err := loadPromptFile(context.Background(), tildePromptFile, wf, intCtx)

	require.NoError(t, err)
	assert.Contains(t, result, "Custom prompt content")
}

// TestExecutionService_loadPromptFile_TemplateInterpolationInPath tests path interpolation.
func TestExecutionService_loadPromptFile_TemplateInterpolationInPath(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	promptPath := filepath.Join(configDir, "prompts", "test-feature.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(promptPath), 0o755))
	require.NoError(t, os.WriteFile(promptPath, []byte("Feature-specific prompt"), 0o644))

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

	promptFile := "{{.awf.config_dir}}/prompts/{{.inputs.feature}}.md"

	result, err := loadPromptFile(context.Background(), promptFile, wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "Feature-specific prompt", result)
}

// TestExecutionService_loadPromptFile_SizeLimit tests 1MB limit enforcement.
func TestExecutionService_loadPromptFile_SizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	oversizedContent := strings.Repeat("a", 1024*1024+1) // 1MB + 1 byte
	promptPath := filepath.Join(tmpDir, "oversized.md")
	require.NoError(t, os.WriteFile(promptPath, []byte(oversizedContent), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	_, err := loadPromptFile(context.Background(), "oversized.md", wf, intCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds 1MB limit")
}

// TestExecutionService_loadPromptFile_FileNotFound tests missing file error.
func TestExecutionService_loadPromptFile_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	_, err := loadPromptFile(context.Background(), "nonexistent.md", wf, intCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent.md")
}

// TestExecutionService_loadPromptFile_DirectoryInsteadOfFile tests directory error.
func TestExecutionService_loadPromptFile_DirectoryInsteadOfFile(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(dirPath, 0o755))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	_, err := loadPromptFile(context.Background(), "prompts", wf, intCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")
}

// TestExecutionService_loadPromptFile_PermissionDenied tests unreadable file error.
func TestExecutionService_loadPromptFile_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "unreadable.md")
	require.NoError(t, os.WriteFile(promptPath, []byte("secret content"), 0o000))
	defer os.Chmod(promptPath, 0o644) // cleanup

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	_, err := loadPromptFile(context.Background(), "unreadable.md", wf, intCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

// TestExecutionService_loadPromptFile_EmptyFile tests empty file handling.
func TestExecutionService_loadPromptFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "empty.md")
	require.NoError(t, os.WriteFile(promptPath, []byte(""), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	result, err := loadPromptFile(context.Background(), "empty.md", wf, intCtx)

	require.NoError(t, err)
	assert.Empty(t, result)
}

// TestExecutionService_loadPromptFile_WhitespaceOnlyFile tests whitespace-only file.
func TestExecutionService_loadPromptFile_WhitespaceOnlyFile(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "whitespace.md")
	require.NoError(t, os.WriteFile(promptPath, []byte("   \n\t\n   "), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	result, err := loadPromptFile(context.Background(), "whitespace.md", wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "   \n\t\n   ", result)
}

// TestExecutionService_loadPromptFile_PathWithSpaces tests file path containing spaces.
func TestExecutionService_loadPromptFile_PathWithSpaces(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "my prompt file.md")
	require.NoError(t, os.WriteFile(promptPath, []byte("Content with spaces in path"), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	result, err := loadPromptFile(context.Background(), "my prompt file.md", wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "Content with spaces in path", result)
}

// TestExecutionService_loadPromptFile_RelativePathResolution tests relative path handling.
func TestExecutionService_loadPromptFile_RelativePathResolution(t *testing.T) {
	tmpDir := t.TempDir()

	workflowDir := filepath.Join(tmpDir, "workflows")
	promptsDir := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(workflowDir, 0o755))
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	promptPath := filepath.Join(promptsDir, "analyze.md")
	require.NoError(t, os.WriteFile(promptPath, []byte("Shared prompt"), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: workflowDir,
	}

	intCtx := &interpolation.Context{}

	relativePromptFile := "../prompts/analyze.md"

	result, err := loadPromptFile(context.Background(), relativePromptFile, wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "Shared prompt", result)
}

// TestExecutionService_loadPromptFile_MultipleAWFVariables tests path with multiple template vars.
func TestExecutionService_loadPromptFile_MultipleAWFVariables(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	dataDir := filepath.Join(tmpDir, "data")
	promptPath := filepath.Join(configDir, dataDir, "prompt.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(promptPath), 0o755))
	require.NoError(t, os.WriteFile(promptPath, []byte("Multi-var prompt"), 0o644))

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

	promptFile := "{{.awf.config_dir}}/{{.awf.data_dir}}/prompt.md"

	result, err := loadPromptFile(context.Background(), promptFile, wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "Multi-var prompt", result)
}

// TestExecutionService_loadPromptFile_BoundarySize tests file at exact 1MB limit.
func TestExecutionService_loadPromptFile_BoundarySize(t *testing.T) {
	tmpDir := t.TempDir()

	exactContent := strings.Repeat("a", 1024*1024) // Exactly 1MB
	promptPath := filepath.Join(tmpDir, "exact-1mb.md")
	require.NoError(t, os.WriteFile(promptPath, []byte(exactContent), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	result, err := loadPromptFile(context.Background(), "exact-1mb.md", wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, exactContent, result)
}

// TestExecutionService_loadPromptFile_ContextCancellation tests context cancellation handling.
func TestExecutionService_loadPromptFile_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "test.md")
	require.NoError(t, os.WriteFile(promptPath, []byte("test content"), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := loadPromptFile(ctx, "test.md", wf, intCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}
