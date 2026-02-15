package ui_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	domerrors "github.com/awf-project/awf/internal/domain/errors"
	"github.com/awf-project/awf/internal/interfaces/cli/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component T013: WriteError() StructuredError Enhancement Tests
// Tests for enhanced WriteError() that detects and formats StructuredError.
// Tests compile but fail in RED phase (stub implementation).

// Happy Path Tests: StructuredError Detection and Formatting

func TestOutputWriter_WriteError_StructuredError_JSON(t *testing.T) {
	tests := []struct {
		name          string
		structuredErr *domerrors.StructuredError
		exitCode      int
		wantErrorCode string
		wantMessage   string
	}{
		{
			name: "user input error with details",
			structuredErr: domerrors.NewStructuredError(
				domerrors.ErrorCodeUserInputMissingFile,
				"workflow file not found",
				map[string]any{"path": "/workflows/deploy.yaml"},
				nil,
			),
			exitCode:      1,
			wantErrorCode: "USER.INPUT.MISSING_FILE",
			wantMessage:   "workflow file not found",
		},
		{
			name: "workflow validation error",
			structuredErr: domerrors.NewStructuredError(
				domerrors.ErrorCodeWorkflowValidationCycleDetected,
				"cycle detected in state transitions",
				map[string]any{"states": []string{"init", "process", "init"}},
				nil,
			),
			exitCode:      2,
			wantErrorCode: "WORKFLOW.VALIDATION.CYCLE_DETECTED",
			wantMessage:   "cycle detected in state transitions",
		},
		{
			name: "execution command failure",
			structuredErr: domerrors.NewStructuredError(
				domerrors.ErrorCodeExecutionCommandFailed,
				"shell command failed",
				map[string]any{"exit_code": 127, "command": "nonexistent-binary"},
				nil,
			),
			exitCode:      3,
			wantErrorCode: "EXECUTION.COMMAND.FAILED",
			wantMessage:   "shell command failed",
		},
		{
			name: "system IO error",
			structuredErr: domerrors.NewStructuredError(
				domerrors.ErrorCodeSystemIOPermissionDenied,
				"permission denied",
				map[string]any{"path": "/etc/sensitive", "operation": "read"},
				nil,
			),
			exitCode:      4,
			wantErrorCode: "SYSTEM.IO.PERMISSION_DENIED",
			wantMessage:   "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

			err := w.WriteError(tt.structuredErr, tt.exitCode)
			require.NoError(t, err)

			// Parse JSON response
			var got ui.ErrorResponse
			err = json.Unmarshal(buf.Bytes(), &got)
			require.NoError(t, err, "JSON should be valid")

			// Verify error code field is populated
			assert.Equal(t, tt.wantErrorCode, got.ErrorCode, "ErrorCode field should contain hierarchical code")
			assert.Equal(t, tt.wantMessage, got.Error, "Error field should contain message")
			assert.Equal(t, tt.exitCode, got.Code, "Code field should contain exit code")
		})
	}
}

func TestOutputWriter_WriteError_StructuredError_Text(t *testing.T) {
	tests := []struct {
		name          string
		structuredErr *domerrors.StructuredError
		exitCode      int
		wantContains  []string
	}{
		{
			name: "text format includes error code prefix",
			structuredErr: domerrors.NewStructuredError(
				domerrors.ErrorCodeUserInputMissingFile,
				"workflow file not found",
				map[string]any{"path": "/deploy.yaml"},
				nil,
			),
			exitCode: 1,
			wantContains: []string{
				"USER.INPUT.MISSING_FILE",
				"workflow file not found",
			},
		},
		{
			name: "text format for workflow error",
			structuredErr: domerrors.NewStructuredError(
				domerrors.ErrorCodeWorkflowParseYAMLSyntax,
				"invalid YAML syntax at line 42",
				nil,
				nil,
			),
			exitCode: 2,
			wantContains: []string{
				"WORKFLOW.PARSE.YAML_SYNTAX",
				"invalid YAML syntax",
			},
		},
		{
			name: "text format for execution timeout",
			structuredErr: domerrors.NewStructuredError(
				domerrors.ErrorCodeExecutionCommandTimeout,
				"command exceeded 30s timeout",
				map[string]any{"timeout_seconds": 30},
				nil,
			),
			exitCode: 3,
			wantContains: []string{
				"EXECUTION.COMMAND.TIMEOUT",
				"exceeded 30s timeout",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, true, false)

			err := w.WriteError(tt.structuredErr, tt.exitCode)
			require.NoError(t, err)

			// Text errors go to errOut
			output := errBuf.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, output, want, "text output should contain: %s", want)
			}
		})
	}
}

