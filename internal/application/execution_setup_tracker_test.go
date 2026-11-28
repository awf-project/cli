package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/tracker"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackerIntegrationAdapter struct {
	calls   int
	lookups []string
}

func (a *trackerIntegrationAdapter) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	a.lookups = append(a.lookups, name)
	if name != "tracker-jira.create_issue" {
		return nil, false
	}
	return &pluginmodel.OperationSchema{Name: name, PluginName: "tracker-jira"}, true
}

func (a *trackerIntegrationAdapter) ListOperations() []*pluginmodel.OperationSchema {
	schema, _ := a.GetOperation("tracker-jira.create_issue")
	return []*pluginmodel.OperationSchema{schema}
}

func (a *trackerIntegrationAdapter) Execute(
	_ context.Context,
	name string,
	inputs map[string]any,
) (*pluginmodel.OperationResult, error) {
	a.calls++
	if name != "tracker-jira.create_issue" || inputs["title"] != "Release blocker" {
		return nil, assert.AnError
	}
	return &pluginmodel.OperationResult{Success: true, Outputs: map[string]any{
		"id": "provider-id", "key": "TEAM-7", "url": "https://tracker.invalid/TEAM-7",
	}}, nil
}

var _ ports.OperationProvider = (*trackerIntegrationAdapter)(nil)

func TestExecutionSetupComposesTrackerWithExternalAdapters(t *testing.T) {
	repository := testmocks.NewMockWorkflowRepository()
	repository.AddWorkflow("tracker-create", &workflow.Workflow{
		Name: "tracker-create", Initial: "create",
		Steps: map[string]*workflow.Step{
			"create": {
				Name: "create", Type: workflow.StepTypeOperation, Operation: "tracker.create_issue",
				OperationInputs: map[string]any{"title": "Release blocker"}, OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	})
	adapter := &trackerIntegrationAdapter{}
	setup := application.NewExecutionSetup(
		repository,
		testmocks.NewMockStateStore(),
		testmocks.NewMockCommandExecutor(),
		testmocks.NewMockLogger(),
		application.WithTrackerConfig(application.TrackerConfig{DefaultBackend: "jira"}),
		application.WithPluginProviders(application.PluginProviders{Operations: adapter}),
	)
	built, err := setup.Build(context.Background())
	require.NoError(t, err)
	defer built.Cleanup()

	execution, err := built.ExecService.RunWithWorkflowAndRunID(
		context.Background(),
		mustLoadWorkflow(t, repository, "tracker-create"),
		nil,
		"tracker-create-run",
	)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execution.Status)
	require.Contains(t, execution.States, "create")
	assert.Contains(t, execution.States["create"].Output, "TEAM-7")
	assert.Contains(t, execution.States["create"].Output, "https://tracker.invalid/TEAM-7")
	assert.Equal(t, 1, adapter.calls)
	assert.NotContains(t, adapter.lookups, "tracker.tracker.create_issue")

	for _, operation := range []string{
		"tracker.create_issue", "tracker.update_issue", "tracker.transition",
		"tracker.add_comment", "tracker.get_issue", "tracker.search",
	} {
		wf := &workflow.Workflow{
			Name: "schema-check", Initial: "operation",
			Steps: map[string]*workflow.Step{
				"operation": {
					Name: "operation", Type: workflow.StepTypeOperation, Operation: operation,
					OperationInputs: trackerSchemaInputs(operation), OnSuccess: "done",
				},
				"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			},
		}
		assert.NoError(t, built.WorkflowSvc.ValidateLoadedWorkflow(
			context.Background(), wf, wf.Name, ports.ValidateOptions{},
		))
	}

	githubWorkflow := &workflow.Workflow{
		Name: "github-schema-check", Initial: "operation",
		Steps: map[string]*workflow.Step{
			"operation": {
				Name: "operation", Type: workflow.StepTypeOperation, Operation: "github.get_issue", OnSuccess: "done",
				OperationInputs: map[string]any{"owner": "awf-project", "repo": "cli", "number": 1},
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
	assert.NoError(t, built.WorkflowSvc.ValidateLoadedWorkflow(
		context.Background(), githubWorkflow, githubWorkflow.Name, ports.ValidateOptions{},
	))
}

func TestExecutionSetupTrackerRequiresBackendWhenConfigurationIsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		option application.SetupOption
	}{
		{name: "option omitted"},
		{
			name:   "explicit zero value",
			option: application.WithTrackerConfig(application.TrackerConfig{}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setup := buildTrackerSetup(test.option)
			built, err := setup.Build(context.Background())
			require.NoError(t, err)
			defer built.Cleanup()

			_, err = built.ExecService.RunWithWorkflowAndRunID(
				context.Background(),
				trackerCreateWorkflow(map[string]any{"title": "Release blocker"}),
				nil,
				"tracker-zero-config-run",
			)

			assert.ErrorIs(t, err, tracker.ErrBackendRequired)
			assert.False(t, errors.Is(err, tracker.ErrAdapterUnavailable))
		})
	}
}

func buildTrackerSetup(option application.SetupOption) *application.ExecutionSetup {
	if option == nil {
		return application.NewExecutionSetup(
			testmocks.NewMockWorkflowRepository(),
			testmocks.NewMockStateStore(),
			testmocks.NewMockCommandExecutor(),
			testmocks.NewMockLogger(),
		)
	}
	return application.NewExecutionSetup(
		testmocks.NewMockWorkflowRepository(),
		testmocks.NewMockStateStore(),
		testmocks.NewMockCommandExecutor(),
		testmocks.NewMockLogger(),
		option,
	)
}

func TestExecutionSetupTrackerReportsUnavailableExternalAdapter(t *testing.T) {
	setup := application.NewExecutionSetup(
		testmocks.NewMockWorkflowRepository(),
		testmocks.NewMockStateStore(),
		testmocks.NewMockCommandExecutor(),
		testmocks.NewMockLogger(),
		application.WithTrackerConfig(application.TrackerConfig{DefaultBackend: "jira"}),
	)
	built, err := setup.Build(context.Background())
	require.NoError(t, err)
	defer built.Cleanup()

	_, err = built.ExecService.RunWithWorkflowAndRunID(
		context.Background(),
		trackerCreateWorkflow(map[string]any{"title": "Release blocker"}),
		nil,
		"tracker-unavailable-run",
	)

	assert.ErrorIs(t, err, tracker.ErrAdapterUnavailable)
}

func trackerCreateWorkflow(inputs map[string]any) *workflow.Workflow {
	return &workflow.Workflow{
		Name: "tracker-create", Initial: "create",
		Steps: map[string]*workflow.Step{
			"create": {
				Name: "create", Type: workflow.StepTypeOperation, Operation: "tracker.create_issue",
				OperationInputs: inputs, OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
}

func trackerSchemaInputs(operation string) map[string]any {
	switch operation {
	case "tracker.create_issue":
		return map[string]any{"title": "schema title"}
	case "tracker.transition":
		return map[string]any{"id": "schema-id", "transition": "schema transition"}
	case "tracker.add_comment":
		return map[string]any{"id": "schema-id", "body": "schema body"}
	case "tracker.search":
		return map[string]any{"query": "schema query"}
	default:
		return map[string]any{"id": "schema-id"}
	}
}

func mustLoadWorkflow(
	t *testing.T,
	repository ports.WorkflowRepository,
	name string,
) *workflow.Workflow {
	t.Helper()
	loaded, err := repository.Load(context.Background(), name)
	require.NoError(t, err)
	return loaded
}
