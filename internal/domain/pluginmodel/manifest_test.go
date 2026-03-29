package pluginmodel_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidCapabilities_ContainsExpectedValues(t *testing.T) {
	assert.Contains(t, pluginmodel.ValidCapabilities, pluginmodel.CapabilityOperations)
	assert.Contains(t, pluginmodel.ValidCapabilities, pluginmodel.CapabilityStepTypes)
	assert.Contains(t, pluginmodel.ValidCapabilities, pluginmodel.CapabilityValidators)
	assert.Len(t, pluginmodel.ValidCapabilities, 3)
}

func TestCapabilityConstants_Values(t *testing.T) {
	assert.Equal(t, "operations", pluginmodel.CapabilityOperations)
	assert.Equal(t, "step_types", pluginmodel.CapabilityStepTypes)
	assert.Equal(t, "validators", pluginmodel.CapabilityValidators)
}

func TestValidConfigTypes_ContainsExpectedValues(t *testing.T) {
	assert.Contains(t, pluginmodel.ValidConfigTypes, pluginmodel.ConfigTypeString)
	assert.Contains(t, pluginmodel.ValidConfigTypes, pluginmodel.ConfigTypeInteger)
	assert.Contains(t, pluginmodel.ValidConfigTypes, pluginmodel.ConfigTypeBoolean)
	assert.Len(t, pluginmodel.ValidConfigTypes, 3)
}

func TestConfigTypeConstants_Values(t *testing.T) {
	assert.Equal(t, "string", pluginmodel.ConfigTypeString)
	assert.Equal(t, "integer", pluginmodel.ConfigTypeInteger)
	assert.Equal(t, "boolean", pluginmodel.ConfigTypeBoolean)
}

// NamePattern Regex Tests (Component T001)
// Feature: C031

func TestNamePattern_ValidNames(t *testing.T) {
	// Component: T001
	// Feature: C031
	tests := []struct {
		name  string
		input string
	}{
		{name: "lowercase single letter", input: "a"},
		{name: "lowercase word", input: "plugin"},
		{name: "lowercase with hyphen", input: "my-plugin"},
		{name: "multiple hyphens", input: "my-awesome-plugin"},
		{name: "numbers after letter", input: "plugin123"},
		{name: "mixed alphanumeric with hyphens", input: "my-plugin-v2"},
		{name: "trailing numbers", input: "test1"},
		{name: "complex pattern", input: "awf-marketplace-v2-beta"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       tt.input,
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			}
			err := m.Validate()
			assert.NoError(t, err, "expected %q to be valid", tt.input)
		})
	}
}

func TestNamePattern_InvalidNames(t *testing.T) {
	// Component: T001
	// Feature: C031
	tests := []struct {
		name  string
		input string
	}{
		{name: "starts with digit", input: "1plugin"},
		{name: "contains uppercase", input: "MyPlugin"},
		{name: "contains underscore", input: "my_plugin"},
		{name: "contains space", input: "my plugin"},
		{name: "starts with hyphen", input: "-plugin"},
		{name: "contains special chars", input: "my@plugin"},
		{name: "contains dot", input: "my.plugin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       tt.input,
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			}
			err := m.Validate()
			assert.Error(t, err, "expected %q to be invalid", tt.input)
			assert.Contains(t, err.Error(), "name")
		})
	}
}

func TestNamePattern_EdgeCases(t *testing.T) {
	// Component: T001
	// Feature: C031
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty string", input: "", wantErr: true},
		{name: "whitespace only", input: "   ", wantErr: true},
		{name: "hyphen only", input: "-", wantErr: true},
		{name: "number only", input: "123", wantErr: true},
		{name: "valid single char", input: "a", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       tt.input,
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			}
			err := m.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "name")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNamePattern_RegexCompilation(t *testing.T) {
	// Component: T001
	// Feature: C031
	// Test that the regex pattern compiles successfully
	// This is implicitly tested by other tests, but explicit verification is good practice
	m := pluginmodel.Manifest{
		Name:       "valid-plugin",
		Version:    "1.0.0",
		AWFVersion: ">=0.4.0",
	}
	err := m.Validate()
	assert.NoError(t, err)
}

func TestManifest_Creation(t *testing.T) {
	m := pluginmodel.Manifest{
		Name:       "test-plugin",
		Version:    "1.0.0",
		AWFVersion: ">=0.4.0",
	}
	assert.Equal(t, "test-plugin", m.Name)
	assert.Equal(t, "1.0.0", m.Version)
	assert.Equal(t, ">=0.4.0", m.AWFVersion)
}

func TestManifest_FullManifest(t *testing.T) {
	m := pluginmodel.Manifest{
		Name:        "marketplace-plugin",
		Version:     "1.2.3",
		AWFVersion:  ">=0.4.0",
		Description: "A marketplace plugin",
		Author:      "AWF Team",
		License:     "MIT",
		Homepage:    "https://github.com/awf-project/cli",
		Capabilities: []string{
			pluginmodel.CapabilityOperations,
			pluginmodel.CapabilityStepTypes,
		},
		Config: map[string]pluginmodel.ConfigField{
			"api_key": {
				Type:        pluginmodel.ConfigTypeString,
				Required:    true,
				Description: "API key for authentication",
			},
		},
	}

	assert.Equal(t, "marketplace-plugin", m.Name)
	assert.Equal(t, "1.2.3", m.Version)
	assert.Equal(t, ">=0.4.0", m.AWFVersion)
	assert.Len(t, m.Capabilities, 2)
	assert.Len(t, m.Config, 1)
}

func TestManifest_EmptyConfig(t *testing.T) {
	m := pluginmodel.Manifest{
		Name:       "test-plugin",
		Version:    "1.0.0",
		AWFVersion: ">=0.4.0",
		Config:     map[string]pluginmodel.ConfigField{},
	}
	assert.NotNil(t, m.Config)
	assert.Len(t, m.Config, 0)
}

func TestManifest_MultipleCapabilities(t *testing.T) {
	m := pluginmodel.Manifest{
		Name:       "test-plugin",
		Version:    "1.0.0",
		AWFVersion: ">=0.4.0",
		Capabilities: []string{
			pluginmodel.CapabilityOperations,
			pluginmodel.CapabilityStepTypes,
			pluginmodel.CapabilityValidators,
		},
	}
	assert.Len(t, m.Capabilities, 3)
	assert.Contains(t, m.Capabilities, pluginmodel.CapabilityOperations)
	assert.Contains(t, m.Capabilities, pluginmodel.CapabilityStepTypes)
	assert.Contains(t, m.Capabilities, pluginmodel.CapabilityValidators)
}

