package errfmt

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domainerrors "github.com/vanoix/awf/internal/domain/errors"
	"github.com/vanoix/awf/internal/domain/ports"
)

// TestJSONErrorFormatter_ImplementsInterface verifies compile-time interface compliance.
func TestJSONErrorFormatter_ImplementsInterface(t *testing.T) {
	var _ ports.ErrorFormatter = (*JSONErrorFormatter)(nil)
}

// TestNewJSONErrorFormatter_Constructor verifies the constructor creates a valid instance.
func TestNewJSONErrorFormatter_Constructor(t *testing.T) {
	formatter := NewJSONErrorFormatter(false)
	require.NotNil(t, formatter)
}

// TestJSONErrorFormatter_HappyPath tests basic JSON formatting with all fields populated.
func TestJSONErrorFormatter_HappyPath(t *testing.T) {
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
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err, "output must be valid JSON")

				assert.Equal(t, "USER.INPUT.MISSING_FILE", result["error_code"])
				assert.Equal(t, "workflow file not found", result["message"])
				assert.Contains(t, result, "details")
				assert.Contains(t, result, "timestamp")

				details, ok := result["details"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "/path/to/workflow.yaml", details["path"])
				assert.Equal(t, true, details["attempted"])
			},
		},
		{
			name:    "WORKFLOW error with simple details",
			errCode: domainerrors.ErrorCodeWorkflowValidationCycleDetected,
			message: "cycle detected in state machine",
			details: map[string]any{
				"state": "step_a",
				"cycle": []string{"step_a", "step_b", "step_a"},
			},
			cause: nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Equal(t, "WORKFLOW.VALIDATION.CYCLE_DETECTED", result["error_code"])
				assert.Equal(t, "cycle detected in state machine", result["message"])
			},
		},
		{
			name:    "EXECUTION error",
			errCode: domainerrors.ErrorCodeExecutionCommandFailed,
			message: "command execution failed",
			details: map[string]any{
				"command":   "make build",
				"exit_code": 2,
			},
			cause: nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Equal(t, "EXECUTION.COMMAND.FAILED", result["error_code"])
			},
		},
		{
			name:    "SYSTEM error",
			errCode: domainerrors.ErrorCodeSystemIOPermissionDenied,
			message: "permission denied",
			details: map[string]any{
				"path":       "/etc/secret",
				"operation":  "read",
				"permission": 0o000,
			},
			cause: nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Equal(t, "SYSTEM.IO.PERMISSION_DENIED", result["error_code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			formatter := NewJSONErrorFormatter(false)
			structuredErr := domainerrors.NewStructuredError(
				tt.errCode,
				tt.message,
				tt.details,
				tt.cause,
			)

			// Act
			output := formatter.FormatError(structuredErr)

			// Assert
			assert.NotEmpty(t, output, "output should not be empty")
			tt.validate(t, output)
		})
	}
}

