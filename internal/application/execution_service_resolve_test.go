package application_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/testutil"
)

// TestResolveOperationValue tests the recursive interpolation resolution
// across string, map, array, and primitive types.
//
// Coverage target: 11.8% -> 80%+
//
// This function is private (resolveOperationValue), so we test it indirectly
// through public API by constructing workflows with operation steps whose
// OperationInputs contain the target value types. The mock OperationProvider
// validates that resolved inputs match expectations.
func TestResolveOperationValue(t *testing.T) {
	tests := []struct {
		name               string
		operationInputs    map[string]any
		workflowInputs     map[string]any
		expectedResolve    map[string]any
		wantErr            bool
		errorContains      string
		mockOperationLogic func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error)
	}{
		{
			name: "string value with template interpolation",
			operationInputs: map[string]any{
				"message": "Hello {{inputs.name}}!",
			},
			workflowInputs: map[string]any{
				"name": "World",
			},
			expectedResolve: map[string]any{
				"message": "Hello World!",
			},
			wantErr: false,
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				// Verify the resolved input
				assert.Equal(t, "Hello World!", inputs["message"])
				return &plugin.OperationResult{Success: true}, nil
			},
		},
		{
			name: "map with nested string values",
			operationInputs: map[string]any{
				"config": map[string]any{
					"key1": "value1",
					"key2": "User: {{inputs.user}}",
					"key3": map[string]any{
						"nested": "Nested {{inputs.value}}",
					},
				},
			},
			workflowInputs: map[string]any{
				"user":  "Alice",
				"value": "data",
			},
			expectedResolve: map[string]any{
				"config": map[string]any{
					"key1": "value1",
					"key2": "User: Alice",
					"key3": map[string]any{
						"nested": "Nested data",
					},
				},
			},
			wantErr: false,
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				config, ok := inputs["config"].(map[string]any)
				require.True(t, ok, "config should be a map")
				assert.Equal(t, "value1", config["key1"])
				assert.Equal(t, "User: Alice", config["key2"])
				nested, ok := config["key3"].(map[string]any)
				require.True(t, ok, "nested should be a map")
				assert.Equal(t, "Nested data", nested["nested"])
				return &plugin.OperationResult{Success: true}, nil
			},
		},
		{
			name: "array of mixed types",
			operationInputs: map[string]any{
				"items": []any{
					"item1",
					"{{inputs.item2}}",
					42,
					true,
					map[string]any{"key": "{{inputs.value}}"},
				},
			},
			workflowInputs: map[string]any{
				"item2": "dynamic-item",
				"value": "resolved",
			},
			expectedResolve: map[string]any{
				"items": []any{
					"item1",
					"dynamic-item",
					42,
					true,
					map[string]any{"key": "resolved"},
				},
			},
			wantErr: false,
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				items, ok := inputs["items"].([]any)
				require.True(t, ok, "items should be an array")
				assert.Len(t, items, 5)
				assert.Equal(t, "item1", items[0])
				assert.Equal(t, "dynamic-item", items[1])
				assert.Equal(t, 42, items[2])
				assert.Equal(t, true, items[3])
				itemMap, ok := items[4].(map[string]any)
				require.True(t, ok, "items[4] should be a map")
				assert.Equal(t, "resolved", itemMap["key"])
				return &plugin.OperationResult{Success: true}, nil
			},
		},
		{
			name: "non-string primitive passthrough (int, bool, nil)",
			operationInputs: map[string]any{
				"count":   42,
				"enabled": true,
				"data":    nil,
				"float":   3.14,
			},
			workflowInputs: map[string]any{},
			expectedResolve: map[string]any{
				"count":   42,
				"enabled": true,
				"data":    nil,
				"float":   3.14,
			},
			wantErr: false,
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				assert.Equal(t, 42, inputs["count"])
				assert.Equal(t, true, inputs["enabled"])
				assert.Nil(t, inputs["data"])
				assert.Equal(t, 3.14, inputs["float"])
				return &plugin.OperationResult{Success: true}, nil
			},
		},
		{
			name: "nested map with interpolation error",
			operationInputs: map[string]any{
				"config": map[string]any{
					"value": "{{inputs.missing}}",
				},
			},
			workflowInputs: map[string]any{},
			wantErr:        true,
			errorContains:  "input \"config\"",
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				t.Fatal("should not reach operation execution")
				return nil, nil
			},
		},
		{
			name: "nested array with interpolation error",
			operationInputs: map[string]any{
				"items": []any{
					"valid",
					"{{inputs.undefined}}",
				},
			},
			workflowInputs: map[string]any{},
			wantErr:        true,
			errorContains:  "input \"items\"",
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				t.Fatal("should not reach operation execution")
				return nil, nil
			},
		},
		{
			name: "empty map and empty array",
			operationInputs: map[string]any{
				"empty_map":   map[string]any{},
				"empty_array": []any{},
			},
			workflowInputs: map[string]any{},
			expectedResolve: map[string]any{
				"empty_map":   map[string]any{},
				"empty_array": []any{},
			},
			wantErr: false,
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				emptyMap, ok := inputs["empty_map"].(map[string]any)
				require.True(t, ok, "empty_map should be a map")
				assert.Empty(t, emptyMap)
				emptyArray, ok := inputs["empty_array"].([]any)
				require.True(t, ok, "empty_array should be an array")
				assert.Empty(t, emptyArray)
				return &plugin.OperationResult{Success: true}, nil
			},
		},
		{
			name: "deeply nested structure with multiple interpolations",
			operationInputs: map[string]any{
				"deep": map[string]any{
					"level1": []any{
						map[string]any{
							"level2": "{{inputs.a}}",
							"level2b": []any{
								"{{inputs.b}}",
								map[string]any{
									"level3": "{{inputs.c}}",
								},
							},
						},
					},
				},
			},
			workflowInputs: map[string]any{
				"a": "value-a",
				"b": "value-b",
				"c": "value-c",
			},
			wantErr: false,
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				deep := inputs["deep"].(map[string]any)
				level1 := deep["level1"].([]any)
				level1Map := level1[0].(map[string]any)
				assert.Equal(t, "value-a", level1Map["level2"])
				level2b := level1Map["level2b"].([]any)
				assert.Equal(t, "value-b", level2b[0])
				level2bMap := level2b[1].(map[string]any)
				assert.Equal(t, "value-c", level2bMap["level3"])
				return &plugin.OperationResult{Success: true}, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Create workflow with operation step
			wf := testutil.NewWorkflowBuilder().
				WithName("resolve-test").
				WithInitial("op").
				WithStep(
					testutil.NewStepBuilder("op").
						WithType(workflow.StepTypeOperation).
						WithOperation("test.operation", tt.operationInputs).
						WithOnSuccess("done").
						Build(),
				).
				WithStep(
					testutil.NewTerminalStep("done", workflow.TerminalSuccess).Build(),
				).
				Build()

			// Create mock operation provider with the test's logic
			provider := &MockOperationProvider{}
			mockOp := &MockOperation{
				ExecuteFunc: tt.mockOperationLogic,
			}
			provider.SetOperation("test.operation", mockOp)

			// Create service with harness
			svc, _ := NewTestHarness(t).
				WithWorkflow("resolve-test", wf).
				Build()
			svc.SetOperationProvider(provider)

			// Act: Execute workflow
			_, err := svc.Run(context.Background(), "resolve-test", tt.workflowInputs)

			// Assert
			if tt.wantErr {
				require.Error(t, err, "expected error but got none")
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err, "unexpected error: %v", err)
				// Successful execution means the mock's assertions passed
				assert.Equal(t, 1, provider.ExecuteCallCount, "operation should be called once")
				assert.Equal(t, "test.operation", provider.LastOperation)
			}
		})
	}
}

