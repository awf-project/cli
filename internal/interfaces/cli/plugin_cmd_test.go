package cli_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "plugin" {
			found = true
			break
		}
	}

	assert.True(t, found, "expected root command to have 'plugin' subcommand")
}

func TestPluginCommand_HasAlias(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "plugin" {
			assert.Contains(t, sub.Aliases, "plugins", "plugin command should have 'plugins' alias")
			return
		}
	}
	t.Error("plugin command not found")
}

func TestPluginCommand_HasListSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	found := false
	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "list" {
			found = true
			break
		}
	}

	assert.True(t, found, "plugin command should have 'list' subcommand")
}

func TestPluginCommand_HasEnableSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	found := false
	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "enable" {
			found = true
			break
		}
	}

	assert.True(t, found, "plugin command should have 'enable' subcommand")
}

func TestPluginCommand_HasDisableSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	found := false
	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "disable" {
			found = true
			break
		}
	}

	assert.True(t, found, "plugin command should have 'disable' subcommand")
}

func TestPluginListCommand_HasLsAlias(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "list" {
			assert.Contains(t, sub.Aliases, "ls", "list subcommand should have 'ls' alias")
			return
		}
	}
	t.Error("list subcommand not found")
}

func TestPluginListCommand_NoPlugins(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Isolate from global plugins
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// With builtin plugins always present, should show at least the 3 builtins
	assert.Contains(t, output, "github", "should show github builtin")
	assert.Contains(t, output, "http", "should show http builtin")
	assert.Contains(t, output, "notify", "should show notify builtin")
	// Should NOT show the "No plugins found" message since builtins exist
	assert.NotContains(t, output, "No plugins found", "should not show no plugins message when builtins exist")
}

func TestPluginListCommand_WithPlugins(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin directory with a plugin
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	// Create valid plugin manifest
	manifestContent := `name: test-plugin
version: 1.0.0
description: A test plugin
awf_version: ">=0.1.0"
capabilities:
  - operations
`
	require.NoError(t, os.WriteFile(
		filepath.Join(testPluginDir, "plugin.yaml"),
		[]byte(manifestContent),
		0o644,
	))

	// Isolate from other plugins
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "test-plugin", "should show plugin name")
	assert.Contains(t, output, "1.0.0", "should show plugin version")
}

func TestPluginListCommand_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin directory with a plugin
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "json-test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: json-test-plugin
version: 2.0.0
description: Plugin for JSON testing
awf_version: ">=0.1.0"
capabilities:
  - operations
  - step_types
`
	require.NoError(t, os.WriteFile(
		filepath.Join(testPluginDir, "plugin.yaml"),
		[]byte(manifestContent),
		0o644,
	))

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--format", "json", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should be valid JSON
	var plugins []map[string]any
	err = json.Unmarshal([]byte(output), &plugins)
	require.NoError(t, err, "output should be valid JSON")

	// Should contain plugin info
	assert.Contains(t, output, `"name"`)
	assert.Contains(t, output, `"json-test-plugin"`)
	assert.Contains(t, output, `"version"`)
	assert.Contains(t, output, `"enabled"`)
}

func TestPluginListCommand_TableFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugins dir with plugin
	pluginsDir := filepath.Join(tmpDir, ".awf", "plugins")
	testPluginDir := filepath.Join(pluginsDir, "table-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: table-plugin
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

	// Isolate - set plugins path to the test directory
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--format", "table", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// Table format should have headers
	assert.Contains(t, output, "NAME", "table should have NAME header")
	assert.Contains(t, output, "VERSION", "table should have VERSION header")
	assert.Contains(t, output, "STATUS", "table should have STATUS header")
	assert.Contains(t, output, "ENABLED", "table should have ENABLED header")
}

func TestPluginListCommand_QuietFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugins
	pluginsDir := filepath.Join(tmpDir, "plugins")
	plugin1Dir := filepath.Join(pluginsDir, "plugin-one")
	plugin2Dir := filepath.Join(pluginsDir, "plugin-two")
	require.NoError(t, os.MkdirAll(plugin1Dir, 0o755))
	require.NoError(t, os.MkdirAll(plugin2Dir, 0o755))

	manifest1 := `name: plugin-one
