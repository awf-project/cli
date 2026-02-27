package mocks_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T001
// Feature: C038

var _ ports.AgentRegistry = (*mocks.MockAgentRegistry)(nil)

// Component: T001
// Feature: C038

func TestMockAgentRegistry_NewMockAgentRegistry(t *testing.T) {
	registry := mocks.NewMockAgentRegistry()

	require.NotNil(t, registry, "NewMockAgentRegistry should return non-nil instance")

	// Verify it's usable immediately
	provider, err := registry.Get("nonexistent")
	assert.Error(t, err, "Get on empty registry should return error")
	assert.Nil(t, provider, "Get on empty registry should return nil provider")

	names := registry.List()
	assert.Empty(t, names, "List on empty registry should return empty slice")

	has := registry.Has("nonexistent")
	assert.False(t, has, "Has on empty registry should return false")
}

func TestMockAgentRegistry_RegisterAndGet_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		wantErr      bool
	}{
		{
			name:         "register and get single provider",
			providerName: "test-agent",
			wantErr:      false,
		},
		{
			name:         "register provider with hyphenated name",
			providerName: "claude-3-opus",
			wantErr:      false,
		},
		{
			name:         "register provider with underscores",
			providerName: "custom_agent_v2",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := mocks.NewMockAgentRegistry()
			provider := &mockAgentProvider{name: tt.providerName}

			err := registry.Register(provider)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err, "Register should not error for valid provider")

			// Verify Get returns the same provider
			retrieved, err := registry.Get(tt.providerName)
			require.NoError(t, err, "Get should not error for registered provider")
			assert.Equal(t, provider, retrieved, "Get should return the same provider instance")
			assert.Equal(t, tt.providerName, retrieved.Name(), "Provider name should match")
		})
	}
}

func TestMockAgentRegistry_List_HappyPath(t *testing.T) {
	tests := []struct {
		name          string
		providerNames []string
		wantCount     int
	}{
		{
			name:          "list empty registry",
			providerNames: []string{},
			wantCount:     0,
		},
		{
			name:          "list single provider",
			providerNames: []string{"claude"},
			wantCount:     1,
		},
		{
			name:          "list multiple providers",
			providerNames: []string{"claude", "gemini", "codex"},
			wantCount:     3,
		},
		{
			name:          "list many providers",
			providerNames: []string{"p1", "p2", "p3", "p4", "p5"},
			wantCount:     5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := mocks.NewMockAgentRegistry()
			for _, name := range tt.providerNames {
				err := registry.Register(&mockAgentProvider{name: name})
				require.NoError(t, err)
			}

			names := registry.List()

			assert.Len(t, names, tt.wantCount, "List should return correct number of providers")

			// Verify all registered providers are in the list
			for _, expectedName := range tt.providerNames {
				assert.Contains(t, names, expectedName, "List should contain provider %s", expectedName)
			}
		})
	}
}

func TestMockAgentRegistry_Has_HappyPath(t *testing.T) {
	registry := mocks.NewMockAgentRegistry()
	err := registry.Register(&mockAgentProvider{name: "claude"})
	require.NoError(t, err)
	err = registry.Register(&mockAgentProvider{name: "gemini"})
	require.NoError(t, err)

	tests := []struct {
		name         string
		providerName string
		want         bool
	}{
		{
			name:         "has existing provider - claude",
			providerName: "claude",
			want:         true,
		},
		{
			name:         "has existing provider - gemini",
			providerName: "gemini",
			want:         true,
		},
		{
			name:         "has nonexistent provider",
			providerName: "codex",
			want:         false,
		},
		{
			name:         "has empty string provider",
			providerName: "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			has := registry.Has(tt.providerName)

			assert.Equal(t, tt.want, has, "Has should return %v for provider %s", tt.want, tt.providerName)
		})
	}
}

// Component: T001
// Feature: C038

func TestMockAgentRegistry_Register_DuplicateProvider(t *testing.T) {
	registry := mocks.NewMockAgentRegistry()
	provider1 := &mockAgentProvider{name: "claude"}
	provider2 := &mockAgentProvider{name: "claude"}

	err1 := registry.Register(provider1)
	err2 := registry.Register(provider2)

	require.NoError(t, err1, "First registration should succeed")
	require.Error(t, err2, "Second registration with same name should fail")
	assert.Contains(t, err2.Error(), "provider already registered", "Error should indicate duplicate")
	assert.Contains(t, err2.Error(), "claude", "Error should contain provider name")
}

