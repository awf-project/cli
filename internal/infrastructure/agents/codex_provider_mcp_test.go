package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogCapture captures log calls for testing.
type testLogCapture struct {
	warnCalls []testLogCall
}

type testLogCall struct {
	msg    string
	fields []any
}

func (m *testLogCapture) Debug(msg string, fields ...any) {}
func (m *testLogCapture) Info(msg string, fields ...any)  {}
func (m *testLogCapture) Warn(msg string, fields ...any) {
	m.warnCalls = append(m.warnCalls, testLogCall{msg, fields})
}
func (m *testLogCapture) Error(msg string, fields ...any) {}
func (m *testLogCapture) WithContext(ctx map[string]any) ports.Logger {
	return m
}

// TestCodexMCPInjector_InterceptBuiltinsTrue tests MCP injection with intercept_builtins enabled.
func TestCodexMCPInjector_InterceptBuiltinsTrue(t *testing.T) {
	args := []string{"exec", "--json"}
	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
	}
	path := "/tmp/mcp-config.json"
	options := map[string]any{}

	mockLog := &testLogCapture{}
	provider := NewCodexProviderWithOptions(func(p *CodexProvider) {
		p.logger = mockLog
	})

	newArgs, newOpts, cleanup, err := provider.codexMCPInjector(context.Background(), args, cfg, path, options)

	require.NoError(t, err, "codexMCPInjector should not error with intercept_builtins enabled")
	require.NotNil(t, cleanup, "cleanup function must not be nil")
	require.NotNil(t, newOpts, "newOptions must not be nil")

	// Should add: -c "mcp_servers.awf-proxy.command=...", -c "mcp_servers.awf-proxy.args=[...]", -s read-only
	// Original 2 args + 6 new args = 8
	assert.Len(t, newArgs, 8, "args should have 8 elements with intercept_builtins enabled")
	assert.Equal(t, "exec", newArgs[0])
	assert.Equal(t, "--json", newArgs[1])
	assert.Equal(t, "-c", newArgs[2])
	assert.True(t, strings.HasPrefix(newArgs[3], "mcp_servers.awf-proxy.command="), "should have mcp_servers command config")
	assert.Equal(t, "-c", newArgs[4])
	assert.True(t, strings.HasPrefix(newArgs[5], "mcp_servers.awf-proxy.args="), "should have mcp_servers args config")
	assert.Equal(t, "-s", newArgs[6])
	assert.Equal(t, "read-only", newArgs[7], "should have read-only value for -s sandbox flag")

	// Verify WARN log was emitted
	assert.Len(t, mockLog.warnCalls, 1, "should emit one WARN log")
	assert.True(t, strings.Contains(mockLog.warnCalls[0].msg, "coexistence mode"), "WARN message should mention coexistence mode")

	// Verify system_prompt is mutated with MCP-only instruction (T011 AC).
	prompt, _ := newOpts["system_prompt"].(string)
	assert.True(t, strings.HasPrefix(prompt, "Use only MCP tools, never built-in tools. "),
		"system_prompt should be prepended with MCP-only instruction, got: %q", prompt)

	assert.NoError(t, cleanup(), "cleanup should succeed")
}

// TestCodexMCPInjector_InterceptBuiltinsFalse tests MCP injection with intercept_builtins disabled.
func TestCodexMCPInjector_InterceptBuiltinsFalse(t *testing.T) {
	args := []string{"exec", "--json"}
	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: false,
	}
	path := "/tmp/mcp-config.json"
	options := map[string]any{"system_prompt": "original"}

	mockLog := &testLogCapture{}
	provider := NewCodexProviderWithOptions(func(p *CodexProvider) {
		p.logger = mockLog
	})

	newArgs, newOpts, cleanup, err := provider.codexMCPInjector(context.Background(), args, cfg, path, options)

	require.NoError(t, err, "codexMCPInjector should not error")
	require.NotNil(t, cleanup, "cleanup function must not be nil")
	require.NotNil(t, newOpts, "newOptions must not be nil")

	// With InterceptBuiltins=false: drops -s read-only
	// Should only add: -c "mcp_servers.awf-proxy.command=...", -c "mcp_servers.awf-proxy.args=[...]"
	// Original 2 args + 4 new args = 6
	assert.Len(t, newArgs, 6, "args should have 6 elements without intercept_builtins")
	assert.Equal(t, "exec", newArgs[0])
	assert.Equal(t, "--json", newArgs[1])
	assert.Equal(t, "-c", newArgs[2])
	assert.True(t, strings.HasPrefix(newArgs[3], "mcp_servers.awf-proxy.command="), "should have mcp_servers command config")
	assert.Equal(t, "-c", newArgs[4])
	assert.True(t, strings.HasPrefix(newArgs[5], "mcp_servers.awf-proxy.args="), "should have mcp_servers args config")

	// Verify NO WARN log was emitted
	assert.Len(t, mockLog.warnCalls, 0, "should NOT emit WARN log when intercept_builtins is false")

	// system_prompt should NOT be mutated when intercept_builtins=false
	assert.Equal(t, "original", newOpts["system_prompt"], "system_prompt should be unchanged when intercept_builtins=false")

	assert.NoError(t, cleanup(), "cleanup should succeed")
}

