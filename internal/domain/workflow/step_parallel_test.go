package workflow

// C013: Domain test file splitting
// Source: internal/domain/workflow/parallel_test.go (Feature F033)
// Test count: 19 tests
// Tests: Parallel execution types - ParallelStrategy parsing and conversion,
//        ParallelConfig defaults, BranchResult handling, ParallelResult aggregation

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseParallelStrategy(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ParallelStrategy
	}{
		{"all_succeed", "all_succeed", StrategyAllSucceed},
		{"any_succeed", "any_succeed", StrategyAnySucceed},
		{"best_effort", "best_effort", StrategyBestEffort},
		{"empty defaults to all_succeed", "", DefaultParallelStrategy},
		{"invalid defaults to all_succeed", "invalid", DefaultParallelStrategy},
		{"uppercase not recognized", "ALL_SUCCEED", DefaultParallelStrategy},
		{"mixed case not recognized", "All_Succeed", DefaultParallelStrategy},
		{"with spaces not recognized", " all_succeed ", DefaultParallelStrategy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseParallelStrategy(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParallelStrategy_String(t *testing.T) {
	tests := []struct {
		strategy ParallelStrategy
		expected string
	}{
		{StrategyAllSucceed, "all_succeed"},
		{StrategyAnySucceed, "any_succeed"},
		{StrategyBestEffort, "best_effort"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.strategy.String())
		})
	}
}

func TestDefaultParallelStrategy(t *testing.T) {
	assert.Equal(t, StrategyAllSucceed, DefaultParallelStrategy)
}

func TestDefaultMaxConcurrent(t *testing.T) {
	assert.Equal(t, 0, DefaultMaxConcurrent, "0 means unlimited")
}

func TestParallelConfig_Defaults(t *testing.T) {
	config := ParallelConfig{}

	assert.Equal(t, ParallelStrategy(""), config.Strategy)
	assert.Equal(t, 0, config.MaxConcurrent)
}

func TestBranchResult_Success(t *testing.T) {
	tests := []struct {
		name     string
		result   BranchResult
		expected bool
	}{
		{
			name:     "success with exit code 0 and no error",
			result:   BranchResult{ExitCode: 0, Error: nil},
			expected: true,
		},
		{
			name:     "failure with non-zero exit code",
			result:   BranchResult{ExitCode: 1, Error: nil},
			expected: false,
		},
		{
			name:     "failure with exit code 127 (command not found)",
			result:   BranchResult{ExitCode: 127, Error: nil},
			expected: false,
		},
		{
			name:     "failure with error but exit code 0",
			result:   BranchResult{ExitCode: 0, Error: errors.New("context canceled")},
			expected: false,
		},
		{
			name:     "failure with both error and non-zero exit",
			result:   BranchResult{ExitCode: 1, Error: errors.New("command failed")},
			expected: false,
		},
		{
			name:     "negative exit code is failure",
			result:   BranchResult{ExitCode: -1, Error: nil},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.Success())
		})
	}
}

func TestBranchResult_Duration(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(2*time.Second + 500*time.Millisecond)

	result := BranchResult{
		StartedAt:   start,
		CompletedAt: end,
	}

	assert.Equal(t, 2*time.Second+500*time.Millisecond, result.Duration())
}

func TestBranchResult_Duration_ZeroTime(t *testing.T) {
	result := BranchResult{}

	// Zero times result in zero duration
	assert.Equal(t, time.Duration(0), result.Duration())
}

func TestBranchResult_Fields(t *testing.T) {
	result := BranchResult{
		Name:     "test-branch",
		Output:   "stdout content",
		Stderr:   "stderr content",
		ExitCode: 42,
	}

	assert.Equal(t, "test-branch", result.Name)
	assert.Equal(t, "stdout content", result.Output)
	assert.Equal(t, "stderr content", result.Stderr)
	assert.Equal(t, 42, result.ExitCode)
}

func TestNewParallelResult(t *testing.T) {
	result := NewParallelResult()

	assert.NotNil(t, result)
	assert.NotNil(t, result.Results)
	assert.Empty(t, result.Results)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
	assert.Nil(t, result.FirstError)
	assert.False(t, result.StartedAt.IsZero())
}

func TestParallelResult_AddResult_Success(t *testing.T) {
	pr := NewParallelResult()

	pr.AddResult(&BranchResult{Name: "step1", ExitCode: 0})
	pr.AddResult(&BranchResult{Name: "step2", ExitCode: 0})

	assert.Equal(t, 2, pr.SuccessCount)
	assert.Equal(t, 0, pr.FailureCount)
	assert.Nil(t, pr.FirstError)
	assert.Len(t, pr.Results, 2)
}

