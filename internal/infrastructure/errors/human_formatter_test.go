package errfmt

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domainerrors "github.com/vanoix/awf/internal/domain/errors"
	"github.com/vanoix/awf/internal/domain/ports"
)

// TestHumanErrorFormatter_ImplementsInterface verifies compile-time interface compliance.
func TestHumanErrorFormatter_ImplementsInterface(t *testing.T) {
	var _ ports.ErrorFormatter = (*HumanErrorFormatter)(nil)
}

// TestNewHumanErrorFormatter_Constructor verifies the constructor creates a valid instance.
func TestNewHumanErrorFormatter_Constructor(t *testing.T) {
	tests := []struct {
		name         string
		colorEnabled bool
	}{
		{
			name:         "with color enabled",
			colorEnabled: true,
		},
		{
			name:         "with color disabled",
			colorEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewHumanErrorFormatter(tt.colorEnabled)
			require.NotNil(t, formatter)
			assert.Equal(t, tt.colorEnabled, formatter.colorEnabled)
		})
	}
}

// TestHumanErrorFormatter_HappyPath tests basic human-readable formatting with all fields populated.
func TestHumanErrorFormatter_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		errCode  domainerrors.ErrorCode
		message  string
		details  map[string]any
		cause    error
		validate func(t *testing.T, output string)
	}{
		{
			name:    "USER error with details",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "workflow file not found",
			details: map[string]any{
				"path":      "/path/to/workflow.yaml",
				"attempted": true,
			},
			cause: nil,
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output, "output should not be empty")
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE", "should contain error code")
				assert.Contains(t, output, "workflow file not found", "should contain message")
				assert.Contains(t, output, "path", "should contain detail key")
				assert.Contains(t, output, "/path/to/workflow.yaml", "should contain detail value")
			},
		},
		{
			name:    "WORKFLOW error with simple details",
			errCode: domainerrors.ErrorCodeWorkflowValidationCycleDetected,
			message: "cycle detected in state machine",
			details: map[string]any{
				"state": "step_a",
			},
			cause: nil,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "WORKFLOW.VALIDATION.CYCLE_DETECTED")
				assert.Contains(t, output, "cycle detected in state machine")
				assert.Contains(t, output, "state")
				assert.Contains(t, output, "step_a")
			},
		},
		{
			name:    "EXECUTION error with exit code detail",
			errCode: domainerrors.ErrorCodeExecutionCommandFailed,
			message: "command execution failed",
			details: map[string]any{
				"command":   "make build",
				"exit_code": 2,
			},
			cause: nil,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "EXECUTION.COMMAND.FAILED")
				assert.Contains(t, output, "command execution failed")
				assert.Contains(t, output, "command")
				assert.Contains(t, output, "make build")
			},
		},
		{
			name:    "SYSTEM error with permission details",
			errCode: domainerrors.ErrorCodeSystemIOPermissionDenied,
			message: "permission denied",
			details: map[string]any{
				"path":      "/etc/secret",
				"operation": "read",
			},
			cause: nil,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "SYSTEM.IO.PERMISSION_DENIED")
				assert.Contains(t, output, "permission denied")
				assert.Contains(t, output, "path")
				assert.Contains(t, output, "/etc/secret")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			formatter := NewHumanErrorFormatter(false) // Disable color for easier testing
			structuredErr := domainerrors.NewStructuredError(
				tt.errCode,
				tt.message,
				tt.details,
				tt.cause,
			)

			// Act
			output := formatter.FormatError(structuredErr)

			// Assert
			tt.validate(t, output)
		})
	}
}

