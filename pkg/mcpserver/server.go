package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// scanResult carries one line (or a scan error) from the stdin reader goroutine.
type scanResult struct {
	line []byte
	err  error // non-nil means the scanner stopped (io.EOF represented as nil line + nil err)
}

const (
	serverName    = "awf-mcp-server"
	serverVersion = "0.1.0"

	// maxRequestLineBytes is the per-line ceiling for the JSON-RPC stdin scanner.
	// The bufio.Scanner default (64 KiB) is far too small for legitimate tools/call
	// payloads — agents routinely pass base64-encoded files, large patches, or long
	// prompts as tool arguments. We size it to match the agent providers' response
	// body limit (10 MiB) so neither direction silently truncates.
	maxRequestLineBytes = 10 * 1024 * 1024
)

// Server is a stdio MCP server. Zero value is not valid; use New().
type Server struct {
	mu     sync.RWMutex
	tools  map[string]toolEntry
	logger *slog.Logger
}

// New returns a Server with an empty tool registry.
// The server defaults to slog.Default() for logging; use WithLogger to inject a custom logger.
func New() *Server {
	return &Server{
		tools:  make(map[string]toolEntry),
		logger: slog.Default(),
	}
}

// WithLogger injects a custom slog.Logger into the server.
// If logger is nil, slog.Default() is used instead.
func (s *Server) WithLogger(logger *slog.Logger) *Server {
	if logger == nil {
		s.logger = slog.Default()
	} else {
		s.logger = logger
	}
	return s
}

// RegisterTool registers a tool with its full definition. The Description field is
// propagated verbatim to tools/list responses per the MCP spec, enabling agents
// such as Gemini (which refuse opaque tools) to understand the tool's contract.
// Returns an error if def.Name is already registered.
func (s *Server) RegisterTool(def ToolDefinition, handler ToolHandler) error { //nolint:gocritic // hugeParam: ToolDefinition is a value type; callers construct it inline without allocation, so copying is cheaper than adding indirection to the API
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tools[def.Name]; exists {
		return fmt.Errorf("mcpserver: tool %q already registered", def.Name)
	}

	s.tools[def.Name] = toolEntry{
		definition: def,
		handler:    handler,
	}
	return nil
}

