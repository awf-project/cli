package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

//
// Tests for while loop execution functionality. Covers:
// - Basic while loop execution with condition evaluation
// - MaxIterations safety limits
// - Condition evaluation errors
// - Break condition support
// - Step execution errors
// - Loop variable propagation (index, first, last, length=-1)
// - Dynamic max_iterations expressions (F037 T012)
//   - From inputs, environment variables, step outputs
//   - Arithmetic expressions (+, -, *, /)
//   - Validation (boundaries, negative, zero, exceeds max)
//   - Integration with break conditions
//   - Context cancellation
//   - Error handling (resolver errors, invalid values)
//
// Shared mock implementations are in loop_executor_mocks_test.go

func TestLoopExecutor_ExecuteWhile_Simple(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	// Condition returns true for first 3 iterations, then false
	callCount := 0
	evaluator.boolResults["states.check.output != 'ready'"] = true

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-while",
		Steps: map[string]*workflow.Step{
			"check": {
				Name:    "check",
				Type:    workflow.StepTypeCommand,
				Command: "check_status",
			},
		},
	}

	step := &workflow.Step{
		Name: "poll_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "states.check.output != 'ready'",
			Body:          []string{"check"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while")

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			// After 3 calls, make condition false
			if callCount >= 3 {
				evaluator.boolResults["states.check.output != 'ready'"] = false
			}
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.TotalCount)
	assert.Equal(t, -1, result.BrokeAt)
	assert.False(t, result.WasBroken())
}

func TestLoopExecutor_ExecuteWhile_MaxIterations(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	// Always return true
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-max",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "infinite_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"work"},
			MaxIterations: 5, // Safety limit
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-max")
	callCount := 0

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 5, result.TotalCount)
	assert.Equal(t, 5, callCount)
}

func TestLoopExecutor_ExecuteWhile_ConditionError(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.err = errors.New("invalid expression")
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-error",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "error_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "invalid_expression",
			Body:          []string{"work"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-error")

	_, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "evaluate condition")
}

func TestLoopExecutor_ExecuteWhile_WithBreakCondition(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-break",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "break_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:           workflow.LoopTypeWhile,
			Condition:      "true",
			Body:           []string{"work"},
			MaxIterations:  100,
			BreakCondition: "states.work.exit_code != 0",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-break")
	callCount := 0

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			// After 2 iterations, trigger break
			if callCount >= 2 {
				evaluator.boolResults["states.work.exit_code != 0"] = true
			}
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.WasBroken())
	assert.Equal(t, 2, result.TotalCount)
}

