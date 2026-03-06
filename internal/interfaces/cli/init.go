package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/spf13/cobra"
)

const (
	awfDir            = ".awf"
	workflowsDir      = "workflows"
	promptsDir        = "prompts"
	scriptsDir        = "scripts"
	configFileName    = ".awf.yaml"
	projectConfigFile = "config.yaml"
	exampleFile       = "example.yaml"
	examplePromptFile = "example.md"
	exampleScriptFile = "example.sh"
)

func newInitCommand(cfg *Config) *cobra.Command {
	var force bool
	var global bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize AWF in the current directory",
		Long: `Initialize AWF in the current directory by creating the local configuration.

This creates:
  .awf.yaml                    Configuration file
  .awf/config.yaml             Project config with inputs template
  .awf/workflows/              Local workflows directory
  .awf/workflows/example.yaml  Example workflow file
  .awf/prompts/                Prompt templates directory
  .awf/prompts/example.md      Example prompt file
  .awf/scripts/                Shell scripts directory
  .awf/scripts/example.sh      Example script with shebang

State persistence uses XDG directories ($XDG_DATA_HOME/awf/ or ~/.local/share/awf/).

Use --global to initialize the global prompts directory at $XDG_CONFIG_HOME/awf/prompts/.

Examples:
  awf init
  awf init --force
  awf init --global`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if global {
				return runInitGlobal(cmd, cfg, force)
			}
			return runInit(cmd, cfg, force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing configuration")
	cmd.Flags().BoolVar(&global, "global", false, "Initialize global prompts and scripts directories")

	return cmd
}

func runInit(cmd *cobra.Command, cfg *Config, force bool) error {
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	awfPath := awfDir
	configPath := configFileName

	// Check if already initialized
	if !force {
		if _, err := os.Stat(awfPath); err == nil {
			formatter.Info(fmt.Sprintf("AWF already initialized in '%s'", awfPath))
			formatter.Info("Use --force to reinitialize")
			return nil
		}
	}

	// Create .awf directory structure
	dirs := []string{
		filepath.Join(awfPath, workflowsDir),
		filepath.Join(awfPath, promptsDir),
		filepath.Join(awfPath, scriptsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	formatter.Success(fmt.Sprintf("Created %s", awfPath))

	// Create config file
	if err := createConfigFile(configPath, force); err != nil {
		return err
	}
	formatter.Success(fmt.Sprintf("Created %s", configPath))

	// Create project config file with inputs template (FR-006)
	projectConfigPath := filepath.Join(awfPath, projectConfigFile)
	if err := createProjectConfigFile(projectConfigPath, force); err != nil {
		return err
	}
	formatter.Success(fmt.Sprintf("Created %s", projectConfigPath))

	// Create example workflow
	examplePath := filepath.Join(awfPath, workflowsDir, exampleFile)
	if err := createExampleWorkflow(examplePath, force); err != nil {
		return err
	}
	formatter.Success(fmt.Sprintf("Created %s", examplePath))

	// Create example prompt
	promptPath := filepath.Join(awfPath, promptsDir, examplePromptFile)
	if err := createExamplePrompt(promptPath, force); err != nil {
		return err
	}
	formatter.Success(fmt.Sprintf("Created %s", promptPath))

	// Create example script
	scriptPath := filepath.Join(awfPath, scriptsDir, exampleScriptFile)
	if err := createExampleScript(scriptPath, force); err != nil {
		return err
	}
	formatter.Success(fmt.Sprintf("Created %s", scriptPath))

	// Next steps
	formatter.Info("\nNext steps:")
	formatter.Info("  awf list          - List available workflows")
	formatter.Info("  awf run example   - Run the example workflow")
	formatter.Info("  awf validate      - Validate a workflow file")

	return nil
}

func runInitGlobal(cmd *cobra.Command, cfg *Config, force bool) error {
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	globalPromptsDir := xdg.AWFPromptsDir()
	globalScriptsDir := xdg.AWFScriptsDir()

	_, err := os.Stat(globalPromptsDir)
	if !force && err == nil {
		formatter.Info(fmt.Sprintf("Global prompts already initialized in '%s'", globalPromptsDir))
		formatter.Info("Use --force to reinitialize")
	} else {
		// Create global prompts directory
		if err := os.MkdirAll(globalPromptsDir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", globalPromptsDir, err)
		}
		formatter.Success(fmt.Sprintf("Created %s", globalPromptsDir))

		// Create example prompt
		promptPath := filepath.Join(globalPromptsDir, examplePromptFile)
		if err := createExamplePrompt(promptPath, force); err != nil {
			return err
		}
		formatter.Success(fmt.Sprintf("Created %s", promptPath))
	}

	// Create global scripts directory (independent of prompts state)
	if err := os.MkdirAll(globalScriptsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", globalScriptsDir, err)
	}
	formatter.Success(fmt.Sprintf("Created %s", globalScriptsDir))

	// Create example script
	scriptPath := filepath.Join(globalScriptsDir, exampleScriptFile)
	if err := createExampleScript(scriptPath, force); err != nil {
		return err
	}
	formatter.Success(fmt.Sprintf("Created %s", scriptPath))

	return nil
}

// writeFileUnlessExists writes content to path with the given permissions.
// If force is false and the file already exists, it skips silently.
func writeFileUnlessExists(path string, force bool, content string, perm os.FileMode) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func createConfigFile(path string, force bool) error {
	return writeFileUnlessExists(path, force, `# AWF Configuration
# https://github.com/awf-project/cli

version: "1"

# Default log level: debug, info, warn, error
log_level: info

# Output format: text, json, table, quiet
output_format: text
`, 0o644)
}

func createExampleWorkflow(path string, force bool) error {
	return writeFileUnlessExists(path, force, `name: example
version: "1.0.0"
description: Example workflow created by awf init

states:
  initial: greet

  greet:
    type: step
    command: echo "Hello from AWF!"
    on_success: done

  done:
    type: terminal
`, 0o644)
}

func createProjectConfigFile(path string, force bool) error {
	return writeFileUnlessExists(path, force, `# AWF Project Configuration
# https://github.com/awf-project/cli
#
# This file provides default values for workflow inputs.
# CLI --input flags override these values.
#
# IMPORTANT: Do not store secrets here - use environment variables instead.

# Workflow inputs - pre-populate values for awf run
# Uncomment and modify the examples below:
inputs:
  # project: my-project
  # environment: staging
  # debug: false
`, 0o644)
}

func createExampleScript(path string, force bool) error {
	return writeFileUnlessExists(path, force, `#!/usr/bin/env bash
# Example script created by awf init
# This script demonstrates the script_file feature (B009).
# Reference it in a workflow step:
#
#   states:
#     run_script:
#       type: step
#       script_file: "{{.awf.scripts_dir}}/example.sh"
#       on_success: done

echo "Hello from AWF script!"
`, 0o755)
}

func createExamplePrompt(path string, force bool) error {
	return writeFileUnlessExists(path, force, `# Example Prompt

This is an example prompt file created by AWF.

## Usage

Reference this prompt in workflow inputs using the @prompts/ prefix:

`+"```"+`bash
awf run my-workflow --input prompt=@prompts/example.md
`+"```"+`

## Template Variables

You can use template variables in your workflow commands:

- `+"`{{inputs.prompt}}`"+` - The content of this file

## Tips

- Store reusable AI prompts here (system prompts, task templates)
- Use .md for markdown or .txt for plain text
- Organize complex prompts in subdirectories (e.g., ai/agents/)
`, 0o644)
}