// Serve reads newline-delimited JSON-RPC 2.0 requests from stdin and writes
// responses to stdout until ctx is canceled or a shutdown request is received.
//
// Stdin is consumed in a dedicated goroutine that pushes scan results into a
// buffered channel. The main loop selects on both the context-cancellation
// signal and the channel so that SIGTERM (or any context cancellation) triggers
// a clean exit even when bufio.Scanner.Scan() is blocked waiting for the next
// line. Without this goroutine, cancellation can only be detected between lines,
// which means a long-idle connection stalls shutdown until the next byte arrives.
//
//nolint:gocognit // Complexity is structural: goroutine-select pattern with JSON-RPC dispatch requires nested branches that cannot be split without introducing additional shared state or indirection.
func (s *Server) Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	enc := json.NewEncoder(stdout)

	// scanCh carries lines from the reader goroutine. A buffer of 1 avoids
	// head-of-line blocking: the goroutine can deposit the next scan result
	// while the main loop is still processing the current one.
	scanCh := make(chan scanResult, 1)

	go func() {
		scanner := bufio.NewScanner(stdin)
		// Grow scanner from 64 KiB up to maxRequestLineBytes so large tool_call payloads
		// do not trip bufio.ErrTooLong and abort the whole stream with an opaque error.
		scanner.Buffer(make([]byte, 0, 64*1024), maxRequestLineBytes)
		for scanner.Scan() {
			line := scanner.Bytes()
			// Copy: scanner reuses its internal buffer on the next Scan call.
			copied := make([]byte, len(line))
			copy(copied, line)
			scanCh <- scanResult{line: copied}
		}
		// Scanner stopped: either EOF or an error.
		scanCh <- scanResult{err: scanner.Err()}
	}()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("mcpserver: %w", ctx.Err())

		case sr := <-scanCh:
			if sr.err != nil {
				return fmt.Errorf("mcpserver: %w", sr.err)
			}
			if sr.line == nil {
				// EOF: scanner goroutine sent sentinel with nil line and nil error.
				return nil
			}

			line := sr.line
			if len(line) == 0 {
				continue
			}

			var req Request
			if err := json.Unmarshal(line, &req); err != nil {
				// JSON-RPC 2.0 §5.1: when the request cannot be parsed the id is unknown,
				// so the response MUST use "id": null explicitly (not omit the field).
				// json.RawMessage("null") is a non-empty byte slice and therefore passes
				// the omitempty check on Response.ID, producing the correct wire output.
				if encErr := enc.Encode(Response{
					JSONRPC: "2.0",
					ID:      json.RawMessage("null"),
					Error:   &RPCError{Code: ErrCodeParseError, Message: "Parse error"},
				}); encErr != nil {
					return fmt.Errorf("mcpserver: %w", encErr)
				}
				continue
			}

			// JSON-RPC 2.0: notifications (no ID) MUST NOT receive any response,
			// regardless of method. The MCP spec defines several notification methods
			// (notifications/initialized, notifications/cancelled, notifications/progress, ...);
			// the server silently ignores all of them.
			if req.ID == nil {
				continue
			}

			resp := s.handle(ctx, &req)
			if resp == nil {
				continue
			}

			if err := enc.Encode(resp); err != nil {
				return fmt.Errorf("mcpserver: %w", err)
			}

			if req.Method == MethodShutdown {
				return nil
			}
		}
	}
}

func (s *Server) handle(ctx context.Context, req *Request) *Response {
	base := Response{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case MethodInitialize:
		base.Result = initializeResult{
			ProtocolVersion: ProtocolVersion,
			ServerInfo:      serverInfo{Name: serverName, Version: serverVersion},
			Capabilities:    serverCapabilities{Tools: map[string]any{}},
		}

	case MethodToolsList:
		s.mu.RLock()
		defs := make([]ToolDefinition, 0, len(s.tools))
		for _, e := range s.tools {
			defs = append(defs, e.definition)
		}
		s.mu.RUnlock()
		base.Result = toolsListResult{Tools: defs}

	case MethodToolsCall:
		return s.handleToolsCall(ctx, req, base)

	case MethodShutdown:
		base.Result = struct{}{}

	default:
		base.Error = &RPCError{Code: ErrCodeMethodNotFound, Message: "Method not found"}
	}

	return &base
}

func (s *Server) handleToolsCall(ctx context.Context, req *Request, base Response) (resp *Response) {
	// Recover from panics in tool handlers so a single buggy handler cannot kill
	// the entire MCP server subprocess. The panic is logged to stderr for diagnostics
	// but the stack trace is never forwarded to the agent (information leak risk).
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("tool handler panic recovered", "panic", r)
			base.Result = toolsCallResult{
				IsError: true,
				Content: []ContentBlock{{Type: "text", Text: "tool handler panicked; see server logs"}},
			}
			resp = &base
		}
	}()

	var params toolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		base.Error = &RPCError{Code: ErrCodeInvalidParams, Message: "Invalid params"}
		return &base
	}

	s.mu.RLock()
	entry, ok := s.tools[params.Name]
	s.mu.RUnlock()

	if !ok {
		base.Error = &RPCError{Code: ErrCodeMethodNotFound, Message: fmt.Sprintf("unknown tool: %s", params.Name)}
		return &base
	}

	result, err := entry.handler(ctx, params.Arguments)
	if err != nil {
		base.Result = toolsCallResult{
			IsError: true,
			Content: []ContentBlock{{Type: "text", Text: err.Error()}},
		}
		return &base
	}

	base.Result = toolsCallResult(result)
	return &base
}
