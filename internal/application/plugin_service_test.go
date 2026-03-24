package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPluginManager is now consolidated in internal/testutil/mocks.go (C037).
// Use mocks.NewMockPluginManager() instead of local implementation.

// mockPluginStore implements ports.PluginStore for testing.
type mockPluginStore struct {
	mu       sync.RWMutex
	states   map[string]*pluginmodel.PluginState
	saveFunc func(ctx context.Context) error
	loadFunc func(ctx context.Context) error
	saveErr  error
	loadErr  error
}

func newMockPluginStore() *mockPluginStore {
	return &mockPluginStore{
		states: make(map[string]*pluginmodel.PluginState),
	}
}

func (m *mockPluginStore) Save(ctx context.Context) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx)
	}
	if m.saveErr != nil {
		return m.saveErr
	}
	return nil
}

func (m *mockPluginStore) Load(ctx context.Context) error {
	if m.loadFunc != nil {
		return m.loadFunc(ctx)
	}
	if m.loadErr != nil {
		return m.loadErr
	}
	return nil
}

func (m *mockPluginStore) GetState(name string) *pluginmodel.PluginState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[name]
}

func (m *mockPluginStore) ListDisabled() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var disabled []string
	for name, state := range m.states {
		if !state.Enabled {
			disabled = append(disabled, name)
		}
	}
	return disabled
}

// mockPluginConfig implements ports.PluginConfig for testing.
type mockPluginConfig struct {
	mu            sync.RWMutex
	states        map[string]*pluginmodel.PluginState
	setEnabledErr error
	setConfigErr  error
}

func newMockPluginConfig() *mockPluginConfig {
	return &mockPluginConfig{
		states: make(map[string]*pluginmodel.PluginState),
	}
}

func (m *mockPluginConfig) SetEnabled(ctx context.Context, name string, enabled bool) error {
	if m.setEnabledErr != nil {
		return m.setEnabledErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.states[name]
	if !ok {
		state = pluginmodel.NewPluginState()
		m.states[name] = state
	}
	state.Enabled = enabled
	return nil
}

func (m *mockPluginConfig) IsEnabled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.states[name]
	if !ok {
		return true // Default: enabled
	}
	return state.Enabled
}

func (m *mockPluginConfig) GetConfig(name string) map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.states[name]
	if !ok {
		return nil
	}
	return state.Config
}

