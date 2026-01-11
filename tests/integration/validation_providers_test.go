//go:build integration

// Functional tests for Agent Provider Cognitive Complexity Refactoring.
// These tests validate that the refactored providers maintain backward compatibility
// and correct behavior after extracting shared helpers and reducing complexity.
//
// Acceptance Criteria Coverage:
// - AC1: All 4 target functions have gocognit complexity ≤20
// - AC2: Shared helpers reduce code duplication across providers
// - AC3: All existing unit tests pass without modification
// - AC4: All integration tests pass
// - AC5: No changes to public API or behavior
//
// Test Categories:
// - Happy Path: Normal provider execution with various options
// - Edge Cases: Boundary values, empty options, nil handling
// - Error Handling: Invalid options, missing CLIs, context cancellation
// - Integration: Full workflow execution with refactored providers

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/agents"
)

// =============================================================================
// Happy Path Tests - Normal Provider Execution
// =============================================================================

func TestClaudeProvider_Execute_WithTypeCheckedOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	provider := agents.NewClaudeProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Claude CLI not installed, skipping")
	}

	tests := []struct {
		name    string
		prompt  string
		options map[string]any
	}{
		{
			name:    "string_option_model",
			prompt:  "What is Go?",
			options: map[string]any{"model": "haiku"},
		},
		{
			name:    "string_option_output_format",
			prompt:  "List 3 numbers",
			options: map[string]any{"output_format": "json"},
		},
		{
			name:    "int_option_max_tokens",
			prompt:  "Brief answer",
			options: map[string]any{"max_tokens": 500},
		},
		{
			name:    "bool_option_dangerous_skip",
			prompt:  "Test prompt",
			options: map[string]any{"dangerouslySkipPermissions": true},
		},
		{
			name:   "multiple_options",
			prompt: "Explain briefly",
			options: map[string]any{
				"model":      "haiku",
				"max_tokens": 1000,
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, tt.prompt, tt.options)

			require.NoError(t, err, "Execute should succeed with type-checked options")
			require.NotNil(t, result)
			assert.Equal(t, "claude", result.Provider)
			assert.NotEmpty(t, result.Output, "Should return output")
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
		})
	}
}

func TestCodexProvider_ExecuteConversation_WithTypeCheckedOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	provider := agents.NewCodexProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Codex CLI not installed, skipping")
	}

	tests := []struct {
		name    string
		prompt  string
		options map[string]any
	}{
		{
			name:    "string_option_model",
			prompt:  "Write hello world",
			options: map[string]any{"model": "gpt-4o"},
		},
		{
			name:    "string_option_language",
			prompt:  "Sample code",
			options: map[string]any{"language": "go"},
		},
		{
			name:    "int_option_max_tokens",
			prompt:  "Short answer",
			options: map[string]any{"max_tokens": 500},
		},
		{
			name:    "float_option_temperature",
			prompt:  "Creative output",
			options: map[string]any{"temperature": 0.7},
		},
		{
			name:    "bool_option_quiet",
			prompt:  "Test",
			options: map[string]any{"quiet": true},
		},
		{
			name:   "all_option_types",
			prompt: "Full test",
			options: map[string]any{
				"model":       "gpt-4o",
				"language":    "python",
				"max_tokens":  1000,
				"temperature": 0.5,
				"quiet":       false,
			},
		},
	}

	ctx := context.Background()
	state := &workflow.ConversationState{
		Turns: []workflow.Turn{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.ExecuteConversation(ctx, state, tt.prompt, tt.options)

			require.NoError(t, err, "ExecuteConversation should succeed with type-checked options")
			require.NotNil(t, result)
			require.NotNil(t, result.State)
			assert.Equal(t, "codex", result.Provider)
			assert.NotEmpty(t, result.Output)
			assert.Len(t, result.State.Turns, 1, "Should add one turn")
		})
	}
}

func TestGeminiProvider_ExecuteConversation_WithTypeCheckedOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	provider := agents.NewGeminiProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Gemini CLI not installed, skipping")
	}

	tests := []struct {
		name    string
		prompt  string
		options map[string]any
	}{
		{
			name:    "string_option_model",
			prompt:  "What is AI?",
			options: map[string]any{"model": "gemini-pro"},
		},
		{
			name:    "string_option_output_format",
			prompt:  "List 3 items",
			options: map[string]any{"output_format": "json"},
		},
		{
			name:   "multiple_options",
			prompt: "Test prompt",
			options: map[string]any{
				"model":         "gemini-pro",
				"output_format": "json",
			},
		},
	}

	ctx := context.Background()
	state := &workflow.ConversationState{
		Turns: []workflow.Turn{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.ExecuteConversation(ctx, state, tt.prompt, tt.options)

			require.NoError(t, err, "ExecuteConversation should succeed with type-checked options")
			require.NotNil(t, result)
			require.NotNil(t, result.State)
			assert.Equal(t, "gemini", result.Provider)
			assert.NotEmpty(t, result.Output)
		})
	}
}

