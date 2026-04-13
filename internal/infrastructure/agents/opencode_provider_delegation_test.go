package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T005: Test OpenCodeProvider delegation to baseCLIProvider
// These tests verify that buildExecuteArgs and buildConversationArgs
// correctly construct CLI arguments that are passed to baseCLIProvider.

func TestOpenCodeProvider_buildExecuteArgs_HappyPath(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name     string
		prompt   string
		options  map[string]any
		wantArgs []string
		wantErr  bool
	}{
		{
			name:     "basic prompt with no options",
			prompt:   "Create a HTTP server",
			options:  nil,
			wantArgs: []string{"run", "Create a HTTP server", "--format", "json"},
			wantErr:  false,
		},
		{
			name:    "prompt with framework option",
			prompt:  "Build a web app",
			options: map[string]any{"framework": "gin"},
			wantArgs: []string{
				"run", "Build a web app",
				"--format", "json",
				"--framework", "gin",
			},
			wantErr: false,
		},
		{
			name:    "prompt with verbose option",
			prompt:  "Debug this issue",
			options: map[string]any{"verbose": true},
			wantArgs: []string{
				"run", "Debug this issue",
				"--format", "json",
				"--verbose",
			},
			wantErr: false,
		},
		{
			name:    "prompt with output_dir option",
			prompt:  "Generate project",
			options: map[string]any{"output_dir": "/tmp/project"},
			wantArgs: []string{
				"run", "Generate project",
				"--format", "json",
				"--output", "/tmp/project",
			},
			wantErr: false,
		},
		{
			name:    "prompt with all options",
			prompt:  "Create app",
			options: map[string]any{"framework": "fastapi", "verbose": true, "output_dir": "/tmp"},
			wantArgs: []string{
				"run", "Create app",
				"--format", "json",
				"--framework", "fastapi",
				"--verbose",
				"--output", "/tmp",
			},
			wantErr: false,
		},
		{
			name:    "prompt with output_format json",
			prompt:  "generate code",
			options: map[string]any{"output_format": "json"},
			wantArgs: []string{
				"run", "generate code",
				"--format", "json",
			},
			wantErr: false,
		},
		{
			name:    "prompt with output_format text",
			prompt:  "test prompt",
			options: map[string]any{"output_format": "text"},
			wantArgs: []string{
				"run", "test prompt",
				"--format", "json",
			},
			wantErr: false,
		},
		{
			name:    "special characters in prompt",
			prompt:  `What's the best way to "deploy" & scale?`,
			options: nil,
			wantArgs: []string{
				"run", `What's the best way to "deploy" & scale?`,
				"--format", "json",
			},
			wantErr: false,
		},
		{
			name:     "empty options map",
			prompt:   "test",
			options:  map[string]any{},
			wantArgs: []string{"run", "test", "--format", "json"},
			wantErr:  false,
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

func TestOpenCodeProvider_buildConversationArgs_HappyPath(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name           string
		stateSessionID string
		stateTurns     []workflow.Turn
		prompt         string
		options        map[string]any
		wantArgs       []string
		wantErr        bool
	}{
		{
			name:           "new conversation with no session",
			stateSessionID: "",
			stateTurns:     []workflow.Turn{},
			prompt:         "Hello",
			options:        nil,
			wantArgs:       []string{"run", "Hello", "--format", "json"},
			wantErr:        false,
		},
		{
			name:           "resume conversation with session ID",
			stateSessionID: "session-xyz-123",
			stateTurns:     []workflow.Turn{{Role: workflow.TurnRoleUser, Content: "previous"}},
			prompt:         "Continue",
			options:        nil,
			wantArgs: []string{
				"run", "Continue",
				"--format", "json",
				"-s", "session-xyz-123",
			},
			wantErr: false,
		},
		{
			name:           "conversation with model option",
			stateSessionID: "",
			stateTurns:     []workflow.Turn{},
			prompt:         "test",
			options:        map[string]any{"model": "gpt-4"},
			wantArgs: []string{
				"run", "test",
				"--format", "json",
				"--model", "gpt-4",
			},
			wantErr: false,
		},
		{
			name:           "conversation with framework and verbose",
			stateSessionID: "",
			stateTurns:     []workflow.Turn{},
			prompt:         "test",
			options:        map[string]any{"framework": "django", "verbose": true},
			wantArgs: []string{
				"run", "test",
				"--format", "json",
				"--framework", "django",
				"--verbose",
			},
			wantErr: false,
		},
		{
			name:           "conversation when turns exist but no session ID",
			stateSessionID: "",
			stateTurns:     []workflow.Turn{{Role: workflow.TurnRoleUser, Content: "first"}, {Role: workflow.TurnRoleAssistant, Content: "response"}},
			prompt:         "next",
			options:        nil,
			wantArgs: []string{
				"run", "next",
				"--format", "json",
				"-c",
			},
			wantErr: false,
		},
		{
			name:           "resume with output_format json",
			stateSessionID: "sess-abc",
			stateTurns:     []workflow.Turn{{Role: workflow.TurnRoleUser, Content: "prev"}},
			prompt:         "continue",
			options:        map[string]any{"output_format": "json"},
			wantArgs: []string{
				"run", "continue",
				"--format", "json",
				"-s", "sess-abc",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &workflow.ConversationState{
				SessionID: tt.stateSessionID,
				Turns:     tt.stateTurns,
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

func TestOpenCodeProvider_extractSessionID_HappyPath(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name       string
		output     string
		wantID     string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "extract session ID from step_start event",
			output:  `{"type":"step_start","sessionID":"sess-123-abc"}`,
			wantID:  "sess-123-abc",
			wantErr: false,
		},
		{
			name:    "extract session ID with additional NDJSON events",
			output:  "{\"type\":\"init\"}\n{\"type\":\"step_start\",\"sessionID\":\"my-session-id\"}\n{\"type\":\"complete\"}",
			wantID:  "my-session-id",
			wantErr: false,
		},
		{
			name:    "extract from mixed NDJSON with blank lines",
			output:  "\n{\"type\":\"init\"}\n\n{\"type\":\"step_start\",\"sessionID\":\"xyz789\"}\n",
			wantID:  "xyz789",
			wantErr: false,
		},
		{
			name:       "no step_start event",
			output:     `{"type":"init"}\n{"type":"complete"}`,
			wantErr:    true,
			wantErrMsg: "step_start",
		},
		{
			name:       "step_start without sessionID field",
			output:     `{"type":"step_start","name":"test"}`,
			wantErr:    true,
			wantErrMsg: "sessionID",
		},
		{
			name:       "step_start with empty sessionID",
			output:     `{"type":"step_start","sessionID":""}`,
			wantErr:    true,
			wantErrMsg: "sessionID",
		},
		{
			name:       "step_start with non-string sessionID",
			output:     `{"type":"step_start","sessionID":123}`,
			wantErr:    true,
			wantErrMsg: "sessionID",
		},
		{
			name:       "empty output",
			output:     "",
			wantErr:    true,
			wantErrMsg: "step_start",
		},
		{
			name:       "invalid JSON",
			output:     `{invalid json}`,
			wantErr:    true,
			wantErrMsg: "step_start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := provider.extractSessionID(tt.output)

			if tt.wantErr {
				assert.Error(t, err, "should return error")
				assert.Contains(t, err.Error(), tt.wantErrMsg, "error message should contain expected text")
			} else {
				require.NoError(t, err, "should not return error")
				assert.Equal(t, tt.wantID, id, "session ID should match")
			}
		})
	}
}

func TestOpenCodeProvider_Execute_DelegatesCorrectly(t *testing.T) {
	tests := []struct {
		name         string
		prompt       string
		options      map[string]any
		mockStdout   []byte
		mockStderr   []byte
		wantOutput   string
		wantProvider string
		wantErr      bool
	}{
		{
			name:         "simple execution",
			prompt:       "Create a function",
			options:      nil,
			mockStdout:   []byte("function created"),
			mockStderr:   []byte(""),
			wantOutput:   "function created",
			wantProvider: "opencode",
			wantErr:      false,
		},
		{
			name:         "execution with options",
			prompt:       "Build API",
			options:      map[string]any{"framework": "fastapi"},
			mockStdout:   []byte("API created"),
			mockStderr:   []byte(""),
			wantOutput:   "API created",
			wantProvider: "opencode",
			wantErr:      false,
		},
		{
			name:         "execution with stderr",
			prompt:       "test",
			options:      nil,
			mockStdout:   []byte("output"),
			mockStderr:   []byte("warning"),
			wantOutput:   "outputwarning",
			wantProvider: "opencode",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, tt.mockStderr)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err, "should not return error")
				require.NotNil(t, result, "result should not be nil")
				assert.Equal(t, tt.wantProvider, result.Provider)
				assert.Equal(t, tt.wantOutput, result.Output)
				assert.True(t, result.TokensEstimated, "TokensEstimated should be true")
			}
		})
	}
}

