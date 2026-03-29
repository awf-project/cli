package sdk

import (
	"context"
	"encoding/json"
	"fmt"

	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

// StepTypeInfo describes a custom step type registered by a plugin.
type StepTypeInfo struct {
	Name        string
	Description string
}

// StepExecuteRequest carries the execution context for a custom step type.
type StepExecuteRequest struct {
	StepName string
	StepType string
	Config   map[string]any
	Inputs   map[string]any
}

// StepExecuteResult holds the result of a custom step type execution.
type StepExecuteResult struct {
	Output   string
	Data     map[string]any
	ExitCode int32
}

// StepTypeHandler is the plugin-author interface for custom step types.
// Implement this interface to add new step types available in workflow YAML via `type:`.
//
// StepTypes is called once after Init() to register available types.
// ExecuteStep is called each time a workflow step with a matching type runs.
type StepTypeHandler interface {
	StepTypes() []StepTypeInfo
	ExecuteStep(ctx context.Context, req StepExecuteRequest) (StepExecuteResult, error)
}

// stepTypeServiceServer implements pluginv1.StepTypeServiceServer.
// Delegates to the plugin if it implements StepTypeHandler; otherwise returns empty results.
type stepTypeServiceServer struct {
	pluginv1.UnimplementedStepTypeServiceServer
	impl Plugin
}

func (s *stepTypeServiceServer) ListStepTypes(ctx context.Context, _ *pluginv1.ListStepTypesRequest) (*pluginv1.ListStepTypesResponse, error) {
	handler, ok := s.impl.(StepTypeHandler)
	if !ok {
		return &pluginv1.ListStepTypesResponse{}, nil
	}

	types := handler.StepTypes()
	infos := make([]*pluginv1.StepTypeInfo, len(types))
	for i, t := range types {
		infos[i] = &pluginv1.StepTypeInfo{
			Name:        t.Name,
			Description: t.Description,
		}
	}
	return &pluginv1.ListStepTypesResponse{StepTypes: infos}, nil
}

func (s *stepTypeServiceServer) ExecuteStep(ctx context.Context, req *pluginv1.ExecuteStepRequest) (*pluginv1.ExecuteStepResponse, error) {
	handler, ok := s.impl.(StepTypeHandler)
	if !ok {
		return nil, fmt.Errorf("plugin does not implement step types")
	}

	var config map[string]any
	if len(req.Config) > 0 {
		if err := json.Unmarshal(req.Config, &config); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
	}

	var inputs map[string]any
	if len(req.Inputs) > 0 {
		if err := json.Unmarshal(req.Inputs, &inputs); err != nil {
			return nil, fmt.Errorf("unmarshal inputs: %w", err)
		}
	}

	result, err := handler.ExecuteStep(ctx, StepExecuteRequest{
		StepName: req.StepName,
		StepType: req.StepType,
		Config:   config,
		Inputs:   inputs,
	})
	if err != nil {
		return nil, fmt.Errorf("execute step: %w", err)
	}

	var dataBytes []byte
	if result.Data != nil {
		encoded, encErr := json.Marshal(result.Data)
		if encErr != nil {
			return nil, fmt.Errorf("marshal result data: %w", encErr)
		}
		dataBytes = encoded
	}

	return &pluginv1.ExecuteStepResponse{
		Output:   result.Output,
		Data:     dataBytes,
		ExitCode: result.ExitCode,
	}, nil
}
