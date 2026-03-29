package workflow

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockExpressionValidator returns an ExpressionCompiler for tests that validates syntax.
// It checks for unbalanced quotes as a simple syntax validation.
func mockExpressionValidator(expr string) error {
	if strings.TrimSpace(expr) == "" {
		return nil
	}
	// Simple check for unbalanced single quotes (mimics real validator behavior)
	singleQuotes := strings.Count(expr, "'")
	if singleQuotes%2 != 0 {
		return errors.New("expression compilation failed: unbalanced quotes")
	}
	return nil
}

// Feature: F033

func TestTurnRole_Constants(t *testing.T) {
	assert.Equal(t, TurnRole("system"), TurnRoleSystem)
	assert.Equal(t, TurnRole("user"), TurnRoleUser)
	assert.Equal(t, TurnRole("assistant"), TurnRoleAssistant)
}

func TestNewTurn(t *testing.T) {
	tests := []struct {
		name    string
		role    TurnRole
		content string
	}{
		{
			name:    "system turn",
			role:    TurnRoleSystem,
			content: "You are a helpful assistant.",
		},
		{
			name:    "user turn",
			role:    TurnRoleUser,
			content: "Analyze this code.",
		},
		{
			name:    "assistant turn",
			role:    TurnRoleAssistant,
			content: "I found 3 issues.",
		},
		{
			name:    "empty content",
			role:    TurnRoleUser,
			content: "",
		},
		{
			name:    "multiline content",
			role:    TurnRoleAssistant,
			content: "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			turn := NewTurn(tt.role, tt.content)

			require.NotNil(t, turn)
			assert.Equal(t, tt.role, turn.Role)
			assert.Equal(t, tt.content, turn.Content)
			assert.GreaterOrEqual(t, turn.Tokens, 0)
		})
	}
}

func TestNewTurn_LargeContent(t *testing.T) {
	largeContent := string(make([]byte, 100000))
	turn := NewTurn(TurnRoleUser, largeContent)

	require.NotNil(t, turn)
	assert.Equal(t, TurnRoleUser, turn.Role)
	assert.Equal(t, largeContent, turn.Content)
}

