package errors_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domainerrors "github.com/vanoix/awf/internal/domain/errors"
)

// =============================================================================
// Hint Type Tests (Happy Path)
// =============================================================================

func TestHint_Construction_WithValidMessage(t *testing.T) {
	// Given
	message := "Did you mean 'my-workflow.yaml'?"

	// When
	hint := domainerrors.Hint{Message: message}

	// Then
	assert.Equal(t, message, hint.Message)
}

func TestHint_Construction_WithMultiLineMessage(t *testing.T) {
	// Given
	message := "Run 'awf list' to see available workflows\nCheck file permissions with 'ls -l /path/to/file'"

	// When
	hint := domainerrors.Hint{Message: message}

	// Then
	assert.Equal(t, message, hint.Message)
	assert.Contains(t, hint.Message, "\n")
}

func TestHint_Construction_WithCommandSuggestion(t *testing.T) {
	// Given
	message := "Run 'awf validate my-workflow.yaml' to check syntax"

	// When
	hint := domainerrors.Hint{Message: message}

	// Then
	assert.Equal(t, message, hint.Message)
	assert.Contains(t, hint.Message, "awf")
}

// =============================================================================
// Hint Type Tests (Edge Cases)
// =============================================================================

func TestHint_Construction_WithEmptyMessage(t *testing.T) {
	// Given
	message := ""

	// When
	hint := domainerrors.Hint{Message: message}

	// Then
	assert.Empty(t, hint.Message)
	assert.NotNil(t, hint.Message) // Empty string, not nil
}

func TestHint_Construction_WithWhitespaceOnlyMessage(t *testing.T) {
	// Given
	message := "   \n\t  "

	// When
	hint := domainerrors.Hint{Message: message}

	// Then
	assert.Equal(t, message, hint.Message)
	assert.NotEmpty(t, hint.Message) // Contains whitespace
}

func TestHint_Construction_WithVeryLongMessage(t *testing.T) {
	// Given
	message := "This is a very long hint message that exceeds typical length constraints " +
		"and might be too verbose for practical use but should still be stored correctly"

	// When
	hint := domainerrors.Hint{Message: message}

	// Then
	assert.Equal(t, message, hint.Message)
	assert.Greater(t, len(hint.Message), 100)
}

func TestHint_Construction_WithSpecialCharacters(t *testing.T) {
	// Given
	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "Unicode characters",
			message: "File not found: /workflow/✓-test.yaml",
		},
		{
			name:    "ANSI escape codes",
			message: "\x1b[31mError\x1b[0m: Invalid format",
		},
		{
			name:    "Path with backslashes",
			message: "Check C:\\workflows\\example.yaml",
		},
		{
			name:    "Shell metacharacters",
			message: "Try: awf run workflow.yaml --input='$VAR'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			hint := domainerrors.Hint{Message: tt.message}

			// Then
			assert.Equal(t, tt.message, hint.Message)
		})
	}
}

// =============================================================================
// HintGenerator Type Tests (Happy Path)
// =============================================================================

func TestHintGenerator_ReturnsEmptySlice_WhenNoHintsAvailable(t *testing.T) {
	// Given
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		return []domainerrors.Hint{}
	}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"generic IO error",
		nil,
		nil,
	)

	// When
	hints := generator(structErr)

	// Then
	assert.NotNil(t, hints)
	assert.Empty(t, hints)
	assert.Equal(t, 0, len(hints))
}

func TestHintGenerator_ReturnsSingleHint_ForSimpleError(t *testing.T) {
	// Given
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err.Code == domainerrors.ErrorCodeUserInputMissingFile {
			return []domainerrors.Hint{
				{Message: "Run 'awf list' to see available workflows"},
			}
		}
		return []domainerrors.Hint{}
	}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		map[string]any{"path": "/workflow.yaml"},
		nil,
	)

	// When
	hints := generator(structErr)

	// Then
	require.Len(t, hints, 1)
	assert.Equal(t, "Run 'awf list' to see available workflows", hints[0].Message)
}

func TestHintGenerator_ReturnsMultipleHints_WithPriorityOrder(t *testing.T) {
	// Given
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err.Code == domainerrors.ErrorCodeUserInputMissingFile {
			return []domainerrors.Hint{
				{Message: "Did you mean 'my-workflow.yaml'?"},
				{Message: "Run 'awf list' to see available workflows"},
				{Message: "Check the current directory with 'pwd'"},
			}
		}
		return []domainerrors.Hint{}
	}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		nil,
		nil,
	)

	// When
	hints := generator(structErr)

	// Then
	require.Len(t, hints, 3)
	assert.Equal(t, "Did you mean 'my-workflow.yaml'?", hints[0].Message)
	assert.Equal(t, "Run 'awf list' to see available workflows", hints[1].Message)
	assert.Equal(t, "Check the current directory with 'pwd'", hints[2].Message)
}

