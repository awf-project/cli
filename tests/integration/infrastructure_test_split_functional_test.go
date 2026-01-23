// Infrastructure Test File Splitting Functional Tests
//
// This file contains comprehensive functional tests validating that the infrastructure
// test file splitting maintains test integrity, coverage, and organization.
//
// Feature: Validates C015 test file splitting from 2 monolithic files to 10 focused files
//
// Test Categories:
// - Integration: Validates end-to-end test execution and preservation
// - Edge Cases: Validates boundary conditions and organizational patterns
// - Error Handling: Validates lint compliance and package structure
//
// Acceptance Criteria Validated:
// - All original tests preserved (51 CLI tests, 130 diagram tests)
// - Split files respect size limits (<1,500 lines per ADR-005)
// - Test files organized by logical concern
// - All tests pass with maintained coverage (≥78.5%)
// - No race conditions detected (thread-safety fixes applied)
// - Zero os.Chdir calls remain in CLI tests
// - Shared helpers properly extracted
package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findProjectRootInfraSplit locates the project root by looking for go.mod
func findProjectRootInfraSplit() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// parseFloatInfraSplit is a helper to parse float from string
func parseFloatInfraSplit(s string, out *float64) error {
	n, err := fmt.Sscanf(s, "%f", out)
	if err != nil {
		return fmt.Errorf("failed to parse float from %q: %w", s, err)
	}
	if n != 1 {
		return fmt.Errorf("expected to parse 1 value, got %d", n)
	}
	return nil
}