func (m *mockPluginConfig) SetConfig(ctx context.Context, name string, config map[string]any) error {
	if m.setConfigErr != nil {
		return m.setConfigErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.states[name]
	if !ok {
		state = pluginmodel.NewPluginState()
		m.states[name] = state
	}
	state.Config = config
	return nil
}

// mockPluginStateStore combines both interfaces for backward compatibility.
type mockPluginStateStore struct {
	*mockPluginStore
	*mockPluginConfig
}

func newMockPluginStateStore() *mockPluginStateStore {
	store := newMockPluginStore()
	config := newMockPluginConfig()
	// Share the same states map between both mocks
	config.states = store.states
	return &mockPluginStateStore{
		mockPluginStore:  store,
		mockPluginConfig: config,
	}
}

func (m *mockPluginStateStore) setPluginEnabled(name string, enabled bool) {
	m.mockPluginStore.mu.Lock()
	defer m.mockPluginStore.mu.Unlock()
	state := pluginmodel.NewPluginState()
	state.Enabled = enabled
	m.mockPluginStore.states[name] = state
}

func (m *mockPluginStateStore) setPluginConfig(name string, config map[string]any) {
	m.mockPluginStore.mu.Lock()
	defer m.mockPluginStore.mu.Unlock()
	state, ok := m.mockPluginStore.states[name]
	if !ok {
		state = pluginmodel.NewPluginState()
		m.mockPluginStore.states[name] = state
	}
	state.Config = config
}

// mockPluginLogger implements ports.Logger for testing.
type mockPluginLogger struct {
	logs []string
}

func newMockPluginLogger() *mockPluginLogger {
	return &mockPluginLogger{logs: make([]string, 0)}
}

func (m *mockPluginLogger) Debug(msg string, fields ...any)             { m.logs = append(m.logs, "DEBUG: "+msg) }
func (m *mockPluginLogger) Info(msg string, fields ...any)              { m.logs = append(m.logs, "INFO: "+msg) }
func (m *mockPluginLogger) Warn(msg string, fields ...any)              { m.logs = append(m.logs, "WARN: "+msg) }
func (m *mockPluginLogger) Error(msg string, fields ...any)             { m.logs = append(m.logs, "ERROR: "+msg) }
func (m *mockPluginLogger) WithContext(ctx map[string]any) ports.Logger { return m }

func TestNewPluginService(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	require.NotNil(t, svc, "expected service to be created")
}

func TestNewPluginService_NilDependencies(t *testing.T) {
	// Should handle nil dependencies gracefully (create service anyway)
	svc := application.NewPluginService(nil, nil, nil)
	require.NotNil(t, svc)
}

func TestPluginService_DiscoverPlugins_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("plugin-a", pluginmodel.StatusDiscovered)
	manager.AddPlugin("plugin-b", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins, err := svc.DiscoverPlugins(context.Background())

	require.NoError(t, err)
	assert.Len(t, plugins, 2)
}

func TestPluginService_DiscoverPlugins_EmptyDirectory(t *testing.T) {
	manager := mocks.NewMockPluginManager() // No plugins
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins, err := svc.DiscoverPlugins(context.Background())

	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestPluginService_DiscoverPlugins_FilterDisabled(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("enabled-plugin", pluginmodel.StatusDiscovered)
	manager.AddPlugin("disabled-plugin", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("disabled-plugin", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins, err := svc.DiscoverPlugins(context.Background())

	require.NoError(t, err)
	assert.Len(t, plugins, 1)
	assert.Equal(t, "enabled-plugin", plugins[0].Manifest.Name)
}

func TestPluginService_DiscoverPlugins_ManagerError(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.SetDiscoverFunc(func(ctx context.Context) ([]*pluginmodel.PluginInfo, error) {
		return nil, errors.New("discovery failed")
	})

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins, err := svc.DiscoverPlugins(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "discovery failed")
	assert.Nil(t, plugins)
}

func TestPluginService_DiscoverPlugins_ContextCanceled(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.DiscoverPlugins(ctx)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestPluginService_LoadPlugin_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	info, found := manager.Get("test-plugin")
	require.True(t, found)
	assert.Equal(t, pluginmodel.StatusLoaded, info.Status)
}

func TestPluginService_LoadPlugin_NotFound(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found")
}

func TestPluginService_LoadPlugin_Disabled(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("disabled-plugin", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("disabled-plugin", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "disabled-plugin")

	assert.ErrorIs(t, err, application.ErrPluginDisabled)
}

func TestPluginService_LoadPlugin_AlreadyLoaded(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusLoaded)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "test-plugin")

	// Should be idempotent (no error)
	require.NoError(t, err)
}

func TestPluginService_LoadPlugin_EmptyName(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "")

	assert.ErrorIs(t, err, application.ErrPluginNameEmpty)
}

func TestPluginService_InitPlugin_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusLoaded)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginConfig("test-plugin", map[string]any{"key": "value"})

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.InitPlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	info, found := manager.Get("test-plugin")
	require.True(t, found)
	assert.Equal(t, pluginmodel.StatusRunning, info.Status)
}

func TestPluginService_InitPlugin_NotLoaded(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	// Plugin discovered but not loaded - mock returns error for init on non-loaded
	manager.AddPlugin("test-plugin", pluginmodel.StatusDiscovered)
	manager.SetInitFunc(func(ctx context.Context, name string, config map[string]any) error {
		return errors.New("plugin must be loaded first")
	})

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.InitPlugin(context.Background(), "test-plugin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin must be loaded first")
}

func TestPluginService_InitPlugin_AlreadyRunning(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.InitPlugin(context.Background(), "test-plugin")

	// Should be idempotent (no error) - mock init just sets to Running
	require.NoError(t, err)
}