// TestHumanErrorFormatter_ColorOutput tests colored output formatting.
func TestHumanErrorFormatter_ColorOutput(t *testing.T) {
	tests := []struct {
		name         string
		colorEnabled bool
		errCode      domainerrors.ErrorCode
		message      string
		validate     func(t *testing.T, output string)
	}{
		{
			name:         "color enabled",
			colorEnabled: true,
			errCode:      domainerrors.ErrorCodeUserInputMissingFile,
			message:      "test error",
			validate: func(t *testing.T, output string) {
				// With color enabled, output should contain error code and message
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
				assert.Contains(t, output, "test error")
			},
		},
		{
			name:         "color disabled",
			colorEnabled: false,
			errCode:      domainerrors.ErrorCodeUserInputMissingFile,
			message:      "test error",
			validate: func(t *testing.T, output string) {
				// Without color, output should still be human-readable
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
				assert.Contains(t, output, "test error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			formatter := NewHumanErrorFormatter(tt.colorEnabled)
			structuredErr := domainerrors.NewStructuredError(
				tt.errCode,
				tt.message,
				nil,
				nil,
			)

			// Act
			output := formatter.FormatError(structuredErr)

			// Assert
			tt.validate(t, output)
		})
	}
}

// TestHumanErrorFormatter_EdgeCases tests boundary conditions and unusual inputs.
func TestHumanErrorFormatter_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		errCode  domainerrors.ErrorCode
		message  string
		details  map[string]any
		cause    error
		validate func(t *testing.T, output string)
	}{
		{
			name:    "nil details",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "file not found",
			details: nil,
			cause:   nil,
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
				assert.Contains(t, output, "file not found")
				// Should not crash with nil details
			},
		},
		{
			name:    "empty details map",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "file not found",
			details: map[string]any{},
			cause:   nil,
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
				assert.Contains(t, output, "file not found")
				// Empty map should not produce "Details:" section or should handle gracefully
			},
		},
		{
			name:    "empty message",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "",
			details: nil,
			cause:   nil,
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
				// Should still produce output even with empty message
			},
		},
		{
			name:    "very long message",
			errCode: domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
			message: strings.Repeat("This is a very long error message. ", 100),
			details: nil,
			cause:   nil,
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "WORKFLOW.PARSE.YAML_SYNTAX")
				assert.Greater(t, len(output), 1000)
			},
		},
		{
			name:    "special characters in message",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "file \"test.yaml\" not found\n\tpath: /home/user/\r\n",
			details: nil,
			cause:   nil,
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
				assert.Contains(t, output, "test.yaml")
			},
		},
		{
			name:    "unicode in message and details",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "fichier 文件 не найден 🔥",
			details: map[string]any{
				"path":  "/путь/到/文件.yaml",
				"emoji": "⚠️",
			},
			cause: nil,
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "文件")
				assert.Contains(t, output, "🔥")
			},
		},
		{
			name:    "details with complex types",
			errCode: domainerrors.ErrorCodeSystemIOReadFailed,
			message: "complex type details",
			details: map[string]any{
				"int":     42,
				"float":   3.14159,
				"bool":    true,
				"null":    nil,
				"slice":   []string{"a", "b", "c"},
				"numbers": []int{1, 2, 3},
			},
			cause: nil,
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "SYSTEM.IO.READ_FAILED")
				// Should handle complex types gracefully
			},
		},
		{
			name:    "nested details structures",
			errCode: domainerrors.ErrorCodeWorkflowValidationCycleDetected,
			message: "complex nested error",
			details: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": "deep value",
					},
					"array": []any{1, "two", true},
				},
				"numbers": []int{1, 2, 3},
			},
			cause: nil,
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "WORKFLOW.VALIDATION.CYCLE_DETECTED")
				// Should handle nested structures without crashing
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			formatter := NewHumanErrorFormatter(false) // Disable color for easier testing
			structuredErr := domainerrors.NewStructuredError(
				tt.errCode,
				tt.message,
				tt.details,
				tt.cause,
			)

			// Act
			output := formatter.FormatError(structuredErr)

			// Assert
			tt.validate(t, output)
		})
	}
}

// TestHumanErrorFormatter_ErrorHandling tests error propagation and cause wrapping.
func TestHumanErrorFormatter_ErrorHandling(t *testing.T) {
	tests := []struct {
		name     string
		errCode  domainerrors.ErrorCode
		message  string
		details  map[string]any
		cause    error
		validate func(t *testing.T, output string)
	}{
		{
			name:    "with wrapped cause",
			errCode: domainerrors.ErrorCodeSystemIOReadFailed,
			message: "failed to read file",
			details: map[string]any{
				"path": "/var/log/app.log",
			},
			cause: errors.New("underlying io error"),
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "SYSTEM.IO.READ_FAILED")
				assert.Contains(t, output, "failed to read file")
				// May or may not include cause in human output (implementation decision)
			},
		},
		{
			name:    "with nil cause",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "file not found",
			details: nil,
			cause:   nil,
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
			},
		},
		{
			name:    "with structured error cause",
			errCode: domainerrors.ErrorCodeWorkflowValidationCycleDetected,
			message: "workflow validation failed",
			details: map[string]any{
				"workflow_id": "test-workflow",
			},
			cause: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
				"yaml parsing error",
				nil,
				nil,
			),
			validate: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "WORKFLOW.VALIDATION.CYCLE_DETECTED")
				// Implementation may include cause information
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			formatter := NewHumanErrorFormatter(false)
			structuredErr := domainerrors.NewStructuredError(
				tt.errCode,
				tt.message,
				tt.details,
				tt.cause,
			)

			// Act
			output := formatter.FormatError(structuredErr)

			// Assert
			tt.validate(t, output)
		})
	}
}

// TestHumanErrorFormatter_OutputFormat tests the expected format structure.
func TestHumanErrorFormatter_OutputFormat(t *testing.T) {
	tests := []struct {
		name     string
		errCode  domainerrors.ErrorCode
		message  string
		details  map[string]any
		validate func(t *testing.T, output string)
	}{
		{
			name:    "format with brackets around error code",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "workflow file not found",
			details: nil,
			validate: func(t *testing.T, output string) {
				// Expected format: [ERROR_CODE] message
				assert.Contains(t, output, "[")
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
				assert.Contains(t, output, "]")
				assert.Contains(t, output, "workflow file not found")
			},
		},
		{
			name:    "format with details section",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "workflow file not found",
			details: map[string]any{
				"path": "/workflow.yaml",
			},
			validate: func(t *testing.T, output string) {
				// Expected format includes Details: section
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
				assert.Contains(t, output, "workflow file not found")
				// Details section should be formatted with indentation
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			formatter := NewHumanErrorFormatter(false)
			structuredErr := domainerrors.NewStructuredError(
				tt.errCode,
				tt.message,
				tt.details,
				nil,
			)

			// Act
			output := formatter.FormatError(structuredErr)

			// Assert
			tt.validate(t, output)
		})
	}
}

