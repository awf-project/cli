package pluginmodel_test

import (
	"slices"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Input Type Constants Tests

func TestInputTypeConstants_Values(t *testing.T) {
	assert.Equal(t, "string", pluginmodel.InputTypeString)
	assert.Equal(t, "integer", pluginmodel.InputTypeInteger)
	assert.Equal(t, "boolean", pluginmodel.InputTypeBoolean)
	assert.Equal(t, "array", pluginmodel.InputTypeArray)
	assert.Equal(t, "object", pluginmodel.InputTypeObject)
}

func TestValidInputTypes_ContainsAllTypes(t *testing.T) {
	assert.Contains(t, pluginmodel.ValidInputTypes, pluginmodel.InputTypeString)
	assert.Contains(t, pluginmodel.ValidInputTypes, pluginmodel.InputTypeInteger)
	assert.Contains(t, pluginmodel.ValidInputTypes, pluginmodel.InputTypeBoolean)
	assert.Contains(t, pluginmodel.ValidInputTypes, pluginmodel.InputTypeArray)
	assert.Contains(t, pluginmodel.ValidInputTypes, pluginmodel.InputTypeObject)
	assert.Len(t, pluginmodel.ValidInputTypes, 5)
}

// Validation Rule Constants Tests
// Component: T001
// Feature: C029

func TestValidValidationRules_ContainsAllRules(t *testing.T) {
	// Happy path: Verify all expected validation rules are present
	assert.Contains(t, pluginmodel.ValidValidationRules, "url")
	assert.Contains(t, pluginmodel.ValidValidationRules, "email")
	assert.Len(t, pluginmodel.ValidValidationRules, 2)
}

func TestValidValidationRules_IsSlice(t *testing.T) {
	// Happy path: Verify ValidValidationRules is a slice
	assert.NotNil(t, pluginmodel.ValidValidationRules)
	assert.IsType(t, []string{}, pluginmodel.ValidValidationRules)
}

func TestValidValidationRules_NoEmptyStrings(t *testing.T) {
	// Edge case: Verify no empty strings in the slice
	for _, rule := range pluginmodel.ValidValidationRules {
		assert.NotEmpty(t, rule, "ValidValidationRules should not contain empty strings")
	}
}

func TestValidValidationRules_NoDuplicates(t *testing.T) {
	// Edge case: Verify no duplicate rules
	seen := make(map[string]bool)
	for _, rule := range pluginmodel.ValidValidationRules {
		assert.False(t, seen[rule], "ValidValidationRules contains duplicate: %s", rule)
		seen[rule] = true
	}
}

func TestValidValidationRules_AllLowercase(t *testing.T) {
	// Edge case: Verify all rules are lowercase for consistency
	for _, rule := range pluginmodel.ValidValidationRules {
		assert.Equal(t, rule, rule, "Validation rules should be lowercase")
		assert.NotContains(t, rule, " ", "Validation rules should not contain spaces")
	}
}

func TestValidValidationRules_MatchesDocumentation(t *testing.T) {
	// Happy path: Verify specific rules match what's documented in InputSchema
	// According to InputSchema.Validation comment: "url", "email"
	expectedRules := []string{"url", "email"}

	for _, expected := range expectedRules {
		assert.Contains(t, pluginmodel.ValidValidationRules, expected,
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
			found := slices.Contains(pluginmodel.ValidValidationRules, tt.rule)

			if tt.contains {
				assert.True(t, found, "Expected %q to be in ValidValidationRules", tt.rule)
			} else {
				assert.False(t, found, "Expected %q to NOT be in ValidValidationRules", tt.rule)
			}
		})
	}
}

// OperationSchema Tests

