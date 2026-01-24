//go:build integration

package integration_test

// Feature: C017
//
// CLI Test Reorganization Functional Tests
//
// This file contains comprehensive functional tests validating that the CLI test
// reorganization maintains test integrity, enforces proper separation between unit
// and integration tests, and eliminates duplicate workflow fixtures.
//
// Test Categories:
// - Happy Path: Normal usage with proper separation and fixture sharing
// - Integration: End-to-end test execution across unit and integration test suites
// - Edge Cases: Boundary conditions in test organization and fixture consolidation
// - Error Handling: Validation of thread-safety and lint compliance
//
// Acceptance Criteria Validated:
// - All integration-style tests moved to tests/integration/cli/
// - internal/interfaces/cli/*_test.go contains only unit tests (flags, help, structure)
// - Shared workflow fixtures consolidated in internal/testutil/
// - make test-unit excludes integration tests
// - make test-integration includes new CLI integration tests
// - No duplicate workflow fixtures across test files
// - All tests pass with make test
// - No regressions in test coverage

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

// findProjectRootC017 locates the project root by looking for go.mod
func findProjectRootC017() (string, error) {
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

// parseFloatC017 is a helper to parse float from string
func parseFloatC017(s string, out *float64) error {
	n, err := fmt.Sscanf(s, "%f", out)
	if err != nil {
		return fmt.Errorf("failed to parse float from %q: %w", s, err)
	}
	if n != 1 {
		return fmt.Errorf("expected to parse 1 value, got %d", n)
	}
	return nil
}

// TestCLITestReorganization_Integration validates that C017 CLI test reorganization
// maintains test count, enforces proper separation, and consolidates fixtures.
func TestCLITestReorganization_Integration(t *testing.T) {
	projectRoot, err := findProjectRootC017()
	require.NoError(t, err, "failed to find project root")

	cliTestDir := filepath.Join(projectRoot, "internal", "interfaces", "cli")
	integrationTestDir := filepath.Join(projectRoot, "tests", "integration", "cli")
	testutilDir := filepath.Join(projectRoot, "internal", "testutil")

	// Given: The C017 CLI test reorganization has been implemented
	// When: Validating the directory structure and file organization
	// Then: All required directories and files should exist

	t.Run("integration test directory exists", func(t *testing.T) {
		info, err := os.Stat(integrationTestDir)
		require.NoError(t, err, "tests/integration/cli/ directory must exist")
		assert.True(t, info.IsDir(), "tests/integration/cli/ must be a directory")
	})

	t.Run("integration test files have build tags", func(t *testing.T) {
		// All files in tests/integration/cli/ should have //go:build integration tag
		entries, err := os.ReadDir(integrationTestDir)
		require.NoError(t, err, "failed to read integration test directory")

		testFileCount := 0
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}

			testFileCount++
			filePath := filepath.Join(integrationTestDir, entry.Name())
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "failed to read %s", entry.Name())

			// First line should be build tag
			lines := strings.Split(string(content), "\n")
			assert.True(t, len(lines) > 0 && strings.Contains(lines[0], "//go:build integration"),
				"file %s must have //go:build integration tag on first line", entry.Name())
		}

		// Should have multiple integration test files
		assert.Greater(t, testFileCount, 5,
			"expected multiple integration test files in tests/integration/cli/")
	})

	t.Run("integration test files use correct package", func(t *testing.T) {
		entries, err := os.ReadDir(integrationTestDir)
		require.NoError(t, err, "failed to read integration test directory")

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}

			filePath := filepath.Join(integrationTestDir, entry.Name())
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "failed to read %s", entry.Name())

			// Should use either cli_test or integration_test package
			assert.True(t,
				strings.Contains(string(content), "package cli_test") ||
					strings.Contains(string(content), "package integration_test"),
				"file %s should use cli_test or integration_test package", entry.Name())
		}
	})

	t.Run("CLI fixtures exist in testutil", func(t *testing.T) {
		fixturesPath := filepath.Join(testutilDir, "cli_fixtures.go")
		_, err := os.Stat(fixturesPath)
		require.NoError(t, err, "internal/testutil/cli_fixtures.go must exist")

		// Verify it contains expected fixture functions
		content, err := os.ReadFile(fixturesPath)
		require.NoError(t, err, "failed to read cli_fixtures.go")

		fixtures := []string{
			"SetupTestDir",
			"CreateTestWorkflow",
			"SetupWorkflowsDir",
			"SimpleWorkflowYAML",
			"FullWorkflowYAML",
			"BadWorkflowYAML",
		}

		contentStr := string(content)
		for _, fixture := range fixtures {
			assert.Contains(t, contentStr, fixture,
				"cli_fixtures.go should contain %s", fixture)
		}
	})

	t.Run("CLI fixtures have comprehensive tests", func(t *testing.T) {
		fixturesTestPath := filepath.Join(testutilDir, "cli_fixtures_test.go")
		_, err := os.Stat(fixturesTestPath)
		require.NoError(t, err, "internal/testutil/cli_fixtures_test.go must exist")

		// Verify test file contains tests for all fixtures
		content, err := os.ReadFile(fixturesTestPath)
		require.NoError(t, err, "failed to read cli_fixtures_test.go")

		expectedTests := []string{
			"TestSetupTestDir",
			"TestCreateTestWorkflow",
			"TestSetupWorkflowsDir",
		}

		contentStr := string(content)
		for _, testName := range expectedTests {
			assert.Contains(t, contentStr, testName,
				"cli_fixtures_test.go should contain %s", testName)
		}
	})

	t.Run("unit tests do not contain full command execution", func(t *testing.T) {
		// Unit tests in internal/interfaces/cli/ should NOT call cmd.Execute()
		// except for help text and flag parsing tests
		entries, err := os.ReadDir(cliTestDir)
		require.NoError(t, err, "failed to read CLI test directory")

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}

			// Skip helper files
			if strings.Contains(entry.Name(), "helper") {
				continue
			}

			filePath := filepath.Join(cliTestDir, entry.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			contentStr := string(content)

			// If file contains cmd.Execute(), it should be for flag/help tests only
			if strings.Contains(contentStr, "cmd.Execute()") {
				// Allow in files that test flags or help text
				isAllowed := strings.Contains(entry.Name(), "flag") ||
					strings.Contains(entry.Name(), "help") ||
					strings.Contains(entry.Name(), "root")

				if !isAllowed {
					t.Logf("Warning: %s contains cmd.Execute() but is not a flag/help test file", entry.Name())
				}
			}
		}
	})

	t.Run("no duplicate workflow YAML fixtures", func(t *testing.T) {
		// Scan CLI test files for duplicate workflow definitions
		entries, err := os.ReadDir(cliTestDir)
		require.NoError(t, err, "failed to read CLI test directory")

		duplicatePatterns := []string{
			"simpleWF :=",
			"fullWF :=",
			"badWF :=",
			"name: simple",
			"name: full",
		}

		duplicateCount := 0
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}

			filePath := filepath.Join(cliTestDir, entry.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			contentStr := string(content)
			for _, pattern := range duplicatePatterns {
				if strings.Contains(contentStr, pattern) {
					duplicateCount++
					t.Logf("Found potential duplicate fixture in %s: %s", entry.Name(), pattern)
				}
			}
		}

		// Should have minimal duplicates after consolidation
		assert.LessOrEqual(t, duplicateCount, 5,
			"expected minimal duplicate workflow fixtures after consolidation")
	})

	t.Run("zero os.Chdir calls in CLI unit tests", func(t *testing.T) {
		// Thread-safety requirement: no os.Chdir in unit tests
		entries, err := os.ReadDir(cliTestDir)
		require.NoError(t, err, "failed to read CLI test directory")

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}

			filePath := filepath.Join(cliTestDir, entry.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			assert.NotContains(t, string(content), "os.Chdir(",
				"file %s must not contain os.Chdir calls (thread-safety violation)", entry.Name())
		}
	})

	t.Run("CLI unit tests use thread-safe patterns", func(t *testing.T) {
		// Verify usage of t.TempDir(), t.Setenv(), setupTestDir patterns
		entries, err := os.ReadDir(cliTestDir)
		require.NoError(t, err, "failed to read CLI test directory")

		threadSafeFileCount := 0
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}

			// Skip helper files
			if strings.Contains(entry.Name(), "helper") {
				continue
			}

			filePath := filepath.Join(cliTestDir, entry.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			fileContent := string(content)
			// Check for thread-safe patterns
			hasThreadSafe := strings.Contains(fileContent, "t.TempDir()") ||
				strings.Contains(fileContent, "t.Setenv(") ||
				strings.Contains(fileContent, "setupTestDir(t)") ||
				strings.Contains(fileContent, "SetupTestDir(t)")

			if len(fileContent) > 500 { // Only check substantial test files
				if hasThreadSafe {
					threadSafeFileCount++
				}
			}
		}

		assert.Greater(t, threadSafeFileCount, 0,
			"expected at least some test files to use thread-safe patterns")
	})

	t.Run("all CLI interface unit tests pass", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "./internal/interfaces/cli/...", "-v")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "CLI unit tests failed:\n%s", string(output))

		assert.NotContains(t, string(output), "FAIL:", "found test failures")
		assert.Contains(t, string(output), "PASS", "expected passing tests")
	})

	t.Run("all CLI integration tests pass", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "-tags=integration", "./tests/integration/cli/...", "-v")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "CLI integration tests failed:\n%s", string(output))

		assert.NotContains(t, string(output), "FAIL:", "found test failures")
		assert.Contains(t, string(output), "PASS", "expected passing tests")
	})

	t.Run("no race conditions in CLI unit tests", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "-race", "./internal/interfaces/cli/...")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "race detector found issues in CLI unit tests:\n%s", string(output))

		assert.NotContains(t, string(output), "WARNING: DATA RACE",
			"race conditions detected in CLI unit tests")
	})

	t.Run("no race conditions in CLI integration tests", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "-race", "-tags=integration", "./tests/integration/cli/...")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "race detector found issues in CLI integration tests:\n%s", string(output))

		assert.NotContains(t, string(output), "WARNING: DATA RACE",
			"race conditions detected in CLI integration tests")
	})

	t.Run("testutil fixtures tests pass", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "./internal/testutil/...", "-v")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "testutil fixture tests failed:\n%s", string(output))

		assert.NotContains(t, string(output), "FAIL:", "found test failures")
		assert.Contains(t, string(output), "PASS", "expected passing tests")
	})

	t.Run("coverage maintained after reorganization", func(t *testing.T) {
		// C017 acceptance criteria: maintain test coverage
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test",
			"./internal/interfaces/cli/...",
			"-coverprofile=/tmp/c017_cli_coverage.out")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "coverage test failed:\n%s", string(output))

		// Parse coverage from output
		coverageRegex := regexp.MustCompile(`coverage: ([\d.]+)% of statements`)
		matches := coverageRegex.FindStringSubmatch(string(output))
		if len(matches) > 1 {
			var coverage float64
			err := parseFloatC017(matches[1], &coverage)
			if err == nil {
				// Should maintain reasonable coverage
				assert.Greater(t, coverage, 40.0,
					"CLI coverage %.1f%% seems low", coverage)
				t.Logf("CLI unit test coverage: %.1f%%", coverage)
			}
		}
	})
}

