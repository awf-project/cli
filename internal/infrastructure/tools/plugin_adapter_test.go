package tools_test

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/tools"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPluginToolAdapter_HappyPath(t *testing.T) {
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs: map[string]pluginmodel.InputSchema{
			"message": {Type: "string"},
		},
	})

	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})

	require.NoError(t, err)
	require.NotNil(t, adapter)
}

func TestNewPluginToolAdapter_UnknownOperation(t *testing.T) {
	provider := mocks.NewMockOperationProvider()

	_, err := tools.NewPluginToolAdapter("notify", provider, []string{"unknown_op"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrUnknownOperation))
	assert.Contains(t, err.Error(), "unknown_op")
}

func TestNewPluginToolAdapter_UnsupportedSchemaArray(t *testing.T) {
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "process",
		PluginName: "batch",
		Inputs: map[string]pluginmodel.InputSchema{
			"items": {Type: "array"},
		},
	})

	_, err := tools.NewPluginToolAdapter("batch", provider, []string{"process"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrUnsupportedSchema))
	assert.Contains(t, err.Error(), "items")
}

func TestNewPluginToolAdapter_UnsupportedSchemaObject(t *testing.T) {
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "configure",
		PluginName: "config",
		Inputs: map[string]pluginmodel.InputSchema{
			"settings": {Type: "object"},
		},
	})

	_, err := tools.NewPluginToolAdapter("config", provider, []string{"configure"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrUnsupportedSchema))
}

func TestPluginToolAdapter_ListTools_ReturnsNamespacedNames(t *testing.T) {
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs: map[string]pluginmodel.InputSchema{
			"message": {Type: "string"},
		},
	})

	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})
	require.NoError(t, err)

	defs, err := adapter.ListTools(context.Background())

	require.NoError(t, err)
	require.Len(t, defs, 1)
	assert.Equal(t, "notify_send", defs[0].Name)
	assert.Equal(t, "plugin:notify", defs[0].Source)
}

func TestPluginToolAdapter_ListTools_ReturnsInputSchema(t *testing.T) {
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs: map[string]pluginmodel.InputSchema{
			"message": {Type: "string", Required: true},
		},
	})

	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})
	require.NoError(t, err)

	defs, err := adapter.ListTools(context.Background())

	require.NoError(t, err)
	require.Len(t, defs, 1)
	assert.NotNil(t, defs[0].InputSchema)
	assert.Equal(t, "object", defs[0].InputSchema["type"])
}

func TestPluginToolAdapter_ListTools_MultipleOperations(t *testing.T) {
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs:     map[string]pluginmodel.InputSchema{},
	})
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "dismiss",
		PluginName: "notify",
		Inputs:     map[string]pluginmodel.InputSchema{},
	})

	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send", "dismiss"})
	require.NoError(t, err)

	defs, err := adapter.ListTools(context.Background())

	require.NoError(t, err)
	require.Len(t, defs, 2)
	names := []string{defs[0].Name, defs[1].Name}
	assert.Contains(t, names, "notify_send")
	assert.Contains(t, names, "notify_dismiss")
}

func TestPluginToolAdapter_CallTool_DispatchesToExecute(t *testing.T) {
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs: map[string]pluginmodel.InputSchema{
			"message": {Type: "string"},
		},
	})

	provider.SetExecuteFunc(func(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
		return &pluginmodel.OperationResult{
			Success: true,
			Outputs: map[string]any{"id": "123"},
		}, nil
	})

	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})
	require.NoError(t, err)

	result, err := adapter.CallTool(context.Background(), "notify_send", map[string]any{"message": "hello"})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.NotNil(t, result.Content)
}

func TestPluginToolAdapter_CallTool_ConvertsError(t *testing.T) {
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs:     map[string]pluginmodel.InputSchema{},
	})

	provider.SetExecuteError(errors.New("send failed"))

	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})
	require.NoError(t, err)

	_, err = adapter.CallTool(context.Background(), "notify_send", map[string]any{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "send failed")
}

func TestPluginToolAdapter_Close_ReturnsNil(t *testing.T) {
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs:     map[string]pluginmodel.InputSchema{},
	})

	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})
	require.NoError(t, err)

	err = adapter.Close(context.Background())

	assert.NoError(t, err)
}

func TestPluginToolAdapter_ImplementsToolProvider(t *testing.T) {
	var _ ports.ToolProvider = (*tools.PluginToolAdapter)(nil)
}

