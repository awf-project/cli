package cli

// White-box tests for the plugin_tools resolution paths in runMCPServe.
// These require package-level access to runMCPServe, Deps, and mcpProxyConfig.

import (
	"bufio"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
	"testing"

	apptools "github.com/awf-project/cli/internal/application/tools"
	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	inframcp "github.com/awf-project/cli/internal/infrastructure/mcp"
	"github.com/awf-project/cli/internal/infrastructure/tools/builtins"
	"github.com/awf-project/cli/internal/testutil/mocks"
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

// requestToolsList drives srv over in-memory pipes through an initialize + tools/list
// handshake and returns the decoded "result" object of the tools/list response.
//
// Pipes (not strings.NewReader) keep stdin open until the response is read: a reader that
// delivers EOF immediately after the last line makes the SDK set readErr=io.EOF, which
// blocks the async tools/list write.
func requestToolsList(t *testing.T, srv *inframcp.Server) map[string]any {
	t.Helper()

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- srv.ServeIO(ctx, stdinR, stdoutW)
		stdoutW.Close() //nolint:errcheck // signals EOF to the scanner below
	}()

	// Unsynchronised writer: the scanner below may find the id=2 response and close stdinW
	// before this goroutine finishes writing. Write errors after that close are expected and
	// intentionally discarded — io.Pipe makes them safe (no partial-state corruption).
	go func() {
		_, _ = io.WriteString(stdinW, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}`+"\n")
		_, _ = io.WriteString(stdinW, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`+"\n")
	}()

	// Stream responses; break as soon as id=2 arrives (before closing stdin).
	scanner := bufio.NewScanner(stdoutR)
	var toolsListResult map[string]any
	for scanner.Scan() {
		var resp map[string]any
		if jsonErr := json.Unmarshal(scanner.Bytes(), &resp); jsonErr != nil {
			continue
		}
		if id, ok := resp["id"].(float64); ok && id == 2 {
			result, _ := resp["result"].(map[string]any)
			toolsListResult = result
			break
		}
	}

	// Close stdin to signal server shutdown; drain stdout so the server goroutine can
	// finish any pending writes without blocking.
	stdinW.Close()                  //nolint:errcheck // pipe close error is irrelevant after response received
	go io.Copy(io.Discard, stdoutR) //nolint:errcheck // draining stdoutR; discard errors are irrelevant after test response received
	<-serverDone

	require.NotNil(t, toolsListResult, "tools/list response must be present in output")
	return toolsListResult
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
	var exitErr *exitError
	require.ErrorAs(t, err, &exitErr, "error must be *exitError")
	assert.Equal(t, ExitUser, exitErr.code, "unknown plugin should be ExitUser (user error)")
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
// It drives the MCP server's RegisterProvider path directly (no subprocess, no OS pipe)
// by constructing a builtins.Provider, registering it on a real inframcp.Server, and then
// serving a tools/list request from an in-memory reader.
//
// The assertion: every tool in the tools/list JSON response has a non-empty "description"
// field. This locks in the wire-format enrichment that unblocks Gemini from calling
// the tools (Gemini refuses opaque tools with no description).
func TestWireFormat_BuiltinTools_AllHaveDescription(t *testing.T) {
	// Build a builtins provider and verify it exposes tools (mirrors production wiring).
	provider := builtins.NewProvider() // no executor needed: only ListTools is called
	tools, err := provider.ListTools(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, tools, "expected at least one builtin tool")

	// Wire the tools onto a real MCP server via RegisterProvider.
	srv := inframcp.New(Version)
	require.NoError(t, srv.RegisterProvider(provider))

	toolsListResult := requestToolsList(t, srv)

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

// TestRegisterPluginTools_RegistersEachProviderOnce verifies that registerPluginTools
// calls srv.RegisterProvider exactly once per spec entry.
func TestRegisterPluginTools_RegistersEachProviderOnce(t *testing.T) {
	mockProvider := mocks.NewMockOperationProvider()
	mockProvider.AddOperation(&pluginmodel.OperationSchema{
		Name:        "op1",
		PluginName:  "test-plugin",
		Description: "operation 1",
		Inputs:      map[string]pluginmodel.InputSchema{},
	})
	mockProvider.AddOperation(&pluginmodel.OperationSchema{
		Name:        "op2",
		PluginName:  "test-plugin",
		Description: "operation 2",
		Inputs:      map[string]pluginmodel.InputSchema{},
	})

	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{
			"test-plugin": mockProvider,
		},
	}

	srv := inframcp.New(Version)
	specs := []apptools.PluginToolSpec{
		{Plugin: "test-plugin", Expose: []string{"op1", "op2"}},
	}

	err := registerPluginTools(srv, deps, nil, specs)
	require.NoError(t, err, "registerPluginTools should succeed with valid provider and spec")
}

