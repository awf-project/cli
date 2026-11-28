package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/tracker"
	"github.com/awf-project/cli/internal/interfaces/notifierwiring"
)

var errUnexpectedExternalExecution = errors.New("external operation execution is not part of validation composition")

type validationExternalOperationProvider struct {
	operation  *pluginmodel.OperationSchema
	executions int
}

func (p *validationExternalOperationProvider) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	if p.operation != nil && p.operation.Name == name {
		return p.operation, true
	}

	return nil, false
}

func (p *validationExternalOperationProvider) ListOperations() []*pluginmodel.OperationSchema {
	if p.operation == nil {
		return nil
	}

	return []*pluginmodel.OperationSchema{p.operation}
}

func (p *validationExternalOperationProvider) Execute(
	context.Context,
	string,
	map[string]any,
) (*pluginmodel.OperationResult, error) {
	p.executions++
	return nil, errUnexpectedExternalExecution
}

func TestValidationProviderComposesNotifierWithDiscoveredAdapters(t *testing.T) {
	external := &validationExternalOperationProvider{operation: &pluginmodel.OperationSchema{
		Name:       "recording.send",
		PluginName: "recording",
	}}
	pluginService := application.NewPluginService(nil, nil, ports.NopLogger{})

	provider := buildValidationOperationProvider(pluginService, external, ports.NopLogger{}, notifierwiring.Settings{})

	for _, name := range []string{
		"notifier.send",
		"notifier.update",
		"notifier.react",
		"notifier.upload",
		"recording.send",
	} {
		operation, found := provider.GetOperation(name)
		require.True(t, found, "operation %q should be available during validation", name)
		require.NotNil(t, operation)
		assert.Equal(t, name, operation.Name)
	}
	assert.Zero(t, external.executions)
}

func TestValidationProviderRoutesNotifierOnlyToDiscoveredAdapters(t *testing.T) {
	external := &validationExternalOperationProvider{operation: &pluginmodel.OperationSchema{
		Name:       "notify.send",
		PluginName: "notify",
	}}
	pluginService := application.NewPluginService(nil, nil, ports.NopLogger{})
	provider := buildValidationOperationProvider(pluginService, external, ports.NopLogger{}, notifierwiring.Settings{})

	result, err := provider.Execute(context.Background(), "notifier.send", map[string]any{
		"backend": "notify",
		"channel": "alerts",
		"text":    "routing-canary",
	})

	require.ErrorIs(t, err, errUnexpectedExternalExecution)
	assert.Nil(t, result)
	assert.Equal(t, 1, external.executions)
}

func TestBuildValidationOperationProvider_IncludesEnabledBuiltins(t *testing.T) {
	pluginService := application.NewPluginService(nil, nil, ports.NopLogger{})
	provider := buildValidationOperationProvider(pluginService, nil, ports.NopLogger{}, notifierwiring.Settings{})
	require.NotNil(t, provider, "validation composition should return an operation provider")

	expectedOwners := map[string]string{
		"github.get_issue":     "github",
		"github.get_pr":        "github",
		"github.create_pr":     "github",
		"github.create_issue":  "github",
		"github.add_labels":    "github",
		"github.list_comments": "github",
		"github.add_comment":   "github",
		"github.batch":         "github",
		"notify.send":          "notify",
		"http.request":         "http",
		"notifier.send":        "notifier",
		"notifier.update":      "notifier",
		"notifier.react":       "notifier",
		"notifier.upload":      "notifier",
		"tracker.create_issue": "tracker",
		"tracker.update_issue": "tracker",
		"tracker.transition":   "tracker",
		"tracker.add_comment":  "tracker",
		"tracker.get_issue":    "tracker",
		"tracker.search":       "tracker",
	}

	listed := provider.ListOperations()
	require.Len(t, listed, len(expectedOwners))
	for _, operation := range listed {
		require.NotNil(t, operation)
		expectedOwner, found := expectedOwners[operation.Name]
		require.True(t, found, "unexpected operation %q", operation.Name)
		assert.Equal(t, expectedOwner, operation.PluginName, "operation %q owner", operation.Name)
	}

	for name, owner := range expectedOwners {
		operation, found := provider.GetOperation(name)
		require.True(t, found, "operation %q should be available by lookup", name)
		require.NotNil(t, operation)
		assert.Equal(t, name, operation.Name)
		assert.Equal(t, owner, operation.PluginName, "operation %q owner", name)
	}
}