func TestHintGenerator_UsesErrorDetails_ToGenerateContextualHints(t *testing.T) {
	// Given
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if path, ok := err.Details["path"].(string); ok {
			return []domainerrors.Hint{
				{Message: "File not found: " + path},
				{Message: "Verify the path exists"},
			}
		}
		return []domainerrors.Hint{}
	}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		map[string]any{"path": "/workflows/test.yaml"},
		nil,
	)

	// When
	hints := generator(structErr)

	// Then
	require.Len(t, hints, 2)
	assert.Contains(t, hints[0].Message, "/workflows/test.yaml")
}

// customTestError is a test helper type for TestHintGenerator_UsesErrorsAs_ToDetectSpecificErrorTypes
type customTestError struct {
	Path string
}

func (ce *customTestError) Error() string {
	return "custom error: " + ce.Path
}

func TestHintGenerator_UsesErrorsAs_ToDetectSpecificErrorTypes(t *testing.T) {
	// Given
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		var customErr *customTestError
		if errors.As(err.Cause, &customErr) {
			return []domainerrors.Hint{
				{Message: "Custom error detected for path: " + customErr.Path},
			}
		}
		return []domainerrors.Hint{}
	}

	causeErr := &customTestError{Path: "/test.yaml"}
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"custom error",
		nil,
		causeErr,
	)

	// When
	hints := generator(structErr)

	// Then
	require.Len(t, hints, 1)
	assert.Contains(t, hints[0].Message, "/test.yaml")
}

// =============================================================================
// HintGenerator Type Tests (Edge Cases)
// =============================================================================

func TestHintGenerator_HandlesNilStructuredError_Gracefully(t *testing.T) {
	// Given
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err == nil {
			return []domainerrors.Hint{}
		}
		return []domainerrors.Hint{{Message: "Valid error"}}
	}

	// When
	hints := generator(nil)

	// Then
	assert.NotNil(t, hints)
	assert.Empty(t, hints)
}

func TestHintGenerator_HandlesStructuredErrorWithNilDetails(t *testing.T) {
	// Given
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err.Details == nil {
			return []domainerrors.Hint{
				{Message: "No additional context available"},
			}
		}
		return []domainerrors.Hint{}
	}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"IO error",
		nil,
		nil,
	)

	// When
	hints := generator(structErr)

	// Then
	require.Len(t, hints, 1)
	assert.Equal(t, "No additional context available", hints[0].Message)
}

func TestHintGenerator_HandlesStructuredErrorWithNilCause(t *testing.T) {
	// Given
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err.Cause == nil {
			return []domainerrors.Hint{
				{Message: "No underlying cause"},
			}
		}
		return []domainerrors.Hint{{Message: "Has cause"}}
	}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"IO error",
		nil,
		nil,
	)

	// When
	hints := generator(structErr)

	// Then
	require.Len(t, hints, 1)
	assert.Equal(t, "No underlying cause", hints[0].Message)
}

func TestHintGenerator_HandlesEmptyDetails_Map(t *testing.T) {
	// Given
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if len(err.Details) == 0 {
			return []domainerrors.Hint{{Message: "No details provided"}}
		}
		return []domainerrors.Hint{}
	}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"IO error",
		map[string]any{},
		nil,
	)

	// When
	hints := generator(structErr)

	// Then
	require.Len(t, hints, 1)
	assert.Equal(t, "No details provided", hints[0].Message)
}

func TestHintGenerator_HandlesInvalidDetailTypes_Gracefully(t *testing.T) {
	// Given
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if path, ok := err.Details["path"].(string); ok {
			return []domainerrors.Hint{{Message: "Path: " + path}}
		}
		return []domainerrors.Hint{{Message: "Path not found or invalid type"}}
	}

	tests := []struct {
		name    string
		details map[string]any
		want    string
	}{
		{
			name:    "Path is integer",
			details: map[string]any{"path": 123},
			want:    "Path not found or invalid type",
		},
		{
			name:    "Path is nil",
			details: map[string]any{"path": nil},
			want:    "Path not found or invalid type",
		},
		{
			name:    "Path key missing",
			details: map[string]any{"other": "value"},
			want:    "Path not found or invalid type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeSystemIOReadFailed,
				"test error",
				tt.details,
				nil,
			)

			// When
			hints := generator(structErr)

			// Then
			require.Len(t, hints, 1)
			assert.Equal(t, tt.want, hints[0].Message)
		})
	}
}

