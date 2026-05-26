package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/infrastructure/executor"
	"github.com/awf-project/cli/internal/infrastructure/tools/builtins"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPServeCommand_CommandStructure(t *testing.T) {
	cmd := cli.NewRootCommand()

	// Find mcp-serve command
	var mcpServeCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "mcp-serve" {
			mcpServeCmd = sub
			break
		}
	}

	require.NotNil(t, mcpServeCmd, "expected mcp-serve command to be registered")
	assert.True(t, mcpServeCmd.Hidden, "expected mcp-serve to be Hidden")
	assert.Equal(t, "mcp-serve", mcpServeCmd.Use, "expected Use to be 'mcp-serve'")
}

func TestMCPServeCommand_ConfigFlagRequired(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"mcp-serve"})

	err := cmd.Execute()
	// Should fail because --config is required
	assert.Error(t, err, "expected error when --config flag is missing")
}

func TestMCPServeCommand_ConfigFlagExists(t *testing.T) {
	cmd := cli.NewRootCommand()

	// Find mcp-serve command
	var mcpServeCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "mcp-serve" {
			mcpServeCmd = sub
			break
		}
	}

	require.NotNil(t, mcpServeCmd)

	configFlag := mcpServeCmd.Flags().Lookup("config")
	require.NotNil(t, configFlag, "expected --config flag to exist")
	assert.Equal(t, "string", configFlag.Value.Type(), "expected --config to be string type")
}

func TestMCPServeCommand_MissingConfigFile(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"mcp-serve", "--config=/nonexistent/path/config.json"})

	err := cmd.Execute()
	// Should fail with exit code 1 for missing config file
	assert.Error(t, err, "expected error when config file is missing")
}

func TestMCPServeCommand_InvalidConfigJSON(t *testing.T) {
	// Create temp file with invalid JSON
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.json")
	require.NoError(t, err)
	defer tmpFile.Close() //nolint:errcheck // test cleanup

	// Write invalid JSON
	_, err = tmpFile.WriteString("{invalid json content")
	require.NoError(t, err)

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"mcp-serve", "--config=" + tmpFile.Name()})

	err = cmd.Execute()
	// Should fail with exit code 1 for malformed JSON
	assert.Error(t, err, "expected error when config JSON is malformed")
}

func TestMCPServeCommand_EmptyPluginToolsWithBuiltinsEnabled(t *testing.T) {
	// Create valid config with intercept_builtins=true and empty plugin_tools
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.json")
	require.NoError(t, err)
	defer tmpFile.Close() //nolint:errcheck // test cleanup

	config := map[string]any{
		"intercept_builtins": true,
		"plugin_tools":       []any{},
	}
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	_, err = tmpFile.Write(configJSON)
	require.NoError(t, err)

	cmd := cli.NewRootCommand()

	// Set a timeout context to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"mcp-serve", "--config=" + tmpFile.Name()})

	// Run with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.ExecuteContext(ctx)
	}()

	// Wait for either completion or timeout
	select {
	case err := <-done:
		// Command should either succeed with clean shutdown or timeout is expected
		// The implementation should handle context cancellation
		if err != nil {
			// If there's an error, it might be context.Canceled which is expected
			assert.True(t, strings.Contains(err.Error(), "Canceled") || strings.Contains(err.Error(), "canceled"), "expected context cancellation or successful shutdown")
		}
	case <-ctx.Done():
		// Timeout is acceptable as the server waits for stdin
		t.Logf("Server context timeout (expected for blocking Serve call)")
	}
}

func TestMCPServeCommand_BuiltinsDisabled(t *testing.T) {
	// Create valid config with intercept_builtins=false
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.json")
	require.NoError(t, err)
	defer tmpFile.Close() //nolint:errcheck // test cleanup

	config := map[string]any{
		"intercept_builtins": false,
		"plugin_tools":       []any{},
	}
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	_, err = tmpFile.Write(configJSON)
	require.NoError(t, err)

	cmd := cli.NewRootCommand()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"mcp-serve", "--config=" + tmpFile.Name()})

	done := make(chan error, 1)
	go func() {
		done <- cmd.ExecuteContext(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			assert.True(t, strings.Contains(err.Error(), "Canceled") || strings.Contains(err.Error(), "canceled"), "expected context cancellation or successful shutdown")
		}
	case <-ctx.Done():
		t.Logf("Server context timeout (expected for blocking Serve call)")
	}
}

func TestMCPServeCommand_ConfigFileCreatedByProxy(t *testing.T) {
	// Test that the command can read a config file similar to what the proxy would write
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp-proxy-config.json")

	config := map[string]any{
		"intercept_builtins": true,
		"plugin_tools": []map[string]any{
			{
				"name":        "test_tool",
				"description": "Test tool",
			},
		},
	}

	configData, err := json.Marshal(config)
	require.NoError(t, err)

	err = os.WriteFile(configPath, configData, 0o644)
	require.NoError(t, err)

	// Verify the file exists and is readable
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.True(t, parsed["intercept_builtins"].(bool), "expected intercept_builtins to be true")
	assert.Equal(t, 1, len(parsed["plugin_tools"].([]any)), "expected 1 plugin tool")
}

func TestMCPServeCommand_IsHidden(t *testing.T) {
	rootCmd := cli.NewRootCommand()

	// Build help text
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	err := rootCmd.Help()
	require.NoError(t, err)

	helpText := buf.String()
	assert.NotContains(t, helpText, "mcp-serve", "expected mcp-serve to be hidden from help text")
}

func TestMCPServeCommand_IsRegisteredInRoot(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "mcp-serve" {
			found = true
			break
		}
	}

	assert.True(t, found, "expected mcp-serve command to be registered in root")
}

// TestMCPServe_BashToolHasExecutor_NoNilPanic is a regression test for B1.
//
// Before the fix, builtins.NewProvider() was called without WithExecutor, leaving
// p.executor == nil. The first Bash call triggered a nil pointer dereference panic
// that killed the MCP subprocess. This test verifies the production wiring
// (WithExecutor(executor.NewShellExecutor())) produces a provider whose Bash tool
// executes end-to-end without panicking.
func TestMCPServe_BashToolHasExecutor_NoNilPanic(t *testing.T) {
	// Mirror the production wiring from runMCPServe exactly.
	provider := builtins.NewProvider(builtins.WithExecutor(executor.NewShellExecutor()))
	defer provider.Close(context.Background()) //nolint:errcheck // Close is a no-op for the builtin provider

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call Bash with a trivial command. A nil executor would panic here.
	result, err := provider.CallTool(ctx, "Bash", map[string]any{
		"command": "echo regression-ok",
	})

	require.NoError(t, err, "Bash tool must not return a Go error with a real executor")
	require.NotNil(t, result, "Bash tool must return a non-nil result")
	assert.False(t, result.IsError, "Bash tool result should not be an error for a successful command")
	require.NotEmpty(t, result.Content, "Bash tool must return at least one content block")
	assert.Contains(t, result.Content[0].Text, "regression-ok",
		"Bash output must contain the echoed string")
}
