package application

import (
	"context"
	"sync"
	"time"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
)

type InputBridge struct {
	session    *RunSession
	responseCh chan ports.InputResponse
	mu         sync.Mutex
	closed     bool
	dispatched bool // true once Respond deposits a value; reset at each ReadInput entry
}

func NewInputBridge(session *RunSession) *InputBridge {
	return &InputBridge{
		session:    session,
		responseCh: make(chan ports.InputResponse, 1),
	}
}

func (b *InputBridge) ReadInput(ctx context.Context, req ports.InputRequest) (ports.InputResponse, error) {
	b.session.appendEvent(ports.Event{
		Kind:      ports.EventInputRequired,
		RunID:     b.session.id,
		Payload:   req,
		Timestamp: time.Now(),
	})

	// Reset dispatched and drain any stale value left from a previous call that
	// exited via ctx.Done() or session.ctx.Done() without consuming responseCh.
	b.mu.Lock()
	select {
	case <-b.responseCh:
	default:
	}
	b.dispatched = false
	b.mu.Unlock()

	select {
	case resp := <-b.responseCh:
		return resp, nil
	case <-ctx.Done():
		// Drain any value that arrived concurrently with the cancellation so it
		// does not leak into the next ReadInput call.
		b.mu.Lock()
		select {
		case <-b.responseCh:
		default:
		}
		b.dispatched = false
		b.mu.Unlock()
		return ports.InputResponse{}, ctx.Err()
	case <-b.session.ctx.Done():
		// Same drain on session context cancellation.
		b.mu.Lock()
		select {
		case <-b.responseCh:
		default:
		}
		b.dispatched = false
		b.mu.Unlock()
		return ports.InputResponse{}, b.session.ctx.Err()
	}
}

// Respond delivers a response to the parked ReadInput. Non-blocking: uses cap-1
// responseCh plus a dispatched flag so duplicate Responds are rejected even after
// the goroutine has already consumed the first value from the channel.
func (b *InputBridge) Respond(r ports.InputResponse) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ports.ErrSessionClosed
	}
	if b.dispatched {
		return ports.ErrDuplicateResponse
	}

	select {
	case b.responseCh <- r:
		b.dispatched = true
		return nil
	default:
		// Channel full and dispatched=false should not occur in normal flow,
		// but treat defensively as a duplicate.
		b.dispatched = true
		return ports.ErrDuplicateResponse
	}
}

func (b *InputBridge) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	b.mu.Unlock()

	b.session.appendEvent(ports.Event{
		Kind:      ports.EventWorkflowFailed,
		RunID:     b.session.id,
		Payload:   domainerrors.ErrorCodeUserFacadeSessionClosed,
		Timestamp: time.Now(),
	})
	b.session.cancel()
}
