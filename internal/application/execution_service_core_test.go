package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	testmocks "github.com/vanoix/awf/internal/testutil/mocks"
	"github.com/vanoix/awf/pkg/interpolation"
)

// Feature: C008 - Test File Restructuring
// Component: T010 - extract_core_tests
//
// This file contains core execution integration tests for ExecutionService.
// Tests verify basic workflow execution, state transitions, context propagation,
// resume functionality, call workflow steps, and step execution fundamentals.
//
// Extracted from: execution_service_test.go (50 core execution tests)
// Test count: 50 core execution tests

// Mock types are defined in execution_service_specialized_mocks_test.go
// Helper functions are defined in execution_service_helpers_test.go

func TestExecutionService_Run_SingleStepWorkflow(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// verify step state captured
	state, ok := ctx.GetStepState("start")
	require.True(t, ok)
	assert.Equal(t, "hello\n", state.Output)
	assert.Equal(t, 0, state.ExitCode)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

func TestExecutionService_Run_MultiStepWorkflow(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "multi",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1", OnSuccess: "step2"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2", OnSuccess: "step3"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("multi", wf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "multi", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// all 3 steps should have been executed
	_, ok := ctx.GetStepState("step1")
	assert.True(t, ok, "step1 should be executed")
	_, ok = ctx.GetStepState("step2")
	assert.True(t, ok, "step2 should be executed")
	_, ok = ctx.GetStepState("step3")
	assert.True(t, ok, "step3 should be executed")
}

func TestExecutionService_Run_FailureTransition(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "fail-test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "success",
				OnFailure: "error",
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal},
			"error":   {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("fail-test", wf).
		WithCommandResult("exit 1", &ports.CommandResult{ExitCode: 1, Stderr: "failed"}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "fail-test", nil)

	require.NoError(t, err) // workflow completes, just via error path
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "error", ctx.CurrentStep) // ended on error terminal

	state, ok := ctx.GetStepState("start")
	require.True(t, ok)
	assert.Equal(t, 1, state.ExitCode)
	assert.Equal(t, workflow.StatusFailed, state.Status)
}

func TestExecutionService_Run_FailureNoTransition(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "fail-no-transition",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "done",
				// no OnFailure
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("fail-no-transition", wf).
		WithCommandResult("exit 1", &ports.CommandResult{ExitCode: 1}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "fail-no-transition", nil)

	require.Error(t, err) // workflow fails
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

func TestExecutionService_Run_StepTimeout(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "timeout-test",
		Initial: "slow",
		Steps: map[string]*workflow.Step{
			"slow": {
				Name:      "slow",
				Type:      workflow.StepTypeCommand,
				Command:   "sleep 10",
				Timeout:   1, // 1 second
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	// use mock that returns timeout error
	executor := &timeoutMockExecutor{
		timeout: 500 * time.Millisecond,
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("timeout-test", wf).
		WithExecutor(executor).
		Build()

	ctx, err := execSvc.Run(context.Background(), "timeout-test", nil)

	require.NoError(t, err) // workflow completes via error path
	assert.Equal(t, "error", ctx.CurrentStep)
}

func TestExecutionService_Run_WorkflowNotFound(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	_, err := execSvc.Run(context.Background(), "nonexistent", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExecutionService_Run_WithInputs(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "input-test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo test", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("input-test", wf).
		Build()

	inputs := map[string]any{"name": "test", "count": 42}
	ctx, err := execSvc.Run(context.Background(), "input-test", inputs)

	require.NoError(t, err)

	val, ok := ctx.GetInput("name")
	assert.True(t, ok)
	assert.Equal(t, "test", val)

	val, ok = ctx.GetInput("count")
	assert.True(t, ok)
	assert.Equal(t, 42, val)
}

func TestExecutionService_Run_StepNotFound(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "bad-ref",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "nonexistent", // bad reference
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("bad-ref", wf).
		Build()

	_, err := execSvc.Run(context.Background(), "bad-ref", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExecutionService_Run_ImmediateTerminal(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "immediate",
		Initial: "done",
		Steps: map[string]*workflow.Step{
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("immediate", wf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "immediate", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

func TestNewExecutionService(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	assert.NotNil(t, execSvc)
}

func TestExecutionService_Run_ExecutorError(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "exec-error",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "something",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := &errorMockExecutor{err: errors.New("command not found")}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("exec-error", wf).
		WithExecutor(executor).
		Build()

	ctx, err := execSvc.Run(context.Background(), "exec-error", nil)

	require.NoError(t, err) // workflow should complete via error path
	assert.Equal(t, "error", ctx.CurrentStep)
}

func TestExecutionService_Run_WithDir(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "dir-test",
		Initial: "build",
		Steps: map[string]*workflow.Step{
			"build": {
				Name:      "build",
				Type:      workflow.StepTypeCommand,
				Command:   "make build",
				Dir:       "/tmp/project",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newCapturingMockExecutor()

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("dir-test", wf).
		WithExecutor(executor).
		Build()

	_, err := execSvc.Run(context.Background(), "dir-test", nil)

	require.NoError(t, err)
	require.NotNil(t, executor.lastCmd, "executor should have received a command")
	assert.Equal(t, "/tmp/project", executor.lastCmd.Dir, "Dir should be passed to executor")
}

func TestExecutionService_Run_WithDirEmpty(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "no-dir-test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newCapturingMockExecutor()

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("no-dir-test", wf).
		WithExecutor(executor).
		Build()

	_, err := execSvc.Run(context.Background(), "no-dir-test", nil)

	require.NoError(t, err)
	require.NotNil(t, executor.lastCmd, "executor should have received a command")
	assert.Equal(t, "", executor.lastCmd.Dir, "Dir should be empty when not specified")
}

func TestExecutionService_Run_SavesCheckpoints(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "checkpoint-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1", OnSuccess: "step2"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2", OnSuccess: "step3"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("checkpoint-test", wf).
		Build()

	execCtx, err := execSvc.Run(context.Background(), "checkpoint-test", nil)
	require.NoError(t, err)

	// State should have been saved (checkpointed)
	saved, err := mocks.StateStore.Load(context.Background(), execCtx.WorkflowID)
	require.NoError(t, err)
	require.NotNil(t, saved, "state should be checkpointed after execution")
	assert.Equal(t, workflow.StatusCompleted, saved.Status)
	assert.Equal(t, "done", saved.CurrentStep)
}

func TestExecutionService_Run_ContinueOnErrorFollowsOnSuccess(t *testing.T) {
	// When continue_on_error is true, even if the step fails (non-zero exit),
	// it should follow on_success instead of on_failure
	wf := &workflow.Workflow{
		Name:    "continue-on-error",
		Initial: "flaky",
		Steps: map[string]*workflow.Step{
			"flaky": {
				Name:            "flaky",
				Type:            workflow.StepTypeCommand,
				Command:         "exit 1",
				ContinueOnError: true, // this is key
				OnSuccess:       "success",
				OnFailure:       "failure",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failure": {
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("continue-on-error", wf).
		WithCommandResult("exit 1", &ports.CommandResult{ExitCode: 1, Stderr: "command failed"}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "continue-on-error", nil)

	require.NoError(t, err, "workflow should complete without error")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "success", ctx.CurrentStep, "should follow on_success despite exit code 1")

	// The step state should still reflect the failure
	state, ok := ctx.GetStepState("flaky")
	require.True(t, ok)
	assert.Equal(t, 1, state.ExitCode, "step exit code should be 1")
}

func TestExecutionService_Run_ContinueOnErrorWithExecutorError(t *testing.T) {
	// When continue_on_error is true and the executor returns an error,
	// it should still follow on_success
	wf := &workflow.Workflow{
		Name:    "continue-exec-error",
		Initial: "failing",
		Steps: map[string]*workflow.Step{
			"failing": {
				Name:            "failing",
				Type:            workflow.StepTypeCommand,
				Command:         "nonexistent_command",
				ContinueOnError: true,
				OnSuccess:       "success",
				OnFailure:       "failure",
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal},
			"failure": {Name: "failure", Type: workflow.StepTypeTerminal},
		},
	}

	executor := &errorMockExecutor{err: errors.New("command not found")}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("continue-exec-error", wf).
		WithExecutor(executor).
		Build()

	ctx, err := execSvc.Run(context.Background(), "continue-exec-error", nil)

	require.NoError(t, err, "workflow should complete without error")
	assert.Equal(t, "success", ctx.CurrentStep, "should follow on_success despite executor error")
}

func TestExecutionService_Run_ContinueOnErrorFalseFollowsOnFailure(t *testing.T) {
	// When continue_on_error is false (default), failure should follow on_failure
	wf := &workflow.Workflow{
		Name:    "normal-failure",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:            "step",
				Type:            workflow.StepTypeCommand,
				Command:         "exit 1",
				ContinueOnError: false, // default behavior
				OnSuccess:       "success",
				OnFailure:       "failure",
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal},
			"failure": {Name: "failure", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("normal-failure", wf).
		WithCommandResult("exit 1", &ports.CommandResult{ExitCode: 1}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "normal-failure", nil)

	require.NoError(t, err) // workflow completes via failure path
	assert.Equal(t, "failure", ctx.CurrentStep, "should follow on_failure when continue_on_error is false")
}

func TestExecutionService_Run_ContinueOnErrorMultipleSteps(t *testing.T) {
	// Test that continue_on_error works correctly across multiple steps
	wf := &workflow.Workflow{
		Name:    "multi-continue",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:            "step1",
				Type:            workflow.StepTypeCommand,
				Command:         "fail1",
				ContinueOnError: true,
				OnSuccess:       "step2",
				OnFailure:       "error",
			},
			"step2": {
				Name:            "step2",
				Type:            workflow.StepTypeCommand,
				Command:         "fail2",
				ContinueOnError: true,
				OnSuccess:       "step3",
				OnFailure:       "error",
			},
			"step3": {
				Name:      "step3",
				Type:      workflow.StepTypeCommand,
				Command:   "success",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("multi-continue", wf).
		WithCommandResult("fail1", &ports.CommandResult{ExitCode: 1}).
		WithCommandResult("fail2", &ports.CommandResult{ExitCode: 2}).
		WithCommandResult("success", &ports.CommandResult{ExitCode: 0}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "multi-continue", nil)

	require.NoError(t, err)
	assert.Equal(t, "done", ctx.CurrentStep, "should reach done despite step1 and step2 failures")

	// Verify all steps were executed
	_, ok := ctx.GetStepState("step1")
	assert.True(t, ok, "step1 should have been executed")
	_, ok = ctx.GetStepState("step2")
	assert.True(t, ok, "step2 should have been executed")
	_, ok = ctx.GetStepState("step3")
	assert.True(t, ok, "step3 should have been executed")
}

func TestExecutionService_Run_ContinueOnErrorNoOnFailure(t *testing.T) {
	// When continue_on_error is true and there's no on_failure defined,
	// it should still follow on_success on failure
	wf := &workflow.Workflow{
		Name:    "continue-no-onfailure",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:            "step",
				Type:            workflow.StepTypeCommand,
				Command:         "exit 1",
				ContinueOnError: true,
				OnSuccess:       "done",
				// no OnFailure defined
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("continue-no-onfailure", wf).
		WithCommandResult("exit 1", &ports.CommandResult{ExitCode: 1}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "continue-no-onfailure", nil)

	require.NoError(t, err)
	assert.Equal(t, "done", ctx.CurrentStep, "should follow on_success when continue_on_error is true")
}

func TestExecutionService_Run_InputValidation_ValidInputs(t *testing.T) {
	// Workflow with input validation - all inputs valid
	min1, max100 := 1, 100
	wf := &workflow.Workflow{
		Name:    "input-validation",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "email",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
				},
			},
			{
				Name:     "count",
				Type:     "integer",
				Required: true,
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("input-validation", wf).
		Build()

	inputs := map[string]any{
		"email": "test@example.com",
		"count": 50,
	}

	ctx, err := execSvc.Run(context.Background(), "input-validation", inputs)

	require.NoError(t, err, "workflow should complete with valid inputs")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

func TestExecutionService_Run_InputValidation_InvalidEmail(t *testing.T) {
	// Workflow with input validation - invalid email pattern
	wf := &workflow.Workflow{
		Name:    "invalid-email",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "email",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("invalid-email", wf).
		Build()

	inputs := map[string]any{
		"email": "not-an-email",
	}

	_, err := execSvc.Run(context.Background(), "invalid-email", inputs)

	require.Error(t, err, "should fail with invalid email")
	assert.Contains(t, err.Error(), "validation")
	assert.Contains(t, err.Error(), "email")
}

func TestExecutionService_Run_InputValidation_RequiredMissing(t *testing.T) {
	// Required input not provided
	wf := &workflow.Workflow{
		Name:    "required-missing",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "required_field",
				Type:     "string",
				Required: true,
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("required-missing", wf).
		Build()

	// Empty inputs - required field missing
	_, err := execSvc.Run(context.Background(), "required-missing", map[string]any{})

	require.Error(t, err, "should fail with missing required input")
	assert.Contains(t, err.Error(), "validation")
	assert.Contains(t, err.Error(), "required_field")
}

func TestExecutionService_Run_InputValidation_IntegerOutOfRange(t *testing.T) {
	// Integer input outside min/max range
	min1, max100 := 1, 100
	wf := &workflow.Workflow{
		Name:    "integer-range",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "count",
				Type:     "integer",
				Required: true,
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("integer-range", wf).
		Build()

	// count=150 exceeds max=100
	inputs := map[string]any{
		"count": 150,
	}

	_, err := execSvc.Run(context.Background(), "integer-range", inputs)

	require.Error(t, err, "should fail with integer out of range")
	assert.Contains(t, err.Error(), "validation")
	assert.Contains(t, err.Error(), "count")
}

func TestExecutionService_Run_InputValidation_EnumInvalid(t *testing.T) {
	// Enum input with invalid value
	wf := &workflow.Workflow{
		Name:    "enum-validation",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "env",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Enum: []string{"dev", "staging", "prod"},
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("enum-validation", wf).
		Build()

	// "local" not in enum list
	inputs := map[string]any{
		"env": "local",
	}

	_, err := execSvc.Run(context.Background(), "enum-validation", inputs)

	require.Error(t, err, "should fail with invalid enum value")
	assert.Contains(t, err.Error(), "validation")
	assert.Contains(t, err.Error(), "env")
}

func TestExecutionService_Run_InputValidation_MultipleErrors(t *testing.T) {
	// Multiple validation errors should be aggregated
	min1, max100 := 1, 100
	wf := &workflow.Workflow{
		Name:    "multiple-errors",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "email",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
				},
			},
			{
				Name:     "count",
				Type:     "integer",
				Required: true,
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
			{
				Name:     "env",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Enum: []string{"dev", "staging", "prod"},
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("multiple-errors", wf).
		Build()

	// All inputs invalid
	inputs := map[string]any{
		"email": "not-an-email",
		"count": 999,
		"env":   "local",
	}

	_, err := execSvc.Run(context.Background(), "multiple-errors", inputs)

	require.Error(t, err, "should fail with multiple validation errors")
	assert.Contains(t, err.Error(), "validation")
	// Error should mention multiple errors or contain multiple error messages
}

func TestExecutionService_Run_InputValidation_DefaultAppliedBeforeValidation(t *testing.T) {
	// Default values should be applied before validation runs
	min1, max100 := 1, 100
	wf := &workflow.Workflow{
		Name:    "default-values",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "count",
				Type:     "integer",
				Required: true,
				Default:  50, // Default value within range
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("default-values", wf).
		Build()

	// No inputs provided - default should be used
	ctx, err := execSvc.Run(context.Background(), "default-values", nil)

	require.NoError(t, err, "should succeed with default value")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify default was applied
	val, ok := ctx.GetInput("count")
	assert.True(t, ok)
	assert.Equal(t, 50, val)
}

func TestExecutionService_Run_InputValidation_TypeCoercion(t *testing.T) {
	// String "42" should be coerced to integer 42 for validation
	min1, max100 := 1, 100
	wf := &workflow.Workflow{
		Name:    "type-coercion",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "count",
				Type:     "integer",
				Required: true,
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("type-coercion", wf).
		Build()

	// String "42" should be coerced to integer
	inputs := map[string]any{
		"count": "42",
	}

	ctx, err := execSvc.Run(context.Background(), "type-coercion", inputs)

	require.NoError(t, err, "should succeed with coerced type")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_Run_InputValidation_NoValidationRules(t *testing.T) {
	// Inputs without validation rules should still be accepted
	wf := &workflow.Workflow{
		Name:    "no-validation",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "name",
				Type:     "string",
				Required: true,
				// No Validation field
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("no-validation", wf).
		Build()

	inputs := map[string]any{
		"name": "anything_goes",
	}

	ctx, err := execSvc.Run(context.Background(), "no-validation", inputs)

	require.NoError(t, err, "should succeed without validation rules")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_Run_InputValidation_NoInputDefinitions(t *testing.T) {
	// Workflow without any input definitions should work
	wf := &workflow.Workflow{
		Name:    "no-inputs",
		Initial: "start",
		// No Inputs field
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("no-inputs", wf).
		Build()

	// Pass some inputs anyway - should be ignored
	inputs := map[string]any{
		"extra": "value",
	}

	ctx, err := execSvc.Run(context.Background(), "no-inputs", inputs)

	require.NoError(t, err, "should succeed without input definitions")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_Run_InputValidation_BooleanType(t *testing.T) {
	// Boolean type validation
	wf := &workflow.Workflow{
		Name:    "boolean-validation",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "verbose",
				Type:     "boolean",
				Required: true,
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("boolean-validation", wf).
		Build()

	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{name: "true bool", input: true, wantErr: false},
		{name: "false bool", input: false, wantErr: false},
		{name: "string true", input: "true", wantErr: false},
		{name: "string false", input: "false", wantErr: false},
		{name: "string yes", input: "yes", wantErr: false},
		{name: "string no", input: "no", wantErr: false},
		{name: "invalid string", input: "maybe", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := map[string]any{"verbose": tt.input}
			_, err := execSvc.Run(context.Background(), "boolean-validation", inputs)

			if tt.wantErr {
				require.Error(t, err, "should fail with invalid boolean")
			} else {
				require.NoError(t, err, "should succeed with valid boolean")
			}
		})
	}
}

func TestExecutionService_Run_InputValidation_OptionalWithValidation(t *testing.T) {
	// Optional input with validation rules - should validate if provided
	min1, max100 := 1, 100
	wf := &workflow.Workflow{
		Name:    "optional-validation",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "count",
				Type:     "integer",
				Required: false, // Optional
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("optional-validation", wf).
		Build()

	t.Run("not provided should succeed", func(t *testing.T) {
		ctx, err := execSvc.Run(context.Background(), "optional-validation", nil)
		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	})

	t.Run("valid value should succeed", func(t *testing.T) {
		inputs := map[string]any{"count": 50}
		ctx, err := execSvc.Run(context.Background(), "optional-validation", inputs)
		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	})

	t.Run("invalid value should fail", func(t *testing.T) {
		inputs := map[string]any{"count": 999}
		_, err := execSvc.Run(context.Background(), "optional-validation", inputs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestExecutionService_Resume_Success(t *testing.T) {
	// Setup: Create a workflow with 3 steps, interrupt after step1, resume from step2
	wf := &workflow.Workflow{
		Name:    "resume-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1", OnSuccess: "step2"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2", OnSuccess: "step3"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Pre-populate store with interrupted state (step1 completed, paused at step2)
	interruptedState := &workflow.ExecutionContext{
		WorkflowID:   "test-id-123",
		WorkflowName: "resume-test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step2",
		Inputs:       map[string]any{"key": "value"},
		States: map[string]workflow.StepState{
			"step1": {
				Name:      "step1",
				Status:    workflow.StatusCompleted,
				Output:    "output1\n",
				ExitCode:  0,
				StartedAt: time.Now().Add(-time.Minute),
			},
		},
		Env:       make(map[string]string),
		StartedAt: time.Now().Add(-time.Minute),
		UpdatedAt: time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("resume-test", wf).
		WithCommandResult("echo 2", &ports.CommandResult{Stdout: "output2\n", ExitCode: 0}).
		WithCommandResult("echo 3", &ports.CommandResult{Stdout: "output3\n", ExitCode: 0}).
		Build()
	mocks.StateStore.Save(context.Background(), interruptedState)

	// Execute resume
	ctx, err := execSvc.Resume(context.Background(), "test-id-123", nil)

	require.NoError(t, err, "resume should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status, "workflow should be completed")
	assert.Equal(t, "done", ctx.CurrentStep, "should end at terminal state")

	// Verify step1 was NOT re-executed (output preserved from before)
	state1, ok := ctx.GetStepState("step1")
	require.True(t, ok, "step1 state should exist")
	assert.Equal(t, "output1\n", state1.Output, "step1 output should be preserved")

	// Verify step2 and step3 were executed
	state2, ok := ctx.GetStepState("step2")
	require.True(t, ok, "step2 state should exist")
	assert.Equal(t, workflow.StatusCompleted, state2.Status, "step2 should be completed")

	state3, ok := ctx.GetStepState("step3")
	require.True(t, ok, "step3 state should exist")
	assert.Equal(t, workflow.StatusCompleted, state3.Status, "step3 should be completed")
}

func TestExecutionService_Resume_NotFound(t *testing.T) {
	// Resume non-existent workflow should fail
	execSvc, _ := NewTestHarness(t).Build()

	_, err := execSvc.Resume(context.Background(), "non-existent-id", nil)

	require.Error(t, err, "resume should fail for non-existent workflow")
	assert.Contains(t, err.Error(), "not found", "error should indicate workflow not found")
}

func TestExecutionService_Resume_AlreadyCompleted(t *testing.T) {
	// Resume already-completed workflow should fail
	wf := &workflow.Workflow{
		Name:    "some-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	completedState := &workflow.ExecutionContext{
		WorkflowID:   "completed-id",
		WorkflowName: "some-workflow",
		Status:       workflow.StatusCompleted, // Already completed
		CurrentStep:  "done",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("some-workflow", wf).
		Build()
	mocks.StateStore.Save(context.Background(), completedState)

	_, err := execSvc.Resume(context.Background(), "completed-id", nil)

	require.Error(t, err, "resume should fail for completed workflow")
	assert.Contains(t, err.Error(), "completed", "error should mention workflow is completed")
}

func TestExecutionService_Resume_WorkflowDefinitionNotFound(t *testing.T) {
	// Resume workflow when definition no longer exists
	orphanState := &workflow.ExecutionContext{
		WorkflowID:   "orphan-id",
		WorkflowName: "deleted-workflow", // This workflow no longer exists in repo
		Status:       workflow.StatusRunning,
		CurrentStep:  "step2",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	// No workflows added - "deleted-workflow" doesn't exist
	execSvc, mocks := NewTestHarness(t).Build()
	mocks.StateStore.Save(context.Background(), orphanState)

	_, err := execSvc.Resume(context.Background(), "orphan-id", nil)

	require.Error(t, err, "resume should fail when workflow definition not found")
	assert.Contains(t, err.Error(), "not found", "error should indicate workflow definition not found")
}

func TestExecutionService_Resume_StepNotFound(t *testing.T) {
	// Resume when current step no longer exists in workflow (definition changed)
	wf := &workflow.Workflow{
		Name:    "changed-workflow",
		Initial: "new_step",
		Steps: map[string]*workflow.Step{
			"new_step": {Name: "new_step", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "done"},
			"done":     {Name: "done", Type: workflow.StepTypeTerminal},
			// Note: "old_step" doesn't exist anymore
		},
	}

	staleState := &workflow.ExecutionContext{
		WorkflowID:   "stale-id",
		WorkflowName: "changed-workflow",
		Status:       workflow.StatusRunning,
		CurrentStep:  "old_step", // This step was removed from workflow
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("changed-workflow", wf).
		Build()
	mocks.StateStore.Save(context.Background(), staleState)

	_, err := execSvc.Resume(context.Background(), "stale-id", nil)

	require.Error(t, err, "resume should fail when current step no longer exists")
	assert.Contains(t, err.Error(), "old_step", "error should mention the missing step name")
}

func TestExecutionService_Resume_InputOverrides(t *testing.T) {
	// Resume with input overrides - verify overrides are merged
	wf := &workflow.Workflow{
		Name:    "override-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo {{.inputs.key}}", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	overrideState := &workflow.ExecutionContext{
		WorkflowID:   "override-id",
		WorkflowName: "override-test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step1",
		Inputs:       map[string]any{"key": "original", "unchanged": "value"},
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("override-test", wf).
		Build()
	mocks.StateStore.Save(context.Background(), overrideState)

	// Resume with overrides
	overrides := map[string]any{"key": "overridden"}
	ctx, err := execSvc.Resume(context.Background(), "override-id", overrides)

	require.NoError(t, err, "resume with overrides should succeed")

	// Check that "key" was overridden
	val, ok := ctx.GetInput("key")
	require.True(t, ok, "key input should exist")
	assert.Equal(t, "overridden", val, "key should be overridden")

	// Check that "unchanged" was preserved
	val, ok = ctx.GetInput("unchanged")
	require.True(t, ok, "unchanged input should exist")
	assert.Equal(t, "value", val, "unchanged should be preserved")
}

func TestExecutionService_Resume_SkipsCompletedSteps(t *testing.T) {
	// Verify that completed steps are skipped during resume
	wf := &workflow.Workflow{
		Name:    "skip-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "step1_cmd", OnSuccess: "step2"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "step2_cmd", OnSuccess: "step3"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "step3_cmd", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Track which commands were executed
	executor := newRetryCountingExecutor()

	// step1 and step2 already completed, currently at step3
	skipState := &workflow.ExecutionContext{
		WorkflowID:   "skip-id",
		WorkflowName: "skip-test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step3",
		Inputs:       make(map[string]any),
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", Status: workflow.StatusCompleted, Output: "out1"},
			"step2": {Name: "step2", Status: workflow.StatusCompleted, Output: "out2"},
		},
		Env: make(map[string]string),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("skip-test", wf).
		WithExecutor(executor).
		Build()
	mocks.StateStore.Save(context.Background(), skipState)

	ctx, err := execSvc.Resume(context.Background(), "skip-id", nil)

	require.NoError(t, err, "resume should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify step1 and step2 were NOT executed (commands not called)
	assert.Equal(t, 0, executor.calls["step1_cmd"], "step1 should not be re-executed")
	assert.Equal(t, 0, executor.calls["step2_cmd"], "step2 should not be re-executed")

	// Verify step3 WAS executed
	assert.Equal(t, 1, executor.calls["step3_cmd"], "step3 should be executed")
}

func TestExecutionService_Resume_FailedStatus(t *testing.T) {
	// Can resume a workflow that was in failed status (retry after fixing issue)
	wf := &workflow.Workflow{
		Name:    "failed-resume",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	failedState := &workflow.ExecutionContext{
		WorkflowID:   "failed-id",
		WorkflowName: "failed-resume",
		Status:       workflow.StatusFailed, // Previously failed
		CurrentStep:  "step1",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("failed-resume", wf).
		Build()
	mocks.StateStore.Save(context.Background(), failedState)

	ctx, err := execSvc.Resume(context.Background(), "failed-id", nil)

	require.NoError(t, err, "resume of failed workflow should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_Resume_CancelledStatus(t *testing.T) {
	// Can resume a workflow that was cancelled
	wf := &workflow.Workflow{
		Name:    "cancelled-resume",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	cancelledState := &workflow.ExecutionContext{
		WorkflowID:   "cancelled-id",
		WorkflowName: "cancelled-resume",
		Status:       workflow.StatusCancelled, // Previously cancelled
		CurrentStep:  "step1",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("cancelled-resume", wf).
		Build()
	mocks.StateStore.Save(context.Background(), cancelledState)

	ctx, err := execSvc.Resume(context.Background(), "cancelled-id", nil)

	require.NoError(t, err, "resume of cancelled workflow should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_ListResumable_FiltersCompleted(t *testing.T) {
	// ListResumable should only return non-completed executions
	execSvc, mocks := NewTestHarness(t).Build()

	// Add various states
	mocks.StateStore.Save(context.Background(), &workflow.ExecutionContext{
		WorkflowID:   "running-1",
		WorkflowName: "wf1",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step2",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	})
	mocks.StateStore.Save(context.Background(), &workflow.ExecutionContext{
		WorkflowID:   "failed-1",
		WorkflowName: "wf2",
		Status:       workflow.StatusFailed,
		CurrentStep:  "step1",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	})
	mocks.StateStore.Save(context.Background(), &workflow.ExecutionContext{
		WorkflowID:   "completed-1",
		WorkflowName: "wf3",
		Status:       workflow.StatusCompleted, // Should be filtered out
		CurrentStep:  "done",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	})
	mocks.StateStore.Save(context.Background(), &workflow.ExecutionContext{
		WorkflowID:   "cancelled-1",
		WorkflowName: "wf4",
		Status:       workflow.StatusCancelled,
		CurrentStep:  "step1",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	})
	mocks.StateStore.Save(context.Background(), &workflow.ExecutionContext{
		WorkflowID:   "pending-1",
		WorkflowName: "wf5",
		Status:       workflow.StatusPending,
		CurrentStep:  "",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	})

	resumable, err := execSvc.ListResumable(context.Background())

	require.NoError(t, err, "list resumable should succeed")
	assert.Len(t, resumable, 4, "should return 4 resumable workflows (all except completed)")

	// Verify completed is not in the list
	for _, exec := range resumable {
		assert.NotEqual(t, workflow.StatusCompleted, exec.Status, "completed workflows should not be included")
	}
}

func TestExecutionService_ListResumable_Empty(t *testing.T) {
	// ListResumable with no states should return empty list
	execSvc, _ := NewTestHarness(t).Build()

	resumable, err := execSvc.ListResumable(context.Background())

	require.NoError(t, err, "list resumable should succeed")
	assert.Empty(t, resumable)
}

func TestExecutionService_ListResumable_AllCompleted(t *testing.T) {
	// ListResumable when all workflows are completed should return empty
	execSvc, mocks := NewTestHarness(t).Build()
	mocks.StateStore.Save(context.Background(), &workflow.ExecutionContext{
		WorkflowID:   "completed-1",
		WorkflowName: "wf1",
		Status:       workflow.StatusCompleted,
		CurrentStep:  "done",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	})
	mocks.StateStore.Save(context.Background(), &workflow.ExecutionContext{
		WorkflowID:   "completed-2",
		WorkflowName: "wf2",
		Status:       workflow.StatusCompleted,
		CurrentStep:  "done",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	})

	resumable, err := execSvc.ListResumable(context.Background())

	require.NoError(t, err, "list resumable should succeed")
	assert.Empty(t, resumable, "should return empty list when all completed")
}

func TestExecutionService_ListResumable_ReturnsCorrectFields(t *testing.T) {
	// Verify ListResumable returns all required fields
	now := time.Now()
	execSvc, mocks := NewTestHarness(t).Build()
	mocks.StateStore.Save(context.Background(), &workflow.ExecutionContext{
		WorkflowID:   "test-id",
		WorkflowName: "test-workflow",
		Status:       workflow.StatusRunning,
		CurrentStep:  "current",
		Inputs:       map[string]any{"key": "value"},
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", Status: workflow.StatusCompleted},
		},
		StartedAt: now.Add(-time.Minute),
		UpdatedAt: now,
	})

	resumable, err := execSvc.ListResumable(context.Background())

	require.NoError(t, err)
	require.Len(t, resumable, 1)

	exec := resumable[0]
	assert.Equal(t, "test-id", exec.WorkflowID)
	assert.Equal(t, "test-workflow", exec.WorkflowName)
	assert.Equal(t, workflow.StatusRunning, exec.Status)
	assert.Equal(t, "current", exec.CurrentStep)
	assert.Equal(t, now.Round(time.Second), exec.UpdatedAt.Round(time.Second))
}

// These tests verify that the dispatcher correctly routes StepTypeCallWorkflow
// steps to the executeCallWorkflowStep method in both Run() and Resume() paths.

func TestExecutionService_Run_CallWorkflow_DispatcherRouting(t *testing.T) {
	// Test that the dispatcher correctly routes call_workflow steps
	// Simple child workflow
	childWf := &workflow.Workflow{
		Name:    "child",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo child",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent workflow with call_workflow step
	parentWf := &workflow.Workflow{
		Name:    "parent",
		Initial: "call_child",
		Steps: map[string]*workflow.Step{
			"call_child": {
				Name: "call_child",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "child",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("child", childWf).
		WithWorkflow("parent", parentWf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify call_child step was executed (recorded in state)
	state, ok := ctx.GetStepState("call_child")
	require.True(t, ok, "call_child step should be recorded")
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

func TestExecutionService_Run_CallWorkflow_InSequence(t *testing.T) {
	// Test call_workflow step in a sequence with other step types
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

	// Parent: command -> call_workflow -> command -> done
	sequenceWf := &workflow.Workflow{
		Name:    "sequence",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step1",
				OnSuccess: "call_helper",
			},
			"call_helper": {
				Name: "call_helper",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "helper",
				},
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step2",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("helper", helperWf).
		WithWorkflow("sequence", sequenceWf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "sequence", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// All steps should be executed
	_, hasStep1 := ctx.GetStepState("step1")
	_, hasCallHelper := ctx.GetStepState("call_helper")
	_, hasStep2 := ctx.GetStepState("step2")

	assert.True(t, hasStep1, "step1 should be recorded")
	assert.True(t, hasCallHelper, "call_helper should be recorded")
	assert.True(t, hasStep2, "step2 should be recorded")
}

func TestExecutionService_Run_CallWorkflow_FailureTransition(t *testing.T) {
	// Test that call_workflow step follows on_failure transition when child fails
	// Failing child workflow
	failingChildWf := &workflow.Workflow{
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

	// Parent with on_failure transition
	parentWf := &workflow.Workflow{
		Name:    "parent-with-handler",
		Initial: "call_child",
		Steps: map[string]*workflow.Step{
			"call_child": {
				Name: "call_child",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "failing-child",
				},
				OnSuccess: "success",
				OnFailure: "error_handler",
			},
			"success":       {Name: "success", Type: workflow.StepTypeTerminal},
			"error_handler": {Name: "error_handler", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("failing-child", failingChildWf).
		WithWorkflow("parent-with-handler", parentWf).
		WithCommandResult("exit 1", &ports.CommandResult{ExitCode: 1, Stderr: "command failed"}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "parent-with-handler", nil)

	require.NoError(t, err) // Workflow completes via error handler
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "error_handler", ctx.CurrentStep, "should follow on_failure transition")
}

func TestExecutionService_Resume_CallWorkflow_DispatcherRouting(t *testing.T) {
	// Test that Resume correctly routes call_workflow steps
	// This tests the executeFromStep dispatcher
	// Child workflow
	childWf := &workflow.Workflow{
		Name:    "child",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo child",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent with call_workflow
	resumeParentWf := &workflow.Workflow{
		Name:    "resume-parent",
		Initial: "prep",
		Steps: map[string]*workflow.Step{
			"prep": {
				Name:      "prep",
				Type:      workflow.StepTypeCommand,
				Command:   "echo prep",
				OnSuccess: "call_child",
			},
			"call_child": {
				Name: "call_child",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "child",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Create persisted state at call_child step
	resumeState := &workflow.ExecutionContext{
		WorkflowID:   "resume-test-id",
		WorkflowName: "resume-parent",
		Status:       workflow.StatusRunning,
		CurrentStep:  "call_child", // Resuming at call_workflow step
		Inputs:       make(map[string]any),
		States: map[string]workflow.StepState{
			"prep": {Name: "prep", Status: workflow.StatusCompleted},
		},
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("child", childWf).
		WithWorkflow("resume-parent", resumeParentWf).
		Build()
	mocks.StateStore.Save(context.Background(), resumeState)

	ctx, err := execSvc.Resume(context.Background(), "resume-test-id", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify call_child was executed via resume
	state, ok := ctx.GetStepState("call_child")
	require.True(t, ok, "call_child step should be recorded after resume")
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

func TestExecutionService_Run_CallWorkflow_MixedStepTypes(t *testing.T) {
	// Test dispatcher correctly handles workflow with mixed step types
	// Testing: command -> call_workflow -> command sequence
	// Child workflow
	subflowWf := &workflow.Workflow{
		Name:    "subflow",
		Initial: "sub_work",
		Steps: map[string]*workflow.Step{
			"sub_work": {
				Name:      "sub_work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo sub",
				OnSuccess: "sub_done",
			},
			"sub_done": {Name: "sub_done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent with command and call_workflow steps interleaved
	mixedTypesWf := &workflow.Workflow{
		Name:    "mixed-types",
		Initial: "first_step",
		Steps: map[string]*workflow.Step{
			"first_step": {
				Name:      "first_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo first",
				OnSuccess: "call_subflow",
			},
			"call_subflow": {
				Name: "call_subflow",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "subflow",
				},
				OnSuccess: "final_step",
			},
			"final_step": {
				Name:      "final_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo final",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("subflow", subflowWf).
		WithWorkflow("mixed-types", mixedTypesWf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "mixed-types", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify all step types were dispatched correctly
	_, hasFirst := ctx.GetStepState("first_step")
	_, hasCallSubflow := ctx.GetStepState("call_subflow")
	_, hasFinal := ctx.GetStepState("final_step")

	assert.True(t, hasFirst, "first_step should be recorded")
	assert.True(t, hasCallSubflow, "call_subflow should be recorded")
	assert.True(t, hasFinal, "final_step should be recorded")
}

func TestExecutionService_Run_CallWorkflow_DefaultStep(t *testing.T) {
	// Ensure call_workflow doesn't fall through to default case
	// This test verifies call_workflow is handled before the default case
	// Child workflow
	simpleChildWf := &workflow.Workflow{
		Name:    "simple-child",
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

	// Parent with only call_workflow step
	onlyCallWf := &workflow.Workflow{
		Name:    "only-call",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "simple-child",
				},
				OnSuccess: "done",
				// Note: No Command field - if dispatcher used default case,
				// it would try to execute empty command and fail
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("simple-child", simpleChildWf).
		WithWorkflow("only-call", onlyCallWf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "only-call", nil)

	// Should succeed without trying to execute empty command
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// Feature: C027 - Application Layer Test Coverage Improvement
//
// This section contains tests for ExecutionService setters that previously
// had zero coverage:
// - SetOperationProvider
// - SetAgentRegistry
// - SetEvaluator
// - SetConversationManager
//
// Test pattern follows existing setter tests from plugin_operation_test.go
// and agent_step_test.go. Each setter has two test scenarios: valid value and nil value.

// TestExecutionService_SetOperationProvider_Valid verifies that a valid
// ports.OperationProvider can be set without panicking.
func TestExecutionService_SetOperationProvider_Valid(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	// Create a simple mock provider (minimal implementation)
	provider := &simpleOperationProvider{}

	// SetOperationProvider should not panic
	assert.NotPanics(t, func() {
		execSvc.SetOperationProvider(provider)
	})
}

// TestExecutionService_SetOperationProvider_Nil verifies that setting
// nil operation provider is handled gracefully (does not panic).
func TestExecutionService_SetOperationProvider_Nil(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	// SetOperationProvider with nil should not panic
	assert.NotPanics(t, func() {
		execSvc.SetOperationProvider(nil)
	})
}

// TestExecutionService_SetAgentRegistry_Valid verifies that a valid
// ports.AgentRegistry can be set without panicking.
func TestExecutionService_SetAgentRegistry_Valid(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	// Create mock registry with test provider
	registry := testmocks.NewMockAgentRegistry()
	claude := testmocks.NewMockAgentProvider("claude")
	_ = registry.Register(claude)

	// SetAgentRegistry should not panic
	assert.NotPanics(t, func() {
		execSvc.SetAgentRegistry(registry)
	})
}

// TestExecutionService_SetAgentRegistry_Nil verifies that setting
// nil agent registry is handled gracefully (does not panic).
func TestExecutionService_SetAgentRegistry_Nil(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	// SetAgentRegistry with nil should not panic
	assert.NotPanics(t, func() {
		execSvc.SetAgentRegistry(nil)
	})
}

// TestExecutionService_SetEvaluator_Valid verifies that a valid
// ExpressionEvaluator can be set without panicking.
func TestExecutionService_SetEvaluator_Valid(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	// Create a simple mock evaluator
	evaluator := &simpleExpressionEvaluator{}

	// SetEvaluator should not panic
	assert.NotPanics(t, func() {
		execSvc.SetEvaluator(evaluator)
	})
}

// TestExecutionService_SetEvaluator_Nil verifies that setting
// nil expression evaluator is handled gracefully (does not panic).
func TestExecutionService_SetEvaluator_Nil(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	// SetEvaluator with nil should not panic
	assert.NotPanics(t, func() {
		execSvc.SetEvaluator(nil)
	})
}

// TestExecutionService_SetConversationManager_Valid verifies that a valid
// ConversationManager can be set without panicking.
func TestExecutionService_SetConversationManager_Valid(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	// Create a minimal ConversationManager instance
	mgr := &application.ConversationManager{}

	// SetConversationManager should not panic
	assert.NotPanics(t, func() {
		execSvc.SetConversationManager(mgr)
	})
}

// TestExecutionService_SetConversationManager_Nil verifies that setting
// nil conversation manager is handled gracefully (does not panic).
func TestExecutionService_SetConversationManager_Nil(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	// SetConversationManager with nil should not panic
	assert.NotPanics(t, func() {
		execSvc.SetConversationManager(nil)
	})
}

// simpleOperationProvider implements ports.OperationProvider for testing SetOperationProvider.
type simpleOperationProvider struct{}

func (s *simpleOperationProvider) GetOperation(name string) (*plugin.OperationSchema, bool) {
	return nil, false
}

func (s *simpleOperationProvider) ListOperations() []*plugin.OperationSchema {
	return nil
}

func (s *simpleOperationProvider) Execute(ctx context.Context, name string, inputs map[string]any) (*plugin.OperationResult, error) {
	return &plugin.OperationResult{Success: true}, nil
}

// simpleExpressionEvaluator implements ports.ExpressionEvaluator for testing SetEvaluator.
// C042: Updated to implement EvaluateBool and EvaluateInt methods.
type simpleExpressionEvaluator struct{}

func (s *simpleExpressionEvaluator) EvaluateBool(expr string, ctx *interpolation.Context) (bool, error) {
	return false, nil
}

func (s *simpleExpressionEvaluator) EvaluateInt(expr string, ctx *interpolation.Context) (int, error) {
	return 0, nil
}
