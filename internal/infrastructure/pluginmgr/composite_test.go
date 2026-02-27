package pluginmgr

import (
	"context"
	"fmt"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Interface compliance tests ---

func TestCompositeOperationProvider_ImplementsInterface(t *testing.T) {
	var _ ports.OperationProvider = (*CompositeOperationProvider)(nil)
}

// --- Constructor tests ---

func TestNewCompositeOperationProvider(t *testing.T) {
	tests := []struct {
		name              string
		providers         []ports.OperationProvider
		expectedProvCount int
	}{
		{
			name:              "empty_providers",
			providers:         []ports.OperationProvider{},
			expectedProvCount: 0,
		},
		{
			name: "single_provider",
			providers: []ports.OperationProvider{
				newMockProvider("provider1", []string{"op1", "op2"}),
			},
			expectedProvCount: 1,
		},
		{
			name: "multiple_providers",
			providers: []ports.OperationProvider{
				newMockProvider("provider1", []string{"op1"}),
				newMockProvider("provider2", []string{"op2"}),
			},
			expectedProvCount: 2,
		},
		{
			name: "github_and_notify_providers",
			providers: []ports.OperationProvider{
				newMockProvider("github", []string{"github.get_issue", "github.create_pr"}),
				newMockProvider("notify", []string{"notify.send"}),
			},
			expectedProvCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composite := NewCompositeOperationProvider(tt.providers...)

			require.NotNil(t, composite, "NewCompositeOperationProvider() should not return nil")
			assert.Len(t, composite.providers, tt.expectedProvCount, "should have expected provider count")
		})
	}
}

func TestNewCompositeOperationProvider_VariadicArgs(t *testing.T) {
	p1 := newMockProvider("p1", []string{"op1"})
	p2 := newMockProvider("p2", []string{"op2"})
	p3 := newMockProvider("p3", []string{"op3"})

	composite := NewCompositeOperationProvider(p1, p2, p3)

	assert.Len(t, composite.providers, 3, "should accept variadic providers")
}

// --- GetOperation tests ---

func TestCompositeOperationProvider_GetOperation_HappyPath(t *testing.T) {
	tests := []struct {
		name          string
		providers     []ports.OperationProvider
		operationName string
		shouldFind    bool
		expectedDesc  string
	}{
		{
			name: "operation_in_first_provider",
			providers: []ports.OperationProvider{
				newMockProvider("p1", []string{"github.get_issue", "github.create_pr"}),
				newMockProvider("p2", []string{"notify.send"}),
			},
			operationName: "github.get_issue",
			shouldFind:    true,
			expectedDesc:  "Operation: github.get_issue",
		},
		{
			name: "operation_in_second_provider",
			providers: []ports.OperationProvider{
				newMockProvider("p1", []string{"github.get_issue"}),
				newMockProvider("p2", []string{"notify.send"}),
			},
			operationName: "notify.send",
			shouldFind:    true,
			expectedDesc:  "Operation: notify.send",
		},
		{
			name: "operation_in_last_provider",
			providers: []ports.OperationProvider{
				newMockProvider("p1", []string{"op1"}),
				newMockProvider("p2", []string{"op2"}),
				newMockProvider("p3", []string{"op3"}),
			},
			operationName: "op3",
			shouldFind:    true,
			expectedDesc:  "Operation: op3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composite := NewCompositeOperationProvider(tt.providers...)

			op, found := composite.GetOperation(tt.operationName)

			if tt.shouldFind {
				assert.True(t, found, "operation %s should be found", tt.operationName)
				require.NotNil(t, op, "operation schema should not be nil")
				assert.Equal(t, tt.operationName, op.Name, "operation name should match")
				assert.Equal(t, tt.expectedDesc, op.Description, "operation description should match")
			} else {
				assert.False(t, found, "operation %s should not be found", tt.operationName)
				assert.Nil(t, op, "operation schema should be nil when not found")
			}
		})
	}
}

