package agents

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAICompatibleProvider_Execute_Success(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		options map[string]any
	}{
		{
			name:   "simple prompt with base URL and model",
			prompt: "What is 2+2?",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama3",
			},
		},
		{
			name:   "prompt with temperature option",
			prompt: "test",
			options: map[string]any{
				"base_url":    "http://api.openai.com/v1",
				"model":       "gpt-4",
				"temperature": 0.7,
			},
		},
		{
			name:   "prompt with max_tokens option",
			prompt: "generate text",
			options: map[string]any{
				"base_url":   "https://ollama.example.com/v1",
				"model":      "mistral",
				"max_tokens": 1024,
			},
		},
		{
			name:   "prompt with multiple options",
			prompt: "test",
			options: map[string]any{
				"base_url":    "http://localhost:8000/v1",
				"model":       "model-name",
				"temperature": 0.5,
				"max_tokens":  512,
				"top_p":       0.9,
			},
		},
		{
			name:   "base URL with trailing slash",
			prompt: "test",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1/",
				"model":    "llama",
			},
		},
		{
			name:   "base URL without trailing slash",
			prompt: "test",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider()

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			// Real implementation must not error and must return AgentResult
			assert.NoError(t, err, "should execute successfully")
			require.NotNil(t, result, "should return AgentResult")
			assert.Equal(t, "openai_compatible", result.Provider)
			assert.NotEmpty(t, result.Output, "should return non-empty output")
			assert.Greater(t, result.Tokens, 0, "should have token count")
			assert.False(t, result.TokensEstimated, "should report actual tokens from API")
			assert.False(t, result.StartedAt.IsZero(), "should have start time")
			assert.False(t, result.CompletedAt.IsZero(), "should have completion time")
			assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt))
		})
	}
}

func TestOpenAICompatibleProvider_Execute_EmptyPrompt(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		wantErr string
	}{
		{
			name:    "empty string",
			prompt:  "",
			wantErr: "prompt cannot be empty",
		},
		{
			name:    "whitespace only",
			prompt:  "   \t\n  ",
			wantErr: "prompt cannot be empty",
		},
		{
			name:    "tabs and newlines",
			prompt:  "\t  \n  ",
			wantErr: "prompt cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
			}
			provider := NewOpenAICompatibleProvider()

			result, err := provider.Execute(context.Background(), tt.prompt, options, nil, nil)
			require.Error(t, err, "should return error for empty prompt")
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestOpenAICompatibleProvider_Validate(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr string
	}{
		{
			name: "missing base_url",
			options: map[string]any{
				"model": "llama",
			},
			wantErr: "base_url is required",
		},
		{
			name: "missing model",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
			},
			wantErr: "model is required",
		},
		{
			name:    "empty options",
			options: map[string]any{},
			wantErr: "base_url is required",
		},
		{
			name: "invalid temperature low",
			options: map[string]any{
				"base_url":    "http://localhost:11434/v1",
				"model":       "llama",
				"temperature": -0.5,
			},
			wantErr: "temperature must be between 0 and 2",
		},
		{
			name: "invalid temperature high",
			options: map[string]any{
				"base_url":    "http://localhost:11434/v1",
				"model":       "llama",
				"temperature": 2.5,
			},
			wantErr: "temperature must be between 0 and 2",
		},
		{
			name: "negative max_tokens",
			options: map[string]any{
				"base_url":   "http://localhost:11434/v1",
				"model":      "llama",
				"max_tokens": -1,
			},
			wantErr: "max_completion_tokens must be non-negative",
		},
		{
			name: "invalid top_p high",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
				"top_p":    5.0,
			},
			wantErr: "top_p must be between 0 and 1",
		},
		{
			name: "invalid top_p negative",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
				"top_p":    -0.1,
			},
			wantErr: "top_p must be between 0 and 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars set by TestMain so fallback paths are not triggered.
			t.Setenv("OPENAI_BASE_URL", "")
			t.Setenv("OPENAI_MODEL", "")

			provider := NewOpenAICompatibleProvider()
			_, err := provider.Execute(context.Background(), "test prompt", tt.options, nil, nil)
			require.Error(t, err, "should return error for invalid options")
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestOpenAICompatibleProvider_ExecuteConversation_Success(t *testing.T) {
	tests := []struct {
		name       string
		stateSetup func() *workflow.ConversationState
		prompt     string
		options    map[string]any
		minTurns   int
	}{
		{
			name: "new conversation",
			stateSetup: func() *workflow.ConversationState {
				return workflow.NewConversationState("You are a helpful assistant")
			},
			prompt:   "Hello",
			minTurns: 2,
		},
		{
			name: "existing conversation with turns",
			stateSetup: func() *workflow.ConversationState {
				state := workflow.NewConversationState("You are helpful")
				state.Turns = []workflow.Turn{
					*workflow.NewTurn(workflow.TurnRoleSystem, "You are helpful"),
					*workflow.NewTurn(workflow.TurnRoleUser, "What is 2+2?"),
					*workflow.NewTurn(workflow.TurnRoleAssistant, "4"),
				}
				state.TotalTurns = 3
				state.TotalTokens = 50
				return state
			},
			prompt:   "What about 3+3?",
			minTurns: 5,
		},
		{
			name: "conversation with system prompt",
			stateSetup: func() *workflow.ConversationState {
				return workflow.NewConversationState("You are a code expert")
			},
			prompt:   "explain loops",
			minTurns: 2,
		},
		{
			name: "conversation with temperature option",
			stateSetup: func() *workflow.ConversationState {
				return workflow.NewConversationState("assistant")
			},
			prompt:   "test",
			options:  map[string]any{"temperature": 0.7},
			minTurns: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider()
			state := tt.stateSetup()

			result, err := provider.ExecuteConversation(context.Background(), state, tt.prompt, tt.options, nil, nil)

			require.NoError(t, err, "should execute conversation successfully")
			require.NotNil(t, result, "should return ConversationResult")
			assert.Equal(t, "openai_compatible", result.Provider)
			assert.NotEmpty(t, result.Output, "should have conversation output")
			assert.NotNil(t, result.State, "should return updated state")
			assert.GreaterOrEqual(t, result.State.TotalTurns, tt.minTurns)
			assert.False(t, result.TokensEstimated, "should report actual tokens from API")
			assert.Greater(t, result.TokensTotal, 0, "should have token total")
			assert.False(t, result.StartedAt.IsZero(), "should have start time")
			assert.False(t, result.CompletedAt.IsZero(), "should have completion time")
		})
	}
}

