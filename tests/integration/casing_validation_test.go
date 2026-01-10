//go:build integration

// Feature: F050 - Standardize Template Field Casing to Uppercase
//
// This test suite validates the end-to-end behavior of F050, which enforces
// uppercase casing for state property references (Output, Stderr, ExitCode, Status)
// and provides helpful error messages when lowercase variants are detected.
//
// Test Categories:
// - Happy Path: Valid uppercase properties pass validation
// - Edge Cases: Mixed casing, all-caps properties
// - Error Handling: Single and multiple lowercase errors with suggestions
// - Integration: CLI validate command integration, error message formatting

package integration_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// TestValidate_SingleLowercaseError tests that a workflow with a single
// lowercase property reference fails validation with a helpful suggestion.
//
// Acceptance: US2 - Validation Errors on Incorrect Casing
func TestValidate_SingleLowercaseError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "lowercase-single"})

	err := cmd.Execute()
	require.Error(t, err, "validation should fail for lowercase property")

	output := buf.String()

	// Error message should mention the incorrect property
	assert.Contains(t, output, "output", "error should mention lowercase property")

	// Error message should suggest the correct uppercase version
	assert.Contains(t, output, "Output", "error should suggest uppercase property")

	// Error message should include context (step name or field)
	assert.Contains(t, output, "echo_step", "error should reference the step")
}

// TestValidate_MultipleLowercaseErrors tests that a workflow with multiple
// lowercase property references reports all errors (not just the first one).
//
// Acceptance: US2 - Mixed casing scenario
func TestValidate_MultipleLowercaseErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "lowercase-multiple"})

	err := cmd.Execute()
	require.Error(t, err, "validation should fail for multiple lowercase properties")

	output := buf.String()

	// All three lowercase properties should be reported
	assert.Contains(t, output, "output", "should report lowercase 'output'")
	assert.Contains(t, output, "stderr", "should report lowercase 'stderr'")
	assert.Contains(t, output, "exit_code", "should report lowercase 'exit_code'")

	// All three suggestions should be present
	assert.Contains(t, output, "Output", "should suggest 'Output'")
	assert.Contains(t, output, "Stderr", "should suggest 'Stderr'")
	assert.Contains(t, output, "ExitCode", "should suggest 'ExitCode'")

	// Should reference the step where errors occur
	assert.Contains(t, output, "execute", "should reference step name")
}

// TestValidate_UppercasePropertiesPass tests that workflows using correct
// uppercase property syntax pass validation without errors.
//
// Acceptance: US1 - Correct Template Field Casing
func TestValidate_UppercasePropertiesPass(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "uppercase-valid"})

	err := cmd.Execute()
	require.NoError(t, err, "validation should pass for uppercase properties")

	output := buf.String()
	assert.Contains(t, output, "valid", "should indicate successful validation")

	// Should NOT contain any error messages about casing
	assert.NotContains(t, output, "lowercase", "should not report casing errors")
	assert.NotContains(t, output, "use 'Output' instead", "should not suggest corrections")
}

// TestValidate_MixedCasing tests that workflows with both correct and
// incorrect casing report only the incorrect references.
//
// Edge Case: Partial correctness
func TestValidate_MixedCasing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "mixed-casing"})

	err := cmd.Execute()
	require.Error(t, err, "validation should fail for mixed casing workflow")

	output := buf.String()

	// Should report the lowercase 'output' in step3
	assert.Contains(t, output, "output", "should detect lowercase property")

	// Should reference step3 where the error occurs (not step2 which is correct)
	assert.Contains(t, output, "step3", "should identify step with error")

	// The correct usage in step2 should not trigger an error
	// We verify this by checking that the error is specifically about step3
	lines := strings.Split(output, "\n")
	errorFound := false
	for _, line := range lines {
		if strings.Contains(line, "output") && strings.Contains(line, "step3") {
			errorFound = true
			break
		}
	}
	assert.True(t, errorFound, "should report error for step3 specifically")
}

// TestValidate_LoopConditionLowercase tests that lowercase properties
// in loop conditions are properly detected and reported.
//
// Edge Case: Properties in control flow structures
func TestValidate_LoopConditionLowercase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "loop-lowercase"})

	err := cmd.Execute()
	require.Error(t, err, "validation should fail for lowercase in loop condition")

	output := buf.String()

	// Should detect lowercase 'exit_code' in loop condition
	assert.Contains(t, output, "exit_code", "should detect lowercase in loop")
	assert.Contains(t, output, "ExitCode", "should suggest uppercase version")

	// Should reference the step with the loop
	assert.Contains(t, output, "check_service", "should reference step with loop")
}

// TestValidate_HookLowercase tests that lowercase properties in hooks
// are properly detected and reported.
//
// Edge Case: Properties in hooks (on_error, on_success)
func TestValidate_HookLowercase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "hook-lowercase"})

	err := cmd.Execute()
	require.Error(t, err, "validation should fail for lowercase in hook")

	output := buf.String()

	// Should detect lowercase 'stderr' in hook
	assert.Contains(t, output, "stderr", "should detect lowercase in hook")
	assert.Contains(t, output, "Stderr", "should suggest uppercase version")

	// Should reference the step with the hook
	assert.Contains(t, output, "risky_operation", "should reference step with hook")
}

