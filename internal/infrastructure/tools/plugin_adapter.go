package tools

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
)

var _ ports.ToolProvider = (*PluginToolAdapter)(nil)

type exposedOp struct {
	opName     string
	schema     *pluginmodel.OperationSchema
	jsonSchema map[string]any
}

// PluginToolAdapter wraps a ports.OperationProvider as a ports.ToolProvider.
// Operation schemas are frozen at construction time via GetOperation; subsequent
// provider-side schema changes are not reflected in the adapter.
type PluginToolAdapter struct {
	pluginName string
	provider   ports.OperationProvider
	tools      map[string]exposedOp
}

// NewPluginToolAdapter constructs an adapter exposing the named operations from provider.
// Tool names are prefixed as "<pluginName>_<op>" (single underscore, snake-case).
// Returns ErrUnknownOperation (wrapped) if any name in expose is absent from provider.
// Returns ErrUnsupportedSchema (wrapped) if any operation uses array/object input types.
func NewPluginToolAdapter(pluginName string, provider ports.OperationProvider, expose []string) (*PluginToolAdapter, error) {
	toolMap := make(map[string]exposedOp, len(expose))

	for _, opName := range expose {
		// Always route with the full "pluginName.opName" prefix so the provider
		// dispatches to the correct plugin rather than falling back to the
		// unprefixed search across all connected plugins (which returns the first
		// non-gRPC-error response regardless of capability).
		schema, ok := provider.GetOperation(pluginName + "." + opName)
		if !ok {
			return nil, fmt.Errorf("%s: %w", opName, ErrUnknownOperation)
		}

		jsonSchema, err := MapOperationSchema(schema)
		if err != nil {
			return nil, err
		}

		toolName := pluginName + "_" + opName
		toolMap[toolName] = exposedOp{
			opName:     opName,
			schema:     schema,
			jsonSchema: normalizeSchema(jsonSchema),
		}
	}

	return &PluginToolAdapter{
		pluginName: pluginName,
		provider:   provider,
		tools:      toolMap,
	}, nil
}

func (a *PluginToolAdapter) ListTools(_ context.Context) ([]ports.ToolDefinition, error) {
	defs := make([]ports.ToolDefinition, 0, len(a.tools))
	for toolName, op := range a.tools {
		defs = append(defs, ports.ToolDefinition{
			Name:        toolName,
			Description: composeToolDescription(op.schema, op.opName, a.pluginName),
			Source:      "plugin:" + a.pluginName,
			InputSchema: op.jsonSchema,
		})
	}
	// Sort by name to ensure deterministic ordering across calls; map iteration is random.
	slices.SortFunc(defs, func(a, b ports.ToolDefinition) int { return cmp.Compare(a.Name, b.Name) })
	return defs, nil
}

func (a *PluginToolAdapter) CallTool(ctx context.Context, name string, args map[string]any) (*ports.ToolResult, error) {
	op, ok := a.tools[name]
	if !ok {
		return nil, fmt.Errorf("%s: %w", name, ErrUnknownOperation)
	}

	// Pass the fully-qualified "pluginName.opName" to force direct routing in the
	// provider; unprefixed names trigger a blind fallback across ALL plugins and
	// return the first non-gRPC-error response, which may come from a plugin that
	// does not implement operations at all.
	result, err := a.provider.Execute(ctx, a.pluginName+"."+op.opName, args)
	if err != nil {
		return nil, err
	}

	toolResult := &ports.ToolResult{
		IsError: !result.Success || result.Error != "",
	}

	if len(result.Outputs) > 0 {
		data, marshalErr := json.Marshal(result.Outputs)
		if marshalErr == nil {
			toolResult.Content = []ports.ToolContent{{Type: "text", Text: string(data)}}
		}
	}

	if result.Error != "" {
		toolResult.Content = append(toolResult.Content, ports.ToolContent{Type: "text", Text: result.Error})
	}

	return toolResult, nil
}

func (a *PluginToolAdapter) Close(_ context.Context) error {
	return nil
}

// composeToolDescription builds the human-readable description forwarded to tools/list.
// Rule: "<schema.Description>. Returns a JSON object with fields: <outputs>."
// When Description is empty a generic sentence is used so agents always receive
// a non-empty contract. When Outputs is empty the outputs sentence is omitted.
func composeToolDescription(schema *pluginmodel.OperationSchema, opName, pluginName string) string {
	base := schema.Description
	if base == "" {
		base = fmt.Sprintf("Operation '%s' from plugin '%s'.", opName, pluginName)
	}

	if len(schema.Outputs) == 0 {
		return base
	}

	return base + " Returns a JSON object with fields: " + strings.Join(schema.Outputs, ", ") + "."
}

// normalizeSchema converts Go-typed values (e.g. []string) to JSON-compatible equivalents
// (e.g. []any) by performing a JSON round-trip. This ensures the schema is directly usable
// by consumers that serialize it to JSON without an intermediate conversion step.
func normalizeSchema(m map[string]any) map[string]any {
	data, err := json.Marshal(m)
	if err != nil {
		return m
	}
	var normalized map[string]any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return m
	}
	return normalized
}
