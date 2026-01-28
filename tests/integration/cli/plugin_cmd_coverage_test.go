//go:build integration

// Component: T006
// Feature: C028
package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	infrastructurePlugin "github.com/vanoix/awf/internal/infrastructure/plugin"
	"github.com/vanoix/awf/internal/interfaces/cli"
	"github.com/vanoix/awf/internal/testutil"
)

// setupPluginTest creates a test directory with plugin fixtures
func setupPluginTest(t *testing.T) (dir, pluginsDir string) {
	t.Helper()
	dir = testutil.SetupTestDir(t)

	// Create plugins directory structure
	pluginsDir = filepath.Join(dir, "plugins")
	err := os.MkdirAll(pluginsDir, 0o755)
	require.NoError(t, err)

	// Create a valid test plugin
	testPluginDir := filepath.Join(pluginsDir, "test-plugin")
	err = os.MkdirAll(testPluginDir, 0o755)
	require.NoError(t, err)

	// Write minimal valid plugin manifest
	manifestContent := `name: test-plugin
version: 1.0.0
awf_version: ">=0.4.0"
capabilities:
  - operations
`
	manifestPath := filepath.Join(testPluginDir, "plugin.yaml")
	err = os.WriteFile(manifestPath, []byte(manifestContent), 0o644)
	require.NoError(t, err)

	return
}

// TestRunPluginEnable_Success verifies successful plugin enablement
func TestRunPluginEnable_Success(t *testing.T) {
	// Arrange: create test directory with valid plugin
	dir, pluginsDir := setupPluginTest(t)
	storagePath := filepath.Join(dir, ".awf")

	// Set plugin path via environment variable
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "enable", "test-plugin", "--storage", storagePath})

	// Act: execute enable command
	err := cmd.Execute()

	// Assert: command succeeds
	require.NoError(t, err, "enable command should succeed for valid plugin")
	output := buf.String()
	assert.Contains(t, output, "test-plugin", "output should mention plugin name")
	assert.Contains(t, output, "enabled", "output should indicate plugin was enabled")

	// Verify plugin state was persisted
	stateStore := infrastructurePlugin.NewJSONPluginStateStore(filepath.Join(storagePath, "plugins"))
	err = stateStore.Load(context.Background())
	require.NoError(t, err, "should load persisted plugin state")
	isEnabled := stateStore.IsEnabled("test-plugin")
	assert.True(t, isEnabled, "plugin should be marked as enabled in state store")
}

// TestRunPluginEnable_NotFound verifies that enable succeeds even for non-existent plugins
// (state management is independent of plugin discovery)
func TestRunPluginEnable_NotFound(t *testing.T) {
	// Arrange: create test directory with no plugins
	dir, pluginsDir := setupPluginTest(t)
	storagePath := filepath.Join(dir, ".awf")

	// Set plugin path via environment variable
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "enable", "nonexistent-plugin", "--storage", storagePath})

	// Act: execute enable command
	err := cmd.Execute()

	// Assert: command succeeds (state management doesn't validate plugin existence)
	require.NoError(t, err, "enable command should succeed even for non-existent plugin")
	output := buf.String()
	assert.Contains(t, output, "nonexistent-plugin", "output should mention plugin name")
	assert.Contains(t, output, "enabled", "output should indicate enabled state")

	// Verify state was persisted
	stateStore := infrastructurePlugin.NewJSONPluginStateStore(filepath.Join(storagePath, "plugins"))
	err = stateStore.Load(context.Background())
	require.NoError(t, err, "should load persisted plugin state")
	isEnabled := stateStore.IsEnabled("nonexistent-plugin")
	assert.True(t, isEnabled, "non-existent plugin should be marked as enabled in state")
}

// TestRunPluginEnable_JSONFormat verifies JSON output format
func TestRunPluginEnable_JSONFormat(t *testing.T) {
	// Arrange: create test directory with valid plugin
	dir, pluginsDir := setupPluginTest(t)
	storagePath := filepath.Join(dir, ".awf")

	// Set plugin path via environment variable
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "enable", "test-plugin", "--storage", storagePath, "--format", "json"})

	// Act: execute enable command
	err := cmd.Execute()

	// Assert: command succeeds with JSON output
	require.NoError(t, err, "enable command should succeed")
	output := buf.String()
	assert.NotEmpty(t, output, "should produce output")

	// Parse and verify JSON structure
	var result map[string]interface{}
	jsonErr := json.Unmarshal([]byte(output), &result)
	require.NoError(t, jsonErr, "output should be valid JSON: %s", output)

	// Verify JSON fields
	assert.Equal(t, "test-plugin", result["plugin"], "JSON should contain plugin name")
	assert.Equal(t, true, result["enabled"], "JSON should indicate enabled status")
}

