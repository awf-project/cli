//go:build integration

package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubClaudeResult is a minimal valid stream-json NDJSON response for the mock executor.
const stubClaudeResult = `{"type":"result","subtype":"success","result":"ok","session_id":"s-test","usage":{"input_tokens":5,"output_tokens":3}}`

// mcpConfigFlagValue returns the value passed to --mcp-config in args, or "" if absent.
// The Claude injector wraps the internal awf proxy config in a Claude-shaped tmp file
// (awf-claude-mcp-*.json) and passes that wrapper path to --mcp-config — NOT the
// caller-provided path. Tests assert on the wrapper-prefix pattern, not the exact path.
func mcpConfigFlagValue(args []string) string {
	for i, a := range args {
		if a == "--mcp-config" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// TestClaudeMCPInjection_InterceptBuiltins_ArgsContainAllFlags verifies that when
// mcp_proxy.enable=true and intercept_builtins=true, the Claude provider injects
// --mcp-config <path>, --tools "", and --strict-mcp-config into the CLI invocation.
func TestClaudeMCPInjection_InterceptBuiltins_ArgsContainAllFlags(t *testing.T) {
	tmpDir := t.TempDir()
	mcpConfigPath := filepath.Join(tmpDir, "awf-mcp-proxy-test.json")
	require.NoError(t, os.WriteFile(mcpConfigPath, []byte(`{"intercept_builtins":true}`), 0o644))

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(stubClaudeResult), nil)

	provider := agents.NewClaudeProviderWithOptions(agents.WithClaudeExecutor(mockExec))

	opts := map[string]any{
		workflow.MCPProxyConfigKey:     &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true},
		workflow.MCPProxyConfigPathKey: mcpConfigPath,
	}

	_, err := provider.Execute(context.Background(), "hello", opts, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1, "claude binary should be invoked exactly once")
	args := calls[0].Args

	// AC: --mcp-config <wrapper-path>. The Claude injector writes a wrapper config
	// in the OS tmp dir and passes that path (not the input one); we only assert
	// on the file-name prefix.
	configFlagValue := mcpConfigFlagValue(args)
	assert.Contains(t, filepath.Base(configFlagValue), "awf-claude-mcp-",
		"--mcp-config must point at the Claude wrapper, got %q", configFlagValue)
	// AC: --strict-mcp-config
	assert.True(t, slices.Contains(args, "--strict-mcp-config"),
		"args %v must contain --strict-mcp-config when intercept_builtins=true", args)
	// AC: --tools ""
	assert.True(t, containsFlag(args, "--tools", ""),
		"args %v must contain --tools \"\" when intercept_builtins=true", args)
}

// TestClaudeMCPInjection_NoInterceptBuiltins_OnlyMCPConfig verifies that when
// intercept_builtins=false, only --mcp-config is appended (no --tools, no --strict-mcp-config).
func TestClaudeMCPInjection_NoInterceptBuiltins_OnlyMCPConfig(t *testing.T) {
	tmpDir := t.TempDir()
	mcpConfigPath := filepath.Join(tmpDir, "awf-mcp-proxy-noicept.json")
	require.NoError(t, os.WriteFile(mcpConfigPath, []byte(`{"intercept_builtins":false}`), 0o644))

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(stubClaudeResult), nil)

	provider := agents.NewClaudeProviderWithOptions(agents.WithClaudeExecutor(mockExec))

	opts := map[string]any{
		workflow.MCPProxyConfigKey:     &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false},
		workflow.MCPProxyConfigPathKey: mcpConfigPath,
	}

	_, err := provider.Execute(context.Background(), "hello", opts, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args

	configFlagValue := mcpConfigFlagValue(args)
	assert.Contains(t, filepath.Base(configFlagValue), "awf-claude-mcp-",
		"--mcp-config must point at the Claude wrapper, got %q", configFlagValue)
	assert.False(t, slices.Contains(args, "--strict-mcp-config"),
		"intercept_builtins=false must omit --strict-mcp-config")
	assert.False(t, slices.Contains(args, "--tools"),
		"intercept_builtins=false must omit --tools")
}

// TestClaudeMCPInjection_ProxyDisabled_NoMCPFlags verifies that when no MCP proxy
// options are present, Claude is invoked without any MCP-specific flags.
func TestClaudeMCPInjection_ProxyDisabled_NoMCPFlags(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(stubClaudeResult), nil)

	provider := agents.NewClaudeProviderWithOptions(agents.WithClaudeExecutor(mockExec))

	_, err := provider.Execute(context.Background(), "hello", nil, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args

	assert.False(t, slices.Contains(args, "--mcp-config"), "proxy disabled: --mcp-config must be absent")
	assert.False(t, slices.Contains(args, "--strict-mcp-config"), "proxy disabled: --strict-mcp-config must be absent")
	assert.False(t, slices.Contains(args, "--tools"), "proxy disabled: --tools must be absent")
}

// TestClaudeMCPInjection_EnableFalse_NoMCPFlags verifies that mcp_proxy.enable=false
// skips injection even when a config path is present.
func TestClaudeMCPInjection_EnableFalse_NoMCPFlags(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(stubClaudeResult), nil)

	provider := agents.NewClaudeProviderWithOptions(agents.WithClaudeExecutor(mockExec))

	opts := map[string]any{
		workflow.MCPProxyConfigKey:     &workflow.MCPProxyConfig{Enable: false, InterceptBuiltins: true},
		workflow.MCPProxyConfigPathKey: "/tmp/should-not-be-used.json",
	}

	_, err := provider.Execute(context.Background(), "hello", opts, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args

	assert.False(t, slices.Contains(args, "--mcp-config"), "enable=false must not inject --mcp-config")
	assert.False(t, slices.Contains(args, "--strict-mcp-config"))
}

// containsFlag checks whether args contains the pair [flag, value] in adjacent positions.
func containsFlag(args []string, flag, value string) bool {
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return true
		}
	}
	return false
}
