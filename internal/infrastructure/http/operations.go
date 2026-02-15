package http

import "github.com/awf-project/awf/internal/domain/plugin"

// AllOperations returns all HTTP operation schemas.
func AllOperations() []plugin.OperationSchema {
	return []plugin.OperationSchema{
		// http.request - Perform HTTP request
		{
			Name:        "http.request",
			Description: "Perform HTTP request (GET, POST, PUT, DELETE) with configurable timeout and headers",
			Inputs: map[string]plugin.InputSchema{
				"url": {
					Type:        plugin.InputTypeString,
					Required:    true,
					Description: "Target URL (must start with http:// or https://)",
					Validation:  "url",
				},
				"method": {
					Type:        plugin.InputTypeString,
					Required:    true,
					Description: "HTTP method: GET, POST, PUT, DELETE (case-insensitive)",
				},
				"headers": {
					Type:        plugin.InputTypeObject,
					Required:    false,
					Description: "HTTP headers (key-value pairs)",
				},
				"body": {
					Type:        plugin.InputTypeString,
					Required:    false,
					Description: "Request body (raw string, JSON encoding is caller's responsibility)",
				},
				"timeout": {
					Type:        plugin.InputTypeInteger,
					Required:    false,
					Description: "Request timeout in seconds (default: 30)",
					Default:     30,
				},
				"retryable_status_codes": {
					Type:        plugin.InputTypeArray,
					Required:    false,
					Description: "HTTP status codes that should be treated as retryable failures (e.g., [429, 502, 503])",
				},
			},
			Outputs: []string{"status_code", "body", "headers", "body_truncated"},
		},
	}
}
