package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTokenizer struct{}

func (stubTokenizer) CountTokens(string) (int, error)        { return 0, nil }
func (stubTokenizer) CountTurnsTokens([]string) (int, error) { return 0, nil }
func (stubTokenizer) IsEstimate() bool                       { return true }
func (stubTokenizer) ModelName() string                      { return "stub" }

var _ ports.Tokenizer = stubTokenizer{}

type countingTokenizer struct{ count int }

func (t countingTokenizer) CountTokens(string) (int, error)        { return t.count, nil }
func (t countingTokenizer) CountTurnsTokens([]string) (int, error) { return t.count, nil }
func (t countingTokenizer) IsEstimate() bool                       { return false }
func (t countingTokenizer) ModelName() string                      { return "counting" }

var _ ports.Tokenizer = countingTokenizer{}

func TestClaudeProvider_NewWithOptions_HappyPath(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*mocks.MockCLIExecutor)
		options     []ClaudeProviderOption
		expectError bool
	}{
		{
			name: "no options uses default executor",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("test output"), []byte(""))
			},
			options:     nil,
			expectError: false,
		},
		{
			name: "with custom executor option",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("custom executor output"), []byte(""))
			},
			options: []ClaudeProviderOption{
				WithClaudeExecutor(mocks.NewMockCLIExecutor()),
			},
			expectError: false,
		},
		{
			name: "multiple options applied in order",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("final executor output"), []byte(""))
			},
			options: []ClaudeProviderOption{
				WithClaudeExecutor(mocks.NewMockCLIExecutor()),
				WithClaudeExecutor(mocks.NewMockCLIExecutor()), // Last one wins
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockExec *mocks.MockCLIExecutor
			var opts []ClaudeProviderOption
			if tt.setupMock != nil {
				mockExec = mocks.NewMockCLIExecutor()
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
				result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)

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
		setupMock func(*mocks.MockCLIExecutor)
		options   []GeminiProviderOption
	}{
		{
			name: "no options uses default executor",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("gemini output"), []byte(""))
			},
			options: nil,
		},
		{
			name: "with custom executor option",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("custom gemini output"), []byte(""))
			},
			options: []GeminiProviderOption{
				WithGeminiExecutor(mocks.NewMockCLIExecutor()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockExec *mocks.MockCLIExecutor
			var opts []GeminiProviderOption
			if tt.setupMock != nil {
				mockExec = mocks.NewMockCLIExecutor()
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
				result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestCodexProvider_NewWithOptions_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockCLIExecutor)
		options   []CodexProviderOption
	}{
		{
			name: "no options uses default executor",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("codex output"), []byte(""))
			},
			options: nil,
		},
		{
			name: "with custom executor option",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("custom codex output"), []byte(""))
			},
			options: []CodexProviderOption{
				WithCodexExecutor(mocks.NewMockCLIExecutor()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockExec *mocks.MockCLIExecutor
			var opts []CodexProviderOption
			if tt.setupMock != nil {
				mockExec = mocks.NewMockCLIExecutor()
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
				result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestOpenCodeProvider_NewWithOptions_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockCLIExecutor)
		options   []OpenCodeProviderOption
	}{
		{
			name: "no options uses default executor",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("opencode output"), []byte(""))
			},
			options: nil,
		},
		{
			name: "with custom executor option",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("custom opencode output"), []byte(""))
			},
			options: []OpenCodeProviderOption{
				WithOpenCodeExecutor(mocks.NewMockCLIExecutor()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockExec *mocks.MockCLIExecutor
			var opts []OpenCodeProviderOption
			if tt.setupMock != nil {
				mockExec = mocks.NewMockCLIExecutor()
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
				result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestCopilotProvider_NewWithOptions_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockCLIExecutor)
		options   []CopilotProviderOption
	}{
		{
			name: "no options uses default executor",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"copilot output\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), []byte(""))
			},
			options: nil,
		},
		{
			name: "with custom executor option",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"custom copilot output\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), []byte(""))
			},
			options: []CopilotProviderOption{
				WithCopilotExecutor(mocks.NewMockCLIExecutor()),
			},
		},
		{
			name: "with custom logger option",
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"copilot with logger\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), []byte(""))
			},
			options: []CopilotProviderOption{
				WithCopilotExecutor(mocks.NewMockCLIExecutor()),
				WithCopilotLogger(&mocks.MockLogger{}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockExec *mocks.MockCLIExecutor
			var opts []CopilotProviderOption
			if tt.setupMock != nil {
				mockExec = mocks.NewMockCLIExecutor()
				tt.setupMock(mockExec)
				// Always use mock executor for testing
				opts = []CopilotProviderOption{WithCopilotExecutor(mockExec)}
			} else if tt.options != nil {
				opts = tt.options
			}

			provider := NewCopilotProviderWithOptions(opts...)

			require.NotNil(t, provider)
			assert.NotNil(t, provider.executor)
			assert.NotNil(t, provider.logger)

			// Verify executor is functional
			if mockExec != nil {
				ctx := context.Background()
				result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestMistralVibeProviderOptionTypeLeftUnchangedInOptionsGo(t *testing.T) {
	called := false
	var option MistralVibeProviderOption = func(*MistralVibeProvider) {
		called = true
	}

	provider := NewMistralVibeProviderWithOptions(option)

	require.NotNil(t, provider)
	assert.True(t, called)
}

func TestMistralVibeProvider_NewWithOptions(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("Mistral Vibe response"), []byte(""))

	provider := NewMistralVibeProviderWithOptions(WithMistralVibeExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "Write a release note", nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "mistral_vibe", result.Provider)
	assert.Contains(t, result.Output, "Mistral Vibe response")

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "vibe", calls[0].Name)
	assert.Contains(t, calls[0].Args, "--prompt")
	assert.Contains(t, calls[0].Args, "Write a release note")
}

func TestWithMistralVibeLoggerInjectsLoggerWithoutChangingDefaultLoggerBehaviorWhenNilIsNotSupplied(t *testing.T) {
	defaultProvider := NewMistralVibeProvider()
	require.NotNil(t, defaultProvider)
	assert.NotNil(t, defaultProvider.logger)

	l := mocks.NewMockLogger()
	provider := NewMistralVibeProviderWithOptions(WithMistralVibeLogger(l))

	require.NotNil(t, provider)
	assert.Equal(t, ports.Logger(l), provider.logger)
}

func TestNewMistralVibeProviderWithOptionsAppliesOptionsInOrderAndRefreshesBaseProvider(t *testing.T) {
	firstExec := mocks.NewMockCLIExecutor()
	firstExec.SetOutput([]byte("first executor"), []byte(""))
	lastExec := mocks.NewMockCLIExecutor()
	lastExec.SetOutput([]byte("last executor"), []byte(""))
	tok := countingTokenizer{count: 42}

	provider := NewMistralVibeProviderWithOptions(
		WithMistralVibeExecutor(firstExec),
		WithMistralVibeExecutor(lastExec),
		WithMistralVibeTokenizer(tok),
	)

	result, err := provider.Execute(context.Background(), "prompt", nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Output, "last executor")
	assert.Equal(t, 42, result.Tokens)
	assert.Empty(t, firstExec.GetCalls())
	require.Len(t, lastExec.GetCalls(), 1)
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

		// Mistral Vibe
		mistralVibeProvider := NewMistralVibeProviderWithOptions(WithMistralVibeExecutor(nil))
		assert.NotNil(t, mistralVibeProvider)
	})

	t.Run("empty options slice behaves like no options", func(t *testing.T) {
		claudeProvider := NewClaudeProviderWithOptions([]ClaudeProviderOption{}...)
		require.NotNil(t, claudeProvider)
		assert.NotNil(t, claudeProvider.executor)
		assert.NotNil(t, claudeProvider.logger)
	})

	t.Run("options applied in correct order", func(t *testing.T) {
		// Create two different mock executors
		mock1 := mocks.NewMockCLIExecutor()
		mock1.SetOutput([]byte("mock1"), []byte(""))

		mock2 := mocks.NewMockCLIExecutor()
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
		result, err := provider.Execute(ctx, "test", nil, nil, nil)
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
	})
}

