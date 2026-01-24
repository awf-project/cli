//go:build integration

// Component: T011
// Feature: C017

package cli_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationTestMigration_AllIntegrationTestsMoved verifies that all
// integration-style tests (those calling cmd.Execute()) have been moved from
// internal/interfaces/cli/ to tests/integration/cli/
func TestIntegrationTestMigration_AllIntegrationTestsMoved(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Source directory with CLI test files
	sourceDir := filepath.Join(repoRoot, "internal", "interfaces", "cli")
	require.DirExists(t, sourceDir, "source directory should exist")

	// When: Scanning source directory for integration-style tests
	testFiles := findTestFiles(t, sourceDir)
	integrationTests := make(map[string][]string) // file -> test function names

	for _, file := range testFiles {
		// Skip UI subdirectory (out of scope for C017)
		if strings.Contains(file, "/ui/") {
			continue
		}

		// Skip helper test files (not actual tests)
		if strings.HasSuffix(file, "_helpers_test.go") {
			continue
		}

		// Parse file and find cmd.Execute() calls
		funcs := findIntegrationTestFunctions(t, file)
		if len(funcs) > 0 {
			relPath, _ := filepath.Rel(sourceDir, file)
			integrationTests[relPath] = funcs
		}
	}

	// Then: No integration tests should remain in source directory
	if len(integrationTests) > 0 {
		var details strings.Builder
		details.WriteString("Found integration-style tests still in internal/interfaces/cli/:\n")
		for file, funcs := range integrationTests {
			details.WriteString("  " + file + ":\n")
			for _, fn := range funcs {
				details.WriteString("    - " + fn + "\n")
			}
		}
		t.Errorf("%s", details.String())
	}
}

// TestIntegrationTestMigration_BuildTagsPresent verifies that all files in
// tests/integration/cli/ have the //go:build integration tag
func TestIntegrationTestMigration_BuildTagsPresent(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Integration test directory
	targetDir := filepath.Join(repoRoot, "tests", "integration", "cli")

	// Skip test if directory doesn't exist yet (before migration)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Skip("Integration test directory not created yet")
	}

	require.DirExists(t, targetDir, "integration test directory should exist")

	// When: Scanning all Go files in integration directory
	testFiles := findTestFiles(t, targetDir)

	filesWithoutTag := []string{}
	for _, file := range testFiles {
		// Skip doc.go (doesn't need tests)
		if strings.HasSuffix(file, "doc.go") {
			continue
		}

		content, err := os.ReadFile(file)
		require.NoError(t, err, "should read file %s", file)

		// Then: Each file should have build tag on first line
		lines := strings.Split(string(content), "\n")
		if len(lines) == 0 || !strings.Contains(lines[0], "//go:build integration") {
			relPath, _ := filepath.Rel(targetDir, file)
			filesWithoutTag = append(filesWithoutTag, relPath)
		}
	}

	if len(filesWithoutTag) > 0 {
		t.Errorf("Files missing //go:build integration tag:\n  %s",
			strings.Join(filesWithoutTag, "\n  "))
	}
}

// TestIntegrationTestMigration_PackageNaming verifies that all integration
// test files use the correct package name (cli_test)
func TestIntegrationTestMigration_PackageNaming(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Integration test directory
	targetDir := filepath.Join(repoRoot, "tests", "integration", "cli")

	// Skip test if directory doesn't exist yet
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Skip("Integration test directory not created yet")
	}

	require.DirExists(t, targetDir, "integration test directory should exist")

	// When: Scanning all Go files
	testFiles := findGoFiles(t, targetDir)

	incorrectPackages := make(map[string]string) // file -> package name
	for _, file := range testFiles {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.PackageClauseOnly)
		require.NoError(t, err, "should parse file %s", file)

		// Then: Package should be cli_test
		if node.Name.Name != "cli_test" {
			relPath, _ := filepath.Rel(targetDir, file)
			incorrectPackages[relPath] = node.Name.Name
		}
	}

	if len(incorrectPackages) > 0 {
		var details strings.Builder
		details.WriteString("Files with incorrect package name (expected cli_test):\n")
		for file, pkg := range incorrectPackages {
			details.WriteString("  " + file + ": package " + pkg + "\n")
		}
		t.Error(details.String())
	}
}

