package workflow_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
)

// Feature: C019 - Memory leak fixes with output limiting and streaming

// TestOutputLimits_DefaultValues tests that the default factory returns backward-compatible defaults.
func TestOutputLimits_DefaultValues(t *testing.T) {
	limits := workflow.DefaultOutputLimits()

	assert.Equal(t, int64(0), limits.MaxSize, "MaxSize should default to 0 (unlimited)")
	assert.False(t, limits.StreamLargeOutput, "StreamLargeOutput should default to false (truncate)")
	assert.Empty(t, limits.TempDir, "TempDir should default to empty (system temp)")
}

// TestOutputLimits_CustomConfiguration tests setting custom output limits.
func TestOutputLimits_CustomConfiguration(t *testing.T) {
	tests := []struct {
		name              string
		maxSize           int64
		streamLargeOutput bool
		tempDir           string
	}{
		{
			name:              "1MB limit with truncation",
			maxSize:           1048576, // 1MB
			streamLargeOutput: false,
			tempDir:           "",
		},
		{
			name:              "10MB limit with streaming",
			maxSize:           10485760, // 10MB
			streamLargeOutput: true,
			tempDir:           "/tmp/awf",
		},
		{
			name:              "small 1KB limit",
			maxSize:           1024,
			streamLargeOutput: false,
			tempDir:           "/custom/tmp",
		},
		{
			name:              "unlimited with streaming enabled",
			maxSize:           0,
			streamLargeOutput: true,
			tempDir:           "/var/awf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := workflow.OutputLimits{
				MaxSize:           tt.maxSize,
				StreamLargeOutput: tt.streamLargeOutput,
				TempDir:           tt.tempDir,
			}

			assert.Equal(t, tt.maxSize, limits.MaxSize)
			assert.Equal(t, tt.streamLargeOutput, limits.StreamLargeOutput)
			assert.Equal(t, tt.tempDir, limits.TempDir)
		})
	}
}

// TestOutputLimits_BoundaryValues tests edge cases for MaxSize.
func TestOutputLimits_BoundaryValues(t *testing.T) {
	tests := []struct {
		name    string
		maxSize int64
	}{
		{"zero (unlimited)", 0},
		{"one byte", 1},
		{"exactly 1MB", 1048576},
		{"large 1GB", 1073741824},
		{"negative (invalid but type-safe)", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := workflow.OutputLimits{MaxSize: tt.maxSize}
			assert.Equal(t, tt.maxSize, limits.MaxSize)
		})
	}
}

// TestLoopMemoryConfig_DefaultValues tests that the default factory returns backward-compatible defaults.
func TestLoopMemoryConfig_DefaultValues(t *testing.T) {
	config := workflow.DefaultLoopMemoryConfig()

	assert.Equal(t, 0, config.MaxRetainedIterations, "MaxRetainedIterations should default to 0 (unlimited)")
}

// TestLoopMemoryConfig_CustomRetention tests setting custom iteration retention limits.
func TestLoopMemoryConfig_CustomRetention(t *testing.T) {
	tests := []struct {
		name                  string
		maxRetainedIterations int
	}{
		{"retain last 1 iteration", 1},
		{"retain last 10 iterations", 10},
		{"retain last 100 iterations", 100},
		{"retain last 1000 iterations", 1000},
		{"unlimited retention", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := workflow.LoopMemoryConfig{
				MaxRetainedIterations: tt.maxRetainedIterations,
			}

			assert.Equal(t, tt.maxRetainedIterations, config.MaxRetainedIterations)
		})
	}
}

// TestLoopMemoryConfig_EdgeCases tests boundary conditions for loop retention.
func TestLoopMemoryConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		maxValue int
	}{
		{"zero (unlimited)", 0},
		{"negative (invalid but type-safe)", -1},
		{"very large", 1000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := workflow.LoopMemoryConfig{MaxRetainedIterations: tt.maxValue}
			assert.Equal(t, tt.maxValue, config.MaxRetainedIterations)
		})
	}
}

