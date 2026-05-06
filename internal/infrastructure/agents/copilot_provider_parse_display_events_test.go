package agents

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopilotProvider_parseCopilotDisplayEvents_MessageDelta(t *testing.T) {
	provider := NewCopilotProvider()

	tests := []struct {
		name     string
		line     []byte
		wantLen  int
		wantKind EventKind
		wantText string
	}{
		{
			name:     "message delta with text",
			line:     []byte(`{"type":"assistant.message_delta","data":{"messageId":"msg-1","deltaContent":"hello world"}}`),
			wantLen:  1,
			wantKind: EventText,
			wantText: "hello world",
		},
		{
			name:     "message delta with multiline text",
			line:     []byte(`{"type":"assistant.message_delta","data":{"messageId":"msg-2","deltaContent":"line1\nline2"}}`),
			wantLen:  1,
			wantKind: EventText,
			wantText: "line1\nline2",
		},
		{
			name:     "message delta with special characters",
			line:     []byte(`{"type":"assistant.message_delta","data":{"messageId":"msg-3","deltaContent":"Response {braces} 你好"}}`),
			wantLen:  1,
			wantKind: EventText,
			wantText: "Response {braces} 你好",
		},
		{
			name:     "message delta with escaped quotes",
			line:     []byte(`{"type":"assistant.message_delta","data":{"messageId":"msg-4","deltaContent":"She said \"hello\""}}`),
			wantLen:  1,
			wantKind: EventText,
			wantText: `She said "hello"`,
		},
		{
			name:    "message delta with empty deltaContent is skipped",
			line:    []byte(`{"type":"assistant.message_delta","data":{"messageId":"msg-5","deltaContent":""}}`),
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCopilotDisplayEvents(tt.line)
			if tt.wantLen == 0 {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, tt.wantLen)
			assert.Equal(t, tt.wantKind, got[0].Kind)
			assert.Equal(t, tt.wantText, got[0].Text)
		})
	}
}

func TestCopilotProvider_parseCopilotDisplayEvents_ToolExecution(t *testing.T) {
	provider := NewCopilotProvider()

	tests := []struct {
		name     string
		line     []byte
		wantLen  int
		wantKind EventKind
		wantName string
	}{
		{
			name:     "tool execution start with bash",
			line:     []byte(`{"type":"tool.execution_start","data":{"toolCallId":"call-1","toolName":"bash","arguments":{"command":"ls"}}}`),
			wantLen:  1,
			wantKind: EventToolUse,
			wantName: "bash",
		},
		{
			name:     "tool execution start with read",
			line:     []byte(`{"type":"tool.execution_start","data":{"toolCallId":"call-2","toolName":"read","arguments":{"file_path":"main.go"}}}`),
			wantLen:  1,
			wantKind: EventToolUse,
			wantName: "read",
		},
		{
			name:     "tool execution start with report_intent",
			line:     []byte(`{"type":"tool.execution_start","data":{"toolCallId":"call-3","toolName":"report_intent","arguments":{"intent":"fixing bug"}}}`),
			wantLen:  1,
			wantKind: EventToolUse,
			wantName: "report_intent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCopilotDisplayEvents(tt.line)
			require.Len(t, got, tt.wantLen)
			assert.Equal(t, tt.wantKind, got[0].Kind)
			assert.Equal(t, tt.wantName, got[0].Name)
		})
	}
}

func TestCopilotProvider_parseCopilotDisplayEvents_IgnoredTypes(t *testing.T) {
	provider := NewCopilotProvider()

	tests := []struct {
		name string
		line []byte
	}{
		{
			name: "result event is not a display event",
			line: []byte(`{"type":"result","sessionId":"sess-1","exitCode":0}`),
		},
		{
			name: "session.mcp_servers_loaded is not a display event",
			line: []byte(`{"type":"session.mcp_servers_loaded","data":{"servers":[]}}`),
		},
		{
			name: "user.message is not a display event",
			line: []byte(`{"type":"user.message","data":{"content":"hello"}}`),
		},
		{
			name: "assistant.turn_start is not a display event",
			line: []byte(`{"type":"assistant.turn_start","data":{"turnId":"0"}}`),
		},
		{
			name: "assistant.turn_end is not a display event",
			line: []byte(`{"type":"assistant.turn_end","data":{"turnId":"0"}}`),
		},
		{
			name: "assistant.message (complete) is not a display event",
			line: []byte(`{"type":"assistant.message","data":{"content":"hello","messageId":"msg-1"}}`),
		},
		{
			name: "tool.execution_complete is not a display event",
			line: []byte(`{"type":"tool.execution_complete","data":{"toolCallId":"call-1","success":true}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCopilotDisplayEvents(tt.line)
			assert.Empty(t, got, "non-display event types should return empty slice")
		})
	}
}

func TestCopilotProvider_parseCopilotDisplayEvents_ErrorPaths(t *testing.T) {
	provider := NewCopilotProvider()

	tests := []struct {
		name string
		line []byte
	}{
		{
			name: "empty line",
			line: []byte(""),
		},
		{
			name: "malformed JSON",
			line: []byte("{broken"),
		},
		{
			name: "incomplete JSON object",
			line: []byte(`{"type":"assistant.message_delta"`),
		},
		{
			name: "not JSON at all",
			line: []byte("this is not json at all"),
		},
		{
			name: "JSON with null type",
			line: []byte(`{"type":null,"data":{"deltaContent":"hello"}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCopilotDisplayEvents(tt.line)
			assert.Empty(t, got, "malformed input should return empty slice")
		})
	}
}

func TestCopilotProvider_parseCopilotDisplayEvents_EventKindMetadata(t *testing.T) {
	provider := NewCopilotProvider()

	tests := []struct {
		name     string
		line     []byte
		wantKind EventKind
	}{
		{
			name:     "message delta has EventText kind",
			line:     []byte(`{"type":"assistant.message_delta","data":{"deltaContent":"test"}}`),
			wantKind: EventText,
		},
		{
			name:     "tool execution has EventToolUse kind",
			line:     []byte(`{"type":"tool.execution_start","data":{"toolName":"bash"}}`),
			wantKind: EventToolUse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseCopilotDisplayEvents(tt.line)
			require.Len(t, got, 1)
			assert.Equal(t, tt.wantKind, got[0].Kind)
			assert.NotEqual(t, EventKind(""), got[0].Kind, "EventKind should never be zero-value")
		})
	}
}

func TestCopilotProvider_parseCopilotDisplayEvents_LargeInput(t *testing.T) {
	provider := NewCopilotProvider()

	largeContent := strings.Repeat("x", 1024*1024)
	line := []byte(`{"type":"assistant.message_delta","data":{"deltaContent":"` + largeContent + `"}}`)

	got := provider.parseCopilotDisplayEvents(line)

	require.Len(t, got, 1)
	assert.Equal(t, EventText, got[0].Kind)
	assert.Len(t, got[0].Text, len(largeContent))
}

func BenchmarkParseCopilotDisplayEvents(b *testing.B) {
	provider := NewCopilotProvider()
	line := []byte(`{"type":"assistant.message_delta","data":{"deltaContent":"Hello from Copilot!"}}`)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = provider.parseCopilotDisplayEvents(line)
	}
}

func BenchmarkParseCopilotDisplayEvents_LargePayload(b *testing.B) {
	provider := NewCopilotProvider()
	largeContent := strings.Repeat("x", 10000)
	line := []byte(`{"type":"assistant.message_delta","data":{"deltaContent":"` + largeContent + `"}}`)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = provider.parseCopilotDisplayEvents(line)
	}
}
