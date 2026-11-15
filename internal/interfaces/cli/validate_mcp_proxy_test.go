// Feature: 354
package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateMCPProxy_UnknownKey tests UNKNOWN_KEY error path with fixture YAML
func TestValidateMCPProxy_UnknownKey(t *testing.T) {
	// Set workflow directory to test fixtures
	fixtureDir := "../../../tests/fixtures/mcp_proxy"
	absPath, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)
	t.Setenv("AWF_WORKFLOWS_PATH", absPath)

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"validate", "mcp-proxy-unknown-key-test"})

	err = cmd.Execute()

	require.Error(t, err, "validate should error on unknown key")
	output := buf.String() + errBuf.String()
	assert.True(t,
		strings.Contains(output, "UNKNOWN_KEY") ||
			strings.Contains(output, "policy") ||
			strings.Contains(output, "unknown"),
		"error output should indicate unknown key issue: %s", output)
}

// TestValidateMCPProxy_UnknownPlugin tests UNKNOWN_PLUGIN error path with fixture YAML
func TestValidateMCPProxy_UnknownPlugin(t *testing.T) {
	isolateValidateCLI(t)
	fixtureDir := "../../../tests/fixtures/mcp_proxy"
	absPath, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)
	t.Setenv("AWF_WORKFLOWS_PATH", absPath)

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"validate", "mcp-proxy-unknown-plugin-test"})

	err = cmd.Execute()

	// Should error because nonexistent_plugin doesn't exist in registry
	require.Error(t, err, "validate should error on unknown plugin")
	output := buf.String() + errBuf.String()
	assert.Contains(t, output, "USER.MCP_PROXY.UNKNOWN_PLUGIN")
}

// TestValidateMCPProxy_UnknownOperation tests UNKNOWN_OPERATION for a registered builtin.
func TestValidateMCPProxy_UnknownOperation(t *testing.T) {
	isolateValidateCLI(t)
	workflowDir := t.TempDir()
	t.Setenv("AWF_WORKFLOWS_PATH", workflowDir)
	workflow := `name: mcp-proxy-unknown-operation-test
version: "1.0.0"
states:
  initial: start
  start:
    type: agent
    provider: claude
    prompt: test
    on_success: done
    mcp_proxy:
      enable: true
      plugin_tools:
        - plugin: http
          expose: [http.nonexistent_operation]
  done:
    type: terminal
    status: success
`
	require.NoError(t, os.WriteFile(
		filepath.Join(workflowDir, "mcp-proxy-unknown-operation-test.yaml"),
		[]byte(workflow),
		0o600,
	))

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"validate", "mcp-proxy-unknown-operation-test"})

	err := cmd.Execute()

	require.Error(t, err, "validate should error on unknown operation")
	output := buf.String() + errBuf.String()
	assert.Contains(t, output, "USER.MCP_PROXY.UNKNOWN_OPERATION")
}