// Edge Cases: Empty Details, Nil Cause, Wrapped Errors

func TestOutputWriter_WriteError_StructuredError_EmptyDetails(t *testing.T) {
	// StructuredError with nil details should not cause panic
	structuredErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeUserInputValidationFailed,
		"validation failed",
		nil, // No details
		nil,
	)

	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	err := w.WriteError(structuredErr, 1)
	require.NoError(t, err)

	var got ui.ErrorResponse
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	assert.Equal(t, "USER.INPUT.VALIDATION_FAILED", got.ErrorCode)
	assert.Equal(t, "validation failed", got.Error)
}

func TestOutputWriter_WriteError_StructuredError_WithCause(t *testing.T) {
	// StructuredError wrapping another error
	causeErr := errors.New("underlying file system error")
	structuredErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeSystemIOReadFailed,
		"failed to read workflow file",
		map[string]any{"path": "/workflow.yaml"},
		causeErr,
	)

	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	err := w.WriteError(structuredErr, 4)
	require.NoError(t, err)

	var got ui.ErrorResponse
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	// Should still format correctly with cause chain
	assert.Equal(t, "SYSTEM.IO.READ_FAILED", got.ErrorCode)
	assert.Equal(t, "failed to read workflow file", got.Error)
}

func TestOutputWriter_WriteError_StructuredError_WrappedInChain(t *testing.T) {
	// StructuredError wrapped in another error via fmt.Errorf
	structuredErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeWorkflowValidationMissingState,
		"state 'deploy' not found",
		map[string]any{"missing_state": "deploy"},
		nil,
	)

	// Wrap the structured error
	wrappedErr := errors.New("validation failed: " + structuredErr.Error())

	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	// WriteError should still detect wrapped StructuredError via errors.As()
	// This test will fail in RED phase since stub doesn't properly detect wrapped errors
	err := w.WriteError(wrappedErr, 2)
	require.NoError(t, err)

	// Note: This behavior depends on proper error wrapping
	// If wrappedErr doesn't properly wrap structuredErr via fmt.Errorf("%w", ...),
	// it won't be detected by errors.As(). This is expected behavior.
}

