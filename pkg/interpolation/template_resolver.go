package interpolation //nolint:wrapcheck // Template resolver returns errors from text/template directly as they are already descriptive

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	tmpl := template.New("cmd").
		Option("missingkey=error").
		Funcs(template.FuncMap{
			"escape":   escapeTemplateFunc,
			"json":     jsonTemplateFunc,
			"inputs":   r.makeInputsAccessor(ctx),
			"states":   r.makeStatesAccessor(ctx),
			"workflow": r.makeWorkflowAccessor(ctx),
			"env":      r.makeEnvAccessor(ctx),
			"loop":     r.makeLoopAccessor(ctx),
			"context":  r.makeContextAccessor(ctx),
			"error":    r.makeErrorAccessor(ctx),
		})

	tmpl, err := tmpl.Parse(tmplStr)
	if err != nil {
		return "", &ParseError{Template: tmplStr, Cause: err}
	}

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
		// Serialize loop item using SerializeLoopItem for proper JSON representation
		serializedLoop := r.serializeLoopData(ctx.Loop)
		data["loop"] = serializedLoop
	}
	return data
}

func (r *TemplateResolver) wrapExecutionError(err error, tmpl string) error {
	errStr := err.Error()

	if strings.Contains(errStr, "map has no entry for key") {
		start := strings.Index(errStr, "\"")
		end := strings.LastIndex(errStr, "\"")
		if start != -1 && end > start {
			varName := errStr[start+1 : end]
			return &UndefinedVariableError{Variable: varName}
		}
	}

	if strings.Contains(errStr, "nil pointer") ||
		strings.Contains(errStr, "can't evaluate field") ||
		strings.Contains(errStr, "is not a struct") {
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

// escapeTemplateFunc wraps ShellEscape to handle both strings and serializableItem.
func escapeTemplateFunc(v any) string {
	if item, ok := v.(serializableItem); ok {
		return ShellEscape(item.String())
	}
	if s, ok := v.(string); ok {
		return ShellEscape(s)
	}
	return ShellEscape(fmt.Sprintf("%v", v))
}

// jsonTemplateFunc serializes a value to JSON format.
func jsonTemplateFunc(v any) (string, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	}
	return string(jsonBytes), nil
}

// serializableItem wraps a loop item value and provides automatic JSON serialization
// when printed in templates, while preserving the original value for the {{json}} function.
type serializableItem struct {
	original   any
	serialized string
}

// String implements fmt.Stringer, which Go templates use for {{.value}} printing.
// This ensures loop items are serialized to JSON automatically.
func (s serializableItem) String() string {
	return s.serialized
}

// MarshalJSON implements json.Marshaler to ensure the original value is used
// when the {{json}} function is applied, avoiding double-encoding.
func (s serializableItem) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(s.original)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}
	return data, nil
}

// serializeLoopData creates a copy of LoopData with the Item field wrapped
// in serializableItem for proper JSON representation in templates.
func (r *TemplateResolver) serializeLoopData(loop *LoopData) *LoopData {
	serialized, _ := SerializeLoopItem(loop.Item) //nolint:errcheck // never returns error

	wrappedItem := serializableItem{
		original:   loop.Item,
		serialized: serialized,
	}

	return &LoopData{
		Item:   wrappedItem,
		Index:  loop.Index,
		First:  loop.First,
		Last:   loop.Last,
		Length: loop.Length,
		Parent: loop.Parent,
	}
}

// makeLoopAccessor returns a function that provides a map with lowercase property names
// for loop data (e.g., {{loop.index}}, {{loop.first}}, {{loop.last}}).
func (r *TemplateResolver) makeLoopAccessor(ctx *Context) func() (map[string]any, error) {
	return func() (map[string]any, error) {
		if ctx.Loop == nil {
			return nil, &UndefinedVariableError{Variable: "loop"}
		}

		serialized := r.serializeLoopData(ctx.Loop)
		return map[string]any{
			"index":  ctx.Loop.Index,
			"first":  ctx.Loop.First,
			"last":   ctx.Loop.Last,
			"length": ctx.Loop.Length,
			"item":   serialized.Item,
		}, nil
	}
}

// makeContextAccessor returns a function that provides a map with lowercase property names
// for context data (e.g., {{context.working_dir}}, {{context.user}}).
func (r *TemplateResolver) makeContextAccessor(ctx *Context) func() map[string]any {
	return func() map[string]any {
		return map[string]any{
			"working_dir": ctx.Context.WorkingDir,
			"user":        ctx.Context.User,
			"hostname":    ctx.Context.Hostname,
		}
	}
}

// makeErrorAccessor returns a function that provides a map with lowercase property names
// for error data (e.g., {{error.message}}, {{error.type}}, {{error.exit_code}}).
func (r *TemplateResolver) makeErrorAccessor(ctx *Context) func() (map[string]any, error) {
	return func() (map[string]any, error) {
		if ctx.Error == nil {
			return nil, &UndefinedVariableError{Variable: "error"}
		}

		return map[string]any{
			"message":   ctx.Error.Message,
			"type":      ctx.Error.Type,
			"exit_code": ctx.Error.ExitCode,
			"state":     ctx.Error.State,
		}, nil
	}
}

// makeInputsAccessor returns a function that provides the inputs map.
func (r *TemplateResolver) makeInputsAccessor(ctx *Context) func() map[string]any {
	return func() map[string]any {
		return ctx.Inputs
	}
}

// makeStatesAccessor returns a function that provides the states map.
func (r *TemplateResolver) makeStatesAccessor(ctx *Context) func() map[string]StepStateData {
	return func() map[string]StepStateData {
		return ctx.States
	}
}

// makeWorkflowAccessor returns a function that provides workflow metadata.
func (r *TemplateResolver) makeWorkflowAccessor(ctx *Context) func() WorkflowData {
	return func() WorkflowData {
		return ctx.Workflow
	}
}

// makeEnvAccessor returns a function that provides environment variables.
func (r *TemplateResolver) makeEnvAccessor(ctx *Context) func() map[string]string {
	return func() map[string]string {
		return ctx.Env
	}
}
