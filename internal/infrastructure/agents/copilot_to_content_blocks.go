package agents

import (
	"encoding/json"

	"github.com/awf-project/cli/internal/domain/transcript"
)

type copilotDisplayLine struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// CopilotToContentBlocks maps a single Copilot NDJSON line to transcript ContentBlocks.
// Mirrors the event-typed shape consumed by parseCopilotDisplayEvents in copilot_provider.go.
func CopilotToContentBlocks(line []byte) []transcript.ContentBlock {
	if len(line) == 0 {
		return []transcript.ContentBlock{}
	}
	var event copilotDisplayLine
	if err := json.Unmarshal(line, &event); err != nil {
		return []transcript.ContentBlock{}
	}
	switch event.Type {
	case "assistant.message":
		var content string
		if s, ok := event.Data["content"].(string); ok {
			content = s
		}
		if content == "" {
			return []transcript.ContentBlock{}
		}
		return []transcript.ContentBlock{{
			Type:     transcript.BlockTypeText,
			Fidelity: transcript.FidelityAgentEmitted,
			Text:     content,
		}}
	case "tool.execution_start":
		var toolName string
		if s, ok := event.Data["toolName"].(string); ok {
			toolName = s
		}
		return []transcript.ContentBlock{{
			Type:     transcript.BlockTypeToolUse,
			Fidelity: transcript.FidelityAgentEmitted,
			ToolName: toolName,
		}}
	default:
		return []transcript.ContentBlock{}
	}
}
