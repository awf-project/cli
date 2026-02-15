//go:build integration

// Feature: C054
// Functional tests validating application layer coverage improvements.
// These tests exercise the code paths covered by C054 components (T001-T009)
// through real infrastructure: YAML workflows, shell execution, and state persistence.

package quality_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/awf/internal/infrastructure/expression"
	"github.com/awf-project/awf/internal/infrastructure/repository"
	"github.com/awf-project/awf/internal/infrastructure/store"
	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/awf-project/awf/tests/integration/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type coverageTestEnv struct {
	execSvc   *application.ExecutionService
	wfSvc     *application.WorkflowService
	store     *store.JSONStore
	tmpDir    string
	statesDir string
}

func setupCoverageTestEnv(t *testing.T) *coverageTestEnv {
	t.Helper()

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	repo := repository.NewYAMLRepository(workflowsDir)
	stateStore := store.NewJSONStore(statesDir)
	exec := executor.NewShellExecutor()
	logger := &testhelpers.MockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, stateStore, logger, resolver, nil, evaluator,
	)

	return &coverageTestEnv{
		execSvc:   execSvc,
		wfSvc:     wfSvc,
		store:     stateStore,
		tmpDir:    tmpDir,
		statesDir: statesDir,
	}
}

func (e *coverageTestEnv) writeWorkflow(t *testing.T, name, yaml string) {
	t.Helper()
	path := filepath.Join(e.tmpDir, "workflows", name+".yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o644))
}

func TestConditionalTransitions_ExpressionEvaluation(t *testing.T) {
	tests := []struct {
		name          string
		inputs        map[string]any
		wantFinalStep string
	}{
		{
			name:          "first transition matches",
			inputs:        map[string]any{"priority": "high"},
			wantFinalStep: "urgent_handler",
		},
		{
			name:          "second transition matches",
			inputs:        map[string]any{"priority": "low"},
			wantFinalStep: "normal_handler",
		},
		{
			name:          "default transition when no condition matches",
			inputs:        map[string]any{"priority": "unknown"},
			wantFinalStep: "default_handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupCoverageTestEnv(t)
			env.writeWorkflow(t, "transition-test", `
name: transition-test
version: "1.0.0"
states:
  initial: classify
  classify:
    type: step
    command: echo "classifying"
    transitions:
      - when: 'inputs.priority == "high"'
        goto: urgent_handler
      - when: 'inputs.priority == "low"'
        goto: normal_handler
      - goto: default_handler
  urgent_handler:
    type: terminal
    status: success
  normal_handler:
    type: terminal
    status: success
  default_handler:
    type: terminal
    status: success
`)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			execCtx, err := env.execSvc.Run(ctx, "transition-test", tt.inputs)

			require.NoError(t, err)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
			assert.Equal(t, tt.wantFinalStep, execCtx.CurrentStep)
		})
	}
}

func TestLoopBreakTransition_Integration(t *testing.T) {
	env := setupCoverageTestEnv(t)
	logFile := filepath.Join(env.tmpDir, "loop_break.log")

	env.writeWorkflow(t, "loop-break", `
name: loop-break
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["a", "b", "STOP", "d"]'
    max_iterations: 10
    body:
      - check_item
    on_complete: all_done
  check_item:
    type: step
    command: 'echo "{{.loop.Item}}" >> `+logFile+`'
    transitions:
      - when: 'loop.Item == "STOP"'
        goto: early_exit
      - goto: process_items
  early_exit:
    type: terminal
    status: success
  all_done:
    type: terminal
    status: success
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := env.execSvc.Run(ctx, "loop-break", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "early_exit", execCtx.CurrentStep, "loop should break to early_exit")

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 3)
	assert.Equal(t, "STOP", lines[2])
}

func TestSingleStepExecution_WithRealShell(t *testing.T) {
	env := setupCoverageTestEnv(t)

	env.writeWorkflow(t, "single-step", `
name: single-step
version: "1.0.0"
inputs:
  - name: greeting
    required: true
states:
  initial: greet
  greet:
    type: step
    command: echo "{{.inputs.greeting}} - prev was {{.states.setup.Output}}"
    on_success: done
  done:
    type: terminal
    status: success
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := env.execSvc.ExecuteSingleStep(
		ctx,
		"single-step",
		"greet",
		map[string]any{"greeting": "Hello"},
		map[string]string{"states.setup.output": "initialized"},
	)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, result.Status)
	assert.Contains(t, result.Output, "Hello")
	assert.Contains(t, result.Output, "prev was initialized")
	assert.Equal(t, 0, result.ExitCode)
}

