package mcp

import (
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
)

func TestToolToMCP_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		toolDef   *ports.ToolDefinition
		checkFunc func(t *testing.T, tool *sdkmcp.Tool)
	}{
		{
			name: "basic tool definition",
			toolDef: &ports.ToolDefinition{
				Name:        "bash",
				Description: "Execute bash commands",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type": "string",
						},
					},
					"required": []any{"command"},
				},
			},
			checkFunc: func(t *testing.T, tool *sdkmcp.Tool) {
				require.NotNil(t, tool)
				assert.Equal(t, "bash", tool.Name)
				assert.Equal(t, "Execute bash commands", tool.Description)
				require.NotNil(t, tool.InputSchema)

				schemaMap, ok := tool.InputSchema.(map[string]any)
				require.True(t, ok, "InputSchema should be convertible to map[string]any")
				assert.Equal(t, "object", schemaMap["type"])
			},
		},
		{
			name: "tool with complex input schema",
			toolDef: &ports.ToolDefinition{
				Name:        "search",
				Description: "Search the web",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type": "string",
						},
						"limit": map[string]any{
							"type": "integer",
						},
					},
					"required": []any{"query"},
				},
			},
			checkFunc: func(t *testing.T, tool *sdkmcp.Tool) {
				require.NotNil(t, tool)
				assert.Equal(t, "search", tool.Name)
				assert.Equal(t, "Search the web", tool.Description)
				require.NotNil(t, tool.InputSchema)

				schemaMap, ok := tool.InputSchema.(map[string]any)
				require.True(t, ok)
				props, ok := schemaMap["properties"].(map[string]any)
				require.True(t, ok)
				assert.NotNil(t, props["query"])
				assert.NotNil(t, props["limit"])
			},
		},
		{
			name: "tool with nil input schema",
			toolDef: &ports.ToolDefinition{
				Name:        "status",
				Description: "Get status",
				InputSchema: nil,
			},
			checkFunc: func(t *testing.T, tool *sdkmcp.Tool) {
				require.NotNil(t, tool)
				assert.Equal(t, "status", tool.Name)
				require.NotNil(t, tool.InputSchema, "InputSchema must be non-nil")

				schemaMap, ok := tool.InputSchema.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "object", schemaMap["type"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := toolToMCP(tt.toolDef)
			require.NotNil(t, tool)
			tt.checkFunc(t, tool)
		})
	}
}

