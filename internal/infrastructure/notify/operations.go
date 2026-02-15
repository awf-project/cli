package notify

import "github.com/awf-project/awf/internal/domain/plugin"

func AllOperations() []plugin.OperationSchema {
	return []plugin.OperationSchema{
		{
			Name:        "notify.send",
			Description: "Send a notification via configured backend (desktop, webhook)",
			Inputs: map[string]plugin.InputSchema{
				"backend": {
					Type:        plugin.InputTypeString,
					Required:    true,
					Description: "Notification backend: desktop, webhook",
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
				"webhook_url": {
					Type:        plugin.InputTypeString,
					Required:    false,
					Description: "Webhook URL (required for webhook backend)",
				},
			},
			Outputs: []string{"backend", "status", "response"},
		},
	}
}
