package transcript

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/transcript"
)

func TestNopRecorder_RecordReturnsNil(t *testing.T) {
	tests := []struct {
		name  string
		event transcript.ExchangeEvent
	}{
		{name: "zero event", event: transcript.ExchangeEvent{}},
		{name: "non-zero event", event: transcript.ExchangeEvent{Type: "test.event"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := NewNopRecorder()
			err := rec.Record(context.Background(), tc.event)
			require.NoError(t, err)
		})
	}
}

func TestNopRecorder_SubscribeReturnsClosedChannel(t *testing.T) {
	rec := NewNopRecorder()

	ch, cancel := rec.Subscribe()
	require.NotNil(t, ch)
	require.NotNil(t, cancel)

	_, ok := <-ch
	require.False(t, ok, "channel must be pre-closed")

	cancel()
	cancel()
}

func TestNopRecorder_CloseIsIdempotent(t *testing.T) {
	rec := NewNopRecorder()
	require.NoError(t, rec.Close())
	require.NoError(t, rec.Close())
}

func TestNopRecorder_ZeroGoroutinesSpawned(t *testing.T) {
	// Criterion #5: no goroutines are spawned by Record/Subscribe/Close after construction.
	rec := NewNopRecorder()
	runtime.Gosched()
	before := runtime.NumGoroutine()

	_ = rec.Record(context.Background(), transcript.ExchangeEvent{Type: "test"})
	ch, cancel := rec.Subscribe()
	<-ch
	cancel()
	_ = rec.Close()

	runtime.Gosched()
	after := runtime.NumGoroutine()
	require.Equal(t, before, after, "NopRecorder must not spawn goroutines")
}
