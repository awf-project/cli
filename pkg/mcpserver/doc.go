// Package mcpserver implements a reusable MCP (Model Context Protocol) server
// over stdio using JSON-RPC 2.0. It exposes a minimal subset of the MCP
// 2024-11-05 specification: initialize, initialized, tools/list, tools/call,
// and shutdown. Prompts, resources, sampling, and notifications/progress are
// explicitly out of scope.
//
// # Stability and Layering
//
// This package lives under pkg/ and MUST have zero imports from
// github.com/awf-project/cli/internal/. This invariant is enforced by the
// architecture_test.go AST scan included in this package. External consumers
// can embed a Server without pulling in any internal AWF dependency.
//
// Because the package is public, any breaking change here is a SemVer break for
// the whole module. The exported surface is intentionally small: New, Server.RegisterTool,
// Server.Serve, plus the data types ToolDefinition, ToolHandler, InputSchema, Result,
// ContentBlock, Request, Response, and RPCError. The wire-protocol method-name and
// error-code constants live in protocol.go.
//
// # Concurrency Model
//
// A single Server processes requests sequentially: Serve reads one newline-delimited
// JSON-RPC frame at a time, dispatches it, and writes the response before reading the
// next frame. The tool registry (tools map) is guarded by an RWMutex so RegisterTool
// is safe to call from other goroutines, but the canonical pattern is to register all
// tools before calling Serve. Tool handlers themselves run on the same goroutine as
// Serve — long-running handlers therefore block subsequent requests on the same stream.
// Callers that need parallel handler execution should spawn their own goroutine inside
// the handler and respond from there.
//
// # Resilience
//
// Tool handler panics are recovered in handleToolsCall: the panic value is logged to
// stderr (never to stdout, which carries the JSON-RPC stream) and the offending request
// returns a generic "tool handler panicked" Result with IsError:true. The server stays
// alive. Stack traces are never forwarded to the agent because they can leak file paths,
// internal type names, and other implementation detail useful for prompt-injection
// reconnaissance.
//
// # Buffer Sizing
//
// The stdin scanner is grown to maxRequestLineBytes (10 MiB) at startup. The bufio.Scanner
// default of 64 KiB is too small for legitimate tool_call payloads such as base64-encoded
// files or large diffs, and silently emits bufio.ErrTooLong on overflow. The 10 MiB cap
// matches the agent providers' response body limit so neither direction truncates.
//
// # Duplicate Tool Registration
//
// Calling RegisterTool with a name that is already registered returns an error.
// Tools are expected to be registered once at startup, before Serve is called.
// Returning an error instead of panicking allows the caller to propagate the
// failure gracefully (e.g., as a startup error in mcp-serve) without crashing
// the whole process silently in a subprocess.
//
// # Error Codes
//
// The package exposes the standard JSON-RPC 2.0 error codes (ErrCodeParseError,
// ErrCodeInvalidRequest, ErrCodeMethodNotFound, ErrCodeInvalidParams, ErrCodeInternalError).
// Method-not-found is also used when tools/call references an unregistered tool name,
// matching the MCP convention.
//
// # Threat Model
//
// The MCP server is designed to run as a trusted local subprocess (mcp-serve) that
// communicates with an AI agent over stdio. Threat scenarios considered:
//
//   - Prompt injection: An agent may be tricked into passing attacker-controlled
//     values as tool arguments. Tool handlers must not trust argument values without
//     validation. The builtins package validates required fields and resolves paths
//     against a rootDir sandbox.
//   - Tool call flooding: Agents running in a tight loop can issue many tool calls per
//     second. Tool handlers that perform expensive I/O (large file reads, grep over many
//     files) must enforce their own caps (MaxReadBytes, MaxGrepLines) to prevent OOM.
//   - Information exfiltration via errors: Tool handler panics are caught and returned
//     as generic error messages. Internal stack traces are never forwarded to the agent.
//   - Tool name collisions: RegisterTool returns an error on duplicate names so
//     operator errors (two plugins registering the same tool) are caught at startup
//     and surfaced to the caller, not silently overridden at runtime.
//
// # Integration with mcp-serve
//
// The AWF CLI command `awf mcp-serve --config=<path>` reads an on-disk config
// (written by ProxyService.StartForStdio), instantiates a mcpserver.Server, registers
// built-in tools and/or plugin adapters according to the config, and then calls
// srv.Serve(ctx, os.Stdin, os.Stdout). The server exits when stdin closes, the parent
// context is cancelled, or the agent sends "shutdown". ProxyService.StartForHTTP follows
// the same pattern in-process for OpenAI-compatible transports.
//
// # Usage
//
//	srv := mcpserver.New()
//	if err := srv.RegisterTool(mcpserver.ToolDefinition{
//	    Name:        "my_tool",
//	    Description: "Does something useful. Returns a JSON object with fields: result.",
//	    InputSchema: mcpserver.InputSchema{
//	        Type:       "object",
//	        Properties: map[string]mcpserver.PropertySchema{
//	            "input": {Type: "string", Description: "The input value."},
//	        },
//	        Required: []string{"input"},
//	    },
//	}, func(ctx context.Context, args json.RawMessage) (mcpserver.Result, error) {
//	    var params struct{ Input string `json:"input"` }
//	    if err := json.Unmarshal(args, &params); err != nil {
//	        return mcpserver.Result{}, err
//	    }
//	    return mcpserver.Result{Content: []mcpserver.ContentBlock{{Type: "text", Text: "ok"}}}, nil
//	}); err != nil {
//	    log.Fatal(err)
//	}
//	if err := srv.Serve(ctx, os.Stdin, os.Stdout); err != nil {
//	    log.Fatal(err)
//	}
package mcpserver
