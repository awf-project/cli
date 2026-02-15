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

	"github.com/awf-project/awf/pkg/httputil"
)

var slackBackendCounter uint64

// slackBackend sends notifications to Slack channels via incoming webhooks.
// It sends HTTP POST requests with JSON payloads formatted as Slack message blocks.
// Configuration is provided via the plugin's config section (slack_webhook_url).
type slackBackend struct {
	// webhookURL is the Slack incoming webhook URL
	webhookURL string

	// client handles HTTP POST requests with timeout and context support
	client *httputil.Client

	// id uniquely identifies this backend instance for testing purposes.
	// Without this field, Go would optimize empty structs to share the same memory location.
	id uint64
}

// newSlackBackend creates a new Slack notification backend.
// The webhookURL should be the full Slack incoming webhook URL.
// Returns an error if webhookURL is empty (missing configuration).
func newSlackBackend(webhookURL string) (*slackBackend, error) {
	// Validate webhookURL is not empty or whitespace-only
	trimmed := strings.TrimSpace(webhookURL)
	if trimmed == "" {
		return nil, errors.New("slack_webhook_url is required but not configured")
	}

	return &slackBackend{
		webhookURL: trimmed,
		client:     httputil.NewClient(httputil.WithTimeout(10 * time.Second)),
		id:         atomic.AddUint64(&slackBackendCounter, 1),
	}, nil
}

// NewSlackBackend creates a new Slack notification backend (exported).
// The webhookURL should be the full Slack incoming webhook URL.
// Returns an error if webhookURL is empty (missing configuration).
// This is the public API used for CLI wiring in run.go.
func NewSlackBackend(webhookURL string) (Backend, error) {
	return newSlackBackend(webhookURL)
}

// Send delivers a notification to Slack via incoming webhook.
// Returns BackendResult with the HTTP response or error on failure.
//
// In test mode (AWF_TEST_MODE=1), returns success without making HTTP requests.
// This allows testing registration logic without network dependencies.
func (s *slackBackend) Send(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
	// Test mode: succeed without making HTTP requests
	// This allows testing backend registration without network dependencies
	if os.Getenv("AWF_TEST_MODE") == "1" {
		return &BackendResult{
			Backend:    "slack",
			StatusCode: 200,
			Response:   "test mode: HTTP request not sent",
		}, nil
	}

	// Set default title if empty
	title := payload.Title
	if title == "" {
		title = "AWF Workflow"
	}

	// Build Slack message blocks
	blocks := buildSlackBlocks(title, payload.Message, payload.Priority, payload.Metadata)

	// Marshal to JSON
	body, err := json.Marshal(blocks)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	// Send HTTP POST request
	// Set Content-Type header for JSON
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Use unlimited body size (0) for Slack responses (typically small)
	resp, err := s.client.Post(ctx, s.webhookURL, headers, string(body), 0)
	if err != nil {
		// Network errors (unreachable, timeout, context cancellation)
		return nil, fmt.Errorf("failed to send Slack notification: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &BackendResult{
			Backend:    "slack",
			StatusCode: resp.StatusCode,
			Response:   resp.Body,
		}, errors.New("slack webhook returned non-2xx status code")
	}

	// Success
	return &BackendResult{
		Backend:    "slack",
		StatusCode: resp.StatusCode,
		Response:   resp.Body,
	}, nil
}

// buildSlackBlocks constructs a Slack Block Kit message structure.
// Returns a map with "blocks" array containing header and context sections.
func buildSlackBlocks(title, message, priority string, metadata map[string]string) map[string]interface{} {
	blocks := []map[string]interface{}{
		// Header block with title and priority emoji
		{
			"type": "header",
			"text": map[string]interface{}{
				"type": "plain_text",
				"text": formatTitleWithPriority(title, priority),
			},
		},
	}

	// Message section
	if message != "" {
		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": message,
			},
		})
	}

	// Metadata context block
	if len(metadata) > 0 {
		elements := make([]map[string]interface{}, 0, len(metadata))
		for key, value := range metadata {
			elements = append(elements, map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s:* %s", key, value),
			})
		}
		blocks = append(blocks, map[string]interface{}{
			"type":     "context",
			"elements": elements,
		})
	}

	return map[string]interface{}{
		"blocks": blocks,
	}
}

// formatTitleWithPriority adds emoji prefix based on priority level.
func formatTitleWithPriority(title, priority string) string {
	switch priority {
	case "high":
		return "🔴 " + title
	case "low":
		return "🟢 " + title
	default: // "default" or empty
		return "🔵 " + title
	}
}
