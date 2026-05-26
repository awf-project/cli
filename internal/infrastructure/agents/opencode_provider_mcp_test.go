package agents

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// opencodeMCPNameRE matches the unique registration name format: awf-proxy-<16 hex chars>.
var opencodeMCPNameRE = regexp.MustCompile(`^awf-proxy-[0-9a-f]{16}$`)

// chdir changes the process working directory to dir for the duration of the test,
// restoring the original directory via t.Cleanup. This is required because
// opencodeMCPInjector calls os.Getwd() to locate the workspace.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})
}

// TestOpencodeMCPInjector_Success verifies the injector writes opencode.json
// and that cleanup removes the entry + deletes the file (fresh workspace).
func TestOpencodeMCPInjector_Success(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	args := []string{"run", "prompt"}
	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
	}
	path := "/tmp/mcp-config.json"
	options := map[string]any{}

	mockLog := &testLogCapture{}
	provider := NewOpenCodeProviderWithOptions(func(p *OpenCodeProvider) {
		p.logger = mockLog
	})

	newArgs, newOpts, cleanup, err := provider.opencodeMCPInjector(context.Background(), args, cfg, path, options)

	require.NoError(t, err, "opencodeMCPInjector should not error")
	require.NotNil(t, cleanup, "cleanup function must not be nil")
	require.NotNil(t, newOpts, "newOptions must not be nil")

	// opencode.json must exist with our entry.
	configPath := filepath.Join(dir, "opencode.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err, "opencode.json must exist after injector call")

	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &top))
	require.Contains(t, top, "mcp")

	var mcpMap map[string]opencodeMCPEntry
	require.NoError(t, json.Unmarshal(top["mcp"], &mcpMap))

	// Find the entry whose key matches the awf-proxy- prefix.
	var foundEntry bool
	for k, v := range mcpMap {
		if !strings.HasPrefix(k, mcpProxyNamePrefix) {
			continue
		}
		foundEntry = true
		assert.Regexp(t, opencodeMCPNameRE, k, "server name must match awf-proxy-<16 hex> pattern")
		assert.Equal(t, "local", v.Type)
		assert.True(t, v.Enabled)
		assert.NotEmpty(t, v.Command)
	}
	assert.True(t, foundEntry, "at least one awf-proxy-* entry must be present")

	// Args must be unchanged — no --mcp-config appended.
	assert.Equal(t, args, newArgs, "new args should be unchanged (no --mcp-config appended)")

	// WARN log for intercept_builtins.
	assert.Len(t, mockLog.warnCalls, 1, "should emit one WARN log")
	assert.True(t, strings.Contains(mockLog.warnCalls[0].msg, "coexistence mode"), "WARN message should mention coexistence mode")

	// Cleanup removes our entry and deletes the file (fresh workspace).
	require.NoError(t, cleanup(), "cleanup should succeed")
	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr), "opencode.json must be deleted after cleanup on a fresh workspace")
}

// TestOpencodeMCPInjector_CleanupIdempotency tests cleanup is idempotent via sync.Once.
func TestOpencodeMCPInjector_CleanupIdempotency(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	args := []string{"run", "prompt"}
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
	options := map[string]any{}

	provider := NewOpenCodeProvider()

	_, _, cleanup, err := provider.opencodeMCPInjector(context.Background(), args, cfg, "/tmp/cfg.json", options)
	require.NoError(t, err)
	require.NotNil(t, cleanup)

	require.NoError(t, cleanup(), "first cleanup should succeed")
	require.NoError(t, cleanup(), "second cleanup must be no-op and return nil")
}

// TestOpencodeMCPInjector_InterceptBuiltinsFalse verifies that without
// intercept_builtins the WARN log is not emitted and system_prompt is not mutated,
// but the file is still written.
func TestOpencodeMCPInjector_InterceptBuiltinsFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	args := []string{"run", "prompt"}
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
	options := map[string]any{"system_prompt": "original"}

	mockLog := &testLogCapture{}
	provider := NewOpenCodeProviderWithOptions(func(p *OpenCodeProvider) {
		p.logger = mockLog
	})

	_, newOpts, cleanup, err := provider.opencodeMCPInjector(context.Background(), args, cfg, "/tmp/cfg.json", options)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	require.NotNil(t, newOpts)

	// opencode.json must still be written.
	_, statErr := os.Stat(filepath.Join(dir, "opencode.json"))
	assert.NoError(t, statErr, "opencode.json must be written even when InterceptBuiltins=false")

	// WARN log must NOT be emitted.
	assert.Len(t, mockLog.warnCalls, 0, "should NOT emit WARN log when InterceptBuiltins=false")

	// system_prompt must NOT be mutated.
	assert.Equal(t, "original", newOpts["system_prompt"], "system_prompt should be unchanged when InterceptBuiltins=false")

	require.NoError(t, cleanup())
}

