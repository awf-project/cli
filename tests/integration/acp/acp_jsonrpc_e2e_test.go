//go:build integration && !windows

package acp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	sdk "github.com/coder/acp-go-sdk"
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

	resp := proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
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
	assert.Equal(t, float64(sdk.ProtocolVersionNumber), protocolVersion, "protocolVersion must be the pinned integer")

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

	proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	start := time.Now()
	resp := proc.request(t, 2, sdk.AgentMethodSessionNew, map[string]any{
		"cwd":        t.TempDir(),
		"mcpServers": []any{},
	})
	elapsed := time.Since(start)

	require.Nil(t, resp.Error, "session/new must succeed: %+v", resp.Error)
	assert.Less(t, elapsed, time.Second, "session/new must complete within 1 second per SC-001")

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")
	require.NotEmpty(t, result, "result must contain sessionId")

	// Per SDK protocol, available commands are delivered as an available_commands_update
	// session/update notification, not in the session/new response body.
	if !proc.drainForAvailableCommands(t, "trivial") {
		t.Fatal("expected available_commands_update notification advertising trivial workflow")
	}
}

func TestACPServeJSONRPC_SessionPrompt_RunsWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath := writeACPConfig(t, fixtureWorkflowsDir(t))
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", configPath))

	proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	sessionResp := proc.request(t, 2, sdk.AgentMethodSessionNew, map[string]any{
		"cwd":        t.TempDir(),
		"mcpServers": []any{},
	})
	result, _ := sessionResp.Result.(map[string]any)
	sessionID := fmt.Sprintf("%v", result["sessionId"])

	resp := proc.request(t, 3, sdk.AgentMethodSessionPrompt, map[string]any{
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

	confidential := []string{
		"PRIVATE_WORKFLOW_INPUT_T036",
		"PRIVATE_COMMAND_OUTPUT_T036",
		"PRIVATE_PROMPT_T036",
		"PRIVATE_ENV_VALUE_T036",
	}
	t.Setenv("T036_PRIVATE_ENV", confidential[3])

	// Use real temporary workflow files so the values traverse the same parser,
	// interpolation, shell executor, and ACP JSON-RPC server as production traffic.
	workflowsDir := t.TempDir()
	trivial, err := os.ReadFile(filepath.Join(fixtureWorkflowsDir(t), "trivial.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "trivial.yaml"), trivial, 0o600))
	longRunning := fmt.Sprintf(`name: long-running
version: "1.0.0"
inputs:
  - name: workflow_input
    type: string
    required: false
states:
  initial: run
  run:
    type: step
    command: "printf '%%s\\n' '{{inputs.workflow_input}}'; printf '%%s\\n' '%s'; printf '%%s\\n' \"$T036_PRIVATE_ENV\"; sleep 30"
    on_success: done
    on_failure: done
  done:
    type: terminal
`, confidential[1])
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "long-running.yaml"), []byte(longRunning), 0o600))

	binaryPath := buildAWFBinary(t)
	configPath := writeACPConfig(t, workflowsDir)
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", configPath))

	proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	sessionResp := proc.request(t, 2, sdk.AgentMethodSessionNew, map[string]any{
		"cwd":        t.TempDir(),
		"mcpServers": []any{},
	})
	result, _ := sessionResp.Result.(map[string]any)
	sessionID := fmt.Sprintf("%v", result["sessionId"])

	type asyncResult struct {
		response jsonRPCResponse
		err      error
	}
	cancelBound := 6 * time.Second
	if deadline, ok := t.Deadline(); ok && time.Until(deadline) < cancelBound {
		cancelBound = time.Until(deadline)
	}
	requestCtx, requestCancel := context.WithTimeout(context.Background(), cancelBound)
	defer requestCancel()
	promptResponse := make(chan asyncResult, 1)
	idKey := jsonIntID(3)
	waiter := make(chan jsonRPCResponse, 1)
	proc.mu.Lock()
	proc.waiters[idKey] = waiter
	proc.mu.Unlock()
	promptRequest := map[string]any{"jsonrpc": "2.0", "id": 3, "method": sdk.AgentMethodSessionPrompt, "params": map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": "/long-running workflow_input=" + confidential[0] + " prompt=" + confidential[2]},
		},
	}}
	payload, err := json.Marshal(promptRequest)
	require.NoError(t, err)
	payload = append(payload, '\n')
	_, err = proc.stdin.Write(payload)
	require.NoError(t, err)
	go func() {
		select {
		case response := <-waiter:
			promptResponse <- asyncResult{response: response}
		case <-requestCtx.Done():
			proc.mu.Lock()
			delete(proc.waiters, idKey)
			proc.mu.Unlock()
			promptResponse <- asyncResult{err: requestCtx.Err()}
		}
	}()

	descendantContains := func(want string) bool {
		output, err := exec.Command("ps", "-eo", "pid=,ppid=,args=").Output()
		require.NoError(t, err, "inspect acp-serve descendants")
		type process struct {
			pid, parent string
			args        string
		}
		var processes []process
		for line := range strings.SplitSeq(string(output), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				processes = append(processes, process{
					pid: fields[0], parent: fields[1], args: strings.Join(fields[2:], " "),
				})
			}
		}
		descendants := map[string]bool{strconv.Itoa(proc.cmd.Process.Pid): true}
		for range processes {
			for i := range processes {
				if descendants[processes[i].parent] {
					descendants[processes[i].pid] = true
				}
			}
		}
		for i := range processes {
			if descendants[processes[i].pid] && strings.Contains(processes[i].args, want) {
				return true
			}
		}
		return false
	}
	require.Eventually(t, func() bool { return descendantContains("sleep 30") },
		acpRPCTimeout, 10*time.Millisecond,
		"long-running command must be active before session/cancel")

	start := time.Now()
	cancelResp := proc.request(t, 4, sdk.AgentMethodSessionCancel, map[string]any{
		"sessionId": sessionID,
	})
	require.Nil(t, cancelResp.Error, "session/cancel must succeed: %+v", cancelResp.Error)

	var resp jsonRPCResponse
	select {
	case promptResult := <-promptResponse:
		require.NoError(t, promptResult.err, "cancelled session/prompt request must complete")
		resp = promptResult.response
	case <-requestCtx.Done():
		t.Fatalf("timed out waiting for cancelled session/prompt response after %s", cancelBound)
	}
	elapsed := time.Since(start)
	assert.Nil(t, resp.Error, "cancelled session/prompt must not error: %+v", resp.Error)
	assert.Less(t, elapsed, 6*time.Second, "cancel response must arrive within 6s (5s SIGTERM grace + 1s overhead per SC-004)")

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")

	stopReason, ok := result["stopReason"].(string)
	assert.True(t, ok, "result must contain stopReason as string")
	assert.Equal(t, "cancelled", stopReason, "stopReason must be 'cancelled'")
	require.Eventually(t, func() bool { return !descendantContains("sleep 30") },
		acpRPCTimeout, 10*time.Millisecond,
		"cancelled command must exit before cleanup completes")
	encodedResponse, err := json.Marshal(resp)
	require.NoError(t, err)

	cleanupResp := proc.request(t, 5, sdk.AgentMethodSessionPrompt, map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": "/trivial"},
		},
	})
	require.Nil(t, cleanupResp.Error,
		"session must remain usable after cancelled workflow cleanup: %+v", cleanupResp.Error)
	cleanupResult, ok := cleanupResp.Result.(map[string]any)
	require.True(t, ok, "post-cancellation prompt result must be a JSON object")
	assert.Equal(t, "end_turn", cleanupResult["stopReason"],
		"a fresh workflow must complete after cancelled workflow cleanup")

	// Error envelopes must not echo values that traversed supported ACP and workflow paths.
	protocolErrors, err := json.Marshal([]*sdk.RequestError{
		sessionResp.Error,
		cancelResp.Error,
		resp.Error,
		cleanupResp.Error,
	})
	require.NoError(t, err)
	for _, secret := range confidential {
		assert.NotContains(t, string(encodedResponse), secret, "JSON-RPC response must not disclose confidential data")
		assert.NotContains(t, string(protocolErrors), secret, "captured JSON-RPC errors must not disclose confidential data")
	}
}

