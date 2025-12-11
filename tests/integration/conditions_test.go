//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/infrastructure/store"
	"github.com/vanoix/awf/pkg/expression"
	"github.com/vanoix/awf/pkg/interpolation"
)

// conditionsMockLogger for integration tests
type conditionsMockLogger struct{}

func (m *conditionsMockLogger) Debug(msg string, fields ...any) {}
func (m *conditionsMockLogger) Info(msg string, fields ...any)  {}
func (m *conditionsMockLogger) Warn(msg string, fields ...any)  {}
func (m *conditionsMockLogger) Error(msg string, fields ...any) {}
func (m *conditionsMockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

func TestConditionalTransitions(t *testing.T) {
	tests := []struct {
		name         string
		workflow     string
		inputs       map[string]any
		wantFinalStep string
		wantErr      bool
	}{
		{
			name: "condition matches first transition",
			workflow: `
name: conditional-test
states:
  initial: process
  process:
    type: step
    command: echo "processing"
    capture:
      stdout: result
    transitions:
      - when: 'inputs.mode == "full"'
        goto: full_report
      - when: 'inputs.mode == "summary"'
        goto: summary_report
      - goto: error
  full_report:
    type: step
    command: echo "full report"
    on_success: done
  summary_report:
    type: step
    command: echo "summary report"
    on_success: done
  error:
    type: terminal
    status: failure
  done:
    type: terminal
    status: success
`,
			inputs:       map[string]any{"mode": "full"},
			wantFinalStep: "done",
			wantErr:      false,
		},
		{
			name: "condition matches second transition",
			workflow: `
name: conditional-test
states:
  initial: process
  process:
    type: step
    command: echo "processing"
    transitions:
      - when: 'inputs.mode == "full"'
        goto: full_report
      - when: 'inputs.mode == "summary"'
        goto: summary_report
      - goto: error
  full_report:
    type: step
    command: echo "full report"
    on_success: done
  summary_report:
    type: step
    command: echo "summary report"
    on_success: done
  error:
    type: terminal
    status: failure
  done:
    type: terminal
    status: success
`,
			inputs:       map[string]any{"mode": "summary"},
			wantFinalStep: "done",
			wantErr:      false,
		},
		{
			name: "falls back to default transition",
			workflow: `
name: conditional-test
states:
  initial: process
  process:
    type: step
    command: echo "processing"
    transitions:
      - when: 'inputs.mode == "full"'
        goto: full_report
      - when: 'inputs.mode == "summary"'
        goto: summary_report
      - goto: fallback
  full_report:
    type: step
    command: echo "full"
    on_success: done
  summary_report:
    type: step
    command: echo "summary"
    on_success: done
  fallback:
    type: step
    command: echo "fallback"
    on_success: done
  done:
    type: terminal
    status: success
`,
			inputs:       map[string]any{"mode": "unknown"},
			wantFinalStep: "done",
			wantErr:      false,
		},
		{
			name: "condition based on exit code",
			workflow: `
name: exit-code-test
states:
  initial: check
  check:
    type: step
    command: exit 0
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
`,
			inputs:       map[string]any{},
			wantFinalStep: "success",
			wantErr:      false,
		},
		{
			name: "complex condition with and",
			workflow: `
name: complex-condition
states:
  initial: process
  process:
    type: step
    command: echo "done"
    transitions:
      - when: 'inputs.count > 5 and inputs.mode == "batch"'
        goto: batch_process
      - goto: single_process
  batch_process:
    type: terminal
    status: success
  single_process:
    type: terminal
    status: success
`,
			inputs:       map[string]any{"count": 10, "mode": "batch"},
			wantFinalStep: "batch_process",
			wantErr:      false,
		},
		{
			name: "backward compatibility with on_success/on_failure",
			workflow: `
name: legacy-workflow
states:
  initial: process
  process:
    type: step
    command: echo "processing"
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`,
			inputs:       map[string]any{},
			wantFinalStep: "done",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup temp directory for workflow file
			tmpDir := t.TempDir()
			workflowPath := filepath.Join(tmpDir, "workflow.yaml")
			err := os.WriteFile(workflowPath, []byte(tt.workflow), 0644)
			require.NoError(t, err)

			// Create dependencies
			log := &conditionsMockLogger{}
			repo := repository.NewYAMLRepository(tmpDir)
			stateStore := store.NewJSONStore(filepath.Join(tmpDir, "states"))
			shellExecutor := executor.NewShellExecutor()
			parallelExecutor := application.NewParallelExecutor(log)
			resolver := interpolation.NewTemplateResolver()
			exprEvaluator := expression.NewExprEvaluator()

			// Create services
			wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, log)
			execSvc := application.NewExecutionServiceWithEvaluator(
				wfSvc,
				shellExecutor,
				parallelExecutor,
				stateStore,
				log,
				resolver,
				nil, // history service
				exprEvaluator,
			)

			// Run workflow
			ctx := context.Background()
			execCtx, err := execSvc.Run(ctx, "workflow", tt.inputs)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantFinalStep, execCtx.CurrentStep)
		})
	}
}

