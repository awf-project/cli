//go:build integration

package cleanup_test

// Feature: C053
//
// Functional tests for C053: Clean Up Skipped Tests Across Codebase.
// This chore removes ~50 dead/stale test skips (commented-out t.Skip, dead recover blocks,
// dead conditional skips, untestable stubs, feature placeholders), implements nil-guard checks
// in HandleExecutionError/HandleNonZeroExit, and converts 1 unconditional skip to testing.Short().
//
// Tests validate:
// - Dead code removal is thorough (acceptance criteria scan)
// - Nil-guard implementation uses correct patterns
// - Short-mode skip pattern is correctly applied
// - Legitimate conditional skips are preserved

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeadCode_Removal_Integration verifies that all dead test code identified
// in C053 has been removed. Scans source files for patterns that should no longer exist.
func TestDeadCode_Removal_Integration(t *testing.T) {
	repoRoot := getRepoRoot(t)

	t.Run("no commented-out t.Skip in T009 tests", func(t *testing.T) {
		// AC1: Zero commented-out t.Skip() lines remain in execution_service_t009_test.go
		filePath := filepath.Join(repoRoot, "internal/application/execution_service_t009_test.go")
		content := readFileContent(t, filePath)

		commentedSkipPattern := regexp.MustCompile(`//\s*t\.Skip\(`)
		matches := commentedSkipPattern.FindAllString(content, -1)

		assert.Empty(t, matches,
			"no commented-out t.Skip() should remain in execution_service_t009_test.go, found %d", len(matches))
	})

	t.Run("no dead recover blocks in graphviz tests", func(t *testing.T) {
		// AC2: Zero defer recover() blocks with "stub not implemented" in graphviz_test.go
		filePath := filepath.Join(repoRoot, "internal/infrastructure/diagram/graphviz_test.go")
		content := readFileContent(t, filePath)

		assert.NotContains(t, content, "stub not implemented",
			"graphviz_test.go should not contain dead 'stub not implemented' recover blocks")
		assert.NotContains(t, content, "recover()",
			"graphviz_test.go should not contain dead recover() calls")
	})

	t.Run("no dead conditional skips for existing directories", func(t *testing.T) {
		// AC3: Zero conditional skips for existing directories/fixtures remain

		// Migration tests: removed os.Stat + t.Skip blocks that checked directory existence
		// from within that same directory (condition always passed, skip never triggered).
		// Note: a helper function uses os.Stat + os.IsNotExist legitimately (returns 0, not skip).
		migrationPath := filepath.Join(repoRoot, "tests/integration/cli/migration_test.go")
		migrationContent := readFileContent(t, migrationPath)

		// The dead pattern was: os.Stat check followed by t.Skip within the same block
		deadSkipCount := countPatternOccurrences(migrationContent, `os\.IsNotExist.*\n.*t\.Skip`)
		assert.Equal(t, 0, deadSkipCount,
			"migration_test.go should not contain os.Stat + t.Skip directory existence guards")

		// Loop tests: existence checks for fixtures that exist
		loopPath := filepath.Join(repoRoot, "tests/integration/loop_test.go")
		loopContent := readFileContent(t, loopPath)

		loopFixtureSkipPattern := regexp.MustCompile(`os\.Stat\([^)]*loop-(foreach|while)\.yaml`)
		loopMatches := loopFixtureSkipPattern.FindAllString(loopContent, -1)
		assert.Empty(t, loopMatches,
			"loop_test.go should not contain dead os.Stat checks for loop fixtures, found %d", len(loopMatches))
	})

	t.Run("no untestable test stubs remain", func(t *testing.T) {
		// AC4: Verify specific stubs were deleted

		// T004a: foreach transition duplicate removed
		foreachPath := filepath.Join(repoRoot, "internal/application/loop_transitions_foreach_test.go")
		foreachContent := readFileContent(t, foreachPath)
		assert.NotContains(t, foreachContent, "TestExecuteForEach_TransitionSupport",
			"duplicate transition test should be removed")

		// T004b/c: intrabody stubs removed
		intrabodyPath := filepath.Join(repoRoot, "internal/application/loop_transitions_intrabody_test.go")
		intrabodyContent := readFileContent(t, intrabodyPath)
		assert.NotContains(t, intrabodyContent, "NilBodyStepIndices",
			"private function nil-body stub should be removed")
		assert.NotContains(t, intrabodyContent, "NegativeIndex",
			"private function negative-index stub should be removed")

		// T004d: signal handler crash test removed
		signalPath := filepath.Join(repoRoot, "internal/interfaces/cli/signal_handler_test.go")
		signalContent := readFileContent(t, signalPath)
		assert.NotContains(t, signalContent, "CallbackPanics",
			"crash test stub should be removed from signal handler tests")

		// T004e/f: testutil stubs removed
		testutilPath := filepath.Join(repoRoot, "internal/testutil/cli_fixtures_test.go")
		testutilContent := readFileContent(t, testutilPath)
		assert.NotContains(t, testutilContent, "NonExistentBaseDir",
			"non-existent base dir stub should be removed")
		assert.NotContains(t, testutilContent, "NilMap",
			"nil-map stub should be removed")
	})

	t.Run("no feature placeholder tests remain", func(t *testing.T) {
		// AC5: Verify placeholders were deleted

		// Domain error placeholders
		errorsPath := filepath.Join(repoRoot, "internal/domain/errors/errors_test.go")
		errorsContent := readFileContent(t, errorsPath)
		assert.NotContains(t, errorsContent, "ErrorCode type not yet implemented",
			"error code placeholder should be removed")
		assert.NotContains(t, errorsContent, "StructuredError type not yet implemented",
			"structured error placeholder should be removed")
		assert.NotContains(t, errorsContent, "Error catalog not yet implemented",
			"error catalog placeholder should be removed")

		// Conversation deferred tests
		conversationPath := filepath.Join(repoRoot, "tests/integration/conversation_test.go")
		conversationContent := readFileContent(t, conversationPath)
		assert.NotContains(t, conversationContent, "MaxTurnsZero",
			"max turns zero placeholder should be removed")
		assert.NotContains(t, conversationContent, "HighMaxTurns",
			"high max turns placeholder should be removed")

		// Memory management deferred test
		memoryPath := filepath.Join(repoRoot, "tests/integration/memory_management_functional_test.go")
		memoryContent := readFileContent(t, memoryPath)
		assert.NotContains(t, memoryContent, "ResumeWithPruning",
			"resume with pruning placeholder should be removed")
	})

	t.Run("json formatter panic skip guard removed", func(t *testing.T) {
		// T004g: panic skip guard and test case removed from json_formatter table-driven test
		jsonFmtPath := filepath.Join(repoRoot, "internal/infrastructure/errors/json_formatter_test.go")
		jsonFmtContent := readFileContent(t, jsonFmtPath)
		assert.NotContains(t, jsonFmtContent, "panics",
			"panic skip guard and test case should be removed from json_formatter_test.go")
	})
}

