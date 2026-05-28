package roles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/skills"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
)

type FilesystemAgentRoleRepository struct {
	searchPaths []string
	logger      ports.Logger
}

func NewFilesystemAgentRoleRepository(logger ports.Logger) *FilesystemAgentRoleRepository {
	var paths []string

	if envPath := os.Getenv("AWF_ROLES_PATH"); envPath != "" {
		paths = filepath.SplitList(envPath)
	} else {
		// Two namespaces are searched, mirroring the skills repository:
		//  - AWF-native:   .awf/roles (local), $XDG_CONFIG_HOME/awf/roles (global)
		//  - cross-client: .agents/roles (local), ~/.agents/roles (global)
		// The cross-client paths let roles authored for other agent tooling be
		// reused without duplication.
		candidates := []string{
			xdg.LocalRolesDir(),
			".agents/roles",
			xdg.AWFRolesDir(),
			crossClientGlobalRolesDir(),
		}
		for _, p := range candidates {
			if p != "" {
				paths = append(paths, p)
			}
		}
	}

	return &FilesystemAgentRoleRepository{
		searchPaths: paths,
		logger:      logger,
	}
}

func (r *FilesystemAgentRoleRepository) Load(_ context.Context, name string) (*workflow.AgentRole, error) {
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid agent role name %q: must not contain path separators or '..'", name)
	}

	for _, searchPath := range r.searchPaths {
		dirPath := filepath.Join(searchPath, name)
		agentsFile := filepath.Join(dirPath, "AGENTS.md")

		if _, err := os.Stat(agentsFile); err != nil {
			continue
		}

		return r.loadAgentsMD(dirPath)
	}

	return nil, &workflow.AgentRoleNotFoundError{Name: name, SearchPaths: r.searchPaths}
}

func (r *FilesystemAgentRoleRepository) LoadFromPath(_ context.Context, absolutePath string) (*workflow.AgentRole, error) {
	cleanPath := filepath.Clean(absolutePath)

	var agentsFile, dirPath string
	if strings.HasSuffix(cleanPath, "AGENTS.md") {
		agentsFile = cleanPath
		dirPath = filepath.Dir(cleanPath)
	} else {
		agentsFile = filepath.Join(cleanPath, "AGENTS.md")
		dirPath = cleanPath
	}

	if _, err := os.Stat(agentsFile); err != nil {
		return nil, &workflow.AgentRoleNotFoundError{Name: filepath.Base(cleanPath), SearchPaths: []string{cleanPath}, IsPathRef: true}
	}

	return r.loadAgentsMD(dirPath)
}

func (r *FilesystemAgentRoleRepository) loadAgentsMD(dirPath string) (*workflow.AgentRole, error) {
	agentsFile := filepath.Join(dirPath, "AGENTS.md")

	data, err := os.ReadFile(agentsFile)
	if err != nil {
		return nil, fmt.Errorf("reading AGENTS.md in %s: %w", dirPath, err)
	}

	if r.logger != nil && len(data) > workflow.AgentRoleSizeWarnBytes {
		r.logger.Warn(
			fmt.Sprintf("AGENTS.md exceeds %dKB, may impact context window", workflow.AgentRoleSizeWarnBytes/1024),
			"path", agentsFile, "size", len(data),
		)
	}

	content := skills.StripFrontmatter(string(data))

	absPath, err := filepath.Abs(agentsFile)
	if err != nil {
		return nil, fmt.Errorf("resolving absolute path for %s: %w", agentsFile, err)
	}

	return &workflow.AgentRole{
		Name:         filepath.Base(dirPath),
		Content:      content,
		SourcePath:   absPath,
		RawSizeBytes: int64(len(data)),
	}, nil
}

func crossClientGlobalRolesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".agents", "roles")
}
