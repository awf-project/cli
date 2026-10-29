package otel

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type blockedSpanExporter struct {
	mu           sync.Mutex
	started      chan struct{}
	finished     chan struct{}
	startedOnce  sync.Once
	finishedOnce sync.Once
	deadline     time.Time
	hasDeadline  bool
}

func newBlockedSpanExporter() *blockedSpanExporter {
	return &blockedSpanExporter{
		started:  make(chan struct{}),
		finished: make(chan struct{}),
	}
}

func (e *blockedSpanExporter) ExportSpans(ctx context.Context, _ []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	e.deadline, e.hasDeadline = ctx.Deadline()
	e.mu.Unlock()
	e.startedOnce.Do(func() { close(e.started) })
	<-ctx.Done()
	e.finishedOnce.Do(func() { close(e.finished) })
	return ctx.Err()
}

func (e *blockedSpanExporter) Shutdown(context.Context) error { return nil }

func (e *blockedSpanExporter) cleanupContext() (time.Time, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.deadline, e.hasDeadline
}

type cooperativeSpanExporter struct {
	exported   chan struct{}
	shutdown   chan struct{}
	exportOnce sync.Once
	stopOnce   sync.Once
}

func newCooperativeSpanExporter() *cooperativeSpanExporter {
	return &cooperativeSpanExporter{
		exported: make(chan struct{}),
		shutdown: make(chan struct{}),
	}
}

func (e *cooperativeSpanExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	e.exportOnce.Do(func() { close(e.exported) })
	return nil
}

func (e *cooperativeSpanExporter) Shutdown(context.Context) error {
	e.stopOnce.Do(func() { close(e.shutdown) })
	return nil
}

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
	t.Run("cooperative exporter completes cleanup", func(t *testing.T) {
		exporter := newCooperativeSpanExporter()
		sdkProvider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
		tp := &TracerProvider{
			provider: sdkProvider,
			tracer:   sdkProvider.Tracer("shutdown-success-test"),
		}
		_, span := tp.Start(context.Background(), "shutdown-success")
		span.End()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, tp.Shutdown(shutdownCtx))
		select {
		case <-exporter.exported:
		case <-time.After(time.Second):
			t.Fatal("shutdown did not flush completed spans")
		}
		select {
		case <-exporter.shutdown:
		case <-time.After(time.Second):
			t.Fatal("shutdown did not complete exporter cleanup")
		}
	})

	t.Run("blocked exporter inherits deadline and finishes before return", func(t *testing.T) {
		exporter := newBlockedSpanExporter()
		sdkProvider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
		tp := &TracerProvider{
			provider: sdkProvider,
			tracer:   sdkProvider.Tracer("shutdown-deadline-test"),
		}

		_, span := tp.Start(context.Background(), "shutdown-deadline")
		span.End()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		shutdownDeadline, ok := shutdownCtx.Deadline()
		require.True(t, ok)

		shutdownResult := make(chan error, 1)
		go func() {
			shutdownResult <- tp.Shutdown(shutdownCtx)
		}()

		select {
		case <-exporter.started:
		case <-time.After(time.Second):
			t.Fatal("shutdown did not start exporter cleanup")
		}

		select {
		case err := <-shutdownResult:
			assert.ErrorIs(t, err, context.DeadlineExceeded)
		case <-time.After(time.Second):
			t.Fatal("shutdown did not honor its context deadline")
		}

		cleanupDeadline, hasDeadline := exporter.cleanupContext()
		assert.True(t, hasDeadline, "exporter cleanup context must retain a deadline")
		assert.False(t, cleanupDeadline.After(shutdownDeadline), "exporter cleanup must not outlive shutdown deadline")
		select {
		case <-exporter.finished:
		default:
			t.Fatal("shutdown returned before blocked exporter work finished")
		}
	})
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

func TestTracerProvider_Shutdown_CancelsAlreadyRunningBackgroundExport(t *testing.T) {
	exporter := newBlockedSpanExporter()
	provider := newProviderWithExporter(exporter, "background-export", sdktrace.WithBatchTimeout(time.Millisecond))
	_, span := provider.Start(context.Background(), "pending-export")
	span.End()
	select {
	case <-exporter.started:
	case <-time.After(time.Second):
		t.Fatal("background export did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, provider.Shutdown(ctx), context.DeadlineExceeded)
	select {
	case <-exporter.finished:
	default:
		t.Fatal("background export outlived shutdown")
	}
}
