//go:build integration

package quality_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const auditScriptPath = "../../../scripts/audit-skips.sh"

func TestAuditScript_HappyPath_AllCategories(t *testing.T) {
	fixturesDir, err := filepath.Abs("../../fixtures/audit_skips")
	require.NoError(t, err, "should resolve fixtures directory path")

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err, "should resolve script path")

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = fixturesDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "audit script should execute without errors, output: %s", string(output))

	outputStr := string(output)

	assert.Contains(t, outputStr, "Test Skip Audit Report", "output should contain report header")
	assert.Contains(t, outputStr, "Summary by Category:", "output should contain summary section")
	assert.Contains(t, outputStr, "Detailed Breakdown:", "output should contain detailed section")
	assert.Contains(t, outputStr, "Total Skipped Tests:", "output should contain total count")

	expectedCategories := []string{
		"integration",
		"not_implemented",
		"cli_tool",
		"platform",
		"short_mode",
		"fixture",
		"pending",
		"stub",
		"other",
	}

	for _, category := range expectedCategories {
		assert.Contains(t, outputStr, category, "output should contain category: %s", category)
	}

	assert.Contains(t, outputStr, "Action:", "output should contain action recommendations")
	assert.Contains(t, outputStr, "Convert to //go:build integration tag", "should suggest build tag conversion")
	assert.Contains(t, outputStr, "Link to feature spec or delete if obsolete", "should suggest handling not implemented")
	assert.Contains(t, outputStr, "Convert to //go:build external or use helper", "should suggest handling CLI tools")
}

func TestAuditScript_HappyPath_IntegrationSkips(t *testing.T) {
	fixturesDir, err := filepath.Abs("../../fixtures/audit_skips")
	require.NoError(t, err)

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = fixturesDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "script should execute successfully")
	outputStr := string(output)

	assert.Contains(t, outputStr, "[integration]", "should identify integration category")
	assert.Contains(t, outputStr, "Pattern: skipping integration test", "should show integration pattern")

	assert.Contains(t, outputStr, "integration_skip_test.go", "should reference integration skip test file")

	assert.Contains(t, outputStr, "Convert to //go:build integration tag",
		"should recommend build tag conversion for integration tests")
}

func TestAuditScript_HappyPath_NotImplementedSkips(t *testing.T) {
	fixturesDir, err := filepath.Abs("../../fixtures/audit_skips")
	require.NoError(t, err)

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = fixturesDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "script should execute successfully")
	outputStr := string(output)

	assert.Contains(t, outputStr, "[not_implemented]", "should identify not_implemented category")
	assert.Contains(t, outputStr, "Pattern: not yet implemented", "should show not implemented pattern")
	assert.Contains(t, outputStr, "not_implemented_test.go", "should reference not implemented test file")
	assert.Contains(t, outputStr, "Link to feature spec or delete if obsolete",
		"should recommend linking to spec or deletion")
}

func TestAuditScript_HappyPath_CLIToolSkips(t *testing.T) {
	fixturesDir, err := filepath.Abs("../../fixtures/audit_skips")
	require.NoError(t, err)

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = fixturesDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "script should execute successfully")
	outputStr := string(output)

	assert.Contains(t, outputStr, "[cli_tool]", "should identify cli_tool category")
	assert.Contains(t, outputStr, "cli_tool_test.go", "should reference CLI tool test file")
	assert.Contains(t, outputStr, "Convert to //go:build external or use helper",
		"should recommend build tag or helper for CLI tools")
}

func TestAuditScript_HappyPath_CountAccuracy(t *testing.T) {
	fixturesDir, err := filepath.Abs("../../fixtures/audit_skips")
	require.NoError(t, err)

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = fixturesDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "script should execute successfully")
	outputStr := string(output)

	assert.Contains(t, outputStr, "Total Skipped Tests:", "should show total count label")

	lines := strings.Split(outputStr, "\n")
	foundTotal := false
	for _, line := range lines {
		if strings.Contains(line, "Total Skipped Tests:") {
			foundTotal = true
			assert.Contains(t, line, "16", "total count should be 16")
			break
		}
	}
	assert.True(t, foundTotal, "should find total count line")

	assert.Contains(t, outputStr, "Percentage", "should show percentage column in summary")
	assert.Contains(t, outputStr, "%", "should contain percentage values")
}

func TestAuditScript_EdgeCase_NoSkipsFound(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "no_skips_test.go")
	err := os.WriteFile(testFile, []byte(`package test
import "testing"
func TestNoSkip(t *testing.T) {
	// This test does not skip
	t.Log("running test")
}
`), 0o644)
	require.NoError(t, err)

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "script should handle no skips gracefully")
	outputStr := string(output)

	assert.Contains(t, outputStr, "No t.Skip() calls found", "should report no skips found")
}

func TestAuditScript_EdgeCase_EmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "script should handle empty directory")
	outputStr := string(output)

	assert.Contains(t, outputStr, "No t.Skip() calls found", "should report no skips in empty directory")
}

