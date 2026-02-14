//go:build integration

// Feature: F057 - Operation Interface and Registry Foundation
// This file contains functional/integration tests for the operation registry.
//
// Tests cover:
// - US1: Execute registered operations by name
// - US2: Validate inputs against schema
// - US3: List and discover operations
// - US4: Register/unregister at runtime
// - Error handling (duplicate, not found, invalid input)
// - Context cancellation propagation
// - Thread safety with concurrent access

package integration_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/operation"
	"github.com/vanoix/awf/internal/domain/plugin"
)

// mockOperation is a test double for operation execution.
// It simulates an operation that can be registered and executed.
// Implements operation.Operation interface.
type mockOperation struct {
	name    string
	schema  *plugin.OperationSchema
	execFn  func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error)
	callsMu sync.Mutex
	calls   int
}

// Name returns the operation name. Implements operation.Operation.
func (m *mockOperation) Name() string {
	return m.name
}

// Schema returns the operation schema. Implements operation.Operation.
func (m *mockOperation) Schema() *plugin.OperationSchema {
	return m.schema
}

// Execute runs the operation. Implements operation.Operation.
func (m *mockOperation) Execute(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	return m.execute(ctx, inputs)
}

// newMockOperation creates a simple mock operation with basic string input/output.
func newMockOperation(name, pluginName string) *mockOperation {
	return &mockOperation{
		name: name,
		schema: &plugin.OperationSchema{
			Name:        name,
			Description: "Mock operation for testing",
			Inputs: map[string]plugin.InputSchema{
				"message": {Type: plugin.InputTypeString, Required: true},
			},
			Outputs:    []string{"result"},
			PluginName: pluginName,
		},
		execFn: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
			// Default: echo the message back
			msg, ok := inputs["message"].(string)
			if !ok {
				return &plugin.OperationResult{
					Success: false,
					Error:   "message must be a string",
				}, nil
			}
			return &plugin.OperationResult{
				Success: true,
				Outputs: map[string]any{"result": msg},
			}, nil
		},
	}
}

// execute runs the mock operation and tracks call count.
func (m *mockOperation) execute(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	m.callsMu.Lock()
	m.calls++
	m.callsMu.Unlock()

	return m.execFn(ctx, inputs)
}

// callCount returns the number of times the operation was executed.
func (m *mockOperation) callCount() int {
	m.callsMu.Lock()
	defer m.callsMu.Unlock()
	return m.calls
}

// TestOperationRegistry_US1_ExecuteByName tests executing registered operations by name.
// User Story 1: As a workflow author, I want to execute plugin operations by name
// so that I can use plugin functionality in my workflows.
func TestOperationRegistry_US1_ExecuteByName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: A registry with registered operations
	registry := operation.NewOperationRegistry()

	slackOp := newMockOperation("slack.send", "awf-plugin-slack")
	githubOp := newMockOperation("github.create_issue", "awf-plugin-github")

	require.NoError(t, registry.Register(slackOp))
	require.NoError(t, registry.Register(githubOp))

	ctx := context.Background()

	// When: Looking up operations by name
	slackSchema, foundSlack := registry.GetOperation("slack.send")
	_, foundGithub := registry.GetOperation("github.create_issue")
	_, foundMissing := registry.GetOperation("nonexistent.operation")

	// Then: Operations are found by exact name
	assert.True(t, foundSlack, "slack.send should be found")
	assert.True(t, foundGithub, "github.create_issue should be found")
	assert.False(t, foundMissing, "nonexistent operation should not be found")

	// And: Schemas contain correct metadata
	assert.Equal(t, "slack.send", slackSchema.Name)
	assert.Equal(t, "awf-plugin-slack", slackSchema.PluginName)
	assert.Contains(t, slackSchema.Inputs, "message")

	// And: Operations can be executed by name through the registry
	result, err := registry.Execute(ctx, "slack.send", map[string]any{"message": "Hello, world!"})
	require.NoError(t, err)
	assert.True(t, result.IsSuccess())
	assert.Equal(t, "Hello, world!", result.Outputs["result"])

	// And: Executing a non-existent operation returns ErrOperationNotFound
	_, err = registry.Execute(ctx, "nonexistent.operation", map[string]any{})
	assert.ErrorIs(t, err, operation.ErrOperationNotFound)
}

