package testutil_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/testutil"
)

// =============================================================================
// MockWorkflowRepository Tests
// =============================================================================

// Feature: C007 Test Infrastructure Modernization
// Component: T002 MockWorkflowRepository

func TestMockWorkflowRepository_NewMockWorkflowRepository(t *testing.T) {
	// Arrange & Act
	repo := testutil.NewMockWorkflowRepository()

	// Assert
	require.NotNil(t, repo, "NewMockWorkflowRepository should return non-nil instance")

	// Verify it's usable immediately
	ctx := context.Background()
	wf, err := repo.Load(ctx, "nonexistent")
	assert.NoError(t, err, "Load on empty repository should not error")
	assert.Nil(t, wf, "Load on empty repository should return nil for nonexistent workflow")

	names, err := repo.List(ctx)
	assert.NoError(t, err, "List on empty repository should not error")
	assert.Empty(t, names, "List on empty repository should return empty slice")

	exists, err := repo.Exists(ctx, "nonexistent")
	assert.NoError(t, err, "Exists on empty repository should not error")
	assert.False(t, exists, "Exists on empty repository should return false")
}

func TestMockWorkflowRepository_Load_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(*testutil.MockWorkflowRepository)
		workflowName string
		want         *workflow.Workflow
		wantErr      bool
	}{
		{
			name: "load existing workflow",
			setupFunc: func(repo *testutil.MockWorkflowRepository) {
				repo.AddWorkflow("test-wf", &workflow.Workflow{
					Name:        "test-wf",
					Description: "Test workflow",
					Initial:     "start",
				})
			},
			workflowName: "test-wf",
			want: &workflow.Workflow{
				Name:        "test-wf",
				Description: "Test workflow",
				Initial:     "start",
			},
			wantErr: false,
		},
		{
			name: "load nonexistent workflow returns nil",
			setupFunc: func(repo *testutil.MockWorkflowRepository) {
				repo.AddWorkflow("other-wf", &workflow.Workflow{Name: "other-wf"})
			},
			workflowName: "nonexistent",
			want:         nil,
			wantErr:      false,
		},
		{
			name: "load from empty repository returns nil",
			setupFunc: func(repo *testutil.MockWorkflowRepository) {
				// No workflows added
			},
			workflowName: "any",
			want:         nil,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := testutil.NewMockWorkflowRepository()
			tt.setupFunc(repo)
			ctx := context.Background()

			// Act
			got, err := repo.Load(ctx, tt.workflowName)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.want.Name, got.Name)
				assert.Equal(t, tt.want.Description, got.Description)
				assert.Equal(t, tt.want.Initial, got.Initial)
			}
		})
	}
}

func TestMockWorkflowRepository_Load_ErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*testutil.MockWorkflowRepository)
		wantErr   error
	}{
		{
			name: "load with configured error",
			setupFunc: func(repo *testutil.MockWorkflowRepository) {
				repo.AddWorkflow("test", &workflow.Workflow{Name: "test"})
				repo.SetLoadError(errors.New("load failed"))
			},
			wantErr: errors.New("load failed"),
		},
		{
			name: "load error overrides existing workflow",
			setupFunc: func(repo *testutil.MockWorkflowRepository) {
				repo.AddWorkflow("test", &workflow.Workflow{Name: "test"})
				repo.SetLoadError(errors.New("simulated error"))
			},
			wantErr: errors.New("simulated error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := testutil.NewMockWorkflowRepository()
			tt.setupFunc(repo)
			ctx := context.Background()

			// Act
			got, err := repo.Load(ctx, "test")

			// Assert
			assert.Error(t, err)
			assert.EqualError(t, err, tt.wantErr.Error())
			assert.Nil(t, got)
		})
	}
}

func TestMockWorkflowRepository_List_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*testutil.MockWorkflowRepository)
		want      []string
		wantErr   bool
	}{
		{
			name: "list empty repository",
			setupFunc: func(repo *testutil.MockWorkflowRepository) {
				// No workflows
			},
			want:    []string{},
			wantErr: false,
		},
		{
			name: "list single workflow",
			setupFunc: func(repo *testutil.MockWorkflowRepository) {
				repo.AddWorkflow("wf1", &workflow.Workflow{Name: "wf1"})
			},
			want:    []string{"wf1"},
			wantErr: false,
		},
		{
			name: "list multiple workflows",
			setupFunc: func(repo *testutil.MockWorkflowRepository) {
				repo.AddWorkflow("wf1", &workflow.Workflow{Name: "wf1"})
				repo.AddWorkflow("wf2", &workflow.Workflow{Name: "wf2"})
				repo.AddWorkflow("wf3", &workflow.Workflow{Name: "wf3"})
			},
			want:    []string{"wf1", "wf2", "wf3"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := testutil.NewMockWorkflowRepository()
			tt.setupFunc(repo)
			ctx := context.Background()

			// Act
			got, err := repo.List(ctx)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.ElementsMatch(t, tt.want, got, "List should return all workflow names")
		})
	}
}

func TestMockWorkflowRepository_List_ErrorHandling(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	repo.AddWorkflow("wf1", &workflow.Workflow{Name: "wf1"})
	repo.SetListError(errors.New("list failed"))
	ctx := context.Background()

	// Act
	got, err := repo.List(ctx)

	// Assert
	assert.Error(t, err)
	assert.EqualError(t, err, "list failed")
	assert.Nil(t, got)
}

