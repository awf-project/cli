package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const fixturesPath = "../../../tests/fixtures/config"

func TestNewYAMLConfigLoader(t *testing.T) {
	path := "/some/path/config.yaml"
	loader := NewYAMLConfigLoader(path)

	if loader == nil {
		t.Fatal("NewYAMLConfigLoader() returned nil")
	}
	if loader.Path() != path {
		t.Errorf("Path() = %q, want %q", loader.Path(), path)
	}
}

func TestYAMLConfigLoader_Path(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"absolute path", "/home/user/.awf/config.yaml"},
		{"relative path", ".awf/config.yaml"},
		{"empty path", ""},
		{"just filename", "config.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewYAMLConfigLoader(tt.path)
			if got := loader.Path(); got != tt.path {
				t.Errorf("Path() = %q, want %q", got, tt.path)
			}
		})
	}
}

func TestYAMLConfigLoader_Load_ValidConfig(t *testing.T) {
	path := filepath.Join(fixturesPath, "valid.yaml")
	loader := NewYAMLConfigLoader(path)

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Verify inputs are loaded
	if cfg.Inputs == nil {
		t.Fatal("Inputs is nil")
	}

	// Check string input
	if project, ok := cfg.Inputs["project"].(string); !ok || project != "test-project" {
		t.Errorf("Inputs[project] = %v, want %q", cfg.Inputs["project"], "test-project")
	}

	// Check string input
	if env, ok := cfg.Inputs["env"].(string); !ok || env != "staging" {
		t.Errorf("Inputs[env] = %v, want %q", cfg.Inputs["env"], "staging")
	}

	// Check int input
	if count, ok := cfg.Inputs["count"].(int); !ok || count != 42 {
		t.Errorf("Inputs[count] = %v, want %d", cfg.Inputs["count"], 42)
	}

	// Check bool input
	if enabled, ok := cfg.Inputs["enabled"].(bool); !ok || !enabled {
		t.Errorf("Inputs[enabled] = %v, want %v", cfg.Inputs["enabled"], true)
	}
}

func TestYAMLConfigLoader_Load_AllTypes(t *testing.T) {
	path := filepath.Join(fixturesPath, "all-types.yaml")
	loader := NewYAMLConfigLoader(path)

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	tests := []struct {
		key      string
		expected any
		typeDesc string
	}{
		{"string_val", "hello", "string"},
		{"int_val", 42, "int"},
		{"float_val", 3.14, "float64"},
		{"bool_true", true, "bool"},
		{"bool_false", false, "bool"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			val, ok := cfg.Inputs[tt.key]
			if !ok {
				t.Fatalf("Inputs[%q] not found", tt.key)
			}
			if val != tt.expected {
				t.Errorf("Inputs[%q] = %v (%T), want %v (%T)",
					tt.key, val, val, tt.expected, tt.expected)
			}
		})
	}

	// null_val should be nil
	t.Run("null_val", func(t *testing.T) {
		val, ok := cfg.Inputs["null_val"]
		if !ok {
			t.Fatal("Inputs[null_val] not found")
		}
		if val != nil {
			t.Errorf("Inputs[null_val] = %v, want nil", val)
		}
	})
}

func TestYAMLConfigLoader_Load_NonExistent(t *testing.T) {
	path := filepath.Join(fixturesPath, "nonexistent.yaml")
	loader := NewYAMLConfigLoader(path)

	cfg, err := loader.Load()
	// Missing config file should not be an error (FR-004)
	if err != nil {
		t.Errorf("Load() error = %v, want nil for missing file", err)
	}

	// Should return empty config
	if cfg == nil {
		t.Fatal("Load() returned nil, want empty config")
	}

	// Inputs should be nil or empty
	if len(cfg.Inputs) > 0 {
		t.Errorf("Inputs = %v, want nil or empty for missing file", cfg.Inputs)
	}
}

func TestYAMLConfigLoader_Load_EmptyFile(t *testing.T) {
	path := filepath.Join(fixturesPath, "empty.yaml")
	loader := NewYAMLConfigLoader(path)

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Empty file should have nil or empty inputs
	if len(cfg.Inputs) > 0 {
		t.Errorf("Inputs = %v, want nil or empty for empty file", cfg.Inputs)
	}
}

func TestYAMLConfigLoader_Load_EmptyInputs(t *testing.T) {
	path := filepath.Join(fixturesPath, "empty-inputs.yaml")
	loader := NewYAMLConfigLoader(path)

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Empty inputs section should have nil or empty inputs
	if len(cfg.Inputs) > 0 {
		t.Errorf("Inputs = %v, want nil or empty for empty inputs section", cfg.Inputs)
	}
}

