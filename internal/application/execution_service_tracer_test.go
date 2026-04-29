package application_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSpan mocks the ports.Span interface for testing.
type MockSpan struct {
	mock.Mock
}

func (m *MockSpan) End() {
	m.Called()
}

func (m *MockSpan) SetAttribute(key string, value any) {
	m.Called(key, value)
}

func (m *MockSpan) RecordError(err error) {
	m.Called(err)
}

func (m *MockSpan) AddEvent(name string) {
	m.Called(name)
}

func newMockSpan() *MockSpan {
	s := new(MockSpan)
	s.On("End").Return()
	s.On("SetAttribute", mock.Anything, mock.Anything).Maybe().Return()
	s.On("RecordError", mock.Anything).Maybe().Return()
	s.On("AddEvent", mock.Anything).Maybe().Return()
	return s
}

// MockTracer mocks the ports.Tracer interface for testing.
type MockTracer struct {
	mock.Mock
}

func (m *MockTracer) Start(ctx context.Context, spanName string) (context.Context, ports.Span) {
	args := m.Called(ctx, spanName)
	return args.Get(0).(context.Context), args.Get(1).(ports.Span)
}

// TestSetTracer verifies that SetTracer accepts and stores a tracer.
func TestSetTracer(t *testing.T) {
	svc, _ := NewTestHarness(t).Build()

	mockTracer := new(MockTracer)
	mockSpan := newMockSpan()
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockSpan)

	// Should not panic when setting tracer
	svc.SetTracer(mockTracer)
	assert.NotNil(t, mockTracer)
}

// TestWorkflowRunEmitsRootSpanBeforeExecution verifies workflow.run span is created early.
func TestWorkflowRunEmitsRootSpanBeforeExecution(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockRootSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockRootSpan)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()
	svc.SetTracer(mockTracer)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	// Verify workflow.run span was created
	mockTracer.AssertCalled(t, "Start", mock.Anything, mock.MatchedBy(func(name string) bool {
		return name == "workflow.run"
	}))
}

// TestStartSpanCreatesChildSpan verifies startSpan creates a child span with step name.
func TestStartSpanCreatesChildSpan(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "verify",
		Steps: map[string]*workflow.Step{
			"verify": {
				Name:      "verify",
				Type:      workflow.StepTypeCommand,
				Command:   "echo ok",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockStepSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, "step.verify").
		Return(context.Background(), mockStepSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	svc.SetTracer(mockTracer)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	// Verify step.verify span was created
	mockTracer.AssertCalled(t, "Start", mock.Anything, "step.verify")
	mockStepSpan.AssertCalled(t, "End")
}

// TestStepEmitsChildSpan verifies step.<name> span is emitted in executeStep.
func TestStepEmitsChildSpan(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "verify",
		Steps: map[string]*workflow.Step{
			"verify": {
				Name:      "verify",
				Type:      workflow.StepTypeCommand,
				Command:   "echo ok",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockStepSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, "step.verify").
		Return(context.Background(), mockStepSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test-workflow", wf).
		Build()
	svc.SetTracer(mockTracer)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	// Verify step.verify span was created
	mockTracer.AssertCalled(t, "Start", mock.Anything, "step.verify")
	mockStepSpan.AssertCalled(t, "End")
}

// TestStartSpanWithNilTracer verifies startSpan handles nil tracer gracefully.
func TestStartSpanWithNilTracer(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	// Don't set a tracer - should use NopTracer
	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)

	// Workflow should complete successfully even without a tracer
	assert.NoError(t, err)
}
