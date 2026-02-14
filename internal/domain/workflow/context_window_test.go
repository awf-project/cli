package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F033

func TestNewContextWindowManager(t *testing.T) {
	manager := NewContextWindowManager()

	require.NotNil(t, manager)
}

func TestContextWindowManager_ApplyStrategy_SlidingWindow(t *testing.T) {
	manager := NewContextWindowManager()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System prompt", Tokens: 50},
		{Role: TurnRoleUser, Content: "Turn 1", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "Response 1", Tokens: 150},
		{Role: TurnRoleUser, Content: "Turn 2", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "Response 2", Tokens: 150},
	}

	maxTokens := 300

	truncated, wasTruncated, err := manager.ApplyStrategy(turns, maxTokens, StrategySlidingWindow)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	assert.True(t, wasTruncated)
	// System prompt should be preserved
	assert.GreaterOrEqual(t, len(truncated), 1)
	if len(truncated) > 0 {
		assert.Equal(t, TurnRoleSystem, truncated[0].Role)
	}
	// Total tokens should be within limit
	totalTokens := 0
	for _, turn := range truncated {
		totalTokens += turn.Tokens
	}
	assert.LessOrEqual(t, totalTokens, maxTokens)
}

func TestContextWindowManager_ApplyStrategy_NoTruncationNeeded(t *testing.T) {
	manager := NewContextWindowManager()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System", Tokens: 50},
		{Role: TurnRoleUser, Content: "User", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "Assistant", Tokens: 150},
	}

	maxTokens := 1000

	truncated, wasTruncated, err := manager.ApplyStrategy(turns, maxTokens, StrategySlidingWindow)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	assert.False(t, wasTruncated)
	assert.Equal(t, len(turns), len(truncated))
}

func TestContextWindowManager_ApplyStrategy_EmptyTurns(t *testing.T) {
	manager := NewContextWindowManager()

	turns := []Turn{}
	maxTokens := 1000

	truncated, wasTruncated, err := manager.ApplyStrategy(turns, maxTokens, StrategySlidingWindow)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	assert.False(t, wasTruncated)
	assert.Empty(t, truncated)
}

func TestContextWindowManager_ApplyStrategy_InvalidStrategy(t *testing.T) {
	manager := NewContextWindowManager()

	turns := []Turn{
		{Role: TurnRoleUser, Content: "Test", Tokens: 100},
	}

	truncated, wasTruncated, err := manager.ApplyStrategy(turns, 1000, ContextWindowStrategy("invalid"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "strategy")
	assert.Nil(t, truncated)
	assert.False(t, wasTruncated)
}

func TestContextWindowManager_ApplyStrategy_NegativeMaxTokens(t *testing.T) {
	manager := NewContextWindowManager()

	turns := []Turn{
		{Role: TurnRoleUser, Content: "Test", Tokens: 100},
	}

	truncated, wasTruncated, err := manager.ApplyStrategy(turns, -1, StrategySlidingWindow)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_tokens")
	assert.Nil(t, truncated)
	assert.False(t, wasTruncated)
}

func TestContextWindowManager_ApplyStrategy_ZeroMaxTokens(t *testing.T) {
	manager := NewContextWindowManager()

	turns := []Turn{
		{Role: TurnRoleUser, Content: "Test", Tokens: 100},
	}

	truncated, wasTruncated, err := manager.ApplyStrategy(turns, 0, StrategySlidingWindow)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_tokens")
	assert.Nil(t, truncated)
	assert.False(t, wasTruncated)
}

func TestContextWindowManager_PreserveSystemPrompt(t *testing.T) {
	manager := NewContextWindowManager()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System prompt", Tokens: 50},
		{Role: TurnRoleUser, Content: "User message", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "Assistant response", Tokens: 150},
	}

	systemTurn, otherTurns := manager.PreserveSystemPrompt(turns)

	require.NotNil(t, systemTurn)
	assert.Equal(t, TurnRoleSystem, systemTurn.Role)
	assert.Equal(t, "System prompt", systemTurn.Content)
	assert.Equal(t, 50, systemTurn.Tokens)

	require.NotNil(t, otherTurns)
	assert.Len(t, otherTurns, 2)
	assert.Equal(t, TurnRoleUser, otherTurns[0].Role)
	assert.Equal(t, TurnRoleAssistant, otherTurns[1].Role)
}

