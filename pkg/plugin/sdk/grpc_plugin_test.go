package sdk

import (
	"context"
	"errors"
	"testing"

	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// brokerAwareTestPlugin implements both Plugin and BrokerAwarePlugin.
type brokerAwareTestPlugin struct {
	BasePlugin
	setHostClientCalled bool
	receivedClient      *HostClient
}

func (p *brokerAwareTestPlugin) SetHostClient(client *HostClient) {
	p.setHostClientCalled = true
	p.receivedClient = client
}

// errorPlugin is a test plugin that returns errors on Init/Shutdown.
type errorPlugin struct {
	BasePlugin
	initErr     error
	shutdownErr error
}

func (p *errorPlugin) Init(_ context.Context, _ map[string]any) error {
	return p.initErr
}

func (p *errorPlugin) Shutdown(_ context.Context) error {
	return p.shutdownErr
}

// TestGRPCServer_WithValidServer verifies GRPCServer correctly registers services on a properly initialized gRPC server.
func TestGRPCServer_WithValidServer(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "test-plugin",
			PluginVersion: "1.0.0",
		},
	}

	bridge := &GRPCPluginBridge{
		impl: plugin,
	}

	server := grpc.NewServer()
	defer server.Stop()

	err := bridge.GRPCServer(nil, server)
	require.NoError(t, err, "GRPCServer must register services without error")

	info := server.GetServiceInfo()
	assert.NotEmpty(t, info, "GRPCServer must register at least one service")
}

// TestGRPCClient_ReturnsErrorOnPluginSide verifies GRPCClient returns an error
// because it is a host-only method and must not be called on the plugin side.
func TestGRPCClient_ReturnsErrorOnPluginSide(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "test-plugin",
			PluginVersion: "1.0.0",
		},
	}

	bridge := &GRPCPluginBridge{
		impl: plugin,
	}

	result, err := bridge.GRPCClient(context.Background(), nil, &grpc.ClientConn{})
	assert.Error(t, err, "GRPCClient must return an error on the plugin side")
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "host-only method")
}

// TestPluginServiceServer_GetInfo verifies pluginServiceServer.GetInfo returns plugin metadata.
func TestPluginServiceServer_GetInfo(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "echo-plugin",
			PluginVersion: "2.1.0",
		},
	}

	server := &pluginServiceServer{impl: plugin}
	resp, err := server.GetInfo(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, "echo-plugin", resp.Name)
	assert.Equal(t, "2.1.0", resp.Version)
}

// TestPluginServiceServer_Init verifies pluginServiceServer.Init calls the plugin's Init method.
func TestPluginServiceServer_Init(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "test-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &pluginServiceServer{impl: plugin}

	// Create a config with JSON-encoded values
	config := make(map[string][]byte)
	config["key"] = []byte(`"value"`) // JSON string

	resp, err := server.Init(context.Background(), &pluginv1.InitRequest{Config: config})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestPluginServiceServer_Shutdown verifies pluginServiceServer.Shutdown calls the plugin's Shutdown method.
func TestPluginServiceServer_Shutdown(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "test-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &pluginServiceServer{impl: plugin}
	resp, err := server.Shutdown(context.Background(), &pluginv1.ShutdownRequest{})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestPluginServiceServer_Init_JSONConfig verifies Init correctly decodes JSON-encoded config values.
func TestPluginServiceServer_Init_JSONConfig(t *testing.T) {
	tests := []struct {
		name        string
		configValue []byte
		expected    any
	}{
		{
			name:        "JSON string",
			configValue: []byte(`"hello"`),
			expected:    "hello",
		},
		{
			name:        "JSON number",
			configValue: []byte(`42`),
			expected:    float64(42),
		},
		{
			name:        "JSON object",
			configValue: []byte(`{"key":"value"}`),
			expected:    map[string]any{"key": "value"},
		},
		{
			name:        "JSON array",
			configValue: []byte(`[1,2,3]`),
			expected:    []any{float64(1), float64(2), float64(3)},
		},
		{
			name:        "invalid JSON falls back to string",
			configValue: []byte(`{invalid}`),
			expected:    "{invalid}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &testPlugin{
				BasePlugin{
					PluginName:    "test-plugin",
					PluginVersion: "1.0.0",
				},
			}

			server := &pluginServiceServer{impl: plugin}
			config := map[string][]byte{"test_key": tt.configValue}

			resp, err := server.Init(context.Background(), &pluginv1.InitRequest{Config: config})

			require.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}

// TestOperationServiceServer_ListOperations verifies ListOperations returns empty list.
func TestOperationServiceServer_ListOperations(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "test-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &operationServiceServer{impl: plugin}
	resp, err := server.ListOperations(context.Background(), &pluginv1.ListOperationsRequest{})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Operations)
}

// TestOperationServiceServer_GetOperation verifies GetOperation returns operation with requested name.
func TestOperationServiceServer_GetOperation(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "test-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &operationServiceServer{impl: plugin}
	resp, err := server.GetOperation(context.Background(), &pluginv1.GetOperationRequest{Name: "test-op"})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Operation)
	assert.Equal(t, "test-op", resp.Operation.Name)
}

// TestOperationServiceServer_Execute verifies Execute returns not implemented error.
func TestOperationServiceServer_Execute(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "test-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &operationServiceServer{impl: plugin}
	resp, err := server.Execute(context.Background(), &pluginv1.ExecuteRequest{})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "plugin does not implement operations", resp.Error)
}