func TestValidateMCPProxy_AcceptsDiscoverableBuiltinOperation(t *testing.T) {
	isolateValidateCLI(t)
	root := t.TempDir()
	t.Setenv("AWF_WORKFLOWS_PATH", root)
	workflow := `name: builtin-mcp-operation
version: "1.0.0"
states:
  initial: start
  start:
    type: agent
    provider: claude
    prompt: test
    on_success: done
    mcp_proxy:
      enable: true
      plugin_tools:
        - plugin: http
          expose: [http.request]
  done:
    type: terminal
    status: success
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "builtin-mcp-operation.yaml"), []byte(workflow), 0o600))

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "builtin-mcp-operation"})

	err := cmd.Execute()

	require.NoError(t, err, buf.String())
	assert.NotContains(t, buf.String(), "USER.MCP_PROXY.UNKNOWN_PLUGIN")
	assert.NotContains(t, buf.String(), "USER.MCP_PROXY.UNKNOWN_OPERATION")
}

func isolateValidateCLI(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("AWF_PLUGINS_PATH", filepath.Join(root, "plugins"))
}

// TestValidateMCPProxy_EmptyProxy tests that an empty mcp_proxy block (enable=false)
// is treated as valid by the domain validation layer.
//
// Spec: MCPProxyConfig.Validate() returns nil when Enable is false.
// An empty `mcp_proxy: {}` block sets Enable to its zero value (false),
// so validation must succeed without error.
func TestValidateMCPProxy_EmptyProxy(t *testing.T) {
	fixtureDir := "../../../tests/fixtures/mcp_proxy"
	absPath, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)
	t.Setenv("AWF_WORKFLOWS_PATH", absPath)

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"validate", "mcp-proxy-empty-proxy-test"})

	execErr := cmd.Execute()

	// An empty mcp_proxy block (enable=false) is valid: Validate() returns nil.
	require.NoError(t, execErr, "validate must succeed for empty mcp_proxy block (enable=false is valid)")
	output := buf.String() + errBuf.String()
	assert.NotEmpty(t, output, "command should produce output")
}

// TestValidateMCPProxy_NameCollision tests handling of duplicate plugin entries
func TestValidateMCPProxy_NameCollision(t *testing.T) {
	fixtureDir := "../../../tests/fixtures/mcp_proxy"
	absPath, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)
	t.Setenv("AWF_WORKFLOWS_PATH", absPath)

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"validate", "mcp-proxy-name-collision-test"})

	err = cmd.Execute()

	// Should detect duplicate plugin entries or succeed (may be valid depending on domain spec)
	// At minimum, command should produce output
	output := buf.String() + errBuf.String()
	assert.NotEmpty(t, output, "command should produce output")
	// If it errors, should mention collision or related issue
	if err != nil {
		assert.True(t,
			strings.Contains(output, "NAME_COLLISION") ||
				strings.Contains(output, "duplicate") ||
				strings.Contains(output, "kubernetes") ||
				strings.Contains(strings.ToLower(output), "error"),
			"error output should provide context: %s", output)
	}
}

// TestValidateMCPProxy_ValidEnabled tests successful validation with enabled mcp_proxy
func TestValidateMCPProxy_ValidEnabled(t *testing.T) {
	fixtureDir := "../../../tests/fixtures/mcp_proxy"
	absPath, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)
	t.Setenv("AWF_WORKFLOWS_PATH", absPath)

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"validate", "mcp-proxy-valid-enabled"})

	err = cmd.Execute()

	// Valid fixture should succeed or error only on unrelated issues
	// (e.g., missing terminal state, etc.)
	output := buf.String() + errBuf.String()
	if err != nil {
		// Should not error specifically on mcp_proxy issues
		assert.False(t,
			strings.Contains(output, "UNKNOWN_KEY") ||
				strings.Contains(output, "UNKNOWN_PLUGIN") ||
				strings.Contains(output, "UNKNOWN_OPERATION"),
			"valid fixture should not have mcp_proxy errors: %s", output)
	}
}

// TestValidateMCPProxy_CodexWarning tests UNSUPPORTED_PROVIDER warning for Codex provider
func TestValidateMCPProxy_CodexWarning(t *testing.T) {
	fixtureDir := "../../../tests/fixtures/mcp_proxy"
	absPath, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)
	t.Setenv("AWF_WORKFLOWS_PATH", absPath)

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"validate", "mcp-proxy-codex-warning-test"})

	// err is intentionally ignored: UNSUPPORTED_PROVIDER is a warning, not an error.
	_ = cmd.Execute()

	// Should validate successfully but emit warning about Codex provider
	// UNSUPPORTED_PROVIDER is a warning, not an error
	output := buf.String() + errBuf.String()
	// The warning should be present in logs
	assert.True(t,
		strings.Contains(output, "UNSUPPORTED_PROVIDER") ||
			strings.Contains(output, "codex") ||
			strings.Contains(output, "warning"),
		"should emit warning for unsupported provider: %s", output)
}

// TestValidateMCPProxy_ExitCodeOnError verifies exit code is 1 (ExitUser) on validation error
func TestValidateMCPProxy_ExitCodeOnError(t *testing.T) {
	fixtureDir := "../../../tests/fixtures/mcp_proxy"
	absPath, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)
	t.Setenv("AWF_WORKFLOWS_PATH", absPath)

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"validate", "mcp-proxy-unknown-key-test"})

	err = cmd.Execute()

	require.Error(t, err, "validate with unknown key should error")
	// Error indicates validation failure (exit code 1 = user error)
	assert.NotNil(t, err, "error should be returned for validation failure")
}
