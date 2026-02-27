package agents

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: agent_providers
// Feature: 39

// mockProvider is a test implementation of AgentProvider
type mockProvider struct {
	name string
}

// Component: T002
// Feature: C022

func TestAgentRegistry_InterfaceCompliance(t *testing.T) {
	// Verify AgentRegistry implements ports.AgentRegistry
	var _ ports.AgentRegistry = (*AgentRegistry)(nil)
}

func (m *mockProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	return &workflow.AgentResult{
		Provider: m.name,
		Output:   "mock output",
	}, nil
}

func (m *mockProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	return nil, nil
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Validate() error {
	return nil
}

func TestNewAgentRegistry(t *testing.T) {
	registry := NewAgentRegistry()

	require.NotNil(t, registry)
	assert.NotNil(t, registry.providers)
	assert.Empty(t, registry.providers)
}

func TestAgentRegistry_RegisterProvider_HappyPath(t *testing.T) {
	registry := NewAgentRegistry()
	provider := &mockProvider{name: "test"}

	err := registry.Register(provider)

	assert.NoError(t, err)

	// Verify it was registered
	registered, err := registry.Get("test")
	require.NoError(t, err)
	assert.Equal(t, provider, registered)
}

func TestAgentRegistry_RegisterProvider_MultipleProviders(t *testing.T) {
	registry := NewAgentRegistry()
	provider1 := &mockProvider{name: "provider1"}
	provider2 := &mockProvider{name: "provider2"}
	provider3 := &mockProvider{name: "provider3"}

	err1 := registry.Register(provider1)
	err2 := registry.Register(provider2)
	err3 := registry.Register(provider3)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)

	// Verify all registered
	list := registry.List()
	assert.Len(t, list, 3)
	assert.Contains(t, list, "provider1")
	assert.Contains(t, list, "provider2")
	assert.Contains(t, list, "provider3")
}

func TestAgentRegistry_RegisterProvider_DuplicateName(t *testing.T) {
	registry := NewAgentRegistry()
	provider1 := &mockProvider{name: "duplicate"}
	provider2 := &mockProvider{name: "duplicate"}

	err1 := registry.Register(provider1)
	assert.NoError(t, err1)

	err2 := registry.Register(provider2)
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "already registered")
	assert.Contains(t, err2.Error(), "duplicate")
}

func TestAgentRegistry_Get_Success(t *testing.T) {
	registry := NewAgentRegistry()
	provider := &mockProvider{name: "test"}
	_ = registry.Register(provider)

	retrieved, err := registry.Get("test")

	assert.NoError(t, err)
	assert.Equal(t, provider, retrieved)
	assert.Equal(t, "test", retrieved.Name())
}

func TestAgentRegistry_Get_NotFound(t *testing.T) {
	registry := NewAgentRegistry()

	retrieved, err := registry.Get("nonexistent")

	assert.Error(t, err)
	assert.Nil(t, retrieved)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestAgentRegistry_Get_EmptyName(t *testing.T) {
	registry := NewAgentRegistry()

	retrieved, err := registry.Get("")

	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestAgentRegistry_Has_Found(t *testing.T) {
	registry := NewAgentRegistry()
	provider := &mockProvider{name: "test"}
	_ = registry.Register(provider)

	exists := registry.Has("test")

	assert.True(t, exists)
}

func TestAgentRegistry_Has_NotFound(t *testing.T) {
	registry := NewAgentRegistry()

	exists := registry.Has("nonexistent")

	assert.False(t, exists)
}

func TestAgentRegistry_Has_EmptyName(t *testing.T) {
	registry := NewAgentRegistry()

	exists := registry.Has("")

	assert.False(t, exists)
}

func TestAgentRegistry_Has_MultipleProviders(t *testing.T) {
	registry := NewAgentRegistry()
	_ = registry.Register(&mockProvider{name: "provider1"})
	_ = registry.Register(&mockProvider{name: "provider2"})
	_ = registry.Register(&mockProvider{name: "provider3"})

	assert.True(t, registry.Has("provider1"))
	assert.True(t, registry.Has("provider2"))
	assert.True(t, registry.Has("provider3"))
	assert.False(t, registry.Has("provider4"))
}

func TestAgentRegistry_Has_CaseSensitive(t *testing.T) {
	registry := NewAgentRegistry()
	_ = registry.Register(&mockProvider{name: "TestProvider"})

	assert.True(t, registry.Has("TestProvider"))
	assert.False(t, registry.Has("testprovider"))
	assert.False(t, registry.Has("TESTPROVIDER"))
	assert.False(t, registry.Has("testProvider"))
}

func TestAgentRegistry_Has_ThreadSafety(t *testing.T) {
	registry := NewAgentRegistry()
	_ = registry.Register(&mockProvider{name: "test"})

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			exists := registry.Has("test")
			assert.True(t, exists)
			notExists := registry.Has("nonexistent")
			assert.False(t, notExists)
		}()
	}

	wg.Wait()
}

