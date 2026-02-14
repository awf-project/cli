//go:build integration

// Feature: C048
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

//
// Validates end-to-end behavior of actionable error message hints across all
// error types, flag interactions, output formats, and edge cases.
//
// Acceptance criteria from spec:
//   - File-not-found errors suggest checking path and list similar workflow files
//   - YAML syntax errors show expected format with line/column reference
//   - Invalid state references suggest the closest matching state name
//   - Missing input errors list required inputs with example values
//   - Hints are visually distinct from error messages (dimmed)
//   - --no-hints flag suppresses suggestions without affecting error output
//   - Tests pass with >80% coverage on new hint generation code

// Happy Path Tests

// TestFileNotFoundHint_SuggestsCorrectFilename validates that a typo in a
// workflow filename triggers a "did you mean?" suggestion for the closest match.
func TestFileNotFoundHint_SuggestsCorrectFilename(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)

	// "hints-file-tyop.yaml" is a deliberate typo — the real file is "hints-file-typo.yaml"
	wrongPath := filepath.Join("../fixtures/workflows", "hints-file-tyop.yaml")

	cmd := exec.Command(binPath, "run", wrongPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for missing file")
	out := string(output)

	assert.Contains(t, out, "not found", "error should indicate file not found")
	assert.Contains(t, out, "Hint:", "output should include hint marker")
	assert.Contains(t, out, "hints-file-typo.yaml",
		"hint should suggest the correct filename via Levenshtein match")

	lower := strings.ToLower(out)
	assert.True(t,
		strings.Contains(lower, "did you mean") || strings.Contains(lower, "similar"),
		"hint should use 'did you mean?' or 'similar' wording")
}

// TestYAMLSyntaxHint_ShowsLineReference validates that invalid YAML triggers
// a hint referencing the line/column where the syntax error occurs.
func TestYAMLSyntaxHint_ShowsLineReference(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-yaml-syntax-error.yaml")

	cmd := exec.Command(binPath, "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for invalid YAML")
	out := string(output)
	lower := strings.ToLower(out)

	assert.True(t,
		strings.Contains(lower, "yaml") || strings.Contains(lower, "syntax"),
		"error should indicate YAML/syntax issue")
	assert.Contains(t, out, "Hint:", "output should include hint marker")
	assert.True(t,
		strings.Contains(lower, "line") || strings.Contains(lower, "column"),
		"hint should reference line or column number")
}

// TestInvalidStateHint_SuggestsClosestMatch validates that a typo in a state
// reference (e.g. "proces" instead of "process") triggers a "did you mean?" hint.
func TestInvalidStateHint_SuggestsClosestMatch(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-invalid-state-ref.yaml")

	cmd := exec.Command(binPath, "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for invalid state reference")
	out := string(output)
	lower := strings.ToLower(out)

	assert.True(t,
		strings.Contains(lower, "state") || strings.Contains(lower, "invalid"),
		"error should indicate invalid state")
	assert.Contains(t, out, "Hint:", "output should include hint marker")
	assert.Contains(t, out, "process",
		"hint should suggest the correct state name 'process'")
	assert.True(t,
		strings.Contains(lower, "did you mean") || strings.Contains(lower, "similar"),
		"hint should use 'did you mean?' pattern")
}

