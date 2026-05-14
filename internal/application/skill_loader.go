package application

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// FormatSkillContent wraps skill content in an agentskills.io <skill_content> XML block.
func FormatSkillContent(skill *workflow.Skill) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "<skill_content name=%q>\n", skill.Name)
	sb.WriteString(skill.Content)
	fmt.Fprintf(&sb, "\n\nSkill directory: %s\nRelative paths in this skill are relative to the skill directory.", skill.Location)
	if len(skill.Resources) > 0 {
		sb.WriteString("\n\n<skill_resources>\n")
		for _, r := range skill.Resources {
			fmt.Fprintf(&sb, "<file>%s</file>\n", r)
		}
		sb.WriteString("</skill_resources>")
	}
	sb.WriteString("\n</skill_content>")
	return sb.String()
}

// ResolveAndFormatSkills resolves each SkillReference via repo and returns concatenated formatted content.
func ResolveAndFormatSkills(
	ctx context.Context,
	repo ports.SkillRepository,
	refs []workflow.SkillReference,
	workflowDir string,
) (string, error) {
	if len(refs) == 0 {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		skill, err := resolveSkillRef(ctx, repo, ref, workflowDir)
		if err != nil {
			return "", err
		}
		parts = append(parts, FormatSkillContent(skill))
	}
	return strings.Join(parts, "\n\n"), nil
}

func resolveSkillRef(ctx context.Context, repo ports.SkillRepository, ref workflow.SkillReference, workflowDir string) (*workflow.Skill, error) {
	if !ref.IsPathBased() {
		return repo.Load(ctx, ref.Name)
	}
	path := filepath.Clean(ref.Path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(workflowDir, path)
	}
	return repo.LoadFromPath(ctx, path)
}
