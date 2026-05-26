package mcpserver_test

import (
	"encoding/json"
	"testing"

	"github.com/awf-project/cli/pkg/mcpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequest_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    mcpserver.Request
		wantErr bool
	}{
		{
			name:  "valid request with id",
			input: `{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
			want: mcpserver.Request{
				JSONRPC: "2.0",
				ID:      json.RawMessage("1"),
				Method:  "initialize",
			},
			wantErr: false,
		},
		{
			name:  "notification without id",
			input: `{"jsonrpc":"2.0","method":"notifications/initialized"}`,
			want: mcpserver.Request{
				JSONRPC: "2.0",
				ID:      nil,
				Method:  "notifications/initialized",
			},
			wantErr: false,
		},
		{
			name:  "request with params",
			input: `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"test"}}`,
			want: mcpserver.Request{
				JSONRPC: "2.0",
				ID:      json.RawMessage("2"),
				Method:  "tools/call",
				Params:  json.RawMessage(`{"name":"test"}`),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req mcpserver.Request
			err := json.Unmarshal([]byte(tt.input), &req)
			if tt.wantErr {
				assert.NotNil(t, err, "expected parse error for input: %s", tt.input)
			} else {
				require.NoError(t, err, "failed to unmarshal request: %s", tt.input)
				assert.Equal(t, tt.want.JSONRPC, req.JSONRPC)
				assert.Equal(t, tt.want.Method, req.Method)
				assert.Equal(t, string(tt.want.ID), string(req.ID))
			}
		})
	}
}

func TestResponse_MarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		resp  mcpserver.Response
		check func(t *testing.T, data []byte)
	}{
		{
			name: "response with result",
			resp: mcpserver.Response{
				JSONRPC: "2.0",
				ID:      json.RawMessage("1"),
				Result:  map[string]string{"key": "value"},
			},
			check: func(t *testing.T, data []byte) {
				var m map[string]any
				err := json.Unmarshal(data, &m)
				require.NoError(t, err)
				assert.Equal(t, "2.0", m["jsonrpc"])
				assert.Nil(t, m["error"])
				assert.NotNil(t, m["result"])
			},
		},
		{
			name: "response with error",
			resp: mcpserver.Response{
				JSONRPC: "2.0",
				ID:      json.RawMessage("2"),
				Error:   &mcpserver.RPCError{Code: mcpserver.ErrCodeMethodNotFound, Message: "Method not found"},
			},
			check: func(t *testing.T, data []byte) {
				var m map[string]any
				err := json.Unmarshal(data, &m)
				require.NoError(t, err)
				assert.NotNil(t, m["error"])
				assert.Nil(t, m["result"])

				errObj := m["error"].(map[string]any)
				assert.Equal(t, float64(mcpserver.ErrCodeMethodNotFound), errObj["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.resp)
			require.NoError(t, err)
			tt.check(t, data)
		})
	}
}

func TestRPCErrorCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		expected int
	}{
		{"parse error", mcpserver.ErrCodeParseError, -32700},
		{"method not found", mcpserver.ErrCodeMethodNotFound, -32601},
		{"invalid params", mcpserver.ErrCodeInvalidParams, -32602},
		{"internal error", mcpserver.ErrCodeInternalError, -32603},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code)
		})
	}
}

func TestProtocolVersion(t *testing.T) {
	assert.Equal(t, "2024-11-05", mcpserver.ProtocolVersion)
}

func TestMethodNames(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		expected string
	}{
		{"initialize", mcpserver.MethodInitialize, "initialize"},
		{"initialized", mcpserver.MethodInitialized, "notifications/initialized"},
		{"tools/list", mcpserver.MethodToolsList, "tools/list"},
		{"tools/call", mcpserver.MethodToolsCall, "tools/call"},
		{"shutdown", mcpserver.MethodShutdown, "shutdown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.method)
		})
	}
}
