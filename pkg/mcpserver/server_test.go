package mcpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/mcpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingReader is an io.Reader that blocks until the done channel is closed.
// Used to simulate an idle stdin that never delivers another line, so we can
// test that context cancellation unblocks Serve without requiring stdin to close.
type blockingReader struct {
	done chan struct{}
	once sync.Once
	buf  []byte // initial data to return on the first read
}

func newBlockingReader(initial string) *blockingReader {
	return &blockingReader{done: make(chan struct{}), buf: []byte(initial)}
}

func (r *blockingReader) Close() {
	r.once.Do(func() { close(r.done) })
}

func (r *blockingReader) Read(p []byte) (int, error) {
	if len(r.buf) > 0 {
		n := copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}
	<-r.done
	return 0, io.EOF
}

// serveSync runs srv.Serve in a goroutine and blocks until it returns.
// This establishes the formal happens-before relationship required by the race detector.
func serveSync(ctx context.Context, srv *mcpserver.Server, stdin *strings.Reader, stdout *bytes.Buffer) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = srv.Serve(ctx, stdin, stdout)
	})
	wg.Wait()
}

func TestNew_ReturnsServer(t *testing.T) {
	srv := mcpserver.New()
	require.NotNil(t, srv, "New should return a non-nil server")
}

func TestRegisterTool_StoresToolDefinition(t *testing.T) {
	srv := mcpserver.New()
	schema := mcpserver.InputSchema{
		Type: "object",
		Properties: map[string]any{
			"name": map[string]string{"type": "string"},
		},
	}
	handler := func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		return mcpserver.Result{
			Content: []mcpserver.ContentBlock{{Type: "text", Text: "ok"}},
		}, nil
	}

	require.NoError(t, srv.RegisterTool(mcpserver.ToolDefinition{Name: "test_tool", InputSchema: schema}, handler))

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	var resp mcpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err)

	result := resp.Result.(map[string]any)
	require.NotNil(t, result, "tools/list result should not be nil")
	tools := result["tools"].([]any)
	require.Len(t, tools, 1, "should have exactly 1 registered tool")
	tool := tools[0].(map[string]any)
	assert.Equal(t, "test_tool", tool["name"], "tool name should match registered name")
}

func TestRegisterTool_ErrorOnDuplicate(t *testing.T) {
	srv := mcpserver.New()
	schema := mcpserver.InputSchema{Type: "object"}
	handler := func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		return mcpserver.Result{}, nil
	}

	require.NoError(t, srv.RegisterTool(mcpserver.ToolDefinition{Name: "my_tool", InputSchema: schema}, handler),
		"first registration should succeed")

	err := srv.RegisterTool(mcpserver.ToolDefinition{Name: "my_tool", InputSchema: schema}, handler)
	require.Error(t, err, "duplicate tool registration should return an error")
	assert.ErrorContains(t, err, "my_tool", "error should mention the duplicate tool name")
}

func TestServe_HandlesInitializeRequest(t *testing.T) {
	srv := mcpserver.New()
	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	var resp mcpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err, "response should be valid JSON")

	require.Nil(t, resp.Error, "initialize should not return an error")
	assert.Equal(t, json.RawMessage("1"), resp.ID, "response ID should match request ID")

	result := resp.Result.(map[string]any)
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "2024-11-05", result["protocolVersion"], "protocol version should match MCP spec")
	assert.NotNil(t, result["serverInfo"], "serverInfo should be present")
	assert.NotNil(t, result["capabilities"], "capabilities should be present")
}

func TestServe_AcceptsInitializedNotification(t *testing.T) {
	srv := mcpserver.New()
	stdin := strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	assert.Empty(t, stdout.String(), "notifications/initialized notification should not produce a response")
}

func TestServe_SilentlyDropsArbitraryNotifications(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{"initialized", "notifications/initialized"},
		{"cancelled", "notifications/cancelled"},
		{"progress", "notifications/progress"},
		{"unknown", "notifications/unknownX"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := mcpserver.New()
			stdin := strings.NewReader(`{"jsonrpc":"2.0","method":"` + tt.method + `"}`)
			stdout := new(bytes.Buffer)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			serveSync(ctx, srv, stdin, stdout)

			assert.Empty(t, stdout.String(), "notification %q must not produce any response", tt.method)
		})
	}
}