func TestContextWindowManager_PreserveSystemPrompt_NoSystem(t *testing.T) {
	manager := NewContextWindowManager()

	turns := []Turn{
		{Role: TurnRoleUser, Content: "User message", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "Assistant response", Tokens: 150},
	}

	systemTurn, otherTurns := manager.PreserveSystemPrompt(turns)

	assert.Nil(t, systemTurn)
	require.NotNil(t, otherTurns)
	assert.Len(t, otherTurns, 2)
}

func TestContextWindowManager_PreserveSystemPrompt_SystemNotFirst(t *testing.T) {
	manager := NewContextWindowManager()

	turns := []Turn{
		{Role: TurnRoleUser, Content: "User message", Tokens: 100},
		{Role: TurnRoleSystem, Content: "System prompt", Tokens: 50},
		{Role: TurnRoleAssistant, Content: "Assistant response", Tokens: 150},
	}

	systemTurn, otherTurns := manager.PreserveSystemPrompt(turns)

	// Should extract system prompt even if not first
	require.NotNil(t, systemTurn)
	assert.Equal(t, TurnRoleSystem, systemTurn.Role)
	require.NotNil(t, otherTurns)
	assert.Len(t, otherTurns, 2)
}

func TestContextWindowManager_PreserveSystemPrompt_EmptyTurns(t *testing.T) {
	manager := NewContextWindowManager()

	turns := []Turn{}

	systemTurn, otherTurns := manager.PreserveSystemPrompt(turns)

	assert.Nil(t, systemTurn)
	require.NotNil(t, otherTurns)
	assert.Empty(t, otherTurns)
}

func TestContextWindowManager_PreserveSystemPrompt_OnlySystem(t *testing.T) {
	manager := NewContextWindowManager()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System only", Tokens: 50},
	}

	systemTurn, otherTurns := manager.PreserveSystemPrompt(turns)

	require.NotNil(t, systemTurn)
	assert.Equal(t, TurnRoleSystem, systemTurn.Role)
	require.NotNil(t, otherTurns)
	assert.Empty(t, otherTurns)
}

func TestContextWindowManager_CalculateTotalTokens(t *testing.T) {
	manager := NewContextWindowManager()

	tests := []struct {
		name     string
		turns    []Turn
		expected int
	}{
		{
			name: "multiple turns",
			turns: []Turn{
				{Role: TurnRoleSystem, Content: "System", Tokens: 50},
				{Role: TurnRoleUser, Content: "User", Tokens: 100},
				{Role: TurnRoleAssistant, Content: "Assistant", Tokens: 150},
			},
			expected: 300,
		},
		{
			name: "single turn",
			turns: []Turn{
				{Role: TurnRoleUser, Content: "User", Tokens: 42},
			},
			expected: 42,
		},
		{
			name:     "empty turns",
			turns:    []Turn{},
			expected: 0,
		},
		{
			name: "turns with zero tokens",
			turns: []Turn{
				{Role: TurnRoleUser, Content: "User", Tokens: 0},
				{Role: TurnRoleAssistant, Content: "Assistant", Tokens: 0},
			},
			expected: 0,
		},
		{
			name: "large conversation",
			turns: []Turn{
				{Role: TurnRoleSystem, Content: "System", Tokens: 100},
				{Role: TurnRoleUser, Content: "User1", Tokens: 500},
				{Role: TurnRoleAssistant, Content: "Assistant1", Tokens: 800},
				{Role: TurnRoleUser, Content: "User2", Tokens: 300},
				{Role: TurnRoleAssistant, Content: "Assistant2", Tokens: 1200},
			},
			expected: 2900,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total := manager.CalculateTotalTokens(tt.turns)
			assert.Equal(t, tt.expected, total)
		})
	}
}

