package operation_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/operation"
	"github.com/vanoix/awf/internal/domain/plugin"
)

// TestValidateInputs_HappyPath tests successful validation scenarios
func TestValidateInputs_HappyPath(t *testing.T) {
	tests := []struct {
		name   string
		schema *plugin.OperationSchema
		inputs map[string]any
	}{
		{
			name: "all required fields present",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"url": {
						Type:     plugin.InputTypeString,
						Required: true,
					},
					"timeout": {
						Type:     plugin.InputTypeInteger,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"url":     "https://example.com",
				"timeout": 30,
			},
		},
		{
			name: "optional fields omitted - defaults applied",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"url": {
						Type:     plugin.InputTypeString,
						Required: true,
					},
					"timeout": {
						Type:     plugin.InputTypeInteger,
						Required: false,
						Default:  30,
					},
				},
			},
			inputs: map[string]any{
				"url": "https://example.com",
			},
		},
		{
			name: "all five types valid",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"name": {
						Type:     plugin.InputTypeString,
						Required: true,
					},
					"count": {
						Type:     plugin.InputTypeInteger,
						Required: true,
					},
					"enabled": {
						Type:     plugin.InputTypeBoolean,
						Required: true,
					},
					"tags": {
						Type:     plugin.InputTypeArray,
						Required: true,
					},
					"config": {
						Type:     plugin.InputTypeObject,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"name":    "test",
				"count":   42,
				"enabled": true,
				"tags":    []any{"tag1", "tag2"},
				"config":  map[string]any{"key": "value"},
			},
		},
		{
			name: "empty inputs with all optional fields",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"optional1": {
						Type:     plugin.InputTypeString,
						Required: false,
						Default:  "default1",
					},
					"optional2": {
						Type:     plugin.InputTypeInteger,
						Required: false,
						Default:  10,
					},
				},
			},
			inputs: map[string]any{},
		},
		{
			name: "validation rule url - valid",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"endpoint": {
						Type:       plugin.InputTypeString,
						Required:   true,
						Validation: "url",
					},
				},
			},
			inputs: map[string]any{
				"endpoint": "https://api.example.com/v1/users",
			},
		},
		{
			name: "validation rule email - valid",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"recipient": {
						Type:       plugin.InputTypeString,
						Required:   true,
						Validation: "email",
					},
				},
			},
			inputs: map[string]any{
				"recipient": "user@example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateInputs(tt.schema, tt.inputs)
			assert.NoError(t, err, "validation should succeed for valid inputs")
		})
	}
}

// TestValidateInputs_RequiredFieldsMissing tests required field validation
func TestValidateInputs_RequiredFieldsMissing(t *testing.T) {
	tests := []struct {
		name          string
		schema        *plugin.OperationSchema
		inputs        map[string]any
		expectedError string
	}{
		{
			name: "single required field missing",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"url": {
						Type:     plugin.InputTypeString,
						Required: true,
					},
				},
			},
			inputs:        map[string]any{},
			expectedError: "url",
		},
		{
			name: "multiple required fields missing",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"url": {
						Type:     plugin.InputTypeString,
						Required: true,
					},
					"method": {
						Type:     plugin.InputTypeString,
						Required: true,
					},
				},
			},
			inputs:        map[string]any{},
			expectedError: "url",
		},
		{
			name: "required field with nil value",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"data": {
						Type:     plugin.InputTypeString,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"data": nil,
			},
			expectedError: "data",
		},
		{
			name: "required field with empty string",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"name": {
						Type:     plugin.InputTypeString,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"name": "",
			},
			expectedError: "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateInputs(tt.schema, tt.inputs)
			require.Error(t, err, "validation should fail for missing required fields")
			assert.True(t, errors.Is(err, operation.ErrInvalidInputs), "error should be ErrInvalidInputs")
			assert.Contains(t, err.Error(), tt.expectedError, "error message should mention missing field")
		})
	}
}

// TestValidateInputs_TypeMismatch tests type validation for each input type
func TestValidateInputs_TypeMismatch(t *testing.T) {
	tests := []struct {
		name          string
		schema        *plugin.OperationSchema
		inputs        map[string]any
		expectedError string
	}{
		{
			name: "string type - int value",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"name": {
						Type:     plugin.InputTypeString,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"name": 123,
			},
			expectedError: "name",
		},
		{
			name: "integer type - string value",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"count": {
						Type:     plugin.InputTypeInteger,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"count": "not-a-number",
			},
			expectedError: "count",
		},
		{
			name: "boolean type - string value",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"enabled": {
						Type:     plugin.InputTypeBoolean,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"enabled": "yes",
			},
			expectedError: "enabled",
		},
		{
			name: "array type - object value",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"items": {
						Type:     plugin.InputTypeArray,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"items": map[string]any{"key": "value"},
			},
			expectedError: "items",
		},
		{
			name: "object type - array value",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"config": {
						Type:     plugin.InputTypeObject,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"config": []any{"item1", "item2"},
			},
			expectedError: "config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateInputs(tt.schema, tt.inputs)
			require.Error(t, err, "validation should fail for type mismatch")
			assert.True(t, errors.Is(err, operation.ErrInvalidInputs), "error should be ErrInvalidInputs")
			assert.Contains(t, err.Error(), tt.expectedError, "error message should mention field name")
		})
	}
}

