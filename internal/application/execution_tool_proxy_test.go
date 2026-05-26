package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application/tools"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
)

// TestBuildToolProxyFactory_BuiltinsOnly verifies the factory returns a single provider
// containing the built-in toolset when InterceptBuiltins is true and PluginTools empty.
func TestBuildToolProxyFactory_BuiltinsOnly(t *testing.T) {
	factory := buildToolProxyFactory(mocks.NewMockCommandExecutor(), nil)

	providers, err := factory(tools.ProxyConfig{InterceptBuiltins: true})

	require.NoError(t, err)
	require.Len(t, providers, 1, "exactly one provider (the built-ins) is expected")
}

// TestBuildToolProxyFactory_PluginToolsRequireOperationProvider verifies that requesting
// plugin tools without an OperationProvider returns a structured error rather than a
// silent skip — the previous behavior that masked F099 plugin_tools entries.
func TestBuildToolProxyFactory_PluginToolsRequireOperationProvider(t *testing.T) {
	factory := buildToolProxyFactory(mocks.NewMockCommandExecutor(), nil)

	_, err := factory(tools.ProxyConfig{
		PluginTools: []tools.PluginToolSpec{{Plugin: "notify", Expose: []string{"send"}}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin_tools requested but no operation provider")
}

// TestBuildToolProxyFactory_PluginToolsBuildAdapter verifies that the factory constructs
// one PluginToolAdapter per spec, sourcing operation schemas from the shared
// OperationProvider.
func TestBuildToolProxyFactory_PluginToolsBuildAdapter(t *testing.T) {
	opProvider := mocks.NewMockOperationProvider()
	opProvider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs: map[string]pluginmodel.InputSchema{
			"message": {Type: pluginmodel.InputTypeString, Required: true},
		},
	})

	factory := buildToolProxyFactory(mocks.NewMockCommandExecutor(), opProvider)

	providers, err := factory(tools.ProxyConfig{
		InterceptBuiltins: true,
		PluginTools:       []tools.PluginToolSpec{{Plugin: "notify", Expose: []string{"send"}}},
	})

	require.NoError(t, err)
	require.Len(t, providers, 2, "expected one built-ins provider plus one plugin adapter")

	// The second provider is the plugin adapter; verify it lists the prefixed tool name.
	defs, listErr := providers[1].ListTools(context.Background())
	require.NoError(t, listErr)
	require.Len(t, defs, 1)
	assert.Equal(t, "notify_send", defs[0].Name)
}

// TestBuildToolProxyFactory_PluginToolsUnknownOperationFails verifies that referencing
// an operation the provider does not know returns an error (wrapped from
// PluginToolAdapter's ErrUnknownOperation).
func TestBuildToolProxyFactory_PluginToolsUnknownOperationFails(t *testing.T) {
	opProvider := mocks.NewMockOperationProvider()
	// no operations registered

	factory := buildToolProxyFactory(mocks.NewMockCommandExecutor(), opProvider)

	_, err := factory(tools.ProxyConfig{
		PluginTools: []tools.PluginToolSpec{{Plugin: "notify", Expose: []string{"send"}}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "notify")
}

// TestMcpProxyConfigToApp_PreservesPluginTools verifies the domain→application conversion
// keeps every PluginToolExpose entry intact and the toggle fields are mapped 1:1.
func TestMcpProxyConfigToApp_PreservesPluginTools(t *testing.T) {
	src := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools: []workflow.PluginToolExpose{
			{Plugin: "notify", Expose: []string{"send", "dismiss"}},
			{Plugin: "db", Expose: []string{"query"}},
		},
	}

	got := mcpProxyConfigToApp(src)

	assert.True(t, got.Enable)
	assert.True(t, got.InterceptBuiltins)
	require.Len(t, got.PluginTools, 2)
	assert.Equal(t, "notify", got.PluginTools[0].Plugin)
	assert.Equal(t, []string{"send", "dismiss"}, got.PluginTools[0].Expose)
	assert.Equal(t, "db", got.PluginTools[1].Plugin)
	assert.Equal(t, []string{"query"}, got.PluginTools[1].Expose)
}

// TestStartToolProxyImpl_NoopWhenProxyNil verifies the helper returns a no-op cleanup
// and never reads step.MCPProxy when the proxy service is not wired (typical for
// dry-run / interactive paths).
func TestStartToolProxyImpl_NoopWhenProxyNil(t *testing.T) {
	opts := map[string]any{}
	step := &workflow.Step{MCPProxy: &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}}

	cleanup, err := startToolProxyImpl(context.Background(), nil, mocks.NewMockLogger(), step, opts, "claude", nil)

	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.NoError(t, cleanup())
	assert.NotContains(t, opts, workflow.MCPProxyConfigKey, "no keys must be injected when proxy is nil")
}

// TestStartToolProxyImpl_NoopWhenDisabled verifies the helper returns a no-op cleanup
// when MCPProxy.Enable is false.
func TestStartToolProxyImpl_NoopWhenDisabled(t *testing.T) {
	proxy := tools.NewProxyService(
		mocks.NewMockCLIExecutor(),
		mocks.NewMockTracer(),
		mocks.NewMockLogger(),
		func(tools.ProxyConfig) ([]ports.ToolProvider, error) { return nil, nil },
	)
	opts := map[string]any{}
	step := &workflow.Step{MCPProxy: &workflow.MCPProxyConfig{Enable: false, InterceptBuiltins: true}}

	cleanup, err := startToolProxyImpl(context.Background(), proxy, mocks.NewMockLogger(), step, opts, "claude", nil)

	require.NoError(t, err)
	assert.NoError(t, cleanup())
	assert.NotContains(t, opts, workflow.MCPProxyConfigKey)
}

// TestStartToolProxyImpl_OpenAICompatibleUsesHTTPPath verifies that the helper routes
// the openai_compatible provider through the in-process HTTP router path (T012 complete)
// rather than the stdio subprocess path, and that MCPProxyConfigKey is injected into opts.
func TestStartToolProxyImpl_OpenAICompatibleUsesHTTPPath(t *testing.T) {
	proxy := tools.NewProxyService(
		mocks.NewMockCLIExecutor(),
		mocks.NewMockTracer(),
		mocks.NewMockLogger(),
		func(tools.ProxyConfig) ([]ports.ToolProvider, error) { return nil, nil },
	)
	opts := map[string]any{}
	step := &workflow.Step{MCPProxy: &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}}

	cleanup, err := startToolProxyImpl(context.Background(), proxy, mocks.NewMockLogger(), step, opts, "openai_compatible", nil)

	require.NoError(t, err)
	assert.NoError(t, cleanup())
	// T012 complete: openai_compatible uses the HTTP router path; MCPProxyConfigKey is set.
	assert.Contains(t, opts, workflow.MCPProxyConfigKey, "openai_compatible must set MCPProxyConfigKey via HTTP path")
	// stdio config path must NOT be set (HTTP path does not write a tmp file)
	assert.NotContains(t, opts, workflow.MCPProxyConfigPathKey, "HTTP path must not set MCPProxyConfigPathKey")
}
