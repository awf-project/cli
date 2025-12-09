package workflow

import "errors"

// StepType defines the type of workflow step.
type StepType string

const (
	StepTypeCommand  StepType = "command"
	StepTypeParallel StepType = "parallel"
	StepTypeTerminal StepType = "terminal"
)

func (s StepType) String() string {
	return string(s)
}

// Step represents a single step in a workflow state machine.
type Step struct {
	Name        string
	Type        StepType
	Description string
	Command     string   // for command type
	Branches    []string // for parallel type
	Timeout     int      // seconds
	OnSuccess   string   // next state name
	OnFailure   string   // next state name
	DependsOn   []string // for ordering in parallel execution
}

// Validate checks if the step configuration is valid.
func (s *Step) Validate() error {
	if s.Name == "" {
		return errors.New("step name is required")
	}

	switch s.Type {
	case StepTypeCommand:
		if s.Command == "" {
			return errors.New("command is required for command-type steps")
		}
	case StepTypeParallel:
		if len(s.Branches) == 0 {
			return errors.New("branches are required for parallel-type steps")
		}
	case StepTypeTerminal:
		// terminal steps don't need additional validation
	default:
		return errors.New("unknown step type")
	}

	return nil
}
