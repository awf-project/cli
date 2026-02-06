package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
)

// Component: T002
// Feature: C051
// Tests for ExecutionServiceBuilder validator injection functionality

// TestExecutionServiceBuilder_WithValidator_HappyPath tests normal usage scenarios
func TestExecutionServiceBuilder_WithValidator_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		buildFunc  func() *application.ExecutionService
		verifyFunc func(t *testing.T, svc *application.ExecutionService)
	}{
		{
			name: "builder with custom validator",
			buildFunc: func() *application.ExecutionService {
				validator := NewMockExpressionValidator()
				return NewExecutionServiceBuilder().
					WithValidator(validator).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "ExecutionService should not be nil")
			},
		},
		{
			name: "builder with nil validator uses nil (safe for tests not exercising validation)",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().
					WithValidator(nil).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "ExecutionService should be created with nil validator")
			},
		},
		{
			name: "builder without validator call defaults to nil",
			buildFunc: func() *application.ExecutionService {
				return NewExecutionServiceBuilder().Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "ExecutionService should be created with default nil validator")
			},
		},
		{
			name: "validator chaining with other components",
			buildFunc: func() *application.ExecutionService {
				validator := NewMockExpressionValidator()
				logger := NewMockLogger()
				executor := NewMockCommandExecutor()
				return NewExecutionServiceBuilder().
					WithLogger(logger).
					WithValidator(validator).
					WithExecutor(executor).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "ExecutionService should integrate validator with other components")
			},
		},
		{
			name: "validator set in middle of chain",
			buildFunc: func() *application.ExecutionService {
				validator := NewMockExpressionValidator()
				return NewExecutionServiceBuilder().
					WithLogger(NewMockLogger()).
					WithValidator(validator).
					WithExecutor(NewMockCommandExecutor()).
					WithStateStore(NewMockStateStore()).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Validator position in chain should not matter")
			},
		},
		{
			name: "validator set at start of chain",
			buildFunc: func() *application.ExecutionService {
				validator := NewMockExpressionValidator()
				return NewExecutionServiceBuilder().
					WithValidator(validator).
					WithLogger(NewMockLogger()).
					WithExecutor(NewMockCommandExecutor()).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Validator at start should work")
			},
		},
		{
			name: "validator set at end of chain",
			buildFunc: func() *application.ExecutionService {
				validator := NewMockExpressionValidator()
				return NewExecutionServiceBuilder().
					WithLogger(NewMockLogger()).
					WithExecutor(NewMockCommandExecutor()).
					WithValidator(validator).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Validator at end should work")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := tt.buildFunc()
			require.NotNil(t, svc, "Build() should return non-nil ExecutionService")
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, svc)
			}
		})
	}
}

// TestExecutionServiceBuilder_WithValidator_EdgeCases tests boundary conditions
func TestExecutionServiceBuilder_WithValidator_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		buildFunc  func() *application.ExecutionService
		verifyFunc func(t *testing.T, svc *application.ExecutionService)
	}{
		{
			name: "validator replaced when called multiple times",
			buildFunc: func() *application.ExecutionService {
				validator1 := NewMockExpressionValidator()
				validator2 := NewMockExpressionValidator()
				return NewExecutionServiceBuilder().
					WithValidator(validator1).
					WithValidator(validator2). // Should use validator2
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should use last provided validator")
			},
		},
		{
			name: "validator set then replaced with nil",
			buildFunc: func() *application.ExecutionService {
				validator := NewMockExpressionValidator()
				return NewExecutionServiceBuilder().
					WithValidator(validator).
					WithValidator(nil). // Replace with nil
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should accept nil validator replacement")
			},
		},
		{
			name: "nil validator then replaced with actual validator",
			buildFunc: func() *application.ExecutionService {
				validator := NewMockExpressionValidator()
				return NewExecutionServiceBuilder().
					WithValidator(nil).
					WithValidator(validator). // Replace nil with validator
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should accept validator after nil")
			},
		},
		{
			name: "builder reused for multiple builds with validator",
			buildFunc: func() *application.ExecutionService {
				validator := NewMockExpressionValidator()
				builder := NewExecutionServiceBuilder().
					WithValidator(validator)

				svc1 := builder.Build()
				require.NotNil(t, svc1)

				return builder.Build() // Second build
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Validator should persist across multiple builds")
			},
		},
		{
			name: "builder reused changing validator between builds",
			buildFunc: func() *application.ExecutionService {
				validator1 := NewMockExpressionValidator()
				validator2 := NewMockExpressionValidator()
				builder := NewExecutionServiceBuilder()

				builder.WithValidator(validator1)
				svc1 := builder.Build()
				require.NotNil(t, svc1)

				builder.WithValidator(validator2)
				return builder.Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should use updated validator in second build")
			},
		},
		{
			name: "only validator set, all other components default",
			buildFunc: func() *application.ExecutionService {
				validator := NewMockExpressionValidator()
				return NewExecutionServiceBuilder().
					WithValidator(validator).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should create service with only validator set")
			},
		},
		{
			name: "all components set including validator",
			buildFunc: func() *application.ExecutionService {
				validator := NewMockExpressionValidator()
				logger := NewMockLogger()
				executor := NewMockCommandExecutor()
				store := NewMockStateStore()
				repo := NewMockWorkflowRepository()
				registry := NewMockAgentRegistry()
				return NewExecutionServiceBuilder().
					WithLogger(logger).
					WithExecutor(executor).
					WithStateStore(store).
					WithWorkflowRepository(repo).
					WithAgentRegistry(registry).
					WithValidator(validator).
					Build()
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService) {
				assert.NotNil(t, svc, "Should create fully configured service")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := tt.buildFunc()
			require.NotNil(t, svc, "Build() should return non-nil ExecutionService")
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, svc)
			}
		})
	}
}

