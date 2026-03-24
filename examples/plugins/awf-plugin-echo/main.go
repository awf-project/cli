package main

import (
	"context"
	"fmt"

	"github.com/awf-project/cli/pkg/plugin/sdk"
)

// EchoPlugin implements sdk.Plugin and sdk.OperationProvider.
// It exposes a single "echo" operation that returns its input text unchanged.
type EchoPlugin struct {
	sdk.BasePlugin
}

func (p *EchoPlugin) Name() string    { return "awf-plugin-echo" }
func (p *EchoPlugin) Version() string { return "1.0.0" }

func (p *EchoPlugin) Operations() []string {
	return []string{"echo"}
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
			PluginName:    "awf-plugin-echo",
			PluginVersion: "1.0.0",
		},
	})
}