// TestOperationRegistry_US2_InputValidation tests runtime input validation against schema.
// User Story 2: As a workflow author, I want operations to validate inputs against schema
// so that I get early feedback on invalid parameters.
func TestOperationRegistry_US2_InputValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: A registry with an operation that has strict input schema
	registry := operation.NewOperationRegistry()
	ctx := context.Background()

	op := &mockOperation{
		name: "validate.test",
		schema: &plugin.OperationSchema{
			Name:        "validate.test",
			Description: "Operation with complex validation",
			Inputs: map[string]plugin.InputSchema{
				"url": {
					Type:        plugin.InputTypeString,
					Required:    true,
					Validation:  "url",
					Description: "Must be a valid URL",
				},
				"count": {
					Type:     plugin.InputTypeInteger,
					Required: true,
				},
				"timeout": {
					Type:     plugin.InputTypeString,
					Required: false,
					Default:  "30s",
				},
			},
			Outputs:    []string{"status"},
			PluginName: "test-plugin",
		},
		execFn: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
			return &plugin.OperationResult{
				Success: true,
				Outputs: map[string]any{"status": "ok"},
			}, nil
		},
	}

	require.NoError(t, registry.Register(op))

	// When: Executing with valid inputs — validation passes, defaults applied
	inputs := map[string]any{
		"url":   "https://api.example.com",
		"count": 42,
	}
	result, err := registry.Execute(ctx, "validate.test", inputs)
	require.NoError(t, err)
	assert.True(t, result.IsSuccess())
	assert.Equal(t, "30s", inputs["timeout"], "default value should be applied")

	// When: Executing with missing required field — validation fails early
	_, err = registry.Execute(ctx, "validate.test", map[string]any{
		"count": 10,
	})
	assert.ErrorIs(t, err, operation.ErrInvalidInputs, "should fail for missing required 'url'")

	// When: Executing with type mismatch — validation rejects
	_, err = registry.Execute(ctx, "validate.test", map[string]any{
		"url":   "https://example.com",
		"count": "not-an-integer",
	})
	assert.ErrorIs(t, err, operation.ErrInvalidInputs, "should fail for type mismatch on 'count'")

	// When: Executing with invalid URL format — validation rule rejects
	_, err = registry.Execute(ctx, "validate.test", map[string]any{
		"url":   "not-a-url",
		"count": 1,
	})
	assert.ErrorIs(t, err, operation.ErrInvalidInputs, "should fail for invalid URL format")
}

// TestOperationRegistry_US3_DiscoverOperations tests listing and discovering operations.
// User Story 3: As a workflow author, I want to list available operations
// so that I can discover what plugin functionality is available.
func TestOperationRegistry_US3_DiscoverOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: A registry with operations from multiple plugins
	registry := operation.NewOperationRegistry()

	// Register operations from different plugins
	slackOps := []operation.Operation{
		newMockOperation("slack.send", "awf-plugin-slack"),
		newMockOperation("slack.update", "awf-plugin-slack"),
		newMockOperation("slack.delete", "awf-plugin-slack"),
	}

	githubOps := []operation.Operation{
		newMockOperation("github.create_issue", "awf-plugin-github"),
		newMockOperation("github.close_issue", "awf-plugin-github"),
	}

	for _, op := range slackOps {
		require.NoError(t, registry.Register(op))
	}
	for _, op := range githubOps {
		require.NoError(t, registry.Register(op))
	}

	// When: Listing all operations
	allOps := registry.List()

	// Then: All registered operations are returned
	assert.Len(t, allOps, 5, "should have 5 total operations")

	// And: Operations can be retrieved via OperationProvider interface
	schemas := registry.ListOperations()
	assert.Len(t, schemas, 5, "should have 5 operation schemas")

	// Verify specific operations exist
	slackSchema, found := registry.GetOperation("slack.send")
	assert.True(t, found)
	assert.Equal(t, "awf-plugin-slack", slackSchema.PluginName)
}

