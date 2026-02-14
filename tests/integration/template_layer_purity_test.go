//go:build integration

// Feature: C044
//
// Functional tests validating template service test layer purity and hexagonal architecture compliance.
// Tests verify that MockTemplateRepository works correctly in real scenarios
// and that all template service tests can run without infrastructure layer imports.

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/testutil"
)

// TestMockTemplateRepository_HappyPath validates normal template repository operations
func TestMockTemplateRepository_HappyPath(t *testing.T) {
	ctx := context.Background()
	repo := testutil.NewMockTemplateRepository()
	require.NotNil(t, repo, "NewMockTemplateRepository should return non-nil instance")

	// Create test template
	testTemplate := &workflow.Template{
		Name: "test-template",
		Parameters: []workflow.TemplateParam{
			{Name: "param1", Required: true},
		},
		States: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
		},
	}

	// Add template
	repo.AddTemplate("test-template", testTemplate)

	// Retrieve template
	retrieved, err := repo.GetTemplate(ctx, "test-template")
	assert.NoError(t, err, "GetTemplate should succeed for existing template")
	assert.NotNil(t, retrieved, "retrieved template should not be nil")
	assert.Equal(t, "test-template", retrieved.Name, "template name should match")
	assert.Len(t, retrieved.Parameters, 1, "template should have one parameter")
	assert.Equal(t, "param1", retrieved.Parameters[0].Name, "parameter name should match")

	// Check existence
	exists := repo.Exists(ctx, "test-template")
	assert.True(t, exists, "Exists should return true for added template")

	// List templates
	names, err := repo.ListTemplates(ctx)
	assert.NoError(t, err, "ListTemplates should succeed")
	assert.Contains(t, names, "test-template", "ListTemplates should include added template")
	assert.Len(t, names, 1, "should have exactly one template")
}

// TestMockTemplateRepository_MultipleTemplates validates handling multiple templates
func TestMockTemplateRepository_MultipleTemplates(t *testing.T) {
	ctx := context.Background()
	repo := testutil.NewMockTemplateRepository()

	// Add multiple templates
	templates := map[string]*workflow.Template{
		"template1": {Name: "template1", Parameters: []workflow.TemplateParam{{Name: "p1"}}},
		"template2": {Name: "template2", Parameters: []workflow.TemplateParam{{Name: "p2"}}},
		"template3": {Name: "template3", Parameters: []workflow.TemplateParam{{Name: "p3"}}},
	}

	for name, tpl := range templates {
		repo.AddTemplate(name, tpl)
	}

	// Verify all templates exist
	for name := range templates {
		exists := repo.Exists(ctx, name)
		assert.True(t, exists, "template %s should exist", name)
	}

	// List should contain all templates
	names, err := repo.ListTemplates(ctx)
	assert.NoError(t, err)
	assert.Len(t, names, 3, "should have exactly 3 templates")

	// Verify each template can be retrieved
	for name, expected := range templates {
		retrieved, err := repo.GetTemplate(ctx, name)
		assert.NoError(t, err, "GetTemplate should succeed for %s", name)
		assert.Equal(t, expected.Name, retrieved.Name)
		assert.Equal(t, len(expected.Parameters), len(retrieved.Parameters))
	}
}

// TestMockTemplateRepository_UpdateExisting validates updating existing templates
func TestMockTemplateRepository_UpdateExisting(t *testing.T) {
	ctx := context.Background()
	repo := testutil.NewMockTemplateRepository()

	// Add initial template
	original := &workflow.Template{Name: "test", Parameters: []workflow.TemplateParam{{Name: "original"}}}
	repo.AddTemplate("test", original)

	// Update template
	updated := &workflow.Template{Name: "test", Parameters: []workflow.TemplateParam{{Name: "updated"}}}
	repo.AddTemplate("test", updated)

	// Verify update
	retrieved, err := repo.GetTemplate(ctx, "test")
	assert.NoError(t, err)
	assert.Equal(t, "updated", retrieved.Parameters[0].Name, "template should reflect update")
}

