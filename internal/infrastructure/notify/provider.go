package notify

import (
	"context"
	"fmt"
	"strings"

	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
)

// NotifyOperationProvider implements ports.OperationProvider for notification operations.
// Dispatches notify.send operation to backend-specific handlers (desktop, ntfy, slack, webhook).
//
// The provider manages:
//   - Operation schema registry (notify.send)
//   - Backend dispatch via Backend interface
//   - Dynamic backend registration via RegisterBackend
//   - Input validation and payload construction
type NotifyOperationProvider struct {
	logger         ports.Logger
	backends       map[string]Backend
	defaultBackend string

	// operations holds the registry of notification operation schemas
	operations map[string]*plugin.OperationSchema
}

// NewNotifyOperationProvider creates a new notification operation provider.
//
// The provider starts with an empty backend registry. Use RegisterBackend to add
// notification backends dynamically. This enables the open/closed principle: new
// backends can be added without modifying the provider implementation.
//
// Parameters:
//   - logger: structured logger for operation tracing
//
// Returns:
//   - *NotifyOperationProvider: configured provider ready for backend registration
func NewNotifyOperationProvider(logger ports.Logger) *NotifyOperationProvider {
	// Build operation registry from schema definitions
	ops := AllOperations()
	registry := make(map[string]*plugin.OperationSchema, len(ops))
	for i := range ops {
		registry[ops[i].Name] = &ops[i]
	}

	return &NotifyOperationProvider{
		logger:     logger,
		backends:   make(map[string]Backend),
		operations: registry,
	}
}

// RegisterBackend registers a notification backend with the provider.
//
// Backends are registered by name (e.g., "desktop", "ntfy", "slack", "webhook")
// and must implement the Backend interface. Registration is idempotent: duplicate
// registrations for the same name return an error.
//
// Parameters:
//   - name: backend identifier (must be unique)
//   - backend: implementation of the Backend interface
//
// Returns:
//   - error: non-nil if name is already registered or backend is nil
func (p *NotifyOperationProvider) RegisterBackend(name string, backend Backend) error {
	// Validate backend name
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("backend name cannot be empty or whitespace-only")
	}

	// Validate backend implementation
	if backend == nil {
		return fmt.Errorf("backend implementation cannot be nil")
	}

	// Check for duplicate registration
	if _, exists := p.backends[name]; exists {
		return fmt.Errorf("backend %q is already registered", name)
	}

	// Register the backend
	p.backends[name] = backend
	return nil
}

// SetDefaultBackend configures the fallback backend name.
//
// The default backend is used when Execute is called without an explicit
// 'backend' input parameter. If no default is set and no backend is specified
// in inputs, Execute returns an error.
//
// Parameters:
//   - name: backend identifier to use as default
func (p *NotifyOperationProvider) SetDefaultBackend(name string) {
	p.defaultBackend = name
}

// GetOperation returns an operation schema by name.
// Implements ports.OperationProvider.
func (p *NotifyOperationProvider) GetOperation(name string) (*plugin.OperationSchema, bool) {
	op, found := p.operations[name]
	return op, found
}

// ListOperations returns all available notification operations.
// Implements ports.OperationProvider.
func (p *NotifyOperationProvider) ListOperations() []*plugin.OperationSchema {
	result := make([]*plugin.OperationSchema, 0, len(p.operations))
	for _, op := range p.operations {
		result = append(result, op)
	}
	return result
}

