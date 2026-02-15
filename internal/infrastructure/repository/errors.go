package repository

import (
	"fmt"
	"regexp"
	"strconv"

	domerrors "github.com/awf-project/awf/internal/domain/errors"
	"github.com/awf-project/awf/internal/domain/workflow"
)

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

var (
	yamlLineRegex   = regexp.MustCompile(`line (\d+)`)
	yamlColumnRegex = regexp.MustCompile(`column (\d+)`)
)

// WrapParseError wraps an existing error as a ParseError.
// Extracts line and column information from yaml.v3 error messages.
func WrapParseError(file string, cause error) *ParseError {
	line := -1
	column := -1
	msg := cause.Error()

	// Extract line number from error message (e.g., "yaml: line 10: ...")
	if m := yamlLineRegex.FindStringSubmatch(msg); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			line = n
		}
	}

	// Extract column number from error message (e.g., "yaml: line 10: column 5: ...")
	if m := yamlColumnRegex.FindStringSubmatch(msg); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			column = n
		}
	}

	return &ParseError{
		File:    file,
		Line:    line,
		Column:  column,
		Message: msg,
		Cause:   cause,
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

// ToStructuredError converts ParseError to a domain StructuredError.
// This enables integration with the error hint system and structured error handling.
//
// Returns:
//   - WORKFLOW.PARSE.YAML_SYNTAX for YAML syntax errors
//   - Includes file, line, column, and field in error Details
func (e *ParseError) ToStructuredError() *domerrors.StructuredError {
	details := map[string]any{
		"file": e.File,
	}

	// Add line and column if available (>= 0 means extracted from yaml.v3 error)
	if e.Line >= 0 {
		details["line"] = e.Line
	}
	if e.Column >= 0 {
		details["column"] = e.Column
	}

	// Add field if specified (for required field validation errors)
	if e.Field != "" {
		details["field"] = e.Field
	}

	return domerrors.NewWorkflowError(
		domerrors.ErrorCodeWorkflowParseYAMLSyntax,
		e.Message,
		details,
		e.Cause,
	)
}

// TemplateNotFoundError is an alias for workflow.TemplateNotFoundError for backward compatibility.
// This allows existing infrastructure code to continue using repository.TemplateNotFoundError
// while the canonical definition lives in the domain layer where it belongs.
type TemplateNotFoundError = workflow.TemplateNotFoundError
