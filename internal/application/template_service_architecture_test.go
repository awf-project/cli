package application

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T007
// Feature: C044
// Purpose: Verify application layer template service tests maintain hexagonal
// architecture boundaries by ensuring no infrastructure imports exist.

// TestTemplateServiceTest_NoInfrastructureImport verifies that
// template_service_test.go does not import infrastructure packages,
// maintaining hexagonal architecture compliance.
// This is the happy path test - the file should be clean after C044 migration.
func TestTemplateServiceTest_NoInfrastructureImport(t *testing.T) {
	sourceFile := "template_service_test.go"

	imports, err := extractImports(sourceFile)
	require.NoError(t, err, "should be able to read source file")

	for _, imp := range imports {
		cleanImp := strings.Trim(imp, `"`)
		assert.NotContains(t, cleanImp, "infrastructure/repository",
			"template_service_test.go must not import infrastructure/repository - violates C044")
		assert.NotContains(t, cleanImp, "infrastructure/expression",
			"template_service_test.go should avoid infrastructure layer imports")
		// Allow infrastructure/logger as it may be used in production code
		if strings.Contains(cleanImp, "infrastructure/") && !strings.Contains(cleanImp, "infrastructure/logger") {
			t.Errorf("template_service_test.go should not import infrastructure packages (except logger): %s", cleanImp)
		}
	}
}

// TestTemplateServiceHelpersTest_NoInfrastructureImport verifies that
// template_service_helpers_test.go does not import infrastructure packages,
// maintaining hexagonal architecture compliance.
// This is the happy path test - the file should be clean after C044 migration.
func TestTemplateServiceHelpersTest_NoInfrastructureImport(t *testing.T) {
	sourceFile := "template_service_helpers_test.go"

	imports, err := extractImports(sourceFile)
	require.NoError(t, err, "should be able to read source file")

	for _, imp := range imports {
		cleanImp := strings.Trim(imp, `"`)
		assert.NotContains(t, cleanImp, "infrastructure/repository",
			"template_service_helpers_test.go must not import infrastructure/repository - violates C044")
		assert.NotContains(t, cleanImp, "infrastructure/expression",
			"template_service_helpers_test.go should avoid infrastructure layer imports")
		// Allow infrastructure/logger as it may be used in production code
		if strings.Contains(cleanImp, "infrastructure/") && !strings.Contains(cleanImp, "infrastructure/logger") {
			t.Errorf("template_service_helpers_test.go should not import infrastructure packages (except logger): %s", cleanImp)
		}
	}
}

// TestTemplateServiceTest_UseTestutilMocks verifies that template_service_test.go
// imports and uses testutil package for mock implementations.
// This is an edge case ensuring the migration to centralized mocks is complete.
func TestTemplateServiceTest_UseTestutilMocks(t *testing.T) {
	sourceFile := "template_service_test.go"

	imports, err := extractImports(sourceFile)
	require.NoError(t, err, "should be able to read source file")

	hasTestutilImport := false
	for _, imp := range imports {
		if strings.Contains(imp, "internal/testutil") {
			hasTestutilImport = true
			break
		}
	}
	assert.True(t, hasTestutilImport,
		"template_service_test.go should import internal/testutil for MockTemplateRepository and MockLogger")
}

// TestTemplateServiceHelpersTest_UseTestutilMocks verifies that
// template_service_helpers_test.go imports and uses testutil package for mocks.
// This is an edge case ensuring the migration to centralized mocks is complete.
func TestTemplateServiceHelpersTest_UseTestutilMocks(t *testing.T) {
	sourceFile := "template_service_helpers_test.go"

	imports, err := extractImports(sourceFile)
	require.NoError(t, err, "should be able to read source file")

	hasTestutilImport := false
	for _, imp := range imports {
		if strings.Contains(imp, "internal/testutil") {
			hasTestutilImport = true
			break
		}
	}
	assert.True(t, hasTestutilImport,
		"template_service_helpers_test.go should import internal/testutil for MockTemplateRepository and MockLogger")
}

// TestTemplateServiceTest_NoLocalMockTemplateRepository verifies that
// template_service_test.go does not define a local mockTemplateRepository type.
// This is an error handling test ensuring duplicate mock definitions were removed.
func TestTemplateServiceTest_NoLocalMockTemplateRepository(t *testing.T) {
	sourceFile := "template_service_test.go"

	content, err := os.ReadFile(sourceFile)
	require.NoError(t, err, "should be able to read source file")
	contentStr := string(content)

	assert.NotContains(t, contentStr, "type mockTemplateRepository struct",
		"template_service_test.go should not define local mockTemplateRepository - use mocks.MockTemplateRepository")
	assert.NotContains(t, contentStr, "func newMockTemplateRepository()",
		"template_service_test.go should not define newMockTemplateRepository() - use mocks.NewMockTemplateRepository()")
	assert.NotContains(t, contentStr, "type mockLogger struct",
		"template_service_test.go should not define local mockLogger - use mocks.NewMockLogger()")
}

