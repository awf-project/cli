package agents

import (
	"encoding/json"

	"github.com/awf-project/cli/internal/domain/transcript"
)

type claudeContentElement struct {
	Type     string         `json:"type"`
	Text     string         `json:"text"`
	Thinking string         `json:"thinking"`
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Input    map[string]any `json:"input"`
}

type claudeNDJSONLine struct {
	Type    string `json:"type"`
	Message *struct {
		Content []claudeContentElement `json:"content"`
	} `json:"message"`
}

func ClaudeToContentBlocks(line []byte) []transcript.ContentBlock {
	var evt claudeNDJSONLine
	if err := json.Unmarshal(line, &evt); err != nil {
		return []transcript.ContentBlock{}
	}
	if evt.Type != "assistant" || evt.Message == nil {
		return []transcript.ContentBlock{}
	}
	blocks := make([]transcript.ContentBlock, 0, len(evt.Message.Content))
	for _, el := range evt.Message.Content {
		switch el.Type {
		case "text":
			if el.Text == "" {
				continue
			}
			blocks = append(blocks, transcript.ContentBlock{
				Type:     transcript.BlockTypeText,
				Fidelity: transcript.FidelityAgentEmitted,
				Text:     el.Text,
			})
		case "thinking":
			blocks = append(blocks, transcript.ContentBlock{
				Type:     transcript.BlockTypeThinking,
				Fidelity: transcript.FidelityAgentEmitted,
				Thinking: el.Thinking,
			})
		case "tool_use":
			blocks = append(blocks, transcript.ContentBlock{
				Type:      transcript.BlockTypeToolUse,
				Fidelity:  transcript.FidelityAgentEmitted,
				ToolName:  el.Name,
				ToolID:    el.ID,
				ToolInput: el.Input,
			})
		}
	}
	return blocks
}
