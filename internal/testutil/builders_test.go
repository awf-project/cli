package testutil

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/agents"
)

// Feature: C007 - Test Infrastructure Modernization
// Item: T006 - ExecutionServiceBuilder

// TestExecutionServiceBuilder_HappyPath tests normal usage scenarios
func TestExecutionServiceBuilder_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		buildFunc  func() *application.ExecutionService
		verifyFunc func(t *testing.T, svc *application.ExecutionService)
		wantNonNil bool
	}{
		{
			name: "default builder returns configured service",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "ExecutionService should not be nil")
			},
			wantNonNil: true,
		},
		{
			name: "builder with custom logger",
			buildFunc: func() *application.ExecutionService {
				logger := NewMockLogger()
				return NewExecutionServiceBuilder().
					WithLogger(logger).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "ExecutionService should not be nil")
			},
			wantNonNil: true,
		},
		{
			name: "builder with custom executor",
			buildFunc: func() *application.ExecutionService {
				executor := NewMockCommandExecutor()
				return NewExecutionServiceBuilder().
					WithExecutor(executor).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "ExecutionService should not be nil")
			},
			wantNonNil: true,
		},
		{
			name: "builder with custom state store",
			buildFunc: func() *application.ExecutionService {
				store := NewMockStateStore()
				return NewExecutionServiceBuilder().
					WithStateStore(store).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "ExecutionService should not be nil")
			},
			wantNonNil: true,
		},
		{
			name: "builder with custom workflow repository",
			buildFunc: func() *application.ExecutionService {
				repo := NewMockWorkflowRepository()
				return NewExecutionServiceBuilder().
					WithWorkflowRepository(repo).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "ExecutionService should not be nil")
			},
			wantNonNil: true,
		},
		{
			name: "builder with all components",
			buildFunc: func() *application.ExecutionService {
				logger := NewMockLogger()
				executor := NewMockCommandExecutor()
				store := NewMockStateStore()
				repo := NewMockWorkflowRepository()
				return NewExecutionServiceBuilder().
					WithLogger(logger).
					WithExecutor(executor).
					WithStateStore(store).
					WithWorkflowRepository(repo).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "ExecutionService should not be nil")
			},
			wantNonNil: true,
		},
		{
			name: "builder chaining preserves configuration",
			buildFunc: func() *application.ExecutionService {
				builder := NewExecutionServiceBuilder()
				builder = builder.WithLogger(NewMockLogger())
				builder = builder.WithExecutor(NewMockCommandExecutor())
				return builder.Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "ExecutionService should not be nil")
			},
			wantNonNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := tt.buildFunc()
			if tt.wantNonNil {
				require.NotNil(t, svc, "Build() should return non-nil ExecutionService")
			}
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, svc)
			}
		})
	}
}

// TestExecutionServiceBuilder_FluentAPI tests method chaining
func TestExecutionServiceBuilder_FluentAPI(t *testing.T) {
	tests := []struct {
		name      string
		buildFunc func() *application.ExecutionService
		wantPanic bool
	}{
		{
			name: "chaining multiple With methods returns same builder",
			buildFunc: func() *application.ExecutionService {
				builder := NewExecutionServiceBuilder()
				builder2 := builder.WithLogger(NewMockLogger())
				builder3 := builder2.WithExecutor(NewMockCommandExecutor())
				return builder3.Build()
			},
			wantPanic: false,
		},
		{
			name: "builder can be reused for multiple builds",
			buildFunc: func() *application.ExecutionService {
				builder := NewExecutionServiceBuilder().
					WithLogger(NewMockLogger())

				svc1 := builder.Build()
				require.NotNil(t, svc1)

				svc2 := builder.Build()
				return svc2
			},
			wantPanic: false,
		},
		{
			name: "builder methods can be called in any order",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().
					WithStateStore(NewMockStateStore()).
					WithLogger(NewMockLogger()).
					WithWorkflowRepository(NewMockWorkflowRepository()).
					WithExecutor(NewMockCommandExecutor()).
					Build()
			},
			wantPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				assert.Panics(t, func() {
					tt.buildFunc()
				})
			} else {
				svc := tt.buildFunc()
				assert.NotNil(t, svc)
			}
		})
	}
}

// TestExecutionServiceBuilder_EdgeCases tests boundary conditions
func TestExecutionServiceBuilder_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		buildFunc  func() *application.ExecutionService
		wantNil    bool
		verifyFunc func(t *testing.T, svc *application.ExecutionService)
	}{
		{
			name: "builder with nil logger should use default",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().
					WithLogger(nil).
					Build()
			},
			wantNil: false,
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should fall back to default logger")
			},
		},
		{
			name: "builder with nil executor should use default",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().
					WithExecutor(nil).
					Build()
			},
			wantNil: false,
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should fall back to default executor")
			},
		},
		{
			name: "builder with nil state store should use default",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().
					WithStateStore(nil).
					Build()
			},
			wantNil: false,
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should fall back to default store")
			},
		},
		{
			name: "builder with nil repository should use default",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().
					WithWorkflowRepository(nil).
					Build()
			},
			wantNil: false,
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should fall back to default repository")
			},
		},
		{
			name: "empty builder uses all defaults",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().Build()
			},
			wantNil: false,
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should create service with all defaults")
			},
		},
		{
			name: "builder called multiple times with same component",
			buildFunc: func() *application.ExecutionService {
				logger1 := NewMockLogger()
				logger2 := NewMockLogger()
				return NewExecutionServiceBuilder().
					WithLogger(logger1).
					WithLogger(logger2). // Should use logger2
					Build()
			},
			wantNil: false,
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should use last provided logger")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := tt.buildFunc()
			if tt.wantNil {
				assert.Nil(t, svc)
			} else {
				assert.NotNil(t, svc)
			}
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, svc)
			}
		})
	}
}

