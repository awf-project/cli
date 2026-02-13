package plugin

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
)

// --- Interface compliance tests ---

func TestOperationRegistry_ImplementsInterface(t *testing.T) {
	var _ ports.PluginRegistry = (*OperationRegistry)(nil)
}

// --- Constructor tests ---

func TestNewOperationRegistry(t *testing.T) {
	registry := NewOperationRegistry()

	require.NotNil(t, registry, "NewOperationRegistry() should not return nil")
	assert.NotNil(t, registry.operations, "operations map should be initialized")
	assert.NotNil(t, registry.sources, "sources map should be initialized")
	assert.Empty(t, registry.operations, "operations map should be empty")
	assert.Empty(t, registry.sources, "sources map should be empty")
}

func TestNewOperationRegistry_MultipleInstances(t *testing.T) {
	r1 := NewOperationRegistry()
	r2 := NewOperationRegistry()

	assert.NotSame(t, r1, r2, "each call should return a new instance")
	// Maps can't use NotSame, but we verify they are independent
	// by checking they are both empty and separate instances
	assert.Empty(t, r1.operations)
	assert.Empty(t, r2.operations)
}

// --- RegisterOperation tests ---

func TestOperationRegistry_RegisterOperation_ValidOperation(t *testing.T) {
	registry := NewOperationRegistry()
	op := &plugin.OperationSchema{
		Name:        "slack.send",
		Description: "Send message to Slack",
		PluginName:  "awf-plugin-slack",
	}

	err := registry.RegisterOperation(op)
	require.NoError(t, err)

	// Verify operation was registered
	retrieved, found := registry.GetOperation("slack.send")
	assert.True(t, found, "operation should be found after registration")
	assert.Equal(t, op, retrieved)
}

func TestOperationRegistry_RegisterOperation_NilOperation(t *testing.T) {
	registry := NewOperationRegistry()

	err := registry.RegisterOperation(nil)
	assert.ErrorIs(t, err, ErrInvalidOperation, "should return ErrInvalidOperation for nil operation")
}

func TestOperationRegistry_RegisterOperation_EmptyName(t *testing.T) {
	registry := NewOperationRegistry()
	op := &plugin.OperationSchema{
		Name:        "",
		Description: "Empty name operation",
		PluginName:  "test-plugin",
	}

	err := registry.RegisterOperation(op)
	assert.ErrorIs(t, err, ErrInvalidOperation, "should return ErrInvalidOperation for empty name")
}

func TestOperationRegistry_RegisterOperation_DuplicateName(t *testing.T) {
	registry := NewOperationRegistry()
	op1 := &plugin.OperationSchema{
		Name:        "my.operation",
		Description: "First operation",
		PluginName:  "plugin-a",
	}
	op2 := &plugin.OperationSchema{
		Name:        "my.operation",
		Description: "Duplicate operation",
		PluginName:  "plugin-b",
	}

	err := registry.RegisterOperation(op1)
	require.NoError(t, err, "first registration should succeed")

	err = registry.RegisterOperation(op2)
	assert.ErrorIs(t, err, ErrOperationAlreadyRegistered, "duplicate registration should fail")
}

func TestOperationRegistry_RegisterOperation_MultipleOperations(t *testing.T) {
	registry := NewOperationRegistry()
	operations := []*plugin.OperationSchema{
		{Name: "op.one", Description: "First", PluginName: "plugin-a"},
		{Name: "op.two", Description: "Second", PluginName: "plugin-a"},
		{Name: "op.three", Description: "Third", PluginName: "plugin-b"},
	}

	for _, op := range operations {
		err := registry.RegisterOperation(op)
		require.NoError(t, err, "registering %s should succeed", op.Name)
	}

	assert.Equal(t, 3, registry.Count(), "should have 3 registered operations")
}