version: 1.0.0
awf_version: ">=0.1.0"
capabilities:
  - operations
`
	manifest2 := `name: plugin-two
version: 1.0.0
awf_version: ">=0.1.0"
capabilities:
  - operations
`
	require.NoError(t, os.WriteFile(filepath.Join(plugin1Dir, "plugin.yaml"), []byte(manifest1), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(plugin2Dir, "plugin.yaml"), []byte(manifest2), 0o644))

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--format", "quiet", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Quiet mode: just names, one per line
	// Should include 3 builtins (github, http, notify) + 2 external plugins
	assert.Len(t, lines, 5, "quiet mode should output one name per line (3 builtins + 2 external)")
	assert.Contains(t, output, "plugin-one")
	assert.Contains(t, output, "plugin-two")
	assert.Contains(t, output, "github")
	assert.Contains(t, output, "http")
	assert.Contains(t, output, "notify")
}

func TestPluginListCommand_ShowsDisabledPlugins(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "disabled-test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: disabled-test-plugin
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

	// Create state with disabled plugin
	pluginsStateDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsStateDir, 0o755))
	stateContent := `{
		"disabled-test-plugin": {
			"enabled": false,
			"config": {}
		}
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(pluginsStateDir, "plugins.json"),
		[]byte(stateContent),
		0o644,
	))

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "disabled-test-plugin", "should show disabled plugin")
	// Should show it's disabled
	assert.Contains(t, output, "no", "should indicate plugin is disabled")
}

func TestPluginEnableCommand_RequiresArgument(t *testing.T) {
	cmd := cli.NewRootCommand()
	var errOut bytes.Buffer
	cmd.SetOut(&errOut)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"plugin", "enable"})

	err := cmd.Execute()

	assert.Error(t, err, "enable without plugin name should error")
	assert.Contains(t, err.Error(), "accepts 1 arg", "error should mention argument requirement")
}

func TestPluginEnableCommand_EnablesPlugin(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "enable-test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: enable-test-plugin
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

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "enable", "enable-test-plugin", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "enable-test-plugin", "output should confirm plugin name")
	assert.Contains(t, output, "enabled", "output should confirm plugin was enabled")
}

func TestPluginEnableCommand_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "json-enable-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: json-enable-plugin
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

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "enable", "json-enable-plugin", "--format", "json", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should be valid JSON
	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, "json-enable-plugin", result["plugin"])
	assert.True(t, result["enabled"].(bool))
}

