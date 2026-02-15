package builders

import (
	"context"
	"testing"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T007
// Feature: C022
//
// Tests verify that ExecutionServiceBuilder.registry field uses ports.AgentRegistry interface
// instead of concrete infrastructure type, ensuring proper Dependency Inversion Principle adherence.

// mockAgentRegistryForT007 implements ports.AgentRegistry interface for testing
// This demonstrates that any implementation of the interface can be used
type mockAgentRegistryForT007 struct {
	registerCalled bool
	getCalled      bool
	hasCalled      bool
	listCalled     bool
	registeredName string
}

func (m *mockAgentRegistryForT007) Register(provider ports.AgentProvider) error {
	m.registerCalled = true
	m.registeredName = provider.Name()
	return nil
}

func (m *mockAgentRegistryForT007) Get(name string) (ports.AgentProvider, error) {
	m.getCalled = true
	return nil, nil
}

func (m *mockAgentRegistryForT007) Has(name string) bool {
	m.hasCalled = true
	return false
}

func (m *mockAgentRegistryForT007) List() []string {
	m.listCalled = true
	return []string{}
}

// alternativeMockAgentRegistry is a second implementation to prove interface flexibility
type alternativeMockAgentRegistry struct {
	providers map[string]ports.AgentProvider
}

func (a *alternativeMockAgentRegistry) Register(provider ports.AgentProvider) error {
	if a.providers == nil {
		a.providers = make(map[string]ports.AgentProvider)
	}
	a.providers[provider.Name()] = provider
	return nil
}

func (a *alternativeMockAgentRegistry) Get(name string) (ports.AgentProvider, error) {
	if p, ok := a.providers[name]; ok {
		return p, nil
	}
	return nil, nil
}

func (a *alternativeMockAgentRegistry) Has(name string) bool {
	_, ok := a.providers[name]
	return ok
}

func (a *alternativeMockAgentRegistry) List() []string {
	names := make([]string, 0, len(a.providers))
	for name := range a.providers {
		names = append(names, name)
	}
	return names
}

// TestExecutionServiceBuilder_WithAgentRegistry_HappyPath tests normal usage scenarios
func TestExecutionServiceBuilder_WithAgentRegistry_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func() (ports.AgentRegistry, *ExecutionServiceBuilder)
		verifyFunc func(t *testing.T, registry ports.AgentRegistry, builder *ExecutionServiceBuilder)
	}{
		{
			name: "builder accepts custom AgentRegistry implementation",
			setupFunc: func() (ports.AgentRegistry, *ExecutionServiceBuilder) {
				registry := &mockAgentRegistryForT007{}
				builder := NewExecutionServiceBuilder().WithAgentRegistry(registry)
				return registry, builder
			},
			verifyFunc: func(t *testing.T, registry ports.AgentRegistry, builder *ExecutionServiceBuilder) {
				assert.NotNil(t, builder, "Builder should not be nil")
				assert.NotNil(t, registry, "Registry should not be nil")
				// Verify builder's registry field is set (via reflection or by using it)
				require.NotNil(t, builder.registry, "Builder registry field should be set")
			},
		},
		{
			name: "builder accepts alternative AgentRegistry implementation",
			setupFunc: func() (ports.AgentRegistry, *ExecutionServiceBuilder) {
				registry := &alternativeMockAgentRegistry{providers: make(map[string]ports.AgentProvider)}
				builder := NewExecutionServiceBuilder().WithAgentRegistry(registry)
				return registry, builder
			},
			verifyFunc: func(t *testing.T, registry ports.AgentRegistry, builder *ExecutionServiceBuilder) {
				assert.NotNil(t, builder)
				assert.NotNil(t, registry)
				require.NotNil(t, builder.registry, "Builder should store registry")
			},
		},
		{
			name: "builder chains WithAgentRegistry with other methods",
			setupFunc: func() (ports.AgentRegistry, *ExecutionServiceBuilder) {
				registry := &mockAgentRegistryForT007{}
				builder := NewExecutionServiceBuilder().
					WithLogger(mocks.NewMockLogger()).
					WithAgentRegistry(registry).
					WithExecutor(mocks.NewMockCommandExecutor())
				return registry, builder
			},
			verifyFunc: func(t *testing.T, registry ports.AgentRegistry, builder *ExecutionServiceBuilder) {
				assert.NotNil(t, builder, "Fluent API should work")
				require.NotNil(t, builder.registry, "Registry should be preserved in chain")
			},
		},
		{
			name: "builder builds ExecutionService with custom registry",
			setupFunc: func() (ports.AgentRegistry, *ExecutionServiceBuilder) {
				registry := &mockAgentRegistryForT007{}
				builder := NewExecutionServiceBuilder().WithAgentRegistry(registry)
				return registry, builder
			},
			verifyFunc: func(t *testing.T, registry ports.AgentRegistry, builder *ExecutionServiceBuilder) {
				svc := builder.Build()
				assert.NotNil(t, svc, "Build should succeed with custom registry")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, builder := tt.setupFunc()
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, registry, builder)
			}
		})
	}
}

