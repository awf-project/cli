package repository

import "fmt"

// ParseError represents an error during YAML parsing.
type ParseError struct {
	File    string // file path
	Line    int    // line number (-1 if unknown)
	Column  int    // column number (-1 if unknown)
	Field   string // field path (e.g., "states.validate.command")
	Message string // error message
	Cause   error  // underlying error
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d:%d: %s: %s", e.File, e.Line, e.Column, e.Field, e.Message)
	}
	if e.Field != "" {
		return fmt.Sprintf("%s: %s: %s", e.File, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Message)
}

// Unwrap returns the underlying error.
func (e *ParseError) Unwrap() error {
	return e.Cause
}

// NewParseError creates a new ParseError with field and message.
func NewParseError(file, field, message string) *ParseError {
	return &ParseError{
		File:    file,
		Field:   field,
		Message: message,
		Line:    -1,
		Column:  -1,
	}
}

// WrapParseError wraps an existing error as a ParseError.
func WrapParseError(file string, cause error) *ParseError {
	return &ParseError{
		File:    file,
		Message: cause.Error(),
		Cause:   cause,
		Line:    -1,
		Column:  -1,
	}
}

// ParseErrorWithLine creates a ParseError with line information.
func ParseErrorWithLine(file string, line, column int, message string) *ParseError {
	return &ParseError{
		File:    file,
		Line:    line,
		Column:  column,
		Message: message,
	}
}