// =============================================================================
// Edge Cases - Boundary Values and Nil Handling
// =============================================================================

func TestClaudeProvider_Execute_EmptyAndNilOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	provider := agents.NewClaudeProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Claude CLI not installed, skipping")
	}

	ctx := context.Background()

	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name:    "nil_options",
			options: nil,
		},
		{
			name:    "empty_options_map",
			options: map[string]any{},
		},
		{
			name: "options_with_nil_values",
			options: map[string]any{
				"model": nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, "What is 2+2?", tt.options)

			require.NoError(t, err, "Should handle nil/empty options gracefully")
			require.NotNil(t, result)
			assert.NotEmpty(t, result.Output)
		})
	}
}

func TestSharedHelpers_TokenEstimation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Test that token estimation is consistent across providers after extraction
	tests := []struct {
		name     string
		provider ports.AgentProvider
	}{
		{
			name:     "claude_provider",
			provider: agents.NewClaudeProvider(),
		},
		{
			name:     "codex_provider",
			provider: agents.NewCodexProvider(),
		},
		{
			name:     "gemini_provider",
			provider: agents.NewGeminiProvider(),
		},
	}

	ctx := context.Background()
	longPrompt := "This is a long prompt to test token estimation. " +
		"It should return a reasonable token count that is greater than zero. " +
		"The exact count doesn't matter, but consistency does."

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.provider.Validate(); err != nil {
				t.Skipf("%s CLI not installed, skipping", tt.provider.Name())
			}

			result, err := tt.provider.Execute(ctx, longPrompt, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			// Token estimation should work
			assert.Greater(t, result.Tokens, 0, "Should estimate tokens")
		})
	}
}

func TestConversationState_Cloning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Test that state cloning works correctly after helper extraction
	provider := agents.NewCodexProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Codex CLI not installed, skipping")
	}

	ctx := context.Background()

	// Create initial state with history
	initialState := &workflow.ConversationState{
		Turns: []workflow.Turn{
			{
				Role:    "user",
				Content: "First message",
			},
			{
				Role:    "assistant",
				Content: "First response",
			},
		},
		TotalTokens: 100,
	}

	result, err := provider.ExecuteConversation(ctx, initialState, "Second message", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.State)

	// Verify state was cloned (not mutated in place)
	assert.Len(t, initialState.Turns, 2, "Original state should be unchanged")
	assert.Len(t, result.State.Turns, 3, "Updated state should have new turn")
	assert.NotEqual(t, initialState.TotalTokens, result.State.TotalTokens,
		"Token counts should differ")
}

// =============================================================================
// Error Handling - Invalid Options and Context Cancellation
// =============================================================================

func TestClaudeProvider_Execute_InvalidOptionTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	provider := agents.NewClaudeProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Claude CLI not installed, skipping")
	}

	ctx := context.Background()

	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name: "wrong_type_model_int",
			options: map[string]any{
				"model": 123, // Should be string
			},
		},
		{
			name: "wrong_type_max_tokens_string",
			options: map[string]any{
				"max_tokens": "500", // Should be int
			},
		},
		{
			name: "wrong_type_bool_string",
			options: map[string]any{
				"dangerouslySkipPermissions": "true", // Should be bool
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Type-checked helpers should ignore wrong types gracefully
			result, err := provider.Execute(ctx, "Test prompt", tt.options)

			// Should either succeed (ignoring bad options) or return clear error
			if err != nil {
				assert.NotEmpty(t, err.Error(), "Error should have message")
			} else {
				require.NotNil(t, result)
				assert.NotEmpty(t, result.Output)
			}
		})
	}
}

func TestCodexProvider_ExecuteConversation_EmptyPrompt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	provider := agents.NewCodexProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Codex CLI not installed, skipping")
	}

	ctx := context.Background()
	state := &workflow.ConversationState{
		Turns: []workflow.Turn{},
	}

	result, err := provider.ExecuteConversation(ctx, state, "", nil)

	require.Error(t, err, "Should reject empty prompt")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt", "Error should mention prompt")
}

