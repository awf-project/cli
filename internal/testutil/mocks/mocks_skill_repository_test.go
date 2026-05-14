package mocks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/workflow"
)

func TestMockSkillRepository_Load_HappyPath(t *testing.T) {
	repo := NewMockSkillRepository()
	skill := &workflow.Skill{
		Name:     "helpers",
		Content:  "helper functions...",
		Location: "/path/to/helpers",
	}

	repo.SetSkill("helpers", skill)

	ctx := context.Background()
	loaded, err := repo.Load(ctx, "helpers")

	require.NoError(t, err)
	assert.NotNil(t, loaded)
	assert.Equal(t, "helpers", loaded.Name)
	assert.Equal(t, "helper functions...", loaded.Content)
}

func TestMockSkillRepository_Load_SkillNotFound(t *testing.T) {
	repo := NewMockSkillRepository()

	ctx := context.Background()
	loaded, err := repo.Load(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, loaded)
	assert.Contains(t, err.Error(), "skill not found")
}

func TestMockSkillRepository_LoadFromPath_HappyPath(t *testing.T) {
	repo := NewMockSkillRepository()
	skill := &workflow.Skill{
		Name:     "custom-skill",
		Content:  "custom skill content...",
		Location: "/absolute/path/to/skill",
	}

	repo.SetSkill("/absolute/path/to/skill", skill)

	ctx := context.Background()
	loaded, err := repo.LoadFromPath(ctx, "/absolute/path/to/skill")

	require.NoError(t, err)
	assert.NotNil(t, loaded)
	assert.Equal(t, "custom-skill", loaded.Name)
	assert.Equal(t, "/absolute/path/to/skill", loaded.Location)
}

func TestMockSkillRepository_LoadFromPath_NotFound(t *testing.T) {
	repo := NewMockSkillRepository()

	ctx := context.Background()
	loaded, err := repo.LoadFromPath(ctx, "/nonexistent/path")

	assert.Error(t, err)
	assert.Nil(t, loaded)
	assert.Contains(t, err.Error(), "skill not found at path")
}

func TestMockSkillRepository_SetSkill_MultipleSkills(t *testing.T) {
	repo := NewMockSkillRepository()

	skill1 := &workflow.Skill{Name: "skill1", Content: "content1"}
	skill2 := &workflow.Skill{Name: "skill2", Content: "content2"}
	skill3 := &workflow.Skill{Name: "skill3", Content: "content3"}

	repo.SetSkill("skill1", skill1)
	repo.SetSkill("skill2", skill2)
	repo.SetSkill("skill3", skill3)

	ctx := context.Background()

	loaded1, err1 := repo.Load(ctx, "skill1")
	assert.NoError(t, err1)
	assert.Equal(t, "skill1", loaded1.Name)

	loaded2, err2 := repo.Load(ctx, "skill2")
	assert.NoError(t, err2)
	assert.Equal(t, "skill2", loaded2.Name)

	loaded3, err3 := repo.Load(ctx, "skill3")
	assert.NoError(t, err3)
	assert.Equal(t, "skill3", loaded3.Name)
}

func TestMockSkillRepository_SetLoadError_Load(t *testing.T) {
	repo := NewMockSkillRepository()
	skill := &workflow.Skill{Name: "skill1", Content: "content"}
	repo.SetSkill("skill1", skill)

	testErr := domainerrors.NewUserError(
		domainerrors.ErrorCodeUserInputMissingSkill,
		"custom error",
		nil,
		nil,
	)
	repo.SetLoadError(testErr)

	ctx := context.Background()
	loaded, err := repo.Load(ctx, "skill1")

	assert.Error(t, err)
	assert.Nil(t, loaded)
	assert.Equal(t, testErr, err)
}

func TestMockSkillRepository_SetLoadError_LoadFromPath(t *testing.T) {
	repo := NewMockSkillRepository()
	skill := &workflow.Skill{Name: "skill1", Content: "content"}
	repo.SetSkill("/path/to/skill", skill)

	testErr := domainerrors.NewUserError(
		domainerrors.ErrorCodeUserInputMissingSkill,
		"custom error",
		nil,
		nil,
	)
	repo.SetLoadError(testErr)

	ctx := context.Background()
	loaded, err := repo.LoadFromPath(ctx, "/path/to/skill")

	assert.Error(t, err)
	assert.Nil(t, loaded)
	assert.Equal(t, testErr, err)
}

func TestMockSkillRepository_ThreadSafe_Load(t *testing.T) {
	repo := NewMockSkillRepository()

	skill := &workflow.Skill{
		Name:    "concurrent-skill",
		Content: "test content",
	}
	repo.SetSkill("concurrent-skill", skill)

	ctx := context.Background()

	results := make(chan *workflow.Skill, 10)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			loaded, err := repo.Load(ctx, "concurrent-skill")
			if err != nil {
				errors <- err
			} else {
				results <- loaded
			}
		}()
	}

	for i := 0; i < 10; i++ {
		select {
		case loaded := <-results:
			assert.NotNil(t, loaded)
			assert.Equal(t, "concurrent-skill", loaded.Name)
		case err := <-errors:
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestMockSkillRepository_ThreadSafe_SetSkill(t *testing.T) {
	repo := NewMockSkillRepository()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		go func(index int) {
			skillName := "skill" + string(rune('0'+index)) //nolint:gosec // index is 0-9, safe conversion
			skill := &workflow.Skill{
				Name:    skillName,
				Content: "content",
			}
			repo.SetSkill(skillName, skill)
		}(i)
	}

	for i := 0; i < 10; i++ {
		skillName := "skill" + string(rune('0'+i))
		loaded, err := repo.Load(ctx, skillName)
		if loaded != nil {
			assert.NoError(t, err)
			assert.Equal(t, skillName, loaded.Name)
		}
	}
}

func TestMockSkillRepository_SetSkill_Overwrite(t *testing.T) {
	repo := NewMockSkillRepository()

	skill1 := &workflow.Skill{Name: "skill", Content: "original"}
	repo.SetSkill("skill", skill1)

	skill2 := &workflow.Skill{Name: "skill", Content: "updated"}
	repo.SetSkill("skill", skill2)

	ctx := context.Background()
	loaded, err := repo.Load(ctx, "skill")

	require.NoError(t, err)
	assert.Equal(t, "updated", loaded.Content)
}

func TestMockSkillRepository_Implements_Interface(t *testing.T) {
	repo := NewMockSkillRepository()

	ctx := context.Background()
	_, _ = repo.Load(ctx, "test")
	_, _ = repo.LoadFromPath(ctx, "/path/to/skill")

	repo.SetSkill("test", &workflow.Skill{})
	repo.SetLoadError(nil)
}
