package application_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//
// These tests verify the rolling window memory management functionality
// for loop execution, including:
//
// - Default behavior: unlimited retention (backward compatibility)
// - Rolling window with MaxRetainedIterations = 1, 10, 100
// - PrunedCount tracking across iterations
// - Memory bounds with large iteration counts
// - Edge cases: zero iterations, exactly at limit, over limit
// - Verify that only the last N iterations are retained
//
// Feature: C019 - Fix memory leaks and goroutine leaks
// Component: loop_iteration_pruning

// createTestWorkflow creates a minimal workflow for memory testing.
func createTestWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name: "test-memory",
		Steps: map[string]*workflow.Step{
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo {{loop.item}}",
			},
		},
	}
}

// createForEachStep creates a foreach loop step with given items and memory config.
func createForEachStep(items string, memConfig *workflow.LoopMemoryConfig) *workflow.Step {
	loopCfg := &workflow.LoopConfig{
		Type:          workflow.LoopTypeForEach,
		Items:         items,
		Body:          []string{"process"},
		MaxIterations: 100000, // High limit to not interfere with memory config
		MemoryConfig:  memConfig,
	}

	return &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: loopCfg,
	}
}

// createWhileStep creates a while loop step with given condition and memory config.
func createWhileStep(condition string, memConfig *workflow.LoopMemoryConfig) *workflow.Step {
	loopCfg := &workflow.LoopConfig{
		Type:          workflow.LoopTypeWhile,
		Condition:     condition,
		Body:          []string{"process"},
		MaxIterations: 100000, // High limit to not interfere with memory config
		MemoryConfig:  memConfig,
	}

	return &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: loopCfg,
	}
}

// TestLoopExecutor_UnlimitedRetention_DefaultBehavior verifies that when
// MaxRetainedIterations is 0 (default), all iterations are retained in memory.
func TestLoopExecutor_UnlimitedRetention_DefaultBehavior(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := createTestWorkflow()
	memConfig := workflow.DefaultLoopMemoryConfig()
	step := createForEachStep(`["a", "b", "c", "d", "e"]`, &memConfig)

	execCtx := workflow.NewExecutionContext("test-id", "test-memory")
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
	assert.Len(t, result.Iterations, 5)
	assert.Equal(t, 0, result.PrunedCount, "no iterations should be pruned with unlimited retention")
	assert.Equal(t, 5, result.TotalCount, "TotalCount should be 5")
}

// TestLoopExecutor_RollingWindow_SingleIteration verifies that when
// MaxRetainedIterations is 1, only the last iteration is kept in memory.
func TestLoopExecutor_RollingWindow_SingleIteration(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := createTestWorkflow()
	memConfig := workflow.LoopMemoryConfig{
		MaxRetainedIterations: 1,
	}
	step := createForEachStep(`["a", "b", "c", "d", "e"]`, &memConfig)

	execCtx := workflow.NewExecutionContext("test-id", "test-memory")
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
	assert.Len(t, result.Iterations, 1)
	assert.Equal(t, 4, result.PrunedCount, "4 iterations should be pruned")
	assert.Equal(t, 5, result.TotalCount, "TotalCount should still be 5")

	// Verify that the retained iteration is the last one (index 4)
	assert.Equal(t, 4, result.Iterations[0].Index, "retained iteration should be the last one")
}

// TestLoopExecutor_RollingWindow_TenIterations verifies that when
// MaxRetainedIterations is 10, only the last 10 iterations are kept.
func TestLoopExecutor_RollingWindow_TenIterations(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := createTestWorkflow()
	memConfig := workflow.LoopMemoryConfig{
		MaxRetainedIterations: 10,
	}

	// Create 25 items as a JSON array
	items := `["a","b","c","d","e","f","g","h","i","j","k","l","m","n","o","p","q","r","s","t","u","v","w","x","y"]`
	step := createForEachStep(items, &memConfig)

	execCtx := workflow.NewExecutionContext("test-id", "test-memory")
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
	assert.Len(t, result.Iterations, 10)
	assert.Equal(t, 15, result.PrunedCount, "15 iterations should be pruned")
	assert.Equal(t, 25, result.TotalCount, "TotalCount should be 25")

	// Verify that retained iterations are the last 10 (indices 15-24)
	assert.Equal(t, 15, result.Iterations[0].Index, "first retained iteration should have index 15")
	assert.Equal(t, 24, result.Iterations[9].Index, "last retained iteration should have index 24")
}

