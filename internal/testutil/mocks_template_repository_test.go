package testutil_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/testutil"
)

// Feature: C044 Fix Test Layer Purity Violations in Template Service Tests
// Component: T004 MockTemplateRepository Tests

// Compile-time assertion verifies that MockTemplateRepository satisfies the
// ports.TemplateRepository interface. If the mock fails to implement the
// interface, the code will not compile, catching interface mismatches early.
var _ ports.TemplateRepository = (*testutil.MockTemplateRepository)(nil)

// Feature: C044 Fix Test Layer Purity Violations in Template Service Tests
// Component: T004 MockTemplateRepository

func TestMockTemplateRepository_NewMockTemplateRepository(t *testing.T) {
	repo := testutil.NewMockTemplateRepository()

	require.NotNil(t, repo, "NewMockTemplateRepository should return non-nil instance")

	// Verify it's usable immediately
	ctx := context.Background()

	// Test GetTemplate on empty repository
	tpl, err := repo.GetTemplate(ctx, "nonexistent")
	assert.Error(t, err, "GetTemplate on empty repository should return error")
	assert.Nil(t, tpl, "GetTemplate on empty repository should return nil template")
	var templateNotFoundErr *workflow.TemplateNotFoundError
	assert.ErrorAs(t, err, &templateNotFoundErr, "Error should be TemplateNotFoundError")
	assert.Equal(t, "nonexistent", templateNotFoundErr.TemplateName)

	// Test ListTemplates on empty repository
	names, err := repo.ListTemplates(ctx)
	assert.NoError(t, err, "ListTemplates on empty repository should not error")
	assert.Empty(t, names, "ListTemplates on empty repository should return empty slice")

	// Test Exists on empty repository
	exists := repo.Exists(ctx, "nonexistent")
	assert.False(t, exists, "Exists on empty repository should return false")
}

func TestMockTemplateRepository_GetTemplate_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(*testutil.MockTemplateRepository)
		templateName string
		want         *workflow.Template
		wantErr      bool
	}{
		{
			name: "get existing template with parameters",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("test-tpl", &workflow.Template{
					Name: "test-tpl",
					Parameters: []workflow.TemplateParam{
						{Name: "param1", Required: true},
						{Name: "param2", Required: false, Default: "default-value"},
					},
					States: map[string]*workflow.Step{
						"start": {Name: "start", Type: workflow.StepTypeCommand},
					},
				})
			},
			templateName: "test-tpl",
			want: &workflow.Template{
				Name: "test-tpl",
				Parameters: []workflow.TemplateParam{
					{Name: "param1", Required: true},
					{Name: "param2", Required: false, Default: "default-value"},
				},
				States: map[string]*workflow.Step{
					"start": {Name: "start", Type: workflow.StepTypeCommand},
				},
			},
			wantErr: false,
		},
		{
			name: "get existing template without parameters",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("simple-tpl", &workflow.Template{
					Name: "simple-tpl",
					States: map[string]*workflow.Step{
						"start": {Name: "start", Type: workflow.StepTypeCommand},
					},
				})
			},
			templateName: "simple-tpl",
			want: &workflow.Template{
				Name: "simple-tpl",
				States: map[string]*workflow.Step{
					"start": {Name: "start", Type: workflow.StepTypeCommand},
				},
			},
			wantErr: false,
		},
		{
			name: "get template with empty name",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("", &workflow.Template{
					Name: "",
					States: map[string]*workflow.Step{
						"start": {Name: "start", Type: workflow.StepTypeCommand},
					},
				})
			},
			templateName: "",
			want: &workflow.Template{
				Name: "",
				States: map[string]*workflow.Step{
					"start": {Name: "start", Type: workflow.StepTypeCommand},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutil.NewMockTemplateRepository()
			tt.setupFunc(repo)
			ctx := context.Background()

			got, err := repo.GetTemplate(ctx, tt.templateName)

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
				assert.Equal(t, len(tt.want.Parameters), len(got.Parameters))
				for i, param := range tt.want.Parameters {
					assert.Equal(t, param.Name, got.Parameters[i].Name)
					assert.Equal(t, param.Required, got.Parameters[i].Required)
					assert.Equal(t, param.Default, got.Parameters[i].Default)
				}
				assert.Equal(t, len(tt.want.States), len(got.States))
			}
		})
	}
}

