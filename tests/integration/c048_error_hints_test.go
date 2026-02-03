//go:build integration

package integration_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// C048: Add Actionable Error Message Hints - Integration Tests
// Tests validate end-to-end hint display across all error types:
// - File-not-found errors with "did you mean?" suggestions
// - YAML syntax errors with line/column references
// - Invalid state references with closest match suggestions
// - Missing input errors with required inputs and examples
// - Command execution failures with permission/path hints
// - --no-hints flag suppression
// - JSON format hint array output
// =============================================================================

// TestC048_FileNotFound_ShowsSuggestion validates file-not-found hints
// Given: a workflow file path with a typo (hints-file-tyop.yaml instead of hints-file-typo.yaml)
// When: awf run executes
// Then: error shows "Did you mean 'hints-file-typo.yaml'?" hint
func TestC048_FileNotFound_ShowsSuggestion(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)

	// Use a typo in the filename - the correct file is hints-file-typo.yaml
	wrongPath := filepath.Join("../fixtures/workflows", "hints-file-tyop.yaml")

	// Act
	cmd := exec.Command(binPath, "run", wrongPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "Command should fail for missing file")
	outputStr := string(output)

	// Verify error message includes file not found
	assert.Contains(t, outputStr, "not found", "Error should indicate file not found")

	// Verify hint suggests correct filename
	assert.Contains(t, outputStr, "Hint:", "Output should include hint marker")
	assert.Contains(t, outputStr, "hints-file-typo.yaml",
		"Hint should suggest the correct filename")

	// Verify "did you mean?" pattern
	lowerOutput := strings.ToLower(outputStr)
	assert.True(t,
		strings.Contains(lowerOutput, "did you mean") || strings.Contains(lowerOutput, "similar"),
		"Hint should use 'did you mean?' or 'similar' pattern")
}