func TestTurn_Validate(t *testing.T) {
	tests := []struct {
		name    string
		turn    Turn
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid system turn",
			turn: Turn{
				Role:    TurnRoleSystem,
				Content: "You are a helpful assistant.",
				Tokens:  10,
			},
			wantErr: false,
		},
		{
			name: "valid user turn",
			turn: Turn{
				Role:    TurnRoleUser,
				Content: "Analyze this.",
				Tokens:  5,
			},
			wantErr: false,
		},
		{
			name: "valid assistant turn",
			turn: Turn{
				Role:    TurnRoleAssistant,
				Content: "Analysis complete.",
				Tokens:  8,
			},
			wantErr: false,
		},
		{
			name: "empty role",
			turn: Turn{
				Role:    TurnRole(""),
				Content: "Test",
				Tokens:  1,
			},
			wantErr: true,
			errMsg:  "role",
		},
		{
			name: "invalid role",
			turn: Turn{
				Role:    TurnRole("invalid"),
				Content: "Test",
				Tokens:  1,
			},
			wantErr: true,
			errMsg:  "role",
		},
		{
			name: "empty content",
			turn: Turn{
				Role:    TurnRoleUser,
				Content: "",
				Tokens:  0,
			},
			wantErr: true,
			errMsg:  "content",
		},
		{
			name: "negative tokens",
			turn: Turn{
				Role:    TurnRoleUser,
				Content: "Test",
				Tokens:  -1,
			},
			wantErr: true,
			errMsg:  "tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.turn.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestContextWindowStrategy_Constants(t *testing.T) {
	assert.Equal(t, ContextWindowStrategy(""), StrategyNone)
	assert.Equal(t, ContextWindowStrategy("sliding_window"), StrategySlidingWindow)
	assert.Equal(t, ContextWindowStrategy("summarize"), StrategySummarize)
	assert.Equal(t, ContextWindowStrategy("truncate_middle"), StrategyTruncateMiddle)
}

func TestConversationConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ConversationConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid minimal config",
			config: ConversationConfig{
				MaxTurns: 5,
			},
			wantErr: false,
		},
		{
			name: "valid full config",
			config: ConversationConfig{
				MaxTurns:         10,
				MaxContextTokens: 100000,
				Strategy:         StrategySlidingWindow,
				StopCondition:    "response contains 'DONE'",
			},
			wantErr: false,
		},
		{
			name: "valid with continue_from",
			config: ConversationConfig{
				MaxTurns:     5,
				ContinueFrom: "previous_conversation",
			},
			wantErr: false,
		},
		{
			name: "valid with inject_context",
			config: ConversationConfig{
				MaxTurns:      5,
				InjectContext: "Additional context here",
			},
			wantErr: false,
		},
		{
			name: "zero max_turns (uses default)",
			config: ConversationConfig{
				MaxTurns: 0,
			},
			wantErr: false,
		},
		{
			name: "negative max_turns",
			config: ConversationConfig{
				MaxTurns: -1,
			},
			wantErr: true,
			errMsg:  "max_turns",
		},
		{
			name: "max_turns exceeds limit",
			config: ConversationConfig{
				MaxTurns: 101,
			},
			wantErr: true,
			errMsg:  "max_turns",
		},
		{
			name: "negative max_context_tokens",
			config: ConversationConfig{
				MaxTurns:         5,
				MaxContextTokens: -1,
			},
			wantErr: true,
			errMsg:  "max_context_tokens",
		},
		{
			name: "invalid strategy",
			config: ConversationConfig{
				MaxTurns: 5,
				Strategy: ContextWindowStrategy("invalid"),
			},
			wantErr: true,
			errMsg:  "strategy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConversationConfig_Validate_Strategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy ContextWindowStrategy
		wantErr  bool
		errMsg   string
	}{
		{"none (default)", StrategyNone, false, ""},
		{"sliding_window", StrategySlidingWindow, false, ""},
		{"summarize", StrategySummarize, true, "not yet implemented"},
		{"truncate_middle", StrategyTruncateMiddle, true, "not yet implemented"},
		{"empty string", ContextWindowStrategy(""), false, ""},
		{"invalid", ContextWindowStrategy("invalid"), true, "invalid"},
		{"typo", ContextWindowStrategy("sliding_windows"), true, "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ConversationConfig{
				MaxTurns: 5,
				Strategy: tt.strategy,
			}
			err := config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConversationConfig_Validate_StopConditions(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		wantErr   bool
	}{
		{
			name:      "simple contains",
			condition: "response contains 'DONE'",
			wantErr:   false,
		},
		{
			name:      "turn count comparison",
			condition: "turn_count >= 5",
			wantErr:   false,
		},
		{
			name:      "token comparison",
			condition: "total_tokens > 50000",
			wantErr:   false,
		},
		{
			name:      "logical AND",
			condition: "turn_count >= 3 && response contains 'APPROVED'",
			wantErr:   false,
		},
		{
			name:      "empty condition (no early exit)",
			condition: "",
			wantErr:   false,
		},
		{
			name:      "complex expression",
			condition: "(turn_count >= 5 || total_tokens > 10000) && response contains 'COMPLETE'",
			wantErr:   false,
		},
		{
			name:      "invalid syntax",
			condition: "response contains DONE'",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ConversationConfig{
				MaxTurns:      5,
				StopCondition: tt.condition,
			}
			// Use mock validator for stop condition syntax checking
			err := config.Validate(mockExpressionValidator)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConversationConfig_GetMaxTurns(t *testing.T) {
	tests := []struct {
		name     string
		maxTurns int
		expected int
	}{
		{
			name:     "zero returns default (10)",
			maxTurns: 0,
			expected: 10,
		},
		{
			name:     "positive returns configured value",
			maxTurns: 5,
			expected: 5,
		},
		{
			name:     "maximum allowed (100)",
			maxTurns: 100,
			expected: 100,
		},
		{
			name:     "exactly default value",
			maxTurns: 10,
			expected: 10,
		},
		{
			name:     "minimum (1)",
			maxTurns: 1,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ConversationConfig{
				MaxTurns: tt.maxTurns,
			}
			assert.Equal(t, tt.expected, config.GetMaxTurns())
		})
	}
}

func TestStopReason_Constants(t *testing.T) {
	assert.Equal(t, StopReason("condition"), StopReasonCondition)
	assert.Equal(t, StopReason("max_turns"), StopReasonMaxTurns)
	assert.Equal(t, StopReason("max_tokens"), StopReasonMaxTokens)
	assert.Equal(t, StopReason("error"), StopReasonError)
}

func TestNewConversationState(t *testing.T) {
	tests := []struct {
		name         string
		systemPrompt string
	}{
		{
			name:         "with system prompt",
			systemPrompt: "You are a helpful assistant.",
		},
		{
			name:         "empty system prompt",
			systemPrompt: "",
		},
		{
			name:         "multiline system prompt",
			systemPrompt: "You are a code reviewer.\nBe thorough and constructive.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewConversationState(tt.systemPrompt)

			require.NotNil(t, state)
			assert.NotNil(t, state.Turns)
			assert.Empty(t, state.StoppedBy)

			if tt.systemPrompt != "" {
				// If system prompt provided, expect first turn to be system
				assert.GreaterOrEqual(t, len(state.Turns), 1)
				assert.Equal(t, 1, state.TotalTurns)
				assert.GreaterOrEqual(t, state.TotalTokens, 0)
				if len(state.Turns) > 0 {
					assert.Equal(t, TurnRoleSystem, state.Turns[0].Role)
					assert.Equal(t, tt.systemPrompt, state.Turns[0].Content)
				}
			} else {
				// Empty system prompt - no turns
				assert.Equal(t, 0, state.TotalTurns)
				assert.Equal(t, 0, state.TotalTokens)
			}
		})
	}
}

func TestConversationState_AddTurn(t *testing.T) {
	state := NewConversationState("You are a helper.")

	// Add user turn
	userTurn := &Turn{
		Role:    TurnRoleUser,
		Content: "Hello",
		Tokens:  5,
	}
	err := state.AddTurn(userTurn)
	assert.NoError(t, err)

	// Add assistant turn
	assistantTurn := &Turn{
		Role:    TurnRoleAssistant,
		Content: "Hi there!",
		Tokens:  10,
	}
	err = state.AddTurn(assistantTurn)
	assert.NoError(t, err)

	// Verify turns are added
	assert.GreaterOrEqual(t, len(state.Turns), 2)
	assert.GreaterOrEqual(t, state.TotalTurns, 2)
	assert.GreaterOrEqual(t, state.TotalTokens, 15)
}

func TestConversationState_AddTurn_NilTurn(t *testing.T) {
	state := NewConversationState("")

	// Adding nil turn should return error
	err := state.AddTurn(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot add nil turn")

	// State should be unchanged
	assert.Empty(t, state.Turns)
	assert.Equal(t, 0, state.TotalTurns)
}

func TestConversationState_GetTotalTokens(t *testing.T) {
	state := &ConversationState{
		Turns: []Turn{
			{Role: TurnRoleSystem, Content: "System", Tokens: 10},
			{Role: TurnRoleUser, Content: "User", Tokens: 20},
			{Role: TurnRoleAssistant, Content: "Assistant", Tokens: 30},
		},
		TotalTokens: 60,
	}

	assert.Equal(t, 60, state.GetTotalTokens())
}

func TestConversationState_GetTotalTokens_Empty(t *testing.T) {
	state := &ConversationState{}
	assert.Equal(t, 0, state.GetTotalTokens())
}

func TestConversationState_GetLastTurn(t *testing.T) {
	state := &ConversationState{
		Turns: []Turn{
			{Role: TurnRoleSystem, Content: "System", Tokens: 10},
			{Role: TurnRoleUser, Content: "User", Tokens: 20},
			{Role: TurnRoleAssistant, Content: "Last turn", Tokens: 30},
		},
	}

	lastTurn := state.GetLastTurn()
	require.NotNil(t, lastTurn)
	assert.Equal(t, TurnRoleAssistant, lastTurn.Role)
	assert.Equal(t, "Last turn", lastTurn.Content)
	assert.Equal(t, 30, lastTurn.Tokens)
}

func TestConversationState_GetLastTurn_Empty(t *testing.T) {
	state := &ConversationState{
		Turns: []Turn{},
	}

	lastTurn := state.GetLastTurn()
	assert.Nil(t, lastTurn)
}

func TestConversationState_GetLastAssistantResponse(t *testing.T) {
	state := &ConversationState{
		Turns: []Turn{
			{Role: TurnRoleSystem, Content: "System", Tokens: 10},
			{Role: TurnRoleUser, Content: "Question?", Tokens: 20},
			{Role: TurnRoleAssistant, Content: "Answer here", Tokens: 30},
			{Role: TurnRoleUser, Content: "Follow-up?", Tokens: 15},
			{Role: TurnRoleAssistant, Content: "Final response", Tokens: 25},
		},
	}

	response := state.GetLastAssistantResponse()
	assert.Equal(t, "Final response", response)
}

func TestConversationState_GetLastAssistantResponse_NoAssistant(t *testing.T) {
	state := &ConversationState{
		Turns: []Turn{
			{Role: TurnRoleSystem, Content: "System", Tokens: 10},
			{Role: TurnRoleUser, Content: "Question?", Tokens: 20},
		},
	}

	response := state.GetLastAssistantResponse()
	assert.Empty(t, response)
}

func TestConversationState_IsStopped(t *testing.T) {
	tests := []struct {
		name      string
		stoppedBy StopReason
		expected  bool
	}{
		{"stopped by condition", StopReasonCondition, true},
		{"stopped by max turns", StopReasonMaxTurns, true},
		{"stopped by max tokens", StopReasonMaxTokens, true},
		{"stopped by error", StopReasonError, true},
		{"not stopped (empty)", StopReason(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &ConversationState{
				StoppedBy: tt.stoppedBy,
			}
			assert.Equal(t, tt.expected, state.IsStopped())
		})
	}
}

func TestNewConversationResult(t *testing.T) {
	provider := "claude"

	result := NewConversationResult(provider)

	require.NotNil(t, result)
	assert.Equal(t, provider, result.Provider)
	assert.NotNil(t, result.State)
	assert.Empty(t, result.Output)
	assert.NotNil(t, result.Response)
	assert.Empty(t, result.Response)
	assert.Equal(t, 0, result.TokensInput)
	assert.Equal(t, 0, result.TokensOutput)
	assert.Equal(t, 0, result.TokensTotal)
	assert.False(t, result.TokensEstimated)
	assert.Nil(t, result.Error)
	assert.False(t, result.StartedAt.IsZero())
	assert.True(t, result.CompletedAt.IsZero())
}

func TestNewConversationResult_EmptyProvider(t *testing.T) {
	result := NewConversationResult("")

	require.NotNil(t, result)
	assert.Equal(t, "", result.Provider)
	assert.NotNil(t, result.Response)
}

func TestNewConversationResult_VariousProviders(t *testing.T) {
	providers := []string{
		"claude",
		"codex",
		"gemini",
		"opencode",
		"custom",
	}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			result := NewConversationResult(provider)
			require.NotNil(t, result)
			assert.Equal(t, provider, result.Provider)
		})
	}
}

