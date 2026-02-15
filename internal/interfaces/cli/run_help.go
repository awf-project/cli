package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/interfaces/cli/ui"
	"github.com/spf13/cobra"
)

// RenderWorkflowHelp renders workflow-specific help output including workflow
// description and input parameters table. This is called when a user runs
// `awf run <workflow> --help` to display dynamic help for the specified workflow.
//
// The output follows Cobra help conventions while adding workflow-specific content:
// - Workflow description (if present)
// - Input parameters table with NAME, TYPE, REQUIRED, DEFAULT, DESCRIPTION columns
func RenderWorkflowHelp(cmd *cobra.Command, wf *workflow.Workflow, out io.Writer, noColor bool) error {
	if wf == nil {
		return fmt.Errorf("workflow is nil")
	}

	// Show workflow description if present (US4)
	if wf.Description != "" {
		_, _ = fmt.Fprintf(out, "Description: %s\n\n", wf.Description)
	}

	// Convert domain inputs to UI InputInfo
	inputs := workflowToInputInfos(wf)

	// Render inputs table
	return formatInputsTable(inputs, out, noColor)
}

// workflowToInputInfos converts domain workflow inputs to UI InputInfo structs.
// This provides the mapping layer between domain and presentation layers.
func workflowToInputInfos(wf *workflow.Workflow) []ui.InputInfo {
	if wf == nil || len(wf.Inputs) == 0 {
		return []ui.InputInfo{}
	}

	infos := make([]ui.InputInfo, len(wf.Inputs))
	for i, input := range wf.Inputs {
		infos[i] = ui.InputInfo{
			Name:        input.Name,
			Type:        input.Type,
			Required:    input.Required,
			Default:     formatAnyToString(input.Default),
			Description: input.Description,
		}
	}
	return infos
}

// formatAnyToString converts any type to its string representation.
func formatAnyToString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

// formatInputsTable renders the input parameters as a formatted table.
// Uses tabwriter for aligned columns in 80-column terminals.
func formatInputsTable(inputs []ui.InputInfo, out io.Writer, noColor bool) error {
	if len(inputs) == 0 {
		_, _ = fmt.Fprintln(out, "No input parameters")
		return nil
	}

	_, _ = fmt.Fprintln(out, "Input Parameters:")

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	// Write header
	_, _ = fmt.Fprintln(tw, "  NAME\tTYPE\tREQUIRED\tDEFAULT\tDESCRIPTION")

	// Write each input row
	for _, input := range inputs {
		_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\n",
			input.Name,
			input.Type,
			formatRequired(input.Required),
			formatDefaultValue(input.Default),
			formatDescription(input.Description),
		)
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush table writer: %w", err)
	}
	return nil
}

// formatDefaultValue formats the default value for display.
// Returns the value as-is for non-empty defaults, "-" otherwise.
func formatDefaultValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// formatRequired formats the required field for display.
// Returns "yes" for required inputs, "no" otherwise.
func formatRequired(required bool) string {
	if required {
		return "yes"
	}
	return "no"
}

// formatDescription formats the description for display.
// Returns "No description" placeholder for empty descriptions.
func formatDescription(description string) string {
	trimmed := strings.TrimSpace(description)
	if trimmed == "" {
		return "No description"
	}
	return trimmed
}
