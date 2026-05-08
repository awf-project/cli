package pluginv1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
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

// TestSeverityEnum verifies Severity enum values match specification.
func TestSeverityEnum(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		value    int32
	}{
		{
			name:     "UNSPECIFIED is zero value",
			severity: Severity_SEVERITY_UNSPECIFIED,
			value:    0,
		},
		{
			name:     "WARNING is 1",
			severity: Severity_SEVERITY_WARNING,
			value:    1,
		},
		{
			name:     "ERROR is 2",
			severity: Severity_SEVERITY_ERROR,
			value:    2,
		},
		{
			name:     "INFO is 3",
			severity: Severity_SEVERITY_INFO,
			value:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, int32(tt.value), int32(tt.severity))
		})
	}
}

// TestValidationIssue_HappyPath verifies ValidationIssue message creation with all fields.
func TestValidationIssue_HappyPath(t *testing.T) {
	issue := &ValidationIssue{
		Severity: Severity_SEVERITY_ERROR,
		Message:  "Invalid step configuration",
		Step:     "step-name",
		Field:    "timeout",
	}
	require.NotNil(t, issue)

	assert.Equal(t, Severity_SEVERITY_ERROR, issue.Severity)
	assert.Equal(t, "Invalid step configuration", issue.Message)
	assert.Equal(t, "step-name", issue.Step)
	assert.Equal(t, "timeout", issue.Field)

	// Verify round-trip serialization
	data, err := proto.Marshal(issue)
	require.NoError(t, err)

	issue2 := &ValidationIssue{}
	err = proto.Unmarshal(data, issue2)
	require.NoError(t, err)

	assert.Equal(t, issue.Severity, issue2.Severity)
	assert.Equal(t, issue.Message, issue2.Message)
	assert.Equal(t, issue.Step, issue2.Step)
	assert.Equal(t, issue.Field, issue2.Field)
}

// TestValidationIssue_SeverityVariants verifies ValidationIssue with different severity levels.
func TestValidationIssue_SeverityVariants(t *testing.T) {
	severities := []Severity{
		Severity_SEVERITY_UNSPECIFIED,
		Severity_SEVERITY_WARNING,
		Severity_SEVERITY_ERROR,
		Severity_SEVERITY_INFO,
	}

	for _, sev := range severities {
		issue := &ValidationIssue{
			Severity: sev,
			Message:  "Test message",
		}
		require.NotNil(t, issue)
		assert.Equal(t, sev, issue.Severity)

		data, err := proto.Marshal(issue)
		require.NoError(t, err)

		issue2 := &ValidationIssue{}
		err = proto.Unmarshal(data, issue2)
		require.NoError(t, err)

		assert.Equal(t, sev, issue2.Severity)
	}
}

// TestValidateWorkflowRequest verifies ValidateWorkflowRequest message.
func TestValidateWorkflowRequest(t *testing.T) {
	workflowJSON := []byte(`{"name":"test-workflow","steps":[]}`)
	req := &ValidateWorkflowRequest{
		WorkflowJson: workflowJSON,
	}
	require.NotNil(t, req)

	assert.Equal(t, workflowJSON, req.WorkflowJson)
	assert.NotEmpty(t, req.WorkflowJson)

	// Verify serialization
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	req2 := &ValidateWorkflowRequest{}
	err = proto.Unmarshal(data, req2)
	require.NoError(t, err)

	assert.Equal(t, workflowJSON, req2.WorkflowJson)
}

// TestValidateWorkflowResponse verifies ValidateWorkflowResponse with validation issues.
func TestValidateWorkflowResponse(t *testing.T) {
	resp := &ValidateWorkflowResponse{
		Issues: []*ValidationIssue{
			{
				Severity: Severity_SEVERITY_WARNING,
				Message:  "Unused variable",
				Step:     "step1",
				Field:    "inputs",
			},
			{
				Severity: Severity_SEVERITY_ERROR,
				Message:  "Invalid timeout format",
				Step:     "step2",
				Field:    "timeout",
			},
		},
	}
	require.NotNil(t, resp)

	assert.Len(t, resp.Issues, 2)
	assert.Equal(t, Severity_SEVERITY_WARNING, resp.Issues[0].Severity)
	assert.Equal(t, Severity_SEVERITY_ERROR, resp.Issues[1].Severity)
	assert.Equal(t, "step1", resp.Issues[0].Step)
	assert.Equal(t, "step2", resp.Issues[1].Step)

	// Verify serialization
	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &ValidateWorkflowResponse{}
	err = proto.Unmarshal(data, resp2)
	require.NoError(t, err)

	assert.Len(t, resp2.Issues, 2)
	assert.Equal(t, resp.Issues[0].Message, resp2.Issues[0].Message)
}

