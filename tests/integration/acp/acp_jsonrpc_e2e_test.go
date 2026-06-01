//go:build integration && !windows

// Feature: F102
package acp_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/acpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACPServeJSONRPC_Initialize_ReturnsCapabilities(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath := writeACPConfig(t, fixtureWorkflowsDir(t))
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", configPath))

	resp := proc.request(t, 1, acpserver.MethodInitialize, map[string]any{
		"protocolVersion": "1.0.0",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	assert.Nil(t, resp.Error, "initialize must succeed: %+v", resp.Error)
	assert.NotNil(t, resp.Result, "initialize must return result with capabilities")

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")

	protocolVersion, ok := result["protocolVersion"].(float64)
	assert.True(t, ok, "result must contain protocolVersion as a JSON number (ADR-018: integer)")
	assert.Equal(t, float64(acpserver.ProtocolVersion), protocolVersion, "protocolVersion must be the pinned integer")

	_, hasAgentCaps := result["agentCapabilities"]
	assert.True(t, hasAgentCaps, "result must advertise agentCapabilities")

	// Per the ACP initialize schema, the agent advertises authMethods as an array (empty
	// here — AWF supports no auth methods in v1). Real clients (JetBrains) expect the field.
	authMethods, hasAuthMethods := result["authMethods"]
	assert.True(t, hasAuthMethods, "result must include authMethods (empty array)")
	assert.Empty(t, authMethods, "AWF advertises no auth methods")
}

func TestACPServeJSONRPC_SessionNew_AdvertisesSlashCommands(t *testing.T) {
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

	start := time.Now()
	resp := proc.request(t, 2, acpserver.MethodSessionNew, map[string]any{
		"sessionId": "test-sn",
	})
	elapsed := time.Since(start)

	require.Nil(t, resp.Error, "session/new must succeed: %+v", resp.Error)
	assert.Less(t, elapsed, time.Second, "session/new must complete within 1 second per SC-001")

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")

	commands, ok := result["commands"].([]any)
	require.True(t, ok, "result must contain commands array")
	require.NotEmpty(t, commands, "commands must list at least one workflow")

	names := make([]string, 0, len(commands))
	for _, raw := range commands {
		cmd, isMap := raw.(map[string]any)
		require.True(t, isMap, "each command must be a JSON object")
		name, isStr := cmd["name"].(string)
		require.True(t, isStr, "each command must have a string name")
		names = append(names, name)
	}

	assert.Contains(t, names, "trivial", "commands must include trivial fixture workflow")
}

func TestACPServeJSONRPC_SessionPrompt_RunsWorkflow(t *testing.T) {
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
		"sessionId": "test-prompt",
	})
	result, _ := sessionResp.Result.(map[string]any)
	sessionID := fmt.Sprintf("%v", result["sessionId"])

	resp := proc.request(t, 3, acpserver.MethodSessionPrompt, map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": "/trivial"},
		},
	})

	assert.Nil(t, resp.Error, "session/prompt must succeed: %+v", resp.Error)
	assert.NotNil(t, resp.Result, "session/prompt must return result with output")

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")
	assert.NotEmpty(t, result, "session/prompt must return non-empty result")
}

func TestACPServeJSONRPC_SessionCancel_ReturnsCancelledStopReason(t *testing.T) {
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
		"sessionId": "test-cancel",
	})
	result, _ := sessionResp.Result.(map[string]any)
	sessionID := fmt.Sprintf("%v", result["sessionId"])

	go func() {
		time.Sleep(1 * time.Second)
		proc.request(t, 4, acpserver.MethodSessionCancel, map[string]any{
			"sessionId": sessionID,
		})
	}()

	start := time.Now()
	resp := proc.request(t, 3, acpserver.MethodSessionPrompt, map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": "/long-running"},
		},
	})
	elapsed := time.Since(start)

	assert.Nil(t, resp.Error, "cancelled session/prompt must not error: %+v", resp.Error)
	assert.Less(t, elapsed, 6*time.Second, "cancel response must arrive within 6s (5s SIGTERM grace + 1s overhead per SC-004)")

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")

	stopReason, ok := result["stopReason"].(string)
	assert.True(t, ok, "result must contain stopReason as string")
	assert.Equal(t, "cancelled", stopReason, "stopReason must be 'cancelled'")
}