// TestLoopExecutor_RollingWindow_ExactlyAtLimit verifies behavior when
// iteration count exactly matches MaxRetainedIterations.
func TestLoopExecutor_RollingWindow_ExactlyAtLimit(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := createTestWorkflow()
	memConfig := workflow.LoopMemoryConfig{
		MaxRetainedIterations: 5,
	}
	step := createForEachStep(`["a", "b", "c", "d", "e"]`, &memConfig)

	execCtx := workflow.NewExecutionContext("test-id", "test-memory")
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
	assert.Len(t, result.Iterations, 5)
	assert.Equal(t, 0, result.PrunedCount, "no iterations should be pruned when count equals limit")
	assert.Equal(t, 5, result.TotalCount, "TotalCount should be 5")
}

// TestLoopExecutor_RollingWindow_BelowLimit verifies behavior when
// iteration count is less than MaxRetainedIterations.
func TestLoopExecutor_RollingWindow_BelowLimit(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := createTestWorkflow()
	memConfig := workflow.LoopMemoryConfig{
		MaxRetainedIterations: 10,
	}
	step := createForEachStep(`["a", "b", "c"]`, &memConfig)

	execCtx := workflow.NewExecutionContext("test-id", "test-memory")
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
	assert.Len(t, result.Iterations, 3)
	assert.Equal(t, 0, result.PrunedCount, "no iterations should be pruned when count is below limit")
	assert.Equal(t, 3, result.TotalCount, "TotalCount should be 3")
}

// TestLoopExecutor_RollingWindow_ZeroIterations verifies behavior when
// loop executes zero iterations (empty items list).
func TestLoopExecutor_RollingWindow_ZeroIterations(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := createTestWorkflow()
	memConfig := workflow.LoopMemoryConfig{
		MaxRetainedIterations: 5,
	}
	step := createForEachStep(`[]`, &memConfig)

	execCtx := workflow.NewExecutionContext("test-id", "test-memory")
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
	assert.Empty(t, result.Iterations)
	assert.Equal(t, 0, result.PrunedCount, "no iterations should be pruned")
	assert.Equal(t, 0, result.TotalCount, "TotalCount should be 0")
}

// TestLoopExecutor_RollingWindow_LargeIterationCount verifies memory
// management with a large number of iterations (1000+).
func TestLoopExecutor_RollingWindow_LargeIterationCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large iteration test in short mode")
	}

	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := createTestWorkflow()
	memConfig := workflow.LoopMemoryConfig{
		MaxRetainedIterations: 100,
	}

	// Create 1000 items as a JSON array
	items := "["
	for i := 0; i < 1000; i++ {
		if i > 0 {
			items += ","
		}
		items += fmt.Sprintf(`"item%d"`, i)
	}
	items += "]"

	step := createForEachStep(items, &memConfig)

	execCtx := workflow.NewExecutionContext("test-id", "test-memory")
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
	assert.Len(t, result.Iterations, 100)
	assert.Equal(t, 900, result.PrunedCount, "900 iterations should be pruned")
	assert.Equal(t, 1000, result.TotalCount, "TotalCount should be 1000")

	// Verify that retained iterations are the last 100 (indices 900-999)
	assert.Equal(t, 900, result.Iterations[0].Index, "first retained iteration should have index 900")
	assert.Equal(t, 999, result.Iterations[99].Index, "last retained iteration should have index 999")
}

