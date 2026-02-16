package github

import (
	"context"
	"testing"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Interface compliance tests ---

func TestGitHubOperationProvider_ImplementsInterface(t *testing.T) {
	var _ ports.OperationProvider = (*GitHubOperationProvider)(nil)
}

// --- Constructor tests ---

func TestNewGitHubOperationProvider(t *testing.T) {
	runner := &mockGHRunner{output: []byte("{}"), err: nil}
	logger := &mockLogger{}

	provider := NewGitHubOperationProvider(runner, logger)

	require.NotNil(t, provider, "NewGitHubOperationProvider() should not return nil")
	// Can't check unexported fields (runner, logger) directly - verify via behavior
	assert.Len(t, provider.operations, len(AllOperations()), "operations map should be initialized with all operations")
}

func TestNewGitHubOperationProvider_MultipleInstances(t *testing.T) {
	runner := &mockGHRunner{output: []byte("{}"), err: nil}
	logger := &mockLogger{}

	p1 := NewGitHubOperationProvider(runner, logger)
	p2 := NewGitHubOperationProvider(runner, logger)

	assert.NotSame(t, p1, p2, "each call should return a new instance")
	// Both providers should have same operations registered but be independent instances
	assert.Len(t, p1.operations, len(AllOperations()))
	assert.Len(t, p2.operations, len(AllOperations()))
}

func TestNewGitHubOperationProvider_RegistersAllOperations(t *testing.T) {
	runner := &mockGHRunner{output: []byte("{}"), err: nil}
	logger := &mockLogger{}

	provider := NewGitHubOperationProvider(runner, logger)

	// AllOperations() returns 8 operations (per spec)
	expectedOps := AllOperations()
	assert.Len(t, provider.operations, len(expectedOps), "should register all operations")

	// Verify each operation is registered
	for _, op := range expectedOps {
		registered, exists := provider.operations[op.Name]
		assert.True(t, exists, "operation %s should be registered", op.Name)
		assert.Equal(t, op.Name, registered.Name, "registered operation name should match")
	}
}

// --- GetOperation tests ---

func TestGitHubOperationProvider_GetOperation_P1Operations(t *testing.T) {
	tests := []struct {
		name   string
		opName string
	}{
		{"get_issue", "github.get_issue"},
		{"get_pr", "github.get_pr"},
		{"create_pr", "github.create_pr"},
		{"create_issue", "github.create_issue"},
	}

	provider := newTestProvider()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, found := provider.GetOperation(tt.opName)

			assert.True(t, found, "P1 operation %s should be found", tt.opName)
			require.NotNil(t, op, "P1 operation %s should return schema", tt.opName)
			assert.Equal(t, tt.opName, op.Name, "operation name should match")
		})
	}
}

func TestGitHubOperationProvider_GetOperation_P2Operations(t *testing.T) {
	tests := []struct {
		name   string
		opName string
	}{
		{"add_labels", "github.add_labels"},
		{"list_comments", "github.list_comments"},
		{"add_comment", "github.add_comment"},
		{"batch", "github.batch"},
	}

	provider := newTestProvider()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, found := provider.GetOperation(tt.opName)

			assert.True(t, found, "P2 operation %s should be found", tt.opName)
			require.NotNil(t, op, "P2 operation %s should return schema", tt.opName)
			assert.Equal(t, tt.opName, op.Name, "operation name should match")
		})
	}
}

func TestGitHubOperationProvider_GetOperation_NonExistent(t *testing.T) {
	provider := newTestProvider()

	op, found := provider.GetOperation("github.nonexistent")
	assert.False(t, found, "should return false for non-existent operation")
	assert.Nil(t, op, "should return nil for non-existent operation")
}

func TestGitHubOperationProvider_GetOperation_EmptyName(t *testing.T) {
	provider := newTestProvider()

	op, found := provider.GetOperation("")
	assert.False(t, found, "should return false for empty name")
	assert.Nil(t, op, "should return nil for empty name")
}

func TestGitHubOperationProvider_GetOperation_CaseSensitive(t *testing.T) {
	provider := newTestProvider()

	// Operation names should be case-sensitive
	_, found := provider.GetOperation("GITHUB.GET_ISSUE")
	assert.False(t, found, "operation names should be case-sensitive")

	_, found = provider.GetOperation("GitHub.Get_Issue")
	assert.False(t, found, "operation names should be case-sensitive")
}

