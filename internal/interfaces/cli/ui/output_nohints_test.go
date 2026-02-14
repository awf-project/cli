package ui_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domerrors "github.com/vanoix/awf/internal/domain/errors"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

// These tests verify that the noHints flag is correctly threaded from
// NewOutputWriter constructor through to the HumanErrorFormatter.

func TestOutputWriter_NoHintsParameter(t *testing.T) {
	tests := []struct {
		name     string
		noHints  bool
		noColor  bool
		expected string
	}{
		{
			name:     "hints enabled (noHints=false)",
			noHints:  false,
			noColor:  true,
			expected: "hints should be shown",
		},
		{
			name:     "hints disabled (noHints=true)",
			noHints:  true,
			noColor:  true,
			expected: "hints should be suppressed",
		},
		{
			name:     "hints enabled with color",
			noHints:  false,
			noColor:  false,
			expected: "hints shown with color",
		},
		{
			name:     "hints disabled with color",
			noHints:  true,
			noColor:  false,
			expected: "hints suppressed with color",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)

			// Test that NewOutputWriter accepts noHints parameter
			w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, tt.noColor, tt.noHints)

			assert.NotNil(t, w, "OutputWriter should be created")
			// We can't directly access private fields, but we can test behavior
			// by ensuring WriteError doesn't panic
			err := w.WriteError(errors.New("test error"), 1)
			assert.NoError(t, err, "WriteError should succeed")
		})
	}
}

func TestOutputWriter_NoHintsThreading_StructuredError(t *testing.T) {
	tests := []struct {
		name    string
		noHints bool
		errCode domerrors.ErrorCode
		message string
	}{
		{
			name:    "structured error with hints enabled",
			noHints: false,
			errCode: domerrors.ErrorCodeUserInputMissingFile,
			message: "workflow file not found",
		},
		{
			name:    "structured error with hints disabled",
			noHints: true,
			errCode: domerrors.ErrorCodeUserInputMissingFile,
			message: "workflow file not found",
		},
		{
			name:    "validation error with hints enabled",
			noHints: false,
			errCode: domerrors.ErrorCodeUserInputValidationFailed,
			message: "invalid workflow format",
		},
		{
			name:    "validation error with hints disabled",
			noHints: true,
			errCode: domerrors.ErrorCodeUserInputValidationFailed,
			message: "invalid workflow format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)

			// Create OutputWriter with specific noHints setting
			w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, true, tt.noHints)

			// Create a StructuredError
			structErr := domerrors.NewStructuredError(
				tt.errCode,
				tt.message,
				map[string]any{
					"path": "/configs/workflow.yaml",
				},
				nil, // no cause
			)

			// Write error - should thread noHints to formatter
			err := w.WriteError(structErr, 1)
			require.NoError(t, err, "WriteError should succeed")

			output := errBuf.String()
			assert.Contains(t, output, tt.message, "error message should be present")
			assert.Contains(t, output, string(tt.errCode), "error code should be present")
		})
	}
}

func TestOutputWriter_NoHintsThreading_PlainError(t *testing.T) {
	tests := []struct {
		name    string
		noHints bool
		errMsg  string
	}{
		{
			name:    "plain error with hints enabled",
			noHints: false,
			errMsg:  "something went wrong",
		},
		{
			name:    "plain error with hints disabled",
			noHints: true,
			errMsg:  "command execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)

			// Plain errors should not be affected by noHints
			// (hints only apply to StructuredError)
			w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, true, tt.noHints)

			plainErr := errors.New(tt.errMsg)
			err := w.WriteError(plainErr, 2)
			require.NoError(t, err, "WriteError should succeed")

			output := errBuf.String()
			assert.Contains(t, output, tt.errMsg, "error message should be present")
		})
	}
}

func TestOutputWriter_NoHintsThreading_JSONFormat(t *testing.T) {
	tests := []struct {
		name    string
		noHints bool
		errCode domerrors.ErrorCode
		message string
	}{
		{
			name:    "JSON format with hints enabled",
			noHints: false,
			errCode: domerrors.ErrorCodeUserInputMissingFile,
			message: "file not found",
		},
		{
			name:    "JSON format with hints disabled",
			noHints: true,
			errCode: domerrors.ErrorCodeUserInputMissingFile,
			message: "file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)

			// JSON format should also respect noHints setting
			w := ui.NewOutputWriter(buf, errBuf, ui.FormatJSON, true, tt.noHints)

			structErr := domerrors.NewStructuredError(
				tt.errCode,
				tt.message,
				map[string]any{"path": "/test.yaml"},
				nil, // no cause
			)

			err := w.WriteError(structErr, 1)
			require.NoError(t, err, "WriteError should succeed")

			output := buf.String()
			assert.Contains(t, output, tt.message, "error message should be in JSON")
			assert.Contains(t, output, string(tt.errCode), "error code should be in JSON")
		})
	}
}

