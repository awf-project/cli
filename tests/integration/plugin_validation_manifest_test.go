//go:build integration

// Feature: C031 - Manifest Validation
// Component: T009
// This file contains integration tests for plugin manifest validation.
// Tests the integration between manifest parsing and validation logic.

package integration_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/plugin"
	infrastructurePlugin "github.com/vanoix/awf/internal/infrastructure/plugin"
)

// =============================================================================
// HAPPY PATH TESTS - Valid Manifests
// =============================================================================

// TestManifestValidation_ValidSimple_Integration tests validation of a minimal valid manifest.
// Acceptance Criteria: Manifest.Validate() returns nil for valid simple manifests
func TestManifestValidation_ValidSimple_Integration(t *testing.T) {
	// Given: A simple valid plugin manifest fixture
	fixturesPath := "../fixtures/plugins"
	parser := infrastructurePlugin.NewManifestParser()
	loader := infrastructurePlugin.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pluginPath := filepath.Join(fixturesPath, "valid-simple")

	// When: Loading the plugin which triggers manifest parsing and validation
	pluginInfo, err := loader.LoadPlugin(ctx, pluginPath)

	// Then: No validation errors occur
	require.NoError(t, err, "valid simple manifest should pass validation")
	require.NotNil(t, pluginInfo)
	require.NotNil(t, pluginInfo.Manifest)

	// Verify manifest fields
	assert.Equal(t, "awf-plugin-simple", pluginInfo.Manifest.Name)
	assert.Equal(t, "1.0.0", pluginInfo.Manifest.Version)
	assert.Equal(t, ">=0.4.0", pluginInfo.Manifest.AWFVersion)
	assert.Contains(t, pluginInfo.Manifest.Capabilities, plugin.CapabilityOperations)
}

// TestManifestValidation_ValidFull_Integration tests validation of a complete manifest with all fields.
// Acceptance Criteria: Manifest.Validate() returns nil for manifests with all optional fields
func TestManifestValidation_ValidFull_Integration(t *testing.T) {
	// Given: A complete plugin manifest fixture with all fields
	fixturesPath := "../fixtures/plugins"
	parser := infrastructurePlugin.NewManifestParser()
	loader := infrastructurePlugin.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pluginPath := filepath.Join(fixturesPath, "valid-full")

	// When: Loading the plugin
	pluginInfo, err := loader.LoadPlugin(ctx, pluginPath)

	// Then: No validation errors occur
	require.NoError(t, err, "valid full manifest should pass validation")
	require.NotNil(t, pluginInfo)
	require.NotNil(t, pluginInfo.Manifest)

	// Verify all fields are present and valid
	assert.NotEmpty(t, pluginInfo.Manifest.Name)
	assert.NotEmpty(t, pluginInfo.Manifest.Version)
	assert.NotEmpty(t, pluginInfo.Manifest.AWFVersion)
	assert.NotEmpty(t, pluginInfo.Manifest.Description)
	assert.NotEmpty(t, pluginInfo.Manifest.Author)
	assert.NotEmpty(t, pluginInfo.Manifest.License)
}

// =============================================================================
// ERROR TESTS - Invalid Manifests
// =============================================================================

// TestManifestValidation_MissingName_Integration tests validation rejection for missing name.
// Acceptance Criteria: Manifest.Validate() returns descriptive error for empty name
func TestManifestValidation_MissingName_Integration(t *testing.T) {
	// Given: A plugin manifest fixture missing the name field
	fixturesPath := "../fixtures/plugins"
	parser := infrastructurePlugin.NewManifestParser()
	loader := infrastructurePlugin.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pluginPath := filepath.Join(fixturesPath, "invalid-missing-name")

	// When: Loading the plugin
	pluginInfo, err := loader.LoadPlugin(ctx, pluginPath)

	// Then: Validation error occurs mentioning "name"
	assert.Error(t, err, "manifest with missing name should fail validation")
	if err != nil {
		assert.Contains(t, err.Error(), "name", "error should mention missing name field")
	}
	if pluginInfo != nil {
		assert.Equal(t, plugin.StatusFailed, pluginInfo.Status)
	}
}

