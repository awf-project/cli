package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T002: Test ClaudeProvider delegation to baseCLIProvider
// These tests verify that buildExecuteArgs and buildConversationArgs
// correctly construct CLI arguments that are passed to baseCLIProvider.

func TestClaudeProvider_buildExecuteArgs_HappyPath(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name     string
		prompt   string
		options  map[string]any
		wantArgs []string
		wantErr  bool
	}{
		{
			name:     "basic prompt with no options",
			prompt:   "What is 2+2?",
			options:  nil,
			wantArgs: []string{"-p", "What is 2+2?", "--output-format", "stream-json", "--verbose"},
			wantErr:  false,
		},
		{
			name:    "prompt with model option",
			prompt:  "test",
			options: map[string]any{"model": "sonnet"},
			wantArgs: []string{
				"-p", "test",
				"--model", "sonnet",
				"--output-format", "stream-json", "--verbose",
			},
			wantErr: false,
		},
		{
			name:    "prompt with output format json",
			prompt:  "list colors",
			options: map[string]any{"output_format": "json"},
			wantArgs: []string{
				"-p", "list colors",
				"--output-format", "stream-json", "--verbose",
			},
			wantErr: false,
		},
		{
			name:    "prompt with allowed_tools option",
			prompt:  "execute code",
			options: map[string]any{"allowed_tools": "bash,read"},
			wantArgs: []string{
				"-p", "execute code",
				"--output-format", "stream-json", "--verbose",
				"--allowedTools", "bash,read",
			},
			wantErr: false,
		},
		{
			name:    "prompt with dangerously_skip_permissions",
			prompt:  "test",
			options: map[string]any{"dangerously_skip_permissions": true},
			wantArgs: []string{
				"-p", "test",
				"--output-format", "stream-json", "--verbose",
				"--dangerously-skip-permissions",
			},
			wantErr: false,
		},
		{
			name:    "empty options map",
			prompt:  "test",
			options: map[string]any{},
			wantArgs: []string{
				"-p", "test",
				"--output-format", "stream-json", "--verbose",
			},
			wantErr: false,
		},
		{
			name:    "special characters in prompt",
			prompt:  `What's the answer to "life, the universe, & everything"?`,
			options: nil,
			wantArgs: []string{
				"-p", `What's the answer to "life, the universe, & everything"?`,
				"--output-format", "stream-json", "--verbose",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := provider.buildExecuteArgs(tt.prompt, tt.options)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err, "should not return error")
				require.NotNil(t, args, "args should not be nil")
				assert.Equal(t, tt.wantArgs, args, "CLI arguments should match expected format")
			}
		})
	}
}

func TestClaudeProvider_buildConversationArgs_HappyPath(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name           string
		stateSessionID string
		prompt         string
		options        map[string]any
		wantArgs       []string
		wantErr        bool
	}{
		{
			name:           "new conversation with no session",
			stateSessionID: "",
			prompt:         "Hello",
			options:        nil,
			wantArgs: []string{
				"-p", "Hello",
				"--output-format", "stream-json", "--verbose",
			},
			wantErr: false,
		},
		{
			name:           "resume conversation with session ID",
			stateSessionID: "session-xyz-123",
			prompt:         "Continue from before",
			options:        nil,
			wantArgs: []string{
				"-p", "Continue from before",
				"--output-format", "stream-json", "--verbose",
				"-r", "session-xyz-123",
			},
			wantErr: false,
		},
		{
			name:           "conversation with model option",
			stateSessionID: "",
			prompt:         "test",
			options:        map[string]any{"model": "opus"},
			wantArgs: []string{
				"-p", "test",
				"--model", "opus",
				"--output-format", "stream-json", "--verbose",
			},
			wantErr: false,
		},
		{
			name:           "conversation forces stream-json format",
			stateSessionID: "",
			prompt:         "test",
			options:        map[string]any{"output_format": "json"},
			wantArgs: []string{
				"-p", "test",
				"--output-format", "stream-json", "--verbose",
			},
			wantErr: false,
		},
		{
			name:           "conversation with allowed_tools",
			stateSessionID: "s1",
			prompt:         "code",
			options:        map[string]any{"allowed_tools": "bash"},
			wantArgs: []string{
				"-p", "code",
				"--output-format", "stream-json", "--verbose",
				"-r", "s1",
				"--allowedTools", "bash",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &workflow.ConversationState{
				SessionID: tt.stateSessionID,
				Turns:     make([]workflow.Turn, 0),
			}

			args, err := provider.buildConversationArgs(state, tt.prompt, tt.options)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err, "should not return error")
				require.NotNil(t, args, "args should not be nil")
				assert.Equal(t, tt.wantArgs, args, "CLI arguments should match expected format")
			}
		})
	}
}

func TestClaudeProvider_DelegationIntegration_Execute(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()

	ndjsonOutput := `{"type":"system","subtype":"init"}
{"type":"assistant","content":"Hello"}
{"type":"result","result":"Hello, how can I help?","session_id":"sess-123"}`

	mockExec.SetOutput([]byte(ndjsonOutput), nil)

	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "Hello", nil, nil, nil)

	require.NoError(t, err, "Execute should succeed with proper args")
	require.NotNil(t, result, "result should not be nil")

	assert.Equal(t, "claude", result.Provider)
	assert.Equal(t, true, result.TokensEstimated)
	assert.NotZero(t, result.Tokens, "tokens should be estimated from output")
	assert.NotZero(t, result.StartedAt)
	assert.NotZero(t, result.CompletedAt)
}

