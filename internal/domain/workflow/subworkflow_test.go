package workflow

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubWorkflowConstants(t *testing.T) {
	assert.Equal(t, 300, DefaultSubWorkflowTimeout)
	assert.Equal(t, 10, MaxCallStackDepth)
	assert.Greater(t, DefaultSubWorkflowTimeout, 0)
	assert.Greater(t, MaxCallStackDepth, 0)
}

func TestCallWorkflowConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  CallWorkflowConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with workflow name only",
			config: CallWorkflowConfig{
				Workflow: "child-workflow",
			},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			config: CallWorkflowConfig{
				Workflow: "analyze-file",
				Inputs: map[string]string{
					"file_path":  "{{inputs.file}}",
					"max_tokens": "{{inputs.tokens}}",
				},
				Outputs: map[string]string{
					"analysis_result": "result",
				},
				Timeout: 120,
			},
			wantErr: false,
		},
		{
			name: "valid config with zero timeout",
			config: CallWorkflowConfig{
				Workflow: "quick-job",
				Timeout:  0, // inherits from step
			},
			wantErr: false,
		},
		{
			name: "valid config with empty inputs",
			config: CallWorkflowConfig{
				Workflow: "standalone-workflow",
				Inputs:   map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "valid config with empty outputs",
			config: CallWorkflowConfig{
				Workflow: "fire-and-forget",
				Outputs:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "valid config with nil inputs and outputs",
			config: CallWorkflowConfig{
				Workflow: "simple-workflow",
				Inputs:   nil,
				Outputs:  nil,
			},
			wantErr: false,
		},
		{
			name: "missing workflow name",
			config: CallWorkflowConfig{
				Inputs: map[string]string{
					"file": "test.txt",
				},
			},
			wantErr: true,
			errMsg:  "workflow name is required",
		},
		{
			name: "empty workflow name",
			config: CallWorkflowConfig{
				Workflow: "",
			},
			wantErr: true,
			errMsg:  "workflow name is required",
		},
		{
			name: "negative timeout",
			config: CallWorkflowConfig{
				Workflow: "my-workflow",
				Timeout:  -1,
			},
			wantErr: true,
			errMsg:  "timeout must be non-negative",
		},
		{
			name: "large negative timeout",
			config: CallWorkflowConfig{
				Workflow: "my-workflow",
				Timeout:  -1000,
			},
			wantErr: true,
			errMsg:  "timeout must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCallWorkflowConfig_Validate_WorkflowNameVariants(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
		wantErr      bool
	}{
		{"simple name", "analyze", false},
		{"hyphenated name", "analyze-file", false},
		{"underscored name", "analyze_file", false},
		{"camelCase name", "analyzeFile", false},
		{"name with numbers", "workflow-v2", false},
		{"path-like name", "utils/helper", false},
		{"relative path", "./workflows/child", false},
		{"name with dots", "my.workflow", false},
		{"single character", "a", false},
		{"whitespace only", "   ", false}, // not validated at this level
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CallWorkflowConfig{
				Workflow: tt.workflowName,
			}
			err := config.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCallWorkflowConfig_GetTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  int
		expected int
	}{
		{
			name:     "zero returns default",
			timeout:  0,
			expected: DefaultSubWorkflowTimeout,
		},
		{
			name:     "positive returns configured value",
			timeout:  60,
			expected: 60,
		},
		{
			name:     "large positive value",
			timeout:  3600,
			expected: 3600,
		},
		{
			name:     "exactly default value",
			timeout:  DefaultSubWorkflowTimeout,
			expected: DefaultSubWorkflowTimeout,
		},
		{
			name:     "one second",
			timeout:  1,
			expected: 1,
		},
		{
			name:     "negative returns default",
			timeout:  -1,
			expected: DefaultSubWorkflowTimeout,
		},
		{
			name:     "large negative returns default",
			timeout:  -1000,
			expected: DefaultSubWorkflowTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CallWorkflowConfig{
				Workflow: "test-workflow",
				Timeout:  tt.timeout,
			}
			assert.Equal(t, tt.expected, config.GetTimeout())
		})
	}
}

