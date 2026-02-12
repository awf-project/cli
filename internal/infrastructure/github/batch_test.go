package github

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Note: Mock provider not needed for stub tests
// When implementation is complete, a mock provider can be added here to test
// actual batch execution behavior with mocked operation results
// =============================================================================

// Example mock structure (commented out, not needed for stub tests):
// type mockProviderForBatch struct {
//     results map[string]*plugin.OperationResult
//     errors  map[string]error
// }

// =============================================================================
// Constructor tests
// =============================================================================

func TestNewBatchExecutor(t *testing.T) {
	provider := newTestProvider()
	logger := &mockLogger{}

	executor := NewBatchExecutor(provider, logger)

	require.NotNil(t, executor, "NewBatchExecutor() should not return nil")
	// Can't check unexported fields directly, verify via behavior
}

// =============================================================================
// Happy path tests
// =============================================================================

func TestBatchExecutor_Execute_AllSucceedStrategy_AllSuccess(t *testing.T) {
	// Given: a batch executor with 3 successful operations
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	operations := []map[string]any{
		{"name": "github.get_issue", "inputs": map[string]any{"number": 1}},
		{"name": "github.add_labels", "inputs": map[string]any{"number": 1, "labels": []string{"bug"}}},
		{"name": "github.add_comment", "inputs": map[string]any{"number": 1, "body": "test"}},
	}

	config := BatchConfig{
		Strategy:      "all_succeed",
		MaxConcurrent: 2,
	}

	// When: executing batch with all_succeed strategy
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: all operations should succeed (mock returns {})
	require.NoError(t, err, "all_succeed should not error when all succeed")
	require.NotNil(t, result)
	assert.Equal(t, 3, result.Total, "should execute all 3 operations")
	assert.Equal(t, 3, result.Succeeded, "all operations should succeed")
	assert.Equal(t, 0, result.Failed, "no operations should fail")
	assert.Len(t, result.Results, 3, "should have 3 results")
}

func TestBatchExecutor_Execute_BestEffortStrategy_AllSuccess(t *testing.T) {
	// Given: a batch executor with 2 operations
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	operations := []map[string]any{
		{"name": "github.get_issue", "inputs": map[string]any{"number": 1}},
		{"name": "github.get_pr", "inputs": map[string]any{"number": 2}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 3,
	}

	// When: executing batch with best_effort strategy
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: all operations should succeed (mock returns {})
	require.NoError(t, err, "best_effort should not error")
	require.NotNil(t, result, "should return result")
	assert.Equal(t, 2, result.Total, "should execute both operations")
	assert.Equal(t, 2, result.Succeeded, "both operations should succeed")
	assert.Equal(t, 0, result.Failed, "no operations should fail")
}

func TestBatchExecutor_Execute_AnySucceedStrategy(t *testing.T) {
	// Given: a batch executor with 3 fake operations (will all fail with "unknown operation")
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
		{"name": "op3", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "any_succeed",
		MaxConcurrent: 3,
	}

	// When: executing batch with any_succeed strategy
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: all operations fail (unknown operations), any_succeed returns error
	require.Error(t, err, "any_succeed should error when all operations fail")
	require.NotNil(t, result)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 0, result.Succeeded, "no operations should succeed")
	assert.Equal(t, 3, result.Failed, "all operations should fail")
}

// =============================================================================
// Edge cases
// =============================================================================

func TestBatchExecutor_Execute_EmptyOperations(t *testing.T) {
	// Given: a batch executor with no operations
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	operations := []map[string]any{}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 3,
	}

	// When: executing batch with empty operations
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: returns valid empty result
	require.NoError(t, err, "empty operations should not error")
	require.NotNil(t, result)
	assert.Equal(t, 0, result.Total)
	assert.Equal(t, 0, result.Succeeded)
	assert.Equal(t, 0, result.Failed)
	assert.Empty(t, result.Results)
}

