package agents

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCopilotMCPInjector exercises copilotMCPInjector in a table-driven format covering
// nil config, disabled intercept_builtins, enabled intercept_builtins, and immutability
// of the input args slice. Wrapper-file shape and cleanup behavior are validated in
// dedicated tests below.
func TestCopilotMCPInjector(t *testing.T) {
	baseArgs := []string{"-p", "test prompt", "--output-format=json", "--silent"}

	tests := []struct {
		name                 string
		args                 []string
		cfg                  *workflow.MCPProxyConfig
		path                 string
		options              map[string]any
		wantArgLen           int
		wantFixedArgAt       map[int]string // index → expected value (non-wrapper paths only)
		wantWrapperPrefixAt  int            // index of the generated wrapper path arg (with `@` prefix); -1 to skip
		wantWarn             bool
		wantSystemPromptPfx  string
		wantOptionsUnchanged bool
		wantErr              bool
	}{
		{
			name:                 "nil config returns args unchanged",
			args:                 baseArgs,
			cfg:                  nil,
			path:                 "/tmp/unused",
			options:              map[string]any{"key": "val"},
			wantArgLen:           4,
			wantWrapperPrefixAt:  -1,
			wantOptionsUnchanged: true,
		},
		{
			name: "intercept_builtins=false appends --additional-mcp-config @<wrapper>",
			args: baseArgs,
			cfg: &workflow.MCPProxyConfig{
				Enable:            true,
				InterceptBuiltins: false,
			},
			path:    "/tmp/mcp-config.json",
			options: map[string]any{},
			// original 4 + --additional-mcp-config + @<wrapper> = 6
			wantArgLen: 6,
			wantFixedArgAt: map[int]string{
				4: "--additional-mcp-config",
			},
			wantWrapperPrefixAt: 5,
		},
		{
			name: "intercept_builtins=true appends --additional-mcp-config @<wrapper> + --disable-builtin-mcps + WARN + system_prompt prefix",
			args: baseArgs,
			cfg: &workflow.MCPProxyConfig{
				Enable:            true,
				InterceptBuiltins: true,
			},
			path:    "/tmp/mcp-config.json",
			options: map[string]any{"model": "gpt-4o"},
			// original 4 + --additional-mcp-config + @<wrapper> + --disable-builtin-mcps = 7
			wantArgLen: 7,
			wantFixedArgAt: map[int]string{
				4: "--additional-mcp-config",
				6: "--disable-builtin-mcps",
			},
			wantWrapperPrefixAt: 5,
			wantWarn:            true,
			wantSystemPromptPfx: "Use only MCP tools, never built-in tools. ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsCopy := make([]string, len(tt.args))
			copy(argsCopy, tt.args)

			mockLog := &testLogCapture{}
			provider := NewCopilotProviderWithOptions(WithCopilotLogger(mockLog))

			newArgs, newOpts, cleanup, err := provider.copilotMCPInjector(context.Background(), tt.args, tt.cfg, tt.path, tt.options)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err, "copilotMCPInjector must not error")
			require.NotNil(t, cleanup, "cleanup function must not be nil")
			require.NotNil(t, newOpts, "newOptions must not be nil")

			assert.Len(t, newArgs, tt.wantArgLen, "arg count mismatch; got %v", newArgs)

			for idx, wantVal := range tt.wantFixedArgAt {
				require.Greater(t, len(newArgs), idx, "newArgs too short for index %d", idx)
				assert.Equal(t, wantVal, newArgs[idx], "arg[%d] mismatch", idx)
			}

			var wrapperPath string
			if tt.wantWrapperPrefixAt >= 0 {
				require.Greater(t, len(newArgs), tt.wantWrapperPrefixAt, "newArgs missing wrapper slot")
				wrapperArg := newArgs[tt.wantWrapperPrefixAt]
				require.True(t, strings.HasPrefix(wrapperArg, "@"),
					"wrapper arg must begin with '@' (Copilot's file-path prefix), got %q", wrapperArg)
				wrapperPath = strings.TrimPrefix(wrapperArg, "@")
				assert.NotEqual(t, tt.path, wrapperPath,
					"wrapper path MUST differ from the internal config path (Copilot expects a different schema)")
				assert.True(t, strings.HasSuffix(wrapperPath, ".json"),
					"wrapper path should end in .json, got %q", wrapperPath)
				_, statErr := os.Stat(wrapperPath)
				assert.NoError(t, statErr, "wrapper file should exist on disk before cleanup")
			}

			if tt.wantWarn {
				assert.Len(t, mockLog.warnCalls, 1, "should emit one WARN log when intercept_builtins=true")
				assert.Contains(t, mockLog.warnCalls[0].msg, "coexistence mode",
					"WARN message should mention coexistence mode")
			} else {
				assert.Empty(t, mockLog.warnCalls, "should not emit WARN log")
			}

			if tt.wantSystemPromptPfx != "" {
				prompt, ok := newOpts["system_prompt"].(string)
				require.True(t, ok, "system_prompt should be a string in newOpts")
				assert.True(t, strings.HasPrefix(prompt, tt.wantSystemPromptPfx),
					"system_prompt should start with %q, got %q", tt.wantSystemPromptPfx, prompt)
			}

			// Cleanup is idempotent and removes the wrapper file.
			assert.NoError(t, cleanup(), "cleanup should succeed on first call")
			if wrapperPath != "" {
				_, statErr := os.Stat(wrapperPath)
				assert.True(t, os.IsNotExist(statErr),
					"wrapper file should be removed after cleanup, got stat err: %v", statErr)
			}
			assert.NoError(t, cleanup(), "cleanup should succeed on second call (idempotent)")

			if tt.wantOptionsUnchanged {
				assert.Equal(t, tt.options, newOpts, "options must be returned unchanged when cfg is nil")
			}

			assert.Equal(t, argsCopy, tt.args, "original args must not be modified")
		})
	}
}

