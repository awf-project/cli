package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/spf13/cobra"
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
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// Route through WorkflowFacade when wired (T069). The facade's List exposes
	// the same discovery (local/global/env/packs) the legacy path used, mapped to
	// lightweight summaries; rendering is shared via writeWorkflowEntries.
	if cfg.Facade != nil {
		summaries, err := cfg.Facade.List(ctx)
		if err != nil {
			if writer.IsJSONFormat() {
				return writer.WriteError(err, ExitUser)
			}
			return writeErrorAndExit(writer, err, ExitUser)
		}
		entries := make([]workflowEntry, len(summaries))
		for i := range summaries {
			entries[i] = workflowEntry{
				Name:        summaries[i].Name,
				Version:     summaries[i].Version,
				Description: summaries[i].Description,
			}
		}
		return writeWorkflowEntries(cmd, cfg, writer, entries)
	}

	// Facade not wired: return a meaningful error stub. Production always wires
	// the facade via NewRootCommandAutoFacade, so this branch is never reached in
	// normal usage.
	err := fmt.Errorf("list requires facade wiring (use NewRootCommandAutoFacade)")
	if writer.IsJSONFormat() {
		return writer.WriteError(err, ExitSystem)
	}
	return writeErrorAndExit(writer, err, ExitSystem)
}

// workflowEntry is a rendering-neutral view of a discovered workflow, populated
// from either the facade summary (T069) or the legacy WorkflowService entry.
type workflowEntry struct {
	Name        string
	Source      string
	Version     string
	Description string
}

// writeWorkflowEntries renders discovered workflows identically for both the
// facade and legacy paths: empty-list "Search paths" guidance, the workflow
// table/JSON, and the verbose footer.
func writeWorkflowEntries(cmd *cobra.Command, cfg *Config, writer *ui.OutputWriter, entries []workflowEntry) error {
	if len(entries) == 0 {
		if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet {
			return writer.WriteWorkflows([]ui.WorkflowInfo{})
		}
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

	workflows := make([]ui.WorkflowInfo, len(entries))
	for i, e := range entries {
		workflows[i] = ui.WorkflowInfo{
			Name:        e.Name,
			Source:      e.Source,
			Version:     e.Version,
			Description: e.Description,
		}
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
		formatter.Debug(fmt.Sprintf("\nFound %d workflow(s)", len(entries)))
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
			if err != nil || d.IsDir() {
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
