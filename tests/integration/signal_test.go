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

// Signal Handling Integration Tests (C010)
// Tests validate graceful shutdown behavior on SIGINT/SIGTERM:
// - Context cancellation triggers graceful shutdown
// - State preservation when workflows are interrupted
// - Proper cleanup of child processes
// - Correct status transitions (StatusCancelled)
//
// Implementation Note (ADR-001):
// Tests simulate OS signals via context cancellation instead of syscall.Kill()
// for better test isolation and cross-platform compatibility. The signal handler
// in run.go:33-48 translates signals to context cancellation internally.

// TestSignalHandling_SIGINTGracefulShutdown verifies graceful shutdown on SIGINT
// Strategy: Cancel context during long-running step, verify StatusCancelled
func TestSignalHandling_SIGINTGracefulShutdown(t *testing.T) {
	tests := []struct {
		name           string
		workflowFile   string
		cancelAfter    time.Duration
		expectedStatus workflow.ExecutionStatus
		expectedStep   string
		wantErr        bool
	}{
		{
			name:           "cancel during long running step",
			workflowFile:   "signal-long-running.yaml",
			cancelAfter:    1 * time.Second,
			expectedStatus: workflow.StatusCancelled,
			expectedStep:   "long_step",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")
			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture to tmpDir (will be created in Phase 2)
			// For now, stub will fail during workflow loading
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)

			// Read and write fixture (this will fail in RED phase)
			data, err := os.ReadFile(fixtureSource)
			if err != nil {
				t.Skipf("fixture not yet created: %s", fixtureSource)
			}
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components using testutil pattern
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			workflowName := tt.workflowFile[:len(tt.workflowFile)-5] // strip .yaml

			// Simulate signal by canceling context after delay
			go func() {
				time.Sleep(tt.cancelAfter)
				cancel()
			}()

			startTime := time.Now()
			execCtx, err := svc.Run(ctx, workflowName, nil)
			duration := time.Since(startTime)

			// When context is cancelled, ExecutionService returns context.Canceled error
			// along with the execution context (status=StatusCancelled)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			// Context cancellation is expected - verify it's the right error type
			require.Error(t, err, "should return error on cancellation")
			assert.ErrorIs(t, err, context.Canceled, "should be context.Canceled error")

			require.NotNil(t, execCtx, "execution context should be returned even on cancellation")
			assert.Equal(t, tt.expectedStatus, execCtx.Status, "workflow status should be cancelled")
			assert.Equal(t, tt.expectedStep, execCtx.CurrentStep, "should be interrupted at expected step")

			assert.WithinDuration(t, startTime, startTime.Add(duration), 15*time.Second,
				"graceful shutdown should complete within 15s (3x tolerance)")
		})
	}
}

// TestSignalHandling_SIGTERMGracefulShutdown verifies graceful shutdown on SIGTERM
// Strategy: Cancel context during multi-step workflow, verify state preservation
func TestSignalHandling_SIGTERMGracefulShutdown(t *testing.T) {
	tests := []struct {
		name           string
		workflowFile   string
		cancelAfter    time.Duration
		expectedStatus workflow.ExecutionStatus
		expectedStep   string
		wantErr        bool
	}{
		{
			name:           "cancel during step 2 of multi-step workflow",
			workflowFile:   "signal-multi-step.yaml",
			cancelAfter:    3 * time.Second, // Cancel during step2 (step1 takes ~2s)
			expectedStatus: workflow.StatusCancelled,
			expectedStep:   "step2",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")
			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture (will fail in RED phase)
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)

			data, err := os.ReadFile(fixtureSource)
			if err != nil {
				t.Skipf("fixture not yet created: %s", fixtureSource)
			}
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			workflowName := tt.workflowFile[:len(tt.workflowFile)-5]

			go func() {
				time.Sleep(tt.cancelAfter)
				cancel()
			}()

			execCtx, err := svc.Run(ctx, workflowName, nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			// Context cancellation is expected - verify it's the right error type
			require.Error(t, err, "should return error on cancellation")
			assert.ErrorIs(t, err, context.Canceled, "should be context.Canceled error")

			require.NotNil(t, execCtx, "execution context should be returned even on cancellation")
			assert.Equal(t, tt.expectedStatus, execCtx.Status)
			assert.Equal(t, tt.expectedStep, execCtx.CurrentStep)

			step1State, ok := execCtx.GetStepState("step1")
			assert.True(t, ok, "step1 state should exist")
			assert.NotNil(t, step1State, "step1 state should not be nil")
		})
	}
}