func TestMockAgentRegistry_Get_NonexistentProvider(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
	}{
		{
			name:         "get nonexistent provider with normal name",
			providerName: "nonexistent",
		},
		{
			name:         "get nonexistent provider with empty string",
			providerName: "",
		},
		{
			name:         "get nonexistent provider with special chars",
			providerName: "agent@#$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := mocks.NewMockAgentRegistry()

			provider, err := registry.Get(tt.providerName)

			require.Error(t, err, "Get should return error for nonexistent provider")
			assert.Nil(t, provider, "Get should return nil provider on error")
			assert.Contains(t, err.Error(), "provider not found", "Error should indicate not found")
		})
	}
}

func TestMockAgentRegistry_List_ReturnsNewSlice(t *testing.T) {
	registry := mocks.NewMockAgentRegistry()
	err := registry.Register(&mockAgentProvider{name: "claude"})
	require.NoError(t, err)

	list1 := registry.List()
	list2 := registry.List()

	assert.Equal(t, list1, list2, "Lists should contain same elements")

	// Modify first list
	if len(list1) > 0 {
		list1[0] = "modified"
	}

	// Second list should be unchanged
	list3 := registry.List()
	assert.NotEqual(t, list1[0], list3[0], "Modifying returned list should not affect registry")
}

func TestMockAgentRegistry_Clear_RemovesAllProviders(t *testing.T) {
	registry := mocks.NewMockAgentRegistry()
	err := registry.Register(&mockAgentProvider{name: "claude"})
	require.NoError(t, err)
	err = registry.Register(&mockAgentProvider{name: "gemini"})
	require.NoError(t, err)
	err = registry.Register(&mockAgentProvider{name: "codex"})
	require.NoError(t, err)

	// Verify setup
	assert.Len(t, registry.List(), 3, "Registry should have 3 providers before clear")

	registry.Clear()

	assert.Empty(t, registry.List(), "List should be empty after Clear")
	assert.False(t, registry.Has("claude"), "Should not have claude after Clear")
	assert.False(t, registry.Has("gemini"), "Should not have gemini after Clear")
	assert.False(t, registry.Has("codex"), "Should not have codex after Clear")

	provider, err := registry.Get("claude")
	assert.Error(t, err, "Get should error after Clear")
	assert.Nil(t, provider, "Get should return nil after Clear")
}

func TestMockAgentRegistry_RegisterAfterClear(t *testing.T) {
	registry := mocks.NewMockAgentRegistry()
	err := registry.Register(&mockAgentProvider{name: "old-provider"})
	require.NoError(t, err)

	registry.Clear()
	err = registry.Register(&mockAgentProvider{name: "new-provider"})

	require.NoError(t, err, "Should be able to register after Clear")
	assert.True(t, registry.Has("new-provider"), "Should have new provider")
	assert.False(t, registry.Has("old-provider"), "Should not have old provider")
}

// Component: T001
// Feature: C038

func TestMockAgentRegistry_ErrorMessages(t *testing.T) {
	tests := []struct {
		name            string
		setupFunc       func(*mocks.MockAgentRegistry)
		operation       func(*mocks.MockAgentRegistry) error
		expectedErrText string
	}{
		{
			name: "register duplicate - error contains provider name",
			setupFunc: func(r *mocks.MockAgentRegistry) {
				_ = r.Register(&mockAgentProvider{name: "test-agent"})
			},
			operation: func(r *mocks.MockAgentRegistry) error {
				return r.Register(&mockAgentProvider{name: "test-agent"})
			},
			expectedErrText: "provider already registered: test-agent",
		},
		{
			name:      "get nonexistent - error contains provider name",
			setupFunc: func(r *mocks.MockAgentRegistry) {},
			operation: func(r *mocks.MockAgentRegistry) error {
				_, err := r.Get("missing-agent")
				return err
			},
			expectedErrText: "provider not found: missing-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := mocks.NewMockAgentRegistry()
			tt.setupFunc(registry)

			err := tt.operation(registry)

			require.Error(t, err)
			assert.Equal(t, tt.expectedErrText, err.Error(), "Error message should match expected format")
		})
	}
}

