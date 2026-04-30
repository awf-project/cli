package agents

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseClaudeDisplayEvents_AssistantWithTextContent(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello, this is Claude"}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventText, result[0].Kind)
	assert.NotEmpty(t, result[0].Text)
	assert.Contains(t, result[0].Text, "Hello, this is Claude")
}

func TestParseClaudeDisplayEvents_NonAssistantEvent(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"system","session_id":"abc123"}`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Empty(t, result, "system events should not produce displayable text")
}

func TestParseClaudeDisplayEvents_AssistantWithMultipleTextBlocks(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"First block"},{"type":"text","text":"Second block"}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	// Each text block is now emitted as a separate EventText event.
	require.Len(t, result, 2)
	assert.Equal(t, EventText, result[0].Kind)
	assert.Equal(t, "First block", result[0].Text)
	assert.Equal(t, EventText, result[1].Kind)
	assert.Equal(t, "Second block", result[1].Text)
}

func TestParseClaudeDisplayEvents_AssistantWithNonTextContent(t *testing.T) {
	provider := NewClaudeProvider()

	// tool_use blocks now produce EventToolUse events, not empty results.
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool123","name":"calculator","input":{}}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "calculator", result[0].Name)
	assert.Equal(t, "tool123", result[0].ID)
	assert.Equal(t, "", result[0].Arg)
}

func TestParseClaudeDisplayEvents_AssistantWithEmptyContentArray(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Empty(t, result, "empty content array should not produce displayable text")
}

func TestParseClaudeDisplayEvents_InvalidJSON(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{invalid json}`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Empty(t, result, "invalid JSON should not produce displayable text")
}

func TestParseClaudeDisplayEvents_EmptyLine(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(``)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Empty(t, result, "empty line should not produce displayable text")
}

func TestParseClaudeDisplayEvents_AssistantWithoutMessage(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant"}`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Empty(t, result, "assistant event without message should not produce displayable text")
}

func TestParseClaudeDisplayEvents_AssistantWithNullMessage(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":null}`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Empty(t, result, "assistant event with null message should not produce displayable text")
}

func TestParseClaudeDisplayEvents_AssistantWithEmptyText(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":""}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Empty(t, result, "text blocks with empty string should not produce displayable text")
}

func TestParseClaudeDisplayEvents_RateLimitEvent(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"rate_limit_event","retry_after_ms":30000}`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Empty(t, result, "rate_limit_event should not produce displayable text")
}

func TestParseClaudeDisplayEvents_ResultEvent(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"result","result":"Final answer"}`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Empty(t, result, "result event should not produce displayable text")
}

func TestParseClaudeDisplayEvents_AssistantWithMixedContent(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Thinking..."},{"type":"thinking","thinking":"internal thoughts"},{"type":"text","text":"Answer"}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	// Each text block is emitted as a separate EventText event; "thinking" blocks are skipped.
	require.Len(t, result, 2)
	assert.Equal(t, EventText, result[0].Kind)
	assert.Equal(t, "Thinking...", result[0].Text)
	assert.Equal(t, EventText, result[1].Kind)
	assert.Equal(t, "Answer", result[1].Text)

	// Verify internal thoughts are not exposed in any event.
	for _, ev := range result {
		assert.NotContains(t, ev.Text, "internal thoughts")
	}
}

func TestParseClaudeDisplayEvents_WhitespaceHandling(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"  spaces  "}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 1)
	assert.NotEmpty(t, result[0].Text)
}

func TestParseClaudeDisplayEvents_UnicodeContent(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello 世界 🌍"}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 1)
	assert.NotEmpty(t, result[0].Text)
	assert.Contains(t, result[0].Text, "世界")
}

func TestParseClaudeDisplayEvents_DisplayEventHasTypeField(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Test"}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 1)
	assert.NotEmpty(t, result[0].Text)
	assert.NotEmpty(t, result[0].Type, "DisplayEvent should have Type field populated")
}

func TestParseClaudeDisplayEvents_ZeroValueWhenNoDisplayableContent(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"system"}`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Empty(t, result, "non-displayable content should return nil/empty slice")
}

// --- New test cases for EventToolUse support ---

func TestParseClaudeDisplayEvents_InterleavedTextAndToolUse(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[` +
		`{"type":"text","text":"Reading file..."},` +
		`{"type":"tool_use","id":"toolu_abc","name":"read_file","input":{"file_path":"/etc/hosts"}},` +
		`{"type":"text","text":"Done."}` +
		`]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 3, "should emit 3 events in source order")

	assert.Equal(t, EventText, result[0].Kind)
	assert.Equal(t, "Reading file...", result[0].Text)

	assert.Equal(t, EventToolUse, result[1].Kind)
	assert.Equal(t, "read_file", result[1].Name)
	assert.Equal(t, "/etc/hosts", result[1].Arg)
	assert.Equal(t, "toolu_abc", result[1].ID)

	assert.Equal(t, EventText, result[2].Kind)
	assert.Equal(t, "Done.", result[2].Text)
}

func TestParseClaudeDisplayEvents_ToolUseWithKnownArgKey_FilePath(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_1","name":"read_file","input":{"file_path":"/etc/hosts"}}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "/etc/hosts", result[0].Arg)
}

func TestParseClaudeDisplayEvents_ToolUseWithKnownArgKey_Command(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_2","name":"bash","input":{"command":"ls -la"}}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "ls -la", result[0].Arg)
}

func TestParseClaudeDisplayEvents_ToolUseWithKnownArgKey_Query(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_3","name":"search","input":{"query":"golang interfaces"}}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "golang interfaces", result[0].Arg)
}

func TestParseClaudeDisplayEvents_ToolUseWithKnownArgKey_Pattern(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_4","name":"grep","input":{"pattern":"func.*Error"}}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "func.*Error", result[0].Arg)
}

func TestParseClaudeDisplayEvents_ToolUseWithLongArg_Truncated(t *testing.T) {
	provider := NewClaudeProvider()

	longPath := "/very/long/path/that/exceeds/forty/characters/totally"
	require.Greater(t, len([]rune(longPath)), 40, "test fixture must be longer than 40 chars")

	lineJSON := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_5","name":"read_file","input":{"file_path":"` + longPath + `"}}]}}`
	result := provider.parseClaudeDisplayEvents([]byte(lineJSON))

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	// truncateArg yields 37 content runes + 1 ellipsis rune = 38 runes total.
	assert.Equal(t, 38, len([]rune(result[0].Arg)), "truncated arg must be 37 runes + ellipsis (U+2026)")
	assert.True(t, strings.HasSuffix(result[0].Arg, "…"), "truncated arg must end with ellipsis U+2026")
}