func TestCompositeOperationProvider_GetOperation_NotFound(t *testing.T) {
	tests := []struct {
		name          string
		providers     []ports.OperationProvider
		operationName string
	}{
		{
			name:          "empty_providers",
			providers:     []ports.OperationProvider{},
			operationName: "any.operation",
		},
		{
			name: "operation_not_in_any_provider",
			providers: []ports.OperationProvider{
				newMockProvider("p1", []string{"github.get_issue"}),
				newMockProvider("p2", []string{"notify.send"}),
			},
			operationName: "slack.send",
		},
		{
			name: "wrong_namespace",
			providers: []ports.OperationProvider{
				newMockProvider("github", []string{"github.get_issue"}),
			},
			operationName: "notify.get_issue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composite := NewCompositeOperationProvider(tt.providers...)

			op, found := composite.GetOperation(tt.operationName)

			assert.False(t, found, "operation %s should not be found", tt.operationName)
			assert.Nil(t, op, "operation schema should be nil when not found")
		})
	}
}

func TestCompositeOperationProvider_GetOperation_FirstProviderWins(t *testing.T) {
	// When multiple providers have the same operation, first provider wins
	p1 := newMockProviderWithOps([]*pluginmodel.OperationSchema{
		{Name: "duplicate.op", Description: "First provider version"},
	})
	p2 := newMockProviderWithOps([]*pluginmodel.OperationSchema{
		{Name: "duplicate.op", Description: "Second provider version"},
	})

	composite := NewCompositeOperationProvider(p1, p2)

	op, found := composite.GetOperation("duplicate.op")

	assert.True(t, found, "operation should be found")
	require.NotNil(t, op, "operation schema should not be nil")
	assert.Equal(t, "First provider version", op.Description, "first provider should win")
}

// --- ListOperations tests ---

func TestCompositeOperationProvider_ListOperations_HappyPath(t *testing.T) {
	tests := []struct {
		name          string
		providers     []ports.OperationProvider
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "empty_providers",
			providers:     []ports.OperationProvider{},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name: "single_provider_single_operation",
			providers: []ports.OperationProvider{
				newMockProvider("p1", []string{"op1"}),
			},
			expectedCount: 1,
			expectedNames: []string{"op1"},
		},
		{
			name: "single_provider_multiple_operations",
			providers: []ports.OperationProvider{
				newMockProvider("p1", []string{"op1", "op2", "op3"}),
			},
			expectedCount: 3,
			expectedNames: []string{"op1", "op2", "op3"},
		},
		{
			name: "multiple_providers_no_overlap",
			providers: []ports.OperationProvider{
				newMockProvider("p1", []string{"github.get_issue", "github.create_pr"}),
				newMockProvider("p2", []string{"notify.send"}),
			},
			expectedCount: 3,
			expectedNames: []string{"github.get_issue", "github.create_pr", "notify.send"},
		},
		{
			name: "github_and_notify_providers",
			providers: []ports.OperationProvider{
				newMockProvider("github", []string{
					"github.get_issue", "github.get_pr", "github.create_pr",
					"github.create_issue", "github.add_labels",
				}),
				newMockProvider("notify", []string{"notify.send"}),
			},
			expectedCount: 6,
			expectedNames: []string{
				"github.get_issue", "github.get_pr", "github.create_pr",
				"github.create_issue", "github.add_labels", "notify.send",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composite := NewCompositeOperationProvider(tt.providers...)

			ops := composite.ListOperations()

			assert.Len(t, ops, tt.expectedCount, "should return expected operation count")

			// Verify all expected operations are present
			opNames := make([]string, len(ops))
			for i, op := range ops {
				opNames[i] = op.Name
			}
			for _, expectedName := range tt.expectedNames {
				assert.Contains(t, opNames, expectedName, "should contain operation %s", expectedName)
			}
		})
	}
}

