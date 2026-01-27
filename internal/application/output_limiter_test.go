package application_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// Feature: C019 - Output truncation to prevent OOM

// TestNewOutputLimiter tests creating an output limiter with various configurations.
func TestNewOutputLimiter(t *testing.T) {
	tests := []struct {
		name   string
		config workflow.OutputLimits
	}{
		{
			name:   "default config",
			config: workflow.DefaultOutputLimits(),
		},
		{
			name: "1MB limit with truncation",
			config: workflow.OutputLimits{
				MaxSize:           1048576,
				StreamLargeOutput: false,
				TempDir:           "",
			},
		},
		{
			name: "10MB limit with streaming",
			config: workflow.OutputLimits{
				MaxSize:           10485760,
				StreamLargeOutput: true,
				TempDir:           "/tmp/awf",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := application.NewOutputLimiter(tt.config)
			require.NotNil(t, limiter)
			assert.Equal(t, tt.config, limiter.Config())
		})
	}
}

// TestOutputLimiter_ShouldLimit_UnlimitedConfig tests that unlimited config never triggers limits.
func TestOutputLimiter_ShouldLimit_UnlimitedConfig(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           0, // unlimited
		StreamLargeOutput: false,
	}
	limiter := application.NewOutputLimiter(config)

	tests := []struct {
		name   string
		output string
	}{
		{"empty", ""},
		{"small", "hello"},
		{"1KB", strings.Repeat("a", 1024)},
		{"1MB", strings.Repeat("a", 1048576)},
		{"10MB", strings.Repeat("a", 10485760)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, limiter.ShouldLimit(tt.output),
				"unlimited config should never trigger limit")
		})
	}
}

// TestOutputLimiter_ShouldLimit_WithLimit tests limit detection with configured size.
func TestOutputLimiter_ShouldLimit_WithLimit(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           1024, // 1KB limit
		StreamLargeOutput: false,
	}
	limiter := application.NewOutputLimiter(config)

	tests := []struct {
		name        string
		output      string
		shouldLimit bool
	}{
		{"empty", "", false},
		{"small (100 bytes)", strings.Repeat("a", 100), false},
		{"exactly at limit (1024 bytes)", strings.Repeat("a", 1024), false},
		{"just over limit (1025 bytes)", strings.Repeat("a", 1025), true},
		{"double limit (2048 bytes)", strings.Repeat("a", 2048), true},
		{"large (10KB)", strings.Repeat("a", 10240), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := limiter.ShouldLimit(tt.output)
			assert.Equal(t, tt.shouldLimit, result)
		})
	}
}

// TestOutputLimiter_Truncate_BasicCases tests basic truncation behavior.
func TestOutputLimiter_Truncate_BasicCases(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100, // 100 byte limit
		StreamLargeOutput: false,
	}
	limiter := application.NewOutputLimiter(config)

	tests := []struct {
		name            string
		input           string
		wantMaxLen      int
		wantTruncMarker bool
	}{
		{
			name:            "empty string",
			input:           "",
			wantMaxLen:      0,
			wantTruncMarker: false,
		},
		{
			name:            "small string (no truncation)",
			input:           "hello world",
			wantMaxLen:      11,
			wantTruncMarker: false,
		},
		{
			name:            "exactly at limit",
			input:           strings.Repeat("a", 100),
			wantMaxLen:      100,
			wantTruncMarker: false,
		},
		{
			name:            "over limit (101 bytes)",
			input:           strings.Repeat("a", 101),
			wantMaxLen:      100,
			wantTruncMarker: true,
		},
		{
			name:            "large string (1KB)",
			input:           strings.Repeat("x", 1024),
			wantMaxLen:      100,
			wantTruncMarker: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := limiter.Truncate(tt.input)
			assert.LessOrEqual(t, len(result), tt.wantMaxLen,
				"truncated output should not exceed limit")

			if tt.wantTruncMarker {
				assert.Contains(t, result, "...[truncated]",
					"truncated output should contain marker")
			} else {
				assert.Equal(t, tt.input, result,
					"small output should not be modified")
			}
		})
	}
}

// TestOutputLimiter_Truncate_PreservesPrefix tests that truncation keeps the beginning of output.
func TestOutputLimiter_Truncate_PreservesPrefix(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           50, // small limit to test prefix preservation
		StreamLargeOutput: false,
	}
	limiter := application.NewOutputLimiter(config)

	input := "IMPORTANT_PREFIX: followed by lots of data that will be truncated away"
	result := limiter.Truncate(input)

	assert.Contains(t, result, "IMPORTANT_PREFIX",
		"truncation should preserve beginning of output for debugging")
	assert.LessOrEqual(t, len(result), 50)
}

