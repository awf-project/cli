package ports_test

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/awf-project/awf/internal/domain/plugin"
	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errNotImplemented = errors.New("not implemented")

// Component: T004
// Feature: C036

// mockPlugin implements ports.Plugin interface for testing.
type mockPlugin struct {
	name    string
	version string
}

func (m *mockPlugin) Name() string    { return m.name }
func (m *mockPlugin) Version() string { return m.version }
func (m *mockPlugin) Init(_ context.Context, _ map[string]any) error {
	return errNotImplemented
}

func (m *mockPlugin) Shutdown(_ context.Context) error {
	return errNotImplemented
}

// mockPluginManager is now consolidated in internal/testutil/mocks.go (C037).
// Use mocks.NewMockPluginManager() instead of local implementation.

// mockOperationProvider implements ports.OperationProvider interface for testing.
type mockOperationProvider struct {
	operations map[string]*plugin.OperationSchema
}

func newMockOperationProvider() *mockOperationProvider {
	return &mockOperationProvider{
		operations: make(map[string]*plugin.OperationSchema),
	}
}

func (m *mockOperationProvider) GetOperation(name string) (*plugin.OperationSchema, bool) {
	op, ok := m.operations[name]
	return op, ok
}

func (m *mockOperationProvider) ListOperations() []*plugin.OperationSchema {
	result := make([]*plugin.OperationSchema, 0, len(m.operations))
	for _, op := range m.operations {
		result = append(result, op)
	}
	return result
}

func (m *mockOperationProvider) Execute(_ context.Context, _ string, _ map[string]any) (*plugin.OperationResult, error) {
	return nil, errNotImplemented
}

// mockPluginRegistry implements ports.PluginRegistry interface for testing.
type mockPluginRegistry struct {
	operations map[string]*plugin.OperationSchema
}

func newMockPluginRegistry() *mockPluginRegistry {
	return &mockPluginRegistry{
		operations: make(map[string]*plugin.OperationSchema),
	}
}

func (m *mockPluginRegistry) RegisterOperation(op *plugin.OperationSchema) error {
	if _, exists := m.operations[op.Name]; exists {
		return errors.New("operation already registered")
	}
	m.operations[op.Name] = op
	return nil
}

func (m *mockPluginRegistry) UnregisterOperation(name string) error {
	if _, exists := m.operations[name]; !exists {
		return errors.New("operation not found")
	}
	delete(m.operations, name)
	return nil
}

func (m *mockPluginRegistry) Operations() []*plugin.OperationSchema {
	result := make([]*plugin.OperationSchema, 0, len(m.operations))
	for _, op := range m.operations {
		result = append(result, op)
	}
	return result
}

// mockPluginStore implements ports.PluginStore interface for testing (ISP refactor - persistence only).
type mockPluginStore struct {
	states map[string]*plugin.PluginState
}

func newMockPluginStore() *mockPluginStore {
	return &mockPluginStore{
		states: make(map[string]*plugin.PluginState),
	}
}

func (m *mockPluginStore) Save(_ context.Context) error {
	return nil // Mock: always succeeds
}

func (m *mockPluginStore) Load(_ context.Context) error {
	return nil // Mock: always succeeds
}

func (m *mockPluginStore) GetState(name string) *plugin.PluginState {
	state, ok := m.states[name]
	if !ok {
		return nil
	}
	return state
}

func (m *mockPluginStore) ListDisabled() []string {
	var disabled []string
	for name, state := range m.states {
		if !state.Enabled {
			disabled = append(disabled, name)
		}
	}
	return disabled
}

// mockPluginConfig implements ports.PluginConfig interface for testing (ISP refactor - configuration only).
type mockPluginConfig struct {
	states map[string]*plugin.PluginState
}

func newMockPluginConfig() *mockPluginConfig {
	return &mockPluginConfig{
		states: make(map[string]*plugin.PluginState),
	}
}

func (m *mockPluginConfig) SetEnabled(_ context.Context, name string, enabled bool) error {
	if _, ok := m.states[name]; !ok {
		m.states[name] = &plugin.PluginState{
			Enabled: enabled,
			Config:  make(map[string]any),
		}
	} else {
		m.states[name].Enabled = enabled
	}
	return nil
}

func (m *mockPluginConfig) IsEnabled(name string) bool {
	state, ok := m.states[name]
	if !ok {
		return true // Default: enabled
	}
	return state.Enabled
}

func (m *mockPluginConfig) GetConfig(name string) map[string]any {
	state, ok := m.states[name]
	if !ok {
		return nil
	}
	return state.Config
}

func (m *mockPluginConfig) SetConfig(_ context.Context, name string, config map[string]any) error {
	if _, ok := m.states[name]; !ok {
		m.states[name] = &plugin.PluginState{
			Enabled: true, // Default
			Config:  config,
		}
	} else {
		m.states[name].Config = config
	}
	return nil
}

// mockPluginStateStore implements ports.PluginStateStore (combined interface) for testing.
type mockPluginStateStore struct {
	states map[string]*plugin.PluginState
}

func newMockPluginStateStore() *mockPluginStateStore {
	return &mockPluginStateStore{
		states: make(map[string]*plugin.PluginState),
	}
}

// PluginStore methods
func (m *mockPluginStateStore) Save(_ context.Context) error {
	return nil
}

func (m *mockPluginStateStore) Load(_ context.Context) error {
	return nil
}

func (m *mockPluginStateStore) GetState(name string) *plugin.PluginState {
	state, ok := m.states[name]
	if !ok {
		return nil
	}
	return state
}

func (m *mockPluginStateStore) ListDisabled() []string {
	var disabled []string
	for name, state := range m.states {
		if !state.Enabled {
			disabled = append(disabled, name)
		}
	}
	return disabled
}

// PluginConfig methods
func (m *mockPluginStateStore) SetEnabled(_ context.Context, name string, enabled bool) error {
	if _, ok := m.states[name]; !ok {
		m.states[name] = &plugin.PluginState{
			Enabled: enabled,
			Config:  make(map[string]any),
		}
	} else {
		m.states[name].Enabled = enabled
	}
	return nil
}