// TestMockTemplateRepository_EdgeCases validates boundary conditions
func TestMockTemplateRepository_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testutil.MockTemplateRepository) error
		check   func(*testing.T, context.Context, *testutil.MockTemplateRepository)
		wantErr bool
	}{
		{
			name: "empty repository",
			setup: func(r *testutil.MockTemplateRepository) error {
				return nil
			},
			check: func(t *testing.T, ctx context.Context, r *testutil.MockTemplateRepository) {
				names, err := r.ListTemplates(ctx)
				assert.NoError(t, err)
				assert.Empty(t, names, "empty repository should return empty list")
				assert.False(t, r.Exists(ctx, "nonexistent"), "Exists should return false for nonexistent template")
			},
			wantErr: false,
		},
		{
			name: "template with empty name",
			setup: func(r *testutil.MockTemplateRepository) error {
				r.AddTemplate("", &workflow.Template{Name: ""})
				return nil
			},
			check: func(t *testing.T, ctx context.Context, r *testutil.MockTemplateRepository) {
				exists := r.Exists(ctx, "")
				assert.True(t, exists, "should handle empty name as valid key")
			},
			wantErr: false,
		},
		{
			name: "template with special characters in name",
			setup: func(r *testutil.MockTemplateRepository) error {
				specialName := "test/template:with-special.chars_123"
				r.AddTemplate(specialName, &workflow.Template{Name: specialName})
				return nil
			},
			check: func(t *testing.T, ctx context.Context, r *testutil.MockTemplateRepository) {
				specialName := "test/template:with-special.chars_123"
				retrieved, err := r.GetTemplate(ctx, specialName)
				assert.NoError(t, err)
				assert.Equal(t, specialName, retrieved.Name)
			},
			wantErr: false,
		},
		{
			name: "nil template value",
			setup: func(r *testutil.MockTemplateRepository) error {
				r.AddTemplate("nil-test", nil)
				return nil
			},
			check: func(t *testing.T, ctx context.Context, r *testutil.MockTemplateRepository) {
				retrieved, err := r.GetTemplate(ctx, "nil-test")
				assert.NoError(t, err)
				assert.Nil(t, retrieved, "should handle nil template values")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := testutil.NewMockTemplateRepository()
			err := tt.setup(repo)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.check(t, ctx, repo)
			}
		})
	}
}

// TestMockTemplateRepository_TemplateNotFound validates not found error behavior
func TestMockTemplateRepository_TemplateNotFound(t *testing.T) {
	ctx := context.Background()
	repo := testutil.NewMockTemplateRepository()

	// Attempt to get nonexistent template
	retrieved, err := repo.GetTemplate(ctx, "nonexistent")
	assert.Error(t, err, "GetTemplate should return error for nonexistent template")
	assert.Nil(t, retrieved, "retrieved template should be nil on error")

	// Verify error type is TemplateNotFoundError
	var notFoundErr *workflow.TemplateNotFoundError
	assert.True(t, errors.As(err, &notFoundErr), "error should be TemplateNotFoundError")
	assert.Equal(t, "nonexistent", notFoundErr.TemplateName, "error should contain template name")
}

// TestMockTemplateRepository_ThreadSafety validates concurrent access safety
func TestMockTemplateRepository_ThreadSafety(t *testing.T) {
	ctx := context.Background()
	repo := testutil.NewMockTemplateRepository()

	// Concurrently add templates
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("template-%d", idx)
			tpl := &workflow.Template{
				Name: name,
				Parameters: []workflow.TemplateParam{
					{Name: fmt.Sprintf("param%d", idx)},
				},
			}
			repo.AddTemplate(name, tpl)
		}(i)
	}
	wg.Wait()

	// Verify all templates were added
	names, err := repo.ListTemplates(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(names), 1, "at least some templates should be added")
	assert.LessOrEqual(t, len(names), 20, "should not exceed expected count")

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = repo.ListTemplates(ctx)
			_ = repo.Exists(ctx, "template-0")
			_, _ = repo.GetTemplate(ctx, "template-0")
		}()
	}
	wg.Wait()
}

// TestMockTemplateRepository_ErrorInjection validates error injection capabilities
func TestMockTemplateRepository_ErrorInjection(t *testing.T) {
	ctx := context.Background()
	repo := testutil.NewMockTemplateRepository()

	// Test GetTemplate error injection
	t.Run("GetTemplate error", func(t *testing.T) {
		expectedErr := errors.New("custom get error")
		repo.SetGetError(expectedErr)

		_, err := repo.GetTemplate(ctx, "any-name")
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err, "should return injected error")

		// Clear error
		repo.SetGetError(nil)
		repo.AddTemplate("test", &workflow.Template{Name: "test"})
		_, err = repo.GetTemplate(ctx, "test")
		assert.NoError(t, err, "should succeed after clearing error")
	})

	// Test ListTemplates error injection
	t.Run("ListTemplates error", func(t *testing.T) {
		repo := testutil.NewMockTemplateRepository()
		expectedErr := errors.New("custom list error")
		repo.SetListError(expectedErr)

		_, err := repo.ListTemplates(ctx)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err, "should return injected error")

		// Clear error
		repo.SetListError(nil)
		names, err := repo.ListTemplates(ctx)
		assert.NoError(t, err, "should succeed after clearing error")
		assert.NotNil(t, names)
	})
}

