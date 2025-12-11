package workflow

import (
	"errors"
	"fmt"
)

// Template represents a reusable workflow template.
type Template struct {
	Name       string
	Parameters []TemplateParam
	States     map[string]*Step
}

// TemplateParam defines a template parameter.
type TemplateParam struct {
	Name     string
	Required bool
	Default  any
}

// WorkflowTemplateRef references a template from a step.
type WorkflowTemplateRef struct {
	TemplateName string
	Parameters   map[string]any
}

// Validate checks if the template is valid.
func (t *Template) Validate() error {
	if t.Name == "" {
		return errors.New("template name is required")
	}
	if len(t.States) == 0 {
		return errors.New("template must define at least one state")
	}

	seen := make(map[string]bool)
	for _, p := range t.Parameters {
		if p.Name == "" {
			return errors.New("parameter name is required")
		}
		if seen[p.Name] {
			return fmt.Errorf("duplicate parameter name: %s", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}

// GetRequiredParams returns names of required parameters.
func (t *Template) GetRequiredParams() []string {
	var required []string
	for _, p := range t.Parameters {
		if p.Required {
			required = append(required, p.Name)
		}
	}
	return required
}

// GetDefaultValues returns a map of parameter defaults.
func (t *Template) GetDefaultValues() map[string]any {
	defaults := make(map[string]any)
	for _, p := range t.Parameters {
		if p.Default != nil {
			defaults[p.Name] = p.Default
		}
	}
	return defaults
}
