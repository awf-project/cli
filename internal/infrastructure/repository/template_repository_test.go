package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domerrors "github.com/vanoix/awf/internal/domain/errors"
	"github.com/vanoix/awf/internal/domain/workflow"
)

const templateFixturesPath = "../../../tests/fixtures/templates"

// =============================================================================
// GetTemplate Tests
// =============================================================================

func TestYAMLTemplateRepository_GetTemplate_Success(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	tmpl, err := repo.GetTemplate(context.Background(), "simple-echo")
	require.NoError(t, err)
	require.NotNil(t, tmpl)

	assert.Equal(t, "simple-echo", tmpl.Name)
	assert.Len(t, tmpl.Parameters, 2)

	// Check required param
	assert.Equal(t, "message", tmpl.Parameters[0].Name)
	assert.True(t, tmpl.Parameters[0].Required)

	// Check optional param with default
	assert.Equal(t, "prefix", tmpl.Parameters[1].Name)
	assert.False(t, tmpl.Parameters[1].Required)
	assert.Equal(t, "[INFO]", tmpl.Parameters[1].Default)

	// Check states exist
	require.NotEmpty(t, tmpl.States)
}

func TestYAMLTemplateRepository_GetTemplate_AIAnalyze(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	tmpl, err := repo.GetTemplate(context.Background(), "ai-analyze")
	require.NoError(t, err)
	require.NotNil(t, tmpl)

	assert.Equal(t, "ai-analyze", tmpl.Name)
	assert.Len(t, tmpl.Parameters, 3)

	// Verify parameters match F017 spec
	paramNames := make(map[string]workflow.TemplateParam)
	for _, p := range tmpl.Parameters {
		paramNames[p.Name] = p
	}

	require.Contains(t, paramNames, "prompt")
	assert.True(t, paramNames["prompt"].Required)

	require.Contains(t, paramNames, "model")
	assert.False(t, paramNames["model"].Required)
	assert.Equal(t, "claude", paramNames["model"].Default)

	require.Contains(t, paramNames, "timeout")
	assert.False(t, paramNames["timeout"].Required)
	assert.Equal(t, "120s", paramNames["timeout"].Default)
}

func TestYAMLTemplateRepository_GetTemplate_NoParams(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	tmpl, err := repo.GetTemplate(context.Background(), "no-params")
	require.NoError(t, err)
	require.NotNil(t, tmpl)

	assert.Equal(t, "no-params", tmpl.Name)
	assert.Empty(t, tmpl.Parameters)
	require.NotEmpty(t, tmpl.States)
}

func TestYAMLTemplateRepository_GetTemplate_NotFound(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	tmpl, err := repo.GetTemplate(context.Background(), "nonexistent-template")

	require.Error(t, err)
	assert.Nil(t, tmpl)

	// Verify error type
	var notFound *TemplateNotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, "nonexistent-template", notFound.TemplateName)
}

func TestYAMLTemplateRepository_GetTemplate_EmptySearchPaths(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{})

	tmpl, err := repo.GetTemplate(context.Background(), "any-template")

	require.Error(t, err)
	assert.Nil(t, tmpl)

	var notFound *TemplateNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestYAMLTemplateRepository_GetTemplate_MultipleSearchPaths(t *testing.T) {
	// Create temp directory with a test template
	tmpDir := t.TempDir()
	templateContent := `
name: temp-template
parameters: []
states:
  initial: run
  run:
    type: command
    command: "echo test"
`
	err := os.WriteFile(filepath.Join(tmpDir, "temp-template.yaml"), []byte(templateContent), 0o644)
	require.NoError(t, err)

	// Use temp dir first, then fixtures
	repo := NewYAMLTemplateRepository([]string{tmpDir, templateFixturesPath})

	// Should find temp-template from first path
	tmpl, err := repo.GetTemplate(context.Background(), "temp-template")
	require.NoError(t, err)
	require.NotNil(t, tmpl)
	assert.Equal(t, "temp-template", tmpl.Name)

	// Should still find simple-echo from second path
	tmpl2, err := repo.GetTemplate(context.Background(), "simple-echo")
	require.NoError(t, err)
	require.NotNil(t, tmpl2)
	assert.Equal(t, "simple-echo", tmpl2.Name)
}

func TestYAMLTemplateRepository_GetTemplate_NonExistentSearchPath(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{"/nonexistent/path", templateFixturesPath})

	// Should still work by falling through to valid path
	tmpl, err := repo.GetTemplate(context.Background(), "simple-echo")
	require.NoError(t, err)
	require.NotNil(t, tmpl)
}