func TestMockWorkflowRepository_Exists_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(*testutil.MockWorkflowRepository)
		workflowName string
		want         bool
		wantErr      bool
	}{
		{
			name: "exists returns true for existing workflow",
			setupFunc: func(repo *testutil.MockWorkflowRepository) {
				repo.AddWorkflow("test", &workflow.Workflow{Name: "test"})
			},
			workflowName: "test",
			want:         true,
			wantErr:      false,
		},
		{
			name: "exists returns false for nonexistent workflow",
			setupFunc: func(repo *testutil.MockWorkflowRepository) {
				repo.AddWorkflow("other", &workflow.Workflow{Name: "other"})
			},
			workflowName: "test",
			want:         false,
			wantErr:      false,
		},
		{
			name: "exists returns false for empty repository",
			setupFunc: func(repo *testutil.MockWorkflowRepository) {
				// No workflows
			},
			workflowName: "test",
			want:         false,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := testutil.NewMockWorkflowRepository()
			tt.setupFunc(repo)
			ctx := context.Background()

			// Act
			got, err := repo.Exists(ctx, tt.workflowName)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMockWorkflowRepository_Exists_ErrorHandling(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	repo.AddWorkflow("test", &workflow.Workflow{Name: "test"})
	repo.SetExistsError(errors.New("exists failed"))
	ctx := context.Background()

	// Act
	got, err := repo.Exists(ctx, "test")

	// Assert
	assert.Error(t, err)
	assert.EqualError(t, err, "exists failed")
	assert.False(t, got)
}

func TestMockWorkflowRepository_AddWorkflow(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	ctx := context.Background()
	wf := &workflow.Workflow{
		Name:        "test",
		Description: "Test workflow",
		Initial:     "start",
	}

	// Act
	repo.AddWorkflow("test", wf)

	// Assert - verify workflow is loadable
	loaded, err := repo.Load(ctx, "test")
	assert.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "test", loaded.Name)
	assert.Equal(t, "Test workflow", loaded.Description)

	// Assert - verify workflow appears in list
	names, err := repo.List(ctx)
	assert.NoError(t, err)
	assert.Contains(t, names, "test")

	// Assert - verify workflow exists
	exists, err := repo.Exists(ctx, "test")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestMockWorkflowRepository_AddWorkflow_OverwritesExisting(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	ctx := context.Background()
	wf1 := &workflow.Workflow{Name: "test", Description: "First"}
	wf2 := &workflow.Workflow{Name: "test", Description: "Second"}

	// Act
	repo.AddWorkflow("test", wf1)
	repo.AddWorkflow("test", wf2)

	// Assert
	loaded, err := repo.Load(ctx, "test")
	assert.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "Second", loaded.Description, "Second workflow should overwrite first")
}

func TestMockWorkflowRepository_Clear(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	ctx := context.Background()
	repo.AddWorkflow("wf1", &workflow.Workflow{Name: "wf1"})
	repo.AddWorkflow("wf2", &workflow.Workflow{Name: "wf2"})
	repo.SetLoadError(errors.New("test error"))

	// Verify workflows exist before clear
	names, _ := repo.List(ctx)
	assert.Len(t, names, 2)

	// Act
	repo.Clear()

	// Assert - workflows are cleared
	names, err := repo.List(ctx)
	assert.NoError(t, err)
	assert.Empty(t, names)

	// Assert - errors are cleared
	wf, err := repo.Load(ctx, "wf1")
	assert.NoError(t, err, "Load error should be cleared")
	assert.Nil(t, wf)

	exists, err := repo.Exists(ctx, "wf1")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestMockWorkflowRepository_ContextCancellation(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	repo.AddWorkflow("test", &workflow.Workflow{Name: "test"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act & Assert - operations should still complete (mock ignores context cancellation)
	wf, err := repo.Load(ctx, "test")
	assert.NoError(t, err, "Mock should ignore context cancellation")
	assert.NotNil(t, wf)

	names, err := repo.List(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, names)

	exists, err := repo.Exists(ctx, "test")
	assert.NoError(t, err)
	assert.True(t, exists)
}

// =============================================================================
// Thread Safety Tests
// =============================================================================

func TestMockWorkflowRepository_ConcurrentLoad(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	ctx := context.Background()

	// Add workflows
	for i := 0; i < 10; i++ {
		name := string(rune('a' + i))
		repo.AddWorkflow(name, &workflow.Workflow{Name: name})
	}

	// Act - concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			name := string(rune('a' + (iteration % 10)))
			wf, err := repo.Load(ctx, name)
			assert.NoError(t, err)
			assert.NotNil(t, wf)
			assert.Equal(t, name, wf.Name)
		}(i)
	}

	// Assert
	wg.Wait() // Should complete without race conditions
}

func TestMockWorkflowRepository_ConcurrentList(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	ctx := context.Background()
	repo.AddWorkflow("wf1", &workflow.Workflow{Name: "wf1"})
	repo.AddWorkflow("wf2", &workflow.Workflow{Name: "wf2"})

	// Act - concurrent list calls
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			names, err := repo.List(ctx)
			assert.NoError(t, err)
			assert.Len(t, names, 2)
		}()
	}

	// Assert
	wg.Wait()
}

func TestMockWorkflowRepository_ConcurrentExists(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	ctx := context.Background()
	repo.AddWorkflow("test", &workflow.Workflow{Name: "test"})

	// Act - concurrent exists calls
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			exists, err := repo.Exists(ctx, "test")
			assert.NoError(t, err)
			assert.True(t, exists)
		}()
	}

	// Assert
	wg.Wait()
}

func TestMockWorkflowRepository_ConcurrentAddAndRead(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	ctx := context.Background()

	// Act - concurrent writes and reads
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := string(rune('a' + id))
			repo.AddWorkflow(name, &workflow.Workflow{Name: name})
		}(i)
	}

	// Readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			name := string(rune('a' + (iteration % 10)))
			// May or may not find workflow depending on timing
			repo.Load(ctx, name)
		}(i)
	}

	// Assert
	wg.Wait() // Should complete without race conditions

	names, err := repo.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, names, 10, "All workflows should be added")
}

