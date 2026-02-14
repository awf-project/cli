// Component: T009 - Agent Config Result Tests
// Feature: C013 - Domain Test File Splitting
// Source: agent_config_test.go (lines 410-940, 1581-1695)
// Test count: 22 tests
// Scope: AgentResult creation, duration calculation, success status, JSON response parsing, token handling, conversation field

package workflow_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// workflow.NewAgentResult Constructor Tests

func TestNewAgentResult(t *testing.T) {
	provider := "claude"

	result := workflow.NewAgentResult(provider)

	require.NotNil(t, result)
	assert.Equal(t, provider, result.Provider)
	assert.Empty(t, result.Output)
	assert.NotNil(t, result.Response)
	assert.Empty(t, result.Response)
	assert.Equal(t, 0, result.Tokens)
	assert.Nil(t, result.Error)
	assert.False(t, result.StartedAt.IsZero())
	assert.True(t, result.CompletedAt.IsZero())
}

func TestNewAgentResult_EmptyProvider(t *testing.T) {
	result := workflow.NewAgentResult("")

	require.NotNil(t, result)
	assert.Equal(t, "", result.Provider)
	assert.NotNil(t, result.Response)
}

func TestNewAgentResult_VariousProviders(t *testing.T) {
	providers := []string{
		"claude",
		"codex",
		"gemini",
		"opencode",
		"custom",
		"my-custom-llm",
	}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			result := workflow.NewAgentResult(provider)
			require.NotNil(t, result)
			assert.Equal(t, provider, result.Provider)
		})
	}
}

// workflow.AgentResult Duration Tests

func TestAgentResult_Duration(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(5*time.Second + 250*time.Millisecond)

	result := workflow.AgentResult{
		Provider:    "claude",
		StartedAt:   start,
		CompletedAt: end,
	}

	expected := 5*time.Second + 250*time.Millisecond
	assert.Equal(t, expected, result.Duration())
}

func TestAgentResult_Duration_ZeroTime(t *testing.T) {
	result := workflow.AgentResult{}
	assert.Equal(t, time.Duration(0), result.Duration())
}

func TestAgentResult_Duration_NotCompleted(t *testing.T) {
	result := workflow.NewAgentResult("claude")
	// CompletedAt is zero, so duration is negative
	duration := result.Duration()
	assert.Less(t, duration, time.Duration(0))
}

func TestAgentResult_Duration_Instant(t *testing.T) {
	now := time.Now()
	result := workflow.AgentResult{
		StartedAt:   now,
		CompletedAt: now,
	}
	assert.Equal(t, time.Duration(0), result.Duration())
}

func TestAgentResult_Duration_LongRunning(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Minute)

	result := workflow.AgentResult{
		StartedAt:   start,
		CompletedAt: end,
	}

	assert.Equal(t, 10*time.Minute, result.Duration())
}

// workflow.AgentResult Success Tests

func TestAgentResult_Success(t *testing.T) {
	tests := []struct {
		name     string
		result   workflow.AgentResult
		expected bool
	}{
		{
			name: "success with nil error",
			result: workflow.AgentResult{
				Provider: "claude",
				Output:   "Analysis complete",
				Error:    nil,
			},
			expected: true,
		},
		{
			name: "failure with error",
			result: workflow.AgentResult{
				Provider: "claude",
				Error:    errors.New("execution failed"),
			},
			expected: false,
		},
		{
			name: "failure with timeout error",
			result: workflow.AgentResult{
				Provider: "codex",
				Error:    errors.New("timeout: agent exceeded 300s"),
			},
			expected: false,
		},
		{
			name: "failure with CLI not found",
			result: workflow.AgentResult{
				Provider: "gemini",
				Error:    errors.New("gemini: executable file not found in $PATH"),
			},
			expected: false,
		},
		{
			name:     "empty result",
			result:   workflow.AgentResult{},
			expected: true, // nil error = success
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.Success())
		})
	}
}

// workflow.AgentResult HasJSONResponse Tests

