package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPluginInitTemplate_ScaffoldCommandCreatesExactlyTheMVPFileSetWithDeterministicRelativePaths(t *testing.T) {
	pluginDir := scaffoldPluginWithCLI(t)

	assert.Equal(t, []string{
		".github/workflows/release.yml",
		"AGENTS.md",
		"Makefile",
		"README.md",
		"examples/demo.yaml",
		"go.mod",
		"main.go",
		"main_test.go",
		"plugin.yaml",
	}, generatedFilePaths(t, pluginDir))
}

func TestPluginInitTemplate_ScaffoldedPluginSourceAndTestsUseSDKServeEchoSchemaMetadataAndManifestContracts(t *testing.T) {
	pluginDir := scaffoldPluginWithCLI(t)
	mainGo := readGeneratedFile(t, pluginDir, "main.go")
	mainTestGo := readGeneratedFile(t, pluginDir, "main_test.go")

	assert.Contains(t, mainGo, `"github.com/awf-project/cli/pkg/plugin/sdk"`)
	assert.Contains(t, mainGo, "sdk.Serve(newPlugin())")
	assert.Contains(t, mainGo, "sdk.BasePlugin")
	assert.Contains(t, mainGo, `operationEcho = "echo"`)
	assert.Contains(t, mainTestGo, "TestEchoOperation")
	assert.Contains(t, mainTestGo, "TestEchoOperationSchemaMetadata")
	assert.Contains(t, mainTestGo, "TestManifestValidation")
}

func TestPluginInitTemplate_ScaffoldedGoModUsesLocalDistributionModulePathWithoutUnpinnedReleaseTimeExecutableDownloads(t *testing.T) {
	pluginDir := scaffoldPluginWithCLI(t)
	goMod := readGeneratedFile(t, pluginDir, "go.mod")

	modulePath := ""
	for _, line := range strings.Split(goMod, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			modulePath = fields[1]
			break
		}
	}

	assert.Equal(t, "awf-plugin-example", modulePath)
	assert.Contains(t, goMod, "github.com/awf-project/cli v0.0.0")
	assert.Contains(t, goMod, "replace github.com/awf-project/cli => ")
	assert.NotContains(t, goMod, "replace github.com/awf-project/cli => .")
	assert.NotContains(t, goMod, "@latest")
	assert.NotContains(t, goMod, "curl ")
	assert.NotContains(t, goMod, "wget ")
}

func TestPluginInitTemplate_ScaffoldedPluginYamlContainsRuntimeNameVersionDescriptionAWFVersionAuthorLicenseAndOperationsCapability(t *testing.T) {
	pluginDir := scaffoldPluginWithCLI(t)
	manifestYAML := readGeneratedFile(t, pluginDir, "plugin.yaml")

	manifest, err := pluginmgr.NewManifestParser().Parse(strings.NewReader(manifestYAML))
	require.NoError(t, err)
	require.NoError(t, manifest.Validate())
	assert.Equal(t, "example", manifest.Name)
	assert.Equal(t, "0.1.0", manifest.Version)
	assert.NotEmpty(t, manifest.Description)
	assert.Equal(t, ">=0.6.0", manifest.AWFVersion)
	assert.NotEmpty(t, manifest.Author)
	assert.NotEmpty(t, manifest.License)
	assert.True(t, manifest.HasCapability(pluginmodel.CapabilityOperations))
}

func TestPluginInitTemplate_ScaffoldedMakefileExposesBuildTestLintInstallLocalUninstallLocalPackageAndChecksumsTargets(t *testing.T) {
	pluginDir := scaffoldPluginWithCLI(t)
	makefile := readGeneratedFile(t, pluginDir, "Makefile")

	targets := parseMakeTargets(makefile)
	for _, target := range []string{"build", "test", "lint", "install-local", "uninstall-local", "package", "checksums"} {
		assert.Contains(t, targets, target)
	}
	assert.Contains(t, makefile, `GOFLAGS ?= -mod=mod`)
	assert.Contains(t, makefile, `mkdir -p "$(DIST_DIR)"`)
	assert.Contains(t, makefile, `go build $(GOFLAGS)`)
	assert.Contains(t, makefile, `go test $(GOFLAGS) .`)
}

