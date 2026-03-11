package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//
// Tests for ExecuteForEach functionality including:
// - Simple iteration over items
// - Break conditions
// - Max iterations limiting
// - Error handling
// - Context cancellation
// - Empty items
// - Multiple body steps
// - Dynamic max_iterations (F047)

func TestLoopExecutor_ExecuteForEach_Simple(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach",
		Steps: map[string]*workflow.Step{
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo {{loop.item}}",
			},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["a", "b", "c"]`,
			Body:          []string{"process"},
			MaxIterations: 100,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach")
	recorder := newStepExecutorRecorder()

	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		recorder.execute,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.TotalCount)
	assert.Equal(t, -1, result.BrokeAt)
	assert.False(t, result.WasBroken())
	assert.Len(t, result.Iterations, 3)
	assert.Len(t, recorder.executions, 3)

	// Verify loop variables were set correctly
	assert.Equal(t, "a", recorder.executions[0].loopData.Item)
	assert.Equal(t, 0, recorder.executions[0].loopData.Index)
	assert.True(t, recorder.executions[0].loopData.First)
	assert.False(t, recorder.executions[0].loopData.Last)
	assert.Equal(t, 3, recorder.executions[0].loopData.Length)

	assert.Equal(t, "b", recorder.executions[1].loopData.Item)
	assert.Equal(t, 1, recorder.executions[1].loopData.Index)
	assert.False(t, recorder.executions[1].loopData.First)
	assert.False(t, recorder.executions[1].loopData.Last)

	assert.Equal(t, "c", recorder.executions[2].loopData.Item)
	assert.Equal(t, 2, recorder.executions[2].loopData.Index)
	assert.False(t, recorder.executions[2].loopData.First)
	assert.True(t, recorder.executions[2].loopData.Last)
}

func TestLoopExecutor_ExecuteForEach_WithBreakCondition(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	// Break at index 1
	evaluator.boolResults["states.process.output == 'stop'"] = true
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-break",
		Steps: map[string]*workflow.Step{
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo",
			},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:           workflow.LoopTypeForEach,
			Items:          `["a", "b", "c", "d"]`,
			Body:           []string{"process"},
			MaxIterations:  100,
			BreakCondition: "states.process.output == 'stop'",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-break")
	recorder := newStepExecutorRecorder()

	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		recorder.execute,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.WasBroken())
	assert.Equal(t, 0, result.BrokeAt) // Breaks after first iteration
	assert.Equal(t, 1, result.TotalCount)
}

func TestLoopExecutor_ExecuteForEach_MaxIterationsLimitsExecution(t *testing.T) {
	// F037: max_iterations now limits execution rather than causing an error
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-foreach-max",
		Steps: map[string]*workflow.Step{},
	}

	// Create items that exceed max_iterations - should only process first 3
	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["a", "b", "c", "d", "e"]`,
			Body:          []string{"process"},
			MaxIterations: 3, // Less than items count - limits to 3
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-max")

	var processedItems []string
	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			if intCtx.Loop != nil {
				processedItems = append(processedItems, fmt.Sprintf("%v", intCtx.Loop.Item))
			}
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.NoError(t, err)
	// Should only process first 3 items
	assert.Len(t, result.Iterations, 3)
	assert.Equal(t, []string{"a", "b", "c"}, processedItems)
}

func TestLoopExecutor_ExecuteForEach_StepError(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-error",
		Steps: map[string]*workflow.Step{
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "fail",
			},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["a", "b", "c"]`,
			Body:          []string{"process"},
			MaxIterations: 100,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-error")
	stepErr := errors.New("step execution failed")
	callCount := 0

	_, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			if callCount == 2 {
				return "", stepErr
			}
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.Error(t, err)
	assert.Equal(t, stepErr, err)
	assert.Equal(t, 2, callCount) // Should stop after error
}

func TestLoopExecutor_ExecuteForEach_ContextCancellation(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-foreach-cancel",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["a", "b", "c", "d", "e"]`,
			Body:          []string{"process"},
			MaxIterations: 100,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-cancel")

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	result, err := loopExec.ExecuteForEach(
		ctx,
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			if callCount == 2 {
				cancel() // Cancel after second iteration
			}
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, result.TotalCount, 5) // Should not complete all iterations
}

func TestLoopExecutor_ExecuteForEach_EmptyItems(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-foreach-empty",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `[]`,
			Body:          []string{"process"},
			MaxIterations: 100,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-empty")

	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			t.Error("should not execute with empty items")
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.TotalCount)
	assert.Empty(t, result.Iterations)
}

