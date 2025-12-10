package workflow

import "time"

// ParallelStrategy defines how parallel execution results are evaluated.
type ParallelStrategy string

const (
	// StrategyAllSucceed requires all branches to succeed; cancels remaining on first failure.
	StrategyAllSucceed ParallelStrategy = "all_succeed"
	// StrategyAnySucceed succeeds if at least one branch succeeds.
	StrategyAnySucceed ParallelStrategy = "any_succeed"
	// StrategyBestEffort runs all branches and collects all results regardless of failures.
	StrategyBestEffort ParallelStrategy = "best_effort"
)

// DefaultParallelStrategy is used when no strategy is specified.
const DefaultParallelStrategy = StrategyAllSucceed

// DefaultMaxConcurrent is the default concurrency limit (0 = unlimited).
const DefaultMaxConcurrent = 0

// ParseParallelStrategy converts a string to ParallelStrategy.
// Returns DefaultParallelStrategy for empty or invalid values.
func ParseParallelStrategy(s string) ParallelStrategy {
	switch s {
	case "all_succeed":
		return StrategyAllSucceed
	case "any_succeed":
		return StrategyAnySucceed
	case "best_effort":
		return StrategyBestEffort
	default:
		return DefaultParallelStrategy
	}
}

func (s ParallelStrategy) String() string {
	return string(s)
}

// ParallelConfig holds configuration for parallel execution.
type ParallelConfig struct {
	Strategy      ParallelStrategy
	MaxConcurrent int
}

// BranchResult holds the result of a single parallel branch execution.
type BranchResult struct {
	Name        string
	Output      string
	Stderr      string
	ExitCode    int
	Error       error
	StartedAt   time.Time
	CompletedAt time.Time
}

// Duration returns the execution time of the branch.
func (r *BranchResult) Duration() time.Duration {
	return r.CompletedAt.Sub(r.StartedAt)
}

// Success returns true if the branch completed without error and exit code 0.
func (r *BranchResult) Success() bool {
	return r.Error == nil && r.ExitCode == 0
}

// ParallelResult holds the aggregated results of parallel execution.
type ParallelResult struct {
	Results      map[string]*BranchResult
	FirstError   error
	SuccessCount int
	FailureCount int
	StartedAt    time.Time
	CompletedAt  time.Time
}

// NewParallelResult creates a new ParallelResult.
func NewParallelResult() *ParallelResult {
	return &ParallelResult{
		Results:   make(map[string]*BranchResult),
		StartedAt: time.Now(),
	}
}

// AddResult records a branch result and updates counters.
func (r *ParallelResult) AddResult(result *BranchResult) {
	r.Results[result.Name] = result
	if result.Success() {
		r.SuccessCount++
	} else {
		r.FailureCount++
		if r.FirstError == nil && result.Error != nil {
			r.FirstError = result.Error
		}
	}
}

// Duration returns the total execution time.
func (r *ParallelResult) Duration() time.Duration {
	return r.CompletedAt.Sub(r.StartedAt)
}

// AllSucceeded returns true if all branches succeeded.
func (r *ParallelResult) AllSucceeded() bool {
	return r.FailureCount == 0 && r.SuccessCount > 0
}

// AnySucceeded returns true if at least one branch succeeded.
func (r *ParallelResult) AnySucceeded() bool {
	return r.SuccessCount > 0
}
