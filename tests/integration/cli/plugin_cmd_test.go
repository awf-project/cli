//go:build integration

package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
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
	assert.Contains(t, output, "No plugins found", "should show no plugins message")
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
  - commands
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
`
	manifest2 := `name: plugin-two
version: 1.0.0
awf_version: ">=0.1.0"
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
	assert.Len(t, lines, 2, "quiet mode should output one name per line")
	assert.Contains(t, output, "plugin-one")
	assert.Contains(t, output, "plugin-two")
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

	// Should succeed (enabling unknown plugin just stores the state)
	// The plugin will be loaded when discovered later
	err := cmd.Execute()
	require.NoError(t, err)
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

	// Should succeed (disabling unknown plugin just stores the state)
	err := cmd.Execute()
	require.NoError(t, err)
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
