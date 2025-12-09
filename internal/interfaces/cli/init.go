package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/infrastructure/xdg"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

func newInitCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize AWF in the current directory",
		Long: `Initialize AWF in the current directory by creating the local workflows directory.

This creates:
  .awf/workflows/           Local workflows directory
  .awf/workflows/example.yaml  Example workflow file

Examples:
  awf init`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, cfg)
		},
	}
}

func runInit(cmd *cobra.Command, cfg *Config) error {
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	workflowsDir := xdg.LocalWorkflowsDir()

	// Check if directory already exists
	if _, err := os.Stat(workflowsDir); err == nil {
		formatter.Info(fmt.Sprintf("Directory '%s' already exists", workflowsDir))
		return nil
	}

	// Create directory
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	formatter.Success(fmt.Sprintf("Created %s", workflowsDir))

	// Create example workflow
	examplePath := filepath.Join(workflowsDir, "example.yaml")
	exampleContent := `name: example
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

	if err := os.WriteFile(examplePath, []byte(exampleContent), 0644); err != nil {
		return fmt.Errorf("failed to create example workflow: %w", err)
	}

	formatter.Success(fmt.Sprintf("Created %s", examplePath))
	formatter.Info("\nRun 'awf list' to see available workflows")
	formatter.Info("Run 'awf run example' to execute the example workflow")

	return nil
}