func TestServe_HandlesToolsListRequest(t *testing.T) {
	srv := mcpserver.New()
	schema := mcpserver.InputSchema{Type: "object"}
	handler := func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		return mcpserver.Result{}, nil
	}

	require.NoError(t, srv.RegisterTool(mcpserver.ToolDefinition{Name: "tool1", InputSchema: schema}, handler))
	require.NoError(t, srv.RegisterTool(mcpserver.ToolDefinition{Name: "tool2", InputSchema: schema}, handler))

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	var resp mcpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err, "response should be valid JSON")

	require.Nil(t, resp.Error, "tools/list should not return an error")
	result := resp.Result.(map[string]any)
	require.NotNil(t, result, "result should not be nil")
	tools := result["tools"].([]any)
	require.Len(t, tools, 2, "should list exactly 2 registered tools")
}

func TestServe_HandlesToolsCallWithValidTool(t *testing.T) {
	srv := mcpserver.New()
	schema := mcpserver.InputSchema{Type: "object"}
	handler := func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		return mcpserver.Result{
			Content: []mcpserver.ContentBlock{{Type: "text", Text: "tool result"}},
			IsError: false,
		}, nil
	}

	require.NoError(t, srv.RegisterTool(mcpserver.ToolDefinition{Name: "my_tool", InputSchema: schema}, handler))

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"my_tool","arguments":{}}}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	var resp mcpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err, "response should be valid JSON")

	require.Nil(t, resp.Error, "tools/call with valid tool should not return an error")
	result := resp.Result.(map[string]any)
	require.NotNil(t, result, "result should not be nil")
	require.False(t, result["isError"].(bool), "isError should be false for successful call")
	require.NotNil(t, result["content"], "content should not be nil")
	content := result["content"].([]any)
	require.NotEmpty(t, content, "content should not be empty")
	assert.Equal(t, "tool result", content[0].(map[string]any)["text"], "content should match handler result")
}

func TestServe_HandlesToolsCallWithUnknownTool(t *testing.T) {
	srv := mcpserver.New()

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"unknown_tool","arguments":{}}}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	var resp mcpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err)

	require.NotNil(t, resp.Error, "expected error response for unknown tool")
	assert.Equal(t, mcpserver.ErrCodeMethodNotFound, resp.Error.Code, "expected method not found error code")
	assert.Contains(t, resp.Error.Message, "unknown tool", "expected error message to mention unknown tool")
}

func TestServe_HandlesToolsCallWithHandlerError(t *testing.T) {
	srv := mcpserver.New()
	schema := mcpserver.InputSchema{Type: "object"}
	handler := func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		return mcpserver.Result{}, fmt.Errorf("tool execution failed")
	}

	require.NoError(t, srv.RegisterTool(mcpserver.ToolDefinition{Name: "failing_tool", InputSchema: schema}, handler))

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"failing_tool","arguments":{}}}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	var resp mcpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err)

	assert.Nil(t, resp.Error, "expected no JSON-RPC error; handler error should be wrapped in content")
	result := resp.Result.(map[string]any)
	require.True(t, result["isError"].(bool), "isError should be true when handler returns error")

	content := result["content"].([]any)
	require.NotEmpty(t, content, "error content should not be empty")
	contentBlock := content[0].(map[string]any)
	assert.Equal(t, "tool execution failed", contentBlock["text"], "error text should match handler error message")
}

func TestServe_HandlesShutdownRequest(t *testing.T) {
	srv := mcpserver.New()

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"shutdown"}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := srv.Serve(ctx, stdin, stdout)

	require.NoError(t, err, "Serve should return nil after shutdown request")

	var resp mcpserver.Response
	dec := json.NewDecoder(stdout)
	errDec := dec.Decode(&resp)
	require.NoError(t, errDec, "response should be valid JSON")

	assert.Nil(t, resp.Error, "shutdown response should have no error")
	assert.Equal(t, json.RawMessage("1"), resp.ID, "response ID should match request ID")
}

func TestServe_ReturnsContextErrorWhenCanceled(t *testing.T) {
	srv := mcpserver.New()

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := srv.Serve(ctx, stdin, stdout)

	require.NotNil(t, err, "Serve should return error when context is canceled")
	assert.ErrorIs(t, err, context.Canceled, "error should be context.Canceled")
}

func TestServe_HandlesMalformedJSON(t *testing.T) {
	srv := mcpserver.New()

	stdin := strings.NewReader(`{invalid json`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	var resp mcpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err)

	require.NotNil(t, resp.Error, "expected error response for malformed JSON")
	assert.Equal(t, mcpserver.ErrCodeParseError, resp.Error.Code, "expected parse error code -32700")
	assert.Equal(t, "Parse error", resp.Error.Message, "expected parse error message")
}

