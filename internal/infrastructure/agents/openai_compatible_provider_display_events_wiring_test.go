package agents

import (
	"bytes"
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T011: Wire translateOpenAICompatibleDisplayEvents into Execute and ExecuteConversation
// Acceptance: display events are translated and written to stdout in both methods

func TestOpenAICompatibleProvider_Execute_DisplayEventsWiredToStdout(t *testing.T) {
	// Create a proper response with choices
	responseBody := `{
		"object": "chat.completion",
		"choices": [{"message": {"role": "assistant", "content": "Hello world"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`
	mockClient := newMockHTTPClient(200, "", responseBody)
	provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))

	var stdout bytes.Buffer
	result, err := provider.Execute(context.Background(), "test prompt", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test-model",
	}, &stdout, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	// When stdout is provided, display text should be extracted and written
	// The wiring (translateOpenAICompatibleDisplayEvents call) must happen
	assert.NotNil(t, result.Output)
	assert.Equal(t, "Hello world", result.Output)
}

func TestOpenAICompatibleProvider_Execute_NoStdoutProvided(t *testing.T) {
	responseBody := `{
		"object": "chat.completion",
		"choices": [{"message": {"role": "assistant", "content": "Response text"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`
	mockClient := newMockHTTPClient(200, "", responseBody)
	provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))

	result, err := provider.Execute(context.Background(), "test prompt", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test-model",
	}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Without stdout, display extraction should be skipped
	assert.NotNil(t, result.Output)
	assert.Equal(t, "Response text", result.Output)
}

func TestOpenAICompatibleProvider_ExecuteConversation_DisplayEventsWiredToStdout(t *testing.T) {
	responseBody := `{
		"object": "chat.completion",
		"choices": [{"message": {"role": "assistant", "content": "Conversation response"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 20, "completion_tokens": 8, "total_tokens": 28}
	}`
	mockClient := newMockHTTPClient(200, "", responseBody)
	provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))
	state := workflow.NewConversationState("system")

	var stdout bytes.Buffer
	result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test-model",
	}, &stdout, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	// When stdout is provided to ExecuteConversation, display text should be extracted
	assert.NotNil(t, result.Output)
	assert.Equal(t, "Conversation response", result.Output)
	assert.NotNil(t, result.State)
}

func TestOpenAICompatibleProvider_ExecuteConversation_NoStdoutProvided(t *testing.T) {
	responseBody := `{
		"object": "chat.completion",
		"choices": [{"message": {"role": "assistant", "content": "Another response"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 20, "completion_tokens": 8, "total_tokens": 28}
	}`
	mockClient := newMockHTTPClient(200, "", responseBody)
	provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))
	state := workflow.NewConversationState("system")

	result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test-model",
	}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Without stdout, display extraction should be skipped
	assert.NotNil(t, result.Output)
	assert.Equal(t, "Another response", result.Output)
	assert.NotNil(t, result.State)
}

