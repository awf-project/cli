package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// knownConfigKeys lists all valid top-level keys in config.yaml.
// Any key not in this list triggers a warning.
var knownConfigKeys = map[string]bool{
	"inputs":        true,
	"version":       true,
	"log_level":     true,
	"output_format": true,
	"notify":        true,
}

// WarningFunc is a callback for reporting non-fatal warnings during config loading.
// The loader calls this for each unknown key found in the config file.
type WarningFunc func(format string, args ...any)

// YAMLConfigLoader loads project configuration from a YAML file.
// The primary config path is .awf/config.yaml relative to project root.
type YAMLConfigLoader struct {
	path   string
	warnFn WarningFunc
}

// NewYAMLConfigLoader creates a new config loader for the given path.
func NewYAMLConfigLoader(path string) *YAMLConfigLoader {
	return &YAMLConfigLoader{path: path}
}

// WithWarningFunc sets a callback for reporting unknown key warnings.
// If not set, unknown keys are silently ignored.
func (l *YAMLConfigLoader) WithWarningFunc(fn WarningFunc) *YAMLConfigLoader {
	l.warnFn = fn
	return l
}

// Path returns the config file path this loader reads from.
func (l *YAMLConfigLoader) Path() string {
	return l.path
}

// Load reads and parses the config file.
//
// Behavior:
//   - Returns empty config if file does not exist (not an error)
//   - Returns ConfigError for invalid YAML syntax
//   - Returns ConfigError for file read errors (permissions, etc.)
//   - Unknown keys in config trigger warnings via WarningFunc (if set)
//   - Unknown keys do NOT cause Load to fail
func (l *YAMLConfigLoader) Load() (*ProjectConfig, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectConfig{}, nil
		}
		return nil, &ConfigError{
			Path:    l.path,
			Op:      "load",
			Message: err.Error(),
			Cause:   err,
		}
	}

	// Check for unknown keys before parsing into struct
	l.checkUnknownKeys(data)

	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, &ConfigError{
			Path:    l.path,
			Op:      "parse",
			Message: err.Error(),
			Cause:   err,
		}
	}

	return &cfg, nil
}

// checkUnknownKeys parses YAML data and warns about any unrecognized top-level keys.
// If warnFn is nil, unknown keys are silently ignored.
func (l *YAMLConfigLoader) checkUnknownKeys(data []byte) {
	if l.warnFn == nil {
		return
	}

	// Parse into generic map to discover all top-level keys
	var rawConfig map[string]any
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		// If parsing fails, don't warn (the main Load will report the error)
		return
	}

	// Check each key against known keys
	for key := range rawConfig {
		if !knownConfigKeys[key] {
			l.warnFn("unknown configuration key %q in %s", key, l.path)
		}
	}
}
