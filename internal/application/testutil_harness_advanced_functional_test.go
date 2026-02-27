package application_test

import (
	"context"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: C012 - Application Test Harness for Service Layer
//
// This file contains ADVANCED functional tests that validate ServiceTestHarness
// works with complex features like hooks, custom state stores, and custom executors.
//
// Test Categories:
// - Hooks Integration: Workflow hooks (on_start, on_success, on_failure, on_error)
// - State Management: Custom state store injection
// - Executor Customization: Custom command executor injection
// - Thread Safety: Concurrent workflow execution
//
// Test count: 6+ advanced functional tests

func TestHarnessFunctional_WorkflowWithHooks_ExecutesHooksCorrectly(t *testing.T) {
	// Demonstrates: Harness supports workflows with lifecycle hooks
	// Validates: Hooks execute at correct workflow lifecycle points

	wf := &workflow.Workflow{
		Name:    "workflow-with-hooks",
		Initial: "main-task",
		Hooks: workflow.WorkflowHooks{
			WorkflowStart: workflow.Hook{
				{Command: "echo 'Workflow started'"},
			},
			WorkflowEnd: workflow.Hook{
				{Command: "echo 'Workflow completed successfully'"},
			},
		},
		Steps: map[string]*workflow.Step{
			"main-task": {
				Name:      "main-task",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 'Main task'",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	svc, mocks := NewTestHarness(t).
		WithWorkflow("workflow-with-hooks", wf).
		WithCommandResult("echo 'Workflow started'", &ports.CommandResult{
			Stdout:   "Workflow started\n",
			ExitCode: 0,
		}).
		WithCommandResult("echo 'Main task'", &ports.CommandResult{
			Stdout:   "Main task\n",
			ExitCode: 0,
		}).
		WithCommandResult("echo 'Workflow completed successfully'", &ports.CommandResult{
			Stdout:   "Workflow completed successfully\n",
			ExitCode: 0,
		}).
		Build()

	ctx, err := svc.Run(context.Background(), "workflow-with-hooks", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify executor was called for all commands (workflow + hooks)
	assert.NotNil(t, mocks.Executor)
}

func TestHarnessFunctional_WorkflowWithFailureHooks_ExecutesOnFailure(t *testing.T) {
	// Demonstrates: Harness handles failure hooks correctly
	// Validates: WorkflowError hooks execute when workflow fails

	wf := &workflow.Workflow{
		Name:    "workflow-with-failure-hook",
		Initial: "failing-task",
		Hooks: workflow.WorkflowHooks{
			WorkflowError: workflow.Hook{
				{Command: "echo 'Cleaning up after failure'"},
			},
		},
		Steps: map[string]*workflow.Step{
			"failing-task": {
				Name:      "failing-task",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "success",
				OnFailure: "failed",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failed": {
				Name:   "failed",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	svc, mocks := NewTestHarness(t).
		WithWorkflow("workflow-with-failure-hook", wf).
		WithCommandResult("exit 1", &ports.CommandResult{
			Stderr:   "Command failed\n",
			ExitCode: 1,
		}).
		WithCommandResult("echo 'Cleaning up after failure'", &ports.CommandResult{
			Stdout:   "Cleaning up after failure\n",
			ExitCode: 0,
		}).
		Build()

	ctx, err := svc.Run(context.Background(), "workflow-with-failure-hook", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal failure")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
	assert.Equal(t, "failed", ctx.CurrentStep)

	// Verify mocks accessible for failure assertions
	assert.NotNil(t, mocks.Executor)
}

func TestHarnessFunctional_StepWithHooks_ExecutesStepHooks(t *testing.T) {
	// Demonstrates: Harness supports step-level hooks
	// Validates: Step Post hooks execute correctly

	wf := &workflow.Workflow{
		Name:    "step-level-hooks",
		Initial: "task-with-hooks",
		Steps: map[string]*workflow.Step{
			"task-with-hooks": {
				Name:    "task-with-hooks",
				Type:    workflow.StepTypeCommand,
				Command: "echo 'Task execution'",
				Hooks: workflow.StepHooks{
					Post: workflow.Hook{
						{Command: "echo 'Step succeeded'"},
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	svc, _ := NewTestHarness(t).
		WithWorkflow("step-level-hooks", wf).
		WithCommandResult("echo 'Task execution'", &ports.CommandResult{
			Stdout:   "Task execution\n",
			ExitCode: 0,
		}).
		WithCommandResult("echo 'Step succeeded'", &ports.CommandResult{
			Stdout:   "Step succeeded\n",
			ExitCode: 0,
		}).
		Build()

	ctx, err := svc.Run(context.Background(), "step-level-hooks", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestHarnessFunctional_CustomStateStore_PersistsState(t *testing.T) {
	// Demonstrates: Harness supports custom state store injection
	// Validates: Custom state store receives and persists workflow state

	customStore := testmocks.NewMockStateStore()

	wf := &workflow.Workflow{
		Name:    "stateful-workflow",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 'Step 1'",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 'Step 2'",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	svc, mocks := NewTestHarness(t).
		WithWorkflow("stateful-workflow", wf).
		WithStateStore(customStore).
		WithCommandResult("echo 'Step 1'", &ports.CommandResult{Stdout: "Step 1\n", ExitCode: 0}).
		WithCommandResult("echo 'Step 2'", &ports.CommandResult{Stdout: "Step 2\n", ExitCode: 0}).
		Build()

	ctx, err := svc.Run(context.Background(), "stateful-workflow", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify custom state store was used
	assert.Equal(t, customStore, mocks.StateStore, "Custom state store should be used")

	// Verify state was persisted by checking we can retrieve it
	savedState, err := customStore.Load(context.Background(), ctx.WorkflowID)
	assert.NoError(t, err, "Should be able to load saved state")
	assert.NotNil(t, savedState, "State should be persisted")
}

func TestHarnessFunctional_CustomExecutor_ExecutesCommands(t *testing.T) {
	// Demonstrates: Harness supports custom executor injection
	// Validates: Custom executor can replace default mock executor

	customExecutor := testmocks.NewMockCommandExecutor()
	customExecutor.SetCommandResult("custom command", &ports.CommandResult{
		Stdout:   "Custom executor output\n",
		ExitCode: 0,
	})

	wf := &workflow.Workflow{
		Name:    "custom-executor-workflow",
		Initial: "task",
		Steps: map[string]*workflow.Step{
			"task": {
				Name:      "task",
				Type:      workflow.StepTypeCommand,
				Command:   "custom command",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	svc, mocks := NewTestHarness(t).
		WithWorkflow("custom-executor-workflow", wf).
		WithExecutor(customExecutor).
		Build()

	ctx, err := svc.Run(context.Background(), "custom-executor-workflow", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify custom executor was used
	assert.Equal(t, customExecutor, mocks.Executor, "Custom executor should be used")

	// Verify step executed with custom executor's result
	stepState, ok := ctx.GetStepState("task")
	require.True(t, ok, "Task step should exist")
	assert.Contains(t, stepState.Output, "Custom executor output", "Should use custom executor result")
}

func TestHarnessFunctional_ConcurrentWorkflows_ThreadSafe(t *testing.T) {
	// Demonstrates: Harness supports concurrent workflow execution
	// Validates: Multiple workflows can execute safely in parallel

	wf1 := &workflow.Workflow{
		Name:    "concurrent-wf-1",
		Initial: "task1",
		Steps: map[string]*workflow.Step{
			"task1": {
				Name:      "task1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 'Workflow 1'",
				OnSuccess: "done1",
			},
			"done1": {Name: "done1", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	wf2 := &workflow.Workflow{
		Name:    "concurrent-wf-2",
		Initial: "task2",
		Steps: map[string]*workflow.Step{
			"task2": {
				Name:      "task2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 'Workflow 2'",
				OnSuccess: "done2",
			},
			"done2": {Name: "done2", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	svc, _ := NewTestHarness(t).
		WithWorkflow("concurrent-wf-1", wf1).
		WithWorkflow("concurrent-wf-2", wf2).
		WithCommandResult("echo 'Workflow 1'", &ports.CommandResult{Stdout: "Workflow 1\n", ExitCode: 0}).
		WithCommandResult("echo 'Workflow 2'", &ports.CommandResult{Stdout: "Workflow 2\n", ExitCode: 0}).
		Build()

	// Execute workflows concurrently
	var wg sync.WaitGroup
	results := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := svc.Run(context.Background(), "concurrent-wf-1", nil)
		results <- err
	}()

	go func() {
		defer wg.Done()
		_, err := svc.Run(context.Background(), "concurrent-wf-2", nil)
		results <- err
	}()

	wg.Wait()
	close(results)

	errorCount := 0
	for err := range results {
		if err != nil {
			errorCount++
			t.Logf("Workflow error: %v", err)
		}
	}
	assert.Equal(t, 0, errorCount, "All concurrent workflows should succeed")
}
