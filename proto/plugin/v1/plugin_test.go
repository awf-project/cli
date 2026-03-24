package pluginv1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// TestPluginServiceMessages_GetInfoRequest verifies GetInfoRequest message creation.
func TestPluginServiceMessages_GetInfoRequest(t *testing.T) {
	req := &GetInfoRequest{}
	require.NotNil(t, req)

	// Verify message can be marshaled
	data, err := proto.Marshal(req)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	// Verify marshaled data can be unmarshaled
	req2 := &GetInfoRequest{}
	err = proto.Unmarshal(data, req2)
	assert.NoError(t, err)
}

// TestPluginServiceMessages_GetInfoResponse verifies GetInfoResponse message with fields.
func TestPluginServiceMessages_GetInfoResponse(t *testing.T) {
	resp := &GetInfoResponse{
		Name:         "test-plugin",
		Version:      "1.0.0",
		Description:  "Test plugin description",
		Capabilities: []string{"capability1", "capability2"},
	}
	require.NotNil(t, resp)

	// Verify fields are accessible
	assert.Equal(t, "test-plugin", resp.Name)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Equal(t, "Test plugin description", resp.Description)
	assert.Len(t, resp.Capabilities, 2)
	assert.Contains(t, resp.Capabilities, "capability1")

	// Verify round-trip serialization
	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &GetInfoResponse{}
	err = proto.Unmarshal(data, resp2)
	require.NoError(t, err)

	assert.Equal(t, resp.Name, resp2.Name)
	assert.Equal(t, resp.Version, resp2.Version)
	assert.Equal(t, resp.Capabilities, resp2.Capabilities)
}

// TestPluginServiceMessages_InitRequest verifies InitRequest with map config.
func TestPluginServiceMessages_InitRequest(t *testing.T) {
	req := &InitRequest{
		Config: map[string][]byte{
			"key1": []byte("value1"),
			"key2": []byte("value2"),
		},
	}
	require.NotNil(t, req)

	// Verify map fields are accessible
	assert.Len(t, req.Config, 2)
	assert.Equal(t, []byte("value1"), req.Config["key1"])

	// Verify serialization with map fields
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	req2 := &InitRequest{}
	err = proto.Unmarshal(data, req2)
	require.NoError(t, err)

	assert.Equal(t, req.Config, req2.Config)
}

// TestOperationServiceMessages_ExecuteRequest verifies ExecuteRequest message.
func TestOperationServiceMessages_ExecuteRequest(t *testing.T) {
	req := &ExecuteRequest{
		Operation: "test-operation",
		Inputs: map[string][]byte{
			"input1": []byte("data1"),
			"input2": []byte("data2"),
		},
	}
	require.NotNil(t, req)

	assert.Equal(t, "test-operation", req.Operation)
	assert.Len(t, req.Inputs, 2)

	// Verify round-trip
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	req2 := &ExecuteRequest{}
	err = proto.Unmarshal(data, req2)
	require.NoError(t, err)

	assert.Equal(t, req.Operation, req2.Operation)
	assert.Equal(t, req.Inputs, req2.Inputs)
}

// TestOperationServiceMessages_ExecuteResponse verifies ExecuteResponse with data map.
func TestOperationServiceMessages_ExecuteResponse(t *testing.T) {
	resp := &ExecuteResponse{
		Success: true,
		Output:  "command output",
		Data: map[string][]byte{
			"result": []byte("data"),
		},
		Error: "",
	}
	require.NotNil(t, resp)

	assert.True(t, resp.Success)
	assert.Equal(t, "command output", resp.Output)
	assert.Empty(t, resp.Error)
	assert.Len(t, resp.Data, 1)

	// Verify serialization
	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &ExecuteResponse{}
	err = proto.Unmarshal(data, resp2)
	require.NoError(t, err)

	assert.Equal(t, resp.Success, resp2.Success)
	assert.Equal(t, resp.Output, resp2.Output)
	assert.Equal(t, resp.Data, resp2.Data)
}

// TestOperationServiceMessages_ExecuteResponseError verifies error response.
func TestOperationServiceMessages_ExecuteResponseError(t *testing.T) {
	resp := &ExecuteResponse{
		Success: false,
		Output:  "",
		Error:   "operation failed: invalid input",
	}
	require.NotNil(t, resp)

	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
	assert.Equal(t, "operation failed: invalid input", resp.Error)
}