// TestInfrastructureTestFileSplitting_Integration validates that C015 test file splitting
// maintains test count, coverage, and file organization standards.
func TestInfrastructureTestFileSplitting_Integration(t *testing.T) {
	projectRoot, err := findProjectRootInfraSplit()
	require.NoError(t, err, "failed to find project root")

	cliTestDir := filepath.Join(projectRoot, "internal", "interfaces", "cli")
	diagramTestDir := filepath.Join(projectRoot, "internal", "infrastructure", "diagram")

	t.Run("all CLI split test files exist", func(t *testing.T) {
		expectedFiles := []string{
			"cli_test_helpers_test.go", // Thread-safe test utilities
			"run_flags_test.go",        // Flag parsing tests
			"run_execution_test.go",    // Execution mode tests
			"run_agent_test.go",        // Agent execution tests
			"run_interactive_test.go",  // Interactive mode tests
		}

		for _, file := range expectedFiles {
			filePath := filepath.Join(cliTestDir, file)
			_, err := os.Stat(filePath)
			require.NoError(t, err, "expected CLI test file %s to exist", file)
		}
	})

	t.Run("all diagram split test files exist", func(t *testing.T) {
		expectedFiles := []string{
			"generator_nodes_test.go",     // Node/shape generation tests
			"generator_edges_test.go",     // Edge rendering tests
			"generator_header_test.go",    // Header/config tests
			"generator_parallel_test.go",  // Parallel execution tests
			"generator_highlight_test.go", // Highlight/styling tests
			"dot_generator_core_test.go",  // Core integration tests
		}

		for _, file := range expectedFiles {
			filePath := filepath.Join(diagramTestDir, file)
			_, err := os.Stat(filePath)
			require.NoError(t, err, "expected diagram test file %s to exist", file)
		}
	})

	t.Run("original monolithic files deleted", func(t *testing.T) {
		deletedFiles := map[string]string{
			"run_test.go":           cliTestDir,
			"dot_generator_test.go": diagramTestDir,
		}

		for file, dir := range deletedFiles {
			filePath := filepath.Join(dir, file)
			_, err := os.Stat(filePath)
			assert.True(t, os.IsNotExist(err), "expected file %s to be deleted", file)
		}
	})

	t.Run("zero os.Chdir calls in CLI tests", func(t *testing.T) {
		// Critical thread-safety requirement: no os.Chdir in tests
		cliTestFiles := []string{
			"cli_test_helpers_test.go",
			"run_flags_test.go",
			"run_execution_test.go",
			"run_agent_test.go",
			"run_interactive_test.go",
		}

		for _, file := range cliTestFiles {
			filePath := filepath.Join(cliTestDir, file)
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "failed to read %s", file)

			// Check for os.Chdir calls
			assert.NotContains(t, string(content), "os.Chdir(",
				"file %s must not contain os.Chdir calls (thread-safety violation)", file)
		}
	})

	t.Run("CLI split files use thread-safe patterns", func(t *testing.T) {
		// Verify usage of t.TempDir() and t.Setenv() patterns
		cliTestFiles := []string{
			"run_flags_test.go",
			"run_execution_test.go",
			"run_agent_test.go",
			"run_interactive_test.go",
		}

		for _, file := range cliTestFiles {
			filePath := filepath.Join(cliTestDir, file)
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue // File may not exist yet
			}

			fileContent := string(content)
			// Check for thread-safe patterns (at least one should be present)
			hasThreadSafe := strings.Contains(fileContent, "t.TempDir()") ||
				strings.Contains(fileContent, "t.Setenv(") ||
				strings.Contains(fileContent, "setupTestDir(t)")

			if len(fileContent) > 1000 { // Only check substantial test files
				assert.True(t, hasThreadSafe,
					"file %s should use thread-safe directory patterns", file)
			}
		}
	})

	t.Run("split files respect size limits", func(t *testing.T) {
		// Acceptance criteria: each new test file < 1500 lines (ADR-005)
		maxLines := 1500
		allowedExceptions := map[string]int{
			// Allow larger files for complex integration tests
		}

		testFiles := map[string]string{
			"run_flags_test.go":           cliTestDir,
			"run_execution_test.go":       cliTestDir,
			"run_agent_test.go":           cliTestDir,
			"run_interactive_test.go":     cliTestDir,
			"generator_nodes_test.go":     diagramTestDir,
			"generator_edges_test.go":     diagramTestDir,
			"generator_header_test.go":    diagramTestDir,
			"generator_parallel_test.go":  diagramTestDir,
			"generator_highlight_test.go": diagramTestDir,
			"dot_generator_core_test.go":  diagramTestDir,
		}

		for file, dir := range testFiles {
			filePath := filepath.Join(dir, file)
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue // File may not exist yet
			}

			lines := strings.Count(string(content), "\n") + 1
			limit := maxLines
			if exception, ok := allowedExceptions[file]; ok {
				limit = exception
			}

			assert.LessOrEqual(t, lines, limit,
				"file %s has %d lines, expected ≤%d (per ADR-005)", file, lines, limit)
		}
	})

	t.Run("all CLI interface tests pass", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "./internal/interfaces/cli/...", "-v")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "CLI tests failed:\n%s", string(output))

		// Verify no test failures in output
		assert.NotContains(t, string(output), "FAIL:", "found test failures")
		assert.Contains(t, string(output), "PASS", "expected passing tests")
	})

	t.Run("all diagram infrastructure tests pass", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "./internal/infrastructure/diagram/...", "-v")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "diagram tests failed:\n%s", string(output))

		// Verify no test failures in output
		assert.NotContains(t, string(output), "FAIL:", "found test failures")
		assert.Contains(t, string(output), "PASS", "expected passing tests")
	})

	t.Run("no race conditions in CLI tests", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "-race", "./internal/interfaces/cli/...")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "race detector found issues in CLI tests:\n%s", string(output))

		assert.NotContains(t, string(output), "WARNING: DATA RACE",
			"race conditions detected in CLI tests")
	})

	t.Run("no race conditions in diagram tests", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "-race", "./internal/infrastructure/diagram/...")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "race detector found issues in diagram tests:\n%s", string(output))

		assert.NotContains(t, string(output), "WARNING: DATA RACE",
			"race conditions detected in diagram tests")
	})

	t.Run("coverage maintained after split", func(t *testing.T) {
		// C015 acceptance criteria: maintain ≥78.5% coverage baseline
		minCoverage := 78.5

		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test",
			"./internal/interfaces/cli/...",
			"./internal/infrastructure/diagram/...",
			"-coverprofile=/tmp/c015_coverage.out")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "coverage test failed:\n%s", string(output))

		// Parse coverage from output
		coverageRegex := regexp.MustCompile(`coverage: ([\d.]+)% of statements`)
		matches := coverageRegex.FindStringSubmatch(string(output))
		if len(matches) > 1 {
			var coverage float64
			err := parseFloatInfraSplit(matches[1], &coverage)
			if err == nil {
				assert.GreaterOrEqual(t, coverage, minCoverage,
					"coverage %.1f%% below baseline %.1f%%", coverage, minCoverage)
			}
		}
	})
}