func TestManifest_OptionalMetadata(t *testing.T) {
	m := pluginmodel.Manifest{
		Name:        "test-plugin",
		Version:     "1.0.0",
		AWFVersion:  ">=0.4.0",
		Description: "Test plugin description",
		Author:      "Test Author",
		License:     "MIT",
		Homepage:    "https://example.com",
	}
	assert.Equal(t, "Test plugin description", m.Description)
	assert.Equal(t, "Test Author", m.Author)
	assert.Equal(t, "MIT", m.License)
	assert.Equal(t, "https://example.com", m.Homepage)
}

func TestManifest_HasCapability(t *testing.T) {
	m := pluginmodel.Manifest{
		Name:       "test-plugin",
		Version:    "1.0.0",
		AWFVersion: ">=0.4.0",
		Capabilities: []string{
			pluginmodel.CapabilityOperations,
		},
	}

	assert.True(t, m.HasCapability(pluginmodel.CapabilityOperations))
	assert.False(t, m.HasCapability(pluginmodel.CapabilityStepTypes))
	assert.False(t, m.HasCapability(pluginmodel.CapabilityValidators))
}

func TestConfigField_Creation(t *testing.T) {
	cf := pluginmodel.ConfigField{
		Type:        pluginmodel.ConfigTypeString,
		Required:    true,
		Description: "API key",
	}
	assert.Equal(t, pluginmodel.ConfigTypeString, cf.Type)
	assert.True(t, cf.Required)
	assert.Equal(t, "API key", cf.Description)
}

func TestConfigField_DefaultValues(t *testing.T) {
	cf := pluginmodel.ConfigField{
		Type:    pluginmodel.ConfigTypeString,
		Default: "default-value",
	}
	assert.Equal(t, "default-value", cf.Default)
}

func TestConfigField_WithEnum(t *testing.T) {
	cf := pluginmodel.ConfigField{
		Type: pluginmodel.ConfigTypeString,
		Enum: []string{"option1", "option2", "option3"},
	}
	assert.Len(t, cf.Enum, 3)
	assert.Contains(t, cf.Enum, "option1")
	assert.Contains(t, cf.Enum, "option2")
	assert.Contains(t, cf.Enum, "option3")
}

func TestConfigField_FullField(t *testing.T) {
	cf := pluginmodel.ConfigField{
		Type:        pluginmodel.ConfigTypeString,
		Required:    true,
		Default:     "default",
		Description: "Configuration field",
		Enum:        []string{"option1", "option2"},
	}
	assert.Equal(t, pluginmodel.ConfigTypeString, cf.Type)
	assert.True(t, cf.Required)
	assert.Equal(t, "default", cf.Default)
	assert.Equal(t, "Configuration field", cf.Description)
	assert.Len(t, cf.Enum, 2)
}

func TestManifest_NameFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{name: "simple lowercase", format: "plugin"},
		{name: "hyphenated", format: "my-plugin"},
		{name: "with numbers", format: "plugin123"},
		{name: "complex", format: "awf-marketplace-v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       tt.format,
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			}
			assert.Equal(t, tt.format, m.Name)
		})
	}
}

func TestManifest_VersionFormats(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{name: "semantic version", version: "1.0.0"},
		{name: "with prerelease", version: "1.0.0-alpha.1"},
		{name: "with build metadata", version: "1.0.0+20230101"},
		{name: "full semver", version: "1.0.0-beta.2+build.123"},
		{name: "major only", version: "1"},
		{name: "major.minor", version: "1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    tt.version,
				AWFVersion: ">=0.4.0",
			}
			assert.Equal(t, tt.version, m.Version)
		})
	}
}

func TestManifest_AWFVersionConstraints(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
	}{
		{name: "greater than or equal", constraint: ">=0.4.0"},
		{name: "caret range", constraint: "^0.4.0"},
		{name: "tilde range", constraint: "~0.4.0"},
		{name: "range", constraint: ">=0.4.0 <1.0.0"},
		{name: "exact version", constraint: "0.4.0"},
		{name: "greater than", constraint: ">0.3.0"},
		{name: "less than", constraint: "<2.0.0"},
		{name: "wildcard minor", constraint: "0.4.x"},
		{name: "or constraint", constraint: ">=0.4.0 || >=1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: tt.constraint,
			}
			assert.Equal(t, tt.constraint, m.AWFVersion)
		})
	}
}

func TestManifest_ConfigWithMultipleFields(t *testing.T) {
	m := pluginmodel.Manifest{
		Name:       "test-plugin",
		Version:    "1.0.0",
		AWFVersion: ">=0.4.0",
		Config: map[string]pluginmodel.ConfigField{
			"api_key": {
				Type:        pluginmodel.ConfigTypeString,
				Required:    true,
				Description: "API key",
			},
			"timeout": {
				Type:        pluginmodel.ConfigTypeInteger,
				Default:     30,
				Description: "Timeout in seconds",
			},
			"enabled": {
				Type:        pluginmodel.ConfigTypeBoolean,
				Default:     true,
				Description: "Enable feature",
			},
		},
	}

	assert.Len(t, m.Config, 3)
	assert.Equal(t, pluginmodel.ConfigTypeString, m.Config["api_key"].Type)
	assert.Equal(t, pluginmodel.ConfigTypeInteger, m.Config["timeout"].Type)
	assert.Equal(t, pluginmodel.ConfigTypeBoolean, m.Config["enabled"].Type)
}

func TestManifest_LicenseFormats(t *testing.T) {
	tests := []struct {
		name    string
		license string
	}{
		{name: "MIT", license: "MIT"},
		{name: "Apache 2.0", license: "Apache-2.0"},
		{name: "GPL v3", license: "GPL-3.0"},
		{name: "BSD 3-Clause", license: "BSD-3-Clause"},
		{name: "Proprietary", license: "Proprietary"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				License:    tt.license,
			}
			assert.Equal(t, tt.license, m.License)
		})
	}
}

func TestManifest_HomepageFormats(t *testing.T) {
	tests := []struct {
		name     string
		homepage string
	}{
		{name: "GitHub URL", homepage: "https://github.com/awf-project/cli"},
		{name: "GitLab URL", homepage: "https://gitlab.com/project/repo"},
		{name: "Custom domain", homepage: "https://awf-pluginmodel.example.com"},
		{name: "HTTP URL", homepage: "http://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Homepage:   tt.homepage,
			}
			assert.Equal(t, tt.homepage, m.Homepage)
		})
	}
}

