package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/interfaces/cli/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDryRunFormatter_Format_SimplePlan(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false) // no color for easier testing

	plan := &workflow.DryRunPlan{
		WorkflowName: "test-workflow",
		Description:  "A test workflow",
		Inputs: map[string]workflow.DryRunInput{
			"name": {Name: "name", Value: "test", Default: false, Required: true},
		},
		Steps: []workflow.DryRunStep{
			{
				Name:    "start",
				Type:    workflow.StepTypeCommand,
				Command: "echo hello",
				Transitions: []workflow.DryRunTransition{
					{Type: "success", Target: "done"},
				},
			},
			{
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "test-workflow", "should show workflow name")
	assert.Contains(t, output, "start", "should show step name")
	assert.Contains(t, output, "done", "should show terminal step")
}

func TestDryRunFormatter_Format_WithInputs(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "input-test",
		Inputs: map[string]workflow.DryRunInput{
			"file_path": {Name: "file_path", Value: "test.txt", Default: false, Required: true},
			"count":     {Name: "count", Value: 10, Default: true, Required: false},
		},
		Steps: []workflow.DryRunStep{
			{Name: "step1", Type: workflow.StepTypeCommand, Command: "echo test"},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "file_path", "should show required input")
	assert.Contains(t, output, "count", "should show optional input with default")
}

func TestDryRunFormatter_Format_WithHooks(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "hooks-test",
		Steps: []workflow.DryRunStep{
			{
				Name:    "step1",
				Type:    workflow.StepTypeCommand,
				Command: "echo main",
				Hooks: workflow.DryRunHooks{
					Pre:  []workflow.DryRunHook{{Type: "log", Content: "Starting"}},
					Post: []workflow.DryRunHook{{Type: "command", Content: "echo done"}},
				},
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	// Should indicate hooks are present
	assert.Contains(t, output, "hook", "should indicate hooks")
}

func TestDryRunFormatter_Format_WithTransitions(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "transition-test",
		Steps: []workflow.DryRunStep{
			{
				Name:    "check",
				Type:    workflow.StepTypeCommand,
				Command: "check-status",
				Transitions: []workflow.DryRunTransition{
					{Type: "conditional", Condition: "exit_code == 0", Target: "success"},
					{Type: "conditional", Condition: "exit_code == 1", Target: "retry"},
					{Type: "default", Target: "error"},
				},
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "success", "should show success transition")
	assert.Contains(t, output, "retry", "should show retry transition")
	assert.Contains(t, output, "error", "should show error transition")
}

func TestDryRunFormatter_Format_ParallelStep(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "parallel-test",
		Steps: []workflow.DryRunStep{
			{
				Name:     "multi",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"lint", "test", "build"},
				Strategy: "all_succeed",
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "parallel", "should indicate parallel step type")
	assert.Contains(t, output, "lint", "should list branches")
	assert.Contains(t, output, "test", "should list branches")
	assert.Contains(t, output, "build", "should list branches")
}

func TestDryRunFormatter_Format_ForEachLoop(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "loop-test",
		Steps: []workflow.DryRunStep{
			{
				Name: "process_files",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.DryRunLoop{
					Type:          "for_each",
					Items:         "{{inputs.files}}",
					Body:          []string{"process_item"},
					MaxIterations: 100,
					OnComplete:    "done",
				},
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "for_each", "should indicate loop type")
	assert.Contains(t, output, "process_files", "should show loop step name")
}

func TestDryRunFormatter_Format_WhileLoop(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "while-test",
		Steps: []workflow.DryRunStep{
			{
				Name: "poll",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.DryRunLoop{
					Type:           "while",
					Condition:      "loop.index < 5",
					Body:           []string{"check"},
					MaxIterations:  10,
					BreakCondition: "states.check.output == 'ready'",
					OnComplete:     "done",
				},
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "while", "should indicate while loop type")
	assert.Contains(t, output, "poll", "should show loop step name")
}

func TestDryRunFormatter_Format_WithRetryConfig(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "retry-test",
		Steps: []workflow.DryRunStep{
			{
				Name:    "flaky",
				Type:    workflow.StepTypeCommand,
				Command: "flaky-command",
				Retry: &workflow.DryRunRetry{
					MaxAttempts:    3,
					InitialDelayMs: 100,
					MaxDelayMs:     1000,
					Backoff:        "exponential",
					Multiplier:     2.0,
				},
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "retry", "should show retry information")
}

func TestDryRunFormatter_Format_WithCapture(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "capture-test",
		Steps: []workflow.DryRunStep{
			{
				Name:    "fetch",
				Type:    workflow.StepTypeCommand,
				Command: "curl http://example.com",
				Capture: &workflow.DryRunCapture{
					Stdout:  "response",
					Stderr:  "errors",
					MaxSize: "10MB",
				},
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "capture", "should show capture information")
}

func TestDryRunFormatter_Format_TerminalStates(t *testing.T) {
	tests := []struct {
		name       string
		status     workflow.TerminalStatus
		wantOutput string
	}{
		{
			name:       "success terminal",
			status:     workflow.TerminalSuccess,
			wantOutput: "success",
		},
		{
			name:       "failure terminal",
			status:     workflow.TerminalFailure,
			wantOutput: "failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewDryRunFormatter(buf, false)

			plan := &workflow.DryRunPlan{
				WorkflowName: "terminal-test",
				Steps: []workflow.DryRunStep{
					{
						Name:   "end",
						Type:   workflow.StepTypeTerminal,
						Status: tt.status,
					},
				},
			}

			err := formatter.Format(plan)

			require.NoError(t, err)
			output := strings.ToLower(buf.String())
			assert.Contains(t, output, tt.wantOutput, "should indicate terminal status")
		})
	}
}

func TestDryRunFormatter_Format_WithTimeout(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "timeout-test",
		Steps: []workflow.DryRunStep{
			{
				Name:    "slow",
				Type:    workflow.StepTypeCommand,
				Command: "slow-command",
				Timeout: 30,
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "timeout", "should show timeout information")
}

func TestDryRunFormatter_Format_WithDescription(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "desc-test",
		Description:  "This workflow does important things",
		Steps: []workflow.DryRunStep{
			{
				Name:        "step1",
				Type:        workflow.StepTypeCommand,
				Description: "This step validates input",
				Command:     "validate",
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "important things", "should show workflow description")
}

func TestDryRunFormatter_Format_WithWorkingDir(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "dir-test",
		Steps: []workflow.DryRunStep{
			{
				Name:    "build",
				Type:    workflow.StepTypeCommand,
				Command: "make build",
				Dir:     "/path/to/project",
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "/path/to/project", "should show working directory")
}

func TestDryRunFormatter_Format_EmptyPlan(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "empty-test",
		Steps:        []workflow.DryRunStep{},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "empty-test", "should show workflow name even with no steps")
}

func TestDryRunFormatter_Format_ContinueOnError(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "continue-test",
		Steps: []workflow.DryRunStep{
			{
				Name:            "optional",
				Type:            workflow.StepTypeCommand,
				Command:         "optional-command",
				ContinueOnError: true,
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "continue", "should indicate continue on error")
}

func TestDryRunFormatter_Format_WithColor(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, true) // with color

	plan := &workflow.DryRunPlan{
		WorkflowName: "color-test",
		Steps: []workflow.DryRunStep{
			{Name: "step1", Type: workflow.StepTypeCommand, Command: "echo test"},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	// Just verify it doesn't panic with colors enabled
	output := buf.String()
	assert.Contains(t, output, "color-test", "should contain workflow name")
}

func TestDryRunFormatter_Format_ComplexWorkflow(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	// Complex workflow with multiple step types and features
	plan := &workflow.DryRunPlan{
		WorkflowName: "complex-workflow",
		Description:  "A complex workflow with all features",
		Inputs: map[string]workflow.DryRunInput{
			"file":   {Name: "file", Value: "input.txt", Required: true},
			"format": {Name: "format", Value: "json", Default: true},
		},
		Steps: []workflow.DryRunStep{
			{
				Name:    "validate",
				Type:    workflow.StepTypeCommand,
				Command: "test -f input.txt",
				Hooks: workflow.DryRunHooks{
					Pre: []workflow.DryRunHook{{Type: "log", Content: "Validating..."}},
				},
				Transitions: []workflow.DryRunTransition{
					{Type: "success", Target: "process"},
					{Type: "failure", Target: "error"},
				},
			},
			{
				Name:     "process",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"analyze", "backup"},
				Strategy: "all_succeed",
				Transitions: []workflow.DryRunTransition{
					{Type: "success", Target: "loop_items"},
				},
			},
			{
				Name: "loop_items",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.DryRunLoop{
					Type:       "for_each",
					Items:      "{{states.process.items}}",
					Body:       []string{"process_item"},
					OnComplete: "done",
				},
			},
			{
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			{
				Name:   "error",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "complex-workflow", "should show workflow name")
	assert.Contains(t, output, "validate", "should show all steps")
	assert.Contains(t, output, "process", "should show parallel step")
	assert.Contains(t, output, "loop_items", "should show loop step")
	assert.Contains(t, output, "done", "should show terminal steps")
}

func TestDryRunFormatter_Format_MaxConcurrent(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewDryRunFormatter(buf, false)

	plan := &workflow.DryRunPlan{
		WorkflowName: "concurrent-test",
		Steps: []workflow.DryRunStep{
			{
				Name:          "parallel",
				Type:          workflow.StepTypeParallel,
				Branches:      []string{"a", "b", "c", "d"},
				Strategy:      "best_effort",
				MaxConcurrent: 2,
			},
		},
	}

	err := formatter.Format(plan)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "2", "should show max concurrent value")
}
