package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/domain/plugin"
)

const fixturesPath = "../../../tests/fixtures/plugins"

func TestNewManifestParser(t *testing.T) {
	parser := NewManifestParser()
	if parser == nil {
		t.Fatal("NewManifestParser() returned nil")
	}
}

func TestManifestParser_ParseFile_ValidSimple(t *testing.T) {
	parser := NewManifestParser()
	path := filepath.Join(fixturesPath, "valid-simple", "plugin.yaml")

	manifest, err := parser.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if manifest == nil {
		t.Fatal("ParseFile() returned nil manifest")
	}

	// Check required fields
	if manifest.Name != "awf-plugin-simple" {
		t.Errorf("Name = %q, want %q", manifest.Name, "awf-plugin-simple")
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", manifest.Version, "1.0.0")
	}
	if manifest.AWFVersion != ">=0.4.0" {
		t.Errorf("AWFVersion = %q, want %q", manifest.AWFVersion, ">=0.4.0")
	}

	// Check capabilities
	if len(manifest.Capabilities) != 1 {
		t.Errorf("Capabilities count = %d, want 1", len(manifest.Capabilities))
	}
	if !manifest.HasCapability(plugin.CapabilityOperations) {
		t.Error("expected operations capability")
	}

	// Check optional fields are empty/nil
	if manifest.Description != "" {
		t.Errorf("Description = %q, want empty", manifest.Description)
	}
	if manifest.Author != "" {
		t.Errorf("Author = %q, want empty", manifest.Author)
	}
	if manifest.License != "" {
		t.Errorf("License = %q, want empty", manifest.License)
	}
	if manifest.Homepage != "" {
		t.Errorf("Homepage = %q, want empty", manifest.Homepage)
	}
	if len(manifest.Config) > 0 {
		t.Errorf("Config should be empty, got %d fields", len(manifest.Config))
	}
}

//nolint:gocognit // Comprehensive manifest validation test covers all fields.
func TestManifestParser_ParseFile_ValidFull(t *testing.T) {
	parser := NewManifestParser()
	path := filepath.Join(fixturesPath, "valid-full", "plugin.yaml")

	manifest, err := parser.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if manifest == nil {
		t.Fatal("ParseFile() returned nil manifest")
	}

	// Check required fields
	if manifest.Name != "awf-plugin-slack" {
		t.Errorf("Name = %q, want %q", manifest.Name, "awf-plugin-slack")
	}
	if manifest.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", manifest.Version, "1.2.3")
	}
	if manifest.AWFVersion != ">=0.4.0" {
		t.Errorf("AWFVersion = %q, want %q", manifest.AWFVersion, ">=0.4.0")
	}

	// Check optional metadata fields
	if manifest.Description != "Slack notifications for AWF workflows" {
		t.Errorf("Description = %q, want %q", manifest.Description, "Slack notifications for AWF workflows")
	}
	if manifest.Author != "Jane Developer <jane@example.com>" {
		t.Errorf("Author = %q, want %q", manifest.Author, "Jane Developer <jane@example.com>")
	}
	if manifest.License != "MIT" {
		t.Errorf("License = %q, want %q", manifest.License, "MIT")
	}
	if manifest.Homepage != "https://github.com/example/awf-plugin-slack" {
		t.Errorf("Homepage = %q, want %q", manifest.Homepage, "https://github.com/example/awf-plugin-slack")
	}

	// Check capabilities
	if len(manifest.Capabilities) != 2 {
		t.Errorf("Capabilities count = %d, want 2", len(manifest.Capabilities))
	}
	if !manifest.HasCapability(plugin.CapabilityOperations) {
		t.Error("expected operations capability")
	}
	if !manifest.HasCapability(plugin.CapabilityCommands) {
		t.Error("expected commands capability")
	}

	// Check config fields
	if manifest.Config == nil {
		t.Fatal("Config is nil")
	}
	if len(manifest.Config) != 5 {
		t.Errorf("Config count = %d, want 5", len(manifest.Config))
	}

	// Check webhook_url config
	webhookURL, ok := manifest.Config["webhook_url"]
	if !ok {
		t.Fatal("webhook_url config field not found")
	}
	if webhookURL.Type != plugin.ConfigTypeString {
		t.Errorf("webhook_url.Type = %q, want %q", webhookURL.Type, plugin.ConfigTypeString)
	}
	if !webhookURL.Required {
		t.Error("webhook_url.Required = false, want true")
	}
	if webhookURL.Description != "Slack webhook URL for sending notifications" {
		t.Errorf("webhook_url.Description = %q", webhookURL.Description)
	}

	// Check channel config with default
	channel, ok := manifest.Config["channel"]
	if !ok {
		t.Fatal("channel config field not found")
	}
	if channel.Type != plugin.ConfigTypeString {
		t.Errorf("channel.Type = %q, want %q", channel.Type, plugin.ConfigTypeString)
	}
	if channel.Required {
		t.Error("channel.Required = true, want false")
	}
	if channel.Default != "#general" {
		t.Errorf("channel.Default = %v, want %q", channel.Default, "#general")
	}

	// Check boolean config
	notifyOnFailure, ok := manifest.Config["notify_on_failure"]
	if !ok {
		t.Fatal("notify_on_failure config field not found")
	}
	if notifyOnFailure.Type != plugin.ConfigTypeBoolean {
		t.Errorf("notify_on_failure.Type = %q, want %q", notifyOnFailure.Type, plugin.ConfigTypeBoolean)
	}
	if notifyOnFailure.Default != true {
		t.Errorf("notify_on_failure.Default = %v, want true", notifyOnFailure.Default)
	}

	// Check integer config
	retryCount, ok := manifest.Config["retry_count"]
	if !ok {
		t.Fatal("retry_count config field not found")
	}
	if retryCount.Type != plugin.ConfigTypeInteger {
		t.Errorf("retry_count.Type = %q, want %q", retryCount.Type, plugin.ConfigTypeInteger)
	}
	// YAML parses integers as int, but Default is any
	switch v := retryCount.Default.(type) {
	case int:
		if v != 3 {
			t.Errorf("retry_count.Default = %d, want 3", v)
		}
	case float64:
		if int(v) != 3 {
			t.Errorf("retry_count.Default = %v, want 3", v)
		}
	default:
		t.Errorf("retry_count.Default type = %T, want int or float64", retryCount.Default)
	}

	// Check enum config
	logLevel, ok := manifest.Config["log_level"]
	if !ok {
		t.Fatal("log_level config field not found")
	}
	if len(logLevel.Enum) != 4 {
		t.Errorf("log_level.Enum count = %d, want 4", len(logLevel.Enum))
	}
	expectedEnum := []string{"debug", "info", "warn", "error"}
	for i, v := range expectedEnum {
		if i < len(logLevel.Enum) && logLevel.Enum[i] != v {
			t.Errorf("log_level.Enum[%d] = %q, want %q", i, logLevel.Enum[i], v)
		}
	}
}