// TestManifestValidation_MissingVersion_Integration tests validation rejection for missing version.
// Acceptance Criteria: Manifest.Validate() returns descriptive error for empty version
func TestManifestValidation_MissingVersion_Integration(t *testing.T) {
	// Given: A plugin manifest fixture missing the version field
	fixturesPath := "../fixtures/plugins"
	parser := infrastructurePlugin.NewManifestParser()
	loader := infrastructurePlugin.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pluginPath := filepath.Join(fixturesPath, "invalid-missing-version")

	// When: Loading the plugin
	pluginInfo, err := loader.LoadPlugin(ctx, pluginPath)

	// Then: Validation error occurs mentioning "version"
	assert.Error(t, err, "manifest with missing version should fail validation")
	if err != nil {
		assert.Contains(t, err.Error(), "version", "error should mention missing version field")
	}
	if pluginInfo != nil {
		assert.Equal(t, plugin.StatusFailed, pluginInfo.Status)
	}
}

// TestManifestValidation_MissingAWFVersion_Integration tests validation rejection for missing awf_version.
// Acceptance Criteria: Manifest.Validate() returns descriptive error for empty awf_version
func TestManifestValidation_MissingAWFVersion_Integration(t *testing.T) {
	// Given: A plugin manifest fixture missing the awf_version field
	fixturesPath := "../fixtures/plugins"
	parser := infrastructurePlugin.NewManifestParser()
	loader := infrastructurePlugin.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pluginPath := filepath.Join(fixturesPath, "invalid-missing-awf-version")

	// When: Loading the plugin
	pluginInfo, err := loader.LoadPlugin(ctx, pluginPath)

	// Then: Validation error occurs mentioning "awf_version"
	assert.Error(t, err, "manifest with missing awf_version should fail validation")
	if err != nil {
		assert.Contains(t, err.Error(), "awf_version", "error should mention missing awf_version field")
	}
	if pluginInfo != nil {
		assert.Equal(t, plugin.StatusFailed, pluginInfo.Status)
	}
}

