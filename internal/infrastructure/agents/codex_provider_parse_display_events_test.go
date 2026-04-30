package agents

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseCodexDisplayEvents parses a raw line from Codex CLI's `exec --json`
// output into a []DisplayEvent. It extracts both event type and displayable text.
// The slice is nil for non-displayable events (skip signal).

func TestCodexProvider_parseCodexDisplayEvents_AssistantMessageCompleted(t *testing.T) {
	provider := NewCodexProvider()

	tests := []struct {
		name     string
		line     []byte
		wantType string
		wantText string
	}{
		{
			name:     "item.completed with assistant_message surfaces text",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"hello"}}`),
			wantType: "item.completed",
			wantText: "hello",
		},
		{
			name:     "assistant_message with multi-line text preserved",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"line1\nline2\nline3"}}`),
			wantType: "item.completed",
			wantText: "line1\nline2\nline3",
		},
		{
			name:     "assistant_message with special characters and unicode",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"Response {braces} [brackets] 你好"}}`),
			wantType: "item.completed",
			wantText: "Response {braces} [brackets] 你好",
		},
		{
			name:     "assistant_message with escaped quotes",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"She said \"hello\""}}`),
			wantType: "item.completed",
			wantText: "She said \"hello\"",
		},
		{
			name:     "assistant_message with empty text",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":""}}`),
			wantType: "item.completed",
			wantText: "",
		},
		{
			name:     "assistant_message with long text",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"` + string(make([]byte, 1000)) + `"}}`),
			wantType: "item.completed",
			wantText: string(make([]byte, 1000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCodexDisplayEvents(tt.line)
			require.Len(t, got, 1)
			assert.Equal(t, tt.wantType, got[0].Type)
			assert.Equal(t, tt.wantText, got[0].Text)
		})
	}
}

func TestCodexProvider_parseCodexDisplayEvents_FunctionCall(t *testing.T) {
	provider := NewCodexProvider()

	tests := []struct {
		name     string
		line     []byte
		wantKind EventKind
		wantName string
		wantArg  string
		wantID   string
	}{
		{
			name:     "function_call with cmd arg",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"function_call","name":"shell","arguments":"{\"cmd\":\"ls -la\"}"}}`),
			wantKind: EventToolUse,
			wantName: "shell",
			wantArg:  "ls -la",
			wantID:   "",
		},
		{
			name:     "function_call with long arg is truncated",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"function_call","name":"shell","arguments":"{\"cmd\":\"` + strings.Repeat("x", 50) + `\"}"}}`),
			wantKind: EventToolUse,
			wantName: "shell",
			wantArg:  strings.Repeat("x", 37) + "…",
			wantID:   "",
		},
		{
			name:     "function_call with file_path arg",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"function_call","name":"read_file","arguments":"{\"file_path\":\"/etc/hosts\"}"}}`),
			wantKind: EventToolUse,
			wantName: "read_file",
			wantArg:  "/etc/hosts",
			wantID:   "",
		},
		{
			name:     "function_call with no recognized key yields empty arg",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"function_call","name":"tool","arguments":"{\"foo\":\"bar\"}"}}`),
			wantKind: EventToolUse,
			wantName: "tool",
			wantArg:  "",
			wantID:   "",
		},
		{
			name:     "function_call with invalid JSON arguments yields empty arg",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"function_call","name":"tool","arguments":"not json"}}`),
			wantKind: EventToolUse,
			wantName: "tool",
			wantArg:  "",
			wantID:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCodexDisplayEvents(tt.line)
			require.Len(t, got, 1)
			assert.Equal(t, "item.completed", got[0].Type)
			assert.Equal(t, tt.wantKind, got[0].Kind)
			assert.Equal(t, tt.wantName, got[0].Name)
			assert.Equal(t, tt.wantArg, got[0].Arg)
			assert.Equal(t, tt.wantID, got[0].ID)
		})
	}
}

