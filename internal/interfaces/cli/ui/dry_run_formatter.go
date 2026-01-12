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
	for i := range plan.Steps {
		if err := f.formatStep(&plan.Steps[i], i+1); err != nil {
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
		return fmt.Errorf("writing header: %w", err)
	}
	_, err = fmt.Fprintln(f.out, strings.Repeat("=", len(title)))
	if err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Description
	if plan.Description != "" {
		_, err = fmt.Fprintf(f.out, "\n%s\n", plan.Description)
		if err != nil {
			return fmt.Errorf("writing description: %w", err)
		}
	}

	// Inputs section
	if len(plan.Inputs) > 0 {
		_, err = fmt.Fprintln(f.out, "\nInputs:")
		if err != nil {
			return fmt.Errorf("writing inputs: %w", err)
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
				return fmt.Errorf("writing input %s: %w", name, err)
			}
		}
	}

	// Execution plan header
	_, err = fmt.Fprintln(f.out, "\nExecution Plan:")
	if err != nil {
		return fmt.Errorf("writing plan header: %w", err)
	}
	_, err = fmt.Fprintln(f.out, strings.Repeat("-", 15))
	if err != nil {
		return fmt.Errorf("writing plan header: %w", err)
	}
	return nil
}

// formatStep renders a single step in the plan.
func (f *DryRunFormatter) formatStep(step *workflow.DryRunStep, index int) error {
	// Step header
	if err := f.formatStepHeader(step, index); err != nil {
		return err
	}

	// Description
	if step.Description != "" {
		_, err := fmt.Fprintf(f.out, "    %s\n", step.Description)
		if err != nil {
			return fmt.Errorf("writing step description: %w", err)
		}
	}

	// Type-specific formatting
	if err := f.formatStepTypeDetails(step); err != nil {
		return err
	}

	// Working directory - use helper for field formatting
	if fmtErr := f.FormatFieldIfPresent("Dir", step.Dir); fmtErr != nil {
		return fmtErr
	}

	// Hooks
	if hookErr := f.formatHooks(step.Hooks); hookErr != nil {
		return hookErr
	}

	// Timeout - use helper for field formatting
	if step.Timeout > 0 {
		timeoutStr := fmt.Sprintf("%ds", step.Timeout)
		if fmtErr := f.FormatFieldIfPresent("Timeout", timeoutStr); fmtErr != nil {
			return fmtErr
		}
	}

	// Retry - use helper for retry formatting
	if fmtErr := f.FormatRetry(step.Retry); fmtErr != nil {
		return fmtErr
	}

	// Capture - use helper for capture formatting
	if fmtErr := f.FormatCapture(step.Capture); fmtErr != nil {
		return fmtErr
	}

	// Continue on error - use helper for field formatting
	if step.ContinueOnError {
		if fmtErr := f.FormatFieldIfPresent("Continue on error", "yes"); fmtErr != nil {
			return fmtErr
		}
	}

	// Transitions
	return f.formatTransitions(step.Transitions)
}

// formatStepHeader renders the step header with type indicator.
func (f *DryRunFormatter) formatStepHeader(step *workflow.DryRunStep, index int) error {
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
		return fmt.Errorf("writing step header: %w", err)
	}
	return nil
}

// formatStepTypeDetails formats type-specific step details.
func (f *DryRunFormatter) formatStepTypeDetails(step *workflow.DryRunStep) error {
	switch step.Type {
	case workflow.StepTypeParallel:
		if fmtErr := f.formatParallelStep(step); fmtErr != nil {
			return fmtErr
		}
	case workflow.StepTypeForEach, workflow.StepTypeWhile:
		if fmtErr := f.formatLoopStep(step); fmtErr != nil {
			return fmtErr
		}
	case workflow.StepTypeAgent:
		if fmtErr := f.formatAgentStep(step); fmtErr != nil {
			return fmtErr
		}
	case workflow.StepTypeOperation:
		// Plugin operations - formatted in detail section
	case workflow.StepTypeCallWorkflow:
		// Subworkflow calls - formatted in detail section
	case workflow.StepTypeCommand:
		// Command - use helper for field formatting
		if fmtErr := f.FormatFieldIfPresent("Command", step.Command); fmtErr != nil {
			return fmtErr
		}
	case workflow.StepTypeTerminal:
		// No additional info for terminal
	}
	return nil
}