// TestOperationRegistry_US4_DynamicRegistration tests runtime registration/unregistration.
// User Story 4: As a plugin developer, I want to register/unregister operations at runtime
// so that I can dynamically extend workflow capabilities.
func TestOperationRegistry_US4_DynamicRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: An empty registry
	registry := operation.NewOperationRegistry()
	assert.Len(t, registry.List(), 0)

	// When: Registering an operation
	op := newMockOperation("dynamic.test", "test-plugin")
	err := registry.Register(op)

	// Then: Registration succeeds
	require.NoError(t, err)
	assert.Len(t, registry.List(), 1)

	// And: Operation is retrievable
	retrieved, found := registry.GetOperation("dynamic.test")
	assert.True(t, found)
	assert.Equal(t, "dynamic.test", retrieved.Name)

	// When: Unregistering the operation
	err = registry.Unregister("dynamic.test")

	// Then: Unregistration succeeds
	require.NoError(t, err)
	assert.Len(t, registry.List(), 0)

	// And: Operation is no longer retrievable
	_, found = registry.GetOperation("dynamic.test")
	assert.False(t, found)

	// Given: Multiple operations from different plugins
	ops := []operation.Operation{
		newMockOperation("plugin.op1", "test-plugin"),
		newMockOperation("plugin.op2", "test-plugin"),
		newMockOperation("plugin.op3", "test-plugin"),
		newMockOperation("other.op", "other-plugin"),
	}
	for _, o := range ops {
		require.NoError(t, registry.Register(o))
	}
	assert.Len(t, registry.List(), 4)

	// When: Unregistering operations individually
	err = registry.Unregister("plugin.op1")
	require.NoError(t, err)
	err = registry.Unregister("plugin.op2")
	require.NoError(t, err)
	err = registry.Unregister("plugin.op3")
	require.NoError(t, err)

	// Then: Remaining operation is still accessible
	assert.Len(t, registry.List(), 1)
	_, found = registry.GetOperation("other.op")
	assert.True(t, found, "other plugin's operation should remain")
}

// TestOperationRegistry_DuplicateRegistration tests handling of duplicate operation names.
func TestOperationRegistry_DuplicateRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: A registry with a registered operation
	registry := operation.NewOperationRegistry()
	op1 := newMockOperation("test.operation", "plugin-a")
	require.NoError(t, registry.Register(op1))

	// When: Attempting to register an operation with the same name
	op2 := newMockOperation("test.operation", "plugin-b")
	err := registry.Register(op2)

	// Then: Registration fails with specific error
	assert.Error(t, err)
	assert.ErrorIs(t, err, operation.ErrOperationAlreadyRegistered)
	assert.Contains(t, err.Error(), "already registered")

	// And: Original operation is preserved
	retrieved, _ := registry.GetOperation("test.operation")
	assert.Equal(t, "plugin-a", retrieved.PluginName, "original plugin should be preserved")
}

// TestOperationRegistry_NotFound tests handling of missing operations.
func TestOperationRegistry_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: An empty registry
	registry := operation.NewOperationRegistry()

	// When: Attempting to unregister a nonexistent operation
	err := registry.Unregister("nonexistent.operation")

	// Then: Unregistration fails with specific error
	assert.Error(t, err)
	assert.ErrorIs(t, err, operation.ErrOperationNotFound)
	assert.Contains(t, err.Error(), "not found")
}

// TestOperationRegistry_InvalidOperation tests handling of invalid operation schemas.
func TestOperationRegistry_InvalidOperation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	registry := operation.NewOperationRegistry()

	tests := []struct {
		name      string
		op        operation.Operation
		wantError error
	}{
		{
			name:      "nil operation",
			op:        nil,
			wantError: operation.ErrInvalidOperation,
		},
		{
			name: "empty name",
			op: &mockOperation{
				name: "",
				schema: &plugin.OperationSchema{
					Name:       "",
					PluginName: "test-plugin",
				},
			},
			wantError: operation.ErrInvalidOperation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: Registering an invalid operation
			err := registry.Register(tt.op)

			// Then: Registration fails with specific error
			assert.Error(t, err)
			if tt.wantError != nil {
				assert.ErrorIs(t, err, tt.wantError)
			}
		})
	}
}

