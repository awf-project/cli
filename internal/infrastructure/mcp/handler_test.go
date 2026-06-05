package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
)

func TestHandlerFor_SuccessfulCall(t *testing.T) {
	provider := &fakeProvider{
		callResult: &ports.ToolResult{
			IsError: false,
			Content: []ports.ToolContent{
				{Type: "text", Text: "success output"},
			},
		},
	}

	handler := handlerFor(provider, "test-tool")
	require.NotNil(t, handler)

	ctx := context.Background()
	args := map[string]any{"key": "value"}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	req := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{
			Name:      "test-tool",
			Arguments: argsJSON,
		},
	}

	result, err := handler(ctx, req)

	require.NoError(t, err, "handler should return nil error on success")
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "success output", textContent.Text)
}

func TestHandlerFor_CallToolError(t *testing.T) {
	provider := &fakeProvider{
		callErr: errors.New("tool execution failed"),
	}

	handler := handlerFor(provider, "test-tool")
	require.NotNil(t, handler)

	ctx := context.Background()
	args := map[string]any{"key": "value"}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	req := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{
			Name:      "test-tool",
			Arguments: argsJSON,
		},
	}

	result, err := handler(ctx, req)

	require.NoError(t, err, "handler should always return nil error (errors wrapped in result)")
	require.NotNil(t, result)
	assert.True(t, result.IsError, "result should have IsError=true")
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "tool execution failed")
}

func TestHandlerFor_NilResult(t *testing.T) {
	// A provider returning (nil, nil) is legal per the interface contract. The handler
	// must surface a structured IsError result instead of panicking on the nil deref.
	provider := &fakeProvider{
		callResult: nil,
		callErr:    nil,
	}

	handler := handlerFor(provider, "test-tool")
	require.NotNil(t, handler)

	ctx := context.Background()
	argsJSON, _ := json.Marshal(map[string]any{"key": "value"})

	req := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{
			Name:      "test-tool",
			Arguments: argsJSON,
		},
	}

	result, err := handler(ctx, req)

	require.NoError(t, err, "handler should always return nil error (errors wrapped in result)")
	require.NotNil(t, result)
	assert.True(t, result.IsError, "nil provider result should map to IsError=true")
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "returned no result")
	assert.NotContains(t, textContent.Text, "panic", "nil result must not go through the panic-recovery path")
}

func TestHandlerFor_NilParams(t *testing.T) {
	// When req.Params is nil the handler must call the provider with nil args rather
	// than dereferencing Params.
	var gotArgs map[string]any
	gotCalled := false
	provider := &recordingProvider{
		onCall: func(_ context.Context, _ string, args map[string]any) (*ports.ToolResult, error) {
			gotCalled = true
			gotArgs = args
			return &ports.ToolResult{Content: []ports.ToolContent{{Type: "text", Text: "ok"}}}, nil
		},
	}

	handler := handlerFor(provider, "test-tool")
	require.NotNil(t, handler)

	result, err := handler(context.Background(), &sdkmcp.CallToolRequest{Params: nil})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.True(t, gotCalled, "provider should be invoked even when Params is nil")
	assert.Nil(t, gotArgs, "args should be nil when no params are provided")
}

func TestHandlerFor_MalformedJSONArgs(t *testing.T) {
	// Malformed JSON arguments must fall back to an empty map so the provider can run
	// and return a structured result, rather than aborting the request at the transport.
	var gotArgs map[string]any
	provider := &recordingProvider{
		onCall: func(_ context.Context, _ string, args map[string]any) (*ports.ToolResult, error) {
			gotArgs = args
			return &ports.ToolResult{Content: []ports.ToolContent{{Type: "text", Text: "ok"}}}, nil
		},
	}

	handler := handlerFor(provider, "test-tool")
	require.NotNil(t, handler)

	req := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{
			Name:      "test-tool",
			Arguments: []byte(`{"key": invalid}`),
		},
	}

	result, err := handler(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	require.NotNil(t, gotArgs, "malformed JSON should yield an empty (non-nil) map")
	assert.Empty(t, gotArgs, "malformed JSON args should produce an empty map")
}

func TestHandlerFor_PanicRecovery(t *testing.T) {
	provider := &fakeProvider{
		callPanic: true,
	}

	handler := handlerFor(provider, "test-tool")
	require.NotNil(t, handler)

	ctx := context.Background()
	args := map[string]any{"key": "value"}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	req := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{
			Name:      "test-tool",
			Arguments: argsJSON,
		},
	}

	result, err := handler(ctx, req)

	require.NoError(t, err, "handler should not propagate panic as error")
	require.NotNil(t, result)
	assert.True(t, result.IsError, "panic should result in IsError=true")
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "panic recovered")
}

func TestHandlerFor_PanicRecoveryDoesNotRepanic(t *testing.T) {
	provider := &fakeProvider{
		callResult: &ports.ToolResult{
			IsError: false,
			Content: []ports.ToolContent{
				{Type: "text", Text: "success"},
			},
		},
	}

	handler := handlerFor(provider, "test-tool")
	require.NotNil(t, handler)

	ctx := context.Background()
	argsJSON, _ := json.Marshal(map[string]any{})

	// First call panics
	provider.callPanic = true
	panicReq := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{
			Name:      "test-tool",
			Arguments: argsJSON,
		},
	}
	panicResult, panicErr := handler(ctx, panicReq)
	require.NoError(t, panicErr)
	require.NotNil(t, panicResult)
	assert.True(t, panicResult.IsError)

	// Second call succeeds (proves panic didn't leave handler in bad state)
	provider.callPanic = false
	successReq := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{
			Name:      "test-tool",
			Arguments: argsJSON,
		},
	}
	successResult, successErr := handler(ctx, successReq)
	require.NoError(t, successErr)
	require.NotNil(t, successResult)
	assert.False(t, successResult.IsError, "subsequent call should succeed after panic recovery")
	require.Len(t, successResult.Content, 1)
	textContent, ok := successResult.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "success", textContent.Text)
}

func TestHandlerFor_ClosureCapturesTool(t *testing.T) {
	provider := &fakeProvider{
		callResult: &ports.ToolResult{
			IsError: false,
			Content: []ports.ToolContent{
				{Type: "text", Text: "result"},
			},
		},
	}

	toolName := "my-captured-tool"
	handler := handlerFor(provider, toolName)
	require.NotNil(t, handler)

	ctx := context.Background()
	argsJSON, _ := json.Marshal(map[string]any{})

	req := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{
			Name:      toolName,
			Arguments: argsJSON,
		},
	}

	result, err := handler(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
}
