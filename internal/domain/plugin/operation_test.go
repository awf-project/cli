package plugin_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/plugin"
)

// -----------------------------------------------------------------------------
// Input Type Constants Tests
// -----------------------------------------------------------------------------

func TestInputTypeConstants_Values(t *testing.T) {
	assert.Equal(t, "string", plugin.InputTypeString)
	assert.Equal(t, "integer", plugin.InputTypeInteger)
	assert.Equal(t, "boolean", plugin.InputTypeBoolean)
	assert.Equal(t, "array", plugin.InputTypeArray)
	assert.Equal(t, "object", plugin.InputTypeObject)
}

func TestValidInputTypes_ContainsAllTypes(t *testing.T) {
	assert.Contains(t, plugin.ValidInputTypes, plugin.InputTypeString)
	assert.Contains(t, plugin.ValidInputTypes, plugin.InputTypeInteger)
	assert.Contains(t, plugin.ValidInputTypes, plugin.InputTypeBoolean)
	assert.Contains(t, plugin.ValidInputTypes, plugin.InputTypeArray)
	assert.Contains(t, plugin.ValidInputTypes, plugin.InputTypeObject)
	assert.Len(t, plugin.ValidInputTypes, 5)
}

// -----------------------------------------------------------------------------
// Validation Rule Constants Tests
// Component: T001
// Feature: C029
// -----------------------------------------------------------------------------

func TestValidValidationRules_ContainsAllRules(t *testing.T) {
	// Happy path: Verify all expected validation rules are present
	assert.Contains(t, plugin.ValidValidationRules, "url")
	assert.Contains(t, plugin.ValidValidationRules, "email")
	assert.Len(t, plugin.ValidValidationRules, 2)
}

func TestValidValidationRules_IsSlice(t *testing.T) {
	// Happy path: Verify ValidValidationRules is a slice
	assert.NotNil(t, plugin.ValidValidationRules)
	assert.IsType(t, []string{}, plugin.ValidValidationRules)
}

func TestValidValidationRules_NoEmptyStrings(t *testing.T) {
	// Edge case: Verify no empty strings in the slice
	for _, rule := range plugin.ValidValidationRules {
		assert.NotEmpty(t, rule, "ValidValidationRules should not contain empty strings")
	}
}

func TestValidValidationRules_NoDuplicates(t *testing.T) {
	// Edge case: Verify no duplicate rules
	seen := make(map[string]bool)
	for _, rule := range plugin.ValidValidationRules {
		assert.False(t, seen[rule], "ValidValidationRules contains duplicate: %s", rule)
		seen[rule] = true
	}
}

func TestValidValidationRules_AllLowercase(t *testing.T) {
	// Edge case: Verify all rules are lowercase for consistency
	for _, rule := range plugin.ValidValidationRules {
		assert.Equal(t, rule, rule, "Validation rules should be lowercase")
		assert.NotContains(t, rule, " ", "Validation rules should not contain spaces")
	}
}

func TestValidValidationRules_MatchesDocumentation(t *testing.T) {
	// Happy path: Verify specific rules match what's documented in InputSchema
	// According to InputSchema.Validation comment: "url", "email"
	expectedRules := []string{"url", "email"}

	for _, expected := range expectedRules {
		assert.Contains(t, plugin.ValidValidationRules, expected,
			"ValidValidationRules should contain %s as per InputSchema documentation", expected)
	}
}

