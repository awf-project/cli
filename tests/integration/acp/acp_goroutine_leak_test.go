//go:build integration && !windows

// Feature: F102
package acp_test

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/awf-project/cli/pkg/acpserver"
	"github.com/stretchr/testify/assert"
)

// TestACPClientHarness_NoInProcessGoroutineLeak_FiveTurnSession drives a real acp-serve
// subprocess through five prompt turns and asserts the *in-process test harness* does not
// leak goroutines across the session.
//
// SCOPE: this test measures goroutines in THIS process (the test binary) only.
// runtime.NumGoroutine() has no visibility into the acp-serve subprocess, so this test
// cannot detect server-side goroutine leaks. Server-side drain (Serve cancels serveCtx
// then s.wg.Wait()s every request handler on shutdown) is covered in-process by the
// pkg/acpserver tests run under -race, not here. This test exclusively guards the
// client-side request/response plumbing of the in-process test harness.
func TestACPClientHarness_NoInProcessGoroutineLeak_FiveTurnSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath := writeACPConfig(t, fixtureWorkflowsDir(t))
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", configPath))

	proc.request(t, 1, acpserver.MethodInitialize, map[string]any{
		"protocolVersion": "1.0.0",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	sessionResp := proc.request(t, 2, acpserver.MethodSessionNew, map[string]any{
		"sessionId": "leak-test-session",
	})
	result, _ := sessionResp.Result.(map[string]any)
	sessionID := fmt.Sprintf("%v", result["sessionId"])

	before := runtime.NumGoroutine()

	for i := range 5 {
		proc.request(t, 3+i, acpserver.MethodSessionPrompt, map[string]any{
			"sessionId": sessionID,
			"prompt": []map[string]any{
				{"type": "text", "text": "/trivial"},
			},
		})
	}

	after := runtime.NumGoroutine()

	assert.InDelta(t, before, after, 2.0,
		"in-process harness goroutine count must not grow after a 5-turn session (before=%d, after=%d); server-side drain is covered by pkg/acpserver -race tests per SC-003",
		before, after)
}