// TestLoopExecutor_RollingWindow_WhileLoop verifies rolling window behavior
// with while loops (not just foreach).
func TestLoopExecutor_RollingWindow_WhileLoop(t *testing.T) {
	logger := &mockLogger{}
	resolver := newMockResolver()

	// Configure evaluator to return true for first 10 calls, then false
	counter := &counterExpressionEvaluator{maxCount: 10}

	loopExec := application.NewLoopExecutor(logger, counter, resolver)

	wf := createTestWorkflow()
	memConfig := workflow.LoopMemoryConfig{
		MaxRetainedIterations: 5,
	}

	step := createWhileStep("{{loop.index < 10}}", &memConfig)

	execCtx := workflow.NewExecutionContext("test-id", "test-memory")
	recorder := newStepExecutorRecorder()

	result, err := loopExec.ExecuteWhile(
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
	assert.Len(t, result.Iterations, 5)
	assert.Equal(t, 5, result.PrunedCount, "5 iterations should be pruned")
	assert.Equal(t, 10, result.TotalCount, "TotalCount should be 10")

	// Verify that retained iterations are the last 5 (indices 5-9)
	assert.Equal(t, 5, result.Iterations[0].Index, "first retained iteration should have index 5")
	assert.Equal(t, 9, result.Iterations[4].Index, "last retained iteration should have index 9")
}

// TestLoopExecutor_RollingWindow_PrunedCountAccumulates verifies that
// PrunedCount accurately reflects all pruned iterations across the execution.
func TestLoopExecutor_RollingWindow_PrunedCountAccumulates(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := createTestWorkflow()
	memConfig := workflow.LoopMemoryConfig{
		MaxRetainedIterations: 3,
	}

	items := `["a","b","c","d","e","f","g","h","i","j"]`
	step := createForEachStep(items, &memConfig)

	execCtx := workflow.NewExecutionContext("test-id", "test-memory")
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
	assert.Len(t, result.Iterations, 3)
	assert.Equal(t, 7, result.PrunedCount, "7 iterations should be pruned (10 total - 3 retained)")
	assert.Equal(t, 10, result.TotalCount, "TotalCount should still reflect all 10 iterations")
}

// TestLoopExecutor_RollingWindow_IterationOrderPreserved verifies that
// the order of retained iterations is preserved (oldest to newest).
func TestLoopExecutor_RollingWindow_IterationOrderPreserved(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := createTestWorkflow()
	memConfig := workflow.LoopMemoryConfig{
		MaxRetainedIterations: 5,
	}

	items := `["a","b","c","d","e","f","g","h"]`
	step := createForEachStep(items, &memConfig)

	execCtx := workflow.NewExecutionContext("test-id", "test-memory")
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
	assert.Len(t, result.Iterations, 5)
	assert.Equal(t, 3, result.PrunedCount, "3 iterations should be pruned")

	// Verify order: indices should be 3, 4, 5, 6, 7 in that order
	for i := 0; i < 5; i++ {
		expectedIndex := i + 3
		assert.Equal(t, expectedIndex, result.Iterations[i].Index,
			"iteration at position %d should have index %d", i, expectedIndex)
	}
}

// TestLoopExecutor_RollingWindow_NegativeMaxRetainedIterations verifies
// that negative values are treated as unlimited (same as zero).
func TestLoopExecutor_RollingWindow_NegativeMaxRetainedIterations(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := createTestWorkflow()
	memConfig := workflow.LoopMemoryConfig{
		MaxRetainedIterations: -1,
	}
	step := createForEachStep(`["a", "b", "c", "d", "e"]`, &memConfig)

	execCtx := workflow.NewExecutionContext("test-id", "test-memory")
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
	assert.Len(t, result.Iterations, 5)
	assert.Equal(t, 0, result.PrunedCount, "no iterations should be pruned with negative limit")
	assert.Equal(t, 5, result.TotalCount, "TotalCount should be 5")
}

// counterExpressionEvaluator returns true for first N calls, then false
// Used for while loop testing
type counterExpressionEvaluator struct {
	count    int
	maxCount int
}

func (c *counterExpressionEvaluator) EvaluateBool(expr string, ctx *interpolation.Context) (bool, error) {
	if c.count >= c.maxCount {
		return false, nil
	}
	c.count++
	return true, nil
}

func (c *counterExpressionEvaluator) EvaluateInt(expr string, ctx *interpolation.Context) (int, error) {
	return 0, fmt.Errorf("not implemented")
}
