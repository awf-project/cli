package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/expression"
)

// YAMLRepository implements WorkflowRepository for YAML files.
// When used standalone (outside a CompositeRepository) it reports all workflows
// as SourceLocal — the caller may override this via WithSource if the repository
// is known to represent a different discovery origin.
type YAMLRepository struct {
	basePath      string
	defaultSource Source
}

func NewYAMLRepository(basePath string) *YAMLRepository {
	return &YAMLRepository{basePath: basePath, defaultSource: SourceLocal}
}

// WithSource returns a shallow copy of the repository configured to report the
// given Source in ListWithSource results. This is used by CompositeRepository
// to avoid creating separate YAMLRepository types per source.
func (r *YAMLRepository) WithSource(s Source) *YAMLRepository {
	return &YAMLRepository{basePath: r.basePath, defaultSource: s}
}

// Load reads and parses a workflow from a YAML file.
func (r *YAMLRepository) Load(ctx context.Context, name string) (*workflow.Workflow, error) {
	filePath, ok := r.resolvePath(name)
	if !ok {
		return nil, domerrors.NewUserError(
			domerrors.ErrorCodeUserInputMissingFile,
			fmt.Sprintf("workflow not found: %s", name),
			map[string]any{"name": name},
			nil,
		)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domerrors.NewUserError(
				domerrors.ErrorCodeUserInputMissingFile,
				fmt.Sprintf("workflow file not found: %s", name),
				map[string]any{"path": filePath},
				err,
			)
		}
		// File read error (not a YAML syntax error, but use SYSTEM.IO.READ_FAILED instead)
		return nil, domerrors.NewSystemError(
			domerrors.ErrorCodeSystemIOReadFailed,
			fmt.Sprintf("failed to read workflow file: %s", filePath),
			map[string]any{"path": filePath},
			err,
		)
	}

	// Parse YAML
	var yamlWf yamlWorkflow
	if unmarshalErr := yaml.Unmarshal(data, &yamlWf); unmarshalErr != nil {
		return nil, WrapParseError(filePath, unmarshalErr).ToStructuredError()
	}

	// Parse states (custom handling for inline steps)
	if parseErr := r.parseStates(data, &yamlWf); parseErr != nil {
		return nil, WrapParseError(filePath, parseErr).ToStructuredError()
	}

	// Validate required fields
	if yamlWf.Name == "" {
		return nil, NewParseError(filePath, "name", "required field missing").ToStructuredError()
	}
	if yamlWf.States.Initial == "" {
		return nil, NewParseError(filePath, "states.initial", "required field missing").ToStructuredError()
	}

	// Map to domain
	wf, err := mapToDomain(filePath, &yamlWf)
	if err != nil {
		// Check if it's a ParseError and convert to StructuredError
		var pe *ParseError
		if errors.As(err, &pe) {
			return nil, pe.ToStructuredError()
		}
		return nil, err
	}

	// Domain validation
	validator := expression.NewExprValidator()
	if err := wf.Validate(validator.Compile, nil); err != nil {
		// Convert domain StateReferenceError to StructuredError
		var stateRefErr *workflow.StateReferenceError
		if errors.As(err, &stateRefErr) {
			availableAny := make([]any, len(stateRefErr.AvailableStates))
			for i, s := range stateRefErr.AvailableStates {
				availableAny[i] = s
			}
			return nil, domerrors.NewWorkflowError(
				domerrors.ErrorCodeWorkflowValidationMissingState,
				stateRefErr.Error(),
				map[string]any{
					"state":            stateRefErr.ReferencedState,
					"available_states": availableAny,
					"step":             stateRefErr.StepName,
					"field":            stateRefErr.Field,
				},
				err,
			)
		}

		// If the validation error already carries a StructuredError (e.g.
		// USER.MCP_PROXY.*), propagate it unchanged so the original domain
		// code reaches the formatter and YAMLSyntaxHintGenerator does not
		// fire spurious YAML-shape hints.  errors.As walks the full chain
		// including errors.Join multi-errors.
		var structErr *domerrors.StructuredError
		if errors.As(err, &structErr) {
			return nil, err
		}

		return nil, NewParseError(filePath, "", err.Error()).ToStructuredError()
	}

	return wf, nil
}