// TestShortModeSkip_Pattern_Integration verifies the retry test uses
// the proper testing.Short() conditional skip pattern instead of unconditional skip.
func TestShortModeSkip_Pattern_Integration(t *testing.T) {
	repoRoot := getRepoRoot(t)

	t.Run("retry test uses testing.Short pattern", func(t *testing.T) {
		// AC7: Slow retry test uses testing.Short() pattern
		filePath := filepath.Join(repoRoot, "internal/application/execution_service_retry_test.go")
		content := readFileContent(t, filePath)

		// Find the retry pattern test function
		require.Contains(t, content, "TestStepExecutorCallback_RetryPattern_ReturnsLoopNameAndNil",
			"retry pattern test should exist")

		// Extract the function body to verify the skip pattern
		funcBody := extractFunctionBody(t, content, "TestStepExecutorCallback_RetryPattern_ReturnsLoopNameAndNil")

		// Should contain testing.Short() conditional skip
		assert.Contains(t, funcBody, "testing.Short()",
			"retry test should use testing.Short() conditional skip")
		assert.Contains(t, funcBody, "t.Skip(",
			"retry test should call t.Skip inside the conditional")
	})
}

// TestCleanup_LegitimateSkips_Preserved_Integration verifies that legitimate
// conditional skips (testing.Short, root, CI, platform, CLI tools) are untouched.
func TestCleanup_LegitimateSkips_Preserved_Integration(t *testing.T) {
	repoRoot := getRepoRoot(t)

	t.Run("testing.Short skips preserved in codebase", func(t *testing.T) {
		// Legitimate testing.Short() skips should still exist across the codebase
		count := countPatternInDir(t, repoRoot, "internal", `testing\.Short\(\)`)
		assert.Greater(t, count, 0,
			"legitimate testing.Short() skips should be preserved")
	})

	t.Run("CI environment skips preserved", func(t *testing.T) {
		// skipInCI helper should still be used
		count := countPatternInDir(t, repoRoot, "tests/integration", `skipInCI\(t\)`)
		assert.Greater(t, count, 0,
			"legitimate CI environment skips should be preserved")
	})

	t.Run("CLI tool skips preserved", func(t *testing.T) {
		// skipIfCLIMissing should still be used
		count := countPatternInDir(t, repoRoot, "tests/integration", `skipIfCLIMissing\(t,`)
		assert.Greater(t, count, 0,
			"legitimate CLI tool availability skips should be preserved")
	})

	t.Run("platform skips preserved", func(t *testing.T) {
		// skipOnPlatform should still be defined
		helperPath := filepath.Join(repoRoot, "tests/integration/test_helpers_test.go")
		content := readFileContent(t, helperPath)
		assert.Contains(t, content, "func skipOnPlatform",
			"skipOnPlatform helper should still be defined")
	})
}

