package workflow

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// LoopType Tests
// =============================================================================

func TestLoopType_String(t *testing.T) {
	tests := []struct {
		loopType LoopType
		want     string
	}{
		{LoopTypeForEach, "for_each"},
		{LoopTypeWhile, "while"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.loopType.String())
		})
	}
}

// =============================================================================
// LoopConfig Tests
// =============================================================================

func TestLoopConfig_Validate_ForEach(t *testing.T) {
	tests := []struct {
		name    string
		config  LoopConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid for_each with items",
			config: LoopConfig{
				Type:          LoopTypeForEach,
				Items:         `["a", "b", "c"]`,
				Body:          []string{"process_item"},
				MaxIterations: 100,
				OnComplete:    "done",
			},
			wantErr: false,
		},
		{
			name: "valid for_each with template items",
			config: LoopConfig{
				Type:          LoopTypeForEach,
				Items:         "{{inputs.files}}",
				Body:          []string{"process"},
				MaxIterations: 50,
			},
			wantErr: false,
		},
		{
			name: "for_each missing items",
			config: LoopConfig{
				Type: LoopTypeForEach,
				Body: []string{"process"},
			},
			wantErr: true,
			errMsg:  "items",
		},
		{
			name: "for_each missing body",
			config: LoopConfig{
				Type:  LoopTypeForEach,
				Items: `["a"]`,
			},
			wantErr: true,
			errMsg:  "body",
		},
		{
			name: "for_each empty body",
			config: LoopConfig{
				Type:  LoopTypeForEach,
				Items: `["a"]`,
				Body:  []string{},
			},
			wantErr: true,
			errMsg:  "body",
		},
		{
			name: "for_each max_iterations exceeds limit",
			config: LoopConfig{
				Type:          LoopTypeForEach,
				Items:         `["a"]`,
				Body:          []string{"process"},
				MaxIterations: MaxAllowedIterations + 1,
			},
			wantErr: true,
			errMsg:  "max_iterations",
		},
		{
			name: "for_each negative max_iterations",
			config: LoopConfig{
				Type:          LoopTypeForEach,
				Items:         `["a"]`,
				Body:          []string{"process"},
				MaxIterations: -1,
			},
			wantErr: true,
			errMsg:  "max_iterations",
		},
		{
			name: "for_each with break condition",
			config: LoopConfig{
				Type:           LoopTypeForEach,
				Items:          `["a", "b"]`,
				Body:           []string{"process"},
				MaxIterations:  100,
				BreakCondition: "states.process.output == 'done'",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoopConfig_Validate_While(t *testing.T) {
	tests := []struct {
		name    string
		config  LoopConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid while with condition",
			config: LoopConfig{
				Type:          LoopTypeWhile,
				Condition:     "states.check.output != 'ready'",
				Body:          []string{"check", "wait"},
				MaxIterations: 60,
				OnComplete:    "proceed",
			},
			wantErr: false,
		},
		{
			name: "while missing condition",
			config: LoopConfig{
				Type: LoopTypeWhile,
				Body: []string{"check"},
			},
			wantErr: true,
			errMsg:  "condition",
		},
		{
			name: "while missing body",
			config: LoopConfig{
				Type:      LoopTypeWhile,
				Condition: "true",
			},
			wantErr: true,
			errMsg:  "body",
		},
		{
			name: "while empty body",
			config: LoopConfig{
				Type:      LoopTypeWhile,
				Condition: "true",
				Body:      []string{},
			},
			wantErr: true,
			errMsg:  "body",
		},
		{
			name: "while max_iterations exceeds limit",
			config: LoopConfig{
				Type:          LoopTypeWhile,
				Condition:     "true",
				Body:          []string{"check"},
				MaxIterations: MaxAllowedIterations + 1,
			},
			wantErr: true,
			errMsg:  "max_iterations",
		},
		{
			name: "while with break condition",
			config: LoopConfig{
				Type:           LoopTypeWhile,
				Condition:      "true",
				Body:           []string{"poll"},
				MaxIterations:  100,
				BreakCondition: "states.poll.exit_code != 0",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoopConfig_Validate_InvalidType(t *testing.T) {
	config := LoopConfig{
		Type:  LoopType("invalid"),
		Items: `["a"]`,
		Body:  []string{"process"},
	}

	err := config.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type")
}

// =============================================================================
// Constants Tests
// =============================================================================

func TestLoopConstants(t *testing.T) {
	assert.Equal(t, 100, DefaultMaxIterations)
	assert.Equal(t, 10000, MaxAllowedIterations)
	assert.Greater(t, MaxAllowedIterations, DefaultMaxIterations)
}

// =============================================================================
// IterationResult Tests
// =============================================================================

func TestIterationResult_Duration(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(2*time.Second + 500*time.Millisecond)

	result := IterationResult{
		StartedAt:   start,
		CompletedAt: end,
	}

	assert.Equal(t, 2*time.Second+500*time.Millisecond, result.Duration())
}

func TestIterationResult_Duration_ZeroTime(t *testing.T) {
	result := IterationResult{}
	assert.Equal(t, time.Duration(0), result.Duration())
}

func TestIterationResult_Success(t *testing.T) {
	tests := []struct {
		name     string
		result   IterationResult
		expected bool
	}{
		{
			name:     "success with no error",
			result:   IterationResult{Error: nil},
			expected: true,
		},
		{
			name:     "failure with error",
			result:   IterationResult{Error: errors.New("failed")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.Success())
		})
	}
}

func TestIterationResult_Fields(t *testing.T) {
	err := errors.New("test error")
	start := time.Now()
	end := start.Add(time.Second)

	result := IterationResult{
		Index: 5,
		Item:  "test-item",
		StepResults: map[string]*StepState{
			"step1": {Name: "step1", Status: StatusCompleted},
		},
		Error:       err,
		StartedAt:   start,
		CompletedAt: end,
	}

	assert.Equal(t, 5, result.Index)
	assert.Equal(t, "test-item", result.Item)
	assert.Len(t, result.StepResults, 1)
	assert.Equal(t, err, result.Error)
	assert.Equal(t, start, result.StartedAt)
	assert.Equal(t, end, result.CompletedAt)
}

// =============================================================================
// LoopResult Tests
// =============================================================================

func TestNewLoopResult(t *testing.T) {
	result := NewLoopResult()

	require.NotNil(t, result)
	assert.NotNil(t, result.Iterations)
	assert.Empty(t, result.Iterations)
	assert.Equal(t, 0, result.TotalCount)
	assert.Equal(t, -1, result.BrokeAt)
	assert.False(t, result.StartedAt.IsZero())
	assert.True(t, result.CompletedAt.IsZero())
}

func TestLoopResult_Duration(t *testing.T) {
	result := NewLoopResult()
	result.StartedAt = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	result.CompletedAt = result.StartedAt.Add(5 * time.Second)

	assert.Equal(t, 5*time.Second, result.Duration())
}

func TestLoopResult_WasBroken(t *testing.T) {
	tests := []struct {
		name     string
		brokeAt  int
		expected bool
	}{
		{"completed normally", -1, false},
		{"broke at iteration 0", 0, true},
		{"broke at iteration 5", 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &LoopResult{BrokeAt: tt.brokeAt}
			assert.Equal(t, tt.expected, result.WasBroken())
		})
	}
}

func TestLoopResult_AllSucceeded(t *testing.T) {
	tests := []struct {
		name       string
		iterations []IterationResult
		expected   bool
	}{
		{
			name: "all succeed",
			iterations: []IterationResult{
				{Error: nil},
				{Error: nil},
				{Error: nil},
			},
			expected: true,
		},
		{
			name: "one fails",
			iterations: []IterationResult{
				{Error: nil},
				{Error: errors.New("failed")},
				{Error: nil},
			},
			expected: false,
		},
		{
			name: "all fail",
			iterations: []IterationResult{
				{Error: errors.New("failed 1")},
				{Error: errors.New("failed 2")},
			},
			expected: false,
		},
		{
			name:       "empty iterations",
			iterations: []IterationResult{},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &LoopResult{Iterations: tt.iterations}
			assert.Equal(t, tt.expected, result.AllSucceeded())
		})
	}
}

func TestLoopResult_AddIteration(t *testing.T) {
	result := NewLoopResult()

	// Add successful iteration
	iter1 := IterationResult{
		Index: 0,
		Item:  "item1",
		Error: nil,
	}
	result.Iterations = append(result.Iterations, iter1)
	result.TotalCount++

	// Add failed iteration
	iter2 := IterationResult{
		Index: 1,
		Item:  "item2",
		Error: errors.New("failed"),
	}
	result.Iterations = append(result.Iterations, iter2)
	result.TotalCount++

	assert.Len(t, result.Iterations, 2)
	assert.Equal(t, 2, result.TotalCount)
	assert.False(t, result.AllSucceeded())
}

// =============================================================================
// Integration-style Tests
// =============================================================================

func TestLoopConfig_WithAllFields(t *testing.T) {
	config := LoopConfig{
		Type:           LoopTypeForEach,
		Items:          `["file1.txt", "file2.txt", "file3.txt"]`,
		Condition:      "",
		Body:           []string{"process_file", "validate_output"},
		MaxIterations:  50,
		BreakCondition: "states.validate_output.exit_code != 0",
		OnComplete:     "aggregate_results",
	}

	assert.Equal(t, LoopTypeForEach, config.Type)
	assert.NotEmpty(t, config.Items)
	assert.Len(t, config.Body, 2)
	assert.Equal(t, 50, config.MaxIterations)
	assert.NotEmpty(t, config.BreakCondition)
	assert.Equal(t, "aggregate_results", config.OnComplete)

	err := config.Validate()
	require.NoError(t, err)
}

func TestLoopResult_CompleteWorkflow(t *testing.T) {
	result := NewLoopResult()

	// Simulate 3 iterations
	items := []string{"a.txt", "b.txt", "c.txt"}
	for i, item := range items {
		iterStart := time.Now()
		iter := IterationResult{
			Index: i,
			Item:  item,
			StepResults: map[string]*StepState{
				"process": {
					Name:   "process",
					Status: StatusCompleted,
					Output: "processed " + item,
				},
			},
			Error:       nil,
			StartedAt:   iterStart,
			CompletedAt: iterStart.Add(100 * time.Millisecond),
		}
		result.Iterations = append(result.Iterations, iter)
		result.TotalCount++
	}
	result.CompletedAt = time.Now()

	assert.Equal(t, 3, result.TotalCount)
	assert.Equal(t, -1, result.BrokeAt)
	assert.False(t, result.WasBroken())
	assert.True(t, result.AllSucceeded())

	// Verify each iteration
	for i, iter := range result.Iterations {
		assert.Equal(t, i, iter.Index)
		assert.True(t, iter.Success())
		assert.Greater(t, iter.Duration(), time.Duration(0))
	}
}

func TestLoopResult_BrokenAtIteration(t *testing.T) {
	result := NewLoopResult()

	// Simulate break at iteration 2
	for i := 0; i < 3; i++ {
		iter := IterationResult{
			Index: i,
			Item:  i,
		}
		if i == 2 {
			// Break condition met
			result.BrokeAt = i
		}
		result.Iterations = append(result.Iterations, iter)
		result.TotalCount++
	}
	result.CompletedAt = time.Now()

	assert.Equal(t, 3, result.TotalCount)
	assert.Equal(t, 2, result.BrokeAt)
	assert.True(t, result.WasBroken())
}
