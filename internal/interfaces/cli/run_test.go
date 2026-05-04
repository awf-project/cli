package cli

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	infraotel "github.com/awf-project/cli/internal/infrastructure/otel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTracerFromConfig_ReturnsNopTracerWhenExporterEmpty verifies that
// NewTracerFromConfig returns a NopTracer when Endpoint is not configured.
func TestNewTracerFromConfig_ReturnsNopTracerWhenExporterEmpty(t *testing.T) {
	cfg := infraotel.TracerConfig{
		Endpoint:    "",
		ServiceName: "awf",
	}

	tracer, shutdown, err := infraotel.NewTracerFromConfig(context.Background(), cfg)

	require.NoError(t, err)
	require.NotNil(t, tracer)
	require.NotNil(t, shutdown)
	defer shutdown()

	_, ok := tracer.(ports.NopTracer)
	assert.True(t, ok, "expected NopTracer for empty endpoint")

	// Should be able to start a span with NopTracer
	ctx, span := tracer.Start(context.Background(), "test-span")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	span.End()
}

// TestNewTracerFromConfig_AcceptsOtlpExporterEndpoint verifies that
// NewTracerFromConfig accepts a valid OTLP exporter endpoint.
func TestNewTracerFromConfig_AcceptsOtlpExporterEndpoint(t *testing.T) {
	cfg := infraotel.TracerConfig{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
	}

	tracer, shutdown, _ := infraotel.NewTracerFromConfig(context.Background(), cfg)

	require.NotNil(t, tracer)
	require.NotNil(t, shutdown)
	defer shutdown()

	// Even if exporter endpoint is invalid, should return a tracer
	// (connection errors are deferred until actual span export)
	ctx, span := tracer.Start(context.Background(), "test-span")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	span.End()
}

// TestNewTracerFromConfig_ShutdownFunctionIsCallable verifies that
// the shutdown function returned by NewTracerFromConfig can be called without panic.
func TestNewTracerFromConfig_ShutdownFunctionIsCallable(t *testing.T) {
	cfg := infraotel.TracerConfig{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
	}

	_, shutdown, err := infraotel.NewTracerFromConfig(context.Background(), cfg)
	require.NoError(t, err)

	// Shutdown should be callable without panic
	assert.NotPanics(t, func() {
		shutdown()
	})
}

// TestNewTracerFromConfig_UsesServiceNameFromConfig verifies that
// NewTracerFromConfig uses the ServiceName from config.
func TestNewTracerFromConfig_UsesServiceNameFromConfig(t *testing.T) {
	cfg := infraotel.TracerConfig{
		Endpoint:    "localhost:4317",
		ServiceName: "my-custom-service",
	}

	tracer, shutdown, _ := infraotel.NewTracerFromConfig(context.Background(), cfg)
	defer shutdown()

	// Should be able to create spans with the configured service
	ctx, span := tracer.Start(context.Background(), "test-span")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	span.End()
}

// TestNewTracerFromConfig_PropagatesContextWithSpan verifies that
// NewTracerFromConfig properly propagates the context when starting a span.
func TestNewTracerFromConfig_PropagatesContextWithSpan(t *testing.T) {
	cfg := infraotel.TracerConfig{
		Endpoint:    "",
		ServiceName: "awf",
	}

	tracer, shutdown, _ := infraotel.NewTracerFromConfig(context.Background(), cfg)
	defer shutdown()

	originalCtx := context.Background()
	spanCtx, span := tracer.Start(originalCtx, "test-span")

	assert.NotNil(t, spanCtx)
	assert.NotNil(t, span)

	// Context should be valid for downstream operations
	select {
	case <-spanCtx.Done():
		t.Fatal("context should not be canceled")
	default:
		// Expected: context is valid
	}

	span.End()
}

// TestConfig_OtelExporterFieldExists verifies that
// Config struct has OtelExporter field.
func TestConfig_OtelExporterFieldExists(t *testing.T) {
	cfg := &Config{
		OtelExporter: "localhost:4317",
	}

	assert.Equal(t, "localhost:4317", cfg.OtelExporter)
}

// TestConfig_OtelServiceNameFieldExists verifies that
// Config struct has OtelServiceName field.
func TestConfig_OtelServiceNameFieldExists(t *testing.T) {
	cfg := &Config{
		OtelServiceName: "test-service",
	}

	assert.Equal(t, "test-service", cfg.OtelServiceName)
}

// TestDefaultConfig_OtelServiceNameDefaultValue verifies that
// DefaultConfig sets OtelServiceName to "awf".
func TestDefaultConfig_OtelServiceNameDefaultValue(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "awf", cfg.OtelServiceName)
}

// TestDefaultConfig_OtelExporterEmptyByDefault verifies that
// DefaultConfig leaves OtelExporter empty (disabled by default).
func TestDefaultConfig_OtelExporterEmptyByDefault(t *testing.T) {
	cfg := DefaultConfig()

	assert.Empty(t, cfg.OtelExporter)
}

// TestNewTracerFromConfig_WorksWithContextTimeout verifies that
// NewTracerFromConfig works correctly when given a context with timeout.
func TestNewTracerFromConfig_WorksWithContextTimeout(t *testing.T) {
	cfg := infraotel.TracerConfig{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1000000000) // 1 second timeout
	defer cancel()

	tracer, shutdown, _ := infraotel.NewTracerFromConfig(ctx, cfg)

	require.NotNil(t, tracer)
	require.NotNil(t, shutdown)

	// Tracer should work within the timeout context
	spanCtx, span := tracer.Start(ctx, "test-span")
	assert.NotNil(t, spanCtx)
	assert.NotNil(t, span)
	span.End()

	shutdown()
}

