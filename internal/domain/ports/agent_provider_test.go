package ports_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// Component: agent_provider_port
// Feature: 39

// mockAgentProvider is a test implementation of AgentProvider interface
type mockAgentProvider struct {
	name                      string
	executeFunc               func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error)
	executeConversationFunc   func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error)
	validateFunc              func() error
	executeCalled             int
	executeConversationCalled int
}

func newMockAgentProvider(name string) *mockAgentProvider {
	return &mockAgentProvider{
		name: name,
		executeFunc: func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
			result := workflow.NewAgentResult(name)
			result.Output = "mock output"
			result.CompletedAt = time.Now()
			return result, nil
		},
		executeConversationFunc: func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
			return nil, errors.New("not implemented")
		},
		validateFunc: func() error {
			return nil
		},
	}
}

func (m *mockAgentProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	m.executeCalled++
	return m.executeFunc(ctx, prompt, options)
}

func (m *mockAgentProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	m.executeConversationCalled++
	return m.executeConversationFunc(ctx, state, prompt, options)
}

func (m *mockAgentProvider) Name() string {
	return m.name
}

func (m *mockAgentProvider) Validate() error {
	return m.validateFunc()
}

// mockAgentRegistry is a test implementation of AgentRegistry interface
type mockAgentRegistry struct {
	providers map[string]ports.AgentProvider
}

func newMockAgentRegistry() *mockAgentRegistry {
	return &mockAgentRegistry{
		providers: make(map[string]ports.AgentProvider),
	}
}

func (m *mockAgentRegistry) Register(provider ports.AgentProvider) error {
	if _, exists := m.providers[provider.Name()]; exists {
		return errors.New("provider already registered: " + provider.Name())
	}
	m.providers[provider.Name()] = provider
	return nil
}

func (m *mockAgentRegistry) Get(name string) (ports.AgentProvider, error) {
	provider, ok := m.providers[name]
	if !ok {
		return nil, errors.New("provider not found: " + name)
	}
	return provider, nil
}

func (m *mockAgentRegistry) List() []string {
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

func (m *mockAgentRegistry) Has(name string) bool {
	_, ok := m.providers[name]
	return ok
}

func TestAgentProviderInterface(t *testing.T) {
	var _ ports.AgentProvider = (*mockAgentProvider)(nil)
}

func TestAgentRegistryInterface(t *testing.T) {
	var _ ports.AgentRegistry = (*mockAgentRegistry)(nil)
}

func TestAgentProvider_Execute_HappyPath(t *testing.T) {
	provider := newMockAgentProvider("claude")
	ctx := context.Background()
	prompt := "Analyze this code"
	options := map[string]any{
		"model":      "claude-sonnet-4",
		"max_tokens": 4096,
	}

	result, err := provider.Execute(ctx, prompt, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Provider != "claude" {
		t.Errorf("expected provider 'claude', got '%s'", result.Provider)
	}
	if result.Output != "mock output" {
		t.Errorf("expected output 'mock output', got '%s'", result.Output)
	}
	if provider.executeCalled != 1 {
		t.Errorf("expected Execute to be called once, got %d", provider.executeCalled)
	}
}

func TestAgentProvider_Name_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
	}{
		{"claude provider", "claude"},
		{"codex provider", "codex"},
		{"gemini provider", "gemini"},
		{"opencode provider", "opencode"},
		{"custom provider", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockAgentProvider(tt.providerName)

			name := provider.Name()
			if name != tt.providerName {
				t.Errorf("expected name '%s', got '%s'", tt.providerName, name)
			}
		})
	}
}

