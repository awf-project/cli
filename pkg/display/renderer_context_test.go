package display

import (
	"context"
	"testing"
)

func TestRendererContext_RoundTrip(t *testing.T) {
	var got []DisplayEvent
	r := EventRenderer(func(events []DisplayEvent) { got = events })

	ctx := WithRenderer(context.Background(), r)
	out := RendererFromContext(ctx)
	if out == nil {
		t.Fatal("expected renderer from context, got nil")
	}
	out([]DisplayEvent{{Kind: EventText, Text: "hi"}})
	if len(got) != 1 || got[0].Text != "hi" {
		t.Fatalf("renderer not invoked correctly: %+v", got)
	}
}

func TestRendererFromContext_Absent_ReturnsNil(t *testing.T) {
	if RendererFromContext(context.Background()) != nil {
		t.Fatal("expected nil renderer when absent")
	}
}