func TestGitHubOperationProvider_GetOperation_WrongNamespace(t *testing.T) {
	provider := newTestProvider()

	// All GitHub operations should be namespaced with "github."
	_, found := provider.GetOperation("get_issue")
	assert.False(t, found, "should not find operation without github. prefix")

	_, found = provider.GetOperation("slack.get_issue")
	assert.False(t, found, "should not find operation with wrong namespace")
}

// --- ListOperations tests ---

func TestGitHubOperationProvider_ListOperations_ReturnsAllOperations(t *testing.T) {
	provider := newTestProvider()

	ops := provider.ListOperations()

	require.NotNil(t, ops, "should return all registered operations")

	// Verify all 8 operation names are present
	names := make(map[string]bool)
	for _, op := range ops {
		names[op.Name] = true
	}
	for _, expected := range AllOperations() {
		assert.True(t, names[expected.Name], "should contain operation %s", expected.Name)
	}
}

func TestGitHubOperationProvider_ListOperations_ExpectedCount(t *testing.T) {
	provider := newTestProvider()

	ops := provider.ListOperations()

	// 8 operations: 4 P1 + 4 P2
	require.NotNil(t, ops, "should return operations")
	assert.Len(t, ops, 8, "should return 8 operations (4 P1 + 4 P2)")
}

func TestGitHubOperationProvider_ListOperations_NotNilEvenWhenEmpty(t *testing.T) {
	provider := newTestProvider()

	ops := provider.ListOperations()

	// Should return non-nil slice for consistent iteration
	assert.NotNil(t, ops, "should return non-nil slice")
}

// --- Execute tests ---

func TestGitHubOperationProvider_Execute_GetIssue_HappyPath(t *testing.T) {
	provider := newTestProvider()
	ctx := context.Background()

	inputs := map[string]any{
		"number": 123,
		"repo":   "owner/repo",
	}

	result, err := provider.Execute(ctx, "github.get_issue", inputs)

	// Mock returns {} which parseJSONOutputs converts to empty map
	require.NoError(t, err, "should not return error with valid inputs")
	assert.NotNil(t, result, "result should not be nil")
	assert.True(t, result.Success, "should return success")
	assert.NotNil(t, result.Outputs, "outputs should not be nil")
	assert.Empty(t, result.Outputs, "mock outputs should be empty map from {}")
}

func TestGitHubOperationProvider_Execute_CreatePR_HappyPath(t *testing.T) {
	provider := newTestProvider()
	ctx := context.Background()

	inputs := map[string]any{
		"title": "Fix bug",
		"head":  "feature/fix",
		"base":  "main",
		"body":  "This fixes the bug",
	}

	result, err := provider.Execute(ctx, "github.create_pr", inputs)

	// Mock returns {} which handleCreatePR treats as URL string
	require.NoError(t, err, "should not return error with valid inputs")
	assert.NotNil(t, result, "result should not be nil")
	assert.True(t, result.Success, "should return success")
}

func TestGitHubOperationProvider_Execute_Batch_HappyPath(t *testing.T) {
	provider := newTestProvider()
	ctx := context.Background()

	inputs := map[string]any{
		"operations": []map[string]any{},
		"strategy":   "best_effort",
	}

	result, err := provider.Execute(ctx, "github.batch", inputs)

	// Empty operations batch returns immediately
	require.NoError(t, err, "should not return error")
	assert.NotNil(t, result, "result should not be nil")
	assert.True(t, result.Success, "should return success")
}

func TestGitHubOperationProvider_Execute_UnknownOperation(t *testing.T) {
	provider := newTestProvider()
	ctx := context.Background()

	inputs := map[string]any{}

	result, err := provider.Execute(ctx, "github.unknown_operation", inputs)

	// Unknown operations now return error
	require.Error(t, err, "should return error for unknown operation")
	assert.Nil(t, result, "result should be nil")
	assert.Contains(t, err.Error(), "unknown operation")
}

func TestGitHubOperationProvider_Execute_EmptyOperationName(t *testing.T) {
	provider := newTestProvider()
	ctx := context.Background()

	inputs := map[string]any{}

	result, err := provider.Execute(ctx, "", inputs)

	// Empty operation name returns error
	require.Error(t, err, "should return error for empty operation name")
	assert.Nil(t, result, "result should be nil")
}