func TestCompositeOperationProvider_ListOperations_WithDuplicates(t *testing.T) {
	// When multiple providers have the same operation, all are included
	// (deduplication is caller's responsibility if needed)
	p1 := newMockProviderWithOps([]*pluginmodel.OperationSchema{
		{Name: "duplicate.op", Description: "First provider version"},
		{Name: "unique1.op", Description: "Unique to p1"},
	})
	p2 := newMockProviderWithOps([]*pluginmodel.OperationSchema{
		{Name: "duplicate.op", Description: "Second provider version"},
		{Name: "unique2.op", Description: "Unique to p2"},
	})

	composite := NewCompositeOperationProvider(p1, p2)

	ops := composite.ListOperations()

	// Should return all operations from both providers (4 total, including duplicate)
	assert.Len(t, ops, 4, "should return all operations including duplicates")

	// Verify specific operations
	opNames := make([]string, len(ops))
	for i, op := range ops {
		opNames[i] = op.Name
	}
	assert.Contains(t, opNames, "unique1.op")
	assert.Contains(t, opNames, "unique2.op")

	// Count duplicates
	duplicateCount := 0
	for _, op := range ops {
		if op.Name == "duplicate.op" {
			duplicateCount++
		}
	}
	assert.Equal(t, 2, duplicateCount, "should include duplicate operation from both providers")
}

// --- Execute tests ---

func TestCompositeOperationProvider_Execute_HappyPath(t *testing.T) {
	tests := []struct {
		name           string
		providers      []ports.OperationProvider
		operationName  string
		inputs         map[string]any
		expectedOutput map[string]any
		expectedErr    string
	}{
		{
			name: "operation_in_first_provider",
			providers: []ports.OperationProvider{
				newMockProviderWithExecutor("p1", []string{"github.get_issue"},
					func(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
						return &pluginmodel.OperationResult{
							Success: true,
							Outputs: map[string]any{"issue_number": 123},
						}, nil
					}),
				newMockProvider("p2", []string{"notify.send"}),
			},
			operationName:  "github.get_issue",
			inputs:         map[string]any{"repo": "test/repo", "number": 123},
			expectedOutput: map[string]any{"issue_number": 123},
		},
		{
			name: "operation_in_second_provider",
			providers: []ports.OperationProvider{
				newMockProvider("p1", []string{"github.get_issue"}),
				newMockProviderWithExecutor("p2", []string{"notify.send"},
					func(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
						return &pluginmodel.OperationResult{
							Success: true,
							Outputs: map[string]any{"sent": true},
						}, nil
					}),
			},
			operationName:  "notify.send",
			inputs:         map[string]any{"backend": "desktop", "message": "Done"},
			expectedOutput: map[string]any{"sent": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composite := NewCompositeOperationProvider(tt.providers...)
			ctx := context.Background()

			result, err := composite.Execute(ctx, tt.operationName, tt.inputs)

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.Success)
				assert.Equal(t, tt.expectedOutput, result.Outputs)
			}
		})
	}
}

func TestCompositeOperationProvider_Execute_OperationNotFound(t *testing.T) {
	tests := []struct {
		name          string
		providers     []ports.OperationProvider
		operationName string
		expectedErr   string
	}{
		{
			name:          "empty_providers",
			providers:     []ports.OperationProvider{},
			operationName: "any.operation",
			expectedErr:   "operation not found",
		},
		{
			name: "operation_not_in_any_provider",
			providers: []ports.OperationProvider{
				newMockProvider("p1", []string{"github.get_issue"}),
				newMockProvider("p2", []string{"notify.send"}),
			},
			operationName: "slack.send",
			expectedErr:   "operation not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composite := NewCompositeOperationProvider(tt.providers...)
			ctx := context.Background()

			result, err := composite.Execute(ctx, tt.operationName, map[string]any{})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
			assert.Nil(t, result)
		})
	}
}

