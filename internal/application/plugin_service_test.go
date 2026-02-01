package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
)

// mockPluginManager implements ports.PluginManager for testing.
type mockPluginManager struct {
	plugins       map[string]*plugin.PluginInfo
	discoverFunc  func(ctx context.Context) ([]*plugin.PluginInfo, error)
	loadFunc      func(ctx context.Context, name string) error
	initFunc      func(ctx context.Context, name string, config map[string]any) error
	shutdownFunc  func(ctx context.Context, name string) error
	shutdownError error
}

func newMockPluginManager() *mockPluginManager {
	return &mockPluginManager{
		plugins: make(map[string]*plugin.PluginInfo),
	}
}

func (m *mockPluginManager) Discover(ctx context.Context) ([]*plugin.PluginInfo, error) {
	if m.discoverFunc != nil {
		return m.discoverFunc(ctx)
	}
	result := make([]*plugin.PluginInfo, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result, nil
}

func (m *mockPluginManager) Load(ctx context.Context, name string) error {
	if m.loadFunc != nil {
		return m.loadFunc(ctx, name)
	}
	if _, ok := m.plugins[name]; !ok {
		return errors.New("plugin not found")
	}
	m.plugins[name].Status = plugin.StatusLoaded
	return nil
}

func (m *mockPluginManager) Init(ctx context.Context, name string, config map[string]any) error {
	if m.initFunc != nil {
		return m.initFunc(ctx, name, config)
	}
	if _, ok := m.plugins[name]; !ok {
		return errors.New("plugin not found")
	}
	m.plugins[name].Status = plugin.StatusRunning
	return nil
}

func (m *mockPluginManager) Shutdown(ctx context.Context, name string) error {
	if m.shutdownFunc != nil {
		return m.shutdownFunc(ctx, name)
	}
	if info, ok := m.plugins[name]; ok {
		info.Status = plugin.StatusStopped
	}
	return nil
}

func (m *mockPluginManager) ShutdownAll(ctx context.Context) error {
	if m.shutdownError != nil {
		return m.shutdownError
	}
	for _, info := range m.plugins {
		if info.Status == plugin.StatusRunning {
			info.Status = plugin.StatusStopped
		}
	}
	return nil
}

func (m *mockPluginManager) Get(name string) (*plugin.PluginInfo, bool) {
	info, ok := m.plugins[name]
	return info, ok
}

func (m *mockPluginManager) List() []*plugin.PluginInfo {
	result := make([]*plugin.PluginInfo, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result
}

func (m *mockPluginManager) addPlugin(name string, status plugin.PluginStatus) *plugin.PluginInfo {
	info := &plugin.PluginInfo{
		Manifest: &plugin.Manifest{
			Name:         name,
			Version:      "1.0.0",
			AWFVersion:   ">=0.4.0",
			Capabilities: []string{plugin.CapabilityOperations},
		},
		Status: status,
		Path:   "/plugins/" + name,
	}
	m.plugins[name] = info
	return info
}

// mockPluginStore implements ports.PluginStore for testing.
type mockPluginStore struct {
	mu       sync.RWMutex
	states   map[string]*plugin.PluginState
	saveFunc func(ctx context.Context) error
	loadFunc func(ctx context.Context) error
	saveErr  error
	loadErr  error
}

func newMockPluginStore() *mockPluginStore {
	return &mockPluginStore{
		states: make(map[string]*plugin.PluginState),
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

func (m *mockPluginStore) GetState(name string) *plugin.PluginState {
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
	states        map[string]*plugin.PluginState
	setEnabledErr error
}

func newMockPluginConfig() *mockPluginConfig {
	return &mockPluginConfig{
		states: make(map[string]*plugin.PluginState),
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
		state = plugin.NewPluginState()
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
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.states[name]
	if !ok {
		state = plugin.NewPluginState()
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
	state := plugin.NewPluginState()
	state.Enabled = enabled
	m.mockPluginStore.states[name] = state
}

func (m *mockPluginStateStore) setPluginConfig(name string, config map[string]any) {
	m.mockPluginStore.mu.Lock()
	defer m.mockPluginStore.mu.Unlock()
	state, ok := m.mockPluginStore.states[name]
	if !ok {
		state = plugin.NewPluginState()
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

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewPluginService(t *testing.T) {
	manager := newMockPluginManager()
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

// =============================================================================
// Discovery Tests
// =============================================================================

func TestPluginService_DiscoverPlugins_Success(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("plugin-a", plugin.StatusDiscovered)
	manager.addPlugin("plugin-b", plugin.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins, err := svc.DiscoverPlugins(context.Background())

	require.NoError(t, err)
	assert.Len(t, plugins, 2)
}

func TestPluginService_DiscoverPlugins_EmptyDirectory(t *testing.T) {
	manager := newMockPluginManager() // No plugins
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins, err := svc.DiscoverPlugins(context.Background())

	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestPluginService_DiscoverPlugins_FilterDisabled(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("enabled-plugin", plugin.StatusDiscovered)
	manager.addPlugin("disabled-plugin", plugin.StatusDiscovered)

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
	manager := newMockPluginManager()
	manager.discoverFunc = func(ctx context.Context) ([]*plugin.PluginInfo, error) {
		return nil, errors.New("discovery failed")
	}

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins, err := svc.DiscoverPlugins(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "discovery failed")
	assert.Nil(t, plugins)
}

func TestPluginService_DiscoverPlugins_ContextCanceled(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.DiscoverPlugins(ctx)

	assert.ErrorIs(t, err, context.Canceled)
}

// =============================================================================
// Load Plugin Tests
// =============================================================================

func TestPluginService_LoadPlugin_Success(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	info, found := manager.Get("test-plugin")
	require.True(t, found)
	assert.Equal(t, plugin.StatusLoaded, info.Status)
}

func TestPluginService_LoadPlugin_NotFound(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found")
}

func TestPluginService_LoadPlugin_Disabled(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("disabled-plugin", plugin.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("disabled-plugin", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "disabled-plugin")

	assert.ErrorIs(t, err, application.ErrPluginDisabled)
}

func TestPluginService_LoadPlugin_AlreadyLoaded(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusLoaded)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "test-plugin")

	// Should be idempotent (no error)
	require.NoError(t, err)
}

func TestPluginService_LoadPlugin_EmptyName(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "")

	assert.ErrorIs(t, err, application.ErrPluginNameEmpty)
}

// =============================================================================
// Init Plugin Tests
// =============================================================================

func TestPluginService_InitPlugin_Success(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusLoaded)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginConfig("test-plugin", map[string]any{"key": "value"})

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.InitPlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	info, found := manager.Get("test-plugin")
	require.True(t, found)
	assert.Equal(t, plugin.StatusRunning, info.Status)
}

func TestPluginService_InitPlugin_NotLoaded(t *testing.T) {
	manager := newMockPluginManager()
	// Plugin discovered but not loaded - mock returns error for init on non-loaded
	manager.addPlugin("test-plugin", plugin.StatusDiscovered)
	manager.initFunc = func(ctx context.Context, name string, config map[string]any) error {
		return errors.New("plugin must be loaded first")
	}

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.InitPlugin(context.Background(), "test-plugin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin must be loaded first")
}

func TestPluginService_InitPlugin_AlreadyRunning(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.InitPlugin(context.Background(), "test-plugin")

	// Should be idempotent (no error) - mock init just sets to Running
	require.NoError(t, err)
}

func TestPluginService_InitPlugin_WithStoredConfig(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusLoaded)

	var capturedConfig map[string]any
	manager.initFunc = func(ctx context.Context, name string, config map[string]any) error {
		capturedConfig = config
		return nil
	}

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

// =============================================================================
// Load and Init Plugin Tests
// =============================================================================

func TestPluginService_LoadAndInitPlugin_Success(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadAndInitPlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	info, found := manager.Get("test-plugin")
	require.True(t, found)
	assert.Equal(t, plugin.StatusRunning, info.Status)
}

func TestPluginService_LoadAndInitPlugin_LoadFails(t *testing.T) {
	manager := newMockPluginManager()
	manager.loadFunc = func(ctx context.Context, name string) error {
		return errors.New("load failed")
	}

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadAndInitPlugin(context.Background(), "test-plugin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "load failed")
}

func TestPluginService_LoadAndInitPlugin_InitFails(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusDiscovered)
	manager.initFunc = func(ctx context.Context, name string, config map[string]any) error {
		return errors.New("init failed")
	}

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadAndInitPlugin(context.Background(), "test-plugin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "init failed")
}

// =============================================================================
// Shutdown Plugin Tests
// =============================================================================

func TestPluginService_ShutdownPlugin_Success(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownPlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	info, found := manager.Get("test-plugin")
	require.True(t, found)
	assert.Equal(t, plugin.StatusStopped, info.Status)
}

func TestPluginService_ShutdownPlugin_NotRunning(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusStopped)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownPlugin(context.Background(), "test-plugin")

	// Should be idempotent (no error)
	require.NoError(t, err)
}

func TestPluginService_ShutdownPlugin_NotFound(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownPlugin(context.Background(), "nonexistent")

	// Should handle gracefully (no error for missing plugin)
	require.NoError(t, err)
}

// =============================================================================
// Shutdown All Tests
// =============================================================================

func TestPluginService_ShutdownAll_Success(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("plugin-a", plugin.StatusRunning)
	manager.addPlugin("plugin-b", plugin.StatusRunning)
	manager.addPlugin("plugin-c", plugin.StatusStopped) // Already stopped

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownAll(context.Background())

	require.NoError(t, err)
	infoA, _ := manager.Get("plugin-a")
	assert.Equal(t, plugin.StatusStopped, infoA.Status)
	infoB, _ := manager.Get("plugin-b")
	assert.Equal(t, plugin.StatusStopped, infoB.Status)
}

func TestPluginService_ShutdownAll_NoPlugins(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownAll(context.Background())

	require.NoError(t, err)
}

func TestPluginService_ShutdownAll_PartialFailure(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("plugin-a", plugin.StatusRunning)
	manager.addPlugin("plugin-b", plugin.StatusRunning)
	manager.shutdownError = errors.New("shutdown failed for one plugin")

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.ShutdownAll(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown failed")
}

// =============================================================================
// Enable Plugin Tests
// =============================================================================

func TestPluginService_EnablePlugin_Success(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusDisabled)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.EnablePlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	assert.True(t, stateStore.IsEnabled("test-plugin"))
}

func TestPluginService_EnablePlugin_AlreadyEnabled(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusRunning)

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
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusDisabled)

	stateStore := newMockPluginStateStore()
	stateStore.setEnabledErr = errors.New("save failed")

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.EnablePlugin(context.Background(), "test-plugin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "save failed")
}

// =============================================================================
// Disable Plugin Tests
// =============================================================================

func TestPluginService_DisablePlugin_Success(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusRunning)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", true)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.DisablePlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	assert.False(t, stateStore.IsEnabled("test-plugin"))
}

func TestPluginService_DisablePlugin_AlreadyDisabled(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusDisabled)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.DisablePlugin(context.Background(), "test-plugin")

	// Should be idempotent
	require.NoError(t, err)
}

func TestPluginService_DisablePlugin_ShutdownsRunning(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.DisablePlugin(context.Background(), "test-plugin")

	require.NoError(t, err)
	// Plugin should be shut down
	info, found := manager.Get("test-plugin")
	require.True(t, found)
	assert.Equal(t, plugin.StatusStopped, info.Status)
}

// =============================================================================
// Config Management Tests
// =============================================================================

func TestPluginService_SetPluginConfig_Success(t *testing.T) {
	manager := newMockPluginManager()
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
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.SetPluginConfig(context.Background(), "test-plugin", nil)

	// Should handle nil config (clear config)
	require.NoError(t, err)
}

func TestPluginService_GetPluginConfig_Exists(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	stateStore.setPluginConfig("test-plugin", map[string]any{"key": "value"})

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	config := svc.GetPluginConfig("test-plugin")

	assert.Equal(t, "value", config["key"])
}

func TestPluginService_GetPluginConfig_NotExists(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	config := svc.GetPluginConfig("nonexistent")

	assert.Nil(t, config)
}

// =============================================================================
// Get/List Plugin Tests
// =============================================================================

func TestPluginService_GetPlugin_Exists(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusRunning)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	info, found := svc.GetPlugin("test-plugin")

	assert.True(t, found)
	require.NotNil(t, info)
	assert.Equal(t, "test-plugin", info.Manifest.Name)
}

func TestPluginService_GetPlugin_NotExists(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	info, found := svc.GetPlugin("nonexistent")

	assert.False(t, found)
	assert.Nil(t, info)
}

func TestPluginService_ListPlugins(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("plugin-a", plugin.StatusRunning)
	manager.addPlugin("plugin-b", plugin.StatusStopped)
	manager.addPlugin("plugin-c", plugin.StatusDisabled)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins := svc.ListPlugins()

	assert.Len(t, plugins, 3)
}

func TestPluginService_ListPlugins_Empty(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins := svc.ListPlugins()

	assert.Empty(t, plugins)
}

func TestPluginService_ListEnabledPlugins(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("enabled-a", plugin.StatusRunning)
	manager.addPlugin("enabled-b", plugin.StatusLoaded)
	manager.addPlugin("disabled", plugin.StatusDisabled)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("disabled", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	plugins := svc.ListEnabledPlugins()

	assert.Len(t, plugins, 2)
}

func TestPluginService_ListDisabledPlugins(t *testing.T) {
	manager := newMockPluginManager()

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
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", true)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	enabled := svc.IsPluginEnabled("test-plugin")

	assert.True(t, enabled)
}

func TestPluginService_IsPluginEnabled_False(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("test-plugin", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	enabled := svc.IsPluginEnabled("test-plugin")

	assert.False(t, enabled)
}

func TestPluginService_IsPluginEnabled_DefaultTrue(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore() // No explicit state
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	enabled := svc.IsPluginEnabled("unknown-plugin")

	// Default: enabled for unknown plugins
	assert.True(t, enabled)
}

// =============================================================================
// State Persistence Tests
// =============================================================================

func TestPluginService_SaveState_Success(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.SaveState(context.Background())

	require.NoError(t, err)
}

func TestPluginService_SaveState_Error(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	stateStore.saveErr = errors.New("disk full")

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.SaveState(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}

func TestPluginService_LoadState_Success(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadState(context.Background())

	require.NoError(t, err)
}

func TestPluginService_LoadState_Error(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	stateStore.loadErr = errors.New("file not found")

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadState(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}

// =============================================================================
// Startup Tests
// =============================================================================

func TestPluginService_StartupEnabledPlugins_Success(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("plugin-a", plugin.StatusDiscovered)
	manager.addPlugin("plugin-b", plugin.StatusDiscovered)
	manager.addPlugin("plugin-disabled", plugin.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	stateStore.setPluginEnabled("plugin-disabled", false)

	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.StartupEnabledPlugins(context.Background())

	require.NoError(t, err)
	// Enabled plugins should be running
	infoA, _ := manager.Get("plugin-a")
	assert.Equal(t, plugin.StatusRunning, infoA.Status)
	infoB, _ := manager.Get("plugin-b")
	assert.Equal(t, plugin.StatusRunning, infoB.Status)
	// Disabled plugin should not be started
	infoDisabled, _ := manager.Get("plugin-disabled")
	assert.Equal(t, plugin.StatusDiscovered, infoDisabled.Status)
}

func TestPluginService_StartupEnabledPlugins_NoPlugins(t *testing.T) {
	manager := newMockPluginManager()
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.StartupEnabledPlugins(context.Background())

	// Should handle empty plugins gracefully
	require.NoError(t, err)
}

func TestPluginService_StartupEnabledPlugins_PartialFailure(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("plugin-ok", plugin.StatusDiscovered)
	manager.addPlugin("plugin-fail", plugin.StatusDiscovered)

	manager.initFunc = func(ctx context.Context, name string, config map[string]any) error {
		if name == "plugin-fail" {
			return errors.New("init failed")
		}
		return nil
	}

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.StartupEnabledPlugins(context.Background())

	// Should continue with other plugins and aggregate errors
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init failed")
}

func TestPluginService_StartupEnabledPlugins_ContextCanceled(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("plugin-a", plugin.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.StartupEnabledPlugins(ctx)

	// Should respect context cancellation
	assert.ErrorIs(t, err, context.Canceled)
}

// =============================================================================
// Edge Cases and Boundary Conditions
// =============================================================================

func TestPluginService_ConcurrentOperations(t *testing.T) {
	manager := newMockPluginManager()
	manager.addPlugin("test-plugin", plugin.StatusDiscovered)

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
	manager := newMockPluginManager()
	// Plugin names should be alphanumeric + hyphens per spec
	manager.addPlugin("valid-plugin-123", plugin.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), "valid-plugin-123")

	require.NoError(t, err)
}

func TestPluginService_VeryLongPluginName(t *testing.T) {
	manager := newMockPluginManager()
	longName := "very-long-plugin-name-that-exceeds-normal-length-expectations-" +
		"and-keeps-going-for-a-while-to-test-boundary-conditions"
	manager.addPlugin(longName, plugin.StatusDiscovered)

	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, stateStore, logger)

	err := svc.LoadPlugin(context.Background(), longName)

	// Should handle long names gracefully
	require.NoError(t, err)
}

// =============================================================================
// Nil Dependencies Tests
// =============================================================================

func TestPluginService_NilManager_DiscoverPlugins(t *testing.T) {
	stateStore := newMockPluginStateStore()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(nil, stateStore, logger)

	plugins, err := svc.DiscoverPlugins(context.Background())

	require.NoError(t, err)
	assert.Nil(t, plugins)
}

func TestPluginService_NilStateStore_IsPluginEnabled(t *testing.T) {
	manager := newMockPluginManager()
	logger := newMockPluginLogger()

	svc := application.NewPluginService(manager, nil, logger)

	enabled := svc.IsPluginEnabled("test-plugin")

	// Default: enabled when no state store
	assert.True(t, enabled)
}
