package operation_test

import (
	"context"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/operation"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: mockOperation is defined in operation_test.go and reused here

// TestOperationRegistry_Register tests successful registration
func TestOperationRegistry_Register(t *testing.T) {
	tests := []struct {
		name      string
		operation operation.Operation
	}{
		{
			name: "register simple operation",
			operation: &mockOperation{
				name: "test.operation",
				schema: &pluginmodel.OperationSchema{
					Name:        "test.operation",
					Description: "Test operation",
					Inputs:      map[string]pluginmodel.InputSchema{},
					Outputs:     []string{"result"},
				},
			},
		},
		{
			name: "register operation with complex schema",
			operation: &mockOperation{
				name: "http.get",
				schema: &pluginmodel.OperationSchema{
					Name:        "http.get",
					Description: "HTTP GET request",
					Inputs: map[string]pluginmodel.InputSchema{
						"url": {
							Type:        pluginmodel.InputTypeString,
							Required:    true,
							Description: "URL to fetch",
						},
						"timeout": {
							Type:        pluginmodel.InputTypeInteger,
							Required:    false,
							Default:     30,
							Description: "Timeout in seconds",
						},
					},
					Outputs: []string{"status_code", "body"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := operation.NewOperationRegistry()

			err := registry.Register(tt.operation)
			require.NoError(t, err, "Register should succeed")

			// Verify operation is retrievable
			op, found := registry.Get(tt.operation.Name())
			assert.True(t, found, "Operation should be found after registration")
			assert.Equal(t, tt.operation, op, "Retrieved operation should match registered operation")
		})
	}
}

// TestOperationRegistry_Register_Errors tests registration error scenarios
func TestOperationRegistry_Register_Errors(t *testing.T) {
	tests := []struct {
		name      string
		operation operation.Operation
		wantErr   error
	}{
		{
			name:      "nil operation",
			operation: nil,
			wantErr:   operation.ErrInvalidOperation,
		},
		{
			name: "empty name",
			operation: &mockOperation{
				name: "",
				schema: &pluginmodel.OperationSchema{
					Name: "",
				},
			},
			wantErr: operation.ErrInvalidOperation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := operation.NewOperationRegistry()

			err := registry.Register(tt.operation)
			assert.ErrorIs(t, err, tt.wantErr, "Register should return expected error")
		})
	}
}

// TestOperationRegistry_Register_Duplicate tests duplicate registration
func TestOperationRegistry_Register_Duplicate(t *testing.T) {
	registry := operation.NewOperationRegistry()

	op1 := &mockOperation{
		name: "test.operation",
		schema: &pluginmodel.OperationSchema{
			Name: "test.operation",
		},
	}

	// First registration should succeed
	err := registry.Register(op1)
	require.NoError(t, err, "First registration should succeed")

	// Second registration with same name should fail
	op2 := &mockOperation{
		name: "test.operation",
		schema: &pluginmodel.OperationSchema{
			Name: "test.operation",
		},
	}

	err = registry.Register(op2)
	assert.ErrorIs(t, err, operation.ErrOperationAlreadyRegistered, "Duplicate registration should fail")

	// Original operation should still be registered
	op, found := registry.Get("test.operation")
	assert.True(t, found, "Original operation should still be registered")
	assert.Equal(t, op1, op, "Original operation should be unchanged")
}

// TestOperationRegistry_Unregister tests successful unregistration
func TestOperationRegistry_Unregister(t *testing.T) {
	registry := operation.NewOperationRegistry()

	op := &mockOperation{
		name: "test.operation",
		schema: &pluginmodel.OperationSchema{
			Name: "test.operation",
		},
	}

	// Register operation
	err := registry.Register(op)
	require.NoError(t, err, "Register should succeed")

	// Unregister operation
	err = registry.Unregister("test.operation")
	assert.NoError(t, err, "Unregister should succeed")

	// Operation should no longer be found
	_, found := registry.Get("test.operation")
	assert.False(t, found, "Operation should not be found after unregistration")
}

// TestOperationRegistry_Unregister_NotFound tests unregistering non-existent operation
func TestOperationRegistry_Unregister_NotFound(t *testing.T) {
	registry := operation.NewOperationRegistry()

	err := registry.Unregister("nonexistent.operation")
	assert.ErrorIs(t, err, operation.ErrOperationNotFound, "Unregistering non-existent operation should fail")
}

// TestOperationRegistry_Get tests operation retrieval
func TestOperationRegistry_Get(t *testing.T) {
	tests := []struct {
		name          string
		registerOps   []operation.Operation
		lookupName    string
		expectFound   bool
		expectMatches int // index in registerOps, -1 if not found
	}{
		{
			name: "get existing operation",
			registerOps: []operation.Operation{
				&mockOperation{
					name: "test.operation",
					schema: &pluginmodel.OperationSchema{
						Name: "test.operation",
					},
				},
			},
			lookupName:    "test.operation",
			expectFound:   true,
			expectMatches: 0,
		},
		{
			name: "get non-existent operation",
			registerOps: []operation.Operation{
				&mockOperation{
					name: "test.operation",
					schema: &pluginmodel.OperationSchema{
						Name: "test.operation",
					},
				},
			},
			lookupName:    "nonexistent.operation",
			expectFound:   false,
			expectMatches: -1,
		},
		{
			name:          "get from empty registry",
			registerOps:   []operation.Operation{},
			lookupName:    "any.operation",
			expectFound:   false,
			expectMatches: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := operation.NewOperationRegistry()

			// Register operations
			for _, op := range tt.registerOps {
				err := registry.Register(op)
				require.NoError(t, err, "Register should succeed")
			}

			// Lookup operation
			op, found := registry.Get(tt.lookupName)

			assert.Equal(t, tt.expectFound, found, "Found status should match")
			if tt.expectFound && tt.expectMatches >= 0 {
				assert.Equal(t, tt.registerOps[tt.expectMatches], op, "Retrieved operation should match")
			} else {
				assert.Nil(t, op, "Operation should be nil when not found")
			}
		})
	}
}

// TestOperationRegistry_List tests listing all operations
func TestOperationRegistry_List(t *testing.T) {
	tests := []struct {
		name        string
		registerOps []operation.Operation
		expectCount int
	}{
		{
			name:        "empty registry",
			registerOps: []operation.Operation{},
			expectCount: 0,
		},
		{
			name: "single operation",
			registerOps: []operation.Operation{
				&mockOperation{
					name: "test.operation",
					schema: &pluginmodel.OperationSchema{
						Name: "test.operation",
					},
				},
			},
			expectCount: 1,
		},
		{
			name: "multiple operations",
			registerOps: []operation.Operation{
				&mockOperation{
					name: "http.get",
					schema: &pluginmodel.OperationSchema{
						Name: "http.get",
					},
				},
				&mockOperation{
					name: "file.read",
					schema: &pluginmodel.OperationSchema{
						Name: "file.read",
					},
				},
				&mockOperation{
					name: "transform.jq",
					schema: &pluginmodel.OperationSchema{
						Name: "transform.jq",
					},
				},
			},
			expectCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := operation.NewOperationRegistry()

			// Register operations
			for _, op := range tt.registerOps {
				err := registry.Register(op)
				require.NoError(t, err, "Register should succeed")
			}

			// List operations
			operations := registry.List()

			assert.Len(t, operations, tt.expectCount, "List should return expected count")

			// Verify all registered operations are in the list
			for _, expectedOp := range tt.registerOps {
				found := false
				for _, listedOp := range operations {
					if listedOp.Name() == expectedOp.Name() {
						found = true
						assert.Equal(t, expectedOp, listedOp, "Listed operation should match registered operation")
						break
					}
				}
				assert.True(t, found, "Registered operation %s should be in list", expectedOp.Name())
			}
		})
	}
}