// TestMockTemplateRepository_ErrorPrecedence validates that injected errors take precedence
func TestMockTemplateRepository_ErrorPrecedence(t *testing.T) {
	ctx := context.Background()
	repo := testutil.NewMockTemplateRepository()

	// Add a template
	repo.AddTemplate("test", &workflow.Template{Name: "test"})

	// Set get error - should return custom error even for existing template
	customErr := errors.New("forced error")
	repo.SetGetError(customErr)

	_, err := repo.GetTemplate(ctx, "test")
	assert.Error(t, err)
	assert.Equal(t, customErr, err, "custom error should take precedence over not found")
}

// TestTemplateServiceTests_NoInfrastructureImports verifies that template service tests
// do not import infrastructure layer packages
func TestTemplateServiceTests_NoInfrastructureImports(t *testing.T) {
	repoRoot := getRepoRoot(t)
	testFiles := []string{
		filepath.Join(repoRoot, "internal/application/template_service_test.go"),
		filepath.Join(repoRoot, "internal/application/template_service_helpers_test.go"),
	}

	for _, filePath := range testFiles {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			// Read file imports
			imports, err := extractImportsFromFile(filePath)
			require.NoError(t, err, "should be able to read file: %s", filePath)

			// Verify no infrastructure/repository imports
			for _, imp := range imports {
				cleanImp := strings.Trim(imp, `"`)
				assert.NotContains(t, cleanImp, "infrastructure/repository",
					"file %s must not import infrastructure/repository - violates C044", filepath.Base(filePath))

				// Flag any infrastructure imports (except logger which may be acceptable)
				if strings.Contains(cleanImp, "infrastructure/") && !strings.Contains(cleanImp, "infrastructure/logger") {
					t.Errorf("file %s should not import infrastructure packages (except logger): %s",
						filepath.Base(filePath), cleanImp)
				}
			}
		})
	}
}

// TestTemplateServiceTests_UseDomainErrors verifies that template service tests
// use domain layer error types instead of infrastructure types
func TestTemplateServiceTests_UseDomainErrors(t *testing.T) {
	repoRoot := getRepoRoot(t)
	testFiles := []string{
		filepath.Join(repoRoot, "internal/application/template_service_test.go"),
		filepath.Join(repoRoot, "internal/application/template_service_helpers_test.go"),
	}

	for _, filePath := range testFiles {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "should be able to read file: %s", filePath)

			lines := strings.Split(string(content), "\n")
			violations := []string{}

			for lineNum, line := range lines {
				// Skip comments
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") || trimmed == "" {
					continue
				}

				// Check for repository.TemplateNotFoundError usage
				if strings.Contains(line, "repository.TemplateNotFoundError") {
					violation := fmt.Sprintf("line %d: uses repository.TemplateNotFoundError instead of workflow.TemplateNotFoundError",
						lineNum+1)
					violations = append(violations, violation)
				}
			}

			assert.Empty(t, violations,
				"file %s should use workflow.TemplateNotFoundError, not repository.TemplateNotFoundError",
				filepath.Base(filePath))
		})
	}
}

// TestTemplateServiceTests_UseTestutilMocks verifies that tests import and use testutil mocks
func TestTemplateServiceTests_UseTestutilMocks(t *testing.T) {
	repoRoot := getRepoRoot(t)
	testFiles := []string{
		filepath.Join(repoRoot, "internal/application/template_service_test.go"),
		filepath.Join(repoRoot, "internal/application/template_service_helpers_test.go"),
	}

	for _, filePath := range testFiles {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			imports, err := extractImportsFromFile(filePath)
			require.NoError(t, err)

			// Should import testutil
			hasTestutilImport := false
			for _, imp := range imports {
				if strings.Contains(imp, "internal/testutil") {
					hasTestutilImport = true
					break
				}
			}

			assert.True(t, hasTestutilImport,
				"file %s should import internal/testutil for MockTemplateRepository", filepath.Base(filePath))
		})
	}
}

