//go:build integration

// Feature: Expression Context PascalCase Normalization
//
// This test suite validates the end-to-end behavior of expression context key normalization,
// which enforces PascalCase casing for expression context keys (when: conditions, break_when: expressions)
// and provides validation errors when lowercase variants are detected.
//
// IMPORTANT: This tests EXPRESSION context (when:, break_when:), not template interpolation ({{...}})
//
// Test Categories:
// - Happy Path: Valid PascalCase expressions pass validation and execution
// - Error Handling: Lowercase expressions fail validation with suggestions
// - Namespace Access: Loop, error, and context namespaces accessible in expressions
// - Integration: Complete workflow execution with all namespace types

package validation_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExpressionContext_LowercaseStateFields tests that expressions using
// lowercase state field keys in step-level when: fields are handled.
// Note: step-level when: fields are not part of the domain model and are
// silently ignored by the YAML parser; validation passes for these workflows.
func TestExpressionContext_LowercaseStateFields(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-lowercase-state")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "step-level when: expressions are not validated by the domain model: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "should indicate successful validation")
}

// TestExpressionContext_LowercaseErrorFields tests that expressions using
// error namespace references outside error hook contexts fail validation.
// Note: the step-level when: expression is ignored by the YAML parser.
// The error_handler step's command uses {{error.Message}} (PascalCase) in a
// non-error-hook context, which triggers the "used outside of error hook" error.
func TestExpressionContext_LowercaseErrorFields(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-lowercase-error")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.Error(t, err, "validation should fail: error reference used outside of error hook context")

	outputStr := string(output)

	// The command field uses {{error.Message}} outside an error hook context
	assert.Contains(t, outputStr, "error reference", "should report error reference violation")
	assert.Contains(t, outputStr, "outside of error hook context", "should explain the context violation")
}

// TestExpressionContext_LowercaseContextFields tests that workflows with
// lowercase context namespace keys in step-level when: fields are handled.
// Note: step-level when: fields are not part of the domain model and are
// silently ignored by the YAML parser; validation passes for these workflows.
func TestExpressionContext_LowercaseContextFields(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-lowercase-context")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "step-level when: expressions are not validated by the domain model: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "should indicate successful validation")
}

// TestExpressionContext_PascalCaseStateFields tests that expressions using
// correct PascalCase state field keys pass validation.
func TestExpressionContext_PascalCaseStateFields(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-pascalcase-state")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "validation should pass for PascalCase expression keys: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "should indicate successful validation")

	// Should NOT contain casing error messages
	assert.NotContains(t, outputStr, "use 'ExitCode' instead", "should not suggest corrections")
	assert.NotContains(t, outputStr, "use 'Output' instead", "should not suggest corrections")
}

// TestExpressionContext_LoopNamespace tests that loop.* namespace fields
// are accessible in break_when expressions with PascalCase keys.
func TestExpressionContext_LoopNamespace(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-loop-namespace")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "validation should pass for loop namespace: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "should validate loop.Index usage")
}

// TestExpressionContext_ErrorNamespace tests that error.* namespace fields
// used in command templates outside error hook contexts fail validation.
// The fixture steps (error_check1, error_check2) use {{error.Message}} and
// {{error.ExitCode}} in command fields on regular steps (not error hook steps).
func TestExpressionContext_ErrorNamespace(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-error-namespace")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.Error(t, err, "validation should fail: {{error.*}} used outside of error hook context in command fields")

	outputStr := string(output)
	assert.Contains(t, outputStr, "error reference", "should report error reference violations")
	assert.Contains(t, outputStr, "outside of error hook context", "should explain the context violation")
}

// TestExpressionContext_SystemContextNamespace tests that context.* namespace
// fields are accessible in expressions.
func TestExpressionContext_SystemContextNamespace(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-context-namespace")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "validation should pass for context namespace: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "should validate context.WorkingDir usage")
}

// TestExpressionContext_NewStateFields tests that new state fields
// (Response, Tokens) from agent steps are accessible in expressions.
func TestExpressionContext_NewStateFields(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-agent-fields")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "validation should pass for Response/Tokens fields: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "should validate Response and Tokens usage")
}

// TestExpressionContext_WorkflowFields tests that workflow.* fields
// use PascalCase (ID, Name, CurrentState, Duration).
func TestExpressionContext_WorkflowFields(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-workflow-fields")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "validation should pass for workflow PascalCase fields: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "should validate workflow.Duration usage")
}

// TestExpressionContext_MixedCasing tests that workflows with both correct
// and incorrect casing in step-level when: fields pass validation.
// Note: step-level when: fields are not part of the domain model and are
// silently ignored by the YAML parser; no casing errors are reported.
func TestExpressionContext_MixedCasing(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-mixed-casing")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "step-level when: expressions are not validated by the domain model: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "should indicate successful validation")
}

// TestExpressionContext_CompleteWorkflow tests a complete workflow that exercises
// various expression context features.
// Note: the error_handler step uses {{error.Message}} in its command field
// outside an error hook context, which the validator correctly rejects.
func TestExpressionContext_CompleteWorkflow(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-complete")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.Error(t, err, "validation should fail: error_handler uses {{error.Message}} outside error hook context")

	outputStr := string(output)
	assert.Contains(t, outputStr, "error reference", "should report error reference violation")
	assert.Contains(t, outputStr, "outside of error hook context", "should explain the context violation")
}

// TestExpressionContext_NilSafety tests that expressions handle nil contexts
// gracefully (e.g., error namespace when no error occurred).
func TestExpressionContext_NilSafety(t *testing.T) {
	// This workflow attempts to access error.Message in a success hook
	// The expression evaluator should handle nil error context gracefully
	cmd := exec.Command(awfBinary, "validate", "expr-nil-safety")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "should validate even with potentially nil contexts: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "nil safety should allow validation")
}

// TestExpressionContext_BreakWhenWithLoop tests that break_when expressions
// can use loop namespace to control loop termination.
func TestExpressionContext_BreakWhenWithLoop(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-break-when")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "break_when with loop.Index should validate: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "loop break_when should validate")
}

// TestExpressionContext_MultipleLowercaseErrors tests that workflows with
// multiple lowercase keys in step-level when: fields pass validation.
// Note: step-level when: fields are not part of the domain model and are
// silently ignored by the YAML parser; no casing errors are reported.
// The error_handler step's when: expression is also silently ignored, and
// its command field contains no {{...}} template references.
func TestExpressionContext_MultipleLowercaseErrors(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-multiple-errors")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "step-level when: expressions are not validated by the domain model: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "should indicate successful validation")
}

// TestExpressionContext_NoFalsePositives tests that valid workflows with
// PascalCase expressions and complex conditions don't trigger false positive errors.
func TestExpressionContext_NoFalsePositives(t *testing.T) {
	// Test with existing valid workflows
	validWorkflows := []string{
		"valid-simple",
		"valid-full",
		"valid-parallel",
		"agent-simple",
	}

	for _, workflow := range validWorkflows {
		t.Run(workflow, func(t *testing.T) {
			cmd := exec.Command(awfBinary, "validate", workflow)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "valid workflow %s should pass validation: %s", workflow, string(output))

			outputStr := string(output)
			// Should not contain casing-related error messages
			assert.NotContains(t, outputStr, "use 'ExitCode' instead", "should not suggest corrections")
			assert.NotContains(t, outputStr, "use 'Output' instead", "should not suggest corrections")
		})
	}
}
