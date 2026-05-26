package agents

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mcpProxyNameRE matches the unique registration name format: awf-proxy-<16 hex chars>.
var mcpProxyNameRE = regexp.MustCompile(`^awf-proxy-[0-9a-f]{16}$`)

// trackingCommandExecutor records every command in order, enabling name-consistency checks.
type trackingCommandExecutor struct {
	commands []*ports.Command
	err      error
}

func (t *trackingCommandExecutor) Execute(_ context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	t.commands = append(t.commands, cmd)
	if t.err != nil {
		return nil, t.err
	}
	return &ports.CommandResult{Stdout: "", Stderr: "", ExitCode: 0}, nil
}

// testCommandExecutor captures Execute calls and optionally returns a fixed error.
// Shared across Gemini and (formerly) OpenCode MCP tests.
type testCommandExecutor struct {
	executeCallCount int
	executeError     error
	lastCommand      *ports.Command
}

func (m *testCommandExecutor) Execute(_ context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	m.executeCallCount++
	m.lastCommand = cmd
	if m.executeError != nil {
		return nil, m.executeError
	}
	return &ports.CommandResult{Stdout: "", Stderr: "", ExitCode: 0}, nil
}

// TestGeminiMCPInjector_Success tests that gemini mcp add is invoked and cleanup runs mcp remove.
func TestGeminiMCPInjector_Success(t *testing.T) {
	args := []string{"-p", "test prompt"}
	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: false,
	}
	path := "/tmp/mcp-config.json"
	options := map[string]any{"model": "gemini-1.5-pro"}

	mockExec := &testCommandExecutor{}
	provider := NewGeminiProviderWithOptions(func(p *GeminiProvider) {
		p.cmdExecutor = mockExec
	})

	newArgs, newOpts, cleanup, err := provider.geminiMCPInjector(context.Background(), args, cfg, path, options)

	require.NoError(t, err, "geminiMCPInjector should not error")
	require.NotNil(t, cleanup, "cleanup function must not be nil")
	require.NotNil(t, newOpts, "newOptions must not be nil")

	// mcp add should have been invoked once.
	assert.Equal(t, 1, mockExec.executeCallCount, "should invoke CommandExecutor for gemini mcp add")
	require.NotNil(t, mockExec.lastCommand)

	// The registration name must be unique: awf-proxy-<16 hex chars>.
	// interpolation.ShellEscape does not add quotes to simple identifiers (no shell metacharacters).
	addCmdRE := regexp.MustCompile(`^gemini mcp add awf-proxy-[0-9a-f]{16} `)
	assert.Regexp(t, addCmdRE, mockExec.lastCommand.Program,
		"mcp add command must match 'gemini mcp add awf-proxy-<id> ...', got: %q", mockExec.lastCommand.Program)
	assert.True(t, strings.Contains(mockExec.lastCommand.Program, "mcp-serve"),
		"mcp add command should contain mcp-serve subcommand")
	assert.True(t, strings.Contains(mockExec.lastCommand.Program, path),
		"mcp add command should contain config path")

	// Without intercept_builtins no extra flags are added — original args are returned as-is.
	assert.Equal(t, args, newArgs, "args should be unchanged when InterceptBuiltins=false")

	// Gemini does not mutate options.
	assert.Equal(t, options, newOpts, "Gemini must return options unchanged")

	// Cleanup should invoke mcp remove with the same unique name used in mcp add.
	assert.NoError(t, cleanup(), "cleanup should succeed")
	assert.Equal(t, 2, mockExec.executeCallCount, "cleanup should invoke CommandExecutor for gemini mcp remove")
	removeCmdRE := regexp.MustCompile(`^gemini mcp remove awf-proxy-[0-9a-f]{16}$`)
	assert.Regexp(t, removeCmdRE, mockExec.lastCommand.Program,
		"mcp remove command must match 'gemini mcp remove awf-proxy-<id>', got: %q", mockExec.lastCommand.Program)
}

// TestGeminiMCPInjector_InterceptBuiltinsTrue tests that --allowed-mcp-server-names is appended
// when InterceptBuiltins=true, and --policy is appended when denyAllPolicyPath is set.
func TestGeminiMCPInjector_InterceptBuiltinsTrue(t *testing.T) {
	args := []string{"-p", "test prompt"}
	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
	}
	path := "/tmp/mcp-config.json"
	options := map[string]any{"model": "gemini-1.5-pro"}

	mockExec := &testCommandExecutor{}
	provider := NewGeminiProviderWithOptions(
		func(p *GeminiProvider) { p.cmdExecutor = mockExec },
		WithGeminiDenyAllPolicy("/etc/gemini-deny-all.json"),
	)

	newArgs, newOpts, cleanup, err := provider.geminiMCPInjector(context.Background(), args, cfg, path, options)

	require.NoError(t, err)
	require.NotNil(t, cleanup)

	// mcp add invoked.
	assert.Equal(t, 1, mockExec.executeCallCount)

	// With InterceptBuiltins=true and a deny-all policy, two extra flags are appended.
	// Original 2 args + --allowed-mcp-server-names <unique-name> + --policy <path> = 6
	assert.Len(t, newArgs, 6, "args should have 6 elements with intercept_builtins and policy")
	assert.Equal(t, "--allowed-mcp-server-names", newArgs[2])
	assert.Regexp(t, mcpProxyNameRE, newArgs[3],
		"allowed server name must be awf-proxy-<16 hex chars>, got: %q", newArgs[3])
	assert.Equal(t, "--policy", newArgs[4])
	assert.Equal(t, "/etc/gemini-deny-all.json", newArgs[5])

	// Options unchanged.
	assert.Equal(t, options, newOpts)

	assert.NoError(t, cleanup())
}

