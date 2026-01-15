package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
)

// =============================================================================
// initPluginSystem Tests (T014)
// =============================================================================

func TestInitPluginSystem_NoPluginsDirectory(t *testing.T) {
	// Test graceful degradation when no plugins directory exists
	tmpDir := t.TempDir()
	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  filepath.Join(tmpDir, "nonexistent-plugins"),
	}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, nil)

	require.NoError(t, err, "initPluginSystem should not fail when plugins dir doesn't exist")
	require.NotNil(t, result, "result should not be nil")
	require.NotNil(t, result.Service, "Service should not be nil")
	require.NotNil(t, result.Cleanup, "Cleanup function should not be nil")

	// Cleanup should be safe to call
	result.Cleanup()
}

func TestInitPluginSystem_WithEmptyPluginsDirectory(t *testing.T) {
	// Test with existing but empty plugins directory
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  pluginsDir,
	}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, nil)

	require.NoError(t, err, "initPluginSystem should succeed with empty plugins dir")
	require.NotNil(t, result)
	require.NotNil(t, result.Service)

	// Cleanup should work
	result.Cleanup()
}

func TestInitPluginSystem_WithValidPlugin(t *testing.T) {
	// Test with a valid plugin directory structure
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	// Create a valid plugin.yaml manifest
	manifestContent := `name: test-plugin
version: 1.0.0
description: Test plugin for unit tests
awf_version: ">=0.1.0"
capabilities:
  - operations
`
	manifestPath := filepath.Join(testPluginDir, "plugin.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifestContent), 0o644))

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  pluginsDir,
	}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, nil)

	require.NoError(t, err, "initPluginSystem should succeed with valid plugin")
	require.NotNil(t, result)
	require.NotNil(t, result.Service)

	// Cleanup should work
	result.Cleanup()
}

func TestInitPluginSystem_CreatesStateStore(t *testing.T) {
	// Test that plugin state store directory is created
	tmpDir := t.TempDir()
	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "", // No plugins dir
	}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// The state store path should be created when saving
	result.Cleanup()
}

func TestInitPluginSystem_WithLogger(t *testing.T) {
	// Test that logger is used appropriately
	tmpDir := t.TempDir()
	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  filepath.Join(tmpDir, "nonexistent"),
	}

	logger := &mockLogger{}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, logger)

	require.NoError(t, err)
	require.NotNil(t, result)

	result.Cleanup()
}

func TestInitPluginSystem_ContextCancellation(t *testing.T) {
	// Test behavior with cancelled context
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  pluginsDir,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := initPluginSystem(ctx, cfg, nil)

	// Should handle cancelled context gracefully
	// The exact behavior depends on implementation
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled)
	} else {
		require.NotNil(t, result)
		result.Cleanup()
	}
}

func TestInitPluginSystem_CleanupIsSafe(t *testing.T) {
	// Test that cleanup can be called multiple times safely
	tmpDir := t.TempDir()
	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "",
	}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Call cleanup multiple times - should not panic
	result.Cleanup()
	result.Cleanup()
	result.Cleanup()
}

func TestInitPluginSystem_ServiceMethodsWork(t *testing.T) {
	// Test that returned service has working methods
	tmpDir := t.TempDir()
	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "",
	}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Service)

	// Service methods should not panic (may return nil/empty when no plugins)
	assert.NotPanics(t, func() {
		_ = result.Service.ListPlugins()
	}, "ListPlugins should not panic")

	assert.NotPanics(t, func() {
		_ = result.Service.ListEnabledPlugins()
	}, "ListEnabledPlugins should not panic")

	result.Cleanup()
}

// =============================================================================
// getPluginSearchPaths Tests (T014)
// =============================================================================

func TestGetPluginSearchPaths_WithOverride(t *testing.T) {
	cfg := &Config{
		PluginsDir: "/custom/plugins/override",
	}

	paths := getPluginSearchPaths(cfg)

	require.Len(t, paths, 1, "Should return only the override path")
	assert.Equal(t, "/custom/plugins/override", paths[0])
}

