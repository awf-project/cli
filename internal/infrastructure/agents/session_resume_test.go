package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T014: Cross-step session resume (FR-009)
// Tests multi-turn conversation resume across workflow steps for Gemini, Codex, and OpenCode.
// Verifies: SessionID extraction, persistence, and resume flag construction.

func TestSessionResume_GeminiExtractsSessionID(t *testing.T) {
	tests := []struct {
		name           string
		mockOutput     []byte
		wantSessionID  string
		wantResumeFlag bool
	}{
		{
			name: "turn 1: extract session_id from type:init JSON",
			mockOutput: []byte(`{"type":"init","timestamp":"2026-04-07T21:55:36.994Z","session_id":"031da63a-73be-42f5-ae0d-890aae0b6323","model":"auto-gemini-3"}` + "\n" +
				`{"type":"result","result":"Hello from Gemini"}`),
			wantSessionID:  "031da63a-73be-42f5-ae0d-890aae0b6323",
			wantResumeFlag: false, // turn 1, no prior session
		},
		{
			name: "turn 2: use --resume flag with extracted UUID",
			mockOutput: []byte(`{"type":"init","timestamp":"2026-04-07T21:55:37.000Z","session_id":"031da63a-73be-42f5-ae0d-890aae0b6323","model":"auto-gemini-3"}` + "\n" +
				`{"type":"result","result":"Context resumed correctly"}`),
			wantSessionID:  "031da63a-73be-42f5-ae0d-890aae0b6323",
			wantResumeFlag: true, // turn 2 with prior session
		},
		{
			name:           "missing session_id field: SessionID stays empty",
			mockOutput:     []byte(`{"type":"result","result":"Response without session"}`),
			wantSessionID:  "",
			wantResumeFlag: false,
		},
		{
			name:           "malformed JSON: graceful fallback to empty SessionID",
			mockOutput:     []byte(`not valid json`),
			wantSessionID:  "",
			wantResumeFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := workflow.NewConversationState("system prompt")
			if tt.wantResumeFlag {
				// Simulate prior session from turn 1
				state.SessionID = "031da63a-73be-42f5-ae0d-890aae0b6323"
			}

			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Verify SessionID is persisted
			assert.Equal(t, tt.wantSessionID, result.State.SessionID)

			// Verify resume flag construction
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			hasResumeFlag := false
			for i, arg := range calls[0].Args {
				if arg == "--resume" && i+1 < len(calls[0].Args) {
					hasResumeFlag = true
					if tt.wantResumeFlag {
						assert.Equal(t, "031da63a-73be-42f5-ae0d-890aae0b6323", calls[0].Args[i+1])
					}
				}
			}
			assert.Equal(t, tt.wantResumeFlag, hasResumeFlag)

			// Verify stream-json is forced in ExecuteConversation
			assert.Contains(t, calls[0].Args, "--output-format")
			assert.Contains(t, calls[0].Args, "stream-json")
		})
	}
}

