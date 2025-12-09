package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretMasker_MaskFields(t *testing.T) {
	tests := []struct {
		name   string
		fields []any
		want   []any
	}{
		{
			name:   "empty fields",
			fields: []any{},
			want:   []any{},
		},
		{
			name:   "no secrets",
			fields: []any{"name", "test", "count", 42},
			want:   []any{"name", "test", "count", 42},
		},
		{
			name:   "mask SECRET_ prefix",
			fields: []any{"SECRET_TOKEN", "my-secret-value"},
			want:   []any{"SECRET_TOKEN", "***"},
		},
		{
			name:   "mask API_KEY prefix",
			fields: []any{"API_KEY", "sk-12345"},
			want:   []any{"API_KEY", "***"},
		},
		{
			name:   "mask API_KEY_ prefix",
			fields: []any{"API_KEY_OPENAI", "sk-12345"},
			want:   []any{"API_KEY_OPENAI", "***"},
		},
		{
			name:   "mask PASSWORD prefix",
			fields: []any{"PASSWORD", "hunter2"},
			want:   []any{"PASSWORD", "***"},
		},
		{
			name:   "mask PASSWORD_ prefix",
			fields: []any{"PASSWORD_DB", "hunter2"},
			want:   []any{"PASSWORD_DB", "***"},
		},
		{
			name:   "case insensitive SECRET",
			fields: []any{"secret_key", "value"},
			want:   []any{"secret_key", "***"},
		},
		{
			name:   "case insensitive API_KEY",
			fields: []any{"api_key", "value"},
			want:   []any{"api_key", "***"},
		},
		{
			name:   "case insensitive PASSWORD",
			fields: []any{"password", "value"},
			want:   []any{"password", "***"},
		},
		{
			name:   "mixed fields with secrets",
			fields: []any{"user", "john", "API_KEY", "sk-123", "count", 5, "SECRET_TOKEN", "abc"},
			want:   []any{"user", "john", "API_KEY", "***", "count", 5, "SECRET_TOKEN", "***"},
		},
		{
			name:   "odd number of fields preserves structure",
			fields: []any{"key1", "value1", "orphan"},
			want:   []any{"key1", "value1", "orphan"},
		},
		{
			name:   "non-string key is preserved",
			fields: []any{123, "value"},
			want:   []any{123, "value"},
		},
		{
			name:   "non-string value is preserved even for secret key",
			fields: []any{"SECRET_COUNT", 42},
			want:   []any{"SECRET_COUNT", "***"},
		},
	}

	masker := NewSecretMasker()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := masker.MaskFields(tt.fields)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSecretMasker_IsSecretKey(t *testing.T) {
	masker := NewSecretMasker()

	secretKeys := []string{
		"SECRET_TOKEN",
		"SECRET_KEY",
		"secret_value",
		"API_KEY",
		"API_KEY_OPENAI",
		"api_key",
		"PASSWORD",
		"PASSWORD_DB",
		"password",
	}

	for _, key := range secretKeys {
		t.Run(key+" should be secret", func(t *testing.T) {
			assert.True(t, masker.IsSecretKey(key))
		})
	}

	nonSecretKeys := []string{
		"name",
		"user",
		"SECRETTOKEN", // no underscore
		"APIKEY",      // no underscore
		"my_password_field",
		"workflow_id",
	}

	for _, key := range nonSecretKeys {
		t.Run(key+" should not be secret", func(t *testing.T) {
			assert.False(t, masker.IsSecretKey(key))
		})
	}
}

func TestSecretMasker_CustomPatterns(t *testing.T) {
	masker := NewSecretMasker("TOKEN_", "CREDENTIAL")

	tests := []struct {
		key      string
		isSecret bool
	}{
		{"TOKEN_AUTH", true},
		{"CREDENTIAL", true},
		{"SECRET_KEY", true}, // default patterns still work
		{"NORMAL_KEY", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.isSecret, masker.IsSecretKey(tt.key))
		})
	}
}