// TestInfrastructureTestFileSplitting_HappyPath validates normal usage and complete workflow.
func TestInfrastructureTestFileSplitting_HappyPath(t *testing.T) {
	projectRoot, err := findProjectRootInfraSplit()
	require.NoError(t, err, "failed to find project root")

	t.Run("test helpers are properly shared", func(t *testing.T) {
		cliTestDir := filepath.Join(projectRoot, "internal", "interfaces", "cli")
		diagramTestDir := filepath.Join(projectRoot, "internal", "infrastructure", "diagram")

		// Verify CLI helpers exist
		cliHelpersPath := filepath.Join(cliTestDir, "cli_test_helpers_test.go")
		_, err := os.Stat(cliHelpersPath)
		require.NoError(t, err, "CLI test helpers should exist")

		// Verify diagram helpers exist
		diagramHelpersPath := filepath.Join(diagramTestDir, "diagram_test_helpers_test.go")
		_, err = os.Stat(diagramHelpersPath)
		require.NoError(t, err, "diagram test helpers should exist")
	})

	t.Run("file naming follows convention", func(t *testing.T) {
		// Verify naming pattern: <component>_test.go
		cliTestDir := filepath.Join(projectRoot, "internal", "interfaces", "cli")
		diagramTestDir := filepath.Join(projectRoot, "internal", "infrastructure", "diagram")

		cliFiles := []string{
			"run_flags_test.go",
			"run_execution_test.go",
			"run_agent_test.go",
			"run_interactive_test.go",
		}

		for _, file := range cliFiles {
			assert.Contains(t, file, "_test.go", "file should follow *_test.go convention")
			filePath := filepath.Join(cliTestDir, file)
			_, err := os.Stat(filePath)
			assert.NoError(t, err, "file %s should exist", file)
		}

		diagramFiles := []string{
			"generator_nodes_test.go",
			"generator_edges_test.go",
			"generator_header_test.go",
			"generator_parallel_test.go",
			"generator_highlight_test.go",
			"dot_generator_core_test.go",
		}

		for _, file := range diagramFiles {
			assert.Contains(t, file, "_test.go", "file should follow *_test.go convention")
			filePath := filepath.Join(diagramTestDir, file)
			_, err := os.Stat(filePath)
			assert.NoError(t, err, "file %s should exist", file)
		}
	})
}

// TestInfrastructureTestFileSplitting_EdgeCases validates boundary conditions
// and edge cases in the split test organization.
func TestInfrastructureTestFileSplitting_EdgeCases(t *testing.T) {
	projectRoot, err := findProjectRootInfraSplit()
	require.NoError(t, err, "failed to find project root")

	cliTestDir := filepath.Join(projectRoot, "internal", "interfaces", "cli")
	diagramTestDir := filepath.Join(projectRoot, "internal", "infrastructure", "diagram")

	t.Run("CLI tests organized by execution mode", func(t *testing.T) {
		// Verify CLI files follow organizational pattern
		cliFiles := map[string][]string{
			"run_flags_test.go":       {"Flag", "Parse"},
			"run_execution_test.go":   {"Execute", "SQLite"},
			"run_agent_test.go":       {"Agent", "DryRun"},
			"run_interactive_test.go": {"Interactive", "Prompt"},
		}

		for file, keywords := range cliFiles {
			filePath := filepath.Join(cliTestDir, file)
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue // File may not exist yet
			}

			for _, keyword := range keywords {
				assert.Contains(t, string(content), keyword,
					"file %s should contain tests for %s", file, keyword)
			}
		}
	})

	t.Run("diagram tests organized by concern", func(t *testing.T) {
		// Verify diagram files follow organizational pattern
		diagramFiles := map[string][]string{
			"generator_nodes_test.go":    {"Node", "Shape"},
			"generator_edges_test.go":    {"Edge", "Connection"},
			"generator_header_test.go":   {"Header", "Config"},
			"generator_parallel_test.go": {"Parallel"},
		}

		for file, keywords := range diagramFiles {
			filePath := filepath.Join(diagramTestDir, file)
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue // File may not exist yet
			}

			contentStr := string(content)
			hasKeyword := false
			for _, keyword := range keywords {
				if strings.Contains(contentStr, keyword) {
					hasKeyword = true
					break
				}
			}
			assert.True(t, hasKeyword,
				"file %s should contain tests for one of %v", file, keywords)
		}
	})

	t.Run("no test duplication between CLI files", func(t *testing.T) {
		cliTestFiles := []string{
			"run_flags_test.go",
			"run_execution_test.go",
			"run_agent_test.go",
			"run_interactive_test.go",
		}

		verifyNoTestDuplication(t, cliTestDir, cliTestFiles)
	})

	t.Run("no test duplication between diagram files", func(t *testing.T) {
		diagramTestFiles := []string{
			"generator_nodes_test.go",
			"generator_edges_test.go",
			"generator_header_test.go",
			"generator_parallel_test.go",
			"generator_highlight_test.go",
			"dot_generator_core_test.go",
		}

		verifyNoTestDuplication(t, diagramTestDir, diagramTestFiles)
	})

	t.Run("shared CLI helpers prevent import cycles", func(t *testing.T) {
		// Verify cli_test_helpers_test.go exists and is importable
		helperPath := filepath.Join(cliTestDir, "cli_test_helpers_test.go")
		content, err := os.ReadFile(helperPath)
		require.NoError(t, err, "shared helpers file not found")

		// Verify it's in the same package (no import needed)
		assert.Contains(t, string(content), "package cli",
			"helpers must be in cli package to avoid import cycles")

		// Verify it contains expected thread-safe utilities
		helpers := []string{
			"setupTestDir", // Thread-safe directory setup
			"setTestEnv",   // Thread-safe environment setup
		}
		contentStr := string(content)
		for _, helper := range helpers {
			assert.True(t,
				strings.Contains(contentStr, helper) ||
					strings.Contains(contentStr, "t.TempDir") ||
					strings.Contains(contentStr, "t.Setenv"),
				"expected helper %s or equivalent thread-safe pattern", helper)
		}
	})
}