func TestContextWindowManager_EstimateTokens(t *testing.T) {
	manager := NewContextWindowManager()

	tests := []struct {
		name     string
		content  string
		expected int // Approximate estimation: len/4
	}{
		{
			name:     "short text",
			content:  "Hello world",
			expected: 2, // 11 chars / 4 ≈ 2
		},
		{
			name:     "medium text",
			content:  "This is a test message with multiple words",
			expected: 10, // 43 chars / 4 ≈ 10
		},
		{
			name:     "long text",
			content:  string(make([]byte, 400)),
			expected: 100, // 400 chars / 4 = 100
		},
		{
			name:     "empty content",
			content:  "",
			expected: 0,
		},
		{
			name:     "single character",
			content:  "x",
			expected: 0, // 1 char / 4 = 0
		},
		{
			name:     "unicode content",
			content:  "Hello 世界",
			expected: 2, // Character-based estimation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimated := manager.EstimateTokens(tt.content)
			assert.GreaterOrEqual(t, estimated, 0)
			// Estimate should be reasonable (within 50% of expected)
			if tt.expected > 0 {
				assert.InDelta(t, tt.expected, estimated, float64(tt.expected)*0.5)
			}
		})
	}
}

func TestNewSlidingWindowStrategy(t *testing.T) {
	strategy := NewSlidingWindowStrategy()

	require.NotNil(t, strategy)
}

func TestSlidingWindowStrategy_Apply(t *testing.T) {
	strategy := NewSlidingWindowStrategy()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System prompt", Tokens: 50},
		{Role: TurnRoleUser, Content: "Turn 1", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "Response 1", Tokens: 150},
		{Role: TurnRoleUser, Content: "Turn 2", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "Response 2", Tokens: 150},
		{Role: TurnRoleUser, Content: "Turn 3", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "Response 3", Tokens: 150},
	}

	maxTokens := 400

	truncated, wasTruncated, err := strategy.Apply(turns, maxTokens)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	assert.True(t, wasTruncated)

	// System prompt must be preserved
	assert.GreaterOrEqual(t, len(truncated), 1)
	if len(truncated) > 0 {
		assert.Equal(t, TurnRoleSystem, truncated[0].Role)
		assert.Equal(t, "System prompt", truncated[0].Content)
	}

	// Total tokens should be within limit
	totalTokens := 0
	for _, turn := range truncated {
		totalTokens += turn.Tokens
	}
	assert.LessOrEqual(t, totalTokens, maxTokens)

	// Most recent turns should be preserved
	lastTurn := truncated[len(truncated)-1]
	assert.Equal(t, TurnRoleAssistant, lastTurn.Role)
	assert.Equal(t, "Response 3", lastTurn.Content)
}

func TestSlidingWindowStrategy_Apply_NoTruncation(t *testing.T) {
	strategy := NewSlidingWindowStrategy()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System", Tokens: 50},
		{Role: TurnRoleUser, Content: "User", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "Assistant", Tokens: 150},
	}

	maxTokens := 1000

	truncated, wasTruncated, err := strategy.Apply(turns, maxTokens)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	assert.False(t, wasTruncated)
	assert.Equal(t, len(turns), len(truncated))
	assert.Equal(t, turns, truncated)
}

func TestSlidingWindowStrategy_Apply_SystemPromptOnly(t *testing.T) {
	strategy := NewSlidingWindowStrategy()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System prompt", Tokens: 50},
	}

	maxTokens := 100

	truncated, wasTruncated, err := strategy.Apply(turns, maxTokens)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	assert.False(t, wasTruncated)
	assert.Len(t, truncated, 1)
	assert.Equal(t, TurnRoleSystem, truncated[0].Role)
}

func TestSlidingWindowStrategy_Apply_SystemPromptExceedsLimit(t *testing.T) {
	strategy := NewSlidingWindowStrategy()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "Very long system prompt", Tokens: 500},
		{Role: TurnRoleUser, Content: "User", Tokens: 100},
	}

	maxTokens := 300

	truncated, _, err := strategy.Apply(turns, maxTokens)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	// System prompt is always preserved even if it exceeds limit
	assert.GreaterOrEqual(t, len(truncated), 1)
	assert.Equal(t, TurnRoleSystem, truncated[0].Role)
}

func TestSlidingWindowStrategy_Apply_EmptyTurns(t *testing.T) {
	strategy := NewSlidingWindowStrategy()

	turns := []Turn{}
	maxTokens := 1000

	truncated, wasTruncated, err := strategy.Apply(turns, maxTokens)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	assert.False(t, wasTruncated)
	assert.Empty(t, truncated)
}