func TestOpenAICompatibleProvider_ExecuteConversation_NilState(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	result, err := provider.ExecuteConversation(context.Background(), nil, "test", nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversation state cannot be nil")
	assert.Nil(t, result)
}

func TestOpenAICompatibleProvider_ExecuteConversation_EmptyPrompt(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		wantErr string
	}{
		{
			name:    "empty string",
			prompt:  "",
			wantErr: "prompt cannot be empty",
		},
		{
			name:    "whitespace only",
			prompt:  "   \t\n  ",
			wantErr: "prompt cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider()
			state := workflow.NewConversationState("system")

			result, err := provider.ExecuteConversation(context.Background(), state, tt.prompt, nil, nil, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestOpenAICompatibleProvider_ExecuteConversation_StatePreservation(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	initialState := workflow.NewConversationState("You are helpful")
	initialState.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleSystem, "You are helpful"),
		*workflow.NewTurn(workflow.TurnRoleUser, "Hello"),
		*workflow.NewTurn(workflow.TurnRoleAssistant, "Hi"),
	}
	initialState.TotalTurns = 3
	initialState.TotalTokens = 50

	initialTurnCount := len(initialState.Turns)
	initialTotalTurns := initialState.TotalTurns
	initialTotalTokens := initialState.TotalTokens

	result, err := provider.ExecuteConversation(context.Background(), initialState, "How are you?", nil, nil, nil)

	require.NoError(t, err, "should execute without error")
	require.NotNil(t, result, "should return result")

	// Original state should NOT be modified (cloned)
	assert.Equal(t, initialTurnCount, len(initialState.Turns))
	assert.Equal(t, initialTotalTurns, initialState.TotalTurns)
	assert.Equal(t, initialTotalTokens, initialState.TotalTokens)

	// Result state should have new turns
	assert.Greater(t, result.State.TotalTurns, initialState.TotalTurns)
	assert.Greater(t, len(result.State.Turns), len(initialState.Turns))
}

func TestOpenAICompatibleProvider_Name(t *testing.T) {
	provider := NewOpenAICompatibleProvider()
	assert.Equal(t, "openai_compatible", provider.Name())
}

func TestOpenAICompatibleProvider_APIKeyNeverInErrors(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		options map[string]any
	}{
		{
			name:   "api_key in options",
			apiKey: "sk-test-secret-key-12345",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
				"api_key":  "sk-test-secret-key-12345",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider()

			result, err := provider.Execute(context.Background(), "test", tt.options, nil, nil)
			if err != nil {
				errMsg := err.Error()
				assert.NotContains(t, errMsg, tt.apiKey, "API key should never appear in error message")
				assert.NotContains(t, errMsg, "sk-", "API key prefix should not appear in error")
			}

			if result != nil {
				assert.NotContains(t, result.Output, tt.apiKey)
			}
		})
	}
}