func TestManifest_AuthorFormats(t *testing.T) {
	tests := []struct {
		name   string
		author string
	}{
		{name: "Simple name", author: "John Doe"},
		{name: "Organization", author: "AWF Team"},
		{name: "With email", author: "John Doe <john@example.com>"},
		{name: "With URL", author: "AWF Team (https://github.com/awf-project)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:    "test-plugin",
				Version: "1.0.0",
				Author:  tt.author,
			}
			assert.Equal(t, tt.author, m.Author)
		})
	}
}

// validateConfigField() Tests (Component T002)
// Feature: C031

// TestValidateConfigField_HappyPath tests valid ConfigField configurations
func TestValidateConfigField_HappyPath(t *testing.T) {
	// Component: T002, T008
	// Feature: C031
	tests := []struct {
		name   string
		field  pluginmodel.ConfigField
		config map[string]pluginmodel.ConfigField
	}{
		{
			name: "valid string field",
			config: map[string]pluginmodel.ConfigField{
				"api_key": {
					Type:        pluginmodel.ConfigTypeString,
					Required:    true,
					Description: "API key for authentication",
				},
			},
		},
		{
			name: "valid integer field",
			config: map[string]pluginmodel.ConfigField{
				"timeout": {
					Type:        pluginmodel.ConfigTypeInteger,
					Required:    false,
					Default:     30,
					Description: "Timeout in seconds",
				},
			},
		},
		{
			name: "valid boolean field",
			config: map[string]pluginmodel.ConfigField{
				"enabled": {
					Type:        pluginmodel.ConfigTypeBoolean,
					Required:    false,
					Default:     true,
					Description: "Enable feature",
				},
			},
		},
		{
			name: "string field with enum",
			config: map[string]pluginmodel.ConfigField{
				"log_level": {
					Type:        pluginmodel.ConfigTypeString,
					Required:    false,
					Default:     "info",
					Enum:        []string{"debug", "info", "warn", "error"},
					Description: "Logging level",
				},
			},
		},
		{
			name: "string field with default matching enum",
			config: map[string]pluginmodel.ConfigField{
				"environment": {
					Type:     pluginmodel.ConfigTypeString,
					Default:  "development",
					Enum:     []string{"development", "staging", "production"},
					Required: false,
				},
			},
		},
		{
			name: "field without default value",
			config: map[string]pluginmodel.ConfigField{
				"optional_field": {
					Type:        pluginmodel.ConfigTypeString,
					Required:    false,
					Description: "Optional field",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config:     tt.config,
			}
			err := m.Validate()
			assert.NoError(t, err)
		})
	}
}

// TestValidateConfigField_TypeValidation tests config field type validation
func TestValidateConfigField_TypeValidation(t *testing.T) {
	// Component: T002, T008
	// Feature: C031
	tests := []struct {
		name    string
		field   pluginmodel.ConfigField
		wantErr bool
		errMsg  string
	}{
		{
			name: "empty type",
			field: pluginmodel.ConfigField{
				Type:        "",
				Description: "Field with empty type",
			},
			wantErr: true,
			errMsg:  "type",
		},
		{
			name: "invalid type",
			field: pluginmodel.ConfigField{
				Type:        "array",
				Description: "Unsupported array type",
			},
			wantErr: true,
			errMsg:  "type",
		},
		{
			name: "unknown type",
			field: pluginmodel.ConfigField{
				Type:        "object",
				Description: "Unsupported object type",
			},
			wantErr: true,
			errMsg:  "type",
		},
		{
			name: "valid string type",
			field: pluginmodel.ConfigField{
				Type:        pluginmodel.ConfigTypeString,
				Description: "Valid string field",
			},
			wantErr: false,
		},
		{
			name: "valid integer type",
			field: pluginmodel.ConfigField{
				Type:        pluginmodel.ConfigTypeInteger,
				Description: "Valid integer field",
			},
			wantErr: false,
		},
		{
			name: "valid boolean type",
			field: pluginmodel.ConfigField{
				Type:        pluginmodel.ConfigTypeBoolean,
				Description: "Valid boolean field",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"test_field": tt.field,
				},
			}
			err := m.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateConfigField_EnumValidation tests enum constraint validation
func TestValidateConfigField_EnumValidation(t *testing.T) {
	// Component: T002, T008
	// Feature: C031
	tests := []struct {
		name    string
		field   pluginmodel.ConfigField
		wantErr bool
		errMsg  string
	}{
		{
			name: "enum on string type - valid",
			field: pluginmodel.ConfigField{
				Type: pluginmodel.ConfigTypeString,
				Enum: []string{"option1", "option2", "option3"},
			},
			wantErr: false,
		},
		{
			name: "enum on integer type - invalid",
			field: pluginmodel.ConfigField{
				Type: pluginmodel.ConfigTypeInteger,
				Enum: []string{"1", "2", "3"},
			},
			wantErr: true,
			errMsg:  "enum",
		},
		{
			name: "enum on boolean type - invalid",
			field: pluginmodel.ConfigField{
				Type: pluginmodel.ConfigTypeBoolean,
				Enum: []string{"true", "false"},
			},
			wantErr: true,
			errMsg:  "enum",
		},
		{
			name: "empty enum on string type",
			field: pluginmodel.ConfigField{
				Type: pluginmodel.ConfigTypeString,
				Enum: []string{},
			},
			wantErr: false,
		},
		{
			name: "nil enum on string type",
			field: pluginmodel.ConfigField{
				Type: pluginmodel.ConfigTypeString,
				Enum: nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"test_field": tt.field,
				},
			}
			err := m.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateConfigField_DefaultTypeValidation tests default value type matching
func TestValidateConfigField_DefaultTypeValidation(t *testing.T) {
	// Component: T002, T008
	// Feature: C031
	tests := []struct {
		name    string
		field   pluginmodel.ConfigField
		wantErr bool
		errMsg  string
	}{
		{
			name: "string default matches string type",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeString,
				Default: "default-value",
			},
			wantErr: false,
		},
		{
			name: "integer default matches integer type - int",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeInteger,
				Default: 42,
			},
			wantErr: false,
		},
		{
			name: "integer default matches integer type - float64 (JSON)",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeInteger,
				Default: float64(42),
			},
			wantErr: false,
		},
		{
			name: "boolean default matches boolean type",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeBoolean,
				Default: true,
			},
			wantErr: false,
		},
		{
			name: "string default mismatch with integer type",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeInteger,
				Default: "not-a-number",
			},
			wantErr: true,
			errMsg:  "type mismatch",
		},
		{
			name: "integer default mismatch with string type",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeString,
				Default: 123,
			},
			wantErr: true,
			errMsg:  "type mismatch",
		},
		{
			name: "boolean default mismatch with string type",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeString,
				Default: true,
			},
			wantErr: true,
			errMsg:  "type mismatch",
		},
		{
			name: "string default mismatch with boolean type",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeBoolean,
				Default: "true",
			},
			wantErr: true,
			errMsg:  "type mismatch",
		},
		{
			name: "nil default value - valid",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeString,
				Default: nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"test_field": tt.field,
				},
			}
			err := m.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateConfigField_EdgeCases tests edge cases in config field validation
