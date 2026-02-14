package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	domerrors "github.com/vanoix/awf/internal/domain/errors"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// SourcedPath represents a workflow directory with its source
type SourcedPath struct {
	Path   string
	Source Source
}

// CompositeRepository aggregates multiple YAMLRepository instances with priority
// Earlier paths take precedence over later ones for workflows with the same name
type CompositeRepository struct {
	paths []SourcedPath
	repos map[Source]*YAMLRepository
}

func NewCompositeRepository(paths []SourcedPath) *CompositeRepository {
	repos := make(map[Source]*YAMLRepository)
	for _, sp := range paths {
		repos[sp.Source] = NewYAMLRepository(sp.Path)
	}
	return &CompositeRepository{
		paths: paths,
		repos: repos,
	}
}

// Load finds and loads a workflow by name, checking paths in priority order
func (r *CompositeRepository) Load(ctx context.Context, name string) (*workflow.Workflow, error) {
	for _, sp := range r.paths {
		if !r.pathExists(sp.Path) {
			continue
		}
		repo := r.repos[sp.Source]
		wf, err := repo.Load(ctx, name)
		if err != nil {
			// Check if this is a "not found" error - if so, continue to next repo
			var se *domerrors.StructuredError
			if errors.As(err, &se) && se.Code == domerrors.ErrorCodeUserInputMissingFile {
				continue // not found in this repo, try next
			}
			return nil, err // real error, propagate
		}
		return wf, nil
	}
	// Not found in any repository
	return nil, domerrors.NewUserError(
		domerrors.ErrorCodeUserInputMissingFile,
		fmt.Sprintf("workflow not found: %s", name),
		map[string]any{"path": name},
		nil,
	)
}

// List returns unique workflow names from all sources
func (r *CompositeRepository) List(ctx context.Context) ([]string, error) {
	seen := make(map[string]bool)
	var names []string

	for _, sp := range r.paths {
		if !r.pathExists(sp.Path) {
			continue
		}
		repo := r.repos[sp.Source]
		repoNames, err := repo.List(ctx)
		if err != nil {
			continue // skip errors for individual repos
		}
		for _, name := range repoNames {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	return names, nil
}

// ListWithSource returns workflow info including source for each workflow
func (r *CompositeRepository) ListWithSource(ctx context.Context) ([]WorkflowInfo, error) {
	seen := make(map[string]bool)
	var infos []WorkflowInfo

	for _, sp := range r.paths {
		if !r.pathExists(sp.Path) {
			continue
		}
		repo := r.repos[sp.Source]
		repoNames, err := repo.List(ctx)
		if err != nil {
			continue
		}
		for _, name := range repoNames {
			if !seen[name] {
				seen[name] = true
				infos = append(infos, WorkflowInfo{
					Name:   name,
					Source: sp.Source,
					Path:   filepath.Join(sp.Path, name+".yaml"),
				})
			}
		}
	}

	return infos, nil
}

// Exists checks if a workflow exists in any source
func (r *CompositeRepository) Exists(ctx context.Context, name string) (bool, error) {
	for _, sp := range r.paths {
		if !r.pathExists(sp.Path) {
			continue
		}
		repo := r.repos[sp.Source]
		exists, err := repo.Exists(ctx, name)
		if err != nil {
			continue
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

// pathExists checks if a directory exists
func (r *CompositeRepository) pathExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
