package agents

import (
	"encoding/json"

	"github.com/awf-project/cli/internal/domain/transcript"
)

type openAICompatibleToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAICompatibleDeltaOrMessage struct {
	Content   string                     `json:"content"`
	ToolCalls []openAICompatibleToolCall `json:"tool_calls"`
}

type openAICompatibleChunk struct {
	Object  string `json:"object"`
	Choices []struct {
		Delta   openAICompatibleDeltaOrMessage `json:"delta"`
		Message openAICompatibleDeltaOrMessage `json:"message"`
	} `json:"choices"`
}

// OpenAICompatibleToContentBlocks maps a single OpenAI-compatible NDJSON line to transcript ContentBlocks.
// Mirrors the delta/message shape consumed by translateOpenAICompatibleDisplayEvents in openai_compatible_provider.go.
func OpenAICompatibleToContentBlocks(line []byte) []transcript.ContentBlock {
	if len(line) == 0 {
		return []transcript.ContentBlock{}
	}
	var chunk openAICompatibleChunk
	if err := json.Unmarshal(line, &chunk); err != nil {
		return []transcript.ContentBlock{}
	}
	if chunk.Object == "" || len(chunk.Choices) == 0 {
		return []transcript.ContentBlock{}
	}

	choice := chunk.Choices[0]
	src := choice.Delta
	if src.Content == "" && len(src.ToolCalls) == 0 {
		src = choice.Message
	}

	var blocks []transcript.ContentBlock
	if src.Content != "" {
		blocks = append(blocks, transcript.ContentBlock{
			Type:     transcript.BlockTypeText,
			Fidelity: transcript.FidelityAgentEmitted,
			Text:     src.Content,
		})
	}
	for _, tc := range src.ToolCalls {
		block := transcript.ContentBlock{
			Type:     transcript.BlockTypeToolUse,
			Fidelity: transcript.FidelityAgentEmitted,
			ToolName: tc.Function.Name,
			ToolID:   tc.ID,
		}
		if tc.Function.Arguments != "" {
			block.ToolInput = tc.Function.Arguments
		}
		blocks = append(blocks, block)
	}

	if blocks == nil {
		return []transcript.ContentBlock{}
	}
	return blocks
}