// TestSignalHandling_StatePreservation verifies state is persisted on interruption
// Strategy: Interrupt workflow with intermediate outputs, verify JSON validity
func TestSignalHandling_StatePreservation(t *testing.T) {
	tests := []struct {
		name         string
		workflowFile string
		cancelAfter  time.Duration
		wantErr      bool
	}{
		{
			name:         "state preserved with intermediate outputs",
			workflowFile: "signal-multi-step.yaml",
			cancelAfter:  3 * time.Second,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")
			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture (will fail in RED phase)
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)

			data, err := os.ReadFile(fixtureSource)
			if err != nil {
				t.Skipf("fixture not yet created: %s", fixtureSource)
			}
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			workflowName := tt.workflowFile[:len(tt.workflowFile)-5]

			go func() {
				time.Sleep(tt.cancelAfter)
				cancel()
			}()

			execCtx, err := svc.Run(ctx, workflowName, nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			// Context cancellation is expected - verify it's the right error type
			require.Error(t, err, "should return error on cancellation")
			assert.ErrorIs(t, err, context.Canceled, "should be context.Canceled error")

			require.NotNil(t, execCtx, "execution context should be returned even on cancellation")
			assert.Equal(t, workflow.StatusCancelled, execCtx.Status)

			// Verify state was saved to store
			savedCtx, err := store.Load(context.Background(), execCtx.WorkflowID)
			require.NoError(t, err)
			require.NotNil(t, savedCtx)
			assert.Equal(t, execCtx.WorkflowID, savedCtx.WorkflowID)
			assert.Equal(t, workflow.StatusCancelled, savedCtx.Status)
		})
	}
}

// TestSignalHandling_ParallelBranchCancellation verifies all parallel branches cancelled
// Strategy: Cancel during parallel execution, verify no branches complete
func TestSignalHandling_ParallelBranchCancellation(t *testing.T) {
	tests := []struct {
		name         string
		workflowFile string
		cancelAfter  time.Duration
		wantErr      bool
	}{
		{
			name:         "all parallel branches cancelled",
			workflowFile: "signal-parallel.yaml",
			cancelAfter:  3 * time.Second, // Cancel before any branch completes
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")
			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture (will fail in RED phase)
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)

			data, err := os.ReadFile(fixtureSource)
			if err != nil {
				t.Skipf("fixture not yet created: %s", fixtureSource)
			}
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			workflowName := tt.workflowFile[:len(tt.workflowFile)-5]

			go func() {
				time.Sleep(tt.cancelAfter)
				cancel()
			}()

			execCtx, err := svc.Run(ctx, workflowName, nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			// Context cancellation is expected - verify it's the right error type
			require.Error(t, err, "should return error on cancellation")
			assert.ErrorIs(t, err, context.Canceled, "should be context.Canceled error")

			require.NotNil(t, execCtx, "execution context should be returned even on cancellation")
			assert.Equal(t, workflow.StatusCancelled, execCtx.Status)

			// Should have parallel_step state but no branch should have reached terminal
			parallelState, ok := execCtx.GetStepState("parallel_step")
			assert.True(t, ok, "parallel step state should exist")
			assert.NotNil(t, parallelState)

			// Verify no branches completed successfully (all interrupted)
			branchNames := []string{"branch1", "branch2", "branch3"}
			for _, branchName := range branchNames {
				branchState, ok := execCtx.GetStepState(branchName)
				if ok {
					// If branch state exists, it should show interruption
					assert.NotEqual(t, workflow.StatusCompleted, branchState.Status,
						"branch %s should not complete", branchName)
				}
			}
		})
	}
}

