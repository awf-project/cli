package agents

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGeminiDisplayEvents_AssistantMessageWithContent(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","content":"Hello, this is Gemini"}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 1)
	assert.NotEmpty(t, result[0].Text)
	assert.Contains(t, result[0].Text, "Hello, this is Gemini")
	assert.Equal(t, "assistant", result[0].Type)
}

func TestParseGeminiDisplayEvents_InitEvent(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"init","session_id":"session-123","model":"gemini-2.0-flash"}`)
	result := provider.parseGeminiDisplayEvents(line)

	assert.Empty(t, result, "init events should not produce displayable text")
}

func TestParseGeminiDisplayEvents_ResultEvent(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"result","status":"success","stats":{"stop_reason":"STOP"}}`)
	result := provider.parseGeminiDisplayEvents(line)

	assert.Empty(t, result, "result events should not produce displayable text")
}

func TestParseGeminiDisplayEvents_UserMessage(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"user","content":"What is 2+2?"}`)
	result := provider.parseGeminiDisplayEvents(line)

	assert.Empty(t, result, "user messages should not produce displayable text")
}

func TestParseGeminiDisplayEvents_AssistantMessageWithEmptyContent(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","content":""}`)
	result := provider.parseGeminiDisplayEvents(line)

	assert.Empty(t, result, "assistant messages with empty content should not produce displayable text")
}

func TestParseGeminiDisplayEvents_InvalidJSON(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{invalid json}`)
	result := provider.parseGeminiDisplayEvents(line)

	assert.Empty(t, result, "invalid JSON should not produce displayable text")
}

func TestParseGeminiDisplayEvents_EmptyLine(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(``)
	result := provider.parseGeminiDisplayEvents(line)

	assert.Empty(t, result, "empty line should not produce displayable text")
}

func TestParseGeminiDisplayEvents_MessageWithoutType(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"role":"assistant","content":"Hello"}`)
	result := provider.parseGeminiDisplayEvents(line)

	assert.Empty(t, result, "message without type field should not produce displayable text")
}

func TestParseGeminiDisplayEvents_MessageWithoutRole(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","content":"Hello"}`)
	result := provider.parseGeminiDisplayEvents(line)

	assert.Empty(t, result, "message without role field should not produce displayable text")
}

func TestParseGeminiDisplayEvents_MessageWithoutContent(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant"}`)
	result := provider.parseGeminiDisplayEvents(line)

	assert.Empty(t, result, "assistant message without content field should not produce displayable text")
}

func TestParseGeminiDisplayEvents_AssistantMessageWithMultilineContent(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","content":"First line\nSecond line"}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 1)
	assert.NotEmpty(t, result[0].Text)
	assert.Contains(t, result[0].Text, "First line")
	assert.Contains(t, result[0].Text, "Second line")
}

func TestParseGeminiDisplayEvents_AssistantMessageWithSpecialCharacters(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","content":"Special: @#$%^&*()"}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 1)
	assert.NotEmpty(t, result[0].Text)
	assert.Contains(t, result[0].Text, "Special: @#$%^&*()")
}

func TestParseGeminiDisplayEvents_AssistantMessageWithUnicodeContent(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","content":"Hello 世界 🌍"}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 1)
	assert.NotEmpty(t, result[0].Text)
	assert.Contains(t, result[0].Text, "世界")
}

func TestParseGeminiDisplayEvents_AssistantMessageWithEscapedCharacters(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","content":"Quote: \"hello\""}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 1)
	assert.NotEmpty(t, result[0].Text)
}

func TestParseGeminiDisplayEvents_AssistantMessageWithDelta(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","content":"streaming text","delta":"text"}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 1)
	assert.NotEmpty(t, result[0].Text)
	assert.Contains(t, result[0].Text, "streaming text")
}

func TestParseGeminiDisplayEvents_NullContent(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","content":null}`)
	result := provider.parseGeminiDisplayEvents(line)

	assert.Empty(t, result, "null content should not produce displayable text")
}

func TestParseGeminiDisplayEvents_MessageWithToolCallsOnly(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","toolCalls":[{"name":"read_file","arguments":{"file_path":"/etc/hosts"}}]}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "read_file", result[0].Name)
	assert.Equal(t, "/etc/hosts", result[0].Arg)
	assert.Equal(t, "", result[0].ID)
}

func TestParseGeminiDisplayEvents_MessageWithTextAndToolCalls(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","content":"Reading file now","toolCalls":[{"name":"read_file","arguments":{"file_path":"/tmp/data.txt"}}]}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 2)
	assert.Equal(t, EventText, result[0].Kind)
	assert.Equal(t, "Reading file now", result[0].Text)
	assert.Equal(t, EventToolUse, result[1].Kind)
	assert.Equal(t, "read_file", result[1].Name)
	assert.Equal(t, "/tmp/data.txt", result[1].Arg)
}

