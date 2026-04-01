package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/awf-project/cli/internal/application"
	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
)

// parseWorkflowNamespace splits a workflow name into pack and workflow components.
// For "speckit/specify", returns ("speckit", "specify").
// For "my-workflow" (no slash), returns ("", "my-workflow").
// Splits on first "/" only per FR-001.
// Note: duplicated as splitCallWorkflowName in internal/application/subworkflow_executor.go
// — cross-layer import (application→interfaces) is forbidden by go-arch-lint.
func parseWorkflowNamespace(name string) (packName, workflowName string) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", name
}

// validatePackWorkflow checks that workflowName is listed as a public workflow in the pack manifest.
// packDir is the root directory of the installed pack (contains manifest.yaml).
// Returns USER.INPUT.VALIDATION_FAILED if the workflow is not listed, error if manifest is missing.
// Rejects path traversal patterns in workflowName before manifest lookup.
func validatePackWorkflow(packDir, workflowName string) error {
	// Reject path traversal patterns before reading filesystem
	if strings.Contains(workflowName, "..") {
		return domerrors.NewUserError(
			domerrors.ErrorCodeUserInputValidationFailed,
			fmt.Sprintf("%s: workflow name contains path traversal: %q", domerrors.ErrorCodeUserInputValidationFailed, workflowName),
			nil,
			nil,
		)
	}

	// Read manifest.yaml
	manifestPath := filepath.Join(packDir, "manifest.yaml")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	// Parse manifest
	manifest, err := workflowpkg.ParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Check if workflow is listed in manifest
	for _, wf := range manifest.Workflows {
		if wf == workflowName {
			return nil
		}
	}

	// Workflow not found in manifest
	return domerrors.NewUserError(
		domerrors.ErrorCodeUserInputValidationFailed,
		fmt.Sprintf("%s: workflow %q is not listed in pack manifest", domerrors.ErrorCodeUserInputValidationFailed, workflowName),
		nil,
		nil,
	)
}

// resolvePackDir finds the installed pack directory by searching local then global paths.
// Returns the absolute pack directory path or a structured error if not found.
// Rejects path traversal patterns in packName before filesystem access.
func resolvePackDir(packName, localPacksDir, globalPacksDir string) (string, error) {
	// Reject path traversal patterns before filesystem access
	if strings.Contains(packName, "..") {
		return "", domerrors.NewUserError(
			domerrors.ErrorCodeUserInputValidationFailed,
			fmt.Sprintf("%s: pack name contains path traversal: %q", domerrors.ErrorCodeUserInputValidationFailed, packName),
			nil,
			nil,
		)
	}

	// Try local directory first
	localPackPath := filepath.Join(localPacksDir, packName)
	if _, err := os.Stat(localPackPath); err == nil {
		absPath, err := filepath.Abs(localPackPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path for pack: %w", err)
		}
		return absPath, nil
	}

	// Fall back to global directory
	globalPackPath := filepath.Join(globalPacksDir, packName)
	if _, err := os.Stat(globalPackPath); err == nil {
		absPath, err := filepath.Abs(globalPackPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path for pack: %w", err)
		}
		return absPath, nil
	}

	// Pack not found in either location
	return "", domerrors.NewUserError(
		domerrors.ErrorCodeUserInputMissingFile,
		fmt.Sprintf("%s: pack %q not found in local or global directories", domerrors.ErrorCodeUserInputMissingFile, packName),
		nil,
		nil,
	)
}

// resolvePackWorkflow resolves and loads a workflow from an installed pack.
// Returns the loaded workflow, the resolved pack directory, or an error.
func resolvePackWorkflow(
	ctx context.Context,
	packName, workflowName string,
	localPacksDir, globalPacksDir string,
) (*workflow.Workflow, string, error) {
	packDir, err := resolvePackDir(packName, localPacksDir, globalPacksDir)
	if err != nil {
		return nil, "", err
	}

	if valErr := validatePackWorkflow(packDir, workflowName); valErr != nil {
		return nil, "", valErr
	}

	workflowsDir := filepath.Join(packDir, "workflows")
	repo := repository.NewYAMLRepository(workflowsDir)
	wf, err := repo.Load(ctx, workflowName)
	if err != nil {
		return nil, "", fmt.Errorf("load pack workflow: %w", err)
	}

	return wf, packDir, nil
}

// buildPackAWFPaths returns AWF paths with pack_name set for pack context.
// The prompts_dir and scripts_dir remain at global XDG paths — the 3-tier
// resolution in resolveLocalOverGlobal handles pack-embedded and user-override paths.
func buildPackAWFPaths(packName string) map[string]string {
	paths := buildAWFPaths()
	paths[application.AWFPackNameKey] = packName
	return paths
}