func TestMockWorkflowRepository_ConcurrentErrorConfiguration(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	ctx := context.Background()
	repo.AddWorkflow("test", &workflow.Workflow{Name: "test"})

	// Act - concurrent error configuration and reads
	var wg sync.WaitGroup

	// Error configurators
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repo.SetLoadError(errors.New("concurrent error"))
		}()
	}

	// Readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// May or may not error depending on timing
			repo.Load(ctx, "test")
		}()
	}

	// Assert
	wg.Wait() // Should complete without race conditions
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestMockWorkflowRepository_EmptyWorkflowName(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	ctx := context.Background()
	repo.AddWorkflow("", &workflow.Workflow{Name: ""})

	// Act & Assert
	wf, err := repo.Load(ctx, "")
	assert.NoError(t, err)
	assert.NotNil(t, wf)

	exists, err := repo.Exists(ctx, "")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestMockWorkflowRepository_NilWorkflow(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	ctx := context.Background()

	// Act
	repo.AddWorkflow("nil-wf", nil)

	// Assert
	wf, err := repo.Load(ctx, "nil-wf")
	assert.NoError(t, err)
	assert.Nil(t, wf, "Loading nil workflow should return nil")

	exists, err := repo.Exists(ctx, "nil-wf")
	assert.NoError(t, err)
	assert.True(t, exists, "Nil workflow should still exist in map")
}

func TestMockWorkflowRepository_SpecialCharacterNames(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
	}{
		{"unicode name", "test-workflow-😀"},
		{"path-like name", "path/to/workflow"},
		{"special chars", "test@#$%^&*()"},
		{"whitespace", "test workflow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := testutil.NewMockWorkflowRepository()
			ctx := context.Background()
			wf := &workflow.Workflow{Name: tt.workflowName}

			// Act
			repo.AddWorkflow(tt.workflowName, wf)

			// Assert
			loaded, err := repo.Load(ctx, tt.workflowName)
			assert.NoError(t, err)
			assert.NotNil(t, loaded)
			assert.Equal(t, tt.workflowName, loaded.Name)
		})
	}
}

func TestMockWorkflowRepository_LargeNumberOfWorkflows(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	ctx := context.Background()
	count := 1000

	// Act - add many workflows
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("%c%d", rune('a'+(i%26)), i)
		repo.AddWorkflow(name, &workflow.Workflow{Name: name})
	}

	// Assert
	names, err := repo.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, names, count)

	// Verify all are loadable
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("%c%d", rune('a'+(i%26)), i)
		wf, err := repo.Load(ctx, name)
		assert.NoError(t, err)
		assert.NotNil(t, wf)
	}
}

// =============================================================================
// MockStateStore Tests
// =============================================================================

// Feature: C007 Test Infrastructure Modernization
// Component: T003 MockStateStore

func TestMockStateStore_NewMockStateStore(t *testing.T) {
	// Arrange & Act
	store := testutil.NewMockStateStore()

	// Assert
	require.NotNil(t, store, "NewMockStateStore should return non-nil instance")

	// Verify it's usable immediately
	ctx := context.Background()
	state, err := store.Load(ctx, "nonexistent")
	assert.NoError(t, err, "Load on empty store should not error")
	assert.Nil(t, state, "Load on empty store should return nil for nonexistent workflow")

	ids, err := store.List(ctx)
	assert.NoError(t, err, "List on empty store should not error")
	assert.Empty(t, ids, "List on empty store should return empty slice")
}

func TestMockStateStore_Save_HappyPath(t *testing.T) {
	tests := []struct {
		name  string
		state *workflow.ExecutionContext
	}{
		{
			name: "valid state with workflow ID",
			state: &workflow.ExecutionContext{
				WorkflowID: "test-workflow-123",
				Status:     workflow.StatusRunning,
			},
		},
		{
			name: "state with multiple fields",
			state: &workflow.ExecutionContext{
				WorkflowID: "complex-workflow",
				Status:     workflow.StatusCompleted,
				States:     make(map[string]workflow.StepState),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			store := testutil.NewMockStateStore()
			ctx := context.Background()

			// Act
			err := store.Save(ctx, tt.state)

			// Assert
			assert.NoError(t, err, "Save should not return error for valid state")

			// Verify state can be loaded back
			loaded, err := store.Load(ctx, tt.state.WorkflowID)
			assert.NoError(t, err, "Load should not error after successful Save")
			assert.Equal(t, tt.state.WorkflowID, loaded.WorkflowID, "Loaded state should match saved state")
		})
	}
}

func TestMockStateStore_Save_ErrorInjection(t *testing.T) {
	// Arrange
	store := testutil.NewMockStateStore()
	ctx := context.Background()
	expectedErr := errors.New("save failed")
	store.SetSaveError(expectedErr)

	state := &workflow.ExecutionContext{
		WorkflowID: "test-workflow",
		Status:     workflow.StatusRunning,
	}

	// Act
	err := store.Save(ctx, state)

	// Assert
	assert.Error(t, err, "Save should return error when error is configured")
	assert.Equal(t, expectedErr, err, "Save should return the configured error")
}

func TestMockStateStore_Load_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.MockStateStore)
		workflowID string
		wantState  bool
	}{
		{
			name: "load existing state",
			setupFunc: func(store *testutil.MockStateStore) {
				state := &workflow.ExecutionContext{
					WorkflowID: "existing",
					Status:     workflow.StatusRunning,
				}
				_ = store.Save(context.Background(), state)
			},
			workflowID: "existing",
			wantState:  true,
		},
		{
			name: "load nonexistent state",
			setupFunc: func(store *testutil.MockStateStore) {
				// No setup needed
			},
			workflowID: "nonexistent",
			wantState:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			store := testutil.NewMockStateStore()
			tt.setupFunc(store)
			ctx := context.Background()

			// Act
			state, err := store.Load(ctx, tt.workflowID)

			// Assert
			assert.NoError(t, err, "Load should not error")
			if tt.wantState {
				assert.NotNil(t, state, "Load should return state when it exists")
				assert.Equal(t, tt.workflowID, state.WorkflowID, "Loaded state should have correct workflow ID")
			} else {
				assert.Nil(t, state, "Load should return nil for nonexistent workflow")
			}
		})
	}
}