func TestValidateConfigField_EdgeCases(t *testing.T) {
	// Component: T002, T008
	// Feature: C031
	tests := []struct {
		name    string
		config  map[string]pluginmodel.ConfigField
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config map",
			config:  nil,
			wantErr: false,
		},
		{
			name:    "empty config map",
			config:  map[string]pluginmodel.ConfigField{},
			wantErr: false,
		},
		{
			name: "multiple valid fields",
			config: map[string]pluginmodel.ConfigField{
				"field1": {Type: pluginmodel.ConfigTypeString, Default: "value1"},
				"field2": {Type: pluginmodel.ConfigTypeInteger, Default: 42},
				"field3": {Type: pluginmodel.ConfigTypeBoolean, Default: true},
			},
			wantErr: false,
		},
		{
			name: "first field valid, second field invalid",
			config: map[string]pluginmodel.ConfigField{
				"valid_field":   {Type: pluginmodel.ConfigTypeString, Default: "value"},
				"invalid_field": {Type: "invalid_type", Default: "value"},
			},
			wantErr: true,
			errMsg:  "type",
		},
		{
			name: "field with all attributes",
			config: map[string]pluginmodel.ConfigField{
				"comprehensive": {
					Type:        pluginmodel.ConfigTypeString,
					Required:    true,
					Default:     "default",
					Description: "Comprehensive field test",
					Enum:        []string{"option1", "option2"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config:     tt.config,
			}
			err := m.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateConfigField_ErrorMessages tests error message clarity
func TestValidateConfigField_ErrorMessages(t *testing.T) {
	// Component: T002, T008
	// Feature: C031
	tests := []struct {
		name        string
		config      map[string]pluginmodel.ConfigField
		expectedErr []string // multiple strings that should appear in error
	}{
		{
			name: "invalid type error includes field name",
			config: map[string]pluginmodel.ConfigField{
				"bad_field": {Type: "invalid"},
			},
			expectedErr: []string{"bad_field"},
		},
		{
			name: "enum on non-string includes field name and constraint",
			config: map[string]pluginmodel.ConfigField{
				"numeric_enum": {
					Type: pluginmodel.ConfigTypeInteger,
					Enum: []string{"1", "2", "3"},
				},
			},
			expectedErr: []string{"numeric_enum", "enum"},
		},
		{
			name: "type mismatch includes expected and actual types",
			config: map[string]pluginmodel.ConfigField{
				"wrong_default": {
					Type:    pluginmodel.ConfigTypeInteger,
					Default: "not-a-number",
				},
			},
			expectedErr: []string{"wrong_default", "type mismatch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config:     tt.config,
			}
			err := m.Validate()
			require.Error(t, err)
			for _, expected := range tt.expectedErr {
				assert.Contains(t, err.Error(), expected,
					"error message should contain %q", expected)
			}
		})
	}
}

// TestValidateConfigField_ComplexScenarios tests realistic complex configurations
func TestValidateConfigField_ComplexScenarios(t *testing.T) {
	// Component: T002, T008
	// Feature: C031
	tests := []struct {
		name    string
		config  map[string]pluginmodel.ConfigField
		wantErr bool
	}{
		{
			name: "realistic marketplace plugin config",
			config: map[string]pluginmodel.ConfigField{
				"api_key": {
					Type:        pluginmodel.ConfigTypeString,
					Required:    true,
					Description: "Marketplace API key",
				},
				"timeout": {
					Type:        pluginmodel.ConfigTypeInteger,
					Default:     30,
					Description: "Request timeout in seconds",
				},
				"environment": {
					Type:        pluginmodel.ConfigTypeString,
					Default:     "production",
					Enum:        []string{"development", "staging", "production"},
					Description: "Deployment environment",
				},
				"cache_enabled": {
					Type:        pluginmodel.ConfigTypeBoolean,
					Default:     true,
					Description: "Enable response caching",
				},
				"log_level": {
					Type:        pluginmodel.ConfigTypeString,
					Default:     "info",
					Enum:        []string{"debug", "info", "warn", "error"},
					Description: "Logging verbosity level",
				},
			},
			wantErr: false,
		},
		{
			name: "config with one invalid field among many valid",
			config: map[string]pluginmodel.ConfigField{
				"valid1":  {Type: pluginmodel.ConfigTypeString, Default: "value"},
				"valid2":  {Type: pluginmodel.ConfigTypeInteger, Default: 42},
				"invalid": {Type: "bad_type", Default: "value"},
				"valid3":  {Type: pluginmodel.ConfigTypeBoolean, Default: true},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config:     tt.config,
			}
			err := m.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// validateDefaultType() Helper Tests (Component T003)

// TestValidateDefaultType_StringType tests string type validation via validateDefaultType
func TestValidateDefaultType_StringType(t *testing.T) {
	// Component: T003, T008
	// Feature: C031
	tests := []struct {
		name         string
		defaultValue any
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "valid string default",
			defaultValue: "hello",
			wantErr:      false,
		},
		{
			name:         "empty string is valid",
			defaultValue: "",
			wantErr:      false,
		},
		{
			name:         "multiline string is valid",
			defaultValue: "line1\nline2\nline3",
			wantErr:      false,
		},
		{
			name:         "string with special characters",
			defaultValue: "test@#$%^&*()_+-={}[]|\\:;\"'<>,.?/",
			wantErr:      false,
		},
		{
			name:         "unicode string is valid",
			defaultValue: "こんにちは世界",
			wantErr:      false,
		},
		{
			name:         "integer instead of string",
			defaultValue: 42,
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "float instead of string",
			defaultValue: 3.14,
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "boolean instead of string",
			defaultValue: true,
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "slice instead of string",
			defaultValue: []string{"a", "b"},
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "map instead of string",
			defaultValue: map[string]string{"key": "value"},
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "nil value is valid (no default)",
			defaultValue: nil,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"test_field": {
						Type:    pluginmodel.ConfigTypeString,
						Default: tt.defaultValue,
					},
				},
			}
			err := m.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateDefaultType_IntegerType tests integer type validation via validateDefaultType
func TestValidateDefaultType_IntegerType(t *testing.T) {
	// Component: T003, T008
	// Feature: C031
	tests := []struct {
		name         string
		defaultValue any
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "valid int",
			defaultValue: 42,
			wantErr:      false,
		},
		{
			name:         "valid int8",
			defaultValue: int8(127),
			wantErr:      false,
		},
		{
			name:         "valid int16",
			defaultValue: int16(32767),
			wantErr:      false,
		},
		{
			name:         "valid int32",
			defaultValue: int32(2147483647),
			wantErr:      false,
		},
		{
			name:         "valid int64",
			defaultValue: int64(9223372036854775807),
			wantErr:      false,
		},
		{
			name:         "valid uint",
			defaultValue: uint(42),
			wantErr:      false,
		},
		{
			name:         "valid uint8",
			defaultValue: uint8(255),
			wantErr:      false,
		},
		{
			name:         "valid uint16",
			defaultValue: uint16(65535),
			wantErr:      false,
		},
		{
			name:         "valid uint32",
			defaultValue: uint32(4294967295),
			wantErr:      false,
		},
		{
			name:         "valid uint64",
			defaultValue: uint64(18446744073709551615),
			wantErr:      false,
		},
		{
			name:         "valid float64 from JSON (42.0)",
			defaultValue: float64(42),
			wantErr:      false,
		},
		{
			name:         "valid float64 from JSON (negative)",
			defaultValue: float64(-100),
			wantErr:      false,
		},
		{
			name:         "valid zero",
			defaultValue: 0,
			wantErr:      false,
		},
		{
			name:         "valid negative integer",
			defaultValue: -42,
			wantErr:      false,
		},
		{
			name:         "string instead of integer",
			defaultValue: "42",
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "boolean instead of integer",
			defaultValue: true,
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "slice instead of integer",
			defaultValue: []int{1, 2, 3},
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "nil value is valid (no default)",
			defaultValue: nil,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"test_field": {
						Type:    pluginmodel.ConfigTypeInteger,
						Default: tt.defaultValue,
					},
				},
			}
			err := m.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateDefaultType_BooleanType tests boolean type validation via validateDefaultType
func TestValidateDefaultType_BooleanType(t *testing.T) {
	// Component: T003, T008
	// Feature: C031
	tests := []struct {
		name         string
		defaultValue any
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "valid true",
			defaultValue: true,
			wantErr:      false,
		},
		{
			name:         "valid false",
			defaultValue: false,
			wantErr:      false,
		},
		{
			name:         "string 'true' instead of boolean",
			defaultValue: "true",
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "string 'false' instead of boolean",
			defaultValue: "false",
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "integer 1 instead of boolean",
			defaultValue: 1,
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "integer 0 instead of boolean",
			defaultValue: 0,
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "slice instead of boolean",
			defaultValue: []bool{true, false},
			wantErr:      true,
			errMsg:       "type mismatch",
		},
		{
			name:         "nil value is valid (no default)",
			defaultValue: nil,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"test_field": {
						Type:    pluginmodel.ConfigTypeBoolean,
						Default: tt.defaultValue,
					},
				},
			}
			err := m.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateDefaultType_TypeMismatchMessages tests error message clarity
func TestValidateDefaultType_TypeMismatchMessages(t *testing.T) {
	// Component: T003, T008
	// Feature: C031
	tests := []struct {
		name          string
		configType    string
		defaultValue  any
		wantErrSubstr []string
	}{
		{
			name:          "string type expects string",
			configType:    pluginmodel.ConfigTypeString,
			defaultValue:  123,
			wantErrSubstr: []string{"type mismatch", "expected string", "got int"},
		},
		{
			name:          "integer type expects integer",
			configType:    pluginmodel.ConfigTypeInteger,
			defaultValue:  "not-a-number",
			wantErrSubstr: []string{"type mismatch", "expected integer", "got string"},
		},
		{
			name:          "boolean type expects bool",
			configType:    pluginmodel.ConfigTypeBoolean,
			defaultValue:  "true",
			wantErrSubstr: []string{"type mismatch", "expected bool", "got string"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"test_field": {
						Type:    tt.configType,
						Default: tt.defaultValue,
					},
				},
			}
			err := m.Validate()
			require.Error(t, err)
			for _, substr := range tt.wantErrSubstr {
				assert.Contains(t, err.Error(), substr, "error message should contain %q", substr)
			}
		})
	}
}

// TestValidateDefaultType_EdgeCases tests edge cases in type validation
func TestValidateDefaultType_EdgeCases(t *testing.T) {
	// Component: T003, T008
	// Feature: C031
	tests := []struct {
		name       string
		field      pluginmodel.ConfigField
		wantErr    bool
		errMsg     string
		skipReason string
	}{
		{
			name: "nil default with string type (optional field)",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeString,
				Default: nil,
			},
			wantErr: false,
		},
		{
			name: "nil default with integer type (optional field)",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeInteger,
				Default: nil,
			},
			wantErr: false,
		},
		{
			name: "nil default with boolean type (optional field)",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeBoolean,
				Default: nil,
			},
			wantErr: false,
		},
		{
			name: "struct instead of primitive type",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeString,
				Default: struct{ Value string }{Value: "test"},
			},
			wantErr: true,
			errMsg:  "type mismatch",
		},
		{
			name: "pointer to string instead of string",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeString,
				Default: func() *string { s := "test"; return &s }(),
			},
			wantErr: true,
			errMsg:  "type mismatch",
		},
		{
			name: "pointer to int instead of int",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeInteger,
				Default: func() *int { i := 42; return &i }(),
			},
			wantErr: true,
			errMsg:  "type mismatch",
		},
		{
			name: "pointer to bool instead of bool",
			field: pluginmodel.ConfigField{
				Type:    pluginmodel.ConfigTypeBoolean,
				Default: func() *bool { b := true; return &b }(),
			},
			wantErr: true,
			errMsg:  "type mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"test_field": tt.field,
				},
			}
			err := m.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateDefaultType_JSONUnmarshalBehavior tests JSON float64 handling for integers
