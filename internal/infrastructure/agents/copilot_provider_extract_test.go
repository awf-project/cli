package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// T001: Extract tests for CopilotProvider validate session ID and text content extraction from JSONL

func TestCopilotProvider_extractSessionID(t *testing.T) {
	provider := NewCopilotProvider()

	tests := []struct {
		name    string
		output  string
		wantID  string
		wantErr bool
	}{
		{
			name:    "result event carries session_id",
			output:  `{"type":"result","sessionId":"abc-123"}`,
			wantID:  "abc-123",
			wantErr: false,
		},
		{
			name:    "result event with result and session_id",
			output:  `{"type":"result","sessionId":"sess-xyz","result":"output"}`,
			wantID:  "sess-xyz",
			wantErr: false,
		},
		{
			name:    "result event with uuid-style session_id",
			output:  `{"type":"result","sessionId":"019bd456-d3d4-70c3-90de-51d31a6c8571"}`,
			wantID:  "019bd456-d3d4-70c3-90de-51d31a6c8571",
			wantErr: false,
		},
		{
			name:    "result with subsequent events",
			output:  `{"type":"result","sessionId":"sess-abc","result":"Generated"}` + "\n" + `{"type":"done"}`,
			wantID:  "sess-abc",
			wantErr: false,
		},
		{
			name:    "numeric-looking session_id string",
			output:  `{"type":"result","sessionId":"98765"}`,
			wantID:  "98765",
			wantErr: false,
		},
		{
			name:    "no result event returns error",
			output:  `{"type":"message","content":"text"}`,
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
			name:    "plain text returns error",
			output:  "plain text with no JSON",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "result event without session_id returns error",
			output:  `{"type":"result","result":"output"}`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "result with empty session_id returns error",
			output:  `{"type":"result","sessionId":""}`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "result with null session_id returns error",
			output:  `{"type":"result","sessionId":null}`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "session_id in non-result event is ignored",
			output:  `{"type":"message","sessionId":"should-not-match"}`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "malformed JSON line is skipped",
			output:  `{"type":"result","sessionId":"incomplete`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "multiple JSONL lines with result first",
			output:  `{"type":"result","sessionId":"found-id"}` + "\n" + `{"type":"message"}` + "\n" + `{"type":"done"}`,
			wantID:  "found-id",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.extractCopilotSessionID(tt.output)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
			} else {
				// Graceful fallback: no error, just empty string if not found
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, got)
			}
		})
	}
}

func TestCopilotProvider_extractTextContent(t *testing.T) {
	provider := NewCopilotProvider()

	tests := []struct {
		name    string
		output  string
		wantOut string
	}{
		{
			name:    "assistant.message event carries content",
			output:  `{"type":"assistant.message","data":{"content":"hello world","messageId":"msg-1"}}`,
			wantOut: "hello world",
		},
		{
			name:    "assistant.message with multiline content",
			output:  `{"type":"assistant.message","data":{"content":"line1\nline2\nline3","messageId":"msg-2"}}`,
			wantOut: "line1\nline2\nline3",
		},
		{
			name:    "assistant.message with empty content falls back to raw",
			output:  `{"type":"assistant.message","data":{"content":"","messageId":"msg-3"}}`,
			wantOut: `{"type":"assistant.message","data":{"content":"","messageId":"msg-3"}}`,
		},
		{
			name:    "assistant.message with special characters",
			output:  `{"type":"assistant.message","data":{"content":"Hello \"World\"!","messageId":"msg-4"}}`,
			wantOut: `Hello "World"!`,
		},
		{
			name:    "full JSONL stream extracts last assistant.message",
			output:  `{"type":"user.message","data":{"content":"test"}}` + "\n" + `{"type":"assistant.message","data":{"content":"response","messageId":"msg-5"}}` + "\n" + `{"type":"result","sessionId":"sess-1","exitCode":0}`,
			wantOut: "response",
		},
		{
			name:    "multiple assistant.message events returns last",
			output:  `{"type":"assistant.message","data":{"content":"first","messageId":"msg-6"}}` + "\n" + `{"type":"assistant.message","data":{"content":"second","messageId":"msg-7"}}`,
			wantOut: "second",
		},
		{
			name:    "no assistant.message returns raw output",
			output:  `{"type":"result","sessionId":"sess-1","exitCode":0}`,
			wantOut: `{"type":"result","sessionId":"sess-1","exitCode":0}`,
		},
		{
			name:    "empty output returns empty unchanged",
			output:  "",
			wantOut: "",
		},
		{
			name:    "plain text returns raw text unchanged",
			output:  "plain text with no JSON",
			wantOut: "plain text with no JSON",
		},
		{
			name:    "malformed JSON line skipped",
			output:  `{"type":"assistant.message","data":{"content":"incomplete`,
			wantOut: `{"type":"assistant.message","data":{"content":"incomplete`,
		},
		{
			name:    "assistant.message with json content",
			output:  `{"type":"assistant.message","data":{"content":"{\"key\":\"value\"}","messageId":"msg-8"}}`,
			wantOut: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.extractCopilotTextContent(tt.output)
			assert.Equal(t, tt.wantOut, got)
		})
	}
}

