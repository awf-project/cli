package cli_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginInitGeneratedDocs(t *testing.T) {
	pluginDir := scaffoldPluginWithCLI(t)
	readme := readGeneratedFile(t, pluginDir, "README.md")

	tests := []struct {
		name    string
		markers []string
	}{
		{
			name: "asserts generated README.md includes the exact first-run command sequence from scaffold through demo workflow execution",
			markers: []string{
				"awf plugin init awf-plugin-example --kind operation",
				"cd awf-plugin-example",
				"make test",
				"make build",
				"make install-local",
				"awf plugin enable awf-plugin-example",
				"awf plugin list --operations",
				"awf run examples/demo.yaml",
			},
		},
		{
			name: "generated README.md explains manifest fields operation capability semantic version expectations AWF version compatibility config schema checksum release expectations common validation failures and that manifest name is the runtime id",
			markers: []string{
				"Required fields",
				"Optional fields",
				"Supported capability",
				"semantic version",
				"AWF version compatibility",
				"config schema",
				"checksum",
				"release",
				"Common validation failures",
				"manifest `name` is the runtime id",
			},
		},
		{
			name: "generated README.md identifies pkg/plugin/sdk as the supported Go authoring API",
			markers: []string{
				"pkg/plugin/sdk",
				"supported Go authoring API",
			},
		},
		{
			name: "generated README.md covers only the SDK APIs used by the operation scaffold",
			markers: []string{
				"sdk.Serve",
				"sdk.BasePlugin",
				"lifecycle methods",
				"OperationProvider",
				"OperationSchemaProvider",
				"schema metadata",
				"input helpers",
				"sdk.NewSuccessResult",
				"sdk.NewErrorResult",
				"structured outputs",
				"compatibility expectations",
			},
		},
		{
			name: "generated README.md documents make build make test make lint make install-local make uninstall-local make package and make checksums",
			markers: []string{
				"make build",
				"make test",
				"make lint",
				"make install-local",
				"make uninstall-local",
				"make package",
				"make checksums",
			},
		},
		{
			name: "generated README.md documents local awf plugin enable distribution-name awf plugin list operations and awf run examples demo yaml",
			markers: []string{
				"awf plugin enable awf-plugin-example",
				"awf plugin list --operations",
				"awf run examples/demo.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, marker := range tt.markers {
				assert.Contains(t, readme, marker)
			}
		})
	}

	assertOrderedSubstrings(
		t,
		readme,
		[]string{
			"awf plugin init awf-plugin-example --kind operation",
			"cd awf-plugin-example",
			"make test",
			"make build",
			"make install-local",
			"awf plugin enable awf-plugin-example",
			"awf plugin list --operations",
			"awf run examples/demo.yaml",
		},
	)
	assert.NotContains(t, readme, "future")
	assert.NotContains(t, readme, "planned")
	for _, kind := range futurePluginInitKinds() {
		assert.NotContains(t, readme, kind)
	}
}

func TestPluginInitHelp(t *testing.T) {
	help := pluginInitHelpOutput(t)

	assert.Contains(t, help, "Supported kinds:")
	assert.Contains(t, help, "operation")
	assert.Contains(t, help, "plugin scaffold kind (supported: operation)")

	assert.NotContains(t, help, "Future/deferred kinds:")
	assert.NotContains(t, help, "planned")
	assert.NotContains(t, help, "deferred")
	for _, kind := range futurePluginInitKinds() {
		assert.NotContains(t, help, kind)
	}
}

func TestPluginInitUnsupportedKindReturnsError(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "awf-plugin-example")

	out, err := executePluginInitCommand(
		t,
		"plugin", "init", "awf-plugin-example",
		"--kind", "canonical-port",
		"--output", outputDir,
	)

	require.Error(t, err, "plugin init output:\n%s", out)
	structErr := requirePluginInitCommandValidationError(t, err, "kind", "canonical-port", "unsupported-kind")
	assert.Equal(
		t,
		`USER.INPUT.VALIDATION_FAILED: invalid plugin init kind "canonical-port" violates unsupported-kind: unsupported plugin init kind "canonical-port"; supported kind is "operation"`,
		structErr.Message,
	)
	assert.NoDirExists(t, outputDir)
}

func pluginInitHelpOutput(t *testing.T) string {
	t.Helper()

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "init", "--help"})

	require.NoError(t, cmd.Execute())
	return out.String()
}

func futurePluginInitKinds() []string {
	return []string{
		"canonical-port",
		"adapter",
		"direct-integration",
		"hybrid",
		"validator",
		"step-type",
		"event-listener",
		"full",
	}
}

func assertOrderedSubstrings(t *testing.T, content string, markers []string) {
	t.Helper()

	offset := 0
	for _, marker := range markers {
		index := strings.Index(content[offset:], marker)
		require.NotEqual(t, -1, index, "expected %q after byte offset %d", marker, offset)
		offset += index + len(marker)
	}
}
