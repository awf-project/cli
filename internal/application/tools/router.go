package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
)

var _ ports.ToolRouter = (*Router)(nil)

type toolEntry struct {
	provider   ports.ToolProvider
	definition ports.ToolDefinition
}

// Router dispatches tool calls to the provider that registered each tool name.
type Router struct {
	mu        sync.RWMutex
	registry  map[string]toolEntry
	tools     []ports.ToolDefinition
	providers []ports.ToolProvider
	tracer    ports.Tracer
	logger    ports.Logger
}

func NewRouter(tracer ports.Tracer, logger ports.Logger) *Router {
	return &Router{
		registry: make(map[string]toolEntry),
		tracer:   tracer,
		logger:   logger,
	}
}

// Register adds a provider's tools to the router. Returns a collision error if any tool name is already registered.
// The context is propagated to provider.ListTools so callers can enforce deadlines/cancellation
// on the initial tool discovery handshake.
func (r *Router) Register(ctx context.Context, provider ports.ToolProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tools, err := provider.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}

	for _, t := range tools {
		if _, exists := r.registry[t.Name]; exists {
			return domerrors.NewUserError(
				domerrors.ErrorCodeUserMCPProxyNameCollision,
				fmt.Sprintf("tool name collision: %q already registered", t.Name),
				map[string]any{"tool": t.Name},
				nil,
			)
		}
	}

	for _, t := range tools {
		r.registry[t.Name] = toolEntry{provider: provider, definition: t}
		r.tools = append(r.tools, t)
	}
	r.providers = append(r.providers, provider)
	return nil
}

func (r *Router) ListTools(ctx context.Context) ([]ports.ToolDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ports.ToolDefinition, len(r.tools))
	copy(result, r.tools)
	return result, nil
}

func (r *Router) CallTool(ctx context.Context, name string, args map[string]any) (*ports.ToolResult, error) {
	start := time.Now()
	_, span := r.tracer.Start(ctx, "tool.call."+name)
	defer span.End()

	span.SetAttribute("tool.name", name)

	r.mu.RLock()
	entry, ok := r.registry[name]
	r.mu.RUnlock()

	if !ok {
		err := domerrors.NewUserError(
			domerrors.ErrorCodeUserMCPProxyUnknownKey,
			fmt.Sprintf("unknown tool: %q", name),
			map[string]any{"tool": name},
			nil,
		)
		span.RecordError(err)
		return nil, err
	}

	span.SetAttribute("tool.source", entry.definition.Source)

	result, err := entry.provider.CallTool(ctx, name, args)

	durationMs := time.Since(start).Milliseconds()
	span.SetAttribute("tool.duration_ms", durationMs)

	if err != nil {
		span.RecordError(err)
	}

	fields := []any{
		"tool", name,
		"source", entry.definition.Source,
		"duration", durationMs,
	}
	if err != nil {
		fields = append(fields, "error", err)
	}
	r.logger.Info("tool called", fields...)

	return result, err
}

func (r *Router) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, p := range r.providers {
		if err := p.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
