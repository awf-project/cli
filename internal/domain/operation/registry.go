package operation

import (
	"context"
	"sync"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
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

func (r *OperationRegistry) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	op, found := r.Get(name)
	if !found {
		return nil, false
	}
	return op.Schema(), true
}

func (r *OperationRegistry) ListOperations() []*pluginmodel.OperationSchema {
	ops := r.List()
	if len(ops) == 0 {
		return []*pluginmodel.OperationSchema{}
	}

	schemas := make([]*pluginmodel.OperationSchema, 0, len(ops))
	for _, op := range ops {
		schemas = append(schemas, op.Schema())
	}
	return schemas
}

func (r *OperationRegistry) Execute(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
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
