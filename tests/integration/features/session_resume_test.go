//go:build integration

// Feature: F073

package features_test

import (
	"context"
	"io"
	"slices"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeSessionResume_MultiTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := agents.NewClaudeProviderWithOptions(agents.WithClaudeExecutor(mockExec))

	mockExec.SetOutput(
		[]byte(`{"session_id":"sess_abc123","result":"Code looks good overall.","cost_usd":0.01}`),
		nil,
	)

	state := workflow.NewConversationState("You are a code reviewer")
	options := map[string]any{"system_prompt": "You are a code reviewer"}

	result, err := provider.ExecuteConversation(context.Background(), state, "Review this code", options, io.Discard, io.Discard)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "sess_abc123", result.State.SessionID)
	assert.Equal(t, "Code looks good overall.", result.Output)

	turn1Calls := mockExec.GetCalls()
	require.Len(t, turn1Calls, 1)
	assert.Contains(t, turn1Calls[0].Args, "--system-prompt")
	assert.NotContains(t, turn1Calls[0].Args, "-r")

	// Turn 2: state carries SessionID from turn 1
	mockExec.Clear()
	mockExec.SetOutput(
		[]byte(`{"session_id":"sess_abc123","result":"Issue #1 is the null check on line 42.","cost_usd":0.02}`),
		nil,
	)

	result2, err := provider.ExecuteConversation(context.Background(), result.State, "What was issue #1?", options, io.Discard, io.Discard)
	require.NoError(t, err)
	require.NotNil(t, result2)

	assert.Equal(t, "sess_abc123", result2.State.SessionID)
	assert.Equal(t, "Issue #1 is the null check on line 42.", result2.Output)

	turn2Calls := mockExec.GetCalls()
	require.Len(t, turn2Calls, 1)

	resumeIdx := slices.Index(turn2Calls[0].Args, "-r")
	require.NotEqual(t, -1, resumeIdx, "turn 2 must have -r flag")
	require.Less(t, resumeIdx+1, len(turn2Calls[0].Args))
	assert.Equal(t, "sess_abc123", turn2Calls[0].Args[resumeIdx+1])

	assert.NotContains(t, turn2Calls[0].Args, "--system-prompt")
}

func TestAllProviders_SessionResumeFlags(t *testing.T) {
	type providerSetup struct {
		name        string
		newProvider func(*mocks.MockCLIExecutor) ports.AgentProvider
		turn1Output []byte
		turn2Output []byte
		wantID      string
		checkResume func(t *testing.T, args []string)
	}

	tests := []providerSetup{
		{
			name: "codex_resume_subcommand",
			newProvider: func(m *mocks.MockCLIExecutor) ports.AgentProvider {
				return agents.NewCodexProviderWithOptions(agents.WithCodexExecutor(m))
			},
			turn1Output: []byte("Session: codex-sess-789\nCode review complete."),
			turn2Output: []byte("Session: codex-sess-789\nFixed the issue."),
			wantID:      "codex-sess-789",
			checkResume: func(t *testing.T, args []string) {
				assert.Contains(t, args, "resume")
				assert.Contains(t, args, "codex-sess-789")
			},
		},
		{
			name: "gemini_resume_flag",
			newProvider: func(m *mocks.MockCLIExecutor) ports.AgentProvider {
				return agents.NewGeminiProviderWithOptions(agents.WithGeminiExecutor(m))
			},
			turn1Output: []byte("Session: gem-sess-456\nAnalysis complete."),
			turn2Output: []byte("Session: gem-sess-456\nUpdated analysis."),
			wantID:      "gem-sess-456",
			checkResume: func(t *testing.T, args []string) {
				idx := slices.Index(args, "--resume")
				require.NotEqual(t, -1, idx)
				require.Less(t, idx+1, len(args))
				assert.Equal(t, "gem-sess-456", args[idx+1])
			},
		},
		{
			name: "opencode_session_flag",
			newProvider: func(m *mocks.MockCLIExecutor) ports.AgentProvider {
				return agents.NewOpenCodeProviderWithOptions(agents.WithOpenCodeExecutor(m))
			},
			turn1Output: []byte("Session: oc-sess-321\nGenerated code."),
			turn2Output: []byte("Session: oc-sess-321\nRefactored."),
			wantID:      "oc-sess-321",
			checkResume: func(t *testing.T, args []string) {
				idx := slices.Index(args, "-s")
				require.NotEqual(t, -1, idx)
				require.Less(t, idx+1, len(args))
				assert.Equal(t, "oc-sess-321", args[idx+1])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			provider := tt.newProvider(mockExec)

			// Turn 1: extract session ID
			mockExec.SetOutput(tt.turn1Output, nil)
			state := workflow.NewConversationState("")
			result, err := provider.ExecuteConversation(context.Background(), state, "Review code", nil, io.Discard, io.Discard)
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, result.State.SessionID)

			// Turn 2: verify resume args
			mockExec.Clear()
			mockExec.SetOutput(tt.turn2Output, nil)
			_, err = provider.ExecuteConversation(context.Background(), result.State, "Fix issue", nil, io.Discard, io.Discard)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			tt.checkResume(t, calls[0].Args)
		})
	}
}