func TestConversationResult_Duration(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(10*time.Second + 500*time.Millisecond)

	result := ConversationResult{
		Provider:    "claude",
		StartedAt:   start,
		CompletedAt: end,
	}

	expected := 10*time.Second + 500*time.Millisecond
	assert.Equal(t, expected, result.Duration())
}

func TestConversationResult_Duration_ZeroTime(t *testing.T) {
	result := ConversationResult{}
	assert.Equal(t, time.Duration(0), result.Duration())
}

func TestConversationResult_Duration_NotCompleted(t *testing.T) {
	result := NewConversationResult("claude")
	// CompletedAt is zero, so duration should be 0
	duration := result.Duration()
	assert.Equal(t, time.Duration(0), duration)
}

func TestConversationResult_Duration_Instant(t *testing.T) {
	now := time.Now()
	result := ConversationResult{
		StartedAt:   now,
		CompletedAt: now,
	}
	assert.Equal(t, time.Duration(0), result.Duration())
}

func TestConversationResult_Success(t *testing.T) {
	tests := []struct {
		name     string
		result   ConversationResult
		expected bool
	}{
		{
			name: "success with nil error",
			result: ConversationResult{
				Provider: "claude",
				Output:   "Conversation complete",
				Error:    nil,
			},
			expected: true,
		},
		{
			name: "failure with error",
			result: ConversationResult{
				Provider: "claude",
				Error:    errors.New("execution failed"),
			},
			expected: false,
		},
		{
			name: "failure with timeout error",
			result: ConversationResult{
				Provider: "codex",
				Error:    errors.New("timeout: conversation exceeded 300s"),
			},
			expected: false,
		},
		{
			name:     "empty result",
			result:   ConversationResult{},
			expected: true, // nil error = success
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.Success())
		})
	}
}