// TestValidateWorkflowResponse_EmptyIssues verifies empty issues list.
func TestValidateWorkflowResponse_EmptyIssues(t *testing.T) {
	resp := &ValidateWorkflowResponse{
		Issues: []*ValidationIssue{},
	}
	require.NotNil(t, resp)

	assert.Empty(t, resp.Issues)
	assert.Len(t, resp.Issues, 0)

	// Empty response should serialize successfully
	data, err := proto.Marshal(resp)
	assert.NoError(t, err)

	resp2 := &ValidateWorkflowResponse{}
	err = proto.Unmarshal(data, resp2)
	assert.NoError(t, err)
	assert.Empty(t, resp2.Issues)
}

// TestValidateStepRequest verifies ValidateStepRequest message.
func TestValidateStepRequest(t *testing.T) {
	workflowJSON := []byte(`{"steps":[{"name":"test-step"}]}`)
	req := &ValidateStepRequest{
		WorkflowJson: workflowJSON,
		StepName:     "test-step",
	}
	require.NotNil(t, req)

	assert.Equal(t, workflowJSON, req.WorkflowJson)
	assert.Equal(t, "test-step", req.StepName)

	// Verify serialization
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	req2 := &ValidateStepRequest{}
	err = proto.Unmarshal(data, req2)
	require.NoError(t, err)

	assert.Equal(t, "test-step", req2.StepName)
	assert.Equal(t, workflowJSON, req2.WorkflowJson)
}

// TestValidateStepResponse verifies ValidateStepResponse message.
func TestValidateStepResponse(t *testing.T) {
	resp := &ValidateStepResponse{
		Issues: []*ValidationIssue{
			{
				Severity: Severity_SEVERITY_ERROR,
				Message:  "Missing required field",
				Step:     "test-step",
				Field:    "type",
			},
		},
	}
	require.NotNil(t, resp)

	assert.Len(t, resp.Issues, 1)
	assert.Equal(t, "Missing required field", resp.Issues[0].Message)

	// Verify round-trip
	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &ValidateStepResponse{}
	err = proto.Unmarshal(data, resp2)
	require.NoError(t, err)

	assert.Len(t, resp2.Issues, 1)
	assert.Equal(t, resp.Issues[0].Message, resp2.Issues[0].Message)
}

// TestListStepTypesRequest verifies empty ListStepTypesRequest.
func TestListStepTypesRequest(t *testing.T) {
	req := &ListStepTypesRequest{}
	require.NotNil(t, req)

	data, err := proto.Marshal(req)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	req2 := &ListStepTypesRequest{}
	err = proto.Unmarshal(data, req2)
	assert.NoError(t, err)
}

// TestStepTypeInfo verifies StepTypeInfo message structure.
func TestStepTypeInfo(t *testing.T) {
	info := &StepTypeInfo{
		Name:        "database",
		Description: "Execute database queries",
	}
	require.NotNil(t, info)

	assert.Equal(t, "database", info.Name)
	assert.Equal(t, "Execute database queries", info.Description)

	// Verify serialization
	data, err := proto.Marshal(info)
	require.NoError(t, err)

	info2 := &StepTypeInfo{}
	err = proto.Unmarshal(data, info2)
	require.NoError(t, err)

	assert.Equal(t, "database", info2.Name)
	assert.Equal(t, "Execute database queries", info2.Description)
}

