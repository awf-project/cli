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
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/expression"
)

// YAMLRepository implements WorkflowRepository for YAML files.
type YAMLRepository struct {
	basePath string
}

func NewYAMLRepository(basePath string) *YAMLRepository {
	return &YAMLRepository{basePath: basePath}
}

// Load reads and parses a workflow from a YAML file.
func (r *YAMLRepository) Load(ctx context.Context, name string) (*workflow.Workflow, error) {
	filePath := r.resolvePath(name)

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

// Exists checks if a workflow file exists.
func (r *YAMLRepository) Exists(ctx context.Context, name string) (bool, error) {
	filePath := r.resolvePath(name)
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("checking workflow file: %w", err)
}

// resolvePath converts workflow name to file path.
func (r *YAMLRepository) resolvePath(name string) string {
	if !strings.HasSuffix(name, ".yaml") {
		name += ".yaml"
	}
	return filepath.Join(r.basePath, name)
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