// TestValidateInputs_Float64ToIntCoercion tests JSON float64 to int conversion
func TestValidateInputs_Float64ToIntCoercion(t *testing.T) {
	tests := []struct {
		name    string
		schema  *plugin.OperationSchema
		inputs  map[string]any
		wantErr bool
	}{
		{
			name: "float64 with whole number - should coerce",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"count": {
						Type:     plugin.InputTypeInteger,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"count": 42.0,
			},
			wantErr: false,
		},
		{
			name: "float64 with fractional part - should fail",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"count": {
						Type:     plugin.InputTypeInteger,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"count": 42.5,
			},
			wantErr: true,
		},
		{
			name: "large integer as float64 - should coerce",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"timestamp": {
						Type:     plugin.InputTypeInteger,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"timestamp": 1704067200.0,
			},
			wantErr: false,
		},
		{
			name: "zero as float64 - should coerce",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"count": {
						Type:     plugin.InputTypeInteger,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"count": 0.0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateInputs(tt.schema, tt.inputs)
			if tt.wantErr {
				require.Error(t, err, "validation should fail for non-whole float64")
				assert.True(t, errors.Is(err, operation.ErrInvalidInputs))
			} else {
				assert.NoError(t, err, "validation should succeed for whole number float64")
			}
		})
	}
}

// TestValidateInputs_DefaultValues tests default value application
func TestValidateInputs_DefaultValues(t *testing.T) {
	tests := []struct {
		name           string
		schema         *plugin.OperationSchema
		inputs         map[string]any
		expectedInputs map[string]any
	}{
		{
			name: "string default applied",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"method": {
						Type:     plugin.InputTypeString,
						Required: false,
						Default:  "GET",
					},
				},
			},
			inputs: map[string]any{},
			expectedInputs: map[string]any{
				"method": "GET",
			},
		},
		{
			name: "integer default applied",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"timeout": {
						Type:     plugin.InputTypeInteger,
						Required: false,
						Default:  30,
					},
				},
			},
			inputs: map[string]any{},
			expectedInputs: map[string]any{
				"timeout": 30,
			},
		},
		{
			name: "boolean default applied",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"verbose": {
						Type:     plugin.InputTypeBoolean,
						Required: false,
						Default:  false,
					},
				},
			},
			inputs: map[string]any{},
			expectedInputs: map[string]any{
				"verbose": false,
			},
		},
		{
			name: "array default applied",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"headers": {
						Type:     plugin.InputTypeArray,
						Required: false,
						Default:  []any{"Content-Type: application/json"},
					},
				},
			},
			inputs: map[string]any{},
			expectedInputs: map[string]any{
				"headers": []any{"Content-Type: application/json"},
			},
		},
		{
			name: "object default applied",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"options": {
						Type:     plugin.InputTypeObject,
						Required: false,
						Default:  map[string]any{"retry": 3},
					},
				},
			},
			inputs: map[string]any{},
			expectedInputs: map[string]any{
				"options": map[string]any{"retry": 3},
			},
		},
		{
			name: "default not applied when value provided",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"timeout": {
						Type:     plugin.InputTypeInteger,
						Required: false,
						Default:  30,
					},
				},
			},
			inputs: map[string]any{
				"timeout": 60,
			},
			expectedInputs: map[string]any{
				"timeout": 60,
			},
		},
		{
			name: "multiple defaults applied selectively",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"method": {
						Type:     plugin.InputTypeString,
						Required: false,
						Default:  "GET",
					},
					"timeout": {
						Type:     plugin.InputTypeInteger,
						Required: false,
						Default:  30,
					},
					"verbose": {
						Type:     plugin.InputTypeBoolean,
						Required: false,
						Default:  false,
					},
				},
			},
			inputs: map[string]any{
				"method": "POST",
			},
			expectedInputs: map[string]any{
				"method":  "POST",
				"timeout": 30,
				"verbose": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateInputs(tt.schema, tt.inputs)
			assert.NoError(t, err, "validation should succeed")

			// Verify defaults were applied to inputs map
			for key, expectedValue := range tt.expectedInputs {
				actualValue, ok := tt.inputs[key]
				assert.True(t, ok, "expected key %s to be in inputs", key)
				assert.Equal(t, expectedValue, actualValue, "default value for %s should match", key)
			}
		})
	}
}