func TestWorkflowValidation_ValidWorkflow(t *testing.T) {
	env := setupCoverageTestEnv(t)

	env.writeWorkflow(t, "valid-workflow", `
name: valid-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "ok"
    on_success: done
  done:
    type: terminal
    status: success
`)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := env.wfSvc.ValidateWorkflow(ctx, "valid-workflow")

	assert.NoError(t, err)
}

func TestSingleStepExecution_EdgeCases(t *testing.T) {
	env := setupCoverageTestEnv(t)

	env.writeWorkflow(t, "edge-cases", `
name: edge-cases
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "ok"
    on_success: done
  done:
    type: terminal
    status: success
`)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tests := []struct {
		name    string
		step    string
		wantErr string
	}{
		{
			name:    "terminal step rejected",
			step:    "done",
			wantErr: "cannot execute terminal step",
		},
		{
			name:    "nonexistent step",
			step:    "nonexistent",
			wantErr: "step not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := env.execSvc.ExecuteSingleStep(ctx, "edge-cases", tt.step, nil, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}

	t.Run("no mocks no inputs", func(t *testing.T) {
		result, err := env.execSvc.ExecuteSingleStep(ctx, "edge-cases", "step1", nil, nil)

		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, result.Status)
	})
}

func TestLoopMaxIterations_ExactBoundary(t *testing.T) {
	env := setupCoverageTestEnv(t)
	logFile := filepath.Join(env.tmpDir, "max_iter.log")

	env.writeWorkflow(t, "max-iter", `
name: max-iter
version: "1.0.0"
states:
  initial: loop
  loop:
    type: for_each
    items: '["one", "two", "three"]'
    max_iterations: 3
    body:
      - do_work
    on_complete: done
  do_work:
    type: step
    command: 'echo "{{.loop.Item}}" >> `+logFile+`'
    on_success: loop
  done:
    type: terminal
    status: success
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := env.execSvc.Run(ctx, "max-iter", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 3)
}

func TestConditionalTransitions_LegacyFallback(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		wantFinalStep string
	}{
		{
			name:          "success falls back to on_success",
			command:       "echo ok",
			wantFinalStep: "success",
		},
		{
			name:          "failure falls back to on_failure",
			command:       "exit 1",
			wantFinalStep: "failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupCoverageTestEnv(t)

			env.writeWorkflow(t, "legacy-fallback", `
name: legacy-fallback
version: "1.0.0"
states:
  initial: process
  process:
    type: step
    command: `+tt.command+`
    on_success: success
    on_failure: failure
  success:
    type: terminal
    status: success
  failure:
    type: terminal
    status: failure
`)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			execCtx, err := env.execSvc.Run(ctx, "legacy-fallback", nil)

			require.NoError(t, err)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
			assert.Equal(t, tt.wantFinalStep, execCtx.CurrentStep)
		})
	}
}

func TestErrorClassification_RealFailures(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		wantFinalStep string
		wantExitCode  int
	}{
		{
			name:          "command not found triggers execution error",
			command:       "nonexistent_command_that_does_not_exist_xyz 2>/dev/null",
			wantFinalStep: "error_handler",
			wantExitCode:  127,
		},
		{
			name:          "non-zero exit classified as execution error",
			command:       "exit 42",
			wantFinalStep: "error_handler",
			wantExitCode:  42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupCoverageTestEnv(t)

			env.writeWorkflow(t, "error-classify", `
name: error-classify
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: `+tt.command+`
    on_success: done
    on_failure: error_handler
  error_handler:
    type: terminal
    status: failure
  done:
    type: terminal
    status: success
`)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			execCtx, err := env.execSvc.Run(ctx, "error-classify", nil)

			require.NoError(t, err)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
			assert.Equal(t, tt.wantFinalStep, execCtx.CurrentStep)

			failState, ok := execCtx.GetStepState("failing_step")
			require.True(t, ok)
			assert.Equal(t, workflow.StatusFailed, failState.Status)
			assert.Equal(t, tt.wantExitCode, failState.ExitCode)
		})
	}
}

func TestStepTimeout_Integration(t *testing.T) {
	env := setupCoverageTestEnv(t)

	env.writeWorkflow(t, "timeout-test", `
name: timeout-test
version: "1.0.0"
states:
  initial: slow_step
  slow_step:
    type: step
    command: sleep 60
    timeout: 1
    on_success: done
    on_failure: timeout_handler
  timeout_handler:
    type: step
    command: echo "timed out"
    on_success: done
  done:
    type: terminal
    status: success
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	execCtx, err := env.execSvc.Run(ctx, "timeout-test", nil)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Less(t, elapsed, 10*time.Second, "should timeout within seconds, not minutes")

	handlerState, ok := execCtx.GetStepState("timeout_handler")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, handlerState.Status)
}

func TestSingleStepExecution_CommandErrors(t *testing.T) {
	env := setupCoverageTestEnv(t)

	env.writeWorkflow(t, "cmd-errors", `
name: cmd-errors
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: exit 42
    on_success: done
  step_with_timeout:
    type: step
    command: sleep 60
    timeout: 1
    on_success: done
  done:
    type: terminal
    status: success
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("non-zero exit returns result not error", func(t *testing.T) {
		result, err := env.execSvc.ExecuteSingleStep(ctx, "cmd-errors", "step1", nil, nil)

		require.NoError(t, err, "ExecuteSingleStep returns nil error for command failures")
		assert.Equal(t, workflow.StatusFailed, result.Status)
		assert.Equal(t, 42, result.ExitCode)
	})

	t.Run("timeout returns result with error string", func(t *testing.T) {
		result, err := env.execSvc.ExecuteSingleStep(ctx, "cmd-errors", "step_with_timeout", nil, nil)

		require.NoError(t, err, "ExecuteSingleStep returns nil error for timeouts")
		assert.Equal(t, workflow.StatusFailed, result.Status)
		assert.NotEmpty(t, result.Error, "error field should describe the timeout")
	})
}

func TestWorkflowValidation_Errors(t *testing.T) {
	env := setupCoverageTestEnv(t)

	t.Run("nonexistent workflow", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := env.wfSvc.ValidateWorkflow(ctx, "nonexistent-workflow")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "load workflow")
	})

	t.Run("invalid state reference", func(t *testing.T) {
		env.writeWorkflow(t, "broken-ref", `
name: broken-ref
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "ok"
    on_success: nonexistent_state
  done:
    type: terminal
    status: success
`)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := env.wfSvc.ValidateWorkflow(ctx, "broken-ref")

		require.Error(t, err)
	})
}

func TestResumeExecution_WithStateTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	logFile := filepath.Join(tmpDir, "resume.log")
	wfYAML := `name: resume-transitions
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "STEP1" >> ` + logFile + `
    on_success: step2
  step2:
    type: step
    command: echo "STEP2" >> ` + logFile + `
    on_success: done
  done:
    type: terminal
    status: success
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "resume-transitions.yaml"), []byte(wfYAML), 0o644))

	execSvc, stateStore := testhelpers.SetupTestWorkflowService(t, workflowsDir, statesDir)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "resume-transitions", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	loaded, err := stateStore.Load(ctx, execCtx.WorkflowID)
	require.NoError(t, err)
	require.NotNil(t, loaded, "state should be persisted after execution")

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "STEP1")
	assert.Contains(t, string(data), "STEP2")
}

