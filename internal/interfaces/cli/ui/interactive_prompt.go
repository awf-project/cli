package ui

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// Ensure CLIPrompt implements InteractivePrompt.
var _ ports.InteractivePrompt = (*CLIPrompt)(nil)

// CLIPrompt implements InteractivePrompt for terminal-based interaction.
type CLIPrompt struct {
	reader    *bufio.Reader
	writer    io.Writer
	colorizer *Colorizer
}

// NewCLIPrompt creates a new CLI prompt with the given I/O streams.
func NewCLIPrompt(reader io.Reader, writer io.Writer, colorEnabled bool) *CLIPrompt {
	return &CLIPrompt{
		reader:    bufio.NewReader(reader),
		writer:    writer,
		colorizer: NewColorizer(colorEnabled),
	}
}

// ShowHeader displays the interactive mode header with workflow name.
func (p *CLIPrompt) ShowHeader(workflowName string) {
	title := fmt.Sprintf("Interactive Mode: %s", workflowName)
	separator := strings.Repeat("=", len(title))
	_, _ = fmt.Fprintf(p.writer, "\n%s\n%s\n\n", p.colorizer.Info(title), separator)
}

// ShowStepDetails displays step information before execution.
func (p *CLIPrompt) ShowStepDetails(info *workflow.InteractiveStepInfo) {
	// Header: [Step N/M] stepname
	header := fmt.Sprintf("[Step %d/%d] %s", info.Index, info.Total, info.Name)
	_, _ = fmt.Fprintf(p.writer, "%s\n", p.colorizer.Bold(header))

	// Type
	if info.Step != nil {
		_, _ = fmt.Fprintf(p.writer, "Type: %s\n", info.Step.Type)
	}

	// Command (resolved)
	if info.Command != "" {
		_, _ = fmt.Fprintf(p.writer, "Command: %s\n", info.Command)
	}

	// Timeout
	if info.Step != nil && info.Step.Timeout > 0 {
		_, _ = fmt.Fprintf(p.writer, "Timeout: %ds\n", info.Step.Timeout)
	}

	// Transitions
	for _, tr := range info.Transitions {
		_, _ = fmt.Fprintf(p.writer, "%s\n", tr)
	}

	_, _ = fmt.Fprintln(p.writer)
}

// PromptAction prompts the user for an action and returns their choice.
func (p *CLIPrompt) PromptAction(hasRetry bool) (workflow.InteractiveAction, error) {
	for {
		// Build prompt based on available options
		prompt := "[c]ontinue [s]kip [a]bort [i]nspect [e]dit"
		if hasRetry {
			prompt += " [r]etry"
		}
		prompt += " > "
		_, _ = fmt.Fprint(p.writer, prompt)

		// Read input
		line, err := p.reader.ReadString('\n')
		if err != nil {
			return workflow.ActionAbort, fmt.Errorf("read input: %w", err)
		}

		// Parse action
		action, err := parseAction(line, hasRetry)
		if err != nil {
			_, _ = fmt.Fprintf(p.writer, "Error: %s\n", err.Error())
			continue
		}

		return action, nil
	}
}

// ShowExecuting displays a message indicating step execution is in progress.
func (p *CLIPrompt) ShowExecuting(stepName string) {
	_, _ = fmt.Fprintf(p.writer, "\nExecuting %s...\n", p.colorizer.Bold(stepName))
	_, _ = fmt.Fprintln(p.writer, strings.Repeat("-", 20))
}

// ShowStepResult displays the outcome of step execution.
func (p *CLIPrompt) ShowStepResult(state *workflow.StepState, nextStep string) {
	// Show output if any
	if state.Output != "" {
		_, _ = fmt.Fprintf(p.writer, "Output: %s\n", strings.TrimSpace(state.Output))
	}

	// Show stderr if any
	if state.Stderr != "" {
		_, _ = fmt.Fprintf(p.writer, "Stderr: %s\n", p.colorizer.Error(strings.TrimSpace(state.Stderr)))
	}

	// Status line
	duration := state.CompletedAt.Sub(state.StartedAt)
	statusSymbol := p.colorizer.Success("✓")
	if state.Status != workflow.StatusCompleted {
		statusSymbol = p.colorizer.Error("✗")
	}
	_, _ = fmt.Fprintf(p.writer, "Exit: %d | Duration: %s | Status: %s %s\n",
		state.ExitCode, duration.Round(10*time.Millisecond), statusSymbol, state.Status)

	// Next step
	if nextStep != "" {
		_, _ = fmt.Fprintf(p.writer, "\n→ Next: %s\n\n", nextStep)
	} else {
		_, _ = fmt.Fprintln(p.writer)
	}
}