func TestAgentProvider_Validate_HappyPath(t *testing.T) {
	provider := newMockAgentProvider("claude")

	err := provider.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentProvider_Execute_EmptyPrompt(t *testing.T) {
	provider := newMockAgentProvider("claude")
	ctx := context.Background()
	prompt := ""
	options := map[string]any{}

	result, err := provider.Execute(ctx, prompt, options)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestAgentProvider_Execute_NilOptions(t *testing.T) {
	provider := newMockAgentProvider("claude")
	ctx := context.Background()
	prompt := "test prompt"

	result, err := provider.Execute(ctx, prompt, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestAgentProvider_Execute_LargePrompt(t *testing.T) {
	provider := newMockAgentProvider("claude")
	ctx := context.Background()
	largePrompt := string(make([]byte, 10240))
	options := map[string]any{}

	result, err := provider.Execute(ctx, largePrompt, options)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestAgentProvider_Execute_ContextCancellation(t *testing.T) {
	provider := newMockAgentProvider("claude")
	provider.executeFunc = func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			result := workflow.NewAgentResult("claude")
			result.Output = "completed"
			result.CompletedAt = time.Now()
			return result, nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := provider.Execute(ctx, "test", nil)

	if err == nil {
		t.Error("expected error from cancelled context")
	}
	if result != nil {
		t.Error("expected nil result from cancelled context")
	}
}

func TestAgentProvider_Execute_ExecutionError(t *testing.T) {
	provider := newMockAgentProvider("claude")
	expectedErr := errors.New("execution failed")
	provider.executeFunc = func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return nil, expectedErr
	}

	result, err := provider.Execute(context.Background(), "test", nil)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if err.Error() != "execution failed" {
		t.Errorf("expected error 'execution failed', got '%v'", err)
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

func TestAgentProvider_Execute_PartialResult(t *testing.T) {
	provider := newMockAgentProvider("claude")
	expectedErr := errors.New("partial execution")
	provider.executeFunc = func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		// Return partial result with error
		result := workflow.NewAgentResult("claude")
		result.Output = "partial output"
		result.Error = expectedErr
		result.CompletedAt = time.Now()
		return result, expectedErr
	}

	result, err := provider.Execute(context.Background(), "test", nil)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result == nil {
		t.Fatal("expected partial result, got nil")
	}
	if result.Output != "partial output" {
		t.Errorf("expected output 'partial output', got '%s'", result.Output)
	}
}

func TestAgentProvider_Validate_ValidationError(t *testing.T) {
	provider := newMockAgentProvider("claude")
	expectedErr := errors.New("binary not found")
	provider.validateFunc = func() error {
		return expectedErr
	}

	err := provider.Validate()

	if err == nil {
		t.Error("expected validation error, got nil")
	}
	if err.Error() != "binary not found" {
		t.Errorf("expected error 'binary not found', got '%v'", err)
	}
}

func TestAgentRegistry_Register_HappyPath(t *testing.T) {
	registry := newMockAgentRegistry()
	provider := newMockAgentProvider("claude")

	err := registry.Register(provider)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !registry.Has("claude") {
		t.Error("expected provider to be registered")
	}
}

func TestAgentRegistry_Get_HappyPath(t *testing.T) {
	registry := newMockAgentRegistry()
	provider := newMockAgentProvider("claude")
	registry.Register(provider)

	retrieved, err := registry.Get("claude")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected provider, got nil")
	}
	if retrieved.Name() != "claude" {
		t.Errorf("expected provider name 'claude', got '%s'", retrieved.Name())
	}
}

func TestAgentRegistry_List_HappyPath(t *testing.T) {
	registry := newMockAgentRegistry()
	registry.Register(newMockAgentProvider("claude"))
	registry.Register(newMockAgentProvider("codex"))
	registry.Register(newMockAgentProvider("gemini"))

	names := registry.List()

	if len(names) != 3 {
		t.Errorf("expected 3 providers, got %d", len(names))
	}
}

func TestAgentRegistry_Has_HappyPath(t *testing.T) {
	registry := newMockAgentRegistry()
	registry.Register(newMockAgentProvider("claude"))

	if !registry.Has("claude") {
		t.Error("expected Has('claude') to return true")
	}
	if registry.Has("nonexistent") {
		t.Error("expected Has('nonexistent') to return false")
	}
}

func TestAgentRegistry_List_EmptyRegistry(t *testing.T) {
	registry := newMockAgentRegistry()

	names := registry.List()

	if len(names) != 0 {
		t.Errorf("expected 0 providers, got %d", len(names))
	}
}

func TestAgentRegistry_Has_EmptyRegistry(t *testing.T) {
	registry := newMockAgentRegistry()

	has := registry.Has("claude")

	if has {
		t.Error("expected Has to return false for empty registry")
	}
}

func TestAgentRegistry_Register_MultipleProviders(t *testing.T) {
	registry := newMockAgentRegistry()
	providers := []string{"claude", "codex", "gemini", "opencode", "custom"}

	for _, name := range providers {
		err := registry.Register(newMockAgentProvider(name))
		if err != nil {
			t.Errorf("unexpected error registering %s: %v", name, err)
		}
	}

	names := registry.List()
	if len(names) != len(providers) {
		t.Errorf("expected %d providers, got %d", len(providers), len(names))
	}
}

func TestAgentRegistry_Register_DuplicateProvider(t *testing.T) {
	registry := newMockAgentRegistry()
	provider1 := newMockAgentProvider("claude")
	provider2 := newMockAgentProvider("claude")
	registry.Register(provider1)

	err := registry.Register(provider2)

	if err == nil {
		t.Error("expected error when registering duplicate provider")
	}
	if err.Error() != "provider already registered: claude" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAgentRegistry_Get_NotFound(t *testing.T) {
	registry := newMockAgentRegistry()

	provider, err := registry.Get("nonexistent")

	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
	if provider != nil {
		t.Error("expected nil provider for nonexistent name")
	}
	if err.Error() != "provider not found: nonexistent" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAgentRegistry_Get_AfterRegisterError(t *testing.T) {
	registry := newMockAgentRegistry()
	provider := newMockAgentProvider("claude")
	registry.Register(provider)

	// Try to register duplicate (should fail)
	registry.Register(newMockAgentProvider("claude"))

	retrieved, err := registry.Get("claude")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected provider, got nil")
	}
	if retrieved.Name() != "claude" {
		t.Errorf("expected provider name 'claude', got '%s'", retrieved.Name())
	}
}

func TestAgentProvider_FullWorkflow(t *testing.T) {
	registry := newMockAgentRegistry()

	// Register multiple providers
	providers := []string{"claude", "codex", "gemini"}
	for _, name := range providers {
		registry.Register(newMockAgentProvider(name))
	}

	for _, name := range providers {
		provider, err := registry.Get(name)
		if err != nil {
			t.Fatalf("failed to get provider %s: %v", name, err)
		}

		// Validate provider
		if err := provider.Validate(); err != nil {
			t.Errorf("validation failed for %s: %v", name, err)
		}

		// Execute provider
		result, err := provider.Execute(context.Background(), "test prompt", nil)
		if err != nil {
			t.Errorf("execution failed for %s: %v", name, err)
		}
		if result == nil {
			t.Errorf("expected result for %s, got nil", name)
			continue
		}
		if result.Provider != name {
			t.Errorf("expected provider '%s', got '%s'", name, result.Provider)
		}
	}
}

// Component: agent_provider_extension
// Feature: F033

// TestAgentProvider_ExecuteConversation_HappyPath tests normal conversation execution
func TestAgentProvider_ExecuteConversation_HappyPath(t *testing.T) {
	provider := newMockAgentProvider("claude")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		result := workflow.NewConversationResult("claude")
		result.Output = "Assistant response"
		result.TokensInput = 100
		result.TokensOutput = 50
		result.TokensTotal = 150
		result.CompletedAt = time.Now()
		return result, nil
	}

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful assistant")
	prompt := "What is Go?"
	options := map[string]any{
		"model":      "claude-sonnet-4",
		"max_tokens": 4096,
	}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Provider != "claude" {
		t.Errorf("expected provider 'claude', got '%s'", result.Provider)
	}
	if result.Output != "Assistant response" {
		t.Errorf("expected output 'Assistant response', got '%s'", result.Output)
	}
	if result.TokensInput != 100 {
		t.Errorf("expected tokens_input 100, got %d", result.TokensInput)
	}
	if result.TokensOutput != 50 {
		t.Errorf("expected tokens_output 50, got %d", result.TokensOutput)
	}
	if result.TokensTotal != 150 {
		t.Errorf("expected tokens_total 150, got %d", result.TokensTotal)
	}
	if provider.executeConversationCalled != 1 {
		t.Errorf("expected ExecuteConversation to be called once, got %d", provider.executeConversationCalled)
	}
}

// TestAgentProvider_ExecuteConversation_WithConversationHistory tests execution with existing turns
func TestAgentProvider_ExecuteConversation_WithConversationHistory(t *testing.T) {
	provider := newMockAgentProvider("claude")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		result := workflow.NewConversationResult("claude")
		result.State = state
		result.Output = "Response based on history"
		result.CompletedAt = time.Now()
		return result, nil
	}

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful assistant")

	// Add existing turns to conversation history
	_ = state.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "What is Go?"))
	_ = state.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "Go is a programming language."))

	prompt := "Tell me more about it"
	options := map[string]any{}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.State == nil {
		t.Error("expected state to be preserved, got nil")
	}
	if result.Output != "Response based on history" {
		t.Errorf("expected contextual response, got '%s'", result.Output)
	}
}

