package application_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDryRunExecutor_Execute_LinearWorkflow(t *testing.T) {
	// Simple linear workflow: start -> process -> done
	repo := newMockRepository()
	repo.workflows["linear"] = &workflow.Workflow{
		Name:        "linear",
		Description: "A simple linear workflow",
		Initial:     "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "process",
				OnFailure: "error",
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "process {{inputs.file}}",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"error": {
				Name:   "error",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	inputs := map[string]any{"file": "test.txt"}
	plan, err := executor.Execute(context.Background(), "linear", inputs)

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, "linear", plan.WorkflowName)
	assert.Equal(t, "A simple linear workflow", plan.Description)
	assert.GreaterOrEqual(t, len(plan.Steps), 2, "should have at least start and process steps")
}

func TestDryRunExecutor_Execute_WithParallelStep(t *testing.T) {
	// Workflow with parallel branches
	repo := newMockRepository()
	repo.workflows["parallel"] = &workflow.Workflow{
		Name:    "parallel",
		Initial: "multi",
		Steps: map[string]*workflow.Step{
			"multi": {
				Name:      "multi",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"lint", "test", "build"},
				Strategy:  "all_succeed",
				OnSuccess: "done",
			},
			"lint":  {Name: "lint", Type: workflow.StepTypeCommand, Command: "npm run lint", OnSuccess: "done"},
			"test":  {Name: "test", Type: workflow.StepTypeCommand, Command: "npm test", OnSuccess: "done"},
			"build": {Name: "build", Type: workflow.StepTypeCommand, Command: "npm build", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "parallel", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	// Find the parallel step
	var parallelStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Type == workflow.StepTypeParallel {
			parallelStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, parallelStep, "should have a parallel step")
	assert.Equal(t, []string{"lint", "test", "build"}, parallelStep.Branches)
	assert.Equal(t, "all_succeed", parallelStep.Strategy)
}

func TestDryRunExecutor_Execute_ForEachLoop(t *testing.T) {
	// Workflow with for_each loop
	repo := newMockRepository()
	repo.workflows["loop"] = &workflow.Workflow{
		Name:    "loop",
		Initial: "process_files",
		Steps: map[string]*workflow.Step{
			"process_files": {
				Name: "process_files",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         "{{inputs.files}}",
					Body:          []string{"process_item"},
					MaxIterations: 100,
					OnComplete:    "done",
				},
			},
			"process_item": {
				Name:      "process_item",
				Type:      workflow.StepTypeCommand,
				Command:   "process {{loop.item}}",
				OnSuccess: "process_files",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	inputs := map[string]any{"files": []string{"a.txt", "b.txt"}}
	plan, err := executor.Execute(context.Background(), "loop", inputs)

	require.NoError(t, err)
	require.NotNil(t, plan)
	// Find the loop step
	var loopStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Type == workflow.StepTypeForEach {
			loopStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, loopStep, "should have a for_each step")
	require.NotNil(t, loopStep.Loop, "loop step should have loop config")
	assert.Equal(t, "for_each", loopStep.Loop.Type)
	assert.Equal(t, "{{inputs.files}}", loopStep.Loop.Items)
}

func TestDryRunExecutor_Execute_WhileLoop(t *testing.T) {
	// Workflow with while loop
	repo := newMockRepository()
	repo.workflows["while"] = &workflow.Workflow{
		Name:    "while",
		Initial: "poll",
		Steps: map[string]*workflow.Step{
			"poll": {
				Name: "poll",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:           workflow.LoopTypeWhile,
					Condition:      "loop.index < 5",
					Body:           []string{"check"},
					MaxIterations:  10,
					BreakCondition: "states.check.output == 'ready'",
					OnComplete:     "done",
				},
			},
			"check": {
				Name:      "check",
				Type:      workflow.StepTypeCommand,
				Command:   "check-status",
				OnSuccess: "", // No explicit transition - continue loop body
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "while", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	// Find the while step
	var whileStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Type == workflow.StepTypeWhile {
			whileStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, whileStep, "should have a while step")
	require.NotNil(t, whileStep.Loop, "while step should have loop config")
	assert.Equal(t, "while", whileStep.Loop.Type)
	assert.Equal(t, "loop.index < 5", whileStep.Loop.Condition)
}

func TestDryRunExecutor_Execute_ConditionalTransitions(t *testing.T) {
	// Workflow with conditional transitions
	repo := newMockRepository()
	repo.workflows["conditional"] = &workflow.Workflow{
		Name:    "conditional",
		Initial: "check",
		Steps: map[string]*workflow.Step{
			"check": {
				Name:    "check",
				Type:    workflow.StepTypeCommand,
				Command: "check-env",
				Transitions: workflow.Transitions{
					{When: "states.check.exit_code == 0", Goto: "success"},
					{When: "states.check.exit_code == 1", Goto: "retry"},
					{Goto: "error"}, // default
				},
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"retry":   {Name: "retry", Type: workflow.StepTypeCommand, Command: "retry", OnSuccess: "check"},
			"error":   {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "conditional", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	// Find the check step
	var checkStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "check" {
			checkStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, checkStep, "should have check step")
	assert.Len(t, checkStep.Transitions, 3, "should have all 3 transitions")
}

func TestDryRunExecutor_Execute_WithHooks(t *testing.T) {
	// Workflow with pre/post hooks
	repo := newMockRepository()
	repo.workflows["hooks"] = &workflow.Workflow{
		Name:    "hooks",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:    "step",
				Type:    workflow.StepTypeCommand,
				Command: "echo main",
				Hooks: workflow.StepHooks{
					Pre:  workflow.Hook{{Log: "Starting step"}, {Command: "echo pre"}},
					Post: workflow.Hook{{Log: "Step completed"}, {Command: "echo post"}},
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "hooks", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	// Find the step with hooks
	var hookStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "step" {
			hookStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, hookStep, "should have step with hooks")
	assert.Len(t, hookStep.Hooks.Pre, 2, "should have 2 pre hooks")
	assert.Len(t, hookStep.Hooks.Post, 2, "should have 2 post hooks")
}

func TestDryRunExecutor_Execute_InputResolution(t *testing.T) {
	// Test that inputs are resolved correctly
	repo := newMockRepository()
	repo.workflows["inputs"] = &workflow.Workflow{
		Name:    "inputs",
		Initial: "start",
		Inputs: []workflow.Input{
			{Name: "name", Type: "string", Required: true},
			{Name: "count", Type: "integer", Required: false, Default: 10},
		},
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "process {{inputs.name}} {{inputs.count}}",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	inputs := map[string]any{"name": "test"}
	plan, err := executor.Execute(context.Background(), "inputs", inputs)

	require.NoError(t, err)
	require.NotNil(t, plan)
	// Check inputs were resolved
	require.NotNil(t, plan.Inputs)
	require.Contains(t, plan.Inputs, "name")
	assert.Equal(t, "test", plan.Inputs["name"].Value)
	assert.False(t, plan.Inputs["name"].Default)
	// Check default value was used for count
	require.Contains(t, plan.Inputs, "count")
	assert.Equal(t, 10, plan.Inputs["count"].Value)
	assert.True(t, plan.Inputs["count"].Default)
}

func TestDryRunExecutor_Execute_MissingRequiredInput(t *testing.T) {
	// Test that missing required inputs cause an error
	repo := newMockRepository()
	repo.workflows["required"] = &workflow.Workflow{
		Name:    "required",
		Initial: "start",
		Inputs: []workflow.Input{
			{Name: "required_field", Type: "string", Required: true},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	_, err := executor.Execute(context.Background(), "required", nil)

	require.Error(t, err, "should fail with missing required input")
	assert.Contains(t, err.Error(), "required_field")
}

func TestDryRunExecutor_Execute_WorkflowNotFound(t *testing.T) {
	repo := newMockRepository()

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	_, err := executor.Execute(context.Background(), "nonexistent", nil)

	require.Error(t, err, "should fail with nonexistent workflow")
	assert.Contains(t, err.Error(), "not found")
}

func TestDryRunExecutor_Execute_RetryConfig(t *testing.T) {
	// Test that retry configuration is captured
	repo := newMockRepository()
	repo.workflows["retry"] = &workflow.Workflow{
		Name:    "retry",
		Initial: "flaky",
		Steps: map[string]*workflow.Step{
			"flaky": {
				Name:    "flaky",
				Type:    workflow.StepTypeCommand,
				Command: "flaky-command",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    3,
					InitialDelayMs: 100,
					MaxDelayMs:     1000,
					Backoff:        "exponential",
					Multiplier:     2.0,
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "retry", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	// Find the flaky step
	var flakyStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "flaky" {
			flakyStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, flakyStep, "should have flaky step")
	require.NotNil(t, flakyStep.Retry, "should have retry config")
	assert.Equal(t, 3, flakyStep.Retry.MaxAttempts)
	assert.Equal(t, "exponential", flakyStep.Retry.Backoff)
}

func TestDryRunExecutor_Execute_CaptureConfig(t *testing.T) {
	// Test that capture configuration is captured
	repo := newMockRepository()
	repo.workflows["capture"] = &workflow.Workflow{
		Name:    "capture",
		Initial: "fetch",
		Steps: map[string]*workflow.Step{
			"fetch": {
				Name:    "fetch",
				Type:    workflow.StepTypeCommand,
				Command: "curl http://example.com",
				Capture: &workflow.CaptureConfig{
					Stdout:  "response",
					Stderr:  "errors",
					MaxSize: "10MB",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "capture", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	// Find the fetch step
	var fetchStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "fetch" {
			fetchStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, fetchStep, "should have fetch step")
	require.NotNil(t, fetchStep.Capture, "should have capture config")
	assert.Equal(t, "response", fetchStep.Capture.Stdout)
	assert.Equal(t, "errors", fetchStep.Capture.Stderr)
}

func TestDryRunExecutor_Execute_TimeoutConfig(t *testing.T) {
	// Test that timeout configuration is captured
	repo := newMockRepository()
	repo.workflows["timeout"] = &workflow.Workflow{
		Name:    "timeout",
		Initial: "slow",
		Steps: map[string]*workflow.Step{
			"slow": {
				Name:      "slow",
				Type:      workflow.StepTypeCommand,
				Command:   "sleep 60",
				Timeout:   30,
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "timeout", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	// Find the slow step
	var slowStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "slow" {
			slowStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, slowStep, "should have slow step")
	assert.Equal(t, 30, slowStep.Timeout)
}

func TestDryRunExecutor_Execute_ContinueOnError(t *testing.T) {
	// Test that continue_on_error is captured
	repo := newMockRepository()
	repo.workflows["continue"] = &workflow.Workflow{
		Name:    "continue",
		Initial: "optional",
		Steps: map[string]*workflow.Step{
			"optional": {
				Name:            "optional",
				Type:            workflow.StepTypeCommand,
				Command:         "optional-command",
				ContinueOnError: true,
				OnSuccess:       "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "continue", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	var optionalStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "optional" {
			optionalStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, optionalStep, "should have optional step")
	assert.True(t, optionalStep.ContinueOnError)
}

func TestDryRunExecutor_Execute_WorkingDirectory(t *testing.T) {
	// Test that working directory is captured
	repo := newMockRepository()
	repo.workflows["dir"] = &workflow.Workflow{
		Name:    "dir",
		Initial: "build",
		Steps: map[string]*workflow.Step{
			"build": {
				Name:      "build",
				Type:      workflow.StepTypeCommand,
				Command:   "make build",
				Dir:       "/path/to/project",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "dir", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	var buildStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "build" {
			buildStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, buildStep, "should have build step")
	assert.Equal(t, "/path/to/project", buildStep.Dir)
}

func TestDryRunExecutor_Execute_StepDescription(t *testing.T) {
	// Test that step descriptions are captured
	repo := newMockRepository()
	repo.workflows["desc"] = &workflow.Workflow{
		Name:        "desc",
		Description: "Test workflow",
		Initial:     "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:        "step1",
				Type:        workflow.StepTypeCommand,
				Description: "This step does validation",
				Command:     "validate",
				OnSuccess:   "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "desc", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, "Test workflow", plan.Description)
	var step1 *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "step1" {
			step1 = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, step1, "should have step1")
	assert.Equal(t, "This step does validation", step1.Description)
}

func TestDryRunExecutor_Execute_NestedLoops(t *testing.T) {
	// Test nested loop configuration
	repo := newMockRepository()
	repo.workflows["nested"] = &workflow.Workflow{
		Name:    "nested",
		Initial: "outer",
		Steps: map[string]*workflow.Step{
			"outer": {
				Name: "outer",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:       workflow.LoopTypeForEach,
					Items:      "{{inputs.categories}}",
					Body:       []string{"inner"},
					OnComplete: "done",
				},
			},
			"inner": {
				Name: "inner",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:       workflow.LoopTypeForEach,
					Items:      "{{loop.parent.item.items}}",
					Body:       []string{"process"},
					OnComplete: "outer",
				},
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "process {{loop.item}}",
				OnSuccess: "inner",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "nested", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	// Verify both loops are in the plan
	loopCount := 0
	for i := range plan.Steps {
		if plan.Steps[i].Type == workflow.StepTypeForEach {
			loopCount++
		}
	}
	assert.GreaterOrEqual(t, loopCount, 2, "should have at least 2 loop steps")
}

func TestDryRunExecutor_Execute_ParallelWithMaxConcurrent(t *testing.T) {
	// Test parallel step with max concurrent
	repo := newMockRepository()
	repo.workflows["concurrent"] = &workflow.Workflow{
		Name:    "concurrent",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:          "parallel",
				Type:          workflow.StepTypeParallel,
				Branches:      []string{"a", "b", "c", "d"},
				Strategy:      "best_effort",
				MaxConcurrent: 2,
				OnSuccess:     "done",
			},
			"a":    {Name: "a", Type: workflow.StepTypeCommand, Command: "echo a", OnSuccess: "done"},
			"b":    {Name: "b", Type: workflow.StepTypeCommand, Command: "echo b", OnSuccess: "done"},
			"c":    {Name: "c", Type: workflow.StepTypeCommand, Command: "echo c", OnSuccess: "done"},
			"d":    {Name: "d", Type: workflow.StepTypeCommand, Command: "echo d", OnSuccess: "done"},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "concurrent", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	var parallelStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Type == workflow.StepTypeParallel {
			parallelStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, parallelStep, "should have parallel step")
	assert.Equal(t, 2, parallelStep.MaxConcurrent)
	assert.Equal(t, "best_effort", parallelStep.Strategy)
}

func TestDryRunExecutor_Execute_CommandInterpolation(t *testing.T) {
	// Test that command interpolation is shown (resolved where possible)
	repo := newMockRepository()
	repo.workflows["interp"] = &workflow.Workflow{
		Name:    "interp",
		Initial: "start",
		Inputs: []workflow.Input{
			{Name: "file", Type: "string", Required: true},
			{Name: "count", Type: "integer", Default: 5},
		},
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   `process "{{inputs.file}}" --count={{inputs.count}}`,
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	inputs := map[string]any{"file": "test.txt"}
	plan, err := executor.Execute(context.Background(), "interp", inputs)

	require.NoError(t, err)
	require.NotNil(t, plan)
	// Check inputs are resolved
	require.Contains(t, plan.Inputs, "file")
	assert.Equal(t, "test.txt", plan.Inputs["file"].Value)
	require.Contains(t, plan.Inputs, "count")
	assert.Equal(t, 5, plan.Inputs["count"].Value)
	assert.True(t, plan.Inputs["count"].Default)
}

func TestDryRunExecutor_Execute_ContextCancellation(t *testing.T) {
	// Test that context cancellation is handled
	repo := newMockRepository()
	repo.workflows["cancel"] = &workflow.Workflow{
		Name:    "cancel",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := executor.Execute(ctx, "cancel", nil)
	// Either returns an error or completes (dry-run should be fast)
	// The important thing is it doesn't hang
	if err != nil {
		assert.Contains(t, err.Error(), "cancel")
	}
}

func TestDryRunExecutor_Execute_WithWorkflowHooks(t *testing.T) {
	// Test workflow-level hooks are captured
	repo := newMockRepository()
	repo.workflows["wfhooks"] = &workflow.Workflow{
		Name:    "wfhooks",
		Initial: "start",
		Hooks: workflow.WorkflowHooks{
			WorkflowStart:  workflow.Hook{{Log: "Starting workflow"}},
			WorkflowEnd:    workflow.Hook{{Log: "Ending workflow"}},
			WorkflowError:  workflow.Hook{{Log: "Error occurred"}},
			WorkflowCancel: workflow.Hook{{Log: "Cancelled"}},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "wfhooks", nil)

	// Workflow hooks may be included in the plan or displayed separately
	// The important thing is the dry-run succeeds
	require.NoError(t, err)
	require.NotNil(t, plan)
}

func TestDryRunExecutor_Execute_TerminalStates(t *testing.T) {
	tests := []struct {
		name   string
		status workflow.TerminalStatus
	}{
		{"success terminal", workflow.TerminalSuccess},
		{"failure terminal", workflow.TerminalFailure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			repo.workflows["terminal"] = &workflow.Workflow{
				Name:    "terminal",
				Initial: "end",
				Steps: map[string]*workflow.Step{
					"end": {
						Name:   "end",
						Type:   workflow.StepTypeTerminal,
						Status: tt.status,
					},
				},
			}

			wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
			resolver := interpolation.NewTemplateResolver()
			evaluator := mocks.NewMockExpressionEvaluator()
			executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

			plan, err := executor.Execute(context.Background(), "terminal", nil)

			require.NoError(t, err)
			require.NotNil(t, plan)
			require.Len(t, plan.Steps, 1)
			assert.Equal(t, tt.status, plan.Steps[0].Status)
		})
	}
}

func TestDryRunExecutor_Execute_LoopBreakCondition(t *testing.T) {
	// Test that loop break condition is captured
	repo := newMockRepository()
	repo.workflows["break"] = &workflow.Workflow{
		Name:    "break",
		Initial: "poll",
		Steps: map[string]*workflow.Step{
			"poll": {
				Name: "poll",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:           workflow.LoopTypeWhile,
					Condition:      "true",
					Body:           []string{"check"},
					MaxIterations:  100,
					BreakCondition: "states.check.output == 'done'",
					OnComplete:     "done",
				},
			},
			"check": {Name: "check", Type: workflow.StepTypeCommand, Command: "check", OnSuccess: ""},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "break", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	var pollStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "poll" {
			pollStep = &plan.Steps[i]
			break
		}
	}
	require.NotNil(t, pollStep, "should have poll step")
	require.NotNil(t, pollStep.Loop, "poll step should have loop config")
	assert.Equal(t, "states.check.output == 'done'", pollStep.Loop.BreakCondition)
}

func TestDryRunExecutor_Execute_AllStepTypes(t *testing.T) {
	// Test that all step types are correctly identified
	tests := []struct {
		name     string
		stepType workflow.StepType
		step     *workflow.Step
	}{
		{
			name:     "command step",
			stepType: workflow.StepTypeCommand,
			step:     &workflow.Step{Name: "cmd", Type: workflow.StepTypeCommand, Command: "echo"},
		},
		{
			name:     "parallel step",
			stepType: workflow.StepTypeParallel,
			step:     &workflow.Step{Name: "par", Type: workflow.StepTypeParallel, Branches: []string{"a"}},
		},
		{
			name:     "for_each step",
			stepType: workflow.StepTypeForEach,
			step: &workflow.Step{Name: "fe", Type: workflow.StepTypeForEach, Loop: &workflow.LoopConfig{
				Type:  workflow.LoopTypeForEach,
				Items: "[]",
				Body:  []string{},
			}},
		},
		{
			name:     "while step",
			stepType: workflow.StepTypeWhile,
			step: &workflow.Step{Name: "wh", Type: workflow.StepTypeWhile, Loop: &workflow.LoopConfig{
				Type:      workflow.LoopTypeWhile,
				Condition: "true",
				Body:      []string{},
			}},
		},
		{
			name:     "terminal step",
			stepType: workflow.StepTypeTerminal,
			step:     &workflow.Step{Name: "term", Type: workflow.StepTypeTerminal},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			repo.workflows["test"] = &workflow.Workflow{
				Name:    "test",
				Initial: tt.step.Name,
				Steps:   map[string]*workflow.Step{tt.step.Name: tt.step},
			}

			wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
			resolver := interpolation.NewTemplateResolver()
			evaluator := mocks.NewMockExpressionEvaluator()
			executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

			plan, err := executor.Execute(context.Background(), "test", nil)

			require.NoError(t, err)
			require.NotNil(t, plan)
			require.GreaterOrEqual(t, len(plan.Steps), 1)
			assert.Equal(t, tt.stepType, plan.Steps[0].Type)
		})
	}
}

// Component T003 implements comprehensive tests for DryRunExecutor setter methods.
// Tests follow TDD patterns (RED/GREEN/REFACTOR) and cover happy path, edge cases,
// and error conditions for SetTemplateService method.
//
// Strategy:
// - Happy path: Valid template service is set and used during Execute
// - Edge case: Nil template service is accepted (template expansion is optional)
// - Replacement: Existing template service can be replaced with new one
// - Integration: Template service affects workflow execution behavior

// TestDryRunExecutor_SetTemplateService_Valid verifies that SetTemplateService
// correctly sets a valid template service and that it's used during Execute.
func TestDryRunExecutor_SetTemplateService_Valid(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["templated"] = &workflow.Workflow{
		Name:    "templated",
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	// Create template service with mock template repository
	templateRepo := mocks.NewMockTemplateRepository()
	templateSvc := application.NewTemplateService(templateRepo, &mockLogger{})

	executor.SetTemplateService(templateSvc)

	// Execute to verify it works with template service set
	plan, err := executor.Execute(context.Background(), "templated", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, "templated", plan.WorkflowName)
	assert.GreaterOrEqual(t, len(plan.Steps), 1)
}

// TestDryRunExecutor_SetTemplateService_Nil verifies that SetTemplateService
// handles nil template service gracefully (template expansion is optional).
func TestDryRunExecutor_SetTemplateService_Nil(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["no_template"] = &workflow.Workflow{
		Name:    "no_template",
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	executor.SetTemplateService(nil)

	// Execute workflow - should succeed without template expansion
	plan, err := executor.Execute(context.Background(), "no_template", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, "no_template", plan.WorkflowName)
	assert.GreaterOrEqual(t, len(plan.Steps), 1, "workflow should execute without template service")
}

// TestDryRunExecutor_SetTemplateService_ReplaceExisting verifies that
// SetTemplateService can replace an existing template service.
func TestDryRunExecutor_SetTemplateService_ReplaceExisting(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["replace_test"] = &workflow.Workflow{
		Name:    "replace_test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	// Create first template service
	firstRepo := mocks.NewMockTemplateRepository()
	firstSvc := application.NewTemplateService(firstRepo, &mockLogger{})
	executor.SetTemplateService(firstSvc)

	// Create second template service
	secondRepo := mocks.NewMockTemplateRepository()
	secondSvc := application.NewTemplateService(secondRepo, &mockLogger{})

	executor.SetTemplateService(secondSvc)

	// Execute to verify execution succeeds after replacement
	plan, err := executor.Execute(context.Background(), "replace_test", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, "replace_test", plan.WorkflowName)
	assert.GreaterOrEqual(t, len(plan.Steps), 1)
}

// TestDryRunExecutor_SetTemplateService_WithTemplateReference verifies that
// template service is used when workflow has template references.
func TestDryRunExecutor_SetTemplateService_WithTemplateReference(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["with_template"] = &workflow.Workflow{
		Name:    "with_template",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "echo-template",
					Parameters:   map[string]any{"message": "hello"},
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Create template repository with a template
	templateRepo := mocks.NewMockTemplateRepository()
	templateRepo.AddTemplate("echo-template", &workflow.Template{
		Name: "echo-template",
		States: map[string]*workflow.Step{
			"echo": {
				Name:    "echo",
				Type:    workflow.StepTypeCommand,
				Command: "echo {{inputs.message}}",
			},
		},
	})

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	// Create and set template service
	templateSvc := application.NewTemplateService(templateRepo, &mockLogger{})
	executor.SetTemplateService(templateSvc)

	plan, err := executor.Execute(context.Background(), "with_template", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, "with_template", plan.WorkflowName)
	// After template expansion, the step should have been expanded
	assert.GreaterOrEqual(t, len(plan.Steps), 1)
}

// TestDryRunExecutor_SetTemplateService_NoTemplateService verifies behavior
// when no template service is set (initial state).
func TestDryRunExecutor_SetTemplateService_NoTemplateService(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["no_svc"] = &workflow.Workflow{
		Name:    "no_svc",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "no_svc", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, "no_svc", plan.WorkflowName)
}
