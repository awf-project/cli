package agents

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopilotToContentBlocks(t *testing.T) {
	tests := []struct {
		name      string
		line      []byte
		wantCount int
		validate  func(t *testing.T, blocks []transcript.ContentBlock)
	}{
		{
			name: "assistant message with content",
			line: []byte(`{"type":"assistant.message","data":{"content":"hi"}}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, transcript.BlockTypeText, blocks[0].Type)
				assert.Equal(t, transcript.FidelityAgentEmitted, blocks[0].Fidelity)
				assert.Equal(t, "hi", blocks[0].Text)
			},
		},
		{
			name: "assistant message with multiline content",
			line: []byte(`{"type":"assistant.message","data":{"content":"line1\nline2"}}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, "line1\nline2", blocks[0].Text)
			},
		},
		{
			name: "assistant message with empty content",
			line: []byte(`{"type":"assistant.message","data":{"content":""}}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "assistant message missing content field",
			line: []byte(`{"type":"assistant.message","data":{}}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "tool execution start with tool name",
			line: []byte(`{"type":"tool.execution_start","data":{"toolName":"bash"}}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, transcript.BlockTypeToolUse, blocks[0].Type)
				assert.Equal(t, transcript.FidelityAgentEmitted, blocks[0].Fidelity)
				assert.Equal(t, "bash", blocks[0].ToolName)
				assert.Empty(t, blocks[0].ToolID)
			},
		},
		{
			name: "tool execution start with empty tool name",
			line: []byte(`{"type":"tool.execution_start","data":{"toolName":""}}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, transcript.BlockTypeToolUse, blocks[0].Type)
				assert.Empty(t, blocks[0].ToolName)
			},
		},
		{
			name: "tool execution start missing tool name",
			line: []byte(`{"type":"tool.execution_start","data":{}}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				require.Len(t, blocks, 1)
				assert.Equal(t, transcript.BlockTypeToolUse, blocks[0].Type)
				assert.Empty(t, blocks[0].ToolName)
			},
		},
		{
			name: "unknown event type",
			line: []byte(`{"type":"unknown.event","data":{"content":"data"}}`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
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
			line: []byte(`{"type":"assistant.message","data":`),
			validate: func(t *testing.T, blocks []transcript.ContentBlock) {
				assert.Empty(t, blocks)
			},
		},
		{
			name: "malformed json invalid structure",
			line: []byte(`not json at all`),
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
			blocks := CopilotToContentBlocks(tt.line)
			tt.validate(t, blocks)
		})
	}
}

func TestCopilotToContentBlocks_AssistantMessage(t *testing.T) {
	line := []byte(`{"type":"assistant.message","data":{"content":"hi"}}`)
	blocks := CopilotToContentBlocks(line)

	require.Len(t, blocks, 1)
	assert.Equal(t, transcript.BlockTypeText, blocks[0].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, blocks[0].Fidelity)
	assert.Equal(t, "hi", blocks[0].Text)
}

func TestCopilotToContentBlocks_ToolExecutionStart(t *testing.T) {
	line := []byte(`{"type":"tool.execution_start","data":{"toolName":"bash"}}`)
	blocks := CopilotToContentBlocks(line)

	require.Len(t, blocks, 1)
	assert.Equal(t, transcript.BlockTypeToolUse, blocks[0].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, blocks[0].Fidelity)
	assert.Equal(t, "bash", blocks[0].ToolName)
}

func TestCopilotToContentBlocks_UnknownType(t *testing.T) {
	line := []byte(`{"type":"x"}`)
	blocks := CopilotToContentBlocks(line)

	assert.Empty(t, blocks)
}

func TestCopilotToContentBlocks_MalformedJSON(t *testing.T) {
	line := []byte(`{invalid}`)
	blocks := CopilotToContentBlocks(line)

	assert.Empty(t, blocks)
	// ensure no panic occurred
}