// Component: T001
// Feature: C038

func TestMockAgentRegistry_ThreadSafety_ConcurrentRegister(t *testing.T) {
	registry := mocks.NewMockAgentRegistry()
	const numGoroutines = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			providerName := fmt.Sprintf("provider-%d", id)
			err := registry.Register(&mockAgentProvider{name: providerName})
			assert.NoError(t, err, "Concurrent registration should not error")
		}(i)
	}

	wg.Wait()

	names := registry.List()
	assert.Len(t, names, numGoroutines, "All providers should be registered")
}

func TestMockAgentRegistry_ThreadSafety_ConcurrentGetAndRegister(t *testing.T) {
	registry := mocks.NewMockAgentRegistry()
	const numReaders = 20
	const numWriters = 5

	// Pre-populate with some providers
	for i := 0; i < 3; i++ {
		err := registry.Register(&mockAgentProvider{name: fmt.Sprintf("existing-%d", i)})
		require.NoError(t, err)
	}

	var wg sync.WaitGroup
	wg.Add(numReaders + numWriters)

	// Readers
	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				// Try reading existing providers
				_, _ = registry.Get("existing-0")
				_ = registry.Has("existing-1")
				_ = registry.List()
			}
		}(i)
	}

	// Writers
	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()
			providerName := fmt.Sprintf("new-%d", id)
			_ = registry.Register(&mockAgentProvider{name: providerName})
		}(i)
	}

	wg.Wait()

	names := registry.List()
	assert.GreaterOrEqual(t, len(names), 3, "Should have at least the pre-populated providers")
}

func TestMockAgentRegistry_ThreadSafety_ConcurrentClear(t *testing.T) {
	registry := mocks.NewMockAgentRegistry()
	const numOperations = 50

	var wg sync.WaitGroup
	wg.Add(numOperations)

	for i := 0; i < numOperations; i++ {
		go func(id int) {
			defer wg.Done()
			switch id % 4 {
			case 0:
				_ = registry.Register(&mockAgentProvider{name: fmt.Sprintf("p-%d", id)})
			case 1:
				_, _ = registry.Get(fmt.Sprintf("p-%d", id))
			case 2:
				_ = registry.List()
			case 3:
				registry.Clear()
			}
		}(i)
	}

	wg.Wait()

	// Registry may be empty or have some providers depending on timing
	names := registry.List()
	t.Logf("Final registry has %d providers after concurrent operations", len(names))
}

// mockAgentProvider is a minimal test implementation of ports.AgentProvider
type mockAgentProvider struct {
	name             string
	executeFunc      func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error)
	conversationFunc func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error)
	validateFunc     func() error
}

func (m *mockAgentProvider) Name() string {
	return m.name
}

func (m *mockAgentProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, prompt, options)
	}
	return &workflow.AgentResult{
		Provider:    m.name,
		Output:      "mock response",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}, nil
}

func (m *mockAgentProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	if m.conversationFunc != nil {
		return m.conversationFunc(ctx, state, prompt, options)
	}
	return &workflow.ConversationResult{
		Provider:    m.name,
		State:       state,
		Output:      "mock conversation response",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}, nil
}

func (m *mockAgentProvider) Validate() error {
	if m.validateFunc != nil {
		return m.validateFunc()
	}
	return nil
}

// Component: T002
// Feature: C038

var _ ports.AgentProvider = (*mocks.MockAgentProvider)(nil)

// Component: T002
// Feature: C038

func TestMockAgentProvider_NewMockAgentProvider(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")

	require.NotNil(t, provider, "NewMockAgentProvider should return non-nil instance")
	assert.Equal(t, "test-agent", provider.Name(), "Provider name should match constructor argument")

	// Verify it's usable immediately with default stub behavior
	ctx := context.Background()
	result, err := provider.Execute(ctx, "test prompt", nil)
	require.NoError(t, err, "Execute should not error with default stub behavior")
	assert.NotNil(t, result, "Execute should return non-nil result")
	assert.Equal(t, "test-agent", result.Provider, "Result provider should match mock name")
	assert.Empty(t, result.Output, "Default stub should return empty output")
	assert.Zero(t, result.Tokens, "Default stub should return zero tokens")

	// Verify Validate returns nil by default
	err = provider.Validate()
	assert.NoError(t, err, "Validate should not error with default stub behavior")
}

