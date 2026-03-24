package executor

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellExecutor_Execute_SimpleCommand(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		wantStdout string
		wantCode   int
	}{
		{
			name:       "echo hello",
			command:    "echo hello",
			wantStdout: "hello\n",
			wantCode:   0,
		},
		{
			name:       "multiline output",
			command:    "echo line1; echo line2",
			wantStdout: "line1\nline2\n",
			wantCode:   0,
		},
		{
			name:       "empty output",
			command:    "true",
			wantStdout: "",
			wantCode:   0,
		},
	}

	executor := NewShellExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ports.Command{Program: tt.command}
			result, err := executor.Execute(context.Background(), &cmd)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStdout, result.Stdout)
			assert.Equal(t, tt.wantCode, result.ExitCode)
		})
	}
}

func TestShellExecutor_Execute_CapturesStderr(t *testing.T) {
	executor := NewShellExecutor()
	cmd := ports.Command{Program: "echo error >&2"}

	result, err := executor.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, "error\n", result.Stderr)
	assert.Empty(t, result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
}

func TestShellExecutor_Execute_BothStdoutAndStderr(t *testing.T) {
	executor := NewShellExecutor()
	cmd := ports.Command{Program: "echo out; echo err >&2"}

	result, err := executor.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, "out\n", result.Stdout)
	assert.Equal(t, "err\n", result.Stderr)
	assert.Equal(t, 0, result.ExitCode)
}

func TestShellExecutor_Execute_NonZeroExitCode(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantCode int
	}{
		{
			name:     "exit 1",
			command:  "exit 1",
			wantCode: 1,
		},
		{
			name:     "exit 42",
			command:  "exit 42",
			wantCode: 42,
		},
		{
			name:     "false command",
			command:  "false",
			wantCode: 1,
		},
	}

	executor := NewShellExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ports.Command{Program: tt.command}
			result, err := executor.Execute(context.Background(), &cmd)

			// non-zero exit code is not an error
			require.NoError(t, err)
			assert.Equal(t, tt.wantCode, result.ExitCode)
		})
	}
}

func TestShellExecutor_Execute_ContextTimeout(t *testing.T) {
	executor := NewShellExecutor()
	cmd := ports.Command{Program: "sleep 10"}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	result, err := executor.Execute(ctx, &cmd)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 3*time.Second)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NotNil(t, result)
}

func TestShellExecutor_Execute_ContextCancellation(t *testing.T) {
	executor := NewShellExecutor()
	cmd := ports.Command{Program: "sleep 10"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// cancel after 500ms
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result, err := executor.Execute(ctx, &cmd)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 2*time.Second)
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotNil(t, result)
}

func TestShellExecutor_Execute_WorkingDirectory(t *testing.T) {
	executor := NewShellExecutor()
	tmpDir := t.TempDir()

	cmd := ports.Command{
		Program: "pwd",
		Dir:     tmpDir,
	}

	result, err := executor.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, tmpDir+"\n", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
}

func TestShellExecutor_Execute_Environment(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		command    string
		wantStdout string
	}{
		{
			name:       "single env var",
			env:        map[string]string{"FOO": "bar"},
			command:    "echo $FOO",
			wantStdout: "bar\n",
		},
		{
			name:       "multiple env vars",
			env:        map[string]string{"FOO": "hello", "BAR": "world"},
			command:    "echo $FOO $BAR",
			wantStdout: "hello world\n",
		},
		{
			name:       "override existing env",
			env:        map[string]string{"HOME": "/custom/home"},
			command:    "echo $HOME",
			wantStdout: "/custom/home\n",
		},
	}

	executor := NewShellExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ports.Command{
				Program: tt.command,
				Env:     tt.env,
			}

			result, err := executor.Execute(context.Background(), &cmd)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStdout, result.Stdout)
		})
	}
}

func TestShellExecutor_ImplementsInterface(t *testing.T) {
	var _ ports.CommandExecutor = (*ShellExecutor)(nil)
}

func TestShellExecutor_Execute_StreamsStdout(t *testing.T) {
	executor := NewShellExecutor()

	var streamBuf bytes.Buffer
	cmd := ports.Command{
		Program: "echo hello",
		Stdout:  &streamBuf,
	}

	result, err := executor.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, "hello\n", result.Stdout, "captured stdout")
	assert.Equal(t, "hello\n", streamBuf.String(), "streamed stdout")
}

func TestShellExecutor_Execute_StreamsStderr(t *testing.T) {
	executor := NewShellExecutor()

	var streamBuf bytes.Buffer
	cmd := ports.Command{
		Program: "echo error >&2",
		Stderr:  &streamBuf,
	}

	result, err := executor.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, "error\n", result.Stderr, "captured stderr")
	assert.Equal(t, "error\n", streamBuf.String(), "streamed stderr")
}

