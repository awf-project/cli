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
// Interface Compliance Checks (Compile-Time)
// =============================================================================

// Feature: C016 Missing Unit Tests for Input Validation and State Persistence
// Component: T001 Testutil Interface Compliance

// These compile-time assertions verify that mock implementations satisfy their
// corresponding port interfaces. If a mock fails to implement its interface,
// the code will not compile, catching interface mismatches early.
var (
	_ ports.WorkflowRepository  = (*testutil.MockWorkflowRepository)(nil)
	_ ports.StateStore          = (*testutil.MockStateStore)(nil)
	_ ports.CommandExecutor     = (*testutil.MockCommandExecutor)(nil)
	_ ports.Logger              = (*testutil.MockLogger)(nil)
	_ ports.HistoryStore        = (*testutil.MockHistoryStore)(nil)
	_ ports.ExpressionValidator = (*testutil.MockExpressionValidator)(nil)
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
				exec.SetCommandResult("", &ports.CommandResult{
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
				exec.SetCommandResult("", &ports.CommandResult{
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
				exec.SetCommandResult("", &ports.CommandResult{
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
	executor.SetCommandResult("", &ports.CommandResult{Stdout: "ok", ExitCode: 0})
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
	executor.SetCommandResult("", &ports.CommandResult{Stdout: "output", ExitCode: 0})
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
				executor.SetCommandResult("", &ports.CommandResult{ExitCode: 0})
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
				executor.SetCommandResult("", &ports.CommandResult{ExitCode: 0})
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
				executor.SetCommandResult("", &ports.CommandResult{
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
				executor.SetCommandResult("", &ports.CommandResult{Stdout: "ok", ExitCode: 0})
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
	executor.SetCommandResult("", &ports.CommandResult{Stdout: "output", ExitCode: 0})
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
	executor.SetCommandResult("", &ports.CommandResult{ExitCode: 0})
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
	t.Run("InterfaceCompliance", func(t *testing.T) {
		// T005 Requirement 1: Test WithContext returns non-nil Logger (interface compliance)
		logger := testutil.NewMockLogger()
		ctx := map[string]any{
			"request_id": "req-123",
			"user_id":    456,
		}

		// Act
		contextLogger := logger.WithContext(ctx)

		// Assert - verify interface compliance
		assert.NotNil(t, contextLogger, "WithContext should return a non-nil logger")
		assert.Implements(t, (*ports.Logger)(nil), contextLogger, "Returned logger should implement ports.Logger interface")

		// Verify the returned logger can be used for logging
		contextLogger.Info("test message")
		messages := logger.GetMessages()
		assert.NotEmpty(t, messages, "Context logger should capture messages")
	})

	t.Run("ContextAccumulation", func(t *testing.T) {
		// T005 Requirement 2: Test context fields accumulate across chained calls
		logger := testutil.NewMockLogger()
		ctx1 := map[string]any{"key1": "val1"}
		ctx2 := map[string]any{"key2": "val2"}
		ctx3 := map[string]any{"key3": "val3"}

		// Act - chain multiple WithContext calls
		logger1 := logger.WithContext(ctx1)
		logger2 := logger1.WithContext(ctx2)
		logger3 := logger2.WithContext(ctx3)
		logger3.Info("test")

		// Assert - all context fields should be present
		messages := logger.GetMessages()
		require.Len(t, messages, 1)
		msg := messages[0]
		assert.Contains(t, msg.Fields, "key1", "First context should be preserved")
		assert.Contains(t, msg.Fields, "val1")
		assert.Contains(t, msg.Fields, "key2", "Second context should be added")
		assert.Contains(t, msg.Fields, "val2")
		assert.Contains(t, msg.Fields, "key3", "Third context should be added")
		assert.Contains(t, msg.Fields, "val3")
	})

	t.Run("OriginalLoggerImmutability", func(t *testing.T) {
		// T005 Requirement 3: Test original logger unchanged (immutability)
		logger := testutil.NewMockLogger()
		ctx := map[string]any{"context_key": "context_val"}

		// Act - create context logger but use original
		_ = logger.WithContext(ctx)
		logger.Info("original message", "msg_key", "msg_val")

		// Assert - original logger should not have context fields
		messages := logger.GetMessages()
		require.Len(t, messages, 1)
		msg := messages[0]
		assert.NotContains(t, msg.Fields, "context_key", "Original logger should remain unchanged")
		assert.Contains(t, msg.Fields, "msg_key", "Original logger should work normally")
	})

	t.Run("LogMessagesIncludeContext", func(t *testing.T) {
		// T005 Requirement 4: Test log messages include context fields
		logger := testutil.NewMockLogger()
		ctx := map[string]any{
			"trace_id": "trace-123",
			"span_id":  "span-456",
		}

		contextLogger := logger.WithContext(ctx)

		// Test all log levels include context
		contextLogger.Debug("debug msg", "debug_key", "debug_val")
		contextLogger.Info("info msg", "info_key", "info_val")
		contextLogger.Warn("warn msg", "warn_key", "warn_val")
		contextLogger.Error("error msg", "error_key", "error_val")

		// Assert - all messages should include context fields
		messages := logger.GetMessages()
		require.Len(t, messages, 4)

		for i, msg := range messages {
			assert.Contains(t, msg.Fields, "trace_id", "Message %d should include trace_id", i)
			assert.Contains(t, msg.Fields, "trace-123", "Message %d should include trace_id value", i)
			assert.Contains(t, msg.Fields, "span_id", "Message %d should include span_id", i)
			assert.Contains(t, msg.Fields, "span-456", "Message %d should include span_id value", i)
		}
	})

	t.Run("ConcurrentAccess50Goroutines", func(t *testing.T) {
		// T005 Requirement 5: Test thread-safety with 50 concurrent goroutines
		logger := testutil.NewMockLogger()
		numGoroutines := 50

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Act - spawn 50 goroutines creating context loggers and logging
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				ctx := map[string]any{
					"goroutine_id": id,
					"iteration":    fmt.Sprintf("iter-%d", id),
				}
				contextLogger := logger.WithContext(ctx)
				contextLogger.Info(fmt.Sprintf("concurrent message %d", id), "extra", id*2)
			}(i)
		}

		wg.Wait()

		// Assert - all 50 messages should be captured without race conditions
		messages := logger.GetMessages()
		assert.Len(t, messages, numGoroutines, "All 50 concurrent goroutines should log successfully")

		// Verify no data corruption - each message has unique goroutine_id
		goroutineIDs := make(map[int]bool)
		for _, msg := range messages {
			for i := 0; i < len(msg.Fields)-1; i += 2 {
				if msg.Fields[i] == "goroutine_id" {
					if id, ok := msg.Fields[i+1].(int); ok {
						goroutineIDs[id] = true
					}
				}
			}
		}
		assert.Len(t, goroutineIDs, numGoroutines, "Each goroutine should have unique ID")
	})

	t.Run("EdgeCaseNilMap", func(t *testing.T) {
		// T005 Requirement 6a: Test edge case - nil map
		logger := testutil.NewMockLogger()

		// Act - nil context should not panic
		contextLogger := logger.WithContext(nil)
		contextLogger.Info("message with nil context", "key", "val")

		// Assert
		messages := logger.GetMessages()
		require.Len(t, messages, 1)
		assert.Equal(t, "message with nil context", messages[0].Msg)
		assert.Contains(t, messages[0].Fields, "key")
	})

	t.Run("EdgeCaseEmptyMap", func(t *testing.T) {
		// T005 Requirement 6b: Test edge case - empty map
		logger := testutil.NewMockLogger()
		emptyCtx := map[string]any{}

		// Act
		contextLogger := logger.WithContext(emptyCtx)
		contextLogger.Info("message with empty context", "key", "val")

		// Assert
		messages := logger.GetMessages()
		require.Len(t, messages, 1)
		msg := messages[0]
		assert.Equal(t, "message with empty context", msg.Msg)
		assert.Contains(t, msg.Fields, "key", "Message fields should still work")
		assert.Contains(t, msg.Fields, "val")
	})
}

// =============================================================================
// T001: ctxFields field tests
// =============================================================================

func TestMockLogger_CtxFields_HappyPath_SingleContext(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{
		"request_id": "req-123",
		"user_id":    456,
	}

	// Act
	contextLogger := logger.WithContext(ctx)
	contextLogger.Info("operation completed", "duration_ms", 150)

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture one message")

	msg := messages[0]
	assert.Equal(t, "INFO", msg.Level)
	assert.Equal(t, "operation completed", msg.Msg)

	// Verify context fields are merged with message fields
	// Expected fields: [request_id, req-123, user_id, 456, duration_ms, 150]
	assert.Contains(t, msg.Fields, "request_id", "Context field key should be present")
	assert.Contains(t, msg.Fields, "req-123", "Context field value should be present")
	assert.Contains(t, msg.Fields, "user_id", "Context field key should be present")
	assert.Contains(t, msg.Fields, 456, "Context field value should be present")
	assert.Contains(t, msg.Fields, "duration_ms", "Message field key should be present")
	assert.Contains(t, msg.Fields, 150, "Message field value should be present")
}

func TestMockLogger_CtxFields_HappyPath_ChainedContexts(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx1 := map[string]any{"request_id": "req-123"}
	ctx2 := map[string]any{"span_id": "span-456"}

	// Act - chain multiple contexts
	logger1 := logger.WithContext(ctx1)
	logger2 := logger1.WithContext(ctx2)
	logger2.Info("nested operation")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture one message")

	msg := messages[0]
	// Verify both context layers are present
	assert.Contains(t, msg.Fields, "request_id", "First context should be preserved")
	assert.Contains(t, msg.Fields, "req-123", "First context value should be preserved")
	assert.Contains(t, msg.Fields, "span_id", "Second context should be added")
	assert.Contains(t, msg.Fields, "span-456", "Second context value should be added")
}

func TestMockLogger_CtxFields_HappyPath_EmptyContext(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	emptyCtx := map[string]any{}

	// Act
	contextLogger := logger.WithContext(emptyCtx)
	contextLogger.Info("message with empty context", "key", "value")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture one message")

	msg := messages[0]
	// Empty context should not add extra fields, only message fields
	assert.Contains(t, msg.Fields, "key", "Message fields should be present")
	assert.Contains(t, msg.Fields, "value", "Message fields should be present")
}

func TestMockLogger_CtxFields_EdgeCase_NilContext(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()

	// Act - nil context map should not panic
	contextLogger := logger.WithContext(nil)
	contextLogger.Info("message with nil context")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture one message even with nil context")
	assert.Equal(t, "message with nil context", messages[0].Msg)
}

func TestMockLogger_CtxFields_EdgeCase_NilValues(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{
		"optional_field": nil,
		"present_field":  "value",
	}

	// Act
	contextLogger := logger.WithContext(ctx)
	contextLogger.Info("message with nil context values")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture message with nil context values")

	msg := messages[0]
	assert.Contains(t, msg.Fields, "optional_field", "Nil value key should be present")
	assert.Contains(t, msg.Fields, "present_field", "Non-nil value key should be present")
	assert.Contains(t, msg.Fields, "value", "Non-nil value should be present")
}

func TestMockLogger_CtxFields_EdgeCase_ComplexTypes(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{
		"string":  "text",
		"int":     42,
		"float":   3.14,
		"bool":    true,
		"slice":   []string{"a", "b", "c"},
		"map":     map[string]int{"count": 5},
		"pointer": &struct{ Name string }{Name: "test"},
	}

	// Act
	contextLogger := logger.WithContext(ctx)
	contextLogger.Info("message with complex types")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture message with complex types")

	msg := messages[0]
	// Verify all types are preserved
	assert.Contains(t, msg.Fields, "string", "String key should be present")
	assert.Contains(t, msg.Fields, "int", "Int key should be present")
	assert.Contains(t, msg.Fields, "float", "Float key should be present")
	assert.Contains(t, msg.Fields, "bool", "Bool key should be present")
	assert.Contains(t, msg.Fields, "slice", "Slice key should be present")
	assert.Contains(t, msg.Fields, "map", "Map key should be present")
	assert.Contains(t, msg.Fields, "pointer", "Pointer key should be present")
}

func TestMockLogger_CtxFields_EdgeCase_DuplicateKeys(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{"key": "context_value"}

	// Act - message field with same key as context field
	contextLogger := logger.WithContext(ctx)
	contextLogger.Info("message", "key", "message_value")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture message")

	msg := messages[0]
	// Both values should be present (order may vary)
	assert.Contains(t, msg.Fields, "key", "Key should be present")
	assert.Contains(t, msg.Fields, "context_value", "Context value should be present")
	assert.Contains(t, msg.Fields, "message_value", "Message value should be present")
}

func TestMockLogger_CtxFields_ErrorHandling_AllLevels(t *testing.T) {
	tests := []struct {
		name    string
		logFunc func(logger ports.Logger, msg string, fields ...any)
		level   string
	}{
		{
			name: "Debug with context",
			logFunc: func(logger ports.Logger, msg string, fields ...any) {
				logger.Debug(msg, fields...)
			},
			level: "DEBUG",
		},
		{
			name: "Info with context",
			logFunc: func(logger ports.Logger, msg string, fields ...any) {
				logger.Info(msg, fields...)
			},
			level: "INFO",
		},
		{
			name: "Warn with context",
			logFunc: func(logger ports.Logger, msg string, fields ...any) {
				logger.Warn(msg, fields...)
			},
			level: "WARN",
		},
		{
			name: "Error with context",
			logFunc: func(logger ports.Logger, msg string, fields ...any) {
				logger.Error(msg, fields...)
			},
			level: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			logger := testutil.NewMockLogger()
			ctx := map[string]any{"context_key": "context_val"}

			// Act
			contextLogger := logger.WithContext(ctx)
			tt.logFunc(contextLogger, "test message", "msg_key", "msg_val")

			// Assert
			messages := logger.GetMessages()
			require.Len(t, messages, 1, "Should capture one message")

			msg := messages[0]
			assert.Equal(t, tt.level, msg.Level)
			assert.Contains(t, msg.Fields, "context_key", "Context fields should be present")
			assert.Contains(t, msg.Fields, "msg_key", "Message fields should be present")
		})
	}
}

func TestMockLogger_CtxFields_Concurrency_MultipleContextLoggers(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	numGoroutines := 50 // Updated to match T005 requirement

	// Act - create multiple context loggers concurrently
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			ctx := map[string]any{"goroutine_id": id}
			contextLogger := logger.WithContext(ctx)
			contextLogger.Info(fmt.Sprintf("message-%d", id))
		}(i)
	}

	wg.Wait()

	// Assert
	messages := logger.GetMessages()
	assert.Len(t, messages, numGoroutines, "All concurrent context loggers should capture messages")

	// Verify each message has its own context
	goroutineIDs := make(map[int]bool)
	for _, msg := range messages {
		// Each message should have a unique goroutine_id in its fields
		hasGoroutineID := false
		for _, field := range msg.Fields {
			if field == "goroutine_id" {
				hasGoroutineID = true
			}
			if id, ok := field.(int); ok && id >= 0 && id < numGoroutines {
				goroutineIDs[id] = true
			}
		}
		assert.True(t, hasGoroutineID, "Each message should have goroutine_id context")
	}

	// Verify all goroutine IDs were logged
	assert.Len(t, goroutineIDs, numGoroutines, "All goroutines should have logged with their ID")
}

func TestMockLogger_CtxFields_Immutability_OriginalLoggerUnaffected(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{"request_id": "req-123"}

	// Act
	contextLogger := logger.WithContext(ctx)

	// Log with original logger (no context)
	logger.Info("original logger message", "key1", "val1")

	// Log with context logger
	contextLogger.Info("context logger message", "key2", "val2")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 2, "Should capture both messages")

	// First message (original logger) should NOT have context fields
	msg1 := messages[0]
	assert.Equal(t, "original logger message", msg1.Msg)
	assert.NotContains(t, msg1.Fields, "request_id", "Original logger should not have context")
	assert.Contains(t, msg1.Fields, "key1", "Original logger should have its own fields")

	// Second message (context logger) should have context fields
	msg2 := messages[1]
	assert.Equal(t, "context logger message", msg2.Msg)
	assert.Contains(t, msg2.Fields, "request_id", "Context logger should have context")
	assert.Contains(t, msg2.Fields, "key2", "Context logger should have its own fields")
}

func TestMockLogger_CtxFields_FieldOrdering_ContextBeforeMessage(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{
		"ctx_key1": "ctx_val1",
		"ctx_key2": "ctx_val2",
	}

	// Act
	contextLogger := logger.WithContext(ctx)
	contextLogger.Info("test", "msg_key1", "msg_val1", "msg_key2", "msg_val2")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture one message")

	msg := messages[0]
	// Verify context fields appear before message fields
	// Fields structure: [ctx_key1, ctx_val1, ctx_key2, ctx_val2, msg_key1, msg_val1, msg_key2, msg_val2]
	assert.GreaterOrEqual(t, len(msg.Fields), 8, "Should have at least 8 fields (4 context + 4 message)")

	// Find indices of context and message keys
	ctxKeyIndex := -1
	msgKeyIndex := -1
	for i, field := range msg.Fields {
		if field == "ctx_key1" {
			ctxKeyIndex = i
		}
		if field == "msg_key1" {
			msgKeyIndex = i
		}
	}

	if ctxKeyIndex != -1 && msgKeyIndex != -1 {
		assert.Less(t, ctxKeyIndex, msgKeyIndex, "Context fields should appear before message fields")
	}
}

// =============================================================================
// T003: Log Methods Field Merging Tests (Component T003 - C041)
// =============================================================================

func TestMockLogger_T003_Debug_MergesCtxFieldsWithMessageFields(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{"trace_id": "abc123", "user_id": 42}
	contextLogger := logger.WithContext(ctx)

	// Act
	contextLogger.Debug("debug message", "key1", "val1", "key2", 100)

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture one message")

	msg := messages[0]
	assert.Equal(t, "DEBUG", msg.Level)
	assert.Equal(t, "debug message", msg.Msg)

	// Verify context fields are prepended to message fields
	assert.Contains(t, msg.Fields, "trace_id", "Context field key should be present")
	assert.Contains(t, msg.Fields, "abc123", "Context field value should be present")
	assert.Contains(t, msg.Fields, "user_id", "Context field key should be present")
	assert.Contains(t, msg.Fields, 42, "Context field value should be present")
	assert.Contains(t, msg.Fields, "key1", "Message field key should be present")
	assert.Contains(t, msg.Fields, "val1", "Message field value should be present")
	assert.Contains(t, msg.Fields, "key2", "Message field key should be present")
	assert.Contains(t, msg.Fields, 100, "Message field value should be present")

	// Verify total field count (4 context fields + 4 message fields = 8)
	assert.Len(t, msg.Fields, 8, "Should have exactly 8 fields")
}

func TestMockLogger_T003_Info_MergesCtxFieldsWithMessageFields(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{"request_id": "req-456"}
	contextLogger := logger.WithContext(ctx)

	// Act
	contextLogger.Info("info message", "status", "success")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture one message")

	msg := messages[0]
	assert.Equal(t, "INFO", msg.Level)
	assert.Equal(t, "info message", msg.Msg)

	// Verify context fields are prepended
	assert.Contains(t, msg.Fields, "request_id", "Context field key should be present")
	assert.Contains(t, msg.Fields, "req-456", "Context field value should be present")
	assert.Contains(t, msg.Fields, "status", "Message field key should be present")
	assert.Contains(t, msg.Fields, "success", "Message field value should be present")

	// Verify total field count (2 context fields + 2 message fields = 4)
	assert.Len(t, msg.Fields, 4, "Should have exactly 4 fields")
}

func TestMockLogger_T003_Warn_MergesCtxFieldsWithMessageFields(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{"service": "api", "environment": "staging"}
	contextLogger := logger.WithContext(ctx)

	// Act
	contextLogger.Warn("warning message", "retry_count", 3)

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture one message")

	msg := messages[0]
	assert.Equal(t, "WARN", msg.Level)
	assert.Equal(t, "warning message", msg.Msg)

	// Verify context fields are prepended
	assert.Contains(t, msg.Fields, "service", "Context field key should be present")
	assert.Contains(t, msg.Fields, "api", "Context field value should be present")
	assert.Contains(t, msg.Fields, "environment", "Context field key should be present")
	assert.Contains(t, msg.Fields, "staging", "Context field value should be present")
	assert.Contains(t, msg.Fields, "retry_count", "Message field key should be present")
	assert.Contains(t, msg.Fields, 3, "Message field value should be present")

	// Verify total field count (4 context fields + 2 message fields = 6)
	assert.Len(t, msg.Fields, 6, "Should have exactly 6 fields")
}

func TestMockLogger_T003_Error_MergesCtxFieldsWithMessageFields(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{"correlation_id": "corr-789", "hostname": "server-01"}
	contextLogger := logger.WithContext(ctx)

	// Act
	contextLogger.Error("error message", "error_code", "ERR_500", "details", "connection timeout")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture one message")

	msg := messages[0]
	assert.Equal(t, "ERROR", msg.Level)
	assert.Equal(t, "error message", msg.Msg)

	// Verify context fields are prepended
	assert.Contains(t, msg.Fields, "correlation_id", "Context field key should be present")
	assert.Contains(t, msg.Fields, "corr-789", "Context field value should be present")
	assert.Contains(t, msg.Fields, "hostname", "Context field key should be present")
	assert.Contains(t, msg.Fields, "server-01", "Context field value should be present")
	assert.Contains(t, msg.Fields, "error_code", "Message field key should be present")
	assert.Contains(t, msg.Fields, "ERR_500", "Message field value should be present")
	assert.Contains(t, msg.Fields, "details", "Message field key should be present")
	assert.Contains(t, msg.Fields, "connection timeout", "Message field value should be present")

	// Verify total field count (4 context fields + 4 message fields = 8)
	assert.Len(t, msg.Fields, 8, "Should have exactly 8 fields")
}

func TestMockLogger_T003_AllLevels_WithoutContextFields(t *testing.T) {
	// Arrange - logger WITHOUT context
	logger := testutil.NewMockLogger()

	// Act - log at all levels without context
	logger.Debug("debug no ctx", "d_key", "d_val")
	logger.Info("info no ctx", "i_key", "i_val")
	logger.Warn("warn no ctx", "w_key", "w_val")
	logger.Error("error no ctx", "e_key", "e_val")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 4, "Should capture 4 messages")

	// Verify each message has ONLY message fields (no context fields)
	for i, msg := range messages {
		assert.Len(t, msg.Fields, 2, "Message %d should have exactly 2 fields (no context)", i)
	}

	// Verify Debug
	assert.Equal(t, "DEBUG", messages[0].Level)
	assert.Contains(t, messages[0].Fields, "d_key")
	assert.Contains(t, messages[0].Fields, "d_val")

	// Verify Info
	assert.Equal(t, "INFO", messages[1].Level)
	assert.Contains(t, messages[1].Fields, "i_key")
	assert.Contains(t, messages[1].Fields, "i_val")

	// Verify Warn
	assert.Equal(t, "WARN", messages[2].Level)
	assert.Contains(t, messages[2].Fields, "w_key")
	assert.Contains(t, messages[2].Fields, "w_val")

	// Verify Error
	assert.Equal(t, "ERROR", messages[3].Level)
	assert.Contains(t, messages[3].Fields, "e_key")
	assert.Contains(t, messages[3].Fields, "e_val")
}

func TestMockLogger_T003_EdgeCase_EmptyContextEmptyFields(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	emptyCtx := map[string]any{}
	contextLogger := logger.WithContext(emptyCtx)

	// Act - log with no message fields
	contextLogger.Debug("debug empty")
	contextLogger.Info("info empty")
	contextLogger.Warn("warn empty")
	contextLogger.Error("error empty")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 4, "Should capture 4 messages")

	// All messages should have empty Fields
	for i, msg := range messages {
		assert.Empty(t, msg.Fields, "Message %d should have empty fields", i)
	}
}

