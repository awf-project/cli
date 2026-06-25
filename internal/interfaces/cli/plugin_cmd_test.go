package cli_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Acceptance: {"component":"integrate_plugin_init_command","criteria":["newPluginInitCommand() returns a Cobra command for awf plugin init <name>","newPluginCommand(cfg *Config) registers init without removing or changing existing plugin subcommands","awf plugin init awf-plugin-example --kind operation creates the full MVP file structure and prints next steps","omitted --kind behaves as operation","--output writes to the requested destination","--force is passed through to repository generation","unsupported --kind validator returns an explicit unsupported-kind error naming operation","repeated --kind and comma-separated --kind fail with single-kind guidance","invalid plugin names return user-facing validation errors before file writes","awf plugin init --help lists only the implemented operation kind as supported","existing plugin command tests still pass for list, install, remove, enable, disable, verify, update, and operations paths","awf plugin enable awf-plugin-example normalizes to runtime id example when the installed plugin manifest is known","awf plugin enable example continues to work"]}

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

func TestPluginCommand_HasSubcommands(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	subcommands := make(map[string]bool)
	for _, sub := range pluginCmd.Commands() {
		subcommands[sub.Name()] = true
	}

	for _, name := range []string{"list", "enable", "disable", "install", "init", "remove", "search", "update", "verify"} {
		assert.True(t, subcommands[name], "plugin command should have '%s' subcommand", name)
	}
}

func TestPluginInitCommand_ReturnsACobraCommandForAwfPluginInitName(t *testing.T) {
	cmd := cli.NewRootCommand()
	initCmd, _, err := cmd.Find([]string{"plugin", "init"})
	require.NoError(t, err)
	require.NotNil(t, initCmd)

	assert.Equal(t, "init", initCmd.Name())
	assert.Contains(t, initCmd.Use, "<name>")
	assert.NotNil(t, initCmd.RunE)
	assert.NotNil(t, initCmd.Flags().Lookup("kind"))
	assert.NotNil(t, initCmd.Flags().Lookup("output"))
	assert.NotNil(t, initCmd.Flags().Lookup("force"))
}

func TestPluginInitCommand_AwfPluginInitAwfPluginExampleKindOperationCreatesTheFullMVPFileStructureAndPrintsNextSteps(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "awf-plugin-example")

	out, err := executePluginInitCommand(
		t,
		"plugin", "init", "awf-plugin-example",
		"--kind", "operation",
		"--output", outputDir,
	)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assertPluginInitGeneratedFileSet(t, outputDir)
	assert.Equal(t, []string{
		"cd " + outputDir,
		"make test",
		"make build",
		"make install-local",
		"awf plugin enable awf-plugin-example",
		"awf plugin list --operations",
		"awf run examples/demo.yaml",
	}, nonEmptyPluginInitOutputLines(out))
}

func TestPluginInitCommand_OmittedKindBehavesAsOperation(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "awf-plugin-example")

	out, err := executePluginInitCommand(
		t,
		"plugin", "init", "awf-plugin-example",
		"--output", outputDir,
	)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assertPluginInitGeneratedFileSet(t, outputDir)
	assert.Contains(t, string(readPluginInitTestFile(t, filepath.Join(outputDir, "plugin.yaml"))), "operations")
	assert.Contains(t, string(readPluginInitTestFile(t, filepath.Join(outputDir, "main.go"))), "echo")
}

func TestPluginInitCommand_OutputWritesToTheRequestedDestination(t *testing.T) {
	requestedOutput := filepath.Join(t.TempDir(), "custom-destination")

	out, err := executePluginInitCommand(
		t,
		"plugin", "init", "awf-plugin-example",
		"--kind", "operation",
		"--output", requestedOutput,
	)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assertPluginInitGeneratedFileSet(t, requestedOutput)
	assert.Contains(t, out, "cd "+requestedOutput)
}

func TestPluginInitCommand_OutputPathContainingNextStepTextIsNotRewritten(t *testing.T) {
	requestedOutput := filepath.Join(t.TempDir(), "awf plugin operations", "awf-plugin-example")

	out, err := executePluginInitCommand(
		t,
		"plugin", "init", "awf-plugin-example",
		"--kind", "operation",
		"--output", requestedOutput,
	)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assertPluginInitGeneratedFileSet(t, requestedOutput)
	assert.Equal(t, []string{
		"cd " + requestedOutput,
		"make test",
		"make build",
		"make install-local",
		"awf plugin enable awf-plugin-example",
		"awf plugin list --operations",
		"awf run examples/demo.yaml",
	}, nonEmptyPluginInitOutputLines(out))
}

