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

// TestSecretMasker_MaskText_HappyPath tests normal usage scenarios for text masking
// Feature: C011 - Task T011
func TestSecretMasker_MaskText_HappyPath(t *testing.T) {
	masker := NewSecretMasker()

	tests := []struct {
		name string
		text string
		env  map[string]string
		want string
	}{
		{
			name: "mask SECRET_ prefix in text",
			text: "Token is SECRET_API_TOKEN=super_secret_value_12345",
			env: map[string]string{
				"SECRET_API_TOKEN": "super_secret_value_12345",
				"PUBLIC_VAR":       "visible",
			},
			want: "Token is SECRET_API_TOKEN=***",
		},
		{
			name: "mask API_KEY prefix in text",
			text: "Using API_KEY_OPENAI=sk-proj-abc123def456 for requests",
			env: map[string]string{
				"API_KEY_OPENAI": "sk-proj-abc123def456",
			},
			want: "Using API_KEY_OPENAI=*** for requests",
		},
		{
			name: "mask PASSWORD prefix in text",
			text: "Database PASSWORD_DB=admin_pass_987654 configured",
			env: map[string]string{
				"PASSWORD_DB": "admin_pass_987654",
			},
			want: "Database PASSWORD_DB=*** configured",
		},
		{
			name: "mask multiple secrets in same text",
			text: "SECRET_TOKEN=abc123 and API_KEY=xyz789",
			env: map[string]string{
				"SECRET_TOKEN": "abc123",
				"API_KEY":      "xyz789",
			},
			want: "SECRET_TOKEN=*** and API_KEY=***",
		},
		{
			name: "preserve non-secret values",
			text: "PUBLIC_VAR=visible and USERNAME=john",
			env: map[string]string{
				"PUBLIC_VAR": "visible",
				"USERNAME":   "john",
			},
			want: "PUBLIC_VAR=visible and USERNAME=john",
		},
		{
			name: "mask only secret values not keys",
			text: "The secret is super_secret_value_12345 here",
			env: map[string]string{
				"SECRET_API_TOKEN": "super_secret_value_12345",
			},
			want: "The secret is *** here",
		},
		{
			name: "text without secrets unchanged",
			text: "No secrets in this text",
			env: map[string]string{
				"SECRET_KEY": "hidden",
			},
			want: "No secrets in this text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := masker.MaskText(tt.text, tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSecretMasker_MaskText_EdgeCases tests boundary conditions and unusual inputs
// Feature: C011 - Task T011
func TestSecretMasker_MaskText_EdgeCases(t *testing.T) {
	masker := NewSecretMasker()

	tests := []struct {
		name string
		text string
		env  map[string]string
		want string
	}{
		{
			name: "empty text",
			text: "",
			env: map[string]string{
				"SECRET_KEY": "value",
			},
			want: "",
		},
		{
			name: "empty env",
			text: "SECRET_KEY=value",
			env:  map[string]string{},
			want: "SECRET_KEY=value",
		},
		{
			name: "nil env",
			text: "SECRET_KEY=value",
			env:  nil,
			want: "SECRET_KEY=value",
		},
		{
			name: "secret appears multiple times",
			text: "Token secret123 used twice: secret123",
			env: map[string]string{
				"SECRET_TOKEN": "secret123",
			},
			want: "Token *** used twice: ***",
		},
		{
			name: "overlapping secret values",
			text: "Values: abc and abcdef",
			env: map[string]string{
				"SECRET_A": "abc",
				"SECRET_B": "abcdef",
			},
			want: "Values: *** and ***",
		},
		{
			name: "secret value with special regex characters",
			text: "Password is p@ss.w0rd+123",
			env: map[string]string{
				"PASSWORD": "p@ss.w0rd+123",
			},
			want: "Password is ***",
		},
		{
			name: "empty secret value",
			text: "SECRET_KEY= is empty",
			env: map[string]string{
				"SECRET_KEY": "",
			},
			want: "SECRET_KEY= is empty",
		},
		{
			name: "very long secret value",
			text: "Token is " + string(make([]byte, 1000)),
			env: map[string]string{
				"SECRET_TOKEN": string(make([]byte, 1000)),
			},
			want: "Token is ***",
		},
		{
			name: "multiline text with secrets",
			text: "Line1: SECRET_KEY=abc123\nLine2: Normal text\nLine3: API_KEY=xyz789",
			env: map[string]string{
				"SECRET_KEY": "abc123",
				"API_KEY":    "xyz789",
			},
			want: "Line1: SECRET_KEY=***\nLine2: Normal text\nLine3: API_KEY=***",
		},
		{
			name: "secret value at text boundaries",
			text: "secret123",
			env: map[string]string{
				"SECRET_TOKEN": "secret123",
			},
			want: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := masker.MaskText(tt.text, tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSecretMasker_MaskText_CaseInsensitive tests case-insensitive key matching
// Feature: C011 - Task T011
func TestSecretMasker_MaskText_CaseInsensitive(t *testing.T) {
	masker := NewSecretMasker()

	tests := []struct {
		name string
		text string
		env  map[string]string
		want string
	}{
		{
			name: "lowercase secret_ prefix",
			text: "Value is secret_key=hidden123",
			env: map[string]string{
				"secret_key": "hidden123",
			},
			want: "Value is secret_key=***",
		},
		{
			name: "mixed case api_key prefix",
			text: "Using Api_Key_OpenAI=xyz",
			env: map[string]string{
				"Api_Key_OpenAI": "xyz",
			},
			want: "Using Api_Key_OpenAI=***",
		},
		{
			name: "uppercase PASSWORD prefix",
			text: "PASSWORD=pass123",
			env: map[string]string{
				"PASSWORD": "pass123",
			},
			want: "PASSWORD=***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := masker.MaskText(tt.text, tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSecretMasker_MaskText_ErrorHandling tests error conditions and invalid inputs
// Feature: C011 - Task T011
func TestSecretMasker_MaskText_ErrorHandling(t *testing.T) {
	masker := NewSecretMasker()

	tests := []struct {
		name string
		text string
		env  map[string]string
		want string
	}{
		{
			name: "non-secret env vars ignored",
			text: "PUBLIC_VAR=visible NORMAL=ok",
			env: map[string]string{
				"PUBLIC_VAR": "visible",
				"NORMAL":     "ok",
			},
			want: "PUBLIC_VAR=visible NORMAL=ok",
		},
		{
			name: "partial prefix match not masked",
			text: "SECRETTOKEN=value APIKEY=value", // no underscore
			env: map[string]string{
				"SECRETTOKEN": "value",
				"APIKEY":      "value",
			},
			want: "SECRETTOKEN=value APIKEY=value",
		},
		{
			name: "secret key not in text",
			text: "This text has no secrets",
			env: map[string]string{
				"SECRET_KEY": "hidden",
			},
			want: "This text has no secrets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := masker.MaskText(tt.text, tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSecretMasker_MaskText_RealWorldScenarios tests realistic command output patterns
// Feature: C011 - Task T011
func TestSecretMasker_MaskText_RealWorldScenarios(t *testing.T) {
	masker := NewSecretMasker()

	tests := []struct {
		name string
		text string
		env  map[string]string
		want string
	}{
		{
			name: "environment variable listing",
			text: `SECRET_API_TOKEN=super_secret_value_12345
API_KEY_OPENAI=sk-proj-abc123
PASSWORD_DB=admin_pass_987
PUBLIC_VAR=visible_value
USERNAME=user
NORMAL_VAR=normal`,
			env: map[string]string{
				"SECRET_API_TOKEN": "super_secret_value_12345",
				"API_KEY_OPENAI":   "sk-proj-abc123",
				"PASSWORD_DB":      "admin_pass_987",
				"PUBLIC_VAR":       "visible_value",
				"USERNAME":         "user",
				"NORMAL_VAR":       "normal",
			},
			want: `SECRET_API_TOKEN=***
API_KEY_OPENAI=***
PASSWORD_DB=***
PUBLIC_VAR=visible_value
USERNAME=user
NORMAL_VAR=normal`,
		},
		{
			name: "command execution error with secret",
			text: `Error: Authentication failed with token super_secret_value_12345
Connection refused`,
			env: map[string]string{
				"SECRET_API_TOKEN": "super_secret_value_12345",
			},
			want: `Error: Authentication failed with token ***
Connection refused`,
		},
		{
			name: "JSON output with secrets",
			text: `{"api_key":"sk-proj-abc123","user":"john","password":"admin_pass"}`,
			env: map[string]string{
				"API_KEY_OPENAI": "sk-proj-abc123",
				"PASSWORD_DB":    "admin_pass",
			},
			want: `{"api_key":"***","user":"john","password":"***"}`,
		},
		{
			name: "shell command with secret arguments",
			text: `curl -H "Authorization: Bearer sk-proj-abc123" https://api.example.com`,
			env: map[string]string{
				"API_KEY": "sk-proj-abc123",
			},
			want: `curl -H "Authorization: Bearer ***" https://api.example.com`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := masker.MaskText(tt.text, tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}
