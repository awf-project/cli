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

// stepExecutorRecorder records step executions for verification
type stepExecutorRecorder struct {
	executions []stepExecution
	results    map[string]error
}

type stepExecution struct {
	stepName string
	loopData *interpolation.LoopData
}

func newStepExecutorRecorder() *stepExecutorRecorder {
	return &stepExecutorRecorder{
		executions: make([]stepExecution, 0),
		results:    make(map[string]error),
	}
}

func (r *stepExecutorRecorder) execute(ctx context.Context, stepName string, intCtx *interpolation.Context) error {
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
		return err
	}
	return nil
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

func TestLoopExecutor_ExecuteForEach_ExceedsMaxIterations(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-foreach-max",
		Steps: map[string]*workflow.Step{},
	}

	// Create items that exceed max_iterations
	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["a", "b", "c", "d", "e"]`,
			Body:          []string{"process"},
			MaxIterations: 3, // Less than items count
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-max")

	_, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) error {
			return nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_iterations")
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
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) error {
			callCount++
			if callCount == 2 {
				return stepErr
			}
			return nil
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
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) error {
			callCount++
			if callCount == 2 {
				cancel() // Cancel after second iteration
			}
			return nil
		},
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
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
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) error {
			t.Error("should not execute with empty items")
			return nil
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
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) error {
			callCount++
			// After 3 calls, make condition false
			if callCount >= 3 {
				evaluator.results["states.check.output != 'ready'"] = false
			}
			return nil
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
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) error {
			callCount++
			return nil
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
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) error {
			return nil
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
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) error {
			callCount++
			// After 2 iterations, trigger break
			if callCount >= 2 {
				evaluator.results["states.work.exit_code != 0"] = true
			}
			return nil
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
		func(ctx context.Context, stepName string, intCtx *interpolation.Context) error {
			callCount++
			if callCount == 3 {
				return stepErr
			}
			return nil
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
	repo := newMockRepository()
	repo.workflows["foreach-workflow"] = &workflow.Workflow{
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
				OnSuccess: "process_files", // Returns to loop
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
	evaluator := newMockExpressionEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, executor, &mockLogger{})
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, executor, newMockParallelExecutor(), store,
		&mockLogger{}, newMockResolver(), nil, evaluator,
	)

	ctx, err := execSvc.Run(context.Background(), "foreach-workflow", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

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
				OnSuccess: "poll",
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
