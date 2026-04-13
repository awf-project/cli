package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// parseGeminiStreamLine extracts assistant text from Gemini CLI's stream-json
// output. Only `message` events with `role:"assistant"` are surfaced.

func TestGeminiProvider_parseGeminiStreamLine_HappyPath(t *testing.T) {
	provider := NewGeminiProvider()

	tests := []struct {
		name     string
		line     []byte
		wantText string
	}{
		{
			name:     "assistant message surfaces content",
			line:     []byte(`{"type":"message","timestamp":"2026-04-13T20:04:45.219Z","role":"assistant","content":"hello","delta":true}`),
			wantText: "hello",
		},
		{
			name:     "assistant content with special chars preserved",
			line:     []byte(`{"type":"message","role":"assistant","content":"Response {braces} [brackets]"}`),
			wantText: "Response {braces} [brackets]",
		},
		{
			name:     "assistant delta chunk surfaces content",
			line:     []byte(`{"type":"message","role":"assistant","content":"partial ","delta":true}`),
			wantText: "partial ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseGeminiStreamLine(tt.line)
			assert.Equal(t, tt.wantText, got)
		})
	}
}

func TestGeminiProvider_parseGeminiStreamLine_ErrorPaths(t *testing.T) {
	provider := NewGeminiProvider()

	tests := []struct {
		name string
		line []byte
	}{
		{
			name: "user message (echo) is skipped",
			line: []byte(`{"type":"message","role":"user","content":"reply with: ok"}`),
		},
		{
			name: "init event is skipped",
			line: []byte(`{"type":"init","session_id":"abc","model":"auto-gemini-3"}`),
		},
		{
			name: "result event is skipped",
			line: []byte(`{"type":"result","status":"success","stats":{"total_tokens":100}}`),
		},
		{
			name: "assistant missing role is skipped",
			line: []byte(`{"type":"message","content":"orphan"}`),
		},
		{
			name: "malformed JSON returns empty",
			line: []byte(`{"type":"message","role":"assistant","content":`),
		},
		{
			name: "empty line returns empty",
			line: []byte(``),
		},
		{
			name: "plain text returns empty",
			line: []byte(`plain text`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseGeminiStreamLine(tt.line)
			assert.Equal(t, "", got)
		})
	}
}
