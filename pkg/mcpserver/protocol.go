package mcpserver

import "encoding/json"

const (
	MethodInitialize  = "initialize"
	MethodInitialized = "notifications/initialized"
	MethodToolsList   = "tools/list"
	MethodToolsCall   = "tools/call"
	MethodShutdown    = "shutdown"

	ProtocolVersion = "2024-11-05"

	// JSON-RPC 2.0 standard error codes (per spec https://www.jsonrpc.org/specification).
	ErrCodeParseError     = -32700 // Invalid JSON was received.
	ErrCodeInvalidRequest = -32600 // The JSON sent is not a valid Request object.
	ErrCodeMethodNotFound = -32601 // The method does not exist or is not available.
	ErrCodeInvalidParams  = -32602 // Invalid method parameter(s).
	ErrCodeInternalError  = -32603 // Internal JSON-RPC error.
)

// Request is a JSON-RPC 2.0 request or notification.
// Notifications have a nil ID.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is the JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// initializeResult is the payload returned for the initialize method.
type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      serverInfo         `json:"serverInfo"`
	Capabilities    serverCapabilities `json:"capabilities"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type serverCapabilities struct {
	Tools map[string]any `json:"tools"`
}

// toolsListResult is the payload returned for tools/list.
type toolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

// toolsCallParams are the parameters for tools/call.
type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// toolsCallResult is the payload returned for tools/call.
type toolsCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError"`
}