// TestPluginToolAdapter_ListTools_PropagatesDescription asserts that ListTools returns a
// Description field composed from the operation's Description and Outputs. This is the
// contract that enables Gemini to accept the tool instead of refusing it as opaque.
func TestPluginToolAdapter_ListTools_PropagatesDescription(t *testing.T) {
	tests := []struct {
		name         string
		schema       *pluginmodel.OperationSchema
		wantContains string
	}{
		{
			name: "description and outputs both present",
			schema: &pluginmodel.OperationSchema{
				Name:        "time",
				PluginName:  "awf-plugin-time",
				Description: "Returns current system date/time",
				Inputs:      map[string]pluginmodel.InputSchema{},
				Outputs:     []string{"output", "timestamp", "timezone", "unix"},
			},
			wantContains: "Returns current system date/time",
		},
		{
			name: "outputs appended to description",
			schema: &pluginmodel.OperationSchema{
				Name:        "time",
				PluginName:  "awf-plugin-time",
				Description: "Returns current system date/time",
				Inputs:      map[string]pluginmodel.InputSchema{},
				Outputs:     []string{"output", "timestamp", "timezone", "unix"},
			},
			wantContains: "output, timestamp, timezone, unix",
		},
		{
			name: "generic fallback when description empty",
			schema: &pluginmodel.OperationSchema{
				Name:       "send",
				PluginName: "notify",
				Inputs:     map[string]pluginmodel.InputSchema{},
				Outputs:    []string{},
			},
			wantContains: "Operation 'send' from plugin 'notify'",
		},
		{
			name: "outputs omitted when empty",
			schema: &pluginmodel.OperationSchema{
				Name:        "ping",
				PluginName:  "health",
				Description: "Check server health",
				Inputs:      map[string]pluginmodel.InputSchema{},
				Outputs:     []string{},
			},
			wantContains: "Check server health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := mocks.NewMockOperationProvider()
			provider.AddOperation(tt.schema)

			adapter, err := tools.NewPluginToolAdapter(tt.schema.PluginName, provider, []string{tt.schema.Name})
			require.NoError(t, err)

			defs, err := adapter.ListTools(context.Background())
			require.NoError(t, err)
			require.Len(t, defs, 1)
			assert.NotEmpty(t, defs[0].Description, "Description must not be empty")
			assert.Contains(t, defs[0].Description, tt.wantContains)
		})
	}
}

// TestPluginToolAdapter_ListTools_OutputsNotInDescriptionWhenEmpty asserts that when
// Outputs is empty, no "Returns a JSON object" sentence appears in the description.
func TestPluginToolAdapter_ListTools_OutputsNotInDescriptionWhenEmpty(t *testing.T) {
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:        "ping",
		PluginName:  "health",
		Description: "Check server health",
		Inputs:      map[string]pluginmodel.InputSchema{},
		Outputs:     []string{},
	})

	adapter, err := tools.NewPluginToolAdapter("health", provider, []string{"ping"})
	require.NoError(t, err)

	defs, err := adapter.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, defs, 1)
	assert.NotContains(t, defs[0].Description, "Returns a JSON object")
}

// TestPluginToolAdapter_CallTool_PrefixesOpNameWithPluginName verifies that CallTool
// passes the fully-qualified "pluginName.opName" to provider.Execute rather than
// the raw short name. Unprefixed names trigger a blind fallback across ALL connected
// plugins, which may return a false-success from a non-operation-provider plugin.
func TestPluginToolAdapter_CallTool_PrefixesOpNameWithPluginName(t *testing.T) {
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs:     map[string]pluginmodel.InputSchema{},
	})

	provider.SetExecuteFunc(func(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
		return &pluginmodel.OperationResult{Success: true}, nil
	})

	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})
	require.NoError(t, err)

	_, err = adapter.CallTool(context.Background(), "notify_send", map[string]any{})
	require.NoError(t, err)

	calls := provider.GetExecuteCalls()
	require.Len(t, calls, 1)
	// Must be the fully-qualified name so the provider routes to the correct plugin.
	assert.Equal(t, "notify.send", calls[0].Name)
}

// TestPluginToolAdapter_CallTool_RoutesByPrefixedName is a regression test for the
// production bug where two plugins both expose an operation with the same short name.
// Without the prefix, the provider's unprefixed fallback loop returns the first
// non-gRPC-error response — which may come from the wrong plugin (or from a plugin
// that returns Success=false because it does not implement operations at all).
// With the prefix, the provider routes directly to the intended plugin.
func TestPluginToolAdapter_CallTool_RoutesByPrefixedName(t *testing.T) {
	provider := mocks.NewMockOperationProvider()

	// "notify" plugin has "send" — this is the target plugin.
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs:     map[string]pluginmodel.InputSchema{},
	})
	// "logger" plugin also has "send" — wrong plugin; should never be called.
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "logger",
		Inputs:     map[string]pluginmodel.InputSchema{},
	})

	provider.SetExecuteFunc(func(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
		return &pluginmodel.OperationResult{Success: true, Outputs: map[string]any{"routed_to": name}}, nil
	})

	// Adapter is for "notify" — must route to "notify.send", not ambiguous "send".
	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})
	require.NoError(t, err)

	_, err = adapter.CallTool(context.Background(), "notify_send", map[string]any{})
	require.NoError(t, err)

	calls := provider.GetExecuteCalls()
	require.Len(t, calls, 1)
	// The provider must receive the fully-qualified name to enable direct routing.
	assert.Equal(t, "notify.send", calls[0].Name, "adapter must pass prefixed name to prevent cross-plugin routing ambiguity")
}