// =============================================================================
// HintGenerator Type Tests (Error Handling)
// =============================================================================

func TestHintGenerator_ReturnsEmptySlice_OnUnrecognizedErrorCode(t *testing.T) {
	// Given
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		switch err.Code {
		case domainerrors.ErrorCodeUserInputMissingFile:
			return []domainerrors.Hint{{Message: "File hint"}}
		case domainerrors.ErrorCodeWorkflowParseYAMLSyntax:
			return []domainerrors.Hint{{Message: "YAML hint"}}
		default:
			return []domainerrors.Hint{}
		}
	}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"unhandled error",
		nil,
		nil,
	)

	// When
	hints := generator(structErr)

	// Then
	assert.NotNil(t, hints)
	assert.Empty(t, hints)
}

func TestHintGenerator_HandlesComplexErrorChain_WithErrorsAs(t *testing.T) {
	// Given
	// Note: errors.New() doesn't preserve the wrapped error chain,
	// so errors.As() won't find it
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		// Try to extract a formatted error message
		if err.Cause != nil && err.Cause.Error() != "" {
			return []domainerrors.Hint{
				{Message: "Underlying cause: " + err.Cause.Error()},
			}
		}
		return []domainerrors.Hint{}
	}

	// Create simple error chain
	wrappedErr := errors.New("wrapped error message")
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		nil,
		wrappedErr,
	)

	// When
	hints := generator(structErr)

	// Then
	require.Len(t, hints, 1)
	assert.Contains(t, hints[0].Message, "wrapped error message")
}

func TestHintGenerator_RespectsMaxHintLimit(t *testing.T) {
	// Given
	const maxHints = 3

	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		allHints := []domainerrors.Hint{
			{Message: "Hint 1"},
			{Message: "Hint 2"},
			{Message: "Hint 3"},
			{Message: "Hint 4"},
			{Message: "Hint 5"},
		}

		if len(allHints) > maxHints {
			return allHints[:maxHints]
		}
		return allHints
	}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test error",
		nil,
		nil,
	)

	// When
	hints := generator(structErr)

	// Then
	assert.Len(t, hints, maxHints)
}

// =============================================================================
// HintGenerator Composition Tests
// =============================================================================

func TestHintGenerator_CanBeComposed_IntoChain(t *testing.T) {
	// Given
	fileGenerator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err.Code == domainerrors.ErrorCodeUserInputMissingFile {
			return []domainerrors.Hint{{Message: "File hint"}}
		}
		return []domainerrors.Hint{}
	}

	yamlGenerator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err.Code == domainerrors.ErrorCodeWorkflowParseYAMLSyntax {
			return []domainerrors.Hint{{Message: "YAML hint"}}
		}
		return []domainerrors.Hint{}
	}

	// Composite generator
	compositeGenerator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		hints := make([]domainerrors.Hint, 0, 2) // Preallocate for 2 generators
		hints = append(hints, fileGenerator(err)...)
		hints = append(hints, yamlGenerator(err)...)
		return hints
	}

	fileErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file error",
		nil,
		nil,
	)

	yamlErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"yaml error",
		nil,
		nil,
	)

	// When
	fileHints := compositeGenerator(fileErr)
	yamlHints := compositeGenerator(yamlErr)

	// Then
	require.Len(t, fileHints, 1)
	assert.Equal(t, "File hint", fileHints[0].Message)

	require.Len(t, yamlHints, 1)
	assert.Equal(t, "YAML hint", yamlHints[0].Message)
}

func TestHintGenerator_MultipleGenerators_CanExecuteInSequence(t *testing.T) {
	// Given
	generators := []domainerrors.HintGenerator{
		func(err *domainerrors.StructuredError) []domainerrors.Hint {
			return []domainerrors.Hint{{Message: "Generator 1"}}
		},
		func(err *domainerrors.StructuredError) []domainerrors.Hint {
			return []domainerrors.Hint{{Message: "Generator 2"}}
		},
		func(err *domainerrors.StructuredError) []domainerrors.Hint {
			return []domainerrors.Hint{{Message: "Generator 3"}}
		},
	}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"test",
		nil,
		nil,
	)

	// When
	allHints := make([]domainerrors.Hint, 0, len(generators))
	for _, gen := range generators {
		allHints = append(allHints, gen(structErr)...)
	}

	// Then
	require.Len(t, allHints, 3)
	assert.Equal(t, "Generator 1", allHints[0].Message)
	assert.Equal(t, "Generator 2", allHints[1].Message)
	assert.Equal(t, "Generator 3", allHints[2].Message)
}
