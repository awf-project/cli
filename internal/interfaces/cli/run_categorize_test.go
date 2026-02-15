package cli

import (
	"errors"
	"fmt"
	"testing"

	domerrors "github.com/awf-project/awf/internal/domain/errors"
	"github.com/stretchr/testify/assert"
)

// TestCategorizeError_StructuredError tests the Phase 1 behavior:
// categorizeError should check for StructuredError first via errors.As()
// and use its ExitCode() method to determine the exit code.
func TestCategorizeError_StructuredError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name: "USER error code maps to exit code 1",
			err: domerrors.NewStructuredError(
				domerrors.ErrorCodeUserInputMissingFile,
				"workflow file not found",
				map[string]any{"path": "/nonexistent.yaml"},
				nil,
			),
			wantCode: ExitUser,
		},
		{
			name: "WORKFLOW error code maps to exit code 2",
			err: domerrors.NewStructuredError(
				domerrors.ErrorCodeWorkflowParseYAMLSyntax,
				"invalid YAML syntax",
				map[string]any{"line": 42},
				nil,
			),
			wantCode: ExitWorkflow,
		},
		{
			name: "EXECUTION error code maps to exit code 3",
			err: domerrors.NewStructuredError(
				domerrors.ErrorCodeExecutionCommandFailed,
				"command exited with non-zero status",
				map[string]any{"exit_code": 127},
				nil,
			),
			wantCode: ExitExecution,
		},
		{
			name: "SYSTEM error code maps to exit code 4",
			err: domerrors.NewStructuredError(
				domerrors.ErrorCodeSystemIOPermissionDenied,
				"permission denied",
				map[string]any{"path": "/etc/protected"},
				nil,
			),
			wantCode: ExitSystem,
		},
		{
			name: "wrapped StructuredError unwraps correctly",
			err: fmt.Errorf("context: %w", domerrors.NewStructuredError(
				domerrors.ErrorCodeUserInputInvalidFormat,
				"invalid file format",
				nil,
				nil,
			)),
			wantCode: ExitUser,
		},
		{
			name: "StructuredError with cause chain",
			err: domerrors.NewStructuredError(
				domerrors.ErrorCodeWorkflowValidationCycleDetected,
				"cycle detected in state machine",
				map[string]any{"states": []string{"A", "B", "A"}},
				errors.New("original validation error"),
			),
			wantCode: ExitWorkflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categorizeError(tt.err)
			assert.Equal(t, tt.wantCode, got, "categorizeError() should use StructuredError.ExitCode()")
		})
	}
}

// TestCategorizeError_StringFallback tests the Phase 2 behavior:
// categorizeError should fall back to legacy string matching when
// the error is not a StructuredError (backward compatibility).
func TestCategorizeError_StringFallback(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "not found string maps to ExitUser",
			err:      errors.New("workflow file not found"),
			wantCode: ExitUser,
		},
		{
			name:     "invalid string maps to ExitWorkflow",
			err:      errors.New("invalid YAML syntax at line 10"),
			wantCode: ExitWorkflow,
		},
		{
			name:     "timeout string maps to ExitExecution",
			err:      errors.New("operation timed out after 30s"),
			wantCode: ExitExecution,
		},
		{
			name:     "exit code string maps to ExitExecution",
			err:      errors.New("command exited with exit code 1"),
			wantCode: ExitExecution,
		},
		{
			name:     "permission string maps to ExitSystem",
			err:      errors.New("permission denied: /etc/hosts"),
			wantCode: ExitSystem,
		},
		{
			name:     "no match defaults to ExitExecution",
			err:      errors.New("unexpected error occurred"),
			wantCode: ExitExecution,
		},
		{
			name:     "wrapped error with not found",
			err:      fmt.Errorf("failed to load: %w", errors.New("config not found")),
			wantCode: ExitUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categorizeError(tt.err)
			assert.Equal(t, tt.wantCode, got, "categorizeError() should fall back to string matching")
		})
	}
}

