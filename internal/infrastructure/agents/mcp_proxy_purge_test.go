package agents

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// purgeLogCapture captures all log levels for purge-specific tests.
type purgeLogCapture struct {
	debugCalls []string
	infoCalls  []string
	warnCalls  []string
}

func (l *purgeLogCapture) Debug(msg string, _ ...any) {
	l.debugCalls = append(l.debugCalls, msg)
}

func (l *purgeLogCapture) Info(msg string, _ ...any) {
	l.infoCalls = append(l.infoCalls, msg)
}

func (l *purgeLogCapture) Warn(msg string, _ ...any) {
	l.warnCalls = append(l.warnCalls, msg)
}

func (l *purgeLogCapture) Error(_ string, _ ...any)                  {}
func (l *purgeLogCapture) WithContext(_ map[string]any) ports.Logger { return l }

// purgeTrackingExecutor records commands and lets callers control responses per command index.
type purgeTrackingExecutor struct {
	commands []*ports.Command
	// responseFor maps the zero-based call index to a custom response.
	// Unspecified indices return empty stdout and nil error.
	responseFor map[int]purgeResponse
}

type purgeResponse struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func (e *purgeTrackingExecutor) Execute(_ context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	idx := len(e.commands)
	e.commands = append(e.commands, cmd)
	if resp, ok := e.responseFor[idx]; ok {
		if resp.err != nil {
			return nil, resp.err
		}
		return &ports.CommandResult{Stdout: resp.stdout, Stderr: resp.stderr, ExitCode: resp.exitCode}, nil
	}
	return &ports.CommandResult{Stdout: "", Stderr: "", ExitCode: 0}, nil
}

// commandPrograms returns the Program field of every recorded command.
func (e *purgeTrackingExecutor) commandPrograms() []string {
	progs := make([]string, len(e.commands))
	for i, c := range e.commands {
		progs[i] = c.Program
	}
	return progs
}

// TestPurgeOrphanMCPRegistrations_NoCLIs verifies that when both `gemini mcp list`
// and `opencode mcp list` fail (e.g. CLI not installed), the function returns nil,
// logs at debug level, and does not panic.
func TestPurgeOrphanMCPRegistrations_NoCLIs(t *testing.T) {
	listErr := errors.New("binary not found")
	exec := &purgeTrackingExecutor{
		responseFor: map[int]purgeResponse{
			0: {err: listErr}, // gemini mcp list
			1: {err: listErr}, // opencode mcp list
		},
	}
	log := &purgeLogCapture{}

	err := PurgeOrphanMCPRegistrations(context.Background(), exec, log)

	require.NoError(t, err, "PurgeOrphanMCPRegistrations must return nil even when CLIs are absent")
	assert.Len(t, exec.commands, 2, "should attempt exactly 2 list commands (one per CLI)")
	assert.NotEmpty(t, log.debugCalls, "should log debug messages when list fails")
	// No remove commands issued.
	for _, prog := range exec.commandPrograms() {
		assert.NotContains(t, prog, "remove", "remove must not be called when list fails")
	}
}

// TestPurgeOrphanMCPRegistrations_PurgesOnlyMatchingPrefix verifies that only
// entries whose name starts with mcpProxyNamePrefix are removed, and non-matching
// entries are left untouched.
func TestPurgeOrphanMCPRegistrations_PurgesOnlyMatchingPrefix(t *testing.T) {
	// Simulate gemini mcp list returning two matching entries and one non-matching.
	geminiListOutput := `
✓ awf-proxy-aaaa1111aaaa1111: /bin/awf mcp-serve --config /tmp/a.json (stdio) - connected
✓ awf-proxy-bbbb2222bbbb2222: /bin/awf mcp-serve --config /tmp/b.json (stdio) - connected
✓ user-server: /usr/bin/myserver (stdio) - connected
`
	exec := &purgeTrackingExecutor{
		responseFor: map[int]purgeResponse{
			0: {stdout: geminiListOutput}, // gemini mcp list
			// calls 1,2 are gemini mcp remove (two matching entries)
			3: {stdout: ""}, // opencode mcp list — empty
		},
	}
	log := &purgeLogCapture{}

	err := PurgeOrphanMCPRegistrations(context.Background(), exec, log)

	require.NoError(t, err)

	progs := exec.commandPrograms()
	// Expect: gemini list, gemini remove awf-proxy-aaaa..., gemini remove awf-proxy-bbbb..., opencode list
	require.Len(t, progs, 4, "expected list+2 removes+list: %v", progs)
	assert.Equal(t, "gemini mcp list", progs[0])
	// interpolation.ShellEscape does not add quotes to simple identifiers (no shell metacharacters),
	// so the server name appears unquoted. Characters such as spaces or quotes would cause quoting.
	assert.Equal(t, "gemini mcp remove awf-proxy-aaaa1111aaaa1111", progs[1])
	assert.Equal(t, "gemini mcp remove awf-proxy-bbbb2222bbbb2222", progs[2])
	assert.Equal(t, "opencode mcp list", progs[3])

	// Verify user-server was never mentioned in any remove command.
	for _, prog := range progs {
		assert.NotContains(t, prog, "user-server", "user-server must not be removed")
	}

	// Two info log entries (one per removed server).
	assert.Len(t, log.infoCalls, 2, "should emit one info log per removed orphan")
}