func TestMockTemplateRepository_GetTemplate_ErrorHandling(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(*testutil.MockTemplateRepository)
		templateName string
		wantErr      error
		checkErr     func(*testing.T, error)
	}{
		{
			name: "get nonexistent template returns TemplateNotFoundError",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("other-tpl", &workflow.Template{Name: "other-tpl"})
			},
			templateName: "nonexistent",
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				var templateNotFoundErr *workflow.TemplateNotFoundError
				assert.ErrorAs(t, err, &templateNotFoundErr, "Error should be TemplateNotFoundError")
				assert.Equal(t, "nonexistent", templateNotFoundErr.TemplateName)
			},
		},
		{
			name: "get from empty repository returns TemplateNotFoundError",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				// No templates added
			},
			templateName: "any",
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				var templateNotFoundErr *workflow.TemplateNotFoundError
				assert.ErrorAs(t, err, &templateNotFoundErr, "Error should be TemplateNotFoundError")
				assert.Equal(t, "any", templateNotFoundErr.TemplateName)
			},
		},
		{
			name: "get with configured error",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("test", &workflow.Template{Name: "test"})
				repo.SetGetError(errors.New("get failed"))
			},
			templateName: "test",
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Equal(t, "get failed", err.Error())
			},
		},
		{
			name: "configured error takes precedence over TemplateNotFoundError",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				// No template added, but error configured
				repo.SetGetError(errors.New("database error"))
			},
			templateName: "nonexistent",
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Equal(t, "database error", err.Error())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutil.NewMockTemplateRepository()
			tt.setupFunc(repo)
			ctx := context.Background()

			got, err := repo.GetTemplate(ctx, tt.templateName)

			assert.Nil(t, got, "GetTemplate should return nil on error")
			tt.checkErr(t, err)
		})
	}
}

func TestMockTemplateRepository_GetTemplate_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*testutil.MockTemplateRepository)
		wantErr   bool
	}{
		{
			name: "get template with unicode name",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("템플릿-日本語-🚀", &workflow.Template{
					Name: "템플릿-日本語-🚀",
					States: map[string]*workflow.Step{
						"start": {Name: "start", Type: workflow.StepTypeCommand},
					},
				})
			},
			wantErr: false,
		},
		{
			name: "get template with very long name",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				longName := "very-long-template-name-" + string(make([]byte, 1000))
				repo.AddTemplate(longName, &workflow.Template{
					Name: longName,
					States: map[string]*workflow.Step{
						"start": {Name: "start", Type: workflow.StepTypeCommand},
					},
				})
			},
			wantErr: false,
		},
		{
			name: "get template with special characters in name",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				specialName := "tpl-@#$%^&*()_+-=[]{}|;:',.<>?/~`"
				repo.AddTemplate(specialName, &workflow.Template{
					Name: specialName,
					States: map[string]*workflow.Step{
						"start": {Name: "start", Type: workflow.StepTypeCommand},
					},
				})
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutil.NewMockTemplateRepository()
			tt.setupFunc(repo)
			ctx := context.Background()

			names, _ := repo.ListTemplates(ctx)
			require.NotEmpty(t, names)
			got, err := repo.GetTemplate(ctx, names[0])

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestMockTemplateRepository_ListTemplates_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*testutil.MockTemplateRepository)
		want      []string
		wantErr   bool
	}{
		{
			name: "list multiple templates",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("tpl1", &workflow.Template{Name: "tpl1"})
				repo.AddTemplate("tpl2", &workflow.Template{Name: "tpl2"})
				repo.AddTemplate("tpl3", &workflow.Template{Name: "tpl3"})
			},
			want:    []string{"tpl1", "tpl2", "tpl3"},
			wantErr: false,
		},
		{
			name: "list single template",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("only-one", &workflow.Template{Name: "only-one"})
			},
			want:    []string{"only-one"},
			wantErr: false,
		},
		{
			name: "list from empty repository",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				// No templates added
			},
			want:    []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutil.NewMockTemplateRepository()
			tt.setupFunc(repo)
			ctx := context.Background()

			got, err := repo.ListTemplates(ctx)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.ElementsMatch(t, tt.want, got, "ListTemplates should return all template names")
		})
	}
}

