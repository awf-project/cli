package application

import (
	"context"
	"errors"
	"fmt"

	domerrors "github.com/awf-project/awf/internal/domain/errors"
	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
)

type WorkflowService struct {
	repo      ports.WorkflowRepository
	store     ports.StateStore
	executor  ports.CommandExecutor
	logger    ports.Logger
	validator ports.ExpressionValidator
}

func NewWorkflowService(
	repo ports.WorkflowRepository,
	store ports.StateStore,
	executor ports.CommandExecutor,
	logger ports.Logger,
	validator ports.ExpressionValidator,
) *WorkflowService {
	return &WorkflowService{
		repo:      repo,
		store:     store,
		executor:  executor,
		logger:    logger,
		validator: validator,
	}
}

func (s *WorkflowService) ListWorkflows(ctx context.Context) ([]string, error) {
	workflows, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	return workflows, nil
}

func (s *WorkflowService) GetWorkflow(ctx context.Context, name string) (*workflow.Workflow, error) {
	wf, err := s.repo.Load(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("load workflow %s: %w", name, err)
	}
	return wf, nil
}

func (s *WorkflowService) ValidateWorkflow(ctx context.Context, name string) error {
	wf, err := s.repo.Load(ctx, name)
	if err != nil {
		return fmt.Errorf("load workflow %s: %w", name, err)
	}
	if err := wf.Validate(s.validator.Compile); err != nil {
		var stateRefErr *workflow.StateReferenceError
		if errors.As(err, &stateRefErr) {
			availableAny := make([]any, len(stateRefErr.AvailableStates))
			for i, s := range stateRefErr.AvailableStates {
				availableAny[i] = s
			}
			return domerrors.NewWorkflowError(
				domerrors.ErrorCodeWorkflowValidationMissingState,
				stateRefErr.Error(),
				map[string]any{
					"state":            stateRefErr.ReferencedState,
					"available_states": availableAny,
					"step":             stateRefErr.StepName,
					"field":            stateRefErr.Field,
				},
				err,
			)
		}
		return fmt.Errorf("validate workflow %s: %w", name, err)
	}
	return nil
}

func (s *WorkflowService) WorkflowExists(ctx context.Context, name string) (bool, error) {
	exists, err := s.repo.Exists(ctx, name)
	if err != nil {
		return false, fmt.Errorf("check workflow exists %s: %w", name, err)
	}
	return exists, nil
}