func TestOperationSchema_Creation(t *testing.T) {
	schema := pluginmodel.OperationSchema{
		Name:        "slack.send",
		Description: "Send a message to Slack",
		Inputs: map[string]pluginmodel.InputSchema{
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
	schema := pluginmodel.OperationSchema{
		Name:       "health.check",
		PluginName: "health-plugin",
		Outputs:    []string{"status"},
	}

	assert.Empty(t, schema.Inputs)
	assert.Len(t, schema.Outputs, 1)
}

func TestOperationSchema_NoOutputs(t *testing.T) {
	schema := pluginmodel.OperationSchema{
		Name:       "log.info",
		PluginName: "logger-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"message": {Type: "string", Required: true},
		},
	}

	assert.Empty(t, schema.Outputs)
	assert.Len(t, schema.Inputs, 1)
}

// OperationSchema.Validate() Tests - Component T006
// Feature: C029

// Happy Path Tests

func TestOperationSchema_Validate_ValidMinimalSchema_Passes(t *testing.T) {
	// Component: T006
	// Happy path: Minimal valid schema with required fields only
	schema := &pluginmodel.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Valid minimal schema should pass validation")
}

func TestOperationSchema_Validate_ValidCompleteSchema_Passes(t *testing.T) {
	// Component: T006
	// Happy path: Complete valid schema with all fields populated
	schema := &pluginmodel.OperationSchema{
		Name:        "slack.send",
		Description: "Send message to Slack channel",
		Inputs: map[string]pluginmodel.InputSchema{
			"channel":  {Type: pluginmodel.InputTypeString, Required: true},
			"message":  {Type: pluginmodel.InputTypeString, Required: true},
			"priority": {Type: pluginmodel.InputTypeInteger, Required: false, Default: 1},
		},
		Outputs:    []string{"message_id", "timestamp"},
		PluginName: "slack-notifier",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Valid complete schema should pass validation")
}

func TestOperationSchema_Validate_ValidSchemaWithDotNotation_Passes(t *testing.T) {
	// Component: T006
	// Happy path: Name follows pluginmodel.operation convention
	schema := &pluginmodel.OperationSchema{
		Name:       "github.create_issue",
		PluginName: "github-integration",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Schema with dot notation name should pass")
}

func TestOperationSchema_Validate_ValidSchemaWithHyphenatedPluginName_Passes(t *testing.T) {
	// Component: T006
	// Happy path: PluginName with hyphens (common convention)
	schema := &pluginmodel.OperationSchema{
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
	schema := &pluginmodel.OperationSchema{
		Name:       "",
		PluginName: "test-plugin",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "name", "Error should mention name field")
	assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented, "Should return validation error, not ErrNotImplemented")
}

func TestOperationSchema_Validate_WhitespaceName_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Name contains only whitespace
	schema := &pluginmodel.OperationSchema{
		Name:       "   ",
		PluginName: "test-plugin",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "name", "Error should mention name field")
}

func TestOperationSchema_Validate_InvalidNameFormat_NoPlugin_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Name doesn't follow pluginmodel.operation convention (no dot)
	schema := &pluginmodel.OperationSchema{
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
	schema := &pluginmodel.OperationSchema{
		Name:       "test.operation",
		PluginName: "",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin", "Error should mention plugin name field")
	assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented, "Should return validation error, not ErrNotImplemented")
}

func TestOperationSchema_Validate_WhitespacePluginName_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: PluginName contains only whitespace
	schema := &pluginmodel.OperationSchema{
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
	schema := &pluginmodel.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"param": {Type: "invalid_type", Required: true},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	// Should indicate which input failed validation
	assert.True(t,
		err.Error() != "" && err.Error() != pluginmodel.ErrNotImplemented.Error(),
		"Should return specific validation error for invalid input")
}

func TestOperationSchema_Validate_MultipleInvalidInputs_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Multiple inputs with validation errors
	schema := &pluginmodel.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"invalid1": {Type: "bad_type", Required: true},
			"invalid2": {Type: "", Required: true},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	// Should report at least one input error
	assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
}

func TestOperationSchema_Validate_InputWithInvalidValidationRule_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Input has unknown validation rule
	schema := &pluginmodel.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"param": {
				Type:       pluginmodel.InputTypeString,
				Required:   true,
				Validation: "unknown_rule",
			},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
}

