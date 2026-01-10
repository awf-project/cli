package application_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// =============================================================================
// Mock StepExecutor for parallel execution tests
// =============================================================================

type mockStepExecutor struct {
	results      map[string]*workflow.BranchResult
	errors       map[string]error
	executionLog []string
	callCount    atomic.Int32
	delay        time.Duration
	mu           sync.Mutex
}

func newMockStepExecutor() *mockStepExecutor {
	return &mockStepExecutor{
		results:      make(map[string]*workflow.BranchResult),
		errors:       make(map[string]error),
		executionLog: make([]string, 0),
	}
}

func (m *mockStepExecutor) ExecuteStep(
	ctx context.Context,
	wf *workflow.Workflow,
	stepName string,
	execCtx *workflow.ExecutionContext,
) (*workflow.BranchResult, error) {
	m.callCount.Add(1)
	m.mu.Lock()
	m.executionLog = append(m.executionLog, stepName)
	m.mu.Unlock()

	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("step execution cancelled: %w", ctx.Err())
		case <-time.After(m.delay):
		}
	}

	if err, ok := m.errors[stepName]; ok {
		return &workflow.BranchResult{
			Name:  stepName,
			Error: err,
		}, err
	}

	if result, ok := m.results[stepName]; ok {
		return result, nil
	}

	return &workflow.BranchResult{
		Name:        stepName,
		Output:      "ok",
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}, nil
}

// Verify mock implements interface
var _ ports.StepExecutor = (*mockStepExecutor)(nil)

// =============================================================================
// ParallelExecutor Tests
// =============================================================================

// NOTE: Domain type tests (ParallelStrategy, BranchResult, ParallelResult)
// are in internal/domain/workflow/parallel_test.go

func TestNewParallelExecutor(t *testing.T) {
	executor := application.NewParallelExecutor(&mockLogger{})
	assert.NotNil(t, executor)
}

func TestParallelExecutor_Execute_AllSucceedStrategy(t *testing.T) {
	stepExecutor := newMockStepExecutor()
	stepExecutor.results["branch1"] = &workflow.BranchResult{Name: "branch1", ExitCode: 0, Output: "out1"}
	stepExecutor.results["branch2"] = &workflow.BranchResult{Name: "branch2", ExitCode: 0, Output: "out2"}
	stepExecutor.results["branch3"] = &workflow.BranchResult{Name: "branch3", ExitCode: 0, Output: "out3"}

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"branch3": {Name: "branch3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy:      workflow.StrategyAllSucceed,
		MaxConcurrent: 0, // unlimited
	}

	result, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"branch1", "branch2", "branch3"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.AllSucceeded())
	assert.Equal(t, 3, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
}

func TestParallelExecutor_Execute_AllSucceedOneFailure(t *testing.T) {
	stepExecutor := newMockStepExecutor()
	stepExecutor.results["branch1"] = &workflow.BranchResult{Name: "branch1", ExitCode: 0}
	stepExecutor.errors["branch2"] = errors.New("command failed")
	stepExecutor.results["branch3"] = &workflow.BranchResult{Name: "branch3", ExitCode: 0}

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand},
			"branch3": {Name: "branch3", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy: workflow.StrategyAllSucceed,
	}

	_, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"branch1", "branch2", "branch3"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.Error(t, err, "all_succeed should return error when any branch fails")
}

func TestParallelExecutor_Execute_AnySucceedStrategy(t *testing.T) {
	stepExecutor := newMockStepExecutor()
	stepExecutor.errors["branch1"] = errors.New("failed 1")
	stepExecutor.results["branch2"] = &workflow.BranchResult{Name: "branch2", ExitCode: 0, Output: "success"}
	stepExecutor.errors["branch3"] = errors.New("failed 3")

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand},
			"branch3": {Name: "branch3", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy: workflow.StrategyAnySucceed,
	}

	result, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"branch1", "branch2", "branch3"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.NoError(t, err, "any_succeed should succeed when at least one branch succeeds")
	require.NotNil(t, result)
	assert.True(t, result.AnySucceeded())
}

func TestParallelExecutor_Execute_AnySucceedAllFail(t *testing.T) {
	stepExecutor := newMockStepExecutor()
	stepExecutor.errors["branch1"] = errors.New("failed 1")
	stepExecutor.errors["branch2"] = errors.New("failed 2")
	stepExecutor.errors["branch3"] = errors.New("failed 3")

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand},
			"branch3": {Name: "branch3", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy: workflow.StrategyAnySucceed,
	}

	_, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"branch1", "branch2", "branch3"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.Error(t, err, "any_succeed should fail when all branches fail")
}