func TestExpressionEvaluator_Integration(t *testing.T) {
	evaluator := expression.NewExprEvaluator()

	tests := []struct {
		name    string
		expr    string
		ctx     *interpolation.Context
		want    bool
		wantErr bool
	}{
		{
			name: "access nested state data",
			expr: `states.build.exit_code == 0 and states.build.output != ""`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"build": {ExitCode: 0, Output: "success"},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "env variable check",
			expr: `env.CI == "true" or inputs.force == true`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"force": false},
				Env:    map[string]string{"CI": "true"},
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluator.Evaluate(tt.expr, tt.ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConditionalTransitions_MultipleConditionsInOrder(t *testing.T) {
	// Tests that conditions are evaluated in order and first match wins
	tmpDir := t.TempDir()

	// This workflow has overlapping conditions - both could match for count > 100
	// First match should win
	workflow := `
name: order-test
states:
  initial: check
  check:
    type: step
    command: echo "checking"
    transitions:
      - when: 'inputs.count > 100'
        goto: high
      - when: 'inputs.count > 50'
        goto: medium
      - when: 'inputs.count > 0'
        goto: low
      - goto: zero
  high:
    type: terminal
    status: success
  medium:
    type: terminal
    status: success
  low:
    type: terminal
    status: success
  zero:
    type: terminal
    status: success
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	err := os.WriteFile(workflowPath, []byte(workflow), 0644)
	require.NoError(t, err)

	log := &conditionsMockLogger{}
	repo := repository.NewYAMLRepository(tmpDir)
	stateStore := store.NewJSONStore(filepath.Join(tmpDir, "states"))
	shellExecutor := executor.NewShellExecutor()
	parallelExecutor := application.NewParallelExecutor(log)
	resolver := interpolation.NewTemplateResolver()
	exprEvaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, log)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc,
		shellExecutor,
		parallelExecutor,
		stateStore,
		log,
		resolver,
		nil,
		exprEvaluator,
	)

	tests := []struct {
		name          string
		count         int
		wantFinalStep string
	}{
		{name: "count 150 - should go to high", count: 150, wantFinalStep: "high"},
		{name: "count 75 - should go to medium", count: 75, wantFinalStep: "medium"},
		{name: "count 25 - should go to low", count: 25, wantFinalStep: "low"},
		{name: "count 0 - should go to zero", count: 0, wantFinalStep: "zero"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			execCtx, err := execSvc.Run(ctx, "workflow", map[string]any{"count": tt.count})
			require.NoError(t, err)
			assert.Equal(t, tt.wantFinalStep, execCtx.CurrentStep)
		})
	}
}

func TestConditionalTransitions_WithOutputCapture(t *testing.T) {
	// Tests that captured output can be used in conditions
	tmpDir := t.TempDir()

	workflow := `
name: output-condition-test
states:
  initial: generate
  generate:
    type: step
    command: echo "error detected"
    capture:
      stdout: result
    transitions:
      - when: 'states.generate.output contains "error"'
        goto: handle_error
      - when: 'states.generate.output contains "success"'
        goto: success
      - goto: unknown
  handle_error:
    type: terminal
    status: failure
  success:
    type: terminal
    status: success
  unknown:
    type: terminal
    status: failure
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	err := os.WriteFile(workflowPath, []byte(workflow), 0644)
	require.NoError(t, err)

	log := &conditionsMockLogger{}
	repo := repository.NewYAMLRepository(tmpDir)
	stateStore := store.NewJSONStore(filepath.Join(tmpDir, "states"))
	shellExecutor := executor.NewShellExecutor()
	parallelExecutor := application.NewParallelExecutor(log)
	resolver := interpolation.NewTemplateResolver()
	exprEvaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, log)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc,
		shellExecutor,
		parallelExecutor,
		stateStore,
		log,
		resolver,
		nil,
		exprEvaluator,
	)

	ctx := context.Background()
	execCtx, err := execSvc.Run(ctx, "workflow", nil)
	require.NoError(t, err)
	assert.Equal(t, "handle_error", execCtx.CurrentStep)
}

func TestConditionalTransitions_NestedConditions(t *testing.T) {
	// Tests complex conditions with multiple operators
	tmpDir := t.TempDir()

	workflow := `
name: nested-condition-test
states:
  initial: check
  check:
    type: step
    command: echo "checking"
    transitions:
      - when: '(inputs.env == "prod" and inputs.approved == true) or inputs.force == true'
        goto: deploy
      - goto: skip
  deploy:
    type: terminal
    status: success
  skip:
    type: terminal
    status: success
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	err := os.WriteFile(workflowPath, []byte(workflow), 0644)
	require.NoError(t, err)

	log := &conditionsMockLogger{}
	repo := repository.NewYAMLRepository(tmpDir)
	stateStore := store.NewJSONStore(filepath.Join(tmpDir, "states"))
	shellExecutor := executor.NewShellExecutor()
	parallelExecutor := application.NewParallelExecutor(log)
	resolver := interpolation.NewTemplateResolver()
	exprEvaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, log)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc,
		shellExecutor,
		parallelExecutor,
		stateStore,
		log,
		resolver,
		nil,
		exprEvaluator,
	)

	tests := []struct {
		name          string
		inputs        map[string]any
		wantFinalStep string
	}{
		{
			name:          "prod + approved = deploy",
			inputs:        map[string]any{"env": "prod", "approved": true, "force": false},
			wantFinalStep: "deploy",
		},
		{
			name:          "prod - not approved = skip",
			inputs:        map[string]any{"env": "prod", "approved": false, "force": false},
			wantFinalStep: "skip",
		},
		{
			name:          "force override = deploy",
			inputs:        map[string]any{"env": "staging", "approved": false, "force": true},
			wantFinalStep: "deploy",
		},
		{
			name:          "staging - no force = skip",
			inputs:        map[string]any{"env": "staging", "approved": true, "force": false},
			wantFinalStep: "skip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			execCtx, err := execSvc.Run(ctx, "workflow", tt.inputs)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFinalStep, execCtx.CurrentStep)
		})
	}
}

func TestConditionalTransitions_InvalidExpression(t *testing.T) {
	// Tests that invalid expressions produce clear errors
	tmpDir := t.TempDir()

	workflow := `
name: invalid-expr-test
states:
  initial: check
  check:
    type: step
    command: echo "checking"
    transitions:
      - when: 'inputs.mode === "full"'
        goto: next
      - goto: error
  next:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	err := os.WriteFile(workflowPath, []byte(workflow), 0644)
	require.NoError(t, err)

	log := &conditionsMockLogger{}
	repo := repository.NewYAMLRepository(tmpDir)
	stateStore := store.NewJSONStore(filepath.Join(tmpDir, "states"))
	shellExecutor := executor.NewShellExecutor()
	parallelExecutor := application.NewParallelExecutor(log)
	resolver := interpolation.NewTemplateResolver()
	exprEvaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, log)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc,
		shellExecutor,
		parallelExecutor,
		stateStore,
		log,
		resolver,
		nil,
		exprEvaluator,
	)

	ctx := context.Background()
	_, err = execSvc.Run(ctx, "workflow", map[string]any{"mode": "full"})
	// Should error because === is not a valid operator
	require.Error(t, err)
}

