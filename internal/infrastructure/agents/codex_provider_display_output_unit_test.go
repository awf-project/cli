package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// parseCodexStreamLine extracts assistant text from Codex CLI's `exec --json`
// output. Only `item.completed` events with `item.item_type:"assistant_message"`
// are surfaced.

func TestCodexProvider_parseCodexStreamLine_HappyPath(t *testing.T) {
	provider := NewCodexProvider()

	tests := []struct {
		name     string
		line     []byte
		wantText string
	}{
		{
			name:     "assistant_message item.completed surfaces text",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"hello"}}`),
			wantText: "hello",
		},
		{
			name:     "assistant_message with multi-line text preserved",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"line1\nline2"}}`),
			wantText: "line1\nline2",
		},
		{
			name:     "assistant_message with special characters",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"Response {braces} [brackets]"}}`),
			wantText: "Response {braces} [brackets]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCodexStreamLine(tt.line)
			assert.Equal(t, tt.wantText, got)
		})
	}
}

func TestCodexProvider_parseCodexStreamLine_ErrorPaths(t *testing.T) {
	provider := NewCodexProvider()

	tests := []struct {
		name string
		line []byte
	}{
		{
			name: "thread.started is skipped",
			line: []byte(`{"type":"thread.started","thread_id":"019d8872"}`),
		},
		{
			name: "turn.started is skipped",
			line: []byte(`{"type":"turn.started"}`),
		},
		{
			name: "turn.completed is skipped",
			line: []byte(`{"type":"turn.completed"}`),
		},
		{
			name: "error event is skipped",
			line: []byte(`{"type":"error","message":"Reconnecting..."}`),
		},
		{
			name: "reasoning item is skipped",
			line: []byte(`{"type":"item.completed","item":{"item_type":"reasoning","text":"thinking"}}`),
		},
		{
			name: "tool_call item is skipped",
			line: []byte(`{"type":"item.completed","item":{"item_type":"tool_call","name":"read_file"}}`),
		},
		{
			name: "malformed JSON returns empty",
			line: []byte(`{"type":"item.completed","item":{"item_type":`),
		},
		{
			name: "empty line returns empty",
			line: []byte(``),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCodexStreamLine(tt.line)
			assert.Equal(t, "", got)
		})
	}
}