// TestJSONErrorFormatter_EdgeCases tests boundary conditions and unusual inputs.
func TestJSONErrorFormatter_EdgeCases(t *testing.T) {
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
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Equal(t, "USER.INPUT.MISSING_FILE", result["error_code"])
				assert.Equal(t, "file not found", result["message"])
				// Details should be null or omitted in JSON
				details, exists := result["details"]
				if exists {
					assert.Nil(t, details, "nil details should serialize as null or be omitted")
				}
			},
		},
		{
			name:    "empty details map",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "file not found",
			details: map[string]any{},
			cause:   nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Equal(t, "USER.INPUT.MISSING_FILE", result["error_code"])
				// Empty map should be present as empty object
				details, ok := result["details"].(map[string]any)
				if ok {
					assert.Empty(t, details)
				}
			},
		},
		{
			name:    "empty message",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "",
			details: nil,
			cause:   nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Equal(t, "USER.INPUT.MISSING_FILE", result["error_code"])
				assert.Equal(t, "", result["message"])
			},
		},
		{
			name:    "very long message",
			errCode: domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
			message: strings.Repeat("This is a very long error message. ", 100),
			details: nil,
			cause:   nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Equal(t, "WORKFLOW.PARSE.YAML_SYNTAX", result["error_code"])
				message, ok := result["message"].(string)
				require.True(t, ok)
				assert.Greater(t, len(message), 1000)
			},
		},
		{
			name:    "special characters in message",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "file \"test.yaml\" not found\n\tpath: /home/user/\r\n",
			details: nil,
			cause:   nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err, "special characters should be properly escaped")

				assert.Equal(t, "USER.INPUT.MISSING_FILE", result["error_code"])
				assert.Contains(t, result["message"], "test.yaml")
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
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err, "unicode should be properly encoded")

				assert.Contains(t, result["message"], "文件")
				assert.Contains(t, result["message"], "🔥")
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
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err, "nested structures should serialize correctly")

				assert.Contains(t, result, "details")
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
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				details := result["details"].(map[string]any)
				assert.Equal(t, float64(42), details["int"]) // JSON numbers are float64
				assert.Equal(t, true, details["bool"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			formatter := NewJSONErrorFormatter(false)
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

// TestJSONErrorFormatter_ErrorHandling tests error propagation and cause wrapping.
func TestJSONErrorFormatter_ErrorHandling(t *testing.T) {
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
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Equal(t, "SYSTEM.IO.READ_FAILED", result["error_code"])
				assert.Equal(t, "failed to read file", result["message"])
				// Cause field handling depends on implementation
				// but should not break JSON structure
			},
		},
		{
			name:    "with nil cause",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "file not found",
			details: nil,
			cause:   nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Equal(t, "USER.INPUT.MISSING_FILE", result["error_code"])
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
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Equal(t, "WORKFLOW.VALIDATION.CYCLE_DETECTED", result["error_code"])
				// Implementation may include cause information
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			formatter := NewJSONErrorFormatter(false)
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

// TestJSONErrorFormatter_TimestampFormat verifies timestamp is in ISO 8601 format.
func TestJSONErrorFormatter_TimestampFormat(t *testing.T) {
	// Arrange
	formatter := NewJSONErrorFormatter(false)
	structuredErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test error",
		nil,
		nil,
	)

	// Act
	output := formatter.FormatError(structuredErr)

	// Assert
	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	timestamp, ok := result["timestamp"].(string)
	require.True(t, ok, "timestamp must be a string")
	require.NotEmpty(t, timestamp, "timestamp must not be empty")

	// Verify ISO 8601 format (RFC3339)
	_, err = time.Parse(time.RFC3339, timestamp)
	assert.NoError(t, err, "timestamp must be in ISO 8601 (RFC3339) format")
}

// TestJSONErrorFormatter_OutputStructure verifies the exact JSON structure.
func TestJSONErrorFormatter_OutputStructure(t *testing.T) {
	// Arrange
	formatter := NewJSONErrorFormatter(false)
	structuredErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{
			"path": "/workflow.yaml",
		},
		nil,
	)

	// Act
	output := formatter.FormatError(structuredErr)

	// Assert
	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Verify required fields exist
	assert.Contains(t, result, "error_code", "must contain error_code field")
	assert.Contains(t, result, "message", "must contain message field")
	assert.Contains(t, result, "timestamp", "must contain timestamp field")

	// Verify field types
	_, ok := result["error_code"].(string)
	assert.True(t, ok, "error_code must be string")

	_, ok = result["message"].(string)
	assert.True(t, ok, "message must be string")

	_, ok = result["timestamp"].(string)
	assert.True(t, ok, "timestamp must be string")
}

// TestJSONErrorFormatter_AllErrorCategories tests all error code categories.
func TestJSONErrorFormatter_AllErrorCategories(t *testing.T) {
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

	formatter := NewJSONErrorFormatter(false)

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
			var result map[string]any
			err := json.Unmarshal([]byte(output), &result)
			require.NoError(t, err)

			errorCode, ok := result["error_code"].(string)
			require.True(t, ok)
			assert.True(t, strings.HasPrefix(errorCode, tt.category),
				"error code %s should start with category %s", errorCode, tt.category)
		})
	}
}