// TestCategorizeError_EdgeCases tests edge cases and boundary conditions
// to ensure robust error handling.
func TestCategorizeError_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "empty error message defaults to ExitExecution",
			err:      errors.New(""),
			wantCode: ExitExecution,
		},
		{
			name:     "multiple keyword matches - not found wins",
			err:      errors.New("invalid file not found"),
			wantCode: ExitUser, // "not found" should match first
		},
		{
			name:     "case insensitive matching - NOT FOUND uppercase",
			err:      errors.New("File NOT FOUND"),
			wantCode: ExitExecution, // string matching is case-sensitive, should default
		},
		{
			name: "StructuredError takes precedence over string matching",
			err: domerrors.NewStructuredError(
				domerrors.ErrorCodeSystemIOReadFailed,
				"workflow file not found", // message contains "not found"
				nil,
				nil,
			),
			wantCode: ExitSystem, // Should use StructuredError.ExitCode(), not string fallback
		},
		{
			name: "deeply wrapped StructuredError",
			err: fmt.Errorf("layer 3: %w",
				fmt.Errorf("layer 2: %w",
					fmt.Errorf("layer 1: %w",
						domerrors.NewStructuredError(
							domerrors.ErrorCodeExecutionCommandTimeout,
							"command timeout",
							nil,
							nil,
						),
					),
				),
			),
			wantCode: ExitExecution,
		},
		{
			name: "StructuredError with all error code categories",
			err: domerrors.NewStructuredError(
				domerrors.ErrorCodeUserInputValidationFailed,
				"validation failed",
				nil,
				nil,
			),
			wantCode: ExitUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categorizeError(tt.err)
			assert.Equal(t, tt.wantCode, got, "categorizeError() edge case handling")
		})
	}
}

// TestCategorizeError_AllErrorCodes tests that categorizeError correctly
// maps ALL defined ErrorCode constants to their expected exit codes.
// This ensures completeness of the error taxonomy integration.
func TestCategorizeError_AllErrorCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     domerrors.ErrorCode
		wantCode int
	}{
		// USER category (exit code 1)
		{"USER.INPUT.MISSING_FILE", domerrors.ErrorCodeUserInputMissingFile, ExitUser},
		{"USER.INPUT.INVALID_FORMAT", domerrors.ErrorCodeUserInputInvalidFormat, ExitUser},
		{"USER.INPUT.VALIDATION_FAILED", domerrors.ErrorCodeUserInputValidationFailed, ExitUser},

		// WORKFLOW category (exit code 2)
		{"WORKFLOW.PARSE.YAML_SYNTAX", domerrors.ErrorCodeWorkflowParseYAMLSyntax, ExitWorkflow},
		{"WORKFLOW.PARSE.UNKNOWN_FIELD", domerrors.ErrorCodeWorkflowParseUnknownField, ExitWorkflow},
		{"WORKFLOW.VALIDATION.CYCLE_DETECTED", domerrors.ErrorCodeWorkflowValidationCycleDetected, ExitWorkflow},
		{"WORKFLOW.VALIDATION.MISSING_STATE", domerrors.ErrorCodeWorkflowValidationMissingState, ExitWorkflow},
		{"WORKFLOW.VALIDATION.INVALID_TRANSITION", domerrors.ErrorCodeWorkflowValidationInvalidTransition, ExitWorkflow},

		// EXECUTION category (exit code 3)
		{"EXECUTION.COMMAND.FAILED", domerrors.ErrorCodeExecutionCommandFailed, ExitExecution},
		{"EXECUTION.COMMAND.TIMEOUT", domerrors.ErrorCodeExecutionCommandTimeout, ExitExecution},
		{"EXECUTION.PARALLEL.PARTIAL_FAILURE", domerrors.ErrorCodeExecutionParallelPartialFailure, ExitExecution},

		// SYSTEM category (exit code 4)
		{"SYSTEM.IO.READ_FAILED", domerrors.ErrorCodeSystemIOReadFailed, ExitSystem},
		{"SYSTEM.IO.WRITE_FAILED", domerrors.ErrorCodeSystemIOWriteFailed, ExitSystem},
		{"SYSTEM.IO.PERMISSION_DENIED", domerrors.ErrorCodeSystemIOPermissionDenied, ExitSystem},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domerrors.NewStructuredError(
				tt.code,
				"test error message",
				nil,
				nil,
			)
			got := categorizeError(err)
			assert.Equal(t, tt.wantCode, got,
				"ErrorCode %s should map to exit code %d", tt.code, tt.wantCode)
		})
	}
}