func TestMockAgentProvider_Execute_HappyPath(t *testing.T) {
	tests := []struct {
		name           string
		providerName   string
		setupFunc      func(*mocks.MockAgentProvider)
		prompt         string
		options        map[string]any
		expectedOutput string
		expectedTokens int
		wantErr        bool
	}{
		{
			name:           "execute with default stub behavior",
			providerName:   "claude",
			setupFunc:      func(p *mocks.MockAgentProvider) {},
			prompt:         "Tell me a joke",
			options:        nil,
			expectedOutput: "",
			expectedTokens: 0,
			wantErr:        false,
		},
		{
			name:         "execute with custom function - simple response",
			providerName: "gemini",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
					return &workflow.AgentResult{
						Provider: "gemini",
						Output:   "Custom response",
						Tokens:   42,
					}, nil
				})
			},
			prompt:         "What is 2+2?",
			options:        map[string]any{"temperature": 0.7},
			expectedOutput: "Custom response",
			expectedTokens: 42,
			wantErr:        false,
		},
		{
			name:         "execute with custom function - echo prompt",
			providerName: "test-agent",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
					return &workflow.AgentResult{
						Provider: "test-agent",
						Output:   fmt.Sprintf("You asked: %s", prompt),
						Tokens:   100,
					}, nil
				})
			},
			prompt:         "Hello, world!",
			options:        nil,
			expectedOutput: "You asked: Hello, world!",
			expectedTokens: 100,
			wantErr:        false,
		},
		{
			name:         "execute with custom function - reads options",
			providerName: "codex",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
					temp := options["temperature"].(float64)
					return &workflow.AgentResult{
						Provider: "codex",
						Output:   fmt.Sprintf("Temp: %.1f", temp),
						Tokens:   50,
					}, nil
				})
			},
			prompt:         "Generate code",
			options:        map[string]any{"temperature": 0.5},
			expectedOutput: "Temp: 0.5",
			expectedTokens: 50,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := mocks.NewMockAgentProvider(tt.providerName)
			tt.setupFunc(provider)
			ctx := context.Background()

			result, err := provider.Execute(ctx, tt.prompt, tt.options)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err, "Execute should not error")
			require.NotNil(t, result, "Execute should return non-nil result")
			assert.Equal(t, tt.providerName, result.Provider, "Result provider should match mock name")
			assert.Equal(t, tt.expectedOutput, result.Output, "Result output should match expected")
			assert.Equal(t, tt.expectedTokens, result.Tokens, "Result tokens should match expected")
		})
	}
}

func TestMockAgentProvider_ExecuteConversation_HappyPath(t *testing.T) {
	tests := []struct {
		name           string
		providerName   string
		setupFunc      func(*mocks.MockAgentProvider)
		initialState   *workflow.ConversationState
		prompt         string
		options        map[string]any
		expectedOutput string
		wantErr        bool
	}{
		{
			name:         "execute conversation with default stub behavior",
			providerName: "claude",
			setupFunc:    func(p *mocks.MockAgentProvider) {},
			initialState: &workflow.ConversationState{
				Turns:       []workflow.Turn{},
				TotalTurns:  0,
				TotalTokens: 0,
			},
			prompt:         "Hello",
			options:        nil,
			expectedOutput: "",
			wantErr:        false,
		},
		{
			name:         "execute conversation with custom function - simple response",
			providerName: "gemini",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
					return &workflow.ConversationResult{
						Provider: "gemini",
						State:    state,
						Output:   "Conversation response",
					}, nil
				})
			},
			initialState: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleUser, Content: "Previous message"},
				},
				TotalTurns:  1,
				TotalTokens: 0,
			},
			prompt:         "Follow-up question",
			options:        map[string]any{"max_tokens": 500},
			expectedOutput: "Conversation response",
			wantErr:        false,
		},
		{
			name:         "execute conversation with stateful response",
			providerName: "test-agent",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
					turnCount := len(state.Turns)
					return &workflow.ConversationResult{
						Provider: "test-agent",
						State:    state,
						Output:   fmt.Sprintf("Turn %d: %s", turnCount+1, prompt),
					}, nil
				})
			},
			initialState: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleUser, Content: "First"},
					{Role: workflow.TurnRoleAssistant, Content: "Response 1"},
				},
				TotalTurns:  2,
				TotalTokens: 0,
			},
			prompt:         "Second question",
			options:        nil,
			expectedOutput: "Turn 3: Second question",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := mocks.NewMockAgentProvider(tt.providerName)
			tt.setupFunc(provider)
			ctx := context.Background()

			result, err := provider.ExecuteConversation(ctx, tt.initialState, tt.prompt, tt.options)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err, "ExecuteConversation should not error")
			require.NotNil(t, result, "ExecuteConversation should return non-nil result")
			assert.Equal(t, tt.providerName, result.Provider, "Result provider should match mock name")
			assert.Equal(t, tt.initialState, result.State, "Result should preserve conversation state")
			assert.Equal(t, tt.expectedOutput, result.Output, "Result output should match expected")
		})
	}
}

