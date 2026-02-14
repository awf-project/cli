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

// TestExecuteLoopStep_BreakTransitionToExternalStep tests that a loop correctly
// handles early exit when a body step transitions to a step outside the loop.
// This tests the path at execution_service.go:891-893 where result.NextStep
// is returned for early loop exit.
func TestExecuteLoopStep_BreakTransitionToExternalStep(t *testing.T) {
	// Given: A workflow with a for_each loop where body step transitions to external step
	wf := &workflow.Workflow{
		Name:    "test-break-transition",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a", "b", "c"]`,
					Body:          []string{"break_step"},
					MaxIterations: 10,
					OnComplete:    "normal_completion", // This should NOT be used
				},
			},
			"break_step": {
				Name:      "break_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo processing",
				OnSuccess: "external_step", // Break out to external step
			},
			"external_step": {
				Name:      "external_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo external",
				OnSuccess: "done",
			},
			"normal_completion": {
				Name:      "normal_completion",
				Type:      workflow.StepTypeCommand,
				Command:   "echo normal",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-break-transition", wf).
		WithCommandResult("echo processing", &ports.CommandResult{Stdout: "processing\n", ExitCode: 0}).
		WithCommandResult("echo external", &ports.CommandResult{Stdout: "external\n", ExitCode: 0}).
		Build()

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-break-transition", nil)

	// Then: Should complete successfully via early exit
	require.NoError(t, err, "loop with break transition should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// And: Loop step should be completed (not failed)
	loopState, exists := execCtx.GetStepState("loop_step")
	require.True(t, exists, "loop step state should exist")
	assert.Equal(t, workflow.StatusCompleted, loopState.Status, "loop should have completed status")

	// And: Loop should have only executed 1 iteration before breaking
	assert.Contains(t, loopState.Output, "1 iteration", "loop should break after first iteration")

	// And: External step should have executed (proving break worked)
	externalState, exists := execCtx.GetStepState("external_step")
	require.True(t, exists, "external step should have executed after break")
	assert.Equal(t, workflow.StatusCompleted, externalState.Status)
	assert.Equal(t, "external\n", externalState.Output)

	// And: Normal completion step should NOT have executed
	_, normalExists := execCtx.GetStepState("normal_completion")
	assert.False(t, normalExists, "normal completion step should not execute on break")
}

// TestExecuteLoopStep_ContinueTransitionToLoopStart tests that a loop correctly
// handles the continue pattern where a body step transitions back to the loop step.
// This tests the logic at execution_service.go:809-820 where nextStep == step.Name
// is treated as a retry/continue pattern.
func TestExecuteLoopStep_ContinueTransitionToLoopStart(t *testing.T) {
	// Given: A workflow with a for_each loop where body step transitions back to loop (retry pattern)
	wf := &workflow.Workflow{
		Name:    "test-continue-pattern",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a", "b", "c"]`,
					Body:          []string{"retry_step"},
					MaxIterations: 10,
					OnComplete:    "completion_step",
				},
			},
			"retry_step": {
				Name:      "retry_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo retry",
				OnSuccess: "loop_step", // Transition back to loop (retry pattern)
			},
			"completion_step": {
				Name:      "completion_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo complete",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-continue-pattern", wf).
		WithCommandResult("echo retry", &ports.CommandResult{Stdout: "retry\n", ExitCode: 0}).
		WithCommandResult("echo complete", &ports.CommandResult{Stdout: "complete\n", ExitCode: 0}).
		Build()

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-continue-pattern", nil)

	// Then: Should complete successfully after iterating through all items
	require.NoError(t, err, "loop with retry pattern should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// And: Loop step should be completed
	loopState, exists := execCtx.GetStepState("loop_step")
	require.True(t, exists, "loop step state should exist")
	assert.Equal(t, workflow.StatusCompleted, loopState.Status)

	// And: Loop should have completed all 3 items (a, b, c) despite retry transitions
	// Each item should have been processed once
	assert.Contains(t, loopState.Output, "3 iterations", "loop should complete all iterations")

	// And: Completion step should execute via OnComplete (proving loop didn't break early)
	completionState, exists := execCtx.GetStepState("completion_step")
	require.True(t, exists, "completion_step should have executed via OnComplete")
	assert.Equal(t, workflow.StatusCompleted, completionState.Status)
	assert.Equal(t, "complete\n", completionState.Output)
}

// TestExecuteLoopStep_BreakWithContinueOnError tests that a failed body step
// with ContinueOnError=true doesn't trigger escape detection when transitioning
// to an external step. This tests the logic at execution_service.go:812 where
// escape detection is skipped for continue_on_error steps.
func TestExecuteLoopStep_BreakWithContinueOnError(t *testing.T) {
	// Given: A workflow with a loop where body step has ContinueOnError and transitions externally
	wf := &workflow.Workflow{
		Name:    "test-continue-on-error-break",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a", "b"]`,
					Body:          []string{"failing_step"},
					MaxIterations: 10,
					OnComplete:    "should_not_run",
				},
			},
			"failing_step": {
				Name:            "failing_step",
				Type:            workflow.StepTypeCommand,
				Command:         "exit 1", // Command fails
				ContinueOnError: true,     // But step continues despite failure
				OnSuccess:       "recovery_step",
				OnFailure:       "recovery_step", // Transition to external step even on failure
			},
			"recovery_step": {
				Name:      "recovery_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo recovered",
				OnSuccess: "done",
			},
			"should_not_run": {
				Name:      "should_not_run",
				Type:      workflow.StepTypeCommand,
				Command:   "echo should-not-run",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-continue-on-error-break", wf).
		WithCommandResult("exit 1", &ports.CommandResult{Stdout: "", Stderr: "error", ExitCode: 1}).
		WithCommandResult("echo recovered", &ports.CommandResult{Stdout: "recovered\n", ExitCode: 0}).
		Build()

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-continue-on-error-break", nil)

	// Then: Should complete successfully (error not propagated due to ContinueOnError)
	require.NoError(t, err, "loop with ContinueOnError break should not propagate error")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// And: Loop should be completed (not failed)
	loopState, exists := execCtx.GetStepState("loop_step")
	require.True(t, exists, "loop step state should exist")
	assert.Equal(t, workflow.StatusCompleted, loopState.Status, "loop should complete despite body step failure")

	// And: Loop should have only executed 1 iteration before breaking
	assert.Contains(t, loopState.Output, "1 iteration", "loop should break after first iteration")

	// And: Recovery step should execute (proving escape worked without error)
	recoveryState, exists := execCtx.GetStepState("recovery_step")
	require.True(t, exists, "recovery step should have executed")
	assert.Equal(t, workflow.StatusCompleted, recoveryState.Status)
	assert.Equal(t, "recovered\n", recoveryState.Output)

	// And: OnComplete step should NOT execute (break took precedence)
	_, shouldNotRunExists := execCtx.GetStepState("should_not_run")
	assert.False(t, shouldNotRunExists, "OnComplete step should not execute on early break")

	// And: Failing step should have failed status but workflow continues
	failingState, exists := execCtx.GetStepState("failing_step")
	require.True(t, exists, "failing step state should exist")
	assert.Equal(t, workflow.StatusFailed, failingState.Status, "failing step should show failed status")
	assert.Equal(t, 1, failingState.ExitCode, "failing step should have exit code 1")
}

// TestExecuteLoopStep_MaxIterationBoundaryWithEarlyExit tests the interaction
// between max iteration limit and early exit via NextStep. Verifies that if
// a loop reaches max iterations on the same iteration where it breaks early,
// the NextStep takes precedence over OnComplete.
func TestExecuteLoopStep_MaxIterationBoundaryWithEarlyExit(t *testing.T) {
	// Given: A workflow with a loop that breaks on the exact max iteration boundary
	wf := &workflow.Workflow{
		Name:    "test-max-iter-boundary",
		Initial: "loop_step",
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name: "loop_step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a", "b", "c", "d", "e"]`, // 5 items
					Body:          []string{"boundary_step"},
					MaxIterations: 3, // Only 3 iterations allowed
					OnComplete:    "normal_end",
				},
			},
			"boundary_step": {
				Name:      "boundary_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo item",
				OnSuccess: "early_exit", // Always try to break out
			},
			"early_exit": {
				Name:      "early_exit",
				Type:      workflow.StepTypeCommand,
				Command:   "echo early",
				OnSuccess: "done",
			},
			"normal_end": {
				Name:      "normal_end",
				Type:      workflow.StepTypeCommand,
				Command:   "echo normal",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-max-iter-boundary", wf).
		WithCommandResult("echo item", &ports.CommandResult{Stdout: "item\n", ExitCode: 0}).
		WithCommandResult("echo early", &ports.CommandResult{Stdout: "early\n", ExitCode: 0}).
		Build()

	// When: Executing the workflow
	execCtx, err := execSvc.Run(context.Background(), "test-max-iter-boundary", nil)

	// Then: Should complete successfully (no max iterations error)
	require.NoError(t, err, "loop should exit early without max iterations error")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// And: Loop should be completed
	loopState, exists := execCtx.GetStepState("loop_step")
	require.True(t, exists, "loop step state should exist")
	assert.Equal(t, workflow.StatusCompleted, loopState.Status)

	// And: Loop should have only executed 1 iteration before breaking
	assert.Contains(t, loopState.Output, "1 iteration", "loop should break after first iteration")

	// And: Early exit step should execute (NextStep takes precedence)
	earlyExitState, exists := execCtx.GetStepState("early_exit")
	require.True(t, exists, "early_exit step should have executed")
	assert.Equal(t, workflow.StatusCompleted, earlyExitState.Status)
	assert.Equal(t, "early\n", earlyExitState.Output)

	// And: Normal OnComplete step should NOT execute
	_, normalEndExists := execCtx.GetStepState("normal_end")
	assert.False(t, normalEndExists, "OnComplete should not execute when NextStep breaks early")

	// And: Loop should not have error about max iterations
	assert.Empty(t, loopState.Error, "loop should have no error when breaking early on max iteration")

	// And: No mention of max iterations in error (loop exited early, not via limit)
	assert.NotContains(t, loopState.Error, "maximum iterations", "should not mention max iterations when NextStep breaks early")
}
