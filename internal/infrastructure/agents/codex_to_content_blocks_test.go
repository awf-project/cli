package agents

import (
	"encoding/json"
	"testing"

	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexToContentBlocks_HappyPathTextOnly(t *testing.T) {
	line := []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"hello"}}`)

	got := CodexToContentBlocks(line)

	require.Len(t, got, 1)
	assert.Equal(t, transcript.BlockTypeText, got[0].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, got[0].Fidelity)
	assert.Equal(t, "hello", got[0].Text)
}

func TestCodexToContentBlocks_AllItemKinds(t *testing.T) {
	tests := []struct {
		name      string
		line      []byte
		wantType  transcript.BlockType
		wantText  string
		wantTool  string
		wantInput string
	}{
		{
			name:     "agent_message via item_type",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"agent_message","text":"resp"}}`),
			wantType: transcript.BlockTypeText,
			wantText: "resp",
		},
		{
			name:     "assistant_message via type field",
			line:     []byte(`{"type":"item.completed","item":{"type":"assistant_message","text":"out"}}`),
			wantType: transcript.BlockTypeText,
			wantText: "out",
		},
		{
			name:      "function_call via item_type",
			line:      []byte(`{"type":"item.completed","item":{"item_type":"function_call","name":"bash","arguments":"{\"cmd\":\"ls\"}"}}`),
			wantType:  transcript.BlockTypeToolUse,
			wantTool:  "bash",
			wantInput: `{"cmd":"ls"}`,
		},
		{
			name:      "command_execution via type field",
			line:      []byte(`{"type":"item.completed","item":{"type":"command_execution","name":"grep","arguments":"{}"}}`),
			wantType:  transcript.BlockTypeToolUse,
			wantTool:  "grep",
			wantInput: "{}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CodexToContentBlocks(tc.line)

			require.Len(t, got, 1)
			assert.Equal(t, tc.wantType, got[0].Type)
			assert.Equal(t, transcript.FidelityAgentEmitted, got[0].Fidelity)
			if tc.wantType == transcript.BlockTypeText {
				assert.Equal(t, tc.wantText, got[0].Text)
			} else {
				assert.Equal(t, tc.wantTool, got[0].ToolName)
				assert.Equal(t, "", got[0].ToolID) // Codex emits no tool-call ids
				assert.Equal(t, tc.wantInput, got[0].ToolInput)
			}
		})
	}
}

func TestCodexToContentBlocks_EmbeddedNUL(t *testing.T) {
	// Build a JSON line with a raw NUL byte inside the text value; raw NUL is
	// not valid JSON so the mapper must escape it to \u0000 before unmarshal.
	prefix := []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"hello`)
	suffix := []byte(`world"}}`)
	line := make([]byte, 0, len(prefix)+1+len(suffix))
	line = append(line, prefix...)
	line = append(line, 0x00)
	line = append(line, suffix...)

	got := CodexToContentBlocks(line)

	require.Len(t, got, 1)
	assert.Equal(t, transcript.BlockTypeText, got[0].Type)
	assert.Equal(t, "hello\x00world", got[0].Text)

	marshaled, err := got[0].MarshalJSON()
	require.NoError(t, err)
	assert.Contains(t, string(marshaled), "\\u0000")

	var recovered transcript.ContentBlock
	require.NoError(t, json.Unmarshal(marshaled, &recovered))
	assert.Equal(t, "hello\x00world", recovered.Text)
}

func TestCodexToContentBlocks_DanglingToolUse(t *testing.T) {
	line := []byte(`{"type":"item.completed","item":{"item_type":"function_call","name":"shell","arguments":"{\"cmd\":\"rm -rf /tmp/x\"}"}}`)

	got := CodexToContentBlocks(line)

	require.Len(t, got, 1)
	assert.Equal(t, transcript.BlockTypeToolUse, got[0].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, got[0].Fidelity)
	assert.Equal(t, "shell", got[0].ToolName)
	assert.Equal(t, "", got[0].ToolID)
}

func TestCodexToContentBlocks_MalformedJSON(t *testing.T) {
	got := CodexToContentBlocks([]byte(`{not valid json}`))
	assert.Equal(t, []transcript.ContentBlock{}, got)
}

func TestCodexToContentBlocks_WrongDiscriminator(t *testing.T) {
	tests := []struct {
		name string
		line []byte
	}{
		{"wrong type field", []byte(`{"type":"item.created","item":{"item_type":"assistant_message","text":"hi"}}`)},
		{"nil item", []byte(`{"type":"item.completed"}`)},
		{"empty object", []byte(`{}`)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CodexToContentBlocks(tc.line)
			assert.Equal(t, []transcript.ContentBlock{}, got)
		})
	}
}

func TestCodexToContentBlocks_NeverReturnsNil(t *testing.T) {
	cases := [][]byte{
		[]byte(`{}`),
		[]byte(`{"type":"item.completed"}`),
		[]byte(`{bad json`),
		nil,
	}
	for _, line := range cases {
		got := CodexToContentBlocks(line)
		assert.NotNil(t, got)
	}
}