// TestManifestValidation_BadCapability_Integration tests validation rejection for invalid capability.
// Acceptance Criteria: Manifest.Validate() rejects unknown capability strings
func TestManifestValidation_BadCapability_Integration(t *testing.T) {
	// Given: A plugin manifest fixture with an invalid capability
	fixturesPath := "../fixtures/plugins"
	parser := infrastructurePlugin.NewManifestParser()
	loader := infrastructurePlugin.NewFileSystemLoader(parser)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pluginPath := filepath.Join(fixturesPath, "invalid-bad-capability")

	// When: Loading the plugin (parsing only, no validation)
	pluginInfo, err := loader.LoadPlugin(ctx, pluginPath)

	// Then: Loading succeeds but manifest validation should fail
	require.NoError(t, err, "loading should succeed")
	require.NotNil(t, pluginInfo)
	require.NotNil(t, pluginInfo.Manifest)

	// When: Explicitly validating the manifest
	validationErr := pluginInfo.Manifest.Validate()

	// Then: Validation error occurs mentioning "capability"
	require.Error(t, validationErr, "manifest with invalid capability should fail validation")
	assert.Contains(t, validationErr.Error(), "capability", "error should mention invalid capability")
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

// TestManifestValidation_EmptyCapabilities_Integration tests validation with empty capabilities list.
// Acceptance Criteria: Empty capabilities list is valid (plugins without capabilities are allowed)
func TestManifestValidation_EmptyCapabilities_Integration(t *testing.T) {
	// Given: A manifest with empty capabilities list
	manifest := &plugin.Manifest{
		Name:         "test-plugin",
		Version:      "1.0.0",
		AWFVersion:   ">=0.4.0",
		Description:  "Test plugin",
		Capabilities: []string{},
		Config:       nil,
	}

	// When: Validating the manifest
	err := manifest.Validate()

	// Then: No error occurs (empty capabilities is valid)
	assert.NoError(t, err, "empty capabilities list should be valid")
}

// TestManifestValidation_NilConfigMap_Integration tests validation with nil config map.
// Acceptance Criteria: Nil config map is valid (plugins without config are allowed)
func TestManifestValidation_NilConfigMap_Integration(t *testing.T) {
	// Given: A manifest with nil config map
	manifest := &plugin.Manifest{
		Name:         "test-plugin",
		Version:      "1.0.0",
		AWFVersion:   ">=0.4.0",
		Description:  "Test plugin",
		Capabilities: []string{plugin.CapabilityOperations},
		Config:       nil,
	}

	// When: Validating the manifest
	err := manifest.Validate()

	// Then: No error occurs (nil config is valid)
	assert.NoError(t, err, "nil config map should be valid")
}

// =============================================================================
// NAME VALIDATION TESTS
// =============================================================================

// TestManifestValidation_InvalidNamePattern_Integration tests rejection of names with invalid patterns.
// Acceptance Criteria: Manifest.Validate() rejects names not matching ^[a-z][a-z0-9-]*$
func TestManifestValidation_InvalidNamePattern_Integration(t *testing.T) {
	tests := []struct {
		name         string
		manifestName string
		wantErr      string
	}{
		{
			name:         "uppercase letters",
			manifestName: "MyPlugin",
			wantErr:      "invalid name",
		},
		{
			name:         "starts with digit",
			manifestName: "1plugin",
			wantErr:      "invalid name",
		},
		{
			name:         "contains underscore",
			manifestName: "my_plugin",
			wantErr:      "invalid name",
		},
		{
			name:         "contains space",
			manifestName: "my plugin",
			wantErr:      "invalid name",
		},
		{
			name:         "contains special chars",
			manifestName: "my@plugin",
			wantErr:      "invalid name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: A manifest with an invalid name pattern
			manifest := &plugin.Manifest{
				Name:       tt.manifestName,
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			}

			// When: Validating the manifest
			err := manifest.Validate()

			// Then: Validation error occurs
			require.Error(t, err, "invalid name pattern should fail validation")
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestManifestValidation_ValidNamePattern_Integration tests acceptance of valid name patterns.
// Acceptance Criteria: Manifest.Validate() accepts names matching ^[a-z][a-z0-9-]*$
func TestManifestValidation_ValidNamePattern_Integration(t *testing.T) {
	tests := []struct {
		name         string
		manifestName string
	}{
		{
			name:         "simple lowercase",
			manifestName: "plugin",
		},
		{
			name:         "with hyphens",
			manifestName: "awf-plugin-slack",
		},
		{
			name:         "with digits",
			manifestName: "plugin2",
		},
		{
			name:         "mixed alphanumeric and hyphens",
			manifestName: "my-plugin-v2",
		},
		{
			name:         "single letter",
			manifestName: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: A manifest with a valid name pattern
			manifest := &plugin.Manifest{
				Name:       tt.manifestName,
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
			}

			// When: Validating the manifest
			err := manifest.Validate()

			// Then: No validation error occurs
			assert.NoError(t, err, "valid name pattern should pass validation")
		})
	}
}

// =============================================================================
// CONFIG FIELD VALIDATION TESTS
// =============================================================================

// TestManifestValidation_InvalidConfigType_Integration tests rejection of invalid config field types.
// Acceptance Criteria: Manifest.Validate() rejects config fields with invalid types
func TestManifestValidation_InvalidConfigType_Integration(t *testing.T) {
	// Given: A manifest with an invalid config field type
	manifest := &plugin.Manifest{
		Name:       "test-plugin",
		Version:    "1.0.0",
		AWFVersion: ">=0.4.0",
		Config: map[string]plugin.ConfigField{
			"invalid_field": {
				Type:     "invalid_type",
				Required: false,
			},
		},
	}

	// When: Validating the manifest
	err := manifest.Validate()

	// Then: Validation error occurs mentioning config field and type
	require.Error(t, err, "invalid config type should fail validation")
	assert.Contains(t, err.Error(), "config field")
	assert.Contains(t, err.Error(), "invalid_field")
	assert.Contains(t, err.Error(), "invalid type")
}

// TestManifestValidation_EnumOnNonStringType_Integration tests rejection of enum on non-string types.
// Acceptance Criteria: Manifest.Validate() rejects enum constraint on non-string config fields
func TestManifestValidation_EnumOnNonStringType_Integration(t *testing.T) {
	tests := []struct {
		name       string
		configType string
	}{
		{
			name:       "integer with enum",
			configType: plugin.ConfigTypeInteger,
		},
		{
			name:       "boolean with enum",
			configType: plugin.ConfigTypeBoolean,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: A manifest with enum on non-string type
			manifest := &plugin.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]plugin.ConfigField{
					"field": {
						Type: tt.configType,
						Enum: []string{"value1", "value2"},
					},
				},
			}

			// When: Validating the manifest
			err := manifest.Validate()

			// Then: Validation error occurs mentioning enum restriction
			require.Error(t, err, "enum on non-string type should fail validation")
			assert.Contains(t, err.Error(), "enum")
		})
	}
}