func TestAgentRegistry_Has_ConcurrentWithRegister(t *testing.T) {
	registry := NewAgentRegistry()

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Register providers
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = registry.Register(&mockProvider{name: fmt.Sprintf("provider%d", i)})
		}
	}()

	// Goroutine 2: Check existence
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = registry.Has(fmt.Sprintf("provider%d", i))
		}
	}()

	wg.Wait()

	// Verify all providers are registered
	for i := 0; i < 50; i++ {
		assert.True(t, registry.Has(fmt.Sprintf("provider%d", i)))
	}
}

func TestAgentRegistry_List_Empty(t *testing.T) {
	registry := NewAgentRegistry()

	list := registry.List()

	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestAgentRegistry_List_Multiple(t *testing.T) {
	registry := NewAgentRegistry()
	_ = registry.Register(&mockProvider{name: "provider1"})
	_ = registry.Register(&mockProvider{name: "provider2"})
	_ = registry.Register(&mockProvider{name: "provider3"})

	list := registry.List()

	assert.Len(t, list, 3)
	assert.Contains(t, list, "provider1")
	assert.Contains(t, list, "provider2")
	assert.Contains(t, list, "provider3")
}

func TestAgentRegistry_RegisterDefaults(t *testing.T) {
	registry := NewAgentRegistry()

	err := registry.RegisterDefaults()

	assert.NoError(t, err)

	// Verify default providers are registered
	list := registry.List()
	assert.Len(t, list, 5)
	assert.Contains(t, list, "claude")
	assert.Contains(t, list, "codex")
	assert.Contains(t, list, "gemini")
	assert.Contains(t, list, "openai_compatible")
	assert.Contains(t, list, "opencode")
}

func TestAgentRegistry_RegisterDefaults_EachProviderRetrievable(t *testing.T) {
	registry := NewAgentRegistry()
	_ = registry.RegisterDefaults()

	tests := []string{"claude", "codex", "gemini", "openai_compatible", "opencode"}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			provider, err := registry.Get(name)

			require.NoError(t, err)
			require.NotNil(t, provider)
			assert.Equal(t, name, provider.Name())
		})
	}
}

func TestAgentRegistry_RegisterDefaults_OpenAICompatibleRegistered(t *testing.T) {
	registry := NewAgentRegistry()

	err := registry.RegisterDefaults()

	require.NoError(t, err)
	assert.True(t, registry.Has("openai_compatible"))

	provider, err := registry.Get("openai_compatible")
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.Equal(t, "openai_compatible", provider.Name())
}

func TestAgentRegistry_RegisterDefaults_Twice(t *testing.T) {
	registry := NewAgentRegistry()

	err1 := registry.RegisterDefaults()
	assert.NoError(t, err1)

	err2 := registry.RegisterDefaults()
	assert.Error(t, err2, "Should fail when registering defaults twice")
	assert.Contains(t, err2.Error(), "already registered")
}