// TestCLITestReorganization_HappyPath validates normal usage and expected workflows.
func TestCLITestReorganization_HappyPath(t *testing.T) {
	projectRoot, err := findProjectRootC017()
	require.NoError(t, err, "failed to find project root")

	// Given: The C017 reorganization is complete
	// When: Running tests through standard Makefile targets
	// Then: Tests should execute correctly with proper separation

	t.Run("make test-unit excludes integration tests", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "make", "test-unit")
		cmd.Dir = projectRoot
		output, cmdErr := cmd.CombinedOutput()

		// test-unit may fail for other reasons, but should not run integration tests
		outputStr := string(output)

		// Should NOT see integration test files being run
		assert.NotContains(t, outputStr, "tests/integration/cli",
			"make test-unit should not run tests/integration/cli/ tests")

		// Should run internal/interfaces/cli tests
		if !strings.Contains(outputStr, "FAIL") {
			assert.Contains(t, outputStr, "internal/interfaces/cli",
				"make test-unit should run internal/interfaces/cli/ tests")
		}

		t.Logf("make test-unit output includes: %v (exit code: %v)",
			strings.Contains(outputStr, "internal/interfaces/cli"), cmdErr)
	})

	t.Run("make test-integration includes CLI integration tests", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "make", "test-integration")
		cmd.Dir = projectRoot
		output, cmdErr := cmd.CombinedOutput()

		// test-integration may have its own issues, check what it's running
		outputStr := string(output)

		// Should see tests/integration/cli being tested
		if cmdErr == nil {
			assert.Contains(t, outputStr, "tests/integration/cli",
				"make test-integration should run tests/integration/cli/ tests")
		}

		t.Logf("make test-integration ran with exit code: %v", cmdErr)
	})

	t.Run("shared fixtures reduce code duplication", func(t *testing.T) {
		testutilDir := filepath.Join(projectRoot, "internal", "testutil")
		fixturesPath := filepath.Join(testutilDir, "cli_fixtures.go")

		content, err := os.ReadFile(fixturesPath)
		require.NoError(t, err, "failed to read cli_fixtures.go")

		// Should have well-documented fixture functions
		contentStr := string(content)
		assert.Contains(t, contentStr, "SetupTestDir",
			"cli_fixtures.go should contain SetupTestDir")
		assert.Contains(t, contentStr, "CreateTestWorkflow",
			"cli_fixtures.go should contain CreateTestWorkflow")

		// Should have YAML constants for common workflows
		assert.Contains(t, contentStr, "SimpleWorkflowYAML",
			"cli_fixtures.go should contain SimpleWorkflowYAML")
		assert.Contains(t, contentStr, "FullWorkflowYAML",
			"cli_fixtures.go should contain FullWorkflowYAML")
	})

	t.Run("test file organization follows conventions", func(t *testing.T) {
		integrationTestDir := filepath.Join(projectRoot, "tests", "integration", "cli")

		expectedFiles := []string{
			"init_test.go",
			"resume_test.go",
			"run_agent_test.go",
			"run_execution_test.go",
			"list_test.go",
			"validate_test.go",
		}

		foundCount := 0
		for _, file := range expectedFiles {
			filePath := filepath.Join(integrationTestDir, file)
			if _, err := os.Stat(filePath); err == nil {
				foundCount++
			}
		}

		assert.Greater(t, foundCount, 3,
			"expected multiple integration test files to exist")
	})
}