func TestOpenCodeProvider_ExecuteConversation_DelegatesCorrectly(t *testing.T) {
	tests := []struct {
		name         string
		sessionID    string
		prompt       string
		options      map[string]any
		mockOutput   []byte
		wantOutput   string
		wantProvider string
		wantErr      bool
	}{
		{
			name:         "new conversation",
			sessionID:    "",
			prompt:       "Hello",
			options:      nil,
			mockOutput:   []byte(`{"type":"step_start","sessionID":"new-sess"}\nHello response`),
			wantOutput:   "Hello response",
			wantProvider: "opencode",
			wantErr:      false,
		},
		{
			name:         "resume conversation with session",
			sessionID:    "existing-sess",
			prompt:       "Continue",
			options:      nil,
			mockOutput:   []byte(`{"type":"step_start","sessionID":"existing-sess"}\nContinued response`),
			wantOutput:   "Continued response",
			wantProvider: "opencode",
			wantErr:      false,
		},
		{
			name:         "conversation with options",
			sessionID:    "",
			prompt:       "test",
			options:      map[string]any{"model": "gpt-4"},
			mockOutput:   []byte(`{"type":"step_start","sessionID":"sess-opt"}\nResponse`),
			wantOutput:   "Response",
			wantProvider: "opencode",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			state := &workflow.ConversationState{
				SessionID: tt.sessionID,
				Turns:     []workflow.Turn{},
			}
			result, err := provider.ExecuteConversation(context.Background(), state, tt.prompt, tt.options, nil, nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err, "should not return error")
				require.NotNil(t, result, "result should not be nil")
				assert.Equal(t, tt.wantProvider, result.Provider)
				assert.NotEmpty(t, result.Output)
				assert.True(t, result.TokensEstimated, "TokensEstimated should be true")
			}
		})
	}
}