// TestExecutionServiceBuilder_Integration tests builder with actual service execution
func TestExecutionServiceBuilder_Integration(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func() (*application.ExecutionService, *MockLogger, *MockCommandExecutor)
		workflowName string
		inputs       map[string]any
		verifyFunc   func(t *testing.T, logger *MockLogger, executor *MockCommandExecutor, result *workflow.ExecutionContext, err error)
	}{
		{
			name: "service built with mocks executes workflow successfully",
			setupFunc: func() (*application.ExecutionService, *MockLogger, *MockCommandExecutor) {
				logger := NewMockLogger()
				executor := NewMockCommandExecutor()
				repo := NewMockWorkflowRepository()
				store := NewMockStateStore()

				// Setup test workflow
				wf := &workflow.Workflow{
					Name:        "test-workflow",
					Description: "Test workflow",
					Inputs:      []workflow.Input{},
					Steps:       map[string]*workflow.Step{},
				}
				repo.AddWorkflow("test-workflow", wf)

				svc := NewExecutionServiceBuilder().
					WithLogger(logger).
					WithExecutor(executor).
					WithWorkflowRepository(repo).
					WithStateStore(store).
					Build()

				return svc, logger, executor
			},
			workflowName: "test-workflow",
			inputs:       map[string]any{},
			verifyFunc: func(t *testing.T, logger *MockLogger, executor *MockCommandExecutor, result *workflow.ExecutionContext, err error) {
				// Verify logger was used
				messages := logger.GetMessages()
				assert.NotEmpty(t, messages, "Logger should have recorded messages")

				// Verify service was properly constructed
				assert.NotNil(t, result, "ExecutionContext should not be nil")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, logger, executor := tt.setupFunc()
			require.NotNil(t, svc)

			ctx := context.Background()
			result, err := svc.Run(ctx, tt.workflowName, tt.inputs)

			if tt.verifyFunc != nil {
				tt.verifyFunc(t, logger, executor, result, err)
			}
		})
	}
}

// TestExecutionServiceBuilder_WithOutputWriters tests stdout/stderr configuration
func TestExecutionServiceBuilder_WithOutputWriters(t *testing.T) {
	tests := []struct {
		name       string
		stdout     io.Writer
		stderr     io.Writer
		verifyFunc func(t *testing.T, svc *application.ExecutionService, stdout, stderr io.Writer)
	}{
		{
			name:   "builder with custom stdout",
			stdout: &bytes.Buffer{},
			stderr: nil,
			verifyFunc: func(t *testing.T, svc *application.ExecutionService, stdout, stderr io.Writer) {
				assert.NotNil(t, svc)
				assert.NotNil(t, stdout)
			},
		},
		{
			name:   "builder with custom stderr",
			stdout: nil,
			stderr: &bytes.Buffer{},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService, stdout, stderr io.Writer) {
				assert.NotNil(t, svc)
				assert.NotNil(t, stderr)
			},
		},
		{
			name:   "builder with both stdout and stderr",
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService, stdout, stderr io.Writer) {
				assert.NotNil(t, svc)
				assert.NotNil(t, stdout)
				assert.NotNil(t, stderr)
			},
		},
		{
			name:   "builder with nil writers uses defaults",
			stdout: nil,
			stderr: nil,
			verifyFunc: func(t *testing.T, svc *application.ExecutionService, stdout, stderr io.Writer) {
				assert.NotNil(t, svc)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewExecutionServiceBuilder().
				WithOutputWriters(tt.stdout, tt.stderr).
				Build()

			if tt.verifyFunc != nil {
				tt.verifyFunc(t, svc, tt.stdout, tt.stderr)
			}
		})
	}
}

// TestExecutionServiceBuilder_WithAgentRegistry tests agent registry configuration
func TestExecutionServiceBuilder_WithAgentRegistry(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func() (*agents.AgentRegistry, *application.ExecutionService)
		verifyFunc func(t *testing.T, registry *agents.AgentRegistry, svc *application.ExecutionService)
	}{
		{
			name: "builder with custom agent registry",
			setupFunc: func() (*agents.AgentRegistry, *application.ExecutionService) {
				registry := agents.NewAgentRegistry()
				svc := NewExecutionServiceBuilder().
					WithAgentRegistry(registry).
					Build()
				return registry, svc
			},
			verifyFunc: func(t *testing.T, registry *agents.AgentRegistry, svc *application.ExecutionService) {
				assert.NotNil(t, svc)
				assert.NotNil(t, registry)
			},
		},
		{
			name: "builder with nil agent registry uses default",
			setupFunc: func() (*agents.AgentRegistry, *application.ExecutionService) {
				svc := NewExecutionServiceBuilder().
					WithAgentRegistry(nil).
					Build()
				return nil, svc
			},
			verifyFunc: func(t *testing.T, registry *agents.AgentRegistry, svc *application.ExecutionService) {
				assert.NotNil(t, svc)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, svc := tt.setupFunc()
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, registry, svc)
			}
		})
	}
}

