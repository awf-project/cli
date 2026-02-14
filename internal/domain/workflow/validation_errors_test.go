package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationCodes_Exist(t *testing.T) {
	// Verify all validation codes are defined as non-empty strings
	codes := []struct {
		name     string
		code     ValidationCode
		expected string
	}{
		{"ErrCycleDetected", ErrCycleDetected, "cycle_detected"},
		{"ErrUnreachableState", ErrUnreachableState, "unreachable_state"},
		{"ErrInvalidTransition", ErrInvalidTransition, "invalid_transition"},
		{"ErrNoTerminalState", ErrNoTerminalState, "no_terminal_state"},
		{"ErrMissingInitialState", ErrMissingInitialState, "missing_initial_state"},
		{"ErrUndefinedInput", ErrUndefinedInput, "undefined_input"},
		{"ErrUndefinedStep", ErrUndefinedStep, "undefined_step"},
		{"ErrForwardReference", ErrForwardReference, "forward_reference"},
		{"ErrInvalidWorkflowProperty", ErrInvalidWorkflowProperty, "invalid_workflow_property"},
		{"ErrInvalidStateProperty", ErrInvalidStateProperty, "invalid_state_property"},
		{"ErrInvalidErrorProperty", ErrInvalidErrorProperty, "invalid_error_property"},
		{"ErrInvalidContextProperty", ErrInvalidContextProperty, "invalid_context_property"},
		{"ErrUnknownReferenceType", ErrUnknownReferenceType, "unknown_reference_type"},
		{"ErrErrorRefOutsideErrorHook", ErrErrorRefOutsideErrorHook, "error_ref_outside_error_hook"},
		{"ErrUndefinedLoopVariable", ErrUndefinedLoopVariable, "undefined_loop_variable"},
	}

	for _, tc := range codes {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tc.code))
			assert.Equal(t, tc.expected, string(tc.code))
		})
	}
}

func TestSubWorkflowValidationCodes_Exist(t *testing.T) {
	tests := []struct {
		name     string
		code     ValidationCode
		expected string
	}{
		{
			name:     "ErrCircularWorkflowCall",
			code:     ErrCircularWorkflowCall,
			expected: "circular_workflow_call",
		},
		{
			name:     "ErrUndefinedSubworkflow",
			code:     ErrUndefinedSubworkflow,
			expected: "undefined_subworkflow",
		},
		{
			name:     "ErrMaxNestingExceeded",
			code:     ErrMaxNestingExceeded,
			expected: "max_nesting_exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.code))
			assert.Equal(t, tt.expected, string(tt.code))
		})
	}
}

func TestSubWorkflowValidationCodes_UniqueValues(t *testing.T) {
	codes := []ValidationCode{
		ErrCircularWorkflowCall,
		ErrUndefinedSubworkflow,
		ErrMaxNestingExceeded,
	}

	seen := make(map[string]bool)
	for _, code := range codes {
		strCode := string(code)
		assert.False(t, seen[strCode], "duplicate validation code: %s", strCode)
		seen[strCode] = true
	}
}

func TestValidationError_CircularWorkflowCall(t *testing.T) {
	err := ValidationError{
		Level:   ValidationLevelError,
		Code:    ErrCircularWorkflowCall,
		Message: "workflow 'parent' calls 'child' which calls 'parent' creating a cycle",
		Path:    "states.call_child.call_workflow",
	}

	assert.True(t, err.IsError())
	assert.Equal(t, ErrCircularWorkflowCall, err.Code)
	assert.Contains(t, err.Error(), "error")
	assert.Contains(t, err.Error(), "states.call_child.call_workflow")
	assert.Contains(t, err.Error(), "cycle")
}

func TestValidationError_UndefinedSubworkflow(t *testing.T) {
	err := ValidationError{
		Level:   ValidationLevelError,
		Code:    ErrUndefinedSubworkflow,
		Message: "sub-workflow 'nonexistent-workflow' not found",
		Path:    "states.process.call_workflow.workflow",
	}

	assert.True(t, err.IsError())
	assert.Equal(t, ErrUndefinedSubworkflow, err.Code)
	assert.Contains(t, err.Error(), "error")
	assert.Contains(t, err.Error(), "nonexistent-workflow")
}

