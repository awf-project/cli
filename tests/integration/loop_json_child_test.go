//go:build integration

package integration_test

import (
	"context"
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

// US3: Integration validation for loop item JSON serialization
// Tests that the child workflow correctly validates JSON vs Go map format

// TestLoopJSONChild_HappyPath_ValidJSONInput tests the child workflow receives
// and validates a properly formatted JSON object.
// Item: T010
// Feature: F047
func TestLoopJSONChild_HappyPath_ValidJSONInput(t *testing.T) {
	// Given: Child workflow fixture exists
	fixturesDir := "../fixtures/workflows"
	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(
		wfSvc, exec, parallelExec, store, logger, resolver, nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When: Child workflow receives valid JSON object
	validJSON := `{"name":"S1","type":"fix","files":["a.go","b.go"]}`
	inputs := map[string]any{
		"item":  validJSON,
		"index": 0,
	}

	execCtx, err := execSvc.Run(ctx, "loop-json-child", inputs)

	// Then: Validation succeeds
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify output contains validation success
	validateState, exists := execCtx.States["validate"]
	require.True(t, exists, "validate state should exist")
	assert.Contains(t, validateState.Output, "VALIDATION_SUCCESS")
	assert.Contains(t, validateState.Output, "name=S1")
	assert.Contains(t, validateState.Output, "type=fix")
}

// TestLoopJSONChild_HappyPath_NestedJSON tests validation with nested objects.
// Item: T010
// Feature: F047
func TestLoopJSONChild_HappyPath_NestedJSON(t *testing.T) {
	// Given: Child workflow fixture
	fixturesDir := "../fixtures/workflows"
	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(
		wfSvc, exec, parallelExec, store, logger, resolver, nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When: Child receives nested JSON structure
	nestedJSON := `{"name":"S2","type":"feat","metadata":{"priority":"high","tags":["urgent","api"]},"files":["c.go"]}`
	inputs := map[string]any{
		"item":  nestedJSON,
		"index": 1,
	}

	execCtx, err := execSvc.Run(ctx, "loop-json-child", inputs)

	// Then: Validation succeeds with nested data
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	validateState, exists := execCtx.States["validate"]
	require.True(t, exists)
	assert.Contains(t, validateState.Output, "VALIDATION_SUCCESS")
	assert.Contains(t, validateState.Output, "name=S2")
	assert.Contains(t, validateState.Output, "type=feat")
}

// TestLoopJSONChild_HappyPath_JSONArray tests validation with array values.
// Item: T010
// Feature: F047
func TestLoopJSONChild_HappyPath_JSONArray(t *testing.T) {
	// Given: Child workflow fixture
	fixturesDir := "../fixtures/workflows"
	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(
		wfSvc, exec, parallelExec, store, logger, resolver, nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When: JSON object has array with multiple files
	arrayJSON := `{"name":"S3","type":"refactor","files":["a.go","b.go","c.go","d.go"]}`
	inputs := map[string]any{
		"item":  arrayJSON,
		"index": 2,
	}

	execCtx, err := execSvc.Run(ctx, "loop-json-child", inputs)

	// Then: Array is properly parsed
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	validateState, exists := execCtx.States["validate"]
	require.True(t, exists)
	assert.Contains(t, validateState.Output, "VALIDATION_SUCCESS")
	assert.Contains(t, validateState.Output, "files_count=4")
}

// TestLoopJSONChild_EdgeCase_EmptyFields tests JSON with empty field values.
// Item: T010
// Feature: F047
func TestLoopJSONChild_EdgeCase_EmptyFields(t *testing.T) {
	// Given: Child workflow fixture
	fixturesDir := "../fixtures/workflows"
	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(
		wfSvc, exec, parallelExec, store, logger, resolver, nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When: JSON has empty arrays or missing fields
	emptyJSON := `{"name":"S4","type":"","files":[]}`
	inputs := map[string]any{
		"item":  emptyJSON,
		"index": 3,
	}

	execCtx, err := execSvc.Run(ctx, "loop-json-child", inputs)

	// Then: Still valid JSON, validation succeeds
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	validateState, exists := execCtx.States["validate"]
	require.True(t, exists)
	assert.Contains(t, validateState.Output, "VALIDATION_SUCCESS")
	assert.Contains(t, validateState.Output, "files_count=0")
}

// TestLoopJSONChild_EdgeCase_UnicodeCharacters tests JSON with unicode/special chars.
// Item: T010
// Feature: F047
func TestLoopJSONChild_EdgeCase_UnicodeCharacters(t *testing.T) {
	// Given: Child workflow fixture
	fixturesDir := "../fixtures/workflows"
	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(
		wfSvc, exec, parallelExec, store, logger, resolver, nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When: JSON contains unicode characters
	unicodeJSON := `{"name":"S5-✨","type":"feat","files":["café.go","日本語.go"]}`
	inputs := map[string]any{
		"item":  unicodeJSON,
		"index": 4,
	}

	execCtx, err := execSvc.Run(ctx, "loop-json-child", inputs)

	// Then: Unicode is properly handled
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	validateState, exists := execCtx.States["validate"]
	require.True(t, exists)
	assert.Contains(t, validateState.Output, "VALIDATION_SUCCESS")
}

// TestLoopJSONChild_EdgeCase_MinimalJSON tests minimal valid JSON object.
// Item: T010
// Feature: F047
func TestLoopJSONChild_EdgeCase_MinimalJSON(t *testing.T) {
	// Given: Child workflow fixture
	fixturesDir := "../fixtures/workflows"
	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(
		wfSvc, exec, parallelExec, store, logger, resolver, nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When: Minimal JSON object with only required field
	minimalJSON := `{}`
	inputs := map[string]any{
		"item":  minimalJSON,
		"index": 5,
	}

	execCtx, err := execSvc.Run(ctx, "loop-json-child", inputs)

	// Then: Empty object is still valid JSON
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	validateState, exists := execCtx.States["validate"]
	require.True(t, exists)
	assert.Contains(t, validateState.Output, "VALIDATION_SUCCESS")
}

// TestLoopJSONChild_ErrorHandling_GoMapFormat tests detection of Go map format.
// This is the CRITICAL test - ensures the bug is detected.
// Item: T010
// Feature: F047
func TestLoopJSONChild_ErrorHandling_GoMapFormat(t *testing.T) {
	// Given: Child workflow fixture
	fixturesDir := "../fixtures/workflows"
	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(
		wfSvc, exec, parallelExec, store, logger, resolver, nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When: Item is in Go map format (the bug we're testing for)
	goMapFormat := `map[name:S1 type:fix files:[a.go b.go]]`
	inputs := map[string]any{
		"item":  goMapFormat,
		"index": 0,
	}

	execCtx, err := execSvc.Run(ctx, "loop-json-child", inputs)

	// Then: Workflow should FAIL with validation error
	// This test MUST FAIL until F047 is implemented
	require.Error(t, err, "Go map format should be rejected")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Equal(t, "error", execCtx.CurrentStep)

	// Verify error message indicates Go map format
	validateState, exists := execCtx.States["validate"]
	require.True(t, exists)
	assert.Contains(t, validateState.Output, "VALIDATION_FAILED")
	assert.Contains(t, validateState.Output, "Go map format")
}

// TestLoopJSONChild_ErrorHandling_InvalidJSON tests malformed JSON detection.
// Item: T010
// Feature: F047
func TestLoopJSONChild_ErrorHandling_InvalidJSON(t *testing.T) {
	// Given: Child workflow fixture
	fixturesDir := "../fixtures/workflows"
	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(
		wfSvc, exec, parallelExec, store, logger, resolver, nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tests := []struct {
		name        string
		invalidJSON string
	}{
		{"missing_brace", `{"name":"S1","type":"fix"`},
		{"trailing_comma", `{"name":"S1","type":"fix",}`},
		{"single_quotes", `{'name':'S1','type':'fix'}`},
		{"unquoted_keys", `{name:"S1",type:"fix"}`},
		{"plain_text", `just some text`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: Invalid JSON is provided
			inputs := map[string]any{
				"item":  tt.invalidJSON,
				"index": 99,
			}

			execCtx, err := execSvc.Run(ctx, "loop-json-child", inputs)

			// Then: Validation fails
			require.Error(t, err, "invalid JSON should be rejected")
			assert.Equal(t, workflow.StatusFailed, execCtx.Status)

			validateState, exists := execCtx.States["validate"]
			require.True(t, exists)
			assert.Contains(t, validateState.Output, "VALIDATION_FAILED")
			assert.Contains(t, validateState.Output, "not valid JSON")
		})
	}
}

// TestLoopJSONChild_ErrorHandling_MissingRequiredInput tests behavior with missing item.
// Item: T010
// Feature: F047
func TestLoopJSONChild_ErrorHandling_MissingRequiredInput(t *testing.T) {
	// Given: Child workflow fixture
	fixturesDir := "../fixtures/workflows"
	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(
		wfSvc, exec, parallelExec, store, logger, resolver, nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When: Required 'item' input is missing
	inputs := map[string]any{
		"index": 0,
		// "item" is missing
	}

	_, err := execSvc.Run(ctx, "loop_json_child", inputs)

	// Then: Should fail with input validation error
	require.Error(t, err, "missing required input should fail")
	assert.True(t,
		strings.Contains(err.Error(), "item") ||
			strings.Contains(err.Error(), "required") ||
			strings.Contains(err.Error(), "input"),
		"error should mention missing required input")
}

// TestLoopJSONChild_EdgeCase_IndexParameter tests optional index parameter.
// Item: T010
// Feature: F047
func TestLoopJSONChild_EdgeCase_IndexParameter(t *testing.T) {
	// Given: Child workflow fixture
	fixturesDir := "../fixtures/workflows"
	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(
		wfSvc, exec, parallelExec, store, logger, resolver, nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tests := []struct {
		name  string
		index any
	}{
		{"explicit_index_0", 0},
		{"explicit_index_5", 5},
		{"omitted_index_defaults", nil}, // Should use default value 0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: Index is provided or omitted
			inputs := map[string]any{
				"item": `{"name":"test","type":"test","files":[]}`,
			}
			if tt.index != nil {
				inputs["index"] = tt.index
			}

			execCtx, err := execSvc.Run(ctx, "loop-json-child", inputs)

			// Then: Workflow succeeds regardless of index value
			require.NoError(t, err)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			validateState, exists := execCtx.States["validate"]
			require.True(t, exists)
			assert.Contains(t, validateState.Output, "VALIDATION_SUCCESS")
		})
	}
}

// TestLoopJSONChild_Integration_LoadsAndValidates verifies fixture can be loaded.
// Item: T010
// Feature: F047
func TestLoopJSONChild_Integration_LoadsAndValidates(t *testing.T) {
	// Given: Fixtures directory
	fixturesDir := "../fixtures/workflows"
	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// When: Loading loop-json-child workflow
	wf, err := wfSvc.GetWorkflow(ctx, "loop-json-child")

	// Then: Workflow loads successfully
	require.NoError(t, err, "loop-json-child.yaml should load without errors")
	require.NotNil(t, wf, "workflow should not be nil")
	assert.Equal(t, "loop-json-child", wf.Name)
	assert.Equal(t, "1.0.0", wf.Version)

	// Verify inputs are defined correctly
	require.Len(t, wf.Inputs, 2, "should have 2 inputs: item and index")
	assert.Equal(t, "item", wf.Inputs[0].Name)
	assert.True(t, wf.Inputs[0].Required)
	assert.Equal(t, "index", wf.Inputs[1].Name)
	assert.False(t, wf.Inputs[1].Required)

	// Verify steps structure (Steps, not States)
	require.Contains(t, wf.Steps, "validate", "should have validate step")
	require.Contains(t, wf.Steps, "done", "should have done step")
	require.Contains(t, wf.Steps, "error", "should have error step")

	// Validate the workflow structure
	err = wfSvc.ValidateWorkflow(ctx, "loop-json-child")
	assert.NoError(t, err, "workflow should pass validation")
}