func TestShellExecutor_Execute_StreamsBoth(t *testing.T) {
	executor := NewShellExecutor()

	var outBuf, errBuf bytes.Buffer
	cmd := ports.Command{
		Program: "echo out; echo err >&2",
		Stdout:  &outBuf,
		Stderr:  &errBuf,
	}

	result, err := executor.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, "out\n", result.Stdout)
	assert.Equal(t, "err\n", result.Stderr)
	assert.Equal(t, "out\n", outBuf.String())
	assert.Equal(t, "err\n", errBuf.String())
}

func TestShellExecutor_Execute_NilWriters_BackwardCompatible(t *testing.T) {
	executor := NewShellExecutor()
	cmd := ports.Command{Program: "echo hello"}

	result, err := executor.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, "hello\n", result.Stdout)
}

// TestShellExecutor_MaskSecrets_HappyPath tests normal secret masking scenarios
// Feature: C011 - Task T018
func TestShellExecutor_MaskSecrets_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		env        map[string]string
		wantStdout string
		wantStderr string
	}{
		{
			name:    "mask SECRET_ prefix in stdout",
			command: "echo $SECRET_API_TOKEN",
			env: map[string]string{
				"SECRET_API_TOKEN": "super_secret_value_12345",
				"PUBLIC_VAR":       "visible",
			},
			wantStdout: "***\n",
			wantStderr: "",
		},
		{
			name:    "mask API_KEY prefix in stdout",
			command: "echo $API_KEY_OPENAI",
			env: map[string]string{
				"API_KEY_OPENAI": "sk-proj-abc123def456",
			},
			wantStdout: "***\n",
			wantStderr: "",
		},
		{
			name:    "mask PASSWORD prefix in stdout",
			command: "echo $PASSWORD_DB",
			env: map[string]string{
				"PASSWORD_DB": "admin_pass_987654",
			},
			wantStdout: "***\n",
			wantStderr: "",
		},
		{
			name:    "mask multiple secrets in same output",
			command: "echo $SECRET_TOKEN $API_KEY",
			env: map[string]string{
				"SECRET_TOKEN": "abc123",
				"API_KEY":      "xyz789",
			},
			wantStdout: "*** ***\n",
			wantStderr: "",
		},
		{
			name:    "preserve non-secret values",
			command: "echo $PUBLIC_VAR $CUSTOM_NAME",
			env: map[string]string{
				"PUBLIC_VAR":  "visible",
				"CUSTOM_NAME": "john",
			},
			wantStdout: "visible john\n",
			wantStderr: "",
		},
		{
			name:    "mask secrets in stderr",
			command: "echo $SECRET_KEY >&2",
			env: map[string]string{
				"SECRET_KEY": "hidden_value",
			},
			wantStdout: "",
			wantStderr: "***\n",
		},
		{
			name:    "mask secrets in both stdout and stderr",
			command: "echo $SECRET_A; echo $SECRET_B >&2",
			env: map[string]string{
				"SECRET_A": "value_a",
				"SECRET_B": "value_b",
			},
			wantStdout: "***\n",
			wantStderr: "***\n",
		},
	}

	executor := NewShellExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ports.Command{
				Program: tt.command,
				Env:     tt.env,
			}

			result, err := executor.Execute(context.Background(), &cmd)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStdout, result.Stdout, "stdout should have secrets masked")
			assert.Equal(t, tt.wantStderr, result.Stderr, "stderr should have secrets masked")
		})
	}
}

// TestShellExecutor_MaskSecrets_EdgeCases tests boundary conditions for secret masking
// Feature: C011 - Task T018
func TestShellExecutor_MaskSecrets_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		env        map[string]string
		wantStdout string
	}{
		{
			name:       "empty env map",
			command:    "echo hello",
			env:        map[string]string{},
			wantStdout: "hello\n",
		},
		{
			name:       "nil env map",
			command:    "echo hello",
			env:        nil,
			wantStdout: "hello\n",
		},
		{
			name:    "secret appears multiple times",
			command: "echo $SECRET_TOKEN $SECRET_TOKEN",
			env: map[string]string{
				"SECRET_TOKEN": "repeated",
			},
			wantStdout: "*** ***\n",
		},
		{
			name:    "overlapping secret values",
			command: "echo abc abcdef",
			env: map[string]string{
				"SECRET_A": "abc",
				"SECRET_B": "abcdef",
			},
			wantStdout: "*** ***\n",
		},
		{
			name:    "empty secret value not masked",
			command: "echo SECRET_KEY=",
			env: map[string]string{
				"SECRET_KEY": "",
			},
			wantStdout: "SECRET_KEY=\n",
		},
		{
			name:    "secret with special characters",
			command: "echo 'p@ss.w0rd+123'",
			env: map[string]string{
				"PASSWORD": "p@ss.w0rd+123",
			},
			wantStdout: "***\n",
		},
		{
			name:    "multiline output with secrets",
			command: "echo line1:$SECRET_KEY; echo line2:normal; echo line3:$API_KEY",
			env: map[string]string{
				"SECRET_KEY": "abc123",
				"API_KEY":    "xyz789",
			},
			wantStdout: "line1:***\nline2:normal\nline3:***\n",
		},
	}

	executor := NewShellExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ports.Command{
				Program: tt.command,
				Env:     tt.env,
			}

			result, err := executor.Execute(context.Background(), &cmd)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStdout, result.Stdout)
		})
	}
}