func TestComplexWorkflow_MultipleCodePaths(t *testing.T) {
	env := setupCoverageTestEnv(t)

	env.writeWorkflow(t, "complex-paths", `
name: complex-paths
version: "1.0.0"
inputs:
  - name: mode
    required: true
states:
  initial: decide
  decide:
    type: step
    command: echo "deciding"
    transitions:
      - when: 'inputs.mode == "safe"'
        goto: safe_step
      - when: 'inputs.mode == "risky"'
        goto: risky_step
      - goto: fallback_step

  safe_step:
    type: step
    command: echo "safe path"
    on_success: done

  risky_step:
    type: step
    command: exit 1
    continue_on_error: true
    on_success: recovery

  recovery:
    type: step
    command: echo "recovered from risky"
    on_success: done

  fallback_step:
    type: step
    command: echo "fallback"
    on_success: done

  done:
    type: terminal
    status: success
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("safe mode uses conditional transition", func(t *testing.T) {
		execCtx, err := env.execSvc.Run(ctx, "complex-paths", map[string]any{"mode": "safe"})

		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
		assert.Equal(t, "done", execCtx.CurrentStep)

		safeState, ok := execCtx.GetStepState("safe_step")
		require.True(t, ok)
		assert.Equal(t, workflow.StatusCompleted, safeState.Status)
	})

	t.Run("risky mode uses continue_on_error and recovery", func(t *testing.T) {
		execCtx, err := env.execSvc.Run(ctx, "complex-paths", map[string]any{"mode": "risky"})

		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
		assert.Equal(t, "done", execCtx.CurrentStep)

		riskyState, ok := execCtx.GetStepState("risky_step")
		require.True(t, ok)
		assert.Equal(t, workflow.StatusFailed, riskyState.Status)

		recoveryState, ok := execCtx.GetStepState("recovery")
		require.True(t, ok)
		assert.Equal(t, workflow.StatusCompleted, recoveryState.Status)
	})

	t.Run("unknown mode falls to default transition", func(t *testing.T) {
		execCtx, err := env.execSvc.Run(ctx, "complex-paths", map[string]any{"mode": "unexpected"})

		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
		assert.Equal(t, "done", execCtx.CurrentStep)

		fallbackState, ok := execCtx.GetStepState("fallback_step")
		require.True(t, ok)
		assert.Equal(t, workflow.StatusCompleted, fallbackState.Status)
	})
}

func TestOutputInterpolation_AcrossSteps(t *testing.T) {
	env := setupCoverageTestEnv(t)

	env.writeWorkflow(t, "interpolation-chain", `
name: interpolation-chain
version: "1.0.0"
inputs:
  - name: prefix
    required: true
states:
  initial: produce
  produce:
    type: step
    command: echo "{{.inputs.prefix}}_VALUE"
    capture_output: true
    on_success: consume
  consume:
    type: step
    command: echo "received {{.states.produce.Output}}"
    on_success: done
  done:
    type: terminal
    status: success
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := env.execSvc.Run(ctx, "interpolation-chain", map[string]any{"prefix": "TEST"})

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	produceState, ok := execCtx.GetStepState("produce")
	require.True(t, ok)
	assert.Contains(t, produceState.Output, "TEST_VALUE")

	consumeState, ok := execCtx.GetStepState("consume")
	require.True(t, ok)
	assert.Contains(t, consumeState.Output, "received")
	assert.Contains(t, consumeState.Output, "TEST_VALUE")
}

func TestLoopWithContinueOnError_Integration(t *testing.T) {
	env := setupCoverageTestEnv(t)
	logFile := filepath.Join(env.tmpDir, "loop_continue.log")

	env.writeWorkflow(t, "loop-continue", `
name: loop-continue
version: "1.0.0"
states:
  initial: process
  process:
    type: for_each
    items: '["ok1", "FAIL", "ok2"]'
    max_iterations: 10
    continue_on_error: true
    body:
      - do_item
    on_complete: done
  do_item:
    type: step
    command: 'bash -c ''if [ "{{.loop.Item}}" = "FAIL" ]; then echo "FAIL" >> `+logFile+`; exit 1; else echo "{{.loop.Item}}" >> `+logFile+`; fi'''
    on_success: process
    on_failure: process
  done:
    type: terminal
    status: success
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := env.execSvc.Run(ctx, "loop-continue", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 3)
}

func TestTransitionBasedOnExitCode(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		wantFinalStep string
	}{
		{
			name:          "exit 0 routes to success",
			command:       "exit 0",
			wantFinalStep: "success",
		},
		{
			name:          "exit 1 routes to failure",
			command:       "exit 1",
			wantFinalStep: "failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupCoverageTestEnv(t)

			env.writeWorkflow(t, "exit-code-route", `
name: exit-code-route
version: "1.0.0"
states:
  initial: check
  check:
    type: step
    command: `+tt.command+`
    transitions:
      - when: 'states.check.exit_code == 0'
        goto: success
      - goto: failure
  success:
    type: terminal
    status: success
  failure:
    type: terminal
    status: failure
`)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			execCtx, err := env.execSvc.Run(ctx, "exit-code-route", nil)

			require.NoError(t, err)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
			assert.Equal(t, tt.wantFinalStep, execCtx.CurrentStep)
		})
	}
}
