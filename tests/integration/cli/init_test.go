//go:build integration

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
	"gopkg.in/yaml.v3"
)

func TestInitCommand(t *testing.T) {
	t.Run("creates .awf/workflows directory", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

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
		tmpDir := setupInitTestDir(t)

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

	t.Run("creates .awf.yaml config file", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify config file was created
		configFile := filepath.Join(tmpDir, ".awf.yaml")
		_, err = os.Stat(configFile)
		require.NoError(t, err)

		// Verify content has expected fields
		content, err := os.ReadFile(configFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "version:")
		assert.Contains(t, string(content), "log_level:")
	})

	t.Run("skips if .awf directory already exists", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		// Create .awf directory first
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Should show message about already initialized
		assert.Contains(t, out.String(), "already initialized")
	})

	t.Run("--force overwrites existing configuration", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		// Create existing .awf directory with custom content
		awfDir := filepath.Join(tmpDir, ".awf")
		workflowsDir := filepath.Join(awfDir, "workflows")
		require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

		// Create a custom workflow file
		customFile := filepath.Join(workflowsDir, "custom.yaml")
		require.NoError(t, os.WriteFile(customFile, []byte("custom: true"), 0o644))

		// Create existing config
		configFile := filepath.Join(tmpDir, ".awf.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte("old: config"), 0o644))

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--force"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify example workflow was created (force should create missing files)
		exampleFile := filepath.Join(workflowsDir, "example.yaml")
		_, err = os.Stat(exampleFile)
		require.NoError(t, err)

		// Custom file should still exist (we don't delete user files)
		_, err = os.Stat(customFile)
		require.NoError(t, err)

		// Config should be updated
		content, err := os.ReadFile(configFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "version:")
		assert.NotContains(t, string(content), "old: config")
	})

	t.Run("displays next steps message", func(t *testing.T) {
		_ = setupInitTestDir(t)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify next steps are shown
		output := out.String()
		assert.Contains(t, output, "awf list")
		assert.Contains(t, output, "awf run example")
	})

	t.Run("creates .awf/prompts directory", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify prompts directory was created
		promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
		info, err := os.Stat(promptsDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("creates example prompt file", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify example prompt was created
		exampleFile := filepath.Join(tmpDir, ".awf", "prompts", "example.md")
		_, err = os.Stat(exampleFile)
		require.NoError(t, err)

		// Verify content has meaningful example
		content, err := os.ReadFile(exampleFile)
		require.NoError(t, err)
		assert.NotEmpty(t, content)
		// Should contain markdown content
		assert.Contains(t, string(content), "#")
	})

	t.Run("help text mentions prompts directory", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		initCmd, _, err := cmd.Find([]string{"init"})
		require.NoError(t, err)

		longDesc := initCmd.Long
		assert.Contains(t, longDesc, "prompts")
		assert.Contains(t, longDesc, ".awf/prompts/")
	})

	t.Run("help text does not mention storage directories", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		initCmd, _, err := cmd.Find([]string{"init"})
		require.NoError(t, err)

		longDesc := initCmd.Long
		assert.NotContains(t, longDesc, "storage/states")
		assert.NotContains(t, longDesc, "storage/logs")
		assert.Contains(t, longDesc, "XDG directories")
	})
}

