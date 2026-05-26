package cli

// White-box tests for the plugin_tools resolution paths in runMCPServe.
// These require package-level access to runMCPServe, Deps, and mcpProxyConfig.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	apptools "github.com/awf-project/cli/internal/application/tools"
	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/tools/builtins"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/mcpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeProxyConfig writes a mcpProxyConfig as JSON to a temp file and returns the path.
func writeProxyConfig(t *testing.T, cfg mcpProxyConfig) string {
	t.Helper()
	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	f, err := os.CreateTemp(t.TempDir(), "mcp-proxy-*.json")
	require.NoError(t, err)
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// TestRunMCPServe_InProcessPath_RegistersPluginTool verifies the in-process deps path:
// when Deps.OperationProviders is populated, runMCPServe registers the named plugin tool
// on the MCP server without calling initPluginSystem.
//
// This is AC-2 evidence: a step with plugin_tools [{plugin: "test-plugin", expose: ["op"]}]
// results in the server registering "test-plugin_op" as a callable tool.
func TestRunMCPServe_InProcessPath_RegistersPluginTool(t *testing.T) {
	// Arrange: mock provider with one operation "op".
	mockProvider := mocks.NewMockOperationProvider()
	mockProvider.AddOperation(&pluginmodel.OperationSchema{
		Name:        "op",
		Description: "test operation",
		PluginName:  "test-plugin",
		Inputs:      map[string]pluginmodel.InputSchema{},
	})

	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{
			"test-plugin": mockProvider,
		},
	}

	configPath := writeProxyConfig(t, mcpProxyConfig{
		InterceptBuiltins: false,
		PluginTools: []apptools.PluginToolSpec{
			{Plugin: "test-plugin", Expose: []string{"op"}},
		},
	})

	// Act: use a context that is cancelled immediately after the server starts so
	// Serve returns quickly. The important side-effect is that registerTools was
	// called before Serve — validated via the cancelled-context return path.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling; Serve sees ctx.Done on first check

	err := runMCPServe(ctx, deps, configPath)

	// Assert: cancelled context yields nil (clean shutdown), not an error.
	// If registration failed, runMCPServe would have returned a mcpServeError before Serve.
	assert.NoError(t, err, "runMCPServe should succeed when plugin is found in Deps")
}

// TestRunMCPServe_InProcessPath_UnknownPlugin verifies that when Deps.OperationProviders
// does not contain the requested plugin, runMCPServe returns UNKNOWN_PLUGIN.
func TestRunMCPServe_InProcessPath_UnknownPlugin(t *testing.T) {
	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{
			"other-plugin": mocks.NewMockOperationProvider(),
		},
	}

	configPath := writeProxyConfig(t, mcpProxyConfig{
		InterceptBuiltins: false,
		PluginTools: []apptools.PluginToolSpec{
			{Plugin: "missing-plugin", Expose: []string{"op"}},
		},
	})

	ctx := context.Background()
	err := runMCPServe(ctx, deps, configPath)

	require.Error(t, err, "runMCPServe should return error when plugin is not found")
	assert.True(
		t,
		strings.Contains(err.Error(), string(domerrors.ErrorCodeUserMCPProxyUnknownPlugin)),
		"error should contain UNKNOWN_PLUGIN code, got: %s", err.Error(),
	)
}

// TestRunMCPServe_SubprocessPath_NoPluginDirs verifies that the subprocess bootstrap path
// (empty Deps) does NOT return USER.MCP_PROXY.UNSUPPORTED_PROVIDER — the feature is now
// supported for all providers. When no matching plugin is found on disk (empty plugin dir),
// the error is an operation resolution failure, not an "unsupported" architecture gate.
//
// This is the key AC-2 correctness test: before the fix, the stdio path returned
// UNSUPPORTED_PROVIDER immediately. After the fix, runMCPServe attempts to bootstrap the
// plugin system and returns a plugin-not-found variant instead.
func TestRunMCPServe_SubprocessPath_NoPluginDirs(t *testing.T) {
	// Override plugin discovery to an empty temp dir so initPluginSystem finds nothing useful.
	t.Setenv("AWF_PLUGINS_PATH", t.TempDir())

	configPath := writeProxyConfig(t, mcpProxyConfig{
		InterceptBuiltins: false,
		PluginTools: []apptools.PluginToolSpec{
			{Plugin: "awf-plugin-time", Expose: []string{"time"}},
		},
	})

	ctx := context.Background()
	err := runMCPServe(ctx, Deps{}, configPath)

	// The error should be something about the plugin/operation not being found,
	// NOT the old UNSUPPORTED_PROVIDER short-circuit that blocked the feature entirely.
	require.Error(t, err, "runMCPServe should return error when plugin is not installed")
	assert.False(
		t,
		strings.Contains(err.Error(), string(domerrors.ErrorCodeUserMCPProxyUnsupportedProvider)),
		"error must NOT contain UNSUPPORTED_PROVIDER — the stdio proxy path now supports plugin_tools; got: %s", err.Error(),
	)
	// The error is either UNKNOWN_PLUGIN (no plugin dir found) or a plugin-adapter error
	// (plugin dir exists but the plugin binary hasn't been installed yet).
	assert.True(
		t,
		strings.Contains(err.Error(), string(domerrors.ErrorCodeUserMCPProxyUnknownPlugin)) ||
			strings.Contains(err.Error(), "unknown operation") ||
			strings.Contains(err.Error(), "plugin adapter"),
		"error should indicate plugin resolution failure, got: %s", err.Error(),
	)
}