// TestStepState_OutputStreamingFields tests the new C019 fields on StepState.
func TestStepState_OutputStreamingFields(t *testing.T) {
	tests := []struct {
		name       string
		state      workflow.StepState
		wantOutput string
		wantStderr string
		wantTrunc  bool
	}{
		{
			name: "in-memory output (no streaming)",
			state: workflow.StepState{
				Name:       "step1",
				Status:     workflow.StatusCompleted,
				Output:     "small output",
				Stderr:     "",
				OutputPath: "",
				StderrPath: "",
				Truncated:  false,
			},
			wantOutput: "",
			wantStderr: "",
			wantTrunc:  false,
		},
		{
			name: "truncated output",
			state: workflow.StepState{
				Name:       "step2",
				Status:     workflow.StatusCompleted,
				Output:     "truncated...",
				Stderr:     "",
				OutputPath: "",
				StderrPath: "",
				Truncated:  true,
			},
			wantOutput: "",
			wantStderr: "",
			wantTrunc:  true,
		},
		{
			name: "streamed to file",
			state: workflow.StepState{
				Name:       "step3",
				Status:     workflow.StatusCompleted,
				Output:     "",
				Stderr:     "",
				OutputPath: "/tmp/awf/output-123.txt",
				StderrPath: "/tmp/awf/stderr-123.txt",
				Truncated:  false,
			},
			wantOutput: "/tmp/awf/output-123.txt",
			wantStderr: "/tmp/awf/stderr-123.txt",
			wantTrunc:  false,
		},
		{
			name: "mixed: output streamed, stderr in-memory",
			state: workflow.StepState{
				Name:       "step4",
				Status:     workflow.StatusCompleted,
				Output:     "",
				Stderr:     "small error",
				OutputPath: "/tmp/awf/output-456.txt",
				StderrPath: "",
				Truncated:  false,
			},
			wantOutput: "/tmp/awf/output-456.txt",
			wantStderr: "",
			wantTrunc:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantOutput, tt.state.OutputPath)
			assert.Equal(t, tt.wantStderr, tt.state.StderrPath)
			assert.Equal(t, tt.wantTrunc, tt.state.Truncated)
		})
	}
}

// TestStepState_OutputFieldsZeroValue tests that new fields have safe zero values.
func TestStepState_OutputFieldsZeroValue(t *testing.T) {
	// Create StepState without setting C019 fields
	state := workflow.StepState{
		Name:   "test",
		Status: workflow.StatusPending,
		Output: "some output",
	}

	// Verify zero values are safe (backward compatibility)
	assert.Empty(t, state.OutputPath, "OutputPath should be empty by default")
	assert.Empty(t, state.StderrPath, "StderrPath should be empty by default")
	assert.False(t, state.Truncated, "Truncated should be false by default")
}

// TestStepState_OutputStreamingMutualExclusivity tests that output is either in-memory OR streamed, not both.
func TestStepState_OutputStreamingMutualExclusivity(t *testing.T) {
	tests := []struct {
		name        string
		state       workflow.StepState
		wantError   bool
		errorReason string
	}{
		{
			name: "valid: in-memory only",
			state: workflow.StepState{
				Output:     "data",
				OutputPath: "",
			},
			wantError: false,
		},
		{
			name: "valid: streamed only",
			state: workflow.StepState{
				Output:     "",
				OutputPath: "/tmp/file",
			},
			wantError: false,
		},
		{
			name: "invalid: both in-memory and streamed",
			state: workflow.StepState{
				Output:     "data",
				OutputPath: "/tmp/file",
			},
			wantError:   true,
			errorReason: "Output and OutputPath should be mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test documents expected behavior; actual validation would be in application layer
			hasOutput := tt.state.Output != ""
			hasOutputPath := tt.state.OutputPath != ""
			bothSet := hasOutput && hasOutputPath

			if tt.wantError {
				assert.True(t, bothSet, tt.errorReason)
			} else {
				assert.False(t, bothSet, "should not have both Output and OutputPath set")
			}
		})
	}
}