// TestInitCommand_HelpText_GlobalFlag tests that the init command help text
// properly documents the --global flag (T008 of F044 US4).
// This ensures users can discover and understand the --global flag functionality
// through the CLI help system.
func TestInitCommand_HelpText_GlobalFlag(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, initCmd *cobra.Command)
	}{
		{
			name: "long description mentions --global flag",
			checkFunc: func(t *testing.T, initCmd *cobra.Command) {
				longDesc := initCmd.Long

				assert.Contains(t, longDesc, "--global",
					"long description should mention --global flag")
			},
		},
		{
			name: "long description explains --global purpose",
			checkFunc: func(t *testing.T, initCmd *cobra.Command) {
				longDesc := initCmd.Long

				// Should explain that --global creates global prompts directory
				assert.Contains(t, longDesc, "global prompts directory",
					"should explain --global creates global prompts directory")
			},
		},
		{
			name: "long description mentions XDG_CONFIG_HOME",
			checkFunc: func(t *testing.T, initCmd *cobra.Command) {
				longDesc := initCmd.Long

				// Should mention the XDG path
				assert.Contains(t, longDesc, "XDG_CONFIG_HOME",
					"should reference XDG_CONFIG_HOME environment variable")
			},
		},
		{
			name: "long description shows global prompts path",
			checkFunc: func(t *testing.T, initCmd *cobra.Command) {
				longDesc := initCmd.Long

				// Should show the expected path: $XDG_CONFIG_HOME/awf/prompts/
				assert.Contains(t, longDesc, "awf/prompts/",
					"should show the awf/prompts/ path structure")
			},
		},
		{
			name: "examples section includes --global usage",
			checkFunc: func(t *testing.T, initCmd *cobra.Command) {
				longDesc := initCmd.Long

				// Examples should show awf init --global
				assert.Contains(t, longDesc, "awf init --global",
					"examples should include 'awf init --global' usage")
			},
		},
		{
			name: "--global flag has proper description in flags",
			checkFunc: func(t *testing.T, initCmd *cobra.Command) {
				globalFlag := initCmd.Flags().Lookup("global")
				require.NotNil(t, globalFlag, "--global flag should exist")

				// Flag description should be meaningful
				assert.NotEmpty(t, globalFlag.Usage,
					"--global flag should have a usage description")
				assert.Contains(t, globalFlag.Usage, "global",
					"flag usage should mention 'global'")
			},
		},
		{
			name: "--global flag default value is false",
			checkFunc: func(t *testing.T, initCmd *cobra.Command) {
				globalFlag := initCmd.Flags().Lookup("global")
				require.NotNil(t, globalFlag)

				// Default should be false (local init by default)
				assert.Equal(t, "false", globalFlag.DefValue,
					"--global flag default should be false")
			},
		},
		{
			name: "help output shows both local and global initialization options",
			checkFunc: func(t *testing.T, initCmd *cobra.Command) {
				longDesc := initCmd.Long

				// Should document local initialization structure
				assert.Contains(t, longDesc, ".awf/workflows/",
					"should show local workflows directory")
				assert.Contains(t, longDesc, ".awf/prompts/",
					"should show local prompts directory")

				// Should document global initialization
				assert.Contains(t, longDesc, "--global",
					"should document --global flag")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := cli.NewRootCommand()
			initCmd, _, err := root.Find([]string{"init"})
			require.NoError(t, err)
			tt.checkFunc(t, initCmd)
		})
	}
}

// TestInitCommand_HelpText_GlobalFlag_Completeness verifies that all aspects
// of the --global flag are documented in the help text as per F044 US4 requirements.
func TestInitCommand_HelpText_GlobalFlag_Completeness(t *testing.T) {
	root := cli.NewRootCommand()
	initCmd, _, err := root.Find([]string{"init"})
	require.NoError(t, err)

	t.Run("short description is concise", func(t *testing.T) {
		// Short description should be brief and not mention --global
		// (details go in long description)
		shortDesc := initCmd.Short
		assert.NotEmpty(t, shortDesc, "should have short description")
		assert.Less(t, len(shortDesc), 80, "short description should be concise")
	})

	t.Run("long description has proper structure", func(t *testing.T) {
		longDesc := initCmd.Long

		// Should have introduction
		assert.Contains(t, longDesc, "Initialize",
			"should start with initialization description")

		// Should have "This creates:" section for local init
		assert.Contains(t, longDesc, "This creates:",
			"should document what local init creates")

		// Should have "Use --global" section
		assert.Contains(t, longDesc, "Use --global",
			"should have instruction for --global usage")

		// Should have "Examples:" section
		assert.Contains(t, longDesc, "Examples:",
			"should have examples section")
	})

	t.Run("examples cover common use cases", func(t *testing.T) {
		longDesc := initCmd.Long

		// Basic usage
		assert.Contains(t, longDesc, "awf init",
			"should show basic usage example")

		// Force flag
		assert.Contains(t, longDesc, "awf init --force",
			"should show --force usage example")

		// Global flag
		assert.Contains(t, longDesc, "awf init --global",
			"should show --global usage example")
	})

	t.Run("flags are properly registered", func(t *testing.T) {
		// Both --force and --global flags should be registered
		forceFlag := initCmd.Flags().Lookup("force")
		require.NotNil(t, forceFlag, "--force flag should be registered")

		globalFlag := initCmd.Flags().Lookup("global")
		require.NotNil(t, globalFlag, "--global flag should be registered")

		// Both should be boolean flags
		assert.Equal(t, "bool", forceFlag.Value.Type())
		assert.Equal(t, "bool", globalFlag.Value.Type())
	})
}

