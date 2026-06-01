package acpserver_test

import (
	"encoding/json"
	"testing"

	"github.com/awf-project/cli/pkg/acpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMethodConstants pins the JSON-RPC method names exchanged with ACP editors. A silent
// rename would break the handler registration ↔ wire-method mapping, so each is asserted.
func TestMethodConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{"initialize", acpserver.MethodInitialize, "initialize"},
		{"session/new", acpserver.MethodSessionNew, "session/new"},
		{"session/prompt", acpserver.MethodSessionPrompt, "session/prompt"},
		{"session/cancel", acpserver.MethodSessionCancel, "session/cancel"},
		{"session/update", acpserver.MethodSessionUpdate, "session/update"},
		{"session/request_permission", acpserver.MethodSessionRequestPermission, "session/request_permission"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.constant)
		})
	}
}

// TestProtocolVersion pins the integer ACP protocol version (NOT a date string like MCP).
func TestProtocolVersion(t *testing.T) {
	assert.Equal(t, 1, acpserver.ProtocolVersion)
}

// TestRequestResponse_JSONRoundTrip verifies the wire envelopes marshal/unmarshal using a
// method constant, and that the JSON tags produce the canonical JSON-RPC field names.
func TestRequestResponse_JSONRoundTrip(t *testing.T) {
	req := acpserver.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  acpserver.MethodSessionPrompt,
		Params:  json.RawMessage(`{"k":"v"}`),
	}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.JSONEq(t, `{"jsonrpc":"2.0","id":1,"method":"session/prompt","params":{"k":"v"}}`, string(data))

	var got acpserver.Request
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, acpserver.MethodSessionPrompt, got.Method)

	resp := acpserver.Response{JSONRPC: "2.0", ID: json.RawMessage("1"), Result: map[string]bool{"ok": true}}
	rdata, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.JSONEq(t, `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`, string(rdata))
}

// TestNewParseErrorResponse verifies the canonical parse-error envelope: id MUST be the
// explicit null literal and the code MUST be ErrParse.
func TestNewParseErrorResponse(t *testing.T) {
	resp := acpserver.NewParseErrorResponse()
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":null`)
	require.NotNil(t, resp.Error)
	assert.Equal(t, acpserver.ErrParse, resp.Error.Code)
}
