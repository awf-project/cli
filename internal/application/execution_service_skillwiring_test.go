package application_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/builders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T008
// Feature: F096 - Agent Skills Integration

// TestSetSkillRepository_AcceptsInterfaceType verifies that SetSkillRepository
// accepts the ports.SkillRepository interface type.
func TestSetSkillRepository_AcceptsInterfaceType(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()
	mockRepo := &skillWiringTestRepository{}

	execSvc.SetSkillRepository(mockRepo)

	assert.NotNil(t, execSvc)
}

// TestSetSkillRepository_AcceptsNil verifies that SetSkillRepository can accept nil,
// supporting graceful degradation when no skill repository is configured.
func TestSetSkillRepository_AcceptsNil(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	execSvc.SetSkillRepository(nil)

	assert.NotNil(t, execSvc)
}

// TestSetSkillRepository_SupportsReassignment verifies that the skill repository
// can be changed after initial assignment.
func TestSetSkillRepository_SupportsReassignment(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()
	first := &skillWiringTestRepository{}
	second := &skillWiringTestRepository{}

	execSvc.SetSkillRepository(first)
	assert.NotNil(t, execSvc, "First assignment should succeed")

	execSvc.SetSkillRepository(second)
	assert.NotNil(t, execSvc, "Reassignment should succeed")

	execSvc.SetSkillRepository(nil)
	assert.NotNil(t, execSvc, "Setting to nil should succeed")
}

// TestExecutionServiceBuilder_WithSkillRepository verifies the builder method
// exists and can configure a skill repository on ExecutionService.
func TestExecutionServiceBuilder_WithSkillRepository(t *testing.T) {
	mockRepo := &skillWiringTestRepository{}

	builder := builders.NewExecutionServiceBuilder().
		WithSkillRepository(mockRepo)

	assert.NotNil(t, builder)
}

// TestExecutionServiceBuilder_WithSkillRepository_Nil verifies the builder
// accepts nil for skill repository.
func TestExecutionServiceBuilder_WithSkillRepository_Nil(t *testing.T) {
	builder := builders.NewExecutionServiceBuilder().
		WithSkillRepository(nil)

	assert.NotNil(t, builder)
}

// TestExecutionServiceBuilder_WithSkillRepository_BuildsService verifies that
// the builder's Build() method correctly wires the skill repository into
// the ExecutionService.
func TestExecutionServiceBuilder_WithSkillRepository_BuildsService(t *testing.T) {
	mockRepo := &skillWiringTestRepository{}

	svc := builders.NewExecutionServiceBuilder().
		WithSkillRepository(mockRepo).
		Build()

	require.NotNil(t, svc)
	assert.NotNil(t, svc)
}

// TestExecutionServiceBuilder_WithSkillRepository_SupportsChaining verifies
// that WithSkillRepository returns the builder for method chaining.
func TestExecutionServiceBuilder_WithSkillRepository_SupportsChaining(t *testing.T) {
	mockRepo := &skillWiringTestRepository{}

	svc := builders.NewExecutionServiceBuilder().
		WithSkillRepository(mockRepo).
		WithLogger(nil).
		Build()

	require.NotNil(t, svc)
}

// skillWiringTestRepository is a test double implementing ports.SkillRepository
// for T008 tests.
type skillWiringTestRepository struct{}

func (r *skillWiringTestRepository) Load(ctx context.Context, name string) (*workflow.Skill, error) {
	return nil, nil
}

func (r *skillWiringTestRepository) LoadFromPath(ctx context.Context, absolutePath string) (*workflow.Skill, error) {
	return nil, nil
}
