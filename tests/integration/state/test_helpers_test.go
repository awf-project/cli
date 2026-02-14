//go:build integration

package state_test

import (
	"context"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// historyMockStateStore for history integration tests
type historyMockStateStore struct {
	states map[string]*workflow.ExecutionContext
}

func newHistoryMockStateStore() *historyMockStateStore {
	return &historyMockStateStore{states: make(map[string]*workflow.ExecutionContext)}
}

func (m *historyMockStateStore) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	m.states[state.WorkflowID] = state
	return nil
}

func (m *historyMockStateStore) Load(ctx context.Context, id string) (*workflow.ExecutionContext, error) {
	if state, ok := m.states[id]; ok {
		return state, nil
	}
	return nil, nil
}

func (m *historyMockStateStore) Delete(ctx context.Context, id string) error {
	delete(m.states, id)
	return nil
}

func (m *historyMockStateStore) List(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(m.states))
	for id := range m.states {
		ids = append(ids, id)
	}
	return ids, nil
}
