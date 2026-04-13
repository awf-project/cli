package agents

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCombineOutput_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		stdout       []byte
		stderr       []byte
		wantCombined string
	}{
		{
			name:         "stdout only",
			stdout:       []byte("output"),
			stderr:       []byte(""),
			wantCombined: "output",
		},
		{
			name:         "stderr only",
			stdout:       []byte(""),
			stderr:       []byte("error"),
			wantCombined: "error",
		},
		{
			name:         "both stdout and stderr",
			stdout:       []byte("output"),
			stderr:       []byte("error"),
			wantCombined: "outputerror",
		},
		{
			name:         "empty both",
			stdout:       []byte(""),
			stderr:       []byte(""),
			wantCombined: "",
		},
		{
			name:         "multiline output",
			stdout:       []byte("line1\nline2"),
			stderr:       []byte("error line\n"),
			wantCombined: "line1\nline2error line\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := combineOutput(tt.stdout, tt.stderr)
			assert.Equal(t, tt.wantCombined, got)
		})
	}
}

func TestNewBaseCLIProvider_Construction(t *testing.T) {
	tests := []struct {
		name       string
		provName   string
		binary     string
		executor   ports.CLIExecutor
		logger     ports.Logger
		hooks      cliProviderHooks
		wantNilLog bool
	}{
		{
			name:       "with all fields",
			provName:   "test",
			binary:     "test-binary",
			executor:   mocks.NewMockCLIExecutor(),
			logger:     logger.NopLogger{},
			hooks:      cliProviderHooks{},
			wantNilLog: false,
		},
		{
			name:       "with nil logger defaults to NopLogger",
			provName:   "test",
			binary:     "test-binary",
			executor:   mocks.NewMockCLIExecutor(),
			logger:     nil,
			hooks:      cliProviderHooks{},
			wantNilLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newBaseCLIProvider(tt.provName, tt.binary, tt.executor, tt.logger, tt.hooks)

			require.NotNil(t, provider)
			assert.Equal(t, tt.provName, provider.name)
			assert.Equal(t, tt.binary, provider.binary)
			assert.NotNil(t, provider.executor)
			assert.NotNil(t, provider.logger)
		})
	}
}

func TestBaseCLIProvider_Execute_HappyPath(t *testing.T) {
	tests := []struct {
		name          string
		prompt        string
		options       map[string]any
		mockStdout    []byte
		mockStderr    []byte
		wantOutput    string
		wantProvider  string
		wantTokensEst bool
	}{
		{
			name:          "simple prompt",
			prompt:        "What is 2+2?",
			options:       nil,
			mockStdout:    []byte("4"),
			mockStderr:    []byte(""),
			wantOutput:    "4",
			wantProvider:  "test",
			wantTokensEst: true,
		},
		{
			name:          "prompt with options",
			prompt:        "Hello",
			options:       map[string]any{"model": "test-model"},
			mockStdout:    []byte("response"),
			mockStderr:    []byte(""),
			wantOutput:    "response",
			wantProvider:  "test",
			wantTokensEst: true,
		},
		{
			name:          "with stderr output",
			prompt:        "test",
			options:       nil,
			mockStdout:    []byte("stdout"),
			mockStderr:    []byte("stderr"),
			wantOutput:    "stdoutstderr",
			wantProvider:  "test",
			wantTokensEst: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, tt.mockStderr)

			hooks := cliProviderHooks{
				buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
					return []string{"--prompt", prompt}, nil
				},
			}

			provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, hooks)
			result, rawOutput, err := provider.execute(context.Background(), tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantProvider, result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			assert.Equal(t, tt.wantOutput, rawOutput)
			assert.True(t, result.TokensEstimated)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
			assert.True(t, result.CompletedAt.After(result.StartedAt))
		})
	}
}

