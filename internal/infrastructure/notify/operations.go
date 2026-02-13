package notify

import "github.com/vanoix/awf/internal/domain/plugin"

// AllOperations returns all notification operation schemas that can be registered
// with the OperationRegistry. Currently defines a single operation: notify.send.
//
// The notify.send operation supports four backends (desktop, ntfy, slack, webhook)
// with backend-specific inputs (topic for ntfy, webhook_url for webhook).
func AllOperations() []plugin.OperationSchema {
	return []plugin.OperationSchema{
		// notify.send - Send notification via configured backend
		{
			Name:        "notify.send",
			Description: "Send a notification via configured backend (desktop, ntfy, slack, webhook)",
			Inputs: map[string]plugin.InputSchema{
				"backend": {
					Type:        plugin.InputTypeString,
					Required:    true,
					Description: "Notification backend: desktop, ntfy, slack, webhook",
				},
				"message": {
					Type:        plugin.InputTypeString,
					Required:    true,
					Description: "Notification message body",
				},
				"title": {
					Type:        plugin.InputTypeString,
					Required:    false,
					Description: "Notification title (defaults to 'AWF Workflow')",
				},
				"priority": {
					Type:        plugin.InputTypeString,
					Required:    false,
					Description: "Priority: low, default, high (defaults to 'default')",
				},
				"topic": {
					Type:        plugin.InputTypeString,
					Required:    false,
					Description: "ntfy topic name (required for ntfy backend)",
				},
				"webhook_url": {
					Type:        plugin.InputTypeString,
					Required:    false,
					Description: "Webhook URL (required for webhook backend)",
				},
				"channel": {
					Type:        plugin.InputTypeString,
					Required:    false,
					Description: "Slack channel override",
				},
			},
			Outputs: []string{"backend", "status", "response"},
		},
	}
}
