package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F078 Tests: Fix CLI Provider Invocations (output_format mapping and ExecuteConversation session handling)

// T001: Execute maps output_format: json to stream-json
func TestClaudeProvider_Execute_OutputFormatMapping(t *testing.T) {
	tests := []struct {
		name           string
		options        map[string]any
		mockOutput     []byte
		wantFormatFlag string
	}{
		{
			name:           "json format mapped to stream-json",
			options:        map[string]any{"output_format": "json"},
			mockOutput:     []byte(`{"result":"test output"}`),
			wantFormatFlag: "stream-json",
		},
		{
			name:           "stream-json format passed through",
			options:        map[string]any{"output_format": "stream-json"},
			mockOutput:     []byte("test output"),
			wantFormatFlag: "stream-json",
		},
		{
			name:           "no output format specified",
			options:        map[string]any{},
			mockOutput:     []byte("test output"),
			wantFormatFlag: "",
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

			if tt.wantFormatFlag != "" {
				// Find --output-format flag in args
				found := false
				for i, arg := range call.Args {
					if arg == "--output-format" && i+1 < len(call.Args) {
						assert.Equal(t, tt.wantFormatFlag, call.Args[i+1])
						found = true
						break
					}
				}
				assert.True(t, found, "expected --output-format flag not found in args: %v", call.Args)
			} else {
				// Ensure no --output-format flag
				for i, arg := range call.Args {
					if arg == "--output-format" {
						t.Errorf("unexpected --output-format flag in args: %v", call.Args)
					}
					_ = i
				}
			}
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
			mockExec.SetOutput([]byte(`{"session_id":"sess-456","result":"response text"}`), nil)
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

// extractSessionID works with stream-json output format
func TestClaudeProvider_ExtractSessionID_StreamJSON(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name      string
		output    string
		wantID    string
		wantError bool
	}{
		{
			name:      "valid stream-json with session_id",
			output:    `{"session_id":"sess-abc123","result":"text","cost_usd":0.001}`,
			wantID:    "sess-abc123",
			wantError: false,
		},
		{
			name:      "valid stream-json with different field order",
			output:    `{"result":"text","session_id":"sess-xyz789","cost_usd":0.002}`,
			wantID:    "sess-xyz789",
			wantError: false,
		},
		{
			name:      "missing session_id field",
			output:    `{"result":"text","cost_usd":0.001}`,
			wantID:    "",
			wantError: true,
		},
		{
			name:      "invalid JSON",
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

// extractTextFromJSON works with stream-json output
func TestClaudeProvider_ExtractTextFromJSON_StreamJSON(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name     string
		output   string
		wantText string
	}{
		{
			name:     "valid stream-json with result field",
			output:   `{"session_id":"sess-123","result":"Hello, this is the response","cost_usd":0.001}`,
			wantText: "Hello, this is the response",
		},
		{
			name:     "stream-json with multiline result",
			output:   `{"session_id":"sess-456","result":"Line 1\nLine 2\nLine 3"}`,
			wantText: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "stream-json with empty result",
			output:   `{"session_id":"sess-789","result":""}`,
			wantText: "",
		},
		{
			name:     "non-JSON output (graceful fallback)",
			output:   `plain text response`,
			wantText: "",
		},
		{
			name:     "invalid JSON (graceful fallback)",
			output:   `{"result": invalid}`,
			wantText: "",
		},
		{
			name:     "missing result field (graceful fallback)",
			output:   `{"session_id":"sess-999","cost_usd":0.001}`,
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