func TestOperationRegistry_RegisterOperation_WithInputs(t *testing.T) {
	registry := NewOperationRegistry()
	op := &plugin.OperationSchema{
		Name:        "http.request",
		Description: "Make HTTP request",
		PluginName:  "awf-plugin-http",
		Inputs: map[string]plugin.InputSchema{
			"url": {
				Type:        plugin.InputTypeString,
				Required:    true,
				Description: "Target URL",
			},
			"method": {
				Type:        plugin.InputTypeString,
				Required:    false,
				Default:     "GET",
				Description: "HTTP method",
			},
		},
		Outputs: []string{"status_code", "body"},
	}

	err := registry.RegisterOperation(op)
	require.NoError(t, err)

	retrieved, found := registry.GetOperation("http.request")
	assert.True(t, found)
	assert.Len(t, retrieved.Inputs, 2)
	assert.Len(t, retrieved.Outputs, 2)
}

func TestOperationRegistry_RegisterOperation_SetsSource(t *testing.T) {
	registry := NewOperationRegistry()
	op := &plugin.OperationSchema{
		Name:        "test.operation",
		Description: "Test",
		PluginName:  "my-test-plugin",
	}

	err := registry.RegisterOperation(op)
	require.NoError(t, err)

	source, found := registry.GetOperationSource("test.operation")
	assert.True(t, found)
	assert.Equal(t, "my-test-plugin", source)
}

// --- UnregisterOperation tests ---

func TestOperationRegistry_UnregisterOperation_Existing(t *testing.T) {
	registry := NewOperationRegistry()
	op := &plugin.OperationSchema{
		Name:        "to.remove",
		Description: "Will be removed",
		PluginName:  "test-plugin",
	}

	err := registry.RegisterOperation(op)
	require.NoError(t, err)

	err = registry.UnregisterOperation("to.remove")
	require.NoError(t, err)

	_, found := registry.GetOperation("to.remove")
	assert.False(t, found, "operation should not be found after unregistration")
}

func TestOperationRegistry_UnregisterOperation_NonExistent(t *testing.T) {
	registry := NewOperationRegistry()

	err := registry.UnregisterOperation("nonexistent")
	assert.ErrorIs(t, err, ErrOperationNotFound, "should return ErrOperationNotFound")
}

func TestOperationRegistry_UnregisterOperation_EmptyName(t *testing.T) {
	registry := NewOperationRegistry()

	err := registry.UnregisterOperation("")
	assert.ErrorIs(t, err, ErrOperationNotFound, "empty name should return ErrOperationNotFound")
}

func TestOperationRegistry_UnregisterOperation_ClearsSource(t *testing.T) {
	registry := NewOperationRegistry()
	op := &plugin.OperationSchema{
		Name:        "test.op",
		Description: "Test",
		PluginName:  "my-plugin",
	}

	err := registry.RegisterOperation(op)
	require.NoError(t, err)

	err = registry.UnregisterOperation("test.op")
	require.NoError(t, err)

	_, found := registry.GetOperationSource("test.op")
	assert.False(t, found, "source should be cleared after unregistration")
}

func TestOperationRegistry_UnregisterOperation_AllowsReregistration(t *testing.T) {
	registry := NewOperationRegistry()
	op1 := &plugin.OperationSchema{
		Name:        "reusable.op",
		Description: "First version",
		PluginName:  "plugin-a",
	}

	err := registry.RegisterOperation(op1)
	require.NoError(t, err)

	err = registry.UnregisterOperation("reusable.op")
	require.NoError(t, err)

	// Should be able to register again with different plugin
	op2 := &plugin.OperationSchema{
		Name:        "reusable.op",
		Description: "Second version",
		PluginName:  "plugin-b",
	}
	err = registry.RegisterOperation(op2)
	require.NoError(t, err)

	source, _ := registry.GetOperationSource("reusable.op")
	assert.Equal(t, "plugin-b", source)
}

// --- Operations tests ---

func TestOperationRegistry_Operations_Empty(t *testing.T) {
	registry := NewOperationRegistry()

	ops := registry.Operations()
	assert.Empty(t, ops, "empty registry should return empty slice")
}

func TestOperationRegistry_Operations_ReturnsAll(t *testing.T) {
	registry := NewOperationRegistry()
	operations := []*plugin.OperationSchema{
		{Name: "op.a", PluginName: "plugin-1"},
		{Name: "op.b", PluginName: "plugin-1"},
		{Name: "op.c", PluginName: "plugin-2"},
	}

	for _, op := range operations {
		err := registry.RegisterOperation(op)
		require.NoError(t, err)
	}

	ops := registry.Operations()
	assert.Len(t, ops, 3, "should return all registered operations")

	names := make(map[string]bool)
	for _, op := range ops {
		names[op.Name] = true
	}
	assert.True(t, names["op.a"])
	assert.True(t, names["op.b"])
	assert.True(t, names["op.c"])
}