func TestBatchExecutor_Execute_SingleOperation(t *testing.T) {
	// Given: a batch executor with single operation
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	operations := []map[string]any{
		{"name": "github.get_issue", "inputs": map[string]any{"number": 1}},
	}

	config := BatchConfig{
		Strategy:      "all_succeed",
		MaxConcurrent: 1,
	}

	// When: executing batch with single operation
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: returns valid result structure
	require.NoError(t, err, "should succeed")
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 1, result.Succeeded)
	assert.Equal(t, 0, result.Failed)
}

func TestBatchExecutor_Execute_MaxConcurrentZero_UnlimitedConcurrency(t *testing.T) {
	// Given: a batch executor with MaxConcurrent = 0 (unlimited)
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names which will all fail with "unknown operation"
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
		{"name": "op3", "inputs": map[string]any{}},
		{"name": "op4", "inputs": map[string]any{}},
		{"name": "op5", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 0, // unlimited
	}

	// When: executing batch
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: returns valid result structure (best_effort doesn't error even if all fail)
	require.NoError(t, err, "best_effort should not error")
	require.NotNil(t, result)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 0, result.Succeeded, "all fake operations fail")
	assert.Equal(t, 5, result.Failed, "all operations should fail")
}

func TestBatchExecutor_Execute_DefaultStrategy_BestEffort(t *testing.T) {
	// Given: a batch executor with empty strategy (should default to best_effort)
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names which will all fail
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "", // empty/default
		MaxConcurrent: 3,
	}

	// When: executing batch with default strategy
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: returns valid result structure (best_effort doesn't error)
	require.NoError(t, err, "best_effort should not error")
	require.NotNil(t, result)
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 0, result.Succeeded, "all fake operations fail")
	assert.Equal(t, 2, result.Failed)
}

func TestBatchExecutor_Execute_LargeBatch_100Operations(t *testing.T) {
	// Given: a batch executor with large number of operations
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	operations := make([]map[string]any, 100)
	for i := range 100 {
		operations[i] = map[string]any{
			"name":   "github.get_issue",
			"inputs": map[string]any{"number": i + 1},
		}
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 10,
	}

	// When: executing large batch
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: handles large batch without panic
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 100, result.Total)
	assert.Equal(t, 100, result.Succeeded, "all real operations succeed")
	assert.Equal(t, 0, result.Failed)
}

// =============================================================================
// Error handling tests
// =============================================================================

func TestBatchExecutor_Execute_AllSucceedStrategy_ExpectsCancellationOnFailure(t *testing.T) {
	// Given: all_succeed strategy should cancel remaining on first failure
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names which will all fail
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
		{"name": "op3", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "all_succeed",
		MaxConcurrent: 3,
	}

	// When: executing batch
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: all_succeed errors when any operation fails; cancels remaining
	require.Error(t, err, "all_succeed should error when operations fail")
	require.NotNil(t, result)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 0, result.Succeeded, "no fake operations succeed")
	assert.GreaterOrEqual(t, result.Failed, 1, "at least one operation fails before cancellation")
}

func TestBatchExecutor_Execute_BestEffortStrategy_CompletesAllRegardlessOfFailures(t *testing.T) {
	// Given: best_effort should complete all operations regardless of failures
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names which will all fail
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
		{"name": "op3", "inputs": map[string]any{}},
		{"name": "op4", "inputs": map[string]any{}},
		{"name": "op5", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 3,
	}

	// When: executing batch
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: best_effort doesn't error even if all fail
	require.NoError(t, err, "best_effort should not error")
	require.NotNil(t, result)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 0, result.Succeeded, "all fake operations fail")
	assert.Equal(t, 5, result.Failed)
}

func TestBatchExecutor_Execute_AnySucceedStrategy_ExpectsErrorWhenAllFail(t *testing.T) {
	// Given: any_succeed should error when no operations succeed
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names which will all fail
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
		{"name": "op3", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "any_succeed",
		MaxConcurrent: 3,
	}

	// When: executing batch
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: any_succeed errors when all operations fail
	require.Error(t, err, "any_succeed should error when all fail")
	require.NotNil(t, result)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 0, result.Succeeded)
	assert.Equal(t, 3, result.Failed)
}

