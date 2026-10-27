package ports

import "context"

// UserInputReader defines the contract for reading user input between conversation turns.
// Driven port — called by ConversationManager to get the next user message.
type UserInputReader interface {
	// ReadInput reads the next user message.
	// Returns empty string when the user submits no input (signals conversation end).
	// Returns error if the context is cancelled or an I/O error occurs.
	ReadInput(ctx context.Context) (string, error)
}

// UserInputPreflighter optionally checks input availability without consuming a
// message or blocking for user interaction. Injected readers without this
// capability retain their existing contract.
type UserInputPreflighter interface {
	PreflightInput(ctx context.Context) error
}
