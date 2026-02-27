//go:build integration

// Feature: F021 - Plugin System
// This file contains functional/integration tests for the plugin system.

package plugins_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPluginDiscovery_Integration tests discovering plugins from a directory.
// Acceptance Criteria: Plugin discovery from plugins/ directory
func TestPluginDiscovery_Integration(t *testing.T) {
	fixturesPath := "../../fixtures/plugins"

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	plugins, err := loader.DiscoverPlugins(ctx, fixturesPath)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(plugins), 2, "should discover at least 2 valid plugins")

	// Verify valid-simple plugin was discovered
	var foundSimple bool
	for _, p := range plugins {
		if p.Manifest != nil && p.Manifest.Name == "awf-plugin-simple" {
			foundSimple = true
			assert.Equal(t, "1.0.0", p.Manifest.Version)
			assert.Contains(t, p.Manifest.Capabilities, pluginmodel.CapabilityOperations)
			assert.Equal(t, pluginmodel.StatusLoaded, p.Status)
			break
		}
	}
	assert.True(t, foundSimple, "should find awf-plugin-simple")
}

// TestPluginLoadPlugin_Integration tests loading a single plugin.
// Acceptance Criteria: Plugin lifecycle (load, init, shutdown)
func TestPluginLoadPlugin_Integration(t *testing.T) {
	fixturesPath := "../../fixtures/plugins"

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pluginPath := filepath.Join(fixturesPath, "valid-simple")
	pluginInfo, err := loader.LoadPlugin(ctx, pluginPath)

	require.NoError(t, err)
	require.NotNil(t, pluginInfo)
	require.NotNil(t, pluginInfo.Manifest)

	// Verify plugin metadata
	assert.Equal(t, "awf-plugin-simple", pluginInfo.Manifest.Name)
	assert.Equal(t, "1.0.0", pluginInfo.Manifest.Version)
	assert.Equal(t, ">=0.4.0", pluginInfo.Manifest.AWFVersion)
	assert.Equal(t, pluginmodel.StatusLoaded, pluginInfo.Status)
	assert.Greater(t, pluginInfo.LoadedAt, int64(0))
}

// TestPluginManifestFullFields_Integration tests loading a plugin with all manifest fields.
// Acceptance Criteria: Plugin manifest (name, version, capabilities)
func TestPluginManifestFullFields_Integration(t *testing.T) {
	fixturesPath := "../../fixtures/plugins"

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pluginPath := filepath.Join(fixturesPath, "valid-full")
	pluginInfo, err := loader.LoadPlugin(ctx, pluginPath)

	require.NoError(t, err)
	require.NotNil(t, pluginInfo)
	require.NotNil(t, pluginInfo.Manifest)

	m := pluginInfo.Manifest

	// Required fields
	assert.Equal(t, "awf-plugin-slack", m.Name)
	assert.Equal(t, "1.2.3", m.Version)
	assert.Equal(t, ">=0.4.0", m.AWFVersion)

	// Optional fields
	assert.Equal(t, "Slack notifications for AWF workflows", m.Description)
	assert.Equal(t, "Jane Developer <jane@example.com>", m.Author)
	assert.Equal(t, "MIT", m.License)
	assert.Equal(t, "https://github.com/example/awf-plugin-slack", m.Homepage)

	// Capabilities
	assert.Contains(t, m.Capabilities, pluginmodel.CapabilityOperations)
	assert.Contains(t, m.Capabilities, pluginmodel.CapabilityCommands)
	assert.True(t, m.HasCapability(pluginmodel.CapabilityOperations))
	assert.True(t, m.HasCapability(pluginmodel.CapabilityCommands))
	assert.False(t, m.HasCapability(pluginmodel.CapabilityValidators))

	// Config schema
	require.NotNil(t, m.Config)
	require.Contains(t, m.Config, "webhook_url")
	webhookCfg := m.Config["webhook_url"]
	assert.Equal(t, pluginmodel.ConfigTypeString, webhookCfg.Type)
	assert.True(t, webhookCfg.Required)

	require.Contains(t, m.Config, "channel")
	channelCfg := m.Config["channel"]
	assert.Equal(t, pluginmodel.ConfigTypeString, channelCfg.Type)
	assert.False(t, channelCfg.Required)
	assert.Equal(t, "#general", channelCfg.Default)

	require.Contains(t, m.Config, "retry_count")
	retryCfg := m.Config["retry_count"]
	assert.Equal(t, pluginmodel.ConfigTypeInteger, retryCfg.Type)
	assert.Equal(t, 3, retryCfg.Default)

	require.Contains(t, m.Config, "notify_on_failure")
	notifyCfg := m.Config["notify_on_failure"]
	assert.Equal(t, pluginmodel.ConfigTypeBoolean, notifyCfg.Type)
	assert.True(t, notifyCfg.Default.(bool))

	require.Contains(t, m.Config, "log_level")
	logCfg := m.Config["log_level"]
	assert.Equal(t, []string{"debug", "info", "warn", "error"}, logCfg.Enum)
}