// TestGeminiMCPInjector_InterceptBuiltinsTrueNoPolicyPath tests --allowed-mcp-server-names
// without --policy when denyAllPolicyPath is empty.
func TestGeminiMCPInjector_InterceptBuiltinsTrueNoPolicyPath(t *testing.T) {
	args := []string{"-p", "test prompt"}
	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
	}
	options := map[string]any{}

	mockExec := &testCommandExecutor{}
	provider := NewGeminiProviderWithOptions(func(p *GeminiProvider) {
		p.cmdExecutor = mockExec
	})

	newArgs, _, cleanup, err := provider.geminiMCPInjector(context.Background(), args, cfg, "/tmp/cfg.json", options)

	require.NoError(t, err)
	require.NotNil(t, cleanup)

	// Only --allowed-mcp-server-names appended; no --policy because denyAllPolicyPath is empty.
	assert.Len(t, newArgs, 4, "args should have 4 elements: original 2 + allowed-mcp-server-names + <unique-name>")
	assert.Equal(t, "--allowed-mcp-server-names", newArgs[2])
	assert.Regexp(t, mcpProxyNameRE, newArgs[3],
		"allowed server name must be awf-proxy-<16 hex chars>, got: %q", newArgs[3])

	assert.NoError(t, cleanup())
}

// TestGeminiMCPInjector_CleanupIdempotency tests that cleanup is idempotent via sync.Once.
func TestGeminiMCPInjector_CleanupIdempotency(t *testing.T) {
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
	mockExec := &testCommandExecutor{}
	provider := NewGeminiProviderWithOptions(func(p *GeminiProvider) {
		p.cmdExecutor = mockExec
	})

	_, _, cleanup, err := provider.geminiMCPInjector(context.Background(), []string{"-p", "x"}, cfg, "/tmp/cfg.json", nil)
	require.NoError(t, err)

	initialCount := mockExec.executeCallCount // 1 (mcp add)

	assert.NoError(t, cleanup(), "first cleanup call should succeed")
	assert.Greater(t, mockExec.executeCallCount, initialCount, "first cleanup should invoke mcp remove")

	removeCount := mockExec.executeCallCount

	// Second call must be a no-op.
	assert.NoError(t, cleanup(), "second cleanup call should succeed")
	assert.Equal(t, removeCount, mockExec.executeCallCount, "second cleanup must not invoke mcp remove again")
}

// TestGeminiMCPInjector_MCPAddFailure tests that an error from mcp add is propagated.
func TestGeminiMCPInjector_MCPAddFailure(t *testing.T) {
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
	mockExec := &testCommandExecutor{executeError: errors.New("gemini not found")}
	provider := NewGeminiProviderWithOptions(func(p *GeminiProvider) {
		p.cmdExecutor = mockExec
	})

	newArgs, _, cleanup, err := provider.geminiMCPInjector(context.Background(), []string{"-p", "x"}, cfg, "/tmp/cfg.json", nil)

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "gemini mcp add"), "error should mention gemini mcp add")
	assert.Nil(t, newArgs, "newArgs should be nil on error")

	// Cleanup should be noop and not error.
	require.NotNil(t, cleanup)
	assert.NoError(t, cleanup())
	// mcp remove must NOT have been called after a failed add.
	assert.Equal(t, 1, mockExec.executeCallCount, "only mcp add should have been attempted")
}

// TestGeminiMCPInjector_NoCmdExecutor tests that missing cmdExecutor returns an error.
func TestGeminiMCPInjector_NoCmdExecutor(t *testing.T) {
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
	provider := NewGeminiProvider() // no cmdExecutor

	newArgs, _, cleanup, err := provider.geminiMCPInjector(context.Background(), []string{"-p", "x"}, cfg, "/tmp/cfg.json", nil)

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "command executor not configured"))
	assert.Nil(t, newArgs)
	require.NotNil(t, cleanup)
	assert.NoError(t, cleanup())
}