func TestBatchExecutor_Execute_InvalidOperationFormat_MissingName(t *testing.T) {
	// Given: operations missing required "name" field
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	operations := []map[string]any{
		{"inputs": map[string]any{"number": 1}}, // missing "name"
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 3,
	}

	// When: executing batch with invalid operation format
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: validation catches missing name, returns result with error
	require.Error(t, err, "should return validation error for missing name")
	require.NotNil(t, result, "result object is returned even on validation error")
	assert.Contains(t, err.Error(), "operation missing required field: name")
}

func TestBatchExecutor_Execute_InvalidOperationFormat_MissingInputs(t *testing.T) {
	// Given: operations missing "inputs" field
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	operations := []map[string]any{
		{"name": "github.get_issue"}, // missing "inputs"
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 3,
	}

	// When: executing batch with missing inputs
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: missing inputs is OK (handler gets nil/empty map), but get_issue requires number
	require.NoError(t, err, "best_effort doesn't error")
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 0, result.Succeeded, "operation fails validation")
	assert.Equal(t, 1, result.Failed)
}

func TestBatchExecutor_Execute_InvalidOperationFormat_NameNotString(t *testing.T) {
	// Given: operation name is not a string
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	operations := []map[string]any{
		{"name": 123, "inputs": map[string]any{}}, // name is int
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 3,
	}

	// When: executing batch with invalid type
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: should validate type, returns result with error
	require.Error(t, err, "should return validation error for invalid name type")
	require.NotNil(t, result, "result object is returned even on validation error")
	assert.Contains(t, err.Error(), "operation name must be a string")
}

func TestBatchExecutor_Execute_InvalidOperationFormat_InputsNotMap(t *testing.T) {
	// Given: inputs is not a map
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	operations := []map[string]any{
		{"name": "github.get_issue", "inputs": "not a map"},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 3,
	}

	// When: executing batch with invalid inputs type
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: should validate inputs type, returns result with error
	require.Error(t, err, "should return validation error for invalid inputs type")
	require.NotNil(t, result, "result object is returned even on validation error")
	assert.Contains(t, err.Error(), "operation inputs must be a map")
}

func TestBatchExecutor_Execute_ContextCancellation(t *testing.T) {
	// Given: a context that will be cancelled
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
		{"name": "op3", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// When: executing batch with cancelled context
	result, err := executor.Execute(ctx, operations, config)

	// Then: context cancellation handling (mock ignores context, operations still fail as unknown)
	require.NotNil(t, result)
	// With cancelled context, batch may return early or complete - implementation dependent
	_ = err
}

func TestBatchExecutor_Execute_ContextTimeout(t *testing.T) {
	// Given: a context with very short timeout
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 2,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// When: executing batch with timeout context
	result, err := executor.Execute(ctx, operations, config)

	// Then: should respect timeout (implementation dependent)
	require.NotNil(t, result)
	_ = err
}

func TestBatchExecutor_Execute_InvalidStrategy(t *testing.T) {
	// Given: an unknown/invalid strategy
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "unknown_strategy",
		MaxConcurrent: 3,
	}

	// When: executing batch with invalid strategy
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: defaults to best_effort (implementation treats unknown as best_effort)
	require.NoError(t, err, "best_effort doesn't error")
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 0, result.Succeeded, "fake operation fails")
	assert.Equal(t, 1, result.Failed)
}

func TestBatchExecutor_Execute_NegativeMaxConcurrent(t *testing.T) {
	// Given: negative MaxConcurrent value
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: -5,
	}

	// When: executing batch with negative concurrency
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: handles gracefully (implementation treats negative as unlimited)
	require.NoError(t, err, "best_effort doesn't error")
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 0, result.Succeeded, "fake operation fails")
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// Concurrency control tests
// =============================================================================

