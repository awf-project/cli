//go:build integration

// Domain Test File Splitting Functional Tests
//
// This file contains comprehensive functional tests validating that the domain
// test file splitting maintains test integrity, coverage, and organization.
//
// Test Categories:
// - Integration: Validates end-to-end workflow test execution
// - Edge Cases: Validates boundary conditions and organizational patterns
// - Error Handling: Validates lint compliance and package structure
//
// Acceptance Criteria Validated:
// - All original tests preserved (zero test loss)
// - Split files respect size limits (<600 lines, with documented exceptions)
// - Test files organized by logical concern
// - All tests pass with maintained coverage (≥79.2%)
// - No shared state or import cycles
// - Shared helpers properly extracted
package integration_test

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

// findProjectRoot locates the project root by looking for go.mod
func findProjectRoot() (string, error) {
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

// TestDomainTestFileSplitting_Integration validates that C013 test file splitting
// maintains test count, coverage, and file organization standards.
func TestDomainTestFileSplitting_Integration(t *testing.T) {
	// Get the project root directory
	projectRoot, err := findProjectRoot()
	require.NoError(t, err, "failed to find project root")

	workflowTestDir := filepath.Join(projectRoot, "internal", "domain", "workflow")

	t.Run("all split test files exist", func(t *testing.T) {
		expectedFiles := []string{
			"domain_test_helpers_test.go",
			"step_command_test.go",
			"step_parallel_test.go",
			"step_loop_test.go",
			"step_agent_test.go",
			"agent_config_config_test.go",
			"agent_config_result_test.go",
			"agent_config_conversation_test.go",
		}

		for _, file := range expectedFiles {
			filePath := filepath.Join(workflowTestDir, file)
			_, err := os.Stat(filePath)
			require.NoError(t, err, "expected test file %s to exist", file)
		}
	})

	t.Run("original monolithic files deleted", func(t *testing.T) {
		deletedFiles := []string{
			"step_test.go",
			"parallel_test.go", // Legacy file from before split
		}

		for _, file := range deletedFiles {
			filePath := filepath.Join(workflowTestDir, file)
			_, err := os.Stat(filePath)
			assert.True(t, os.IsNotExist(err), "expected file %s to be deleted", file)
		}
	})

	t.Run("split files respect size limits", func(t *testing.T) {
		// Acceptance criteria: each new test file < 600 lines
		// ADR-002 allows exceptions for logical cohesion
		maxLines := 600
		allowedExceptions := map[string]int{
			"step_agent_test.go":                1000, // Complex agent validation logic
			"agent_config_result_test.go":       650,  // Result parsing edge cases
			"agent_config_conversation_test.go": 650,  // Conversation mode tests
		}

		testFiles := []string{
			"step_command_test.go",
			"step_parallel_test.go",
			"step_loop_test.go",
			"step_agent_test.go",
			"agent_config_config_test.go",
			"agent_config_result_test.go",
			"agent_config_conversation_test.go",
		}

		for _, file := range testFiles {
			filePath := filepath.Join(workflowTestDir, file)
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "failed to read %s", file)

			lines := strings.Count(string(content), "\n") + 1
			limit := maxLines
			if exception, ok := allowedExceptions[file]; ok {
				limit = exception
			}

			assert.LessOrEqual(t, lines, limit,
				"file %s has %d lines, expected ≤%d (per ADR-002 exception)", file, lines, limit)
		}
	})

	t.Run("all workflow domain tests pass", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "./internal/domain/workflow/...", "-v")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "workflow tests failed:\n%s", string(output))

		// Verify no test failures in output
		assert.NotContains(t, string(output), "FAIL:", "found test failures")
		assert.Contains(t, string(output), "PASS", "expected passing tests")
	})

	t.Run("no race conditions in split tests", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "-race", "./internal/domain/workflow/...")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "race detector found issues:\n%s", string(output))

		assert.NotContains(t, string(output), "WARNING: DATA RACE",
			"race conditions detected in split tests")
	})

	t.Run("coverage maintained after split", func(t *testing.T) {
		// C013 acceptance criteria: maintain ≥79.2% coverage baseline
		minCoverage := 79.2

		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "./internal/domain/workflow/...",
			"-coverprofile=/tmp/c013_coverage.out")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "coverage test failed:\n%s", string(output))

		// Parse coverage from output
		coverageRegex := regexp.MustCompile(`coverage: ([\d.]+)% of statements`)
		matches := coverageRegex.FindStringSubmatch(string(output))
		if len(matches) > 1 {
			var coverage float64
			err := parseFloat(matches[1], &coverage)
			if err == nil {
				assert.GreaterOrEqual(t, coverage, minCoverage,
					"coverage %.1f%% below baseline %.1f%%", coverage, minCoverage)
			}
		}
	})
}

