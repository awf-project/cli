package notify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
)

var ntfyBackendCounter uint64

// ntfyBackend sends notifications to ntfy.sh or self-hosted ntfy servers.
// It sends HTTP POST requests to the configured ntfy URL with the topic appended to the path.
// Configuration is provided via the plugin's config section (ntfy_url).
type ntfyBackend struct {
	// baseURL is the ntfy server URL (e.g., "https://ntfy.sh")
	baseURL string

	// sender handles HTTP POST requests with timeout and context support
	sender *httpSender

	// id uniquely identifies this backend instance for testing purposes.
	// Without this field, Go would optimize empty structs to share the same memory location.
	id uint64
}

// newNtfyBackend creates a new ntfy notification backend.
// The baseURL should be the ntfy server URL without trailing slash (e.g., "https://ntfy.sh").
// Returns an error if baseURL is empty (missing configuration).
func newNtfyBackend(baseURL string) (*ntfyBackend, error) {
	// Validate baseURL is not empty or whitespace-only
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return nil, errors.New("ntfy_url is required but not configured")
	}

	return &ntfyBackend{
		baseURL: trimmed,
		sender:  newHTTPSender(),
		id:      atomic.AddUint64(&ntfyBackendCounter, 1),
	}, nil
}

// NewNtfyBackend creates a new ntfy notification backend (exported).
// The baseURL should be the ntfy server URL without trailing slash (e.g., "https://ntfy.sh").
// Returns an error if baseURL is empty (missing configuration).
// This is the public API used for CLI wiring in run.go.
func NewNtfyBackend(baseURL string) (Backend, error) {
	return newNtfyBackend(baseURL)
}

// Send delivers a notification to the specified ntfy topic.
// The topic must be provided in the payload's Metadata["topic"].
// Returns BackendResult with the HTTP response or error on failure.
//
// In test mode (AWF_TEST_MODE=1), returns success without making HTTP requests.
// This allows testing registration logic without network dependencies.
func (n *ntfyBackend) Send(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
	// Extract topic from metadata
	topic, ok := payload.Metadata["topic"]
	if !ok {
		return nil, errors.New("topic is required in metadata for ntfy backend")
	}

	// Validate topic is not empty or whitespace-only
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil, errors.New("topic cannot be empty for ntfy backend")
	}

	// Test mode: succeed without making HTTP requests
	// This allows testing backend registration without network dependencies
	if os.Getenv("AWF_TEST_MODE") == "1" {
		return &BackendResult{
			Backend:    "ntfy",
			StatusCode: 200,
			Response:   "test mode: HTTP request not sent",
		}, nil
	}

	// Construct URL: baseURL + "/" + topic
	// Handle baseURL trailing slash to avoid double slashes
	url := strings.TrimRight(n.baseURL, "/") + "/" + topic

	// Set default title if empty
	title := payload.Title
	if title == "" {
		title = "AWF Workflow"
	}

	// Set default priority if empty
	priority := payload.Priority
	if priority == "" {
		priority = "default"
	}

	// Create request body (ntfy expects plain text message body)
	body := []byte(payload.Message)

	// Create HTTP request to set custom headers
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create ntfy request: %w", err)
	}

	// Set ntfy-specific headers
	req.Header.Set("Title", title)
	req.Header.Set("Priority", priority)

	// Execute request using sender's client
	resp, err := n.sender.client.Do(req)
	if err != nil {
		// Network errors (unreachable, timeout, etc.)
		return nil, fmt.Errorf("failed to send ntfy notification: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseData, readErr := io.ReadAll(resp.Body)
	responseStr := string(responseData)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return &BackendResult{
			Backend:    "ntfy",
			StatusCode: resp.StatusCode,
			Response:   "",
		}, fmt.Errorf("failed to read ntfy response: %w", readErr)
	}

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &BackendResult{
			Backend:    "ntfy",
			StatusCode: resp.StatusCode,
			Response:   responseStr,
		}, errors.New("ntfy server returned non-2xx status code")
	}

	// Success
	return &BackendResult{
		Backend:    "ntfy",
		StatusCode: resp.StatusCode,
		Response:   responseStr,
	}, nil
}