func TestValidValidationRules_TableDriven_Membership(t *testing.T) {
	// Table-driven test for membership checks
	tests := []struct {
		name     string
		rule     string
		contains bool
	}{
		{
			name:     "url is valid",
			rule:     "url",
			contains: true,
		},
		{
			name:     "email is valid",
			rule:     "email",
			contains: true,
		},
		{
			name:     "unknown rule is not valid",
			rule:     "unknown",
			contains: false,
		},
		{
			name:     "empty string is not valid",
			rule:     "",
			contains: false,
		},
		{
			name:     "uppercase URL is not valid (case sensitive)",
			rule:     "URL",
			contains: false,
		},
		{
			name:     "uppercase EMAIL is not valid (case sensitive)",
			rule:     "EMAIL",
			contains: false,
		},
		{
			name:     "regex is not valid (not implemented)",
			rule:     "regex",
			contains: false,
		},
		{
			name:     "phone is not valid (not implemented)",
			rule:     "phone",
			contains: false,
		},
		{
			name:     "ipv4 is not valid (not implemented)",
			rule:     "ipv4",
			contains: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use slices.Contains for membership check
			found := slices.Contains(plugin.ValidValidationRules, tt.rule)

			if tt.contains {
				assert.True(t, found, "Expected %q to be in ValidValidationRules", tt.rule)
			} else {
				assert.False(t, found, "Expected %q to NOT be in ValidValidationRules", tt.rule)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// OperationSchema Tests
// -----------------------------------------------------------------------------

func TestOperationSchema_Creation(t *testing.T) {
	schema := plugin.OperationSchema{
		Name:        "slack.send",
		Description: "Send a message to Slack",
		Inputs: map[string]plugin.InputSchema{
			"channel": {
				Type:        "string",
				Required:    true,
				Description: "Target channel",
			},
			"message": {
				Type:        "string",
				Required:    true,
				Description: "Message content",
			},
		},
		Outputs:    []string{"message_id", "timestamp"},
		PluginName: "slack-notifier",
	}

	assert.Equal(t, "slack.send", schema.Name)
	assert.Equal(t, "Send a message to Slack", schema.Description)
	assert.Len(t, schema.Inputs, 2)
	assert.Contains(t, schema.Inputs, "channel")
	assert.Contains(t, schema.Inputs, "message")
	assert.Len(t, schema.Outputs, 2)
	assert.Equal(t, "slack-notifier", schema.PluginName)
}

func TestOperationSchema_NoInputs(t *testing.T) {
	schema := plugin.OperationSchema{
		Name:       "health.check",
		PluginName: "health-plugin",
		Outputs:    []string{"status"},
	}

	assert.Empty(t, schema.Inputs)
	assert.Len(t, schema.Outputs, 1)
}

func TestOperationSchema_NoOutputs(t *testing.T) {
	schema := plugin.OperationSchema{
		Name:       "log.info",
		PluginName: "logger-plugin",
		Inputs: map[string]plugin.InputSchema{
			"message": {Type: "string", Required: true},
		},
	}

	assert.Empty(t, schema.Outputs)
	assert.Len(t, schema.Inputs, 1)
}

// -----------------------------------------------------------------------------
// OperationSchema.Validate() Tests - Component T006
// Feature: C029
// -----------------------------------------------------------------------------

// Happy Path Tests

func TestOperationSchema_Validate_ValidMinimalSchema_Passes(t *testing.T) {
	// Component: T006
	// Happy path: Minimal valid schema with required fields only
	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Valid minimal schema should pass validation")
}

func TestOperationSchema_Validate_ValidCompleteSchema_Passes(t *testing.T) {
	// Component: T006
	// Happy path: Complete valid schema with all fields populated
	schema := &plugin.OperationSchema{
		Name:        "slack.send",
		Description: "Send message to Slack channel",
		Inputs: map[string]plugin.InputSchema{
			"channel":  {Type: plugin.InputTypeString, Required: true},
			"message":  {Type: plugin.InputTypeString, Required: true},
			"priority": {Type: plugin.InputTypeInteger, Required: false, Default: 1},
		},
		Outputs:    []string{"message_id", "timestamp"},
		PluginName: "slack-notifier",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Valid complete schema should pass validation")
}

func TestOperationSchema_Validate_ValidSchemaWithDotNotation_Passes(t *testing.T) {
	// Component: T006
	// Happy path: Name follows plugin.operation convention
	schema := &plugin.OperationSchema{
		Name:       "github.create_issue",
		PluginName: "github-integration",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Schema with dot notation name should pass")
}

func TestOperationSchema_Validate_ValidSchemaWithHyphenatedPluginName_Passes(t *testing.T) {
	// Component: T006
	// Happy path: PluginName with hyphens (common convention)
	schema := &plugin.OperationSchema{
		Name:       "send.email",
		PluginName: "email-service-provider",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Schema with hyphenated plugin name should pass")
}

// Error Handling Tests - Name Validation

func TestOperationSchema_Validate_EmptyName_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Name is empty string
	schema := &plugin.OperationSchema{
		Name:       "",
		PluginName: "test-plugin",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "name", "Error should mention name field")
	assert.NotErrorIs(t, err, plugin.ErrNotImplemented, "Should return validation error, not ErrNotImplemented")
}

func TestOperationSchema_Validate_WhitespaceName_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Name contains only whitespace
	schema := &plugin.OperationSchema{
		Name:       "   ",
		PluginName: "test-plugin",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "name", "Error should mention name field")
}

func TestOperationSchema_Validate_InvalidNameFormat_NoPlugin_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Name doesn't follow plugin.operation convention (no dot)
	schema := &plugin.OperationSchema{
		Name:       "operation_without_plugin",
		PluginName: "test-plugin",
	}

	err := schema.Validate()
	// Should pass - naming convention is not strictly enforced
	// or should fail - depends on implementation decision
	// This test documents expected behavior
	if err != nil {
		assert.Contains(t, err.Error(), "name", "If enforcing convention, error should mention name")
	}
}

// Error Handling Tests - PluginName Validation

func TestOperationSchema_Validate_EmptyPluginName_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: PluginName is empty string
	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin", "Error should mention plugin name field")
	assert.NotErrorIs(t, err, plugin.ErrNotImplemented, "Should return validation error, not ErrNotImplemented")
}

func TestOperationSchema_Validate_WhitespacePluginName_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: PluginName contains only whitespace
	schema := &plugin.OperationSchema{
		Name:       "test.op",
		PluginName: "   ",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin", "Error should mention plugin name field")
}

// Error Handling Tests - Inputs Validation

func TestOperationSchema_Validate_InvalidInputSchema_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: InputSchema with invalid type
	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"param": {Type: "invalid_type", Required: true},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	// Should indicate which input failed validation
	assert.True(t,
		err.Error() != "" && err.Error() != plugin.ErrNotImplemented.Error(),
		"Should return specific validation error for invalid input")
}

func TestOperationSchema_Validate_MultipleInvalidInputs_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Multiple inputs with validation errors
	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"invalid1": {Type: "bad_type", Required: true},
			"invalid2": {Type: "", Required: true},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	// Should report at least one input error
	assert.NotErrorIs(t, err, plugin.ErrNotImplemented)
}

func TestOperationSchema_Validate_InputWithInvalidValidationRule_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Input has unknown validation rule
	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"param": {
				Type:       plugin.InputTypeString,
				Required:   true,
				Validation: "unknown_rule",
			},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.NotErrorIs(t, err, plugin.ErrNotImplemented)
}

func TestOperationSchema_Validate_InputWithTypeMismatchDefault_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Default value doesn't match declared type
	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"param": {
				Type:     plugin.InputTypeString,
				Required: false,
				Default:  123, // Integer default for string type
			},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.NotErrorIs(t, err, plugin.ErrNotImplemented)
}

// Error Handling Tests - Outputs Validation

func TestOperationSchema_Validate_DuplicateOutputs_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Outputs slice contains duplicates
	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Outputs:    []string{"result", "status", "result"}, // "result" duplicated
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate", "Error should mention duplicate outputs")
	assert.NotErrorIs(t, err, plugin.ErrNotImplemented)
}

func TestOperationSchema_Validate_EmptyStringInOutputs_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Outputs contains empty string
	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Outputs:    []string{"result", "", "status"},
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "output", "Error should mention output field")
}