func TestOpenAICompatibleProvider_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr string
	}{
		{
			name: "missing base_url",
			options: map[string]any{
				"model": "llama",
			},
			wantErr: "base_url",
		},
		{
			name: "missing model",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
			},
			wantErr: "model",
		},
		{
			name: "negative temperature",
			options: map[string]any{
				"base_url":    "http://localhost:11434/v1",
				"model":       "llama",
				"temperature": -0.5,
			},
			wantErr: "temperature",
		},
		{
			name: "temperature too high",
			options: map[string]any{
				"base_url":    "http://localhost:11434/v1",
				"model":       "llama",
				"temperature": 2.5,
			},
			wantErr: "temperature",
		},
		{
			name: "negative max_tokens",
			options: map[string]any{
				"base_url":   "http://localhost:11434/v1",
				"model":      "llama",
				"max_tokens": -1,
			},
			wantErr: "max_completion_tokens must be non-negative",
		},
		{
			name: "temperature int too high",
			options: map[string]any{
				"base_url":    "http://localhost:11434/v1",
				"model":       "llama",
				"temperature": 3,
			},
			wantErr: "temperature",
		},
		{
			name: "top_p int too high",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
				"top_p":    2,
			},
			wantErr: "top_p",
		},
		{
			name: "temperature invalid type",
			options: map[string]any{
				"base_url":    "http://localhost:11434/v1",
				"model":       "llama",
				"temperature": "hot",
			},
			wantErr: "temperature must be a number",
		},
		{
			name: "top_p invalid type",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
				"top_p":    "high",
			},
			wantErr: "top_p must be a number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars so fallbacks don't mask missing options
			t.Setenv("OPENAI_BASE_URL", "")
			t.Setenv("OPENAI_MODEL", "")

			provider := NewOpenAICompatibleProvider()

			result, err := provider.Execute(context.Background(), "test prompt", tt.options, nil, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestOpenAICompatibleProvider_MaxCompletionTokensMigration(t *testing.T) {
	tests := []struct {
		name       string
		options    map[string]any
		wantErr    bool
		wantErrMsg string
		setupHTTP  bool
	}{
		{
			name: "max_completion_tokens is accepted",
			options: map[string]any{
				"base_url":              "http://localhost:11434/v1",
				"model":                 "llama",
				"max_completion_tokens": 1024,
			},
			wantErr: false,
		},
		{
			name: "legacy max_tokens still works (fallback)",
			options: map[string]any{
				"base_url":   "http://localhost:11434/v1",
				"model":      "llama",
				"max_tokens": 2048,
			},
			wantErr: false,
		},
		{
			name: "max_completion_tokens takes priority over max_tokens",
			options: map[string]any{
				"base_url":              "http://localhost:11434/v1",
				"model":                 "llama",
				"max_completion_tokens": 512,
				"max_tokens":            2048,
			},
			wantErr: false,
		},
		{
			name: "negative max_completion_tokens rejected",
			options: map[string]any{
				"base_url":              "http://localhost:11434/v1",
				"model":                 "llama",
				"max_completion_tokens": -1,
			},
			wantErr:    true,
			wantErrMsg: "max_completion_tokens must be non-negative",
		},
		{
			name: "negative max_tokens (legacy) rejected",
			options: map[string]any{
				"base_url":   "http://localhost:11434/v1",
				"model":      "llama",
				"max_tokens": -10,
			},
			wantErr:    true,
			wantErrMsg: "max_completion_tokens must be non-negative",
		},
		{
			name: "max_completion_tokens as float converted to int",
			options: map[string]any{
				"base_url":              "http://localhost:11434/v1",
				"model":                 "llama",
				"max_completion_tokens": 256.0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENAI_BASE_URL", "")
			t.Setenv("OPENAI_MODEL", "")

			provider := NewOpenAICompatibleProvider()
			_, err := provider.Execute(context.Background(), "test prompt", tt.options, nil, nil)

			if tt.wantErr {
				require.Error(t, err, "expected error for %s", tt.name)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			} else {
				// Expect HTTP error since we're not mocking HTTP, but validation should pass
				if err != nil {
					assert.NotContains(t, err.Error(), "max_completion_tokens")
					assert.NotContains(t, err.Error(), "max_tokens")
				}
			}
		})
	}
}

func TestOpenAICompatibleProvider_MaxCompletionTokensStructField(t *testing.T) {
	t.Run("chatCompletionsRequest uses MaxCompletionTokens field", func(t *testing.T) {
		req := chatCompletionsRequest{
			Model:               "llama",
			MaxCompletionTokens: ptrInt(1024),
		}
		body, err := json.Marshal(req)
		require.NoError(t, err)
		assert.Contains(t, string(body), "\"max_completion_tokens\"")
		assert.NotContains(t, string(body), "\"max_tokens\"")
	})
}

func TestOpenAICompatibleProvider_MaxCompletionTokensParsingLogic(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		want    *int
		wantErr bool
	}{
		{
			name:    "prefers max_completion_tokens over max_tokens",
			options: map[string]any{"max_completion_tokens": 512, "max_tokens": 2048},
			want:    ptrInt(512),
		},
		{
			name:    "falls back to max_tokens when max_completion_tokens absent",
			options: map[string]any{"max_tokens": 256},
			want:    ptrInt(256),
		},
		{
			name:    "handles float64 conversion",
			options: map[string]any{"max_completion_tokens": 512.0},
			want:    ptrInt(512),
		},
		{
			name:    "rejects negative max_completion_tokens",
			options: map[string]any{"max_completion_tokens": -1},
			wantErr: true,
		},
		{
			name:    "rejects negative max_tokens legacy",
			options: map[string]any{"max_tokens": -100},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseMaxCompletionTokensOption(tt.options)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "max_completion_tokens must be non-negative")
			} else {
				require.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, result)
				} else {
					assert.NotNil(t, result)
					assert.Equal(t, *tt.want, *result)
				}
			}
		})
	}
}

func ptrInt(v int) *int {
	return &v
}

func TestOpenAICompatibleProvider_Execute_TokenTracking(t *testing.T) {
	t.Run("tokens are actual not estimated", func(t *testing.T) {
		provider := NewOpenAICompatibleProvider()
		options := map[string]any{
			"base_url": "http://localhost:11434/v1",
			"model":    "llama",
		}

		result, err := provider.Execute(context.Background(), "test", options, nil, nil)

		require.NoError(t, err, "should execute without error")
		require.NotNil(t, result, "should return result")
		// Real implementation should have TokensEstimated=false
		assert.False(t, result.TokensEstimated, "OpenAI-compatible provider should report actual tokens, not estimates")
		assert.Greater(t, result.Tokens, 0, "Tokens should be populated from API response")
	})
}

func TestOpenAICompatibleProvider_ExecuteConversation_TokenTracking(t *testing.T) {
	t.Run("conversation tokens from API usage", func(t *testing.T) {
		provider := NewOpenAICompatibleProvider()
		state := workflow.NewConversationState("You are helpful")

		result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

		require.NoError(t, err, "should execute without error")
		require.NotNil(t, result, "should return result")
		// Real implementation should track tokens from API usage
		assert.False(t, result.TokensEstimated, "should use actual tokens from API")
		assert.Greater(t, result.TokensTotal, 0, "total tokens should be set")
		assert.Greater(t, result.TokensInput, 0, "input tokens should be set")
		assert.Greater(t, result.TokensOutput, 0, "output tokens should be set")
		assert.Equal(t, result.TokensInput+result.TokensOutput, result.TokensTotal)
	})
}

