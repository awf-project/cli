package workflowpkg

import (
	"context"
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/repository"
)

// PackDiscovererAdapter implements ports.PackDiscoverer using PackLoader.
// It scans multiple directories in priority order, deduplicating by pack name.
// The first directory in the list wins when the same pack name appears in multiple directories.
type PackDiscovererAdapter struct {
	loader *PackLoader
	dirs   []string
}

// NewPackDiscovererAdapter creates a PackDiscovererAdapter scanning the given
// directories in priority order (first directory wins for duplicate pack names).
func NewPackDiscovererAdapter(dirs []string) *PackDiscovererAdapter {
	return &PackDiscovererAdapter{
		loader: NewPackLoader(),
		dirs:   dirs,
	}
}

// DiscoverWorkflows returns WorkflowEntry items for all enabled packs found
// across the configured directories. Disabled packs and packs without a
// readable manifest are silently skipped. The returned slice may be nil when
// no enabled packs are found.
func (a *PackDiscovererAdapter) DiscoverWorkflows(ctx context.Context) ([]workflow.WorkflowEntry, error) {
	// First pass: collect unique pack directories, first directory wins.
	packMap := make(map[string]string) // pack name -> absolute pack directory
	for _, dir := range a.dirs {
		packs, err := a.loader.DiscoverPacks(ctx, dir)
		if err != nil {
			// Non-fatal: a missing or unreadable directory should not prevent
			// discovery from other configured directories.
			continue
		}
		for _, p := range packs {
			// Defense-in-depth: skip pack names that fail the name regex even if
			// the loader's Validate already rejects them. This prevents path
			// traversal through a crafted pack name reaching filepath.Join.
			if !nameRegex.MatchString(p.Name) {
				continue
			}
			if _, seen := packMap[p.Name]; !seen {
				packMap[p.Name] = filepath.Join(dir, p.Name)
			}
		}
	}

	// Second pass: build WorkflowEntry values for each enabled pack.
	var entries []workflow.WorkflowEntry
	for packName, packDir := range packMap {
		state, err := a.loader.LoadPackState(packDir)
		if err != nil || !state.Enabled {
			continue
		}

		manifestData, err := readFileLimited(filepath.Join(packDir, "manifest.yaml"), MaxManifestSize)
		if err != nil {
			continue
		}
		manifest, err := ParseManifest(manifestData)
		if err != nil {
			continue
		}

		for _, wfName := range manifest.Workflows {
			// Defense-in-depth: skip workflow names that fail the name regex.
			// Manifest.Validate already enforces this, but the second ParseManifest
			// call (without Validate) in this path makes a defensive check necessary.
			if !nameRegex.MatchString(wfName) {
				continue
			}
			entries = append(entries, workflow.WorkflowEntry{
				Name:        packName + "/" + wfName,
				Source:      "pack",
				Scope:       packName,
				Workflow:    wfName,
				Version:     manifest.Version,
				Description: loadWorkflowDescription(packDir, wfName),
			})
		}
	}

	return entries, nil
}

// LoadWorkflow loads a single workflow from an installed pack by pack name and
// workflow name. It searches configured directories in priority order.
func (a *PackDiscovererAdapter) LoadWorkflow(ctx context.Context, packName, workflowName string) (*workflow.Workflow, error) {
	for _, dir := range a.dirs {
		packDir := filepath.Join(dir, packName)
		workflowsDir := filepath.Join(packDir, "workflows")

		repo := repository.NewYAMLRepository(workflowsDir)
		wf, err := repo.Load(ctx, workflowName)
		if err != nil {
			continue
		}
		return wf, nil
	}
	return nil, fmt.Errorf("workflow %q not found in pack %q", workflowName, packName)
}

// loadWorkflowDescription reads the description field from a workflow YAML file.
// Returns an empty string if the file cannot be read or does not contain a description.
func loadWorkflowDescription(packDir, workflowName string) string {
	// Defense-in-depth: reject workflow names that would escape the workflows/
	// subdirectory. nameRegex already enforces this at the call site, but guard
	// here too since loadWorkflowDescription is a package-internal helper that
	// could be called directly.
	if !nameRegex.MatchString(workflowName) {
		return ""
	}
	data, err := readFileLimited(filepath.Join(packDir, "workflows", workflowName+".yaml"), 1<<20)
	if err != nil {
		return ""
	}
	var wf struct {
		Description string `yaml:"description"`
	}
	if yaml.Unmarshal(data, &wf) != nil {
		return ""
	}
	return wf.Description
}
