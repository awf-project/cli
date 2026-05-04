package otel

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
)

// TracerConfig holds parameters for creating an OpenTelemetry tracer.
type TracerConfig struct {
	Endpoint    string
	ServiceName string
}

// NewTracerFromConfig creates a tracer from the given config.
// Returns NopTracer when Endpoint is empty (tracing disabled).
// The caller must defer the returned shutdown func.
func NewTracerFromConfig(ctx context.Context, cfg TracerConfig) (ports.Tracer, func(), error) {
	noop := func() {}

	if cfg.Endpoint == "" {
		return ports.NopTracer{}, noop, nil
	}

	endpoint := cfg.Endpoint
	insecure := true
	if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
		insecure = false
	} else if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
	}

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "awf"
	}

	tp, err := NewTracerProvider(ctx, Config{
		Endpoint:    endpoint,
		ServiceName: serviceName,
		Insecure:    insecure,
	})
	if err != nil {
		return ports.NopTracer{}, noop, err
	}

	var once sync.Once
	shutdown := func() {
		once.Do(func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tp.Shutdown(shutdownCtx) //nolint:errcheck // best-effort shutdown; caller cannot act on this
		})
	}

	return tp, shutdown, nil
}