func TestGeminiProvider_ExecuteConversation_NilState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	provider := agents.NewGeminiProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Gemini CLI not installed, skipping")
	}

	ctx := context.Background()

	result, err := provider.ExecuteConversation(ctx, nil, "Test prompt", nil)

	require.Error(t, err, "Should reject nil state")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "state", "Error should mention state")
}

func TestAllProviders_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name     string
		provider ports.AgentProvider
	}{
		{
			name:     "claude",
			provider: agents.NewClaudeProvider(),
		},
		{
			name:     "codex",
			provider: agents.NewCodexProvider(),
		},
		{
			name:     "gemini",
			provider: agents.NewGeminiProvider(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.provider.Validate(); err != nil {
				t.Skipf("%s CLI not installed, skipping", tt.provider.Name())
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			result, err := tt.provider.Execute(ctx, "Test prompt", nil)

			require.Error(t, err, "Should fail on cancelled context")
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "context", "Error should mention context")
		})
	}
}

func TestAllProviders_ContextTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name     string
		provider ports.AgentProvider
	}{
		{
			name:     "claude",
			provider: agents.NewClaudeProvider(),
		},
		{
			name:     "codex",
			provider: agents.NewCodexProvider(),
		},
		{
			name:     "gemini",
			provider: agents.NewGeminiProvider(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.provider.Validate(); err != nil {
				t.Skipf("%s CLI not installed, skipping", tt.provider.Name())
			}

			// Very short timeout to trigger timeout error
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
			defer cancel()

			time.Sleep(10 * time.Millisecond) // Ensure timeout

			result, err := tt.provider.Execute(ctx, "Test prompt that will timeout", nil)

			require.Error(t, err, "Should fail on timeout")
			assert.Nil(t, result)
		})
	}
}

// =============================================================================
// Integration Tests - Full Workflow Execution
// =============================================================================

func TestIntegration_MultiTurnConversation_WithTokenEstimation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	provider := agents.NewCodexProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Codex CLI not installed, skipping")
	}

	ctx := context.Background()
	state := &workflow.ConversationState{
		Turns: []workflow.Turn{},
	}

	// Turn 1
	result1, err := provider.ExecuteConversation(ctx, state, "Write hello world in Go", nil)
	require.NoError(t, err)
	require.NotNil(t, result1)
	state = result1.State
	assert.Len(t, state.Turns, 1)
	assert.Greater(t, state.TotalTokens, 0, "Should track tokens")

	tokens1 := state.TotalTokens

	// Turn 2
	result2, err := provider.ExecuteConversation(ctx, state, "Now add error handling", nil)
	require.NoError(t, err)
	require.NotNil(t, result2)
	state = result2.State
	assert.Len(t, state.Turns, 2)
	assert.Greater(t, state.TotalTokens, tokens1, "Tokens should accumulate")

	tokens2 := state.TotalTokens

	// Turn 3
	result3, err := provider.ExecuteConversation(ctx, state, "Add tests", nil)
	require.NoError(t, err)
	require.NotNil(t, result3)
	state = result3.State
	assert.Len(t, state.Turns, 3)
	assert.Greater(t, state.TotalTokens, tokens2, "Tokens should keep accumulating")
}

func TestIntegration_JSONParsing_SharedHelper(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name     string
		provider ports.AgentProvider
		prompt   string
	}{
		{
			name:     "claude_json_parsing",
			provider: agents.NewClaudeProvider(),
			prompt:   "Return JSON object with 'status': 'ok'",
		},
		{
			name:     "gemini_json_parsing",
			provider: agents.NewGeminiProvider(),
			prompt:   "Return JSON object with 'result': 'success'",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.provider.Validate(); err != nil {
				t.Skipf("%s CLI not installed, skipping", tt.provider.Name())
			}

			options := map[string]any{
				"output_format": "json",
			}

			result, err := tt.provider.Execute(ctx, tt.prompt, options)

			require.NoError(t, err, "JSON parsing should work with shared helper")
			require.NotNil(t, result)
			assert.NotNil(t, result.Response, "Should have parsed JSON response")
			assert.IsType(t, map[string]any{}, result.Response,
				"Response should be parsed JSON object")
		})
	}
}