// List returns all workflow names in the repository.
func (r *YAMLRepository) List(ctx context.Context) ([]string, error) {
	pattern := filepath.Join(r.basePath, "*.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing workflow files: %w", err)
	}

	names := make([]string, 0, len(matches))
	for _, match := range matches {
		base := filepath.Base(match)
		name := strings.TrimSuffix(base, ".yaml")
		names = append(names, name)
	}
	return names, nil
}

// ListWithSource returns all workflow names together with their discovery source.
// Every entry carries the source configured on this repository (defaultSource),
// which is SourceLocal unless overridden via WithSource.
func (r *YAMLRepository) ListWithSource(ctx context.Context) ([]ports.WorkflowInfo, error) {
	names, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	infos := make([]ports.WorkflowInfo, 0, len(names))
	for _, name := range names {
		infos = append(infos, ports.WorkflowInfo{
			Name:   name,
			Source: ports.WorkflowSource(r.defaultSource.String()),
			Path:   filepath.Join(r.basePath, name+".yaml"),
		})
	}
	return infos, nil
}

// Exists checks if a workflow file exists.
func (r *YAMLRepository) Exists(ctx context.Context, name string) (bool, error) {
	filePath, ok := r.resolvePath(name)
	if !ok {
		return false, nil
	}
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("checking workflow file: %w", err)
}

// resolvePath converts a workflow name to an absolute-safe file path.
//
// It rejects names that resolve outside basePath after cleaning, which
// prevents path traversal attacks such as "../../etc/passwd" or absolute
// paths like "/etc/passwd" that filepath.Join would happily accept.
//
// Legitimate pack-qualified names ("speckit/specify") are allowed as long as
// the cleaned path remains inside basePath.
//
// Returns ("", false) when the name would escape basePath.
//
// # Security note — lexical guard only
//
// This guard is intentionally LEXICAL: it uses filepath.Clean for path
// normalisation but does NOT call filepath.EvalSymlinks. Symbolic links
// inside basePath are therefore NOT resolved, so a symlink that points
// outside basePath would pass this check.
//
// This is a deliberate design decision consistent with the rest of the codebase:
// callers that build basePath from trusted sources (XDG data directories,
// project-local .awf/ directories) accept this trade-off because resolving
// symlinks would break legitimate use cases such as development setups that
// symlink individual workflow files. The higher-level name validation in
// pkg/validation (ValidateName) and the manifest-list checks provide the
// first line of defense against untrusted names; this lexical check is the
// final backstop for names that somehow bypass earlier guards.
func (r *YAMLRepository) resolvePath(name string) (string, bool) {
	if !strings.HasSuffix(name, ".yaml") {
		name += ".yaml"
	}

	// filepath.Join cleans the path but does NOT block absolute components; an
	// absolute name silently replaces the base. Use filepath.Join then re-check.
	joined := filepath.Join(r.basePath, name)
	cleaned := filepath.Clean(joined)
	base := filepath.Clean(r.basePath)

	// The cleaned path must start with base + separator to remain inside base.
	// We also accept an exact match (base itself), though that would be unusual.
	if cleaned != base && !strings.HasPrefix(cleaned, base+string(filepath.Separator)) {
		return "", false
	}
	return cleaned, true
}

// parseStates parses the states section with inline step definitions.
// YAML structure:
//
//	states:
//	  initial: start
//	  start:
//	    type: step
//	    command: echo hello
func (r *YAMLRepository) parseStates(data []byte, wf *yamlWorkflow) error {
	// Parse into generic map to extract steps
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling YAML: %w", err)
	}

	statesRaw, ok := raw["states"].(map[string]any)
	if !ok {
		return nil // no states section
	}

	wf.States.Steps = make(map[string]yamlStep)
	var parseErrors []error

	for key, value := range statesRaw {
		if key == "initial" {
			continue // skip initial field
		}

		// Convert step value to yamlStep
		stepMap, ok := value.(map[string]any)
		if !ok {
			parseErrors = append(parseErrors, fmt.Errorf("state %q: expected map, got %T", key, value))
			continue
		}

		// Marshal back to YAML and unmarshal to yamlStep
		stepYAML, err := yaml.Marshal(stepMap)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("state %q: marshal error: %w", key, err))
			continue
		}

		var step yamlStep
		if err := yaml.Unmarshal(stepYAML, &step); err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("state %q: %w", key, err))
			continue
		}

		wf.States.Steps[key] = step
	}

	if len(parseErrors) > 0 {
		return errors.Join(parseErrors...)
	}

	return nil
}
