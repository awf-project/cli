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