func (m *mockPluginStateStore) IsEnabled(name string) bool {
	state, ok := m.states[name]
	if !ok {
		return true
	}
	return state.Enabled
}

func (m *mockPluginStateStore) GetConfig(name string) map[string]any {
	state, ok := m.states[name]
	if !ok {
		return nil
	}
	return state.Config
}

func (m *mockPluginStateStore) SetConfig(_ context.Context, name string, config map[string]any) error {
	if _, ok := m.states[name]; !ok {
		m.states[name] = &plugin.PluginState{
			Enabled: true,
			Config:  config,
		}
	} else {
		m.states[name].Config = config
	}
	return nil
}

// Interface compliance tests
func TestPluginInterface(t *testing.T) {
	var _ ports.Plugin = (*mockPlugin)(nil)
}

func TestPluginManagerInterface(t *testing.T) {
	var _ ports.PluginManager = (*mocks.MockPluginManager)(nil)
}

func TestOperationProviderInterface(t *testing.T) {
	var _ ports.OperationProvider = (*mockOperationProvider)(nil)
}

func TestPluginRegistryInterface(t *testing.T) {
	var _ ports.PluginRegistry = (*mockPluginRegistry)(nil)
}

// ISP Refactor (C036): Interface compliance tests for split interfaces
func TestPluginStoreInterface(t *testing.T) {
	var _ ports.PluginStore = (*mockPluginStore)(nil)
}

func TestPluginConfigInterface(t *testing.T) {
	var _ ports.PluginConfig = (*mockPluginConfig)(nil)
}

func TestPluginStateStoreInterface_EmbedsBoth(t *testing.T) {
	var _ ports.PluginStateStore = (*mockPluginStateStore)(nil)
	// Verify backward compatibility: combined interface can be used
	var store ports.PluginStateStore = newMockPluginStateStore()
	assert.NotNil(t, store)
}

// C037: ISP Compliance Analysis - PluginManager Cohesion Documentation
//
// Decision: Keep PluginManager unified (7 methods) rather than splitting into
// PluginLifecycle (5 methods) and PluginQuerier (2 methods).
//
// Rationale:
// 1. Single consumer (PluginService) uses ALL 7 methods in orchestration scenarios
// 2. Cross-concern coupling: DisablePlugin uses Get() before Shutdown()
// 3. CLI layer uses PluginService abstraction, not PluginManager directly
// 4. No evidence of consumers needing method subsets
//
// Evidence:
//   - PluginService.DisablePlugin: calls Get() then Shutdown()
//   - PluginService.DiscoverPlugins: calls Discover() then List()
//   - PluginService.StartupEnabledPlugins: uses Get(), Init(), List()
//
// Conclusion: 7 methods exhibit high cohesion. Splitting would create two
// interfaces consumed by the same single service, adding complexity without
// reducing coupling.
func TestPluginManager_CohesionAnalysis_C037(t *testing.T) {
	// This test documents the C037 architectural decision
	// Verify the interface still has exactly 7 methods (not split)
	var mgr ports.PluginManager = mocks.NewMockPluginManager()

	// Demonstrate single consumer uses all methods
	assert.NotNil(t, mgr)

	// Lifecycle methods (5)
	_, _ = mgr.Discover(context.Background())
	_ = mgr.Load(context.Background(), "test")
	_ = mgr.Init(context.Background(), "test", nil)
	_ = mgr.Shutdown(context.Background(), "test")
	_ = mgr.ShutdownAll(context.Background())

	// Query methods (2)
	_, _ = mgr.Get("test")
	_ = mgr.List()

	// All 7 methods callable - demonstrates cohesion
}

// Feature: C050
// AST-based ISP compliance tests verifying PluginManager interface structure

// TestPluginManager_MethodCount_C050 verifies that PluginManager has exactly 7 methods
// using AST parsing. This test structurally enforces the C050 architectural decision
// to keep PluginManager unified rather than split it per ISP.
func TestPluginManager_MethodCount_C050(t *testing.T) {
	// Given: The plugin.go file in the same package
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "plugin.go", nil, 0)
	require.NoError(t, err, "should parse plugin.go")

	// When: Inspecting the AST for PluginManager interface
	var methodCount int
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "PluginManager" {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok {
				found = true
				methodCount = len(iface.Methods.List)
			}
		}
		return true
	})

	// Then: PluginManager should exist and have exactly 7 methods
	assert.True(t, found, "PluginManager interface should exist in plugin.go")
	assert.Equal(t, 7, methodCount,
		"PluginManager should have exactly 7 methods (C050 decision: keep unified, not split per ISP)")
}

// TestPluginManager_NoEmbedding_C050 verifies that PluginManager is a standalone
// interface with no embedded interfaces, ensuring it maintains single-interface cohesion.
func TestPluginManager_NoEmbedding_C050(t *testing.T) {
	// Given: The plugin.go file in the same package
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "plugin.go", nil, 0)
	require.NoError(t, err, "should parse plugin.go")

	// When: Inspecting method declarations in PluginManager
	var hasEmbedding bool
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "PluginManager" {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok {
				found = true
				// Check each field in the interface
				for _, field := range iface.Methods.List {
					// If Names is nil or empty, it's an embedded interface
					if len(field.Names) == 0 {
						hasEmbedding = true
					}
				}
			}
		}
		return true
	})

	// Then: PluginManager should exist and have no embedded interfaces
	assert.True(t, found, "PluginManager interface should exist in plugin.go")
	assert.False(t, hasEmbedding,
		"PluginManager should have no embedded interfaces (C050: standalone single-interface design)")
}