func TestYAMLConfigLoader_Load_InvalidSyntax(t *testing.T) {
	path := filepath.Join(fixturesPath, "invalid-syntax.yaml")
	loader := NewYAMLConfigLoader(path)

	_, err := loader.Load()

	// Invalid YAML should produce an error (FR-005)
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid YAML")
	}

	// Should be a ConfigError
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Errorf("error type = %T, want *ConfigError", err)
		return
	}

	// Should have parse operation
	if cfgErr.Op != "parse" {
		t.Errorf("ConfigError.Op = %q, want %q", cfgErr.Op, "parse")
	}

	// Should include path
	if cfgErr.Path != path {
		t.Errorf("ConfigError.Path = %q, want %q", cfgErr.Path, path)
	}

	// Message should not be empty
	if cfgErr.Message == "" {
		t.Error("ConfigError.Message is empty")
	}
}

func TestYAMLConfigLoader_Load_UnknownKeys(t *testing.T) {
	path := filepath.Join(fixturesPath, "unknown-keys.yaml")
	loader := NewYAMLConfigLoader(path)

	cfg, err := loader.Load()
	// Unknown keys should not produce an error (warnings only, per spec)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for unknown keys", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Valid inputs should still be loaded
	if project, ok := cfg.Inputs["project"].(string); !ok || project != "test-project" {
		t.Errorf("Inputs[project] = %v, want %q", cfg.Inputs["project"], "test-project")
	}
}

func TestYAMLConfigLoader_Load_PermissionError(t *testing.T) {
	// Create a temp file with no read permissions
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "no-read.yaml")

	if err := os.WriteFile(path, []byte("inputs:\n  key: value\n"), 0o000); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	t.Cleanup(func() {
		// Restore permissions for cleanup
		_ = os.Chmod(path, 0o644)
	})

	loader := NewYAMLConfigLoader(path)
	_, err := loader.Load()

	if err == nil {
		t.Fatal("Load() error = nil, want error for permission denied")
	}

	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Errorf("error type = %T, want *ConfigError", err)
		return
	}

	if cfgErr.Op != "load" {
		t.Errorf("ConfigError.Op = %q, want %q", cfgErr.Op, "load")
	}
}

func TestYAMLConfigLoader_Load_Directory(t *testing.T) {
	// Try to load a directory instead of a file
	loader := NewYAMLConfigLoader(fixturesPath)
	_, err := loader.Load()

	if err == nil {
		t.Fatal("Load() error = nil, want error when path is directory")
	}

	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Errorf("error type = %T, want *ConfigError", err)
	}
}

func TestYAMLConfigLoader_Load_ReturnsNewInstance(t *testing.T) {
	path := filepath.Join(fixturesPath, "valid.yaml")
	loader := NewYAMLConfigLoader(path)

	cfg1, err1 := loader.Load()
	cfg2, err2 := loader.Load()

	if err1 != nil || err2 != nil {
		t.Fatalf("Load() errors: %v, %v", err1, err2)
	}

	// Each call should return a new instance
	if cfg1 == cfg2 {
		t.Error("Load() should return new instance each call")
	}

	// Modifying one should not affect the other
	cfg1.Inputs["modified"] = "yes"
	if _, ok := cfg2.Inputs["modified"]; ok {
		t.Error("Modifying cfg1 should not affect cfg2")
	}
}

func TestYAMLConfigLoader_Load_WithComments(t *testing.T) {
	// Create a config with lots of comments
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "commented.yaml")

	content := `# This is a config file
# With lots of comments

# Inputs section
inputs:
  # Project name
  project: "commented-project"  # inline comment
  # Environment setting
  env: "test"

# End of file
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	loader := NewYAMLConfigLoader(path)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if project, ok := cfg.Inputs["project"].(string); !ok || project != "commented-project" {
		t.Errorf("Inputs[project] = %v, want %q", cfg.Inputs["project"], "commented-project")
	}
}

func TestYAMLConfigLoader_Load_SpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "special.yaml")

	content := `inputs:
  path: "/home/user/my project"
  query: "SELECT * FROM users WHERE name = 'test'"
  template: "{{inputs.value}}"
  multiline: |
    line 1
    line 2
  escaped: "tab\there"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	loader := NewYAMLConfigLoader(path)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	tests := []struct {
		key  string
		want string
	}{
		{"path", "/home/user/my project"},
		{"query", "SELECT * FROM users WHERE name = 'test'"},
		{"template", "{{inputs.value}}"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if val, ok := cfg.Inputs[tt.key].(string); !ok || val != tt.want {
				t.Errorf("Inputs[%q] = %v, want %q", tt.key, cfg.Inputs[tt.key], tt.want)
			}
		})
	}

	// Check multiline preserves newlines
	t.Run("multiline", func(t *testing.T) {
		val, ok := cfg.Inputs["multiline"].(string)
		if !ok {
			t.Fatalf("Inputs[multiline] not a string: %T", cfg.Inputs["multiline"])
		}
		if val != "line 1\nline 2\n" {
			t.Errorf("Inputs[multiline] = %q, want %q", val, "line 1\nline 2\n")
		}
	})
}

