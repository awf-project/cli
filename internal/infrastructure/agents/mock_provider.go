package agents

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// MockProvider provides deterministic responses for testing without external CLI calls.
// It implements ports.AgentProvider interface.
type MockProvider struct {
	name                  string
	responses             map[string]*workflow.AgentResult
	conversationResponses map[string]*workflow.ConversationResult
	defaultResult         *workflow.AgentResult
	defaultConvResult     *workflow.ConversationResult
	delay                 time.Duration
	validateError         error

	mu    sync.Mutex
	calls []MockCall
}

// MockCall records a call made to the mock provider.
type MockCall struct {
	Method  string
	Prompt  string
	Options map[string]any
	State   *workflow.ConversationState
}

// NewMockProvider creates a new mock provider with the given name.
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name:                  name,
		responses:             make(map[string]*workflow.AgentResult),
		conversationResponses: make(map[string]*workflow.ConversationResult),
		delay:                 1 * time.Millisecond,
		calls:                 make([]MockCall, 0),
	}
}

// WithResponse configures a response for prompts containing the given pattern.
func (m *MockProvider) WithResponse(pattern string, result *workflow.AgentResult) *MockProvider {
	m.responses[pattern] = result
	return m
}

// WithConversationResponse configures a conversation response for prompts containing the pattern.
func (m *MockProvider) WithConversationResponse(pattern string, result *workflow.ConversationResult) *MockProvider {
	m.conversationResponses[pattern] = result
	return m
}

// WithDefaultResponse sets the default response when no pattern matches.
func (m *MockProvider) WithDefaultResponse(result *workflow.AgentResult) *MockProvider {
	m.defaultResult = result
	return m
}

// WithDefaultConversationResponse sets the default conversation response.
func (m *MockProvider) WithDefaultConversationResponse(result *workflow.ConversationResult) *MockProvider {
	m.defaultConvResult = result
	return m
}

// WithDelay sets the simulated processing delay.
func (m *MockProvider) WithDelay(d time.Duration) *MockProvider {
	m.delay = d
	return m
}

// WithValidateError configures the provider to return an error on Validate().
func (m *MockProvider) WithValidateError(err error) *MockProvider {
	m.validateError = err
	return m
}

// Execute implements ports.AgentProvider.
func (m *MockProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	m.mu.Lock()
	m.calls = append(m.calls, MockCall{
		Method:  "Execute",
		Prompt:  prompt,
		Options: options,
	})
	m.mu.Unlock()

	// Simulate processing delay
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.delay):
	}

	// Find matching response by pattern
	for pattern, result := range m.responses {
		if strings.Contains(prompt, pattern) {
			return m.cloneResult(result), nil
		}
	}

	// Return default or generate one
	if m.defaultResult != nil {
		return m.cloneResult(m.defaultResult), nil
	}

	return &workflow.AgentResult{
		Provider:    m.name,
		Output:      "mock response for: " + truncate(prompt, 50),
		Response:    map[string]any{"mock": true},
		Tokens:      100,
		StartedAt:   time.Now().Add(-m.delay),
		CompletedAt: time.Now(),
	}, nil
}

// ExecuteConversation implements ports.AgentProvider.
func (m *MockProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	m.mu.Lock()
	m.calls = append(m.calls, MockCall{
		Method:  "ExecuteConversation",
		Prompt:  prompt,
		Options: options,
		State:   state,
	})
	m.mu.Unlock()

	// Simulate processing delay
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.delay):
	}

	// Find matching response by pattern
	for pattern, result := range m.conversationResponses {
		if strings.Contains(prompt, pattern) {
			return result, nil
		}
	}

	// Return default or generate one
	if m.defaultConvResult != nil {
		return m.defaultConvResult, nil
	}

	// Create a minimal conversation result
	newState := workflow.NewConversationState("")
	if state != nil {
		// Copy existing turns
		for i := range state.Turns {
			if err := newState.AddTurn(&state.Turns[i]); err != nil {
				return nil, err
			}
		}
	}
	// Add user message and assistant response
	if err := newState.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, prompt)); err != nil {
		return nil, err
	}
	if err := newState.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "mock conversation response")); err != nil {
		return nil, err
	}

	return &workflow.ConversationResult{
		State:        newState,
		Output:       "mock conversation response",
		TokensInput:  50,
		TokensOutput: 30,
		TokensTotal:  80,
	}, nil
}

// Name implements ports.AgentProvider.
func (m *MockProvider) Name() string {
	return m.name
}

// Validate implements ports.AgentProvider.
func (m *MockProvider) Validate() error {
	return m.validateError
}

// Calls returns all recorded calls.
func (m *MockProvider) Calls() []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MockCall{}, m.calls...)
}

// CallCount returns the number of calls made.
func (m *MockProvider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// ExecuteCallCount returns the number of Execute calls.
func (m *MockProvider) ExecuteCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, c := range m.calls {
		if c.Method == "Execute" {
			count++
		}
	}
	return count
}

// ConversationCallCount returns the number of ExecuteConversation calls.
func (m *MockProvider) ConversationCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, c := range m.calls {
		if c.Method == "ExecuteConversation" {
			count++
		}
	}
	return count
}

// Reset clears all recorded calls.
func (m *MockProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]MockCall, 0)
}

// cloneResult creates a copy of the result with updated timestamps.
func (m *MockProvider) cloneResult(r *workflow.AgentResult) *workflow.AgentResult {
	return &workflow.AgentResult{
		Provider:    r.Provider,
		Output:      r.Output,
		Response:    r.Response,
		Tokens:      r.Tokens,
		StartedAt:   time.Now().Add(-m.delay),
		CompletedAt: time.Now(),
		Error:       r.Error,
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