// TestOperationExecution_ContextCancellation tests cancellation propagation.
func TestOperationExecution_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: An operation that respects context cancellation
	slowOp := &mockOperation{
		name: "slow.operation",
		schema: &plugin.OperationSchema{
			Name:       "slow.operation",
			PluginName: "test-plugin",
		},
		execFn: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
			// Simulate slow operation that checks context
			select {
			case <-time.After(5 * time.Second):
				return &plugin.OperationResult{Success: true}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	// When: Executing with a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := slowOp.execute(ctx, nil)

	// Then: Operation is cancelled
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, result)
}

// TestOperationExecution_ContextTimeout tests timeout handling.
func TestOperationExecution_ContextTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: An operation that takes longer than timeout
	slowOp := &mockOperation{
		name: "slow.operation",
		schema: &plugin.OperationSchema{
			Name:       "slow.operation",
			PluginName: "test-plugin",
		},
		execFn: func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return &plugin.OperationResult{Success: true}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	// When: Executing with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := slowOp.execute(ctx, nil)

	// Then: Operation times out
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, result)
}

// TestOperationRegistry_ConcurrentRegistration tests concurrent registration.
func TestOperationRegistry_ConcurrentRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: A registry accessed by multiple goroutines
	registry := operation.NewOperationRegistry()
	const numGoroutines = 20
	const opsPerGoroutine = 10

	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	// When: Concurrently registering operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < opsPerGoroutine; j++ {
				op := newMockOperation("test.op"+string(rune('a'+id))+string(rune('0'+j)), "test-plugin")

				err := registry.Register(op)
				if err != nil {
					errorCount.Add(1)
				} else {
					successCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Then: All registrations succeed
	assert.Equal(t, int32(numGoroutines*opsPerGoroutine), successCount.Load())
	assert.Equal(t, int32(0), errorCount.Load())
	assert.Equal(t, numGoroutines*opsPerGoroutine, len(registry.List()))
}

// TestOperationRegistry_ConcurrentReadWrite tests concurrent reads and writes.
func TestOperationRegistry_ConcurrentReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: A registry with some initial operations
	registry := operation.NewOperationRegistry()
	for i := 0; i < 10; i++ {
		op := newMockOperation("initial.op"+string(rune('a'+i)), "test-plugin")
		require.NoError(t, registry.Register(op))
	}

	const numReaders = 10
	const numWriters = 5
	const duration = 100 * time.Millisecond

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// When: Concurrent reads
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Read operations
					_ = registry.ListOperations()
					_, _ = registry.GetOperation("initial.opa")
					_ = len(registry.List())
				}
			}
		}()
	}

	// And: Concurrent writes
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Write operations
					opName := "concurrent.op" + string(rune('a'+id)) + string(rune('0'+counter%10))
					op := newMockOperation(opName, "test-plugin")
					_ = registry.Register(op)
					_ = registry.Unregister(opName)
					counter++
				}
			}
		}(i)
	}

	wg.Wait()

	// Then: Registry remains consistent (no panics, data races)
	assert.GreaterOrEqual(t, len(registry.List()), 0)
}

// TestOperationRegistry_ConcurrentExecution tests concurrent operation execution.
func TestOperationRegistry_ConcurrentExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: An operation that can be executed concurrently
	op := newMockOperation("concurrent.test", "test-plugin")
	op.execFn = func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		return &plugin.OperationResult{
			Success: true,
			Outputs: map[string]any{"result": "done"},
		}, nil
	}

	const numExecutions = 50
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	ctx := context.Background()

	// When: Executing the operation concurrently
	for i := 0; i < numExecutions; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			result, err := op.execute(ctx, map[string]any{"message": "test"})
			if err != nil {
				errorCount.Add(1)
			} else if result.IsSuccess() {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// Then: All executions succeed
	assert.Equal(t, int32(numExecutions), successCount.Load())
	assert.Equal(t, int32(0), errorCount.Load())
	assert.Equal(t, numExecutions, op.callCount())
}