func TestValidationError_MaxNestingExceeded(t *testing.T) {
	err := ValidationError{
		Level:   ValidationLevelError,
		Code:    ErrMaxNestingExceeded,
		Message: "maximum sub-workflow nesting depth (10) exceeded: A -> B -> C -> D -> ... -> K",
		Path:    "states.deep_call.call_workflow",
	}

	assert.True(t, err.IsError())
	assert.Equal(t, ErrMaxNestingExceeded, err.Code)
	assert.Contains(t, err.Error(), "error")
	assert.Contains(t, err.Error(), "10")
}

func TestValidationError_SubWorkflow_WithoutPath(t *testing.T) {
	err := ValidationError{
		Level:   ValidationLevelError,
		Code:    ErrCircularWorkflowCall,
		Message: "circular dependency detected",
	}

	// Error without path should still format correctly
	errStr := err.Error()
	assert.Contains(t, errStr, "error")
	assert.Contains(t, errStr, "circular dependency detected")
	assert.NotContains(t, errStr, ":")
}

func TestValidationResult_AddError_SubWorkflowCodes(t *testing.T) {
	tests := []struct {
		name    string
		code    ValidationCode
		path    string
		message string
	}{
		{
			name:    "circular workflow call",
			code:    ErrCircularWorkflowCall,
			path:    "states.invoke.call_workflow",
			message: "workflow 'A' calls 'B' which calls 'A'",
		},
		{
			name:    "undefined subworkflow",
			code:    ErrUndefinedSubworkflow,
			path:    "states.process.call_workflow.workflow",
			message: "sub-workflow 'missing' not found",
		},
		{
			name:    "max nesting exceeded",
			code:    ErrMaxNestingExceeded,
			path:    "states.deep.call_workflow",
			message: "nesting depth 11 exceeds maximum 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{}
			result.AddError(tt.code, tt.path, tt.message)

			require.True(t, result.HasErrors())
			require.Len(t, result.Errors, 1)

			err := result.Errors[0]
			assert.Equal(t, ValidationLevelError, err.Level)
			assert.Equal(t, tt.code, err.Code)
			assert.Equal(t, tt.path, err.Path)
			assert.Equal(t, tt.message, err.Message)
		})
	}
}

func TestValidationResult_AddWarning_SubWorkflowCodes(t *testing.T) {
	// While these are typically errors, test that they can be added as warnings
	result := &ValidationResult{}
	result.AddWarning(ErrMaxNestingExceeded, "states.deep", "nesting depth 8 is close to maximum 10")

	require.True(t, result.HasWarnings())
	require.False(t, result.HasErrors())
	require.Len(t, result.Warnings, 1)

	warning := result.Warnings[0]
	assert.Equal(t, ValidationLevelWarning, warning.Level)
	assert.Equal(t, ErrMaxNestingExceeded, warning.Code)
}

func TestValidationResult_MultipleSubWorkflowErrors(t *testing.T) {
	result := &ValidationResult{}

	// Add multiple sub-workflow validation errors
	result.AddError(ErrCircularWorkflowCall, "states.call1", "cycle: A -> B -> A")
	result.AddError(ErrUndefinedSubworkflow, "states.call2", "sub-workflow 'missing' not found")
	result.AddError(ErrMaxNestingExceeded, "states.call3", "nesting depth exceeded")

	require.True(t, result.HasErrors())
	assert.Len(t, result.Errors, 3)

	// Verify each error is preserved
	codes := make(map[ValidationCode]bool)
	for _, err := range result.Errors {
		codes[err.Code] = true
	}
	assert.True(t, codes[ErrCircularWorkflowCall])
	assert.True(t, codes[ErrUndefinedSubworkflow])
	assert.True(t, codes[ErrMaxNestingExceeded])
}