// TestOperationRegistry_GetOperation tests ports.OperationProvider.GetOperation implementation
func TestOperationRegistry_GetOperation(t *testing.T) {
	registry := operation.NewOperationRegistry()

	schema := &pluginmodel.OperationSchema{
		Name:        "test.operation",
		Description: "Test operation",
		Inputs: map[string]pluginmodel.InputSchema{
			"url": {
				Type:     pluginmodel.InputTypeString,
				Required: true,
			},
		},
		Outputs: []string{"result"},
	}

	op := &mockOperation{
		name:   "test.operation",
		schema: schema,
	}

	// Register operation
	err := registry.Register(op)
	require.NoError(t, err, "Register should succeed")

	// GetOperation should return the schema
	retrievedSchema, found := registry.GetOperation("test.operation")
	assert.True(t, found, "Operation should be found")
	assert.Equal(t, schema, retrievedSchema, "Retrieved schema should match")

	// GetOperation for non-existent operation
	_, found = registry.GetOperation("nonexistent.operation")
	assert.False(t, found, "Non-existent operation should not be found")
}

// TestOperationRegistry_ListOperations tests ports.OperationProvider.ListOperations implementation
func TestOperationRegistry_ListOperations(t *testing.T) {
	registry := operation.NewOperationRegistry()

	schemas := []*pluginmodel.OperationSchema{
		{
			Name:        "http.get",
			Description: "HTTP GET request",
			Inputs:      map[string]pluginmodel.InputSchema{},
			Outputs:     []string{"status_code"},
		},
		{
			Name:        "file.read",
			Description: "Read file",
			Inputs:      map[string]pluginmodel.InputSchema{},
			Outputs:     []string{"content"},
		},
	}

	// Register operations
	for _, schema := range schemas {
		op := &mockOperation{
			name:   schema.Name,
			schema: schema,
		}
		err := registry.Register(op)
		require.NoError(t, err, "Register should succeed")
	}

	// ListOperations should return all schemas
	listedSchemas := registry.ListOperations()
	assert.Len(t, listedSchemas, len(schemas), "ListOperations should return all schemas")

	// Verify all schemas are present
	for _, expectedSchema := range schemas {
		found := false
		for _, listedSchema := range listedSchemas {
			if listedSchema.Name == expectedSchema.Name {
				found = true
				assert.Equal(t, expectedSchema, listedSchema, "Listed schema should match")
				break
			}
		}
		assert.True(t, found, "Schema %s should be in list", expectedSchema.Name)
	}
}

