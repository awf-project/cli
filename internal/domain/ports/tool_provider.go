package ports

import "context"

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
	Source      string
}

type ToolContent struct {
	Type string
	Text string
}

type ToolResult struct {
	Content []ToolContent
	IsError bool
}

type ToolProvider interface {
	ListTools(ctx context.Context) ([]ToolDefinition, error)
	CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error)
	Close(ctx context.Context) error
}

// ToolRouter is the narrow contract handed to agent providers that need to discover and
// invoke tools without owning their lifecycle. It is structurally a ToolProvider minus
// Close — the lifecycle stays with the component that constructed the router (e.g. the
// application's MCP proxy service), so leaking Close to the agent would invite
// double-close bugs. Both application/tools.Router and any future routing implementation
// satisfy this interface.
type ToolRouter interface {
	ListTools(ctx context.Context) ([]ToolDefinition, error)
	CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error)
}
