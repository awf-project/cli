package notifier

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifierSchemasExposeProviderNeutralContracts(t *testing.T) {
	tests := []struct {
		name     string
		inputs   map[string]pluginmodel.InputSchema
		outputs  []string
		required []string
	}{
		{
			name: OperationSend,
			inputs: map[string]pluginmodel.InputSchema{
				"backend":   {Type: pluginmodel.InputTypeString, Required: true},
				"channel":   {Type: pluginmodel.InputTypeString, Required: true},
				"text":      {Type: pluginmodel.InputTypeString, Required: true},
				"rich":      {Type: pluginmodel.InputTypeObject},
				"thread_id": {Type: pluginmodel.InputTypeString},
			},
			outputs:  []string{"message_id", "channel"},
			required: []string{"backend", "channel", "text"},
		},
		{
			name: OperationUpdate,
			inputs: map[string]pluginmodel.InputSchema{
				"backend":    {Type: pluginmodel.InputTypeString, Required: true},
				"channel":    {Type: pluginmodel.InputTypeString, Required: true},
				"message_id": {Type: pluginmodel.InputTypeString, Required: true},
				"text":       {Type: pluginmodel.InputTypeString, Required: true},
				"rich":       {Type: pluginmodel.InputTypeObject},
			},
			outputs:  []string{"message_id"},
			required: []string{"backend", "channel", "message_id", "text"},
		},
		{
			name: OperationReact,
			inputs: map[string]pluginmodel.InputSchema{
				"backend":    {Type: pluginmodel.InputTypeString, Required: true},
				"channel":    {Type: pluginmodel.InputTypeString, Required: true},
				"message_id": {Type: pluginmodel.InputTypeString, Required: true},
				"emoji":      {Type: pluginmodel.InputTypeString, Required: true},
			},
			outputs:  []string{"ok"},
			required: []string{"backend", "channel", "message_id", "emoji"},
		},
		{
			name: OperationUpload,
			inputs: map[string]pluginmodel.InputSchema{
				"backend":  {Type: pluginmodel.InputTypeString, Required: true},
				"channel":  {Type: pluginmodel.InputTypeString, Required: true},
				"filename": {Type: pluginmodel.InputTypeString, Required: true},
				"content":  {Type: pluginmodel.InputTypeString, Required: true},
				"title":    {Type: pluginmodel.InputTypeString},
			},
			outputs:  []string{"file_id", "permalink"},
			required: []string{"backend", "channel", "filename", "content"},
		},
	}

	provider := NewProvider(nil, "", "")
	listed := provider.ListOperations()
	require.Len(t, listed, len(tests))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema, found := provider.GetOperation(test.name)
			require.True(t, found)
			require.NotNil(t, schema)
			assert.Equal(t, test.name, schema.Name)
			require.Len(t, schema.Inputs, len(test.inputs))
			for inputName, expected := range test.inputs {
				actual, exists := schema.Inputs[inputName]
				require.True(t, exists, inputName)
				assert.Equal(t, expected.Type, actual.Type, inputName)
				assert.Equal(t, expected.Required, actual.Required, inputName)
			}
			assert.Equal(t, test.outputs, schema.Outputs)
			assert.ElementsMatch(t, test.required, schema.GetRequiredInputs())
		})
	}

	withDefaultChannel := NewProvider(nil, "", "alerts")
	for _, schema := range withDefaultChannel.ListOperations() {
		assert.False(t, schema.Inputs["channel"].Required, schema.Name)
	}
	withDefaultBackend := NewProvider(nil, "slack", "")
	for _, schema := range withDefaultBackend.ListOperations() {
		backend := schema.Inputs["backend"]
		assert.False(t, backend.Required, schema.Name)
		assert.Equal(t, "slack", backend.Default, schema.Name)
		assert.Equal(t, pluginmodel.ValidationRuleNotifierBackend, backend.Validation, schema.Name)
	}

	unknown, found := provider.GetOperation("notifier.unknown")
	assert.False(t, found)
	assert.Nil(t, unknown)
}