func TestMockLogger_T003_EdgeCase_OnlyContextFieldsNoMessageFields(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{"ctx_only": "value"}
	contextLogger := logger.WithContext(ctx)

	// Act - log without message fields
	contextLogger.Debug("debug ctx only")
	contextLogger.Info("info ctx only")
	contextLogger.Warn("warn ctx only")
	contextLogger.Error("error ctx only")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 4, "Should capture 4 messages")

	// All messages should have ONLY context fields
	for i, msg := range messages {
		assert.Len(t, msg.Fields, 2, "Message %d should have exactly 2 fields (context only)", i)
		assert.Contains(t, msg.Fields, "ctx_only", "Context field key should be present in message %d", i)
		assert.Contains(t, msg.Fields, "value", "Context field value should be present in message %d", i)
	}
}

func TestMockLogger_T003_EdgeCase_ChainedContextMerging(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx1 := map[string]any{"level1": "a"}
	ctx2 := map[string]any{"level2": "b"}

	// Act - chain contexts and log at all levels
	logger1 := logger.WithContext(ctx1)
	logger2 := logger1.WithContext(ctx2)

	logger2.Debug("chained debug", "msg", "d")
	logger2.Info("chained info", "msg", "i")
	logger2.Warn("chained warn", "msg", "w")
	logger2.Error("chained error", "msg", "e")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 4, "Should capture 4 messages")

	// All messages should have both context levels plus message fields
	for i, msg := range messages {
		assert.Len(t, msg.Fields, 6, "Message %d should have 6 fields (2 ctx1 + 2 ctx2 + 2 msg)", i)
		assert.Contains(t, msg.Fields, "level1", "First context should be present in message %d", i)
		assert.Contains(t, msg.Fields, "a", "First context value should be present in message %d", i)
		assert.Contains(t, msg.Fields, "level2", "Second context should be present in message %d", i)
		assert.Contains(t, msg.Fields, "b", "Second context value should be present in message %d", i)
		assert.Contains(t, msg.Fields, "msg", "Message field key should be present in message %d", i)
	}
}