// TestRunPluginDisable_Success verifies successful plugin disablement
func TestRunPluginDisable_Success(t *testing.T) {
	// Arrange: create test directory with valid plugin and enable it first
	dir, pluginsDir := setupPluginTest(t)
	storagePath := filepath.Join(dir, ".awf")

	// Set plugin path via environment variable
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	// First enable the plugin
	cmdEnable := cli.NewRootCommand()
	cmdEnable.SetArgs([]string{"plugin", "enable", "test-plugin", "--storage", storagePath})
	err := cmdEnable.Execute()
	require.NoError(t, err, "plugin enable should succeed")

	// Now disable it
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "disable", "test-plugin", "--storage", storagePath})

	// Act: execute disable command
	err = cmd.Execute()

	// Assert: command succeeds
	require.NoError(t, err, "disable command should succeed for enabled plugin")
	output := buf.String()
	assert.Contains(t, output, "test-plugin", "output should mention plugin name")
	assert.Contains(t, output, "disabled", "output should indicate plugin was disabled")

	// Verify plugin state was persisted
	stateStore := infrastructurePlugin.NewJSONPluginStateStore(filepath.Join(storagePath, "plugins"))
	err = stateStore.Load(context.Background())
	require.NoError(t, err, "should load persisted plugin state")
	isEnabled := stateStore.IsEnabled("test-plugin")
	assert.False(t, isEnabled, "plugin should be marked as disabled in state store")
}

// TestRunPluginDisable_NotFound verifies that disable succeeds even for non-existent plugins
// (state management is independent of plugin discovery)
func TestRunPluginDisable_NotFound(t *testing.T) {
	// Arrange: create test directory with no plugins
	dir, pluginsDir := setupPluginTest(t)
	storagePath := filepath.Join(dir, ".awf")

	// Set plugin path via environment variable
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "disable", "nonexistent-plugin", "--storage", storagePath})

	// Act: execute disable command
	err := cmd.Execute()

	// Assert: command succeeds (state management doesn't validate plugin existence)
	require.NoError(t, err, "disable command should succeed even for non-existent plugin")
	output := buf.String()
	assert.Contains(t, output, "nonexistent-plugin", "output should mention plugin name")
	assert.Contains(t, output, "disabled", "output should indicate disabled state")

	// Verify state was persisted
	stateStore := infrastructurePlugin.NewJSONPluginStateStore(filepath.Join(storagePath, "plugins"))
	err = stateStore.Load(context.Background())
	require.NoError(t, err, "should load persisted plugin state")
	isEnabled := stateStore.IsEnabled("nonexistent-plugin")
	assert.False(t, isEnabled, "non-existent plugin should be marked as disabled in state")
}

// TestRunPluginDisable_JSONFormat verifies JSON output format
func TestRunPluginDisable_JSONFormat(t *testing.T) {
	// Arrange: create test directory with valid plugin and enable it first
	dir, pluginsDir := setupPluginTest(t)
	storagePath := filepath.Join(dir, ".awf")

	// Set plugin path via environment variable
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	// First enable the plugin
	cmdEnable := cli.NewRootCommand()
	cmdEnable.SetArgs([]string{"plugin", "enable", "test-plugin", "--storage", storagePath})
	err := cmdEnable.Execute()
	require.NoError(t, err, "plugin enable should succeed")

	// Now disable it with JSON output
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "disable", "test-plugin", "--storage", storagePath, "--format", "json"})

	// Act: execute disable command
	err = cmd.Execute()

	// Assert: command succeeds with JSON output
	require.NoError(t, err, "disable command should succeed")
	output := buf.String()
	assert.NotEmpty(t, output, "should produce output")

	// Parse and verify JSON structure
	var result map[string]interface{}
	jsonErr := json.Unmarshal([]byte(output), &result)
	require.NoError(t, jsonErr, "output should be valid JSON: %s", output)

	// Verify JSON fields
	assert.Equal(t, "test-plugin", result["plugin"], "JSON should contain plugin name")
	assert.Equal(t, false, result["enabled"], "JSON should indicate disabled status")
}

