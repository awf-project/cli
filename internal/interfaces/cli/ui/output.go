package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	errfmt "github.com/awf-project/cli/internal/infrastructure/errors"
)

// Type aliases for error formatting interfaces
type (
	ErrorFormatter = ports.ErrorFormatter
)

// Factory functions for error formatters
var (
	NewHumanErrorFormatter = errfmt.NewHumanErrorFormatter
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
	Error     string `json:"error"`
	Code      int    `json:"code"`
	ErrorCode string `json:"error_code,omitempty"` // Hierarchical error code (e.g., "USER.INPUT.MISSING_FILE")
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

// ResumableInfo contains information for resumable workflow display.
type ResumableInfo struct {
	WorkflowID   string `json:"workflow_id"`
	WorkflowName string `json:"workflow_name"`
	Status       string `json:"status"`
	CurrentStep  string `json:"current_step"`
	UpdatedAt    string `json:"updated_at"`
	Progress     string `json:"progress"`
}

// RunResult is the JSON structure for run command.
type RunResult struct {
	WorkflowID string     `json:"workflow_id"`
	Status     string     `json:"status"`
	DurationMs int64      `json:"duration_ms"`
	Error      string     `json:"error,omitempty"`
	Steps      []StepInfo `json:"steps,omitempty"`
}

// ValidationResult is the JSON structure for validate command.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Workflow string   `json:"workflow"`
	Errors   []string `json:"errors,omitempty"`
}

// InputInfo represents workflow input for table output.
type InputInfo struct {
	Name        string
	Type        string
	Required    bool
	Default     string
	Description string
}

// StepSummary represents a workflow step summary for table output.
type StepSummary struct {
	Name string
	Type string
	Next string
}

// ValidationResultTable is the structure for validate command table output.
type ValidationResultTable struct {
	Valid    bool
	Workflow string
	Inputs   []InputInfo
	Steps    []StepSummary
	Errors   []string
}

// PromptInfo represents a prompt file for list prompts command.
type PromptInfo struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time,omitempty"`
}