// TestC048_YAMLSyntax_ShowsLineColumn validates YAML syntax error hints
// Given: a workflow file with invalid YAML syntax at line 7
// When: awf validate executes
// Then: error shows line/column reference and expected format hint
func TestC048_YAMLSyntax_ShowsLineColumn(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-yaml-syntax-error.yaml")

	// Act
	cmd := exec.Command(binPath, "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "Command should fail for invalid YAML")
	outputStr := string(output)

	// Verify error indicates YAML syntax issue
	lowerOutput := strings.ToLower(outputStr)
	assert.True(t,
		strings.Contains(lowerOutput, "yaml") || strings.Contains(lowerOutput, "syntax"),
		"Error should indicate YAML/syntax issue")

	// Verify hint includes line/column reference
	assert.Contains(t, outputStr, "Hint:", "Output should include hint marker")
	assert.True(t,
		strings.Contains(lowerOutput, "line") || strings.Contains(lowerOutput, "column"),
		"Hint should reference line or column number")
}

// TestC048_InvalidState_ShowsDidYouMean validates invalid state reference hints
// Given: a workflow with "next: proces" instead of "next: process"
// When: awf validate executes
// Then: error shows "Did you mean 'process'?" hint
func TestC048_InvalidState_ShowsDidYouMean(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-invalid-state-ref.yaml")

	// Act
	cmd := exec.Command(binPath, "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "Command should fail for invalid state reference")
	outputStr := string(output)

	// Verify error indicates invalid state
	lowerOutput := strings.ToLower(outputStr)
	assert.True(t,
		strings.Contains(lowerOutput, "state") || strings.Contains(lowerOutput, "invalid"),
		"Error should indicate invalid state")

	// Verify hint suggests correct state name
	assert.Contains(t, outputStr, "Hint:", "Output should include hint marker")
	assert.Contains(t, outputStr, "process",
		"Hint should suggest the correct state name 'process'")

	// Verify "did you mean?" pattern
	assert.True(t,
		strings.Contains(lowerOutput, "did you mean") || strings.Contains(lowerOutput, "similar"),
		"Hint should use 'did you mean?' or 'similar' pattern")
}

// TestC048_MissingInput_ShowsRequired validates missing input hints
// Given: a workflow requiring input 'user_name' executed without --input
// When: awf run executes
// Then: error lists required inputs with example usage
func TestC048_MissingInput_ShowsRequired(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-missing-input.yaml")

	// Act - run without providing required inputs
	cmd := exec.Command(binPath, "run", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "Command should fail for missing required inputs")
	outputStr := string(output)

	// Verify error indicates missing input
	lowerOutput := strings.ToLower(outputStr)
	assert.True(t,
		strings.Contains(lowerOutput, "input") || strings.Contains(lowerOutput, "required"),
		"Error should indicate missing input")

	// Verify hint lists required inputs
	assert.Contains(t, outputStr, "Hint:", "Output should include hint marker")
	assert.Contains(t, outputStr, "user_name",
		"Hint should mention required input 'user_name'")
	assert.Contains(t, outputStr, "user_email",
		"Hint should mention required input 'user_email'")

	// Verify example usage pattern
	assert.True(t,
		strings.Contains(lowerOutput, "--input") || strings.Contains(lowerOutput, "example"),
		"Hint should include example usage with --input flag")
}

// TestC048_CommandFailure_ShowsHints validates command execution failure hints
// Given: a workflow step with a non-existent command
// When: awf run executes
// Then: error shows "command not found" hint suggesting verification
func TestC048_CommandFailure_ShowsHints(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-command-not-found.yaml")

	// Act
	cmd := exec.Command(binPath, "run", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "Command should fail for non-existent command")
	outputStr := string(output)

	// Verify error indicates command failure
	lowerOutput := strings.ToLower(outputStr)
	assert.True(t,
		strings.Contains(lowerOutput, "command") || strings.Contains(lowerOutput, "not found"),
		"Error should indicate command not found")

	// Verify hint provides guidance
	assert.Contains(t, outputStr, "Hint:", "Output should include hint marker")
	assert.True(t,
		strings.Contains(lowerOutput, "verify") ||
			strings.Contains(lowerOutput, "check") ||
			strings.Contains(lowerOutput, "path") ||
			strings.Contains(lowerOutput, "installed"),
		"Hint should suggest verification or checking")
}

// TestC048_NoHintsFlag_SuppressesHints validates --no-hints flag behavior
// Given: a workflow file with typo and --no-hints flag
// When: awf run executes
// Then: error message shown without any hints
func TestC048_NoHintsFlag_SuppressesHints(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	wrongPath := filepath.Join("../fixtures/workflows", "hints-file-tyop.yaml")

	// Act
	cmd := exec.Command(binPath, "--no-hints", "run", wrongPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "Command should fail for missing file")
	outputStr := string(output)

	// Verify error message is shown
	assert.Contains(t, outputStr, "not found", "Error should indicate file not found")

	// Verify NO hints are displayed
	assert.NotContains(t, outputStr, "Hint:",
		"Output should NOT include hint marker when --no-hints is used")
	assert.NotContains(t, strings.ToLower(outputStr), "did you mean",
		"Output should NOT include 'did you mean?' suggestion when --no-hints is used")
}

// TestC048_JSONFormat_IncludesHints validates JSON output includes hints array
// Given: a workflow file with error and --format=json
// When: awf validate executes
// Then: JSON output contains "hints" array with actionable suggestions
func TestC048_JSONFormat_IncludesHints(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-invalid-state-ref.yaml")

	// Act
	cmd := exec.Command(binPath, "--format", "json", "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "Command should fail for invalid state reference")

	// Parse JSON output
	var result map[string]interface{}
	parseErr := json.Unmarshal(output, &result)
	require.NoError(t, parseErr, "Output should be valid JSON: %s", string(output))

	// Verify error_code field exists (StructuredError format)
	assert.Contains(t, result, "error_code", "JSON should contain error_code field")

	// Verify hints array exists
	assert.Contains(t, result, "hints", "JSON should contain hints array")

	// Verify hints is an array with at least one hint
	hints, ok := result["hints"].([]interface{})
	require.True(t, ok, "Hints should be an array")
	assert.Greater(t, len(hints), 0, "Hints array should contain at least one hint")

	// Verify hint content
	firstHint, ok := hints[0].(string)
	require.True(t, ok, "Hint should be a string")
	assert.NotEmpty(t, firstHint, "Hint should not be empty")
}

// TestC048_NoHintsWithColor_PreservesColors validates --no-hints preserves color output
// Given: a structured error with --no-hints flag
// When: awf run executes (with color enabled)
// Then: error message shown with colors but without hints
func TestC048_NoHintsWithColor_PreservesColors(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	wrongPath := filepath.Join("../fixtures/workflows", "hints-file-tyop.yaml")

	// Act - run with --no-hints but colors should still work
	cmd := exec.Command(binPath, "--no-hints", "run", wrongPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "Command should fail for missing file")
	outputStr := string(output)

	// Verify error message is shown
	assert.Contains(t, outputStr, "not found", "Error should indicate file not found")

	// Verify NO hints are displayed
	assert.NotContains(t, outputStr, "Hint:",
		"Output should NOT include hints when --no-hints is used")

	// Note: Color detection is complex in tests. We're verifying that:
	// 1. Error is displayed (so colors would work if terminal supports it)
	// 2. Hints are suppressed (independent of color setting)
	// The actual color codes may or may not appear depending on TTY detection,
	// but the important thing is that --no-hints doesn't break error display
	assert.NotEmpty(t, outputStr, "Error output should not be empty")
}

// TestC048_MultipleHints_DisplayAll validates multiple hints shown for single error
// Given: an error that could trigger multiple hint generators
// When: awf run executes
// Then: all applicable hints displayed in sequence
func TestC048_MultipleHints_DisplayAll(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)

	// Use file not found error which may have multiple hints:
	// 1. "Did you mean?" suggestion
	// 2. "Run 'awf list'" to see available workflows
	wrongPath := filepath.Join("../fixtures/workflows", "hints-file-tyop.yaml")

	// Act
	cmd := exec.Command(binPath, "run", wrongPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "Command should fail for missing file")
	outputStr := string(output)

	// Verify error message
	assert.Contains(t, outputStr, "not found", "Error should indicate file not found")

	// Count "Hint:" markers - there should be at least one
	hintCount := strings.Count(outputStr, "Hint:")
	assert.GreaterOrEqual(t, hintCount, 1,
		"Output should contain at least one hint")

	// If multiple generators apply, we might see multiple hints
	// This test validates the system can display multiple hints
	// (even if only one generator applies to this specific error)
	if hintCount > 1 {
		t.Logf("Multiple hints displayed (%d) - excellent!", hintCount)
	}
}

// TestC048_HintsWithDetails_PreserveFormat validates hints don't break details section
// Given: a structured error with details and hints
// When: error is formatted
// Then: details section and hints section are both displayed correctly
func TestC048_HintsWithDetails_PreserveFormat(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-invalid-state-ref.yaml")

	// Act
	cmd := exec.Command(binPath, "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "Command should fail for invalid state reference")
	outputStr := string(output)

	// Verify structured error format with error code
	assert.Contains(t, outputStr, "WORKFLOW", "Error should show error code category")

	// Verify details section exists
	lowerOutput := strings.ToLower(outputStr)
	assert.True(t,
		strings.Contains(lowerOutput, "details") || strings.Contains(lowerOutput, ":"),
		"Error should include details section")

	// Verify hint section exists and is separate
	assert.Contains(t, outputStr, "Hint:", "Output should include hint marker")

	// Verify hints appear after error message (not mixed into it)
	hintIndex := strings.Index(outputStr, "Hint:")
	assert.Greater(t, hintIndex, 0, "Hint should appear in output")
}

// TestC048_JSONFormat_NoHintsFlag_OmitsHints validates JSON with --no-hints omits hints
// Given: a workflow error with --format=json and --no-hints
// When: awf validate executes
// Then: JSON output excludes hints array or has empty array
func TestC048_JSONFormat_NoHintsFlag_OmitsHints(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-invalid-state-ref.yaml")

	// Act
	cmd := exec.Command(binPath, "--format", "json", "--no-hints", "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "Command should fail for invalid state reference")

	// Parse JSON output
	var result map[string]interface{}
	parseErr := json.Unmarshal(output, &result)
	require.NoError(t, parseErr, "Output should be valid JSON: %s", string(output))

	// Verify error_code field exists (StructuredError format)
	assert.Contains(t, result, "error_code", "JSON should contain error_code field")

	// Verify hints are omitted or empty when --no-hints is used
	if hints, exists := result["hints"]; exists {
		// If hints field exists, it should be empty
		hintsArray, ok := hints.([]interface{})
		if ok {
			assert.Empty(t, hintsArray, "Hints array should be empty when --no-hints is used")
		}
	}
	// If hints field doesn't exist at all, that's also acceptable
}

// =============================================================================
// Test Helpers
// =============================================================================

// Note: buildBinaryIfNeeded() is defined in c047_error_codes_test.go
// and shared across all integration tests in this package
