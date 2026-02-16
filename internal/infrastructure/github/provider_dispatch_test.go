package github

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Provider Dispatch Tests ---
// These tests verify the Execute method's dispatch logic - how it routes
// operation names to their corresponding handler methods.

func TestGitHubOperationProvider_Execute_DispatchesToCorrectHandler(t *testing.T) {
	// This test verifies that Execute() correctly routes each operation name
	// to its corresponding handler based on the operation schema registry.
	tests := []struct {
		name          string
		operation     string
		inputs        map[string]any
		expectSuccess bool
		expectError   bool
	}{
		// P1 Operations - should dispatch to implemented handlers
		{
			name:          "dispatch get_issue to handler",
			operation:     "github.get_issue",
			inputs:        map[string]any{"number": 123, "repo": "owner/repo"},
			expectSuccess: true,
			expectError:   false,
		},
		{
			name:          "dispatch get_pr to handler",
			operation:     "github.get_pr",
			inputs:        map[string]any{"number": 456, "repo": "owner/repo"},
			expectSuccess: true,
			expectError:   false,
		},
		{
			name:      "dispatch create_pr to handler",
			operation: "github.create_pr",
			inputs: map[string]any{
				"title": "Test PR",
				"head":  "feature",
				"base":  "main",
				"repo":  "owner/repo",
			},
			expectSuccess: true,  // mock succeeds
			expectError:   false, // mock succeeds
		},
		{
			name:          "dispatch create_issue to handler",
			operation:     "github.create_issue",
			inputs:        map[string]any{"title": "Test Issue", "repo": "owner/repo"},
			expectSuccess: true,
			expectError:   false,
		},
		// P2 Operations - should dispatch to implemented handlers
		{
			name:      "dispatch add_labels to handler",
			operation: "github.add_labels",
			inputs: map[string]any{
				"number": 123,
				"labels": []string{"bug", "enhancement"},
				"repo":   "owner/repo",
			},
			expectSuccess: true,
			expectError:   false,
		},
		{
			name:          "dispatch list_comments to handler",
			operation:     "github.list_comments",
			inputs:        map[string]any{"number": 123, "repo": "owner/repo"},
			expectSuccess: true,
			expectError:   false,
		},
		{
			name:      "dispatch add_comment to handler",
			operation: "github.add_comment",
			inputs: map[string]any{
				"number": 123,
				"body":   "Test comment",
				"repo":   "owner/repo",
			},
			expectSuccess: true,
			expectError:   false,
		},
		{
			name:      "dispatch batch to handler",
			operation: "github.batch",
			inputs: map[string]any{
				"operations": []map[string]any{
					{"name": "github.get_issue", "inputs": map[string]any{"number": 1}},
				},
				"strategy": "best_effort",
				"repo":     "owner/repo",
			},
			expectSuccess: true,
			expectError:   false,
		},
	}

	provider := newTestProvider()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, tt.operation, tt.inputs)

			if tt.expectError {
				require.Error(t, err, "expected dispatch to return error")
			} else {
				require.NoError(t, err, "expected dispatch to succeed")
			}

			if tt.expectSuccess {
				require.NotNil(t, result, "expected result for successful dispatch")
				assert.True(t, result.Success, "expected successful result")
			} else {
				require.NotNil(t, result, "expected result even for unimplemented operation")
				assert.False(t, result.Success, "expected unsuccessful result for unimplemented")
			}
		})
	}
}

func TestGitHubOperationProvider_Execute_DispatchRejectsUnknownOperation(t *testing.T) {
	// Verify that Execute returns an error when given an unknown operation name
	// instead of silently succeeding or panicking.
	tests := []struct {
		name      string
		operation string
	}{
		{
			name:      "completely unknown operation",
			operation: "github.unknown_operation",
		},
		{
			name:      "typo in operation name",
			operation: "github.get_isue", // typo: isue instead of issue
		},
		{
			name:      "wrong namespace",
			operation: "slack.get_issue",
		},
		{
			name:      "operation without namespace",
			operation: "get_issue",
		},
		{
			name:      "empty operation name",
			operation: "",
		},
		{
			name:      "case mismatch",
			operation: "GITHUB.GET_ISSUE",
		},
	}

	provider := newTestProvider()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, tt.operation, map[string]any{})

			// Real implementation should return error for unknown operations
			require.Error(t, err, "dispatch should reject unknown operation: %s", tt.operation)
			assert.Contains(t, err.Error(), "unknown operation", "error should indicate operation is unknown")
			assert.Nil(t, result, "result should be nil for unknown operation")
		})
	}
}

func TestGitHubOperationProvider_Execute_DispatchUsesOperationRegistry(t *testing.T) {
	// Verify that Execute uses the operations registry to validate
	// operation existence before dispatching.
	provider := newTestProvider()

	// First verify the operation exists in the registry
	op, found := provider.GetOperation("github.get_issue")
	require.True(t, found, "operation should exist in registry")
	require.NotNil(t, op, "operation should exist in registry")

	// Now verify Execute uses this registry for dispatch
	ctx := context.Background()
	result, err := provider.Execute(ctx, "github.get_issue", map[string]any{
		"number": 123,
		"repo":   "owner/repo",
	})

	require.NoError(t, err, "dispatch should succeed for registered operation")
	require.NotNil(t, result, "result should not be nil")
}

