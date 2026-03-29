package sdk

import (
	"context"
	"encoding/json"
	"fmt"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

// pluginServiceServer implements pluginv1.PluginServiceServer by delegating to the SDK Plugin.
type pluginServiceServer struct {
	pluginv1.UnimplementedPluginServiceServer
	impl Plugin
}

// GetInfo returns information about the plugin.
func (s *pluginServiceServer) GetInfo(ctx context.Context, req *pluginv1.GetInfoRequest) (*pluginv1.GetInfoResponse, error) {
	return &pluginv1.GetInfoResponse{
		Name:         s.impl.Name(),
		Version:      s.impl.Version(),
		Description:  "",
		Capabilities: []string{},
	}, nil
}

// Init initializes the plugin with the provided configuration.
func (s *pluginServiceServer) Init(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	// Decode map[string][]byte to map[string]any using JSON
	config := make(map[string]any)
	for k, v := range req.Config {
		var val any
		if err := json.Unmarshal(v, &val); err != nil {
			// If JSON decode fails, store as raw string
			val = string(v)
		}
		config[k] = val
	}

	if err := s.impl.Init(ctx, config); err != nil {
		return nil, fmt.Errorf("plugin init failed: %w", err)
	}

	return &pluginv1.InitResponse{}, nil
}

// Shutdown shuts down the plugin.
func (s *pluginServiceServer) Shutdown(ctx context.Context, req *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	if err := s.impl.Shutdown(ctx); err != nil {
		return nil, fmt.Errorf("plugin shutdown failed: %w", err)
	}

	return &pluginv1.ShutdownResponse{}, nil
}

// operationServiceServer implements pluginv1.OperationServiceServer.
// Delegates to the plugin if it implements OperationProvider; otherwise returns stubs.
type operationServiceServer struct {
	pluginv1.UnimplementedOperationServiceServer
	impl Plugin
}

// ListOperations returns the list of operations supported by the plugin.
func (s *operationServiceServer) ListOperations(ctx context.Context, req *pluginv1.ListOperationsRequest) (*pluginv1.ListOperationsResponse, error) {
	provider, ok := s.impl.(OperationProvider)
	if !ok {
		return &pluginv1.ListOperationsResponse{
			Operations: []*pluginv1.OperationSchema{},
		}, nil
	}

	opNames := provider.Operations()
	ops := make([]*pluginv1.OperationSchema, len(opNames))
	for i, name := range opNames {
		ops[i] = &pluginv1.OperationSchema{
			Name: name,
		}
	}
	return &pluginv1.ListOperationsResponse{
		Operations: ops,
	}, nil
}

// GetOperation returns information about a specific operation.
func (s *operationServiceServer) GetOperation(ctx context.Context, req *pluginv1.GetOperationRequest) (*pluginv1.GetOperationResponse, error) {
	provider, ok := s.impl.(OperationProvider)
	if !ok {
		return &pluginv1.GetOperationResponse{
			Operation: &pluginv1.OperationSchema{
				Name: req.Name,
			},
		}, nil
	}

	ops := provider.Operations()
	for _, opName := range ops {
		if opName == req.Name {
			return &pluginv1.GetOperationResponse{
				Operation: &pluginv1.OperationSchema{
					Name: req.Name,
				},
			}, nil
		}
	}

	return nil, fmt.Errorf("operation %q not found", req.Name)
}

// Execute executes an operation on the plugin.
func (s *operationServiceServer) Execute(ctx context.Context, req *pluginv1.ExecuteRequest) (resp *pluginv1.ExecuteResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			resp = &pluginv1.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("plugin panic: %v", r),
			}
			err = nil
		}
	}()

	provider, ok := s.impl.(OperationProvider)
	if !ok {
		return &pluginv1.ExecuteResponse{
			Success: false,
			Error:   "plugin does not implement operations",
		}, nil
	}

	// Convert inputs from map[string][]byte to map[string]any
	inputs := make(map[string]any)
	for key, data := range req.Inputs {
		var val any
		if err := json.Unmarshal(data, &val); err != nil {
			// If JSON decode fails, store as raw bytes string
			val = string(data)
		}
		inputs[key] = val
	}

	// Call the operation handler
	result, opErr := provider.HandleOperation(ctx, req.Operation, inputs)
	if opErr != nil {
		return &pluginv1.ExecuteResponse{
			Success: false,
			Error:   opErr.Error(),
		}, nil
	}

	// Convert result back to protobuf format
	// Initialize Data map upfront to avoid potential issues with nil maps
	data := make(map[string][]byte)
	if result.Data != nil {
		for key, val := range result.Data {
			encoded, encErr := json.Marshal(val)
			if encErr == nil {
				data[key] = encoded
			}
		}
	}

	response := &pluginv1.ExecuteResponse{
		Success: result.Success,
		Output:  result.Output,
		Error:   result.Error,
		Data:    data,
	}

	return response, nil
}

// GRPCServer implements the go-plugin GRPCPlugin interface by registering
// the PluginService and OperationService gRPC servers.
func (b *GRPCPluginBridge) GRPCServer(_ *goplugin.GRPCBroker, s *grpc.Server) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// Recover from panic if gRPC server is uninitialized (e.g., created with struct literal).
			// This handles edge cases where server.services map is nil. In normal operation,
			// grpc.NewServer() initializes the server properly and registration succeeds.
			err = fmt.Errorf("panic during gRPC server setup: %v", r)
		}
	}()

	if b == nil || b.impl == nil {
		return fmt.Errorf("gRPC bridge not properly initialized")
	}

	pluginv1.RegisterPluginServiceServer(s, &pluginServiceServer{impl: b.impl})
	pluginv1.RegisterOperationServiceServer(s, &operationServiceServer{impl: b.impl})
	pluginv1.RegisterValidatorServiceServer(s, &validatorServiceServer{impl: b.impl})
	pluginv1.RegisterStepTypeServiceServer(s, &stepTypeServiceServer{impl: b.impl})
	return nil
}

// GRPCClient is required by the go-plugin GRPCPlugin interface but is never
// called on the plugin side. The host uses its own client implementation.
func (b *GRPCPluginBridge) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, _ *grpc.ClientConn) (interface{}, error) {
	return nil, fmt.Errorf("GRPCClient called on plugin side — this is a host-only method")
}