// TestManifestValidation_ConfigDefaultTypeMismatch_Integration tests rejection of mismatched default types.
// Acceptance Criteria: Manifest.Validate() rejects config fields where default value doesn't match declared type
func TestManifestValidation_ConfigDefaultTypeMismatch_Integration(t *testing.T) {
	tests := []struct {
		name         string
		configType   string
		defaultValue any
		wantErr      string
	}{
		{
			name:         "string type with integer default",
			configType:   plugin.ConfigTypeString,
			defaultValue: 42,
			wantErr:      "type mismatch",
		},
		{
			name:         "integer type with string default",
			configType:   plugin.ConfigTypeInteger,
			defaultValue: "not a number",
			wantErr:      "type mismatch",
		},
		{
			name:         "boolean type with string default",
			configType:   plugin.ConfigTypeBoolean,
			defaultValue: "true",
			wantErr:      "type mismatch",
		},
		{
			name:         "integer type with boolean default",
			configType:   plugin.ConfigTypeInteger,
			defaultValue: true,
			wantErr:      "type mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: A manifest with mismatched default value type
			manifest := &plugin.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Config: map[string]plugin.ConfigField{
					"field": {
						Type:    tt.configType,
						Default: tt.defaultValue,
					},
				},
			}

			// When: Validating the manifest
			err := manifest.Validate()

			// Then: Validation error occurs mentioning type mismatch
			require.Error(t, err, "mismatched default type should fail validation")
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestManifestValidation_ValidConfigDefaults_Integration tests acceptance of valid default values.
// Acceptance Criteria: Manifest.Validate() accepts config fields with correctly typed defaults
func TestManifestValidation_ValidConfigDefaults_Integration(t *testing.T) {
	// Given: A manifest with correctly typed default values
	manifest := &plugin.Manifest{
		Name:       "test-plugin",
		Version:    "1.0.0",
		AWFVersion: ">=0.4.0",
		Config: map[string]plugin.ConfigField{
			"string_field": {
				Type:    plugin.ConfigTypeString,
				Default: "default value",
			},
			"integer_field": {
				Type:    plugin.ConfigTypeInteger,
				Default: 42,
			},
			"boolean_field": {
				Type:    plugin.ConfigTypeBoolean,
				Default: true,
			},
			"integer_from_json": {
				Type:    plugin.ConfigTypeInteger,
				Default: float64(10), // JSON unmarshaling produces float64
			},
		},
	}

	// When: Validating the manifest
	err := manifest.Validate()

	// Then: No validation error occurs
	assert.NoError(t, err, "valid config defaults should pass validation")
}

// =============================================================================
// CAPABILITIES VALIDATION TESTS
// =============================================================================

// TestManifestValidation_MultipleValidCapabilities_Integration tests validation with multiple valid capabilities.
// Acceptance Criteria: Manifest.Validate() accepts manifests with multiple valid capabilities
func TestManifestValidation_MultipleValidCapabilities_Integration(t *testing.T) {
	// Given: A manifest with all valid capabilities
	manifest := &plugin.Manifest{
		Name:       "test-plugin",
		Version:    "1.0.0",
		AWFVersion: ">=0.4.0",
		Capabilities: []string{
			plugin.CapabilityOperations,
			plugin.CapabilityCommands,
			plugin.CapabilityValidators,
		},
	}

	// When: Validating the manifest
	err := manifest.Validate()

	// Then: No validation error occurs
	assert.NoError(t, err, "multiple valid capabilities should pass validation")
}

// TestManifestValidation_DuplicateCapabilities_Integration tests validation with duplicate capabilities.
// Acceptance Criteria: Manifest.Validate() accepts duplicate capabilities (idempotent)
func TestManifestValidation_DuplicateCapabilities_Integration(t *testing.T) {
	// Given: A manifest with duplicate capabilities
	manifest := &plugin.Manifest{
		Name:       "test-plugin",
		Version:    "1.0.0",
		AWFVersion: ">=0.4.0",
		Capabilities: []string{
			plugin.CapabilityOperations,
			plugin.CapabilityOperations,
		},
	}

	// When: Validating the manifest
	err := manifest.Validate()

	// Then: No validation error occurs (duplicates are allowed)
	assert.NoError(t, err, "duplicate capabilities should be valid")
}

// =============================================================================
// INTEGRATION WORKFLOW TESTS
// =============================================================================

// TestManifestValidation_CompleteWorkflow_Integration tests the complete validation workflow.
// Acceptance Criteria: All validation methods work together correctly
func TestManifestValidation_CompleteWorkflow_Integration(t *testing.T) {
	// Given: A complete and valid manifest
	manifest := &plugin.Manifest{
		Name:        "awf-plugin-github",
		Version:     "2.1.0",
		Description: "GitHub integration for AWF",
		AWFVersion:  ">=0.4.0",
		Author:      "John Developer <john@example.com>",
		License:     "Apache-2.0",
		Homepage:    "https://github.com/example/awf-plugin-github",
		Capabilities: []string{
			plugin.CapabilityOperations,
			plugin.CapabilityCommands,
		},
		Config: map[string]plugin.ConfigField{
			"token": {
				Type:        plugin.ConfigTypeString,
				Required:    true,
				Description: "GitHub personal access token",
			},
			"api_url": {
				Type:        plugin.ConfigTypeString,
				Required:    false,
				Default:     "https://api.github.com",
				Description: "GitHub API base URL",
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
				Description: "Verify SSL certificates",
			},
			"log_level": {
				Type:        plugin.ConfigTypeString,
				Required:    false,
				Default:     "info",
				Description: "Logging verbosity",
				Enum:        []string{"debug", "info", "warn", "error"},
			},
		},
	}

	// When: Validating the complete manifest
	err := manifest.Validate()

	// Then: No validation errors occur
	assert.NoError(t, err, "complete valid manifest should pass validation")

	// Verify HasCapability works correctly
	assert.True(t, manifest.HasCapability(plugin.CapabilityOperations))
	assert.True(t, manifest.HasCapability(plugin.CapabilityCommands))
	assert.False(t, manifest.HasCapability(plugin.CapabilityValidators))
}

// TestManifestValidation_MultipleErrors_Integration tests error propagation with multiple invalid fields.
// Acceptance Criteria: Validation fails fast on first error encountered
func TestManifestValidation_MultipleErrors_Integration(t *testing.T) {
	// Given: A manifest with multiple validation errors
	manifest := &plugin.Manifest{
		Name:       "", // Empty name (first error)
		Version:    "", // Empty version (would be second error)
		AWFVersion: "", // Empty awf_version (would be third error)
	}

	// When: Validating the manifest
	err := manifest.Validate()

	// Then: Validation fails fast with first error
	require.Error(t, err, "manifest with multiple errors should fail validation")
	assert.Contains(t, err.Error(), "name", "should fail on first error: empty name")
}
