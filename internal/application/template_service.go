package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
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

	// Circular reference detection
	if visited[ref.TemplateName] {
		chain := make([]string, 0, len(visited)+1)
		for k := range visited {
			chain = append(chain, k)
		}
		chain = append(chain, ref.TemplateName)
		return &workflow.CircularTemplateError{Chain: chain}
	}
	visited[ref.TemplateName] = true
	defer delete(visited, ref.TemplateName)

	// Load template
	tmpl, err := s.repo.GetTemplate(ctx, ref.TemplateName)
	if err != nil {
		return fmt.Errorf("load template %s: %w", ref.TemplateName, err)
	}

	// Validate and merge parameters
	params, err := s.mergeParameters(tmpl, ref.Parameters)
	if err != nil {
		return err
	}

	// Get the primary state from template
	// Priority: state with same name as template > first state
	var templateStep *workflow.Step
	if ts, ok := tmpl.States[tmpl.Name]; ok {
		templateStep = ts
	} else {
		// Use first state
		for _, ts := range tmpl.States {
			templateStep = ts
			break
		}
	}

	if templateStep == nil {
		return fmt.Errorf("template %q has no states", tmpl.Name)
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
	// Replace {{parameters.X}} with escaped values
	// Security: All parameter values are shell-escaped to prevent injection attacks
	result := template
	for name, value := range params {
		placeholder := fmt.Sprintf("{{parameters.%s}}", name)
		escaped := interpolation.ShellEscape(fmt.Sprintf("%v", value))
		result = strings.ReplaceAll(result, placeholder, escaped)
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