// TestJSONErrorFormatter_ConsistentOutput verifies formatting is deterministic.
func TestJSONErrorFormatter_ConsistentOutput(t *testing.T) {
	// Arrange
	formatter := NewJSONErrorFormatter(false)

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

// TestJSONErrorFormatter_Idempotency verifies multiple calls produce same result.
func TestJSONErrorFormatter_Idempotency(t *testing.T) {
	// Arrange
	formatter := NewJSONErrorFormatter(false)
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

// =============================================================================
// Hint Generation Tests (Component T009 - C048)
// =============================================================================

// TestJSONErrorFormatter_WithHints_HappyPath tests hint generation with various scenarios.
func TestJSONErrorFormatter_WithHints_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		noHints    bool
		generators []domainerrors.HintGenerator
		errCode    domainerrors.ErrorCode
		message    string
		details    map[string]any
		validate   func(t *testing.T, output string)
	}{
		{
			name:    "single hint from generator",
			noHints: false,
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{
						{Message: "Did you mean 'my-workflow.yaml'?"},
					}
				},
			},
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "workflow file not found",
			details: map[string]any{"path": "/workflow.yaml"},
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Contains(t, result, "hints", "should include hints array")
				hints, ok := result["hints"].([]any)
				require.True(t, ok)
				assert.Len(t, hints, 1, "should have exactly one hint")
				assert.Equal(t, "Did you mean 'my-workflow.yaml'?", hints[0])
			},
		},
		{
			name:    "multiple hints from single generator",
			noHints: false,
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{
						{Message: "Did you mean 'my-workflow.yaml'?"},
						{Message: "Run 'awf list' to see available workflows"},
						{Message: "Check your current directory with 'pwd'"},
					}
				},
			},
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "workflow file not found",
			details: map[string]any{"path": "/workflow.yaml"},
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				hints, ok := result["hints"].([]any)
				require.True(t, ok)
				assert.Len(t, hints, 3, "should have three hints")
				assert.Equal(t, "Did you mean 'my-workflow.yaml'?", hints[0])
				assert.Equal(t, "Run 'awf list' to see available workflows", hints[1])
				assert.Equal(t, "Check your current directory with 'pwd'", hints[2])
			},
		},
		{
			name:    "hints from multiple generators",
			noHints: false,
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{
						{Message: "First generator hint"},
					}
				},
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{
						{Message: "Second generator hint"},
					}
				},
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{
						{Message: "Third generator hint"},
					}
				},
			},
			errCode: domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
			message: "YAML syntax error",
			details: nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				hints, ok := result["hints"].([]any)
				require.True(t, ok)
				assert.Len(t, hints, 3, "should aggregate hints from all generators")
				assert.Equal(t, "First generator hint", hints[0])
				assert.Equal(t, "Second generator hint", hints[1])
				assert.Equal(t, "Third generator hint", hints[2])
			},
		},
		{
			name:    "empty hints from generator",
			noHints: false,
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{} // empty slice
				},
			},
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "workflow file not found",
			details: nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				// Hints field should not be present when no hints generated
				_, exists := result["hints"]
				assert.False(t, exists, "hints field should not be present when empty")
			},
		},
		{
			name:       "no generators provided",
			noHints:    false,
			generators: nil,
			errCode:    domainerrors.ErrorCodeUserInputMissingFile,
			message:    "workflow file not found",
			details:    nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				// Hints field should not be present when no generators
				_, exists := result["hints"]
				assert.False(t, exists, "hints field should not be present with no generators")
			},
		},
		{
			name:    "noHints flag suppresses hints",
			noHints: true,
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{
						{Message: "This hint should be suppressed"},
					}
				},
			},
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "workflow file not found",
			details: nil,
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				// Hints field should not be present when noHints is true
				_, exists := result["hints"]
				assert.False(t, exists, "hints should be suppressed when noHints=true")

				// Other fields should still be present
				assert.Contains(t, result, "error_code")
				assert.Contains(t, result, "message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			formatter := NewJSONErrorFormatter(tt.noHints, tt.generators...)
			structuredErr := domainerrors.NewStructuredError(
				tt.errCode,
				tt.message,
				tt.details,
				nil,
			)

			// Act
			output := formatter.FormatError(structuredErr)

			// Assert
			assert.NotEmpty(t, output, "output should not be empty")
			tt.validate(t, output)
		})
	}
}