func TestConditionalTransitions_MixedWithLegacy(t *testing.T) {
	// Tests that transitions take precedence over on_success/on_failure
	// when both are specified
	tmpDir := t.TempDir()

	workflow := `
name: mixed-transitions-test
states:
  initial: step1
  step1:
    type: step
    command: echo "step1"
    transitions:
      - when: 'inputs.use_transitions == true'
        goto: via_transitions
      - goto: via_transitions_default
    on_success: via_legacy
    on_failure: error
  via_transitions:
    type: terminal
    status: success
  via_transitions_default:
    type: terminal
    status: success
  via_legacy:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	err := os.WriteFile(workflowPath, []byte(workflow), 0644)
	require.NoError(t, err)

	log := &conditionsMockLogger{}
	repo := repository.NewYAMLRepository(tmpDir)
	stateStore := store.NewJSONStore(filepath.Join(tmpDir, "states"))
	shellExecutor := executor.NewShellExecutor()
	parallelExecutor := application.NewParallelExecutor(log)
	resolver := interpolation.NewTemplateResolver()
	exprEvaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, log)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc,
		shellExecutor,
		parallelExecutor,
		stateStore,
		log,
		resolver,
		nil,
		exprEvaluator,
	)

	tests := []struct {
		name          string
		inputs        map[string]any
		wantFinalStep string
	}{
		{
			name:          "transitions condition matches - uses transitions",
			inputs:        map[string]any{"use_transitions": true},
			wantFinalStep: "via_transitions",
		},
		{
			name:          "transitions default - uses transitions default",
			inputs:        map[string]any{"use_transitions": false},
			wantFinalStep: "via_transitions_default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			execCtx, err := execSvc.Run(ctx, "workflow", tt.inputs)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFinalStep, execCtx.CurrentStep)
		})
	}
}

func TestConditionalTransitions_NoMatchNoDefault(t *testing.T) {
	// Tests behavior when no condition matches and there's no default
	// Should fall back to on_success/on_failure if available
	tmpDir := t.TempDir()

	workflow := `
name: no-match-test
states:
  initial: check
  check:
    type: step
    command: echo "checking"
    transitions:
      - when: 'inputs.mode == "A"'
        goto: a
      - when: 'inputs.mode == "B"'
        goto: b
    on_success: fallback_success
    on_failure: fallback_error
  a:
    type: terminal
    status: success
  b:
    type: terminal
    status: success
  fallback_success:
    type: terminal
    status: success
  fallback_error:
    type: terminal
    status: failure
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	err := os.WriteFile(workflowPath, []byte(workflow), 0644)
	require.NoError(t, err)

	log := &conditionsMockLogger{}
	repo := repository.NewYAMLRepository(tmpDir)
	stateStore := store.NewJSONStore(filepath.Join(tmpDir, "states"))
	shellExecutor := executor.NewShellExecutor()
	parallelExecutor := application.NewParallelExecutor(log)
	resolver := interpolation.NewTemplateResolver()
	exprEvaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, log)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc,
		shellExecutor,
		parallelExecutor,
		stateStore,
		log,
		resolver,
		nil,
		exprEvaluator,
	)

	ctx := context.Background()
	// mode "C" doesn't match any transition - should fall back to on_success
	execCtx, err := execSvc.Run(ctx, "workflow", map[string]any{"mode": "C"})
	require.NoError(t, err)
	assert.Equal(t, "fallback_success", execCtx.CurrentStep)
}

