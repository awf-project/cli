package ui_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

// formatFieldIfPresent Tests (C004)
// Feature: C004 - Extract field formatters to reduce formatStep complexity

func TestFormatFieldIfPresent_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		label      string
		value      string
		wantOutput string
	}{
		{
			name:       "formats simple field",
			label:      "Dir",
			value:      "/tmp/work",
			wantOutput: "    Dir: /tmp/work\n",
		},
		{
			name:       "formats field with special chars",
			label:      "Command",
			value:      "echo 'hello world'",
			wantOutput: "    Command: echo 'hello world'\n",
		},
		{
			name:       "formats field with newlines",
			label:      "Note",
			value:      "line1\nline2",
			wantOutput: "    Note: line1\nline2\n",
		},
		{
			name:       "formats timeout field",
			label:      "Timeout",
			value:      "30s",
			wantOutput: "    Timeout: 30s\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewDryRunFormatter(buf, false)

			err := formatter.FormatFieldIfPresent(tt.label, tt.value)

			require.NoError(t, err, "should format field without error")
			assert.Equal(t, tt.wantOutput, buf.String(), "output should match expected format")
		})
	}
}

func TestFormatFieldIfPresent_EmptyValue(t *testing.T) {
	tests := []struct {
		name        string
		label       string
		value       string
		shouldWrite bool
	}{
		{
			name:        "skips empty string",
			label:       "Dir",
			value:       "",
			shouldWrite: false,
		},
		{
			name:        "writes zero value",
			label:       "Count",
			value:       "0",
			shouldWrite: true,
		},
		{
			name:        "writes whitespace only",
			label:       "Note",
			value:       "   ",
			shouldWrite: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewDryRunFormatter(buf, false)

			err := formatter.FormatFieldIfPresent(tt.label, tt.value)

			require.NoError(t, err)
			if tt.shouldWrite {
				assert.NotEmpty(t, buf.String(), "should write non-empty value")
			} else {
				assert.Empty(t, buf.String(), "should not write empty value")
			}
		})
	}
}

func TestFormatFieldIfPresent_WriterError(t *testing.T) {
	// Use failing writer to test error handling
	failWriter := &failingWriter{err: errors.New("write failed")}
	formatter := ui.NewDryRunFormatterWithWriter(failWriter, false)

	err := formatter.FormatFieldIfPresent("Label", "value")

	assert.Error(t, err, "should propagate writer error")
	assert.Contains(t, err.Error(), "write failed", "error should include underlying cause")
}

