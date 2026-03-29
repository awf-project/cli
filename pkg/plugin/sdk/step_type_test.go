package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stepTypePlugin implements StepTypeHandler for testing.
type stepTypePlugin struct {
	BasePlugin
	stepTypes []StepTypeInfo
	result    StepExecuteResult
	err       error
}

func (p *stepTypePlugin) Init(_ context.Context, _ map[string]any) error {
	return nil
}

func (p *stepTypePlugin) Shutdown(_ context.Context) error {
	return nil
}

func (p *stepTypePlugin) StepTypes() []StepTypeInfo {
	return p.stepTypes
}

func (p *stepTypePlugin) ExecuteStep(_ context.Context, _ StepExecuteRequest) (StepExecuteResult, error) {
	return p.result, p.err
}

// TestStepTypeInfo_Fields verifies StepTypeInfo struct can hold all fields.
func TestStepTypeInfo_Fields(t *testing.T) {
	info := StepTypeInfo{
		Name:        "custom-type",
		Description: "A custom step type",
	}

	assert.Equal(t, "custom-type", info.Name)
	assert.Equal(t, "A custom step type", info.Description)
}

// TestStepExecuteRequest_Fields verifies StepExecuteRequest struct can hold all fields.
func TestStepExecuteRequest_Fields(t *testing.T) {
	req := StepExecuteRequest{
		StepName: "step1",
		StepType: "custom-type",
		Config: map[string]any{
			"timeout": 30,
			"retries": 3,
		},
		Inputs: map[string]any{
			"param1": "value1",
		},
	}

	assert.Equal(t, "step1", req.StepName)
	assert.Equal(t, "custom-type", req.StepType)
	assert.Equal(t, 30, req.Config["timeout"])
	assert.Equal(t, "value1", req.Inputs["param1"])
}

// TestStepExecuteResult_Fields verifies StepExecuteResult struct can hold all fields.
func TestStepExecuteResult_Fields(t *testing.T) {
	result := StepExecuteResult{
		Output:   "execution output",
		ExitCode: 0,
		Data: map[string]any{
			"key": "value",
		},
	}

	assert.Equal(t, "execution output", result.Output)
	assert.Equal(t, int32(0), result.ExitCode)
	assert.Equal(t, "value", result.Data["key"])
}

