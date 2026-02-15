package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/stretchr/testify/assert"
)

// TestExecutionService_detectLoopPatterns tests the detectLoopPatterns helper method
// that examines loop iterations to detect step failures and complex step types.
// Feature: C006 - Component T001
func TestExecutionService_detectLoopPatterns(t *testing.T) {
	tests := []struct {
		name              string
		result            *workflow.LoopResult
		wf                *workflow.Workflow
		wantHadFailures   bool
		wantHasComplexity bool
	}{
		{
			name: "no failures and no complexity - empty iterations",
			result: &workflow.LoopResult{
				Iterations: []workflow.IterationResult{
					{
						StepResults: map[string]*workflow.StepState{
							"step1": {Status: workflow.StatusCompleted},
							"step2": {Status: workflow.StatusCompleted},
						},
					},
				},
			},
			wf: &workflow.Workflow{
				Steps: map[string]*workflow.Step{
					"step1": {Type: workflow.StepTypeCommand},
					"step2": {Type: workflow.StepTypeCommand},
				},
			},
			wantHadFailures:   false,
			wantHasComplexity: false,
		},
		{
			name: "only failures - steps with StatusFailed",
			result: &workflow.LoopResult{
				Iterations: []workflow.IterationResult{
					{
						StepResults: map[string]*workflow.StepState{
							"step1": {Status: workflow.StatusFailed},
							"step2": {Status: workflow.StatusCompleted},
						},
					},
				},
			},
			wf: &workflow.Workflow{
				Steps: map[string]*workflow.Step{
					"step1": {Type: workflow.StepTypeCommand},
					"step2": {Type: workflow.StepTypeCommand},
				},
			},
			wantHadFailures:   true,
			wantHasComplexity: false,
		},
		{
			name: "only complex steps - while/foreach/parallel/callworkflow",
			result: &workflow.LoopResult{
				Iterations: []workflow.IterationResult{
					{
						StepResults: map[string]*workflow.StepState{
							"step1": {Status: workflow.StatusCompleted},
							"step2": {Status: workflow.StatusCompleted},
						},
					},
				},
			},
			wf: &workflow.Workflow{
				Steps: map[string]*workflow.Step{
					"step1": {Type: workflow.StepTypeWhile},
					"step2": {Type: workflow.StepTypeCommand},
				},
			},
			wantHadFailures:   false,
			wantHasComplexity: true,
		},
		{
			name: "both conditions - failures and complex steps",
			result: &workflow.LoopResult{
				Iterations: []workflow.IterationResult{
					{
						StepResults: map[string]*workflow.StepState{
							"step1": {Status: workflow.StatusFailed},
							"step2": {Status: workflow.StatusCompleted},
						},
					},
				},
			},
			wf: &workflow.Workflow{
				Steps: map[string]*workflow.Step{
					"step1": {Type: workflow.StepTypeParallel},
					"step2": {Type: workflow.StepTypeCommand},
				},
			},
			wantHadFailures:   true,
			wantHasComplexity: true,
		},
		{
			name:              "nil result - should return false gracefully",
			result:            nil,
			wf:                &workflow.Workflow{},
			wantHadFailures:   false,
			wantHasComplexity: false,
		},
		{
			name: "nil iterations - should return false gracefully",
			result: &workflow.LoopResult{
				Iterations: nil,
			},
			wf:                &workflow.Workflow{},
			wantHadFailures:   false,
			wantHasComplexity: false,
		},
		{
			name: "early exit optimization - should stop when both found",
			result: &workflow.LoopResult{
				Iterations: []workflow.IterationResult{
					{
						StepResults: map[string]*workflow.StepState{
							"step1": {Status: workflow.StatusFailed},
							"step2": {Status: workflow.StatusCompleted},
						},
					},
					{
						StepResults: map[string]*workflow.StepState{
							"step3": {Status: workflow.StatusCompleted},
						},
					},
				},
			},
			wf: &workflow.Workflow{
				Steps: map[string]*workflow.Step{
					"step1": {Type: workflow.StepTypeForEach},
					"step2": {Type: workflow.StepTypeCommand},
					"step3": {Type: workflow.StepTypeCommand},
				},
			},
			wantHadFailures:   true,
			wantHasComplexity: true,
		},
		{
			name: "nil step state - should skip gracefully",
			result: &workflow.LoopResult{
				Iterations: []workflow.IterationResult{
					{
						StepResults: map[string]*workflow.StepState{
							"step1": nil,
							"step2": {Status: workflow.StatusCompleted},
						},
					},
				},
			},
			wf: &workflow.Workflow{
				Steps: map[string]*workflow.Step{
					"step1": {Type: workflow.StepTypeCommand},
					"step2": {Type: workflow.StepTypeCommand},
				},
			},
			wantHadFailures:   false,
			wantHasComplexity: false,
		},
		{
			name: "nil workflow - should skip complexity check",
			result: &workflow.LoopResult{
				Iterations: []workflow.IterationResult{
					{
						StepResults: map[string]*workflow.StepState{
							"step1": {Status: workflow.StatusCompleted},
						},
					},
				},
			},
			wf:                nil,
			wantHadFailures:   false,
			wantHasComplexity: false,
		},
		{
			name: "step not found in workflow - should skip complexity check",
			result: &workflow.LoopResult{
				Iterations: []workflow.IterationResult{
					{
						StepResults: map[string]*workflow.StepState{
							"unknown_step": {Status: workflow.StatusCompleted},
						},
					},
				},
			},
			wf: &workflow.Workflow{
				Steps: map[string]*workflow.Step{
					"step1": {Type: workflow.StepTypeCommand},
				},
			},
			wantHadFailures:   false,
			wantHasComplexity: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execSvc := &ExecutionService{outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits())}

			gotHadFailures, gotHasComplexity := execSvc.detectLoopPatterns(tt.result, tt.wf)

			assert.Equal(t, tt.wantHadFailures, gotHadFailures, "hadFailures mismatch")
			assert.Equal(t, tt.wantHasComplexity, gotHasComplexity, "hasComplexSteps mismatch")
		})
	}
}

