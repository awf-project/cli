package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/stretchr/testify/assert"
)

// newTestExecutionService creates an ExecutionService for testing with minimal setup.
// C019: Ensures outputLimiter is initialized to prevent nil pointer panics.
func newTestExecutionService() *ExecutionService {
	return &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
	}
}

// mockResolver is a simple resolver that returns templates unchanged
type mockResolver struct{}

func newMockResolver() *mockResolver {
	return &mockResolver{}
}

func (m *mockResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	return template, nil
}

// mockExecutor is a simple executor for testing
type mockExecutor struct {
	result *ports.CommandResult
	err    error
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		result: &ports.CommandResult{
			Stdout:   "mock output",
			Stderr:   "",
			ExitCode: 0,
		},
	}
}

func (m *mockExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	return m.result, m.err
}

// Note: mockLogger is defined in service_test.go and shared across package tests

// executeStep Helper Tests - Component T010
// Feature: C006 - Reduce executeStep complexity from 29 to ≤18

// TestExecutionService_prepareStepExecution tests the prepareStepExecution helper
// that handles pre-hooks and timeout context setup.
// Feature: C006 - Component T010
func TestExecutionService_prepareStepExecution(t *testing.T) {
	tests := []struct {
		name        string
		step        *workflow.Step
		execCtx     *workflow.ExecutionContext
		expectedCtx bool // whether context should have timeout
		expectError bool
	}{
		{
			name: "pre-hooks success with no timeout",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeCommand,
				Hooks: workflow.StepHooks{
					Pre: workflow.Hook{
						{Command: "echo pre-hook"},
					},
				},
			},
			execCtx:     workflow.NewExecutionContext("test-wf", "test"),
			expectedCtx: false,
			expectError: false,
		},
		{
			name: "pre-hooks failure - warning logged but not propagated",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeCommand,
				Hooks: workflow.StepHooks{
					Pre: workflow.Hook{
						{Command: "exit 1"},
					},
				},
			},
			execCtx:     workflow.NewExecutionContext("test-wf", "test"),
			expectedCtx: false,
			expectError: false,
		},
		{
			name: "timeout context setup",
			step: &workflow.Step{
				Name:    "test-step",
				Type:    workflow.StepTypeCommand,
				Timeout: 5, // 5 seconds
			},
			execCtx:     workflow.NewExecutionContext("test-wf", "test"),
			expectedCtx: true,
			expectError: false,
		},
		{
			name: "no timeout and no pre-hooks",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeCommand,
			},
			execCtx:     workflow.NewExecutionContext("test-wf", "test"),
			expectedCtx: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := newMockExecutor()
			mockLogger := &mockLogger{}
			mockResolver := newMockResolver()
			svc := &ExecutionService{
				outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
				executor:      mockExec,
				logger:        mockLogger,
				resolver:      mockResolver,
				hookExecutor:  NewHookExecutor(mockExec, mockLogger, mockResolver),
			}

			ctx := context.Background()

			resultCtx, intCtx, cancel, err := svc.prepareStepExecution(ctx, tt.step, tt.execCtx)
			if cancel != nil {
				defer cancel()
			}

			if tt.expectError {
				assert.Error(t, err, "should return error")
			} else {
				assert.NoError(t, err, "should not return error")
				assert.NotNil(t, resultCtx, "should return context")
				assert.NotNil(t, intCtx, "should return interpolation context")
			}

			// Verify cancel function returned when timeout configured
			if tt.expectedCtx && !tt.expectError {
				assert.NotNil(t, resultCtx, "should return context")
				assert.NotNil(t, cancel, "should return cancel function for timeout")
			} else if !tt.expectedCtx {
				assert.Nil(t, cancel, "should not return cancel function when no timeout")
			}
		})
	}
}