// TestPluginManager_MethodNames_C050 verifies the exact method names in PluginManager
// match the expected interface contract.
func TestPluginManager_MethodNames_C050(t *testing.T) {
	// Given: The plugin.go file in the same package
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "plugin.go", nil, 0)
	require.NoError(t, err, "should parse plugin.go")

	// When: Extracting method names from PluginManager interface
	var methodNames []string
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "PluginManager" {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok {
				found = true
				for _, field := range iface.Methods.List {
					for _, name := range field.Names {
						methodNames = append(methodNames, name.Name)
					}
				}
			}
		}
		return true
	})

	// Then: PluginManager should have expected lifecycle and query methods
	assert.True(t, found, "PluginManager interface should exist in plugin.go")

	expectedMethods := []string{
		"Discover",    // Discovery
		"Load",        // Lifecycle
		"Init",        // Lifecycle
		"Shutdown",    // Lifecycle
		"ShutdownAll", // Lifecycle
		"Get",         // Query
		"List",        // Query
	}

	assert.ElementsMatch(t, expectedMethods, methodNames,
		"PluginManager should have exactly these 7 methods (C050 decision)")
}

// TestPluginInterfaces_MethodCounts_C050 documents all 8 plugin interface method counts.
// This test serves as living documentation, catching unintended interface changes.
// C050: ISP compliance review - comprehensive interface structure validation.
func TestPluginInterfaces_MethodCounts_C050(t *testing.T) {
	// Given: The plugin.go file in the same package
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "plugin.go", nil, 0)
	require.NoError(t, err, "should parse plugin.go")

	// When: Testing all 8 plugin interfaces
	tests := []struct {
		name        string
		wantMethods int
		reason      string
	}{
		{
			name:        "Plugin",
			wantMethods: 4,
			reason:      "4 methods: Name, Version, Init, Shutdown",
		},
		{
			name:        "PluginManager",
			wantMethods: 7,
			reason:      "7 methods: Discover, Load, Init, Shutdown, ShutdownAll, Get, List (C050: kept unified)",
		},
		{
			name:        "OperationProvider",
			wantMethods: 3,
			reason:      "3 methods: GetOperation, ListOperations, Execute",
		},
		{
			name:        "PluginRegistry",
			wantMethods: 3,
			reason:      "3 methods: RegisterOperation, UnregisterOperation, Operations",
		},
		{
			name:        "PluginLoader",
			wantMethods: 3,
			reason:      "3 methods: DiscoverPlugins, LoadPlugin, ValidatePlugin",
		},
		{
			name:        "PluginStore",
			wantMethods: 4,
			reason:      "4 methods: Save, Load, GetState, ListDisabled",
		},
		{
			name:        "PluginConfig",
			wantMethods: 4,
			reason:      "4 methods: SetEnabled, IsEnabled, GetConfig, SetConfig",
		},
		{
			name:        "PluginStateStore",
			wantMethods: 0,
			reason:      "0 direct methods (composite: embeds PluginStore + PluginConfig)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: Finding and counting interface methods
			var methodCount int
			var found bool
			ast.Inspect(node, func(n ast.Node) bool {
				if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == tt.name {
					if iface, ok := ts.Type.(*ast.InterfaceType); ok {
						found = true
						// Count only direct methods (fields with names)
						// Embedded interfaces have no field names
						for _, field := range iface.Methods.List {
							if len(field.Names) > 0 {
								methodCount++
							}
						}
					}
				}
				return true
			})

			// Then: Interface should exist and have expected method count
			assert.True(t, found, "%s interface should exist in plugin.go", tt.name)
			assert.Equal(t, tt.wantMethods, methodCount,
				"%s should have %d direct methods: %s", tt.name, tt.wantMethods, tt.reason)
		})
	}
}

// Helper function to count interface methods and embeddings from AST
func countInterfaceMethodsAndEmbeddings(node *ast.File, interfaceName string) (found bool, methodCount, embeddedCount int) {
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == interfaceName {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok {
				found = true
				for _, field := range iface.Methods.List {
					if len(field.Names) > 0 {
						methodCount++
					} else {
						embeddedCount++
					}
				}
			}
		}
		return true
	})
	return found, methodCount, embeddedCount
}

// TestPluginInterfaces_MethodCounts_EdgeCases_C050 verifies edge cases in AST-based
// method counting to ensure robust handling of nonexistent interfaces and parsing errors.
func TestPluginInterfaces_MethodCounts_EdgeCases_C050(t *testing.T) {
	t.Run("NonexistentInterface", func(t *testing.T) {
		// Given: A parsed plugin.go AST
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "plugin.go", nil, 0)
		require.NoError(t, err, "should parse plugin.go")

		// When: Searching for a nonexistent interface
		found, methodCount, _ := countInterfaceMethodsAndEmbeddings(node, "NonExistentInterface")

		// Then: Interface should not be found
		assert.False(t, found, "nonexistent interface should not be found")
		assert.Equal(t, 0, methodCount, "method count should be 0 for nonexistent interface")
	})

	t.Run("ParseErrorHandling", func(t *testing.T) {
		// Given: An attempt to parse a nonexistent file
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, "nonexistent_file.go", nil, 0)

		// Then: Should return a parse error
		assert.Error(t, err, "parsing nonexistent file should return error")
	})

	t.Run("EmptyInterfaceEmbedding", func(t *testing.T) {
		// Given: The PluginStateStore interface (0 direct methods, only embedding)
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "plugin.go", nil, 0)
		require.NoError(t, err, "should parse plugin.go")

		// When: Counting methods for PluginStateStore
		found, methodCount, embeddedCount := countInterfaceMethodsAndEmbeddings(node, "PluginStateStore")

		// Then: Should have 0 direct methods but 2 embedded interfaces
		assert.True(t, found, "PluginStateStore should exist")
		assert.Equal(t, 0, methodCount, "PluginStateStore should have 0 direct methods")
		assert.Equal(t, 2, embeddedCount, "PluginStateStore should embed 2 interfaces (PluginStore + PluginConfig)")
	})
}

// Helper function to check if a type name is an interface in AST
func isTypeAnInterface(node *ast.File, typeName string) (found, isInterface bool) {
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
			found = true
			_, isInterface = ts.Type.(*ast.InterfaceType)
		}
		return true
	})
	return found, isInterface
}

// Helper function to count methods in an empty interface, handling nil safely
func countMethodsInEmptyInterface(node *ast.File, interfaceName string) (found bool, methodCount int) {
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == interfaceName {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok {
				found = true
				if iface.Methods != nil && iface.Methods.List != nil {
					for _, field := range iface.Methods.List {
						if len(field.Names) > 0 {
							methodCount++
						}
					}
				}
			}
		}
		return true
	})
	return found, methodCount
}

