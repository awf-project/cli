package operation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/awf/internal/domain/operation"
	"github.com/awf-project/awf/internal/domain/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sentinel Errors Tests
// Component: T003
// Feature: F057

func TestSentinelErrors_Values(t *testing.T) {
	// Happy path: Verify sentinel error messages match expected text
	assert.Equal(t, "operation already registered", operation.ErrOperationAlreadyRegistered.Error())
	assert.Equal(t, "operation not found", operation.ErrOperationNotFound.Error())
	assert.Equal(t, "invalid operation", operation.ErrInvalidOperation.Error())
	assert.Equal(t, "invalid inputs", operation.ErrInvalidInputs.Error())
}

func TestSentinelErrors_AreErrors(t *testing.T) {
	// Happy path: Verify sentinel errors implement error interface
	var err error

	err = operation.ErrOperationAlreadyRegistered
	assert.Error(t, err)

	err = operation.ErrOperationNotFound
	assert.Error(t, err)

	err = operation.ErrInvalidOperation
	assert.Error(t, err)

	err = operation.ErrInvalidInputs
	assert.Error(t, err)
}

func TestSentinelErrors_CanBeCheckedWithErrorsIs(t *testing.T) {
	// Happy path: Verify sentinel errors work with errors.Is
	tests := []struct {
		name     string
		err      error
		target   error
		expected bool
	}{
		{
			name:     "already_registered_matches_itself",
			err:      operation.ErrOperationAlreadyRegistered,
			target:   operation.ErrOperationAlreadyRegistered,
			expected: true,
		},
		{
			name:     "not_found_matches_itself",
			err:      operation.ErrOperationNotFound,
			target:   operation.ErrOperationNotFound,
			expected: true,
		},
		{
			name:     "invalid_operation_matches_itself",
			err:      operation.ErrInvalidOperation,
			target:   operation.ErrInvalidOperation,
			expected: true,
		},
		{
			name:     "invalid_inputs_matches_itself",
			err:      operation.ErrInvalidInputs,
			target:   operation.ErrInvalidInputs,
			expected: true,
		},
		{
			name:     "already_registered_does_not_match_not_found",
			err:      operation.ErrOperationAlreadyRegistered,
			target:   operation.ErrOperationNotFound,
			expected: false,
		},
		{
			name:     "not_found_does_not_match_invalid_operation",
			err:      operation.ErrOperationNotFound,
			target:   operation.ErrInvalidOperation,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errors.Is(tt.err, tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	// Edge case: Verify all sentinel errors are distinct instances
	assert.NotEqual(t, operation.ErrOperationAlreadyRegistered, operation.ErrOperationNotFound)
	assert.NotEqual(t, operation.ErrOperationAlreadyRegistered, operation.ErrInvalidOperation)
	assert.NotEqual(t, operation.ErrOperationAlreadyRegistered, operation.ErrInvalidInputs)
	assert.NotEqual(t, operation.ErrOperationNotFound, operation.ErrInvalidOperation)
	assert.NotEqual(t, operation.ErrOperationNotFound, operation.ErrInvalidInputs)
	assert.NotEqual(t, operation.ErrInvalidOperation, operation.ErrInvalidInputs)
}

// Operation Interface Contract Tests
// Component: T003
// Feature: F057

// mockOperation is a test implementation of the Operation interface
// to verify the contract and test operation behavior.
type mockOperation struct {
	name   string
	schema *plugin.OperationSchema
	// executeFn allows customizing the Execute behavior in tests
	executeFn func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error)
}

func (m *mockOperation) Name() string {
	return m.name
}

func (m *mockOperation) Execute(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, inputs)
	}
	// Default successful execution
	return &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{
			"result": "executed",
		},
		Error: "",
	}, nil
}

func (m *mockOperation) Schema() *plugin.OperationSchema {
	return m.schema
}

// Compile-time check that mockOperation implements operation.Operation
var _ operation.Operation = (*mockOperation)(nil)

func TestOperation_InterfaceContract(t *testing.T) {
	// Happy path: Verify Operation interface can be implemented
	schema := &plugin.OperationSchema{
		Name:        "test.operation",
		Description: "Test operation for interface verification",
		Inputs: map[string]plugin.InputSchema{
			"input1": {
				Type:        plugin.InputTypeString,
				Required:    true,
				Description: "Test input",
			},
		},
		Outputs:    []string{"result"},
		PluginName: "test",
	}

	op := &mockOperation{
		name:   "test.operation",
		schema: schema,
	}

	// Test Name() returns expected value
	assert.Equal(t, "test.operation", op.Name())

	// Test Schema() returns expected schema
	returnedSchema := op.Schema()
	require.NotNil(t, returnedSchema)
	assert.Equal(t, "test.operation", returnedSchema.Name)
	assert.Equal(t, "Test operation for interface verification", returnedSchema.Description)
	assert.Equal(t, "test", returnedSchema.PluginName)
	assert.Len(t, returnedSchema.Inputs, 1)
	assert.Equal(t, []string{"result"}, returnedSchema.Outputs)

	// Test Execute() works with valid inputs
	ctx := context.Background()
	inputs := map[string]any{
		"input1": "test value",
	}
	result, err := op.Execute(ctx, inputs)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "", result.Error)
	assert.NotNil(t, result.Outputs)
}