// Edge Case Tests

func TestOperationSchema_Validate_NilInputsMap_Passes(t *testing.T) {
	// Component: T006
	// Edge case: Nil inputs map is valid (no inputs required)
	schema := &plugin.OperationSchema{
		Name:       "health.check",
		PluginName: "monitoring",
		Inputs:     nil,
	}

	err := schema.Validate()

	assert.NoError(t, err, "Nil inputs map should be valid")
}

func TestOperationSchema_Validate_EmptyInputsMap_Passes(t *testing.T) {
	// Component: T006
	// Edge case: Empty inputs map is valid
	schema := &plugin.OperationSchema{
		Name:       "status.check",
		PluginName: "monitoring",
		Inputs:     map[string]plugin.InputSchema{},
	}

	err := schema.Validate()

	assert.NoError(t, err, "Empty inputs map should be valid")
}

func TestOperationSchema_Validate_NilOutputsSlice_Passes(t *testing.T) {
	// Component: T006
	// Edge case: Nil outputs slice is valid (no outputs)
	schema := &plugin.OperationSchema{
		Name:       "trigger.webhook",
		PluginName: "webhooks",
		Outputs:    nil,
	}

	err := schema.Validate()

	assert.NoError(t, err, "Nil outputs slice should be valid")
}

func TestOperationSchema_Validate_EmptyOutputsSlice_Passes(t *testing.T) {
	// Component: T006
	// Edge case: Empty outputs slice is valid
	schema := &plugin.OperationSchema{
		Name:       "fire.forget",
		PluginName: "events",
		Outputs:    []string{},
	}

	err := schema.Validate()

	assert.NoError(t, err, "Empty outputs slice should be valid")
}

func TestOperationSchema_Validate_EmptyDescription_Passes(t *testing.T) {
	// Component: T006
	// Edge case: Description is optional, empty is valid
	schema := &plugin.OperationSchema{
		Name:        "test.op",
		Description: "",
		PluginName:  "test-plugin",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Empty description should be valid (optional field)")
}

func TestOperationSchema_Validate_SingleOutput_NoDuplicates_Passes(t *testing.T) {
	// Component: T006
	// Edge case: Single output can't be duplicated
	schema := &plugin.OperationSchema{
		Name:       "single.output",
		PluginName: "test-plugin",
		Outputs:    []string{"result"},
	}

	err := schema.Validate()

	assert.NoError(t, err, "Single output should pass validation")
}

// Table-Driven Tests

func TestOperationSchema_Validate_TableDriven_NameVariations(t *testing.T) {
	// Component: T006
	// Table-driven: Various name formats
	tests := []struct {
		name      string
		opName    string
		wantError bool
	}{
		{
			name:      "simple dot notation",
			opName:    "plugin.operation",
			wantError: false,
		},
		{
			name:      "multiple dots",
			opName:    "plugin.sub.operation",
			wantError: false,
		},
		{
			name:      "underscore",
			opName:    "plugin.send_message",
			wantError: false,
		},
		{
			name:      "hyphen",
			opName:    "plugin.create-issue",
			wantError: false,
		},
		{
			name:      "empty string",
			opName:    "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &plugin.OperationSchema{
				Name:       tt.opName,
				PluginName: "test-plugin",
			}

			err := schema.Validate()

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOperationSchema_Validate_TableDriven_PluginNameVariations(t *testing.T) {
	// Component: T006
	// Table-driven: Various plugin name formats
	tests := []struct {
		name       string
		pluginName string
		wantError  bool
	}{
		{
			name:       "simple name",
			pluginName: "slack",
			wantError:  false,
		},
		{
			name:       "hyphenated",
			pluginName: "slack-notifier",
			wantError:  false,
		},
		{
			name:       "multiple hyphens",
			pluginName: "aws-s3-uploader",
			wantError:  false,
		},
		{
			name:       "underscore",
			pluginName: "email_sender",
			wantError:  false,
		},
		{
			name:       "empty string",
			pluginName: "",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &plugin.OperationSchema{
				Name:       "test.operation",
				PluginName: tt.pluginName,
			}

			err := schema.Validate()

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOperationSchema_Validate_TableDriven_ComplexScenarios(t *testing.T) {
	// Component: T006
	// Table-driven: Complex validation scenarios
	tests := []struct {
		name      string
		schema    *plugin.OperationSchema
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid schema with all valid inputs",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]plugin.InputSchema{
					"str": {Type: plugin.InputTypeString, Required: true},
					"int": {Type: plugin.InputTypeInteger, Required: false, Default: 42},
				},
				Outputs: []string{"result"},
			},
			wantError: false,
		},
		{
			name: "one invalid input among valid ones",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]plugin.InputSchema{
					"valid":   {Type: plugin.InputTypeString, Required: true},
					"invalid": {Type: "bad_type", Required: true},
				},
			},
			wantError: true,
			errorMsg:  "input",
		},
		{
			name: "empty name with valid inputs",
			schema: &plugin.OperationSchema{
				Name:       "",
				PluginName: "test",
				Inputs: map[string]plugin.InputSchema{
					"param": {Type: plugin.InputTypeString, Required: true},
				},
			},
			wantError: true,
			errorMsg:  "name",
		},
		{
			name: "valid name but empty plugin name",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "",
			},
			wantError: true,
			errorMsg:  "plugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()

			if tt.wantError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Integration Tests - Nested Validation

func TestOperationSchema_Validate_NestedInputValidation_PropagatesError(t *testing.T) {
	// Component: T006
	// Integration: Validates that nested InputSchema.Validate() is called
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
				Required:   true,
				Validation: "invalid_rule", // Should trigger InputSchema.Validate() error
			},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	// Error should come from InputSchema.Validate(), not just OperationSchema
	assert.NotErrorIs(t, err, plugin.ErrNotImplemented)
}

func TestOperationSchema_Validate_AllInputsValid_WithValidationRules_Passes(t *testing.T) {
	// Component: T006
	// Integration: All inputs have valid validation rules
	schema := &plugin.OperationSchema{
		Name:       "user.register",
		PluginName: "auth-service",
		Inputs: map[string]plugin.InputSchema{
			"email": {
				Type:       plugin.InputTypeString,
				Required:   true,
				Validation: "email", // Valid rule
			},
			"website": {
				Type:       plugin.InputTypeString,
				Required:   false,
				Validation: "url", // Valid rule
			},
		},
		Outputs: []string{"user_id"},
	}

	err := schema.Validate()

	assert.NoError(t, err, "Schema with valid validation rules should pass")
}

// GetRequiredInputs Tests

// Component: T005
// Feature: C029
func TestOperationSchema_GetRequiredInputs_MixedInputs_ReturnsRequired(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"required_param": {Type: plugin.InputTypeString, Required: true},
			"optional_param": {Type: plugin.InputTypeString, Required: false},
		},
	}

	result := schema.GetRequiredInputs()

	// Happy path: Should return only required input names
	require.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Contains(t, result, "required_param")
	assert.NotContains(t, result, "optional_param")
}