func TestAgentResult_HasJSONResponse(t *testing.T) {
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
				"result": "analysis",
			},
			expected: true,
		},
		{
			name: "multiple keys response",
			response: map[string]any{
				"result": "analysis",
				"count":  42,
				"items":  []string{"a", "b"},
			},
			expected: true,
		},
		{
			name: "response with nil value",
			response: map[string]any{
				"result": nil,
			},
			expected: true, // has key, even if value is nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := workflow.AgentResult{
				Response: tt.response,
			}
			assert.Equal(t, tt.expected, result.HasJSONResponse())
		})
	}
}

// workflow.AgentResult Output Tests

func TestAgentResult_Output(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "simple text",
			output: "Analysis complete",
		},
		{
			name:   "multiline text",
			output: "Line 1\nLine 2\nLine 3",
		},
		{
			name:   "JSON string",
			output: `{"result": "success", "count": 42}`,
		},
		{
			name:   "empty output",
			output: "",
		},
		{
			name:   "large output",
			output: string(make([]byte, 100000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := workflow.AgentResult{
				Provider: "claude",
				Output:   tt.output,
			}
			assert.Equal(t, tt.output, result.Output)
		})
	}
}

// workflow.AgentResult Response Tests

func TestAgentResult_Response(t *testing.T) {
	result := workflow.NewAgentResult("claude")

	// Add parsed JSON response
	result.Response["result"] = "analysis completed"
	result.Response["count"] = 42
	result.Response["items"] = []string{"a", "b", "c"}
	result.Response["metadata"] = map[string]any{
		"duration": 1.5,
		"success":  true,
	}

	assert.Len(t, result.Response, 4)
	assert.Equal(t, "analysis completed", result.Response["result"])
	assert.Equal(t, 42, result.Response["count"])
	assert.Equal(t, []string{"a", "b", "c"}, result.Response["items"])
	assert.True(t, result.HasJSONResponse())
}

func TestAgentResult_Response_NilValue(t *testing.T) {
	result := workflow.NewAgentResult("claude")
	result.Response["empty"] = nil

	assert.Len(t, result.Response, 1)
	assert.Nil(t, result.Response["empty"])
	assert.True(t, result.HasJSONResponse())
}

// workflow.AgentResult Tokens Tests

func TestAgentResult_Tokens(t *testing.T) {
	tests := []struct {
		name   string
		tokens int
	}{
		{"zero tokens", 0},
		{"small usage", 100},
		{"medium usage", 4096},
		{"large usage", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := workflow.AgentResult{
				Provider: "claude",
				Tokens:   tt.tokens,
			}
			assert.Equal(t, tt.tokens, result.Tokens)
		})
	}
}

// workflow.AgentResult Fields Tests

func TestAgentResult_AllFields(t *testing.T) {
	err := errors.New("timeout exceeded")
	start := time.Now()
	end := start.Add(5 * time.Second)

	result := workflow.AgentResult{
		Provider: "claude",
		Output:   "Partial analysis",
		Response: map[string]any{
			"status": "timeout",
		},
		Tokens:      2048,
		Error:       err,
		StartedAt:   start,
		CompletedAt: end,
	}

	assert.Equal(t, "claude", result.Provider)
	assert.Equal(t, "Partial analysis", result.Output)
	assert.Len(t, result.Response, 1)
	assert.Equal(t, 2048, result.Tokens)
	assert.Equal(t, err, result.Error)
	assert.Equal(t, start, result.StartedAt)
	assert.Equal(t, end, result.CompletedAt)
	assert.False(t, result.Success())
	assert.True(t, result.HasJSONResponse())
	assert.Equal(t, 5*time.Second, result.Duration())
}