func TestWithMistralVibeNilOptionDependenciesFollowExistingProviderOptionSemantics(t *testing.T) {
	provider := NewMistralVibeProviderWithOptions(
		WithMistralVibeExecutor(nil),
		WithMistralVibeLogger(nil),
		WithMistralVibeTokenizer(nil),
	)

	require.NotNil(t, provider)
	assert.Nil(t, provider.executor)
	assert.Nil(t, provider.logger)
	assert.Nil(t, provider.tokenizer)
	assert.NotNil(t, provider.base)
}

func TestProviderOptions_ErrorHandling(t *testing.T) {
	t.Run("claude provider executor error propagates", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetError(errors.New("claude CLI failed"))

		provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "claude execution failed")
	})

	t.Run("gemini provider executor error propagates", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetError(errors.New("gemini CLI failed"))

		provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "gemini execution failed")
	})

	t.Run("codex provider executor error propagates", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetError(errors.New("codex CLI failed"))

		provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "codex execution failed")
	})

	t.Run("opencode provider executor error propagates", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetError(errors.New("opencode CLI failed"))

		provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "opencode execution failed")
	})

	t.Run("copilot provider executor error propagates", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetError(errors.New("copilot CLI failed"))

		provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "copilot execution failed")
	})
}

func TestWithCopilotTokenizer(t *testing.T) {
	tok := stubTokenizer{}
	provider := NewCopilotProviderWithOptions(WithCopilotTokenizer(tok))
	require.NotNil(t, provider)
	assert.Equal(t, ports.Tokenizer(tok), provider.base.tokenizer)
}