func TestACPServeJSONRPC_UnsupportedBlock_RejectsWithUSERACPUnsupportedBlock(t *testing.T) {
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
		"sessionId": "test-unsupported",
	})
	result, _ := sessionResp.Result.(map[string]any)
	sessionID := fmt.Sprintf("%v", result["sessionId"])

	resp := proc.request(t, 3, acpserver.MethodSessionPrompt, map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]any{
			{
				"type": "image",
				"source": map[string]any{
					"type":     "base64",
					"mimeType": "image/png",
					"data":     "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
				},
			},
		},
	})

	assert.Nil(t, resp.Error, "unsupported block must return response (not error): %+v", resp.Error)

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")

	// The turn ends cleanly with a valid ACP stop reason; the USER.ACP.UNSUPPORTED_BLOCK
	// explanation is delivered to the user as an agent_message_chunk session/update (asserted
	// in the application-layer unit test), not encoded in the stopReason.
	stopReason, ok := result["stopReason"].(string)
	assert.True(t, ok, "result must contain stopReason")
	assert.Equal(t, "end_turn", stopReason, "unsupported block must end the turn with a valid stop reason")
}

func TestACPServeJSONRPC_MalformedJSONLine_Returns32700WithIDNull(t *testing.T) {
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

	proc.writeRaw(t, []byte("{bad json\n"))
	rawLine := proc.readRawLine(t, "malformed-json-response")

	var parseErrResp acpserver.Response
	require.NoError(t, json.Unmarshal(rawLine, &parseErrResp),
		"parse error response must be valid JSON: %s", rawLine)

	require.NotNil(t, parseErrResp.Error, "malformed JSON must produce an error response")
	assert.Equal(t, acpserver.ErrParse, parseErrResp.Error.Code,
		"error code must be -32700 (parse error)")

	assert.Equal(t, json.RawMessage("null"), parseErrResp.ID,
		"error response ID must be JSON null for parse error")

	recoveryResp := proc.request(t, 2, acpserver.MethodInitialize, map[string]any{
		"protocolVersion": "1.0.0",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	assert.Nil(t, recoveryResp.Error,
		"server must recover after parse error; subsequent valid request must succeed")
}

func TestACPServeJSONRPC_SessionPrompt_StreamsShellOutputLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	binaryPath := buildAWFBinary(t)
	configPath := writeACPConfig(t, fixtureWorkflowsDir(t))
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", configPath))

	proc.request(t, 1, acpserver.MethodInitialize, map[string]any{
		"protocolVersion": "1.0.0", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test", "version": "1.0.0"},
	})
	sn := proc.request(t, 2, acpserver.MethodSessionNew, map[string]any{})
	res, _ := sn.Result.(map[string]any)
	sid := fmt.Sprintf("%v", res["sessionId"])

	resp := proc.request(t, 3, acpserver.MethodSessionPrompt, map[string]any{
		"sessionId": sid,
		"prompt":    []map[string]any{{"type": "text", "text": "/input-echo --input=message=streamhello"}},
	})
	require.Nil(t, resp.Error, "prompt must succeed: %+v", resp.Error)

	if !proc.drainForChunk(t, "streamhello") {
		t.Fatal("expected live agent_message_chunk containing shell output 'streamhello'")
	}
}

func TestACPServeJSONRPC_OversizeLine_ReturnsStructuredError(t *testing.T) {
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

	oversizeLine := make([]byte, 10*1024*1024+1)
	for i := range oversizeLine {
		oversizeLine[i] = 'x'
	}
	oversizeLine[len(oversizeLine)-1] = '\n'
	proc.writeRaw(t, oversizeLine)

	rawLine := proc.readRawLine(t, "oversize-line-response")

	var errResp acpserver.Response
	require.NoError(t, json.Unmarshal(rawLine, &errResp),
		"oversize error response must be valid JSON: %s", rawLine)

	require.NotNil(t, errResp.Error, "oversize line (>10 MiB) must produce an error response")

	recoveryResp := proc.request(t, 2, acpserver.MethodInitialize, map[string]any{
		"protocolVersion": "1.0.0",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	assert.Nil(t, recoveryResp.Error,
		"server must survive oversize line; subsequent valid request must succeed")
}
