// execution_service_loop_test.go
// Extracted from: internal/application/execution_service_test.go (lines 3592-4247)
// Extracted on: 2026-01-14
// Component: T005 - Loop execution tests (ForEach/While loops)
// Purpose: Test loop iteration, nested loops, max iterations, break conditions
// Test count: 13 functions

package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

func TestExecuteLoopStep_ForEach_HappyPath(t *testing.T) {
	// Given: A workflow with a for_each loop step
	wf := &workflow.Workflow{
		Name:    "test-foreach",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a", "b", "c"]`,
					Body:          []string{"process_item"},
					MaxIterations: 100,
					OnComplete:    "done",
				},
			},
			"process_item": {
				Name:      "process_item",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.loop.Item}}",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-foreach", wf).
		WithCommandResult("echo a", &ports.CommandResult{Stdout: "a\n", ExitCode: 0}).
		WithCommandResult("echo b", &ports.CommandResult{Stdout: "b\n", ExitCode: 0}).
		WithCommandResult("echo c", &ports.CommandResult{Stdout: "c\n", ExitCode: 0}).
		Build()

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-foreach", nil)

	// Then: Should execute successfully
	require.NoError(t, err, "for_each loop should execute successfully")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

// TestExecuteLoopStep_While_HappyPath verifies that executeLoopStep correctly
// calls ExecuteWhile and the code compiles with the updated signature.
func TestExecuteLoopStep_While_HappyPath(t *testing.T) {
	// Given: A workflow with a while loop step
	repo := newMockRepository()
	repo.workflows["test-while"] = &workflow.Workflow{
		Name:    "test-while",
		Initial: "counter_loop",
		Steps: map[string]*workflow.Step{
			"counter_loop": {
				Name: "counter_loop",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:           workflow.LoopTypeWhile,
					Condition:      "true",
					Body:           []string{"increment"},
					MaxIterations:  3,
					BreakCondition: "false",
					OnComplete:     "done",
				},
			},
			"increment": {
				Name:      "increment",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.loop.Index}}",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo 0"] = &ports.CommandResult{Stdout: "0\n", ExitCode: 0}
	executor.results["echo 1"] = &ports.CommandResult{Stdout: "1\n", ExitCode: 0}
	executor.results["echo 2"] = &ports.CommandResult{Stdout: "2\n", ExitCode: 0}

	evaluator := newConditionMockEvaluator()
	evaluator.evaluations["true"] = true
	evaluator.evaluations["false"] = false

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionServiceWithEvaluator(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil, evaluator)

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-while", nil)

	// Then: Should execute successfully
	require.NoError(t, err, "while loop should execute successfully")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

func TestExecuteLoopStep_NestedForEach(t *testing.T) {
	// Given: A workflow with nested for_each loops
	repo := newMockRepository()
	repo.workflows["test-nested"] = &workflow.Workflow{
		Name:    "test-nested",
		Initial: "outer_loop",
		Steps: map[string]*workflow.Step{
			"outer_loop": {
				Name: "outer_loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["x", "y"]`,
					Body:          []string{"inner_loop"},
					MaxIterations: 10,
					OnComplete:    "done",
				},
			},
			"inner_loop": {
				Name: "inner_loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["1", "2"]`,
					Body:          []string{"process"},
					MaxIterations: 10,
					OnComplete:    "",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.loop.Item}}",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo 1"] = &ports.CommandResult{Stdout: "1\n", ExitCode: 0}
	executor.results["echo 2"] = &ports.CommandResult{Stdout: "2\n", ExitCode: 0}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-nested", nil)

	// Then: Should execute successfully
	require.NoError(t, err, "nested for_each loops should execute successfully")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