func TestCompositeOperationProvider_Execute_ProviderError(t *testing.T) {
	// Provider returns an error during execution
	providerErr := fmt.Errorf("backend connection failed")
	p1 := newMockProviderWithExecutor("p1", []string{"notify.send"},
		func(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
			return nil, providerErr
		})

	composite := NewCompositeOperationProvider(p1)
	ctx := context.Background()

	result, err := composite.Execute(ctx, "notify.send", map[string]any{})

	require.Error(t, err)
	assert.ErrorIs(t, err, providerErr, "should propagate provider error")
	assert.Nil(t, result)
}

func TestCompositeOperationProvider_Execute_ProviderReturnsFailure(t *testing.T) {
	// Provider returns OperationResult with Success=false
	p1 := newMockProviderWithExecutor("p1", []string{"notify.send"},
		func(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
			return &pluginmodel.OperationResult{
				Success: false,
				Error:   "desktop notification unavailable",
			}, nil
		})

	composite := NewCompositeOperationProvider(p1)
	ctx := context.Background()

	result, err := composite.Execute(ctx, "notify.send", map[string]any{})

	require.NoError(t, err, "should not return error when provider returns result")
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "desktop notification unavailable", result.Error)
}

func TestCompositeOperationProvider_Execute_ContextCancellation(t *testing.T) {
	// Verify context is passed to provider
	ctxPassed := false
	p1 := newMockProviderWithExecutor("p1", []string{"notify.send"},
		func(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
			ctxPassed = (ctx != nil)
			return &pluginmodel.OperationResult{Success: true}, nil
		})

	composite := NewCompositeOperationProvider(p1)

	_, err := composite.Execute(t.Context(), "notify.send", map[string]any{})

	require.NoError(t, err)
	assert.True(t, ctxPassed, "context should be passed to provider")
}

// --- Edge case tests ---

func TestCompositeOperationProvider_NilProvider(t *testing.T) {
	// Passing nil provider should not panic (handled gracefully)
	composite := NewCompositeOperationProvider(nil)

	assert.Len(t, composite.providers, 1, "should accept nil provider")

	ops := composite.ListOperations()
	assert.Len(t, ops, 0, "nil provider should contribute no operations")
}

func TestCompositeOperationProvider_EmptyOperationName(t *testing.T) {
	p1 := newMockProvider("p1", []string{"github.get_issue"})
	composite := NewCompositeOperationProvider(p1)

	op, found := composite.GetOperation("")

	assert.False(t, found, "empty operation name should not be found")
	assert.Nil(t, op)
}

// --- Mock provider implementation ---

type mockProvider struct {
	name       string
	operations []*pluginmodel.OperationSchema
	executor   func(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error)
}

func newMockProvider(name string, opNames []string) *mockProvider {
	ops := make([]*pluginmodel.OperationSchema, len(opNames))
	for i, opName := range opNames {
		ops[i] = &pluginmodel.OperationSchema{
			Name:        opName,
			Description: fmt.Sprintf("Operation: %s", opName),
			Inputs:      map[string]pluginmodel.InputSchema{},
			Outputs:     []string{},
			PluginName:  name,
		}
	}
	return &mockProvider{
		name:       name,
		operations: ops,
	}
}

func newMockProviderWithOps(ops []*pluginmodel.OperationSchema) *mockProvider {
	return &mockProvider{
		operations: ops,
	}
}

func newMockProviderWithExecutor(name string, opNames []string,
	executor func(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error),
) *mockProvider {
	provider := newMockProvider(name, opNames)
	provider.executor = executor
	return provider
}

func (m *mockProvider) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	if m == nil || m.operations == nil {
		return nil, false
	}
	for _, op := range m.operations {
		if op.Name == name {
			return op, true
		}
	}
	return nil, false
}

func (m *mockProvider) ListOperations() []*pluginmodel.OperationSchema {
	if m == nil || m.operations == nil {
		return nil
	}
	return m.operations
}

func (m *mockProvider) Execute(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
	if m == nil {
		return nil, fmt.Errorf("nil provider")
	}
	if m.executor != nil {
		return m.executor(ctx, name, inputs)
	}
	// Default: operation not implemented
	return nil, fmt.Errorf("not implemented")
}