func TestConversationResult_HasJSONResponse(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		expected bool
	}{
		{
			name:     "empty response map",
			response: map[string]any{},
			expected: false,
		},
		{
			name:     "nil response",
			response: nil,
			expected: false,
		},
		{
			name: "single key response",
			response: map[string]any{
				"result": "complete",
			},
			expected: true,
		},
		{
			name: "multiple keys response",
			response: map[string]any{
				"result": "complete",
				"turns":  5,
				"tokens": 1000,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConversationResult{
				Response: tt.response,
			}
			assert.Equal(t, tt.expected, result.HasJSONResponse())
		})
	}
}

func TestConversationResult_TurnCount(t *testing.T) {
	tests := []struct {
		name     string
		state    *ConversationState
		expected int
	}{
		{
			name: "three turns",
			state: &ConversationState{
				Turns: []Turn{
					{Role: TurnRoleSystem, Content: "System"},
					{Role: TurnRoleUser, Content: "User"},
					{Role: TurnRoleAssistant, Content: "Assistant"},
				},
				TotalTurns: 3,
			},
			expected: 3,
		},
		{
			name: "zero turns",
			state: &ConversationState{
				Turns:      []Turn{},
				TotalTurns: 0,
			},
			expected: 0,
		},
		{
			name:     "nil state",
			state:    nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConversationResult{
				State: tt.state,
			}
			assert.Equal(t, tt.expected, result.TurnCount())
		})
	}
}