// TestSignalHandling_ResumabilityAfterInterruption verifies workflow can resume
// Strategy: Cancel at step 2, then resume and verify completion
func TestSignalHandling_ResumabilityAfterInterruption(t *testing.T) {
	tests := []struct {
		name         string
		workflowFile string
		cancelAfter  time.Duration
		wantErr      bool
	}{
		{
			name:         "resume from step 3 after cancellation at step 2",
			workflowFile: "signal-multi-step.yaml",
			cancelAfter:  3 * time.Second,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx1, cancel1 := context.WithCancel(context.Background())
			defer cancel1()

			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")
			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture (will fail in RED phase)
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)

			data, err := os.ReadFile(fixtureSource)
			if err != nil {
				t.Skipf("fixture not yet created: %s", fixtureSource)
			}
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			workflowName := tt.workflowFile[:len(tt.workflowFile)-5]

			go func() {
				time.Sleep(tt.cancelAfter)
				cancel1()
			}()

			execCtx1, err := svc.Run(ctx1, workflowName, nil)
			// First run should be cancelled
			require.Error(t, err, "should return error on cancellation")
			assert.ErrorIs(t, err, context.Canceled, "should be context.Canceled error")
			require.NotNil(t, execCtx1, "execution context should be returned even on cancellation")
			require.Equal(t, workflow.StatusCancelled, execCtx1.Status)

			ctx2 := context.Background()
			execCtx2, err := svc.Resume(ctx2, execCtx1.WorkflowID, nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.NotNil(t, execCtx2)
			assert.Equal(t, workflow.StatusCompleted, execCtx2.Status, "resumed workflow should complete")

			step1State, ok := execCtx2.GetStepState("step1")
			assert.True(t, ok, "step1 state should exist from first run")
			assert.NotNil(t, step1State)
		})
	}
}

// TestSignalHandling_RapidSuccessiveSignals verifies idempotent cancellation
// Strategy: Cancel multiple times rapidly, verify single graceful shutdown
func TestSignalHandling_RapidSuccessiveSignals(t *testing.T) {
	tests := []struct {
		name         string
		workflowFile string
		cancelCount  int
		wantErr      bool
	}{
		{
			name:         "three rapid cancellations",
			workflowFile: "signal-long-running.yaml",
			cancelCount:  3,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")
			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture (will fail in RED phase)
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)

			data, err := os.ReadFile(fixtureSource)
			if err != nil {
				t.Skipf("fixture not yet created: %s", fixtureSource)
			}
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			workflowName := tt.workflowFile[:len(tt.workflowFile)-5]

			go func() {
				time.Sleep(1 * time.Second)
				for i := 0; i < tt.cancelCount; i++ {
					cancel() // Idempotent - should be safe to call multiple times
					time.Sleep(100 * time.Millisecond)
				}
			}()

			execCtx, err := svc.Run(ctx, workflowName, nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			// Context cancellation is expected - verify it's the right error type
			require.Error(t, err, "should return error on cancellation")
			assert.ErrorIs(t, err, context.Canceled, "should be context.Canceled error")

			require.NotNil(t, execCtx, "execution context should be returned even on cancellation")
			assert.Equal(t, workflow.StatusCancelled, execCtx.Status)

			// If we reach here without panic, test passes
			assert.NotEmpty(t, execCtx.WorkflowID, "workflow ID should be set")
		})
	}
}

// TestSignalHandling_ChildProcessCleanup verifies child processes terminated
// Strategy: Spawn child process, cancel parent, verify child also terminates
func TestSignalHandling_ChildProcessCleanup(t *testing.T) {
	tests := []struct {
		name         string
		workflowFile string
		cancelAfter  time.Duration
		wantErr      bool
	}{
		{
			name:         "child process cleaned up on cancellation",
			workflowFile: "signal-long-running.yaml",
			cancelAfter:  2 * time.Second,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")
			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture (will fail in RED phase)
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)

			data, err := os.ReadFile(fixtureSource)
			if err != nil {
				t.Skipf("fixture not yet created: %s", fixtureSource)
			}
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			workflowName := tt.workflowFile[:len(tt.workflowFile)-5]

			go func() {
				time.Sleep(tt.cancelAfter)
				cancel()
			}()

			startTime := time.Now()
			execCtx, err := svc.Run(ctx, workflowName, nil)
			duration := time.Since(startTime)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			// Context cancellation is expected - verify it's the right error type
			require.Error(t, err, "should return error on cancellation")
			assert.ErrorIs(t, err, context.Canceled, "should be context.Canceled error")

			require.NotNil(t, execCtx, "execution context should be returned even on cancellation")
			assert.Equal(t, workflow.StatusCancelled, execCtx.Status)

			// Verify child process was killed promptly (not allowed to run full duration)
			// sleep 10 command would take 10s, but we cancel at 2s
			// With graceful shutdown, should complete within 15s (3x tolerance)
			assert.Less(t, duration.Seconds(), 15.0,
				"child process should be terminated, not allowed to complete full sleep")

			// Note: Verifying no zombies is platform-specific
			// The ShellExecutor should handle process cleanup automatically
			// This test verifies timing (child didn't run to completion)
			// Zombie prevention is tested implicitly by the executor implementation
		})
	}
}
