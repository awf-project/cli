package ports_test

import (
	"context"
	"testing"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
)

type mockStateStore struct {
	states map[string]*workflow.ExecutionContext
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{
		states: make(map[string]*workflow.ExecutionContext),
	}
}

func (m *mockStateStore) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	m.states[state.WorkflowID] = state
	return nil
}

func (m *mockStateStore) Load(ctx context.Context, workflowID string) (*workflow.ExecutionContext, error) {
	if state, ok := m.states[workflowID]; ok {
		return state, nil
	}
	return nil, nil
}

func (m *mockStateStore) Delete(ctx context.Context, workflowID string) error {
	delete(m.states, workflowID)
	return nil
}

func (m *mockStateStore) List(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(m.states))
	for id := range m.states {
		ids = append(ids, id)
	}
	return ids, nil
}

func TestStateStoreInterface(t *testing.T) {
	var _ ports.StateStore = (*mockStateStore)(nil)
}

func TestMockStateStoreSaveAndLoad(t *testing.T) {
	store := newMockStateStore()
	ctx := context.Background()

	execCtx := workflow.NewExecutionContext("test-123", "test-workflow")
	execCtx.SetInput("key", "value")

	err := store.Save(ctx, execCtx)
	if err != nil {
		t.Errorf("unexpected error on Save: %v", err)
	}

	loaded, err := store.Load(ctx, "test-123")
	if err != nil {
		t.Errorf("unexpected error on Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected state, got nil")
	}
	if loaded.WorkflowID != "test-123" {
		t.Errorf("expected WorkflowID 'test-123', got '%s'", loaded.WorkflowID)
	}

	val, ok := loaded.GetInput("key")
	if !ok {
		t.Error("expected input 'key' to exist")
	}
	if val != "value" {
		t.Errorf("expected value 'value', got '%v'", val)
	}
}

func TestMockStateStoreDelete(t *testing.T) {
	store := newMockStateStore()
	ctx := context.Background()

	execCtx := workflow.NewExecutionContext("to-delete", "test")
	_ = store.Save(ctx, execCtx)

	err := store.Delete(ctx, "to-delete")
	if err != nil {
		t.Errorf("unexpected error on Delete: %v", err)
	}

	loaded, _ := store.Load(ctx, "to-delete")
	if loaded != nil {
		t.Error("expected state to be deleted")
	}
}

func TestMockStateStoreList(t *testing.T) {
	store := newMockStateStore()
	ctx := context.Background()

	_ = store.Save(ctx, workflow.NewExecutionContext("id1", "wf1"))
	_ = store.Save(ctx, workflow.NewExecutionContext("id2", "wf2"))

	ids, err := store.List(ctx)
	if err != nil {
		t.Errorf("unexpected error on List: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 states, got %d", len(ids))
	}
}
