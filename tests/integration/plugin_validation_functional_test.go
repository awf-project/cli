//go:build integration

// Feature: C029 - Plugin Operation Validation
// This file contains functional/integration tests for plugin operation validation methods.

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/plugin"
)

// =============================================================================
// HAPPY PATH TESTS - Valid Schemas
// =============================================================================

// TestOperationSchemaValidate_ValidMinimal_Integration tests validation of a minimal valid operation schema.
// Acceptance Criteria: OperationSchema.Validate() returns nil for valid schemas
func TestOperationSchemaValidate_ValidMinimal_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs:     map[string]plugin.InputSchema{},
		Outputs:    []string{},
	}

	err := schema.Validate()

	assert.NoError(t, err, "minimal valid schema should pass validation")
}

// TestOperationSchemaValidate_ValidComplete_Integration tests validation of a complete operation schema.
// Acceptance Criteria: OperationSchema.Validate() validates all fields including nested InputSchemas
func TestOperationSchemaValidate_ValidComplete_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:        "slack.send",
		Description: "Send a message to Slack",
		PluginName:  "awf-plugin-slack",
		Inputs: map[string]plugin.InputSchema{
			"channel": {
				Type:        plugin.InputTypeString,
				Required:    true,
				Description: "Slack channel ID",
			},
			"message": {
				Type:        plugin.InputTypeString,
				Required:    true,
				Description: "Message content",
			},
			"webhook_url": {
				Type:        plugin.InputTypeString,
				Required:    false,
				Default:     "https://hooks.slack.com/default",
				Validation:  "url",
				Description: "Webhook URL",
			},
		},
		Outputs: []string{"message_id", "timestamp"},
	}

	err := schema.Validate()

	assert.NoError(t, err, "complete valid schema should pass validation")
}

// TestInputSchemaValidate_AllValidTypes_Integration tests validation of all valid input types.
// Acceptance Criteria: InputSchema.Validate() accepts all valid input types
func TestInputSchemaValidate_AllValidTypes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	validTypes := []string{
		plugin.InputTypeString,
		plugin.InputTypeInteger,
		plugin.InputTypeBoolean,
		plugin.InputTypeArray,
		plugin.InputTypeObject,
	}

	for _, validType := range validTypes {
		t.Run(validType, func(t *testing.T) {
			schema := plugin.InputSchema{
				Type:     validType,
				Required: true,
			}

			err := schema.Validate()

			assert.NoError(t, err, "type %q should be valid", validType)
		})
	}
}

// TestGetRequiredInputs_MixedRequiredOptional_Integration tests filtering required inputs.
// Acceptance Criteria: GetRequiredInputs() returns correct slice of required input names
func TestGetRequiredInputs_MixedRequiredOptional_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"required1": {Type: plugin.InputTypeString, Required: true},
			"optional1": {Type: plugin.InputTypeString, Required: false},
			"required2": {Type: plugin.InputTypeInteger, Required: true},
			"optional2": {Type: plugin.InputTypeBoolean, Required: false},
		},
	}

	required := schema.GetRequiredInputs()

	assert.Len(t, required, 2, "should return exactly 2 required inputs")
	assert.Contains(t, required, "required1")
	assert.Contains(t, required, "required2")
	assert.NotContains(t, required, "optional1")
	assert.NotContains(t, required, "optional2")
}

// TestIsValidType_ValidTypes_Integration tests type validation for valid types.
// Acceptance Criteria: IsValidType() returns true for all valid types
func TestIsValidType_ValidTypes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	validTypes := []string{
		plugin.InputTypeString,
		plugin.InputTypeInteger,
		plugin.InputTypeBoolean,
		plugin.InputTypeArray,
		plugin.InputTypeObject,
	}

	for _, validType := range validTypes {
		t.Run(validType, func(t *testing.T) {
			schema := plugin.InputSchema{Type: validType}

			result := schema.IsValidType()

			assert.True(t, result, "type %q should be valid", validType)
		})
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

// TestOperationSchemaValidate_EmptyInputsMap_Integration tests handling of empty inputs map.
// Acceptance Criteria: OperationSchema.Validate() handles empty inputs gracefully
func TestOperationSchemaValidate_EmptyInputsMap_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs:     map[string]plugin.InputSchema{},
		Outputs:    []string{"result"},
	}

	err := schema.Validate()

	assert.NoError(t, err, "empty inputs map should be valid")
}

