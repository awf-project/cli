package agents

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiToContentBlocks_TextAndToolCall(t *testing.T) {
	line := []byte(`{"type":"message","role":"assistant","content":"hi","toolCalls":[{"name":"n","arguments":{"k":1}}]}`)

	got := GeminiToContentBlocks(line)

	require.Len(t, got, 2)

	assert.Equal(t, transcript.BlockTypeText, got[0].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, got[0].Fidelity)
	assert.Equal(t, "hi", got[0].Text)

	assert.Equal(t, transcript.BlockTypeToolUse, got[1].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, got[1].Fidelity)
	assert.Equal(t, "n", got[1].ToolName)
	assert.Equal(t, "", got[1].ToolID)
	assert.Equal(t, map[string]any{"k": float64(1)}, got[1].ToolInput)
}

func TestGeminiToContentBlocks_WrongDiscriminator(t *testing.T) {
	tests := []struct {
		name string
		line []byte
	}{
		{"user role", []byte(`{"type":"message","role":"user","content":"hi"}`)},
		{"wrong type", []byte(`{"type":"system","role":"assistant","content":"hi"}`)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GeminiToContentBlocks(tc.line)
			assert.Equal(t, []transcript.ContentBlock{}, got)
		})
	}
}

func TestGeminiToContentBlocks_EmptyContent(t *testing.T) {
	line := []byte(`{"type":"message","role":"assistant","content":"","toolCalls":[]}`)

	got := GeminiToContentBlocks(line)

	assert.Equal(t, []transcript.ContentBlock{}, got)
}

func TestGeminiToContentBlocks_MalformedJSON(t *testing.T) {
	got := GeminiToContentBlocks([]byte(`{not valid json}`))
	assert.Equal(t, []transcript.ContentBlock{}, got)
}

func TestGeminiToContentBlocks_AllFidelitiesAreAgentEmitted(t *testing.T) {
	line := []byte(`{"type":"message","role":"assistant","content":"text","toolCalls":[{"name":"tool","arguments":{}}]}`)

	got := GeminiToContentBlocks(line)

	require.Len(t, got, 2)
	for _, block := range got {
		assert.Equal(t, transcript.FidelityAgentEmitted, block.Fidelity)
	}
}

func TestGeminiToContentBlocks_NeverReturnsNil(t *testing.T) {
	cases := [][]byte{
		[]byte(`{}`),
		[]byte(`{"type":"message","role":"assistant","content":"","toolCalls":[]}`),
		[]byte(`{bad json`),
		nil,
	}
	for _, line := range cases {
		got := GeminiToContentBlocks(line)
		assert.NotNil(t, got)
	}
}

func TestGeminiToContentBlocks_OnlyToolCalls(t *testing.T) {
	line := []byte(`{"type":"message","role":"assistant","content":"","toolCalls":[{"name":"bash","arguments":{"cmd":"ls"}}]}`)

	got := GeminiToContentBlocks(line)

	require.Len(t, got, 1)
	assert.Equal(t, transcript.BlockTypeToolUse, got[0].Type)
	assert.Equal(t, "bash", got[0].ToolName)
	assert.Equal(t, "", got[0].ToolID)
}
