package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

func newListCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List available workflows or prompts",
		Long:    "Display all available workflows from the configured workflows directory.",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, cfg)
		},
	}

	// Add prompts subcommand
	cmd.AddCommand(newListPromptsCommand(cfg))

	return cmd
}

func runList(cmd *cobra.Command, cfg *Config) error {
	ctx := context.Background()

	// Create output writer
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)

	// Create composite repository with XDG paths
	repo := NewWorkflowRepository()

	// List workflows with source info
	infos, err := repo.ListWithSource(ctx)
	if err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitUser)
		}
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	if len(infos) == 0 {
		// For JSON/quiet, output empty result
		if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet {
			return writer.WriteWorkflows([]ui.WorkflowInfo{})
		}
		// For text/table, show helpful message
		formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
			Verbose: cfg.Verbose,
			Quiet:   cfg.Quiet,
			NoColor: cfg.NoColor,
		})
		formatter.Info("No workflows found")
		formatter.Info("Search paths:")
		for _, sp := range BuildWorkflowPaths() {
			formatter.Info(fmt.Sprintf("  - %s (%s)", sp.Path, sp.Source))
		}
		return nil
	}

	// Sort by source then name
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Source != infos[j].Source {
			return infos[i].Source < infos[j].Source
		}
		return infos[i].Name < infos[j].Name
	})

	// Build workflow list
	workflows := make([]ui.WorkflowInfo, 0, len(infos))
	for _, info := range infos {
		wf, loadErr := repo.Load(ctx, info.Name)

		wfInfo := ui.WorkflowInfo{
			Name:   info.Name,
			Source: info.Source.String(),
		}

		if loadErr == nil && wf != nil {
			wfInfo.Version = wf.Version
			wfInfo.Description = wf.Description
		}

		workflows = append(workflows, wfInfo)
	}

	if err := writer.WriteWorkflows(workflows); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	if cfg.Verbose && !writer.IsJSONFormat() {
		formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
			Verbose: cfg.Verbose,
			Quiet:   cfg.Quiet,
			NoColor: cfg.NoColor,
		})
		formatter.Debug(fmt.Sprintf("\nFound %d workflow(s)", len(infos)))
	}

	return nil
}

func newListPromptsCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "prompts",
		Short: "List available prompt files",
		Long: `Display all available prompt files from the .awf/prompts/ directory.

Prompts are reusable text templates that can be referenced in workflow inputs
using the @prompts/ prefix (e.g., --input prompt=@prompts/my-prompt.md).

Examples:
  awf list prompts`,
		Aliases: []string{"p"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListPrompts(cmd, cfg)
		},
	}
}

func runListPrompts(cmd *cobra.Command, cfg *Config) error {
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	promptsDir := ".awf/prompts"

	// Check if prompts directory exists
	if _, err := os.Stat(promptsDir); os.IsNotExist(err) {
		if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet {
			return writer.WritePrompts([]ui.PromptInfo{})
		}
		formatter.Info("No prompts directory found")
		formatter.Info("Run 'awf init' to create the prompts directory")
		return nil
	}

	// Walk the prompts directory to find all prompt files
	var prompts []ui.PromptInfo
	err := filepath.WalkDir(promptsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Get relative path from prompts directory
		relPath, err := filepath.Rel(promptsDir, path)
		if err != nil {
			return err
		}

		// Get file info for size and mod time
		info, err := d.Info()
		if err != nil {
			return err
		}

		prompts = append(prompts, ui.PromptInfo{
			Name:    relPath,
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan prompts directory: %w", err)
	}

	if len(prompts) == 0 {
		if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet {
			return writer.WritePrompts([]ui.PromptInfo{})
		}
		formatter.Info("No prompts found in .awf/prompts/")
		return nil
	}

	// Sort prompts by name
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].Name < prompts[j].Name
	})

	return writer.WritePrompts(prompts)
}
