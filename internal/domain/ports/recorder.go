package ports

import (
	"context"
	"errors"

	"github.com/awf-project/cli/internal/domain/transcript"
)

// ErrInvalidEvent is returned when a Record call is made with a zero-Type event.
var ErrInvalidEvent = errors.New("invalid event: Type must not be empty")

// RecorderFactory creates a new Recorder that writes to path.
// The parent directory of path must already exist.
type RecorderFactory func(path string) (Recorder, error)

// Recorder is the port for writing and broadcasting transcript exchange events.
type Recorder interface {
	// Record appends the event to the transcript. Returns ErrInvalidEvent when
	// event.Type is empty. Honors ctx cancellation.
	Record(ctx context.Context, event transcript.ExchangeEvent) error

	// Subscribe returns a channel that receives every subsequent recorded event
	// and a cancel closure that unregisters the subscriber and closes the channel.
	// Calling cancel more than once is a no-op.
	Subscribe() (ch <-chan transcript.ExchangeEvent, cancel func())

	// Close flushes and releases resources. Idempotent: second call returns nil.
	Close() error
}