// ShowContext displays the current runtime context.
func (p *CLIPrompt) ShowContext(ctx *workflow.RuntimeContext) {
	_, _ = fmt.Fprintf(p.writer, "\n┌─ Context %s┐\n", strings.Repeat("─", 50))

	// Show inputs
	if len(ctx.Inputs) > 0 {
		_, _ = fmt.Fprintln(p.writer, "│ Inputs:")
		// Sort keys for deterministic output
		keys := make([]string, 0, len(ctx.Inputs))
		for k := range ctx.Inputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			_, _ = fmt.Fprintf(p.writer, "│   %s: %v\n", k, ctx.Inputs[k])
		}
	}

	// Show states
	if len(ctx.States) > 0 {
		_, _ = fmt.Fprintln(p.writer, "│ States:")
		// Sort keys for deterministic output
		keys := make([]string, 0, len(ctx.States))
		for k := range ctx.States {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			state := ctx.States[k]
			_, _ = fmt.Fprintf(p.writer, "│   %s.Output: %s\n", k, truncateString(state.Output, 50))
			_, _ = fmt.Fprintf(p.writer, "│   %s.ExitCode: %d\n", k, state.ExitCode)
		}
	}

	_, _ = fmt.Fprintf(p.writer, "└%s┘\n\n", strings.Repeat("─", 60))
}

// EditInput prompts the user to edit an input value.
func (p *CLIPrompt) EditInput(name string, current any) (any, error) {
	_, _ = fmt.Fprintf(p.writer, "Edit input '%s' (current: %v)\n", name, current)
	_, _ = fmt.Fprint(p.writer, "New value: ")

	line, err := p.reader.ReadString('\n')
	if err != nil {
		return current, fmt.Errorf("read input: %w", err)
	}

	newValue := strings.TrimSpace(line)
	if newValue == "" {
		return current, nil // Keep current value if empty input
	}

	return newValue, nil
}

// ShowAborted displays a message indicating workflow was aborted.
func (p *CLIPrompt) ShowAborted() {
	_, _ = fmt.Fprintf(p.writer, "\n%s Workflow aborted by user.\n", p.colorizer.Warning("⚠"))
}

// ShowSkipped displays a message indicating step was skipped.
func (p *CLIPrompt) ShowSkipped(stepName, nextStep string) {
	_, _ = fmt.Fprintf(p.writer, "\n%s Skipped step '%s'\n", p.colorizer.Warning("↷"), stepName)
	if nextStep != "" {
		_, _ = fmt.Fprintf(p.writer, "→ Next: %s\n\n", nextStep)
	}
}

// ShowCompleted displays a message indicating workflow completed.
func (p *CLIPrompt) ShowCompleted(status workflow.ExecutionStatus) {
	if status == workflow.StatusCompleted {
		_, _ = fmt.Fprintf(p.writer, "\n%s Workflow completed successfully.\n", p.colorizer.Success("✓"))
	} else {
		_, _ = fmt.Fprintf(p.writer, "\n%s Workflow completed with status: %s\n", p.colorizer.Warning("!"), status)
	}
}

// ShowError displays an error message.
func (p *CLIPrompt) ShowError(err error) {
	_, _ = fmt.Fprintf(p.writer, "%s Error: %s\n", p.colorizer.Error("✗"), err.Error())
}

// parseAction converts a single-character input to an InteractiveAction.
func parseAction(input string, hasRetry bool) (workflow.InteractiveAction, error) {
	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "c":
		return workflow.ActionContinue, nil
	case "s":
		return workflow.ActionSkip, nil
	case "a":
		return workflow.ActionAbort, nil
	case "i":
		return workflow.ActionInspect, nil
	case "e":
		return workflow.ActionEdit, nil
	case "r":
		if hasRetry {
			return workflow.ActionRetry, nil
		}
		return "", fmt.Errorf("retry not available for this step")
	default:
		return "", fmt.Errorf("invalid action: %s", input)
	}
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	// Replace newlines with spaces for display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