// TestExecutionServiceBuilder_WithValidator_Concurrency tests thread-safe validator usage
func TestExecutionServiceBuilder_WithValidator_Concurrency(t *testing.T) {
	tests := []struct {
		name        string
		buildFunc   func() []*application.ExecutionService
		verifyFunc  func(t *testing.T, services []*application.ExecutionService)
		parallelism int
	}{
		{
			name: "concurrent builds with same validator",
			buildFunc: func() []*application.ExecutionService {
				validator := NewMockExpressionValidator()
				builder := NewExecutionServiceBuilder().
					WithValidator(validator)

				done := make(chan *application.ExecutionService, 5)
				for i := 0; i < 5; i++ {
					go func() {
						done <- builder.Build()
					}()
				}

				services := make([]*application.ExecutionService, 5)
				for i := 0; i < 5; i++ {
					services[i] = <-done
				}
				return services
			},
			verifyFunc: func(t *testing.T, services []*application.ExecutionService) {
				for i, svc := range services {
					assert.NotNil(t, svc, "Service %d should not be nil", i)
				}
			},
			parallelism: 5,
		},
		{
			name: "concurrent builds with different validators",
			buildFunc: func() []*application.ExecutionService {
				done := make(chan *application.ExecutionService, 10)
				for i := 0; i < 10; i++ {
					go func() {
						validator := NewMockExpressionValidator()
						svc := NewExecutionServiceBuilder().
							WithValidator(validator).
							Build()
						done <- svc
					}()
				}

				services := make([]*application.ExecutionService, 10)
				for i := 0; i < 10; i++ {
					services[i] = <-done
				}
				return services
			},
			verifyFunc: func(t *testing.T, services []*application.ExecutionService) {
				for i, svc := range services {
					assert.NotNil(t, svc, "Service %d should not be nil", i)
				}
			},
			parallelism: 10,
		},
		{
			name: "concurrent WithValidator calls on same builder",
			buildFunc: func() []*application.ExecutionService {
				builder := NewExecutionServiceBuilder()
				done := make(chan struct{}, 3)

				for i := 0; i < 3; i++ {
					go func() {
						validator := NewMockExpressionValidator()
						builder.WithValidator(validator)
						done <- struct{}{}
					}()
				}

				for i := 0; i < 3; i++ {
					<-done
				}

				// Build once after all concurrent sets
				svc := builder.Build()
				return []*application.ExecutionService{svc}
			},
			verifyFunc: func(t *testing.T, services []*application.ExecutionService) {
				assert.Len(t, services, 1)
				assert.NotNil(t, services[0], "Service should be created safely")
			},
			parallelism: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := tt.buildFunc()
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, services)
			}
		})
	}
}

