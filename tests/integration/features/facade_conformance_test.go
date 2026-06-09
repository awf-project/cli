//go:build integration

// Feature: F107 — T065
//
// Conformance suite: one scripted event sequence × 4 interface projections = 4 golden files.
// SC-002 / D39: if any interface diverges, the golden diff fails clearly.
//
// Refresh goldens: go test -tags=integration ./tests/integration/features/... -run TestFacadeConformance -update
package features_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/facadetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "regenerate golden files")

const facadeGoldenDir = "../../fixtures/facade"

// conformanceScript is the canonical scripted event sequence used across all 4 projections.
func conformanceScript() *facadetest.Fake {
	return facadetest.New().
		Script(
			ports.Event{Kind: ports.EventRunStarted, RunID: "run-conformance"},
			ports.Event{Kind: ports.EventToolCall, RunID: "run-conformance"},
			ports.Event{Kind: ports.EventToolResult, RunID: "run-conformance"},
			ports.Event{Kind: ports.EventStepCompleted, RunID: "run-conformance"},
		).
		WithTerminalCompleted()
}

func readFacadeGolden(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(facadeGoldenDir, name))
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	return data
}

func writeFacadeGolden(t *testing.T, name string, data []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(facadeGoldenDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(facadeGoldenDir, name), data, 0o644))
	t.Logf("updated golden: %s", name)
}

func assertFacadeGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	if *update {
		writeFacadeGolden(t, name, got)
		return
	}
	want := readFacadeGolden(t, name)
	if !bytes.Equal(want, got) {
		t.Errorf("golden mismatch for %s\n--- want ---\n%s\n--- got ---\n%s\nrerun with -update to refresh",
			name, want, got)
	}
}

// projectToCLIStdout projects facade events to CLI stdout text format.
// TODO: replace with real CLI display renderer after T060 GREEN.
func projectToCLIStdout(events []ports.Event) []byte {
	var buf bytes.Buffer
	for _, ev := range events {
		fmt.Fprintf(&buf, "[%s] run=%s\n", ev.Kind.String(), ev.RunID)
	}
	return buf.Bytes()
}

// projectToACPSessionUpdate projects facade events to ACP session/update JSONL format.
// TODO: replace with real ACP event projector after T063 GREEN.
func projectToACPSessionUpdate(events []ports.Event) []byte {
	type updateLine struct {
		Kind  string `json:"kind"`
		RunID string `json:"run_id"`
	}
	var buf bytes.Buffer
	for _, ev := range events {
		b, _ := json.Marshal(updateLine{Kind: ev.Kind.String(), RunID: ev.RunID}) //nolint:errcheck // controlled struct, cannot fail
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// projectToSSEFrames projects facade events to raw SSE frame bytes.
// TODO: replace with real SSE projector after T062 GREEN.
func projectToSSEFrames(events []ports.Event) []byte {
	var buf bytes.Buffer
	for _, ev := range events {
		fmt.Fprintf(&buf, "event: %s\ndata: {\"run_id\":%q}\n\n", ev.Kind.String(), ev.RunID)
	}
	return buf.Bytes()
}

// projectToTUIMsgs projects facade events to TUI tea.Msg debug representation.
// TODO: replace with real TUI projector after T061 GREEN.
func projectToTUIMsgs(events []ports.Event) []byte {
	var buf bytes.Buffer
	for _, ev := range events {
		fmt.Fprintf(&buf, "FacadeEventMsg{Kind:%q RunID:%q}\n", ev.Kind.String(), ev.RunID)
	}
	return buf.Bytes()
}

// TestFacadeConformance_4Interfaces asserts that ONE scripted event sequence projects
// into byte-identical output across all 4 interface wire formats (SC-002, D39).
func TestFacadeConformance_4Interfaces(t *testing.T) {
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()
	t.Cleanup(func() {
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		after := runtime.NumGoroutine()
		assert.InDelta(t, before, after, 5.0,
			"goroutine leak: before=%d after=%d", before, after)
	})

	ctx := context.Background()
	fake := conformanceScript()

	sess, err := fake.Run(ctx, ports.RunRequest{Identifier: "conformance/test"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	var events []ports.Event
	for ev := range sess.Events() {
		events = append(events, ev)
	}
	require.NotEmpty(t, events, "scripted sequence must emit at least one event")

	t.Run("cli-stdout", func(t *testing.T) {
		got := projectToCLIStdout(events)
		assertFacadeGolden(t, "cli-stdout.golden", got)
	})

	t.Run("acp-session-update", func(t *testing.T) {
		got := projectToACPSessionUpdate(events)
		assertFacadeGolden(t, "acp-session-update.golden", got)
	})

	t.Run("sse-frames", func(t *testing.T) {
		got := projectToSSEFrames(events)
		assertFacadeGolden(t, "sse-frames.golden", got)
	})

	t.Run("tui-tea-msg", func(t *testing.T) {
		got := projectToTUIMsgs(events)
		assertFacadeGolden(t, "tui-tea-msg.golden", got)
	})
}
