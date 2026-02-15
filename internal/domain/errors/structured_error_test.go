package errors_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	domainerrors "github.com/awf-project/awf/internal/domain/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStructuredError_WithAllFields(t *testing.T) {
	// Given
	causeErr := errors.New("underlying error")
	details := map[string]any{
		"path":     "/workflow.yaml",
		"line":     42,
		"expected": "string",
	}

	// When
	before := time.Now()
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		details,
		causeErr,
	)
	after := time.Now()

	// Then
	assert.NotNil(t, err)
	assert.Equal(t, domainerrors.ErrorCodeUserInputMissingFile, err.Code)
	assert.Equal(t, "workflow file not found", err.Message)
	assert.Equal(t, details, err.Details)
	assert.Equal(t, causeErr, err.Cause)
	assert.True(t, err.Timestamp.After(before) || err.Timestamp.Equal(before))
	assert.True(t, err.Timestamp.Before(after) || err.Timestamp.Equal(after))
}

func TestNewStructuredError_WithNilDetails(t *testing.T) {
	// Given/When
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		nil,
		nil,
	)

	// Then
	assert.NotNil(t, err)
	assert.Nil(t, err.Details)
	assert.Nil(t, err.Cause)
	assert.NotZero(t, err.Timestamp)
}

func TestNewStructuredError_WithEmptyDetails(t *testing.T) {
	// Given/When
	emptyDetails := map[string]any{}
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command execution failed",
		emptyDetails,
		nil,
	)

	// Then
	assert.NotNil(t, err)
	assert.Empty(t, err.Details)
	assert.NotNil(t, err.Details) // Not nil, just empty
}

func TestNewUserError(t *testing.T) {
	// Given
	details := map[string]any{"field": "value"}
	cause := errors.New("cause")

	// When
	err := domainerrors.NewUserError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"validation failed",
		details,
		cause,
	)

	// Then
	require.NotNil(t, err)
	assert.Equal(t, domainerrors.ErrorCodeUserInputValidationFailed, err.Code)
	assert.Equal(t, "USER", err.Code.Category())
	assert.Equal(t, 1, err.ExitCode())
}

func TestNewWorkflowError(t *testing.T) {
	// Given
	err := domainerrors.NewWorkflowError(
		domainerrors.ErrorCodeWorkflowValidationCycleDetected,
		"cycle detected",
		nil,
		nil,
	)

	// Then
	require.NotNil(t, err)
	assert.Equal(t, "WORKFLOW", err.Code.Category())
	assert.Equal(t, 2, err.ExitCode())
}

func TestNewExecutionError(t *testing.T) {
	// Given
	err := domainerrors.NewExecutionError(
		domainerrors.ErrorCodeExecutionCommandTimeout,
		"command timed out",
		nil,
		nil,
	)

	// Then
	require.NotNil(t, err)
	assert.Equal(t, "EXECUTION", err.Code.Category())
	assert.Equal(t, 3, err.ExitCode())
}

func TestNewSystemError(t *testing.T) {
	// Given
	err := domainerrors.NewSystemError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"read failed",
		nil,
		nil,
	)

	// Then
	require.NotNil(t, err)
	assert.Equal(t, "SYSTEM", err.Code.Category())
	assert.Equal(t, 4, err.ExitCode())
}

func TestStructuredError_Error(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "simple message",
			message:  "workflow file not found",
			expected: "workflow file not found",
		},
		{
			name:     "message with special characters",
			message:  "invalid character '\\n' in YAML",
			expected: "invalid character '\\n' in YAML",
		},
		{
			name:     "empty message",
			message:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			err := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				tt.message,
				nil,
				nil,
			)

			// When
			result := err.Error()

			// Then
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStructuredError_Unwrap_WithCause(t *testing.T) {
	// Given
	causeErr := errors.New("underlying error")
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"failed to read file",
		nil,
		causeErr,
	)

	// When
	unwrapped := err.Unwrap()

	// Then
	assert.Equal(t, causeErr, unwrapped)
}