func TestParallelExecutor_Execute_BestEffortStrategy(t *testing.T) {
	stepExecutor := newMockStepExecutor()
	stepExecutor.results["branch1"] = &workflow.BranchResult{Name: "branch1", ExitCode: 0}
	stepExecutor.errors["branch2"] = errors.New("failed")
	stepExecutor.results["branch3"] = &workflow.BranchResult{Name: "branch3", ExitCode: 0}

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand},
			"branch3": {Name: "branch3", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy: workflow.StrategyBestEffort,
	}

	result, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"branch1", "branch2", "branch3"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.NoError(t, err, "best_effort should not return error even with failures")
	require.NotNil(t, result)
	assert.Equal(t, 3, len(result.Results), "best_effort should collect all results")
	assert.Equal(t, 2, result.SuccessCount)
	assert.Equal(t, 1, result.FailureCount)
}

func TestParallelExecutor_Execute_MaxConcurrent(t *testing.T) {
	stepExecutor := newMockStepExecutor()
	stepExecutor.delay = 50 * time.Millisecond

	for i := 1; i <= 5; i++ {
		name := "branch" + string(rune('0'+i))
		stepExecutor.results[name] = &workflow.BranchResult{Name: name, ExitCode: 0}
	}

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand},
			"branch3": {Name: "branch3", Type: workflow.StepTypeCommand},
			"branch4": {Name: "branch4", Type: workflow.StepTypeCommand},
			"branch5": {Name: "branch5", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy:      workflow.StrategyBestEffort,
		MaxConcurrent: 2, // only 2 at a time
	}

	result, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"branch1", "branch2", "branch3", "branch4", "branch5"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 5, len(result.Results))
}

