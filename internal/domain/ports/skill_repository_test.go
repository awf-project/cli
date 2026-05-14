package ports_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSkillRepository struct {
	skills map[string]*workflow.Skill
	paths  map[string]*workflow.Skill
}

func newMockSkillRepository() *mockSkillRepository {
	return &mockSkillRepository{
		skills: make(map[string]*workflow.Skill),
		paths:  make(map[string]*workflow.Skill),
	}
}

func (m *mockSkillRepository) Load(ctx context.Context, name string) (*workflow.Skill, error) {
	if skill, ok := m.skills[name]; ok {
		return skill, nil
	}
	return nil, nil
}

func (m *mockSkillRepository) LoadFromPath(ctx context.Context, absolutePath string) (*workflow.Skill, error) {
	if skill, ok := m.paths[absolutePath]; ok {
		return skill, nil
	}
	return nil, nil
}

func TestSkillRepositoryInterface(t *testing.T) {
	var _ ports.SkillRepository = (*mockSkillRepository)(nil)
}

func TestSkillRepository_Load_ByName(t *testing.T) {
	repo := newMockSkillRepository()
	ctx := context.Background()

	skill := &workflow.Skill{
		Name:      "test-skill",
		Content:   "# Test Skill\n\nTest content",
		Location:  "/path/to/test-skill",
		Resources: []string{"readme.md"},
	}
	repo.skills["test-skill"] = skill

	loaded, err := repo.Load(ctx, "test-skill")

	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "test-skill", loaded.Name)
	assert.Equal(t, "# Test Skill\n\nTest content", loaded.Content)
	assert.Equal(t, "/path/to/test-skill", loaded.Location)
}

func TestSkillRepository_Load_NotFound(t *testing.T) {
	repo := newMockSkillRepository()
	ctx := context.Background()

	loaded, err := repo.Load(ctx, "nonexistent-skill")

	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestSkillRepository_LoadFromPath_Absolute(t *testing.T) {
	repo := newMockSkillRepository()
	ctx := context.Background()

	skill := &workflow.Skill{
		Name:      "explicit-skill",
		Content:   "Explicit content",
		Location:  "/absolute/path/to/skill",
		Resources: []string{"file1.txt", "file2.json"},
	}
	repo.paths["/absolute/path/to/skill"] = skill

	loaded, err := repo.LoadFromPath(ctx, "/absolute/path/to/skill")

	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "explicit-skill", loaded.Name)
	assert.Equal(t, "/absolute/path/to/skill", loaded.Location)
	assert.Len(t, loaded.Resources, 2)
}

func TestSkillRepository_LoadFromPath_NotFound(t *testing.T) {
	repo := newMockSkillRepository()
	ctx := context.Background()

	loaded, err := repo.LoadFromPath(ctx, "/nonexistent/path")

	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestSkillRepository_Load_MultipleSkills(t *testing.T) {
	repo := newMockSkillRepository()
	ctx := context.Background()

	skill1 := &workflow.Skill{
		Name:     "skill-one",
		Content:  "Content 1",
		Location: "/path/skill-one",
	}
	skill2 := &workflow.Skill{
		Name:     "skill-two",
		Content:  "Content 2",
		Location: "/path/skill-two",
	}

	repo.skills["skill-one"] = skill1
	repo.skills["skill-two"] = skill2

	loaded1, err1 := repo.Load(ctx, "skill-one")
	loaded2, err2 := repo.Load(ctx, "skill-two")

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, "skill-one", loaded1.Name)
	assert.Equal(t, "skill-two", loaded2.Name)
}

func TestSkillRepository_ContextPropagation(t *testing.T) {
	repo := newMockSkillRepository()
	ctx := context.Background()

	skill := &workflow.Skill{
		Name:     "ctx-test",
		Content:  "Test",
		Location: "/test",
	}
	repo.skills["ctx-test"] = skill

	loaded, err := repo.Load(ctx, "ctx-test")

	require.NoError(t, err)
	assert.NotNil(t, loaded)
}