func TestMockStateStore_Load_ErrorInjection(t *testing.T) {
	// Arrange
	store := testutil.NewMockStateStore()
	ctx := context.Background()
	expectedErr := errors.New("load failed")
	store.SetLoadError(expectedErr)

	// Act
	state, err := store.Load(ctx, "any-workflow")

	// Assert
	assert.Error(t, err, "Load should return error when error is configured")
	assert.Equal(t, expectedErr, err, "Load should return the configured error")
	assert.Nil(t, state, "Load should return nil state when error occurs")
}

func TestMockStateStore_Delete_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.MockStateStore)
		workflowID string
	}{
		{
			name: "delete existing state",
			setupFunc: func(store *testutil.MockStateStore) {
				state := &workflow.ExecutionContext{
					WorkflowID: "to-delete",
					Status:     workflow.StatusRunning,
				}
				_ = store.Save(context.Background(), state)
			},
			workflowID: "to-delete",
		},
		{
			name: "delete nonexistent state",
			setupFunc: func(store *testutil.MockStateStore) {
				// No setup needed
			},
			workflowID: "nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			store := testutil.NewMockStateStore()
			tt.setupFunc(store)
			ctx := context.Background()

			// Act
			err := store.Delete(ctx, tt.workflowID)

			// Assert
			assert.NoError(t, err, "Delete should not error")

			// Verify state is gone
			state, err := store.Load(ctx, tt.workflowID)
			assert.NoError(t, err, "Load after Delete should not error")
			assert.Nil(t, state, "State should not exist after Delete")
		})
	}
}

func TestMockStateStore_Delete_ErrorInjection(t *testing.T) {
	// Arrange
	store := testutil.NewMockStateStore()
	ctx := context.Background()
	expectedErr := errors.New("delete failed")
	store.SetDeleteError(expectedErr)

	// Act
	err := store.Delete(ctx, "any-workflow")

	// Assert
	assert.Error(t, err, "Delete should return error when error is configured")
	assert.Equal(t, expectedErr, err, "Delete should return the configured error")
}

func TestMockStateStore_List_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*testutil.MockStateStore)
		wantCount int
		wantIDs   []string
	}{
		{
			name: "empty store",
			setupFunc: func(store *testutil.MockStateStore) {
				// No setup needed
			},
			wantCount: 0,
			wantIDs:   []string{},
		},
		{
			name: "single state",
			setupFunc: func(store *testutil.MockStateStore) {
				state := &workflow.ExecutionContext{
					WorkflowID: "workflow-1",
					Status:     workflow.StatusRunning,
				}
				_ = store.Save(context.Background(), state)
			},
			wantCount: 1,
			wantIDs:   []string{"workflow-1"},
		},
		{
			name: "multiple states",
			setupFunc: func(store *testutil.MockStateStore) {
				for i := 1; i <= 3; i++ {
					state := &workflow.ExecutionContext{
						WorkflowID: fmt.Sprintf("workflow-%d", i),
						Status:     workflow.StatusRunning,
					}
					_ = store.Save(context.Background(), state)
				}
			},
			wantCount: 3,
			wantIDs:   []string{"workflow-1", "workflow-2", "workflow-3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			store := testutil.NewMockStateStore()
			tt.setupFunc(store)
			ctx := context.Background()

			// Act
			ids, err := store.List(ctx)

			// Assert
			assert.NoError(t, err, "List should not error")
			assert.Len(t, ids, tt.wantCount, "List should return correct number of IDs")
			assert.ElementsMatch(t, tt.wantIDs, ids, "List should return correct workflow IDs")
		})
	}
}

func TestMockStateStore_List_ErrorInjection(t *testing.T) {
	// Arrange
	store := testutil.NewMockStateStore()
	ctx := context.Background()
	expectedErr := errors.New("list failed")
	store.SetListError(expectedErr)

	// Act
	ids, err := store.List(ctx)

	// Assert
	assert.Error(t, err, "List should return error when error is configured")
	assert.Equal(t, expectedErr, err, "List should return the configured error")
	assert.Nil(t, ids, "List should return nil when error occurs")
}

func TestMockStateStore_ConcurrentAccess(t *testing.T) {
	// Arrange
	store := testutil.NewMockStateStore()
	ctx := context.Background()
	numGoroutines := 50

	// Act - concurrent reads and writes
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // save + load + list operations

	// Concurrent Save operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			state := &workflow.ExecutionContext{
				WorkflowID: fmt.Sprintf("workflow-%d", id),
				Status:     workflow.StatusRunning,
			}
			_ = store.Save(ctx, state)
		}(i)
	}

	// Concurrent Load operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			_, _ = store.Load(ctx, fmt.Sprintf("workflow-%d", id))
		}(i)
	}

	// Concurrent List operations
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = store.List(ctx)
		}()
	}

	wg.Wait()

	// Assert - verify no race conditions and data consistency
	ids, err := store.List(ctx)
	assert.NoError(t, err, "List should not error after concurrent access")
	assert.NotEmpty(t, ids, "Store should contain states after concurrent saves")
}

