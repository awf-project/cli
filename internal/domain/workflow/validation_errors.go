package workflow

import (
	"errors"
	"fmt"
)

// ValidationLevel indicates the severity of a validation issue.
type ValidationLevel string

const (
	ValidationLevelError   ValidationLevel = "error"
	ValidationLevelWarning ValidationLevel = "warning"
)

// ValidationCode identifies specific validation issues.
//
// Relationship to errors.ErrorCode: ValidationCode and errors.ErrorCode are
// intentionally separate types. ValidationCode covers static graph-validation
// issues (cycle, missing state, template reference) that are discovered at
// parse time and reported through ValidationResult. errors.ErrorCode covers
// runtime structured errors that drive exit-code mapping and machine-readable
// output. Some runtime codes (e.g. ErrorCodeUserMCPProxyEmptyProxy) are
// "borrowed" into validation by explicit conversion when the same condition
// must be reported in both contexts. The explicit conversion makes the
// cross-layer usage visible at the call site and avoids creating a circular
// import between domain/workflow and domain/errors.
type ValidationCode string

const (
	ErrCycleDetected       ValidationCode = "cycle_detected"
	ErrUnreachableState    ValidationCode = "unreachable_state"
	ErrInvalidTransition   ValidationCode = "invalid_transition"
	ErrNoTerminalState     ValidationCode = "no_terminal_state"
	ErrMissingInitialState ValidationCode = "missing_initial_state"

	// Template reference validation codes
	ErrUndefinedInput           ValidationCode = "undefined_input"
	ErrUndefinedStep            ValidationCode = "undefined_step"
	ErrForwardReference         ValidationCode = "forward_reference"
	ErrInvalidWorkflowProperty  ValidationCode = "invalid_workflow_property"
	ErrInvalidStateProperty     ValidationCode = "invalid_state_property"
	ErrInvalidErrorProperty     ValidationCode = "invalid_error_property"
	ErrInvalidContextProperty   ValidationCode = "invalid_context_property"
	ErrInvalidLoopProperty      ValidationCode = "invalid_loop_property"
	ErrUnknownReferenceType     ValidationCode = "unknown_reference_type"
	ErrErrorRefOutsideErrorHook ValidationCode = "error_ref_outside_error_hook"

	// Loop expression validation codes
	ErrUndefinedLoopVariable ValidationCode = "undefined_loop_variable"

	// Sub-workflow validation codes
	ErrCircularWorkflowCall ValidationCode = "circular_workflow_call"
	ErrUndefinedSubworkflow ValidationCode = "undefined_subworkflow"
	ErrMaxNestingExceeded   ValidationCode = "max_nesting_exceeded"

	// Skill validation codes
	ErrSkillNotFound       ValidationCode = "skill_not_found"
	ErrSkillMissingSkillMD ValidationCode = "skill_missing_skillmd"
	ErrSkillEmptyContent   ValidationCode = "skill_empty_content"

	// Agent role validation codes
	ErrRoleNotFound        ValidationCode = "role_not_found"
	ErrRoleMissingAgentsMD ValidationCode = "role_missing_agents_md"
	ErrRoleEmptyContent    ValidationCode = "role_empty_content"
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
	// Aggregate multiple errors preserving each individual error's detail so
	// callers can inspect them via errors.Is / errors.As over the joined chain.
	errs := make([]error, len(r.Errors))
	for i, e := range r.Errors {
		errs[i] = e
	}
	return errors.Join(errs...)
}
