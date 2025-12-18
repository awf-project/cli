package sdk_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vanoix/awf/pkg/plugin/sdk"
)

// customPlugin extends BasePlugin for testing.
type customPlugin struct {
	sdk.BasePlugin
	initCalled     bool
	shutdownCalled bool
	config         map[string]any
}

func (p *customPlugin) Init(ctx context.Context, config map[string]any) error {
	p.initCalled = true
	p.config = config
	return nil
}

func (p *customPlugin) Shutdown(ctx context.Context) error {
	p.shutdownCalled = true
	return nil
}

// TestPluginInterface verifies sdk.Plugin interface compliance.
func TestPluginInterface(t *testing.T) {
	var _ sdk.Plugin = (*sdk.BasePlugin)(nil)
	var _ sdk.Plugin = (*customPlugin)(nil)
}

func TestBasePlugin_Name(t *testing.T) {
	p := &sdk.BasePlugin{
		PluginName:    "test-plugin",
		PluginVersion: "1.0.0",
	}

	assert.Equal(t, "test-plugin", p.Name())
}

func TestBasePlugin_Version(t *testing.T) {
	p := &sdk.BasePlugin{
		PluginName:    "test-plugin",
		PluginVersion: "2.5.0",
	}

	assert.Equal(t, "2.5.0", p.Version())
}

func TestBasePlugin_Init_NoOp(t *testing.T) {
	p := &sdk.BasePlugin{}

	err := p.Init(context.Background(), map[string]any{"key": "value"})

	assert.NoError(t, err)
}

func TestBasePlugin_Shutdown_NoOp(t *testing.T) {
	p := &sdk.BasePlugin{}

	err := p.Shutdown(context.Background())

	assert.NoError(t, err)
}

func TestBasePlugin_EmptyName(t *testing.T) {
	p := &sdk.BasePlugin{}

	assert.Empty(t, p.Name())
}

func TestBasePlugin_EmptyVersion(t *testing.T) {
	p := &sdk.BasePlugin{}

	assert.Empty(t, p.Version())
}

func TestCustomPlugin_Embedding(t *testing.T) {
	p := &customPlugin{
		BasePlugin: sdk.BasePlugin{
			PluginName:    "custom-plugin",
			PluginVersion: "3.0.0",
		},
	}

	assert.Equal(t, "custom-plugin", p.Name())
	assert.Equal(t, "3.0.0", p.Version())
}

func TestCustomPlugin_Init_Override(t *testing.T) {
	p := &customPlugin{
		BasePlugin: sdk.BasePlugin{
			PluginName: "custom-plugin",
		},
	}
	config := map[string]any{"webhook_url": "https://example.com"}

	err := p.Init(context.Background(), config)

	assert.NoError(t, err)
	assert.True(t, p.initCalled)
	assert.Equal(t, config, p.config)
}

func TestCustomPlugin_Shutdown_Override(t *testing.T) {
	p := &customPlugin{
		BasePlugin: sdk.BasePlugin{
			PluginName: "custom-plugin",
		},
	}

	err := p.Shutdown(context.Background())

	assert.NoError(t, err)
	assert.True(t, p.shutdownCalled)
}

func TestBasePlugin_Init_WithNilConfig(t *testing.T) {
	p := &sdk.BasePlugin{}

	err := p.Init(context.Background(), nil)

	assert.NoError(t, err)
}

