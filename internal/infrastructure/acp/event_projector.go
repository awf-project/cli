package acp

import (
	"context"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

type WorkflowEventProjector struct {
	// sessionID is the ACP session ("sess_<uuid>") this projector emits to. Every
	// session/update notification MUST carry the ACP session ID so the editor routes it
	// to the right session. It is NOT the workflow run ID: event.Metadata["workflow_id"]
	// is the per-run execution UUID minted by the execution service and is meaningless to
	// the editor's session router. Binding the ACP session ID at construction (mirroring
	// the Emitter/Renderer wiring) is what makes lifecycle notifications actually reach the
	// editor — emitting with the workflow_id silently dropped every update.
	sessionID string
	emitter   application.SessionUpdateEmitter
	logger    ports.Logger
}

var _ ports.EventPublisher = (*WorkflowEventProjector)(nil)

func NewWorkflowEventProjector(sessionID string, emitter application.SessionUpdateEmitter, logger ports.Logger) *WorkflowEventProjector {
	return &WorkflowEventProjector{
		sessionID: sessionID,
		emitter:   emitter,
		logger:    logger,
	}
}

func (p *WorkflowEventProjector) Publish(ctx context.Context, event *pluginmodel.DomainEvent) error {
	if event == nil {
		p.logger.Warn("acp projector: nil event dropped")
		return nil
	}
	// Gate on workflow_id presence so only workflow lifecycle events are projected (and
	// to extract the step name). workflowID is used for diagnostics only — the emit is
	// routed by the ACP sessionID bound at construction, never by the run's workflow_id.
	workflowID, stepName, ok := extractWorkflowMeta(event)
	if !ok {
		return nil
	}

	var kind string
	var fields map[string]any
	switch event.Type {
	case workflow.EventWorkflowStarted:
		kind = "workflow_started"
	case workflow.EventWorkflowCompleted:
		kind = "workflow_completed"
		fields = map[string]any{
			"duration_ms": event.Metadata["duration_ms"],
		}
	case workflow.EventWorkflowFailed:
		kind = "workflow_failed"
		fields = map[string]any{
			"error": event.Metadata["error"],
		}
	case workflow.EventStepStarted:
		kind = "step_started"
		fields = map[string]any{
			"step_name": stepName,
		}
	case workflow.EventStepCompleted:
		kind = "step_completed"
		fields = map[string]any{
			"step_name": stepName,
		}
	case workflow.EventStepFailed:
		kind = "step_failed"
		fields = map[string]any{
			"step_name": stepName,
			"error":     event.Metadata["error"],
		}
	case workflow.EventStepRetrying:
		kind = "step_retrying"
		fields = map[string]any{
			"step_name": stepName,
		}
	default:
		p.logger.Debug("acp projector: unhandled event type", "type", event.Type)
		return nil
	}

	if err := p.emitter.EmitSessionUpdate(ctx, p.sessionID, kind, fields); err != nil {
		p.logger.Warn("emit session update failed", "sessionId", p.sessionID, "workflow_id", workflowID, "event", event.Type, "error", err)
		return err
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
