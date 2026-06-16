package application

import (
	"context"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/mock"
)

// fakeInputResponder is a test double for ACPInputResponder
type fakeInputResponder struct {
	mu             sync.Mutex
	recordedInputs []string
}

func (f *fakeInputResponder) ReadInput(ctx context.Context) (string, error) {
	return "", nil
}

func (f *fakeInputResponder) Respond(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordedInputs = append(f.recordedInputs, text)
}

func (f *fakeInputResponder) SetParkHooks(_, _ func()) {}

func (f *fakeInputResponder) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]string, len(f.recordedInputs))
	copy(result, f.recordedInputs)
	return result
}

// MockWorkflowRepository is a mock for ports.WorkflowRepository
type MockWorkflowRepository struct {
	mock.Mock
}

func (m *MockWorkflowRepository) ListWithSource(ctx context.Context) ([]ports.WorkflowInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	infos, _ := args.Get(0).([]ports.WorkflowInfo)
	return infos, args.Error(1)
}

func (m *MockWorkflowRepository) Load(ctx context.Context, name string) (*workflow.Workflow, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	wf, _ := args.Get(0).(*workflow.Workflow)
	return wf, args.Error(1)
}

func (m *MockWorkflowRepository) Exists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

func (m *MockWorkflowRepository) List(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	names, _ := args.Get(0).([]string)
	return names, args.Error(1)
}

// testWorkflow creates a minimal test workflow
func testWorkflow(name string) *workflow.Workflow {
	return &workflow.Workflow{
		Name:        name,
		Description: "test workflow",
		Inputs:      []workflow.Input{},
		Steps:       make(map[string]*workflow.Step),
	}
}

// fakeEmitter is a test double for SessionUpdateEmitter
type fakeEmitter struct {
	mu            sync.Mutex
	agentTextList []string
}

func (f *fakeEmitter) EmitSessionUpdate(ctx context.Context, sessionID, kind string, fields map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if kind == "agent_message_chunk" {
		if content, ok := fields["content"].(map[string]any); ok {
			if text, ok := content["text"].(string); ok {
				f.agentTextList = append(f.agentTextList, text)
			}
		}
	}
	return nil
}

// resultMap extracts the result map from a HandleSessionNew response
func resultMap(t *testing.T, result any) map[string]any {
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	return m
}
