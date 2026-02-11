package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	infraexpr "github.com/vanoix/awf/internal/infrastructure/expression"
	"github.com/vanoix/awf/internal/testutil"
)

// =============================================================================
// resolveNextStep Tests
// Feature: C054 - Increase Application Layer Test Coverage
// Component: T003 - resolve_next_step_tests
// =============================================================================
//
// This file tests the resolveNextStep function which determines the next step
// using conditional transitions or legacy OnSuccess/OnFailure.
//
// Coverage target: 23.1% -> 80%+
//
// This function is private (resolveNextStep), so we test it indirectly through
// the public API by constructing workflows with steps that have transitions
// and observing execution flow.
// =============================================================================

func TestResolveNextStep_LegacyOnSuccess(t *testing.T) {
	// Arrange: Workflow with step using legacy OnSuccess (no transitions)
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": testutil.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo hello").
				WithOnSuccess("success_step").
				WithOnFailure("failure_step").
				Build(),
			"success_step": {
				Name: "success_step",
				Type: workflow.StepTypeTerminal,
			},
			"failure_step": {
				Name: "failure_step",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	// Act: Execute workflow with successful command
	ctx, err := execSvc.Run(context.Background(), "test", nil)

	// Assert: Should follow OnSuccess path
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "success_step", ctx.CurrentStep, "should follow OnSuccess when no transitions and success=true")
}

func TestResolveNextStep_LegacyOnFailure(t *testing.T) {
	// Arrange: Workflow with step using legacy OnFailure (no transitions)
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": testutil.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 1").
				WithOnSuccess("success_step").
				WithOnFailure("failure_step").
				Build(),
			"success_step": {
				Name: "success_step",
				Type: workflow.StepTypeTerminal,
			},
			"failure_step": {
				Name: "failure_step",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 1", &ports.CommandResult{Stderr: "", ExitCode: 1}).
		Build()

	// Act: Execute workflow with failing command
	ctx, err := execSvc.Run(context.Background(), "test", nil)

	// Assert: Should follow OnFailure path
	require.NoError(t, err)
	assert.Equal(t, "failure_step", ctx.CurrentStep, "should follow OnFailure when no transitions and success=false")
}

func TestResolveNextStep_TransitionMatches(t *testing.T) {
	// Arrange: Workflow with step that has matching transition
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": testutil.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo hello").
				WithTransitions(workflow.Transitions{
					{When: "true", Goto: "matched_step"},
				}).
				WithOnSuccess("default_success").
				Build(),
			"matched_step": {
				Name: "matched_step",
				Type: workflow.StepTypeTerminal,
			},
			"default_success": {
				Name: "default_success",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	// Act: Execute workflow
	ctx, err := execSvc.Run(context.Background(), "test", nil)

	// Assert: Should follow transition instead of OnSuccess
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "matched_step", ctx.CurrentStep, "should follow matching transition over OnSuccess")
}

func TestResolveNextStep_TransitionNoMatch_FallbackToLegacy(t *testing.T) {
	// Arrange: Workflow with transition that doesn't match
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": testutil.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo hello").
				WithTransitions(workflow.Transitions{
					{When: "false", Goto: "never_matched"},
				}).
				WithOnSuccess("fallback_success").
				Build(),
			"never_matched": {
				Name: "never_matched",
				Type: workflow.StepTypeTerminal,
			},
			"fallback_success": {
				Name: "fallback_success",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	// Act: Execute workflow
	ctx, err := execSvc.Run(context.Background(), "test", nil)

	// Assert: Should fall back to OnSuccess when no transition matches
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "fallback_success", ctx.CurrentStep, "should fallback to OnSuccess when no transition matches")
}

func TestResolveNextStep_MultipleTransitions_FirstMatchWins(t *testing.T) {
	// Arrange: Workflow with multiple transitions - first match should win
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": testutil.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo hello").
				WithTransitions(workflow.Transitions{
					{When: "true", Goto: "first_match"},
					{When: "true", Goto: "second_match"},
				}).
				WithOnSuccess("default_success").
				Build(),
			"first_match": {
				Name: "first_match",
				Type: workflow.StepTypeTerminal,
			},
			"second_match": {
				Name: "second_match",
				Type: workflow.StepTypeTerminal,
			},
			"default_success": {
				Name: "default_success",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	// Act: Execute workflow
	ctx, err := execSvc.Run(context.Background(), "test", nil)

	// Assert: First matching transition should be selected
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "first_match", ctx.CurrentStep, "should select first matching transition when multiple match")
}