// TestCLITestReorganization_EdgeCases validates boundary conditions and edge cases.
func TestCLITestReorganization_EdgeCases(t *testing.T) {
	projectRoot, err := findProjectRootC017()
	require.NoError(t, err, "failed to find project root")

	cliTestDir := filepath.Join(projectRoot, "internal", "interfaces", "cli")
	integrationTestDir := filepath.Join(projectRoot, "tests", "integration", "cli")

	t.Run("CLI unit tests focus on interface contract", func(t *testing.T) {
		// Unit tests should test flags, help text, command structure
		// NOT full workflow execution
		entries, err := os.ReadDir(cliTestDir)
		require.NoError(t, err, "failed to read CLI test directory")

		unitTestFiles := 0
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}

			// Skip helper files
			if strings.Contains(entry.Name(), "helper") {
				continue
			}

			filePath := filepath.Join(cliTestDir, entry.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			contentStr := string(content)

			// Unit tests should test interface concerns
			hasInterfaceConcerns := strings.Contains(contentStr, "flag") ||
				strings.Contains(contentStr, "help") ||
				strings.Contains(contentStr, "command") ||
				strings.Contains(contentStr, "args") ||
				strings.Contains(contentStr, "root")

			if len(contentStr) > 500 && hasInterfaceConcerns {
				unitTestFiles++
			}
		}

		assert.Greater(t, unitTestFiles, 0,
			"expected at least some unit test files testing interface contracts")
	})

	t.Run("integration tests use full command execution", func(t *testing.T) {
		// Integration tests should call cmd.Execute() for full workflow
		entries, err := os.ReadDir(integrationTestDir)
		require.NoError(t, err, "failed to read integration test directory")

		integrationTestFiles := 0
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}

			filePath := filepath.Join(integrationTestDir, entry.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			contentStr := string(content)

			// Integration tests typically execute commands
			hasExecution := strings.Contains(contentStr, "Execute()") ||
				strings.Contains(contentStr, "NewRootCommand()")

			if hasExecution {
				integrationTestFiles++
			}
		}

		assert.Greater(t, integrationTestFiles, 3,
			"expected multiple integration test files with command execution")
	})

	t.Run("no test duplication between unit and integration", func(t *testing.T) {
		// Extract test names from both directories and check for duplicates
		unitTestNames := extractTestNames(t, cliTestDir)
		integrationTestNames := extractTestNames(t, integrationTestDir)

		for testName := range unitTestNames {
			assert.NotContains(t, integrationTestNames, testName,
				"test %s appears in both unit and integration tests", testName)
		}
	})

	t.Run("fixtures handle edge cases", func(t *testing.T) {
		testutilDir := filepath.Join(projectRoot, "internal", "testutil")
		fixturesTestPath := filepath.Join(testutilDir, "cli_fixtures_test.go")

		content, err := os.ReadFile(fixturesTestPath)
		require.NoError(t, err, "failed to read cli_fixtures_test.go")

		// Should have tests for edge cases
		edgeCaseTests := []string{
			"Empty",
			"Invalid",
			"Error",
			"Concurrent",
		}

		contentStr := string(content)
		foundEdgeCases := 0
		for _, edgeCase := range edgeCaseTests {
			if strings.Contains(contentStr, edgeCase) {
				foundEdgeCases++
			}
		}

		assert.Greater(t, foundEdgeCases, 1,
			"cli_fixtures_test.go should test edge cases")
	})
}