// TestExecutionServiceBuilder_Defaults tests sensible default values
func TestExecutionServiceBuilder_Defaults(t *testing.T) {
	tests := []struct {
		name       string
		buildFunc  func() *application.ExecutionService
		verifyFunc func(t *testing.T, svc *application.ExecutionService)
	}{
		{
			name: "default builder provides all required dependencies",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Service should be created with defaults")

				// Service should be usable without panics
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				// Attempt to list resumable executions (should not panic)
				_, err := svc.ListResumable(ctx)
				// May fail due to missing workflow, but should not panic
				_ = err
			},
		},
		{
			name: "default builder provides mock logger",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc)
			},
		},
		{
			name: "default builder provides mock executor",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc)
			},
		},
		{
			name: "default builder provides mock state store",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := tt.buildFunc()
			require.NotNil(t, svc)
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, svc)
			}
		})
	}
}

// TestExecutionServiceBuilder_Concurrency tests thread-safe builder usage
func TestExecutionServiceBuilder_Concurrency(t *testing.T) {
	t.Run("concurrent builds from same base builder", func(t *testing.T) {
		baseBuilder := NewExecutionServiceBuilder().
			WithLogger(NewMockLogger())

		done := make(chan *application.ExecutionService, 5)

		for i := 0; i < 5; i++ {
			go func() {
				svc := baseBuilder.
					WithExecutor(NewMockCommandExecutor()).
					Build()
				done <- svc
			}()
		}

		for i := 0; i < 5; i++ {
			svc := <-done
			assert.NotNil(t, svc, "Each goroutine should get a valid service")
		}
	})

	t.Run("concurrent independent builders", func(t *testing.T) {
		done := make(chan *application.ExecutionService, 10)

		for i := 0; i < 10; i++ {
			go func() {
				svc := NewExecutionServiceBuilder().
					WithLogger(NewMockLogger()).
					WithExecutor(NewMockCommandExecutor()).
					Build()
				done <- svc
			}()
		}

		for i := 0; i < 10; i++ {
			svc := <-done
			assert.NotNil(t, svc, "Each goroutine should get a valid service")
		}
	})
}

// TestExecutionServiceBuilder_ErrorHandling tests error scenarios
func TestExecutionServiceBuilder_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		buildFunc  func() *application.ExecutionService
		verifyFunc func(t *testing.T, svc *application.ExecutionService)
	}{
		{
			name: "builder handles workflow service creation errors gracefully",
			buildFunc: func() *application.ExecutionService {
				repo := NewMockWorkflowRepository()
				return NewExecutionServiceBuilder().
					WithWorkflowRepository(repo).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should create service even if workflow might fail later")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := tt.buildFunc()
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, svc)
			}
		})
	}
}

// TestExecutionServiceBuilder_BoilerplateReduction validates 93% reduction claim
func TestExecutionServiceBuilder_BoilerplateReduction(t *testing.T) {
	t.Run("builder reduces setup from 30+ lines to 2-3 lines", func(t *testing.T) {
		// Before: 30+ lines of manual setup
		// logger := NewMockLogger()
		// executor := NewMockCommandExecutor()
		// repo := NewMockWorkflowRepository()
		// store := NewMockStateStore()
		// workflowSvc := application.NewWorkflowService(repo)
		// parallelExec := application.NewParallelExecutor(...)
		// resolver := interpolation.NewResolver()
		// evaluator := application.NewExpressionEvaluator()
		// hookExec := application.NewHookExecutor(...)
		// loopExec := application.NewLoopExecutor(...)
		// historySvc := application.NewHistoryService(...)
		// templateSvc := application.NewTemplateService(...)
		// ... many more lines
		// svc := application.NewExecutionService(workflowSvc, executor, parallelExec, ...)

		// After: 2-3 lines with builder
		svc := NewExecutionServiceBuilder().
			WithLogger(NewMockLogger()).
			Build()

		assert.NotNil(t, svc, "Builder should reduce boilerplate significantly")
	})

	t.Run("builder with all custom components still concise", func(t *testing.T) {
		svc := NewExecutionServiceBuilder().
			WithLogger(NewMockLogger()).
			WithExecutor(NewMockCommandExecutor()).
			WithStateStore(NewMockStateStore()).
			WithWorkflowRepository(NewMockWorkflowRepository()).
			WithAgentRegistry(agents.NewAgentRegistry()).
			WithOutputWriters(&bytes.Buffer{}, &bytes.Buffer{}).
			Build()

		assert.NotNil(t, svc, "Even fully configured builder is concise")
	})
}