func TestSessionResume_CodexExtractsThreadID(t *testing.T) {
	tests := []struct {
		name           string
		mockOutput     []byte
		wantSessionID  string
		wantResumeCmd  bool
		wantNoPrefixID bool
	}{
		{
			name: "turn 1: extract thread_id from type:thread.started JSON",
			mockOutput: []byte(`{"type":"thread.started","thread_id":"019bd456-d3d4-70c3-90de-51d31a6c8571"}` + "\n" +
				`{"type":"message","content":"Code generated"}`),
			wantSessionID: "019bd456-d3d4-70c3-90de-51d31a6c8571",
			wantResumeCmd: false,
		},
		{
			name: "turn 2: use resume <thread_id> subcommand",
			mockOutput: []byte(`{"type":"thread.started","thread_id":"019bd456-d3d4-70c3-90de-51d31a6c8571"}` + "\n" +
				`{"type":"message","content":"Context continued"}`),
			wantSessionID: "019bd456-d3d4-70c3-90de-51d31a6c8571",
			wantResumeCmd: true,
		},
		{
			name:          "missing thread_id: graceful fallback",
			mockOutput:    []byte(`{"type":"message","content":"Response"}`),
			wantSessionID: "",
			wantResumeCmd: false,
		},
		{
			name:           "malformed JSON: SessionID remains empty",
			mockOutput:     []byte(`invalid json`),
			wantSessionID:  "",
			wantResumeCmd:  false,
			wantNoPrefixID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("")
			if tt.wantResumeCmd {
				// Simulate prior thread from turn 1
				state.SessionID = "019bd456-d3d4-70c3-90de-51d31a6c8571"
			}

			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Verify ThreadID is persisted
			assert.Equal(t, tt.wantSessionID, result.State.SessionID)

			// Verify SessionID never contains fabricated "codex-" prefix (applies to every case, including missing thread_id).
			assert.NotContains(t, result.State.SessionID, "codex-",
				"SessionID must not contain fabricated 'codex-' prefix")

			// Verify resume subcommand construction
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			hasResumeCmd := false
			for i, arg := range calls[0].Args {
				if arg == "resume" && tt.wantResumeCmd && i+1 < len(calls[0].Args) {
					if calls[0].Args[i+1] == tt.wantSessionID {
						hasResumeCmd = true
					}
				}
			}
			assert.Equal(t, tt.wantResumeCmd, hasResumeCmd)
		})
	}
}

func TestSessionResume_OpenCodeExtractsSessionID(t *testing.T) {
	tests := []struct {
		name              string
		mockOutput        []byte
		wantSessionID     string
		wantResumeFlag    string
		priorSessionTurns int
	}{
		{
			name: "turn 1: extract sessionID from type:step_start JSON",
			mockOutput: []byte(`{"type":"step_start","timestamp":1775599542766,"sessionID":"ses_296052f0bffeFudXE4xOn0vSEJ"}` + "\n" +
				`{"type":"step_end","status":"ok","output":"Generated code"}`),
			wantSessionID:     "ses_296052f0bffeFudXE4xOn0vSEJ",
			wantResumeFlag:    "",
			priorSessionTurns: 0,
		},
		{
			name: "turn 2: use -s flag with extracted sessionID",
			mockOutput: []byte(`{"type":"step_start","timestamp":1775599542800,"sessionID":"ses_296052f0bffeFudXE4xOn0vSEJ"}` + "\n" +
				`{"type":"step_end","status":"ok","output":"Continued execution"}`),
			wantSessionID:     "ses_296052f0bffeFudXE4xOn0vSEJ",
			wantResumeFlag:    "-s",
			priorSessionTurns: 1,
		},
		{
			name: "JSON extraction fails but has prior turns: use -c fallback",
			mockOutput: []byte(`{"type":"step_start","timestamp":1775599542800}` + "\n" +
				`{"type":"step_end","status":"ok","output":"Using fallback"}`),
			wantSessionID:     "",
			wantResumeFlag:    "-c",
			priorSessionTurns: 2,
		},
		{
			name: "JSON extraction fails and no prior turns: no resume flag",
			mockOutput: []byte(`{"type":"step_start","timestamp":1775599542800}` + "\n" +
				`{"type":"step_end","status":"ok","output":"First execution"}`),
			wantSessionID:     "",
			wantResumeFlag:    "",
			priorSessionTurns: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			state := workflow.NewConversationState("")
			for i := 0; i < tt.priorSessionTurns; i++ {
				state.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "prior response"))
			}
			if tt.wantResumeFlag == "-s" {
				state.SessionID = "ses_296052f0bffeFudXE4xOn0vSEJ"
			}

			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, tt.wantSessionID, result.State.SessionID)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			if tt.wantResumeFlag != "" {
				assert.Contains(t, calls[0].Args, tt.wantResumeFlag,
					"expected resume flag %q in args %v", tt.wantResumeFlag, calls[0].Args)

				if tt.wantResumeFlag == "-s" {
					for i, arg := range calls[0].Args {
						if arg == "-s" && i+1 < len(calls[0].Args) {
							assert.Equal(t, tt.wantSessionID, calls[0].Args[i+1])
						}
					}
				}
			} else {
				assert.NotContains(t, calls[0].Args, "-s")
				assert.NotContains(t, calls[0].Args, "-c")
			}
		})
	}
}