// Execute runs a notification operation by name with the given inputs.
// Dispatches to backend-specific handlers based on the 'backend' input.
//
// Implements ports.OperationProvider.
func (p *NotifyOperationProvider) Execute(ctx context.Context, name string, inputs map[string]any) (*plugin.OperationResult, error) {
	if p.logger != nil {
		p.logger.Debug("executing notify operation", "operation", name, "inputs", inputs)
	}

	// Validate operation exists
	opSchema, found := p.operations[name]
	if !found {
		return &plugin.OperationResult{
			Success: false,
			Outputs: make(map[string]any),
		}, fmt.Errorf("notify: operation %q not found", name)
	}

	// Extract backend input (optional - can fall back to default)
	backend, err := extractStringInput(inputs, "backend", false)
	if err != nil {
		return &plugin.OperationResult{
			Success: false,
			Outputs: make(map[string]any),
		}, fmt.Errorf("notify.send: %w", err)
	}

	// Fall back to default backend if no backend specified
	if backend == "" {
		backend = p.defaultBackend
	}

	// Validate that a backend is specified (either explicit or default)
	if backend == "" {
		return &plugin.OperationResult{
			Success: false,
			Outputs: make(map[string]any),
		}, fmt.Errorf("notify.send: no backend specified and no default backend configured")
	}

	message, err := extractStringInput(inputs, "message", true)
	if err != nil {
		return &plugin.OperationResult{
			Success: false,
			Outputs: make(map[string]any),
		}, fmt.Errorf("notify.send: %w", err)
	}

	// Extract optional inputs
	title, _ := extractStringInput(inputs, "title", false) //nolint:errcheck // optional input, error ignored
	if title == "" {
		title = "AWF Workflow"
	}

	priority, _ := extractStringInput(inputs, "priority", false) //nolint:errcheck // optional input, error ignored
	if priority == "" {
		priority = "default"
	}

	// Validate priority value
	if priority != "low" && priority != "default" && priority != "high" {
		return &plugin.OperationResult{
			Success: false,
			Outputs: make(map[string]any),
		}, fmt.Errorf("notify.send: invalid priority %q (must be: low, default, high)", priority)
	}

	// Check if backend is available
	backendImpl, ok := p.backends[backend]
	if !ok {
		availableBackends := make([]string, 0, len(p.backends))
		for k := range p.backends {
			availableBackends = append(availableBackends, k)
		}
		return &plugin.OperationResult{
			Success: false,
			Outputs: make(map[string]any),
		}, fmt.Errorf("notify.send: backend %q not available (available: %v)", backend, availableBackends)
	}

	// Build metadata map for backend-specific inputs
	metadata := make(map[string]string)

	// Add backend-specific inputs to metadata
	if topic, _ := extractStringInput(inputs, "topic", false); topic != "" { //nolint:errcheck // optional input
		metadata["topic"] = topic
	}
	if webhookURL, _ := extractStringInput(inputs, "webhook_url", false); webhookURL != "" { //nolint:errcheck // optional input
		metadata["webhook_url"] = webhookURL
	}
	if channel, _ := extractStringInput(inputs, "channel", false); channel != "" { //nolint:errcheck // optional input
		metadata["channel"] = channel
	}

	// Construct notification payload
	payload := NotificationPayload{
		Title:    title,
		Message:  message,
		Priority: priority,
		Metadata: metadata,
	}

	// Validate required inputs for specific backends
	if validateErr := validateBackendInputs(backend, opSchema, inputs); validateErr != nil {
		return &plugin.OperationResult{
			Success: false,
			Outputs: make(map[string]any),
		}, fmt.Errorf("notify.send: %w", validateErr)
	}

	// Dispatch to backend
	if p.logger != nil {
		p.logger.Debug("dispatching to backend", "backend", backend, "title", title)
	}

	result, err := backendImpl.Send(ctx, payload)
	if err != nil {
		// Backend failed
		if p.logger != nil {
			p.logger.Error("backend send failed", "backend", backend, "error", err)
		}
		return &plugin.OperationResult{
			Success: false,
			Outputs: make(map[string]any),
		}, fmt.Errorf("notify.send: backend %q failed: %w", backend, err)
	}

	// Success - convert BackendResult to OperationResult
	if p.logger != nil {
		p.logger.Info("notification sent", "backend", result.Backend, "status", result.StatusCode)
	}

	return &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{
			"backend":  result.Backend,
			"status":   result.StatusCode,
			"response": result.Response,
		},
	}, nil
}

// extractStringInput safely extracts a string input from the inputs map.
// Returns the string value and error if required input is missing or invalid type.
func extractStringInput(inputs map[string]any, key string, required bool) (string, error) {
	value, ok := inputs[key]
	if !ok {
		if required {
			return "", fmt.Errorf("required input %q is missing", key)
		}
		return "", nil
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("input %q must be a string, got %T", key, value)
	}

	return strings.TrimSpace(strValue), nil
}

// validateBackendInputs checks that backend-specific required inputs are present.
// For example, ntfy requires 'topic', webhook requires 'webhook_url'.
//
// Note: This only validates known backends with specific input requirements.
// Unknown/custom backends are allowed and skip validation (open/closed principle).
func validateBackendInputs(backend string, _ *plugin.OperationSchema, inputs map[string]any) error {
	switch backend {
	case "ntfy":
		topic, err := extractStringInput(inputs, "topic", false)
		if err != nil {
			return err
		}
		if topic == "" {
			return fmt.Errorf("backend %q requires 'topic' input", backend)
		}
	case "webhook":
		webhookURL, err := extractStringInput(inputs, "webhook_url", false)
		if err != nil {
			return err
		}
		if webhookURL == "" {
			return fmt.Errorf("backend %q requires 'webhook_url' input", backend)
		}
	case "slack":
		// Slack uses config slack_webhook_url, no input validation needed
	case "desktop":
		// Desktop has no special input requirements
	default:
		// Custom/unknown backends are allowed - no validation
	}
	return nil
}
