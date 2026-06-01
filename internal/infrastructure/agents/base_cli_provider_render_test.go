package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/pkg/display"
)

func TestApplyStreamFilter_UsesRendererFromContext(t *testing.T) {
	b := &baseCLIProvider{
		name: "fake",
		hooks: cliProviderHooks{
			parseDisplayEvents: func(line []byte) []display.DisplayEvent {
				return []display.DisplayEvent{{Kind: display.EventText, Text: string(line)}}
			},
		},
	}

	var rendered []display.DisplayEvent
	r := display.EventRenderer(func(events []display.DisplayEvent) {
		rendered = append(rendered, events...)
	})
	ctx := display.WithRenderer(context.Background(), r)

	var sink countingWriter
	wrapped, filter := b.applyStreamFilter(ctx, &sink, false)
	if filter == nil {
		t.Fatal("expected a StreamFilterWriter when parser present")
	}
	_, _ = wrapped.Write([]byte("hello\n"))
	_ = filter.Flush()

	if len(rendered) == 0 || rendered[0].Text != "hello" {
		t.Fatalf("renderer not invoked from context: %+v", rendered)
	}
	if sink.n != 0 {
		t.Fatalf("inner writer should be discarded when renderer present, wrote %d bytes", sink.n)
	}
}

func TestApplyStreamFilter_NoRenderer_WritesToInner(t *testing.T) {
	b := &baseCLIProvider{
		name: "fake",
		hooks: cliProviderHooks{
			parseDisplayEvents: func(line []byte) []display.DisplayEvent {
				return []display.DisplayEvent{{Kind: display.EventText, Text: string(line)}}
			},
		},
	}
	var sink countingWriter
	wrapped, filter := b.applyStreamFilter(context.Background(), &sink, false)
	if filter == nil {
		t.Fatal("expected filter")
	}
	_, _ = wrapped.Write([]byte("hello\n"))
	_ = filter.Flush()
	if sink.n == 0 {
		t.Fatal("inner writer should receive text when no renderer present")
	}
}

func TestApplyStreamFilter_NilStdout_WithParser_ReturnsFilterNotNil(t *testing.T) {
	// When stdout is nil but a parser is present, applyStreamFilter must still
	// return a non-nil StreamFilterWriter backed by io.Discard so that display
	// events are parsed. Previously the function returned (nil, nil) in this
	// path, causing display events to be lost silently.
	b := &baseCLIProvider{
		name: "fake",
		hooks: cliProviderHooks{
			parseDisplayEvents: func(line []byte) []display.DisplayEvent {
				return []display.DisplayEvent{{Kind: display.EventText, Text: string(line)}}
			},
		},
	}

	var parsedEvents []display.DisplayEvent
	// Use a nil renderer — we are testing the nil-stdout path, not the renderer path.
	_ = parsedEvents

	wrapped, filter := b.applyStreamFilter(context.Background(), nil, false)

	if filter == nil {
		t.Fatal("applyStreamFilter must return a non-nil StreamFilterWriter when parser is present and stdout is nil")
	}
	if wrapped == nil {
		t.Fatal("applyStreamFilter must return a non-nil writer when parser is present and stdout is nil")
	}
	// Writing to the returned writer must not panic (backed by io.Discard).
	_, err := wrapped.Write([]byte("event line\n"))
	if err != nil {
		t.Fatalf("write to Discard-backed filter must not error: %v", err)
	}
	if flushErr := filter.Flush(); flushErr != nil {
		t.Fatalf("flush must not error: %v", flushErr)
	}
}

func TestApplyStreamFilter_NilStdout_NilParser_ReturnsNilPair(t *testing.T) {
	// When both stdout and parser are nil, applyStreamFilter returns (nil, nil) —
	// this is the pass-through path for providers that do not emit display events.
	b := &baseCLIProvider{
		name:  "fake",
		hooks: cliProviderHooks{},
	}
	wrapped, filter := b.applyStreamFilter(context.Background(), nil, false)
	if wrapped != nil || filter != nil {
		t.Fatalf("expected (nil, nil) when no parser; got wrapped=%v filter=%v", wrapped, filter)
	}
}

type countingWriter struct{ n int }

func (w *countingWriter) Write(p []byte) (int, error) { w.n += len(p); return len(p), nil }
