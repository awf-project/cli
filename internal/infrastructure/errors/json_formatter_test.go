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
	formatter := NewJSONErrorFormatter()
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
			formatter := NewJSONErrorFormatter()
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
			formatter := NewJSONErrorFormatter()
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
			formatter := NewJSONErrorFormatter()
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
	formatter := NewJSONErrorFormatter()
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
	formatter := NewJSONErrorFormatter()
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

	formatter := NewJSONErrorFormatter()

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
	formatter := NewJSONErrorFormatter()

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
	formatter := NewJSONErrorFormatter()
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