func TestBuildValidationOperationProvider_IncludesDiscoveredExternalProvider(t *testing.T) {
	externalOperation := &pluginmodel.OperationSchema{
		Name:       "example.inspect",
		PluginName: "example",
	}
	external := &validationExternalOperationProvider{operation: externalOperation}
	pluginService := application.NewPluginService(nil, nil, ports.NopLogger{})

	provider := buildValidationOperationProvider(pluginService, external, ports.NopLogger{}, notifierwiring.Settings{})
	require.NotNil(t, provider, "validation composition should return an operation provider")

	assert.Contains(t, provider.ListOperations(), externalOperation)
	operation, found := provider.GetOperation(externalOperation.Name)
	require.True(t, found)
	assert.Same(t, externalOperation, operation)
}

func TestBuildValidationOperationProvider_ExcludesDisabledBuiltin(t *testing.T) {
	stateStore := pluginmgr.NewJSONPluginStateStore(t.TempDir())
	require.NoError(t, stateStore.SetEnabled(context.Background(), "github", false))
	pluginService := application.NewPluginService(nil, stateStore, ports.NopLogger{})

	provider := buildValidationOperationProvider(pluginService, nil, ports.NopLogger{}, notifierwiring.Settings{})
	require.NotNil(t, provider, "validation composition should return an operation provider")

	for _, operation := range provider.ListOperations() {
		require.NotNil(t, operation)
		assert.NotEqual(t, "github", operation.PluginName)
		assert.NotContains(t, operation.Name, "github.")
	}
	for _, name := range []string{
		"github.get_issue",
		"github.get_pr",
		"github.create_pr",
		"github.create_issue",
		"github.add_labels",
		"github.list_comments",
		"github.add_comment",
		"github.batch",
	} {
		operation, found := provider.GetOperation(name)
		assert.False(t, found, "disabled operation %q should not be available", name)
		assert.Nil(t, operation)
	}
}

func TestBuildValidationOperationProvider_ExcludesDisabledTracker(t *testing.T) {
	stateStore := pluginmgr.NewJSONPluginStateStore(t.TempDir())
	require.NoError(t, stateStore.SetEnabled(context.Background(), "tracker", false))
	pluginService := application.NewPluginService(nil, stateStore, ports.NopLogger{})

	provider := buildValidationOperationProvider(
		pluginService,
		nil,
		ports.NopLogger{},
		notifierwiring.Settings{},
	)
	require.NotNil(t, provider)

	for _, schema := range tracker.OperationSchemas() {
		operation, found := provider.GetOperation(schema.Name)
		assert.False(t, found, "disabled operation %q should not be available", schema.Name)
		assert.Nil(t, operation)
	}
}

func TestValidationProviderMatchesRuntimeTrackerOperations(t *testing.T) {
	externalOperation := &pluginmodel.OperationSchema{
		Name:       "tracker-jira.create_issue",
		PluginName: "tracker-jira",
	}
	external := &validationExternalOperationProvider{operation: externalOperation}
	pluginService := application.NewPluginService(nil, nil, ports.NopLogger{})

	validationProvider := buildValidationOperationProvider(
		pluginService,
		external,
		ports.NopLogger{},
		notifierwiring.Settings{},
	)
	runtimeProvider := tracker.NewProvider(external, "jira", ports.NopLogger{})

	for _, schema := range tracker.OperationSchemas() {
		validationSchema, found := validationProvider.GetOperation(schema.Name)
		require.True(t, found, "validation operation %q", schema.Name)
		runtimeSchema, found := runtimeProvider.GetOperation(schema.Name)
		require.True(t, found, "runtime operation %q", schema.Name)
		assert.Equal(t, runtimeSchema, validationSchema)
	}

	for _, name := range []string{"github.get_issue", externalOperation.Name} {
		_, found := validationProvider.GetOperation(name)
		assert.True(t, found, "preserved operation %q", name)
	}
}
