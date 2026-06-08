package agents

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAICompatibleToContentBlocks(t *testing.T) {
	tests := []struct {
		name     string
		line     []byte
		validate func(t *testing.T, blocks []transcript.ContentBlock)
	}{
		{
			name: "delta with content only",
			line: []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"content":"hello"}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, transcript.BlockTypeText, blocks[0].Type)
				assert.Equal(t, transcript.FidelityAgentEmitted, blocks[0].Fidelity)
				assert.Equal(t, "hello", blocks[0].Text)
			},
		},
		{
			name: "delta with tool call only",
			line: []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"tool_calls":[{"id":"t1","function":{"name":"bash","arguments":"{}"}}]}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, transcript.BlockTypeToolUse, blocks[0].Type)
				assert.Equal(t, transcript.FidelityAgentEmitted, blocks[0].Fidelity)
				assert.Equal(t, "bash", blocks[0].ToolName)
				assert.Equal(t, "t1", blocks[0].ToolID)
			},
		},
		{
			name: "delta with content and single tool call",
			line: []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"content":"hi","tool_calls":[{"id":"t1","function":{"name":"n","arguments":"{}"}}]}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 2)
				assert.Equal(t, transcript.BlockTypeText, blocks[0].Type)
				assert.Equal(t, "hi", blocks[0].Text)
				assert.Equal(t, transcript.BlockTypeToolUse, blocks[1].Type)
				assert.Equal(t, "n", blocks[1].ToolName)
				assert.Equal(t, "t1", blocks[1].ToolID)
			},
		},
		{
			name: "delta with content and multiple tool calls",
			line: []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"content":"text","tool_calls":[{"id":"t1","function":{"name":"tool1","arguments":"{}"}},{"id":"t2","function":{"name":"tool2","arguments":"{\"key\":\"value\"}"}}]}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 3)
				assert.Equal(t, transcript.BlockTypeText, blocks[0].Type)
				assert.Equal(t, transcript.BlockTypeToolUse, blocks[1].Type)
				assert.Equal(t, "tool1", blocks[1].ToolName)
				assert.Equal(t, "t1", blocks[1].ToolID)
				assert.Equal(t, transcript.BlockTypeToolUse, blocks[2].Type)
				assert.Equal(t, "tool2", blocks[2].ToolName)
				assert.Equal(t, "t2", blocks[2].ToolID)
			},
		},
		{
			name: "message fallback when delta is empty",
			line: []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{},"message":{"content":"fallback"}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, transcript.BlockTypeText, blocks[0].Type)
				assert.Equal(t, "fallback", blocks[0].Text)
			},
		},
		{
			name: "message fallback with tool call",
			line: []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{},"message":{"tool_calls":[{"id":"t3","function":{"name":"fallback_tool","arguments":"{}"}}]}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, transcript.BlockTypeToolUse, blocks[0].Type)
				assert.Equal(t, "fallback_tool", blocks[0].ToolName)
				assert.Equal(t, "t3", blocks[0].ToolID)
			},
		},
		{
			name: "delta preferred over message",
			line: []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"content":"delta_content"},"message":{"content":"message_content"}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, "delta_content", blocks[0].Text)
			},
		},
		{
			name: "empty object",
			line: []byte(`{"object":"","choices":[{"delta":{"content":"text"}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "empty choices array",
			line: []byte(`{"object":"chat.completion.chunk","choices":[]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "missing object field",
			line: []byte(`{"choices":[{"delta":{"content":"text"}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "missing choices field",
			line: []byte(`{"object":"chat.completion.chunk"}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "delta and message both empty",
			line: []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{},"message":{}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "empty content string",
			line: []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"content":""}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "tool call with empty name",
			line: []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"tool_calls":[{"id":"t1","function":{"name":"","arguments":"{}"}}]}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, transcript.BlockTypeToolUse, blocks[0].Type)
				assert.Empty(t, blocks[0].ToolName)
				assert.Equal(t, "t1", blocks[0].ToolID)
			},
		},
		{
			name: "tool call with complex arguments",
			line: []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"tool_calls":[{"id":"t1","function":{"name":"tool","arguments":"{\"nested\":{\"key\":\"value\"}}"}}]}}]}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, "tool", blocks[0].ToolName)
				assert.Equal(t, "t1", blocks[0].ToolID)
			},
		},
		{
			name: "malformed json invalid utf8",
			line: []byte{0xff, 0xfe},
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "malformed json truncated",
			line: []byte(`{"object":"chat.completion.chunk","choices":`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "malformed json not json",
			line: []byte(`not json`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "empty input",
			line: []byte(``),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := OpenAICompatibleToContentBlocks(tt.line)
			tt.validate(t, blocks)
		})
	}
}

func TestOpenAICompatibleToContentBlocks_DeltaContentAndToolCall(t *testing.T) {
	line := []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"content":"hi","tool_calls":[{"id":"t1","function":{"name":"n","arguments":"{}"}}]}}]}`)
	blocks := OpenAICompatibleToContentBlocks(line)

	require.Len(t, blocks, 2)
	assert.Equal(t, transcript.BlockTypeText, blocks[0].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, blocks[0].Fidelity)
	assert.Equal(t, "hi", blocks[0].Text)

	assert.Equal(t, transcript.BlockTypeToolUse, blocks[1].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, blocks[1].Fidelity)
	assert.Equal(t, "n", blocks[1].ToolName)
	assert.Equal(t, "t1", blocks[1].ToolID)
}

func TestOpenAICompatibleToContentBlocks_MessageFallback(t *testing.T) {
	line := []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{},"message":{"content":"fallback"}}]}`)
	blocks := OpenAICompatibleToContentBlocks(line)

	require.Len(t, blocks, 1)
	assert.Equal(t, transcript.BlockTypeText, blocks[0].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, blocks[0].Fidelity)
	assert.Equal(t, "fallback", blocks[0].Text)
}

func TestOpenAICompatibleToContentBlocks_DanglingToolUse(t *testing.T) {
	line := []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"tool_calls":[{"id":"t1","function":{"name":"bash","arguments":"{}"}}]}}]}`)
	blocks := OpenAICompatibleToContentBlocks(line)

	require.Len(t, blocks, 1)
	assert.Equal(t, transcript.BlockTypeToolUse, blocks[0].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, blocks[0].Fidelity)
	assert.Equal(t, "bash", blocks[0].ToolName)
	assert.Equal(t, "t1", blocks[0].ToolID)
}

func TestOpenAICompatibleToContentBlocks_NoObjectOrChoices(t *testing.T) {
	testCases := []struct {
		name string
		line []byte
	}{
		{
			name: "empty object",
			line: []byte(`{"object":"","choices":[{"delta":{"content":"text"}}]}`),
		},
		{
			name: "empty choices",
			line: []byte(`{"object":"chat.completion.chunk","choices":[]}`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blocks := OpenAICompatibleToContentBlocks(tc.line)
			assert.Empty(t, blocks)
		})
	}
}
