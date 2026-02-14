package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/testutil/mocks"
)

// Component: C025 - T008 - Unit Tests for OpenCodeProvider (WITHOUT integration build tag)
// These tests use MockCLIExecutor to avoid external CLI dependencies

func TestOpenCodeProvider_Execute_Success(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		options    map[string]any
		mockStdout []byte
		wantOutput string
	}{
		{
			name:       "simple prompt",
			prompt:     "Create a function",
			mockStdout: []byte("function created"),
			wantOutput: "function created",
		},
		{
			name:       "prompt with framework option",
			prompt:     "test",
			options:    map[string]any{"framework": "react"},
			mockStdout: []byte("response"),
			wantOutput: "response",
		},
		{
			name:       "prompt with verbose option",
			prompt:     "test",
			options:    map[string]any{"verbose": true},
			mockStdout: []byte("verbose response"),
			wantOutput: "verbose response",
		},
		{
			name:       "prompt with output_dir option",
			prompt:     "test",
			options:    map[string]any{"output_dir": "/tmp/output"},
			mockStdout: []byte("output written"),
			wantOutput: "output written",
		},
		{
			name:       "large output",
			prompt:     "generate",
			mockStdout: []byte("This is a very long output " + string(make([]byte, 1000))),
			wantOutput: "This is a very long output " + string(make([]byte, 1000)),
		},
		{
			name:       "empty output",
			prompt:     "silent",
			mockStdout: []byte(""),
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "opencode", result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			// Token estimation is ~4 chars per token
			expectedTokens := len(tt.wantOutput) / 4
			assert.Equal(t, expectedTokens, result.Tokens)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
			assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt))
		})
	}
}

func TestOpenCodeProvider_Execute_WithOptions(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		options    map[string]any
		mockStdout []byte
	}{
		{
			name:       "framework option",
			prompt:     "test",
			options:    map[string]any{"framework": "react"},
			mockStdout: []byte("ok"),
		},
		{
			name:       "verbose option",
			prompt:     "test",
			options:    map[string]any{"verbose": true},
			mockStdout: []byte("ok"),
		},
		{
			name:       "output_dir option",
			prompt:     "test",
			options:    map[string]any{"output_dir": "/tmp/test"},
			mockStdout: []byte("ok"),
		},
		{
			name:       "multiple options",
			prompt:     "test",
			options:    map[string]any{"framework": "vue", "verbose": true, "output_dir": "/tmp"},
			mockStdout: []byte("ok"),
		},
		{
			name:       "no options",
			prompt:     "test",
			options:    nil,
			mockStdout: []byte("ok"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "opencode", result.Provider)

			// Verify the CLI call captured the correct arguments
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "opencode", calls[0].Name)
			assert.Contains(t, calls[0].Args, "run")
			assert.Contains(t, calls[0].Args, tt.prompt)
		})
	}
}

func TestOpenCodeProvider_Execute_EmptyPrompt(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		wantErr string
	}{
		{
			name:    "empty string",
			prompt:  "",
			wantErr: "prompt cannot be empty",
		},
		{
			name:    "whitespace only",
			prompt:  "   \t\n  ",
			wantErr: "prompt cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, nil)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestOpenCodeProvider_Execute_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		options map[string]any
		wantErr string
	}{
		{
			name:    "invalid output_dir type",
			prompt:  "test",
			options: map[string]any{"output_dir": 123},
			wantErr: "output_dir must be a string",
		},
		{
			name:    "invalid verbose type",
			prompt:  "test",
			options: map[string]any{"verbose": "true"},
			wantErr: "verbose must be a boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestOpenCodeProvider_Execute_ContextErrors(t *testing.T) {
	tests := []struct {
		name    string
		ctxFunc func() context.Context
		wantErr string
	}{
		{
			name: "canceled context",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: "context canceled",
		},
		{
			name: "deadline exceeded context",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Hour))
				defer cancel()
				return ctx
			},
			wantErr: "deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(tt.ctxFunc(), "test prompt", nil)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestOpenCodeProvider_Execute_CLIErrors(t *testing.T) {
	tests := []struct {
		name    string
		mockErr error
		wantErr string
	}{
		{
			name:    "command not found",
			mockErr: errors.New("executable file not found"),
			wantErr: "opencode execution failed",
		},
		{
			name:    "permission denied",
			mockErr: errors.New("permission denied"),
			wantErr: "opencode execution failed",
		},
		{
			name:    "generic error",
			mockErr: errors.New("something went wrong"),
			wantErr: "opencode execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", nil)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestOpenCodeProvider_Execute_StdoutStderr(t *testing.T) {
	tests := []struct {
		name       string
		mockStdout []byte
		mockStderr []byte
		wantOutput string
	}{
		{
			name:       "stdout only",
			mockStdout: []byte("output"),
			mockStderr: nil,
			wantOutput: "output",
		},
		{
			name:       "stderr only",
			mockStdout: nil,
			mockStderr: []byte("error"),
			wantOutput: "error",
		},
		{
			name:       "both stdout and stderr",
			mockStdout: []byte("output"),
			mockStderr: []byte("warning"),
			wantOutput: "outputwarning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, tt.mockStderr)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantOutput, result.Output)
		})
	}
}

func TestOpenCodeProvider_Execute_TokenEstimation(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantTokens int
	}{
		{
			name:       "empty output",
			output:     "",
			wantTokens: 0,
		},
		{
			name:       "short output",
			output:     "test",
			wantTokens: 1,
		},
		{
			name:       "medium output",
			output:     "This is a test output",
			wantTokens: 5,
		},
		{
			name:       "long output",
			output:     string(make([]byte, 400)),
			wantTokens: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(tt.output), nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantTokens, result.Tokens)
		})
	}
}

