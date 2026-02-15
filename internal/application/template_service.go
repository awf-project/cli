package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
)

// TemplateService handles template resolution and expansion.
type TemplateService struct {
	repo   ports.TemplateRepository
	logger ports.Logger
}

// NewTemplateService creates a new template service.
func NewTemplateService(
	repo ports.TemplateRepository,
	logger ports.Logger,
) *TemplateService {
	return &TemplateService{
		repo:   repo,
		logger: logger,
	}
}

// ExpandWorkflow resolves all template references in a workflow.
func (s *TemplateService) ExpandWorkflow(ctx context.Context, wf *workflow.Workflow) error {
	if wf == nil {
		return nil
	}

	visited := make(map[string]bool)

	for name, step := range wf.Steps {
		if step.TemplateRef == nil {
			continue
		}

		if err := s.expandStep(ctx, name, step, visited); err != nil {
			return err
		}
	}

	return nil
}

func (s *TemplateService) expandStep(
	ctx context.Context,
	stepName string,
	step *workflow.Step,
	visited map[string]bool,
) error {
	ref := step.TemplateRef

	// Validate and load template (circular detection + loading)
	tmpl, err := s.ValidateAndLoadTemplate(ctx, ref, visited)
	if err != nil {
		return err
	}
	// Clean up visited map after expansion completes
	defer delete(visited, ref.TemplateName)

	// Validate and merge parameters
	params, err := s.mergeParameters(tmpl, ref.Parameters)
	if err != nil {
		return err
	}

	// Get the primary step from template
	templateStep, err := s.SelectPrimaryStep(tmpl)
	if err != nil {
		return err
	}

	// Check if template step also has a template ref (nested templates)
	if templateStep.TemplateRef != nil {
		if nestedErr := s.expandStep(ctx, stepName, templateStep, visited); nestedErr != nil {
			return nestedErr
		}
	}

	// Substitute parameters in template fields
	expandedCmd, err := s.substituteParams(templateStep.Command, params)
	if err != nil {
		return fmt.Errorf("expand command: %w", err)
	}

	expandedDir := templateStep.Dir
	if expandedDir != "" {
		expandedDir, err = s.substituteParams(expandedDir, params)
		if err != nil {
			return fmt.Errorf("expand dir: %w", err)
		}
	}

	// Merge template into step (step values take precedence for transitions)
	step.Type = templateStep.Type
	step.Command = expandedCmd

	// Dir: workflow step takes precedence, fallback to template
	if step.Dir == "" {
		step.Dir = expandedDir
	}

	// Timeout: workflow step takes precedence if non-zero
	if step.Timeout == 0 && templateStep.Timeout > 0 {
		step.Timeout = templateStep.Timeout
	}

	// Retry: workflow step takes precedence if set
	if step.Retry == nil && templateStep.Retry != nil {
		step.Retry = templateStep.Retry
	}

	// Capture: workflow step takes precedence if set
	if step.Capture == nil && templateStep.Capture != nil {
		step.Capture = templateStep.Capture
	}

	// OnSuccess/OnFailure from workflow step take precedence (already set)

	// Clear template ref after expansion
	step.TemplateRef = nil

	s.logger.Debug("expanded template",
		"step", stepName,
		"template", ref.TemplateName,
		"command", expandedCmd)

	return nil
}

// ValidateAndLoadTemplate checks for circular references and loads the template.
// Combines circular detection with template loading.
// Exported for testing during TDD RED phase (C005-T001).
// Note: Does not clean up visited map - caller is responsible for cleanup.
func (s *TemplateService) ValidateAndLoadTemplate(
	ctx context.Context,
	ref *workflow.WorkflowTemplateRef,
	visited map[string]bool,
) (*workflow.Template, error) {
	// Circular reference detection
	if visited[ref.TemplateName] {
		chain := make([]string, 0, len(visited)+1)
		for k := range visited {
			chain = append(chain, k)
		}
		chain = append(chain, ref.TemplateName)
		return nil, &workflow.CircularTemplateError{Chain: chain}
	}
	visited[ref.TemplateName] = true

	// Load template
	tmpl, err := s.repo.GetTemplate(ctx, ref.TemplateName)
	if err != nil {
		return nil, fmt.Errorf("load template %s: %w", ref.TemplateName, err)
	}
	return tmpl, nil
}

// SelectPrimaryStep determines which step to use from the template.
// Priority: step with same name as template > first step.
// Exported for testing during TDD RED phase (C005-T001).
func (s *TemplateService) SelectPrimaryStep(tmpl *workflow.Template) (*workflow.Step, error) {
	// Validate input
	if tmpl == nil {
		return nil, fmt.Errorf("cannot select primary step from nil template")
	}

	// Check for empty states
	if len(tmpl.States) == 0 {
		return nil, fmt.Errorf("template %q has no steps", tmpl.Name)
	}

	// Priority 1: state with same name as template
	if step, ok := tmpl.States[tmpl.Name]; ok {
		return step, nil
	}

	// Priority 2: first state (map iteration order is undefined, but we take first)
	for _, step := range tmpl.States {
		return step, nil
	}

	// Should never reach here since we checked len(tmpl.States) > 0
	return nil, fmt.Errorf("template %q has no steps", tmpl.Name)
}