func TestGitHubOperationProvider_Execute_NilInputs(t *testing.T) {
	provider := newTestProvider()
	ctx := context.Background()

	result, err := provider.Execute(ctx, "github.get_issue", nil)

	// get_issue with nil inputs: formatNumber(nil) returns "" which fails validation
	require.Error(t, err, "should return error for missing required input")
	assert.Nil(t, result, "result should be nil")
	assert.Contains(t, err.Error(), "number is required")
}

func TestGitHubOperationProvider_Execute_EmptyInputs(t *testing.T) {
	provider := newTestProvider()
	ctx := context.Background()

	inputs := map[string]any{}

	result, err := provider.Execute(ctx, "github.get_issue", inputs)

	// get_issue requires "number" field
	require.Error(t, err, "should return error for missing required input")
	assert.Nil(t, result, "result should be nil")
	assert.Contains(t, err.Error(), "number is required")
}

func TestGitHubOperationProvider_Execute_MissingRequiredInput(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		inputs    map[string]any
		wantErr   bool
	}{
		{
			name:      "get_issue missing number",
			operation: "github.get_issue",
			inputs:    map[string]any{"repo": "owner/repo"},
			wantErr:   true, // number is required
		},
		{
			name:      "create_pr missing title",
			operation: "github.create_pr",
			inputs:    map[string]any{"head": "feature", "base": "main"},
			wantErr:   true, // title is required
		},
		{
			name:      "create_pr missing head",
			operation: "github.create_pr",
			inputs:    map[string]any{"title": "Fix", "base": "main"},
			wantErr:   true, // head is required
		},
		{
			name:      "add_labels missing labels",
			operation: "github.add_labels",
			inputs:    map[string]any{"number": 123},
			wantErr:   true, // labels is required
		},
	}

	provider := newTestProvider()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, tt.operation, tt.inputs)

			if tt.wantErr {
				require.Error(t, err, "should return validation error for missing required input")
				assert.Nil(t, result, "result should be nil when validation fails")
			} else {
				require.NoError(t, err, "should succeed")
				assert.NotNil(t, result, "result should not be nil")
				assert.True(t, result.Success, "should succeed")
			}
		})
	}
}

func TestGitHubOperationProvider_Execute_InvalidInputType(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		inputs    map[string]any
		wantErr   bool
	}{
		{
			name:      "number as string instead of integer - formatNumber handles",
			operation: "github.get_issue",
			inputs:    map[string]any{"number": "123"},
			wantErr:   false, // formatNumber converts strings
		},
		{
			name:      "draft as string instead of boolean - parseBool handles",
			operation: "github.create_pr",
			inputs: map[string]any{
				"title": "Fix",
				"head":  "feature",
				"base":  "main",
				"draft": "true",
			},
			wantErr: false, // parseBool handles string, mock succeeds
		},
		{
			name:      "labels as string instead of array",
			operation: "github.add_labels",
			inputs: map[string]any{
				"number": 123,
				"labels": "bug,enhancement",
			},
			wantErr: true, // toStringSlice returns nil for non-slice
		},
	}

	provider := newTestProvider()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, tt.operation, tt.inputs)

			if tt.wantErr {
				require.Error(t, err, "should return error for invalid input type")
			} else {
				require.NoError(t, err, "should handle type conversion")
				assert.NotNil(t, result, "result should not be nil")
			}
		})
	}
}

func TestGitHubOperationProvider_Execute_ContextCancellation(t *testing.T) {
	provider := newTestProvider()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inputs := map[string]any{"number": 123}

	result, err := provider.Execute(ctx, "github.get_issue", inputs)

	// Mock runner ignores context, so operation succeeds
	require.NoError(t, err, "mock runner ignores cancelled context")
	assert.NotNil(t, result, "result should not be nil")
}

func TestGitHubOperationProvider_Execute_AllP1Operations(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		inputs    map[string]any
		wantErr   bool
	}{
		{
			name:      "get_issue",
			operation: "github.get_issue",
			inputs:    map[string]any{"number": 123},
			wantErr:   false, // mock succeeds
		},
		{
			name:      "get_pr",
			operation: "github.get_pr",
			inputs:    map[string]any{"number": 456},
			wantErr:   false, // mock succeeds
		},
		{
			name:      "create_pr",
			operation: "github.create_pr",
			inputs: map[string]any{
				"title": "Test PR",
				"head":  "feature",
				"base":  "main",
			},
			wantErr: false, // mock succeeds
		},
		{
			name:      "create_issue",
			operation: "github.create_issue",
			inputs:    map[string]any{"title": "Test Issue"},
			wantErr:   false, // mock succeeds
		},
	}

	provider := newTestProvider()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, tt.operation, tt.inputs)

			if tt.wantErr {
				require.Error(t, err, "should return error")
			} else {
				require.NoError(t, err, "should not return error with mock")
				assert.NotNil(t, result, "result should not be nil")
				assert.True(t, result.Success, "should return success with mock")
			}
		})
	}
}