// =============================================================================
// GetTemplate Caching Tests
// =============================================================================

func TestYAMLTemplateRepository_GetTemplate_Cache(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	// First call - load from disk
	tmpl1, err := repo.GetTemplate(context.Background(), "simple-echo")
	require.NoError(t, err)
	require.NotNil(t, tmpl1)

	// Second call - should return cached value
	tmpl2, err := repo.GetTemplate(context.Background(), "simple-echo")
	require.NoError(t, err)
	require.NotNil(t, tmpl2)

	// Should be the same pointer (cached)
	assert.Same(t, tmpl1, tmpl2)
}

func TestYAMLTemplateRepository_GetTemplate_CacheMultipleTemplates(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	// Load multiple templates
	tmpl1, err := repo.GetTemplate(context.Background(), "simple-echo")
	require.NoError(t, err)

	tmpl2, err := repo.GetTemplate(context.Background(), "ai-analyze")
	require.NoError(t, err)

	// Both should be cached
	tmpl1Cached, err := repo.GetTemplate(context.Background(), "simple-echo")
	require.NoError(t, err)
	assert.Same(t, tmpl1, tmpl1Cached)

	tmpl2Cached, err := repo.GetTemplate(context.Background(), "ai-analyze")
	require.NoError(t, err)
	assert.Same(t, tmpl2, tmpl2Cached)
}

// =============================================================================
// ListTemplates Tests
// =============================================================================

func TestYAMLTemplateRepository_ListTemplates(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	names, err := repo.ListTemplates(context.Background())
	require.NoError(t, err)

	// Should find our test fixtures
	assert.Contains(t, names, "simple-echo")
	assert.Contains(t, names, "ai-analyze")
	assert.Contains(t, names, "no-params")
	assert.Contains(t, names, "multi-state")
	assert.Contains(t, names, "all-required")

	// Should include invalid ones too (they're valid YAML files)
	assert.Contains(t, names, "invalid-syntax")
	assert.Contains(t, names, "invalid-missing-name")
}

func TestYAMLTemplateRepository_ListTemplates_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewYAMLTemplateRepository([]string{tmpDir})

	names, err := repo.ListTemplates(context.Background())
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestYAMLTemplateRepository_ListTemplates_NonExistentDir(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{"/nonexistent/path"})

	names, err := repo.ListTemplates(context.Background())
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestYAMLTemplateRepository_ListTemplates_MultipleSearchPaths(t *testing.T) {
	// Create temp directory with additional templates
	tmpDir := t.TempDir()
	templateContent := `
name: extra-template
parameters: []
states:
  initial: run
  run:
    type: command
    command: "echo extra"
`
	err := os.WriteFile(filepath.Join(tmpDir, "extra-template.yaml"), []byte(templateContent), 0o644)
	require.NoError(t, err)

	repo := NewYAMLTemplateRepository([]string{tmpDir, templateFixturesPath})

	names, err := repo.ListTemplates(context.Background())
	require.NoError(t, err)

	// Should include templates from both paths
	assert.Contains(t, names, "extra-template")
	assert.Contains(t, names, "simple-echo")
}

func TestYAMLTemplateRepository_ListTemplates_NoDuplicates(t *testing.T) {
	// Create temp dir with same-named template as fixtures
	tmpDir := t.TempDir()
	templateContent := `
name: simple-echo
parameters: []
states:
  initial: run
  run:
    type: command
    command: "echo override"
`
	err := os.WriteFile(filepath.Join(tmpDir, "simple-echo.yaml"), []byte(templateContent), 0o644)
	require.NoError(t, err)

	repo := NewYAMLTemplateRepository([]string{tmpDir, templateFixturesPath})

	names, err := repo.ListTemplates(context.Background())
	require.NoError(t, err)

	// Count occurrences of simple-echo
	count := 0
	for _, n := range names {
		if n == "simple-echo" {
			count++
		}
	}
	assert.Equal(t, 1, count, "should not have duplicate template names")
}

func TestYAMLTemplateRepository_ListTemplates_IgnoresDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory
	err := os.Mkdir(filepath.Join(tmpDir, "subdir.yaml"), 0o755)
	require.NoError(t, err)

	// Create a valid template
	templateContent := `
name: valid
parameters: []
states:
  initial: run
  run:
    type: command
    command: "echo test"
`
	err = os.WriteFile(filepath.Join(tmpDir, "valid.yaml"), []byte(templateContent), 0o644)
	require.NoError(t, err)

	repo := NewYAMLTemplateRepository([]string{tmpDir})

	names, err := repo.ListTemplates(context.Background())
	require.NoError(t, err)

	assert.Contains(t, names, "valid")
	assert.NotContains(t, names, "subdir")
}

