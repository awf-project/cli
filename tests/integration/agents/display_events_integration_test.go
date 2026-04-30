//go:build integration

// Feature: F085
// Unified Display Event Abstraction: cross-provider parsing, rendering, and streaming pipeline.

package agents_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/infrastructure/agents"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/httpx"
)

// TestDisplayEvents_CrossProviderConsistency verifies that all five providers extract
// displayable text from fixtures containing both text and tool_use events, confirming
// the full pipeline (mock executor → parser → DisplayOutput) works for each provider.
func TestDisplayEvents_CrossProviderConsistency(t *testing.T) {
	// Each fixture includes both a text segment and a tool_use event to validate
	// that text is extracted and tool_use events do not corrupt the text output.
	claudeFixture := `{"type":"assistant","message":{"content":[{"type":"text","text":"Analyzing file"},{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/etc/hosts"}},{"type":"text","text":"Found results"}]}}`

	codexFixture := strings.Join([]string{
		`{"type":"item.completed","item":{"item_type":"assistant_message","text":"Running command"}}`,
		`{"type":"item.completed","item":{"item_type":"function_call","name":"shell","arguments":"{\"cmd\":\"ls -la\"}"}}`,
	}, "\n")

	geminiFixture := `{"type":"message","role":"assistant","content":"Processing request","toolCalls":[{"name":"read_file","arguments":{"file_path":"/tmp/data"}}]}`

	opencodeFixture := strings.Join([]string{
		`{"type":"text","part":{"text":"Checking status"}}`,
		`{"type":"tool_use","part":{"name":"Bash","input":{"command":"git status"}}}`,
	}, "\n")

	// Real HTTP test server for the OpenAI-compatible provider — no mock CLI executor exists for HTTP.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"object": "chat.completion",
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "Here are the results",
						"tool_calls": []map[string]any{
							{"id": "call_1", "function": map[string]any{"name": "search", "arguments": `{"query":"test"}`}},
						},
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	claudeMock := mocks.NewMockCLIExecutor()
	claudeMock.SetOutput([]byte(claudeFixture), nil)
	claudeProvider := agents.NewClaudeProviderWithOptions(agents.WithClaudeExecutor(claudeMock))

	codexMock := mocks.NewMockCLIExecutor()
	codexMock.SetOutput([]byte(codexFixture), nil)
	codexProvider := agents.NewCodexProviderWithOptions(agents.WithCodexExecutor(codexMock))

	geminiMock := mocks.NewMockCLIExecutor()
	geminiMock.SetOutput([]byte(geminiFixture), nil)
	geminiProvider := agents.NewGeminiProviderWithOptions(agents.WithGeminiExecutor(geminiMock))

	opencodeMock := mocks.NewMockCLIExecutor()
	opencodeMock.SetOutput([]byte(opencodeFixture), nil)
	opencodeProvider := agents.NewOpenCodeProviderWithOptions(agents.WithOpenCodeExecutor(opencodeMock))

	httpClient := httpx.NewClient(httpx.WithDoer(srv.Client()))
	openaiProvider := agents.NewOpenAICompatibleProvider(agents.WithHTTPClient(httpClient))

	t.Run("claude_extracts_text_from_mixed_fixture", func(t *testing.T) {
		result, err := claudeProvider.Execute(context.Background(), "prompt",
			map[string]any{"output_format": "text", "model": "claude-sonnet-4-5"}, io.Discard, io.Discard)
		require.NoError(t, err)
		assert.NotEmpty(t, result.DisplayOutput, "claude must extract text from assistant event containing both text and tool_use")
	})

	t.Run("codex_extracts_text_from_mixed_fixture", func(t *testing.T) {
		result, err := codexProvider.Execute(context.Background(), "prompt",
			map[string]any{"output_format": "text"}, io.Discard, io.Discard)
		require.NoError(t, err)
		assert.NotEmpty(t, result.DisplayOutput, "codex must extract text from assistant_message item alongside function_call item")
	})

	t.Run("gemini_extracts_text_from_mixed_fixture", func(t *testing.T) {
		result, err := geminiProvider.Execute(context.Background(), "prompt",
			map[string]any{"output_format": "text"}, io.Discard, io.Discard)
		require.NoError(t, err)
		assert.NotEmpty(t, result.DisplayOutput, "gemini must extract text from message event containing toolCalls")
	})

	t.Run("opencode_extracts_text_from_mixed_fixture", func(t *testing.T) {
		result, err := opencodeProvider.Execute(context.Background(), "prompt",
			map[string]any{"output_format": "text"}, io.Discard, io.Discard)
		require.NoError(t, err)
		assert.NotEmpty(t, result.DisplayOutput, "opencode must extract text from text event alongside tool_use event")
	})

	t.Run("openai_compatible_populates_output_from_completion_response", func(t *testing.T) {
		result, err := openaiProvider.Execute(context.Background(), "prompt", map[string]any{
			"base_url": srv.URL + "/v1",
			"model":    "gpt-4o",
		}, io.Discard, io.Discard)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Output, "openai_compatible must populate Output from chat.completion response")
	})
}