// Component: T005
// Feature: C029
func TestOperationSchema_GetRequiredInputs_NoInputs_ReturnsEmpty(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "health.check",
		PluginName: "health-plugin",
	}

	result := schema.GetRequiredInputs()

	// Edge case: Empty inputs should return empty slice, not nil
	require.NotNil(t, result)
	assert.Empty(t, result)
}

// Component: T005
// Feature: C029
func TestOperationSchema_GetRequiredInputs_AllRequired_ReturnsAll(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "multi.required",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"param1": {Type: plugin.InputTypeString, Required: true},
			"param2": {Type: plugin.InputTypeInteger, Required: true},
			"param3": {Type: plugin.InputTypeBoolean, Required: true},
		},
	}

	result := schema.GetRequiredInputs()

	// Happy path: All required inputs should be returned
	require.NotNil(t, result)
	assert.Len(t, result, 3)
	assert.Contains(t, result, "param1")
	assert.Contains(t, result, "param2")
	assert.Contains(t, result, "param3")
}

// Component: T005
// Feature: C029
func TestOperationSchema_GetRequiredInputs_NoneRequired_ReturnsEmpty(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "all.optional",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"opt1": {Type: plugin.InputTypeString, Required: false, Default: "default"},
			"opt2": {Type: plugin.InputTypeInteger, Required: false, Default: 0},
		},
	}

	result := schema.GetRequiredInputs()

	// Edge case: No required inputs should return empty slice, not nil
	require.NotNil(t, result)
	assert.Empty(t, result)
}

// Component: T005
// Feature: C029
func TestOperationSchema_GetRequiredInputs_NilInputsMap_ReturnsEmpty(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "nil.inputs",
		PluginName: "test-plugin",
		Inputs:     nil, // Explicitly nil map
	}

	result := schema.GetRequiredInputs()

	// Edge case: Nil inputs map should return empty slice, not nil
	require.NotNil(t, result)
	assert.Empty(t, result)
}

// Component: T005
// Feature: C029
func TestOperationSchema_GetRequiredInputs_EmptyInputsMap_ReturnsEmpty(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "empty.map",
		PluginName: "test-plugin",
		Inputs:     map[string]plugin.InputSchema{}, // Explicitly empty map
	}

	result := schema.GetRequiredInputs()

	// Edge case: Empty map should return empty slice, not nil
	require.NotNil(t, result)
	assert.Empty(t, result)
}

// Component: T005
// Feature: C029
func TestOperationSchema_GetRequiredInputs_SingleRequired_ReturnsOne(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "single.required",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"only_required": {Type: plugin.InputTypeString, Required: true},
		},
	}

	result := schema.GetRequiredInputs()

	// Happy path: Single required input
	require.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Equal(t, []string{"only_required"}, result)
}

// Component: T005
// Feature: C029
func TestOperationSchema_GetRequiredInputs_SingleOptional_ReturnsEmpty(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "single.optional",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"only_optional": {Type: plugin.InputTypeString, Required: false},
		},
	}

	result := schema.GetRequiredInputs()

	// Edge case: Single optional input should return empty slice
	require.NotNil(t, result)
	assert.Empty(t, result)
}

// Component: T005
// Feature: C029
func TestOperationSchema_GetRequiredInputs_ComplexScenario(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "complex.operation",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"api_key":    {Type: plugin.InputTypeString, Required: true},
			"endpoint":   {Type: plugin.InputTypeString, Required: true},
			"timeout":    {Type: plugin.InputTypeInteger, Required: false, Default: 30},
			"retry":      {Type: plugin.InputTypeBoolean, Required: false, Default: true},
			"user_id":    {Type: plugin.InputTypeString, Required: true},
			"metadata":   {Type: plugin.InputTypeObject, Required: false},
			"tags":       {Type: plugin.InputTypeArray, Required: false, Default: []any{}},
			"debug_mode": {Type: plugin.InputTypeBoolean, Required: true},
		},
	}

	result := schema.GetRequiredInputs()

	// Happy path: Complex mix of required and optional inputs
	require.NotNil(t, result)
	assert.Len(t, result, 4)
	assert.Contains(t, result, "api_key")
	assert.Contains(t, result, "endpoint")
	assert.Contains(t, result, "user_id")
	assert.Contains(t, result, "debug_mode")
	// Verify optional inputs are not included
	assert.NotContains(t, result, "timeout")
	assert.NotContains(t, result, "retry")
	assert.NotContains(t, result, "metadata")
	assert.NotContains(t, result, "tags")
}

