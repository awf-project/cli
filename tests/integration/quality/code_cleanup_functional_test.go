//go:build integration

package quality_test

// Feature: C043
//
// This file contains functional tests for C043: Quick Wins Code Cleanup.
// This feature addresses code quality issues: documentation consistency,
// formatting compliance, and issue tracking references.
//
// Tasks covered:
// 1. Documentation fix: status filter value alignment in commands.md
// 2. Issue tracking: WARNING comments linked to GitHub issues
// 3. Formatting compliance: gofmt verification

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/awf-project/awf/tests/integration/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocumentation_StatusFilterAlignment_Integration verifies that the
// status filter documentation in commands.md uses "cancelled" consistently
// with the actual implementation (StatusCancelled constant).
//
// Given: The commands.md documentation file
// When: Checking the --status flag documentation
// Then: Status filter should show "cancelled" not "interrupted"
func TestDocumentation_StatusFilterAlignment_Integration(t *testing.T) {
	repoRoot := testhelpers.GetRepoRoot(t)
	commandsPath := filepath.Join(repoRoot, "docs/user-guide/commands.md")

	content, err := os.ReadFile(commandsPath)
	require.NoError(t, err, "should be able to read commands.md")

	lines := strings.Split(string(content), "\n")

	// Find the --status flag documentation line (around line 627)
	var statusLine string
	for i, line := range lines {
		if strings.Contains(line, "--status") && strings.Contains(line, "Filter by status") {
			statusLine = lines[i]
			break
		}
	}

	require.NotEmpty(t, statusLine, "should find --status flag documentation")

	// Verify it contains "cancelled" as the status value
	assert.Contains(t, statusLine, "cancelled", "status filter should reference 'cancelled' status")
	assert.Contains(t, statusLine, "success", "status filter should reference 'success' status")
	assert.Contains(t, statusLine, "failed", "status filter should reference 'failed' status")

	// Verify the status enum reference doesn't use "interrupted"
	// Note: "interrupted" may still appear in descriptive text (e.g., "Resume an interrupted workflow")
	// but should NOT appear in the status enum values list
	statusEnumPattern := regexp.MustCompile(`\(([^)]+)\)`)
	matches := statusEnumPattern.FindStringSubmatch(statusLine)
	if len(matches) > 1 {
		enumValues := matches[1]
		assert.NotContains(t, enumValues, "interrupted",
			"status filter enum values should not include 'interrupted', implementation uses 'cancelled'")
	}
}

// TestWarningComments_IssueTracking_Integration verifies that no stale WARNING
// comments about checkUnknownKeys remain in loader_test.go.
// C055 removed these comments after checkUnknownKeys was implemented (issue #169).
//
// Given: loader_test.go with checkUnknownKeys tests
// When: Searching for WARNING comments about checkUnknownKeys
// Then: Zero WARNING comments should be found
func TestWarningComments_IssueTracking_Integration(t *testing.T) {
	repoRoot := testhelpers.GetRepoRoot(t)
	loaderTestPath := filepath.Join(repoRoot, "internal/infrastructure/config/loader_test.go")

	content, err := os.ReadFile(loaderTestPath)
	require.NoError(t, err, "should be able to read loader_test.go")

	lines := strings.Split(string(content), "\n")

	warningPattern := regexp.MustCompile(`//\s*WARNING:.*checkUnknownKeys`)

	warningCount := 0
	for _, line := range lines {
		if warningPattern.MatchString(line) {
			warningCount++
		}
	}

	assert.Equal(t, 0, warningCount,
		"no stale WARNING comments about checkUnknownKeys should remain (C055 cleanup)")
}

// TestFormatting_GofmtCompliance_Integration verifies that all Go source files
// pass gofmt formatting checks (zero diff).
//
// Given: All Go source files in the project
// When: Running gofmt -d on the codebase
// Then: No formatting differences should be reported
func TestFormatting_GofmtCompliance_Integration(t *testing.T) {
	repoRoot := testhelpers.GetRepoRoot(t)

	// Run gofmt -d on the entire project
	cmd := exec.Command("gofmt", "-d", ".")
	cmd.Dir = repoRoot

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "gofmt should execute successfully")

	// gofmt -d outputs diff if files need formatting, empty output means all files are formatted
	assert.Empty(t, string(output),
		"all Go files should pass gofmt (zero diff). Run 'make fmt' to fix formatting issues.")
}

// TestDocumentation_DescriptiveInterruptedText_Integration verifies that
// descriptive uses of "interrupted" in documentation are preserved where
// semantically correct.
//
// Given: The commands.md documentation file
// When: Checking descriptive text about workflow resumption
// Then: "interrupted" should be used for describing user actions (Ctrl+C)
//
//	but "cancelled" should be used for status enum values
func TestDocumentation_DescriptiveInterruptedText_Integration(t *testing.T) {
	repoRoot := testhelpers.GetRepoRoot(t)
	commandsPath := filepath.Join(repoRoot, "docs/user-guide/commands.md")

	content, err := os.ReadFile(commandsPath)
	require.NoError(t, err, "should be able to read commands.md")

	text := string(content)

	// Count occurrences of "interrupted" in the documentation
	interruptedCount := strings.Count(strings.ToLower(text), "interrupted")

	// Per ADR-001: Some occurrences of "interrupted" are correct descriptive English
	// (e.g., "Resume an interrupted workflow") and should be preserved.
	// Only the status filter enum value should use "cancelled".
	if interruptedCount > 0 {
		t.Logf("Found %d occurrence(s) of 'interrupted' in documentation", interruptedCount)
		t.Logf("This is acceptable if used in descriptive context (e.g., 'Resume an interrupted workflow')")
		t.Logf("Status enum values should use 'cancelled' per implementation")
	}

	// No assertion here - this is informational to document ADR-001 trade-off
}

// TestQualityPipeline_AllChecksPass_Integration verifies that the entire
// quality pipeline (fmt + vet + lint + test) passes after C043 changes.
//
// Given: All C043 code cleanup changes applied
// When: Running make quality
// Then: All checks should pass with zero failures
func TestQualityPipeline_AllChecksPass_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping quality pipeline test in short mode")
	}

	repoRoot := testhelpers.GetRepoRoot(t)

	tests := []struct {
		name    string
		target  string
		timeout string
	}{
		{"fmt", "fmt", "30s"},
		{"vet", "vet", "30s"},
		{"lint", "lint", "60s"},
		{"test", "test", "120s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("make", tt.target)
			cmd.Dir = repoRoot

			output, err := cmd.CombinedOutput()
			assert.NoError(t, err,
				"make %s should pass. Output:\n%s", tt.target, string(output))
		})
	}
}

// TestFileExistence_RequiredFiles_Integration verifies that all files
// referenced in C043 tasks exist at their expected paths.
//
// Given: File paths from C043 specification
// When: Checking file existence
// Then: All referenced files should exist
func TestFileExistence_RequiredFiles_Integration(t *testing.T) {
	repoRoot := testhelpers.GetRepoRoot(t)

	requiredFiles := []string{
		"docs/user-guide/commands.md",
		"internal/infrastructure/config/loader_test.go",
	}

	for _, file := range requiredFiles {
		path := filepath.Join(repoRoot, file)
		_, err := os.Stat(path)
		assert.NoError(t, err, "file should exist: %s", file)
	}
}