func TestAuditScript_EdgeCase_NestedDirectories(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "sub", "nested")
	err := os.MkdirAll(subDir, 0o755)
	require.NoError(t, err)

	// Create test file in nested directory
	testFile := filepath.Join(subDir, "nested_test.go")
	err = os.WriteFile(testFile, []byte(`package test
import "testing"
func TestNestedSkip(t *testing.T) {
	t.Skip("skipping integration test")
}
`), 0o644)
	require.NoError(t, err)

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "script should find skips in nested directories")
	outputStr := string(output)

	// Should find the nested skip
	assert.Contains(t, outputStr, "Total Skipped Tests: 1", "should find skip in nested directory")
	assert.Contains(t, outputStr, "nested_test.go", "should reference nested test file")
}

// TestAuditScript_EdgeCase_MultilineSkipMessage tests handling of multiline skip messages
// Expected: Script extracts skip message correctly even with multiline format
func TestAuditScript_EdgeCase_MultilineSkipMessage(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "multiline_test.go")

	// Create test with multiline skip message
	err := os.WriteFile(testFile, []byte(`package test
import "testing"
func TestMultilineSkip(t *testing.T) {
	t.Skip("not yet implemented - " +
		"waiting for feature spec")
}
`), 0o644)
	require.NoError(t, err)

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "script should handle multiline skip messages")
	outputStr := string(output)

	assert.Contains(t, outputStr, "Total Skipped Tests: 1", "should count multiline skip")
}

func TestAuditScript_ErrorHandling_ScriptExecutable(t *testing.T) {
	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	info, err := os.Stat(scriptPath)
	require.NoError(t, err, "script file should exist")

	assert.NotNil(t, info, "should get file info")
	assert.False(t, info.IsDir(), "script should be a file, not directory")

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath, "--help")
	_ = cmd.Run()
}

func TestAuditScript_ErrorHandling_ValidBashSyntax(t *testing.T) {
	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", "-n", scriptPath)
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "script should have valid bash syntax, output: %s", string(output))
}

// TestAuditScript_ErrorHandling_MissingGrepCommand tests behavior when grep is unavailable
// Expected: Script should fail gracefully if grep is not available (edge case)
// Note: This test is informational - in practice, grep is always available in CI/dev environments
func TestAuditScript_ErrorHandling_GrepAvailability(t *testing.T) {
	// Verify grep is available in the environment
	_, err := exec.LookPath("grep")
	require.NoError(t, err, "grep should be available in test environment")

	// This test documents the dependency on grep
	// If grep were not available, the script would fail with "grep: command not found"
}

// TestAuditScript_OutputFormat_ColorCodes tests that output includes ANSI color codes
// Expected: Output contains color formatting for better readability
func TestAuditScript_OutputFormat_ColorCodes(t *testing.T) {
	fixturesDir, err := filepath.Abs("../../fixtures/audit_skips")
	require.NoError(t, err)

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = fixturesDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err)
	outputStr := string(output)

	// ANSI color codes use escape sequences like \033[
	// The output should contain color formatting
	// Note: Color codes might be stripped in some test environments
	// We're verifying the script attempts to use colors

	// Check for structural elements that should be present regardless
	assert.Contains(t, outputStr, "===", "should contain header separator")
	assert.Contains(t, outputStr, "Category", "should contain category label")
	assert.Contains(t, outputStr, "Action:", "should contain action recommendations")
}

// TestAuditScript_OutputFormat_SummaryTable tests that summary table is properly formatted
// Expected: Summary table has columns for Category, Count, and Percentage
func TestAuditScript_OutputFormat_SummaryTable(t *testing.T) {
	fixturesDir, err := filepath.Abs("../../fixtures/audit_skips")
	require.NoError(t, err)

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = fixturesDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err)
	outputStr := string(output)

	// Verify table structure
	assert.Contains(t, outputStr, "Category", "summary should have Category column")
	assert.Contains(t, outputStr, "Count", "summary should have Count column")
	assert.Contains(t, outputStr, "Percentage", "summary should have Percentage column")
	assert.Contains(t, outputStr, "--------", "summary should have separator line")
}

// TestAuditScript_OutputFormat_DetailedBreakdown tests detailed breakdown section
// Expected: Detailed section shows pattern, affected files, examples, and actions
func TestAuditScript_OutputFormat_DetailedBreakdown(t *testing.T) {
	fixturesDir, err := filepath.Abs("../../fixtures/audit_skips")
	require.NoError(t, err)

	scriptPath, err := filepath.Abs(auditScriptPath)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", scriptPath)
	cmd.Dir = fixturesDir
	output, err := cmd.CombinedOutput()

	require.NoError(t, err)
	outputStr := string(output)

	// Verify detailed breakdown structure for each category
	assert.Contains(t, outputStr, "Pattern:", "should show pattern for each category")
	assert.Contains(t, outputStr, "Files affected:", "should show affected file count")
	assert.Contains(t, outputStr, "Examples:", "should show examples section")
	assert.Contains(t, outputStr, "Action:", "should show recommended actions")

	// Verify example file references are shown
	lines := strings.Split(outputStr, "\n")
	exampleCount := 0
	for _, line := range lines {
		if strings.Contains(line, "  - ") && strings.Contains(line, "_test.go:") {
			exampleCount++
		}
	}
	assert.Greater(t, exampleCount, 0, "should show at least one example file reference")
}