// TestExecutionService_shouldCheckLoopProblems tests the shouldCheckLoopProblems guard helper
// that determines if we should analyze a loop for problematic patterns.
// Feature: C006 - Component T003
func TestExecutionService_shouldCheckLoopProblems(t *testing.T) {
	tests := []struct {
		name   string
		result *workflow.LoopResult
		step   *workflow.Step
		want   bool
	}{
		{
			name:   "nil result - returns false",
			result: nil,
			step: &workflow.Step{
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{MaxIterations: 10},
			},
			want: false,
		},
		{
			name: "non-while step - returns false",
			result: &workflow.LoopResult{
				TotalCount: 10,
				BrokeAt:    -1,
			},
			step: &workflow.Step{
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{MaxIterations: 10},
			},
			want: false,
		},
		{
			name: "no max iterations configured - returns false",
			result: &workflow.LoopResult{
				TotalCount: 5,
				BrokeAt:    -1,
			},
			step: &workflow.Step{
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{MaxIterations: 0},
			},
			want: false,
		},
		{
			name: "negative max iterations - returns false",
			result: &workflow.LoopResult{
				TotalCount: 5,
				BrokeAt:    -1,
			},
			step: &workflow.Step{
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{MaxIterations: -1},
			},
			want: false,
		},
		{
			name: "loop broke early (BrokeAt != -1) - returns false",
			result: &workflow.LoopResult{
				TotalCount: 5,
				BrokeAt:    3,
			},
			step: &workflow.Step{
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{MaxIterations: 10},
			},
			want: false,
		},
		{
			name: "loop hit max iterations - returns true",
			result: &workflow.LoopResult{
				TotalCount: 10,
				BrokeAt:    -1,
			},
			step: &workflow.Step{
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{MaxIterations: 10},
			},
			want: true,
		},
		{
			name: "loop completed fewer iterations than max - returns false",
			result: &workflow.LoopResult{
				TotalCount: 8,
				BrokeAt:    -1,
			},
			step: &workflow.Step{
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{MaxIterations: 10},
			},
			want: false,
		},
		{
			name: "loop completed exactly at max iterations - returns true",
			result: &workflow.LoopResult{
				TotalCount: 100,
				BrokeAt:    -1,
			},
			step: &workflow.Step{
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{MaxIterations: 100},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execSvc := &ExecutionService{outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits())}

			got := execSvc.shouldCheckLoopProblems(tt.result, tt.step)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestIsComplexStepType tests the isComplexStepType utility function
// that identifies complex step types that make loop debugging difficult.
// Feature: C006 - Component T001
func TestIsComplexStepType(t *testing.T) {
	tests := []struct {
		name     string
		stepType workflow.StepType
		want     bool
	}{
		{
			name:     "StepTypeWhile - returns true",
			stepType: workflow.StepTypeWhile,
			want:     true,
		},
		{
			name:     "StepTypeForEach - returns true",
			stepType: workflow.StepTypeForEach,
			want:     true,
		},
		{
			name:     "StepTypeParallel - returns true",
			stepType: workflow.StepTypeParallel,
			want:     true,
		},
		{
			name:     "StepTypeCallWorkflow - returns true",
			stepType: workflow.StepTypeCallWorkflow,
			want:     true,
		},
		{
			name:     "StepTypeCommand - returns false",
			stepType: workflow.StepTypeCommand,
			want:     false,
		},
		{
			name:     "StepTypeTerminal - returns false",
			stepType: workflow.StepTypeTerminal,
			want:     false,
		},
		{
			name:     "StepTypeOperation - returns false",
			stepType: workflow.StepTypeOperation,
			want:     false,
		},
		{
			name:     "StepTypeAgent - returns false",
			stepType: workflow.StepTypeAgent,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isComplexStepType(tt.stepType)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExecutionService_buildLoopFailureError tests the buildLoopFailureError helper
// that constructs descriptive error messages based on detected loop patterns.
// Feature: C006 - Component T006
func TestExecutionService_buildLoopFailureError(t *testing.T) {
	tests := []struct {
		name            string
		hadFailures     bool
		hasComplexSteps bool
		want            string
	}{
		{
			name:            "failures only - appends 'with step failures'",
			hadFailures:     true,
			hasComplexSteps: false,
			want:            "loop reached maximum iterations with step failures",
		},
		{
			name:            "complexity only - appends 'with nested complexity'",
			hadFailures:     false,
			hasComplexSteps: true,
			want:            "loop reached maximum iterations with nested complexity",
		},
		{
			name:            "both conditions - failures takes precedence",
			hadFailures:     true,
			hasComplexSteps: true,
			want:            "loop reached maximum iterations with step failures",
		},
		{
			name:            "neither condition - base message only",
			hadFailures:     false,
			hasComplexSteps: false,
			want:            "loop reached maximum iterations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execSvc := &ExecutionService{outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits())}

			got := execSvc.buildLoopFailureError(tt.hadFailures, tt.hasComplexSteps)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExecutionService_executeLoopPostHooks tests the executeLoopPostHooks helper
// that executes post-failure hooks and handles errors gracefully.
// Feature: C006 - Component T006
func TestExecutionService_executeLoopPostHooks(t *testing.T) {
	tests := []struct {
		name             string
		step             *workflow.Step
		execCtx          *workflow.ExecutionContext
		setupMocks       func() (*loopTestLogger, *HookExecutor)
		expectWarningLog bool
	}{
		{
			name: "hooks execute successfully - no warning",
			step: &workflow.Step{
				Name: "test_loop",
				Hooks: workflow.StepHooks{
					Post: workflow.Hook{
						workflow.HookAction{Command: "echo 'cleanup'"},
					},
				},
			},
			execCtx: &workflow.ExecutionContext{},
			setupMocks: func() (*loopTestLogger, *HookExecutor) {
				logger := &loopTestLogger{logs: []logEntry{}}
				mockExec := &loopTestCommandExecutor{executeErr: nil}
				resolver := interpolation.NewTemplateResolver()
				hookExec := NewHookExecutor(mockExec, logger, resolver)
				return logger, hookExec
			},
			expectWarningLog: false,
		},
		{
			name: "hooks fail - warning logged but no panic",
			step: &workflow.Step{
				Name: "test_loop",
				Hooks: workflow.StepHooks{
					Post: workflow.Hook{
						workflow.HookAction{Command: "exit 1"},
					},
				},
			},
			execCtx: &workflow.ExecutionContext{},
			setupMocks: func() (*loopTestLogger, *HookExecutor) {
				logger := &loopTestLogger{logs: []logEntry{}}
				mockExec := &loopTestCommandExecutor{executeErr: fmt.Errorf("hook execution failed")}
				resolver := interpolation.NewTemplateResolver()
				hookExec := NewHookExecutor(mockExec, logger, resolver)
				return logger, hookExec
			},
			expectWarningLog: true,
		},
		{
			name: "empty hooks - no action",
			step: &workflow.Step{
				Name:  "test_loop",
				Hooks: workflow.StepHooks{},
			},
			execCtx: &workflow.ExecutionContext{},
			setupMocks: func() (*loopTestLogger, *HookExecutor) {
				logger := &loopTestLogger{logs: []logEntry{}}
				mockExec := &loopTestCommandExecutor{executeErr: nil}
				resolver := interpolation.NewTemplateResolver()
				hookExec := NewHookExecutor(mockExec, logger, resolver)
				return logger, hookExec
			},
			expectWarningLog: false,
		},
		{
			name: "nil step hooks - should handle gracefully",
			step: &workflow.Step{
				Name: "test_loop",
			},
			execCtx: &workflow.ExecutionContext{},
			setupMocks: func() (*loopTestLogger, *HookExecutor) {
				logger := &loopTestLogger{logs: []logEntry{}}
				mockExec := &loopTestCommandExecutor{executeErr: nil}
				resolver := interpolation.NewTemplateResolver()
				hookExec := NewHookExecutor(mockExec, logger, resolver)
				return logger, hookExec
			},
			expectWarningLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, hookExec := tt.setupMocks()

			execSvc := &ExecutionService{
				outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
				logger:        logger,
				hookExecutor:  hookExec,
			}

			assert.NotPanics(t, func() {
				execSvc.executeLoopPostHooks(context.Background(), tt.step, tt.execCtx)
			})

			if tt.expectWarningLog {
				// Should have logged a warning
				hasWarning := false
				for _, entry := range logger.logs {
					if entry.level == "warn" && entry.msg == "post-hook failed" {
						hasWarning = true
						break
					}
				}
				assert.True(t, hasWarning, "expected warning log for hook failure")
			} else {
				// Should not have logged a warning
				for _, entry := range logger.logs {
					if entry.level == "warn" && entry.msg == "post-hook failed" {
						assert.Fail(t, "unexpected warning log")
					}
				}
			}
		})
	}
}

// Mock implementations for loop pattern helper testing (to avoid conflicts with other test files)

type logEntry struct {
	level string
	msg   string
}

type loopTestLogger struct {
	logs []logEntry
}

func (m *loopTestLogger) Info(msg string, fields ...any) {
	m.logs = append(m.logs, logEntry{level: "info", msg: msg})
}

func (m *loopTestLogger) Warn(msg string, fields ...any) {
	m.logs = append(m.logs, logEntry{level: "warn", msg: msg})
}

func (m *loopTestLogger) Error(msg string, fields ...any) {
	m.logs = append(m.logs, logEntry{level: "error", msg: msg})
}

func (m *loopTestLogger) Debug(msg string, fields ...any) {
	m.logs = append(m.logs, logEntry{level: "debug", msg: msg})
}

func (m *loopTestLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

type loopTestCommandExecutor struct {
	executeErr error
	onExecute  func(cmd string) // Callback for tracking command execution
}

func (m *loopTestCommandExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	// Call the onExecute callback if set
	if m.onExecute != nil {
		m.onExecute(cmd.Program)
	}

	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return &ports.CommandResult{
		ExitCode: 0,
		Stdout:   "",
		Stderr:   "",
	}, nil
}