// TestInitCommand_GlobalFlag tests the --global flag for F044 US4.
// This is separate from TestInitGlobalCommand in init_global_test.go which has
// more comprehensive scenario tests. This test verifies the flag exists and
// triggers the global initialization behavior.
func TestInitCommand_GlobalFlag(t *testing.T) {
	t.Run("--global flag creates prompts in XDG_CONFIG_HOME", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Save and restore XDG_CONFIG_HOME
		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify global prompts directory was created
		globalPromptsDir := filepath.Join(tmpDir, "awf", "prompts")
		info, statErr := os.Stat(globalPromptsDir)
		require.NoError(t, statErr, "global prompts directory should be created at $XDG_CONFIG_HOME/awf/prompts")
		assert.True(t, info.IsDir())

		// Verify example prompt was created
		examplePrompt := filepath.Join(globalPromptsDir, "example.md")
		_, statErr = os.Stat(examplePrompt)
		require.NoError(t, statErr, "example prompt should be created in global directory")
	})

	t.Run("--global flag is recognized by init command", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		initCmd, _, err := cmd.Find([]string{"init"})
		require.NoError(t, err)

		// Check that --global flag is defined
		globalFlag := initCmd.Flags().Lookup("global")
		require.NotNil(t, globalFlag, "--global flag should be defined on init command")
		assert.Equal(t, "bool", globalFlag.Value.Type())
	})
}