func TestAgentResult_ExecutionLifecycle(t *testing.T) {
	// Simulate a complete agent execution lifecycle

	// Start execution
	result := workflow.NewAgentResult("claude")
	assert.Equal(t, "claude", result.Provider)
	assert.False(t, result.StartedAt.IsZero())
	assert.True(t, result.CompletedAt.IsZero())

	// Simulate agent processing
	time.Sleep(10 * time.Millisecond)

	// Capture output
	result.Output = "Security analysis complete. Found 3 issues."

	// Parse JSON response
	result.Response["issues_found"] = 3
	result.Response["severity"] = "medium"
	result.Response["recommendations"] = []string{"Fix XSS", "Add CSRF token", "Validate input"}

	// Record token usage
	result.Tokens = 2048

	// Complete execution
	result.CompletedAt = time.Now()

	// Verify final state
	assert.True(t, result.Success())
	assert.Greater(t, result.Duration(), time.Duration(0))
	assert.NotEmpty(t, result.Output)
	assert.True(t, result.HasJSONResponse())
	assert.Len(t, result.Response, 3)
	assert.Equal(t, 3, result.Response["issues_found"])
	assert.Greater(t, result.Tokens, 0)
}

func TestAgentResult_FailedExecution(t *testing.T) {
	// Simulate a failed agent execution

	result := workflow.NewAgentResult("codex")

	// Simulate execution that fails
	result.Error = errors.New("codex: executable file not found in $PATH")
	result.CompletedAt = time.Now()

	// Verify failure state
	assert.False(t, result.Success())
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "not found")
	assert.False(t, result.HasJSONResponse())
}

func TestAgentResult_JSONParseSuccess(t *testing.T) {
	// Simulate successful JSON parsing

	result := workflow.NewAgentResult("claude")

	// Raw JSON output
	result.Output = `{"analysis": "complete", "score": 95, "issues": []}`

	// Parsed response
	result.Response["analysis"] = "complete"
	result.Response["score"] = 95
	result.Response["issues"] = []any{}

	result.Tokens = 1024
	result.CompletedAt = time.Now()

	// Verify state
	assert.True(t, result.Success())
	assert.True(t, result.HasJSONResponse())
	assert.Len(t, result.Response, 3)
	assert.Equal(t, "complete", result.Response["analysis"])
	assert.Equal(t, 95, result.Response["score"])
}

func TestAgentResult_TextOnlyResponse(t *testing.T) {
	// Simulate text-only response (no JSON)

	result := workflow.NewAgentResult("opencode")

	result.Output = "The code looks good. No major issues found."
	result.Tokens = 512
	result.CompletedAt = time.Now()

	// Response map is empty (no JSON parsed)
	assert.True(t, result.Success())
	assert.False(t, result.HasJSONResponse())
	assert.Empty(t, result.Response)
	assert.NotEmpty(t, result.Output)
}

func TestAgentResult_ResponseTypes(t *testing.T) {
	result := workflow.NewAgentResult("claude")

	// Test various response types
	result.Response["string"] = "hello"
	result.Response["int"] = 42
	result.Response["float"] = 3.14
	result.Response["bool"] = true
	result.Response["slice"] = []string{"a", "b", "c"}
	result.Response["map"] = map[string]any{"key": "value"}
	result.Response["nil"] = nil
	result.Response["nested"] = map[string]any{
		"level2": map[string]any{
			"level3": "deep",
		},
	}

	assert.Len(t, result.Response, 8)
	assert.IsType(t, "", result.Response["string"])
	assert.IsType(t, 0, result.Response["int"])
	assert.IsType(t, 0.0, result.Response["float"])
	assert.IsType(t, false, result.Response["bool"])
	assert.IsType(t, []string{}, result.Response["slice"])
	assert.IsType(t, map[string]any{}, result.Response["map"])
	assert.Nil(t, result.Response["nil"])
	assert.True(t, result.HasJSONResponse())
}

// workflow.AgentResult Conversation Field Tests