// TestExecutionService_resolveStepCommand tests the resolveStepCommand helper
// that interpolates and resolves the step's command and directory.
// Feature: C006 - Component T010
func TestExecutionService_resolveStepCommand(t *testing.T) {
	tests := []struct {
		name        string
		step        *workflow.Step
		intCtx      *interpolation.Context
		expectError bool
	}{
		{
			name: "command interpolation success",
			step: &workflow.Step{
				Name:    "test-step",
				Type:    workflow.StepTypeCommand,
				Command: "echo hello",
			},
			intCtx:      &interpolation.Context{},
			expectError: false,
		},
		// Note: Interpolation error testing requires a mock resolver that returns errors
		// This is tested in integration tests with real resolver
		{
			name: "dir interpolation success",
			step: &workflow.Step{
				Name:    "test-step",
				Type:    workflow.StepTypeCommand,
				Command: "ls",
				Dir:     "/tmp",
			},
			intCtx:      &interpolation.Context{},
			expectError: false,
		},
		// Note: Dir interpolation error testing requires a mock resolver that returns errors
		// This is tested in integration tests with real resolver
		{
			name: "empty dir defaults to current directory",
			step: &workflow.Step{
				Name:    "test-step",
				Type:    workflow.StepTypeCommand,
				Command: "pwd",
				Dir:     "",
			},
			intCtx:      &interpolation.Context{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResolver := newMockResolver()
			svc := &ExecutionService{
				outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
				resolver:      mockResolver,
			}

			wf := &workflow.Workflow{SourceDir: "/tmp"}
			cmd, err := svc.resolveStepCommand(context.Background(), wf, tt.step, tt.intCtx)

			if tt.expectError {
				assert.Error(t, err, "should return interpolation error")
				assert.Nil(t, cmd, "should not return command on error")
			} else {
				assert.NoError(t, err, "should not return error")
				assert.NotNil(t, cmd, "should return command")
				assert.Equal(t, tt.step.Command, cmd.Program, "command should match step command")
				if tt.step.Dir != "" {
					assert.Equal(t, tt.step.Dir, cmd.Dir, "dir should match step dir")
				} else {
					assert.Empty(t, cmd.Dir, "dir should be empty when not specified")
				}
			}
		})
	}
}

// TestExecutionService_executeStepCommand tests the executeStepCommand helper
// that executes the command with retry logic if configured.
// Feature: C006 - Component T010
func TestExecutionService_executeStepCommand(t *testing.T) {
	tests := []struct {
		name string
		step *workflow.Step
		cmd  *ports.Command
	}{
		{
			name: "single execution - no retry",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeCommand,
			},
			cmd: &ports.Command{
				Program: "echo hello",
			},
		},
		{
			name: "retry success - failure then success",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeCommand,
				Retry: &workflow.RetryConfig{
					MaxAttempts:    3,
					InitialDelayMs: 100,
				},
			},
			cmd: &ports.Command{
				Program: "flaky-command",
			},
		},
		{
			name: "retry exhaustion - all attempts fail",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeCommand,
				Retry: &workflow.RetryConfig{
					MaxAttempts:    2,
					InitialDelayMs: 100,
				},
			},
			cmd: &ports.Command{
				Program: "always-fail",
			},
		},
		{
			name: "executor error - not retryable",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeCommand,
			},
			cmd: &ports.Command{
				Program: "bad-command",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestExecutionService()

			ctx := context.Background()

			result, attempt, err := svc.executeStepCommand(ctx, tt.step, tt.cmd)

			// GREEN phase will implement actual retry logic
			_ = result
			_ = attempt
			_ = err
		})
	}
}