// TestExecutionServiceBuilder_WithAgentRegistry_EdgeCases tests boundary conditions
func TestExecutionServiceBuilder_WithAgentRegistry_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func() *ExecutionServiceBuilder
		verifyFunc func(t *testing.T, builder *ExecutionServiceBuilder)
	}{
		{
			name: "builder with nil registry uses default",
			setupFunc: func() *ExecutionServiceBuilder {
				return NewExecutionServiceBuilder().WithAgentRegistry(nil)
			},
			verifyFunc: func(t *testing.T, builder *ExecutionServiceBuilder) {
				assert.NotNil(t, builder)
				svc := builder.Build()
				assert.NotNil(t, svc, "Build should succeed with nil registry")
			},
		},
		{
			name:      "builder without WithAgentRegistry call",
			setupFunc: NewExecutionServiceBuilder,
			verifyFunc: func(t *testing.T, builder *ExecutionServiceBuilder) {
				svc := builder.Build()
				assert.NotNil(t, svc, "Build should succeed without registry")
			},
		},
		{
			name: "multiple calls to WithAgentRegistry uses last value",
			setupFunc: func() *ExecutionServiceBuilder {
				registry1 := &mockAgentRegistryForT007{}
				registry2 := &alternativeMockAgentRegistry{}
				return NewExecutionServiceBuilder().
					WithAgentRegistry(registry1).
					WithAgentRegistry(registry2)
			},
			verifyFunc: func(t *testing.T, builder *ExecutionServiceBuilder) {
				assert.NotNil(t, builder)
				// Verify the last registry is used
				_, ok := builder.registry.(*alternativeMockAgentRegistry)
				assert.True(t, ok, "Should use last registry (alternativeMockAgentRegistry)")
			},
		},
		{
			name:      "empty builder creates service with default registry behavior",
			setupFunc: NewExecutionServiceBuilder,
			verifyFunc: func(t *testing.T, builder *ExecutionServiceBuilder) {
				svc := builder.Build()
				require.NotNil(t, svc)
				// Service should be usable even without explicit registry
				ctx := context.Background()
				_, err := svc.ListResumable(ctx)
				_ = err // May fail, but shouldn't panic
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.setupFunc()
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, builder)
			}
		})
	}
}

