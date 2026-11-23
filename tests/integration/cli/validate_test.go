//go:build integration

// Feature: 354
package cli_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validationPluginName = "awf-plugin-validation-test"

func TestValidateAndDryRun_AcceptDiscoveredExternalPluginOperation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")
	workflowsDir := filepath.Join(root, "workflows")
	storageDir := filepath.Join(root, "storage")
	pluginDir := filepath.Join(pluginsDir, validationPluginName)
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	repoRoot, err := findRepoRoot()
	require.NoError(t, err)
	fixtureSource := filepath.Join(repoRoot, "tests", "fixtures", "mcp_proxy", "mcp-proxy-known-external-operation.yaml")
	fixturePath := filepath.Join(workflowsDir, "known-external-operation.yaml")
	copyFile(t, fixtureSource, fixturePath, 0o600)
	installCurrentTestExecutable(t, filepath.Join(pluginDir, validationPluginName))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(`name: awf-plugin-validation-test
version: 1.0.0
awf_version: ">=0.1.0"
capabilities:
  - operations
`), 0o600))

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)
	t.Setenv("AWF_WORKFLOWS_PATH", workflowsDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "xdg-state"))

	probeCtx, cancelProbe := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancelProbe)
	probe, err := pluginmgr.InitSystem(probeCtx, []string{pluginsDir}, filepath.Join(storageDir, "probe-plugins"), "", ports.NopLogger{})
	require.NoError(t, err)
	cleanupProbe := sync.OnceFunc(probe.Cleanup)
	t.Cleanup(cleanupProbe)
	schema, ok := probe.Manager.GetOperation(validationPluginName + ".echo")
	require.True(t, ok)
	assert.Equal(t, validationPluginName+".echo", schema.Name)
	assert.Equal(t, validationPluginName, schema.PluginName)
	cleanupProbe()
	cancelProbe()

	validateOutput, validateErr := executeAutoFacadeCommand(t, []string{"--storage=" + storageDir, "validate", "known-external-operation"})
	require.NoError(t, validateErr, validateOutput)
	assert.Contains(t, validateOutput, "is valid")
	assert.NotContains(t, validateOutput, "USER.MCP_PROXY.UNKNOWN_PLUGIN")
	assert.NotContains(t, validateOutput, "USER.MCP_PROXY.UNKNOWN_OPERATION")

	dryRunOutput, dryRunErr := executeAutoFacadeCommand(t, []string{"--storage=" + storageDir, "run", "known-external-operation", "--dry-run"})
	require.NoError(t, dryRunErr, dryRunOutput)
	assert.Contains(t, dryRunOutput, "Dry Run")
	assert.NotContains(t, dryRunOutput, "USER.MCP_PROXY.UNKNOWN_PLUGIN")
	assert.NotContains(t, dryRunOutput, "USER.MCP_PROXY.UNKNOWN_OPERATION")

	missingOperationSource := strings.ReplaceAll(
		string(requireReadFile(t, fixturePath)),
		validationPluginName+".echo",
		validationPluginName+".missing",
	)
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "missing-external-operation.yaml"), []byte(missingOperationSource), 0o600))
	negativeOutput, negativeErr := executeAutoFacadeCommand(t, []string{"--storage=" + storageDir, "validate", "missing-external-operation"})
	require.Error(t, negativeErr)
	negativeDiagnostic := negativeOutput + negativeErr.Error()
	assert.Contains(t, negativeDiagnostic, "USER.MCP_PROXY.UNKNOWN_OPERATION")
	assert.NotContains(t, negativeDiagnostic, "USER.MCP_PROXY.UNKNOWN_PLUGIN")
}

