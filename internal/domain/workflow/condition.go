package workflow

import (
	"errors"
	"fmt"
)

// Transition defines a conditional transition to another state.
// When the When expression evaluates to true (or When is empty for default),
// the workflow transitions to the Goto state.
type Transition struct {
	When string // condition expression (empty = default/fallback)
	Goto string // target state name
}

// Transitions is an ordered list of conditional transitions.
// Evaluated in order; first match wins.
// A transition with empty When serves as the default fallback.
type Transitions []Transition

// HasDefault returns true if the transitions include a default (unconditional) transition.
func (t Transitions) HasDefault() bool {
	for _, tr := range t {
		if tr.When == "" {
			return true
		}
	}
	return false
}

// Validate checks if the transition is valid.
func (tr Transition) Validate() error {
	if tr.Goto == "" {
		return errors.New("transition goto is required")
	}
	return nil
}

// String returns a human-readable representation of the transition.
func (tr Transition) String() string {
	if tr.When == "" {
		return fmt.Sprintf("goto %s", tr.Goto)
	}
	return fmt.Sprintf("when '%s' goto %s", tr.When, tr.Goto)
}

// Validate checks if all transitions are valid.
func (t Transitions) Validate() error {
	for i, tr := range t {
		if err := tr.Validate(); err != nil {
			return fmt.Errorf("transition %d: %w", i, err)
		}
	}
	return nil
}

// GetTargetStates returns all target states referenced by these transitions.
func (t Transitions) GetTargetStates() []string {
	targets := make([]string, 0, len(t))
	for _, tr := range t {
		targets = append(targets, tr.Goto)
	}
	return targets
}

// DefaultIndex returns the index of the first default (unconditional) transition,
// or -1 if there is no default.
func (t Transitions) DefaultIndex() int {
	for i, tr := range t {
		if tr.When == "" {
			return i
		}
	}
	return -1
}

// EvaluatorFunc is a function that evaluates a condition expression.
type EvaluatorFunc func(expr string) (bool, error)

// EvaluateFirstMatch evaluates transitions in order and returns the first matching goto target.
// Returns (goto, found, error). If no match is found and there's a default, returns the default.
// If no match and no default, returns ("", false, nil).
func (t Transitions) EvaluateFirstMatch(eval EvaluatorFunc) (string, bool, error) {
	for _, tr := range t {
		// Default transition (no condition) always matches
		if tr.When == "" {
			return tr.Goto, true, nil
		}

		result, err := eval(tr.When)
		if err != nil {
			return "", false, fmt.Errorf("evaluate condition '%s': %w", tr.When, err)
		}

		if result {
			return tr.Goto, true, nil
		}
	}

	// No match found
	return "", false, nil
}