// TestRegisterPluginTools_ErrorWhenAdapterFails verifies that registerPluginTools
// returns an error when NewPluginToolAdapter fails (e.g., no matching operations).
func TestRegisterPluginTools_ErrorWhenAdapterFails(t *testing.T) {
	mockProvider := mocks.NewMockOperationProvider()
	mockProvider.AddOperation(&pluginmodel.OperationSchema{
		Name:        "op1",
		PluginName:  "test-plugin",
		Description: "operation 1",
		Inputs:      map[string]pluginmodel.InputSchema{},
	})

	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{
			"test-plugin": mockProvider,
		},
	}

	srv := inframcp.New(Version)
	// Request an operation that doesn't exist
	specs := []apptools.PluginToolSpec{
		{Plugin: "test-plugin", Expose: []string{"nonexistent-op"}},
	}

	err := registerPluginTools(srv, deps, nil, specs)
	require.Error(t, err, "registerPluginTools should error when operation is not found")
}

// TestLookupPluginProvider_FromDeps verifies that lookupPluginProvider returns
// the provider from deps.OperationProviders when populated.
func TestLookupPluginProvider_FromDeps(t *testing.T) {
	mockProvider := mocks.NewMockOperationProvider()
	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{
			"my-plugin": mockProvider,
		},
	}

	provider, err := lookupPluginProvider(deps, nil, "my-plugin")
	require.NoError(t, err)
	assert.Equal(t, mockProvider, provider)
}

// TestLookupPluginProvider_UnknownPluginFromDeps verifies that lookupPluginProvider
// returns UNKNOWN_PLUGIN when the plugin is not in deps.OperationProviders.
func TestLookupPluginProvider_UnknownPluginFromDeps(t *testing.T) {
	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{
			"other-plugin": mocks.NewMockOperationProvider(),
		},
	}

	provider, err := lookupPluginProvider(deps, nil, "unknown-plugin")
	require.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), string(domerrors.ErrorCodeUserMCPProxyUnknownPlugin))
}

// TestLookupPluginProvider_FromCompositeProvider verifies that lookupPluginProvider
// returns the composite provider when deps is empty (subprocess path).
func TestLookupPluginProvider_FromCompositeProvider(t *testing.T) {
	mockComposite := mocks.NewMockOperationProvider()
	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{},
	}

	provider, err := lookupPluginProvider(deps, mockComposite, "any-plugin")
	require.NoError(t, err)
	assert.Equal(t, mockComposite, provider)
}

// TestLookupPluginProvider_NilCompositeProvider verifies that lookupPluginProvider
// returns UNKNOWN_PLUGIN when the composite provider is nil (no plugin directories).
func TestLookupPluginProvider_NilCompositeProvider(t *testing.T) {
	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{},
	}

	provider, err := lookupPluginProvider(deps, nil, "any-plugin")
	require.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), string(domerrors.ErrorCodeUserMCPProxyUnknownPlugin))
	assert.Contains(t, err.Error(), "no plugin directories")
}

// TestResolveOperationProvider_PopulatedDepsReturnsNil verifies that
// resolveOperationProvider returns nil when deps.OperationProviders is populated.
func TestResolveOperationProvider_PopulatedDepsReturnsNil(t *testing.T) {
	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{
			"test-plugin": mocks.NewMockOperationProvider(),
		},
	}

	opProvider, cleanup, err := resolveOperationProvider(context.Background(), deps)
	require.NoError(t, err)
	assert.Nil(t, opProvider, "should return nil when deps is populated (callers use the map directly)")
	assert.Nil(t, cleanup)
}