// TestListStepTypesResponse verifies ListStepTypesResponse with step types.
func TestListStepTypesResponse(t *testing.T) {
	resp := &ListStepTypesResponse{
		StepTypes: []*StepTypeInfo{
			{
				Name:        "database",
				Description: "Execute database queries",
			},
			{
				Name:        "http",
				Description: "Make HTTP requests",
			},
		},
	}
	require.NotNil(t, resp)

	assert.Len(t, resp.StepTypes, 2)
	assert.Equal(t, "database", resp.StepTypes[0].Name)
	assert.Equal(t, "http", resp.StepTypes[1].Name)

	// Verify serialization
	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &ListStepTypesResponse{}
	err = proto.Unmarshal(data, resp2)
	require.NoError(t, err)

	assert.Len(t, resp2.StepTypes, 2)
	assert.Equal(t, resp.StepTypes[0].Name, resp2.StepTypes[0].Name)
}

// TestListStepTypesResponse_EmptyStepTypes verifies empty step types list.
func TestListStepTypesResponse_EmptyStepTypes(t *testing.T) {
	resp := &ListStepTypesResponse{
		StepTypes: []*StepTypeInfo{},
	}
	require.NotNil(t, resp)

	assert.Empty(t, resp.StepTypes)
	assert.Len(t, resp.StepTypes, 0)

	data, err := proto.Marshal(resp)
	assert.NoError(t, err)

	resp2 := &ListStepTypesResponse{}
	err = proto.Unmarshal(data, resp2)
	assert.NoError(t, err)
	assert.Empty(t, resp2.StepTypes)
}

// TestExecuteStepRequest verifies ExecuteStepRequest message with all fields.
func TestExecuteStepRequest(t *testing.T) {
	config := []byte(`{"host":"localhost","port":5432}`)
	inputs := []byte(`{"query":"SELECT * FROM users"}`)

	req := &ExecuteStepRequest{
		StepName: "db-query",
		StepType: "database",
		Config:   config,
		Inputs:   inputs,
	}
	require.NotNil(t, req)

	assert.Equal(t, "db-query", req.StepName)
	assert.Equal(t, "database", req.StepType)
	assert.Equal(t, config, req.Config)
	assert.Equal(t, inputs, req.Inputs)

	// Verify serialization
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	req2 := &ExecuteStepRequest{}
	err = proto.Unmarshal(data, req2)
	require.NoError(t, err)

	assert.Equal(t, "db-query", req2.StepName)
	assert.Equal(t, "database", req2.StepType)
	assert.Equal(t, config, req2.Config)
	assert.Equal(t, inputs, req2.Inputs)
}

// TestExecuteStepRequest_MinimalFields verifies ExecuteStepRequest with minimal fields.
func TestExecuteStepRequest_MinimalFields(t *testing.T) {
	req := &ExecuteStepRequest{
		StepName: "step1",
		StepType: "custom",
	}
	require.NotNil(t, req)

	assert.Equal(t, "step1", req.StepName)
	assert.Equal(t, "custom", req.StepType)
	assert.Nil(t, req.Config)
	assert.Nil(t, req.Inputs)

	// Verify serialization without optional fields
	data, err := proto.Marshal(req)
	assert.NoError(t, err)

	req2 := &ExecuteStepRequest{}
	err = proto.Unmarshal(data, req2)
	assert.NoError(t, err)

	assert.Equal(t, "step1", req2.StepName)
}

// TestExecuteStepResponse verifies ExecuteStepResponse message.
func TestExecuteStepResponse(t *testing.T) {
	output := "Query executed successfully"
	data := []byte(`{"rows":10}`)

	resp := &ExecuteStepResponse{
		Output:   output,
		Data:     data,
		ExitCode: 0,
	}
	require.NotNil(t, resp)

	assert.Equal(t, output, resp.Output)
	assert.Equal(t, data, resp.Data)
	assert.Equal(t, int32(0), resp.ExitCode)

	// Verify serialization
	msgData, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &ExecuteStepResponse{}
	err = proto.Unmarshal(msgData, resp2)
	require.NoError(t, err)

	assert.Equal(t, output, resp2.Output)
	assert.Equal(t, data, resp2.Data)
	assert.Equal(t, int32(0), resp2.ExitCode)
}