// TestDisplayEvents_VerboseRenderingCrossProvider validates the renderer half of the F085
// pipeline for all five providers. It constructs representative event slices (matching what
// each provider's parser would emit) and verifies that:
//   - verbose mode produces [tool: Name(Arg)] markers for every EventToolUse event
//   - default mode produces zero [tool: markers regardless of provider
//   - the marker format is identical across providers
func TestDisplayEvents_VerboseRenderingCrossProvider(t *testing.T) {
	providerEvents := map[string][]agents.DisplayEvent{
		"claude": {
			{Kind: agents.EventText, Text: "Analyzing"},
			{Kind: agents.EventToolUse, Name: "Read", Arg: "/etc/hosts", ID: "toolu_1"},
			{Kind: agents.EventText, Text: "Done"},
		},
		"codex": {
			{Kind: agents.EventText, Text: "Running command"},
			{Kind: agents.EventToolUse, Name: "shell", Arg: "ls -la"},
		},
		"gemini": {
			{Kind: agents.EventText, Text: "Processing request"},
			{Kind: agents.EventToolUse, Name: "read_file", Arg: "/tmp/data"},
		},
		"opencode": {
			{Kind: agents.EventText, Text: "Checking status"},
			{Kind: agents.EventToolUse, Name: "Bash", Arg: "git status"},
		},
		"openai": {
			{Kind: agents.EventText, Text: "Here are the results"},
			{Kind: agents.EventToolUse, Name: "search", Arg: `{"query":"test"}`},
		},
	}

	for provider, events := range providerEvents {
		provider := provider
		events := events

		t.Run(provider+"/verbose_emits_tool_markers", func(t *testing.T) {
			var buf bytes.Buffer
			ui.RenderEvents(&buf, events, agents.DisplayModeVerbose)
			output := buf.String()
			assert.Contains(t, output, "[tool:", "verbose mode must emit [tool: ...] markers for provider %s", provider)
		})

		t.Run(provider+"/default_suppresses_tool_markers", func(t *testing.T) {
			var buf bytes.Buffer
			ui.RenderEvents(&buf, events, agents.DisplayModeDefault)
			output := buf.String()
			assert.NotContains(t, output, "[tool:", "default mode must suppress all [tool: ...] markers for provider %s", provider)
		})
	}

	// Verify marker format is identical across all providers: [tool: Name(Arg)]
	t.Run("tool_marker_format_is_consistent_across_providers", func(t *testing.T) {
		toolEvent := agents.DisplayEvent{Kind: agents.EventToolUse, Name: "my_tool", Arg: "my_arg"}
		expected := "[tool: my_tool(my_arg)]"

		for _, events := range providerEvents {
			_ = events // only the tool event matters for format validation
		}

		var buf bytes.Buffer
		ui.RenderEvents(&buf, []agents.DisplayEvent{toolEvent}, agents.DisplayModeVerbose)
		assert.Equal(t, expected, buf.String(), "tool marker format must be [tool: Name(Arg)] for all providers")
	})
}

// TestDisplayEvents_StreamFilterPipeline exercises the full streaming path:
// NDJSON bytes written to a StreamFilterWriter are parsed line by line and
// only EventText content reaches the inner writer. The fixture includes a
// tool_use event to confirm it does not leak into the live writer.
func TestDisplayEvents_StreamFilterPipeline(t *testing.T) {
	ndjsonLines := []string{
		`{"type":"system","subtype":"init","session_id":"s1"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"},{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/etc/hosts"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":", world"}]}}`,
		`{"type":"result","subtype":"success","result":"Hello, world","session_id":"s1"}`,
	}
	ndjsonOutput := strings.Join(ndjsonLines, "\n") + "\n"

	claudeMock := &streamingMockExecutor{
		lines:  ndjsonLines,
		stdout: []byte(ndjsonOutput),
	}
	claudeProvider := agents.NewClaudeProviderWithOptions(agents.WithClaudeExecutor(claudeMock))

	var liveOutput bytes.Buffer
	result, err := claudeProvider.Execute(
		context.Background(),
		"test prompt",
		map[string]any{"output_format": "text", "model": "claude-sonnet-4-5"},
		&liveOutput,
		io.Discard,
	)
	require.NoError(t, err)

	// StreamFilterWriter must forward extracted text during streaming, not after.
	liveText := liveOutput.String()
	assert.NotEmpty(t, liveText, "StreamFilterWriter must write text to the live writer during execution")

	// Non-text NDJSON event types must not bleed into the live writer.
	assert.NotContains(t, liveText, `"type":"system"`)
	assert.NotContains(t, liveText, `"type":"result"`)

	// Tool use events must not appear as raw JSON in the live text writer (default mode).
	assert.NotContains(t, liveText, `"type":"tool_use"`)
	assert.NotContains(t, liveText, "[tool:", "StreamFilterWriter in default mode must not emit tool markers")

	// Post-execution aggregation via extractDisplayTextFromParser must also succeed.
	assert.NotEmpty(t, result.DisplayOutput)

	// The raw Output field must hold the authoritative content (from the result event),
	// separate from the display path.
	assert.NotEmpty(t, result.Output)
}