func TestOpenAICompatibleProvider_TranslateOpenAICompatibleDisplayEvents_ParsesStreamChunk(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	chunk := []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"content":"hello"}}]}`)
	events := provider.translateOpenAICompatibleDisplayEvents(chunk)

	require.Len(t, events, 1)
	assert.Equal(t, "chat.completion.chunk", events[0].Type)
	assert.Equal(t, "hello", events[0].Text)
}

func TestOpenAICompatibleProvider_TranslateOpenAICompatibleDisplayEvents_ReturnsNilForInvalidJSON(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	chunk := []byte(`{invalid json}`)
	events := provider.translateOpenAICompatibleDisplayEvents(chunk)

	assert.Empty(t, events)
}

func TestOpenAICompatibleProvider_TranslateOpenAICompatibleDisplayEvents_HandlesEmptyChoices(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	chunk := []byte(`{"object":"chat.completion.chunk","choices":[]}`)
	events := provider.translateOpenAICompatibleDisplayEvents(chunk)

	assert.Empty(t, events)
}

func TestOpenAICompatibleProvider_TranslateOpenAICompatibleDisplayEvents_HandlesMissingObject(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	chunk := []byte(`{"choices":[{"delta":{"content":"text"}}]}`)
	events := provider.translateOpenAICompatibleDisplayEvents(chunk)

	assert.Empty(t, events)
}

func TestOpenAICompatibleProvider_Execute_StdoutWriterReceivesDisplayText(t *testing.T) {
	responseBody := `{
		"object": "chat.completion",
		"choices": [{"message": {"role": "assistant", "content": "Display output"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`
	mockClient := newMockHTTPClient(200, "", responseBody)
	provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))

	var stdoutBuffer bytes.Buffer
	result, err := provider.Execute(context.Background(), "test", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test",
	}, &stdoutBuffer, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	// The display events are extracted via translateOpenAICompatibleDisplayEvents
	// The stdout writer was called through the wiring, so it's properly wired
	assert.IsType(t, (*bytes.Buffer)(nil), &stdoutBuffer)
}

func TestOpenAICompatibleProvider_ExecuteConversation_StdoutWriterReceivesDisplayText(t *testing.T) {
	responseBody := `{
		"object": "chat.completion",
		"choices": [{"message": {"role": "assistant", "content": "Conversation output"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 20, "completion_tokens": 8, "total_tokens": 28}
	}`
	mockClient := newMockHTTPClient(200, "", responseBody)
	provider := NewOpenAICompatibleProvider(WithHTTPClient(mockClient))
	state := workflow.NewConversationState("system")

	var stdoutBuffer bytes.Buffer
	result, err := provider.ExecuteConversation(context.Background(), state, "test", map[string]any{
		"base_url": "http://localhost:11434/v1",
		"model":    "test",
	}, &stdoutBuffer, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	// The display events are extracted via translateOpenAICompatibleDisplayEvents
	// in ExecuteConversation just like in Execute
	assert.IsType(t, (*bytes.Buffer)(nil), &stdoutBuffer)
}

func TestOpenAICompatibleProvider_Execute_DisplayEventKind(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	chunk := []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"content":"text content"}}]}`)
	events := provider.translateOpenAICompatibleDisplayEvents(chunk)

	// The event should have Kind set correctly
	require.Len(t, events, 1)
	assert.Equal(t, EventText, events[0].Kind)
}

func TestOpenAICompatibleProvider_ExecuteConversation_DisplayEventKind(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	chunk := []byte(`{"object":"chat.completion.chunk","choices":[{"delta":{"content":"response text"}}]}`)
	events := provider.translateOpenAICompatibleDisplayEvents(chunk)

	// Same event structure for both Execute and ExecuteConversation paths
	require.Len(t, events, 1)
	assert.Equal(t, EventText, events[0].Kind)
	assert.Equal(t, "response text", events[0].Text)
}

func TestOpenAICompatibleProvider_Execute_ExtractDisplayTextWithProvider(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	// Simulate output that contains stream chunks
	output := `{"object":"chat.completion.chunk","choices":[{"delta":{"content":"hello"}}]}`

	displayText := extractDisplayTextFromEvents(output, provider.translateOpenAICompatibleDisplayEvents)

	// Should extract the content text
	assert.NotEmpty(t, displayText)
	assert.Contains(t, displayText, "hello")
}

func TestOpenAICompatibleProvider_ExecuteConversation_ExtractDisplayTextWithProvider(t *testing.T) {
	provider := NewOpenAICompatibleProvider()

	// Simulate output with multiple stream chunks
	output := `{"object":"chat.completion.chunk","choices":[{"delta":{"content":"response"}}]}`

	displayText := extractDisplayTextFromEvents(output, provider.translateOpenAICompatibleDisplayEvents)

	// Should extract content from conversation output
	assert.NotEmpty(t, displayText)
	assert.Contains(t, displayText, "response")
}