// TestOutputLimiter_Apply_NoLimits tests that Apply passes through when no limits configured.
func TestOutputLimiter_Apply_NoLimits(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           0, // unlimited
		StreamLargeOutput: false,
	}
	limiter := application.NewOutputLimiter(config)

	tests := []struct {
		name   string
		output string
		stderr string
	}{
		{"empty", "", ""},
		{"small", "hello", "world"},
		{"large output", strings.Repeat("a", 10000), ""},
		{"large stderr", "", strings.Repeat("b", 10000)},
		{"both large", strings.Repeat("x", 5000), strings.Repeat("y", 5000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := limiter.Apply(tt.output, tt.stderr)
			require.NoError(t, err)

			assert.Equal(t, tt.output, result.Output, "output should pass through unchanged")
			assert.Equal(t, tt.stderr, result.Stderr, "stderr should pass through unchanged")
			assert.Empty(t, result.OutputPath, "should not stream when unlimited")
			assert.Empty(t, result.StderrPath, "should not stream when unlimited")
			assert.False(t, result.Truncated, "should not truncate when unlimited")
		})
	}
}

// TestOutputLimiter_Apply_TruncationMode tests truncation when StreamLargeOutput is false.
func TestOutputLimiter_Apply_TruncationMode(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           1024, // 1KB limit
		StreamLargeOutput: false,
		TempDir:           "",
	}
	limiter := application.NewOutputLimiter(config)

	tests := []struct {
		name        string
		output      string
		stderr      string
		expectTrunc bool
	}{
		{
			name:        "small output (no truncation)",
			output:      "short output",
			stderr:      "short error",
			expectTrunc: false,
		},
		{
			name:        "output exceeds limit",
			output:      strings.Repeat("a", 2000),
			stderr:      "short error",
			expectTrunc: true,
		},
		{
			name:        "stderr exceeds limit",
			output:      "short output",
			stderr:      strings.Repeat("e", 2000),
			expectTrunc: true,
		},
		{
			name:        "both exceed limit",
			output:      strings.Repeat("o", 2000),
			stderr:      strings.Repeat("e", 2000),
			expectTrunc: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := limiter.Apply(tt.output, tt.stderr)
			require.NoError(t, err)

			// In truncation mode, should not create temp files
			assert.Empty(t, result.OutputPath, "should not stream in truncation mode")
			assert.Empty(t, result.StderrPath, "should not stream in truncation mode")

			// Check truncation flag
			assert.Equal(t, tt.expectTrunc, result.Truncated)

			// Check output sizes
			if tt.expectTrunc {
				if len(tt.output) > 1024 {
					assert.LessOrEqual(t, len(result.Output), 1024,
						"output should be truncated to limit")
					assert.Contains(t, result.Output, "...[truncated]",
						"truncated output should have marker")
				}
				if len(tt.stderr) > 1024 {
					assert.LessOrEqual(t, len(result.Stderr), 1024,
						"stderr should be truncated to limit")
					assert.Contains(t, result.Stderr, "...[truncated]",
						"truncated stderr should have marker")
				}
			} else {
				assert.Equal(t, tt.output, result.Output)
				assert.Equal(t, tt.stderr, result.Stderr)
			}
		})
	}
}

// TestOutputLimiter_Apply_StreamingMode tests streaming when StreamLargeOutput is true.
func TestOutputLimiter_Apply_StreamingMode(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           1024, // 1KB limit
		StreamLargeOutput: true,
		TempDir:           t.TempDir(), // use test temp directory
	}
	limiter := application.NewOutputLimiter(config)

	tests := []struct {
		name             string
		output           string
		stderr           string
		expectOutputPath bool
		expectStderrPath bool
	}{
		{
			name:             "small output (no streaming)",
			output:           "short output",
			stderr:           "short error",
			expectOutputPath: false,
			expectStderrPath: false,
		},
		{
			name:             "output exceeds limit (stream)",
			output:           strings.Repeat("a", 2000),
			stderr:           "short error",
			expectOutputPath: true,
			expectStderrPath: false,
		},
		{
			name:             "stderr exceeds limit (stream)",
			output:           "short output",
			stderr:           strings.Repeat("e", 2000),
			expectOutputPath: false,
			expectStderrPath: true,
		},
		{
			name:             "both exceed limit (stream both)",
			output:           strings.Repeat("o", 2000),
			stderr:           strings.Repeat("e", 2000),
			expectOutputPath: true,
			expectStderrPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := limiter.Apply(tt.output, tt.stderr)
			require.NoError(t, err)

			// In streaming mode, should not truncate
			assert.False(t, result.Truncated, "should not truncate in streaming mode")

			// Check output path
			if tt.expectOutputPath {
				assert.NotEmpty(t, result.OutputPath, "should create output temp file")
				assert.Empty(t, result.Output, "output should be empty when streamed")
				// TODO(#149): Verify file exists and contains correct content
			} else {
				assert.Empty(t, result.OutputPath, "should not stream small output")
				assert.Equal(t, tt.output, result.Output)
			}

			// Check stderr path
			if tt.expectStderrPath {
				assert.NotEmpty(t, result.StderrPath, "should create stderr temp file")
				assert.Empty(t, result.Stderr, "stderr should be empty when streamed")
				// TODO(#149): Verify file exists and contains correct content
			} else {
				assert.Empty(t, result.StderrPath, "should not stream small stderr")
				assert.Equal(t, tt.stderr, result.Stderr)
			}
		})
	}
}