func TestParseClaudeDisplayEvents_ToolUseWithArgExactly40Chars_NotTruncated(t *testing.T) {
	provider := NewClaudeProvider()

	// Exactly 40 characters — must not be truncated.
	exact40 := strings.Repeat("a", 40)
	lineJSON := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_6","name":"tool","input":{"file_path":"` + exact40 + `"}}]}}`
	result := provider.parseClaudeDisplayEvents([]byte(lineJSON))

	require.Len(t, result, 1)
	assert.Equal(t, exact40, result[0].Arg)
	assert.False(t, strings.HasSuffix(result[0].Arg, "…"))
}

func TestParseClaudeDisplayEvents_ToolUseWithID(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_123","name":"bash","input":{"command":"pwd"}}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "toolu_123", result[0].ID)
}

func TestParseClaudeDisplayEvents_ToolUseWithNoRecognizedInputKey(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_7","name":"custom_tool","input":{"unknown":"value","other":"data"}}]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 1)
	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "", result[0].Arg, "no recognized key should yield empty arg")
}

func TestParseClaudeDisplayEvents_MultipleToolUseBlocks(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":{"content":[` +
		`{"type":"tool_use","id":"toolu_a","name":"read_file","input":{"file_path":"/a"}},` +
		`{"type":"tool_use","id":"toolu_b","name":"bash","input":{"command":"echo hi"}}` +
		`]}}`)
	result := provider.parseClaudeDisplayEvents(line)

	require.Len(t, result, 2, "all tool_use blocks must be emitted in order")

	assert.Equal(t, EventToolUse, result[0].Kind)
	assert.Equal(t, "read_file", result[0].Name)
	assert.Equal(t, "/a", result[0].Arg)
	assert.Equal(t, "toolu_a", result[0].ID)

	assert.Equal(t, EventToolUse, result[1].Kind)
	assert.Equal(t, "bash", result[1].Name)
	assert.Equal(t, "echo hi", result[1].Arg)
	assert.Equal(t, "toolu_b", result[1].ID)
}

func TestParseClaudeDisplayEvents_SystemEventReturnsNil(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"system","subtype":"init","session_id":"sess_xyz"}`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Nil(t, result, "system events must return nil")
}

func TestParseClaudeDisplayEvents_RateLimitEventReturnsNil(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"rate_limit_event","retry_after_ms":5000}`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Nil(t, result, "rate_limit_event must return nil")
}

func TestParseClaudeDisplayEvents_MalformedJSONReturnsNil(t *testing.T) {
	provider := NewClaudeProvider()

	line := []byte(`{"type":"assistant","message":`)
	result := provider.parseClaudeDisplayEvents(line)

	assert.Nil(t, result, "malformed JSON must return nil")
}

// --- truncateArg unit tests ---

func TestTruncateArg_ShortString_Unchanged(t *testing.T) {
	assert.Equal(t, "hello", truncateArg("hello"))
}

func TestTruncateArg_Exactly40Chars_Unchanged(t *testing.T) {
	s := strings.Repeat("x", 40)
	assert.Equal(t, s, truncateArg(s))
}

func TestTruncateArg_41Chars_Truncated(t *testing.T) {
	s := strings.Repeat("a", 41)
	result := truncateArg(s)
	// 37 content runes + 1 ellipsis rune = 38 runes total.
	assert.Equal(t, 38, len([]rune(result)))
	assert.True(t, strings.HasSuffix(result, "…"))
}

func TestTruncateArg_UnicodeMultibyte_CountsByRune(t *testing.T) {
	// 38 two-byte runes — still ≤40, must be returned as-is.
	s := strings.Repeat("é", 38)
	assert.Equal(t, s, truncateArg(s))

	// 41 two-byte runes — must be truncated to 37 runes + ellipsis.
	long := strings.Repeat("é", 41)
	result := truncateArg(long)
	assert.Equal(t, 38, len([]rune(result)))
	assert.True(t, strings.HasSuffix(result, "…"))
}