func TestOpenAICompatibleProvider_BaseURLNormalization(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{
			name:    "trailing slash",
			baseURL: "http://localhost:11434/v1/",
		},
		{
			name:    "no trailing slash",
			baseURL: "http://localhost:11434/v1",
		},
		{
			name:    "https with trailing slash",
			baseURL: "https://api.openai.com/v1/",
		},
		{
			name:    "https without trailing slash",
			baseURL: "https://api.openai.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider()
			options := map[string]any{
				"base_url": tt.baseURL,
				"model":    "llama",
			}

			result, err := provider.Execute(context.Background(), "test", options, nil, nil)

			// Should normalize URL regardless of trailing slash — no error expected.
			assert.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}

func TestOpenAICompatibleProvider_ConstructorWithOptions(t *testing.T) {
	t.Run("constructor with functional options", func(t *testing.T) {
		provider := NewOpenAICompatibleProvider()
		assert.NotNil(t, provider)
		assert.Equal(t, "openai_compatible", provider.Name())
	})
}

func TestOpenAICompatibleProvider_Execute_WithOptionsMap(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name: "with temperature",
			options: map[string]any{
				"base_url":    "http://localhost:11434/v1",
				"model":       "llama",
				"temperature": 0.7,
			},
		},
		{
			name: "with max_tokens",
			options: map[string]any{
				"base_url":   "http://localhost:11434/v1",
				"model":      "llama",
				"max_tokens": 2048,
			},
		},
		{
			name: "with top_p",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
				"top_p":    0.9,
			},
		},
		{
			name: "with multiple options",
			options: map[string]any{
				"base_url":    "http://localhost:11434/v1",
				"model":       "llama",
				"temperature": 0.5,
				"max_tokens":  1024,
				"top_p":       0.95,
			},
		},
		{
			name: "temperature as int (YAML unmarshal)",
			options: map[string]any{
				"base_url":    "http://localhost:11434/v1",
				"model":       "llama",
				"temperature": 1,
			},
		},
		{
			name: "top_p as int zero (deterministic sampling)",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
				"top_p":    0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider()

			result, err := provider.Execute(context.Background(), "test prompt", tt.options, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)
		})
	}
}

func TestOpenAICompatibleProvider_ExecuteConversation_WithHistory(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	state := workflow.NewConversationState("assistant")
	state.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleUser, "What is AI?"),
		*workflow.NewTurn(workflow.TurnRoleAssistant, "AI is..."),
	}
	state.TotalTurns = 2
	state.TotalTokens = 30

	result, err := provider.ExecuteConversation(context.Background(), state, "Tell me more", nil, nil, nil)

	require.NoError(t, err, "should execute without error")
	require.NotNil(t, result, "should return result")
	assert.NotNil(t, result.State, "should have updated state")
	assert.GreaterOrEqual(t, result.State.TotalTurns, 4, "should have more turns after adding user and assistant response")
	assert.Greater(t, result.State.TotalTokens, state.TotalTokens, "should have more tokens after conversation")
}

func TestOpenAICompatibleProvider_Execute_JSONOutputFormat(t *testing.T) {
	tests := []struct {
		name       string
		options    map[string]any
		shouldFail bool
		errSubstr  string
	}{
		{
			name: "with output_format json expects json but mock returns text",
			options: map[string]any{
				"base_url":      "http://localhost:11434/v1",
				"model":         "llama",
				"output_format": "json",
			},
			shouldFail: true,
			errSubstr:  "parse response as json",
		},
		{
			name: "with output_format json and other options",
			options: map[string]any{
				"base_url":      "http://localhost:11434/v1",
				"model":         "llama",
				"output_format": "json",
				"temperature":   0.7,
				"max_tokens":    512,
			},
			shouldFail: true,
			errSubstr:  "parse response as json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider()

			result, err := provider.Execute(context.Background(), "Return valid JSON", tt.options, nil, nil)

			if tt.shouldFail {
				// Should fail because mock server returns plain text, not json
				assert.Error(t, err, "should error when output_format is json but response is not json")
				assert.Nil(t, result, "should not return result when json parsing fails")
				assert.Contains(t, err.Error(), tt.errSubstr, "error should mention json parsing")
			} else {
				require.NoError(t, err, "should execute successfully with json format")
				require.NotNil(t, result, "should return AgentResult")
				assert.Equal(t, "openai_compatible", result.Provider)
				assert.NotEmpty(t, result.Output, "should have output content")

				// When output_format is json, the Response field should be populated with parsed json
				assert.NotNil(t, result.Response, "Response field should be populated when output_format is json")
				if result.Response != nil {
					assert.IsType(t, map[string]any{}, result.Response, "Response should be a map")
				}
			}
		})
	}
}

func TestOpenAICompatibleProvider_Execute_JSONParsing_InvalidJSON(t *testing.T) {
	t.Run("invalid JSON response with output_format json", func(t *testing.T) {
		provider := NewOpenAICompatibleProvider()
		options := map[string]any{
			"base_url":      "http://localhost:11434/v1",
			"model":         "llama",
			"output_format": "json",
		}

		// The mock server returns plain text "This is a mock response..."
		// When output_format is "json", parseJSONResponse should fail on non-JSON
		result, err := provider.Execute(context.Background(), "Return invalid json: {broken", options, nil, nil)

		// parseJSONResponse should return an error when response is not valid JSON
		require.Error(t, err, "should error when response cannot be parsed as json")
		assert.Nil(t, result, "should not return result when JSON parsing fails")
		assert.Contains(t, err.Error(), "parse response as json", "error should mention json parsing")
	})
}