// TestInitCommand_GlobalFlag_CreatesExamplePrompt tests F044 US4 example prompt creation.
// This test verifies that the example prompt file:
// - Is created with proper filename (example.md)
// - Contains meaningful markdown content
// - Includes usage instructions with @prompts/ reference
// - Documents template variable syntax
func TestInitCommand_GlobalFlag_CreatesExamplePrompt(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T, tmpDir string)
		force          bool
		wantFile       bool
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:     "creates example.md with markdown content",
			setup:    nil,
			force:    false,
			wantFile: true,
			wantContains: []string{
				"# Example Prompt",
				"@prompts/",
				"{{inputs.prompt}}",
			},
		},
		{
			name: "skips example.md if already exists without force",
			setup: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, "awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0o755))
				existingContent := "# My Custom Prompt\n\nDo not overwrite me!"
				require.NoError(t, os.WriteFile(
					filepath.Join(promptsDir, "example.md"),
					[]byte(existingContent),
					0o644,
				))
			},
			force:    false,
			wantFile: true,
			wantContains: []string{
				"My Custom Prompt",
				"Do not overwrite me",
			},
			wantNotContain: []string{
				"@prompts/",
			},
		},
		{
			name: "overwrites example.md with force flag",
			setup: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, "awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0o755))
				existingContent := "# Old Content\n\nThis will be replaced."
				require.NoError(t, os.WriteFile(
					filepath.Join(promptsDir, "example.md"),
					[]byte(existingContent),
					0o644,
				))
			},
			force:    true,
			wantFile: true,
			wantContains: []string{
				"# Example Prompt",
				"@prompts/",
			},
			wantNotContain: []string{
				"Old Content",
				"This will be replaced",
			},
		},
		{
			name:     "example.md contains usage documentation",
			setup:    nil,
			force:    false,
			wantFile: true,
			wantContains: []string{
				"## Usage",
				"awf run",
				"--input",
			},
		},
		{
			name:     "example.md contains template variable documentation",
			setup:    nil,
			force:    false,
			wantFile: true,
			wantContains: []string{
				"## Template Variables",
				"{{inputs.prompt}}",
			},
		},
		{
			name:     "example.md contains tips section",
			setup:    nil,
			force:    false,
			wantFile: true,
			wantContains: []string{
				"## Tips",
				".md",
				".txt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Save and restore XDG_CONFIG_HOME
			t.Setenv("XDG_CONFIG_HOME", tmpDir)

			// Run setup if provided
			if tt.setup != nil {
				tt.setup(t, tmpDir)
			}

			// Build command args
			args := []string{"init", "--global"}
			if tt.force {
				args = append(args, "--force")
			}

			cmd := cli.NewRootCommand()
			cmd.SetArgs(args)

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			err := cmd.Execute()
			require.NoError(t, err)

			// Check file existence
			examplePath := filepath.Join(tmpDir, "awf", "prompts", "example.md")
			_, statErr := os.Stat(examplePath)
			if tt.wantFile {
				require.NoError(t, statErr, "example.md should exist")
			} else {
				require.Error(t, statErr, "example.md should not exist")
				return
			}

			// Check content
			content, err := os.ReadFile(examplePath)
			require.NoError(t, err)
			contentStr := string(content)

			for _, want := range tt.wantContains {
				assert.Contains(t, contentStr, want,
					"example.md should contain %q", want)
			}

			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, contentStr, notWant,
					"example.md should not contain %q", notWant)
			}
		})
	}
}

// TestInitCommand_GlobalFlag_PreservesExisting verifies that existing files are NOT
// overwritten when running `awf init --global` without the --force flag.
// This test explicitly validates the acceptance scenario from F044 US4:
// "Given `~/.config/awf/prompts/` already exists, when I run `awf init --global`,
// then existing files are preserved"
func TestInitCommand_GlobalFlag_PreservesExisting(t *testing.T) {
	t.Run("does not overwrite existing example.md without --force", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Save and restore XDG_CONFIG_HOME
		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		// Pre-create the prompts directory with a custom example.md
		promptsDir := filepath.Join(tmpDir, "awf", "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))

		customContent := "# My Custom Prompt\n\nThis content must be preserved!\n"
		examplePath := filepath.Join(promptsDir, "example.md")
		require.NoError(t, os.WriteFile(examplePath, []byte(customContent), 0o644))

		// Run init --global WITHOUT --force
		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify file content was NOT overwritten
		content, err := os.ReadFile(examplePath)
		require.NoError(t, err)
		assert.Equal(t, customContent, string(content),
			"existing example.md must be preserved without --force flag")
	})

	t.Run("preserves user-created prompts alongside example.md", func(t *testing.T) {
		tmpDir := t.TempDir()

		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		// Pre-create prompts directory with user files
		promptsDir := filepath.Join(tmpDir, "awf", "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))

		userPrompt := "# User's Custom Prompt\n\nDo not touch this!\n"
		userPromptPath := filepath.Join(promptsDir, "my-prompt.md")
		require.NoError(t, os.WriteFile(userPromptPath, []byte(userPrompt), 0o644))

		// Run init --global
		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify user's prompt file is preserved
		content, err := os.ReadFile(userPromptPath)
		require.NoError(t, err)
		assert.Equal(t, userPrompt, string(content),
			"user-created prompts must not be modified")
	})

	t.Run("shows already initialized message when directory exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		// Pre-create the prompts directory with example.md
		promptsDir := filepath.Join(tmpDir, "awf", "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(promptsDir, "example.md"),
			[]byte("existing"),
			0o644,
		))

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Should indicate directory already exists
		output := out.String()
		assert.Contains(t, output, "already",
			"should inform user that global prompts already initialized")
	})
}