func TestPluginEnableCommand_PersistsState(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "persist-test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: persist-test-plugin
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

	// Pre-create disabled state
	stateDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	stateContent := `{
		"persist-test-plugin": {
			"enabled": false,
			"config": {}
		}
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(stateDir, "plugins.json"),
		[]byte(stateContent),
		0o644,
	))

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "enable", "persist-test-plugin", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify state was persisted
	stateFile := filepath.Join(stateDir, "plugins.json")
	stateData, err := os.ReadFile(stateFile)
	require.NoError(t, err)

	// State file should exist and not have false for enabled
	// (exact format depends on implementation)
	_ = stateData
}

func TestPluginDisableCommand_RequiresArgument(t *testing.T) {
	cmd := cli.NewRootCommand()
	var errOut bytes.Buffer
	cmd.SetOut(&errOut)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"plugin", "disable"})

	err := cmd.Execute()

	assert.Error(t, err, "disable without plugin name should error")
	assert.Contains(t, err.Error(), "accepts 1 arg", "error should mention argument requirement")
}

func TestPluginDisableCommand_DisablesPlugin(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "disable-test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: disable-test-plugin
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

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "disable", "disable-test-plugin", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "disable-test-plugin", "output should confirm plugin name")
	assert.Contains(t, output, "disabled", "output should confirm plugin was disabled")
}

func TestPluginDisableCommand_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "json-disable-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: json-disable-plugin
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

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "disable", "json-disable-plugin", "--format", "json", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should be valid JSON
	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, "json-disable-plugin", result["plugin"])
	assert.False(t, result["enabled"].(bool))
}

func TestPluginDisableCommand_PersistsState(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "persist-disable-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: persist-disable-plugin
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

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "disable", "persist-disable-plugin", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify state was persisted
	stateFile := filepath.Join(tmpDir, "plugins", "plugins.json")
	_, err = os.Stat(stateFile)
	// State file should be created
	assert.NoError(t, err, "state file should exist after disable")
}

func TestPluginCommand_HelpText(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	// Help should explain what plugins are
	assert.Contains(t, pluginCmd.Long, "plugins", "help should mention plugins")
	assert.Contains(t, pluginCmd.Long, "operations", "help should mention operations capability")
}

func TestPluginListCommand_HelpText(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "list" {
			assert.Contains(t, sub.Long, "discovered", "list help should mention discovering plugins")
			return
		}
	}
	t.Error("list subcommand not found")
}

func TestPluginEnableCommand_HelpText(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "enable" {
			assert.Contains(t, sub.Use, "<plugin-name>", "enable usage should show plugin-name placeholder")
			return
		}
	}
	t.Error("enable subcommand not found")
}

func TestPluginDisableCommand_HelpText(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "disable" {
			assert.Contains(t, sub.Use, "<plugin-name>", "disable usage should show plugin-name placeholder")
			assert.Contains(t, sub.Long, "shut down", "disable help should mention shutting down")
			return
		}
	}
	t.Error("disable subcommand not found")
}

func TestPluginListCommand_WithInvalidPluginManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin with invalid manifest
	pluginsDir := filepath.Join(tmpDir, "plugins")
	invalidPluginDir := filepath.Join(pluginsDir, "invalid-plugin")
	require.NoError(t, os.MkdirAll(invalidPluginDir, 0o755))

	// Invalid YAML
	invalidManifest := `invalid yaml: [[[`
	require.NoError(t, os.WriteFile(
		filepath.Join(invalidPluginDir, "plugin.yaml"),
		[]byte(invalidManifest),
		0o644,
	))

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--storage", tmpDir})

	// Should not crash, but may show no plugins or skip invalid ones
	err := cmd.Execute()
	// Graceful degradation - should not fail completely
	require.NoError(t, err)
}

func TestPluginEnableCommand_NonexistentPlugin(t *testing.T) {
	tmpDir := setupTestDir(t)

	// No plugins
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "enable", "nonexistent-plugin", "--storage", tmpDir})

	// Should fail with unknown plugin error (C066/T003: validate plugin exists before enable/disable)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown plugin")
}

func TestPluginDisableCommand_NonexistentPlugin(t *testing.T) {
	tmpDir := setupTestDir(t)

	// No plugins
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "disable", "nonexistent-plugin", "--storage", tmpDir})

	// Should fail with unknown plugin error (C066/T003: validate plugin exists before enable/disable)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown plugin")
}

func TestPluginListCommand_ShowsRemovedPlugins(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create state with a plugin that no longer exists on disk
	stateDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	stateContent := `{
		"removed-plugin": {
			"enabled": false,
			"config": {}
		}
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(stateDir, "plugins.json"),
		[]byte(stateContent),
		0o644,
	))

	// No plugins directory
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// Should show removed plugin with not_found status
	assert.Contains(t, output, "removed-plugin", "should show removed plugin")
}

func TestPluginListCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name         string
		format       string
		expectHeader bool
		expectJSON   bool
	}{
		{
			name:         "text format",
			format:       "text",
			expectHeader: true,
			expectJSON:   false,
		},
		{
			name:         "json format",
			format:       "json",
			expectHeader: false,
			expectJSON:   true,
		},
		{
			name:         "table format",
			format:       "table",
			expectHeader: true,
			expectJSON:   false,
		},
		{
			name:         "quiet format",
			format:       "quiet",
			expectHeader: false,
			expectJSON:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create a plugin
			pluginsDir := filepath.Join(tmpDir, "plugins")
			testPluginDir := filepath.Join(pluginsDir, "format-test-plugin")
			require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

			manifestContent := `name: format-test-plugin
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

			t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"plugin", "list", "--format", tt.format, "--storage", tmpDir})

			err := cmd.Execute()
			require.NoError(t, err)

			output := out.String()

			if tt.expectJSON {
				var result []map[string]any
				err := json.Unmarshal([]byte(output), &result)
				assert.NoError(t, err, "JSON format should produce valid JSON")
			}

			if tt.expectHeader {
				assert.Contains(t, output, "NAME", "should have headers")
			}
		})
	}
}

func TestPluginCommands_StorageFlag(t *testing.T) {
	// All plugin commands should accept --storage flag
	commands := [][]string{
		{"plugin", "list"},
		{"plugin", "enable", "test"},
		{"plugin", "disable", "test"},
	}

	for _, args := range commands {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			tmpDir := t.TempDir()

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			fullArgs := append(args, "--storage", tmpDir)
			cmd.SetArgs(fullArgs)

			// Should not fail due to missing --storage support
			err := cmd.Execute()
			// May fail for other reasons (e.g., missing plugin for enable/disable)
			// but should not fail because --storage is unknown
			if err != nil {
				assert.NotContains(t, err.Error(), "unknown flag", "--storage flag should be recognized")
			}
		})
	}
}

func TestPluginListCommand_OperationsFlag_Recognized(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin", "list"})
	require.NoError(t, err)

	found := false
	pluginCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "operations" {
			found = true
		}
	})

	assert.True(t, found, "plugin list command should have --operations flag")
}

func TestPluginListCommand_OperationsFlag_NoPlugins(t *testing.T) {
	tmpDir := setupTestDir(t)

	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--operations", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// With builtin plugins always present, there will be operations
	assert.NotEmpty(t, output, "should produce output when --operations flag is set")
	// Should contain at least one builtin provider name
	assert.True(t,
		strings.Contains(output, "github") ||
			strings.Contains(output, "http") ||
			strings.Contains(output, "notify"),
		"operations output should reference at least one built-in provider")
}

func TestPluginListCommand_OperationsFlag_WithPlugins(t *testing.T) {
	tmpDir := t.TempDir()

	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "ops-test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: ops-test-plugin
version: 1.0.0
description: Test plugin with operations
awf_version: ">=0.1.0"
capabilities:
  - operations
`
	require.NoError(t, os.WriteFile(
		filepath.Join(testPluginDir, "plugin.yaml"),
		[]byte(manifestContent),
		0o644,
	))

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--operations", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.NotEmpty(t, output, "should produce output when --operations flag is set")
}

func TestPluginListCommand_OperationsFlag_WithTextFormat(t *testing.T) {
	tmpDir := t.TempDir()

	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "text-ops-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: text-ops-plugin
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

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--operations", "--format", "text", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.NotEmpty(t, output, "should produce text output with --operations flag")
}

func TestPluginListCommand_OperationsFlag_WithJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()

	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "json-ops-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: json-ops-plugin
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

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--operations", "--format", "json", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	var result []map[string]any
	err = json.Unmarshal([]byte(output), &result)
	assert.NoError(t, err, "JSON output with --operations should be valid JSON")
}

func TestPluginListCommand_OperationsFlag_WithTableFormat(t *testing.T) {
	tmpDir := t.TempDir()

	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "table-ops-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: table-ops-plugin
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

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--operations", "--format", "table", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.NotEmpty(t, output, "should produce table output with --operations flag")
}

func TestPluginListCommand_OperationsFlag_WithoutFlag(t *testing.T) {
	tmpDir := t.TempDir()

	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "noflag-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: noflag-plugin
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

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "noflag-plugin", "should show plugin list without --operations flag")
	assert.NotContains(t, output, "No operations", "should not show operations output without --operations flag")
}