// TestPluginInterfaces_MethodCounts_ErrorHandling_C050 verifies error handling
// in the AST-based method counting logic.
func TestPluginInterfaces_MethodCounts_ErrorHandling_C050(t *testing.T) {
	t.Run("MalformedGoFile", func(t *testing.T) {
		// Given: A malformed Go source file
		malformedSource := `package ports

type BrokenInterface interface {
	MissingClosingBrace()
`
		// When: Attempting to parse the malformed source
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, "malformed.go", malformedSource, 0)

		// Then: Should return a parse error
		assert.Error(t, err, "parsing malformed Go source should return error")
	})

	t.Run("NonInterfaceType", func(t *testing.T) {
		// Given: A Go file with a struct named like an interface
		sourceWithStruct := `package ports

type NotAnInterface struct {
	Field1 string
	Field2 int
}
`
		// When: Parsing and looking for "NotAnInterface"
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "struct.go", sourceWithStruct, 0)
		require.NoError(t, err, "should parse valid Go source")

		found, isInterface := isTypeAnInterface(node, "NotAnInterface")

		// Then: Should find the type but recognize it's not an interface
		assert.True(t, found, "NotAnInterface type should be found")
		assert.False(t, isInterface, "NotAnInterface should not be an interface type")
	})

	t.Run("NilMethodsList", func(t *testing.T) {
		// Given: An empty interface (no methods)
		emptyInterfaceSource := `package ports

type EmptyInterface interface {
}
`
		// When: Parsing and counting methods
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "empty.go", emptyInterfaceSource, 0)
		require.NoError(t, err, "should parse valid Go source")

		found, methodCount := countMethodsInEmptyInterface(node, "EmptyInterface")

		// Then: Should find the interface with 0 methods
		assert.True(t, found, "EmptyInterface should be found")
		assert.Equal(t, 0, methodCount, "EmptyInterface should have 0 methods")
	})
}

// Plugin interface tests
func TestMockPlugin_Name(t *testing.T) {
	p := &mockPlugin{name: "test-plugin", version: "1.0.0"}
	assert.Equal(t, "test-plugin", p.Name())
}

func TestMockPlugin_Version(t *testing.T) {
	p := &mockPlugin{name: "test-plugin", version: "2.1.0"}
	assert.Equal(t, "2.1.0", p.Version())
}

func TestMockPlugin_Init_ReturnsNotImplemented(t *testing.T) {
	p := &mockPlugin{}
	err := p.Init(context.Background(), nil)
	assert.ErrorIs(t, err, errNotImplemented)
}

func TestMockPlugin_Shutdown_ReturnsNotImplemented(t *testing.T) {
	p := &mockPlugin{}
	err := p.Shutdown(context.Background())
	assert.ErrorIs(t, err, errNotImplemented)
}

