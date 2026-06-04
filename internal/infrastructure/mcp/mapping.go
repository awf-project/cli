package mcp

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/awf-project/cli/internal/domain/ports"
)

func toolToMCP(td *ports.ToolDefinition) *sdkmcp.Tool {
	schema := td.InputSchema
	if len(schema) == 0 {
		schema = map[string]any{"type": "object"}
	}
	return &sdkmcp.Tool{
		Name:        td.Name,
		Description: td.Description,
		InputSchema: schema,
	}
}

func resultToMCP(r *ports.ToolResult) *sdkmcp.CallToolResult {
	result := &sdkmcp.CallToolResult{IsError: r.IsError}
	for _, c := range r.Content {
		if c.Type == "text" {
			result.Content = append(result.Content, &sdkmcp.TextContent{Text: c.Text})
		}
	}
	return result
}