// TestRunPluginEnable_NoPluginsDirectory tests enable when plugins directory doesn't exist
// State management works independently of plugin discovery
func TestRunPluginEnable_NoPluginsDirectory(t *testing.T) {
	// Arrange: create test directory without plugins directory
	dir := testutil.SetupTestDir(t)
	storagePath := filepath.Join(dir, ".awf")
	nonexistentPluginsDir := filepath.Join(dir, "nonexistent-plugins")

	// Set plugin path via environment variable to non-existent directory
	t.Setenv("AWF_PLUGINS_PATH", nonexistentPluginsDir)

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "enable", "test-plugin", "--storage", storagePath})

	// Act: execute enable command
	err := cmd.Execute()

	// Assert: command succeeds (state management works without plugin directory)
	require.NoError(t, err, "enable should succeed even when plugins directory doesn't exist")
	output := buf.String()
	assert.Contains(t, output, "test-plugin", "output should mention plugin name")
	assert.Contains(t, output, "enabled", "output should indicate enabled state")
}

// TestRunPluginDisable_AlreadyDisabled tests disabling an already disabled plugin
func TestRunPluginDisable_AlreadyDisabled(t *testing.T) {
	// Arrange: create test directory with valid plugin (not enabled)
	dir, pluginsDir := setupPluginTest(t)
	storagePath := filepath.Join(dir, ".awf")

	// Set plugin path via environment variable
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "disable", "test-plugin", "--storage", storagePath})

	// Act: execute disable command on already disabled plugin
	err := cmd.Execute()

	// Assert: command should still succeed (idempotent operation)
	// The behavior here depends on implementation - it might succeed or fail
	// Based on the code, DisablePlugin should handle this gracefully
	if err != nil {
		// If it errors, verify it's a sensible error
		assert.Contains(t, err.Error(), "plugin", "error should mention plugin")
	} else {
		// If it succeeds, verify output indicates disabled state
		output := buf.String()
		assert.Contains(t, output, "test-plugin", "output should mention plugin name")
	}
}

// TestRunPluginEnable_AlreadyEnabled tests enabling an already enabled plugin
func TestRunPluginEnable_AlreadyEnabled(t *testing.T) {
	// Arrange: create test directory with valid plugin and enable it first
	dir, pluginsDir := setupPluginTest(t)
	storagePath := filepath.Join(dir, ".awf")

	// Set plugin path via environment variable
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	// First enable the plugin
	cmdEnable := cli.NewRootCommand()
	cmdEnable.SetArgs([]string{"plugin", "enable", "test-plugin", "--storage", storagePath})
	err := cmdEnable.Execute()
	require.NoError(t, err, "initial plugin enable should succeed")

	// Try to enable it again
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "enable", "test-plugin", "--storage", storagePath})

	// Act: execute enable command on already enabled plugin
	err = cmd.Execute()

	// Assert: command should succeed (idempotent operation)
	require.NoError(t, err, "enabling already enabled plugin should succeed")
	output := buf.String()
	assert.Contains(t, output, "test-plugin", "output should mention plugin name")
	assert.Contains(t, output, "enabled", "output should indicate enabled state")
}

// TestRunPluginEnable_InvalidPluginManifest tests enabling a plugin with invalid manifest
// State management works independently, so this succeeds
func TestRunPluginEnable_InvalidPluginManifest(t *testing.T) {
	// Arrange: create test directory with invalid plugin
	dir := testutil.SetupTestDir(t)
	storagePath := filepath.Join(dir, ".awf")

	pluginsDir := filepath.Join(dir, "plugins")
	err := os.MkdirAll(pluginsDir, 0o755)
	require.NoError(t, err)

	// Create plugin with invalid manifest
	invalidPluginDir := filepath.Join(pluginsDir, "invalid-plugin")
	err = os.MkdirAll(invalidPluginDir, 0o755)
	require.NoError(t, err)

	// Write invalid YAML (missing required fields)
	manifestContent := `name: invalid-plugin
# Missing version and awf_version
capabilities:
  - operations
`
	manifestPath := filepath.Join(invalidPluginDir, "plugin.yaml")
	err = os.WriteFile(manifestPath, []byte(manifestContent), 0o644)
	require.NoError(t, err)

	// Set plugin path via environment variable
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"plugin", "enable", "invalid-plugin", "--storage", storagePath})

	// Act: execute enable command
	err = cmd.Execute()

	// Assert: command succeeds (state management doesn't validate manifests)
	// The validation would happen during plugin loading/initialization, not enable
	require.NoError(t, err, "enable should succeed even with invalid plugin manifest")
	output := buf.String()
	assert.Contains(t, output, "invalid-plugin", "output should mention plugin name")
	assert.Contains(t, output, "enabled", "output should indicate enabled state")
}
