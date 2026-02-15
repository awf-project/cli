package errfmt

import (
	"encoding/json"

	domainerrors "github.com/awf-project/awf/internal/domain/errors"
	"github.com/awf-project/awf/internal/domain/ports"
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
//	  "timestamp": "2025-01-15T10:30:45Z",
//	  "hints": ["Did you mean 'my-workflow.yaml'?"]
//	}
//
// Usage:
//
//	formatter := NewJSONErrorFormatter(false, generators...)
//	output := formatter.FormatError(structuredErr)
//	fmt.Println(output)
//
// Component: C047 Structured Error Codes Taxonomy, C048 Actionable Error Hints
// Layer: Infrastructure
type JSONErrorFormatter struct {
	noHints    bool
	generators []domainerrors.HintGenerator
}

// Compile-time assertion that JSONErrorFormatter implements ports.ErrorFormatter
var _ ports.ErrorFormatter = (*JSONErrorFormatter)(nil)

// NewJSONErrorFormatter creates a new JSONErrorFormatter instance.
//
// Parameters:
//   - noHints: If true, suppresses hint generation (for scripted usage)
//   - generators: Optional hint generators to invoke for contextual suggestions
//
// Returns:
//   - A new JSONErrorFormatter ready to format structured errors as JSON
//
// Example:
//
//	formatter := NewJSONErrorFormatter(false, FileNotFoundHintGenerator, YAMLSyntaxHintGenerator)
//	output := formatter.FormatError(err)
func NewJSONErrorFormatter(noHints bool, generators ...domainerrors.HintGenerator) *JSONErrorFormatter {
	return &JSONErrorFormatter{
		noHints:    noHints,
		generators: generators,
	}
}

// FormatError converts a StructuredError into JSON string representation.
//
// Implements the ErrorFormatter port interface. Returns a JSON object containing:
//   - error_code: hierarchical error code (e.g., "USER.INPUT.MISSING_FILE")
//   - message: human-readable error message
//   - details: structured key-value pairs for additional context
//   - timestamp: ISO 8601 formatted timestamp
//   - hints: array of actionable suggestions (if noHints is false)
//
// Parameters:
//   - err: The structured error to format
//
// Returns:
//   - JSON string representation of the error
//
// Example:
//
//	formatter := NewJSONErrorFormatter(false, FileNotFoundHintGenerator)
//	err := domainerrors.NewStructuredError(
//	    domainerrors.ErrorCodeUserInputMissingFile,
//	    "workflow file not found",
//	    map[string]any{"path": "/workflow.yaml"},
//	    nil,
//	)
//	output := formatter.FormatError(err)
//	// {"error_code":"USER.INPUT.MISSING_FILE","message":"workflow file not found",...,"hints":[...]}
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

	// Generate and include hints if enabled
	if !f.noHints {
		hints := f.generateHints(err)
		if len(hints) > 0 {
			output["hints"] = hints
		}
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

// generateHints aggregates hints from all registered generators.
// Returns a slice of hint message strings.
//
// Iterates through all registered generators in order, invoking each with the
// structured error. Aggregates all returned hints into a single slice, preserving
// generator order and hint order within each generator's results.
//
// Implementation notes:
//   - Generators returning nil or empty slices are skipped (no hints added)
//   - Empty hint messages are included (filtering is presentation concern)
//   - Panicking generators are not caught (fail-fast for proper error reporting)
//   - Thread-safe: generators must be stateless, no shared mutable state
//
// Returns:
//   - Slice of hint message strings, empty if no hints generated
func (f *JSONErrorFormatter) generateHints(err *domainerrors.StructuredError) []string {
	if len(f.generators) == 0 {
		return []string{}
	}

	var allHints []string

	for _, generator := range f.generators {
		hints := generator(err)

		// Skip nil or empty slices
		if len(hints) == 0 {
			continue
		}

		// Extract message strings from Hint structs
		for _, hint := range hints {
			allHints = append(allHints, hint.Message)
		}
	}

	return allHints
}