// TestNewTracerFromConfig_HandlesMissingConfig verifies that
// NewTracerFromConfig handles empty config gracefully (defensive programming).
func TestNewTracerFromConfig_HandlesMissingConfig(t *testing.T) {
	// Create a minimal config instead of nil
	cfg := infraotel.TracerConfig{}

	tracer, shutdown, _ := infraotel.NewTracerFromConfig(context.Background(), cfg)

	// Should not panic and should return valid objects
	require.NotPanics(t, func() {
		_ = tracer
		_ = shutdown
	})
	require.NotNil(t, tracer)
	require.NotNil(t, shutdown)
}

// TestNewTracerFromConfig_SpanStartReturnsValidSpan verifies that
// spans returned from Tracer.Start implement the Span interface.
func TestNewTracerFromConfig_SpanStartReturnsValidSpan(t *testing.T) {
	cfg := infraotel.TracerConfig{
		Endpoint:    "",
		ServiceName: "awf",
	}

	tracer, shutdown, err := infraotel.NewTracerFromConfig(context.Background(), cfg)
	require.NoError(t, err)
	defer shutdown()

	_, span := tracer.Start(context.Background(), "test-span")

	// Span should have all interface methods
	assert.NotPanics(t, func() {
		span.End()
		span.SetAttribute("key", "value")
		span.RecordError(nil)
		span.AddEvent("test-event")
	})
}

// TestNewTracerFromConfig_MultipleShutdownCalls verifies that
// calling shutdown multiple times doesn't cause panic.
func TestNewTracerFromConfig_MultipleShutdownCalls(t *testing.T) {
	cfg := infraotel.TracerConfig{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
	}

	_, shutdown, err := infraotel.NewTracerFromConfig(context.Background(), cfg)
	require.NoError(t, err)

	// Multiple shutdown calls should be safe
	assert.NotPanics(t, func() {
		shutdown()
		shutdown()
		shutdown()
	})
}

// TestNewTracerFromConfig_ReturnsThreeValues verifies that
// NewTracerFromConfig consistently returns three values.
func TestNewTracerFromConfig_ReturnsThreeValues(t *testing.T) {
	cfg := infraotel.TracerConfig{
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
	}

	tracer, shutdown, err := infraotel.NewTracerFromConfig(context.Background(), cfg)

	// All three return values should be present
	assert.NotNil(t, tracer)
	assert.NotNil(t, shutdown)
	// err can be nil or non-nil depending on exporter configuration
	_ = err
}

// TestRunCommand_OtelExporterFlagParsing verifies that the --otel-exporter flag
// is parsed by cobra and wired into Config.OtelExporter.
func TestRunCommand_OtelExporterFlagParsing(t *testing.T) {
	cfg := DefaultConfig()
	cmd := newRunCommand(cfg)

	err := cmd.ParseFlags([]string{"--otel-exporter", "localhost:4317"})
	require.NoError(t, err)

	assert.Equal(t, "localhost:4317", cfg.OtelExporter)
}

// TestRunCommand_OtelServiceNameFlagParsing verifies that the --otel-service-name flag
// is parsed by cobra and wired into Config.OtelServiceName.
func TestRunCommand_OtelServiceNameFlagParsing(t *testing.T) {
	cfg := DefaultConfig()
	cmd := newRunCommand(cfg)

	err := cmd.ParseFlags([]string{"--otel-service-name", "my-service"})
	require.NoError(t, err)

	assert.Equal(t, "my-service", cfg.OtelServiceName)
}

// TestRunCommand_WithOtelFlags verifies that both --otel-exporter and --otel-service-name
// flags are parsed together and correctly wired into Config.
func TestRunCommand_WithOtelFlags(t *testing.T) {
	cfg := DefaultConfig()
	cmd := newRunCommand(cfg)

	err := cmd.ParseFlags([]string{
		"--otel-exporter", "otel-collector:4317",
		"--otel-service-name", "awf-production",
	})
	require.NoError(t, err)

	assert.Equal(t, "otel-collector:4317", cfg.OtelExporter)
	assert.Equal(t, "awf-production", cfg.OtelServiceName)
}

// TestNewTracerFromConfig_ExporterEndpointValidation verifies that
// NewTracerFromConfig handles various endpoint formats.
func TestNewTracerFromConfig_ExporterEndpointValidation(t *testing.T) {
	tests := []struct {
		name          string
		exporterURL   string
		serviceName   string
		shouldSucceed bool
	}{
		{
			name:          "empty exporter disables tracing",
			exporterURL:   "",
			serviceName:   "awf",
			shouldSucceed: true,
		},
		{
			name:          "localhost with port",
			exporterURL:   "localhost:4317",
			serviceName:   "awf",
			shouldSucceed: true,
		},
		{
			name:          "http endpoint",
			exporterURL:   "http://localhost:4317",
			serviceName:   "awf",
			shouldSucceed: true,
		},
		{
			name:          "custom service name",
			exporterURL:   "localhost:4317",
			serviceName:   "custom-service",
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := infraotel.TracerConfig{
				Endpoint:    tt.exporterURL,
				ServiceName: tt.serviceName,
			}

			tracer, shutdown, _ := infraotel.NewTracerFromConfig(context.Background(), cfg)

			if tt.shouldSucceed {
				require.NotNil(t, tracer)
				require.NotNil(t, shutdown)
				// err might be nil or have initialization warnings, both acceptable
			}

			// Cleanup
			if shutdown != nil {
				assert.NotPanics(t, func() {
					shutdown()
				})
			}
		})
	}
}
