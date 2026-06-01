package workflowpkg

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/pkg/validation"
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
			// Defense-in-depth: skip pack names that fail the shared name rule even if
			// the loader's Validate already rejects them. This prevents path
			// traversal through a crafted pack name reaching filepath.Join.
			if validation.ValidateName(p.Name) != nil {
				continue
			}
			if _, seen := packMap[p.Name]; !seen {
				packMap[p.Name] = filepath.Join(dir, p.Name)
			}
		}
	}

	// Second pass: build WorkflowEntry values for each enabled pack.
	// Sort pack names so the output order is deterministic across calls.
	// This matters for the ACP available_commands_update message: clients must
	// receive a stable list between reconnections.
	packNames := make([]string, 0, len(packMap))
	for k := range packMap {
		packNames = append(packNames, k)
	}
	sort.Strings(packNames)

	var entries []workflow.WorkflowEntry
	for _, packName := range packNames {
		packDir := packMap[packName]
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

		// Sort workflow names within the pack for deterministic output order.
		// This stabilizes the ACP available_commands_update message: clients
		// receive an identical list between reconnections regardless of manifest
		// declaration order or Go map iteration.
		sortedWorkflows := slices.Clone(manifest.Workflows)
		sort.Strings(sortedWorkflows)

		for _, wfName := range sortedWorkflows {
			// Defense-in-depth: skip workflow names that fail the shared name rule.
			// Manifest.Validate already enforces this, but the second ParseManifest
			// call (without Validate) in this path makes a defensive check necessary.
			if validation.ValidateName(wfName) != nil {
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
//
// Both packName and workflowName are validated with the shared ValidateName rule
// before any filepath.Join — this is the central choke-point that prevents
// path traversal for all GetWorkflow-by-pack callers.
func (a *PackDiscovererAdapter) LoadWorkflow(ctx context.Context, packName, workflowName string) (*workflow.Workflow, error) {
	// S1: validate names before any filesystem access. ValidateName rejects
	// "..", "/", uppercase, digits-first, and other invalid patterns, so
	// filepath.Join(dir, packName) can never escape dir for a valid packName.
	if err := validation.ValidateName(packName); err != nil {
		return nil, fmt.Errorf("pack name: %w", err)
	}
	if err := validation.ValidateName(workflowName); err != nil {
		return nil, fmt.Errorf("workflow name: %w", err)
	}

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
	// subdirectory. validation.ValidateName already enforces this at the call site,
	// but guard here too since loadWorkflowDescription is a package-internal helper
	// that could be called directly.
	if validation.ValidateName(workflowName) != nil {
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
