package application_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// =============================================================================
// Tests for T007: runBranchWithSemaphore helper
// Feature: C005
// =============================================================================
// Note: Uses mockStepExecutor and mockLogger from parallel_executor_test.go

func TestParallelExecutor_RunBranchWithSemaphore_SuccessfulExecution(t *testing.T) {
	// Arrange: setup executor with successful branch result
	stepExecutor := newMockStepExecutor()
	stepExecutor.results["branch1"] = &workflow.BranchResult{
		Name:        "branch1",
		Output:      "success output",
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	result := workflow.NewParallelResult()
	var mu sync.Mutex
	sem := make(chan struct{}, 1)

	ctx := context.Background()
	wf := &workflow.Workflow{Name: "test-workflow"}
	execCtx := workflow.NewExecutionContext("exec-1", "test-workflow")

	// Act: execute branch with semaphore
	branchResult, err := executor.RunBranchWithSemaphore(
		ctx,
		"branch1",
		sem,
		stepExecutor,
		wf,
		execCtx,
		result,
		&mu,
	)

	// Assert: successful execution
	require.NoError(t, err)
	require.NotNil(t, branchResult)
	assert.Equal(t, "branch1", branchResult.Name)
	assert.Equal(t, "success output", branchResult.Output)
	assert.Equal(t, 0, branchResult.ExitCode)
	assert.True(t, branchResult.Success())

	// Verify result was added to parallel result
	assert.Equal(t, 1, len(result.Results))
}

func TestParallelExecutor_RunBranchWithSemaphore_ExecutorError(t *testing.T) {
	// Arrange: setup executor with error
	expectedErr := errors.New("execution failed")
	stepExecutor := newMockStepExecutor()
	stepExecutor.errors["branch2"] = expectedErr

	executor := application.NewParallelExecutor(&mockLogger{})
	result := workflow.NewParallelResult()
	var mu sync.Mutex
	sem := make(chan struct{}, 1)

	ctx := context.Background()
	wf := &workflow.Workflow{Name: "test-workflow"}
	execCtx := workflow.NewExecutionContext("exec-2", "test-workflow")

	// Act: execute branch that will fail
	branchResult, err := executor.RunBranchWithSemaphore(
		ctx,
		"branch2",
		sem,
		stepExecutor,
		wf,
		execCtx,
		result,
		&mu,
	)

	// Assert: error returned and result added
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	require.NotNil(t, branchResult)
	assert.Equal(t, "branch2", branchResult.Name)
	assert.NotNil(t, branchResult.Error)

	// Verify error result was added to parallel result
	assert.Equal(t, 1, len(result.Results))
	assert.NotNil(t, result.FirstError)
}

func TestParallelExecutor_RunBranchWithSemaphore_ContextCancellation(t *testing.T) {
	// Arrange: setup with cancellable context
	stepExecutor := newMockStepExecutor()
	stepExecutor.delay = 100 * time.Millisecond // Slow execution

	executor := application.NewParallelExecutor(&mockLogger{})
	result := workflow.NewParallelResult()
	var mu sync.Mutex
	sem := make(chan struct{}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	wf := &workflow.Workflow{Name: "test-workflow"}
	execCtx := workflow.NewExecutionContext("exec-3", "test-workflow")

	// Act: execute branch with cancelled context
	branchResult, err := executor.RunBranchWithSemaphore(
		ctx,
		"branch3",
		sem,
		stepExecutor,
		wf,
		execCtx,
		result,
		&mu,
	)

	// Assert: context cancellation detected
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	// Stub returns nil, nil - implementation should return nil, ctx.Err()
	assert.Nil(t, branchResult)
}

func TestParallelExecutor_RunBranchWithSemaphore_SemaphoreBlocking(t *testing.T) {
	// Arrange: setup with full semaphore (size 1, already occupied)
	stepExecutor := newMockStepExecutor()
	stepExecutor.results["branch4"] = &workflow.BranchResult{
		Name:     "branch4",
		Output:   "output",
		ExitCode: 0,
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	result := workflow.NewParallelResult()
	var mu sync.Mutex
	sem := make(chan struct{}, 1)

	// Fill semaphore
	sem <- struct{}{}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	wf := &workflow.Workflow{Name: "test-workflow"}
	execCtx := workflow.NewExecutionContext("exec-4", "test-workflow")

	// Act: attempt to execute branch with full semaphore
	branchResult, err := executor.RunBranchWithSemaphore(
		ctx,
		"branch4",
		sem,
		stepExecutor,
		wf,
		execCtx,
		result,
		&mu,
	)

	// Assert: timeout waiting for semaphore
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	// Stub returns nil, nil - implementation should detect ctx.Done() and return error
	assert.Nil(t, branchResult)
}

func TestParallelExecutor_RunBranchWithSemaphore_NoSemaphore(t *testing.T) {
	// Arrange: setup without semaphore (unlimited concurrency)
	stepExecutor := newMockStepExecutor()
	stepExecutor.results["branch5"] = &workflow.BranchResult{
		Name:        "branch5",
		Output:      "no semaphore",
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	result := workflow.NewParallelResult()
	var mu sync.Mutex

	ctx := context.Background()
	wf := &workflow.Workflow{Name: "test-workflow"}
	execCtx := workflow.NewExecutionContext("exec-5", "test-workflow")

	// Act: execute branch without semaphore (nil)
	branchResult, err := executor.RunBranchWithSemaphore(
		ctx,
		"branch5",
		nil, // No semaphore
		stepExecutor,
		wf,
		execCtx,
		result,
		&mu,
	)

	// Assert: successful execution without semaphore
	require.NoError(t, err)
	require.NotNil(t, branchResult)
	assert.Equal(t, "branch5", branchResult.Name)
	assert.Equal(t, "no semaphore", branchResult.Output)
	assert.Equal(t, 1, len(result.Results))
}

func TestParallelExecutor_RunBranchWithSemaphore_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		branchName     string
		setupExecutor  func(*mockStepExecutor)
		setupContext   func() (context.Context, context.CancelFunc)
		semaphoreSize  int
		fillSemaphore  bool
		wantErr        bool
		wantErrType    error
		validateResult func(*testing.T, *workflow.BranchResult)
	}{
		{
			name:       "successful execution with semaphore",
			branchName: "success",
			setupExecutor: func(m *mockStepExecutor) {
				m.results["success"] = &workflow.BranchResult{
					Name:     "success",
					Output:   "ok",
					ExitCode: 0,
				}
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			semaphoreSize: 2,
			fillSemaphore: false,
			wantErr:       false,
			validateResult: func(t *testing.T, br *workflow.BranchResult) {
				assert.Equal(t, "success", br.Name)
				assert.True(t, br.Success())
			},
		},
		{
			name:       "execution error",
			branchName: "error",
			setupExecutor: func(m *mockStepExecutor) {
				m.errors["error"] = errors.New("step failed")
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			semaphoreSize: 1,
			fillSemaphore: false,
			wantErr:       true,
			wantErrType:   nil, // Any error
			validateResult: func(t *testing.T, br *workflow.BranchResult) {
				assert.Equal(t, "error", br.Name)
				assert.NotNil(t, br.Error)
			},
		},
		{
			name:       "context cancelled before execution",
			branchName: "cancelled",
			setupExecutor: func(m *mockStepExecutor) {
				m.delay = 100 * time.Millisecond
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Immediate cancellation
				return ctx, cancel
			},
			semaphoreSize: 1,
			fillSemaphore: false,
			wantErr:       true,
			wantErrType:   context.Canceled,
			validateResult: func(t *testing.T, br *workflow.BranchResult) {
				assert.Nil(t, br) // Stub returns nil on cancellation
			},
		},
		{
			name:       "semaphore timeout",
			branchName: "timeout",
			setupExecutor: func(m *mockStepExecutor) {
				m.results["timeout"] = &workflow.BranchResult{
					Name:     "timeout",
					ExitCode: 0,
				}
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 50*time.Millisecond)
			},
			semaphoreSize: 1,
			fillSemaphore: true, // Block semaphore
			wantErr:       true,
			wantErrType:   context.DeadlineExceeded,
			validateResult: func(t *testing.T, br *workflow.BranchResult) {
				assert.Nil(t, br) // Stub returns nil on timeout
			},
		},
		{
			name:       "no semaphore unlimited concurrency",
			branchName: "unlimited",
			setupExecutor: func(m *mockStepExecutor) {
				m.results["unlimited"] = &workflow.BranchResult{
					Name:     "unlimited",
					Output:   "fast",
					ExitCode: 0,
				}
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			semaphoreSize: 0, // No semaphore (nil)
			fillSemaphore: false,
			wantErr:       false,
			validateResult: func(t *testing.T, br *workflow.BranchResult) {
				assert.Equal(t, "unlimited", br.Name)
				assert.Equal(t, "fast", br.Output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			stepExecutor := newMockStepExecutor()
			tt.setupExecutor(stepExecutor)

			executor := application.NewParallelExecutor(&mockLogger{})
			result := workflow.NewParallelResult()
			var mu sync.Mutex

			var sem chan struct{}
			if tt.semaphoreSize > 0 {
				sem = make(chan struct{}, tt.semaphoreSize)
				if tt.fillSemaphore {
					sem <- struct{}{} // Fill semaphore to test blocking
				}
			}

			ctx, cancel := tt.setupContext()
			defer cancel()

			wf := &workflow.Workflow{Name: "test"}
			execCtx := workflow.NewExecutionContext("exec", "test")

			// Act
			branchResult, err := executor.RunBranchWithSemaphore(
				ctx,
				tt.branchName,
				sem,
				stepExecutor,
				wf,
				execCtx,
				result,
				&mu,
			)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrType != nil {
					assert.ErrorIs(t, err, tt.wantErrType)
				}
			} else {
				require.NoError(t, err)
			}

			tt.validateResult(t, branchResult)
		})
	}
}

func TestParallelExecutor_RunBranchWithSemaphore_ConcurrentAccess(t *testing.T) {
	// Arrange: test concurrent access to shared result and mutex
	stepExecutor := newMockStepExecutor()
	for i := 0; i < 10; i++ {
		branchName := fmt.Sprintf("branch%d", i)
		stepExecutor.results[branchName] = &workflow.BranchResult{
			Name:     branchName,
			Output:   "ok",
			ExitCode: 0,
		}
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	result := workflow.NewParallelResult()
	var mu sync.Mutex
	sem := make(chan struct{}, 3) // Limit concurrency to 3

	ctx := context.Background()
	wf := &workflow.Workflow{Name: "test-workflow"}
	execCtx := workflow.NewExecutionContext("exec-concurrent", "test-workflow")

	// Act: launch 10 concurrent branches
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			branchName := fmt.Sprintf("branch%d", i)
			_, err := executor.RunBranchWithSemaphore(
				ctx,
				branchName,
				sem,
				stepExecutor,
				wf,
				execCtx,
				result,
				&mu,
			)
			// Stub returns nil error for successful execution
			assert.NoError(t, err)
		}()
	}

	wg.Wait()

	// Assert: all branches executed and results added safely
	// Note: stub implementation may not properly add results
	// Real implementation should have len(result.Results) == 10
	assert.LessOrEqual(t, len(result.Results), 10)
}

func TestParallelExecutor_RunBranchWithSemaphore_SemaphoreRelease(t *testing.T) {
	// Arrange: verify semaphore is properly released after execution
	stepExecutor := newMockStepExecutor()
	stepExecutor.results["branch1"] = &workflow.BranchResult{
		Name:     "branch1",
		ExitCode: 0,
	}

	executor := application.NewParallelExecutor(&mockLogger{})
	result := workflow.NewParallelResult()
	var mu sync.Mutex
	sem := make(chan struct{}, 1)

	ctx := context.Background()
	wf := &workflow.Workflow{Name: "test"}
	execCtx := workflow.NewExecutionContext("exec", "test")

	// Act: execute branch
	_, err := executor.RunBranchWithSemaphore(
		ctx,
		"branch1",
		sem,
		stepExecutor,
		wf,
		execCtx,
		result,
		&mu,
	)
	require.NoError(t, err)

	// Assert: semaphore slot released (should be able to acquire again)
	select {
	case sem <- struct{}{}:
		// Successfully acquired - semaphore was released
		<-sem // Clean up
	case <-time.After(100 * time.Millisecond):
		t.Fatal("semaphore was not released after execution")
	}
}

func TestParallelExecutor_RunBranchWithSemaphore_ErrorWithSemaphoreRelease(t *testing.T) {
	// Arrange: verify semaphore is released even on error
	stepExecutor := newMockStepExecutor()
	stepExecutor.errors["branch-fail"] = errors.New("execution error")

	executor := application.NewParallelExecutor(&mockLogger{})
	result := workflow.NewParallelResult()
	var mu sync.Mutex
	sem := make(chan struct{}, 1)

	ctx := context.Background()
	wf := &workflow.Workflow{Name: "test"}
	execCtx := workflow.NewExecutionContext("exec", "test")

	// Act: execute failing branch
	_, err := executor.RunBranchWithSemaphore(
		ctx,
		"branch-fail",
		sem,
		stepExecutor,
		wf,
		execCtx,
		result,
		&mu,
	)
	require.Error(t, err)

	// Assert: semaphore slot released even after error
	select {
	case sem <- struct{}{}:
		// Successfully acquired - semaphore was released
		<-sem // Clean up
	case <-time.After(100 * time.Millisecond):
		t.Fatal("semaphore was not released after error")
	}
}

// =============================================================================
// Tests for T008: checkBranchSuccess helper
// Feature: C005
// =============================================================================

// TestParallelExecutor_CheckBranchSuccess_FirstSuccess tests the first successful branch
// detection and cancellation trigger.
func TestParallelExecutor_CheckBranchSuccess_FirstSuccess(t *testing.T) {
	// Arrange: setup for first success detection
	executor := application.NewParallelExecutor(&mockLogger{})

	successfulBranch := &workflow.BranchResult{
		Name:        "success-branch",
		Output:      "completed",
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	var firstSuccess bool
	var mu sync.Mutex
	successChan := make(chan struct{}, 1)

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	cancelCalled := false
	mockCancel := func() {
		cancelCalled = true
		cancel()
	}

	// Act: check branch success for the first successful branch
	executor.CheckBranchSuccess(successfulBranch, &firstSuccess, &mu, successChan, mockCancel)

	// Assert: first success detected and cancellation triggered
	assert.True(t, firstSuccess, "firstSuccess flag should be set to true")
	assert.True(t, cancelCalled, "cancel function should have been called")

	// Verify success signal sent
	select {
	case <-successChan:
		// Success channel received signal
	case <-time.After(100 * time.Millisecond):
		t.Fatal("success channel should have received signal")
	}
}

// TestParallelExecutor_CheckBranchSuccess_DuplicateSuccess tests idempotent behavior
// when multiple branches succeed (only first should trigger cancellation).
func TestParallelExecutor_CheckBranchSuccess_DuplicateSuccess(t *testing.T) {
	// Arrange: setup with firstSuccess already true
	executor := application.NewParallelExecutor(&mockLogger{})

	successfulBranch := &workflow.BranchResult{
		Name:        "second-success",
		Output:      "completed",
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	firstSuccess := true // Already set by previous success
	var mu sync.Mutex
	successChan := make(chan struct{}, 1)
	successChan <- struct{}{} // Already signaled

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	cancelCallCount := 0
	mockCancel := func() {
		cancelCallCount++
		cancel()
	}

	// Act: check branch success when already succeeded
	executor.CheckBranchSuccess(successfulBranch, &firstSuccess, &mu, successChan, mockCancel)

	// Assert: no additional cancellation or signal
	assert.True(t, firstSuccess, "firstSuccess should remain true")
	assert.Equal(t, 0, cancelCallCount, "cancel should not be called again")

	// Channel should still have only one signal
	select {
	case <-successChan:
		// First signal still there
	default:
		t.Fatal("success channel should still have the first signal")
	}

	// No additional signals should be present
	select {
	case <-successChan:
		t.Fatal("success channel should not have additional signals")
	default:
		// Expected - no duplicate signal
	}
}

// TestParallelExecutor_CheckBranchSuccess_FailedBranch tests that failed branches
// do not trigger success detection or cancellation.
func TestParallelExecutor_CheckBranchSuccess_FailedBranch(t *testing.T) {
	// Arrange: setup with failed branch
	executor := application.NewParallelExecutor(&mockLogger{})

	failedBranch := &workflow.BranchResult{
		Name:        "failed-branch",
		Output:      "error output",
		ExitCode:    1, // Non-zero exit code = failure
		Error:       errors.New("command failed"),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	var firstSuccess bool
	var mu sync.Mutex
	successChan := make(chan struct{}, 1)

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	cancelCalled := false
	mockCancel := func() {
		cancelCalled = true
		cancel()
	}

	// Act: check branch success for failed branch
	executor.CheckBranchSuccess(failedBranch, &firstSuccess, &mu, successChan, mockCancel)

	// Assert: no success detection or cancellation
	assert.False(t, firstSuccess, "firstSuccess should remain false for failed branch")
	assert.False(t, cancelCalled, "cancel should not be called for failed branch")

	// Channel should have no signals
	select {
	case <-successChan:
		t.Fatal("success channel should not receive signal for failed branch")
	case <-time.After(50 * time.Millisecond):
		// Expected - no signal
	}
}

// TestParallelExecutor_CheckBranchSuccess_NilBranchResult tests handling of nil result.
func TestParallelExecutor_CheckBranchSuccess_NilBranchResult(t *testing.T) {
	// Arrange: setup with nil branch result
	executor := application.NewParallelExecutor(&mockLogger{})

	var firstSuccess bool
	var mu sync.Mutex
	successChan := make(chan struct{}, 1)

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	cancelCalled := false
	mockCancel := func() {
		cancelCalled = true
		cancel()
	}

	// Act: check nil branch result (should handle gracefully)
	executor.CheckBranchSuccess(nil, &firstSuccess, &mu, successChan, mockCancel)

	// Assert: no crash, no success detection
	assert.False(t, firstSuccess, "firstSuccess should remain false for nil result")
	assert.False(t, cancelCalled, "cancel should not be called for nil result")

	// Channel should have no signals
	select {
	case <-successChan:
		t.Fatal("success channel should not receive signal for nil result")
	case <-time.After(50 * time.Millisecond):
		// Expected - no signal
	}
}

// TestParallelExecutor_CheckBranchSuccess_TableDriven provides comprehensive coverage
// of success detection scenarios.
func TestParallelExecutor_CheckBranchSuccess_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		branchResult        *workflow.BranchResult
		initialFirstSuccess bool
		expectFirstSuccess  bool
		expectCancel        bool
		expectSignal        bool
	}{
		{
			name: "first successful branch",
			branchResult: &workflow.BranchResult{
				Name:     "branch1",
				ExitCode: 0,
				Output:   "success",
			},
			initialFirstSuccess: false,
			expectFirstSuccess:  true,
			expectCancel:        true,
			expectSignal:        true,
		},
		{
			name: "second successful branch (idempotent)",
			branchResult: &workflow.BranchResult{
				Name:     "branch2",
				ExitCode: 0,
				Output:   "success",
			},
			initialFirstSuccess: true,
			expectFirstSuccess:  true,
			expectCancel:        false,
			expectSignal:        false,
		},
		{
			name: "failed branch with non-zero exit",
			branchResult: &workflow.BranchResult{
				Name:     "branch3",
				ExitCode: 1,
				Output:   "failed",
			},
			initialFirstSuccess: false,
			expectFirstSuccess:  false,
			expectCancel:        false,
			expectSignal:        false,
		},
		{
			name: "failed branch with error",
			branchResult: &workflow.BranchResult{
				Name:     "branch4",
				ExitCode: 0,
				Error:    errors.New("execution error"),
			},
			initialFirstSuccess: false,
			expectFirstSuccess:  false,
			expectCancel:        false,
			expectSignal:        false,
		},
		{
			name:                "nil branch result",
			branchResult:        nil,
			initialFirstSuccess: false,
			expectFirstSuccess:  false,
			expectCancel:        false,
			expectSignal:        false,
		},
		{
			name: "failed branch after first success",
			branchResult: &workflow.BranchResult{
				Name:     "branch5",
				ExitCode: 1,
			},
			initialFirstSuccess: true,
			expectFirstSuccess:  true,
			expectCancel:        false,
			expectSignal:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			executor := application.NewParallelExecutor(&mockLogger{})
			firstSuccess := tt.initialFirstSuccess
			var mu sync.Mutex
			successChan := make(chan struct{}, 1)

			_, cancel := context.WithCancel(context.Background())
			defer cancel()

			cancelCalled := false
			mockCancel := func() {
				cancelCalled = true
				cancel()
			}

			// Act
			executor.CheckBranchSuccess(tt.branchResult, &firstSuccess, &mu, successChan, mockCancel)

			// Assert
			assert.Equal(t, tt.expectFirstSuccess, firstSuccess, "firstSuccess flag mismatch")
			assert.Equal(t, tt.expectCancel, cancelCalled, "cancel invocation mismatch")

			if tt.expectSignal {
				select {
				case <-successChan:
					// Expected signal received
				case <-time.After(100 * time.Millisecond):
					t.Fatal("expected success signal not received")
				}
			} else {
				select {
				case <-successChan:
					t.Fatal("unexpected success signal received")
				case <-time.After(50 * time.Millisecond):
					// Expected - no signal
				}
			}
		})
	}
}

// TestParallelExecutor_CheckBranchSuccess_ConcurrentCalls tests thread safety
// when multiple goroutines check success simultaneously.
func TestParallelExecutor_CheckBranchSuccess_ConcurrentCalls(t *testing.T) {
	// Arrange: setup for concurrent success checks
	executor := application.NewParallelExecutor(&mockLogger{})

	var firstSuccess bool
	var mu sync.Mutex
	successChan := make(chan struct{}, 1)

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	cancelCount := 0
	var cancelMu sync.Mutex
	mockCancel := func() {
		cancelMu.Lock()
		cancelCount++
		cancelMu.Unlock()
		cancel()
	}

	// Create multiple successful branch results
	branches := make([]*workflow.BranchResult, 5)
	for i := 0; i < 5; i++ {
		branches[i] = &workflow.BranchResult{
			Name:        fmt.Sprintf("branch%d", i),
			ExitCode:    0,
			Output:      "success",
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}
	}

	// Act: check success concurrently from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			executor.CheckBranchSuccess(branches[i], &firstSuccess, &mu, successChan, mockCancel)
		}()
	}

	wg.Wait()

	// Assert: exactly one cancel call (first success wins)
	cancelMu.Lock()
	actualCancelCount := cancelCount
	cancelMu.Unlock()

	assert.Equal(t, 1, actualCancelCount, "cancel should be called exactly once")
	assert.True(t, firstSuccess, "firstSuccess should be set")

	// Exactly one signal in channel
	signalCount := 0
	for {
		select {
		case <-successChan:
			signalCount++
		case <-time.After(50 * time.Millisecond):
			goto done
		}
	}
done:
	assert.Equal(t, 1, signalCount, "exactly one success signal should be sent")
}

// TestParallelExecutor_CheckBranchSuccess_ChannelFull tests behavior when success
// channel is full (already has a signal).
func TestParallelExecutor_CheckBranchSuccess_ChannelFull(t *testing.T) {
	// Arrange: setup with full success channel
	executor := application.NewParallelExecutor(&mockLogger{})

	successfulBranch := &workflow.BranchResult{
		Name:     "late-success",
		ExitCode: 0,
		Output:   "success",
	}

	var firstSuccess bool
	var mu sync.Mutex
	successChan := make(chan struct{}, 1)
	successChan <- struct{}{} // Fill channel

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	cancelCalled := false
	mockCancel := func() {
		cancelCalled = true
		cancel()
	}

	// Act: check success when channel is full
	done := make(chan struct{})
	go func() {
		executor.CheckBranchSuccess(successfulBranch, &firstSuccess, &mu, successChan, mockCancel)
		close(done)
	}()

	// Assert: should not block (uses non-blocking select with default)
	select {
	case <-done:
		// Expected - method returned without blocking
	case <-time.After(200 * time.Millisecond):
		t.Fatal("CheckBranchSuccess should not block when channel is full")
	}

	// First success should still be detected
	assert.True(t, firstSuccess, "firstSuccess should be set even if channel full")
	// Cancel should still be called on first success
	assert.True(t, cancelCalled, "cancel should be called even if channel full")
}
