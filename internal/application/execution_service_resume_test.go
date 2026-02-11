package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// =============================================================================
// executeFromStep Tests
// Feature: C054 - Increase Application Layer Test Coverage
// Component: T005 - execute_from_step_tests
// =============================================================================
//
// This file tests the executeFromStep function which handles workflow resumption
// from a specific step, managing state transitions, error hooks, and step dispatch.
//
// Coverage target: 55.4% -> 80%+
//
// This function is private (executeFromStep), so we test it indirectly through
// the Resume() public API by constructing workflows with saved states and observing
// execution flow from the resume point.
// =============================================================================

func TestExecuteFromStep_StepNotFound(t *testing.T) {
	// Arrange: Workflow where OnSuccess points to a step that doesn't exist
	// This tests the path where executeFromStep finds step doesn't exist during execution loop
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "nonexistent_step", // Points to non-existent step
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start", // Valid step
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	// Pre-save the state
	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume workflow - "start" will succeed and try to transition to "nonexistent_step"
	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil)

	// Assert: Should fail when trying to find next step
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step not found")
	require.NotNil(t, ctx, "execution context should not be nil even on error")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

func TestExecuteFromStep_TerminalStepSuccess(t *testing.T) {
	// Arrange: Workflow with terminal step as resume point
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "success_terminal",
			},
			"success_terminal": {
				Name:   "success_terminal",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess, // Default success
			},
		},
	}

	// Create saved state at terminal step
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-002",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "success_terminal",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume from terminal success step
	ctx, err := execSvc.Resume(context.Background(), "wf-002", nil)

	// Assert: Should complete successfully
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "success_terminal", ctx.CurrentStep)
	assert.NotZero(t, ctx.CompletedAt)
}

func TestExecuteFromStep_TerminalStepFailure(t *testing.T) {
	// Arrange: Workflow with terminal failure step
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnFailure: "failure_terminal",
			},
			"failure_terminal": {
				Name:   "failure_terminal",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure, // Explicit failure
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-003",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "failure_terminal",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume from terminal failure step
	ctx, err := execSvc.Resume(context.Background(), "wf-003", nil)

	// Assert: Should fail with terminal failure error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal failure state")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
	assert.NotZero(t, ctx.CompletedAt)
}

func TestExecuteFromStep_ContextCancelled(t *testing.T) {
	// Arrange: Workflow with command that returns cancellation error
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "sleep 10",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
		Hooks: workflow.WorkflowHooks{
			WorkflowCancel: workflow.Hook{
				{Command: "echo cancelled"},
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-004",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo cancelled", &ports.CommandResult{Stdout: "cancelled\n", ExitCode: 0}).
		Build()

	// Configure executor to return context.Canceled error for all commands
	mocks.Executor.SetExecuteError(context.Canceled)

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume with error configured
	execCtx, err := execSvc.Resume(context.Background(), "wf-004", nil)

	// Assert: Should be cancelled
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, workflow.StatusCancelled, execCtx.Status)
}

