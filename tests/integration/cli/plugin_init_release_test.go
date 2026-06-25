//go:build integration

// Feature: F111
package cli_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginInitReleasePackaging(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	awfBin := buildPluginInitReleasePackagingAWF(t, ctx)
	env := pluginInitReleasePackagingEnv(t)
	workspace := t.TempDir()
	outputDir := filepath.Join(workspace, "awf-plugin-example")
	distributionName := "awf-plugin-example"
	expectedArchiveName := fmt.Sprintf("%s_0.1.0_%s_%s.tar.gz", distributionName, runtime.GOOS, runtime.GOARCH)
	expectedArchivePath := filepath.Join(outputDir, "dist", expectedArchiveName)
	alternateGOOS, alternateGOARCH := alternatePluginInitReleasePackagingPlatform()

	t.Run("scaffolds a plugin repository in a temporary directory", func(t *testing.T) {
		runPluginInitReleasePackagingCommand(
			t, ctx, workspace, env, awfBin,
			[]string{
				"plugin", "init", distributionName,
				"--kind", "operation",
				"--output", outputDir,
			},
		)

		assert.FileExists(t, filepath.Join(outputDir, "go.mod"))
		assert.FileExists(t, filepath.Join(outputDir, "Makefile"))
		assert.FileExists(t, filepath.Join(outputDir, "plugin.yaml"))
	})

	t.Run("runs generated make build make package and make checksums without manual edits", func(t *testing.T) {
		runPluginInitReleasePackagingCommand(t, ctx, outputDir, env, "make", []string{"build"})
		require.NoError(t, os.MkdirAll(filepath.Join(outputDir, "dist", "package"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(outputDir, "dist", "package", "stale.txt"), []byte("stale"), 0o644))
		runPluginInitReleasePackagingCommand(t, ctx, outputDir, env, "make", []string{"package"})
		runPluginInitReleasePackagingCommand(
			t,
			ctx,
			outputDir,
			env,
			"make",
			[]string{"package", "GOOS=" + alternateGOOS, "GOARCH=" + alternateGOARCH},
		)
		runPluginInitReleasePackagingCommand(t, ctx, outputDir, env, "make", []string{"checksums"})
	})

	t.Run("make package produces a tar.gz archive for the current GOOS and GOARCH with the distribution name version 0.1.0 and Go runtime suffix expected by current installer lookup", func(t *testing.T) {
		assert.FileExists(t, expectedArchivePath)
	})

	t.Run("generated package names are compatible with pkg registry FindPlatformAsset expectations or the current installer helper used by existing integration tests", func(t *testing.T) {
		asset, err := registry.FindPlatformAsset([]registry.Asset{{Name: expectedArchiveName}}, runtime.GOOS, runtime.GOARCH)

		require.NoError(t, err)
		assert.Equal(t, expectedArchiveName, asset.Name)
	})

	t.Run("the generated archive contains the plugin binary named with the distribution name and plugin.yaml", func(t *testing.T) {
		files := readTarGzFileNames(t, expectedArchivePath)

		assert.Contains(t, files, distributionName)
		assert.Contains(t, files, "plugin.yaml")
	})

	t.Run("the archive does not require unrelated repository files for installation", func(t *testing.T) {
		files := readTarGzFileNames(t, expectedArchivePath)

		assert.ElementsMatch(t, []string{distributionName, "plugin.yaml"}, files)
	})

	t.Run("checksums.txt contains a SHA-256 checksum entry for every generated package archive", func(t *testing.T) {
		archives, err := filepath.Glob(filepath.Join(outputDir, "dist", "*.tar.gz"))
		require.NoError(t, err)
		require.NotEmpty(t, archives)

		checksums := readChecksumEntries(t, filepath.Join(outputDir, "dist", "checksums.txt"))
		for _, archive := range archives {
			name := filepath.Base(archive)
			sum := sha256FileHex(t, archive)
			assert.Equal(t, sum, checksums[name])
		}
	})

	t.Run("the generated release workflow uses the same archive naming pattern asserted by the integration test", func(t *testing.T) {
		release, err := os.ReadFile(filepath.Join(outputDir, ".github", "workflows", "release.yml"))
		require.NoError(t, err)

		expectedCommand := `tar -C dist/package -czf "dist/` + distributionName + `_0.1.0_${GOOS}_${GOARCH}.tar.gz" .`
		assertReleaseWorkflowLine(t, string(release), expectedCommand)
		assertReleaseWorkflowLine(t, string(release), `GOFLAGS: -mod=mod`)
	})
}

func buildPluginInitReleasePackagingAWF(t *testing.T, ctx context.Context) string {
	t.Helper()

	repoRoot := pluginInitReleasePackagingRepoRoot(t)
	binPath := filepath.Join(t.TempDir(), "awf")
	runPluginInitReleasePackagingCommand(t, ctx, repoRoot, os.Environ(), "go", []string{"build", "-o", binPath, "./cmd/awf"})
	return binPath
}

func pluginInitReleasePackagingEnv(t *testing.T) []string {
	t.Helper()

	root := t.TempDir()
	home := filepath.Join(root, "home")
	pluginsDir := filepath.Join(home, ".local", "share", "awf", "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	return append(
		os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_DATA_HOME="+filepath.Join(root, "data"),
		"XDG_STATE_HOME="+filepath.Join(root, "state"),
		"AWF_PLUGINS_PATH="+pluginsDir,
	)
}

func pluginInitReleasePackagingRepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

func alternatePluginInitReleasePackagingPlatform() (string, string) {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		return "linux", "amd64"
	}

	return "darwin", "arm64"
}

func runPluginInitReleasePackagingCommand(
	t *testing.T,
	ctx context.Context,
	dir string,
	env []string,
	name string,
	args []string,
) string {
	t.Helper()

	cmd := exec.CommandContext(ctx, name)
	cmd.Args = make([]string, len(args)+1)
	cmd.Args[0] = name
	copy(cmd.Args[1:], args)
	cmd.Dir = dir
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s %v failed in %s:\n%s", name, args, dir, output)
	return string(output)
}

func assertReleaseWorkflowLine(t *testing.T, release, expected string) {
	t.Helper()

	count := 0
	for _, line := range strings.Split(release, "\n") {
		if strings.TrimSpace(line) == expected {
			count++
		}
	}

	assert.Equal(t, 1, count, "release.yml must contain exactly one matching archive command")
}

func readTarGzFileNames(t *testing.T, path string) []string {
	t.Helper()

	file, err := os.Open(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, file.Close())
	}()

	gzipReader, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, gzipReader.Close())
	}()

	var files []string
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if header.Typeflag != tar.TypeReg {
			continue
		}
		files = append(files, strings.TrimPrefix(filepath.Clean(header.Name), "."+string(filepath.Separator)))
	}

	return files
}

func readChecksumEntries(t *testing.T, path string) map[string]string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	entries := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		entries[fields[1]] = fields[0]
	}

	return entries
}

func sha256FileHex(t *testing.T, path string) string {
	t.Helper()

	file, err := os.Open(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, file.Close())
	}()

	hash := sha256.New()
	_, err = io.Copy(hash, file)
	require.NoError(t, err)

	return hex.EncodeToString(hash.Sum(nil))
}