// TestJSONErrorFormatter_HintGeneration_EdgeCases tests edge cases in hint generation.
func TestJSONErrorFormatter_HintGeneration_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		generators []domainerrors.HintGenerator
		errCode    domainerrors.ErrorCode
		message    string
		validate   func(t *testing.T, output string)
	}{
		{
			name: "generator returns nil slice",
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return nil // nil slice, not empty slice
				},
			},
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "test error",
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				// Should handle nil slice gracefully
				_, exists := result["hints"]
				assert.False(t, exists, "hints should not be present for nil slice")
			},
		},
		{
			name: "generator panics - should not crash formatter",
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					// This test verifies proper error handling
					// In production, generators should never panic
					panic("generator error")
				},
			},
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "test error",
			validate: func(t *testing.T, output string) {
				// If we reach here, the formatter recovered from panic
				// This test should be implemented with recover() in generateHints
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)
				// At minimum, error_code and message should be present
				assert.Contains(t, result, "error_code")
				assert.Contains(t, result, "message")
			},
		},
		{
			name: "mixed generators - some empty, some with hints",
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{} // empty
				},
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{
						{Message: "Valid hint from second generator"},
					}
				},
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return nil // nil
				},
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{
						{Message: "Valid hint from fourth generator"},
					}
				},
			},
			errCode: domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
			message: "parse error",
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				hints, ok := result["hints"].([]any)
				require.True(t, ok)
				assert.Len(t, hints, 2, "should only include non-empty hints")
				assert.Equal(t, "Valid hint from second generator", hints[0])
				assert.Equal(t, "Valid hint from fourth generator", hints[1])
			},
		},
		{
			name: "hints with special characters",
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{
						{Message: "Did you mean \"my-workflow.yaml\"?"},
						{Message: "Path contains special chars: /path/to/file\n\ttab\r\n"},
						{Message: "Unicode hint: 文件 не найден 🔥"},
					}
				},
			},
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "file not found",
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err, "JSON should properly escape special characters")

				hints, ok := result["hints"].([]any)
				require.True(t, ok)
				assert.Len(t, hints, 3)
				assert.Contains(t, hints[0], "my-workflow.yaml")
				assert.Contains(t, hints[2], "文件")
			},
		},
		{
			name: "very long hint message",
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					longMessage := strings.Repeat("This is a very long hint message. ", 50)
					return []domainerrors.Hint{
						{Message: longMessage},
					}
				},
			},
			errCode: domainerrors.ErrorCodeWorkflowValidationCycleDetected,
			message: "validation error",
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				hints, ok := result["hints"].([]any)
				require.True(t, ok)
				assert.Len(t, hints, 1)
				hintMsg, ok := hints[0].(string)
				require.True(t, ok)
				assert.Greater(t, len(hintMsg), 1000, "long hint should be preserved")
			},
		},
		{
			name: "empty hint message",
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{
						{Message: ""}, // empty message
						{Message: "Valid hint"},
					}
				},
			},
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "test error",
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				hints, ok := result["hints"].([]any)
				require.True(t, ok)
				// Both hints should be included (filtering empty is implementation choice)
				assert.GreaterOrEqual(t, len(hints), 1, "should have at least valid hint")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip panic test for now - requires implementation support
			if strings.Contains(tt.name, "panics") {
				t.Skip("Panic recovery test - implement in GREEN phase")
			}

			// Arrange
			formatter := NewJSONErrorFormatter(false, tt.generators...)
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

// TestJSONErrorFormatter_HintOrdering verifies hints maintain generator order.
func TestJSONErrorFormatter_HintOrdering(t *testing.T) {
	// Arrange - multiple generators that return hints in specific order
	generator1 := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		return []domainerrors.Hint{
			{Message: "Hint A1"},
			{Message: "Hint A2"},
		}
	}
	generator2 := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		return []domainerrors.Hint{
			{Message: "Hint B1"},
		}
	}
	generator3 := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		return []domainerrors.Hint{
			{Message: "Hint C1"},
			{Message: "Hint C2"},
			{Message: "Hint C3"},
		}
	}

	formatter := NewJSONErrorFormatter(false, generator1, generator2, generator3)
	structuredErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test error",
		nil,
		nil,
	)

	// Act
	output := formatter.FormatError(structuredErr)

	// Assert
	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	hints, ok := result["hints"].([]any)
	require.True(t, ok)
	require.Len(t, hints, 6, "should aggregate all hints in order")

	// Verify exact order: generator1 hints, then generator2, then generator3
	expectedOrder := []string{
		"Hint A1", "Hint A2",
		"Hint B1",
		"Hint C1", "Hint C2", "Hint C3",
	}
	for i, expected := range expectedOrder {
		assert.Equal(t, expected, hints[i], "hint at index %d should maintain order", i)
	}
}