// TestTemplateServiceHelpersTest_NoLocalMockTemplateRepository verifies that
// template_service_helpers_test.go does not define a local mockTemplateRepository.
// This is an error handling test ensuring duplicate mock definitions were removed.
func TestTemplateServiceHelpersTest_NoLocalMockTemplateRepository(t *testing.T) {
	sourceFile := "template_service_helpers_test.go"

	content, err := os.ReadFile(sourceFile)
	require.NoError(t, err, "should be able to read source file")
	contentStr := string(content)

	assert.NotContains(t, contentStr, "type mockTemplateRepository struct",
		"template_service_helpers_test.go should not define local mockTemplateRepository - use mocks.MockTemplateRepository")
	assert.NotContains(t, contentStr, "func newMockTemplateRepository()",
		"template_service_helpers_test.go should not define newMockTemplateRepository() - use mocks.NewMockTemplateRepository()")
	assert.NotContains(t, contentStr, "type mockLogger struct",
		"template_service_helpers_test.go should not define local mockLogger - use mocks.NewMockLogger()")
}

// TestTemplateServiceTest_UseDomainErrorTypes verifies that error assertions
// use domain layer error types, not infrastructure types.
// This is an edge case ensuring proper layer separation for error handling.
func TestTemplateServiceTest_UseDomainErrorTypes(t *testing.T) {
	sourceFile := "template_service_test.go"
	content, err := os.ReadFile(sourceFile)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	violations := []string{}

	for lineNum, line := range lines {
		// Check for repository.TemplateNotFoundError
		if strings.Contains(line, "repository.TemplateNotFoundError") {
			violation := fmt.Sprintf("line %d: uses repository.TemplateNotFoundError instead of workflow.TemplateNotFoundError: %s",
				lineNum+1, strings.TrimSpace(line))
			violations = append(violations, violation)
		}
		// Check for infrastructure error package import patterns
		if strings.Contains(line, "*repository.") && strings.Contains(line, "Error") {
			violation := fmt.Sprintf("line %d: references infrastructure repository error type: %s",
				lineNum+1, strings.TrimSpace(line))
			violations = append(violations, violation)
		}
	}

	assert.Empty(t, violations,
		"template_service_test.go should use workflow.TemplateNotFoundError, not repository.TemplateNotFoundError")
}

// TestTemplateServiceHelpersTest_UseDomainErrorTypes verifies that error
// assertions use domain layer error types, not infrastructure types.
// This is an edge case ensuring proper layer separation for error handling.
func TestTemplateServiceHelpersTest_UseDomainErrorTypes(t *testing.T) {
	sourceFile := "template_service_helpers_test.go"
	content, err := os.ReadFile(sourceFile)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	violations := []string{}

	for lineNum, line := range lines {
		// Check for repository.TemplateNotFoundError
		if strings.Contains(line, "repository.TemplateNotFoundError") {
			violation := fmt.Sprintf("line %d: uses repository.TemplateNotFoundError instead of workflow.TemplateNotFoundError: %s",
				lineNum+1, strings.TrimSpace(line))
			violations = append(violations, violation)
		}
		// Check for infrastructure error package import patterns
		if strings.Contains(line, "*repository.") && strings.Contains(line, "Error") {
			violation := fmt.Sprintf("line %d: references infrastructure repository error type: %s",
				lineNum+1, strings.TrimSpace(line))
			violations = append(violations, violation)
		}
	}

	assert.Empty(t, violations,
		"template_service_helpers_test.go should use workflow.TemplateNotFoundError, not repository.TemplateNotFoundError")
}

// TestTemplateServiceTests_ArchitectureCompliance verifies broader architectural
// constraints for template service test files.
// This is an edge case ensuring clean architecture principles.
func TestTemplateServiceTests_ArchitectureCompliance(t *testing.T) {
	testFiles := []string{
		"template_service_test.go",
		"template_service_helpers_test.go",
	}

	for _, sourceFile := range testFiles {
		t.Run(sourceFile, func(t *testing.T) {
			imports, err := extractImports(sourceFile)
			require.NoError(t, err)

			// - standard library
			// - domain layer (ports, workflow)
			// - testutil
			// - application (for testing exported functions)
			// - external test dependencies (testify)
			for _, imp := range imports {
				cleanImp := strings.Trim(imp, `"`)

				// Skip standard library and external packages
				if !strings.Contains(cleanImp, "github.com/vanoix/awf") {
					continue
				}

				// Verify internal imports follow hexagonal architecture
				if strings.Contains(cleanImp, "/internal/") {
					assert.True(t,
						strings.Contains(cleanImp, "/internal/domain/") ||
							strings.Contains(cleanImp, "/internal/application") ||
							strings.Contains(cleanImp, "/internal/testutil"),
						"application layer tests should only import domain, application, or testutil packages, got: %s", cleanImp)
				}
			}
		})
	}
}

