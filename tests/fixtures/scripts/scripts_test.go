package scripts_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixtureDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller should succeed")
	return filepath.Dir(file)
}

func TestFixtureScripts_AllExecutable(t *testing.T) {
	dir := fixtureDir(t)
	scripts := []string{
		"shebang_python.py",
		"shebang_bash.sh",
		"no_shebang.sh",
	}
	for _, name := range scripts {
		info, err := os.Stat(filepath.Join(dir, name))
		require.NoError(t, err, "%s must exist", name)
		assert.NotZero(t, info.Mode()&0o111, "%s must have executable bit set", name)
	}
}

func TestFixtureScripts_ShebangPython_HasShebang(t *testing.T) {
	dir := fixtureDir(t)
	content, err := os.ReadFile(filepath.Join(dir, "shebang_python.py"))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(content), "#!/usr/bin/env python3"),
		"shebang_python.py must start with #!/usr/bin/env python3")
}

func TestFixtureScripts_ShebangBash_HasShebang(t *testing.T) {
	dir := fixtureDir(t)
	content, err := os.ReadFile(filepath.Join(dir, "shebang_bash.sh"))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(content), "#!/bin/bash"),
		"shebang_bash.sh must start with #!/bin/bash")
}

func TestFixtureScripts_NoShebang_HasNoShebang(t *testing.T) {
	dir := fixtureDir(t)
	content, err := os.ReadFile(filepath.Join(dir, "no_shebang.sh"))
	require.NoError(t, err)
	assert.False(t, strings.HasPrefix(string(content), "#!"),
		"no_shebang.sh must not start with a shebang line")
}

func TestFixtureScripts_NoShebang_ProducesExpectedOutput(t *testing.T) {
	dir := fixtureDir(t)
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	content, err := os.ReadFile(filepath.Join(dir, "no_shebang.sh"))
	require.NoError(t, err)
	out, err := exec.CommandContext(context.Background(), shell, "-c", string(content)).Output() //nolint:gosec // shell and content are controlled test fixtures
	require.NoError(t, err, "no_shebang.sh content must execute via $SHELL -c without error")
	assert.Contains(t, string(out), "no-shebang-executed",
		"no_shebang.sh must produce expected output when run via $SHELL -c")
}