func TestOperationRegistry_Operations_NotNilEvenWhenEmpty(t *testing.T) {
	registry := NewOperationRegistry()

	ops := registry.Operations()
	// Should return empty slice, not nil (for consistent iteration)
	assert.NotNil(t, ops, "Operations() should return empty slice, not nil")
}

// --- GetOperation tests ---

func TestOperationRegistry_GetOperation_Existing(t *testing.T) {
	registry := NewOperationRegistry()
	op := &plugin.OperationSchema{
		Name:        "find.me",
		Description: "Findable operation",
		PluginName:  "test-plugin",
	}

	err := registry.RegisterOperation(op)
	require.NoError(t, err)

	retrieved, found := registry.GetOperation("find.me")
	assert.True(t, found)
	assert.Equal(t, op, retrieved)
	assert.Equal(t, "find.me", retrieved.Name)
}

func TestOperationRegistry_GetOperation_NonExistent(t *testing.T) {
	registry := NewOperationRegistry()

	retrieved, found := registry.GetOperation("does.not.exist")
	assert.False(t, found)
	assert.Nil(t, retrieved)
}

func TestOperationRegistry_GetOperation_EmptyName(t *testing.T) {
	registry := NewOperationRegistry()

	retrieved, found := registry.GetOperation("")
	assert.False(t, found)
	assert.Nil(t, retrieved)
}

func TestOperationRegistry_GetOperation_CaseSensitive(t *testing.T) {
	registry := NewOperationRegistry()
	op := &plugin.OperationSchema{
		Name:        "My.Operation",
		Description: "Case sensitive",
		PluginName:  "test-plugin",
	}

	err := registry.RegisterOperation(op)
	require.NoError(t, err)

	_, found := registry.GetOperation("my.operation") // lowercase
	assert.False(t, found, "operation names should be case-sensitive")

	_, found = registry.GetOperation("My.Operation") // exact case
	assert.True(t, found)
}

// --- UnregisterPluginOperations tests ---

func TestOperationRegistry_UnregisterPluginOperations_RemovesAll(t *testing.T) {
	registry := NewOperationRegistry()
	operations := []*plugin.OperationSchema{
		{Name: "plugin-a.op1", PluginName: "plugin-a"},
		{Name: "plugin-a.op2", PluginName: "plugin-a"},
		{Name: "plugin-b.op1", PluginName: "plugin-b"},
	}

	for _, op := range operations {
		err := registry.RegisterOperation(op)
		require.NoError(t, err)
	}

	err := registry.UnregisterPluginOperations("plugin-a")
	require.NoError(t, err)

	// plugin-a operations should be gone
	_, found := registry.GetOperation("plugin-a.op1")
	assert.False(t, found, "plugin-a.op1 should be removed")
	_, found = registry.GetOperation("plugin-a.op2")
	assert.False(t, found, "plugin-a.op2 should be removed")

	// plugin-b operations should remain
	_, found = registry.GetOperation("plugin-b.op1")
	assert.True(t, found, "plugin-b.op1 should remain")
}

func TestOperationRegistry_UnregisterPluginOperations_NonExistentPlugin(t *testing.T) {
	registry := NewOperationRegistry()

	err := registry.UnregisterPluginOperations("nonexistent-plugin")
	// Should succeed even if plugin has no operations (no-op)
	assert.NoError(t, err, "unregistering non-existent plugin should succeed silently")
}

func TestOperationRegistry_UnregisterPluginOperations_EmptyPluginName(t *testing.T) {
	registry := NewOperationRegistry()

	_ = registry.UnregisterPluginOperations("")
	// Empty plugin name should be handled gracefully (no-op or error)
	// Implementation decides the behavior
}

