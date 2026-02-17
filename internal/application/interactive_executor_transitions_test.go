package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/infrastructure/expression"
	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F068 T002: handleNonZeroExit transition routing tests for InteractiveExecutor
//
// These tests verify that InteractiveExecutor.HandleNonZeroExit correctly evaluates
// transitions before falling back to legacy OnFailure/ContinueOnError behavior.
// This is the mirror/complement to ExecutionService tests (F068 T001).

func TestInteractiveExecutor_HandleNonZeroExit_TransitionMatches(t *testing.T) {
	// Happy path: transition condition matches on non-zero exit
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "build_step",
		Command: "exit 1",
		Transitions: workflow.Transitions{
			{When: "true", Goto: "error_handler"},
		},
		OnFailure: "legacy_failure",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    1,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 1,
		Stderr:   "build failed",
	}

	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	require.NoError(t, err, "should not return error when transition matches")
	assert.Equal(t, "error_handler", nextStep, "should follow matching transition over OnFailure")
}

func TestInteractiveExecutor_HandleNonZeroExit_ExitCodeEqualityComparison(t *testing.T) {
	// Edge case: transition uses ExitCode == specific_value
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "check_step",
		Command: "exit 42",
		Transitions: workflow.Transitions{
			{When: "states.check_step.ExitCode == 42", Goto: "specific_handler"},
		},
		OnFailure: "generic_failure",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    42,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 42,
		Stderr:   "custom exit code",
	}

	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "specific_handler", nextStep, "should match when ExitCode == 42")
}

func TestInteractiveExecutor_HandleNonZeroExit_ExitCodeGreaterThanComparison(t *testing.T) {
	// Edge case: transition uses ExitCode > threshold
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "severity_check",
		Command: "exit 5",
		Transitions: workflow.Transitions{
			{When: "states.severity_check.ExitCode > 3", Goto: "high_severity"},
		},
		OnFailure: "default_failure",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    5,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 5,
		Stderr:   "",
	}

	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "high_severity", nextStep, "should match when ExitCode > 3")
}

func TestInteractiveExecutor_HandleNonZeroExit_NoTransitionMatch_FallsBackToOnFailure(t *testing.T) {
	// Edge case: transition condition is false, should fall back to legacy OnFailure
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "deploy_step",
		Command: "exit 1",
		Transitions: workflow.Transitions{
			{When: "states.deploy_step.ExitCode == 0", Goto: "success_handler"},
		},
		OnFailure: "fallback_failure",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    1,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 1,
		Stderr:   "deployment failed",
	}

	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	require.NoError(t, err, "should fall back to OnFailure when no transition matches")
	assert.Equal(t, "fallback_failure", nextStep, "should use OnFailure when transition does not match")
}

func TestInteractiveExecutor_HandleNonZeroExit_MultipleTransitions_FirstMatchWins(t *testing.T) {
	// Edge case: multiple transitions exist, first matching should win
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "multi_exit",
		Command: "exit 2",
		Transitions: workflow.Transitions{
			{When: "states.multi_exit.ExitCode > 1", Goto: "first_match"},
			{When: "states.multi_exit.ExitCode > 0", Goto: "second_match"},
		},
		OnFailure: "legacy_failure",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    2,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 2,
		Stderr:   "",
	}

	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "first_match", nextStep, "should select first matching transition")
}

func TestInteractiveExecutor_HandleNonZeroExit_DefaultTransition_AlwaysMatches(t *testing.T) {
	// Edge case: empty When condition (default transition) should always match
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "with_default",
		Command: "exit 99",
		Transitions: workflow.Transitions{
			{When: "false", Goto: "never_matched"},
			{When: "", Goto: "default_handler"}, // Default transition
		},
		OnFailure: "legacy_failure",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    99,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 99,
		Stderr:   "",
	}

	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "default_handler", nextStep, "should match default transition (empty When)")
}

func TestInteractiveExecutor_HandleNonZeroExit_ContinueOnError_TransitionTakesPriority(t *testing.T) {
	// Edge case: when ContinueOnError=true AND transition matches,
	// transition should take priority over OnSuccess
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:            "tolerant_step",
		Command:         "exit 1",
		ContinueOnError: true,
		Transitions: workflow.Transitions{
			{When: "true", Goto: "transition_target"},
		},
		OnSuccess: "legacy_success",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    1,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 1,
		Stderr:   "",
	}

	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "transition_target", nextStep, "transition should take priority over ContinueOnError")
}

func TestInteractiveExecutor_HandleNonZeroExit_ContinueOnError_WithoutTransition_FollowsOnSuccess(t *testing.T) {
	// Edge case: ContinueOnError=true AND no transition matches,
	// should follow OnSuccess (not OnFailure)
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:            "continue_step",
		Command:         "exit 1",
		ContinueOnError: true,
		Transitions: workflow.Transitions{
			{When: "false", Goto: "never_reached"},
		},
		OnSuccess: "continued_path",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    1,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 1,
		Stderr:   "",
	}

	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "continued_path", nextStep, "should follow OnSuccess when ContinueOnError=true and no transition matches")
}

func TestInteractiveExecutor_HandleNonZeroExit_ExitCodeNotEqualComparison(t *testing.T) {
	// Edge case: transition uses ExitCode != value
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "not_equal_test",
		Command: "exit 5",
		Transitions: workflow.Transitions{
			{When: "states.not_equal_test.ExitCode != 0", Goto: "any_error"},
		},
		OnFailure: "legacy_failure",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    5,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 5,
		Stderr:   "",
	}

	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "any_error", nextStep, "should match when ExitCode != 0")
}

func TestInteractiveExecutor_HandleNonZeroExit_ExitCodeLessThanComparison(t *testing.T) {
	// Edge case: transition uses ExitCode < threshold
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "less_than_test",
		Command: "exit 2",
		Transitions: workflow.Transitions{
			{When: "states.less_than_test.ExitCode < 10", Goto: "minor_error"},
		},
		OnFailure: "major_error",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    2,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 2,
		Stderr:   "",
	}

	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "minor_error", nextStep, "should match when ExitCode < 10")
}

func TestInteractiveExecutor_HandleNonZeroExit_NoTransitions_LegacyOnFailure(t *testing.T) {
	// Happy path: no transitions defined, should use OnFailure (backward compatibility)
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:      "legacy_step",
		Command:   "exit 1",
		OnFailure: "legacy_handler",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    1,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 1,
		Stderr:   "",
	}

	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	require.NoError(t, err)
	assert.Equal(t, "legacy_handler", nextStep, "should use OnFailure when no transitions defined")
}

func TestInteractiveExecutor_HandleNonZeroExit_NilStep_ReturnsError(t *testing.T) {
	// Error handling: nil step should return error
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	state := &workflow.StepState{}
	result := &ports.CommandResult{ExitCode: 1}
	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), nil, state, result, intCtx)

	require.Error(t, err, "should return error when step is nil")
	assert.Empty(t, nextStep, "nextStep should be empty")
}

func TestInteractiveExecutor_HandleNonZeroExit_NilResult_ReturnsError(t *testing.T) {
	// Error handling: nil result should return error
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name: "test_step",
	}
	state := &workflow.StepState{}
	intCtx := interpolation.NewContext()

	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, state, nil, intCtx)

	require.Error(t, err, "should return error when result is nil")
	assert.Empty(t, nextStep, "nextStep should be empty")
}