// TestIntegrationTestMigration_TestCountStability verifies that the total
// number of tests remains stable after migration (no tests lost)
func TestIntegrationTestMigration_TestCountStability(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Baseline test count (from plan.md: 456 total, 280 integration)
	baselineTotal := 456
	baselineIntegration := 280

	// When: Counting tests in both locations
	sourceDir := filepath.Join(repoRoot, "internal", "interfaces", "cli")
	targetDir := filepath.Join(repoRoot, "tests", "integration", "cli")

	sourceTests := countTestFunctions(t, sourceDir)

	var targetTests int
	if _, err := os.Stat(targetDir); err == nil {
		targetTests = countTestFunctions(t, targetDir)
	}

	totalTests := sourceTests + targetTests

	// Then: Total should match baseline
	// Allow small variance (±5%) for refactoring/consolidation
	tolerance := int(float64(baselineTotal) * 0.05)

	assert.InDelta(t, baselineTotal, totalTests, float64(tolerance),
		"Total test count should remain stable (source: %d, target: %d, total: %d, baseline: %d)",
		sourceTests, targetTests, totalTests, baselineTotal)

	// After migration is complete, verify integration test count
	if targetTests > 0 {
		// Should have moved approximately 280 integration tests
		integrationTolerance := int(float64(baselineIntegration) * 0.10)
		assert.InDelta(t, baselineIntegration, targetTests, float64(integrationTolerance),
			"Integration test count in target should match baseline (got: %d, baseline: %d)",
			targetTests, baselineIntegration)
	}
}

// TestIntegrationTestMigration_NoUnitTestsInIntegration verifies that only
// integration-style tests (with cmd.Execute()) are in tests/integration/cli/
func TestIntegrationTestMigration_NoUnitTestsInIntegration(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Integration test directory
	targetDir := filepath.Join(repoRoot, "tests", "integration", "cli")

	// Skip test if directory doesn't exist yet
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Skip("Integration test directory not created yet")
	}

	require.DirExists(t, targetDir, "integration test directory should exist")

	// When: Scanning all test files
	testFiles := findTestFiles(t, targetDir)

	filesWithoutIntegrationPattern := []string{}
	for _, file := range testFiles {
		// Skip helper files and doc.go
		if strings.HasSuffix(file, "_helpers_test.go") || strings.HasSuffix(file, "doc.go") {
			continue
		}

		content, err := os.ReadFile(file)
		require.NoError(t, err, "should read file %s", file)

		// Then: Integration tests should have cmd.Execute() calls
		// (or other integration patterns like workflow execution)
		hasIntegrationPattern := strings.Contains(string(content), "cmd.Execute()") ||
			strings.Contains(string(content), "NewRootCommand()") ||
			strings.Contains(string(content), "workflowService.Execute")

		if !hasIntegrationPattern {
			relPath, _ := filepath.Rel(targetDir, file)
			filesWithoutIntegrationPattern = append(filesWithoutIntegrationPattern, relPath)
		}
	}

	if len(filesWithoutIntegrationPattern) > 0 {
		t.Logf("WARNING: Files in integration directory without clear integration patterns:\n  %s",
			strings.Join(filesWithoutIntegrationPattern, "\n  "))
	}
}

// TestIntegrationTestMigration_SourceDirectoryStructure verifies that after
// migration, internal/interfaces/cli/ retains only unit test files
func TestIntegrationTestMigration_SourceDirectoryStructure(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Source directory with CLI tests
	sourceDir := filepath.Join(repoRoot, "internal", "interfaces", "cli")
	require.DirExists(t, sourceDir, "source directory should exist")

	// When: Scanning for remaining test files
	testFiles := findTestFiles(t, sourceDir)

	// Expected unit test patterns (from plan.md)
	expectedUnitTestFiles := map[string]bool{
		"run_flags_test.go":    true,
		"run_help_test.go":     true,
		"root_test.go":         true,
		"plugins_test.go":      true,
		"list_helpers_test.go": true,
		"exitcodes_test.go":    true,
	}

	unexpectedFiles := []string{}
	for _, file := range testFiles {
		basename := filepath.Base(file)

		// Skip helper files and UI subdirectory
		if strings.HasSuffix(basename, "_helpers_test.go") || strings.Contains(file, "/ui/") {
			continue
		}

		// Skip expected unit test files
		if expectedUnitTestFiles[basename] {
			continue
		}

		// Then: Unexpected test files should be flagged
		// (they should have been moved to integration/)
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Only flag if it contains integration patterns
		if strings.Contains(string(content), "cmd.Execute()") {
			unexpectedFiles = append(unexpectedFiles, basename)
		}
	}

	if len(unexpectedFiles) > 0 {
		t.Errorf("Found unexpected integration-style test files in source directory:\n  %s\n"+
			"These should be moved to tests/integration/cli/",
			strings.Join(unexpectedFiles, "\n  "))
	}
}

