package sdk

import (
	"context"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// testPlugin implements Plugin for testing.
type testPlugin struct {
	BasePlugin
}

func (p *testPlugin) Init(_ context.Context, _ map[string]any) error {
	return nil
}

func (p *testPlugin) Shutdown(_ context.Context) error {
	return nil
}

// TestHandshakeConfig_Values verifies the handshake has correct magic cookie and protocol version.
func TestHandshakeConfig_Values(t *testing.T) {
	assert.Equal(t, "AWF_PLUGIN", Handshake.MagicCookieKey)
	assert.Equal(t, "awf-plugin-v1", Handshake.MagicCookieValue)
	assert.Equal(t, uint(1), Handshake.ProtocolVersion)
}

// TestHandshakeConfig_Exported verifies Handshake is accessible from outside the package.
func TestHandshakeConfig_Exported(t *testing.T) {
	// This test verifies the Handshake constant is exported (accessible).
	// If Handshake were unexported, this would fail to compile.
	hs := Handshake
	assert.NotNil(t, hs)
}

// TestGRPCPluginBridge_Implements_GRPCPlugin verifies GRPCPluginBridge satisfies go-plugin interface.
func TestGRPCPluginBridge_Implements_GRPCPlugin(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "test-bridge",
			PluginVersion: "1.0.0",
		},
	}

	// This compile-time check verifies GRPCPluginBridge implements goplugin.GRPCPlugin.
	// The test passes if the bridge can be assigned to the interface.
	var _ goplugin.GRPCPlugin = (*GRPCPluginBridge)(nil)

	bridge := &GRPCPluginBridge{
		NetRPCUnsupportedPlugin: goplugin.NetRPCUnsupportedPlugin{},
		impl:                    plugin,
	}

	assert.NotNil(t, bridge)
}

// TestGRPCPluginBridge_StoresPlugin verifies GRPCPluginBridge correctly stores the plugin implementation.
func TestGRPCPluginBridge_StoresPlugin(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "stored-plugin",
			PluginVersion: "2.0.0",
		},
	}

	bridge := &GRPCPluginBridge{
		NetRPCUnsupportedPlugin: goplugin.NetRPCUnsupportedPlugin{},
		impl:                    plugin,
	}

	assert.Equal(t, plugin, bridge.impl)
	assert.Equal(t, "stored-plugin", bridge.impl.Name())
	assert.Equal(t, "2.0.0", bridge.impl.Version())
}

// TestGRPCPluginBridge_GRPCServer_RegistersPlugin verifies GRPCServer registers plugin with gRPC.
func TestGRPCPluginBridge_GRPCServer_RegistersPlugin(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "server-test",
			PluginVersion: "1.0.0",
		},
	}

	bridge := &GRPCPluginBridge{
		NetRPCUnsupportedPlugin: goplugin.NetRPCUnsupportedPlugin{},
		impl:                    plugin,
	}

	server := grpc.NewServer()
	defer server.Stop()

	err := bridge.GRPCServer(nil, server)
	assert.NoError(t, err)
}

// TestGRPCPluginBridge_GRPCClient_IsHostOnlyMethod verifies GRPCClient returns an error
// on the plugin side, since it is a host-only method never invoked during plugin execution.
func TestGRPCPluginBridge_GRPCClient_IsHostOnlyMethod(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "client-test",
			PluginVersion: "1.0.0",
		},
	}

	bridge := &GRPCPluginBridge{
		NetRPCUnsupportedPlugin: goplugin.NetRPCUnsupportedPlugin{},
		impl:                    plugin,
	}

	// GRPCClient is required by the go-plugin GRPCPlugin interface but must not be
	// called on the plugin side. The host has its own client implementation.
	result, err := bridge.GRPCClient(context.Background(), nil, &grpc.ClientConn{})

	assert.Error(t, err, "GRPCClient must return an error when called on the plugin side")
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "host-only method")
}

