package workflow

import (
	"errors"
	"fmt"
)

type Transition struct {
	When string
	Goto string
}

// Evaluated in order; first match wins.
// A transition with empty When serves as the default fallback.
type Transitions []Transition

func (t Transitions) HasDefault() bool {
	for _, tr := range t {
		if tr.When == "" {
			return true
		}
	}
	return false
}

func (tr Transition) Validate() error {
	if tr.Goto == "" {
		return errors.New("transition goto is required")
	}
	return nil
}

func (tr Transition) String() string {
	if tr.When == "" {
		return fmt.Sprintf("goto %s", tr.Goto)
	}
	return fmt.Sprintf("when '%s' goto %s", tr.When, tr.Goto)
}

func (t Transitions) Validate() error {
	for i, tr := range t {
		if err := tr.Validate(); err != nil {
			return fmt.Errorf("transition %d: %w", i, err)
		}
	}
	return nil
}

func (t Transitions) GetTargetStates() []string {
	targets := make([]string, 0, len(t))
	for _, tr := range t {
		targets = append(targets, tr.Goto)
	}
	return targets
}

func (t Transitions) DefaultIndex() int {
	for i, tr := range t {
		if tr.When == "" {
			return i
		}
	}
	return -1
}

type EvaluatorFunc func(expr string) (bool, error)

func (t Transitions) EvaluateFirstMatch(eval EvaluatorFunc) (nextStep string, found bool, err error) {
	for _, tr := range t {
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

	return "", false, nil
}
