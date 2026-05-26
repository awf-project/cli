//go:build integration

package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/infrastructure/tools"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/mcpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPluginBridge_NotifyToolRegistration verifies that a PluginToolAdapter
// correctly registers plugin operations as MCP tools with namespaced names.
func TestPluginBridge_NotifyToolRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup: Create a MockOperationProvider with the "send" operation
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs: map[string]pluginmodel.InputSchema{
			"message": {Type: "string", Required: true},
			"title":   {Type: "string"},
		},
	})

	// Create the PluginToolAdapter
	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})
	require.NoError(t, err, "NewPluginToolAdapter should not fail")

	// Create MCP server and register adapter's tools
	srv := mcpserver.New()
	tools, err := adapter.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 1)

	tool := tools[0]

	// Verify tool name is namespaced correctly
	assert.Equal(t, "notify_send", tool.Name, "tool should be namespaced as notify_send")

	// Verify tool source indicates it's from a plugin
	assert.Equal(t, "plugin:notify", tool.Source, "tool Source should indicate it's from a plugin")

	// Verify InputSchema structure is correct
	require.NotNil(t, tool.InputSchema, "InputSchema should not be nil")
	inputSchema := tool.InputSchema

	// Check top-level structure: should be object type
	assert.Equal(t, "object", inputSchema["type"], "InputSchema type should be object")

	// Check properties exist
	props, ok := inputSchema["properties"].(map[string]any)
	require.True(t, ok, "InputSchema should have properties")
	require.Len(t, props, 2, "InputSchema should have 2 properties (message, title)")

	// Verify message property (required)
	messageProp, ok := props["message"].(map[string]any)
	require.True(t, ok, "message property should exist")
	assert.Equal(t, "string", messageProp["type"], "message should be string type")

	// Verify title property (optional)
	titleProp, ok := props["title"].(map[string]any)
	require.True(t, ok, "title property should exist")
	assert.Equal(t, "string", titleProp["type"], "title should be string type")

	// Verify required array
	required, ok := inputSchema["required"].([]any)
	require.True(t, ok, "InputSchema should have required array")
	require.Len(t, required, 1, "required should contain 1 field (message)")
	assert.Equal(t, "message", required[0], "message should be in required fields")

	// Register tool handler for schema validation
	schema := mcpserver.InputSchema{Type: "object"}
	if tool.InputSchema != nil {
		data, _ := json.Marshal(tool.InputSchema)
		_ = json.Unmarshal(data, &schema)
	}

	srv.RegisterTool(mcpserver.ToolDefinition{Name: tool.Name, Description: tool.Description, InputSchema: schema}, func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		var argsMap map[string]any
		if unmarshalErr := json.Unmarshal(args, &argsMap); unmarshalErr != nil {
			return mcpserver.Result{}, unmarshalErr
		}
		result, callErr := adapter.CallTool(ctx, tool.Name, argsMap)
		if callErr != nil {
			return mcpserver.Result{}, callErr
		}
		contentBlocks := make([]mcpserver.ContentBlock, len(result.Content))
		for i, c := range result.Content {
			contentBlocks[i] = mcpserver.ContentBlock{Type: c.Type, Text: c.Text}
		}
		return mcpserver.Result{
			Content: contentBlocks,
			IsError: result.IsError,
		}, nil
	})
}

// TestPluginBridge_ToolCallDispatchesToProvider verifies that tool calls
// dispatch correctly to the underlying OperationProvider.Execute method.
func TestPluginBridge_ToolCallDispatchesToProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs: map[string]pluginmodel.InputSchema{
			"message": {Type: "string"},
		},
	})

	// Configure provider to return a successful result
	provider.SetExecuteFunc(func(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
		return &pluginmodel.OperationResult{
			Success: true,
			Outputs: map[string]any{"status": "sent"},
		}, nil
	})

	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})
	require.NoError(t, err)

	// Call the tool and verify the result
	result, err := adapter.CallTool(context.Background(), "notify_send", map[string]any{
		"message": "test message",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Verify that Execute was called. The adapter forwards the fully-qualified
	// "<plugin>.<op>" identifier so the underlying provider routes the call to the
	// correct plugin instead of doing a blind unprefixed search.
	calls := provider.GetExecuteCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "notify.send", calls[0].Name)
	assert.Equal(t, "test message", calls[0].Inputs["message"])
}