func TestOpenAICompatibleProvider_Execute_NoJSONOutputFormat(t *testing.T) {
	t.Run("without output_format should skip JSON parsing", func(t *testing.T) {
		provider := NewOpenAICompatibleProvider()
		options := map[string]any{
			"base_url": "http://localhost:11434/v1",
			"model":    "llama",
		}

		result, err := provider.Execute(context.Background(), "test prompt", options, nil, nil)

		require.NoError(t, err, "should execute without error")
		require.NotNil(t, result, "should return AgentResult")

		// Without output_format: json, Response field should remain empty (as initialized)
		// NewAgentResult initializes Response to an empty map, so we check it's empty
		assert.Empty(t, result.Response, "Response should be empty when output_format is not json")
		assert.NotEmpty(t, result.Output, "but Output should still have content")
	})
}

func TestOpenAICompatibleProvider_Execute_JSONOutputFormat_TextOnly(t *testing.T) {
	t.Run("output_format json with non-json response should error", func(t *testing.T) {
		provider := NewOpenAICompatibleProvider()
		options := map[string]any{
			"base_url":      "http://localhost:11434/v1",
			"model":         "llama",
			"output_format": "json",
		}

		// The mock server returns plain text "This is a mock response..."
		// When output_format is "json", parseJSONResponse must fail
		result, err := provider.Execute(context.Background(), "What is 2+2?", options, nil, nil)

		// parseJSONResponse should fail on non-json text
		require.Error(t, err, "should error when response is not json")
		assert.Nil(t, result, "should not return result on parse failure")
		assert.Contains(t, err.Error(), "parse response as json", "error should mention json parsing")
	})
}

func TestOpenAICompatibleProvider_ExecuteConversation_MessageArrayStructure(t *testing.T) {
	// T004: Verify ConversationState.Turns are converted to messages array with correct role/content
	tests := []struct {
		name     string
		setup    func() *workflow.ConversationState
		prompt   string
		options  map[string]any
		validate func(t *testing.T, result *workflow.ConversationResult, err error)
	}{
		{
			name: "empty turns creates user message",
			setup: func() *workflow.ConversationState {
				return workflow.NewConversationState("system message")
			},
			prompt: "Hello",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
			},
			validate: func(t *testing.T, result *workflow.ConversationResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				// Should have at least user + assistant turns
				assert.GreaterOrEqual(t, len(result.State.Turns), 2, "should have at least user and assistant turns")
				// User turn should be last or second-to-last (before assistant)
				assert.NotEmpty(t, result.Output, "should have assistant response")
			},
		},
		{
			name: "existing turns are preserved and new turns added",
			setup: func() *workflow.ConversationState {
				state := workflow.NewConversationState("helpful assistant")
				state.Turns = []workflow.Turn{
					*workflow.NewTurn(workflow.TurnRoleUser, "What is AI?"),
					*workflow.NewTurn(workflow.TurnRoleAssistant, "AI is..."),
				}
				return state
			},
			prompt: "Tell me more",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "gpt-4",
			},
			validate: func(t *testing.T, result *workflow.ConversationResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.NotNil(t, result.State)
				// Should have original 2 turns + new user + new assistant = 4
				assert.GreaterOrEqual(t, len(result.State.Turns), 4, "should preserve and add to existing turns")
				// Last turn should be assistant
				lastTurn := result.State.Turns[len(result.State.Turns)-1]
				assert.Equal(t, workflow.TurnRoleAssistant, lastTurn.Role, "last turn should be assistant response")
			},
		},
		{
			name: "multiple turns conversation maintains order",
			setup: func() *workflow.ConversationState {
				state := workflow.NewConversationState("system")
				state.Turns = []workflow.Turn{
					*workflow.NewTurn(workflow.TurnRoleUser, "Q1"),
					*workflow.NewTurn(workflow.TurnRoleAssistant, "A1"),
					*workflow.NewTurn(workflow.TurnRoleUser, "Q2"),
					*workflow.NewTurn(workflow.TurnRoleAssistant, "A2"),
				}
				return state
			},
			prompt: "Q3",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
			},
			validate: func(t *testing.T, result *workflow.ConversationResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.NotNil(t, result.State)
				// Should have 4 original + new user + new assistant = 6
				assert.GreaterOrEqual(t, len(result.State.Turns), 6, "should maintain all turns")
				// Verify second-to-last is user message with new prompt
				newUserIdx := len(result.State.Turns) - 2
				if newUserIdx >= 0 && newUserIdx < len(result.State.Turns) {
					assert.Equal(t, "Q3", result.State.Turns[newUserIdx].Content, "new user prompt should be in state")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider()
			state := tt.setup()
			result, err := provider.ExecuteConversation(context.Background(), state, tt.prompt, tt.options, nil, nil)
			tt.validate(t, result, err)
		})
	}
}

