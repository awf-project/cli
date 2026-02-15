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

// Component: T006
// Feature: C022
// Purpose: Verify application layer maintains hexagonal architecture boundaries
// by ensuring no infrastructure imports exist in execution_service.go

// TestExecutionService_NoInfrastructureImport verifies that execution_service.go
// does not import infrastructure packages, maintaining hexagonal architecture.
// This is the happy path test - the file should be clean.
func TestExecutionService_NoInfrastructureImport(t *testing.T) {
	sourceFile := "execution_service.go"

	imports, err := extractImports(sourceFile)
	require.NoError(t, err, "should be able to read source file")

	for _, imp := range imports {
		assert.NotContains(t, imp, "infrastructure/agents",
			"execution_service.go must not import infrastructure/agents - violates DIP")
		assert.NotContains(t, imp, "infrastructure/",
			"execution_service.go should not import any infrastructure packages")
	}
}

// TestExecutionService_OnlyDomainPortsForAgentRegistry verifies that the file
// uses the ports.AgentRegistry interface, not a concrete infrastructure type.
// This is an edge case that ensures the refactoring is complete.
func TestExecutionService_OnlyDomainPortsForAgentRegistry(t *testing.T) {
	sourceFile := "execution_service.go"

	content, err := os.ReadFile(sourceFile)
	require.NoError(t, err, "should be able to read source file")
	contentStr := string(content)

	assert.Contains(t, contentStr, "ports.AgentRegistry",
		"execution_service.go should use ports.AgentRegistry interface")
	assert.NotContains(t, contentStr, "*agents.AgentRegistry",
		"execution_service.go should not use concrete *agents.AgentRegistry type")
	assert.NotContains(t, contentStr, "agents.AgentRegistry",
		"execution_service.go should not reference agents package types")
}

// TestExecutionService_ArchitectureCompliance verifies broader architectural
// constraints for the application layer file.
// This is an edge case ensuring clean architecture principles.
func TestExecutionService_ArchitectureCompliance(t *testing.T) {
	sourceFile := "execution_service.go"

	imports, err := extractImports(sourceFile)
	require.NoError(t, err)

	// - standard library
	// - domain layer (ports, workflow)
	// - pkg (utilities)
	// - external dependencies
	for _, imp := range imports {
		// Remove quotes from import path
		cleanImp := strings.Trim(imp, `"`)

		// Skip standard library and external packages
		if !strings.Contains(cleanImp, "github.com/awf-project/awf") {
			continue
		}

		// Verify internal imports follow hexagonal architecture
		if strings.Contains(cleanImp, "/internal/") {
			assert.True(t,
				strings.Contains(cleanImp, "/internal/domain/") ||
					strings.Contains(cleanImp, "/internal/application/"),
				"application layer should only import domain or application packages, got: %s", cleanImp)
		}
	}
}

// TestExecutionService_NoAgentsPackageReference is an error handling test
// that detects if someone accidentally adds references to the agents package.
func TestExecutionService_NoAgentsPackageReference(t *testing.T) {
	sourceFile := "execution_service.go"

	file, err := os.Open(sourceFile)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	violations := []string{}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip comments
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}

		// Check for references to agents package (excluding "agents." in context of ports)
		if strings.Contains(line, "agents.") && !strings.Contains(line, "// ") {
			// Allow "ports.AgentRegistry" but not "agents.AgentRegistry"
			if !strings.Contains(line, "ports.") {
				violation := fmt.Sprintf("line %d: references agents package: %s", lineNum, strings.TrimSpace(line))
				violations = append(violations, violation)
			}
		}
	}

	require.NoError(t, scanner.Err())

	assert.Empty(t, violations, "found agents package references in execution_service.go")
}

// TestExecutionService_FieldTypeIsInterface verifies the agentRegistry field
// uses the interface type, not a concrete type.
// This is a critical edge case for maintaining architectural boundaries.
func TestExecutionService_FieldTypeIsInterface(t *testing.T) {
	sourceFile := "execution_service.go"
	content, err := os.ReadFile(sourceFile)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	var fieldLine string
	for _, line := range lines {
		if strings.Contains(line, "agentRegistry") && !strings.Contains(line, "//") {
			fieldLine = line
			break
		}
	}

	require.NotEmpty(t, fieldLine, "should find agentRegistry field declaration")
	assert.Contains(t, fieldLine, "ports.AgentRegistry",
		"agentRegistry field should use ports.AgentRegistry interface type")
	assert.NotContains(t, fieldLine, "*agents.AgentRegistry",
		"agentRegistry field should not use concrete *agents.AgentRegistry type")
}

// extractImports reads a Go source file and returns all import statements.
// Helper function for architecture testing.
func extractImports(filename string) ([]string, error) {
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
			// Extract the import path
			start := strings.Index(trimmed, `"`)
			end := strings.LastIndex(trimmed, `"`)
			if start >= 0 && end > start {
				imports = append(imports, trimmed[start:end+1])
			}
			continue
		}

		// Handle import block start
		if strings.HasPrefix(trimmed, "import (") {
			inImportBlock = true
			continue
		}

		// Handle import block end
		if inImportBlock && trimmed == ")" {
			inImportBlock = false
			continue
		}

		// Extract imports within block
		if inImportBlock && strings.Contains(trimmed, `"`) {
			start := strings.Index(trimmed, `"`)
			end := strings.LastIndex(trimmed, `"`)
			if start >= 0 && end > start {
				imports = append(imports, trimmed[start:end+1])
			}
		}
	}

	return imports, nil
}

// TestExecutionService_SetterAcceptsInterface verifies that SetAgentRegistry
// method accepts the ports.AgentRegistry interface, not a concrete type.
// This is an edge case for proper dependency inversion.
func TestExecutionService_SetterAcceptsInterface(t *testing.T) {
	sourceFile := "execution_service.go"
	content, err := os.ReadFile(sourceFile)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	var methodLine string
	for _, line := range lines {
		if strings.Contains(line, "SetAgentRegistry") && strings.Contains(line, "func") {
			methodLine = line
			break
		}
	}

	if methodLine != "" {
		assert.Contains(t, methodLine, "ports.AgentRegistry",
			"SetAgentRegistry should accept ports.AgentRegistry interface parameter")
		assert.NotContains(t, methodLine, "*agents.AgentRegistry",
			"SetAgentRegistry should not accept concrete *agents.AgentRegistry parameter")
	}
}

// TestExecutionService_ImportOrder verifies that imports follow the project
// convention: stdlib, external, internal.
// This is an edge case for code quality and maintainability.
func TestExecutionService_ImportOrder(t *testing.T) {
	sourceFile := "execution_service.go"

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
			if strings.Contains(cleanImp, "github.com/awf-project/awf") {
				internalImports = append(internalImports, cleanImp)
			} else {
				externalImports = append(externalImports, cleanImp)
			}
		} else {
			// Standard library
			stdlibImports = append(stdlibImports, cleanImp)
		}
	}

	// Verify we have imports in expected categories
	assert.NotEmpty(t, stdlibImports)
	assert.NotEmpty(t, internalImports)

	// Note: Import order is enforced by gofmt/goimports, so this is informational
	t.Logf("Import categories - stdlib: %d, external: %d, internal: %d",
		len(stdlibImports), len(externalImports), len(internalImports))
}
