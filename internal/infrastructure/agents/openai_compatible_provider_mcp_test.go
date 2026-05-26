package agents

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubToolRouter is a minimal ToolRouter test double.
//
// It exists because the OpenAI-compatible provider integrates MCP via the
// HTTP-native path (SetToolRouter + buildToolList + dispatchToolCall) instead
// of the mcpInjector hook used by CLI providers, so the standard CLI-injector
// test helpers do not apply here.
type stubToolRouter struct {
	tools       []ports.ToolDefinition
	listErr     error
	callResult  *ports.ToolResult
	callErr     error
	lastCallCtx context.Context
	lastName    string
	lastArgs    map[string]any
}

func (s *stubToolRouter) ListTools(_ context.Context) ([]ports.ToolDefinition, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.tools, nil
}

func (s *stubToolRouter) CallTool(ctx context.Context, name string, args map[string]any) (*ports.ToolResult, error) {
	s.lastCallCtx = ctx
	s.lastName = name
	s.lastArgs = args
	if s.callErr != nil {
		return nil, s.callErr
	}
	return s.callResult, nil
}

// TestOpenAICompatibleSetToolRouter_WiresRouter verifies that SetToolRouter
// installs the dependency that the buildToolList / dispatchToolCall paths read.
func TestOpenAICompatibleSetToolRouter_WiresRouter(t *testing.T) {
	p := NewOpenAICompatibleProvider()
	require.Nil(t, p.toolRouter, "router must start unset")

	r := &stubToolRouter{}
	p.SetToolRouter(r)

	assert.Same(t, r, p.toolRouter, "SetToolRouter must store the provided router")
}

// TestOpenAICompatibleBuildToolList_NilConfig confirms the no-op path when
// no MCP proxy config is present — the HTTP request must omit tools entirely.
func TestOpenAICompatibleBuildToolList_NilConfig(t *testing.T) {
	p := NewOpenAICompatibleProvider()
	p.SetToolRouter(&stubToolRouter{tools: []ports.ToolDefinition{{Name: "x"}}})

	tools, choice, err := p.buildToolList(context.Background(), nil)

	require.NoError(t, err)
	assert.Nil(t, tools)
	assert.Empty(t, choice)
}

// TestOpenAICompatibleBuildToolList_DisabledConfig confirms cfg.Enable=false
// short-circuits before calling the router (parity with CLI providers).
func TestOpenAICompatibleBuildToolList_DisabledConfig(t *testing.T) {
	router := &stubToolRouter{
		tools: []ports.ToolDefinition{{Name: "should_not_be_listed"}},
	}
	p := NewOpenAICompatibleProvider()
	p.SetToolRouter(router)

	cfg := &workflow.MCPProxyConfig{Enable: false}
	tools, choice, err := p.buildToolList(context.Background(), cfg)

	require.NoError(t, err)
	assert.Nil(t, tools)
	assert.Empty(t, choice)
	assert.Empty(t, router.lastName, "ListTools must not be invoked when cfg.Enable=false")
}

// TestOpenAICompatibleBuildToolList_NoRouter confirms that an enabled config
// with no router installed degrades gracefully (no panic, no tools).
func TestOpenAICompatibleBuildToolList_NoRouter(t *testing.T) {
	p := NewOpenAICompatibleProvider()

	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}
	tools, choice, err := p.buildToolList(context.Background(), cfg)

	require.NoError(t, err)
	assert.Nil(t, tools)
	assert.Empty(t, choice)
}

// TestOpenAICompatibleBuildToolList_InterceptBuiltinsTrue lists both plugin
// and builtin tools when intercept_builtins is true.
func TestOpenAICompatibleBuildToolList_InterceptBuiltinsTrue(t *testing.T) {
	router := &stubToolRouter{
		tools: []ports.ToolDefinition{
			{Name: "read", Description: "read file", Source: "builtin"},
			{Name: "github_search", Description: "search GH", Source: "github"},
		},
	}
	p := NewOpenAICompatibleProvider()
	p.SetToolRouter(router)

	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}
	tools, choice, err := p.buildToolList(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, tools, 2)
	assert.Equal(t, "auto", choice)
	assert.Equal(t, "function", tools[0].Type)
	assert.Equal(t, "read", tools[0].Function.Name)
	assert.Equal(t, "github_search", tools[1].Function.Name)
}