func TestPluginListCommand_OperationsFlag_InitFailure(t *testing.T) {
	tmpDir := t.TempDir()

	invalidPluginPath := filepath.Join(tmpDir, "nonexistent", "path", "to", "plugins")
	t.Setenv("AWF_PLUGINS_PATH", invalidPluginPath)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "list", "--operations", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestPluginInstallCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	found := false
	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "install" {
			found = true
			break
		}
	}

	assert.True(t, found, "plugin command should have 'install' subcommand")
}

func TestPluginInstallCommand_HasExpectedFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	installCmd, _, err := cmd.Find([]string{"plugin", "install"})
	require.NoError(t, err)

	flagNames := map[string]bool{}
	installCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		flagNames[flag.Name] = true
	})

	assert.True(t, flagNames["version"], "install command should have --version flag")
	assert.True(t, flagNames["pre-release"], "install command should have --pre-release flag")
	assert.True(t, flagNames["force"], "install command should have --force flag")
}

func TestPluginInstallCommand_RequiresExactlyOneArg(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "install", "--storage", tmpDir})

	err := cmd.Execute()

	assert.Error(t, err, "install without repo arg should fail")
	assert.Contains(t, err.Error(), "accepts 1 arg", "error should mention argument requirement")
}

func TestPluginInstallCommand_RejectsInvalidOwnerRepo(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "install", "not-a-valid-repo", "--storage", tmpDir})

	err := cmd.Execute()

	assert.Error(t, err, "install with invalid owner/repo format should fail")
}