// TestInitCommand_GlobalFlag_WithForce verifies that --force flag with --global
// correctly overwrites existing files. This test explicitly validates that
// the force flag behavior documented in the CLI works as expected.
func TestInitCommand_GlobalFlag_WithForce(t *testing.T) {
	t.Run("overwrites existing example.md when --force is used", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Save and restore XDG_CONFIG_HOME
		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		// Pre-create prompts directory with custom example.md
		promptsDir := filepath.Join(tmpDir, "awf", "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))

		oldContent := "# Old Custom Content\n\nThis should be replaced with --force.\n"
		examplePath := filepath.Join(promptsDir, "example.md")
		require.NoError(t, os.WriteFile(examplePath, []byte(oldContent), 0o644))

		// Run init --global --force
		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global", "--force"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify file was overwritten with new content
		content, err := os.ReadFile(examplePath)
		require.NoError(t, err)
		contentStr := string(content)

		assert.NotContains(t, contentStr, "Old Custom Content",
			"old content should be replaced when --force is used")
		assert.Contains(t, contentStr, "# Example Prompt",
			"new example prompt content should be present")
		assert.Contains(t, contentStr, "@prompts/",
			"new example prompt should contain usage instructions")
	})

	t.Run("preserves non-example user prompts even with --force", func(t *testing.T) {
		tmpDir := t.TempDir()

		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		// Pre-create prompts directory with user files
		promptsDir := filepath.Join(tmpDir, "awf", "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))

		// Create user's custom prompt (not example.md)
		userPromptContent := "# My Important Prompt\n\nDo not delete this!\n"
		userPromptPath := filepath.Join(promptsDir, "my-important-prompt.md")
		require.NoError(t, os.WriteFile(userPromptPath, []byte(userPromptContent), 0o644))

		// Create a custom example.md that will be overwritten
		oldExample := "# Custom Example\n"
		require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "example.md"), []byte(oldExample), 0o644))

		// Run init --global --force
		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global", "--force"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify user's prompt is preserved (force only affects example.md)
		content, err := os.ReadFile(userPromptPath)
		require.NoError(t, err)
		assert.Equal(t, userPromptContent, string(content),
			"user's custom prompts must be preserved even with --force")
	})

	t.Run("creates example.md in pre-existing empty directory with --force", func(t *testing.T) {
		tmpDir := t.TempDir()

		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		// Pre-create empty prompts directory (simulating partial manual setup)
		promptsDir := filepath.Join(tmpDir, "awf", "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))

		// Run init --global --force on empty directory
		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global", "--force"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify example.md was created
		examplePath := filepath.Join(promptsDir, "example.md")
		_, statErr := os.Stat(examplePath)
		require.NoError(t, statErr, "example.md should be created in pre-existing empty directory")

		content, err := os.ReadFile(examplePath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "# Example Prompt")
	})
}

