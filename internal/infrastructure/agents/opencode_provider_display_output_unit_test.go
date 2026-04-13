package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// parseOpencodeStreamLine extracts assistant text from OpenCode CLI's
// `run --format json` output. Only `text` events (with part.text) are surfaced;
// step_start / step_finish / metadata events are skipped.

func TestOpenCodeProvider_parseOpencodeStreamLine_HappyPath(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name     string
		line     []byte
		wantText string
	}{
		{
			name:     "text event surfaces part.text",
			line:     []byte(`{"type":"text","timestamp":1776110705680,"sessionID":"ses_abc","part":{"id":"prt_1","type":"text","text":"ok"}}`),
			wantText: "ok",
		},
		{
			name:     "text event with special chars preserved",
			line:     []byte(`{"type":"text","part":{"text":"Response {braces} [brackets]"}}`),
			wantText: "Response {braces} [brackets]",
		},
		{
			name:     "text event with multi-line content",
			line:     []byte(`{"type":"text","part":{"text":"line1\nline2"}}`),
			wantText: "line1\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseOpencodeStreamLine(tt.line)
			assert.Equal(t, tt.wantText, got)
		})
	}
}

func TestOpenCodeProvider_parseOpencodeStreamLine_ErrorPaths(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name string
		line []byte
	}{
		{
			name: "step_start is skipped",
			line: []byte(`{"type":"step_start","sessionID":"ses_abc","part":{"type":"step-start"}}`),
		},
		{
			name: "step_finish is skipped",
			line: []byte(`{"type":"step_finish","sessionID":"ses_abc","part":{"type":"step-finish","tokens":{"total":100}}}`),
		},
		{
			name: "text event with empty text returns empty",
			line: []byte(`{"type":"text","part":{"text":""}}`),
		},
		{
			name: "text event without part is empty",
			line: []byte(`{"type":"text"}`),
		},
		{
			name: "malformed JSON returns empty",
			line: []byte(`{"type":"text","part":{"text":`),
		},
		{
			name: "empty line returns empty",
			line: []byte(``),
		},
		{
			name: "plain text returns empty",
			line: []byte(`not json`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseOpencodeStreamLine(tt.line)
			assert.Equal(t, "", got)
		})
	}
}
