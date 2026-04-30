package agents

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseOpencodeDisplayEvents parses a raw line from OpenCode CLI's `--format json`
// output into a []DisplayEvent. It extracts both event type and displayable text.
// Only "text" events carry displayable content; all others return nil (skip signal).

func TestOpenCodeProvider_parseOpencodeDisplayEvents_TextEvents(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name     string
		line     []byte
		wantType string
		wantText string
	}{
		{
			name:     "text event surfaces text field",
			line:     []byte(`{"type":"text","part":{"text":"hello world"}}`),
			wantType: "text",
			wantText: "hello world",
		},
		{
			name:     "text event with multi-line content preserved",
			line:     []byte(`{"type":"text","part":{"text":"line1\nline2\nline3"}}`),
			wantType: "text",
			wantText: "line1\nline2\nline3",
		},
		{
			name:     "text event with special characters and unicode",
			line:     []byte(`{"type":"text","part":{"text":"Response {braces} [brackets] 你好"}}`),
			wantType: "text",
			wantText: "Response {braces} [brackets] 你好",
		},
		{
			name:     "text event with escaped quotes",
			line:     []byte(`{"type":"text","part":{"text":"She said \"hello\""}}`),
			wantType: "text",
			wantText: "She said \"hello\"",
		},
		{
			name:     "text event with empty text",
			line:     []byte(`{"type":"text","part":{"text":""}}`),
			wantType: "text",
			wantText: "",
		},
		{
			name:     "text event with long content",
			line:     []byte(`{"type":"text","part":{"text":"` + strings.Repeat("x", 1000) + `"}}`),
			wantType: "text",
			wantText: strings.Repeat("x", 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseOpencodeDisplayEvents(tt.line)
			require.Len(t, got, 1)
			assert.Equal(t, tt.wantType, got[0].Type)
			assert.Equal(t, tt.wantText, got[0].Text)
		})
	}
}

func TestOpenCodeProvider_parseOpencodeDisplayEvents_NonTextEvents(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name string
		line []byte
	}{
		{
			name: "step_start event has no displayable text",
			line: []byte(`{"type":"step_start","sessionID":"sess-123","part":{}}`),
		},
		{
			name: "step_finish event has no displayable text",
			line: []byte(`{"type":"step_finish","sessionID":"sess-123","part":{"tokens":150,"cost":0.001}}`),
		},
		{
			name: "thinking event has no displayable text",
			line: []byte(`{"type":"thinking","part":{"text":"internal reasoning"}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseOpencodeDisplayEvents(tt.line)
			// Non-text events that are not tool_use return nil
			for _, evt := range got {
				assert.NotEqual(t, EventText, evt.Kind)
			}
		})
	}
}

func TestOpenCodeProvider_parseOpencodeDisplayEvents_ToolUseEvent(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name     string
		line     []byte
		wantName string
		wantArg  string
		wantID   string
	}{
		{
			name:     "tool_use with name and file_path input",
			line:     []byte(`{"type":"tool_use","part":{"name":"Read","input":{"file_path":"/etc/hosts"}}}`),
			wantName: "Read",
			wantArg:  "/etc/hosts",
			wantID:   "",
		},
		{
			name:     "tool_use with name and query key",
			line:     []byte(`{"type":"tool_use","part":{"name":"search","input":{"query":"example"}}}`),
			wantName: "search",
			wantArg:  "example",
			wantID:   "",
		},
		{
			name:     "tool_use with command key",
			line:     []byte(`{"type":"tool_use","part":{"name":"Bash","input":{"command":"ls -la"}}}`),
			wantName: "Bash",
			wantArg:  "ls -la",
			wantID:   "",
		},
		{
			name:     "tool_use with cmd key",
			line:     []byte(`{"type":"tool_use","part":{"name":"Run","input":{"cmd":"make build"}}}`),
			wantName: "Run",
			wantArg:  "make build",
			wantID:   "",
		},
		{
			name:     "tool_use with pattern key",
			line:     []byte(`{"type":"tool_use","part":{"name":"Grep","input":{"pattern":"func main"}}}`),
			wantName: "Grep",
			wantArg:  "func main",
			wantID:   "",
		},
		{
			name:     "tool_use with long arg truncated to 37 runes plus ellipsis",
			line:     []byte(`{"type":"tool_use","part":{"name":"Read","input":{"file_path":"/very/long/path/that/exceeds/forty/characters/limit.go"}}}`),
			wantName: "Read",
			wantArg:  "/very/long/path/that/exceeds/forty/ch" + "…",
			wantID:   "",
		},
		{
			name:     "tool_use with unrecognized key returns empty arg",
			line:     []byte(`{"type":"tool_use","part":{"name":"Custom","input":{"foo":"bar"}}}`),
			wantName: "Custom",
			wantArg:  "",
			wantID:   "",
		},
		{
			name:     "tool_use with nil part returns empty name and arg",
			line:     []byte(`{"type":"tool_use"}`),
			wantName: "",
			wantArg:  "",
			wantID:   "",
		},
		{
			name:     "tool_use with null part returns empty name and arg",
			line:     []byte(`{"type":"tool_use","part":null}`),
			wantName: "",
			wantArg:  "",
			wantID:   "",
		},
		{
			name:     "tool_use with empty input map returns empty arg",
			line:     []byte(`{"type":"tool_use","part":{"name":"Tool","input":{}}}`),
			wantName: "Tool",
			wantArg:  "",
			wantID:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseOpencodeDisplayEvents(tt.line)
			require.Len(t, got, 1)
			assert.Equal(t, "tool_use", got[0].Type)
			assert.Equal(t, EventToolUse, got[0].Kind)
			assert.Empty(t, got[0].Text)
			assert.Equal(t, tt.wantName, got[0].Name)
			assert.Equal(t, tt.wantArg, got[0].Arg)
			assert.Equal(t, tt.wantID, got[0].ID)
		})
	}
}

func TestOpenCodeProvider_parseOpencodeDisplayEvents_ToolUseArgTruncation(t *testing.T) {
	provider := NewOpenCodeProvider()

	t.Run("arg exactly 40 runes is not truncated", func(t *testing.T) {
		arg40 := strings.Repeat("a", 40)
		line := []byte(`{"type":"tool_use","part":{"name":"Read","input":{"file_path":"` + arg40 + `"}}}`)
		got := provider.parseOpencodeDisplayEvents(line)
		require.Len(t, got, 1)
		assert.Equal(t, arg40, got[0].Arg)
	})

	t.Run("arg of 41 runes is truncated to 37 plus ellipsis", func(t *testing.T) {
		arg41 := strings.Repeat("b", 41)
		line := []byte(`{"type":"tool_use","part":{"name":"Read","input":{"file_path":"` + arg41 + `"}}}`)
		got := provider.parseOpencodeDisplayEvents(line)
		require.Len(t, got, 1)
		assert.Equal(t, strings.Repeat("b", 37)+"…", got[0].Arg)
	})

	t.Run("unicode arg truncates by rune count not byte count", func(t *testing.T) {
		// Each Chinese character is 3 UTF-8 bytes but 1 rune; 41 runes exceeds limit
		arg := strings.Repeat("中", 41)
		line := []byte(`{"type":"tool_use","part":{"name":"Read","input":{"query":"` + arg + `"}}}`)
		got := provider.parseOpencodeDisplayEvents(line)
		require.Len(t, got, 1)
		assert.Equal(t, strings.Repeat("中", 37)+"…", got[0].Arg)
	})
}

func TestOpenCodeProvider_parseOpencodeDisplayEvents_StepEventsReturnNil(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name string
		line []byte
	}{
		{
			name: "step_start returns nil",
			line: []byte(`{"type":"step_start","sessionID":"abc-123","part":{}}`),
		},
		{
			name: "step_finish returns nil",
			line: []byte(`{"type":"step_finish","sessionID":"abc-123","part":{"tokens":99}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseOpencodeDisplayEvents(tt.line)
			assert.Nil(t, got)
		})
	}
}