// TestGRPCServer_RegistersPluginServiceAndOperationService verifies both services are registered.
func TestGRPCServer_RegistersPluginServiceAndOperationService(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "test-plugin",
			PluginVersion: "1.0.0",
		},
	}

	bridge := &GRPCPluginBridge{impl: plugin}

	server := grpc.NewServer()
	defer server.Stop()

	err := bridge.GRPCServer(nil, server)
	require.NoError(t, err)

	info := server.GetServiceInfo()
	assert.GreaterOrEqual(t, len(info), 2, "must register at least PluginService and OperationService")
}

// TestGRPCClient_IsHostOnlyMethod verifies GRPCClient is a stub that errors on the plugin side,
// enforcing that only the host uses this method.
func TestGRPCClient_IsHostOnlyMethod(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "test-plugin",
			PluginVersion: "1.0.0",
		},
	}

	bridge := &GRPCPluginBridge{impl: plugin}

	result, err := bridge.GRPCClient(context.Background(), nil, &grpc.ClientConn{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "GRPCClient called on plugin side")
}

// TestPluginServiceServer_Init_PluginInitError verifies Init returns error when plugin.Init fails.
func TestPluginServiceServer_Init_PluginInitError(t *testing.T) {
	testErr := errors.New("plugin initialization failed")
	plugin := &errorPlugin{
		BasePlugin: BasePlugin{
			PluginName:    "error-plugin",
			PluginVersion: "1.0.0",
		},
		initErr: testErr,
	}

	server := &pluginServiceServer{impl: plugin}
	resp, err := server.Init(context.Background(), &pluginv1.InitRequest{Config: map[string][]byte{}})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "plugin init failed")
	assert.ErrorIs(t, err, testErr)
}

// TestPluginServiceServer_Shutdown_PluginShutdownError verifies Shutdown returns error when plugin.Shutdown fails.
func TestPluginServiceServer_Shutdown_PluginShutdownError(t *testing.T) {
	testErr := errors.New("plugin shutdown failed")
	plugin := &errorPlugin{
		BasePlugin: BasePlugin{
			PluginName:    "error-plugin",
			PluginVersion: "1.0.0",
		},
		shutdownErr: testErr,
	}

	server := &pluginServiceServer{impl: plugin}
	resp, err := server.Shutdown(context.Background(), &pluginv1.ShutdownRequest{})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "plugin shutdown failed")
	assert.ErrorIs(t, err, testErr)
}

// TestPluginServiceServer_GetInfo_ReturnsEmptyFields verifies GetInfo provides empty description and capabilities.
func TestPluginServiceServer_GetInfo_ReturnsEmptyFields(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "minimal-plugin",
			PluginVersion: "0.1.0",
		},
	}

	server := &pluginServiceServer{impl: plugin}
	resp, err := server.GetInfo(context.Background(), &pluginv1.GetInfoRequest{})

	require.NoError(t, err)
	assert.Equal(t, "minimal-plugin", resp.Name)
	assert.Equal(t, "0.1.0", resp.Version)
	assert.Empty(t, resp.Description)
	assert.Empty(t, resp.Capabilities)
}

func TestGRPCServer_CallsSetHostClientWhenBrokerAwarePlugin(t *testing.T) {
	plugin := &brokerAwareTestPlugin{
		BasePlugin: BasePlugin{PluginName: "aware", PluginVersion: "1.0.0"},
	}
	bridge := &GRPCPluginBridge{impl: plugin}
	server := grpc.NewServer()
	defer server.Stop()

	err := bridge.GRPCServer(nil, server)

	require.NoError(t, err)
	assert.True(t, plugin.setHostClientCalled, "SetHostClient must be called when plugin implements BrokerAwarePlugin")
}

func TestGRPCServer_SkipsSetHostClientWhenNotBrokerAwarePlugin(t *testing.T) {
	plugin := &testPlugin{BasePlugin: BasePlugin{PluginName: "plain", PluginVersion: "1.0.0"}}
	bridge := &GRPCPluginBridge{impl: plugin}
	server := grpc.NewServer()
	defer server.Stop()

	err := bridge.GRPCServer(nil, server)

	require.NoError(t, err)
}

// --- OperationSchemaProvider tests ---

// richSchemaPlugin implements both OperationProvider and OperationSchemaProvider.
// It is used to verify that the gRPC bridge propagates full metadata when both
// interfaces are present.
type richSchemaPlugin struct {
	BasePlugin
}

func (p *richSchemaPlugin) Operations() []string { return []string{"greet"} }

func (p *richSchemaPlugin) HandleOperation(_ context.Context, _ string, _ map[string]any) (*OperationResult, error) {
	return NewSuccessResult("hello", nil), nil
}

