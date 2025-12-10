package workflow

import "fmt"

// ValidationLevel indicates the severity of a validation issue.
type ValidationLevel string

const (
	ValidationLevelError   ValidationLevel = "error"
	ValidationLevelWarning ValidationLevel = "warning"
)

// ValidationCode identifies specific validation issues.
type ValidationCode string

const (
	ErrCycleDetected       ValidationCode = "cycle_detected"
	ErrUnreachableState    ValidationCode = "unreachable_state"
	ErrInvalidTransition   ValidationCode = "invalid_transition"
	ErrNoTerminalState     ValidationCode = "no_terminal_state"
	ErrMissingInitialState ValidationCode = "missing_initial_state"
)

// ValidationError represents a single validation issue.
type ValidationError struct {
	Level   ValidationLevel
	Code    ValidationCode
	Message string
	Path    string // e.g., "states.validate.on_success"
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Level, e.Path, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Level, e.Message)
}

// IsError returns true if this is an error-level issue (not a warning).
func (e ValidationError) IsError() bool {
	return e.Level == ValidationLevelError
}

// ValidationResult holds the complete result of graph validation.
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
}

// HasErrors returns true if there are any error-level issues.
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if there are any warning-level issues.
func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// AllIssues returns all errors and warnings combined.
func (r *ValidationResult) AllIssues() []ValidationError {
	all := make([]ValidationError, 0, len(r.Errors)+len(r.Warnings))
	all = append(all, r.Errors...)
	all = append(all, r.Warnings...)
	return all
}

// AddError adds an error-level validation issue.
func (r *ValidationResult) AddError(code ValidationCode, path, message string) {
	r.Errors = append(r.Errors, ValidationError{
		Level:   ValidationLevelError,
		Code:    code,
		Message: message,
		Path:    path,
	})
}

// AddWarning adds a warning-level validation issue.
func (r *ValidationResult) AddWarning(code ValidationCode, path, message string) {
	r.Warnings = append(r.Warnings, ValidationError{
		Level:   ValidationLevelWarning,
		Code:    code,
		Message: message,
		Path:    path,
	})
}

// ToError converts the validation result to a single error if there are errors.
// Returns nil if no errors (warnings don't cause an error).
func (r *ValidationResult) ToError() error {
	if !r.HasErrors() {
		return nil
	}
	if len(r.Errors) == 1 {
		return r.Errors[0]
	}
	// Aggregate multiple errors
	errs := make([]error, len(r.Errors))
	for i, e := range r.Errors {
		errs[i] = e
	}
	return fmt.Errorf("validation failed with %d errors", len(r.Errors))
}