// TestCodexMCPInjector_SystemPromptMutation tests that system_prompt is prepended when InterceptBuiltins=true.
func TestCodexMCPInjector_SystemPromptMutation(t *testing.T) {
	args := []string{"exec", "--json"}
	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
	}
	path := "/tmp/mcp-config.json"
	options := map[string]any{
		"system_prompt": "Existing system prompt.",
	}

	mockLog := &testLogCapture{}
	provider := NewCodexProviderWithOptions(func(p *CodexProvider) {
		p.logger = mockLog
	})

	_, newOpts, cleanup, err := provider.codexMCPInjector(context.Background(), args, cfg, path, options)

	require.NoError(t, err, "codexMCPInjector should not error")
	require.NotNil(t, cleanup, "cleanup function must not be nil")
	require.NotNil(t, newOpts, "newOptions must not be nil")

	// system_prompt should have MCP-only instruction prepended
	modifiedPrompt, ok := newOpts["system_prompt"].(string)
	require.True(t, ok, "system_prompt should be a string")
	assert.True(t, strings.HasPrefix(modifiedPrompt, "Use only MCP tools, never built-in tools. "),
		"system_prompt should start with MCP-only instruction")
	assert.Contains(t, modifiedPrompt, "Existing system prompt.",
		"original content should be preserved after the MCP-only instruction")

	// Original options map must NOT be mutated
	assert.Equal(t, "Existing system prompt.", options["system_prompt"],
		"original options map must not be mutated")

	// -s read-only flag signals MCP-only mode
	joined := strings.Join(args, " ") // original args, unchanged
	_ = joined
	assert.NoError(t, cleanup(), "cleanup should succeed")
}

// TestCodexMCPInjector_SystemPromptMutation_NoExisting tests mutation with no existing system_prompt.
func TestCodexMCPInjector_SystemPromptMutation_NoExisting(t *testing.T) {
	args := []string{"exec", "--json"}
	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
	}
	path := "/tmp/mcp-config.json"
	options := map[string]any{
		"model": "o1",
	}

	mockLog := &testLogCapture{}
	provider := NewCodexProviderWithOptions(func(p *CodexProvider) {
		p.logger = mockLog
	})

	_, newOpts, _, err := provider.codexMCPInjector(context.Background(), args, cfg, path, options)

	require.NoError(t, err)
	require.NotNil(t, newOpts)

	// system_prompt should be created with just the MCP-only instruction
	modifiedPrompt, ok := newOpts["system_prompt"].(string)
	require.True(t, ok, "system_prompt should be a string in newOpts")
	assert.Equal(t, "Use only MCP tools, never built-in tools. ", modifiedPrompt,
		"should have MCP-only instruction when no prior prompt")

	// Original options must not be mutated
	_, hasPrompt := options["system_prompt"]
	assert.False(t, hasPrompt, "original options should not have system_prompt added")
}

// TestCodexMCPInjector_CommandArgUsesShellEscape verifies that the
// mcp_servers.awf-proxy.command argument uses interpolation.ShellEscape (not Go %q)
// so it is safe when the executable path contains shell-significant characters.
// This is the regression test for M3: %q is not POSIX-safe.
// interpolation.ShellEscape quotes only when shell metacharacters are present;
// for simple paths it returns the value unquoted, which is equally safe.
func TestCodexMCPInjector_CommandArgUsesShellEscape(t *testing.T) {
	args := []string{"exec", "--json"}
	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: false,
	}
	options := map[string]any{}

	provider := NewCodexProvider()
	newArgs, _, cleanup, err := provider.codexMCPInjector(context.Background(), args, cfg, "/tmp/cfg.json", options)
	require.NoError(t, err)
	require.NotNil(t, cleanup)

	// Find the command argument.
	var commandArg string
	for _, a := range newArgs {
		if strings.HasPrefix(a, "mcp_servers.awf-proxy.command=") {
			commandArg = a
			break
		}
	}
	require.NotEmpty(t, commandArg, "must have mcp_servers.awf-proxy.command argument")

	// The value after the "=" must NOT use Go double-quoting (starts with '"').
	// interpolation.ShellEscape produces either an unquoted safe identifier or a
	// POSIX single-quoted string — never Go-style double-quoting.
	value := strings.TrimPrefix(commandArg, "mcp_servers.awf-proxy.command=")
	assert.False(t, strings.HasPrefix(value, `"`),
		"command value must not use Go double-quoting: %s", value)

	assert.NoError(t, cleanup())
}

