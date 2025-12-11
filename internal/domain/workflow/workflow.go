package workflow

import (
	"errors"
	"fmt"
)

// Input defines an input parameter for a workflow.
type Input struct {
	Name        string
	Type        string // string, integer, boolean
	Description string
	Required    bool
	Default     any
	Validation  *InputValidation // validation rules
}

// Workflow represents a complete workflow definition.
type Workflow struct {
	Name        string
	Description string
	Version     string
	Author      string
	Tags        []string
	Inputs      []Input
	Env         []string         // required environment variables
	Initial     string           // initial state name
	Steps       map[string]*Step // state name -> step
	Hooks       WorkflowHooks    // workflow-level hooks
}

// GetStep retrieves a step by name.
func (w *Workflow) GetStep(name string) (*Step, bool) {
	step, ok := w.Steps[name]
	return step, ok
}

// Validate checks if the workflow configuration is valid.
func (w *Workflow) Validate() error {
	if w.Name == "" {
		return errors.New("workflow name is required")
	}
	if w.Initial == "" {
		return errors.New("initial state is required")
	}

	// Check initial state exists
	if _, ok := w.Steps[w.Initial]; !ok {
		return fmt.Errorf("initial state '%s' not found in steps", w.Initial)
	}

	// Check at least one terminal state exists
	hasTerminal := false
	for _, step := range w.Steps {
		if step.Type == StepTypeTerminal {
			hasTerminal = true
			break
		}
	}
	if !hasTerminal {
		return errors.New("at least one terminal state is required")
	}

	// Validate each step
	for name, step := range w.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("step '%s': %w", name, err)
		}

		// Non-terminal steps must have some way to transition
		// Either: OnSuccess/OnFailure (legacy) OR Transitions (conditional)
		if step.Type == StepTypeCommand {
			hasLegacyTransitions := step.OnSuccess != "" || step.OnFailure != ""
			hasConditionalTransitions := len(step.Transitions) > 0
			if !hasLegacyTransitions && !hasConditionalTransitions {
				return fmt.Errorf("step '%s': command step must have OnSuccess/OnFailure or Transitions", name)
			}
		}

		// Validate Transitions targets exist
		for i, tr := range step.Transitions {
			if err := tr.Validate(); err != nil {
				return fmt.Errorf("step '%s': transition %d: %w", name, i, err)
			}
			if _, ok := w.Steps[tr.Goto]; !ok {
				return fmt.Errorf("step '%s': transition %d references unknown state '%s'", name, i, tr.Goto)
			}
		}

		// Validate legacy state references exist
		if step.OnSuccess != "" {
			if _, ok := w.Steps[step.OnSuccess]; !ok {
				return fmt.Errorf("step '%s': on_success references unknown state '%s'", name, step.OnSuccess)
			}
		}
		if step.OnFailure != "" {
			if _, ok := w.Steps[step.OnFailure]; !ok {
				return fmt.Errorf("step '%s': on_failure references unknown state '%s'", name, step.OnFailure)
			}
		}
	}

	return nil
}
