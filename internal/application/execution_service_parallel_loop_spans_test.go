package application_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestExecuteParallelStep_EmitsParallelSpan verifies parallel span is created with name "parallel".
func TestExecuteParallelStep_EmitsParallelSpan(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Strategy:  "all_succeed",
				Branches:  []string{"branch1", "branch2"},
				OnSuccess: "end",
			},
			"branch1": {
				Name:      "branch1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo branch1",
				OnSuccess: "end",
			},
			"branch2": {
				Name:      "branch2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo branch2",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockParallelSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, "parallel").
		Return(context.Background(), mockParallelSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()
	svc.SetTracer(mockTracer)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	mockTracer.AssertCalled(t, "Start", mock.Anything, "parallel")
	mockParallelSpan.AssertCalled(t, "End")
}

// TestExecuteLoopStep_ForEach_EmitsLoopForEachSpan verifies loop.for_each span is created for forEach loops.
func TestExecuteLoopStep_ForEach_EmitsLoopForEachSpan(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:       workflow.LoopTypeForEach,
					Items:      `["a", "b"]`,
					Body:       []string{"process"},
					OnComplete: "end",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockLoopSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, "loop.for_each").
		Return(context.Background(), mockLoopSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()
	svc.SetTracer(mockTracer)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	mockTracer.AssertCalled(t, "Start", mock.Anything, "loop.for_each")
	mockLoopSpan.AssertCalled(t, "End")
}

// TestExecuteLoopStep_While_EmitsLoopWhileSpan verifies loop.while span is created for while loops.
func TestExecuteLoopStep_While_EmitsLoopWhileSpan(t *testing.T) {
	repo := newMockRepository()
	wf := &workflow.Workflow{
		Name:    "test-while",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeWhile,
					Condition:     "true",
					Body:          []string{"process"},
					MaxIterations: 2,
					OnComplete:    "end",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}
	repo.workflows["test-while"] = wf

	mockTracer := new(MockTracer)
	mockLoopSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, "loop.while").
		Return(context.Background(), mockLoopSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	evaluator := newConditionMockEvaluator()
	evaluator.evaluations["true"] = true

	executor := newMockExecutor()
	executor.results["echo test"] = &ports.CommandResult{Stdout: "test\n", ExitCode: 0}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	svc := application.NewExecutionServiceWithEvaluator(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil, evaluator)
	svc.SetTracer(mockTracer)

	_, err := svc.Run(context.Background(), "test-while", nil)
	require.NoError(t, err)

	mockTracer.AssertCalled(t, "Start", mock.Anything, "loop.while")
	mockLoopSpan.AssertCalled(t, "End")
}

// TestExecuteLoopStep_EmitsPerIterationChildSpans verifies loop.iteration.N spans are created for each iteration.
func TestExecuteLoopStep_EmitsPerIterationChildSpans(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:       workflow.LoopTypeForEach,
					Items:      `[1, 2]`,
					Body:       []string{"process"},
					OnComplete: "end",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockIterSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, mock.MatchedBy(func(name string) bool {
		return name == "loop.iteration.1" || name == "loop.iteration.2"
	})).Return(context.Background(), mockIterSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()
	svc.SetTracer(mockTracer)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	mockTracer.AssertCalled(t, "Start", mock.Anything, "loop.iteration.1")
	mockTracer.AssertCalled(t, "Start", mock.Anything, "loop.iteration.2")
	mockIterSpan.AssertCalled(t, "End")
}

// TestExecuteParallelStep_WithoutTracer_CompletesSuccessfully verifies execution works when tracer is nil.
func TestExecuteParallelStep_WithoutTracer_CompletesSuccessfully(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Strategy:  "all_succeed",
				Branches:  []string{"branch1"},
				OnSuccess: "end",
			},
			"branch1": {
				Name:      "branch1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)
}

// TestExecuteLoopStep_WithoutTracer_CompletesSuccessfully verifies loop execution works when tracer is nil.
func TestExecuteLoopStep_WithoutTracer_CompletesSuccessfully(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:       workflow.LoopTypeForEach,
					Items:      `[1]`,
					Body:       []string{"process"},
					OnComplete: "end",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)
}