func TestCallWorkflowConfig_InputMappings(t *testing.T) {
	tests := []struct {
		name   string
		inputs map[string]string
	}{
		{
			name:   "nil inputs",
			inputs: nil,
		},
		{
			name:   "empty inputs",
			inputs: map[string]string{},
		},
		{
			name: "single input",
			inputs: map[string]string{
				"file": "{{inputs.file_path}}",
			},
		},
		{
			name: "multiple inputs",
			inputs: map[string]string{
				"file":       "{{inputs.file_path}}",
				"max_tokens": "{{inputs.tokens}}",
				"verbose":    "true",
			},
		},
		{
			name: "literal values",
			inputs: map[string]string{
				"mode":    "production",
				"retries": "3",
			},
		},
		{
			name: "state references",
			inputs: map[string]string{
				"previous_result": "{{states.prepare.output}}",
			},
		},
		{
			name: "loop context",
			inputs: map[string]string{
				"item":  "{{loop.item}}",
				"index": "{{loop.index}}",
			},
		},
		{
			name: "environment variables",
			inputs: map[string]string{
				"api_key": "{{env.API_KEY}}",
			},
		},
		{
			name: "mixed template types",
			inputs: map[string]string{
				"file":     "{{inputs.file}}",
				"api_key":  "{{env.API_KEY}}",
				"prev_out": "{{states.init.output}}",
				"constant": "fixed-value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CallWorkflowConfig{
				Workflow: "test-workflow",
				Inputs:   tt.inputs,
			}
			err := config.Validate()
			require.NoError(t, err)
			assert.Equal(t, tt.inputs, config.Inputs)
		})
	}
}

func TestCallWorkflowConfig_OutputMappings(t *testing.T) {
	tests := []struct {
		name    string
		outputs map[string]string
	}{
		{
			name:    "nil outputs",
			outputs: nil,
		},
		{
			name:    "empty outputs",
			outputs: map[string]string{},
		},
		{
			name: "single output",
			outputs: map[string]string{
				"result": "analysis_result",
			},
		},
		{
			name: "multiple outputs",
			outputs: map[string]string{
				"result":  "analysis_result",
				"summary": "summary_text",
				"status":  "exit_status",
			},
		},
		{
			name: "output names with underscores",
			outputs: map[string]string{
				"final_result":    "my_result",
				"execution_stats": "stats_output",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CallWorkflowConfig{
				Workflow: "test-workflow",
				Outputs:  tt.outputs,
			}
			err := config.Validate()
			require.NoError(t, err)
			assert.Equal(t, tt.outputs, config.Outputs)
		})
	}
}

func TestNewSubWorkflowResult(t *testing.T) {
	workflowName := "my-child-workflow"

	result := NewSubWorkflowResult(workflowName)

	require.NotNil(t, result)
	assert.Equal(t, workflowName, result.WorkflowName)
	assert.NotNil(t, result.Outputs)
	assert.Empty(t, result.Outputs)
	assert.Nil(t, result.Error)
	assert.False(t, result.StartedAt.IsZero())
	assert.True(t, result.CompletedAt.IsZero())
}

func TestNewSubWorkflowResult_EmptyName(t *testing.T) {
	result := NewSubWorkflowResult("")

	require.NotNil(t, result)
	assert.Equal(t, "", result.WorkflowName)
	assert.NotNil(t, result.Outputs)
}

func TestNewSubWorkflowResult_SpecialCharacters(t *testing.T) {
	tests := []string{
		"workflow-with-dashes",
		"workflow_with_underscores",
		"workflow.with.dots",
		"path/to/workflow",
		"./relative/workflow",
		"workflow-v2.1",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			result := NewSubWorkflowResult(name)
			require.NotNil(t, result)
			assert.Equal(t, name, result.WorkflowName)
		})
	}
}

