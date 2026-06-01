package acp

import (
	"context"
	"fmt"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

type SessionNotifier interface {
	NotifySessionUpdate(ctx context.Context, workflowID string, update SessionUpdate) error
}

type SessionUpdate struct {
	Kind     string
	StepName string
	Error    string
	Duration string
	Metadata map[string]string
}

type WorkflowEventProjector struct {
	notifier SessionNotifier
	logger   ports.Logger
}

var _ ports.EventPublisher = (*WorkflowEventProjector)(nil)

func NewWorkflowEventProjector(notifier SessionNotifier, logger ports.Logger) *WorkflowEventProjector {
	return &WorkflowEventProjector{
		notifier: notifier,
		logger:   logger,
	}
}

func (p *WorkflowEventProjector) Publish(ctx context.Context, event *pluginmodel.DomainEvent) error {
	if event == nil {
		p.logger.Warn("acp projector: nil event dropped")
		return nil
	}
	workflowID, stepName, ok := extractWorkflowMeta(event)
	if !ok {
		return nil
	}

	var update SessionUpdate
	switch event.Type {
	case workflow.EventWorkflowStarted:
		update = SessionUpdate{Kind: "workflow_started"}
	case workflow.EventWorkflowCompleted:
		update = SessionUpdate{Kind: "workflow_completed", Duration: event.Metadata["duration_ms"]}
	case workflow.EventWorkflowFailed:
		update = SessionUpdate{Kind: "workflow_failed", Error: event.Metadata["error"]}
	case workflow.EventStepStarted:
		update = SessionUpdate{Kind: "step_started", StepName: stepName}
	case workflow.EventStepCompleted:
		update = SessionUpdate{Kind: "step_completed", StepName: stepName}
	case workflow.EventStepFailed:
		update = SessionUpdate{Kind: "step_failed", StepName: stepName, Error: event.Metadata["error"]}
	case workflow.EventStepRetrying:
		update = SessionUpdate{Kind: "step_retrying", StepName: stepName}
	default:
		p.logger.Debug("acp projector: unhandled event type", "type", event.Type)
		return nil
	}

	if err := p.notifier.NotifySessionUpdate(ctx, workflowID, update); err != nil {
		p.logger.Warn("notify session update failed", "workflow_id", workflowID, "event", event.Type, "error", err)
		return fmt.Errorf("acp projector: notify session update: %w", err)
	}
	return nil
}

func (p *WorkflowEventProjector) Close() error {
	return nil
}

func extractWorkflowMeta(event *pluginmodel.DomainEvent) (workflowID, stepName string, ok bool) {
	workflowID = event.Metadata["workflow_id"]
	if workflowID == "" {
		return "", "", false
	}
	return workflowID, event.Metadata["step_name"], true
}
