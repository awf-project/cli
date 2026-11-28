package tracker

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
)

var (
	ErrBackendRequired        = errors.New("tracker backend is required")
	ErrAdapterUnavailable     = errors.New("tracker adapter unavailable")
	ErrMalformedAdapterResult = errors.New("malformed tracker adapter result")
)

// Provider exposes the canonical tracker operations and delegates them to an
// externally supplied tracker adapter.
type Provider struct {
	external       ports.OperationProvider
	defaultBackend string
	logger         ports.Logger
}

var _ ports.OperationProvider = (*Provider)(nil)

// NewProvider constructs a canonical tracker operation provider.
func NewProvider(external ports.OperationProvider, defaultBackend string, logger ports.Logger) *Provider {
	if logger == nil {
		logger = ports.NopLogger{}
	}
	return &Provider{external: external, defaultBackend: defaultBackend, logger: logger}
}

func (p *Provider) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	for _, schema := range OperationSchemas() {
		if schema.Name == name {
			return schema, true
		}
	}
	return nil, false
}

func (p *Provider) ListOperations() []*pluginmodel.OperationSchema { return OperationSchemas() }

func (p *Provider) Execute(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
	schema, ok := p.GetOperation(name)
	if !ok {
		return nil, fmt.Errorf("operation: %w", ErrInvalidTrackerInput)
	}
	if err := validateInputs(schema, name, inputs); err != nil {
		return nil, err
	}

	backend, err := p.selectBackend(inputs)
	if err != nil {
		return nil, err
	}
	target, err := AdapterOperationName(backend, name)
	if err != nil {
		return nil, err
	}
	if p.external == nil {
		return nil, p.unavailableError(backend, target)
	}
	if _, found := p.external.GetOperation(target); !found {
		return nil, p.unavailableError(backend, target)
	}

	payload := make(map[string]any, len(inputs))
	for key, value := range inputs {
		if key != "backend" {
			payload[key] = value
		}
	}
	p.logger.Debug("delegating tracker operation", "operation", name, "backend", backend)
	result, err := p.external.Execute(ctx, target, payload)
	if err != nil {
		return nil, &trackerAdapterExecutionError{cause: err}
	}
	return canonicalResult(name, result)
}

type trackerAdapterExecutionError struct {
	cause error
}

func (*trackerAdapterExecutionError) Error() string {
	return "tracker adapter execution failed"
}

func (e *trackerAdapterExecutionError) Unwrap() error {
	return e.cause
}

func (p *Provider) selectBackend(inputs map[string]any) (string, error) {
	backend := p.defaultBackend
	if raw, exists := inputs["backend"]; exists {
		value, ok := raw.(string)
		if !ok {
			return "", invalidField("backend")
		}
		if value != "" {
			backend = value
		}
	}
	if backend == "" {
		return "", fmt.Errorf("backend: %w", ErrBackendRequired)
	}
	if _, err := AdapterOperationName(backend, createIssueOperation); err != nil {
		return "", err
	}
	return backend, nil
}

func (p *Provider) unavailableError(backend, target string) error {
	candidates := make([]string, 0)
	if p.external != nil {
		seen := make(map[string]struct{})
		for _, schema := range p.external.ListOperations() {
			if schema == nil || !strings.HasPrefix(schema.Name, "tracker-") {
				continue
			}
			prefix, _, found := strings.Cut(schema.Name, ".")
			if !found {
				continue
			}
			candidate := strings.TrimPrefix(prefix, "tracker-")
			seen[candidate] = struct{}{}
		}
		for candidate := range seen {
			candidates = append(candidates, candidate)
		}
		sort.Strings(candidates)
	}
	return fmt.Errorf("backend %s operation %s unavailable (candidates: %s): %w",
		backend, strings.TrimPrefix(target, "tracker-"+backend+"."), strings.Join(candidates, ", "), ErrAdapterUnavailable)
}