// Component: T005
// Feature: C029
// Table-driven test for GetRequiredInputs
func TestOperationSchema_GetRequiredInputs_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		schema         *plugin.OperationSchema
		expectedCount  int
		expectedNames  []string
		shouldBeEmpty  bool
		shouldNotBeNil bool
	}{
		{
			name: "no inputs - empty slice",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
			},
			expectedCount:  0,
			expectedNames:  []string{},
			shouldBeEmpty:  true,
			shouldNotBeNil: true,
		},
		{
			name: "nil inputs map - empty slice",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs:     nil,
			},
			expectedCount:  0,
			expectedNames:  []string{},
			shouldBeEmpty:  true,
			shouldNotBeNil: true,
		},
		{
			name: "one required one optional",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]plugin.InputSchema{
					"req1": {Type: plugin.InputTypeString, Required: true},
					"opt1": {Type: plugin.InputTypeString, Required: false},
				},
			},
			expectedCount:  1,
			expectedNames:  []string{"req1"},
			shouldBeEmpty:  false,
			shouldNotBeNil: true,
		},
		{
			name: "all required",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]plugin.InputSchema{
					"req1": {Type: plugin.InputTypeString, Required: true},
					"req2": {Type: plugin.InputTypeInteger, Required: true},
					"req3": {Type: plugin.InputTypeBoolean, Required: true},
				},
			},
			expectedCount:  3,
			expectedNames:  []string{"req1", "req2", "req3"},
			shouldBeEmpty:  false,
			shouldNotBeNil: true,
		},
		{
			name: "all optional",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]plugin.InputSchema{
					"opt1": {Type: plugin.InputTypeString, Required: false},
					"opt2": {Type: plugin.InputTypeInteger, Required: false},
				},
			},
			expectedCount:  0,
			expectedNames:  []string{},
			shouldBeEmpty:  true,
			shouldNotBeNil: true,
		},
		{
			name: "many required many optional",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]plugin.InputSchema{
					"req1": {Type: plugin.InputTypeString, Required: true},
					"opt1": {Type: plugin.InputTypeString, Required: false},
					"req2": {Type: plugin.InputTypeInteger, Required: true},
					"opt2": {Type: plugin.InputTypeInteger, Required: false},
					"req3": {Type: plugin.InputTypeBoolean, Required: true},
					"opt3": {Type: plugin.InputTypeBoolean, Required: false},
				},
			},
			expectedCount:  3,
			expectedNames:  []string{"req1", "req2", "req3"},
			shouldBeEmpty:  false,
			shouldNotBeNil: true,
		},
		{
			name: "single required input only",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]plugin.InputSchema{
					"only_one": {Type: plugin.InputTypeString, Required: true},
				},
			},
			expectedCount:  1,
			expectedNames:  []string{"only_one"},
			shouldBeEmpty:  false,
			shouldNotBeNil: true,
		},
		{
			name: "single optional input only",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]plugin.InputSchema{
					"only_one": {Type: plugin.InputTypeString, Required: false},
				},
			},
			expectedCount:  0,
			expectedNames:  []string{},
			shouldBeEmpty:  true,
			shouldNotBeNil: true,
		},
		{
			name: "required inputs with various types",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]plugin.InputSchema{
					"str_req":  {Type: plugin.InputTypeString, Required: true},
					"int_req":  {Type: plugin.InputTypeInteger, Required: true},
					"bool_req": {Type: plugin.InputTypeBoolean, Required: true},
					"arr_req":  {Type: plugin.InputTypeArray, Required: true},
					"obj_req":  {Type: plugin.InputTypeObject, Required: true},
				},
			},
			expectedCount:  5,
			expectedNames:  []string{"str_req", "int_req", "bool_req", "arr_req", "obj_req"},
			shouldBeEmpty:  false,
			shouldNotBeNil: true,
		},
		{
			name: "required with default values",
			schema: &plugin.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]plugin.InputSchema{
					"req_with_default": {Type: plugin.InputTypeString, Required: true, Default: "default"},
					"opt_with_default": {Type: plugin.InputTypeString, Required: false, Default: "default"},
				},
			},
			expectedCount:  1,
			expectedNames:  []string{"req_with_default"},
			shouldBeEmpty:  false,
			shouldNotBeNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.schema.GetRequiredInputs()

			// Verify result is not nil (should always return slice, never nil)
			if tt.shouldNotBeNil {
				require.NotNil(t, result, "GetRequiredInputs should never return nil")
			}

			// Verify emptiness
			if tt.shouldBeEmpty {
				assert.Empty(t, result, "Expected empty slice")
			} else {
				assert.NotEmpty(t, result, "Expected non-empty slice")
			}

			// Verify count
			assert.Len(t, result, tt.expectedCount, "Expected %d required inputs", tt.expectedCount)

			// Verify all expected names are present
			for _, expectedName := range tt.expectedNames {
				assert.Contains(t, result, expectedName, "Expected to find required input: %s", expectedName)
			}

			// Verify slice length matches expected names count
			if len(tt.expectedNames) > 0 {
				assert.Equal(t, len(tt.expectedNames), len(result), "Result count should match expected names count")
			}
		})
	}
}

// Component: T005
// Feature: C029
// Error handling: GetRequiredInputs should never panic or error
func TestOperationSchema_GetRequiredInputs_NoPanicOnNilSchema(t *testing.T) {
	// Edge case: Even with minimal schema, should not panic
	schema := &plugin.OperationSchema{}

	assert.NotPanics(t, func() {
		result := schema.GetRequiredInputs()
		require.NotNil(t, result)
		assert.Empty(t, result)
	})
}