// TestOutputLimiter_Apply_BoundaryValues tests edge cases around the limit boundary.
func TestOutputLimiter_Apply_BoundaryValues(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: false,
	}
	limiter := application.NewOutputLimiter(config)

	tests := []struct {
		name        string
		outputSize  int
		expectTrunc bool
	}{
		{"0 bytes", 0, false},
		{"1 byte", 1, false},
		{"99 bytes", 99, false},
		{"exactly 100 bytes", 100, false},
		{"101 bytes", 101, true},
		{"102 bytes", 102, true},
		{"1000 bytes", 1000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := strings.Repeat("x", tt.outputSize)
			result, err := limiter.Apply(output, "")
			require.NoError(t, err)

			if tt.expectTrunc {
				assert.True(t, result.Truncated)
				assert.LessOrEqual(t, len(result.Output), 100)
			} else {
				assert.False(t, result.Truncated)
				assert.Equal(t, output, result.Output)
			}
		})
	}
}

// TestOutputLimiter_Apply_IndependentLimiting tests that output and stderr are limited independently.
func TestOutputLimiter_Apply_IndependentLimiting(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: false,
	}
	limiter := application.NewOutputLimiter(config)

	output := strings.Repeat("o", 150) // exceeds limit
	stderr := "small error"            // within limit

	result, err := limiter.Apply(output, stderr)
	require.NoError(t, err)

	assert.True(t, result.Truncated, "should truncate because output exceeds")
	assert.LessOrEqual(t, len(result.Output), 100, "output should be truncated")
	assert.Equal(t, stderr, result.Stderr, "stderr should pass through unchanged")
}

// TestOutputLimiter_Apply_ErrorHandling tests error cases.
func TestOutputLimiter_Apply_ErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		config  workflow.OutputLimits
		output  string
		stderr  string
		wantErr bool
	}{
		{
			name: "valid config",
			config: workflow.OutputLimits{
				MaxSize:           1024,
				StreamLargeOutput: false,
			},
			output:  "test",
			stderr:  "test",
			wantErr: false,
		},
		{
			name: "streaming with invalid temp dir (should handle gracefully)",
			config: workflow.OutputLimits{
				MaxSize:           100,
				StreamLargeOutput: true,
				TempDir:           "/nonexistent/dir/that/cannot/be/created",
			},
			output:  strings.Repeat("x", 200),
			stderr:  "",
			wantErr: true, // should error when cannot create temp file
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := application.NewOutputLimiter(tt.config)
			result, err := limiter.Apply(tt.output, tt.stderr)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestOutputLimiter_Truncate_Unicode tests truncation with multibyte Unicode characters.
func TestOutputLimiter_Truncate_Unicode(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           50,
		StreamLargeOutput: false,
	}
	limiter := application.NewOutputLimiter(config)

	// Unicode string with emoji and multibyte characters
	input := "Hello 世界 🌍 " + strings.Repeat("x", 100)
	result := limiter.Truncate(input)

	// Should not corrupt UTF-8 by cutting in the middle of a multibyte char
	assert.True(t, len(result) <= 50, "should respect byte limit")
	// Note: Proper implementation should ensure valid UTF-8
}

// TestOutputLimiter_Config tests retrieving the configuration.
func TestOutputLimiter_Config(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           2048,
		StreamLargeOutput: true,
		TempDir:           "/tmp/test",
	}
	limiter := application.NewOutputLimiter(config)

	retrieved := limiter.Config()
	assert.Equal(t, config, retrieved)
}