// TestCleanup_FileIntegrity_Integration verifies that modified files still contain
// expected structure after cleanup.
func TestCleanup_FileIntegrity_Integration(t *testing.T) {
	repoRoot := getRepoRoot(t)

	t.Run("graphviz test file has test functions", func(t *testing.T) {
		// After removing 19 recover blocks, tests should still exist
		filePath := filepath.Join(repoRoot, "internal/infrastructure/diagram/graphviz_test.go")
		content := readFileContent(t, filePath)

		funcPattern := regexp.MustCompile(`func Test\w+\(t \*testing\.T\)`)
		matches := funcPattern.FindAllString(content, -1)
		assert.GreaterOrEqual(t, len(matches), 5,
			"graphviz_test.go should still have at least 5 test functions after cleanup")
	})

	t.Run("T009 test file has test functions", func(t *testing.T) {
		// After removing 12 commented skips, tests should still exist
		filePath := filepath.Join(repoRoot, "internal/application/execution_service_t009_test.go")
		content := readFileContent(t, filePath)

		funcPattern := regexp.MustCompile(`func Test\w+\(t \*testing\.T\)`)
		matches := funcPattern.FindAllString(content, -1)
		assert.GreaterOrEqual(t, len(matches), 10,
			"execution_service_t009_test.go should still have at least 10 test functions after cleanup")
	})

	t.Run("handler test file has nil-guard tests", func(t *testing.T) {
		// After replacing stubs, proper nil-guard tests should exist
		filePath := filepath.Join(repoRoot, "internal/application/interactive_executor_handlers_test.go")
		content := readFileContent(t, filePath)

		assert.Contains(t, content, "TestHandleExecutionError_NilStep_ReturnsError",
			"nil-step test for HandleExecutionError should exist")
		assert.Contains(t, content, "TestHandleNonZeroExit_NilStep_ReturnsError",
			"nil-step test for HandleNonZeroExit should exist")
		assert.Contains(t, content, "TestHandleNonZeroExit_NilResult_ReturnsError",
			"nil-result test for HandleNonZeroExit should exist")
	})
}

