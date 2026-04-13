package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// T004: parseClaudeStreamLine extracts displayable text from Claude CLI's stream-json
// output. Claude CLI (claude -p --output-format stream-json --verbose) emits one JSON
// object per line; the "assistant" event carries message.content[] blocks whose {type,text}
// entries contain the text to surface. Other event types (system, result, rate_limit_event)
// are ignored — result.result is consumed separately by extractResultEvent.

func TestClaudeProvider_parseClaudeStreamLine_HappyPath(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name     string
		line     []byte
		wantText string
	}{
		{
			name:     "assistant message with single text block",
			line:     []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello, world!"}]}}`),
			wantText: "Hello, world!",
		},
		{
			name:     "assistant message with special characters",
			line:     []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Response {braces} [brackets]"}]}}`),
			wantText: "Response {braces} [brackets]",
		},
		{
			name:     "assistant message with multiple text blocks joined by newline",
			line:     []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"first"},{"type":"text","text":"second"}]}}`),
			wantText: "first\nsecond",
		},
		{
			name:     "tool_use blocks are skipped, text blocks preserved",
			line:     []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1"},{"type":"text","text":"after tool"}]}}`),
			wantText: "after tool",
		},
		{
			name:     "empty text block returns empty",
			line:     []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":""}]}}`),
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseClaudeStreamLine(tt.line)
			assert.Equal(t, tt.wantText, got)
		})
	}
}

func TestClaudeProvider_parseClaudeStreamLine_ErrorPaths(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name     string
		line     []byte
		wantText string
	}{
		{
			name:     "malformed JSON",
			line:     []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"incomplete`),
			wantText: "",
		},
		{
			name:     "system event (ignored)",
			line:     []byte(`{"type":"system","subtype":"hook_started","session_id":"abc"}`),
			wantText: "",
		},
		{
			name:     "result event (ignored, consumed separately)",
			line:     []byte(`{"type":"result","result":"final answer"}`),
			wantText: "",
		},
		{
			name:     "rate_limit_event (ignored)",
			line:     []byte(`{"type":"rate_limit_event","reset_at":"2026-04-13T22:00:00Z"}`),
			wantText: "",
		},
		{
			name:     "assistant without message field",
			line:     []byte(`{"type":"assistant"}`),
			wantText: "",
		},
		{
			name:     "assistant with empty content array",
			line:     []byte(`{"type":"assistant","message":{"content":[]}}`),
			wantText: "",
		},
		{
			name:     "assistant with only tool_use blocks",
			line:     []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1"}]}}`),
			wantText: "",
		},
		{
			name:     "empty line",
			line:     []byte(``),
			wantText: "",
		},
		{
			name:     "plain text (not JSON)",
			line:     []byte(`this is not JSON`),
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseClaudeStreamLine(tt.line)
			assert.Equal(t, tt.wantText, got)
		})
	}
}
