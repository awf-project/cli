package ports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
)

var errNotImplemented = errors.New("not implemented")

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

// mockPluginManager implements ports.PluginManager interface for testing.
type mockPluginManager struct {
	plugins map[string]*plugin.PluginInfo
}

func newMockPluginManager() *mockPluginManager {
	return &mockPluginManager{
		plugins: make(map[string]*plugin.PluginInfo),
	}
}

func (m *mockPluginManager) Discover(_ context.Context) ([]*plugin.PluginInfo, error) {
	return nil, errNotImplemented
}

func (m *mockPluginManager) Load(_ context.Context, _ string) error {
	return errNotImplemented
}

func (m *mockPluginManager) Init(_ context.Context, _ string, _ map[string]any) error {
	return errNotImplemented
}

func (m *mockPluginManager) Shutdown(_ context.Context, _ string) error {
	return errNotImplemented
}

func (m *mockPluginManager) ShutdownAll(_ context.Context) error {
	return errNotImplemented
}

func (m *mockPluginManager) Get(name string) (*plugin.PluginInfo, bool) {
	info, ok := m.plugins[name]
	return info, ok
}

func (m *mockPluginManager) List() []*plugin.PluginInfo {
	result := make([]*plugin.PluginInfo, 0, len(m.plugins))
	for _, info := range m.plugins {
		result = append(result, info)
	}
	return result
}

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

// Interface compliance tests
func TestPluginInterface(t *testing.T) {
	var _ ports.Plugin = (*mockPlugin)(nil)
}

func TestPluginManagerInterface(t *testing.T) {
	var _ ports.PluginManager = (*mockPluginManager)(nil)
}

func TestOperationProviderInterface(t *testing.T) {
	var _ ports.OperationProvider = (*mockOperationProvider)(nil)
}

func TestPluginRegistryInterface(t *testing.T) {
	var _ ports.PluginRegistry = (*mockPluginRegistry)(nil)
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
func TestMockPluginManager_Discover_ReturnsNotImplemented(t *testing.T) {
	pm := newMockPluginManager()
	_, err := pm.Discover(context.Background())
	assert.ErrorIs(t, err, errNotImplemented)
}

func TestMockPluginManager_Load_ReturnsNotImplemented(t *testing.T) {
	pm := newMockPluginManager()
	err := pm.Load(context.Background(), "test-plugin")
	assert.ErrorIs(t, err, errNotImplemented)
}

func TestMockPluginManager_Init_ReturnsNotImplemented(t *testing.T) {
	pm := newMockPluginManager()
	err := pm.Init(context.Background(), "test-plugin", nil)
	assert.ErrorIs(t, err, errNotImplemented)
}

func TestMockPluginManager_Get(t *testing.T) {
	pm := newMockPluginManager()
	pm.plugins["test"] = &plugin.PluginInfo{
		Manifest: &plugin.Manifest{Name: "test"},
		Status:   plugin.StatusRunning,
	}

	info, ok := pm.Get("test")
	assert.True(t, ok)
	assert.Equal(t, "test", info.Manifest.Name)

	_, ok = pm.Get("nonexistent")
	assert.False(t, ok)
}

func TestMockPluginManager_List(t *testing.T) {
	pm := newMockPluginManager()
	pm.plugins["p1"] = &plugin.PluginInfo{Manifest: &plugin.Manifest{Name: "p1"}}
	pm.plugins["p2"] = &plugin.PluginInfo{Manifest: &plugin.Manifest{Name: "p2"}}

	list := pm.List()
	assert.Len(t, list, 2)
}

func TestMockPluginManager_List_Empty(t *testing.T) {
	pm := newMockPluginManager()
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

// Additional PluginManager tests

func TestMockPluginManager_Shutdown_ReturnsNotImplemented(t *testing.T) {
	pm := newMockPluginManager()
	err := pm.Shutdown(context.Background(), "test-plugin")
	assert.ErrorIs(t, err, errNotImplemented)
}

func TestMockPluginManager_ShutdownAll_ReturnsNotImplemented(t *testing.T) {
	pm := newMockPluginManager()
	err := pm.ShutdownAll(context.Background())
	assert.ErrorIs(t, err, errNotImplemented)
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
		setup      func(*mockPluginManager)
		lookupName string
		wantFound  bool
		wantStatus plugin.PluginStatus
	}{
		{
			name: "find running plugin",
			setup: func(pm *mockPluginManager) {
				pm.plugins["running"] = &plugin.PluginInfo{
					Manifest: &plugin.Manifest{Name: "running"},
					Status:   plugin.StatusRunning,
				}
			},
			lookupName: "running",
			wantFound:  true,
			wantStatus: plugin.StatusRunning,
		},
		{
			name: "find stopped plugin",
			setup: func(pm *mockPluginManager) {
				pm.plugins["stopped"] = &plugin.PluginInfo{
					Manifest: &plugin.Manifest{Name: "stopped"},
					Status:   plugin.StatusStopped,
				}
			},
			lookupName: "stopped",
			wantFound:  true,
			wantStatus: plugin.StatusStopped,
		},
		{
			name: "find failed plugin",
			setup: func(pm *mockPluginManager) {
				pm.plugins["failed"] = &plugin.PluginInfo{
					Manifest: &plugin.Manifest{Name: "failed"},
					Status:   plugin.StatusFailed,
					Error:    errors.New("init failed"),
				}
			},
			lookupName: "failed",
			wantFound:  true,
			wantStatus: plugin.StatusFailed,
		},
		{
			name:       "plugin not found",
			setup:      func(_ *mockPluginManager) {},
			lookupName: "nonexistent",
			wantFound:  false,
		},
		{
			name:       "empty name lookup",
			setup:      func(_ *mockPluginManager) {},
			lookupName: "",
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := newMockPluginManager()
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
	pm := newMockPluginManager()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pm.Discover(ctx)
	// Mock returns errNotImplemented, real impl should respect ctx
	assert.Error(t, err)
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
	pm := newMockPluginManager()
	config := map[string]any{
		"api_key": "secret-key",
		"retries": 3,
	}

	err := pm.Init(context.Background(), "plugin-name", config)
	assert.ErrorIs(t, err, errNotImplemented)
}

// Multiple plugins lifecycle tests

func TestPluginManager_List_MultiplePlugins(t *testing.T) {
	pm := newMockPluginManager()

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
		pm.plugins[name] = &plugin.PluginInfo{
			Manifest: &plugin.Manifest{Name: name},
			Status:   status,
		}
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