// TestAgentProvider_ExecuteConversation_WithOptions tests conversation with various options
func TestAgentProvider_ExecuteConversation_WithOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name: "with model option",
			options: map[string]any{
				"model": "claude-opus-4",
			},
		},
		{
			name: "with max_tokens option",
			options: map[string]any{
				"max_tokens": 2048,
			},
		},
		{
			name: "with temperature option",
			options: map[string]any{
				"temperature": 0.7,
			},
		},
		{
			name: "with multiple options",
			options: map[string]any{
				"model":       "claude-sonnet-4",
				"max_tokens":  4096,
				"temperature": 0.5,
			},
		},
		{
			name:    "with empty options",
			options: map[string]any{},
		},
		{
			name:    "with nil options",
			options: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockAgentProvider("claude")
			var capturedOptions map[string]any
			provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
				capturedOptions = options
				result := workflow.NewConversationResult("claude")
				result.Output = "Response"
				result.CompletedAt = time.Now()
				return result, nil
			}

			ctx := context.Background()
			state := workflow.NewConversationState("System prompt")
			prompt := "Test prompt"

			result, err := provider.ExecuteConversation(ctx, state, prompt, tt.options)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("expected result, got nil")
			}
			// Verify options were passed through
			if tt.options == nil && capturedOptions != nil {
				t.Error("expected nil options, got non-nil")
			}
			if tt.options != nil && len(tt.options) != len(capturedOptions) {
				t.Errorf("expected %d options, got %d", len(tt.options), len(capturedOptions))
			}
		})
	}
}