func TestSubWorkflowResult_Duration(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(5*time.Second + 250*time.Millisecond)

	result := SubWorkflowResult{
		WorkflowName: "test",
		StartedAt:    start,
		CompletedAt:  end,
	}

	expected := 5*time.Second + 250*time.Millisecond
	assert.Equal(t, expected, result.Duration())
}

func TestSubWorkflowResult_Duration_ZeroTime(t *testing.T) {
	result := SubWorkflowResult{}
	assert.Equal(t, time.Duration(0), result.Duration())
}

func TestSubWorkflowResult_Duration_NotCompleted(t *testing.T) {
	result := NewSubWorkflowResult("test")
	// CompletedAt is zero, so duration is negative (but that's expected)
	duration := result.Duration()
	assert.Less(t, duration, time.Duration(0))
}

func TestSubWorkflowResult_Duration_Instant(t *testing.T) {
	now := time.Now()
	result := SubWorkflowResult{
		StartedAt:   now,
		CompletedAt: now,
	}
	assert.Equal(t, time.Duration(0), result.Duration())
}

func TestSubWorkflowResult_Duration_LongRunning(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour) // 1 day

	result := SubWorkflowResult{
		StartedAt:   start,
		CompletedAt: end,
	}

	assert.Equal(t, 24*time.Hour, result.Duration())
}

func TestSubWorkflowResult_Success(t *testing.T) {
	tests := []struct {
		name     string
		result   SubWorkflowResult
		expected bool
	}{
		{
			name: "success with nil error",
			result: SubWorkflowResult{
				WorkflowName: "test",
				Error:        nil,
			},
			expected: true,
		},
		{
			name: "failure with error",
			result: SubWorkflowResult{
				WorkflowName: "test",
				Error:        errors.New("execution failed"),
			},
			expected: false,
		},
		{
			name: "failure with wrapped error",
			result: SubWorkflowResult{
				WorkflowName: "test",
				Error:        errors.New("timeout: sub-workflow exceeded 300s"),
			},
			expected: false,
		},
		{
			name:     "empty result",
			result:   SubWorkflowResult{},
			expected: true, // nil error = success
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.Success())
		})
	}
}

func TestSubWorkflowResult_Outputs(t *testing.T) {
	result := NewSubWorkflowResult("test")

	// Add outputs
	result.Outputs["result"] = "analysis completed"
	result.Outputs["count"] = 42
	result.Outputs["items"] = []string{"a", "b", "c"}
	result.Outputs["metadata"] = map[string]any{
		"duration": 1.5,
		"success":  true,
	}

	assert.Len(t, result.Outputs, 4)
	assert.Equal(t, "analysis completed", result.Outputs["result"])
	assert.Equal(t, 42, result.Outputs["count"])
	assert.Equal(t, []string{"a", "b", "c"}, result.Outputs["items"])
}

func TestSubWorkflowResult_Outputs_NilValue(t *testing.T) {
	result := NewSubWorkflowResult("test")
	result.Outputs["empty"] = nil

	assert.Len(t, result.Outputs, 1)
	assert.Nil(t, result.Outputs["empty"])
}

func TestSubWorkflowResult_AllFields(t *testing.T) {
	err := errors.New("timeout exceeded")
	start := time.Now()
	end := start.Add(5 * time.Second)

	result := SubWorkflowResult{
		WorkflowName: "child-workflow",
		Outputs: map[string]any{
			"result": "partial",
		},
		Error:       err,
		StartedAt:   start,
		CompletedAt: end,
	}

	assert.Equal(t, "child-workflow", result.WorkflowName)
	assert.Len(t, result.Outputs, 1)
	assert.Equal(t, err, result.Error)
	assert.Equal(t, start, result.StartedAt)
	assert.Equal(t, end, result.CompletedAt)
	assert.False(t, result.Success())
	assert.Equal(t, 5*time.Second, result.Duration())
}