func TestSessionResume_CursorExtractsChatID(t *testing.T) {
	tests := []struct {
		name           string
		mockOutput     []byte
		wantSessionID  string
		wantResumeFlag bool
	}{
		{
			name: "turn 1: extract chat_id from system init event",
			mockOutput: []byte(`{"type":"system","subtype":"init","chat_id":"chat-abc123","model":"composer-2"}` + "\n" +
				`{"type":"result","result":"Done"}`),
			wantSessionID:  "chat-abc123",
			wantResumeFlag: false,
		},
		{
			name: "turn 2: use --resume with extracted chat_id",
			mockOutput: []byte(`{"type":"system","subtype":"init","chat_id":"chat-abc123","model":"composer-2"}` + "\n" +
				`{"type":"result","result":"Continued"}`),
			wantSessionID:  "chat-abc123",
			wantResumeFlag: true,
		},
		{
			name:           "missing chat identifier: graceful fallback",
			mockOutput:     []byte(`{"type":"result","result":"No session info"}`),
			wantSessionID:  "",
			wantResumeFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewCursorProviderWithOptions(WithCursorExecutor(mockExec))

			state := workflow.NewConversationState("")
			if tt.wantResumeFlag {
				state.SessionID = "chat-abc123"
			}

			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantSessionID, result.State.SessionID)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			hasResumeFlag := false
			for i, arg := range calls[0].Args {
				if arg == "--resume" && i+1 < len(calls[0].Args) {
					hasResumeFlag = true
					if tt.wantResumeFlag {
						assert.Equal(t, "chat-abc123", calls[0].Args[i+1])
					}
				}
			}
			assert.Equal(t, tt.wantResumeFlag, hasResumeFlag)
		})
	}
}