func TestSlidingWindowStrategy_Apply_InvalidMaxTokens(t *testing.T) {
	strategy := NewSlidingWindowStrategy()

	turns := []Turn{
		{Role: TurnRoleUser, Content: "Test", Tokens: 100},
	}

	tests := []struct {
		name      string
		maxTokens int
	}{
		{"negative", -1},
		{"zero", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncated, wasTruncated, err := strategy.Apply(turns, tt.maxTokens)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "max_tokens")
			assert.Nil(t, truncated)
			assert.False(t, wasTruncated)
		})
	}
}

func TestSlidingWindowStrategy_Apply_PreservesOrder(t *testing.T) {
	strategy := NewSlidingWindowStrategy()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System", Tokens: 50},
		{Role: TurnRoleUser, Content: "Q1", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "A1", Tokens: 100},
		{Role: TurnRoleUser, Content: "Q2", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "A2", Tokens: 100},
		{Role: TurnRoleUser, Content: "Q3", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "A3", Tokens: 100},
	}

	maxTokens := 400

	truncated, wasTruncated, err := strategy.Apply(turns, maxTokens)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	assert.True(t, wasTruncated)

	// Verify chronological order is maintained
	for i := 1; i < len(truncated); i++ {
		// Each turn should come after the previous one in the original sequence
		prevContent := truncated[i-1].Content
		currContent := truncated[i].Content

		// Find indices in original
		prevIdx := -1
		currIdx := -1
		for j, turn := range turns {
			if turn.Content == prevContent {
				prevIdx = j
			}
			if turn.Content == currContent {
				currIdx = j
			}
		}

		assert.Less(t, prevIdx, currIdx, "Turn order should be preserved")
	}
}

func TestNewSummarizeStrategy(t *testing.T) {
	strategy := NewSummarizeStrategy()

	require.NotNil(t, strategy)
}

func TestSummarizeStrategy_Apply_NotImplemented(t *testing.T) {
	strategy := NewSummarizeStrategy()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System", Tokens: 50},
		{Role: TurnRoleUser, Content: "User", Tokens: 100},
	}

	truncated, _, err := strategy.Apply(turns, 1000)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Nil(t, truncated)
}

func TestNewTruncateMiddleStrategy(t *testing.T) {
	strategy := NewTruncateMiddleStrategy()

	require.NotNil(t, strategy)
}

func TestTruncateMiddleStrategy_Apply_NotImplemented(t *testing.T) {
	strategy := NewTruncateMiddleStrategy()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System", Tokens: 50},
		{Role: TurnRoleUser, Content: "User", Tokens: 100},
	}

	truncated, _, err := strategy.Apply(turns, 1000)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Nil(t, truncated)
}

func TestNewContextWindowState(t *testing.T) {
	tests := []struct {
		name     string
		strategy ContextWindowStrategy
	}{
		{"none", StrategyNone},
		{"sliding_window", StrategySlidingWindow},
		{"summarize", StrategySummarize},
		{"truncate_middle", StrategyTruncateMiddle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewContextWindowState(tt.strategy)

			require.NotNil(t, state)
			assert.Equal(t, tt.strategy, state.Strategy)
			assert.Equal(t, 0, state.TruncationCount)
			assert.Equal(t, 0, state.TurnsDropped)
			assert.Equal(t, 0, state.TokensDropped)
			assert.Equal(t, 0, state.LastTruncatedAt)
		})
	}
}

func TestContextWindowState_RecordTruncation(t *testing.T) {
	state := NewContextWindowState(StrategySlidingWindow)

	// First truncation
	state.RecordTruncation(2, 300, 5)

	assert.Equal(t, 1, state.TruncationCount)
	assert.Equal(t, 2, state.TurnsDropped)
	assert.Equal(t, 300, state.TokensDropped)
	assert.Equal(t, 5, state.LastTruncatedAt)

	// Second truncation
	state.RecordTruncation(3, 450, 10)

	assert.Equal(t, 2, state.TruncationCount)
	assert.Equal(t, 5, state.TurnsDropped)    // 2 + 3
	assert.Equal(t, 750, state.TokensDropped) // 300 + 450
	assert.Equal(t, 10, state.LastTruncatedAt)
}