// TestNilGuard_ErrorMessages_Integration verifies that nil-guard error messages
// are descriptive and actionable for debugging.
func TestNilGuard_ErrorMessages_Integration(t *testing.T) {
	repoRoot := getRepoRoot(t)

	t.Run("error messages include method name for traceability", func(t *testing.T) {
		// Verify the nil-guard implementation includes method names in errors
		filePath := filepath.Join(repoRoot, "internal/application/interactive_executor.go")
		content := readFileContent(t, filePath)

		// HandleExecutionError nil guard should mention its name
		assert.Contains(t, content, `HandleExecutionError: step cannot be nil`,
			"HandleExecutionError nil-guard should include method name")

		// HandleNonZeroExit nil guards should mention their name
		assert.Contains(t, content, `HandleNonZeroExit: step cannot be nil`,
			"HandleNonZeroExit step nil-guard should include method name")
		assert.Contains(t, content, `HandleNonZeroExit: result cannot be nil`,
			"HandleNonZeroExit result nil-guard should include method name")
	})

	t.Run("nil guards return errors not panics", func(t *testing.T) {
		// Verify the implementation uses fmt.Errorf (returns error) not panic()
		filePath := filepath.Join(repoRoot, "internal/application/interactive_executor.go")
		content := readFileContent(t, filePath)

		// Extract the HandleExecutionError and HandleNonZeroExit method bodies
		execErrBody := extractMethodBody(t, content, "HandleExecutionError")
		nonZeroBody := extractMethodBody(t, content, "HandleNonZeroExit")

		// Both should use fmt.Errorf for nil guards, not panic
		assert.Contains(t, execErrBody, "fmt.Errorf",
			"HandleExecutionError should use fmt.Errorf for nil guard")
		assert.NotContains(t, execErrBody, "panic(",
			"HandleExecutionError should not use panic for nil guard")

		assert.Contains(t, nonZeroBody, "fmt.Errorf",
			"HandleNonZeroExit should use fmt.Errorf for nil guard")
		assert.NotContains(t, nonZeroBody, "panic(",
			"HandleNonZeroExit should not use panic for nil guard")
	})

	t.Run("nil guard tests use require.Error assertions", func(t *testing.T) {
		// Verify test implementations follow the project's assertion pattern
		filePath := filepath.Join(repoRoot, "internal/application/interactive_executor_handlers_test.go")
		content := readFileContent(t, filePath)

		// All three nil-guard tests should use require.Error + assert.Contains
		nilGuardTests := []string{
			"TestHandleExecutionError_NilStep_ReturnsError",
			"TestHandleNonZeroExit_NilStep_ReturnsError",
			"TestHandleNonZeroExit_NilResult_ReturnsError",
		}

		for _, testName := range nilGuardTests {
			funcBody := extractFunctionBody(t, content, testName)
			assert.Contains(t, funcBody, "require.Error",
				"%s should use require.Error assertion", testName)
			assert.Contains(t, funcBody, "assert.Contains",
				"%s should use assert.Contains to verify error message", testName)
		}
	})
}

