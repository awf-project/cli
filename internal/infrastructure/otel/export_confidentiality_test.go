package otel

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type confidentialityErrorExporter struct{}

func (confidentialityErrorExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	return errors.New("PRIVATE_EXPORT_RESPONSE_CANARY")
}
func (confidentialityErrorExporter) Shutdown(context.Context) error { return nil }
func TestTelemetryErrorConfidentiality(t *testing.T) {
	provider := newProviderWithExporter(confidentialityErrorExporter{}, "audit")
	_, span := provider.Start(context.Background(), "operation")
	span.End()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := provider.Shutdown(ctx)
	if err != nil && strings.Contains(err.Error(), "PRIVATE_EXPORT_RESPONSE_CANARY") {
		t.Errorf("T040: shutdown error exposes exporter response: %v", err)
	}
}
