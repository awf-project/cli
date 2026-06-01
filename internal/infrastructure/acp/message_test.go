package acp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMessageType_ConstantValues pins the wire string for each MessageType. These values
// are the ACP session/update variant kinds; a silent change would desynchronize the
// renderer from the protocol, so they are asserted explicitly.
func TestMessageType_ConstantValues(t *testing.T) {
	tests := []struct {
		constant MessageType
		want     string
	}{
		{MsgAgentMessageChunk, "agent_message_chunk"},
		{MsgAgentThoughtChunk, "agent_thought_chunk"},
		{MsgToolCall, "tool_call"},
		{MsgToolCallUpdate, "tool_call_update"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.constant))
		})
	}
}

// TestMessage_JSONRoundTrip verifies every field survives a marshal/unmarshal cycle for
// each MessageType, including the tool-call fields.
func TestMessage_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
	}{
		{
			name: "agent message chunk",
			msg:  Message{Type: MsgAgentMessageChunk, StepID: "step-1", Seq: 1, Content: "hello"},
		},
		{
			name: "agent thought chunk",
			msg:  Message{Type: MsgAgentThoughtChunk, StepID: "step-1", Seq: 2, Content: "thinking"},
		},
		{
			name: "tool call",
			msg:  Message{Type: MsgToolCall, StepID: "step-2", Seq: 3, Content: `{"path":"x"}`, ToolID: "t-1", Tool: "read"},
		},
		{
			name: "tool call update",
			msg:  Message{Type: MsgToolCallUpdate, StepID: "step-2", Seq: 4, Content: "done", ToolID: "t-1", Tool: "read"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			require.NoError(t, err)

			var got Message
			require.NoError(t, json.Unmarshal(data, &got))
			assert.Equal(t, tt.msg, got, "round-tripped Message should equal the original")
		})
	}
}

// TestMessage_JSONKeysAreCamelCase verifies that the ACP wire format uses camelCase keys,
// not PascalCase. A change to Go field names must not silently break the wire protocol.
func TestMessage_JSONKeysAreCamelCase(t *testing.T) {
	msg := Message{
		Type:    MsgToolCall,
		StepID:  "step-1",
		Seq:     42,
		Content: "arg",
		ToolID:  "t-99",
		Tool:    "bash",
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	raw := string(data)
	// Assert camelCase keys are present.
	assert.Contains(t, raw, `"type"`)
	assert.Contains(t, raw, `"stepId"`)
	assert.Contains(t, raw, `"seq"`)
	assert.Contains(t, raw, `"content"`)
	assert.Contains(t, raw, `"toolId"`)
	assert.Contains(t, raw, `"tool"`)
	// Assert PascalCase keys are absent.
	assert.NotContains(t, raw, `"Type"`)
	assert.NotContains(t, raw, `"StepID"`)
	assert.NotContains(t, raw, `"Seq"`)
	assert.NotContains(t, raw, `"Content"`)
	assert.NotContains(t, raw, `"ToolID"`)
	assert.NotContains(t, raw, `"Tool"`)
}

// TestMessage_JSONOmitsEmptyToolFields verifies that ToolID and Tool are omitted from
// the JSON when empty (omitempty), keeping non-tool messages compact.
func TestMessage_JSONOmitsEmptyToolFields(t *testing.T) {
	msg := Message{
		Type:    MsgAgentMessageChunk,
		StepID:  "step-1",
		Seq:     1,
		Content: "hello",
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	raw := string(data)
	assert.NotContains(t, raw, `"toolId"`, "empty ToolID must be omitted")
	assert.NotContains(t, raw, `"tool"`, "empty Tool must be omitted")
}

// senderSpy records the last message sent, proving the Sender interface is satisfiable.
type senderSpy struct{ last Message }

//nolint:gocritic // hugeParam: Send must match the Sender interface signature (value Message), so a pointer param is not an option.
func (s *senderSpy) Send(_ context.Context, msg Message) error {
	s.last = msg
	return nil
}

func TestSender_InterfaceContract(t *testing.T) {
	var s Sender = &senderSpy{}
	msg := Message{Type: MsgToolCall, StepID: "s", Seq: 7, ToolID: "id", Tool: "bash"}
	require.NoError(t, s.Send(context.Background(), msg))
	assert.Equal(t, msg, s.(*senderSpy).last)
}
