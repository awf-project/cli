package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecuteConversation_MCPInjector_EnabledConfig verifies that executeConversation
// invokes mcpInjector when cfg.Enable=true, passing modified args to the CLI executor.
// This is the critical T010 audit fix: executeConversation must apply MCP injection
// identically to execute.
func TestExecuteConversation_MCPInjector_EnabledConfig(t *testing.T) {
	injectorCalled := false
	injectedArgs := []string{}
	cleanupCalled := false

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("assistant reply"), nil)

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"base-arg"}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "session-1", nil
		},
		mcpInjector: func(_ context.Context, args []string, cfg *workflow.MCPProxyConfig, path string, options map[string]any) ([]string, map[string]any, func() error, error) {
			injectorCalled = true
			injectedArgs = append(args, "--mcp-config", path)
			cleanup := func() error {
				cleanupCalled = true
				return nil
			}
			return injectedArgs, options, cleanup, nil
		},
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExec, nil, hooks)

	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}
	options := map[string]any{
		workflow.MCPProxyConfigKey:     cfg,
		workflow.MCPProxyConfigPathKey: "/tmp/mcp-config.json",
	}

	state := workflow.NewConversationState("test")
	_, _, err := provider.executeConversation(context.Background(), state, "hello", options, nil, nil)

	require.NoError(t, err)
	assert.True(t, injectorCalled, "mcpInjector must be called in executeConversation when cfg.Enable=true")
	assert.True(t, cleanupCalled, "mcpInjector cleanup must be invoked after executeConversation")
	assert.Contains(t, injectedArgs, "--mcp-config", "injected args should contain --mcp-config")
	assert.Contains(t, injectedArgs, "/tmp/mcp-config.json", "injected args should contain mcp config path")
}

// TestExecuteConversation_MCPInjector_NilHook verifies that when hooks.mcpInjector is nil,
// executeConversation proceeds normally without injection.
func TestExecuteConversation_MCPInjector_NilHook(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("assistant reply"), nil)

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"base-arg"}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "session-1", nil
		},
		mcpInjector: nil, // no injector
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExec, nil, hooks)

	cfg := &workflow.MCPProxyConfig{Enable: true}
	options := map[string]any{
		workflow.MCPProxyConfigKey:     cfg,
		workflow.MCPProxyConfigPathKey: "/tmp/mcp-config.json",
	}

	state := workflow.NewConversationState("test")
	result, _, err := provider.executeConversation(context.Background(), state, "hello", options, nil, nil)

	require.NoError(t, err)
	assert.NotNil(t, result, "should produce a result even with no injector")
}

// TestExecuteConversation_MCPInjector_DisabledConfig verifies that when cfg.Enable=false,
// the injector is NOT called even if hooks.mcpInjector is set.
func TestExecuteConversation_MCPInjector_DisabledConfig(t *testing.T) {
	injectorCalled := false

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("assistant reply"), nil)

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"base-arg"}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "session-1", nil
		},
		mcpInjector: func(_ context.Context, args []string, cfg *workflow.MCPProxyConfig, path string, options map[string]any) ([]string, map[string]any, func() error, error) {
			injectorCalled = true
			return args, options, noopMCPCleanup, nil
		},
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExec, nil, hooks)

	// cfg.Enable=false → should skip injection
	cfg := &workflow.MCPProxyConfig{Enable: false}
	options := map[string]any{
		workflow.MCPProxyConfigKey:     cfg,
		workflow.MCPProxyConfigPathKey: "/tmp/mcp-config.json",
	}

	state := workflow.NewConversationState("test")
	_, _, err := provider.executeConversation(context.Background(), state, "hello", options, nil, nil)

	require.NoError(t, err)
	assert.False(t, injectorCalled, "mcpInjector must NOT be called when cfg.Enable=false")
}

// TestExecuteConversation_MCPInjector_NilConfig verifies that nil MCPProxyConfig
// skips injection even if hooks.mcpInjector is set.
func TestExecuteConversation_MCPInjector_NilConfig(t *testing.T) {
	injectorCalled := false

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("assistant reply"), nil)

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"base-arg"}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "session-1", nil
		},
		mcpInjector: func(_ context.Context, args []string, cfg *workflow.MCPProxyConfig, path string, options map[string]any) ([]string, map[string]any, func() error, error) {
			injectorCalled = true
			return args, options, noopMCPCleanup, nil
		},
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExec, nil, hooks)

	// No MCPProxyConfigKey in options (nil config)
	options := map[string]any{}

	state := workflow.NewConversationState("test")
	_, _, err := provider.executeConversation(context.Background(), state, "hello", options, nil, nil)

	require.NoError(t, err)
	assert.False(t, injectorCalled, "mcpInjector must NOT be called when config is absent from options")
}

// TestExecuteConversation_MCPInjector_CleanupCalledOnce verifies the cleanup is invoked
// exactly once after executeConversation completes, even on success.
func TestExecuteConversation_MCPInjector_CleanupCalledOnce(t *testing.T) {
	cleanupCount := 0

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("assistant reply"), nil)

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"base-arg"}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "session-1", nil
		},
		mcpInjector: func(_ context.Context, args []string, cfg *workflow.MCPProxyConfig, path string, options map[string]any) ([]string, map[string]any, func() error, error) {
			cleanup := func() error {
				cleanupCount++
				return nil
			}
			return args, options, cleanup, nil
		},
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExec, nil, hooks)

	cfg := &workflow.MCPProxyConfig{Enable: true}
	options := map[string]any{
		workflow.MCPProxyConfigKey:     cfg,
		workflow.MCPProxyConfigPathKey: "/tmp/mcp-config.json",
	}

	state := workflow.NewConversationState("test")
	_, _, err := provider.executeConversation(context.Background(), state, "hello", options, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, 1, cleanupCount, "cleanup must be called exactly once after executeConversation")
}
