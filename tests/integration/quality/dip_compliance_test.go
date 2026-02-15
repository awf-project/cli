//go:build integration

// Feature: C042
//
// Architecture verification tests for C042 - Fix DIP Violations in Application Layer.
// Validates that the application layer no longer imports infrastructure packages directly,
// ensuring compliance with Dependency Inversion Principle and hexagonal architecture.

package quality_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/infrastructure/expression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoExprLangImportInApplicationLayer verifies that no file in the
// application layer directly imports the expr-lang/expr library.
// This is a critical DIP compliance test - if this fails, it means the
// application layer has a direct infrastructure dependency.
func TestNoExprLangImportInApplicationLayer(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Application layer directory
	appDir := filepath.Join(repoRoot, "internal", "application")
	require.DirExists(t, appDir, "application directory should exist")

	// When: Scanning all Go source files in application layer
	appFiles := findGoFiles(t, appDir)
	require.NotEmpty(t, appFiles, "should find Go files in application layer")

	filesWithExprImport := make(map[string]bool)
	for _, file := range appFiles {
		// Then: No file should import expr-lang/expr
		if hasImport(t, file, "github.com/expr-lang/expr") {
			relPath, _ := filepath.Rel(appDir, file)
			filesWithExprImport[relPath] = true
		}
	}

	if len(filesWithExprImport) > 0 {
		var fileList []string
		for file := range filesWithExprImport {
			fileList = append(fileList, file)
		}
		t.Errorf("Application layer files importing expr-lang/expr (DIP violation):\n  %s",
			strings.Join(fileList, "\n  "))
	}
}

// TestNoInfrastructureImportsInApplicationLayer verifies that no file
// in the application layer imports from internal/infrastructure/.
// This ensures complete hexagonal architecture compliance for the application layer.
func TestNoInfrastructureImportsInApplicationLayer(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Application layer directory
	appDir := filepath.Join(repoRoot, "internal", "application")
	require.DirExists(t, appDir, "application directory should exist")

	// When: Scanning all Go source files in application layer
	appFiles := findGoFiles(t, appDir)
	require.NotEmpty(t, appFiles, "should find Go files in application layer")

	filesWithInfraImport := make(map[string][]string) // file -> list of infrastructure imports
	for _, file := range appFiles {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		require.NoError(t, err, "should parse file %s", file)

		var infraImports []string
		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			// Then: No file should import from internal/infrastructure/
			if strings.Contains(importPath, "internal/infrastructure") {
				infraImports = append(infraImports, importPath)
			}
		}

		if len(infraImports) > 0 {
			relPath, _ := filepath.Rel(appDir, file)
			filesWithInfraImport[relPath] = infraImports
		}
	}

	if len(filesWithInfraImport) > 0 {
		var details strings.Builder
		details.WriteString("Application layer files importing infrastructure packages (DIP violation):\n")
		for file, imports := range filesWithInfraImport {
			details.WriteString("  " + file + ":\n")
			for _, imp := range imports {
				details.WriteString("    - " + imp + "\n")
			}
		}
		t.Errorf("%s", details.String())
	}
}

// TestExpressionEvaluatorPortExists verifies that the ExpressionEvaluator
// interface exists in the domain/ports layer with the correct method signatures.
func TestExpressionEvaluatorPortExists(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Domain ports directory
	portsFile := filepath.Join(repoRoot, "internal", "domain", "ports", "expression_evaluator.go")
	require.FileExists(t, portsFile, "ExpressionEvaluator port file should exist")

	// When: Parsing the port file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, portsFile, nil, 0)
	require.NoError(t, err, "should parse port file")

	// Then: ExpressionEvaluator interface should exist
	hasInterface := false
	hasEvaluateBool := false
	hasEvaluateInt := false

	ast.Inspect(node, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if typeSpec.Name.Name == "ExpressionEvaluator" {
				hasInterface = true
				if iface, ok := typeSpec.Type.(*ast.InterfaceType); ok {
					for _, method := range iface.Methods.List {
						for _, name := range method.Names {
							if name.Name == "EvaluateBool" {
								hasEvaluateBool = true
							}
							if name.Name == "EvaluateInt" {
								hasEvaluateInt = true
							}
						}
					}
				}
			}
		}
		return true
	})

	assert.True(t, hasInterface, "ExpressionEvaluator interface should exist in ports package")
	assert.True(t, hasEvaluateBool, "ExpressionEvaluator should have EvaluateBool method")
	assert.True(t, hasEvaluateInt, "ExpressionEvaluator should have EvaluateInt method")
}