// TestStepState_TruncatedFlagSemantics tests the meaning of the Truncated flag.
func TestStepState_TruncatedFlagSemantics(t *testing.T) {
	tests := []struct {
		name     string
		state    workflow.StepState
		wantDesc string
	}{
		{
			name: "truncated (no streaming)",
			state: workflow.StepState{
				Output:     "truncated...",
				OutputPath: "",
				Truncated:  true,
			},
			wantDesc: "output was truncated and stored in-memory",
		},
		{
			name: "streamed (not truncated)",
			state: workflow.StepState{
				Output:     "",
				OutputPath: "/tmp/file",
				Truncated:  false,
			},
			wantDesc: "output was streamed to file without truncation",
		},
		{
			name: "small output (no limit hit)",
			state: workflow.StepState{
				Output:     "small",
				OutputPath: "",
				Truncated:  false,
			},
			wantDesc: "output stored in-memory without hitting limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test documents the semantics of Truncated flag
			if tt.state.Truncated {
				assert.NotEmpty(t, tt.state.Output, "truncated output should have partial data in Output field")
				assert.Empty(t, tt.state.OutputPath, "truncated output should not have OutputPath")
			}
			if tt.state.OutputPath != "" {
				assert.False(t, tt.state.Truncated, "streamed output should not be marked as truncated")
			}
		})
	}
}

// TestOutputLimits_ConfigurationScenarios tests realistic configuration scenarios.
func TestOutputLimits_ConfigurationScenarios(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		config   workflow.OutputLimits
	}{
		{
			name:     "development environment (no limits)",
			scenario: "developer wants full output for debugging",
			config: workflow.OutputLimits{
				MaxSize:           0,
				StreamLargeOutput: false,
				TempDir:           "",
			},
		},
		{
			name:     "production CI pipeline (aggressive truncation)",
			scenario: "CI wants to prevent OOM with 100KB limit",
			config: workflow.OutputLimits{
				MaxSize:           102400, // 100KB
				StreamLargeOutput: false,
				TempDir:           "",
			},
		},
		{
			name:     "long-running workflow (streaming)",
			scenario: "long workflow streams large outputs to disk",
			config: workflow.OutputLimits{
				MaxSize:           1048576, // 1MB
				StreamLargeOutput: true,
				TempDir:           "/var/awf/temp",
			},
		},
		{
			name:     "memory-constrained environment",
			scenario: "small memory limit with streaming fallback",
			config: workflow.OutputLimits{
				MaxSize:           524288, // 512KB
				StreamLargeOutput: true,
				TempDir:           "/tmp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify configuration is valid and matches scenario
			assert.GreaterOrEqual(t, tt.config.MaxSize, int64(0))
			if tt.config.StreamLargeOutput {
				// If streaming is enabled, we should have a reasonable size limit
				assert.Greater(t, tt.config.MaxSize, int64(0),
					"streaming requires a MaxSize threshold")
			}
		})
	}
}

// TestLoopMemoryConfig_RetentionScenarios tests realistic retention scenarios.
func TestLoopMemoryConfig_RetentionScenarios(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		config   workflow.LoopMemoryConfig
	}{
		{
			name:     "unlimited retention (default)",
			scenario: "short loops (<100 iterations) keep all results",
			config:   workflow.LoopMemoryConfig{MaxRetainedIterations: 0},
		},
		{
			name:     "rolling window (last 10)",
			scenario: "keep only last 10 iterations for debugging",
			config:   workflow.LoopMemoryConfig{MaxRetainedIterations: 10},
		},
		{
			name:     "minimal retention (last 1)",
			scenario: "only current iteration needed, prune aggressively",
			config:   workflow.LoopMemoryConfig{MaxRetainedIterations: 1},
		},
		{
			name:     "moderate retention (last 100)",
			scenario: "balance memory and debuggability for 1000+ iteration loops",
			config:   workflow.LoopMemoryConfig{MaxRetainedIterations: 100},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify configuration is valid
			assert.GreaterOrEqual(t, tt.config.MaxRetainedIterations, 0)
		})
	}
}
