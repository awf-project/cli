//go:build integration

// Feature: C061
package execution_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupExecutionService(t *testing.T, tmpDir string) *application.ExecutionService {
	t.Helper()

	log := &conditionsMockLogger{}
	repo := repository.NewYAMLRepository(tmpDir)
	stateStore := store.NewJSONStore(filepath.Join(tmpDir, "states"))
	shellExec := executor.NewShellExecutor()
	parallelExec := application.NewParallelExecutor(log)
	resolver := interpolation.NewTemplateResolver()
	exprEval := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExec, log, infraExpr.NewExprValidator())
	return application.NewExecutionServiceWithEvaluator(
		wfSvc, shellExec, parallelExec, stateStore, log, resolver, nil, exprEval,
	)
}

func TestTransitionPriorityOnExecutionError_Integration(t *testing.T) {
	// A step that times out (execution error) with continue_on_error + transitions:
	// transitions must be evaluated before ContinueOnError fallback (ADR-001).
	tmpDir := t.TempDir()

	workflowYAML := `
name: transition-priority-test
states:
  initial: slow_step
  slow_step:
    type: step
    command: sleep 60
    timeout: 1
    continue_on_error: true
    transitions:
      - when: "true"
        goto: transition_target
    on_success: legacy_success
    on_failure: legacy_failure
  transition_target:
    type: terminal
    status: success
  legacy_success:
    type: terminal
    status: success
  legacy_failure:
    type: terminal
    status: failure
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowYAML), 0o644))

	execSvc := setupExecutionService(t, tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "workflow", nil)

	require.NoError(t, err)
	assert.Equal(t, "transition_target", execCtx.CurrentStep,
		"transitions should take priority over ContinueOnError on execution error")
}

func TestContinueOnErrorFallbackOnExecutionError_Integration(t *testing.T) {
	// When no transition matches on execution error, ContinueOnError falls back to OnSuccess.
	tmpDir := t.TempDir()

	workflowYAML := `
name: fallback-test
states:
  initial: slow_step
  slow_step:
    type: step
    command: sleep 60
    timeout: 1
    continue_on_error: true
    transitions:
      - when: "false"
        goto: never_reached
    on_success: continued
    on_failure: failure
  never_reached:
    type: terminal
    status: success
  continued:
    type: terminal
    status: success
  failure:
    type: terminal
    status: failure
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowYAML), 0o644))

	execSvc := setupExecutionService(t, tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "workflow", nil)

	require.NoError(t, err)
	assert.Equal(t, "continued", execCtx.CurrentStep,
		"with no matching transition and ContinueOnError=true, should follow OnSuccess")
}

func TestTimeoutWithDurationString_Integration(t *testing.T) {
	// Timeout field accepts Go duration strings (e.g., "1s") — validates Finding 1 + Finding 2.
	tmpDir := t.TempDir()

	workflowYAML := `
name: duration-timeout-test
states:
  initial: timed_step
  timed_step:
    type: step
    command: sleep 60
    timeout: "1s"
    on_success: success
    on_failure: timeout_handler
  timeout_handler:
    type: step
    command: echo "timed out"
    on_success: done
  success:
    type: terminal
    status: success
  done:
    type: terminal
    status: success
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowYAML), 0o644))

	execSvc := setupExecutionService(t, tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	execCtx, err := execSvc.Run(ctx, "workflow", nil)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)
	assert.Less(t, elapsed, 10*time.Second, "timeout should trigger quickly")

	stepState, ok := execCtx.GetStepState("timeout_handler")
	require.True(t, ok)
	assert.Contains(t, stepState.Output, "timed out")
}