// TestOpencodeMCPInjector_ConfigNil tests nil config — no file written, args/options unchanged.
func TestOpencodeMCPInjector_ConfigNil(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	args := []string{"run", "prompt"}
	options := map[string]any{}
	provider := NewOpenCodeProvider()

	newArgs, newOpts, cleanup, err := provider.opencodeMCPInjector(context.Background(), args, nil, "/tmp/unused", options)

	require.NoError(t, err)
	assert.Equal(t, args, newArgs, "args should be unchanged when config is nil")
	assert.Equal(t, options, newOpts, "options should be unchanged when config is nil")

	// No file must be created.
	_, statErr := os.Stat(filepath.Join(dir, "opencode.json"))
	assert.True(t, os.IsNotExist(statErr), "opencode.json must not exist when config is nil")

	assert.NoError(t, cleanup())
}

// TestOpencodeMCPInjector_SystemPromptMutation verifies system_prompt mutation when InterceptBuiltins=true.
func TestOpencodeMCPInjector_SystemPromptMutation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	args := []string{"run", "prompt"}
	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}
	options := map[string]any{"system_prompt": "Original system prompt"}

	provider := NewOpenCodeProvider()

	_, newOpts, cleanup, err := provider.opencodeMCPInjector(context.Background(), args, cfg, "/tmp/cfg.json", options)
	require.NoError(t, err)
	require.NoError(t, cleanup())
	require.NotNil(t, newOpts)

	modifiedPrompt, ok := newOpts["system_prompt"].(string)
	require.True(t, ok, "system_prompt should be a string")
	assert.True(t, strings.HasPrefix(modifiedPrompt, "Use only MCP tools, never built-in tools. "),
		"system_prompt should start with MCP-only instruction, got: %q", modifiedPrompt)
	assert.Contains(t, modifiedPrompt, "Original system prompt",
		"original system_prompt content should be preserved")

	// Original options map must NOT be mutated.
	assert.Equal(t, "Original system prompt", options["system_prompt"],
		"original options map must not be mutated")
}

// TestOpencodeMCPInjector_SystemPromptMutation_NoExisting tests mutation when system_prompt is absent.
func TestOpencodeMCPInjector_SystemPromptMutation_NoExisting(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}
	options := map[string]any{}

	provider := NewOpenCodeProvider()

	_, newOpts, cleanup, err := provider.opencodeMCPInjector(context.Background(), []string{"run"}, cfg, "/tmp/cfg.json", options)
	require.NoError(t, err)
	require.NoError(t, cleanup())
	require.NotNil(t, newOpts)

	modifiedPrompt, ok := newOpts["system_prompt"].(string)
	require.True(t, ok, "system_prompt should be a string")
	assert.Equal(t, "Use only MCP tools, never built-in tools. ", modifiedPrompt,
		"should create system_prompt with MCP-only instruction when none exists")
}

// TestOpencodeMCPInjector_CleanupNameConsistency verifies the name written to
// opencode.json is consistent and matches the expected pattern.
func TestOpencodeMCPInjector_CleanupNameConsistency(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
	provider := NewOpenCodeProvider()

	_, _, cleanup, err := provider.opencodeMCPInjector(context.Background(), []string{"run"}, cfg, "/tmp/cfg.json", nil)
	require.NoError(t, err)

	// Read the file to capture the written name.
	configPath := filepath.Join(dir, "opencode.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &top))
	var mcpMap map[string]opencodeMCPEntry
	require.NoError(t, json.Unmarshal(top["mcp"], &mcpMap))

	var registeredName string
	for k := range mcpMap {
		if strings.HasPrefix(k, mcpProxyNamePrefix) {
			registeredName = k
			break
		}
	}
	require.NotEmpty(t, registeredName, "a registered name must be found")
	assert.Regexp(t, opencodeMCPNameRE, registeredName, "registered name must match awf-proxy-<16 hex chars> pattern")

	// After cleanup the file should be gone (fresh workspace).
	require.NoError(t, cleanup())
	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr), "opencode.json must be removed by cleanup")
}

// TestOpencodeMCPInjector_NoShellOutToMCPAdd verifies the injector does NOT call
// `opencode mcp add` via shell — the registration is purely file-based.
// This test catches regression if someone reintroduces cmdExecutor-based add.
func TestOpencodeMCPInjector_NoShellOutToMCPAdd(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cfg := &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: false}
	// Provider has no cmdExecutor — if the code called it, it would panic or return error.
	provider := NewOpenCodeProvider()

	_, _, cleanup, err := provider.opencodeMCPInjector(context.Background(), []string{"run"}, cfg, "/tmp/cfg.json", nil)
	require.NoError(t, err, "injector must succeed without a cmdExecutor (file-based, no shell-out)")
	require.NoError(t, cleanup())
}