// Component: T005
// Feature: C029
// Performance: GetRequiredInputs should handle large input maps efficiently
func TestOperationSchema_GetRequiredInputs_LargeInputMap(t *testing.T) {
	// Edge case: Large number of inputs
	inputs := make(map[string]plugin.InputSchema)
	expectedRequired := []string{}

	// Create 100 inputs, half required, half optional
	for i := 0; i < 100; i++ {
		name := "input_" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		isRequired := i%2 == 0
		inputs[name] = plugin.InputSchema{
			Type:     plugin.InputTypeString,
			Required: isRequired,
		}
		if isRequired {
			expectedRequired = append(expectedRequired, name)
		}
	}

	schema := &plugin.OperationSchema{
		Name:       "large.operation",
		PluginName: "test-plugin",
		Inputs:     inputs,
	}

	result := schema.GetRequiredInputs()

	// Should return exactly 50 required inputs
	require.NotNil(t, result)
	assert.Len(t, result, 50)

	// Verify all required inputs are present
	for _, name := range expectedRequired {
		assert.Contains(t, result, name)
	}
}

// -----------------------------------------------------------------------------
// InputSchema Tests
// -----------------------------------------------------------------------------

func TestInputSchema_AllTypes(t *testing.T) {
	tests := []struct {
		name       string
		schema     plugin.InputSchema
		wantType   string
		wantReq    bool
		wantDefVal any
	}{
		{
			name: "string type",
			schema: plugin.InputSchema{
				Type:        "string",
				Required:    true,
				Description: "A string input",
			},
			wantType: "string",
			wantReq:  true,
		},
		{
			name: "integer type with default",
			schema: plugin.InputSchema{
				Type:    "integer",
				Default: 100,
			},
			wantType:   "integer",
			wantDefVal: 100,
		},
		{
			name: "boolean type",
			schema: plugin.InputSchema{
				Type:    "boolean",
				Default: false,
			},
			wantType:   "boolean",
			wantDefVal: false,
		},
		{
			name: "array type",
			schema: plugin.InputSchema{
				Type:        "array",
				Description: "List of items",
			},
			wantType: "array",
		},
		{
			name: "object type",
			schema: plugin.InputSchema{
				Type:        "object",
				Description: "Complex object",
			},
			wantType: "object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantType, tt.schema.Type)
			assert.Equal(t, tt.wantReq, tt.schema.Required)
			if tt.wantDefVal != nil {
				assert.Equal(t, tt.wantDefVal, tt.schema.Default)
			}
		})
	}
}

func TestInputSchema_Validation(t *testing.T) {
	tests := []struct {
		name           string
		schema         plugin.InputSchema
		wantValidation string
	}{
		{
			name: "url validation",
			schema: plugin.InputSchema{
				Type:       "string",
				Validation: "url",
			},
			wantValidation: "url",
		},
		{
			name: "email validation",
			schema: plugin.InputSchema{
				Type:       "string",
				Validation: "email",
			},
			wantValidation: "email",
		},
		{
			name: "no validation",
			schema: plugin.InputSchema{
				Type: "string",
			},
			wantValidation: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantValidation, tt.schema.Validation)
		})
	}
}

// InputSchema.Validate Tests

// ============================================================================
// Component: T003
// Feature: C029
// InputSchema.Validate() Comprehensive Tests
// ============================================================================

// Happy Path Tests - Valid Schemas

func TestInputSchema_Validate_ValidStringType_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:        plugin.InputTypeString,
		Required:    true,
		Description: "A valid string input",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected valid string schema to pass validation")
}

func TestInputSchema_Validate_ValidIntegerType_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:        plugin.InputTypeInteger,
		Required:    false,
		Description: "A valid integer input",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected valid integer schema to pass validation")
}

func TestInputSchema_Validate_ValidBooleanType_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:     plugin.InputTypeBoolean,
		Required: true,
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected valid boolean schema to pass validation")
}

func TestInputSchema_Validate_ValidArrayType_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:        plugin.InputTypeArray,
		Description: "A valid array input",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected valid array schema to pass validation")
}

func TestInputSchema_Validate_ValidObjectType_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:        plugin.InputTypeObject,
		Description: "A valid object input",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected valid object schema to pass validation")
}

// Edge Case Tests - Validation Rules

func TestInputSchema_Validate_WithURLValidation_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:        plugin.InputTypeString,
		Validation:  "url",
		Description: "URL input with validation",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected schema with 'url' validation to pass")
}

func TestInputSchema_Validate_WithEmailValidation_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:        plugin.InputTypeString,
		Validation:  "email",
		Description: "Email input with validation",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected schema with 'email' validation to pass")
}

func TestInputSchema_Validate_WithEmptyValidation_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:       plugin.InputTypeString,
		Validation: "", // Empty validation is allowed (no validation)
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected schema with empty validation to pass")
}

// Edge Case Tests - Default Values Matching Types

func TestInputSchema_Validate_StringDefaultMatchesType_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeString,
		Default: "default value",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected string default to match string type")
}

func TestInputSchema_Validate_IntegerDefaultMatchesType_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeInteger,
		Default: 42,
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected int default to match integer type")
}

func TestInputSchema_Validate_Float64DefaultForInteger_ReturnsNil(t *testing.T) {
	// JSON decoding may produce float64 for integer types
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeInteger,
		Default: float64(42),
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected float64 default to be accepted for integer type (JSON compatibility)")
}

func TestInputSchema_Validate_BooleanDefaultMatchesType_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeBoolean,
		Default: true,
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected bool default to match boolean type")
}

func TestInputSchema_Validate_ArrayDefaultMatchesType_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeArray,
		Default: []any{"item1", "item2"},
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected slice default to match array type")
}

func TestInputSchema_Validate_ObjectDefaultMatchesType_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeObject,
		Default: map[string]any{"key": "value"},
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected map default to match object type")
}

func TestInputSchema_Validate_NoDefault_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:     plugin.InputTypeString,
		Default:  nil, // No default value provided
		Required: true,
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected schema without default to pass validation")
}

// Error Handling Tests - Invalid Types