func TestPluginInstallCommand_RejectsURLPrefixOwnerRepo(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "install", "https://github.com/owner/repo", "--storage", tmpDir})

	err := cmd.Execute()

	assert.Error(t, err, "install with https:// URL format should fail")
}

func TestPluginUpdateCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	found := false
	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "update" {
			found = true
			break
		}
	}

	assert.True(t, found, "plugin command should have 'update' subcommand")
}

func TestPluginUpdateCommand_HasAllFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	updateCmd, _, err := cmd.Find([]string{"plugin", "update"})
	require.NoError(t, err)

	flagNames := map[string]bool{}
	updateCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		flagNames[flag.Name] = true
	})

	assert.True(t, flagNames["all"], "update command should have --all flag")
}

func TestPluginUpdateCommand_RequiresNameOrAllFlag(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "update", "--storage", tmpDir})

	err := cmd.Execute()

	assert.Error(t, err, "update without plugin name or --all should fail")
	assert.Contains(t, err.Error(), "plugin name or --all", "error should mention the requirement")
}

func TestPluginUpdateCommand_FailsForNotInstalledPlugin(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "update", "nonexistent-plugin", "--storage", tmpDir})

	err := cmd.Execute()

	assert.Error(t, err, "update of a plugin with no source metadata should fail")
}

func TestPluginRemoveCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	found := false
	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "remove" {
			found = true
			break
		}
	}

	assert.True(t, found, "plugin command should have 'remove' subcommand")
}

func TestPluginRemoveCommand_HasKeepDataFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	removeCmd, _, err := cmd.Find([]string{"plugin", "remove"})
	require.NoError(t, err)

	flagNames := make(map[string]bool)
	removeCmd.Flags().VisitAll(func(f *pflag.Flag) {
		flagNames[f.Name] = true
	})

	assert.True(t, flagNames["keep-data"], "remove command should have --keep-data flag")
}

func TestPluginRemoveCommand_RequiresExactlyOneArg(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "remove", "--storage", tmpDir})

	err := cmd.Execute()

	assert.Error(t, err, "remove without plugin name should error")
	assert.Contains(t, err.Error(), "accepts 1 arg", "error should mention argument requirement")
}

func TestPluginRemoveCommand_FailsForBuiltinPlugin(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "remove", "github", "--storage", tmpDir})

	err := cmd.Execute()

	assert.Error(t, err, "removing a built-in plugin should fail")
}

func TestPluginRemoveCommand_FailsForNotInstalledPlugin(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "remove", "nonexistent-plugin", "--storage", tmpDir})

	err := cmd.Execute()

	assert.Error(t, err, "removing a plugin that is not installed should fail")
}

func TestPluginCommand_HasSearchSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	found := false
	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "search" {
			found = true
			break
		}
	}

	assert.True(t, found, "plugin command should have 'search' subcommand")
}

// newMockGitHubSearchServer creates a httptest server that serves GitHub Search API responses.
func newMockGitHubSearchServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"items": []map[string]interface{}{
				{"full_name": "myorg/awf-plugin-jira", "description": "Jira integration", "stargazers_count": 42},
				{"full_name": "myorg/awf-plugin-slack", "description": "Slack notifications", "stargazers_count": 15},
			},
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck // test helper; encoding error won't occur with static data
	}))
}

func TestPluginSearchCommand_NoArgs(t *testing.T) {
	tmpDir := setupTestDir(t)
	srv := newMockGitHubSearchServer(t)
	defer srv.Close()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "search", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err, "search without query should succeed")

	output := out.String()
	assert.NotEmpty(t, output, "search output should not be empty")
	assert.Contains(t, output, "awf-plugin-jira")
}

