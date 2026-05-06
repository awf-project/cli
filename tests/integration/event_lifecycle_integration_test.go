//go:build integration

package integration_test

// Feature: F090

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowExecution_EmitsLifecycleEventsToSubscribers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	logger := &testEventLogger{}
	bus := pluginmgr.NewEventBus(logger)
	defer bus.Close()

	collector := &eventCollector{}
	bus.Subscribe("test-plugin", []string{"workflow.*", "step.*"}, collector)

	svc := buildExecutionServiceWithPublisher(t, bus)

	wf := &workflow.Workflow{
		Name:    "lifecycle-test",
		Initial: "run",
		Steps: map[string]*workflow.Step{
			"run": {
				Name:      "run",
				Type:      workflow.StepTypeCommand,
				Command:   "echo ok",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	repo := testmocks.NewMockWorkflowRepository()
	repo.AddWorkflow("lifecycle-test", wf)
	svc = buildExecutionServiceWithRepoAndPublisher(t, repo, bus)

	execCtx, err := svc.Run(context.Background(), "lifecycle-test", nil)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	events := collector.getEvents()
	require.GreaterOrEqual(t, len(events), 3, "expected at least workflow.started + step.started + step.completed + workflow.completed")

	typeSeq := make([]string, len(events))
	for i, e := range events {
		typeSeq[i] = e.Type
	}

	assert.Contains(t, typeSeq, workflow.EventWorkflowStarted)
	assert.Contains(t, typeSeq, workflow.EventStepStarted)
	assert.Contains(t, typeSeq, workflow.EventWorkflowCompleted)

	for _, e := range events {
		assert.Equal(t, "core", e.Source)
		assert.NotEmpty(t, e.ID)
		assert.NotEmpty(t, e.Metadata["workflow_id"])
		assert.Equal(t, execCtx.WorkflowID, e.Metadata["workflow_id"])
	}
}

func TestInterPluginEventRouting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger := &testEventLogger{}
	bus := pluginmgr.NewEventBus(logger)
	defer bus.Close()

	pluginA := &emittingSubscriber{
		emitType: "plugin-a.processed",
		emitMeta: map[string]string{"origin": "plugin-a"},
	}
	pluginB := &eventCollector{}

	bus.Subscribe("plugin-a", []string{"workflow.completed"}, pluginA)
	bus.Subscribe("plugin-b", []string{"plugin-a.*"}, pluginB)

	trigger := pluginmodel.NewDomainEvent(workflow.EventWorkflowCompleted, "core", map[string]string{"wf": "demo"}, nil)
	err := bus.Publish(ctx, trigger)
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	received := pluginB.getEvents()
	require.Len(t, received, 1, "plugin-b should receive the event emitted by plugin-a")
	assert.Equal(t, "plugin-a.processed", received[0].Type)
	assert.Equal(t, "plugin-a", received[0].Metadata["origin"])
	assert.Equal(t, 1, received[0].PropagationDepth)
}

func TestWorkflowExecution_MasksSecretsBeforeDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	logger := &testEventLogger{}
	bus := pluginmgr.NewEventBus(logger)
	defer bus.Close()

	collector := &eventCollector{}
	bus.Subscribe("spy", []string{"workflow.*"}, collector)

	repo := testmocks.NewMockWorkflowRepository()
	wf := &workflow.Workflow{
		Name:    "secret-test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeTerminal,
			},
		},
	}
	repo.AddWorkflow("secret-test", wf)

	svc := buildExecutionServiceWithRepoAndPublisher(t, repo, bus)

	inputs := map[string]any{
		"SECRET_TOKEN": "super-secret-value",
		"API_KEY":      "key-12345",
		"normal_var":   "visible",
	}

	_, err := svc.Run(context.Background(), "secret-test", inputs)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	events := collector.getEvents()
	for _, e := range events {
		if val, ok := e.Metadata["SECRET_TOKEN"]; ok {
			assert.Equal(t, "***", val, "SECRET_TOKEN must be masked")
		}
		if val, ok := e.Metadata["API_KEY"]; ok {
			assert.Equal(t, "***", val, "API_KEY must be masked")
		}
		if val, ok := e.Metadata["normal_var"]; ok {
			assert.Equal(t, "visible", val, "normal_var should not be masked")
		}
	}
}

func TestWorkflowExecution_SucceedsWhenNoSubscribers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	logger := &testEventLogger{}
	bus := pluginmgr.NewEventBus(logger)
	defer bus.Close()

	repo := testmocks.NewMockWorkflowRepository()
	wf := &workflow.Workflow{
		Name:    "no-subs",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeTerminal,
			},
		},
	}
	repo.AddWorkflow("no-subs", wf)

	svc := buildExecutionServiceWithRepoAndPublisher(t, repo, bus)

	execCtx, err := svc.Run(context.Background(), "no-subs", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

// --- helpers ---

func buildExecutionServiceWithPublisher(t *testing.T, publisher ports.EventPublisher) *application.ExecutionService {
	t.Helper()
	repo := testmocks.NewMockWorkflowRepository()
	return buildExecutionServiceWithRepoAndPublisher(t, repo, publisher)
}

func buildExecutionServiceWithRepoAndPublisher(t *testing.T, repo ports.WorkflowRepository, publisher ports.EventPublisher) *application.ExecutionService {
	t.Helper()

	store := testmocks.NewMockStateStore()
	executor := testmocks.NewMockCommandExecutor()
	logger := testmocks.NewMockLogger()

	executor.SetCommandResult("", &ports.CommandResult{ExitCode: 0})
	executor.SetCommandResult("echo ok", &ports.CommandResult{Stdout: "ok\n", ExitCode: 0})

	wfSvc := application.NewWorkflowService(repo, store, executor, logger, nil)
	parallelExec := application.NewParallelExecutor(logger)
	resolver := interpolation.NewTemplateResolver()
	historyStore := testmocks.NewMockHistoryStore()
	historySvc := application.NewHistoryService(historyStore, logger)

	svc := application.NewExecutionService(wfSvc, executor, parallelExec, store, logger, resolver, historySvc)
	svc.SetEventPublisher(publisher)
	svc.SetAuditTrailWriter(testmocks.NewMockAuditTrailWriter())

	return svc
}

type eventCollector struct {
	mu     sync.Mutex
	events []*pluginmodel.DomainEvent
}

func (c *eventCollector) DeliverEvent(_ context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
	return nil, nil
}

func (c *eventCollector) getEvents() []*pluginmodel.DomainEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]*pluginmodel.DomainEvent, len(c.events))
	copy(result, c.events)
	return result
}

type emittingSubscriber struct {
	emitType string
	emitMeta map[string]string
}

func (s *emittingSubscriber) DeliverEvent(_ context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
	emitted := &pluginmodel.DomainEvent{
		ID:               "emitted-" + event.ID,
		Type:             s.emitType,
		Timestamp:        event.Timestamp,
		Source:           "plugin-a",
		Metadata:         s.emitMeta,
		PropagationDepth: event.PropagationDepth + 1,
	}
	return []*pluginmodel.DomainEvent{emitted}, nil
}
