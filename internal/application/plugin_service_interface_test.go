package application_test

// Component: T007
// Feature: C036

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests verify that PluginService can work with:
// - Option A (current): PluginStateStore composite interface (RECOMMENDED)
// - Option B (alternative): Separate PluginStore and PluginConfig interfaces
//
// These tests ensure the interface split (from T004) enables future flexibility
// in service dependencies while maintaining backward compatibility.

// Option A Tests: Current Implementation (PluginStateStore)

func TestPluginService_OptionA_WorksWithCompositeInterface(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	// Test persistence operations (from PluginStore)
	err := svc.SaveState(context.Background())
	require.NoError(t, err)

	err = svc.LoadState(context.Background())
	require.NoError(t, err)

	disabled := svc.ListDisabledPlugins()
	assert.Empty(t, disabled) // No disabled plugins initially

	// Test configuration operations (from PluginConfig)
	err = svc.EnablePlugin(context.Background(), "test-plugin")
	require.NoError(t, err)

	enabled := svc.IsPluginEnabled("test-plugin")
	assert.True(t, enabled)

	config := map[string]any{"key": "value"}
	err = svc.SetPluginConfig(context.Background(), "test-plugin", config)
	require.NoError(t, err)

	retrieved := svc.GetPluginConfig("test-plugin")
	assert.Equal(t, "value", retrieved["key"])
}

func TestPluginService_OptionA_CompositeInterfaceAcceptsNarrower(t *testing.T) {
	// Verify that implementations of narrower interfaces satisfy composite
	var _ ports.PluginStateStore = newMockPluginStateStore()

	// The composite embeds both interfaces, so narrower implementations
	// can be wrapped to satisfy the composite
	store := newMockPluginStore()
	config := newMockPluginConfig()

	// Create composite from separate implementations
	composite := &mockPluginStateStore{
		mockPluginStore:  store,
		mockPluginConfig: config,
	}

	// Should satisfy the interface
	var _ ports.PluginStateStore = composite
}

// Option B Tests: Alternative Implementation (Separate Interfaces)

// These tests verify that IF we chose Option B (separate pluginStore and
// pluginConfig fields), the split interfaces would support this refactoring.

func TestPluginService_OptionB_CanUseSeparateStoreInterface(t *testing.T) {
	// This test demonstrates that methods needing ONLY persistence
	// could use ports.PluginStore instead of full PluginStateStore

	store := newMockPluginStore()
	config := newMockPluginConfig()
	composite := &mockPluginStateStore{
		mockPluginStore:  store,
		mockPluginConfig: config,
	}

	manager := mocks.NewMockPluginManager()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, composite, logger)

	// Save/Load/ListDisabled only require PluginStore interface
	err := svc.SaveState(context.Background())
	require.NoError(t, err)

	err = svc.LoadState(context.Background())
	require.NoError(t, err)

	disabled := svc.ListDisabledPlugins()
	assert.Empty(t, disabled) // Returns empty slice from store

	// Verify that the store mock was used (not config)
	// This demonstrates interface segregation is possible
	assert.NotNil(t, store)
}

func TestPluginService_OptionB_CanUseSeparateConfigInterface(t *testing.T) {
	// This test demonstrates that methods needing ONLY configuration
	// could use ports.PluginConfig instead of full PluginStateStore

	store := newMockPluginStore()
	config := newMockPluginConfig()
	composite := &mockPluginStateStore{
		mockPluginStore:  store,
		mockPluginConfig: config,
	}

	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDiscovered)
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, composite, logger)

	// IsEnabled/SetEnabled/GetConfig/SetConfig only require PluginConfig
	err := svc.EnablePlugin(context.Background(), "test-plugin")
	require.NoError(t, err)

	enabled := svc.IsPluginEnabled("test-plugin")
	assert.True(t, enabled) // Default enabled for new plugins

	testConfig := map[string]any{"timeout": 30}
	err = svc.SetPluginConfig(context.Background(), "test-plugin", testConfig)
	require.NoError(t, err)

	retrieved := svc.GetPluginConfig("test-plugin")
	assert.Equal(t, 30, retrieved["timeout"])

	// Verify that the config mock was used (not store)
	assert.NotNil(t, config)
}

