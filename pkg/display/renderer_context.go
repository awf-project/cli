package display

import "context"

// EventRenderer receives parsed DisplayEvents for rendering to a transport.
// It shares the same underlying function type as agents.DisplayEventRenderer
// (func([]DisplayEvent)), but Go's type system requires an explicit conversion
// between named types from different packages even when the underlying types
// are identical — e.g. DisplayEventRenderer(r) at the call site.
type EventRenderer func(events []DisplayEvent)

type rendererCtxKey struct{}

// WithRenderer returns a context carrying a per-step EventRenderer. A nil renderer
// is stored as-is (RendererFromContext then returns nil).
func WithRenderer(ctx context.Context, r EventRenderer) context.Context {
	return context.WithValue(ctx, rendererCtxKey{}, r)
}

// RendererFromContext extracts the EventRenderer set by WithRenderer, or nil when
// none is present (the common case — all non-ACP execution paths).
func RendererFromContext(ctx context.Context) EventRenderer {
	r, ok := ctx.Value(rendererCtxKey{}).(EventRenderer)
	if !ok {
		return nil
	}
	return r
}
