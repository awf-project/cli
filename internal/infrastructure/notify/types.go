package notify

import "context"

// NotificationPayload contains the data sent to a notification backend.
// It is an immutable value object constructed by the provider and passed
// to backend implementations.
type NotificationPayload struct {
	// Title is the notification title (optional, defaults to "AWF Workflow")
	Title string

	// Message is the notification body (required)
	Message string

	// Priority indicates the notification urgency: "low", "default", or "high"
	// (optional, defaults to "default")
	Priority string

	// Metadata contains additional context like workflow name, status, duration
	Metadata map[string]string
}

// BackendResult contains the outcome of a notification delivery.
// It is returned by backend implementations after successful or failed sends.
type BackendResult struct {
	// Backend identifies which backend handled the notification
	Backend string

	// StatusCode is the HTTP status code for network backends (0 for desktop)
	StatusCode int

	// Response is the response body or confirmation message
	Response string
}

// Backend defines how a notification is delivered.
// Each backend implementation (desktop, webhook) implements
// this interface with backend-specific delivery logic.
type Backend interface {
	// Send delivers a notification using this backend.
	// Returns BackendResult on success or error on failure.
	// The context is used for cancellation and timeout control.
	Send(ctx context.Context, payload NotificationPayload) (*BackendResult, error)
}

// NotifyConfig holds notification plugin configuration from .awf/config.yaml.
// These values are loaded at provider initialization and used by backends
// to resolve configuration like URLs and default backend selection.
type NotifyConfig struct {
	// DefaultBackend is the backend to use when not specified in operation inputs
	// (valid values: "desktop", "webhook")
	DefaultBackend string
}