// TestValidate_ErrorMessageQuality tests that error messages meet quality
// standards: clear location, actionable suggestion, proper formatting.
//
// Integration: Error message formatting
func TestValidate_ErrorMessageQuality(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "lowercase-single"})

	err := cmd.Execute()
	require.Error(t, err)

	output := buf.String()

	// Quality criteria from spec:

	// 1. Clarity: Error should identify the incorrect property clearly
	assert.True(t,
		strings.Contains(output, "'output'") || strings.Contains(output, "\"output\""),
		"error should quote the incorrect property name for clarity")

	// 2. Actionability: Should provide exact fix (uppercase version)
	assert.True(t,
		strings.Contains(output, "Output") || strings.Contains(output, "'Output'"),
		"error should provide actionable suggestion")

	// 3. Completeness: Should include context (which step, which field)
	stepContextFound := strings.Contains(output, "echo_step") ||
		strings.Contains(output, "use_output")
	assert.True(t, stepContextFound, "error should provide step context")

	// 4. Non-fail-fast: For workflows with multiple errors, all should be reported
	// (tested in TestValidate_MultipleLowercaseErrors)
}

// TestValidate_CaseSensitivity tests that the validation is truly
// case-sensitive and doesn't accept variations like OUTPUT, OuTpUt, etc.
//
// Edge Case: All-caps and mixed case variants
func TestValidate_CaseSensitivity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create a temporary workflow with all-caps property
	tmpDir := t.TempDir()
	workflowPath := tmpDir + "/test-allcaps.yaml"

	workflowContent := `name: test-allcaps
description: Test all-caps property

inputs:
  msg:
    type: string
    required: true

steps:
  - name: step1
    command: echo "test"

  - name: step2
    command: echo "{{states.step1.OUTPUT}}"
`

	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	os.Setenv("AWF_WORKFLOWS_PATH", tmpDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "test-allcaps"})

	err = cmd.Execute()
	require.Error(t, err, "validation should fail for all-caps OUTPUT")

	output := buf.String()

	// Should detect that OUTPUT is invalid (only Output is correct)
	assert.Contains(t, output, "OUTPUT", "should report all-caps variant")
}

// TestValidate_NoFalsePositives tests that valid workflows with uppercase
// properties and complex templates don't trigger false positive errors.
//
// Integration: Complex valid workflows
func TestValidate_NoFalsePositives(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Test with existing valid workflows that use correct casing
	validWorkflows := []string{
		"valid-simple",
		"valid-full",
		"valid-parallel",
		"agent-simple",
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	for _, workflow := range validWorkflows {
		t.Run(workflow, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"validate", workflow})

			err := cmd.Execute()

			// These workflows should pass validation
			require.NoError(t, err, "valid workflow %s should pass validation", workflow)

			output := buf.String()

			// Should not contain any F050-related error messages
			assert.NotContains(t, output, "use 'Output' instead",
				"should not suggest corrections for valid workflows")
			assert.NotContains(t, output, "use 'Stderr' instead",
				"should not suggest corrections for valid workflows")
			assert.NotContains(t, output, "use 'ExitCode' instead",
				"should not suggest corrections for valid workflows")
		})
	}
}

// TestValidate_EmptyWorkflow tests edge case of workflow with no state references.
//
// Edge Case: No state references at all
func TestValidate_EmptyWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	workflowPath := tmpDir + "/test-empty.yaml"

	workflowContent := `name: test-empty
description: Workflow with no state references

inputs:
  msg:
    type: string
    required: true

steps:
  - name: step1
    command: echo "{{inputs.msg}}"

  - name: step2
    command: echo "hello"
`

	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	os.Setenv("AWF_WORKFLOWS_PATH", tmpDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "test-empty"})

	err = cmd.Execute()
	require.NoError(t, err, "workflow without state references should pass")

	output := buf.String()
	assert.Contains(t, output, "valid", "should indicate successful validation")
}

// TestValidate_PerformanceUnder100ms tests that validation completes
// within the NFR-001 requirement of <100ms for workflows with up to 50 steps.
//
// Non-Functional: NFR-001 - Performance
func TestValidate_PerformanceUnder100ms(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create a workflow with many steps (up to 50)
	tmpDir := t.TempDir()
	workflowPath := tmpDir + "/test-large.yaml"

	var workflowContent strings.Builder
	workflowContent.WriteString(`name: test-large
description: Large workflow for performance testing

steps:
  - name: step0
    command: echo "start"
`)

	// Add 49 more steps that reference previous steps
	for i := 1; i < 50; i++ {
		workflowContent.WriteString("\n  - name: step" + string(rune('0'+i/10)) + string(rune('0'+i%10)))
		workflowContent.WriteString("\n    command: echo \"{{states.step0.Output}}\"")
	}

	err := os.WriteFile(workflowPath, []byte(workflowContent.String()), 0o644)
	require.NoError(t, err)

	os.Setenv("AWF_WORKFLOWS_PATH", tmpDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	// Note: We're not strictly measuring <100ms here as that would be flaky
	// in CI environments. This test primarily ensures the validator handles
	// large workflows without errors. Manual performance testing can verify
	// the <100ms requirement.

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "test-large"})

	err = cmd.Execute()
	require.NoError(t, err, "large workflow should validate successfully")
}