func TestCallWorkflowConfig_CompleteExample(t *testing.T) {
	config := CallWorkflowConfig{
		Workflow: "analyze-single-file",
		Inputs: map[string]string{
			"file_path":  "{{loop.item}}",
			"max_tokens": "{{inputs.max_tokens}}",
			"format":     "json",
		},
		Outputs: map[string]string{
			"analysis_result": "result",
			"error_count":     "errors",
		},
		Timeout: 120,
	}

	// Validate structure
	err := config.Validate()
	require.NoError(t, err)

	// Check field values
	assert.Equal(t, "analyze-single-file", config.Workflow)
	assert.Len(t, config.Inputs, 3)
	assert.Len(t, config.Outputs, 2)
	assert.Equal(t, 120, config.GetTimeout())

	// Check individual mappings
	assert.Equal(t, "{{loop.item}}", config.Inputs["file_path"])
	assert.Equal(t, "result", config.Outputs["analysis_result"])
}

func TestSubWorkflowResult_ExecutionLifecycle(t *testing.T) {
	// Simulate a complete sub-workflow execution lifecycle

	// Start execution
	result := NewSubWorkflowResult("data-processor")
	assert.Equal(t, "data-processor", result.WorkflowName)
	assert.False(t, result.StartedAt.IsZero())
	assert.True(t, result.CompletedAt.IsZero())

	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	// Add outputs during execution
	result.Outputs["processed_items"] = 100
	result.Outputs["status"] = "completed"

	// Complete execution
	result.CompletedAt = time.Now()

	// Verify final state
	assert.True(t, result.Success())
	assert.Greater(t, result.Duration(), time.Duration(0))
	assert.Len(t, result.Outputs, 2)
	assert.Equal(t, 100, result.Outputs["processed_items"])
}

func TestSubWorkflowResult_FailedExecution(t *testing.T) {
	// Simulate a failed sub-workflow execution

	result := NewSubWorkflowResult("failing-workflow")

	// Simulate work that fails
	result.Error = errors.New("command exited with code 1")
	result.CompletedAt = time.Now()

	// Verify failure state
	assert.False(t, result.Success())
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "code 1")
}

func TestCallWorkflowConfig_TimeoutBoundaries(t *testing.T) {
	tests := []struct {
		name            string
		timeout         int
		expectedTimeout int
		wantErr         bool
	}{
		{"minimum valid (1)", 1, 1, false},
		{"zero (uses default)", 0, DefaultSubWorkflowTimeout, false},
		{"large timeout (1 hour)", 3600, 3600, false},
		{"very large timeout (1 day)", 86400, 86400, false},
		{"negative (-1)", -1, DefaultSubWorkflowTimeout, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CallWorkflowConfig{
				Workflow: "test",
				Timeout:  tt.timeout,
			}
			err := config.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedTimeout, config.GetTimeout())
			}
		})
	}
}

func TestSubWorkflowResult_OutputTypes(t *testing.T) {
	result := NewSubWorkflowResult("test")

	// Test various output types
	result.Outputs["string"] = "hello"
	result.Outputs["int"] = 42
	result.Outputs["float"] = 3.14
	result.Outputs["bool"] = true
	result.Outputs["slice"] = []int{1, 2, 3}
	result.Outputs["map"] = map[string]string{"key": "value"}
	result.Outputs["nil"] = nil

	assert.Len(t, result.Outputs, 7)
	assert.IsType(t, "", result.Outputs["string"])
	assert.IsType(t, 0, result.Outputs["int"])
	assert.IsType(t, 0.0, result.Outputs["float"])
	assert.IsType(t, false, result.Outputs["bool"])
	assert.IsType(t, []int{}, result.Outputs["slice"])
	assert.IsType(t, map[string]string{}, result.Outputs["map"])
	assert.Nil(t, result.Outputs["nil"])
}