// TestCodexMCPInjector_ConfigNil tests nil config returns args unchanged.
func TestCodexMCPInjector_ConfigNil(t *testing.T) {
	args := []string{"exec", "--json"}
	options := map[string]any{"key": "val"}
	mockLog := &testLogCapture{}
	provider := NewCodexProviderWithOptions(func(p *CodexProvider) {
		p.logger = mockLog
	})

	newArgs, newOpts, cleanup, err := provider.codexMCPInjector(context.Background(), args, nil, "/tmp/unused", options)

	require.NoError(t, err)
	assert.Equal(t, args, newArgs, "args should be unchanged when config is nil")
	assert.Equal(t, options, newOpts, "options should be unchanged when config is nil")
	assert.Len(t, mockLog.warnCalls, 0, "should not emit WARN when config is nil")
	assert.NoError(t, cleanup(), "cleanup should succeed")
}

// TestCodexMCPInjector_DoesNotMutateInput verifies original args are not modified.
func TestCodexMCPInjector_DoesNotMutateInput(t *testing.T) {
	originalArgs := []string{"exec", "--json"}
	argsCopy := make([]string, len(originalArgs))
	copy(argsCopy, originalArgs)

	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
	}
	options := map[string]any{}

	provider := NewCodexProvider()
	newArgs, _, cleanup, _ := provider.codexMCPInjector(context.Background(), originalArgs, cfg, "/tmp/config.json", options)

	require.NotNil(t, cleanup)
	assert.Equal(t, argsCopy, originalArgs, "original args should not be modified")
	assert.Greater(t, len(newArgs), len(originalArgs), "new args should be longer than original")
}

// TestCodexMCPInjector_SpecialCharactersInConfigPath verifies that paths containing
// characters that would break naively-interpolated JSON (double-quotes, backslashes,
// closing brackets, spaces) are correctly JSON-escaped in the args argument.
func TestCodexMCPInjector_SpecialCharactersInConfigPath(t *testing.T) {
	tests := []struct {
		name     string
		cfgPath  string
		wantPart string // expected substring inside the args JSON value
	}{
		{
			name:     "path with double quote",
			cfgPath:  `/tmp/config"file.json`,
			wantPart: `"--config=/tmp/config\"file.json"`,
		},
		{
			name:     "path with backslash",
			cfgPath:  `/tmp/config\file.json`,
			wantPart: `"--config=/tmp/config\\file.json"`,
		},
		{
			name:     "path with closing bracket",
			cfgPath:  `/tmp/config]file.json`,
			wantPart: `"--config=/tmp/config]file.json"`,
		},
		{
			name:     "path with space",
			cfgPath:  `/tmp/my config/file.json`,
			wantPart: `"--config=/tmp/my config/file.json"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"exec", "--json"}
			cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
			options := map[string]any{}

			provider := NewCodexProvider()
			newArgs, _, cleanup, err := provider.codexMCPInjector(context.Background(), args, cfg, tt.cfgPath, options)

			require.NoError(t, err, "codexMCPInjector must not error for path: %s", tt.cfgPath)
			require.NotNil(t, cleanup)

			// Find the args value (the element after the second "-c")
			var argsValue string
			for i, a := range newArgs {
				if a == "-c" && i+1 < len(newArgs) && strings.HasPrefix(newArgs[i+1], "mcp_servers.awf-proxy.args=") {
					argsValue = newArgs[i+1]
					break
				}
			}
			require.NotEmpty(t, argsValue, "should find mcp_servers.awf-proxy.args argument")

			// Extract the JSON array part after "mcp_servers.awf-proxy.args="
			jsonPart := strings.TrimPrefix(argsValue, "mcp_servers.awf-proxy.args=")

			// Verify it is valid JSON
			var decoded []string
			require.NoError(t, json.Unmarshal([]byte(jsonPart), &decoded),
				"args value must be valid JSON array, got: %s", jsonPart)

			// Verify the second element contains the config path correctly
			require.Len(t, decoded, 2, "args array must have exactly 2 elements")
			assert.Equal(t, "mcp-serve", decoded[0])
			assert.Equal(t, "--config="+tt.cfgPath, decoded[1],
				"decoded config path must match original, no injection possible")

			// Verify the raw JSON contains the expected escape sequence
			assert.Contains(t, jsonPart, tt.wantPart,
				"raw JSON must contain properly escaped path")
		})
	}
}