// TestResolveOperationValue_EdgeCases tests edge cases for recursive interpolation resolution.
func TestResolveOperationValue_EdgeCases(t *testing.T) {
	tests := []struct {
		name               string
		operationInputs    map[string]any
		workflowInputs     map[string]any
		wantErr            bool
		mockOperationLogic func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error)
	}{
		{
			name:            "nil operation inputs",
			operationInputs: nil,
			workflowInputs:  map[string]any{"key": "value"},
			wantErr:         false,
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				// Nil inputs should result in empty map or nil
				return &plugin.OperationResult{Success: true}, nil
			},
		},
		{
			name: "special characters in string interpolation",
			operationInputs: map[string]any{
				"value": "Special: {{inputs.special}}",
			},
			workflowInputs: map[string]any{
				"special": "!@#$%^&*()",
			},
			wantErr: false,
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				assert.Equal(t, "Special: !@#$%^&*()", inputs["value"])
				return &plugin.OperationResult{Success: true}, nil
			},
		},
		{
			name: "numeric keys in map",
			operationInputs: map[string]any{
				"123":   "numeric-key",
				"key-2": "{{inputs.value}}",
			},
			workflowInputs: map[string]any{
				"value": "resolved",
			},
			wantErr: false,
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				assert.Equal(t, "numeric-key", inputs["123"])
				assert.Equal(t, "resolved", inputs["key-2"])
				return &plugin.OperationResult{Success: true}, nil
			},
		},
		{
			name: "malformed template syntax",
			operationInputs: map[string]any{
				"value": "{{inputs.incomplete",
			},
			workflowInputs: map[string]any{},
			wantErr:        true,
			mockOperationLogic: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				t.Fatal("should not reach operation execution")
				return nil, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			wf := testutil.NewWorkflowBuilder().
				WithName("edge-test").
				WithInitial("op").
				WithStep(
					testutil.NewStepBuilder("op").
						WithType(workflow.StepTypeOperation).
						WithOperation("test.edge", tt.operationInputs).
						WithOnSuccess("done").
						Build(),
				).
				WithStep(
					testutil.NewTerminalStep("done", workflow.TerminalSuccess).Build(),
				).
				Build()

			provider := &MockOperationProvider{}
			mockOp := &MockOperation{
				ExecuteFunc: tt.mockOperationLogic,
			}
			provider.SetOperation("test.edge", mockOp)

			svc, _ := NewTestHarness(t).
				WithWorkflow("edge-test", wf).
				Build()
			svc.SetOperationProvider(provider)

			// Act
			_, err := svc.Run(context.Background(), "edge-test", tt.workflowInputs)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestResolveOperationValue_ErrorPropagation tests that interpolation errors
// are properly propagated with context about which input field failed.
func TestResolveOperationValue_ErrorPropagation(t *testing.T) {
	tests := []struct {
		name            string
		operationInputs map[string]any
		workflowInputs  map[string]any
		errorField      string // Expected field name in error message
	}{
		{
			name: "error in top-level string",
			operationInputs: map[string]any{
				"field1": "{{inputs.missing}}",
			},
			workflowInputs: map[string]any{},
			errorField:     "field1",
		},
		{
			name: "error in nested map",
			operationInputs: map[string]any{
				"outer": map[string]any{
					"inner": "{{inputs.undefined}}",
				},
			},
			workflowInputs: map[string]any{},
			errorField:     "outer",
		},
		{
			name: "error in array element",
			operationInputs: map[string]any{
				"list": []any{
					"valid",
					"{{inputs.notfound}}",
				},
			},
			workflowInputs: map[string]any{},
			errorField:     "list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			wf := testutil.NewWorkflowBuilder().
				WithName("error-test").
				WithInitial("op").
				WithStep(
					testutil.NewStepBuilder("op").
						WithType(workflow.StepTypeOperation).
						WithOperation("test.error", tt.operationInputs).
						WithOnSuccess("done").
						Build(),
				).
				WithStep(
					testutil.NewTerminalStep("done", workflow.TerminalSuccess).Build(),
				).
				Build()

			provider := &MockOperationProvider{}
			provider.SetOperation("test.error", &MockOperation{
				ExecuteFunc: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
					t.Fatal("should not reach operation execution on error")
					return nil, nil
				},
			})

			svc, _ := NewTestHarness(t).
				WithWorkflow("error-test", wf).
				Build()
			svc.SetOperationProvider(provider)

			// Act
			_, err := svc.Run(context.Background(), "error-test", tt.workflowInputs)

			// Assert
			require.Error(t, err, "expected error from interpolation failure")
			assert.Contains(t, err.Error(), fmt.Sprintf("input %q", tt.errorField),
				"error should include field name that failed")
		})
	}
}