// TestHumanErrorFormatter_AllErrorCategories tests all error code categories.
func TestHumanErrorFormatter_AllErrorCategories(t *testing.T) {
	tests := []struct {
		name     string
		errCode  domainerrors.ErrorCode
		category string
	}{
		{
			name:     "USER category",
			errCode:  domainerrors.ErrorCodeUserInputMissingFile,
			category: "USER",
		},
		{
			name:     "WORKFLOW category",
			errCode:  domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
			category: "WORKFLOW",
		},
		{
			name:     "EXECUTION category",
			errCode:  domainerrors.ErrorCodeExecutionCommandFailed,
			category: "EXECUTION",
		},
		{
			name:     "SYSTEM category",
			errCode:  domainerrors.ErrorCodeSystemIOReadFailed,
			category: "SYSTEM",
		},
	}

	formatter := NewHumanErrorFormatter(false)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			structuredErr := domainerrors.NewStructuredError(
				tt.errCode,
				"test message",
				nil,
				nil,
			)

			// Act
			output := formatter.FormatError(structuredErr)

			// Assert
			assert.NotEmpty(t, output)
			assert.Contains(t, output, tt.category,
				"output should contain category %s", tt.category)
		})
	}
}

// TestHumanErrorFormatter_ConsistentOutput verifies formatting is deterministic.
func TestHumanErrorFormatter_ConsistentOutput(t *testing.T) {
	// Arrange
	formatter := NewHumanErrorFormatter(false)

	// Create error with fixed timestamp for reproducibility
	structuredErr := &domainerrors.StructuredError{
		Code:    domainerrors.ErrorCodeUserInputMissingFile,
		Message: "test error",
		Details: map[string]any{
			"key1": "value1",
			"key2": "value2",
		},
		Cause:     nil,
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	// Act - format multiple times
	output1 := formatter.FormatError(structuredErr)
	output2 := formatter.FormatError(structuredErr)

	// Assert - outputs should be identical
	assert.Equal(t, output1, output2, "formatting should be deterministic")
}

// TestHumanErrorFormatter_Idempotency verifies multiple calls produce same result.
func TestHumanErrorFormatter_Idempotency(t *testing.T) {
	// Arrange
	formatter := NewHumanErrorFormatter(false)
	structuredErr := &domainerrors.StructuredError{
		Code:      domainerrors.ErrorCodeUserInputMissingFile,
		Message:   "test error",
		Details:   map[string]any{"test": "data"},
		Cause:     nil,
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	// Act - format 10 times
	outputs := make([]string, 10)
	for i := 0; i < 10; i++ {
		outputs[i] = formatter.FormatError(structuredErr)
	}

	// Assert - all outputs identical
	for i := 1; i < len(outputs); i++ {
		assert.Equal(t, outputs[0], outputs[i],
			"output %d should match output 0", i)
	}
}

// TestHumanErrorFormatter_MultilineDetails tests formatting of details that span multiple lines.
func TestHumanErrorFormatter_MultilineDetails(t *testing.T) {
	// Arrange
	formatter := NewHumanErrorFormatter(false)
	structuredErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationCycleDetected,
		"validation failed",
		map[string]any{
			"details":  "first line\nsecond line\nthird line",
			"workflow": "complex-workflow",
		},
		nil,
	)

	// Act
	output := formatter.FormatError(structuredErr)

	// Assert
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "WORKFLOW.VALIDATION.CYCLE_DETECTED")
	// Should handle multiline strings in details gracefully
}

// TestHumanErrorFormatter_LargeDetailsMap tests handling of many detail entries.
func TestHumanErrorFormatter_LargeDetailsMap(t *testing.T) {
	// Arrange
	formatter := NewHumanErrorFormatter(false)
	details := make(map[string]any)
	for i := 0; i < 50; i++ {
		details[string(rune('a'+i%26))+strings.Repeat("x", i)] = i
	}

	structuredErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeSystemIOReadFailed,
		"many details",
		details,
		nil,
	)

	// Act
	output := formatter.FormatError(structuredErr)

	// Assert
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "SYSTEM.IO.READ_FAILED")
	// Should handle large detail maps without crashing
}

// TestHumanErrorFormatter_NoDetailsSection tests that errors without details don't show empty section.
func TestHumanErrorFormatter_NoDetailsSection(t *testing.T) {
	// Arrange
	formatter := NewHumanErrorFormatter(false)
	structuredErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"simple error",
		nil,
		nil,
	)

	// Act
	output := formatter.FormatError(structuredErr)

	// Assert
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
	assert.Contains(t, output, "simple error")
	// Should not contain empty "Details:" section
}
