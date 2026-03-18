//go:build integration

package errors_test

// Feature: B012

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	freshBinaryPath string
	freshBinaryOnce sync.Once
)

// buildFreshBinary builds the CLI binary to a stable temp location to avoid stale binary issues.
// Uses os.MkdirTemp instead of t.TempDir() so the binary survives across tests.
func buildFreshBinary(t *testing.T) string {
	t.Helper()

	freshBinaryOnce.Do(func() {
		projectRoot, err := filepath.Abs("../../..")
		if err != nil {
			t.Fatalf("resolve project root: %v", err)
		}

		tmpDir, err := os.MkdirTemp("", "awf-b012-test-*")
		if err != nil {
			t.Fatalf("create temp dir: %v", err)
		}

		binPath := filepath.Join(tmpDir, "awf")

		cmd := exec.Command("go", "build", "-o", binPath, "./cmd/awf")
		cmd.Dir = projectRoot

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build binary: %s\n%s", err, string(output))
		}

		freshBinaryPath = binPath
	})

	if freshBinaryPath == "" {
		t.Fatal("binary not built")
	}
	return freshBinaryPath
}

func TestJSONErrorOutput_GoesToStderr_Integration(t *testing.T) {
	binaryPath := buildFreshBinary(t)
	fixtureDir, err := filepath.Abs("../../fixtures/workflows")
	require.NoError(t, err)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing required input produces JSON error on stderr",
			args: []string{"--format", "json", "run", "exit-user-error.yaml"},
		},
		{
			name: "nonexistent workflow produces JSON error on stderr",
			args: []string{"--format", "json", "run", "nonexistent-workflow.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			require.Error(t, err, "command should fail")

			assert.Empty(t, stdout.String(),
				"stdout should be empty when errors occur in JSON mode")

			assert.NotEmpty(t, stderr.String(),
				"stderr should contain error output")

			var errorResp map[string]any
			jsonErr := json.Unmarshal(stderr.Bytes(), &errorResp)
			require.NoError(t, jsonErr,
				"stderr should contain valid JSON, got: %s", stderr.String())

			assert.NotEmpty(t, errorResp["error"], "JSON error response should have 'error' field")
			assert.NotNil(t, errorResp["code"], "JSON error response should have 'code' field")
		})
	}
}

func TestTextErrorOutput_GoesToStderr_Integration(t *testing.T) {
	binaryPath := buildFreshBinary(t)
	fixtureDir, err := filepath.Abs("../../fixtures/workflows")
	require.NoError(t, err)

	cmd := exec.Command(binaryPath, "run", "nonexistent-workflow.yaml")
	cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	require.Error(t, err, "command should fail for nonexistent workflow")

	assert.NotEmpty(t, stderr.String(),
		"stderr should contain error message in text mode")
}

func TestJSONErrorOutput_ExitCodeMatchesErrorType_Integration(t *testing.T) {
	binaryPath := buildFreshBinary(t)
	fixtureDir, err := filepath.Abs("../../fixtures/workflows")
	require.NoError(t, err)

	tests := []struct {
		name         string
		args         []string
		wantExitCode int
		jsonOnStderr bool // pre-execution errors go to stderr; execution errors produce JSON on stdout
	}{
		{
			name:         "user error maps to exit code 1",
			args:         []string{"--format", "json", "run", "exit-user-error.yaml"},
			wantExitCode: 1,
			jsonOnStderr: true,
		},
		{
			name:         "execution error maps to exit code 3",
			args:         []string{"--format", "json", "run", "exit-execution-error.yaml"},
			wantExitCode: 3,
			jsonOnStderr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			require.Error(t, err)

			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "should be an ExitError")
			assert.Equal(t, tt.wantExitCode, exitErr.ExitCode())

			jsonSource := stderr.Bytes()
			if !tt.jsonOnStderr {
				jsonSource = stdout.Bytes()
			}

			var errorResp map[string]any
			jsonErr := json.Unmarshal(jsonSource, &errorResp)
			require.NoError(t, jsonErr, "should contain valid JSON")

			assert.NotEmpty(t, errorResp["error"], "JSON response should have 'error' field")
		})
	}
}
