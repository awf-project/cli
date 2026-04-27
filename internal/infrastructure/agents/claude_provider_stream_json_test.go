package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F078 Tests: Fix CLI Provider Invocations (output_format mapping and ExecuteConversation session handling)

// T001: Execute maps output_format and forces stream-json by default for live streaming
func TestClaudeProvider_Execute_OutputFormatMapping(t *testing.T) {
	// NDJSON result event used so extractTextFromJSON can populate clean output
	ndjson := []byte(`{"type":"system","subtype":"init"}
{"type":"result","subtype":"success","result":"test output","session_id":"sess-1"}`)

	tests := []struct {
		name           string
		options        map[string]any
		mockOutput     []byte
		wantFormatFlag string
		wantVerbose    bool
	}{
		{
			name:           "json format mapped to stream-json with --verbose",
			options:        map[string]any{"output_format": "json"},
			mockOutput:     ndjson,
			wantFormatFlag: "stream-json",
			wantVerbose:    true,
		},
		{
			name:           "stream-json format passed through with --verbose",
			options:        map[string]any{"output_format": "stream-json"},
			mockOutput:     ndjson,
			wantFormatFlag: "stream-json",
			wantVerbose:    true,
		},
		{
			name:           "no output format specified forces stream-json for live streaming",
			options:        map[string]any{},
			mockOutput:     ndjson,
			wantFormatFlag: "stream-json",
			wantVerbose:    true,
		},
		{
			// F082: Claude CLI is always invoked with stream-json; the text vs
			// json distinction is handled by the application-layer display filter,
			// not by toggling Claude's --output-format flag.
			name:           "explicit text format also maps to stream-json (F082)",
			options:        map[string]any{"output_format": "text"},
			mockOutput:     ndjson,
			wantFormatFlag: "stream-json",
			wantVerbose:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			_, err := provider.Execute(context.Background(), "test prompt", tt.options, nil, nil)

			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			call := calls[0]

			found := false
			for i, arg := range call.Args {
				if arg == "--output-format" && i+1 < len(call.Args) {
					assert.Equal(t, tt.wantFormatFlag, call.Args[i+1])
					found = true
					break
				}
			}
			assert.True(t, found, "expected --output-format flag not found in args: %v", call.Args)

			hasVerbose := false
			for _, arg := range call.Args {
				if arg == "--verbose" {
					hasVerbose = true
					break
				}
			}
			assert.Equal(t, tt.wantVerbose, hasVerbose, "--verbose presence mismatch in args: %v", call.Args)
		})
	}
}