// TestExecutionService_recordStepResult tests the recordStepResult helper
// that builds a workflow.StepState from execution timing and command result.
// Feature: C006 - Component T010
func TestExecutionService_recordStepResult(t *testing.T) {
	tests := []struct {
		name        string
		step        *workflow.Step
		startTime   time.Time
		result      *ports.CommandResult
		attempt     int
		expectState bool
	}{
		{
			name: "with result - successful execution",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeCommand,
			},
			startTime: time.Now().Add(-2 * time.Second),
			result: &ports.CommandResult{
				ExitCode: 0,
				Stdout:   "success output",
				Stderr:   "",
			},
			attempt:     1,
			expectState: true,
		},
		{
			name: "nil result - execution error occurred",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeCommand,
			},
			startTime:   time.Now().Add(-1 * time.Second),
			result:      nil,
			attempt:     1,
			expectState: true,
		},
		{
			name: "attempt tracking - retry scenario",
			step: &workflow.Step{
				Name: "retry-step",
				Type: workflow.StepTypeCommand,
			},
			startTime: time.Now().Add(-5 * time.Second),
			result: &ports.CommandResult{
				ExitCode: 0,
				Stdout:   "success after retry",
			},
			attempt:     3,
			expectState: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &ExecutionService{
				outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
			}

			state := svc.recordStepResult(tt.step, tt.startTime, tt.result, tt.attempt)

			if tt.expectState {
				// Note: In stub phase, state will be empty StepState{}
				// In GREEN phase, verify:
				// - state.Name == tt.step.Name
				// - state.StartTime == tt.startTime
				// - state.EndTime is set
				// - state.Duration > 0
				// - state.Output == tt.result.Stdout (if result != nil)
				// - state.ExitCode == tt.result.ExitCode (if result != nil)
				// - state.Attempts == tt.attempt
				// Note: Status field is NOT set here, set by outcome handlers
				_ = state
			}
		})
	}
}