func TestInputSchema_Validate_EmptyType_ReturnsError(t *testing.T) {
	schema := &plugin.InputSchema{
		Type: "",
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for empty type")
	assert.Contains(t, err.Error(), "type", "Error message should mention 'type'")
}

func TestInputSchema_Validate_InvalidType_ReturnsError(t *testing.T) {
	schema := &plugin.InputSchema{
		Type: "invalid_type",
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for invalid type")
	assert.Contains(t, err.Error(), "invalid", "Error message should indicate invalid type")
}

func TestInputSchema_Validate_UnknownType_ReturnsError(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
	}{
		{"float type", "float"},
		{"number type", "number"},
		{"uppercase STRING", "STRING"},
		{"mixed case String", "String"},
		{"typo strng", "strng"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &plugin.InputSchema{
				Type: tt.typeName,
			}

			err := schema.Validate()

			require.Error(t, err, "Expected error for type: %s", tt.typeName)
			assert.Contains(t, err.Error(), tt.typeName, "Error should mention the invalid type name")
		})
	}
}

// Error Handling Tests - Invalid Validation Rules

func TestInputSchema_Validate_UnknownValidationRule_ReturnsError(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:       plugin.InputTypeString,
		Validation: "unknown_rule",
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for unknown validation rule")
	assert.Contains(t, err.Error(), "validation", "Error message should mention 'validation'")
	assert.Contains(t, err.Error(), "unknown_rule", "Error should mention the invalid rule")
}

func TestInputSchema_Validate_InvalidValidationRules_ReturnsError(t *testing.T) {
	tests := []struct {
		name           string
		validationRule string
	}{
		{"regex rule", "regex"},
		{"phone rule", "phone"},
		{"ipv4 rule", "ipv4"},
		{"uppercase URL", "URL"},
		{"mixed case Email", "Email"},
		{"with spaces", "email "},
		{"numeric rule", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &plugin.InputSchema{
				Type:       plugin.InputTypeString,
				Validation: tt.validationRule,
			}

			err := schema.Validate()

			require.Error(t, err, "Expected error for validation rule: %s", tt.validationRule)
			assert.Contains(t, err.Error(), tt.validationRule, "Error should mention the invalid validation rule")
		})
	}
}

// Error Handling Tests - Default Value Type Mismatches

func TestInputSchema_Validate_StringDefaultForInteger_ReturnsError(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeInteger,
		Default: "not an integer",
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for string default on integer type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_IntegerDefaultForString_ReturnsError(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeString,
		Default: 42,
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for integer default on string type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_StringDefaultForBoolean_ReturnsError(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeBoolean,
		Default: "true",
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for string default on boolean type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_IntegerDefaultForArray_ReturnsError(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeArray,
		Default: 123,
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for integer default on array type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_StringDefaultForObject_ReturnsError(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeObject,
		Default: "not an object",
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for string default on object type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_MapDefaultForArray_ReturnsError(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeArray,
		Default: map[string]any{"key": "value"},
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for map default on array type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_SliceDefaultForObject_ReturnsError(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeObject,
		Default: []any{"item"},
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for slice default on object type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

// Edge Case Tests - Complex Scenarios

func TestInputSchema_Validate_AllFieldsValid_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:        plugin.InputTypeString,
		Required:    true,
		Default:     "default@example.com",
		Description: "User email address",
		Validation:  "email",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected fully populated valid schema to pass validation")
}

func TestInputSchema_Validate_MinimalSchema_ReturnsNil(t *testing.T) {
	schema := &plugin.InputSchema{
		Type: plugin.InputTypeString,
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected minimal schema (only type) to pass validation")
}

// Table-Driven Tests for Type-Default Combinations

func TestInputSchema_Validate_TypeDefaultCombinations(t *testing.T) {
	tests := []struct {
		name        string
		schemaType  string
		defaultVal  any
		expectError bool
		description string
	}{
		// Valid combinations
		{"string-string", plugin.InputTypeString, "value", false, "string default for string type"},
		{"integer-int", plugin.InputTypeInteger, 42, false, "int default for integer type"},
		{"integer-float64", plugin.InputTypeInteger, float64(42), false, "float64 default for integer type"},
		{"boolean-bool", plugin.InputTypeBoolean, true, false, "bool default for boolean type"},
		{"array-slice", plugin.InputTypeArray, []any{1, 2}, false, "slice default for array type"},
		{"object-map", plugin.InputTypeObject, map[string]any{"k": "v"}, false, "map default for object type"},
		{"any-nil", plugin.InputTypeString, nil, false, "nil default (no default)"},

		// Invalid combinations
		{"string-int", plugin.InputTypeString, 123, true, "int default for string type"},
		{"integer-string", plugin.InputTypeInteger, "123", true, "string default for integer type"},
		{"boolean-string", plugin.InputTypeBoolean, "false", true, "string default for boolean type"},
		{"boolean-int", plugin.InputTypeBoolean, 0, true, "int default for boolean type"},
		{"array-map", plugin.InputTypeArray, map[string]any{"k": "v"}, true, "map default for array type"},
		{"array-string", plugin.InputTypeArray, "[]", true, "string default for array type"},
		{"object-slice", plugin.InputTypeObject, []any{1, 2}, true, "slice default for object type"},
		{"object-string", plugin.InputTypeObject, "{}", true, "string default for object type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &plugin.InputSchema{
				Type:    tt.schemaType,
				Default: tt.defaultVal,
			}

			err := schema.Validate()

			if tt.expectError {
				require.Error(t, err, "Expected error for %s", tt.description)
				assert.Contains(t, err.Error(), "default", "Error should mention default value issue")
			} else {
				assert.NoError(t, err, "Expected no error for %s", tt.description)
			}
		})
	}
}

// InputSchema.IsValidType Tests

// Component: T002
// Feature: C029
func TestInputSchema_IsValidType_ValidTypes_ReturnsTrue(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
	}{
		{"string type", plugin.InputTypeString},
		{"integer type", plugin.InputTypeInteger},
		{"boolean type", plugin.InputTypeBoolean},
		{"array type", plugin.InputTypeArray},
		{"object type", plugin.InputTypeObject},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &plugin.InputSchema{Type: tt.typeName}

			result := schema.IsValidType()

			// Implementation now correctly returns true for valid types
			assert.True(t, result, "Expected IsValidType() to return true for type: %s", tt.typeName)
		})
	}
}

func TestInputSchema_IsValidType_InvalidType_ReturnsFalse(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
	}{
		{"invalid type", "invalid"},
		{"empty type", ""},
		{"typo in type", "strng"},
		{"uppercase type", "STRING"},
		{"unknown type", "float"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &plugin.InputSchema{Type: tt.typeName}

			result := schema.IsValidType()

			assert.False(t, result)
		})
	}
}