func TestLoopExecutor_ExecuteWhile_StepError(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-step-error",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "error_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"work"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-step-error")
	callCount := 0
	stepErr := errors.New("step failed")

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			if callCount == 3 {
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
	assert.Equal(t, 3, callCount)
	assert.Equal(t, 3, result.TotalCount)
}

func TestLoopExecutor_ExecuteWhile_LoopVariables(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-vars",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "var_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"work"},
			MaxIterations: 3,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-vars")
	recorder := newStepExecutorRecorder()

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		recorder.execute,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			if ec.CurrentLoop != nil {
				ctx.Loop = &interpolation.LoopData{
					Item:   ec.CurrentLoop.Item,
					Index:  ec.CurrentLoop.Index,
					First:  ec.CurrentLoop.First,
					Last:   ec.CurrentLoop.Last,
					Length: ec.CurrentLoop.Length,
				}
			}
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, recorder.executions, 3)

	// Verify while loop variables
	assert.Equal(t, 0, recorder.executions[0].loopData.Index)
	assert.True(t, recorder.executions[0].loopData.First)
	assert.Equal(t, -1, recorder.executions[0].loopData.Length) // Unknown for while

	assert.Equal(t, 1, recorder.executions[1].loopData.Index)
	assert.False(t, recorder.executions[1].loopData.First)

	assert.Equal(t, 2, recorder.executions[2].loopData.Index)
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_FromInput(t *testing.T) {
	// Test US1: while loop max_iterations from {{inputs.limit}}
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true // Condition always true (will hit max)
	resolver := newConfigurableMockResolver()

	// Configure resolver for max_iterations expression
	resolver.results["{{inputs.limit}}"] = "3"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-max",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "while_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterations:     0, // Not used when dynamic expr is set
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-max")
	callCount := 0

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "3"
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.TotalCount)
	assert.Equal(t, 3, callCount)
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_FromEnv(t *testing.T) {
	// Test US1: while loop max_iterations from {{env.MAX_RETRIES}}
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	resolver.results["{{env.MAX_RETRIES}}"] = "5"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-max-env",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "retry_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"retry"},
			MaxIterationsExpr: "{{env.MAX_RETRIES}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-max-env")
	callCount := 0

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Env["MAX_RETRIES"] = "5"
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 5, result.TotalCount)
	assert.Equal(t, 5, callCount)
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_FromStepOutput(t *testing.T) {
	// Test US2: while loop max_iterations from {{states.count.output}}
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	resolver.results["{{states.setup.output}}"] = "4"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-max-state",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "poll_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"poll"},
			MaxIterationsExpr: "{{states.setup.output}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-max-state")
	callCount := 0

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.States["setup"] = interpolation.StepStateData{
				Output:   "4",
				ExitCode: 0,
				Status:   "completed",
			}
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 4, result.TotalCount)
	assert.Equal(t, 4, callCount)
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_Arithmetic(t *testing.T) {
	// Test US3: arithmetic expression in while loop max_iterations
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	// Expression resolves to "2 * 3" = 6
	resolver.results["{{inputs.retries * inputs.multiplier}}"] = "2 * 3"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-max-arithmetic",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "calc_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{inputs.retries * inputs.multiplier}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-max-arithmetic")
	callCount := 0

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["retries"] = "2"
			ctx.Inputs["multiplier"] = "3"
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 6, result.TotalCount)
	assert.Equal(t, 6, callCount)
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_ConditionExitsEarly(t *testing.T) {
	// Test: condition becomes false before hitting dynamic max_iterations
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "100" // High limit

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-early-exit",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "early_exit_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "states.check.output != 'done'",
			Body:              []string{"check"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-early-exit")
	callCount := 0

	// Condition returns true for first 3 calls, then false
	evaluator.boolResults["states.check.output != 'done'"] = true

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			if callCount >= 3 {
				evaluator.boolResults["states.check.output != 'done'"] = false
			}
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "100"
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.TotalCount)
	assert.Equal(t, 3, callCount)
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_ResolverError(t *testing.T) {
	// Test: resolver fails to resolve max_iterations expression
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	// Resolver returns error for undefined variable
	resolver.err = errors.New("undefined variable: inputs.missing")

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-resolver-error",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "error_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{inputs.missing}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-resolver-error")

	_, err := loopExec.ExecuteWhile(
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

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_InvalidValue(t *testing.T) {
	// Test: resolved max_iterations is not a valid integer
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "not_a_number"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-invalid",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "invalid_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-invalid")

	_, err := loopExec.ExecuteWhile(
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

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_ZeroValue(t *testing.T) {
	// Test: resolved max_iterations is zero (invalid, must be >= 1)
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "0"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-zero",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "zero_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-zero")

	_, err := loopExec.ExecuteWhile(
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

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_NegativeValue(t *testing.T) {
	// Test: resolved max_iterations is negative (invalid)
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "-5"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-negative",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "negative_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-negative")

	_, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			t.Error("should not execute when max_iterations is negative")
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "-5"
			return ctx
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 1")
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_ExceedsLimit(t *testing.T) {
	// Test: resolved max_iterations exceeds MaxAllowedIterations (10000)
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "50000"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-exceeds",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "exceeds_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-exceeds")

	_, err := loopExec.ExecuteWhile(
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

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_StaticStillWorks(t *testing.T) {
	// Test backward compatibility: static max_iterations still works for while loops
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-static-max",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "static_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterations:     4,  // Static value
			MaxIterationsExpr: "", // No dynamic expression
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-static-max")
	callCount := 0

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 4, result.TotalCount)
	assert.Equal(t, 4, callCount)
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_BoundaryMin(t *testing.T) {
	// Test boundary: max_iterations = 1 (minimum valid value)
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "1"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-min",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "min_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-min")
	callCount := 0

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "1"
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, 1, callCount)
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_BoundaryMax(t *testing.T) {
	// Test boundary: max_iterations = 10000 (maximum allowed value)
	// Note: we don't actually run 10000 iterations, just verify it's accepted
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "10000"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-max-boundary",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "max_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "states.done.output == 'yes'",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-max-boundary")

	// Condition is false immediately, so loop exits after 0 iterations
	// This test just verifies the max_iterations value is accepted
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "10000"
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Condition was false from start, so no iterations
	assert.Equal(t, 0, result.TotalCount)
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_WithBreakCondition(t *testing.T) {
	// Test: dynamic max_iterations combined with break condition
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true // While condition
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "10"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-with-break",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "break_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{inputs.limit}}",
			BreakCondition:    "states.work.output == 'stop'",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-with-break")
	callCount := 0

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			// Trigger break after 3 iterations
			if callCount >= 3 {
				evaluator.boolResults["states.work.output == 'stop'"] = true
			}
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "10"
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.WasBroken())
	assert.Equal(t, 3, result.TotalCount) // Stopped at break, not max
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_WhitespaceInValue(t *testing.T) {
	// Test: resolved value has whitespace (common with command output)
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	// Simulates command output with trailing newline
	resolver.results["{{states.count.output}}"] = "  7  \n"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-whitespace",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "whitespace_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{states.count.output}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-whitespace")
	callCount := 0

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.States["count"] = interpolation.StepStateData{
				Output:   "  7  \n",
				ExitCode: 0,
				Status:   "completed",
			}
			return ctx
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 7, result.TotalCount)
	assert.Equal(t, 7, callCount)
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_StepError(t *testing.T) {
	// Test: step error during execution with dynamic max_iterations
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "10"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-step-error",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "error_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-step-error")
	callCount := 0
	stepErr := errors.New("step execution failed")

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			if callCount == 3 {
				return "", stepErr
			}
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			ctx := interpolation.NewContext()
			ctx.Inputs["limit"] = "10"
			return ctx
		},
	)

	require.Error(t, err)
	assert.Equal(t, stepErr, err)
	assert.Equal(t, 3, callCount)
	assert.Equal(t, 3, result.TotalCount)
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_ContextCancellation(t *testing.T) {
	// Test: context cancellation with dynamic max_iterations
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "100"

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-while-dynamic-cancel",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "cancel_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:              workflow.LoopTypeWhile,
			Condition:         "true",
			Body:              []string{"work"},
			MaxIterationsExpr: "{{inputs.limit}}",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-dynamic-cancel")

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	result, err := loopExec.ExecuteWhile(
		ctx,
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			if callCount == 3 {
				cancel() // Cancel after 3rd iteration
			}
			return "", nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			intCtx := interpolation.NewContext()
			intCtx.Inputs["limit"] = "100"
			return intCtx
		},
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, result.TotalCount, 100) // Should not complete all iterations
}

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_TableDriven(t *testing.T) {
	// Table-driven tests for various dynamic max_iterations scenarios
	tests := []struct {
		name               string
		maxIterationsExpr  string
		resolveResult      string
		resolveErr         error
		conditionResults   map[string]bool
		expectedIterations int
		wantErr            bool
		errContains        string
	}{
		{
			name:               "valid small limit",
			maxIterationsExpr:  "{{inputs.n}}",
			resolveResult:      "2",
			conditionResults:   map[string]bool{"true": true},
			expectedIterations: 2,
			wantErr:            false,
		},
		{
			name:               "valid medium limit",
			maxIterationsExpr:  "{{env.LIMIT}}",
			resolveResult:      "50",
			conditionResults:   map[string]bool{"true": true},
			expectedIterations: 50,
			wantErr:            false,
		},
		{
			name:              "zero value",
			maxIterationsExpr: "{{inputs.zero}}",
			resolveResult:     "0",
			conditionResults:  map[string]bool{"true": true},
			wantErr:           true,
			errContains:       "at least 1",
		},
		{
			name:              "negative value",
			maxIterationsExpr: "{{inputs.neg}}",
			resolveResult:     "-1",
			conditionResults:  map[string]bool{"true": true},
			wantErr:           true,
			errContains:       "at least 1",
		},
		{
			name:              "exceeds limit",
			maxIterationsExpr: "{{inputs.huge}}",
			resolveResult:     "20000",
			conditionResults:  map[string]bool{"true": true},
			wantErr:           true,
			errContains:       "exceeds",
		},
		{
			name:              "non-numeric",
			maxIterationsExpr: "{{inputs.text}}",
			resolveResult:     "abc",
			conditionResults:  map[string]bool{"true": true},
			wantErr:           true,
		},
		{
			name:              "resolver error",
			maxIterationsExpr: "{{inputs.missing}}",
			resolveErr:        errors.New("variable not found"),
			conditionResults:  map[string]bool{"true": true},
			wantErr:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			evaluator := newMockExpressionEvaluator()
			for k, v := range tt.conditionResults {
				evaluator.boolResults[k] = v
			}
			resolver := newConfigurableMockResolver()

			if tt.resolveErr != nil {
				resolver.err = tt.resolveErr
			} else {
				resolver.results[tt.maxIterationsExpr] = tt.resolveResult
			}

			loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

			wf := &workflow.Workflow{
				Name:  "test-while-table",
				Steps: map[string]*workflow.Step{},
			}

			step := &workflow.Step{
				Name: "table_loop",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:              workflow.LoopTypeWhile,
					Condition:         "true",
					Body:              []string{"work"},
					MaxIterationsExpr: tt.maxIterationsExpr,
				},
			}

			execCtx := workflow.NewExecutionContext("test-id", "test-while-table")
			callCount := 0

			result, err := loopExec.ExecuteWhile(
				context.Background(),
				wf,
				step,
				execCtx,
				func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
					callCount++
					return "", nil
				},
				func(ec *workflow.ExecutionContext) *interpolation.Context {
					return interpolation.NewContext()
				},
			)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedIterations, result.TotalCount)
				assert.Equal(t, tt.expectedIterations, callCount)
			}
		})
	}
}
