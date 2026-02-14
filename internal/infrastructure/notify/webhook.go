package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vanoix/awf/pkg/httputil"
)

var webhookBackendCounter uint64

// webhookBackend sends notifications to arbitrary webhook URLs via HTTP POST.
// It sends HTTP POST requests with JSON payloads containing workflow completion details.
// The webhook URL is provided dynamically via the operation input (webhook_url).
type webhookBackend struct {
	// client handles HTTP POST requests with timeout and context support
	client *httputil.Client

	// id uniquely identifies this backend instance for testing purposes.
	// Without this field, Go would optimize empty structs to share the same memory location.
	id uint64
}

// newWebhookBackend creates a new webhook notification backend.
// Unlike ntfy and slack, webhook URLs are provided per-request via metadata.
func newWebhookBackend() *webhookBackend {
	return &webhookBackend{
		client: httputil.NewClient(httputil.WithTimeout(10 * time.Second)),
		id:     atomic.AddUint64(&webhookBackendCounter, 1),
	}
}

// NewWebhookBackend creates a new webhook notification backend (exported).
// Unlike ntfy and slack, webhook URLs are provided per-request via metadata.
// This is the public API used for CLI wiring in run.go.
func NewWebhookBackend() Backend {
	return newWebhookBackend()
}

// Send delivers a notification to an arbitrary webhook URL.
// The webhook URL must be provided in the payload's Metadata["webhook_url"].
// Returns BackendResult with the HTTP response or error on failure.
//
// In test mode (AWF_TEST_MODE=1), returns success without making HTTP requests.
// This allows testing registration logic without network dependencies.
func (w *webhookBackend) Send(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
	// Extract webhook_url from metadata
	webhookURL, ok := payload.Metadata["webhook_url"]
	if !ok {
		return nil, errors.New("webhook_url is required in metadata for webhook backend")
	}

	// Validate webhook_url is not empty or whitespace-only
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return nil, errors.New("webhook_url cannot be empty for webhook backend")
	}

	// Build webhook JSON payload
	body, err := buildWebhookPayload(payload)
	if err != nil {
		return nil, err
	}

	// Test mode: succeed without making HTTP requests
	// This allows testing backend registration without network dependencies
	// Note: Still validates inputs and payload construction above
	if os.Getenv("AWF_TEST_MODE") == "1" {
		return &BackendResult{
			Backend:    "webhook",
			StatusCode: 200,
			Response:   "test mode: HTTP request not sent",
		}, nil
	}

	// Send HTTP POST request
	// Set Content-Type header for JSON
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Use unlimited body size (0) for webhook responses (typically small)
	resp, err := w.client.Post(ctx, webhookURL, headers, string(body), 0)
	if err != nil {
		// Network errors (unreachable, timeout, context cancellation)
		return nil, fmt.Errorf("failed to send webhook notification: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &BackendResult{
			Backend:    "webhook",
			StatusCode: resp.StatusCode,
			Response:   resp.Body,
		}, errors.New("webhook returned non-2xx status code")
	}

	// Success
	return &BackendResult{
		Backend:    "webhook",
		StatusCode: resp.StatusCode,
		Response:   resp.Body,
	}, nil
}

// buildWebhookPayload constructs a JSON payload for webhook POST requests.
// Returns marshaled JSON bytes containing title, message, priority, and metadata fields.
func buildWebhookPayload(payload NotificationPayload) ([]byte, error) {
	// Build webhook JSON structure
	webhookData := map[string]interface{}{
		"title":    payload.Title,
		"message":  payload.Message,
		"priority": payload.Priority,
	}

	// Add metadata fields (excluding webhook_url which is internal)
	metadata := make(map[string]string)
	for key, value := range payload.Metadata {
		if key != "webhook_url" {
			metadata[key] = value
		}
	}
	if len(metadata) > 0 {
		webhookData["metadata"] = metadata
	}

	// Marshal to JSON
	body, err := json.Marshal(webhookData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	return body, nil
}
