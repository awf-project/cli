package workflow

import (
	"errors"
	"time"
)

// LoopType defines the type of loop construct.
type LoopType string

const (
	LoopTypeForEach LoopType = "for_each"
	LoopTypeWhile   LoopType = "while"
)

func (t LoopType) String() string {
	return string(t)
}

// DefaultMaxIterations is the default iteration limit.
const DefaultMaxIterations = 100

// MaxAllowedIterations is the hard limit for safety.
const MaxAllowedIterations = 10000

// LoopConfig holds configuration for loop execution.
type LoopConfig struct {
	Type           LoopType // for_each or while
	Items          string   // template expression for items (for_each)
	Condition      string   // expression to evaluate (while)
	Body           []string // step names to execute each iteration
	MaxIterations  int      // safety limit (default: 100, max: 10000)
	BreakCondition string   // optional early exit expression
	OnComplete     string   // next state after loop completes
}

// Validate checks if the loop configuration is valid.
func (c *LoopConfig) Validate() error {
	// Validate loop type
	switch c.Type {
	case LoopTypeForEach:
		if c.Items == "" {
			return errors.New("items is required for for_each loops")
		}
	case LoopTypeWhile:
		if c.Condition == "" {
			return errors.New("condition is required for while loops")
		}
	default:
		return errors.New("invalid loop type: must be 'for_each' or 'while'")
	}

	// Validate body
	if len(c.Body) == 0 {
		return errors.New("body is required for loop steps")
	}

	// Validate max_iterations
	if c.MaxIterations < 0 {
		return errors.New("max_iterations must be non-negative")
	}
	if c.MaxIterations > MaxAllowedIterations {
		return errors.New("max_iterations exceeds maximum allowed limit")
	}

	return nil
}

// IterationResult holds the result of a single loop iteration.
type IterationResult struct {
	Index       int
	Item        any // for for_each
	StepResults map[string]*StepState
	Error       error
	StartedAt   time.Time
	CompletedAt time.Time
}

// Duration returns the execution time of the iteration.
func (r *IterationResult) Duration() time.Duration {
	return r.CompletedAt.Sub(r.StartedAt)
}

// Success returns true if the iteration completed without error.
func (r *IterationResult) Success() bool {
	return r.Error == nil
}

// LoopResult holds aggregated results of loop execution.
type LoopResult struct {
	Iterations  []IterationResult
	TotalCount  int
	BrokeAt     int // -1 if completed normally, index if break triggered
	StartedAt   time.Time
	CompletedAt time.Time
}

// NewLoopResult creates a new LoopResult with initialized values.
func NewLoopResult() *LoopResult {
	return &LoopResult{
		Iterations: make([]IterationResult, 0),
		BrokeAt:    -1,
		StartedAt:  time.Now(),
	}
}

// Duration returns the total execution time.
func (r *LoopResult) Duration() time.Duration {
	return r.CompletedAt.Sub(r.StartedAt)
}

// WasBroken returns true if the loop was terminated by a break condition.
func (r *LoopResult) WasBroken() bool {
	return r.BrokeAt >= 0
}

// AllSucceeded returns true if all iterations succeeded.
func (r *LoopResult) AllSucceeded() bool {
	for _, iter := range r.Iterations {
		if !iter.Success() {
			return false
		}
	}
	return len(r.Iterations) > 0
}