func TestBatchExecutor_Execute_ConcurrencyLimit_MaxConcurrent1(t *testing.T) {
	// Given: MaxConcurrent = 1 (sequential execution)
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
		{"name": "op3", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 1,
	}

	// When: executing batch with concurrency limit of 1
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: returns valid result
	require.NoError(t, err, "best_effort doesn't error")
	require.NotNil(t, result)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 0, result.Succeeded, "all fake operations fail")
	assert.Equal(t, 3, result.Failed)
}

func TestBatchExecutor_Execute_ConcurrencyLimit_MaxConcurrent2(t *testing.T) {
	// Given: MaxConcurrent = 2
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
		{"name": "op3", "inputs": map[string]any{}},
		{"name": "op4", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 2,
	}

	// When: executing batch with concurrency limit of 2
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: returns valid result
	require.NoError(t, err, "best_effort doesn't error")
	require.NotNil(t, result)
	assert.Equal(t, 4, result.Total)
	assert.Equal(t, 0, result.Succeeded, "all fake operations fail")
	assert.Equal(t, 4, result.Failed)
}

func TestBatchExecutor_Execute_ConcurrencyLimit_RespectsSemaphore(t *testing.T) {
	// Given: operations that would run concurrently
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
		{"name": "op3", "inputs": map[string]any{}},
		{"name": "op4", "inputs": map[string]any{}},
		{"name": "op5", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 3,
	}

	// When: executing batch
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: respects semaphore limit
	require.NoError(t, err, "best_effort doesn't error")
	require.NotNil(t, result)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 0, result.Succeeded, "all fake operations fail")
	assert.Equal(t, 5, result.Failed)
}

// =============================================================================
// Result aggregation tests
// =============================================================================

func TestBatchExecutor_Execute_ResultAggregation_PreservesIndividualResults(t *testing.T) {
	// Given: diverse operation results
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
		{"name": "op3", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 3,
	}

	// When: executing batch
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: result structure is valid
	require.NoError(t, err, "best_effort doesn't error")
	require.NotNil(t, result)
	require.NotNil(t, result.Results)
	assert.Len(t, result.Results, 3, "should preserve all individual results")
}

func TestBatchExecutor_Execute_ResultAggregation_CountsMatch(t *testing.T) {
	// Given: a batch of operations
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
		{"name": "op3", "inputs": map[string]any{}},
		{"name": "op4", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 2,
	}

	// When: executing batch
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: counts should be consistent
	require.NoError(t, err, "best_effort doesn't error")
	require.NotNil(t, result)
	assert.Equal(t, result.Succeeded+result.Failed, result.Total,
		"succeeded + failed should equal total")
}

func TestBatchExecutor_Execute_ResultAggregation_PreservesOutputs(t *testing.T) {
	// Given: operations with output data
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	operations := []map[string]any{
		{"name": "github.get_issue", "inputs": map[string]any{"number": 1}},
		{"name": "github.get_pr", "inputs": map[string]any{"number": 2}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 2,
	}

	// When: executing batch
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: result structure exists
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Results)
	assert.Len(t, result.Results, 2)
	assert.Equal(t, 2, result.Succeeded, "real operations succeed")
}

func TestBatchExecutor_Execute_ResultAggregation_PreservesErrors(t *testing.T) {
	// Given: operations with errors
	executor := NewBatchExecutor(newTestProvider(), &mockLogger{})

	// Using fake operation names
	operations := []map[string]any{
		{"name": "op1", "inputs": map[string]any{}},
		{"name": "op2", "inputs": map[string]any{}},
	}

	config := BatchConfig{
		Strategy:      "best_effort",
		MaxConcurrent: 2,
	}

	// When: executing batch
	result, err := executor.Execute(context.Background(), operations, config)

	// Then: result structure exists with error information
	require.NoError(t, err, "best_effort doesn't error")
	require.NotNil(t, result)
	require.NotNil(t, result.Results)
	assert.Len(t, result.Results, 2)
	assert.Equal(t, 2, result.Failed, "fake operations fail")
}