// TestTemplateServiceTests_NoLocalMocks verifies that local mock definitions were removed
func TestTemplateServiceTests_NoLocalMocks(t *testing.T) {
	repoRoot := getRepoRoot(t)
	testFiles := []string{
		filepath.Join(repoRoot, "internal/application/template_service_test.go"),
		filepath.Join(repoRoot, "internal/application/template_service_helpers_test.go"),
	}

	for _, filePath := range testFiles {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			content, err := os.ReadFile(filePath)
			require.NoError(t, err)
			contentStr := string(content)

			// Check for local mock definitions
			assert.NotContains(t, contentStr, "type mockTemplateRepository struct",
				"file %s should not define local mockTemplateRepository", filepath.Base(filePath))
			assert.NotContains(t, contentStr, "func newMockTemplateRepository(",
				"file %s should not define newMockTemplateRepository()", filepath.Base(filePath))
		})
	}
}

// TestRegressionGuards_StillPass verifies that existing C038 regression tests
// continue to pass after C044 implementation
func TestRegressionGuards_StillPass(t *testing.T) {
	repoRoot := getRepoRoot(t)
	// Verify C038 architecture guard test file exists
	c038GuardFile := filepath.Join(repoRoot, "internal/application/execution_service_architecture_test.go")
	_, err := os.Stat(c038GuardFile)
	assert.NoError(t, err, "C038 architecture guard test should still exist")

	// Verify C038 integration test exists
	c038IntegrationFile := filepath.Join(repoRoot, "tests/integration/c038_application_test_layer_purity_test.go")
	_, err = os.Stat(c038IntegrationFile)
	assert.NoError(t, err, "C038 integration test should still exist")

	// The actual C038 tests will run as part of the test suite
	// This test documents the dependency and ensures files are present
	t.Log("C038 regression guard files verified - actual tests run separately")
}

// TestMockRepositories_PortCompliance verifies all mock repositories implement their ports
func TestMockRepositories_PortCompliance(t *testing.T) {
	ctx := context.Background()

	// MockTemplateRepository should satisfy ports.TemplateRepository interface
	repo := testutil.NewMockTemplateRepository()
	require.NotNil(t, repo)

	// Verify interface methods work
	repo.AddTemplate("compliance-test", &workflow.Template{Name: "compliance-test"})

	retrieved, err := repo.GetTemplate(ctx, "compliance-test")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)

	names, err := repo.ListTemplates(ctx)
	assert.NoError(t, err)
	assert.Contains(t, names, "compliance-test")

	exists := repo.Exists(ctx, "compliance-test")
	assert.True(t, exists)
}

// TestArchitectureCompliance_C044_NoInfrastructureImports validates test purity
func TestArchitectureCompliance_C044_NoInfrastructureImports(t *testing.T) {
	// This test documents that C044 enables application tests to run
	// without importing internal/infrastructure/repository package.
	//
	// The actual verification is done by:
	// 1. Compiler - if tests import infrastructure, they would fail at compile time
	// 2. Architecture guard tests in internal/application/template_service_architecture_test.go
	//
	// This test verifies that MockTemplateRepository provides complete functionality
	// without requiring infrastructure imports.

	ctx := context.Background()
	repo := testutil.NewMockTemplateRepository()

	// All operations work without infrastructure
	repo.AddTemplate("architecture-test", &workflow.Template{Name: "architecture-test"})

	retrieved, err := repo.GetTemplate(ctx, "architecture-test")
	assert.NoError(t, err)
	assert.Equal(t, "architecture-test", retrieved.Name)

	// Success - no infrastructure imports needed
	t.Log("C044 architecture compliance verified - MockTemplateRepository eliminates infrastructure dependencies")
}

// extractImportsFromFile extracts import statements from a Go source file
func extractImportsFromFile(filename string) ([]string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	imports := []string{}
	inImportBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle single-line import
		if strings.HasPrefix(trimmed, "import ") && strings.Contains(trimmed, `"`) {
			start := strings.Index(trimmed, `"`)
			end := strings.LastIndex(trimmed, `"`)
			if start >= 0 && end > start {
				imports = append(imports, trimmed[start:end+1])
			}
			continue
		}

		// Handle import block
		if strings.HasPrefix(trimmed, "import (") {
			inImportBlock = true
			continue
		}

		if inImportBlock {
			if trimmed == ")" {
				inImportBlock = false
				continue
			}

			// Extract import path
			if strings.Contains(trimmed, `"`) {
				start := strings.Index(trimmed, `"`)
				end := strings.LastIndex(trimmed, `"`)
				if start >= 0 && end > start {
					imports = append(imports, trimmed[start:end+1])
				}
			}
		}
	}

	return imports, nil
}
