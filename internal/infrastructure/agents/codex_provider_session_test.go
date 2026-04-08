package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T007: Codex session resume implementation
// Tests for extractSessionID() and session resume logic in ExecuteConversation

func TestCodexProvider_extractSessionID(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		wantSessionID string
	}{
		{
			name:          "session ID from 'Session: <id>' format",
			output:        "Session: codex-session-abc123\nCode generation result",
			wantSessionID: "codex-session-abc123",
		},
		{
			name:          "session ID with complex alphanumeric",
			output:        "Session: sess_f47ac10b58cc4372a5670e02b2c3d479\n...",
			wantSessionID: "sess_f47ac10b58cc4372a5670e02b2c3d479",
		},
		{
			name:          "session ID as numeric",
			output:        "Session: 9876543210\nResponse content",
			wantSessionID: "9876543210",
		},
		{
			name:          "no Session line returns empty",
			output:        "No session info\nJust code output",
			wantSessionID: "",
		},
		{
			name:          "malformed Session line returns empty",
			output:        "Session:\nNo ID provided",
			wantSessionID: "",
		},
		{
			name:          "empty output returns empty",
			output:        "",
			wantSessionID: "",
		},
		{
			name:          "Session line with trailing whitespace",
			output:        "Session: codex-sess-123  \nMore content",
			wantSessionID: "codex-sess-123",
		},
		{
			name:          "multiple Session lines uses first",
			output:        "Session: first-id\nSession: second-id\nCode",
			wantSessionID: "first-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractSessionIDFromLines(tt.output)
			assert.Equal(t, tt.wantSessionID, got)
			if tt.wantSessionID == "" {
				assert.Error(t, err, "extraction failure should return error")
			} else {
				assert.NoError(t, err, "successful extraction should not error")
			}
		})
	}
}

func TestCodexProvider_ExecuteConversation_ResumeFlag(t *testing.T) {
	tests := []struct {
		name            string
		sessionID       string
		wantResumeFlag  bool
		wantExecCommand bool
	}{
		{
			name:            "turn 2+ with session ID uses resume subcommand",
			sessionID:       "codex-session-abc",
			wantResumeFlag:  true,
			wantExecCommand: false,
		},
		{
			name:            "turn 1 without session ID uses exec subcommand",
			sessionID:       "",
			wantResumeFlag:  false,
			wantExecCommand: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("Session: codex-new-session\nGenerated code"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			state.SessionID = tt.sessionID

			result, err := provider.ExecuteConversation(context.Background(), state, "write a function", nil, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			hasResumeSubcommand := false
			hasExecSubcommand := false
			for i, arg := range calls[0].Args {
				if arg == "resume" && i+1 < len(calls[0].Args) {
					hasResumeSubcommand = true
					if tt.wantResumeFlag {
						assert.Equal(t, tt.sessionID, calls[0].Args[i+1])
					}
				}
				if arg == "exec" {
					hasExecSubcommand = true
				}
			}
			assert.Equal(t, tt.wantResumeFlag, hasResumeSubcommand)
			assert.Equal(t, tt.wantExecCommand, hasExecSubcommand)
			// Both paths should include --json
			assert.Contains(t, calls[0].Args, "--json")
		})
	}
}

func TestCodexProvider_ExecuteConversation_SessionIDExtracted(t *testing.T) {
	tests := []struct {
		name           string
		mockOutput     []byte
		wantSessionID  string
		wantOutputText string
	}{
		{
			name:           "session ID extracted from output",
			mockOutput:     []byte("Session: codex-extracted-123\nfunction doSomething() { return 42; }"),
			wantSessionID:  "codex-extracted-123",
			wantOutputText: "Session: codex-extracted-123\nfunction doSomething() { return 42; }",
		},
		{
			name:           "no session line yields empty SessionID",
			mockOutput:     []byte("Code output without session info"),
			wantSessionID:  "",
			wantOutputText: "Code output without session info",
		},
		{
			name:           "empty session in line yields empty",
			mockOutput:     []byte("Session: \nsome code"),
			wantSessionID:  "",
			wantOutputText: "Session: \nsome code",
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
			require.NotNil(t, result)
			assert.Equal(t, tt.wantSessionID, result.State.SessionID)
			assert.Equal(t, tt.wantOutputText, result.Output)
		})
	}
}