func TestOpenAICompatibleProvider_ExecuteConversation_TokenFieldsPopulated(t *testing.T) {
	// T004: Verify ConversationResult token fields are populated from API usage (TokensEstimated=false)
	tests := []struct {
		name     string
		setup    func() *workflow.ConversationState
		options  map[string]any
		validate func(t *testing.T, result *workflow.ConversationResult)
	}{
		{
			name: "single turn populates all token fields",
			setup: func() *workflow.ConversationState {
				return workflow.NewConversationState("be helpful")
			},
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
			},
			validate: func(t *testing.T, result *workflow.ConversationResult) {
				require.NotNil(t, result)
				// All token fields must be set from API response
				assert.Greater(t, result.TokensInput, 0, "TokensInput should be populated from API usage.prompt_tokens")
				assert.Greater(t, result.TokensOutput, 0, "TokensOutput should be populated from API usage.completion_tokens")
				assert.Greater(t, result.TokensTotal, 0, "TokensTotal should be populated from API usage.total_tokens")
				assert.False(t, result.TokensEstimated, "TokensEstimated must be false (actual tokens from API)")
			},
		},
		{
			name: "multi-turn conversation tracks token totals",
			setup: func() *workflow.ConversationState {
				state := workflow.NewConversationState("system")
				state.Turns = []workflow.Turn{
					*workflow.NewTurn(workflow.TurnRoleUser, "First question"),
					*workflow.NewTurn(workflow.TurnRoleAssistant, "First answer"),
				}
				state.TotalTokens = 25
				return state
			},
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "gpt-4",
			},
			validate: func(t *testing.T, result *workflow.ConversationResult) {
				require.NotNil(t, result)
				// Conversation result should have individual turn tokens
				assert.Greater(t, result.TokensInput, 0, "TokensInput from new turn should be set")
				assert.Greater(t, result.TokensOutput, 0, "TokensOutput from new turn should be set")
				// Total should be sum of input and output from this turn
				assert.Equal(t, result.TokensInput+result.TokensOutput, result.TokensTotal,
					"TokensTotal should equal TokensInput + TokensOutput for this turn")
				assert.False(t, result.TokensEstimated, "must use actual tokens from API")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider()
			state := tt.setup()
			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", tt.options, nil, nil)

			require.NoError(t, err, "should execute without error")
			tt.validate(t, result)
		})
	}
}

func TestOpenAICompatibleProvider_ExecuteConversation_StateUpdateValidation(t *testing.T) {
	// T004: Verify that new user and assistant turns are added to ConversationState
	provider := NewOpenAICompatibleProvider()

	state := workflow.NewConversationState("You are helpful")
	state.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleUser, "Hello"),
		*workflow.NewTurn(workflow.TurnRoleAssistant, "Hi there"),
	}

	originalTurnCount := len(state.Turns)

	result, err := provider.ExecuteConversation(context.Background(), state, "How are you?", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "llama",
	}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.State)

	// Verify new turns were added
	assert.Equal(t, originalTurnCount+2, len(result.State.Turns),
		"should add exactly 2 new turns (user + assistant)")

	// Verify second-to-last turn is the new user message
	newUserTurn := result.State.Turns[len(result.State.Turns)-2]
	assert.Equal(t, workflow.TurnRoleUser, newUserTurn.Role, "second-to-last turn should be user")
	assert.Equal(t, "How are you?", newUserTurn.Content, "user turn content should match the prompt")

	// Verify last turn is assistant response
	newAssistantTurn := result.State.Turns[len(result.State.Turns)-1]
	assert.Equal(t, workflow.TurnRoleAssistant, newAssistantTurn.Role, "last turn should be assistant")
	assert.Equal(t, result.Output, newAssistantTurn.Content,
		"assistant turn content should match ConversationResult Output")
}

func TestOpenAICompatibleProvider_ExecuteConversation_ResponseOutput(t *testing.T) {
	// T004: Verify Output field is populated with assistant response
	provider := NewOpenAICompatibleProvider()

	state := workflow.NewConversationState("system")
	result, err := provider.ExecuteConversation(context.Background(), state, "What is 2+2?", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "llama",
	}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Output must be populated from the API response
	assert.NotEmpty(t, result.Output, "Output should contain the assistant's response")
	assert.False(t, result.TokensEstimated, "TokensEstimated must be false")

	// Provider name must be set
	assert.Equal(t, "openai_compatible", result.Provider, "Provider should be 'openai_compatible'")
}

func TestOpenAICompatibleProvider_ExecuteConversation_SystemPromptHandling(t *testing.T) {
	// T004: system_prompt configuration is available in options (though message construction happens internally)
	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name: "with system_prompt option",
			options: map[string]any{
				"base_url":      "http://localhost:11434/v1",
				"model":         "gpt-4",
				"system_prompt": "You are a helpful coding assistant",
			},
		},
		{
			name: "without system_prompt option",
			options: map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "llama",
			},
		},
		{
			name: "with system_prompt and other options",
			options: map[string]any{
				"base_url":      "http://localhost:11434/v1",
				"model":         "mistral",
				"system_prompt": "You are a JSON API",
				"temperature":   0.5,
				"max_tokens":    1024,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider()
			state := workflow.NewConversationState("system")

			result, err := provider.ExecuteConversation(context.Background(), state, "test", tt.options, nil, nil)

			require.NoError(t, err, "should execute regardless of system_prompt presence")
			require.NotNil(t, result)
			assert.NotEmpty(t, result.Output, "should return assistant response")
			assert.False(t, result.TokensEstimated)
		})
	}
}

func TestOpenAICompatibleProvider_ExecuteConversation_ErrorPropagation(t *testing.T) {
	// T004: ExecuteConversation error handling for invalid options
	tests := []struct {
		name    string
		state   *workflow.ConversationState
		prompt  string
		options map[string]any
		wantErr string
	}{
		{
			name:   "invalid temperature (too high)",
			state:  workflow.NewConversationState("system"),
			prompt: "test",
			options: map[string]any{
				"base_url":    "http://localhost:11434/v1",
				"model":       "llama",
				"temperature": 3.5,
			},
			wantErr: "temperature",
		},
		{
			name:   "negative max_tokens",
			state:  workflow.NewConversationState("system"),
			prompt: "test",
			options: map[string]any{
				"base_url":   "http://localhost:11434/v1",
				"model":      "llama",
				"max_tokens": -100,
			},
			wantErr: "max_completion_tokens must be non-negative",
		},
		{
			name:   "invalid temperature (negative)",
			state:  workflow.NewConversationState("system"),
			prompt: "test",
			options: map[string]any{
				"base_url":    "http://localhost:11434/v1",
				"model":       "llama",
				"temperature": -0.5,
			},
			wantErr: "temperature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider()

			result, err := provider.ExecuteConversation(context.Background(), tt.state, tt.prompt, tt.options, nil, nil)

			require.Error(t, err, "should return error for invalid options")
			assert.Nil(t, result, "should not return result on error")
			assert.Contains(t, err.Error(), tt.wantErr, "error should mention the problem")
		})
	}
}