// TestExecuteStepResponse_ErrorCase verifies ExecuteStepResponse with error exit code.
func TestExecuteStepResponse_ErrorCase(t *testing.T) {
	resp := &ExecuteStepResponse{
		Output:   "Error executing step",
		ExitCode: 1,
	}
	require.NotNil(t, resp)

	assert.Equal(t, "Error executing step", resp.Output)
	assert.Equal(t, int32(1), resp.ExitCode)
	assert.Nil(t, resp.Data)

	// Verify serialization
	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &ExecuteStepResponse{}
	err = proto.Unmarshal(data, resp2)
	require.NoError(t, err)

	assert.Equal(t, int32(1), resp2.ExitCode)
}

// TestHandleEventRequest_HappyPath verifies HandleEventRequest with all fields.
func TestHandleEventRequest_HappyPath(t *testing.T) {
	req := &HandleEventRequest{
		Id:                 "event-123",
		Type:               "workflow.step.completed",
		TimestampUnixNanos: 1700000000000000000,
		Source:             "plugin-a",
		Metadata: map[string]string{
			"correlation_id": "corr-456",
		},
		Payload:          []byte(`{"result":"ok"}`),
		PropagationDepth: 2,
	}
	require.NotNil(t, req)

	assert.Equal(t, "event-123", req.Id)
	assert.Equal(t, "workflow.step.completed", req.Type)
	assert.Equal(t, int64(1700000000000000000), req.TimestampUnixNanos)
	assert.Equal(t, "plugin-a", req.Source)
	assert.Equal(t, "corr-456", req.Metadata["correlation_id"])
	assert.Equal(t, []byte(`{"result":"ok"}`), req.Payload)
	assert.Equal(t, int32(2), req.PropagationDepth)

	data, err := proto.Marshal(req)
	require.NoError(t, err)

	req2 := &HandleEventRequest{}
	err = proto.Unmarshal(data, req2)
	require.NoError(t, err)

	assert.Equal(t, req.Id, req2.Id)
	assert.Equal(t, req.Type, req2.Type)
	assert.Equal(t, req.TimestampUnixNanos, req2.TimestampUnixNanos)
	assert.Equal(t, req.Source, req2.Source)
	assert.Equal(t, req.Metadata, req2.Metadata)
	assert.Equal(t, req.Payload, req2.Payload)
	assert.Equal(t, req.PropagationDepth, req2.PropagationDepth)
}

// TestHandleEventResponse_HappyPath verifies HandleEventResponse with emitted events.
func TestHandleEventResponse_HappyPath(t *testing.T) {
	resp := &HandleEventResponse{
		EmittedEvents: []*HandleEventRequest{
			{
				Id:     "child-event-1",
				Type:   "workflow.step.started",
				Source: "plugin-b",
			},
		},
	}
	require.NotNil(t, resp)

	assert.Len(t, resp.EmittedEvents, 1)
	assert.Equal(t, "child-event-1", resp.EmittedEvents[0].Id)
	assert.Equal(t, "workflow.step.started", resp.EmittedEvents[0].Type)

	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &HandleEventResponse{}
	err = proto.Unmarshal(data, resp2)
	require.NoError(t, err)

	assert.Len(t, resp2.EmittedEvents, 1)
	assert.Equal(t, resp.EmittedEvents[0].Id, resp2.EmittedEvents[0].Id)
}