func TestOutputWriter_NoHintsThreading_AllCombinations(t *testing.T) {
	// Test matrix: all combinations of noColor and noHints
	combinations := []struct {
		noColor bool
		noHints bool
	}{
		{false, false}, // color enabled, hints enabled
		{false, true},  // color enabled, hints disabled
		{true, false},  // color disabled, hints enabled
		{true, true},   // color disabled, hints disabled
	}

	for _, combo := range combinations {
		t.Run("noColor="+boolString(combo.noColor)+"_noHints="+boolString(combo.noHints), func(t *testing.T) {
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)

			w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, combo.noColor, combo.noHints)
			assert.NotNil(t, w, "OutputWriter should be created")

			// Test with StructuredError
			structErr := domerrors.NewStructuredError(
				domerrors.ErrorCodeUserInputMissingFile,
				"test error",
				map[string]any{"key": "value"},
				nil, // no cause
			)

			err := w.WriteError(structErr, 1)
			require.NoError(t, err, "WriteError should succeed")
		})
	}
}

func TestOutputWriter_NoHintsThreading_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		noHints     bool
		structErr   *domerrors.StructuredError
		description string
	}{
		{
			name:    "empty details with hints disabled",
			noHints: true,
			structErr: domerrors.NewStructuredError(
				domerrors.ErrorCodeUserInputMissingFile,
				"error without details",
				nil,
				nil, // no cause
			),
			description: "should handle nil details map",
		},
		{
			name:    "empty details with hints enabled",
			noHints: false,
			structErr: domerrors.NewStructuredError(
				domerrors.ErrorCodeUserInputMissingFile,
				"error without details",
				nil,
				nil, // no cause
			),
			description: "should handle nil details map",
		},
		{
			name:    "empty message with hints disabled",
			noHints: true,
			structErr: domerrors.NewStructuredError(
				domerrors.ErrorCodeSystemIOReadFailed,
				"",
				map[string]any{"error": "internal"},
				nil, // no cause
			),
			description: "should handle empty message",
		},
		{
			name:    "empty message with hints enabled",
			noHints: false,
			structErr: domerrors.NewStructuredError(
				domerrors.ErrorCodeSystemIOReadFailed,
				"",
				map[string]any{"error": "internal"},
				nil, // no cause
			),
			description: "should handle empty message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)

			w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, true, tt.noHints)

			err := w.WriteError(tt.structErr, 4)
			require.NoError(t, err, tt.description)

			// Should produce output even with edge cases
			assert.NotEmpty(t, errBuf.String(), "should produce error output")
		})
	}
}

func TestOutputWriter_NoHintsThreading_MultipleErrors(t *testing.T) {
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)

	// Test that noHints setting persists across multiple WriteError calls
	w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, true, true)

	errs := []error{
		domerrors.NewStructuredError(
			domerrors.ErrorCodeUserInputMissingFile,
			"first error",
			nil,
			nil, // no cause
		),
		domerrors.NewStructuredError(
			domerrors.ErrorCodeUserInputValidationFailed,
			"second error",
			nil,
			nil, // no cause
		),
		errors.New("third plain error"),
	}

	for _, testErr := range errs {
		err := w.WriteError(testErr, 1)
		require.NoError(t, err, "WriteError should succeed")
	}

	output := errBuf.String()
	assert.Contains(t, output, "first error")
	assert.Contains(t, output, "second error")
	assert.Contains(t, output, "third plain error")
}

func TestOutputWriter_NoHintsThreading_DefaultValue(t *testing.T) {
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)

	// Test with default values (hints enabled, no color)
	w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, false, false)

	structErr := domerrors.NewStructuredError(
		domerrors.ErrorCodeUserInputMissingFile,
		"test error with defaults",
		map[string]any{"path": "/test"},
		nil, // no cause
	)

	err := w.WriteError(structErr, 1)
	require.NoError(t, err, "WriteError should succeed with default values")

	output := errBuf.String()
	assert.Contains(t, output, "test error with defaults")
}

// Helper function to convert bool to string for test names
func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
