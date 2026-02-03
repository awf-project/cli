package errfmt

import (
	"encoding/json"

	domainerrors "github.com/vanoix/awf/internal/domain/errors"
	"github.com/vanoix/awf/internal/domain/ports"
)

// JSONErrorFormatter implements the ErrorFormatter port interface, providing
// machine-readable JSON output for structured errors.
//
// Output format:
//
//	{
//	  "error_code": "USER.INPUT.MISSING_FILE",
//	  "message": "workflow file not found",
//	  "details": {"path": "/workflow.yaml"},
//	  "timestamp": "2025-01-15T10:30:45Z"
//	}
//
// Usage:
//
//	formatter := NewJSONErrorFormatter()
//	output := formatter.FormatError(structuredErr)
//	fmt.Println(output)
//
// Component: C047 Structured Error Codes Taxonomy
// Layer: Infrastructure
type JSONErrorFormatter struct{}

// Compile-time assertion that JSONErrorFormatter implements ports.ErrorFormatter
var _ ports.ErrorFormatter = (*JSONErrorFormatter)(nil)

// NewJSONErrorFormatter creates a new JSONErrorFormatter instance.
//
// Returns:
//   - A new JSONErrorFormatter ready to format structured errors as JSON
//
// Example:
//
//	formatter := NewJSONErrorFormatter()
//	output := formatter.FormatError(err)
func NewJSONErrorFormatter() *JSONErrorFormatter {
	return &JSONErrorFormatter{}
}

// FormatError converts a StructuredError into JSON string representation.
//
// Implements the ErrorFormatter port interface. Returns a JSON object containing:
//   - error_code: hierarchical error code (e.g., "USER.INPUT.MISSING_FILE")
//   - message: human-readable error message
//   - details: structured key-value pairs for additional context
//   - timestamp: ISO 8601 formatted timestamp
//
// Parameters:
//   - err: The structured error to format
//
// Returns:
//   - JSON string representation of the error
//
// Example:
//
//	formatter := NewJSONErrorFormatter()
//	err := domainerrors.NewStructuredError(
//	    domainerrors.ErrorCodeUserInputMissingFile,
//	    "workflow file not found",
//	    map[string]any{"path": "/workflow.yaml"},
//	    nil,
//	)
//	output := formatter.FormatError(err)
//	// {"error_code":"USER.INPUT.MISSING_FILE","message":"workflow file not found",...}
func (f *JSONErrorFormatter) FormatError(err *domainerrors.StructuredError) string {
	// Create JSON structure with required fields
	output := map[string]any{
		"error_code": string(err.Code),
		"message":    err.Message,
		"timestamp":  err.Timestamp.Format("2006-01-02T15:04:05Z07:00"), // RFC3339/ISO 8601
	}

	// Include details if present (even if empty map)
	if err.Details != nil {
		output["details"] = err.Details
	}

	// Serialize to JSON
	jsonBytes, jsonErr := json.Marshal(output)
	if jsonErr != nil {
		// Fallback to minimal JSON if serialization fails
		// This should not happen in practice with well-formed StructuredErrors
		return `{"error_code":"SYSTEM.SERIALIZATION.FAILED","message":"failed to serialize error"}`
	}

	return string(jsonBytes)
}