// TestOperationRegistry_Execute tests ports.OperationProvider.Execute implementation
func TestOperationRegistry_Execute(t *testing.T) {
	tests := []struct {
		name          string
		operation     operation.Operation
		executeName   string
		executeInputs map[string]any
		wantSuccess   bool
		wantErr       error
		wantOutputs   map[string]any
	}{
		{
			name: "execute registered operation",
			operation: &mockOperation{
				name: "test.operation",
				schema: &pluginmodel.OperationSchema{
					Name: "test.operation",
					Inputs: map[string]pluginmodel.InputSchema{
						"url": {
							Type:     pluginmodel.InputTypeString,
							Required: true,
						},
					},
					Outputs: []string{"result"},
				},
				executeFn: func(ctx context.Context, inputs map[string]any) (*pluginmodel.OperationResult, error) {
					return &pluginmodel.OperationResult{
						Success: true,
						Outputs: map[string]any{
							"result": "success",
						},
					}, nil
				},
			},
			executeName: "test.operation",
			executeInputs: map[string]any{
				"url": "https://example.com",
			},
			wantSuccess: true,
			wantErr:     nil,
			wantOutputs: map[string]any{
				"result": "success",
			},
		},
		{
			name: "execute non-existent operation",
			operation: &mockOperation{
				name: "test.operation",
				schema: &pluginmodel.OperationSchema{
					Name: "test.operation",
				},
			},
			executeName:   "nonexistent.operation",
			executeInputs: map[string]any{},
			wantSuccess:   false,
			wantErr:       operation.ErrOperationNotFound,
		},
		{
			name: "execute with validation failure - missing required field",
			operation: &mockOperation{
				name: "test.operation",
				schema: &pluginmodel.OperationSchema{
					Name: "test.operation",
					Inputs: map[string]pluginmodel.InputSchema{
						"url": {
							Type:     pluginmodel.InputTypeString,
							Required: true,
						},
					},
				},
			},
			executeName:   "test.operation",
			executeInputs: map[string]any{}, // missing required "url"
			wantSuccess:   false,
			wantErr:       operation.ErrInvalidInputs,
		},
		{
			name: "execute with validation failure - type mismatch",
			operation: &mockOperation{
				name: "test.operation",
				schema: &pluginmodel.OperationSchema{
					Name: "test.operation",
					Inputs: map[string]pluginmodel.InputSchema{
						"count": {
							Type:     pluginmodel.InputTypeInteger,
							Required: true,
						},
					},
				},
			},
			executeName: "test.operation",
			executeInputs: map[string]any{
				"count": "not-an-integer",
			},
			wantSuccess: false,
			wantErr:     operation.ErrInvalidInputs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := operation.NewOperationRegistry()
			ctx := context.Background()

			// Register operation
			err := registry.Register(tt.operation)
			require.NoError(t, err, "Register should succeed")

			// Execute operation
			result, err := registry.Execute(ctx, tt.executeName, tt.executeInputs)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr, "Execute should return expected error")
			} else {
				assert.NoError(t, err, "Execute should succeed")
				assert.NotNil(t, result, "Result should not be nil")
				assert.Equal(t, tt.wantSuccess, result.Success, "Result success status should match")
				if tt.wantOutputs != nil {
					assert.Equal(t, tt.wantOutputs, result.Outputs, "Result outputs should match")
				}
			}
		})
	}
}