func TestValidateDefaultType_JSONUnmarshalBehavior(t *testing.T) {
	// Component: T003, T008
	// Feature: C031
	//
	// This test verifies that validateDefaultType correctly handles the JSON
	// unmarshaling behavior where all numbers are decoded as float64.
	// This is critical for YAML/JSON config parsing.
	tests := []struct {
		name         string
		defaultValue any
		wantErr      bool
	}{
		{
			name:         "float64(42) from JSON should be valid for integer type",
			defaultValue: float64(42),
			wantErr:      false,
		},
		{
			name:         "float64(0) should be valid for integer type",
			defaultValue: float64(0),
			wantErr:      false,
		},
		{
			name:         "float64(-100) should be valid for integer type",
			defaultValue: float64(-100),
			wantErr:      false,
		},
		{
			name:         "float64(3.14) should be valid (no fractional check)",
			defaultValue: float64(3.14),
			wantErr:      false,
		},
		{
			name:         "native int should still work",
			defaultValue: 42,
			wantErr:      false,
		},
		{
			name:         "float32 is NOT accepted (not from JSON)",
			defaultValue: float32(42),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"test_field": {
						Type:    pluginmodel.ConfigTypeInteger,
						Default: tt.defaultValue,
					},
				},
			}
			err := m.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateDefaultType_MultipleFields tests type validation across multiple config fields
func TestValidateDefaultType_MultipleFields(t *testing.T) {
	// Component: T003, T008
	// Feature: C031
	tests := []struct {
		name    string
		config  map[string]pluginmodel.ConfigField
		wantErr bool
		errMsg  string
	}{
		{
			name: "all fields valid with correct types",
			config: map[string]pluginmodel.ConfigField{
				"api_key":     {Type: pluginmodel.ConfigTypeString, Default: "secret-key"},
				"port":        {Type: pluginmodel.ConfigTypeInteger, Default: 8080},
				"enabled":     {Type: pluginmodel.ConfigTypeBoolean, Default: true},
				"timeout":     {Type: pluginmodel.ConfigTypeInteger, Default: float64(30)},
				"description": {Type: pluginmodel.ConfigTypeString, Default: ""},
			},
			wantErr: false,
		},
		{
			name: "first field invalid type",
			config: map[string]pluginmodel.ConfigField{
				"api_key": {Type: pluginmodel.ConfigTypeString, Default: 12345},
				"port":    {Type: pluginmodel.ConfigTypeInteger, Default: 8080},
			},
			wantErr: true,
			errMsg:  "type mismatch",
		},
		{
			name: "middle field invalid type",
			config: map[string]pluginmodel.ConfigField{
				"api_key": {Type: pluginmodel.ConfigTypeString, Default: "secret"},
				"port":    {Type: pluginmodel.ConfigTypeInteger, Default: "8080"},
				"enabled": {Type: pluginmodel.ConfigTypeBoolean, Default: true},
			},
			wantErr: true,
			errMsg:  "type mismatch",
		},
		{
			name: "last field invalid type",
			config: map[string]pluginmodel.ConfigField{
				"api_key": {Type: pluginmodel.ConfigTypeString, Default: "secret"},
				"port":    {Type: pluginmodel.ConfigTypeInteger, Default: 8080},
				"enabled": {Type: pluginmodel.ConfigTypeBoolean, Default: "true"},
			},
			wantErr: true,
			errMsg:  "type mismatch",
		},
		{
			name: "all fields with nil defaults",
			config: map[string]pluginmodel.ConfigField{
				"api_key": {Type: pluginmodel.ConfigTypeString, Default: nil},
				"port":    {Type: pluginmodel.ConfigTypeInteger, Default: nil},
				"enabled": {Type: pluginmodel.ConfigTypeBoolean, Default: nil},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config:     tt.config,
			}
			err := m.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Manifest.Validate() Integration Tests (Component T004)
// Feature: C031

// TestManifestValidate_HappyPath tests valid manifests that should pass all validation
func TestManifestValidate_HappyPath(t *testing.T) {
	// Component: T004
	// Feature: C031
	tests := []struct {
		name     string
		manifest pluginmodel.Manifest
	}{
		{
			name: "minimal valid manifest",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
		},
		{
			name: "complete valid manifest with all fields",
			manifest: pluginmodel.Manifest{
				Name:        "marketplace-plugin",
				Version:     "1.2.3",
				AWFVersion:  ">=0.4.0",
				Description: "A marketplace integration plugin",
				Author:      "AWF Team <team@awf.dev>",
				License:     "MIT",
				Homepage:    "https://github.com/awf-project/cli",
				Capabilities: []string{
					pluginmodel.CapabilityOperations,
					pluginmodel.CapabilityStepTypes,
					pluginmodel.CapabilityValidators,
				},
				Config: map[string]pluginmodel.ConfigField{
					"api_key": {
						Type:        pluginmodel.ConfigTypeString,
						Required:    true,
						Description: "API key for authentication",
					},
					"timeout": {
						Type:        pluginmodel.ConfigTypeInteger,
						Default:     30,
						Description: "Request timeout in seconds",
					},
					"enabled": {
						Type:        pluginmodel.ConfigTypeBoolean,
						Default:     true,
						Description: "Enable plugin",
					},
					"environment": {
						Type:        pluginmodel.ConfigTypeString,
						Default:     "production",
						Enum:        []string{"development", "staging", "production"},
						Description: "Deployment environment",
					},
				},
			},
		},
		{
			name: "manifest with single capability",
			manifest: pluginmodel.Manifest{
				Name:         "simple-plugin",
				Version:      "0.1.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{pluginmodel.CapabilityOperations},
			},
		},
		{
			name: "manifest with empty capabilities list",
			manifest: pluginmodel.Manifest{
				Name:         "basic-plugin",
				Version:      "2.0.0",
				AWFVersion:   "^0.4.0",
				Capabilities: []string{},
			},
		},
		{
			name: "manifest with nil config",
			manifest: pluginmodel.Manifest{
				Name:       "no-config-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config:     nil,
			},
		},
		{
			name: "manifest with version prerelease",
			manifest: pluginmodel.Manifest{
				Name:       "beta-plugin",
				Version:    "1.0.0-beta.1",
				AWFVersion: ">=0.4.0",
			},
		},
		{
			name: "manifest with version build metadata",
			manifest: pluginmodel.Manifest{
				Name:       "build-plugin",
				Version:    "1.0.0+build.123",
				AWFVersion: ">=0.4.0",
			},
		},
		{
			name: "manifest with complex name pattern",
			manifest: pluginmodel.Manifest{
				Name:       "awf-marketplace-v2-beta",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			assert.NoError(t, err, "expected valid manifest to pass validation")
			assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented, "should not return ErrNotImplemented")
		})
	}
}

// TestManifestValidate_NameValidation tests name field validation
func TestManifestValidate_NameValidation(t *testing.T) {
	// Component: T004, T005
	// Feature: C031
	tests := []struct {
		name      string
		manifest  pluginmodel.Manifest
		wantErr   bool
		errSubstr string
	}{
		{
			name: "empty name",
			manifest: pluginmodel.Manifest{
				Name:       "",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr:   true,
			errSubstr: "name cannot be empty",
		},
		{
			name: "whitespace only name",
			manifest: pluginmodel.Manifest{
				Name:       "   ",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr:   true,
			errSubstr: "invalid name",
		},
		{
			name: "name starts with digit",
			manifest: pluginmodel.Manifest{
				Name:       "1plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr:   true,
			errSubstr: "invalid name",
		},
		{
			name: "name contains uppercase",
			manifest: pluginmodel.Manifest{
				Name:       "MyPlugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr:   true,
			errSubstr: "invalid name",
		},
		{
			name: "name contains underscore",
			manifest: pluginmodel.Manifest{
				Name:       "my_plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr:   true,
			errSubstr: "invalid name",
		},
		{
			name: "name contains space",
			manifest: pluginmodel.Manifest{
				Name:       "my plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr:   true,
			errSubstr: "invalid name",
		},
		{
			name: "name contains special characters",
			manifest: pluginmodel.Manifest{
				Name:       "my@plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr:   true,
			errSubstr: "invalid name",
		},
		{
			name: "name starts with hyphen",
			manifest: pluginmodel.Manifest{
				Name:       "-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr:   true,
			errSubstr: "invalid name",
		},
		{
			name: "valid lowercase name",
			manifest: pluginmodel.Manifest{
				Name:       "plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr: false,
		},
		{
			name: "valid name with hyphens and numbers",
			manifest: pluginmodel.Manifest{
				Name:       "my-plugin-v2",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestManifestValidate_VersionValidation tests version field validation
func TestManifestValidate_VersionValidation(t *testing.T) {
	// Component: T004, T006
	// Feature: C031
	tests := []struct {
		name      string
		manifest  pluginmodel.Manifest
		wantErr   bool
		errSubstr string
	}{
		{
			name: "empty version",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "",
				AWFVersion: ">=0.4.0",
			},
			wantErr:   true,
			errSubstr: "version cannot be empty",
		},
		{
			name: "valid semantic version",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr: false,
		},
		{
			name: "valid version with prerelease",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0-alpha.1",
				AWFVersion: ">=0.4.0",
			},
			wantErr: false,
		},
		{
			name: "valid version with build metadata",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0+20230101",
				AWFVersion: ">=0.4.0",
			},
			wantErr: false,
		},
		{
			name: "valid version major.minor",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr: false,
		},
		{
			name: "valid version major only",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1",
				AWFVersion: ">=0.4.0",
			},
			wantErr: false,
		},
		{
			name: "arbitrary version string (ADR-003 minimal validation)",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "v2024.01.01",
				AWFVersion: ">=0.4.0",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestManifestValidate_AWFVersionValidation tests AWFVersion field validation
func TestManifestValidate_AWFVersionValidation(t *testing.T) {
	// Component: T004, T006
	// Feature: C031
	tests := []struct {
		name      string
		manifest  pluginmodel.Manifest
		wantErr   bool
		errSubstr string
	}{
		{
			name: "empty AWFVersion",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: "",
			},
			wantErr:   true,
			errSubstr: "awf_version cannot be empty",
		},
		{
			name: "valid constraint >=",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr: false,
		},
		{
			name: "valid constraint ^",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: "^0.4.0",
			},
			wantErr: false,
		},
		{
			name: "valid constraint ~",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: "~0.4.0",
			},
			wantErr: false,
		},
		{
			name: "valid range constraint",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0 <1.0.0",
			},
			wantErr: false,
		},
		{
			name: "valid exact version",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: "0.4.0",
			},
			wantErr: false,
		},
		{
			name: "arbitrary constraint string (ADR-003 minimal validation)",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: "latest",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestManifestValidate_CapabilitiesValidation tests capabilities list validation
func TestManifestValidate_CapabilitiesValidation(t *testing.T) {
	// Component: T004, T007
	// Feature: C031
	tests := []struct {
		name      string
		manifest  pluginmodel.Manifest
		wantErr   bool
		errSubstr string
	}{
		{
			name: "nil capabilities list",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: nil,
			},
			wantErr: false,
		},
		{
			name: "empty capabilities list",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{},
			},
			wantErr: false,
		},
		{
			name: "single valid capability - operations",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{pluginmodel.CapabilityOperations},
			},
			wantErr: false,
		},
		{
			name: "single valid capability - step_types",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{pluginmodel.CapabilityStepTypes},
			},
			wantErr: false,
		},
		{
			name: "single valid capability - validators",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{pluginmodel.CapabilityValidators},
			},
			wantErr: false,
		},
		{
			name: "all valid capabilities",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Capabilities: []string{
					pluginmodel.CapabilityOperations,
					pluginmodel.CapabilityStepTypes,
					pluginmodel.CapabilityValidators,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid capability",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{"unknown"},
			},
			wantErr:   true,
			errSubstr: "invalid capability",
		},
		{
			name: "mixed valid and invalid capabilities",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Capabilities: []string{
					pluginmodel.CapabilityOperations,
					"invalid-capability",
				},
			},
			wantErr:   true,
			errSubstr: "invalid capability",
		},
		{
			name: "duplicate valid capabilities",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Capabilities: []string{
					pluginmodel.CapabilityOperations,
					pluginmodel.CapabilityOperations,
				},
			},
			wantErr: false, // duplicates are allowed per spec
		},
		{
			name: "capability with typo",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{"operation"}, // missing 's'
			},
			wantErr:   true,
			errSubstr: "invalid capability",
		},
		{
			name: "capability with wrong case",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{"Operations"}, // capitalized
			},
			wantErr:   true,
			errSubstr: "invalid capability",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestManifestValidate_ConfigValidation tests config field validation integration
func TestManifestValidate_ConfigValidation(t *testing.T) {
	// Component: T004, T008
	// Feature: C031
	tests := []struct {
		name      string
		manifest  pluginmodel.Manifest
		wantErr   bool
		errSubstr string
	}{
		{
			name: "nil config map",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config:     nil,
			},
			wantErr: false,
		},
		{
			name: "empty config map",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config:     map[string]pluginmodel.ConfigField{},
			},
			wantErr: false,
		},
		{
			name: "valid single config field",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"api_key": {
						Type:        pluginmodel.ConfigTypeString,
						Required:    true,
						Description: "API key",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid config field type",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"bad_field": {
						Type: "invalid_type",
					},
				},
			},
			wantErr:   true,
			errSubstr: "config field \"bad_field\"",
		},
		{
			name: "config field with enum on non-string type",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"port": {
						Type: pluginmodel.ConfigTypeInteger,
						Enum: []string{"8080", "8081"},
					},
				},
			},
			wantErr:   true,
			errSubstr: "enum",
		},
		{
			name: "config field with type mismatch default",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"timeout": {
						Type:    pluginmodel.ConfigTypeInteger,
						Default: "not-a-number",
					},
				},
			},
			wantErr:   true,
			errSubstr: "type mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestManifestValidate_FailFastBehavior tests that validation returns first error
func TestManifestValidate_FailFastBehavior(t *testing.T) {
	// Component: T004
	// Feature: C031
	//
	// Per ADR-001, validation should fail fast and return the first error encountered.
	// This test verifies that multiple errors don't result in aggregated errors.
	tests := []struct {
		name        string
		manifest    pluginmodel.Manifest
		firstErrMsg string // The first validation error expected
		notErrMsg   string // A later error that should NOT appear
	}{
		{
			name: "empty name fails before version check",
			manifest: pluginmodel.Manifest{
				Name:       "",
				Version:    "", // also invalid
				AWFVersion: ">=0.4.0",
			},
			firstErrMsg: "name",
			notErrMsg:   "version",
		},
		{
			name: "invalid name fails before version check",
			manifest: pluginmodel.Manifest{
				Name:       "Invalid-Name",
				Version:    "", // also invalid
				AWFVersion: ">=0.4.0",
			},
			firstErrMsg: "name",
			notErrMsg:   "version",
		},
		{
			name: "empty version fails before AWFVersion check",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "",
				AWFVersion: "", // also invalid
			},
			firstErrMsg: "version",
			notErrMsg:   "awf_version",
		},
		{
			name: "empty AWFVersion fails before capabilities check",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   "",
				Capabilities: []string{"invalid"}, // also invalid
			},
			firstErrMsg: "awf_version",
			notErrMsg:   "capability",
		},
		{
			name: "invalid capability fails before config check",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{"bad-capability"},
				Config: map[string]pluginmodel.ConfigField{
					"bad": {Type: "invalid"}, // also invalid
				},
			},
			firstErrMsg: "capability",
			notErrMsg:   "config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.firstErrMsg,
				"should contain first error: %s", tt.firstErrMsg)
			assert.NotContains(t, err.Error(), tt.notErrMsg,
				"should NOT contain later error: %s (fail-fast behavior)", tt.notErrMsg)
			assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
		})
	}
}