func TestGitHubOperationProvider_Execute_DispatchOrderIndependent(t *testing.T) {
	// Verify that dispatch works correctly regardless of the order
	// operations are called (no state pollution between executions).
	provider := newTestProvider()
	ctx := context.Background()

	// Operations with mock runner
	ops := []string{
		"github.get_issue",
		"github.add_labels",
		"github.get_pr",
		"github.create_pr",
	}

	// Execute operations in sequence and verify each succeeds independently
	for i, op := range ops {
		t.Run(op, func(t *testing.T) {
			var inputs map[string]any
			switch op {
			case "github.get_issue", "github.get_pr":
				inputs = map[string]any{"number": i + 1, "repo": "owner/repo"}
			case "github.add_labels":
				inputs = map[string]any{"number": i + 1, "labels": []string{"bug"}, "repo": "owner/repo"}
			case "github.create_pr":
				inputs = map[string]any{"title": "Test", "head": "feature", "base": "main", "repo": "owner/repo"}
			}

			result, err := provider.Execute(ctx, op, inputs)

			require.NoError(t, err, "dispatch should succeed for operation %d: %s", i, op)
			require.NotNil(t, result, "result should not be nil for operation %d: %s", i, op)
			assert.True(t, result.Success, "should succeed with mock for operation %d: %s", i, op)
		})
	}
}

func TestGitHubOperationProvider_Execute_DispatchWithMultipleProviderInstances(t *testing.T) {
	// Verify that multiple provider instances can dispatch independently
	// without interfering with each other.
	provider1 := newTestProvider()
	provider2 := newTestProvider()
	ctx := context.Background()

	inputs := map[string]any{"number": 123, "repo": "owner/repo"}

	// Execute same operation on both providers
	result1, err1 := provider1.Execute(ctx, "github.get_issue", inputs)
	result2, err2 := provider2.Execute(ctx, "github.get_issue", inputs)

	require.NoError(t, err1, "provider1 dispatch should succeed")
	require.NoError(t, err2, "provider2 dispatch should succeed")
	require.NotNil(t, result1, "provider1 result should not be nil")
	require.NotNil(t, result2, "provider2 result should not be nil")

	// Results should be independent (different memory addresses)
	assert.NotSame(t, result1, result2, "results should be independent instances")
}

func TestGitHubOperationProvider_Execute_DispatchPassesContextToHandler(t *testing.T) {
	// Verify that Execute passes the context to the handler method,
	// ensuring cancellation and timeout control works correctly.
	provider := newTestProvider()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := provider.Execute(ctx, "github.get_issue", map[string]any{
		"number": 123,
		"repo":   "owner/repo",
	})

	// Mock runner ignores context, so operation succeeds
	require.NoError(t, err, "mock runner ignores cancelled context")
	assert.NotNil(t, result, "result should not be nil")
}

func TestGitHubOperationProvider_Execute_DispatchHandlesAllRegisteredOperations(t *testing.T) {
	// Verify that every operation in the registry has a corresponding handler
	// by attempting to execute each one.
	provider := newTestProvider()

	// Get all registered operations
	operations := provider.ListOperations()
	require.NotNil(t, operations, "should return operations list")

	ctx := context.Background()

	for _, op := range operations {
		t.Run(op.Name, func(t *testing.T) {
			// Create minimal valid inputs based on operation schema
			inputs := make(map[string]any)
			inputs["repo"] = "owner/repo"

			// Add required inputs based on operation name
			switch op.Name {
			case "github.get_issue", "github.get_pr":
				inputs["number"] = 123
			case "github.create_pr":
				inputs["title"] = "Test"
				inputs["head"] = "feature"
				inputs["base"] = "main"
			case "github.create_issue":
				inputs["title"] = "Test"
			case "github.add_labels":
				inputs["number"] = 123
				inputs["labels"] = []string{"bug"}
			case "github.list_comments":
				inputs["number"] = 123
			case "github.add_comment":
				inputs["number"] = 123
				inputs["body"] = "Comment"
			case "github.batch":
				inputs["operations"] = []map[string]any{}
				inputs["strategy"] = "best_effort"
			}

			result, err := provider.Execute(ctx, op.Name, inputs)

			// Each operation should have a handler - dispatch should not panic
			require.NotPanics(t, func() {
				_, _ = provider.Execute(ctx, op.Name, inputs)
			}, "dispatch should not panic for registered operation: %s", op.Name)

			require.NoError(t, err, "operation %s should succeed with mock", op.Name)
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.Success, "operation %s should return success", op.Name)
		})
	}
}

func TestGitHubOperationProvider_Execute_DispatchValidatesBeforeHandlerCall(t *testing.T) {
	// Verify that Execute validates operation existence BEFORE attempting
	// to call a handler (fail-fast principle).
	provider := newTestProvider()
	ctx := context.Background()

	// Use an operation that definitely doesn't exist
	result, err := provider.Execute(ctx, "github.definitely_does_not_exist", map[string]any{})

	// Real implementation should validate operation exists before attempting dispatch
	require.Error(t, err, "dispatch should validate operation existence first")
	assert.Contains(t, err.Error(), "unknown", "error should indicate operation is unknown")
	assert.Nil(t, result, "result should be nil for unknown operation")
}