// TestCleanup_Metrics_Integration validates the overall cleanup metrics match expectations.
func TestCleanup_Metrics_Integration(t *testing.T) {
	repoRoot := getRepoRoot(t)

	t.Run("graphviz test file reduced after recover block removal", func(t *testing.T) {
		// After removing 19 recover blocks (~171 lines), the file should be smaller
		filePath := filepath.Join(repoRoot, "internal/infrastructure/diagram/graphviz_test.go")
		content := readFileContent(t, filePath)
		lineCount := strings.Count(content, "\n")

		assert.Less(t, lineCount, 350,
			"graphviz_test.go should be significantly smaller after removing 19 recover blocks")
	})

	t.Run("errors_test.go has no 'not yet implemented' placeholders", func(t *testing.T) {
		filePath := filepath.Join(repoRoot, "internal/domain/errors/errors_test.go")
		content := readFileContent(t, filePath)

		assert.NotContains(t, content, "not yet implemented",
			"no 'not yet implemented' placeholders should remain")
	})

	t.Run("all affected files exist", func(t *testing.T) {
		// C053 affects 15 files across 6 packages
		affectedFiles := []string{
			"internal/application/execution_service_t009_test.go",
			"internal/application/execution_service_retry_test.go",
			"internal/application/interactive_executor.go",
			"internal/application/interactive_executor_handlers_test.go",
			"internal/application/loop_transitions_foreach_test.go",
			"internal/application/loop_transitions_intrabody_test.go",
			"internal/domain/errors/errors_test.go",
			"internal/infrastructure/diagram/graphviz_test.go",
			"internal/infrastructure/errors/json_formatter_test.go",
			"internal/interfaces/cli/signal_handler_test.go",
			"internal/testutil/cli_fixtures_test.go",
			"tests/integration/cli/migration_test.go",
			"tests/integration/conversation_test.go",
			"tests/integration/loop_test.go",
			"tests/integration/memory_management_functional_test.go",
		}

		for _, relPath := range affectedFiles {
			absPath := filepath.Join(repoRoot, relPath)
			_, err := os.Stat(absPath)
			assert.NoError(t, err, "affected file should exist: %s", relPath)
		}
	})

	t.Run("changelog documents C053", func(t *testing.T) {
		filePath := filepath.Join(repoRoot, "CHANGELOG.md")
		content := readFileContent(t, filePath)
		assert.Contains(t, content, "C053",
			"CHANGELOG.md should document C053 cleanup work")
	})
}

// getRepoRoot returns the repository root directory by walking up from cwd.
func getRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err, "should get current directory")

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root (no go.mod found)")
		}
		dir = parent
	}
}

// readFileContent reads a file and returns its content as a string.
func readFileContent(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err, "should be able to read file: %s", path)
	return string(content)
}

// extractFunctionBody extracts the body of a Go test function from file content.
func extractFunctionBody(t *testing.T, content string, funcName string) string {
	t.Helper()

	pattern := regexp.MustCompile(`func ` + regexp.QuoteMeta(funcName) + `\(t \*testing\.T\) \{`)
	loc := pattern.FindStringIndex(content)
	require.NotNil(t, loc, "should find function %s", funcName)

	// Track brace depth to find the closing brace
	depth := 0
	start := loc[0]
	for i := loc[1] - 1; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[start : i+1]
			}
		}
	}

	t.Fatalf("could not find closing brace for function %s", funcName)
	return ""
}

// extractMethodBody extracts a method body by looking for a receiver method pattern.
func extractMethodBody(t *testing.T, content string, methodName string) string {
	t.Helper()

	pattern := regexp.MustCompile(`func \(e \*InteractiveExecutor\) ` + regexp.QuoteMeta(methodName) + `\(`)
	loc := pattern.FindStringIndex(content)
	require.NotNil(t, loc, "should find method %s", methodName)

	// Find the opening brace of the function body
	braceStart := strings.Index(content[loc[0]:], "{")
	require.NotEqual(t, -1, braceStart, "should find opening brace for %s", methodName)

	// Track brace depth
	depth := 0
	absStart := loc[0] + braceStart
	for i := absStart; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[absStart : i+1]
			}
		}
	}

	t.Fatalf("could not find closing brace for method %s", methodName)
	return ""
}

// countPatternInDir counts regex pattern matches across all .go files in a directory.
func countPatternInDir(t *testing.T, repoRoot, relDir, pattern string) int {
	t.Helper()

	dir := filepath.Join(repoRoot, relDir)
	re := regexp.MustCompile(pattern)
	count := 0

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if re.MatchString(scanner.Text()) {
				count++
			}
		}
		return nil
	})

	require.NoError(t, err, "should walk directory: %s", relDir)
	return count
}

// countPatternOccurrences counts multiline regex pattern matches in content.
func countPatternOccurrences(content, pattern string) int {
	re := regexp.MustCompile(`(?s)` + pattern)
	return len(re.FindAllString(content, -1))
}
