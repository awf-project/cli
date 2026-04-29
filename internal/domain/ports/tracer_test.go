package ports

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpanInterface_Defined(t *testing.T) {
	var _ Span = (*NopSpan)(nil)
}

func TestTracerInterface_Defined(t *testing.T) {
	var _ Tracer = (*NopTracer)(nil)
}

func TestNopSpan_End_DoesNotPanic(t *testing.T) {
	span := NopSpan{}
	assert.NotPanics(t, func() {
		span.End()
	})
}

func TestNopSpan_SetAttribute_AcceptsKeyAndValue(t *testing.T) {
	span := NopSpan{}
	assert.NotPanics(t, func() {
		span.SetAttribute("key", "value")
		span.SetAttribute("int", 42)
		span.SetAttribute("bool", true)
		span.SetAttribute("nil", nil)
	})
}

func TestNopSpan_RecordError_AcceptsError(t *testing.T) {
	span := NopSpan{}
	assert.NotPanics(t, func() {
		span.RecordError(nil)
		span.RecordError(errors.New("test error"))
	})
}

func TestNopSpan_AddEvent_AcceptsEventName(t *testing.T) {
	span := NopSpan{}
	assert.NotPanics(t, func() {
		span.AddEvent("event-name")
		span.AddEvent("")
	})
}

func TestNopTracer_Start_ReturnsContextAndSpan(t *testing.T) {
	tracer := NopTracer{}
	ctx := context.Background()

	newCtx, span := tracer.Start(ctx, "test-span")

	assert.NotNil(t, newCtx)
	assert.NotNil(t, span)
	assert.Equal(t, ctx, newCtx)
}

func TestNopTracer_Start_ReturnedSpanIsNopSpan(t *testing.T) {
	tracer := NopTracer{}
	ctx := context.Background()

	_, span := tracer.Start(ctx, "test-span")

	_, ok := span.(NopSpan)
	assert.True(t, ok, "span should be a NopSpan")
}

func TestNopTracer_Start_SpanIsCallable(t *testing.T) {
	tracer := NopTracer{}
	ctx := context.Background()

	_, span := tracer.Start(ctx, "test-span")

	assert.NotPanics(t, func() {
		span.End()
		span.SetAttribute("key", "value")
		span.RecordError(errors.New("error"))
		span.AddEvent("event")
	})
}

func TestNopTracer_Start_WithEmptySpanName(t *testing.T) {
	tracer := NopTracer{}
	ctx := context.Background()

	newCtx, span := tracer.Start(ctx, "")

	assert.NotNil(t, newCtx)
	assert.NotNil(t, span)
}

func TestNopTracer_Start_WithContextualContext(t *testing.T) {
	tracer := NopTracer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	newCtx, span := tracer.Start(ctx, "test-span")

	assert.Equal(t, ctx, newCtx)
	assert.NotNil(t, span)
}
