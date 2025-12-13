package ui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// DryRunFormatter formats dry-run execution plans for display.
type DryRunFormatter struct {
	out       io.Writer
	colorizer *Colorizer
}

// NewDryRunFormatter creates a new DryRunFormatter.
func NewDryRunFormatter(out io.Writer, useColor bool) *DryRunFormatter {
	return &DryRunFormatter{
		out:       out,
		colorizer: NewColorizer(useColor),
	}
}

// Format renders the dry-run plan to the output writer.
func (f *DryRunFormatter) Format(plan *workflow.DryRunPlan) error {
	if err := f.formatHeader(plan); err != nil {
		return err
	}

	// Format each step
	for i, step := range plan.Steps {
		if err := f.formatStep(&step, i+1); err != nil {
			return err
		}
	}

	return f.formatFooter()
}

// formatHeader renders the plan header with workflow name and inputs.
func (f *DryRunFormatter) formatHeader(plan *workflow.DryRunPlan) error {
	// Title
	title := fmt.Sprintf("Dry Run: %s", plan.WorkflowName)
	_, err := fmt.Fprintln(f.out, f.colorizer.Bold(title))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f.out, strings.Repeat("=", len(title)))
	if err != nil {
		return err
	}

	// Description
	if plan.Description != "" {
		_, err = fmt.Fprintf(f.out, "\n%s\n", plan.Description)
		if err != nil {
			return err
		}
	}

	// Inputs section
	if len(plan.Inputs) > 0 {
		_, err = fmt.Fprintln(f.out, "\nInputs:")
		if err != nil {
			return err
		}

		// Sort inputs for consistent output
		var inputNames []string
		for name := range plan.Inputs {
			inputNames = append(inputNames, name)
		}
		sort.Strings(inputNames)

		for _, name := range inputNames {
			input := plan.Inputs[name]
			valueStr := fmt.Sprintf("%v", input.Value)
			if input.Default {
				valueStr = fmt.Sprintf("%v (default)", input.Value)
			}
			_, err = fmt.Fprintf(f.out, "  %s: %s\n", name, valueStr)
			if err != nil {
				return err
			}
		}
	}

	// Execution plan header
	_, err = fmt.Fprintln(f.out, "\nExecution Plan:")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f.out, strings.Repeat("-", 15))
	return err
}

// formatStep renders a single step in the plan.
func (f *DryRunFormatter) formatStep(step *workflow.DryRunStep, index int) error {
	// Step header with type indicator
	label := stepTypeLabel(step.Type)
	var header string
	switch step.Type {
	case workflow.StepTypeTerminal:
		statusStr := "success"
		if step.Status == workflow.TerminalFailure {
			statusStr = "failure"
		}
		header = fmt.Sprintf("\n%s %s (%s)", label, step.Name, statusStr)
	case workflow.StepTypeParallel:
		header = fmt.Sprintf("\n%s %s", label, step.Name)
	case workflow.StepTypeForEach, workflow.StepTypeWhile:
		header = fmt.Sprintf("\n%s %s", label, step.Name)
	default:
		header = fmt.Sprintf("\n[%d] %s", index, step.Name)
	}
	_, err := fmt.Fprintln(f.out, f.colorizer.Bold(header))
	if err != nil {
		return err
	}

	// Description
	if step.Description != "" {
		_, err = fmt.Fprintf(f.out, "    %s\n", step.Description)
		if err != nil {
			return err
		}
	}

	// Type-specific formatting
	switch step.Type {
	case workflow.StepTypeParallel:
		if fmtErr := f.formatParallelStep(step); fmtErr != nil {
			return fmtErr
		}
	case workflow.StepTypeForEach, workflow.StepTypeWhile:
		if fmtErr := f.formatLoopStep(step); fmtErr != nil {
			return fmtErr
		}
	case workflow.StepTypeCommand:
		// Command
		if step.Command != "" {
			_, err = fmt.Fprintf(f.out, "    Command: %s\n", step.Command)
			if err != nil {
				return err
			}
		}
	case workflow.StepTypeTerminal:
		// No additional info for terminal
	}

	// Working directory
	if step.Dir != "" {
		_, err = fmt.Fprintf(f.out, "    Dir: %s\n", step.Dir)
		if err != nil {
			return err
		}
	}

	// Hooks
	if hookErr := f.formatHooks(step.Hooks); hookErr != nil {
		return hookErr
	}

	// Timeout
	if step.Timeout > 0 {
		_, err = fmt.Fprintf(f.out, "    Timeout: %ds\n", step.Timeout)
		if err != nil {
			return err
		}
	}

	// Retry
	if step.Retry != nil {
		_, err = fmt.Fprintf(f.out, "    Retry: %d attempts, %s backoff\n",
			step.Retry.MaxAttempts, step.Retry.Backoff)
		if err != nil {
			return err
		}
	}

	// Capture
	if step.Capture != nil {
		parts := []string{}
		if step.Capture.Stdout != "" {
			parts = append(parts, fmt.Sprintf("stdout -> %s", step.Capture.Stdout))
		}
		if step.Capture.Stderr != "" {
			parts = append(parts, fmt.Sprintf("stderr -> %s", step.Capture.Stderr))
		}
		if len(parts) > 0 {
			_, err = fmt.Fprintf(f.out, "    Capture: %s\n", strings.Join(parts, ", "))
			if err != nil {
				return err
			}
		}
	}

	// Continue on error
	if step.ContinueOnError {
		_, err = fmt.Fprintln(f.out, "    Continue on error: yes")
		if err != nil {
			return err
		}
	}

	// Transitions
	return f.formatTransitions(step.Transitions)
}