// TestLoopExecutorUsesPort verifies that LoopExecutor uses the
// ports.ExpressionEvaluator interface rather than direct expr-lang types.
func TestLoopExecutorUsesPort(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: LoopExecutor source file
	loopExecFile := filepath.Join(repoRoot, "internal", "application", "loop_executor.go")
	require.FileExists(t, loopExecFile, "loop_executor.go should exist")

	// When: Checking imports in loop_executor.go
	// Then: Should not import expr-lang/expr directly
	hasExprImport := hasImport(t, loopExecFile, "github.com/expr-lang/expr")
	assert.False(t, hasExprImport, "loop_executor.go should not import expr-lang/expr directly")

	// When: Checking for ports.ExpressionEvaluator usage
	content, err := os.ReadFile(loopExecFile)
	require.NoError(t, err, "should read loop_executor.go")

	// Then: Should use ports.ExpressionEvaluator
	usesPortInterface := strings.Contains(string(content), "ports.ExpressionEvaluator") ||
		strings.Contains(string(content), "ExpressionEvaluator")
	assert.True(t, usesPortInterface, "loop_executor.go should use ExpressionEvaluator from ports")
}

// TestInfrastructureAdapterImplementsPort verifies that the infrastructure
// adapter correctly implements the ports.ExpressionEvaluator interface with
// compile-time verification.
func TestInfrastructureAdapterImplementsPort(t *testing.T) {
	// Compile-time verification using var _ pattern
	var _ ports.ExpressionEvaluator = (*expression.ExprEvaluator)(nil)

	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Infrastructure adapter file
	adapterFile := filepath.Join(repoRoot, "internal", "infrastructure", "expression", "expr_evaluator.go")
	require.FileExists(t, adapterFile, "expr_evaluator.go adapter should exist")

	// When: Checking for compile-time interface verification
	// Then: Adapter should have compile-time interface check
	hasCheck := hasCompileTimeInterfaceCheck(t, adapterFile, "ports.ExpressionEvaluator", "ExprEvaluator")
	assert.True(t, hasCheck, "adapter should have compile-time interface verification: var _ ports.ExpressionEvaluator = (*ExprEvaluator)(nil)")
}

// TestPortCompliance_ExpressionEvaluator validates the adapter implementation
func TestPortCompliance_ExpressionEvaluator(t *testing.T) {
	// Verify compile-time interface compliance
	var _ ports.ExpressionEvaluator = (*expression.ExprEvaluator)(nil)

	// Create evaluator instance
	evaluator := expression.NewExprEvaluator()
	require.NotNil(t, evaluator)

	// Verify interface methods exist and can be called
	// (actual behavior is tested in unit tests, this just verifies the contract)
	// Note: This is a structural test, not a behavioral test
	// We're just verifying the methods exist and can be invoked
	t.Run("EvaluateBool method exists", func(t *testing.T) {
		// This will compile if the method signature matches the interface
		_ = func(expr string) (bool, error) {
			// Using nil context is acceptable here - we're testing compilation, not behavior
			return evaluator.EvaluateBool(expr, nil)
		}
	})

	t.Run("EvaluateInt method exists", func(t *testing.T) {
		// This will compile if the method signature matches the interface
		_ = func(expr string) (int, error) {
			// Using nil context is acceptable here - we're testing compilation, not behavior
			return evaluator.EvaluateInt(expr, nil)
		}
	})
}