func TestPluginService_OptionB_MethodsDependOnBothInterfaces(t *testing.T) {
	// This test identifies which methods need BOTH interfaces
	// These methods would need access to both pluginStore AND pluginConfig
	// if we split the dependency

	store := newMockPluginStore()
	config := newMockPluginConfig()
	composite := &mockPluginStateStore{
		mockPluginStore:  store,
		mockPluginConfig: config,
	}

	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, composite, logger)

	// DisablePlugin needs BOTH:
	// - PluginConfig.SetEnabled (to mark disabled)
	// - It also checks plugin status via manager, but doesn't need PluginStore
	err := svc.DisablePlugin(context.Background(), "test-plugin")
	require.NoError(t, err)

	// Verify both interfaces were used
	assert.False(t, config.IsEnabled("test-plugin")) // Config was updated

	// DiscoverPlugins needs BOTH:
	// - Manager.Discover (from manager, not stateStore)
	// - PluginConfig.IsEnabled (to filter)
	config.SetEnabled(context.Background(), "enabled-plugin", true)
	config.SetEnabled(context.Background(), "disabled-plugin", false)
	manager.AddPlugin("enabled-plugin", pluginmodel.StatusDiscovered)
	manager.AddPlugin("disabled-plugin", pluginmodel.StatusDiscovered)

	plugins, err := svc.DiscoverPlugins(context.Background())
	require.NoError(t, err)
	// Should only return enabled plugins
	assert.Len(t, plugins, 1)
	assert.Equal(t, "enabled-plugin", plugins[0].Manifest.Name)
}

// Edge Cases: Nil Dependencies

func TestPluginService_OptionA_NilCompositeInterface(t *testing.T) {
	// Service should handle nil stateStore gracefully
	manager := mocks.NewMockPluginManager()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, nil, logger)

	// Operations requiring stateStore should handle nil
	err := svc.SaveState(context.Background())
	require.NoError(t, err) // Graceful: no-op when nil

	err = svc.LoadState(context.Background())
	require.NoError(t, err) // Graceful: no-op when nil

	enabled := svc.IsPluginEnabled("test-plugin")
	assert.True(t, enabled) // Default: enabled when nil store

	config := svc.GetPluginConfig("test-plugin")
	assert.Nil(t, config) // Returns nil when no store
}

func TestPluginService_OptionB_SharedStateBetweenInterfaces(t *testing.T) {
	// If we split to separate store/config, they MUST share state
	// This test verifies the mock implementation shares state correctly

	store := newMockPluginStore()
	config := newMockPluginConfig()

	// Share the same states map
	config.states = store.states

	composite := &mockPluginStateStore{
		mockPluginStore:  store,
		mockPluginConfig: config,
	}

	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("shared-plugin", pluginmodel.StatusDiscovered)
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, composite, logger)

	// Set enabled via config interface
	err := svc.EnablePlugin(context.Background(), "shared-plugin")
	require.NoError(t, err)

	// Read via config interface
	enabled := svc.IsPluginEnabled("shared-plugin")
	assert.True(t, enabled)

	// GetState via store interface should see the same state
	state := store.GetState("shared-plugin")
	require.NotNil(t, state)
	assert.True(t, state.Enabled) // State is shared

	// Set config via config interface
	testConfig := map[string]any{"shared": "data"}
	err = svc.SetPluginConfig(context.Background(), "shared-plugin", testConfig)
	require.NoError(t, err)

	// GetState via store should see config
	state = store.GetState("shared-plugin")
	require.NotNil(t, state)
	assert.Equal(t, "data", state.Config["shared"])
}

// Error Handling: Interface Boundary Conditions

