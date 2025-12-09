package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

func TestInitCommand(t *testing.T) {
	t.Run("creates .awf/workflows directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify directory was created
		workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
		info, err := os.Stat(workflowsDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("creates example workflow", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify example workflow was created
		exampleFile := filepath.Join(tmpDir, ".awf", "workflows", "example.yaml")
		_, err = os.Stat(exampleFile)
		require.NoError(t, err)

		// Verify content
		content, err := os.ReadFile(exampleFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "name: example")
	})

	t.Run("skips if directory already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Create directory first
		workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
		require.NoError(t, os.MkdirAll(workflowsDir, 0755))

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Should show message about already existing
		assert.Contains(t, out.String(), "already exists")
	})
}