func TestCodexProvider_ExecuteConversation_SystemPromptOnFirstTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("Session: codex-1\nGenerated response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("")
	options := map[string]any{"system_prompt": "You are a code generator"}

	_, err := provider.ExecuteConversation(context.Background(), state, "Generate a hello world", options, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Args, "exec")
	assert.Contains(t, calls[0].Args, "--json")
	assert.Contains(t, calls[0].Args, "--system-prompt")
	assert.Contains(t, calls[0].Args, "You are a code generator")
}

func TestCodexProvider_ExecuteConversation_NoSystemPromptOnResumeTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("Session: codex-resume\nContinued response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.SessionID = "existing-session-id"
	options := map[string]any{"system_prompt": "You are a code generator"}

	_, err := provider.ExecuteConversation(context.Background(), state, "Continue", options, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.NotContains(t, calls[0].Args, "--system-prompt")
}

func TestCodexProvider_ExecuteConversation_GracefulFallback(t *testing.T) {
	tests := []struct {
		name       string
		mockOutput []byte
	}{
		{
			name:       "malformed output no extraction error",
			mockOutput: []byte("Response without session identifier line"),
		},
		{
			name:       "empty output no extraction error",
			mockOutput: []byte(""),
		},
		{
			name:       "invalid session format no error",
			mockOutput: []byte("Session:\nNo ID following colon"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil, nil, nil)

			require.NoError(t, err, "extraction failure must not cause error (graceful fallback)")
			require.NotNil(t, result)
			assert.Empty(t, result.State.SessionID, "SessionID should be empty when extraction fails")
			assert.NotEmpty(t, result.Output, "output should still be populated")
		})
	}
}

func TestCodexProvider_ExecuteConversation_ResumeFallback_NonPrefixedSessionID(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("No session info in output"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("system")
	// "previous-session" lacks the "codex-" prefix, so the prefix guard fires:
	// isResume = false and the resume subcommand is never added to args.
	state.SessionID = "previous-session"

	result, err := provider.ExecuteConversation(context.Background(), state, "prompt", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)

	hasResumeSubcommand := false
	hasExecSubcommand := false
	for _, arg := range calls[0].Args {
		if arg == "resume" {
			hasResumeSubcommand = true
		}
		if arg == "exec" {
			hasExecSubcommand = true
		}
	}
	assert.False(t, hasResumeSubcommand, "resume subcommand must be absent when session ID lacks codex- prefix")
	assert.True(t, hasExecSubcommand, "exec subcommand must be used when session ID lacks prefix")
	assert.Contains(t, calls[0].Args, "--json", "--json flag must be present in exec subcommand")
}

func TestCodexProvider_ExecuteConversation_ResumeFallback_ExtractionFailure(t *testing.T) {
	// "codex-valid-prefix" passes the prefix guard so the resume subcommand IS
	// attempted, but the CLI output contains no "Session: <id>" line. The provider
	// must clear SessionID rather than propagating the stale value.
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("Generated code without any session line"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("system")
	state.SessionID = "codex-valid-prefix"

	result, err := provider.ExecuteConversation(context.Background(), state, "prompt", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)

	// Resume subcommand was attempted because of the valid prefix.
	hasResumeSubcommand := false
	for _, arg := range calls[0].Args {
		if arg == "resume" {
			hasResumeSubcommand = true
			break
		}
	}
	assert.True(t, hasResumeSubcommand, "resume subcommand must be present for codex- prefixed session ID")

	// After the turn, SessionID must be cleared because extraction failed.
	assert.Empty(t, result.State.SessionID, "SessionID must be cleared when output contains no Session line")
}