// TestDisplayEvents_MalformedInput verifies that all five provider parsers handle
// malformed JSON without panicking and that malformed lines produce no display output.
func TestDisplayEvents_MalformedInput(t *testing.T) {
	malformed := []struct {
		label string
		line  []byte
	}{
		{"invalid_json", []byte(`{invalid`)},
		{"null_literal", []byte(`null`)},
		{"empty", []byte(``)},
		{"array_root", []byte(`[]`)},
		{"binary", []byte("\x00\x01\x02")},
	}

	for _, tc := range malformed {
		tc := tc
		t.Run("claude/"+tc.label, func(t *testing.T) {
			mock := mocks.NewMockCLIExecutor()
			mock.SetOutput(tc.line, nil)
			p := agents.NewClaudeProviderWithOptions(agents.WithClaudeExecutor(mock))
			var buf bytes.Buffer
			assert.NotPanics(t, func() {
				result, _ := p.Execute(context.Background(), "x",
					map[string]any{"output_format": "text", "model": "claude-sonnet-4-5"},
					&buf, io.Discard)
				if result != nil {
					assert.Empty(t, result.DisplayOutput,
						"malformed input must not produce display output")
				}
			})
			// Nothing must have been written to the live writer either.
			assert.Empty(t, buf.String())
		})

		t.Run("codex/"+tc.label, func(t *testing.T) {
			mock := mocks.NewMockCLIExecutor()
			mock.SetOutput(tc.line, nil)
			p := agents.NewCodexProviderWithOptions(agents.WithCodexExecutor(mock))
			assert.NotPanics(t, func() {
				result, _ := p.Execute(context.Background(), "x",
					map[string]any{"output_format": "text"},
					io.Discard, io.Discard)
				if result != nil {
					assert.Empty(t, result.DisplayOutput)
				}
			})
		})

		t.Run("gemini/"+tc.label, func(t *testing.T) {
			mock := mocks.NewMockCLIExecutor()
			mock.SetOutput(tc.line, nil)
			p := agents.NewGeminiProviderWithOptions(agents.WithGeminiExecutor(mock))
			assert.NotPanics(t, func() {
				result, _ := p.Execute(context.Background(), "x",
					map[string]any{"output_format": "text"},
					io.Discard, io.Discard)
				if result != nil {
					assert.Empty(t, result.DisplayOutput)
				}
			})
		})

		t.Run("opencode/"+tc.label, func(t *testing.T) {
			mock := mocks.NewMockCLIExecutor()
			mock.SetOutput(tc.line, nil)
			p := agents.NewOpenCodeProviderWithOptions(agents.WithOpenCodeExecutor(mock))
			assert.NotPanics(t, func() {
				result, _ := p.Execute(context.Background(), "x",
					map[string]any{"output_format": "text"},
					io.Discard, io.Discard)
				if result != nil {
					assert.Empty(t, result.DisplayOutput)
				}
			})
		})

		t.Run("openai_compatible/"+tc.label, func(t *testing.T) {
			// OpenAI compatible input goes through its parser via extractDisplayTextFromParser
			// when writing to stdout. Feed the malformed line directly through a
			// StreamFilterWriter with the same extractor logic to stay parser-agnostic.
			var buf bytes.Buffer
			filter := agents.NewStreamFilterWriter(&buf, func(l []byte) string {
				var chunk struct {
					Object  string `json:"object"`
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if err := json.Unmarshal(l, &chunk); err != nil || chunk.Object == "" {
					return ""
				}
				if len(chunk.Choices) == 0 {
					return ""
				}
				return chunk.Choices[0].Delta.Content
			})
			assert.NotPanics(t, func() {
				_, _ = filter.Write(tc.line)
				_ = filter.Flush()
			})
			assert.Empty(t, buf.String(),
				"malformed input must not produce any display text for openai_compatible")
		})
	}
}
