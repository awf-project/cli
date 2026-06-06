package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"

	sdk "github.com/coder/acp-go-sdk"

	"github.com/awf-project/cli/internal/application"
)

var _ application.SessionUpdateEmitter = (*Emitter)(nil)

// sessionUpdater is the subset of *sdk.AgentSideConnection consumed by Emitter.
// Declaring it as an interface keeps the adapter unit-testable with a fake connection,
// so the marshal/unmarshal/emit path is exercised without a live SDK transport;
// production wires the concrete SDK connection.
type sessionUpdater interface {
	SessionUpdate(ctx context.Context, notif sdk.SessionNotification) error
}

type Emitter struct {
	conn   sessionUpdater
	logger *slog.Logger
}

// NewEmitter binds the adapter to a live SDK connection. A nil connection is normalised
// to "no transport" so EmitSessionUpdate degrades to a no-op instead of dereferencing a
// typed-nil pointer through the interface field.
func NewEmitter(conn *sdk.AgentSideConnection, logger *slog.Logger) *Emitter {
	if conn == nil {
		return &Emitter{logger: logger}
	}
	return &Emitter{conn: conn, logger: logger}
}

func (e *Emitter) EmitSessionUpdate(ctx context.Context, sessionID, kind string, fields map[string]any) error {
	if kind == "" {
		e.logger.Warn("acp emitter: empty update kind dropped")
		return nil
	}
	if e.conn == nil {
		return nil
	}

	// Build the SessionUpdate JSON by merging the kind discriminator into fields.
	// The SDK's SessionUpdate uses a custom UnmarshalJSON that dispatches on the
	// "sessionUpdate" discriminator field to construct the correct variant.
	// When the caller supplies no fields (e.g. workflow_started), skip the copy and
	// emit just the discriminator to avoid an allocation on these frequent events.
	var updateFields map[string]any
	if len(fields) == 0 {
		updateFields = map[string]any{"sessionUpdate": kind}
	} else {
		updateFields = make(map[string]any, len(fields)+1)
		maps.Copy(updateFields, fields)
		updateFields["sessionUpdate"] = kind
	}

	updateJSON, err := json.Marshal(updateFields)
	if err != nil {
		return fmt.Errorf("acp emitter: marshal update: %w", err)
	}

	var update sdk.SessionUpdate
	if err := json.Unmarshal(updateJSON, &update); err != nil {
		// Unknown or malformed kind — log and skip rather than returning an error
		// that would abort the caller's workflow run.
		e.logger.Warn("acp emitter: skipping unrecognized update kind", "kind", kind, "error", err)
		return nil
	}

	if err := e.conn.SessionUpdate(ctx, sdk.SessionNotification{
		SessionId: sdk.SessionId(sessionID),
		Update:    update,
	}); err != nil {
		return fmt.Errorf("acp emitter: session update: %w", err)
	}
	return nil
}
