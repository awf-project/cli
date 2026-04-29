package otel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTracerProvider_ExporterIsOTLPGRPC validates that the provider
// initialises an OTLP gRPC exporter (tp.exporter must be non-nil).
func TestNewTracerProvider_ExporterIsOTLPGRPC(t *testing.T) {
	cfg := Config{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		Insecure:    true,
	}

	tp, err := NewTracerProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, tp)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	})

	assert.NotNil(t, tp.exporter, "TracerProvider must initialise an OTLP gRPC exporter (otlptracegrpc)")
	assert.NotNil(t, tp.provider, "TracerProvider must initialise an SDK tracer provider wired to the exporter")
}

// TestNewTracerProvider_WithEmptyServiceName_ReturnsError validates that an
// empty ServiceName is rejected at construction time.
func TestNewTracerProvider_WithEmptyServiceName_ReturnsError(t *testing.T) {
	cfg := Config{
		Endpoint:    "localhost:4317",
		ServiceName: "",
		Insecure:    true,
	}

	tp, err := NewTracerProvider(context.Background(), cfg)
	require.Error(t, err)
	assert.Nil(t, tp)
}

// TestNewTracerProvider_WithCancelledContext_ReturnsError validates that a
// pre-cancelled context causes exporter initialisation to fail.
func TestNewTracerProvider_WithCancelledContext_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := Config{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		Insecure:    true,
	}

	tp, err := NewTracerProvider(ctx, cfg)
	require.Error(t, err)
	assert.Nil(t, tp)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestTracerProvider_Shutdown_WithExpiredContext_ReturnsDeadlineExceeded
// validates that Shutdown propagates context deadline exceeded.
func TestTracerProvider_Shutdown_WithExpiredContext_ReturnsDeadlineExceeded(t *testing.T) {
	cfg := Config{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		Insecure:    true,
	}

	tp, err := NewTracerProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, tp)

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	err = tp.Shutdown(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestTracerProvider_ImplementsTracer(t *testing.T) {
	var _ ports.Tracer = (*TracerProvider)(nil)
}

func TestNewTracerProvider_WithValidInsecureConfig_ReturnsProvider(t *testing.T) {
	cfg := Config{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		Insecure:    true,
	}

	tp, err := NewTracerProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, tp)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	})
}

func TestTracerProvider_Start_ReturnsContextAndSpan(t *testing.T) {
	cfg := Config{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		Insecure:    true,
	}

	tp, err := NewTracerProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, tp)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	})

	newCtx, span := tp.Start(context.Background(), "test-operation")

	assert.NotNil(t, newCtx)
	require.NotNil(t, span)
	span.End()
}

func TestTracerProvider_Start_SpanIsRealOTelSpan(t *testing.T) {
	cfg := Config{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		Insecure:    true,
	}

	tp, err := NewTracerProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, tp)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	})

	_, span := tp.Start(context.Background(), "test-operation")
	defer span.End()

	_, isNop := span.(ports.NopSpan)
	assert.False(t, isNop, "Start must return a real OTel span, not a NopSpan")
}

func TestTracerProvider_Shutdown_CompletesWithinTimeout(t *testing.T) {
	cfg := Config{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		Insecure:    true,
	}

	tp, err := NewTracerProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, tp)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = tp.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

// newTestProvider is a shared helper that creates a TracerProvider for
// spanAdapter tests and registers its Shutdown in t.Cleanup.
func newTestProvider(t *testing.T) *TracerProvider {
	t.Helper()

	cfg := Config{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		Insecure:    true,
	}

	tp, err := NewTracerProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, tp)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	})

	return tp
}

// TestSpanAdapter_End verifies that calling End on a spanAdapter does not panic.
func TestSpanAdapter_End(t *testing.T) {
	tp := newTestProvider(t)

	_, span := tp.Start(context.Background(), "test-end")
	assert.NotPanics(t, func() {
		span.End()
	})
}

// TestSpanAdapter_SetAttribute verifies that SetAttribute does not panic for
// several representative value types (string, int, float64, bool).
func TestSpanAdapter_SetAttribute(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value any
	}{
		{
			name:  "string value",
			key:   "attr.string",
			value: "hello",
		},
		{
			name:  "int value",
			key:   "attr.int",
			value: 42,
		},
		{
			name:  "float64 value",
			key:   "attr.float64",
			value: 3.14,
		},
		{
			name:  "bool value",
			key:   "attr.bool",
			value: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := newTestProvider(t)
			_, span := tp.Start(context.Background(), "test-set-attribute")
			defer span.End()

			assert.NotPanics(t, func() {
				span.SetAttribute(tt.key, tt.value)
			})
		})
	}
}

// TestSpanAdapter_RecordError verifies that RecordError does not panic when
// called with a real error or with nil.
func TestSpanAdapter_RecordError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "with real error",
			err:  errors.New("something went wrong"),
		},
		{
			name: "with nil error",
			err:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := newTestProvider(t)
			_, span := tp.Start(context.Background(), "test-record-error")
			defer span.End()

			assert.NotPanics(t, func() {
				span.RecordError(tt.err)
			})
		})
	}
}

// TestSpanAdapter_AddEvent verifies that AddEvent does not panic when called
// with a valid event name.
func TestSpanAdapter_AddEvent(t *testing.T) {
	tp := newTestProvider(t)

	_, span := tp.Start(context.Background(), "test-add-event")
	defer span.End()

	assert.NotPanics(t, func() {
		span.AddEvent("cache.miss")
	})
}

// TestSpanAdapter_ImplementsPortsSpan is a compile-time guard that ensures
// spanAdapter satisfies the ports.Span interface contract.
func TestSpanAdapter_ImplementsPortsSpan(t *testing.T) {
	var _ ports.Span = spanAdapter{}
}