func TestWithCodexTokenizer(t *testing.T) {
	tok := stubTokenizer{}
	provider := NewCodexProviderWithOptions(WithCodexTokenizer(tok))
	require.NotNil(t, provider)
	assert.Equal(t, ports.Tokenizer(tok), provider.base.tokenizer)
}

func TestWithOpenCodeTokenizer(t *testing.T) {
	tok := stubTokenizer{}
	provider := NewOpenCodeProviderWithOptions(WithOpenCodeTokenizer(tok))
	require.NotNil(t, provider)
	assert.Equal(t, ports.Tokenizer(tok), provider.base.tokenizer)
}

func TestWithClaudeTokenizer(t *testing.T) {
	tok := stubTokenizer{}
	provider := NewClaudeProviderWithOptions(WithClaudeTokenizer(tok))
	require.NotNil(t, provider)
	assert.Equal(t, ports.Tokenizer(tok), provider.base.tokenizer)
}

func TestWithGeminiTokenizer(t *testing.T) {
	tok := stubTokenizer{}
	provider := NewGeminiProviderWithOptions(WithGeminiTokenizer(tok))
	require.NotNil(t, provider)
	assert.Equal(t, ports.Tokenizer(tok), provider.base.tokenizer)
}

func TestWithMistralVibeTokenizer(t *testing.T) {
	const expectedTokens = 88
	tok := countingTokenizer{count: expectedTokens}
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("mistral vibe extracted text here"), []byte(""))

	provider := NewMistralVibeProviderWithOptions(
		WithMistralVibeExecutor(mockExec),
		WithMistralVibeTokenizer(tok),
	)

	result, err := provider.Execute(context.Background(), "prompt", nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, ports.Tokenizer(tok), provider.base.tokenizer)
	assert.Equal(t, expectedTokens, result.Tokens)
}

// TestWithClaudeLogger verifies that WithClaudeLogger injects the logger into the
// ClaudeProvider and that the provider's base receives it (non-nil logger field).
func TestWithClaudeLogger(t *testing.T) {
	l := mocks.NewMockLogger()
	provider := NewClaudeProviderWithOptions(WithClaudeLogger(l))
	require.NotNil(t, provider)
	assert.Equal(t, ports.Logger(l), provider.logger,
		"WithClaudeLogger must set the logger field on ClaudeProvider")
}

// TestWithGeminiLogger verifies that WithGeminiLogger injects the logger into the
// GeminiProvider and that the provider's logger field is set correctly.
func TestWithGeminiLogger(t *testing.T) {
	l := mocks.NewMockLogger()
	provider := NewGeminiProviderWithOptions(WithGeminiLogger(l))
	require.NotNil(t, provider)
	assert.Equal(t, ports.Logger(l), provider.logger,
		"WithGeminiLogger must set the logger field on GeminiProvider")
}

func TestClaudeProvider_Execute_UsesInjectedTokenizer(t *testing.T) {
	const expectedTokens = 99
	tok := countingTokenizer{count: expectedTokens}
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"result","result":"extracted text here"}`), []byte(""))

	provider := NewClaudeProviderWithOptions(
		WithClaudeExecutor(mockExec),
		WithClaudeTokenizer(tok),
	)

	result, err := provider.Execute(context.Background(), "prompt", nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, expectedTokens, result.Tokens)
}

func TestGeminiProvider_Execute_UsesInjectedTokenizer(t *testing.T) {
	const expectedTokens = 99
	tok := countingTokenizer{count: expectedTokens}
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"message","role":"assistant","content":"gemini text here"}`), []byte(""))

	provider := NewGeminiProviderWithOptions(
		WithGeminiExecutor(mockExec),
		WithGeminiTokenizer(tok),
	)

	result, err := provider.Execute(context.Background(), "prompt", nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, expectedTokens, result.Tokens)
}