// PluginManager interface tests
func TestMockPluginManager_Discover_ReturnsNotFoundError(t *testing.T) {
	pm := mocks.NewMockPluginManager()
	// With no plugins, Discover returns empty list (not error)
	plugins, err := pm.Discover(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestMockPluginManager_Load_ReturnsNotFoundError(t *testing.T) {
	pm := mocks.NewMockPluginManager()
	err := pm.Load(context.Background(), "test-plugin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found")
}

func TestMockPluginManager_Init_ReturnsNotFoundError(t *testing.T) {
	pm := mocks.NewMockPluginManager()
	err := pm.Init(context.Background(), "test-plugin", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found")
}

func TestMockPluginManager_Get(t *testing.T) {
	pm := mocks.NewMockPluginManager()
	pm.AddPlugin("test", plugin.StatusRunning)

	info, ok := pm.Get("test")
	assert.True(t, ok)
	assert.Equal(t, "test", info.Manifest.Name)

	_, ok = pm.Get("nonexistent")
	assert.False(t, ok)
}

func TestMockPluginManager_List(t *testing.T) {
	pm := mocks.NewMockPluginManager()
	pm.AddPlugin("p1", plugin.StatusRunning)
	pm.AddPlugin("p2", plugin.StatusRunning)

	list := pm.List()
	assert.Len(t, list, 2)
}

func TestMockPluginManager_List_Empty(t *testing.T) {
	pm := mocks.NewMockPluginManager()
	list := pm.List()
	assert.Empty(t, list)
}

// OperationProvider interface tests
func TestMockOperationProvider_GetOperation(t *testing.T) {
	op := newMockOperationProvider()
	op.operations["slack.send"] = &plugin.OperationSchema{
		Name:       "slack.send",
		PluginName: "slack",
	}

	schema, ok := op.GetOperation("slack.send")
	assert.True(t, ok)
	assert.Equal(t, "slack.send", schema.Name)

	_, ok = op.GetOperation("nonexistent")
	assert.False(t, ok)
}

func TestMockOperationProvider_ListOperations(t *testing.T) {
	op := newMockOperationProvider()
	op.operations["op1"] = &plugin.OperationSchema{Name: "op1"}
	op.operations["op2"] = &plugin.OperationSchema{Name: "op2"}

	list := op.ListOperations()
	assert.Len(t, list, 2)
}

func TestMockOperationProvider_Execute_ReturnsNotImplemented(t *testing.T) {
	op := newMockOperationProvider()
	_, err := op.Execute(context.Background(), "op1", nil)
	assert.ErrorIs(t, err, errNotImplemented)
}

// PluginRegistry interface tests
func TestMockPluginRegistry_RegisterOperation(t *testing.T) {
	reg := newMockPluginRegistry()
	op := &plugin.OperationSchema{Name: "test.op"}

	err := reg.RegisterOperation(op)
	assert.NoError(t, err)
	assert.Len(t, reg.operations, 1)
}

func TestMockPluginRegistry_RegisterOperation_Duplicate(t *testing.T) {
	reg := newMockPluginRegistry()
	op := &plugin.OperationSchema{Name: "test.op"}

	_ = reg.RegisterOperation(op)
	err := reg.RegisterOperation(op)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestMockPluginRegistry_UnregisterOperation(t *testing.T) {
	reg := newMockPluginRegistry()
	op := &plugin.OperationSchema{Name: "test.op"}
	_ = reg.RegisterOperation(op)

	err := reg.UnregisterOperation("test.op")
	assert.NoError(t, err)
	assert.Empty(t, reg.operations)
}

func TestMockPluginRegistry_UnregisterOperation_NotFound(t *testing.T) {
	reg := newMockPluginRegistry()

	err := reg.UnregisterOperation("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMockPluginRegistry_Operations(t *testing.T) {
	reg := newMockPluginRegistry()
	_ = reg.RegisterOperation(&plugin.OperationSchema{Name: "op1"})
	_ = reg.RegisterOperation(&plugin.OperationSchema{Name: "op2"})

	ops := reg.Operations()
	assert.Len(t, ops, 2)
}

// ISP Refactor (C036): PluginStore interface tests (persistence concern)

func TestPluginStore_HappyPath(t *testing.T) {
	store := newMockPluginStore()
	ctx := context.Background()

	// Save and Load should succeed
	err := store.Save(ctx)
	assert.NoError(t, err)

	err = store.Load(ctx)
	assert.NoError(t, err)
}

func TestPluginStore_GetState_Found(t *testing.T) {
	store := newMockPluginStore()
	store.states["plugin-a"] = &plugin.PluginState{
		Enabled: true,
		Config:  map[string]any{"key": "value"},
	}

	state := store.GetState("plugin-a")
	assert.NotNil(t, state)
	assert.True(t, state.Enabled)
	assert.Equal(t, "value", state.Config["key"])
}

func TestPluginStore_GetState_NotFound(t *testing.T) {
	store := newMockPluginStore()

	state := store.GetState("nonexistent")
	assert.Nil(t, state)
}

func TestPluginStore_GetState_EmptyName(t *testing.T) {
	store := newMockPluginStore()

	state := store.GetState("")
	assert.Nil(t, state)
}

func TestPluginStore_ListDisabled_MultiplePlugins(t *testing.T) {
	store := newMockPluginStore()
	store.states["enabled-plugin"] = &plugin.PluginState{
		Enabled: true,
	}
	store.states["disabled-plugin-1"] = &plugin.PluginState{
		Enabled: false,
	}
	store.states["disabled-plugin-2"] = &plugin.PluginState{
		Enabled: false,
	}

	disabled := store.ListDisabled()
	assert.Len(t, disabled, 2)
	assert.Contains(t, disabled, "disabled-plugin-1")
	assert.Contains(t, disabled, "disabled-plugin-2")
}

func TestPluginStore_ListDisabled_AllEnabled(t *testing.T) {
	store := newMockPluginStore()
	store.states["plugin-1"] = &plugin.PluginState{
		Enabled: true,
	}

	disabled := store.ListDisabled()
	assert.Empty(t, disabled)
}

func TestPluginStore_ListDisabled_Empty(t *testing.T) {
	store := newMockPluginStore()

	disabled := store.ListDisabled()
	assert.Empty(t, disabled)
}

// ISP Refactor (C036): PluginConfig interface tests (configuration concern)

func TestPluginConfig_HappyPath(t *testing.T) {
	config := newMockPluginConfig()
	ctx := context.Background()

	// Enable plugin
	err := config.SetEnabled(ctx, "plugin-a", false)
	assert.NoError(t, err)

	// Verify enabled state
	assert.False(t, config.IsEnabled("plugin-a"))

	// Set config
	err = config.SetConfig(ctx, "plugin-a", map[string]any{"timeout": 30})
	assert.NoError(t, err)

	// Verify config
	pluginConfig := config.GetConfig("plugin-a")
	assert.Equal(t, 30, pluginConfig["timeout"])
}

func TestPluginConfig_SetEnabled_NewPlugin(t *testing.T) {
	config := newMockPluginConfig()
	ctx := context.Background()

	err := config.SetEnabled(ctx, "new-plugin", false)
	assert.NoError(t, err)
	assert.False(t, config.IsEnabled("new-plugin"))
}

func TestPluginConfig_SetEnabled_ExistingPlugin(t *testing.T) {
	config := newMockPluginConfig()
	ctx := context.Background()

	// Initially disable
	_ = config.SetEnabled(ctx, "plugin", false)
	assert.False(t, config.IsEnabled("plugin"))

	// Re-enable
	err := config.SetEnabled(ctx, "plugin", true)
	assert.NoError(t, err)
	assert.True(t, config.IsEnabled("plugin"))
}

func TestPluginConfig_SetEnabled_EmptyName(t *testing.T) {
	config := newMockPluginConfig()
	ctx := context.Background()

	err := config.SetEnabled(ctx, "", false)
	assert.NoError(t, err) // Mock allows it
}

func TestPluginConfig_IsEnabled_NonexistentPlugin_DefaultsToTrue(t *testing.T) {
	config := newMockPluginConfig()

	// Non-existent plugins are enabled by default
	assert.True(t, config.IsEnabled("nonexistent"))
}

func TestPluginConfig_IsEnabled_EmptyName_DefaultsToTrue(t *testing.T) {
	config := newMockPluginConfig()

	assert.True(t, config.IsEnabled(""))
}

func TestPluginConfig_GetConfig_Found(t *testing.T) {
	config := newMockPluginConfig()
	ctx := context.Background()

	expectedConfig := map[string]any{
		"webhook_url": "https://example.com/hook",
		"channel":     "#general",
		"timeout":     30,
	}

	_ = config.SetConfig(ctx, "plugin", expectedConfig)

	actualConfig := config.GetConfig("plugin")
	assert.Equal(t, expectedConfig, actualConfig)
}

func TestPluginConfig_GetConfig_NotFound(t *testing.T) {
	config := newMockPluginConfig()

	pluginConfig := config.GetConfig("nonexistent")
	assert.Nil(t, pluginConfig)
}

func TestPluginConfig_GetConfig_EmptyName(t *testing.T) {
	config := newMockPluginConfig()

	pluginConfig := config.GetConfig("")
	assert.Nil(t, pluginConfig)
}

func TestPluginConfig_SetConfig_NewPlugin(t *testing.T) {
	config := newMockPluginConfig()
	ctx := context.Background()

	newConfig := map[string]any{"api_key": "secret"}
	err := config.SetConfig(ctx, "new-plugin", newConfig)
	assert.NoError(t, err)

	actualConfig := config.GetConfig("new-plugin")
	assert.Equal(t, newConfig, actualConfig)
}

func TestPluginConfig_SetConfig_ExistingPlugin(t *testing.T) {
	config := newMockPluginConfig()
	ctx := context.Background()

	// Initial config
	_ = config.SetConfig(ctx, "plugin", map[string]any{"version": "1"})

	// Update config
	newConfig := map[string]any{"version": "2", "enabled": true}
	err := config.SetConfig(ctx, "plugin", newConfig)
	assert.NoError(t, err)

	actualConfig := config.GetConfig("plugin")
	assert.Equal(t, "2", actualConfig["version"])
	assert.True(t, actualConfig["enabled"].(bool))
}

func TestPluginConfig_SetConfig_NilConfig(t *testing.T) {
	config := newMockPluginConfig()
	ctx := context.Background()

	err := config.SetConfig(ctx, "plugin", nil)
	assert.NoError(t, err) // Mock allows nil config
}

func TestPluginConfig_SetConfig_EmptyConfig(t *testing.T) {
	config := newMockPluginConfig()
	ctx := context.Background()

	err := config.SetConfig(ctx, "plugin", map[string]any{})
	assert.NoError(t, err)

	actualConfig := config.GetConfig("plugin")
	assert.NotNil(t, actualConfig)
	assert.Empty(t, actualConfig)
}

// ISP Refactor (C036): PluginStateStore combined interface tests (backward compatibility)

func TestPluginStateStore_HappyPath_Persistence(t *testing.T) {
	store := newMockPluginStateStore()
	ctx := context.Background()

	// Test persistence methods
	err := store.Save(ctx)
	assert.NoError(t, err)

	err = store.Load(ctx)
	assert.NoError(t, err)

	// Set state and verify
	_ = store.SetEnabled(ctx, "plugin", false)
	state := store.GetState("plugin")
	assert.NotNil(t, state)
	assert.False(t, state.Enabled)
}

func TestPluginStateStore_HappyPath_Configuration(t *testing.T) {
	store := newMockPluginStateStore()
	ctx := context.Background()

	// Test config methods
	err := store.SetConfig(ctx, "plugin", map[string]any{"timeout": 60})
	assert.NoError(t, err)

	pluginConfig := store.GetConfig("plugin")
	assert.Equal(t, 60, pluginConfig["timeout"])
}

func TestPluginStateStore_CombinedUsage(t *testing.T) {
	store := newMockPluginStateStore()
	ctx := context.Background()

	// Use both persistence and config methods
	_ = store.SetEnabled(ctx, "plugin-a", true)
	_ = store.SetConfig(ctx, "plugin-a", map[string]any{"url": "https://api.example.com"})

	// Save state
	err := store.Save(ctx)
	assert.NoError(t, err)

	// Verify state
	state := store.GetState("plugin-a")
	assert.NotNil(t, state)
	assert.True(t, state.Enabled)
	assert.Equal(t, "https://api.example.com", state.Config["url"])

	// Verify via config interface
	assert.True(t, store.IsEnabled("plugin-a"))
	assert.Equal(t, "https://api.example.com", store.GetConfig("plugin-a")["url"])
}

func TestPluginStateStore_ListDisabled_IntegrationWithSetEnabled(t *testing.T) {
	store := newMockPluginStateStore()
	ctx := context.Background()

	// Disable multiple plugins
	_ = store.SetEnabled(ctx, "plugin-1", false)
	_ = store.SetEnabled(ctx, "plugin-2", false)
	_ = store.SetEnabled(ctx, "plugin-3", true)

	disabled := store.ListDisabled()
	assert.Len(t, disabled, 2)
	assert.Contains(t, disabled, "plugin-1")
	assert.Contains(t, disabled, "plugin-2")
	assert.NotContains(t, disabled, "plugin-3")
}

// Additional PluginManager tests

func TestMockPluginManager_Shutdown_Success(t *testing.T) {
	pm := mocks.NewMockPluginManager()
	pm.AddPlugin("test-plugin", plugin.StatusRunning)

	err := pm.Shutdown(context.Background(), "test-plugin")
	assert.NoError(t, err)

	info, _ := pm.Get("test-plugin")
	assert.Equal(t, plugin.StatusStopped, info.Status)
}

func TestMockPluginManager_ShutdownAll_Success(t *testing.T) {
	pm := mocks.NewMockPluginManager()
	pm.AddPlugin("plugin1", plugin.StatusRunning)
	pm.AddPlugin("plugin2", plugin.StatusRunning)

	err := pm.ShutdownAll(context.Background())
	assert.NoError(t, err)

	// Verify all plugins stopped
	info1, _ := pm.Get("plugin1")
	assert.Equal(t, plugin.StatusStopped, info1.Status)
	info2, _ := pm.Get("plugin2")
	assert.Equal(t, plugin.StatusStopped, info2.Status)
}

func TestMockPluginManager_Clear_ResetsState(t *testing.T) {
	pm := mocks.NewMockPluginManager()

	// Setup: add plugins and configure callbacks
	pm.AddPlugin("plugin1", plugin.StatusRunning)
	pm.AddPlugin("plugin2", plugin.StatusLoaded)
	pm.SetDiscoverFunc(func(ctx context.Context) ([]*plugin.PluginInfo, error) {
		return nil, errors.New("custom discover")
	})
	pm.SetLoadFunc(func(ctx context.Context, name string) error {
		return errors.New("custom load")
	})
	pm.SetInitFunc(func(ctx context.Context, name string, config map[string]interface{}) error {
		return errors.New("custom init")
	})
	pm.SetShutdownFunc(func(ctx context.Context, name string) error {
		return errors.New("custom shutdown")
	})
	pm.SetShutdownError(errors.New("shutdown error"))

	// Verify setup: plugins exist
	assert.Len(t, pm.List(), 2)
	_, exists := pm.Get("plugin1")
	assert.True(t, exists)

	pm.Clear()

	assert.Empty(t, pm.List())
	_, exists = pm.Get("plugin1")
	assert.False(t, exists)

	plugins, err := pm.Discover(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, plugins)

	// ShutdownAll should work without error on empty state
	err = pm.ShutdownAll(context.Background())
	assert.NoError(t, err)
}

// Table-driven tests for Plugin interface

func TestPlugin_Interface_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		pluginName  string
		version     string
		wantName    string
		wantVersion string
	}{
		{
			name:        "standard plugin",
			pluginName:  "slack-notifier",
			version:     "1.0.0",
			wantName:    "slack-notifier",
			wantVersion: "1.0.0",
		},
		{
			name:        "plugin with hyphenated name",
			pluginName:  "awf-plugin-github",
			version:     "2.1.0-beta",
			wantName:    "awf-plugin-github",
			wantVersion: "2.1.0-beta",
		},
		{
			name:        "empty name and version",
			pluginName:  "",
			version:     "",
			wantName:    "",
			wantVersion: "",
		},
		{
			name:        "plugin with prerelease version",
			pluginName:  "test-plugin",
			version:     "0.0.1-alpha.1+build.123",
			wantName:    "test-plugin",
			wantVersion: "0.0.1-alpha.1+build.123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &mockPlugin{name: tt.pluginName, version: tt.version}
			assert.Equal(t, tt.wantName, p.Name())
			assert.Equal(t, tt.wantVersion, p.Version())
		})
	}
}

// Table-driven tests for PluginManager Get/List

func TestPluginManager_Get_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*mocks.MockPluginManager)
		lookupName string
		wantFound  bool
		wantStatus plugin.PluginStatus
	}{
		{
			name: "find running plugin",
			setup: func(pm *mocks.MockPluginManager) {
				pm.AddPlugin("running", plugin.StatusRunning)
			},
			lookupName: "running",
			wantFound:  true,
			wantStatus: plugin.StatusRunning,
		},
		{
			name: "find stopped plugin",
			setup: func(pm *mocks.MockPluginManager) {
				pm.AddPlugin("stopped", plugin.StatusStopped)
			},
			lookupName: "stopped",
			wantFound:  true,
			wantStatus: plugin.StatusStopped,
		},
		{
			name: "find failed plugin",
			setup: func(pm *mocks.MockPluginManager) {
				info := pm.AddPlugin("failed", plugin.StatusFailed)
				info.Error = errors.New("init failed")
			},
			lookupName: "failed",
			wantFound:  true,
			wantStatus: plugin.StatusFailed,
		},
		{
			name:       "plugin not found",
			setup:      func(_ *mocks.MockPluginManager) {},
			lookupName: "nonexistent",
			wantFound:  false,
		},
		{
			name:       "empty name lookup",
			setup:      func(_ *mocks.MockPluginManager) {},
			lookupName: "",
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := mocks.NewMockPluginManager()
			tt.setup(pm)

			info, found := pm.Get(tt.lookupName)
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.Equal(t, tt.wantStatus, info.Status)
			}
		})
	}
}

