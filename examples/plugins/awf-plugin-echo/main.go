package main

import (
	"context"

	"github.com/awf-project/cli/pkg/plugin/sdk"
)

type EchoPlugin struct {
	sdk.BasePlugin
}

func (p *EchoPlugin) Operations() []string {
	return []string{"echo"}
}

func (p *EchoPlugin) GetOperationSchema(name string) (sdk.OperationMeta, bool) {
	if name != "echo" {
		return sdk.OperationMeta{}, false
	}

	return sdk.OperationMeta{
		Description: "Echo text, optionally prepending a prefix.",
		Inputs: []sdk.InputMeta{
			{Name: "text", Type: sdk.InputTypeString, Required: true, Description: "Text to echo."},
			{Name: "prefix", Type: sdk.InputTypeString, Required: false, Description: "Optional prefix prepended to the text."},
		},
		Outputs: []sdk.OutputMeta{
			{Name: "output", Type: sdk.InputTypeString, Description: "Final echoed output."},
			{Name: "text", Type: sdk.InputTypeString, Description: "Original text input."},
			{Name: "prefix", Type: sdk.InputTypeString, Description: "Prefix input, when provided."},
		},
	}, true
}

func (p *EchoPlugin) HandleOperation(_ context.Context, name string, inputs map[string]any) (*sdk.OperationResult, error) {
	if name != "echo" {
		return sdk.NewErrorResult("unknown operation: " + name), nil
	}

	text, ok := sdk.GetString(inputs, "text")
	if !ok || text == "" {
		return sdk.NewErrorResult("text is required"), nil
	}

	prefix, _ := sdk.GetString(inputs, "prefix")
	output := text
	if prefix != "" {
		output = prefix + text
	}

	return sdk.NewSuccessResult(output, map[string]any{
		"output": output,
		"text":   text,
		"prefix": prefix,
	}), nil
}

func main() {
	sdk.Serve(&EchoPlugin{
		BasePlugin: sdk.BasePlugin{
			PluginName:    "echo",
			PluginVersion: "1.0.0",
		},
	})
}
