package operation

import (
	"context"
	"sync"

	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
)

// Compile-time check that OperationRegistry implements ports.OperationProvider.
var _ ports.OperationProvider = (*OperationRegistry)(nil)

// OperationRegistry manages the lifecycle of executable operations.
// It provides thread-safe registration, unregistration, and lookup of operations.
// The registry implements ports.OperationProvider to integrate with ExecutionService.
type OperationRegistry struct {
	mu         sync.RWMutex
	operations map[string]Operation
}

// NewOperationRegistry creates an empty operation registry.
func NewOperationRegistry() *OperationRegistry {
	return &OperationRegistry{
		operations: make(map[string]Operation),
	}
}

// Register adds an operation to the registry.
// Returns ErrOperationAlreadyRegistered if the operation name is already registered.
// Returns ErrInvalidOperation if the operation is nil or has an empty name.
func (r *OperationRegistry) Register(op Operation) error {
	if op == nil || op.Name() == "" {
		return ErrInvalidOperation
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.operations[op.Name()]; exists {
		return ErrOperationAlreadyRegistered
	}

	r.operations[op.Name()] = op
	return nil
}

// Unregister removes an operation from the registry by name.
// Returns ErrOperationNotFound if the operation is not registered.
func (r *OperationRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.operations[name]; !exists {
		return ErrOperationNotFound
	}

	delete(r.operations, name)
	return nil
}

// Get retrieves an operation by name.
// Returns (operation, true) if found, (nil, false) if not found.
func (r *OperationRegistry) Get(name string) (Operation, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	op, found := r.operations[name]
	return op, found
}

// List returns all registered operations.
// Returns an empty slice if no operations are registered.
func (r *OperationRegistry) List() []Operation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.operations) == 0 {
		return []Operation{}
	}

	ops := make([]Operation, 0, len(r.operations))
	for _, op := range r.operations {
		ops = append(ops, op)
	}
	return ops
}

// GetOperation returns an operation schema by name.
// Implements ports.OperationProvider.
func (r *OperationRegistry) GetOperation(name string) (*plugin.OperationSchema, bool) {
	op, found := r.Get(name)
	if !found {
		return nil, false
	}
	return op.Schema(), true
}

// ListOperations returns all available operation schemas.
// Implements ports.OperationProvider.
func (r *OperationRegistry) ListOperations() []*plugin.OperationSchema {
	ops := r.List()
	if len(ops) == 0 {
		return []*plugin.OperationSchema{}
	}

	schemas := make([]*plugin.OperationSchema, 0, len(ops))
	for _, op := range ops {
		schemas = append(schemas, op.Schema())
	}
	return schemas
}

// Execute runs a registered operation with input validation.
// Implements ports.OperationProvider.
func (r *OperationRegistry) Execute(ctx context.Context, name string, inputs map[string]any) (*plugin.OperationResult, error) {
	op, found := r.Get(name)
	if !found {
		return nil, ErrOperationNotFound
	}

	schema := op.Schema()
	if schema == nil {
		return nil, ErrInvalidOperation
	}

	// Validate inputs and apply defaults
	if err := ValidateInputs(schema, inputs); err != nil {
		return nil, err
	}

	// Execute the operation
	return op.Execute(ctx, inputs)
}
