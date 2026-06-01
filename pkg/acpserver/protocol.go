package acpserver

import "encoding/json"

const (
	ErrParse          = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternal       = -32603
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response envelope.
//
// Per JSON-RPC 2.0 §5, exactly one of Result/Error is meaningful, but the wire
// shape still requires "result" to be present (as null) whenever an error is
// reported — "result" MUST be null if "error" is present. Result therefore has
// no omitempty: a success carries the real value, an error serializes
// "result":null. ID likewise omits omitempty so an error response with an
// unknown request id ("id":null literal) always emits the id field, as the
// parse-error and oversize-line paths rely on.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
	Error   *Error          `json:"error,omitempty"`
}

type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func NewParseErrorResponse() Response {
	return Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage("null"),
		Error: &Error{
			Code:    ErrParse,
			Message: "Parse error",
		},
	}
}
