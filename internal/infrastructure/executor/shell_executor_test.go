package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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

func TestShellExecutor_Execute_WithoutDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell synchronization fixture requires POSIX named pipes")
	}

	tmpDir := t.TempDir()
	fifoPath := filepath.Join(tmpDir, "release")
	readyPath := filepath.Join(tmpDir, "ready")
	require.NoError(t, syscall.Mkfifo(fifoPath, 0o600))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, hasDeadline := ctx.Deadline()
	require.False(t, hasDeadline)

	resultCh := make(chan shellExecution, 1)
	go func() {
		result, err := NewShellExecutor().Execute(ctx, &ports.Command{
			Program: fmt.Sprintf("touch %q; read release < %q; printf completed", readyPath, fifoPath),
		})
		resultCh <- shellExecution{result: result, err: err}
	}()
	completed := false
	t.Cleanup(func() {
		cancel()
		if completed {
			return
		}
		select {
		case <-resultCh:
		case <-time.After(2 * time.Second):
			t.Errorf("executor did not stop during cleanup")
		}
	})

	require.Eventually(t, func() bool {
		_, err := os.Stat(readyPath)
		return err == nil
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, releaseFIFO(ctx, fifoPath, time.Second))

	var execution shellExecution
	select {
	case execution = <-resultCh:
		completed = true
	case <-time.After(2 * time.Second):
		t.Fatal("executor did not return after the FIFO was released")
	}
	require.NoError(t, execution.err)
	require.NotNil(t, execution.result)
	assert.Equal(t, "completed", execution.result.Stdout)
	_, hasDeadline = ctx.Deadline()
	assert.False(t, hasDeadline)
}

func releaseFIFO(ctx context.Context, path string, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			return syscall.Close(fd)
		}
		if !errors.Is(err, syscall.ENXIO) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("timed out releasing FIFO %q", path)
		case <-ticker.C:
		}
	}
}

func TestShellExecutor_Execute_ContextTimeout(t *testing.T) {
	testShellExecutorContextTermination(t, true)
}

func TestShellExecutor_Execute_ContextCancellation(t *testing.T) {
	testShellExecutorContextTermination(t, false)
}

type shellExecution struct {
	result *ports.CommandResult
	err    error
}

func testShellExecutorContextTermination(t *testing.T, useTimeout bool) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("process-group lifecycle assertions require POSIX signals")
	}

	const (
		privateCommandValue = "PRIVATE_COMMAND_VALUE"
		privateEnvValue     = "PRIVATE_ENVIRONMENT_VALUE"
		privateOutputValue  = "PRIVATE_TOOL_OUTPUT"
		privatePromptValue  = "PRIVATE_PROMPT"
		privateRequestValue = "PRIVATE_REQUEST_BODY"
		privateHeaderValue  = "PRIVATE_AUTHORIZATION_HEADER"
		privatePluginValue  = "PRIVATE_PLUGIN_CONFIGURATION"
		privateTokenValue   = "PRIVATE_TOKEN"
		privateCredential   = "PRIVATE_CREDENTIAL"
	)
	tmpDir := t.TempDir()
	readyPath := filepath.Join(tmpDir, "ready")
	pidPath := filepath.Join(tmpDir, "child.pid")
	cmd := &ports.Command{
		Program: fmt.Sprintf(
			"printf '%%s' %q \"$SECRET_EXECUTOR_VALUE\" \"$SECRET_PROMPT\" \"$SECRET_REQUEST\" \"$SECRET_HEADER\" \"$SECRET_PLUGIN\" \"$SECRET_TOKEN\" \"$SECRET_CREDENTIAL\" >/dev/null; printf %q; sleep 30 & child=$!; printf '%%s' \"$child\" > %q; touch %q; wait \"$child\"",
			privateCommandValue,
			privateOutputValue,
			pidPath,
			readyPath,
		),
		Env: map[string]string{
			"SECRET_EXECUTOR_VALUE": privateEnvValue,
			"SECRET_PROMPT":         privatePromptValue,
			"SECRET_REQUEST":        privateRequestValue,
			"SECRET_HEADER":         privateHeaderValue,
			"SECRET_PLUGIN":         privatePluginValue,
			"SECRET_TOKEN":          privateTokenValue,
			"SECRET_CREDENTIAL":     privateCredential,
		},
	}

	var (
		ctx     context.Context
		cancel  context.CancelFunc
		wantErr error
	)
	if useTimeout {
		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		wantErr = context.DeadlineExceeded
	} else {
		ctx, cancel = context.WithCancel(context.Background())
		wantErr = context.Canceled
	}
	defer cancel()

	resultCh := make(chan shellExecution, 1)
	go func() {
		result, err := NewShellExecutor().Execute(ctx, cmd)
		resultCh <- shellExecution{result: result, err: err}
	}()

	require.Eventually(t, func() bool {
		_, err := os.Stat(readyPath)
		return err == nil
	}, 2*time.Second, 10*time.Millisecond, "child process did not reach the synchronized blocker")

	pidBytes, err := os.ReadFile(pidPath)
	require.NoError(t, err)
	childPID, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	require.NoError(t, err)
	if !useTimeout {
		cancel()
	}

	var execution shellExecution
	select {
	case execution = <-resultCh:
	case <-time.After(2 * time.Second):
		t.Fatal("executor did not return after context termination")
	}

	require.Error(t, execution.err)
	assert.ErrorIs(t, execution.err, wantErr)
	assert.NotNil(t, execution.result)
	for _, sensitive := range []string{
		privateCommandValue,
		privateEnvValue,
		privateOutputValue,
		privatePromptValue,
		privateRequestValue,
		privateHeaderValue,
		privatePluginValue,
		privateTokenValue,
		privateCredential,
	} {
		assert.NotContains(t, execution.err.Error(), sensitive)
	}
	assert.Eventually(t, func() bool {
		err := syscall.Kill(childPID, 0)
		return errors.Is(err, syscall.ESRCH)
	}, time.Second, 10*time.Millisecond, "descendant process %d survived context termination", childPID)
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
