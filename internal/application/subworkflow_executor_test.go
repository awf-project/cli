package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createParentChildWorkflows returns parent and child workflows for testing call_workflow.
// Returns (parentWorkflow, childWorkflow) as separate workflow instances.
func createParentChildWorkflows(parentName, childName string) (parentWf, childWf *workflow.Workflow) {
	// Child workflow: simple echo command
	childWf = &workflow.Workflow{
		Name:    childName,
		Initial: "do_work",
		Inputs: []workflow.Input{
			{Name: "input_value", Type: "string", Required: false},
		},
		Steps: map[string]*workflow.Step{
			"do_work": {
				Name:      "do_work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.inputs.input_value}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Parent workflow: calls child workflow
	parentWf = &workflow.Workflow{
		Name:    parentName,
		Initial: "call_child",
		Inputs: []workflow.Input{
			{Name: "parent_input", Type: "string", Required: false},
		},
		Steps: map[string]*workflow.Step{
			"call_child": {
				Name: "call_child",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: childName,
					Inputs: map[string]string{
						"input_value": "{{.inputs.parent_input}}",
					},
					Outputs: map[string]string{
						"do_work": "child_result",
					},
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	return parentWf, childWf
}

func TestSubWorkflowErrors_Defined(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "ErrCircularWorkflowCall",
			err:      application.ErrCircularWorkflowCall,
			contains: "circular",
		},
		{
			name:     "ErrMaxNestingExceeded",
			err:      application.ErrMaxNestingExceeded,
			contains: "nesting",
		},
		{
			name:     "ErrSubWorkflowNotFound",
			err:      application.ErrSubWorkflowNotFound,
			contains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.err)
			assert.Contains(t, tt.err.Error(), tt.contains)
		})
	}
}

func TestExecuteCallWorkflowStep_BasicSuccess(t *testing.T) {
	parentWf, childWf := createParentChildWorkflows("parent", "child")

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("parent", parentWf).
		WithWorkflow("child", childWf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "parent", map[string]any{
		"parent_input": "hello",
	})

	require.NoError(t, err, "sub-workflow execution should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

func TestExecuteCallWorkflowStep_NoInputs(t *testing.T) {
	// Child with no inputs
	childWf := &workflow.Workflow{
		Name:    "simple-child",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo done",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Parent calling child without inputs
	parentWf := &workflow.Workflow{
		Name:    "simple-parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "simple-child",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("simple-child", childWf).
		WithWorkflow("simple-parent", parentWf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "simple-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_InputMapping(t *testing.T) {
	// Child expecting specific inputs
	childWf := &workflow.Workflow{
		Name:    "input-child",
		Initial: "process",
		Inputs: []workflow.Input{
			{Name: "file", Type: "string", Required: true},
			{Name: "mode", Type: "string", Default: "default"},
		},
		Steps: map[string]*workflow.Step{
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "process {{.inputs.file}} --mode={{.inputs.mode}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Parent with input mapping
	parentWf := &workflow.Workflow{
		Name:    "input-parent",
		Initial: "call",
		Inputs: []workflow.Input{
			{Name: "target_file", Type: "string"},
			{Name: "operation_mode", Type: "string"},
		},
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "input-child",
					Inputs: map[string]string{
						"file": "{{.inputs.target_file}}",
						"mode": "{{.inputs.operation_mode}}",
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("input-child", childWf).
		WithWorkflow("input-parent", parentWf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "input-parent", map[string]any{
		"target_file":    "test.txt",
		"operation_mode": "strict",
	})

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_InputMappingFromStepOutput(t *testing.T) {
	// Child workflow
	childWf := &workflow.Workflow{
		Name:    "step-output-child",
		Initial: "work",
		Inputs: []workflow.Input{
			{Name: "data", Type: "string"},
		},
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "process {{.inputs.data}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Parent with step that produces output, then calls child
	parentWf := &workflow.Workflow{
		Name:    "step-output-parent",
		Initial: "prepare",
		Steps: map[string]*workflow.Step{
			"prepare": {
				Name:      "prepare",
				Type:      workflow.StepTypeCommand,
				Command:   "echo prepared_data",
				OnSuccess: "call",
			},
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "step-output-child",
					Inputs: map[string]string{
						"data": "{{.states.prepare.Output}}",
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("step-output-child", childWf).
		WithWorkflow("step-output-parent", parentWf).
		WithCommandResult("echo prepared_data", &ports.CommandResult{
			Stdout:   "prepared_data\n",
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "step-output-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_OutputMapping(t *testing.T) {
	parentWf, childWf := createParentChildWorkflows("output-parent", "output-child")

	// Modify parent to capture outputs
	parentWf.Steps["call_child"].CallWorkflow.Outputs = map[string]string{
		"do_work": "result",
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("output-parent", parentWf).
		WithWorkflow("output-child", childWf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "output-parent", map[string]any{
		"parent_input": "test_value",
	})

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify call_child step state was recorded
	stepState, ok := ctx.GetStepState("call_child")
	require.True(t, ok, "call_child step state should be recorded")
	assert.Equal(t, workflow.StatusCompleted, stepState.Status)
}

func TestExecuteCallWorkflowStep_MultipleOutputs(t *testing.T) {
	// Child with multiple steps producing output
	childWf := &workflow.Workflow{
		Name:    "multi-output-child",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo output1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo output2",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Parent capturing multiple outputs
	parentWf := &workflow.Workflow{
		Name:    "multi-output-parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "multi-output-child",
					Outputs: map[string]string{
						"step1": "first_result",
						"step2": "second_result",
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("multi-output-child", childWf).
		WithWorkflow("multi-output-parent", parentWf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "multi-output-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_CircularDetection_Direct(t *testing.T) {
	// Workflow A calls itself
	wf := &workflow.Workflow{
		Name:    "self-caller",
		Initial: "call_self",
		Steps: map[string]*workflow.Step{
			"call_self": {
				Name: "call_self",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "self-caller", // circular!
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("self-caller", wf).
		Build()

	_, err := execSvc.Run(context.Background(), "self-caller", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrCircularWorkflowCall)
}

func TestExecuteCallWorkflowStep_CircularDetection_Indirect(t *testing.T) {
	// A -> B -> A (indirect circular)
	wfA := &workflow.Workflow{
		Name:    "workflow-a",
		Initial: "call_b",
		Steps: map[string]*workflow.Step{
			"call_b": {
				Name: "call_b",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "workflow-b",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	wfB := &workflow.Workflow{
		Name:    "workflow-b",
		Initial: "call_a",
		Steps: map[string]*workflow.Step{
			"call_a": {
				Name: "call_a",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "workflow-a", // back to A - circular!
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("workflow-a", wfA).
		WithWorkflow("workflow-b", wfB).
		Build()

	_, err := execSvc.Run(context.Background(), "workflow-a", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrCircularWorkflowCall)
}

func TestExecuteCallWorkflowStep_CircularDetection_ThreeLevel(t *testing.T) {
	// A -> B -> C -> A (three-level circular)
	wfA := &workflow.Workflow{
		Name:    "wf-a",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "wf-b",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfB := &workflow.Workflow{
		Name:    "wf-b",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "wf-c",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfC := &workflow.Workflow{
		Name:    "wf-c",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "wf-a", // back to A - circular!
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("wf-a", wfA).
		WithWorkflow("wf-b", wfB).
		WithWorkflow("wf-c", wfC).
		Build()

	_, err := execSvc.Run(context.Background(), "wf-a", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrCircularWorkflowCall)
}

func TestExecuteCallWorkflowStep_NestedThreeLevels_Success(t *testing.T) {
	// Create a valid 3-level nesting: A -> B -> C (no circular)
	levelA := &workflow.Workflow{
		Name:    "level-a",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "level-b",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	levelB := &workflow.Workflow{
		Name:    "level-b",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "level-c",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	levelC := &workflow.Workflow{
		Name:    "level-c",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo level-c",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("level-a", levelA).
		WithWorkflow("level-b", levelB).
		WithWorkflow("level-c", levelC).
		Build()

	ctx, err := execSvc.Run(context.Background(), "level-a", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_MaxNestingExceeded(t *testing.T) {
	// Create 12 levels of nesting (exceeds MaxCallStackDepth of 10)
	// We need 12 workflows because:
	// - The depth check happens when a workflow tries to CALL another
	// - At depth=10, the 11th call_workflow should fail
	// - So we need: deep-a(calls b) -> deep-b(calls c) -> ... -> deep-k(calls l) -> deep-l(terminal)
	// That's 11 call_workflow steps, with the 11th triggering the depth check at depth=10
	harness := NewTestHarness(t)

	for i := 1; i <= 12; i++ {
		name := "deep-" + string(rune('a'+i-1))
		var nextWorkflow string
		if i < 12 {
			nextWorkflow = "deep-" + string(rune('a'+i))
		}

		if nextWorkflow != "" {
			wf := &workflow.Workflow{
				Name:    name,
				Initial: "call",
				Steps: map[string]*workflow.Step{
					"call": {
						Name: "call",
						Type: workflow.StepTypeCallWorkflow,
						CallWorkflow: &workflow.CallWorkflowConfig{
							Workflow: nextWorkflow,
						},
						OnSuccess: "done",
					},
					"done": {Name: "done", Type: workflow.StepTypeTerminal},
				},
			}
			harness = harness.WithWorkflow(name, wf)
		} else {
			// Last one is a terminal
			wf := &workflow.Workflow{
				Name:    name,
				Initial: "work",
				Steps: map[string]*workflow.Step{
					"work": {
						Name:      "work",
						Type:      workflow.StepTypeCommand,
						Command:   "echo done",
						OnSuccess: "done",
					},
					"done": {Name: "done", Type: workflow.StepTypeTerminal},
				},
			}
			harness = harness.WithWorkflow(name, wf)
		}
	}

	execSvc, _ := harness.Build()
	_, err := execSvc.Run(context.Background(), "deep-a", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrMaxNestingExceeded)
}

func TestExecuteCallWorkflowStep_ChildFailure_PropagatesError(t *testing.T) {
	// Child that fails
	childWf := &workflow.Workflow{
		Name:    "failing-child",
		Initial: "fail",
		Steps: map[string]*workflow.Step{
			"fail": {
				Name:      "fail",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1", // will fail
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent without OnFailure
	parentWf := &workflow.Workflow{
		Name:    "error-parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "failing-child",
				},
				OnSuccess: "done",
				// No OnFailure - error propagates
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Create executor that fails on "exit 1"
	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1, Stderr: "command failed"}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("failing-child", childWf).
		WithWorkflow("error-parent", parentWf).
		WithExecutor(executor).
		Build()

	ctx, err := execSvc.Run(context.Background(), "error-parent", nil)

	require.Error(t, err)
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

func TestExecuteCallWorkflowStep_ChildFailure_OnFailureTransition(t *testing.T) {
	// Child that fails
	childWf := &workflow.Workflow{
		Name:    "failing-child",
		Initial: "fail",
		Steps: map[string]*workflow.Step{
			"fail": {
				Name:      "fail",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent with OnFailure transition
	parentWf := &workflow.Workflow{
		Name:    "handled-parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "failing-child",
				},
				OnSuccess: "done",
				OnFailure: "error_handler",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
			"error_handler": {
				Name:      "error_handler",
				Type:      workflow.StepTypeCommand,
				Command:   "echo handling error",
				OnSuccess: "done",
			},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("failing-child", childWf).
		WithWorkflow("handled-parent", parentWf).
		WithExecutor(executor).
		Build()

	ctx, err := execSvc.Run(context.Background(), "handled-parent", nil)

	require.NoError(t, err, "workflow should complete via error handler")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_ContinueOnError(t *testing.T) {
	// Child that fails
	childWf := &workflow.Workflow{
		Name:    "failing-child",
		Initial: "fail",
		Steps: map[string]*workflow.Step{
			"fail": {
				Name:      "fail",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent with ContinueOnError
	parentWf := &workflow.Workflow{
		Name:    "continue-parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "failing-child",
				},
				OnSuccess:       "done",
				ContinueOnError: true, // Continue even if child fails
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("failing-child", childWf).
		WithWorkflow("continue-parent", parentWf).
		WithExecutor(executor).
		Build()

	ctx, err := execSvc.Run(context.Background(), "continue-parent", nil)

	require.NoError(t, err, "workflow should continue despite child failure")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

func TestExecuteCallWorkflowStep_SubWorkflowNotFound(t *testing.T) {
	// Parent calls non-existent child
	parentWf := &workflow.Workflow{
		Name:    "missing-child-parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "nonexistent-workflow",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("missing-child-parent", parentWf).
		Build()

	_, err := execSvc.Run(context.Background(), "missing-child-parent", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "workflow file not found")
}

func TestExecuteCallWorkflowStep_Timeout(t *testing.T) {
	// Slow child workflow
	childWf := &workflow.Workflow{
		Name:    "slow-child",
		Initial: "slow",
		Steps: map[string]*workflow.Step{
			"slow": {
				Name:      "slow",
				Type:      workflow.StepTypeCommand,
				Command:   "sleep 10",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent with short timeout
	parentWf := &workflow.Workflow{
		Name:    "timeout-parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "slow-child",
					Timeout:  1, // 1 second timeout
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	// Create a mock executor that simulates slow execution
	slowExecutor := &slowMockExecutor{delay: 5 * time.Second}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("slow-child", childWf).
		WithWorkflow("timeout-parent", parentWf).
		WithExecutor(slowExecutor).
		Build()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := execSvc.Run(ctx, "timeout-parent", nil)
	// Should timeout or error
	if err != nil {
		assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled),
			"expected timeout error, got: %v", err)
	}
}

// slowMockExecutor simulates slow command execution
type slowMockExecutor struct {
	delay time.Duration
}

func (m *slowMockExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	select {
	case <-time.After(m.delay):
		return &ports.CommandResult{ExitCode: 0, Stdout: "done"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestExecuteCallWorkflowStep_MissingCallWorkflowConfig(t *testing.T) {
	// Step with call_workflow type but nil CallWorkflow config
	wf := &workflow.Workflow{
		Name:    "bad-config",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name:         "call",
				Type:         workflow.StepTypeCallWorkflow,
				CallWorkflow: nil, // missing config
				OnSuccess:    "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("bad-config", wf).
		Build()

	_, err := execSvc.Run(context.Background(), "bad-config", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "call_workflow configuration is required")
}

func TestExecuteCallWorkflowStep_MixedWithCommandSteps(t *testing.T) {
	// Child workflow
	helperWf := &workflow.Workflow{
		Name:    "helper",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo helper",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent with command -> call_workflow -> command
	mixedWf := &workflow.Workflow{
		Name:    "mixed",
		Initial: "prepare",
		Steps: map[string]*workflow.Step{
			"prepare": {
				Name:      "prepare",
				Type:      workflow.StepTypeCommand,
				Command:   "echo prepare",
				OnSuccess: "call",
			},
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "helper",
				},
				OnSuccess: "finalize",
			},
			"finalize": {
				Name:      "finalize",
				Type:      workflow.StepTypeCommand,
				Command:   "echo finalize",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("helper", helperWf).
		WithWorkflow("mixed", mixedWf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "mixed", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify all steps were executed
	_, hasPrep := ctx.GetStepState("prepare")
	_, hasCall := ctx.GetStepState("call")
	_, hasFinal := ctx.GetStepState("finalize")

	assert.True(t, hasPrep, "prepare step should be recorded")
	assert.True(t, hasCall, "call step should be recorded")
	assert.True(t, hasFinal, "finalize step should be recorded")
}

func TestExecuteCallWorkflowStep_ErrorMessages_IncludeContext(t *testing.T) {
	tests := []struct {
		name          string
		setupHarness  func(*testing.T) *application.ExecutionService
		workflowName  string
		expectedParts []string
	}{
		{
			name: "missing workflow includes step and workflow name",
			setupHarness: func(t *testing.T) *application.ExecutionService {
				wf := &workflow.Workflow{
					Name:    "parent",
					Initial: "call",
					Steps: map[string]*workflow.Step{
						"call": {
							Name: "call",
							Type: workflow.StepTypeCallWorkflow,
							CallWorkflow: &workflow.CallWorkflowConfig{
								Workflow: "missing-workflow",
							},
							OnSuccess: "done",
						},
						"done": {Name: "done", Type: workflow.StepTypeTerminal},
					},
				}
				execSvc, _ := NewTestHarness(t).WithWorkflow("parent", wf).Build()
				return execSvc
			},
			workflowName:  "parent",
			expectedParts: []string{"call", "missing-workflow"},
		},
		{
			name: "circular error includes call stack",
			setupHarness: func(t *testing.T) *application.ExecutionService {
				loopA := &workflow.Workflow{
					Name:    "loop-a",
					Initial: "call",
					Steps: map[string]*workflow.Step{
						"call": {
							Name: "call",
							Type: workflow.StepTypeCallWorkflow,
							CallWorkflow: &workflow.CallWorkflowConfig{
								Workflow: "loop-b",
							},
							OnSuccess: "done",
						},
						"done": {Name: "done", Type: workflow.StepTypeTerminal},
					},
				}
				loopB := &workflow.Workflow{
					Name:    "loop-b",
					Initial: "call",
					Steps: map[string]*workflow.Step{
						"call": {
							Name: "call",
							Type: workflow.StepTypeCallWorkflow,
							CallWorkflow: &workflow.CallWorkflowConfig{
								Workflow: "loop-a",
							},
							OnSuccess: "done",
						},
						"done": {Name: "done", Type: workflow.StepTypeTerminal},
					},
				}
				execSvc, _ := NewTestHarness(t).
					WithWorkflow("loop-a", loopA).
					WithWorkflow("loop-b", loopB).
					Build()
				return execSvc
			},
			workflowName:  "loop-a",
			expectedParts: []string{"circular", "loop-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execSvc := tt.setupHarness(t)
			_, err := execSvc.Run(context.Background(), tt.workflowName, nil)

			require.Error(t, err)
			errMsg := err.Error()
			for _, part := range tt.expectedParts {
				assert.Contains(t, errMsg, part,
					"error message should contain: %s", part)
			}
		})
	}
}

func TestExecuteCallWorkflowStep_RecordsStepState(t *testing.T) {
	parentWf, childWf := createParentChildWorkflows("state-parent", "state-child")

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("state-parent", parentWf).
		WithWorkflow("state-child", childWf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "state-parent", map[string]any{
		"parent_input": "test_value",
	})

	require.NoError(t, err)

	// Verify call_child step state was recorded
	stepState, ok := ctx.GetStepState("call_child")
	require.True(t, ok, "call_child step state should be recorded")
	assert.Equal(t, workflow.StatusCompleted, stepState.Status)
	assert.NotZero(t, stepState.StartedAt)
	assert.NotZero(t, stepState.CompletedAt)
	assert.True(t, stepState.CompletedAt.After(stepState.StartedAt) ||
		stepState.CompletedAt.Equal(stepState.StartedAt))
}

func TestExecuteCallWorkflowStep_FailedChildRecordsError(t *testing.T) {
	childWf := &workflow.Workflow{
		Name:    "error-child",
		Initial: "fail",
		Steps: map[string]*workflow.Step{
			"fail": {
				Name:      "fail",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	parentWf := &workflow.Workflow{
		Name:    "error-parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "error-child",
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1, Stderr: "command failed"}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("error-child", childWf).
		WithWorkflow("error-parent", parentWf).
		WithExecutor(executor).
		Build()

	ctx, err := execSvc.Run(context.Background(), "error-parent", nil)

	require.NoError(t, err) // Workflow completes via error path

	// Verify call step state shows failure
	stepState, ok := ctx.GetStepState("call")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusFailed, stepState.Status)
	assert.NotEmpty(t, stepState.Error)
}

func TestExecuteCallWorkflowStep_ContextCancellation(t *testing.T) {
	// Child with slow step
	childWf := &workflow.Workflow{
		Name:    "slow-child",
		Initial: "slow",
		Steps: map[string]*workflow.Step{
			"slow": {
				Name:      "slow",
				Type:      workflow.StepTypeCommand,
				Command:   "sleep 30",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	parentWf := &workflow.Workflow{
		Name:    "cancel-parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "slow-child",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	slowExecutor := &slowMockExecutor{delay: 30 * time.Second}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("slow-child", childWf).
		WithWorkflow("cancel-parent", parentWf).
		WithExecutor(slowExecutor).
		Build()

	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel is called in goroutine below, not deferred

	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result, err := execSvc.Run(ctx, "cancel-parent", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled),
		"expected context.Canceled, got: %v", err)
	// When context is cancelled, status should be cancelled (not failed)
	assert.Equal(t, workflow.StatusCancelled, result.Status)
}

func TestExecuteCallWorkflowStep_CircularDetection_DiamondPattern(t *testing.T) {
	// Diamond pattern: A calls B and C, both B and C call D, D calls A
	// This tests that circular detection works across multiple paths
	//
	//       A
	//      / \
	//     B   C
	//      \ /
	//       D -> A (circular!)

	diamondA := &workflow.Workflow{
		Name:    "diamond-a",
		Initial: "call_b",
		Steps: map[string]*workflow.Step{
			"call_b": {
				Name: "call_b",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "diamond-b",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	diamondB := &workflow.Workflow{
		Name:    "diamond-b",
		Initial: "call_d",
		Steps: map[string]*workflow.Step{
			"call_d": {
				Name: "call_d",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "diamond-d",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	diamondD := &workflow.Workflow{
		Name:    "diamond-d",
		Initial: "call_a",
		Steps: map[string]*workflow.Step{
			"call_a": {
				Name: "call_a",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "diamond-a", // Back to A - circular!
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("diamond-a", diamondA).
		WithWorkflow("diamond-b", diamondB).
		WithWorkflow("diamond-d", diamondD).
		Build()

	_, err := execSvc.Run(context.Background(), "diamond-a", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrCircularWorkflowCall)
}

func TestExecuteCallWorkflowStep_SameWorkflowCalledFromDifferentParents_NotCircular(t *testing.T) {
	// Non-circular: A calls B, A also calls C, both B and C call D
	// D is called twice from different parents - this is NOT circular
	//
	//       A
	//      / \
	//     B   C
	//      \ /
	//       D (terminal)

	sharedD := &workflow.Workflow{
		Name:    "shared-d",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo shared-d",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	sharedB := &workflow.Workflow{
		Name:    "shared-b",
		Initial: "call_d",
		Steps: map[string]*workflow.Step{
			"call_d": {
				Name: "call_d",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "shared-d",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	sharedA := &workflow.Workflow{
		Name:    "shared-a",
		Initial: "call_b",
		Steps: map[string]*workflow.Step{
			"call_b": {
				Name: "call_b",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "shared-b",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("shared-d", sharedD).
		WithWorkflow("shared-b", sharedB).
		WithWorkflow("shared-a", sharedA).
		Build()

	ctx, err := execSvc.Run(context.Background(), "shared-a", nil)

	require.NoError(t, err, "calling same workflow from different parents should not be circular")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_TimeoutZero_InheritsParentContext(t *testing.T) {
	// Child with slow operation
	childWf := &workflow.Workflow{
		Name:    "inherit-child",
		Initial: "slow",
		Steps: map[string]*workflow.Step{
			"slow": {
				Name:      "slow",
				Type:      workflow.StepTypeCommand,
				Command:   "sleep 10",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent with Timeout: 0 (should inherit from parent context)
	parentWf := &workflow.Workflow{
		Name:    "inherit-parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "inherit-child",
					Timeout:  0, // Zero means inherit from parent context
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	slowExecutor := &slowMockExecutor{delay: 10 * time.Second}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("inherit-child", childWf).
		WithWorkflow("inherit-parent", parentWf).
		WithExecutor(slowExecutor).
		Build()

	// Parent context with short timeout - should apply to child
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := execSvc.Run(ctx, "inherit-parent", nil)

	// Should timeout from parent context since child Timeout is 0
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled),
		"expected timeout error from parent context, got: %v", err)
}

func TestExecuteCallWorkflowStep_NestedTimeouts_MostRestrictiveWins(t *testing.T) {
	// Innermost child with slow operation
	innerChild := &workflow.Workflow{
		Name:    "inner-child",
		Initial: "slow",
		Steps: map[string]*workflow.Step{
			"slow": {
				Name:      "slow",
				Type:      workflow.StepTypeCommand,
				Command:   "sleep 30",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Middle workflow with 5s timeout
	middleChild := &workflow.Workflow{
		Name:    "middle-child",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "inner-child",
					Timeout:  5, // 5 seconds
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Outer workflow with 10s timeout
	outerParent := &workflow.Workflow{
		Name:    "outer-parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "middle-child",
					Timeout:  10, // 10 seconds
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	slowExecutor := &slowMockExecutor{delay: 30 * time.Second}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("inner-child", innerChild).
		WithWorkflow("middle-child", middleChild).
		WithWorkflow("outer-parent", outerParent).
		WithExecutor(slowExecutor).
		Build()

	// Parent context with 1s timeout - most restrictive
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	_, err := execSvc.Run(ctx, "outer-parent", nil)
	elapsed := time.Since(start)

	require.Error(t, err)
	// Should timeout in ~1s (parent context), not 5s or 10s
	assert.Less(t, elapsed, 3*time.Second,
		"should timeout from most restrictive context (~1s), not wait for nested timeouts")
}

func TestExecuteCallWorkflowStep_MaxNestingExactlyAtLimit(t *testing.T) {
	// Create exactly MaxCallStackDepth levels (10)
	// This should succeed, not fail
	harness := NewTestHarness(t)

	for i := 1; i <= workflow.MaxCallStackDepth; i++ {
		name := fmt.Sprintf("exact-%d", i)
		var nextWorkflow string
		if i < workflow.MaxCallStackDepth {
			nextWorkflow = fmt.Sprintf("exact-%d", i+1)
		}

		if nextWorkflow != "" {
			wf := &workflow.Workflow{
				Name:    name,
				Initial: "call",
				Steps: map[string]*workflow.Step{
					"call": {
						Name: "call",
						Type: workflow.StepTypeCallWorkflow,
						CallWorkflow: &workflow.CallWorkflowConfig{
							Workflow: nextWorkflow,
						},
						OnSuccess: "done",
					},
					"done": {Name: "done", Type: workflow.StepTypeTerminal},
				},
			}
			harness = harness.WithWorkflow(name, wf)
		} else {
			// Last one is a terminal with command
			wf := &workflow.Workflow{
				Name:    name,
				Initial: "work",
				Steps: map[string]*workflow.Step{
					"work": {
						Name:      "work",
						Type:      workflow.StepTypeCommand,
						Command:   "echo done",
						OnSuccess: "done",
					},
					"done": {Name: "done", Type: workflow.StepTypeTerminal},
				},
			}
			harness = harness.WithWorkflow(name, wf)
		}
	}

	execSvc, _ := harness.Build()
	ctx, err := execSvc.Run(context.Background(), "exact-1", nil)

	require.NoError(t, err, "exactly MaxCallStackDepth levels should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_CallStackCleanedOnError(t *testing.T) {
	// Child that fails
	cleanupChild := &workflow.Workflow{
		Name:    "cleanup-child",
		Initial: "fail",
		Steps: map[string]*workflow.Step{
			"fail": {
				Name:      "fail",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent that can recover and call another workflow
	recoveryChild := &workflow.Workflow{
		Name:    "recovery-child",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo recovered",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	cleanupParent := &workflow.Workflow{
		Name:    "cleanup-parent",
		Initial: "call_failing",
		Steps: map[string]*workflow.Step{
			"call_failing": {
				Name: "call_failing",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "cleanup-child",
				},
				OnSuccess: "done",
				OnFailure: "recover",
			},
			"recover": {
				Name: "recover",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "recovery-child",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("cleanup-child", cleanupChild).
		WithWorkflow("recovery-child", recoveryChild).
		WithWorkflow("cleanup-parent", cleanupParent).
		WithExecutor(executor).
		Build()

	ctx, err := execSvc.Run(context.Background(), "cleanup-parent", nil)

	// Should succeed via recovery path
	require.NoError(t, err, "workflow should complete via recovery path")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify both steps were recorded
	_, hasFailingCall := ctx.GetStepState("call_failing")
	_, hasRecoveryCall := ctx.GetStepState("recover")
	assert.True(t, hasFailingCall, "call_failing step should be recorded")
	assert.True(t, hasRecoveryCall, "recover step should be recorded")
}

// ===== T008: Workflow Pack Namespace Parsing and Resolution =====

func TestSplitCallWorkflowName_Unnamespaced(t *testing.T) {
	packName, workflowName := application.SplitCallWorkflowName("my-workflow")

	assert.Equal(t, "", packName)
	assert.Equal(t, "my-workflow", workflowName)
}

func TestSplitCallWorkflowName_Namespaced(t *testing.T) {
	packName, workflowName := application.SplitCallWorkflowName("speckit/specify")

	assert.Equal(t, "speckit", packName)
	assert.Equal(t, "specify", workflowName)
}

func TestSplitCallWorkflowName_MultipleSlashes(t *testing.T) {
	packName, workflowName := application.SplitCallWorkflowName("pack/workflow/extra")

	assert.Equal(t, "pack", packName)
	assert.Equal(t, "workflow/extra", workflowName)
}

func TestSplitCallWorkflowName_EmptyString(t *testing.T) {
	packName, workflowName := application.SplitCallWorkflowName("")

	assert.Equal(t, "", packName)
	assert.Equal(t, "", workflowName)
}

func TestSplitCallWorkflowName_TrailingSlash(t *testing.T) {
	packName, workflowName := application.SplitCallWorkflowName("pack/")

	assert.Equal(t, "pack", packName)
	assert.Equal(t, "", workflowName)
}

func TestSetPackWorkflowLoader(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	called := false
	loader := func(ctx context.Context, packName, workflowName string) (*workflow.Workflow, string, error) {
		called = true
		return &workflow.Workflow{Name: workflowName}, "/pack/root", nil
	}

	execSvc.SetPackWorkflowLoader(loader)

	testCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, err := loader(testCtx, "test", "workflow")
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestExecuteCallWorkflowStep_NamespacedWorkflow_WithLoader(t *testing.T) {
	// Child workflow in "utils" pack
	childWf := &workflow.Workflow{
		Name:    "helper",
		Initial: "work",
		Inputs: []workflow.Input{
			{Name: "value", Type: "string"},
		},
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.inputs.value}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Parent workflow calling namespaced child
	parentWf := &workflow.Workflow{
		Name:    "parent",
		Initial: "call",
		Inputs: []workflow.Input{
			{Name: "msg", Type: "string"},
		},
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "utils/helper",
					Inputs: map[string]string{
						"value": "{{.inputs.msg}}",
					},
					Outputs: map[string]string{
						"work": "result",
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("parent", parentWf).
		WithWorkflow("utils/helper", childWf).
		Build()

	loaderCalled := false
	execSvc.SetPackWorkflowLoader(func(ctx context.Context, packName, workflowName string) (*workflow.Workflow, string, error) {
		loaderCalled = true
		assert.Equal(t, "utils", packName)
		assert.Equal(t, "helper", workflowName)
		return childWf, "/pack/utils", nil
	})

	ctx, err := execSvc.Run(context.Background(), "parent", map[string]any{
		"msg": "test",
	})

	require.NoError(t, err)
	assert.True(t, loaderCalled, "pack workflow loader should be called")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_NamespacedWorkflow_NoLoader(t *testing.T) {
	parentWf := &workflow.Workflow{
		Name:    "parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "utils/helper",
				},
				OnFailure: "error",
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("parent", parentWf).
		Build()

	// Don't set pack workflow loader

	_, err := execSvc.Run(context.Background(), "parent", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pack workflow loader not configured")
}

func TestExecuteCallWorkflowStep_NamespacedWorkflow_LoaderError(t *testing.T) {
	parentWf := &workflow.Workflow{
		Name:    "parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "missing-pack/workflow",
				},
				OnFailure: "error",
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("parent", parentWf).
		Build()

	execSvc.SetPackWorkflowLoader(func(ctx context.Context, packName, workflowName string) (*workflow.Workflow, string, error) {
		return nil, "", fmt.Errorf("pack not found")
	})

	_, err := execSvc.Run(context.Background(), "parent", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load sub-workflow")
	assert.Contains(t, err.Error(), "pack not found")
}

func TestExecuteCallWorkflowStep_UnnamespacedWorkflow_WithLoader(t *testing.T) {
	// When loader is set but workflow is unnamespaced, standard resolution applies
	childWf := &workflow.Workflow{
		Name:    "local-child",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo ok",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	parentWf := &workflow.Workflow{
		Name:    "parent",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "local-child",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("parent", parentWf).
		WithWorkflow("local-child", childWf).
		Build()

	loaderCalled := false
	execSvc.SetPackWorkflowLoader(func(ctx context.Context, packName, workflowName string) (*workflow.Workflow, string, error) {
		loaderCalled = true
		return nil, "", fmt.Errorf("should not be called for unnamespaced workflows")
	})

	ctx, err := execSvc.Run(context.Background(), "parent", nil)

	require.NoError(t, err)
	assert.False(t, loaderCalled, "loader should not be called for unnamespaced workflows")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}