func TestResolveNextStep_DefaultTransition_AlwaysMatches(t *testing.T) {
	// Arrange: Workflow with default transition (empty When)
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": testutil.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo hello").
				WithTransitions(workflow.Transitions{
					{When: "false", Goto: "never_reached"},
					{When: "", Goto: "default_target"}, // Default transition
				}).
				WithOnSuccess("legacy_success").
				Build(),
			"never_reached": {
				Name: "never_reached",
				Type: workflow.StepTypeTerminal,
			},
			"default_target": {
				Name: "default_target",
				Type: workflow.StepTypeTerminal,
			},
			"legacy_success": {
				Name: "legacy_success",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	// Act: Execute workflow
	ctx, err := execSvc.Run(context.Background(), "test", nil)

	// Assert: Default transition should match when no conditional matches
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "default_target", ctx.CurrentStep, "should match default transition (empty When)")
}

func TestResolveNextStep_TransitionWithBooleanExpression(t *testing.T) {
	// Arrange: Workflow with transition using arithmetic comparison expression
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": testutil.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo hello").
				WithTransitions(workflow.Transitions{
					{When: "1 + 1 == 2", Goto: "math_matched"},
				}).
				WithOnSuccess("default_next").
				Build(),
			"math_matched": {
				Name: "math_matched",
				Type: workflow.StepTypeTerminal,
			},
			"default_next": {
				Name: "default_next",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	// Act: Execute workflow
	ctx, err := execSvc.Run(context.Background(), "test", nil)

	// Assert: Transition with arithmetic expression should evaluate correctly
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "math_matched", ctx.CurrentStep, "should evaluate arithmetic expression in transition")
}

func TestResolveNextStep_NoEvaluator_FallbackToLegacy(t *testing.T) {
	// Arrange: Workflow with transitions but no evaluator configured (evaluator=nil)
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": testutil.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo hello").
				WithTransitions(workflow.Transitions{
					{When: "true", Goto: "never_evaluated"},
				}).
				WithOnSuccess("legacy_success").
				Build(),
			"never_evaluated": {
				Name: "never_evaluated",
				Type: workflow.StepTypeTerminal,
			},
			"legacy_success": {
				Name: "legacy_success",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Build service WITHOUT evaluator (nil)
	execSvc, _ := NewTestHarness(t). // Note: not using NewTestHarnessWithEvaluator
						WithWorkflow("test", wf).
						WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
						Build()

	// Act: Execute workflow
	ctx, err := execSvc.Run(context.Background(), "test", nil)

	// Assert: Should fall back to legacy OnSuccess when evaluator is nil
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "legacy_success", ctx.CurrentStep, "should fallback to OnSuccess when evaluator is nil")
}

func TestResolveNextStep_EmptyOnSuccessReturnsEmptyString(t *testing.T) {
	// Arrange: Step with empty OnSuccess - should return empty string (workflow end)
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": testutil.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo hello").
				WithOnSuccess("done"). // Need to go somewhere to complete workflow
				Build(),
			"done": {
				Name:      "done",
				Type:      workflow.StepTypeTerminal,
				OnSuccess: "", // Terminal step with empty OnSuccess
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	// Act: Execute workflow
	ctx, err := execSvc.Run(context.Background(), "test", nil)

	// Assert: Terminal step with empty OnSuccess should complete successfully
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep, "should be on terminal step")
}

func TestResolveNextStep_TransitionEvaluationError(t *testing.T) {
	// Arrange: Workflow with invalid transition expression
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": testutil.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo hello").
				WithTransitions(workflow.Transitions{
					{When: "invalid syntax here @@#$%", Goto: "next_step"},
				}).
				WithOnSuccess("fallback").
				Build(),
			"next_step": {
				Name: "next_step",
				Type: workflow.StepTypeTerminal,
			},
			"fallback": {
				Name: "fallback",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	// Act: Execute workflow
	ctx, err := execSvc.Run(context.Background(), "test", nil)

	// Assert: Should fail with evaluation error
	require.Error(t, err, "should return error when transition evaluation fails")
	assert.Contains(t, err.Error(), "evaluate transitions", "error should indicate transition evaluation failure")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}
