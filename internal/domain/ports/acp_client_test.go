package ports_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noopACPClient is a minimal no-op implementation of ACPClient for testing.
type noopACPClient struct {
	responseOptionID string
}

func (n *noopACPClient) RequestPermission(ctx context.Context, req ports.PermissionRequest) (ports.PermissionResponse, error) {
	return ports.PermissionResponse{
		OptionID: n.responseOptionID,
	}, nil
}

var _ ports.ACPClient = (*noopACPClient)(nil)

func TestACPClient_NoopRoundTrip(t *testing.T) {
	tests := []struct {
		name             string
		request          ports.PermissionRequest
		responseOptionID string
		expectedResponse ports.PermissionResponse
	}{
		{
			name: "approval granted returns allow option ID",
			request: ports.PermissionRequest{
				SessionID:  "session-001",
				ToolCallID: "call-001",
				Prompt:     "Allow access to filesystem?",
				Options: []ports.PermissionOption{
					{
						ID:    "allow",
						Label: "Allow",
						Kind:  "allow",
					},
					{
						ID:    "deny",
						Label: "Deny",
						Kind:  "deny",
					},
				},
			},
			responseOptionID: "allow",
			expectedResponse: ports.PermissionResponse{
				OptionID: "allow",
			},
		},
		{
			name: "approval denied returns deny option ID",
			request: ports.PermissionRequest{
				SessionID:  "session-002",
				ToolCallID: "call-002",
				Prompt:     "Allow network access?",
				Options: []ports.PermissionOption{
					{
						ID:    "allow",
						Label: "Allow",
						Kind:  "allow",
					},
					{
						ID:    "deny",
						Label: "Deny",
						Kind:  "deny",
					},
				},
			},
			responseOptionID: "deny",
			expectedResponse: ports.PermissionResponse{
				OptionID: "deny",
			},
		},
		{
			name: "cancelled prompt returns empty option ID",
			request: ports.PermissionRequest{
				SessionID:  "session-003",
				ToolCallID: "call-003",
				Prompt:     "Continue?",
				Options: []ports.PermissionOption{
					{
						ID:    "yes",
						Label: "Yes",
						Kind:  "allow",
					},
					{
						ID:    "no",
						Label: "No",
						Kind:  "deny",
					},
				},
			},
			responseOptionID: "",
			expectedResponse: ports.PermissionResponse{
				OptionID: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &noopACPClient{
				responseOptionID: tt.responseOptionID,
			}

			response, err := client.RequestPermission(context.Background(), tt.request)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedResponse.OptionID, response.OptionID)
		})
	}
}