func TestLoopExecutor_ExecuteForEach_MultipleBodySteps(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-multi",
		Steps: map[string]*workflow.Step{
			"fetch": {Name: "fetch", Type: workflow.StepTypeCommand, Command: "curl"},
			"parse": {Name: "parse", Type: workflow.StepTypeCommand, Command: "jq"},
			"store": {Name: "store", Type: workflow.StepTypeCommand, Command: "save"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["item1", "item2"]`,
			Body:          []string{"fetch", "parse", "store"},
			MaxIterations: 100,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-multi")
	recorder := newStepExecutorRecorder()

	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		recorder.execute,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalCount)
	assert.Len(t, recorder.executions, 6) // 2 items * 3 body steps

	// Verify execution order
	assert.Equal(t, "fetch", recorder.executions[0].stepName)
	assert.Equal(t, "parse", recorder.executions[1].stepName)
	assert.Equal(t, "store", recorder.executions[2].stepName)
	assert.Equal(t, "fetch", recorder.executions[3].stepName)
	assert.Equal(t, "parse", recorder.executions[4].stepName)
	assert.Equal(t, "store", recorder.executions[5].stepName)
}

func TestLoopExecutor_ExecuteForEach_DynamicMaxIterations_FromInput(t *testing.T) {
	// Test US1: max_iterations from {{inputs.limit}}
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Configure resolver: items resolve normally, max_iterations expr resolves to "5"
	resolver.results[`["a", "b", "c"]`] = `["a", "b", "c"]`
	resolver.results["{{inputs.limit}}"] = "5"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-dynamic-max",
		Steps: map[string]*workflow.Step{
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo {{loop.item}}",
			},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeForEach,
			Items:             `["a", "b", "c"]`,
			Body:              []string{"process"},
			MaxIterations:     0, // Not used when dynamic expr is set
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-dynamic-max")
	recorder := newStepExecutorRecorder()

	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		recorder.execute,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "5"
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.TotalCount)
	assert.Len(t, recorder.executions, 3)
}

func TestLoopExecutor_ExecuteForEach_DynamicMaxIterations_LimitsToResolvedValue(t *testing.T) {
	// F037: max_iterations limits execution to resolved value (not an error)
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// 5 items but max_iterations resolves to 3 - should only process first 3
	resolver.results[`["a", "b", "c", "d", "e"]`] = `["a", "b", "c", "d", "e"]`
	resolver.results["{{inputs.limit}}"] = "3"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-dynamic-max-limited",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeForEach,
			Items:             `["a", "b", "c", "d", "e"]`,
			Body:              []string{"process"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-dynamic-max-limited")

	var processedItems []string
	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			if intCtx.Loop != nil {
				processedItems = append(processedItems, fmt.Sprintf("%v", intCtx.Loop.Item))
			}
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "3"
			return ctx
		},
	)

	require.NoError(t, err)
	// Should process only first 3 items (limited by dynamic max_iterations)
	assert.Len(t, result.Iterations, 3)
	assert.Equal(t, []string{"a", "b", "c"}, processedItems)
}

func TestLoopExecutor_ExecuteForEach_DynamicMaxIterations_ResolverError(t *testing.T) {
	// Test: resolver fails to resolve max_iterations expression
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Items resolve fine, but max_iterations expression fails
	resolver.results[`["a", "b"]`] = `["a", "b"]`
	// Note: We need a resolver that can fail for specific expressions
	// Since our mock uses a single err field, we'll create a custom one

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-dynamic-max-error",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeForEach,
			Items:             `["a", "b"]`,
			Body:              []string{"process"},
			MaxIterationsExpr: "{{inputs.undefined_var}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-dynamic-max-error")

	// Use a resolver that returns the expression unchanged (undefined variable)
	// The ResolveMaxIterations will then fail to parse it as int
	_, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			t.Error("should not execute when resolver fails")
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve max_iterations")
}

func TestLoopExecutor_ExecuteForEach_DynamicMaxIterations_InvalidValue(t *testing.T) {
	// Test: resolved max_iterations is not a valid integer
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results[`["a", "b"]`] = `["a", "b"]`
	resolver.results["{{inputs.limit}}"] = "not_a_number"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-dynamic-max-invalid",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeForEach,
			Items:             `["a", "b"]`,
			Body:              []string{"process"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-dynamic-max-invalid")

	_, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			t.Error("should not execute when max_iterations is invalid")
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "not_a_number"
			return ctx
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve max_iterations")
}

func TestLoopExecutor_ExecuteForEach_DynamicMaxIterations_ZeroValue(t *testing.T) {
	// Test: resolved max_iterations is zero (invalid)
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results[`["a"]`] = `["a"]`
	resolver.results["{{inputs.limit}}"] = "0"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-dynamic-max-zero",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeForEach,
			Items:             `["a"]`,
			Body:              []string{"process"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-dynamic-max-zero")

	_, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			t.Error("should not execute when max_iterations is zero")
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "0"
			return ctx
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 1")
}

func TestLoopExecutor_ExecuteForEach_DynamicMaxIterations_ExceedsLimit(t *testing.T) {
	// Test: resolved max_iterations exceeds MaxAllowedIterations (10000)
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results[`["a"]`] = `["a"]`
	resolver.results["{{inputs.limit}}"] = "50000"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-dynamic-max-exceeds",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeForEach,
			Items:             `["a"]`,
			Body:              []string{"process"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-dynamic-max-exceeds")

	_, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			t.Error("should not execute when max_iterations exceeds limit")
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "50000"
			return ctx
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestLoopExecutor_ExecuteForEach_DynamicMaxIterations_Arithmetic(t *testing.T) {
	// Test US3: arithmetic expression in max_iterations
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results[`["a", "b", "c", "d", "e"]`] = `["a", "b", "c", "d", "e"]`
	resolver.results["{{inputs.a + inputs.b}}"] = "2 + 3" // Resolves to arithmetic
	evaluator.intResults["2 + 3"] = 5                     // Arithmetic evaluates to 5

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-dynamic-max-arithmetic",
		Steps: map[string]*workflow.Step{
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo",
			},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeForEach,
			Items:             `["a", "b", "c", "d", "e"]`,
			Body:              []string{"process"},
			MaxIterationsExpr: "{{inputs.a + inputs.b}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-dynamic-max-arithmetic")
	recorder := newStepExecutorRecorder()

	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		recorder.execute,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["a"] = "2"
			ctx.Inputs["b"] = "3"
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 5, result.TotalCount)
}

func TestLoopExecutor_ExecuteForEach_DynamicMaxIterations_FromEnv(t *testing.T) {
	// Test US1: max_iterations from {{env.LOOP_LIMIT}}
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results[`["x", "y"]`] = `["x", "y"]`
	resolver.results["{{env.LOOP_LIMIT}}"] = "10"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-dynamic-max-env",
		Steps: map[string]*workflow.Step{
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo",
			},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeForEach,
			Items:             `["x", "y"]`,
			Body:              []string{"process"},
			MaxIterationsExpr: "{{env.LOOP_LIMIT}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-dynamic-max-env")
	recorder := newStepExecutorRecorder()

	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		recorder.execute,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Env["LOOP_LIMIT"] = "10"
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalCount)
}

func TestLoopExecutor_ExecuteForEach_DynamicMaxIterations_FromStepOutput(t *testing.T) {
	// Test US2: max_iterations from {{states.count.output}}
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results[`["item1", "item2", "item3"]`] = `["item1", "item2", "item3"]`
	resolver.results["{{states.count.output}}"] = "5"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-dynamic-max-step-output",
		Steps: map[string]*workflow.Step{
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo",
			},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeForEach,
			Items:             `["item1", "item2", "item3"]`,
			Body:              []string{"process"},
			MaxIterationsExpr: "{{states.count.output}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-dynamic-max-step-output")
	recorder := newStepExecutorRecorder()

	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		recorder.execute,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.States["count"] = interpolation.StepStateData{
				Output:   "5",
				ExitCode: 0,
				Status:   "completed",
			}
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.TotalCount)
}

func TestLoopExecutor_ExecuteForEach_StaticMaxIterations_StillWorks(t *testing.T) {
	// Test backward compatibility: static max_iterations still works
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results[`["a", "b"]`] = `["a", "b"]`

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-static-max",
		Steps: map[string]*workflow.Step{
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo",
			},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeForEach,
			Items:             `["a", "b"]`,
			Body:              []string{"process"},
			MaxIterations:     100, // Static value
			MaxIterationsExpr: "",  // No dynamic expression
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-static-max")
	recorder := newStepExecutorRecorder()

	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		recorder.execute,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalCount)
}

func TestLoopExecutor_ExecuteForEach_DynamicMaxIterations_ExactMatch(t *testing.T) {
	// Test: items count exactly matches resolved max_iterations (boundary)
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results[`["a", "b", "c"]`] = `["a", "b", "c"]`
	resolver.results["{{inputs.limit}}"] = "3"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-dynamic-max-exact",
		Steps: map[string]*workflow.Step{
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo",
			},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeForEach,
			Items:             `["a", "b", "c"]`,
			Body:              []string{"process"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-dynamic-max-exact")
	recorder := newStepExecutorRecorder()

	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		recorder.execute,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "3"
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.TotalCount)
}
