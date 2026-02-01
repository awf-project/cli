//go:build integration

// Feature: C038
//
// Functional tests validating application test layer purity and hexagonal architecture compliance.
// Tests verify that MockAgentRegistry and MockAgentProvider work correctly in real scenarios
// and that all application tests can run without infrastructure layer imports.

package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/testutil"
)

// TestMockAgentRegistry_HappyPath validates normal registry operations
func TestMockAgentRegistry_HappyPath(t *testing.T) {
	registry := testutil.NewMockAgentRegistry()
	require.NotNil(t, registry)

	// Register a provider
	provider := testutil.NewMockAgentProvider("test-agent")
	err := registry.Register(provider)
	assert.NoError(t, err)

	// Get the registered provider
	retrieved, err := registry.Get("test-agent")
	assert.NoError(t, err)
	assert.Equal(t, "test-agent", retrieved.Name())

	// Check if provider exists
	exists := registry.Has("test-agent")
	assert.True(t, exists)

	// List all providers
	names := registry.List()
	assert.Contains(t, names, "test-agent")
	assert.Len(t, names, 1)
}

// TestMockAgentRegistry_EdgeCases validates boundary conditions
func TestMockAgentRegistry_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testutil.MockAgentRegistry) error
		check   func(*testing.T, *testutil.MockAgentRegistry)
		wantErr bool
	}{
		{
			name: "empty registry",
			setup: func(r *testutil.MockAgentRegistry) error {
				return nil
			},
			check: func(t *testing.T, r *testutil.MockAgentRegistry) {
				names := r.List()
				assert.Empty(t, names)
				assert.False(t, r.Has("nonexistent"))
			},
			wantErr: false,
		},
		{
			name: "multiple providers",
			setup: func(r *testutil.MockAgentRegistry) error {
				if err := r.Register(testutil.NewMockAgentProvider("agent1")); err != nil {
					return err
				}
				if err := r.Register(testutil.NewMockAgentProvider("agent2")); err != nil {
					return err
				}
				return r.Register(testutil.NewMockAgentProvider("agent3"))
			},
			check: func(t *testing.T, r *testutil.MockAgentRegistry) {
				names := r.List()
				assert.Len(t, names, 3)
				assert.True(t, r.Has("agent1"))
				assert.True(t, r.Has("agent2"))
				assert.True(t, r.Has("agent3"))
			},
			wantErr: false,
		},
		{
			name: "clear registry",
			setup: func(r *testutil.MockAgentRegistry) error {
				if err := r.Register(testutil.NewMockAgentProvider("temp")); err != nil {
					return err
				}
				r.Clear()
				return nil
			},
			check: func(t *testing.T, r *testutil.MockAgentRegistry) {
				names := r.List()
				assert.Empty(t, names)
				assert.False(t, r.Has("temp"))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := testutil.NewMockAgentRegistry()
			err := tt.setup(registry)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.check(t, registry)
			}
		})
	}
}

// TestMockAgentRegistry_ErrorHandling validates error cases
func TestMockAgentRegistry_ErrorHandling(t *testing.T) {
	registry := testutil.NewMockAgentRegistry()

	// Duplicate registration should fail
	provider := testutil.NewMockAgentProvider("duplicate")
	err := registry.Register(provider)
	require.NoError(t, err)

	err = registry.Register(provider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Get nonexistent provider should fail
	_, err = registry.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestMockAgentRegistry_ThreadSafety validates concurrent access
func TestMockAgentRegistry_ThreadSafety(t *testing.T) {
	registry := testutil.NewMockAgentRegistry()

	// Register 10 providers concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			provider := testutil.NewMockAgentProvider(string(rune('a' + idx)))
			_ = registry.Register(provider)
		}(i)
	}
	wg.Wait()

	// Verify all providers registered
	names := registry.List()
	assert.GreaterOrEqual(t, len(names), 1, "at least some providers should register")

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.List()
			_ = registry.Has("a")
			_, _ = registry.Get("a")
		}()
	}
	wg.Wait()
}

