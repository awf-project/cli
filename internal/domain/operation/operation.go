package operation

import (
	"context"
	"errors"

	"github.com/awf-project/awf/internal/domain/plugin"
)

// Operation defines an executable operation with typed inputs and schema.
// Operations can be HTTP requests, file I/O, transforms, or custom logic.
// Implementations must be safe for concurrent execution.
type Operation interface {
	// Name returns the unique operation identifier (e.g., "http.get", "file.read").
	Name() string

	// Execute runs the operation with provided inputs.
	// Context is used for cancellation and timeout propagation.
	// Inputs must be validated against Schema() before calling Execute.
	Execute(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error)

	// Schema returns the operation metadata including input/output definitions.
	Schema() *plugin.OperationSchema
}

// Sentinel errors for operation lifecycle and execution.
var (
	// ErrOperationAlreadyRegistered indicates duplicate registration attempt.
	ErrOperationAlreadyRegistered = errors.New("operation already registered")

	// ErrOperationNotFound indicates operation lookup failure.
	ErrOperationNotFound = errors.New("operation not found")

	// ErrInvalidOperation indicates operation fails validation constraints.
	ErrInvalidOperation = errors.New("invalid operation")

	// ErrInvalidInputs indicates operation inputs fail schema validation.
	ErrInvalidInputs = errors.New("invalid inputs")
)