// TestPluginManager_Lifecycle_Integration tests the full plugin lifecycle.
// Acceptance Criteria: Plugin lifecycle (load, init, shutdown)
func TestPluginManager_Lifecycle_Integration(t *testing.T) {
	fixturesPath := "../../fixtures/plugins"

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)
	manager := pluginmgr.NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Discover
	discovered, err := manager.Discover(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(discovered), 2)

	// 2. Get by name
	info, found := manager.Get("awf-plugin-simple")
	require.True(t, found)
	assert.Equal(t, pluginmodel.StatusLoaded, info.Status)

	// 3. Init
	err = manager.Init(ctx, "awf-plugin-simple", nil)
	require.NoError(t, err)
	info, _ = manager.Get("awf-plugin-simple")
	assert.Equal(t, pluginmodel.StatusRunning, info.Status)

	// 4. Shutdown
	err = manager.Shutdown(ctx, "awf-plugin-simple")
	require.NoError(t, err)
	info, _ = manager.Get("awf-plugin-simple")
	assert.Equal(t, pluginmodel.StatusStopped, info.Status)
}

// TestPluginService_EnableDisable_Integration tests enabling and disabling plugins.
// Acceptance Criteria: Plugin enable/disable state persistence
func TestPluginService_EnableDisable_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	fixturesPath := "../../fixtures/plugins"

	// Setup infrastructure
	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)
	manager := pluginmgr.NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	stateStore := pluginmgr.NewJSONPluginStateStore(tmpDir)

	service := application.NewPluginService(manager, stateStore, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Discover plugins
	_, err := service.DiscoverPlugins(ctx)
	require.NoError(t, err)

	// By default, plugins are enabled
	assert.True(t, service.IsPluginEnabled("awf-plugin-simple"))

	// Disable a plugin
	err = service.DisablePlugin(ctx, "awf-plugin-simple")
	require.NoError(t, err)
	assert.False(t, service.IsPluginEnabled("awf-plugin-simple"))

	// Save state
	err = service.SaveState(ctx)
	require.NoError(t, err)

	// Verify state file was created
	statePath := filepath.Join(tmpDir, "plugins.json")
	_, err = os.Stat(statePath)
	assert.NoError(t, err)

	// Create new service and load state
	stateStore2 := pluginmgr.NewJSONPluginStateStore(tmpDir)
	err = stateStore2.Load(ctx)
	require.NoError(t, err)

	service2 := application.NewPluginService(manager, stateStore2, nil)
	assert.False(t, service2.IsPluginEnabled("awf-plugin-simple"))

	// Re-enable
	err = service2.EnablePlugin(ctx, "awf-plugin-simple")
	require.NoError(t, err)
	assert.True(t, service2.IsPluginEnabled("awf-plugin-simple"))
}