// TestEmitRequest_HappyPath verifies EmitRequest has all required fields.
func TestEmitRequest_HappyPath(t *testing.T) {
	req := &EmitRequest{
		EventType:          "workflow.step.completed",
		Payload:            []byte(`{"output":"done"}`),
		SourcePlugin:       "my-plugin",
		PropagationDepth:   1,
		TimestampUnixNanos: 1700000000000000000,
		Metadata: map[string]string{
			"trace_id": "trace-abc",
		},
	}
	require.NotNil(t, req)

	assert.Equal(t, "workflow.step.completed", req.EventType)
	assert.Equal(t, []byte(`{"output":"done"}`), req.Payload)
	assert.Equal(t, "my-plugin", req.SourcePlugin)
	assert.Equal(t, int32(1), req.PropagationDepth)
	assert.Equal(t, int64(1700000000000000000), req.TimestampUnixNanos)
	assert.Equal(t, "trace-abc", req.Metadata["trace_id"])

	data, err := proto.Marshal(req)
	require.NoError(t, err)

	req2 := &EmitRequest{}
	err = proto.Unmarshal(data, req2)
	require.NoError(t, err)

	assert.Equal(t, req.EventType, req2.EventType)
	assert.Equal(t, req.Payload, req2.Payload)
	assert.Equal(t, req.SourcePlugin, req2.SourcePlugin)
	assert.Equal(t, req.PropagationDepth, req2.PropagationDepth)
	assert.Equal(t, req.TimestampUnixNanos, req2.TimestampUnixNanos)
	assert.Equal(t, req.Metadata, req2.Metadata)
}

// TestEmitResponse_HappyPath verifies EmitResponse success fields.
func TestEmitResponse_HappyPath(t *testing.T) {
	resp := &EmitResponse{
		Success:      true,
		ErrorMessage: "",
		EventId:      "evt-789",
	}
	require.NotNil(t, resp)

	assert.True(t, resp.Success)
	assert.Empty(t, resp.ErrorMessage)
	assert.Equal(t, "evt-789", resp.EventId)

	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &EmitResponse{}
	err = proto.Unmarshal(data, resp2)
	require.NoError(t, err)

	assert.True(t, resp2.Success)
	assert.Equal(t, "evt-789", resp2.EventId)
}

// TestEmitResponse_ErrorCase verifies EmitResponse when emission fails.
func TestEmitResponse_ErrorCase(t *testing.T) {
	resp := &EmitResponse{
		Success:      false,
		ErrorMessage: "propagation depth exceeded",
		EventId:      "",
	}
	require.NotNil(t, resp)

	assert.False(t, resp.Success)
	assert.Equal(t, "propagation depth exceeded", resp.ErrorMessage)
	assert.Empty(t, resp.EventId)

	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &EmitResponse{}
	err = proto.Unmarshal(data, resp2)
	require.NoError(t, err)

	assert.False(t, resp2.Success)
	assert.Equal(t, resp.ErrorMessage, resp2.ErrorMessage)
}

// TestEventStreamMessage_HappyPath verifies EventStreamMessage has all 8 required fields.
func TestEventStreamMessage_HappyPath(t *testing.T) {
	msg := &EventStreamMessage{
		Id:                 "stream-evt-001",
		Type:               "workflow.step.started",
		TimestampUnixNanos: 1700000000000000000,
		Source:             "host",
		Metadata: map[string]string{
			"workflow_id": "wf-123",
		},
		Payload:          []byte(`{"step":"init"}`),
		PropagationDepth: 0,
		SequenceNumber:   42,
	}
	require.NotNil(t, msg)

	assert.Equal(t, "stream-evt-001", msg.Id)
	assert.Equal(t, "workflow.step.started", msg.Type)
	assert.Equal(t, int64(1700000000000000000), msg.TimestampUnixNanos)
	assert.Equal(t, "host", msg.Source)
	assert.Equal(t, "wf-123", msg.Metadata["workflow_id"])
	assert.Equal(t, []byte(`{"step":"init"}`), msg.Payload)
	assert.Equal(t, int32(0), msg.PropagationDepth)
	assert.Equal(t, uint64(42), msg.SequenceNumber)

	data, err := proto.Marshal(msg)
	require.NoError(t, err)

	msg2 := &EventStreamMessage{}
	err = proto.Unmarshal(data, msg2)
	require.NoError(t, err)

	assert.Equal(t, msg.Id, msg2.Id)
	assert.Equal(t, msg.Type, msg2.Type)
	assert.Equal(t, msg.TimestampUnixNanos, msg2.TimestampUnixNanos)
	assert.Equal(t, msg.Source, msg2.Source)
	assert.Equal(t, msg.Metadata, msg2.Metadata)
	assert.Equal(t, msg.Payload, msg2.Payload)
	assert.Equal(t, msg.PropagationDepth, msg2.PropagationDepth)
	assert.Equal(t, msg.SequenceNumber, msg2.SequenceNumber)
}