// TestOperationSchemaValidate_NilInputsMap_Integration tests handling of nil inputs map.
// Acceptance Criteria: OperationSchema.Validate() handles nil inputs gracefully
func TestOperationSchemaValidate_NilInputsMap_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs:     nil,
		Outputs:    []string{"result"},
	}

	err := schema.Validate()

	assert.NoError(t, err, "nil inputs map should be valid")
}

// TestOperationSchemaValidate_EmptyOutputsList_Integration tests handling of empty outputs list.
// Acceptance Criteria: OperationSchema.Validate() handles empty outputs gracefully
func TestOperationSchemaValidate_EmptyOutputsList_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs:     map[string]plugin.InputSchema{},
		Outputs:    []string{},
	}

	err := schema.Validate()

	assert.NoError(t, err, "empty outputs list should be valid")
}

// TestGetRequiredInputs_EmptyInputs_Integration tests filtering with no inputs.
// Acceptance Criteria: GetRequiredInputs() returns empty slice for empty inputs
func TestGetRequiredInputs_EmptyInputs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs:     map[string]plugin.InputSchema{},
	}

	required := schema.GetRequiredInputs()

	assert.NotNil(t, required, "should return non-nil slice")
	assert.Empty(t, required, "should return empty slice for no inputs")
}

// TestGetRequiredInputs_NilInputs_Integration tests filtering with nil inputs.
// Acceptance Criteria: GetRequiredInputs() returns empty slice for nil inputs
func TestGetRequiredInputs_NilInputs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs:     nil,
	}

	required := schema.GetRequiredInputs()

	assert.NotNil(t, required, "should return non-nil slice")
	assert.Empty(t, required, "should return empty slice for nil inputs")
}

// TestInputSchemaValidate_UnicodeDescriptions_Integration tests handling of unicode in descriptions.
// Acceptance Criteria: InputSchema.Validate() handles unicode text correctly
func TestInputSchemaValidate_UnicodeDescriptions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := plugin.InputSchema{
		Type:        plugin.InputTypeString,
		Description: "Unicode test: こんにちは 世界 🌍 émojis 中文",
		Required:    true,
	}

	err := schema.Validate()

	assert.NoError(t, err, "unicode descriptions should be valid")
}

// TestInputSchemaValidate_DefaultValues_Integration tests various default value types.
// Acceptance Criteria: InputSchema.Validate() validates default value types match declared type
func TestInputSchemaValidate_DefaultValues_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name    string
		schema  plugin.InputSchema
		wantErr bool
	}{
		{
			name: "string default",
			schema: plugin.InputSchema{
				Type:    plugin.InputTypeString,
				Default: "test value",
			},
			wantErr: false,
		},
		{
			name: "integer default",
			schema: plugin.InputSchema{
				Type:    plugin.InputTypeInteger,
				Default: 42,
			},
			wantErr: false,
		},
		{
			name: "boolean default",
			schema: plugin.InputSchema{
				Type:    plugin.InputTypeBoolean,
				Default: true,
			},
			wantErr: false,
		},
		{
			name: "array default",
			schema: plugin.InputSchema{
				Type:    plugin.InputTypeArray,
				Default: []any{"item1", "item2"},
			},
			wantErr: false,
		},
		{
			name: "object default",
			schema: plugin.InputSchema{
				Type:    plugin.InputTypeObject,
				Default: map[string]any{"key": "value"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

// TestOperationSchemaValidate_EmptyName_Integration tests rejection of empty name.
// Acceptance Criteria: OperationSchema.Validate() rejects empty Name with descriptive error
func TestOperationSchemaValidate_EmptyName_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "",
		PluginName: "test-plugin",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "operation name cannot be empty")
}

// TestOperationSchemaValidate_WhitespaceName_Integration tests rejection of whitespace-only name.
// Acceptance Criteria: OperationSchema.Validate() rejects whitespace-only Name
func TestOperationSchemaValidate_WhitespaceName_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "   \t\n",
		PluginName: "test-plugin",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "operation name cannot be empty")
}

// TestOperationSchemaValidate_EmptyPluginName_Integration tests rejection of empty plugin name.
// Acceptance Criteria: OperationSchema.Validate() rejects empty PluginName
func TestOperationSchemaValidate_EmptyPluginName_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin name cannot be empty")
}

// TestOperationSchemaValidate_InvalidInputSchema_Integration tests rejection of invalid input schemas.
// Acceptance Criteria: OperationSchema.Validate() validates nested InputSchemas
func TestOperationSchemaValidate_InvalidInputSchema_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"invalid_input": {
				Type:     "invalid_type",
				Required: true,
			},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input schema")
	assert.Contains(t, err.Error(), "invalid_input")
}