// TestResolveListTimeout verifies env-var parsing for the list timeout override.
func TestResolveListTimeout(t *testing.T) {
	log := &purgeLogCapture{}

	t.Run("default when unset", func(t *testing.T) {
		t.Setenv(mcpListTimeoutEnv, "")
		assert.Equal(t, mcpListDefaultTimeout, resolveListTimeout(log))
	})
	t.Run("valid override", func(t *testing.T) {
		t.Setenv(mcpListTimeoutEnv, "12s")
		assert.Equal(t, 12*time.Second, resolveListTimeout(log))
	})
	t.Run("falls back on unparseable value", func(t *testing.T) {
		t.Setenv(mcpListTimeoutEnv, "not-a-duration")
		assert.Equal(t, mcpListDefaultTimeout, resolveListTimeout(log))
	})
	t.Run("falls back on non-positive value", func(t *testing.T) {
		t.Setenv(mcpListTimeoutEnv, "0s")
		assert.Equal(t, mcpListDefaultTimeout, resolveListTimeout(log))
	})
}

// TestPurgeOrphanMCPRegistrations_TimeoutLogsWarn verifies that a `mcp list`
// timeout (context deadline) is surfaced at WARN — distinct from "not installed" —
// and does not trigger any remove, since the CLI is installed but unresponsive.
func TestPurgeOrphanMCPRegistrations_TimeoutLogsWarn(t *testing.T) {
	// Mirror the shell executor, which wraps ctx.Err() as "command execution: %w".
	timeoutErr := fmt.Errorf("command execution: %w", context.DeadlineExceeded)
	exec := &purgeTrackingExecutor{
		responseFor: map[int]purgeResponse{
			0: {err: timeoutErr}, // gemini mcp list times out
			1: {err: timeoutErr}, // opencode mcp list times out
		},
	}
	log := &purgeLogCapture{}

	err := PurgeOrphanMCPRegistrations(context.Background(), exec, log)

	require.NoError(t, err, "timeout must not propagate as an error")
	assert.Len(t, log.warnCalls, 2, "each timed-out CLI must log exactly one warning")
	assert.Empty(t, log.infoCalls, "no orphan should be purged on timeout")
	for _, prog := range exec.commandPrograms() {
		assert.NotContains(t, prog, "remove", "remove must not run when list times out")
	}
}

// TestPurgeOrphanMCPRegistrations_NotInstalledIsQuiet verifies that an exit code
// 127 (binary not found via the shell) is treated as "not installed": logged at
// debug, no warning, no remove.
func TestPurgeOrphanMCPRegistrations_NotInstalledIsQuiet(t *testing.T) {
	exec := &purgeTrackingExecutor{
		responseFor: map[int]purgeResponse{
			0: {exitCode: 127, stderr: "gemini: command not found"},   // gemini not installed
			1: {exitCode: 127, stderr: "opencode: command not found"}, // opencode not installed
		},
	}
	log := &purgeLogCapture{}

	err := PurgeOrphanMCPRegistrations(context.Background(), exec, log)

	require.NoError(t, err)
	assert.Empty(t, log.warnCalls, "a missing CLI must not warn — it is an expected, quiet case")
	assert.NotEmpty(t, log.debugCalls, "a missing CLI should log at debug level")
	assert.Len(t, exec.commands, 2, "exactly one list attempt per CLI, no removes")
	for _, prog := range exec.commandPrograms() {
		assert.NotContains(t, prog, "remove", "remove must not run when the CLI is absent")
	}
}

// TestPurgeOrphanMCPRegistrations_RespectsEnvOptOut verifies that when
// AWF_MCP_PROXY_NO_PURGE is set to any non-empty value, no commands are executed.
func TestPurgeOrphanMCPRegistrations_RespectsEnvOptOut(t *testing.T) {
	t.Setenv("AWF_MCP_PROXY_NO_PURGE", "1")

	exec := &purgeTrackingExecutor{}
	log := &purgeLogCapture{}

	err := PurgeOrphanMCPRegistrations(context.Background(), exec, log)

	require.NoError(t, err, "must return nil when opt-out env var is set")
	assert.Empty(t, exec.commands, "no commands must be executed when opt-out is active")
	assert.NotEmpty(t, log.debugCalls, "should log a debug message explaining the skip")
}

// TestPurgeOrphanMCPRegistrations_RemovalFailureIsNonFatal verifies that a failure
// during `mcp remove` does not cause PurgeOrphanMCPRegistrations to return an error.
func TestPurgeOrphanMCPRegistrations_RemovalFailureIsNonFatal(t *testing.T) {
	geminiListOutput := "✓ awf-proxy-dead1234dead1234: /bin/awf mcp-serve (stdio) - connected\n"
	exec := &purgeTrackingExecutor{
		responseFor: map[int]purgeResponse{
			0: {stdout: geminiListOutput},             // gemini mcp list
			1: {err: errors.New("permission denied")}, // gemini mcp remove — fails
			2: {stdout: ""},                           // opencode mcp list
		},
	}
	log := &purgeLogCapture{}

	err := PurgeOrphanMCPRegistrations(context.Background(), exec, log)

	require.NoError(t, err, "removal failure must not propagate as an error")
	assert.NotEmpty(t, log.debugCalls, "failure should be logged at debug level")
}