func TestConversationConfig_CompleteExample(t *testing.T) {
	config := ConversationConfig{
		MaxTurns:         10,
		MaxContextTokens: 100000,
		Strategy:         StrategySlidingWindow,
		StopCondition:    "response contains 'APPROVED'",
	}

	// Validate structure
	err := config.Validate(nil)
	require.NoError(t, err)

	// Check field values
	assert.Equal(t, 10, config.MaxTurns)
	assert.Equal(t, 100000, config.MaxContextTokens)
	assert.Equal(t, StrategySlidingWindow, config.Strategy)
	assert.Equal(t, "response contains 'APPROVED'", config.StopCondition)
	assert.Equal(t, 10, config.GetMaxTurns())
}

func TestConversationResult_ExecutionLifecycle(t *testing.T) {
	// Simulate a complete conversation execution lifecycle

	// Start execution
	result := NewConversationResult("claude")
	assert.Equal(t, "claude", result.Provider)
	assert.False(t, result.StartedAt.IsZero())
	assert.True(t, result.CompletedAt.IsZero())

	// Initialize conversation state
	result.State = NewConversationState("You are a code reviewer.")

	// Simulate turn 1 (user)
	userTurn1 := &Turn{
		Role:    TurnRoleUser,
		Content: "Review this code",
		Tokens:  50,
	}
	_ = result.State.AddTurn(userTurn1)

	// Simulate turn 2 (assistant)
	assistantTurn1 := &Turn{
		Role:    TurnRoleAssistant,
		Content: "I found 3 issues",
		Tokens:  100,
	}
	_ = result.State.AddTurn(assistantTurn1)

	// Simulate turn 3 (user)
	userTurn2 := &Turn{
		Role:    TurnRoleUser,
		Content: "Fix them",
		Tokens:  20,
	}
	_ = result.State.AddTurn(userTurn2)

	// Simulate turn 4 (assistant - final)
	assistantTurn2 := &Turn{
		Role:    TurnRoleAssistant,
		Content: "Fixed. APPROVED",
		Tokens:  80,
	}
	_ = result.State.AddTurn(assistantTurn2)

	// Mark conversation as stopped
	result.State.StoppedBy = StopReasonCondition

	// Capture final output
	result.Output = "Fixed. APPROVED"
	result.TokensInput = 70
	result.TokensOutput = 180
	result.TokensTotal = 250

	// Parse JSON response (if any)
	if result.Response == nil {
		result.Response = make(map[string]any)
	}
	result.Response["status"] = "approved"
	result.Response["issues_fixed"] = 3

	// Complete execution
	result.CompletedAt = time.Now()

	// Verify final state
	assert.True(t, result.Success())
	assert.Greater(t, result.Duration(), time.Duration(0))
	assert.NotEmpty(t, result.Output)
	assert.True(t, result.HasJSONResponse())
	assert.GreaterOrEqual(t, result.TurnCount(), 4)
	assert.True(t, result.State.IsStopped())
	assert.Equal(t, StopReasonCondition, result.State.StoppedBy)
	assert.Greater(t, result.TokensTotal, 0)
}