// TestValidateInputs_ValidationRules tests validation rule enforcement
func TestValidateInputs_ValidationRules(t *testing.T) {
	tests := []struct {
		name    string
		schema  *plugin.OperationSchema
		inputs  map[string]any
		wantErr bool
	}{
		{
			name: "url validation - valid",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"endpoint": {
						Type:       plugin.InputTypeString,
						Required:   true,
						Validation: "url",
					},
				},
			},
			inputs: map[string]any{
				"endpoint": "https://api.example.com",
			},
			wantErr: false,
		},
		{
			name: "url validation - invalid",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"endpoint": {
						Type:       plugin.InputTypeString,
						Required:   true,
						Validation: "url",
					},
				},
			},
			inputs: map[string]any{
				"endpoint": "not-a-url",
			},
			wantErr: true,
		},
		{
			name: "email validation - valid",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"recipient": {
						Type:       plugin.InputTypeString,
						Required:   true,
						Validation: "email",
					},
				},
			},
			inputs: map[string]any{
				"recipient": "test@example.com",
			},
			wantErr: false,
		},
		{
			name: "email validation - invalid",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"recipient": {
						Type:       plugin.InputTypeString,
						Required:   true,
						Validation: "email",
					},
				},
			},
			inputs: map[string]any{
				"recipient": "not-an-email",
			},
			wantErr: true,
		},
		{
			name: "no validation rule - any string accepted",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"data": {
						Type:       plugin.InputTypeString,
						Required:   true,
						Validation: "",
					},
				},
			},
			inputs: map[string]any{
				"data": "any-string-here",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateInputs(tt.schema, tt.inputs)
			if tt.wantErr {
				require.Error(t, err, "validation should fail for invalid value")
				assert.True(t, errors.Is(err, operation.ErrInvalidInputs))
			} else {
				assert.NoError(t, err, "validation should succeed for valid value")
			}
		})
	}
}

// TestValidateInputs_EdgeCases tests boundary conditions and edge cases
func TestValidateInputs_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		schema  *plugin.OperationSchema
		inputs  map[string]any
		wantErr bool
	}{
		{
			name:   "nil schema",
			schema: nil,
			inputs: map[string]any{
				"key": "value",
			},
			wantErr: true,
		},
		{
			name: "nil inputs map",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"optional": {
						Type:     plugin.InputTypeString,
						Required: false,
						Default:  "default",
					},
				},
			},
			inputs:  nil,
			wantErr: true,
		},
		{
			name: "empty schema inputs - any inputs accepted",
			schema: &plugin.OperationSchema{
				Name:   "test.operation",
				Inputs: map[string]plugin.InputSchema{},
			},
			inputs: map[string]any{
				"anything": "goes",
			},
			wantErr: false,
		},
		{
			name: "extra inputs not in schema - ignored",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"defined": {
						Type:     plugin.InputTypeString,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"defined":   "value",
				"undefined": "extra",
			},
			wantErr: false,
		},
		{
			name: "zero integer value for required field - valid",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"count": {
						Type:     plugin.InputTypeInteger,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"count": 0,
			},
			wantErr: false,
		},
		{
			name: "false boolean value for required field - valid",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"enabled": {
						Type:     plugin.InputTypeBoolean,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"enabled": false,
			},
			wantErr: false,
		},
		{
			name: "empty array for required field - valid",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"tags": {
						Type:     plugin.InputTypeArray,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"tags": []any{},
			},
			wantErr: false,
		},
		{
			name: "empty object for required field - valid",
			schema: &plugin.OperationSchema{
				Name: "test.operation",
				Inputs: map[string]plugin.InputSchema{
					"config": {
						Type:     plugin.InputTypeObject,
						Required: true,
					},
				},
			},
			inputs: map[string]any{
				"config": map[string]any{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateInputs(tt.schema, tt.inputs)
			if tt.wantErr {
				assert.Error(t, err, "validation should fail for edge case")
			} else {
				assert.NoError(t, err, "validation should succeed for edge case")
			}
		})
	}
}

// TestValidateInputs_MultipleErrors tests error aggregation
func TestValidateInputs_MultipleErrors(t *testing.T) {
	schema := &plugin.OperationSchema{
		Name: "test.operation",
		Inputs: map[string]plugin.InputSchema{
			"url": {
				Type:       plugin.InputTypeString,
				Required:   true,
				Validation: "url",
			},
			"email": {
				Type:       plugin.InputTypeString,
				Required:   true,
				Validation: "email",
			},
			"count": {
				Type:     plugin.InputTypeInteger,
				Required: true,
			},
		},
	}

	inputs := map[string]any{
		"url":   "not-a-url",
		"email": "not-an-email",
		"count": "not-a-number",
	}

	err := operation.ValidateInputs(schema, inputs)
	require.Error(t, err, "validation should fail with multiple errors")
	assert.True(t, errors.Is(err, operation.ErrInvalidInputs))

	// Error message should mention multiple violations
	errMsg := err.Error()
	assert.Contains(t, errMsg, "url", "error should mention url field")
	assert.Contains(t, errMsg, "email", "error should mention email field")
	assert.Contains(t, errMsg, "count", "error should mention count field")
}