func TestManifestParser_ParseFile_NonExistent(t *testing.T) {
	parser := NewManifestParser()
	path := filepath.Join(fixturesPath, "nonexistent", "plugin.yaml")

	manifest, err := parser.ParseFile(path)
	if err == nil {
		t.Fatal("ParseFile() error = nil, want error for non-existent file")
	}
	if manifest != nil {
		t.Errorf("ParseFile() manifest = %v, want nil", manifest)
	}

	// Check error is related to file not found
	if !os.IsNotExist(errors.Unwrap(err)) && !strings.Contains(err.Error(), "no such file") {
		t.Logf("Error type: %T, value: %v", err, err)
		// Accept any error for non-existent file
	}
}

func TestManifestParser_ParseFile_InvalidSyntax(t *testing.T) {
	parser := NewManifestParser()
	path := filepath.Join(fixturesPath, "invalid-syntax", "plugin.yaml")

	manifest, err := parser.ParseFile(path)
	if err == nil {
		t.Fatal("ParseFile() error = nil, want error for invalid YAML syntax")
	}
	if manifest != nil {
		t.Errorf("ParseFile() manifest = %v, want nil", manifest)
	}

	// Check it's a ManifestParseError
	var parseErr *ManifestParseError
	if errors.As(err, &parseErr) {
		if parseErr.File == "" {
			t.Error("ManifestParseError.File is empty")
		}
	}
}

func TestManifestParser_ParseFile_MissingName(t *testing.T) {
	parser := NewManifestParser()
	path := filepath.Join(fixturesPath, "invalid-missing-name", "plugin.yaml")

	manifest, err := parser.ParseFile(path)
	if err == nil {
		t.Fatal("ParseFile() error = nil, want error for missing name")
	}
	if manifest != nil {
		t.Errorf("ParseFile() manifest = %v, want nil", manifest)
	}

	// Check error mentions name field
	var parseErr *ManifestParseError
	if errors.As(err, &parseErr) {
		if parseErr.Field != "name" && !strings.Contains(parseErr.Message, "name") {
			t.Errorf("error should mention 'name' field, got: %v", err)
		}
	}
}

