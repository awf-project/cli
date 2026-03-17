//go:build integration

package agents

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/httpx"
)

// TestOpenAICompatibleProvider_SingleTurnHappyPath_Integration verifies the full
// single-turn lifecycle (US1): configure provider, execute with prompt against
// httptest.Server, verify AgentResult fields populated correctly.
func TestOpenAICompatibleProvider_SingleTurnHappyPath_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "chatcmpl-integration-test",
			"object": "chat.completion",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "The answer is 4.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 8,
				"total_tokens":      18,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := httpx.NewClient(
		httpx.WithTimeout(10 * time.Second),
	)

	provider := NewOpenAICompatibleProvider(
		WithHTTPClient(httpClient),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := provider.Execute(ctx, "What is 2+2?", map[string]any{
		"base_url": server.URL,
		"model":    "test-model",
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "The answer is 4.", result.Output)
	assert.Equal(t, 18, result.Tokens)
	assert.False(t, result.TokensEstimated)
	assert.Equal(t, "openai_compatible", result.Provider)
	assert.NotZero(t, result.StartedAt)
	assert.NotZero(t, result.CompletedAt)
}

// TestOpenAICompatibleProvider_SingleTurnWithOptions_Integration verifies options
// (temperature, max_tokens) are passed correctly to the API.
func TestOpenAICompatibleProvider_SingleTurnWithOptions_Integration(t *testing.T) {
	var capturedReq chatCompletionsRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &capturedReq)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "chatcmpl-options-test",
			"object": "chat.completion",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Creative response.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     5,
				"completion_tokens": 3,
				"total_tokens":      8,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := httpx.NewClient(
		httpx.WithTimeout(10 * time.Second),
	)

	provider := NewOpenAICompatibleProvider(
		WithHTTPClient(httpClient),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := provider.Execute(ctx, "Be creative", map[string]any{
		"base_url":    server.URL,
		"model":       "gpt-3.5-turbo",
		"temperature": 1.5,
		"max_tokens":  100,
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "Creative response.", result.Output)
	assert.Equal(t, capturedReq.Model, "gpt-3.5-turbo")
	assert.NotNil(t, capturedReq.Temperature)
	assert.Equal(t, *capturedReq.Temperature, 1.5)
	assert.NotNil(t, capturedReq.MaxTokens)
	assert.Equal(t, *capturedReq.MaxTokens, 100)
}

// TestOpenAICompatibleProvider_Conversation_Integration verifies multi-turn
// conversation (US2): execute with 2 prior turns in ConversationState,
// verify full message history is sent, response updates state correctly.
func TestOpenAICompatibleProvider_Conversation_Integration(t *testing.T) {
	var capturedReq chatCompletionsRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &capturedReq)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "chatcmpl-conversation-test",
			"object": "chat.completion",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Response to third turn.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     35,
				"completion_tokens": 10,
				"total_tokens":      45,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := httpx.NewClient(
		httpx.WithTimeout(10 * time.Second),
	)

	provider := NewOpenAICompatibleProvider(
		WithHTTPClient(httpClient),
	)

	state := &workflow.ConversationState{
		Turns: []workflow.Turn{
			{
				Role:    workflow.TurnRoleUser,
				Content: "First question?",
				Tokens:  10,
			},
			{
				Role:    workflow.TurnRoleAssistant,
				Content: "First answer.",
				Tokens:  8,
			},
			{
				Role:    workflow.TurnRoleUser,
				Content: "Second question?",
				Tokens:  12,
			},
		},
		TotalTurns:  3,
		TotalTokens: 30,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := provider.ExecuteConversation(ctx, state, "What about the third question?", map[string]any{
		"base_url":      server.URL,
		"model":         "test-model",
		"system_prompt": "You are a helpful assistant.",
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "Response to third turn.", result.Output)
	assert.Equal(t, 45, result.TokensTotal)
	assert.Equal(t, 35, result.TokensInput)
	assert.Equal(t, 10, result.TokensOutput)
	assert.False(t, result.TokensEstimated)
	assert.NotNil(t, result.State)
	assert.Greater(t, result.State.TotalTurns, 3)

	require.Len(t, capturedReq.Messages, 5, "should include system message + 3 prior turns + new user message")
	assert.Equal(t, capturedReq.Messages[0].Role, "system", "first message should be system prompt")
	assert.Equal(t, capturedReq.Messages[0].Content, "You are a helpful assistant.")
	assert.Equal(t, capturedReq.Messages[1].Role, "user", "first prior turn should be user")
	assert.Equal(t, capturedReq.Messages[1].Content, "First question?")
	assert.Equal(t, capturedReq.Messages[2].Role, "assistant", "first response should be assistant")
	assert.Equal(t, capturedReq.Messages[2].Content, "First answer.")
	assert.Equal(t, capturedReq.Messages[3].Role, "user", "second prior turn should be user")
	assert.Equal(t, capturedReq.Messages[3].Content, "Second question?")
	assert.Equal(t, capturedReq.Messages[4].Role, "user", "new turn should be user")
	assert.Equal(t, capturedReq.Messages[4].Content, "What about the third question?")
}

// TestOpenAICompatibleProvider_HTTP401Unauthorized_Integration verifies (US3)
// HTTP 401 produces clear authentication error without leaking API key.
func TestOpenAICompatibleProvider_HTTP401Unauthorized_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid API key",
		})
	}))
	defer server.Close()

	httpClient := httpx.NewClient(
		httpx.WithTimeout(10 * time.Second),
	)

	provider := NewOpenAICompatibleProvider(
		WithHTTPClient(httpClient),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := provider.Execute(ctx, "Test prompt", map[string]any{
		"base_url": server.URL,
		"model":    "test-model",
		"api_key":  "sk-secret-key-value-1234",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
	assert.NotContains(t, err.Error(), "sk-secret-key-value-1234")
	assert.NotContains(t, err.Error(), "secret")
	assert.NotContains(t, err.Error(), "API key")
}

// TestOpenAICompatibleProvider_HTTP429RateLimit_Integration verifies (US3)
// HTTP 429 produces rate-limit error with Retry-After info.
func TestOpenAICompatibleProvider_HTTP429RateLimit_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Rate limit exceeded",
		})
	}))
	defer server.Close()

	httpClient := httpx.NewClient(
		httpx.WithTimeout(10 * time.Second),
	)

	provider := NewOpenAICompatibleProvider(
		WithHTTPClient(httpClient),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := provider.Execute(ctx, "Test prompt", map[string]any{
		"base_url": server.URL,
		"model":    "test-model",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
	assert.Contains(t, err.Error(), "60")
}

// TestOpenAICompatibleProvider_HTTP500ServerError_Integration verifies (US3)
// HTTP 5xx produces server error message.
func TestOpenAICompatibleProvider_HTTP500ServerError_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Internal server error",
		})
	}))
	defer server.Close()

	httpClient := httpx.NewClient(
		httpx.WithTimeout(10 * time.Second),
	)

	provider := NewOpenAICompatibleProvider(
		WithHTTPClient(httpClient),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := provider.Execute(ctx, "Test prompt", map[string]any{
		"base_url": server.URL,
		"model":    "test-model",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "server error")
	assert.Contains(t, err.Error(), "500")
}

// TestOpenAICompatibleProvider_RequestBodyValidation_Integration verifies
// the request body contains expected structure: model, messages array,
// optional fields (temperature, max_tokens, top_p).
func TestOpenAICompatibleProvider_RequestBodyValidation_Integration(t *testing.T) {
	var capturedReq chatCompletionsRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &capturedReq)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "chatcmpl-validation-test",
			"object": "chat.completion",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Response.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     5,
				"completion_tokens": 1,
				"total_tokens":      6,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := httpx.NewClient(
		httpx.WithTimeout(10 * time.Second),
	)

	provider := NewOpenAICompatibleProvider(
		WithHTTPClient(httpClient),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := provider.Execute(ctx, "Test prompt", map[string]any{
		"base_url":    server.URL,
		"model":       "gpt-3.5-turbo",
		"temperature": 0.7,
		"max_tokens":  1024,
		"top_p":       0.9,
	})

	require.NoError(t, err)

	assert.Equal(t, "gpt-3.5-turbo", capturedReq.Model)
	assert.Len(t, capturedReq.Messages, 1)
	assert.Equal(t, "user", capturedReq.Messages[0].Role)
	assert.Equal(t, "Test prompt", capturedReq.Messages[0].Content)
	assert.NotNil(t, capturedReq.Temperature)
	assert.Equal(t, 0.7, *capturedReq.Temperature)
	assert.NotNil(t, capturedReq.MaxTokens)
	assert.Equal(t, 1024, *capturedReq.MaxTokens)
	assert.NotNil(t, capturedReq.TopP)
	assert.Equal(t, 0.9, *capturedReq.TopP)
}

// TestOpenAICompatibleProvider_JSONOutput_Integration verifies output_format: json
// parsing works correctly from the API response.
func TestOpenAICompatibleProvider_JSONOutput_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "chatcmpl-json-test",
			"object": "chat.completion",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"name": "John", "age": 30, "city": "New York"}`,
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     5,
				"completion_tokens": 3,
				"total_tokens":      8,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := httpx.NewClient(
		httpx.WithTimeout(10 * time.Second),
	)

	provider := NewOpenAICompatibleProvider(
		WithHTTPClient(httpClient),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := provider.Execute(ctx, "Return JSON", map[string]any{
		"base_url":      server.URL,
		"model":         "test-model",
		"output_format": "json",
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.HasJSONResponse())
	assert.Len(t, result.Response, 3)
	assert.Equal(t, "John", result.Response["name"])
	assert.Equal(t, float64(30), result.Response["age"])
	assert.Equal(t, "New York", result.Response["city"])
}

// TestOpenAICompatibleProvider_BaseURLNormalization_Integration verifies
// trailing slash is handled correctly in base_url.
func TestOpenAICompatibleProvider_BaseURLNormalization_Integration(t *testing.T) {
	handlerCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		assert.Equal(t, "/chat/completions", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "chatcmpl-url-test",
			"object": "chat.completion",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "OK",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := httpx.NewClient(
		httpx.WithTimeout(10 * time.Second),
	)

	provider := NewOpenAICompatibleProvider(
		WithHTTPClient(httpClient),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	baseURLWithSlash := server.URL + "/"

	_, err := provider.Execute(ctx, "Test", map[string]any{
		"base_url": baseURLWithSlash,
		"model":    "test-model",
	})

	require.NoError(t, err)
	assert.True(t, handlerCalled, "handler should be called with normalized URL")
}

// TestOpenAICompatibleProvider_ConversationWithSystemPrompt_Integration verifies
// system_prompt is included as first message in conversation requests.
func TestOpenAICompatibleProvider_ConversationWithSystemPrompt_Integration(t *testing.T) {
	var capturedReq chatCompletionsRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &capturedReq)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "chatcmpl-system-test",
			"object": "chat.completion",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "System response.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     20,
				"completion_tokens": 5,
				"total_tokens":      25,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := httpx.NewClient(
		httpx.WithTimeout(10 * time.Second),
	)

	provider := NewOpenAICompatibleProvider(
		WithHTTPClient(httpClient),
	)

	state := &workflow.ConversationState{
		Turns: []workflow.Turn{
			{
				Role:    workflow.TurnRoleUser,
				Content: "Hello",
				Tokens:  5,
			},
		},
		TotalTurns:  1,
		TotalTokens: 5,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := provider.ExecuteConversation(ctx, state, "Follow-up", map[string]any{
		"base_url":      server.URL,
		"model":         "test-model",
		"system_prompt": "You are an expert assistant.",
	})

	require.NoError(t, err)

	assert.Len(t, capturedReq.Messages, 3)
	assert.Equal(t, "system", capturedReq.Messages[0].Role)
	assert.Equal(t, "You are an expert assistant.", capturedReq.Messages[0].Content)
	assert.Equal(t, "user", capturedReq.Messages[1].Role)
	assert.Equal(t, "Hello", capturedReq.Messages[1].Content)
	assert.Equal(t, "user", capturedReq.Messages[2].Role)
	assert.Equal(t, "Follow-up", capturedReq.Messages[2].Content)
}
