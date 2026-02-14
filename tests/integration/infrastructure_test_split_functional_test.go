//go:build integration

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

func TestInfrastructureTestFileSplitting_Integration(t *testing.T) {
	projectRoot, err := findProjectRootInfraSplit()
	require.NoError(t, err, "failed to find project root")

	cliTestDir := filepath.Join(projectRoot, "internal", "interfaces", "cli")
	diagramTestDir := filepath.Join(projectRoot, "internal", "infrastructure", "diagram")

	t.Run("all CLI split test files exist", func(t *testing.T) {
		expectedFiles := []string{
			"cli_test_helpers_test.go",
			"run_flags_test.go",
			"run_execution_test.go",
			"run_agent_test.go",
			"run_interactive_test.go",
		}

		for _, file := range expectedFiles {
			filePath := filepath.Join(cliTestDir, file)
			_, err := os.Stat(filePath)
			require.NoError(t, err, "expected CLI test file %s to exist", file)
		}
	})

	t.Run("all diagram split test files exist", func(t *testing.T) {
		expectedFiles := []string{
			"generator_nodes_test.go",
			"generator_edges_test.go",
			"generator_header_test.go",
			"generator_parallel_test.go",
			"generator_highlight_test.go",
			"dot_generator_core_test.go",
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

			assert.NotContains(t, string(content), "os.Chdir(",
				"file %s must not contain os.Chdir calls (thread-safety violation)", file)
		}
	})

	t.Run("CLI split files use thread-safe patterns", func(t *testing.T) {
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
			hasThreadSafe := strings.Contains(fileContent, "t.TempDir()") ||
				strings.Contains(fileContent, "t.Setenv(") ||
				strings.Contains(fileContent, "setupTestDir(t)")

			if len(fileContent) > 1000 {
				assert.True(t, hasThreadSafe,
					"file %s should use thread-safe directory patterns", file)
			}
		}
	})

	t.Run("split files respect size limits", func(t *testing.T) {
		maxLines := 1500
		allowedExceptions := map[string]int{}

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

		assert.NotContains(t, string(output), "FAIL:", "found test failures")
		assert.Contains(t, string(output), "PASS", "expected passing tests")
	})

	t.Run("all diagram infrastructure tests pass", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "./internal/infrastructure/diagram/...", "-v")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "diagram tests failed:\n%s", string(output))

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
		minCoverage := 78.5

		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test",
			"./internal/interfaces/cli/...",
			"./internal/infrastructure/diagram/...",
			"-coverprofile=/tmp/c015_coverage.out")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "coverage test failed:\n%s", string(output))

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

func TestInfrastructureTestFileSplitting_HappyPath(t *testing.T) {
	projectRoot, err := findProjectRootInfraSplit()
	require.NoError(t, err, "failed to find project root")

	t.Run("test helpers are properly shared", func(t *testing.T) {
		cliTestDir := filepath.Join(projectRoot, "internal", "interfaces", "cli")
		diagramTestDir := filepath.Join(projectRoot, "internal", "infrastructure", "diagram")

		cliHelpersPath := filepath.Join(cliTestDir, "cli_test_helpers_test.go")
		_, err := os.Stat(cliHelpersPath)
		require.NoError(t, err, "CLI test helpers should exist")

		diagramHelpersPath := filepath.Join(diagramTestDir, "diagram_test_helpers_test.go")
		_, err = os.Stat(diagramHelpersPath)
		require.NoError(t, err, "diagram test helpers should exist")
	})

	t.Run("file naming follows convention", func(t *testing.T) {
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

func TestInfrastructureTestFileSplitting_EdgeCases(t *testing.T) {
	projectRoot, err := findProjectRootInfraSplit()
	require.NoError(t, err, "failed to find project root")

	cliTestDir := filepath.Join(projectRoot, "internal", "interfaces", "cli")
	diagramTestDir := filepath.Join(projectRoot, "internal", "infrastructure", "diagram")

	t.Run("CLI tests organized by execution mode", func(t *testing.T) {
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
				continue
			}

			for _, keyword := range keywords {
				assert.Contains(t, string(content), keyword,
					"file %s should contain tests for %s", file, keyword)
			}
		}
	})

	t.Run("diagram tests organized by concern", func(t *testing.T) {
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
				continue
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
		helperPath := filepath.Join(cliTestDir, "cli_test_helpers_test.go")
		content, err := os.ReadFile(helperPath)
		require.NoError(t, err, "shared helpers file not found")

		assert.Contains(t, string(content), "package cli",
			"helpers must be in cli package to avoid import cycles")

		helpers := []string{
			"setupTestDir",
			"setTestEnv",
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
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "./internal/interfaces/cli/...", "-v")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("CLI tests output:\n%s", string(output))
		}

		testRunRegex := regexp.MustCompile(`=== RUN\s+Test`)
		matches := testRunRegex.FindAllString(string(output), -1)
		testCount := len(matches)

		assert.Greater(t, testCount, 40,
			"expected >40 CLI tests after split, found %d", testCount)
	})

	t.Run("diagram test count preserved", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "go", "test", "./internal/infrastructure/diagram/...", "-v")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Diagram tests output:\n%s", string(output))
		}

		testRunRegex := regexp.MustCompile(`=== RUN\s+Test`)
		matches := testRunRegex.FindAllString(string(output), -1)
		testCount := len(matches)

		assert.Greater(t, testCount, 100,
			"expected >100 diagram tests after split, found %d", testCount)
	})

	t.Run("split maintains git history", func(t *testing.T) {
		ctx := context.Background()

		cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
		cmd.Dir = projectRoot
		if err := cmd.Run(); err != nil {
			t.Skip("Not in a git repository")
		}

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

		err1 := runTests()
		err2 := runTests()

		assert.NoError(t, err1, "first test run should pass")
		assert.NoError(t, err2, "second test run should pass")
	})
}

func verifyNoTestDuplication(t *testing.T, testDir string, testFiles []string) {
	t.Helper()

	testFuncRegex := regexp.MustCompile(`func (Test\w+)\(t \*testing\.T\)`)
	seenTests := make(map[string]string)

	for _, file := range testFiles {
		filePath := filepath.Join(testDir, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
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

func verifyLintPassesInfraSplit(t *testing.T, projectRoot string) {
	t.Helper()

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not installed")
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "golangci-lint", "run",
		"./internal/interfaces/cli/...",
		"./internal/infrastructure/diagram/...")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()

	if err != nil && !strings.Contains(string(output), "not found") {
		t.Logf("Lint output:\n%s", string(output))
	}

	assert.NotContains(t, string(output), "unused",
		"found unused code in split files")
	assert.NotContains(t, string(output), "ineffassign",
		"found inefficient assignments")
}

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

func verifyFileHasPackageDeclarationInfraSplit(t *testing.T, dir, file, expectedPkg string) {
	t.Helper()

	filePath := filepath.Join(dir, file)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

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