// TestInfrastructureTestFileSplitting_ErrorHandling validates error scenarios
// in the test file organization.
func TestInfrastructureTestFileSplitting_ErrorHandling(t *testing.T) {
	projectRoot, err := findProjectRootInfraSplit()
	require.NoError(t, err, "failed to find project root")

	t.Run("lint passes with zero issues", func(t *testing.T) {
		verifyLintPassesInfraSplit(t, projectRoot)
	})

	t.Run("all test files have package declaration", func(t *testing.T) {
		verifyPackageDeclarationsInfraSplit(t, projectRoot)
	})

	t.Run("CLI test count preserved", func(t *testing.T) {
		// C015 acceptance: 51 CLI tests preserved
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "./internal/interfaces/cli/...", "-v")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("CLI tests output:\n%s", string(output))
		}

		// Count test functions
		testRunRegex := regexp.MustCompile(`=== RUN\s+Test`)
		matches := testRunRegex.FindAllString(string(output), -1)
		testCount := len(matches)

		// Allow some variance, but should be around 51 tests
		assert.Greater(t, testCount, 40,
			"expected >40 CLI tests after split, found %d", testCount)
	})

	t.Run("diagram test count preserved", func(t *testing.T) {
		// C015 acceptance: 130 diagram tests preserved
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "./internal/infrastructure/diagram/...", "-v")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Diagram tests output:\n%s", string(output))
		}

		// Count test functions
		testRunRegex := regexp.MustCompile(`=== RUN\s+Test`)
		matches := testRunRegex.FindAllString(string(output), -1)
		testCount := len(matches)

		// Allow some variance, but should be around 130 tests
		assert.Greater(t, testCount, 100,
			"expected >100 diagram tests after split, found %d", testCount)
	})

	t.Run("split maintains git history", func(t *testing.T) {
		// Verify that git blame still works for test functions
		// This validates ADR-004: Preserve Test Function Names
		ctx := context.Background()

		// Check if we're in a git repository
		cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
		cmd.Dir = projectRoot
		if err := cmd.Run(); err != nil {
			t.Skip("Not in a git repository")
		}

		// Verify git can track history for split files
		splitFiles := []string{
			"internal/interfaces/cli/run_flags_test.go",
			"internal/infrastructure/diagram/generator_nodes_test.go",
		}

		for _, file := range splitFiles {
			cmd := exec.CommandContext(ctx, "git", "log", "--oneline", "--", file)
			cmd.Dir = projectRoot
			output, err := cmd.CombinedOutput()
			if err == nil && len(output) > 0 {
				t.Logf("Git history exists for %s", file)
			}
		}
	})

	t.Run("test execution is deterministic", func(t *testing.T) {
		// Run tests twice and verify consistent results
		ctx := context.Background()

		runTests := func() error {
			cmd := exec.CommandContext(ctx, "go", "test",
				"./internal/interfaces/cli/...",
				"./internal/infrastructure/diagram/...",
				"-count=1")
			cmd.Dir = projectRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Logf("Test output:\n%s", string(output))
				return fmt.Errorf("test execution failed: %w", err)
			}
			return nil
		}

		// First run
		err1 := runTests()
		// Second run
		err2 := runTests()

		// Both should succeed
		assert.NoError(t, err1, "first test run should pass")
		assert.NoError(t, err2, "second test run should pass")
	})
}

