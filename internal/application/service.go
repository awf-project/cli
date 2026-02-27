package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
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

	return s.validatePromptFiles(wf)
}

func (s *WorkflowService) validatePromptFiles(wf *workflow.Workflow) error {
	for _, step := range wf.Steps {
		if step.Type != workflow.StepTypeAgent || step.Agent == nil {
			continue
		}

		if step.Agent.PromptFile == "" {
			continue
		}

		// Skip validation for paths with template expressions — resolved at runtime
		if strings.Contains(step.Agent.PromptFile, "{{") {
			continue
		}

		path := step.Agent.PromptFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(wf.SourceDir, path)
		}

		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return domerrors.NewStructuredError(
					domerrors.ErrorCodeUserInputMissingFile,
					fmt.Sprintf("prompt_file not found: %s", step.Agent.PromptFile),
					map[string]any{
						"path": path,
						"step": step.Name,
					},
					err,
				)
			}
			return domerrors.NewStructuredError(
				domerrors.ErrorCodeUserInputMissingFile,
				fmt.Sprintf("prompt_file cannot be accessed: %s", step.Agent.PromptFile),
				map[string]any{
					"path": path,
					"step": step.Name,
				},
				err,
			)
		}

		if info.IsDir() {
			return domerrors.NewStructuredError(
				domerrors.ErrorCodeUserInputMissingFile,
				fmt.Sprintf("prompt_file is a directory, not a file: %s", step.Agent.PromptFile),
				map[string]any{
					"path": path,
					"step": step.Name,
				},
				nil,
			)
		}

		f, err := os.Open(path)
		if err != nil {
			return domerrors.NewStructuredError(
				domerrors.ErrorCodeUserInputMissingFile,
				fmt.Sprintf("prompt_file cannot be read: %s", step.Agent.PromptFile),
				map[string]any{
					"path": path,
					"step": step.Name,
				},
				err,
			)
		}
		_ = f.Close()
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
