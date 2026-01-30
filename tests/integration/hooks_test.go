//go:build integration

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/testutil"
)

// =============================================================================
// Hook Lifecycle Integration Tests (C011 - US1)
// Tests validate hook execution order and variable interpolation:
// - workflow_start executes before first step
// - workflow_end executes after successful completion
// - workflow_error executes on failure with error context
// - Step pre/post hooks execute in correct order
// - Hook failures propagate and abort workflow
// - Variable interpolation works in all hook types
//
// Implementation Note (ADR-003):
// Tests use file-based verification to avoid flaky timing assertions.
// Hooks write to a shared log file, tests read file to assert order.
// =============================================================================

// TestHooks_WorkflowStartEnd_Integration verifies workflow_start/workflow_end execution order
// Feature: C011 - Task T004
// Strategy: Run workflow with hooks writing to log file, verify order
func TestHooks_WorkflowStartEnd_Integration(t *testing.T) {

	tests := []struct {
		name         string
		workflowFile string
		wantLogOrder []string // Expected log entries in order
		wantErr      bool
	}{
		{
			name:         "workflow_start before steps, workflow_end after all steps",
			workflowFile: "hooks-lifecycle.yaml",
			wantLogOrder: []string{
				"WORKFLOW_START",
				"STEP1_PRE",
				"STEP1_COMMAND",
				"STEP1_POST",
				"STEP2_PRE",
				"STEP2_COMMAND",
				"STEP2_POST",
				"WORKFLOW_END",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")
			logFile := filepath.Join(tmpDir, "hooks.log")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture to tmpDir
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)

			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err, "fixture file should exist")
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components using testutil pattern (C007)
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			// Set HOOK_LOG_FILE environment variable for hooks
			t.Setenv("HOOK_LOG_FILE", logFile)

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow
			workflowName := strings.TrimSuffix(tt.workflowFile, ".yaml")
			execCtx, err := svc.Run(ctx, workflowName, nil)

			// Assert - Layer 1: Error checking
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err, "workflow should complete successfully")

			// Assert - Layer 2: Status verification
			require.NotNil(t, execCtx, "execution context should be returned")
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "workflow should complete")

			// Assert - Layer 3: Hook execution order verification
			logData, err := os.ReadFile(logFile)
			require.NoError(t, err, "log file should exist")

			logLines := strings.Split(strings.TrimSpace(string(logData)), "\n")
			require.Len(t, logLines, len(tt.wantLogOrder), "log should have expected number of entries")

			for i, expectedLine := range tt.wantLogOrder {
				assert.Equal(t, expectedLine, logLines[i], "log line %d should match", i)
			}
		})
	}
}

// TestHooks_WorkflowError_Integration verifies workflow_error hook execution on failure
// Feature: C011 - Task T005
// Strategy: Run workflow that fails, verify workflow_error hook receives error context
func TestHooks_WorkflowError_Integration(t *testing.T) {

	tests := []struct {
		name            string
		workflowYAML    string // Inline YAML for custom error scenarios
		wantLogContains []string
		wantStatus      workflow.ExecutionStatus
		wantErr         bool
	}{
		{
			name: "workflow_error receives error.type and error.message",
			workflowYAML: `name: hooks-error-test
version: "1.0.0"
hooks:
  workflow_error:
    - command: echo "ERROR_TYPE={{.error.Type}}" >> {{.env.HOOK_LOG_FILE}}
    - command: echo "ERROR_MESSAGE={{.error.Message}}" >> {{.env.HOOK_LOG_FILE}}
states:
  initial: fail_step
  fail_step:
    type: step
    command: exit 42
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`,
			wantLogContains: []string{
				"ERROR_TYPE=",
				"ERROR_MESSAGE=",
			},
			wantStatus: workflow.StatusFailed,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment with inline YAML
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")
			logFile := filepath.Join(tmpDir, "hooks.log")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Write inline workflow
			workflowPath := filepath.Join(workflowsDir, "test-error.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			t.Setenv("HOOK_LOG_FILE", logFile)

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute failing workflow
			execCtx, err := svc.Run(ctx, "test-error", nil)

			// Assert - Layer 1: Error checking
			if tt.wantErr {
				require.Error(t, err, "workflow should fail")
			} else {
				require.NoError(t, err)
			}

			// Assert - Layer 2: Status verification
			require.NotNil(t, execCtx, "execution context should be returned")
			assert.Equal(t, tt.wantStatus, execCtx.Status, "workflow should have expected status")

			// Assert - Layer 3: workflow_error hook execution verification
			logData, err := os.ReadFile(logFile)
			require.NoError(t, err, "log file should exist")
			logContent := string(logData)

			for _, expectedSubstr := range tt.wantLogContains {
				assert.Contains(t, logContent, expectedSubstr, "log should contain %s", expectedSubstr)
			}
		})
	}
}