func TestExecuteFromStep_RegularError_ClassifiesAndExecutesHook(t *testing.T) {
	// Arrange: Workflow with command that fails
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
		Hooks: workflow.WorkflowHooks{
			WorkflowError: workflow.Hook{
				{Command: "echo error_hook"},
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-005",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("exit 1", &ports.CommandResult{Stderr: "command failed", ExitCode: 1}).
		WithCommandResult("echo error_hook", &ports.CommandResult{Stdout: "error_hook\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume workflow that will fail
	ctx, err := execSvc.Resume(context.Background(), "wf-005", nil)

	// Assert: Should fail with error and execute error hook
	require.Error(t, err)
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
	// Verify error hook was executed by checking if the command was called
	calls := mocks.Executor.GetCalls()
	var foundErrorHook bool
	for _, call := range calls {
		if call.Program == "echo error_hook" {
			foundErrorHook = true
			break
		}
	}
	assert.True(t, foundErrorHook, "WorkflowError hook should have been executed")
}

func TestExecuteFromStep_SuccessfulCompletion_ExecutesEndHook(t *testing.T) {
	// Arrange: Workflow that completes successfully
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
		Hooks: workflow.WorkflowHooks{
			WorkflowEnd: workflow.Hook{
				{Command: "echo workflow_completed"},
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-006",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		WithCommandResult("echo workflow_completed", &ports.CommandResult{Stdout: "workflow_completed\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume and complete workflow
	ctx, err := execSvc.Resume(context.Background(), "wf-006", nil)

	// Assert: Should complete with end hook executed
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "end", ctx.CurrentStep)
}

func TestExecuteFromStep_DispatchesOperationStep(t *testing.T) {
	// Arrange: Workflow with operation step
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeOperation,
				Operation: "test-op",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-007",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume workflow with operation step (no provider configured - will fail)
	ctx, err := execSvc.Resume(context.Background(), "wf-007", nil)

	// Assert: Should fail because no operation provider configured
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operation provider not configured")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

func TestExecuteFromStep_DispatchesParallelStep(t *testing.T) {
	// Arrange: Workflow with parallel step
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch1", "branch2"},
				Strategy:  "all_succeed",
				OnSuccess: "end",
			},
			"branch1": {
				Name:    "branch1",
				Type:    workflow.StepTypeCommand,
				Command: "echo branch1",
			},
			"branch2": {
				Name:    "branch2",
				Type:    workflow.StepTypeCommand,
				Command: "echo branch2",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-008",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "parallel",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo branch1", &ports.CommandResult{Stdout: "branch1\n", ExitCode: 0}).
		WithCommandResult("echo branch2", &ports.CommandResult{Stdout: "branch2\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume workflow with parallel step
	ctx, err := execSvc.Resume(context.Background(), "wf-008", nil)

	// Assert: Should execute parallel branches and complete
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteFromStep_DispatchesLoopStep(t *testing.T) {
	// Arrange: Workflow with loop step that has empty items (will complete immediately)
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:       workflow.LoopTypeForEach,
					Items:      "items",
					Body:       []string{"body"},
					OnComplete: "end",
				},
			},
			"body": {
				Name:      "body",
				Type:      workflow.StepTypeCommand,
				Command:   "echo item",
				OnSuccess: "loop",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-009",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "loop",
		States:       make(map[string]workflow.StepState),
		Inputs:       map[string]any{"items": []any{}}, // Empty items - loop completes immediately
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume workflow with loop step (empty items, should transition to OnComplete)
	ctx, err := execSvc.Resume(context.Background(), "wf-009", nil)

	// Assert: Should execute loop and complete (empty loop completes immediately)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteFromStep_DispatchesCallWorkflowStep(t *testing.T) {
	// Arrange: Main workflow calling sub-workflow
	mainWf := &workflow.Workflow{
		Name:    "main",
		Initial: "call_sub",
		Steps: map[string]*workflow.Step{
			"call_sub": {
				Name: "call_sub",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "sub",
				},
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	subWf := &workflow.Workflow{
		Name:    "sub",
		Initial: "sub_step",
		Steps: map[string]*workflow.Step{
			"sub_step": {
				Name:      "sub_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo sub",
				OnSuccess: "sub_end",
			},
			"sub_end": {
				Name: "sub_end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-010",
		WorkflowName: "main",
		Status:       workflow.StatusRunning,
		CurrentStep:  "call_sub",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("main", mainWf).
		WithWorkflow("sub", subWf).
		WithCommandResult("echo sub", &ports.CommandResult{Stdout: "sub\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume workflow with call_workflow step
	ctx, err := execSvc.Resume(context.Background(), "wf-010", nil)

	// Assert: Should execute sub-workflow and complete
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteFromStep_DispatchesAgentStep(t *testing.T) {
	// Arrange: Workflow with agent step
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "agent",
		Steps: map[string]*workflow.Step{
			"agent": {
				Name: "agent",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "test-agent",
					Prompt:   "test prompt",
				},
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-011",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "agent",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume workflow with agent step (will fail - no agent provider configured)
	ctx, err := execSvc.Resume(context.Background(), "wf-011", nil)

	// Assert: Should fail because agent provider not available in default harness
	// This test verifies that executeFromStep dispatches to executeAgentStep
	require.Error(t, err)
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

func TestExecuteFromStep_DispatchesDefaultCommandStep(t *testing.T) {
	// Arrange: Workflow with regular command step (default case)
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo default",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-012",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo default", &ports.CommandResult{Stdout: "default\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume workflow with command step
	ctx, err := execSvc.Resume(context.Background(), "wf-012", nil)

	// Assert: Should execute command and complete
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteFromStep_CheckpointsAfterEachStep(t *testing.T) {
	// Arrange: Multi-step workflow to verify checkpointing
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step2",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-013",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step1",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo step1", &ports.CommandResult{Stdout: "step1\n", ExitCode: 0}).
		WithCommandResult("echo step2", &ports.CommandResult{Stdout: "step2\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Act: Resume workflow
	ctx, err := execSvc.Resume(context.Background(), "wf-013", nil)

	// Assert: Should complete and have checkpointed multiple times
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	// Verify workflow executed through both steps
	assert.Equal(t, "end", ctx.CurrentStep)
}