// TestShellExecutor_MaskSecrets_CaseInsensitive tests case-insensitive key matching
// Feature: C011 - Task T018
func TestShellExecutor_MaskSecrets_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		env        map[string]string
		wantStdout string
	}{
		{
			name:    "lowercase secret_ prefix",
			command: "echo $secret_key",
			env: map[string]string{
				"secret_key": "hidden123",
			},
			wantStdout: "***\n",
		},
		{
			name:    "mixed case api_key prefix",
			command: "echo $Api_Key_OpenAI",
			env: map[string]string{
				"Api_Key_OpenAI": "xyz",
			},
			wantStdout: "***\n",
		},
		{
			name:    "uppercase PASSWORD prefix",
			command: "echo $PASSWORD",
			env: map[string]string{
				"PASSWORD": "pass123",
			},
			wantStdout: "***\n",
		},
	}

	executor := NewShellExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ports.Command{
				Program: tt.command,
				Env:     tt.env,
			}

			result, err := executor.Execute(context.Background(), &cmd)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStdout, result.Stdout)
		})
	}
}

// TestShellExecutor_MaskSecrets_ErrorHandling tests error output masking
// Feature: C011 - Task T018
func TestShellExecutor_MaskSecrets_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		env        map[string]string
		wantStderr string
		wantCode   int
	}{
		{
			name:    "mask secret in error message",
			command: "echo 'Error: Authentication failed with token '$SECRET_TOKEN >&2; exit 1",
			env: map[string]string{
				"SECRET_TOKEN": "secret123",
			},
			wantStderr: "Error: Authentication failed with token ***\n",
			wantCode:   1,
		},
		{
			name:    "mask API key in curl error",
			command: "echo 'curl: unauthorized key='$API_KEY >&2; exit 1",
			env: map[string]string{
				"API_KEY": "sk-abc123",
			},
			wantStderr: "curl: unauthorized key=***\n",
			wantCode:   1,
		},
		{
			name:    "non-secret error preserved",
			command: "echo 'Error: command not found' >&2; exit 1",
			env: map[string]string{
				"SECRET_KEY": "hidden",
			},
			wantStderr: "Error: command not found\n",
			wantCode:   1,
		},
	}

	executor := NewShellExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ports.Command{
				Program: tt.command,
				Env:     tt.env,
			}

			result, err := executor.Execute(context.Background(), &cmd)

			// non-zero exit is not an error for executor
			require.NoError(t, err)
			assert.Equal(t, tt.wantStderr, result.Stderr, "error output should mask secrets")
			assert.Equal(t, tt.wantCode, result.ExitCode)
		})
	}
}