// TestServer_ParseError_HasExplicitNullID verifies that the ParseError response
// emits "id":null explicitly, as required by JSON-RPC 2.0 §5.1. Without this,
// a strict client that validates the presence of the id field would reject the
// response. The implementation uses json.RawMessage("null") which passes the
// omitempty guard because it is a non-empty byte slice.
func TestServer_ParseError_HasExplicitNullID(t *testing.T) {
	srv := mcpserver.New()

	stdin := strings.NewReader(`{not valid json at all`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	rawOutput := stdout.String()
	require.NotEmpty(t, rawOutput, "server must emit a response for parse errors")

	// Unmarshal into a raw map to check the id field independently of the
	// Response struct's json tags (which might affect how null is decoded).
	var rawResp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(rawOutput)), &rawResp),
		"response must be valid JSON")

	idField, hasID := rawResp["id"]
	require.True(t, hasID, "JSON-RPC 2.0 §5.1: ParseError response MUST include 'id' field")
	assert.Equal(t, json.RawMessage("null"), idField,
		"JSON-RPC 2.0 §5.1: ParseError id MUST be null when request id cannot be determined")
}

// TestServer_ToolHandlerPanic_DoesNotKillServer is a regression test for B2.
//
// Before the fix, a panic inside a tool handler propagated unchecked through
// handleToolsCall → handle → Serve's scanner loop, terminating the whole process.
// This caused every subsequent tool call to fail with "MCP connection closed".
// After the fix, a deferred recover() in handleToolsCall catches the panic,
// logs it, and returns IsError:true so the server remains alive for further calls.
func TestServer_ToolHandlerPanic_DoesNotKillServer(t *testing.T) {
	srv := mcpserver.New()
	schema := mcpserver.InputSchema{Type: "object"}

	// Register a tool whose handler unconditionally panics.
	require.NoError(t, srv.RegisterTool(mcpserver.ToolDefinition{Name: "panicking_tool", InputSchema: schema}, func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		panic("boom")
	}))

	// Register a second tool that succeeds, used to prove the server is still alive.
	require.NoError(t, srv.RegisterTool(mcpserver.ToolDefinition{Name: "healthy_tool", InputSchema: schema}, func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		return mcpserver.Result{
			Content: []mcpserver.ContentBlock{{Type: "text", Text: "still alive"}},
		}, nil
	}))

	// Send two requests: first to the panicking tool, then to the healthy tool.
	const input = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"panicking_tool","arguments":{}}}` +
		"\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"healthy_tool","arguments":{}}}`

	stdin := strings.NewReader(input)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	dec := json.NewDecoder(stdout)

	// First response: panicking_tool must return IsError:true, not a transport error.
	var panicResp mcpserver.Response
	require.NoError(t, dec.Decode(&panicResp), "first response must be valid JSON; server must not have died")
	require.Nil(t, panicResp.Error, "panic must not produce a JSON-RPC level error; it must be wrapped in content")
	panicResult, ok := panicResp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")
	assert.True(t, panicResult["isError"].(bool), "isError must be true when the handler panicked")

	// Second response: healthy_tool must still respond successfully (server is alive).
	var healthyResp mcpserver.Response
	require.NoError(t, dec.Decode(&healthyResp), "second response must be valid JSON; server must still be alive after the panic")
	require.Nil(t, healthyResp.Error, "healthy_tool must not produce a JSON-RPC error")
	healthyResult, ok := healthyResp.Result.(map[string]any)
	require.True(t, ok, "healthy_tool result must be a JSON object")
	assert.False(t, healthyResult["isError"].(bool), "isError must be false for healthy_tool")
	content := healthyResult["content"].([]any)
	require.NotEmpty(t, content, "healthy_tool must return content")
	assert.Equal(t, "still alive", content[0].(map[string]any)["text"], "healthy_tool content must match")
}