// TestManifestValidate_EdgeCases tests edge cases and boundary conditions
func TestManifestValidate_EdgeCases(t *testing.T) {
	// Component: T004
	// Feature: C031
	tests := []struct {
		name      string
		manifest  pluginmodel.Manifest
		wantErr   bool
		errSubstr string
	}{
		{
			name: "all fields empty",
			manifest: pluginmodel.Manifest{
				Name:       "",
				Version:    "",
				AWFVersion: "",
			},
			wantErr:   true,
			errSubstr: "name",
		},
		{
			name: "whitespace in name",
			manifest: pluginmodel.Manifest{
				Name:       "  ",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr:   true,
			errSubstr: "name",
		},
		{
			name: "single character name",
			manifest: pluginmodel.Manifest{
				Name:       "a",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr: false,
		},
		{
			name: "very long valid name",
			manifest: pluginmodel.Manifest{
				Name:       "very-long-plugin-name-with-many-hyphens-and-numbers-12345",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			wantErr: false,
		},
		{
			name: "manifest with all optional fields filled",
			manifest: pluginmodel.Manifest{
				Name:        "complete-plugin",
				Version:     "1.0.0",
				AWFVersion:  ">=0.4.0",
				Description: "Complete plugin with all fields",
				Author:      "Author Name",
				License:     "MIT",
				Homepage:    "https://example.com",
				Capabilities: []string{
					pluginmodel.CapabilityOperations,
					pluginmodel.CapabilityStepTypes,
				},
				Config: map[string]pluginmodel.ConfigField{
					"key": {
						Type:        pluginmodel.ConfigTypeString,
						Required:    true,
						Default:     "value",
						Description: "A key",
						Enum:        []string{"value", "other"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "zero-length capabilities slice (not nil)",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{},
			},
			wantErr: false,
		},
		{
			name: "config with empty field name key",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"": {Type: pluginmodel.ConfigTypeString},
				},
			},
			wantErr: false, // empty keys are allowed in Go maps
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestManifestValidate_ErrorMessages tests error message clarity and helpfulness
func TestManifestValidate_ErrorMessages(t *testing.T) {
	// Component: T004
	// Feature: C031
	tests := []struct {
		name           string
		manifest       pluginmodel.Manifest
		expectedSubstr []string // All substrings that should appear in error
	}{
		{
			name: "invalid name error shows pattern",
			manifest: pluginmodel.Manifest{
				Name:       "Invalid_Name",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
			expectedSubstr: []string{"invalid name", "Invalid_Name", "pattern"},
		},
		{
			name: "invalid capability error shows valid options",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{"bad-capability"},
			},
			expectedSubstr: []string{"invalid capability", "bad-capability", "operations", "step_types", "validators"},
		},
		{
			name: "config field error includes field name",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]pluginmodel.ConfigField{
					"problematic_field": {Type: "invalid"},
				},
			},
			expectedSubstr: []string{"config field", "problematic_field"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			require.Error(t, err)
			for _, substr := range tt.expectedSubstr {
				assert.Contains(t, err.Error(), substr,
					"error message should contain %q", substr)
			}
		})
	}
}

// TestCapabilityStepTypes_ReplaceCommands verifies T005: "commands" is renamed to "step_types"
func TestCapabilityStepTypes_ReplaceCommands(t *testing.T) {
	// Component: T005
	// Feature: C069
	assert.NotContains(t, pluginmodel.ValidCapabilities, "commands",
		"'commands' capability was renamed to 'step_types' in T005")

	m := pluginmodel.Manifest{
		Name:         "test-plugin",
		Version:      "1.0.0",
		AWFVersion:   ">=0.4.0",
		Capabilities: []string{"commands"},
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid capability")
	assert.Contains(t, err.Error(), "step_types",
		"error should suggest 'step_types' as the replacement")
}

// TestManifestValidate_NoLongerReturnsErrNotImplemented verifies stub is replaced
func TestManifestValidate_NoLongerReturnsErrNotImplemented(t *testing.T) {
	// Component: T004
	// Feature: C031
	//
	// This test explicitly verifies that Manifest.Validate() no longer returns
	// ErrNotImplemented, confirming that the stub has been fully replaced with
	// actual validation logic.
	tests := []struct {
		name     string
		manifest pluginmodel.Manifest
	}{
		{
			name: "valid manifest should not return ErrNotImplemented",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
		},
		{
			name: "invalid manifest should not return ErrNotImplemented",
			manifest: pluginmodel.Manifest{
				Name:       "", // invalid
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented,
				"Validate() should never return ErrNotImplemented after C031 implementation")
		})
	}
}