func TestPluginService_InitPlugin_WithStoredConfig(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusLoaded)

	var capturedConfig map[string]any
	manager.SetInitFunc(func(ctx context.Context, name string, config map[string]any) error {
		capturedConfig = config
		return nil
	})

	stateStore := newMockPluginStateStore()
	stateStore.setPluginConfig("test-plugin", map[string]any{
		"webhook_url": "https://example.com/hook",
		"timeout":     30,
	})

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.InitPlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/hook", capturedConfig["webhook_url"])
	assert.Equal(t, 30, capturedConfig["timeout"])
}

func TestPluginService_LoadAndInitPlugin_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadAndInitPlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	info, found := manager.Get("test-plugin")
	require.True(t, found)
	assert.Equal(t, pluginmodel.StatusRunning, info.Status)
}

func TestPluginService_LoadAndInitPlugin_LoadFails(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.SetLoadFunc(func(ctx context.Context, name string) error {
		return errors.New("load failed")
	})

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadAndInitPlugin(context.Background(), "test-plugin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "load failed")
}

func TestPluginService_LoadAndInitPlugin_InitFails(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDiscovered)
	manager.SetInitFunc(func(ctx context.Context, name string, config map[string]any) error {
		return errors.New("init failed")
	})

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadAndInitPlugin(context.Background(), "test-plugin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "init failed")
}

func TestPluginService_ShutdownPlugin_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownPlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	info, found := manager.Get("test-plugin")
	require.True(t, found)
	assert.Equal(t, pluginmodel.StatusStopped, info.Status)
}

func TestPluginService_ShutdownPlugin_NotRunning(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusStopped)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownPlugin(context.Background(), "test-plugin")

	// Should be idempotent (no error)
	require.NoError(t, err)
}

func TestPluginService_ShutdownPlugin_NotFound(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownPlugin(context.Background(), "nonexistent")

	// Should handle gracefully (no error for missing plugin)
	require.NoError(t, err)
}

func TestPluginService_ShutdownAll_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("plugin-a", pluginmodel.StatusRunning)
	manager.AddPlugin("plugin-b", pluginmodel.StatusRunning)
	manager.AddPlugin("plugin-c", pluginmodel.StatusStopped) // Already stopped

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownAll(context.Background())

	require.NoError(t, err)
	infoA, _ := manager.Get("plugin-a")
	assert.Equal(t, pluginmodel.StatusStopped, infoA.Status)
	infoB, _ := manager.Get("plugin-b")
	assert.Equal(t, pluginmodel.StatusStopped, infoB.Status)
}

func TestPluginService_ShutdownAll_NoPlugins(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownAll(context.Background())

	require.NoError(t, err)
}

func TestPluginService_ShutdownAll_PartialFailure(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("plugin-a", pluginmodel.StatusRunning)
	manager.AddPlugin("plugin-b", pluginmodel.StatusRunning)
	manager.SetShutdownError(errors.New("shutdown failed for one plugin"))

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownAll(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown failed")
}

func TestPluginService_EnablePlugin_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDisabled)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.EnablePlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	assert.True(t, stateStore.IsEnabled("test-plugin"))
}

func TestPluginService_EnablePlugin_AlreadyEnabled(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", true)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.EnablePlugin(context.Background(), "test-plugin")

	// Should be idempotent (no error)
	require.NoError(t, err)
	assert.True(t, stateStore.IsEnabled("test-plugin"))
}

func TestPluginService_EnablePlugin_SaveStateFails(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDisabled)

	stateStore := newMockPluginStateStore()
	stateStore.setEnabledErr = errors.New("save failed")

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.EnablePlugin(context.Background(), "test-plugin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "save failed")
}

func TestPluginService_DisablePlugin_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", true)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.DisablePlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	assert.False(t, stateStore.IsEnabled("test-plugin"))
}

