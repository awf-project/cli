package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// Mock Implementations for Loop Executor Tests
// =============================================================================

// mockExpressionEvaluator implements ExpressionEvaluator for testing
type mockExpressionEvaluator struct {
	results map[string]bool
	calls   []string
	err     error
}

func newMockExpressionEvaluator() *mockExpressionEvaluator {
	return &mockExpressionEvaluator{
		results: make(map[string]bool),
		calls:   make([]string, 0),
	}
}

func (m *mockExpressionEvaluator) Evaluate(expr string, ctx *interpolation.Context) (bool, error) {
	m.calls = append(m.calls, expr)
	if m.err != nil {
		return false, m.err
	}
	if result, ok := m.results[expr]; ok {
		return result, nil
	}
	return false, nil
}

// configurableMockResolver implements interpolation.Resolver with configurable results
type configurableMockResolver struct {
	results map[string]string
	calls   []string
	err     error
}

func newConfigurableMockResolver() *configurableMockResolver {
	return &configurableMockResolver{
		results: make(map[string]string),
		calls:   make([]string, 0),
	}
}

func (m *configurableMockResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	m.calls = append(m.calls, template)
	if m.err != nil {
		return "", m.err
	}
	if result, ok := m.results[template]; ok {
		return result, nil
	}
	// Default: return template unchanged
	return template, nil
}

// stepExecutorRecorder records step executions for verification
// F048: Updated to support new StepExecutorFunc signature
type stepExecutorRecorder struct {
	executions  []stepExecution
	results     map[string]error
	transitions map[string]string // F048: Map of stepName -> nextStep for transition testing
}

type stepExecution struct {
	stepName string
	loopData *interpolation.LoopData
}

func newStepExecutorRecorder() *stepExecutorRecorder {
	return &stepExecutorRecorder{
		executions:  make([]stepExecution, 0),
		results:     make(map[string]error),
		transitions: make(map[string]string),
	}
}

// F048: Updated to return (nextStep, error)
func (r *stepExecutorRecorder) execute(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
	exec := stepExecution{stepName: stepName}
	if intCtx.Loop != nil {
		exec.loopData = &interpolation.LoopData{
			Item:   intCtx.Loop.Item,
			Index:  intCtx.Loop.Index,
			First:  intCtx.Loop.First,
			Last:   intCtx.Loop.Last,
			Length: intCtx.Loop.Length,
		}
	}
	r.executions = append(r.executions, exec)

	if err, ok := r.results[stepName]; ok {
		return "", err
	}

	// F048: Return transition if configured for this step
	if nextStep, ok := r.transitions[stepName]; ok {
		return nextStep, nil
	}

	return "", nil
}

// =============================================================================
// LoopExecutor Unit Tests
// =============================================================================

func TestNewLoopExecutor(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	assert.NotNil(t, executor)
}

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
	evaluator.results["states.process.output == 'stop'"] = true
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

// =============================================================================
// While Loop Tests
// =============================================================================