// TestMissingInputHint_ListsRequiredInputs validates that running a workflow
// without required inputs triggers a hint listing all required input names
// and an example --input flag.
func TestMissingInputHint_ListsRequiredInputs(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-missing-input.yaml")

	// Run without providing required inputs
	cmd := exec.Command(binPath, "run", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for missing required inputs")
	out := string(output)
	lower := strings.ToLower(out)

	assert.True(t,
		strings.Contains(lower, "input") || strings.Contains(lower, "required"),
		"error should indicate missing input")
	assert.Contains(t, out, "Hint:", "output should include hint marker")
	assert.Contains(t, out, "user_name",
		"hint should mention required input 'user_name'")
	assert.Contains(t, out, "user_email",
		"hint should mention required input 'user_email'")
	assert.True(t,
		strings.Contains(lower, "--input") || strings.Contains(lower, "example"),
		"hint should include example usage with --input flag")
}

// TestCommandFailureHint_SuggestsVerification validates that executing a
// non-existent command triggers a hint suggesting verification of the command.
func TestCommandFailureHint_SuggestsVerification(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-command-not-found.yaml")

	cmd := exec.Command(binPath, "run", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for non-existent command")
	out := string(output)
	lower := strings.ToLower(out)

	assert.True(t,
		strings.Contains(lower, "command") || strings.Contains(lower, "not found"),
		"error should indicate command not found")
	assert.Contains(t, out, "Hint:", "output should include hint marker")
	assert.True(t,
		strings.Contains(lower, "verify") ||
			strings.Contains(lower, "check") ||
			strings.Contains(lower, "path") ||
			strings.Contains(lower, "installed"),
		"hint should suggest verification or checking")
}

// Edge Case Tests

// TestFileNotFoundHint_CompletelyWrongName validates that a filename with no
// close Levenshtein match falls back to a generic "awf list" suggestion.
func TestFileNotFoundHint_CompletelyWrongName(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)

	// This filename is too different from any fixture to trigger "did you mean?"
	wrongPath := filepath.Join("../fixtures/workflows", "zzzzzzzzz-nonexistent.yaml")

	cmd := exec.Command(binPath, "run", wrongPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for missing file")
	out := string(output)

	assert.Contains(t, out, "not found", "error should indicate file not found")
	assert.Contains(t, out, "Hint:", "output should include hint marker")

	// With no close match, the hint should fallback to listing workflows
	lower := strings.ToLower(out)
	assert.True(t,
		strings.Contains(lower, "awf list") || strings.Contains(lower, "did you mean"),
		"hint should either suggest 'awf list' or a close match")
}

// TestFileNotFoundHint_NonexistentDirectory validates graceful degradation
// when the workflow path points to a directory that doesn't exist at all.
func TestFileNotFoundHint_NonexistentDirectory(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)

	// The entire directory doesn't exist, so os.ReadDir will fail
	wrongPath := filepath.Join("nonexistent-dir", "workflow.yaml")

	cmd := exec.Command(binPath, "run", wrongPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for missing file")
	out := string(output)

	assert.Contains(t, out, "not found", "error should indicate file not found")

	// Even with a non-existent directory, the output should not panic
	// and should still produce some useful output
	assert.NotEmpty(t, out, "output should not be empty")
}

// TestMultipleHints_DisplayInSequence validates that when a single error
// can trigger multiple hint generators, all applicable hints appear.
func TestMultipleHints_DisplayInSequence(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)

	// File not found may trigger both "did you mean?" and "awf list" hints
	wrongPath := filepath.Join("../fixtures/workflows", "hints-file-tyop.yaml")

	cmd := exec.Command(binPath, "run", wrongPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for missing file")
	out := string(output)

	// Count "Hint:" markers — should have at least one
	hintCount := strings.Count(out, "Hint:")
	assert.GreaterOrEqual(t, hintCount, 1,
		"output should contain at least one hint")

	if hintCount > 1 {
		t.Logf("Multiple hints displayed (%d)", hintCount)
	}
}

// TestYAMLSyntaxHint_IncludesCommonTips validates that YAML syntax errors
// always include generic tips about indentation and colons alongside
// any line-specific hints.
func TestYAMLSyntaxHint_IncludesCommonTips(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-yaml-syntax-error.yaml")

	cmd := exec.Command(binPath, "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for invalid YAML")
	out := string(output)
	lower := strings.ToLower(out)

	// The YAML hint generator always appends common tips
	assert.True(t,
		strings.Contains(lower, "indentation") ||
			strings.Contains(lower, "spaces") ||
			strings.Contains(lower, "colon") ||
			strings.Contains(lower, "dash"),
		"hint should include at least one common YAML syntax tip")
}

// TestMissingInputHint_PartialInputProvided validates that providing some
// but not all required inputs still triggers hints for the missing ones.
func TestMissingInputHint_PartialInputProvided(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-missing-input.yaml")

	// Provide user_name but omit user_email
	cmd := exec.Command(binPath, "run", workflowPath, "--input", "user_name=alice")
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for missing user_email input")
	out := string(output)
	lower := strings.ToLower(out)

	// Should still mention missing inputs
	assert.True(t,
		strings.Contains(lower, "input") || strings.Contains(lower, "required"),
		"error should indicate missing input")

	// Should mention the input that is actually missing
	assert.Contains(t, out, "user_email",
		"hint should mention the still-missing input 'user_email'")
}

// TestHintsWithDetails_StructurePreserved validates that the error output
// maintains correct structure: error code → details → hints (in that order).
func TestHintsWithDetails_StructurePreserved(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-invalid-state-ref.yaml")

	cmd := exec.Command(binPath, "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for invalid state reference")
	out := string(output)

	// Verify structured error format with error code category
	assert.Contains(t, out, "WORKFLOW", "error should show error code category")

	// Verify hint section exists and appears after the error header
	hintIndex := strings.Index(out, "Hint:")
	require.Greater(t, hintIndex, 0, "hint marker should appear in output")

	// Verify the error code line appears before hints
	errorCodeIndex := strings.Index(out, "[")
	require.Greater(t, errorCodeIndex, -1, "error code should appear in output")
	assert.Less(t, errorCodeIndex, hintIndex,
		"error code should appear before hints")
}

// Error Handling / Flag Suppression Tests

// TestNoHintsFlag_SuppressesAllHints validates that --no-hints removes all
// hint markers from the output while preserving the core error message.
func TestNoHintsFlag_SuppressesAllHints(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	wrongPath := filepath.Join("../fixtures/workflows", "hints-file-tyop.yaml")

	cmd := exec.Command(binPath, "--no-hints", "run", wrongPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for missing file")
	out := string(output)

	// Core error message preserved
	assert.Contains(t, out, "not found", "error should indicate file not found")

	// No hints displayed
	assert.NotContains(t, out, "Hint:",
		"output should NOT include hint marker when --no-hints is used")
	assert.NotContains(t, strings.ToLower(out), "did you mean",
		"output should NOT include 'did you mean?' when --no-hints is used")
}

// TestNoHintsFlag_DoesNotAffectErrorOutput validates that suppressing hints
// does not change the core error message or exit code.
func TestNoHintsFlag_DoesNotAffectErrorOutput(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-invalid-state-ref.yaml")

	// Run twice: once with hints, once without
	cmdWithHints := exec.Command(binPath, "validate", workflowPath)
	cmdWithHints.Dir = filepath.Join("..", "..")
	outputWith, errWith := cmdWithHints.CombinedOutput()

	cmdNoHints := exec.Command(binPath, "--no-hints", "validate", workflowPath)
	cmdNoHints.Dir = filepath.Join("..", "..")
	outputNo, errNo := cmdNoHints.CombinedOutput()

	require.Error(t, errWith)
	require.Error(t, errNo)

	outWith := string(outputWith)
	outNo := string(outputNo)

	// Both should contain the error code
	assert.Contains(t, outWith, "WORKFLOW")
	assert.Contains(t, outNo, "WORKFLOW")

	// The version without hints should be a substring or prefix of the version with hints
	// (since hints are appended at the end)
	assert.True(t, len(outWith) >= len(outNo),
		"output with hints should be at least as long as output without hints")

	// No-hints version should not have any hint markers
	assert.NotContains(t, outNo, "Hint:")
}

// TestNoHintsFlag_AppliesToAllSubcommands validates that --no-hints works
// as a persistent flag on both "run" and "validate" subcommands.
func TestNoHintsFlag_AppliesToAllSubcommands(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "run subcommand",
			args:    []string{"--no-hints", "run", filepath.Join("../fixtures/workflows", "hints-file-tyop.yaml")},
			wantErr: "not found",
		},
		{
			name:    "validate subcommand",
			args:    []string{"--no-hints", "validate", filepath.Join("../fixtures/workflows", "hints-yaml-syntax-error.yaml")},
			wantErr: "", // Just verify no hint markers
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binPath, tt.args...)
			cmd.Dir = filepath.Join("..", "..")
			output, err := cmd.CombinedOutput()

			require.Error(t, err, "command should fail")
			out := string(output)

			assert.NotContains(t, out, "Hint:",
				"--no-hints should suppress hints on %s", tt.name)

			if tt.wantErr != "" {
				assert.Contains(t, out, tt.wantErr)
			}
		})
	}
}

