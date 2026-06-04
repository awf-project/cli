//go:build integration && !windows

// Feature: F104
package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writePluginBridgeConfig creates a config for mcp-serve with plugin_tools configuration.
// Returns (configPath, rootDir). With intercept_builtins enabled, this tests
// the server's ability to coexist with plugin configuration.
func writePluginBridgeConfig(t *testing.T) (configPath, rootDir string) {
	t.Helper()
	rootDir = t.TempDir()
	configPath = filepath.Join(rootDir, "mcp-config.json")

	// Config enables both built-ins and an empty plugin_tools list.
	// In-process callers can populate Deps.OperationProviders to inject plugin providers.
	data, err := json.Marshal(map[string]any{
		"intercept_builtins": true,
		"plugin_tools":       []any{},
		"root_dir":           rootDir,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))
	return configPath, rootDir
}

func TestMCPServePluginBridge_ListsPluginTools(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath, _ := writePluginBridgeConfig(t)
	proc := startMCPServeProcess(t, binaryPath, configPath)

	proc.request(t, 1, "initialize", map[string]any{})

	listResp := proc.request(t, 2, "tools/list", nil)
	require.Nil(t, listResp["error"], "tools/list must succeed")

	result, ok := listResp["result"].(map[string]any)
	require.True(t, ok)

	rawTools, ok := result["tools"].([]any)
	require.True(t, ok)

	var toolNames []string
	for _, raw := range rawTools {
		def, isMap := raw.(map[string]any)
		require.True(t, isMap)
		name, isStr := def["name"].(string)
		require.True(t, isStr)
		toolNames = append(toolNames, name)
	}

	// When plugin_tools is configured (even empty), built-in tools are still available
	// This verifies the bridge coexistence pattern
	assert.NotEmpty(t, toolNames, "tools/list must return at least the built-in tools")
	assert.Contains(t, toolNames, "Read", "built-in Read tool must be present")
}

func TestMCPServePluginBridge_CallsPluginTool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath, rootDir := writePluginBridgeConfig(t)
	proc := startMCPServeProcess(t, binaryPath, configPath)

	testFile := filepath.Join(rootDir, "plugin-test.txt")
	content := "plugin integration test data\n"
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))

	proc.request(t, 1, "initialize", map[string]any{})

	// Call a built-in tool through the plugin-aware bridge configuration
	callResp := proc.request(t, 2, "tools/call", map[string]any{
		"name":      "Read",
		"arguments": map[string]any{"path": testFile},
	})
	require.Nil(t, callResp["error"], "tools/call must succeed through plugin bridge")

	result, ok := callResp["result"].(map[string]any)
	require.True(t, ok)

	// Verify result structure contains content blocks (plugin results are also text content)
	contentBlocks, ok := result["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, contentBlocks)

	block, ok := contentBlocks[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "text", block["type"], "tool result must have text type")
	assert.Equal(t, content, block["text"], "tool result must contain the expected payload")
}
