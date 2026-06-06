package acp

import (
	"context"
	"fmt"
	"log/slog"

	sdk "github.com/coder/acp-go-sdk"

	"github.com/awf-project/cli/internal/domain/ports"
)

var _ ports.ACPClient = (*PermissionClient)(nil)

// permissionRequester is the subset of *sdk.AgentSideConnection consumed by
// PermissionClient. Declaring it as an interface keeps the adapter unit-testable
// with a fake connection; production wires the concrete SDK connection.
type permissionRequester interface {
	RequestPermission(ctx context.Context, params sdk.RequestPermissionRequest) (sdk.RequestPermissionResponse, error)
}

// PermissionClient implements ports.ACPClient by binding RequestPermission to the
// SDK connection's outbound session/request_permission call (FR-003). It is the
// transport adapter only: the call site that decides WHEN to request permission is
// delivered by F108 Axis B (spec US2 — F105 wires transport, not the gate logic).
type PermissionClient struct {
	conn   permissionRequester
	logger *slog.Logger
}

// NewPermissionClient binds the adapter to a live SDK connection. A nil connection
// is normalised to "no transport" so the adapter degrades gracefully instead of
// dereferencing a typed-nil pointer through the interface field.
func NewPermissionClient(conn *sdk.AgentSideConnection, logger *slog.Logger) *PermissionClient {
	if conn == nil {
		return &PermissionClient{logger: logger}
	}
	return &PermissionClient{conn: conn, logger: logger}
}

// RequestPermission maps the neutral ports.PermissionRequest onto the SDK request,
// issues the outbound call via conn.RequestPermission, and maps the SDK outcome back
// to a neutral ports.PermissionResponse (Selected → chosen option id; Cancelled or
// absent → empty OptionID).
//
// ctx cancellation is honored before the call and returned verbatim — never swallowed.
// No internal lock is taken: the SDK serializes outbound writes (SPIKE finding #8), so
// adding one here would only risk deadlock.
func (c *PermissionClient) RequestPermission(ctx context.Context, req ports.PermissionRequest) (ports.PermissionResponse, error) {
	if err := ctx.Err(); err != nil {
		//nolint:wrapcheck // ctx.Err() (context.Canceled/DeadlineExceeded) is returned verbatim, never wrapped or swallowed
		return ports.PermissionResponse{}, err
	}
	if c.conn == nil {
		// No transport wired (constructed with a nil connection). Nothing to call.
		return ports.PermissionResponse{}, nil
	}

	options := make([]sdk.PermissionOption, len(req.Options))
	for i := range req.Options {
		options[i] = sdk.PermissionOption{
			OptionId: sdk.PermissionOptionId(req.Options[i].ID),
			Name:     req.Options[i].Label,
			Kind:     sdk.PermissionOptionKind(req.Options[i].Kind),
		}
	}

	toolCall := sdk.ToolCallUpdate{ToolCallId: sdk.ToolCallId(req.ToolCallID)}
	if req.Prompt != "" {
		// The SDK has no top-level prompt field; the human-readable prompt is carried
		// as the tool call's title (the editor renders it alongside the options).
		prompt := req.Prompt
		toolCall.Title = &prompt
	}

	resp, err := c.conn.RequestPermission(ctx, sdk.RequestPermissionRequest{
		SessionId: sdk.SessionId(req.SessionID),
		ToolCall:  toolCall,
		Options:   options,
	})
	if err != nil {
		return ports.PermissionResponse{}, fmt.Errorf("acp permission request (session %s): %w", req.SessionID, err)
	}

	if resp.Outcome.Selected != nil {
		return ports.PermissionResponse{OptionID: string(resp.Outcome.Selected.OptionId)}, nil
	}
	return ports.PermissionResponse{}, nil
}
