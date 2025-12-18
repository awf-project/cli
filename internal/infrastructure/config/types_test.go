package config

import (
	"errors"
	"testing"
)

func TestProjectConfig_DefaultValues(t *testing.T) {
	cfg := &ProjectConfig{}

	if cfg.Inputs != nil {
		t.Errorf("Inputs = %v, want nil for zero value", cfg.Inputs)
	}
}

func TestProjectConfig_WithInputs(t *testing.T) {
	cfg := &ProjectConfig{
		Inputs: map[string]any{
			"project": "my-project",
			"count":   5,
			"enabled": true,
			"ratio":   0.5,
		},
	}

	if len(cfg.Inputs) != 4 {
		t.Errorf("len(Inputs) = %d, want 4", len(cfg.Inputs))
	}

	tests := []struct {
		key  string
		want any
	}{
		{"project", "my-project"},
		{"count", 5},
		{"enabled", true},
		{"ratio", 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := cfg.Inputs[tt.key]; got != tt.want {
				t.Errorf("Inputs[%q] = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestProjectConfig_EmptyInputs(t *testing.T) {
	cfg := &ProjectConfig{
		Inputs: map[string]any{},
	}

	if cfg.Inputs == nil {
		t.Error("Inputs should not be nil when initialized to empty map")
	}
	if len(cfg.Inputs) != 0 {
		t.Errorf("len(Inputs) = %d, want 0", len(cfg.Inputs))
	}
}

func TestConfigError_Error_WithPath(t *testing.T) {
	tests := []struct {
		name string
		err  *ConfigError
		want string
	}{
		{
			name: "load operation with path",
			err: &ConfigError{
				Path:    ".awf/config.yaml",
				Op:      "load",
				Message: "file not found",
			},
			want: "load: .awf/config.yaml: file not found",
		},
		{
			name: "parse operation with path",
			err: &ConfigError{
				Path:    "/home/user/.awf/config.yaml",
				Op:      "parse",
				Message: "invalid YAML syntax at line 5",
			},
			want: "parse: /home/user/.awf/config.yaml: invalid YAML syntax at line 5",
		},
		{
			name: "validate operation with path",
			err: &ConfigError{
				Path:    "config.yaml",
				Op:      "validate",
				Message: "unknown key 'invalid_field'",
			},
			want: "validate: config.yaml: unknown key 'invalid_field'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("ConfigError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigError_Error_WithoutPath(t *testing.T) {
	tests := []struct {
		name string
		err  *ConfigError
		want string
	}{
		{
			name: "load operation without path",
			err: &ConfigError{
				Op:      "load",
				Message: "permission denied",
			},
			want: "load: permission denied",
		},
		{
			name: "parse operation without path",
			err: &ConfigError{
				Op:      "parse",
				Message: "unexpected EOF",
			},
			want: "parse: unexpected EOF",
		},
		{
			name: "empty path string",
			err: &ConfigError{
				Path:    "",
				Op:      "validate",
				Message: "config is nil",
			},
			want: "validate: config is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("ConfigError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigError_Unwrap(t *testing.T) {
	cause := errors.New("underlying YAML error")
	err := &ConfigError{
		Path:    ".awf/config.yaml",
		Op:      "parse",
		Message: "invalid syntax",
		Cause:   cause,
	}

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestConfigError_Unwrap_NilCause(t *testing.T) {
	err := &ConfigError{
		Path:    ".awf/config.yaml",
		Op:      "load",
		Message: "file not found",
		Cause:   nil,
	}

	if unwrapped := err.Unwrap(); unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
}

func TestConfigError_ErrorsIs(t *testing.T) {
	sentinel := errors.New("sentinel error")
	err := &ConfigError{
		Path:    ".awf/config.yaml",
		Op:      "load",
		Message: "failed",
		Cause:   sentinel,
	}

	if !errors.Is(err, sentinel) {
		t.Error("errors.Is() should return true for wrapped sentinel error")
	}
}

func TestConfigError_ErrorsAs(t *testing.T) {
	cause := &ConfigError{
		Op:      "inner",
		Message: "inner error",
	}
	err := &ConfigError{
		Path:    ".awf/config.yaml",
		Op:      "outer",
		Message: "outer error",
		Cause:   cause,
	}

	var target *ConfigError
	if !errors.As(err, &target) {
		t.Error("errors.As() should return true for *ConfigError")
	}
	if target.Op != "outer" {
		t.Errorf("errors.As() target.Op = %q, want %q", target.Op, "outer")
	}
}

func TestConfigError_ImplementsErrorInterface(t *testing.T) {
	var _ error = &ConfigError{}
}

func TestConfigError_Operations(t *testing.T) {
	ops := []string{"load", "parse", "validate"}

	for _, op := range ops {
		t.Run(op, func(t *testing.T) {
			err := &ConfigError{
				Path:    "config.yaml",
				Op:      op,
				Message: "test error",
			}
			got := err.Error()
			if got == "" {
				t.Error("Error() returned empty string")
			}
			if !containsSubstring(got, op) {
				t.Errorf("Error() = %q, should contain operation %q", got, op)
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
