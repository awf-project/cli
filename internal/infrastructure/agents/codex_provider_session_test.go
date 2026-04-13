package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T004: Codex session resume via NDJSON extraction (removes codex- prefix logic)
// Tests verify that:
// 1. Any non-empty SessionID triggers resume (no prefix checking)
// 2. thread_id is extracted from thread.started event in NDJSON output
// 3. No "Session: " text pattern extraction is used

func TestCodexProvider_ExecuteConversation_AnySessionIDTriggersResume(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		wantResume bool
	}{
		{
			name:       "non-empty session ID triggers resume (no prefix required)",
			sessionID:  "any-session-id-without-prefix",
			wantResume: true,
		},
		{
			name:       "old codex- prefixed ID still works (backward compat in resume logic)",
			sessionID:  "codex-session-abc",
			wantResume: true,
		},
		{
			name:       "UUID-style session ID triggers resume",
			sessionID:  "019bd456-d3d4-70c3-90de-51d31a6c8571",
			wantResume: true,
		},
		{
			name:       "empty session ID uses exec (first turn)",
			sessionID:  "",
			wantResume: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			// Mock returns proper NDJSON with thread.started event
			mockExec.SetOutput([]byte(`{"type":"thread.started","thread_id":"new-session-456"}`+"\n"+`{"type":"message","content":"Response"}`), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			state.SessionID = tt.sessionID

			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			hasResume := false
			for _, arg := range calls[0].Args {
				if arg == "resume" {
					hasResume = true
					break
				}
			}

			assert.Equal(t, tt.wantResume, hasResume, "resume subcommand presence should match session ID emptiness")
			if tt.wantResume && hasResume {
				// Verify the session ID is passed as argument
				resumeIndex := -1
				for i, arg := range calls[0].Args {
					if arg == "resume" {
						resumeIndex = i
						break
					}
				}
				require.Greater(t, resumeIndex, -1)
				require.Less(t, resumeIndex+1, len(calls[0].Args))
				assert.Equal(t, tt.sessionID, calls[0].Args[resumeIndex+1], "session ID should be passed after resume")
			}
		})
	}
}

func TestCodexProvider_ExecuteConversation_ExtractsThreadIDFromNDJSON(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		wantSessionID string
		wantError     bool
	}{
		{
			name:          "extracts thread_id from thread.started NDJSON event",
			mockOutput:    `{"type":"thread.started","thread_id":"019bd456-d3d4-70c3-90de-51d31a6c8571"}`,
			wantSessionID: "019bd456-d3d4-70c3-90de-51d31a6c8571",
			wantError:     false,
		},
		{
			name:          "ignores old text-based Session: pattern (dead code path)",
			mockOutput:    "Session: old-session-123\n" + `{"type":"thread.started","thread_id":"real-thread-456"}`,
			wantSessionID: "real-thread-456",
			wantError:     false,
		},
		{
			name:          "handles NDJSON with multiple events (thread.started first)",
			mockOutput:    `{"type":"thread.started","thread_id":"abc-123"}` + "\n" + `{"type":"message","content":"Generated code"}` + "\n" + `{"type":"done"}`,
			wantSessionID: "abc-123",
			wantError:     false,
		},
		{
			name:          "no thread.started event yields empty SessionID gracefully",
			mockOutput:    `{"type":"message","content":"Response"}` + "\n" + `{"type":"done"}`,
			wantSessionID: "",
			wantError:     false,
		},
		{
			name:          "plain text without JSON yields empty SessionID gracefully",
			mockOutput:    "Just plain text output",
			wantSessionID: "",
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(tt.mockOutput), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.State)

			if tt.wantError {
				// If extraction fails, SessionID should be cleared
				assert.Empty(t, result.State.SessionID)
			} else {
				assert.Equal(t, tt.wantSessionID, result.State.SessionID, "session ID should match extracted thread_id from NDJSON")
			}
		})
	}
}

func TestCodexProvider_ExecuteConversation_FallbackOnExtractionFailure(t *testing.T) {
	tests := []struct {
		name           string
		priorSessionID string
		mockOutput     string
		description    string
	}{
		{
			name:           "clears session ID when thread.started not found",
			priorSessionID: "previous-session-123",
			mockOutput:     `{"type":"message","content":"Response"}`,
			description:    "prior session ID should be cleared when extraction fails",
		},
		{
			name:           "clears session ID when output is empty",
			priorSessionID: "previous-session-456",
			mockOutput:     "",
			description:    "empty output should clear session ID",
		},
		{
			name:           "clears session ID when JSON is malformed",
			priorSessionID: "previous-session-789",
			mockOutput:     `{"type":"thread.started","thread_id":`,
			description:    "malformed JSON should gracefully clear session ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(tt.mockOutput), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			state.SessionID = tt.priorSessionID

			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil, nil, nil)

			require.NoError(t, err, "extraction failure should not cause error (graceful fallback)")
			require.NotNil(t, result)
			assert.Empty(t, result.State.SessionID, tt.description)
			assert.NotEmpty(t, result.Output, "output should still be available")
		})
	}
}