// TestOperationSchemaValidate_DuplicateOutputs_Integration tests rejection of duplicate output names.
// Acceptance Criteria: OperationSchema.Validate() rejects duplicate output names
func TestOperationSchemaValidate_DuplicateOutputs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Outputs:    []string{"result", "status", "result"},
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate output name")
	assert.Contains(t, err.Error(), "result")
}

// TestOperationSchemaValidate_EmptyOutputName_Integration tests rejection of empty output names.
// Acceptance Criteria: OperationSchema.Validate() rejects empty output names
func TestOperationSchemaValidate_EmptyOutputName_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Outputs:    []string{"result", "", "status"},
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "output name cannot be empty")
}

// TestInputSchemaValidate_EmptyType_Integration tests rejection of empty type.
// Acceptance Criteria: InputSchema.Validate() rejects empty Type
func TestInputSchemaValidate_EmptyType_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := plugin.InputSchema{
		Type:     "",
		Required: true,
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "type cannot be empty")
}

// TestInputSchemaValidate_InvalidType_Integration tests rejection of invalid types.
// Acceptance Criteria: InputSchema.Validate() rejects invalid types with descriptive error
func TestInputSchemaValidate_InvalidType_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	invalidTypes := []string{"invalid", "float", "number", "text", "null"}

	for _, invalidType := range invalidTypes {
		t.Run(invalidType, func(t *testing.T) {
			schema := plugin.InputSchema{
				Type:     invalidType,
				Required: true,
			}

			err := schema.Validate()

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid input schema type")
			assert.Contains(t, err.Error(), invalidType)
		})
	}
}

// TestInputSchemaValidate_InvalidValidationRule_Integration tests rejection of unknown validation rules.
// Acceptance Criteria: InputSchema.Validate() rejects unknown validation rules
func TestInputSchemaValidate_InvalidValidationRule_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := plugin.InputSchema{
		Type:       plugin.InputTypeString,
		Validation: "unknown_rule",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid validation rule")
	assert.Contains(t, err.Error(), "unknown_rule")
}

// TestInputSchemaValidate_ValidValidationRules_Integration tests acceptance of valid validation rules.
// Acceptance Criteria: InputSchema.Validate() accepts recognized validation rules
func TestInputSchemaValidate_ValidValidationRules_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	validRules := []string{"url", "email"}

	for _, rule := range validRules {
		t.Run(rule, func(t *testing.T) {
			schema := plugin.InputSchema{
				Type:       plugin.InputTypeString,
				Validation: rule,
			}

			err := schema.Validate()

			assert.NoError(t, err, "validation rule %q should be valid", rule)
		})
	}
}