func TestMockLogger_T003_EdgeCase_LargeNumberOfFields(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{
		"ctx1": "val1",
		"ctx2": "val2",
		"ctx3": "val3",
	}
	contextLogger := logger.WithContext(ctx)

	// Act - log with many message fields
	messageFields := []any{
		"field1", 1,
		"field2", 2,
		"field3", 3,
		"field4", 4,
		"field5", 5,
	}

	contextLogger.Debug("large fields debug", messageFields...)
	contextLogger.Info("large fields info", messageFields...)
	contextLogger.Warn("large fields warn", messageFields...)
	contextLogger.Error("large fields error", messageFields...)

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 4, "Should capture 4 messages")

	// Each message should have 6 context fields + 10 message fields = 16 total
	for i, msg := range messages {
		assert.Len(t, msg.Fields, 16, "Message %d should have 16 fields (6 ctx + 10 msg)", i)

		// Verify context fields are present
		assert.Contains(t, msg.Fields, "ctx1", "Context field should be present in message %d", i)
		assert.Contains(t, msg.Fields, "ctx2", "Context field should be present in message %d", i)
		assert.Contains(t, msg.Fields, "ctx3", "Context field should be present in message %d", i)

		// Verify message fields are present
		assert.Contains(t, msg.Fields, "field1", "Message field should be present in message %d", i)
		assert.Contains(t, msg.Fields, "field5", "Message field should be present in message %d", i)
	}
}

