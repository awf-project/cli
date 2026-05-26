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

// TestClaudeMCPInjector exercises claudeMCPInjector in a table-driven format covering
// nil config, disabled intercept_builtins, enabled intercept_builtins, and immutability
// of the input args slice. Wrapper-file shape and cleanup behavior are validated in
// dedicated tests below.
func TestClaudeMCPInjector(t *testing.T) {
	baseArgs := []string{"-p", "test prompt", "--output-format", "stream-json"}

	tests := []struct {
		name                 string
		args                 []string
		cfg                  *workflow.MCPProxyConfig
		path                 string
		options              map[string]any
		wantArgLen           int
		wantFixedArgAt       map[int]string // index → expected value (non-wrapper paths only)
		wantWrapperAt        int            // index of the generated wrapper path; -1 to skip
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
			wantWrapperAt:        -1,
			wantOptionsUnchanged: true,
		},
		{
			name: "intercept_builtins=false appends --mcp-config <wrapper>",
			args: baseArgs,
			cfg: &workflow.MCPProxyConfig{
				Enable:            true,
				InterceptBuiltins: false,
			},
			path:    "/tmp/mcp-config.json",
			options: map[string]any{},
			// original 4 + --mcp-config + wrapper = 6
			wantArgLen: 6,
			wantFixedArgAt: map[int]string{
				4: "--mcp-config",
			},
			wantWrapperAt:        5,
			wantOptionsUnchanged: true,
		},
		{
			name: "intercept_builtins=true appends --mcp-config <wrapper> --tools '' --strict-mcp-config",
			args: baseArgs,
			cfg: &workflow.MCPProxyConfig{
				Enable:            true,
				InterceptBuiltins: true,
			},
			path:    "/tmp/mcp-config.json",
			options: map[string]any{"model": "claude-3-sonnet"},
			// original 4 + --mcp-config + wrapper + --tools + "" + --strict-mcp-config = 9
			wantArgLen: 9,
			wantFixedArgAt: map[int]string{
				4: "--mcp-config",
				6: "--tools",
				7: "",
				8: "--strict-mcp-config",
			},
			wantWrapperAt:        5,
			wantOptionsUnchanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Keep a copy to test immutability of the input args.
			argsCopy := make([]string, len(tt.args))
			copy(argsCopy, tt.args)

			newArgs, newOpts, cleanup, err := claudeMCPInjector(context.Background(), tt.args, tt.cfg, tt.path, tt.options)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err, "claudeMCPInjector must not error")
			require.NotNil(t, cleanup, "cleanup function must not be nil")
			require.NotNil(t, newOpts, "newOptions must not be nil")

			assert.Len(t, newArgs, tt.wantArgLen,
				"arg count mismatch; got %v", newArgs)

			for idx, wantVal := range tt.wantFixedArgAt {
				require.Greater(t, len(newArgs), idx, "newArgs too short for index %d", idx)
				assert.Equal(t, wantVal, newArgs[idx],
					"arg[%d] mismatch", idx)
			}

			// When a wrapper path is expected, verify it points to an existing JSON file.
			if tt.wantWrapperAt >= 0 {
				require.Greater(t, len(newArgs), tt.wantWrapperAt, "newArgs missing wrapper slot")
				wrapperPath := newArgs[tt.wantWrapperAt]
				assert.NotEqual(t, tt.path, wrapperPath,
					"wrapper path MUST differ from the internal config path (Claude expects a different schema)")
				assert.True(t, strings.HasSuffix(wrapperPath, ".json"),
					"wrapper path should end in .json, got %q", wrapperPath)
				_, statErr := os.Stat(wrapperPath)
				assert.NoError(t, statErr, "wrapper file should exist on disk before cleanup")
			}

			// Cleanup must be idempotent and (when a wrapper was created) must remove the file.
			var wrapperPath string
			if tt.wantWrapperAt >= 0 {
				wrapperPath = newArgs[tt.wantWrapperAt]
			}
			assert.NoError(t, cleanup(), "cleanup should succeed on first call")
			if wrapperPath != "" {
				_, statErr := os.Stat(wrapperPath)
				assert.True(t, os.IsNotExist(statErr),
					"wrapper file should be removed after cleanup, got stat err: %v", statErr)
			}
			assert.NoError(t, cleanup(), "cleanup should succeed on second call (idempotent)")

			// Claude never mutates options.
			if tt.wantOptionsUnchanged {
				assert.Equal(t, tt.options, newOpts, "Claude must return options unchanged")
			}

			// Input args slice must be immutable.
			assert.Equal(t, argsCopy, tt.args, "original args must not be modified")
		})
	}
}