// TestGRPCPluginBridge_GRPCServer_WithValidServer verifies GRPCServer registers on valid server.
func TestGRPCPluginBridge_GRPCServer_WithValidServer(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "valid-server",
			PluginVersion: "1.0.0",
		},
	}

	bridge := &GRPCPluginBridge{
		NetRPCUnsupportedPlugin: goplugin.NetRPCUnsupportedPlugin{},
		impl:                    plugin,
	}

	// Real implementation must register services on the provided gRPC server.
	server := grpc.NewServer()
	defer server.Stop()

	err := bridge.GRPCServer(nil, server)
	require.NoError(t, err, "GRPCServer must succeed with valid server")

	// Verify server has services registered by checking ServiceInfo.
	info := server.GetServiceInfo()
	assert.NotEmpty(t, info, "GRPCServer must register services on the server")
}

// TestGRPCPluginBridge_GRPCClient_RejectsConnectionArg verifies GRPCClient errors regardless
// of the provided connection, since this method must never be invoked on the plugin side.
func TestGRPCPluginBridge_GRPCClient_RejectsConnectionArg(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "valid-conn",
			PluginVersion: "1.0.0",
		},
	}

	bridge := &GRPCPluginBridge{
		NetRPCUnsupportedPlugin: goplugin.NetRPCUnsupportedPlugin{},
		impl:                    plugin,
	}

	// GRPCClient is a host-only method. Even with a valid connection argument,
	// it must return an error when called on the plugin side.
	result, err := bridge.GRPCClient(context.Background(), nil, &grpc.ClientConn{})

	assert.Error(t, err, "GRPCClient must error when called on the plugin side")
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "GRPCClient called on plugin side")
}

// TestGRPCPluginBridge_MultipleInstances_Independent verifies multiple bridges are independent.
func TestGRPCPluginBridge_MultipleInstances_Independent(t *testing.T) {
	plugin1 := &testPlugin{
		BasePlugin{
			PluginName:    "plugin-1",
			PluginVersion: "1.0.0",
		},
	}

	plugin2 := &testPlugin{
		BasePlugin{
			PluginName:    "plugin-2",
			PluginVersion: "2.0.0",
		},
	}

	bridge1 := &GRPCPluginBridge{
		NetRPCUnsupportedPlugin: goplugin.NetRPCUnsupportedPlugin{},
		impl:                    plugin1,
	}

	bridge2 := &GRPCPluginBridge{
		NetRPCUnsupportedPlugin: goplugin.NetRPCUnsupportedPlugin{},
		impl:                    plugin2,
	}

	assert.NotEqual(t, bridge1.impl.Name(), bridge2.impl.Name())
	assert.Equal(t, "plugin-1", bridge1.impl.Name())
	assert.Equal(t, "plugin-2", bridge2.impl.Name())
}

// TestServeFunction_Signature verifies Serve function exists and is callable.
func TestServeFunction_Signature(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "serve-test",
			PluginVersion: "1.0.0",
		},
	}

	// The Serve function is blocking and starts the go-plugin server.
	// We cannot fully test it without spawning a real process, but we verify
	// the function signature is correct and accepts a Plugin.
	// Calling Serve in a test would block indefinitely, so we skip the actual call.

	// Compile-time verification that Serve accepts a Plugin.
	var _ Plugin = plugin
	_ = Serve // Verify function exists
}

// TestGRPCPluginBridge_PluginInterface_Compliance verifies bridge works with Plugin interface.
func TestGRPCPluginBridge_PluginInterface_Compliance(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "interface-test",
			PluginVersion: "3.0.0",
		},
	}

	// Create a bridge with a plugin that satisfies Plugin interface.
	var iface Plugin = plugin
	bridge := &GRPCPluginBridge{
		NetRPCUnsupportedPlugin: goplugin.NetRPCUnsupportedPlugin{},
		impl:                    iface,
	}

	assert.Equal(t, iface, bridge.impl)
	assert.Equal(t, "interface-test", bridge.impl.Name())
}
