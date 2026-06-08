package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
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
	recorder  ports.Recorder
	runID     string
}

// SetRecorder attaches an optional transcript recorder for capturing tool.call / tool.result events.
func (r *Router) SetRecorder(rec ports.Recorder) {
	r.recorder = rec
}

// SetRunID sets the workflow run identifier stamped onto emitted tool.call / tool.result
// events so tool exchanges can be correlated to their originating workflow run.
func (r *Router) SetRunID(id string) {
	r.runID = id
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

	var callID string
	if r.recorder != nil {
		callID = uuid.New().String()
		callEvent := transcript.ExchangeEvent{
			Type:      transcript.EventTypeToolCall,
			RunID:     r.runID,
			Timestamp: time.Now(),
			Payload:   &transcript.ToolPayload{Name: name, CallID: callID, Input: args, Fidelity: transcript.FidelityRouter},
		}
		if recErr := r.recorder.Record(ctx, callEvent); recErr != nil {
			r.logger.Warn("transcript record warning", "error", recErr, "event", transcript.EventTypeToolCall)
		}
	}

	result, err := entry.provider.CallTool(ctx, name, args)

	durationMs := time.Since(start).Milliseconds()
	span.SetAttribute("tool.duration_ms", durationMs)

	if err != nil {
		span.RecordError(err)
	}

	if r.recorder != nil {
		resultPayload := &transcript.ToolPayload{Name: name, CallID: callID, Input: args, Fidelity: transcript.FidelityRouter, Output: result}
		if err != nil {
			resultPayload.Error = err.Error()
		}
		resultEvent := transcript.ExchangeEvent{
			Type:      transcript.EventTypeToolResult,
			RunID:     r.runID,
			Timestamp: time.Now(),
			Payload:   resultPayload,
		}
		if recErr := r.recorder.Record(ctx, resultEvent); recErr != nil {
			r.logger.Warn("transcript record warning", "error", recErr, "event", transcript.EventTypeToolResult)
		}
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