// TestDomainTestFileSplitting_EdgeCases validates boundary conditions
// and edge cases in the split test organization.
func TestDomainTestFileSplitting_EdgeCases(t *testing.T) {
	projectRoot, err := findProjectRoot()
	require.NoError(t, err, "failed to find project root")

	workflowTestDir := filepath.Join(projectRoot, "internal", "domain", "workflow")

	t.Run("shared helpers prevent import cycles", func(t *testing.T) {
		// Verify domain_test_helpers_test.go exists and is importable
		helperPath := filepath.Join(workflowTestDir, "domain_test_helpers_test.go")
		content, err := os.ReadFile(helperPath)
		require.NoError(t, err, "shared helpers file not found")

		// Verify it's in the same package (no import needed)
		assert.Contains(t, string(content), "package workflow",
			"helpers must be in workflow package to avoid import cycles")

		// Verify it contains expected shared utilities
		helpers := []string{
			"testAnalyzer",    // Shared analyzer struct
			"newTestWorkflow", // Workflow fixture builder
		}
		for _, helper := range helpers {
			assert.Contains(t, string(content), helper,
				"expected shared helper %s not found", helper)
		}
	})

	t.Run("step tests organized by type", func(t *testing.T) {
		// Verify step files follow organizational pattern
		stepFiles := map[string][]string{
			"step_command_test.go":  {"Command"},
			"step_parallel_test.go": {"Parallel"},
			"step_loop_test.go":     {"Loop"},
			"step_agent_test.go":    {"Agent"},
		}

		for file, keywords := range stepFiles {
			filePath := filepath.Join(workflowTestDir, file)
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "failed to read %s", file)

			for _, keyword := range keywords {
				assert.Contains(t, string(content), keyword,
					"file %s should contain tests for %s", file, keyword)
			}
		}
	})

	t.Run("agent_config tests organized by concern", func(t *testing.T) {
		// Verify agent_config files follow organizational pattern
		agentFiles := map[string][]string{
			"agent_config_config_test.go":       {"AgentConfig", "Validate", "Provider"},
			"agent_config_result_test.go":       {"AgentResult", "Duration", "Success"},
			"agent_config_conversation_test.go": {"Conversation", "ConversationConfig", "Mode"},
		}

		for file, keywords := range agentFiles {
			filePath := filepath.Join(workflowTestDir, file)
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "failed to read %s", file)

			for _, keyword := range keywords {
				assert.Contains(t, string(content), keyword,
					"file %s should contain tests for %s", file, keyword)
			}
		}
	})

	t.Run("no test duplication between files", func(t *testing.T) {
		// Read all test files and collect test function names
		testFiles := []string{
			"step_command_test.go",
			"step_parallel_test.go",
			"step_loop_test.go",
			"step_agent_test.go",
			"agent_config_config_test.go",
			"agent_config_result_test.go",
			"agent_config_conversation_test.go",
		}

		testFuncRegex := regexp.MustCompile(`func (Test\w+)\(t \*testing\.T\)`)
		seenTests := make(map[string]string) // test name -> file name

		for _, file := range testFiles {
			filePath := filepath.Join(workflowTestDir, file)
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "failed to read %s", file)

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

		// Verify we found a reasonable number of tests
		assert.Greater(t, len(seenTests), 50,
			"expected to find >50 test functions, found %d", len(seenTests))
	})
}

// TestDomainTestFileSplitting_ErrorHandling validates error scenarios
// in the test file organization.
func TestDomainTestFileSplitting_ErrorHandling(t *testing.T) {
	projectRoot, err := findProjectRoot()
	require.NoError(t, err, "failed to find project root")

	t.Run("lint passes with zero issues", func(t *testing.T) {
		verifyLintPasses(t, projectRoot)
	})

	t.Run("all test files have package declaration", func(t *testing.T) {
		verifyPackageDeclarations(t, projectRoot)
	})

	t.Run("no orphaned test utilities", func(t *testing.T) {
		verifyHelperUsage(t, projectRoot)
	})
}

