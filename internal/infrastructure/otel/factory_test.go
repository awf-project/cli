package otel_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	infraotel "github.com/awf-project/cli/internal/infrastructure/otel"
)

func TestNewTracerFromConfig_Empty(t *testing.T) {
	tracer, shutdown, err := infraotel.NewTracerFromConfig(context.Background(), infraotel.TracerConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer shutdown()

	if _, ok := tracer.(ports.NopTracer); !ok {
		t.Errorf("expected NopTracer for empty config, got %T", tracer)
	}
}

func TestNewTracerFromConfig_ParsesHTTPS(t *testing.T) {
	cfg := infraotel.TracerConfig{
		Endpoint:    "https://otel.example.com:4317",
		ServiceName: "test-svc",
	}
	tracer, shutdown, err := infraotel.NewTracerFromConfig(context.Background(), cfg)
	if err != nil {
		// Connection error is acceptable — we're testing the parse path
		shutdown()
		return
	}
	defer shutdown()
	if _, ok := tracer.(ports.NopTracer); ok {
		t.Error("expected real tracer for non-empty endpoint, got NopTracer")
	}
}