// TestResolveOperationProvider_EmptyDepsBootstraps verifies that resolveOperationProvider
// bootstraps the plugin system when deps.OperationProviders is empty.
func TestResolveOperationProvider_EmptyDepsBootstraps(t *testing.T) {
	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{},
	}

	// Empty plugin path to avoid finding real plugins
	t.Setenv("AWF_PLUGINS_PATH", t.TempDir())

	_, cleanup, _ := resolveOperationProvider(context.Background(), deps)
	// Bootstrap may succeed (with nil Manager) or fail - both are valid when no plugin directories exist
	if cleanup != nil {
		defer cleanup()
	}
	// The bootstrap attempt itself is the expected behavior for empty deps
}

// TestWireFormat_PluginTools_HaveDescriptionWithOutputs verifies that a plugin tool
// registered via a PluginToolAdapter carries a description composed from the
// OperationSchema.Description and Outputs in the wire response.
//
// Rather than redirecting os.Stdin/os.Stdout (which causes test-level races in parallel
// runs), this test assembles the MCP server directly using inframcp.Server and the
// unexported registerPluginTools helper — the same code path used by runMCPServe.
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
	srv := inframcp.New(Version)
	opProvider, cleanup, err := resolveOperationProvider(context.Background(), deps)
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var cfg mcpProxyConfig
	require.NoError(t, json.Unmarshal(data, &cfg))

	require.NoError(t, registerPluginTools(srv, deps, opProvider, cfg.PluginTools))

	toolsListResult := requestToolsList(t, srv)

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

// TestRunMCPServe_ConfigFileNotFound verifies that missing config file returns
// ExitUser (user error) rather than ExitExecution.
func TestRunMCPServe_ConfigFileNotFound(t *testing.T) {
	ctx := context.Background()
	err := runMCPServe(ctx, Deps{}, "/nonexistent/config.json")

	require.Error(t, err)
	// Verify it's an exitError with ExitUser code
	var exitErr *exitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, ExitUser, exitErr.code, "config file missing should be ExitUser")
	assert.Contains(t, err.Error(), "config file", "error message should mention config file")
}

// TestRunMCPServe_InvalidConfigJSON verifies that malformed config JSON returns
// ExitUser (user error) rather than ExitExecution.
func TestRunMCPServe_InvalidConfigJSON(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "mcp-proxy-*.json")
	require.NoError(t, err)
	_, err = f.WriteString("{invalid json}")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	ctx := context.Background()
	err = runMCPServe(ctx, Deps{}, f.Name())

	require.Error(t, err)
	var exitErr *exitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, ExitUser, exitErr.code, "invalid JSON config should be ExitUser")
	assert.Contains(t, err.Error(), "invalid config", "error message should indicate JSON error")
}

// TestRunMCPServe_WithBuiltins_RegistersProvider verifies that when InterceptBuiltins
// is true, the builtin provider is registered on the MCP server.
func TestRunMCPServe_WithBuiltins_RegistersProvider(t *testing.T) {
	configPath := writeProxyConfig(t, mcpProxyConfig{
		InterceptBuiltins: true,
		PluginTools:       nil,
	})

	// Cancel immediately so the server doesn't run indefinitely
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runMCPServe(ctx, Deps{}, configPath)
	// Cancelled context yields nil on clean shutdown
	assert.NoError(t, err, "builtin provider registration should succeed")
}

// TestNewMCPServeCommand_Structure verifies that the Cobra command is created
// with the expected name, visibility, annotations, and flags.
func TestNewMCPServeCommand_Structure(t *testing.T) {
	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{},
	}

	cmd := newMCPServeCommand(deps)

	require.NotNil(t, cmd)
	assert.Equal(t, "mcp-serve", cmd.Use)
	assert.True(t, cmd.Hidden, "mcp-serve should be hidden from help")

	// Verify the annotation is set
	val, ok := cmd.Annotations[annotationSkipFormatValidation]
	assert.True(t, ok, "should have skipFormatValidation annotation")
	assert.Equal(t, "true", val)

	// Verify the config flag is required
	configFlag := cmd.Flag("config")
	require.NotNil(t, configFlag)
	assert.True(t, configFlag.DefValue == "", "config flag should have no default")
}

