package pluginmgr

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertExecuteResponse_HappyPath tests successful conversion of ExecuteResponse.
// Output field is mapped to Outputs["output"], Data entries are JSON-decoded and merged.
func TestConvertExecuteResponse_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		pluginID string
		resp     *pluginv1.ExecuteResponse
		want     *pluginmodel.OperationResult
	}{
		{
			name:     "output only",
			pluginID: "test-plugin",
			resp: &pluginv1.ExecuteResponse{
				Success: true,
				Output:  "hello world",
				Data:    map[string][]byte{},
			},
			want: &pluginmodel.OperationResult{
				Success: true,
				Outputs: map[string]any{
					"output": "hello world",
				},
				Error: "",
			},
		},
		{
			name:     "output with data",
			pluginID: "test-plugin",
			resp: &pluginv1.ExecuteResponse{
				Success: true,
				Output:  "ok",
				Data: map[string][]byte{
					"result": []byte(`"value1"`),
					"count":  []byte(`42`),
				},
			},
			want: &pluginmodel.OperationResult{
				Success: true,
				Outputs: map[string]any{
					"output": "ok",
					"result": "value1",
					"count":  float64(42),
				},
				Error: "",
			},
		},
		{
			name:     "data wins on conflict",
			pluginID: "test-plugin",
			resp: &pluginv1.ExecuteResponse{
				Success: true,
				Output:  "original",
				Data: map[string][]byte{
					"output": []byte(`"overwritten"`),
				},
			},
			want: &pluginmodel.OperationResult{
				Success: true,
				Outputs: map[string]any{
					"output": "overwritten",
				},
				Error: "",
			},
		},
		{
			name:     "error response",
			pluginID: "test-plugin",
			resp: &pluginv1.ExecuteResponse{
				Success: false,
				Output:  "",
				Error:   "operation failed",
				Data:    map[string][]byte{},
			},
			want: &pluginmodel.OperationResult{
				Success: false,
				Outputs: map[string]any{},
				Error:   "operation failed",
			},
		},
		{
			name:     "empty output and data",
			pluginID: "test-plugin",
			resp: &pluginv1.ExecuteResponse{
				Success: true,
				Output:  "",
				Data:    map[string][]byte{},
			},
			want: &pluginmodel.OperationResult{
				Success: true,
				Outputs: map[string]any{},
				Error:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertExecuteResponse(tt.pluginID, tt.resp)

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Success, got.Success)
			assert.Equal(t, tt.want.Error, got.Error)
			assert.Equal(t, tt.want.Outputs, got.Outputs)
		})
	}
}

// TestConvertExecuteResponse_InvalidJSON tests handling of malformed JSON in Data field.
// Invalid JSON is preserved as a raw string in Outputs.
func TestConvertExecuteResponse_InvalidJSON(t *testing.T) {
	tests := []struct {
		name         string
		pluginID     string
		resp         *pluginv1.ExecuteResponse
		wantKeyInOut string
		wantValueStr string
	}{
		{
			name:     "invalid json in data",
			pluginID: "test-plugin",
			resp: &pluginv1.ExecuteResponse{
				Success: true,
				Output:  "test",
				Data: map[string][]byte{
					"bad": []byte(`{invalid json}`),
				},
			},
			wantKeyInOut: "bad",
			wantValueStr: "{invalid json}",
		},
		{
			name:     "malformed json number",
			pluginID: "test-plugin",
			resp: &pluginv1.ExecuteResponse{
				Success: false,
				Data: map[string][]byte{
					"count": []byte(`not-a-number`),
				},
			},
			wantKeyInOut: "count",
			wantValueStr: "not-a-number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertExecuteResponse(tt.pluginID, tt.resp)

			require.NotNil(t, got)
			// Malformed JSON should be preserved as raw string
			val, exists := got.GetOutput(tt.wantKeyInOut)
			require.True(t, exists, "key %q should exist in Outputs", tt.wantKeyInOut)
			assert.Equal(t, tt.wantValueStr, val)
		})
	}
}

