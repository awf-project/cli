package application

import (
	"context"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// WorkflowService orchestrates workflow operations.
type WorkflowService struct {
	repo     ports.WorkflowRepository
	store    ports.StateStore
	executor ports.CommandExecutor
	logger   ports.Logger
}

// NewWorkflowService creates a new workflow service with injected dependencies.
func NewWorkflowService(
	repo ports.WorkflowRepository,
	store ports.StateStore,
	executor ports.CommandExecutor,
	logger ports.Logger,
) *WorkflowService {
	return &WorkflowService{
		repo:     repo,
		store:    store,
		executor: executor,
		logger:   logger,
	}
}

// ListWorkflows returns all available workflow names.
func (s *WorkflowService) ListWorkflows(ctx context.Context) ([]string, error) {
	return s.repo.List(ctx)
}

// GetWorkflow retrieves a workflow by name.
func (s *WorkflowService) GetWorkflow(ctx context.Context, name string) (*workflow.Workflow, error) {
	return s.repo.Load(ctx, name)
}

// ValidateWorkflow validates a workflow definition.
func (s *WorkflowService) ValidateWorkflow(ctx context.Context, name string) error {
	wf, err := s.repo.Load(ctx, name)
	if err != nil {
		return err
	}
	if wf == nil {
		return nil
	}
	return wf.Validate()
}

// WorkflowExists checks if a workflow exists.
func (s *WorkflowService) WorkflowExists(ctx context.Context, name string) (bool, error) {
	return s.repo.Exists(ctx, name)
}