// TestShellExecutor_MaskSecrets_RealWorldScenarios tests realistic command patterns
// Feature: C011 - Task T018
func TestShellExecutor_MaskSecrets_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		env        map[string]string
		wantStdout string
	}{
		{
			name:    "environment variable listing",
			command: "echo SECRET_API_TOKEN=$SECRET_API_TOKEN; echo API_KEY_OPENAI=$API_KEY_OPENAI; echo PUBLIC_VAR=$PUBLIC_VAR",
			env: map[string]string{
				"SECRET_API_TOKEN": "super_secret_value_12345",
				"API_KEY_OPENAI":   "sk-proj-abc123",
				"PUBLIC_VAR":       "visible_value",
			},
			wantStdout: "SECRET_API_TOKEN=***\nAPI_KEY_OPENAI=***\nPUBLIC_VAR=visible_value\n",
		},
		{
			name:    "JSON output with secrets",
			command: `echo '{"api_key":"'$API_KEY_OPENAI'","user":"john","password":"'$PASSWORD_DB'"}'`,
			env: map[string]string{
				"API_KEY_OPENAI": "sk-proj-abc123",
				"PASSWORD_DB":    "admin_pass",
			},
			wantStdout: `{"api_key":"***","user":"john","password":"***"}` + "\n",
		},
		{
			name:    "shell command with secret arguments",
			command: `echo 'curl -H "Authorization: Bearer '$API_KEY'" https://api.example.com'`,
			env: map[string]string{
				"API_KEY": "sk-proj-abc123",
			},
			wantStdout: `curl -H "Authorization: Bearer ***" https://api.example.com` + "\n",
		},
		{
			name:    "mixed secrets and public data",
			command: "echo user=$TEST_USER token=$SECRET_TOKEN status=$STATUS",
			env: map[string]string{
				"TEST_USER":    "alice",
				"SECRET_TOKEN": "abc123",
				"STATUS":       "active",
			},
			wantStdout: "user=alice token=*** status=active\n",
		},
	}

	executor := NewShellExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ports.Command{
				Program: tt.command,
				Env:     tt.env,
			}

			result, err := executor.Execute(context.Background(), &cmd)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStdout, result.Stdout)
		})
	}
}

func TestDetectShell_Scenarios(t *testing.T) {
	validShellDir := t.TempDir()
	validShellPath := validShellDir + "/mysh"
	require.NoError(t, os.WriteFile(validShellPath, []byte("#!/bin/sh\nexec sh \"$@\""), 0o755))

	tests := []struct {
		name      string
		shellEnv  string
		wantShell string
	}{
		{
			name:      "SHELL unset falls back to /bin/sh",
			shellEnv:  "",
			wantShell: "/bin/sh",
		},
		{
			name:      "SHELL=/bin/sh returns /bin/sh",
			shellEnv:  "/bin/sh",
			wantShell: "/bin/sh",
		},
		{
			name:      "relative SHELL path falls back to /bin/sh",
			shellEnv:  "bash",
			wantShell: "/bin/sh",
		},
		{
			name:      "absolute but missing SHELL path falls back to /bin/sh",
			shellEnv:  "/nonexistent/shell/binary",
			wantShell: "/bin/sh",
		},
		{
			name:      "valid absolute SHELL path is used",
			shellEnv:  validShellPath,
			wantShell: validShellPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SHELL", tt.shellEnv)
			got := detectShell()
			assert.Equal(t, tt.wantShell, got)
		})
	}
}

func TestWithShell_OverridesDetection(t *testing.T) {
	tests := []struct {
		name          string
		shellEnv      string
		opts          []ShellExecutorOption
		wantShellPath string
	}{
		{
			name:          "WithShell sets path when SHELL is empty",
			shellEnv:      "",
			opts:          []ShellExecutorOption{WithShell("/bin/sh")},
			wantShellPath: "/bin/sh",
		},
		{
			name:          "WithShell overrides detected SHELL",
			shellEnv:      "/bin/sh",
			opts:          []ShellExecutorOption{WithShell("/custom/shell")},
			wantShellPath: "/custom/shell",
		},
		{
			name:          "no opts with empty SHELL falls back to /bin/sh",
			shellEnv:      "",
			opts:          nil,
			wantShellPath: "/bin/sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SHELL", tt.shellEnv)
			e := NewShellExecutor(tt.opts...)
			assert.Equal(t, tt.wantShellPath, e.shellPath)
		})
	}
}

func TestShellExecutor_Execute_BashSyntax(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found in PATH, skipping bash-specific syntax test")
	}

	e := NewShellExecutor(WithShell(bashPath))
	cmd := ports.Command{Program: "arr=(one two three); echo ${arr[1]}"}

	result, execErr := e.Execute(context.Background(), &cmd)

	require.NoError(t, execErr)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "two\n", result.Stdout)
}

// TestShellExecutor_MaskSecrets_WithStreaming tests masking works with streaming output
// Feature: C011 - Task T018
func TestShellExecutor_MaskSecrets_WithStreaming(t *testing.T) {
	executor := NewShellExecutor()

	var streamBuf bytes.Buffer
	cmd := ports.Command{
		Program: "echo $SECRET_TOKEN",
		Env: map[string]string{
			"SECRET_TOKEN": "secret123",
		},
		Stdout: &streamBuf,
	}

	result, err := executor.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	// Captured result should be masked
	assert.Equal(t, "***\n", result.Stdout, "captured stdout should mask secret")
	// Note: Streamed output is NOT masked (masking happens after execution)
	// This is expected behavior - streaming shows raw output, final result is masked
	assert.Equal(t, "secret123\n", streamBuf.String(), "streamed output is raw (not masked)")
}
