package application_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	infraexpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/testutil/builders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolveNextStep Tests
// Feature: C054 - Increase Application Layer Test Coverage
// Component: T003 - resolve_next_step_tests
//
// This file tests the resolveNextStep function which determines the next step
// using conditional transitions or legacy OnSuccess/OnFailure.
//
// Coverage target: 23.1% -> 80%+
//
// This function is private (resolveNextStep), so we test it indirectly through
// the public API by constructing workflows with steps that have transitions
// and observing execution flow.

func TestResolveNextStep_LegacyOnSuccess(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
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

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "success_step", ctx.CurrentStep, "should follow OnSuccess when no transitions and success=true")
}

func TestResolveNextStep_LegacyOnFailure(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
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

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "failure_step", ctx.CurrentStep, "should follow OnFailure when no transitions and success=false")
}

func TestResolveNextStep_TransitionMatches(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
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

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "matched_step", ctx.CurrentStep, "should follow matching transition over OnSuccess")
}

func TestResolveNextStep_TransitionNoMatch_FallbackToLegacy(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
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

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "fallback_success", ctx.CurrentStep, "should fallback to OnSuccess when no transition matches")
}

func TestResolveNextStep_MultipleTransitions_FirstMatchWins(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
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

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "first_match", ctx.CurrentStep, "should select first matching transition when multiple match")
}

func TestResolveNextStep_DefaultTransition_AlwaysMatches(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
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

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "default_target", ctx.CurrentStep, "should match default transition (empty When)")
}

func TestResolveNextStep_TransitionWithBooleanExpression(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
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

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "math_matched", ctx.CurrentStep, "should evaluate arithmetic expression in transition")
}

func TestResolveNextStep_NoEvaluator_FallbackToLegacy(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
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

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "legacy_success", ctx.CurrentStep, "should fallback to OnSuccess when evaluator is nil")
}

func TestResolveNextStep_EmptyOnSuccessReturnsEmptyString(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
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

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep, "should be on terminal step")
}