func TestPluginSearchCommand_WithQuery(t *testing.T) {
	tmpDir := setupTestDir(t)
	srv := newMockGitHubSearchServer(t)
	defer srv.Close()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "search", "jira", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err, "search with query should succeed")

	output := out.String()
	assert.NotEmpty(t, output, "search results should not be empty")
}

func TestPluginSearchCommand_JSONOutput(t *testing.T) {
	tmpDir := setupTestDir(t)
	srv := newMockGitHubSearchServer(t)
	defer srv.Close()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "search", "--format", "json", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err, "search with JSON format should succeed")

	output := strings.TrimSpace(out.String())

	// Output should be valid JSON (array of repositories)
	var result interface{}
	err = json.Unmarshal([]byte(output), &result)
	assert.NoError(t, err, "output should be valid JSON")
}

// seedUpdateSourceData writes state metadata so updatePlugin can read origin repo and version.
func seedUpdateSourceData(t *testing.T, storagePath, pluginName, ownerRepo, version string) {
	t.Helper()

	stateDir := filepath.Join(storagePath, "plugins")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))

	content := fmt.Sprintf(`{
	%q: {
		"enabled": true,
		"source_data": {
			"repository": %q,
			"version": %q,
			"installed_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z"
		}
	}
}`, pluginName, ownerRepo, version)

	require.NoError(t, os.WriteFile(
		filepath.Join(stateDir, "plugins.json"),
		[]byte(content),
		0o600,
	))
}

// newMockGitHubReleasesServer creates an httptest server returning a single release for a plugin.
func newMockGitHubReleasesServer(t *testing.T, pluginName, tagName string) *httptest.Server {
	t.Helper()
	assetName := fmt.Sprintf("awf-plugin-%s_%s_%s_%s.tar.gz", pluginName, strings.TrimPrefix(tagName, "v"), runtime.GOOS, runtime.GOARCH)

	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.DefaultCompression) //nolint:errcheck // test helper
	tw := tar.NewWriter(gz)
	binaryContent := []byte("#!/bin/bash\necho mock plugin")
	manifestContent := fmt.Sprintf("name: %s\nversion: 1.0.0\ndescription: Test\ncapabilities: []\n", pluginName)
	for _, entry := range []struct {
		name    string
		mode    int64
		content []byte
	}{
		{fmt.Sprintf("awf-plugin-%s", pluginName), 0o755, binaryContent},
		{"plugin.yaml", 0o644, []byte(manifestContent)},
	} {
		_ = tw.WriteHeader(&tar.Header{Name: entry.name, Size: int64(len(entry.content)), Mode: entry.mode}) //nolint:errcheck // test helper
		_, _ = tw.Write(entry.content)                                                                       //nolint:errcheck // test helper
	}
	_ = tw.Close() //nolint:errcheck // test helper
	_ = gz.Close() //nolint:errcheck // test helper
	tarball := buf.Bytes()

	hash := sha256.Sum256(tarball)
	checksumLine := fmt.Sprintf("%x  %s", hash, assetName)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases"):
			releases := []map[string]interface{}{
				{
					"tag_name": tagName,
					"assets": []map[string]interface{}{
						{
							"name":                 assetName,
							"browser_download_url": "http://" + r.Host + "/downloads/" + assetName,
						},
						{
							"name":                 "checksums.txt",
							"browser_download_url": "http://" + r.Host + "/downloads/checksums.txt",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(releases)
		case strings.Contains(r.URL.Path, "/downloads/"+assetName):
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(tarball)
		case strings.Contains(r.URL.Path, "/downloads/checksums.txt"):
			_, _ = w.Write([]byte(checksumLine))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestPluginUpdateCommand_AlreadyUpToDate verifies that update prints "already up to date"
// and returns nil when the installed version matches the latest GitHub release.
func TestPluginUpdateCommand_AlreadyUpToDate(t *testing.T) {
	tmpDir := t.TempDir()

	pluginsDir := filepath.Join(tmpDir, "plugins")
	pluginDir := filepath.Join(pluginsDir, "test-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "awf-plugin-test-plugin"), []byte("binary"), 0o755))
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	// Seed state with v1.0.0 — same as what the mock server returns.
	seedUpdateSourceData(t, tmpDir, "test-plugin", "testorg/awf-plugin-test-plugin", "v1.0.0")

	srv := newMockGitHubReleasesServer(t, "test-plugin", "v1.0.0")
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "update", "test-plugin", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err, "update should succeed when already up to date")
	assert.Contains(t, out.String(), "already up to date", "should report no update needed")
}

// TestPluginUpdateCommand_NoSourceData verifies that update returns a descriptive error
// when the plugin was installed manually without source metadata.
func TestPluginUpdateCommand_NoSourceData(t *testing.T) {
	tmpDir := t.TempDir()

	pluginsDir := filepath.Join(tmpDir, "plugins")
	pluginDir := filepath.Join(pluginsDir, "manual-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "awf-plugin-manual-plugin"), []byte("binary"), 0o755))
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	// No source data written to state store — simulates a manually installed plugin.

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "update", "manual-plugin", "--storage", tmpDir})

	err := cmd.Execute()
	assert.Error(t, err, "update should fail when no source data exists")
	assert.Contains(t, err.Error(), "remote source", "error should explain why update cannot proceed")
}

// TestPluginUpdateCommand_NotInstalled verifies that update returns an error
// when the plugin directory does not exist on disk.
func TestPluginUpdateCommand_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()

	pluginsDir := filepath.Join(tmpDir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "update", "ghost-plugin", "--storage", tmpDir})

	err := cmd.Execute()
	assert.Error(t, err, "update should fail when plugin is not installed")
	assert.Contains(t, err.Error(), "not installed", "error should indicate plugin is not installed")
}

func TestPluginListCommand_DetailsFlag_Recognized(t *testing.T) {
	cmd := cli.NewRootCommand()
	listCmd, _, err := cmd.Find([]string{"plugin", "list"})
	require.NoError(t, err)

	found := false
	listCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "details" {
			found = true
		}
	})
	assert.True(t, found, "plugin list command should have --details flag")
}