func TestFormatFieldIfPresent_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		label      string
		value      string
		wantOutput string
	}{
		{
			name:       "empty label",
			label:      "",
			value:      "value",
			wantOutput: "    : value\n",
		},
		{
			name:       "very long value",
			label:      "Path",
			value:      strings.Repeat("a", 1000),
			wantOutput: "    Path: " + strings.Repeat("a", 1000) + "\n",
		},
		{
			name:       "unicode in label and value",
			label:      "文件",
			value:      "データ",
			wantOutput: "    文件: データ\n",
		},
		{
			name:       "value with tabs",
			label:      "Config",
			value:      "key\tvalue",
			wantOutput: "    Config: key\tvalue\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewDryRunFormatter(buf, false)

			err := formatter.FormatFieldIfPresent(tt.label, tt.value)

			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

// formatRetry Tests (C004)

func TestFormatRetry_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		retry      *workflow.DryRunRetry
		wantOutput string
	}{
		{
			name: "formats exponential backoff",
			retry: &workflow.DryRunRetry{
				MaxAttempts:    3,
				InitialDelayMs: 100,
				MaxDelayMs:     1000,
				Backoff:        "exponential",
				Multiplier:     2.0,
			},
			wantOutput: "    Retry: 3 attempts, exponential backoff\n",
		},
		{
			name: "formats linear backoff",
			retry: &workflow.DryRunRetry{
				MaxAttempts:    5,
				InitialDelayMs: 500,
				MaxDelayMs:     5000,
				Backoff:        "linear",
				Multiplier:     1.0,
			},
			wantOutput: "    Retry: 5 attempts, linear backoff\n",
		},
		{
			name: "formats constant backoff",
			retry: &workflow.DryRunRetry{
				MaxAttempts:    10,
				InitialDelayMs: 1000,
				MaxDelayMs:     1000,
				Backoff:        "constant",
				Multiplier:     1.0,
			},
			wantOutput: "    Retry: 10 attempts, constant backoff\n",
		},
		{
			name: "single attempt",
			retry: &workflow.DryRunRetry{
				MaxAttempts: 1,
				Backoff:     "none",
			},
			wantOutput: "    Retry: 1 attempts, none backoff\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewDryRunFormatter(buf, false)

			err := formatter.FormatRetry(tt.retry)

			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

func TestFormatRetry_NilRetry(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	err := formatter.FormatRetry(nil)

	require.NoError(t, err, "should handle nil retry gracefully")
	assert.Empty(t, buf.String(), "should not write anything for nil retry")
}

func TestFormatRetry_WriterError(t *testing.T) {
	failWriter := &failingWriter{err: errors.New("write error")}
	formatter := ui.NewDryRunFormatterWithWriter(failWriter, false)

	retry := &workflow.DryRunRetry{
		MaxAttempts: 3,
		Backoff:     "exponential",
	}

	err := formatter.FormatRetry(retry)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write error")
}

func TestFormatRetry_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		retry      *workflow.DryRunRetry
		wantOutput string
	}{
		{
			name: "zero max attempts",
			retry: &workflow.DryRunRetry{
				MaxAttempts: 0,
				Backoff:     "none",
			},
			wantOutput: "    Retry: 0 attempts, none backoff\n",
		},
		{
			name: "empty backoff strategy",
			retry: &workflow.DryRunRetry{
				MaxAttempts: 3,
				Backoff:     "",
			},
			wantOutput: "    Retry: 3 attempts,  backoff\n",
		},
		{
			name: "very large max attempts",
			retry: &workflow.DryRunRetry{
				MaxAttempts: 99999,
				Backoff:     "exponential",
			},
			wantOutput: "    Retry: 99999 attempts, exponential backoff\n",
		},
		{
			name: "negative max attempts (invalid but should handle)",
			retry: &workflow.DryRunRetry{
				MaxAttempts: -1,
				Backoff:     "exponential",
			},
			wantOutput: "    Retry: -1 attempts, exponential backoff\n",
		},
		{
			name: "custom backoff strategy",
			retry: &workflow.DryRunRetry{
				MaxAttempts: 5,
				Backoff:     "jittered_exponential",
			},
			wantOutput: "    Retry: 5 attempts, jittered_exponential backoff\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewDryRunFormatter(buf, false)

			err := formatter.FormatRetry(tt.retry)

			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

// formatCapture Tests (C004)

func TestFormatCapture_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		capture    *workflow.DryRunCapture
		wantOutput string
	}{
		{
			name: "captures stdout only",
			capture: &workflow.DryRunCapture{
				Stdout: "output_var",
				Stderr: "",
			},
			wantOutput: "    Capture: stdout -> output_var\n",
		},
		{
			name: "captures stderr only",
			capture: &workflow.DryRunCapture{
				Stdout: "",
				Stderr: "error_var",
			},
			wantOutput: "    Capture: stderr -> error_var\n",
		},
		{
			name: "captures both stdout and stderr",
			capture: &workflow.DryRunCapture{
				Stdout: "out",
				Stderr: "err",
			},
			wantOutput: "    Capture: stdout -> out, stderr -> err\n",
		},
		{
			name: "captures with max size",
			capture: &workflow.DryRunCapture{
				Stdout:  "result",
				MaxSize: "1MB",
			},
			wantOutput: "    Capture: stdout -> result\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewDryRunFormatter(buf, false)

			err := formatter.FormatCapture(tt.capture)

			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

func TestFormatCapture_NilCapture(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	err := formatter.FormatCapture(nil)

	require.NoError(t, err, "should handle nil capture gracefully")
	assert.Empty(t, buf.String(), "should not write anything for nil capture")
}

func TestFormatCapture_EmptyCapture(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	capture := &workflow.DryRunCapture{
		Stdout: "",
		Stderr: "",
	}

	err := formatter.FormatCapture(capture)

	require.NoError(t, err)
	assert.Empty(t, buf.String(), "should not write anything when both streams empty")
}

func TestFormatCapture_WriterError(t *testing.T) {
	failWriter := &failingWriter{err: errors.New("io error")}
	formatter := ui.NewDryRunFormatterWithWriter(failWriter, false)

	capture := &workflow.DryRunCapture{
		Stdout: "out",
	}

	err := formatter.FormatCapture(capture)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "io error")
}

func TestFormatCapture_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		capture    *workflow.DryRunCapture
		wantOutput string
	}{
		{
			name: "long variable names",
			capture: &workflow.DryRunCapture{
				Stdout: "very_long_variable_name_for_output_data",
				Stderr: "very_long_variable_name_for_error_data",
			},
			wantOutput: "    Capture: stdout -> very_long_variable_name_for_output_data, stderr -> very_long_variable_name_for_error_data\n",
		},
		{
			name: "special characters in variable names",
			capture: &workflow.DryRunCapture{
				Stdout: "output.result",
				Stderr: "error_msg",
			},
			wantOutput: "    Capture: stdout -> output.result, stderr -> error_msg\n",
		},
		{
			name: "unicode variable names",
			capture: &workflow.DryRunCapture{
				Stdout: "結果",
				Stderr: "エラー",
			},
			wantOutput: "    Capture: stdout -> 結果, stderr -> エラー\n",
		},
		{
			name: "whitespace only variable names",
			capture: &workflow.DryRunCapture{
				Stdout: "   ",
				Stderr: "  ",
			},
			wantOutput: "    Capture: stdout ->    , stderr ->   \n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewDryRunFormatter(buf, false)

			err := formatter.FormatCapture(tt.capture)

			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

func TestFormatCapture_OutputOrdering(t *testing.T) {
	// Test that stdout is always listed before stderr
	capture := &workflow.DryRunCapture{
		Stderr: "err_first",
		Stdout: "out_second",
	}

	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	err := formatter.FormatCapture(capture)

	require.NoError(t, err)
	output := buf.String()

	// Find positions of stdout and stderr in output
	stdoutPos := strings.Index(output, "stdout")
	stderrPos := strings.Index(output, "stderr")

	assert.True(t, stdoutPos < stderrPos, "stdout should appear before stderr in output")
}

// failingWriter is a test helper that always returns an error on Write.
type failingWriter struct {
	err error
}

func (fw *failingWriter) Write(p []byte) (n int, err error) {
	return 0, fw.err
}

// Ensure failingWriter implements io.Writer
var _ io.Writer = (*failingWriter)(nil)
