package pluginmgr

import (
	"encoding/json"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

// convertExecuteResponse converts a gRPC ExecuteResponse to a domain OperationResult.
// Maps Output to Outputs["output"]; Data entries are JSON-decoded and merged (Data wins on conflict).
func convertExecuteResponse(_ string, resp *pluginv1.ExecuteResponse) *pluginmodel.OperationResult {
	outputs := make(map[string]any)

	if resp.Output != "" {
		outputs["output"] = resp.Output
	}

	for key, raw := range resp.Data {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			// Preserve malformed data as raw string fallback.
			value = string(raw)
		}
		outputs[key] = value
	}

	return &pluginmodel.OperationResult{
		Success: resp.Success,
		Outputs: outputs,
		Error:   resp.Error,
	}
}

// convertOperationSchema converts a proto OperationSchema to a domain OperationSchema,
// injecting pluginName from the connection context (not present in proto).
func convertOperationSchema(pluginID string, schema *pluginv1.OperationSchema) *pluginmodel.OperationSchema {
	inputs := make(map[string]pluginmodel.InputSchema)
	for _, protoInput := range schema.Inputs {
		name, domainInput := convertInputSchema(protoInput)
		if name != "" {
			inputs[name] = domainInput
		}
	}

	outputs := make([]string, 0, len(schema.Outputs))
	for _, out := range schema.Outputs {
		outputs = append(outputs, out.Name)
	}

	return &pluginmodel.OperationSchema{
		Name:        pluginID + "." + schema.Name,
		Description: schema.Description,
		PluginName:  pluginID,
		Inputs:      inputs,
		Outputs:     outputs,
	}
}

// convertInputSchema converts a proto InputSchema to a domain InputSchema.
// Returns the input name and the domain schema.
func convertInputSchema(input *pluginv1.InputSchema) (string, pluginmodel.InputSchema) {
	if input == nil {
		return "", pluginmodel.InputSchema{}
	}

	schema := pluginmodel.InputSchema{
		Type:        input.Type,
		Required:    input.Required,
		Description: input.Description,
		Validation:  input.Validation,
	}

	if input.Default != "" {
		schema.Default = input.Default
	}

	return input.Name, schema
}