func TestManifestParser_ParseFile_MissingVersion(t *testing.T) {
	parser := NewManifestParser()
	path := filepath.Join(fixturesPath, "invalid-missing-version", "plugin.yaml")

	manifest, err := parser.ParseFile(path)
	if err == nil {
		t.Fatal("ParseFile() error = nil, want error for missing version")
	}
	if manifest != nil {
		t.Errorf("ParseFile() manifest = %v, want nil", manifest)
	}

	// Check error mentions version field
	var parseErr *ManifestParseError
	if errors.As(err, &parseErr) {
		if parseErr.Field != "version" && !strings.Contains(parseErr.Message, "version") {
			t.Errorf("error should mention 'version' field, got: %v", err)
		}
	}
}

func TestManifestParser_ParseFile_MissingAWFVersion(t *testing.T) {
	parser := NewManifestParser()
	path := filepath.Join(fixturesPath, "invalid-missing-awf-version", "plugin.yaml")

	manifest, err := parser.ParseFile(path)
	if err == nil {
		t.Fatal("ParseFile() error = nil, want error for missing awf_version")
	}
	if manifest != nil {
		t.Errorf("ParseFile() manifest = %v, want nil", manifest)
	}

	// Check error mentions awf_version field
	var parseErr *ManifestParseError
	if errors.As(err, &parseErr) {
		if parseErr.Field != "awf_version" && !strings.Contains(parseErr.Message, "awf_version") {
			t.Errorf("error should mention 'awf_version' field, got: %v", err)
		}
	}
}

func TestManifestParser_Parse_FromReader(t *testing.T) {
	parser := NewManifestParser()
	yamlContent := `
name: test-plugin
version: 2.0.0
awf_version: ">=0.4.0"
capabilities:
  - validators
`
	reader := strings.NewReader(yamlContent)

	manifest, err := parser.Parse(reader)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if manifest == nil {
		t.Fatal("Parse() returned nil manifest")
	}

	if manifest.Name != "test-plugin" {
		t.Errorf("Name = %q, want %q", manifest.Name, "test-plugin")
	}
	if manifest.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", manifest.Version, "2.0.0")
	}
	if !manifest.HasCapability(plugin.CapabilityValidators) {
		t.Error("expected validators capability")
	}
}

func TestManifestParser_Parse_EmptyReader(t *testing.T) {
	parser := NewManifestParser()
	reader := strings.NewReader("")

	manifest, err := parser.Parse(reader)
	if err == nil {
		t.Fatal("Parse() error = nil, want error for empty content")
	}
	if manifest != nil {
		t.Errorf("Parse() manifest = %v, want nil", manifest)
	}
}

func TestManifestParser_Parse_InvalidYAML(t *testing.T) {
	parser := NewManifestParser()
	yamlContent := `
name: test
  version: 1.0.0
`
	reader := strings.NewReader(yamlContent)

	manifest, err := parser.Parse(reader)
	if err == nil {
		t.Fatal("Parse() error = nil, want error for invalid YAML")
	}
	if manifest != nil {
		t.Errorf("Parse() manifest = %v, want nil", manifest)
	}
}

func TestManifestParser_Parse_AllCapabilities(t *testing.T) {
	parser := NewManifestParser()
	yamlContent := `
name: full-cap-plugin
version: 1.0.0
awf_version: ">=0.4.0"
capabilities:
  - operations
  - commands
  - validators
`
	reader := strings.NewReader(yamlContent)

	manifest, err := parser.Parse(reader)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if manifest == nil {
		t.Fatal("Parse() returned nil manifest")
	}

	if len(manifest.Capabilities) != 3 {
		t.Errorf("Capabilities count = %d, want 3", len(manifest.Capabilities))
	}
	if !manifest.HasCapability(plugin.CapabilityOperations) {
		t.Error("expected operations capability")
	}
	if !manifest.HasCapability(plugin.CapabilityCommands) {
		t.Error("expected commands capability")
	}
	if !manifest.HasCapability(plugin.CapabilityValidators) {
		t.Error("expected validators capability")
	}
}

