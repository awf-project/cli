//go:build integration

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	workflow "github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/testutil"
)

// =============================================================================
// Parallel Execution Integration Tests (C009)
// Tests validate CLI-level parallel execution for all strategies:
// - all_succeed: workflow fails if any branch fails
// - any_succeed: workflow succeeds if any branch succeeds
// - best_effort: all branches complete regardless of failures
// Also tests max_concurrent limit enforcement and failure propagation.
// =============================================================================

// TestParallelExecution_AllSucceedStrategy validates all_succeed strategy
// Strategy behavior: workflow fails if ANY branch fails
func TestParallelExecution_AllSucceedStrategy(t *testing.T) {
	tests := []struct {
		name           string
		workflowYAML   string
		expectedStatus workflow.ExecutionStatus
		expectedStep   string
		wantErr        bool
	}{
		{
			name: "all branches succeed",
			workflowYAML: `name: all-succeed-pass
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
      - branch_c
    strategy: all_succeed
    on_success: done
    on_failure: error
  branch_a:
    type: step
    command: echo "Branch A success"
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: echo "Branch B success"
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: echo "Branch C success"
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			wantErr:        false,
		},
		{
			name: "one branch fails",
			workflowYAML: `name: all-succeed-one-fail
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
      - branch_c
    strategy: all_succeed
    on_success: done
    on_failure: error
  branch_a:
    type: step
    command: echo "Branch A success"
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: exit 1
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: echo "Branch C success"
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "error",
			wantErr:        false,
		},
		{
			name: "all branches fail",
			workflowYAML: `name: all-succeed-all-fail
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
      - branch_c
    strategy: all_succeed
    on_success: done
    on_failure: error
  branch_a:
    type: step
    command: exit 1
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: exit 2
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: exit 3
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "error",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			require.NoError(t, os.WriteFile(wfPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components using testutil pattern
			repo := repository.NewYAMLRepository(tmpDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow
			execCtx, err := svc.Run(ctx, "workflow", nil)

			// Assert - Layer 1: Error checking
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Assert - Layer 2: Status verification
			require.NotNil(t, execCtx)
			assert.Equal(t, tt.expectedStatus, execCtx.Status, "workflow status mismatch")
			assert.Equal(t, tt.expectedStep, execCtx.CurrentStep, "final step mismatch")

			// Assert - Layer 3: Output verification (all branches must have state)
			parallelState, ok := execCtx.GetStepState("parallel_step")
			require.True(t, ok, "parallel step state not found")
			require.NotNil(t, parallelState)

			// Verify all branches were executed
			branchNames := []string{"branch_a", "branch_b", "branch_c"}
			for _, branchName := range branchNames {
				branchState, ok := execCtx.GetStepState(branchName)
				assert.True(t, ok, "branch %s state not found", branchName)
				assert.NotNil(t, branchState, "branch %s state is nil", branchName)
			}
		})
	}
}

// TestParallelExecution_AnySucceedStrategy validates any_succeed strategy
// Strategy behavior: workflow succeeds if AT LEAST ONE branch succeeds
func TestParallelExecution_AnySucceedStrategy(t *testing.T) {
	tests := []struct {
		name           string
		workflowYAML   string
		expectedStatus workflow.ExecutionStatus
		expectedStep   string
		wantErr        bool
	}{
		{
			name: "all branches succeed",
			workflowYAML: `name: any-succeed-all-pass
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
      - branch_c
    strategy: any_succeed
    on_success: done
    on_failure: error
  branch_a:
    type: step
    command: echo "Branch A success"
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: echo "Branch B success"
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: echo "Branch C success"
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			wantErr:        false,
		},
		{
			name: "mixed results - one success two failures",
			workflowYAML: `name: any-succeed-mixed
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
      - branch_c
    strategy: any_succeed
    on_success: done
    on_failure: error
  branch_a:
    type: step
    command: exit 1
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: echo "Branch B success"
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: exit 2
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			wantErr:        false,
		},
		{
			name: "all branches fail",
			workflowYAML: `name: any-succeed-all-fail
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
      - branch_c
    strategy: any_succeed
    on_success: done
    on_failure: error
  branch_a:
    type: step
    command: exit 1
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: exit 2
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: exit 3
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "error",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			require.NoError(t, os.WriteFile(wfPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(tmpDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow
			execCtx, err := svc.Run(ctx, "workflow", nil)

			// Assert - Layer 1: Error checking
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Assert - Layer 2: Status verification
			require.NotNil(t, execCtx)
			assert.Equal(t, tt.expectedStatus, execCtx.Status, "workflow status mismatch")
			assert.Equal(t, tt.expectedStep, execCtx.CurrentStep, "final step mismatch")

			// Assert - Layer 3: Output verification
			parallelState, ok := execCtx.GetStepState("parallel_step")
			require.True(t, ok, "parallel step state not found")
			require.NotNil(t, parallelState)

			// Verify all branches were attempted
			branchNames := []string{"branch_a", "branch_b", "branch_c"}
			for _, branchName := range branchNames {
				branchState, ok := execCtx.GetStepState(branchName)
				assert.True(t, ok, "branch %s state not found", branchName)
				assert.NotNil(t, branchState, "branch %s state is nil", branchName)
			}
		})
	}
}

// TestParallelExecution_BestEffortStrategy validates best_effort strategy
// Strategy behavior: workflow completes REGARDLESS of branch failures
func TestParallelExecution_BestEffortStrategy(t *testing.T) {
	tests := []struct {
		name           string
		workflowYAML   string
		expectedStatus workflow.ExecutionStatus
		expectedStep   string
		wantErr        bool
	}{
		{
			name: "all branches succeed",
			workflowYAML: `name: best-effort-all-pass
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
      - branch_c
    strategy: best_effort
    on_success: done
    on_failure: error
  branch_a:
    type: step
    command: echo "Branch A success"
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: echo "Branch B success"
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: echo "Branch C success"
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			wantErr:        false,
		},
		{
			name: "all branches fail - workflow still completes",
			workflowYAML: `name: best-effort-all-fail
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
      - branch_c
    strategy: best_effort
    on_success: done
    on_failure: error
  branch_a:
    type: step
    command: exit 1
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: exit 2
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: exit 3
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			wantErr:        false,
		},
		{
			name: "mixed results - workflow completes",
			workflowYAML: `name: best-effort-mixed
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
      - branch_c
    strategy: best_effort
    on_success: done
    on_failure: error
  branch_a:
    type: step
    command: echo "Branch A success"
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: exit 1
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: echo "Branch C success"
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			require.NoError(t, os.WriteFile(wfPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(tmpDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow
			execCtx, err := svc.Run(ctx, "workflow", nil)

			// Assert - Layer 1: Error checking
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Assert - Layer 2: Status verification
			require.NotNil(t, execCtx)
			assert.Equal(t, tt.expectedStatus, execCtx.Status, "workflow status mismatch")
			assert.Equal(t, tt.expectedStep, execCtx.CurrentStep, "final step mismatch")

			// Assert - Layer 3: Output verification
			parallelState, ok := execCtx.GetStepState("parallel_step")
			require.True(t, ok, "parallel step state not found")
			require.NotNil(t, parallelState)

			// Verify all branches completed (critical for best_effort)
			branchNames := []string{"branch_a", "branch_b", "branch_c"}
			for _, branchName := range branchNames {
				branchState, ok := execCtx.GetStepState(branchName)
				assert.True(t, ok, "branch %s state not found", branchName)
				assert.NotNil(t, branchState, "branch %s state is nil", branchName)
				// Verify branch reached a terminal state (completed or failed)
				assert.NotEqual(t, workflow.StatusRunning, branchState.Status, "branch %s still running", branchName)
			}
		})
	}
}

// TestParallelExecution_MaxConcurrent validates max_concurrent limit enforcement
// Uses timing-based assertions with ADR-004: 3x margin for CI variability
func TestParallelExecution_MaxConcurrent(t *testing.T) {
	tests := []struct {
		name          string
		workflowYAML  string
		maxConcurrent int
		branches      int
		minDuration   time.Duration // expected minimum duration
		maxDuration   time.Duration // expected maximum duration (3x margin)
	}{
		{
			name: "max_concurrent=2 with 3 branches",
			workflowYAML: `name: max-concurrent-limited
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
      - branch_c
    strategy: all_succeed
    max_concurrent: 2
    on_success: done
    on_failure: error
  branch_a:
    type: step
    command: sleep 0.2
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: sleep 0.2
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: sleep 0.2
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			maxConcurrent: 2,
			branches:      3,
			// With max_concurrent=2: 2 run first (200ms), then 1 runs (200ms) = 400ms minimum
			minDuration: 400 * time.Millisecond,
			// 3x margin for CI variability per ADR-004
			maxDuration: 1200 * time.Millisecond,
		},
		{
			name: "no max_concurrent - unlimited parallelism",
			workflowYAML: `name: max-concurrent-unlimited
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
      - branch_c
    strategy: all_succeed
    on_success: done
    on_failure: error
  branch_a:
    type: step
    command: sleep 0.2
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: sleep 0.2
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: sleep 0.2
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			maxConcurrent: 0,
			branches:      3,
			// All 3 run in parallel = 200ms minimum
			minDuration: 200 * time.Millisecond,
			// 3x margin for CI variability per ADR-004
			maxDuration: 600 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			require.NoError(t, os.WriteFile(wfPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(tmpDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow with timing measurement
			startTime := time.Now()
			execCtx, err := svc.Run(ctx, "workflow", nil)
			elapsed := time.Since(startTime)

			// Assert - Layer 1: Error checking
			require.NoError(t, err)
			require.NotNil(t, execCtx)

			// Assert - Layer 2: Status verification
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "workflow status mismatch")
			assert.Equal(t, "done", execCtx.CurrentStep, "final step mismatch")

			// Assert - Layer 3: Timing verification (validates concurrency limit)
			verifyTimingConstraints(t, elapsed, tt.minDuration, tt.maxDuration)

			// Verify all branches completed
			branchNames := []string{"branch_a", "branch_b", "branch_c"}
			for _, branchName := range branchNames {
				branchState, ok := execCtx.GetStepState(branchName)
				assert.True(t, ok, "branch %s state not found", branchName)
				assert.Equal(t, workflow.StatusCompleted, branchState.Status, "branch %s not completed", branchName)
			}
		})
	}
}

// TestParallelExecution_FailurePropagation validates error capture and propagation
// Verifies that branch failures are captured in state and on_failure transitions work
func TestParallelExecution_FailurePropagation(t *testing.T) {
	tests := []struct {
		name           string
		workflowYAML   string
		failedBranch   string
		expectedStatus workflow.ExecutionStatus
		expectedStep   string
		expectedCode   int
		wantErr        bool
	}{
		{
			name: "branch error captured in ExecutionContext.States",
			workflowYAML: `name: failure-propagation-capture
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_success
      - branch_failure
    strategy: all_succeed
    on_success: done
    on_failure: error
  branch_success:
    type: step
    command: echo "success"
    on_success: done
    on_failure: error
  branch_failure:
    type: step
    command: exit 42
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			failedBranch:   "branch_failure",
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "error",
			expectedCode:   42,
			wantErr:        false,
		},
		{
			name: "on_failure transition respected in parallel context",
			workflowYAML: `name: failure-propagation-transition
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_a
      - branch_b
    strategy: all_succeed
    on_success: success_state
    on_failure: failure_handler
  branch_a:
    type: step
    command: echo "A success"
    on_success: success_state
    on_failure: failure_handler
  branch_b:
    type: step
    command: exit 1
    on_success: success_state
    on_failure: failure_handler
  failure_handler:
    type: step
    command: echo "handling failure"
    on_success: done
    on_failure: error
  success_state:
    type: terminal
    status: success
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			failedBranch:   "branch_b",
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			expectedCode:   1,
			wantErr:        false,
		},
		{
			name: "error details preserved in StepState",
			workflowYAML: `name: failure-propagation-details
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - branch_detailed
    strategy: all_succeed
    on_success: done
    on_failure: error
  branch_detailed:
    type: step
    command: sh -c 'echo "error details" >&2; exit 99'
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			failedBranch:   "branch_detailed",
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "error",
			expectedCode:   99,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			require.NoError(t, os.WriteFile(wfPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(tmpDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow
			execCtx, err := svc.Run(ctx, "workflow", nil)

			// Assert - Layer 1: Error checking
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Assert - Layer 2: Status verification
			require.NotNil(t, execCtx)
			assert.Equal(t, tt.expectedStatus, execCtx.Status, "workflow status mismatch")
			assert.Equal(t, tt.expectedStep, execCtx.CurrentStep, "final step mismatch")

			// Assert - Layer 3: Verify branch error details in state
			branchState, ok := execCtx.GetStepState(tt.failedBranch)
			require.True(t, ok, "failed branch state not found")
			require.NotNil(t, branchState, "failed branch state is nil")

			// Verify failure was captured
			assert.Equal(t, workflow.StatusFailed, branchState.Status, "branch should have failed status")
			assert.Equal(t, tt.expectedCode, branchState.ExitCode, "exit code mismatch")

			// Verify error message is populated
			assert.NotEmpty(t, branchState.Error, "error message should be populated")
		})
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// verifyTimingConstraints validates execution duration with CI-friendly margins
// ADR-004: Use 3x margin for CI variability to prevent flaky tests
func verifyTimingConstraints(t *testing.T, elapsed, minDuration, maxDuration time.Duration) {
	t.Helper()
	assert.GreaterOrEqual(t, elapsed, minDuration,
		"execution too fast (%v < %v) - concurrency limit not enforced", elapsed, minDuration)
	assert.LessOrEqual(t, elapsed, maxDuration,
		"execution too slow (%v > %v) - exceeded expected duration with 3x margin", elapsed, maxDuration)
}