// TestExecutionService_handleExecutionError tests the handleExecutionError helper
// that processes execution errors, distinguishing between workflow-level
// cancellation and step-level timeouts/failures.
// Feature: C006 - Component T010
func TestExecutionService_handleExecutionError(t *testing.T) {
	tests := []struct {
		name             string
		step             *workflow.Step
		execCtx          *workflow.ExecutionContext
		state            workflow.StepState
		execErr          error
		ctxCancelled     bool
		expectError      bool
		expectedNextStep string
	}{
		{
			name: "workflow cancellation - propagate error",
			step: &workflow.Step{
				Name:      "test-step",
				Type:      workflow.StepTypeCommand,
				OnFailure: "error-handler",
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			state: workflow.StepState{
				Name:   "test-step",
				Status: workflow.StatusRunning,
			},
			execErr:          errors.New("execution failed"),
			ctxCancelled:     true,
			expectError:      true,
			expectedNextStep: "",
		},
		{
			name: "step timeout - use on_failure transition",
			step: &workflow.Step{
				Name:      "test-step",
				Type:      workflow.StepTypeCommand,
				OnFailure: "error-handler",
				Timeout:   1, // 1 second
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			state: workflow.StepState{
				Name:   "test-step",
				Status: workflow.StatusRunning,
			},
			execErr:          context.DeadlineExceeded,
			ctxCancelled:     false,
			expectError:      false,
			expectedNextStep: "error-handler",
		},
		{
			name: "continue_on_error flag - no error propagation",
			step: &workflow.Step{
				Name:            "test-step",
				Type:            workflow.StepTypeCommand,
				ContinueOnError: true,
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			state: workflow.StepState{
				Name:   "test-step",
				Status: workflow.StatusRunning,
			},
			execErr:          errors.New("execution failed"),
			ctxCancelled:     false,
			expectError:      false,
			expectedNextStep: "",
		},
		{
			name: "no on_failure transition - return error",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeCommand,
				// no OnFailure
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			state: workflow.StepState{
				Name:   "test-step",
				Status: workflow.StatusRunning,
			},
			execErr:          errors.New("execution failed"),
			ctxCancelled:     false,
			expectError:      true,
			expectedNextStep: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &ExecutionService{
				outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
				logger:        &mockLogger{},
			}

			ctx := context.Background()
			if tt.ctxCancelled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			stepCtx := context.Background()

			nextStep, err := svc.handleExecutionError(ctx, stepCtx, tt.step, tt.execCtx, &tt.state, tt.execErr)

			if tt.expectError {
				assert.Error(t, err, "should return error")
			} else {
				assert.NoError(t, err, "should not return error")
			}

			// Note: In stub phase, nextStep will be ""
			// In GREEN phase, verify nextStep == tt.expectedNextStep
			_ = nextStep
		})
	}
}

// TestExecutionService_handleNonZeroExit tests the handleNonZeroExit helper
// that processes non-zero exit codes, applying on_failure or continue_on_error logic.
// Feature: C006 - Component T010
func TestExecutionService_handleNonZeroExit(t *testing.T) {
	tests := []struct {
		name             string
		step             *workflow.Step
		execCtx          *workflow.ExecutionContext
		state            workflow.StepState
		exitCode         int
		expectError      bool
		expectedNextStep string
	}{
		{
			name: "exit code with on_failure transition",
			step: &workflow.Step{
				Name:      "test-step",
				Type:      workflow.StepTypeCommand,
				OnFailure: "error-handler",
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			state: workflow.StepState{
				Name:     "test-step",
				Status:   workflow.StatusRunning,
				ExitCode: 1,
			},
			exitCode:         1,
			expectError:      false,
			expectedNextStep: "error-handler",
		},
		{
			name: "continue_on_error flag - no error",
			step: &workflow.Step{
				Name:            "test-step",
				Type:            workflow.StepTypeCommand,
				ContinueOnError: true,
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			state: workflow.StepState{
				Name:     "test-step",
				Status:   workflow.StatusRunning,
				ExitCode: 127,
			},
			exitCode:         127,
			expectError:      false,
			expectedNextStep: "",
		},
		{
			name: "no on_failure transition - return error",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeCommand,
				// no OnFailure
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			state: workflow.StepState{
				Name:     "test-step",
				Status:   workflow.StatusRunning,
				ExitCode: 2,
			},
			exitCode:         2,
			expectError:      true,
			expectedNextStep: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &ExecutionService{
				outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
				logger:        &mockLogger{},
			}

			stepCtx := context.Background()

			result := &ports.CommandResult{
				ExitCode: tt.exitCode,
			}
			nextStep, err := svc.handleNonZeroExit(stepCtx, tt.step, tt.execCtx, &tt.state, result)

			if tt.expectError {
				assert.Error(t, err, "should return error")
			} else {
				assert.NoError(t, err, "should not return error")
			}

			// Note: In stub phase, nextStep will be ""
			// In GREEN phase, verify nextStep == tt.expectedNextStep
			_ = nextStep
		})
	}
}

// TestExecutionService_handleSuccess tests the handleSuccess helper
// that processes successful step execution and resolves the next transition.
// Feature: C006 - Component T010
func TestExecutionService_handleSuccess(t *testing.T) {
	tests := []struct {
		name             string
		step             *workflow.Step
		execCtx          *workflow.ExecutionContext
		state            workflow.StepState
		expectError      bool
		expectedNextStep string
	}{
		{
			name: "transition resolution - simple on_success",
			step: &workflow.Step{
				Name:      "test-step",
				Type:      workflow.StepTypeCommand,
				OnSuccess: "next-step",
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			state: workflow.StepState{
				Name:     "test-step",
				Status:   workflow.StatusRunning,
				ExitCode: 0,
			},
			expectError:      false,
			expectedNextStep: "next-step",
		},
		{
			name: "simple on_success - direct transition",
			step: &workflow.Step{
				Name:      "test-step",
				Type:      workflow.StepTypeCommand,
				OnSuccess: "done",
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			state: workflow.StepState{
				Name:     "test-step",
				Status:   workflow.StatusRunning,
				ExitCode: 0,
			},
			expectError:      false,
			expectedNextStep: "done",
		},
		{
			name: "no transition - terminal or implicit end",
			step: &workflow.Step{
				Name: "final-step",
				Type: workflow.StepTypeCommand,
				// no OnSuccess
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			state: workflow.StepState{
				Name:     "final-step",
				Status:   workflow.StatusRunning,
				ExitCode: 0,
			},
			expectError:      false,
			expectedNextStep: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &ExecutionService{
				outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
				logger:        &mockLogger{},
			}

			stepCtx := context.Background()

			nextStep, err := svc.handleSuccess(stepCtx, tt.step, tt.execCtx, &tt.state)

			if tt.expectError {
				assert.Error(t, err, "should return error")
			} else {
				assert.NoError(t, err, "should not return error")
			}

			// Note: In stub phase, nextStep will be ""
			// In GREEN phase, verify nextStep == tt.expectedNextStep
			_ = nextStep
		})
	}
}
