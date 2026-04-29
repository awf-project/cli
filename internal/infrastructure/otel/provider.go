package otel

import (
	"context"
	"errors"
	"fmt"

	"github.com/awf-project/cli/internal/domain/ports"
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
	provider *sdktrace.TracerProvider
	tracer   oteltrace.Tracer
	exporter *otlptrace.Exporter
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
		return nil, fmt.Errorf("otel provider: %w", err)
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exp, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create otlp grpc exporter: %w", err)
	}

	res := resource.NewSchemaless(
		attribute.String("service.name", cfg.ServiceName),
	)

	sdkProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	return &TracerProvider{
		provider: sdkProvider,
		tracer:   sdkProvider.Tracer(cfg.ServiceName),
		exporter: exp,
	}, nil
}

func (p *TracerProvider) Start(ctx context.Context, spanName string) (context.Context, ports.Span) {
	newCtx, span := p.tracer.Start(ctx, spanName)
	return newCtx, spanAdapter{span: span}
}

func (p *TracerProvider) Shutdown(ctx context.Context) error {
	if err := p.provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown tracer provider: %w", err)
	}
	return nil
}