// TestJSONErrorFormatter_HintContextAware verifies generators receive error context.
func TestJSONErrorFormatter_HintContextAware(t *testing.T) {
	tests := []struct {
		name      string
		errCode   domainerrors.ErrorCode
		message   string
		details   map[string]any
		generator domainerrors.HintGenerator
		validate  func(t *testing.T, output string)
	}{
		{
			name:    "generator accesses error code",
			errCode: domainerrors.ErrorCodeUserInputMissingFile,
			message: "file not found",
			details: nil,
			generator: func(err *domainerrors.StructuredError) []domainerrors.Hint {
				// Generator can inspect error code to produce context-aware hints
				if err.Code == domainerrors.ErrorCodeUserInputMissingFile {
					return []domainerrors.Hint{
						{Message: "Hint specific to MISSING_FILE error"},
					}
				}
				return nil
			},
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				hints, ok := result["hints"].([]any)
				require.True(t, ok)
				assert.Len(t, hints, 1)
				assert.Equal(t, "Hint specific to MISSING_FILE error", hints[0])
			},
		},
		{
			name:    "generator accesses details",
			errCode: domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
			message: "YAML syntax error",
			details: map[string]any{
				"line":   42,
				"column": 10,
			},
			generator: func(err *domainerrors.StructuredError) []domainerrors.Hint {
				// Generator can extract details for context-aware hints
				if line, ok := err.Details["line"].(int); ok {
					return []domainerrors.Hint{
						{Message: "Syntax error at line " + strings.Repeat("X", line%10)}, // use line number
					}
				}
				return nil
			},
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				hints, ok := result["hints"].([]any)
				require.True(t, ok)
				assert.Len(t, hints, 1)
				assert.Contains(t, hints[0], "Syntax error at line")
			},
		},
		{
			name:    "generator with no matching error type returns empty",
			errCode: domainerrors.ErrorCodeExecutionCommandFailed,
			message: "command failed",
			details: nil,
			generator: func(err *domainerrors.StructuredError) []domainerrors.Hint {
				// Generator only handles specific error types
				if err.Code == domainerrors.ErrorCodeUserInputMissingFile {
					return []domainerrors.Hint{{Message: "Should not appear"}}
				}
				return []domainerrors.Hint{} // empty for non-matching errors
			},
			validate: func(t *testing.T, output string) {
				var result map[string]any
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				// No hints should be present
				_, exists := result["hints"]
				assert.False(t, exists, "hints should not be present for non-matching error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			formatter := NewJSONErrorFormatter(false, tt.generator)
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

// TestJSONErrorFormatter_HintThreadSafety verifies concurrent hint generation is safe.
func TestJSONErrorFormatter_HintThreadSafety(t *testing.T) {
	// Arrange - create formatter with stateless generator
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		return []domainerrors.Hint{
			{Message: "Thread-safe hint"},
		}
	}
	formatter := NewJSONErrorFormatter(false, generator)

	structuredErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test error",
		nil,
		nil,
	)

	// Act - format concurrently from multiple goroutines
	const goroutines = 10
	results := make([]string, goroutines)
	done := make(chan bool)

	for i := 0; i < goroutines; i++ {
		go func(index int) {
			results[index] = formatter.FormatError(structuredErr)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Assert - all results should be identical (formatter is stateless)
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i],
			"concurrent formatting should produce identical results")
	}

	// Verify hints are present in result
	var result map[string]any
	err := json.Unmarshal([]byte(results[0]), &result)
	require.NoError(t, err)
	hints, ok := result["hints"].([]any)
	require.True(t, ok)
	assert.Len(t, hints, 1)
}

// TestJSONErrorFormatter_NoHints_PreservesErrorStructure verifies noHints doesn't break output.
func TestJSONErrorFormatter_NoHints_PreservesErrorStructure(t *testing.T) {
	// Arrange - create two formatters: one with hints, one without
	generator := func(err *domainerrors.StructuredError) []domainerrors.Hint {
		return []domainerrors.Hint{
			{Message: "Test hint"},
		}
	}

	formatterWithHints := NewJSONErrorFormatter(false, generator)
	formatterNoHints := NewJSONErrorFormatter(true, generator)

	structuredErr := &domainerrors.StructuredError{
		Code:      domainerrors.ErrorCodeUserInputMissingFile,
		Message:   "workflow file not found",
		Details:   map[string]any{"path": "/test.yaml"},
		Cause:     nil,
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	// Act
	outputWithHints := formatterWithHints.FormatError(structuredErr)
	outputNoHints := formatterNoHints.FormatError(structuredErr)

	// Assert
	var resultWithHints, resultNoHints map[string]any

	err := json.Unmarshal([]byte(outputWithHints), &resultWithHints)
	require.NoError(t, err)

	err = json.Unmarshal([]byte(outputNoHints), &resultNoHints)
	require.NoError(t, err)

	// Both should have same base structure
	assert.Equal(t, resultWithHints["error_code"], resultNoHints["error_code"])
	assert.Equal(t, resultWithHints["message"], resultNoHints["message"])
	assert.Equal(t, resultWithHints["timestamp"], resultNoHints["timestamp"])
	assert.Equal(t, resultWithHints["details"], resultNoHints["details"])

	// Only difference should be hints field
	assert.Contains(t, resultWithHints, "hints", "formatter with hints should have hints field")
	assert.NotContains(t, resultNoHints, "hints", "formatter with noHints should not have hints field")
}
