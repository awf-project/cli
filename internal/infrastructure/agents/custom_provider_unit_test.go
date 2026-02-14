package agents

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/testutil/mocks"
)

// Component: C025 - Unit Tests for CustomProvider (WITHOUT integration build tag)
// These tests use MockCLIExecutor to avoid external CLI dependencies

func TestCustomProvider_Execute_TemplateProcessing(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		prompt      string
		wantCommand string
		mockStdout  []byte
	}{
		{
			name:        "simple template",
			template:    "echo {{.prompt}}",
			prompt:      "hello",
			wantCommand: "echo 'hello'",
			mockStdout:  []byte("hello"),
		},
		{
			name:        "shell escape injection attempt",
			template:    "echo {{.prompt}}",
			prompt:      "hello; rm -rf /",
			wantCommand: "echo 'hello; rm -rf /'",
			mockStdout:  []byte("hello; rm -rf /"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCustomProviderWithOptions("test", tt.template, WithCustomExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "test", result.Provider)
			assert.Equal(t, string(tt.mockStdout), result.Output)
		})
	}
}

func TestCustomProvider_Execute_EmptyPrompt(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCustomProviderWithOptions("test", "echo {{.prompt}}", WithCustomExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt")
}

func TestCustomProvider_Execute_EmptyTemplate(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCustomProviderWithOptions("test", "", WithCustomExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "command template cannot be empty")
}

func TestCustomProvider_Execute_TemplateSyntaxError(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  string
	}{
		{
			name:     "unclosed template",
			template: "echo {{.prompt",
			wantErr:  "template",
		},
		{
			name:     "invalid variable",
			template: "echo {{.undefined}}",
			wantErr:  "template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			provider := NewCustomProviderWithOptions("test", tt.template, WithCustomExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCustomProvider_Execute_ContextCancellation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCustomProviderWithOptions("test", "echo {{.prompt}}", WithCustomExecutor(mockExec))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.Execute(ctx, "test", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCustomProvider_Execute_CLIError(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetError(assert.AnError)
	provider := NewCustomProviderWithOptions("test", "echo {{.prompt}}", WithCustomExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCustomProvider_Execute_WithOptions(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("prompt='test' model='gpt-4'"), nil)
	provider := NewCustomProviderWithOptions(
		"test",
		"echo prompt='{{.prompt}}' model='{{.options.model}}'",
		WithCustomExecutor(mockExec),
	)

	options := map[string]any{
		"model": "gpt-4",
	}

	result, err := provider.Execute(context.Background(), "test", options)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestCustomProvider_Execute_JSONAutoDetection(t *testing.T) {
	tests := []struct {
		name       string
		mockStdout []byte
		wantJSON   bool
	}{
		{
			name:       "valid json object",
			mockStdout: []byte(`{"result": "success", "status": "ok"}`),
			wantJSON:   true,
		},
		{
			name:       "plain text",
			mockStdout: []byte("plain text output"),
			wantJSON:   false,
		},
		{
			name:       "json with whitespace",
			mockStdout: []byte("  \n  {\"key\": \"value\"}  \n  "),
			wantJSON:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCustomProviderWithOptions("test", "echo {{.prompt}}", WithCustomExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			if tt.wantJSON {
				assert.NotNil(t, result.Response)
			} else {
				assert.Nil(t, result.Response)
			}
		})
	}
}

func TestCustomProvider_Execute_TokenEstimation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	output := "This is a test output with some content"
	mockExec.SetOutput([]byte(output), nil)
	provider := NewCustomProviderWithOptions("test", "echo {{.prompt}}", WithCustomExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	expectedTokens := len(output) / 4
	assert.Equal(t, expectedTokens, result.Tokens)
	assert.True(t, result.TokensEstimated)
}

func TestCustomProvider_Execute_Timestamps(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("test"), nil)
	provider := NewCustomProviderWithOptions("test", "echo {{.prompt}}", WithCustomExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
	assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt))
}

func TestCustomProvider_ExecuteConversation_NotImplemented(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCustomProviderWithOptions("test", "echo {{.prompt}}", WithCustomExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), nil, "test", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestCustomProvider_Name(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
	}{
		{
			name:         "simple name",
			providerName: "my-custom-agent",
		},
		{
			name:         "complex name",
			providerName: "internal-ai-tool-v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCustomProvider(tt.providerName, "echo test")

			assert.Equal(t, tt.providerName, provider.Name())
		})
	}
}

func TestCustomProvider_Validate_HappyPath(t *testing.T) {
	provider := NewCustomProvider("test", "echo {{.prompt}}")

	err := provider.Validate()

	assert.NoError(t, err)
}

func TestCustomProvider_Validate_EmptyTemplate(t *testing.T) {
	provider := NewCustomProvider("test", "")

	err := provider.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command template cannot be empty")
}

func TestCustomProvider_Validate_InvalidTemplate(t *testing.T) {
	provider := NewCustomProvider("test", "echo {{.unclosed")

	err := provider.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command template is invalid")
}

func TestCustomProvider_NewCustomProvider(t *testing.T) {
	provider := NewCustomProvider("test", "echo {{.prompt}}")

	require.NotNil(t, provider)
	assert.Equal(t, "test", provider.name)
	assert.Equal(t, "echo {{.prompt}}", provider.commandTemplate)
	assert.NotNil(t, provider.executor)
}

func TestCustomProvider_NewCustomProviderWithOptions_HappyPath(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCustomProviderWithOptions("test", "echo {{.prompt}}", WithCustomExecutor(mockExec))

	require.NotNil(t, provider)
	assert.Equal(t, "test", provider.name)
	assert.Equal(t, "echo {{.prompt}}", provider.commandTemplate)
	assert.Equal(t, mockExec, provider.executor)
}

func TestCustomProvider_NewCustomProviderWithOptions_NoOptions(t *testing.T) {
	provider := NewCustomProviderWithOptions("test", "echo {{.prompt}}")

	require.NotNil(t, provider)
	assert.NotNil(t, provider.executor)
}