func TestOpenAICompatibleProvider_ExecuteConversation_TimestampTracking(t *testing.T) {
	// T004: Verify StartedAt and CompletedAt are set
	provider := NewOpenAICompatibleProvider()
	state := workflow.NewConversationState("system")

	startBefore := time.Now()
	result, err := provider.ExecuteConversation(context.Background(), state, "test", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "llama",
	}, nil, nil)
	endAfter := time.Now()

	require.NoError(t, err)
	require.NotNil(t, result)

	// Timestamps must be within reasonable bounds
	assert.False(t, result.StartedAt.IsZero(), "StartedAt must be set")
	assert.False(t, result.CompletedAt.IsZero(), "CompletedAt must be set")
	assert.True(t, result.StartedAt.After(startBefore.Add(-time.Second)) || result.StartedAt.Equal(startBefore),
		"StartedAt should be near function start")
	assert.True(t, result.CompletedAt.Before(endAfter.Add(time.Second)) || result.CompletedAt.Equal(endAfter),
		"CompletedAt should be near function end")
	assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt),
		"CompletedAt should be after or equal to StartedAt")
}

// Component T005: HTTP Error Mapping Tests
// These tests verify error handling for HTTP status codes and network failures

type mockHTTPDoer struct {
	statusCode int
	retryAfter string
	body       string
}

func (m *mockHTTPDoer) Do(_ *http.Request) (*http.Response, error) {
	header := http.Header{}
	if m.retryAfter != "" {
		header.Set("Retry-After", m.retryAfter)
	}
	body := m.body
	if body == "" {
		body = `{"error":"test error"}`
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

// newMockHTTPClient creates a mock HTTP client that returns a specific status code and optional Retry-After header.
// Used for testing error handling without real network calls.
func newMockHTTPClient(statusCode int, retryAfter, body string) *httpx.Client {
	return httpx.NewClient(httpx.WithDoer(&mockHTTPDoer{
		statusCode: statusCode,
		retryAfter: retryAfter,
		body:       body,
	}))
}

func TestOpenAICompatibleProvider_HTTPErrorMapping_401Unauthorized_Execute(t *testing.T) {
	// T005: 401 → "authentication failed" error message
	tests := []struct {
		name       string
		statusCode int
		wantErrMsg string
	}{
		{
			name:       "401 Unauthorized returns authentication failed error",
			statusCode: 401,
			wantErrMsg: "authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockHTTPClient(tt.statusCode, "", "")
			provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))

			result, err := provider.Execute(context.Background(), "test prompt", map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "test-model",
			}, nil, nil)

			require.Error(t, err, "should return error for 401 status")
			assert.Nil(t, result, "should not return result on error")
			assert.Contains(t, err.Error(), tt.wantErrMsg, "error should contain authentication failed message")
		})
	}
}

func TestOpenAICompatibleProvider_HTTPErrorMapping_429RateLimited_Execute(t *testing.T) {
	// T005: 429 + Retry-After → "rate limited" with retry duration
	tests := []struct {
		name       string
		statusCode int
		retryAfter string
		wantErrMsg string
	}{
		{
			name:       "429 with Retry-After header includes duration",
			statusCode: 429,
			retryAfter: "60",
			wantErrMsg: "rate limited",
		},
		{
			name:       "429 without Retry-After header",
			statusCode: 429,
			retryAfter: "",
			wantErrMsg: "rate limited",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockHTTPClient(tt.statusCode, tt.retryAfter, "")
			provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))

			result, err := provider.Execute(context.Background(), "test prompt", map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "test-model",
			}, nil, nil)

			require.Error(t, err, "should return error for 429 status")
			assert.Nil(t, result, "should not return result on error")
			assert.Contains(t, err.Error(), tt.wantErrMsg, "error should contain rate limited message")

			if tt.retryAfter != "" {
				assert.Contains(t, err.Error(), tt.retryAfter, "error should include Retry-After value")
			}
		})
	}
}

func TestOpenAICompatibleProvider_HTTPErrorMapping_5xxServerError_Execute(t *testing.T) {
	// T005: 5xx → "server error" message with status code
	tests := []struct {
		name       string
		statusCode int
		wantErrMsg string
	}{
		{
			name:       "500 Internal Server Error",
			statusCode: 500,
			wantErrMsg: "server error",
		},
		{
			name:       "502 Bad Gateway",
			statusCode: 502,
			wantErrMsg: "server error",
		},
		{
			name:       "503 Service Unavailable",
			statusCode: 503,
			wantErrMsg: "server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockHTTPClient(tt.statusCode, "", "")
			provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))

			result, err := provider.Execute(context.Background(), "test prompt", map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "test-model",
			}, nil, nil)

			require.Error(t, err, "should return error for 5xx status")
			assert.Nil(t, result, "should not return result on error")
			assert.Contains(t, err.Error(), tt.wantErrMsg, "error should contain server error message")
		})
	}
}

