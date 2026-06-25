package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/pkg/plugin/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func newTestEchoPlugin() *EchoPlugin {
	return &EchoPlugin{BasePlugin: sdk.BasePlugin{PluginName: "echo", PluginVersion: "1.0.0"}}
}

func TestMainGoUsesSDKServePluginAsTheProcessEntryPoint(t *testing.T) {
	source := readEchoSource(t)

	assert.Contains(t, source, "sdk.Serve(")
}

func TestMainGoEmbedsOrUsesSDKBasePluginWithRuntimePluginIDEcho(t *testing.T) {
	var _ sdk.Plugin = (*EchoPlugin)(nil)
	plugin := newTestEchoPlugin()

	assert.Equal(t, "echo", plugin.Name())
	assert.Equal(t, "1.0.0", plugin.Version())
}

func TestMainGoImplementsOperationsWithOperationEcho(t *testing.T) {
	var _ sdk.OperationProvider = (*EchoPlugin)(nil)

	assert.Equal(t, []string{"echo"}, newTestEchoPlugin().Operations())
}

func TestMainGoImplementsHandleOperationUsingSDKInputHelpersForRequiredTextAndOptionalPrefix(t *testing.T) {
	source := readEchoSource(t)

	assert.Contains(t, source, `sdk.GetString(inputs, "text")`)
	assert.Contains(t, source, `sdk.GetString(inputs, "prefix")`)
}

func TestHandleOperationReturnsSDKNewErrorResultNilForMissingText(t *testing.T) {
	result, err := newTestEchoPlugin().HandleOperation(context.Background(), "echo", map[string]any{})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "text")
}

func TestHandleOperationReturnsSDKNewErrorResultNilForEmptyText(t *testing.T) {
	result, err := newTestEchoPlugin().HandleOperation(context.Background(), "echo", map[string]any{"text": ""})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "text")
}

func TestHandleOperationReturnsSDKNewSuccessResultNilWithOutputTextAndPrefix(t *testing.T) {
	result, err := newTestEchoPlugin().HandleOperation(context.Background(), "echo", map[string]any{"text": "hello"})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)
	assert.Equal(t, "hello", result.Output)
	assert.Equal(t, map[string]any{"output": "hello", "text": "hello", "prefix": ""}, result.Data)
}

func TestPrefixBehaviorPrependsPrefixOnlyWhenProvided(t *testing.T) {
	plugin := newTestEchoPlugin()

	withoutPrefix, err := plugin.HandleOperation(context.Background(), "echo", map[string]any{"text": "hello"})
	require.NoError(t, err)
	require.NotNil(t, withoutPrefix)
	assert.Equal(t, "hello", withoutPrefix.Output)

	withPrefix, err := plugin.HandleOperation(context.Background(), "echo", map[string]any{"text": "hello", "prefix": "say: "})
	require.NoError(t, err)
	require.NotNil(t, withPrefix)
	assert.Equal(t, "say: hello", withPrefix.Output)
	assert.Equal(t, "say: ", withPrefix.Data["prefix"])
}

func TestMainGoImplementsOperationSchemaMetadataEquivalentToTheGeneratedOperationTemplate(t *testing.T) {
	var _ sdk.OperationSchemaProvider = (*EchoPlugin)(nil)

	schema, ok := newTestEchoPlugin().GetOperationSchema("echo")

	require.True(t, ok)
	assert.Equal(t, "Echo text, optionally prepending a prefix.", schema.Description)
	require.Len(t, schema.Inputs, 2)
	assert.Equal(t, sdk.InputMeta{Name: "text", Type: sdk.InputTypeString, Required: true, Description: "Text to echo."}, schema.Inputs[0])
	assert.Equal(t, sdk.InputMeta{Name: "prefix", Type: sdk.InputTypeString, Required: false, Description: "Optional prefix prepended to the text."}, schema.Inputs[1])
	assert.ElementsMatch(t, []sdk.OutputMeta{
		{Name: "output", Type: sdk.InputTypeString, Description: "Final echoed output."},
		{Name: "text", Type: sdk.InputTypeString, Description: "Original text input."},
		{Name: "prefix", Type: sdk.InputTypeString, Description: "Prefix input, when provided."},
	}, schema.Outputs)
}