func TestYAMLTemplateRepository_ListTemplates_OnlyYAMLFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create various files
	err := os.WriteFile(filepath.Join(tmpDir, "template.yaml"), []byte("name: yaml\nparameters: []\nstates:\n  initial: run\n  run:\n    type: command\n    command: echo"), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# README"), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "script.sh"), []byte("#!/bin/bash"), 0o644)
	require.NoError(t, err)

	repo := NewYAMLTemplateRepository([]string{tmpDir})

	names, err := repo.ListTemplates(context.Background())
	require.NoError(t, err)

	assert.Contains(t, names, "template")
	assert.NotContains(t, names, "readme")
	assert.NotContains(t, names, "script")
}

// =============================================================================
// Exists Tests
// =============================================================================

func TestYAMLTemplateRepository_Exists(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	tests := []struct {
		name   string
		exists bool
	}{
		{"simple-echo", true},
		{"ai-analyze", true},
		{"no-params", true},
		{"nonexistent", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repo.Exists(context.Background(), tt.name)
			assert.Equal(t, tt.exists, result)
		})
	}
}

func TestYAMLTemplateRepository_Exists_EmptySearchPaths(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{})

	assert.False(t, repo.Exists(context.Background(), "any-template"))
}

func TestYAMLTemplateRepository_Exists_MultipleSearchPaths(t *testing.T) {
	tmpDir := t.TempDir()
	templateContent := `
name: temp-only
parameters: []
states:
  initial: run
  run:
    type: command
    command: "echo test"
`
	err := os.WriteFile(filepath.Join(tmpDir, "temp-only.yaml"), []byte(templateContent), 0o644)
	require.NoError(t, err)

	repo := NewYAMLTemplateRepository([]string{tmpDir, templateFixturesPath})

	// Should find in first path
	assert.True(t, repo.Exists(context.Background(), "temp-only"))

	// Should find in second path
	assert.True(t, repo.Exists(context.Background(), "simple-echo"))

	// Should not find nonexistent
	assert.False(t, repo.Exists(context.Background(), "nonexistent"))
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestYAMLTemplateRepository_GetTemplate_InvalidSyntax(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	tmpl, err := repo.GetTemplate(context.Background(), "invalid-syntax")

	require.Error(t, err)
	assert.Nil(t, tmpl)

	// Should be a StructuredError with WORKFLOW.PARSE.YAML_SYNTAX code
	var structErr *domerrors.StructuredError
	require.ErrorAs(t, err, &structErr)
	assert.Equal(t, domerrors.ErrorCodeWorkflowParseYAMLSyntax, structErr.Code)
}

func TestYAMLTemplateRepository_GetTemplate_MissingName(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	tmpl, err := repo.GetTemplate(context.Background(), "invalid-missing-name")

	// The template should fail validation (name is required)
	require.Error(t, err)
	assert.Nil(t, tmpl)
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewYAMLTemplateRepository(t *testing.T) {
	tests := []struct {
		name        string
		searchPaths []string
	}{
		{
			name:        "single path",
			searchPaths: []string{templateFixturesPath},
		},
		{
			name:        "multiple paths",
			searchPaths: []string{"/path/one", "/path/two", "/path/three"},
		},
		{
			name:        "empty paths",
			searchPaths: []string{},
		},
		{
			name:        "nil paths",
			searchPaths: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewYAMLTemplateRepository(tt.searchPaths)
			require.NotNil(t, repo)
			assert.Equal(t, tt.searchPaths, repo.searchPaths)
			assert.NotNil(t, repo.cache)
		})
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestYAMLTemplateRepository_ConcurrentGetTemplate(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	const goroutines = 10
	done := make(chan struct{})
	errors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()

			tmpl, err := repo.GetTemplate(context.Background(), "simple-echo")
			if err != nil {
				errors <- err
				return
			}
			if tmpl == nil {
				errors <- assert.AnError
				return
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}
	close(errors)

	for err := range errors {
		t.Errorf("concurrent access error: %v", err)
	}
}

// =============================================================================
// Interface Compliance Tests
// =============================================================================

func TestYAMLTemplateRepository_ImplementsInterface(t *testing.T) {
	repo := NewYAMLTemplateRepository([]string{templateFixturesPath})

	// This compilation check ensures YAMLTemplateRepository implements TemplateRepository
	var _ interface {
		GetTemplate(context.Context, string) (*workflow.Template, error)
		ListTemplates(context.Context) ([]string, error)
		Exists(context.Context, string) bool
	} = repo
}