func TestConversationResult_FailedExecution(t *testing.T) {
	// Simulate a failed conversation execution

	result := NewConversationResult("codex")

	// Simulate execution that fails
	result.Error = errors.New("codex: executable file not found in $PATH")
	result.CompletedAt = time.Now()

	// Verify failure state
	assert.False(t, result.Success())
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "not found")
	assert.False(t, result.HasJSONResponse())
	assert.Equal(t, 0, result.TurnCount())
}

func TestConversationResult_JSONParseSuccess(t *testing.T) {
	// Simulate successful JSON parsing

	result := NewConversationResult("claude")

	// Initialize state
	result.State = NewConversationState("System")

	// Add turns
	_ = result.State.AddTurn(&Turn{
		Role:    TurnRoleUser,
		Content: "Analyze",
		Tokens:  10,
	})
	_ = result.State.AddTurn(&Turn{
		Role:    TurnRoleAssistant,
		Content: `{"analysis": "complete", "score": 95}`,
		Tokens:  20,
	})

	// Raw JSON output
	result.Output = `{"analysis": "complete", "score": 95}`

	// Parsed response
	if result.Response == nil {
		result.Response = make(map[string]any)
	}
	result.Response["analysis"] = "complete"
	result.Response["score"] = 95

	result.TokensTotal = 30
	result.CompletedAt = time.Now()

	// Verify state
	assert.True(t, result.Success())
	assert.True(t, result.HasJSONResponse())
	assert.Len(t, result.Response, 2)
	assert.Equal(t, "complete", result.Response["analysis"])
	assert.Equal(t, 95, result.Response["score"])
}

func TestConversationResult_TextOnlyResponse(t *testing.T) {
	// Simulate text-only response (no JSON)

	result := NewConversationResult("opencode")

	// Initialize state
	result.State = NewConversationState("System")
	_ = result.State.AddTurn(&Turn{
		Role:    TurnRoleUser,
		Content: "Review",
		Tokens:  5,
	})
	_ = result.State.AddTurn(&Turn{
		Role:    TurnRoleAssistant,
		Content: "Looks good. No issues.",
		Tokens:  15,
	})

	result.Output = "Looks good. No issues."
	result.TokensTotal = 20
	result.CompletedAt = time.Now()

	// Response map is empty (no JSON parsed)
	assert.True(t, result.Success())
	assert.False(t, result.HasJSONResponse())
	assert.Empty(t, result.Response)
	assert.NotEmpty(t, result.Output)
	assert.Equal(t, 3, result.TurnCount()) // System + User + Assistant
}

func TestConversationConfig_MaxTurnsBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		maxTurns int
		expected int
		wantErr  bool
	}{
		{"minimum valid (1)", 1, 1, false},
		{"zero (uses default)", 0, 10, false},
		{"default (10)", 10, 10, false},
		{"high valid (50)", 50, 50, false},
		{"maximum valid (100)", 100, 100, false},
		{"exceeds max (101)", 101, 0, true},
		{"negative (-1)", -1, 0, true},
		{"large negative", -9999, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ConversationConfig{
				MaxTurns: tt.maxTurns,
			}
			err := config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, config.GetMaxTurns())
			}
		})
	}
}