// TestExecutionServiceBuilder_WithAgentRegistry_InterfaceFlexibility verifies interface usage
func TestExecutionServiceBuilder_WithAgentRegistry_InterfaceFlexibility(t *testing.T) {
	t.Run("accepts any ports.AgentRegistry implementation", func(t *testing.T) {
		implementations := []struct {
			name     string
			registry ports.AgentRegistry
		}{
			{
				name:     "mockAgentRegistryForT007",
				registry: &mockAgentRegistryForT007{},
			},
			{
				name:     "alternativeMockAgentRegistry",
				registry: &alternativeMockAgentRegistry{},
			},
		}

		for _, impl := range implementations {
			t.Run(impl.name, func(t *testing.T) {
				builder := NewExecutionServiceBuilder().WithAgentRegistry(impl.registry)
				assert.NotNil(t, builder)

				svc := builder.Build()
				assert.NotNil(t, svc, "Should build successfully with %s", impl.name)
			})
		}
	})

	t.Run("registry field type is ports.AgentRegistry interface", func(t *testing.T) {
		// This test verifies compile-time type safety
		var registry ports.AgentRegistry = &mockAgentRegistryForT007{}

		builder := NewExecutionServiceBuilder().WithAgentRegistry(registry)
		assert.NotNil(t, builder)

		// If this compiles, it proves the field accepts the interface
		svc := builder.Build()
		assert.NotNil(t, svc)
	})

	t.Run("builder does not depend on infrastructure layer", func(t *testing.T) {
		// This test verifies architectural boundaries
		// If this test compiles without importing infrastructure/agents,
		// it proves the DIP violation is fixed

		builder := NewExecutionServiceBuilder()
		registry := &mockAgentRegistryForT007{}

		// Should compile using only domain/ports interface
		builder.WithAgentRegistry(registry)
		assert.NotNil(t, builder.registry)
	})
}

// TestExecutionServiceBuilder_WithAgentRegistry_ThreadSafety verifies concurrent access
func TestExecutionServiceBuilder_WithAgentRegistry_ThreadSafety(t *testing.T) {
	t.Run("concurrent WithAgentRegistry calls are safe", func(t *testing.T) {
		builder := NewExecutionServiceBuilder()
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				registry := &mockAgentRegistryForT007{}
				builder.WithAgentRegistry(registry)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}

		assert.NotNil(t, builder, "Builder should survive concurrent access")
	})

	t.Run("concurrent builds with registry are safe", func(t *testing.T) {
		registry := &mockAgentRegistryForT007{}
		builder := NewExecutionServiceBuilder().WithAgentRegistry(registry)

		done := make(chan bool, 5)

		for i := 0; i < 5; i++ {
			go func() {
				svc := builder.Build()
				assert.NotNil(t, svc)
				done <- true
			}()
		}

		for i := 0; i < 5; i++ {
			<-done
		}
	})
}

// TestExecutionServiceBuilder_RegistryField_DIPCompliance verifies architectural correctness
func TestExecutionServiceBuilder_RegistryField_DIPCompliance(t *testing.T) {
	t.Run("registry field uses interface not concrete type", func(t *testing.T) {
		// This test documents the fix for C022/T007
		// Before: registry *agents.AgentRegistry (concrete type from infrastructure)
		// After: registry ports.AgentRegistry (interface from domain)

		builder := NewExecutionServiceBuilder()
		require.NotNil(t, builder)

		// Verify we can assign different implementations
		impl1 := &mockAgentRegistryForT007{}
		builder.WithAgentRegistry(impl1)
		assert.NotNil(t, builder.registry)

		impl2 := &alternativeMockAgentRegistry{}
		builder.WithAgentRegistry(impl2)
		assert.NotNil(t, builder.registry)

		// If both assignments work, the field is an interface
	})

	t.Run("no infrastructure layer dependencies", func(t *testing.T) {
		// Architectural test: testutil should not import infrastructure
		// This file imports only:
		// - testing (stdlib)
		// - testify (test framework)
		// - internal/domain/ports (domain layer)
		// - internal/testutil (same package)
		//
		// NO import of internal/infrastructure/* should exist

		builder := NewExecutionServiceBuilder()
		registry := &mockAgentRegistryForT007{}

		// This should compile without infrastructure imports
		result := builder.WithAgentRegistry(registry)
		assert.NotNil(t, result)
	})
}