// TestOperationRegistry_Execute_ContextCancellation tests context cancellation
func TestOperationRegistry_Execute_ContextCancellation(t *testing.T) {
	registry := operation.NewOperationRegistry()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel context immediately

	op := &mockOperation{
		name: "test.operation",
		schema: &pluginmodel.OperationSchema{
			Name:   "test.operation",
			Inputs: map[string]pluginmodel.InputSchema{},
		},
		executeFn: func(ctx context.Context, inputs map[string]any) (*pluginmodel.OperationResult, error) {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return &pluginmodel.OperationResult{Success: true}, nil
			}
		},
	}

	err := registry.Register(op)
	require.NoError(t, err, "Register should succeed")

	// Execute with cancelled context
	result, err := registry.Execute(ctx, "test.operation", map[string]any{})

	// Should propagate context error
	assert.Error(t, err, "Execute should fail with cancelled context")
	assert.ErrorIs(t, err, context.Canceled, "Error should be context.Canceled")
	assert.Nil(t, result, "Result should be nil on context cancellation")
}

// TestOperationRegistry_Execute_AppliesDefaults tests default value application
func TestOperationRegistry_Execute_AppliesDefaults(t *testing.T) {
	registry := operation.NewOperationRegistry()
	ctx := context.Background()

	var receivedInputs map[string]any

	op := &mockOperation{
		name: "test.operation",
		schema: &pluginmodel.OperationSchema{
			Name: "test.operation",
			Inputs: map[string]pluginmodel.InputSchema{
				"url": {
					Type:     pluginmodel.InputTypeString,
					Required: true,
				},
				"timeout": {
					Type:     pluginmodel.InputTypeInteger,
					Required: false,
					Default:  30,
				},
			},
		},
		executeFn: func(ctx context.Context, inputs map[string]any) (*pluginmodel.OperationResult, error) {
			receivedInputs = inputs
			return &pluginmodel.OperationResult{Success: true}, nil
		},
	}

	err := registry.Register(op)
	require.NoError(t, err, "Register should succeed")

	// Execute with only required field
	_, err = registry.Execute(ctx, "test.operation", map[string]any{
		"url": "https://example.com",
	})

	assert.NoError(t, err, "Execute should succeed")
	assert.Equal(t, "https://example.com", receivedInputs["url"], "URL should be passed through")
	assert.Equal(t, 30, receivedInputs["timeout"], "Default timeout should be applied")
}

// TestOperationRegistry_ConcurrentAccess tests thread safety
func TestOperationRegistry_ConcurrentAccess(t *testing.T) {
	registry := operation.NewOperationRegistry()
	ctx := context.Background()

	// Register initial operations
	registerInitialOps(t, registry)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent reads
	runConcurrentReads(t, &wg, registry, errors)

	// Concurrent writes
	runConcurrentWrites(t, &wg, registry, errors)

	// Concurrent executions
	runConcurrentExecutions(t, registry, ctx, &wg, errors)

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

// Helper functions for TestOperationRegistry_ConcurrentAccess

func registerInitialOps(t *testing.T, registry *operation.OperationRegistry) {
	t.Helper()
	for i := 0; i < 5; i++ {
		op := &mockOperation{
			name: "initial.op" + string(rune('0'+i)),
			schema: &pluginmodel.OperationSchema{
				Name: "initial.op" + string(rune('0'+i)),
			},
		}
		err := registry.Register(op)
		require.NoError(t, err, "Initial registration should succeed")
	}
}

func runConcurrentReads(t *testing.T, wg *sync.WaitGroup, registry *operation.OperationRegistry, errors chan<- error) {
	t.Helper()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				performReads(registry, errors)
			}
		}(i)
	}
}

func performReads(registry *operation.OperationRegistry, errors chan<- error) {
	// Get operation
	_, found := registry.Get("initial.op0")
	if !found {
		errors <- assert.AnError
	}

	// List operations
	ops := registry.List()
	if len(ops) < 5 {
		errors <- assert.AnError
	}

	// GetOperation
	_, found = registry.GetOperation("initial.op1")
	if !found {
		errors <- assert.AnError
	}

	// ListOperations
	schemas := registry.ListOperations()
	if len(schemas) < 5 {
		errors <- assert.AnError
	}
}

func runConcurrentWrites(t *testing.T, wg *sync.WaitGroup, registry *operation.OperationRegistry, errors chan<- error) {
	t.Helper()
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				performWrites(id, j, registry, errors)
			}
		}(i)
	}
}

