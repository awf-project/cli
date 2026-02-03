package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/infrastructure/repository"
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
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// Create composite repository with XDG paths
	repo := NewWorkflowRepository()

	// List workflows with source info
	infos, err := repo.ListWithSource(ctx)
	if err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitUser)
		}
		return writeErrorAndExit(writer, err, ExitUser)
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

		if loadErr == nil {
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
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	// Get all prompt paths in priority order (local first, then global)
	promptPaths := BuildPromptPaths()

	// Collect prompts from all paths, deduplicating by name (first wins = local wins)
	prompts, err := collectPromptsFromPaths(promptPaths)
	if err != nil {
		return fmt.Errorf("failed to scan prompts directories: %w", err)
	}

	if len(prompts) == 0 {
		if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet {
			return writer.WritePrompts([]ui.PromptInfo{})
		}
		formatter.Info("No prompts found")
		formatter.Info("Search paths:")
		for _, sp := range promptPaths {
			formatter.Info(fmt.Sprintf("  - %s (%s)", sp.Path, sp.Source))
		}
		formatter.Info("Run 'awf init' to create the prompts directory")
		return nil
	}

	// Sort prompts by source then name
	sort.Slice(prompts, func(i, j int) bool {
		if prompts[i].Source != prompts[j].Source {
			return prompts[i].Source < prompts[j].Source
		}
		return prompts[i].Name < prompts[j].Name
	})

	return writer.WritePrompts(prompts)
}

// collectPromptsFromPaths walks multiple prompt directories and returns deduplicated prompts.
// Earlier paths take precedence (local wins over global for same-named prompts).
// shouldProcessEntry determines if a directory entry should be processed.
// Returns (process=true, skipDir=false) for files to process.
// Returns (process=false, skipDir=false) to skip the entry.
// Returns (process=false, skipDir=true) to skip entire directory subtrees.
func shouldProcessEntry(d fs.DirEntry, err error) (process, skipDir bool) {
	if err != nil {
		return false, false // Skip entries with errors
	}
	if d.IsDir() {
		return false, false // Skip directories themselves
	}
	return true, false // Process regular files
}

// buildPromptInfo constructs a PromptInfo from a file entry.
// Returns nil if the entry should be skipped (e.g., already seen, errors).
func buildPromptInfo(path string, d fs.DirEntry, basePath, source string, seen map[string]struct{}) (*ui.PromptInfo, error) {
	// Calculate relative name from base path
	relName, err := filepath.Rel(basePath, path)
	if err != nil {
		return nil, nil // Skip if paths are not related
	}

	// Skip if relative path goes outside base directory (starts with "..")
	if len(relName) >= 2 && relName[0] == '.' && relName[1] == '.' {
		return nil, nil
	}

	// Skip if already seen (earlier path wins)
	if seen != nil {
		if _, exists := seen[relName]; exists {
			return nil, nil
		}
		seen[relName] = struct{}{}
	}

	// Get file info for size and mod time
	fileInfo, err := d.Info()
	if err != nil {
		return nil, err
	}

	return &ui.PromptInfo{
		Name:    relName,
		Source:  source,
		Path:    path,
		Size:    fileInfo.Size(),
		ModTime: fileInfo.ModTime().Format("2006-01-02 15:04:05"),
	}, nil
}

func collectPromptsFromPaths(paths []repository.SourcedPath) ([]ui.PromptInfo, error) {
	if len(paths) == 0 {
		return []ui.PromptInfo{}, nil
	}

	// Track seen prompt names for deduplication (first wins)
	seen := make(map[string]struct{})
	var prompts []ui.PromptInfo

	for _, sp := range paths {
		basePath := filepath.Clean(sp.Path)

		// Skip non-existent directories
		info, err := os.Stat(basePath)
		if err != nil || !info.IsDir() {
			continue
		}

		// Walk directory tree
		err = filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
			// Check if we should process this entry
			process, _ := shouldProcessEntry(d, err)
			if !process {
				return nil
			}

			// Build prompt info using extracted helper
			promptInfo, err := buildPromptInfo(path, d, basePath, sp.Source.String(), seen)
			if err != nil {
				return fmt.Errorf("building prompt info for %s: %w", path, err)
			}
			if promptInfo != nil {
				prompts = append(prompts, *promptInfo)
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking %s: %w", basePath, err)
		}
	}

	return prompts, nil
}
