package application

import (
	"context"

	"github.com/awf-project/cli/internal/domain/ports"
)

// Drain consumes all events from session until its Events channel closes,
// then returns session.Err(). Rendering is the caller's responsibility — Drain
// performs no output itself. This is the single shared consumer helper (FR-015);
// interfaces MUST NOT reimplement it.
//
// Drain blocks forever if the session never closes. Use DrainContext when a
// deadline or cancellation signal is available.
func Drain(session ports.RunSession) error {
	for range session.Events() { //nolint:revive // intentionally empty: caller renders events before calling Drain, or via a separate goroutine
	}
	return session.Err()
}

// DrainContext consumes all events from session until its Events channel closes or ctx
// is cancelled (whichever comes first). When ctx is cancelled before the session closes,
// DrainContext returns ctx.Err(). When the session closes normally, it returns
// session.Err() (nil on success, the execution error on failure).
//
// Use DrainContext in preference to Drain whenever a deadline or parent context is
// available; it prevents goroutine leaks when the workflow is slow or hung.
//
// Drain is a thin wrapper around DrainContext with context.Background().
func DrainContext(ctx context.Context, session ports.RunSession) error {
	events := session.Events()
	for {
		select {
		case _, ok := <-events:
			if !ok {
				// Channel closed: session has finished (or was closed).
				return session.Err()
			}
			// Event received — continue draining; rendering is caller's responsibility.
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