func TestOperationRegistry_UnregisterPluginOperations_PartialRemoval(t *testing.T) {
	registry := NewOperationRegistry()

	// Register operations from multiple plugins
	for i := 0; i < 5; i++ {
		opA := &plugin.OperationSchema{
			Name:       fmt.Sprintf("plugin-a.op%d", i),
			PluginName: "plugin-a",
		}
		opB := &plugin.OperationSchema{
			Name:       fmt.Sprintf("plugin-b.op%d", i),
			PluginName: "plugin-b",
		}
		err := registry.RegisterOperation(opA)
		require.NoError(t, err)
		err = registry.RegisterOperation(opB)
		require.NoError(t, err)
	}

	assert.Equal(t, 10, registry.Count(), "should have 10 operations")

	err := registry.UnregisterPluginOperations("plugin-a")
	require.NoError(t, err)

	assert.Equal(t, 5, registry.Count(), "should have 5 operations after removal")

	// All remaining should be from plugin-b
	ops := registry.GetPluginOperations("plugin-b")
	assert.Len(t, ops, 5)
}

// --- GetPluginOperations tests ---

func TestOperationRegistry_GetPluginOperations_ReturnsMatching(t *testing.T) {
	registry := NewOperationRegistry()
	operations := []*plugin.OperationSchema{
		{Name: "target.op1", PluginName: "target-plugin"},
		{Name: "target.op2", PluginName: "target-plugin"},
		{Name: "other.op1", PluginName: "other-plugin"},
	}

	for _, op := range operations {
		err := registry.RegisterOperation(op)
		require.NoError(t, err)
	}

	ops := registry.GetPluginOperations("target-plugin")
	assert.Len(t, ops, 2, "should return 2 operations for target-plugin")

	names := make(map[string]bool)
	for _, op := range ops {
		names[op.Name] = true
	}
	assert.True(t, names["target.op1"])
	assert.True(t, names["target.op2"])
}

func TestOperationRegistry_GetPluginOperations_NonExistentPlugin(t *testing.T) {
	registry := NewOperationRegistry()

	ops := registry.GetPluginOperations("nonexistent")
	assert.Empty(t, ops, "should return empty slice for non-existent plugin")
}

func TestOperationRegistry_GetPluginOperations_EmptyPluginName(t *testing.T) {
	registry := NewOperationRegistry()

	ops := registry.GetPluginOperations("")
	assert.Empty(t, ops, "should return empty slice for empty plugin name")
}

// --- Count tests ---

func TestOperationRegistry_Count_Empty(t *testing.T) {
	registry := NewOperationRegistry()

	assert.Equal(t, 0, registry.Count())
}

func TestOperationRegistry_Count_AfterRegistration(t *testing.T) {
	registry := NewOperationRegistry()

	for i := 0; i < 5; i++ {
		op := &plugin.OperationSchema{
			Name:       fmt.Sprintf("op.%d", i),
			PluginName: "test-plugin",
		}
		err := registry.RegisterOperation(op)
		require.NoError(t, err)
	}

	assert.Equal(t, 5, registry.Count())
}

func TestOperationRegistry_Count_AfterUnregistration(t *testing.T) {
	registry := NewOperationRegistry()

	for i := 0; i < 5; i++ {
		op := &plugin.OperationSchema{
			Name:       fmt.Sprintf("op.%d", i),
			PluginName: "test-plugin",
		}
		err := registry.RegisterOperation(op)
		require.NoError(t, err)
	}

	err := registry.UnregisterOperation("op.2")
	require.NoError(t, err)

	assert.Equal(t, 4, registry.Count())
}

// --- GetOperationSource tests ---

func TestOperationRegistry_GetOperationSource_Existing(t *testing.T) {
	registry := NewOperationRegistry()
	op := &plugin.OperationSchema{
		Name:        "sourced.op",
		Description: "Has known source",
		PluginName:  "source-plugin",
	}

	err := registry.RegisterOperation(op)
	require.NoError(t, err)

	source, found := registry.GetOperationSource("sourced.op")
	assert.True(t, found)
	assert.Equal(t, "source-plugin", source)
}

func TestOperationRegistry_GetOperationSource_NonExistent(t *testing.T) {
	registry := NewOperationRegistry()

	source, found := registry.GetOperationSource("nonexistent")
	assert.False(t, found)
	assert.Empty(t, source)
}

// --- Clear tests ---

func TestOperationRegistry_Clear_EmptyRegistry(t *testing.T) {
	registry := NewOperationRegistry()

	// Should not panic on empty registry
	registry.Clear()

	assert.Equal(t, 0, registry.Count())
}