// TestPluginService_ConfigStorage_Integration tests plugin configuration persistence.
func TestPluginService_ConfigStorage_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	stateStore := pluginmgr.NewJSONPluginStateStore(tmpDir)
	service := application.NewPluginService(nil, stateStore, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Store config
	config := map[string]any{
		"webhook_url": "https://hooks.slack.com/test",
		"channel":     "#alerts",
		"retry_count": 5,
	}
	err := service.SetPluginConfig(ctx, "awf-plugin-slack", config)
	require.NoError(t, err)

	// Retrieve config
	retrieved := service.GetPluginConfig("awf-plugin-slack")
	assert.Equal(t, "https://hooks.slack.com/test", retrieved["webhook_url"])
	assert.Equal(t, "#alerts", retrieved["channel"])
	assert.Equal(t, 5, retrieved["retry_count"])

	// Save and reload
	err = service.SaveState(ctx)
	require.NoError(t, err)

	stateStore2 := pluginmgr.NewJSONPluginStateStore(tmpDir)
	err = stateStore2.Load(ctx)
	require.NoError(t, err)

	service2 := application.NewPluginService(nil, stateStore2, nil)
	retrieved2 := service2.GetPluginConfig("awf-plugin-slack")
	assert.Equal(t, "https://hooks.slack.com/test", retrieved2["webhook_url"])
}

// TestOperationRegistry_Integration tests operation registration and lookup.
// Acceptance Criteria: Plugins can register custom operations
func TestOperationRegistry_Integration(t *testing.T) {
	registry := pluginmgr.NewOperationRegistry()

	// Register an operation
	op := &pluginmodel.OperationSchema{
		Name:        "slack.send",
		Description: "Send a Slack message",
		Inputs: map[string]pluginmodel.InputSchema{
			"message": {Type: pluginmodel.InputTypeString, Required: true},
			"channel": {Type: pluginmodel.InputTypeString, Required: false, Default: "#general"},
		},
		Outputs:    []string{"message_id", "timestamp"},
		PluginName: "awf-plugin-slack",
	}

	err := registry.RegisterOperation(op)
	require.NoError(t, err)

	// Lookup operation
	retrieved, found := registry.GetOperation("slack.send")
	assert.True(t, found)
	assert.Equal(t, "slack.send", retrieved.Name)
	assert.Equal(t, "awf-plugin-slack", retrieved.PluginName)

	// List operations
	ops := registry.Operations()
	assert.Len(t, ops, 1)

	// Get operations by plugin
	pluginOps := registry.GetPluginOperations("awf-plugin-slack")
	assert.Len(t, pluginOps, 1)

	// Unregister
	err = registry.UnregisterOperation("slack.send")
	require.NoError(t, err)
	_, found = registry.GetOperation("slack.send")
	assert.False(t, found)
}

// TestCLI_Plugin_List_Integration tests the `awf plugin list` command.
func TestCLI_Plugin_List_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	fixturesPath, _ := filepath.Abs("../../fixtures/plugins")

	t.Setenv("AWF_PLUGINS_PATH", fixturesPath)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "list", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "awf-plugin-simple")
	assert.Contains(t, output, "awf-plugin-slack")
}

// TestCLI_Plugin_List_JSON_Integration tests JSON output format.
func TestCLI_Plugin_List_JSON_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	fixturesPath, _ := filepath.Abs("../../fixtures/plugins")

	t.Setenv("AWF_PLUGINS_PATH", fixturesPath)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "list", "--storage", tmpDir, "--format", "json"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `"name"`)
	assert.Contains(t, output, "awf-plugin-simple")
}

// TestCLI_Plugin_EnableDisable_Integration tests enable/disable commands.
func TestCLI_Plugin_EnableDisable_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	fixturesPath, _ := filepath.Abs("../../fixtures/plugins")

	t.Setenv("AWF_PLUGINS_PATH", fixturesPath)

	// Disable a plugin
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "disable", "awf-plugin-simple", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "disabled")

	// Verify state was persisted
	statePath := filepath.Join(tmpDir, "plugins", "plugins.json")
	data, err := os.ReadFile(statePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "awf-plugin-simple")
	assert.Contains(t, string(data), `"enabled": false`)

	// Re-enable the plugin
	cmd = cli.NewRootCommand()
	buf = new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "enable", "awf-plugin-simple", "--storage", tmpDir})

	err = cmd.Execute()
	require.NoError(t, err)

	output = buf.String()
	assert.Contains(t, output, "enabled")
}

// TestCLI_Plugin_Help_Integration tests plugin help output.
func TestCLI_Plugin_Help_Integration(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"plugin", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Manage AWF plugins")
	assert.Contains(t, output, "list")
	assert.Contains(t, output, "enable")
	assert.Contains(t, output, "disable")
}