func TestStructuredError_Unwrap_WithoutCause(t *testing.T) {
	// Given
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		nil,
		nil,
	)

	// When
	unwrapped := err.Unwrap()

	// Then
	assert.Nil(t, unwrapped)
}

func TestStructuredError_ErrorsIs_WithCause(t *testing.T) {
	// Given
	causeErr := errors.New("sentinel error")
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		nil,
		causeErr,
	)

	// When/Then
	assert.True(t, errors.Is(err, causeErr))
}

func TestStructuredError_Is_SameCode(t *testing.T) {
	// Given
	err1 := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"message 1",
		nil,
		nil,
	)
	err2 := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"message 2",
		map[string]any{"different": "details"},
		nil,
	)

	// When/Then
	assert.True(t, err1.Is(err2))
	assert.True(t, err2.Is(err1))
}

func TestStructuredError_Is_DifferentCode(t *testing.T) {
	// Given
	err1 := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"message",
		nil,
		nil,
	)
	err2 := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputInvalidFormat,
		"message",
		nil,
		nil,
	)

	// When/Then
	assert.False(t, err1.Is(err2))
	assert.False(t, err2.Is(err1))
}

func TestStructuredError_Is_NotStructuredError(t *testing.T) {
	// Given
	err1 := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"message",
		nil,
		nil,
	)
	err2 := errors.New("plain error")

	// When/Then
	assert.False(t, err1.Is(err2))
}

func TestStructuredError_ErrorsIs_Integration(t *testing.T) {
	// Given
	targetErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationCycleDetected,
		"cycle detected",
		nil,
		nil,
	)
	actualErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationCycleDetected,
		"cycle detected in workflow",
		map[string]any{"workflow": "test.yaml"},
		nil,
	)

	// When/Then - errors.Is should use our Is method
	assert.True(t, errors.Is(actualErr, targetErr))
}

func TestStructuredError_As_StructuredError(t *testing.T) {
	// Given
	original := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{"exit_code": 1},
		nil,
	)

	// When
	var target *domainerrors.StructuredError
	ok := original.As(&target)

	// Then
	assert.True(t, ok)
	require.NotNil(t, target)
	assert.Equal(t, original, target)
	assert.Equal(t, original.Code, target.Code)
	assert.Equal(t, original.Message, target.Message)
}

func TestStructuredError_As_WrongType(t *testing.T) {
	// Given
	original := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		nil,
		nil,
	)

	// When
	var target error
	ok := original.As(&target)

	// Then
	assert.False(t, ok)
}

func TestStructuredError_ErrorsAs_Integration(t *testing.T) {
	// Given
	original := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOPermissionDenied,
		"permission denied",
		map[string]any{"path": "/etc/config"},
		nil,
	)

	// When
	var target *domainerrors.StructuredError
	ok := errors.As(original, &target)

	// Then
	assert.True(t, ok)
	require.NotNil(t, target)
	assert.Equal(t, original.Code, target.Code)
}

