package ports_test

import (
	"context"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRecorder is a minimal in-memory Recorder for contract verification.
type fakeRecorder struct {
	mu          sync.Mutex
	closed      bool
	subscribers []chan transcript.ExchangeEvent
}

func (f *fakeRecorder) Record(ctx context.Context, event transcript.ExchangeEvent) error {
	if event.Type == "" {
		return ports.ErrInvalidEvent
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ch := range f.subscribers {
		ch <- event
	}
	return nil
}

func (f *fakeRecorder) Subscribe() (<-chan transcript.ExchangeEvent, func()) {
	ch := make(chan transcript.ExchangeEvent, 16)
	f.mu.Lock()
	f.subscribers = append(f.subscribers, ch)
	f.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			f.mu.Lock()
			defer f.mu.Unlock()
			for i, s := range f.subscribers {
				if s == ch {
					f.subscribers = append(f.subscribers[:i], f.subscribers[i+1:]...)
					close(ch)
					break
				}
			}
		})
	}
	return ch, cancel
}

func (f *fakeRecorder) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	return nil
}

func TestRecorderContract_FakeIdempotentClose(t *testing.T) {
	rec := &fakeRecorder{}
	require.NoError(t, rec.Close())
	assert.NoError(t, rec.Close())
}

func TestRecorderContract_SubscribeCancelIdempotent(t *testing.T) {
	rec := &fakeRecorder{}
	_, cancel := rec.Subscribe()
	cancel()
	assert.NotPanics(t, func() { cancel() })
}

func TestRecorderContract_RecordNilEventReturnsError(t *testing.T) {
	rec := &fakeRecorder{}
	err := rec.Record(context.Background(), transcript.ExchangeEvent{})
	assert.ErrorIs(t, err, ports.ErrInvalidEvent)
}

// TestRecorderContract_RecordRespectsCancelation verifies that Record respects
// context cancellation and returns the context error.
func TestRecorderContract_RecordRespectsCancelation(t *testing.T) {
	rec := &fakeRecorder{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := transcript.ExchangeEvent{
		Type: transcript.EventTypeRunStarted,
	}

	err := rec.Record(ctx, event)

	assert.ErrorIs(t, err, context.Canceled)
}