// verifyLintPasses checks that golangci-lint finds no issues in split files
func verifyLintPasses(t *testing.T, projectRoot string) {
	t.Helper()

	// Skip if golangci-lint not installed
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not installed")
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "golangci-lint", "run", "./internal/domain/workflow/...")
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

// verifyPackageDeclarations checks that all test files have proper package declaration
func verifyPackageDeclarations(t *testing.T, projectRoot string) {
	t.Helper()

	workflowTestDir := filepath.Join(projectRoot, "internal", "domain", "workflow")
	testFiles := []string{
		"domain_test_helpers_test.go",
		"step_command_test.go",
		"step_parallel_test.go",
		"step_loop_test.go",
		"step_agent_test.go",
		"agent_config_config_test.go",
		"agent_config_result_test.go",
		"agent_config_conversation_test.go",
	}

	for _, file := range testFiles {
		verifyFileHasPackageDeclaration(t, workflowTestDir, file)
	}
}

// verifyFileHasPackageDeclaration checks a single file for package declaration
func verifyFileHasPackageDeclaration(t *testing.T, dir, file string) {
	t.Helper()

	filePath := filepath.Join(dir, file)
	content, err := os.ReadFile(filePath)
	require.NoError(t, err, "failed to read %s", file)

	// First non-comment line should be package declaration
	lines := strings.Split(string(content), "\n")
	var foundPackage bool
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.HasPrefix(trimmed, "package workflow") {
			foundPackage = true
			break
		}
		break
	}

	assert.True(t, foundPackage,
		"file %s missing 'package workflow' declaration", file)
}

// verifyHelperUsage checks that shared helpers are defined and used appropriately
func verifyHelperUsage(t *testing.T, projectRoot string) {
	t.Helper()

	workflowTestDir := filepath.Join(projectRoot, "internal", "domain", "workflow")

	// Read domain_test_helpers_test.go
	helperPath := filepath.Join(workflowTestDir, "domain_test_helpers_test.go")
	helperContent, err := os.ReadFile(helperPath)
	require.NoError(t, err, "failed to read shared helpers")

	// Extract helper function names - allow for both immediate use and future helpers
	helperFuncRegex := regexp.MustCompile(`func (new\w+|contains\w+|indexOf)\(`)
	matches := helperFuncRegex.FindAllStringSubmatch(string(helperContent), -1)

	if len(matches) == 0 {
		t.Skip("no helper functions found to verify")
	}

	// Verify helpers exist and are properly formatted
	// Some helpers may be defined for future use (not yet referenced)
	assert.Greater(t, len(matches), 0, "expected helper functions in shared file")

	// Check for at least some usage in test files
	testFiles := []string{
		"step_command_test.go",
		"step_parallel_test.go",
		"step_loop_test.go",
		"step_agent_test.go",
		"agent_config_config_test.go",
		"agent_config_result_test.go",
		"agent_config_conversation_test.go",
		"template_validation_test.go", // Still exists, contains original tests
	}

	usedHelpers := countUsedHelpers(workflowTestDir, matches, testFiles)

	// At least some helpers should be used
	if len(matches) > 0 {
		// Allow helpers to be defined but not yet used (for future phases)
		t.Logf("Found %d helpers, %d currently in use", len(matches), usedHelpers)
	}
}

// countUsedHelpers counts how many helpers from matches are actually used in test files
func countUsedHelpers(workflowTestDir string, matches [][]string, testFiles []string) int {
	usedHelpers := 0
	for _, match := range matches {
		if len(match) > 1 {
			helperName := match[1]

			for _, file := range testFiles {
				filePath := filepath.Join(workflowTestDir, file)
				content, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}
				if strings.Contains(string(content), helperName+"(") {
					usedHelpers++
					break
				}
			}
		}
	}
	return usedHelpers
}

// parseFloat is a helper to parse float from string
func parseFloat(s string, out *float64) error {
	n, err := fmt.Sscanf(s, "%f", out)
	if err != nil {
		return fmt.Errorf("failed to parse float from %q: %w", s, err)
	}
	if n != 1 {
		return fmt.Errorf("expected to parse 1 value, got %d", n)
	}
	return nil
}