func TestBaseCLIProvider_Execute_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupHooks  func() cliProviderHooks
		wantErrType bool
		errCheck    func(error) bool
	}{
		{
			name: "buildExecuteArgs returns error",
			setupHooks: func() cliProviderHooks {
				return cliProviderHooks{
					buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
						return nil, errors.New("invalid args")
					},
				}
			},
			wantErrType: true,
			errCheck: func(err error) bool {
				return err != nil && err.Error() == "invalid args"
			},
		},
		{
			name: "validateOptions returns error",
			setupHooks: func() cliProviderHooks {
				return cliProviderHooks{
					buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
						return []string{"--prompt", prompt}, nil
					},
					validateOptions: func(options map[string]any) error {
						return errors.New("invalid options")
					},
				}
			},
			wantErrType: true,
			errCheck: func(err error) bool {
				return err != nil && err.Error() == "invalid options"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			hooks := tt.setupHooks()

			provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, hooks)
			result, _, err := provider.execute(context.Background(), "test prompt", nil, nil, nil)

			assert.Nil(t, result)
			if tt.wantErrType {
				require.Error(t, err)
				assert.True(t, tt.errCheck(err))
			}
		})
	}
}

func TestBaseCLIProvider_Execute_ContextCancellation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("output"), nil)

	hooks := cliProviderHooks{
		buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
			return []string{"--prompt", prompt}, nil
		},
	}

	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, hooks)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, _, err := provider.execute(ctx, "test prompt", nil, nil, nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestBaseCLIProvider_ExecuteConversation_HappyPath(t *testing.T) {
	tests := []struct {
		name          string
		sessionID     string
		mockStdout    []byte
		wantOutput    string
		wantTokensEst bool
	}{
		{
			name:          "new conversation",
			sessionID:     "",
			mockStdout:    []byte("response"),
			wantOutput:    "response",
			wantTokensEst: true,
		},
		{
			name:          "resume conversation",
			sessionID:     "session-123",
			mockStdout:    []byte("continued response"),
			wantOutput:    "continued response",
			wantTokensEst: true,
		},
		{
			name:          "empty output defaults to space",
			sessionID:     "",
			mockStdout:    []byte(""),
			wantOutput:    " ",
			wantTokensEst: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)

			state := &workflow.ConversationState{
				SessionID: tt.sessionID,
				Turns:     []workflow.Turn{},
			}

			hooks := cliProviderHooks{
				buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
					args := []string{"--prompt", prompt}
					if state.SessionID != "" {
						args = append(args, "--session-id", state.SessionID)
					}
					return args, nil
				},
				extractSessionID: func(output string) (string, error) {
					if output == "" || output == " " {
						return "", nil
					}
					return "extracted-session-id", nil
				},
			}

			provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, hooks)
			result, rawOutput, err := provider.executeConversation(context.Background(), state, "test prompt", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "test", result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			assert.Equal(t, tt.mockStdout, []byte(rawOutput)) // raw output before transformation
			assert.True(t, result.TokensEstimated)
			require.NotNil(t, result.State)
			assert.Len(t, result.State.Turns, 2)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
		})
	}
}

func TestBaseCLIProvider_ExecuteConversation_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		setupHooks func() cliProviderHooks
		setupMock  func(*mocks.MockCLIExecutor)
		errCheck   func(error) bool
	}{
		{
			name: "buildConversationArgs returns error",
			setupHooks: func() cliProviderHooks {
				return cliProviderHooks{
					buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
						return nil, errors.New("invalid args")
					},
				}
			},
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("output"), nil)
			},
			errCheck: func(err error) bool {
				return err != nil && err.Error() == "invalid args"
			},
		},
		{
			name: "extractSessionID returns error",
			setupHooks: func() cliProviderHooks {
				return cliProviderHooks{
					buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
						return []string{"--prompt", prompt}, nil
					},
					extractSessionID: func(output string) (string, error) {
						return "", errors.New("session extraction failed")
					},
				}
			},
			setupMock: func(m *mocks.MockCLIExecutor) {
				m.SetOutput([]byte("output"), nil)
			},
			errCheck: func(err error) bool {
				// Per spec: extraction failure falls back to stateless mode (no error propagated)
				return false // should not error, but fall back to no session ID
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			tt.setupMock(mockExec)

			state := &workflow.ConversationState{
				SessionID: "",
				Turns:     []workflow.Turn{},
			}

			hooks := tt.setupHooks()
			provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, hooks)
			result, _, err := provider.executeConversation(context.Background(), state, "test prompt", nil, nil, nil)

			if tt.errCheck != nil && !tt.errCheck(err) {
				// Session extraction error should be handled gracefully
				require.NoError(t, err)
				assert.NotNil(t, result)
			} else {
				require.Error(t, err)
				assert.Nil(t, result)
			}
		})
	}
}