func TestAgentRegistry_ThreadSafety_ConcurrentRegister(t *testing.T) {
	registry := NewAgentRegistry()
	done := make(chan bool)

	// Register providers concurrently
	for i := 0; i < 10; i++ {
		go func(idx int) {
			provider := &mockProvider{name: "provider"}
			_ = registry.Register(provider)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have exactly one provider registered (first one wins)
	list := registry.List()
	assert.Len(t, list, 1)
}

func TestAgentRegistry_ThreadSafety_ConcurrentGetAndRegister(t *testing.T) {
	registry := NewAgentRegistry()
	_ = registry.Register(&mockProvider{name: "test"})
	done := make(chan bool)

	// Mix of reads and writes
	for i := 0; i < 20; i++ {
		go func(idx int) {
			if idx%2 == 0 {
				// Read
				_, _ = registry.Get("test")
			} else {
				// Write (will fail but tests concurrency)
				_ = registry.Register(&mockProvider{name: "test"})
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Registry should still be consistent
	provider, err := registry.Get("test")
	assert.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestAgentRegistry_ThreadSafety_ConcurrentList(t *testing.T) {
	registry := NewAgentRegistry()
	_ = registry.Register(&mockProvider{name: "provider1"})
	_ = registry.Register(&mockProvider{name: "provider2"})
	done := make(chan bool)

	// Concurrent list calls
	for i := 0; i < 10; i++ {
		go func() {
			list := registry.List()
			assert.Len(t, list, 2)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestAgentRegistry_OverwritePrevention(t *testing.T) {
	registry := NewAgentRegistry()
	provider1 := &mockProvider{name: "test"}
	provider2 := &mockProvider{name: "test"}

	err1 := registry.Register(provider1)
	require.NoError(t, err1)

	err2 := registry.Register(provider2)
	assert.Error(t, err2)

	// Verify first provider is still registered
	retrieved, err := registry.Get("test")
	require.NoError(t, err)
	assert.Equal(t, "test", retrieved.Name())
}

func TestAgentRegistry_CaseSensitiveNames(t *testing.T) {
	registry := NewAgentRegistry()
	_ = registry.Register(&mockProvider{name: "Provider"})
	_ = registry.Register(&mockProvider{name: "provider"})

	// Both should be registered separately
	list := registry.List()
	assert.Len(t, list, 2)
	assert.Contains(t, list, "Provider")
	assert.Contains(t, list, "provider")
}

// Component: T010
// Feature: C025

func TestAgentRegistry_RegisterDefaults_PartialFailure(t *testing.T) {
	// Pre-register one default provider (e.g., claude), then call RegisterDefaults
	// Verify: returns aggregated error mentioning already-registered provider
	// Verify: other default providers ARE registered (RegisterDefaults continues on error)
	registry := NewAgentRegistry()

	// Pre-register one default provider
	claudeProvider := NewClaudeProvider()
	err := registry.Register(claudeProvider)
	require.NoError(t, err)

	// Call RegisterDefaults - should fail for claude but register others
	err = registry.RegisterDefaults()

	// Should return error mentioning the already-registered provider
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
	assert.Contains(t, err.Error(), "claude")

	// Verify RegisterDefaults continues on error - other providers should be registered
	list := registry.List()
	assert.Len(t, list, 5, "All 5 default providers should be registered (1 pre-existing + 4 new)")
	assert.Contains(t, list, "claude")
	assert.Contains(t, list, "codex")
	assert.Contains(t, list, "gemini")
	assert.Contains(t, list, "openai_compatible")
	assert.Contains(t, list, "opencode")

	// Verify each provider is retrievable
	for _, name := range []string{"claude", "codex", "gemini", "openai_compatible", "opencode"} {
		provider, getErr := registry.Get(name)
		assert.NoError(t, getErr, "Provider %s should be retrievable", name)
		assert.NotNil(t, provider)
		assert.Equal(t, name, provider.Name())
	}
}

func TestAgentRegistry_RegisterDefaults_EmptyRegistry(t *testing.T) {
	// Create fresh registry, call RegisterDefaults
	// Verify: no error returned
	// Verify: all 4 default providers registered (claude, gemini, codex, opencode)
	registry := NewAgentRegistry()

	err := registry.RegisterDefaults()

	// Should succeed without errors
	require.NoError(t, err)

	// Verify all 5 default providers are registered
	list := registry.List()
	assert.Len(t, list, 5)
	assert.Contains(t, list, "claude")
	assert.Contains(t, list, "codex")
	assert.Contains(t, list, "gemini")
	assert.Contains(t, list, "openai_compatible")
	assert.Contains(t, list, "opencode")

	// Verify each provider is retrievable and functional
	for _, name := range []string{"claude", "codex", "gemini", "openai_compatible", "opencode"} {
		provider, getErr := registry.Get(name)
		require.NoError(t, getErr, "Provider %s should be retrievable", name)
		require.NotNil(t, provider)
		assert.Equal(t, name, provider.Name())

		// Verify provider has Has() check
		assert.True(t, registry.Has(name))
	}
}

func TestAgentRegistry_RegisterDefaults_MultiplePreRegistered(t *testing.T) {
	// Edge case: pre-register multiple default providers
	registry := NewAgentRegistry()

	// Pre-register two default providers
	_ = registry.Register(NewClaudeProvider())
	_ = registry.Register(NewGeminiProvider())

	err := registry.RegisterDefaults()

	// Should return aggregated error for both failures
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "claude")
	assert.Contains(t, err.Error(), "gemini")

	// All 5 providers should still be registered
	list := registry.List()
	assert.Len(t, list, 5)
	assert.Contains(t, list, "claude")
	assert.Contains(t, list, "codex")
	assert.Contains(t, list, "gemini")
	assert.Contains(t, list, "openai_compatible")
	assert.Contains(t, list, "opencode")
}

func TestAgentRegistry_RegisterDefaults_AllPreRegistered(t *testing.T) {
	// Edge case: all default providers already registered
	registry := NewAgentRegistry()

	// Register all defaults manually
	err1 := registry.RegisterDefaults()
	require.NoError(t, err1)

	// Try to register defaults again
	err2 := registry.RegisterDefaults()

	// Should fail with aggregated error for all 5 providers
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "claude")
	assert.Contains(t, err2.Error(), "codex")
	assert.Contains(t, err2.Error(), "gemini")
	assert.Contains(t, err2.Error(), "openai_compatible")
	assert.Contains(t, err2.Error(), "opencode")

	// Should still have exactly 5 providers (no duplicates)
	list := registry.List()
	assert.Len(t, list, 5)
}