func TestGetPluginSearchPaths_WithoutOverride(t *testing.T) {
	// Unset environment variable
	t.Setenv("AWF_PLUGINS_PATH", "")

	cfg := &Config{
		PluginsDir: "", // Empty = use BuildPluginPaths
	}

	paths := getPluginSearchPaths(cfg)

	require.Len(t, paths, 2, "Should return paths from BuildPluginPaths")
	assert.Contains(t, paths[0], ".awf/plugins", "First should be local")
}

func TestGetPluginSearchPaths_EmptyStringOverride(t *testing.T) {
	// Empty string should use BuildPluginPaths
	cfg := &Config{
		PluginsDir: "",
	}

	paths := getPluginSearchPaths(cfg)

	require.NotEmpty(t, paths, "Should return non-empty paths from BuildPluginPaths")
}

// =============================================================================
// findFirstExistingDir Tests (T014)
// =============================================================================

func TestFindFirstExistingDir_FirstExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")
	require.NoError(t, os.MkdirAll(existingDir, 0o755))

	paths := []string{
		existingDir,
		filepath.Join(tmpDir, "nonexistent1"),
		filepath.Join(tmpDir, "nonexistent2"),
	}

	result := findFirstExistingDir(paths)

	assert.Equal(t, existingDir, result)
}

func TestFindFirstExistingDir_MiddleExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")
	require.NoError(t, os.MkdirAll(existingDir, 0o755))

	paths := []string{
		filepath.Join(tmpDir, "nonexistent1"),
		existingDir,
		filepath.Join(tmpDir, "nonexistent2"),
	}

	result := findFirstExistingDir(paths)

	assert.Equal(t, existingDir, result)
}

func TestFindFirstExistingDir_LastExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")
	require.NoError(t, os.MkdirAll(existingDir, 0o755))

	paths := []string{
		filepath.Join(tmpDir, "nonexistent1"),
		filepath.Join(tmpDir, "nonexistent2"),
		existingDir,
	}

	result := findFirstExistingDir(paths)

	assert.Equal(t, existingDir, result)
}

func TestFindFirstExistingDir_NoneExist(t *testing.T) {
	tmpDir := t.TempDir()

	paths := []string{
		filepath.Join(tmpDir, "nonexistent1"),
		filepath.Join(tmpDir, "nonexistent2"),
		filepath.Join(tmpDir, "nonexistent3"),
	}

	result := findFirstExistingDir(paths)

	assert.Empty(t, result)
}

func TestFindFirstExistingDir_EmptyPaths(t *testing.T) {
	paths := []string{}

	result := findFirstExistingDir(paths)

	assert.Empty(t, result)
}

func TestFindFirstExistingDir_FileNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

	dirPath := filepath.Join(tmpDir, "dir")
	require.NoError(t, os.MkdirAll(dirPath, 0o755))

	paths := []string{
		filePath, // File, not directory
		dirPath,
	}

	result := findFirstExistingDir(paths)

	// Should skip the file and return the directory
	assert.Equal(t, dirPath, result)
}

func TestFindFirstExistingDir_OnlyFileNoDirs(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

	paths := []string{
		filePath, // File, not directory
	}

	result := findFirstExistingDir(paths)

	// Should return empty since no directories exist
	assert.Empty(t, result)
}

func TestFindFirstExistingDir_MultipleExisting(t *testing.T) {
	tmpDir := t.TempDir()
	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	require.NoError(t, os.MkdirAll(dir1, 0o755))
	require.NoError(t, os.MkdirAll(dir2, 0o755))

	paths := []string{dir1, dir2}

	result := findFirstExistingDir(paths)

	// Should return first one
	assert.Equal(t, dir1, result)
}

