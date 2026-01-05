package agents

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// Component: agent_providers
// Feature: 39

// mockProvider is a test implementation of AgentProvider
type mockProvider struct {
	name string
}

func (m *mockProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	return &workflow.AgentResult{
		Provider: m.name,
		Output:   "mock output",
	}, nil
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
	assert.Len(t, list, 4)
	assert.Contains(t, list, "claude")
	assert.Contains(t, list, "codex")
	assert.Contains(t, list, "gemini")
	assert.Contains(t, list, "opencode")
}

func TestAgentRegistry_RegisterDefaults_EachProviderRetrievable(t *testing.T) {
	registry := NewAgentRegistry()
	_ = registry.RegisterDefaults()

	tests := []string{"claude", "codex", "gemini", "opencode"}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			provider, err := registry.Get(name)

			require.NoError(t, err)
			require.NotNil(t, provider)
			assert.Equal(t, name, provider.Name())
		})
	}
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

func TestAgentRegistry_CustomProviderIntegration(t *testing.T) {
	registry := NewAgentRegistry()
	_ = registry.RegisterDefaults()

	// Add custom provider
	customProvider := NewCustomProvider("my-custom", "echo {{.prompt}}")
	err := registry.Register(customProvider)

	require.NoError(t, err)

	// Verify it's in the list
	list := registry.List()
	assert.Len(t, list, 5) // 4 defaults + 1 custom
	assert.Contains(t, list, "my-custom")

	// Verify it's retrievable
	retrieved, err := registry.Get("my-custom")
	require.NoError(t, err)
	assert.Equal(t, "my-custom", retrieved.Name())
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