// TestStreamEventsResponse_EmptyMessage verifies StreamEventsResponse is an empty message.
func TestStreamEventsResponse_EmptyMessage(t *testing.T) {
	resp := &StreamEventsResponse{}
	require.NotNil(t, resp)

	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	resp2 := &StreamEventsResponse{}
	err = proto.Unmarshal(data, resp2)
	require.NoError(t, err)

	assert.EqualValues(t, "StreamEventsResponse", resp.ProtoReflect().Descriptor().Name())
}

// TestEventService_StreamEventsClientInterface verifies the client-side streaming interface.
// The host (client) must be able to Send(*EventStreamMessage) and CloseAndRecv() (*StreamEventsResponse, error).
func TestEventService_StreamEventsClientInterface(t *testing.T) {
	// Compile-time assertion: EventService_StreamEventsClient satisfies the required interface.
	// This test fails to compile if the generated type does not expose Send/CloseAndRecv.
	type streamEventsClientIface interface {
		Send(*EventStreamMessage) error
		CloseAndRecv() (*StreamEventsResponse, error)
	}
	var _ streamEventsClientIface = EventService_StreamEventsClient(nil)
}

// TestEventService_StreamEventsServerInterface verifies the server-side streaming interface.
// The plugin (server) must be able to Recv() (*EventStreamMessage, error) and SendAndClose(*StreamEventsResponse) error.
func TestEventService_StreamEventsServerInterface(t *testing.T) {
	// Compile-time assertion: EventService_StreamEventsServer satisfies the required interface.
	type streamEventsServerIface interface {
		Recv() (*EventStreamMessage, error)
		SendAndClose(*StreamEventsResponse) error
	}
	var _ streamEventsServerIface = EventService_StreamEventsServer(nil)
}

// TestHostEventService_ServiceDescriptor verifies HostEventService descriptor is registered.
func TestHostEventService_ServiceDescriptor(t *testing.T) {
	desc := &HostEventService_ServiceDesc
	assert.Equal(t, "plugin.v1.HostEventService", desc.ServiceName)
	assert.Len(t, desc.Methods, 1)
	assert.Equal(t, "Emit", desc.Methods[0].MethodName)
	assert.Empty(t, desc.Streams)
}

// TestEventService_StreamEventsDescriptor verifies EventService has StreamEvents as a client-side stream.
func TestEventService_StreamEventsDescriptor(t *testing.T) {
	desc := &EventService_ServiceDesc
	assert.Equal(t, "plugin.v1.EventService", desc.ServiceName)

	var streamDesc *grpc.StreamDesc
	for i := range desc.Streams {
		if desc.Streams[i].StreamName == "StreamEvents" {
			streamDesc = &desc.Streams[i]
			break
		}
	}
	require.NotNil(t, streamDesc, "StreamEvents stream descriptor not found in EventService")
	assert.True(t, streamDesc.ClientStreams, "StreamEvents must be client-side streaming")
	assert.False(t, streamDesc.ServerStreams, "StreamEvents must not be server-side streaming")
}

// TestValidationIssue_AllSeveritiesSerialize verifies all severity enum values serialize correctly.
func TestValidationIssue_AllSeveritiesSerialize(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
	}{
		{"UNSPECIFIED", Severity_SEVERITY_UNSPECIFIED},
		{"WARNING", Severity_SEVERITY_WARNING},
		{"ERROR", Severity_SEVERITY_ERROR},
		{"INFO", Severity_SEVERITY_INFO},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &ValidationIssue{
				Severity: tt.severity,
				Message:  "Test message for " + tt.name,
			}

			data, err := proto.Marshal(issue)
			require.NoError(t, err, "failed to marshal %s severity", tt.name)

			issue2 := &ValidationIssue{}
			err = proto.Unmarshal(data, issue2)
			require.NoError(t, err, "failed to unmarshal %s severity", tt.name)

			assert.Equal(t, tt.severity, issue2.Severity)
			assert.Equal(t, issue.Message, issue2.Message)
		})
	}
}