func TestOpenCodeProvider_Execute_EmptyPrompt_Error(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "", nil, nil, nil)

	assert.Error(t, err, "should error on empty prompt")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt", "error should mention prompt")
}

func TestOpenCodeProvider_ExecuteConversation_EmptyPrompt_Error(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	state := &workflow.ConversationState{
		SessionID: "",
		Turns:     []workflow.Turn{},
	}
	result, err := provider.ExecuteConversation(context.Background(), state, "", nil, nil, nil)

	assert.Error(t, err, "should error on empty prompt")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt", "error should mention prompt")
}

func TestOpenCodeProvider_ExecuteConversation_NilState_Error(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), nil, "test", nil, nil, nil)

	assert.Error(t, err, "should error on nil state")
	assert.Nil(t, result)
}

func TestOpenCodeProvider_validateOpenCodeOptions_InvalidTypes(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
		wantMsg string
	}{
		{
			name:    "valid options",
			options: map[string]any{"framework": "django", "verbose": true, "output_dir": "/tmp"},
			wantErr: false,
		},
		{
			name:    "output_dir is not string",
			options: map[string]any{"output_dir": 123},
			wantErr: true,
			wantMsg: "output_dir must be a string",
		},
		{
			name:    "verbose is not bool",
			options: map[string]any{"verbose": "yes"},
			wantErr: true,
			wantMsg: "verbose must be a boolean",
		},
		{
			name:    "nil options",
			options: nil,
			wantErr: false,
		},
		{
			name:    "empty options",
			options: map[string]any{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOpenCodeOptions(tt.options)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