// TestOpenAICompatibleBuildToolList_InterceptBuiltinsFalse filters out
// source=="builtin" tools so the model only sees plugin-sourced tools.
func TestOpenAICompatibleBuildToolList_InterceptBuiltinsFalse(t *testing.T) {
	router := &stubToolRouter{
		tools: []ports.ToolDefinition{
			{Name: "read", Description: "read file", Source: "builtin"},
			{Name: "github_search", Description: "search GH", Source: "github"},
		},
	}
	p := NewOpenAICompatibleProvider()
	p.SetToolRouter(router)

	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
	tools, choice, err := p.buildToolList(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, tools, 1, "builtin source must be filtered out")
	assert.Equal(t, "github_search", tools[0].Function.Name)
	assert.Equal(t, "auto", choice)
}

// TestOpenAICompatibleBuildToolList_AllBuiltinsFilteredOut covers the
// edge case where filtering leaves zero tools — tool_choice must be empty
// so the Chat Completions request does not advertise an empty tools array.
func TestOpenAICompatibleBuildToolList_AllBuiltinsFilteredOut(t *testing.T) {
	router := &stubToolRouter{
		tools: []ports.ToolDefinition{
			{Name: "read", Source: "builtin"},
			{Name: "write", Source: "builtin"},
		},
	}
	p := NewOpenAICompatibleProvider()
	p.SetToolRouter(router)

	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
	tools, choice, err := p.buildToolList(context.Background(), cfg)

	require.NoError(t, err)
	assert.Nil(t, tools)
	assert.Empty(t, choice)
}

// TestOpenAICompatibleBuildToolList_PropagatesInputSchema verifies that
// the provider attaches the ports.ToolDefinition.InputSchema as the
// `function.parameters` of the Chat Completions tool entry.
func TestOpenAICompatibleBuildToolList_PropagatesInputSchema(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string"},
		},
		"required": []any{"path"},
	}
	router := &stubToolRouter{
		tools: []ports.ToolDefinition{
			{Name: "read", InputSchema: schema, Source: "builtin"},
		},
	}
	p := NewOpenAICompatibleProvider()
	p.SetToolRouter(router)

	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}
	tools, _, err := p.buildToolList(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, schema, tools[0].Function.Parameters)
}

// TestOpenAICompatibleBuildToolList_RouterError surfaces ListTools errors
// to the caller wrapped with provider context (so logs can identify origin).
func TestOpenAICompatibleBuildToolList_RouterError(t *testing.T) {
	router := &stubToolRouter{listErr: errors.New("router exploded")}
	p := NewOpenAICompatibleProvider()
	p.SetToolRouter(router)

	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}
	tools, choice, err := p.buildToolList(context.Background(), cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "openai_compatible")
	assert.Contains(t, err.Error(), "router exploded")
	assert.Nil(t, tools)
	assert.Empty(t, choice)
}

// TestOpenAICompatibleDispatchToolCall_Success routes a model-emitted tool
// call through the router and concatenates content parts.
func TestOpenAICompatibleDispatchToolCall_Success(t *testing.T) {
	router := &stubToolRouter{
		callResult: &ports.ToolResult{
			Content: []ports.ToolContent{
				{Type: "text", Text: "hello"},
				{Type: "text", Text: "world"},
			},
		},
	}
	p := NewOpenAICompatibleProvider()
	p.SetToolRouter(router)

	args, _ := json.Marshal(map[string]any{"q": "x"})
	tc := chatToolCall{ID: "call_1"}
	tc.Function.Name = "github_search"
	tc.Function.Arguments = string(args)

	out, err := p.dispatchToolCall(context.Background(), tc)

	require.NoError(t, err)
	assert.Equal(t, "hello\nworld", out)
	assert.Equal(t, "github_search", router.lastName)
	assert.Equal(t, "x", router.lastArgs["q"])
}

// TestOpenAICompatibleDispatchToolCall_IsErrorResult formats router IsError
// results with the documented `error: ` prefix so the model can detect failure.
func TestOpenAICompatibleDispatchToolCall_IsErrorResult(t *testing.T) {
	router := &stubToolRouter{
		callResult: &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "boom"}},
			IsError: true,
		},
	}
	p := NewOpenAICompatibleProvider()
	p.SetToolRouter(router)

	tc := chatToolCall{ID: "call_2"}
	tc.Function.Name = "github_search"
	tc.Function.Arguments = `{}`

	out, err := p.dispatchToolCall(context.Background(), tc)

	require.NoError(t, err, "IsError is conveyed via content, not as a Go error")
	assert.Equal(t, "error: boom", out)
}

// TestOpenAICompatibleDispatchToolCall_InvalidArguments returns a useful
// content string to the model (so it can self-correct) AND surfaces the
// parse error to the caller for logging.
func TestOpenAICompatibleDispatchToolCall_InvalidArguments(t *testing.T) {
	p := NewOpenAICompatibleProvider()
	p.SetToolRouter(&stubToolRouter{})

	tc := chatToolCall{ID: "call_3"}
	tc.Function.Name = "github_search"
	tc.Function.Arguments = `{this is not json`

	out, err := p.dispatchToolCall(context.Background(), tc)

	require.Error(t, err)
	assert.Contains(t, out, "error: invalid tool arguments for github_search")
}