func TestClaudeProvider_DelegationIntegration_ExecuteConversation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()

	ndjsonOutput := `{"type":"system","subtype":"init"}
{"type":"assistant","content":"I understand"}
{"type":"result","result":"Got it!","session_id":"sess-456"}`

	mockExec.SetOutput([]byte(ndjsonOutput), nil)

	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	state := &workflow.ConversationState{
		SessionID: "",
		Turns:     make([]workflow.Turn, 0),
	}

	result, err := provider.ExecuteConversation(context.Background(), state, "Continue", nil, nil, nil)

	require.NoError(t, err, "ExecuteConversation should succeed with proper args")
	require.NotNil(t, result, "result should not be nil")

	assert.Equal(t, "claude", result.Provider)
	assert.NotNil(t, result.State, "conversation state should not be nil")
	assert.True(t, result.TokensEstimated, "tokens should be estimated")
	assert.NotZero(t, result.TokensOutput, "TokensOutput should be calculated")
	assert.NotZero(t, result.StartedAt)
	assert.NotZero(t, result.CompletedAt)
}

func TestClaudeProvider_InvalidModel_ValidationError(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)

	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", map[string]any{"model": "gpt-4"}, nil, nil)

	assert.Error(t, err, "should reject invalid model")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid model format")
}

func TestClaudeProvider_ArgumentConstruction_CorrectlyFormatted(t *testing.T) {
	provider := NewClaudeProvider()

	args, err := provider.buildExecuteArgs("test prompt", map[string]any{
		"model":         "sonnet",
		"allowed_tools": "bash,read",
	})

	require.NoError(t, err)
	assert.NotNil(t, args)
	assert.Contains(t, args, "-p", "prompt flag should be present")
	assert.Contains(t, args, "test prompt", "prompt value should be present")
	assert.Contains(t, args, "--model", "model flag should be present")
	assert.Contains(t, args, "sonnet", "model value should be present")
	assert.Contains(t, args, "--allowedTools", "tools flag should be present")
	assert.Contains(t, args, "bash,read", "tools value should be present")
}

func TestClaudeProvider_SessionResumeArgs(t *testing.T) {
	provider := NewClaudeProvider()

	state := &workflow.ConversationState{
		SessionID: "my-session-id",
		Turns:     make([]workflow.Turn, 0),
	}

	args, err := provider.buildConversationArgs(state, "continue", nil)

	require.NoError(t, err)
	assert.NotNil(t, args)

	sessionFlagIdx := -1
	for i, arg := range args {
		if arg == "-r" {
			sessionFlagIdx = i
			break
		}
	}

	assert.Greater(t, sessionFlagIdx, -1, "should include -r flag for resume")
	if sessionFlagIdx >= 0 && sessionFlagIdx+1 < len(args) {
		assert.Equal(t, "my-session-id", args[sessionFlagIdx+1], "session ID should follow -r flag")
	}
}

func TestClaudeProvider_OutputFormatMapping(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name     string
		inputFmt string
		expected string
	}{
		{"json maps to stream-json", "json", "stream-json"},
		{"stream-json stays stream-json", "stream-json", "stream-json"},
		{"text stays text", "text", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := provider.buildExecuteArgs("test", map[string]any{
				"output_format": tt.inputFmt,
			})

			require.NoError(t, err)

			var found bool
			for i, arg := range args {
				if arg == "--output-format" && i+1 < len(args) {
					assert.Equal(t, tt.expected, args[i+1])
					found = true
					break
				}
			}
			assert.True(t, found, "output format flag should be present")
		})
	}
}

func TestClaudeProvider_ExecuteConversation_EmptyOutputGuard(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(""), nil)

	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	state := &workflow.ConversationState{
		SessionID: "",
		Turns:     make([]workflow.Turn, 0),
	}

	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, " ", result.Output, "empty output should be replaced with single space")
}

func TestClaudeProvider_ConversationAddsTurnsToState(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"result","result":"Response","session_id":"s123"}`), nil)

	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	state := &workflow.ConversationState{
		SessionID: "",
		Turns:     make([]workflow.Turn, 0),
	}

	result, err := provider.ExecuteConversation(context.Background(), state, "User prompt", nil, nil, nil)

	require.NoError(t, err)
	assert.NotNil(t, result.State)
	assert.GreaterOrEqual(t, len(result.State.Turns), 2, "state should have at least user and assistant turns")
}

func TestClaudeProvider_SessionIDExtraction(t *testing.T) {
	output := `{"type":"result","result":"text","session_id":"extracted-session-123"}`

	provider := NewClaudeProvider()
	sessionID, err := provider.extractSessionID(output)

	require.NoError(t, err)
	assert.Equal(t, "extracted-session-123", sessionID)
}

func TestClaudeProvider_SessionIDExtractionErrors(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		shouldFail bool
	}{
		{"empty output", "", true},
		{"missing session_id", `{"type":"result","result":"text"}`, true},
		{"null session_id", `{"type":"result","session_id":null}`, true},
		{"valid session_id", `{"type":"result","session_id":"s123"}`, false},
	}

	provider := NewClaudeProvider()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID, err := provider.extractSessionID(tt.output)

			if tt.shouldFail {
				assert.Error(t, err)
				assert.Equal(t, "", sessionID)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, sessionID)
			}
		})
	}
}