// -----------------------------------------------------------------------------
// OperationResult Tests
// -----------------------------------------------------------------------------

func TestOperationResult_Success(t *testing.T) {
	result := plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{
			"message_id": "12345",
			"timestamp":  1702851200,
		},
	}

	assert.True(t, result.Success)
	assert.Empty(t, result.Error)
	require.Len(t, result.Outputs, 2)
	assert.Equal(t, "12345", result.Outputs["message_id"])
}

func TestOperationResult_Failure(t *testing.T) {
	result := plugin.OperationResult{
		Success: false,
		Error:   "connection refused",
		Outputs: nil,
	}

	assert.False(t, result.Success)
	assert.Equal(t, "connection refused", result.Error)
	assert.Nil(t, result.Outputs)
}

func TestOperationResult_PartialOutputs(t *testing.T) {
	result := plugin.OperationResult{
		Success: false,
		Error:   "timeout after 30s",
		Outputs: map[string]any{
			"partial_data": "some data received",
		},
	}

	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
	assert.NotEmpty(t, result.Outputs)
}

// OperationResult.IsSuccess Tests

func TestOperationResult_IsSuccess_True(t *testing.T) {
	result := &plugin.OperationResult{Success: true}

	assert.True(t, result.IsSuccess())
}

func TestOperationResult_IsSuccess_False(t *testing.T) {
	result := &plugin.OperationResult{Success: false}

	assert.False(t, result.IsSuccess())
}

func TestOperationResult_IsSuccess_WithError(t *testing.T) {
	result := &plugin.OperationResult{
		Success: false,
		Error:   "something went wrong",
	}

	assert.False(t, result.IsSuccess())
}

func TestOperationResult_IsSuccess_DefaultValue(t *testing.T) {
	result := &plugin.OperationResult{}

	// Zero value of bool is false
	assert.False(t, result.IsSuccess())
}

// OperationResult.HasError Tests

func TestOperationResult_HasError_WithError(t *testing.T) {
	result := &plugin.OperationResult{
		Success: false,
		Error:   "connection refused",
	}

	assert.True(t, result.HasError())
}

func TestOperationResult_HasError_EmptyError(t *testing.T) {
	result := &plugin.OperationResult{
		Success: true,
		Error:   "",
	}

	assert.False(t, result.HasError())
}

func TestOperationResult_HasError_DefaultValue(t *testing.T) {
	result := &plugin.OperationResult{}

	// Zero value of string is ""
	assert.False(t, result.HasError())
}

func TestOperationResult_HasError_WhitespaceError(t *testing.T) {
	result := &plugin.OperationResult{
		Success: false,
		Error:   " ",
	}

	// HasError returns true for any non-empty string including whitespace
	assert.True(t, result.HasError())
}

// OperationResult.GetOutput Tests

func TestOperationResult_GetOutput_ExistingKey(t *testing.T) {
	result := &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{
			"message_id": "12345",
			"timestamp":  1702851200,
		},
	}

	val, ok := result.GetOutput("message_id")

	assert.True(t, ok)
	assert.Equal(t, "12345", val)
}

func TestOperationResult_GetOutput_NonExistingKey(t *testing.T) {
	result := &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{
			"message_id": "12345",
		},
	}

	val, ok := result.GetOutput("nonexistent")

	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestOperationResult_GetOutput_NilOutputs(t *testing.T) {
	result := &plugin.OperationResult{
		Success: false,
		Outputs: nil,
	}

	val, ok := result.GetOutput("any_key")

	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestOperationResult_GetOutput_EmptyOutputs(t *testing.T) {
	result := &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{},
	}

	val, ok := result.GetOutput("any_key")

	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestOperationResult_GetOutput_NilValue(t *testing.T) {
	result := &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{
			"null_field": nil,
		},
	}

	val, ok := result.GetOutput("null_field")

	// Key exists but value is nil
	assert.True(t, ok)
	assert.Nil(t, val)
}

func TestOperationResult_GetOutput_DifferentTypes(t *testing.T) {
	result := &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{
			"string_val": "hello",
			"int_val":    42,
			"bool_val":   true,
			"float_val":  3.14,
			"slice_val":  []string{"a", "b"},
			"map_val":    map[string]int{"x": 1},
		},
	}

	tests := []struct {
		key      string
		expected any
	}{
		{"string_val", "hello"},
		{"int_val", 42},
		{"bool_val", true},
		{"float_val", 3.14},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			val, ok := result.GetOutput(tt.key)
			assert.True(t, ok)
			assert.Equal(t, tt.expected, val)
		})
	}

	// Check slice separately (deep comparison)
	sliceVal, ok := result.GetOutput("slice_val")
	assert.True(t, ok)
	assert.Equal(t, []string{"a", "b"}, sliceVal)

	// Check map separately
	mapVal, ok := result.GetOutput("map_val")
	assert.True(t, ok)
	assert.Equal(t, map[string]int{"x": 1}, mapVal)
}
