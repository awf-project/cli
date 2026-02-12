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

// StateReferenceError represents a reference to an undefined state.
// This domain error carries structured information about invalid state references
// for conversion to StructuredError in the application layer.
type StateReferenceError struct {
	StepName        string
	ReferencedState string
	AvailableStates []string
	Field           string // "initial", "on_success", "on_failure", "transition", "loop_body", "on_complete"
}

func (e *StateReferenceError) Error() string {
	switch e.Field {
	case "initial":
		return fmt.Sprintf("initial state '%s' not found in steps", e.ReferencedState)
	case "on_success":
		return fmt.Sprintf("step '%s': on_success references unknown state '%s'", e.StepName, e.ReferencedState)
	case "on_failure":
		return fmt.Sprintf("step '%s': on_failure references unknown state '%s'", e.StepName, e.ReferencedState)
	case "transition":
		return fmt.Sprintf("step '%s': transition references unknown state '%s'", e.StepName, e.ReferencedState)
	case "loop_body":
		return fmt.Sprintf("step '%s': loop body references unknown step '%s'", e.StepName, e.ReferencedState)
	case "on_complete":
		return fmt.Sprintf("step '%s': on_complete references unknown state '%s'", e.StepName, e.ReferencedState)
	default:
		return fmt.Sprintf("step '%s': references unknown state '%s'", e.StepName, e.ReferencedState)
	}
}

// Validate checks if the workflow configuration is valid.
// The validator parameter is used to check expression syntax in agent configurations.
//
//nolint:gocognit // Complexity 62: workflow validation is comprehensive, checking inputs, steps, graph, templates, parallel strategies. Central validation requires thorough checking.
func (w *Workflow) Validate(validator ExpressionCompiler) error {
	if w.Name == "" {
		return errors.New("workflow name is required")
	}
	if w.Initial == "" {
		return errors.New("initial state is required")
	}

	// Check initial state exists
	if _, ok := w.Steps[w.Initial]; !ok {
		availableStates := make([]string, 0, len(w.Steps))
		for stateName := range w.Steps {
			availableStates = append(availableStates, stateName)
		}
		return &StateReferenceError{
			StepName:        "",
			ReferencedState: w.Initial,
			AvailableStates: availableStates,
			Field:           "initial",
		}
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

	// Build set of steps that are part of loop bodies
	// F048: Loop body steps can have transitions to targets outside the workflow
	// (will be validated at runtime by loop executor per ADR-005)
	loopBodySteps := make(map[string]bool)
	for _, step := range w.Steps {
		if step.Loop != nil {
			for _, bodyStepName := range step.Loop.Body {
				loopBodySteps[bodyStepName] = true
			}
		}
	}

	// Build set of steps that are parallel branch children
	// B004: Parallel branch children have transitions discarded by execution engine
	parallelBranchSteps := make(map[string]bool)
	for _, step := range w.Steps {
		if step.Type == StepTypeParallel {
			for _, branchName := range step.Branches {
				parallelBranchSteps[branchName] = true
			}
		}
	}

	// Validate each step
	for name, step := range w.Steps {
		if err := step.Validate(validator); err != nil {
			return fmt.Errorf("step '%s': %w", name, err)
		}

		// Non-terminal steps must have some way to transition
		// Either: OnSuccess/OnFailure (legacy) OR Transitions (conditional)
		// B004: Parallel branch children are exempt (transitions discarded by executor)
		if step.Type == StepTypeCommand && !parallelBranchSteps[name] {
			hasLegacyTransitions := step.OnSuccess != "" || step.OnFailure != ""
			hasConditionalTransitions := len(step.Transitions) > 0
			if !hasLegacyTransitions && !hasConditionalTransitions {
				return fmt.Errorf("step '%s': command step must have OnSuccess/OnFailure or Transitions", name)
			}
		}

		// Validate Transitions targets exist
		// F048: Skip validation for loop body steps (runtime validation per ADR-005)
		isLoopBodyStep := loopBodySteps[name]
		for i, tr := range step.Transitions {
			if err := tr.Validate(); err != nil {
				return fmt.Errorf("step '%s': transition %d: %w", name, i, err)
			}
			// F048: Loop body steps can transition to targets not in workflow
			// (will be handled gracefully at runtime per ADR-005)
			if !isLoopBodyStep {
				if _, ok := w.Steps[tr.Goto]; !ok {
					availableStates := make([]string, 0, len(w.Steps))
					for stateName := range w.Steps {
						availableStates = append(availableStates, stateName)
					}
					return &StateReferenceError{
						StepName:        name,
						ReferencedState: tr.Goto,
						AvailableStates: availableStates,
						Field:           "transition",
					}
				}
			}
		}

		// Validate legacy state references exist
		if step.OnSuccess != "" {
			if _, ok := w.Steps[step.OnSuccess]; !ok {
				availableStates := make([]string, 0, len(w.Steps))
				for stateName := range w.Steps {
					availableStates = append(availableStates, stateName)
				}
				return &StateReferenceError{
					StepName:        name,
					ReferencedState: step.OnSuccess,
					AvailableStates: availableStates,
					Field:           "on_success",
				}
			}
		}
		if step.OnFailure != "" {
			if _, ok := w.Steps[step.OnFailure]; !ok {
				availableStates := make([]string, 0, len(w.Steps))
				for stateName := range w.Steps {
					availableStates = append(availableStates, stateName)
				}
				return &StateReferenceError{
					StepName:        name,
					ReferencedState: step.OnFailure,
					AvailableStates: availableStates,
					Field:           "on_failure",
				}
			}
		}

		// Validate loop body step references exist
		if step.Loop != nil && len(step.Loop.Body) > 0 {
			for _, bodyStepName := range step.Loop.Body {
				if _, ok := w.Steps[bodyStepName]; !ok {
					availableStates := make([]string, 0, len(w.Steps))
					for stateName := range w.Steps {
						availableStates = append(availableStates, stateName)
					}
					return &StateReferenceError{
						StepName:        name,
						ReferencedState: bodyStepName,
						AvailableStates: availableStates,
						Field:           "loop_body",
					}
				}
			}
			if step.Loop.OnComplete != "" {
				if _, ok := w.Steps[step.Loop.OnComplete]; !ok {
					availableStates := make([]string, 0, len(w.Steps))
					for stateName := range w.Steps {
						availableStates = append(availableStates, stateName)
					}
					return &StateReferenceError{
						StepName:        name,
						ReferencedState: step.Loop.OnComplete,
						AvailableStates: availableStates,
						Field:           "on_complete",
					}
				}
			}
		}
	}

	return nil
}
