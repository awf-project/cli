//go:build integration

package cli_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/awf-project/cli/pkg/plugin/sdk"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

// The fixture serves the real go-plugin handshake and records only deadline
// presence at the RPC boundary; it never records request/configuration content.
type timeoutPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
}

func (*timeoutPlugin) GRPCClient(context.Context, *goplugin.GRPCBroker, *grpc.ClientConn) (any, error) {
	return nil, errors.New("fixture only serves plugins")
}

func (*timeoutPlugin) GRPCServer(_ *goplugin.GRPCBroker, server *grpc.Server) error {
	fixture := &timeoutPluginService{}
	pluginv1.RegisterPluginServiceServer(server, fixture)
	pluginv1.RegisterStepTypeServiceServer(server, fixture)
	return nil
}

func serveTimeoutPlugin() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: sdk.Handshake,
		Plugins:         goplugin.PluginSet{"awf-plugin": &timeoutPlugin{}},
		GRPCServer:      goplugin.DefaultGRPCServer,
	})
}

type timeoutPluginService struct {
	pluginv1.UnimplementedPluginServiceServer
	pluginv1.UnimplementedStepTypeServiceServer
}

func (*timeoutPluginService) GetInfo(context.Context, *pluginv1.GetInfoRequest) (*pluginv1.GetInfoResponse, error) {
	return &pluginv1.GetInfoResponse{Name: "deadline-fixture", Version: "1.0.0", Capabilities: []string{"step_types"}}, nil
}

func (*timeoutPluginService) Init(context.Context, *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	return &pluginv1.InitResponse{}, nil
}

func (*timeoutPluginService) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

func (*timeoutPluginService) ListStepTypes(ctx context.Context, _ *pluginv1.ListStepTypesRequest) (*pluginv1.ListStepTypesResponse, error) {
	_, bounded := ctx.Deadline()
	if err := os.WriteFile(filepath.Join(os.Getenv("AWF_TIMEOUT_FIXTURE_DIR"), "discovery"), []byte(boolText(bounded)), 0o600); err != nil { //nolint:gosec // G703: test-owned directory supplied by the self-hosting parent
		return nil, err
	}
	return &pluginv1.ListStepTypesResponse{StepTypes: []*pluginv1.StepTypeInfo{{Name: "wait"}}}, nil
}

func (*timeoutPluginService) ExecuteStep(ctx context.Context, _ *pluginv1.ExecuteStepRequest) (*pluginv1.ExecuteStepResponse, error) {
	root := os.Getenv("AWF_TIMEOUT_FIXTURE_DIR")
	deadline, bounded := ctx.Deadline()
	evidence, err := json.Marshal(struct {
		Bounded  bool
		Deadline time.Time
	}{bounded, deadline})
	if err != nil {
		return nil, err
	}
	if writeErr := os.WriteFile(filepath.Join(root, "execution"), evidence, 0o600); writeErr != nil { //nolint:gosec // G703: test-owned directory supplied by the self-hosting parent
		return nil, writeErr
	}
	defer func() { _ = os.WriteFile(filepath.Join(root, "stopped"), nil, 0o600) }() //nolint:gosec // G703: test-owned directory supplied by the self-hosting parent
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if timeoutFileExists(filepath.Join(root, "release")) {
				return &pluginv1.ExecuteStepResponse{Output: "plugin-completed"}, nil
			}
		}
	}
}

func boolText(value bool) string {
	if value {
		return "bounded"
	}
	return "unbounded"
}
