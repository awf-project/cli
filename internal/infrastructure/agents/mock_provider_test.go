package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: agent_providers
// Feature: test-optimization

func TestMockProvider_Name(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
	}{
		{"claude provider", "claude"},
		{"codex provider", "codex"},
		{"custom provider", "my-agent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockProvider(tt.providerName)
			assert.Equal(t, tt.providerName, mock.Name())
		})
	}
}

func TestMockProvider_Validate_Success(t *testing.T) {
	mock := NewMockProvider("claude")

	err := mock.Validate()

	assert.NoError(t, err)
}

func TestMockProvider_Validate_Error(t *testing.T) {
	expectedErr := errors.New("CLI not found")
	mock := NewMockProvider("claude").WithValidateError(expectedErr)

	err := mock.Validate()

	assert.Equal(t, expectedErr, err)
}

func TestMockProvider_Execute_DefaultResponse(t *testing.T) {
	mock := NewMockProvider("claude")
	ctx := context.Background()

	result, err := mock.Execute(ctx, "What is 2+2?", nil)

	require.NoError(t, err)
	assert.Equal(t, "claude", result.Provider)
	assert.Contains(t, result.Output, "mock response")
	assert.Equal(t, 100, result.Tokens)
	assert.Equal(t, 1, mock.CallCount())
}

func TestMockProvider_Execute_PatternMatch(t *testing.T) {
	expected := &workflow.AgentResult{
		Provider: "claude",
		Output:   "The answer is 4",
		Tokens:   42,
	}
	mock := NewMockProvider("claude").WithResponse("2+2", expected)
	ctx := context.Background()

	result, err := mock.Execute(ctx, "What is 2+2?", nil)

	require.NoError(t, err)
	assert.Equal(t, "The answer is 4", result.Output)
	assert.Equal(t, 42, result.Tokens)
}

func TestMockProvider_Execute_MultiplePatterns(t *testing.T) {
	mathResult := &workflow.AgentResult{Output: "math answer", Tokens: 10}
	codeResult := &workflow.AgentResult{Output: "code review", Tokens: 50}

	mock := NewMockProvider("claude").
		WithResponse("calculate", mathResult).
		WithResponse("review", codeResult)

	ctx := context.Background()

	// First pattern
	r1, _ := mock.Execute(ctx, "calculate 5+5", nil)
	assert.Equal(t, "math answer", r1.Output)

	// Second pattern
	r2, _ := mock.Execute(ctx, "review this code", nil)
	assert.Equal(t, "code review", r2.Output)

	assert.Equal(t, 2, mock.CallCount())
}

func TestMockProvider_Execute_ContextCancellation(t *testing.T) {
	mock := NewMockProvider("claude").WithDelay(100 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	result, err := mock.Execute(ctx, "test", nil)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestMockProvider_Execute_RecordsCalls(t *testing.T) {
	mock := NewMockProvider("claude")
	ctx := context.Background()
	opts := map[string]any{"model": "sonnet"}

	_, _ = mock.Execute(ctx, "prompt 1", nil)
	_, _ = mock.Execute(ctx, "prompt 2", opts)

	calls := mock.Calls()
	require.Len(t, calls, 2)

	assert.Equal(t, "Execute", calls[0].Method)
	assert.Equal(t, "prompt 1", calls[0].Prompt)
	assert.Nil(t, calls[0].Options)

	assert.Equal(t, "Execute", calls[1].Method)
	assert.Equal(t, "prompt 2", calls[1].Prompt)
	assert.Equal(t, "sonnet", calls[1].Options["model"])
}

func TestMockProvider_ExecuteConversation_DefaultResponse(t *testing.T) {
	mock := NewMockProvider("claude")
	ctx := context.Background()
	state := workflow.NewConversationState("You are helpful")

	result, err := mock.ExecuteConversation(ctx, state, "Hello", nil)

	require.NoError(t, err)
	assert.Equal(t, "mock conversation response", result.Output)
	assert.Greater(t, result.TokensTotal, 0)
	assert.Equal(t, 1, mock.ConversationCallCount())
}

func TestMockProvider_ExecuteConversation_PatternMatch(t *testing.T) {
	expected := &workflow.ConversationResult{
		Output:      "Code looks good. APPROVED",
		TokensTotal: 500,
	}
	mock := NewMockProvider("claude").WithConversationResponse("review", expected)
	ctx := context.Background()
	state := workflow.NewConversationState("You are a reviewer")

	result, err := mock.ExecuteConversation(ctx, state, "Please review this", nil)

	require.NoError(t, err)
	assert.Equal(t, "Code looks good. APPROVED", result.Output)
	assert.Equal(t, 500, result.TokensTotal)
}

func TestMockProvider_ExecuteConversation_NilState(t *testing.T) {
	mock := NewMockProvider("claude")
	ctx := context.Background()

	result, err := mock.ExecuteConversation(ctx, nil, "Hello", nil)

	require.NoError(t, err)
	assert.NotNil(t, result.State)
	assert.Equal(t, 2, result.State.TotalTurns) // user + assistant
}

func TestMockProvider_Reset(t *testing.T) {
	mock := NewMockProvider("claude")
	ctx := context.Background()

	_, _ = mock.Execute(ctx, "test", nil)
	assert.Equal(t, 1, mock.CallCount())

	mock.Reset()

	assert.Equal(t, 0, mock.CallCount())
}

func TestMockProvider_CallCounters(t *testing.T) {
	mock := NewMockProvider("claude")
	ctx := context.Background()
	state := workflow.NewConversationState("")

	_, _ = mock.Execute(ctx, "exec 1", nil)
	_, _ = mock.Execute(ctx, "exec 2", nil)
	_, _ = mock.ExecuteConversation(ctx, state, "conv 1", nil)

	assert.Equal(t, 3, mock.CallCount())
	assert.Equal(t, 2, mock.ExecuteCallCount())
	assert.Equal(t, 1, mock.ConversationCallCount())
}

func TestMockProvider_WithDefaultResponse(t *testing.T) {
	defaultResult := &workflow.AgentResult{
		Provider: "claude",
		Output:   "default output",
		Tokens:   999,
	}
	mock := NewMockProvider("claude").WithDefaultResponse(defaultResult)
	ctx := context.Background()

	result, err := mock.Execute(ctx, "any prompt", nil)

	require.NoError(t, err)
	assert.Equal(t, "default output", result.Output)
	assert.Equal(t, 999, result.Tokens)
}

func TestMockProvider_ConcurrentCalls(t *testing.T) {
	mock := NewMockProvider("claude").WithDelay(10 * time.Millisecond)
	ctx := context.Background()

	// Launch concurrent calls
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = mock.Execute(ctx, "concurrent test", nil)
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, mock.CallCount())
}
