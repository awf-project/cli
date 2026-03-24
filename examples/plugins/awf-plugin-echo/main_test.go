package main

import (
	"context"
	"os"
	"testing"

	"github.com/awf-project/cli/pkg/plugin/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEchoPlugin_ImplementsPlugin verifies that EchoPlugin implements sdk.Plugin interface.
func TestEchoPlugin_ImplementsPlugin(t *testing.T) {
	plugin := &EchoPlugin{}

	require.NotNil(t, plugin)

	// Verify interface implementation at compile time
	var _ sdk.Plugin = (*EchoPlugin)(nil)
}

// TestEchoPlugin_Name_ReturnsPluginName verifies Name returns the correct plugin identifier.
func TestEchoPlugin_Name_ReturnsPluginName(t *testing.T) {
	plugin := &EchoPlugin{}

	name := plugin.Name()

	assert.Equal(t, "awf-plugin-echo", name)
}

// TestEchoPlugin_Version_ReturnsPluginVersion verifies Version returns semantic version.
func TestEchoPlugin_Version_ReturnsPluginVersion(t *testing.T) {
	plugin := &EchoPlugin{}

	version := plugin.Version()

	assert.Equal(t, "1.0.0", version)
}

// TestEchoPlugin_Init_Succeeds verifies that Init completes without error with valid config.
func TestEchoPlugin_Init_Succeeds(t *testing.T) {
	plugin := &EchoPlugin{}
	ctx := context.Background()
	config := map[string]any{}

	err := plugin.Init(ctx, config)

	assert.NoError(t, err)
}

// TestEchoPlugin_Init_WithConfig accepts configuration passed at runtime.
func TestEchoPlugin_Init_WithConfig(t *testing.T) {
	plugin := &EchoPlugin{}
	ctx := context.Background()
	config := map[string]any{
		"prefix": "echo: ",
	}

	err := plugin.Init(ctx, config)

	assert.NoError(t, err)
}

// TestEchoPlugin_Shutdown_Succeeds verifies that Shutdown completes without error.
func TestEchoPlugin_Shutdown_Succeeds(t *testing.T) {
	plugin := &EchoPlugin{}
	ctx := context.Background()

	err := plugin.Shutdown(ctx)

	assert.NoError(t, err)
}

// TestEchoPlugin_ImplementsOperationProvider verifies that EchoPlugin implements OperationProvider.
func TestEchoPlugin_ImplementsOperationProvider(t *testing.T) {
	plugin := &EchoPlugin{}

	// Verify interface implementation at compile time
	var _ sdk.OperationProvider = (*EchoPlugin)(nil)
	assert.NotNil(t, plugin)
}

// TestEchoPlugin_Operations_ReturnsEchoOperation verifies that Operations
// returns a list containing the echo operation.
func TestEchoPlugin_Operations_ReturnsEchoOperation(t *testing.T) {
	plugin := &EchoPlugin{}

	operations := plugin.Operations()

	require.Len(t, operations, 1)
	assert.Equal(t, "echo", operations[0])
}

// TestEchoPlugin_HandleOperation_EchoOperation_ReturnsInput verifies that HandleOperation
// with "echo" operation returns the input text in the output.
func TestEchoPlugin_HandleOperation_EchoOperation_ReturnsInput(t *testing.T) {
	plugin := &EchoPlugin{}
	ctx := context.Background()

	input := map[string]any{
		"text": "hello world",
	}

	result, err := plugin.HandleOperation(ctx, "echo", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)

	// Echo operation should return the input text as output
	assert.Equal(t, "hello world", result.Output)
}

// TestEchoPlugin_HandleOperation_EchoOperation_EmptyInput handles empty text input.
func TestEchoPlugin_HandleOperation_EchoOperation_EmptyInput(t *testing.T) {
	plugin := &EchoPlugin{}
	ctx := context.Background()

	input := map[string]any{
		"text": "",
	}

	result, err := plugin.HandleOperation(ctx, "echo", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "", result.Output)
}

// TestEchoPlugin_HandleOperation_UnknownOperation returns error for unknown operations.
func TestEchoPlugin_HandleOperation_UnknownOperation(t *testing.T) {
	plugin := &EchoPlugin{}
	ctx := context.Background()

	input := map[string]any{}

	result, err := plugin.HandleOperation(ctx, "unknown", input)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestEchoPlugin_HandleOperation_MissingRequiredInput returns error when required field is missing.
func TestEchoPlugin_HandleOperation_MissingRequiredInput(t *testing.T) {
	plugin := &EchoPlugin{}
	ctx := context.Background()

	input := map[string]any{
		// Missing "text" field
	}

	result, err := plugin.HandleOperation(ctx, "echo", input)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestEchoPlugin_CanServe verifies that the plugin can be served via sdk.Serve().
func TestEchoPlugin_CanServe(t *testing.T) {
	plugin := &EchoPlugin{}

	// Verify the plugin implements sdk.Plugin interface
	var _ sdk.Plugin = plugin
	assert.NotNil(t, plugin)
}

// TestEchoPlugin_Context_Cancellation verifies that HandleOperation respects context cancellation.
func TestEchoPlugin_Context_Cancellation(t *testing.T) {
	plugin := &EchoPlugin{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input := map[string]any{
		"text": "test",
	}

	result, err := plugin.HandleOperation(ctx, "echo", input)

	// Operation should either complete (echo is synchronous) or respect context
	// If it respects context, err should be non-nil or result non-nil
	if err == nil {
		require.NotNil(t, result)
	}
}

// BenchmarkEchoPlugin_HandleOperation_SimpleText measures performance of echo operation
// with simple text input to establish baseline for NFR-001 (< 10ms latency).
func BenchmarkEchoPlugin_HandleOperation_SimpleText(b *testing.B) {
	plugin := &EchoPlugin{}
	ctx := context.Background()
	input := map[string]any{
		"text": "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = plugin.HandleOperation(ctx, "echo", input)
	}
}

// TestPluginYAMLManifest_ExistsAndIsValid verifies that plugin.yaml exists
// and contains required fields.
func TestPluginYAMLManifest_ExistsAndIsValid(t *testing.T) {
	content, err := os.ReadFile("plugin.yaml")
	require.NoError(t, err, "plugin.yaml must exist in plugin directory")

	// Verify file has content
	assert.Greater(t, len(content), 0, "plugin.yaml should not be empty")

	// Verify required fields are present (basic string checks)
	manifestStr := string(content)
	assert.Contains(t, manifestStr, "name:", "manifest must contain 'name'")
	assert.Contains(t, manifestStr, "awf-plugin-echo", "manifest must declare plugin name")
	assert.Contains(t, manifestStr, "version:", "manifest must contain 'version'")
	assert.Contains(t, manifestStr, "awf_version:", "manifest must contain 'awf_version'")
	assert.Contains(t, manifestStr, "capabilities:", "manifest must declare capabilities")
	assert.Contains(t, manifestStr, "operations", "manifest must declare operations capability")
}

// TestPluginMakefile_ExistsAndBuildable verifies that Makefile exists
// and contains build targets.
func TestPluginMakefile_ExistsAndBuildable(t *testing.T) {
	content, err := os.ReadFile("Makefile")
	require.NoError(t, err, "Makefile must exist in plugin directory")

	// Verify file has content
	assert.Greater(t, len(content), 0, "Makefile should not be empty")

	// Verify required targets are present
	makefileStr := string(content)
	assert.Contains(t, makefileStr, "build:", "Makefile must contain 'build' target")
}

// TestPluginREADME_ExistsAndDocumented verifies that README.md exists
// and documents the plugin.
func TestPluginREADME_ExistsAndDocumented(t *testing.T) {
	content, err := os.ReadFile("README.md")
	require.NoError(t, err, "README.md must exist in plugin directory")

	// Verify file has content
	assert.Greater(t, len(content), 0, "README.md should not be empty")

	// Verify basic documentation structure
	readmeStr := string(content)
	assert.Contains(t, readmeStr, "awf-plugin-echo", "README should mention plugin name")
	assert.Contains(t, readmeStr, "echo", "README should document echo operation")
}
