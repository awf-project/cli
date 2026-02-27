package notify

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock Logger ---

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...any)             {}
func (m *mockLogger) Info(msg string, fields ...any)              {}
func (m *mockLogger) Warn(msg string, fields ...any)              {}
func (m *mockLogger) Error(msg string, fields ...any)             {}
func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger { return m }

// --- Interface compliance tests ---

func TestNotifyOperationProvider_ImplementsInterface(t *testing.T) {
	var _ ports.OperationProvider = (*NotifyOperationProvider)(nil)
}

// --- Constructor tests ---

func TestNewNotifyOperationProvider_RegistersAllOperations(t *testing.T) {
	logger := &mockLogger{}

	provider := NewNotifyOperationProvider(logger)

	expectedOps := AllOperations()
	assert.Len(t, provider.operations, len(expectedOps), "should register all operations")

	for _, expectedOp := range expectedOps {
		registeredOp, found := provider.operations[expectedOp.Name]
		require.True(t, found, "operation %s should be registered", expectedOp.Name)
		assert.Equal(t, expectedOp.Name, registeredOp.Name, "operation name should match")
		assert.Equal(t, expectedOp.Description, registeredOp.Description, "operation description should match")
	}
}

func TestNewNotifyOperationProvider_InitializesEmptyBackendsMap(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	require.NotNil(t, provider, "NewNotifyOperationProvider() should not return nil")
	assert.NotNil(t, provider.operations, "operations registry should be initialized")
	assert.NotNil(t, provider.backends, "backends map should be initialized")
	assert.Empty(t, provider.backends, "backends map should be empty initially")
}

// --- GetOperation tests ---

func TestNotifyOperationProvider_GetOperation_NotifySend(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	op, found := provider.GetOperation("notify.send")

	require.True(t, found, "notify.send should be found")
	require.NotNil(t, op, "operation should not be nil")
	assert.Equal(t, "notify.send", op.Name)
	assert.Contains(t, op.Description, "notification")
	assert.NotEmpty(t, op.Inputs, "should have inputs defined")
	assert.NotEmpty(t, op.Outputs, "should have outputs defined")
}

func TestNotifyOperationProvider_GetOperation_NonExistent(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	tests := []struct {
		name          string
		operationName string
	}{
		{"unknown_operation", "notify.unknown"},
		{"empty_name", ""},
		{"wrong_namespace", "github.create_pr"},
		{"partial_match", "notify"},
		{"case_mismatch", "Notify.Send"},
		{"typo", "notify.sendd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, found := provider.GetOperation(tt.operationName)

			assert.False(t, found, "operation %s should not be found", tt.operationName)
			assert.Nil(t, op, "operation should be nil when not found")
		})
	}
}

func TestNotifyOperationProvider_GetOperation_RequiredInputs(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	op, found := provider.GetOperation("notify.send")

	require.True(t, found)
	require.NotNil(t, op)

	// Verify required inputs
	requiredInputs := []string{"backend", "message"}
	for _, inputName := range requiredInputs {
		input, exists := op.Inputs[inputName]
		require.True(t, exists, "required input %s should exist", inputName)
		assert.True(t, input.Required, "input %s should be required", inputName)
	}

	// Verify optional inputs
	optionalInputs := []string{"title", "priority", "webhook_url"}
	for _, inputName := range optionalInputs {
		input, exists := op.Inputs[inputName]
		require.True(t, exists, "optional input %s should exist", inputName)
		assert.False(t, input.Required, "input %s should be optional", inputName)
	}
}

// --- ListOperations tests ---

func TestNotifyOperationProvider_ListOperations_ReturnsAllOperations(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	operations := provider.ListOperations()

	expectedOps := AllOperations()
	assert.Len(t, operations, len(expectedOps), "should return all registered operations")

	// Verify each operation is present
	operationNames := make(map[string]bool)
	for _, op := range operations {
		operationNames[op.Name] = true
	}

	for _, expectedOp := range expectedOps {
		assert.True(t, operationNames[expectedOp.Name], "operation %s should be in list", expectedOp.Name)
	}
}

func TestNotifyOperationProvider_ListOperations_NotNilEvenWhenEmpty(t *testing.T) {
	// Create provider with no operations (hypothetical edge case)
	provider := &NotifyOperationProvider{
		operations: make(map[string]*pluginmodel.OperationSchema),
	}

	operations := provider.ListOperations()

	assert.NotNil(t, operations, "ListOperations() should never return nil")
	assert.Empty(t, operations, "should return empty slice when no operations")
}

func TestNotifyOperationProvider_ListOperations_MultipleInvocations(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	ops1 := provider.ListOperations()
	ops2 := provider.ListOperations()

	assert.Len(t, ops1, len(ops2), "should return same count on multiple invocations")
}

// --- Execute tests ---
// See provider_execute_test.go for comprehensive Execute method tests
