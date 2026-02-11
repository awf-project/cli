// Package ui provides user interface components for the AWF CLI.
//
// The ui package implements output formatting, interactive prompts,
// input collection, and visual presentation for the command-line interface.
//
// # Output Formatting
//
// The package supports multiple output formats (text, JSON, table, quiet)
// via OutputWriter, which handles workflow listings, execution status,
// validation results, and error display:
//
//	writer := ui.NewOutputWriter(os.Stdout, ui.FormatTable)
//	writer.WriteExecution(executionInfo)
//	writer.WriteError(err)
//
// # Color System
//
// Colorizer provides semantic color coding with automatic detection
// of terminal capabilities:
//
//	c := ui.NewColorizer()
//	fmt.Println(c.Success("Workflow completed"))
//	fmt.Println(c.Error("Step failed"))
//
// # Interactive Prompts
//
// CLIPrompt implements step-by-step interactive execution with
// step details, context display, and action selection (execute/skip/abort):
//
//	prompt := ui.NewCLIPrompt()
//	prompt.ShowStepDetails(step)
//	action, err := prompt.PromptAction()
//
// # Input Collection
//
// CLIInputCollector handles runtime input prompts with type coercion
// and validation:
//
//	collector := ui.NewCLIInputCollector(schema)
//	values, err := collector.PromptForInput()
//
// # Dry Run Formatting
//
// DryRunFormatter generates human-readable previews of workflow
// execution without running commands:
//
//	formatter := ui.NewDryRunFormatter()
//	formatter.Format(workflow)
//
// The package integrates with the CLI layer to provide consistent
// visual presentation across all AWF commands.
package ui
