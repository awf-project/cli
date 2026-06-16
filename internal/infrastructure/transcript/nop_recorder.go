package transcript

import (
	"context"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
)

// NopRecorder is the single canonical no-op implementation of ports.Recorder.
// It replaces the former private per-interface no-op recorder types (CLI and TUI),
// which were identical in behavior. Subscribe returns a pre-closed channel so any
// accidental consumer terminates immediately without blocking.
type NopRecorder struct {
	closed chan transcript.ExchangeEvent
}

// NewNopRecorder returns a NopRecorder with its closed channel pre-allocated.
func NewNopRecorder() *NopRecorder {
	ch := make(chan transcript.ExchangeEvent)
	close(ch)
	return &NopRecorder{closed: ch}
}

var _ ports.Recorder = (*NopRecorder)(nil)

func (n *NopRecorder) Record(_ context.Context, _ transcript.ExchangeEvent) error { //nolint:gocritic // hugeParam: ports.Recorder contract requires value type
	return nil
}

func (n *NopRecorder) Subscribe() (ch <-chan transcript.ExchangeEvent, cancel func()) {
	return n.closed, func() {}
}

func (n *NopRecorder) Close() error { return nil }