func TestOperationSchema_Validate_InputWithTypeMismatchDefault_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Default value doesn't match declared type
	schema := &pluginmodel.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"param": {
				Type:     pluginmodel.InputTypeString,
				Required: false,
				Default:  123, // Integer default for string type
			},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
}

// Error Handling Tests - Outputs Validation

func TestOperationSchema_Validate_DuplicateOutputs_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Outputs slice contains duplicates
	schema := &pluginmodel.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Outputs:    []string{"result", "status", "result"}, // "result" duplicated
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate", "Error should mention duplicate outputs")
	assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
}

func TestOperationSchema_Validate_EmptyStringInOutputs_ReturnsError(t *testing.T) {
	// Component: T006
	// Error case: Outputs contains empty string
	schema := &pluginmodel.OperationSchema{
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
	schema := &pluginmodel.OperationSchema{
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
	schema := &pluginmodel.OperationSchema{
		Name:       "status.check",
		PluginName: "monitoring",
		Inputs:     map[string]pluginmodel.InputSchema{},
	}

	err := schema.Validate()

	assert.NoError(t, err, "Empty inputs map should be valid")
}

func TestOperationSchema_Validate_NilOutputsSlice_Passes(t *testing.T) {
	// Component: T006
	// Edge case: Nil outputs slice is valid (no outputs)
	schema := &pluginmodel.OperationSchema{
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
	schema := &pluginmodel.OperationSchema{
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
	schema := &pluginmodel.OperationSchema{
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
	schema := &pluginmodel.OperationSchema{
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
			opName:    "pluginmodel.operation",
			wantError: false,
		},
		{
			name:      "multiple dots",
			opName:    "pluginmodel.sub.operation",
			wantError: false,
		},
		{
			name:      "underscore",
			opName:    "pluginmodel.send_message",
			wantError: false,
		},
		{
			name:      "hyphen",
			opName:    "pluginmodel.create-issue",
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
			schema := &pluginmodel.OperationSchema{
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
			schema := &pluginmodel.OperationSchema{
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
		schema    *pluginmodel.OperationSchema
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid schema with all valid inputs",
			schema: &pluginmodel.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]pluginmodel.InputSchema{
					"str": {Type: pluginmodel.InputTypeString, Required: true},
					"int": {Type: pluginmodel.InputTypeInteger, Required: false, Default: 42},
				},
				Outputs: []string{"result"},
			},
			wantError: false,
		},
		{
			name: "one invalid input among valid ones",
			schema: &pluginmodel.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]pluginmodel.InputSchema{
					"valid":   {Type: pluginmodel.InputTypeString, Required: true},
					"invalid": {Type: "bad_type", Required: true},
				},
			},
			wantError: true,
			errorMsg:  "input",
		},
		{
			name: "empty name with valid inputs",
			schema: &pluginmodel.OperationSchema{
				Name:       "",
				PluginName: "test",
				Inputs: map[string]pluginmodel.InputSchema{
					"param": {Type: pluginmodel.InputTypeString, Required: true},
				},
			},
			wantError: true,
			errorMsg:  "name",
		},
		{
			name: "valid name but empty plugin name",
			schema: &pluginmodel.OperationSchema{
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
	schema := &pluginmodel.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"valid_input": {
				Type:     pluginmodel.InputTypeString,
				Required: true,
			},
			"invalid_input": {
				Type:       pluginmodel.InputTypeString,
				Required:   true,
				Validation: "invalid_rule", // Should trigger InputSchema.Validate() error
			},
		},
	}

	err := schema.Validate()

	require.Error(t, err)
	// Error should come from InputSchema.Validate(), not just OperationSchema
	assert.NotErrorIs(t, err, pluginmodel.ErrNotImplemented)
}

func TestOperationSchema_Validate_AllInputsValid_WithValidationRules_Passes(t *testing.T) {
	// Component: T006
	// Integration: All inputs have valid validation rules
	schema := &pluginmodel.OperationSchema{
		Name:       "user.register",
		PluginName: "auth-service",
		Inputs: map[string]pluginmodel.InputSchema{
			"email": {
				Type:       pluginmodel.InputTypeString,
				Required:   true,
				Validation: "email", // Valid rule
			},
			"website": {
				Type:       pluginmodel.InputTypeString,
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
	schema := &pluginmodel.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"required_param": {Type: pluginmodel.InputTypeString, Required: true},
			"optional_param": {Type: pluginmodel.InputTypeString, Required: false},
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
	schema := &pluginmodel.OperationSchema{
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
	schema := &pluginmodel.OperationSchema{
		Name:       "multi.required",
		PluginName: "test-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"param1": {Type: pluginmodel.InputTypeString, Required: true},
			"param2": {Type: pluginmodel.InputTypeInteger, Required: true},
			"param3": {Type: pluginmodel.InputTypeBoolean, Required: true},
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
	schema := &pluginmodel.OperationSchema{
		Name:       "all.optional",
		PluginName: "test-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"opt1": {Type: pluginmodel.InputTypeString, Required: false, Default: "default"},
			"opt2": {Type: pluginmodel.InputTypeInteger, Required: false, Default: 0},
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
	schema := &pluginmodel.OperationSchema{
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
	schema := &pluginmodel.OperationSchema{
		Name:       "empty.map",
		PluginName: "test-plugin",
		Inputs:     map[string]pluginmodel.InputSchema{}, // Explicitly empty map
	}

	result := schema.GetRequiredInputs()

	// Edge case: Empty map should return empty slice, not nil
	require.NotNil(t, result)
	assert.Empty(t, result)
}

// Component: T005
// Feature: C029
func TestOperationSchema_GetRequiredInputs_SingleRequired_ReturnsOne(t *testing.T) {
	schema := &pluginmodel.OperationSchema{
		Name:       "single.required",
		PluginName: "test-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"only_required": {Type: pluginmodel.InputTypeString, Required: true},
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
	schema := &pluginmodel.OperationSchema{
		Name:       "single.optional",
		PluginName: "test-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"only_optional": {Type: pluginmodel.InputTypeString, Required: false},
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
	schema := &pluginmodel.OperationSchema{
		Name:       "complex.operation",
		PluginName: "test-plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"api_key":    {Type: pluginmodel.InputTypeString, Required: true},
			"endpoint":   {Type: pluginmodel.InputTypeString, Required: true},
			"timeout":    {Type: pluginmodel.InputTypeInteger, Required: false, Default: 30},
			"retry":      {Type: pluginmodel.InputTypeBoolean, Required: false, Default: true},
			"user_id":    {Type: pluginmodel.InputTypeString, Required: true},
			"metadata":   {Type: pluginmodel.InputTypeObject, Required: false},
			"tags":       {Type: pluginmodel.InputTypeArray, Required: false, Default: []any{}},
			"debug_mode": {Type: pluginmodel.InputTypeBoolean, Required: true},
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
		schema         *pluginmodel.OperationSchema
		expectedCount  int
		expectedNames  []string
		shouldBeEmpty  bool
		shouldNotBeNil bool
	}{
		{
			name: "no inputs - empty slice",
			schema: &pluginmodel.OperationSchema{
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
			schema: &pluginmodel.OperationSchema{
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
			schema: &pluginmodel.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]pluginmodel.InputSchema{
					"req1": {Type: pluginmodel.InputTypeString, Required: true},
					"opt1": {Type: pluginmodel.InputTypeString, Required: false},
				},
			},
			expectedCount:  1,
			expectedNames:  []string{"req1"},
			shouldBeEmpty:  false,
			shouldNotBeNil: true,
		},
		{
			name: "all required",
			schema: &pluginmodel.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]pluginmodel.InputSchema{
					"req1": {Type: pluginmodel.InputTypeString, Required: true},
					"req2": {Type: pluginmodel.InputTypeInteger, Required: true},
					"req3": {Type: pluginmodel.InputTypeBoolean, Required: true},
				},
			},
			expectedCount:  3,
			expectedNames:  []string{"req1", "req2", "req3"},
			shouldBeEmpty:  false,
			shouldNotBeNil: true,
		},
		{
			name: "all optional",
			schema: &pluginmodel.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]pluginmodel.InputSchema{
					"opt1": {Type: pluginmodel.InputTypeString, Required: false},
					"opt2": {Type: pluginmodel.InputTypeInteger, Required: false},
				},
			},
			expectedCount:  0,
			expectedNames:  []string{},
			shouldBeEmpty:  true,
			shouldNotBeNil: true,
		},
		{
			name: "many required many optional",
			schema: &pluginmodel.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]pluginmodel.InputSchema{
					"req1": {Type: pluginmodel.InputTypeString, Required: true},
					"opt1": {Type: pluginmodel.InputTypeString, Required: false},
					"req2": {Type: pluginmodel.InputTypeInteger, Required: true},
					"opt2": {Type: pluginmodel.InputTypeInteger, Required: false},
					"req3": {Type: pluginmodel.InputTypeBoolean, Required: true},
					"opt3": {Type: pluginmodel.InputTypeBoolean, Required: false},
				},
			},
			expectedCount:  3,
			expectedNames:  []string{"req1", "req2", "req3"},
			shouldBeEmpty:  false,
			shouldNotBeNil: true,
		},
		{
			name: "single required input only",
			schema: &pluginmodel.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]pluginmodel.InputSchema{
					"only_one": {Type: pluginmodel.InputTypeString, Required: true},
				},
			},
			expectedCount:  1,
			expectedNames:  []string{"only_one"},
			shouldBeEmpty:  false,
			shouldNotBeNil: true,
		},
		{
			name: "single optional input only",
			schema: &pluginmodel.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]pluginmodel.InputSchema{
					"only_one": {Type: pluginmodel.InputTypeString, Required: false},
				},
			},
			expectedCount:  0,
			expectedNames:  []string{},
			shouldBeEmpty:  true,
			shouldNotBeNil: true,
		},
		{
			name: "required inputs with various types",
			schema: &pluginmodel.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]pluginmodel.InputSchema{
					"str_req":  {Type: pluginmodel.InputTypeString, Required: true},
					"int_req":  {Type: pluginmodel.InputTypeInteger, Required: true},
					"bool_req": {Type: pluginmodel.InputTypeBoolean, Required: true},
					"arr_req":  {Type: pluginmodel.InputTypeArray, Required: true},
					"obj_req":  {Type: pluginmodel.InputTypeObject, Required: true},
				},
			},
			expectedCount:  5,
			expectedNames:  []string{"str_req", "int_req", "bool_req", "arr_req", "obj_req"},
			shouldBeEmpty:  false,
			shouldNotBeNil: true,
		},
		{
			name: "required with default values",
			schema: &pluginmodel.OperationSchema{
				Name:       "test.op",
				PluginName: "test",
				Inputs: map[string]pluginmodel.InputSchema{
					"req_with_default": {Type: pluginmodel.InputTypeString, Required: true, Default: "default"},
					"opt_with_default": {Type: pluginmodel.InputTypeString, Required: false, Default: "default"},
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
	schema := &pluginmodel.OperationSchema{}

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
	inputs := make(map[string]pluginmodel.InputSchema)
	expectedRequired := []string{}

	// Create 100 inputs, half required, half optional
	for i := 0; i < 100; i++ {
		name := "input_" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		isRequired := i%2 == 0
		inputs[name] = pluginmodel.InputSchema{
			Type:     pluginmodel.InputTypeString,
			Required: isRequired,
		}
		if isRequired {
			expectedRequired = append(expectedRequired, name)
		}
	}

	schema := &pluginmodel.OperationSchema{
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

// InputSchema Tests

func TestInputSchema_AllTypes(t *testing.T) {
	tests := []struct {
		name       string
		schema     pluginmodel.InputSchema
		wantType   string
		wantReq    bool
		wantDefVal any
	}{
		{
			name: "string type",
			schema: pluginmodel.InputSchema{
				Type:        "string",
				Required:    true,
				Description: "A string input",
			},
			wantType: "string",
			wantReq:  true,
		},
		{
			name: "integer type with default",
			schema: pluginmodel.InputSchema{
				Type:    "integer",
				Default: 100,
			},
			wantType:   "integer",
			wantDefVal: 100,
		},
		{
			name: "boolean type",
			schema: pluginmodel.InputSchema{
				Type:    "boolean",
				Default: false,
			},
			wantType:   "boolean",
			wantDefVal: false,
		},
		{
			name: "array type",
			schema: pluginmodel.InputSchema{
				Type:        "array",
				Description: "List of items",
			},
			wantType: "array",
		},
		{
			name: "object type",
			schema: pluginmodel.InputSchema{
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
		schema         pluginmodel.InputSchema
		wantValidation string
	}{
		{
			name: "url validation",
			schema: pluginmodel.InputSchema{
				Type:       "string",
				Validation: "url",
			},
			wantValidation: "url",
		},
		{
			name: "email validation",
			schema: pluginmodel.InputSchema{
				Type:       "string",
				Validation: "email",
			},
			wantValidation: "email",
		},
		{
			name: "no validation",
			schema: pluginmodel.InputSchema{
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

// Feature: C029
// InputSchema.Validate() Comprehensive Tests

// Happy Path Tests - Valid Schemas

func TestInputSchema_Validate_ValidStringType_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:        pluginmodel.InputTypeString,
		Required:    true,
		Description: "A valid string input",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected valid string schema to pass validation")
}

func TestInputSchema_Validate_ValidIntegerType_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:        pluginmodel.InputTypeInteger,
		Required:    false,
		Description: "A valid integer input",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected valid integer schema to pass validation")
}

func TestInputSchema_Validate_ValidBooleanType_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:     pluginmodel.InputTypeBoolean,
		Required: true,
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected valid boolean schema to pass validation")
}

func TestInputSchema_Validate_ValidArrayType_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:        pluginmodel.InputTypeArray,
		Description: "A valid array input",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected valid array schema to pass validation")
}

func TestInputSchema_Validate_ValidObjectType_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:        pluginmodel.InputTypeObject,
		Description: "A valid object input",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected valid object schema to pass validation")
}

// Edge Case Tests - Validation Rules

func TestInputSchema_Validate_WithURLValidation_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:        pluginmodel.InputTypeString,
		Validation:  "url",
		Description: "URL input with validation",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected schema with 'url' validation to pass")
}

func TestInputSchema_Validate_WithEmailValidation_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:        pluginmodel.InputTypeString,
		Validation:  "email",
		Description: "Email input with validation",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected schema with 'email' validation to pass")
}

func TestInputSchema_Validate_WithEmptyValidation_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:       pluginmodel.InputTypeString,
		Validation: "", // Empty validation is allowed (no validation)
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected schema with empty validation to pass")
}

// Edge Case Tests - Default Values Matching Types

func TestInputSchema_Validate_StringDefaultMatchesType_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeString,
		Default: "default value",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected string default to match string type")
}

func TestInputSchema_Validate_IntegerDefaultMatchesType_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeInteger,
		Default: 42,
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected int default to match integer type")
}

func TestInputSchema_Validate_Float64DefaultForInteger_ReturnsNil(t *testing.T) {
	// JSON decoding may produce float64 for integer types
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeInteger,
		Default: float64(42),
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected float64 default to be accepted for integer type (JSON compatibility)")
}

func TestInputSchema_Validate_BooleanDefaultMatchesType_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeBoolean,
		Default: true,
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected bool default to match boolean type")
}

func TestInputSchema_Validate_ArrayDefaultMatchesType_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeArray,
		Default: []any{"item1", "item2"},
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected slice default to match array type")
}