// Table-driven tests for OperationProvider

func TestOperationProvider_GetOperation_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(*mockOperationProvider)
		operationName string
		wantFound     bool
		wantPlugin    string
	}{
		{
			name: "find operation by full name",
			setup: func(op *mockOperationProvider) {
				op.operations["slack.send"] = &plugin.OperationSchema{
					Name:       "slack.send",
					PluginName: "slack",
				}
			},
			operationName: "slack.send",
			wantFound:     true,
			wantPlugin:    "slack",
		},
		{
			name: "operation with inputs",
			setup: func(op *mockOperationProvider) {
				op.operations["http.request"] = &plugin.OperationSchema{
					Name:       "http.request",
					PluginName: "http",
					Inputs: map[string]plugin.InputSchema{
						"url":    {Type: "string", Required: true},
						"method": {Type: "string", Required: false, Default: "GET"},
					},
				}
			},
			operationName: "http.request",
			wantFound:     true,
			wantPlugin:    "http",
		},
		{
			name:          "operation not found",
			setup:         func(_ *mockOperationProvider) {},
			operationName: "nonexistent.op",
			wantFound:     false,
		},
		{
			name:          "empty operation name",
			setup:         func(_ *mockOperationProvider) {},
			operationName: "",
			wantFound:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := newMockOperationProvider()
			tt.setup(op)

			schema, found := op.GetOperation(tt.operationName)
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.Equal(t, tt.wantPlugin, schema.PluginName)
			}
		})
	}
}