// TestConvertOperationSchema_HappyPath tests successful conversion with PluginName injection.
func TestConvertOperationSchema_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		pluginID string
		schema   *pluginv1.OperationSchema
		want     *pluginmodel.OperationSchema
	}{
		{
			name:     "simple schema",
			pluginID: "my-plugin",
			schema: &pluginv1.OperationSchema{
				Name:        "echo",
				Description: "Echo operation",
				Inputs: []*pluginv1.InputSchema{
					{
						Name:        "message",
						Type:        "string",
						Required:    true,
						Description: "Message to echo",
					},
				},
				Outputs: []*pluginv1.OutputSchema{
					{
						Name:        "result",
						Type:        "string",
						Description: "Echoed message",
					},
				},
			},
			want: &pluginmodel.OperationSchema{
				Name:        "my-plugin.echo",
				Description: "Echo operation",
				PluginName:  "my-plugin",
				Inputs: map[string]pluginmodel.InputSchema{
					"message": {
						Type:        "string",
						Required:    true,
						Description: "Message to echo",
					},
				},
				Outputs: []string{"result"},
			},
		},
		{
			name:     "schema without inputs",
			pluginID: "other-plugin",
			schema: &pluginv1.OperationSchema{
				Name:        "get-time",
				Description: "Get current time",
				Inputs:      []*pluginv1.InputSchema{},
				Outputs: []*pluginv1.OutputSchema{
					{Name: "timestamp", Type: "int64"},
				},
			},
			want: &pluginmodel.OperationSchema{
				Name:        "other-plugin.get-time",
				Description: "Get current time",
				PluginName:  "other-plugin",
				Inputs:      map[string]pluginmodel.InputSchema{},
				Outputs:     []string{"timestamp"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertOperationSchema(tt.pluginID, tt.schema)

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.Description, got.Description)
			assert.Equal(t, tt.pluginID, got.PluginName, "PluginName should be injected from pluginID parameter")
			assert.Equal(t, tt.want.Inputs, got.Inputs)
			assert.Equal(t, tt.want.Outputs, got.Outputs)
		})
	}
}

// TestConvertOperationSchema_PluginNameInjection ensures pluginID is correctly injected as PluginName.
func TestConvertOperationSchema_PluginNameInjection(t *testing.T) {
	pluginID := "github-plugin"
	schema := &pluginv1.OperationSchema{
		Name: "create-issue",
	}

	result := convertOperationSchema(pluginID, schema)

	require.NotNil(t, result)
	assert.Equal(t, pluginID, result.PluginName, "PluginName must be set to pluginID from context")
}