func TestACPServeJSONRPC_UnsupportedBlock_RejectsWithUSERACPUnsupportedBlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath := writeACPConfig(t, fixtureWorkflowsDir(t))
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", configPath))

	proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	sessionResp := proc.request(t, 2, sdk.AgentMethodSessionNew, map[string]any{
		"cwd":        t.TempDir(),
		"mcpServers": []any{},
	})
	result, _ := sessionResp.Result.(map[string]any)
	sessionID := fmt.Sprintf("%v", result["sessionId"])

	resp := proc.request(t, 3, sdk.AgentMethodSessionPrompt, map[string]any{
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

	proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	proc.writeRaw(t, []byte("{bad json\n"))
	rawLine := proc.readRawLine(t, "malformed-json-response")

	var parseErrResp jsonRPCResponse
	require.NoError(t, json.Unmarshal(rawLine, &parseErrResp),
		"parse error response must be valid JSON: %s", rawLine)

	require.NotNil(t, parseErrResp.Error, "malformed JSON must produce an error response")
	assert.Equal(t, -32700, parseErrResp.Error.Code,
		"error code must be -32700 (parse error)")

	assert.Equal(t, json.RawMessage("null"), parseErrResp.ID,
		"error response ID must be JSON null for parse error")

	recoveryResp := proc.request(t, 2, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
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

	proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber, "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test", "version": "1.0.0"},
	})
	sn := proc.request(t, 2, sdk.AgentMethodSessionNew, map[string]any{
		"cwd":        t.TempDir(),
		"mcpServers": []any{},
	})
	res, _ := sn.Result.(map[string]any)
	sid := fmt.Sprintf("%v", res["sessionId"])

	resp := proc.request(t, 3, sdk.AgentMethodSessionPrompt, map[string]any{
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

	proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
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

	var errResp jsonRPCResponse
	require.NoError(t, json.Unmarshal(rawLine, &errResp),
		"oversize error response must be valid JSON: %s", rawLine)

	require.NotNil(t, errResp.Error, "oversize line (>10 MiB) must produce an error response")

	recoveryResp := proc.request(t, 2, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	assert.Nil(t, recoveryResp.Error,
		"server must survive oversize line; subsequent valid request must succeed")
}