// TestOpenAICompatibleDispatchToolCall_NoRouter guards the "router never
// installed" path so a misconfigured caller gets a clear error rather than
// a nil deref.
func TestOpenAICompatibleDispatchToolCall_NoRouter(t *testing.T) {
	p := NewOpenAICompatibleProvider()

	tc := chatToolCall{ID: "call_4"}
	tc.Function.Name = "github_search"
	tc.Function.Arguments = `{}`

	out, err := p.dispatchToolCall(context.Background(), tc)

	require.Error(t, err)
	assert.Contains(t, out, "no tool router")
}

// TestOpenAICompatibleDispatchToolCall_RouterError surfaces upstream router
// errors via the returned error AND still produces a content string for the
// model so the multi-turn loop can recover.
func TestOpenAICompatibleDispatchToolCall_RouterError(t *testing.T) {
	router := &stubToolRouter{callErr: errors.New("network down")}
	p := NewOpenAICompatibleProvider()
	p.SetToolRouter(router)

	tc := chatToolCall{ID: "call_5"}
	tc.Function.Name = "github_search"
	tc.Function.Arguments = `{}`

	out, err := p.dispatchToolCall(context.Background(), tc)

	require.Error(t, err)
	assert.Contains(t, out, "error calling tool github_search")
	assert.Contains(t, out, "network down")
}

// TestOpenAICompatibleExecuteConversation_ToolCallLoop verifies that
// ExecuteConversation dispatches tool_calls (MCP) and loops until stop,
// matching the behavior of Execute. Before this fix, MCP was silently
// inactive in conversation mode because the single-shot call path never
// checked finish_reason.
func TestOpenAICompatibleExecuteConversation_ToolCallLoop(t *testing.T) {
	// callCount tracks how many requests the fake server receives:
	// turn 0 → finish_reason=tool_calls, turn 1 → finish_reason=stop.
	var callCount atomic.Int32

	router := &stubToolRouter{
		tools: []ports.ToolDefinition{
			{Name: "echo_tool", Description: "echo", Source: "test"},
		},
		callResult: &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "tool result content"}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		var resp map[string]any
		if n == 1 {
			// First call: return tool_calls finish_reason.
			resp = map[string]any{
				"id":     "chatcmpl-conv-1",
				"object": "chat.completion",
				"choices": []map[string]any{
					{
						"index": 0,
						"message": map[string]any{
							"role":    "assistant",
							"content": "",
							"tool_calls": []map[string]any{
								{
									"id":   "call_conv_1",
									"type": "function",
									"function": map[string]any{
										"name":      "echo_tool",
										"arguments": `{"input":"hello"}`,
									},
								},
							},
						},
						"finish_reason": "tool_calls",
					},
				},
				"usage": map[string]any{
					"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15,
				},
			}
		} else {
			// Second call: model has seen the tool result, returns stop.
			resp = map[string]any{
				"id":     "chatcmpl-conv-2",
				"object": "chat.completion",
				"choices": []map[string]any{
					{
						"index": 0,
						"message": map[string]any{
							"role":    "assistant",
							"content": "final answer after tool",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]any{
					"prompt_tokens": 20, "completion_tokens": 8, "total_tokens": 28,
				},
			}
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	p := NewOpenAICompatibleProvider(WithHTTPClient(httpx.NewClient(httpx.WithDoer(srv.Client()))))
	p.SetToolRouter(router)

	state := workflow.NewConversationState("system prompt")
	options := map[string]any{
		"base_url": srv.URL + "/v1",
		"model":    "test-model",
		workflow.MCPProxyConfigKey: &workflow.MCPProxyConfig{
			Enable:            true,
			InterceptBuiltins: true,
		},
	}

	result, err := p.ExecuteConversation(context.Background(), state, "run the tool", options, nil, nil)

	require.NoError(t, err, "ExecuteConversation must not error when tool loop resolves cleanly")
	require.NotNil(t, result)

	// The server must have been called exactly twice: once to get tool_calls, once after dispatching.
	assert.Equal(t, int32(2), callCount.Load(), "server must be called twice: tool_calls turn + stop turn")

	// The router must have received exactly one CallTool invocation.
	assert.Equal(t, "echo_tool", router.lastName, "tool router must have dispatched the tool call")

	// The final output must come from the stop turn, not the empty tool_calls turn.
	assert.Equal(t, "final answer after tool", result.Output,
		"output must be the assistant content from the stop turn")
}
