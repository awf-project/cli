package workflow

import "fmt"

// CircularTemplateError indicates a circular dependency in template references.
type CircularTemplateError struct {
	Chain []string // chain of template names forming the cycle
}

func (e *CircularTemplateError) Error() string {
	return fmt.Sprintf("circular template reference detected: %v", e.Chain)
}

// MissingParameterError indicates a required template parameter was not provided.
type MissingParameterError struct {
	TemplateName  string   // template name
	ParameterName string   // missing parameter name
	Required      []string // list of all required parameters
}

func (e *MissingParameterError) Error() string {
	return fmt.Sprintf("template %q missing required parameter %q", e.TemplateName, e.ParameterName)
}

// TemplateNotFoundError indicates a referenced template does not exist.
type TemplateNotFoundError struct {
	TemplateName string // name of the missing template
	ReferencedBy string // file or step that referenced it
}

func (e *TemplateNotFoundError) Error() string {
	if e.ReferencedBy != "" {
		return fmt.Sprintf("template %q not found (referenced by %s)", e.TemplateName, e.ReferencedBy)
	}
	return fmt.Sprintf("template %q not found", e.TemplateName)
}
