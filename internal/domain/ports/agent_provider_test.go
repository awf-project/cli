package ports_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// Component: agent_provider_port
// Feature: 39

// ============================================================================
// Mock Implementations
// ============================================================================

// mockAgentProvider is a test implementation of AgentProvider interface
type mockAgentProvider struct {
	name          string
	executeFunc   func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error)
	validateFunc  func() error
	executeCalled int
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
		validateFunc: func() error {
			return nil
		},
	}
}

func (m *mockAgentProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	m.executeCalled++
	return m.executeFunc(ctx, prompt, options)
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

// ============================================================================
// Interface Compliance Tests
// ============================================================================

func TestAgentProviderInterface(t *testing.T) {
	var _ ports.AgentProvider = (*mockAgentProvider)(nil)
}

func TestAgentRegistryInterface(t *testing.T) {
	var _ ports.AgentRegistry = (*mockAgentRegistry)(nil)
}

// ============================================================================
// AgentProvider Tests - Happy Path
// ============================================================================

func TestAgentProvider_Execute_HappyPath(t *testing.T) {
	// Arrange
	provider := newMockAgentProvider("claude")
	ctx := context.Background()
	prompt := "Analyze this code"
	options := map[string]any{
		"model":      "claude-sonnet-4",
		"max_tokens": 4096,
	}

	// Act
	result, err := provider.Execute(ctx, prompt, options)

	// Assert
	if err != nil {
		t.Errorf("unexpected error: %v", err)
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
			// Arrange
			provider := newMockAgentProvider(tt.providerName)

			// Act
			name := provider.Name()

			// Assert
			if name != tt.providerName {
				t.Errorf("expected name '%s', got '%s'", tt.providerName, name)
			}
		})
	}
}