// ExpandNestedTemplate recursively expands nested template references.
// Exported for testing during TDD RED phase (C005-T001).
func (s *TemplateService) ExpandNestedTemplate(
	ctx context.Context,
	stepName string,
	templateStep *workflow.Step,
	visited map[string]bool,
) error {
	// Check if template step has a nested template reference
	if templateStep.TemplateRef == nil {
		return nil
	}

	// Recursively expand the nested template
	return s.expandStep(ctx, stepName, templateStep, visited)
}

// ApplyTemplateFields merges template fields into the workflow step.
// Step values take precedence over template values for most fields.
// Exported for testing during TDD RED phase (C005-T001).
func (s *TemplateService) ApplyTemplateFields(
	step *workflow.Step,
	templateStep *workflow.Step,
	params map[string]any,
) error {
	// Validate inputs
	if step == nil {
		return fmt.Errorf("cannot apply template fields to nil step")
	}
	if templateStep == nil {
		return fmt.Errorf("cannot apply fields from nil template step")
	}

	// Substitute parameters in template fields
	expandedCmd, err := s.substituteParams(templateStep.Command, params)
	if err != nil {
		return fmt.Errorf("expand command: %w", err)
	}

	expandedDir := templateStep.Dir
	if expandedDir != "" {
		expandedDir, err = s.substituteParams(expandedDir, params)
		if err != nil {
			return fmt.Errorf("expand dir: %w", err)
		}
	}

	// Merge template into step (step values take precedence for transitions)
	step.Type = templateStep.Type

	// Command: workflow step takes precedence if non-empty
	if step.Command == "" {
		step.Command = expandedCmd
	}

	// Dir: workflow step takes precedence, fallback to template
	if step.Dir == "" {
		step.Dir = expandedDir
	}

	// Timeout: workflow step takes precedence if non-zero
	if step.Timeout == 0 && templateStep.Timeout > 0 {
		step.Timeout = templateStep.Timeout
	}

	// Retry: merge field-by-field
	if templateStep.Retry != nil {
		if step.Retry == nil {
			// No step retry config, use entire template retry
			step.Retry = templateStep.Retry
		} else {
			// Merge individual fields - step values take precedence if non-zero/non-empty
			if step.Retry.MaxAttempts == 0 {
				step.Retry.MaxAttempts = templateStep.Retry.MaxAttempts
			}
			if step.Retry.InitialDelayMs == 0 {
				step.Retry.InitialDelayMs = templateStep.Retry.InitialDelayMs
			}
			if step.Retry.Backoff == "" {
				step.Retry.Backoff = templateStep.Retry.Backoff
			}
		}
	}

	// Capture: merge field-by-field
	if templateStep.Capture != nil {
		if step.Capture == nil {
			// No step capture config, use entire template capture
			step.Capture = templateStep.Capture
		} else {
			// Merge individual fields - step values take precedence if non-empty
			if step.Capture.Stdout == "" {
				step.Capture.Stdout = templateStep.Capture.Stdout
			}
			if step.Capture.Stderr == "" {
				step.Capture.Stderr = templateStep.Capture.Stderr
			}
		}
	}

	// OnSuccess/OnFailure from workflow step take precedence (already set)
	// Clear template ref after expansion
	step.TemplateRef = nil

	return nil
}

func (s *TemplateService) mergeParameters(
	tmpl *workflow.Template,
	provided map[string]any,
) (map[string]any, error) {
	params := tmpl.GetDefaultValues()

	// Override with provided values
	for k, v := range provided {
		params[k] = v
	}

	// Check required parameters
	for _, name := range tmpl.GetRequiredParams() {
		if _, ok := params[name]; !ok {
			return nil, &workflow.MissingParameterError{
				TemplateName:  tmpl.Name,
				ParameterName: name,
				Required:      tmpl.GetRequiredParams(),
			}
		}
	}

	return params, nil
}

func (s *TemplateService) substituteParams(template string, params map[string]any) (string, error) {
	// Replace {{parameters.X}} with values
	// Note: Values are NOT shell-escaped here. Escaping should occur at execution layer if needed.
	result := template
	for name, value := range params {
		placeholder := fmt.Sprintf("{{parameters.%s}}", name)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	return result, nil
}

// ValidateTemplateRef validates a template reference without expanding.
func (s *TemplateService) ValidateTemplateRef(ctx context.Context, ref *workflow.WorkflowTemplateRef) error {
	tmpl, err := s.repo.GetTemplate(ctx, ref.TemplateName)
	if err != nil {
		return fmt.Errorf("load template %s: %w", ref.TemplateName, err)
	}

	_, err = s.mergeParameters(tmpl, ref.Parameters)
	return err
}