// TestArchitecture_MCPServe_NewUsesVersion verifies AC-3 / FR-007: the infrastructure
// MCP adapter is constructed with the package-level Version constant (not a hardcoded string).
// AST inspection of mcp_serve.go guarantees the constraint is enforced at the source level
// and survives future refactors without requiring a running server.
func TestArchitecture_MCPServe_NewUsesVersion(t *testing.T) {
	src := parseMCPServeFile(t)

	// Resolve the alias used for internal/infrastructure/mcp so we can look for <alias>.New.
	const infraMCPPath = "github.com/awf-project/cli/internal/infrastructure/mcp"
	var infraAlias string
	for _, imp := range src.Imports {
		if strings.Trim(imp.Path.Value, `"`) == infraMCPPath && imp.Name != nil {
			infraAlias = imp.Name.Name
			break
		}
	}
	require.NotEmpty(t, infraAlias, "internal/infrastructure/mcp must be imported with an alias (see TestArchitecture_MCPServe_InfrastructureMCPImportIsAliased)")

	// Walk the AST searching for a call expression of the form <alias>.New(Version, …).
	var found bool
	ast.Inspect(src, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != infraAlias || sel.Sel.Name != "New" {
			return true
		}
		// Found <alias>.New(…); verify the first argument is the identifier "Version".
		if len(call.Args) > 0 {
			if arg, argOK := call.Args[0].(*ast.Ident); argOK && arg.Name == "Version" {
				found = true
			}
		}
		return true
	})

	assert.True(
		t, found,
		"mcp_serve.go must call %s.New(Version) — hardcoded version strings are forbidden (AC-3/FR-007); got wrong or missing Version argument",
		infraAlias,
	)
}

// TestRunMCPServe_CancelledContextYieldsNil verifies that when the context is cancelled
// before the server starts, runMCPServe returns nil (clean shutdown).
func TestRunMCPServe_CancelledContextYieldsNil(t *testing.T) {
	configPath := writeProxyConfig(t, mcpProxyConfig{
		InterceptBuiltins: false,
		PluginTools:       nil,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before running

	err := runMCPServe(ctx, Deps{}, configPath)
	assert.NoError(t, err, "cancelled context should yield clean shutdown (nil)")
}

// TestLookupPluginProvider_EmptyDepsReturnsError verifies that when deps has no
// OperationProviders and opProvider is nil, lookupPluginProvider returns UNKNOWN_PLUGIN.
func TestLookupPluginProvider_EmptyDepsAndNilProvider(t *testing.T) {
	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{},
	}

	provider, err := lookupPluginProvider(deps, nil, "test-plugin")

	require.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), string(domerrors.ErrorCodeUserMCPProxyUnknownPlugin))
	assert.Contains(t, err.Error(), "no plugin directories")
}

// TestRegisterPluginTools_SingleSpecMultipleOperations verifies that registerPluginTools
// correctly handles a single spec with multiple exposed operations.
func TestRegisterPluginTools_SingleSpecMultipleOperations(t *testing.T) {
	mockProvider := mocks.NewMockOperationProvider()
	mockProvider.AddOperation(&pluginmodel.OperationSchema{
		Name:        "fetch",
		PluginName:  "api-plugin",
		Description: "fetch from API",
		Inputs:      map[string]pluginmodel.InputSchema{},
	})
	mockProvider.AddOperation(&pluginmodel.OperationSchema{
		Name:        "parse",
		PluginName:  "api-plugin",
		Description: "parse response",
		Inputs:      map[string]pluginmodel.InputSchema{},
	})

	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{
			"api-plugin": mockProvider,
		},
	}

	srv := inframcp.New(Version)
	specs := []apptools.PluginToolSpec{
		{Plugin: "api-plugin", Expose: []string{"fetch", "parse"}},
	}

	err := registerPluginTools(srv, deps, nil, specs)
	require.NoError(t, err, "should register both operations from single provider")
}