func TestParallelResult_AddResult_Failure(t *testing.T) {
	pr := NewParallelResult()
	err1 := errors.New("first error")
	err2 := errors.New("second error")

	pr.AddResult(&BranchResult{Name: "step1", ExitCode: 1, Error: err1})
	pr.AddResult(&BranchResult{Name: "step2", ExitCode: 1, Error: err2})

	assert.Equal(t, 0, pr.SuccessCount)
	assert.Equal(t, 2, pr.FailureCount)
	assert.Equal(t, err1, pr.FirstError, "FirstError should be the first one added")
}

func TestParallelResult_AddResult_NonZeroExitWithoutError(t *testing.T) {
	pr := NewParallelResult()

	// Non-zero exit code without error is still a failure
	pr.AddResult(&BranchResult{Name: "step1", ExitCode: 1, Error: nil})

	assert.Equal(t, 0, pr.SuccessCount)
	assert.Equal(t, 1, pr.FailureCount)
	assert.Nil(t, pr.FirstError, "FirstError only set when Error field is non-nil")
}

func TestParallelResult_AddResult_Mixed(t *testing.T) {
	pr := NewParallelResult()
	err := errors.New("step2 failed")

	pr.AddResult(&BranchResult{Name: "step1", ExitCode: 0})
	pr.AddResult(&BranchResult{Name: "step2", ExitCode: 1, Error: err})
	pr.AddResult(&BranchResult{Name: "step3", ExitCode: 0})
	pr.AddResult(&BranchResult{Name: "step4", ExitCode: 2})

	assert.Equal(t, 2, pr.SuccessCount)
	assert.Equal(t, 2, pr.FailureCount)
	assert.Equal(t, err, pr.FirstError)
	assert.Len(t, pr.Results, 4)
}

func TestParallelResult_AddResult_Overwrites(t *testing.T) {
	pr := NewParallelResult()

	pr.AddResult(&BranchResult{Name: "step1", ExitCode: 0, Output: "first"})
	pr.AddResult(&BranchResult{Name: "step1", ExitCode: 0, Output: "second"})

	// Same name overwrites, but counts are incremented each time
	assert.Equal(t, 2, pr.SuccessCount, "counts still increment")
	assert.Len(t, pr.Results, 1, "map has single entry")
	assert.Equal(t, "second", pr.Results["step1"].Output)
}

func TestParallelResult_Duration(t *testing.T) {
	pr := NewParallelResult()
	pr.StartedAt = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	pr.CompletedAt = pr.StartedAt.Add(5 * time.Second)

	assert.Equal(t, 5*time.Second, pr.Duration())
}

func TestParallelResult_AllSucceeded(t *testing.T) {
	tests := []struct {
		name         string
		successCount int
		failureCount int
		expected     bool
	}{
		{"all succeed", 3, 0, true},
		{"one fails", 2, 1, false},
		{"all fail", 0, 3, false},
		{"empty (no branches)", 0, 0, false}, // no successes means not "all succeeded"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &ParallelResult{
				SuccessCount: tt.successCount,
				FailureCount: tt.failureCount,
			}
			assert.Equal(t, tt.expected, pr.AllSucceeded())
		})
	}
}

func TestParallelResult_AnySucceeded(t *testing.T) {
	tests := []struct {
		name         string
		successCount int
		failureCount int
		expected     bool
	}{
		{"all succeed", 3, 0, true},
		{"one succeeds", 1, 2, true},
		{"all fail", 0, 3, false},
		{"empty", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &ParallelResult{
				SuccessCount: tt.successCount,
				FailureCount: tt.failureCount,
			}
			assert.Equal(t, tt.expected, pr.AnySucceeded())
		})
	}
}

func TestParallelResult_GetBranchOutput(t *testing.T) {
	pr := NewParallelResult()
	pr.AddResult(&BranchResult{Name: "step_a", Output: "output_a", ExitCode: 0})
	pr.AddResult(&BranchResult{Name: "step_b", Output: "output_b", ExitCode: 0})

	// Access individual outputs
	assert.Equal(t, "output_a", pr.Results["step_a"].Output)
	assert.Equal(t, "output_b", pr.Results["step_b"].Output)

	// Non-existent branch
	_, exists := pr.Results["nonexistent"]
	assert.False(t, exists)
}