func TestMockStateStore_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "save with empty workflow ID",
			test: func(t *testing.T) {
				store := testutil.NewMockStateStore()
				ctx := context.Background()
				state := &workflow.ExecutionContext{
					WorkflowID: "",
					Status:     workflow.StatusRunning,
				}
				err := store.Save(ctx, state)
				assert.NoError(t, err, "Save should handle empty workflow ID")
			},
		},
		{
			name: "save nil state",
			test: func(t *testing.T) {
				store := testutil.NewMockStateStore()
				ctx := context.Background()
				err := store.Save(ctx, nil)
				// Behavior depends on implementation - test documents expected behavior
				_ = err // Current stub doesn't validate
			},
		},
		{
			name: "load with special characters in ID",
			test: func(t *testing.T) {
				store := testutil.NewMockStateStore()
				ctx := context.Background()
				specialID := "workflow-!@#$%^&*()"
				state := &workflow.ExecutionContext{
					WorkflowID: specialID,
					Status:     workflow.StatusRunning,
				}
				_ = store.Save(ctx, state)
				loaded, err := store.Load(ctx, specialID)
				assert.NoError(t, err, "Load should handle special characters in ID")
				if loaded != nil {
					assert.Equal(t, specialID, loaded.WorkflowID, "Should preserve special characters")
				}
			},
		},
		{
			name: "save overwrites existing state",
			test: func(t *testing.T) {
				store := testutil.NewMockStateStore()
				ctx := context.Background()
				workflowID := "workflow-1"

				// Save initial state
				state1 := &workflow.ExecutionContext{
					WorkflowID: workflowID,
					Status:     workflow.StatusRunning,
				}
				_ = store.Save(ctx, state1)

				// Save updated state
				state2 := &workflow.ExecutionContext{
					WorkflowID: workflowID,
					Status:     workflow.StatusCompleted,
				}
				_ = store.Save(ctx, state2)

				// Load and verify latest state
				loaded, err := store.Load(ctx, workflowID)
				assert.NoError(t, err, "Load should succeed")
				if loaded != nil {
					assert.Equal(t, workflow.StatusCompleted, loaded.Status, "Save should overwrite existing state")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func TestMockStateStore_Clear(t *testing.T) {
	// Arrange
	store := testutil.NewMockStateStore()
	ctx := context.Background()

	// Add some states
	for i := 1; i <= 3; i++ {
		state := &workflow.ExecutionContext{
			WorkflowID: fmt.Sprintf("workflow-%d", i),
			Status:     workflow.StatusRunning,
		}
		_ = store.Save(ctx, state)
	}

	// Configure error
	store.SetSaveError(errors.New("save error"))

	// Act
	store.Clear()

	// Assert
	// Verify all states are removed
	ids, err := store.List(ctx)
	require.NoError(t, err, "List should not error after Clear")
	assert.Empty(t, ids, "Clear should remove all states")

	// Verify errors are reset
	state := &workflow.ExecutionContext{
		WorkflowID: "new-workflow",
		Status:     workflow.StatusRunning,
	}
	err = store.Save(ctx, state)
	assert.NoError(t, err, "Clear should reset error configuration")
}

// =============================================================================
// MockCommandExecutor Tests
// =============================================================================

// Feature: C007 Test Infrastructure Modernization
// Component: T004 MockCommandExecutor

func TestMockCommandExecutor_NewMockCommandExecutor(t *testing.T) {
	// Arrange & Act
	executor := testutil.NewMockCommandExecutor()

	// Assert
	require.NotNil(t, executor, "NewMockCommandExecutor should return non-nil instance")

	// Verify initial state
	calls := executor.GetCalls()
	assert.Empty(t, calls, "New executor should have no recorded calls")
}

func TestMockCommandExecutor_Execute_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.MockCommandExecutor)
		cmd        *ports.Command
		wantStdout string
		wantStderr string
		wantExit   int
	}{
		{
			name: "simple command execution",
			setupFunc: func(exec *testutil.MockCommandExecutor) {
				exec.SetResult(&ports.CommandResult{
					Stdout:   "output",
					Stderr:   "",
					ExitCode: 0,
				})
			},
			cmd: &ports.Command{
				Program: "echo hello",
				Dir:     "/tmp",
			},
			wantStdout: "output",
			wantStderr: "",
			wantExit:   0,
		},
		{
			name: "command with error output",
			setupFunc: func(exec *testutil.MockCommandExecutor) {
				exec.SetResult(&ports.CommandResult{
					Stdout:   "",
					Stderr:   "error message",
					ExitCode: 1,
				})
			},
			cmd: &ports.Command{
				Program: "false",
			},
			wantStdout: "",
			wantStderr: "error message",
			wantExit:   1,
		},
		{
			name: "command with both stdout and stderr",
			setupFunc: func(exec *testutil.MockCommandExecutor) {
				exec.SetResult(&ports.CommandResult{
					Stdout:   "normal output",
					Stderr:   "warning",
					ExitCode: 0,
				})
			},
			cmd: &ports.Command{
				Program: "some-tool",
				Dir:     "/workspace",
				Env:     map[string]string{"VAR": "value"},
			},
			wantStdout: "normal output",
			wantStderr: "warning",
			wantExit:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			executor := testutil.NewMockCommandExecutor()
			tt.setupFunc(executor)
			ctx := context.Background()

			// Act
			result, err := executor.Execute(ctx, tt.cmd)

			// Assert
			assert.NoError(t, err, "Execute should not error for configured result")
			assert.NotNil(t, result, "Execute should return result")
			assert.Equal(t, tt.wantStdout, result.Stdout, "Stdout should match configured value")
			assert.Equal(t, tt.wantStderr, result.Stderr, "Stderr should match configured value")
			assert.Equal(t, tt.wantExit, result.ExitCode, "ExitCode should match configured value")

			// Verify call was recorded
			calls := executor.GetCalls()
			assert.Len(t, calls, 1, "Execute should record the call")
			assert.Equal(t, tt.cmd.Program, calls[0].Program, "Recorded call should match executed command")
		})
	}
}

