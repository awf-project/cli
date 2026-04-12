package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests for extractSessionID() and extractTextFromJSON() — claude CLI stream-json
// is NDJSON (one JSON object per line); the "result" event carries the final
// session_id and text payload.

func TestClaudeProvider_extractSessionID(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name    string
		output  string
		wantID  string
		wantErr bool
	}{
		{
			name:    "NDJSON result event carries session_id",
			output:  `{"type":"system","subtype":"init"}` + "\n" + `{"type":"result","subtype":"success","session_id":"claude-session-12345","result":"ok"}`,
			wantID:  "claude-session-12345",
			wantErr: false,
		},
		{
			name:    "NDJSON with uuid-like session_id in result event",
			output:  `{"type":"result","subtype":"success","session_id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","result":"response"}`,
			wantID:  "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			wantErr: false,
		},
		{
			name:    "NDJSON with numeric-looking session_id string",
			output:  `{"type":"result","subtype":"success","session_id":"98765","result":"content"}`,
			wantID:  "98765",
			wantErr: false,
		},
		{
			name:    "malformed JSON returns error",
			output:  `{"session_id":"incomplete`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "result event without session_id returns error",
			output:  `{"type":"result","subtype":"success","result":"ok"}`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "empty output returns error",
			output:  "",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "plain text (no result event) returns error",
			output:  "this is plain text output",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "null session_id in result event returns error",
			output:  `{"type":"result","subtype":"success","session_id":null,"result":"ok"}`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "session_id only present in non-result event is ignored",
			output:  `{"type":"system","subtype":"init","session_id":"should-not-match"}`,
			wantID:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.extractSessionID(tt.output)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, got)
			}
		})
	}
}

func TestClaudeProvider_extractTextFromJSON(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name     string
		output   string
		wantText string
	}{
		{
			name:     "NDJSON result event with string result",
			output:   `{"type":"result","subtype":"success","result":"Hello, world!","session_id":"abc123"}`,
			wantText: "Hello, world!",
		},
		{
			name:     "NDJSON with preceding events before result",
			output:   `{"type":"system","subtype":"init"}` + "\n" + `{"type":"assistant","message":{}}` + "\n" + `{"type":"result","subtype":"success","result":"Line 1\nLine 2\nLine 3"}`,
			wantText: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "result field with special characters",
			output:   `{"type":"result","subtype":"success","result":"Response with {braces} and [brackets] and \"quotes\"","id":"123"}`,
			wantText: `Response with {braces} and [brackets] and "quotes"`,
		},
		{
			name:     "empty result field returns empty",
			output:   `{"type":"result","subtype":"success","result":"","session_id":"xyz"}`,
			wantText: "",
		},
		{
			name:     "result event without result field returns empty",
			output:   `{"type":"result","subtype":"success","session_id":"abc","output":"not result field"}`,
			wantText: "",
		},
		{
			name:     "malformed JSON returns empty",
			output:   `{"result":"incomplete`,
			wantText: "",
		},
		{
			name:     "empty output returns empty",
			output:   "",
			wantText: "",
		},
		{
			name:     "plain text returns empty",
			output:   "plain text response",
			wantText: "",
		},
		{
			name:     "numeric result value returned as string",
			output:   `{"type":"result","subtype":"success","result":42,"session_id":"abc"}`,
			wantText: "42",
		},
		{
			name:     "null result field returns empty",
			output:   `{"type":"result","subtype":"success","result":null,"session_id":"abc"}`,
			wantText: "",
		},
		{
			name:     "no result event in stream returns empty",
			output:   `{"type":"system","subtype":"init","session_id":"abc"}`,
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.extractTextFromJSON(tt.output)
			assert.Equal(t, tt.wantText, got)
		})
	}
}