func TestStructuredError_ExitCode(t *testing.T) {
	tests := []struct {
		name         string
		code         domainerrors.ErrorCode
		expectedExit int
	}{
		{
			name:         "USER category returns 1",
			code:         domainerrors.ErrorCodeUserInputMissingFile,
			expectedExit: 1,
		},
		{
			name:         "WORKFLOW category returns 2",
			code:         domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
			expectedExit: 2,
		},
		{
			name:         "EXECUTION category returns 3",
			code:         domainerrors.ErrorCodeExecutionCommandFailed,
			expectedExit: 3,
		},
		{
			name:         "SYSTEM category returns 4",
			code:         domainerrors.ErrorCodeSystemIOReadFailed,
			expectedExit: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			err := domainerrors.NewStructuredError(
				tt.code,
				"test message",
				nil,
				nil,
			)

			// When
			exitCode := err.ExitCode()

			// Then
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}

func TestStructuredError_WithDetails_AddsNewDetails(t *testing.T) {
	// Given
	original := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"parse error",
		map[string]any{"line": 10},
		nil,
	)

	// When
	enhanced := original.WithDetails(map[string]any{"column": 5})

	// Then
	assert.NotEqual(t, original, enhanced) // New instance
	assert.Len(t, original.Details, 1)
	assert.Len(t, enhanced.Details, 2)
	assert.Equal(t, 10, enhanced.Details["line"])
	assert.Equal(t, 5, enhanced.Details["column"])
}

func TestStructuredError_WithDetails_OverwritesExistingKeys(t *testing.T) {
	// Given
	original := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"validation error",
		map[string]any{"field": "old_value", "other": "keep"},
		nil,
	)

	// When
	updated := original.WithDetails(map[string]any{"field": "new_value"})

	// Then
	assert.Equal(t, "old_value", original.Details["field"]) // Original unchanged
	assert.Equal(t, "new_value", updated.Details["field"])  // New value in copy
	assert.Equal(t, "keep", updated.Details["other"])       // Other keys preserved
}

func TestStructuredError_WithDetails_FromNilDetails(t *testing.T) {
	// Given
	original := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandTimeout,
		"timeout",
		nil,
		nil,
	)

	// When
	enhanced := original.WithDetails(map[string]any{"timeout_seconds": 30})

	// Then
	assert.Nil(t, original.Details)
	assert.Len(t, enhanced.Details, 1)
	assert.Equal(t, 30, enhanced.Details["timeout_seconds"])
}

func TestStructuredError_WithDetails_PreservesTimestamp(t *testing.T) {
	// Given
	original := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOWriteFailed,
		"write failed",
		nil,
		nil,
	)
	originalTime := original.Timestamp

	// Sleep to ensure time would differ if Timestamp was regenerated
	time.Sleep(2 * time.Millisecond)

	// When
	enhanced := original.WithDetails(map[string]any{"path": "/tmp/file"})

	// Then
	assert.Equal(t, originalTime, enhanced.Timestamp)
}

func TestStructuredError_WithDetails_PreservesOtherFields(t *testing.T) {
	// Given
	causeErr := errors.New("underlying")
	original := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"missing state",
		map[string]any{"state": "start"},
		causeErr,
	)

	// When
	enhanced := original.WithDetails(map[string]any{"workflow": "test.yaml"})

	// Then
	assert.Equal(t, original.Code, enhanced.Code)
	assert.Equal(t, original.Message, enhanced.Message)
	assert.Equal(t, original.Cause, enhanced.Cause)
}

func TestStructuredError_Format_SimpleString(t *testing.T) {
	// Given
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{"path": "/workflow.yaml"},
		nil,
	)

	// When/Then - %s and %v show message only
	assert.Equal(t, "workflow file not found", fmt.Sprintf("%s", err))
	assert.Equal(t, "workflow file not found", fmt.Sprintf("%v", err))
}

func TestStructuredError_Format_VerboseWithDetails(t *testing.T) {
	// Given
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{"line": 42, "column": 10},
		nil,
	)

	// When
	formatted := fmt.Sprintf("%+v", err)

	// Then - %+v shows code and details
	assert.Contains(t, formatted, "WORKFLOW.PARSE.YAML_SYNTAX")
	assert.Contains(t, formatted, "invalid YAML syntax")
	assert.Contains(t, formatted, "line=42")
	assert.Contains(t, formatted, "column=10")
}

func TestStructuredError_Format_VerboseWithoutDetails(t *testing.T) {
	// Given
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		nil,
		nil,
	)

	// When
	formatted := fmt.Sprintf("%+v", err)

	// Then
	assert.Equal(t, "EXECUTION.COMMAND.FAILED: command failed", formatted)
	assert.NotContains(t, formatted, "()")
}

func TestStructuredError_Format_VerboseWithEmptyDetails(t *testing.T) {
	// Given
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOPermissionDenied,
		"permission denied",
		map[string]any{},
		nil,
	)

	// When
	formatted := fmt.Sprintf("%+v", err)

	// Then
	assert.Equal(t, "SYSTEM.IO.PERMISSION_DENIED: permission denied", formatted)
	assert.NotContains(t, formatted, "()")
}