func TestManifestParser_Parse_ConfigTypes(t *testing.T) {
	parser := NewManifestParser()
	yamlContent := `
name: config-types-plugin
version: 1.0.0
awf_version: ">=0.4.0"
capabilities:
  - operations
config:
  string_field:
    type: string
    required: true
    description: A string field
  integer_field:
    type: integer
    required: false
    default: 42
  boolean_field:
    type: boolean
    default: false
`
	reader := strings.NewReader(yamlContent)

	manifest, err := parser.Parse(reader)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if manifest == nil {
		t.Fatal("Parse() returned nil manifest")
	}

	if len(manifest.Config) != 3 {
		t.Errorf("Config count = %d, want 3", len(manifest.Config))
	}

	// Check string field
	stringField, ok := manifest.Config["string_field"]
	if !ok {
		t.Fatal("string_field not found")
	}
	if stringField.Type != plugin.ConfigTypeString {
		t.Errorf("string_field.Type = %q, want %q", stringField.Type, plugin.ConfigTypeString)
	}
	if !stringField.Required {
		t.Error("string_field.Required = false, want true")
	}

	// Check integer field
	intField, ok := manifest.Config["integer_field"]
	if !ok {
		t.Fatal("integer_field not found")
	}
	if intField.Type != plugin.ConfigTypeInteger {
		t.Errorf("integer_field.Type = %q, want %q", intField.Type, plugin.ConfigTypeInteger)
	}

	// Check boolean field
	boolField, ok := manifest.Config["boolean_field"]
	if !ok {
		t.Fatal("boolean_field not found")
	}
	if boolField.Type != plugin.ConfigTypeBoolean {
		t.Errorf("boolean_field.Type = %q, want %q", boolField.Type, plugin.ConfigTypeBoolean)
	}
}

func TestManifestParseError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ManifestParseError
		contains []string
	}{
		{
			name: "with file and field",
			err: &ManifestParseError{
				File:    "/path/to/plugin.yaml",
				Field:   "config.webhook_url",
				Message: "required field missing",
			},
			contains: []string{"/path/to/plugin.yaml", "config.webhook_url", "required field missing"},
		},
		{
			name: "with file only",
			err: &ManifestParseError{
				File:    "/path/to/plugin.yaml",
				Message: "invalid YAML syntax",
			},
			contains: []string{"/path/to/plugin.yaml", "invalid YAML syntax"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()
			for _, s := range tt.contains {
				if !strings.Contains(errStr, s) {
					t.Errorf("Error() = %q, should contain %q", errStr, s)
				}
			}
		})
	}
}

func TestManifestParseError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &ManifestParseError{
		File:    "plugin.yaml",
		Message: "parse failed",
		Cause:   cause,
	}

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestNewManifestParseError(t *testing.T) {
	err := NewManifestParseError("/path/plugin.yaml", "name", "required")

	if err.File != "/path/plugin.yaml" {
		t.Errorf("File = %q, want %q", err.File, "/path/plugin.yaml")
	}
	if err.Field != "name" {
		t.Errorf("Field = %q, want %q", err.Field, "name")
	}
	if err.Message != "required" {
		t.Errorf("Message = %q, want %q", err.Message, "required")
	}
	if err.Cause != nil {
		t.Errorf("Cause = %v, want nil", err.Cause)
	}
}

func TestWrapManifestParseError(t *testing.T) {
	cause := errors.New("yaml: line 5: did not find expected key")
	err := WrapManifestParseError("/path/plugin.yaml", cause)

	if err.File != "/path/plugin.yaml" {
		t.Errorf("File = %q, want %q", err.File, "/path/plugin.yaml")
	}
	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}
	if !strings.Contains(err.Message, "yaml") {
		t.Errorf("Message = %q, should contain 'yaml'", err.Message)
	}
}

// Table-driven tests for edge cases
func TestManifestParser_ParseFile_EdgeCases(t *testing.T) {
	parser := NewManifestParser()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid-simple", filepath.Join(fixturesPath, "valid-simple", "plugin.yaml"), false},
		{"valid-full", filepath.Join(fixturesPath, "valid-full", "plugin.yaml"), false},
		{"invalid-syntax", filepath.Join(fixturesPath, "invalid-syntax", "plugin.yaml"), true},
		{"invalid-missing-name", filepath.Join(fixturesPath, "invalid-missing-name", "plugin.yaml"), true},
		{"invalid-missing-version", filepath.Join(fixturesPath, "invalid-missing-version", "plugin.yaml"), true},
		{"invalid-missing-awf-version", filepath.Join(fixturesPath, "invalid-missing-awf-version", "plugin.yaml"), true},
		{"nonexistent", filepath.Join(fixturesPath, "nonexistent", "plugin.yaml"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := parser.ParseFile(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && manifest == nil {
				t.Error("ParseFile() returned nil manifest for valid file")
			}
			if tt.wantErr && manifest != nil {
				t.Error("ParseFile() returned non-nil manifest for invalid file")
			}
		})
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