func TestExecuteLoopStep_WhileContainingForEach(t *testing.T) {
	// Given: A workflow with a while loop containing a for_each loop
	repo := newMockRepository()
	repo.workflows["test-while-foreach"] = &workflow.Workflow{
		Name:    "test-while-foreach",
		Initial: "outer_while",
		Steps: map[string]*workflow.Step{
			"outer_while": {
				Name: "outer_while",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:           workflow.LoopTypeWhile,
					Condition:      "true",
					Body:           []string{"inner_foreach"},
					MaxIterations:  10,
					BreakCondition: "loop.index == 1",
					OnComplete:     "done",
				},
			},
			"inner_foreach": {
				Name: "inner_foreach",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a", "b"]`,
					Body:          []string{"process"},
					MaxIterations: 10,
					OnComplete:    "",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.loop.Item}}",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo a"] = &ports.CommandResult{Stdout: "a\n", ExitCode: 0}
	executor.results["echo b"] = &ports.CommandResult{Stdout: "b\n", ExitCode: 0}

	evaluator := newConditionMockEvaluator()
	evaluator.evaluations["true"] = true
	evaluator.evaluations["false"] = false
	evaluator.evaluations["loop.index == 1"] = true

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionServiceWithEvaluator(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil, evaluator)

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-while-foreach", nil)

	// Then: Should execute successfully
	require.NoError(t, err, "while loop containing for_each should execute successfully")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

func TestExecuteLoopStep_ForEach_BodyStepError(t *testing.T) {
	// Given: A workflow with a for_each loop where the body step fails
	repo := newMockRepository()
	repo.workflows["test-foreach-error"] = &workflow.Workflow{
		Name:    "test-foreach-error",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a", "b"]`,
					Body:          []string{"fail_step"},
					MaxIterations: 10,
					OnComplete:    "done",
				},
			},
			"fail_step": {
				Name:      "fail_step",
				Type:      workflow.StepTypeCommand,
				Command:   "false",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["false"] = &ports.CommandResult{Stdout: "", ExitCode: 1}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-foreach-error", nil)

	// Then: Should fail
	require.Error(t, err, "for_each loop with failing body step should error")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
}

func TestExecuteLoopStep_While_BodyStepError(t *testing.T) {
	// Given: A workflow with a while loop where the body step fails
	repo := newMockRepository()
	repo.workflows["test-while-error"] = &workflow.Workflow{
		Name:    "test-while-error",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:           workflow.LoopTypeWhile,
					Condition:      "true",
					Body:           []string{"fail_step"},
					MaxIterations:  3,
					BreakCondition: "false",
					OnComplete:     "done",
				},
			},
			"fail_step": {
				Name:      "fail_step",
				Type:      workflow.StepTypeCommand,
				Command:   "false",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["false"] = &ports.CommandResult{Stdout: "", ExitCode: 1}

	evaluator := newConditionMockEvaluator()
	evaluator.evaluations["true"] = true
	evaluator.evaluations["false"] = false

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionServiceWithEvaluator(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil, evaluator)

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-while-error", nil)

	// Then: Should fail
	require.Error(t, err, "while loop with failing body step should error")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
}

func TestExecuteLoopStep_ForEach_BodyStepNotFound(t *testing.T) {
	// Given: A workflow with a for_each loop where body step doesn't exist
	repo := newMockRepository()
	repo.workflows["test-foreach-notfound"] = &workflow.Workflow{
		Name:    "test-foreach-notfound",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a"]`,
					Body:          []string{"nonexistent"},
					MaxIterations: 10,
					OnComplete:    "done",
				},
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-foreach-notfound", nil)

	// Then: Should fail with step not found error
	require.Error(t, err, "for_each loop with nonexistent body step should error")
	assert.Contains(t, err.Error(), "nonexistent", "error should mention missing step name")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
}