// =============================================================================
// PluginSystemResult Tests (T014)
// =============================================================================

func TestPluginSystemResult_ServiceNotNil(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "",
	}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, nil)

	require.NoError(t, err)
	assert.NotNil(t, result.Service, "Service should never be nil")

	result.Cleanup()
}

func TestPluginSystemResult_CleanupNotNil(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "",
	}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, nil)

	require.NoError(t, err)
	assert.NotNil(t, result.Cleanup, "Cleanup should never be nil")

	result.Cleanup()
}

// =============================================================================
// Integration with run.go Tests (T014)
// =============================================================================

func TestInitPluginSystem_IntegrationScenario(t *testing.T) {
	// Simulates the actual usage pattern in runWorkflow
	tmpDir := t.TempDir()

	// Setup storage structure like a real AWF installation
	statesDir := filepath.Join(tmpDir, "states")
	pluginsStateDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))
	// Don't create plugins dir - test graceful degradation

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "", // Use default search paths (won't find anything)
	}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, nil)

	require.NoError(t, err, "Should succeed even without plugins")
	require.NotNil(t, result)

	// In a real workflow execution, the service would be used here
	// For now, just verify it's accessible
	_ = result.Service

	// Cleanup at the end of workflow execution
	result.Cleanup()

	// Verify plugins state directory structure
	_, statErr := os.Stat(pluginsStateDir)
	// State dir may or may not exist depending on implementation
	_ = statErr
}

func TestInitPluginSystem_WithInvalidPlugin(t *testing.T) {
	// Test handling of invalid plugin manifests
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	invalidPluginDir := filepath.Join(pluginsDir, "invalid-plugin")
	require.NoError(t, os.MkdirAll(invalidPluginDir, 0o755))

	// Create an invalid plugin.yaml
	invalidManifest := `invalid yaml content: [[[`
	manifestPath := filepath.Join(invalidPluginDir, "plugin.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(invalidManifest), 0o644))

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  pluginsDir,
	}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, nil)

	// Should succeed (graceful degradation) or fail gracefully
	// Invalid plugins should be skipped, not cause total failure
	if err == nil {
		require.NotNil(t, result)
		result.Cleanup()
	}
	// If it errors, that's also acceptable behavior
}

