// Package errors provides infrastructure adapters for error formatting.
//
// This package implements the ErrorFormatter port from the domain layer,
// providing concrete formatters for different output modes:
//   - JSONErrorFormatter: Machine-readable JSON output for CI/CD pipelines
//   - HumanErrorFormatter: Human-readable CLI output with color and formatting
//
// Architecture:
//   - Domain defines: ErrorFormatter port interface, StructuredError type
//   - Infrastructure provides: JSONErrorFormatter, HumanErrorFormatter adapters
//   - Application/CLI inject: Formatter via dependency injection
//
// Example usage:
//
//	// JSON output for programmatic consumption
//	formatter := errors.NewJSONErrorFormatter()
//	output := formatter.FormatError(structuredErr)
//	// {"error_code":"USER.INPUT.MISSING_FILE","message":"workflow file not found",...}
//
//	// Human-readable output for CLI
//	formatter := errors.NewHumanErrorFormatter()
//	output := formatter.FormatError(structuredErr)
//	// [USER.INPUT.MISSING_FILE] workflow file not found
//
// Component: C047 Structured Error Codes Taxonomy
// Layer: Infrastructure
package errfmt