func TestSessionResume_ContinueFromCrossStep(t *testing.T) {
	t.Run("Gemini: continue_from links turn 2 to turn 1 session ID", func(t *testing.T) {
		mockExec1 := mocks.NewMockCLIExecutor()
		mockExec1.SetOutput([]byte(`{"type":"init","session_id":"uuid-step-1","model":"gemini"}`+"\n"+
			`{"type":"result","result":"Step 1 response"}`), nil)
		provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec1))

		state := workflow.NewConversationState("system")
		result1, err := provider.ExecuteConversation(context.Background(), state, "step 1 prompt", nil, nil, nil)
		require.NoError(t, err)
		require.Equal(t, "uuid-step-1", result1.State.SessionID)

		mockExec2 := mocks.NewMockCLIExecutor()
		mockExec2.SetOutput([]byte(`{"type":"init","session_id":"uuid-step-1","model":"gemini"}`+"\n"+
			`{"type":"result","result":"Step 2 response with context"}`), nil)
		provider2 := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec2))

		state2 := workflow.NewConversationState("system")
		state2.SessionID = result1.State.SessionID
		_, err = provider2.ExecuteConversation(context.Background(), state2, "step 2 prompt", nil, nil, nil)
		require.NoError(t, err)

		calls2 := mockExec2.GetCalls()
		require.Len(t, calls2, 1)
		assert.Contains(t, calls2[0].Args, "--resume")
		assert.Contains(t, calls2[0].Args, "uuid-step-1")
	})

	t.Run("Codex: continue_from links turn 2 to turn 1 thread ID", func(t *testing.T) {
		mockExec1 := mocks.NewMockCLIExecutor()
		mockExec1.SetOutput([]byte(`{"type":"thread.started","thread_id":"thread-abc123"}`+"\n"+
			`{"type":"message","content":"Step 1 code"}`), nil)
		provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec1))

		state := workflow.NewConversationState("")
		result1, err := provider.ExecuteConversation(context.Background(), state, "step 1 prompt", nil, nil, nil)
		require.NoError(t, err)
		require.Equal(t, "thread-abc123", result1.State.SessionID)

		mockExec2 := mocks.NewMockCLIExecutor()
		mockExec2.SetOutput([]byte(`{"type":"thread.started","thread_id":"thread-abc123"}`+"\n"+
			`{"type":"message","content":"Step 2 code"}`), nil)
		provider2 := NewCodexProviderWithOptions(WithCodexExecutor(mockExec2))

		state2 := workflow.NewConversationState("")
		state2.SessionID = result1.State.SessionID
		_, err = provider2.ExecuteConversation(context.Background(), state2, "step 2 prompt", nil, nil, nil)
		require.NoError(t, err)

		calls2 := mockExec2.GetCalls()
		require.Len(t, calls2, 1)
		assert.Contains(t, calls2[0].Args, "resume")
		assert.Contains(t, calls2[0].Args, "thread-abc123")
	})

	t.Run("OpenCode: continue_from links turn 2 to turn 1 session ID", func(t *testing.T) {
		mockExec1 := mocks.NewMockCLIExecutor()
		mockExec1.SetOutput([]byte(`{"type":"step_start","sessionID":"ses_abc123"}`+"\n"+
			`{"type":"step_end","output":"Step 1"}`), nil)
		provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec1))

		state := workflow.NewConversationState("")
		result1, err := provider.ExecuteConversation(context.Background(), state, "step 1 prompt", nil, nil, nil)
		require.NoError(t, err)
		require.Equal(t, "ses_abc123", result1.State.SessionID)

		mockExec2 := mocks.NewMockCLIExecutor()
		mockExec2.SetOutput([]byte(`{"type":"step_start","sessionID":"ses_abc123"}`+"\n"+
			`{"type":"step_end","output":"Step 2"}`), nil)
		provider2 := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec2))

		state2 := workflow.NewConversationState("")
		state2.SessionID = result1.State.SessionID
		state2.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "step 1"))
		_, err = provider2.ExecuteConversation(context.Background(), state2, "step 2 prompt", nil, nil, nil)
		require.NoError(t, err)

		calls2 := mockExec2.GetCalls()
		require.Len(t, calls2, 1)
		assert.Contains(t, calls2[0].Args, "-s")
		assert.Contains(t, calls2[0].Args, "ses_abc123")
	})

	t.Run("Cursor: continue_from links turn 2 to turn 1 chat ID", func(t *testing.T) {
		mockExec1 := mocks.NewMockCLIExecutor()
		mockExec1.SetOutput([]byte(`{"type":"system","subtype":"init","chat_id":"chat-abc123"}`+"\n"+
			`{"type":"result","result":"Step 1"}`), nil)
		provider := NewCursorProviderWithOptions(WithCursorExecutor(mockExec1))

		state := workflow.NewConversationState("")
		result1, err := provider.ExecuteConversation(context.Background(), state, "step 1 prompt", nil, nil, nil)
		require.NoError(t, err)
		require.Equal(t, "chat-abc123", result1.State.SessionID)

		mockExec2 := mocks.NewMockCLIExecutor()
		mockExec2.SetOutput([]byte(`{"type":"system","subtype":"init","chat_id":"chat-abc123"}`+"\n"+
			`{"type":"result","result":"Step 2"}`), nil)
		provider2 := NewCursorProviderWithOptions(WithCursorExecutor(mockExec2))

		state2 := workflow.NewConversationState("")
		state2.SessionID = result1.State.SessionID
		_, err = provider2.ExecuteConversation(context.Background(), state2, "step 2 prompt", nil, nil, nil)
		require.NoError(t, err)

		calls2 := mockExec2.GetCalls()
		require.Len(t, calls2, 1)
		assert.Contains(t, calls2[0].Args, "--resume")
		assert.Contains(t, calls2[0].Args, "chat-abc123")
	})
}