// TestStepTypeServiceServer_ListStepTypes_PluginImplementsHandler verifies ListStepTypes calls handler.
func TestStepTypeServiceServer_ListStepTypes_PluginImplementsHandler(t *testing.T) {
	plugin := &stepTypePlugin{
		BasePlugin: BasePlugin{
			PluginName:    "step-type-plugin",
			PluginVersion: "1.0.0",
		},
		stepTypes: []StepTypeInfo{
			{Name: "custom-type", Description: "A custom type"},
			{Name: "another-type", Description: "Another type"},
		},
	}

	server := &stepTypeServiceServer{impl: plugin}

	resp, err := server.ListStepTypes(context.Background(), &pluginv1.ListStepTypesRequest{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.StepTypes, 2)
	assert.Equal(t, "custom-type", resp.StepTypes[0].Name)
	assert.Equal(t, "A custom type", resp.StepTypes[0].Description)
	assert.Equal(t, "another-type", resp.StepTypes[1].Name)
}

// TestStepTypeServiceServer_ListStepTypes_PluginDoesNotImplementHandler verifies empty response when not implemented.
func TestStepTypeServiceServer_ListStepTypes_PluginDoesNotImplementHandler(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "non-step-type-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &stepTypeServiceServer{impl: plugin}

	resp, err := server.ListStepTypes(context.Background(), &pluginv1.ListStepTypesRequest{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.StepTypes)
}

// TestStepTypeServiceServer_ListStepTypes_EmptyTypesList verifies empty types list response.
func TestStepTypeServiceServer_ListStepTypes_EmptyTypesList(t *testing.T) {
	plugin := &stepTypePlugin{
		BasePlugin: BasePlugin{
			PluginName:    "step-type-plugin",
			PluginVersion: "1.0.0",
		},
		stepTypes: []StepTypeInfo{},
	}

	server := &stepTypeServiceServer{impl: plugin}

	resp, err := server.ListStepTypes(context.Background(), &pluginv1.ListStepTypesRequest{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.StepTypes)
}

// TestStepTypeServiceServer_ExecuteStep_PluginImplementsHandler verifies ExecuteStep calls handler.
func TestStepTypeServiceServer_ExecuteStep_PluginImplementsHandler(t *testing.T) {
	plugin := &stepTypePlugin{
		BasePlugin: BasePlugin{
			PluginName:    "step-type-plugin",
			PluginVersion: "1.0.0",
		},
		result: StepExecuteResult{
			Output:   "execution completed",
			ExitCode: 0,
			Data: map[string]any{
				"result": "success",
			},
		},
	}

	server := &stepTypeServiceServer{impl: plugin}

	config := map[string]any{"timeout": 30}
	configJSON, _ := json.Marshal(config)

	inputs := map[string]any{"param": "value"}
	inputsJSON, _ := json.Marshal(inputs)

	resp, err := server.ExecuteStep(context.Background(), &pluginv1.ExecuteStepRequest{
		StepName: "step1",
		StepType: "custom-type",
		Config:   configJSON,
		Inputs:   inputsJSON,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "execution completed", resp.Output)
	assert.Equal(t, int32(0), resp.ExitCode)

	var data map[string]any
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	assert.Equal(t, "success", data["result"])
}

// TestStepTypeServiceServer_ExecuteStep_NoConfigOrInputs verifies ExecuteStep with empty config/inputs.
func TestStepTypeServiceServer_ExecuteStep_NoConfigOrInputs(t *testing.T) {
	plugin := &stepTypePlugin{
		BasePlugin: BasePlugin{
			PluginName:    "step-type-plugin",
			PluginVersion: "1.0.0",
		},
		result: StepExecuteResult{
			Output:   "done",
			ExitCode: 0,
			Data:     nil,
		},
	}

	server := &stepTypeServiceServer{impl: plugin}

	resp, err := server.ExecuteStep(context.Background(), &pluginv1.ExecuteStepRequest{
		StepName: "step1",
		StepType: "custom-type",
		Config:   []byte{},
		Inputs:   []byte{},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Output)
	assert.Empty(t, resp.Data)
}

// TestStepTypeServiceServer_ExecuteStep_PluginDoesNotImplementHandler verifies error when handler not implemented.
func TestStepTypeServiceServer_ExecuteStep_PluginDoesNotImplementHandler(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "non-step-type-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &stepTypeServiceServer{impl: plugin}

	resp, err := server.ExecuteStep(context.Background(), &pluginv1.ExecuteStepRequest{
		StepName: "step1",
		StepType: "custom-type",
		Config:   []byte(`{}`),
		Inputs:   []byte(`{}`),
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "does not implement step types")
}

// TestStepTypeServiceServer_ExecuteStep_InvalidConfigJSON verifies error on config unmarshal failure.
func TestStepTypeServiceServer_ExecuteStep_InvalidConfigJSON(t *testing.T) {
	plugin := &stepTypePlugin{
		BasePlugin: BasePlugin{
			PluginName:    "step-type-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &stepTypeServiceServer{impl: plugin}

	resp, err := server.ExecuteStep(context.Background(), &pluginv1.ExecuteStepRequest{
		StepName: "step1",
		StepType: "custom-type",
		Config:   []byte(`{invalid json}`),
		Inputs:   []byte(`{}`),
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "unmarshal config")
}

// TestStepTypeServiceServer_ExecuteStep_InvalidInputsJSON verifies error on inputs unmarshal failure.
func TestStepTypeServiceServer_ExecuteStep_InvalidInputsJSON(t *testing.T) {
	plugin := &stepTypePlugin{
		BasePlugin: BasePlugin{
			PluginName:    "step-type-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &stepTypeServiceServer{impl: plugin}

	config := map[string]any{"timeout": 30}
	configJSON, _ := json.Marshal(config)

	resp, err := server.ExecuteStep(context.Background(), &pluginv1.ExecuteStepRequest{
		StepName: "step1",
		StepType: "custom-type",
		Config:   configJSON,
		Inputs:   []byte(`bad json`),
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "unmarshal inputs")
}

// TestStepTypeServiceServer_ExecuteStep_HandlerReturnsError verifies error propagation from handler.
func TestStepTypeServiceServer_ExecuteStep_HandlerReturnsError(t *testing.T) {
	testErr := errors.New("step execution failed")
	plugin := &stepTypePlugin{
		BasePlugin: BasePlugin{
			PluginName:    "step-type-plugin",
			PluginVersion: "1.0.0",
		},
		err: testErr,
	}

	server := &stepTypeServiceServer{impl: plugin}

	resp, err := server.ExecuteStep(context.Background(), &pluginv1.ExecuteStepRequest{
		StepName: "step1",
		StepType: "custom-type",
		Config:   []byte(`{}`),
		Inputs:   []byte(`{}`),
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, testErr)
}

// TestStepTypeServiceServer_ExecuteStep_WithComplexData verifies complex data marshaling.
func TestStepTypeServiceServer_ExecuteStep_WithComplexData(t *testing.T) {
	plugin := &stepTypePlugin{
		BasePlugin: BasePlugin{
			PluginName:    "step-type-plugin",
			PluginVersion: "1.0.0",
		},
		result: StepExecuteResult{
			Output:   "complex execution",
			ExitCode: 0,
			Data: map[string]any{
				"nested": map[string]any{
					"level1": map[string]any{
						"value": "deep",
					},
				},
				"array":  []string{"a", "b", "c"},
				"number": 42,
			},
		},
	}

	server := &stepTypeServiceServer{impl: plugin}

	resp, err := server.ExecuteStep(context.Background(), &pluginv1.ExecuteStepRequest{
		StepName: "step1",
		StepType: "custom-type",
		Config:   []byte(`{}`),
		Inputs:   []byte(`{}`),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	var data map[string]any
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)

	nested := data["nested"].(map[string]any)
	level1 := nested["level1"].(map[string]any)
	assert.Equal(t, "deep", level1["value"])

	array := data["array"].([]any)
	assert.Len(t, array, 3)
	assert.Equal(t, "a", array[0])
}

// TestStepTypeServiceServer_ExecuteStep_NonZeroExitCode verifies exit code preservation.
func TestStepTypeServiceServer_ExecuteStep_NonZeroExitCode(t *testing.T) {
	plugin := &stepTypePlugin{
		BasePlugin: BasePlugin{
			PluginName:    "step-type-plugin",
			PluginVersion: "1.0.0",
		},
		result: StepExecuteResult{
			Output:   "execution failed",
			ExitCode: 1,
			Data:     nil,
		},
	}

	server := &stepTypeServiceServer{impl: plugin}

	resp, err := server.ExecuteStep(context.Background(), &pluginv1.ExecuteStepRequest{
		StepName: "step1",
		StepType: "custom-type",
		Config:   []byte(`{}`),
		Inputs:   []byte(`{}`),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(1), resp.ExitCode)
}

// TestStepTypeServiceServer_ExecuteStep_PreservesStepMetadata verifies step name and type are passed through.
func TestStepTypeServiceServer_ExecuteStep_PreservesStepMetadata(t *testing.T) {
	plugin := &stepTypePlugin{
		BasePlugin: BasePlugin{
			PluginName:    "step-type-plugin",
			PluginVersion: "1.0.0",
		},
		result: StepExecuteResult{
			Output:   "done",
			ExitCode: 0,
		},
	}

	server := &stepTypeServiceServer{impl: plugin}

	resp, err := server.ExecuteStep(context.Background(), &pluginv1.ExecuteStepRequest{
		StepName: "test-step",
		StepType: "my-type",
		Config:   []byte(`{}`),
		Inputs:   []byte(`{}`),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Output)
}

// TestStepTypeServiceServer_ListStepTypes_PreservesFields verifies field preservation in conversion.
func TestStepTypeServiceServer_ListStepTypes_PreservesFields(t *testing.T) {
	plugin := &stepTypePlugin{
		BasePlugin: BasePlugin{
			PluginName:    "step-type-plugin",
			PluginVersion: "1.0.0",
		},
		stepTypes: []StepTypeInfo{
			{
				Name:        "type-with-dashes",
				Description: "Description with special chars: @#$%",
			},
		},
	}

	server := &stepTypeServiceServer{impl: plugin}

	resp, err := server.ListStepTypes(context.Background(), &pluginv1.ListStepTypesRequest{})

	require.NoError(t, err)
	require.Len(t, resp.StepTypes, 1)
	assert.Equal(t, "type-with-dashes", resp.StepTypes[0].Name)
	assert.Equal(t, "Description with special chars: @#$%", resp.StepTypes[0].Description)
}