func TestPluginService_DisablePlugin_AlreadyDisabled(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDisabled)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.DisablePlugin(context.Background(), "test-plugin")

	// Should be idempotent
	require.NoError(t, err)
}

func TestPluginService_DisablePlugin_ShutdownsRunning(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.DisablePlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	// Plugin should be shut down
	info, found := manager.Get("test-plugin")
	require.True(t, found)
	assert.Equal(t, pluginmodel.StatusStopped, info.Status)
}

func TestPluginService_SetPluginConfig_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	config := map[string]any{
		"webhook_url": "https://example.com/hook",
		"timeout":     30,
	}
	err := svc.SetPluginConfig(context.Background(), "test-plugin", config)

	require.NoError(t, err)
	storedConfig := stateStore.GetConfig("test-plugin")
	assert.Equal(t, "https://example.com/hook", storedConfig["webhook_url"])
}

func TestPluginService_SetPluginConfig_NilConfig(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.SetPluginConfig(context.Background(), "test-plugin", nil)

	// Should handle nil config (clear config)
	require.NoError(t, err)
}

func TestPluginService_GetPluginConfig_Exists(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	stateStore.setPluginConfig("test-plugin", map[string]any{"key": "value"})

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	config := svc.GetPluginConfig("test-plugin")

	assert.Equal(t, "value", config["key"])
}

func TestPluginService_GetPluginConfig_NotExists(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	config := svc.GetPluginConfig("nonexistent")

	assert.Nil(t, config)
}

func TestPluginService_GetPlugin_Exists(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	info, found := svc.GetPlugin("test-plugin")

	assert.True(t, found)
	require.NotNil(t, info)
	assert.Equal(t, "test-plugin", info.Manifest.Name)
}

func TestPluginService_GetPlugin_NotExists(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	info, found := svc.GetPlugin("nonexistent")

	assert.False(t, found)
	assert.Nil(t, info)
}

func TestPluginService_ListPlugins(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("plugin-a", pluginmodel.StatusRunning)
	manager.AddPlugin("plugin-b", pluginmodel.StatusStopped)
	manager.AddPlugin("plugin-c", pluginmodel.StatusDisabled)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins := svc.ListPlugins()

	assert.Len(t, plugins, 3)
}

func TestPluginService_ListPlugins_Empty(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins := svc.ListPlugins()

	assert.Empty(t, plugins)
}

func TestPluginService_ListEnabledPlugins(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("enabled-a", pluginmodel.StatusRunning)
	manager.AddPlugin("enabled-b", pluginmodel.StatusLoaded)
	manager.AddPlugin("disabled", pluginmodel.StatusDisabled)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("disabled", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins := svc.ListEnabledPlugins()

	assert.Len(t, plugins, 2)
}

func TestPluginService_ListDisabledPlugins(t *testing.T) {
	manager := mocks.NewMockPluginManager()

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("disabled-a", false)
	stateStore.setPluginEnabled("disabled-b", false)
	stateStore.setPluginEnabled("enabled", true)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	disabled := svc.ListDisabledPlugins()

	assert.Len(t, disabled, 2)
	assert.Contains(t, disabled, "disabled-a")
	assert.Contains(t, disabled, "disabled-b")
}

func TestPluginService_IsPluginEnabled_True(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", true)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	enabled := svc.IsPluginEnabled("test-plugin")

	assert.True(t, enabled)
}

func TestPluginService_IsPluginEnabled_False(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	enabled := svc.IsPluginEnabled("test-plugin")

	assert.False(t, enabled)
}

func TestPluginService_IsPluginEnabled_DefaultTrue(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore() // No explicit state
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	enabled := svc.IsPluginEnabled("unknown-plugin")

	// Default: enabled for unknown plugins
	assert.True(t, enabled)
}

func TestPluginService_SaveState_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.SaveState(context.Background())

	require.NoError(t, err)
}

func TestPluginService_SaveState_Error(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	stateStore.saveErr = errors.New("disk full")

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.SaveState(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}

func TestPluginService_LoadState_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadState(context.Background())

	require.NoError(t, err)
}

