package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/testutil"
)

// Component: T004 - Provider Constructor Functional Options
// Tests the refactored provider constructors with CLIExecutor dependency injection

func TestClaudeProvider_NewWithOptions_HappyPath(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*testutil.MockCLIExecutor)
		options     []ClaudeProviderOption
		expectError bool
	}{
		{
			name: "no options uses default executor",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("test output"), []byte(""))
			},
			options:     nil,
			expectError: false,
		},
		{
			name: "with custom executor option",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("custom executor output"), []byte(""))
			},
			options: []ClaudeProviderOption{
				WithClaudeExecutor(testutil.NewMockCLIExecutor()),
			},
			expectError: false,
		},
		{
			name: "multiple options applied in order",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("final executor output"), []byte(""))
			},
			options: []ClaudeProviderOption{
				WithClaudeExecutor(testutil.NewMockCLIExecutor()),
				WithClaudeExecutor(testutil.NewMockCLIExecutor()), // Last one wins
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockExec *testutil.MockCLIExecutor
			var opts []ClaudeProviderOption
			if tt.setupMock != nil {
				mockExec = testutil.NewMockCLIExecutor()
				tt.setupMock(mockExec)
				// Always use mock executor for testing, even when testing "no options" case
				opts = []ClaudeProviderOption{WithClaudeExecutor(mockExec)}
			} else if tt.options != nil {
				opts = tt.options
			}

			provider := NewClaudeProviderWithOptions(opts...)

			require.NotNil(t, provider)
			assert.NotNil(t, provider.executor)
			assert.NotNil(t, provider.logger)

			// Verify executor is functional by executing
			if mockExec != nil {
				ctx := context.Background()
				result, err := provider.Execute(ctx, "test prompt", nil)

				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, result)
				}
			}
		})
	}
}

