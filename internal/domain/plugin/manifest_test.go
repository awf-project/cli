package plugin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/plugin"
)

// =============================================================================
// Constants Tests
// =============================================================================

func TestValidCapabilities_ContainsExpectedValues(t *testing.T) {
	assert.Contains(t, plugin.ValidCapabilities, plugin.CapabilityOperations)
	assert.Contains(t, plugin.ValidCapabilities, plugin.CapabilityCommands)
	assert.Contains(t, plugin.ValidCapabilities, plugin.CapabilityValidators)
	assert.Len(t, plugin.ValidCapabilities, 3)
}

func TestCapabilityConstants_Values(t *testing.T) {
	assert.Equal(t, "operations", plugin.CapabilityOperations)
	assert.Equal(t, "commands", plugin.CapabilityCommands)
	assert.Equal(t, "validators", plugin.CapabilityValidators)
}

func TestValidConfigTypes_ContainsExpectedValues(t *testing.T) {
	assert.Contains(t, plugin.ValidConfigTypes, plugin.ConfigTypeString)
	assert.Contains(t, plugin.ValidConfigTypes, plugin.ConfigTypeInteger)
	assert.Contains(t, plugin.ValidConfigTypes, plugin.ConfigTypeBoolean)
	assert.Len(t, plugin.ValidConfigTypes, 3)
}

func TestConfigTypeConstants_Values(t *testing.T) {
	assert.Equal(t, "string", plugin.ConfigTypeString)
	assert.Equal(t, "integer", plugin.ConfigTypeInteger)
	assert.Equal(t, "boolean", plugin.ConfigTypeBoolean)
}

// =============================================================================
// Manifest Creation Tests
// =============================================================================

func TestManifest_Creation(t *testing.T) {
	m := plugin.Manifest{
		Name:         "slack-notifier",
		Version:      "1.0.0",
		Description:  "Send notifications to Slack",
		AWFVersion:   ">=0.4.0",
		Capabilities: []string{"operations"},
		Config: map[string]plugin.ConfigField{
			"webhook_url": {
				Type:        "string",
				Required:    true,
				Description: "Slack webhook URL",
			},
		},
	}

	assert.Equal(t, "slack-notifier", m.Name)
	assert.Equal(t, "1.0.0", m.Version)
	assert.Equal(t, "Send notifications to Slack", m.Description)
	assert.Equal(t, ">=0.4.0", m.AWFVersion)
	assert.Contains(t, m.Capabilities, "operations")
	require.Contains(t, m.Config, "webhook_url")
	assert.Equal(t, "string", m.Config["webhook_url"].Type)
	assert.True(t, m.Config["webhook_url"].Required)
}

func TestManifest_FullManifest(t *testing.T) {
	m := plugin.Manifest{
		Name:         "awf-plugin-slack",
		Version:      "1.0.0",
		Description:  "Slack notifications for AWF workflows",
		AWFVersion:   ">=0.4.0",
		Author:       "Jane Developer <jane@example.com>",
		License:      "MIT",
		Homepage:     "https://github.com/example/awf-plugin-slack",
		Capabilities: []string{plugin.CapabilityOperations, plugin.CapabilityCommands},
		Config: map[string]plugin.ConfigField{
			"webhook_url": {
				Type:        plugin.ConfigTypeString,
				Required:    true,
				Description: "Slack webhook URL",
			},
			"channel": {
				Type:        plugin.ConfigTypeString,
				Required:    false,
				Default:     "#general",
				Description: "Target Slack channel",
			},
		},
	}

	assert.Equal(t, "awf-plugin-slack", m.Name)
	assert.Equal(t, "1.0.0", m.Version)
	assert.Equal(t, "Slack notifications for AWF workflows", m.Description)
	assert.Equal(t, ">=0.4.0", m.AWFVersion)
	assert.Equal(t, "Jane Developer <jane@example.com>", m.Author)
	assert.Equal(t, "MIT", m.License)
	assert.Equal(t, "https://github.com/example/awf-plugin-slack", m.Homepage)
	assert.Len(t, m.Capabilities, 2)
	assert.Len(t, m.Config, 2)
}