// TestAgentProvider_ExecuteConversation_EmptyPrompt tests conversation with empty prompt
func TestAgentProvider_ExecuteConversation_EmptyPrompt(t *testing.T) {
	provider := newMockAgentProvider("claude")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		if prompt == "" {
			return nil, errors.New("prompt cannot be empty")
		}
		result := workflow.NewConversationResult("claude")
		result.CompletedAt = time.Now()
		return result, nil
	}

	ctx := context.Background()
	state := workflow.NewConversationState("System prompt")
	prompt := ""
	options := map[string]any{}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)

	if err == nil {
		t.Error("expected error for empty prompt")
	}
	if result != nil {
		t.Error("expected nil result for empty prompt")
	}
	if err != nil && err.Error() != "prompt cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestAgentProvider_ExecuteConversation_NilState tests conversation with nil state
func TestAgentProvider_ExecuteConversation_NilState(t *testing.T) {
	provider := newMockAgentProvider("claude")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		if state == nil {
			return nil, errors.New("conversation state cannot be nil")
		}
		result := workflow.NewConversationResult("claude")
		result.CompletedAt = time.Now()
		return result, nil
	}

	ctx := context.Background()
	var state *workflow.ConversationState // nil
	prompt := "What is Go?"
	options := map[string]any{}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)

	if err == nil {
		t.Error("expected error for nil conversation state")
	}
	if result != nil {
		t.Error("expected nil result for nil state")
	}
}

// TestAgentProvider_ExecuteConversation_EmptyState tests conversation with empty state (no turns)
func TestAgentProvider_ExecuteConversation_EmptyState(t *testing.T) {
	provider := newMockAgentProvider("claude")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		result := workflow.NewConversationResult("claude")
		result.Output = "First response"
		result.State = state
		result.CompletedAt = time.Now()
		return result, nil
	}

	ctx := context.Background()
	state := workflow.NewConversationState("") // Empty system prompt, no turns
	prompt := "Hello"
	options := map[string]any{}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Output != "First response" {
		t.Errorf("expected output 'First response', got '%s'", result.Output)
	}
}