// TestCopilotMCPInjector_WrapperFileShape verifies the on-disk JSON written by
// copilotMCPInjector has the exact shape Copilot CLI expects from
// --additional-mcp-config:
//
//	{ "mcpServers": { "awf-proxy": { "type": "local", "command": "...", "args": [...] } } }
func TestCopilotMCPInjector_WrapperFileShape(t *testing.T) {
	internalPath := "/tmp/awf-internal-config-xyz.json"
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}

	provider := NewCopilotProviderWithOptions(WithCopilotLogger(&testLogCapture{}))
	newArgs, _, cleanup, err := provider.copilotMCPInjector(
		context.Background(), []string{"-p", "x"}, cfg, internalPath, map[string]any{},
	)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	// Locate the wrapper path argument (next arg after --additional-mcp-config, stripped of @).
	var wrapperPath string
	for i := 0; i < len(newArgs)-1; i++ {
		if newArgs[i] == "--additional-mcp-config" {
			wrapperPath = strings.TrimPrefix(newArgs[i+1], "@")
			break
		}
	}
	require.NotEmpty(t, wrapperPath, "could not find --additional-mcp-config in newArgs")
	require.NotEqual(t, internalPath, wrapperPath,
		"wrapper path must differ from internal path — passing internal directly would not parse")

	data, readErr := os.ReadFile(wrapperPath) //nolint:gosec // wrapperPath is generated by os.CreateTemp in this same call
	require.NoError(t, readErr, "wrapper file must exist and be readable")

	var parsed copilotMCPWrapperConfig
	require.NoError(t, json.Unmarshal(data, &parsed),
		"wrapper file must be valid JSON in mcpServers shape")

	require.Contains(t, parsed.MCPServers, "awf-proxy",
		"wrapper must declare a server named 'awf-proxy'")

	server := parsed.MCPServers["awf-proxy"]
	assert.Equal(t, "local", server.Type, "server.type must be 'local' for stdio MCP servers")
	assert.NotEmpty(t, server.Command, "server.command must be the resolved awf binary path")
	require.NotEmpty(t, server.Args, "server.args must include mcp-serve and --config")
	assert.Equal(t, "mcp-serve", server.Args[0], "first arg must be the mcp-serve subcommand")
	require.GreaterOrEqual(t, len(server.Args), 2, "expected at least mcp-serve and --config")
	assert.Equal(t, "--config="+internalPath, server.Args[1],
		"second arg must point to the INTERNAL config path; this is the indirection")
}