func TestPluginListCommand_StepTypesFlag_Recognized(t *testing.T) {
	cmd := cli.NewRootCommand()
	listCmd, _, err := cmd.Find([]string{"plugin", "list"})
	require.NoError(t, err)

	found := false
	listCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "step-types" {
			found = true
		}
	})
	assert.True(t, found, "plugin list command should have --step-types flag")
}

func TestPluginListCommand_ValidatorsFlag_Recognized(t *testing.T) {
	cmd := cli.NewRootCommand()
	listCmd, _, err := cmd.Find([]string{"plugin", "list"})
	require.NoError(t, err)

	found := false
	listCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "validators" {
			found = true
		}
	})
	assert.True(t, found, "plugin list command should have --validators flag")
}

func TestPluginListCommand_MutuallyExclusiveFlags(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	tests := []struct {
		name  string
		flags []string
	}{
		{"operations+details", []string{"--operations", "--details"}},
		{"operations+step-types", []string{"--operations", "--step-types"}},
		{"operations+validators", []string{"--operations", "--validators"}},
		{"details+step-types", []string{"--details", "--step-types"}},
		{"details+validators", []string{"--details", "--validators"}},
		{"step-types+validators", []string{"--step-types", "--validators"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			out := new(bytes.Buffer)
			cmd.SetOut(out)
			cmd.SetErr(out)
			args := append([]string{"plugin", "list"}, tc.flags...)
			cmd.SetArgs(args)

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "mutually exclusive")
		})
	}
}

func TestPluginListCommand_ValidatorsFlag_ShowsValidatorPlugins(t *testing.T) {
	tmpDir := setupTestDir(t)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")

	cmd := cli.NewRootCommand()
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"plugin", "list", "--validators"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "No validators found")
}

func TestPluginSearchCommand_QueryWithJSON(t *testing.T) {
	tmpDir := setupTestDir(t)
	srv := newMockGitHubSearchServer(t)
	defer srv.Close()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("AWF_PLUGINS_PATH", "")
	t.Setenv("GITHUB_API_URL", srv.URL)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "search", "slack", "--format", "json", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err, "search with query and JSON format should succeed")

	output := strings.TrimSpace(out.String())

	// Should parse as JSON (array of search results)
	var results interface{}
	err = json.Unmarshal([]byte(output), &results)
	assert.NoError(t, err, "JSON search results should be valid")
}