func TestOperationRegistry_Clear_PopulatedRegistry(t *testing.T) {
	registry := NewOperationRegistry()

	for i := 0; i < 5; i++ {
		op := &plugin.OperationSchema{
			Name:       fmt.Sprintf("op.%d", i),
			PluginName: "test-plugin",
		}
		err := registry.RegisterOperation(op)
		require.NoError(t, err)
	}

	registry.Clear()

	assert.Equal(t, 0, registry.Count(), "count should be 0 after clear")
	ops := registry.Operations()
	assert.Empty(t, ops, "operations should be empty after clear")
}

func TestOperationRegistry_Clear_AllowsReregistration(t *testing.T) {
	registry := NewOperationRegistry()
	op := &plugin.OperationSchema{
		Name:        "clearable.op",
		Description: "Can be re-registered",
		PluginName:  "test-plugin",
	}

	err := registry.RegisterOperation(op)
	require.NoError(t, err)

	registry.Clear()

	// Should be able to register same operation again
	err = registry.RegisterOperation(op)
	assert.NoError(t, err, "should be able to register after clear")
}

// --- Concurrency tests ---

func TestOperationRegistry_ConcurrentRegister(t *testing.T) {
	t.Parallel()

	registry := NewOperationRegistry()
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			op := &plugin.OperationSchema{
				Name:       fmt.Sprintf("concurrent.op.%d", n),
				PluginName: fmt.Sprintf("plugin-%d", n%5),
			}
			_ = registry.RegisterOperation(op)
		}(i)
	}

	wg.Wait()

	// All operations should be registered (no race conditions)
	count := registry.Count()
	assert.Equal(t, goroutines, count, "all concurrent registrations should succeed")
}

func TestOperationRegistry_ConcurrentUnregister(t *testing.T) {
	t.Parallel()

	registry := NewOperationRegistry()
	const operations = 50

	// Pre-register operations
	for i := 0; i < operations; i++ {
		op := &plugin.OperationSchema{
			Name:       fmt.Sprintf("to.unregister.%d", i),
			PluginName: "test-plugin",
		}
		err := registry.RegisterOperation(op)
		require.NoError(t, err)
	}

	var wg sync.WaitGroup
	wg.Add(operations)

	for i := 0; i < operations; i++ {
		go func(n int) {
			defer wg.Done()
			_ = registry.UnregisterOperation(fmt.Sprintf("to.unregister.%d", n))
		}(i)
	}

	wg.Wait()

	assert.Equal(t, 0, registry.Count(), "all operations should be unregistered")
}

func TestOperationRegistry_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()

	registry := NewOperationRegistry()
	const iterations = 100
	var wg sync.WaitGroup

	// Writers (register)
	wg.Add(iterations / 2)
	for i := 0; i < iterations/2; i++ {
		go func(n int) {
			defer wg.Done()
			op := &plugin.OperationSchema{
				Name:       fmt.Sprintf("readwrite.op.%d", n),
				PluginName: "test-plugin",
			}
			_ = registry.RegisterOperation(op)
		}(i)
	}

	// Readers
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func(n int) {
			defer wg.Done()
			_ = registry.Operations()
			_ = registry.Count()
			_, _ = registry.GetOperation(fmt.Sprintf("readwrite.op.%d", n%50))
			_, _ = registry.GetOperationSource(fmt.Sprintf("readwrite.op.%d", n%50))
			_ = registry.GetPluginOperations("test-plugin")
		}(i)
	}

	wg.Wait()
	// Should not panic - mutex protects concurrent access
}

func TestOperationRegistry_ConcurrentPluginOperations(t *testing.T) {
	t.Parallel()

	registry := NewOperationRegistry()
	const plugins = 5
	const opsPerPlugin = 10

	// Register operations for multiple plugins
	for p := 0; p < plugins; p++ {
		for o := 0; o < opsPerPlugin; o++ {
			op := &plugin.OperationSchema{
				Name:       fmt.Sprintf("plugin-%d.op-%d", p, o),
				PluginName: fmt.Sprintf("plugin-%d", p),
			}
			err := registry.RegisterOperation(op)
			require.NoError(t, err)
		}
	}

	var wg sync.WaitGroup

	// Concurrent plugin removal
	wg.Add(plugins)
	for p := 0; p < plugins; p++ {
		go func(pluginNum int) {
			defer wg.Done()
			_ = registry.UnregisterPluginOperations(fmt.Sprintf("plugin-%d", pluginNum))
		}(p)
	}

	wg.Wait()

	assert.Equal(t, 0, registry.Count(), "all plugin operations should be removed")
}

