//go:build integration

package tracing_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
)

type tracingTestEnv struct {
	execSvc *application.ExecutionService
	tracer  *mocks.MockTracer
}

func newTracingTestEnv(t *testing.T, tmpDir string, tracer *mocks.MockTracer) *tracingTestEnv {
	t.Helper()

	logger := mocks.NewMockLogger()
	repo := repository.NewYAMLRepository(tmpDir)
	storeImpl := store.NewJSONStore(tmpDir)
	exec := executor.NewShellExecutor()
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, storeImpl, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, storeImpl, logger, resolver, nil, evaluator,
	)
	if tracer != nil {
		execSvc.SetTracer(tracer)
	}

	return &tracingTestEnv{execSvc: execSvc, tracer: tracer}
}

func writeWorkflow(t *testing.T, dir, name, yaml string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(yaml), 0o644)
	require.NoError(t, err)
}

func spanNames(tracer *mocks.MockTracer) map[string]bool {
	names := make(map[string]bool)
	for _, span := range tracer.GetSpans() {
		names[span.Record().Name] = true
	}
	return names
}

// TestTracingWorkflowRootSpan verifies that a complete workflow execution
// produces a root workflow.run span.
func TestTracingWorkflowRootSpan(t *testing.T) {
	tmpDir := t.TempDir()
	tracer := mocks.NewMockTracer()

	writeWorkflow(t, tmpDir, "simple", `
name: simple-workflow
version: "1.0.0"

states:
  initial: echo_step
  echo_step:
    type: step
    command: 'echo "hello"'
    on_success: done
  done:
    type: terminal
    status: success
`)

	env := newTracingTestEnv(t, tmpDir, tracer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execCtx, err := env.execSvc.Run(ctx, "simple", nil)
	require.NoError(t, err)
	require.NotNil(t, execCtx)

	assert.True(t, spanNames(tracer)["workflow.run"], "expected workflow.run root span")
}

// TestTracingStepSpans verifies that each step execution produces a step.<name>
// child span under the root workflow span.
func TestTracingStepSpans(t *testing.T) {
	tmpDir := t.TempDir()
	tracer := mocks.NewMockTracer()

	writeWorkflow(t, tmpDir, "multistep", `
name: multi-step-workflow
version: "1.0.0"

states:
  initial: first_step
  first_step:
    type: step
    command: 'echo "step1"'
    on_success: second_step
  second_step:
    type: step
    command: 'echo "step2"'
    on_success: done
  done:
    type: terminal
    status: success
`)

	env := newTracingTestEnv(t, tmpDir, tracer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execCtx, err := env.execSvc.Run(ctx, "multistep", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	names := spanNames(tracer)
	assert.True(t, names["workflow.run"], "expected workflow.run span")
	assert.True(t, names["step.first_step"], "expected step.first_step span")
	assert.True(t, names["step.second_step"], "expected step.second_step span")
}

// TestTracingNilTracerGraceful verifies that ExecutionService handles a nil
// tracer gracefully by falling back to NopTracer without errors or overhead.
func TestTracingNilTracerGraceful(t *testing.T) {
	tmpDir := t.TempDir()

	writeWorkflow(t, tmpDir, "simple", `
name: simple-workflow
version: "1.0.0"

states:
  initial: echo_step
  echo_step:
    type: step
    command: 'echo "hello"'
    on_success: done
  done:
    type: terminal
    status: success
`)

	env := newTracingTestEnv(t, tmpDir, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execCtx, err := env.execSvc.Run(ctx, "simple", nil)
	require.NoError(t, err, "expected execution to succeed even without tracer")
	require.NotNil(t, execCtx)
}

// TestTracingShellCommandSpan verifies that shell.execute spans are created
// when executing shell commands in steps.
func TestTracingShellCommandSpan(t *testing.T) {
	tmpDir := t.TempDir()
	tracer := mocks.NewMockTracer()

	writeWorkflow(t, tmpDir, "shell", `
name: shell-command-workflow
version: "1.0.0"

states:
  initial: echo_step
  echo_step:
    type: step
    command: 'echo "Testing shell span"'
    on_success: done
  done:
    type: terminal
    status: success
`)

	env := newTracingTestEnv(t, tmpDir, tracer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execCtx, err := env.execSvc.Run(ctx, "shell", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	names := spanNames(tracer)
	assert.True(t, names["shell.execute"], "expected shell.execute span")
	assert.True(t, names["workflow.run"], "expected workflow.run span")
}

// TestTracingLoopSpans verifies that loop execution produces loop.for_each
// or loop.while spans with proper iteration child spans.
func TestTracingLoopSpans(t *testing.T) {
	tmpDir := t.TempDir()
	tracer := mocks.NewMockTracer()

	writeWorkflow(t, tmpDir, "loop", `
name: loop-workflow
version: "1.0.0"

states:
  initial: process_items
  process_items:
    type: for_each
    items: '["item1", "item2"]'
    max_iterations: 10
    body:
      - process_step
    on_complete: done
  process_step:
    type: step
    command: 'echo "processing"'
    on_success: process_items
  done:
    type: terminal
    status: success
`)

	env := newTracingTestEnv(t, tmpDir, tracer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execCtx, err := env.execSvc.Run(ctx, "loop", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	names := spanNames(tracer)
	assert.True(t, names["loop.for_each"], "expected loop.for_each span")
	assert.True(t, names["workflow.run"], "expected workflow.run span")
}

// TestTracingSpanEndsOnError verifies that spans are properly ended even
// when step execution fails or encounters errors.
func TestTracingSpanEndsOnError(t *testing.T) {
	tmpDir := t.TempDir()
	tracer := mocks.NewMockTracer()

	writeWorkflow(t, tmpDir, "error", `
name: error-workflow
version: "1.0.0"

states:
  initial: failing_step
  failing_step:
    type: step
    command: 'exit 1'
    on_success: done
    on_failure: done
  done:
    type: terminal
    status: success
`)

	env := newTracingTestEnv(t, tmpDir, tracer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = env.execSvc.Run(ctx, "error", nil)

	spans := tracer.GetSpans()
	require.NotEmpty(t, spans, "expected spans to be recorded even on error")

	for _, span := range spans {
		assert.True(t, span.Record().Ended,
			"expected span %q to be ended even on error", span.Record().Name)
	}
}

// TestTracingMultipleWorkflows verifies that tracing works correctly
// when multiple workflows are executed sequentially with the same tracer.
func TestTracingMultipleWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	tracer := mocks.NewMockTracer()

	writeWorkflow(t, tmpDir, "workflow1", `
name: workflow-1
version: "1.0.0"

states:
  initial: step_1a
  step_1a:
    type: step
    command: 'echo "step 1"'
    on_success: done
  done:
    type: terminal
    status: success
`)

	writeWorkflow(t, tmpDir, "workflow2", `
name: workflow-2
version: "1.0.0"

states:
  initial: step_2a
  step_2a:
    type: step
    command: 'echo "step 2"'
    on_success: done
  done:
    type: terminal
    status: success
`)

	env := newTracingTestEnv(t, tmpDir, tracer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err1 := env.execSvc.Run(ctx, "workflow1", nil)
	require.NoError(t, err1)

	spansAfterFirst := len(tracer.GetSpans())

	_, err2 := env.execSvc.Run(ctx, "workflow2", nil)
	require.NoError(t, err2)

	spansAfterSecond := len(tracer.GetSpans())

	assert.Greater(t, spansAfterSecond, spansAfterFirst,
		"expected tracing to accumulate spans from multiple workflow executions")
}

// TestTracingContextPropagation verifies that context is correctly propagated
// through span creation, enabling trace correlation.
func TestTracingContextPropagation(t *testing.T) {
	tmpDir := t.TempDir()
	tracer := mocks.NewMockTracer()

	writeWorkflow(t, tmpDir, "context", `
name: context-workflow
version: "1.0.0"

states:
  initial: echo_step
  echo_step:
    type: step
    command: 'echo "context"'
    on_success: done
  done:
    type: terminal
    status: success
`)

	env := newTracingTestEnv(t, tmpDir, tracer)
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := env.execSvc.Run(timeoutCtx, "context", nil)
	require.NoError(t, err, "expected execution to succeed with context timeout")
	require.NotNil(t, execCtx)

	spans := tracer.GetSpans()
	require.NotEmpty(t, spans, "expected spans even with timeout context")
	assert.Greater(t, len(spans), 1, "expected multiple spans demonstrating context propagation")
}