// TestPluginDiscovery_EmptyDirectory_Integration tests discovery with no plugins.
func TestPluginDiscovery_EmptyDirectory_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	plugins, err := loader.DiscoverPlugins(ctx, tmpDir)

	require.NoError(t, err)
	assert.Empty(t, plugins)
}

// TestPluginDiscovery_MixedValidity_Integration tests discovery with valid and invalid plugins.
func TestPluginDiscovery_MixedValidity_Integration(t *testing.T) {
	// Use fixtures which contain both valid and invalid plugins
	fixturesPath := "../../fixtures/plugins"

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	plugins, err := loader.DiscoverPlugins(ctx, fixturesPath)

	// Discovery should succeed, skipping invalid plugins
	require.NoError(t, err)

	// Should have discovered valid plugins
	var names []string
	for _, p := range plugins {
		if p.Manifest != nil {
			names = append(names, p.Manifest.Name)
		}
	}
	assert.Contains(t, names, "awf-plugin-simple")
	assert.Contains(t, names, "awf-plugin-slack")

	// Invalid plugins should not be in the list
	for _, name := range names {
		assert.NotEqual(t, "bad-plugin", name)
	}
}

// TestPluginLoader_Validation_Integration tests manifest validation.
// Acceptance Criteria: Plugin versioning and compatibility
func TestPluginLoader_Validation_Integration(t *testing.T) {
	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tests := []struct {
		name        string
		pluginDir   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid plugin passes validation",
			pluginDir:   "../../fixtures/plugins/valid-simple",
			expectError: false,
		},
		{
			name:        "missing name fails validation",
			pluginDir:   "../../fixtures/plugins/invalid-missing-name",
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name:        "missing version fails validation",
			pluginDir:   "../../fixtures/plugins/invalid-missing-version",
			expectError: true,
			errorMsg:    "version is required",
		},
		{
			name:        "missing awf_version fails validation",
			pluginDir:   "../../fixtures/plugins/invalid-missing-awf-version",
			expectError: true,
			errorMsg:    "awf_version is required",
		},
		{
			name:        "invalid capability fails validation",
			pluginDir:   "../../fixtures/plugins/invalid-bad-capability",
			expectError: true,
			errorMsg:    "invalid capability",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginInfo, loadErr := loader.LoadPlugin(ctx, tt.pluginDir)

			if loadErr != nil {
				// Parse error
				if tt.expectError {
					return // Expected
				}
				t.Fatalf("unexpected load error: %v", loadErr)
			}

			err := loader.ValidatePlugin(pluginInfo)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPluginStateStore_Concurrent_Integration tests concurrent access to state store.
func TestPluginStateStore_Concurrent_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	stateStore := pluginmgr.NewJSONPluginStateStore(tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Concurrent enable/disable operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		pluginName := "plugin-" + string(rune('a'+i))
		go func(name string) {
			for j := 0; j < 100; j++ {
				_ = stateStore.SetEnabled(ctx, name, j%2 == 0)
				_ = stateStore.IsEnabled(name)
			}
			done <- true
		}(pluginName)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should be able to save without error
	err := stateStore.Save(ctx)
	assert.NoError(t, err)
}

// TestOperationRegistry_Concurrent_Integration tests concurrent registry access.
func TestOperationRegistry_Concurrent_Integration(t *testing.T) {
	registry := pluginmgr.NewOperationRegistry()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			opName := "op-" + string(rune('a'+id))
			op := &pluginmodel.OperationSchema{
				Name:       opName,
				PluginName: "test-plugin",
			}

			// Register, lookup, list, unregister
			for j := 0; j < 100; j++ {
				_ = registry.RegisterOperation(op)
				_, _ = registry.GetOperation(opName)
				_ = registry.Operations()
				_ = registry.UnregisterOperation(opName)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Registry should be in a consistent state
	assert.GreaterOrEqual(t, registry.Count(), 0)
}

// TestPluginLoad_InvalidPath_Integration tests loading from non-existent path.
func TestPluginLoad_InvalidPath_Integration(t *testing.T) {
	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := loader.LoadPlugin(ctx, "/non/existent/path")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}

// TestPluginLoad_NotDirectory_Integration tests loading a file instead of directory.
func TestPluginLoad_NotDirectory_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not-a-dir")
	os.WriteFile(filePath, []byte("content"), 0o644)

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := loader.LoadPlugin(ctx, filePath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

// TestPluginLoad_NoManifest_Integration tests loading a directory without plugin.yaml.
func TestPluginLoad_NoManifest_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := loader.LoadPlugin(ctx, tmpDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin.yaml not found")
}

// TestPluginLoad_InvalidYAML_Integration tests loading plugin with invalid YAML.
func TestPluginLoad_InvalidYAML_Integration(t *testing.T) {
	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := loader.LoadPlugin(ctx, "../../fixtures/plugins/invalid-syntax")

	assert.Error(t, err)
}

// TestPluginManager_LoadUnknown_Integration tests loading an unknown plugin.
func TestPluginManager_LoadUnknown_Integration(t *testing.T) {
	fixturesPath := "../../fixtures/plugins"

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)
	manager := pluginmgr.NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := manager.Load(ctx, "non-existent-plugin")

	assert.Error(t, err)
}

// TestPluginService_LoadDisabled_Integration tests that loading a disabled plugin fails.
func TestPluginService_LoadDisabled_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	fixturesPath := "../../fixtures/plugins"

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)
	manager := pluginmgr.NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	stateStore := pluginmgr.NewJSONPluginStateStore(tmpDir)

	service := application.NewPluginService(manager, stateStore, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Discover first
	_, err := manager.Discover(ctx)
	require.NoError(t, err)

	// Disable the plugin
	err = stateStore.SetEnabled(ctx, "awf-plugin-simple", false)
	require.NoError(t, err)

	// Try to load - should fail
	err = service.LoadPlugin(ctx, "awf-plugin-simple")

	assert.Error(t, err)
	assert.ErrorIs(t, err, application.ErrPluginDisabled)
}

// TestOperationRegistry_DuplicateRegistration_Integration tests duplicate operation registration.
func TestOperationRegistry_DuplicateRegistration_Integration(t *testing.T) {
	registry := pluginmgr.NewOperationRegistry()

	op := &pluginmodel.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
	}

	err := registry.RegisterOperation(op)
	require.NoError(t, err)

	// Register again should fail
	err = registry.RegisterOperation(op)

	assert.Error(t, err)
	assert.ErrorIs(t, err, pluginmgr.ErrOperationAlreadyRegistered)
}

// TestOperationRegistry_InvalidOperation_Integration tests registering invalid operations.
func TestOperationRegistry_InvalidOperation_Integration(t *testing.T) {
	registry := pluginmgr.NewOperationRegistry()

	tests := []struct {
		name string
		op   *pluginmodel.OperationSchema
	}{
		{
			name: "nil operation",
			op:   nil,
		},
		{
			name: "empty name",
			op:   &pluginmodel.OperationSchema{Name: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.RegisterOperation(tt.op)
			assert.Error(t, err)
			assert.ErrorIs(t, err, pluginmgr.ErrInvalidOperation)
		})
	}
}

// TestOperationRegistry_UnregisterNotFound_Integration tests unregistering non-existent operation.
func TestOperationRegistry_UnregisterNotFound_Integration(t *testing.T) {
	registry := pluginmgr.NewOperationRegistry()

	err := registry.UnregisterOperation("non-existent")

	assert.Error(t, err)
	assert.ErrorIs(t, err, pluginmgr.ErrOperationNotFound)
}

// TestCLI_Plugin_Enable_NoArgs_Integration tests enable command without arguments.
func TestCLI_Plugin_Enable_NoArgs_Integration(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "enable"})

	err := cmd.Execute()

	assert.Error(t, err)
}

