package config

import "fmt"

// ProjectConfig holds the project-level configuration loaded from .awf/config.yaml.
// It provides default values that can be overridden by CLI flags.
type ProjectConfig struct {
	// Inputs contains pre-populated workflow input values.
	// These are merged with CLI --input flags, with CLI taking precedence.
	Inputs map[string]any `yaml:"inputs"`

	// Notify holds notification backend configuration.
	// Loaded from .awf/config.yaml under "notify:" key.
	// Type is defined in internal/infrastructure/notify/types.go
	Notify struct {
		DefaultBackend string `yaml:"default_backend"`
	} `yaml:"notify"`
}

// ConfigError represents an error during config file operations.
type ConfigError struct {
	Path    string // config file path (may be empty)
	Op      string // operation: "load", "parse", "validate"
	Message string // human-readable error message
	Cause   error  // underlying error (optional)
}

// Error implements the error interface.
func (e *ConfigError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s: %s", e.Op, e.Path, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *ConfigError) Unwrap() error {
	return e.Cause
}
