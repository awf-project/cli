package application

import (
	"context"
	"encoding/json"
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
	repo              ports.WorkflowRepository
	store             ports.StateStore
	executor          ports.CommandExecutor
	logger            ports.Logger
	validator         ports.ExpressionValidator
	validatorProvider ports.WorkflowValidatorProvider
	packDiscoverer    ports.PackDiscoverer
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

func (s *WorkflowService) SetValidatorProvider(p ports.WorkflowValidatorProvider) {
	s.validatorProvider = p
}

func (s *WorkflowService) SetPackDiscoverer(d ports.PackDiscoverer) {
	s.packDiscoverer = d
}

func (s *WorkflowService) ListAllWorkflows(ctx context.Context) ([]workflow.WorkflowEntry, error) {
	names, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}

	entries := make([]workflow.WorkflowEntry, 0, len(names))
	for _, name := range names {
		entry := workflow.WorkflowEntry{Name: name}
		if wf, loadErr := s.repo.Load(ctx, name); loadErr == nil {
			entry.Version = wf.Version
			entry.Description = wf.Description
		}
		entries = append(entries, entry)
	}

	if s.packDiscoverer != nil {
		packEntries, packErr := s.packDiscoverer.DiscoverWorkflows(ctx)
		if packErr == nil {
			entries = append(entries, packEntries...)
		}
	}

	return entries, nil
}

func (s *WorkflowService) ListWorkflows(ctx context.Context) ([]string, error) {
	workflows, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	return workflows, nil
}

func (s *WorkflowService) GetWorkflow(ctx context.Context, name string) (*workflow.Workflow, error) {
	if packName, wfName, ok := strings.Cut(name, "/"); ok && s.packDiscoverer != nil {
		wf, err := s.packDiscoverer.LoadWorkflow(ctx, packName, wfName)
		if err != nil {
			return nil, fmt.Errorf("load pack workflow %s: %w", name, err)
		}
		return wf, nil
	}

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
	if err := wf.Validate(s.validator.Compile, nil); err != nil {
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

	if err := s.validatePromptFiles(wf); err != nil {
		return err
	}

	return s.validateWithPluginProvider(ctx, wf)
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

func (s *WorkflowService) validateWithPluginProvider(ctx context.Context, wf *workflow.Workflow) error {
	if s.validatorProvider == nil {
		return nil
	}

	workflowJSON, err := json.Marshal(wf)
	if err != nil {
		return fmt.Errorf("marshal workflow for plugin validation: %w", err)
	}

	results, err := s.validatorProvider.ValidateWorkflow(ctx, workflowJSON)
	if err != nil {
		return fmt.Errorf("plugin validation error: %w", err)
	}

	for _, result := range results {
		if result.Severity == ports.SeverityError {
			return fmt.Errorf("workflow validation failed: %s", result.Message)
		}
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
