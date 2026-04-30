package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// translateOpenAICompatibleDisplayEvents parses a raw NDJSON line from the
// OpenAI Chat Completions streaming API into a []DisplayEvent.
// Streaming chunks carry content in choices[0].delta.content;
// non-content chunks (role-only, finish) and error inputs return nil.

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_ChunkWithContent(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	tests := []struct {
		name     string
		line     []byte
		wantType string
		wantText string
	}{
		{
			name:     "streaming chunk with content surfaces text",
			line:     []byte(`{"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`),
			wantType: "chat.completion.chunk",
			wantText: "hello",
		},
		{
			name:     "streaming chunk with multi-line content preserved",
			line:     []byte(`{"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"line1\nline2\nline3"},"finish_reason":null}]}`),
			wantType: "chat.completion.chunk",
			wantText: "line1\nline2\nline3",
		},
		{
			name:     "streaming chunk with special characters and unicode",
			line:     []byte(`{"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Response {braces} 你好"},"finish_reason":null}]}`),
			wantType: "chat.completion.chunk",
			wantText: "Response {braces} 你好",
		},
		{
			name:     "streaming chunk with escaped quotes in content",
			line:     []byte(`{"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"She said \"hello\""},"finish_reason":null}]}`),
			wantType: "chat.completion.chunk",
			wantText: "She said \"hello\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.translateOpenAICompatibleDisplayEvents(tt.line)
			require.Len(t, got, 1)
			assert.Equal(t, tt.wantType, got[0].Type)
			assert.Equal(t, tt.wantText, got[0].Text)
		})
	}
}

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_NonContentChunks(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	tests := []struct {
		name string
		line []byte
	}{
		{
			name: "first chunk with role-only delta returns nil",
			line: []byte(`{"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`),
		},
		{
			name: "finish chunk with stop reason returns nil",
			line: []byte(`{"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`),
		},
		{
			name: "DONE terminator returns nil",
			line: []byte(`[DONE]`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.translateOpenAICompatibleDisplayEvents(tt.line)
			assert.Empty(t, got)
		})
	}
}

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_ErrorPaths(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	tests := []struct {
		name string
		line []byte
	}{
		{
			name: "malformed JSON returns nil",
			line: []byte(`{"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[`),
		},
		{
			name: "empty line returns nil",
			line: []byte(``),
		},
		{
			name: "invalid JSON returns nil",
			line: []byte(`this is not json at all`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.translateOpenAICompatibleDisplayEvents(tt.line)
			assert.Empty(t, got)
		})
	}
}

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_MissingChoices(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	line := []byte(`{"id":"chatcmpl-abc","object":"chat.completion.chunk"}`)
	got := provider.translateOpenAICompatibleDisplayEvents(line)

	assert.Empty(t, got, "JSON with missing choices field returns nil")
}

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_StreamingToolCall(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	line := []byte(`{"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_1","function":{"name":"read","arguments":"{\"file_path\": \"/etc/hosts\"}"}}]},"finish_reason":null}]}`)
	got := provider.translateOpenAICompatibleDisplayEvents(line)

	require.Len(t, got, 1)
	assert.Equal(t, EventToolUse, got[0].Kind)
	assert.Equal(t, "read", got[0].Name)
	assert.Equal(t, "/etc/hosts", got[0].Arg)
	assert.Equal(t, "call_1", got[0].ID)
}

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_CompleteResponseWithTextAndToolCall(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	line := []byte(`{"id":"chatcmpl-abc","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"Here is the result:","tool_calls":[{"id":"call_2","function":{"name":"read_file","arguments":"{\"file_path\": \"/tmp/data.txt\"}"}}]},"finish_reason":"tool_calls"}]}`)
	got := provider.translateOpenAICompatibleDisplayEvents(line)

	require.Len(t, got, 2)
	assert.Equal(t, EventText, got[0].Kind)
	assert.Equal(t, "Here is the result:", got[0].Text)
	assert.Equal(t, EventToolUse, got[1].Kind)
	assert.Equal(t, "read_file", got[1].Name)
	assert.Equal(t, "/tmp/data.txt", got[1].Arg)
	assert.Equal(t, "call_2", got[1].ID)
}

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_ToolCallWithLongArg(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	// file_path value is 50 chars → should be truncated to 37 runes + "…"
	line := []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"tool_calls":[{"id":"call_3","function":{"name":"read","arguments":"{\"file_path\": \"/very/long/path/to/some/deeply/nested/file.txt\"}"}}]}}]}`)
	got := provider.translateOpenAICompatibleDisplayEvents(line)

	require.Len(t, got, 1)
	assert.Equal(t, EventToolUse, got[0].Kind)
	assert.Equal(t, "read", got[0].Name)
	// 37 runes + "…" = 38 rune string, len in bytes may differ
	runes := []rune(got[0].Arg)
	assert.LessOrEqual(t, len(runes), 38, "arg must be truncated to at most 37 chars + ellipsis")
	assert.True(t, len(runes) > 0, "arg must not be empty")
}

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_ToolCallNoRecognizedKey(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	line := []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"tool_calls":[{"id":"call_4","function":{"name":"some_tool","arguments":"{\"unknown_key\": \"some_value\"}"}}]}}]}`)
	got := provider.translateOpenAICompatibleDisplayEvents(line)

	require.Len(t, got, 1)
	assert.Equal(t, EventToolUse, got[0].Kind)
	assert.Equal(t, "some_tool", got[0].Name)
	assert.Equal(t, "", got[0].Arg)
}

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_ToolCallInvalidJSONArguments(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	line := []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"tool_calls":[{"id":"call_5","function":{"name":"some_tool","arguments":"not-valid-json"}}]}}]}`)
	got := provider.translateOpenAICompatibleDisplayEvents(line)

	require.Len(t, got, 1)
	assert.Equal(t, EventToolUse, got[0].Kind)
	assert.Equal(t, "some_tool", got[0].Name)
	assert.Equal(t, "", got[0].Arg)
}

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_StreamingTextChunkUnchanged(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	line := []byte(`{"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"hello world"},"finish_reason":null}]}`)
	got := provider.translateOpenAICompatibleDisplayEvents(line)

	require.Len(t, got, 1)
	assert.Equal(t, EventText, got[0].Kind)
	assert.Equal(t, "hello world", got[0].Text)
	assert.Equal(t, "chat.completion.chunk", got[0].Type)
}

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_EmptyChoicesStillNil(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	line := []byte(`{"object":"chat.completion.chunk","choices":[]}`)
	got := provider.translateOpenAICompatibleDisplayEvents(line)

	assert.Empty(t, got)
}

func TestOpenAICompatibleProvider_translateOpenAICompatibleDisplayEvents_MalformedJSONStillNil(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	line := []byte(`{"object":"chat.completion.chunk","choices":[`)
	got := provider.translateOpenAICompatibleDisplayEvents(line)

	assert.Empty(t, got)
}