func TestMockTemplateRepository_ListTemplates_ErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*testutil.MockTemplateRepository)
		wantErr   error
	}{
		{
			name: "list with configured error",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("tpl1", &workflow.Template{Name: "tpl1"})
				repo.SetListError(errors.New("list failed"))
			},
			wantErr: errors.New("list failed"),
		},
		{
			name: "list error on empty repository",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.SetListError(errors.New("database unavailable"))
			},
			wantErr: errors.New("database unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutil.NewMockTemplateRepository()
			tt.setupFunc(repo)
			ctx := context.Background()

			got, err := repo.ListTemplates(ctx)

			assert.Error(t, err)
			assert.Equal(t, tt.wantErr.Error(), err.Error())
			assert.Nil(t, got, "ListTemplates should return nil on error")
		})
	}
}

func TestMockTemplateRepository_Exists_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(*testutil.MockTemplateRepository)
		templateName string
		want         bool
	}{
		{
			name: "exists returns true for existing template",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("test-tpl", &workflow.Template{Name: "test-tpl"})
			},
			templateName: "test-tpl",
			want:         true,
		},
		{
			name: "exists returns false for nonexistent template",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("other-tpl", &workflow.Template{Name: "other-tpl"})
			},
			templateName: "nonexistent",
			want:         false,
		},
		{
			name: "exists returns false on empty repository",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				// No templates added
			},
			templateName: "any",
			want:         false,
		},
		{
			name: "exists returns true for empty name if added",
			setupFunc: func(repo *testutil.MockTemplateRepository) {
				repo.AddTemplate("", &workflow.Template{Name: ""})
			},
			templateName: "",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutil.NewMockTemplateRepository()
			tt.setupFunc(repo)
			ctx := context.Background()

			got := repo.Exists(ctx, tt.templateName)

			assert.Equal(t, tt.want, got, "Exists should return correct boolean")
		})
	}
}

func TestMockTemplateRepository_AddTemplate_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		template     *workflow.Template
	}{
		{
			name:         "add template with parameters",
			templateName: "test-tpl",
			template: &workflow.Template{
				Name: "test-tpl",
				Parameters: []workflow.TemplateParam{
					{Name: "param1", Required: true},
				},
				States: map[string]*workflow.Step{
					"start": {Name: "start", Type: workflow.StepTypeCommand},
				},
			},
		},
		{
			name:         "add template without parameters",
			templateName: "simple-tpl",
			template: &workflow.Template{
				Name: "simple-tpl",
				States: map[string]*workflow.Step{
					"start": {Name: "start", Type: workflow.StepTypeCommand},
				},
			},
		},
		{
			name:         "add template with empty name",
			templateName: "",
			template: &workflow.Template{
				Name: "",
				States: map[string]*workflow.Step{
					"start": {Name: "start", Type: workflow.StepTypeCommand},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutil.NewMockTemplateRepository()
			ctx := context.Background()

			repo.AddTemplate(tt.templateName, tt.template)

			got, err := repo.GetTemplate(ctx, tt.templateName)
			assert.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.template.Name, got.Name)
		})
	}
}

func TestMockTemplateRepository_AddTemplate_Update(t *testing.T) {
	repo := testutil.NewMockTemplateRepository()
	ctx := context.Background()

	original := &workflow.Template{
		Name: "test-tpl",
		Parameters: []workflow.TemplateParam{
			{Name: "param1", Required: true},
		},
	}

	updated := &workflow.Template{
		Name: "test-tpl",
		Parameters: []workflow.TemplateParam{
			{Name: "param1", Required: true},
			{Name: "param2", Required: false},
		},
	}

	repo.AddTemplate("test-tpl", original)
	gotOriginal, err := repo.GetTemplate(ctx, "test-tpl")
	assert.NoError(t, err)
	assert.Len(t, gotOriginal.Parameters, 1)

	repo.AddTemplate("test-tpl", updated)
	gotUpdated, err := repo.GetTemplate(ctx, "test-tpl")

	assert.NoError(t, err)
	require.NotNil(t, gotUpdated)
	assert.Len(t, gotUpdated.Parameters, 2, "AddTemplate should update existing template")
}