func TestOpenCodeProvider_Execute_Timestamps(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("test output"), nil)
	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	before := time.Now()
	result, err := provider.Execute(context.Background(), "test prompt", nil)
	after := time.Now()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
	assert.True(t, result.StartedAt.After(before) || result.StartedAt.Equal(before))
	assert.True(t, result.StartedAt.Before(after) || result.StartedAt.Equal(after))
	assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt))
	assert.True(t, result.CompletedAt.Before(after) || result.CompletedAt.Equal(after))
}

func TestOpenCodeProvider_Execute_ProviderName(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("test output"), nil)
	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test prompt", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "opencode", result.Provider)
}

func TestOpenCodeProvider_Execute_JSONDetection(t *testing.T) {
	tests := []struct {
		name         string
		mockOutput   []byte
		wantResponse map[string]any
	}{
		{
			name:         "valid json object",
			mockOutput:   []byte(`{"status":"ok"}`),
			wantResponse: map[string]any{"status": "ok"},
		},
		{
			name:         "json with nested objects",
			mockOutput:   []byte(`{"result":{"count":42,"items":["a","b"]}}`),
			wantResponse: map[string]any{"result": map[string]any{"count": float64(42), "items": []any{"a", "b"}}},
		},
		{
			name:         "non-json response",
			mockOutput:   []byte("plain text"),
			wantResponse: nil,
		},
		{
			name:         "json with whitespace",
			mockOutput:   []byte("\n  {\"key\":\"value\"}  \n"),
			wantResponse: map[string]any{"key": "value"},
		},
		{
			name:         "malformed json",
			mockOutput:   []byte(`{"incomplete`),
			wantResponse: nil,
		},
		{
			name:         "json array (not detected)",
			mockOutput:   []byte(`["item1","item2"]`),
			wantResponse: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			if tt.wantResponse != nil {
				assert.Equal(t, tt.wantResponse, result.Response)
			} else {
				assert.Nil(t, result.Response)
			}
		})
	}
}

func TestOpenCodeProvider_ExecuteConversation_NotImplemented(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
	state := workflow.NewConversationState("")

	result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestOpenCodeProvider_Name(t *testing.T) {
	provider := NewOpenCodeProvider()
	assert.Equal(t, "opencode", provider.Name())
}

func TestOpenCodeProvider_Validate_Success(t *testing.T) {
	provider := NewOpenCodeProvider()
	err := provider.Validate()
	// This test depends on system state (whether opencode CLI is installed)
	// We just verify it doesn't panic and returns either nil or an error
	if err != nil {
		assert.Contains(t, err.Error(), "opencode CLI not found")
	}
}

func TestOpenCodeProvider_NewOpenCodeProvider_DefaultExecutor(t *testing.T) {
	provider := NewOpenCodeProvider()
	require.NotNil(t, provider)
	assert.NotNil(t, provider.executor)
	assert.Equal(t, "opencode", provider.Name())
}

func TestOpenCodeProvider_NewOpenCodeProviderWithOptions(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	require.NotNil(t, provider)
	assert.Equal(t, mockExec, provider.executor)
	assert.Equal(t, "opencode", provider.Name())
}

func TestOpenCodeProvider_Execute_CLIArguments(t *testing.T) {
	tests := []struct {
		name        string
		prompt      string
		options     map[string]any
		wantCmdName string
		wantCmdArgs []string
	}{
		{
			name:        "basic prompt with run subcommand",
			prompt:      "test prompt",
			options:     nil,
			wantCmdName: "opencode",
			wantCmdArgs: []string{"run", "test prompt"},
		},
		{
			name:        "with framework option",
			prompt:      "test",
			options:     map[string]any{"framework": "react"},
			wantCmdName: "opencode",
			wantCmdArgs: []string{"run", "test", "--framework", "react"},
		},
		{
			name:        "with verbose option",
			prompt:      "test",
			options:     map[string]any{"verbose": true},
			wantCmdName: "opencode",
			wantCmdArgs: []string{"run", "test", "--verbose"},
		},
		{
			name:        "with output_dir option",
			prompt:      "test",
			options:     map[string]any{"output_dir": "/tmp/out"},
			wantCmdName: "opencode",
			wantCmdArgs: []string{"run", "test", "--output", "/tmp/out"},
		},
		{
			name:        "with all options",
			prompt:      "test",
			options:     map[string]any{"framework": "vue", "verbose": true, "output_dir": "/tmp"},
			wantCmdName: "opencode",
			wantCmdArgs: []string{"run", "test", "--framework", "vue", "--verbose", "--output", "/tmp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("ok"), nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options)
			require.NoError(t, err)

			// Verify the CLI arguments
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, tt.wantCmdName, calls[0].Name)
			assert.Equal(t, tt.wantCmdArgs, calls[0].Args)
		})
	}
}