func TestAgentResult_ConversationField(t *testing.T) {
	tests := []struct {
		name         string
		conversation *workflow.ConversationResult
	}{
		{
			name:         "nil conversation result",
			conversation: nil,
		},
		{
			name: "empty conversation result",
			conversation: &workflow.ConversationResult{
				Provider: "claude",
				State: &workflow.ConversationState{
					Turns:       []workflow.Turn{},
					TotalTurns:  0,
					TotalTokens: 0,
				},
			},
		},
		{
			name: "conversation with turns",
			conversation: &workflow.ConversationResult{
				Provider: "claude",
				State: &workflow.ConversationState{
					Turns: []workflow.Turn{
						{Role: workflow.TurnRoleSystem, Content: "You are helpful", Tokens: 10},
						{Role: workflow.TurnRoleUser, Content: "Hello", Tokens: 5},
						{Role: workflow.TurnRoleAssistant, Content: "Hi there", Tokens: 8},
					},
					TotalTurns:  3,
					TotalTokens: 23,
					StoppedBy:   workflow.StopReasonMaxTurns,
				},
			},
		},
		{
			name: "conversation stopped by condition",
			conversation: &workflow.ConversationResult{
				Provider: "claude",
				State: &workflow.ConversationState{
					Turns: []workflow.Turn{
						{Role: workflow.TurnRoleUser, Content: "Review", Tokens: 5},
						{Role: workflow.TurnRoleAssistant, Content: "APPROVED", Tokens: 10},
					},
					TotalTurns:  2,
					TotalTokens: 15,
					StoppedBy:   workflow.StopReasonCondition,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &workflow.AgentResult{
				Provider:     "claude",
				Output:       "Final response",
				Conversation: tt.conversation,
			}

			if tt.conversation == nil {
				assert.Nil(t, result.Conversation)
			} else {
				require.NotNil(t, result.Conversation)
				require.NotNil(t, result.Conversation.State)
				assert.Equal(t, tt.conversation.State.TotalTurns, result.Conversation.State.TotalTurns)
				assert.Equal(t, tt.conversation.State.TotalTokens, result.Conversation.State.TotalTokens)
				assert.Equal(t, tt.conversation.State.StoppedBy, result.Conversation.State.StoppedBy)
				assert.Equal(t, len(tt.conversation.State.Turns), len(result.Conversation.State.Turns))
			}
		})
	}
}

func TestAgentResult_ConversationField_Integration(t *testing.T) {
	// Simulate a complete conversation execution
	result := workflow.NewAgentResult("claude")

	// Set up conversation result
	result.Conversation = &workflow.ConversationResult{
		Provider: "claude",
		State: &workflow.ConversationState{
			Turns: []workflow.Turn{
				{Role: workflow.TurnRoleSystem, Content: "You are a code reviewer", Tokens: 50},
				{Role: workflow.TurnRoleUser, Content: "Review this code: {{code}}", Tokens: 500},
				{Role: workflow.TurnRoleAssistant, Content: "I found 3 issues...", Tokens: 800},
				{Role: workflow.TurnRoleUser, Content: "Fix the issues", Tokens: 20},
				{Role: workflow.TurnRoleAssistant, Content: "Here's the fixed code... APPROVED", Tokens: 600},
			},
			TotalTurns:  5,
			TotalTokens: 1970,
			StoppedBy:   workflow.StopReasonCondition,
		},
		Output:      "Here's the fixed code... APPROVED",
		TokensTotal: 1970,
		CompletedAt: time.Now(),
	}

	// Set overall result fields
	result.Output = "Here's the fixed code... APPROVED"
	result.Tokens = 1970
	result.CompletedAt = time.Now()

	// Verify
	assert.True(t, result.Success())
	require.NotNil(t, result.Conversation)
	require.NotNil(t, result.Conversation.State)
	assert.Equal(t, 5, result.Conversation.State.TotalTurns)
	assert.Equal(t, 1970, result.Conversation.State.TotalTokens)
	assert.Equal(t, workflow.StopReasonCondition, result.Conversation.State.StoppedBy)
	assert.Len(t, result.Conversation.State.Turns, 5)
	assert.Equal(t, workflow.TurnRoleSystem, result.Conversation.State.Turns[0].Role)
	assert.Equal(t, workflow.TurnRoleAssistant, result.Conversation.State.Turns[4].Role)
}
