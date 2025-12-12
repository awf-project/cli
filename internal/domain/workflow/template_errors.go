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
