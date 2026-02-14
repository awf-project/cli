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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExpressionContext_LowercaseStateFields tests that expressions using
// lowercase state field keys fail validation with helpful suggestions.
func TestExpressionContext_LowercaseStateFields(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-lowercase-state")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.Error(t, err, "validation should fail for lowercase expression keys")

	outputStr := string(output)

	// Should detect lowercase 'exit_code' in when: expression
	assert.Contains(t, outputStr, "exit_code", "should detect lowercase 'exit_code' in expression")
	assert.Contains(t, outputStr, "ExitCode", "should suggest uppercase 'ExitCode'")

	// Should detect lowercase 'output' in when: expression
	assert.Contains(t, outputStr, "output", "should detect lowercase 'output' in expression")
	assert.Contains(t, outputStr, "Output", "should suggest uppercase 'Output'")
}

// TestExpressionContext_LowercaseErrorFields tests that expressions using
// lowercase error namespace keys fail validation.
func TestExpressionContext_LowercaseErrorFields(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-lowercase-error")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.Error(t, err, "validation should fail for lowercase error field keys")

	outputStr := string(output)

	// Should detect lowercase 'message' in error namespace
	assert.Contains(t, outputStr, "message", "should detect lowercase 'message' in error namespace")
	assert.Contains(t, outputStr, "Message", "should suggest uppercase 'Message'")
}

// TestExpressionContext_LowercaseContextFields tests that expressions using
// lowercase context namespace keys fail validation.
func TestExpressionContext_LowercaseContextFields(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-lowercase-context")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.Error(t, err, "validation should fail for lowercase context field keys")

	outputStr := string(output)

	// Should detect lowercase 'working_dir' in context namespace
	assert.Contains(t, outputStr, "working_dir", "should detect lowercase 'working_dir' in context namespace")
	assert.Contains(t, outputStr, "WorkingDir", "should suggest uppercase 'WorkingDir'")
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
// are accessible in error hook when: expressions.
func TestExpressionContext_ErrorNamespace(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-error-namespace")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "validation should pass for error namespace: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "should validate error.Message usage")
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
// and incorrect casing report only the incorrect references.
func TestExpressionContext_MixedCasing(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-mixed-casing")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.Error(t, err, "validation should fail for mixed casing workflow")

	outputStr := string(output)

	// Should report the lowercase 'output' but not complain about correct 'ExitCode'
	assert.Contains(t, outputStr, "output", "should detect lowercase property")
	assert.Contains(t, outputStr, "Output", "should suggest uppercase version")

	// The correct PascalCase usage should not trigger errors
	// We verify by ensuring the error message doesn't suggest fixing already-correct fields
	assert.NotContains(t, outputStr, "use 'ExitCode' instead", "should not suggest fixing correct field")
}

// TestExpressionContext_CompleteWorkflow tests a complete workflow that uses
// all expression context features: state fields, loop namespace, error namespace,
// context namespace, and workflow fields.
func TestExpressionContext_CompleteWorkflow(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-complete")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "complete workflow should validate: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "valid", "complete workflow should pass validation")
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
// multiple lowercase expression keys report all errors (non-fail-fast).
func TestExpressionContext_MultipleLowercaseErrors(t *testing.T) {
	cmd := exec.Command(awfBinary, "validate", "expr-multiple-errors")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH=../../fixtures/workflows")

	output, err := cmd.CombinedOutput()
	require.Error(t, err, "validation should fail for multiple lowercase keys")

	outputStr := string(output)

	// Should report multiple errors
	errorCount := strings.Count(outputStr, "exit_code") + strings.Count(outputStr, "output") + strings.Count(outputStr, "stderr")
	assert.Greater(t, errorCount, 1, "should report multiple errors")
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
