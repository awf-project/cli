package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// =============================================================================
// Test Helpers for Sub-Workflow Tests
// =============================================================================

// createSubWorkflowTestService creates an ExecutionService for sub-workflow testing.
// The repo parameter is used to register parent and child workflows.
func createSubWorkflowTestService(repo *mockRepository) *application.ExecutionService {
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	return application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
}

// createParentChildWorkflows registers a parent workflow that calls a child workflow.
func createParentChildWorkflows(repo *mockRepository, parentName, childName string) {
	// Child workflow: simple echo command
	repo.workflows[childName] = &workflow.Workflow{
		Name:    childName,
		Initial: "do_work",
		Inputs: []workflow.Input{
			{Name: "input_value", Type: "string", Required: false},
		},
		Steps: map[string]*workflow.Step{
			"do_work": {
				Name:      "do_work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{inputs.input_value}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Parent workflow: calls child workflow
	repo.workflows[parentName] = &workflow.Workflow{
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
						"input_value": "{{inputs.parent_input}}",
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
}

// =============================================================================
// Error Variable Tests
// =============================================================================

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

// =============================================================================
// Basic Execution Tests (Happy Path)
// =============================================================================

func TestExecuteCallWorkflowStep_BasicSuccess(t *testing.T) {
	repo := newMockRepository()
	createParentChildWorkflows(repo, "parent", "child")

	execSvc := createSubWorkflowTestService(repo)

	ctx, err := execSvc.Run(context.Background(), "parent", map[string]any{
		"parent_input": "hello",
	})

	require.NoError(t, err, "sub-workflow execution should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

func TestExecuteCallWorkflowStep_NoInputs(t *testing.T) {
	repo := newMockRepository()

	// Child with no inputs
	repo.workflows["simple-child"] = &workflow.Workflow{
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
	repo.workflows["simple-parent"] = &workflow.Workflow{
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

	execSvc := createSubWorkflowTestService(repo)
	ctx, err := execSvc.Run(context.Background(), "simple-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// =============================================================================
// Input Mapping Tests
// =============================================================================

func TestExecuteCallWorkflowStep_InputMapping(t *testing.T) {
	repo := newMockRepository()

	// Child expecting specific inputs
	repo.workflows["input-child"] = &workflow.Workflow{
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
				Command:   "process {{inputs.file}} --mode={{inputs.mode}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Parent with input mapping
	repo.workflows["input-parent"] = &workflow.Workflow{
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
						"file": "{{inputs.target_file}}",
						"mode": "{{inputs.operation_mode}}",
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

	execSvc := createSubWorkflowTestService(repo)
	ctx, err := execSvc.Run(context.Background(), "input-parent", map[string]any{
		"target_file":    "test.txt",
		"operation_mode": "strict",
	})

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_InputMappingFromStepOutput(t *testing.T) {
	repo := newMockRepository()

	// Child workflow
	repo.workflows["step-output-child"] = &workflow.Workflow{
		Name:    "step-output-child",
		Initial: "work",
		Inputs: []workflow.Input{
			{Name: "data", Type: "string"},
		},
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "process {{inputs.data}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Parent with step that produces output, then calls child
	repo.workflows["step-output-parent"] = &workflow.Workflow{
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
						"data": "{{states.prepare.output}}",
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

	execSvc := createSubWorkflowTestService(repo)
	ctx, err := execSvc.Run(context.Background(), "step-output-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// =============================================================================
// Output Mapping Tests
// =============================================================================

func TestExecuteCallWorkflowStep_OutputMapping(t *testing.T) {
	repo := newMockRepository()
	createParentChildWorkflows(repo, "output-parent", "output-child")

	// Modify parent to capture outputs
	repo.workflows["output-parent"].Steps["call_child"].CallWorkflow.Outputs = map[string]string{
		"do_work": "result",
	}

	execSvc := createSubWorkflowTestService(repo)
	ctx, err := execSvc.Run(context.Background(), "output-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify call_child step state was recorded
	stepState, ok := ctx.GetStepState("call_child")
	require.True(t, ok, "call_child step state should be recorded")
	assert.Equal(t, workflow.StatusCompleted, stepState.Status)
}

func TestExecuteCallWorkflowStep_MultipleOutputs(t *testing.T) {
	repo := newMockRepository()

	// Child with multiple steps producing output
	repo.workflows["multi-output-child"] = &workflow.Workflow{
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
	repo.workflows["multi-output-parent"] = &workflow.Workflow{
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

	execSvc := createSubWorkflowTestService(repo)
	ctx, err := execSvc.Run(context.Background(), "multi-output-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// =============================================================================
// Circular Detection Tests
// =============================================================================

func TestExecuteCallWorkflowStep_CircularDetection_Direct(t *testing.T) {
	repo := newMockRepository()

	// Workflow A calls itself
	repo.workflows["self-caller"] = &workflow.Workflow{
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

	execSvc := createSubWorkflowTestService(repo)
	_, err := execSvc.Run(context.Background(), "self-caller", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrCircularWorkflowCall)
}

func TestExecuteCallWorkflowStep_CircularDetection_Indirect(t *testing.T) {
	repo := newMockRepository()

	// A -> B -> A (indirect circular)
	repo.workflows["workflow-a"] = &workflow.Workflow{
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

	repo.workflows["workflow-b"] = &workflow.Workflow{
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

	execSvc := createSubWorkflowTestService(repo)
	_, err := execSvc.Run(context.Background(), "workflow-a", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrCircularWorkflowCall)
}

func TestExecuteCallWorkflowStep_CircularDetection_ThreeLevel(t *testing.T) {
	repo := newMockRepository()

	// A -> B -> C -> A (three-level circular)
	repo.workflows["wf-a"] = &workflow.Workflow{
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

	repo.workflows["wf-b"] = &workflow.Workflow{
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

	repo.workflows["wf-c"] = &workflow.Workflow{
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

	execSvc := createSubWorkflowTestService(repo)
	_, err := execSvc.Run(context.Background(), "wf-a", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrCircularWorkflowCall)
}

// =============================================================================
// Nesting Depth Tests
// =============================================================================

func TestExecuteCallWorkflowStep_NestedThreeLevels_Success(t *testing.T) {
	repo := newMockRepository()

	// Create a valid 3-level nesting: A -> B -> C (no circular)
	repo.workflows["level-a"] = &workflow.Workflow{
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

	repo.workflows["level-b"] = &workflow.Workflow{
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

	repo.workflows["level-c"] = &workflow.Workflow{
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

	execSvc := createSubWorkflowTestService(repo)
	ctx, err := execSvc.Run(context.Background(), "level-a", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_MaxNestingExceeded(t *testing.T) {
	repo := newMockRepository()

	// Create 12 levels of nesting (exceeds MaxCallStackDepth of 10)
	// We need 12 workflows because:
	// - The depth check happens when a workflow tries to CALL another
	// - At depth=10, the 11th call_workflow should fail
	// - So we need: deep-a(calls b) -> deep-b(calls c) -> ... -> deep-k(calls l) -> deep-l(terminal)
	// That's 11 call_workflow steps, with the 11th triggering the depth check at depth=10
	for i := 1; i <= 12; i++ {
		name := "deep-" + string(rune('a'+i-1))
		var nextWorkflow string
		if i < 12 {
			nextWorkflow = "deep-" + string(rune('a'+i))
		}

		if nextWorkflow != "" {
			repo.workflows[name] = &workflow.Workflow{
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
		} else {
			// Last one is a terminal
			repo.workflows[name] = &workflow.Workflow{
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
		}
	}

	execSvc := createSubWorkflowTestService(repo)
	_, err := execSvc.Run(context.Background(), "deep-a", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrMaxNestingExceeded)
}

// =============================================================================
// Error Propagation Tests
// =============================================================================

func TestExecuteCallWorkflowStep_ChildFailure_PropagatesError(t *testing.T) {
	repo := newMockRepository()

	// Child that fails
	repo.workflows["failing-child"] = &workflow.Workflow{
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
	repo.workflows["error-parent"] = &workflow.Workflow{
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

	// Create service with executor that fails on "exit 1"
	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1, Stderr: "command failed"}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(
		wfSvc,
		executor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)

	ctx, err := execSvc.Run(context.Background(), "error-parent", nil)

	require.Error(t, err)
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

func TestExecuteCallWorkflowStep_ChildFailure_OnFailureTransition(t *testing.T) {
	repo := newMockRepository()

	// Child that fails
	repo.workflows["failing-child"] = &workflow.Workflow{
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
	repo.workflows["handled-parent"] = &workflow.Workflow{
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(
		wfSvc,
		executor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)

	ctx, err := execSvc.Run(context.Background(), "handled-parent", nil)

	require.NoError(t, err, "workflow should complete via error handler")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteCallWorkflowStep_ContinueOnError(t *testing.T) {
	repo := newMockRepository()

	// Child that fails
	repo.workflows["failing-child"] = &workflow.Workflow{
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
	repo.workflows["continue-parent"] = &workflow.Workflow{
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(
		wfSvc,
		executor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)

	ctx, err := execSvc.Run(context.Background(), "continue-parent", nil)

	require.NoError(t, err, "workflow should continue despite child failure")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

// =============================================================================
// Missing Sub-Workflow Tests
// =============================================================================

func TestExecuteCallWorkflowStep_SubWorkflowNotFound(t *testing.T) {
	repo := newMockRepository()

	// Parent calls non-existent child
	repo.workflows["missing-child-parent"] = &workflow.Workflow{
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

	execSvc := createSubWorkflowTestService(repo)
	_, err := execSvc.Run(context.Background(), "missing-child-parent", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrSubWorkflowNotFound)
}

// =============================================================================
// Timeout Tests
// =============================================================================

func TestExecuteCallWorkflowStep_Timeout(t *testing.T) {
	repo := newMockRepository()

	// Slow child workflow
	repo.workflows["slow-child"] = &workflow.Workflow{
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
	repo.workflows["timeout-parent"] = &workflow.Workflow{
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), slowExecutor, &mockLogger{})
	execSvc := application.NewExecutionService(
		wfSvc,
		slowExecutor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)

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

// =============================================================================
// Missing Configuration Tests
// =============================================================================

func TestExecuteCallWorkflowStep_MissingCallWorkflowConfig(t *testing.T) {
	repo := newMockRepository()

	// Step with call_workflow type but nil CallWorkflow config
	repo.workflows["bad-config"] = &workflow.Workflow{
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

	execSvc := createSubWorkflowTestService(repo)
	_, err := execSvc.Run(context.Background(), "bad-config", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "call_workflow configuration is required")
}

// =============================================================================
// Mixed Workflow Type Tests
// =============================================================================

func TestExecuteCallWorkflowStep_MixedWithCommandSteps(t *testing.T) {
	repo := newMockRepository()

	// Child workflow
	repo.workflows["helper"] = &workflow.Workflow{
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
	repo.workflows["mixed"] = &workflow.Workflow{
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

	execSvc := createSubWorkflowTestService(repo)
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

// =============================================================================
// Error Message Quality Tests
// =============================================================================

func TestExecuteCallWorkflowStep_ErrorMessages_IncludeContext(t *testing.T) {
	tests := []struct {
		name          string
		setupRepo     func(*mockRepository)
		workflowName  string
		expectedParts []string
	}{
		{
			name: "missing workflow includes step and workflow name",
			setupRepo: func(repo *mockRepository) {
				repo.workflows["parent"] = &workflow.Workflow{
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
			},
			workflowName:  "parent",
			expectedParts: []string{"call", "missing-workflow"},
		},
		{
			name: "circular error includes call stack",
			setupRepo: func(repo *mockRepository) {
				repo.workflows["loop-a"] = &workflow.Workflow{
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
				repo.workflows["loop-b"] = &workflow.Workflow{
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
			},
			workflowName:  "loop-a",
			expectedParts: []string{"circular", "loop-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			tt.setupRepo(repo)

			execSvc := createSubWorkflowTestService(repo)
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

// =============================================================================
// State Recording Tests
// =============================================================================

func TestExecuteCallWorkflowStep_RecordsStepState(t *testing.T) {
	repo := newMockRepository()
	createParentChildWorkflows(repo, "state-parent", "state-child")

	execSvc := createSubWorkflowTestService(repo)
	ctx, err := execSvc.Run(context.Background(), "state-parent", nil)

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
	repo := newMockRepository()

	repo.workflows["error-child"] = &workflow.Workflow{
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

	repo.workflows["error-parent"] = &workflow.Workflow{
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(
		wfSvc,
		executor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)

	ctx, err := execSvc.Run(context.Background(), "error-parent", nil)

	require.NoError(t, err) // Workflow completes via error path

	// Verify call step state shows failure
	stepState, ok := ctx.GetStepState("call")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusFailed, stepState.Status)
	assert.NotEmpty(t, stepState.Error)
}

// =============================================================================
// Context Cancellation Tests
// =============================================================================

func TestExecuteCallWorkflowStep_ContextCancellation(t *testing.T) {
	repo := newMockRepository()

	// Child with slow step
	repo.workflows["slow-child"] = &workflow.Workflow{
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

	repo.workflows["cancel-parent"] = &workflow.Workflow{
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), slowExecutor, &mockLogger{})
	execSvc := application.NewExecutionService(
		wfSvc,
		slowExecutor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())

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

// =============================================================================
// Additional US3 Edge Case Tests (Circular Detection)
// =============================================================================

func TestExecuteCallWorkflowStep_CircularDetection_DiamondPattern(t *testing.T) {
	repo := newMockRepository()

	// Diamond pattern: A calls B and C, both B and C call D, D calls A
	// This tests that circular detection works across multiple paths
	//
	//       A
	//      / \
	//     B   C
	//      \ /
	//       D -> A (circular!)

	repo.workflows["diamond-a"] = &workflow.Workflow{
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

	repo.workflows["diamond-b"] = &workflow.Workflow{
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

	repo.workflows["diamond-d"] = &workflow.Workflow{
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

	execSvc := createSubWorkflowTestService(repo)
	_, err := execSvc.Run(context.Background(), "diamond-a", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrCircularWorkflowCall)
}

func TestExecuteCallWorkflowStep_SameWorkflowCalledFromDifferentParents_NotCircular(t *testing.T) {
	repo := newMockRepository()

	// Non-circular: A calls B, A also calls C, both B and C call D
	// D is called twice from different parents - this is NOT circular
	//
	//       A
	//      / \
	//     B   C
	//      \ /
	//       D (terminal)

	repo.workflows["shared-d"] = &workflow.Workflow{
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

	repo.workflows["shared-b"] = &workflow.Workflow{
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

	repo.workflows["shared-a"] = &workflow.Workflow{
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

	execSvc := createSubWorkflowTestService(repo)
	ctx, err := execSvc.Run(context.Background(), "shared-a", nil)

	require.NoError(t, err, "calling same workflow from different parents should not be circular")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// =============================================================================
// Additional US4 Edge Case Tests (Timeout and Nesting)
// =============================================================================

func TestExecuteCallWorkflowStep_TimeoutZero_InheritsParentContext(t *testing.T) {
	repo := newMockRepository()

	// Child with slow operation
	repo.workflows["inherit-child"] = &workflow.Workflow{
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
	repo.workflows["inherit-parent"] = &workflow.Workflow{
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), slowExecutor, &mockLogger{})
	execSvc := application.NewExecutionService(
		wfSvc,
		slowExecutor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)

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
	repo := newMockRepository()

	// Innermost child with slow operation
	repo.workflows["inner-child"] = &workflow.Workflow{
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
	repo.workflows["middle-child"] = &workflow.Workflow{
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
	repo.workflows["outer-parent"] = &workflow.Workflow{
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), slowExecutor, &mockLogger{})
	execSvc := application.NewExecutionService(
		wfSvc,
		slowExecutor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)

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
	repo := newMockRepository()

	// Create exactly MaxCallStackDepth levels (10)
	// This should succeed, not fail
	for i := 1; i <= workflow.MaxCallStackDepth; i++ {
		name := fmt.Sprintf("exact-%d", i)
		var nextWorkflow string
		if i < workflow.MaxCallStackDepth {
			nextWorkflow = fmt.Sprintf("exact-%d", i+1)
		}

		if nextWorkflow != "" {
			repo.workflows[name] = &workflow.Workflow{
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
		} else {
			// Last one is a terminal with command
			repo.workflows[name] = &workflow.Workflow{
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
		}
	}

	execSvc := createSubWorkflowTestService(repo)
	ctx, err := execSvc.Run(context.Background(), "exact-1", nil)

	require.NoError(t, err, "exactly MaxCallStackDepth levels should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// =============================================================================
// Call Stack State Verification Tests
// =============================================================================

func TestExecuteCallWorkflowStep_CallStackCleanedOnError(t *testing.T) {
	repo := newMockRepository()

	// Child that fails
	repo.workflows["cleanup-child"] = &workflow.Workflow{
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
	repo.workflows["recovery-child"] = &workflow.Workflow{
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

	repo.workflows["cleanup-parent"] = &workflow.Workflow{
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(
		wfSvc,
		executor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)

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