func TestParallelExecutor_Execute_ContextCancellation(t *testing.T) {
	stepExecutor := newMockStepExecutor()
	stepExecutor.delay = 500 * time.Millisecond

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy: workflow.StrategyAllSucceed,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := executor.Execute(
		ctx,
		wf,
		[]string{"branch1", "branch2"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.Error(t, err, "should return error on context cancellation")
}

func TestParallelExecutor_Execute_EmptyBranches(t *testing.T) {
	wf := &workflow.Workflow{Name: "test"}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{Strategy: workflow.StrategyAllSucceed}

	result, err := executor.Execute(
		context.Background(),
		wf,
		[]string{}, // empty branches
		config,
		workflow.NewExecutionContext("test-id", "test"),
		newMockStepExecutor(),
	)

	require.NoError(t, err, "empty branches should succeed")
	require.NotNil(t, result)
	assert.Equal(t, 0, len(result.Results))
}

func TestParallelExecutor_Execute_BranchResultsAccessible(t *testing.T) {
	stepExecutor := newMockStepExecutor()
	stepExecutor.results["step_a"] = &workflow.BranchResult{
		Name:     "step_a",
		Output:   "output_a",
		ExitCode: 0,
	}
	stepExecutor.results["step_b"] = &workflow.BranchResult{
		Name:     "step_b",
		Output:   "output_b",
		ExitCode: 0,
	}

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"step_a": {Name: "step_a", Type: workflow.StepTypeCommand},
			"step_b": {Name: "step_b", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{Strategy: workflow.StrategyAllSucceed}

	result, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"step_a", "step_b"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	branchA, ok := result.Results["step_a"]
	require.True(t, ok, "step_a result should be accessible")
	assert.Equal(t, "output_a", branchA.Output)

	branchB, ok := result.Results["step_b"]
	require.True(t, ok, "step_b result should be accessible")
	assert.Equal(t, "output_b", branchB.Output)
}

// =============================================================================
// Single Branch Tests
// =============================================================================

func TestParallelExecutor_Execute_SingleBranch(t *testing.T) {
	tests := []struct {
		name      string
		strategy  workflow.ParallelStrategy
		success   bool
		wantError bool
	}{
		{"all_succeed single success", workflow.StrategyAllSucceed, true, false},
		{"all_succeed single failure", workflow.StrategyAllSucceed, false, true},
		{"any_succeed single success", workflow.StrategyAnySucceed, true, false},
		{"any_succeed single failure", workflow.StrategyAnySucceed, false, true},
		{"best_effort single success", workflow.StrategyBestEffort, true, false},
		{"best_effort single failure", workflow.StrategyBestEffort, false, false}, // best_effort never errors
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stepExecutor := newMockStepExecutor()
			if tt.success {
				stepExecutor.results["only"] = &workflow.BranchResult{Name: "only", ExitCode: 0}
			} else {
				stepExecutor.errors["only"] = errors.New("failed")
			}

			wf := &workflow.Workflow{
				Name: "test",
				Steps: map[string]*workflow.Step{
					"only": {Name: "only", Type: workflow.StepTypeCommand},
				},
			}

			executor := application.NewParallelExecutor(&mockLogger{})
			config := workflow.ParallelConfig{Strategy: tt.strategy}

			_, err := executor.Execute(
				context.Background(),
				wf,
				[]string{"only"},
				config,
				workflow.NewExecutionContext("test-id", "test"),
				stepExecutor,
			)

			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// NonZero ExitCode vs Error Tests
// =============================================================================

func TestParallelExecutor_Execute_NonZeroExitCodeWithoutError(t *testing.T) {
	// Tests that non-zero exit codes are treated as failures even without an error
	stepExecutor := newMockStepExecutor()
	stepExecutor.results["branch1"] = &workflow.BranchResult{
		Name:     "branch1",
		ExitCode: 0,
		Output:   "success",
	}
	stepExecutor.results["branch2"] = &workflow.BranchResult{
		Name:     "branch2",
		ExitCode: 1, // failed but no error
		Output:   "failed output",
	}

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})

	t.Run("all_succeed fails on non-zero exit", func(t *testing.T) {
		config := workflow.ParallelConfig{Strategy: workflow.StrategyAllSucceed}

		_, err := executor.Execute(
			context.Background(),
			wf,
			[]string{"branch1", "branch2"},
			config,
			workflow.NewExecutionContext("test-id", "test"),
			stepExecutor,
		)

		require.Error(t, err, "all_succeed should fail when any branch has non-zero exit")
	})

	t.Run("any_succeed succeeds with one zero exit", func(t *testing.T) {
		config := workflow.ParallelConfig{Strategy: workflow.StrategyAnySucceed}

		result, err := executor.Execute(
			context.Background(),
			wf,
			[]string{"branch1", "branch2"},
			config,
			workflow.NewExecutionContext("test-id", "test"),
			stepExecutor,
		)

		require.NoError(t, err)
		assert.True(t, result.AnySucceeded())
	})

	t.Run("best_effort collects all with mixed exits", func(t *testing.T) {
		config := workflow.ParallelConfig{Strategy: workflow.StrategyBestEffort}

		result, err := executor.Execute(
			context.Background(),
			wf,
			[]string{"branch1", "branch2"},
			config,
			workflow.NewExecutionContext("test-id", "test"),
			stepExecutor,
		)

		require.NoError(t, err)
		assert.Equal(t, 1, result.SuccessCount)
		assert.Equal(t, 1, result.FailureCount)
		assert.Equal(t, 2, len(result.Results))
	})
}

// =============================================================================
// Concurrency Control Tests
// =============================================================================

func TestParallelExecutor_Execute_ConcurrencyLimiting(t *testing.T) {
	// Use a mock that tracks concurrent execution count
	maxConcurrent := 2
	branchCount := 6
	var currentConcurrent atomic.Int32
	var maxObservedConcurrent atomic.Int32

	stepExecutor := &trackingStepExecutor{
		onExecute: func(ctx context.Context, stepName string) (*workflow.BranchResult, error) {
			// Increment current concurrent count
			current := currentConcurrent.Add(1)

			// Track maximum observed
			for {
				observed := maxObservedConcurrent.Load()
				if current <= observed || maxObservedConcurrent.CompareAndSwap(observed, current) {
					break
				}
			}

			// Simulate work
			select {
			case <-ctx.Done():
				currentConcurrent.Add(-1)
				return nil, ctx.Err()
			case <-time.After(50 * time.Millisecond):
			}

			currentConcurrent.Add(-1)
			return &workflow.BranchResult{Name: stepName, ExitCode: 0}, nil
		},
	}

	wf := &workflow.Workflow{
		Name:  "test",
		Steps: make(map[string]*workflow.Step),
	}
	branches := make([]string, branchCount)
	for i := 0; i < branchCount; i++ {
		name := "branch" + string(rune('A'+i))
		branches[i] = name
		wf.Steps[name] = &workflow.Step{Name: name, Type: workflow.StepTypeCommand}
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy:      workflow.StrategyBestEffort,
		MaxConcurrent: maxConcurrent,
	}

	result, err := executor.Execute(
		context.Background(),
		wf,
		branches,
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, branchCount, len(result.Results))

	// Verify concurrency was limited
	assert.LessOrEqual(t, int(maxObservedConcurrent.Load()), maxConcurrent,
		"concurrent executions should never exceed MaxConcurrent")
}

func TestParallelExecutor_Execute_UnlimitedConcurrency(t *testing.T) {
	// MaxConcurrent = 0 means unlimited
	branchCount := 5
	var maxObservedConcurrent atomic.Int32
	var currentConcurrent atomic.Int32

	stepExecutor := &trackingStepExecutor{
		onExecute: func(ctx context.Context, stepName string) (*workflow.BranchResult, error) {
			current := currentConcurrent.Add(1)
			for {
				observed := maxObservedConcurrent.Load()
				if current <= observed || maxObservedConcurrent.CompareAndSwap(observed, current) {
					break
				}
			}

			time.Sleep(100 * time.Millisecond)
			currentConcurrent.Add(-1)
			return &workflow.BranchResult{Name: stepName, ExitCode: 0}, nil
		},
	}

	wf := &workflow.Workflow{
		Name:  "test",
		Steps: make(map[string]*workflow.Step),
	}
	branches := make([]string, branchCount)
	for i := 0; i < branchCount; i++ {
		name := "branch" + string(rune('A'+i))
		branches[i] = name
		wf.Steps[name] = &workflow.Step{Name: name, Type: workflow.StepTypeCommand}
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy:      workflow.StrategyBestEffort,
		MaxConcurrent: 0, // unlimited
	}

	_, err := executor.Execute(
		context.Background(),
		wf,
		branches,
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.NoError(t, err)
	// With unlimited concurrency and all starting at once, we should observe all running concurrently
	assert.GreaterOrEqual(t, int(maxObservedConcurrent.Load()), branchCount-1,
		"with unlimited concurrency, all branches should run nearly simultaneously")
}

// =============================================================================
// AllSucceed Cancellation Behavior Tests
// =============================================================================

func TestParallelExecutor_Execute_AllSucceedCancelsOnFirstFailure(t *testing.T) {
	// First branch fails quickly, others should be cancelled
	var executedBranches atomic.Int32
	var cancelledBranches atomic.Int32

	stepExecutor := &trackingStepExecutor{
		onExecute: func(ctx context.Context, stepName string) (*workflow.BranchResult, error) {
			executedBranches.Add(1)

			if stepName == "fast_fail" {
				// Fail immediately
				return &workflow.BranchResult{
					Name:     stepName,
					ExitCode: 1,
					Error:    errors.New("fast failure"),
				}, errors.New("fast failure")
			}

			// Slow branches - wait and check for cancellation
			select {
			case <-ctx.Done():
				cancelledBranches.Add(1)
				return nil, ctx.Err()
			case <-time.After(1 * time.Second):
				return &workflow.BranchResult{Name: stepName, ExitCode: 0}, nil
			}
		},
	}

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"fast_fail": {Name: "fast_fail", Type: workflow.StepTypeCommand},
			"slow1":     {Name: "slow1", Type: workflow.StepTypeCommand},
			"slow2":     {Name: "slow2", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy:      workflow.StrategyAllSucceed,
		MaxConcurrent: 0, // all start at once
	}

	start := time.Now()
	_, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"fast_fail", "slow1", "slow2"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)
	elapsed := time.Since(start)

	require.Error(t, err, "should return error on failure")

	// Should complete quickly (not wait for slow branches)
	assert.Less(t, elapsed, 500*time.Millisecond,
		"all_succeed should cancel remaining branches on first failure")

	// Slow branches should have been cancelled
	assert.Greater(t, int(cancelledBranches.Load()), 0,
		"slow branches should receive context cancellation")
}

func TestParallelExecutor_Execute_AnySucceedCancelsOnFirstSuccess(t *testing.T) {
	// First branch succeeds quickly, others might be cancelled
	var cancelledCount atomic.Int32

	stepExecutor := &trackingStepExecutor{
		onExecute: func(ctx context.Context, stepName string) (*workflow.BranchResult, error) {
			if stepName == "fast_success" {
				return &workflow.BranchResult{Name: stepName, ExitCode: 0}, nil
			}

			// Slow branches
			select {
			case <-ctx.Done():
				cancelledCount.Add(1)
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
				return nil, errors.New("timeout - should have been cancelled")
			}
		},
	}

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"fast_success": {Name: "fast_success", Type: workflow.StepTypeCommand},
			"slow1":        {Name: "slow1", Type: workflow.StepTypeCommand},
			"slow2":        {Name: "slow2", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy:      workflow.StrategyAnySucceed,
		MaxConcurrent: 0,
	}

	start := time.Now()
	result, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"fast_success", "slow1", "slow2"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)
	elapsed := time.Since(start)

	require.NoError(t, err, "any_succeed should succeed when one branch succeeds")
	assert.True(t, result.AnySucceeded())
	assert.Less(t, elapsed, 500*time.Millisecond,
		"any_succeed should return quickly after first success")
}

func TestParallelExecutor_Execute_BestEffortWaitsForAll(t *testing.T) {
	// BestEffort should wait for all branches regardless of failures
	completedCount := atomic.Int32{}

	stepExecutor := &trackingStepExecutor{
		onExecute: func(ctx context.Context, stepName string) (*workflow.BranchResult, error) {
			time.Sleep(50 * time.Millisecond)
			completedCount.Add(1)

			if stepName == "will_fail" {
				return &workflow.BranchResult{
					Name:     stepName,
					ExitCode: 1,
					Error:    errors.New("expected failure"),
				}, errors.New("expected failure")
			}
			return &workflow.BranchResult{Name: stepName, ExitCode: 0}, nil
		},
	}

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"will_fail": {Name: "will_fail", Type: workflow.StepTypeCommand},
			"success1":  {Name: "success1", Type: workflow.StepTypeCommand},
			"success2":  {Name: "success2", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{
		Strategy: workflow.StrategyBestEffort,
	}

	result, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"will_fail", "success1", "success2"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.NoError(t, err, "best_effort should not return error")
	assert.Equal(t, int32(3), completedCount.Load(), "all branches should complete")
	assert.Equal(t, 2, result.SuccessCount)
	assert.Equal(t, 1, result.FailureCount)
	assert.Len(t, result.Results, 3)
}

// =============================================================================
// Context Cancellation Edge Cases
// =============================================================================

func TestParallelExecutor_Execute_AlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before execution

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{Strategy: workflow.StrategyAllSucceed}

	_, err := executor.Execute(
		ctx,
		wf,
		[]string{"branch1"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		newMockStepExecutor(),
	)

	require.Error(t, err, "should fail immediately with cancelled context")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestParallelExecutor_Execute_ContextDeadlineExceeded(t *testing.T) {
	stepExecutor := newMockStepExecutor()
	stepExecutor.delay = 500 * time.Millisecond

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{Strategy: workflow.StrategyAllSucceed}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := executor.Execute(
		ctx,
		wf,
		[]string{"branch1"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled),
		"error should be context-related")
}

// =============================================================================
// Result Timing Tests
// =============================================================================

func TestParallelExecutor_Execute_ResultTiming(t *testing.T) {
	stepExecutor := newMockStepExecutor()
	stepExecutor.results["branch1"] = &workflow.BranchResult{
		Name:     "branch1",
		ExitCode: 0,
	}

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	config := workflow.ParallelConfig{Strategy: workflow.StrategyAllSucceed}

	beforeExec := time.Now()
	result, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"branch1"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)
	afterExec := time.Now()

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.StartedAt.IsZero(), "StartedAt should be set")
	assert.False(t, result.CompletedAt.IsZero(), "CompletedAt should be set")
	assert.True(t, result.StartedAt.After(beforeExec) || result.StartedAt.Equal(beforeExec))
	assert.True(t, result.CompletedAt.Before(afterExec) || result.CompletedAt.Equal(afterExec))
	assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt))
}