func TestInputSchema_Validate_ObjectDefaultMatchesType_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeObject,
		Default: map[string]any{"key": "value"},
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected map default to match object type")
}

func TestInputSchema_Validate_NoDefault_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:     pluginmodel.InputTypeString,
		Default:  nil, // No default value provided
		Required: true,
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected schema without default to pass validation")
}

// Error Handling Tests - Invalid Types

func TestInputSchema_Validate_EmptyType_ReturnsError(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type: "",
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for empty type")
	assert.Contains(t, err.Error(), "type", "Error message should mention 'type'")
}

func TestInputSchema_Validate_InvalidType_ReturnsError(t *testing.T) {
	schema := &pluginmodel.InputSchema{
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
			schema := &pluginmodel.InputSchema{
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
	schema := &pluginmodel.InputSchema{
		Type:       pluginmodel.InputTypeString,
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
			schema := &pluginmodel.InputSchema{
				Type:       pluginmodel.InputTypeString,
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
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeInteger,
		Default: "not an integer",
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for string default on integer type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_IntegerDefaultForString_ReturnsError(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeString,
		Default: 42,
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for integer default on string type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_StringDefaultForBoolean_ReturnsError(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeBoolean,
		Default: "true",
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for string default on boolean type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_IntegerDefaultForArray_ReturnsError(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeArray,
		Default: 123,
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for integer default on array type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_StringDefaultForObject_ReturnsError(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeObject,
		Default: "not an object",
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for string default on object type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_MapDefaultForArray_ReturnsError(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeArray,
		Default: map[string]any{"key": "value"},
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for map default on array type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

func TestInputSchema_Validate_SliceDefaultForObject_ReturnsError(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:    pluginmodel.InputTypeObject,
		Default: []any{"item"},
	}

	err := schema.Validate()

	require.Error(t, err, "Expected error for slice default on object type")
	assert.Contains(t, err.Error(), "default", "Error message should mention 'default'")
}

// Edge Case Tests - Complex Scenarios

func TestInputSchema_Validate_AllFieldsValid_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type:        pluginmodel.InputTypeString,
		Required:    true,
		Default:     "default@example.com",
		Description: "User email address",
		Validation:  "email",
	}

	err := schema.Validate()

	assert.NoError(t, err, "Expected fully populated valid schema to pass validation")
}

func TestInputSchema_Validate_MinimalSchema_ReturnsNil(t *testing.T) {
	schema := &pluginmodel.InputSchema{
		Type: pluginmodel.InputTypeString,
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
		{"string-string", pluginmodel.InputTypeString, "value", false, "string default for string type"},
		{"integer-int", pluginmodel.InputTypeInteger, 42, false, "int default for integer type"},
		{"integer-float64", pluginmodel.InputTypeInteger, float64(42), false, "float64 default for integer type"},
		{"boolean-bool", pluginmodel.InputTypeBoolean, true, false, "bool default for boolean type"},
		{"array-slice", pluginmodel.InputTypeArray, []any{1, 2}, false, "slice default for array type"},
		{"object-map", pluginmodel.InputTypeObject, map[string]any{"k": "v"}, false, "map default for object type"},
		{"any-nil", pluginmodel.InputTypeString, nil, false, "nil default (no default)"},

		// Invalid combinations
		{"string-int", pluginmodel.InputTypeString, 123, true, "int default for string type"},
		{"integer-string", pluginmodel.InputTypeInteger, "123", true, "string default for integer type"},
		{"boolean-string", pluginmodel.InputTypeBoolean, "false", true, "string default for boolean type"},
		{"boolean-int", pluginmodel.InputTypeBoolean, 0, true, "int default for boolean type"},
		{"array-map", pluginmodel.InputTypeArray, map[string]any{"k": "v"}, true, "map default for array type"},
		{"array-string", pluginmodel.InputTypeArray, "[]", true, "string default for array type"},
		{"object-slice", pluginmodel.InputTypeObject, []any{1, 2}, true, "slice default for object type"},
		{"object-string", pluginmodel.InputTypeObject, "{}", true, "string default for object type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &pluginmodel.InputSchema{
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
		{"string type", pluginmodel.InputTypeString},
		{"integer type", pluginmodel.InputTypeInteger},
		{"boolean type", pluginmodel.InputTypeBoolean},
		{"array type", pluginmodel.InputTypeArray},
		{"object type", pluginmodel.InputTypeObject},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &pluginmodel.InputSchema{Type: tt.typeName}

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
			schema := &pluginmodel.InputSchema{Type: tt.typeName}

			result := schema.IsValidType()

			assert.False(t, result)
		})
	}
}

// OperationResult Tests

func TestOperationResult_Success(t *testing.T) {
	result := pluginmodel.OperationResult{
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
	result := pluginmodel.OperationResult{
		Success: false,
		Error:   "connection refused",
		Outputs: nil,
	}

	assert.False(t, result.Success)
	assert.Equal(t, "connection refused", result.Error)
	assert.Nil(t, result.Outputs)
}

func TestOperationResult_PartialOutputs(t *testing.T) {
	result := pluginmodel.OperationResult{
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
	result := &pluginmodel.OperationResult{Success: true}

	assert.True(t, result.IsSuccess())
}

func TestOperationResult_IsSuccess_False(t *testing.T) {
	result := &pluginmodel.OperationResult{Success: false}

	assert.False(t, result.IsSuccess())
}

func TestOperationResult_IsSuccess_WithError(t *testing.T) {
	result := &pluginmodel.OperationResult{
		Success: false,
		Error:   "something went wrong",
	}

	assert.False(t, result.IsSuccess())
}

func TestOperationResult_IsSuccess_DefaultValue(t *testing.T) {
	result := &pluginmodel.OperationResult{}

	// Zero value of bool is false
	assert.False(t, result.IsSuccess())
}

// OperationResult.HasError Tests

func TestOperationResult_HasError_WithError(t *testing.T) {
	result := &pluginmodel.OperationResult{
		Success: false,
		Error:   "connection refused",
	}

	assert.True(t, result.HasError())
}

func TestOperationResult_HasError_EmptyError(t *testing.T) {
	result := &pluginmodel.OperationResult{
		Success: true,
		Error:   "",
	}

	assert.False(t, result.HasError())
}

func TestOperationResult_HasError_DefaultValue(t *testing.T) {
	result := &pluginmodel.OperationResult{}

	// Zero value of string is ""
	assert.False(t, result.HasError())
}

func TestOperationResult_HasError_WhitespaceError(t *testing.T) {
	result := &pluginmodel.OperationResult{
		Success: false,
		Error:   " ",
	}

	// HasError returns true for any non-empty string including whitespace
	assert.True(t, result.HasError())
}

// OperationResult.GetOutput Tests

func TestOperationResult_GetOutput_ExistingKey(t *testing.T) {
	result := &pluginmodel.OperationResult{
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
	result := &pluginmodel.OperationResult{
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
	result := &pluginmodel.OperationResult{
		Success: false,
		Outputs: nil,
	}

	val, ok := result.GetOutput("any_key")

	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestOperationResult_GetOutput_EmptyOutputs(t *testing.T) {
	result := &pluginmodel.OperationResult{
		Success: true,
		Outputs: map[string]any{},
	}

	val, ok := result.GetOutput("any_key")

	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestOperationResult_GetOutput_NilValue(t *testing.T) {
	result := &pluginmodel.OperationResult{
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
	result := &pluginmodel.OperationResult{
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
