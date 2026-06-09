package application

import "github.com/awf-project/cli/internal/domain/ports"

// Drain consumes all events from session until its Events channel closes,
// then returns session.Err(). Rendering is the caller's responsibility — Drain
// performs no output itself. This is the single shared consumer helper (FR-015);
// interfaces MUST NOT reimplement it.
func Drain(session ports.RunSession) error {
	for range session.Events() { //nolint:revive // intentionally empty: caller renders events before calling Drain, or via a separate goroutine
	}
	return session.Err()
}