func TestMockTemplateRepository_Clear_HappyPath(t *testing.T) {
	repo := testutil.NewMockTemplateRepository()
	ctx := context.Background()

	repo.AddTemplate("tpl1", &workflow.Template{Name: "tpl1"})
	repo.AddTemplate("tpl2", &workflow.Template{Name: "tpl2"})
	repo.SetGetError(errors.New("test error"))
	repo.SetListError(errors.New("test error"))

	// Verify templates exist before clear
	_, err := repo.ListTemplates(ctx)
	assert.Error(t, err) // Error is set

	repo.Clear()

	names, err := repo.ListTemplates(ctx)
	assert.NoError(t, err, "Clear should reset errors")
	assert.Empty(t, names, "Clear should remove all templates")

	assert.False(t, repo.Exists(ctx, "tpl1"))
	assert.False(t, repo.Exists(ctx, "tpl2"))

	tpl, err := repo.GetTemplate(ctx, "tpl1")
	assert.Error(t, err)
	assert.Nil(t, tpl)
	var templateNotFoundErr *workflow.TemplateNotFoundError
	assert.ErrorAs(t, err, &templateNotFoundErr)
}

func TestMockTemplateRepository_SetGetError_HappyPath(t *testing.T) {
	repo := testutil.NewMockTemplateRepository()
	ctx := context.Background()

	repo.AddTemplate("test", &workflow.Template{Name: "test"})
	customErr := errors.New("custom get error")

	repo.SetGetError(customErr)

	_, err := repo.GetTemplate(ctx, "test")
	assert.Error(t, err)
	assert.Equal(t, customErr.Error(), err.Error())
}

func TestMockTemplateRepository_SetListError_HappyPath(t *testing.T) {
	repo := testutil.NewMockTemplateRepository()
	ctx := context.Background()

	repo.AddTemplate("test", &workflow.Template{Name: "test"})
	customErr := errors.New("custom list error")

	repo.SetListError(customErr)

	_, err := repo.ListTemplates(ctx)
	assert.Error(t, err)
	assert.Equal(t, customErr.Error(), err.Error())
}

func TestMockTemplateRepository_ThreadSafety(t *testing.T) {
	// This test verifies thread-safe concurrent access to the mock repository.
	// Feature: C044 requires thread-safe mock implementations.

	repo := testutil.NewMockTemplateRepository()
	ctx := context.Background()

	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // 3 operations per goroutine

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				name := "tpl-" + string(rune('A'+i))
				repo.AddTemplate(name, &workflow.Template{Name: name})
			}
		}()
	}

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				name := "tpl-" + string(rune('A'+i))
				_, _ = repo.GetTemplate(ctx, name)
			}
		}()
	}

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = repo.ListTemplates(ctx)
				name := "tpl-" + string(rune('A'+i))
				_ = repo.Exists(ctx, name)
			}
		}()
	}

	wg.Wait()

	// Verify final state is consistent
	names, err := repo.ListTemplates(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, names, "Should have templates after concurrent operations")
}

func TestMockTemplateRepository_ConcurrentErrorConfiguration(t *testing.T) {
	// This test verifies thread-safe error configuration.
	// Feature: C044 requires thread-safe error configuration.

	repo := testutil.NewMockTemplateRepository()
	ctx := context.Background()

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			repo.SetGetError(errors.New("get error"))
			repo.SetGetError(nil)
		}()

		go func() {
			defer wg.Done()
			repo.SetListError(errors.New("list error"))
			repo.SetListError(nil)
		}()
	}

	wg.Wait()

	// Verify final state
	_, err := repo.ListTemplates(ctx)
	assert.NoError(t, err, "Errors should be nil after reset")
}

func TestMockTemplateRepository_ContextCancellation(t *testing.T) {
	// This test verifies behavior with cancelled context.
	// The mock should ignore context cancellation and always execute.

	repo := testutil.NewMockTemplateRepository()
	repo.AddTemplate("test", &workflow.Template{Name: "test"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// (Mocks typically ignore context cancellation for simplicity)
	tpl, err := repo.GetTemplate(ctx, "test")
	assert.NoError(t, err)
	assert.NotNil(t, tpl)

	names, err := repo.ListTemplates(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, names)

	exists := repo.Exists(ctx, "test")
	assert.True(t, exists)
}