func TestBasePlugin_Init_WithCancelledContext(t *testing.T) {
	p := &sdk.BasePlugin{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.Init(ctx, nil)

	assert.NoError(t, err)
}

func TestBasePlugin_Shutdown_WithCancelledContext(t *testing.T) {
	p := &sdk.BasePlugin{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.Shutdown(ctx)

	assert.NoError(t, err)
}

// OperationHandler tests
type mockOperationHandler struct {
	lastInputs map[string]any
	outputs    map[string]any
	err        error
}

func (h *mockOperationHandler) Handle(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	h.lastInputs = inputs
	return h.outputs, h.err
}

func TestOperationHandlerInterface(t *testing.T) {
	var _ sdk.OperationHandler = (*mockOperationHandler)(nil)
}

func TestMockOperationHandler_Success(t *testing.T) {
	handler := &mockOperationHandler{
		outputs: map[string]any{"result": "success"},
	}

	outputs, err := handler.Handle(context.Background(), map[string]any{"input": "value"})

	assert.NoError(t, err)
	assert.Equal(t, "success", outputs["result"])
	assert.Equal(t, "value", handler.lastInputs["input"])
}

func TestMockOperationHandler_Error(t *testing.T) {
	handler := &mockOperationHandler{
		err: sdk.ErrNotImplemented,
	}

	_, err := handler.Handle(context.Background(), nil)

	assert.ErrorIs(t, err, sdk.ErrNotImplemented)
}

func TestErrNotImplemented(t *testing.T) {
	assert.NotNil(t, sdk.ErrNotImplemented)
	assert.Equal(t, "not implemented", sdk.ErrNotImplemented.Error())
}

func TestErrInvalidInput(t *testing.T) {
	assert.NotNil(t, sdk.ErrInvalidInput)
	assert.Equal(t, "invalid input", sdk.ErrInvalidInput.Error())
}

func TestErrOperationFailed(t *testing.T) {
	assert.NotNil(t, sdk.ErrOperationFailed)
	assert.Equal(t, "operation failed", sdk.ErrOperationFailed.Error())
}

// OperationResult tests

func TestNewSuccessResult(t *testing.T) {
	data := map[string]any{"key": "value"}
	result := sdk.NewSuccessResult("operation completed", data)

	assert.True(t, result.Success)
	assert.Equal(t, "operation completed", result.Output)
	assert.Equal(t, data, result.Data)
	assert.Empty(t, result.Error)
}

func TestNewSuccessResult_NilData(t *testing.T) {
	result := sdk.NewSuccessResult("done", nil)

	assert.True(t, result.Success)
	assert.Equal(t, "done", result.Output)
	assert.Nil(t, result.Data)
}

func TestNewSuccessResult_EmptyOutput(t *testing.T) {
	result := sdk.NewSuccessResult("", map[string]any{"count": 5})

	assert.True(t, result.Success)
	assert.Empty(t, result.Output)
	assert.Equal(t, 5, result.Data["count"])
}

func TestNewErrorResult(t *testing.T) {
	result := sdk.NewErrorResult("something went wrong")

	assert.False(t, result.Success)
	assert.Equal(t, "something went wrong", result.Error)
	assert.Empty(t, result.Output)
	assert.Nil(t, result.Data)
}

func TestNewErrorResult_EmptyMessage(t *testing.T) {
	result := sdk.NewErrorResult("")

	assert.False(t, result.Success)
	assert.Empty(t, result.Error)
}

func TestNewErrorResultf_WithString(t *testing.T) {
	result := sdk.NewErrorResultf("failed to process %s", "input")

	assert.False(t, result.Success)
	assert.Equal(t, "failed to process input", result.Error)
}

func TestNewErrorResultf_WithInt(t *testing.T) {
	result := sdk.NewErrorResultf("error code: %d", 42)

	assert.False(t, result.Success)
	assert.Equal(t, "error code: 42", result.Error)
}

func TestNewErrorResultf_WithMultipleArgs(t *testing.T) {
	result := sdk.NewErrorResultf("step %s failed with code %d", "validate", 1)

	assert.False(t, result.Success)
	assert.Equal(t, "step validate failed with code 1", result.Error)
}

func TestNewErrorResultf_NoArgs(t *testing.T) {
	result := sdk.NewErrorResultf("static error message")

	assert.False(t, result.Success)
	assert.Equal(t, "static error message", result.Error)
}

func TestOperationResult_ToMap_Success(t *testing.T) {
	result := sdk.NewSuccessResult("completed", map[string]any{"items": 10})
	m := result.ToMap()

	assert.True(t, m["success"].(bool))
	assert.Equal(t, "completed", m["output"])
	assert.Equal(t, map[string]any{"items": 10}, m["data"])
	_, hasError := m["error"]
	assert.False(t, hasError)
}

func TestOperationResult_ToMap_Error(t *testing.T) {
	result := sdk.NewErrorResult("failed")
	m := result.ToMap()

	assert.False(t, m["success"].(bool))
	assert.Equal(t, "", m["output"])
	assert.Equal(t, "failed", m["error"])
	_, hasData := m["data"]
	assert.False(t, hasData)
}

func TestOperationResult_ToMap_NilData(t *testing.T) {
	result := sdk.NewSuccessResult("done", nil)
	m := result.ToMap()

	_, hasData := m["data"]
	assert.False(t, hasData)
}

// InputSchema and OperationSchema tests

func TestInputSchema_Fields(t *testing.T) {
	schema := sdk.InputSchema{
		Type:        sdk.InputTypeString,
		Required:    true,
		Default:     "default_value",
		Description: "A test input",
		Validation:  "url",
	}

	assert.Equal(t, "string", schema.Type)
	assert.True(t, schema.Required)
	assert.Equal(t, "default_value", schema.Default)
	assert.Equal(t, "A test input", schema.Description)
	assert.Equal(t, "url", schema.Validation)
}

func TestRequiredInput(t *testing.T) {
	schema := sdk.RequiredInput(sdk.InputTypeString, "user name")

	assert.Equal(t, sdk.InputTypeString, schema.Type)
	assert.True(t, schema.Required)
	assert.Equal(t, "user name", schema.Description)
	assert.Nil(t, schema.Default)
}

func TestRequiredInput_AllTypes(t *testing.T) {
	tests := []struct {
		inputType string
	}{
		{sdk.InputTypeString},
		{sdk.InputTypeInteger},
		{sdk.InputTypeBoolean},
		{sdk.InputTypeArray},
		{sdk.InputTypeObject},
	}

	for _, tt := range tests {
		t.Run(tt.inputType, func(t *testing.T) {
			schema := sdk.RequiredInput(tt.inputType, "desc")
			assert.Equal(t, tt.inputType, schema.Type)
			assert.True(t, schema.Required)
		})
	}
}

func TestOptionalInput(t *testing.T) {
	schema := sdk.OptionalInput(sdk.InputTypeInteger, "retry count", 3)

	assert.Equal(t, sdk.InputTypeInteger, schema.Type)
	assert.False(t, schema.Required)
	assert.Equal(t, "retry count", schema.Description)
	assert.Equal(t, 3, schema.Default)
}

func TestOptionalInput_NilDefault(t *testing.T) {
	schema := sdk.OptionalInput(sdk.InputTypeObject, "optional object", nil)

	assert.False(t, schema.Required)
	assert.Nil(t, schema.Default)
}

func TestOptionalInput_StringDefault(t *testing.T) {
	schema := sdk.OptionalInput(sdk.InputTypeString, "channel", "#general")

	assert.Equal(t, "#general", schema.Default)
}

func TestOperationSchema_Fields(t *testing.T) {
	schema := sdk.OperationSchema{
		Name:        "slack.send",
		Description: "Send a message to Slack",
		Inputs: map[string]sdk.InputSchema{
			"message": sdk.RequiredInput(sdk.InputTypeString, "message to send"),
			"channel": sdk.OptionalInput(sdk.InputTypeString, "target channel", "#general"),
		},
		Outputs: []string{"message_id", "timestamp"},
	}

	assert.Equal(t, "slack.send", schema.Name)
	assert.Equal(t, "Send a message to Slack", schema.Description)
	assert.Len(t, schema.Inputs, 2)
	assert.True(t, schema.Inputs["message"].Required)
	assert.False(t, schema.Inputs["channel"].Required)
	assert.Equal(t, []string{"message_id", "timestamp"}, schema.Outputs)
}

// Input type constants tests

func TestInputTypeConstants(t *testing.T) {
	assert.Equal(t, "string", sdk.InputTypeString)
	assert.Equal(t, "integer", sdk.InputTypeInteger)
	assert.Equal(t, "boolean", sdk.InputTypeBoolean)
	assert.Equal(t, "array", sdk.InputTypeArray)
	assert.Equal(t, "object", sdk.InputTypeObject)
}

func TestValidInputTypes(t *testing.T) {
	expected := []string{"string", "integer", "boolean", "array", "object"}
	assert.Equal(t, expected, sdk.ValidInputTypes)
}

func TestIsValidInputType_Valid(t *testing.T) {
	validTypes := []string{
		sdk.InputTypeString,
		sdk.InputTypeInteger,
		sdk.InputTypeBoolean,
		sdk.InputTypeArray,
		sdk.InputTypeObject,
	}

	for _, typ := range validTypes {
		t.Run(typ, func(t *testing.T) {
			assert.True(t, sdk.IsValidInputType(typ))
		})
	}
}

func TestIsValidInputType_Invalid(t *testing.T) {
	invalidTypes := []string{"", "str", "int", "bool", "list", "dict", "map", "STRING", "Integer"}

	for _, typ := range invalidTypes {
		t.Run(typ, func(t *testing.T) {
			assert.False(t, sdk.IsValidInputType(typ))
		})
	}
}

// Value extractor tests

func TestGetString_Found(t *testing.T) {
	inputs := map[string]any{"name": "test"}
	val, ok := sdk.GetString(inputs, "name")

	assert.True(t, ok)
	assert.Equal(t, "test", val)
}

func TestGetString_NotFound(t *testing.T) {
	inputs := map[string]any{"other": "value"}
	val, ok := sdk.GetString(inputs, "name")

	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestGetString_WrongType(t *testing.T) {
	inputs := map[string]any{"count": 42}
	val, ok := sdk.GetString(inputs, "count")

	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestGetString_EmptyString(t *testing.T) {
	inputs := map[string]any{"empty": ""}
	val, ok := sdk.GetString(inputs, "empty")

	assert.True(t, ok)
	assert.Empty(t, val)
}

func TestGetString_NilMap(t *testing.T) {
	var inputs map[string]any
	val, ok := sdk.GetString(inputs, "key")

	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestGetStringDefault_Found(t *testing.T) {
	inputs := map[string]any{"channel": "#dev"}
	val := sdk.GetStringDefault(inputs, "channel", "#general")

	assert.Equal(t, "#dev", val)
}

func TestGetStringDefault_NotFound(t *testing.T) {
	inputs := map[string]any{}
	val := sdk.GetStringDefault(inputs, "channel", "#general")

	assert.Equal(t, "#general", val)
}

func TestGetStringDefault_WrongType(t *testing.T) {
	inputs := map[string]any{"channel": 123}
	val := sdk.GetStringDefault(inputs, "channel", "#general")

	assert.Equal(t, "#general", val)
}

func TestGetInt_FoundInt(t *testing.T) {
	inputs := map[string]any{"count": 42}
	val, ok := sdk.GetInt(inputs, "count")

	assert.True(t, ok)
	assert.Equal(t, 42, val)
}

func TestGetInt_FoundInt64(t *testing.T) {
	inputs := map[string]any{"count": int64(100)}
	val, ok := sdk.GetInt(inputs, "count")

	assert.True(t, ok)
	assert.Equal(t, 100, val)
}

func TestGetInt_FoundFloat64(t *testing.T) {
	// JSON unmarshaling produces float64 for numbers
	inputs := map[string]any{"count": float64(50)}
	val, ok := sdk.GetInt(inputs, "count")

	assert.True(t, ok)
	assert.Equal(t, 50, val)
}

func TestGetInt_NotFound(t *testing.T) {
	inputs := map[string]any{}
	val, ok := sdk.GetInt(inputs, "count")

	assert.False(t, ok)
	assert.Equal(t, 0, val)
}

func TestGetInt_WrongType(t *testing.T) {
	inputs := map[string]any{"count": "not a number"}
	val, ok := sdk.GetInt(inputs, "count")

	assert.False(t, ok)
	assert.Equal(t, 0, val)
}

func TestGetInt_Zero(t *testing.T) {
	inputs := map[string]any{"count": 0}
	val, ok := sdk.GetInt(inputs, "count")

	assert.True(t, ok)
	assert.Equal(t, 0, val)
}

func TestGetInt_Negative(t *testing.T) {
	inputs := map[string]any{"offset": -10}
	val, ok := sdk.GetInt(inputs, "offset")

	assert.True(t, ok)
	assert.Equal(t, -10, val)
}

func TestGetIntDefault_Found(t *testing.T) {
	inputs := map[string]any{"retries": 5}
	val := sdk.GetIntDefault(inputs, "retries", 3)

	assert.Equal(t, 5, val)
}

func TestGetIntDefault_NotFound(t *testing.T) {
	inputs := map[string]any{}
	val := sdk.GetIntDefault(inputs, "retries", 3)

	assert.Equal(t, 3, val)
}

func TestGetIntDefault_WrongType(t *testing.T) {
	inputs := map[string]any{"retries": "five"}
	val := sdk.GetIntDefault(inputs, "retries", 3)

	assert.Equal(t, 3, val)
}

func TestGetBool_FoundTrue(t *testing.T) {
	inputs := map[string]any{"enabled": true}
	val, ok := sdk.GetBool(inputs, "enabled")

	assert.True(t, ok)
	assert.True(t, val)
}

func TestGetBool_FoundFalse(t *testing.T) {
	inputs := map[string]any{"enabled": false}
	val, ok := sdk.GetBool(inputs, "enabled")

	assert.True(t, ok)
	assert.False(t, val)
}

func TestGetBool_NotFound(t *testing.T) {
	inputs := map[string]any{}
	val, ok := sdk.GetBool(inputs, "enabled")

	assert.False(t, ok)
	assert.False(t, val)
}

func TestGetBool_WrongType(t *testing.T) {
	inputs := map[string]any{"enabled": "true"}
	val, ok := sdk.GetBool(inputs, "enabled")

	assert.False(t, ok)
	assert.False(t, val)
}

func TestGetBool_WrongTypeInt(t *testing.T) {
	inputs := map[string]any{"enabled": 1}
	val, ok := sdk.GetBool(inputs, "enabled")

	assert.False(t, ok)
	assert.False(t, val)
}

func TestGetBoolDefault_Found(t *testing.T) {
	inputs := map[string]any{"verbose": true}
	val := sdk.GetBoolDefault(inputs, "verbose", false)

	assert.True(t, val)
}

func TestGetBoolDefault_NotFound(t *testing.T) {
	inputs := map[string]any{}
	val := sdk.GetBoolDefault(inputs, "verbose", true)

	assert.True(t, val)
}

func TestGetBoolDefault_WrongType(t *testing.T) {
	inputs := map[string]any{"verbose": "yes"}
	val := sdk.GetBoolDefault(inputs, "verbose", true)

	assert.True(t, val)
}

// OperationProvider interface test

type mockOperationProvider struct {
	operations []string
	results    map[string]*sdk.OperationResult
	errors     map[string]error
}

func (m *mockOperationProvider) Operations() []string {
	return m.operations
}

func (m *mockOperationProvider) HandleOperation(ctx context.Context, name string, inputs map[string]any) (*sdk.OperationResult, error) {
	if err := m.errors[name]; err != nil {
		return nil, err
	}
	if result := m.results[name]; result != nil {
		return result, nil
	}
	return sdk.NewSuccessResult("default", nil), nil
}

func TestOperationProviderInterface(t *testing.T) {
	var _ sdk.OperationProvider = (*mockOperationProvider)(nil)
}

func TestOperationProvider_Operations(t *testing.T) {
	provider := &mockOperationProvider{
		operations: []string{"op1", "op2", "op3"},
	}

	ops := provider.Operations()

	assert.Equal(t, []string{"op1", "op2", "op3"}, ops)
}

func TestOperationProvider_HandleOperation_Success(t *testing.T) {
	provider := &mockOperationProvider{
		results: map[string]*sdk.OperationResult{
			"test.op": sdk.NewSuccessResult("done", map[string]any{"id": "123"}),
		},
	}

	result, err := provider.HandleOperation(context.Background(), "test.op", nil)

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "done", result.Output)
}

func TestOperationProvider_HandleOperation_Error(t *testing.T) {
	provider := &mockOperationProvider{
		errors: map[string]error{
			"failing.op": sdk.ErrOperationFailed,
		},
	}

	result, err := provider.HandleOperation(context.Background(), "failing.op", nil)

	assert.ErrorIs(t, err, sdk.ErrOperationFailed)
	assert.Nil(t, result)
}