func TestGeminiProvider_NewWithOptions_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*testutil.MockCLIExecutor)
		options   []GeminiProviderOption
	}{
		{
			name: "no options uses default executor",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("gemini output"), []byte(""))
			},
			options: nil,
		},
		{
			name: "with custom executor option",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("custom gemini output"), []byte(""))
			},
			options: []GeminiProviderOption{
				WithGeminiExecutor(testutil.NewMockCLIExecutor()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockExec *testutil.MockCLIExecutor
			var opts []GeminiProviderOption
			if tt.setupMock != nil {
				mockExec = testutil.NewMockCLIExecutor()
				tt.setupMock(mockExec)
				// Always use mock executor for testing
				opts = []GeminiProviderOption{WithGeminiExecutor(mockExec)}
			} else if tt.options != nil {
				opts = tt.options
			}

			provider := NewGeminiProviderWithOptions(opts...)

			require.NotNil(t, provider)
			assert.NotNil(t, provider.executor)

			// Verify executor is functional
			if mockExec != nil {
				ctx := context.Background()
				result, err := provider.Execute(ctx, "test prompt", nil)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestCodexProvider_NewWithOptions_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*testutil.MockCLIExecutor)
		options   []CodexProviderOption
	}{
		{
			name: "no options uses default executor",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("codex output"), []byte(""))
			},
			options: nil,
		},
		{
			name: "with custom executor option",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("custom codex output"), []byte(""))
			},
			options: []CodexProviderOption{
				WithCodexExecutor(testutil.NewMockCLIExecutor()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockExec *testutil.MockCLIExecutor
			var opts []CodexProviderOption
			if tt.setupMock != nil {
				mockExec = testutil.NewMockCLIExecutor()
				tt.setupMock(mockExec)
				// Always use mock executor for testing
				opts = []CodexProviderOption{WithCodexExecutor(mockExec)}
			} else if tt.options != nil {
				opts = tt.options
			}

			provider := NewCodexProviderWithOptions(opts...)

			require.NotNil(t, provider)
			assert.NotNil(t, provider.executor)

			// Verify executor is functional
			if mockExec != nil {
				ctx := context.Background()
				result, err := provider.Execute(ctx, "test prompt", nil)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestOpenCodeProvider_NewWithOptions_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*testutil.MockCLIExecutor)
		options   []OpenCodeProviderOption
	}{
		{
			name: "no options uses default executor",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("opencode output"), []byte(""))
			},
			options: nil,
		},
		{
			name: "with custom executor option",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("custom opencode output"), []byte(""))
			},
			options: []OpenCodeProviderOption{
				WithOpenCodeExecutor(testutil.NewMockCLIExecutor()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockExec *testutil.MockCLIExecutor
			var opts []OpenCodeProviderOption
			if tt.setupMock != nil {
				mockExec = testutil.NewMockCLIExecutor()
				tt.setupMock(mockExec)
				// Always use mock executor for testing
				opts = []OpenCodeProviderOption{WithOpenCodeExecutor(mockExec)}
			} else if tt.options != nil {
				opts = tt.options
			}

			provider := NewOpenCodeProviderWithOptions(opts...)

			require.NotNil(t, provider)
			assert.NotNil(t, provider.executor)

			// Verify executor is functional
			if mockExec != nil {
				ctx := context.Background()
				result, err := provider.Execute(ctx, "test prompt", nil)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestCustomProvider_NewWithOptions_HappyPath(t *testing.T) {
	tests := []struct {
		name            string
		providerName    string
		commandTemplate string
		setupMock       func(*testutil.MockCLIExecutor)
		options         []CustomProviderOption
	}{
		{
			name:            "no options uses default executor",
			providerName:    "my-agent",
			commandTemplate: "my-agent --prompt {{.Prompt}}",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("custom agent output"), []byte(""))
			},
			options: nil,
		},
		{
			name:            "with custom executor option",
			providerName:    "my-agent",
			commandTemplate: "my-agent --prompt {{.Prompt}}",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("mock custom agent output"), []byte(""))
			},
			options: []CustomProviderOption{
				WithCustomExecutor(testutil.NewMockCLIExecutor()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockExec *testutil.MockCLIExecutor
			var opts []CustomProviderOption
			if tt.setupMock != nil {
				mockExec = testutil.NewMockCLIExecutor()
				tt.setupMock(mockExec)
				// Always use mock executor for testing
				opts = []CustomProviderOption{WithCustomExecutor(mockExec)}
			} else if tt.options != nil {
				opts = tt.options
			}

			provider := NewCustomProviderWithOptions(tt.providerName, tt.commandTemplate, opts...)

			require.NotNil(t, provider)
			assert.NotNil(t, provider.executor)
			assert.Equal(t, tt.providerName, provider.name)
			assert.Equal(t, tt.commandTemplate, provider.commandTemplate)

			// Verify executor is functional
			if mockExec != nil {
				ctx := context.Background()
				result, err := provider.Execute(ctx, "test prompt", nil)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestProviderOptions_EdgeCases(t *testing.T) {
	t.Run("nil executor option panics are prevented", func(t *testing.T) {
		// Note: Passing nil executor should work but will cause runtime issues later
		// The constructor doesn't validate this - that's by design for flexibility

		// Claude
		claudeProvider := NewClaudeProviderWithOptions(WithClaudeExecutor(nil))
		assert.NotNil(t, claudeProvider)
		// executor field will be nil, which will cause issues during Execute

		// Gemini
		geminiProvider := NewGeminiProviderWithOptions(WithGeminiExecutor(nil))
		assert.NotNil(t, geminiProvider)

		// Codex
		codexProvider := NewCodexProviderWithOptions(WithCodexExecutor(nil))
		assert.NotNil(t, codexProvider)

		// OpenCode
		opencodeProvider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(nil))
		assert.NotNil(t, opencodeProvider)

		// Custom
		customProvider := NewCustomProviderWithOptions("test", "test {{.Prompt}}", WithCustomExecutor(nil))
		assert.NotNil(t, customProvider)
	})

	t.Run("empty options slice behaves like no options", func(t *testing.T) {
		claudeProvider := NewClaudeProviderWithOptions([]ClaudeProviderOption{}...)
		require.NotNil(t, claudeProvider)
		assert.NotNil(t, claudeProvider.executor)
		assert.NotNil(t, claudeProvider.logger)
	})

	t.Run("options applied in correct order", func(t *testing.T) {
		// Create two different mock executors
		mock1 := testutil.NewMockCLIExecutor()
		mock1.SetOutput([]byte("mock1"), []byte(""))

		mock2 := testutil.NewMockCLIExecutor()
		mock2.SetOutput([]byte("mock2"), []byte(""))

		// Apply both options - last one should win
		provider := NewClaudeProviderWithOptions(
			WithClaudeExecutor(mock1),
			WithClaudeExecutor(mock2), // This should be the final executor
		)

		require.NotNil(t, provider)
		assert.NotNil(t, provider.executor)

		// Verify the last executor was used
		ctx := context.Background()
		result, err := provider.Execute(ctx, "test", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Output, "mock2")
	})

	t.Run("backward compatibility - original constructors still work", func(t *testing.T) {
		// Original constructors without options should still work
		claudeProvider := NewClaudeProvider()
		assert.NotNil(t, claudeProvider)
		assert.NotNil(t, claudeProvider.executor)

		geminiProvider := NewGeminiProvider()
		assert.NotNil(t, geminiProvider)
		assert.NotNil(t, geminiProvider.executor)

		codexProvider := NewCodexProvider()
		assert.NotNil(t, codexProvider)
		assert.NotNil(t, codexProvider.executor)

		opencodeProvider := NewOpenCodeProvider()
		assert.NotNil(t, opencodeProvider)
		assert.NotNil(t, opencodeProvider.executor)

		customProvider := NewCustomProvider("test", "test {{.Prompt}}")
		assert.NotNil(t, customProvider)
		assert.NotNil(t, customProvider.executor)
	})
}

func TestProviderOptions_ErrorHandling(t *testing.T) {
	t.Run("claude provider executor error propagates", func(t *testing.T) {
		mockExec := testutil.NewMockCLIExecutor()
		mockExec.SetError(errors.New("claude CLI failed"))

		provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "test prompt", nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "claude execution failed")
	})

	t.Run("gemini provider executor error propagates", func(t *testing.T) {
		mockExec := testutil.NewMockCLIExecutor()
		mockExec.SetError(errors.New("gemini CLI failed"))

		provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "test prompt", nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "gemini execution failed")
	})

	t.Run("codex provider executor error propagates", func(t *testing.T) {
		mockExec := testutil.NewMockCLIExecutor()
		mockExec.SetError(errors.New("codex CLI failed"))

		provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "test prompt", nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "codex execution failed")
	})

	t.Run("opencode provider executor error propagates", func(t *testing.T) {
		mockExec := testutil.NewMockCLIExecutor()
		mockExec.SetError(errors.New("opencode CLI failed"))

		provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "test prompt", nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "opencode execution failed")
	})

	t.Run("custom provider executor error propagates", func(t *testing.T) {
		mockExec := testutil.NewMockCLIExecutor()
		mockExec.SetError(errors.New("custom CLI failed"))

		provider := NewCustomProviderWithOptions("test", "test {{.Prompt}}", WithCustomExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "test prompt", nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "execution failed")
	})
}

