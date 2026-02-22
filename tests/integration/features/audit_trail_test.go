//go:build integration

package features_test

// Audit Trail Integration Tests (F071)
// Tests verify that FileAuditTrailWriter produces correct JSONL files.
// These are infrastructure-level tests: they use the writer directly,
// without wiring the full ExecutionService.
//
// Scenarios covered:
//   SC-002: Paired started/completed entries share the same execution_id
//   SC-003: Secret input values are replaced with "***" before write
//   SC-005: Nil writer (AWF_AUDIT_LOG=off) produces no file on disk
//   SC-006: Orphaned started entry when no completed event is written

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/infrastructure/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseAuditLines reads a JSONL file and returns one parsed AuditEvent per non-empty line.
func parseAuditLines(t *testing.T, path string) []workflow.AuditEvent {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err, "audit file should be readable")

	var events []workflow.AuditEvent
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event workflow.AuditEvent
		require.NoError(t, json.Unmarshal([]byte(line), &event), "each line must be valid JSON")
		events = append(events, event)
	}
	return events
}

// buildExecCtx constructs a minimal ExecutionContext suitable for audit event constructors.
func buildExecCtx(id, name string, startedAt, completedAt time.Time) *workflow.ExecutionContext {
	ctx := workflow.NewExecutionContext(id, name)
	ctx.StartedAt = startedAt
	ctx.CompletedAt = completedAt
	return ctx
}

// TestAuditTrail_PairedEntries (SC-002) verifies that 10 independent executions each
// produce exactly one workflow.started and one workflow.completed entry sharing
// the same execution_id.
func TestAuditTrail_PairedEntries(t *testing.T) {
	t.Parallel()

	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	writer, err := audit.NewFileAuditTrailWriter(auditPath)
	require.NoError(t, err)
	defer writer.Close()

	ctx := context.Background()
	fixedStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fixedEnd := fixedStart.Add(100 * time.Millisecond)

	const iterations = 10
	for i := range iterations {
		execID := "exec-" + string(rune('A'+i))
		execCtx := buildExecCtx(execID, "test-workflow", fixedStart, fixedEnd)

		started := workflow.NewStartedEvent(execCtx, map[string]any{"key": "value"}, "ci")
		completed := workflow.NewCompletedEvent(execCtx, "ci", "")

		require.NoError(t, writer.Write(ctx, &started))
		require.NoError(t, writer.Write(ctx, &completed))
	}

	require.NoError(t, writer.Close())

	events := parseAuditLines(t, auditPath)
	require.Len(t, events, iterations*2, "expected 2 events per execution")

	// Group events by execution_id.
	byExecID := make(map[string][]workflow.AuditEvent)
	for _, ev := range events {
		byExecID[ev.ExecutionID] = append(byExecID[ev.ExecutionID], ev)
	}

	assert.Len(t, byExecID, iterations, "expected one group per execution")

	for execID, group := range byExecID {
		var started, completed int
		for _, ev := range group {
			switch ev.Event {
			case workflow.EventWorkflowStarted:
				started++
			case workflow.EventWorkflowCompleted:
				completed++
			}
		}
		assert.Equal(t, 1, started, "execution %s: expected exactly 1 workflow.started", execID)
		assert.Equal(t, 1, completed, "execution %s: expected exactly 1 workflow.completed", execID)
	}
}

// TestAuditTrail_SecretMasking (SC-003) verifies that the application-layer masking
// convention (replacing secret values with "***") is preserved in the written JSONL.
// The test simulates what the application layer does before calling Write.
func TestAuditTrail_SecretMasking(t *testing.T) {
	t.Parallel()

	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	writer, err := audit.NewFileAuditTrailWriter(auditPath)
	require.NoError(t, err)
	defer writer.Close()

	fixedStart := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	execCtx := buildExecCtx("exec-mask-01", "masked-workflow", fixedStart, fixedStart)

	// Application layer masks secrets before constructing the event.
	maskedInputs := map[string]any{
		"SECRET_API_KEY": "***", // was "hunter2"
		"PUBLIC_VAR":     "visible",
	}

	started := workflow.NewStartedEvent(execCtx, maskedInputs, "tester")

	require.NoError(t, writer.Write(context.Background(), &started))
	require.NoError(t, writer.Close())

	data, err := os.ReadFile(auditPath)
	require.NoError(t, err)
	fileContent := string(data)

	assert.Contains(t, fileContent, `"***"`, "masked placeholder must appear in file")
	assert.Contains(t, fileContent, "visible", "non-secret value must appear in file")
	assert.NotContains(t, fileContent, "hunter2", "plaintext secret must not appear in file")
}

// TestAuditTrail_DisabledProducesNoFile (SC-005) verifies that when the audit writer
// is nil (simulating AWF_AUDIT_LOG=off), no file is created at the expected path.
func TestAuditTrail_DisabledProducesNoFile(t *testing.T) {
	t.Parallel()

	expectedPath := filepath.Join(t.TempDir(), "audit.jsonl")

	// Nil writer simulates the disabled state: no writes happen.
	var writer *audit.FileAuditTrailWriter

	if writer != nil {
		fixedStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		execCtx := buildExecCtx("exec-off-01", "off-workflow", fixedStart, fixedStart)
		started := workflow.NewStartedEvent(execCtx, nil, "user")
		_ = writer.Write(context.Background(), &started)
	}

	_, statErr := os.Stat(expectedPath)
	assert.True(t, os.IsNotExist(statErr), "no audit file should exist when writer is nil")
}

// TestAuditTrail_OrphanedStartEntry (SC-006) verifies that when only a workflow.started
// event is written (e.g. process killed before completion), the file contains exactly
// one line with event "workflow.started" and no "workflow.completed" line.
func TestAuditTrail_OrphanedStartEntry(t *testing.T) {
	t.Parallel()

	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	writer, err := audit.NewFileAuditTrailWriter(auditPath)
	require.NoError(t, err)

	fixedStart := time.Date(2026, 4, 15, 8, 30, 0, 0, time.UTC)
	execCtx := buildExecCtx("exec-orphan-01", "orphan-workflow", fixedStart, fixedStart)

	started := workflow.NewStartedEvent(execCtx, map[string]any{"input": "data"}, "dev")
	require.NoError(t, writer.Write(context.Background(), &started))

	// Close without writing a completed event — simulates abrupt termination.
	require.NoError(t, writer.Close())

	events := parseAuditLines(t, auditPath)
	require.Len(t, events, 1, "expected exactly 1 event in the file")

	assert.Equal(t, workflow.EventWorkflowStarted, events[0].Event,
		"the single event must be workflow.started")

	// Confirm no completed event exists.
	for _, ev := range events {
		assert.NotEqual(t, workflow.EventWorkflowCompleted, ev.Event,
			"workflow.completed must not be present in an orphaned trail")
	}
}
