package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// OutputFormat defines the output format for CLI commands.
type OutputFormat int

const (
	FormatText  OutputFormat = iota // default: human-readable
	FormatJSON                      // JSON output
	FormatTable                     // tabular output
	FormatQuiet                     // minimal: IDs/names only
)

func (f OutputFormat) String() string {
	switch f {
	case FormatText:
		return "text"
	case FormatJSON:
		return "json"
	case FormatTable:
		return "table"
	case FormatQuiet:
		return "quiet"
	default:
		return "unknown"
	}
}

// ParseOutputFormat parses a string to OutputFormat.
func ParseOutputFormat(s string) (OutputFormat, error) {
	switch s {
	case "text", "":
		return FormatText, nil
	case "json":
		return FormatJSON, nil
	case "table":
		return FormatTable, nil
	case "quiet":
		return FormatQuiet, nil
	default:
		return FormatText, fmt.Errorf("invalid output format: %s (valid: text, json, table, quiet)", s)
	}
}

// ErrorResponse is the JSON structure for errors.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

// WorkflowInfo is the JSON structure for list command.
type WorkflowInfo struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
}

// StepInfo is the JSON structure for step status.
type StepInfo struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Output      string `json:"output,omitempty"`
	Stderr      string `json:"stderr,omitempty"`
	ExitCode    int    `json:"exit_code,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	Error       string `json:"error,omitempty"`
}

// ExecutionInfo is the JSON structure for status command.
type ExecutionInfo struct {
	WorkflowID   string     `json:"workflow_id"`
	WorkflowName string     `json:"workflow_name"`
	Status       string     `json:"status"`
	CurrentStep  string     `json:"current_step,omitempty"`
	StartedAt    string     `json:"started_at,omitempty"`
	CompletedAt  string     `json:"completed_at,omitempty"`
	DurationMs   int64      `json:"duration_ms"`
	Steps        []StepInfo `json:"steps,omitempty"`
}

// RunResult is the JSON structure for run command.
type RunResult struct {
	WorkflowID string `json:"workflow_id"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// ValidationResult is the JSON structure for validate command.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Workflow string   `json:"workflow"`
	Errors   []string `json:"errors,omitempty"`
}

// OutputWriter handles structured output for different formats.
type OutputWriter struct {
	out       io.Writer
	errOut    io.Writer
	format    OutputFormat
	noColor   bool
	colorizer *Colorizer
}

// NewOutputWriter creates a writer for the specified format.
func NewOutputWriter(out, errOut io.Writer, format OutputFormat, noColor bool) *OutputWriter {
	return &OutputWriter{
		out:       out,
		errOut:    errOut,
		format:    format,
		noColor:   noColor,
		colorizer: NewColorizer(!noColor),
	}
}

// IsJSONFormat returns true if the output format is JSON.
func (w *OutputWriter) IsJSONFormat() bool {
	return w.format == FormatJSON
}

// WriteWorkflows outputs workflow list.
func (w *OutputWriter) WriteWorkflows(workflows []WorkflowInfo) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(workflows)
	case FormatTable:
		return w.writeWorkflowsTable(workflows)
	case FormatQuiet:
		for _, wf := range workflows {
			fmt.Fprintln(w.out, wf.Name)
		}
		return nil
	default: // text
		return w.writeWorkflowsTable(workflows)
	}
}

// WriteExecution outputs execution status.
func (w *OutputWriter) WriteExecution(exec ExecutionInfo) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(exec)
	case FormatQuiet:
		fmt.Fprintln(w.out, exec.Status)
		return nil
	default: // text, table
		return w.writeExecutionText(exec)
	}
}

// WriteRunResult outputs run command result.
func (w *OutputWriter) WriteRunResult(result RunResult) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(result)
	case FormatQuiet:
		fmt.Fprintln(w.out, result.WorkflowID)
		return nil
	default: // text, table
		return w.writeRunResultText(result)
	}
}

// WriteValidation outputs validation result.
func (w *OutputWriter) WriteValidation(result ValidationResult) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(result)
	case FormatQuiet:
		if result.Valid {
			fmt.Fprintln(w.out, "valid")
		} else {
			fmt.Fprintln(w.out, "invalid")
		}
		return nil
	default: // text, table
		return w.writeValidationText(result)
	}
}

// WriteError outputs an error in the appropriate format.
func (w *OutputWriter) WriteError(err error, code int) error {
	if w.format == FormatJSON {
		return w.writeJSON(ErrorResponse{
			Error: err.Error(),
			Code:  code,
		})
	}

	// Text format: write to errOut
	fmt.Fprintf(w.errOut, "Error: %s\n", err.Error())
	return nil
}

func (w *OutputWriter) writeJSON(v any) error {
	enc := json.NewEncoder(w.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (w *OutputWriter) writeWorkflowsTable(workflows []WorkflowInfo) error {
	tw := tabwriter.NewWriter(w.out, 0, 0, 2, ' ', 0)

	// Header
	fmt.Fprintln(tw, "NAME\tSOURCE\tVERSION\tDESCRIPTION")

	for _, wf := range workflows {
		desc := wf.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", wf.Name, wf.Source, wf.Version, desc)
	}

	return tw.Flush()
}

func (w *OutputWriter) writeExecutionText(exec ExecutionInfo) error {
	fmt.Fprintf(w.out, "Workflow: %s\n", exec.WorkflowName)
	fmt.Fprintf(w.out, "ID: %s\n", exec.WorkflowID)
	fmt.Fprintf(w.out, "Status: %s\n", w.colorizer.Status(exec.Status, exec.Status))

	if exec.CurrentStep != "" {
		fmt.Fprintf(w.out, "Current Step: %s\n", exec.CurrentStep)
	}
	if exec.DurationMs > 0 {
		fmt.Fprintf(w.out, "Duration: %dms\n", exec.DurationMs)
	}

	if len(exec.Steps) > 0 {
		fmt.Fprintln(w.out, "\nSteps:")
		for _, step := range exec.Steps {
			status := w.colorizer.Status(step.Status, step.Status)
			fmt.Fprintf(w.out, "  - %s: %s\n", step.Name, status)
			if step.Error != "" {
				fmt.Fprintf(w.out, "    Error: %s\n", step.Error)
			}
		}
	}

	return nil
}

func (w *OutputWriter) writeRunResultText(result RunResult) error {
	status := w.colorizer.Status(result.Status, result.Status)
	fmt.Fprintf(w.out, "Workflow %s in %dms\n", status, result.DurationMs)
	fmt.Fprintf(w.out, "Workflow ID: %s\n", result.WorkflowID)

	if result.Error != "" {
		fmt.Fprintf(w.out, "Error: %s\n", result.Error)
	}

	return nil
}

func (w *OutputWriter) writeValidationText(result ValidationResult) error {
	if result.Valid {
		fmt.Fprintf(w.out, "%s Workflow '%s' is valid\n", w.colorizer.Success("✓"), result.Workflow)
	} else {
		fmt.Fprintf(w.out, "%s Workflow '%s' is invalid\n", w.colorizer.Error("✗"), result.Workflow)
		for _, e := range result.Errors {
			fmt.Fprintf(w.out, "  - %s\n", e)
		}
	}
	return nil
}
