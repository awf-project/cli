package output_test

import (
	"strings"
	"testing"

	"github.com/awf-project/awf/pkg/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F065

func TestStripCodeFences_NoFences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text without fences",
			input: `{"key":"value"}`,
			want:  `{"key":"value"}`,
		},
		{
			name:  "multiline text without fences",
			input: "line1\nline2\nline3",
			want:  "line1\nline2\nline3",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   \n  \t  ",
			want:  "   \n  \t  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := output.StripCodeFences(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStripCodeFences_WithFences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "json code fence",
			input: "```json\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			name:  "bash code fence",
			input: "```bash\necho hello\n```",
			want:  "echo hello",
		},
		{
			name:  "no language tag",
			input: "```\ncontent here\n```",
			want:  "content here",
		},
		{
			name:  "multiline content",
			input: "```json\n{\n  \"name\": \"alice\",\n  \"count\": 3\n}\n```",
			want:  "{\n  \"name\": \"alice\",\n  \"count\": 3\n}",
		},
		{
			name:  "leading whitespace before fence",
			input: "  ```json\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			name:  "trailing whitespace after fence",
			input: "```json\n{\"key\":\"value\"}\n```  ",
			want:  `{"key":"value"}`,
		},
		{
			name:  "both leading and trailing whitespace",
			input: "  ```json\n{\"key\":\"value\"}\n```  \n",
			want:  `{"key":"value"}`,
		},
		{
			name:  "empty content inside fences",
			input: "```json\n\n```",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := output.StripCodeFences(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStripCodeFences_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "only outermost fence stripped when nested",
			input: "```json\n```inner\ncode\n```\n```",
			want:  "```inner\ncode\n```",
		},
		{
			name:  "multiple fence blocks - only first processed",
			input: "```json\n{\"a\":1}\n```\nsome text\n```json\n{\"b\":2}\n```",
			want:  `{"a":1}`,
		},
		{
			name:  "windows line endings CRLF",
			input: "```json\r\n{\"key\":\"value\"}\r\n```",
			want:  `{"key":"value"}`,
		},
		{
			name:  "mixed line endings",
			input: "```json\r\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			name:  "fence with capital letters in language tag",
			input: "```JSON\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			name:  "fence with numbers in language tag",
			input: "```python3\nprint('hello')\n```",
			want:  "print('hello')",
		},
		{
			name:  "incomplete fence - opening only",
			input: "```json\n{\"key\":\"value\"}",
			want:  "```json\n{\"key\":\"value\"}",
		},
		{
			name:  "incomplete fence - closing only",
			input: "{\"key\":\"value\"}\n```",
			want:  "{\"key\":\"value\"}\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := output.StripCodeFences(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStripCodeFences_LargeInput(t *testing.T) {
	largeContent := strings.Repeat("x", 1024*1024)
	input := "```json\n" + largeContent + "\n```"

	got := output.StripCodeFences(input)

	assert.Equal(t, largeContent, got)
}

func TestValidateAndParseJSON_ValidJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
	}{
		{
			name:     "valid object",
			input:    `{"key":"value"}`,
			wantType: "map[string]any",
		},
		{
			name:     "valid object with nested fields",
			input:    `{"name":"alice","count":3,"nested":{"inner":"value"}}`,
			wantType: "map[string]any",
		},
		{
			name:     "valid array",
			input:    `[1,2,3]`,
			wantType: "[]any",
		},
		{
			name:     "valid array of objects",
			input:    `[{"id":1},{"id":2}]`,
			wantType: "[]any",
		},
		{
			name:     "valid empty object",
			input:    `{}`,
			wantType: "map[string]any",
		},
		{
			name:     "valid empty array",
			input:    `[]`,
			wantType: "[]any",
		},
		{
			name:     "valid with whitespace",
			input:    `  {"key": "value"}  `,
			wantType: "map[string]any",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := output.ValidateAndParseJSON(tt.input)
			require.NoError(t, err)
			require.NotNil(t, got)

			switch tt.wantType {
			case "map[string]any":
				_, ok := got.(map[string]any)
				assert.True(t, ok, "expected map[string]any, got %T", got)
			case "[]any":
				_, ok := got.([]any)
				assert.True(t, ok, "expected []any, got %T", got)
			}
		})
	}
}

func TestValidateAndParseJSON_ValidJSONContent(t *testing.T) {
	input := `{"name":"alice","count":3}`

	got, err := output.ValidateAndParseJSON(input)
	require.NoError(t, err)

	result, ok := got.(map[string]any)
	require.True(t, ok, "expected map[string]any")
	assert.Equal(t, "alice", result["name"])
	assert.Equal(t, float64(3), result["count"])
}

