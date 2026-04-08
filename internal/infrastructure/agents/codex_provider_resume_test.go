package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F078 Tests: Fix CLI Provider Invocations - Codex provider subcommand and output format corrections

// T005: ExecuteConversation first turn uses `exec --json` subcommand
func TestCodexProvider_ExecuteConversation_T005_FirstTurnExecSubcommand(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("Session: codex-sess-123\nGenerated code"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("system")
	result, err := provider.ExecuteConversation(context.Background(), state, "write hello function", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	call := calls[0]

	assert.Equal(t, "codex", call.Name)
	assert.GreaterOrEqual(t, len(call.Args), 3)
	assert.Equal(t, "exec", call.Args[0], "first turn should use exec subcommand")
	assert.Equal(t, "--json", call.Args[1], "exec should have --json flag")
	assert.Equal(t, "write hello function", call.Args[2], "prompt should follow --json")
}

// T005: ExecuteConversation resume turn uses `resume <sessionID> --json` subcommand
func TestCodexProvider_ExecuteConversation_T005_ResumeTurnResumeSubcommand(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("Session: codex-sess-456\nContinued response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("system")
	state.SessionID = "codex-sess-123"
	result, err := provider.ExecuteConversation(context.Background(), state, "add error handling", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	call := calls[0]

	assert.Equal(t, "codex", call.Name)
	assert.GreaterOrEqual(t, len(call.Args), 4)
	assert.Equal(t, "resume", call.Args[0], "resume turn should use resume subcommand")
	assert.Equal(t, "codex-sess-123", call.Args[1], "session ID should be args[1]")
	assert.Equal(t, "--json", call.Args[2], "resume should have --json flag after session ID")
	assert.Equal(t, "add error handling", call.Args[3], "prompt should follow --json")
}

// T005: ExecuteConversation correctly handles non-codex-prefixed session IDs (fallback to exec)
func TestCodexProvider_ExecuteConversation_T005_NonCodexPrefixedSessionIDFallback(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("Session: codex-new\nResponse"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("system")
	state.SessionID = "non-codex-prefixed-id"
	result, err := provider.ExecuteConversation(context.Background(), state, "continue", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	call := calls[0]

	assert.Equal(t, "exec", call.Args[0], "non-codex-prefixed ID should fall back to exec")
	assert.Equal(t, "--json", call.Args[1])
	assert.Equal(t, "continue", call.Args[2])
}

// T005: Quiet flag is NOT passed (replaced by --json on exec/resume)
func TestCodexProvider_ExecuteConversation_T005_QuietFlagRemoved(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name:    "quiet true - should omit --quiet",
			options: map[string]any{"quiet": true},
		},
		{
			name:    "quiet false - should omit --quiet",
			options: map[string]any{"quiet": false},
		},
		{
			name:    "no quiet option - should omit --quiet",
			options: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("Session: codex-sess\nResponse"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			_, err := provider.ExecuteConversation(context.Background(), state, "test", tt.options, nil, nil)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			call := calls[0]

			// Verify --quiet is NOT present
			for _, arg := range call.Args {
				assert.NotEqual(t, "--quiet", arg, "quiet flag should be removed")
			}
			// Verify --json IS present
			assert.Contains(t, call.Args, "--json")
		})
	}
}

// T005: Execute uses new `exec --json` subcommand (not --prompt)
func TestCodexProvider_Execute_T005_ExecSubcommand(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		options map[string]any
	}{
		{
			name:    "minimal execute",
			prompt:  "generate code",
			options: nil,
		},
		{
			name:    "with language",
			prompt:  "write function",
			options: map[string]any{"language": "go"},
		},
		{
			name:    "with model",
			prompt:  "test",
			options: map[string]any{"model": "gpt-4"},
		},
		{
			name:    "with dangerously_skip_permissions",
			prompt:  "risky",
			options: map[string]any{"dangerously_skip_permissions": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("result"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			call := calls[0]

			assert.Equal(t, "codex", call.Name)
			assert.GreaterOrEqual(t, len(call.Args), 3)
			assert.Equal(t, "exec", call.Args[0], "Execute should use exec subcommand")
			assert.Equal(t, "--json", call.Args[1], "exec should have --json flag")
			assert.Equal(t, tt.prompt, call.Args[2], "prompt should follow --json")

			// Verify no --prompt flag
			for _, arg := range call.Args {
				assert.NotEqual(t, "--prompt", arg, "--prompt flag should not be used")
			}
		})
	}
}

// T005: Options (model, language, dangerously_skip_permissions) work with exec/resume subcommands
func TestCodexProvider_ExecuteConversation_T005_OptionsWithSubcommands(t *testing.T) {
	tests := []struct {
		name             string
		sessionID        string
		options          map[string]any
		expectedSubcmd   string
		shouldHaveOption bool
	}{
		{
			name:             "first turn with model",
			sessionID:        "",
			options:          map[string]any{"model": "gpt-4o"},
			expectedSubcmd:   "exec",
			shouldHaveOption: true,
		},
		{
			name:             "resume turn with language",
			sessionID:        "codex-sess-abc",
			options:          map[string]any{"language": "python"},
			expectedSubcmd:   "resume",
			shouldHaveOption: true,
		},
		{
			name:             "first turn with yolo",
			sessionID:        "",
			options:          map[string]any{"dangerously_skip_permissions": true},
			expectedSubcmd:   "exec",
			shouldHaveOption: true,
		},
		{
			name:             "resume turn with yolo",
			sessionID:        "codex-sess-xyz",
			options:          map[string]any{"dangerously_skip_permissions": true},
			expectedSubcmd:   "resume",
			shouldHaveOption: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("Session: codex-result\nResponse"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			state.SessionID = tt.sessionID
			_, err := provider.ExecuteConversation(context.Background(), state, "prompt", tt.options, nil, nil)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			call := calls[0]

			assert.Equal(t, tt.expectedSubcmd, call.Args[0])
			// Verify options are passed when shouldHaveOption is true
			if tt.shouldHaveOption {
				optionPresent := false
				for _, arg := range call.Args {
					if arg == "--model" || arg == "--language" || arg == "--yolo" {
						optionPresent = true
						break
					}
				}
				assert.True(t, optionPresent, "option should be in args")
			}
		})
	}
}

// T005: Session ID extraction works with new output format
func TestCodexProvider_ExecuteConversation_T005_SessionIDExtraction(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    []byte
		wantSessionID string
	}{
		{
			name:          "session extracted from Session: line",
			mockOutput:    []byte("Session: codex-abc-123\nGenerated code"),
			wantSessionID: "codex-abc-123",
		},
		{
			name:          "malformed output - empty session ID",
			mockOutput:    []byte("Session:\nNo ID"),
			wantSessionID: "",
		},
		{
			name:          "no session line - extraction fails gracefully",
			mockOutput:    []byte("Just output\nNo session info"),
			wantSessionID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

			require.NoError(t, err)
			assert.Equal(t, tt.wantSessionID, result.State.SessionID)
		})
	}
}

// T005: System prompt only passed on first turn (exec), not on resume
func TestCodexProvider_ExecuteConversation_T005_SystemPromptHandling(t *testing.T) {
	tests := []struct {
		name                 string
		sessionID            string
		hasSystemPrompt      bool
		expectedSubcmd       string
		shouldHavePromptFlag bool
	}{
		{
			name:                 "first turn with system prompt",
			sessionID:            "",
			hasSystemPrompt:      true,
			expectedSubcmd:       "exec",
			shouldHavePromptFlag: true,
		},
		{
			name:                 "first turn without system prompt",
			sessionID:            "",
			hasSystemPrompt:      false,
			expectedSubcmd:       "exec",
			shouldHavePromptFlag: false,
		},
		{
			name:                 "resume turn with system prompt (should ignore)",
			sessionID:            "codex-sess-abc",
			hasSystemPrompt:      true,
			expectedSubcmd:       "resume",
			shouldHavePromptFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("Session: codex-new\nResponse"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			state.SessionID = tt.sessionID
			options := map[string]any{}
			if tt.hasSystemPrompt {
				options["system_prompt"] = "You are a code generator"
			}

			_, err := provider.ExecuteConversation(context.Background(), state, "test", options, nil, nil)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			call := calls[0]

			assert.Equal(t, tt.expectedSubcmd, call.Args[0])

			// Check for --system-prompt flag
			hasPromptFlag := false
			for _, arg := range call.Args {
				if arg == "--system-prompt" {
					hasPromptFlag = true
					break
				}
			}
			assert.Equal(t, tt.shouldHavePromptFlag, hasPromptFlag)
		})
	}
}
