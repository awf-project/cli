package application

import (
	"context"
	"fmt"

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
	workflows, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	return workflows, nil
}

// GetWorkflow retrieves a workflow by name.
func (s *WorkflowService) GetWorkflow(ctx context.Context, name string) (*workflow.Workflow, error) {
	wf, err := s.repo.Load(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("load workflow %s: %w", name, err)
	}
	return wf, nil
}

// ValidateWorkflow validates a workflow definition.
func (s *WorkflowService) ValidateWorkflow(ctx context.Context, name string) error {
	wf, err := s.repo.Load(ctx, name)
	if err != nil {
		return fmt.Errorf("load workflow %s: %w", name, err)
	}
	if wf == nil {
		return nil
	}
	if err := wf.Validate(); err != nil {
		return fmt.Errorf("validate workflow %s: %w", name, err)
	}
	return nil
}

// WorkflowExists checks if a workflow exists.
func (s *WorkflowService) WorkflowExists(ctx context.Context, name string) (bool, error) {
	exists, err := s.repo.Exists(ctx, name)
	if err != nil {
		return false, fmt.Errorf("check workflow exists %s: %w", name, err)
	}
	return exists, nil
}