func TestResultToMCP_TextContent(t *testing.T) {
	tests := []struct {
		name      string
		result    *ports.ToolResult
		checkFunc func(t *testing.T, callToolResult *sdkmcp.CallToolResult)
	}{
		{
			name: "single text content",
			result: &ports.ToolResult{
				Content: []ports.ToolContent{
					{
						Type: "text",
						Text: "hello world",
					},
				},
				IsError: false,
			},
			checkFunc: func(t *testing.T, callToolResult *sdkmcp.CallToolResult) {
				require.NotNil(t, callToolResult)
				require.NotNil(t, callToolResult.Content)
				assert.Len(t, callToolResult.Content, 1)
				textContent, ok := callToolResult.Content[0].(*sdkmcp.TextContent)
				require.True(t, ok, "content should be TextContent type")
				assert.Equal(t, "hello world", textContent.Text)
				assert.False(t, callToolResult.IsError)
			},
		},
		{
			name: "multiple text content blocks",
			result: &ports.ToolResult{
				Content: []ports.ToolContent{
					{
						Type: "text",
						Text: "first block",
					},
					{
						Type: "text",
						Text: "second block",
					},
				},
				IsError: false,
			},
			checkFunc: func(t *testing.T, callToolResult *sdkmcp.CallToolResult) {
				require.NotNil(t, callToolResult)
				require.NotNil(t, callToolResult.Content)
				assert.Len(t, callToolResult.Content, 2)
				textContent1, ok := callToolResult.Content[0].(*sdkmcp.TextContent)
				require.True(t, ok)
				assert.Equal(t, "first block", textContent1.Text)
				textContent2, ok := callToolResult.Content[1].(*sdkmcp.TextContent)
				require.True(t, ok)
				assert.Equal(t, "second block", textContent2.Text)
			},
		},
		{
			name: "text content with error flag",
			result: &ports.ToolResult{
				Content: []ports.ToolContent{
					{
						Type: "text",
						Text: "error message",
					},
				},
				IsError: true,
			},
			checkFunc: func(t *testing.T, callToolResult *sdkmcp.CallToolResult) {
				require.NotNil(t, callToolResult)
				assert.True(t, callToolResult.IsError)
				assert.Len(t, callToolResult.Content, 1)
				textContent, ok := callToolResult.Content[0].(*sdkmcp.TextContent)
				require.True(t, ok)
				assert.Equal(t, "error message", textContent.Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callToolResult := resultToMCP(tt.result)
			require.NotNil(t, callToolResult)
			tt.checkFunc(t, callToolResult)
		})
	}
}

func TestResultToMCP_MixedAndNonTextContent(t *testing.T) {
	tests := []struct {
		name      string
		result    *ports.ToolResult
		checkFunc func(t *testing.T, callToolResult *sdkmcp.CallToolResult)
	}{
		{
			name: "silently drop non-text content",
			result: &ports.ToolResult{
				Content: []ports.ToolContent{
					{
						Type: "image",
						Text: "should be dropped",
					},
				},
				IsError: false,
			},
			checkFunc: func(t *testing.T, callToolResult *sdkmcp.CallToolResult) {
				require.NotNil(t, callToolResult)
				assert.Empty(t, callToolResult.Content, "non-text content should be silently dropped")
			},
		},
		{
			name: "mixed text and non-text content",
			result: &ports.ToolResult{
				Content: []ports.ToolContent{
					{
						Type: "text",
						Text: "keep this",
					},
					{
						Type: "image",
						Text: "drop this",
					},
					{
						Type: "text",
						Text: "keep this too",
					},
					{
						Type: "structured",
						Text: "drop this",
					},
				},
				IsError: false,
			},
			checkFunc: func(t *testing.T, callToolResult *sdkmcp.CallToolResult) {
				require.NotNil(t, callToolResult)
				require.NotNil(t, callToolResult.Content)
				assert.Len(t, callToolResult.Content, 2, "only text content should be kept")
				textContent1, ok := callToolResult.Content[0].(*sdkmcp.TextContent)
				require.True(t, ok)
				assert.Equal(t, "keep this", textContent1.Text)
				textContent2, ok := callToolResult.Content[1].(*sdkmcp.TextContent)
				require.True(t, ok)
				assert.Equal(t, "keep this too", textContent2.Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callToolResult := resultToMCP(tt.result)
			tt.checkFunc(t, callToolResult)
		})
	}
}

func TestResultToMCP_EmptyContent(t *testing.T) {
	tests := []struct {
		name      string
		result    *ports.ToolResult
		checkFunc func(t *testing.T, callToolResult *sdkmcp.CallToolResult)
	}{
		{
			name: "empty content slice",
			result: &ports.ToolResult{
				Content: []ports.ToolContent{},
				IsError: false,
			},
			checkFunc: func(t *testing.T, callToolResult *sdkmcp.CallToolResult) {
				require.NotNil(t, callToolResult, "resultToMCP must return non-nil even with empty content")
				assert.Empty(t, callToolResult.Content)
				assert.False(t, callToolResult.IsError)
			},
		},
		{
			name: "nil content slice",
			result: &ports.ToolResult{
				Content: nil,
				IsError: true,
			},
			checkFunc: func(t *testing.T, callToolResult *sdkmcp.CallToolResult) {
				require.NotNil(t, callToolResult, "resultToMCP must return non-nil even with nil content")
				assert.True(t, callToolResult.IsError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callToolResult := resultToMCP(tt.result)
			tt.checkFunc(t, callToolResult)
		})
	}
}