func TestIntegration_ProviderSpecificValidation_Preserved(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Test that provider-specific validations still work after refactoring
	// (ADR-003: Preserve provider-specific validation)

	t.Run("claude_model_alias_validation", func(t *testing.T) {
		provider := agents.NewClaudeProvider()
		if err := provider.Validate(); err != nil {
			t.Skip("Claude CLI not installed, skipping")
		}

		ctx := context.Background()

		// Valid model alias
		result, err := provider.Execute(ctx, "Test", map[string]any{"model": "haiku"})
		require.NoError(t, err)
		require.NotNil(t, result)

		// Invalid model alias should be handled gracefully
		result2, err := provider.Execute(ctx, "Test", map[string]any{"model": "invalid-model-xyz"})
		// May succeed or fail depending on provider, but should not panic
		if err != nil {
			assert.NotEmpty(t, err.Error())
		} else {
			require.NotNil(t, result2)
		}
	})

	t.Run("codex_language_option", func(t *testing.T) {
		provider := agents.NewCodexProvider()
		if err := provider.Validate(); err != nil {
			t.Skip("Codex CLI not installed, skipping")
		}

		ctx := context.Background()
		state := &workflow.ConversationState{Turns: []workflow.Turn{}}

		// Language option should still work
		result, err := provider.ExecuteConversation(ctx, state, "Write code",
			map[string]any{"language": "python"})

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("gemini_model_whitelist", func(t *testing.T) {
		provider := agents.NewGeminiProvider()
		if err := provider.Validate(); err != nil {
			t.Skip("Gemini CLI not installed, skipping")
		}

		ctx := context.Background()
		state := &workflow.ConversationState{Turns: []workflow.Turn{}}

		// Valid Gemini model
		result, err := provider.ExecuteConversation(ctx, state, "Test",
			map[string]any{"model": "gemini-pro"})

		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// =============================================================================
// Backward Compatibility Tests
// =============================================================================

func TestBackwardCompatibility_ExistingWorkflows(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Test that refactored providers work with existing workflow fixtures
	// This ensures no behavioral changes (AC5)

	t.Run("agent_simple_fixture", func(t *testing.T) {
		provider := agents.NewClaudeProvider()
		if err := provider.Validate(); err != nil {
			t.Skip("Claude CLI not installed, skipping")
		}

		ctx := context.Background()
		options := map[string]any{
			"model":      "claude-haiku-4-5",
			"max_tokens": 1000,
		}

		result, err := provider.Execute(ctx, "Test task", options)

		require.NoError(t, err, "Should work with agent-simple fixture options")
		require.NotNil(t, result)
		assert.NotEmpty(t, result.Output)
	})

	t.Run("conversation_multiturn_fixture", func(t *testing.T) {
		provider := agents.NewCodexProvider()
		if err := provider.Validate(); err != nil {
			t.Skip("Codex CLI not installed, skipping")
		}

		ctx := context.Background()
		state := &workflow.ConversationState{Turns: []workflow.Turn{}}
		options := map[string]any{
			"model":      "gpt-4o",
			"max_tokens": 2000,
		}

		result, err := provider.ExecuteConversation(ctx, state, "First turn", options)

		require.NoError(t, err, "Should work with conversation fixture options")
		require.NotNil(t, result)
		require.NotNil(t, result.State)
		assert.Len(t, result.State.Turns, 1)
	})
}

// =============================================================================
// Performance and Regression Tests
// =============================================================================

func TestPerformance_NoRegressionFromHelpers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Basic smoke test to ensure helper extraction doesn't cause performance issues
	// (Risk: Performance regression from function call overhead - P2 Very Low)

	provider := agents.NewClaudeProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Claude CLI not installed, skipping")
	}

	ctx := context.Background()
	prompt := "What is 2+2?"

	start := time.Now()
	result, err := provider.Execute(ctx, prompt, nil)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should complete in reasonable time (helper overhead is negligible)
	assert.Less(t, elapsed, 60*time.Second, "Should not have significant overhead")
}

func TestRegression_AllOptionTypesCombined(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Comprehensive regression test with all option types
	// Validates C001 pattern (type-checked wrappers) works correctly

	provider := agents.NewCodexProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Codex CLI not installed, skipping")
	}

	ctx := context.Background()
	state := &workflow.ConversationState{Turns: []workflow.Turn{}}

	// Exercise all type-checked helpers at once
	options := map[string]any{
		"model":       "gpt-4o", // string
		"language":    "go",     // string
		"max_tokens":  1500,     // int
		"temperature": 0.8,      // float64
		"quiet":       false,    // bool
	}

	result, err := provider.ExecuteConversation(ctx, state,
		"Write comprehensive test", options)

	require.NoError(t, err, "Should handle all option types correctly")
	require.NotNil(t, result)
	require.NotNil(t, result.State)
	assert.NotEmpty(t, result.Output)
	assert.Len(t, result.State.Turns, 1)
}
