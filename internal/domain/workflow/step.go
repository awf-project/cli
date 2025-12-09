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

// RetryConfig defines retry behavior for a step.
type RetryConfig struct {
	MaxAttempts        int     // max retry attempts (default: 1)
	InitialDelayMs     int     // initial delay in milliseconds
	MaxDelayMs         int     // max delay in milliseconds
	Backoff            string  // constant, linear, exponential
	Multiplier         float64 // for exponential backoff
	Jitter             float64 // ± randomization (0.0-1.0)
	RetryableExitCodes []int   // exit codes to retry on
}

// CaptureConfig defines output capture behavior.
type CaptureConfig struct {
	Stdout   string // variable name to store stdout
	Stderr   string // variable name to store stderr
	MaxSize  string // max bytes (e.g., "10MB")
	Encoding string // e.g., "utf-8"
}

// Step represents a single step in a workflow state machine.
type Step struct {
	Name            string
	Type            StepType
	Description     string
	Command         string         // for command type
	Branches        []string       // for parallel type
	Strategy        string         // for parallel: all_succeed, any_succeed, best_effort
	MaxConcurrent   int            // for parallel: max concurrent branches
	Timeout         int            // seconds
	OnSuccess       string         // next state name
	OnFailure       string         // next state name
	DependsOn       []string       // for ordering in parallel execution
	Retry           *RetryConfig   // retry configuration
	Capture         *CaptureConfig // output capture configuration
	Hooks           StepHooks      // pre/post hooks
	ContinueOnError bool           // don't fail workflow on error
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