func TestMockLogger_T003_FieldOrderingPreservation(t *testing.T) {
	// Arrange
	logger := testutil.NewMockLogger()
	ctx := map[string]any{"ctx_key": "ctx_val"}
	contextLogger := logger.WithContext(ctx)

	// Act
	contextLogger.Debug("test", "msg_key", "msg_val")

	// Assert
	messages := logger.GetMessages()
	require.Len(t, messages, 1, "Should capture one message")

	msg := messages[0]
	// Fields should be: [ctx_key, ctx_val, msg_key, msg_val]
	assert.Len(t, msg.Fields, 4, "Should have exactly 4 fields")

	// Find indices to verify ordering
	ctxKeyIdx := -1
	msgKeyIdx := -1
	for i, field := range msg.Fields {
		if field == "ctx_key" {
			ctxKeyIdx = i
		}
		if field == "msg_key" {
			msgKeyIdx = i
		}
	}

	assert.NotEqual(t, -1, ctxKeyIdx, "Context key should be found")
	assert.NotEqual(t, -1, msgKeyIdx, "Message key should be found")
	assert.Less(t, ctxKeyIdx, msgKeyIdx, "Context fields must appear before message fields")
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

// =============================================================================
// MockExpressionValidator Tests
// =============================================================================

// Feature: C021 Domain Purity Violation Fix
// Component: T006 MockExpressionValidator Tests

func TestMockExpressionValidator_NewMockExpressionValidator(t *testing.T) {
	// Arrange & Act
	validator := &testutil.MockExpressionValidator{}

	// Assert
	require.NotNil(t, validator, "MockExpressionValidator should be non-nil")

	// Verify it's usable immediately with default behavior
	err := validator.Compile("valid.expression")
	assert.NoError(t, err, "Compile on new validator should return nil by default")

	err = validator.Compile("")
	assert.NoError(t, err, "Compile with empty string should return nil by default")
}

func TestMockExpressionValidator_Compile_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.MockExpressionValidator)
		expression string
		wantErr    bool
	}{
		{
			name:       "default behavior returns nil",
			setupFunc:  func(v *testutil.MockExpressionValidator) {},
			expression: "user.name == 'John'",
			wantErr:    false,
		},
		{
			name:       "empty expression with default behavior",
			setupFunc:  func(v *testutil.MockExpressionValidator) {},
			expression: "",
			wantErr:    false,
		},
		{
			name:       "whitespace expression with default behavior",
			setupFunc:  func(v *testutil.MockExpressionValidator) {},
			expression: "   ",
			wantErr:    false,
		},
		{
			name:       "complex expression with default behavior",
			setupFunc:  func(v *testutil.MockExpressionValidator) {},
			expression: `user.age > 18 && user.status in ["active", "pending"]`,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			validator := &testutil.MockExpressionValidator{}
			tt.setupFunc(validator)

			// Act
			err := validator.Compile(tt.expression)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMockExpressionValidator_Compile_ErrorInjection(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.MockExpressionValidator)
		expression string
		wantErr    string
	}{
		{
			name: "SetCompileError injects error",
			setupFunc: func(v *testutil.MockExpressionValidator) {
				v.SetCompileError(errors.New("syntax error"))
			},
			expression: "user.name",
			wantErr:    "syntax error",
		},
		{
			name: "SetCompileError with empty expression",
			setupFunc: func(v *testutil.MockExpressionValidator) {
				v.SetCompileError(errors.New("empty expression"))
			},
			expression: "",
			wantErr:    "empty expression",
		},
		{
			name: "SetCompileFunc with custom error",
			setupFunc: func(v *testutil.MockExpressionValidator) {
				v.SetCompileFunc(func(expr string) error {
					if expr == "" {
						return errors.New("expression cannot be empty")
					}
					return nil
				})
			},
			expression: "",
			wantErr:    "expression cannot be empty",
		},
		{
			name: "SetCompileFunc with expression-specific validation",
			setupFunc: func(v *testutil.MockExpressionValidator) {
				v.SetCompileFunc(func(expr string) error {
					if len(expr) > 100 {
						return errors.New("expression too long")
					}
					return nil
				})
			},
			expression: string(make([]byte, 101)),
			wantErr:    "expression too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			validator := &testutil.MockExpressionValidator{}
			tt.setupFunc(validator)

			// Act
			err := validator.Compile(tt.expression)

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestMockExpressionValidator_SetCompileFunc_CustomBehavior(t *testing.T) {
	tests := []struct {
		name        string
		compileFunc func(string) error
		expression  string
		wantErr     bool
	}{
		{
			name: "custom validation logic - reject invalid prefix",
			compileFunc: func(expr string) error {
				if expr != "" && expr[0] == '!' {
					return errors.New("expression cannot start with '!'")
				}
				return nil
			},
			expression: "!invalid",
			wantErr:    true,
		},
		{
			name: "custom validation logic - accept valid prefix",
			compileFunc: func(expr string) error {
				if expr != "" && expr[0] == '!' {
					return errors.New("expression cannot start with '!'")
				}
				return nil
			},
			expression: "valid.expression",
			wantErr:    false,
		},
		{
			name: "always succeed function",
			compileFunc: func(expr string) error {
				return nil
			},
			expression: "anything",
			wantErr:    false,
		},
		{
			name: "always fail function",
			compileFunc: func(expr string) error {
				return errors.New("always fails")
			},
			expression: "anything",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			validator := &testutil.MockExpressionValidator{}
			validator.SetCompileFunc(tt.compileFunc)

			// Act
			err := validator.Compile(tt.expression)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMockExpressionValidator_SetCompileError_OverridesFunc(t *testing.T) {
	// Arrange
	validator := &testutil.MockExpressionValidator{}
	validator.SetCompileFunc(func(expr string) error {
		return errors.New("from func")
	})

	// Act - SetCompileError should override the func
	validator.SetCompileError(errors.New("from error"))
	err := validator.Compile("test")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "from error", err.Error(), "SetCompileError should override SetCompileFunc")
}

func TestMockExpressionValidator_SetCompileFunc_OverridesError(t *testing.T) {
	// Arrange
	validator := &testutil.MockExpressionValidator{}
	validator.SetCompileError(errors.New("from error"))

	// Act - SetCompileFunc should override the error
	validator.SetCompileFunc(func(expr string) error {
		return errors.New("from func")
	})
	err := validator.Compile("test")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "from func", err.Error(), "SetCompileFunc should override SetCompileError")
}

func TestMockExpressionValidator_Clear(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*testutil.MockExpressionValidator)
	}{
		{
			name: "clear after SetCompileError",
			setupFunc: func(v *testutil.MockExpressionValidator) {
				v.SetCompileError(errors.New("test error"))
			},
		},
		{
			name: "clear after SetCompileFunc",
			setupFunc: func(v *testutil.MockExpressionValidator) {
				v.SetCompileFunc(func(expr string) error {
					return errors.New("test error")
				})
			},
		},
		{
			name:      "clear on fresh validator",
			setupFunc: func(v *testutil.MockExpressionValidator) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			validator := &testutil.MockExpressionValidator{}
			tt.setupFunc(validator)

			// Act
			validator.Clear()
			err := validator.Compile("test")

			// Assert
			assert.NoError(t, err, "Clear should reset to default behavior (return nil)")
		})
	}
}

func TestMockExpressionValidator_ConcurrentAccess(t *testing.T) {
	// Arrange
	validator := &testutil.MockExpressionValidator{}
	const goroutines = 10
	const iterations = 100

	// Act - concurrent Compile calls
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Readers
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				expr := fmt.Sprintf("expr_%d_%d", id, j)
				_ = validator.Compile(expr)
			}
		}(i)
	}

	// Writers (configuration changes)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if j%2 == 0 {
					validator.SetCompileError(fmt.Errorf("error_%d_%d", id, j))
				} else {
					validator.SetCompileFunc(func(expr string) error {
						return nil
					})
				}
			}
		}(i)
	}

	// Assert - should complete without race conditions
	wg.Wait()
	assert.True(t, true, "Concurrent access should not cause races")
}

