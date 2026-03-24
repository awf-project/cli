package sdk

import (
	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

// Handshake is the shared handshake config between AWF host and plugins.
// Plugin binaries must use this config to be recognized by AWF.
var Handshake = goplugin.HandshakeConfig{
	// AWF_PLUGIN is the magic cookie key; the value must match for the
	// host to accept the plugin process (prevents accidental execution).
	MagicCookieKey:   "AWF_PLUGIN",
	MagicCookieValue: "awf-plugin-v1",
	ProtocolVersion:  1,
}

// pluginSetKey is the key used in the PluginSet map for the AWF plugin.
const pluginSetKey = "awf-plugin"

// Serve starts the plugin process, blocking until the host disconnects.
// Plugin binaries call this from their main() after constructing their Plugin.
func Serve(p Plugin) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: goplugin.PluginSet{
			pluginSetKey: &GRPCPluginBridge{impl: p},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger:     hclog.NewNullLogger(),
	})
}

// GRPCPluginBridge adapts sdk.Plugin to the go-plugin GRPCPlugin interface.
// Implementation in grpc_plugin.go bridges go-plugin interface to gRPC servers/clients.
type GRPCPluginBridge struct {
	goplugin.NetRPCUnsupportedPlugin
	impl Plugin
}

var (
	_ goplugin.Plugin     = (*GRPCPluginBridge)(nil)
	_ goplugin.GRPCPlugin = (*GRPCPluginBridge)(nil)
)