func TestOpenCodeProvider_parseOpencodeDisplayEvents_ErrorPaths(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name string
		line []byte
	}{
		{
			name: "malformed JSON returns nil",
			line: []byte(`{"type":"text","part":{"text":`),
		},
		{
			name: "empty line returns nil",
			line: []byte(``),
		},
		{
			name: "invalid JSON returns nil",
			line: []byte(`this is not json at all`),
		},
		{
			name: "incomplete JSON object returns nil",
			line: []byte(`{"type":"text"`),
		},
		{
			name: "JSON with missing type field returns nil",
			line: []byte(`{"part":{"text":"hello"}}`),
		},
		{
			name: "JSON with null type returns nil",
			line: []byte(`{"type":null,"part":{"text":"hello"}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseOpencodeDisplayEvents(tt.line)
			assert.Empty(t, got)
		})
	}
}

func TestOpenCodeProvider_parseOpencodeDisplayEvents_TextEventMissingPart(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name     string
		line     []byte
		wantType string
		wantText string
	}{
		{
			name:     "text event with missing part field",
			line:     []byte(`{"type":"text"}`),
			wantType: "text",
			wantText: "",
		},
		{
			name:     "text event with null part field",
			line:     []byte(`{"type":"text","part":null}`),
			wantType: "text",
			wantText: "",
		},
		{
			name:     "text event with missing text field in part",
			line:     []byte(`{"type":"text","part":{}}`),
			wantType: "text",
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseOpencodeDisplayEvents(tt.line)
			require.Len(t, got, 1)
			assert.Equal(t, tt.wantType, got[0].Type)
			assert.Equal(t, tt.wantText, got[0].Text)
		})
	}
}

func TestOpenCodeProvider_parseOpencodeDisplayEvents_EdgeCases(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name     string
		line     []byte
		wantType string
		wantText string
	}{
		{
			name:     "extra JSON fields are ignored",
			line:     []byte(`{"type":"text","part":{"text":"response","extra":"ignored"},"metadata":{}}`),
			wantType: "text",
			wantText: "response",
		},
		{
			name:     "whitespace in JSON is handled",
			line:     []byte(`{ "type" : "text" , "part" : { "text" : "hello" } }`),
			wantType: "text",
			wantText: "hello",
		},
		{
			name:     "text with JSON-like content is preserved",
			line:     []byte(`{"type":"text","part":{"text":"{\"nested\":\"json\"}"}}`),
			wantType: "text",
			wantText: `{"nested":"json"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseOpencodeDisplayEvents(tt.line)
			require.Len(t, got, 1)
			assert.Equal(t, tt.wantType, got[0].Type)
			assert.Equal(t, tt.wantText, got[0].Text)
		})
	}
}
