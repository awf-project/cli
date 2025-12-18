package plugin_test

import (
	"errors"
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

// Validate Tests (Stub - returns ErrNotImplemented)

func TestOperationSchema_Validate_ReturnsNotImplemented(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.True(t, errors.Is(err, plugin.ErrNotImplemented))
}

func TestOperationSchema_Validate_ValidSchema_StillReturnsNotImplemented(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:        "slack.send",
		Description: "Send message to Slack",
		Inputs: map[string]plugin.InputSchema{
			"channel": {Type: plugin.InputTypeString, Required: true},
			"message": {Type: plugin.InputTypeString, Required: true},
		},
		Outputs:    []string{"message_id"},
		PluginName: "slack-notifier",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.True(t, errors.Is(err, plugin.ErrNotImplemented))
}

func TestOperationSchema_Validate_EmptyName_StillReturnsNotImplemented(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "",
		PluginName: "test-plugin",
	}

	err := schema.Validate()

	// Until implemented, always returns ErrNotImplemented
	require.Error(t, err)
	assert.True(t, errors.Is(err, plugin.ErrNotImplemented))
}

func TestOperationSchema_Validate_EmptyPluginName_StillReturnsNotImplemented(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "test.op",
		PluginName: "",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.True(t, errors.Is(err, plugin.ErrNotImplemented))
}

// GetRequiredInputs Tests (Stub - returns nil)

func TestOperationSchema_GetRequiredInputs_ReturnsNil(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "test.operation",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"required_param": {Type: plugin.InputTypeString, Required: true},
			"optional_param": {Type: plugin.InputTypeString, Required: false},
		},
	}

	result := schema.GetRequiredInputs()

	// Stub returns nil
	assert.Nil(t, result)
}

func TestOperationSchema_GetRequiredInputs_NoInputs_ReturnsNil(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "health.check",
		PluginName: "health-plugin",
	}

	result := schema.GetRequiredInputs()

	assert.Nil(t, result)
}

func TestOperationSchema_GetRequiredInputs_AllRequired_StillReturnsNil(t *testing.T) {
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

	// Stub returns nil, but when implemented should return ["param1", "param2", "param3"]
	assert.Nil(t, result)
}

func TestOperationSchema_GetRequiredInputs_NoneRequired_StillReturnsNil(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name:       "all.optional",
		PluginName: "test-plugin",
		Inputs: map[string]plugin.InputSchema{
			"opt1": {Type: plugin.InputTypeString, Required: false, Default: "default"},
			"opt2": {Type: plugin.InputTypeInteger, Required: false, Default: 0},
		},
	}

	result := schema.GetRequiredInputs()

	// Stub returns nil, when implemented should return empty slice
	assert.Nil(t, result)
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

// InputSchema.Validate Tests (Stub - returns ErrNotImplemented)

func TestInputSchema_Validate_ReturnsNotImplemented(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:     plugin.InputTypeString,
		Required: true,
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.True(t, errors.Is(err, plugin.ErrNotImplemented))
}

func TestInputSchema_Validate_ValidSchema_StillReturnsNotImplemented(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:        plugin.InputTypeString,
		Required:    true,
		Description: "A valid input parameter",
		Validation:  "email",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.True(t, errors.Is(err, plugin.ErrNotImplemented))
}

func TestInputSchema_Validate_InvalidType_StillReturnsNotImplemented(t *testing.T) {
	schema := &plugin.InputSchema{
		Type: "invalid_type",
	}

	err := schema.Validate()

	// Until implemented, always returns ErrNotImplemented
	require.Error(t, err)
	assert.True(t, errors.Is(err, plugin.ErrNotImplemented))
}

func TestInputSchema_Validate_EmptyType_StillReturnsNotImplemented(t *testing.T) {
	schema := &plugin.InputSchema{
		Type: "",
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.True(t, errors.Is(err, plugin.ErrNotImplemented))
}

func TestInputSchema_Validate_WithDefault_StillReturnsNotImplemented(t *testing.T) {
	schema := &plugin.InputSchema{
		Type:    plugin.InputTypeInteger,
		Default: 42,
	}

	err := schema.Validate()

	require.Error(t, err)
	assert.True(t, errors.Is(err, plugin.ErrNotImplemented))
}

// InputSchema.IsValidType Tests (Stub - returns false)

func TestInputSchema_IsValidType_ValidTypes_ReturnsFalse(t *testing.T) {
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

			// Stub returns false, but when implemented should return true
			assert.False(t, result)
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

			// Stub returns false (correct for invalid types)
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