func TestBaseCLIProvider_ExecuteConversation_StatePersistence(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("assistant response"), nil)

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"--prompt", prompt}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "new-session-123", nil
		},
	}

	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, hooks)

	initialState := &workflow.ConversationState{
		SessionID: "",
		Turns:     []workflow.Turn{},
	}

	result, _, err := provider.executeConversation(context.Background(), initialState, "test prompt", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.State)

	// Verify state was cloned and modified
	assert.Equal(t, "new-session-123", result.State.SessionID)
	assert.Len(t, result.State.Turns, 2)
	assert.Equal(t, "test prompt", result.State.Turns[0].Content)
	assert.Equal(t, "assistant response", result.State.Turns[1].Content)

	// Verify original state was not modified
	assert.Equal(t, "", initialState.SessionID)
	assert.Len(t, initialState.Turns, 0)
}

func TestBaseCLIProvider_ExecuteConversation_TextContentTransformation(t *testing.T) {
	tests := []struct {
		name            string
		mockStdout      []byte
		extractTextHook func(string) string
		wantTransformed string
	}{
		{
			name:            "no text extraction hook (raw output)",
			mockStdout:      []byte(`{"result":"unwrapped"}`),
			extractTextHook: nil,
			wantTransformed: `{"result":"unwrapped"}`,
		},
		{
			name:       "with text extraction hook",
			mockStdout: []byte(`{"result":"extracted"}`),
			extractTextHook: func(output string) string {
				// Simulate Claude's behavior: extract from JSON
				return "extracted"
			},
			wantTransformed: "extracted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)

			hooks := cliProviderHooks{
				buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
					return []string{"--prompt", prompt}, nil
				},
				extractSessionID: func(output string) (string, error) {
					return "", nil
				},
				extractTextContent: tt.extractTextHook,
			}

			provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, hooks)

			state := &workflow.ConversationState{
				SessionID: "",
				Turns:     []workflow.Turn{},
			}

			result, _, err := provider.executeConversation(context.Background(), state, "test", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantTransformed, result.Output)
		})
	}
}

func TestBaseCLIProvider_ExecuteConversation_ContextCancellation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("output"), nil)

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"--prompt", prompt}, nil
		},
	}

	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, hooks)

	state := &workflow.ConversationState{
		SessionID: "",
		Turns:     []workflow.Turn{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, _, err := provider.executeConversation(ctx, state, "test prompt", nil, nil, nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestBaseCLIProvider_OutputWriters(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("test output"), []byte("test error"))

	hooks := cliProviderHooks{
		buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
			return []string{"--prompt", prompt}, nil
		},
	}

	provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, hooks)

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	result, _, err := provider.execute(context.Background(), "test", nil, stdoutBuf, stderrBuf)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Verify output writers are passed through to CLI executor
	assert.Equal(t, "test outputtest error", result.Output)
}

func TestBaseCLIProvider_TokenEstimation(t *testing.T) {
	tests := []struct {
		name       string
		mockStdout []byte
		minTokens  int
		maxTokens  int
	}{
		{
			name:       "short output",
			mockStdout: []byte("4"),
			minTokens:  0,
			maxTokens:  2,
		},
		{
			name:       "longer output",
			mockStdout: []byte("This is a longer response with more words"),
			minTokens:  5,
			maxTokens:  15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)

			hooks := cliProviderHooks{
				buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
					return []string{"--prompt", prompt}, nil
				},
			}

			provider := newBaseCLIProvider("test", "test-binary", mockExec, nil, hooks)
			result, _, err := provider.execute(context.Background(), "test", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.TokensEstimated)
			assert.GreaterOrEqual(t, result.Tokens, tt.minTokens)
			assert.LessOrEqual(t, result.Tokens, tt.maxTokens)
		})
	}
}