func TestCopilotProvider_extractSessionID_MultilineJSONL(t *testing.T) {
	provider := NewCopilotProvider()

	// Test that we find the first result event with session_id in JSONL stream
	output := `{"type":"progress","pct":25}
{"type":"result","sessionId":"sess-important","result":"code"}
{"type":"progress","pct":100}
{"type":"done"}`

	id, err := provider.extractCopilotSessionID(output)

	assert.NoError(t, err)
	assert.Equal(t, "sess-important", id)
}

func TestCopilotProvider_extractTextContent_LastMessageWins(t *testing.T) {
	provider := NewCopilotProvider()

	output := `{"type":"assistant.message","data":{"content":"first turn","messageId":"msg-1"}}
{"type":"tool.execution_start","data":{"toolName":"bash"}}
{"type":"assistant.message","data":{"content":"second turn","messageId":"msg-2"}}
{"type":"result","sessionId":"sess-1","exitCode":0}`

	content := provider.extractCopilotTextContent(output)

	assert.Equal(t, "second turn", content)
}

func TestCopilotProvider_extractTextContent_NoAssistantMessage(t *testing.T) {
	provider := NewCopilotProvider()

	tests := []struct {
		name    string
		output  string
		wantOut string
	}{
		{
			name:    "only progress events",
			output:  `{"type":"session.mcp_servers_loaded","data":{}}` + "\n" + `{"type":"result","sessionId":"s","exitCode":0}`,
			wantOut: `{"type":"session.mcp_servers_loaded","data":{}}` + "\n" + `{"type":"result","sessionId":"s","exitCode":0}`,
		},
		{
			name:    "user message only",
			output:  `{"type":"user.message","data":{"content":"hello"}}`,
			wantOut: `{"type":"user.message","data":{"content":"hello"}}`,
		},
		{
			name:    "plain error text",
			output:  "command failed: permission denied",
			wantOut: "command failed: permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := provider.extractCopilotTextContent(tt.output)
			assert.Equal(t, tt.wantOut, content)
		})
	}
}

func TestCopilotProvider_extractTextContent_ContentVariations(t *testing.T) {
	provider := NewCopilotProvider()

	tests := []struct {
		name    string
		output  string
		wantOut string
	}{
		{
			name:    "unicode characters",
			output:  `{"type":"assistant.message","data":{"content":"Hello 世界 🌍","messageId":"msg-1"}}`,
			wantOut: "Hello 世界 🌍",
		},
		{
			name:    "content with newlines",
			output:  `{"type":"assistant.message","data":{"content":"line1\nline2","messageId":"msg-2"}}`,
			wantOut: "line1\nline2",
		},
		{
			name:    "content with tool requests is still extracted",
			output:  `{"type":"assistant.message","data":{"content":"I will run ls","toolRequests":[{"name":"bash"}],"messageId":"msg-3"}}`,
			wantOut: "I will run ls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.extractCopilotTextContent(tt.output)
			assert.Equal(t, tt.wantOut, got)
		})
	}
}