func TestExecuteLoopStep_ForEach_EmptyItems(t *testing.T) {
	// Given: A workflow with a for_each loop with empty items list
	repo := newMockRepository()
	repo.workflows["test-foreach-empty"] = &workflow.Workflow{
		Name:    "test-foreach-empty",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `[]`,
					Body:          []string{"process"},
					MaxIterations: 10,
					OnComplete:    "done",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo item",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-foreach-empty", nil)

	// Then: Should complete successfully without executing body
	require.NoError(t, err, "for_each loop with empty items should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

func TestExecuteLoopStep_While_ConditionFalseInitially(t *testing.T) {
	// Given: A workflow with a while loop where condition is false from start
	repo := newMockRepository()
	repo.workflows["test-while-false"] = &workflow.Workflow{
		Name:    "test-while-false",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:           workflow.LoopTypeWhile,
					Condition:      "false",
					Body:           []string{"process"},
					MaxIterations:  10,
					BreakCondition: "false",
					OnComplete:     "done",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo item",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()

	evaluator := newConditionMockEvaluator()
	evaluator.evaluations["false"] = false

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionServiceWithEvaluator(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil, evaluator)

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-while-false", nil)

	// Then: Should complete successfully without executing body
	require.NoError(t, err, "while loop with false condition should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

func TestExecuteLoopStep_ForEach_MaxIterationsLimit(t *testing.T) {
	// Given: A workflow with a for_each loop that exceeds max iterations
	repo := newMockRepository()
	repo.workflows["test-foreach-maxiter"] = &workflow.Workflow{
		Name:    "test-foreach-maxiter",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a", "b", "c", "d", "e"]`,
					Body:          []string{"process"},
					MaxIterations: 2,
					OnComplete:    "done",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.loop.Item}}",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo a"] = &ports.CommandResult{Stdout: "a\n", ExitCode: 0}
	executor.results["echo b"] = &ports.CommandResult{Stdout: "b\n", ExitCode: 0}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-foreach-maxiter", nil)

	// Then: Should complete successfully (only processes first 2 items)
	require.NoError(t, err, "for_each loop with max iterations should complete successfully")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

func TestExecuteLoopStep_While_MaxIterationsReached(t *testing.T) {
	// Given: A workflow with a while loop that reaches max iterations
	repo := newMockRepository()
	repo.workflows["test-while-maxiter"] = &workflow.Workflow{
		Name:    "test-while-maxiter",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:           workflow.LoopTypeWhile,
					Condition:      "true",
					Body:           []string{"process"},
					MaxIterations:  2,
					BreakCondition: "false",
					OnComplete:     "done",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.loop.Index}}",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo 0"] = &ports.CommandResult{Stdout: "0\n", ExitCode: 0}
	executor.results["echo 1"] = &ports.CommandResult{Stdout: "1\n", ExitCode: 0}

	evaluator := newConditionMockEvaluator()
	evaluator.evaluations["true"] = true
	evaluator.evaluations["false"] = false

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionServiceWithEvaluator(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil, evaluator)

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-while-maxiter", nil)

	// Then: Should complete successfully after reaching max iterations
	require.NoError(t, err, "while loop reaching max iterations should complete successfully")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

func TestExecuteLoopStep_ContextCancellation(t *testing.T) {
	// Given: A workflow with a loop step and a cancelled context
	repo := newMockRepository()
	repo.workflows["test-cancel"] = &workflow.Workflow{
		Name:    "test-cancel",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a", "b", "c"]`,
					Body:          []string{"process"},
					MaxIterations: 10,
					OnComplete:    "done",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.loop.Item}}",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	// When: Executing with a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	execCtx, err := execSvc.Run(ctx, "test-cancel", nil)

	// Then: Should fail with context cancellation error
	require.Error(t, err, "loop execution with cancelled context should error")
	assert.Equal(t, workflow.StatusCancelled, execCtx.Status)
}

func TestExecuteLoopStep_ForEach_BreakCondition(t *testing.T) {
	// Given: A workflow with a for_each loop that has a break condition
	repo := newMockRepository()
	repo.workflows["test-foreach-break"] = &workflow.Workflow{
		Name:    "test-foreach-break",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:           workflow.LoopTypeForEach,
					Items:          `["a", "b", "c"]`,
					Body:           []string{"process"},
					MaxIterations:  10,
					BreakCondition: "loop.index == 1",
					OnComplete:     "done",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.loop.Item}}",
				OnSuccess: "",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo a"] = &ports.CommandResult{Stdout: "a\n", ExitCode: 0}
	executor.results["echo b"] = &ports.CommandResult{Stdout: "b\n", ExitCode: 0}

	evaluator := newConditionMockEvaluator()
	// Break condition evaluates to true after second item
	evaluator.evaluations["loop.index == 1"] = true

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionServiceWithEvaluator(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil, evaluator)

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-foreach-break", nil)

	// Then: Should complete successfully after breaking
	require.NoError(t, err, "for_each loop with break condition should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

// ============================================================================
// HandleMaxIterationFailure Tests (C027-T005)
// ============================================================================

// TestHandleMaxIterationFailure_WithFailures tests HandleMaxIterationFailure when loop has step failures
func TestHandleMaxIterationFailure_WithFailures(t *testing.T) {
	// Given: A loop result with step failures
	loopResult := &workflow.LoopResult{
		Iterations: []workflow.IterationResult{
			{
				StepResults: map[string]*workflow.StepState{
					"body_step": {
						Name:   "body_step",
						Status: workflow.StatusFailed,
						Error:  "command failed",
					},
				},
			},
		},
		TotalCount: 10,
	}

	step := &workflow.Step{
		Name: "test_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Body:          []string{"body_step"},
			MaxIterations: 10,
		},
		OnFailure: "", // No OnFailure configured
	}

	wf := &workflow.Workflow{
		Name: "test-workflow",
		Steps: map[string]*workflow.Step{
			"test_loop": step,
			"body_step": {
				Name:    "body_step",
				Type:    workflow.StepTypeCommand,
				Command: "echo test",
			},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	loopState := workflow.StepState{
		Name:   "test_loop",
		Status: workflow.StatusRunning,
	}

	// Create execution service using testutil harness
	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-workflow", wf).
		Build()

	// When: HandleMaxIterationFailure is called
	nextStep, err := execSvc.HandleMaxIterationFailure(
		context.Background(),
		loopResult,
		step,
		wf,
		execCtx,
		&loopState,
	)

	// Then: Should return error with "with step failures" message
	require.Error(t, err, "should return error when no OnFailure configured")
	assert.Contains(t, err.Error(), "with step failures", "error should mention step failures")
	assert.Empty(t, nextStep, "should not return next step when error occurs")

	// And: Loop state should be set to Failed
	assert.Equal(t, workflow.StatusFailed, loopState.Status)
	assert.Contains(t, loopState.Error, "with step failures")
}

// TestHandleMaxIterationFailure_WithComplexSteps tests HandleMaxIterationFailure when loop has complex nested steps
func TestHandleMaxIterationFailure_WithComplexSteps(t *testing.T) {
	// Given: A loop result with complex nested steps (while loop inside)
	loopResult := &workflow.LoopResult{
		Iterations: []workflow.IterationResult{
			{
				StepResults: map[string]*workflow.StepState{
					"nested_while": {
						Name:   "nested_while",
						Status: workflow.StatusCompleted,
					},
				},
			},
		},
		TotalCount: 5,
	}

	step := &workflow.Step{
		Name: "outer_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Body:          []string{"nested_while"},
			MaxIterations: 5,
		},
		OnFailure: "", // No OnFailure configured
	}

	wf := &workflow.Workflow{
		Name: "test-workflow",
		Steps: map[string]*workflow.Step{
			"outer_loop": step,
			"nested_while": {
				Name: "nested_while",
				Type: workflow.StepTypeWhile, // Complex step type
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeWhile,
					Body:          []string{"inner_step"},
					MaxIterations: 3,
				},
			},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	loopState := workflow.StepState{
		Name:   "outer_loop",
		Status: workflow.StatusRunning,
	}

	// Create execution service using testutil harness
	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-workflow", wf).
		Build()

	// When: HandleMaxIterationFailure is called
	nextStep, err := execSvc.HandleMaxIterationFailure(
		context.Background(),
		loopResult,
		step,
		wf,
		execCtx,
		&loopState,
	)

	// Then: Should return error with "with nested complexity" message
	require.Error(t, err, "should return error when no OnFailure configured")
	assert.Contains(t, err.Error(), "with nested complexity", "error should mention nested complexity")
	assert.Empty(t, nextStep, "should not return next step when error occurs")

	// And: Loop state should be set to Failed
	assert.Equal(t, workflow.StatusFailed, loopState.Status)
	assert.Contains(t, loopState.Error, "with nested complexity")
}

// TestHandleMaxIterationFailure_WithOnFailureTransition tests HandleMaxIterationFailure when step has OnFailure configured
func TestHandleMaxIterationFailure_WithOnFailureTransition(t *testing.T) {
	// Given: A loop result with failures and OnFailure configured
	loopResult := &workflow.LoopResult{
		Iterations: []workflow.IterationResult{
			{
				StepResults: map[string]*workflow.StepState{
					"body_step": {
						Name:   "body_step",
						Status: workflow.StatusFailed,
						Error:  "command failed",
					},
				},
			},
		},
		TotalCount: 10,
	}

	step := &workflow.Step{
		Name: "test_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Body:          []string{"body_step"},
			MaxIterations: 10,
		},
		OnFailure: "error_handler", // OnFailure configured
	}

	wf := &workflow.Workflow{
		Name: "test-workflow",
		Steps: map[string]*workflow.Step{
			"test_loop": step,
			"body_step": {
				Name:    "body_step",
				Type:    workflow.StepTypeCommand,
				Command: "false",
			},
			"error_handler": {
				Name: "error_handler",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	loopState := workflow.StepState{
		Name:   "test_loop",
		Status: workflow.StatusRunning,
	}

	// Create execution service using testutil harness
	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-workflow", wf).
		Build()

	// When: HandleMaxIterationFailure is called
	nextStep, err := execSvc.HandleMaxIterationFailure(
		context.Background(),
		loopResult,
		step,
		wf,
		execCtx,
		&loopState,
	)

	// Then: Should return OnFailure step without error
	require.NoError(t, err, "should not return error when OnFailure configured")
	assert.Equal(t, "error_handler", nextStep, "should return OnFailure step")

	// And: Loop state should be set to Failed
	assert.Equal(t, workflow.StatusFailed, loopState.Status)
	assert.Contains(t, loopState.Error, "with step failures")
}

// TestHandleMaxIterationFailure_NoOnFailureReturnsError tests HandleMaxIterationFailure when no OnFailure is configured
func TestHandleMaxIterationFailure_NoOnFailureReturnsError(t *testing.T) {
	// Given: A loop result with failures and NO OnFailure configured
	loopResult := &workflow.LoopResult{
		Iterations: []workflow.IterationResult{
			{
				StepResults: map[string]*workflow.StepState{
					"body_step": {
						Name:   "body_step",
						Status: workflow.StatusFailed,
					},
				},
			},
		},
		TotalCount: 5,
	}

	step := &workflow.Step{
		Name: "test_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Body:          []string{"body_step"},
			MaxIterations: 5,
		},
		OnFailure: "", // Explicitly no OnFailure
	}

	wf := &workflow.Workflow{
		Name: "test-workflow",
		Steps: map[string]*workflow.Step{
			"test_loop": step,
			"body_step": {
				Name:    "body_step",
				Type:    workflow.StepTypeCommand,
				Command: "exit 1",
			},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	loopState := workflow.StepState{
		Name:   "test_loop",
		Status: workflow.StatusRunning,
	}

	// Create execution service using testutil harness
	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-workflow", wf).
		Build()

	// When: HandleMaxIterationFailure is called
	nextStep, err := execSvc.HandleMaxIterationFailure(
		context.Background(),
		loopResult,
		step,
		wf,
		execCtx,
		&loopState,
	)

	// Then: Should return error and empty next step
	require.Error(t, err, "should return error when no OnFailure configured")
	assert.Empty(t, nextStep, "should not return next step")
	assert.Contains(t, err.Error(), "test_loop", "error should mention loop name")
	assert.Contains(t, err.Error(), "loop reached maximum iterations", "error should mention max iterations")
}

// TestHandleMaxIterationFailure_ExecutesPostHooks tests HandleMaxIterationFailure executes post-hooks
func TestHandleMaxIterationFailure_ExecutesPostHooks(t *testing.T) {
	// Given: A loop with post-hooks configured
	loopResult := &workflow.LoopResult{
		Iterations: []workflow.IterationResult{
			{
				StepResults: map[string]*workflow.StepState{
					"body_step": {
						Name:   "body_step",
						Status: workflow.StatusFailed,
					},
				},
			},
		},
		TotalCount: 3,
	}

	step := &workflow.Step{
		Name: "test_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Body:          []string{"body_step"},
			MaxIterations: 3,
		},
		Hooks: workflow.StepHooks{
			Post: workflow.Hook{
				workflow.HookAction{Command: "echo 'cleanup after failure'"},
			},
		},
		OnFailure: "error_handler",
	}

	wf := &workflow.Workflow{
		Name: "test-workflow",
		Steps: map[string]*workflow.Step{
			"test_loop": step,
			"body_step": {
				Name:    "body_step",
				Type:    workflow.StepTypeCommand,
				Command: "false",
			},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	loopState := workflow.StepState{
		Name:   "test_loop",
		Status: workflow.StatusRunning,
	}

	// Create execution service using testutil harness
	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test-workflow", wf).
		Build()

	// When: HandleMaxIterationFailure is called
	nextStep, err := execSvc.HandleMaxIterationFailure(
		context.Background(),
		loopResult,
		step,
		wf,
		execCtx,
		&loopState,
	)

	// Then: Should execute post-hooks
	require.NoError(t, err, "should not error with OnFailure configured")
	assert.Equal(t, "error_handler", nextStep)

	// Verify hook was executed by checking executor calls
	calls := mocks.Executor.GetCalls()
	hookExecuted := false
	for _, call := range calls {
		if call.Program == "echo 'cleanup after failure'" {
			hookExecuted = true
			break
		}
	}
	assert.True(t, hookExecuted, "post-hook should have been executed")
}

// TestHandleMaxIterationFailure_UpdatesLoopState tests HandleMaxIterationFailure updates loop state correctly
func TestHandleMaxIterationFailure_UpdatesLoopState(t *testing.T) {
	// Given: A loop result with both failures and complex steps
	loopResult := &workflow.LoopResult{
		Iterations: []workflow.IterationResult{
			{
				StepResults: map[string]*workflow.StepState{
					"body_step": {
						Name:   "body_step",
						Status: workflow.StatusFailed,
						Error:  "step failed",
					},
				},
			},
		},
		TotalCount: 7,
	}

	step := &workflow.Step{
		Name: "test_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Body:          []string{"body_step"},
			MaxIterations: 7,
		},
		OnFailure: "cleanup",
	}

	wf := &workflow.Workflow{
		Name: "test-workflow",
		Steps: map[string]*workflow.Step{
			"test_loop": step,
			"body_step": {
				Name:    "body_step",
				Type:    workflow.StepTypeCommand,
				Command: "exit 1",
			},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	loopState := workflow.StepState{
		Name:   "test_loop",
		Status: workflow.StatusRunning,
		Output: "initial output",
	}

	// Create execution service using testutil harness
	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-workflow", wf).
		Build()

	// When: HandleMaxIterationFailure is called
	_, err := execSvc.HandleMaxIterationFailure(
		context.Background(),
		loopResult,
		step,
		wf,
		execCtx,
		&loopState,
	)

	// Then: Loop state should be updated correctly
	require.NoError(t, err)

	// Verify loop state updates
	assert.Equal(t, workflow.StatusFailed, loopState.Status, "status should be Failed")
	assert.NotEmpty(t, loopState.Error, "error message should be set")
	assert.Contains(t, loopState.Error, "loop reached maximum iterations", "error should contain base message")
	assert.Contains(t, loopState.Error, "with step failures", "error should mention failures")

	// Verify state was saved to execution context
	savedState, exists := execCtx.GetStepState("test_loop")
	assert.True(t, exists, "state should be saved to execution context")
	assert.Equal(t, workflow.StatusFailed, savedState.Status)
	assert.Equal(t, loopState.Error, savedState.Error)
}
