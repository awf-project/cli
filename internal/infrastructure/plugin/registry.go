package plugin

import (
	"errors"
	"sync"

	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
)

// Registry errors.
var (
	ErrOperationAlreadyRegistered = errors.New("operation already registered")
	ErrOperationNotFound          = errors.New("operation not found")
	ErrInvalidOperation           = errors.New("invalid operation schema")
	ErrRegistryNotImplemented     = errors.New("registry: not implemented")
)

// OperationRegistry manages registration of plugin-provided operations.
// Thread-safe for concurrent access.
type OperationRegistry struct {
	mu         sync.RWMutex
	operations map[string]*plugin.OperationSchema // key: operation name
	sources    map[string]string                  // key: operation name, value: plugin name
}

// Compile-time interface check.
var _ ports.PluginRegistry = (*OperationRegistry)(nil)

// NewOperationRegistry creates a new empty operation registry.
func NewOperationRegistry() *OperationRegistry {
	return &OperationRegistry{
		operations: make(map[string]*plugin.OperationSchema),
		sources:    make(map[string]string),
	}
}

// RegisterOperation adds a plugin operation to the registry.
// Returns ErrOperationAlreadyRegistered if an operation with the same name exists.
// Returns ErrInvalidOperation if the operation schema is nil or has no name.
func (r *OperationRegistry) RegisterOperation(op *plugin.OperationSchema) error {
	if op == nil || op.Name == "" {
		return ErrInvalidOperation
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.operations[op.Name]; exists {
		return ErrOperationAlreadyRegistered
	}

	r.operations[op.Name] = op
	r.sources[op.Name] = op.PluginName

	return nil
}

// UnregisterOperation removes a plugin operation from the registry.
// Returns ErrOperationNotFound if the operation is not registered.
func (r *OperationRegistry) UnregisterOperation(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.operations[name]; !exists {
		return ErrOperationNotFound
	}

	delete(r.operations, name)
	delete(r.sources, name)

	return nil
}

// Operations returns all registered operations as a slice.
// Returns an empty slice if no operations are registered.
func (r *OperationRegistry) Operations() []*plugin.OperationSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*plugin.OperationSchema, 0, len(r.operations))
	for _, op := range r.operations {
		result = append(result, op)
	}

	return result
}

// GetOperation returns an operation by name.
// Returns nil and false if the operation is not found.
func (r *OperationRegistry) GetOperation(name string) (*plugin.OperationSchema, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	op, exists := r.operations[name]
	return op, exists
}

// UnregisterPluginOperations removes all operations provided by a specific plugin.
// Useful when unloading or disabling a plugin.
func (r *OperationRegistry) UnregisterPluginOperations(pluginName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Collect operation names to remove
	toRemove := make([]string, 0)
	for name, source := range r.sources {
		if source == pluginName {
			toRemove = append(toRemove, name)
		}
	}

	// Remove collected operations
	for _, name := range toRemove {
		delete(r.operations, name)
		delete(r.sources, name)
	}

	return nil
}

// GetPluginOperations returns all operations registered by a specific plugin.
func (r *OperationRegistry) GetPluginOperations(pluginName string) []*plugin.OperationSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*plugin.OperationSchema, 0)
	for name, source := range r.sources {
		if source == pluginName {
			if op, exists := r.operations[name]; exists {
				result = append(result, op)
			}
		}
	}

	return result
}

// Count returns the total number of registered operations.
func (r *OperationRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.operations)
}

// GetOperationSource returns the plugin name that registered an operation.
// Returns empty string and false if the operation is not found.
func (r *OperationRegistry) GetOperationSource(operationName string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	source, exists := r.sources[operationName]
	return source, exists
}

// Clear removes all registered operations. Useful for testing.
func (r *OperationRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.operations = make(map[string]*plugin.OperationSchema)
	r.sources = make(map[string]string)
}