func TestSessionResume_GracefulFallback(t *testing.T) {
	tests := []struct {
		name        string
		output      []byte
		newProvider func(*mocks.MockCLIExecutor) ports.AgentProvider
	}{
		{
			name:   "claude_non_json_output",
			output: []byte("Plain text response without JSON"),
			newProvider: func(m *mocks.MockCLIExecutor) ports.AgentProvider {
				return agents.NewClaudeProviderWithOptions(agents.WithClaudeExecutor(m))
			},
		},
		{
			name:   "codex_no_session_line",
			output: []byte("Just a plain code review.\nNo session info here."),
			newProvider: func(m *mocks.MockCLIExecutor) ports.AgentProvider {
				return agents.NewCodexProviderWithOptions(agents.WithCodexExecutor(m))
			},
		},
		{
			name:   "gemini_no_session_line",
			output: []byte("Analysis complete. No session metadata."),
			newProvider: func(m *mocks.MockCLIExecutor) ports.AgentProvider {
				return agents.NewGeminiProviderWithOptions(agents.WithGeminiExecutor(m))
			},
		},
		{
			name:   "opencode_no_session_line",
			output: []byte("Generated code. Done."),
			newProvider: func(m *mocks.MockCLIExecutor) ports.AgentProvider {
				return agents.NewOpenCodeProviderWithOptions(agents.WithOpenCodeExecutor(m))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.output, nil)
			provider := tt.newProvider(mockExec)

			state := workflow.NewConversationState("")
			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil, io.Discard, io.Discard)

			require.NoError(t, err, "extraction failure must not cause error")
			require.NotNil(t, result)
			assert.Empty(t, result.State.SessionID)
			assert.NotEmpty(t, result.Output)
		})
	}
}

func TestSessionID_PersistsThroughThreeTurns(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := agents.NewClaudeProviderWithOptions(agents.WithClaudeExecutor(mockExec))

	state := workflow.NewConversationState("")

	for i, prompt := range []string{"Turn 1", "Turn 2", "Turn 3"} {
		mockExec.Clear()
		mockExec.SetOutput(
			[]byte(`{"session_id":"sess_persistent","result":"response"}`),
			nil,
		)
		result, err := provider.ExecuteConversation(context.Background(), state, prompt, nil, io.Discard, io.Discard)
		require.NoError(t, err)

		assert.Equal(t, "sess_persistent", result.State.SessionID)
		state = result.State

		if i > 0 {
			calls := mockExec.GetCalls()
			assert.Contains(t, calls[0].Args, "-r",
				"turn %d should have resume flag", i+1)
		}
	}

	// 3*(user+assistant) = 6 turns (empty system prompt doesn't add a turn)
	assert.Equal(t, 6, len(state.Turns))
}