// formatHooks renders step hooks.
func (f *DryRunFormatter) formatHooks(hooks workflow.DryRunHooks) error {
	// Pre hooks
	for _, hook := range hooks.Pre {
		hookType := "pre"
		if hook.Type == "log" {
			_, err := fmt.Fprintf(f.out, "    Hook (%s): log %q\n", hookType, hook.Content)
			if err != nil {
				return err
			}
		} else {
			_, err := fmt.Fprintf(f.out, "    Hook (%s): %s\n", hookType, hook.Content)
			if err != nil {
				return err
			}
		}
	}

	// Post hooks
	for _, hook := range hooks.Post {
		hookType := "post"
		if hook.Type == "log" {
			_, err := fmt.Fprintf(f.out, "    Hook (%s): log %q\n", hookType, hook.Content)
			if err != nil {
				return err
			}
		} else {
			_, err := fmt.Fprintf(f.out, "    Hook (%s): %s\n", hookType, hook.Content)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// formatTransitions renders step transitions.
func (f *DryRunFormatter) formatTransitions(transitions []workflow.DryRunTransition) error {
	for _, tr := range transitions {
		arrow := f.colorizer.Success("->")
		if tr.Type == "failure" {
			arrow = f.colorizer.Error("->")
		}

		if tr.Condition != "" {
			_, err := fmt.Fprintf(f.out, "    %s when %q: %s\n", arrow, tr.Condition, tr.Target)
			if err != nil {
				return err
			}
		} else {
			label := tr.Type
			if label == "" {
				label = "default"
			}
			_, err := fmt.Fprintf(f.out, "    %s on_%s: %s\n", arrow, label, tr.Target)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// formatLoopStep renders loop-specific step information.
func (f *DryRunFormatter) formatLoopStep(step *workflow.DryRunStep) error {
	if step.Loop == nil {
		return nil
	}

	// Loop type
	_, err := fmt.Fprintf(f.out, "    Type: %s\n", step.Loop.Type)
	if err != nil {
		return err
	}

	// Items or condition
	if step.Loop.Items != "" {
		_, err = fmt.Fprintf(f.out, "    Items: %s\n", step.Loop.Items)
		if err != nil {
			return err
		}
	}
	if step.Loop.Condition != "" {
		_, err = fmt.Fprintf(f.out, "    Condition: %s\n", step.Loop.Condition)
		if err != nil {
			return err
		}
	}

	// Body
	if len(step.Loop.Body) > 0 {
		_, err = fmt.Fprintf(f.out, "    Body: %s\n", strings.Join(step.Loop.Body, ", "))
		if err != nil {
			return err
		}
	}

	// Max iterations
	if step.Loop.MaxIterations > 0 {
		_, err = fmt.Fprintf(f.out, "    Max iterations: %d\n", step.Loop.MaxIterations)
		if err != nil {
			return err
		}
	}

	// Break condition
	if step.Loop.BreakCondition != "" {
		_, err = fmt.Fprintf(f.out, "    Break when: %s\n", step.Loop.BreakCondition)
		if err != nil {
			return err
		}
	}

	// On complete
	if step.Loop.OnComplete != "" {
		_, err = fmt.Fprintf(f.out, "    On complete: %s\n", step.Loop.OnComplete)
		if err != nil {
			return err
		}
	}

	return nil
}

// formatParallelStep renders parallel-specific step information.
func (f *DryRunFormatter) formatParallelStep(step *workflow.DryRunStep) error {
	// Branches
	if len(step.Branches) > 0 {
		_, err := fmt.Fprintf(f.out, "    Branches: %s\n", strings.Join(step.Branches, ", "))
		if err != nil {
			return err
		}
	}

	// Strategy
	if step.Strategy != "" {
		_, err := fmt.Fprintf(f.out, "    Strategy: %s\n", step.Strategy)
		if err != nil {
			return err
		}
	}

	// Max concurrent
	if step.MaxConcurrent > 0 {
		_, err := fmt.Fprintf(f.out, "    Max concurrent: %d\n", step.MaxConcurrent)
		if err != nil {
			return err
		}
	}

	return nil
}

// formatFooter renders the plan footer.
func (f *DryRunFormatter) formatFooter() error {
	_, err := fmt.Fprintf(f.out, "\n%s No commands will be executed (dry-run mode).\n",
		f.colorizer.Success("OK"))
	return err
}

// stepTypeLabel returns a display label for the step type.
func stepTypeLabel(stepType workflow.StepType) string {
	switch stepType {
	case workflow.StepTypeCommand:
		return ""
	case workflow.StepTypeParallel:
		return "[PARALLEL]"
	case workflow.StepTypeTerminal:
		return "[T]"
	case workflow.StepTypeForEach:
		return "[LOOP:for_each]"
	case workflow.StepTypeWhile:
		return "[LOOP:while]"
	default:
		return "[?]"
	}
}
