package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/infrastructure/config"
	"github.com/vanoix/awf/internal/infrastructure/xdg"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

// newConfigCommand creates the config command group.
func newConfigCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage project configuration",
		Long: `Manage AWF project configuration.

The project configuration file (.awf/config.yaml) provides default values
for workflow inputs that can be overridden by CLI flags.`,
	}

	cmd.AddCommand(newConfigShowCommand(cfg))
	return cmd
}

// newConfigShowCommand creates the 'config show' subcommand.
func newConfigShowCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display current project configuration",
		Long: `Display the current project configuration values.

Shows all configured inputs from .awf/config.yaml that will be
pre-populated when running workflows.

Examples:
  awf config show
  awf config show --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigShow(cmd, cfg)
		},
	}
}

// runConfigShow displays the project configuration.
func runConfigShow(cmd *cobra.Command, cfg *Config) error {
	configPath := xdg.LocalConfigPath()
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// Check if config file exists
	_, err := os.Stat(configPath)
	configExists := err == nil

	// Load config (returns empty config if file doesn't exist)
	loader := config.NewYAMLConfigLoader(configPath)
	projectCfg, err := loader.Load()
	if err != nil {
		// Invalid YAML or read error
		if writer.IsJSONFormat() {
			return fmt.Errorf("write error: %w", writer.WriteError(err, ExitUser))
		}
		return fmt.Errorf("config error: %w", err)
	}

	// Handle different output formats
	switch cfg.OutputFormat {
	case ui.FormatJSON:
		return writeConfigJSON(writer, configPath, configExists, projectCfg)
	case ui.FormatQuiet:
		return writeConfigQuiet(cmd, projectCfg)
	case ui.FormatTable:
		return writeConfigTable(cmd, configPath, configExists, projectCfg, cfg.NoColor)
	default: // text
		return writeConfigText(cmd, configPath, configExists, projectCfg, cfg.NoColor)
	}
}

// ConfigShowOutput represents the structured output for 'config show' command.
// Used for JSON format output.
type ConfigShowOutput struct {
	Path   string         `json:"path"`
	Exists bool           `json:"exists"`
	Inputs map[string]any `json:"inputs,omitempty"`
}

// writeConfigJSON outputs config as JSON.
func writeConfigJSON(writer *ui.OutputWriter, configPath string, exists bool, projectCfg *config.ProjectConfig) error {
	output := ConfigShowOutput{
		Path:   configPath,
		Exists: exists,
	}
	if exists && len(projectCfg.Inputs) > 0 {
		output.Inputs = projectCfg.Inputs
	}
	return writer.WriteJSON(output)
}

// writeConfigQuiet outputs just the input keys, one per line.
func writeConfigQuiet(cmd *cobra.Command, projectCfg *config.ProjectConfig) error {
	if len(projectCfg.Inputs) == 0 {
		return nil
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(projectCfg.Inputs))
	for k := range projectCfg.Inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintln(cmd.OutOrStdout(), k)
	}
	return nil
}

// writeConfigTable outputs config as a bordered table.
func writeConfigTable(cmd *cobra.Command, configPath string, exists bool, projectCfg *config.ProjectConfig, noColor bool) error {
	out := cmd.OutOrStdout()

	if !exists {
		displayNoConfigFound(ui.NewFormatter(out, ui.FormatOptions{NoColor: noColor}))
		return nil
	}

	// Header
	fmt.Fprintf(out, "Config: %s\n\n", configPath)

	if len(projectCfg.Inputs) == 0 {
		fmt.Fprintln(out, "No inputs configured")
		return nil
	}

	// Table header
	fmt.Fprintln(out, "+----------------------+------------------------------+")
	fmt.Fprintln(out, "| KEY                  | VALUE                        |")
	fmt.Fprintln(out, "+----------------------+------------------------------+")

	// Sort keys for consistent output
	keys := make([]string, 0, len(projectCfg.Inputs))
	for k := range projectCfg.Inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := projectCfg.Inputs[k]
		valueStr := fmt.Sprintf("%v", v)
		if len(valueStr) > 28 {
			valueStr = valueStr[:25] + "..."
		}
		if len(k) > 20 {
			k = k[:17] + "..."
		}
		fmt.Fprintf(out, "| %-20s | %-28s |\n", k, valueStr)
	}
	fmt.Fprintln(out, "+----------------------+------------------------------+")

	return nil
}

// writeConfigText outputs config in human-readable text format.
func writeConfigText(cmd *cobra.Command, configPath string, exists bool, projectCfg *config.ProjectConfig, noColor bool) error {
	out := cmd.OutOrStdout()
	formatter := ui.NewFormatter(out, ui.FormatOptions{NoColor: noColor})

	if !exists {
		displayNoConfigFound(formatter)
		return nil
	}

	displayConfigShowText(formatter, configPath, projectCfg)
	return nil
}

// displayConfigShowText renders config in human-readable text format.
func displayConfigShowText(formatter *ui.Formatter, configPath string, projectCfg *config.ProjectConfig) {
	color := formatter.Colorizer()

	formatter.Printf("Config: %s\n", color.Bold(configPath))
	formatter.Println()

	if len(projectCfg.Inputs) == 0 {
		formatter.Println("No inputs configured")
		return
	}

	formatter.Println(color.Bold("Inputs:"))

	// Sort keys for consistent output
	keys := make([]string, 0, len(projectCfg.Inputs))
	for k := range projectCfg.Inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := projectCfg.Inputs[k]
		formatter.Printf("  %s: %v\n", k, v)
	}
}

// displayNoConfigFound renders the message when no config file exists.
func displayNoConfigFound(formatter *ui.Formatter) {
	formatter.Println("No project configuration found")
	formatter.Println()
	formatter.Println("Run 'awf init' to create .awf/config.yaml")
}