func (p *richSchemaPlugin) GetOperationSchema(name string) (OperationMeta, bool) {
	if name != "greet" {
		return OperationMeta{}, false
	}
	return OperationMeta{
		Description: "Greet a person.",
		Inputs: []InputMeta{
			{Name: "name", Type: InputTypeString, Required: true, Description: "Person's name."},
			{Name: "formal", Type: InputTypeBoolean, Description: "Use formal greeting."},
		},
		Outputs: []OutputMeta{
			{Name: "message", Type: InputTypeString, Description: "The greeting message."},
		},
	}, true
}

// TestListOperations_WithSchemaProvider_EmitsFullMetadata asserts that ListOperations
// propagates Description, Inputs, and Outputs when the plugin implements OperationSchemaProvider.
func TestListOperations_WithSchemaProvider_EmitsFullMetadata(t *testing.T) {
	plugin := &richSchemaPlugin{BasePlugin: BasePlugin{PluginName: "rich", PluginVersion: "1.0.0"}}
	server := &operationServiceServer{impl: plugin}

	resp, err := server.ListOperations(context.Background(), &pluginv1.ListOperationsRequest{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Operations, 1)

	op := resp.Operations[0]
	assert.Equal(t, "greet", op.Name)
	assert.Equal(t, "Greet a person.", op.Description)
	require.Len(t, op.Inputs, 2)
	assert.Equal(t, "name", op.Inputs[0].Name)
	assert.Equal(t, "string", op.Inputs[0].Type)
	assert.True(t, op.Inputs[0].Required)
	assert.Equal(t, "Person's name.", op.Inputs[0].Description)
	assert.Equal(t, "formal", op.Inputs[1].Name)
	assert.Equal(t, "boolean", op.Inputs[1].Type)
	assert.False(t, op.Inputs[1].Required)
	require.Len(t, op.Outputs, 1)
	assert.Equal(t, "message", op.Outputs[0].Name)
	assert.Equal(t, "string", op.Outputs[0].Type)
	assert.Equal(t, "The greeting message.", op.Outputs[0].Description)
}

// TestGetOperation_WithSchemaProvider_EmitsFullMetadata asserts that GetOperation
// propagates Description, Inputs, and Outputs when the plugin implements OperationSchemaProvider.
func TestGetOperation_WithSchemaProvider_EmitsFullMetadata(t *testing.T) {
	plugin := &richSchemaPlugin{BasePlugin: BasePlugin{PluginName: "rich", PluginVersion: "1.0.0"}}
	server := &operationServiceServer{impl: plugin}

	resp, err := server.GetOperation(context.Background(), &pluginv1.GetOperationRequest{Name: "greet"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Operation)
	assert.Equal(t, "greet", resp.Operation.Name)
	assert.Equal(t, "Greet a person.", resp.Operation.Description)
	require.Len(t, resp.Operation.Inputs, 2)
	require.Len(t, resp.Operation.Outputs, 1)
	assert.Equal(t, "message", resp.Operation.Outputs[0].Name)
}

// TestListOperations_WithoutSchemaProvider_RemainsNameOnly asserts that a plugin
// implementing only OperationProvider (no OperationSchemaProvider) produces
// name-only schemas — backwards compatibility is preserved.
func TestListOperations_WithoutSchemaProvider_RemainsNameOnly(t *testing.T) {
	srv := &operationServiceServer{impl: &legacyNoSchemaPlugin{
		BasePlugin: BasePlugin{PluginName: "legacy", PluginVersion: "1.0.0"},
	}}

	resp, err := srv.ListOperations(context.Background(), &pluginv1.ListOperationsRequest{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Operations, 1)

	op := resp.Operations[0]
	assert.Equal(t, "myop", op.Name)
	assert.Empty(t, op.Description, "description must be empty when no OperationSchemaProvider")
	assert.Empty(t, op.Inputs, "inputs must be empty when no OperationSchemaProvider")
	assert.Empty(t, op.Outputs, "outputs must be empty when no OperationSchemaProvider")
}

// legacyNoSchemaPlugin is a helper for TestListOperations_WithoutSchemaProvider_RemainsNameOnly.
// It implements OperationProvider but NOT OperationSchemaProvider, representing the
// class of plugins that existed before the optional interface was introduced.
type legacyNoSchemaPlugin struct {
	BasePlugin
}

func (p *legacyNoSchemaPlugin) Operations() []string { return []string{"myop"} }

func (p *legacyNoSchemaPlugin) HandleOperation(_ context.Context, _ string, _ map[string]any) (*OperationResult, error) {
	return NewSuccessResult("done", nil), nil
}

// TestGetOperationSchema_UnknownName_ReturnsNotOK is a protocol test for the
// GetOperationSchema helper: unknown names must return (zero, false).
func TestGetOperationSchema_UnknownName_ReturnsNotOK(t *testing.T) {
	plugin := &richSchemaPlugin{BasePlugin: BasePlugin{PluginName: "rich", PluginVersion: "1.0.0"}}

	meta, ok := plugin.GetOperationSchema("does-not-exist")

	assert.False(t, ok)
	assert.Empty(t, meta.Description)
	assert.Empty(t, meta.Inputs)
	assert.Empty(t, meta.Outputs)
}
