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
	mockExec.SetOutput([]byte(`{"type":"thread.started","thread_id":"codex-sess-123"}`+"\n"+`{"type":"message","content":"Generated code"}`), nil)
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
	mockExec.SetOutput([]byte(`{"type":"thread.started","thread_id":"codex-sess-456"}`+"\n"+`{"type":"message","content":"Continued response"}`), nil)
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

// T005: ExecuteConversation uses resume for any non-empty session ID (no prefix required)
func TestCodexProvider_ExecuteConversation_T005_NonCodexPrefixedSessionIDFallback(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"thread.started","thread_id":"new-thread-id"}`+"\n"+`{"type":"message","content":"Response"}`), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("system")
	state.SessionID = "non-codex-prefixed-id"
	result, err := provider.ExecuteConversation(context.Background(), state, "continue", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	call := calls[0]

	assert.Equal(t, "resume", call.Args[0], "any non-empty session ID triggers resume subcommand")
	assert.Equal(t, "non-codex-prefixed-id", call.Args[1], "session ID passed after resume")
	assert.Equal(t, "--json", call.Args[2])
	assert.Equal(t, "continue", call.Args[3])
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
			mockExec.SetOutput([]byte(`{"type":"thread.started","thread_id":"codex-sess"}`+"\n"+`{"type":"message","content":"Response"}`), nil)
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

// T005: Supported options (model, dangerously_skip_permissions) work with exec/resume subcommands.
// `language` is intentionally NOT supported — the codex CLI has no --language flag.
func TestCodexProvider_ExecuteConversation_T005_OptionsWithSubcommands(t *testing.T) {
	tests := []struct {
		name           string
		sessionID      string
		options        map[string]any
		expectedSubcmd string
		expectedFlag   string
	}{
		{
			name:           "first turn with model",
			sessionID:      "",
			options:        map[string]any{"model": "gpt-4o"},
			expectedSubcmd: "exec",
			expectedFlag:   "--model",
		},
		{
			name:           "resume turn with model",
			sessionID:      "codex-sess-abc",
			options:        map[string]any{"model": "gpt-4o"},
			expectedSubcmd: "resume",
			expectedFlag:   "--model",
		},
		{
			name:           "first turn with dangerously_skip_permissions",
			sessionID:      "",
			options:        map[string]any{"dangerously_skip_permissions": true},
			expectedSubcmd: "exec",
			expectedFlag:   "--dangerously-bypass-approvals-and-sandbox",
		},
		{
			name:           "resume turn with dangerously_skip_permissions",
			sessionID:      "codex-sess-xyz",
			options:        map[string]any{"dangerously_skip_permissions": true},
			expectedSubcmd: "resume",
			expectedFlag:   "--dangerously-bypass-approvals-and-sandbox",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`{"type":"thread.started","thread_id":"codex-result"}`+"\n"+`{"type":"message","content":"Response"}`), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			state.SessionID = tt.sessionID
			_, err := provider.ExecuteConversation(context.Background(), state, "prompt", tt.options, nil, nil)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			call := calls[0]

			assert.Equal(t, tt.expectedSubcmd, call.Args[0])
			assert.Contains(t, call.Args, tt.expectedFlag, "expected flag should be in args")
			assert.NotContains(t, call.Args, "--language", "--language is not supported")
			assert.NotContains(t, call.Args, "--yolo", "--yolo is not supported")
		})
	}
}

// T005: Session ID extraction works with NDJSON output format
func TestCodexProvider_ExecuteConversation_T005_SessionIDExtraction(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    []byte
		wantSessionID string
	}{
		{
			name:          "session extracted from thread.started NDJSON event",
			mockOutput:    []byte(`{"type":"thread.started","thread_id":"codex-abc-123"}` + "\n" + `{"type":"message","content":"Generated code"}`),
			wantSessionID: "codex-abc-123",
		},
		{
			name:          "malformed JSON - empty session ID",
			mockOutput:    []byte(`{"type":"thread.started","thread_id":`),
			wantSessionID: "",
		},
		{
			name:          "no thread.started event - extraction fails gracefully",
			mockOutput:    []byte(`{"type":"message","content":"Just output"}`),
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

// T005: System prompt is inlined in the first-turn message (codex CLI has no
// --system-prompt flag). On resume turns it must NOT be re-sent.
func TestCodexProvider_ExecuteConversation_T005_SystemPromptHandling(t *testing.T) {
	const systemPrompt = "You are a code generator"
	const userPrompt = "test"

	tests := []struct {
		name            string
		sessionID       string
		hasSystemPrompt bool
		expectedSubcmd  string
		shouldInline    bool
	}{
		{
			name:            "first turn with system prompt",
			sessionID:       "",
			hasSystemPrompt: true,
			expectedSubcmd:  "exec",
			shouldInline:    true,
		},
		{
			name:            "first turn without system prompt",
			sessionID:       "",
			hasSystemPrompt: false,
			expectedSubcmd:  "exec",
			shouldInline:    false,
		},
		{
			name:            "resume turn with system prompt (should ignore)",
			sessionID:       "codex-sess-abc",
			hasSystemPrompt: true,
			expectedSubcmd:  "resume",
			shouldInline:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`{"type":"thread.started","thread_id":"codex-new"}`+"\n"+`{"type":"message","content":"Response"}`), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			state.SessionID = tt.sessionID
			options := map[string]any{}
			if tt.hasSystemPrompt {
				options["system_prompt"] = systemPrompt
			}

			_, err := provider.ExecuteConversation(context.Background(), state, userPrompt, options, nil, nil)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			args := calls[0].Args

			assert.Equal(t, tt.expectedSubcmd, args[0])
			assert.NotContains(t, args, "--system-prompt", "codex has no --system-prompt flag")

			var promptArg string
			switch args[0] {
			case "exec":
				promptArg = args[2]
			case "resume":
				promptArg = args[3]
			}

			if tt.shouldInline {
				assert.Contains(t, promptArg, systemPrompt, "system prompt should be inlined in first-turn message")
				assert.Contains(t, promptArg, userPrompt, "user prompt should remain in first-turn message")
			} else {
				assert.NotContains(t, promptArg, systemPrompt, "system prompt must not be in message")
				assert.Equal(t, userPrompt, promptArg, "message should be just the user prompt")
			}
		})
	}
}
