package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/infrastructure/diagram"
)

// diagramOptions holds command-specific flags for the diagram command.
type diagramOptions struct {
	Output    string // file path for image export (empty = stdout)
	Direction string // graph layout direction: TB, LR, BT, RL
	Highlight string // step name to visually emphasize
}

// newDiagramCommand creates the diagram subcommand.
// Per Step 4 of implementation plan: Create CLI command with flags.
func newDiagramCommand(cfg *Config) *cobra.Command {
	opts := &diagramOptions{}

	cmd := &cobra.Command{
		Use:   "diagram <workflow>",
		Short: "Generate workflow diagram in DOT format",
		Long: `Generate a visual diagram of a workflow in DOT (Graphviz) format.

By default, outputs DOT format to stdout for piping to graphviz.
Use --output to export directly to an image file (requires graphviz).

Step types are rendered with distinct shapes:
  - command     → box
  - parallel    → diamond (with branch subgraph)
  - terminal    → oval (doubleoval for failure)
  - for_each    → hexagon
  - while       → hexagon
  - operation   → box3d
  - call_workflow → folder

Examples:
  awf diagram my-workflow                     # Output DOT to stdout
  awf diagram my-workflow > workflow.dot      # Save DOT to file
  awf diagram my-workflow | dot -Tpng -o out.png  # Pipe to graphviz
  awf diagram my-workflow --output workflow.png   # Export to PNG directly
  awf diagram my-workflow --direction LR          # Left-to-right layout
  awf diagram my-workflow --highlight build_step  # Emphasize a step`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiagram(cmd, cfg, args[0], opts)
		},
	}

	// Flags per FR-006, FR-007, US3
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "Export to image file (format from extension: .png, .svg, .pdf, .dot)")
	cmd.Flags().StringVar(&opts.Direction, "direction", "TB", "Graph layout direction (TB, LR, BT, RL)")
	cmd.Flags().StringVar(&opts.Highlight, "highlight", "", "Step name to visually emphasize")

	return cmd
}

// runDiagram executes the diagram generation.
// Follows the validate.go pattern: load workflow via repository, generate DOT, output.
func runDiagram(cmd *cobra.Command, _ *Config, workflowName string, opts *diagramOptions) error {
	ctx := context.Background()

	// Validate direction option
	direction := diagram.Direction(opts.Direction)
	if !direction.IsValid() {
		return fmt.Errorf("invalid direction %q: must be one of TB, LR, BT, RL", opts.Direction)
	}

	// Load workflow via repository
	repo := NewWorkflowRepository()
	wf, err := repo.Load(ctx, workflowName)
	if err != nil {
		return fmt.Errorf("failed to load workflow: %w", err)
	}
	if wf == nil {
		return fmt.Errorf("workflow not found: %s", workflowName)
	}

	// Build diagram config from CLI options
	config := &diagram.DiagramConfig{
		Direction:  direction,
		OutputPath: opts.Output,
		Highlight:  opts.Highlight,
		ShowLabels: true,
	}

	// Generate DOT output
	generator := diagram.NewGenerator(config)
	dotOutput := generator.Generate(wf)

	// Handle output: file or stdout
	if opts.Output == "" {
		// Output to stdout
		_, err = fmt.Fprint(cmd.OutOrStdout(), dotOutput)
		return fmt.Errorf("write DOT output: %w", err)
	}

	// Output to file - check extension for format
	ext := strings.ToLower(filepath.Ext(opts.Output))

	if ext == ".dot" {
		// Write DOT directly to file
		if err := os.WriteFile(opts.Output, []byte(dotOutput), 0o600); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		return nil
	}

	// Image export requires graphviz
	if !diagram.CheckGraphviz() {
		return fmt.Errorf("graphviz not installed: 'dot' command not found in PATH. Install graphviz to export to %s format", ext)
	}

	if err := diagram.Export(dotOutput, opts.Output); err != nil {
		return fmt.Errorf("export diagram: %w", err)
	}
	return nil
}
