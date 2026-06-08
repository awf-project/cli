package agents

import (
	"encoding/json"

	"github.com/awf-project/cli/internal/domain/transcript"
)

type geminiToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type geminiNDJSONLine struct {
	Type      string           `json:"type"`
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []geminiToolCall `json:"toolCalls"`
}

func GeminiToContentBlocks(line []byte) []transcript.ContentBlock {
	var evt geminiNDJSONLine
	if err := json.Unmarshal(line, &evt); err != nil {
		return []transcript.ContentBlock{}
	}
	if evt.Type != "message" || evt.Role != "assistant" {
		return []transcript.ContentBlock{}
	}
	blocks := make([]transcript.ContentBlock, 0, 1+len(evt.ToolCalls))
	if evt.Content != "" {
		blocks = append(blocks, transcript.ContentBlock{
			Type:     transcript.BlockTypeText,
			Fidelity: transcript.FidelityAgentEmitted,
			Text:     evt.Content,
		})
	}
	for _, tc := range evt.ToolCalls {
		blocks = append(blocks, transcript.ContentBlock{
			Type:      transcript.BlockTypeToolUse,
			Fidelity:  transcript.FidelityAgentEmitted,
			ToolName:  tc.Name,
			ToolInput: tc.Arguments,
		})
	}
	return blocks
}