func TestMockCommandExecutor_Execute_ErrorInjection(t *testing.T) {
	// Arrange
	executor := testutil.NewMockCommandExecutor()
	ctx := context.Background()
	expectedErr := errors.New("execution failed")
	executor.SetExecuteError(expectedErr)

	cmd := &ports.Command{
		Program: "test-cmd",
	}

	// Act
	result, err := executor.Execute(ctx, cmd)

	// Assert
	assert.Error(t, err, "Execute should return error when error is configured")
	assert.Equal(t, expectedErr, err, "Execute should return the configured error")
	assert.Nil(t, result, "Execute should return nil result when error occurs")
}

func TestMockCommandExecutor_CallRecording(t *testing.T) {
	// Arrange
	executor := testutil.NewMockCommandExecutor()
	executor.SetResult(&ports.CommandResult{Stdout: "ok", ExitCode: 0})
	ctx := context.Background()

	commands := []*ports.Command{
		{Program: "cmd1", Dir: "/dir1"},
		{Program: "cmd2", Dir: "/dir2", Env: map[string]string{"KEY": "val"}},
		{Program: "cmd3", Timeout: 30},
	}

	// Act - execute multiple commands
	for _, cmd := range commands {
		_, _ = executor.Execute(ctx, cmd)
	}

	// Assert
	calls := executor.GetCalls()
	assert.Len(t, calls, 3, "All executions should be recorded")

	for i, cmd := range commands {
		assert.Equal(t, cmd.Program, calls[i].Program, "Call %d program should match", i)
		assert.Equal(t, cmd.Dir, calls[i].Dir, "Call %d dir should match", i)
		if cmd.Env != nil {
			assert.Equal(t, cmd.Env, calls[i].Env, "Call %d env should match", i)
		}
	}
}

func TestMockCommandExecutor_ConcurrentAccess(t *testing.T) {
	// Arrange
	executor := testutil.NewMockCommandExecutor()
	executor.SetResult(&ports.CommandResult{Stdout: "output", ExitCode: 0})
	ctx := context.Background()
	numGoroutines := 50

	// Act - concurrent executions
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			cmd := &ports.Command{
				Program: fmt.Sprintf("cmd-%d", id),
			}
			_, _ = executor.Execute(ctx, cmd)
		}(i)
	}

	wg.Wait()

	// Assert - verify no race conditions
	calls := executor.GetCalls()
	assert.Len(t, calls, numGoroutines, "All concurrent executions should be recorded")
}

