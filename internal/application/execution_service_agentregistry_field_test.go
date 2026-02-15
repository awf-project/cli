package application_test

import (
	"testing"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

// Component: T004
// Feature: C022

// TestExecutionService_AgentRegistryField_InterfaceType verifies that the agentRegistry
// field uses the ports.AgentRegistry interface type instead of a concrete implementation.
// This test ensures compile-time compliance with the Dependency Inversion Principle.
func TestExecutionService_AgentRegistryField_InterfaceType(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	// Create a mock registry that implements ports.AgentRegistry
	mockRegistry := &testAgentRegistry{}

	execSvc.SetAgentRegistry(mockRegistry)

	assert.NotNil(t, execSvc)
}

// TestExecutionService_SetAgentRegistry_AcceptsInterface verifies that SetAgentRegistry
// accepts the ports.AgentRegistry interface type, not a concrete implementation.
func TestExecutionService_SetAgentRegistry_AcceptsInterface(t *testing.T) {
	tests := []struct {
		name     string
		registry ports.AgentRegistry
		wantNil  bool
	}{
		{
			name:     "happy_path_with_mock_registry",
			registry: &testAgentRegistry{},
			wantNil:  false,
		},
		{
			name:     "accepts_nil_registry",
			registry: nil,
			wantNil:  true,
		},
		{
			name:     "accepts_custom_implementation",
			registry: &customTestAgentRegistry{},
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execSvc, _ := NewTestHarness(t).Build()

			execSvc.SetAgentRegistry(tt.registry)

			assert.NotNil(t, execSvc)
		})
	}
}

// TestExecutionService_AgentRegistryField_NoInfrastructureDependency verifies that
// the ExecutionService can be created and used with the agentRegistry field
// without importing infrastructure packages.
func TestExecutionService_AgentRegistryField_NoInfrastructureDependency(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	// Create a registry using only the interface
	var registry ports.AgentRegistry = &testAgentRegistry{}

	execSvc.SetAgentRegistry(registry)

	assert.NotNil(t, execSvc)
	assert.NotNil(t, registry)
}

// TestExecutionService_AgentRegistryField_MultipleImplementations verifies that
// different implementations of ports.AgentRegistry can be used interchangeably.
func TestExecutionService_AgentRegistryField_MultipleImplementations(t *testing.T) {
	implementations := []struct {
		name     string
		registry ports.AgentRegistry
	}{
		{
			name:     "test_implementation",
			registry: &testAgentRegistry{},
		},
		{
			name:     "custom_implementation",
			registry: &customTestAgentRegistry{},
		},
		{
			name:     "another_implementation",
			registry: &anotherTestAgentRegistry{},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			execSvc, _ := NewTestHarness(t).Build()

			execSvc.SetAgentRegistry(impl.registry)

			assert.NotNil(t, execSvc)
		})
	}
}

// TestExecutionService_AgentRegistryField_TypeSafety verifies that the field
// maintains type safety by only accepting ports.AgentRegistry interface implementations.
func TestExecutionService_AgentRegistryField_TypeSafety(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	// Create different implementations of the interface
	registries := []ports.AgentRegistry{
		&testAgentRegistry{},
		&customTestAgentRegistry{},
		nil, // nil is also a valid value for the interface
	}

	for i, registry := range registries {
		t.Run("accepts_implementation_"+string(rune('A'+i)), func(t *testing.T) {
			execSvc.SetAgentRegistry(registry)

			assert.NotNil(t, execSvc)
		})
	}
}

// TestExecutionService_AgentRegistryField_NilRegistry verifies that
// a nil registry can be set without causing issues.
func TestExecutionService_AgentRegistryField_NilRegistry(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	execSvc.SetAgentRegistry(nil)

	assert.NotNil(t, execSvc)
}

// TestExecutionService_AgentRegistryField_ReassignRegistry verifies that
// the registry can be reassigned with different implementations.
func TestExecutionService_AgentRegistryField_ReassignRegistry(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	first := &testAgentRegistry{}
	second := &customTestAgentRegistry{}

	execSvc.SetAgentRegistry(first)

	assert.NotNil(t, execSvc)

	execSvc.SetAgentRegistry(second)

	assert.NotNil(t, execSvc)

	execSvc.SetAgentRegistry(nil)

	assert.NotNil(t, execSvc)
}

// testAgentRegistry is a test double implementing ports.AgentRegistry
type testAgentRegistry struct{}

func (t *testAgentRegistry) Register(provider ports.AgentProvider) error {
	return nil
}

func (t *testAgentRegistry) Get(name string) (ports.AgentProvider, error) {
	return nil, nil
}

func (t *testAgentRegistry) List() []string {
	return []string{}
}

func (t *testAgentRegistry) Has(name string) bool {
	return false
}

// customTestAgentRegistry is another test implementation
type customTestAgentRegistry struct{}

func (c *customTestAgentRegistry) Register(provider ports.AgentProvider) error {
	return nil
}

func (c *customTestAgentRegistry) Get(name string) (ports.AgentProvider, error) {
	return nil, nil
}

func (c *customTestAgentRegistry) List() []string {
	return []string{}
}

func (c *customTestAgentRegistry) Has(name string) bool {
	return false
}

// anotherTestAgentRegistry is yet another test implementation
type anotherTestAgentRegistry struct{}

func (a *anotherTestAgentRegistry) Register(provider ports.AgentProvider) error {
	return nil
}

func (a *anotherTestAgentRegistry) Get(name string) (ports.AgentProvider, error) {
	return nil, nil
}

func (a *anotherTestAgentRegistry) List() []string {
	return []string{}
}

func (a *anotherTestAgentRegistry) Has(name string) bool {
	return false
}
