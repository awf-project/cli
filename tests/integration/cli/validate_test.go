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