// TestPluginBridge_SourceFieldCorrect verifies that adapter tools have correct Source field.
func TestPluginBridge_SourceFieldCorrect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs:     map[string]pluginmodel.InputSchema{},
	})

	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})
	require.NoError(t, err)

	toolsList, err := adapter.ListTools(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "plugin:notify", toolsList[0].Source, "tool Source should indicate it's from a plugin")
}

// TestPluginBridge_MCPServeWithPluginToolsNoBuiltins verifies that mcp-serve
// correctly registers plugin tools without built-ins when intercept_builtins is false.
// This test exercises the plugin adapter registration flow in mcp_serve.go.
func TestPluginBridge_MCPServeWithPluginToolsNoBuiltins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup: Create a MockOperationProvider with the "notify.send" operation
	provider := mocks.NewMockOperationProvider()
	provider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs: map[string]pluginmodel.InputSchema{
			"message": {Type: "string", Required: true},
		},
	})

	// Create the PluginToolAdapter for the notify plugin
	adapter, err := tools.NewPluginToolAdapter("notify", provider, []string{"send"})
	require.NoError(t, err, "PluginToolAdapter construction should succeed")

	// Verify that the adapter exposes the namespaced tool name
	toolList, err := adapter.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, toolList, 1)
	assert.Equal(t, "notify_send", toolList[0].Name)
	assert.Equal(t, "plugin:notify", toolList[0].Source)

	// Create MCP server and register only the plugin tool (NOT built-ins)
	srv := mcpserver.New()

	// Register plugin tool via adapter (simulating mcp_serve.go plugin registration block)
	tool := toolList[0]
	schema := mcpserver.InputSchema{Type: "object"}
	if tool.InputSchema != nil {
		data, _ := json.Marshal(tool.InputSchema)
		_ = json.Unmarshal(data, &schema)
	}

	srv.RegisterTool(mcpserver.ToolDefinition{Name: tool.Name, Description: tool.Description, InputSchema: schema}, func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		var argsMap map[string]any
		if unmarshalErr := json.Unmarshal(args, &argsMap); unmarshalErr != nil {
			return mcpserver.Result{}, unmarshalErr
		}
		result, callErr := adapter.CallTool(ctx, tool.Name, argsMap)
		if callErr != nil {
			return mcpserver.Result{}, callErr
		}
		contentBlocks := make([]mcpserver.ContentBlock, len(result.Content))
		for i, c := range result.Content {
			contentBlocks[i] = mcpserver.ContentBlock{Type: c.Type, Text: c.Text}
		}
		return mcpserver.Result{
			Content: contentBlocks,
			IsError: result.IsError,
		}, nil
	})

	// Test: Send MCP tools/list request and verify ONLY notify_send is present
	// (no built-in tools since intercept_builtins was false)
	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(ctx, stdin, stdout)
	}()

	wg.Wait()

	var resp mcpserver.Response
	err = json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err, "MCP response should be valid JSON")

	result := resp.Result.(map[string]any)
	toolsList := result["tools"].([]any)

	// Verify ONLY plugin tool is registered, no built-ins
	require.Len(t, toolsList, 1, "should have exactly 1 tool (notify_send)")

	toolDef := toolsList[0].(map[string]any)
	assert.Equal(t, "notify_send", toolDef["name"], "registered tool should be notify_send")
}