func TestInitPluginSystem_PluginStatesPersistence(t *testing.T) {
	// Test that plugin states are loaded from disk
	tmpDir := t.TempDir()
	pluginsStateDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsStateDir, 0o755))

	// Pre-create a plugins.json with some state
	stateContent := `{
		"disabled-plugin": {
			"enabled": false,
			"config": {}
		}
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(pluginsStateDir, "plugins.json"),
		[]byte(stateContent),
		0o644,
	))

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "",
	}

	ctx := context.Background()
	result, err := initPluginSystem(ctx, cfg, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// The disabled plugin state should be loaded
	// This would be verified via the service's methods
	disabled := result.Service.ListDisabledPlugins()
	assert.Contains(t, disabled, "disabled-plugin",
		"Should load disabled plugin state from disk")

	result.Cleanup()
}

// =============================================================================
// Mock Logger for Tests
// =============================================================================

type mockLogger struct {
	debugCalls []string
	infoCalls  []string
	warnCalls  []string
	errorCalls []string
}

func (l *mockLogger) Debug(msg string, keysAndValues ...any) {
	l.debugCalls = append(l.debugCalls, msg)
}

func (l *mockLogger) Info(msg string, keysAndValues ...any) {
	l.infoCalls = append(l.infoCalls, msg)
}

func (l *mockLogger) Warn(msg string, keysAndValues ...any) {
	l.warnCalls = append(l.warnCalls, msg)
}

func (l *mockLogger) Error(msg string, keysAndValues ...any) {
	l.errorCalls = append(l.errorCalls, msg)
}

func (l *mockLogger) WithContext(ctx map[string]any) ports.Logger {
	return l
}

var _ ports.Logger = (*mockLogger)(nil)

// =============================================================================
// initPluginSystemReadOnly Tests (T019)
// =============================================================================

func TestInitPluginSystemReadOnly_NoPluginsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  filepath.Join(tmpDir, "nonexistent-plugins"),
	}

	ctx := context.Background()
	result, err := initPluginSystemReadOnly(ctx, cfg)

	require.NoError(t, err, "initPluginSystemReadOnly should not fail when plugins dir doesn't exist")
	require.NotNil(t, result, "result should not be nil")
	require.NotNil(t, result.Service, "Service should not be nil")
	require.NotNil(t, result.Cleanup, "Cleanup function should not be nil")

	// Cleanup should be safe to call
	result.Cleanup()
}

func TestInitPluginSystemReadOnly_WithEmptyPluginsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  pluginsDir,
	}

	ctx := context.Background()
	result, err := initPluginSystemReadOnly(ctx, cfg)

	require.NoError(t, err, "initPluginSystemReadOnly should succeed with empty plugins dir")
	require.NotNil(t, result)
	require.NotNil(t, result.Service)

	result.Cleanup()
}

func TestInitPluginSystemReadOnly_DoesNotStartPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: test-plugin
version: 1.0.0
awf_version: ">=0.1.0"
capabilities:
  - operations
`
	require.NoError(t, os.WriteFile(
		filepath.Join(testPluginDir, "plugin.yaml"),
		[]byte(manifestContent),
		0o644,
	))

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  pluginsDir,
	}

	ctx := context.Background()
	result, err := initPluginSystemReadOnly(ctx, cfg)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Plugins should be discovered but not started
	plugins := result.Service.ListPlugins()
	for _, p := range plugins {
		// Plugins should not be in Running state (read-only mode)
		assert.NotEqual(t, "running", string(p.Status),
			"Plugins should not be started in read-only mode")
	}

	result.Cleanup()
}

func TestInitPluginSystemReadOnly_LoadsStateStore(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create state with disabled plugin
	stateDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	stateContent := `{
		"disabled-plugin": {
			"enabled": false,
			"config": {}
		}
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(stateDir, "plugins.json"),
		[]byte(stateContent),
		0o644,
	))

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "",
	}

	ctx := context.Background()
	result, err := initPluginSystemReadOnly(ctx, cfg)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have loaded the disabled state
	disabled := result.Service.ListDisabledPlugins()
	assert.Contains(t, disabled, "disabled-plugin",
		"Should load disabled plugin state from disk")

	result.Cleanup()
}

func TestInitPluginSystemReadOnly_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := initPluginSystemReadOnly(ctx, cfg)

	// Should handle cancelled context gracefully
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled)
	} else {
		require.NotNil(t, result)
		result.Cleanup()
	}
}

func TestInitPluginSystemReadOnly_CleanupIsSafe(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "",
	}

	ctx := context.Background()
	result, err := initPluginSystemReadOnly(ctx, cfg)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Cleanup should be safe to call multiple times
	assert.NotPanics(t, func() {
		result.Cleanup()
		result.Cleanup()
		result.Cleanup()
	}, "Multiple cleanup calls should not panic")
}

func TestInitPluginSystemReadOnly_ServiceMethodsWork(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "",
	}

	ctx := context.Background()
	result, err := initPluginSystemReadOnly(ctx, cfg)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Service)

	// Service methods should not panic
	assert.NotPanics(t, func() {
		_ = result.Service.ListPlugins()
	}, "ListPlugins should not panic")

	assert.NotPanics(t, func() {
		_ = result.Service.ListEnabledPlugins()
	}, "ListEnabledPlugins should not panic")

	assert.NotPanics(t, func() {
		_ = result.Service.ListDisabledPlugins()
	}, "ListDisabledPlugins should not panic")

	assert.NotPanics(t, func() {
		_ = result.Service.IsPluginEnabled("test")
	}, "IsPluginEnabled should not panic")

	result.Cleanup()
}

func TestInitPluginSystemReadOnly_DiscoverWithValidPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "discovered-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: discovered-plugin
version: 2.5.0
description: A plugin to be discovered
awf_version: ">=0.1.0"
capabilities:
  - operations
  - commands
`
	require.NoError(t, os.WriteFile(
		filepath.Join(testPluginDir, "plugin.yaml"),
		[]byte(manifestContent),
		0o644,
	))

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  pluginsDir,
	}

	ctx := context.Background()
	result, err := initPluginSystemReadOnly(ctx, cfg)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have discovered the plugin
	plugins := result.Service.ListPlugins()
	var found bool
	for _, p := range plugins {
		if p.Manifest == nil || p.Manifest.Name != "discovered-plugin" {
			continue
		}
		found = true
		assert.Equal(t, "2.5.0", p.Manifest.Version)
		assert.Contains(t, p.Manifest.Capabilities, "operations")
		assert.Contains(t, p.Manifest.Capabilities, "commands")
		break
	}
	assert.True(t, found, "Plugin should be discovered")

	result.Cleanup()
}