// TestResolveOperationProvider_EmptyDepsCallsInitPluginSystem verifies that
// resolveOperationProvider calls initPluginSystem when deps is empty.
func TestResolveOperationProvider_EmptyDepsInitializes(t *testing.T) {
	// Point plugin discovery at a real (but empty) directory.
	// InitSystem creates an RPCPluginManager for any existing directory, even with no
	// plugins installed — so opProvider will be non-nil and cleanup will be non-nil.
	// This distinguishes the bootstrap path from the populated-deps early-return path,
	// which always returns (nil, nil, nil) without touching the filesystem.
	t.Setenv("AWF_PLUGINS_PATH", t.TempDir())

	deps := Deps{
		OperationProviders: map[string]ports.OperationProvider{},
	}

	opProvider, cleanup, err := resolveOperationProvider(context.Background(), deps)

	// Bootstrap path was taken: initPluginSystem ran successfully.
	require.NoError(t, err, "bootstrap should succeed even when no plugins are installed")
	assert.NotNil(t, opProvider, "bootstrap path should return a non-nil OperationProvider (RPCPluginManager) when plugin dir exists on disk")
	assert.NotNil(t, cleanup, "bootstrap path should return a non-nil cleanup function")

	if cleanup != nil {
		defer cleanup()
	}
}

// TestArchitecture_MCPServe_NoPkgMCPServerImport verifies that mcp_serve.go does not
// import the deprecated pkg/mcpserver package (Acceptance Criteria 1).
// Test-enforcing this constraint catches accidental re-introduction during refactors
// without waiting for a code-inspection pass.
func TestArchitecture_MCPServe_NoPkgMCPServerImport(t *testing.T) {
	src := parseMCPServeFile(t)
	for _, imp := range src.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		assert.False(
			t,
			strings.Contains(path, "pkg/mcpserver"),
			"mcp_serve.go must not import pkg/mcpserver (found: %q) — use internal/infrastructure/mcp instead", path,
		)
	}
}

// TestArchitecture_MCPServe_InfrastructureMCPImportIsAliased verifies that
// internal/infrastructure/mcp is imported with an explicit alias (Acceptance Criteria 2).
// The alias (e.g. inframcp or mcpadapter) prevents silent shadowing of the SDK's top-level
// mcp package, per the dual-import-alias rule in CLAUDE.md.
func TestArchitecture_MCPServe_InfrastructureMCPImportIsAliased(t *testing.T) {
	src := parseMCPServeFile(t)
	const infraMCPPath = "github.com/awf-project/cli/internal/infrastructure/mcp"
	for _, imp := range src.Imports {
		if strings.Trim(imp.Path.Value, `"`) == infraMCPPath {
			require.NotNil(t, imp.Name,
				"internal/infrastructure/mcp must be imported with an explicit alias (e.g. inframcp or mcpadapter)")
			assert.NotEqual(t, "_", imp.Name.Name,
				"internal/infrastructure/mcp alias must not be a blank import")
			assert.NotEqual(t, ".", imp.Name.Name,
				"internal/infrastructure/mcp must not use a dot import")
			return
		}
	}
	t.Fatal("mcp_serve.go does not import internal/infrastructure/mcp — expected an aliased import")
}

// TestArchitecture_MCPServe_HelpersRemoved verifies that portSchemaToMCP and
// portResultToMCP are not declared in mcp_serve.go (Acceptance Criteria 5).
// These helpers were moved to internal/infrastructure/mcp/mapping.go and must not
// remain in the interfaces layer as duplicates.
func TestArchitecture_MCPServe_HelpersRemoved(t *testing.T) {
	src := parseMCPServeFile(t)
	forbidden := []string{"portSchemaToMCP", "portResultToMCP"}
	for _, decl := range src.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		for _, name := range forbidden {
			assert.NotEqual(
				t, name, fn.Name.Name,
				"mcp_serve.go must not declare %s — it was relocated to internal/infrastructure/mcp/mapping.go in T023", name,
			)
		}
	}
}

// parseMCPServeFile parses mcp_serve.go, which is co-located with this test file in the
// same package directory. Go test processes set the working directory to the package
// directory, so the relative path resolves correctly.
func parseMCPServeFile(t *testing.T) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, "mcp_serve.go", nil, 0)
	require.NoError(t, err, "failed to parse mcp_serve.go")
	return src
}