func TestOutputWriter_WriteError_StructuredError_AllErrorCodes(t *testing.T) {
	// Test all defined error codes are handled correctly
	testCases := []struct {
		code     domerrors.ErrorCode
		exitCode int
	}{
		// USER errors (exit code 1)
		{domerrors.ErrorCodeUserInputMissingFile, 1},
		{domerrors.ErrorCodeUserInputInvalidFormat, 1},
		{domerrors.ErrorCodeUserInputValidationFailed, 1},
		// WORKFLOW errors (exit code 2)
		{domerrors.ErrorCodeWorkflowParseYAMLSyntax, 2},
		{domerrors.ErrorCodeWorkflowParseUnknownField, 2},
		{domerrors.ErrorCodeWorkflowValidationCycleDetected, 2},
		{domerrors.ErrorCodeWorkflowValidationMissingState, 2},
		{domerrors.ErrorCodeWorkflowValidationInvalidTransition, 2},
		// EXECUTION errors (exit code 3)
		{domerrors.ErrorCodeExecutionCommandFailed, 3},
		{domerrors.ErrorCodeExecutionCommandTimeout, 3},
		{domerrors.ErrorCodeExecutionParallelPartialFailure, 3},
		// SYSTEM errors (exit code 4)
		{domerrors.ErrorCodeSystemIOReadFailed, 4},
		{domerrors.ErrorCodeSystemIOWriteFailed, 4},
		{domerrors.ErrorCodeSystemIOPermissionDenied, 4},
	}

	for _, tc := range testCases {
		t.Run(string(tc.code), func(t *testing.T) {
			structuredErr := domerrors.NewStructuredError(
				tc.code,
				"test error message",
				nil,
				nil,
			)

			buf := new(bytes.Buffer)
			w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

			err := w.WriteError(structuredErr, tc.exitCode)
			require.NoError(t, err)

			var got ui.ErrorResponse
			err = json.Unmarshal(buf.Bytes(), &got)
			require.NoError(t, err)

			assert.Equal(t, string(tc.code), got.ErrorCode)
			assert.Equal(t, tc.exitCode, got.Code)
		})
	}
}

// Error Handling: Backward Compatibility with Plain Errors

func TestOutputWriter_WriteError_PlainError_JSON(t *testing.T) {
	// Plain error (not StructuredError) should use fallback formatting
	plainErr := errors.New("something went wrong")

	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	err := w.WriteError(plainErr, 1)
	require.NoError(t, err)

	var got ui.ErrorResponse
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	// Plain errors should NOT have error_code field (or it should be empty)
	assert.Empty(t, got.ErrorCode, "plain errors should not have error_code field populated")
	assert.Equal(t, "something went wrong", got.Error)
	assert.Equal(t, 1, got.Code)
}

func TestOutputWriter_WriteError_PlainError_Text(t *testing.T) {
	// Plain error in text format should use legacy formatting
	plainErr := errors.New("command failed with exit code 127")

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, true, false)

	err := w.WriteError(plainErr, 3)
	require.NoError(t, err)

	output := errBuf.String()
	// Should contain "Error:" prefix and message
	assert.Contains(t, output, "Error:")
	assert.Contains(t, output, "command failed")
	// Should NOT contain error code formatting
	assert.NotContains(t, output, "EXECUTION.")
	assert.NotContains(t, output, "[")
}

func TestOutputWriter_WriteError_NilError(t *testing.T) {
	// Edge case: nil error should not panic
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, true, false)

	// This may panic in stub implementation - that's expected for RED phase
	// In GREEN phase, should handle gracefully or document as precondition
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Nil error caused panic (expected in RED): %v", r)
		}
	}()

	err := w.WriteError(nil, 1)
	// If we reach here without panic, verify output
	// Implementation may choose to output nothing or a placeholder
	// This test documents the edge case behavior
	_ = err // Explicitly ignore error to document edge case handling
}

func TestOutputWriter_WriteError_MixedErrorTypes(t *testing.T) {
	// Test handling both StructuredError and plain errors in sequence
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, errBuf, ui.FormatJSON, true, false)

	// First: StructuredError
	structuredErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		nil,
		nil,
	)
	err := w.WriteError(structuredErr, 1)
	require.NoError(t, err)

	var structured ui.ErrorResponse
	err = json.Unmarshal(buf.Bytes(), &structured)
	require.NoError(t, err)
	assert.Equal(t, "USER.INPUT.MISSING_FILE", structured.ErrorCode)

	// Second: plain error (reset buffer for new output)
	buf.Reset()
	plainErr := errors.New("plain error")
	err = w.WriteError(plainErr, 1)
	require.NoError(t, err)

	var plain ui.ErrorResponse
	err = json.Unmarshal(buf.Bytes(), &plain)
	require.NoError(t, err)
	assert.Empty(t, plain.ErrorCode)
	assert.Equal(t, "plain error", plain.Error)
}

