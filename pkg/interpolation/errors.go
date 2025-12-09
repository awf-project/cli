package interpolation

import "fmt"

// UndefinedVariableError is returned when a variable path cannot be resolved.
type UndefinedVariableError struct {
	Variable string
}

func (e *UndefinedVariableError) Error() string {
	return fmt.Sprintf("undefined variable: %s", e.Variable)
}

// ParseError is returned when template parsing fails.
type ParseError struct {
	Template string
	Cause    error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse template: %v", e.Cause)
}

func (e *ParseError) Unwrap() error {
	return e.Cause
}