// TestIntegrationTestMigration_ImportsUpdated verifies that migrated tests
// properly import from the public CLI package
func TestIntegrationTestMigration_ImportsUpdated(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Integration test directory
	targetDir := filepath.Join(repoRoot, "tests", "integration", "cli")

	// Skip test if directory doesn't exist yet
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Skip("Integration test directory not created yet")
	}

	require.DirExists(t, targetDir, "integration test directory should exist")

	// When: Scanning all test files for imports
	testFiles := findTestFiles(t, targetDir)

	filesWithBadImports := []string{}
	for _, file := range testFiles {
		// Skip doc.go
		if strings.HasSuffix(file, "doc.go") {
			continue
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		require.NoError(t, err, "should parse file %s", file)

		// Then: Should import github.com/vanoix/awf/internal/interfaces/cli
		hasCliImport := false
		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(importPath, "internal/interfaces/cli") {
				hasCliImport = true
				break
			}
		}

		if !hasCliImport {
			relPath, _ := filepath.Rel(targetDir, file)
			filesWithBadImports = append(filesWithBadImports, relPath)
		}
	}

	if len(filesWithBadImports) > 0 {
		t.Logf("WARNING: Files without CLI package import (may be intentional for doc.go):\n  %s",
			strings.Join(filesWithBadImports, "\n  "))
	}
}

// TestIntegrationTestMigration_NoThreadUnsafePatterns verifies that migrated
// tests don't use thread-unsafe patterns like os.Chdir
func TestIntegrationTestMigration_NoThreadUnsafePatterns(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Integration test directory
	targetDir := filepath.Join(repoRoot, "tests", "integration", "cli")

	// Skip test if directory doesn't exist yet
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Skip("Integration test directory not created yet")
	}

	require.DirExists(t, targetDir, "integration test directory should exist")

	// When: Scanning for os.Chdir usage
	testFiles := findTestFiles(t, targetDir)

	filesWithChdir := make(map[string][]int) // file -> line numbers
	for _, file := range testFiles {
		content, err := os.ReadFile(file)
		require.NoError(t, err, "should read file %s", file)

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			// Skip comments
			if strings.Contains(line, "//") && strings.Index(line, "//") < strings.Index(line, "os.Chdir") {
				continue
			}

			// Then: Should not contain os.Chdir calls
			if strings.Contains(line, "os.Chdir(") {
				relPath, _ := filepath.Rel(targetDir, file)
				filesWithChdir[relPath] = append(filesWithChdir[relPath], i+1)
			}
		}
	}

	if len(filesWithChdir) > 0 {
		var details strings.Builder
		details.WriteString("Found thread-unsafe os.Chdir calls in migrated tests:\n")
		for file, lines := range filesWithChdir {
			details.WriteString("  " + file + " at lines: ")
			for i, line := range lines {
				if i > 0 {
					details.WriteString(", ")
				}
				details.WriteString(fmt.Sprintf("%d", line))
			}
			details.WriteString("\n")
		}
		t.Errorf("%s", details.String())
	}
}

// Helper functions

func findTestFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})

	require.NoError(t, err, "should walk directory %s", dir)
	return files
}

func findGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})

	require.NoError(t, err, "should walk directory %s", dir)
	return files
}

func findIntegrationTestFunctions(t *testing.T, file string) []string {
	t.Helper()

	content, err := os.ReadFile(file)
	if err != nil {
		return nil
	}

	// Simple heuristic: if file contains cmd.Execute(), it has integration tests
	if !strings.Contains(string(content), "cmd.Execute()") {
		return nil
	}

	// Parse file to find test function names
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return nil
	}

	var funcs []string
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if strings.HasPrefix(fn.Name.Name, "Test") {
				funcs = append(funcs, fn.Name.Name)
			}
		}
		return true
	})

	return funcs
}

func countTestFunctions(t *testing.T, dir string) int {
	t.Helper()

	// Skip if directory doesn't exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0
	}

	testFiles := findTestFiles(t, dir)
	count := 0

	for _, file := range testFiles {
		// Skip UI subdirectory
		if strings.Contains(file, "/ui/") {
			continue
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			continue
		}

		ast.Inspect(node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				if strings.HasPrefix(fn.Name.Name, "Test") {
					count++
				}
			}
			return true
		})
	}

	return count
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check if go.mod exists (indicates repo root)
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