// PluginInfo represents a plugin for list plugins command.
type PluginInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version,omitempty"`
	Description  string   `json:"description,omitempty"`
	Status       string   `json:"status"`
	Enabled      bool     `json:"enabled"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// tableWriter renders ASCII-bordered tables.
type tableWriter struct {
	w       io.Writer
	columns []int
}

func newTableWriter(w io.Writer, columns ...int) *tableWriter {
	return &tableWriter{w: w, columns: columns}
}

func (t *tableWriter) separator() {
	_, _ = fmt.Fprint(t.w, "+")
	for _, width := range t.columns {
		_, _ = fmt.Fprintf(t.w, "%s+", strings.Repeat("-", width+2))
	}
	_, _ = fmt.Fprintln(t.w)
}

func (t *tableWriter) row(cells ...string) {
	_, _ = fmt.Fprint(t.w, "|")
	for i, width := range t.columns {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		if len(cell) > width {
			cell = cell[:width-3] + "..."
		}
		_, _ = fmt.Fprintf(t.w, " %-*s |", width, cell)
	}
	_, _ = fmt.Fprintln(t.w)
}

func (t *tableWriter) fullWidthSeparator() {
	total := 1
	for _, width := range t.columns {
		total += width + 3
	}
	_, _ = fmt.Fprintf(t.w, "+%s+\n", strings.Repeat("-", total-2))
}

func (t *tableWriter) fullWidthRow(content string) {
	total := 1
	for _, width := range t.columns {
		total += width + 3
	}
	innerWidth := total - 4
	if len(content) > innerWidth {
		content = content[:innerWidth-3] + "..."
	}
	_, _ = fmt.Fprintf(t.w, "| %-*s |\n", innerWidth, content)
}

// OutputWriter handles structured output for different formats.
type OutputWriter struct {
	out       io.Writer
	errOut    io.Writer
	format    OutputFormat
	noColor   bool
	noHints   bool
	colorizer *Colorizer
}

// NewOutputWriter creates a writer for the specified format.
func NewOutputWriter(out, errOut io.Writer, format OutputFormat, noColor, noHints bool) *OutputWriter {
	return &OutputWriter{
		out:       out,
		errOut:    errOut,
		format:    format,
		noColor:   noColor,
		noHints:   noHints,
		colorizer: NewColorizer(!noColor),
	}
}

// IsJSONFormat returns true if the output format is JSON.
func (w *OutputWriter) IsJSONFormat() bool {
	return w.format == FormatJSON
}

// Out returns the output writer.
func (w *OutputWriter) Out() io.Writer {
	return w.out
}

// WriteJSON outputs any value as JSON.
func (w *OutputWriter) WriteJSON(v any) error {
	return w.writeJSON(v)
}

// WriteWorkflows outputs workflow list.
func (w *OutputWriter) WriteWorkflows(workflows []WorkflowInfo) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(workflows)
	case FormatTable:
		return w.writeWorkflowsBorderedTable(workflows)
	case FormatQuiet:
		for _, wf := range workflows {
			_, _ = fmt.Fprintln(w.out, wf.Name)
		}
		return nil
	default: // text
		return w.writeWorkflowsTable(workflows)
	}
}

// WriteExecution outputs execution status.
func (w *OutputWriter) WriteExecution(exec *ExecutionInfo) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(exec)
	case FormatTable:
		return w.writeExecutionTable(exec)
	case FormatQuiet:
		_, _ = fmt.Fprintln(w.out, exec.Status)
		return nil
	default: // text
		return w.writeExecutionText(exec)
	}
}

// WriteRunResult outputs run command result.
func (w *OutputWriter) WriteRunResult(result *RunResult) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(result)
	case FormatTable:
		return w.writeRunResultTable(result)
	case FormatQuiet:
		_, _ = fmt.Fprintln(w.out, result.WorkflowID)
		return nil
	default: // text
		return w.writeRunResultText(result)
	}
}

// WriteValidation outputs validation result.
func (w *OutputWriter) WriteValidation(result ValidationResult) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(result)
	case FormatTable:
		return w.writeValidationTable(result)
	case FormatQuiet:
		if result.Valid {
			_, _ = fmt.Fprintln(w.out, "valid")
		} else {
			_, _ = fmt.Fprintln(w.out, "invalid")
		}
		return nil
	default: // text
		return w.writeValidationText(result)
	}
}

// WriteValidationTable outputs validation result with detailed table.
func (w *OutputWriter) WriteValidationTable(result *ValidationResultTable) error {
	if w.format == FormatJSON {
		return w.writeJSON(result)
	}
	if w.format == FormatQuiet {
		if result.Valid {
			_, _ = fmt.Fprintln(w.out, "valid")
		} else {
			_, _ = fmt.Fprintln(w.out, "invalid")
		}
		return nil
	}
	return w.writeValidationResultTable(result)
}

// WriteError outputs an error in the appropriate format.
// Detects StructuredError and uses formatters for enhanced error output.
func (w *OutputWriter) WriteError(err error, code int) error {
	// Check if error is a StructuredError
	var structuredErr *domerrors.StructuredError
	if errors.As(err, &structuredErr) {
		// Use structured error handling
		return w.writeStructuredError(structuredErr, code)
	}

	// Fallback: legacy error handling for plain errors
	if w.format == FormatJSON {
		return w.writeJSON(ErrorResponse{
			Error: err.Error(),
			Code:  code,
		})
	}

	// Text format: write to errOut
	_, _ = fmt.Fprintf(w.errOut, "Error: %s\n", err.Error())
	return nil
}

// writeStructuredError handles formatting of StructuredError instances.
// Uses HumanErrorFormatter for text output and JSON for machine-readable output.
func (w *OutputWriter) writeStructuredError(err *domerrors.StructuredError, code int) error {
	if w.format == FormatJSON {
		return w.writeJSON(ErrorResponse{
			Error:     err.Error(),
			Code:      code,
			ErrorCode: string(err.Code),
		})
	}

	// Text format: use HumanErrorFormatter for rich output
	formatter := w.newHumanErrorFormatter()
	formatted := formatter.FormatError(err)
	_, _ = fmt.Fprintln(w.errOut, formatted)
	return nil
}

// newHumanErrorFormatter creates a HumanErrorFormatter using the output writer's color settings.
// Extracted as a method to facilitate testing and maintain separation of concerns.
//
// Includes all 5 hint generators for C048:
//   - FileNotFoundHintGenerator: "did you mean?" for missing files
//   - YAMLSyntaxHintGenerator: line/column references for syntax errors
//   - InvalidStateHintGenerator: closest state match suggestions
//   - MissingInputHintGenerator: required inputs with examples
//   - CommandFailureHintGenerator: permission/path verification guidance
func (w *OutputWriter) newHumanErrorFormatter() ErrorFormatter {
	return NewHumanErrorFormatter(!w.noColor, w.noHints,
		errfmt.FileNotFoundHintGenerator,
		errfmt.YAMLSyntaxHintGenerator,
		errfmt.InvalidStateHintGenerator,
		errfmt.MissingInputHintGenerator,
		errfmt.CommandFailureHintGenerator,
	)
}

// WritePrompts outputs prompt file list.
func (w *OutputWriter) WritePrompts(prompts []PromptInfo) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(prompts)
	case FormatTable:
		return w.writePromptsBorderedTable(prompts)
	case FormatQuiet:
		for _, p := range prompts {
			_, _ = fmt.Fprintln(w.out, p.Name)
		}
		return nil
	default: // text
		return w.writePromptsTable(prompts)
	}
}

// WritePlugins outputs plugin list.
func (w *OutputWriter) WritePlugins(plugins []PluginInfo) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(plugins)
	case FormatTable:
		return w.writePluginsBorderedTable(plugins)
	case FormatQuiet:
		for _, p := range plugins {
			_, _ = fmt.Fprintln(w.out, p.Name)
		}
		return nil
	default: // text
		return w.writePluginsTable(plugins)
	}
}

// WriteDryRun outputs the dry-run execution plan.
func (w *OutputWriter) WriteDryRun(plan *workflow.DryRunPlan, formatter *DryRunFormatter) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(plan)
	case FormatQuiet:
		_, _ = fmt.Fprintln(w.out, plan.WorkflowName)
		return nil
	default: // text and table use the formatter
		return formatter.Format(plan)
	}
}

// WriteResumableList outputs a list of resumable workflows.
func (w *OutputWriter) WriteResumableList(infos []ResumableInfo) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(infos)
	case FormatTable:
		return w.writeResumableBorderedTable(infos)
	case FormatQuiet:
		for _, info := range infos {
			_, _ = fmt.Fprintln(w.out, info.WorkflowID)
		}
		return nil
	default: // text
		return w.writeResumableTable(infos)
	}
}

func (w *OutputWriter) writeResumableTable(infos []ResumableInfo) error {
	if len(infos) == 0 {
		_, _ = fmt.Fprintln(w.out, "No resumable workflows found")
		return nil
	}

	tw := tabwriter.NewWriter(w.out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tWORKFLOW\tSTATUS\tCURRENT STEP\tPROGRESS\tUPDATED")
	for _, info := range infos {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			info.WorkflowID, info.WorkflowName, info.Status, info.CurrentStep, info.Progress, info.UpdatedAt)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flushing table: %w", err)
	}
	return nil
}

func (w *OutputWriter) writeResumableBorderedTable(infos []ResumableInfo) error {
	if len(infos) == 0 {
		_, _ = fmt.Fprintln(w.out, "No resumable workflows found")
		return nil
	}

	table := newTableWriter(w.out, 20, 15, 10, 15, 12, 20)
	table.separator()
	table.row("ID", "WORKFLOW", "STATUS", "CURRENT STEP", "PROGRESS", "UPDATED")
	table.separator()
	for _, info := range infos {
		table.row(info.WorkflowID, info.WorkflowName, info.Status, info.CurrentStep, info.Progress, info.UpdatedAt)
	}
	table.separator()
	return nil
}

func (w *OutputWriter) writePromptsTable(prompts []PromptInfo) error {
	tw := tabwriter.NewWriter(w.out, 0, 0, 2, ' ', 0)

	// Header
	_, _ = fmt.Fprintln(tw, "NAME\tSOURCE\tSIZE\tMODIFIED")

	for _, p := range prompts {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d B\t%s\n", p.Name, p.Source, p.Size, p.ModTime)
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flushing table: %w", err)
	}
	return nil
}

func (w *OutputWriter) writePromptsBorderedTable(prompts []PromptInfo) error {
	table := newTableWriter(w.out, 30, 8, 10, 16)

	table.separator()
	table.row("NAME", "SOURCE", "SIZE", "MODIFIED")
	table.separator()

	for _, p := range prompts {
		table.row(p.Name, p.Source, fmt.Sprintf("%d B", p.Size), p.ModTime)
	}
	table.separator()

	return nil
}

func (w *OutputWriter) writePluginsTable(plugins []PluginInfo) error {
	if len(plugins) == 0 {
		_, _ = fmt.Fprintln(w.out, "No plugins found")
		return nil
	}

	tw := tabwriter.NewWriter(w.out, 0, 0, 2, ' ', 0)

	// Header
	_, _ = fmt.Fprintln(tw, "NAME\tVERSION\tSTATUS\tENABLED\tCAPABILITIES")

	for _, p := range plugins {
		enabled := "yes"
		if !p.Enabled {
			enabled = "no"
		}
		caps := strings.Join(p.Capabilities, ", ")
		if caps == "" {
			caps = "-"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.Name, p.Version, p.Status, enabled, caps)
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flushing table: %w", err)
	}
	return nil
}

func (w *OutputWriter) writePluginsBorderedTable(plugins []PluginInfo) error {
	if len(plugins) == 0 {
		_, _ = fmt.Fprintln(w.out, "No plugins found")
		return nil
	}

	table := newTableWriter(w.out, 20, 10, 12, 8, 25)

	table.separator()
	table.row("NAME", "VERSION", "STATUS", "ENABLED", "CAPABILITIES")
	table.separator()

	for _, p := range plugins {
		enabled := "yes"
		if !p.Enabled {
			enabled = "no"
		}
		caps := strings.Join(p.Capabilities, ", ")
		if caps == "" {
			caps = "-"
		}
		table.row(p.Name, p.Version, p.Status, enabled, caps)
	}
	table.separator()

	return nil
}

func (w *OutputWriter) writeJSON(v any) error {
	enc := json.NewEncoder(w.out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

func (w *OutputWriter) writeWorkflowsTable(workflows []WorkflowInfo) error {
	tw := tabwriter.NewWriter(w.out, 0, 0, 2, ' ', 0)

	// Header
	_, _ = fmt.Fprintln(tw, "NAME\tSOURCE\tVERSION\tDESCRIPTION")

	for _, wf := range workflows {
		desc := wf.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", wf.Name, wf.Source, wf.Version, desc)
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flushing table: %w", err)
	}
	return nil
}

func (w *OutputWriter) writeWorkflowsBorderedTable(workflows []WorkflowInfo) error {
	table := newTableWriter(w.out, 20, 10, 10, 40)

	table.separator()
	table.row("NAME", "SOURCE", "VERSION", "DESCRIPTION")
	table.separator()

	for _, wf := range workflows {
		desc := wf.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		table.row(wf.Name, wf.Source, wf.Version, desc)
	}
	table.separator()

	return nil
}

func (w *OutputWriter) writeExecutionText(exec *ExecutionInfo) error {
	_, _ = fmt.Fprintf(w.out, "Workflow: %s\n", exec.WorkflowName)
	_, _ = fmt.Fprintf(w.out, "ID: %s\n", exec.WorkflowID)
	_, _ = fmt.Fprintf(w.out, "Status: %s\n", w.colorizer.Status(exec.Status, exec.Status))

	if exec.CurrentStep != "" {
		_, _ = fmt.Fprintf(w.out, "Current Step: %s\n", exec.CurrentStep)
	}
	if exec.DurationMs > 0 {
		_, _ = fmt.Fprintf(w.out, "Duration: %dms\n", exec.DurationMs)
	}

	if len(exec.Steps) > 0 {
		_, _ = fmt.Fprintln(w.out, "\nSteps:")
		for _, step := range exec.Steps {
			status := w.colorizer.Status(step.Status, step.Status)
			_, _ = fmt.Fprintf(w.out, "  - %s: %s\n", step.Name, status)
			if step.Error != "" {
				_, _ = fmt.Fprintf(w.out, "    Error: %s\n", step.Error)
			}
		}
	}

	return nil
}

func (w *OutputWriter) writeRunResultText(result *RunResult) error {
	status := w.colorizer.Status(result.Status, result.Status)
	_, _ = fmt.Fprintf(w.out, "Workflow %s in %dms\n", status, result.DurationMs)
	_, _ = fmt.Fprintf(w.out, "Workflow ID: %s\n", result.WorkflowID)

	if result.Error != "" {
		_, _ = fmt.Fprintf(w.out, "Error: %s\n", result.Error)
	}

	return nil
}

func (w *OutputWriter) writeValidationText(result ValidationResult) error {
	if result.Valid {
		_, _ = fmt.Fprintf(w.out, "%s Workflow '%s' is valid\n", w.colorizer.Success("✓"), result.Workflow)
	} else {
		_, _ = fmt.Fprintf(w.out, "%s Workflow '%s' is invalid\n", w.colorizer.Error("✗"), result.Workflow)
		for _, e := range result.Errors {
			_, _ = fmt.Fprintf(w.out, "  - %s\n", e)
		}
	}
	return nil
}

func (w *OutputWriter) writeExecutionTable(exec *ExecutionInfo) error {
	tw := newTableWriter(w.out, 12, 10, 10, 30)

	// Header section
	tw.fullWidthSeparator()
	tw.fullWidthRow(fmt.Sprintf("Workflow: %-20s ID: %s", exec.WorkflowName, truncateID(exec.WorkflowID)))
	tw.fullWidthRow(fmt.Sprintf("Status: %-22s Duration: %dms", exec.Status, exec.DurationMs))

	// Steps table
	if len(exec.Steps) > 0 {
		tw.separator()
		tw.row("STEP", "STATUS", "DURATION", "ERROR")
		tw.separator()
		for _, step := range exec.Steps {
			duration := "-"
			if step.StartedAt != "" && step.CompletedAt != "" {
				duration = calculateDuration(step.StartedAt, step.CompletedAt)
			}
			errMsg := "-"
			if step.Error != "" {
				errMsg = step.Error
			}
			tw.row(step.Name, step.Status, duration, errMsg)
		}
	}
	tw.separator()

	return nil
}

func (w *OutputWriter) writeRunResultTable(result *RunResult) error {
	tw := newTableWriter(w.out, 12, 10, 10, 30)

	// Header section
	tw.fullWidthSeparator()
	tw.fullWidthRow(fmt.Sprintf("Workflow ID: %s", result.WorkflowID))

	// Steps table
	if len(result.Steps) > 0 {
		tw.separator()
		tw.row("STEP", "STATUS", "DURATION", "OUTPUT")
		tw.separator()
		for _, step := range result.Steps {
			duration := "-"
			if step.StartedAt != "" && step.CompletedAt != "" {
				duration = calculateDuration(step.StartedAt, step.CompletedAt)
			}
			output := "-"
			if step.Output != "" {
				output = strings.TrimSpace(step.Output)
				if idx := strings.Index(output, "\n"); idx > 0 {
					output = output[:idx] + "..."
				}
			}
			tw.row(step.Name, step.Status, duration, output)
		}
	}

	// Footer
	tw.separator()
	stepCount := len(result.Steps)
	statusText := w.colorizer.Status(result.Status, result.Status)
	tw.fullWidthRow(fmt.Sprintf("Total: %d steps | Duration: %dms | Status: %s", stepCount, result.DurationMs, statusText))
	tw.fullWidthSeparator()

	if result.Error != "" {
		_, _ = fmt.Fprintf(w.out, "Error: %s\n", result.Error)
	}

	return nil
}

func (w *OutputWriter) writeValidationTable(result ValidationResult) error {
	tw := newTableWriter(w.out, 20, 40)

	// Header section
	tw.fullWidthSeparator()
	status := "valid"
	if !result.Valid {
		status = "invalid"
	}
	tw.fullWidthRow(fmt.Sprintf("Workflow: %-25s Status: %s", result.Workflow, status))
	tw.fullWidthSeparator()

	// Errors
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			tw.fullWidthRow(fmt.Sprintf("ERROR: %s", e))
		}
		tw.fullWidthSeparator()
	}

	return nil
}

// formatInputRow formats a single input row for the validation table.
// Returns: name, type, required, defaultVal as strings.
func formatInputRow(inp InputInfo) (name, typ, required, defaultVal string) {
	name = inp.Name
	typ = inp.Type
	required = "no"
	if inp.Required {
		required = "yes"
	}
	defaultVal = inp.Default
	if defaultVal == "" {
		defaultVal = "-"
	}
	return name, typ, required, defaultVal
}

// formatStepRow formats a single step row for the validation table.
// Returns: name, type, next as strings.
func formatStepRow(step StepSummary) (name, typ, next string) {
	name = step.Name
	typ = step.Type
	next = step.Next
	if next == "" {
		next = "(terminal)"
	}
	return name, typ, next
}

// renderStatusHeader renders the workflow status header line.
func renderStatusHeader(tw *tableWriter, workflowName string, valid bool) {
	status := "valid"
	if !valid {
		status = "invalid"
	}
	tw.fullWidthSeparator()

	// Calculate table inner width
	total := 1
	for _, width := range tw.columns {
		total += width + 3
	}
	innerWidth := total - 4

	// Format: "Workflow: <name>    Status: <status>"
	formatStr := fmt.Sprintf("Workflow: %s    Status: %s", workflowName, status)

	// If content is too long for a single row, split across two rows
	// and output without truncation to preserve all content
	if len(formatStr) > innerWidth {
		workflowLine := fmt.Sprintf("Workflow: %s", workflowName)
		statusLine := fmt.Sprintf("Status: %s", status)

		// Output workflow line (may still be too long, but try fullWidthRow first)
		if len(workflowLine) > innerWidth {
			// Bypass truncation by writing directly with proper padding
			_, _ = fmt.Fprintf(tw.w, "| %-*s |\n", innerWidth, workflowLine)
		} else {
			tw.fullWidthRow(workflowLine)
		}
		tw.fullWidthRow(statusLine)
	} else {
		tw.fullWidthRow(formatStr)
	}
	tw.fullWidthSeparator()
}

func (w *OutputWriter) writeValidationResultTable(result *ValidationResultTable) error {
	// Inputs table
	if len(result.Inputs) > 0 {
		tw := newTableWriter(w.out, 15, 10, 10, 20)
		renderStatusHeader(tw, result.Workflow, result.Valid)
		tw.row("INPUT", "TYPE", "REQUIRED", "DEFAULT")
		tw.separator()
		for _, inp := range result.Inputs {
			name, typ, required, defaultVal := formatInputRow(inp)
			tw.row(name, typ, required, defaultVal)
		}
		tw.separator()
	}

	// Steps table
	if len(result.Steps) > 0 {
		tw := newTableWriter(w.out, 15, 10, 20)
		if len(result.Inputs) == 0 {
			renderStatusHeader(tw, result.Workflow, result.Valid)
		}
		tw.row("STEP", "TYPE", "NEXT")
		tw.separator()
		for _, step := range result.Steps {
			name, typ, next := formatStepRow(step)
			tw.row(name, typ, next)
		}
		tw.separator()
	}

	// Errors
	if len(result.Errors) > 0 {
		_, _ = fmt.Fprintln(w.out)
		for _, e := range result.Errors {
			_, _ = fmt.Fprintf(w.out, "ERROR: %s\n", e)
		}
	}

	return nil
}

func truncateID(id string) string {
	if len(id) > 12 {
		return id[:12] + "..."
	}
	return id
}

func calculateDuration(startAt, completedAt string) string {
	start, err := time.Parse(time.RFC3339, startAt)
	if err != nil {
		return "-"
	}
	end, err := time.Parse(time.RFC3339, completedAt)
	if err != nil {
		return "-"
	}
	d := end.Sub(start)
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Round(time.Millisecond).String()
}
