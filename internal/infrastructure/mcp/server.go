package mcp

import (
	"context"
	"fmt"
	"io"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/awf-project/cli/internal/domain/ports"
)

const serverName = "awf-mcp-server"

// Server wraps *mcp.Server with the two-method public surface required by the MCP serve command.
type Server struct {
	srv   *sdkmcp.Server
	names map[string]struct{}
}

// New returns a Server with an empty tool registry. version is passed to the SDK implementation.
func New(version string) *Server {
	srv := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: serverName, Version: version},
		nil,
	)
	return &Server{
		srv:   srv,
		names: make(map[string]struct{}),
	}
}

// RegisterProvider iterates p.ListTools, deduplicates by name, and registers each tool on the SDK server.
//
// Registration is atomic: all tool names are validated for collisions (against the
// existing registry AND within the provider's own list) BEFORE any tool is added.
// The SDK exposes no RemoveTool, so a mid-loop failure would otherwise leave the
// first K-1 tools permanently registered with a misleading error — this pre-pass
// guarantees an all-or-nothing outcome.
func (s *Server) RegisterProvider(p ports.ToolProvider) error {
	ctx := context.Background()
	tools, err := p.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}
	// Validation pass: detect duplicates without mutating any state — both against the
	// existing registry (s.names) AND within this provider's own list (seen).
	seen := make(map[string]struct{}, len(tools))
	for i := range tools {
		name := tools[i].Name
		if _, exists := s.names[name]; exists {
			return fmt.Errorf("register tool %q: tool already registered", name)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("register tool %q: tool already registered", name)
		}
		seen[name] = struct{}{}
	}
	// Commit pass: state is only touched once every name is known-unique.
	for i := range tools {
		name := tools[i].Name
		s.names[name] = struct{}{}
		s.srv.AddTool(toolToMCP(&tools[i]), handlerFor(p, name))
	}
	return nil
}

// ServeStdio drives the SDK's StdioTransport until ctx is cancelled or the connection closes.
func (s *Server) ServeStdio(ctx context.Context) error {
	return s.serve(ctx, &sdkmcp.StdioTransport{})
}

// ServeIO drives the SDK server over the provided reader/writer closers. Intended for testing.
func (s *Server) ServeIO(ctx context.Context, r io.ReadCloser, w io.WriteCloser) error {
	return s.serve(ctx, &sdkmcp.IOTransport{Reader: r, Writer: w})
}

func (s *Server) serve(ctx context.Context, t sdkmcp.Transport) error {
	if err := s.srv.Run(ctx, t); err != nil {
		return fmt.Errorf("mcp serve: %w", err)
	}
	return nil
}