func performWrites(id, j int, registry *operation.OperationRegistry, errors chan<- error) {
	opName := "concurrent.op" + string(rune('0'+id)) + string(rune('0'+j)) //nolint:gosec // G115: controlled test input, id/j are small loop indices
	op := &mockOperation{
		name: opName,
		schema: &pluginmodel.OperationSchema{
			Name: opName,
		},
	}

	// Register
	if err := registry.Register(op); err != nil {
		errors <- err
	}

	// Unregister
	if err := registry.Unregister(opName); err != nil {
		errors <- err
	}
}

//nolint:revive // Test helper - t *testing.T must be first parameter
func runConcurrentExecutions(t *testing.T, registry *operation.OperationRegistry, ctx context.Context, wg *sync.WaitGroup, errors chan<- error) {
	t.Helper()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, err := registry.Execute(ctx, "initial.op0", map[string]any{})
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}
}

// TestOperationRegistry_Lifecycle tests complete register-use-unregister lifecycle
func TestOperationRegistry_Lifecycle(t *testing.T) {
	registry := operation.NewOperationRegistry()
	ctx := context.Background()

	op := &mockOperation{
		name: "lifecycle.operation",
		schema: &pluginmodel.OperationSchema{
			Name: "lifecycle.operation",
			Inputs: map[string]pluginmodel.InputSchema{
				"value": {
					Type:     pluginmodel.InputTypeString,
					Required: true,
				},
			},
		},
		executeFn: func(ctx context.Context, inputs map[string]any) (*pluginmodel.OperationResult, error) {
			return &pluginmodel.OperationResult{
				Success: true,
				Outputs: map[string]any{
					"result": inputs["value"],
				},
			}, nil
		},
	}

	// Initial state - operation not found
	_, found := registry.Get("lifecycle.operation")
	assert.False(t, found, "Operation should not be found initially")

	// Register operation
	err := registry.Register(op)
	require.NoError(t, err, "Registration should succeed")

	// Verify registration
	retrievedOp, found := registry.Get("lifecycle.operation")
	assert.True(t, found, "Operation should be found after registration")
	assert.Equal(t, op, retrievedOp, "Retrieved operation should match")

	// Execute operation
	result, err := registry.Execute(ctx, "lifecycle.operation", map[string]any{
		"value": "test-value",
	})
	require.NoError(t, err, "Execution should succeed")
	assert.True(t, result.Success, "Result should be successful")
	assert.Equal(t, "test-value", result.Outputs["result"], "Output should match input")

	// Unregister operation
	err = registry.Unregister("lifecycle.operation")
	require.NoError(t, err, "Unregistration should succeed")

	// Verify unregistration
	_, found = registry.Get("lifecycle.operation")
	assert.False(t, found, "Operation should not be found after unregistration")

	// Execution should fail after unregistration
	_, err = registry.Execute(ctx, "lifecycle.operation", map[string]any{
		"value": "test-value",
	})
	assert.ErrorIs(t, err, operation.ErrOperationNotFound, "Execution should fail after unregistration")
}

// TestOperationRegistry_EmptyRegistryOperations tests all operations on empty registry
func TestOperationRegistry_EmptyRegistryOperations(t *testing.T) {
	registry := operation.NewOperationRegistry()
	ctx := context.Background()

	// Get on empty registry
	op, found := registry.Get("any.operation")
	assert.False(t, found, "Get should return false on empty registry")
	assert.Nil(t, op, "Get should return nil operation on empty registry")

	// List on empty registry
	operations := registry.List()
	assert.NotNil(t, operations, "List should not return nil on empty registry")
	assert.Len(t, operations, 0, "List should return empty slice on empty registry")

	// GetOperation on empty registry
	schema, found := registry.GetOperation("any.operation")
	assert.False(t, found, "GetOperation should return false on empty registry")
	assert.Nil(t, schema, "GetOperation should return nil schema on empty registry")

	// ListOperations on empty registry
	schemas := registry.ListOperations()
	assert.NotNil(t, schemas, "ListOperations should not return nil on empty registry")
	assert.Len(t, schemas, 0, "ListOperations should return empty slice on empty registry")

	// Execute on empty registry
	result, err := registry.Execute(ctx, "any.operation", map[string]any{})
	assert.ErrorIs(t, err, operation.ErrOperationNotFound, "Execute should fail on empty registry")
	assert.Nil(t, result, "Execute should return nil result on empty registry")

	// Unregister on empty registry
	err = registry.Unregister("any.operation")
	assert.ErrorIs(t, err, operation.ErrOperationNotFound, "Unregister should fail on empty registry")
}