// TestInputSchemaValidate_DefaultTypeMismatch_Integration tests rejection of mismatched default values.
// Acceptance Criteria: InputSchema.Validate() rejects default values that don't match declared type
func TestInputSchemaValidate_DefaultTypeMismatch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name    string
		schema  plugin.InputSchema
		wantErr string
	}{
		{
			name: "string type with integer default",
			schema: plugin.InputSchema{
				Type:    plugin.InputTypeString,
				Default: 42,
			},
			wantErr: "default value type mismatch",
		},
		{
			name: "integer type with string default",
			schema: plugin.InputSchema{
				Type:    plugin.InputTypeInteger,
				Default: "not a number",
			},
			wantErr: "default value type mismatch",
		},
		{
			name: "boolean type with string default",
			schema: plugin.InputSchema{
				Type:    plugin.InputTypeBoolean,
				Default: "true",
			},
			wantErr: "default value type mismatch",
		},
		{
			name: "array type with non-array default",
			schema: plugin.InputSchema{
				Type:    plugin.InputTypeArray,
				Default: "not an array",
			},
			wantErr: "default value type mismatch",
		},
		{
			name: "object type with non-map default",
			schema: plugin.InputSchema{
				Type:    plugin.InputTypeObject,
				Default: "not an object",
			},
			wantErr: "default value type mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestIsValidType_InvalidTypes_Integration tests type validation for invalid types.
// Acceptance Criteria: IsValidType() returns false for invalid types
func TestIsValidType_InvalidTypes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	invalidTypes := []string{"", "invalid", "float", "number", "null", "undefined"}

	for _, invalidType := range invalidTypes {
		t.Run(invalidType, func(t *testing.T) {
			schema := plugin.InputSchema{Type: invalidType}

			result := schema.IsValidType()

			assert.False(t, result, "type %q should be invalid", invalidType)
		})
	}
}

// =============================================================================
// INTEGRATION TESTS - Multiple Components Working Together
// =============================================================================

// TestOperationSchemaFullWorkflow_Integration tests complete validation workflow.
// Acceptance Criteria: All validation methods work together correctly
func TestOperationSchemaFullWorkflow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create a complete operation schema
	schema := &plugin.OperationSchema{
		Name:        "github.create_issue",
		Description: "Create a GitHub issue",
		PluginName:  "awf-plugin-github",
		Inputs: map[string]plugin.InputSchema{
			"repository": {
				Type:        plugin.InputTypeString,
				Required:    true,
				Description: "Repository name (owner/repo)",
			},
			"title": {
				Type:        plugin.InputTypeString,
				Required:    true,
				Description: "Issue title",
			},
			"body": {
				Type:        plugin.InputTypeString,
				Required:    false,
				Default:     "",
				Description: "Issue body",
			},
			"labels": {
				Type:        plugin.InputTypeArray,
				Required:    false,
				Default:     []any{},
				Description: "Issue labels",
			},
			"assignees": {
				Type:        plugin.InputTypeArray,
				Required:    false,
				Description: "Issue assignees",
			},
		},
		Outputs: []string{"issue_number", "issue_url", "created_at"},
	}

	// Step 1: Validate the entire schema
	err := schema.Validate()
	require.NoError(t, err, "schema validation should pass")

	// Step 2: Get required inputs
	requiredInputs := schema.GetRequiredInputs()
	assert.Len(t, requiredInputs, 2, "should have 2 required inputs")
	assert.Contains(t, requiredInputs, "repository")
	assert.Contains(t, requiredInputs, "title")
	assert.NotContains(t, requiredInputs, "body")

	// Step 3: Validate individual input types
	for name, inputSchema := range schema.Inputs {
		assert.True(t, inputSchema.IsValidType(), "input %q should have valid type", name)
	}

	// Step 4: Verify each input schema is valid
	for name, inputSchema := range schema.Inputs {
		err := inputSchema.Validate()
		assert.NoError(t, err, "input schema %q should be valid", name)
	}
}

// TestOperationSchemaComplexValidation_Integration tests validation with complex nested schemas.
// Acceptance Criteria: Validation correctly handles complex, deeply nested structures
func TestOperationSchemaComplexValidation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:        "database.query",
		Description: "Execute a database query",
		PluginName:  "awf-plugin-database",
		Inputs: map[string]plugin.InputSchema{
			"connection": {
				Type:        plugin.InputTypeObject,
				Required:    true,
				Description: "Database connection details",
				Default: map[string]any{
					"host":    "localhost",
					"port":    5432,
					"timeout": 30,
				},
			},
			"query": {
				Type:        plugin.InputTypeString,
				Required:    true,
				Description: "SQL query to execute",
			},
			"parameters": {
				Type:        plugin.InputTypeArray,
				Required:    false,
				Default:     []any{},
				Description: "Query parameters",
			},
			"timeout": {
				Type:        plugin.InputTypeInteger,
				Required:    false,
				Default:     30,
				Description: "Query timeout in seconds",
			},
			"fetch_metadata": {
				Type:        plugin.InputTypeBoolean,
				Required:    false,
				Default:     false,
				Description: "Whether to fetch result metadata",
			},
		},
		Outputs: []string{"rows", "affected_rows", "execution_time"},
	}

	// Validate entire schema
	err := schema.Validate()
	assert.NoError(t, err, "complex schema should validate successfully")

	// Verify required inputs filtering works correctly
	requiredInputs := schema.GetRequiredInputs()
	assert.Len(t, requiredInputs, 2)
	assert.Contains(t, requiredInputs, "connection")
	assert.Contains(t, requiredInputs, "query")
}

// TestOperationSchemaErrorPropagation_Integration tests error propagation from nested validations.
// Acceptance Criteria: Errors from nested validations propagate correctly with context
func TestOperationSchemaErrorPropagation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"valid_input": {
				Type:     plugin.InputTypeString,
				Required: true,
			},
			"invalid_input": {
				Type:       plugin.InputTypeString,
				Validation: "invalid_rule",
			},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input schema")
	assert.Contains(t, err.Error(), "invalid_input")
	assert.Contains(t, err.Error(), "invalid validation rule")
}
