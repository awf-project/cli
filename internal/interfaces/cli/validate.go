package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

func newValidateCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <workflow>",
		Short: "Validate a workflow definition",
		Long: `Validate a workflow YAML file without executing it.

Checks for:
  - Valid YAML syntax
  - Required fields (name, initial state, terminal state)
  - Valid state references
  - No cycles in state transitions (if detectable)

Examples:
  awf validate my-workflow
  awf validate analyze-code --verbose`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd, cfg, args[0])
		},
	}
}

func runValidate(cmd *cobra.Command, cfg *Config, workflowName string) error {
	ctx := context.Background()

	// Create output writer
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)

	// Initialize repository
	repo := NewWorkflowRepository()

	// Create service
	svc := application.NewWorkflowService(repo, nil, nil, nil)

	// Load workflow first to check existence
	wf, err := svc.GetWorkflow(ctx, workflowName)
	if err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitUser)
		}
		return fmt.Errorf("failed to load workflow: %w", err)
	}
	if wf == nil {
		err := fmt.Errorf("workflow not found: %s", workflowName)
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitUser)
		}
		return err
	}

	// Validate
	validationErr := svc.ValidateWorkflow(ctx, workflowName)

	// JSON/quiet format
	if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet {
		result := ui.ValidationResult{
			Valid:    validationErr == nil,
			Workflow: workflowName,
		}
		if validationErr != nil {
			result.Errors = []string{validationErr.Error()}
		}
		if err := writer.WriteValidation(result); err != nil {
			return err
		}
		if validationErr != nil {
			return &exitError{code: ExitWorkflow, err: validationErr}
		}
		return nil
	}

	// Text/table format
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	if validationErr != nil {
		formatter.Error(fmt.Sprintf("Validation failed: %s", validationErr))
		return &exitError{code: ExitWorkflow, err: validationErr}
	}

	// Show success
	formatter.Success(fmt.Sprintf("Workflow '%s' is valid", workflowName))

	// Verbose output
	if cfg.Verbose {
		formatter.Println()
		formatter.Printf("Name:        %s\n", wf.Name)
		if wf.Version != "" {
			formatter.Printf("Version:     %s\n", wf.Version)
		}
		if wf.Description != "" {
			formatter.Printf("Description: %s\n", wf.Description)
		}
		formatter.Printf("Initial:     %s\n", wf.Initial)
		formatter.Printf("Steps:       %d\n", len(wf.Steps))

		if len(wf.Inputs) > 0 {
			formatter.Println()
			formatter.Println("Inputs:")
			for _, input := range wf.Inputs {
				required := ""
				if input.Required {
					required = " (required)"
				}
				formatter.Printf("  - %s: %s%s\n", input.Name, input.Type, required)
			}
		}
	}

	return nil
}