// TestHooks_StepPrePost_Integration verifies step pre/post hook execution order
// Feature: C011 - Task T005
// Strategy: Run workflow with step hooks, verify pre executes before command, post after
func TestHooks_StepPrePost_Integration(t *testing.T) {

	tests := []struct {
		name         string
		workflowFile string
		wantLogOrder []string
		wantErr      bool
	}{
		{
			name:         "step pre before command, post after command",
			workflowFile: "hooks-lifecycle.yaml",
			wantLogOrder: []string{
				"WORKFLOW_START",
				"STEP1_PRE",     // Step 1 pre hook
				"STEP1_COMMAND", // Step 1 command
				"STEP1_POST",    // Step 1 post hook
				"STEP2_PRE",     // Step 2 pre hook
				"STEP2_COMMAND", // Step 2 command
				"STEP2_POST",    // Step 2 post hook
				"WORKFLOW_END",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			logFile := filepath.Join(tmpDir, "hooks.log")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			// Copy fixture
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)
			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			t.Setenv("HOOK_LOG_FILE", logFile)

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow
			workflowName := strings.TrimSuffix(tt.workflowFile, ".yaml")
			execCtx, err := svc.Run(ctx, workflowName, nil)

			// Assert - Layer 1: Error
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Assert - Layer 2: Status
			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			// Assert - Layer 3: Hook execution order
			logData, err := os.ReadFile(logFile)
			require.NoError(t, err)
			logLines := strings.Split(strings.TrimSpace(string(logData)), "\n")
			require.Len(t, logLines, len(tt.wantLogOrder))

			for i, expectedLine := range tt.wantLogOrder {
				assert.Equal(t, expectedLine, logLines[i], "log line %d", i)
			}
		})
	}
}

// TestHooks_FailurePropagation_Integration verifies hook failures abort workflow
// Feature: C011 - Task T006
// Strategy: Run workflow with failing hook, verify workflow aborts and main command doesn't execute
func TestHooks_FailurePropagation_Integration(t *testing.T) {

	tests := []struct {
		name               string
		workflowFile       string
		wantLogContains    []string
		wantLogNotContains []string
		wantErr            bool
	}{
		{
			name:         "hook failure aborts workflow",
			workflowFile: "hooks-failure.yaml",
			wantLogContains: []string{
				"WORKFLOW_START",
				"FAILING_HOOK",
			},
			wantLogNotContains: []string{
				"STEP1_SHOULD_NOT_EXECUTE", // Main step should not run
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			logFile := filepath.Join(tmpDir, "hooks.log")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			// Copy fixture
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)
			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			t.Setenv("HOOK_LOG_FILE", logFile)

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow with failing hook
			workflowName := strings.TrimSuffix(tt.workflowFile, ".yaml")
			execCtx, err := svc.Run(ctx, workflowName, nil)

			// Assert - Layer 1: Error checking
			if tt.wantErr {
				require.Error(t, err, "workflow should fail due to hook failure")
			}

			// Assert - Layer 2: Status verification
			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusFailed, execCtx.Status, "workflow should be marked as failed")

			// Assert - Layer 3: Hook failure propagation verification
			logData, err := os.ReadFile(logFile)
			require.NoError(t, err)
			logContent := string(logData)

			// Verify expected log entries exist
			for _, expectedSubstr := range tt.wantLogContains {
				assert.Contains(t, logContent, expectedSubstr, "log should contain %s", expectedSubstr)
			}

			// Verify main command did NOT execute (hook aborted workflow)
			for _, unexpectedSubstr := range tt.wantLogNotContains {
				assert.NotContains(t, logContent, unexpectedSubstr, "log should NOT contain %s", unexpectedSubstr)
			}
		})
	}
}

// TestHooks_VariableInterpolation_Integration verifies variable interpolation in hooks
// Feature: C011 - Task T006
// Strategy: Run workflow with variables in hooks, verify {{workflow.id}}, {{inputs.x}}, {{states.y.output}} interpolated
func TestHooks_VariableInterpolation_Integration(t *testing.T) {

	tests := []struct {
		name            string
		workflowFile    string
		inputs          map[string]any
		wantLogContains []string
		wantErr         bool
	}{
		{
			name:         "workflow.id and inputs interpolated in hooks",
			workflowFile: "hooks-variables.yaml",
			inputs: map[string]any{
				"test_input": "my_test_value",
			},
			wantLogContains: []string{
				"START workflow_id=",        // workflow.id should be present
				"START input=my_test_value", // inputs.test_input interpolated
				"PRE workflow_id=",          // Step pre hook interpolation
				"PRE input=my_test_value",
				"POST workflow_id=",                 // Step post hook interpolation
				"POST output=step1_captured_output", // states.step1.output interpolated
				"END workflow_id=",                  // workflow_end interpolation
				"END step1_output=step1_captured_output",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			logFile := filepath.Join(tmpDir, "hooks.log")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			// Copy fixture
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)
			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			t.Setenv("HOOK_LOG_FILE", logFile)

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow with inputs
			workflowName := strings.TrimSuffix(tt.workflowFile, ".yaml")
			execCtx, err := svc.Run(ctx, workflowName, tt.inputs)

			// Assert - Layer 1: Error checking
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Assert - Layer 2: Status verification
			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			// Assert - Layer 3: Variable interpolation verification
			logData, err := os.ReadFile(logFile)
			require.NoError(t, err)
			logContent := string(logData)

			for _, expectedSubstr := range tt.wantLogContains {
				assert.Contains(t, logContent, expectedSubstr, "log should contain interpolated value: %s", expectedSubstr)
			}
		})
	}
}

// NOTE: Test for failOnError feature removed - feature not implemented and out of scope for C011
// The C011 specification focuses on testing existing hook functionality (lifecycle, error propagation),
// not adding new features like per-hook failure handling. If failOnError is needed in the future,
// it should be implemented as a separate feature (requires domain model changes, YAML parsing, executor logic).