func TestParseGeminiDisplayEvents_ToolCallWithLongArg(t *testing.T) {
	provider := NewGeminiProvider()

	longPath := "/home/user/projects/my-very-long-directory-name/src/deeply/nested/file.go"
	line := []byte(`{"type":"message","role":"assistant","toolCalls":[{"name":"read_file","arguments":{"file_path":"` + longPath + `"}}]}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "read_file", result[0].Name)
	assert.Len(t, []rune(result[0].Arg), 38, "truncated arg should be 37 visible chars + ellipsis rune")
	assert.Equal(t, longPath[:37]+"…", result[0].Arg)
}

func TestParseGeminiDisplayEvents_ToolCallWithNoRecognizedKey(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","toolCalls":[{"name":"custom_tool","arguments":{"foo":"bar"}}]}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "custom_tool", result[0].Name)
	assert.Equal(t, "", result[0].Arg)
}

func TestParseGeminiDisplayEvents_ToolCallWithEmptyArguments(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","toolCalls":[{"name":"list_dir","arguments":{}}]}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "list_dir", result[0].Name)
	assert.Equal(t, "", result[0].Arg)
}

func TestParseGeminiDisplayEvents_MultipleToolCalls(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","toolCalls":[{"name":"read_file","arguments":{"file_path":"/a"}},{"name":"run_cmd","arguments":{"command":"ls"}}]}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 2)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "read_file", result[0].Name)
	assert.Equal(t, "/a", result[0].Arg)
	assert.Equal(t, EventToolUse, result[1].Kind)
	assert.Equal(t, "run_cmd", result[1].Name)
	assert.Equal(t, "ls", result[1].Arg)
}

func TestParseGeminiDisplayEvents_ArgKeyPrecedence(t *testing.T) {
	provider := NewGeminiProvider()

	tests := []struct {
		name    string
		args    string
		wantArg string
	}{
		{
			name:    "file_path takes precedence",
			args:    `{"file_path":"/etc/passwd","command":"ignored"}`,
			wantArg: "/etc/passwd",
		},
		{
			name:    "command recognized",
			args:    `{"command":"echo hello"}`,
			wantArg: "echo hello",
		},
		{
			name:    "cmd recognized",
			args:    `{"cmd":"ls -la"}`,
			wantArg: "ls -la",
		},
		{
			name:    "query recognized",
			args:    `{"query":"SELECT * FROM users"}`,
			wantArg: "SELECT * FROM users",
		},
		{
			name:    "pattern recognized",
			args:    `{"pattern":"*.go"}`,
			wantArg: "*.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := []byte(`{"type":"message","role":"assistant","toolCalls":[{"name":"tool","arguments":` + tt.args + `}]}`)
			result := provider.parseGeminiDisplayEvents(line)
			require.Len(t, result, 1)
			assert.Equal(t, tt.wantArg, result[0].Arg)
		})
	}
}

func TestParseGeminiDisplayEvents_ToolCallIDAlwaysEmpty(t *testing.T) {
	provider := NewGeminiProvider()

	line := []byte(`{"type":"message","role":"assistant","toolCalls":[{"name":"read_file","arguments":{"file_path":"/tmp/x"}}]}`)
	result := provider.parseGeminiDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, "", result[0].ID, "Gemini does not emit tool-call IDs; ID must always be empty")
}

func TestTruncateArg(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "short string unchanged",
			input: "/etc/hosts",
			want:  "/etc/hosts",
		},
		{
			name:  "exactly 40 chars unchanged",
			input: strings.Repeat("a", 40),
			want:  strings.Repeat("a", 40),
		},
		{
			name:  "41 chars truncated to 37 + ellipsis",
			input: strings.Repeat("b", 41),
			want:  strings.Repeat("b", 37) + "…",
		},
		{
			name:  "empty string unchanged",
			input: "",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateArg(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractArgPreviewFromMap(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "nil map returns empty",
			args: nil,
			want: "",
		},
		{
			name: "empty map returns empty",
			args: map[string]any{},
			want: "",
		},
		{
			name: "file_path recognized",
			args: map[string]any{"file_path": "/tmp/file.txt"},
			want: "/tmp/file.txt",
		},
		{
			name: "command recognized",
			args: map[string]any{"command": "echo hi"},
			want: "echo hi",
		},
		{
			name: "cmd recognized",
			args: map[string]any{"cmd": "pwd"},
			want: "pwd",
		},
		{
			name: "query recognized",
			args: map[string]any{"query": "find . -name '*.go'"},
			want: "find . -name '*.go'",
		},
		{
			name: "pattern recognized",
			args: map[string]any{"pattern": "*.yaml"},
			want: "*.yaml",
		},
		{
			name: "unknown key returns empty",
			args: map[string]any{"foo": "bar"},
			want: "",
		},
		{
			name: "non-string value for recognized key returns empty",
			args: map[string]any{"file_path": 42},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractArgPreviewFromMap(tt.args)
			assert.Equal(t, tt.want, got)
		})
	}
}
