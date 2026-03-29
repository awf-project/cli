package main

import (
	"context"

	"github.com/awf-project/cli/pkg/plugin/sdk"
)

// DatabasePlugin implements sdk.Plugin and sdk.StepTypeHandler.
// It provides a custom step type for database operations.
type DatabasePlugin struct {
	sdk.BasePlugin
}

func (p *DatabasePlugin) StepTypes() []sdk.StepTypeInfo {
	return []sdk.StepTypeInfo{
		{
			Name:        "query",
			Description: "Execute a database query and return results",
		},
		{
			Name:        "execute",
			Description: "Execute a database command without returning results",
		},
	}
}

func (p *DatabasePlugin) ExecuteStep(ctx context.Context, req sdk.StepExecuteRequest) (sdk.StepExecuteResult, error) {
	// Simulate successful database operation
	output := "Query executed successfully"

	return sdk.StepExecuteResult{
		Output:   output,
		ExitCode: 0,
		Data: map[string]any{
			"rows_affected": 0,
			"timestamp":     "2026-03-29T05:00:00Z",
		},
	}, nil
}

func main() {
	plugin := &DatabasePlugin{
		BasePlugin: sdk.BasePlugin{
			PluginName:    "database",
			PluginVersion: "1.0.0",
		},
	}
	sdk.Serve(plugin)
}
