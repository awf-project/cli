package mcpserver

import (
	"context"
	"encoding/json"
)

// ContentBlock represents a single piece of content in a tool result.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Result is the value returned by a ToolHandler.
type Result struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError"`
}

// InputSchema is a JSON Schema document describing the tool's input.
type InputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string       `json:"required,omitempty"`
}

// ToolHandler is the function signature for a registered MCP tool.
type ToolHandler func(ctx context.Context, args json.RawMessage) (Result, error)

// ToolDefinition holds the public metadata for a registered tool.
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema InputSchema `json:"inputSchema"`
}

// toolEntry is the internal registry entry combining metadata and handler.
type toolEntry struct {
	definition ToolDefinition
	handler    ToolHandler
}
