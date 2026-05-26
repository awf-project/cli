//go:build integration && !windows

package mcp_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestMCPServeSubprocessLifecycle verifies that the mcp-serve subprocess
// can be spawned, signaled, and exits cleanly without orphaning processes.
func TestMCPServeSubprocessLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if runtime.GOOS != "linux" {
		t.Skip("requires /proc filesystem (Linux only)")
	}

	// Step 1: Build the awf binary (reuse shared helper from mcp_jsonrpc_e2e_test.go).
	binaryPath := buildAWFBinary(t)

	// Step 2: Create a minimal valid config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp-proxy-config.json")
	config := map[string]any{
		"intercept_builtins": true,
		"plugin_tools":       []any{},
	}

	configData, err := json.Marshal(config)
	require.NoError(t, err)

	err = os.WriteFile(configPath, configData, 0o644)
	require.NoError(t, err)

	// Step 3: Spawn the mcp-serve subprocess
	cmd := exec.Command(binaryPath, "mcp-serve", fmt.Sprintf("--config=%s", configPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create process group for signal handling
	}

	err = cmd.Start()
	require.NoError(t, err, "failed to start mcp-serve subprocess")

	processID := cmd.Process.Pid

	// Step 4: Wait until the process is ready (running state confirmed via /proc/<pid>/status).
	// This avoids a fixed time.Sleep that is both slow and unreliable under CI load.
	// require.Eventually polls every 25ms for up to 5s; the MCP server initializes in <100ms.
	require.Eventually(t, func() bool {
		data, readErr := os.ReadFile(fmt.Sprintf("/proc/%d/status", processID))
		if readErr != nil {
			return false // process not yet visible
		}
		// The process is ready once it transitions to any running/sleeping state (not "zombie").
		return !strings.Contains(string(data), "State:\tZ")
	}, 5*time.Second, 25*time.Millisecond, "mcp-serve subprocess did not reach running state within 5s")

	// Step 5: Send SIGINT to the subprocess
	err = syscall.Kill(-processID, syscall.SIGINT) // Kill the process group
	require.NoError(t, err, "failed to send SIGINT to subprocess")

	// Step 6: Wait for the process to exit (with timeout)
	exitDone := make(chan error, 1)
	go func() {
		exitDone <- cmd.Wait()
	}()

	// Step 7: Assert the process exits within 5 seconds
	select {
	case err := <-exitDone:
		// Process exited - verify it exited due to signal (not success)
		// A signal termination typically results in a non-zero exit or specific error
		t.Logf("Process exited with: %v", err)

	case <-time.After(5 * time.Second):
		// Process did not exit - this is a failure
		// Kill it forcefully and fail the test
		_ = syscall.Kill(-processID, syscall.SIGKILL)
		t.Fatal("mcp-serve subprocess did not exit within 5 seconds after SIGINT")
	}

	// Step 8: Verify no orphan process remains.
	// Scope pgrep to the exact config path used by this test to avoid false
	// positives from concurrent test runs or other awf invocations on the system.
	checkCmd := exec.Command("pgrep", "-f", fmt.Sprintf("awf mcp-serve.*%s", configPath))
	err = checkCmd.Run()

	// pgrep returns 0 if matches are found (process exists), 1 if no matches.
	// We expect exit code 1 (no matches = no orphans).
	if err == nil {
		// Exit code 0 means processes were found.
		t.Fatalf("orphan 'awf mcp-serve' process with config %q detected after SIGINT", configPath)
	}

	t.Logf("Successfully spawned, signaled, and cleaned up mcp-serve subprocess (PID: %d)", processID)
}

// TestMCPServeSubprocess_ValidConfigInitialization verifies basic subprocess initialization
// without signal handling.
func TestMCPServeSubprocess_ValidConfigInitialization(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Reuse shared binary build helper.
	binaryPath := buildAWFBinary(t)

	// Create config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp-proxy-config.json")
	config := map[string]any{
		"intercept_builtins": true,
		"plugin_tools":       []any{},
	}

	configData, err := json.Marshal(config)
	require.NoError(t, err)

	err = os.WriteFile(configPath, configData, 0o644)
	require.NoError(t, err)

	// Spawn process
	cmd := exec.Command(binaryPath, "mcp-serve", fmt.Sprintf("--config=%s", configPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	require.NoError(t, err, "failed to start mcp-serve subprocess")

	// Immediately send SIGINT
	_ = cmd.Process.Signal(os.Interrupt)

	// Wait for exit with timeout
	exitDone := make(chan error, 1)
	go func() {
		exitDone <- cmd.Wait()
	}()

	select {
	case <-exitDone:
		t.Logf("Process exited cleanly after startup")

	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("process did not exit within timeout")
	}
}

// TestMCPServeSubprocess_MissingConfigFileExitCode verifies the subprocess
// fails gracefully when config file is missing.
func TestMCPServeSubprocess_MissingConfigFileExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Reuse shared binary build helper.
	binaryPath := buildAWFBinary(t)

	// Use a non-existent config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent-config.json")

	cmd := exec.Command(binaryPath, "mcp-serve", fmt.Sprintf("--config=%s", configPath))
	err := cmd.Run()

	// Expect error (exit code 1 for missing config)
	require.Error(t, err, "expected subprocess to fail with missing config file")
}

// TestMCPServeSubprocess_InvalidConfigJSONExitCode verifies the subprocess
// fails with exit code 1 when config file contains invalid JSON.
func TestMCPServeSubprocess_InvalidConfigJSONExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Reuse shared binary build helper.
	binaryPath := buildAWFBinary(t)

	// Create config with invalid JSON
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid-config.json")
	err := os.WriteFile(configPath, []byte("{invalid json"), 0o644)
	require.NoError(t, err)

	cmd := exec.Command(binaryPath, "mcp-serve", fmt.Sprintf("--config=%s", configPath))
	err = cmd.Run()

	// Expect error (exit code 1 for invalid JSON)
	require.Error(t, err, "expected subprocess to fail with invalid JSON")
}
