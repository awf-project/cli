package acp

import "context"

// MessageType identifies the kind of agent output carried by a Message.
type MessageType string

const (
	MsgAgentMessageChunk MessageType = "agent_message_chunk"
	MsgAgentThoughtChunk MessageType = "agent_thought_chunk"
	MsgToolCall          MessageType = "tool_call"
	MsgToolCallUpdate    MessageType = "tool_call_update"
)

// Message carries a single agent-stream chunk or tool-call event from the renderer
// to the ACP peer. Shapes are pinned by data-model.md.
// JSON tags use camelCase to match the ACP wire protocol (FR-004).
type Message struct {
	Type    MessageType `json:"type"`
	StepID  string      `json:"stepId"`
	Seq     uint64      `json:"seq"`
	Content string      `json:"content"`
	ToolID  string      `json:"toolId,omitempty"`
	Tool    string      `json:"tool,omitempty"`
}

// Sender transports a Message to the ACP peer. The ctx carries the workflow's
// cancellation signal so a peer that disconnects (stdin EOF / signal) stops the
// emission instead of writing to a potentially dead stdout.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