// TestApplicationLayerPurity validates overall architecture compliance
func TestApplicationLayerPurity(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Application layer directory
	appDir := filepath.Join(repoRoot, "internal", "application")
	require.DirExists(t, appDir, "application directory should exist")

	// When: Scanning all application layer files
	appFiles := findGoFiles(t, appDir)
	require.NotEmpty(t, appFiles, "should find Go files in application layer")

	violations := make(map[string][]string) // file -> list of violations

	for _, file := range appFiles {
		relPath, _ := filepath.Rel(appDir, file)
		var fileViolations []string

		// Check for expr-lang import
		if hasImport(t, file, "github.com/expr-lang/expr") {
			fileViolations = append(fileViolations, "imports expr-lang/expr")
		}

		// Check for infrastructure imports
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		require.NoError(t, err, "should parse file %s", file)

		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(importPath, "internal/infrastructure") {
				fileViolations = append(fileViolations, "imports "+importPath)
			}
		}

		if len(fileViolations) > 0 {
			violations[relPath] = fileViolations
		}
	}

	// Then: Application layer should be pure (no infrastructure dependencies)
	if len(violations) > 0 {
		var details strings.Builder
		details.WriteString("Application layer purity violations detected:\n")
		for file, fileViolations := range violations {
			details.WriteString("  " + file + ":\n")
			for _, violation := range fileViolations {
				details.WriteString("    - " + violation + "\n")
			}
		}
		t.Errorf("%s", details.String())
	}
}

// Helper functions

// findRepoRoot finds the repository root by looking for go.mod file
func findRepoRoot() (string, error) {
	// Start from current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// findGoFiles finds all .go files (non-test) in a directory recursively
func findGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})

	require.NoError(t, err, "should walk directory %s", dir)
	return files
}

// hasImport checks if a Go file imports a specific package
func hasImport(t *testing.T, filePath, importPath string) bool {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	require.NoError(t, err, "should parse file %s", filePath)

	for _, imp := range node.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path == importPath || strings.Contains(path, importPath) {
			return true
		}
	}

	return false
}

// findInterfaceMethod searches for a specific method in an interface definition
func findInterfaceMethod(t *testing.T, filePath, interfaceName, methodName string) bool {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, 0)
	require.NoError(t, err, "should parse file %s", filePath)

	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		// Look for interface type specs
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if typeSpec.Name.Name == interfaceName {
				if iface, ok := typeSpec.Type.(*ast.InterfaceType); ok {
					for _, method := range iface.Methods.List {
						for _, name := range method.Names {
							if name.Name == methodName {
								found = true
								return false
							}
						}
					}
				}
			}
		}
		return true
	})

	return found
}

// hasCompileTimeInterfaceCheck verifies that a file contains a compile-time
// interface verification statement like: var _ ports.Interface = (*Type)(nil)
func hasCompileTimeInterfaceCheck(t *testing.T, filePath, interfaceName, typeName string) bool {
	t.Helper()

	content, err := os.ReadFile(filePath)
	require.NoError(t, err, "should read file %s", filePath)

	// Simple pattern matching for compile-time check
	pattern := "var _ " + interfaceName
	return strings.Contains(string(content), pattern)
}

// hasCompileTimeInterfaceCheckFlexible checks for compile-time interface verification
// in both individual var statement and var() block formats.
func hasCompileTimeInterfaceCheckFlexible(t *testing.T, filePath, interfaceName, typeName string) bool {
	t.Helper()

	content, err := os.ReadFile(filePath)
	require.NoError(t, err, "should read file %s", filePath)

	// Check for the pattern: _ InterfaceName = (*TypeName)(nil)
	// This matches both:
	// 1. Individual statement: var _ ports.Interface = (*Type)(nil)
	// 2. Block format: \t_ ports.Interface     = (*Type)(nil)
	// We just need to find the interface assignment, regardless of prefix whitespace
	pattern := "_ " + interfaceName
	typePattern := "(*" + typeName + ")(nil)"

	contentStr := string(content)
	return strings.Contains(contentStr, pattern) && strings.Contains(contentStr, typePattern)
}
