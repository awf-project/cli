package ports_test

import (
	"context"
	"testing"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

type mockRepository struct {
	workflows map[string]*workflow.Workflow
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		workflows: make(map[string]*workflow.Workflow),
	}
}

func (m *mockRepository) Load(ctx context.Context, name string) (*workflow.Workflow, error) {
	if wf, ok := m.workflows[name]; ok {
		return wf, nil
	}
	return nil, nil
}

func (m *mockRepository) List(ctx context.Context) ([]string, error) {
	names := make([]string, 0, len(m.workflows))
	for name := range m.workflows {
		names = append(names, name)
	}
	return names, nil
}

func (m *mockRepository) Exists(ctx context.Context, name string) (bool, error) {
	_, ok := m.workflows[name]
	return ok, nil
}

func TestWorkflowRepositoryInterface(t *testing.T) {
	var _ ports.WorkflowRepository = (*mockRepository)(nil)
}

func TestMockRepositoryLoad(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{Name: "test"}

	wf, err := repo.Load(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if wf == nil {
		t.Fatal("expected workflow, got nil")
	}
	if wf.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", wf.Name)
	}
}

func TestMockRepositoryList(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["wf1"] = &workflow.Workflow{Name: "wf1"}
	repo.workflows["wf2"] = &workflow.Workflow{Name: "wf2"}

	names, err := repo.List(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 workflows, got %d", len(names))
	}
}

func TestMockRepositoryExists(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["exists"] = &workflow.Workflow{Name: "exists"}

	exists, err := repo.Exists(context.Background(), "exists")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected workflow to exist")
	}

	exists, err = repo.Exists(context.Background(), "nonexistent")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected workflow to not exist")
	}
}