func TestConditionalTransitions_ChainedConditions(t *testing.T) {
	// Tests a workflow with multiple steps, each with conditional transitions
	tmpDir := t.TempDir()

	workflow := `
name: chained-conditions-test
states:
  initial: step1
  step1:
    type: step
    command: echo "step1"
    transitions:
      - when: 'inputs.path == 1'
        goto: step2a
      - goto: step2b
  step2a:
    type: step
    command: echo "step2a"
    transitions:
      - when: 'inputs.proceed == true'
        goto: final_a
      - goto: final_b
  step2b:
    type: step
    command: echo "step2b"
    transitions:
      - when: 'inputs.proceed == true'
        goto: final_c
      - goto: final_d
  final_a:
    type: terminal
    status: success
  final_b:
    type: terminal
    status: success
  final_c:
    type: terminal
    status: success
  final_d:
    type: terminal
    status: success
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	err := os.WriteFile(workflowPath, []byte(workflow), 0644)
	require.NoError(t, err)

	log := &conditionsMockLogger{}
	repo := repository.NewYAMLRepository(tmpDir)
	stateStore := store.NewJSONStore(filepath.Join(tmpDir, "states"))
	shellExecutor := executor.NewShellExecutor()
	parallelExecutor := application.NewParallelExecutor(log)
	resolver := interpolation.NewTemplateResolver()
	exprEvaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, log)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc,
		shellExecutor,
		parallelExecutor,
		stateStore,
		log,
		resolver,
		nil,
		exprEvaluator,
	)

	tests := []struct {
		name          string
		inputs        map[string]any
		wantFinalStep string
	}{
		{
			name:          "path 1, proceed true - final_a",
			inputs:        map[string]any{"path": 1, "proceed": true},
			wantFinalStep: "final_a",
		},
		{
			name:          "path 1, proceed false - final_b",
			inputs:        map[string]any{"path": 1, "proceed": false},
			wantFinalStep: "final_b",
		},
		{
			name:          "path 2, proceed true - final_c",
			inputs:        map[string]any{"path": 2, "proceed": true},
			wantFinalStep: "final_c",
		},
		{
			name:          "path 2, proceed false - final_d",
			inputs:        map[string]any{"path": 2, "proceed": false},
			wantFinalStep: "final_d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			execCtx, err := execSvc.Run(ctx, "workflow", tt.inputs)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFinalStep, execCtx.CurrentStep)
		})
	}
}
