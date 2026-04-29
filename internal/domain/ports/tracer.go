package ports

import "context"

type Span interface {
	End()
	SetAttribute(key string, value any)
	RecordError(err error)
	AddEvent(name string)
}

type Tracer interface {
	Start(ctx context.Context, spanName string) (context.Context, Span)
}

type NopSpan struct{}

func (NopSpan) End()                         {}
func (NopSpan) SetAttribute(_ string, _ any) {}
func (NopSpan) RecordError(_ error)          {}
func (NopSpan) AddEvent(_ string)            {}

type NopTracer struct{}

func (NopTracer) Start(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, NopSpan{}
}
