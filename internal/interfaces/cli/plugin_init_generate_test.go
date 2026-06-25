package cli_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginInitGenerate_CreatesAMissingOutputDirectoryAndWritesEveryRenderedFile(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "awf-plugin-example")

	out, err := executePluginInitGenerateCommand(t, outputDir, false)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assertPluginInitGeneratedFileSet(t, outputDir)
}

func TestPluginInitGenerate_SucceedsInAnExistingEmptyOutputDirectory(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "awf-plugin-example")
	require.NoError(t, os.Mkdir(outputDir, 0o755))

	out, err := executePluginInitGenerateCommand(t, outputDir, false)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assertPluginInitGeneratedFileSet(t, outputDir)
}

func TestPluginInitGenerate_SucceedsInAnExistingOutputDirectoryContainingUnrelatedFilesAndPreservesThoseFiles(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "awf-plugin-example")
	require.NoError(t, os.Mkdir(outputDir, 0o755))
	unrelatedPath := filepath.Join(outputDir, "notes.txt")
	unrelatedContent := []byte("keep this file\n")
	require.NoError(t, os.WriteFile(unrelatedPath, unrelatedContent, 0o644))

	out, err := executePluginInitGenerateCommand(t, outputDir, false)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assertPluginInitGeneratedFileSet(t, outputDir)
	assert.Equal(t, unrelatedContent, readPluginInitTestFile(t, unrelatedPath))
}

func TestPluginInitGenerate_RejectsAnExistingConflictingGeneratedFileWithoutForce(t *testing.T) {
	outputDir := t.TempDir()
	conflictPath := filepath.Join(outputDir, "go.mod")
	require.NoError(t, os.WriteFile(conflictPath, []byte("module existing\n"), 0o644))

	out, err := executePluginInitGenerateCommand(t, outputDir, false)

	require.EqualError(t, err, "plugin init output go.mod already exists; use --force to overwrite", "plugin init output:\n%s", out)
	assert.Equal(t, []byte("module existing\n"), readPluginInitTestFile(t, conflictPath))
}

func TestPluginInitGenerate_ConflictRejectionHappensBeforeAnyRenderedFileIsWrittenOrOverwritten(t *testing.T) {
	outputDir := t.TempDir()
	conflictPath := filepath.Join(outputDir, "go.mod")
	require.NoError(t, os.WriteFile(conflictPath, []byte("module existing\n"), 0o644))

	out, err := executePluginInitGenerateCommand(t, outputDir, false)

	require.EqualError(t, err, "plugin init output go.mod already exists; use --force to overwrite", "plugin init output:\n%s", out)
	assert.Equal(t, []byte("module existing\n"), readPluginInitTestFile(t, conflictPath))
	assert.NoFileExists(t, filepath.Join(outputDir, "main.go"))
	assert.NoFileExists(t, filepath.Join(outputDir, "plugin.yaml"))
}

func TestPluginInitGenerate_RejectsSymlinkedGeneratedDirectoryWithoutWritingOutsideOutput(t *testing.T) {
	outputDir := t.TempDir()
	victimDir := t.TempDir()
	symlinkPath := filepath.Join(outputDir, "examples")
	require.NoError(t, os.Symlink(victimDir, symlinkPath))

	out, err := executePluginInitGenerateCommand(t, outputDir, false)

	require.EqualError(t, err, "plugin init output examples/demo.yaml uses symbolic link examples", "plugin init output:\n%s", out)
	assert.NoFileExists(t, filepath.Join(victimDir, "demo.yaml"))
	assert.NoFileExists(t, filepath.Join(outputDir, "go.mod"))
}

func TestPluginInitGenerate_RejectsSymlinkedGeneratedFileWithForceWithoutWritingOutsideOutput(t *testing.T) {
	outputDir := t.TempDir()
	victimDir := t.TempDir()
	victimGoMod := filepath.Join(victimDir, "go.mod")
	require.NoError(t, os.WriteFile(victimGoMod, []byte("module victim\n"), 0o644))
	require.NoError(t, os.Symlink(victimGoMod, filepath.Join(outputDir, "go.mod")))

	out, err := executePluginInitGenerateCommand(t, outputDir, true)

	require.EqualError(t, err, "plugin init output go.mod uses symbolic link go.mod", "plugin init output:\n%s", out)
	assert.Equal(t, []byte("module victim\n"), readPluginInitTestFile(t, victimGoMod))
	assert.NoFileExists(t, filepath.Join(outputDir, "README.md"))
}

func TestPluginInitGenerate_RejectsSymlinkedOutputDirectoryWithoutWritingOutsideOutput(t *testing.T) {
	parentDir := t.TempDir()
	victimDir := t.TempDir()
	outputDir := "awf-plugin-example"
	require.NoError(t, os.Symlink(victimDir, filepath.Join(parentDir, outputDir)))

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(parentDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(wd)) })

	out, err := executePluginInitGenerateCommand(t, outputDir, true)

	require.EqualError(t, err, "plugin init output directory uses symbolic link", "plugin init output:\n%s", out)
	assert.NoFileExists(t, filepath.Join(victimDir, "go.mod"))
	assert.NoFileExists(t, filepath.Join(victimDir, "plugin.yaml"))
}

