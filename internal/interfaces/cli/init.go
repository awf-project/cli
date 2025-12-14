package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/infrastructure/xdg"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

const (
	awfDir            = ".awf"
	workflowsDir      = "workflows"
	promptsDir        = "prompts"
	storageDir        = "storage"
	statesDir         = "states"
	logsDir           = "logs"
	configFileName    = ".awf.yaml"
	exampleFile       = "example.yaml"
	examplePromptFile = "example.md"
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
  .awf/workflows/              Local workflows directory
  .awf/workflows/example.yaml  Example workflow file
  .awf/prompts/                Prompt templates directory
  .awf/prompts/example.md      Example prompt file
  .awf/storage/states/         State persistence directory
  .awf/storage/logs/           Log files directory

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
	cmd.Flags().BoolVar(&global, "global", false, "Initialize global prompts directory")

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
		filepath.Join(awfPath, storageDir, statesDir),
		filepath.Join(awfPath, storageDir, logsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	formatter.Success(fmt.Sprintf("Created %s", awfPath))

	// Create config file
	if err := createConfigFile(configPath, force); err != nil {
		return err
	}
	formatter.Success(fmt.Sprintf("Created %s", configPath))

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

	// Check if already initialized
	if !force {
		if _, err := os.Stat(globalPromptsDir); err == nil {
			formatter.Info(fmt.Sprintf("Global prompts already initialized in '%s'", globalPromptsDir))
			formatter.Info("Use --force to reinitialize")
			return nil
		}
	}

	// Create global prompts directory
	if err := os.MkdirAll(globalPromptsDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", globalPromptsDir, err)
	}
	formatter.Success(fmt.Sprintf("Created %s", globalPromptsDir))

	// Create example prompt
	promptPath := filepath.Join(globalPromptsDir, examplePromptFile)
	if err := createExamplePrompt(promptPath, force); err != nil {
		return err
	}
	formatter.Success(fmt.Sprintf("Created %s", promptPath))

	return nil
}

func createConfigFile(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil // File exists, skip
		}
	}

	content := `# AWF Configuration
# https://github.com/vanoix/awf

version: "1"

# Default log level: debug, info, warn, error
log_level: info

# Output format: text, json, table, quiet
output_format: text
`
	return os.WriteFile(path, []byte(content), 0644)
}

func createExampleWorkflow(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil // File exists, skip
		}
	}

	content := `name: example
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
`
	return os.WriteFile(path, []byte(content), 0644)
}

func createExamplePrompt(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil // File exists, skip
		}
	}

	content := `# Example Prompt

This is an example prompt file created by AWF.

## Usage

Reference this prompt in workflow inputs using the @prompts/ prefix:

` + "```" + `bash
awf run my-workflow --input prompt=@prompts/example.md
` + "```" + `

## Template Variables

You can use template variables in your workflow commands:

- ` + "`{{inputs.prompt}}`" + ` - The content of this file

## Tips

- Store reusable AI prompts here (system prompts, task templates)
- Use .md for markdown or .txt for plain text
- Organize complex prompts in subdirectories (e.g., ai/agents/)
`
	return os.WriteFile(path, []byte(content), 0644)
}
