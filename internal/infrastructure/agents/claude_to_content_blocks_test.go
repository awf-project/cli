package agents

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeToContentBlocks_TextThinkingToolUse(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"a"},{"type":"thinking","thinking":"t"},{"type":"tool_use","id":"x","name":"n","input":{}}]}}`)

	got := ClaudeToContentBlocks(line)

	require.Len(t, got, 3)

	assert.Equal(t, transcript.BlockTypeText, got[0].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, got[0].Fidelity)
	assert.Equal(t, "a", got[0].Text)

	assert.Equal(t, transcript.BlockTypeThinking, got[1].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, got[1].Fidelity)
	assert.Equal(t, "t", got[1].Thinking)

	assert.Equal(t, transcript.BlockTypeToolUse, got[2].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, got[2].Fidelity)
	assert.Equal(t, "n", got[2].ToolName)
	assert.Equal(t, "x", got[2].ToolID)
}

func TestClaudeToContentBlocks_DanglingToolUseWithoutResult(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"dangling","name":"bash","input":{"cmd":"ls"}}]}}`)

	got := ClaudeToContentBlocks(line)

	require.Len(t, got, 1)
	assert.Equal(t, transcript.BlockTypeToolUse, got[0].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, got[0].Fidelity)
	assert.Equal(t, "dangling", got[0].ToolID)
	assert.Equal(t, "bash", got[0].ToolName)
}

func TestClaudeToContentBlocks_NonAssistantLineReturnsEmpty(t *testing.T) {
	tests := []struct {
		name string
		line []byte
	}{
		{"user type", []byte(`{"type":"user","message":{"content":[{"type":"text","text":"hi"}]}}`)},
		{"missing message", []byte(`{"type":"assistant"}`)},
		{"system type", []byte(`{"type":"system","session_id":"abc"}`)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClaudeToContentBlocks(tc.line)
			assert.Equal(t, []transcript.ContentBlock{}, got)
		})
	}
}

func TestClaudeToContentBlocks_MalformedJSON(t *testing.T) {
	got := ClaudeToContentBlocks([]byte(`{not valid json}`))
	assert.Equal(t, []transcript.ContentBlock{}, got)
}

func TestClaudeToContentBlocks_AllFidelitiesAreAgentEmitted(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"hello"},{"type":"thinking","thinking":"..."},{"type":"tool_use","id":"1","name":"grep","input":{}}]}}`)

	got := ClaudeToContentBlocks(line)

	require.Len(t, got, 3)
	for _, block := range got {
		assert.Equal(t, transcript.FidelityAgentEmitted, block.Fidelity)
	}
}

func TestClaudeToContentBlocks_UnknownContentTypeSkipped(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"unknown_future_type"},{"type":"text","text":"keep"}]}}`)

	got := ClaudeToContentBlocks(line)

	require.Len(t, got, 1)
	assert.Equal(t, transcript.BlockTypeText, got[0].Type)
	assert.Equal(t, "keep", got[0].Text)
}

func TestClaudeToContentBlocks_EmptyContentArrayReturnsEmpty(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[]}}`)

	got := ClaudeToContentBlocks(line)

	assert.Equal(t, []transcript.ContentBlock{}, got)
}

func TestClaudeToContentBlocks_NeverReturnsNil(t *testing.T) {
	cases := [][]byte{
		[]byte(`{}`),
		[]byte(`{"type":"assistant","message":{"content":[]}}`),
		[]byte(`{bad json`),
		nil,
	}
	for _, line := range cases {
		got := ClaudeToContentBlocks(line)
		assert.NotNil(t, got)
	}
}
