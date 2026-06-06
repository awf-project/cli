//go:build integration && !windows

package acp_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	sdk "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

type jsonRPCResponse struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      json.RawMessage   `json:"id"`
	Result  any               `json:"result"`
	Error   *sdk.RequestError `json:"error,omitempty"`
}

const acpRPCTimeout = 5 * time.Second

func buildAWFBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "awf")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/awf")
	buildCmd.Dir = "../../.."
	require.NoError(t, buildCmd.Run(), "failed to build awf binary")
	return binaryPath
}

func writeACPConfig(t *testing.T, workflowsDir string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "acp-config.json")
	data, err := json.Marshal(map[string]any{
		"workflows_dir": workflowsDir,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))
	return configPath
}

func fixtureWorkflowsDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "acp", "workflows"))
	require.NoError(t, err)
	return abs
}

// acpProcess drives a running acp-serve subprocess. A single background read pump consumes
// stdout and demultiplexes frames: JSON-RPC responses are routed to the waiter registered
// for their id; server-originated notifications (session/update) and any unmatched frames
// (e.g. parse-error responses with id null) are forwarded to rawCh for tests that read the
// stream directly. This mirrors how a real ACP editor drives a concurrent agent: requests
// and responses are correlated by id, and notifications are interleaved freely.
type acpProcess struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	mu      sync.Mutex
	waiters map[string]chan jsonRPCResponse
	rawCh   chan []byte
}

func startACPServeProcess(t *testing.T, binaryPath string, args ...string) *acpProcess {
	t.Helper()
	cmdArgs := append([]string{"acp-serve"}, args...)
	cmd := exec.Command(binaryPath, cmdArgs...) //nolint:gosec // controlled test binary path
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, cmd.Start(), "failed to start acp-serve subprocess")

	p := &acpProcess{
		cmd:     cmd,
		stdin:   stdin,
		waiters: make(map[string]chan jsonRPCResponse),
		rawCh:   make(chan []byte, 1024),
	}
	go p.readPump(stdout)

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

	return p
}

// readPump reads newline-delimited frames and routes them. Responses (no "method") go to
// the matching id waiter when one is registered; everything else is forwarded to rawCh.
func (p *acpProcess) readPump(stdout io.Reader) {
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			p.route(line)
		}
		if err != nil {
			return
		}
	}
}

func (p *acpProcess) route(line []byte) {
	var probe struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if json.Unmarshal(line, &probe) == nil && probe.Method == "" && len(probe.ID) > 0 {
		key := string(probe.ID)
		p.mu.Lock()
		ch, ok := p.waiters[key]
		if ok {
			delete(p.waiters, key)
		}
		p.mu.Unlock()
		if ok {
			var resp jsonRPCResponse
			_ = json.Unmarshal(line, &resp)
			ch <- resp
			return
		}
	}
	// Notification, server request, or unmatched response (e.g. parse error, id null).
	// The default branch is intentionally non-blocking so the read pump is never stalled
	// by a slow or absent consumer. A log to stderr makes any drop visible in CI output
	// rather than silently losing frames that a test may be waiting for.
	select {
	case p.rawCh <- line:
	default:
		fmt.Fprintf(os.Stderr, "acp_test: rawCh full (capacity %d): dropping frame: %s\n", cap(p.rawCh), line)
	}
}

func (p *acpProcess) request(t *testing.T, id int, method string, params any) jsonRPCResponse {
	t.Helper()

	idKey := jsonIntID(id)
	ch := make(chan jsonRPCResponse, 1)
	p.mu.Lock()
	p.waiters[idKey] = ch
	p.mu.Unlock()

	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	payload, err := json.Marshal(req)
	require.NoError(t, err)
	payload = append(payload, '\n')

	_, err = p.stdin.Write(payload)
	require.NoError(t, err, "writing request to acp-serve stdin")

	select {
	case resp := <-ch:
		return resp
	case <-time.After(acpRPCTimeout):
		p.mu.Lock()
		delete(p.waiters, idKey)
		p.mu.Unlock()
		t.Fatalf("timed out waiting for response to %s (id=%d)", method, id)
	}
	return jsonRPCResponse{}
}

// jsonIntID renders an integer id the way encoding/json marshals it, so it matches the
// id bytes echoed back by the server.
func jsonIntID(id int) string {
	b, _ := json.Marshal(id)
	return string(b)
}

func (p *acpProcess) writeRaw(t *testing.T, data []byte) {
	t.Helper()
	_, err := p.stdin.Write(data)
	require.NoError(t, err, "writing raw bytes to acp-serve stdin")
}

func (p *acpProcess) readRawLine(t *testing.T, label string) []byte {
	t.Helper()
	select {
	case line := <-p.rawCh:
		return line
	case <-time.After(acpRPCTimeout):
		t.Fatalf("timed out waiting for raw response (%s)", label)
	}
	return nil
}

// drainForAvailableCommands reads session/update notifications from rawCh until one is an
// available_commands_update that advertises a command with the given name.
func (p *acpProcess) drainForAvailableCommands(t *testing.T, want string) bool {
	t.Helper()
	deadline := time.After(acpRPCTimeout)
	for {
		select {
		case line := <-p.rawCh:
			var n struct {
				Params struct {
					Update struct {
						SessionUpdate     string `json:"sessionUpdate"`
						AvailableCommands []struct {
							Name string `json:"name"`
						} `json:"availableCommands"`
					} `json:"update"`
				} `json:"params"`
			}
			if json.Unmarshal(line, &n) == nil &&
				n.Params.Update.SessionUpdate == "available_commands_update" {
				for _, cmd := range n.Params.Update.AvailableCommands {
					if cmd.Name == want {
						return true
					}
				}
			}
		case <-deadline:
			return false
		}
	}
}

// drainForChunk reads session/update notifications from rawCh until one is an
// agent_message_chunk whose text contains want, or the timeout elapses.
func (p *acpProcess) drainForChunk(t *testing.T, want string) bool {
	t.Helper()
	deadline := time.After(acpRPCTimeout)
	for {
		select {
		case line := <-p.rawCh:
			var n struct {
				Method string `json:"method"`
				Params struct {
					Update struct {
						SessionUpdate string `json:"sessionUpdate"`
						Content       struct {
							Text string `json:"text"`
						} `json:"content"`
					} `json:"update"`
				} `json:"params"`
			}
			if json.Unmarshal(line, &n) == nil &&
				n.Params.Update.SessionUpdate == "agent_message_chunk" &&
				strings.Contains(n.Params.Update.Content.Text, want) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}