// TestRunMCPServe_SubprocessPath_NoPluginTools verifies that empty plugin_tools with
// empty Deps starts the server normally (no bootstrap needed).
func TestRunMCPServe_SubprocessPath_NoPluginTools(t *testing.T) {
	configPath := writeProxyConfig(t, mcpProxyConfig{
		InterceptBuiltins: false,
		PluginTools:       nil,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runMCPServe(ctx, Deps{}, configPath)
	// Cancelled context yields nil on clean shutdown.
	assert.NoError(t, err, "empty plugin_tools with empty Deps should not error")
}

// TestWireFormat_BuiltinTools_AllHaveDescription is a forensic wire-format test.
//
// It drives the MCP server's registerTools path directly (no subprocess, no OS pipe)
// by constructing a builtins.Provider, listing its tools, registering them on a real
// mcpserver.Server, and then serving a tools/list request from an in-memory reader.
//
// The assertion: every tool in the tools/list JSON response has a non-empty "description"
// field. This locks in the wire-format enrichment that unblocks Gemini from calling
// the tools (Gemini refuses opaque tools with no description).
func TestWireFormat_BuiltinTools_AllHaveDescription(t *testing.T) {
	// Build a builtins provider and list its tools (mirrors production wiring in runMCPServe).
	provider := builtins.NewProvider() // no executor needed: only ListTools is called
	tools, err := provider.ListTools(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, tools, "expected at least one builtin tool")

	// Wire the tools onto a real MCP server.
	srv := mcpserver.New()
	registerTools(srv, provider, tools)

	// Prepare an in-memory stdin with initialize + tools/list.
	const input = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"

	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Serve in a goroutine; the server exits when stdin is exhausted.
	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(ctx, stdin, &stdout)
	}()
	<-done

	// Parse all JSON-RPC responses from stdout.
	scanner := bufio.NewScanner(&stdout)
	var toolsListResult map[string]any
	for scanner.Scan() {
		var resp map[string]any
		if jsonErr := json.Unmarshal(scanner.Bytes(), &resp); jsonErr != nil {
			continue
		}
		// The tools/list response has id=2.
		if id, ok := resp["id"].(float64); ok && id == 2 {
			result, _ := resp["result"].(map[string]any)
			toolsListResult = result
			break
		}
	}

	require.NotNil(t, toolsListResult, "tools/list response must be present in output")

	rawTools, ok := toolsListResult["tools"].([]any)
	require.True(t, ok, "tools/list result must have a 'tools' array")
	require.NotEmpty(t, rawTools, "tools array must not be empty")

	// Assert every tool has a non-empty description in the wire response.
	for _, raw := range rawTools {
		toolMap, ok := raw.(map[string]any)
		require.True(t, ok, "each tool entry must be a JSON object")

		name, _ := toolMap["name"].(string)
		desc, _ := toolMap["description"].(string)
		assert.NotEmpty(t, desc, "tool %q must have a non-empty description in the tools/list wire response", name)
	}
}

// TestWireFormat_PluginTools_HaveDescriptionWithOutputs verifies that a plugin tool
// registered via a PluginToolAdapter carries a description composed from the
// OperationSchema.Description and Outputs in the wire response.
//
// Rather than redirecting os.Stdin/os.Stdout (which causes test-level races in parallel
// runs), this test assembles the MCP server directly using the exported mcpserver.Server
// and the unexported registerTools helper — the same code path used by runMCPServe.
func TestWireFormat_PluginTools_HaveDescriptionWithOutputs(t *testing.T) {
	mockProvider := mocks.NewMockOperationProvider()
	mockProvider.AddOperation(&pluginmodel.OperationSchema{
		Name:        "time",
		PluginName:  "awf-plugin-time",
		Description: "Returns current system date/time",
		Inputs:      map[string]pluginmodel.InputSchema{},
		Outputs:     []string{"output", "timestamp", "timezone", "unix"},
	})

	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{
			"awf-plugin-time": mockProvider,
		},
	}

	configPath := writeProxyConfig(t, mcpProxyConfig{
		InterceptBuiltins: false,
		PluginTools: []apptools.PluginToolSpec{
			{Plugin: "awf-plugin-time", Expose: []string{"time"}},
		},
	})

	// Bootstrap the MCP server via registerPluginTools (same path as runMCPServe)
	// but using an in-memory stdin/stdout pair rather than os.Stdin/os.Stdout.
	srv := mcpserver.New()
	opProvider, cleanup, err := resolveOperationProvider(context.Background(), deps)
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var cfg mcpProxyConfig
	require.NoError(t, json.Unmarshal(data, &cfg))

	require.NoError(t, registerPluginTools(context.Background(), srv, deps, opProvider, cfg.PluginTools))

	// Serve from an in-memory reader/writer.
	const input = `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, stdin, &stdout) }()
	<-done

	// Parse the tools/list response.
	scanner := bufio.NewScanner(&stdout)
	var toolsListResult map[string]any
	for scanner.Scan() {
		var resp map[string]any
		if jsonErr := json.Unmarshal(scanner.Bytes(), &resp); jsonErr != nil {
			continue
		}
		if _, hasResult := resp["result"]; hasResult {
			result, _ := resp["result"].(map[string]any)
			toolsListResult = result
			break
		}
	}

	require.NotNil(t, toolsListResult, "tools/list response must be present")
	rawTools, ok := toolsListResult["tools"].([]any)
	require.True(t, ok)
	require.Len(t, rawTools, 1, "expected exactly one plugin tool")

	tool := rawTools[0].(map[string]any)
	desc, _ := tool["description"].(string)
	assert.NotEmpty(t, desc, "plugin tool must have a description in wire response")
	assert.Contains(t, desc, "Returns current system date/time", "description must include plugin's own description")
	assert.Contains(t, desc, "output", "description must mention output fields")
	assert.Contains(t, desc, "timestamp", "description must mention output fields")
}