func TestValidateCommand_OwnsDiscoveredNotifierAdapterLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")
	pluginDir := filepath.Join(pluginsDir, validationPluginName)
	workflowsDir := filepath.Join(root, "workflows")
	storageDir := filepath.Join(root, "storage")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".awf"), 0o755))
	installCurrentTestExecutable(t, filepath.Join(pluginDir, validationPluginName))
	const adapterCredentialCanary = "adapter-credential-output-canary"
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(`name: awf-plugin-validation-test
version: 1.0.0
awf_version: ">=0.1.0"
capabilities:
  - operations
config:
  adapter_credential:
    type: string
    default: adapter-credential-output-canary
`), 0o600))

	repoRoot, err := findRepoRoot()
	require.NoError(t, err)
	fixture := string(requireReadFile(t, filepath.Join(repoRoot, "tests", "fixtures", "workflows", "http-get-test.yaml")))
	fixture = strings.Replace(fixture, "name: http-get-test", "name: notifier-validation", 1)
	fixture = strings.Replace(fixture, "operation: http.request", "operation: notifier.send", 1)
	fixture = strings.Replace(
		fixture,
		"      method: GET\n      url: \"{{inputs.url}}\"",
		"      text: workflow-text-output-canary\n      rich:\n        marker: workflow-rich-output-canary",
		1,
	)
	workflowPath := filepath.Join(workflowsDir, "notifier-validation.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(fixture), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(`notifier:
  backend: awf-plugin-validation-test
  default_channel: notifier-channel-output-canary
`), 0o600))

	lifecycleRecord := filepath.Join(root, "lifecycle.log")
	dispatchRecord := filepath.Join(root, "dispatch.log")
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)
	t.Setenv("AWF_WORKFLOWS_PATH", workflowsDir)
	t.Setenv("AWF_VALIDATION_LIFECYCLE_RECORD", lifecycleRecord)
	t.Setenv("AWF_VALIDATION_DISPATCH_RECORD", dispatchRecord)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "xdg-state"))
	t.Chdir(root)
	hostLog, err := os.CreateTemp(root, "validation-host-*.log")
	require.NoError(t, err)
	previousStderr := os.Stderr
	os.Stderr = hostLog
	t.Cleanup(func() {
		os.Stderr = previousStderr
		require.NoError(t, hostLog.Close())
	})

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	cleanupCommand := sync.OnceFunc(cleanup)
	t.Cleanup(cleanupCommand)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--no-color", "--storage=" + storageDir, "validate", "notifier-validation"})

	require.NoError(t, cmd.Execute())
	cleanupDone := make(chan struct{})
	go func() {
		cleanupCommand()
		close(cleanupDone)
	}()
	select {
	case <-cleanupDone:
	case <-time.After(5 * time.Second):
		t.Fatal("adapter cleanup did not complete within 5s")
	}

	assert.Equal(t, "Workflow 'notifier-validation' is valid\n", stdout.String())
	assert.Empty(t, stderr.String())
	require.NoError(t, hostLog.Sync())
	hostLogContents, readErr := os.ReadFile(hostLog.Name()) //nolint:gosec // test-owned host-log destination
	require.NoError(t, readErr)
	for _, canary := range []string{"workflow-text-output-canary", "workflow-rich-output-canary", "notifier-channel-output-canary", adapterCredentialCanary} {
		assert.NotContains(t, stdout.String(), canary)
		assert.NotContains(t, stderr.String(), canary)
		assert.NotContains(t, string(hostLogContents), canary)
	}
	lifecycle := string(requireReadFile(t, lifecycleRecord))
	assert.Equal(t, 1, strings.Count(lifecycle, "start:"))
	assert.Equal(t, 1, strings.Count(lifecycle, "exit:"))
	dispatches, readErr := os.ReadFile(dispatchRecord) //nolint:gosec // test-owned record path
	if !os.IsNotExist(readErr) {
		require.NoError(t, readErr)
	}
	assert.Empty(t, dispatches)

	t.Run("skip plugins keeps the notifier schema", func(t *testing.T) {
		output, err := executeAutoFacadeCommand(t, []string{
			"--no-color", "--storage=" + storageDir,
			"validate", "notifier-validation", "--skip-plugins",
		})
		require.NoError(t, err, "%s", output)
		assert.Equal(t, "Workflow 'notifier-validation' is valid\n", output)
		assert.Equal(t, lifecycle, string(requireReadFile(t, lifecycleRecord)))
	})
}

func TestValidateCommand_ChecksNotifierBackend(t *testing.T) {
	const workflow = `name: notifier-backend-validation
version: "1.0.0"
states:
  initial: send
  send:
    type: operation
    operation: notifier.send
    inputs:
      channel: general
      text: hello
    on_success: done
  done:
    type: terminal
    status: success
`
	tests := []struct {
		name         string
		config       string
		backendInput string
		wantError    bool
	}{
		{name: "project default", config: "notifier:\n  backend: slack\n"},
		{name: "explicit override", config: "notifier:\n  backend: x.y\n", backendInput: "slack"},
		{name: "missing backend", config: "{}\n", wantError: true},
		{name: "dotted explicit backend", config: "notifier:\n  backend: slack\n", backendInput: "x.y", wantError: true},
		{name: "dotted project default", config: "notifier:\n  backend: x.y\n", wantError: true},
		{name: "explicit null", config: "notifier:\n  backend: slack\n", backendInput: "null", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			configPath := filepath.Join(root, "config.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(tt.config), 0o600))
			definition := workflow
			if tt.backendInput != "" {
				definition = strings.Replace(workflow, "      channel:", "      backend: "+tt.backendInput+"\n      channel:", 1)
			}
			require.NoError(t, os.WriteFile(filepath.Join(root, "notifier-backend-validation.yaml"), []byte(definition), 0o600))

			t.Setenv("AWF_CONFIG_PATH", configPath)
			t.Setenv("AWF_WORKFLOWS_PATH", root)
			t.Setenv("AWF_PLUGINS_PATH", filepath.Join(root, "plugins"))
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
			t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
			t.Chdir(root)

			for _, mode := range []struct {
				name  string
				flags []string
			}{
				{name: "default"},
				{name: "skip plugins", flags: []string{"--skip-plugins"}},
			} {
				t.Run(mode.name, func(t *testing.T) {
					args := []string{"--no-color", "--storage=" + filepath.Join(root, "storage"), "validate", "notifier-backend-validation"}
					output, err := executeAutoFacadeCommand(t, append(args, mode.flags...))
					if tt.wantError {
						require.Error(t, err, "%s", output)
						assert.Contains(t, output, `operation "notifier.send"`)
						assert.Regexp(t, `inputs\.backend|required field "backend"`, output)
						return
					}
					require.NoError(t, err, "%s", output)
					assert.Contains(t, output, "Workflow 'notifier-backend-validation' is valid")
				})
			}
		})
	}
}

func executeAutoFacadeCommand(t *testing.T, args []string) (string, error) {
	t.Helper()
	cmd, cleanup := cli.NewRootCommandAutoFacade()
	cleanupCommand := sync.OnceFunc(cleanup)
	t.Cleanup(cleanupCommand)
	output := new(bytes.Buffer)
	cmd.SetOut(output)
	cmd.SetErr(output)
	cmd.SetArgs(args)
	err := cmd.Execute()
	cleanupCommand()
	return output.String(), err
}

func installCurrentTestExecutable(t *testing.T, destination string) {
	t.Helper()
	source, err := os.Executable()
	require.NoError(t, err)
	copyFile(t, source, destination+".tmp", 0o755)
	require.NoError(t, os.Rename(destination+".tmp", destination))
}

func copyFile(t *testing.T, source, destination string, mode os.FileMode) {
	t.Helper()
	in, err := os.Open(source) //nolint:gosec // test-owned fixture path
	require.NoError(t, err)
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode) //nolint:gosec // test-owned destination
	require.NoError(t, err)
	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Close())
	require.NoError(t, os.Chmod(destination, mode))
}

func requireReadFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path) //nolint:gosec // test-owned fixture path
	require.NoError(t, err)
	return contents
}

func TestValidateCommand_NoArgs(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no workflow name provided")
	}
}

func TestValidateCommand_NotFound(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "nonexistent-workflow"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

func TestValidateCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "validate" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'validate' subcommand")
	}
}

func TestValidateCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Validate") {
		t.Errorf("expected help text, got: %s", output)
	}
}