// formatHooks renders step hooks.
func (f *DryRunFormatter) formatHooks(hooks workflow.DryRunHooks) error {
	// Pre hooks
	for _, hook := range hooks.Pre {
		hookType := "pre"
		if hook.Type == "log" {
			_, err := fmt.Fprintf(f.out, "    Hook (%s): log %q\n", hookType, hook.Content)
			if err != nil {
				return fmt.Errorf("writing pre hook: %w", err)
			}
		} else {
			_, err := fmt.Fprintf(f.out, "    Hook (%s): %s\n", hookType, hook.Content)
			if err != nil {
				return fmt.Errorf("writing pre hook: %w", err)
			}
		}
	}

	// Post hooks
	for _, hook := range hooks.Post {
		hookType := "post"
		if hook.Type == "log" {
			_, err := fmt.Fprintf(f.out, "    Hook (%s): log %q\n", hookType, hook.Content)
			if err != nil {
				return fmt.Errorf("writing post hook: %w", err)
			}
		} else {
			_, err := fmt.Fprintf(f.out, "    Hook (%s): %s\n", hookType, hook.Content)
			if err != nil {
				return fmt.Errorf("writing post hook: %w", err)
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
				return fmt.Errorf("writing transition: %w", err)
			}
		} else {
			label := tr.Type
			if label == "" {
				label = "default"
			}
			_, err := fmt.Fprintf(f.out, "    %s on_%s: %s\n", arrow, label, tr.Target)
			if err != nil {
				return fmt.Errorf("writing transition: %w", err)
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
		return fmt.Errorf("writing loop type: %w", err)
	}

	// Items or condition
	if step.Loop.Items != "" {
		_, err = fmt.Fprintf(f.out, "    Items: %s\n", step.Loop.Items)
		if err != nil {
			return fmt.Errorf("writing loop items: %w", err)
		}
	}
	if step.Loop.Condition != "" {
		_, err = fmt.Fprintf(f.out, "    Condition: %s\n", step.Loop.Condition)
		if err != nil {
			return fmt.Errorf("writing loop condition: %w", err)
		}
	}

	// Body
	if len(step.Loop.Body) > 0 {
		_, err = fmt.Fprintf(f.out, "    Body: %s\n", strings.Join(step.Loop.Body, ", "))
		if err != nil {
			return fmt.Errorf("writing loop body: %w", err)
		}
	}

	// Max iterations
	if step.Loop.MaxIterations > 0 {
		_, err = fmt.Fprintf(f.out, "    Max iterations: %d\n", step.Loop.MaxIterations)
		if err != nil {
			return fmt.Errorf("writing loop max iterations: %w", err)
		}
	}

	// Break condition
	if step.Loop.BreakCondition != "" {
		_, err = fmt.Fprintf(f.out, "    Break when: %s\n", step.Loop.BreakCondition)
		if err != nil {
			return fmt.Errorf("writing loop break condition: %w", err)
		}
	}

	// On complete
	if step.Loop.OnComplete != "" {
		_, err = fmt.Fprintf(f.out, "    On complete: %s\n", step.Loop.OnComplete)
		if err != nil {
			return fmt.Errorf("writing loop on complete: %w", err)
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
			return fmt.Errorf("writing parallel branches: %w", err)
		}
	}

	// Strategy
	if step.Strategy != "" {
		_, err := fmt.Fprintf(f.out, "    Strategy: %s\n", step.Strategy)
		if err != nil {
			return fmt.Errorf("writing parallel strategy: %w", err)
		}
	}

	// Max concurrent
	if step.MaxConcurrent > 0 {
		_, err := fmt.Fprintf(f.out, "    Max concurrent: %d\n", step.MaxConcurrent)
		if err != nil {
			return fmt.Errorf("writing parallel max concurrent: %w", err)
		}
	}

	return nil
}

// formatAgentStep renders agent-specific step information.
func (f *DryRunFormatter) formatAgentStep(step *workflow.DryRunStep) error {
	if step.Agent == nil {
		return nil
	}

	// String fields using extracted helper
	if err := f.FormatFieldIfPresent("Provider", step.Agent.Provider); err != nil {
		return err
	}
	if err := f.FormatFieldIfPresent("Prompt", step.Agent.ResolvedPrompt); err != nil {
		return err
	}
	if err := f.FormatFieldIfPresent("CLI", step.Agent.CLICommand); err != nil {
		return err
	}

	// Options map using extracted helper
	if err := f.formatAgentOptions(step.Agent.Options); err != nil {
		return err
	}

	// Timeout using extracted helper
	return f.FormatIntFieldIfPositive("Agent timeout", step.Agent.Timeout, "s")
}

// formatFooter renders the plan footer.
func (f *DryRunFormatter) formatFooter() error {
	_, err := fmt.Fprintf(f.out, "\n%s No commands will be executed (dry-run mode).\n",
		f.colorizer.Success("OK"))
	if err != nil {
		return fmt.Errorf("writing footer: %w", err)
	}
	return nil
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
	case workflow.StepTypeAgent:
		return "[AGENT]"
	default:
		return "[?]"
	}
}

// =============================================================================
// Field Formatters - Helper extraction to reduce formatStep complexity
// =============================================================================

// NewDryRunFormatterWithWriter creates a DryRunFormatter with custom writer (for testing).
func NewDryRunFormatterWithWriter(out io.Writer, useColor bool) *DryRunFormatter {
	return &DryRunFormatter{
		out:       out,
		colorizer: NewColorizer(useColor),
	}
}

// FormatFieldIfPresent formats a configuration field if value is non-empty.
func (f *DryRunFormatter) FormatFieldIfPresent(label, value string) error {
	// Skip empty values (guard pattern from C001)
	if value == "" {
		return nil
	}

	// Write formatted field with 4-space indentation
	_, err := fmt.Fprintf(f.out, "    %s: %s\n", label, value)
	if err != nil {
		return fmt.Errorf("writing field %s: %w", label, err)
	}
	return nil
}

// FormatIntFieldIfPositive formats an integer field if value is positive.
func (f *DryRunFormatter) FormatIntFieldIfPositive(label string, value int, unit string) error {
	if value <= 0 {
		return nil
	}
	_, err := fmt.Fprintf(f.out, "    %s: %d%s\n", label, value, unit)
	if err != nil {
		return fmt.Errorf("writing field %s: %w", label, err)
	}
	return nil
}

// FormatRetry formats retry configuration.
func (f *DryRunFormatter) FormatRetry(retry *workflow.DryRunRetry) error {
	// Skip nil retry (guard pattern)
	if retry == nil {
		return nil
	}

	// Write retry configuration in format: "X attempts, Y backoff"
	_, err := fmt.Fprintf(f.out, "    Retry: %d attempts, %s backoff\n",
		retry.MaxAttempts, retry.Backoff)
	if err != nil {
		return fmt.Errorf("writing retry: %w", err)
	}
	return nil
}

// FormatCapture formats capture configuration.
func (f *DryRunFormatter) FormatCapture(capture *workflow.DryRunCapture) error {
	// Skip nil capture (guard pattern)
	if capture == nil {
		return nil
	}

	// Build parts slice with stdout-before-stderr ordering
	parts := []string{}
	if capture.Stdout != "" {
		parts = append(parts, fmt.Sprintf("stdout -> %s", capture.Stdout))
	}
	if capture.Stderr != "" {
		parts = append(parts, fmt.Sprintf("stderr -> %s", capture.Stderr))
	}

	// Skip if no streams configured
	if len(parts) == 0 {
		return nil
	}

	// Write capture configuration
	_, err := fmt.Fprintf(f.out, "    Capture: %s\n", strings.Join(parts, ", "))
	if err != nil {
		return fmt.Errorf("writing capture: %w", err)
	}
	return nil
}

// formatAgentOptions formats agent options map with sorted keys.
func (f *DryRunFormatter) formatAgentOptions(options map[string]any) error {
	if len(options) == 0 {
		return nil
	}

	if _, err := fmt.Fprintln(f.out, "    Options:"); err != nil {
		return fmt.Errorf("writing agent options header: %w", err)
	}

	keys := make([]string, 0, len(options))
	for k := range options {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if _, err := fmt.Fprintf(f.out, "      %s: %v\n", key, options[key]); err != nil {
			return fmt.Errorf("writing agent option %s: %w", key, err)
		}
	}
	return nil
}