func TestManifest_EmptyConfig(t *testing.T) {
	m := plugin.Manifest{
		Name:         "simple-plugin",
		Version:      "0.1.0",
		Capabilities: []string{"operations"},
	}

	assert.Empty(t, m.Config)
	assert.Empty(t, m.Description)
	assert.Empty(t, m.AWFVersion)
	assert.Empty(t, m.Author)
	assert.Empty(t, m.License)
	assert.Empty(t, m.Homepage)
}

func TestManifest_MultipleCapabilities(t *testing.T) {
	m := plugin.Manifest{
		Name:         "full-plugin",
		Version:      "2.0.0",
		Capabilities: []string{"operations", "commands", "validators"},
	}

	assert.Len(t, m.Capabilities, 3)
	assert.Contains(t, m.Capabilities, "operations")
	assert.Contains(t, m.Capabilities, "commands")
	assert.Contains(t, m.Capabilities, "validators")
}

func TestManifest_OptionalMetadata(t *testing.T) {
	tests := []struct {
		name     string
		manifest plugin.Manifest
	}{
		{
			name: "with author only",
			manifest: plugin.Manifest{
				Name:    "test-plugin",
				Version: "1.0.0",
				Author:  "Test Author",
			},
		},
		{
			name: "with license only",
			manifest: plugin.Manifest{
				Name:    "test-plugin",
				Version: "1.0.0",
				License: "Apache-2.0",
			},
		},
		{
			name: "with homepage only",
			manifest: plugin.Manifest{
				Name:     "test-plugin",
				Version:  "1.0.0",
				Homepage: "https://example.com",
			},
		},
		{
			name: "with all optional metadata",
			manifest: plugin.Manifest{
				Name:        "test-plugin",
				Version:     "1.0.0",
				Description: "A test plugin",
				Author:      "Test Author <test@example.com>",
				License:     "MIT",
				Homepage:    "https://github.com/example/test-plugin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.manifest.Name)
			assert.NotEmpty(t, tt.manifest.Version)
		})
	}
}

// =============================================================================
// HasCapability Tests
// =============================================================================

func TestManifest_HasCapability(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []string
		check        string
		want         bool
	}{
		{
			name:         "has operations capability",
			capabilities: []string{plugin.CapabilityOperations},
			check:        plugin.CapabilityOperations,
			want:         true,
		},
		{
			name:         "has commands capability",
			capabilities: []string{plugin.CapabilityCommands},
			check:        plugin.CapabilityCommands,
			want:         true,
		},
		{
			name:         "has validators capability",
			capabilities: []string{plugin.CapabilityValidators},
			check:        plugin.CapabilityValidators,
			want:         true,
		},
		{
			name:         "multiple capabilities - check first",
			capabilities: []string{plugin.CapabilityOperations, plugin.CapabilityCommands},
			check:        plugin.CapabilityOperations,
			want:         true,
		},
		{
			name:         "multiple capabilities - check second",
			capabilities: []string{plugin.CapabilityOperations, plugin.CapabilityCommands},
			check:        plugin.CapabilityCommands,
			want:         true,
		},
		{
			name:         "all capabilities - check validators",
			capabilities: []string{plugin.CapabilityOperations, plugin.CapabilityCommands, plugin.CapabilityValidators},
			check:        plugin.CapabilityValidators,
			want:         true,
		},
		{
			name:         "does not have capability",
			capabilities: []string{plugin.CapabilityOperations},
			check:        plugin.CapabilityCommands,
			want:         false,
		},
		{
			name:         "empty capabilities",
			capabilities: []string{},
			check:        plugin.CapabilityOperations,
			want:         false,
		},
		{
			name:         "nil capabilities",
			capabilities: nil,
			check:        plugin.CapabilityOperations,
			want:         false,
		},
		{
			name:         "unknown capability check",
			capabilities: []string{plugin.CapabilityOperations},
			check:        "unknown",
			want:         false,
		},
		{
			name:         "empty string capability check",
			capabilities: []string{plugin.CapabilityOperations},
			check:        "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := plugin.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				Capabilities: tt.capabilities,
			}

			got := m.HasCapability(tt.check)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// ConfigField Tests
// =============================================================================

func TestConfigField_Creation(t *testing.T) {
	tests := []struct {
		name     string
		field    plugin.ConfigField
		wantType string
		wantReq  bool
	}{
		{
			name: "required string field",
			field: plugin.ConfigField{
				Type:        "string",
				Required:    true,
				Description: "A required string",
			},
			wantType: "string",
			wantReq:  true,
		},
		{
			name: "optional integer with default",
			field: plugin.ConfigField{
				Type:        "integer",
				Required:    false,
				Default:     42,
				Description: "Port number",
			},
			wantType: "integer",
			wantReq:  false,
		},
		{
			name: "boolean field",
			field: plugin.ConfigField{
				Type:     "boolean",
				Required: false,
				Default:  true,
			},
			wantType: "boolean",
			wantReq:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantType, tt.field.Type)
			assert.Equal(t, tt.wantReq, tt.field.Required)
		})
	}
}

