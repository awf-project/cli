package executor

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasShebang(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"#!/bin/sh\necho hello", true},
		{"#!/usr/bin/env python3\nprint('hi')", true},
		{"#!/bin/bash", true},
		{"echo hello", false},
		{"", false},
		{"# comment\necho hi", false},
		{" #!/bin/sh", false}, // leading space — not a shebang
	}

	for _, tt := range tests {
		t.Run(tt.content[:min(len(tt.content), 20)], func(t *testing.T) {
			assert.Equal(t, tt.want, hasShebang(tt.content))
		})
	}
}

func TestShellExecutor_ScriptFile_ShebangPython(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found")
	}

	e := NewShellExecutor()
	cmd := ports.Command{
		Program:      "#!/usr/bin/env python3\nprint('hello-from-python')\n",
		IsScriptFile: true,
	}

	result, err := e.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "hello-from-python")
}

func TestShellExecutor_ScriptFile_ShebangBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not found")
	}

	e := NewShellExecutor()
	cmd := ports.Command{
		Program:      "#!/bin/bash\necho hello-from-bash\n",
		IsScriptFile: true,
	}

	result, err := e.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "hello-from-bash")
}

func TestShellExecutor_ScriptFile_NoShebangFallback(t *testing.T) {
	e := NewShellExecutor(WithShell("/bin/sh"))
	cmd := ports.Command{
		Program:      "echo no-shebang\n",
		IsScriptFile: true,
	}

	result, err := e.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "no-shebang")
}

func TestShellExecutor_ScriptFile_NonScriptUnchanged(t *testing.T) {
	e := NewShellExecutor()
	cmd := ports.Command{
		Program:      "echo not-a-script",
		IsScriptFile: false,
	}

	result, err := e.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "not-a-script")
}

func TestShellExecutor_Execute_NotScriptFile_DoesNotUseShebangPath(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found — needed to distinguish shell vs shebang execution")
	}

	e := NewShellExecutor()
	cmd := ports.Command{
		Program:      "#!/usr/bin/env python3\nprint('hello')\n",
		IsScriptFile: false, // must NOT use shebang path
	}

	result, err := e.Execute(context.Background(), &cmd)

	// shell -c treats #! as a comment, then fails on print('hello') (syntax error in sh)
	require.NoError(t, err) // executor doesn't surface non-zero exit as error
	assert.NotEqual(t, 0, result.ExitCode, "shell should fail to interpret Python syntax")
}

func TestShellExecutor_ScriptFile_TempFileCleanup(t *testing.T) {
	// Count existing awf-script-* files before execution to avoid false positives
	// from parallel test runs.
	countScriptFiles := func() int {
		entries, err := os.ReadDir(os.TempDir())
		if err != nil {
			return 0
		}
		count := 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "awf-script-") {
				count++
			}
		}
		return count
	}

	before := countScriptFiles()

	e := NewShellExecutor()
	cmd := ports.Command{
		Program:      "#!/bin/sh\necho cleanup-test\n",
		IsScriptFile: true,
	}

	result, err := e.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "cleanup-test")

	after := countScriptFiles()
	assert.LessOrEqual(t, after, before, "Execute must not leave awf-script-* temp files behind")
}

func TestShellExecutor_ScriptFile_ContextCancellation(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not found")
	}

	e := NewShellExecutor()
	cmd := ports.Command{
		Program:      "#!/bin/bash\nsleep 30\n",
		IsScriptFile: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	result, err := e.Execute(ctx, &cmd)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 3*time.Second, "cancellation should stop execution quickly")
	assert.NotNil(t, result)
	// Either the context error is surfaced or the exit code is non-zero.
	if err == nil {
		assert.NotEqual(t, 0, result.ExitCode, "cancelled script should not exit 0")
	} else {
		assert.Error(t, err)
	}
}

func TestShellExecutor_ScriptFile_EnvPropagation(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not found")
	}

	e := NewShellExecutor()
	cmd := ports.Command{
		Program:      "#!/bin/bash\necho $MY_TEST_VAR\n",
		IsScriptFile: true,
		Env:          map[string]string{"MY_TEST_VAR": "propagated"},
	}

	result, err := e.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "propagated")
}

func TestShellExecutor_ScriptFile_WorkingDirectory(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not found")
	}

	tmpDir := t.TempDir()
	e := NewShellExecutor()
	cmd := ports.Command{
		Program:      "#!/bin/bash\npwd\n",
		IsScriptFile: true,
		Dir:          tmpDir,
	}

	result, err := e.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, tmpDir)
}

func TestShellExecutor_ScriptFile_SecretMasking(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not found")
	}

	e := NewShellExecutor()
	cmd := ports.Command{
		Program:      "#!/bin/bash\necho $SECRET_KEY\n",
		IsScriptFile: true,
		Env:          map[string]string{"SECRET_KEY": "supersecret"},
	}

	result, err := e.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.NotContains(t, result.Stdout, "supersecret", "secret value must be masked in output")
	assert.Equal(t, "***\n", result.Stdout)
}

func TestShellExecutor_Execute_ScriptFile_WithShebang_NonZeroExit(t *testing.T) {
	e := NewShellExecutor()
	cmd := ports.Command{
		Program:      "#!/bin/sh\nexit 42\n",
		IsScriptFile: true,
	}

	result, err := e.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, 42, result.ExitCode)
}

func TestShellExecutor_Execute_ScriptFile_WithShebang_CapturesOutput(t *testing.T) {
	e := NewShellExecutor()
	cmd := ports.Command{
		Program:      "#!/bin/sh\necho stdout-line\necho stderr-line >&2\n",
		IsScriptFile: true,
	}

	result, err := e.Execute(context.Background(), &cmd)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "stdout-line\n", result.Stdout)
	assert.Equal(t, "stderr-line\n", result.Stderr)
}
