package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkill_StructFields(t *testing.T) {
	skill := Skill{
		Name:      "example-skill",
		Content:   "# Example\n\nSkill content here",
		Location:  "/path/to/skill",
		Resources: []string{"file1.txt", "file2.json"},
	}

	assert.Equal(t, "example-skill", skill.Name)
	assert.Equal(t, "# Example\n\nSkill content here", skill.Content)
	assert.Equal(t, "/path/to/skill", skill.Location)
	assert.Len(t, skill.Resources, 2)
	assert.Contains(t, skill.Resources, "file1.txt")
	assert.Contains(t, skill.Resources, "file2.json")
}

func TestSkill_EmptyResourcesList(t *testing.T) {
	skill := Skill{
		Name:      "minimal-skill",
		Content:   "Minimal content",
		Location:  "/path/to/minimal",
		Resources: []string{},
	}

	assert.Empty(t, skill.Resources)
}

func TestSkillReferenceIsPathBased(t *testing.T) {
	tests := []struct {
		name      string
		reference SkillReference
		want      bool
	}{
		{
			name: "name only - discovery based",
			reference: SkillReference{
				Name: "my-skill",
				Path: "",
			},
			want: false,
		},
		{
			name: "path set - explicit path",
			reference: SkillReference{
				Name: "",
				Path: "/absolute/path/to/skill",
			},
			want: true,
		},
		{
			name: "both name and path set - path takes precedence",
			reference: SkillReference{
				Name: "my-skill",
				Path: "/path/to/skill",
			},
			want: true,
		},
		{
			name: "zero-value reference - neither set",
			reference: SkillReference{
				Name: "",
				Path: "",
			},
			want: false,
		},
		{
			name: "path with relative path",
			reference: SkillReference{
				Name: "",
				Path: "./relative/path/skill",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.reference.IsPathBased()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSkillReferenceFields(t *testing.T) {
	ref := SkillReference{
		Name: "test-skill",
		Path: "/test/path",
	}

	assert.Equal(t, "test-skill", ref.Name)
	assert.Equal(t, "/test/path", ref.Path)
}