// TestAgentProvider_ExecuteConversation_LargeConversationHistory tests with many turns
func TestAgentProvider_ExecuteConversation_LargeConversationHistory(t *testing.T) {
	provider := newMockAgentProvider("claude")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		result := workflow.NewConversationResult("claude")
		result.State = state
		result.Output = "Response to turn 100"
		result.CompletedAt = time.Now()
		return result, nil
	}

	ctx := context.Background()
	state := workflow.NewConversationState("System prompt")

	// Add 99 turns (simulating a long conversation)
	for i := 0; i < 99; i++ {
		if i%2 == 0 {
			_ = state.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "Question"))
		} else {
			_ = state.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "Answer"))
		}
	}

	prompt := "100th question"
	options := map[string]any{}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.State == nil {
		t.Error("expected state to be preserved")
	}
}

// TestAgentProvider_ExecuteConversation_LongPrompt tests with very long prompt
func TestAgentProvider_ExecuteConversation_LongPrompt(t *testing.T) {
	provider := newMockAgentProvider("claude")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		result := workflow.NewConversationResult("claude")
		result.Output = "Response to long prompt"
		result.CompletedAt = time.Now()
		return result, nil
	}

	ctx := context.Background()
	state := workflow.NewConversationState("System prompt")

	// Create a very long prompt (10KB)
	longPrompt := ""
	for i := 0; i < 1000; i++ {
		longPrompt += "This is a very long prompt. "
	}

	options := map[string]any{}

	result, err := provider.ExecuteConversation(ctx, state, longPrompt, options)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

// TestAgentProvider_ExecuteConversation_ProviderError tests error from provider
func TestAgentProvider_ExecuteConversation_ProviderError(t *testing.T) {
	provider := newMockAgentProvider("claude")
	expectedError := errors.New("provider execution failed")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		return nil, expectedError
	}

	ctx := context.Background()
	state := workflow.NewConversationState("System prompt")
	prompt := "What is Go?"
	options := map[string]any{}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)

	if err == nil {
		t.Error("expected error from provider")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
	if err != expectedError {
		t.Errorf("expected error '%v', got '%v'", expectedError, err)
	}
}

// TestAgentProvider_ExecuteConversation_ContextCanceled tests canceled context
func TestAgentProvider_ExecuteConversation_ContextCanceled(t *testing.T) {
	provider := newMockAgentProvider("claude")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		// Check for context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		result := workflow.NewConversationResult("claude")
		result.CompletedAt = time.Now()
		return result, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	state := workflow.NewConversationState("System prompt")
	prompt := "What is Go?"
	options := map[string]any{}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)

	if err == nil {
		t.Error("expected error for canceled context")
	}
	if result != nil {
		t.Error("expected nil result for canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

// TestAgentProvider_ExecuteConversation_ContextTimeout tests timeout
func TestAgentProvider_ExecuteConversation_ContextTimeout(t *testing.T) {
	provider := newMockAgentProvider("claude")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		// Simulate work that respects context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			result := workflow.NewConversationResult("claude")
			result.CompletedAt = time.Now()
			return result, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	state := workflow.NewConversationState("System prompt")
	prompt := "What is Go?"
	options := map[string]any{}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)

	if err == nil {
		t.Error("expected timeout error")
	}
	if result != nil {
		t.Error("expected nil result on timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded error, got %v", err)
	}
}

// TestAgentProvider_ExecuteConversation_InvalidOptions tests with invalid options
func TestAgentProvider_ExecuteConversation_InvalidOptions(t *testing.T) {
	tests := []struct {
		name        string
		options     map[string]any
		expectedErr string
	}{
		{
			name: "negative max_tokens",
			options: map[string]any{
				"max_tokens": -100,
			},
			expectedErr: "max_tokens must be non-negative",
		},
		{
			name: "temperature out of range",
			options: map[string]any{
				"temperature": 2.0,
			},
			expectedErr: "temperature must be between 0 and 1",
		},
		{
			name: "invalid model",
			options: map[string]any{
				"model": "",
			},
			expectedErr: "model cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockAgentProvider("claude")
			provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
				// Validate options
				if maxTokens, ok := options["max_tokens"].(int); ok && maxTokens < 0 {
					return nil, errors.New("max_tokens must be non-negative")
				}
				if temp, ok := options["temperature"].(float64); ok && (temp < 0 || temp > 1) {
					return nil, errors.New("temperature must be between 0 and 1")
				}
				if model, ok := options["model"].(string); ok && model == "" {
					return nil, errors.New("model cannot be empty")
				}
				result := workflow.NewConversationResult("claude")
				result.CompletedAt = time.Now()
				return result, nil
			}

			ctx := context.Background()
			state := workflow.NewConversationState("System prompt")
			prompt := "What is Go?"

			result, err := provider.ExecuteConversation(ctx, state, prompt, tt.options)

			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
			if result != nil {
				t.Error("expected nil result on validation error")
			}
			if err != nil && err.Error() != tt.expectedErr {
				t.Errorf("expected error '%s', got '%v'", tt.expectedErr, err)
			}
		})
	}
}

