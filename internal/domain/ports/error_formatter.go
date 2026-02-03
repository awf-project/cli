package ports

import "github.com/vanoix/awf/internal/domain/errors"

// ErrorFormatter defines the contract for formatting structured errors
// into different output representations (JSON, human-readable, etc.).
// Implementations live in infrastructure layer.
//
// ErrorFormatter enables:
//   - Machine-readable JSON output for CI/CD pipelines
//   - Human-readable CLI output with color and formatting
//   - Consistent error presentation across output modes
//
// Example usage:
//
//	formatter := infrastructure.NewJSONErrorFormatter()
//	output := formatter.FormatError(structuredErr)
type ErrorFormatter interface {
	// FormatError converts a StructuredError into a formatted string representation.
	// The format depends on the implementation (JSON, human-readable, etc.).
	//
	// Parameters:
	//   - err: The structured error to format
	//
	// Returns:
	//   - Formatted error string according to the implementation's output mode
	//
	// Example:
	//
	//	// JSON formatter:
	//	// {"error_code":"USER.INPUT.MISSING_FILE","message":"workflow file not found",...}
	//
	//	// Human formatter:
	//	// [USER.INPUT.MISSING_FILE] workflow file not found
	FormatError(err *errors.StructuredError) string
}