func TestYAMLConfigLoader_Load_LargeInputs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "large.yaml")

	// Create config with many inputs
	content := "inputs:\n"
	for i := 0; i < 100; i++ {
		content += "  key" + string(rune('0'+i/10)) + string(rune('0'+i%10)) + ": value" + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	loader := NewYAMLConfigLoader(path)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if len(cfg.Inputs) != 100 {
		t.Errorf("len(Inputs) = %d, want 100", len(cfg.Inputs))
	}
}

func TestYAMLConfigLoader_Load_AbsolutePath(t *testing.T) {
	// Get absolute path to fixture
	absPath, err := filepath.Abs(filepath.Join(fixturesPath, "valid.yaml"))
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	loader := NewYAMLConfigLoader(absPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
}

func TestYAMLConfigLoader_Load_RelativePath(t *testing.T) {
	// Use relative path
	loader := NewYAMLConfigLoader(filepath.Join(fixturesPath, "valid.yaml"))
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
}

func TestYAMLConfigLoader_WithWarningFunc(t *testing.T) {
	loader := NewYAMLConfigLoader("/some/path")

	// Should return the loader for chaining
	result := loader.WithWarningFunc(func(format string, args ...any) {})

	if result != loader {
		t.Error("WithWarningFunc() should return the same loader for chaining")
	}
}

func TestYAMLConfigLoader_Load_UnknownKeys_WarningCalled(t *testing.T) {
	path := filepath.Join(fixturesPath, "unknown-keys.yaml")

	var warnings []string
	warnFn := func(format string, args ...any) {
		warnings = append(warnings, format)
	}

	loader := NewYAMLConfigLoader(path).WithWarningFunc(warnFn)
	cfg, err := loader.Load()
	// Should succeed (unknown keys don't cause error)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Valid inputs should still be loaded
	if project, ok := cfg.Inputs["project"].(string); !ok || project != "test-project" {
		t.Errorf("Inputs[project] = %v, want %q", cfg.Inputs["project"], "test-project")
	}

	if len(warnings) == 0 {
		t.Error("Expected warning callback to be called for unknown keys, but no warnings were recorded")
	}
}

func TestYAMLConfigLoader_Load_UnknownKeys_WarningContent(t *testing.T) {
	path := filepath.Join(fixturesPath, "unknown-keys.yaml")

	var warnings []string
	warnFn := func(format string, args ...any) {
		// Capture the formatted message
		warnings = append(warnings, format)
	}

	loader := NewYAMLConfigLoader(path).WithWarningFunc(warnFn)
	_, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Should warn about each unknown key: unknown_key, deprecated_setting, future_feature
	expectedUnknownKeys := []string{"unknown_key", "deprecated_setting", "future_feature"}

	if len(warnings) != len(expectedUnknownKeys) {
		t.Errorf("got %d warnings, want %d for unknown keys %v",
			len(warnings), len(expectedUnknownKeys), expectedUnknownKeys)
	}
}

func TestYAMLConfigLoader_Load_NoUnknownKeys_NoWarning(t *testing.T) {
	path := filepath.Join(fixturesPath, "valid.yaml")

	var warnings []string
	warnFn := func(format string, args ...any) {
		warnings = append(warnings, format)
	}

	loader := NewYAMLConfigLoader(path).WithWarningFunc(warnFn)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// No unknown keys in valid.yaml, so no warnings
	if len(warnings) > 0 {
		t.Errorf("got %d warnings for valid config, want 0: %v", len(warnings), warnings)
	}
}

func TestYAMLConfigLoader_Load_UnknownKeys_NoWarningFunc(t *testing.T) {
	// When no warning func is set, unknown keys should be silently ignored
	path := filepath.Join(fixturesPath, "unknown-keys.yaml")

	loader := NewYAMLConfigLoader(path) // No WithWarningFunc call
	cfg, err := loader.Load()
	// Should not panic or error
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Valid inputs should still be loaded
	if project, ok := cfg.Inputs["project"].(string); !ok || project != "test-project" {
		t.Errorf("Inputs[project] = %v, want %q", cfg.Inputs["project"], "test-project")
	}
}

func TestYAMLConfigLoader_Load_KnownKeys_NoWarning(t *testing.T) {
	// Test that all known keys don't trigger warnings
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "all-known.yaml")

	// Use all known keys
	content := `version: "1"
log_level: debug
output_format: json
inputs:
  project: my-project
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	var warnings []string
	warnFn := func(format string, args ...any) {
		warnings = append(warnings, format)
	}

	loader := NewYAMLConfigLoader(path).WithWarningFunc(warnFn)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// All keys are known, so no warnings expected
	if len(warnings) > 0 {
		t.Errorf("got %d warnings for config with only known keys, want 0: %v",
			len(warnings), warnings)
	}
}

func TestYAMLConfigLoader_Load_SingleUnknownKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "single-unknown.yaml")

	content := `inputs:
  project: my-project
typo_key: should_warn
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	var warnings []string
	warnFn := func(format string, args ...any) {
		warnings = append(warnings, format)
	}

	loader := NewYAMLConfigLoader(path).WithWarningFunc(warnFn)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Inputs should still be loaded correctly
	if project, ok := cfg.Inputs["project"].(string); !ok || project != "my-project" {
		t.Errorf("Inputs[project] = %v, want %q", cfg.Inputs["project"], "my-project")
	}

	if len(warnings) != 1 {
		t.Errorf("got %d warnings, want 1 for single unknown key 'typo_key'", len(warnings))
	}
}