func TestHandleOperationReturnsStructuredErrorResultForUnknownOperation(t *testing.T) {
	result, err := newTestEchoPlugin().HandleOperation(context.Background(), "missing", map[string]any{"text": "hello"})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unknown operation")
}

func TestMainTestGoValidatesPluginYAMLThroughTheSameParserBackedManifestPathUsedByGeneratedPluginTests(t *testing.T) {
	manifest, err := pluginmgr.NewManifestParser().ParseFile("plugin.yaml")

	require.NoError(t, err)
	require.NoError(t, manifest.Validate())
	assert.Equal(t, "echo", manifest.Name)
	assert.Equal(t, "1.0.0", manifest.Version)
	assert.NotEmpty(t, manifest.Description)
	assert.NotEmpty(t, manifest.AWFVersion)
	assert.True(t, manifest.HasCapability(pluginmodel.CapabilityOperations))
}

func TestPluginYAMLUsesManifestRuntimeNameEchoNotAWFPluginEchoAndDeclaresOperationsCapability(t *testing.T) {
	manifest, err := pluginmgr.NewManifestParser().ParseFile("plugin.yaml")

	require.NoError(t, err)
	assert.Equal(t, "echo", manifest.Name)
	assert.NotEqual(t, "awf-plugin-echo", manifest.Name)
	assert.True(t, manifest.HasCapability(pluginmodel.CapabilityOperations))
}

func TestMakefileProvidesBuildTestLintInstallLocalUninstallLocalPackageAndChecksumsTargets(t *testing.T) {
	targets := parseEchoMakeTargets(readEchoMakefile(t))

	assert.ElementsMatch(t, []string{
		"build",
		"test",
		"lint",
		"install-local",
		"uninstall-local",
		"package",
		"checksums",
	}, targets)
}

func TestMakeBuildBuildsTheAWFPluginEchoBinaryIntoTheExamplesLocalBuildOutputDirectory(t *testing.T) {
	makefile := readEchoMakefile(t)

	assert.Contains(t, makefile, "DIST_DIR := dist")
	assert.Contains(t, makefile, "PLUGIN_NAME := awf-plugin-echo")
	assert.Contains(t, makefile, "mkdir -p \"$(DIST_DIR)\"")
	assert.Contains(t, makefile, "go build -o $(DIST_DIR)/$(PLUGIN_NAME) .")
}

func TestMakeTestRunsTheExamplesGoTests(t *testing.T) {
	assert.Contains(t, readEchoMakefile(t), "go test .")
}

func TestMakeLintRunsConfiguredLintingOrADeterministicLightweightCheckDocumentedInTheMakefile(t *testing.T) {
	assert.Contains(t, readEchoMakefile(t), "go vet .")
}

func TestMakeInstallLocalCopiesTheBinaryAndPluginYAMLIntoTheAWFPluginDirectoryUsingGeneratedTemplatePathRules(t *testing.T) {
	makefile := readEchoMakefile(t)

	assert.Contains(t, makefile, "install-local: build")
	assert.Contains(t, makefile, `mkdir -p "$${HOME}/.local/share/awf/plugins/echo"`)
	assert.Contains(t, makefile, `cp "$(DIST_DIR)/$(PLUGIN_NAME)" "$${HOME}/.local/share/awf/plugins/echo/$(PLUGIN_NAME)"`)
	assert.Contains(t, makefile, `cp plugin.yaml "$${HOME}/.local/share/awf/plugins/echo/plugin.yaml"`)
}

