//go:build integration && !windows

// Feature: F099
package mcp_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"syscall"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/mcpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mcpRPCTimeout = 5 * time.Second

func buildAWFBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "awf")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/awf")
	buildCmd.Dir = "../../.."
	require.NoError(t, buildCmd.Run(), "failed to build awf binary")
	return binaryPath
}

// writeBuiltinsConfig writes an mcp-serve config that enables built-ins. It returns
// (configPath, rootDir). rootDir is the directory the proxy will treat as the
// workspace root; both the config file and any test files the agent will Read/Write
// must live under it for the path-traversal guard in builtins.WithRootDir to allow
// them through.
func writeBuiltinsConfig(t *testing.T) (configPath, rootDir string) {
	t.Helper()
	rootDir = t.TempDir()
	configPath = filepath.Join(rootDir, "mcp-config.json")
	data, err := json.Marshal(map[string]any{
		"intercept_builtins": true,
		"plugin_tools":       []any{},
		"root_dir":           rootDir,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))
	return configPath, rootDir
}

type mcpProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

func startMCPServeProcess(t *testing.T, binaryPath, configPath string) *mcpProcess {
	t.Helper()
	cmd := exec.Command(binaryPath, "mcp-serve", fmt.Sprintf("--config=%s", configPath))
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, cmd.Start(), "failed to start mcp-serve subprocess")

	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
	})

	return &mcpProcess{cmd: cmd, stdin: stdin, stdout: bufio.NewReader(stdout)}
}

func (p *mcpProcess) request(t *testing.T, id int, method string, params any) mcpserver.Response {
	t.Helper()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	payload, err := json.Marshal(req)
	require.NoError(t, err)
	payload = append(payload, '\n')

	_, err = p.stdin.Write(payload)
	require.NoError(t, err, "writing request to mcp-serve stdin")

	respCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		line, readErr := p.stdout.ReadBytes('\n')
		if readErr != nil {
			errCh <- readErr
			return
		}
		respCh <- line
	}()

	select {
	case line := <-respCh:
		var resp mcpserver.Response
		require.NoError(t, json.Unmarshal(line, &resp), "decoding response: %s", line)
		return resp
	case err := <-errCh:
		t.Fatalf("reading response: %v", err)
	case <-time.After(mcpRPCTimeout):
		t.Fatalf("timed out waiting for response to %s", method)
	}
	return mcpserver.Response{}
}

func TestMCPServeJSONRPC_ToolsList_ReturnsAllSixBuiltins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath, _ := writeBuiltinsConfig(t)
	proc := startMCPServeProcess(t, binaryPath, configPath)

	initResp := proc.request(t, 1, mcpserver.MethodInitialize, map[string]any{})
	require.Nil(t, initResp.Error, "initialize must succeed")

	listResp := proc.request(t, 2, mcpserver.MethodToolsList, nil)
	require.Nil(t, listResp.Error, "tools/list must succeed")

	result, ok := listResp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")
	rawTools, ok := result["tools"].([]any)
	require.True(t, ok, "result must contain a tools array")

	names := make([]string, 0, len(rawTools))
	for _, raw := range rawTools {
		def, isMap := raw.(map[string]any)
		require.True(t, isMap, "each tool must be an object")
		name, isStr := def["name"].(string)
		require.True(t, isStr, "each tool must have a string name")
		names = append(names, name)
	}
	sort.Strings(names)

	assert.Equal(t, []string{"Bash", "Edit", "Glob", "Grep", "Read", "Write"}, names,
		"proxy must expose exactly the six built-in tools")
}

func TestMCPServeJSONRPC_CallRead_ReturnsFileContents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath, rootDir := writeBuiltinsConfig(t)
	proc := startMCPServeProcess(t, binaryPath, configPath)

	target := filepath.Join(rootDir, "hello.txt")
	const want = "hello from F099\n"
	require.NoError(t, os.WriteFile(target, []byte(want), 0o644))

	proc.request(t, 1, mcpserver.MethodInitialize, map[string]any{})

	callResp := proc.request(t, 2, mcpserver.MethodToolsCall, map[string]any{
		"name":      "Read",
		"arguments": map[string]any{"path": target},
	})
	require.Nil(t, callResp.Error, "tools/call must succeed: %+v", callResp.Error)

	result, ok := callResp.Result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, result["isError"], "Read on an existing file must not flag isError")

	content, ok := result["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, content, "Read must produce at least one content block")

	block, ok := content[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, want, block["text"], "Read must return the file's exact contents")
}

func TestMCPServeJSONRPC_CallBash_ReturnsStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath, _ := writeBuiltinsConfig(t)
	proc := startMCPServeProcess(t, binaryPath, configPath)

	proc.request(t, 1, mcpserver.MethodInitialize, map[string]any{})

	callResp := proc.request(t, 2, mcpserver.MethodToolsCall, map[string]any{
		"name":      "Bash",
		"arguments": map[string]any{"command": "echo proxied-bash"},
	})
	require.Nil(t, callResp.Error, "tools/call must succeed: %+v", callResp.Error)

	result, ok := callResp.Result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, result["isError"], "successful bash command must not flag isError")

	content, ok := result["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, content)

	block, ok := content[0].(map[string]any)
	require.True(t, ok)
	text, _ := block["text"].(string)
	assert.Contains(t, text, "proxied-bash", "Bash stdout must reach the MCP client")
}

func TestMCPServeJSONRPC_CallUnknownTool_ReturnsRPCError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath, _ := writeBuiltinsConfig(t)
	proc := startMCPServeProcess(t, binaryPath, configPath)

	proc.request(t, 1, mcpserver.MethodInitialize, map[string]any{})

	callResp := proc.request(t, 2, mcpserver.MethodToolsCall, map[string]any{
		"name":      "NotARealTool",
		"arguments": map[string]any{},
	})

	require.NotNil(t, callResp.Error, "unknown tool must produce a JSON-RPC error, not a successful result")
	assert.Equal(t, mcpserver.ErrCodeMethodNotFound, callResp.Error.Code,
		"unknown tool must use the JSON-RPC method-not-found error code")
}