// TestClaudeMCPInjector_WrapperFileShape verifies the on-disk JSON written by
// claudeMCPInjector has the exact shape Claude CLI expects from --mcp-config:
//
//	{ "mcpServers": { "awf-proxy": { "command": "...", "args": [...] } } }
//
// This is the regression test for the bug where AWF was passing its internal
// proxy config (with shape {"intercept_builtins", "plugin_tools"}) directly
// to --mcp-config, which Claude rejected with:
//
//	"Invalid MCP configuration: mcpServers: Invalid input: expected record, received undefined".
func TestClaudeMCPInjector_WrapperFileShape(t *testing.T) {
	internalPath := "/tmp/awf-internal-config-xyz.json"
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}

	newArgs, _, cleanup, err := claudeMCPInjector(
		context.Background(), []string{"-p", "x"}, cfg, internalPath, map[string]any{},
	)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	// Locate the wrapper path argument.
	require.GreaterOrEqual(t, len(newArgs), 4, "expected --mcp-config <wrapper> in args")
	var wrapperPath string
	for i := 0; i < len(newArgs)-1; i++ {
		if newArgs[i] == "--mcp-config" {
			wrapperPath = newArgs[i+1]
			break
		}
	}
	require.NotEmpty(t, wrapperPath, "could not find --mcp-config in newArgs")
	require.NotEqual(t, internalPath, wrapperPath,
		"wrapper path must differ from internal path — passing internal directly is the original bug")

	// Read and parse the wrapper file as Claude would.
	data, readErr := os.ReadFile(wrapperPath) //nolint:gosec // wrapperPath is generated by os.CreateTemp in this same call
	require.NoError(t, readErr, "wrapper file must exist and be readable")

	var parsed claudeMCPWrapperConfig
	require.NoError(t, json.Unmarshal(data, &parsed),
		"wrapper file must be valid JSON in claude_desktop_config.json shape")

	require.Contains(t, parsed.MCPServers, "awf-proxy",
		"wrapper must declare a server named 'awf-proxy'")

	server := parsed.MCPServers["awf-proxy"]
	assert.NotEmpty(t, server.Command, "server.command must be the resolved awf binary path")
	require.NotEmpty(t, server.Args, "server.args must include mcp-serve and --config")
	assert.Equal(t, "mcp-serve", server.Args[0], "first arg must be the mcp-serve subcommand")
	require.GreaterOrEqual(t, len(server.Args), 2, "expected at least mcp-serve and --config")
	assert.Equal(t, "--config="+internalPath, server.Args[1],
		"second arg must point to the INTERNAL config path; this is the indirection that fixes the original bug")
}

// TestClaudeMCPInjector_WrapperCleanupRemovesFile verifies the cleanup contract:
// after cleanup() returns, the temp wrapper file must no longer exist on disk.
func TestClaudeMCPInjector_WrapperCleanupRemovesFile(t *testing.T) {
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}

	newArgs, _, cleanup, err := claudeMCPInjector(
		context.Background(), []string{"-p", "x"}, cfg, "/tmp/some-internal.json", map[string]any{},
	)
	require.NoError(t, err)

	var wrapperPath string
	for i := 0; i < len(newArgs)-1; i++ {
		if newArgs[i] == "--mcp-config" {
			wrapperPath = newArgs[i+1]
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

// TestValidateClaudeOptions_MCPConfigPath tests that mcp_proxy_config_path is accepted
// as a valid option key by the Claude options validator.
func TestValidateClaudeOptions_MCPConfigPath(t *testing.T) {
	options := map[string]any{
		"mcp_proxy_config_path": "/tmp/mcp-config.json",
	}

	err := validateClaudeOptions(options)

	assert.NoError(t, err, "validateClaudeOptions should accept mcp_proxy_config_path")
}

// TestValidateClaudeOptions_Model verifies accepted and rejected model name formats.
func TestValidateClaudeOptions_Model(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		wantErr bool
	}{
		{
			name:    "alias sonnet",
			model:   "sonnet",
			wantErr: false,
		},
		{
			name:    "alias opus",
			model:   "opus",
			wantErr: false,
		},
		{
			name:    "claude-prefix",
			model:   "claude-3-sonnet-20240229",
			wantErr: false,
		},
		{
			name:    "invalid model",
			model:   "invalid-model",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := map[string]any{"model": tt.model}
			err := validateClaudeOptions(options)

			if tt.wantErr {
				assert.Error(t, err, "validateClaudeOptions should reject invalid model: %s", tt.model)
				assert.Contains(t, err.Error(), "invalid model format",
					"error message should indicate invalid model format")
			} else {
				assert.NoError(t, err, "validateClaudeOptions should accept valid model: %s", tt.model)
			}
		})
	}
}
