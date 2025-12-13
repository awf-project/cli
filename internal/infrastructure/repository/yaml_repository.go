package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// YAMLRepository implements WorkflowRepository for YAML files.
type YAMLRepository struct {
	basePath string
}

// NewYAMLRepository creates a new YAML repository.
func NewYAMLRepository(basePath string) *YAMLRepository {
	return &YAMLRepository{basePath: basePath}
}

// Load reads and parses a workflow from a YAML file.
func (r *YAMLRepository) Load(ctx context.Context, name string) (*workflow.Workflow, error) {
	filePath := r.resolvePath(name)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // workflow not found
		}
		return nil, WrapParseError(filePath, err)
	}

	// Parse YAML
	var yamlWf yamlWorkflow
	if unmarshalErr := yaml.Unmarshal(data, &yamlWf); unmarshalErr != nil {
		return nil, WrapParseError(filePath, unmarshalErr)
	}

	// Parse states (custom handling for inline steps)
	if parseErr := r.parseStates(data, &yamlWf); parseErr != nil {
		return nil, WrapParseError(filePath, parseErr)
	}

	// Validate required fields
	if yamlWf.Name == "" {
		return nil, NewParseError(filePath, "name", "required field missing")
	}
	if yamlWf.States.Initial == "" {
		return nil, NewParseError(filePath, "states.initial", "required field missing")
	}

	// Map to domain
	wf, err := mapToDomain(filePath, &yamlWf)
	if err != nil {
		return nil, err
	}

	// Domain validation
	if err := wf.Validate(); err != nil {
		return nil, NewParseError(filePath, "", err.Error())
	}

	return wf, nil
}

// List returns all workflow names in the repository.
func (r *YAMLRepository) List(ctx context.Context) ([]string, error) {
	pattern := filepath.Join(r.basePath, "*.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
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
	return false, err
}

// resolvePath converts workflow name to file path.
func (r *YAMLRepository) resolvePath(name string) string {
	if !strings.HasSuffix(name, ".yaml") {
		name = name + ".yaml"
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
		return err
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