func TestConfigField_DefaultValues(t *testing.T) {
	tests := []struct {
		name        string
		field       plugin.ConfigField
		wantDefault any
	}{
		{
			name: "string default",
			field: plugin.ConfigField{
				Type:    "string",
				Default: "default-value",
			},
			wantDefault: "default-value",
		},
		{
			name: "integer default",
			field: plugin.ConfigField{
				Type:    "integer",
				Default: 8080,
			},
			wantDefault: 8080,
		},
		{
			name: "boolean default true",
			field: plugin.ConfigField{
				Type:    "boolean",
				Default: true,
			},
			wantDefault: true,
		},
		{
			name: "boolean default false",
			field: plugin.ConfigField{
				Type:    "boolean",
				Default: false,
			},
			wantDefault: false,
		},
		{
			name: "nil default",
			field: plugin.ConfigField{
				Type: "string",
			},
			wantDefault: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantDefault, tt.field.Default)
		})
	}
}

func TestConfigField_WithEnum(t *testing.T) {
	tests := []struct {
		name     string
		field    plugin.ConfigField
		wantEnum []string
	}{
		{
			name: "string field with enum",
			field: plugin.ConfigField{
				Type:        plugin.ConfigTypeString,
				Required:    true,
				Description: "Environment",
				Enum:        []string{"development", "staging", "production"},
			},
			wantEnum: []string{"development", "staging", "production"},
		},
		{
			name: "single value enum",
			field: plugin.ConfigField{
				Type: plugin.ConfigTypeString,
				Enum: []string{"only-option"},
			},
			wantEnum: []string{"only-option"},
		},
		{
			name: "empty enum",
			field: plugin.ConfigField{
				Type: plugin.ConfigTypeString,
				Enum: []string{},
			},
			wantEnum: []string{},
		},
		{
			name: "nil enum",
			field: plugin.ConfigField{
				Type: plugin.ConfigTypeString,
			},
			wantEnum: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantEnum == nil {
				assert.Nil(t, tt.field.Enum)
			} else {
				assert.Equal(t, tt.wantEnum, tt.field.Enum)
			}
		})
	}
}

func TestConfigField_FullField(t *testing.T) {
	field := plugin.ConfigField{
		Type:        plugin.ConfigTypeString,
		Required:    true,
		Default:     "info",
		Description: "Log level for the plugin",
		Enum:        []string{"debug", "info", "warn", "error"},
	}

	assert.Equal(t, plugin.ConfigTypeString, field.Type)
	assert.True(t, field.Required)
	assert.Equal(t, "info", field.Default)
	assert.Equal(t, "Log level for the plugin", field.Description)
	assert.Len(t, field.Enum, 4)
	assert.Contains(t, field.Enum, "debug")
	assert.Contains(t, field.Enum, "info")
	assert.Contains(t, field.Enum, "warn")
	assert.Contains(t, field.Enum, "error")
}

// =============================================================================
// Edge Cases and Boundary Tests
// =============================================================================