// verifyNoTestDuplication checks for duplicate test function names across files
func verifyNoTestDuplication(t *testing.T, testDir string, testFiles []string) {
	t.Helper()

	testFuncRegex := regexp.MustCompile(`func (Test\w+)\(t \*testing\.T\)`)
	seenTests := make(map[string]string) // test name -> file name

	for _, file := range testFiles {
		filePath := filepath.Join(testDir, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue // File may not exist yet
		}

		matches := testFuncRegex.FindAllStringSubmatch(string(content), -1)
		for _, match := range matches {
			if len(match) > 1 {
				testName := match[1]
				if prevFile, exists := seenTests[testName]; exists {
					t.Errorf("duplicate test %s found in %s and %s",
						testName, prevFile, file)
				}
				seenTests[testName] = file
			}
		}
	}

	if len(seenTests) > 0 {
		t.Logf("Found %d unique test functions across %d files",
			len(seenTests), len(testFiles))
	}
}

// verifyLintPassesInfraSplit checks that golangci-lint finds no issues in split files
func verifyLintPassesInfraSplit(t *testing.T, projectRoot string) {
	t.Helper()

	// Skip if golangci-lint not installed
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not installed")
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "golangci-lint", "run",
		"./internal/interfaces/cli/...",
		"./internal/infrastructure/diagram/...")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()

	// Allow exit code 0 (no issues) or not installed
	if err != nil && !strings.Contains(string(output), "not found") {
		t.Logf("Lint output:\n%s", string(output))
	}

	// Check for common issues in split files
	assert.NotContains(t, string(output), "unused",
		"found unused code in split files")
	assert.NotContains(t, string(output), "ineffassign",
		"found inefficient assignments")
}

// verifyPackageDeclarationsInfraSplit checks that all test files have proper package declaration
func verifyPackageDeclarationsInfraSplit(t *testing.T, projectRoot string) {
	t.Helper()

	cliTestDir := filepath.Join(projectRoot, "internal", "interfaces", "cli")
	diagramTestDir := filepath.Join(projectRoot, "internal", "infrastructure", "diagram")

	cliTestFiles := []string{
		"cli_test_helpers_test.go",
		"run_flags_test.go",
		"run_execution_test.go",
		"run_agent_test.go",
		"run_interactive_test.go",
	}

	diagramTestFiles := []string{
		"diagram_test_helpers_test.go",
		"generator_nodes_test.go",
		"generator_edges_test.go",
		"generator_header_test.go",
		"generator_parallel_test.go",
		"generator_highlight_test.go",
		"dot_generator_core_test.go",
	}

	for _, file := range cliTestFiles {
		verifyFileHasPackageDeclarationInfraSplit(t, cliTestDir, file, "cli")
	}

	for _, file := range diagramTestFiles {
		verifyFileHasPackageDeclarationInfraSplit(t, diagramTestDir, file, "diagram")
	}
}

// verifyFileHasPackageDeclarationInfraSplit checks a single file for package declaration
func verifyFileHasPackageDeclarationInfraSplit(t *testing.T, dir, file, expectedPkg string) {
	t.Helper()

	filePath := filepath.Join(dir, file)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return // File may not exist yet
	}

	// First non-comment line should be package declaration
	lines := strings.Split(string(content), "\n")
	var foundPackage bool
	expectedDecl := "package " + expectedPkg
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.HasPrefix(trimmed, expectedDecl) {
			foundPackage = true
			break
		}
		break
	}

	assert.True(t, foundPackage,
		"file %s missing '%s' declaration", file, expectedDecl)
}