func TestResolveNextStep_TransitionEvaluationError(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
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

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.Error(t, err, "should return error when transition evaluation fails")
	assert.Contains(t, err.Error(), "evaluate transitions", "error should indicate transition evaluation failure")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

// F068: Exit Code Based Transition Routing
// Tests for handleNonZeroExit calling resolveNextStep before legacy fallback.

func TestResolveNextStep_NonZeroExit_TransitionMatches(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 1").
				WithTransitions(workflow.Transitions{
					{When: "true", Goto: "transition_target"},
				}).
				WithOnFailure("legacy_failure").
				Build(),
			"transition_target": {
				Name: "transition_target",
				Type: workflow.StepTypeTerminal,
			},
			"legacy_failure": {
				Name: "legacy_failure",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 1", &ports.CommandResult{Stderr: "", ExitCode: 1}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "transition_target", ctx.CurrentStep, "transition should take priority over OnFailure on non-zero exit")
}

func TestResolveNextStep_NonZeroExit_NoMatchFallsBackToOnFailure(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": builders.NewStepBuilder("start").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 1").
				WithTransitions(workflow.Transitions{
					{When: "false", Goto: "never_reached"},
				}).
				WithOnFailure("legacy_failure").
				Build(),
			"never_reached": {
				Name: "never_reached",
				Type: workflow.StepTypeTerminal,
			},
			"legacy_failure": {
				Name: "legacy_failure",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 1", &ports.CommandResult{Stderr: "", ExitCode: 1}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "legacy_failure", ctx.CurrentStep, "should fallback to OnFailure when no transition matches on non-zero exit")
}

func TestResolveNextStep_ExitCode42_RoutingViaTransition(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "build",
		Steps: map[string]*workflow.Step{
			"build": builders.NewStepBuilder("build").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 42").
				WithTransitions(workflow.Transitions{
					{When: "states.build.ExitCode == 42", Goto: "specific_handler"},
					{When: "states.build.ExitCode == 0", Goto: "deploy"},
				}).
				WithOnFailure("generic_failure").
				Build(),
			"specific_handler": {
				Name: "specific_handler",
				Type: workflow.StepTypeTerminal,
			},
			"deploy": {
				Name: "deploy",
				Type: workflow.StepTypeTerminal,
			},
			"generic_failure": {
				Name: "generic_failure",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 42", &ports.CommandResult{Stderr: "", ExitCode: 42}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "specific_handler", ctx.CurrentStep, "should route to specific_handler when ExitCode == 42")
}

// F068 T001: handleNonZeroExit comprehensive test coverage
// Tests for exit code-based routing with transitions before legacy fallback

func TestHandleNonZeroExit_NumericComparison_GreaterThan(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 3").
				WithTransitions(workflow.Transitions{
					{When: "states.step1.ExitCode > 1", Goto: "high_error"},
				}).
				WithOnFailure("default_fail").
				Build(),
			"high_error": {
				Name: "high_error",
				Type: workflow.StepTypeTerminal,
			},
			"default_fail": {
				Name: "default_fail",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 3", &ports.CommandResult{Stderr: "", ExitCode: 3}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "high_error", ctx.CurrentStep, "exit code 3 > 1 should route to high_error")
}

func TestHandleNonZeroExit_NumericComparison_LessThan(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 2").
				WithTransitions(workflow.Transitions{
					{When: "states.step1.ExitCode < 5", Goto: "low_error"},
				}).
				WithOnFailure("default_fail").
				Build(),
			"low_error": {
				Name: "low_error",
				Type: workflow.StepTypeTerminal,
			},
			"default_fail": {
				Name: "default_fail",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 2", &ports.CommandResult{Stderr: "", ExitCode: 2}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "low_error", ctx.CurrentStep, "exit code 2 < 5 should route to low_error")
}

func TestHandleNonZeroExit_NumericComparison_NotEqual(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 1").
				WithTransitions(workflow.Transitions{
					{When: "states.step1.ExitCode != 0", Goto: "any_error"},
				}).
				WithOnSuccess("success").
				Build(),
			"any_error": {
				Name: "any_error",
				Type: workflow.StepTypeTerminal,
			},
			"success": {
				Name: "success",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 1", &ports.CommandResult{Stderr: "", ExitCode: 1}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "any_error", ctx.CurrentStep, "exit code 1 != 0 should route to any_error")
}

func TestHandleNonZeroExit_ContinueOnError_TransitionTakesPriority(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 1").
				WithContinueOnError(true).
				WithTransitions(workflow.Transitions{
					{When: "true", Goto: "transition_target"},
				}).
				WithOnSuccess("legacy_success").
				Build(),
			"transition_target": {
				Name: "transition_target",
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
		WithCommandResult("exit 1", &ports.CommandResult{Stderr: "", ExitCode: 1}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "transition_target", ctx.CurrentStep, "transition should take priority even with ContinueOnError=true")
}

func TestHandleNonZeroExit_ContinueOnError_WithoutTransition_FollowsOnSuccess(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 1").
				WithContinueOnError(true).
				WithTransitions(workflow.Transitions{
					{When: "false", Goto: "never_reached"},
				}).
				WithOnSuccess("continued").
				Build(),
			"never_reached": {
				Name: "never_reached",
				Type: workflow.StepTypeTerminal,
			},
			"continued": {
				Name: "continued",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 1", &ports.CommandResult{Stderr: "", ExitCode: 1}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "continued", ctx.CurrentStep, "with ContinueOnError=true and no matching transition, should follow OnSuccess")
}

func TestHandleNonZeroExit_ExitCodeZeroButTransitionOnNonZero(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo success").
				WithTransitions(workflow.Transitions{
					{When: "states.step1.ExitCode != 0", Goto: "error_handler"},
				}).
				WithOnSuccess("normal_success").
				Build(),
			"error_handler": {
				Name: "error_handler",
				Type: workflow.StepTypeTerminal,
			},
			"normal_success": {
				Name: "normal_success",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("echo success", &ports.CommandResult{Stdout: "success\n", ExitCode: 0}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "normal_success", ctx.CurrentStep, "exit code 0 should not match ExitCode != 0 transition, should follow OnSuccess")
}

func TestHandleNonZeroExit_MixedExitCodeAndOutputTransitions(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 1").
				WithTransitions(workflow.Transitions{
					{When: "states.step1.Output contains 'ERROR'", Goto: "parse_error"},
					{When: "states.step1.ExitCode == 1", Goto: "exit_error"},
				}).
				WithOnFailure("default_failure").
				Build(),
			"parse_error": {
				Name: "parse_error",
				Type: workflow.StepTypeTerminal,
			},
			"exit_error": {
				Name: "exit_error",
				Type: workflow.StepTypeTerminal,
			},
			"default_failure": {
				Name: "default_failure",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 1", &ports.CommandResult{Stdout: "", Stderr: "", ExitCode: 1}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "exit_error", ctx.CurrentStep, "first matching transition (ExitCode == 1) should win over later Output condition")
}

func TestHandleNonZeroExit_DefaultTransitionOnNonZeroExit(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 99").
				WithTransitions(workflow.Transitions{
					{When: "states.step1.ExitCode == 42", Goto: "specific"},
					{When: "", Goto: "default_handler"}, // Default transition
				}).
				WithOnFailure("failure").
				Build(),
			"specific": {
				Name: "specific",
				Type: workflow.StepTypeTerminal,
			},
			"default_handler": {
				Name: "default_handler",
				Type: workflow.StepTypeTerminal,
			},
			"failure": {
				Name: "failure",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 99", &ports.CommandResult{Stderr: "", ExitCode: 99}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "default_handler", ctx.CurrentStep, "default transition (empty When) should catch unmatched exit codes")
}

func TestHandleNonZeroExit_ExitCodeInInterpolation(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 5").
				WithTransitions(workflow.Transitions{
					{When: "states.step1.ExitCode == 5", Goto: "log_step"},
				}).
				WithOnFailure("failure").
				Build(),
			"log_step": builders.NewStepBuilder("log_step").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo Exit code was: {{states.step1.ExitCode}}").
				WithOnSuccess("done").
				Build(),
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"failure": {
				Name: "failure",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 5", &ports.CommandResult{Stderr: "", ExitCode: 5}).
		WithCommandResult("echo Exit code was: 5", &ports.CommandResult{Stdout: "Exit code was: 5\n", ExitCode: 0}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "done", ctx.CurrentStep, "should complete workflow after log_step via ExitCode == 5 transition")
	// Verify ExitCode was captured and interpolated correctly
	assert.Equal(t, 5, ctx.States["step1"].ExitCode, "step1 state should have ExitCode=5")
}

func TestHandleNonZeroExit_RangeOperators_GreaterOrEqual(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 5").
				WithTransitions(workflow.Transitions{
					{When: "states.step1.ExitCode >= 5", Goto: "critical"},
				}).
				WithOnFailure("failure").
				Build(),
			"critical": {
				Name: "critical",
				Type: workflow.StepTypeTerminal,
			},
			"failure": {
				Name: "failure",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 5", &ports.CommandResult{Stderr: "", ExitCode: 5}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "critical", ctx.CurrentStep, "exit code 5 >= 5 should route to critical")
}

func TestHandleNonZeroExit_RangeOperators_LessOrEqual(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 2").
				WithTransitions(workflow.Transitions{
					{When: "states.step1.ExitCode <= 2", Goto: "minor"},
				}).
				WithOnFailure("failure").
				Build(),
			"minor": {
				Name: "minor",
				Type: workflow.StepTypeTerminal,
			},
			"failure": {
				Name: "failure",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 2", &ports.CommandResult{Stderr: "", ExitCode: 2}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "minor", ctx.CurrentStep, "exit code 2 <= 2 should route to minor")
}

func TestHandleNonZeroExit_MultipleSteps_SecondStepExitCodeRouting(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo ok").
				WithOnSuccess("step2").
				Build(),
			"step2": builders.NewStepBuilder("step2").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 7").
				WithTransitions(workflow.Transitions{
					{When: "states.step2.ExitCode == 7", Goto: "caught_step2_error"},
				}).
				WithOnFailure("uncaught_failure").
				Build(),
			"caught_step2_error": {
				Name: "caught_step2_error",
				Type: workflow.StepTypeTerminal,
			},
			"uncaught_failure": {
				Name: "uncaught_failure",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("echo ok", &ports.CommandResult{Stdout: "ok\n", ExitCode: 0}).
		WithCommandResult("exit 7", &ports.CommandResult{Stderr: "", ExitCode: 7}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "caught_step2_error", ctx.CurrentStep, "second step should route by ExitCode == 7 transition")
}

func TestHandleNonZeroExit_WithoutTransitions_FallsBackToOnFailure(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("exit 1").
				WithOnFailure("failure_handler").
				Build(),
			"failure_handler": {
				Name: "failure_handler",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("exit 1", &ports.CommandResult{Stderr: "", ExitCode: 1}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "failure_handler", ctx.CurrentStep, "without transitions, should follow OnFailure for non-zero exit")
}

func TestHandleNonZeroExit_ExitCode0IsSuccess_NotCaughtByFailurePath(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": builders.NewStepBuilder("step1").
				WithType(workflow.StepTypeCommand).
				WithCommand("echo success").
				WithTransitions(workflow.Transitions{
					{When: "states.step1.ExitCode == 0", Goto: "success_via_transition"},
				}).
				WithOnSuccess("success_via_legacy").
				WithOnFailure("failure").
				Build(),
			"success_via_transition": {
				Name: "success_via_transition",
				Type: workflow.StepTypeTerminal,
			},
			"success_via_legacy": {
				Name: "success_via_legacy",
				Type: workflow.StepTypeTerminal,
			},
			"failure": {
				Name: "failure",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
		WithWorkflow("test", wf).
		WithCommandResult("echo success", &ports.CommandResult{Stdout: "success\n", ExitCode: 0}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "success_via_transition", ctx.CurrentStep, "exit code 0 should follow success path (handleSuccess, not handleNonZeroExit)")
}