func TestCodexProvider_parseCodexDisplayEvents_NonAssistantItems(t *testing.T) {
	provider := NewCodexProvider()

	tests := []struct {
		name     string
		line     []byte
		wantType string
		wantText string
	}{
		{
			name:     "reasoning item.completed has no displayable text",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"reasoning","text":"thinking..."}}`),
			wantType: "item.completed",
			wantText: "",
		},
		{
			name:     "tool_call item.completed has no displayable text",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"tool_call","name":"read_file","input":{"path":"/etc/passwd"}}}`),
			wantType: "item.completed",
			wantText: "",
		},
		{
			name:     "tool_result item.completed has no displayable text",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"tool_result","text":"file contents"}}`),
			wantType: "item.completed",
			wantText: "",
		},
		{
			name:     "system_message item.completed has no displayable text",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"system_message","text":"status"}}`),
			wantType: "item.completed",
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCodexDisplayEvents(tt.line)
			// Non-assistant items return nil (no displayable events)
			assert.Empty(t, got)
		})
	}
}

func TestCodexProvider_parseCodexDisplayEvents_OtherEventTypes(t *testing.T) {
	provider := NewCodexProvider()

	tests := []struct {
		name     string
		line     []byte
		wantType string
		wantText string
	}{
		{
			name:     "thread.started has no displayable text",
			line:     []byte(`{"type":"thread.started","thread_id":"019d8872","created_at":"2025-04-30T12:34:56Z"}`),
			wantType: "thread.started",
			wantText: "",
		},
		{
			name:     "turn.started has no displayable text",
			line:     []byte(`{"type":"turn.started","turn_id":"abc123"}`),
			wantType: "turn.started",
			wantText: "",
		},
		{
			name:     "turn.completed has no displayable text",
			line:     []byte(`{"type":"turn.completed","duration":1234}`),
			wantType: "turn.completed",
			wantText: "",
		},
		{
			name:     "error event has no displayable text",
			line:     []byte(`{"type":"error","message":"Reconnecting to server..."}`),
			wantType: "error",
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCodexDisplayEvents(tt.line)
			// Other event types return nil (no displayable events)
			assert.Empty(t, got)
		})
	}
}

func TestCodexProvider_parseCodexDisplayEvents_ErrorPaths(t *testing.T) {
	provider := NewCodexProvider()

	tests := []struct {
		name    string
		line    []byte
		wantNil bool
	}{
		{
			name:    "malformed JSON returns nil",
			line:    []byte(`{"type":"item.completed","item":{"item_type":`),
			wantNil: true,
		},
		{
			name:    "empty line returns nil",
			line:    []byte(``),
			wantNil: true,
		},
		{
			name:    "invalid JSON returns nil",
			line:    []byte(`this is not json at all`),
			wantNil: true,
		},
		{
			name:    "incomplete JSON object returns nil",
			line:    []byte(`{"type":"item.completed"`),
			wantNil: true,
		},
		{
			name:    "JSON with missing type field returns nil",
			line:    []byte(`{"item":{"item_type":"assistant_message","text":"hello"}}`),
			wantNil: true,
		},
		{
			name:    "JSON with null type returns nil",
			line:    []byte(`{"type":null,"item":{"item_type":"assistant_message","text":"hello"}}`),
			wantNil: true,
		},
		{
			name:    "JSON with missing item field returns nil",
			line:    []byte(`{"type":"item.completed"}`),
			wantNil: true,
		},
		{
			name:    "JSON with null item field returns nil",
			line:    []byte(`{"type":"item.completed","item":null}`),
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCodexDisplayEvents(tt.line)
			assert.Empty(t, got)
		})
	}
}

func TestCodexProvider_parseCodexDisplayEvents_EdgeCases(t *testing.T) {
	provider := NewCodexProvider()

	tests := []struct {
		name     string
		line     []byte
		wantType string
		wantText string
	}{
		{
			name:     "extra JSON fields are ignored",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"response","extra_field":"ignored"},"metadata":{}}`),
			wantType: "item.completed",
			wantText: "response",
		},
		{
			name:     "whitespace in JSON is handled",
			line:     []byte(`{ "type" : "item.completed" , "item" : { "item_type" : "assistant_message" , "text" : "hello" } }`),
			wantType: "item.completed",
			wantText: "hello",
		},
		{
			name:     "no space after colons in JSON",
			line:     []byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"hello"}}`),
			wantType: "item.completed",
			wantText: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCodexDisplayEvents(tt.line)
			require.Len(t, got, 1)
			assert.Equal(t, tt.wantType, got[0].Type)
			assert.Equal(t, tt.wantText, got[0].Text)
		})
	}
}