// TestRegisterTool_DescriptionAppearsInToolsList asserts that the Description set in
// ToolDefinition is propagated verbatim in the tools/list wire response. This is the
// contract Gemini and other strict agents rely on: an opaque tool with no description
// is refused, causing the agent to fall back to native filesystem tools.
func TestRegisterTool_DescriptionAppearsInToolsList(t *testing.T) {
	srv := mcpserver.New()
	schema := mcpserver.InputSchema{Type: "object"}
	handler := func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		return mcpserver.Result{}, nil
	}

	require.NoError(t, srv.RegisterTool(mcpserver.ToolDefinition{
		Name:        "described_tool",
		Description: "Does something useful. Returns a JSON object with fields: foo, bar.",
		InputSchema: schema,
	}, handler))

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	var resp mcpserver.Response
	require.NoError(t, json.NewDecoder(stdout).Decode(&resp))
	require.Nil(t, resp.Error)

	result := resp.Result.(map[string]any)
	tools := result["tools"].([]any)
	require.Len(t, tools, 1)

	tool := tools[0].(map[string]any)
	assert.Equal(t, "described_tool", tool["name"])
	assert.Equal(t, "Does something useful. Returns a JSON object with fields: foo, bar.", tool["description"],
		"description must be propagated to tools/list wire response")
}

func TestServe_PresservesIsErrorFlag(t *testing.T) {
	srv := mcpserver.New()
	schema := mcpserver.InputSchema{Type: "object"}
	handler := func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		return mcpserver.Result{
			Content: []mcpserver.ContentBlock{{Type: "text", Text: "error occurred"}},
			IsError: true,
		}, nil
	}

	require.NoError(t, srv.RegisterTool(mcpserver.ToolDefinition{Name: "error_tool", InputSchema: schema}, handler))

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"error_tool","arguments":{}}}`)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	serveSync(ctx, srv, stdin, stdout)

	var resp mcpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err, "response should be valid JSON")

	result := resp.Result.(map[string]any)
	require.NotNil(t, result, "result should not be nil")
	require.True(t, result["isError"].(bool), "isError flag should be preserved from handler result")
	assert.Equal(t, "error occurred", result["content"].([]any)[0].(map[string]any)["text"], "error content should match handler result")
}

// TestServe_AcceptsRequestLargerThanScannerDefault is a regression guard for the
// F099 review finding: bufio.NewScanner defaults to 64 KiB per line, which is too
// small for real-world tool_call payloads (base64-encoded files, large diffs,
// multi-page prompts). The server must grow its scan buffer to maxRequestLineBytes
// (~10 MiB) so a large but well-formed request is processed normally instead of
// crashing the stream with bufio.ErrTooLong.
func TestServe_AcceptsRequestLargerThanScannerDefault(t *testing.T) {
	srv := mcpserver.New()
	schema := mcpserver.InputSchema{Type: "object"}
	handler := func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
		return mcpserver.Result{Content: []mcpserver.ContentBlock{{Type: "text", Text: "ok"}}}, nil
	}
	require.NoError(t, srv.RegisterTool(mcpserver.ToolDefinition{Name: "big_tool", InputSchema: schema}, handler))

	// Build a tools/call payload comfortably above bufio.MaxScanTokenSize (64 KiB)
	// without crossing maxRequestLineBytes. 256 KiB exercises the new buffer growth.
	payload := strings.Repeat("a", 256*1024)
	req := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"big_tool","arguments":{"data":%q}}}`, payload)
	stdin := strings.NewReader(req)
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	serveSync(ctx, srv, stdin, stdout)

	var resp mcpserver.Response
	require.NoError(t, json.NewDecoder(stdout).Decode(&resp),
		"large payload must be processed; default 64 KiB scanner would error out here")
	require.Nil(t, resp.Error, "no RPC error expected: %+v", resp.Error)
	result := resp.Result.(map[string]any)
	assert.Equal(t, false, result["isError"])
}

// TestServe_ContextCancellationUnblocksBlockedScan is a regression test for M2:
// before the fix, Serve used a blocking scanner.Scan() call in the main goroutine.
// When stdin had no more data but was not closed (the typical SIGTERM scenario),
// Serve would block indefinitely even after the context was canceled.
//
// After the fix, the scanner runs in a dedicated goroutine; Serve selects on both
// ctx.Done() and the scan channel, so cancellation is observed immediately.
func TestServe_ContextCancellationUnblocksBlockedScan(t *testing.T) {
	srv := mcpserver.New()

	// A blocking reader: delivers one initialize request then blocks forever
	// until explicitly closed — simulating an idle stdin.
	reader := newBlockingReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n")
	stdout := new(bytes.Buffer)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(ctx, reader, stdout)
	}()

	// Wait for the initialize response to arrive so we know Serve is running.
	time.Sleep(50 * time.Millisecond)

	// Cancel the context and expect Serve to return promptly.
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled,
			"Serve must return context.Canceled immediately after cancellation, not block on stdin")
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return within 2 s after context cancellation; stdin goroutine is likely blocked")
	}

	// Allow the blocking reader goroutine to exit.
	reader.Close()
}
