package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
)

// skillSizeWarnBytes is the SKILL.md size (in bytes) above which a
// context-window warning is emitted. Intentionally separate from
// workflow.AgentRoleSizeWarnBytes to allow independent evolution of the two
// thresholds without coupling the skills and roles domains.
const skillSizeWarnBytes = 500 * 1024

type FilesystemSkillRepository struct {
	searchPaths []string
	logger      ports.Logger
}

func NewFilesystemSkillRepository(logger ports.Logger) *FilesystemSkillRepository {
	var paths []string

	if envPath := os.Getenv("AWF_SKILLS_PATH"); envPath != "" {
		paths = filepath.SplitList(envPath)
	} else {
		candidates := []string{
			xdg.LocalSkillsDir(),
			".agents/skills",
			".claude/skills",
			xdg.AWFSkillsDir(),
			crossClientGlobalSkillsDir(),
			claudeGlobalSkillsDir(),
		}
		for _, p := range candidates {
			if p != "" {
				paths = append(paths, p)
			}
		}
	}

	return &FilesystemSkillRepository{
		searchPaths: paths,
		logger:      logger,
	}
}

func (r *FilesystemSkillRepository) Load(_ context.Context, name string) (*workflow.Skill, error) {
	// Reject both / and \ regardless of OS, and double-dot sequences, to prevent
	// path traversal. Using ContainsAny mirrors the roles repository for consistency.
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid skill name %q: must not contain path separators or '..'", name)
	}

	for _, searchPath := range r.searchPaths {
		dirPath := filepath.Join(searchPath, name)
		skillFile := filepath.Join(dirPath, "SKILL.md")

		if _, err := os.Stat(skillFile); err != nil {
			continue
		}

		return r.loadSkillFromDir(dirPath)
	}

	return nil, &workflow.SkillNotFoundError{Name: name, SearchPaths: r.searchPaths}
}

func (r *FilesystemSkillRepository) LoadFromPath(_ context.Context, absolutePath string) (*workflow.Skill, error) {
	cleanPath := filepath.Clean(absolutePath)
	skillFile := filepath.Join(cleanPath, "SKILL.md")

	if _, err := os.Stat(skillFile); err != nil {
		return nil, &workflow.SkillNotFoundError{Name: filepath.Base(cleanPath), SearchPaths: []string{cleanPath}}
	}

	return r.loadSkillFromDir(cleanPath)
}

func (r *FilesystemSkillRepository) loadSkillFromDir(dirPath string) (*workflow.Skill, error) {
	skillFile := filepath.Join(dirPath, "SKILL.md")

	data, err := os.ReadFile(skillFile)
	if err != nil {
		return nil, fmt.Errorf("reading SKILL.md in %s: %w", dirPath, err)
	}

	if r.logger != nil && len(data) > skillSizeWarnBytes {
		r.logger.Warn(
			fmt.Sprintf("SKILL.md exceeds %dKB, may impact context window", skillSizeWarnBytes/1024),
			"path", skillFile, "size", len(data),
		)
	}

	content := StripFrontmatter(string(data))

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("resolving absolute path for %s: %w", dirPath, err)
	}

	resources, err := r.enumerateResources(absPath)
	if err != nil {
		return nil, err
	}

	return &workflow.Skill{
		Name:      filepath.Base(dirPath),
		Content:   content,
		Location:  absPath,
		Resources: resources,
	}, nil
}

func (r *FilesystemSkillRepository) enumerateResources(dirPath string) ([]string, error) {
	var resources []string
	maxDepth := strings.Count(dirPath, string(filepath.Separator)) + 4

	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			if strings.Count(path, string(filepath.Separator)) > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(dirPath, path)
		if err != nil {
			return nil
		}
		if rel != "SKILL.md" {
			resources = append(resources, rel)
		}
		return nil
	})

	if err == nil {
		sort.Strings(resources)
	}
	return resources, err
}

func crossClientGlobalSkillsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".agents", "skills")
}

func claudeGlobalSkillsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "skills")
}