// TestCategorizeError_BackwardCompatibility ensures that the enhanced
// categorizeError maintains backward compatibility with existing error
// handling behavior for non-StructuredError errors.
func TestCategorizeError_BackwardCompatibility(t *testing.T) {
	// This test documents the expected behavior before C047 enhancement.
	// After enhancement, these same errors should still categorize the same way.
	legacyErrors := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "legacy not found error",
			err:      errors.New("workflow not found"),
			wantCode: ExitUser,
		},
		{
			name:     "legacy validation error",
			err:      errors.New("invalid state reference"),
			wantCode: ExitWorkflow,
		},
		{
			name:     "legacy execution error",
			err:      errors.New("command timed out"),
			wantCode: ExitExecution,
		},
		{
			name:     "legacy system error",
			err:      errors.New("permission denied"),
			wantCode: ExitSystem,
		},
		{
			name:     "legacy unknown error",
			err:      errors.New("something went wrong"),
			wantCode: ExitExecution, // default
		},
	}

	for _, tt := range legacyErrors {
		t.Run(tt.name, func(t *testing.T) {
			got := categorizeError(tt.err)
			assert.Equal(t, tt.wantCode, got,
				"Legacy error categorization must remain unchanged for backward compatibility")
		})
	}
}

// TestCategorizeError_DualPath verifies that the two-phase approach
// (StructuredError first, string fallback second) works correctly
// and that StructuredError takes precedence.
func TestCategorizeError_DualPath(t *testing.T) {
	t.Run("StructuredError takes precedence over string matching", func(t *testing.T) {
		// Create a StructuredError with a message that would match string fallback
		// differently than the error code's actual category
		err := domerrors.NewStructuredError(
			domerrors.ErrorCodeSystemIOWriteFailed, // SYSTEM -> exit code 4
			"invalid file format",                  // message contains "invalid" -> would be exit code 2 in fallback
			nil,
			nil,
		)

		got := categorizeError(err)
		assert.Equal(t, ExitSystem, got,
			"StructuredError.ExitCode() should take precedence over string matching in message")
	})

	t.Run("string fallback activates when no StructuredError", func(t *testing.T) {
		// Plain error that should match string fallback
		err := errors.New("command timeout")

		got := categorizeError(err)
		assert.Equal(t, ExitExecution, got,
			"String fallback should activate for non-StructuredError errors")
	})

	t.Run("wrapped non-StructuredError uses string fallback", func(t *testing.T) {
		// Wrapped plain error should still use string matching on the full error string
		err := fmt.Errorf("failed to execute: %w", errors.New("permission denied"))

		got := categorizeError(err)
		assert.Equal(t, ExitSystem, got,
			"String fallback should work on wrapped plain errors")
	})
}

// TestCategorizeError_ErrorInterface tests that categorizeError
// properly handles the error interface and error chain traversal.
func TestCategorizeError_ErrorInterface(t *testing.T) {
	t.Run("nil error should not panic", func(t *testing.T) {
		// Note: In production, categorizeError would likely never receive nil,
		// but this tests defensive programming
		assert.NotPanics(t, func() {
			// This test will fail until implementation handles nil properly
			_ = categorizeError(nil)
		}, "categorizeError should handle nil error gracefully")
	})

	t.Run("error chain with multiple wrappers", func(t *testing.T) {
		baseErr := domerrors.NewStructuredError(
			domerrors.ErrorCodeWorkflowValidationMissingState,
			"state not found",
			nil,
			nil,
		)
		wrapped1 := fmt.Errorf("validation failed: %w", baseErr)
		wrapped2 := fmt.Errorf("workflow load error: %w", wrapped1)
		wrapped3 := fmt.Errorf("run command failed: %w", wrapped2)

		got := categorizeError(wrapped3)
		assert.Equal(t, ExitWorkflow, got,
			"Should traverse error chain to find StructuredError")
	})

	t.Run("StructuredError with cause chain", func(t *testing.T) {
		originalErr := errors.New("yaml parse error")
		structuredErr := domerrors.NewStructuredError(
			domerrors.ErrorCodeWorkflowParseYAMLSyntax,
			"failed to parse workflow",
			map[string]any{"line": 10},
			originalErr,
		)

		got := categorizeError(structuredErr)
		assert.Equal(t, ExitWorkflow, got,
			"StructuredError with Cause should still use its own ExitCode")
	})
}

// TestCategorizeError_ExitCodeConstants verifies that the exit code
// constants used match the documented values.
func TestCategorizeError_ExitCodeConstants(t *testing.T) {
	// Document expected exit code values
	assert.Equal(t, 1, ExitUser, "ExitUser should be 1")
	assert.Equal(t, 2, ExitWorkflow, "ExitWorkflow should be 2")
	assert.Equal(t, 3, ExitExecution, "ExitExecution should be 3")
	assert.Equal(t, 4, ExitSystem, "ExitSystem should be 4")
}