func TestPluginInitCommand_ForceIsPassedThroughToRepositoryGeneration(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "awf-plugin-example")
	require.NoError(t, os.MkdirAll(outputDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte("module stale\n"), 0o644))

	out, err := executePluginInitCommand(
		t,
		"plugin", "init", "awf-plugin-example",
		"--kind", "operation",
		"--output", outputDir,
		"--force",
	)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assertPluginInitGeneratedFileSet(t, outputDir)
	assert.NotContains(t, string(readPluginInitTestFile(t, filepath.Join(outputDir, "go.mod"))), "module stale")
}

func TestPluginInitCommand_UnsupportedKindValidatorReturnsAnExplicitUnsupportedKindErrorNamingOperation(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "awf-plugin-example")

	out, err := executePluginInitCommand(
		t,
		"plugin", "init", "awf-plugin-example",
		"--kind", "validator",
		"--output", outputDir,
	)

	require.Error(t, err, "plugin init output:\n%s", out)
	structErr := requirePluginInitCommandValidationError(t, err, "kind", "validator", "unsupported-kind")
	assert.Equal(
		t,
		`USER.INPUT.VALIDATION_FAILED: invalid plugin init kind "validator" violates unsupported-kind: unsupported plugin init kind "validator"; supported kind is "operation"`,
		structErr.Message,
	)
	assert.NoDirExists(t, outputDir)
}

func TestPluginInitCommand_RepeatedKindAndCommaSeparatedKindFailWithSingleKindGuidance(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantValue   any
		wantMessage string
	}{
		{
			name:      "repeated --kind",
			args:      []string{"plugin", "init", "awf-plugin-example", "--kind", "operation", "--kind", "validator"},
			wantValue: []string{"operation", "validator"},
			wantMessage: `USER.INPUT.VALIDATION_FAILED: invalid plugin init kind ["operation" "validator"] violates single-kind: ` +
				`choose exactly one --kind value; awf plugin init supports a single scaffold kind`,
		},
		{
			name:      "comma-separated --kind",
			args:      []string{"plugin", "init", "awf-plugin-example", "--kind", "operation,validator"},
			wantValue: "operation,validator",
			wantMessage: `USER.INPUT.VALIDATION_FAILED: invalid plugin init kind "operation,validator" violates single-kind: ` +
				`choose exactly one --kind value; supported kind is "operation"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := executePluginInitCommand(t, tt.args...)

			require.Error(t, err, "plugin init output:\n%s", out)
			structErr := requirePluginInitCommandValidationError(t, err, "kind", tt.wantValue, "single-kind")
			assert.Equal(t, tt.wantMessage, structErr.Message)
		})
	}
}

func TestPluginInitCommand_InvalidPluginNamesReturnUserFacingValidationErrorsBeforeFileWrites(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "bad-output")

	out, err := executePluginInitCommand(
		t,
		"plugin", "init", "example",
		"--output", outputDir,
	)

	require.Error(t, err, "plugin init output:\n%s", out)
	structErr := requirePluginInitCommandValidationError(t, err, "name", "example", "required-prefix")
	assert.Equal(
		t,
		`USER.INPUT.VALIDATION_FAILED: invalid plugin init name "example" violates required-prefix: plugin distribution name must start with "awf-plugin-", for example awf-plugin-example`,
		structErr.Message,
	)
	assert.NoDirExists(t, outputDir)
}

func TestPluginInitCommand_HelpListsOnlyImplementedOperationKind(t *testing.T) {
	out, err := executePluginInitCommand(t, "plugin", "init", "--help")

	require.NoError(t, err, "plugin init help output:\n%s", out)
	assert.Contains(t, out, "operation")
	assert.Contains(t, strings.ToLower(out), "supported")
	assert.Contains(t, out, "plugin scaffold kind (supported: operation)")
	assert.NotContains(t, strings.ToLower(out), "future")
	assert.NotContains(t, strings.ToLower(out), "deferred")
	assert.NotContains(t, out, "validator")
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

func TestPluginCommand_EnableAwfPluginExampleNormalizesToRuntimeIDExampleWhenInstalledPluginManifestIsKnown(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	writePluginCommandTestManifest(t, pluginsDir, "awf-plugin-example", "example")
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "enable", "awf-plugin-example", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err, "plugin enable output:\n%s", out.String())
	assert.Contains(t, out.String(), "example")
	assert.NotContains(t, out.String(), "awf-plugin-example")
}

func TestPluginCommand_EnableExampleContinuesToWork(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	writePluginCommandTestManifest(t, pluginsDir, "awf-plugin-example", "example")
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "enable", "example", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err, "plugin enable output:\n%s", out.String())
	assert.Contains(t, out.String(), "example")
	assert.Contains(t, out.String(), "enabled")
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

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, "json-enable-plugin", result["name"])
	assert.Equal(t, true, result["enabled"])
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

func TestPluginVerifyCommand_IsRegistered(t *testing.T) {
	cmd := cli.NewRootCommand()
	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)

	found := false
	for _, sub := range pluginCmd.Commands() {
		if sub.Name() == "verify" {
			found = true
			break
		}
	}

	assert.True(t, found, "plugin command should have 'verify' subcommand")
}

func TestPluginVerifyCommand_UpdateFlagExists(t *testing.T) {
	cmd := cli.NewRootCommand()
	verifyCmd, _, err := cmd.Find([]string{"plugin", "verify"})
	require.NoError(t, err)

	var updateFlag *pflag.Flag
	verifyCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "update" {
			updateFlag = f
		}
	})

	require.NotNil(t, updateFlag, "verify command should have --update flag")
	assert.Equal(t, "bool", updateFlag.Value.Type(), "--update should be a boolean flag")
}

func TestPluginVerifyCommand_VerifyAllPluginsWithPass(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plugin directory with a binary
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "verify-test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	// Create plugin manifest
	manifestContent := `name: verify-test-plugin
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

	// Create binary file
	binaryPath := filepath.Join(testPluginDir, "awf-plugin-verify-test-plugin")
	require.NoError(t, os.WriteFile(binaryPath, []byte("test binary"), 0o755))

	// Compute hash
	hash := sha256.Sum256([]byte("test binary"))
	expectedHash := hex.EncodeToString(hash[:])

	// Create plugin state with stored checksum matching the binary hash
	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	stateContent := fmt.Sprintf(`{
		"verify-test-plugin": {
			"enabled": true,
			"config": {},
			"checksum": "%s"
		}
	}`, expectedHash)
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
	cmd.SetArgs([]string{"plugin", "verify", "--storage", stateDir})

	err := cmd.Execute()
	require.NoError(t, err, "verify should succeed when all plugins pass")

	output := out.String()
	assert.Contains(t, output, "verify-test-plugin", "output should show plugin name")
	assert.Contains(t, output, "PASS", "output should show PASS status")
}

func TestPluginVerifyCommand_VerifyNamedPluginsWithFail(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plugin directory
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "fail-test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: fail-test-plugin
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

	// Create binary file
	binaryPath := filepath.Join(testPluginDir, "awf-plugin-fail-test-plugin")
	require.NoError(t, os.WriteFile(binaryPath, []byte("actual binary"), 0o755))

	// Compute actual hash
	actualHash := sha256.Sum256([]byte("actual binary"))
	actualHashHex := hex.EncodeToString(actualHash[:])

	// Store different hash (simulating checksum mismatch)
	expectedHash := sha256.Sum256([]byte("different binary"))
	expectedHashHex := hex.EncodeToString(expectedHash[:])

	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	stateContent := fmt.Sprintf(`{
		"fail-test-plugin": {
			"enabled": true,
			"config": {},
			"checksum": "%s"
		}
	}`, expectedHashHex)
	require.NoError(t, os.WriteFile(
		filepath.Join(stateDir, "plugins.json"),
		[]byte(stateContent),
		0o644,
	))

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"plugin", "verify", "fail-test-plugin", "--storage", stateDir})

	err := cmd.Execute()
	require.Error(t, err, "verify should fail when checksum mismatches")

	output := out.String() + errOut.String()
	assert.Contains(t, output, "fail-test-plugin", "output should show plugin name")
	assert.Contains(t, output, "FAIL", "output should show FAIL status")
	assert.Contains(t, output, expectedHashHex, "output should show expected hash")
	assert.Contains(t, output, actualHashHex, "output should show actual hash")
}

