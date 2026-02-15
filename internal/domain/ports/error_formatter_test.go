package ports_test

import (
	"testing"
	"time"

	"github.com/awf-project/awf/internal/domain/errors"
	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

// mockJSONFormatter simulates a JSON error formatter
type mockJSONFormatter struct {
	formatCalled bool
	lastError    *errors.StructuredError
}

func (m *mockJSONFormatter) FormatError(err *errors.StructuredError) string {
	m.formatCalled = true
	m.lastError = err
	// Simulate JSON output
	return `{"error_code":"` + string(err.Code) + `","message":"` + err.Message + `"}`
}

// mockHumanFormatter simulates a human-readable error formatter
type mockHumanFormatter struct {
	formatCalled bool
	lastError    *errors.StructuredError
}

func (m *mockHumanFormatter) FormatError(err *errors.StructuredError) string {
	m.formatCalled = true
	m.lastError = err
	// Simulate human-readable output with bracketed error code
	return "[" + string(err.Code) + "] " + err.Message
}

// mockEmptyFormatter returns empty string (edge case)
type mockEmptyFormatter struct{}

func (m *mockEmptyFormatter) FormatError(err *errors.StructuredError) string {
	return ""
}

func TestErrorFormatter_InterfaceCompliance(t *testing.T) {
	// Verify that mock implementations satisfy the ErrorFormatter interface
	var _ ports.ErrorFormatter = (*mockJSONFormatter)(nil)
	var _ ports.ErrorFormatter = (*mockHumanFormatter)(nil)
	var _ ports.ErrorFormatter = (*mockEmptyFormatter)(nil)
}

func TestErrorFormatter_FormatError_JSONStyle(t *testing.T) {
	tests := []struct {
		name       string
		inputError *errors.StructuredError
		wantOutput string
	}{
		{
			name: "User error with details",
			inputError: errors.NewStructuredError(
				errors.ErrorCodeUserInputMissingFile,
				"workflow file not found",
				map[string]any{"path": "/path/to/workflow.yaml"},
				nil,
			),
			wantOutput: `{"error_code":"USER.INPUT.MISSING_FILE","message":"workflow file not found"}`,
		},
		{
			name: "Workflow error without details",
			inputError: errors.NewStructuredError(
				errors.ErrorCodeWorkflowValidationCycleDetected,
				"cycle detected in state machine",
				nil,
				nil,
			),
			wantOutput: `{"error_code":"WORKFLOW.VALIDATION.CYCLE_DETECTED","message":"cycle detected in state machine"}`,
		},
		{
			name: "Execution error with cause",
			inputError: errors.NewStructuredError(
				errors.ErrorCodeExecutionCommandTimeout,
				"command exceeded timeout",
				map[string]any{"timeout": "30s"},
				assert.AnError,
			),
			wantOutput: `{"error_code":"EXECUTION.COMMAND.TIMEOUT","message":"command exceeded timeout"}`,
		},
		{
			name: "System error",
			inputError: errors.NewStructuredError(
				errors.ErrorCodeSystemIOReadFailed,
				"failed to read state file",
				nil,
				nil,
			),
			wantOutput: `{"error_code":"SYSTEM.IO.READ_FAILED","message":"failed to read state file"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := &mockJSONFormatter{}
			output := formatter.FormatError(tt.inputError)

			assert.True(t, formatter.formatCalled, "FormatError should be called")
			assert.Equal(t, tt.inputError, formatter.lastError, "Should format the input error")
			assert.Equal(t, tt.wantOutput, output, "JSON output should match expected format")
		})
	}
}

func TestErrorFormatter_FormatError_HumanStyle(t *testing.T) {
	tests := []struct {
		name       string
		inputError *errors.StructuredError
		wantOutput string
	}{
		{
			name: "User error with details",
			inputError: errors.NewStructuredError(
				errors.ErrorCodeUserInputMissingFile,
				"workflow file not found",
				map[string]any{"path": "/path/to/workflow.yaml"},
				nil,
			),
			wantOutput: "[USER.INPUT.MISSING_FILE] workflow file not found",
		},
		{
			name: "Workflow validation error",
			inputError: errors.NewStructuredError(
				errors.ErrorCodeWorkflowValidationInvalidTransition,
				"invalid state transition",
				nil,
				nil,
			),
			wantOutput: "[WORKFLOW.VALIDATION.INVALID_TRANSITION] invalid state transition",
		},
		{
			name: "Execution command failed",
			inputError: errors.NewStructuredError(
				errors.ErrorCodeExecutionCommandFailed,
				"command exited with non-zero status",
				map[string]any{"exit_code": 127},
				nil,
			),
			wantOutput: "[EXECUTION.COMMAND.FAILED] command exited with non-zero status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := &mockHumanFormatter{}
			output := formatter.FormatError(tt.inputError)

			assert.True(t, formatter.formatCalled, "FormatError should be called")
			assert.Equal(t, tt.inputError, formatter.lastError, "Should format the input error")
			assert.Equal(t, tt.wantOutput, output, "Human-readable output should include bracketed error code")
		})
	}
}

func TestErrorFormatter_MultipleFormats(t *testing.T) {
	// Same error formatted by different formatters should produce different outputs
	err := errors.NewStructuredError(
		errors.ErrorCodeUserInputValidationFailed,
		"input validation failed",
		map[string]any{"field": "workflow_name"},
		nil,
	)

	jsonFormatter := &mockJSONFormatter{}
	humanFormatter := &mockHumanFormatter{}

	jsonOutput := jsonFormatter.FormatError(err)
	humanOutput := humanFormatter.FormatError(err)

	assert.NotEqual(t, jsonOutput, humanOutput, "Different formatters should produce different outputs")
	assert.Contains(t, jsonOutput, `"error_code":"USER.INPUT.VALIDATION_FAILED"`, "JSON should include error_code field")
	assert.Contains(t, humanOutput, "[USER.INPUT.VALIDATION_FAILED]", "Human format should include bracketed code")
}

func TestErrorFormatter_NilDetails(t *testing.T) {
	// Error with nil details should be formatted correctly
	err := errors.NewStructuredError(
		errors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		nil, // nil details
		nil,
	)

	formatter := &mockJSONFormatter{}
	output := formatter.FormatError(err)

	assert.NotEmpty(t, output, "Should format error even with nil details")
	assert.Contains(t, output, "invalid YAML syntax", "Message should be included")
}

func TestErrorFormatter_EmptyDetails(t *testing.T) {
	// Error with empty details map should be formatted correctly
	err := errors.NewStructuredError(
		errors.ErrorCodeSystemIOWriteFailed,
		"failed to write file",
		map[string]any{}, // empty details
		nil,
	)

	humanFormatter := &mockHumanFormatter{}
	output := humanFormatter.FormatError(err)

	assert.NotEmpty(t, output, "Should format error even with empty details")
	assert.Contains(t, output, "[SYSTEM.IO.WRITE_FAILED]", "Error code should be included")
}

func TestErrorFormatter_EmptyMessage(t *testing.T) {
	// Error with empty message should be formatted (edge case)
	err := errors.NewStructuredError(
		errors.ErrorCodeUserInputInvalidFormat,
		"", // empty message
		nil,
		nil,
	)

	formatter := &mockJSONFormatter{}
	output := formatter.FormatError(err)

	assert.NotEmpty(t, output, "Should return output even with empty message")
	assert.Contains(t, output, `"error_code":"USER.INPUT.INVALID_FORMAT"`, "Error code should still be included")
}

func TestErrorFormatter_LongMessage(t *testing.T) {
	// Error with very long message should be formatted correctly
	longMessage := "This is a very long error message that contains multiple sentences and detailed information about what went wrong during workflow execution. " +
		"It includes information about the state machine, the current step, the inputs provided, and the expected behavior versus actual behavior. " +
		"This tests whether formatters can handle lengthy error descriptions without truncation or corruption."

	err := errors.NewStructuredError(
		errors.ErrorCodeExecutionParallelPartialFailure,
		longMessage,
		nil,
		nil,
	)

	humanFormatter := &mockHumanFormatter{}
	output := humanFormatter.FormatError(err)

	assert.Contains(t, output, longMessage, "Long message should be preserved in output")
	assert.Contains(t, output, "[EXECUTION.PARALLEL.PARTIAL_FAILURE]", "Error code should be included")
}

func TestErrorFormatter_WithCause(t *testing.T) {
	// Error with wrapped cause should be formatted (formatters may choose to include or exclude cause)
	cause := assert.AnError
	err := errors.NewStructuredError(
		errors.ErrorCodeSystemIOPermissionDenied,
		"permission denied",
		map[string]any{"file": "/etc/config.yaml"},
		cause,
	)

	formatter := &mockJSONFormatter{}
	output := formatter.FormatError(err)

	assert.NotEmpty(t, output, "Should format error with cause")
	assert.Equal(t, cause, formatter.lastError.Unwrap(), "Cause should be preserved in error")
}

func TestErrorFormatter_WithTimestamp(t *testing.T) {
	// Error timestamp should be accessible to formatter
	beforeTime := time.Now()
	err := errors.NewStructuredError(
		errors.ErrorCodeWorkflowParseUnknownField,
		"unknown field in YAML",
		nil,
		nil,
	)
	afterTime := time.Now()

	formatter := &mockJSONFormatter{}
	formatter.FormatError(err)

	assert.NotNil(t, formatter.lastError.Timestamp, "Timestamp should be set")
	assert.True(t, formatter.lastError.Timestamp.After(beforeTime.Add(-time.Second)), "Timestamp should be recent")
	assert.True(t, formatter.lastError.Timestamp.Before(afterTime.Add(time.Second)), "Timestamp should be recent")
}

func TestErrorFormatter_ComplexDetails(t *testing.T) {
	// Error with complex nested details
	err := errors.NewStructuredError(
		errors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"workflow_name": "deploy-pipeline",
			"state_name":    "review",
			"referenced_in": "approve-step",
			"line":          42,
			"metadata": map[string]any{
				"file":   "workflow.yaml",
				"author": "system",
			},
		},
		nil,
	)

	formatter := &mockJSONFormatter{}
	output := formatter.FormatError(err)

	assert.NotEmpty(t, output, "Should format error with complex details")
	assert.Len(t, formatter.lastError.Details, 5)
}

func TestErrorFormatter_EmptyOutput(t *testing.T) {
	// Edge case: formatter that returns empty string
	err := errors.NewStructuredError(
		errors.ErrorCodeExecutionCommandFailed,
		"command failed",
		nil,
		nil,
	)

	emptyFormatter := &mockEmptyFormatter{}
	output := emptyFormatter.FormatError(err)

	assert.Empty(t, output, "Empty formatter should return empty string")
}

func TestErrorFormatter_AllErrorCategories(t *testing.T) {
	// Verify formatter works with all error categories
	testCases := []struct {
		category string
		code     errors.ErrorCode
		message  string
	}{
		{"USER", errors.ErrorCodeUserInputMissingFile, "missing file"},
		{"WORKFLOW", errors.ErrorCodeWorkflowParseYAMLSyntax, "YAML syntax error"},
		{"EXECUTION", errors.ErrorCodeExecutionCommandTimeout, "timeout exceeded"},
		{"SYSTEM", errors.ErrorCodeSystemIOReadFailed, "read failed"},
	}

	formatter := &mockHumanFormatter{}

	for _, tc := range testCases {
		t.Run(tc.category, func(t *testing.T) {
			err := errors.NewStructuredError(tc.code, tc.message, nil, nil)
			output := formatter.FormatError(err)

			assert.NotEmpty(t, output, "Should format %s category error", tc.category)
			assert.Contains(t, output, tc.category, "Output should contain category name")
			assert.Contains(t, output, tc.message, "Output should contain message")
		})
	}
}

func TestErrorFormatter_ConcurrentAccess(t *testing.T) {
	// Test that formatters can be called concurrently (no shared state mutation)
	err1 := errors.NewStructuredError(
		errors.ErrorCodeUserInputMissingFile,
		"file 1 not found",
		nil,
		nil,
	)
	err2 := errors.NewStructuredError(
		errors.ErrorCodeWorkflowValidationCycleDetected,
		"cycle in workflow 2",
		nil,
		nil,
	)

	formatter := &mockJSONFormatter{}

	// Format errors in sequence
	output1 := formatter.FormatError(err1)
	output2 := formatter.FormatError(err2)

	assert.Contains(t, output1, "USER.INPUT.MISSING_FILE", "First error should be formatted correctly")
	assert.Contains(t, output2, "WORKFLOW.VALIDATION.CYCLE_DETECTED", "Second error should be formatted correctly")
	assert.NotEqual(t, output1, output2, "Different errors should produce different outputs")
}

func TestErrorFormatter_WithAllErrorTypes(t *testing.T) {
	// Test formatter with errors created via convenience constructors
	tests := []struct {
		name    string
		err     *errors.StructuredError
		wantCat string
	}{
		{
			name:    "NewUserError",
			err:     errors.NewUserError(errors.ErrorCodeUserInputValidationFailed, "validation failed", nil, nil),
			wantCat: "USER",
		},
		{
			name:    "NewWorkflowError",
			err:     errors.NewWorkflowError(errors.ErrorCodeWorkflowParseUnknownField, "unknown field", nil, nil),
			wantCat: "WORKFLOW",
		},
		{
			name:    "NewExecutionError",
			err:     errors.NewExecutionError(errors.ErrorCodeExecutionCommandFailed, "command failed", nil, nil),
			wantCat: "EXECUTION",
		},
		{
			name:    "NewSystemError",
			err:     errors.NewSystemError(errors.ErrorCodeSystemIOWriteFailed, "write failed", nil, nil),
			wantCat: "SYSTEM",
		},
	}

	formatter := &mockHumanFormatter{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatter.FormatError(tt.err)

			assert.NotEmpty(t, output, "Should format error from %s", tt.name)
			assert.Contains(t, output, tt.wantCat, "Output should contain category")
		})
	}
}