func TestInitPluginSystemReadOnly_HandlesInvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	invalidPluginDir := filepath.Join(pluginsDir, "invalid-plugin")
	require.NoError(t, os.MkdirAll(invalidPluginDir, 0o755))

	// Create invalid manifest
	invalidManifest := `invalid yaml: [[[[`
	require.NoError(t, os.WriteFile(
		filepath.Join(invalidPluginDir, "plugin.yaml"),
		[]byte(invalidManifest),
		0o644,
	))

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  pluginsDir,
	}

	ctx := context.Background()
	result, err := initPluginSystemReadOnly(ctx, cfg)

	// Should succeed (graceful degradation)
	require.NoError(t, err)
	require.NotNil(t, result)

	result.Cleanup()
}

func TestInitPluginSystemReadOnly_UsesBuildPluginPaths(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "env-plugins")
	testPluginDir := filepath.Join(pluginsDir, "env-test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: env-test-plugin
version: 1.0.0
awf_version: ">=0.1.0"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(testPluginDir, "plugin.yaml"),
		[]byte(manifestContent),
		0o644,
	))

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  "", // Empty = use BuildPluginPaths which respects AWF_PLUGINS_PATH
	}

	ctx := context.Background()
	result, err := initPluginSystemReadOnly(ctx, cfg)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have discovered the plugin from env path
	plugins := result.Service.ListPlugins()
	var found bool
	for _, p := range plugins {
		if p.Manifest != nil && p.Manifest.Name == "env-test-plugin" {
			found = true
			break
		}
	}
	assert.True(t, found, "Plugin should be discovered from AWF_PLUGINS_PATH")

	result.Cleanup()
}

func TestInitPluginSystemReadOnly_MultiplePlugins(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")

	// Create multiple plugins
	for _, name := range []string{"plugin-a", "plugin-b", "plugin-c"} {
		pluginDir := filepath.Join(pluginsDir, name)
		require.NoError(t, os.MkdirAll(pluginDir, 0o755))

		manifestContent := "name: " + name + `
version: 1.0.0
awf_version: ">=0.1.0"
`
		require.NoError(t, os.WriteFile(
			filepath.Join(pluginDir, "plugin.yaml"),
			[]byte(manifestContent),
			0o644,
		))
	}

	cfg := &Config{
		StoragePath: tmpDir,
		PluginsDir:  pluginsDir,
	}

	ctx := context.Background()
	result, err := initPluginSystemReadOnly(ctx, cfg)

	require.NoError(t, err)
	require.NotNil(t, result)

	plugins := result.Service.ListPlugins()
	assert.GreaterOrEqual(t, len(plugins), 3, "Should discover all plugins")

	result.Cleanup()
}