func TestPluginInitTemplate_ScaffoldedExamplesDemoYamlCallsRuntimePluginIDEchoThroughNormalTypeOperationSyntaxAndEndsInTerminalSuccess(t *testing.T) {
	pluginDir := scaffoldPluginWithCLI(t)
	demo := readGeneratedFile(t, pluginDir, "examples/demo.yaml")

	var parsed struct {
		States struct {
			Initial string `yaml:"initial"`
			Echo    struct {
				Type      string `yaml:"type"`
				Operation string `yaml:"operation"`
			} `yaml:"echo"`
			Done struct {
				Type   string `yaml:"type"`
				Status string `yaml:"status"`
			} `yaml:"done"`
		} `yaml:"states"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(demo), &parsed))

	assert.Equal(t, "echo", parsed.States.Initial)
	assert.Equal(t, "operation", parsed.States.Echo.Type)
	assert.Equal(t, "example.echo", parsed.States.Echo.Operation)
	assert.Equal(t, "terminal", parsed.States.Done.Type)
	assert.Equal(t, "success", parsed.States.Done.Status)
}

func TestPluginInitTemplate_ScaffoldedAgentsInstructionsDefineActionableAWFPluginRules(t *testing.T) {
	pluginDir := scaffoldPluginWithCLI(t)
	agents := readGeneratedFile(t, pluginDir, "AGENTS.md")

	for _, marker := range []string{
		"# Agent Instructions",
		"load and use the `awf-knowledge` skill",
		"https://github.com/awf-project/awf-marketplace",
		"Do not invent AWF plugin SDK APIs from memory",
		"runtime plugin id is `example`",
		"generated distribution name is `awf-plugin-example`",
		"only implemented manifest capability in this scaffold is `operations`",
		"sdk.Serve(newPlugin())",
		"OperationProvider",
		"OperationSchemaProvider",
		"sdk.NewErrorResult",
		"sdk.NewSuccessResult",
		"No operation may be a no-op",
		"make test",
		"make lint",
		"make build",
		"awf plugin list --operations",
		"awf run examples/demo.yaml",
	} {
		assert.Contains(t, agents, marker)
	}

	for _, marker := range []string{
		"for people",
		"optional guidance",
		"validators are supported",
		"custom step types are supported",
		"event listeners are supported",
	} {
		assert.NotContains(t, strings.ToLower(agents), marker)
	}
}

func TestPluginInitTemplate_ScaffoldedReadmeDocumentsImplementedOperationPluginWorkflow(t *testing.T) {
	pluginDir := scaffoldPluginWithCLI(t)
	readme := strings.ToLower(readGeneratedFile(t, pluginDir, "README.md"))

	for _, marker := range []string{
		"first plugin",
		"manifest",
		"sdk",
		"testing",
		"install",
		"enable",
		"list",
		"run",
		"package",
		"checksum",
		"release",
		"capabilities",
	} {
		assert.Contains(t, readme, marker)
	}

	for _, marker := range []string{
		"future",
		"planned",
		"canonical-port",
		"adapter",
		"direct-integration",
		"hybrid",
		"validator",
		"step-type",
		"event-listener",
		"full",
	} {
		assert.NotContains(t, readme, marker)
	}
}

func TestPluginInitTemplate_ScaffoldedGithubWorkflowsReleaseYmlPackagesBinaryPlusPluginYamlGeneratesSHA256ChecksumsAndUsesInstallerCompatibleArchiveNames(t *testing.T) {
	pluginDir := scaffoldPluginWithCLI(t)
	release := readGeneratedFile(t, pluginDir, ".github/workflows/release.yml")

	assert.Contains(t, release, "plugin.yaml")
	assert.Contains(t, strings.ToLower(release), "sha256")
	assert.Contains(t, release, ".tar.gz")
	assert.Contains(t, release, "awf-plugin-example_0.1.0_")
	assert.Contains(t, release, "GOOS")
	assert.Contains(t, release, "GOARCH")
}

func scaffoldPluginWithCLI(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp(".", ".plugin-init-test-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	})
	pluginDir := filepath.Join(tmpDir, "awf-plugin-example")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"plugin",
		"init",
		"awf-plugin-example",
		"--kind",
		"operation",
		"--output",
		pluginDir,
		"--force",
		"--storage",
		filepath.Join(tmpDir, "storage"),
	})

	err = cmd.Execute()
	require.NoError(t, err, "plugin init should scaffold through the public CLI command:\n%s", out.String())

	return pluginDir
}

func generatedFilePaths(t *testing.T, root string) []string {
	t.Helper()

	var paths []string
	require.NoError(t, filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	}))
	sort.Strings(paths)

	return paths
}

func readGeneratedFile(t *testing.T, root, relativePath string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
	require.NoError(t, err)
	require.NotEmpty(t, content, "expected generated file %q to have content", relativePath)

	return string(content)
}

func parseMakeTargets(makefile string) map[string]struct{} {
	targets := make(map[string]struct{})
	for _, line := range strings.Split(makefile, "\n") {
		if line == "" || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "#") {
			continue
		}
		target, _, found := strings.Cut(line, ":")
		if found && target != "" && !strings.ContainsAny(target, " \t") {
			targets[target] = struct{}{}
		}
	}
	return targets
}