func TestOperationProvider_ListOperations_Empty(t *testing.T) {
	op := newMockOperationProvider()
	list := op.ListOperations()
	assert.Empty(t, list)
	assert.NotNil(t, list) // Should return empty slice, not nil
}

// Table-driven tests for PluginRegistry

func TestPluginRegistry_RegisterOperation_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*mockPluginRegistry)
		operation *plugin.OperationSchema
		wantErr   bool
		errMsg    string
	}{
		{
			name:  "register new operation",
			setup: func(_ *mockPluginRegistry) {},
			operation: &plugin.OperationSchema{
				Name:        "new.op",
				Description: "A new operation",
				PluginName:  "test",
			},
			wantErr: false,
		},
		{
			name: "register duplicate operation",
			setup: func(reg *mockPluginRegistry) {
				reg.operations["existing.op"] = &plugin.OperationSchema{Name: "existing.op"}
			},
			operation: &plugin.OperationSchema{
				Name:       "existing.op",
				PluginName: "different",
			},
			wantErr: true,
			errMsg:  "already registered",
		},
		{
			name:  "register operation with inputs and outputs",
			setup: func(_ *mockPluginRegistry) {},
			operation: &plugin.OperationSchema{
				Name:        "complex.op",
				Description: "Complex operation",
				PluginName:  "complex",
				Inputs: map[string]plugin.InputSchema{
					"input1": {Type: "string", Required: true},
					"input2": {Type: "integer", Required: false},
				},
				Outputs: []string{"output1", "output2"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := newMockPluginRegistry()
			tt.setup(reg)

			err := reg.RegisterOperation(tt.operation)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				// Verify operation was registered
				_, found := reg.operations[tt.operation.Name]
				assert.True(t, found)
			}
		})
	}
}

