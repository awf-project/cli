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

	// Create template with custom functions
	tmpl := template.New("cmd").
		Option("missingkey=error").
		Funcs(template.FuncMap{
			"escape": escapeTemplateFunc,
			"json":   jsonTemplateFunc,
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
		// Serialize loop item using SerializeLoopItem for proper JSON representation
		serializedLoop := r.serializeLoopData(ctx.Loop)
		data["loop"] = serializedLoop
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

// escapeTemplateFunc wraps ShellEscape to handle both strings and serializableItem.
// This function is used in templates via {{escape .var}} syntax.
func escapeTemplateFunc(v any) string {
	// Handle serializableItem by extracting its serialized string
	if item, ok := v.(serializableItem); ok {
		return ShellEscape(item.String())
	}
	// Handle regular strings
	if s, ok := v.(string); ok {
		return ShellEscape(s)
	}
	// Fallback for other types: convert to string first
	return ShellEscape(fmt.Sprintf("%v", v))
}

// jsonTemplateFunc serializes a value to JSON format.
// This function is used in templates via {{json .var}} syntax.
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
	// Serialize the item using SerializeLoopItem
	// SerializeLoopItem never returns an error (it handles failures internally via graceful degradation)
	// nolint:errcheck // SerializeLoopItem always returns nil error
	serialized, _ := SerializeLoopItem(loop.Item)

	// Wrap the item in serializableItem to provide both automatic serialization
	// via String() and correct JSON encoding via MarshalJSON()
	wrappedItem := serializableItem{
		original:   loop.Item,
		serialized: serialized,
	}

	// Create a copy of LoopData with all fields preserved
	return &LoopData{
		Item:   wrappedItem,
		Index:  loop.Index,
		First:  loop.First,
		Last:   loop.Last,
		Length: loop.Length,
		Parent: loop.Parent,
	}
}
