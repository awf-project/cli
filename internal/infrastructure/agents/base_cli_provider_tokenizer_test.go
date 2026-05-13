package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testExecuteHooks() cliProviderHooks {
	return cliProviderHooks{
		buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
			return []string{"--prompt", prompt}, nil
		},
	}
}

func testConversationHooks() cliProviderHooks {
	return cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"--prompt", prompt}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "", nil
		},
	}
}

// mockTokenizerWithTracker records calls and returns configured values.
type mockTokenizerWithTracker struct {
	countTokensResult      int
	countTokensError       error
	countTurnsTokensResult int
	countTurnsTokensError  error
	isEstimate             bool
	modelName              string

	countTokensCalls      []string
	countTurnsTokensCalls [][]string
}

func newMockTokenizerWithTracker(tokensResult int, isEst bool) *mockTokenizerWithTracker {
	return &mockTokenizerWithTracker{
		countTokensResult:      tokensResult,
		countTurnsTokensResult: tokensResult,
		isEstimate:             isEst,
		modelName:              "test-tokenizer",
	}
}

func (m *mockTokenizerWithTracker) CountTokens(text string) (int, error) {
	m.countTokensCalls = append(m.countTokensCalls, text)
	return m.countTokensResult, m.countTokensError
}

func (m *mockTokenizerWithTracker) CountTurnsTokens(turns []string) (int, error) {
	m.countTurnsTokensCalls = append(m.countTurnsTokensCalls, turns)
	return m.countTurnsTokensResult, m.countTurnsTokensError
}

func (m *mockTokenizerWithTracker) IsEstimate() bool {
	return m.isEstimate
}

func (m *mockTokenizerWithTracker) ModelName() string {
	return m.modelName
}

func TestBaseCLIProvider_HasTokenizerField(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, testExecuteHooks())

	require.NotNil(t, provider)
	assert.NotNil(t, provider.tokenizer)
}

func TestBaseCLIProvider_Execute_UsesInjectedTokenizer(t *testing.T) {
	tests := []struct {
		name           string
		outputStr      string
		expectedTokens int
	}{
		{
			name:           "simple output",
			outputStr:      "4",
			expectedTokens: 42,
		},
		{
			name:           "longer output",
			outputStr:      "This is a longer response",
			expectedTokens: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(tt.outputStr), nil)

			mockTokenizer := newMockTokenizerWithTracker(tt.expectedTokens, false)

			provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, testExecuteHooks())
			provider.tokenizer = mockTokenizer

			result, _, err := provider.execute(context.Background(), "test prompt", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedTokens, result.Tokens)
			assert.Len(t, mockTokenizer.countTokensCalls, 1)
			assert.Equal(t, tt.outputStr, mockTokenizer.countTokensCalls[0])
		})
	}
}

func TestBaseCLIProvider_Execute_TokensEstimatedFromTokenizer(t *testing.T) {
	tests := []struct {
		name              string
		isEstimate        bool
		expectedEstimated bool
	}{
		{
			name:              "approximation tokenizer",
			isEstimate:        true,
			expectedEstimated: true,
		},
		{
			name:              "exact tokenizer",
			isEstimate:        false,
			expectedEstimated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("output"), nil)

			mockTokenizer := newMockTokenizerWithTracker(42, tt.isEstimate)

			provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, testExecuteHooks())
			provider.tokenizer = mockTokenizer

			result, _, err := provider.execute(context.Background(), "test", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedEstimated, result.TokensEstimated)
		})
	}
}

func TestBaseCLIProvider_ExecuteConversation_UsesInjectedTokenizerForAssistantTokens(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("assistant response"), nil)

	mockTokenizer := newMockTokenizerWithTracker(77, false)

	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, testConversationHooks())
	provider.tokenizer = mockTokenizer

	state := &workflow.ConversationState{
		SessionID: "",
		Turns:     []workflow.Turn{},
	}

	result, _, err := provider.executeConversation(context.Background(), state, "user prompt", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 77, result.TokensOutput)
	assert.Len(t, mockTokenizer.countTokensCalls, 1)
	assert.Equal(t, "assistant response", mockTokenizer.countTokensCalls[0])
}

func TestBaseCLIProvider_ExecuteConversation_UsesCountTurnsTokens(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)

	mockTokenizer := newMockTokenizerWithTracker(100, false)

	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, testConversationHooks())
	provider.tokenizer = mockTokenizer

	state := &workflow.ConversationState{
		SessionID: "",
		Turns: []workflow.Turn{
			{Role: workflow.TurnRoleUser, Content: "first user message"},
			{Role: workflow.TurnRoleAssistant, Content: "first assistant response"},
		},
	}

	result, _, err := provider.executeConversation(context.Background(), state, "second user message", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 100, result.TokensInput)
	assert.Len(t, mockTokenizer.countTurnsTokensCalls, 1)
	// After adding user turn (3 turns total) and assistant turn (4 turns total),
	// limit = 4 - 1 = 3, so countTurnsTokens receives turns[0:3] = original 2 + new user turn
	assert.Len(t, mockTokenizer.countTurnsTokensCalls[0], 3)
	assert.Equal(t, "first user message", mockTokenizer.countTurnsTokensCalls[0][0])
	assert.Equal(t, "first assistant response", mockTokenizer.countTurnsTokensCalls[0][1])
	assert.Equal(t, "second user message", mockTokenizer.countTurnsTokensCalls[0][2])
}