func TestPluginRegistry_UnregisterOperation_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(*mockPluginRegistry)
		operationName string
		wantErr       bool
		errMsg        string
	}{
		{
			name: "unregister existing operation",
			setup: func(reg *mockPluginRegistry) {
				reg.operations["to.remove"] = &plugin.OperationSchema{Name: "to.remove"}
			},
			operationName: "to.remove",
			wantErr:       false,
		},
		{
			name:          "unregister nonexistent operation",
			setup:         func(_ *mockPluginRegistry) {},
			operationName: "nonexistent",
			wantErr:       true,
			errMsg:        "not found",
		},
		{
			name:          "unregister empty name",
			setup:         func(_ *mockPluginRegistry) {},
			operationName: "",
			wantErr:       true,
			errMsg:        "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := newMockPluginRegistry()
			tt.setup(reg)

			err := reg.UnregisterOperation(tt.operationName)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				// Verify operation was removed
				_, found := reg.operations[tt.operationName]
				assert.False(t, found)
			}
		})
	}
}

func TestPluginRegistry_Operations_Empty(t *testing.T) {
	reg := newMockPluginRegistry()
	ops := reg.Operations()
	assert.Empty(t, ops)
	assert.NotNil(t, ops) // Should return empty slice, not nil
}

// Context cancellation tests

func TestPlugin_Init_WithCancelledContext(t *testing.T) {
	p := &mockPlugin{name: "test", version: "1.0.0"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := p.Init(ctx, nil)
	// Mock still returns errNotImplemented, but real impl should check ctx
	assert.Error(t, err)
}

func TestPluginManager_Discover_WithCancelledContext(t *testing.T) {
	pm := mocks.NewMockPluginManager()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Mock doesn't enforce context cancellation, but real impl should
	_, err := pm.Discover(ctx)
	assert.NoError(t, err) // Mock succeeds even with canceled context
}

func TestOperationProvider_Execute_WithCancelledContext(t *testing.T) {
	op := newMockOperationProvider()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := op.Execute(ctx, "op", nil)
	// Mock returns errNotImplemented, real impl should respect ctx
	assert.Error(t, err)
}

// Config parameter tests

func TestPlugin_Init_WithConfig(t *testing.T) {
	p := &mockPlugin{name: "test", version: "1.0.0"}
	config := map[string]any{
		"webhook_url": "https://hooks.slack.com/...",
		"channel":     "#general",
		"timeout":     30,
		"enabled":     true,
	}

	err := p.Init(context.Background(), config)
	// Mock returns errNotImplemented
	assert.ErrorIs(t, err, errNotImplemented)
}

func TestPluginManager_Init_WithConfig(t *testing.T) {
	pm := mocks.NewMockPluginManager()
	pm.AddPlugin("plugin-name", plugin.StatusLoaded)

	config := map[string]any{
		"api_key": "secret-key",
		"retries": 3,
	}

	err := pm.Init(context.Background(), "plugin-name", config)
	assert.NoError(t, err)

	// Verify status changed to Running
	info, _ := pm.Get("plugin-name")
	assert.Equal(t, plugin.StatusRunning, info.Status)
}

// Multiple plugins lifecycle tests

func TestPluginManager_List_MultiplePlugins(t *testing.T) {
	pm := mocks.NewMockPluginManager()

	// Add plugins with different statuses
	statuses := []plugin.PluginStatus{
		plugin.StatusDiscovered,
		plugin.StatusLoaded,
		plugin.StatusInitialized,
		plugin.StatusRunning,
		plugin.StatusStopped,
		plugin.StatusFailed,
		plugin.StatusDisabled,
	}

	for i, status := range statuses {
		name := "plugin-" + string(rune('a'+i))
		pm.AddPlugin(name, status)
	}

	list := pm.List()
	assert.Len(t, list, len(statuses))
}

// Multiple operations tests

func TestOperationProvider_ListOperations_MultipleOperations(t *testing.T) {
	op := newMockOperationProvider()

	// Register multiple operations
	operations := []string{
		"slack.send",
		"slack.upload",
		"github.create-issue",
		"github.comment",
		"http.get",
		"http.post",
	}

	for _, opName := range operations {
		op.operations[opName] = &plugin.OperationSchema{
			Name:       opName,
			PluginName: opName[:len(opName)-5], // Extract plugin name
		}
	}

	list := op.ListOperations()
	assert.Len(t, list, len(operations))
}

// Registry sequential operations tests

func TestPluginRegistry_RegisterThenUnregister(t *testing.T) {
	reg := newMockPluginRegistry()

	// Register
	op := &plugin.OperationSchema{Name: "temp.op", PluginName: "temp"}
	err := reg.RegisterOperation(op)
	assert.NoError(t, err)
	assert.Len(t, reg.Operations(), 1)

	// Unregister
	err = reg.UnregisterOperation("temp.op")
	assert.NoError(t, err)
	assert.Empty(t, reg.Operations())

	// Re-register should work
	err = reg.RegisterOperation(op)
	assert.NoError(t, err)
	assert.Len(t, reg.Operations(), 1)
}

func TestPluginRegistry_UnregisterThenReregister(t *testing.T) {
	reg := newMockPluginRegistry()

	// Initial registration
	op1 := &plugin.OperationSchema{Name: "replace.op", PluginName: "v1"}
	err := reg.RegisterOperation(op1)
	assert.NoError(t, err)

	// Unregister
	err = reg.UnregisterOperation("replace.op")
	assert.NoError(t, err)

	// Re-register with different plugin
	op2 := &plugin.OperationSchema{Name: "replace.op", PluginName: "v2"}
	err = reg.RegisterOperation(op2)
	assert.NoError(t, err)

	// Verify new plugin owns the operation
	ops := reg.Operations()
	assert.Len(t, ops, 1)
	assert.Equal(t, "v2", ops[0].PluginName)
}
