package transcript

import (
	"encoding/json"
	"errors"
	"fmt"
)

type BlockType string

const (
	BlockTypeText       BlockType = "text"
	BlockTypeThinking   BlockType = "thinking"
	BlockTypeToolUse    BlockType = "tool_use"
	BlockTypeToolResult BlockType = "tool_result"
	BlockTypeCommand    BlockType = "command"
	BlockTypeStream     BlockType = "stream"
)

type Fidelity string

const (
	FidelityRouter       Fidelity = "router"
	FidelityAgentEmitted Fidelity = "agent_emitted"
)

var ErrUnknownBlockType = errors.New("unknown block type")

type ContentBlock struct {
	Type     BlockType `json:"type"`
	Fidelity Fidelity  `json:"fidelity"`

	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`

	ToolName    string `json:"tool_name,omitempty"`
	ToolID      string `json:"tool_id,omitempty"`
	ToolInput   any    `json:"tool_input,omitempty"`
	ToolContent any    `json:"tool_content,omitempty"`

	Command string `json:"command,omitempty"`
	Chunk   string `json:"chunk,omitempty"`
}

func (b ContentBlock) MarshalJSON() ([]byte, error) { //nolint:gocritic // hugeParam: value receiver required so json.Marshal(block) invokes custom marshaler
	type wire struct {
		Type        BlockType `json:"type"`
		Fidelity    Fidelity  `json:"fidelity"`
		Text        string    `json:"text,omitempty"`
		Thinking    string    `json:"thinking,omitempty"`
		ToolName    string    `json:"tool_name,omitempty"`
		ToolID      string    `json:"tool_id,omitempty"`
		ToolInput   any       `json:"tool_input,omitempty"`
		ToolContent any       `json:"tool_content,omitempty"`
		Command     string    `json:"command,omitempty"`
		Chunk       string    `json:"chunk,omitempty"`
	}
	w := wire(b) //nolint:govet // wire has identical field layout; conversion is safe
	data, err := json.Marshal(w)
	if err != nil {
		return nil, fmt.Errorf("marshaling content block: %w", err)
	}
	return data, nil
}

func (b *ContentBlock) UnmarshalJSON(data []byte) error {
	type rawBlock ContentBlock
	var raw rawBlock
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decoding content block: %w", err)
	}
	if !ValidBlockType(raw.Type) {
		return fmt.Errorf("%w: %s", ErrUnknownBlockType, raw.Type)
	}
	*b = ContentBlock(raw)
	return nil
}

func ValidBlockType(bt BlockType) bool {
	switch bt {
	case BlockTypeText, BlockTypeThinking, BlockTypeToolUse, BlockTypeToolResult, BlockTypeCommand, BlockTypeStream:
		return true
	default:
		return false
	}
}

func ValidFidelity(f Fidelity) bool {
	switch f {
	case FidelityRouter, FidelityAgentEmitted:
		return true
	default:
		return false
	}
}
