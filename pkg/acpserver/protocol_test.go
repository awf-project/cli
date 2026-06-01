package acpserver_test

import (
	"encoding/json"
	"testing"

	"github.com/awf-project/cli/pkg/acpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequest_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		req  acpserver.Request
	}{
		{
			name: "string ID",
			req:  acpserver.Request{JSONRPC: "2.0", ID: json.RawMessage(`"abc"`), Method: "initialize"},
		},
		{
			name: "numeric ID",
			req:  acpserver.Request{JSONRPC: "2.0", ID: json.RawMessage(`42`), Method: "session/new"},
		},
		{
			name: "null ID notification",
			req:  acpserver.Request{JSONRPC: "2.0", Method: "session/update"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.req)
			require.NoError(t, err)

			var got acpserver.Request
			require.NoError(t, json.Unmarshal(data, &got))

			assert.Equal(t, tt.req.JSONRPC, got.JSONRPC)
			assert.Equal(t, tt.req.Method, got.Method)
			assert.Equal(t, string(tt.req.ID), string(got.ID))
		})
	}
}

func TestRequest_PreservesIDType(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		wantID  string
	}{
		{
			name:    "numeric ID preserved",
			jsonStr: `{"jsonrpc":"2.0","id":123,"method":"initialize"}`,
			wantID:  `123`,
		},
		{
			name:    "string ID preserved",
			jsonStr: `{"jsonrpc":"2.0","id":"req-abc","method":"session/new"}`,
			wantID:  `"req-abc"`,
		},
		{
			name:    "null ID preserved",
			jsonStr: `{"jsonrpc":"2.0","id":null,"method":"session/cancel"}`,
			wantID:  `null`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req acpserver.Request
			err := json.Unmarshal([]byte(tt.jsonStr), &req)
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, string(req.ID))
		})
	}
}

func TestRequest_WithParams(t *testing.T) {
	jsonStr := `{"jsonrpc":"2.0","id":1,"method":"session/request_permission","params":{"scope":"read","duration":3600}}`

	var req acpserver.Request
	err := json.Unmarshal([]byte(jsonStr), &req)
	require.NoError(t, err)

	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, `1`, string(req.ID))
	assert.Equal(t, "session/request_permission", req.Method)
	assert.NotEmpty(t, req.Params)

	var params map[string]any
	err = json.Unmarshal(req.Params, &params)
	require.NoError(t, err)
	assert.Equal(t, "read", params["scope"])
}

func TestResponse_JSONRoundTrip(t *testing.T) {
	resp := acpserver.Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  map[string]any{"ok": true},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var got acpserver.Response
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, "2.0", got.JSONRPC)
	assert.Equal(t, `1`, string(got.ID))
}

func TestResponse_WithError(t *testing.T) {
	resp := acpserver.Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-1"`),
		Error: &acpserver.Error{
			Code:    acpserver.ErrMethodNotFound,
			Message: "Method not found",
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.NotNil(t, raw["error"])
	assert.Nil(t, raw["result"])

	// JSON-RPC 2.0 §5: when "error" is present, "result" MUST be present as null,
	// not omitted. Assert the literal "result":null is on the wire.
	_, resultPresent := raw["result"]
	assert.True(t, resultPresent, `error response must include "result":null on the wire`)
	assert.Contains(t, string(data), `"result":null`)

	errObj := raw["error"].(map[string]any)
	assert.Equal(t, float64(acpserver.ErrMethodNotFound), errObj["code"])
	assert.Equal(t, "Method not found", errObj["message"])
}

func TestResponse_ErrorWithData(t *testing.T) {
	resp := acpserver.Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Error: &acpserver.Error{
			Code:    acpserver.ErrInvalidParams,
			Message: "Invalid parameter",
			Data:    map[string]string{"param": "scope", "reason": "unknown scope"},
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	errObj := raw["error"].(map[string]any)
	assert.NotNil(t, errObj["data"])
}

func TestNotification_JSONRoundTrip(t *testing.T) {
	notif := acpserver.Notification{
		JSONRPC: "2.0",
		Method:  "session/update",
		Params:  json.RawMessage(`{"status":"ready"}`),
	}

	data, err := json.Marshal(notif)
	require.NoError(t, err)

	var got acpserver.Notification
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, "2.0", got.JSONRPC)
	assert.Equal(t, "session/update", got.Method)
	assert.Equal(t, `{"status":"ready"}`, string(got.Params))
}

func TestNotification_WithoutParams(t *testing.T) {
	notif := acpserver.Notification{
		JSONRPC: "2.0",
		Method:  "session/cancel",
	}

	data, err := json.Marshal(notif)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Equal(t, "2.0", raw["jsonrpc"])
	assert.Equal(t, "session/cancel", raw["method"])
}

func TestNewParseErrorResponse_NullID(t *testing.T) {
	resp := acpserver.NewParseErrorResponse()

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"id":null`)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Nil(t, raw["id"])
}

func TestNewParseErrorResponse_HasErrorCode(t *testing.T) {
	resp := acpserver.NewParseErrorResponse()

	require.NotNil(t, resp.Error)
	assert.Equal(t, acpserver.ErrParse, resp.Error.Code)
	assert.NotEmpty(t, resp.Error.Message)
}

func TestNewParseErrorResponse_NullResult(t *testing.T) {
	resp := acpserver.NewParseErrorResponse()

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	// JSON-RPC 2.0 §5: an error response carries "result":null (present, not omitted).
	assert.Contains(t, string(data), `"result":null`)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	_, resultPresent := raw["result"]
	assert.True(t, resultPresent, `error response must include "result":null`)
	assert.Nil(t, raw["result"], "result value must be null when error is present")
}

func TestErrorCodeConstants(t *testing.T) {
	assert.Equal(t, -32700, acpserver.ErrParse)
	assert.Equal(t, -32600, acpserver.ErrInvalidRequest)
	assert.Equal(t, -32601, acpserver.ErrMethodNotFound)
	assert.Equal(t, -32602, acpserver.ErrInvalidParams)
	assert.Equal(t, -32603, acpserver.ErrInternal)
}

func TestMethodNameConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
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
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestProtocolVersion_Type(t *testing.T) {
	assert.IsType(t, 0, acpserver.ProtocolVersion)
}

func TestProtocolVersion_NotZero(t *testing.T) {
	assert.NotZero(t, acpserver.ProtocolVersion, "ProtocolVersion must be set to spec-pinned integer")
}

func TestError_JSONStructure(t *testing.T) {
	errObj := &acpserver.Error{
		Code:    acpserver.ErrInvalidRequest,
		Message: "The JSON sent is not a valid Request object",
	}

	data, err := json.Marshal(errObj)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Equal(t, float64(acpserver.ErrInvalidRequest), raw["code"])
	assert.Equal(t, "The JSON sent is not a valid Request object", raw["message"])
}