func TestPluginInitGenerate_OverwritesConflictingGeneratedTargetFilesWhenForceIsTrue(t *testing.T) {
	outputDir := t.TempDir()
	conflictPath := filepath.Join(outputDir, "go.mod")
	require.NoError(t, os.WriteFile(conflictPath, []byte("module existing\n"), 0o644))

	out, err := executePluginInitGenerateCommand(t, outputDir, true)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assertPluginInitGeneratedFileSet(t, outputDir)
	assert.NotEqual(t, []byte("module existing\n"), readPluginInitTestFile(t, conflictPath))
}

func TestPluginInitGenerate_NeverDeletesUnrelatedFilesWhenForceIsTrue(t *testing.T) {
	outputDir := t.TempDir()
	unrelatedPath := filepath.Join(outputDir, "local-only.txt")
	unrelatedContent := []byte("local state\n")
	require.NoError(t, os.WriteFile(unrelatedPath, unrelatedContent, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte("module existing\n"), 0o644))

	out, err := executePluginInitGenerateCommand(t, outputDir, true)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assertPluginInitGeneratedFileSet(t, outputDir)
	assert.Equal(t, unrelatedContent, readPluginInitTestFile(t, unrelatedPath))
}

func TestPluginInitGenerate_WrittenDirectoriesUseMode0755AndGeneratedRegularFilesUseMode0644(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "awf-plugin-example")

	out, err := executePluginInitGenerateCommand(t, outputDir, false)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assertPluginInitPathMode(t, outputDir, 0o755)
	assertPluginInitPathMode(t, filepath.Join(outputDir, ".github"), 0o755)
	assertPluginInitPathMode(t, filepath.Join(outputDir, ".github", "workflows"), 0o755)
	assertPluginInitPathMode(t, filepath.Join(outputDir, "examples"), 0o755)

	for _, path := range expectedPluginInitGeneratedPaths() {
		t.Run(path, func(t *testing.T) {
			assertPluginInitPathMode(t, filepath.Join(outputDir, filepath.FromSlash(path)), 0o644)
		})
	}
}

func TestPluginInitGenerate_WriteErrorsIncludeTheRelativeGeneratedPathThatFailed(t *testing.T) {
	outputDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, ".github"), []byte("not a directory\n"), 0o644))

	out, err := executePluginInitGenerateCommand(t, outputDir, true)

	require.Error(t, err, "plugin init output:\n%s", out)
	assert.Contains(t, err.Error(), ".github/workflows/release.yml")
}

func TestPluginInitGenerate_SuccessfulOutputIncludesExactlyTheRequiredNextStepStages(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "awf-plugin-example")

	out, err := executePluginInitGenerateCommand(t, outputDir, false)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assert.Equal(t, []string{
		"cd " + outputDir,
		"make test",
		"make build",
		"make install-local",
		"awf plugin enable awf-plugin-example",
		"awf plugin list --operations",
		"awf run examples/demo.yaml",
	}, nonEmptyPluginInitOutputLines(out))
}

func TestPluginInitGenerate_SuccessfulOutputAvoidsUnrelatedAbsolutePrivatePathsExceptTheRequestedOrImpliedTargetPath(t *testing.T) {
	privateRoot := t.TempDir()
	outputDir := filepath.Join(privateRoot, "awf-plugin-example")

	out, err := executePluginInitGenerateCommand(t, outputDir, false)

	require.NoError(t, err, "plugin init output:\n%s", out)
	assert.Contains(t, out, outputDir)
	outputWithoutTarget := strings.ReplaceAll(out, outputDir, "")
	assert.NotContains(t, outputWithoutTarget, "/home/")
	assert.NotContains(t, outputWithoutTarget, "/tmp/awf-code-ctx")
	assert.NotContains(t, outputWithoutTarget, privateRoot)
}

func executePluginInitGenerateCommand(t *testing.T, outputDir string, force bool) (string, error) {
	t.Helper()

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	args := []string{
		"plugin",
		"init",
		"awf-plugin-example",
		"--kind",
		"operation",
		"--output",
		outputDir,
		"--storage",
		filepath.Join(t.TempDir(), "storage"),
	}
	if force {
		args = append(args, "--force")
	}
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

func assertPluginInitGeneratedFileSet(t *testing.T, outputDir string) {
	t.Helper()

	for _, path := range expectedPluginInitGeneratedPaths() {
		fullPath := filepath.Join(outputDir, filepath.FromSlash(path))
		assert.FileExists(t, fullPath)
		assert.NotEmpty(t, readPluginInitTestFile(t, fullPath))
	}
}

func expectedPluginInitGeneratedPaths() []string {
	return []string{
		".github/workflows/release.yml",
		"AGENTS.md",
		"Makefile",
		"README.md",
		"examples/demo.yaml",
		"go.mod",
		"main.go",
		"main_test.go",
		"plugin.yaml",
	}
}

func assertPluginInitPathMode(t *testing.T, path string, want fs.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, want, info.Mode().Perm())
}

func readPluginInitTestFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}

func nonEmptyPluginInitOutputLines(output string) []string {
	lines := strings.Split(output, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	return filtered
}
