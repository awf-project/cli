package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

func newListCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available workflows",
		Long:  "Display all available workflows from the configured workflows directory.",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, cfg)
		},
	}
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
