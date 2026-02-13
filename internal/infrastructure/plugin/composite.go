package plugin

import (
	"context"
	"fmt"

	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
)

// CompositeOperationProvider wraps multiple OperationProvider instances into a single provider,
// delegating GetOperation/ListOperations/Execute by operation name.
// Enables coexistence of multiple built-in providers (e.g., github and notify).
type CompositeOperationProvider struct {
	providers []ports.OperationProvider
}

// NewCompositeOperationProvider creates a new composite operation provider
// that delegates to the given providers.
func NewCompositeOperationProvider(providers ...ports.OperationProvider) *CompositeOperationProvider {
	return &CompositeOperationProvider{
		providers: providers,
	}
}

// GetOperation returns an operation by name from the first provider that has it.
func (c *CompositeOperationProvider) GetOperation(name string) (*plugin.OperationSchema, bool) {
	for _, provider := range c.providers {
		if provider == nil {
			continue
		}
		if op, found := provider.GetOperation(name); found {
			return op, true
		}
	}
	return nil, false
}

// ListOperations returns all available operations from all providers.
func (c *CompositeOperationProvider) ListOperations() []*plugin.OperationSchema {
	var result []*plugin.OperationSchema
	for _, provider := range c.providers {
		if provider == nil {
			continue
		}
		ops := provider.ListOperations()
		result = append(result, ops...)
	}
	return result
}

// Execute runs a plugin operation by delegating to the appropriate provider.
func (c *CompositeOperationProvider) Execute(ctx context.Context, name string, inputs map[string]any) (*plugin.OperationResult, error) {
	// Find the provider that has this operation
	for _, provider := range c.providers {
		if provider == nil {
			continue
		}
		if _, found := provider.GetOperation(name); found {
			return provider.Execute(ctx, name, inputs)
		}
	}
	return nil, fmt.Errorf("operation not found: %s", name)
}