func TestMakeUninstallLocalRemovesOnlyFilesInstalledForEcho(t *testing.T) {
	makefile := readEchoMakefile(t)

	assert.Contains(t, makefile, "uninstall-local:")
	assert.Contains(t, makefile, `rm -f "$${HOME}/.local/share/awf/plugins/echo/$(PLUGIN_NAME)"`)
	assert.Contains(t, makefile, `rm -f "$${HOME}/.local/share/awf/plugins/echo/plugin.yaml"`)
	assert.NotContains(t, makefile, "rm -rf")
}

func TestMakePackageProducesInstallerCompatibleArchiveNamesUsingGeneratedOperationTemplatePattern(t *testing.T) {
	makefile := readEchoMakefile(t)

	assert.Contains(t, makefile, "ARCHIVE := $(PLUGIN_NAME)_$(VERSION)_$(GOOS)_$(GOARCH).tar.gz")
	assert.Contains(t, makefile, `tar -C "$(DIST_DIR)/$(PACKAGE_DIR)" -czf "$(DIST_DIR)/$(ARCHIVE)" .`)
	assert.NotContains(t, makefile, ".zip")
}

func TestMakeChecksumsWritesSHA256ChecksumsForPackageArtifacts(t *testing.T) {
	makefile := readEchoMakefile(t)

	assert.Contains(t, makefile, "checksums: package")
	assert.Contains(t, makefile, `sha256sum *.tar.gz > checksums.txt`)
}

func TestExamplesDemoYAMLCallsEchoEchoWithRequiredTextAndOptionalPrefixDataThroughTypeOperation(t *testing.T) {
	demo := readEchoDemoWorkflow(t)

	state := demo.States.Echo
	require.Equal(t, "operation", state.Type)
	assert.Equal(t, "echo.echo", state.Operation)
	assert.Equal(t, "hello from echo", state.Inputs["text"])
	assert.Equal(t, "AWF: ", state.Inputs["prefix"])
}

func TestExamplesDemoYAMLEndsInTerminalSuccessWhenTheOperationSucceeds(t *testing.T) {
	demo := readEchoDemoWorkflow(t)

	echoState := demo.States.Echo
	doneState := demo.States.Done
	assert.Equal(t, "done", echoState.OnSuccess)
	assert.Equal(t, "terminal", doneState.Type)
	assert.Equal(t, "success", doneState.Status)
}

func readEchoSource(t *testing.T) string {
	t.Helper()

	source, err := os.ReadFile("main.go")
	require.NoError(t, err)
	return string(source)
}

func readEchoMakefile(t *testing.T) string {
	t.Helper()

	source, err := os.ReadFile("Makefile")
	require.NoError(t, err)
	return string(source)
}

func parseEchoMakeTargets(makefile string) []string {
	targets := make([]string, 0)
	for _, line := range strings.Split(makefile, "\n") {
		if strings.HasPrefix(line, "\t") || !strings.Contains(line, ":") || strings.Contains(line, ":=") {
			continue
		}
		name := strings.TrimSpace(strings.SplitN(line, ":", 2)[0])
		if name == ".PHONY" || strings.Contains(name, " ") || name == "" {
			continue
		}
		targets = append(targets, name)
	}
	return targets
}

type echoDemoWorkflow struct {
	States struct {
		Initial string `yaml:"initial"`
		Echo    struct {
			Type      string            `yaml:"type"`
			Operation string            `yaml:"operation"`
			Inputs    map[string]string `yaml:"inputs"`
			OnSuccess string            `yaml:"on_success"`
		} `yaml:"echo"`
		Done struct {
			Type   string `yaml:"type"`
			Status string `yaml:"status"`
		} `yaml:"done"`
	} `yaml:"states"`
}

func readEchoDemoWorkflow(t *testing.T) echoDemoWorkflow {
	t.Helper()

	source, err := os.ReadFile("examples/demo.yaml")
	require.NoError(t, err)

	var demo echoDemoWorkflow
	require.NoError(t, yaml.Unmarshal(source, &demo))
	return demo
}