func TestContextWindowState_RecordTruncation_ZeroDropped(t *testing.T) {
	state := NewContextWindowState(StrategySlidingWindow)

	state.RecordTruncation(0, 0, 5)

	assert.Equal(t, 1, state.TruncationCount)
	assert.Equal(t, 0, state.TurnsDropped)
	assert.Equal(t, 0, state.TokensDropped)
	assert.Equal(t, 5, state.LastTruncatedAt)
}

func TestContextWindowState_RecordTruncation_NegativeValues(t *testing.T) {
	state := NewContextWindowState(StrategySlidingWindow)

	// Should handle negative values gracefully (defensive programming)
	state.RecordTruncation(-1, -100, 3)

	// Behavior depends on implementation
	assert.GreaterOrEqual(t, state.TruncationCount, 1)
}

func TestContextWindowState_WasTruncated(t *testing.T) {
	tests := []struct {
		name            string
		truncationCount int
		expected        bool
	}{
		{"no truncation", 0, false},
		{"single truncation", 1, true},
		{"multiple truncations", 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &ContextWindowState{
				Strategy:        StrategySlidingWindow,
				TruncationCount: tt.truncationCount,
			}

			assert.Equal(t, tt.expected, state.WasTruncated())
		})
	}
}

func TestSlidingWindowStrategy_Apply_SingleTurnExceedsLimit(t *testing.T) {
	strategy := NewSlidingWindowStrategy()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System", Tokens: 50},
		{Role: TurnRoleUser, Content: "Very long user message", Tokens: 500},
	}

	maxTokens := 100

	truncated, _, err := strategy.Apply(turns, maxTokens)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	// Should preserve system prompt + most recent turn even if exceeding limit
	assert.GreaterOrEqual(t, len(truncated), 1)
	assert.Equal(t, TurnRoleSystem, truncated[0].Role)
}

func TestSlidingWindowStrategy_Apply_AllTurnsExceedLimit(t *testing.T) {
	strategy := NewSlidingWindowStrategy()

	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System", Tokens: 200},
		{Role: TurnRoleUser, Content: "User1", Tokens: 300},
		{Role: TurnRoleAssistant, Content: "Assistant1", Tokens: 400},
		{Role: TurnRoleUser, Content: "User2", Tokens: 350},
	}

	maxTokens := 100

	truncated, _, err := strategy.Apply(turns, maxTokens)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	// Should preserve at minimum: system prompt + most recent turn
	assert.GreaterOrEqual(t, len(truncated), 1)
	assert.Equal(t, TurnRoleSystem, truncated[0].Role)
}

func TestContextWindowManager_ApplyStrategy_LargeConversation(t *testing.T) {
	manager := NewContextWindowManager()

	// Create a large conversation with 100 turns
	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System", Tokens: 100},
	}

	for i := 0; i < 50; i++ {
		turns = append(turns,
			Turn{
				Role:    TurnRoleUser,
				Content: "User message",
				Tokens:  50,
			},
			Turn{
				Role:    TurnRoleAssistant,
				Content: "Assistant response",
				Tokens:  100,
			},
		)
	}

	maxTokens := 1000

	truncated, wasTruncated, err := manager.ApplyStrategy(turns, maxTokens, StrategySlidingWindow)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	assert.True(t, wasTruncated)

	// Verify total tokens within limit
	totalTokens := manager.CalculateTotalTokens(truncated)
	assert.LessOrEqual(t, totalTokens, maxTokens)

	// Verify system prompt preserved
	assert.GreaterOrEqual(t, len(truncated), 1)
	assert.Equal(t, TurnRoleSystem, truncated[0].Role)
}

func TestContextWindowState_MultipleTruncations(t *testing.T) {
	state := NewContextWindowState(StrategySlidingWindow)

	// Simulate multiple truncation events
	truncations := []struct {
		turnsDropped  int
		tokensDropped int
		currentTurn   int
	}{
		{2, 300, 5},
		{1, 150, 7},
		{3, 450, 12},
		{2, 280, 18},
	}

	for _, tr := range truncations {
		state.RecordTruncation(tr.turnsDropped, tr.tokensDropped, tr.currentTurn)
	}

	assert.Equal(t, 4, state.TruncationCount)
	assert.Equal(t, 8, state.TurnsDropped)     // 2+1+3+2
	assert.Equal(t, 1180, state.TokensDropped) // 300+150+450+280
	assert.Equal(t, 18, state.LastTruncatedAt)
	assert.True(t, state.WasTruncated())
}