func TestConversationConfig_MaxContextTokensBoundaries(t *testing.T) {
	tests := []struct {
		name             string
		maxContextTokens int
		wantErr          bool
	}{
		{"zero (provider default)", 0, false},
		{"small valid (1000)", 1000, false},
		{"medium valid (100000)", 100000, false},
		{"large valid (1000000)", 1000000, false},
		{"negative (-1)", -1, true},
		{"large negative", -50000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ConversationConfig{
				MaxTurns:         5,
				MaxContextTokens: tt.maxContextTokens,
			}
			err := config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConversationState_LargeConversation(t *testing.T) {
	state := NewConversationState("System prompt")

	// Add many turns
	for i := 0; i < 50; i++ {
		if i%2 == 0 {
			_ = state.AddTurn(&Turn{
				Role:    TurnRoleUser,
				Content: "Question",
				Tokens:  10,
			})
		} else {
			_ = state.AddTurn(&Turn{
				Role:    TurnRoleAssistant,
				Content: "Answer",
				Tokens:  20,
			})
		}
	}

	assert.GreaterOrEqual(t, len(state.Turns), 50)
	assert.GreaterOrEqual(t, state.TotalTurns, 50)
	assert.Greater(t, state.TotalTokens, 0)
}

func TestConversationResult_ResponseTypes(t *testing.T) {
	result := NewConversationResult("claude")

	// Test various response types
	if result.Response == nil {
		result.Response = make(map[string]any)
	}
	result.Response["string"] = "hello"
	result.Response["int"] = 42
	result.Response["float"] = 3.14
	result.Response["bool"] = true
	result.Response["slice"] = []string{"a", "b", "c"}
	result.Response["map"] = map[string]any{"key": "value"}
	result.Response["nil"] = nil

	assert.Len(t, result.Response, 7)
	assert.IsType(t, "", result.Response["string"])
	assert.IsType(t, 0, result.Response["int"])
	assert.IsType(t, 0.0, result.Response["float"])
	assert.IsType(t, false, result.Response["bool"])
	assert.IsType(t, []string{}, result.Response["slice"])
	assert.IsType(t, map[string]any{}, result.Response["map"])
	assert.Nil(t, result.Response["nil"])
	assert.True(t, result.HasJSONResponse())
}

func TestConversationResult_TokenAccounting(t *testing.T) {
	result := ConversationResult{
		Provider:        "claude",
		TokensInput:     1000,
		TokensOutput:    500,
		TokensTotal:     1500,
		TokensEstimated: false,
	}

	assert.Equal(t, 1000, result.TokensInput)
	assert.Equal(t, 500, result.TokensOutput)
	assert.Equal(t, 1500, result.TokensTotal)
	assert.False(t, result.TokensEstimated)
}

func TestConversationResult_EstimatedTokens(t *testing.T) {
	result := ConversationResult{
		Provider:        "custom",
		TokensTotal:     800,
		TokensEstimated: true,
	}

	assert.Equal(t, 800, result.TokensTotal)
	assert.True(t, result.TokensEstimated)
}

// F073: SessionID field tests

// TestConversationState_SessionID_Assignment verifies SessionID can be set and retrieved
func TestConversationState_SessionID_Assignment(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
	}{
		{
			name:      "empty session id",
			sessionID: "",
		},
		{
			name:      "uuid format session id",
			sessionID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:      "numeric session id",
			sessionID: "12345",
		},
		{
			name:      "alphanumeric session id",
			sessionID: "sess_abc123xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewConversationState("")
			state.SessionID = tt.sessionID
			assert.Equal(t, tt.sessionID, state.SessionID)
		})
	}
}

// TestConversationState_SessionID_JSONRoundTrip verifies SessionID survives JSON marshal/unmarshal
// with PascalCase naming (matching existing ConversationState fields)
func TestConversationState_SessionID_JSONRoundTrip(t *testing.T) {
	originalState := NewConversationState("")
	originalState.SessionID = "sess_12345"

	// Add a turn to verify SessionID survives alongside other state
	turn := NewTurn(TurnRoleUser, "test prompt")
	turn.Tokens = 10
	require.NoError(t, originalState.AddTurn(turn))

	// Marshal to JSON
	data, err := json.Marshal(originalState)
	require.NoError(t, err)

	// Verify JSON uses PascalCase field names (not snake_case)
	var jsonObj map[string]interface{}
	err = json.Unmarshal(data, &jsonObj)
	require.NoError(t, err)

	// Check that SessionID is present with PascalCase name
	assert.Contains(t, jsonObj, "SessionID", "JSON should use PascalCase SessionID field name")
	assert.NotContains(t, jsonObj, "session_id", "JSON should not use snake_case")

	// Unmarshal back to ConversationState
	var restoredState ConversationState
	err = json.Unmarshal(data, &restoredState)
	require.NoError(t, err)

	// Verify SessionID survived round-trip
	assert.Equal(t, originalState.SessionID, restoredState.SessionID)

	// Verify other fields survived too
	assert.Equal(t, originalState.TotalTurns, restoredState.TotalTurns)
	assert.Equal(t, originalState.TotalTokens, restoredState.TotalTokens)
}

// TestConversationState_SessionID_InitialValue verifies SessionID starts empty
func TestConversationState_SessionID_InitialValue(t *testing.T) {
	state := NewConversationState("")
	assert.Equal(t, "", state.SessionID, "SessionID should be empty when state is created")
}

// TestConversationState_SessionID_JSONSerializationFormat verifies the exact JSON field name
func TestConversationState_SessionID_JSONSerializationFormat(t *testing.T) {
	state := NewConversationState("")
	state.SessionID = "test-session-id"

	data, err := json.Marshal(state)
	require.NoError(t, err)

	// Verify JSON contains the exact string for SessionID field
	jsonStr := string(data)
	assert.Contains(t, jsonStr, "\"SessionID\"", "JSON must contain quoted SessionID field name")
	assert.Contains(t, jsonStr, "test-session-id", "JSON must contain the SessionID value")
}

// C069: Tests for StepTypeChecker function type

func TestStepTypeChecker_Basic(t *testing.T) {
	tests := []struct {
		name      string
		checker   StepTypeChecker
		typeName  string
		wantFound bool
	}{
		{
			name: "checker returns true for known type",
			checker: func(typeName string) bool {
				return typeName == "custom-step"
			},
			typeName:  "custom-step",
			wantFound: true,
		},
		{
			name: "checker returns false for unknown type",
			checker: func(typeName string) bool {
				return typeName == "custom-step"
			},
			typeName:  "unknown-step",
			wantFound: false,
		},
		{
			name: "checker with empty type name",
			checker: func(typeName string) bool {
				return typeName != ""
			},
			typeName:  "",
			wantFound: false,
		},
		{
			name: "checker accepts multiple types",
			checker: func(typeName string) bool {
				return typeName == "type-a" || typeName == "type-b" || typeName == "type-c"
			},
			typeName:  "type-b",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.checker(tt.typeName)
			assert.Equal(t, tt.wantFound, result)
		})
	}
}