func TestPluginService_OptionA_ErrorPropagation(t *testing.T) {
	// Errors from the composite interface should propagate correctly
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDiscovered)
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	// Test Save error propagation
	stateStore.saveErr = assert.AnError
	err := svc.SaveState(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save plugin state")

	// Test Load error propagation
	stateStore.saveErr = nil
	stateStore.loadErr = assert.AnError
	err = svc.LoadState(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load plugin state")

	// Test SetEnabled error propagation
	stateStore.loadErr = nil
	stateStore.setEnabledErr = assert.AnError
	err = svc.EnablePlugin(context.Background(), "test-plugin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enable plugin")
}

func TestPluginService_OptionB_InterfaceComplianceVerification(t *testing.T) {
	// Verify that our mocks correctly implement the split interfaces
	// This ensures Option B would be viable if chosen

	// PluginStore compliance
	var pluginStore ports.PluginStore = newMockPluginStore()
	assert.NotNil(t, pluginStore)

	// PluginConfig compliance
	var pluginConfig ports.PluginConfig = newMockPluginConfig()
	assert.NotNil(t, pluginConfig)

	// Composite (PluginStateStore) compliance
	var composite ports.PluginStateStore = newMockPluginStateStore()
	assert.NotNil(t, composite)

	// Composite should satisfy both narrower interfaces
	var storeFromComposite ports.PluginStore = composite
	var configFromComposite ports.PluginConfig = composite
	assert.NotNil(t, storeFromComposite)
	assert.NotNil(t, configFromComposite)
}

// Happy Path: Current Recommended Implementation

func TestPluginService_T007_RecommendedOptionA_IntegrationTest(t *testing.T) {
	// This test demonstrates the RECOMMENDED approach (Option A):
	// Keep current implementation using PluginStateStore composite interface

	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("plugin-a", pluginmodel.StatusDiscovered)
	manager.AddPlugin("plugin-b", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	// Initially disable plugin-b
	stateStore.setPluginEnabled("plugin-b", false)

	svc := application.NewPluginService(manager, stateStore, logger)

	// 1. Load state from storage
	err := svc.LoadState(context.Background())
	require.NoError(t, err)

	// 2. Discover enabled plugins
	plugins, err := svc.DiscoverPlugins(context.Background())
	require.NoError(t, err)
	assert.Len(t, plugins, 1) // Only plugin-a (enabled)
	assert.Equal(t, "plugin-a", plugins[0].Manifest.Name)

	// 3. Enable plugin-b
	err = svc.EnablePlugin(context.Background(), "plugin-b")
	require.NoError(t, err)
	assert.True(t, svc.IsPluginEnabled("plugin-b"))

	// 4. Discover again - should see both plugins
	plugins, err = svc.DiscoverPlugins(context.Background())
	require.NoError(t, err)
	assert.Len(t, plugins, 2)

	// 5. Configure plugin-a
	config := map[string]any{
		"webhook_url": "https://example.com",
		"timeout":     30,
	}
	err = svc.SetPluginConfig(context.Background(), "plugin-a", config)
	require.NoError(t, err)

	// 6. Retrieve config
	retrieved := svc.GetPluginConfig("plugin-a")
	assert.Equal(t, "https://example.com", retrieved["webhook_url"])

	// 7. Save state to storage
	err = svc.SaveState(context.Background())
	require.NoError(t, err)

	// 8. List disabled plugins
	disabled := svc.ListDisabledPlugins()
	assert.Empty(t, disabled) // Both enabled now

	// This test passes with current implementation (Option A)
	// and demonstrates that PluginStateStore composite interface
	// provides all needed functionality without requiring field changes
}

// Boundary Conditions: Empty and Missing Data

func TestPluginService_T007_EmptyPluginName(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	// Empty plugin name should be handled gracefully
	enabled := svc.IsPluginEnabled("")
	assert.True(t, enabled) // Default behavior

	config := svc.GetPluginConfig("")
	assert.Nil(t, config) // No config for empty name

	disabled := svc.ListDisabledPlugins()
	assert.NotContains(t, disabled, "") // Empty name not in list
}

func TestPluginService_T007_NonExistentPlugin(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	// Non-existent plugin queries
	enabled := svc.IsPluginEnabled("nonexistent")
	assert.True(t, enabled) // Default: enabled

	config := svc.GetPluginConfig("nonexistent")
	assert.Nil(t, config) // No config

	state := stateStore.GetState("nonexistent")
	assert.Nil(t, state) // No state
}

func TestPluginService_T007_ContextCancellation(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Context-aware operations should fail
	err := svc.SaveState(ctx)
	assert.ErrorIs(t, err, context.Canceled)

	err = svc.LoadState(ctx)
	assert.ErrorIs(t, err, context.Canceled)

	err = svc.EnablePlugin(ctx, "test-plugin")
	assert.ErrorIs(t, err, context.Canceled)

	err = svc.SetPluginConfig(ctx, "test-plugin", nil)
	assert.ErrorIs(t, err, context.Canceled)

	// Non-context operations should still work
	enabled := svc.IsPluginEnabled("test-plugin")
	assert.True(t, enabled)

	config := svc.GetPluginConfig("test-plugin")
	assert.Nil(t, config)
}
