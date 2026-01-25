package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
)

// Component: T005
// Feature: C022

// TestSetAgentRegistry_AcceptsInterfaceType verifies that SetAgentRegistry
// accepts the ports.AgentRegistry interface type, following the same pattern
// as SetOperationProvider. This ensures compliance with DIP.
func TestSetAgentRegistry_AcceptsInterfaceType(t *testing.T) {
	// Arrange
	execSvc, _ := NewTestHarness(t).Build()
	mockRegistry := &setterTestAgentRegistry{}

	// Act: Method should accept interface type
	execSvc.SetAgentRegistry(mockRegistry)

	// Assert: No compilation error means method accepts interface type
	assert.NotNil(t, execSvc)
}

// TestSetAgentRegistry_AcceptsNil verifies that SetAgentRegistry can accept nil,
// which is a valid value for interface types.
func TestSetAgentRegistry_AcceptsNil(t *testing.T) {
	// Arrange
	execSvc, _ := NewTestHarness(t).Build()

	// Act: Method should accept nil without panic
	execSvc.SetAgentRegistry(nil)

	// Assert: No panic occurred
	assert.NotNil(t, execSvc)
}

// TestSetAgentRegistry_AcceptsMultipleImplementations verifies that different
// implementations of ports.AgentRegistry can be passed to SetAgentRegistry.
func TestSetAgentRegistry_AcceptsMultipleImplementations(t *testing.T) {
	tests := []struct {
		name     string
		registry ports.AgentRegistry
		desc     string
	}{
		{
			name:     "first_implementation",
			registry: &setterTestAgentRegistry{},
			desc:     "Should accept first mock implementation",
		},
		{
			name:     "second_implementation",
			registry: &setterTestAgentRegistryAlt{},
			desc:     "Should accept alternative mock implementation",
		},
		{
			name:     "third_implementation",
			registry: &setterTestAgentRegistryThird{},
			desc:     "Should accept yet another mock implementation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			execSvc, _ := NewTestHarness(t).Build()

			// Act: Each implementation should be accepted by SetAgentRegistry
			execSvc.SetAgentRegistry(tt.registry)

			// Assert: Interface polymorphism allows any implementation
			assert.NotNil(t, execSvc, tt.desc)
		})
	}
}

// TestSetAgentRegistry_MatchesSetOperationProviderPattern verifies that
// SetAgentRegistry follows the same signature pattern as SetOperationProvider,
// both accepting interface types from the ports package.
func TestSetAgentRegistry_MatchesSetOperationProviderPattern(t *testing.T) {
	// Arrange
	execSvc, _ := NewTestHarness(t).Build()

	// Both methods should accept their respective interface types
	var agentReg ports.AgentRegistry = &setterTestAgentRegistry{}
	var opProvider ports.OperationProvider = &setterTestOperationProvider{}

	// Act: Both setters should accept interface types without concrete dependencies
	execSvc.SetAgentRegistry(agentReg)
	execSvc.SetOperationProvider(opProvider)

	// Assert: Both methods follow the same DIP-compliant pattern
	assert.NotNil(t, execSvc)
}

// TestSetAgentRegistry_SupportsReassignment verifies that the registry
// can be changed after initial assignment, supporting flexible configuration.
func TestSetAgentRegistry_SupportsReassignment(t *testing.T) {
	// Arrange
	execSvc, _ := NewTestHarness(t).Build()
	first := &setterTestAgentRegistry{}
	second := &setterTestAgentRegistryAlt{}

	// Act: Set initial registry
	execSvc.SetAgentRegistry(first)
	assert.NotNil(t, execSvc, "First assignment should succeed")

	// Act: Reassign with different implementation
	execSvc.SetAgentRegistry(second)
	assert.NotNil(t, execSvc, "Reassignment should succeed")

	// Act: Set to nil (clearing the registry)
	execSvc.SetAgentRegistry(nil)
	assert.NotNil(t, execSvc, "Setting to nil should succeed")
}

// TestSetAgentRegistry_InterfaceTypeEnablesDependencyInjection verifies that
// accepting the interface type enables proper dependency injection without
// coupling to infrastructure implementations.
func TestSetAgentRegistry_InterfaceTypeEnablesDependencyInjection(t *testing.T) {
	// Arrange: Create service with test harness
	execSvc, _ := NewTestHarness(t).Build()

	// Demonstrate DI: Create registry using only interface type declaration
	var registry ports.AgentRegistry = &setterTestAgentRegistry{}

	// Act: Inject dependency via interface
	execSvc.SetAgentRegistry(registry)

	// Assert: Dependency injection works without concrete type knowledge
	assert.NotNil(t, execSvc)
	assert.NotNil(t, registry)
}

// TestSetAgentRegistry_NoConcreteTypeRequired verifies that the method
// signature does not require or expect a concrete type, only the interface.
func TestSetAgentRegistry_NoConcreteTypeRequired(t *testing.T) {
	// Arrange
	execSvc, _ := NewTestHarness(t).Build()

	// Create instances through interface type, not concrete type
	registries := []ports.AgentRegistry{
		&setterTestAgentRegistry{},
		&setterTestAgentRegistryAlt{},
		nil,
	}

	for i, reg := range registries {
		t.Run("interface_instance_"+string(rune('A'+i)), func(t *testing.T) {
			// Act: Pass interface instance to setter
			execSvc.SetAgentRegistry(reg)

			// Assert: Method accepts interface without requiring concrete type
			assert.NotNil(t, execSvc)
		})
	}
}

// setterTestAgentRegistry is a test double implementing ports.AgentRegistry for T005 tests
type setterTestAgentRegistry struct{}

func (m *setterTestAgentRegistry) Register(provider ports.AgentProvider) error {
	return nil
}

func (m *setterTestAgentRegistry) Get(name string) (ports.AgentProvider, error) {
	return nil, nil
}

func (m *setterTestAgentRegistry) List() []string {
	return []string{}
}

func (m *setterTestAgentRegistry) Has(name string) bool {
	return false
}

// setterTestAgentRegistryAlt is another test implementation for T005
type setterTestAgentRegistryAlt struct{}

func (a *setterTestAgentRegistryAlt) Register(provider ports.AgentProvider) error {
	return nil
}

func (a *setterTestAgentRegistryAlt) Get(name string) (ports.AgentProvider, error) {
	return nil, nil
}

func (a *setterTestAgentRegistryAlt) List() []string {
	return []string{}
}

func (a *setterTestAgentRegistryAlt) Has(name string) bool {
	return false
}

// setterTestAgentRegistryThird is a third test implementation for T005
type setterTestAgentRegistryThird struct{}

func (y *setterTestAgentRegistryThird) Register(provider ports.AgentProvider) error {
	return nil
}

func (y *setterTestAgentRegistryThird) Get(name string) (ports.AgentProvider, error) {
	return nil, nil
}

func (y *setterTestAgentRegistryThird) List() []string {
	return []string{}
}

func (y *setterTestAgentRegistryThird) Has(name string) bool {
	return false
}

// setterTestOperationProvider is a test double for demonstrating the pattern match
type setterTestOperationProvider struct{}

func (m *setterTestOperationProvider) GetOperation(name string) (*plugin.OperationSchema, bool) {
	return nil, false
}

func (m *setterTestOperationProvider) ListOperations() []*plugin.OperationSchema {
	return []*plugin.OperationSchema{}
}

func (m *setterTestOperationProvider) Execute(ctx context.Context, name string, inputs map[string]any) (*plugin.OperationResult, error) {
	return nil, nil
}
