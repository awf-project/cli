package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/infrastructure/repository"
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

	// Determine workflows directory
	workflowsPath := getWorkflowsPath(cfg)

	// Create repository
	repo := repository.NewYAMLRepository(workflowsPath)

	// Create service (minimal, just for listing)
	svc := application.NewWorkflowService(repo, nil, nil, nil)

	// List workflows
	names, err := svc.ListWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	// Create formatter
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	if len(names) == 0 {
		formatter.Info("No workflows found in " + workflowsPath)
		return nil
	}

	// Sort names
	sort.Strings(names)

	// Load workflow details for table
	headers := []string{"NAME", "VERSION", "DESCRIPTION"}
	rows := make([][]string, 0, len(names))

	for _, name := range names {
		wf, err := repo.Load(ctx, name)
		if err != nil || wf == nil {
			rows = append(rows, []string{name, "-", "(error loading)"})
			continue
		}

		version := wf.Version
		if version == "" {
			version = "-"
		}

		desc := wf.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		if desc == "" {
			desc = "-"
		}

		rows = append(rows, []string{name, version, desc})
	}

	formatter.Table(headers, rows)

	if cfg.Verbose {
		formatter.Debug(fmt.Sprintf("\nFound %d workflow(s) in %s", len(names), workflowsPath))
	}

	return nil
}

func getWorkflowsPath(cfg *Config) string {
	// Check environment variable first
	if envPath := os.Getenv("AWF_WORKFLOWS_PATH"); envPath != "" {
		return envPath
	}

	// Check if ./configs/workflows exists (local project)
	if _, err := os.Stat("configs/workflows"); err == nil {
		return "configs/workflows"
	}

	// Default to storage path
	return filepath.Join(cfg.StoragePath, "workflows")
}