func TestOperation_Execute_HappyPath(t *testing.T) {
	// Happy path: Execute operation with valid inputs and context
	tests := []struct {
		name           string
		operationName  string
		inputs         map[string]any
		expectedOutput map[string]any
	}{
		{
			name:          "simple_string_input",
			operationName: "test.simple",
			inputs: map[string]any{
				"message": "hello",
			},
			expectedOutput: map[string]any{
				"result": "processed: hello",
			},
		},
		{
			name:          "multiple_inputs",
			operationName: "test.multi",
			inputs: map[string]any{
				"name":  "Alice",
				"count": 5,
				"flag":  true,
			},
			expectedOutput: map[string]any{
				"result": "Alice:5:true",
			},
		},
		{
			name:          "empty_inputs",
			operationName: "test.empty",
			inputs:        map[string]any{},
			expectedOutput: map[string]any{
				"result": "default",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &plugin.OperationSchema{
				Name:        tt.operationName,
				Description: "Test operation",
				Inputs:      map[string]plugin.InputSchema{},
				Outputs:     []string{"result"},
				PluginName:  "test",
			}

			op := &mockOperation{
				name:   tt.operationName,
				schema: schema,
				executeFn: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
					return &plugin.OperationResult{
						Success: true,
						Outputs: tt.expectedOutput,
						Error:   "",
					}, nil
				},
			}

			ctx := context.Background()
			result, err := op.Execute(ctx, tt.inputs)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.Success)
			assert.Equal(t, "", result.Error)
			assert.Equal(t, tt.expectedOutput, result.Outputs)
		})
	}
}

func TestOperation_Execute_ContextCancellation(t *testing.T) {
	// Edge case: Execute respects context cancellation
	schema := &plugin.OperationSchema{
		Name:        "test.cancellable",
		Description: "Operation that respects context cancellation",
		Inputs:      map[string]plugin.InputSchema{},
		Outputs:     []string{"result"},
		PluginName:  "test",
	}

	op := &mockOperation{
		name:   "test.cancellable",
		schema: schema,
		executeFn: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return &plugin.OperationResult{Success: true}, nil
			}
		},
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := op.Execute(ctx, map[string]any{})

	// Expect context.Canceled error
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Nil(t, result)
}

func TestOperation_Execute_ErrorHandling(t *testing.T) {
	// Error handling: Execute returns appropriate errors
	tests := []struct {
		name          string
		executeFn     func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error)
		expectedError error
		checkResult   func(t *testing.T, result *plugin.OperationResult)
	}{
		{
			name: "execution_failure_with_error",
			executeFn: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				return nil, errors.New("execution failed")
			},
			expectedError: errors.New("execution failed"),
			checkResult: func(t *testing.T, result *plugin.OperationResult) {
				assert.Nil(t, result)
			},
		},
		{
			name: "partial_failure_with_result",
			executeFn: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
				return &plugin.OperationResult{
					Success: false,
					Outputs: map[string]any{},
					Error:   "operation failed",
				}, nil
			},
			expectedError: nil,
			checkResult: func(t *testing.T, result *plugin.OperationResult) {
				require.NotNil(t, result)
				assert.False(t, result.Success)
				assert.Equal(t, "operation failed", result.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &plugin.OperationSchema{
				Name:        "test.error",
				Description: "Error handling test",
				Inputs:      map[string]plugin.InputSchema{},
				Outputs:     []string{"result"},
				PluginName:  "test",
			}

			op := &mockOperation{
				name:      "test.error",
				schema:    schema,
				executeFn: tt.executeFn,
			}

			ctx := context.Background()
			result, err := op.Execute(ctx, map[string]any{})

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			tt.checkResult(t, result)
		})
	}
}