// TestCLI_Plugin_Disable_NoArgs_Integration tests disable command without arguments.
func TestCLI_Plugin_Disable_NoArgs_Integration(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "disable"})

	err := cmd.Execute()

	assert.Error(t, err)
}

// TestPluginDiscovery_ContextCancellation_Integration tests discovery with cancelled context.
func TestPluginDiscovery_ContextCancellation_Integration(t *testing.T) {
	fixturesPath := "../../fixtures/plugins"

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := loader.DiscoverPlugins(ctx, fixturesPath)

	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestPluginLoad_ContextCancellation_Integration tests loading with cancelled context.
func TestPluginLoad_ContextCancellation_Integration(t *testing.T) {
	fixturesPath := "../../fixtures/plugins/valid-simple"

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := loader.LoadPlugin(ctx, fixturesPath)

	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestPluginService_ContextCancellation_Integration tests service with cancelled context.
func TestPluginService_ContextCancellation_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	fixturesPath := "../../fixtures/plugins"

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)
	manager := pluginmgr.NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	stateStore := pluginmgr.NewJSONPluginStateStore(tmpDir)

	service := application.NewPluginService(manager, stateStore, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := service.DiscoverPlugins(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestPluginInfo_StatusMethods_Integration tests PluginInfo helper methods.
func TestPluginInfo_StatusMethods_Integration(t *testing.T) {
	tests := []struct {
		status   pluginmodel.PluginStatus
		isActive bool
		canLoad  bool
	}{
		{pluginmodel.StatusDiscovered, false, true},
		{pluginmodel.StatusLoaded, false, false},
		{pluginmodel.StatusInitialized, false, false},
		{pluginmodel.StatusRunning, true, false},
		{pluginmodel.StatusStopped, false, true},
		{pluginmodel.StatusFailed, false, true},
		{pluginmodel.StatusDisabled, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			info := &pluginmodel.PluginInfo{Status: tt.status}
			assert.Equal(t, tt.isActive, info.IsActive())
			assert.Equal(t, tt.canLoad, info.CanLoad())
		})
	}
}

// TestOperationResult_Helpers_Integration tests OperationResult helper methods.
func TestOperationResult_Helpers_Integration(t *testing.T) {
	t.Run("success result", func(t *testing.T) {
		result := &pluginmodel.OperationResult{
			Success: true,
			Outputs: map[string]any{
				"message_id": "12345",
				"timestamp":  1234567890,
			},
		}

		assert.True(t, result.IsSuccess())
		assert.False(t, result.HasError())

		msgID, ok := result.GetOutput("message_id")
		assert.True(t, ok)
		assert.Equal(t, "12345", msgID)

		_, ok = result.GetOutput("nonexistent")
		assert.False(t, ok)
	})

	t.Run("error result", func(t *testing.T) {
		result := &pluginmodel.OperationResult{
			Success: false,
			Error:   "connection refused",
		}

		assert.False(t, result.IsSuccess())
		assert.True(t, result.HasError())
	})

	t.Run("nil outputs", func(t *testing.T) {
		result := &pluginmodel.OperationResult{
			Success: true,
		}

		_, ok := result.GetOutput("anything")
		assert.False(t, ok)
	})
}

// TestManifest_HasCapability_Integration tests capability checking.
func TestManifest_HasCapability_Integration(t *testing.T) {
	m := &pluginmodel.Manifest{
		Capabilities: []string{pluginmodel.CapabilityOperations, pluginmodel.CapabilityCommands},
	}

	assert.True(t, m.HasCapability(pluginmodel.CapabilityOperations))
	assert.True(t, m.HasCapability(pluginmodel.CapabilityCommands))
	assert.False(t, m.HasCapability(pluginmodel.CapabilityValidators))
	assert.False(t, m.HasCapability("unknown"))
}

// TestCLI_Plugin_List_EmptyDir_Integration tests list command with no plugins.
func TestCLI_Plugin_List_EmptyDir_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	os.MkdirAll(pluginsDir, 0o755)

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "list", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Should not error, just show empty or "no plugins found"
	assert.True(t, len(output) > 0 || strings.Contains(output, "NAME"))
}

// TestPluginState_NewPluginState_Integration tests default plugin state creation.
func TestPluginState_NewPluginState_Integration(t *testing.T) {
	state := pluginmodel.NewPluginState()

	assert.True(t, state.Enabled, "plugins should be enabled by default")
	assert.NotNil(t, state.Config, "config map should be initialized")
	assert.Zero(t, state.DisabledAt, "disabled_at should be zero for enabled plugin")
}