func TestLoopExecutor_ExecuteWhile_Simple(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	// Condition returns true for first 3 iterations, then false
	callCount := 0
	evaluator.results["states.check.output != 'ready'"] = true

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
				evaluator.results["states.check.output != 'ready'"] = false
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
				evaluator.results["states.work.exit_code != 0"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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

// =============================================================================
// ParseItems Tests
// =============================================================================

func TestLoopExecutor_ParseItems_JSONArray(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	tests := []struct {
		name     string
		input    string
		expected []any
	}{
		{
			name:     "string array",
			input:    `["a", "b", "c"]`,
			expected: []any{"a", "b", "c"},
		},
		{
			name:     "integer array",
			input:    `[1, 2, 3]`,
			expected: []any{float64(1), float64(2), float64(3)}, // JSON numbers are float64
		},
		{
			name:     "mixed array",
			input:    `["file.txt", 42, true]`,
			expected: []any{"file.txt", float64(42), true},
		},
		{
			name:     "empty array",
			input:    `[]`,
			expected: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := loopExec.ParseItems(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, items)
		})
	}
}

func TestLoopExecutor_ParseItems_CommaSeparated(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	input := "a, b, c"
	items, err := loopExec.ParseItems(input)

	require.NoError(t, err)
	assert.Len(t, items, 3)
	assert.Equal(t, "a", items[0])
	assert.Equal(t, "b", items[1])
	assert.Equal(t, "c", items[2])
}

// =============================================================================
// Integration with ExecutionService
// =============================================================================

func TestExecutionService_Run_ForEachStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "foreach-workflow",
		Initial: "process_files",
		Steps: map[string]*workflow.Step{
			"process_files": {
				Name: "process_files",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a.txt", "b.txt"]`,
					Body:          []string{"echo_file"},
					MaxIterations: 10,
					OnComplete:    "done",
				},
			},
			"echo_file": {
				Name:      "echo_file",
				Type:      workflow.StepTypeCommand,
				Command:   "echo processing",
				OnSuccess: "", // No explicit transition - continue loop body
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("foreach-workflow", wf).
		Build()

	ctx, err := execSvc.Run(context.Background(), "foreach-workflow", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

// TestExecutionService_Run_WhileStep tests while loop integration with ExecutionService.
// Note: This test uses NewExecutionServiceWithEvaluator with a custom counterExpressionEvaluator
// to control loop iterations. ServiceTestHarness doesn't support custom evaluators,
// so this test retains the manual setup pattern.
func TestExecutionService_Run_WhileStep(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["while-workflow"] = &workflow.Workflow{
		Name:    "while-workflow",
		Initial: "poll",
		Steps: map[string]*workflow.Step{
			"poll": {
				Name: "poll",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeWhile,
					Condition:     "states.check.output != 'ready'",
					Body:          []string{"check"},
					MaxIterations: 5,
					OnComplete:    "done",
				},
			},
			"check": {
				Name:      "check",
				Type:      workflow.StepTypeCommand,
				Command:   "check_status",
				OnSuccess: "", // No explicit transition - continue loop body
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	executor := newMockExecutor()
	store := newMockStateStore()
	// Use a counter-based evaluator that returns true for first 3 calls
	evaluator := newCounterExpressionEvaluator(3)

	wfSvc := application.NewWorkflowService(repo, store, executor, &mockLogger{})
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, executor, newMockParallelExecutor(), store,
		&mockLogger{}, newMockResolver(), nil, evaluator,
	)

	ctx, err := execSvc.Run(context.Background(), "while-workflow", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

// counterExpressionEvaluator returns true for first N calls, then false
type counterExpressionEvaluator struct {
	maxTrue int
	count   int
}

func newCounterExpressionEvaluator(maxTrue int) *counterExpressionEvaluator {
	return &counterExpressionEvaluator{maxTrue: maxTrue}
}

func (e *counterExpressionEvaluator) Evaluate(expr string, ctx *interpolation.Context) (bool, error) {
	e.count++
	return e.count <= e.maxTrue, nil
}

// =============================================================================
// LoopResult Tests
// =============================================================================

func TestLoopResult_Duration(t *testing.T) {
	result := workflow.NewLoopResult()
	result.StartedAt = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	result.CompletedAt = result.StartedAt.Add(5 * time.Second)

	duration := result.Duration()
	assert.Equal(t, 5*time.Second, duration)
}

func TestLoopResult_AllSucceeded(t *testing.T) {
	result := workflow.NewLoopResult()

	// Empty should be false
	assert.False(t, result.AllSucceeded())

	// All success
	result.Iterations = []workflow.IterationResult{
		{Error: nil},
		{Error: nil},
	}
	assert.True(t, result.AllSucceeded())

	// One failure
	result.Iterations = []workflow.IterationResult{
		{Error: nil},
		{Error: errors.New("failed")},
	}
	assert.False(t, result.AllSucceeded())
}

// =============================================================================
// F043: Nested Loop Context Push/Pop Tests
// =============================================================================

func TestLoopExecutor_PushPopContext_Basic(t *testing.T) {
	logger := &mockLogger{}
	exec := application.NewLoopExecutor(logger, nil, nil)
	execCtx := workflow.NewExecutionContext("test-id", "test-wf")

	// Initial state: no loop context
	assert.Nil(t, execCtx.CurrentLoop)

	// Push outer loop
	exec.PushLoopContext(execCtx, "outer-item", 0, true, true, 1)
	require.NotNil(t, execCtx.CurrentLoop)
	assert.Equal(t, "outer-item", execCtx.CurrentLoop.Item)
	assert.Equal(t, 0, execCtx.CurrentLoop.Index)
	assert.True(t, execCtx.CurrentLoop.First)
	assert.True(t, execCtx.CurrentLoop.Last)
	assert.Equal(t, 1, execCtx.CurrentLoop.Length)
	assert.Nil(t, execCtx.CurrentLoop.Parent)

	// Pop outer
	popped := exec.PopLoopContext(execCtx)
	assert.NotNil(t, popped)
	assert.Equal(t, "outer-item", popped.Item)
	assert.Nil(t, execCtx.CurrentLoop)
}

func TestLoopExecutor_PushPopContext_Nested(t *testing.T) {
	logger := &mockLogger{}
	exec := application.NewLoopExecutor(logger, nil, nil)
	execCtx := workflow.NewExecutionContext("test-id", "test-wf")

	// Push outer loop
	exec.PushLoopContext(execCtx, "outer", 0, true, true, 1)
	assert.Equal(t, "outer", execCtx.CurrentLoop.Item)
	assert.Nil(t, execCtx.CurrentLoop.Parent)

	// Push inner loop
	exec.PushLoopContext(execCtx, "inner", 0, true, true, 1)
	assert.Equal(t, "inner", execCtx.CurrentLoop.Item)
	require.NotNil(t, execCtx.CurrentLoop.Parent)
	assert.Equal(t, "outer", execCtx.CurrentLoop.Parent.Item)

	// Pop inner - should restore outer
	exec.PopLoopContext(execCtx)
	require.NotNil(t, execCtx.CurrentLoop)
	assert.Equal(t, "outer", execCtx.CurrentLoop.Item)
	assert.Nil(t, execCtx.CurrentLoop.Parent)

	// Pop outer - should be nil
	exec.PopLoopContext(execCtx)
	assert.Nil(t, execCtx.CurrentLoop)
}

func TestLoopExecutor_PushPopContext_TripleNesting(t *testing.T) {
	logger := &mockLogger{}
	exec := application.NewLoopExecutor(logger, nil, nil)
	execCtx := workflow.NewExecutionContext("test-id", "test-wf")

	// Push three levels
	exec.PushLoopContext(execCtx, "L1", 0, true, true, 1)
	exec.PushLoopContext(execCtx, "L2", 1, false, false, 3)
	exec.PushLoopContext(execCtx, "L3", 2, false, true, 5)

	// Verify chain
	require.NotNil(t, execCtx.CurrentLoop)
	assert.Equal(t, "L3", execCtx.CurrentLoop.Item)
	assert.Equal(t, 2, execCtx.CurrentLoop.Index)
	assert.Equal(t, 5, execCtx.CurrentLoop.Length)

	require.NotNil(t, execCtx.CurrentLoop.Parent)
	assert.Equal(t, "L2", execCtx.CurrentLoop.Parent.Item)
	assert.Equal(t, 1, execCtx.CurrentLoop.Parent.Index)
	assert.Equal(t, 3, execCtx.CurrentLoop.Parent.Length)

	require.NotNil(t, execCtx.CurrentLoop.Parent.Parent)
	assert.Equal(t, "L1", execCtx.CurrentLoop.Parent.Parent.Item)
	assert.Nil(t, execCtx.CurrentLoop.Parent.Parent.Parent)

	// Pop all three
	exec.PopLoopContext(execCtx)
	assert.Equal(t, "L2", execCtx.CurrentLoop.Item)

	exec.PopLoopContext(execCtx)
	assert.Equal(t, "L1", execCtx.CurrentLoop.Item)

	exec.PopLoopContext(execCtx)
	assert.Nil(t, execCtx.CurrentLoop)
}

func TestLoopExecutor_PopLoopContext_EmptyStack(t *testing.T) {
	logger := &mockLogger{}
	exec := application.NewLoopExecutor(logger, nil, nil)
	execCtx := workflow.NewExecutionContext("test-id", "test-wf")

	// Pop from empty - should be safe (nil)
	popped := exec.PopLoopContext(execCtx)
	assert.Nil(t, popped)
	assert.Nil(t, execCtx.CurrentLoop)
}

func TestLoopExecutor_PushPopContext_PreservesAllFields(t *testing.T) {
	logger := &mockLogger{}
	exec := application.NewLoopExecutor(logger, nil, nil)
	execCtx := workflow.NewExecutionContext("test-id", "test-wf")

	tests := []struct {
		name   string
		item   any
		index  int
		first  bool
		last   bool
		length int
	}{
		{
			name:   "string item at start",
			item:   "first-item",
			index:  0,
			first:  true,
			last:   false,
			length: 5,
		},
		{
			name:   "string item at middle",
			item:   "middle-item",
			index:  2,
			first:  false,
			last:   false,
			length: 5,
		},
		{
			name:   "string item at end",
			item:   "last-item",
			index:  4,
			first:  false,
			last:   true,
			length: 5,
		},
		{
			name:   "int item",
			item:   42,
			index:  0,
			first:  true,
			last:   true,
			length: 1,
		},
		{
			name:   "map item",
			item:   map[string]any{"key": "value", "num": 123},
			index:  1,
			first:  false,
			last:   false,
			length: 3,
		},
		{
			name:   "nil item (while loop)",
			item:   nil,
			index:  5,
			first:  false,
			last:   false,
			length: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset context
			execCtx.CurrentLoop = nil

			exec.PushLoopContext(execCtx, tt.item, tt.index, tt.first, tt.last, tt.length)

			require.NotNil(t, execCtx.CurrentLoop)
			assert.Equal(t, tt.item, execCtx.CurrentLoop.Item)
			assert.Equal(t, tt.index, execCtx.CurrentLoop.Index)
			assert.Equal(t, tt.first, execCtx.CurrentLoop.First)
			assert.Equal(t, tt.last, execCtx.CurrentLoop.Last)
			assert.Equal(t, tt.length, execCtx.CurrentLoop.Length)
			assert.Nil(t, execCtx.CurrentLoop.Parent)

			popped := exec.PopLoopContext(execCtx)
			assert.Equal(t, tt.item, popped.Item)
			assert.Nil(t, execCtx.CurrentLoop)
		})
	}
}

func TestLoopExecutor_PushPopContext_MixedLoopTypes(t *testing.T) {
	logger := &mockLogger{}
	exec := application.NewLoopExecutor(logger, nil, nil)
	execCtx := workflow.NewExecutionContext("test-id", "test-wf")

	// Push a for_each loop (has item and known length)
	exec.PushLoopContext(execCtx, "outer-item", 0, true, false, 3)
	assert.Equal(t, "outer-item", execCtx.CurrentLoop.Item)
	assert.Equal(t, 3, execCtx.CurrentLoop.Length)

	// Push a while loop inside (no item, unknown length)
	exec.PushLoopContext(execCtx, nil, 0, true, false, -1)
	assert.Nil(t, execCtx.CurrentLoop.Item)
	assert.Equal(t, -1, execCtx.CurrentLoop.Length)
	// Parent should still have for_each data
	require.NotNil(t, execCtx.CurrentLoop.Parent)
	assert.Equal(t, "outer-item", execCtx.CurrentLoop.Parent.Item)
	assert.Equal(t, 3, execCtx.CurrentLoop.Parent.Length)

	// Pop while loop
	exec.PopLoopContext(execCtx)
	assert.Equal(t, "outer-item", execCtx.CurrentLoop.Item)
	assert.Equal(t, 3, execCtx.CurrentLoop.Length)

	// Pop for_each loop
	exec.PopLoopContext(execCtx)
	assert.Nil(t, execCtx.CurrentLoop)
}

func TestLoopExecutor_PushPopContext_DeepNesting(t *testing.T) {
	logger := &mockLogger{}
	exec := application.NewLoopExecutor(logger, nil, nil)
	execCtx := workflow.NewExecutionContext("test-id", "test-wf")

	// Push 10 levels of nesting (arbitrary deep nesting)
	for i := 0; i < 10; i++ {
		exec.PushLoopContext(execCtx, fmt.Sprintf("level-%d", i), i, i == 0, false, 10)
	}

	// Verify depth - traverse parent chain
	current := execCtx.CurrentLoop
	depth := 0
	for current != nil {
		depth++
		current = current.Parent
	}
	assert.Equal(t, 10, depth)

	// Verify innermost item
	assert.Equal(t, "level-9", execCtx.CurrentLoop.Item)
	assert.Equal(t, 9, execCtx.CurrentLoop.Index)

	// Pop all levels and verify order
	for i := 9; i >= 0; i-- {
		require.NotNil(t, execCtx.CurrentLoop)
		assert.Equal(t, fmt.Sprintf("level-%d", i), execCtx.CurrentLoop.Item)
		exec.PopLoopContext(execCtx)
	}
	assert.Nil(t, execCtx.CurrentLoop)
}

func TestLoopExecutor_PushPopContext_MultiplePopOnEmpty(t *testing.T) {
	logger := &mockLogger{}
	exec := application.NewLoopExecutor(logger, nil, nil)
	execCtx := workflow.NewExecutionContext("test-id", "test-wf")

	// Multiple pops on empty should be safe
	for i := 0; i < 5; i++ {
		popped := exec.PopLoopContext(execCtx)
		assert.Nil(t, popped)
		assert.Nil(t, execCtx.CurrentLoop)
	}
}

func TestLoopExecutor_PushPopContext_ParentChainPreserved(t *testing.T) {
	logger := &mockLogger{}
	exec := application.NewLoopExecutor(logger, nil, nil)
	execCtx := workflow.NewExecutionContext("test-id", "test-wf")

	// Push outer loop
	exec.PushLoopContext(execCtx, "A", 0, true, false, 2)
	outer := execCtx.CurrentLoop

	// Push inner loop
	exec.PushLoopContext(execCtx, "1", 0, true, false, 3)
	inner := execCtx.CurrentLoop

	// Verify parent pointer is same instance as outer
	assert.Same(t, outer, inner.Parent)

	// Pop inner
	exec.PopLoopContext(execCtx)

	// Outer context should be unchanged
	assert.Equal(t, "A", execCtx.CurrentLoop.Item)
	assert.Equal(t, 0, execCtx.CurrentLoop.Index)
	assert.True(t, execCtx.CurrentLoop.First)
	assert.False(t, execCtx.CurrentLoop.Last)
	assert.Equal(t, 2, execCtx.CurrentLoop.Length)
}

// =============================================================================
// F037: ResolveMaxIterations Tests
// =============================================================================

func TestLoopExecutor_ResolveMaxIterations_SimpleInteger(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Configure resolver to return the literal value
	resolver.results["{{inputs.limit}}"] = "5"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "5"

	result, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 5, result)
}

func TestLoopExecutor_ResolveMaxIterations_FromEnvVariable(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{env.LOOP_LIMIT}}"] = "10"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Env["LOOP_LIMIT"] = "10"

	result, err := exec.ResolveMaxIterations("{{env.LOOP_LIMIT}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 10, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticAddition(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "2 + 3"
	resolver.results["{{inputs.a + inputs.b}}"] = "2 + 3"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["a"] = "2"
	ctx.Inputs["b"] = "3"

	result, err := exec.ResolveMaxIterations("{{inputs.a + inputs.b}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 5, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticMultiplication(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression: pages * retries_per_page
	resolver.results["{{inputs.pages * inputs.retries_per_page}}"] = "3 * 2"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["pages"] = "3"
	ctx.Inputs["retries_per_page"] = "2"

	result, err := exec.ResolveMaxIterations("{{inputs.pages * inputs.retries_per_page}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 6, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticSubtraction(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "10 - 3"
	resolver.results["{{inputs.total - inputs.offset}}"] = "10 - 3"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["total"] = "10"
	ctx.Inputs["offset"] = "3"

	result, err := exec.ResolveMaxIterations("{{inputs.total - inputs.offset}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 7, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticDivision(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "20 / 4" (exact integer division)
	resolver.results["{{inputs.total / inputs.batch_size}}"] = "20 / 4"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["total"] = "20"
	ctx.Inputs["batch_size"] = "4"

	result, err := exec.ResolveMaxIterations("{{inputs.total / inputs.batch_size}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 5, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticModulo(t *testing.T) {
	// NOTE: FR-006 only requires +, -, *, / operators.
	// Modulo (%) is NOT in the spec, but expr-lang supports it.
	// The implementation currently doesn't recognize % as an arithmetic operator
	// because it's not in strings.ContainsAny("+-*/").
	// This test verifies that % expressions are NOT currently supported.
	// If modulo support is added later, update this test.
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "17 % 5" - but % is not recognized as operator
	resolver.results["{{inputs.total % inputs.divisor}}"] = "17 % 5"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["total"] = "17"
	ctx.Inputs["divisor"] = "5"

	_, err := exec.ResolveMaxIterations("{{inputs.total % inputs.divisor}}", ctx)

	// Modulo is not supported per FR-006, should error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticComplexExpression(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "(2 + 3) * 4"
	resolver.results["{{(inputs.a + inputs.b) * inputs.c}}"] = "(2 + 3) * 4"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["a"] = "2"
	ctx.Inputs["b"] = "3"
	ctx.Inputs["c"] = "4"

	result, err := exec.ResolveMaxIterations("{{(inputs.a + inputs.b) * inputs.c}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 20, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticDivisionByZero(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "10 / 0"
	resolver.results["{{inputs.total / inputs.divisor}}"] = "10 / 0"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["total"] = "10"
	ctx.Inputs["divisor"] = "0"

	_, err := exec.ResolveMaxIterations("{{inputs.total / inputs.divisor}}", ctx)

	// Division by zero should return error (expr-lang behavior may vary)
	// If it returns infinity or NaN, it will fail the integer conversion
	require.Error(t, err)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticNonWholeNumber(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "7 / 2" = 3.5 (non-whole number)
	resolver.results["{{inputs.a / inputs.b}}"] = "7 / 2"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["a"] = "7"
	ctx.Inputs["b"] = "2"

	_, err := exec.ResolveMaxIterations("{{inputs.a / inputs.b}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be an integer")
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticInvalidSyntax(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to truly invalid syntax (unclosed parenthesis)
	resolver.results["{{inputs.expr}}"] = "2 + (3 * 4"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["expr"] = "2 + (3 * 4"

	_, err := exec.ResolveMaxIterations("{{inputs.expr}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticNegativeResult(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "3 - 10" = -7
	resolver.results["{{inputs.a - inputs.b}}"] = "3 - 10"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["a"] = "3"
	ctx.Inputs["b"] = "10"

	_, err := exec.ResolveMaxIterations("{{inputs.a - inputs.b}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be at least 1")
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticExceedsMax(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "5000 * 3" = 15000 (exceeds max 10000)
	resolver.results["{{inputs.a * inputs.b}}"] = "5000 * 3"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["a"] = "5000"
	ctx.Inputs["b"] = "3"

	_, err := exec.ResolveMaxIterations("{{inputs.a * inputs.b}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestLoopExecutor_ResolveMaxIterations_BoundaryMin(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "1"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "1"

	result, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result)
}

func TestLoopExecutor_ResolveMaxIterations_BoundaryMax(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "10000"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "10000"

	result, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 10000, result)
}

func TestLoopExecutor_ResolveMaxIterations_ErrorZero(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "0"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "0"

	_, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be at least 1")
}

func TestLoopExecutor_ResolveMaxIterations_ErrorNegative(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "-5"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "-5"

	_, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be at least 1")
}

func TestLoopExecutor_ResolveMaxIterations_ErrorExceedsMax(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "10001"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "10001"

	_, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestLoopExecutor_ResolveMaxIterations_ErrorNonInteger(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "not_a_number"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "not_a_number"

	_, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestLoopExecutor_ResolveMaxIterations_ErrorFloat(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "3.14"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "3.14"

	_, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.Error(t, err)
	// Could be "invalid syntax" or custom message about integers
}

func TestLoopExecutor_ResolveMaxIterations_ErrorMissingVariable(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Resolver returns error for missing variable
	resolver.err = errors.New("undefined variable: inputs.undefined_var")

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	_, err := exec.ResolveMaxIterations("{{inputs.undefined_var}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "undefined variable")
}

func TestLoopExecutor_ResolveMaxIterations_ErrorEmptyExpression(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	_, err := exec.ResolveMaxIterations("", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestLoopExecutor_ResolveMaxIterations_FromStepOutput(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{states.count.output}}"] = "7"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.States["count"] = interpolation.StepStateData{
		Output:   "7",
		ExitCode: 0,
		Status:   "completed",
	}

	result, err := exec.ResolveMaxIterations("{{states.count.output}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 7, result)
}

func TestLoopExecutor_ResolveMaxIterations_TrimWhitespace(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Resolved value has whitespace (e.g., from command output)
	resolver.results["{{inputs.limit}}"] = "  42  \n"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "  42  \n"

	result, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

// =============================================================================
// F037 T011: ExecuteForEach with Dynamic MaxIterationsExpr Tests
// =============================================================================

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

// =============================================================================
// F037 T012: ExecuteWhile with Dynamic MaxIterationsExpr Tests
// =============================================================================

func TestLoopExecutor_ExecuteWhile_DynamicMaxIterations_FromInput(t *testing.T) {
	// Test US1: while loop max_iterations from {{inputs.limit}}
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["true"] = true // Condition always true (will hit max)
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["states.check.output != 'done'"] = true

	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
			callCount++
			if callCount >= 3 {
				evaluator.results["states.check.output != 'done'"] = false
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true // While condition
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
				evaluator.results["states.work.output == 'stop'"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
				evaluator.results[k] = v
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

func TestLoopExecutor_ResolveMaxIterations_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		resolveResult string
		resolveErr    error
		wantValue     int
		wantErr       bool
		errContains   string
	}{
		{
			name:          "valid integer from input",
			expr:          "{{inputs.count}}",
			resolveResult: "5",
			wantValue:     5,
			wantErr:       false,
		},
		{
			name:          "valid min boundary",
			expr:          "{{inputs.min}}",
			resolveResult: "1",
			wantValue:     1,
			wantErr:       false,
		},
		{
			name:          "valid max boundary",
			expr:          "{{inputs.max}}",
			resolveResult: "10000",
			wantValue:     10000,
			wantErr:       false,
		},
		{
			name:          "large valid value",
			expr:          "{{inputs.large}}",
			resolveResult: "9999",
			wantValue:     9999,
			wantErr:       false,
		},
		{
			name:          "zero is invalid",
			expr:          "{{inputs.zero}}",
			resolveResult: "0",
			wantErr:       true,
			errContains:   "at least 1",
		},
		{
			name:          "negative is invalid",
			expr:          "{{inputs.neg}}",
			resolveResult: "-10",
			wantErr:       true,
			errContains:   "at least 1",
		},
		{
			name:          "exceeds max limit",
			expr:          "{{inputs.huge}}",
			resolveResult: "100000",
			wantErr:       true,
			errContains:   "exceeds",
		},
		{
			name:          "non-numeric string",
			expr:          "{{inputs.text}}",
			resolveResult: "hello",
			wantErr:       true,
			errContains:   "invalid",
		},
		{
			name:          "float value",
			expr:          "{{inputs.float}}",
			resolveResult: "5.5",
			wantErr:       true,
		},
		{
			name:       "resolver error",
			expr:       "{{inputs.missing}}",
			resolveErr: errors.New("variable not found"),
			wantErr:    true,
		},
		{
			name:        "empty expression",
			expr:        "",
			wantErr:     true,
			errContains: "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			evaluator := newMockExpressionEvaluator()
			resolver := newConfigurableMockResolver()

			if tt.resolveErr != nil {
				resolver.err = tt.resolveErr
			} else {
				resolver.results[tt.expr] = tt.resolveResult
			}

			exec := application.NewLoopExecutor(logger, evaluator, resolver)
			ctx := interpolation.NewContext()

			result, err := exec.ResolveMaxIterations(tt.expr, ctx)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, result)
			}
		})
	}
}

// =============================================================================
// Component T001: StepExecutorFunc Type Signature Update (F048)
// =============================================================================

// =============================================================================
// Component T001: StepExecutorFunc Type Signature Update
// Feature: F048 - While Loop Transitions Support
// =============================================================================

// TestStepExecutorFunc_TypeSignature_ReturnsNextStepAndError verifies the
// StepExecutorFunc type signature includes nextStep return value.
// This is the foundational change required for F048 transition support.
func TestStepExecutorFunc_TypeSignature_ReturnsNextStepAndError(t *testing.T) {
	// Arrange: Create a step executor that returns nextStep
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// Return a transition to another step
		return "target_step", nil
	}

	// Act: Execute the function
	ctx := context.Background()
	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "test_step", intCtx)

	// Assert: Verify both return values
	assert.NoError(t, err)
	assert.Equal(t, "target_step", nextStep, "should return nextStep value")
}

// TestStepExecutorFunc_NoTransition_ReturnsEmptyString tests that when no
// transition occurs, the executor returns empty string for nextStep.
func TestStepExecutorFunc_NoTransition_ReturnsEmptyString(t *testing.T) {
	// Arrange: Create executor with no transition
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// No transition - return empty nextStep
		return "", nil
	}

	// Act
	ctx := context.Background()
	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "test_step", intCtx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "", nextStep, "should return empty string when no transition")
}

// TestStepExecutorFunc_ErrorWithoutTransition tests error handling when
// step execution fails without a transition.
func TestStepExecutorFunc_ErrorWithoutTransition(t *testing.T) {
	// Arrange: Create executor that fails
	expectedErr := errors.New("step execution failed")
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// Error case - return empty nextStep with error
		return "", expectedErr
	}

	// Act
	ctx := context.Background()
	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "test_step", intCtx)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, "", nextStep, "should return empty nextStep on error")
}

// TestStepExecutorFunc_ErrorWithTransition tests edge case where both
// error and nextStep are returned (error takes precedence).
func TestStepExecutorFunc_ErrorWithTransition(t *testing.T) {
	// Arrange: Executor returns both error and nextStep
	expectedErr := errors.New("step failed but has on_failure transition")
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// On-failure transition case
		return "error_handler_step", expectedErr
	}

	// Act
	ctx := context.Background()
	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "test_step", intCtx)

	// Assert: Both values should be returned
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, "error_handler_step", nextStep, "should return nextStep even on error")
}

// =============================================================================
// ExecuteForEach Integration with New StepExecutorFunc Signature
// =============================================================================

// TestExecuteForEach_StepExecutorReturnsNextStep_CurrentlyIgnored verifies
// that ExecuteForEach receives nextStep from stepExecutor but currently
// ignores it (stub implementation). This test will FAIL in RED phase and
// PASS after T003 implements transition logic.
func TestExecuteForEach_StepExecutorReturnsNextStep_CurrentlyIgnored(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-transition",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["a", "b"]`,
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 100,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-transition")

	// Create step executor that returns nextStep from step1 to step3 (skip step2)
	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// Step1 transitions to step3 (should skip step2)
		if stepName == "step1" {
			return "step3", nil
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify transition logic: step1 transitions to step3, skipping step2
	t.Log("Current execution order:", executionOrder)
	assert.Equal(t, []string{"step1", "step3", "step1", "step3"}, executionOrder,
		"step2 skipped due to step1 -> step3 transition")
}

// TestExecuteForEach_MultipleStepsReturnTransitions verifies behavior when
// multiple steps in body return different nextStep values.
func TestExecuteForEach_MultipleStepsReturnTransitions(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-multi-transition",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"step4": {Name: "step4", Type: workflow.StepTypeCommand, Command: "echo 4"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["item1"]`,
			Body:          []string{"step1", "step2", "step3", "step4"},
			MaxIterations: 100,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-multi-transition")

	transitions := map[string]string{
		"step1": "step3", // step1 -> step3 (skip step2)
		"step3": "step4", // step3 -> step4 (no skip)
	}

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		if nextStep, ok := transitions[stepName]; ok {
			return nextStep, nil
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify transition logic: step2 skipped due to step1 -> step3 transition
	t.Log("Current execution order:", executionOrder)
	assert.Equal(t, []string{"step1", "step3", "step4"}, executionOrder,
		"step2 skipped due to step1 -> step3 transition")
}

// =============================================================================
// ExecuteWhile Integration with New StepExecutorFunc Signature
// =============================================================================

// TestExecuteWhile_StepExecutorReturnsNextStep_CurrentlyIgnored verifies
// that ExecuteWhile receives nextStep from stepExecutor but currently
// ignores it. This is the primary use case from F048 spec.
func TestExecuteWhile_StepExecutorReturnsNextStep_CurrentlyIgnored(t *testing.T) {
	// Arrange: Recreate spec scenario
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	// Condition true for first iteration, false for second
	evaluator.results["loop.index < 1"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-while-transition",
		Steps: map[string]*workflow.Step{
			"run_tests_green":     {Name: "run_tests_green", Type: workflow.StepTypeCommand},
			"check_tests_passed":  {Name: "check_tests_passed", Type: workflow.StepTypeCommand},
			"prepare_impl_prompt": {Name: "prepare_impl_prompt", Type: workflow.StepTypeCommand},
			"implement_item":      {Name: "implement_item", Type: workflow.StepTypeCommand},
			"run_fmt":             {Name: "run_fmt", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "green_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "loop.index < 1",
			Body:          []string{"run_tests_green", "check_tests_passed", "prepare_impl_prompt", "implement_item", "run_fmt"},
			MaxIterations: 2,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-transition")

	// Executor simulates: check_tests_passed returns transition to run_fmt (skip 2 steps)
	executionOrder := []string{}
	callCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		callCount++
		// GREEN phase: After first iteration (3 steps), make condition false
		// RED phase would be 5 steps
		if callCount >= 3 {
			evaluator.results["loop.index < 1"] = false
		}
		// check_tests_passed should transition to run_fmt (skip prepare_impl_prompt, implement_item)
		if stepName == "check_tests_passed" {
			return "run_fmt", nil
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify transition logic: prepare_impl_prompt and implement_item skipped due to transition
	t.Log("Current execution order:", executionOrder)
	assert.Equal(t, []string{"run_tests_green", "check_tests_passed", "run_fmt"}, executionOrder,
		"prepare_impl_prompt and implement_item skipped due to transition")
}

// TestExecuteWhile_TransitionOutsideLoopBody verifies that when a step
// returns nextStep pointing outside the loop body, the behavior is defined.
// Current stub: ignores it. Expected after T003: breaks loop early.
func TestExecuteWhile_TransitionOutsideLoopBody(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	// Condition always true (would loop forever without transition)
	evaluator.results["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-while-exit-transition",
		Steps: map[string]*workflow.Step{
			"step1":       {Name: "step1", Type: workflow.StepTypeCommand},
			"step2":       {Name: "step2", Type: workflow.StepTypeCommand},
			"step3":       {Name: "step3", Type: workflow.StepTypeCommand},
			"exit_target": {Name: "exit_target", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-exit-transition")

	iterationCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// step2 transitions to exit_target (outside loop body)
		if stepName == "step2" {
			return "exit_target", nil
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			iterationCount++
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// RED phase (stub behavior): Loops until max_iterations (10)
	// assert.Equal(t, 10, result.TotalCount, "Stub ignores early exit transition")

	// GREEN phase (T005+T006 implemented): Early exit logic works
	assert.Equal(t, 1, result.TotalCount, "Should exit loop on first iteration when step2 transitions outside")
}

// TestExecuteWhile_ErrorPropagationWithNextStep verifies that errors are
// properly propagated even when nextStep is returned (on_failure transition case).
func TestExecuteWhile_ErrorPropagationWithNextStep(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-while-error-transition",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand},
			"error_handler": {Name: "error_handler", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2"},
			MaxIterations: 5,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-while-error-transition")

	expectedErr := errors.New("step1 failed")
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// step1 fails with on_failure transition
		if stepName == "step1" {
			return "error_handler", expectedErr
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert: Error should be propagated, loop should stop
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalCount, "Loop should stop on first error")
}

// =============================================================================
// Edge Cases and Boundary Conditions
// =============================================================================

// TestStepExecutorFunc_ContextCancellation tests that context cancellation
// is properly handled with the new signature.
func TestStepExecutorFunc_ContextCancellation(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// Check context
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "next_step", nil
	}

	// Act
	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "test_step", intCtx)

	// Assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, "", nextStep, "Should return empty nextStep on cancellation")
}

// TestStepExecutorFunc_NilInterpolationContext tests graceful handling
// of nil interpolation context.
func TestStepExecutorFunc_NilInterpolationContext(t *testing.T) {
	// Arrange
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// Verify nil is handled (executor should validate)
		if intCtx == nil {
			return "", errors.New("interpolation context is nil")
		}
		return "", nil
	}

	// Act
	ctx := context.Background()
	nextStep, err := stepExecutor(ctx, "test_step", nil)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
	assert.Equal(t, "", nextStep)
}

// TestStepExecutorFunc_EmptyStepName tests behavior with empty step name.
func TestStepExecutorFunc_EmptyStepName(t *testing.T) {
	// Arrange
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		if stepName == "" {
			return "", errors.New("step name cannot be empty")
		}
		return "", nil
	}

	// Act
	ctx := context.Background()
	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "", intCtx)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
	assert.Equal(t, "", nextStep)
}

// TestExecuteForEach_StepExecutorReturnsNextStepToItself tests self-transition
// (retry pattern) - should this continue or be treated specially?
func TestExecuteForEach_StepExecutorReturnsNextStepToItself(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-self-transition",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["item1"]`,
			Body:          []string{"step1", "step2"},
			MaxIterations: 100,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-self-transition")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// step1 transitions to itself (retry pattern)
		if stepName == "step1" && len(executionOrder) == 1 {
			return "step1", nil
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify self-transition: step1 executes twice due to self-transition
	t.Log("Current execution order:", executionOrder)
	assert.Equal(t, []string{"step1", "step1", "step2"}, executionOrder,
		"step1 executes twice due to self-transition")
}