// TestInitCommand_GlobalFlag_XDGConfigHome verifies that the --global flag
// correctly respects the XDG_CONFIG_HOME environment variable (FR-002).
// Tests:
// - Uses XDG_CONFIG_HOME when set
// - Does NOT use default ~/.config path when XDG_CONFIG_HOME is set
func TestInitCommand_GlobalFlag_XDGConfigHome(t *testing.T) {
	t.Run("uses XDG_CONFIG_HOME when set", func(t *testing.T) {
		customConfigHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", customConfigHome)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify prompts directory was created in custom XDG_CONFIG_HOME
		expectedDir := filepath.Join(customConfigHome, "awf", "prompts")
		info, statErr := os.Stat(expectedDir)
		require.NoError(t, statErr, "should create prompts in $XDG_CONFIG_HOME/awf/prompts/")
		assert.True(t, info.IsDir())

		// Verify example.md exists
		examplePath := filepath.Join(expectedDir, "example.md")
		_, statErr = os.Stat(examplePath)
		require.NoError(t, statErr, "example.md should be created in custom XDG_CONFIG_HOME")
	})

	t.Run("does not create in default location when XDG_CONFIG_HOME is set", func(t *testing.T) {
		customConfigHome := t.TempDir()

		// Set a different HOME to ensure we're testing the right thing
		fakeHome := t.TempDir()
		t.Setenv("HOME", fakeHome)
		t.Setenv("XDG_CONFIG_HOME", customConfigHome)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify prompts was NOT created in $HOME/.config/awf/prompts
		defaultDir := filepath.Join(fakeHome, ".config", "awf", "prompts")
		_, statErr := os.Stat(defaultDir)
		assert.True(t, os.IsNotExist(statErr),
			"should NOT create prompts in default ~/.config when XDG_CONFIG_HOME is set")

		// Verify prompts WAS created in custom location
		customDir := filepath.Join(customConfigHome, "awf", "prompts")
		_, statErr = os.Stat(customDir)
		require.NoError(t, statErr, "should create prompts in custom XDG_CONFIG_HOME")
	})

	t.Run("output message shows correct path with custom XDG_CONFIG_HOME", func(t *testing.T) {
		customConfigHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", customConfigHome)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Output should contain the actual custom path
		output := out.String()
		assert.Contains(t, output, customConfigHome,
			"output should show the actual XDG_CONFIG_HOME path used")
	})
}

// TestInitCommand_ProjectConfigFile tests creation of .awf/config.yaml (FR-006).
// This is the project configuration file that contains the inputs template.
// It is separate from .awf.yaml (root config) and contains workflow input defaults.
func TestInitCommand_ProjectConfigFile(t *testing.T) {
	t.Run("creates .awf/config.yaml with inputs template", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify project config file was created at .awf/config.yaml
		projectConfigPath := filepath.Join(tmpDir, ".awf", "config.yaml")
		_, statErr := os.Stat(projectConfigPath)
		require.NoError(t, statErr, ".awf/config.yaml should be created")

		// Verify content has inputs section (commented template)
		content, err := os.ReadFile(projectConfigPath)
		require.NoError(t, err)
		contentStr := string(content)

		// FR-006: Must have commented inputs: section
		assert.Contains(t, contentStr, "inputs:",
			"project config should contain inputs: section")
	})

	t.Run("project config contains commented example inputs", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		projectConfigPath := filepath.Join(tmpDir, ".awf", "config.yaml")
		content, err := os.ReadFile(projectConfigPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Per data model example, should have commented example inputs
		assert.Contains(t, contentStr, "# project:",
			"should have commented project example")
		assert.Contains(t, contentStr, "# environment:",
			"should have commented environment example")
	})

	t.Run("project config documents CLI override behavior", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		projectConfigPath := filepath.Join(tmpDir, ".awf", "config.yaml")
		content, err := os.ReadFile(projectConfigPath)
		require.NoError(t, err)
		contentStr := string(content)

		// FR-003: CLI flags override config - should be documented
		assert.Contains(t, contentStr, "--input",
			"should document that CLI --input flags override config values")
	})

	t.Run("project config warns about secrets", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		projectConfigPath := filepath.Join(tmpDir, ".awf", "config.yaml")
		content, err := os.ReadFile(projectConfigPath)
		require.NoError(t, err)
		contentStr := string(content)

		// NFR-002: No secrets guidance
		assert.Contains(t, contentStr, "secret",
			"should warn about not storing secrets")
	})

	t.Run("skips project config if already exists without force", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		// First run init to create the initial structure
		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Now modify the config.yaml with custom content
		awfDir := filepath.Join(tmpDir, ".awf")
		existingContent := "# My custom project config\ninputs:\n  myvar: myvalue\n"
		projectConfigPath := filepath.Join(awfDir, "config.yaml")
		require.NoError(t, os.WriteFile(projectConfigPath, []byte(existingContent), 0o644))

		// Run init again WITHOUT --force (should skip since .awf exists)
		cmd2 := cli.NewRootCommand()
		cmd2.SetArgs([]string{"init"})

		var out2 bytes.Buffer
		cmd2.SetOut(&out2)
		cmd2.SetErr(&out2)

		err = cmd2.Execute()
		require.NoError(t, err)

		// Should say "already initialized"
		assert.Contains(t, out2.String(), "already initialized",
			"should indicate already initialized")

		// Verify existing content is preserved
		content, err := os.ReadFile(projectConfigPath)
		require.NoError(t, err)

		assert.Contains(t, string(content), "myvar",
			"existing project config should be preserved without --force")
	})

	t.Run("force overwrites existing project config", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		// Pre-create .awf directory and config.yaml
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		oldContent := "# Old config that should be replaced\nold: content\n"
		projectConfigPath := filepath.Join(awfDir, "config.yaml")
		require.NoError(t, os.WriteFile(projectConfigPath, []byte(oldContent), 0o644))

		// Run init --force
		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--force"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify content was overwritten with new template
		content, err := os.ReadFile(projectConfigPath)
		require.NoError(t, err)
		contentStr := string(content)

		assert.NotContains(t, contentStr, "Old config",
			"old content should be replaced with --force")
		assert.Contains(t, contentStr, "inputs:",
			"new template should have inputs section")
	})

	t.Run("output mentions project config creation", func(t *testing.T) {
		_ = setupInitTestDir(t)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		// Should show that .awf/config.yaml was created
		assert.Contains(t, output, ".awf/config.yaml",
			"output should mention .awf/config.yaml creation")
	})

	t.Run("help text documents project config", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		initCmd, _, err := cmd.Find([]string{"init"})
		require.NoError(t, err)

		longDesc := initCmd.Long
		assert.Contains(t, longDesc, ".awf/config.yaml",
			"help text should document .awf/config.yaml creation")
	})
}