func TestBaseCLIProvider_ExecuteConversation_TokensEstimatedFromTokenizer(t *testing.T) {
	tests := []struct {
		name              string
		isEstimate        bool
		expectedEstimated bool
	}{
		{
			name:              "approximation tokenizer",
			isEstimate:        true,
			expectedEstimated: true,
		},
		{
			name:              "exact tokenizer",
			isEstimate:        false,
			expectedEstimated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)

			mockTokenizer := newMockTokenizerWithTracker(42, tt.isEstimate)

			provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, testConversationHooks())
			provider.tokenizer = mockTokenizer

			state := &workflow.ConversationState{
				SessionID: "",
				Turns:     []workflow.Turn{},
			}

			result, _, err := provider.executeConversation(context.Background(), state, "test", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedEstimated, result.TokensEstimated)
		})
	}
}

func TestBaseCLIProvider_ExecuteConversation_NoTurnMutation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)

	mockTokenizer := newMockTokenizerWithTracker(50, false)

	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, testConversationHooks())
	provider.tokenizer = mockTokenizer

	initialState := &workflow.ConversationState{
		SessionID: "",
		Turns: []workflow.Turn{
			{Role: workflow.TurnRoleUser, Content: "initial turn", Tokens: 10},
		},
	}

	originalTokens := initialState.Turns[0].Tokens

	_, _, err := provider.executeConversation(context.Background(), initialState, "test prompt", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, originalTokens, initialState.Turns[0].Tokens, "prior turn tokens should not be mutated")
	assert.Len(t, initialState.Turns, 1, "original state should not be modified")
}

func TestBaseCLIProvider_ExecuteConversation_ExtractsLastTurnContents(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("new response"), nil)

	mockTokenizer := newMockTokenizerWithTracker(100, false)

	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, testConversationHooks())
	provider.tokenizer = mockTokenizer

	state := &workflow.ConversationState{
		SessionID: "",
		Turns: []workflow.Turn{
			{Role: workflow.TurnRoleUser, Content: "turn 1"},
			{Role: workflow.TurnRoleAssistant, Content: "turn 2"},
			{Role: workflow.TurnRoleUser, Content: "turn 3"},
		},
	}

	result, _, err := provider.executeConversation(context.Background(), state, "new user message", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, mockTokenizer.countTurnsTokensCalls, 1)

	turnContents := mockTokenizer.countTurnsTokensCalls[0]
	// Starting with 3 turns, add user (4 total), add assistant (5 total)
	// limit = 5 - 1 = 4, so turns[0:4] = original 3 + new user message
	assert.Len(t, turnContents, 4, "should include all prior turns and new user message, excluding newly added assistant turn")
	assert.Equal(t, "turn 1", turnContents[0])
	assert.Equal(t, "turn 2", turnContents[1])
	assert.Equal(t, "turn 3", turnContents[2])
	assert.Equal(t, "new user message", turnContents[3])
}

func TestBaseCLIProvider_ExecuteConversation_EmptyTurnsCountTurnsTokens(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)

	mockTokenizer := newMockTokenizerWithTracker(42, false)

	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, testConversationHooks())
	provider.tokenizer = mockTokenizer

	state := &workflow.ConversationState{
		SessionID: "",
		Turns:     []workflow.Turn{},
	}

	result, _, err := provider.executeConversation(context.Background(), state, "first message", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, mockTokenizer.countTurnsTokensCalls, 1)
	// Starting with 0 turns, add user (1 turn total), add assistant (2 turns total)
	// limit = 2 - 1 = 1, so turns[0:1] = the first user message
	assert.Len(t, mockTokenizer.countTurnsTokensCalls[0], 1, "should include the first user message added by executeConversation")
	assert.Equal(t, "first message", mockTokenizer.countTurnsTokensCalls[0][0])
}

func TestBaseCLIProvider_TokenizerNotCalledOnError(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockTokenizer := newMockTokenizerWithTracker(42, false)

	hooks := cliProviderHooks{
		buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
			return nil, errors.New("build args failed")
		},
	}

	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, hooks)
	provider.tokenizer = mockTokenizer

	_, _, err := provider.execute(context.Background(), "test", nil, nil, nil)

	require.Error(t, err)
	assert.Empty(t, mockTokenizer.countTokensCalls, "tokenizer should not be called on buildExecuteArgs error")
}

func TestBaseCLIProvider_TokenizerWithBothExecutePaths(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("output"), nil)

	mockTokenizer := newMockTokenizerWithTracker(88, false)

	conversationHooks := testConversationHooks()
	conversationHooks.extractSessionID = func(output string) (string, error) {
		return "session-1", nil
	}

	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, testExecuteHooks())
	provider.tokenizer = mockTokenizer

	result1, _, err1 := provider.execute(context.Background(), "test1", nil, nil, nil)
	require.NoError(t, err1)
	require.NotNil(t, result1)
	assert.Equal(t, 88, result1.Tokens)

	mockExec.SetOutput([]byte("output2"), nil)
	provider.hooks = conversationHooks

	state := &workflow.ConversationState{
		SessionID: "",
		Turns:     []workflow.Turn{},
	}

	result2, _, err2 := provider.executeConversation(context.Background(), state, "test2", nil, nil, nil)
	require.NoError(t, err2)
	require.NotNil(t, result2)
	assert.Equal(t, 88, result2.TokensOutput)

	assert.Len(t, mockTokenizer.countTokensCalls, 2, "tokenizer should be used by both execute paths")
	assert.Len(t, mockTokenizer.countTurnsTokensCalls, 1, "only conversation path uses CountTurnsTokens")
}
