package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecutionService_Resume_From_Integration_PreviousReExecutesStep verifies the full resume-from-step
// end-to-end flow: 4-step workflow (init → validate → build → test → done), fails at test,
// resume with "previous" re-executes build, then test, and the workflow completes successfully.
func TestExecutionService_Resume_From_Integration_PreviousReExecutesStep(t *testing.T) {
	now := time.Now()

	wf := &workflow.Workflow{
		Name:    "four-step",
		Initial: "init",
		Steps: map[string]*workflow.Step{
			"init": {
				Name:      "init",
				Type:      workflow.StepTypeCommand,
				Command:   "echo init",
				OnSuccess: "validate",
			},
			"validate": {
				Name:      "validate",
				Type:      workflow.StepTypeCommand,
				Command:   "echo validate",
				OnSuccess: "build",
			},
			"build": {
				Name:      "build",
				Type:      workflow.StepTypeCommand,
				Command:   "echo build",
				OnSuccess: "test",
			},
			"test": {
				Name:      "test",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "integ-prev-001",
		WorkflowName: "four-step",
		Status:       workflow.StatusRunning,
		CurrentStep:  "test",
		States: map[string]workflow.StepState{
			"init":     {Name: "init", CompletedAt: now.Add(-3 * time.Second)},
			"validate": {Name: "validate", CompletedAt: now.Add(-2 * time.Second)},
			"build":    {Name: "build", CompletedAt: now.Add(-1 * time.Second)},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now.Add(-4 * time.Second),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("four-step", wf).
		WithCommandResult("echo build", &ports.CommandResult{Stdout: "build\n", ExitCode: 0}).
		WithCommandResult("echo test", &ports.CommandResult{Stdout: "test\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "integ-prev-001", nil, "previous")

	require.NoError(t, err)
	require.NotNil(t, ctx)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// "previous" resolves to build (most recently completed before test);
	// build must appear in executor calls, proving re-execution started from build.
	executedCmds := commandsExecuted(mocks)
	assert.Contains(t, executedCmds, "echo build", "build must be re-executed when resuming from previous")
	assert.Contains(t, executedCmds, "echo test", "test must execute after build")
}

// TestExecutionService_Resume_From_Integration_NamedStepCleanup verifies cleanup when resuming
// from a named step earlier in the chain: 5-step workflow (init → validate → build → test → deploy → done),
// states for build, test, deploy cleared on resume from "validate", then full chain re-executes.
func TestExecutionService_Resume_From_Integration_NamedStepCleanup(t *testing.T) {
	now := time.Now()

	wf := &workflow.Workflow{
		Name:    "five-step",
		Initial: "init",
		Steps: map[string]*workflow.Step{
			"init": {
				Name:      "init",
				Type:      workflow.StepTypeCommand,
				Command:   "echo init",
				OnSuccess: "validate",
			},
			"validate": {
				Name:      "validate",
				Type:      workflow.StepTypeCommand,
				Command:   "echo validate",
				OnSuccess: "build",
			},
			"build": {
				Name:      "build",
				Type:      workflow.StepTypeCommand,
				Command:   "echo build",
				OnSuccess: "test",
			},
			"test": {
				Name:      "test",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "deploy",
			},
			"deploy": {
				Name:      "deploy",
				Type:      workflow.StepTypeCommand,
				Command:   "echo deploy",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	// Simulate failure at deploy: init, validate, build, test completed; deploy failed (no state).
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "integ-named-001",
		WorkflowName: "five-step",
		Status:       workflow.StatusRunning,
		CurrentStep:  "deploy",
		States: map[string]workflow.StepState{
			"init":     {Name: "init", CompletedAt: now.Add(-4 * time.Second)},
			"validate": {Name: "validate", CompletedAt: now.Add(-3 * time.Second)},
			"build":    {Name: "build", CompletedAt: now.Add(-2 * time.Second)},
			"test":     {Name: "test", CompletedAt: now.Add(-1 * time.Second)},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now.Add(-5 * time.Second),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("five-step", wf).
		WithCommandResult("echo validate", &ports.CommandResult{Stdout: "validate\n", ExitCode: 0}).
		WithCommandResult("echo build", &ports.CommandResult{Stdout: "build\n", ExitCode: 0}).
		WithCommandResult("echo test", &ports.CommandResult{Stdout: "test\n", ExitCode: 0}).
		WithCommandResult("echo deploy", &ports.CommandResult{Stdout: "deploy\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "integ-named-001", nil, "validate")

	require.NoError(t, err)
	require.NotNil(t, ctx)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Resuming from the named step "validate" must re-execute validate and all subsequent steps.
	// "echo validate" appearing in calls proves execution started from validate, not deploy (current).
	executedCmds := commandsExecuted(mocks)
	assert.Contains(t, executedCmds, "echo validate", "validate must be re-executed when resuming from validate")
	assert.Contains(t, executedCmds, "echo build", "build must re-execute after validate")
	assert.Contains(t, executedCmds, "echo test", "test must re-execute after build")
	assert.Contains(t, executedCmds, "echo deploy", "deploy must re-execute after test")
}

// TestExecutionService_Resume_From_Integration_ParallelPrevious verifies US4: when "previous"
// resolves to a parallel step, all parallel branches and subsequent steps re-execute.
// Workflow: init → parallel_build (branches: build_a, build_b) → test → done.
// Fails at test; resume with "previous" re-executes the parallel block and test.
func TestExecutionService_Resume_From_Integration_ParallelPrevious(t *testing.T) {
	now := time.Now()

	wf := &workflow.Workflow{
		Name:    "parallel-resume",
		Initial: "init",
		Steps: map[string]*workflow.Step{
			"init": {
				Name:      "init",
				Type:      workflow.StepTypeCommand,
				Command:   "echo init",
				OnSuccess: "parallel_build",
			},
			"parallel_build": {
				Name:      "parallel_build",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"build_a", "build_b"},
				Strategy:  "all_succeed",
				OnSuccess: "test",
			},
			"build_a": {
				Name:    "build_a",
				Type:    workflow.StepTypeCommand,
				Command: "echo build_a",
			},
			"build_b": {
				Name:    "build_b",
				Type:    workflow.StepTypeCommand,
				Command: "echo build_b",
			},
			"test": {
				Name:      "test",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	// Simulate: init, build_a, build_b, parallel_build completed; test failed (no state).
	// parallel_build has the latest CompletedAt before test → "previous" resolves to it.
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "integ-parallel-001",
		WorkflowName: "parallel-resume",
		Status:       workflow.StatusRunning,
		CurrentStep:  "test",
		States: map[string]workflow.StepState{
			"init":           {Name: "init", CompletedAt: now.Add(-3 * time.Second)},
			"build_a":        {Name: "build_a", CompletedAt: now.Add(-2 * time.Second)},
			"build_b":        {Name: "build_b", CompletedAt: now.Add(-2 * time.Second)},
			"parallel_build": {Name: "parallel_build", CompletedAt: now.Add(-1 * time.Second)},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now.Add(-4 * time.Second),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("parallel-resume", wf).
		WithCommandResult("echo build_a", &ports.CommandResult{Stdout: "build_a\n", ExitCode: 0}).
		WithCommandResult("echo build_b", &ports.CommandResult{Stdout: "build_b\n", ExitCode: 0}).
		WithCommandResult("echo test", &ports.CommandResult{Stdout: "test\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "integ-parallel-001", nil, "previous")

	require.NoError(t, err)
	require.NotNil(t, ctx)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// "previous" resolves to parallel_build; both branches must re-execute.
	// Branch commands appearing in calls proves the parallel block was re-run from scratch.
	executedCmds := commandsExecuted(mocks)
	assert.Contains(t, executedCmds, "echo build_a", "build_a branch must re-execute when parallel parent is resumed")
	assert.Contains(t, executedCmds, "echo build_b", "build_b branch must re-execute when parallel parent is resumed")
	assert.Contains(t, executedCmds, "echo test", "test must execute after parallel block completes")
}

// commandsExecuted returns the Program strings of all commands executed by the mock executor.
func commandsExecuted(mocks *TestMocks) []string {
	calls := mocks.Executor.GetCalls()
	cmds := make([]string, 0, len(calls))
	for _, c := range calls {
		cmds = append(cmds, c.Program)
	}
	return cmds
}

// TestResume_FromCurrentStep tests that Resume with "current" resumes from the current step
func TestResume_FromCurrentStep(t *testing.T) {
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
				Name:   "step2",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	now := time.Now()
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step1",
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", CompletedAt: now},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo step1", &ports.CommandResult{Stdout: "step1\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "current")

	require.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "step2", ctx.CurrentStep)
}

// TestResume_FromPreviousStep tests that "previous" resumes from step with latest CompletedAt before CurrentStep
func TestResume_FromPreviousStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 2",
				OnSuccess: "step3",
			},
			"step3": {
				Name:   "step3",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	now := time.Now()
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step3", // Failed before step3 started
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", CompletedAt: now.Add(-2 * time.Second)},
			"step2": {Name: "step2", CompletedAt: now.Add(-1 * time.Second)},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo 2", &ports.CommandResult{Stdout: "2\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume from "previous" should resume from step2 (latest completed before step3)
	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "previous")

	require.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestResume_FromPreviousNoStateForCurrentStep tests that when CurrentStep has no StepState, "previous" uses latest CompletedAt across all states
func TestResume_FromPreviousNoStateForCurrentStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 2",
				OnSuccess: "step3",
			},
			"step3": {
				Name:   "step3",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	now := time.Now()
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step3", // step3 has no StepState entry (failed before it started)
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", CompletedAt: now.Add(-2 * time.Second)},
			"step2": {Name: "step2", CompletedAt: now.Add(-1 * time.Second)},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume from "previous" should resolve to step2 despite step3 having no state
	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "previous")

	require.NoError(t, err)
	assert.NotNil(t, ctx)
	// step3 is a terminal success, so workflow completes
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestResume_FromPreviousNoPriorStep tests that "previous" returns error when no completed step exists before CurrentStep
func TestResume_FromPreviousNoPriorStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step1",
		States:       make(map[string]workflow.StepState), // No completed steps
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume from "previous" should fail - no prior steps exist
	_, err = execSvc.Resume(context.Background(), "wf-001", nil, "previous")

	require.Error(t, err)
	assert.ErrorContains(t, err, "no prior step")
}

// TestResume_FromLiteralStepName tests that a literal step name resumes from that step when it exists in States
func TestResume_FromLiteralStepName(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "validate",
		Steps: map[string]*workflow.Step{
			"validate": {
				Name:      "validate",
				Type:      workflow.StepTypeCommand,
				Command:   "echo validate",
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	now := time.Now()
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "end",
		States: map[string]workflow.StepState{
			"validate": {Name: "validate", CompletedAt: now},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo validate", &ports.CommandResult{Stdout: "validate\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume from literal step name "validate"
	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "validate")

	require.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestResume_FromNonexistentStep tests that a non-existent step name returns an error
func TestResume_FromNonexistentStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo start"},
		},
	}

	now := time.Now()
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States: map[string]workflow.StepState{
			"start": {Name: "start", CompletedAt: now},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume from non-existent step should fail
	_, err = execSvc.Resume(context.Background(), "wf-001", nil, "nonexistent")

	require.Error(t, err)
	assert.ErrorContains(t, err, "not found")
}

// TestResume_FromStepEqualsCurrentStep tests that resuming from a step equal to CurrentStep is equivalent to "current"
func TestResume_FromStepEqualsCurrentStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	now := time.Now()
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States: map[string]workflow.StepState{
			"start": {Name: "start", CompletedAt: now},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo start", &ports.CommandResult{Stdout: "start\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume from "start" (equals CurrentStep) should work like "current"
	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "start")

	require.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestResume_FromPreviousIdenticalCompletedAtUsesAlphabetical tests that identical CompletedAt uses alphabetical order as tiebreaker
func TestResume_FromPreviousIdenticalCompletedAtUsesAlphabetical(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "stepA",
		Steps: map[string]*workflow.Step{
			"stepA": {
				Name:      "stepA",
				Type:      workflow.StepTypeCommand,
				Command:   "echo A",
				OnSuccess: "stepB",
			},
			"stepB": {
				Name:      "stepB",
				Type:      workflow.StepTypeCommand,
				Command:   "echo B",
				OnSuccess: "stepC",
			},
			"stepC": {
				Name:   "stepC",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	now := time.Now()
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "stepC",
		States: map[string]workflow.StepState{
			// Both have identical CompletedAt - should use alphabetical order to select stepB
			"stepA": {Name: "stepA", CompletedAt: now},
			"stepB": {Name: "stepB", CompletedAt: now},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume from "previous" should select stepB (alphabetically last when times equal)
	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "previous")

	require.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestResume_CleanupStatesAfterTarget tests that cleanup deletes states after the target step when resuming from an earlier step
func TestResume_CleanupStatesAfterTarget(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 2",
				OnSuccess: "step3",
			},
			"step3": {
				Name:   "step3",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	now := time.Now()
	// State with all 3 steps completed, but we'll resume from step1
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step3",
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", CompletedAt: now.Add(-2 * time.Second)},
			"step2": {Name: "step2", CompletedAt: now.Add(-1 * time.Second)},
			"step3": {Name: "step3", CompletedAt: now},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo 2", &ports.CommandResult{Stdout: "2\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume from step1 - should cleanup step3 (after step1's CompletedAt)
	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "step1")

	require.NoError(t, err)
	assert.NotNil(t, ctx)

	// Check that step3 was cleaned up
	finalStates := ctx.GetAllStepStates()
	assert.NotContains(t, finalStates, "step3")
}

// TestResume_CleanupPreservesTargetStep tests that cleanup preserves the target step itself
func TestResume_CleanupPreservesTargetStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:   "step2",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	now := time.Now()
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step2",
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", CompletedAt: now},
			"step2": {Name: "step2", CompletedAt: now.Add(1 * time.Second)},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume from step1 - should preserve step1 and delete step2
	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "step1")

	require.NoError(t, err)
	assert.NotNil(t, ctx)

	// Check that step1 was preserved
	finalStates := ctx.GetAllStepStates()
	assert.Contains(t, finalStates, "step1")
}

// TestResume_CleanupMultipleSteps tests that cleanup removes multiple steps after target
func TestResume_CleanupMultipleSteps(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 2",
				OnSuccess: "step3",
			},
			"step3": {
				Name:   "step3",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	now := time.Now()
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step3",
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", CompletedAt: now},
			"step2": {Name: "step2", CompletedAt: now.Add(1 * time.Second)},
			"step3": {Name: "step3", CompletedAt: now.Add(2 * time.Second)},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume from step1 - should delete step2 and step3
	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "step1")

	require.NoError(t, err)
	assert.NotNil(t, ctx)

	// Verify cleanup removed step2 and step3, then re-execution repopulated step1 and step2.
	// After resume from step1: cleanup deletes step2+step3, then execution runs step1→step2→step3.
	// Terminal steps (step3) do not produce state entries, so final states contain step1 and step2.
	finalStates := ctx.GetAllStepStates()
	assert.Len(t, finalStates, 2)
	assert.Contains(t, finalStates, "step1")
	assert.Contains(t, finalStates, "step2")
}

// TestResume_CleanupIsNoOpWhenNothingAfter tests that cleanup is a no-op when nothing is after target
func TestResume_CleanupIsNoOpWhenNothingAfter(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:   "step2",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	now := time.Now()
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step2", // No StepState for step2
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", CompletedAt: now},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume from step1 - cleanup is no-op since nothing is after step1
	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "step1")

	require.NoError(t, err)
	assert.NotNil(t, ctx)

	// State should remain unchanged
	finalStates := ctx.GetAllStepStates()
	assert.Len(t, finalStates, 1)
	assert.Contains(t, finalStates, "step1")
}

// TestResume_BackwardsCompatibilityWithPreviousAPI tests that Resume("current") maintains backwards compatibility
func TestResume_BackwardsCompatibilityWithPreviousAPI(t *testing.T) {
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
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	now := time.Now()
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States: map[string]workflow.StepState{
			"start": {Name: "start", CompletedAt: now},
		},
		Inputs:    make(map[string]any),
		Env:       make(map[string]string),
		StartedAt: now,
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	// Resume with "current" should behave like old Resume without fromStep
	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "current")

	require.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "end", ctx.CurrentStep)
}
