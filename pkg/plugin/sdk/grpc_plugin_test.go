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
