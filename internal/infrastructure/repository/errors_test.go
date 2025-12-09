package repository

import (
	"errors"
	"testing"
)

func TestParseError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ParseError
		want string
	}{
		{
			name: "with line and column",
			err: &ParseError{
				File:    "workflow.yaml",
				Line:    10,
				Column:  5,
				Field:   "states.validate",
				Message: "invalid type",
			},
			want: "workflow.yaml:10:5: states.validate: invalid type",
		},
		{
			name: "with field only",
			err: &ParseError{
				File:    "workflow.yaml",
				Line:    -1,
				Column:  -1,
				Field:   "name",
				Message: "required field missing",
			},
			want: "workflow.yaml: name: required field missing",
		},
		{
			name: "message only",
			err: &ParseError{
				File:    "workflow.yaml",
				Line:    -1,
				Column:  -1,
				Message: "invalid YAML syntax",
			},
			want: "workflow.yaml: invalid YAML syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("ParseError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &ParseError{
		File:    "test.yaml",
		Message: "wrapped",
		Cause:   cause,
	}

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestNewParseError(t *testing.T) {
	err := NewParseError("test.yaml", "states.start", "missing command")

	if err.File != "test.yaml" {
		t.Errorf("File = %q, want %q", err.File, "test.yaml")
	}
	if err.Field != "states.start" {
		t.Errorf("Field = %q, want %q", err.Field, "states.start")
	}
	if err.Line != -1 {
		t.Errorf("Line = %d, want -1", err.Line)
	}
}

func TestWrapParseError(t *testing.T) {
	cause := errors.New("yaml: line 5: mapping values are not allowed")
	err := WrapParseError("test.yaml", cause)

	if err.Cause != cause {
		t.Error("Cause not set correctly")
	}
	if err.Message != cause.Error() {
		t.Errorf("Message = %q, want %q", err.Message, cause.Error())
	}
}

func TestParseErrorWithLine(t *testing.T) {
	err := ParseErrorWithLine("test.yaml", 42, 10, "unexpected token")

	if err.Line != 42 {
		t.Errorf("Line = %d, want 42", err.Line)
	}
	if err.Column != 10 {
		t.Errorf("Column = %d, want 10", err.Column)
	}
}