func TestManifest_NameFormats(t *testing.T) {
	tests := []struct {
		name       string
		pluginName string
	}{
		{name: "simple name", pluginName: "myplugin"},
		{name: "with hyphen", pluginName: "my-plugin"},
		{name: "with multiple hyphens", pluginName: "my-awesome-plugin"},
		{name: "with numbers", pluginName: "plugin123"},
		{name: "with prefix", pluginName: "awf-plugin-slack"},
		{name: "single char", pluginName: "a"},
		{name: "numeric start", pluginName: "123plugin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := plugin.Manifest{
				Name:    tt.pluginName,
				Version: "1.0.0",
			}
			assert.Equal(t, tt.pluginName, m.Name)
		})
	}
}

func TestManifest_VersionFormats(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{name: "standard semver", version: "1.0.0"},
		{name: "with v prefix", version: "v1.0.0"},
		{name: "prerelease", version: "1.0.0-alpha"},
		{name: "prerelease with number", version: "1.0.0-beta.1"},
		{name: "build metadata", version: "1.0.0+build.123"},
		{name: "full semver", version: "1.0.0-rc.1+build.456"},
		{name: "zero version", version: "0.0.0"},
		{name: "high version", version: "99.99.99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := plugin.Manifest{
				Name:    "test-plugin",
				Version: tt.version,
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
			m := plugin.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: tt.constraint,
			}
			assert.Equal(t, tt.constraint, m.AWFVersion)
		})
	}
}

func TestManifest_ConfigWithMultipleFields(t *testing.T) {
	m := plugin.Manifest{
		Name:         "complex-plugin",
		Version:      "1.0.0",
		Capabilities: []string{plugin.CapabilityOperations},
		Config: map[string]plugin.ConfigField{
			"api_url": {
				Type:        plugin.ConfigTypeString,
				Required:    true,
				Description: "API endpoint URL",
			},
			"timeout": {
				Type:        plugin.ConfigTypeInteger,
				Required:    false,
				Default:     30,
				Description: "Request timeout in seconds",
			},
			"verify_ssl": {
				Type:        plugin.ConfigTypeBoolean,
				Required:    false,
				Default:     true,
				Description: "Whether to verify SSL certificates",
			},
			"log_level": {
				Type:        plugin.ConfigTypeString,
				Required:    false,
				Default:     "info",
				Description: "Logging level",
				Enum:        []string{"debug", "info", "warn", "error"},
			},
		},
	}

	assert.Len(t, m.Config, 4)

	apiURL := m.Config["api_url"]
	assert.Equal(t, plugin.ConfigTypeString, apiURL.Type)
	assert.True(t, apiURL.Required)

	timeout := m.Config["timeout"]
	assert.Equal(t, plugin.ConfigTypeInteger, timeout.Type)
	assert.False(t, timeout.Required)
	assert.Equal(t, 30, timeout.Default)

	verifySSL := m.Config["verify_ssl"]
	assert.Equal(t, plugin.ConfigTypeBoolean, verifySSL.Type)
	assert.Equal(t, true, verifySSL.Default)

	logLevel := m.Config["log_level"]
	assert.Len(t, logLevel.Enum, 4)
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
		{name: "LGPL", license: "LGPL-2.1"},
		{name: "Proprietary", license: "Proprietary"},
		{name: "Unlicense", license: "Unlicense"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := plugin.Manifest{
				Name:    "test-plugin",
				Version: "1.0.0",
				License: tt.license,
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
		{name: "GitHub", homepage: "https://github.com/user/repo"},
		{name: "GitLab", homepage: "https://gitlab.com/user/repo"},
		{name: "Custom domain", homepage: "https://myplugin.example.com"},
		{name: "With path", homepage: "https://example.com/plugins/my-plugin"},
		{name: "HTTP", homepage: "http://insecure.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := plugin.Manifest{
				Name:     "test-plugin",
				Version:  "1.0.0",
				Homepage: tt.homepage,
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
		{name: "name only", author: "John Doe"},
		{name: "with email", author: "John Doe <john@example.com>"},
		{name: "email only", author: "<john@example.com>"},
		{name: "organization", author: "ACME Inc."},
		{name: "multiple authors", author: "John Doe, Jane Smith"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := plugin.Manifest{
				Name:    "test-plugin",
				Version: "1.0.0",
				Author:  tt.author,
			}
			assert.Equal(t, tt.author, m.Author)
		})
	}
}