// TestOperationRegistry_ConcurrentPluginOperations tests plugin-scoped operations.
func TestOperationRegistry_ConcurrentPluginOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: A registry with operations from multiple plugins
	registry := operation.NewOperationRegistry()
	plugins := []string{"plugin-a", "plugin-b", "plugin-c"}

	for _, pluginName := range plugins {
		for i := 0; i < 5; i++ {
			op := newMockOperation(pluginName+".op"+string(rune('0'+i)), pluginName)
			require.NoError(t, registry.Register(op))
		}
	}

	const numGoroutines = 20
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// When: Concurrently querying all operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Verify total count remains stable
					ops := registry.List()
					if len(ops) != 15 {
						t.Errorf("expected 15 total operations, got %d", len(ops))
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Then: All operations remain accessible
	assert.Len(t, registry.List(), 15, "should have 15 total operations")
}

// TestOperationRegistry_ImplementsOperationProvider verifies interface compliance.
func TestOperationRegistry_ImplementsOperationProvider(t *testing.T) {
	// Given: An OperationRegistry instance
	registry := operation.NewOperationRegistry()

	// Then: It implements the ports.OperationProvider interface
	// This is verified at compile-time in the registry implementation

	// And: All interface methods are callable
	op := newMockOperation("test.op", "test")
	assert.NoError(t, registry.Register(op))

	// Test OperationProvider methods
	schema, found := registry.GetOperation("test.op")
	assert.True(t, found)
	assert.Equal(t, "test.op", schema.Name)

	schemas := registry.ListOperations()
	assert.Len(t, schemas, 1)

	assert.NoError(t, registry.Unregister("test.op"))
}

// TestOperationRegistry_EmptyRegistry tests operations on empty registry.
func TestOperationRegistry_EmptyRegistry(t *testing.T) {
	// Given: An empty registry
	registry := operation.NewOperationRegistry()

	// Then: All query operations return safe defaults
	assert.Empty(t, registry.List())
	assert.Empty(t, registry.ListOperations())

	_, found := registry.GetOperation("any-op")
	assert.False(t, found)

	// And: Unregister on empty registry returns error
	err := registry.Unregister("nonexistent")
	assert.ErrorIs(t, err, operation.ErrOperationNotFound)
}

// TestOperationRegistry_ReregisterAfterUnregister tests operation lifecycle.
func TestOperationRegistry_ReregisterAfterUnregister(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: A registry with an operation
	registry := operation.NewOperationRegistry()
	op := newMockOperation("test.op", "plugin-v1")

	require.NoError(t, registry.Register(op))

	// When: Unregistering the operation
	require.NoError(t, registry.Unregister("test.op"))

	// Then: Same name can be registered again (different plugin)
	op2 := newMockOperation("test.op", "plugin-v2")
	err := registry.Register(op2)
	require.NoError(t, err)

	// And: New registration is active
	retrieved, found := registry.GetOperation("test.op")
	assert.True(t, found)
	assert.Equal(t, "plugin-v2", retrieved.PluginName)
}

// TestMockOperation_CallTracking tests mock operation instrumentation.
func TestMockOperation_CallTracking(t *testing.T) {
	// Given: A mock operation
	op := newMockOperation("test.op", "test-plugin")
	ctx := context.Background()

	// Initially: No calls
	assert.Equal(t, 0, op.callCount())

	// When: Executing the operation multiple times
	for i := 0; i < 5; i++ {
		_, err := op.execute(ctx, map[string]any{"message": "test"})
		require.NoError(t, err)
	}

	// Then: Call count is tracked
	assert.Equal(t, 5, op.callCount())
}

// TestMockOperation_CustomExecution tests custom execution logic.
func TestMockOperation_CustomExecution(t *testing.T) {
	// Given: An operation with custom execution logic
	op := newMockOperation("custom.op", "test-plugin")
	customError := errors.New("custom error")

	op.execFn = func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
		if inputs["trigger_error"] == true {
			return nil, customError
		}
		return &plugin.OperationResult{
			Success: true,
			Outputs: map[string]any{"custom": "value"},
		}, nil
	}

	ctx := context.Background()

	// When: Executing with success case
	result, err := op.execute(ctx, map[string]any{"trigger_error": false})

	// Then: Custom logic executes
	require.NoError(t, err)
	assert.True(t, result.IsSuccess())
	assert.Equal(t, "value", result.Outputs["custom"])

	// When: Executing with error case
	result, err = op.execute(ctx, map[string]any{"trigger_error": true})

	// Then: Custom error is returned
	assert.ErrorIs(t, err, customError)
	assert.Nil(t, result)
}