func TestMockCommandExecutor_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "execute with nil command",
			test: func(t *testing.T) {
				executor := testutil.NewMockCommandExecutor()
				executor.SetResult(&ports.CommandResult{ExitCode: 0})
				ctx := context.Background()

				result, err := executor.Execute(ctx, nil)
				// Test documents expected behavior for nil command
				_ = result
				_ = err
			},
		},
		{
			name: "execute without setting result",
			test: func(t *testing.T) {
				executor := testutil.NewMockCommandExecutor()
				ctx := context.Background()
				cmd := &ports.Command{Program: "test"}

				result, err := executor.Execute(ctx, cmd)
				// Without SetResult, stub returns nil
				assert.NoError(t, err, "Execute without result config should not error")
				assert.Nil(t, result, "Execute without result config returns nil")
			},
		},
		{
			name: "command with empty program",
			test: func(t *testing.T) {
				executor := testutil.NewMockCommandExecutor()
				executor.SetResult(&ports.CommandResult{ExitCode: 0})
				ctx := context.Background()
				cmd := &ports.Command{Program: ""}

				_, err := executor.Execute(ctx, cmd)
				assert.NoError(t, err, "Execute should handle empty program")

				calls := executor.GetCalls()
				assert.Len(t, calls, 1, "Empty program should still be recorded")
			},
		},
		{
			name: "command with large output",
			test: func(t *testing.T) {
				executor := testutil.NewMockCommandExecutor()
				largeOutput := string(make([]byte, 10000))
				executor.SetResult(&ports.CommandResult{
					Stdout:   largeOutput,
					ExitCode: 0,
				})
				ctx := context.Background()
				cmd := &ports.Command{Program: "generate-data"}

				result, err := executor.Execute(ctx, cmd)
				assert.NoError(t, err, "Execute should handle large output")
				assert.Len(t, result.Stdout, 10000, "Large output should be preserved")
			},
		},
		{
			name: "multiple executions of same command",
			test: func(t *testing.T) {
				executor := testutil.NewMockCommandExecutor()
				executor.SetResult(&ports.CommandResult{Stdout: "ok", ExitCode: 0})
				ctx := context.Background()
				cmd := &ports.Command{Program: "repeated-cmd"}

				// Execute same command 3 times
				for i := 0; i < 3; i++ {
					_, err := executor.Execute(ctx, cmd)
					assert.NoError(t, err, "Execution %d should succeed", i)
				}

				calls := executor.GetCalls()
				assert.Len(t, calls, 3, "All executions should be recorded")
				for _, call := range calls {
					assert.Equal(t, "repeated-cmd", call.Program, "All calls should have same program")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func TestMockCommandExecutor_Clear(t *testing.T) {
	// Arrange
	executor := testutil.NewMockCommandExecutor()
	executor.SetResult(&ports.CommandResult{Stdout: "output", ExitCode: 0})
	executor.SetExecuteError(errors.New("error"))
	ctx := context.Background()

	// Execute some commands
	for i := 0; i < 3; i++ {
		cmd := &ports.Command{Program: fmt.Sprintf("cmd-%d", i)}
		_, _ = executor.Execute(ctx, cmd)
	}

	// Act
	executor.Clear()

	// Assert
	// Verify calls are cleared
	calls := executor.GetCalls()
	assert.Empty(t, calls, "Clear should remove all recorded calls")

	// Verify result is cleared
	cmd := &ports.Command{Program: "new-cmd"}
	result, err := executor.Execute(ctx, cmd)
	assert.NoError(t, err, "Clear should reset error configuration")
	assert.Nil(t, result, "Clear should reset result configuration")
}

func TestMockCommandExecutor_GetCalls_IsolatedCopy(t *testing.T) {
	// Arrange
	executor := testutil.NewMockCommandExecutor()
	executor.SetResult(&ports.CommandResult{ExitCode: 0})
	ctx := context.Background()

	cmd := &ports.Command{Program: "test-cmd"}
	_, _ = executor.Execute(ctx, cmd)

	// Act - get calls twice
	calls1 := executor.GetCalls()
	calls2 := executor.GetCalls()

	// Assert - modifications to returned slice shouldn't affect internal state
	calls1[0].Program = "modified"
	assert.NotEqual(t, calls1[0].Program, calls2[0].Program, "GetCalls should return isolated copy")
	assert.Equal(t, "test-cmd", calls2[0].Program, "Internal state should be unaffected by modifications")
}

// =============================================================================
// MockLogger Tests
// =============================================================================

// Feature: C007 Test Infrastructure Modernization
// Component: T005 MockLogger

func TestMockLogger_NewMockLogger(t *testing.T) {
	// Arrange & Act
	logger := testutil.NewMockLogger()

	// Assert
	require.NotNil(t, logger, "NewMockLogger should return non-nil instance")

	// Verify initial state
	messages := logger.GetMessages()
	assert.Empty(t, messages, "New logger should have no captured messages")
}

func TestMockLogger_Debug(t *testing.T) {
	tests := []struct {
		name   string
		msg    string
		fields []any
	}{
		{
			name:   "simple debug message",
			msg:    "debug message",
			fields: nil,
		},
		{
			name:   "debug with single field",
			msg:    "user action",
			fields: []any{"user_id", 123},
		},
		{
			name:   "debug with multiple fields",
			msg:    "request processed",
			fields: []any{"method", "GET", "path", "/api/users", "duration_ms", 45},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			logger := testutil.NewMockLogger()

			// Act
			logger.Debug(tt.msg, tt.fields...)

			// Assert
			messages := logger.GetMessages()
			assert.Len(t, messages, 1, "Debug should capture message")
			assert.Equal(t, "DEBUG", messages[0].Level, "Level should be DEBUG")
			assert.Equal(t, tt.msg, messages[0].Msg, "Message should match")
			if tt.fields != nil {
				assert.Equal(t, tt.fields, messages[0].Fields, "Fields should match")
			}
		})
	}
}

func TestMockLogger_Info(t *testing.T) {
	tests := []struct {
		name   string
		msg    string
		fields []any
	}{
		{
			name:   "simple info message",
			msg:    "application started",
			fields: nil,
		},
		{
			name:   "info with fields",
			msg:    "workflow completed",
			fields: []any{"workflow_id", "wf-123", "duration", "5s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			logger := testutil.NewMockLogger()

			// Act
			logger.Info(tt.msg, tt.fields...)

			// Assert
			messages := logger.GetMessages()
			assert.Len(t, messages, 1, "Info should capture message")
			assert.Equal(t, "INFO", messages[0].Level, "Level should be INFO")
			assert.Equal(t, tt.msg, messages[0].Msg, "Message should match")
		})
	}
}

func TestMockLogger_Warn(t *testing.T) {
	tests := []struct {
		name   string
		msg    string
		fields []any
	}{
		{
			name:   "simple warning",
			msg:    "deprecated API used",
			fields: nil,
		},
		{
			name:   "warning with context",
			msg:    "retry attempted",
			fields: []any{"attempt", 2, "max_attempts", 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			logger := testutil.NewMockLogger()

			// Act
			logger.Warn(tt.msg, tt.fields...)

			// Assert
			messages := logger.GetMessages()
			assert.Len(t, messages, 1, "Warn should capture message")
			assert.Equal(t, "WARN", messages[0].Level, "Level should be WARN")
			assert.Equal(t, tt.msg, messages[0].Msg, "Message should match")
		})
	}
}

func TestMockLogger_Error(t *testing.T) {
	tests := []struct {
		name   string
		msg    string
		fields []any
	}{
		{
			name:   "simple error",
			msg:    "operation failed",
			fields: nil,
		},
		{
			name:   "error with details",
			msg:    "database connection failed",
			fields: []any{"error", "connection timeout", "db_host", "localhost:5432"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			logger := testutil.NewMockLogger()

			// Act
			logger.Error(tt.msg, tt.fields...)

			// Assert
			messages := logger.GetMessages()
			assert.Len(t, messages, 1, "Error should capture message")
			assert.Equal(t, "ERROR", messages[0].Level, "Level should be ERROR")
			assert.Equal(t, tt.msg, messages[0].Msg, "Message should match")
		})
	}
}

func TestMockLogger_MultipleMessages(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()

	// Act - log messages at different levels
	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")

	// Assert
	messages := logger.GetMessages()
	assert.Len(t, messages, 4, "All messages should be captured")

	expectedLevels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	for i, level := range expectedLevels {
		assert.Equal(t, level, messages[i].Level, "Message %d should have correct level", i)
	}
}

func TestMockLogger_GetMessagesByLevel(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*testutil.MockLogger)
		level     string
		wantCount int
	}{
		{
			name: "filter debug messages",
			setupFunc: func(logger *testutil.MockLogger) {
				logger.Debug("debug1")
				logger.Info("info1")
				logger.Debug("debug2")
				logger.Error("error1")
			},
			level:     "DEBUG",
			wantCount: 2,
		},
		{
			name: "filter info messages",
			setupFunc: func(logger *testutil.MockLogger) {
				logger.Info("info1")
				logger.Info("info2")
				logger.Warn("warn1")
			},
			level:     "INFO",
			wantCount: 2,
		},
		{
			name: "filter non-existent level",
			setupFunc: func(logger *testutil.MockLogger) {
				logger.Info("info1")
				logger.Error("error1")
			},
			level:     "TRACE",
			wantCount: 0,
		},
		{
			name: "no messages at level",
			setupFunc: func(logger *testutil.MockLogger) {
				logger.Info("info1")
				logger.Warn("warn1")
			},
			level:     "ERROR",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			logger := testutil.NewMockLogger()
			tt.setupFunc(logger)

			// Act
			filtered := logger.GetMessagesByLevel(tt.level)

			// Assert
			assert.Len(t, filtered, tt.wantCount, "Should return correct number of messages for level %s", tt.level)
			for _, msg := range filtered {
				assert.Equal(t, tt.level, msg.Level, "All filtered messages should match requested level")
			}
		})
	}
}