func TestPluginService_LoadState_Error(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	stateStore.loadErr = errors.New("file not found")

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadState(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}

func TestPluginService_StartupEnabledPlugins_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("plugin-a", pluginmodel.StatusDiscovered)
	manager.AddPlugin("plugin-b", pluginmodel.StatusDiscovered)
	manager.AddPlugin("plugin-disabled", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("plugin-disabled", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.StartupEnabledPlugins(context.Background())

	require.NoError(t, err)
	// Enabled plugins should be running
	infoA, _ := manager.Get("plugin-a")
	assert.Equal(t, pluginmodel.StatusRunning, infoA.Status)
	infoB, _ := manager.Get("plugin-b")
	assert.Equal(t, pluginmodel.StatusRunning, infoB.Status)
	// Disabled plugin should not be started
	infoDisabled, _ := manager.Get("plugin-disabled")
	assert.Equal(t, pluginmodel.StatusDiscovered, infoDisabled.Status)
}

func TestPluginService_StartupEnabledPlugins_NoPlugins(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.StartupEnabledPlugins(context.Background())

	// Should handle empty plugins gracefully
	require.NoError(t, err)
}

func TestPluginService_StartupEnabledPlugins_PartialFailure(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("plugin-ok", pluginmodel.StatusDiscovered)
	manager.AddPlugin("plugin-fail", pluginmodel.StatusDiscovered)

	manager.SetInitFunc(func(ctx context.Context, name string, config map[string]any) error {
		if name == "plugin-fail" {
			return errors.New("init failed")
		}
		return nil
	})

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.StartupEnabledPlugins(context.Background())

	// Should continue with other plugins and aggregate errors
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init failed")
}

func TestPluginService_StartupEnabledPlugins_ContextCanceled(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("plugin-a", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.StartupEnabledPlugins(ctx)

	// Should respect context cancellation
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPluginService_ConcurrentOperations(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	// Simulate concurrent enable/disable operations
	done := make(chan bool, 2)

	go func() {
		_ = svc.EnablePlugin(context.Background(), "test-plugin")
		done <- true
	}()

	go func() {
		_ = svc.DisablePlugin(context.Background(), "test-plugin")
		done <- true
	}()

	// Wait for both operations
	<-done
	<-done

	// Test passes if no race condition panic occurs
	// When implemented, the final state depends on operation order
}

func TestPluginService_SpecialCharactersInPluginName(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	// Plugin names should be alphanumeric + hyphens per spec
	manager.AddPlugin("valid-plugin-123", pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "valid-plugin-123")

	require.NoError(t, err)
}

func TestPluginService_VeryLongPluginName(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	longName := "very-long-plugin-name-that-exceeds-normal-length-expectations-" +
		"and-keeps-going-for-a-while-to-test-boundary-conditions"
	manager.AddPlugin(longName, pluginmodel.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), longName)

	// Should handle long names gracefully
	require.NoError(t, err)
}

func TestPluginService_NilManager_DiscoverPlugins(t *testing.T) {
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(nil, stateStore, logger)

	plugins, err := svc.DiscoverPlugins(context.Background())

	require.NoError(t, err)
	assert.Nil(t, plugins)
}

func TestPluginService_NilStateStore_IsPluginEnabled(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, nil, logger)

	enabled := svc.IsPluginEnabled("test-plugin")

	// Default: enabled when no state store
	assert.True(t, enabled)
}

// TestPluginService_ShutdownPlugin_ContextCanceled tests ShutdownPlugin with canceled context
func TestPluginService_ShutdownPlugin_ContextCanceled(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := svc.ShutdownPlugin(ctx, "test-plugin")

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestPluginService_ShutdownPlugin_ManagerError tests ShutdownPlugin when manager returns error
func TestPluginService_ShutdownPlugin_ManagerError(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)
	shutdownErr := errors.New("shutdown hardware failure")
	manager.SetShutdownFunc(func(ctx context.Context, name string) error {
		return shutdownErr
	})

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownPlugin(context.Background(), "test-plugin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown hardware failure")
	assert.ErrorIs(t, err, shutdownErr)
}

// TestPluginService_EnablePlugin_ContextCanceled tests EnablePlugin with canceled context
func TestPluginService_EnablePlugin_ContextCanceled(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusDisabled)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before calling EnablePlugin

	err := svc.EnablePlugin(ctx, "test-plugin")

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestPluginService_DisablePlugin_ContextCanceled tests DisablePlugin with canceled context
func TestPluginService_DisablePlugin_ContextCanceled(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before calling DisablePlugin

	err := svc.DisablePlugin(ctx, "test-plugin")

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestPluginService_DisablePlugin_ShutdownError tests DisablePlugin when shutdown during disable fails
func TestPluginService_DisablePlugin_ShutdownError(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)
	shutdownErr := errors.New("plugin shutdown failed")
	manager.SetShutdownFunc(func(ctx context.Context, name string) error {
		return shutdownErr
	})

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", true)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.DisablePlugin(context.Background(), "test-plugin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown plugin")
	assert.ErrorIs(t, err, shutdownErr)
	// State should remain enabled since shutdown failed
	assert.True(t, stateStore.IsEnabled("test-plugin"))
}

// TestPluginService_DisablePlugin_SetEnabledError tests DisablePlugin when SetEnabled fails
func TestPluginService_DisablePlugin_SetEnabledError(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("test-plugin", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", true)
	persistErr := errors.New("database connection lost")
	stateStore.setEnabledErr = persistErr

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.DisablePlugin(context.Background(), "test-plugin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "disable plugin")
	assert.ErrorIs(t, err, persistErr)
	// Plugin should be stopped even if state persistence failed
	info, found := manager.Get("test-plugin")
	require.True(t, found)
	assert.Equal(t, pluginmodel.StatusStopped, info.Status)
}

// TestPluginService_SetPluginConfig_ContextCanceled tests SetPluginConfig with canceled context
func TestPluginService_SetPluginConfig_ContextCanceled(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before calling SetPluginConfig

	config := map[string]any{"key": "value"}
	err := svc.SetPluginConfig(ctx, "test-plugin", config)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestPluginService_SetPluginConfig_SetConfigError tests SetPluginConfig when SetConfig fails
func TestPluginService_SetPluginConfig_SetConfigError(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	configErr := errors.New("disk write failed")
	stateStore.setConfigErr = configErr

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	config := map[string]any{"timeout": 30}
	err := svc.SetPluginConfig(context.Background(), "test-plugin", config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "set config for plugin")
	assert.ErrorIs(t, err, configErr)
}

// T003: RegisterBuiltin and isKnownPlugin tests

func TestPluginService_RegisterBuiltin_HappyPath(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	svc.RegisterBuiltin("github", "GitHub operation provider", "1.0.0", []string{"create_issue", "list_repos"})

	info, found := svc.GetPlugin("github")

	require.True(t, found)
	require.NotNil(t, info)
	assert.Equal(t, "github", info.Manifest.Name)
	assert.Equal(t, pluginmodel.PluginTypeBuiltin, info.Type)
	assert.Len(t, info.Operations, 2)
}

func TestPluginService_RegisterBuiltin_Idempotency(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	svc.RegisterBuiltin("http", "HTTP operation provider", "1.0.0", []string{"request"})
	svc.RegisterBuiltin("http", "HTTP operation provider", "1.0.0", []string{"request"})

	plugins := svc.ListPlugins()

	httpPlugins := 0
	for _, p := range plugins {
		if p.Manifest.Name == "http" {
			httpPlugins++
		}
	}
	assert.Equal(t, 1, httpPlugins, "RegisterBuiltin should be idempotent, no duplicates")
}

func TestPluginService_ListPlugins_MergesBuiltins(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("external-plugin", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	svc.RegisterBuiltin("github", "GitHub provider", "1.0.0", []string{"create_issue"})
	svc.RegisterBuiltin("notify", "Notification provider", "1.0.0", []string{"send_message"})

	plugins := svc.ListPlugins()

	assert.Len(t, plugins, 3)

	names := make(map[string]bool)
	for _, p := range plugins {
		names[p.Manifest.Name] = true
	}
	assert.True(t, names["github"])
	assert.True(t, names["notify"])
	assert.True(t, names["external-plugin"])
}

func TestPluginService_GetPlugin_ReturnsBuiltin(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	svc.RegisterBuiltin("http", "HTTP provider", "1.0.0", []string{"request", "get", "post"})

	info, found := svc.GetPlugin("http")

	require.True(t, found)
	assert.Equal(t, "http", info.Manifest.Name)
	assert.Equal(t, pluginmodel.PluginTypeBuiltin, info.Type)
	assert.Len(t, info.Operations, 3)
}

func TestPluginService_ListEnabledPlugins_IncludesBuiltins(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("external", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	svc.RegisterBuiltin("github", "GitHub provider", "1.0.0", []string{"create_issue"})
	svc.RegisterBuiltin("notify", "Notification provider", "1.0.0", []string{"send_message"})

	enabled := svc.ListEnabledPlugins()

	assert.Len(t, enabled, 3)

	names := make(map[string]bool)
	for _, p := range enabled {
		names[p.Manifest.Name] = true
	}
	assert.True(t, names["github"])
	assert.True(t, names["notify"])
	assert.True(t, names["external"])
}

func TestPluginService_ListEnabledPlugins_ExcludesDisabledBuiltin(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	svc.RegisterBuiltin("github", "GitHub provider", "1.0.0", []string{"create_issue"})
	svc.RegisterBuiltin("http", "HTTP provider", "1.0.0", []string{"request"})

	ctx := context.Background()
	err := svc.DisablePlugin(ctx, "http")
	require.NoError(t, err)

	enabled := svc.ListEnabledPlugins()

	names := make(map[string]bool)
	for _, p := range enabled {
		names[p.Manifest.Name] = true
	}
	assert.True(t, names["github"])
	assert.False(t, names["http"])
}

func TestPluginService_EnablePlugin_UnknownName_Error(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	ctx := context.Background()
	err := svc.EnablePlugin(ctx, "unknown-plugin")

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrUnknownPlugin)
}

func TestPluginService_DisablePlugin_UnknownName_Error(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	ctx := context.Background()
	err := svc.DisablePlugin(ctx, "unknown-plugin")

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrUnknownPlugin)
}

func TestPluginService_EnablePlugin_KnownBuiltin_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	svc.RegisterBuiltin("github", "GitHub provider", "1.0.0", []string{"create_issue"})

	ctx := context.Background()
	err := svc.DisablePlugin(ctx, "github")
	require.NoError(t, err)

	err = svc.EnablePlugin(ctx, "github")

	require.NoError(t, err)
	assert.True(t, svc.IsPluginEnabled("github"))
}

func TestPluginService_DisablePlugin_KnownBuiltin_Success(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	svc.RegisterBuiltin("notify", "Notification provider", "1.0.0", []string{"send_message"})

	ctx := context.Background()
	err := svc.DisablePlugin(ctx, "notify")

	require.NoError(t, err)
	assert.False(t, svc.IsPluginEnabled("notify"))
}

func TestPluginService_isKnownPlugin_Builtin(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	svc.RegisterBuiltin("github", "GitHub provider", "1.0.0", []string{"create_issue"})

	info, found := svc.GetPlugin("github")

	require.True(t, found)
	assert.Equal(t, "github", info.Manifest.Name)
}

func TestPluginService_isKnownPlugin_External(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	manager.AddPlugin("external-plugin", pluginmodel.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	info, found := svc.GetPlugin("external-plugin")

	require.True(t, found)
	assert.Equal(t, "external-plugin", info.Manifest.Name)
}

func TestPluginService_GetPlugin_UnknownReturnsNotFound(t *testing.T) {
	manager := mocks.NewMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	info, found := svc.GetPlugin("unknown-plugin")

	assert.False(t, found)
	assert.Nil(t, info)
}
