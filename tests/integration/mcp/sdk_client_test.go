//go:build integration

// Feature: F104
package mcp_test

import (
	"context"
	"errors"
	"net"
	"sort"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	inframcp "github.com/awf-project/cli/internal/infrastructure/mcp"
)

// scriptedProvider is a real ports.ToolProvider exercised end-to-end through the SDK client.
// callFn is dispatched per tool name so a single provider exposes a mix of passing,
// error-returning, and panicking tools — the shape US3 acceptance tests require.
type scriptedProvider struct {
	tools  []ports.ToolDefinition
	callFn func(name string, args map[string]any) (*ports.ToolResult, error)
}

func (s *scriptedProvider) ListTools(_ context.Context) ([]ports.ToolDefinition, error) {
	return s.tools, nil
}

func (s *scriptedProvider) CallTool(_ context.Context, name string, args map[string]any) (*ports.ToolResult, error) {
	return s.callFn(name, args)
}

func (*scriptedProvider) Close(_ context.Context) error { return nil }

// startServerWithClient wires the new infrastructure adapter to a net.Pipe pair, runs the
// server via ServeIO in a goroutine, and connects a real SDK client to the other end.
// All resources are released through t.Cleanup.
func startServerWithClient(t *testing.T, provider ports.ToolProvider) *sdkmcp.ClientSession {
	t.Helper()

	srv := inframcp.New("test-version")
	require.NoError(t, srv.RegisterProvider(provider))

	serverConn, clientConn := net.Pipe()

	ctx, cancel := context.WithCancel(context.Background())

	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		// net.Conn satisfies io.ReadCloser and io.WriteCloser; same conn handles both directions.
		_ = srv.ServeIO(ctx, serverConn, serverConn)
	}()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v1.0.0"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.IOTransport{Reader: clientConn, Writer: clientConn}, nil)
	require.NoError(t, err, "client must connect over net.Pipe")

	t.Cleanup(func() {
		// Signal shutdown intent first, then drain the transports, then wait. Cancelling
		// the context before closing the conns guarantees the serve goroutine observes
		// cancellation rather than racing on a transport-close error.
		cancel()
		_ = session.Close()
		_ = clientConn.Close()
		_ = serverConn.Close()
		select {
		case <-serveDone:
		case <-time.After(2 * time.Second):
			t.Log("server goroutine did not exit within 2s after cancel")
		}
	})

	return session
}

func TestMCPServer_SDKClient_ListsRegisteredTools(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	provider := &scriptedProvider{
		tools: []ports.ToolDefinition{
			{Name: "alpha", Description: "first tool"},
			{Name: "beta", Description: "second tool"},
		},
		callFn: func(string, map[string]any) (*ports.ToolResult, error) {
			return &ports.ToolResult{Content: []ports.ToolContent{{Type: "text", Text: "ok"}}}, nil
		},
	}

	session := startServerWithClient(t, provider)

	resp, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	names := make([]string, 0, len(resp.Tools))
	for _, tool := range resp.Tools {
		names = append(names, tool.Name)
		assert.NotEmpty(t, tool.Description, "tool %q must propagate description (Gemini rejects opaque tools)", tool.Name)
	}
	sort.Strings(names)
	assert.Equal(t, []string{"alpha", "beta"}, names)
}

func TestMCPServer_SDKClient_CallsToolReturnsTextContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	provider := &scriptedProvider{
		tools: []ports.ToolDefinition{{Name: "echo", Description: "echo input"}},
		callFn: func(_ string, args map[string]any) (*ports.ToolResult, error) {
			text, _ := args["text"].(string)
			return &ports.ToolResult{Content: []ports.ToolContent{{Type: "text", Text: "got: " + text}}}, nil
		},
	}

	session := startServerWithClient(t, provider)

	resp, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"text": "hello F104"},
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError, "passing tool must not flag IsError")
	require.Len(t, resp.Content, 1)
	text, ok := resp.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok, "content[0] must be *TextContent")
	assert.Equal(t, "got: hello F104", text.Text)
}

func TestMCPServer_SDKClient_PanicSurfacesAsIsError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	provider := &scriptedProvider{
		tools: []ports.ToolDefinition{
			{Name: "boom", Description: "panics"},
			{Name: "fail", Description: "returns Go error"},
			{Name: "ok", Description: "succeeds after sibling failures"},
		},
		callFn: func(name string, _ map[string]any) (*ports.ToolResult, error) {
			switch name {
			case "boom":
				panic("synthetic panic for F104 isolation test")
			case "fail":
				return nil, errors.New("provider rejected the call")
			default:
				return &ports.ToolResult{Content: []ports.ToolContent{{Type: "text", Text: "still alive"}}}, nil
			}
		},
	}

	session := startServerWithClient(t, provider)
	ctx := context.Background()

	panicResp, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "boom"})
	require.NoError(t, err, "panic must not surface as JSON-RPC transport error (US1 AC3)")
	require.True(t, panicResp.IsError, "panicking handler must produce IsError=true")

	errResp, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "fail"})
	require.NoError(t, err, "handler error must not surface as JSON-RPC error")
	require.True(t, errResp.IsError, "handler-returned error must produce IsError=true")

	okResp, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "ok"})
	require.NoError(t, err, "server MUST remain alive after panic (NFR-003)")
	assert.False(t, okResp.IsError, "subsequent call after panic must succeed")
	require.Len(t, okResp.Content, 1)
	text, ok := okResp.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "still alive", text.Text)
}