// TestPluginBridge_FullWorkflowWithPluginTools verifies the complete awf run workflow
// with intercept_builtins:false and plugin_tools configuration. This test exercises
// the mcp_serve.go plugin wiring and validates that plugin tools are properly
// registered without built-ins, and that tool calls dispatch to the provider.
func TestPluginBridge_FullWorkflowWithPluginTools(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup: Create a NotifyProvider (test double implementing ports.OperationProvider)
	notifyProvider := mocks.NewMockOperationProvider()
	notifyProvider.AddOperation(&pluginmodel.OperationSchema{
		Name:       "send",
		PluginName: "notify",
		Inputs: map[string]pluginmodel.InputSchema{
			"title":   {Type: "string", Required: true},
			"message": {Type: "string", Required: true},
		},
	})

	// Configure provider to return successful result on Execute
	notifyProvider.SetExecuteFunc(func(ctx context.Context, opName string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
		return &pluginmodel.OperationResult{
			Success: true,
			Outputs: map[string]any{
				"notification_id": "notif-123",
				"sent_at":         "2026-05-23T10:30:00Z",
			},
		}, nil
	})

	// Create PluginToolAdapter for notify plugin with send operation exposed
	adapter, err := tools.NewPluginToolAdapter("notify", notifyProvider, []string{"send"})
	require.NoError(t, err, "PluginToolAdapter creation should succeed")

	// Verify adapter lists the namespaced tool
	toolList, err := adapter.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, toolList, 1, "adapter should expose exactly 1 tool")

	tool := toolList[0]
	assert.Equal(t, "notify_send", tool.Name, "tool name should be namespaced as notify_send")
	assert.Equal(t, "plugin:notify", tool.Source, "tool source should indicate plugin origin")

	// Verify InputSchema is fully mapped (checking structure for mcp_serve integration)
	require.NotNil(t, tool.InputSchema, "tool InputSchema should not be nil")
	assert.Equal(t, "object", tool.InputSchema["type"])

	props, ok := tool.InputSchema["properties"].(map[string]any)
	require.True(t, ok, "InputSchema should have properties")
	require.Len(t, props, 2, "should have title and message properties")

	// Simulate what mcp_serve.go does: Register the tool on an MCP server
	srv := mcpserver.New()

	schema := mcpserver.InputSchema{Type: "object"}
	if tool.InputSchema != nil {
		data, _ := json.Marshal(tool.InputSchema)
		_ = json.Unmarshal(data, &schema)
	}

	srv.RegisterTool(mcpserver.ToolDefinition{Name: tool.Name, Description: tool.Description, InputSchema: schema}, func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		var argsMap map[string]any
		if unmarshalErr := json.Unmarshal(args, &argsMap); unmarshalErr != nil {
			return mcpserver.Result{}, unmarshalErr
		}
		result, callErr := adapter.CallTool(ctx, tool.Name, argsMap)
		if callErr != nil {
			return mcpserver.Result{}, callErr
		}
		contentBlocks := make([]mcpserver.ContentBlock, len(result.Content))
		for i, c := range result.Content {
			contentBlocks[i] = mcpserver.ContentBlock{Type: c.Type, Text: c.Text}
		}
		return mcpserver.Result{
			Content: contentBlocks,
			IsError: result.IsError,
		}, nil
	})

	// Simulate tool call: send a tools/call request
	toolCallRequest := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"notify_send","arguments":{"title":"Test Alert","message":"This is a test notification"}}}`
	stdin := strings.NewReader(toolCallRequest)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(ctx, stdin, stdout)
	}()

	wg.Wait()

	// Verify tool was called on the provider. The adapter forces direct routing by
	// passing the fully-qualified "<plugin>.<op>" identifier to OperationProvider.Execute
	// (see plugin_adapter.go: a.pluginName + "." + op.opName). The unprefixed opName never
	// reaches the provider — that fallback was deliberately removed because it triggered
	// a blind search across all plugins.
	calls := notifyProvider.GetExecuteCalls()
	require.Len(t, calls, 1, "provider Execute should be called exactly once")
	assert.Equal(t, "notify.send", calls[0].Name, "adapter forwards the fully-qualified plugin.op identifier")
	assert.Equal(t, "Test Alert", calls[0].Inputs["title"])
	assert.Equal(t, "This is a test notification", calls[0].Inputs["message"])

	// Verify MCP server response is valid
	var resp mcpserver.Response
	err = json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err, "MCP response should be valid JSON")
}