func TestMockAgentProvider_Name_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
	}{
		{
			name:         "simple name",
			providerName: "claude",
		},
		{
			name:         "hyphenated name",
			providerName: "claude-3-opus",
		},
		{
			name:         "underscored name",
			providerName: "custom_agent_v2",
		},
		{
			name:         "empty name",
			providerName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := mocks.NewMockAgentProvider(tt.providerName)

			name := provider.Name()

			assert.Equal(t, tt.providerName, name, "Name should match constructor argument")

			// Verify Name is idempotent
			name2 := provider.Name()
			assert.Equal(t, name, name2, "Name should return same value on repeated calls")
		})
	}
}

func TestMockAgentProvider_Validate_HappyPath(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*mocks.MockAgentProvider)
		wantErr     bool
		expectedErr string
	}{
		{
			name:      "validate with default stub behavior - no error",
			setupFunc: func(p *mocks.MockAgentProvider) {},
			wantErr:   false,
		},
		{
			name: "validate with custom function - returns nil",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetValidateFunc(func() error {
					return nil
				})
			},
			wantErr: false,
		},
		{
			name: "validate with custom function - returns error",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetValidateFunc(func() error {
					return fmt.Errorf("provider not configured")
				})
			},
			wantErr:     true,
			expectedErr: "provider not configured",
		},
		{
			name: "validate with custom function - checks configuration",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetValidateFunc(func() error {
					return fmt.Errorf("API key missing")
				})
			},
			wantErr:     true,
			expectedErr: "API key missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := mocks.NewMockAgentProvider("test-agent")
			tt.setupFunc(provider)

			err := provider.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.expectedErr, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Component: T002
// Feature: C038

func TestMockAgentProvider_SetExecuteFunc_OverwritesPrevious(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	ctx := context.Background()

	// Set first function
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "test-agent",
			Output:   "First response",
			Tokens:   10,
		}, nil
	})

	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "test-agent",
			Output:   "Second response",
			Tokens:   20,
		}, nil
	})

	result, err := provider.Execute(ctx, "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "Second response", result.Output, "Should use the second function")
	assert.Equal(t, 20, result.Tokens, "Should use the second function's token count")
}

func TestMockAgentProvider_SetConversationFunc_OverwritesPrevious(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	ctx := context.Background()
	state := &workflow.ConversationState{
		Turns:       []workflow.Turn{},
		TotalTurns:  0,
		TotalTokens: 0,
	}

	// Set first function
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		return &workflow.ConversationResult{
			Provider: "test-agent",
			State:    state,
			Output:   "First conversation",
		}, nil
	})

	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		return &workflow.ConversationResult{
			Provider: "test-agent",
			State:    state,
			Output:   "Second conversation",
		}, nil
	})

	result, err := provider.ExecuteConversation(ctx, state, "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "Second conversation", result.Output, "Should use the second function")
}

func TestMockAgentProvider_SetValidateFunc_OverwritesPrevious(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")

	// Set first function
	provider.SetValidateFunc(func() error {
		return fmt.Errorf("first error")
	})

	provider.SetValidateFunc(func() error {
		return fmt.Errorf("second error")
	})

	err := provider.Validate()

	require.Error(t, err)
	assert.Equal(t, "second error", err.Error(), "Should use the second function")
}