func TestOpenAICompatibleProvider_HTTPErrorMapping_400BadRequest_Execute(t *testing.T) {
	// T005: 400+ (non-5xx) → "bad request" message
	tests := []struct {
		name       string
		statusCode int
		wantErrMsg string
	}{
		{
			name:       "400 Bad Request",
			statusCode: 400,
			wantErrMsg: "bad request",
		},
		{
			name:       "403 Forbidden",
			statusCode: 403,
			wantErrMsg: "bad request",
		},
		{
			name:       "404 Not Found",
			statusCode: 404,
			wantErrMsg: "bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockHTTPClient(tt.statusCode, "", "")
			provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))

			result, err := provider.Execute(context.Background(), "test prompt", map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "test-model",
			}, nil, nil)

			require.Error(t, err, "should return error for 4xx status")
			assert.Nil(t, result, "should not return result on error")
			assert.Contains(t, err.Error(), tt.wantErrMsg, "error should contain bad request message")
		})
	}
}

func TestOpenAICompatibleProvider_HTTPErrorMapping_401Unauthorized_ExecuteConversation(t *testing.T) {
	// T005: 401 in ExecuteConversation → "authentication failed"
	mockClient := newMockHTTPClient(401, "", "")
	provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))
	state := workflow.NewConversationState("system")

	result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test-model",
	}, nil, nil)

	require.Error(t, err, "should return error for 401 status")
	assert.Nil(t, result, "should not return result on error")
	assert.Contains(t, err.Error(), "authentication failed", "error should contain authentication failed message")
}

func TestOpenAICompatibleProvider_HTTPErrorMapping_429RateLimited_ExecuteConversation(t *testing.T) {
	// T005: 429 in ExecuteConversation → "rate limited"
	mockClient := newMockHTTPClient(429, "120", "")
	provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))
	state := workflow.NewConversationState("system")

	result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test-model",
	}, nil, nil)

	require.Error(t, err, "should return error for 429 status")
	assert.Nil(t, result, "should not return result on error")
	assert.Contains(t, err.Error(), "rate limited", "error should contain rate limited message")
}

func TestOpenAICompatibleProvider_HTTPErrorMapping_5xxServerError_ExecuteConversation(t *testing.T) {
	// T005: 5xx in ExecuteConversation → "server error"
	mockClient := newMockHTTPClient(503, "", "")
	provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))
	state := workflow.NewConversationState("system")

	result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test-model",
	}, nil, nil)

	require.Error(t, err, "should return error for 5xx status")
	assert.Nil(t, result, "should not return result on error")
	assert.Contains(t, err.Error(), "server error", "error should contain server error message")
}

func TestOpenAICompatibleProvider_HTTPErrorMapping_APIKeyNeverInError(t *testing.T) {
	// T005: NFR-002 - API key must never appear in error messages
	tests := []struct {
		name       string
		statusCode int
		apiKey     string
	}{
		{
			name:       "401 with API key should not leak key",
			statusCode: 401,
			apiKey:     "sk-test-secret-key-12345", //nolint:gosec // test fixture value for API key leak detection test
		},
		{ //nolint:gosec // G101: test fixture value for API key leak detection test
			name:       "429 with API key should not leak key",
			statusCode: 429,
			apiKey:     "sk-another-secret-key",
		},
		{ //nolint:gosec // G101: test fixture value for API key leak detection test
			name:       "500 with API key should not leak key",
			statusCode: 500,
			apiKey:     "sk-secret-api-key-xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockHTTPClient(tt.statusCode, "", "")
			provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))

			result, err := provider.Execute(context.Background(), "test prompt", map[string]any{
				"base_url": "http://localhost:11434/v1",
				"model":    "test-model",
				"api_key":  tt.apiKey,
			}, nil, nil)

			require.Error(t, err, "should return error for error status")
			assert.Nil(t, result, "should not return result on error")

			// Critical: API key must never appear in error message
			assert.NotContains(t, err.Error(), tt.apiKey, "error message must not contain API key")
			assert.NotContains(t, err.Error(), "Bearer", "error message must not contain Bearer token prefix")
		})
	}
}

func TestOpenAICompatibleProvider_ContextDeadlineExceeded_Execute(t *testing.T) {
	// T005: Context deadline exceeded → "deadline exceeded" error
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel context

	provider := NewOpenAICompatibleProvider()

	result, err := provider.Execute(ctx, "test prompt", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test-model",
	}, nil, nil)

	require.Error(t, err, "should return error for canceled context")
	assert.Nil(t, result, "should not return result on error")
	assert.Contains(t, err.Error(), "context canceled", "error should indicate context was canceled")
}

func TestOpenAICompatibleProvider_ContextDeadlineExceeded_ExecuteConversation(t *testing.T) {
	// T005: Context deadline exceeded in ExecuteConversation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel context

	provider := NewOpenAICompatibleProvider()
	state := workflow.NewConversationState("system")

	result, err := provider.ExecuteConversation(ctx, state, "test prompt", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test-model",
	}, nil, nil)

	require.Error(t, err, "should return error for canceled context")
	assert.Nil(t, result, "should not return result on error")
	assert.Contains(t, err.Error(), "context canceled", "error should indicate context was canceled")
}

func TestOpenAICompatibleProvider_HTTPErrorMapping_UnexpectedStatus_Execute(t *testing.T) {
	// T005: Edge case - non-standard status codes still report with status number
	mockClient := newMockHTTPClient(418, "", "") // 418 I'm a teapot
	provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))

	result, err := provider.Execute(context.Background(), "test prompt", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test-model",
	}, nil, nil)

	require.Error(t, err, "should return error for unexpected status")
	assert.Nil(t, result, "should not return result on error")
	assert.Contains(t, err.Error(), "unexpected status", "error should mention unexpected status")
	assert.Contains(t, err.Error(), "418", "error should include the status code")
}