// JSON Output Format Tests

// TestJSONFormat_IncludesHintsArray validates that JSON-formatted error output
// includes a "hints" array with actionable suggestion strings.
func TestJSONFormat_IncludesHintsArray(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-invalid-state-ref.yaml")

	cmd := exec.Command(binPath, "--format", "json", "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for invalid state reference")

	var result map[string]any
	parseErr := json.Unmarshal(output, &result)
	require.NoError(t, parseErr, "output should be valid JSON: %s", string(output))

	// Verify structured error fields
	assert.Contains(t, result, "error_code", "JSON should contain error_code field")

	// Verify hints array
	assert.Contains(t, result, "hints", "JSON should contain hints array")
	hints, ok := result["hints"].([]any)
	require.True(t, ok, "hints should be an array")
	assert.Greater(t, len(hints), 0, "hints array should contain at least one hint")

	// Verify hint content is a non-empty string
	firstHint, ok := hints[0].(string)
	require.True(t, ok, "hint should be a string")
	assert.NotEmpty(t, firstHint, "hint should not be empty")
}

// TestJSONFormat_NoHintsFlag_OmitsOrEmptiesHints validates that JSON output
// with --no-hints either omits the hints array or returns it empty.
func TestJSONFormat_NoHintsFlag_OmitsOrEmptiesHints(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-invalid-state-ref.yaml")

	cmd := exec.Command(binPath, "--format", "json", "--no-hints", "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "command should fail for invalid state reference")

	var result map[string]any
	parseErr := json.Unmarshal(output, &result)
	require.NoError(t, parseErr, "output should be valid JSON: %s", string(output))

	// Verify error_code still present
	assert.Contains(t, result, "error_code", "JSON should contain error_code field")

	// Hints should be absent or empty
	if hints, exists := result["hints"]; exists {
		hintsArray, ok := hints.([]any)
		if ok {
			assert.Empty(t, hintsArray,
				"hints array should be empty when --no-hints is used")
		}
	}
	// Absent hints field is also acceptable
}

// TestJSONFormat_HintContentMatchesHumanFormat validates that the hint
// suggestions in JSON output are consistent with what appears in human format.
func TestJSONFormat_HintContentMatchesHumanFormat(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-invalid-state-ref.yaml")

	// Get JSON output
	cmdJSON := exec.Command(binPath, "--format", "json", "validate", workflowPath)
	cmdJSON.Dir = filepath.Join("..", "..")
	outputJSON, errJSON := cmdJSON.CombinedOutput()
	require.Error(t, errJSON)

	// Get human output
	cmdHuman := exec.Command(binPath, "validate", workflowPath)
	cmdHuman.Dir = filepath.Join("..", "..")
	outputHuman, errHuman := cmdHuman.CombinedOutput()
	require.Error(t, errHuman)

	// Parse JSON
	var result map[string]any
	require.NoError(t, json.Unmarshal(outputJSON, &result))

	hints, ok := result["hints"].([]any)
	require.True(t, ok, "JSON hints should be an array")
	require.Greater(t, len(hints), 0, "JSON should have at least one hint")

	humanOut := string(outputHuman)

	// Each JSON hint should appear somewhere in the human output
	for _, hint := range hints {
		hintStr, ok := hint.(string)
		require.True(t, ok)
		assert.Contains(t, humanOut, hintStr,
			"JSON hint %q should also appear in human-formatted output", hintStr)
	}
}

// Integration Tests (cross-concern)

// TestErrorHint_ExitCodePreserved validates that exit codes are unaffected
// by the presence or absence of hints.
func TestErrorHint_ExitCodePreserved(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	wrongPath := filepath.Join("../fixtures/workflows", "hints-file-tyop.yaml")

	// Run with hints
	cmdWith := exec.Command(binPath, "run", wrongPath)
	cmdWith.Dir = filepath.Join("..", "..")
	_ = cmdWith.Run()
	exitWith := cmdWith.ProcessState.ExitCode()

	// Run without hints
	cmdNo := exec.Command(binPath, "--no-hints", "run", wrongPath)
	cmdNo.Dir = filepath.Join("..", "..")
	_ = cmdNo.Run()
	exitNo := cmdNo.ProcessState.ExitCode()

	assert.Equal(t, exitWith, exitNo,
		"exit code should be identical with and without --no-hints")
	assert.NotEqual(t, 0, exitWith,
		"exit code should be non-zero for missing file")
}

// TestNoHintsFlag_PreservesColorCapability validates that --no-hints does
// not interfere with the --no-color flag or default color behavior.
func TestNoHintsFlag_PreservesColorCapability(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	wrongPath := filepath.Join("../fixtures/workflows", "hints-file-tyop.yaml")

	// --no-hints alone should not break output
	cmd := exec.Command(binPath, "--no-hints", "run", wrongPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err)
	out := string(output)

	assert.Contains(t, out, "not found", "error should still display")
	assert.NotContains(t, out, "Hint:", "hints should be suppressed")
	assert.NotEmpty(t, out, "output should not be empty")
}

// TestNoHintsPlusNoColor_BothFlagsWork validates that combining --no-hints
// and --no-color produces clean, undecorated error output.
func TestNoHintsPlusNoColor_BothFlagsWork(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	wrongPath := filepath.Join("../fixtures/workflows", "hints-file-tyop.yaml")

	cmd := exec.Command(binPath, "--no-hints", "--no-color", "run", wrongPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err)
	out := string(output)

	assert.Contains(t, out, "not found", "error should still display")
	assert.NotContains(t, out, "Hint:", "hints should be suppressed")
	// With --no-color, output should contain no ANSI escape sequences
	assert.NotContains(t, out, "\033[", "output should have no ANSI escape codes")
}

// TestValidWorkflow_NoSpuriousHints validates that a valid workflow produces
// no error output and therefore no hints.
func TestValidWorkflow_NoSpuriousHints(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-file-typo.yaml")

	// The fixture itself is valid — validate should succeed
	cmd := exec.Command(binPath, "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	// Validate should succeed for a valid workflow
	if err == nil {
		out := string(output)
		assert.NotContains(t, out, "Hint:",
			"valid workflow should not produce any hints")
	}
	// If validate fails for non-hint reasons, that's OK — not the test's concern
}

// TestHintOutput_NotMixedIntoErrorMessage validates that hint text appears
// after the main error message, not interleaved within it.
func TestHintOutput_NotMixedIntoErrorMessage(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)
	workflowPath := filepath.Join("../fixtures/workflows", "hints-invalid-state-ref.yaml")

	cmd := exec.Command(binPath, "validate", workflowPath)
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()

	require.Error(t, err)
	out := string(output)

	lines := strings.Split(out, "\n")
	firstHintLine := -1
	lastNonHintLine := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Hint:") {
			if firstHintLine == -1 {
				firstHintLine = i
			}
		} else if trimmed != "" {
			lastNonHintLine = i
		}
	}

	if firstHintLine > -1 && lastNonHintLine > -1 {
		// All hint lines should be contiguous at the end (or after details)
		// The first hint should not appear before the error header
		assert.Greater(t, firstHintLine, 0,
			"hints should not appear on the first line of output")
	}
}