func TestStructuredError_Format_VerboseWithCause(t *testing.T) {
	// Given
	causeErr := errors.New("underlying IO error")
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"failed to read file",
		map[string]any{"path": "/etc/config"},
		causeErr,
	)

	// When
	formatted := fmt.Sprintf("%+v", err)

	// Then
	assert.Contains(t, formatted, "SYSTEM.IO.READ_FAILED")
	assert.Contains(t, formatted, "failed to read file")
	assert.Contains(t, formatted, "path=/etc/config")
	assert.Contains(t, formatted, "underlying IO error")
}

func TestStructuredError_EmptyMessage(t *testing.T) {
	// Given
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"",
		nil,
		nil,
	)

	// Then
	assert.Equal(t, "", err.Error())
}

func TestStructuredError_NilDetailsMapOperations(t *testing.T) {
	// Given
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationInvalidTransition,
		"invalid transition",
		nil,
		nil,
	)

	// When - Should not panic
	formatted := fmt.Sprintf("%+v", err)

	// Then
	assert.Contains(t, formatted, "invalid transition")
	assert.NotContains(t, formatted, "()")
}

func TestStructuredError_DetailsWithVariousTypes(t *testing.T) {
	// Given
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionParallelPartialFailure,
		"partial failure",
		map[string]any{
			"string":  "value",
			"int":     42,
			"float":   3.14,
			"bool":    true,
			"nil":     nil,
			"slice":   []string{"a", "b"},
			"map":     map[string]int{"x": 1},
			"complex": struct{ Name string }{"test"},
		},
		nil,
	)

	// When - Should not panic
	formatted := fmt.Sprintf("%+v", err)

	// Then
	assert.Contains(t, formatted, "EXECUTION.PARALLEL.PARTIAL_FAILURE")
	assert.Contains(t, formatted, "partial failure")
	// Details should be formatted somehow (not checking exact format)
}

func TestStructuredError_CauseChainingMultipleLevels(t *testing.T) {
	// Given
	root := errors.New("root cause")
	middle := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"read failed",
		nil,
		root,
	)
	top := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"parse failed",
		nil,
		middle,
	)

	// When/Then - errors.Is should traverse the chain
	assert.True(t, errors.Is(top, middle))
	assert.True(t, errors.Is(top, root))
}

func TestStructuredError_AsWithCauseChain(t *testing.T) {
	// Given
	innerErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"read failed",
		nil,
		nil,
	)
	outerErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"parse failed",
		nil,
		innerErr,
	)

	// When
	var target *domainerrors.StructuredError
	ok := errors.As(outerErr, &target)

	// Then - Should find the outer error first
	assert.True(t, ok)
	assert.Equal(t, domainerrors.ErrorCodeWorkflowParseYAMLSyntax, target.Code)

	// And - Should be able to find inner error through chain
	target = nil
	ok = errors.As(outerErr.Unwrap(), &target)
	assert.True(t, ok)
	assert.Equal(t, domainerrors.ErrorCodeSystemIOReadFailed, target.Code)
}

func TestStructuredError_TimestampIsRecent(t *testing.T) {
	// Given
	before := time.Now()

	// When
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test",
		nil,
		nil,
	)

	// Then
	after := time.Now()
	assert.True(t, !err.Timestamp.Before(before))
	assert.True(t, !err.Timestamp.After(after))
}

func TestStructuredError_TimestampIsUnique(t *testing.T) {
	// Given/When - Create multiple errors quickly
	err1 := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test1",
		nil,
		nil,
	)
	err2 := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test2",
		nil,
		nil,
	)

	// Then - Timestamps should be equal or err2 later (depends on clock resolution)
	assert.True(t, err2.Timestamp.Equal(err1.Timestamp) || err2.Timestamp.After(err1.Timestamp))
}
