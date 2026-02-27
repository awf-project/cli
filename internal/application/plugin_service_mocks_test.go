package application_test

// Component: T006
// Feature: C036
// Tests for split mock implementations (mockPluginStore, mockPluginConfig, mockPluginStateStore).
// Test count: 29 tests

import (
	"context"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPluginStore Tests (Happy Path)

func TestMockPluginStore_Save_HappyPath(t *testing.T) {
	store := newMockPluginStore()

	err := store.Save(context.Background())

	require.NoError(t, err, "Save should succeed by default")
}

func TestMockPluginStore_Load_HappyPath(t *testing.T) {
	store := newMockPluginStore()

	err := store.Load(context.Background())

	require.NoError(t, err, "Load should succeed by default")
}

func TestMockPluginStore_GetState_HappyPath(t *testing.T) {
	store := newMockPluginStore()
	expectedState := pluginmodel.NewPluginState()
	expectedState.Enabled = true
	store.states["test-plugin"] = expectedState

	state := store.GetState("test-plugin")

	require.NotNil(t, state)
	assert.True(t, state.Enabled)
}

func TestMockPluginStore_ListDisabled_HappyPath(t *testing.T) {
	store := newMockPluginStore()

	// Add disabled plugins
	disabledState1 := pluginmodel.NewPluginState()
	disabledState1.Enabled = false
	store.states["disabled-1"] = disabledState1

	disabledState2 := pluginmodel.NewPluginState()
	disabledState2.Enabled = false
	store.states["disabled-2"] = disabledState2

	// Add enabled plugin
	enabledState := pluginmodel.NewPluginState()
	enabledState.Enabled = true
	store.states["enabled"] = enabledState

	disabled := store.ListDisabled()

	assert.Len(t, disabled, 2, "Should return only disabled plugins")
	assert.Contains(t, disabled, "disabled-1")
	assert.Contains(t, disabled, "disabled-2")
	assert.NotContains(t, disabled, "enabled")
}

// mockPluginStore Tests (Edge Cases)

func TestMockPluginStore_GetState_NotExists(t *testing.T) {
	store := newMockPluginStore()

	state := store.GetState("nonexistent")

	assert.Nil(t, state, "Should return nil for nonexistent plugin")
}

func TestMockPluginStore_ListDisabled_Empty(t *testing.T) {
	store := newMockPluginStore()

	disabled := store.ListDisabled()

	assert.Empty(t, disabled, "Should return empty list when no disabled plugins")
}

func TestMockPluginStore_ListDisabled_AllEnabled(t *testing.T) {
	store := newMockPluginStore()
	enabledState := pluginmodel.NewPluginState()
	enabledState.Enabled = true
	store.states["plugin-1"] = enabledState
	store.states["plugin-2"] = enabledState

	disabled := store.ListDisabled()

	assert.Empty(t, disabled, "Should return empty list when all plugins enabled")
}

// mockPluginStore Tests (Error Handling)

func TestMockPluginStore_Save_Error(t *testing.T) {
	store := newMockPluginStore()
	expectedErr := assert.AnError
	store.saveErr = expectedErr

	err := store.Save(context.Background())

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestMockPluginStore_Load_Error(t *testing.T) {
	store := newMockPluginStore()
	expectedErr := assert.AnError
	store.loadErr = expectedErr

	err := store.Load(context.Background())

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestMockPluginStore_Save_WithCustomFunc(t *testing.T) {
	store := newMockPluginStore()
	called := false
	store.saveFunc = func(ctx context.Context) error {
		called = true
		return nil
	}

	err := store.Save(context.Background())

	require.NoError(t, err)
	assert.True(t, called, "Custom saveFunc should be called")
}

func TestMockPluginStore_Load_WithCustomFunc(t *testing.T) {
	store := newMockPluginStore()
	called := false
	store.loadFunc = func(ctx context.Context) error {
		called = true
		return nil
	}

	err := store.Load(context.Background())

	require.NoError(t, err)
	assert.True(t, called, "Custom loadFunc should be called")
}

// mockPluginConfig Tests (Happy Path)

func TestMockPluginConfig_SetEnabled_HappyPath(t *testing.T) {
	config := newMockPluginConfig()

	err := config.SetEnabled(context.Background(), "test-plugin", true)

	require.NoError(t, err)
	assert.True(t, config.IsEnabled("test-plugin"))
}

func TestMockPluginConfig_IsEnabled_DefaultTrue(t *testing.T) {
	config := newMockPluginConfig()

	enabled := config.IsEnabled("unknown-plugin")

	assert.True(t, enabled, "Should default to enabled for unknown plugins")
}

func TestMockPluginConfig_GetConfig_HappyPath(t *testing.T) {
	config := newMockPluginConfig()
	expectedConfig := map[string]any{"key": "value"}
	state := pluginmodel.NewPluginState()
	state.Config = expectedConfig
	config.states["test-plugin"] = state

	cfg := config.GetConfig("test-plugin")

	assert.Equal(t, expectedConfig, cfg)
}

func TestMockPluginConfig_SetConfig_HappyPath(t *testing.T) {
	config := newMockPluginConfig()
	expectedConfig := map[string]any{"webhook_url": "https://example.com"}

	err := config.SetConfig(context.Background(), "test-plugin", expectedConfig)

	require.NoError(t, err)
	assert.Equal(t, expectedConfig, config.GetConfig("test-plugin"))
}

// mockPluginConfig Tests (Edge Cases)

func TestMockPluginConfig_GetConfig_NotExists(t *testing.T) {
	config := newMockPluginConfig()

	cfg := config.GetConfig("nonexistent")

	assert.Nil(t, cfg, "Should return nil for nonexistent plugin")
}

func TestMockPluginConfig_SetEnabled_TogglingState(t *testing.T) {
	config := newMockPluginConfig()

	// Enable
	err := config.SetEnabled(context.Background(), "test-plugin", true)
	require.NoError(t, err)
	assert.True(t, config.IsEnabled("test-plugin"))

	// Disable
	err = config.SetEnabled(context.Background(), "test-plugin", false)
	require.NoError(t, err)
	assert.False(t, config.IsEnabled("test-plugin"))

	// Re-enable
	err = config.SetEnabled(context.Background(), "test-plugin", true)
	require.NoError(t, err)
	assert.True(t, config.IsEnabled("test-plugin"))
}

func TestMockPluginConfig_SetConfig_NilConfig(t *testing.T) {
	config := newMockPluginConfig()

	err := config.SetConfig(context.Background(), "test-plugin", nil)

	require.NoError(t, err)
	assert.Nil(t, config.GetConfig("test-plugin"))
}

// mockPluginConfig Tests (Error Handling)

func TestMockPluginConfig_SetEnabled_Error(t *testing.T) {
	config := newMockPluginConfig()
	expectedErr := assert.AnError
	config.setEnabledErr = expectedErr

	err := config.SetEnabled(context.Background(), "test-plugin", true)

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

// mockPluginStateStore Tests (Combined Interface)

func TestMockPluginStateStore_CombinesInterfaces(t *testing.T) {
	stateStore := newMockPluginStateStore()

	// Test PluginStore methods
	t.Run("PluginStore methods", func(t *testing.T) {
		err := stateStore.Save(context.Background())
		assert.NoError(t, err)

		err = stateStore.Load(context.Background())
		assert.NoError(t, err)

		state := stateStore.GetState("test")
		assert.Nil(t, state) // Not set yet

		disabled := stateStore.ListDisabled()
		assert.Empty(t, disabled)
	})

	// Test PluginConfig methods
	t.Run("PluginConfig methods", func(t *testing.T) {
		err := stateStore.SetEnabled(context.Background(), "test-plugin", false)
		assert.NoError(t, err)

		enabled := stateStore.IsEnabled("test-plugin")
		assert.False(t, enabled)

		cfg := map[string]any{"key": "value"}
		err = stateStore.SetConfig(context.Background(), "test-plugin", cfg)
		assert.NoError(t, err)

		retrievedCfg := stateStore.GetConfig("test-plugin")
		assert.Equal(t, cfg, retrievedCfg)
	})
}

func TestMockPluginStateStore_SharedState(t *testing.T) {
	stateStore := newMockPluginStateStore()

	// Set enabled via PluginConfig interface
	err := stateStore.SetEnabled(context.Background(), "test-plugin", true)
	require.NoError(t, err)

	// Verify state visible via PluginStore interface
	state := stateStore.GetState("test-plugin")
	require.NotNil(t, state)
	assert.True(t, state.Enabled)

	// Verify via PluginConfig interface
	assert.True(t, stateStore.IsEnabled("test-plugin"))
}

func TestMockPluginStateStore_HelperMethods(t *testing.T) {
	stateStore := newMockPluginStateStore()

	t.Run("setPluginEnabled helper", func(t *testing.T) {
		stateStore.setPluginEnabled("plugin-1", false)

		assert.False(t, stateStore.IsEnabled("plugin-1"))
		state := stateStore.GetState("plugin-1")
		require.NotNil(t, state)
		assert.False(t, state.Enabled)
	})

	t.Run("setPluginConfig helper", func(t *testing.T) {
		cfg := map[string]any{"timeout": 30}
		stateStore.setPluginConfig("plugin-2", cfg)

		retrievedCfg := stateStore.GetConfig("plugin-2")
		assert.Equal(t, cfg, retrievedCfg)

		state := stateStore.GetState("plugin-2")
		require.NotNil(t, state)
		assert.Equal(t, cfg, state.Config)
	})
}

func TestMockPluginStore_ConcurrentAccess(t *testing.T) {
	store := newMockPluginStore()
	var wg sync.WaitGroup
	const goroutines = 20

	// Concurrent writes
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			state := pluginmodel.NewPluginState()
			state.Enabled = id%2 == 0
			store.mu.Lock()
			store.states[string(rune('a'+id))] = state
			store.mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Should not panic and have correct count
	assert.Len(t, store.states, goroutines)
}

func TestMockPluginConfig_ConcurrentAccess(t *testing.T) {
	config := newMockPluginConfig()
	var wg sync.WaitGroup
	const goroutines = 20

	// Concurrent SetEnabled calls
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := config.SetEnabled(context.Background(), string(rune('a'+id)), id%2 == 0)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	// Should not panic and have correct count
	assert.Len(t, config.states, goroutines)
}

func TestMockPluginStateStore_ConcurrentAccessSharedState(t *testing.T) {
	stateStore := newMockPluginStateStore()
	var wg sync.WaitGroup
	const goroutines = 10

	// Concurrent operations via single interface (thread-safe within each mock)
	// Note: Concurrent access via different interfaces (mockPluginStore.mu vs mockPluginConfig.mu)
	// requires external synchronization since they share the states map.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pluginName := string(rune('a' + id))
			// Use only PluginConfig interface to avoid mutex conflict
			err := stateStore.SetEnabled(context.Background(), pluginName, true)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	// All plugins should have state
	for i := 0; i < goroutines; i++ {
		pluginName := string(rune('a' + i))
		state := stateStore.GetState(pluginName)
		assert.NotNil(t, state, "plugin %s should have state", pluginName)
	}
}

func TestMockPluginStore_EmptyStates(t *testing.T) {
	store := newMockPluginStore()

	// All methods should handle empty states gracefully
	assert.Nil(t, store.GetState("anything"))
	assert.Empty(t, store.ListDisabled())
	assert.NoError(t, store.Save(context.Background()))
	assert.NoError(t, store.Load(context.Background()))
}

func TestMockPluginConfig_EmptyPluginName(t *testing.T) {
	config := newMockPluginConfig()

	// Should handle empty string plugin name
	err := config.SetEnabled(context.Background(), "", true)
	assert.NoError(t, err)

	assert.True(t, config.IsEnabled(""))

	// After SetEnabled, GetConfig returns empty map (state created)
	cfg := config.GetConfig("")
	assert.NotNil(t, cfg, "Config should exist after SetEnabled creates state")
	assert.Empty(t, cfg, "Config map should be empty")
}