func TestMockLogger_WithContext(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{
		"request_id": "req-123",
		"user_id":    456,
	}

	// Act
	contextLogger := logger.WithContext(ctx)

	// Assert
	assert.NotNil(t, contextLogger, "WithContext should return a logger")

	// Log with context logger
	contextLogger.Info("test message")

	messages := logger.GetMessages()
	assert.NotEmpty(t, messages, "Context logger should capture messages")
	// Note: actual context implementation will be tested when implemented
}

func TestMockLogger_ConcurrentAccess(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	numGoroutines := 50

	// Act - concurrent logging
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 4) // 4 log levels

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			logger.Debug(fmt.Sprintf("debug-%d", id))
		}(i)
		go func(id int) {
			defer wg.Done()
			logger.Info(fmt.Sprintf("info-%d", id))
		}(i)
		go func(id int) {
			defer wg.Done()
			logger.Warn(fmt.Sprintf("warn-%d", id))
		}(i)
		go func(id int) {
			defer wg.Done()
			logger.Error(fmt.Sprintf("error-%d", id))
		}(i)
	}

	wg.Wait()

	// Assert - verify no race conditions
	messages := logger.GetMessages()
	assert.Len(t, messages, numGoroutines*4, "All concurrent log calls should be captured")
}

func TestMockLogger_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "empty message",
			test: func(t *testing.T) {
				logger := testutil.NewMockLogger()
				logger.Info("")
				messages := logger.GetMessages()
				assert.Len(t, messages, 1, "Empty message should be captured")
				assert.Equal(t, "", messages[0].Msg, "Empty message should be preserved")
			},
		},
		{
			name: "long message",
			test: func(t *testing.T) {
				logger := testutil.NewMockLogger()
				longMsg := string(make([]byte, 10000))
				logger.Info(longMsg)
				messages := logger.GetMessages()
				assert.Len(t, messages, 1, "Long message should be captured")
				assert.Len(t, messages[0].Msg, 10000, "Long message should be preserved")
			},
		},
		{
			name: "special characters in message",
			test: func(t *testing.T) {
				logger := testutil.NewMockLogger()
				specialMsg := "message with \n\t special \r chars 日本語"
				logger.Info(specialMsg)
				messages := logger.GetMessages()
				assert.Equal(t, specialMsg, messages[0].Msg, "Special characters should be preserved")
			},
		},
		{
			name: "odd number of fields",
			test: func(t *testing.T) {
				logger := testutil.NewMockLogger()
				logger.Info("test", "key1", "value1", "key2") // missing value for key2
				messages := logger.GetMessages()
				assert.Len(t, messages, 1, "Message with odd fields should be captured")
				// Behavior depends on implementation
			},
		},
		{
			name: "nil fields",
			test: func(t *testing.T) {
				logger := testutil.NewMockLogger()
				logger.Info("test", "key", nil)
				messages := logger.GetMessages()
				assert.Len(t, messages, 1, "Message with nil field value should be captured")
			},
		},
		{
			name: "many fields",
			test: func(t *testing.T) {
				logger := testutil.NewMockLogger()
				fields := make([]any, 100)
				for i := 0; i < 50; i++ {
					fields[i*2] = fmt.Sprintf("key%d", i)
					fields[i*2+1] = fmt.Sprintf("value%d", i)
				}
				logger.Info("test", fields...)
				messages := logger.GetMessages()
				assert.Len(t, messages, 1, "Message with many fields should be captured")
				assert.Len(t, messages[0].Fields, 100, "All fields should be captured")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func TestMockLogger_Clear(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()

	// Log multiple messages
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	// Verify messages exist
	messages := logger.GetMessages()
	require.Len(t, messages, 4, "Should have messages before clear")

	// Act
	logger.Clear()

	// Assert
	messages = logger.GetMessages()
	assert.Empty(t, messages, "Clear should remove all messages")

	// Verify logger still works after clear
	logger.Info("new message")
	messages = logger.GetMessages()
	assert.Len(t, messages, 1, "Logger should work after clear")
	assert.Equal(t, "new message", messages[0].Msg, "New message should be captured")
}

func TestMockLogger_GetMessages_IsolatedCopy(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	logger.Info("test message")

	// Act - get messages twice
	messages1 := logger.GetMessages()
	messages2 := logger.GetMessages()

	// Assert - modifications to returned slice shouldn't affect internal state
	messages1[0].Msg = "modified"
	assert.NotEqual(t, messages1[0].Msg, messages2[0].Msg, "GetMessages should return isolated copy")
	assert.Equal(t, "test message", messages2[0].Msg, "Internal state should be unaffected by modifications")
}

func TestMockLogger_MessageOrdering(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()

	// Act - log messages in specific order
	logger.Debug("first")
	logger.Info("second")
	logger.Warn("third")
	logger.Error("fourth")

	// Assert - messages should be in order
	messages := logger.GetMessages()
	assert.Equal(t, "first", messages[0].Msg, "Message order should be preserved")
	assert.Equal(t, "second", messages[1].Msg, "Message order should be preserved")
	assert.Equal(t, "third", messages[2].Msg, "Message order should be preserved")
	assert.Equal(t, "fourth", messages[3].Msg, "Message order should be preserved")
}