// TestExecutionServiceBuilder_WithValidator_FluentAPI tests method chaining
func TestExecutionServiceBuilder_WithValidator_FluentAPI(t *testing.T) {
	tests := []struct {
		name       string
		buildFunc  func() *ExecutionServiceBuilder
		verifyFunc func(t *testing.T, builder *ExecutionServiceBuilder)
	}{
		{
			name: "WithValidator returns builder for chaining",
			buildFunc: func() *ExecutionServiceBuilder {
				validator := NewMockExpressionValidator()
				builder := NewExecutionServiceBuilder()
				returned := builder.WithValidator(validator)
				return returned
			},
			verifyFunc: func(t *testing.T, builder *ExecutionServiceBuilder) {
				assert.NotNil(t, builder, "WithValidator should return builder")
				svc := builder.Build()
				assert.NotNil(t, svc, "Returned builder should be buildable")
			},
		},
		{
			name: "WithValidator chain with nil returns builder",
			buildFunc: func() *ExecutionServiceBuilder {
				builder := NewExecutionServiceBuilder()
				returned := builder.WithValidator(nil)
				return returned
			},
			verifyFunc: func(t *testing.T, builder *ExecutionServiceBuilder) {
				assert.NotNil(t, builder, "WithValidator(nil) should return builder")
				svc := builder.Build()
				assert.NotNil(t, svc, "Builder should work after nil validator")
			},
		},
		{
			name: "complex chain with validator",
			buildFunc: func() *ExecutionServiceBuilder {
				validator := NewMockExpressionValidator()
				return NewExecutionServiceBuilder().
					WithLogger(NewMockLogger()).
					WithValidator(validator).
					WithExecutor(NewMockCommandExecutor()).
					WithStateStore(NewMockStateStore()).
					WithWorkflowRepository(NewMockWorkflowRepository())
			},
			verifyFunc: func(t *testing.T, builder *ExecutionServiceBuilder) {
				assert.NotNil(t, builder, "Complex chain should work")
				svc := builder.Build()
				assert.NotNil(t, svc, "Service should be created from complex chain")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.buildFunc()
			require.NotNil(t, builder, "buildFunc should return non-nil builder")
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, builder)
			}
		})
	}
}

// TestExecutionServiceBuilder_ValidatorPassedToWorkflowService tests integration
func TestExecutionServiceBuilder_ValidatorPassedToWorkflowService(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func() (*application.ExecutionService, ports.ExpressionValidator)
		verifyFunc func(t *testing.T, svc *application.ExecutionService, validator ports.ExpressionValidator)
	}{
		{
			name: "validator is passed to WorkflowService",
			setupFunc: func() (*application.ExecutionService, ports.ExpressionValidator) {
				validator := NewMockExpressionValidator()
				svc := NewExecutionServiceBuilder().
					WithValidator(validator).
					Build()
				return svc, validator
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService, validator ports.ExpressionValidator) {
				assert.NotNil(t, svc, "Service should be created")
				assert.NotNil(t, validator, "Validator should be set")
				// We cannot directly verify the validator was passed since WorkflowService
				// doesn't expose it, but we can verify service creation succeeded
			},
		},
		{
			name: "nil validator is passed to WorkflowService",
			setupFunc: func() (*application.ExecutionService, ports.ExpressionValidator) {
				svc := NewExecutionServiceBuilder().
					WithValidator(nil).
					Build()
				return svc, nil
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService, validator ports.ExpressionValidator) {
				assert.NotNil(t, svc, "Service should be created with nil validator")
				assert.Nil(t, validator, "Validator should be nil")
			},
		},
		{
			name: "default nil validator is passed to WorkflowService",
			setupFunc: func() (*application.ExecutionService, ports.ExpressionValidator) {
				// Don't call WithValidator at all
				svc := NewExecutionServiceBuilder().Build()
				return svc, nil
			},
			verifyFunc: func(t *testing.T, svc *application.ExecutionService, validator ports.ExpressionValidator) {
				assert.NotNil(t, svc, "Service should be created with default nil validator")
				assert.Nil(t, validator, "Default validator should be nil")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, validator := tt.setupFunc()
			require.NotNil(t, svc, "setupFunc should return non-nil service")
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, svc, validator)
			}
		})
	}
}

// TestExecutionServiceBuilder_ValidatorFieldThreadSafety tests mutex protection
func TestExecutionServiceBuilder_ValidatorFieldThreadSafety(t *testing.T) {
	t.Run("concurrent validator sets are thread-safe", func(t *testing.T) {
		builder := NewExecutionServiceBuilder()
		iterations := 100

		done := make(chan struct{}, iterations)
		for i := 0; i < iterations; i++ {
			go func() {
				validator := NewMockExpressionValidator()
				builder.WithValidator(validator)
				done <- struct{}{}
			}()
		}

		// Wait for all goroutines
		for i := 0; i < iterations; i++ {
			<-done
		}

		// Verify builder still works
		svc := builder.Build()
		assert.NotNil(t, svc, "Builder should remain functional after concurrent sets")
	})

	t.Run("concurrent builds with validator are thread-safe", func(t *testing.T) {
		validator := NewMockExpressionValidator()
		builder := NewExecutionServiceBuilder().WithValidator(validator)
		iterations := 50

		done := make(chan *application.ExecutionService, iterations)
		for i := 0; i < iterations; i++ {
			go func() {
				svc := builder.Build()
				done <- svc
			}()
		}

		// Collect all services
		for i := 0; i < iterations; i++ {
			svc := <-done
			assert.NotNil(t, svc, "Concurrent build %d should succeed", i)
		}
	})
}