func TestMockExpressionValidator_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.MockExpressionValidator)
		expression string
		wantErr    bool
	}{
		{
			name:       "empty string",
			setupFunc:  func(v *testutil.MockExpressionValidator) {},
			expression: "",
			wantErr:    false,
		},
		{
			name:       "single character",
			setupFunc:  func(v *testutil.MockExpressionValidator) {},
			expression: "x",
			wantErr:    false,
		},
		{
			name:       "very long expression",
			setupFunc:  func(v *testutil.MockExpressionValidator) {},
			expression: string(make([]byte, 10000)),
			wantErr:    false,
		},
		{
			name:       "unicode characters",
			setupFunc:  func(v *testutil.MockExpressionValidator) {},
			expression: "用户.名称 == '张三'",
			wantErr:    false,
		},
		{
			name:       "special characters",
			setupFunc:  func(v *testutil.MockExpressionValidator) {},
			expression: `!@#$%^&*()_+-=[]{}|;:'",.<>?/`,
			wantErr:    false,
		},
		{
			name: "nil error from SetCompileError",
			setupFunc: func(v *testutil.MockExpressionValidator) {
				v.SetCompileError(nil)
			},
			expression: "test",
			wantErr:    false,
		},
		{
			name: "nil function from SetCompileFunc",
			setupFunc: func(v *testutil.MockExpressionValidator) {
				v.SetCompileFunc(nil)
			},
			expression: "test",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			validator := &testutil.MockExpressionValidator{}
			tt.setupFunc(validator)

			// Act
			err := validator.Compile(tt.expression)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMockExpressionValidator_RealWorldExpressions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{
			name:       "simple comparison",
			expression: "user.age > 18",
		},
		{
			name:       "logical AND",
			expression: "user.active && user.verified",
		},
		{
			name:       "logical OR",
			expression: "user.role == 'admin' || user.role == 'moderator'",
		},
		{
			name:       "in operator",
			expression: `user.status in ["active", "pending"]`,
		},
		{
			name:       "complex nested",
			expression: `(user.age >= 18 && user.country == "US") || user.role == "admin"`,
		},
		{
			name:       "string concatenation",
			expression: `user.firstName + " " + user.lastName`,
		},
		{
			name:       "method call",
			expression: "len(user.permissions) > 0",
		},
		{
			name:       "ternary-like",
			expression: "user.premium ? 100 : 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			validator := &testutil.MockExpressionValidator{}

			// Act - default behavior should accept all
			err := validator.Compile(tt.expression)

			// Assert
			assert.NoError(t, err, "Real-world expressions should pass with default behavior")
		})
	}
}

func TestMockExpressionValidator_ClearMultipleTimes(t *testing.T) {
	// Arrange
	validator := &testutil.MockExpressionValidator{}

	// Act & Assert - multiple Clear calls should be safe
	validator.Clear()
	err := validator.Compile("test1")
	assert.NoError(t, err)

	validator.SetCompileError(errors.New("error"))
	validator.Clear()
	err = validator.Compile("test2")
	assert.NoError(t, err)

	validator.Clear()
	validator.Clear()
	err = validator.Compile("test3")
	assert.NoError(t, err, "Multiple Clear calls should be idempotent")
}

func TestMockExpressionValidator_StateIsolation(t *testing.T) {
	// Arrange
	validator1 := &testutil.MockExpressionValidator{}
	validator2 := &testutil.MockExpressionValidator{}

	// Act - configure each differently
	validator1.SetCompileError(errors.New("error1"))
	validator2.SetCompileError(errors.New("error2"))

	err1 := validator1.Compile("test")
	err2 := validator2.Compile("test")

	// Assert - state should be isolated
	require.Error(t, err1)
	require.Error(t, err2)
	assert.Equal(t, "error1", err1.Error())
	assert.Equal(t, "error2", err2.Error())
}
