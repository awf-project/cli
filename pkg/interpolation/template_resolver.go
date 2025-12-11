package interpolation

import (
	"bytes"
	"strings"
	"text/template"
)

// TemplateResolver implements Resolver using Go's text/template.
type TemplateResolver struct{}

// NewTemplateResolver creates a new TemplateResolver.
func NewTemplateResolver() *TemplateResolver {
	return &TemplateResolver{}
}

// Resolve interpolates variables in the template string.
func (r *TemplateResolver) Resolve(tmplStr string, ctx *Context) (string, error) {
	if tmplStr == "" {
		return "", nil
	}

	// Build template data map
	data := r.buildTemplateData(ctx)

	// Create template with custom functions
	tmpl := template.New("cmd").
		Option("missingkey=error").
		Funcs(template.FuncMap{
			"escape": ShellEscape,
		})

	// Parse template
	tmpl, err := tmpl.Parse(tmplStr)
	if err != nil {
		return "", &ParseError{Template: tmplStr, Cause: err}
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", r.wrapExecutionError(err, tmplStr)
	}

	return buf.String(), nil
}

func (r *TemplateResolver) buildTemplateData(ctx *Context) map[string]any {
	data := map[string]any{
		"inputs":   ctx.Inputs,
		"states":   ctx.States,
		"workflow": ctx.Workflow,
		"env":      ctx.Env,
		"context":  ctx.Context,
	}
	if ctx.Error != nil {
		data["error"] = ctx.Error
	}
	if ctx.Loop != nil {
		data["loop"] = ctx.Loop
	}
	return data
}

func (r *TemplateResolver) wrapExecutionError(err error, tmpl string) error {
	errStr := err.Error()

	// text/template error format: "... map has no entry for key \"foo\""
	if strings.Contains(errStr, "map has no entry for key") {
		start := strings.Index(errStr, "\"")
		end := strings.LastIndex(errStr, "\"")
		if start != -1 && end > start {
			varName := errStr[start+1 : end]
			return &UndefinedVariableError{Variable: varName}
		}
	}

	// Handle nil pointer/zero value access (e.g., .error.Message when error is nil)
	if strings.Contains(errStr, "nil pointer") ||
		strings.Contains(errStr, "can't evaluate field") ||
		strings.Contains(errStr, "is not a struct") {
		// Try to extract the field name
		if idx := strings.Index(errStr, "field "); idx != -1 {
			rest := errStr[idx+6:]
			if end := strings.IndexAny(rest, " "); end != -1 {
				return &UndefinedVariableError{Variable: rest[:end]}
			}
		}
		return &UndefinedVariableError{Variable: "unknown"}
	}

	return &ParseError{Template: tmpl, Cause: err}
}