// TestAgentProvider_ExecuteConversation_MultipleProviders tests different providers
func TestAgentProvider_ExecuteConversation_MultipleProviders(t *testing.T) {
	providers := []string{"claude", "codex", "gemini", "opencode"}

	for _, name := range providers {
		t.Run(name, func(t *testing.T) {
			provider := newMockAgentProvider(name)
			provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
				result := workflow.NewConversationResult(name)
				result.Output = "Response from " + name
				result.CompletedAt = time.Now()
				return result, nil
			}

			ctx := context.Background()
			state := workflow.NewConversationState("System prompt")
			prompt := "Test"
			options := map[string]any{}

			result, err := provider.ExecuteConversation(ctx, state, prompt, options)
			if err != nil {
				t.Errorf("unexpected error for %s: %v", name, err)
			}
			if result == nil {
				t.Fatalf("expected result for %s, got nil", name)
			}
			if result.Provider != name {
				t.Errorf("expected provider '%s', got '%s'", name, result.Provider)
			}
		})
	}
}

// TestAgentProvider_ExecuteConversation_WithRegistry tests conversation through registry
func TestAgentProvider_ExecuteConversation_WithRegistry(t *testing.T) {
	registry := newMockAgentRegistry()

	provider := newMockAgentProvider("claude")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		result := workflow.NewConversationResult("claude")
		result.Output = "Response"
		result.CompletedAt = time.Now()
		return result, nil
	}

	registry.Register(provider)

	retrieved, err := registry.Get("claude")
	if err != nil {
		t.Fatalf("failed to get provider: %v", err)
	}

	ctx := context.Background()
	state := workflow.NewConversationState("System prompt")
	result, err := retrieved.ExecuteConversation(ctx, state, "Test", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Provider != "claude" {
		t.Errorf("expected provider 'claude', got '%s'", result.Provider)
	}
}

// TestAgentProvider_ExecuteConversation_TokenCounting tests token usage tracking
func TestAgentProvider_ExecuteConversation_TokenCounting(t *testing.T) {
	provider := newMockAgentProvider("claude")
	provider.executeConversationFunc = func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		result := workflow.NewConversationResult("claude")
		result.Output = "Response"
		result.TokensInput = 250
		result.TokensOutput = 100
		result.TokensTotal = 350
		result.TokensEstimated = false
		result.CompletedAt = time.Now()
		return result, nil
	}

	ctx := context.Background()
	state := workflow.NewConversationState("System prompt")
	prompt := "What is Go?"
	options := map[string]any{}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.TokensInput != 250 {
		t.Errorf("expected tokens_input 250, got %d", result.TokensInput)
	}
	if result.TokensOutput != 100 {
		t.Errorf("expected tokens_output 100, got %d", result.TokensOutput)
	}
	if result.TokensTotal != 350 {
		t.Errorf("expected tokens_total 350, got %d", result.TokensTotal)
	}
	if result.TokensEstimated {
		t.Error("expected TokensEstimated to be false")
	}
}

// TestAgentProvider_ExecuteConversation_NotImplementedError tests stub behavior
func TestAgentProvider_ExecuteConversation_NotImplementedError(t *testing.T) {
	provider := newMockAgentProvider("claude")
	// Don't override executeConversationFunc - use default stub

	ctx := context.Background()
	state := workflow.NewConversationState("System prompt")
	prompt := "What is Go?"
	options := map[string]any{}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)

	if err == nil {
		t.Error("expected 'not implemented' error from stub")
	}
	if result != nil {
		t.Error("expected nil result from stub")
	}
	if err != nil && err.Error() != "not implemented" {
		t.Errorf("expected 'not implemented' error, got '%v'", err)
	}
	if provider.executeConversationCalled != 1 {
		t.Errorf("expected method to be called once, got %d", provider.executeConversationCalled)
	}
}
