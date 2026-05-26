package tools_test

import (
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/infrastructure/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapOperationSchema_PrimitiveTypes(t *testing.T) {
	tests := []struct {
		name         string
		inputType    string
		expectedType string
	}{
		{
			name:         "string type",
			inputType:    "string",
			expectedType: "string",
		},
		{
			name:         "integer type",
			inputType:    "integer",
			expectedType: "integer",
		},
		{
			name:         "boolean type",
			inputType:    "boolean",
			expectedType: "boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := pluginmodel.OperationSchema{
				Name:       "test_op",
				PluginName: "test_plugin",
				Inputs: map[string]pluginmodel.InputSchema{
					"param": {Type: tt.inputType},
				},
			}

			result, err := tools.MapOperationSchema(&schema)

			require.NoError(t, err)
			assert.Equal(t, "object", result["type"])
			props := result["properties"].(map[string]any)
			paramProp := props["param"].(map[string]any)
			assert.Equal(t, tt.expectedType, paramProp["type"])
		})
	}
}

func TestMapOperationSchema_RequiredField(t *testing.T) {
	schema := pluginmodel.OperationSchema{
		Name:       "test_op",
		PluginName: "test_plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"required_param": {Type: "string", Required: true},
			"optional_param": {Type: "string", Required: false},
		},
	}

	result, err := tools.MapOperationSchema(&schema)

	require.NoError(t, err)
	required := result["required"].([]string)
	assert.Contains(t, required, "required_param")
	assert.NotContains(t, required, "optional_param")
}

func TestMapOperationSchema_DefaultValue(t *testing.T) {
	schema := pluginmodel.OperationSchema{
		Name:       "test_op",
		PluginName: "test_plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"with_default": {Type: "string", Default: "default_value"},
		},
	}

	result, err := tools.MapOperationSchema(&schema)

	require.NoError(t, err)
	props := result["properties"].(map[string]any)
	prop := props["with_default"].(map[string]any)
	assert.Equal(t, "default_value", prop["default"])
}

func TestMapOperationSchema_Description(t *testing.T) {
	schema := pluginmodel.OperationSchema{
		Name:       "test_op",
		PluginName: "test_plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"documented": {Type: "string", Description: "A test parameter"},
		},
	}

	result, err := tools.MapOperationSchema(&schema)

	require.NoError(t, err)
	props := result["properties"].(map[string]any)
	prop := props["documented"].(map[string]any)
	assert.Equal(t, "A test parameter", prop["description"])
}

func TestMapOperationSchema_ValidationURL(t *testing.T) {
	schema := pluginmodel.OperationSchema{
		Name:       "test_op",
		PluginName: "test_plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"website": {Type: "string", Validation: "url"},
		},
	}

	result, err := tools.MapOperationSchema(&schema)

	require.NoError(t, err)
	props := result["properties"].(map[string]any)
	prop := props["website"].(map[string]any)
	assert.Equal(t, "uri", prop["format"])
}

func TestMapOperationSchema_ValidationEmail(t *testing.T) {
	schema := pluginmodel.OperationSchema{
		Name:       "test_op",
		PluginName: "test_plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"email_addr": {Type: "string", Validation: "email"},
		},
	}

	result, err := tools.MapOperationSchema(&schema)

	require.NoError(t, err)
	props := result["properties"].(map[string]any)
	prop := props["email_addr"].(map[string]any)
	assert.Equal(t, "email", prop["format"])
}

func TestMapOperationSchema_UnsupportedArrayType(t *testing.T) {
	schema := pluginmodel.OperationSchema{
		Name:       "test_op",
		PluginName: "test_plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"items": {Type: "array"},
		},
	}

	_, err := tools.MapOperationSchema(&schema)

	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrUnsupportedSchema))
	assert.Contains(t, err.Error(), "items")
}

func TestMapOperationSchema_UnsupportedObjectType(t *testing.T) {
	schema := pluginmodel.OperationSchema{
		Name:       "test_op",
		PluginName: "test_plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"config": {Type: "object"},
		},
	}

	_, err := tools.MapOperationSchema(&schema)

	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrUnsupportedSchema))
	assert.Contains(t, err.Error(), "config")
}

func TestMapOperationSchema_DocumentStructure(t *testing.T) {
	schema := pluginmodel.OperationSchema{
		Name:       "test_op",
		PluginName: "test_plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"param": {Type: "string", Required: true},
		},
	}

	result, err := tools.MapOperationSchema(&schema)

	require.NoError(t, err)

	assert.Equal(t, "object", result["type"])
	assert.NotNil(t, result["properties"])
	assert.NotNil(t, result["required"])
}

// TestMapOperationSchema_RequiredIsSorted verifies that the required field list is
// always returned in lexicographic order regardless of map iteration order.
// Agents that compare tools/list responses across calls must not see spurious diffs.
func TestMapOperationSchema_RequiredIsSorted(t *testing.T) {
	schema := pluginmodel.OperationSchema{
		Name:       "multi_required",
		PluginName: "test_plugin",
		Inputs: map[string]pluginmodel.InputSchema{
			"zebra":  {Type: "string", Required: true},
			"apple":  {Type: "string", Required: true},
			"mango":  {Type: "string", Required: true},
			"banana": {Type: "string", Required: true},
		},
	}

	// Call multiple times to expose any non-determinism from map iteration.
	for i := range 10 {
		result, err := tools.MapOperationSchema(&schema)
		require.NoError(t, err)
		required := result["required"].([]string)
		require.Len(t, required, 4)
		assert.Equal(t, []string{"apple", "banana", "mango", "zebra"}, required,
			"required fields must be sorted lexicographically (iteration %d)", i)
	}
}
