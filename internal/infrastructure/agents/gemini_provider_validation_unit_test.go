package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for validateGeminiOptions and isValidGeminiModel
// Implements F081: Model Validation by Prefix/Pattern for Gemini provider
// Scope: US1 (Gemini Users Can Use Any Valid Gemini Model) + US3 (Clear Error Messages)

func TestIsValidGeminiModel_ValidModels(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{
			name:  "gemini-pro",
			model: "gemini-pro",
			want:  true,
		},
		{
			name:  "gemini-pro-vision",
			model: "gemini-pro-vision",
			want:  true,
		},
		{
			name:  "gemini-ultra",
			model: "gemini-ultra",
			want:  true,
		},
		{
			name:  "gemini-2.0-flash",
			model: "gemini-2.0-flash",
			want:  true,
		},
		{
			name:  "gemini-1.5-pro",
			model: "gemini-1.5-pro",
			want:  true,
		},
		{
			name:  "gemini-1.5-pro-latest",
			model: "gemini-1.5-pro-latest",
			want:  true,
		},
		{
			name:  "gemini-1.5-flash",
			model: "gemini-1.5-flash",
			want:  true,
		},
		{
			name:  "gemini-1.5-flash-latest",
			model: "gemini-1.5-flash-latest",
			want:  true,
		},
		{
			name:  "gemini-2.0-pro",
			model: "gemini-2.0-pro",
			want:  true,
		},
		{
			name:  "gemini- (dash alone - provider CLI will reject downstream)",
			model: "gemini-",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidGeminiModel(tt.model)
			assert.Equal(t, tt.want, got, "isValidGeminiModel(%q) should return %v", tt.model, tt.want)
		})
	}
}

func TestIsValidGeminiModel_InvalidModels(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{
			name:  "gpt-4 (cross-provider)",
			model: "gpt-4",
			want:  false,
		},
		{
			name:  "claude-3-opus (cross-provider)",
			model: "claude-3-opus",
			want:  false,
		},
		{
			name:  "codex-mini (cross-provider)",
			model: "codex-mini",
			want:  false,
		},
		{
			name:  "empty string",
			model: "",
			want:  false,
		},
		{
			name:  "gemini (no dash)",
			model: "gemini",
			want:  false,
		},
		{
			name:  "GEMINI-pro (wrong case)",
			model: "GEMINI-pro",
			want:  false,
		},
		{
			name:  "toto (random invalid)",
			model: "toto",
			want:  false,
		},
		{
			name:  "o1 (o-series, not gemini)",
			model: "o1",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidGeminiModel(tt.model)
			assert.Equal(t, tt.want, got, "isValidGeminiModel(%q) should return %v", tt.model, tt.want)
		})
	}
}

func TestValidateGeminiOptions_ValidModel(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
	}{
		{
			name:    "model gemini-pro",
			options: map[string]any{"model": "gemini-pro"},
			wantErr: false,
		},
		{
			name:    "model gemini-2.0-flash",
			options: map[string]any{"model": "gemini-2.0-flash"},
			wantErr: false,
		},
		{
			name:    "model gemini-1.5-pro-latest",
			options: map[string]any{"model": "gemini-1.5-pro-latest"},
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
			options: map[string]any{"temperature": 0.7},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGeminiOptions(tt.options)

			if tt.wantErr {
				assert.Error(t, err, "validateGeminiOptions should return error")
			} else {
				assert.NoError(t, err, "validateGeminiOptions should not return error")
			}
		})
	}
}

func TestValidateGeminiOptions_InvalidModel(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
		errMsg  string
	}{
		{
			name:    "model gpt-4 (cross-provider)",
			options: map[string]any{"model": "gpt-4"},
			wantErr: true,
			errMsg:  "invalid model format",
		},
		{
			name:    "model claude-3-opus (cross-provider)",
			options: map[string]any{"model": "claude-3-opus"},
			wantErr: true,
			errMsg:  "invalid model format",
		},
		{
			name:    "model codex-mini (cross-provider)",
			options: map[string]any{"model": "codex-mini"},
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
			name:    "model gemini (no dash)",
			options: map[string]any{"model": "gemini"},
			wantErr: true,
			errMsg:  "invalid model format",
		},
		{
			name:    "model toto (random invalid)",
			options: map[string]any{"model": "toto"},
			wantErr: true,
			errMsg:  "invalid model format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGeminiOptions(tt.options)

			require.Error(t, err, "validateGeminiOptions should return error for %q", tt.options["model"])
			assert.Contains(t, err.Error(), tt.errMsg, "error message should contain expected text")
			assert.Contains(t, err.Error(), "gemini-", "error should mention gemini- prefix")
		})
	}
}

func TestValidateGeminiOptions_ErrorMessageQuality(t *testing.T) {
	// Verify error messages are clear and actionable per US3
	tests := []struct {
		name      string
		options   map[string]any
		checkText []string
	}{
		{
			name:      "error message includes required prefix",
			options:   map[string]any{"model": "invalid"},
			checkText: []string{"gemini-"},
		},
		{
			name:      "error message shows invalid model value",
			options:   map[string]any{"model": "gpt-4"},
			checkText: []string{"gpt-4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGeminiOptions(tt.options)
			require.Error(t, err)

			for _, text := range tt.checkText {
				assert.Contains(t, err.Error(), text, "error message should include %q", text)
			}
		})
	}
}

func TestValidateGeminiOptions_WithOtherOptions(t *testing.T) {
	// Verify validation only checks model, ignores other options
	options := map[string]any{
		"model":       "gemini-pro",
		"temperature": 2.5,     // Invalid temp
		"max_tokens":  -100,    // Invalid tokens
		"unknown":     "value", // Unknown option
	}

	err := validateGeminiOptions(options)
	assert.NoError(t, err, "validateGeminiOptions should only validate model, not other options")
}

func TestValidateGeminiOptions_BackwardCompatibility(t *testing.T) {
	// Verify previously valid models continue to work
	tests := []struct {
		name  string
		model string
	}{
		{"gemini-pro", "gemini-pro"},
		{"gemini-pro-vision", "gemini-pro-vision"},
		{"gemini-ultra", "gemini-ultra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGeminiOptions(map[string]any{"model": tt.model})
			assert.NoError(t, err, "previously valid model %q should continue to pass", tt.model)
		})
	}
}

func TestValidateGeminiOptions_FutureModels(t *testing.T) {
	// Verify future models with gemini- prefix are accepted
	tests := []struct {
		name  string
		model string
	}{
		{"gemini-3.0", "gemini-3.0"},
		{"gemini-3.0-ultra", "gemini-3.0-ultra"},
		{"gemini-custom-future", "gemini-custom-future"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGeminiOptions(map[string]any{"model": tt.model})
			assert.NoError(t, err, "future gemini model %q should be accepted", tt.model)
		})
	}
}