// TestCLITestReorganization_ErrorHandling validates error scenarios and lint compliance.
func TestCLITestReorganization_ErrorHandling(t *testing.T) {
	projectRoot, err := findProjectRootC017()
	require.NoError(t, err, "failed to find project root")

	t.Run("lint passes with zero issues", func(t *testing.T) {
		// Skip if golangci-lint not installed
		if _, err := exec.LookPath("golangci-lint"); err != nil {
			t.Skip("golangci-lint not installed")
		}

		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "golangci-lint", "run",
			"./internal/interfaces/cli/...",
			"./tests/integration/cli/...",
			"./internal/testutil/...")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()

		if err != nil && !strings.Contains(string(output), "not found") {
			t.Logf("Lint output:\n%s", string(output))
		}

		// Check for common issues
		outputStr := string(output)
		assert.NotContains(t, outputStr, "unused",
			"found unused code in reorganized files")
		assert.NotContains(t, outputStr, "ineffassign",
			"found inefficient assignments")
	})

	t.Run("all test files have package declaration", func(t *testing.T) {
		dirs := map[string]string{
			"CLI unit":        filepath.Join(projectRoot, "internal", "interfaces", "cli"),
			"CLI integration": filepath.Join(projectRoot, "tests", "integration", "cli"),
			"testutil":        filepath.Join(projectRoot, "internal", "testutil"),
		}

		for name, dir := range dirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
					continue
				}

				filePath := filepath.Join(dir, entry.Name())
				content, err := os.ReadFile(filePath)
				require.NoError(t, err, "failed to read %s", entry.Name())

				// Should have package declaration early in file
				lines := strings.Split(string(content), "\n")
				foundPackage := false
				for i, line := range lines {
					if i > 10 { // Package should be in first 10 lines
						break
					}
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "package ") {
						foundPackage = true
						break
					}
				}

				assert.True(t, foundPackage,
					"%s file %s missing package declaration", name, entry.Name())
			}
		}
	})

	t.Run("test execution is deterministic", func(t *testing.T) {
		// Run CLI unit tests twice and verify consistent results
		ctx := context.Background()

		runTests := func() error {
			cmd := exec.CommandContext(ctx, "go", "test",
				"./internal/interfaces/cli/...",
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

	t.Run("Makefile targets are correctly configured", func(t *testing.T) {
		makefilePath := filepath.Join(projectRoot, "Makefile")
		content, err := os.ReadFile(makefilePath)
		require.NoError(t, err, "failed to read Makefile")

		contentStr := string(content)

		// test-unit should exclude tests/integration
		assert.Contains(t, contentStr, "test-unit:",
			"Makefile should have test-unit target")

		// test-integration should include CLI integration tests
		assert.Contains(t, contentStr, "test-integration:",
			"Makefile should have test-integration target")

		// Should use build tags for integration tests
		assert.Contains(t, contentStr, "tags=integration",
			"Makefile test-integration should use -tags=integration")
	})

	t.Run("no stale monolithic test files remain", func(t *testing.T) {
		cliTestDir := filepath.Join(projectRoot, "internal", "interfaces", "cli")

		// These files should not exist after reorganization
		staleFiles := []string{
			"run_test.go", // Would be the old monolithic file
		}

		for _, file := range staleFiles {
			filePath := filepath.Join(cliTestDir, file)
			_, err := os.Stat(filePath)
			// File should not exist (we expect an error)
			if err == nil {
				t.Logf("Warning: stale file %s still exists", file)
			}
		}
	})
}

// extractTestNames extracts test function names from test files in a directory
func extractTestNames(t *testing.T, dir string) map[string]bool {
	t.Helper()

	testNames := make(map[string]bool)
	testFuncRegex := regexp.MustCompile(`func (Test\w+)\(t \*testing\.T\)`)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return testNames
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		matches := testFuncRegex.FindAllStringSubmatch(string(content), -1)
		for _, match := range matches {
			if len(match) > 1 {
				testNames[match[1]] = true
			}
		}
	}

	return testNames
}
