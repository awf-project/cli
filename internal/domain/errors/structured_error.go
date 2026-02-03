//nolint:revive // Package name "errors" is intentional; fully qualified import path avoids stdlib conflict
package errors

import (
	"fmt"
	"time"
)

// StructuredError represents a domain error with hierarchical error code,
// human-readable message, structured details, optional cause chain, and timestamp.
// Implements the error interface and supports error wrapping via Unwrap().
//
// StructuredError enables:
//   - Machine-readable error codes for programmatic handling
//   - Exit code mapping via ErrorCode.ExitCode()
//   - Error cause chains for debugging
//   - Structured details (key-value pairs) for context
//   - Timestamp for telemetry and logging
//
// Example:
//
//	err := NewStructuredError(
//	    ErrorCodeUserInputMissingFile,
//	    "workflow file not found",
//	    map[string]any{"path": "/path/to/workflow.yaml"},
//	    originalErr,
//	)
//
// StructuredError is distinct from workflow.ValidationError:
//   - StructuredError: cross-cutting error taxonomy, all layers
//   - ValidationError: workflow-specific validation, has Level/Path fields
type StructuredError struct {
	// Code is the hierarchical error identifier (CATEGORY.SUBCATEGORY.SPECIFIC).
	Code ErrorCode

	// Message is the human-readable error description.
	Message string

	// Details contains additional structured context (e.g., file paths, field names).
	// Optional. Use for machine-readable debugging information.
	Details map[string]any

	// Cause is the wrapped underlying error, if any.
	// Optional. Enables error cause chains via Unwrap().
	Cause error

	// Timestamp records when the error was created.
	// Used for telemetry, logging, and debugging temporal issues.
	Timestamp time.Time
}

// NewStructuredError creates a new StructuredError with the given code, message, details, and cause.
// Automatically sets Timestamp to time.Now().
//
// Parameters:
//   - code: ErrorCode constant (e.g., ErrorCodeUserInputMissingFile)
//   - message: Human-readable error message
//   - details: Optional structured context (pass nil if not needed)
//   - cause: Optional underlying error (pass nil if not wrapping)
//
// Example:
//
//	err := NewStructuredError(
//	    ErrorCodeWorkflowParseYAMLSyntax,
//	    "invalid YAML syntax",
//	    map[string]any{"line": 42, "column": 10},
//	    yamlErr,
//	)
func NewStructuredError(code ErrorCode, message string, details map[string]any, cause error) *StructuredError {
	return &StructuredError{
		Code:      code,
		Message:   message,
		Details:   details,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// Error implements the error interface.
// Returns the message field for string representation.
func (e *StructuredError) Error() string {
	return e.Message
}

// Unwrap returns the underlying cause error, enabling error chain traversal.
// Returns nil if no cause is set.
func (e *StructuredError) Unwrap() error {
	return e.Cause
}

// NewUserError creates a StructuredError with a USER.* error code.
// Convenience constructor for user-facing input errors (exit code 1).
func NewUserError(code ErrorCode, message string, details map[string]any, cause error) *StructuredError {
	return NewStructuredError(code, message, details, cause)
}

// NewWorkflowError creates a StructuredError with a WORKFLOW.* error code.
// Convenience constructor for workflow definition errors (exit code 2).
func NewWorkflowError(code ErrorCode, message string, details map[string]any, cause error) *StructuredError {
	return NewStructuredError(code, message, details, cause)
}

// NewExecutionError creates a StructuredError with an EXECUTION.* error code.
// Convenience constructor for runtime execution errors (exit code 3).
func NewExecutionError(code ErrorCode, message string, details map[string]any, cause error) *StructuredError {
	return NewStructuredError(code, message, details, cause)
}

// NewSystemError creates a StructuredError with a SYSTEM.* error code.
// Convenience constructor for infrastructure errors (exit code 4).
func NewSystemError(code ErrorCode, message string, details map[string]any, cause error) *StructuredError {
	return NewStructuredError(code, message, details, cause)
}

// ExitCode returns the process exit code for this error by delegating to ErrorCode.ExitCode().
// Maps error categories to exit codes:
//   - USER.* → 1
//   - WORKFLOW.* → 2
//   - EXECUTION.* → 3
//   - SYSTEM.* → 4
func (e *StructuredError) ExitCode() int {
	return e.Code.ExitCode()
}

// WithDetails returns a new StructuredError with the given details merged into the existing details.
// Useful for adding context as errors propagate up the call stack.
//
// Example:
//
//	err := baseErr.WithDetails(map[string]any{"workflow_id": wf.ID})
func (e *StructuredError) WithDetails(additionalDetails map[string]any) *StructuredError {
	// Merge details
	merged := make(map[string]any, len(e.Details)+len(additionalDetails))
	for k, v := range e.Details {
		merged[k] = v
	}
	for k, v := range additionalDetails {
		merged[k] = v
	}

	return &StructuredError{
		Code:      e.Code,
		Message:   e.Message,
		Details:   merged,
		Cause:     e.Cause,
		Timestamp: e.Timestamp, // Preserve original timestamp
	}
}

// Is implements error matching for errors.Is().
// Returns true if the target is a StructuredError with the same ErrorCode.
func (e *StructuredError) Is(target error) bool {
	t, ok := target.(*StructuredError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// As implements error type assertion for errors.As().
// Enables error chain traversal via errors.As().
func (e *StructuredError) As(target any) bool {
	if t, ok := target.(**StructuredError); ok {
		*t = e
		return true
	}
	return false
}

// Format implements fmt.Formatter for custom error formatting.
// Supports:
//   - %s, %v: message only
//   - %+v: message with code and details
//
// Example:
//
//	fmt.Printf("%+v", err)  // "USER.INPUT.MISSING_FILE: workflow file not found (path=/workflow.yaml)"
func (e *StructuredError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			// Verbose format: include code and details
			fmt.Fprintf(s, "%s: %s", e.Code, e.Message)
			if len(e.Details) > 0 {
				fmt.Fprintf(s, " (")
				first := true
				for k, v := range e.Details {
					if !first {
						fmt.Fprintf(s, ", ")
					}
					fmt.Fprintf(s, "%s=%v", k, v)
					first = false
				}
				fmt.Fprintf(s, ")")
			}
			if e.Cause != nil {
				fmt.Fprintf(s, ": %+v", e.Cause)
			}
		} else {
			fmt.Fprint(s, e.Message)
		}
	case 's':
		fmt.Fprint(s, e.Message)
	default:
		fmt.Fprint(s, e.Message)
	}
}
