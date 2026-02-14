//go:build integration

package loops_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	infraExpr "github.com/vanoix/awf/internal/infrastructure/expression"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/pkg/interpolation"
)

// Functional Tests - End-to-End Scenarios
//
// These tests validate the complete feature implementation across real-world
// usage patterns, ensuring all acceptance criteria are met:
//
// AC1: {{.loop.Item}} passed to call_workflow produces valid JSON
// AC2: Nested objects and arrays properly serialized
// AC3: Unit tests cover serialization scenarios (see pkg/interpolation tests)
// AC4: Existing workflows work without workarounds

// TestHappyPath_CallWorkflowWithJSONObjects tests AC1:
// for_each loop passing JSON objects to call_workflow via {{.loop.Item}}
// Feature: F047
func TestHappyPath_CallWorkflowWithJSONObjects(t *testing.T) {
	// Given: Parent workflow iterates over JSON objects and calls child workflow
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	err := os.MkdirAll(outputDir, 0o755)
	require.NoError(t, err)

	// Create child workflow that validates and processes JSON item
	childYAML := `name: process-item
version: "1.0.0"
inputs:
  - name: item
    type: string
    required: true
outputs:
  - name: result
    from: states.process.output
states:
  initial: process
  process:
    type: step
    command: |
      item='{{.inputs.item}}'
      # Validate it's JSON, not Go map format
      if echo "$item" | grep -q '^map\['; then
        echo "ERROR: Received Go map format" >&2
        exit 1
      fi
      # Parse with jq to ensure valid JSON
      name=$(echo "$item" | jq -r '.name')
      type=$(echo "$item" | jq -r '.type')
      echo "Processed: $name ($type)"
    on_success: done
  done:
    type: terminal
    status: success
`
	err = os.WriteFile(filepath.Join(tmpDir, "process-item.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent workflow with for_each calling child
	parentYAML := `name: parent
version: "1.0.0"
states:
  initial: extract_items
  extract_items:
    type: step
    command: echo '[{"name":"S1","type":"fix","files":["a.go"]},{"name":"S2","type":"feat","files":["b.go","c.go"]}]'
    on_success: loop_items
  loop_items:
    type: for_each
    items: "{{.states.extract_items.Output}}"
    body:
      - call_child
    on_complete: done
  call_child:
    type: call_workflow
    workflow: process-item
    inputs:
      item: "{{.loop.Item}}"
    on_success: loop_items
  done:
    type: terminal
    status: success
`
	err = os.WriteFile(filepath.Join(tmpDir, "parent.yaml"), []byte(parentYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// When: Parent workflow executes
	execCtx, err := execSvc.Run(ctx, "parent", nil)

	// Then: Workflow completes successfully
	require.NoError(t, err, "call_workflow with JSON items should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify child received valid JSON (no errors about Go map format)
	callChildState, exists := execCtx.GetStepState("call_child")
	require.True(t, exists, "call_child state should exist")
	assert.NotContains(t, callChildState.Output, "Go map format", "child should not receive Go map format")
}

// TestHappyPath_NestedObjectsAndArrays tests AC2:
// Complex nested structures serialize correctly
// Feature: F047
func TestHappyPath_NestedObjectsAndArrays(t *testing.T) {
	// Given: Workflow with deeply nested JSON structures
	tmpDir := t.TempDir()

	wfYAML := `name: nested-json
version: "1.0.0"
states:
  initial: create_nested
  create_nested:
    type: step
    command: echo '[{"task":"Feature1","metadata":{"priority":"high","tags":["urgent","bug"],"assignees":[{"id":1,"name":"Alice","roles":["dev","reviewer"]},{"id":2,"name":"Bob","roles":["qa"]}]},"files":["a.go","b.go"]}]'
    on_success: loop_items
  loop_items:
    type: for_each
    items: "{{.states.create_nested.Output}}"
    body:
      - validate_nested
    on_complete: done
  validate_nested:
    type: step
    command: |
      item='{{.loop.Item}}'
      # Verify it's valid JSON
      if ! echo "$item" | jq empty 2>/dev/null; then
        echo "Invalid JSON" >&2
        exit 1
      fi
      # Verify nested structures are preserved
      task=$(echo "$item" | jq -r '.task')
      priority=$(echo "$item" | jq -r '.metadata.priority')
      tag_count=$(echo "$item" | jq -r '.metadata.tags | length')
      assignee_count=$(echo "$item" | jq -r '.metadata.assignees | length')
      first_assignee_role_count=$(echo "$item" | jq -r '.metadata.assignees[0].roles | length')

      echo "Valid: task=$task, priority=$priority, tags=$tag_count, assignees=$assignee_count, first_roles=$first_assignee_role_count"

      # Assertions
      [ "$task" = "Feature1" ] || exit 1
      [ "$priority" = "high" ] || exit 1
      [ "$tag_count" = "2" ] || exit 1
      [ "$assignee_count" = "2" ] || exit 1
      [ "$first_assignee_role_count" = "2" ] || exit 1
    on_success: loop_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-json.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// When: Workflow with nested structures executes
	execCtx, err := execSvc.Run(ctx, "nested-json", nil)

	// Then: Nested structures are properly serialized and parsed
	require.NoError(t, err, "nested structures should serialize correctly")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	validateState, exists := execCtx.GetStepState("validate_nested")
	require.True(t, exists)
	assert.Contains(t, validateState.Output, "Valid:", "nested JSON should validate")
}

// TestEdgeCase_EmptyObjectsAndArrays tests edge cases with empty structures
// Feature: F047
func TestEdgeCase_EmptyObjectsAndArrays(t *testing.T) {
	// Given: Items with empty objects and arrays
	tmpDir := t.TempDir()

	wfYAML := `name: empty-structures
version: "1.0.0"
states:
  initial: loop_items
  loop_items:
    type: for_each
    items: '[{}, {"data":[]}, {"nested":{"values":[]}}]'
    body:
      - validate_empty
    on_complete: done
  validate_empty:
    type: step
    command: |
      item='{{.loop.Item}}'
      # Must be valid JSON
      if ! echo "$item" | jq empty 2>/dev/null; then
        echo "Not valid JSON" >&2
        exit 1
      fi
      echo "Valid empty structure: $item"
    on_success: loop_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "empty-structures.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// When: Processing empty structures
	execCtx, err := execSvc.Run(ctx, "empty-structures", nil)

	// Then: Empty structures serialize as valid JSON
	require.NoError(t, err, "empty structures should be valid JSON")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

// TestEdgeCase_PrimitiveTypes tests AC4: primitives pass through unchanged
// Feature: F047
func TestEdgeCase_PrimitiveTypes(t *testing.T) {
	// Given: Loop items are primitive types (not objects)
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	tests := []struct {
		name     string
		items    string
		expected []string
	}{
		{
			name:     "strings",
			items:    `["string1", "string2", "string3"]`,
			expected: []string{"string1", "string2", "string3"},
		},
		{
			name:     "numbers",
			items:    `[1, 2, 3]`,
			expected: []string{"1", "2", "3"},
		},
		{
			name:     "booleans",
			items:    `[true, false, true]`,
			expected: []string{"true", "false", "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean output file
			_ = os.Remove(outputFile)

			wfYAML := `name: primitive-items
version: "1.0.0"
states:
  initial: loop
  loop:
    type: for_each
    items: '` + tt.items + `'
    body:
      - echo_item
    on_complete: done
  echo_item:
    type: step
    command: 'echo "{{.loop.Item}}" >> ` + outputFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`
			err := os.WriteFile(filepath.Join(tmpDir, "primitive-items.yaml"), []byte(wfYAML), 0o644)
			require.NoError(t, err)

			repo := repository.NewYAMLRepository(tmpDir)
			store := newMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &mockLogger{}
			resolver := interpolation.NewTemplateResolver()
			evaluator := infraExpr.NewExprEvaluator()

			wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
			parallelExec := application.NewParallelExecutor(logger)
			execSvc := application.NewExecutionServiceWithEvaluator(
				wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
			)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// When: Processing primitive items
			execCtx, err := execSvc.Run(ctx, "primitive-items", nil)

			// Then: Primitives pass through unchanged (not JSON-encoded)
			require.NoError(t, err, "primitive items should work")
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			data, err := os.ReadFile(outputFile)
			require.NoError(t, err)
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")

			require.Len(t, lines, len(tt.expected))
			for i, expected := range tt.expected {
				assert.Equal(t, expected, lines[i], "primitive should not be JSON-encoded")
			}
		})
	}
}

// TestEdgeCase_UnicodeAndSpecialCharacters tests unicode handling
// Feature: F047
func TestEdgeCase_UnicodeAndSpecialCharacters(t *testing.T) {
	// Given: Items with unicode characters
	tmpDir := t.TempDir()

	wfYAML := `name: unicode-test
version: "1.0.0"
states:
  initial: loop
  loop:
    type: for_each
    items: '[{"name":"测试","emoji":"🚀🎉","desc":"Café \"special\" quotes"}]'
    body:
      - validate_unicode
    on_complete: done
  validate_unicode:
    type: step
    command: |
      item='{{.loop.Item}}'
      # Validate JSON
      if ! echo "$item" | jq empty 2>/dev/null; then
        echo "Invalid JSON with unicode" >&2
        exit 1
      fi
      # Extract fields to verify unicode preservation
      name=$(echo "$item" | jq -r '.name')
      emoji=$(echo "$item" | jq -r '.emoji')
      echo "Unicode preserved: name=$name, emoji=$emoji"
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "unicode-test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// When: Processing unicode characters
	execCtx, err := execSvc.Run(ctx, "unicode-test", nil)

	// Then: Unicode is properly handled in JSON
	require.NoError(t, err, "unicode should be handled correctly")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	validateState, exists := execCtx.GetStepState("validate_unicode")
	require.True(t, exists)
	assert.Contains(t, validateState.Output, "Unicode preserved")
}

// TestIntegration_RealWorldWorkflowPattern tests a realistic scenario
// Feature: F047
func TestIntegration_RealWorldWorkflowPattern(t *testing.T) {
	// Given: A realistic PR review workflow pattern
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "review-results.txt")

	// Child workflow: review a single file
	reviewFileYAML := `name: review-file
version: "1.0.0"
inputs:
  - name: file_info
    type: string
    required: true
outputs:
  - name: review_result
    from: states.review.output
states:
  initial: review
  review:
    type: step
    command: |
      file_info='{{.inputs.file_info}}'
      # Parse JSON
      path=$(echo "$file_info" | jq -r '.path')
      type=$(echo "$file_info" | jq -r '.type')
      lines=$(echo "$file_info" | jq -r '.lines')
      echo "Reviewed: $path ($type, $lines lines)" >> ` + outputFile + `
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "review-file.yaml"), []byte(reviewFileYAML), 0o644)
	require.NoError(t, err)

	// Parent workflow: PR review orchestrator
	prReviewYAML := `name: pr-review
version: "1.0.0"
states:
  initial: get_changed_files
  get_changed_files:
    type: step
    command: echo '[{"path":"src/main.go","type":"modified","lines":42},{"path":"pkg/util.go","type":"added","lines":15},{"path":"README.md","type":"modified","lines":3}]'
    on_success: review_loop
  review_loop:
    type: for_each
    items: "{{.states.get_changed_files.Output}}"
    body:
      - review_file
    on_complete: done
  review_file:
    type: call_workflow
    workflow: review-file
    inputs:
      file_info: "{{.loop.Item}}"
    on_success: review_loop
  done:
    type: terminal
    status: success
`
	err = os.WriteFile(filepath.Join(tmpDir, "pr-review.yaml"), []byte(prReviewYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// When: PR review workflow executes
	execCtx, err := execSvc.Run(ctx, "pr-review", nil)

	// Then: All files are reviewed successfully
	require.NoError(t, err, "PR review workflow should complete")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify all files were processed
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	output := string(data)

	assert.Contains(t, output, "src/main.go (modified, 42 lines)")
	assert.Contains(t, output, "pkg/util.go (added, 15 lines)")
	assert.Contains(t, output, "README.md (modified, 3 lines)")

	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 3, "should have reviewed 3 files")
}

// TestJSONSerialization_BackwardCompatibility tests AC4:
// Workflows created before F047 continue to work unchanged
// Feature: F047
func TestJSONSerialization_BackwardCompatibility(t *testing.T) {
	// Given: Pre-F047 workflows with simple string/number items
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	tests := []struct {
		name     string
		workflow string
		expected []string
	}{
		{
			name: "simple_strings",
			workflow: `name: simple-strings
version: "1.0.0"
states:
  initial: loop
  loop:
    type: for_each
    items: '["file1.go", "file2.go", "file3.go"]'
    body:
      - process
    on_complete: done
  process:
    type: step
    command: 'echo "Processing {{.loop.Item}}" >> ` + outputFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`,
			expected: []string{
				"Processing file1.go",
				"Processing file2.go",
				"Processing file3.go",
			},
		},
		{
			name: "numeric_iterations",
			workflow: `name: numeric-iterations
version: "1.0.0"
states:
  initial: loop
  loop:
    type: for_each
    items: '[1, 2, 3, 4, 5]'
    body:
      - process
    on_complete: done
  process:
    type: step
    command: 'echo "Iteration {{.loop.Item}}" >> ` + outputFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`,
			expected: []string{
				"Iteration 1",
				"Iteration 2",
				"Iteration 3",
				"Iteration 4",
				"Iteration 5",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean output file
			_ = os.Remove(outputFile)

			err := os.WriteFile(filepath.Join(tmpDir, tt.name+".yaml"), []byte(tt.workflow), 0o644)
			require.NoError(t, err)

			repo := repository.NewYAMLRepository(tmpDir)
			store := newMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &mockLogger{}
			resolver := interpolation.NewTemplateResolver()
			evaluator := infraExpr.NewExprEvaluator()

			wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
			parallelExec := application.NewParallelExecutor(logger)
			execSvc := application.NewExecutionServiceWithEvaluator(
				wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
			)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// When: Running pre-F047 workflow
			execCtx, err := execSvc.Run(ctx, tt.name, nil)

			// Then: Workflow works exactly as before
			require.NoError(t, err, "pre-F047 workflow should still work")
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			data, err := os.ReadFile(outputFile)
			require.NoError(t, err)
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")

			require.Len(t, lines, len(tt.expected))
			for i, expected := range tt.expected {
				assert.Equal(t, expected, lines[i], "output should match pre-F047 behavior")
			}
		})
	}
}