func TestStepTypeChecker_CaseInsensitive(t *testing.T) {
	checker := func(typeName string) bool {
		normalized := strings.ToLower(typeName)
		return normalized == "custom-validator" || normalized == "custom-processor"
	}

	assert.True(t, checker("custom-validator"))
	assert.True(t, checker("CUSTOM-VALIDATOR"))
	assert.True(t, checker("Custom-Validator"))
	assert.False(t, checker("unknown-validator"))
}

func TestStepTypeChecker_MapBased(t *testing.T) {
	registeredTypes := map[string]bool{
		"validator":   true,
		"processor":   true,
		"transformer": true,
	}

	checker := func(typeName string) bool {
		_, exists := registeredTypes[typeName]
		return exists
	}

	assert.True(t, checker("validator"))
	assert.True(t, checker("processor"))
	assert.False(t, checker("unknown"))
}

func TestStepTypeChecker_PrefixMatching(t *testing.T) {
	checker := func(typeName string) bool {
		return strings.HasPrefix(typeName, "awf-plugin-")
	}

	assert.True(t, checker("awf-plugin-custom"))
	assert.True(t, checker("awf-plugin-validator"))
	assert.False(t, checker("custom-plugin"))
	assert.False(t, checker("awf-other"))
}

func TestStepTypeChecker_Callable(t *testing.T) {
	checker := func(typeName string) bool {
		return typeName == "test-type"
	}

	assert.NotNil(t, checker)
	assert.True(t, checker("test-type"))
	assert.False(t, checker("other-type"))
}

func TestStepTypeChecker_CanBeNil(t *testing.T) {
	var checker StepTypeChecker
	assert.Nil(t, checker)
	// A nil function type won't execute, but can be safely assigned
	// This validates the function type can represent uninitialized state
}

func TestStepTypeChecker_MultipleInstances(t *testing.T) {
	checker1 := func(typeName string) bool {
		return typeName == "type-a"
	}

	checker2 := func(typeName string) bool {
		return typeName == "type-b"
	}

	assert.True(t, checker1("type-a"))
	assert.False(t, checker1("type-b"))
	assert.False(t, checker2("type-a"))
	assert.True(t, checker2("type-b"))
}

func TestStepTypeChecker_WithRegularExpression(t *testing.T) {
	checker := func(typeName string) bool {
		matched, _ := regexp.MatchString(`^[a-z]+-[a-z]+$`, typeName)
		return matched
	}

	assert.True(t, checker("custom-step"))
	assert.True(t, checker("my-validator"))
	assert.False(t, checker("InvalidFormat"))
	assert.False(t, checker("123"))
}

func TestStepTypeChecker_WithLength(t *testing.T) {
	checker := func(typeName string) bool {
		return typeName != "" && len(typeName) <= 50
	}

	assert.True(t, checker("short"))
	assert.True(t, checker("this-is-a-medium-length-type-name"))
	assert.False(t, checker(""))
	assert.False(t, checker(strings.Repeat("x", 51)))
}