// TestGeminiMCPInjector_ConfigNil tests that nil config returns args unchanged without any executor calls.
func TestGeminiMCPInjector_ConfigNil(t *testing.T) {
	originalArgs := []string{"-p", "test prompt"}
	options := map[string]any{"key": "val"}
	mockExec := &testCommandExecutor{}
	provider := NewGeminiProviderWithOptions(func(p *GeminiProvider) {
		p.cmdExecutor = mockExec
	})

	newArgs, newOpts, cleanup, err := provider.geminiMCPInjector(context.Background(), originalArgs, nil, "/tmp/unused", options)

	require.NoError(t, err)
	assert.Equal(t, originalArgs, newArgs)
	assert.Equal(t, options, newOpts)
	assert.Equal(t, 0, mockExec.executeCallCount, "should not invoke executor when config is nil")
	assert.NoError(t, cleanup())
}

// TestGeminiMCPInjector_DoesNotMutateInput verifies original args slice is not modified.
func TestGeminiMCPInjector_DoesNotMutateInput(t *testing.T) {
	originalArgs := []string{"-p", "test prompt"}
	argsCopy := make([]string, len(originalArgs))
	copy(argsCopy, originalArgs)

	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}
	mockExec := &testCommandExecutor{}
	provider := NewGeminiProviderWithOptions(
		func(p *GeminiProvider) { p.cmdExecutor = mockExec },
		WithGeminiDenyAllPolicy("/etc/deny.json"),
	)

	newArgs, _, cleanup, err := provider.geminiMCPInjector(context.Background(), originalArgs, cfg, "/tmp/config.json", map[string]any{})

	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.Equal(t, argsCopy, originalArgs, "original args slice must not be modified")
	assert.Greater(t, len(newArgs), len(originalArgs))
}

// TestGeminiMCPInjector_MCPAddCommandFormat verifies the mcp add command includes
// the awf-proxy name and mcp-serve subcommand with the config path.
func TestGeminiMCPInjector_MCPAddCommandFormat(t *testing.T) {
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
	path := "/tmp/mcp-config.json"
	mockExec := &testCommandExecutor{}
	provider := NewGeminiProviderWithOptions(func(p *GeminiProvider) {
		p.cmdExecutor = mockExec
	})

	_, _, cleanup, err := provider.geminiMCPInjector(context.Background(), nil, cfg, path, nil)

	require.NoError(t, err)
	require.NotNil(t, mockExec.lastCommand)

	prog := mockExec.lastCommand.Program
	// interpolation.ShellEscape does not add quotes to simple identifiers (no metacharacters).
	addCmdRE2 := regexp.MustCompile(`^gemini mcp add awf-proxy-[0-9a-f]{16} `)
	assert.Regexp(t, addCmdRE2, prog,
		"add command must match 'gemini mcp add awf-proxy-<id> <cmd...>', got: %q", prog)
	assert.True(t, strings.Contains(prog, "mcp-serve"), "should contain mcp-serve")
	assert.True(t, strings.Contains(prog, path), "should contain config path")

	assert.NoError(t, cleanup())
}

// TestGeminiMCPInjector_CleanupNameConsistency verifies that the name registered via
// `gemini mcp add` is the exact same name used in `gemini mcp remove`.
// This is the core invariant that prevents orphan registrations: each injector call
// owns exactly one named registration and removes exactly that name.
func TestGeminiMCPInjector_CleanupNameConsistency(t *testing.T) {
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
	trackingExec := &trackingCommandExecutor{}
	provider := NewGeminiProviderWithOptions(func(p *GeminiProvider) {
		p.cmdExecutor = trackingExec
	})

	_, _, cleanup, err := provider.geminiMCPInjector(context.Background(), []string{"-p", "x"}, cfg, "/tmp/cfg.json", nil)
	require.NoError(t, err)

	addCmd := trackingExec.commands[0].Program

	require.NoError(t, cleanup())

	removeCmd := trackingExec.commands[1].Program

	// Extract the unique name from the add command: "gemini mcp add awf-proxy-XXXX ..."
	// interpolation.ShellEscape does not quote simple identifiers without metacharacters.
	addParts := strings.SplitN(addCmd, " ", 5) // ["gemini", "mcp", "add", "<name>", "..."]
	require.Len(t, addParts, 5, "add command should have at least 5 parts")
	name := addParts[3]

	// The remove command should be exactly "gemini mcp remove <same-name>"
	assert.Equal(t, "gemini mcp remove "+name, removeCmd,
		"cleanup must remove the same name that was registered")
	assert.Regexp(t, mcpProxyNameRE, name,
		"registered name must match awf-proxy-<16 hex chars> pattern")
}

// TestGeminiMCPInjector_PolicyFallbackOption tests WithGeminiCommandExecutor option is wired correctly.
func TestGeminiMCPInjector_PolicyFallbackOption(t *testing.T) {
	mockExec := &testCommandExecutor{}
	provider := NewGeminiProviderWithOptions(
		WithGeminiCommandExecutor(mockExec),
		WithGeminiDenyAllPolicy("/etc/deny.json"),
	)

	require.NotNil(t, provider, "provider with options should be created successfully")
	assert.Equal(t, mockExec, provider.cmdExecutor, "cmdExecutor should be wired via WithGeminiCommandExecutor")
	assert.Equal(t, "/etc/deny.json", provider.denyAllPolicyPath)
}
