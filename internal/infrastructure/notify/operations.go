package notify

import "github.com/awf-project/cli/internal/domain/pluginmodel"

func AllOperations() []pluginmodel.OperationSchema {
	return []pluginmodel.OperationSchema{
		{
			Name:        "notify.send",
			Description: "Send a notification via configured backend (desktop, webhook)",
			Inputs: map[string]pluginmodel.InputSchema{
				"backend": {
					Type:        pluginmodel.InputTypeString,
					Required:    true,
					Description: "Notification backend: desktop, webhook",
				},
				"message": {
					Type:        pluginmodel.InputTypeString,
					Required:    true,
					Description: "Notification message body",
				},
				"title": {
					Type:        pluginmodel.InputTypeString,
					Required:    false,
					Description: "Notification title (defaults to 'AWF Workflow')",
				},
				"priority": {
					Type:        pluginmodel.InputTypeString,
					Required:    false,
					Description: "Priority: low, default, high (defaults to 'default')",
				},
				"webhook_url": {
					Type:        pluginmodel.InputTypeString,
					Required:    false,
					Description: "Webhook URL (required for webhook backend)",
				},
			},
			Outputs: []string{"backend", "status", "response"},
		},
	}
}
