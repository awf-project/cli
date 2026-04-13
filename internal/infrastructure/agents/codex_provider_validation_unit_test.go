package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for validateCodexOptions and isValidCodexModel
// Implements F081: Model Validation by Prefix/Pattern for Codex provider
// Scope: US2 (Codex Users Get Typo Protection) + US3 (Clear Error Messages)

func TestIsValidCodexModel_ValidModels(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{
			name:  "gpt-4o",
			model: "gpt-4o",
			want:  true,
		},
		{
			name:  "gpt-3.5-turbo",
			model: "gpt-3.5-turbo",
			want:  true,
		},
		{
			name:  "gpt-4-turbo",
			model: "gpt-4-turbo",
			want:  true,
		},
		{
			name:  "gpt-4",
			model: "gpt-4",
			want:  true,
		},
		{
			name:  "codex-mini",
			model: "codex-mini",
			want:  true,
		},
		{
			name:  "codex-002",
			model: "codex-002",
			want:  true,
		},
		{
			name:  "o1",
			model: "o1",
			want:  true,
		},
		{
			name:  "o3",
			model: "o3",
			want:  true,
		},
		{
			name:  "o4-mini",
			model: "o4-mini",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidCodexModel(tt.model)
			assert.Equal(t, tt.want, got, "isValidCodexModel(%q) should return %v", tt.model, tt.want)
		})
	}
}

func TestIsValidCodexModel_InvalidModels(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{
			name:  "claude-3-opus (cross-provider)",
			model: "claude-3-opus",
			want:  false,
		},
		{
			name:  "gemini-pro (cross-provider)",
			model: "gemini-pro",
			want:  false,
		},
		{
			name:  "empty string",
			model: "",
			want:  false,
		},
		{
			name:  "toto (random invalid)",
			model: "toto",
			want:  false,
		},
		{
			name:  "o (single char, no digit)",
			model: "o",
			want:  false,
		},
		{
			name:  "ollama (starts with o but second char is not digit)",
			model: "ollama",
			want:  false,
		},
		{
			name:  "oracle (starts with o but second char is not digit)",
			model: "oracle",
			want:  false,
		},
		{
			name:  "GPT-4 (wrong case)",
			model: "GPT-4",
			want:  false,
		},
		{
			name:  "gpt (no dash or suffix)",
			model: "gpt",
			want:  false,
		},
		{
			name:  "codex (no dash or suffix)",
			model: "codex",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidCodexModel(tt.model)
			assert.Equal(t, tt.want, got, "isValidCodexModel(%q) should return %v", tt.model, tt.want)
		})
	}
}

func TestValidateCodexOptions_ValidModel(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
	}{
		{
			name:    "model gpt-4o",
			options: map[string]any{"model": "gpt-4o"},
			wantErr: false,
		},
		{
			name:    "model gpt-3.5-turbo",
			options: map[string]any{"model": "gpt-3.5-turbo"},
			wantErr: false,
		},
		{
			name:    "model codex-mini",
			options: map[string]any{"model": "codex-mini"},
			wantErr: false,
		},
		{
			name:    "model o1",
			options: map[string]any{"model": "o1"},
			wantErr: false,
		},
		{
			name:    "model o4-mini",
			options: map[string]any{"model": "o4-mini"},
			wantErr: false,
		},
		{
			name:    "nil options",
			options: nil,
			wantErr: false,
		},
		{
			name:    "empty options",
			options: map[string]any{},
			wantErr: false,
		},
		{
			name:    "options without model key",
			options: map[string]any{"temperature": 0.5},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCodexOptions(tt.options)

			if tt.wantErr {
				assert.Error(t, err, "validateCodexOptions should return error")
			} else {
				assert.NoError(t, err, "validateCodexOptions should not return error")
			}
		})
	}
}

func TestValidateCodexOptions_InvalidModel(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
		errMsg  string
	}{
		{
			name:    "model claude-3-opus",
			options: map[string]any{"model": "claude-3-opus"},
			wantErr: true,
			errMsg:  "invalid model format",
		},
		{
			name:    "model gemini-pro",
			options: map[string]any{"model": "gemini-pro"},
			wantErr: true,
			errMsg:  "invalid model format",
		},
		{
			name:    "model toto",
			options: map[string]any{"model": "toto"},
			wantErr: true,
			errMsg:  "invalid model format",
		},
		{
			name:    "model empty string",
			options: map[string]any{"model": ""},
			wantErr: true,
			errMsg:  "invalid model format",
		},
		{
			name:    "model ollama",
			options: map[string]any{"model": "ollama"},
			wantErr: true,
			errMsg:  "invalid model format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCodexOptions(tt.options)

			require.Error(t, err, "validateCodexOptions should return error for %q", tt.options["model"])
			assert.Contains(t, err.Error(), tt.errMsg, "error message should contain expected text")
			assert.Contains(t, err.Error(), "gpt-", "error should mention gpt- prefix")
		})
	}
}

func TestValidateCodexOptions_ErrorMessageQuality(t *testing.T) {
	// Verify error messages are clear and actionable per US3
	tests := []struct {
		name      string
		options   map[string]any
		checkText []string
	}{
		{
			name:      "error message includes accepted prefixes",
			options:   map[string]any{"model": "invalid"},
			checkText: []string{"gpt-", "codex-", "o-series"},
		},
		{
			name:      "error message shows invalid model value",
			options:   map[string]any{"model": "toto"},
			checkText: []string{"toto"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCodexOptions(tt.options)
			require.Error(t, err)

			for _, text := range tt.checkText {
				assert.Contains(t, err.Error(), text, "error message should include %q", text)
			}
		})
	}
}

func TestValidateCodexOptions_WithOtherOptions(t *testing.T) {
	// Verify validation only checks model, ignores other options
	options := map[string]any{
		"model":       "gpt-4o",
		"temperature": 2.5,     // Invalid temp
		"max_tokens":  -100,    // Invalid tokens
		"unknown":     "value", // Unknown option
	}

	err := validateCodexOptions(options)
	assert.NoError(t, err, "validateCodexOptions should only validate model, not other options")
}