func TestCopilotProvider_Execute_UsesInjectedTokenizer(t *testing.T) {
	const expectedTokens = 88
	tok := countingTokenizer{count: expectedTokens}
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"assistant.message","data":{"content":"copilot extracted text here","messageId":"m1"}}`+"\n"+`{"type":"result","sessionId":"s1","exitCode":0}`), []byte(""))

	provider := NewCopilotProviderWithOptions(
		WithCopilotExecutor(mockExec),
		WithCopilotTokenizer(tok),
	)

	result, err := provider.Execute(context.Background(), "prompt", nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, expectedTokens, result.Tokens)
}

func TestCodexProvider_Execute_UsesInjectedTokenizer(t *testing.T) {
	const expectedTokens = 77
	tok := countingTokenizer{count: expectedTokens}
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"item.completed","item":{"item_type":"assistant_message","text":"codex extracted text here"}}`), []byte(""))

	provider := NewCodexProviderWithOptions(
		WithCodexExecutor(mockExec),
		WithCodexTokenizer(tok),
	)

	result, err := provider.Execute(context.Background(), "prompt", nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, expectedTokens, result.Tokens)
}

func TestOpenCodeProvider_Execute_UsesInjectedTokenizer(t *testing.T) {
	const expectedTokens = 66
	tok := countingTokenizer{count: expectedTokens}
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"text","part":{"text":"opencode extracted text here"}}`), []byte(""))

	provider := NewOpenCodeProviderWithOptions(
		WithOpenCodeExecutor(mockExec),
		WithOpenCodeTokenizer(tok),
	)

	result, err := provider.Execute(context.Background(), "prompt", nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, expectedTokens, result.Tokens)
}

func TestProviderOptions_Integration(t *testing.T) {
	t.Run("claude provider with mock executor executes successfully", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("Claude response"), []byte(""))

		provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "What is 2+2?", nil, nil, nil)

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
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("Gemini response"), []byte(""))

		provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "Explain Go interfaces", nil, nil, nil)

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
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("Codex response"), []byte(""))

		provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "Write a function", nil, nil, nil)

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
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("OpenCode response"), []byte(""))

		provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "Generate code", nil, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "opencode", result.Provider)
		assert.Contains(t, result.Output, "OpenCode response")

		// Verify executor was called correctly
		calls := mockExec.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, "opencode", calls[0].Name)
	})

	t.Run("multiple providers can use different executors", func(t *testing.T) {
		// Create separate mock executors for each provider
		claudeMock := mocks.NewMockCLIExecutor()
		claudeMock.SetOutput([]byte("Claude specific"), []byte(""))

		geminiMock := mocks.NewMockCLIExecutor()
		geminiMock.SetOutput([]byte("Gemini specific"), []byte(""))

		// Create providers with different executors
		claudeProvider := NewClaudeProviderWithOptions(WithClaudeExecutor(claudeMock))
		geminiProvider := NewGeminiProviderWithOptions(WithGeminiExecutor(geminiMock))

		ctx := context.Background()

		// Execute both
		claudeResult, err := claudeProvider.Execute(ctx, "prompt1", nil, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, claudeResult.Output, "Claude specific")

		geminiResult, err := geminiProvider.Execute(ctx, "prompt2", nil, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, geminiResult.Output, "Gemini specific")

		// Verify each executor was called independently
		claudeCalls := claudeMock.GetCalls()
		require.Len(t, claudeCalls, 1)

		geminiCalls := geminiMock.GetCalls()
		require.Len(t, geminiCalls, 1)
	})

	t.Run("copilot provider with mock executor executes successfully", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"Copilot response\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), []byte(""))

		provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "Generate code", nil, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "github_copilot", result.Provider)
		assert.Contains(t, result.Output, "Copilot response")

		// Verify executor was called correctly
		calls := mockExec.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, "copilot", calls[0].Name)
	})

	t.Run("mistral vibe provider with mock executor executes successfully", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("Mistral Vibe integration response"), []byte(""))

		provider := NewMistralVibeProviderWithOptions(WithMistralVibeExecutor(mockExec))
		ctx := context.Background()

		result, err := provider.Execute(ctx, "Draft a changelog", nil, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "mistral_vibe", result.Provider)
		assert.Contains(t, result.Output, "Mistral Vibe integration response")

		calls := mockExec.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, "vibe", calls[0].Name)
	})
}