// --- Edge case tests ---

func TestOperationRegistry_SpecialCharactersInName(t *testing.T) {
	tests := []struct {
		name        string
		opName      string
		shouldError bool
	}{
		{"dotted name", "plugin.operation.sub", false},
		{"hyphenated", "my-operation", false},
		{"underscored", "my_operation", false},
		{"mixed", "plugin.my-operation_v2", false},
		{"numbers", "op123", false},
		{"unicode", "操作", false}, // Chinese characters
		{"emoji", "🔧.operation", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewOperationRegistry()
			op := &plugin.OperationSchema{
				Name:        tt.opName,
				Description: "Test",
				PluginName:  "test-plugin",
			}

			err := registry.RegisterOperation(op)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				retrieved, found := registry.GetOperation(tt.opName)
				assert.True(t, found)
				assert.Equal(t, tt.opName, retrieved.Name)
			}
		})
	}
}

func TestOperationRegistry_LongOperationName(t *testing.T) {
	registry := NewOperationRegistry()

	// Very long operation name
	longName := "this.is.a.very.long.operation.name.that.might.cause.issues.with.certain.implementations.but.should.work.fine"
	op := &plugin.OperationSchema{
		Name:        longName,
		Description: "Long name operation",
		PluginName:  "test-plugin",
	}

	err := registry.RegisterOperation(op)
	assert.NoError(t, err)

	retrieved, found := registry.GetOperation(longName)
	assert.True(t, found)
	assert.Equal(t, longName, retrieved.Name)
}

// --- Error sentinel tests ---

func TestRegistryErrors(t *testing.T) {
	t.Run("ErrOperationAlreadyRegistered", func(t *testing.T) {
		assert.Error(t, ErrOperationAlreadyRegistered)
		assert.Contains(t, ErrOperationAlreadyRegistered.Error(), "already registered")
	})

	t.Run("ErrOperationNotFound", func(t *testing.T) {
		assert.Error(t, ErrOperationNotFound)
		assert.Contains(t, ErrOperationNotFound.Error(), "not found")
	})

	t.Run("ErrInvalidOperation", func(t *testing.T) {
		assert.Error(t, ErrInvalidOperation)
		assert.Contains(t, ErrInvalidOperation.Error(), "invalid")
	})
}

// --- Table-driven comprehensive test ---

func TestOperationRegistry_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		operations []*plugin.OperationSchema
		query      string
		wantFound  bool
	}{
		{
			name:       "empty registry",
			operations: nil,
			query:      "any.op",
			wantFound:  false,
		},
		{
			name: "single operation - found",
			operations: []*plugin.OperationSchema{
				{Name: "single.op", PluginName: "plugin"},
			},
			query:     "single.op",
			wantFound: true,
		},
		{
			name: "single operation - not found",
			operations: []*plugin.OperationSchema{
				{Name: "single.op", PluginName: "plugin"},
			},
			query:     "other.op",
			wantFound: false,
		},
		{
			name: "multiple operations - found",
			operations: []*plugin.OperationSchema{
				{Name: "op.one", PluginName: "plugin-a"},
				{Name: "op.two", PluginName: "plugin-a"},
				{Name: "op.three", PluginName: "plugin-b"},
			},
			query:     "op.two",
			wantFound: true,
		},
		{
			name: "multiple operations - not found",
			operations: []*plugin.OperationSchema{
				{Name: "op.one", PluginName: "plugin-a"},
				{Name: "op.two", PluginName: "plugin-a"},
			},
			query:     "op.three",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewOperationRegistry()

			for _, op := range tt.operations {
				err := registry.RegisterOperation(op)
				require.NoError(t, err)
			}

			_, found := registry.GetOperation(tt.query)
			assert.Equal(t, tt.wantFound, found)
		})
	}
}