func TestMockAgentProvider_Execute_EmptyPrompt(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "test-agent",
			Output:   fmt.Sprintf("Prompt length: %d", len(prompt)),
			Tokens:   1,
		}, nil
	})
	ctx := context.Background()

	result, err := provider.Execute(ctx, "", nil)

	require.NoError(t, err)
	assert.Equal(t, "Prompt length: 0", result.Output, "Should handle empty prompt")
}

func TestMockAgentProvider_Execute_NilOptions(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		optionsNil := options == nil
		return &workflow.AgentResult{
			Provider: "test-agent",
			Output:   fmt.Sprintf("Options nil: %v", optionsNil),
			Tokens:   1,
		}, nil
	})
	ctx := context.Background()

	result, err := provider.Execute(ctx, "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "Options nil: true", result.Output, "Should handle nil options")
}

func TestMockAgentProvider_Execute_EmptyOptions(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "test-agent",
			Output:   fmt.Sprintf("Options count: %d", len(options)),
			Tokens:   1,
		}, nil
	})
	ctx := context.Background()

	result, err := provider.Execute(ctx, "test", map[string]any{})

	require.NoError(t, err)
	assert.Equal(t, "Options count: 0", result.Output, "Should handle empty options map")
}

func TestMockAgentProvider_ExecuteConversation_NilState(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		stateNil := state == nil
		return &workflow.ConversationResult{
			Provider: "test-agent",
			State:    state,
			Output:   fmt.Sprintf("State nil: %v", stateNil),
		}, nil
	})
	ctx := context.Background()

	result, err := provider.ExecuteConversation(ctx, nil, "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "State nil: true", result.Output, "Should handle nil state")
	assert.Nil(t, result.State, "Result state should be nil")
}

func TestMockAgentProvider_ExecuteConversation_EmptyTurns(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		return &workflow.ConversationResult{
			Provider: "test-agent",
			State:    state,
			Output:   fmt.Sprintf("Turn count: %d", len(state.Turns)),
		}, nil
	})
	ctx := context.Background()
	state := &workflow.ConversationState{
		Turns:       []workflow.Turn{},
		TotalTurns:  0,
		TotalTokens: 0,
	}

	result, err := provider.ExecuteConversation(ctx, state, "test", nil)

	require.NoError(t, err)
	assert.Equal(t, "Turn count: 0", result.Output, "Should handle empty turns")
}

// Component: T002
// Feature: C038

func TestMockAgentProvider_Execute_CustomError(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*mocks.MockAgentProvider)
		expectedErr string
	}{
		{
			name: "execute returns custom error",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
					return nil, fmt.Errorf("agent execution failed: API timeout")
				})
			},
			expectedErr: "agent execution failed: API timeout",
		},
		{
			name: "execute returns error with nil result",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
					return nil, fmt.Errorf("authentication failed")
				})
			},
			expectedErr: "authentication failed",
		},
		{
			name: "execute returns error based on prompt",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
					if prompt == "" {
						return nil, fmt.Errorf("prompt cannot be empty")
					}
					return &workflow.AgentResult{Provider: "test-agent", Output: "ok"}, nil
				})
			},
			expectedErr: "prompt cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := mocks.NewMockAgentProvider("test-agent")
			tt.setupFunc(provider)
			ctx := context.Background()

			result, err := provider.Execute(ctx, "", nil)

			require.Error(t, err)
			assert.Nil(t, result, "Result should be nil when error occurs")
			assert.Equal(t, tt.expectedErr, err.Error())
		})
	}
}

func TestMockAgentProvider_ExecuteConversation_CustomError(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*mocks.MockAgentProvider)
		expectedErr string
	}{
		{
			name: "conversation returns custom error",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
					return nil, fmt.Errorf("conversation failed: rate limit exceeded")
				})
			},
			expectedErr: "conversation failed: rate limit exceeded",
		},
		{
			name: "conversation returns error with nil result",
			setupFunc: func(p *mocks.MockAgentProvider) {
				p.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
					return nil, fmt.Errorf("invalid conversation state")
				})
			},
			expectedErr: "invalid conversation state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := mocks.NewMockAgentProvider("test-agent")
			tt.setupFunc(provider)
			ctx := context.Background()
			state := &workflow.ConversationState{
				Turns:       []workflow.Turn{},
				TotalTurns:  0,
				TotalTokens: 0,
			}

			result, err := provider.ExecuteConversation(ctx, state, "test", nil)

			require.Error(t, err)
			assert.Nil(t, result, "Result should be nil when error occurs")
			assert.Equal(t, tt.expectedErr, err.Error())
		})
	}
}