// TestConvertInputSchema_HappyPath tests conversion of InputSchema messages.
func TestConvertInputSchema_HappyPath(t *testing.T) {
	tests := []struct {
		name   string
		input  *pluginv1.InputSchema
		wantID string
		want   pluginmodel.InputSchema
	}{
		{
			name: "required string input",
			input: &pluginv1.InputSchema{
				Name:        "username",
				Type:        "string",
				Required:    true,
				Description: "User name",
			},
			wantID: "username",
			want: pluginmodel.InputSchema{
				Type:        "string",
				Required:    true,
				Description: "User name",
			},
		},
		{
			name: "optional number with default",
			input: &pluginv1.InputSchema{
				Name:        "timeout",
				Type:        "integer",
				Required:    false,
				Default:     "30",
				Description: "Timeout in seconds",
			},
			wantID: "timeout",
			want: pluginmodel.InputSchema{
				Type:        "integer",
				Required:    false,
				Default:     "30",
				Description: "Timeout in seconds",
			},
		},
		{
			name: "with validation",
			input: &pluginv1.InputSchema{
				Name:        "email",
				Type:        "string",
				Required:    true,
				Validation:  `^[^@]+@[^@]+\.[^@]+$`,
				Description: "Email address",
			},
			wantID: "email",
			want: pluginmodel.InputSchema{
				Type:        "string",
				Required:    true,
				Validation:  `^[^@]+@[^@]+\.[^@]+$`,
				Description: "Email address",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, got := convertInputSchema(tt.input)

			assert.Equal(t, tt.wantID, id, "returned ID should match InputSchema.Name")
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestConvertInputSchema_NilInput tests handling of nil InputSchema.
func TestConvertInputSchema_NilInput(t *testing.T) {
	id, schema := convertInputSchema(nil)

	assert.Equal(t, "", id)
	assert.Equal(t, pluginmodel.InputSchema{}, schema)
}

// TestConvertInputSchema_AllTypes tests conversion of all 5 input types (string, integer, boolean, array, object).
func TestConvertInputSchema_AllTypes(t *testing.T) {
	tests := []struct {
		name   string
		input  *pluginv1.InputSchema
		wantID string
		want   pluginmodel.InputSchema
	}{
		{
			name: "integer type",
			input: &pluginv1.InputSchema{
				Name:        "count",
				Type:        "integer",
				Required:    false,
				Default:     "10",
				Description: "Number of items",
			},
			wantID: "count",
			want: pluginmodel.InputSchema{
				Type:        "integer",
				Required:    false,
				Default:     "10",
				Description: "Number of items",
			},
		},
		{
			name: "boolean type",
			input: &pluginv1.InputSchema{
				Name:        "enabled",
				Type:        "boolean",
				Required:    false,
				Default:     "true",
				Description: "Enable feature",
			},
			wantID: "enabled",
			want: pluginmodel.InputSchema{
				Type:        "boolean",
				Required:    false,
				Default:     "true",
				Description: "Enable feature",
			},
		},
		{
			name: "array type",
			input: &pluginv1.InputSchema{
				Name:        "tags",
				Type:        "array",
				Required:    false,
				Default:     "[\"prod\",\"api\"]",
				Description: "List of tags",
			},
			wantID: "tags",
			want: pluginmodel.InputSchema{
				Type:        "array",
				Required:    false,
				Default:     "[\"prod\",\"api\"]",
				Description: "List of tags",
			},
		},
		{
			name: "object type",
			input: &pluginv1.InputSchema{
				Name:        "config",
				Type:        "object",
				Required:    false,
				Default:     "{\"key\":\"value\"}",
				Description: "Configuration object",
			},
			wantID: "config",
			want: pluginmodel.InputSchema{
				Type:        "object",
				Required:    false,
				Default:     "{\"key\":\"value\"}",
				Description: "Configuration object",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, got := convertInputSchema(tt.input)

			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Required, got.Required)
			assert.Equal(t, tt.want.Default, got.Default)
			assert.Equal(t, tt.want.Description, got.Description)
		})
	}
}

// TestConvertExecuteResponse_ComplexDataMerge tests complex Data field with nested JSON.
func TestConvertExecuteResponse_ComplexDataMerge(t *testing.T) {
	pluginID := "test-plugin"
	resp := &pluginv1.ExecuteResponse{
		Success: true,
		Output:  "processed",
		Data: map[string][]byte{
			"metadata": []byte(`{"version":"1.0","author":"test"}`),
			"items":    []byte(`[1,2,3]`),
			"status":   []byte(`"completed"`),
		},
	}

	result := convertExecuteResponse(pluginID, resp)

	require.NotNil(t, result)
	assert.Equal(t, true, result.Success)
	assert.Equal(t, "processed", result.Outputs["output"])

	// Data entries should be JSON-decoded and available in Outputs
	assert.NotNil(t, result.Outputs["metadata"])
	assert.NotNil(t, result.Outputs["items"])
	assert.NotNil(t, result.Outputs["status"])
}

// TestConvertOperationSchema_MultipleInputsOutputs tests schema with multiple inputs and outputs.
func TestConvertOperationSchema_MultipleInputsOutputs(t *testing.T) {
	pluginID := "github-plugin"
	schema := &pluginv1.OperationSchema{
		Name:        "create-issue",
		Description: "Create a GitHub issue",
		Inputs: []*pluginv1.InputSchema{
			{
				Name:     "title",
				Type:     "string",
				Required: true,
			},
			{
				Name:     "body",
				Type:     "string",
				Required: false,
			},
			{
				Name:     "labels",
				Type:     "array",
				Required: false,
			},
		},
		Outputs: []*pluginv1.OutputSchema{
			{Name: "issue_id", Type: "integer"},
			{Name: "issue_url", Type: "string"},
			{Name: "created_at", Type: "string"},
		},
	}

	result := convertOperationSchema(pluginID, schema)

	require.NotNil(t, result)
	assert.Equal(t, "github-plugin.create-issue", result.Name)
	assert.Equal(t, "Create a GitHub issue", result.Description)
	assert.Equal(t, pluginID, result.PluginName)
	assert.Len(t, result.Inputs, 3, "should have 3 inputs")
	assert.Len(t, result.Outputs, 3, "should have 3 outputs")

	// Verify inputs are present
	assert.Contains(t, result.Inputs, "title")
	assert.Contains(t, result.Inputs, "body")
	assert.Contains(t, result.Inputs, "labels")

	// Verify outputs are present
	assert.Contains(t, result.Outputs, "issue_id")
	assert.Contains(t, result.Outputs, "issue_url")
	assert.Contains(t, result.Outputs, "created_at")
}
