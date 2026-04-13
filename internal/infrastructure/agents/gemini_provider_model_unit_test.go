package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T001: Gemini model validation — prefix/pattern-based (F081/US1, US3)

func TestIsValidGeminiModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{"gemini prefix is valid", "gemini-pro", true},
		{"gemini prefix with version is valid", "gemini-2.0-flash", true},
		{"gemini prefix with suffix is valid", "gemini-1.5-pro-latest", true},
		{"gemini prefix vision is valid", "gemini-pro-vision", true},
		{"gemini prefix ultra is valid", "gemini-ultra", true},
		{"gpt prefix is invalid", "gpt-4", false},
		{"claude prefix is invalid", "claude-3-opus", false},
		{"no prefix is invalid", "toto", false},
		{"empty string is invalid", "", false},
		{"gemini- prefix alone is valid", "gemini-", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidGeminiModel(tt.model)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateGeminiOptions(t *testing.T) {
	tests := []struct {
		name        string
		options     map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name:    "nil options returns no error",
			options: nil,
			wantErr: false,
		},
		{
			name:    "empty options returns no error",
			options: map[string]any{},
			wantErr: false,
		},
		{
			name:    "valid gemini-pro model passes",
			options: map[string]any{"model": "gemini-pro"},
			wantErr: false,
		},
		{
			name:    "valid gemini-2.0-flash model passes",
			options: map[string]any{"model": "gemini-2.0-flash"},
			wantErr: false,
		},
		{
			name:        "gpt-4 model fails with message",
			options:     map[string]any{"model": "gpt-4"},
			wantErr:     true,
			errContains: "gemini-",
		},
		{
			name:        "claude prefix fails with message",
			options:     map[string]any{"model": "claude-3-opus"},
			wantErr:     true,
			errContains: "gemini-",
		},
		{
			name:        "arbitrary string fails with message",
			options:     map[string]any{"model": "toto"},
			wantErr:     true,
			errContains: "gemini-",
		},
		{
			name:        "empty model string fails",
			options:     map[string]any{"model": ""},
			wantErr:     true,
			errContains: "gemini-",
		},
		{
			name:    "options without model key passes",
			options: map[string]any{"output_format": "json"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGeminiOptions(tt.options)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGeminiProvider_Execute_ModelValidation(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
	}{
		{
			name:    "valid gemini-pro model accepted",
			options: map[string]any{"model": "gemini-pro"},
			wantErr: false,
		},
		{
			name:    "valid gemini-2.0-flash model accepted",
			options: map[string]any{"model": "gemini-2.0-flash"},
			wantErr: false,
		},
		{
			name:    "invalid gpt-4 model rejected",
			options: map[string]any{"model": "gpt-4"},
			wantErr: true,
		},
		{
			name:    "invalid empty model rejected",
			options: map[string]any{"model": ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", tt.options, nil, nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}