func TestGitHubOperationProvider_Execute_AllP2Operations(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		inputs    map[string]any
	}{
		{
			name:      "add_labels",
			operation: "github.add_labels",
			inputs: map[string]any{
				"number": 123,
				"labels": []string{"bug", "enhancement"},
			},
		},
		{
			name:      "list_comments",
			operation: "github.list_comments",
			inputs:    map[string]any{"number": 123},
		},
		{
			name:      "add_comment",
			operation: "github.add_comment",
			inputs: map[string]any{
				"number": 123,
				"body":   "Test comment",
			},
		},
		{
			name:      "batch",
			operation: "github.batch",
			inputs: map[string]any{
				"operations": []map[string]any{},
				"strategy":   "best_effort",
			},
		},
	}

	provider := newTestProvider()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, tt.operation, tt.inputs)

			require.NoError(t, err, "should not return error with mock")
			assert.NotNil(t, result, "result should not be nil")
			assert.True(t, result.Success, "should return success with mock")
		})
	}
}

// --- Edge case tests ---

func TestGitHubOperationProvider_Execute_SpecialCharactersInInputs(t *testing.T) {
	tests := []struct {
		name   string
		inputs map[string]any
	}{
		{
			name: "unicode in title",
			inputs: map[string]any{
				"title": "修正バグ 🐛",
				"head":  "feature",
				"base":  "main",
			},
		},
		{
			name: "special chars in body",
			inputs: map[string]any{
				"title": "Test",
				"head":  "feature",
				"base":  "main",
				"body":  "Test with <html> & \"quotes\" and 'apostrophes'",
			},
		},
		{
			name: "newlines in body",
			inputs: map[string]any{
				"title": "Test",
				"head":  "feature",
				"base":  "main",
				"body":  "Line 1\nLine 2\r\nLine 3",
			},
		},
	}

	provider := newTestProvider()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, "github.create_pr", tt.inputs)

			// Mock runner succeeds
			require.NoError(t, err, "should not return error with mock")
			assert.NotNil(t, result, "result should not be nil")
			assert.True(t, result.Success, "should succeed with mock")
		})
	}
}

func TestGitHubOperationProvider_Execute_LargeInputs(t *testing.T) {
	provider := newTestProvider()
	ctx := context.Background()

	// Very long body text
	longBody := make([]byte, 10000)
	for i := range longBody {
		longBody[i] = 'a'
	}

	inputs := map[string]any{
		"title": "Test",
		"head":  "feature",
		"base":  "main",
		"body":  string(longBody),
	}

	result, err := provider.Execute(ctx, "github.create_pr", inputs)

	// Mock runner succeeds
	require.NoError(t, err, "should not return error with mock")
	assert.NotNil(t, result, "result should not be nil")
	assert.True(t, result.Success, "should succeed with mock")
}

func TestGitHubOperationProvider_Execute_EmptyStringInputs(t *testing.T) {
	provider := newTestProvider()
	ctx := context.Background()

	inputs := map[string]any{
		"title": "",
		"head":  "",
		"base":  "",
	}

	result, err := provider.Execute(ctx, "github.create_pr", inputs)

	// create_pr validates that title, head, and base are non-empty
	require.Error(t, err, "should return validation error for empty required fields")
	assert.Nil(t, result, "result should be nil when validation fails")
	assert.Contains(t, err.Error(), "title, head, and base are required")
}

func TestGitHubOperationProvider_Execute_NullInputValues(t *testing.T) {
	provider := newTestProvider()
	ctx := context.Background()

	inputs := map[string]any{
		"number": nil,
		"repo":   nil,
	}

	result, err := provider.Execute(ctx, "github.get_issue", inputs)

	// formatNumber(nil) returns "" which fails validation
	require.Error(t, err, "should return error for nil number")
	assert.Nil(t, result, "result should be nil")
}
