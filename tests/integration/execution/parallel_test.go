//go:build integration

package execution_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	workflow "github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/testutil/builders"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/tests/integration/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Parallel Execution Integration Tests (C009)
// Tests validate CLI-level parallel execution for all strategies:
// - all_succeed: workflow fails if any branch fails
// - any_succeed: workflow succeeds if any branch succeeds
// - best_effort: all branches complete regardless of failures
// Also tests max_concurrent limit enforcement and failure propagation.

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
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			require.NoError(t, os.WriteFile(wfPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components using testutil pattern
			repo := repository.NewYAMLRepository(tmpDir)
			store := mocks.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &testhelpers.MockLogger{}

			svc := builders.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			execCtx, err := svc.Run(ctx, "workflow", nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.NotNil(t, execCtx)
			assert.Equal(t, tt.expectedStatus, execCtx.Status)
			assert.Equal(t, tt.expectedStep, execCtx.CurrentStep)

			parallelState, ok := execCtx.GetStepState("parallel_step")
			require.True(t, ok)
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
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			require.NoError(t, os.WriteFile(wfPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(tmpDir)
			store := mocks.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &testhelpers.MockLogger{}

			svc := builders.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			execCtx, err := svc.Run(ctx, "workflow", nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.NotNil(t, execCtx)
			assert.Equal(t, tt.expectedStatus, execCtx.Status)
			assert.Equal(t, tt.expectedStep, execCtx.CurrentStep)

			parallelState, ok := execCtx.GetStepState("parallel_step")
			require.True(t, ok)
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
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			require.NoError(t, os.WriteFile(wfPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(tmpDir)
			store := mocks.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &testhelpers.MockLogger{}

			svc := builders.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			execCtx, err := svc.Run(ctx, "workflow", nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.NotNil(t, execCtx)
			assert.Equal(t, tt.expectedStatus, execCtx.Status)
			assert.Equal(t, tt.expectedStep, execCtx.CurrentStep)

			parallelState, ok := execCtx.GetStepState("parallel_step")
			require.True(t, ok)
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
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			require.NoError(t, os.WriteFile(wfPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(tmpDir)
			store := mocks.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &testhelpers.MockLogger{}

			svc := builders.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			startTime := time.Now()
			execCtx, err := svc.Run(ctx, "workflow", nil)
			elapsed := time.Since(startTime)

			require.NoError(t, err)
			require.NotNil(t, execCtx)

			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
			assert.Equal(t, "done", execCtx.CurrentStep)

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
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			require.NoError(t, os.WriteFile(wfPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(tmpDir)
			store := mocks.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &testhelpers.MockLogger{}

			svc := builders.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			execCtx, err := svc.Run(ctx, "workflow", nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.NotNil(t, execCtx)
			assert.Equal(t, tt.expectedStatus, execCtx.Status)
			assert.Equal(t, tt.expectedStep, execCtx.CurrentStep)

			branchState, ok := execCtx.GetStepState(tt.failedBranch)
			require.True(t, ok)
			require.NotNil(t, branchState)

			// Verify failure was captured
			assert.Equal(t, workflow.StatusFailed, branchState.Status)
			assert.Equal(t, tt.expectedCode, branchState.ExitCode)

			// Verify error message is populated
			assert.NotEmpty(t, branchState.Error)
		})
	}
}

// B004 Bug Fix: Parallel Branch Validation (Integration Level)
// Tests validate YAML → domain validation flow for parallel branch children
// without required transitions (exemption from transition requirements)

// TestParallelValidation_BranchChildrenWithoutTransitions validates B004 fix:
// parallel branch children should not require on_success/on_failure transitions.
// This integration test validates the full YAML parsing → domain validation pipeline.
func TestParallelValidation_BranchChildrenWithoutTransitions(t *testing.T) {
	// Given: A parallel workflow where branch children have no transitions
	workflowYAML := `name: b004-no-child-transitions
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - task_a
      - task_b
    strategy: all_succeed
    on_success: done
    on_failure: error
  task_a:
    type: step
    command: echo "Task A"
  task_b:
    type: step
    command: echo "Task B"
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	wfPath := filepath.Join(tmpDir, "workflow.yaml")
	require.NoError(t, os.WriteFile(wfPath, []byte(workflowYAML), 0o644))

	repo := repository.NewYAMLRepository(tmpDir)
	store := mocks.NewMockStateStore()
	exec := executor.NewShellExecutor()
	logger := mocks.NewMockLogger()

	svc := builders.NewExecutionServiceBuilder().
		WithWorkflowRepository(repo).
		WithStateStore(store).
		WithExecutor(exec).
		WithLogger(logger).
		Build()

	// When: The workflow is executed
	execCtx, err := svc.Run(ctx, "workflow", nil)

	// Then: No validation error, workflow completes successfully
	require.NoError(t, err)
	require.NotNil(t, execCtx)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)
}

// TestParallelValidation_AcceptanceCriteria validates all B004 acceptance criteria
// using a table-driven approach to ensure complete coverage.
func TestParallelValidation_AcceptanceCriteria(t *testing.T) {
	tests := []struct {
		name           string
		workflowYAML   string
		expectedStatus workflow.ExecutionStatus
		expectedStep   string
		wantErr        bool
		description    string
	}{
		{
			name: "AC1: parallel branch children without transitions (exemption)",
			workflowYAML: `name: ac1-no-transitions
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - task_a
      - task_b
    strategy: all_succeed
    on_success: done
    on_failure: error
  task_a:
    type: step
    command: echo "Task A"
  task_b:
    type: step
    command: echo "Task B"
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			wantErr:        false,
			description:    "Parallel branch children should not require transitions",
		},
		{
			name: "AC2: parallel branch children with optional transitions",
			workflowYAML: `name: ac2-optional-transitions
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - task_a
      - task_b
    strategy: all_succeed
    on_success: done
    on_failure: error
  task_a:
    type: step
    command: echo "Task A"
    on_success: done
    on_failure: error
  task_b:
    type: step
    command: echo "Task B"
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
			description:    "Parallel branch children may still define transitions",
		},
		{
			name: "AC2: mixed - some branches with transitions, some without",
			workflowYAML: `name: ac2-mixed-transitions
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - task_a
      - task_b
      - task_c
    strategy: all_succeed
    on_success: done
    on_failure: error
  task_a:
    type: step
    command: echo "Task A with transitions"
    on_success: done
    on_failure: error
  task_b:
    type: step
    command: echo "Task B without transitions"
  task_c:
    type: step
    command: echo "Task C with transitions"
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
			description:    "Mixed: some branches with transitions, some without",
		},
		{
			name: "AC4: regression - non-parallel command step requires transitions",
			workflowYAML: `name: ac4-regression-non-parallel
version: "1.0.0"
states:
  initial: regular_step
  regular_step:
    type: step
    command: echo "Regular step"
  done:
    type: terminal
    status: success`,
			expectedStatus: workflow.StatusPending,
			expectedStep:   "",
			wantErr:        true,
			description:    "Non-parallel command steps must still require transitions (regression test)",
		},
		{
			name: "AC3: existing workflows with transitions continue to validate",
			workflowYAML: `name: ac3-existing-workflow
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
    command: echo "Branch A"
    on_success: done
    on_failure: error
  branch_b:
    type: step
    command: echo "Branch B"
    on_success: done
    on_failure: error
  branch_c:
    type: step
    command: echo "Branch C"
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
			description:    "Existing workflows with transitions should continue working",
		},
		{
			name: "nested parallel: branch children without transitions",
			workflowYAML: `name: nested-parallel-no-transitions
version: "1.0.0"
states:
  initial: outer_parallel
  outer_parallel:
    type: parallel
    parallel:
      - inner_parallel
      - task_x
    strategy: all_succeed
    on_success: done
    on_failure: error
  inner_parallel:
    type: parallel
    parallel:
      - task_a
      - task_b
    strategy: all_succeed
  task_a:
    type: step
    command: echo "Inner Task A"
  task_b:
    type: step
    command: echo "Inner Task B"
  task_x:
    type: step
    command: echo "Task X"
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			wantErr:        false,
			description:    "Nested parallel: all branch children at all levels should not require transitions",
		},
		{
			name: "multiple parallel steps in sequence",
			workflowYAML: `name: sequential-parallel-steps
version: "1.0.0"
states:
  initial: parallel_1
  parallel_1:
    type: parallel
    parallel:
      - task_a
      - task_b
    strategy: all_succeed
    on_success: parallel_2
    on_failure: error
  task_a:
    type: step
    command: echo "Task A"
  task_b:
    type: step
    command: echo "Task B"
  parallel_2:
    type: parallel
    parallel:
      - task_c
      - task_d
    strategy: all_succeed
    on_success: done
    on_failure: error
  task_c:
    type: step
    command: echo "Task C"
  task_d:
    type: step
    command: echo "Task D"
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure`,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			wantErr:        false,
			description:    "Multiple parallel steps in sequence with transitionless branch children",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			require.NoError(t, os.WriteFile(wfPath, []byte(tt.workflowYAML), 0o644))

			repo := repository.NewYAMLRepository(tmpDir)
			store := mocks.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &testhelpers.MockLogger{}

			svc := builders.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			execCtx, err := svc.Run(ctx, "workflow", nil)

			if tt.wantErr {
				require.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)
				require.NotNil(t, execCtx)
				assert.Equal(t, tt.expectedStatus, execCtx.Status, tt.description)
				assert.Equal(t, tt.expectedStep, execCtx.CurrentStep, tt.description)
			}
		})
	}
}

// verifyTimingConstraints validates execution duration with CI-friendly margins
// ADR-004: Use 3x margin for CI variability to prevent flaky tests
func verifyTimingConstraints(t *testing.T, elapsed, minDuration, maxDuration time.Duration) {
	t.Helper()
	assert.GreaterOrEqual(t, elapsed, minDuration,
		"execution too fast (%v < %v) - concurrency limit not enforced", elapsed, minDuration)
	assert.LessOrEqual(t, elapsed, maxDuration,
		"execution too slow (%v > %v) - exceeded expected duration with 3x margin", elapsed, maxDuration)
}