func TestValidationResult_ToError_SubWorkflowCodes(t *testing.T) {
	t.Run("single sub-workflow error", func(t *testing.T) {
		result := &ValidationResult{}
		result.AddError(ErrCircularWorkflowCall, "states.invoke", "circular call detected")

		err := result.ToError()
		require.Error(t, err)

		// Single error should be returned directly
		valErr, ok := err.(ValidationError)
		require.True(t, ok)
		assert.Equal(t, ErrCircularWorkflowCall, valErr.Code)
	})

	t.Run("multiple sub-workflow errors", func(t *testing.T) {
		result := &ValidationResult{}
		result.AddError(ErrCircularWorkflowCall, "states.call1", "cycle detected")
		result.AddError(ErrUndefinedSubworkflow, "states.call2", "missing workflow")

		err := result.ToError()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "2 errors")
	})

	t.Run("no errors only warnings", func(t *testing.T) {
		result := &ValidationResult{}
		result.AddWarning(ErrMaxNestingExceeded, "states.deep", "close to max")

		err := result.ToError()
		assert.NoError(t, err) // Warnings don't cause error
	})
}

func TestValidationResult_AllIssues_IncludesSubWorkflowCodes(t *testing.T) {
	result := &ValidationResult{}
	result.AddError(ErrCircularWorkflowCall, "path1", "error message")
	result.AddWarning(ErrMaxNestingExceeded, "path2", "warning message")

	issues := result.AllIssues()
	require.Len(t, issues, 2)

	// Errors come first
	assert.Equal(t, ErrCircularWorkflowCall, issues[0].Code)
	assert.Equal(t, ErrMaxNestingExceeded, issues[1].Code)
}

func TestValidationLevel_Constants(t *testing.T) {
	assert.Equal(t, ValidationLevel("error"), ValidationLevelError)
	assert.Equal(t, ValidationLevel("warning"), ValidationLevelWarning)
}

func TestValidationError_IsError(t *testing.T) {
	tests := []struct {
		name     string
		level    ValidationLevel
		expected bool
	}{
		{"error level is error", ValidationLevelError, true},
		{"warning level is not error", ValidationLevelWarning, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidationError{
				Level:   tt.level,
				Code:    ErrCircularWorkflowCall,
				Message: "test",
			}
			assert.Equal(t, tt.expected, err.IsError())
		})
	}
}

func TestValidationError_EmptyMessage(t *testing.T) {
	err := ValidationError{
		Level: ValidationLevelError,
		Code:  ErrCircularWorkflowCall,
		Path:  "states.invoke",
	}

	errStr := err.Error()
	assert.Contains(t, errStr, "error")
	assert.Contains(t, errStr, "states.invoke")
}

func TestValidationError_EmptyPath(t *testing.T) {
	err := ValidationError{
		Level:   ValidationLevelError,
		Code:    ErrUndefinedSubworkflow,
		Message: "workflow not found",
	}

	errStr := err.Error()
	assert.Contains(t, errStr, "error")
	assert.Contains(t, errStr, "workflow not found")
	// Path separator shouldn't appear
	assert.NotContains(t, errStr, "]: :")
}

func TestValidationCode_CanBeUsedAsMapKey(t *testing.T) {
	codeMessages := map[ValidationCode]string{
		ErrCircularWorkflowCall: "Workflow contains circular calls",
		ErrUndefinedSubworkflow: "Referenced sub-workflow not found",
		ErrMaxNestingExceeded:   "Sub-workflow nesting too deep",
	}

	assert.Len(t, codeMessages, 3)
	assert.Equal(t, "Workflow contains circular calls", codeMessages[ErrCircularWorkflowCall])
	assert.Equal(t, "Referenced sub-workflow not found", codeMessages[ErrUndefinedSubworkflow])
	assert.Equal(t, "Sub-workflow nesting too deep", codeMessages[ErrMaxNestingExceeded])
}