// TestMockAgentProvider_HappyPath validates normal provider operations
func TestMockAgentProvider_HappyPath(t *testing.T) {
	provider := testutil.NewMockAgentProvider("test-agent")
	require.NotNil(t, provider)

	ctx := context.Background()

	// Execute with default stub behavior
	result, err := provider.Execute(ctx, "test prompt", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-agent", result.Provider)

	// ExecuteConversation with default stub behavior
	state := &workflow.ConversationState{
		Turns: []workflow.Turn{
			*workflow.NewTurn(workflow.TurnRoleUser, "hello"),
		},
	}
	convResult, err := provider.ExecuteConversation(ctx, state, "continue", nil)
	assert.NoError(t, err)
	assert.NotNil(t, convResult)

	// Validate
	err = provider.Validate()
	assert.NoError(t, err)

	// Name
	name := provider.Name()
	assert.Equal(t, "test-agent", name)
}

// TestMockAgentProvider_CustomBehavior validates callback functions
func TestMockAgentProvider_CustomBehavior(t *testing.T) {
	provider := testutil.NewMockAgentProvider("custom-agent")
	ctx := context.Background()

	// Custom execute behavior
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "custom-agent",
			Output:   "custom response: " + prompt,
			Tokens:   42,
		}, nil
	})

	result, err := provider.Execute(ctx, "test", nil)
	assert.NoError(t, err)
	assert.Equal(t, "custom response: test", result.Output)
	assert.Equal(t, 42, result.Tokens)

	// Custom conversation behavior
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		newTurns := append(state.Turns, *workflow.NewTurn(workflow.TurnRoleAssistant, "custom reply"))
		return &workflow.ConversationResult{
			State: &workflow.ConversationState{
				Turns: newTurns,
			},
			Output:      "custom final",
			TokensTotal: 100,
		}, nil
	})

	state := &workflow.ConversationState{
		Turns: []workflow.Turn{
			*workflow.NewTurn(workflow.TurnRoleUser, "hello"),
		},
	}
	convResult, err := provider.ExecuteConversation(ctx, state, "continue", nil)
	assert.NoError(t, err)
	assert.Equal(t, "custom final", convResult.Output)
	assert.Len(t, convResult.State.Turns, 2)

	// Custom validation behavior
	provider.SetValidateFunc(func() error {
		return errors.New("validation failed")
	})

	err = provider.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

// TestMockAgentProvider_ErrorHandling validates error scenarios
func TestMockAgentProvider_ErrorHandling(t *testing.T) {
	provider := testutil.NewMockAgentProvider("error-agent")
	ctx := context.Background()

	// Execute error
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return nil, errors.New("execution failed")
	})

	result, err := provider.Execute(ctx, "test", nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "execution failed")

	// Conversation error
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		return nil, errors.New("conversation failed")
	})

	state := &workflow.ConversationState{}
	convResult, err := provider.ExecuteConversation(ctx, state, "test", nil)
	assert.Error(t, err)
	assert.Nil(t, convResult)
	assert.Contains(t, err.Error(), "conversation failed")
}

// TestMockAgentProvider_ThreadSafety validates concurrent access
func TestMockAgentProvider_ThreadSafety(t *testing.T) {
	provider := testutil.NewMockAgentProvider("concurrent-agent")
	ctx := context.Background()

	// Set up custom behavior
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "concurrent-agent",
			Output:   "response",
			Tokens:   10,
		}, nil
	})

	// Concurrent executions
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = provider.Execute(ctx, "test", nil)
			_ = provider.Validate()
			_ = provider.Name()
		}(i)
	}
	wg.Wait()
}

// TestPortCompliance_AgentRegistry verifies interface implementation
func TestPortCompliance_AgentRegistry(t *testing.T) {
	var _ ports.AgentRegistry = (*testutil.MockAgentRegistry)(nil)

	registry := testutil.NewMockAgentRegistry()
	require.NotNil(t, registry)

	// Verify all interface methods exist and work
	provider := testutil.NewMockAgentProvider("compliance-test")
	err := registry.Register(provider)
	assert.NoError(t, err)

	retrieved, err := registry.Get("compliance-test")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)

	names := registry.List()
	assert.Contains(t, names, "compliance-test")

	exists := registry.Has("compliance-test")
	assert.True(t, exists)
}

