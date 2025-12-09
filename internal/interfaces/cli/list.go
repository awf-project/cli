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

	// Create composite repository with XDG paths
	repo := NewWorkflowRepository()

	// List workflows with source info
	infos, err := repo.ListWithSource(ctx)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	// Create formatter
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	if len(infos) == 0 {
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

	// Load workflow details for table
	headers := []string{"NAME", "SOURCE", "VERSION", "DESCRIPTION"}
	rows := make([][]string, 0, len(infos))

	for _, info := range infos {
		wf, err := repo.Load(ctx, info.Name)
		if err != nil || wf == nil {
			rows = append(rows, []string{info.Name, info.Source.String(), "-", "(error loading)"})
			continue
		}

		version := wf.Version
		if version == "" {
			version = "-"
		}

		desc := wf.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		if desc == "" {
			desc = "-"
		}

		rows = append(rows, []string{info.Name, info.Source.String(), version, desc})
	}

	formatter.Table(headers, rows)

	if cfg.Verbose {
		formatter.Debug(fmt.Sprintf("\nFound %d workflow(s)", len(infos)))
	}

	return nil
}
