package acp

import (
	"testing"

	"github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
)

func TestToACPError(t *testing.T) {
	tests := []struct {
		name          string
		input         *application.ACPHandlerError
		expectNil     bool
		expectCode    int
		expectData    any
		expectMessage string
	}{
		{
			name:      "nil input returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name:          "ACPErrInvalidParams maps to InvalidParams code and preserves Message",
			input:         &application.ACPHandlerError{Kind: application.ACPErrInvalidParams, Message: "invalid params"},
			expectCode:    -32602, // JSON-RPC Invalid params
			expectMessage: "invalid params",
		},
		{
			name:          "ACPErrInternal maps to InternalError code and preserves Message",
			input:         &application.ACPHandlerError{Kind: application.ACPErrInternal, Message: "internal error"},
			expectCode:    -32603, // JSON-RPC Internal error
			expectMessage: "internal error",
		},
		{
			name:       "ACPErrMethodNotFound maps to MethodNotFound code",
			input:      &application.ACPHandlerError{Kind: application.ACPErrMethodNotFound, Message: "method not found"},
			expectCode: -32601, // JSON-RPC Method not found
			expectData: map[string]any{"method": "method not found"},
			// MethodNotFound uses the SDK constructor's generic message by design.
			expectMessage: "Method not found",
		},
		{
			name: "Message and Data are both preserved in conversion",
			input: &application.ACPHandlerError{
				Kind:    application.ACPErrInvalidParams,
				Message: "bad input",
				Data:    map[string]string{"code": "ERR_BAD_INPUT"},
			},
			expectCode:    -32602,
			expectData:    map[string]string{"code": "ERR_BAD_INPUT"},
			expectMessage: "bad input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toACPError(tt.input)

			if tt.expectNil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			sdkErr, ok := result.(*acp.RequestError)
			require.True(t, ok, "result should be *acp.RequestError")

			assert.Equal(t, tt.expectCode, sdkErr.Code)
			assert.Equal(t, tt.expectData, sdkErr.Data)
			assert.Equal(t, tt.expectMessage, sdkErr.Message,
				"human-readable Message must reach the editor, not be dropped for the SDK's generic string")
		})
	}
}

func TestInvalidParamsErr(t *testing.T) {
	msg := "invalid session id"
	result := invalidParamsErr(msg)

	require.NotNil(t, result)
	sdkErr, ok := result.(*acp.RequestError)
	require.True(t, ok, "result should be *acp.RequestError")

	assert.Equal(t, -32602, sdkErr.Code)
	assert.Nil(t, sdkErr.Data)
	assert.Equal(t, msg, sdkErr.Message, "invalidParamsErr message must reach the editor")
}

func TestInternalErr(t *testing.T) {
	msg := "failed to create session"
	result := internalErr(msg)

	require.NotNil(t, result)
	sdkErr, ok := result.(*acp.RequestError)
	require.True(t, ok, "result should be *acp.RequestError")

	assert.Equal(t, -32603, sdkErr.Code)
	assert.Nil(t, sdkErr.Data)
	assert.Equal(t, msg, sdkErr.Message, "internalErr message must reach the editor")
}

func TestMethodNotFoundErr(t *testing.T) {
	method := "unknown_method"
	result := methodNotFoundErr(method)

	require.NotNil(t, result)
	sdkErr, ok := result.(*acp.RequestError)
	require.True(t, ok, "result should be *acp.RequestError")

	assert.Equal(t, -32601, sdkErr.Code)
	assert.Equal(t, map[string]any{"method": method}, sdkErr.Data)
}