// TestInitCommand_ProjectConfigFile_ValidYAML tests that the generated project
// config is valid YAML that can be parsed.
func TestInitCommand_ProjectConfigFile_ValidYAML(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"init"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	projectConfigPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	content, err := os.ReadFile(projectConfigPath)
	require.NoError(t, err)

	// Try to parse as YAML - should not error
	var parsed map[string]interface{}
	err = parseYAML(content, &parsed)
	require.NoError(t, err, "generated project config should be valid YAML")
}

// TestInitCommand_ProjectConfigFile_Permissions tests file permissions.
func TestInitCommand_ProjectConfigFile_Permissions(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"init"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	projectConfigPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	info, err := os.Stat(projectConfigPath)
	require.NoError(t, err)

	// Should be 0644 (readable by owner/group, writable by owner)
	mode := info.Mode().Perm()
	assert.True(t, mode&0o400 != 0, "file should be readable by owner")
	assert.True(t, mode&0o200 != 0, "file should be writable by owner")
}

// parseYAML is a helper for YAML parsing in tests.
func parseYAML(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

// TestInitCommand_GlobalFlag_ExamplePromptPermissions verifies file permissions.
func TestInitCommand_GlobalFlag_ExamplePromptPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cmd := cli.NewRootCommand()
	cmd.SetArgs([]string{"init", "--global"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	// Check file permissions (should be 0644)
	examplePath := filepath.Join(tmpDir, "awf", "prompts", "example.md")
	info, err := os.Stat(examplePath)
	require.NoError(t, err)

	// On Unix, check that file is readable by owner and group
	mode := info.Mode().Perm()
	assert.True(t, mode&0o400 != 0, "file should be readable by owner")
	assert.True(t, mode&0o200 != 0, "file should be writable by owner")
}