func TestValidateAndParseJSON_InvalidJSON(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		wantErrContains   string
		wantPreviewPrefix string
	}{
		{
			name:              "malformed object",
			input:             `{"key":`,
			wantErrContains:   "invalid JSON",
			wantPreviewPrefix: `{"key":`,
		},
		{
			name:              "unquoted key",
			input:             `{key:"value"}`,
			wantErrContains:   "invalid JSON",
			wantPreviewPrefix: `{key:"value"}`,
		},
		{
			name:              "trailing comma",
			input:             `{"key":"value",}`,
			wantErrContains:   "invalid JSON",
			wantPreviewPrefix: `{"key":"value",}`,
		},
		{
			name:              "single quoted string",
			input:             `{'key':'value'}`,
			wantErrContains:   "invalid JSON",
			wantPreviewPrefix: `{'key':'value'}`,
		},
		{
			name:              "empty string",
			input:             "",
			wantErrContains:   "invalid JSON",
			wantPreviewPrefix: "",
		},
		{
			name:              "whitespace only",
			input:             "   \n  ",
			wantErrContains:   "invalid JSON",
			wantPreviewPrefix: "   \n  ",
		},
		{
			name:              "plain text",
			input:             "not json at all",
			wantErrContains:   "invalid JSON",
			wantPreviewPrefix: "not json at all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := output.ValidateAndParseJSON(tt.input)
			require.Error(t, err)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), tt.wantErrContains)

			if tt.input != "" {
				expectedPreview := tt.input
				if len(expectedPreview) > 200 {
					expectedPreview = expectedPreview[:200]
				}
				assert.Contains(t, err.Error(), expectedPreview)
			}
		})
	}
}

func TestValidateAndParseJSON_ErrorPreviewTruncation(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 300; i++ {
		b.WriteString(string(rune('0' + (i % 10))))
		b.WriteString(string(rune('a' + (i % 26))))
	}
	longInput := b.String()

	got, err := output.ValidateAndParseJSON(longInput)
	require.Error(t, err)
	assert.Nil(t, got)

	errMsg := err.Error()
	assert.Contains(t, errMsg, longInput[:200])
	assert.NotContains(t, errMsg, longInput[201:])
}

func TestProcessOutputFormat_JSON(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantOutput  string
		wantParsed  map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name:       "json format with fences and valid JSON",
			output:     "```json\n{\"key\":\"value\"}\n```",
			wantOutput: `{"key":"value"}`,
			wantParsed: map[string]any{"key": "value"},
		},
		{
			name:       "json format with valid JSON no fences",
			output:     `{"key":"value"}`,
			wantOutput: `{"key":"value"}`,
			wantParsed: map[string]any{"key": "value"},
		},
		{
			name:       "json format with nested object",
			output:     "```json\n{\"user\":{\"name\":\"alice\",\"age\":30}}\n```",
			wantOutput: `{"user":{"name":"alice","age":30}}`,
			wantParsed: map[string]any{"user": map[string]any{"name": "alice", "age": float64(30)}},
		},
		{
			name:        "json format with invalid JSON after stripping",
			output:      "```json\n{\"key\":\n```",
			wantErr:     true,
			errContains: "invalid JSON",
		},
		{
			name:        "json format with plain text",
			output:      "```json\nnot json\n```",
			wantErr:     true,
			errContains: "invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processed, parsed, err := output.ProcessOutputFormat(tt.output, "json")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, processed)

			result, ok := parsed.(map[string]any)
			require.True(t, ok, "expected map[string]any, got %T", parsed)

			for k, v := range tt.wantParsed {
				assert.Equal(t, v, result[k])
			}
		})
	}
}

func TestProcessOutputFormat_Text(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantOutput string
	}{
		{
			name:       "text format with bash fences",
			output:     "```bash\necho hello\n```",
			wantOutput: "echo hello",
		},
		{
			name:       "text format with json fences (no validation)",
			output:     "```json\n{\"key\":\"value\"}\n```",
			wantOutput: `{"key":"value"}`,
		},
		{
			name:       "text format with plain text",
			output:     "```\nplain text content\n```",
			wantOutput: "plain text content",
		},
		{
			name:       "text format with no fences",
			output:     "raw text output",
			wantOutput: "raw text output",
		},
		{
			name:       "text format with python fences",
			output:     "```python\nprint('hello')\n```",
			wantOutput: "print('hello')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processed, parsed, err := output.ProcessOutputFormat(tt.output, "text")

			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, processed)
			assert.Nil(t, parsed)
		})
	}
}

func TestProcessOutputFormat_Empty(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantOutput string
	}{
		{
			name:       "empty format with fences (no stripping)",
			output:     "```json\n{\"key\":\"value\"}\n```",
			wantOutput: "```json\n{\"key\":\"value\"}\n```",
		},
		{
			name:       "empty format with plain text",
			output:     "raw output",
			wantOutput: "raw output",
		},
		{
			name:       "empty format preserves everything",
			output:     "line1\n```code\nstuff\n```\nline2",
			wantOutput: "line1\n```code\nstuff\n```\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processed, parsed, err := output.ProcessOutputFormat(tt.output, "")

			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, processed)
			assert.Nil(t, parsed)
		})
	}
}

func TestProcessOutputFormat_BackwardCompatibility(t *testing.T) {
	rawOutput := "Agent response with ```json\n{\"key\":\"value\"}\n``` embedded"

	processed, parsed, err := output.ProcessOutputFormat(rawOutput, "")

	require.NoError(t, err)
	assert.Equal(t, rawOutput, processed)
	assert.Nil(t, parsed)
}