func TestContextWindowManager_FullWorkflow(t *testing.T) {
	manager := NewContextWindowManager()

	// Build conversation with system prompt
	turns := []Turn{
		{Role: TurnRoleSystem, Content: "You are a helpful assistant", Tokens: 100},
		{Role: TurnRoleUser, Content: "Question 1", Tokens: 50},
		{Role: TurnRoleAssistant, Content: "Answer 1", Tokens: 150},
		{Role: TurnRoleUser, Content: "Question 2", Tokens: 50},
		{Role: TurnRoleAssistant, Content: "Answer 2", Tokens: 150},
		{Role: TurnRoleUser, Content: "Question 3", Tokens: 50},
		{Role: TurnRoleAssistant, Content: "Answer 3", Tokens: 150},
	}

	// Extract system prompt
	systemTurn, otherTurns := manager.PreserveSystemPrompt(turns)
	assert.NotNil(t, systemTurn)
	assert.Len(t, otherTurns, 6)

	// Calculate total tokens
	totalTokens := manager.CalculateTotalTokens(turns)
	assert.Equal(t, 700, totalTokens) // 100+50+150+50+150+50+150

	// Apply sliding window strategy
	maxTokens := 400
	truncated, wasTruncated, err := manager.ApplyStrategy(turns, maxTokens, StrategySlidingWindow)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	assert.True(t, wasTruncated)

	// Verify system prompt preserved
	assert.Equal(t, TurnRoleSystem, truncated[0].Role)

	// Verify within token limit
	truncatedTotal := manager.CalculateTotalTokens(truncated)
	assert.LessOrEqual(t, truncatedTotal, maxTokens)

	// Verify most recent turns preserved
	lastTurn := truncated[len(truncated)-1]
	assert.Equal(t, "Answer 3", lastTurn.Content)
}

func TestSlidingWindowStrategy_CompleteExample(t *testing.T) {
	strategy := NewSlidingWindowStrategy()
	state := NewContextWindowState(StrategySlidingWindow)

	// Initial conversation
	turns := []Turn{
		{Role: TurnRoleSystem, Content: "System", Tokens: 50},
		{Role: TurnRoleUser, Content: "Q1", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "A1", Tokens: 100},
		{Role: TurnRoleUser, Content: "Q2", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "A2", Tokens: 100},
		{Role: TurnRoleUser, Content: "Q3", Tokens: 100},
		{Role: TurnRoleAssistant, Content: "A3", Tokens: 100},
	}

	maxTokens := 300

	// Apply truncation
	truncated, wasTruncated, err := strategy.Apply(turns, maxTokens)

	require.NoError(t, err)
	require.NotNil(t, truncated)
	assert.True(t, wasTruncated)

	// Calculate dropped turns and tokens
	turnsDropped := len(turns) - len(truncated)
	tokensDropped := 0
	for i := 0; i < turnsDropped; i++ {
		if i < len(turns) && turns[i].Role != TurnRoleSystem {
			tokensDropped += turns[i].Tokens
		}
	}

	// Record truncation
	state.RecordTruncation(turnsDropped, tokensDropped, len(turns))

	// Verify state
	assert.True(t, state.WasTruncated())
	assert.Equal(t, 1, state.TruncationCount)
	assert.Greater(t, state.TurnsDropped, 0)
	assert.Greater(t, state.TokensDropped, 0)
	assert.Equal(t, len(turns), state.LastTruncatedAt)
}

func TestContextWindowManager_EstimateAndCalculate(t *testing.T) {
	manager := NewContextWindowManager()

	content := "This is a sample message for token estimation."

	// Estimate tokens
	estimated := manager.EstimateTokens(content)
	assert.Greater(t, estimated, 0)

	// Create turn with estimated tokens
	turn := Turn{
		Role:    TurnRoleUser,
		Content: content,
		Tokens:  estimated,
	}

	// Calculate total with single turn
	total := manager.CalculateTotalTokens([]Turn{turn})
	assert.Equal(t, estimated, total)
}