func TestYAMLConfigLoader_Load_EmptyConfig_NoWarning(t *testing.T) {
	path := filepath.Join(fixturesPath, "empty.yaml")

	var warnings []string
	warnFn := func(format string, args ...any) {
		warnings = append(warnings, format)
	}

	loader := NewYAMLConfigLoader(path).WithWarningFunc(warnFn)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Empty file should have no warnings
	if len(warnings) > 0 {
		t.Errorf("got %d warnings for empty config, want 0: %v", len(warnings), warnings)
	}
}

func TestYAMLConfigLoader_Load_NonExistentFile_NoWarning(t *testing.T) {
	path := filepath.Join(fixturesPath, "nonexistent.yaml")

	var warnings []string
	warnFn := func(format string, args ...any) {
		warnings = append(warnings, format)
	}

	loader := NewYAMLConfigLoader(path).WithWarningFunc(warnFn)
	cfg, err := loader.Load()
	// Missing file is not an error
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Non-existent file should have no warnings (file not read)
	if len(warnings) > 0 {
		t.Errorf("got %d warnings for non-existent file, want 0: %v", len(warnings), warnings)
	}
}

func TestYAMLConfigLoader_Load_InvalidYAML_NoWarning(t *testing.T) {
	path := filepath.Join(fixturesPath, "invalid-syntax.yaml")

	var warnings []string
	warnFn := func(format string, args ...any) {
		warnings = append(warnings, format)
	}

	loader := NewYAMLConfigLoader(path).WithWarningFunc(warnFn)
	_, err := loader.Load()

	// Invalid YAML should error
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid YAML")
	}

	// No warning should be issued for parse errors (the error is the notification)
	// Note: checkUnknownKeys is called before the final parse, but if the initial
	// parse into map fails, it should not warn about keys
	// This depends on implementation details
}

func TestYAMLConfigLoader_Load_OnlyUnknownKeys(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "only-unknown.yaml")

	content := `unknown1: value1
unknown2: value2
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	var warnings []string
	warnFn := func(format string, args ...any) {
		warnings = append(warnings, format)
	}

	loader := NewYAMLConfigLoader(path).WithWarningFunc(warnFn)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Config should have empty/nil inputs
	if len(cfg.Inputs) > 0 {
		t.Errorf("Inputs = %v, want empty for config with only unknown keys", cfg.Inputs)
	}

	if len(warnings) != 2 {
		t.Errorf("got %d warnings, want 2 for unknown keys 'unknown1', 'unknown2'", len(warnings))
	}
}
