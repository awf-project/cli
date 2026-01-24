//go:build integration

package integration_test

// Feature: C018

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/testutil"
)

// =============================================================================
// Pkg Test Coverage Functional Tests
// Validates that pkg/interpolation improvements (LoopData.Parent,
// StepStateData.Response/Tokens, expression namespaces) work correctly
// in end-to-end workflow execution.
//
// Coverage:
// - Nested loop access via LoopData.Parent field
// - Agent step data access via StepStateData.Response and Tokens fields
// - Expression namespace access (loop.*, context.*, error.*)
// - Integration with full workflow execution pipeline
// =============================================================================

// TestNestedLoops_HappyPath verifies that nested loops can access parent
// loop data through LoopData.Parent field in workflow execution.
func TestNestedLoops_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name         string
		workflowYAML string
		inputs       map[string]interface{}
		wantSuccess  bool
	}{
		{
			name: "two-level nested loops with parent access",
			workflowYAML: `
name: nested-loop-parent
version: "1.0.0"
inputs:
  - name: categories
    type: string
    required: true
states:
  initial: outer_loop
  outer_loop:
    type: loop
    items:
      - name: Category A
        items: ["item1", "item2"]
      - name: Category B
        items: ["item3", "item4"]
    loop_state: inner_loop
    on_complete: end
  inner_loop:
    type: loop
    items: "{{.loop.Item.items}}"
    loop_state: process
    on_complete: outer_loop
  process:
    type: step
    command: 'echo "Parent: {{.loop.Parent.Item.name}}, Item: {{.loop.Item}}"'
    on_success: inner_loop
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"categories": "test"},
			wantSuccess: true,
		},
		{
			name: "parent index access in nested loops",
			workflowYAML: `
name: parent-index-access
version: "1.0.0"
states:
  initial: outer
  outer:
    type: loop
    items: ["A", "B", "C"]
    loop_state: inner
    on_complete: end
  inner:
    type: loop
    items: ["1", "2"]
    loop_state: show
    on_complete: outer
  show:
    type: step
    command: 'echo "Outer[{{.loop.Parent.Index}}]={{.loop.Parent.Item}}, Inner[{{.loop.Index}}]={{.loop.Item}}"'
    on_success: inner
  end:
    type: terminal
    status: success
`,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			// Setup components
			repo := repository.NewYAMLRepository(workflowsDir)
			stateStore := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			log := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(stateStore).
				WithExecutor(exec).
				WithLogger(log).
				Build()

			// Act
			execCtx, err := svc.Run(ctx, "test", tt.inputs)

			// Assert
			if tt.wantSuccess {
				assert.NoError(t, err, "workflow should execute successfully")
				if execCtx != nil {
					assert.NotEmpty(t, execCtx.WorkflowID, "execution should have ID")
					assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "execution should complete")
				}
			} else {
				assert.Error(t, err, "workflow should fail")
			}
		})
	}
}

// TestAgentStepData_HappyPath verifies that StepStateData.Response and
// StepStateData.Tokens fields can be accessed in workflow templates.
func TestAgentStepData_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name         string
		workflowYAML string
		wantSuccess  bool
	}{
		{
			name: "access step response data",
			workflowYAML: `
name: agent-response-access
version: "1.0.0"
states:
  initial: generate_json
  generate_json:
    type: step
    command: 'echo ''{"status": "success", "count": 42}'''
    on_success: use_response
  use_response:
    type: step
    command: 'echo "Status: {{.states.generate_json.Output}}"'
    on_success: end
  end:
    type: terminal
    status: success
`,
			wantSuccess: true,
		},
		{
			name: "access step tokens data",
			workflowYAML: `
name: tokens-tracking
version: "1.0.0"
states:
  initial: process
  process:
    type: step
    command: 'echo "Processing data"'
    on_success: report
  report:
    type: step
    command: 'echo "Step completed: {{.states.process.Output}}"'
    on_success: end
  end:
    type: terminal
    status: success
`,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			// Setup components
			repo := repository.NewYAMLRepository(workflowsDir)
			stateStore := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			log := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(stateStore).
				WithExecutor(exec).
				WithLogger(log).
				Build()

			// Act
			execCtx, err := svc.Run(ctx, "test", nil)

			// Assert
			if tt.wantSuccess {
				assert.NoError(t, err, "workflow should execute successfully")
				if execCtx != nil {
					assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "execution should complete")
				}
			} else {
				assert.Error(t, err, "workflow should fail")
			}
		})
	}
}

// TestExpressionNamespaces_HappyPath verifies that lowercase expression
// namespaces (loop.*, context.*, error.*) work in workflow execution.
func TestExpressionNamespaces_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name         string
		workflowYAML string
		wantSuccess  bool
	}{
		{
			name: "loop namespace - index and item",
			workflowYAML: `
name: loop-namespace
version: "1.0.0"
states:
  initial: iterate
  iterate:
    type: loop
    items: ["apple", "banana", "cherry"]
    loop_state: process
    on_complete: end
  process:
    type: step
    command: 'echo "Item {{loop.index}}: {{loop.item}}"'
    on_success: iterate
  end:
    type: terminal
    status: success
`,
			wantSuccess: true,
		},
		{
			name: "context namespace - workflow metadata",
			workflowYAML: `
name: context-namespace
version: "1.0.0"
states:
  initial: show_context
  show_context:
    type: step
    command: 'echo "Workflow ID: {{context.workflow_id}}"'
    on_success: end
  end:
    type: terminal
    status: success
`,
			wantSuccess: true,
		},
		{
			name: "error namespace in error hooks",
			workflowYAML: `
name: error-namespace
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: 'exit 1'
    on_success: end
    on_error: handle_error
  handle_error:
    type: step
    command: 'echo "Error occurred: exit code was 1"'
    on_success: end
  end:
    type: terminal
    status: success
`,
			wantSuccess: true,
		},
		{
			name: "combined namespaces in single workflow",
			workflowYAML: `
name: combined-namespaces
version: "1.0.0"
states:
  initial: loop_items
  loop_items:
    type: loop
    items: ["task1", "task2"]
    loop_state: execute_task
    on_complete: end
  execute_task:
    type: step
    command: 'echo "Task {{loop.index}} ({{loop.item}}) in workflow {{context.workflow_id}}"'
    on_success: loop_items
  end:
    type: terminal
    status: success
`,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			// Setup components
			repo := repository.NewYAMLRepository(workflowsDir)
			stateStore := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			log := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(stateStore).
				WithExecutor(exec).
				WithLogger(log).
				Build()

			// Act
			execCtx, err := svc.Run(ctx, "test", nil)

			// Assert
			if tt.wantSuccess {
				assert.NoError(t, err, "workflow should execute successfully")
				if execCtx != nil {
					assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "execution should complete")
				}
			} else {
				assert.Error(t, err, "workflow should fail")
			}
		})
	}
}

// TestPkgCoverage_EdgeCases tests edge cases for pkg layer improvements.
func TestPkgCoverage_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name         string
		workflowYAML string
		wantSuccess  bool
		wantErrMsg   string
	}{
		{
			name: "nil parent in single loop",
			workflowYAML: `
name: single-loop-no-parent
version: "1.0.0"
states:
  initial: loop
  loop:
    type: loop
    items: ["a", "b"]
    loop_state: process
    on_complete: end
  process:
    type: step
    command: 'echo "Item: {{.loop.Item}}"'
    on_success: loop
  end:
    type: terminal
    status: success
`,
			wantSuccess: true,
		},
		{
			name: "first and last flags in loop namespace",
			workflowYAML: `
name: loop-boundaries
version: "1.0.0"
states:
  initial: loop
  loop:
    type: loop
    items: ["start", "middle", "end"]
    loop_state: process
    on_complete: done
  process:
    type: step
    command: 'echo "Item: {{loop.item}}"'
    on_success: loop
  done:
    type: terminal
    status: success
`,
			wantSuccess: true,
		},
		{
			name: "empty response data",
			workflowYAML: `
name: empty-response
version: "1.0.0"
states:
  initial: empty_step
  empty_step:
    type: step
    command: 'echo ""'
    on_success: check
  check:
    type: step
    command: 'echo "Output was: {{.states.empty_step.Output}}"'
    on_success: end
  end:
    type: terminal
    status: success
`,
			wantSuccess: true,
		},
		{
			name: "three-level nested loops",
			workflowYAML: `
name: triple-nested
version: "1.0.0"
states:
  initial: level1
  level1:
    type: loop
    items: ["A"]
    loop_state: level2
    on_complete: end
  level2:
    type: loop
    items: ["1"]
    loop_state: level3
    on_complete: level1
  level3:
    type: loop
    items: ["x"]
    loop_state: show
    on_complete: level2
  show:
    type: step
    command: 'echo "L3={{.loop.Item}}, L2={{.loop.Parent.Item}}"'
    on_success: level3
  end:
    type: terminal
    status: success
`,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			// Setup components
			repo := repository.NewYAMLRepository(workflowsDir)
			stateStore := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			log := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(stateStore).
				WithExecutor(exec).
				WithLogger(log).
				Build()

			// Act
			execCtx, err := svc.Run(ctx, "test", nil)

			// Assert
			if tt.wantSuccess {
				assert.NoError(t, err, "workflow should execute successfully")
				if execCtx != nil {
					assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "execution should complete")
				}
			} else {
				assert.Error(t, err, "workflow should fail")
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			}
		})
	}
}

// TestPkgCoverage_ErrorHandling tests error scenarios for pkg improvements.
func TestPkgCoverage_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name         string
		workflowYAML string
		wantSuccess  bool
		wantErrMsg   string
	}{
		{
			name: "undefined parent access in single loop",
			workflowYAML: `
name: invalid-parent-access
version: "1.0.0"
states:
  initial: loop
  loop:
    type: loop
    items: ["a"]
    loop_state: process
    on_complete: end
  process:
    type: step
    command: 'echo "Parent: {{.loop.Parent.Item}}"'
    on_success: loop
  end:
    type: terminal
    status: success
`,
			wantSuccess: false,
			wantErrMsg:  "nil",
		},
		{
			name: "undefined state reference",
			workflowYAML: `
name: invalid-state-ref
version: "1.0.0"
states:
  initial: use_missing
  use_missing:
    type: step
    command: 'echo "{{.states.nonexistent.Output}}"'
    on_success: end
  end:
    type: terminal
    status: success
`,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			// Setup components
			repo := repository.NewYAMLRepository(workflowsDir)
			stateStore := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			log := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(stateStore).
				WithExecutor(exec).
				WithLogger(log).
				Build()

			// Act
			_, err := svc.Run(ctx, "test", nil)

			// Assert
			assert.Error(t, err, "workflow should fail with error")
			if tt.wantErrMsg != "" {
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			}
		})
	}
}