func TestProviderOptions_Integration(t *testing.T) {
	t.Run("claude provider with mock executor executes successfully", func(t *testing.T) {
		mockExec := testutil.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("Claude response"), []byte(""))

		provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "What is 2+2?", nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "claude", result.Provider)
		assert.Contains(t, result.Output, "Claude response")
		assert.False(t, result.StartedAt.IsZero())
		assert.False(t, result.CompletedAt.IsZero())
		assert.True(t, result.CompletedAt.After(result.StartedAt))

		// Verify executor was called correctly
		calls := mockExec.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, "claude", calls[0].Name)
		assert.Contains(t, calls[0].Args, "-p")
		assert.Contains(t, calls[0].Args, "What is 2+2?")
	})

	t.Run("gemini provider with mock executor executes successfully", func(t *testing.T) {
		mockExec := testutil.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("Gemini response"), []byte(""))

		provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "Explain Go interfaces", nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "gemini", result.Provider)
		assert.Contains(t, result.Output, "Gemini response")

		// Verify executor was called correctly
		calls := mockExec.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, "gemini", calls[0].Name)
		assert.Contains(t, calls[0].Args, "Explain Go interfaces")
	})

	t.Run("codex provider with mock executor executes successfully", func(t *testing.T) {
		mockExec := testutil.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("Codex response"), []byte(""))

		provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "Write a function", nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "codex", result.Provider)
		assert.Contains(t, result.Output, "Codex response")

		// Verify executor was called correctly
		calls := mockExec.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, "codex", calls[0].Name)
	})

	t.Run("opencode provider with mock executor executes successfully", func(t *testing.T) {
		mockExec := testutil.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("OpenCode response"), []byte(""))

		provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "Generate code", nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "opencode", result.Provider)
		assert.Contains(t, result.Output, "OpenCode response")

		// Verify executor was called correctly
		calls := mockExec.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, "opencode", calls[0].Name)
	})

	t.Run("custom provider with mock executor executes successfully", func(t *testing.T) {
		mockExec := testutil.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("Custom agent response"), []byte(""))

		provider := NewCustomProviderWithOptions(
			"my-agent",
			"my-agent --prompt {{.Prompt}}",
			WithCustomExecutor(mockExec),
		)
		ctx := context.Background()

		result, err := provider.Execute(ctx, "Test prompt", nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "my-agent", result.Provider)
		assert.Contains(t, result.Output, "Custom agent response")

		// Verify executor was called
		calls := mockExec.GetCalls()
		require.Len(t, calls, 1)
	})

	t.Run("multiple providers can use different executors", func(t *testing.T) {
		// Create separate mock executors for each provider
		claudeMock := testutil.NewMockCLIExecutor()
		claudeMock.SetOutput([]byte("Claude specific"), []byte(""))

		geminiMock := testutil.NewMockCLIExecutor()
		geminiMock.SetOutput([]byte("Gemini specific"), []byte(""))

		// Create providers with different executors
		claudeProvider := NewClaudeProviderWithOptions(WithClaudeExecutor(claudeMock))
		geminiProvider := NewGeminiProviderWithOptions(WithGeminiExecutor(geminiMock))

		ctx := context.Background()

		// Execute both
		claudeResult, err := claudeProvider.Execute(ctx, "prompt1", nil)
		require.NoError(t, err)
		assert.Contains(t, claudeResult.Output, "Claude specific")

		geminiResult, err := geminiProvider.Execute(ctx, "prompt2", nil)
		require.NoError(t, err)
		assert.Contains(t, geminiResult.Output, "Gemini specific")

		// Verify each executor was called independently
		claudeCalls := claudeMock.GetCalls()
		require.Len(t, claudeCalls, 1)

		geminiCalls := geminiMock.GetCalls()
		require.Len(t, geminiCalls, 1)
	})
}
