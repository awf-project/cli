package mocks

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMockTracer_Start_CreatesSpan verifies Start returns the original context,
// a non-nil ports.Span, and records the span name.
func TestMockTracer_Start_CreatesSpan(t *testing.T) {
	tracer := NewMockTracer()
	ctx := context.Background()

	returnedCtx, span := tracer.Start(ctx, "test-span")

	require.NotNil(t, span)
	_ = span // ensure span is of type ports.Span
	assert.Equal(t, ctx, returnedCtx)
	assert.Equal(t, "test-span", span.(*MockSpan).Record().Name)
}

func TestMockTracer_Start_EmptySpanName(t *testing.T) {
	tracer := NewMockTracer()

	_, span := tracer.Start(context.Background(), "")

	require.NotNil(t, span)
	assert.Equal(t, "", span.(*MockSpan).Record().Name)
}

func TestMockSpan_End_MarksSpanEnded(t *testing.T) {
	tracer := NewMockTracer()
	_, span := tracer.Start(context.Background(), "test-span")
	mockSpan := span.(*MockSpan)

	assert.False(t, mockSpan.Record().Ended)
	span.End()
	assert.True(t, mockSpan.Record().Ended)
}

func TestMockSpan_SetAttribute_StoresValue(t *testing.T) {
	tracer := NewMockTracer()
	_, span := tracer.Start(context.Background(), "test-span")

	span.SetAttribute("key", "value")
	span.SetAttribute("count", 42)

	record := span.(*MockSpan).Record()
	assert.Equal(t, "value", record.Attributes["key"])
	assert.Equal(t, 42, record.Attributes["count"])
}

func TestMockSpan_SetAttribute_OverwritesExistingKey(t *testing.T) {
	tracer := NewMockTracer()
	_, span := tracer.Start(context.Background(), "test-span")

	span.SetAttribute("key", "old")
	span.SetAttribute("key", "new")

	assert.Equal(t, "new", span.(*MockSpan).Record().Attributes["key"])
}

// TestMockSpan_SetAttribute_NilValue verifies that a nil value is stored without panic.
func TestMockSpan_SetAttribute_NilValue(t *testing.T) {
	tracer := NewMockTracer()
	_, span := tracer.Start(context.Background(), "test-span")

	span.SetAttribute("key", nil)

	assert.Nil(t, span.(*MockSpan).Record().Attributes["key"])
}

func TestMockSpan_RecordError_StoresError(t *testing.T) {
	tracer := NewMockTracer()
	_, span := tracer.Start(context.Background(), "test-span")
	err := errors.New("something failed")

	span.RecordError(err)

	record := span.(*MockSpan).Record()
	require.Len(t, record.Errors, 1)
	assert.Equal(t, err, record.Errors[0])
}

// TestMockSpan_RecordError_NilError verifies that a nil error is recorded without panic.
func TestMockSpan_RecordError_NilError(t *testing.T) {
	tracer := NewMockTracer()
	_, span := tracer.Start(context.Background(), "test-span")

	span.RecordError(nil)

	record := span.(*MockSpan).Record()
	require.Len(t, record.Errors, 1)
	assert.Nil(t, record.Errors[0])
}

func TestMockSpan_AddEvent_RecordsEvent(t *testing.T) {
	tracer := NewMockTracer()
	_, span := tracer.Start(context.Background(), "test-span")

	span.AddEvent("started")
	span.AddEvent("completed")

	record := span.(*MockSpan).Record()
	require.Len(t, record.Events, 2)
	assert.Equal(t, "started", record.Events[0])
	assert.Equal(t, "completed", record.Events[1])
}

func TestMockTracer_GetSpans_ReturnsAll(t *testing.T) {
	tracer := NewMockTracer()
	assert.Len(t, tracer.GetSpans(), 0)

	tracer.Start(context.Background(), "span1")
	tracer.Start(context.Background(), "span2")

	spans := tracer.GetSpans()
	require.Len(t, spans, 2)
	assert.Equal(t, "span1", spans[0].Record().Name)
	assert.Equal(t, "span2", spans[1].Record().Name)
}

func TestMockTracer_Clear_RemovesSpans(t *testing.T) {
	tracer := NewMockTracer()
	tracer.Start(context.Background(), "span1")
	tracer.Start(context.Background(), "span2")

	tracer.Clear()

	assert.Len(t, tracer.GetSpans(), 0)
}

func TestMockTracer_InterfaceCompliance(t *testing.T) {
	var _ ports.Tracer = NewMockTracer()
	var _ ports.Span = &MockSpan{}
}

// TestMockTracer_CompleteWorkflow exercises the full span lifecycle: attribute
// mutation, error recording, event ordering, and End marking.
func TestMockTracer_CompleteWorkflow(t *testing.T) {
	tracer := NewMockTracer()

	_, span := tracer.Start(context.Background(), "operation")
	span.SetAttribute("status", "running")
	span.AddEvent("started")
	span.RecordError(errors.New("transient error"))
	span.AddEvent("retried")
	span.SetAttribute("status", "done")
	span.End()

	spans := tracer.GetSpans()
	require.Len(t, spans, 1)
	record := spans[0].Record()
	assert.Equal(t, "operation", record.Name)
	assert.True(t, record.Ended)
	assert.Equal(t, "done", record.Attributes["status"])
	assert.Len(t, record.Events, 2)
	assert.Len(t, record.Errors, 1)
}
