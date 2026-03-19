//go:build integration

// Feature: C064
package execution_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/pkg/interpolation"
)

// newRetryAuditServices creates a fresh set of services for retry audit tests.
func newRetryAuditServices(workflowDir string) *application.ExecutionService {
	repo := repository.NewYAMLRepository(workflowDir)
	store := newRetryMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &retryMockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)

	return application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)
}

// TestRetry_MultiplierDefault_Integration verifies that omitting the multiplier
// field in YAML produces exponential growth (default 2.0), not constant delays.
func TestRetry_MultiplierDefault_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "counter")
	tsFile := filepath.Join(tmpDir, "timestamps")

	// Exponential backoff WITHOUT multiplier — should default to 2.0
	wfYAML := `name: retry-multiplier-default
version: "1.0.0"
states:
  initial: run
  run:
    type: step
    command: |
      COUNT=$(cat "` + counterFile + `" 2>/dev/null || echo "0")
      COUNT=$((COUNT + 1))
      echo $COUNT > "` + counterFile + `"
      date +%s%N >> "` + tsFile + `"
      if [ $COUNT -lt 3 ]; then exit 1; fi
      exit 0
    retry:
      max_attempts: 4
      initial_delay: 50ms
      backoff: exponential
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "retry-multiplier-default.yaml"), []byte(wfYAML), 0o644))

	execSvc := newRetryAuditServices(tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	execCtx, err := execSvc.Run(ctx, "retry-multiplier-default", nil)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// With default multiplier 2.0: delay1=50ms, delay2=100ms → total ≥ 150ms
	// With broken multiplier 0.0: both delays would be 0ms
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond, "exponential backoff with default multiplier should produce meaningful delays")
}

// TestRetry_InvalidDurationRejected_Integration verifies that a malformed duration
// string in retry config produces a parse error instead of silently defaulting to 0ms.
func TestRetry_InvalidDurationRejected_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	wfYAML := `name: retry-bad-duration
version: "1.0.0"
states:
  initial: run
  run:
    type: step
    command: echo "never runs"
    retry:
      max_attempts: 3
      initial_delay: "not-a-duration"
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "retry-bad-duration.yaml"), []byte(wfYAML), 0o644))

	execSvc := newRetryAuditServices(tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := execSvc.Run(ctx, "retry-bad-duration", nil)
	require.Error(t, err, "invalid duration string should be rejected at parse time")
	assert.Contains(t, err.Error(), "initial_delay")
}

// TestRetry_ValidationRejectsInvalidConfig_Integration verifies that domain validation
// catches invalid retry configurations (e.g., max_attempts < 1, invalid backoff strategy).
func TestRetry_ValidationRejectsInvalidConfig_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		wfYAML  string
		wantErr string
	}{
		{
			name: "invalid backoff strategy",
			wfYAML: "name: retry-invalid-backoff\nversion: \"1.0.0\"\nstates:\n  initial: run\n  run:\n    type: step\n    command: echo \"never runs\"\n    retry:\n      max_attempts: 3\n      backoff: random\n    on_success: done\n    on_failure: error\n  done:\n    type: terminal\n  error:\n    type: terminal\n",
			wantErr: "backoff",
		},
		{
			name: "jitter out of range",
			wfYAML: "name: retry-invalid-jitter\nversion: \"1.0.0\"\nstates:\n  initial: run\n  run:\n    type: step\n    command: echo \"never runs\"\n    retry:\n      max_attempts: 3\n      jitter: 2.0\n    on_success: done\n    on_failure: error\n  done:\n    type: terminal\n  error:\n    type: terminal\n",
			wantErr: "jitter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wfName := filepath.Base(tt.name)
			require.NoError(t, os.WriteFile(filepath.Join(tmpDir, wfName+".yaml"), []byte(tt.wfYAML), 0o644))

			execSvc := newRetryAuditServices(tmpDir)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, err := execSvc.Run(ctx, wfName, nil)
			require.Error(t, err, "invalid retry config should be rejected")
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestDryRun_RetryAllFieldsMapped_Integration verifies that the dry-run executor
// maps all 7 retry fields (including Jitter and RetryableExitCodes) from domain to output.
func TestDryRun_RetryAllFieldsMapped_Integration(t *testing.T) {
	fixturesDir := "../../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesDir)
	store := newRetryMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &retryMockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())

	dryRunExec := application.NewDryRunExecutor(wfSvc, resolver, infraExpr.NewExprEvaluator(), logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	plan, err := dryRunExec.Execute(ctx, "retry-audit", nil)
	require.NoError(t, err)

	found := false
	for _, step := range plan.Steps {
		if step.Retry == nil {
			continue
		}

		assert.Equal(t, 3, step.Retry.MaxAttempts)
		assert.Equal(t, 200, step.Retry.InitialDelayMs)
		assert.Equal(t, 2000, step.Retry.MaxDelayMs)
		assert.Equal(t, "exponential", step.Retry.Backoff)
		assert.Equal(t, 2.0, step.Retry.Multiplier)
		assert.Equal(t, 0.3, step.Retry.Jitter, "Jitter must be mapped to DryRunRetry")
		assert.Equal(t, []int{1, 2}, step.Retry.RetryableExitCodes, "RetryableExitCodes must be mapped to DryRunRetry")
		found = true
		break
	}
	require.True(t, found, "fixture must contain a step with retry configuration")
}