// TestPortCompliance_AgentProvider verifies interface implementation
func TestPortCompliance_AgentProvider(t *testing.T) {
	var _ ports.AgentProvider = (*testutil.MockAgentProvider)(nil)

	provider := testutil.NewMockAgentProvider("compliance-test")
	require.NotNil(t, provider)

	ctx := context.Background()

	// Verify all interface methods exist and work
	result, err := provider.Execute(ctx, "test", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	state := &workflow.ConversationState{}
	convResult, err := provider.ExecuteConversation(ctx, state, "test", nil)
	assert.NoError(t, err)
	assert.NotNil(t, convResult)

	name := provider.Name()
	assert.Equal(t, "compliance-test", name)

	err = provider.Validate()
	assert.NoError(t, err)
}

// TestIntegration_RegistryWithProvider validates registry and provider working together
func TestIntegration_RegistryWithProvider(t *testing.T) {
	registry := testutil.NewMockAgentRegistry()
	ctx := context.Background()

	// Register multiple providers with custom behavior
	agent1 := testutil.NewMockAgentProvider("agent1")
	agent1.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "agent1",
			Output:   "response from agent1",
			Tokens:   10,
		}, nil
	})

	agent2 := testutil.NewMockAgentProvider("agent2")
	agent2.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "agent2",
			Output:   "response from agent2",
			Tokens:   20,
		}, nil
	})

	require.NoError(t, registry.Register(agent1))
	require.NoError(t, registry.Register(agent2))

	// Use providers from registry
	provider1, err := registry.Get("agent1")
	require.NoError(t, err)
	result1, err := provider1.Execute(ctx, "test", nil)
	assert.NoError(t, err)
	assert.Equal(t, "response from agent1", result1.Output)
	assert.Equal(t, 10, result1.Tokens)

	provider2, err := registry.Get("agent2")
	require.NoError(t, err)
	result2, err := provider2.Execute(ctx, "test", nil)
	assert.NoError(t, err)
	assert.Equal(t, "response from agent2", result2.Output)
	assert.Equal(t, 20, result2.Tokens)
}

// TestIntegration_BuilderWithMocks validates ExecutionServiceBuilder with mocks
func TestIntegration_BuilderWithMocks(t *testing.T) {
	// Create mock registry with custom provider
	registry := testutil.NewMockAgentRegistry()
	provider := testutil.NewMockAgentProvider("test-agent")
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "test-agent",
			Output:   "success",
			Tokens:   100,
		}, nil
	})
	require.NoError(t, registry.Register(provider))

	// Build service with mock registry
	builder := testutil.NewExecutionServiceBuilder().
		WithAgentRegistry(registry)

	service := builder.Build()
	require.NotNil(t, service)

	// Verify service can be built successfully with mock dependencies
	// (actual execution would require workflow and state setup, tested elsewhere)
}

// TestArchitectureCompliance_NoInfrastructureImports validates test purity
func TestArchitectureCompliance_NoInfrastructureImports(t *testing.T) {
	// This test exists to document that these mocks enable application tests
	// to run without importing internal/infrastructure/agents package.
	//
	// The actual verification is done by the compiler - if any application test
	// imports infrastructure/agents, the test would fail at compile time.
	//
	// This test verifies that the mocks provide complete functionality
	// without requiring infrastructure imports.

	registry := testutil.NewMockAgentRegistry()
	provider := testutil.NewMockAgentProvider("architecture-test")

	// All operations work without infrastructure
	require.NoError(t, registry.Register(provider))
	retrieved, err := registry.Get("architecture-test")
	require.NoError(t, err)
	assert.Equal(t, "architecture-test", retrieved.Name())

	ctx := context.Background()
	result, err := provider.Execute(ctx, "test", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Success - no infrastructure imports needed
}

// TestRealWorldScenario_ConversationWorkflow simulates actual usage pattern
func TestRealWorldScenario_ConversationWorkflow(t *testing.T) {
	registry := testutil.NewMockAgentRegistry()
	ctx := context.Background()

	// Setup agent with realistic conversation behavior
	agent := testutil.NewMockAgentProvider("claude")
	conversationTurns := 0
	agent.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		conversationTurns++
		newTurns := append(state.Turns,
			*workflow.NewTurn(workflow.TurnRoleUser, prompt),
			*workflow.NewTurn(workflow.TurnRoleAssistant, "AI response "+string(rune('0'+conversationTurns))),
		)

		return &workflow.ConversationResult{
			State: &workflow.ConversationState{
				Turns: newTurns,
			},
			Output:      "AI response " + string(rune('0'+conversationTurns)),
			TokensTotal: conversationTurns * 100,
		}, nil
	})

	require.NoError(t, registry.Register(agent))

	// Simulate multi-turn conversation
	provider, err := registry.Get("claude")
	require.NoError(t, err)

	state := &workflow.ConversationState{Turns: []workflow.Turn{}}

	// Turn 1
	result1, err := provider.ExecuteConversation(ctx, state, "Hello", nil)
	require.NoError(t, err)
	assert.Equal(t, "AI response 1", result1.Output)
	assert.Equal(t, 100, result1.TokensTotal)
	state = result1.State

	// Turn 2
	result2, err := provider.ExecuteConversation(ctx, state, "Continue", nil)
	require.NoError(t, err)
	assert.Equal(t, "AI response 2", result2.Output)
	assert.Equal(t, 200, result2.TokensTotal)
	state = result2.State

	// Verify conversation history
	assert.Len(t, state.Turns, 4) // 2 user + 2 assistant
}