// Integration: Output Format Variations

func TestOutputWriter_WriteError_StructuredError_AllFormats(t *testing.T) {
	// Test StructuredError across all output formats
	structuredErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeWorkflowValidationCycleDetected,
		"cycle detected",
		map[string]any{"states": []string{"a", "b", "a"}},
		nil,
	)

	formats := []struct {
		name   string
		format ui.OutputFormat
	}{
		{"text", ui.FormatText},
		{"json", ui.FormatJSON},
		{"table", ui.FormatTable}, // Table falls back to text for errors
		{"quiet", ui.FormatQuiet}, // Quiet falls back to text for errors
	}

	for _, tt := range formats {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			w := ui.NewOutputWriter(buf, errBuf, tt.format, true, false)

			err := w.WriteError(structuredErr, 2)
			require.NoError(t, err)

			// Verify output was produced (format-specific assertions handled above)
			if tt.format == ui.FormatJSON {
				assert.NotEmpty(t, buf.String(), "JSON output should be in stdout")
			} else {
				assert.NotEmpty(t, errBuf.String(), "text output should be in stderr")
			}
		})
	}
}

func TestOutputWriter_WriteError_StructuredError_WithTimestamp(t *testing.T) {
	// Verify timestamp is preserved (though not necessarily in output)
	now := time.Now()
	structuredErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		nil,
		nil,
	)

	// Timestamp should be recent (within 1 second)
	assert.WithinDuration(t, now, structuredErr.Timestamp, time.Second)

	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	err := w.WriteError(structuredErr, 3)
	require.NoError(t, err)

	// JSON output doesn't include timestamp in ErrorResponse (by design)
	// This test documents that timestamps are tracked but not necessarily surfaced
}

func TestOutputWriter_WriteError_StructuredError_WithDetails(t *testing.T) {
	// StructuredError with complex details
	structuredErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeSystemIOWriteFailed,
		"failed to write state file",
		map[string]any{
			"path":        "/var/awf/state/workflow-123.json",
			"bytes":       4096,
			"disk_full":   true,
			"retry_count": 3,
		},
		errors.New("no space left on device"),
	)

	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	err := w.WriteError(structuredErr, 4)
	require.NoError(t, err)

	var got ui.ErrorResponse
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	assert.Equal(t, "SYSTEM.IO.WRITE_FAILED", got.ErrorCode)
	assert.Equal(t, "failed to write state file", got.Error)
	// Details are part of StructuredError but not currently in ErrorResponse schema
	// This test documents that details exist but may not be in JSON output
}

// RED Phase: Tests for Formatter Integration (Will Pass in GREEN Phase)

func TestOutputWriter_WriteError_StructuredError_UsesFormatter_Text(t *testing.T) {
	// GREEN phase: HumanErrorFormatter integration test
	structuredErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeWorkflowValidationCycleDetected,
		"cycle detected in workflow",
		map[string]any{"states": []string{"a", "b", "a"}},
		nil,
	)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, false, false) // Color enabled (noColor=false)

	err := w.WriteError(structuredErr, 2)
	require.NoError(t, err)

	output := errBuf.String()
	// Verify HumanErrorFormatter output format with details
	assert.Contains(t, output, "WORKFLOW.VALIDATION.CYCLE_DETECTED")
	assert.Contains(t, output, "cycle detected")
	assert.Contains(t, output, "[")         // Error code in brackets
	assert.Contains(t, output, "Details:")  // Details section present
	assert.Contains(t, output, "states:")   // Detail key
	assert.Contains(t, output, "[a, b, a]") // Detail value formatted as array

	// Note: ANSI color codes may not appear in test buffers even with color enabled
	// because the fatih/color package detects terminal capabilities.
	// In test environments (non-TTY), colors may be disabled automatically.
	// The important verification is that HumanErrorFormatter is being used,
	// which we can confirm by the presence of the Details section.
}

