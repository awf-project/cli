package interpolation_test

import (
	"errors"
	"testing"

	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RED Phase: Test stubs for interpolation error types
// These tests will compile but fail when run - implementation validation needed

func TestUndefinedVariableError_Error(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		want     string
	}{
		{
			name:     "simple variable name",
			variable: "inputs.name",
			want:     "undefined variable: inputs.name",
		},
		{
			name:     "nested variable path",
			variable: "states.fetch.output",
			want:     "undefined variable: states.fetch.output",
		},
		{
			name:     "empty variable",
			variable: "",
			want:     "undefined variable: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &interpolation.UndefinedVariableError{Variable: tt.variable}
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

func TestParseError_Error(t *testing.T) {
	tests := []struct {
		name     string
		template string
		cause    error
		wantMsg  string
	}{
		{
			name:     "unclosed template",
			template: "{{.inputs.name",
			cause:    errors.New("unexpected EOF"),
			wantMsg:  "failed to parse template: unexpected EOF",
		},
		{
			name:     "invalid syntax",
			template: "{{if}}",
			cause:    errors.New("missing condition"),
			wantMsg:  "failed to parse template: missing condition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &interpolation.ParseError{
				Template: tt.template,
				Cause:    tt.cause,
			}
			assert.Contains(t, err.Error(), "failed to parse template")
			assert.Contains(t, err.Error(), tt.cause.Error())
		})
	}
}

func TestParseError_Unwrap(t *testing.T) {
	causeErr := errors.New("underlying error")
	parseErr := &interpolation.ParseError{
		Template: "{{.broken}",
		Cause:    causeErr,
	}

	unwrapped := parseErr.Unwrap()
	require.NotNil(t, unwrapped)
	assert.Equal(t, causeErr, unwrapped)

	// Verify errors.Is/As compatibility
	assert.True(t, errors.Is(parseErr, causeErr))
}
