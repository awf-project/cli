package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/awf-project/cli/internal/domain/ports"
)

func handlerFor(provider ports.ToolProvider, name string) sdkmcp.ToolHandler {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest) (result *sdkmcp.CallToolResult, err error) {
		defer func() {
			if r := recover(); r != nil {
				result = panicResult(r)
			}
		}()

		var args map[string]any
		if req.Params != nil && len(req.Params.Arguments) > 0 {
			if jsonErr := json.Unmarshal(req.Params.Arguments, &args); jsonErr != nil {
				// Malformed JSON args: proceed with an empty map so the tool can return a
				// structured IsError result to the client rather than aborting the whole
				// request (which would surface as an opaque transport-level failure).
				args = map[string]any{}
			}
		}

		toolResult, callErr := provider.CallTool(ctx, name, args)
		if callErr != nil {
			return resultToMCP(&ports.ToolResult{
				IsError: true,
				Content: []ports.ToolContent{{Type: "text", Text: callErr.Error()}},
			}), nil
		}
		if toolResult == nil {
			// A provider returning (nil, nil) is legal per the Go interface contract but
			// would panic resultToMCP on the nil deref. Surface a structured IsError result
			// instead of letting the panic-recovery path emit an opaque transport failure.
			return resultToMCP(&ports.ToolResult{
				IsError: true,
				Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("tool %q returned no result", name)}},
			}), nil
		}
		return resultToMCP(toolResult), nil
	}
}

func panicResult(r any) *sdkmcp.CallToolResult {
	return resultToMCP(&ports.ToolResult{
		IsError: true,
		Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("panic recovered: %v", r)}},
	})
}