// =============================================================================
// Default Strategy Tests
// =============================================================================

func TestParallelExecutor_Execute_DefaultStrategy(t *testing.T) {
	stepExecutor := newMockStepExecutor()
	stepExecutor.results["branch1"] = &workflow.BranchResult{Name: "branch1", ExitCode: 0}
	stepExecutor.errors["branch2"] = errors.New("failed")

	wf := &workflow.Workflow{
		Name: "test",
		Steps: map[string]*workflow.Step{
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand},
		},
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	// Empty strategy should default to all_succeed behavior
	config := workflow.ParallelConfig{
		Strategy: "",
	}

	_, err := executor.Execute(
		context.Background(),
		wf,
		[]string{"branch1", "branch2"},
		config,
		workflow.NewExecutionContext("test-id", "test"),
		stepExecutor,
	)

	require.Error(t, err, "default (empty) strategy should behave like all_succeed")
}

// =============================================================================
// Helper Types for Advanced Tests
// =============================================================================

// trackingStepExecutor allows custom execution behavior for testing concurrency
type trackingStepExecutor struct {
	onExecute func(ctx context.Context, stepName string) (*workflow.BranchResult, error)
}

func (e *trackingStepExecutor) ExecuteStep(
	ctx context.Context,
	wf *workflow.Workflow,
	stepName string,
	execCtx *workflow.ExecutionContext,
) (*workflow.BranchResult, error) {
	return e.onExecute(ctx, stepName)
}

var _ ports.StepExecutor = (*trackingStepExecutor)(nil)