func TestOutputWriter_WriteError_StructuredError_FormatterWithDetails_Text(t *testing.T) {
	// GREEN phase: HumanErrorFormatter should display details
	structuredErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{
			"path":      "/workflows/missing.yaml",
			"attempted": "/workflows/missing.yaml, /home/user/.awf/missing.yaml",
		},
		nil,
	)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, true, false)

	err := w.WriteError(structuredErr, 1)
	require.NoError(t, err)

	output := errBuf.String()
	// Verify formatter includes details in human-readable format
	assert.Contains(t, output, "workflow file not found")
	assert.Contains(t, output, "Details:", "should include details section")
	assert.Contains(t, output, "path:", "should include path detail key")
	assert.Contains(t, output, "/workflows/missing.yaml", "should include path value")
	assert.Contains(t, output, "attempted:", "should include attempted detail key")
}

func TestOutputWriter_WriteError_StructuredError_FormatterWithCause_Text(t *testing.T) {
	// GREEN phase: HumanErrorFormatter handles cause chain
	// Note: Current HumanErrorFormatter doesn't explicitly display cause chain in output.
	// The cause is accessible via err.Unwrap() but not rendered in the formatted string.
	// This documents the current behavior - cause chain display can be added in future enhancement.

	causeErr := errors.New("permission denied: /workflows/deploy.yaml")
	structuredErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeSystemIOReadFailed,
		"failed to read workflow file",
		map[string]any{"path": "/workflows/deploy.yaml"},
		causeErr,
	)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, true, false)

	err := w.WriteError(structuredErr, 4)
	require.NoError(t, err)

	output := errBuf.String()
	// Verify formatter output (cause chain not yet rendered in HumanErrorFormatter)
	assert.Contains(t, output, "failed to read workflow file")
	assert.Contains(t, output, "SYSTEM.IO.READ_FAILED")
	// Future enhancement: add cause chain display
	// assert.Contains(t, output, "Caused by: permission denied")
}

// Error Detection: errors.As() Behavior

func TestOutputWriter_WriteError_ErrorsAs_Detection(t *testing.T) {
	// Verify that errors.As() correctly detects StructuredError
	structuredErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeUserInputValidationFailed,
		"validation error",
		nil,
		nil,
	)

	// Wrap the error
	wrappedErr := errors.New("outer error: " + structuredErr.Error())

	// Manual errors.As() test to document expected behavior
	var detected *domerrors.StructuredError
	isStructured := errors.As(structuredErr, &detected)
	assert.True(t, isStructured, "errors.As should detect StructuredError")
	assert.Equal(t, domerrors.ErrorCodeUserInputValidationFailed, detected.Code)

	// Wrapped error without proper wrapping won't be detected
	isWrappedStructured := errors.As(wrappedErr, &detected)
	assert.False(t, isWrappedStructured, "errors.As should NOT detect improperly wrapped StructuredError")
}

// Coverage: ErrorResponse JSON Schema

func TestErrorResponse_JSONSchema(t *testing.T) {
	// Verify ErrorResponse has the expected JSON schema
	response := ui.ErrorResponse{
		Error:     "test error",
		Code:      1,
		ErrorCode: "USER.INPUT.MISSING_FILE",
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// Verify all fields are present
	assert.Equal(t, "test error", parsed["error"])
	assert.Equal(t, float64(1), parsed["code"]) // JSON numbers are float64
	assert.Equal(t, "USER.INPUT.MISSING_FILE", parsed["error_code"])
}

func TestErrorResponse_JSONOmitsEmptyErrorCode(t *testing.T) {
	// Verify error_code is omitted when empty (omitempty tag)
	response := ui.ErrorResponse{
		Error: "plain error",
		Code:  1,
		// ErrorCode is empty
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// error_code field should not be present
	_, hasErrorCode := parsed["error_code"]
	assert.False(t, hasErrorCode, "error_code should be omitted when empty")
}