// TestCopilotMCPInjector_WrapperCleanupRemovesFile verifies the cleanup contract:
// after cleanup() returns, the temp wrapper file must no longer exist on disk.
func TestCopilotMCPInjector_WrapperCleanupRemovesFile(t *testing.T) {
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}

	provider := NewCopilotProviderWithOptions(WithCopilotLogger(&testLogCapture{}))
	newArgs, _, cleanup, err := provider.copilotMCPInjector(
		context.Background(), []string{"-p", "x"}, cfg, "/tmp/some-internal.json", map[string]any{},
	)
	require.NoError(t, err)

	var wrapperPath string
	for i := 0; i < len(newArgs)-1; i++ {
		if newArgs[i] == "--additional-mcp-config" {
			wrapperPath = strings.TrimPrefix(newArgs[i+1], "@")
			break
		}
	}
	require.NotEmpty(t, wrapperPath)
	_, statErr := os.Stat(wrapperPath)
	require.NoError(t, statErr, "wrapper must exist before cleanup")

	require.NoError(t, cleanup())
	_, statErr = os.Stat(wrapperPath)
	assert.True(t, os.IsNotExist(statErr),
		"wrapper file must be removed by cleanup, got stat err: %v", statErr)
}

// TestCopilotMCPInjector_SystemPromptMutation_NoExisting tests mutation with no existing system_prompt.
func TestCopilotMCPInjector_SystemPromptMutation_NoExisting(t *testing.T) {
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}
	options := map[string]any{"model": "gpt-4o"}

	provider := NewCopilotProviderWithOptions(WithCopilotLogger(&testLogCapture{}))
	_, newOpts, cleanup, err := provider.copilotMCPInjector(
		context.Background(), []string{"-p", "x"}, cfg, "/tmp/mcp-config.json", options,
	)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	modifiedPrompt, ok := newOpts["system_prompt"].(string)
	require.True(t, ok, "system_prompt should be a string in newOpts")
	assert.Equal(t, "Use only MCP tools, never built-in tools. ", modifiedPrompt,
		"should have MCP-only instruction when no prior prompt")

	_, hasPrompt := options["system_prompt"]
	assert.False(t, hasPrompt, "original options must not have system_prompt added")
}

// TestCopilotMCPInjector_SystemPromptMutation_ExistingPreserved tests that an existing
// system_prompt is preserved after the MCP-only prefix.
func TestCopilotMCPInjector_SystemPromptMutation_ExistingPreserved(t *testing.T) {
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}
	options := map[string]any{"system_prompt": "Existing system prompt."}

	provider := NewCopilotProviderWithOptions(WithCopilotLogger(&testLogCapture{}))
	_, newOpts, cleanup, err := provider.copilotMCPInjector(
		context.Background(), []string{"-p", "x"}, cfg, "/tmp/mcp-config.json", options,
	)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	modifiedPrompt, ok := newOpts["system_prompt"].(string)
	require.True(t, ok, "system_prompt should be a string in newOpts")
	assert.True(t, strings.HasPrefix(modifiedPrompt, "Use only MCP tools, never built-in tools. "),
		"system_prompt should start with MCP-only instruction")
	assert.Contains(t, modifiedPrompt, "Existing system prompt.",
		"original content should be preserved after the MCP-only instruction")

	assert.Equal(t, "Existing system prompt.", options["system_prompt"],
		"original options map must not be mutated")
}

// TestValidateCopilotOptions_MCPConfigPath tests that mcp_proxy_config_path is accepted
// as a valid option key by the Copilot options validator (it ignores unknown keys).
func TestValidateCopilotOptions_MCPConfigPath(t *testing.T) {
	options := map[string]any{
		"mcp_proxy_config_path": "/tmp/mcp-config.json",
	}

	err := validateCopilotOptions(options)

	assert.NoError(t, err, "validateCopilotOptions should accept mcp_proxy_config_path")
}
