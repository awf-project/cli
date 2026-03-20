package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// T004: Extract Session ID and Text from JSON
// Tests for extractSessionID() and extractTextFromJSON() methods

func TestClaudeProvider_extractSessionID(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name    string
		output  string
		wantID  string
		wantErr bool
	}{
		{
			name:    "valid json with session_id",
			output:  `{"session_id":"claude-session-12345","result":"ok"}`,
			wantID:  "claude-session-12345",
			wantErr: false,
		},
		{
			name:    "json with uuid-like session_id",
			output:  `{"session_id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","result":"response"}`,
			wantID:  "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			wantErr: false,
		},
		{
			name:    "json with numeric session_id",
			output:  `{"session_id":"98765","data":"content"}`,
			wantID:  "98765",
			wantErr: false,
		},
		{
			name:    "malformed json returns error",
			output:  `{"session_id":"incomplete`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "json without session_id field returns error",
			output:  `{"result":"ok","data":"no session"}`,
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
			name:    "plain text (not json) returns error",
			output:  "this is plain text output",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "null session_id returns error",
			output:  `{"session_id":null,"result":"ok"}`,
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
			name:     "valid json with result field",
			output:   `{"result":"Hello, world!","session_id":"abc123"}`,
			wantText: "Hello, world!",
		},
		{
			name:     "result field with multiline text",
			output:   `{"result":"Line 1\nLine 2\nLine 3","metadata":{}}`,
			wantText: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "result field with special characters",
			output:   `{"result":"Response with {braces} and [brackets] and \"quotes\"","id":"123"}`,
			wantText: `Response with {braces} and [brackets] and "quotes"`,
		},
		{
			name:     "empty result field returns empty",
			output:   `{"result":"","session_id":"xyz"}`,
			wantText: "",
		},
		{
			name:     "json without result field returns empty",
			output:   `{"session_id":"abc","output":"not result field"}`,
			wantText: "",
		},
		{
			name:     "malformed json returns empty",
			output:   `{"result":"incomplete`,
			wantText: "",
		},
		{
			name:     "empty output returns empty",
			output:   "",
			wantText: "",
		},
		{
			name:     "plain text (not json) returns empty",
			output:   "plain text response",
			wantText: "",
		},
		{
			name:     "result field with numeric value returns string representation",
			output:   `{"result":42,"session_id":"abc"}`,
			wantText: "42",
		},
		{
			name:     "null result field returns empty",
			output:   `{"result":null,"session_id":"abc"}`,
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
