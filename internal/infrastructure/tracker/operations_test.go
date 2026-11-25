package tracker

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperationSchemasExposeStableProviderIndependentContract(t *testing.T) {
	tests := []struct {
		name    string
		inputs  map[string]pluginmodel.InputSchema
		outputs []string
	}{
		{
			name: "tracker.create_issue",
			inputs: trackerInputs(
				requiredString("title"),
				optionalStrings("backend", "project", "team", "type", "description", "priority", "assignee"),
				optionalArray("labels"),
			),
			outputs: []string{"id", "key", "url"},
		},
		{
			name: "tracker.update_issue",
			inputs: trackerInputs(
				optionalStrings("backend", "id", "key", "title", "description", "priority", "assignee", "state"),
				optionalArray("labels"),
			),
			outputs: []string{"id", "key"},
		},
		{
			name: "tracker.transition",
			inputs: trackerInputs(
				requiredString("transition"),
				optionalStrings("backend", "id", "key"),
			),
			outputs: []string{"id", "key", "status"},
		},
		{
			name: "tracker.add_comment",
			inputs: trackerInputs(
				requiredString("body"),
				optionalStrings("backend", "id", "key"),
			),
			outputs: []string{"comment_id"},
		},
		{
			name:    "tracker.get_issue",
			inputs:  trackerInputs(optionalStrings("backend", "id", "key")),
			outputs: []string{"id", "key", "title", "status", "description", "assignee", "url"},
		},
		{
			name: "tracker.search",
			inputs: trackerInputs(
				requiredString("query"),
				optionalStrings("backend", "project", "team", "state"),
				optionalInteger("max_results"),
			),
			outputs: []string{"issues", "total"},
		},
	}

	schemas := OperationSchemas()
	require.Len(t, schemas, len(tests))

	byName := make(map[string]*pluginmodel.OperationSchema, len(schemas))
	for _, schema := range schemas {
		require.NotNil(t, schema)
		require.NoError(t, schema.Validate())
		assert.Equal(t, "tracker", schema.PluginName)
		assert.NotEmpty(t, schema.Description)
		_, duplicate := byName[schema.Name]
		assert.False(t, duplicate, "duplicate operation schema %q", schema.Name)
		byName[schema.Name] = schema
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, ok := byName[tt.name]
			require.True(t, ok)
			require.Len(t, schema.Inputs, len(tt.inputs))
			for name, expected := range tt.inputs {
				actual, exists := schema.Inputs[name]
				require.True(t, exists, "missing input %q", name)
				assert.Equal(t, expected.Type, actual.Type)
				assert.Equal(t, expected.Required, actual.Required)
				assert.NotEmpty(t, actual.Description)
			}
			assert.Equal(t, tt.outputs, schema.Outputs)
		})
	}
}

func TestAdapterOperationNameDerivesCanonicalAdapterTarget(t *testing.T) {
	for _, operation := range []string{
		"tracker.create_issue",
		"tracker.update_issue",
		"tracker.transition",
		"tracker.add_comment",
		"tracker.get_issue",
		"tracker.search",
	} {
		t.Run(operation, func(t *testing.T) {
			name, err := AdapterOperationName("jira", operation)
			require.NoError(t, err)
			assert.Equal(t, "tracker-jira."+operation[len("tracker."):], name)
		})
	}
}

func TestAdapterOperationNameRejectsInvalidNamespaceAndOperationWithoutEchoingValues(t *testing.T) {
	tests := []struct {
		name      string
		backend   string
		operation string
		field     string
		secret    string
	}{
		{name: "empty backend", operation: "tracker.search", field: "backend"},
		{name: "dotted backend", backend: "private.secret", operation: "tracker.search", field: "backend", secret: "private.secret"},
		{name: "backend containing whitespace", backend: "private secret", operation: "tracker.search", field: "backend", secret: "private secret"},
		{name: "unknown operation", backend: "jira", operation: "tracker.private_secret", field: "operation", secret: "tracker.private_secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, err := AdapterOperationName(tt.backend, tt.operation)
			assert.Empty(t, name)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidTrackerInput)
			assert.Contains(t, err.Error(), tt.field)
			if tt.secret != "" {
				assert.NotContains(t, err.Error(), tt.secret)
			}
		})
	}
}

type trackerInputOption func(map[string]pluginmodel.InputSchema)

func trackerInputs(options ...trackerInputOption) map[string]pluginmodel.InputSchema {
	inputs := make(map[string]pluginmodel.InputSchema)
	for _, option := range options {
		option(inputs)
	}
	return inputs
}

func requiredString(name string) trackerInputOption {
	return func(inputs map[string]pluginmodel.InputSchema) {
		inputs[name] = pluginmodel.InputSchema{Type: pluginmodel.InputTypeString, Required: true}
	}
}

func optionalStrings(names ...string) trackerInputOption {
	return func(inputs map[string]pluginmodel.InputSchema) {
		for _, name := range names {
			inputs[name] = pluginmodel.InputSchema{Type: pluginmodel.InputTypeString}
		}
	}
}

func optionalArray(name string) trackerInputOption {
	return func(inputs map[string]pluginmodel.InputSchema) {
		inputs[name] = pluginmodel.InputSchema{Type: pluginmodel.InputTypeArray}
	}
}

func optionalInteger(name string) trackerInputOption {
	return func(inputs map[string]pluginmodel.InputSchema) {
		inputs[name] = pluginmodel.InputSchema{Type: pluginmodel.InputTypeInteger}
	}
}