func TestMockAgentProvider_Validate_CustomError(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	provider.SetValidateFunc(func() error {
		return fmt.Errorf("validation failed: missing API key")
	})

	err := provider.Validate()

	require.Error(t, err)
	assert.Equal(t, "validation failed: missing API key", err.Error())
}

// Component: T002
// Feature: C038

func TestMockAgentProvider_ThreadSafety_ConcurrentExecute(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	var callCount int
	var mu sync.Mutex

	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		return &workflow.AgentResult{
			Provider: "test-agent",
			Output:   fmt.Sprintf("Response to: %s", prompt),
			Tokens:   10,
		}, nil
	})

	const numGoroutines = 20
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			prompt := fmt.Sprintf("prompt-%d", id)
			result, err := provider.Execute(ctx, prompt, nil)
			assert.NoError(t, err, "Concurrent Execute should not error")
			assert.NotNil(t, result, "Concurrent Execute should return result")
		}(i)
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, numGoroutines, callCount, "All concurrent calls should have been executed")
}

func TestMockAgentProvider_ThreadSafety_ConcurrentExecuteConversation(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	var callCount int
	var mu sync.Mutex

	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		return &workflow.ConversationResult{
			Provider: "test-agent",
			State:    state,
			Output:   fmt.Sprintf("Conversation: %s", prompt),
		}, nil
	})

	const numGoroutines = 20
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			state := &workflow.ConversationState{
				Turns:       []workflow.Turn{},
				TotalTurns:  0,
				TotalTokens: 0,
			}
			result, err := provider.ExecuteConversation(ctx, state, fmt.Sprintf("prompt-%d", id), nil)
			assert.NoError(t, err, "Concurrent ExecuteConversation should not error")
			assert.NotNil(t, result, "Concurrent ExecuteConversation should return result")
		}(i)
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, numGoroutines, callCount, "All concurrent conversation calls should have been executed")
}

func TestMockAgentProvider_ThreadSafety_ConcurrentSetAndExecute(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	const numReaders = 30
	const numWriters = 5
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(numReaders + numWriters)

	// Readers (Execute calls)
	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_, _ = provider.Execute(ctx, fmt.Sprintf("prompt-%d-%d", id, j), nil)
				_ = provider.Name()
				_ = provider.Validate()
			}
		}(i)
	}

	// Writers (SetExecuteFunc calls)
	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()
			provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
				return &workflow.AgentResult{
					Provider: "test-agent",
					Output:   fmt.Sprintf("Writer-%d: %s", id, prompt),
					Tokens:   int(id),
				}, nil
			})
		}(i)
	}

	wg.Wait()

	result, err := provider.Execute(ctx, "final-test", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMockAgentProvider_ThreadSafety_ConcurrentName(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	const numGoroutines = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			name := provider.Name()
			assert.Equal(t, "test-agent", name, "Name should be consistent")
		}()
	}

	wg.Wait()

	assert.Equal(t, "test-agent", provider.Name())
}

func TestMockAgentProvider_ThreadSafety_MixedOperations(t *testing.T) {
	provider := mocks.NewMockAgentProvider("test-agent")
	const numOperations = 100
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(numOperations)

	for i := 0; i < numOperations; i++ {
		go func(id int) {
			defer wg.Done()
			switch id % 5 {
			case 0:
				_, _ = provider.Execute(ctx, fmt.Sprintf("prompt-%d", id), nil)
			case 1:
				state := &workflow.ConversationState{
					Turns:       []workflow.Turn{},
					TotalTurns:  0,
					TotalTokens: 0,
				}
				_, _ = provider.ExecuteConversation(ctx, state, fmt.Sprintf("prompt-%d", id), nil)
			case 2:
				_ = provider.Name()
			case 3:
				_ = provider.Validate()
			case 4:
				provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
					return &workflow.AgentResult{Provider: "test-agent", Output: "ok"}, nil
				})
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, "test-agent", provider.Name())
}