func TestCodexProvider_ExecuteConversation_SystemPromptFirstTurnOnly(t *testing.T) {
	// Codex CLI has no --system-prompt flag; the system prompt is inlined
	// into the first turn's message. On resume turns it must NOT be re-sent.
	const systemPrompt = "You are a helpful code generator"
	const userPrompt = "test prompt"

	tests := []struct {
		name         string
		sessionID    string
		shouldInline bool
	}{
		{
			name:         "system_prompt inlined on first turn (no SessionID)",
			sessionID:    "",
			shouldInline: true,
		},
		{
			name:         "system_prompt NOT inlined on resume turn (with SessionID)",
			sessionID:    "existing-session-123",
			shouldInline: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`{"type":"thread.started","thread_id":"new-session"}`), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			state.SessionID = tt.sessionID
			options := map[string]any{"system_prompt": systemPrompt}

			result, err := provider.ExecuteConversation(context.Background(), state, userPrompt, options, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			args := calls[0].Args

			// The --system-prompt flag must never be passed.
			assert.NotContains(t, args, "--system-prompt", "codex has no --system-prompt flag")

			// The prompt arg sits immediately after the subcommand positional args.
			// exec: args = ["exec", "--json", <prompt>, ...]
			// resume: args = ["resume", <thread_id>, "--json", <prompt>, ...]
			var promptArg string
			switch args[0] {
			case "exec":
				promptArg = args[2]
			case "resume":
				promptArg = args[3]
			default:
				t.Fatalf("unexpected subcommand %q", args[0])
			}

			if tt.shouldInline {
				assert.Contains(t, promptArg, systemPrompt, "system prompt should be inlined in first-turn message")
				assert.Contains(t, promptArg, userPrompt, "user prompt should remain in first-turn message")
			} else {
				assert.NotContains(t, promptArg, systemPrompt, "system prompt must not be re-sent on resume turn")
				assert.Equal(t, userPrompt, promptArg, "resume turn should send only the user prompt")
			}
		})
	}
}

func TestCodexProvider_ExecuteConversation_ConversationStateUpdated(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	// NDJSON output with thread.started event
	mockExec.SetOutput([]byte(`{"type":"thread.started","thread_id":"session-abc-123"}`+"\n"+`{"type":"message","content":"Generated response"}`), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("you are a code generator")
	userPrompt := "write a function"

	result, err := provider.ExecuteConversation(context.Background(), state, userPrompt, nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.State)

	// Verify state is updated with session ID
	assert.Equal(t, "session-abc-123", result.State.SessionID, "session ID should be extracted and stored")

	// Verify conversation turns are recorded (system + user + assistant)
	assert.Len(t, result.State.Turns, 3, "should have system, user, and assistant turns")
	assert.Equal(t, result.State.Turns[0].Role, workflow.TurnRoleSystem)
	assert.Equal(t, result.State.Turns[0].Content, "you are a code generator")
	assert.Equal(t, result.State.Turns[1].Role, workflow.TurnRoleUser)
	assert.Equal(t, result.State.Turns[1].Content, userPrompt)
	assert.Equal(t, result.State.Turns[2].Role, workflow.TurnRoleAssistant)
}

func TestCodexProvider_ExecuteConversation_OptionsValidation(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		check   func(t *testing.T, args []string)
	}{
		{
			name:    "model option passed to CLI",
			options: map[string]any{"model": "codex-002"},
			check: func(t *testing.T, args []string) {
				assert.Contains(t, args, "--model")
				idx := indexOf(args, "--model")
				require.Less(t, idx+1, len(args))
				assert.Equal(t, "codex-002", args[idx+1])
			},
		},
		{
			name:    "unknown language option silently ignored",
			options: map[string]any{"language": "python"},
			check: func(t *testing.T, args []string) {
				assert.NotContains(t, args, "--language")
			},
		},
		{
			name:    "dangerously_skip_permissions maps to bypass flag",
			options: map[string]any{"dangerously_skip_permissions": true},
			check: func(t *testing.T, args []string) {
				assert.Contains(t, args, "--dangerously-bypass-approvals-and-sandbox")
				assert.NotContains(t, args, "--yolo")
			},
		},
		{
			name:    "combined supported options present",
			options: map[string]any{"model": "codex-002", "language": "go", "dangerously_skip_permissions": true},
			check: func(t *testing.T, args []string) {
				assert.Contains(t, args, "--model")
				assert.NotContains(t, args, "--language")
				assert.Contains(t, args, "--dangerously-bypass-approvals-and-sandbox")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`{"type":"thread.started","thread_id":"test-session"}`), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test", tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			tt.check(t, calls[0].Args)
		})
	}
}

// Helper function to find index of string in slice
func indexOf(slice []string, target string) int {
	for i, v := range slice {
		if v == target {
			return i
		}
	}
	return -1
}
