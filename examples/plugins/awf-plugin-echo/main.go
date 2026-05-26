package main

import (
	"context"
	"fmt"

	"github.com/awf-project/cli/pkg/plugin/sdk"
)

// EchoPlugin implements sdk.Plugin, sdk.OperationProvider, and sdk.OperationSchemaProvider.
// It exposes a single "echo" operation that returns its input text unchanged.
// The rich schema is surfaced via OperationSchemaProvider so that MCP hosts and
// AI agents see documented inputs and outputs rather than an opaque tool handle.
type EchoPlugin struct {
	sdk.BasePlugin
}

func (p *EchoPlugin) Operations() []string {
	return []string{"echo"}
}

// GetOperationSchema implements sdk.OperationSchemaProvider.
// Returns full metadata for the "echo" operation so that MCP hosts can expose
// a documented tool surface to AI agents. Returns (zero, false) for unknown names.
func (p *EchoPlugin) GetOperationSchema(name string) (sdk.OperationMeta, bool) {
	if name != "echo" {
		return sdk.OperationMeta{}, false
	}
	return sdk.OperationMeta{
		Description: "Echo the input text back, optionally prefixed.",
		Inputs: []sdk.InputMeta{
			{Name: "text", Type: sdk.InputTypeString, Required: true, Description: "Text to echo back."},
			{Name: "prefix", Type: sdk.InputTypeString, Description: "Optional prefix prepended to the text."},
		},
		Outputs: []sdk.OutputMeta{
			{Name: "text", Type: sdk.InputTypeString, Description: "The original input text."},
			{Name: "prefix", Type: sdk.InputTypeString, Description: "The prefix that was applied (empty if none)."},
		},
	}, true
}

func (p *EchoPlugin) HandleOperation(_ context.Context, name string, inputs map[string]any) (*sdk.OperationResult, error) {
	if name != "echo" {
		return nil, fmt.Errorf("unknown operation: %s", name)
	}

	text, ok := sdk.GetString(inputs, "text")
	if !ok {
		return nil, fmt.Errorf("missing required input: text")
	}

	prefix := sdk.GetStringDefault(inputs, "prefix", "")
	output := text
	if prefix != "" {
		output = fmt.Sprintf("%s %s", prefix, text)
	}

	return sdk.NewSuccessResult(output, map[string]any{
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