// TestOperationSchema_Message verifies OperationSchema message structure.
func TestOperationSchema_Message(t *testing.T) {
	schema := &OperationSchema{
		Name:        "echo-operation",
		Description: "Echo operation description",
		Inputs: []*InputSchema{
			{
				Name:        "text",
				Type:        "string",
				Required:    true,
				Description: "Text to echo",
			},
		},
		Outputs: []*OutputSchema{
			{
				Name:        "result",
				Type:        "string",
				Description: "Echoed text",
			},
		},
	}
	require.NotNil(t, schema)

	assert.Equal(t, "echo-operation", schema.Name)
	assert.Len(t, schema.Inputs, 1)
	assert.Len(t, schema.Outputs, 1)
	assert.Equal(t, "text", schema.Inputs[0].Name)
	assert.True(t, schema.Inputs[0].Required)

	// Verify serialization
	data, err := proto.Marshal(schema)
	require.NoError(t, err)

	schema2 := &OperationSchema{}
	err = proto.Unmarshal(data, schema2)
	require.NoError(t, err)

	assert.Equal(t, schema.Name, schema2.Name)
	assert.Len(t, schema2.Inputs, 1)
}

// TestInputSchema_AllFields verifies InputSchema field population.
func TestInputSchema_AllFields(t *testing.T) {
	input := &InputSchema{
		Name:        "filename",
		Type:        "string",
		Required:    true,
		Default:     "config.json",
		Description: "Configuration file path",
		Validation:  "^[a-zA-Z0-9._/-]+$",
	}
	require.NotNil(t, input)

	assert.Equal(t, "filename", input.Name)
	assert.Equal(t, "string", input.Type)
	assert.True(t, input.Required)
	assert.Equal(t, "config.json", input.Default)
	assert.NotEmpty(t, input.Validation)
}

// TestListOperationsResponse verifies repeated operations field.
func TestListOperationsResponse(t *testing.T) {
	resp := &ListOperationsResponse{
		Operations: []*OperationSchema{
			{
				Name:        "op1",
				Description: "First operation",
			},
			{
				Name:        "op2",
				Description: "Second operation",
			},
		},
	}
	require.NotNil(t, resp)

	assert.Len(t, resp.Operations, 2)
	assert.Equal(t, "op1", resp.Operations[0].Name)
	assert.Equal(t, "op2", resp.Operations[1].Name)

	// Verify serialization
	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &ListOperationsResponse{}
	err = proto.Unmarshal(data, resp2)
	require.NoError(t, err)

	assert.Len(t, resp2.Operations, 2)
}

// TestGetOperationResponse verifies single operation retrieval.
func TestGetOperationResponse(t *testing.T) {
	resp := &GetOperationResponse{
		Operation: &OperationSchema{
			Name:        "test-op",
			Description: "Test operation",
		},
	}
	require.NotNil(t, resp)
	require.NotNil(t, resp.Operation)

	assert.Equal(t, "test-op", resp.Operation.Name)

	// Verify nil operation case
	resp2 := &GetOperationResponse{
		Operation: nil,
	}
	assert.Nil(t, resp2.Operation)
}

// TestShutdownMessages verifies empty response messages.
func TestShutdownMessages(t *testing.T) {
	req := &ShutdownRequest{}
	resp := &ShutdownResponse{}

	require.NotNil(t, req)
	require.NotNil(t, resp)

	// Verify empty messages serialize correctly
	reqData, err := proto.Marshal(req)
	require.NoError(t, err)

	respData, err := proto.Marshal(resp)
	require.NoError(t, err)

	// Verify unmarshal
	req2 := &ShutdownRequest{}
	err = proto.Unmarshal(reqData, req2)
	assert.NoError(t, err)

	resp2 := &ShutdownResponse{}
	err = proto.Unmarshal(respData, resp2)
	assert.NoError(t, err)
}

// TestProtoMessageDescriptor verifies message types have proper reflection support.
func TestProtoMessageDescriptor(t *testing.T) {
	msg := &GetInfoResponse{Name: "test"}

	descriptor := msg.ProtoReflect().Descriptor()
	require.NotNil(t, descriptor)

	// Verify message name is correct — protoreflect.Name is a named string type, EqualValues coerces for comparison
	assert.EqualValues(t, "GetInfoResponse", descriptor.Name())
}

// TestOperationServiceEmptyRequests verifies empty request messages.
func TestOperationServiceEmptyRequests(t *testing.T) {
	listReq := &ListOperationsRequest{}
	require.NotNil(t, listReq)

	// Should serialize without error
	data, err := proto.Marshal(listReq)
	assert.NoError(t, err)
	assert.NotNil(t, data)
}

// TestGetOperationRequest_OperationName verifies operation name field.
func TestGetOperationRequest_OperationName(t *testing.T) {
	req := &GetOperationRequest{
		Name: "my-operation",
	}
	require.NotNil(t, req)

	assert.Equal(t, "my-operation", req.Name)

	// Verify empty name case
	req2 := &GetOperationRequest{Name: ""}
	assert.Empty(t, req2.Name)
}
