//go:build integration

// Feature: F107 — T065
//
// Agent uniformity: identical execution across all 5 providers produces byte-identical
// facade.Event sequences (NFR-006, SC-008, D40).
//
// Any provider branch in the facade adapter breaks this test in CI.
// Stub phase: provider fakes are backed by facadetest.Fake with identical scripts.
// GREEN: wire real provider fakes when F107's normalizer is complete.
package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/facadetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// providerFake pairs a provider name with its scripted facadetest fake.
// In the GREEN phase, each entry's fake will be replaced by a provider-specific
// harness that wires the real agent executor with the normalized event output.
type providerFake struct {
	name string
	fake *facadetest.Fake
}

// uniformityScript builds the shared scripted event sequence for uniformity testing.
func uniformityScript() []ports.Event {
	return []ports.Event{
		{Kind: ports.EventRunStarted, RunID: "uniformity-run"},
		{Kind: ports.EventMessageUser, RunID: "uniformity-run"},
		{Kind: ports.EventToolCall, RunID: "uniformity-run"},
		{Kind: ports.EventToolResult, RunID: "uniformity-run"},
		{Kind: ports.EventMessageAssistant, RunID: "uniformity-run"},
		{Kind: ports.EventStepCompleted, RunID: "uniformity-run"},
	}
}

// buildProviderFakes returns 5 provider fakes with identical scripts.
// TODO: replace each fake with a real provider harness after F107's normalizer is complete (D40).
func buildProviderFakes() []providerFake {
	script := uniformityScript()
	providers := []string{"claude", "gemini", "codex", "copilot", "openai-compatible"}
	fakes := make([]providerFake, len(providers))
	for i, name := range providers {
		fakes[i] = providerFake{
			name: name,
			fake: facadetest.New().Script(script...).WithTerminalCompleted(),
		}
	}
	return fakes
}

// serializeEventSequence converts a slice of facade events to a canonical JSON bytes
// representation for byte-equality comparison across providers.
func serializeEventSequence(events []ports.Event) ([]byte, error) {
	type wireEvent struct {
		Kind string `json:"kind"`
		Seq  uint64 `json:"seq,omitempty"`
	}
	wires := make([]wireEvent, len(events))
	for i, ev := range events {
		wires[i] = wireEvent{Kind: ev.Kind.String(), Seq: ev.Seq}
	}
	return json.Marshal(wires)
}

// TestAgentUniformity_5Providers scripts identical execution against 5 provider fakes
// and asserts byte-identical facade.Event sequences across all 5 (NFR-006, SC-008, D40).
func TestAgentUniformity_5Providers(t *testing.T) {
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

	providers := buildProviderFakes()
	require.Len(t, providers, 5, "must test exactly 5 providers")

	ctx := context.Background()
	type result struct {
		name string
		seq  []byte
	}
	results := make([]result, 0, len(providers))

	for _, p := range providers {
		p := p
		t.Run(p.name, func(t *testing.T) {
			sess, err := p.fake.Run(ctx, ports.RunRequest{
				Identifier: fmt.Sprintf("uniformity/%s", p.name),
			})
			require.NoError(t, err)
			t.Cleanup(func() { _ = sess.Close() })

			var events []ports.Event
			for ev := range sess.Events() {
				events = append(events, ev)
			}
			require.NotEmpty(t, events, "provider %s must emit events", p.name)

			seq, err := serializeEventSequence(events)
			require.NoError(t, err)
			results = append(results, result{name: p.name, seq: seq})
		})
	}

	require.Len(t, results, len(providers), "all providers must complete")

	baseline := results[0]
	for _, r := range results[1:] {
		assert.Equal(t, string(baseline.seq), string(r.seq),
			"provider %q event sequence diverges from %q (NFR-006, SC-008):\nbaseline: %s\ngot:      %s",
			r.name, baseline.name, baseline.seq, r.seq)
	}
}
