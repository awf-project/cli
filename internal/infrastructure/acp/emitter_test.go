package acp_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/infrastructure/acp"
)

func TestEmitter_EmitSessionUpdate_ValidKind(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))

	emitter := acp.NewEmitter(nil, logger)

	ctx := context.Background()
	err := emitter.EmitSessionUpdate(ctx, "sess-123", "workflow_started", map[string]any{})

	require.NoError(t, err)
	// Stub returns nil; implementation will build SDK SessionUpdate and emit
	assert.NotContains(t, buf.String(), "unknown update kind dropped")
}

func TestEmitter_EmitSessionUpdate_EmptyKind(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))

	emitter := acp.NewEmitter(nil, logger)

	ctx := context.Background()
	err := emitter.EmitSessionUpdate(ctx, "sess-123", "", map[string]any{})

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "acp emitter: empty update kind dropped")
}

func TestEmitter_EmitSessionUpdate_WorkflowCompleted(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))

	emitter := acp.NewEmitter(nil, logger)

	ctx := context.Background()
	fields := map[string]any{
		"duration_ms": "5000",
	}
	err := emitter.EmitSessionUpdate(ctx, "sess-123", "workflow_completed", fields)

	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "unknown update kind dropped")
}

func TestEmitter_EmitSessionUpdate_StepKinds(t *testing.T) {
	stepKinds := []string{
		"step_started",
		"step_completed",
		"step_failed",
		"step_retrying",
	}

	for _, kind := range stepKinds {
		t.Run(kind, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := slog.New(slog.NewTextHandler(buf, nil))

			emitter := acp.NewEmitter(nil, logger)

			ctx := context.Background()
			fields := map[string]any{
				"step_name": "validate",
			}
			err := emitter.EmitSessionUpdate(ctx, "sess-123", kind, fields)

			require.NoError(t, err)
			assert.NotContains(t, buf.String(), "unknown update kind dropped")
		})
	}
}

func TestEmitter_EmitSessionUpdate_ContextPropagation(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))

	emitter := acp.NewEmitter(nil, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := emitter.EmitSessionUpdate(ctx, "sess-123", "workflow_started", map[string]any{})

	// Implementation will pass cancelled context to conn.SessionUpdate
	require.NoError(t, err)
}

func TestEmitter_OtherSessionUpdateKinds(t *testing.T) {
	otherKinds := []string{
		"available_commands_update",
		"agent_message_chunk",
	}

	for _, kind := range otherKinds {
		t.Run(kind, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := slog.New(slog.NewTextHandler(buf, nil))

			emitter := acp.NewEmitter(nil, logger)

			ctx := context.Background()
			err := emitter.EmitSessionUpdate(ctx, "sess-123", kind, map[string]any{})

			require.NoError(t, err)
		})
	}
}
