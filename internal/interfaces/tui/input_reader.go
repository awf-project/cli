package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/awf-project/cli/internal/domain/ports"
)

var _ ports.UserInputReader = (*TUIInputReader)(nil)

// MsgSender is a function that sends a tea.Msg to the Bubble Tea program.
// Typically bound to (*tea.Program).Send.
type MsgSender func(msg tea.Msg)

// TUIInputReader implements ports.UserInputReader for the TUI.
// It bridges the blocking ConversationManager goroutine with the Bubble Tea
// event loop via channels.
type TUIInputReader struct {
	requestCh  chan struct{}
	responseCh chan string
	sender     MsgSender
}

// NewTUIInputReader creates a TUIInputReader. sender may be nil during tests;
// when non-nil it is called to notify the Bubble Tea program that input is needed.
func NewTUIInputReader(sender MsgSender) *TUIInputReader {
	return &TUIInputReader{
		requestCh:  make(chan struct{}, 1),
		responseCh: make(chan string, 1),
		sender:     sender,
	}
}

// SetSender sets the tea.Msg sender (typically (*tea.Program).Send).
// Called after the program is created but before any execution starts.
func (r *TUIInputReader) SetSender(sender MsgSender) {
	r.sender = sender
}

// ReadInput blocks until the user submits input via the TUI or the context
// is cancelled. It signals the Bubble Tea model that input is needed by
// sending InputRequestedMsg.
func (r *TUIInputReader) ReadInput(ctx context.Context) (string, error) {
	select {
	case r.requestCh <- struct{}{}:
	default:
	}

	if r.sender != nil {
		r.sender(InputRequestedMsg{})
	}

	select {
	case text := <-r.responseCh:
		return text, nil
	case <-ctx.Done():
		return "", fmt.Errorf("input cancelled: %w", ctx.Err())
	}
}

// RequestCh returns the channel that signals when input is requested.
// Used in tests; the TUI model uses InputRequestedMsg instead.
func (r *TUIInputReader) RequestCh() <-chan struct{} {
	return r.requestCh
}

// Respond sends user input back to the blocked ReadInput call.
func (r *TUIInputReader) Respond(text string) {
	r.responseCh <- text
}