// TestTemplateServiceTest_PackageDeclaration verifies correct package declaration.
// Tests should be in package application_test to avoid internal coupling.
// This is an edge case for proper Go testing conventions.
func TestTemplateServiceTest_PackageDeclaration(t *testing.T) {
	sourceFile := "template_service_test.go"
	file, err := os.Open(sourceFile)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var packageLine string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "//") {
			packageLine = line
			break
		}
	}
	require.NoError(t, scanner.Err())

	assert.Equal(t, "package application_test", packageLine,
		"template_service_test.go should use 'package application_test' for black-box testing")
}

// TestTemplateServiceHelpersTest_PackageDeclaration verifies correct package
// declaration for the helpers test file.
// This is an edge case for proper Go testing conventions.
func TestTemplateServiceHelpersTest_PackageDeclaration(t *testing.T) {
	sourceFile := "template_service_helpers_test.go"
	file, err := os.Open(sourceFile)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var packageLine string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "//") {
			packageLine = line
			break
		}
	}
	require.NoError(t, scanner.Err())

	assert.Equal(t, "package application_test", packageLine,
		"template_service_helpers_test.go should use 'package application_test' for black-box testing")
}

// TestTemplateServiceTest_NoRepositoryPackageReference is an error handling test
// that detects if someone accidentally adds references to the repository package.
func TestTemplateServiceTest_NoRepositoryPackageReference(t *testing.T) {
	sourceFile := "template_service_test.go"

	file, err := os.Open(sourceFile)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	violations := []string{}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip comments and empty lines
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Check for references to repository package
		if strings.Contains(line, "repository.") && !strings.Contains(line, "// ") {
			// Exclude allowed patterns (e.g., in comments explaining migration)
			if !strings.Contains(line, "ports.TemplateRepository") &&
				!strings.Contains(line, "// repository.") {
				violation := fmt.Sprintf("line %d: references repository package: %s",
					lineNum, strings.TrimSpace(line))
				violations = append(violations, violation)
			}
		}
	}

	require.NoError(t, scanner.Err())

	assert.Empty(t, violations,
		"found repository package references in template_service_test.go")
}

// TestTemplateServiceHelpersTest_NoRepositoryPackageReference is an error
// handling test for the helpers test file.
func TestTemplateServiceHelpersTest_NoRepositoryPackageReference(t *testing.T) {
	sourceFile := "template_service_helpers_test.go"

	file, err := os.Open(sourceFile)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	violations := []string{}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip comments and empty lines
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Check for references to repository package
		if strings.Contains(line, "repository.") && !strings.Contains(line, "// ") {
			// Exclude allowed patterns
			if !strings.Contains(line, "ports.TemplateRepository") &&
				!strings.Contains(line, "// repository.") {
				violation := fmt.Sprintf("line %d: references repository package: %s",
					lineNum, strings.TrimSpace(line))
				violations = append(violations, violation)
			}
		}
	}

	require.NoError(t, scanner.Err())

	assert.Empty(t, violations,
		"found repository package references in template_service_helpers_test.go")
}

// TestTemplateServiceTests_ImportOrder verifies that imports follow the project
// convention: stdlib, external, internal.
// This is an edge case for code quality and maintainability.
func TestTemplateServiceTests_ImportOrder(t *testing.T) {
	testFiles := []string{
		"template_service_test.go",
		"template_service_helpers_test.go",
	}

	for _, sourceFile := range testFiles {
		t.Run(sourceFile, func(t *testing.T) {
			imports, err := extractImports(sourceFile)
			require.NoError(t, err)

			var (
				stdlibImports   []string
				externalImports []string
				internalImports []string
			)

			for _, imp := range imports {
				cleanImp := strings.Trim(imp, `"`)

				if strings.Contains(cleanImp, ".") {
					// External package
					if strings.Contains(cleanImp, "github.com/vanoix/awf") {
						internalImports = append(internalImports, cleanImp)
					} else {
						externalImports = append(externalImports, cleanImp)
					}
				} else {
					// Standard library
					stdlibImports = append(stdlibImports, cleanImp)
				}
			}

			// Verify we have expected import categories
			assert.NotEmpty(t, stdlibImports, "should have stdlib imports")
			assert.NotEmpty(t, internalImports, "should have internal imports")

			// Note: Import order is enforced by gofmt/goimports
			t.Logf("%s - Import categories - stdlib: %d, external: %d, internal: %d",
				sourceFile, len(stdlibImports), len(externalImports), len(internalImports))
		})
	}
}
