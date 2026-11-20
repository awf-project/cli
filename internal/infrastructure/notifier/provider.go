package notifier

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
)

var _ ports.OperationProvider = (*Provider)(nil)

func NewProvider(adapters ports.OperationProvider, defaultBackend, defaultChannel string) *Provider {
	return &Provider{
		adapters:       adapters,
		defaultBackend: defaultBackend,
		defaultChannel: defaultChannel,
	}
}

func (p *Provider) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	for _, schema := range operationSchemas(p.defaultChannel) {
		if schema.Name == name {
			return schema, true
		}
	}
	return nil, false
}

func (p *Provider) ListOperations() []*pluginmodel.OperationSchema {
	return operationSchemas(p.defaultChannel)
}

func (p *Provider) Execute(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
	request, err := normalizeRequest(name, inputs, settings{backend: p.defaultBackend, channel: p.defaultChannel})
	if err != nil {
		return nil, err
	}
	qualifiedOperation := request.backend + "." + request.operation
	if isNilProvider(p.adapters) {
		return nil, unavailableOperation(qualifiedOperation, nil)
	}
	operation, found := p.adapters.GetOperation(qualifiedOperation)
	if !found || operation == nil {
		return nil, unavailableOperation(qualifiedOperation, discoveredBackends(p.adapters.ListOperations()))
	}

	result, err := p.adapters.Execute(ctx, qualifiedOperation, request.inputs)
	if err != nil {
		return nil, fmt.Errorf("notifier %s via %s: %w", name, qualifiedOperation, err)
	}
	if result == nil {
		return nil, fmt.Errorf("%w: %q (adapter returned no result)", ErrAdapterUnavailable, qualifiedOperation)
	}
	return result, nil
}

func isNilProvider(provider ports.OperationProvider) bool {
	if provider == nil {
		return true
	}
	value := reflect.ValueOf(provider)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func discoveredBackends(operations []*pluginmodel.OperationSchema) []string {
	backends := make(map[string]struct{})
	for _, operation := range operations {
		if operation == nil {
			continue
		}
		prefix, _, found := strings.Cut(operation.Name, ".")
		if found && prefix != "" {
			backends[prefix] = struct{}{}
		}
	}
	result := make([]string, 0, len(backends))
	for backend := range backends {
		result = append(result, backend)
	}
	sort.Strings(result)
	return result
}

func unavailableOperation(operation string, backends []string) error {
	return fmt.Errorf("%w: %q (discovered backends: [%s])", ErrAdapterUnavailable, operation, strings.Join(backends, " "))
}
