package otel

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/pkg/httpx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var _ ports.Tracer = (*TracerProvider)(nil)

// Config holds OTLP gRPC exporter configuration for the tracer provider.
type Config struct {
	Endpoint    string
	ServiceName string
	Insecure    bool
}

// TracerProvider wraps the OTel SDK tracer provider with OTLP gRPC export.
// Created via otlptracegrpc.New(); use Shutdown to flush and stop.
type TracerProvider struct {
	provider      *sdktrace.TracerProvider
	tracer        oteltrace.Tracer
	exporter      *otlptrace.Exporter
	cancelExports context.CancelFunc
}

type spanAdapter struct {
	span oteltrace.Span
}

func (s spanAdapter) End() { s.span.End() }
func (s spanAdapter) SetAttribute(key string, value any) {
	switch v := value.(type) {
	case string:
		s.span.SetAttributes(attribute.String(key, v))
	case int:
		s.span.SetAttributes(attribute.Int(key, v))
	case int64:
		s.span.SetAttributes(attribute.Int64(key, v))
	case float64:
		s.span.SetAttributes(attribute.Float64(key, v))
	case bool:
		s.span.SetAttributes(attribute.Bool(key, v))
	default:
		s.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
	}
}
func (s spanAdapter) RecordError(err error) { s.span.RecordError(err) }
func (s spanAdapter) AddEvent(name string)  { s.span.AddEvent(name) }

func NewTracerProvider(ctx context.Context, cfg Config) (*TracerProvider, error) {
	if cfg.ServiceName == "" {
		return nil, errors.New("service name is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("otel provider: %w", httpx.RedactError(err))
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exp, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create otlp grpc exporter: %w", httpx.RedactError(err))
	}

	provider := newProviderWithExporter(exp, cfg.ServiceName)
	provider.exporter = exp
	return provider, nil
}

// newProviderWithExporter keeps background batches cancellable during shutdown,
// including exports already running before ForceFlush starts.
func newProviderWithExporter(exp sdktrace.SpanExporter, serviceName string, opts ...sdktrace.BatchSpanProcessorOption) *TracerProvider {
	lifetime, cancel := context.WithCancel(context.Background())
	res := resource.NewSchemaless(
		attribute.String("service.name", serviceName),
	)

	sdkProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(shutdownSpanExporter{SpanExporter: exp, lifetime: lifetime}, opts...),
		sdktrace.WithResource(res),
	)

	return &TracerProvider{
		provider:      sdkProvider,
		tracer:        sdkProvider.Tracer(serviceName),
		cancelExports: cancel,
	}
}

func (p *TracerProvider) Start(ctx context.Context, spanName string) (context.Context, ports.Span) {
	newCtx, span := p.tracer.Start(ctx, spanName)
	return newCtx, spanAdapter{span: span}
}

func (p *TracerProvider) Shutdown(ctx context.Context) error {
	// Flush under the caller deadline before SDK shutdown drains its queue with a
	// background context. Reserve time for the batch worker to join after cancellation.
	flushCtx := ctx
	cancel := func() {}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		flushCtx, cancel = context.WithDeadline(ctx, deadline.Add(-remaining/4))
	}
	defer cancel()
	if p.cancelExports != nil {
		stop := context.AfterFunc(flushCtx, p.cancelExports)
		defer stop()
		defer p.cancelExports()
	}
	flushErr := p.provider.ForceFlush(flushCtx)
	if err := errors.Join(flushErr, p.provider.Shutdown(ctx)); err != nil {
		return fmt.Errorf("shutdown tracer provider: %w", httpx.RedactError(err))
	}
	return nil
}

// shutdownSpanExporter links SDK background exports to the provider lifetime.
// The SDK otherwise drains its queue using a context unrelated to Shutdown.
type shutdownSpanExporter struct {
	sdktrace.SpanExporter
	lifetime context.Context
}

func (e shutdownSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(e.lifetime, cancel)
	defer stop()
	if err := e.SpanExporter.ExportSpans(ctx, spans); err != nil {
		return fmt.Errorf("export spans: %w", httpx.RedactError(err))
	}
	return nil
}

func (e shutdownSpanExporter) Shutdown(ctx context.Context) error {
	if err := e.SpanExporter.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown exporter: %w", httpx.RedactError(err))
	}
	return nil
}