func TestPluginVerifyCommand_VerifyMissingChecksum(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plugin directory
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "missing-checksum-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: missing-checksum-plugin
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

	// Create binary file
	binaryPath := filepath.Join(testPluginDir, "awf-plugin-missing-checksum-plugin")
	require.NoError(t, os.WriteFile(binaryPath, []byte("test binary"), 0o755))

	// Create plugin state WITHOUT checksum
	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	stateContent := `{
		"missing-checksum-plugin": {
			"enabled": true,
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
	cmd.SetArgs([]string{"plugin", "verify", "missing-checksum-plugin", "--storage", stateDir})

	err := cmd.Execute()
	require.Error(t, err, "verify should fail when checksum is missing")

	output := out.String()
	assert.Contains(t, output, "missing-checksum-plugin", "output should show plugin name")
	assert.Contains(t, output, "MISSING", "output should show MISSING status")
}

func TestPluginVerifyCommand_UpdateFlagRecomputesChecksum(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plugin directory
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "update-test-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: update-test-plugin
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

	// Create binary file
	binaryContent := []byte("new binary content")
	binaryPath := filepath.Join(testPluginDir, "awf-plugin-update-test-plugin")
	require.NoError(t, os.WriteFile(binaryPath, binaryContent, 0o755))

	// Compute actual hash
	actualHash := sha256.Sum256(binaryContent)
	actualHashHex := hex.EncodeToString(actualHash[:])

	// Create plugin state with old/different checksum
	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	oldHash := sha256.Sum256([]byte("old binary"))
	oldHashHex := hex.EncodeToString(oldHash[:])

	stateContent := fmt.Sprintf(`{
		"update-test-plugin": {
			"enabled": true,
			"config": {},
			"checksum": "%s"
		}
	}`, oldHashHex)
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
	cmd.SetArgs([]string{"plugin", "verify", "update-test-plugin", "--update", "--storage", stateDir})

	err := cmd.Execute()
	require.NoError(t, err, "verify --update should succeed")

	// Verify the state was updated
	stateData, err := os.ReadFile(filepath.Join(stateDir, "plugins.json"))
	require.NoError(t, err)

	var state map[string]map[string]any
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	assert.Equal(t, actualHashHex, state["update-test-plugin"]["checksum"], "checksum should be updated to actual hash")
}

func TestPluginVerifyCommand_PluginNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin state directory
	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	stateContent := `{}`
	require.NoError(t, os.WriteFile(
		filepath.Join(stateDir, "plugins.json"),
		[]byte(stateContent),
		0o644,
	))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"plugin", "verify", "nonexistent-plugin", "--storage", stateDir})

	err := cmd.Execute()
	require.Error(t, err, "verify should error when plugin not found")

	output := out.String() + errOut.String()
	assert.Contains(t, output, "nonexistent-plugin", "output should mention the missing plugin")
}

func TestPluginVerifyCommand_ExitCodeZeroOnPass(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plugin with matching checksum
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "pass-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: pass-plugin
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

	binaryContent := []byte("pass binary")
	binaryPath := filepath.Join(testPluginDir, "awf-plugin-pass-plugin")
	require.NoError(t, os.WriteFile(binaryPath, binaryContent, 0o755))

	hash := sha256.Sum256(binaryContent)
	hashHex := hex.EncodeToString(hash[:])

	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	stateContent := fmt.Sprintf(`{
		"pass-plugin": {
			"enabled": true,
			"config": {},
			"checksum": "%s"
		}
	}`, hashHex)
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
	cmd.SetArgs([]string{"plugin", "verify", "pass-plugin", "--storage", stateDir})

	err := cmd.Execute()
	require.NoError(t, err, "should exit with code 0 when verification passes")
}

func TestPluginVerifyCommand_ExitCodeOneOnFail(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plugin with mismatching checksum
	pluginsDir := filepath.Join(tmpDir, "plugins")
	testPluginDir := filepath.Join(pluginsDir, "fail-plugin")
	require.NoError(t, os.MkdirAll(testPluginDir, 0o755))

	manifestContent := `name: fail-plugin
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

	binaryPath := filepath.Join(testPluginDir, "awf-plugin-fail-plugin")
	require.NoError(t, os.WriteFile(binaryPath, []byte("actual"), 0o755))

	// Store different hash
	differentHash := sha256.Sum256([]byte("different"))
	hashHex := hex.EncodeToString(differentHash[:])

	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	stateContent := fmt.Sprintf(`{
		"fail-plugin": {
			"enabled": true,
			"config": {},
			"checksum": "%s"
		}
	}`, hashHex)
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
	cmd.SetArgs([]string{"plugin", "verify", "fail-plugin", "--storage", stateDir})

	err := cmd.Execute()
	require.Error(t, err, "should exit with code 1 when verification fails")
}

func executePluginInitCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append(args, "--storage", filepath.Join(t.TempDir(), "storage")))

	err := cmd.Execute()
	return out.String(), err
}

func requirePluginInitCommandValidationError(t *testing.T, err error, field string, value any, rule string) *domerrors.StructuredError {
	t.Helper()

	var structErr *domerrors.StructuredError
	require.True(t, errors.As(err, &structErr), "error must be a *domerrors.StructuredError; got %T: %v", err, err)
	assert.Equal(t, domerrors.ErrorCodeUserInputValidationFailed, structErr.Code)
	assert.Equal(t, 1, structErr.ExitCode())
	assert.Equal(t, field, structErr.Details["field"])
	assert.Equal(t, value, structErr.Details["value"])
	assert.Equal(t, rule, structErr.Details["rule"])

	return structErr
}

func writePluginCommandTestManifest(t *testing.T, pluginsDir, directoryName, runtimeID string) {
	t.Helper()

	pluginDir := filepath.Join(pluginsDir, directoryName)
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	manifestContent := fmt.Sprintf(`name: %s
version: 1.0.0
awf_version: ">=0.1.0"
capabilities:
  - operations
`, runtimeID)
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(manifestContent), 0o644))
}