func TestAgentProvider_Validate_HappyPath(t *testing.T) {
	// Arrange
	provider := newMockAgentProvider("claude")

	// Act
	err := provider.Validate()

	// Assert
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================================
// AgentProvider Tests - Edge Cases
// ============================================================================

func TestAgentProvider_Execute_EmptyPrompt(t *testing.T) {
	// Arrange
	provider := newMockAgentProvider("claude")
	ctx := context.Background()
	prompt := ""
	options := map[string]any{}

	// Act
	result, err := provider.Execute(ctx, prompt, options)

	// Assert - should handle empty prompt gracefully
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestAgentProvider_Execute_NilOptions(t *testing.T) {
	// Arrange
	provider := newMockAgentProvider("claude")
	ctx := context.Background()
	prompt := "test prompt"

	// Act
	result, err := provider.Execute(ctx, prompt, nil)

	// Assert - should handle nil options
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestAgentProvider_Execute_LargePrompt(t *testing.T) {
	// Arrange
	provider := newMockAgentProvider("claude")
	ctx := context.Background()
	// Create a large prompt (10KB)
	largePrompt := string(make([]byte, 10240))
	options := map[string]any{}

	// Act
	result, err := provider.Execute(ctx, largePrompt, options)

	// Assert - should handle large prompts
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestAgentProvider_Execute_ContextCancellation(t *testing.T) {
	// Arrange
	provider := newMockAgentProvider("claude")
	provider.executeFunc = func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		// Check if context is cancelled
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

	// Act
	result, err := provider.Execute(ctx, "test", nil)

	// Assert - should respect context cancellation
	if err == nil {
		t.Error("expected error from cancelled context")
	}
	if result != nil {
		t.Error("expected nil result from cancelled context")
	}
}

// ============================================================================
// AgentProvider Tests - Error Handling
// ============================================================================

func TestAgentProvider_Execute_ExecutionError(t *testing.T) {
	// Arrange
	provider := newMockAgentProvider("claude")
	expectedErr := errors.New("execution failed")
	provider.executeFunc = func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return nil, expectedErr
	}

	// Act
	result, err := provider.Execute(context.Background(), "test", nil)

	// Assert
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
	// Arrange
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

	// Act
	result, err := provider.Execute(context.Background(), "test", nil)

	// Assert
	if err == nil {
		t.Error("expected error, got nil")
	}
	if result == nil {
		t.Fatal("expected partial result, got nil")
	}
	if result.Output != "partial output" {
		t.Errorf("expected output 'partial output', got '%s'", result.Output)
	}
}

func TestAgentProvider_Validate_ValidationError(t *testing.T) {
	// Arrange
	provider := newMockAgentProvider("claude")
	expectedErr := errors.New("binary not found")
	provider.validateFunc = func() error {
		return expectedErr
	}

	// Act
	err := provider.Validate()

	// Assert
	if err == nil {
		t.Error("expected validation error, got nil")
	}
	if err.Error() != "binary not found" {
		t.Errorf("expected error 'binary not found', got '%v'", err)
	}
}

// ============================================================================
// AgentRegistry Tests - Happy Path
// ============================================================================

func TestAgentRegistry_Register_HappyPath(t *testing.T) {
	// Arrange
	registry := newMockAgentRegistry()
	provider := newMockAgentProvider("claude")

	// Act
	err := registry.Register(provider)

	// Assert
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !registry.Has("claude") {
		t.Error("expected provider to be registered")
	}
}

func TestAgentRegistry_Get_HappyPath(t *testing.T) {
	// Arrange
	registry := newMockAgentRegistry()
	provider := newMockAgentProvider("claude")
	registry.Register(provider)

	// Act
	retrieved, err := registry.Get("claude")

	// Assert
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
	// Arrange
	registry := newMockAgentRegistry()
	registry.Register(newMockAgentProvider("claude"))
	registry.Register(newMockAgentProvider("codex"))
	registry.Register(newMockAgentProvider("gemini"))

	// Act
	names := registry.List()

	// Assert
	if len(names) != 3 {
		t.Errorf("expected 3 providers, got %d", len(names))
	}
}

func TestAgentRegistry_Has_HappyPath(t *testing.T) {
	// Arrange
	registry := newMockAgentRegistry()
	registry.Register(newMockAgentProvider("claude"))

	// Act & Assert
	if !registry.Has("claude") {
		t.Error("expected Has('claude') to return true")
	}
	if registry.Has("nonexistent") {
		t.Error("expected Has('nonexistent') to return false")
	}
}

// ============================================================================
// AgentRegistry Tests - Edge Cases
// ============================================================================

func TestAgentRegistry_List_EmptyRegistry(t *testing.T) {
	// Arrange
	registry := newMockAgentRegistry()

	// Act
	names := registry.List()

	// Assert
	if len(names) != 0 {
		t.Errorf("expected 0 providers, got %d", len(names))
	}
}

func TestAgentRegistry_Has_EmptyRegistry(t *testing.T) {
	// Arrange
	registry := newMockAgentRegistry()

	// Act
	has := registry.Has("claude")

	// Assert
	if has {
		t.Error("expected Has to return false for empty registry")
	}
}

func TestAgentRegistry_Register_MultipleProviders(t *testing.T) {
	// Arrange
	registry := newMockAgentRegistry()
	providers := []string{"claude", "codex", "gemini", "opencode", "custom"}

	// Act
	for _, name := range providers {
		err := registry.Register(newMockAgentProvider(name))
		if err != nil {
			t.Errorf("unexpected error registering %s: %v", name, err)
		}
	}

	// Assert
	names := registry.List()
	if len(names) != len(providers) {
		t.Errorf("expected %d providers, got %d", len(providers), len(names))
	}
}

// ============================================================================
// AgentRegistry Tests - Error Handling
// ============================================================================

func TestAgentRegistry_Register_DuplicateProvider(t *testing.T) {
	// Arrange
	registry := newMockAgentRegistry()
	provider1 := newMockAgentProvider("claude")
	provider2 := newMockAgentProvider("claude")
	registry.Register(provider1)

	// Act
	err := registry.Register(provider2)

	// Assert
	if err == nil {
		t.Error("expected error when registering duplicate provider")
	}
	if err.Error() != "provider already registered: claude" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAgentRegistry_Get_NotFound(t *testing.T) {
	// Arrange
	registry := newMockAgentRegistry()

	// Act
	provider, err := registry.Get("nonexistent")

	// Assert
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
	// Arrange
	registry := newMockAgentRegistry()
	provider := newMockAgentProvider("claude")
	registry.Register(provider)

	// Try to register duplicate (should fail)
	registry.Register(newMockAgentProvider("claude"))

	// Act - Get should still work for the first registered provider
	retrieved, err := registry.Get("claude")

	// Assert
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

// ============================================================================
// Integration Tests
// ============================================================================

func TestAgentProvider_FullWorkflow(t *testing.T) {
	// Arrange
	registry := newMockAgentRegistry()

	// Register multiple providers
	providers := []string{"claude", "codex", "gemini"}
	for _, name := range providers {
		registry.Register(newMockAgentProvider(name))
	}

	// Act - Get and execute each provider
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