// ExecuteConversation always forces stream-json for session ID extraction
func TestClaudeProvider_ExecuteConversation_ForcesStreamJSON(t *testing.T) {
	tests := []struct {
		name      string
		state     *workflow.ConversationState
		sessionID string
	}{
		{
			name:      "first turn (no sessionID)",
			state:     &workflow.ConversationState{Turns: []workflow.Turn{}},
			sessionID: "",
		},
		{
			name: "resume turn (with existing sessionID)",
			state: &workflow.ConversationState{
				SessionID: "session-123",
				Turns:     []workflow.Turn{},
			},
			sessionID: "session-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`{"type":"system","subtype":"init","session_id":"sess-456"}
{"type":"result","subtype":"success","result":"response text","session_id":"sess-456"}`), nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			_, err := provider.ExecuteConversation(context.Background(), tt.state, "user prompt", map[string]any{}, nil, nil)

			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			call := calls[0]

			// Assert --output-format stream-json is always present
			found := false
			for i, arg := range call.Args {
				if arg == "--output-format" && i+1 < len(call.Args) {
					assert.Equal(t, "stream-json", call.Args[i+1],
						"ExecuteConversation must force stream-json for session ID extraction")
					found = true
					break
				}
			}
			assert.True(t, found, "--output-format stream-json not found in args: %v", call.Args)

			// Verify session resume flag when sessionID exists
			if tt.sessionID != "" {
				hasResume := false
				for i, arg := range call.Args {
					if arg == "-r" && i+1 < len(call.Args) && call.Args[i+1] == tt.sessionID {
						hasResume = true
						break
					}
				}
				assert.True(t, hasResume, "expected -r %s flag in resume turn", tt.sessionID)
			}
		})
	}
}

// extractSessionID works with stream-json NDJSON output (one JSON object per line)
func TestClaudeProvider_ExtractSessionID_StreamJSON(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name      string
		output    string
		wantID    string
		wantError bool
	}{
		{
			name: "valid NDJSON with session_id in result event",
			output: `{"type":"system","subtype":"init","session_id":"sess-abc123"}
{"type":"result","subtype":"success","result":"text","session_id":"sess-abc123","cost_usd":0.001}`,
			wantID:    "sess-abc123",
			wantError: false,
		},
		{
			name: "valid NDJSON with multiple events before result",
			output: `{"type":"system","subtype":"init"}
{"type":"assistant","message":{}}
{"type":"result","subtype":"success","result":"text","session_id":"sess-xyz789"}`,
			wantID:    "sess-xyz789",
			wantError: false,
		},
		{
			name:      "result event missing session_id",
			output:    `{"type":"result","subtype":"success","result":"text"}`,
			wantID:    "",
			wantError: true,
		},
		{
			name: "no result event in stream",
			output: `{"type":"system","subtype":"init"}
{"type":"assistant","message":{}}`,
			wantID:    "",
			wantError: true,
		},
		{
			name:      "all invalid JSON lines",
			output:    `not valid json`,
			wantID:    "",
			wantError: true,
		},
		{
			name:      "empty output",
			output:    "",
			wantID:    "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := provider.extractSessionID(tt.output)

			if tt.wantError {
				assert.Error(t, err)
				assert.Equal(t, "", id)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}

// extractTextFromJSON works with stream-json NDJSON output
func TestClaudeProvider_ExtractTextFromJSON_StreamJSON(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name     string
		output   string
		wantText string
	}{
		{
			name: "valid NDJSON with result event",
			output: `{"type":"system","subtype":"init"}
{"type":"result","subtype":"success","result":"Hello, this is the response","session_id":"sess-123"}`,
			wantText: "Hello, this is the response",
		},
		{
			name:     "NDJSON with multiline result",
			output:   `{"type":"result","subtype":"success","result":"Line 1\nLine 2\nLine 3","session_id":"sess-456"}`,
			wantText: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "NDJSON with empty result",
			output:   `{"type":"result","subtype":"success","result":"","session_id":"sess-789"}`,
			wantText: "",
		},
		{
			name:     "non-JSON output (graceful fallback)",
			output:   `plain text response`,
			wantText: "",
		},
		{
			name:     "invalid JSON lines (graceful fallback)",
			output:   `{"result": invalid}`,
			wantText: "",
		},
		{
			name:     "no result event in stream",
			output:   `{"type":"system","subtype":"init","session_id":"sess-999"}`,
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := provider.extractTextFromJSON(tt.output)
			assert.Equal(t, tt.wantText, text)
		})
	}
}

// Execute with invalid input
func TestClaudeProvider_Execute_ErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		prompt    string
		options   map[string]any
		wantError bool
		errMsg    string
	}{
		{
			name:      "empty prompt",
			prompt:    "",
			options:   map[string]any{},
			wantError: true,
			errMsg:    "prompt cannot be empty",
		},
		{
			name:      "invalid model format",
			prompt:    "test",
			options:   map[string]any{"model": "invalid-model"},
			wantError: true,
			errMsg:    "invalid model format",
		},
		{
			name:      "valid model alias",
			prompt:    "test",
			options:   map[string]any{"model": "sonnet"},
			wantError: false,
		},
		{
			name:      "valid claude-prefixed model",
			prompt:    "test",
			options:   map[string]any{"model": "claude-3-opus"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			if !tt.wantError {
				mockExec.SetOutput([]byte("response"), nil)
			}
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ExecuteConversation with nil state
func TestClaudeProvider_ExecuteConversation_NilState(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	_, err := provider.ExecuteConversation(context.Background(), nil, "prompt", map[string]any{}, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conversation state cannot be nil")
}

// Regression: lifecycle events ahead of the result event must not leak into result.Output.
func TestClaudeProvider_Execute_JSONFormat_NDJSONLifecycleEvents(t *testing.T) {
	agentJSON := `{"colors":["red","blue"]}`
	resultEvent, err := json.Marshal(map[string]any{
		"type":       "result",
		"subtype":    "success",
		"result":     agentJSON,
		"session_id": "sess-1",
	})
	require.NoError(t, err)

	ndjson := strings.Join([]string{
		`{"type":"system","subtype":"hook_started","hook_id":"hk-1","hook_name":"SessionStart:startup"}`,
		`{"type":"system","subtype":"hook_started","hook_id":"hk-2","hook_name":"SessionStart:startup"}`,
		`{"type":"system","subtype":"hook_response","output":"context"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"thinking..."}]}}`,
		string(resultEvent),
	}, "\n")

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "list 2 colors as JSON",
		map[string]any{"output_format": "json"}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, agentJSON, result.Output)

	var parsed any
	require.NoError(t, json.Unmarshal([]byte(result.Output), &parsed))

	require.NotNil(t, result.Response)
	assert.Equal(t, "result", result.Response["type"])
	assert.Equal(t, "sess-1", result.Response["session_id"])
}