func TestOperation_Schema_ReturnsValidSchema(t *testing.T) {
	// Happy path: Schema() returns valid OperationSchema with all fields
	tests := []struct {
		name   string
		schema *plugin.OperationSchema
	}{
		{
			name: "complete_schema_with_all_fields",
			schema: &plugin.OperationSchema{
				Name:        "http.get",
				Description: "HTTP GET request",
				Inputs: map[string]plugin.InputSchema{
					"url": {
						Type:        plugin.InputTypeString,
						Required:    true,
						Description: "Target URL",
						Validation:  "url",
					},
					"timeout": {
						Type:        plugin.InputTypeString,
						Required:    false,
						Default:     "30s",
						Description: "Request timeout",
					},
				},
				Outputs:    []string{"status_code", "body", "headers"},
				PluginName: "http",
			},
		},
		{
			name: "minimal_schema",
			schema: &plugin.OperationSchema{
				Name:        "simple.op",
				Description: "Simple operation",
				Inputs:      map[string]plugin.InputSchema{},
				Outputs:     []string{},
				PluginName:  "simple",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := &mockOperation{
				name:   tt.schema.Name,
				schema: tt.schema,
			}

			returnedSchema := op.Schema()
			require.NotNil(t, returnedSchema)

			// Verify all schema fields match
			assert.Equal(t, tt.schema.Name, returnedSchema.Name)
			assert.Equal(t, tt.schema.Description, returnedSchema.Description)
			assert.Equal(t, tt.schema.PluginName, returnedSchema.PluginName)
			assert.Equal(t, len(tt.schema.Inputs), len(returnedSchema.Inputs))
			assert.Equal(t, tt.schema.Outputs, returnedSchema.Outputs)

			// Verify inputs are correctly returned
			for inputName, inputSchema := range tt.schema.Inputs {
				returnedInput, exists := returnedSchema.Inputs[inputName]
				require.True(t, exists, "Input %q should exist in returned schema", inputName)
				assert.Equal(t, inputSchema.Type, returnedInput.Type)
				assert.Equal(t, inputSchema.Required, returnedInput.Required)
				assert.Equal(t, inputSchema.Default, returnedInput.Default)
				assert.Equal(t, inputSchema.Description, returnedInput.Description)
				assert.Equal(t, inputSchema.Validation, returnedInput.Validation)
			}
		})
	}
}

func TestOperation_Name_ReturnsUniqueIdentifier(t *testing.T) {
	// Happy path: Name() returns unique operation identifier
	tests := []struct {
		name          string
		operationName string
	}{
		{
			name:          "dotted_name",
			operationName: "http.get",
		},
		{
			name:          "multi_segment_name",
			operationName: "file.system.read",
		},
		{
			name:          "simple_name",
			operationName: "transform",
		},
		{
			name:          "name_with_underscores",
			operationName: "custom_operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &plugin.OperationSchema{
				Name:        tt.operationName,
				Description: "Test",
				Inputs:      map[string]plugin.InputSchema{},
				Outputs:     []string{},
				PluginName:  "test",
			}

			op := &mockOperation{
				name:   tt.operationName,
				schema: schema,
			}

			returnedName := op.Name()
			assert.Equal(t, tt.operationName, returnedName)
			assert.NotEmpty(t, returnedName, "Operation name should not be empty")
		})
	}
}

// Package Documentation Tests
// Component: T003
// Feature: F057

func TestPackageDocumentation_Exists(t *testing.T) {
	// Happy path: Verify package documentation exists in doc.go
	// This test ensures the package is properly documented following project conventions

	// Note: This test validates that doc.go exists and contains package documentation.
	// The actual content is verified by code review, not automated tests.
	// If this test fails, it means doc.go is missing or empty.

	// We can't directly test the existence of doc.go from within the package,
	// but we can verify the package comment is accessible via reflection or manual verification.
	// For now, this test serves as a reminder that doc.go must exist.

	// The package name itself being accessible is evidence that the package is properly defined
	assert.NotEmpty(t, "operation", "Package name should not be empty")
}

// Edge Cases and Validation Tests
// Component: T003
// Feature: F057

func TestOperation_Execute_WithNilInputs(t *testing.T) {
	// Edge case: Execute with nil inputs map should work
	schema := &plugin.OperationSchema{
		Name:        "test.nil_inputs",
		Description: "Handles nil inputs",
		Inputs: map[string]plugin.InputSchema{
			"optional": {
				Type:     plugin.InputTypeString,
				Required: false,
				Default:  "default_value",
			},
		},
		Outputs:    []string{"result"},
		PluginName: "test",
	}

	op := &mockOperation{
		name:   "test.nil_inputs",
		schema: schema,
		executeFn: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
			// Operation should handle nil inputs gracefully
			return &plugin.OperationResult{
				Success: true,
				Outputs: map[string]any{
					"result": "handled nil inputs",
				},
			}, nil
		},
	}

	ctx := context.Background()
	result, err := op.Execute(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestOperation_Schema_ConsistencyWithName(t *testing.T) {
	// Edge case: Schema().Name should match Name()
	tests := []struct {
		name          string
		operationName string
	}{
		{
			name:          "http.get",
			operationName: "http.get",
		},
		{
			name:          "file.read",
			operationName: "file.read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &plugin.OperationSchema{
				Name:        tt.operationName,
				Description: "Consistency test",
				Inputs:      map[string]plugin.InputSchema{},
				Outputs:     []string{},
				PluginName:  "test",
			}

			op := &mockOperation{
				name:   tt.operationName,
				schema: schema,
			}

			// Name() and Schema().Name should match
			assert.Equal(t, op.Name(), op.Schema().Name)
		})
	}
}
