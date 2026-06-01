package acp

import (
	"context"
	"fmt"

	"github.com/awf-project/cli/internal/domain/ports"
)

var _ ports.UserInputReader = (*ACPInputReader)(nil)

// EndTurnNotifier is called exactly once per ReadInput entry to signal the ACP
// serve loop that the current turn should close with stopReason = end_turn.
type EndTurnNotifier func()

// ParkHook is invoked by ReadInput around the blocking wait for user input. OnPark
// fires immediately before the goroutine parks on the response channel; OnUnpark
// fires once the wait completes (whether a response arrived or the context was
// cancelled). The two hooks always pair: every OnPark is followed by exactly one
// OnUnpark, which lets the application layer maintain a balanced parked-turn counter
// (ACPSession.ParkedTurnCount) without the infrastructure layer importing it.
//
// Both hooks are optional (nil is a no-op). They run on the workflow goroutine, so
// implementations must be cheap and non-blocking; an atomic increment/decrement is
// the intended use.
type ParkHook func()

// ACPInputReader bridges a workflow goroutine running inside ConversationManager
// across multiple ACP turns. It mirrors the TUIInputReader channel pattern,
// substituting EndTurnNotifier for the Bubble Tea MsgSender side-effect.
//
// The reader carries no internal turn counter: the buffered responseCh is the sole
// synchronization primitive, and park accounting is delegated to the caller via the
// OnPark/OnUnpark hooks (kept lock-free and application-agnostic by design).
type ACPInputReader struct {
	responseCh chan string
	notifier   EndTurnNotifier
	onPark     ParkHook
	onUnpark   ParkHook
}

// NewACPInputReader creates an ACPInputReader. The buffered responseCh of size 1
// enforces the one-Respond-per-ReadInput contract without blocking the caller.
func NewACPInputReader(notifier EndTurnNotifier) *ACPInputReader {
	return &ACPInputReader{
		responseCh: make(chan string, 1),
		notifier:   notifier,
	}
}

// SetParkHooks installs the OnPark/OnUnpark callbacks invoked around the blocking
// wait in ReadInput. Passing nil for either hook disables that side. Intended to be
// called once at wiring time, before the reader is handed to a workflow goroutine;
// it is not safe to call concurrently with ReadInput.
//
// The parameters are plain func() (not the named ParkHook type) so this method satisfies
// the application-layer ACPInputResponder interface, whose SetParkHooks signature uses
// func() to avoid importing this package's ParkHook type.
func (r *ACPInputReader) SetParkHooks(onPark, onUnpark func()) {
	r.onPark = onPark
	r.onUnpark = onUnpark
}

// ReadInput blocks until Respond is called or ctx is cancelled.
// It fires the EndTurnNotifier exactly once per call on entry, then invokes OnPark
// immediately before parking and OnUnpark once the wait resolves.
func (r *ACPInputReader) ReadInput(ctx context.Context) (string, error) {
	if r.notifier != nil {
		r.notifier()
	}

	if r.onPark != nil {
		r.onPark()
	}
	if r.onUnpark != nil {
		defer r.onUnpark()
	}

	select {
	case text := <-r.responseCh:
		return text, nil
	case <-ctx.Done():
		return "", fmt.Errorf("input cancelled: %w", ctx.Err())
	}
}

// Respond unblocks the goroutine parked in ReadInput. Non-blocking: if no reader
// is currently parked the send is dropped (documented contract: one Respond per ReadInput).
// A dropped send indicates a protocol bug in the caller (double Respond without a matching
// ReadInput). No logger is available here without changing the constructor signature; the
// caller is responsible for ensuring the one-Respond-per-ReadInput contract is upheld.
// If logging of drops becomes necessary, add a ports.Logger to NewACPInputReader.
func (r *ACPInputReader) Respond(text string) {
	select {
	case r.responseCh <- text:
	default:
		// double Respond dropped — protocol bug: caller sent Respond without a parked ReadInput
	}
}
