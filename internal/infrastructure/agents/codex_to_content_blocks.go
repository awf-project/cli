package agents

import (
	"bytes"
	"encoding/json"

	"github.com/awf-project/cli/internal/domain/transcript"
)

type codexItem struct {
	ItemType  string `json:"item_type"`
	Kind      string `json:"type"`
	Text      string `json:"text"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type codexNDJSONLine struct {
	Type string     `json:"type"`
	Item *codexItem `json:"item"`
}

// CodexToContentBlocks maps a single Codex NDJSON line to transcript ContentBlocks.
// Raw NUL bytes (0x00) are escaped to the six-byte JSON unicode sequence before
// unmarshal, matching the behavior in codex_provider.go:343.
func CodexToContentBlocks(line []byte) []transcript.ContentBlock {
	sanitized := bytes.ReplaceAll(line, []byte{0x00}, []byte{0x5c, 0x75, 0x30, 0x30, 0x30, 0x30})

	var evt codexNDJSONLine
	if err := json.Unmarshal(sanitized, &evt); err != nil {
		return []transcript.ContentBlock{}
	}
	if evt.Type != "item.completed" || evt.Item == nil {
		return []transcript.ContentBlock{}
	}

	itemKind := evt.Item.ItemType
	if itemKind == "" {
		itemKind = evt.Item.Kind
	}

	switch itemKind {
	case "assistant_message", "agent_message":
		return []transcript.ContentBlock{{
			Type:     transcript.BlockTypeText,
			Fidelity: transcript.FidelityAgentEmitted,
			Text:     evt.Item.Text,
		}}
	case "function_call", "command_execution":
		return []transcript.ContentBlock{{
			Type:      transcript.BlockTypeToolUse,
			Fidelity:  transcript.FidelityAgentEmitted,
			ToolName:  evt.Item.Name,
			ToolID:    "",
			ToolInput: evt.Item.Arguments,
		}}
	}
	return []transcript.ContentBlock{}
}
