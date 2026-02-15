package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/awf-project/awf/internal/domain/workflow"
)

// YAMLTemplateRepository loads templates from YAML files.
type YAMLTemplateRepository struct {
	searchPaths []string
	cache       map[string]*workflow.Template
	mu          sync.RWMutex
}

// NewYAMLTemplateRepository creates a template repository.
func NewYAMLTemplateRepository(searchPaths []string) *YAMLTemplateRepository {
	return &YAMLTemplateRepository{
		searchPaths: searchPaths,
		cache:       make(map[string]*workflow.Template),
	}
}

// GetTemplate loads a template by name.
func (r *YAMLTemplateRepository) GetTemplate(_ context.Context, name string) (*workflow.Template, error) {
	// Check cache first
	r.mu.RLock()
	if t, ok := r.cache[name]; ok {
		r.mu.RUnlock()
		return t, nil
	}
	r.mu.RUnlock()

	// Search for template file
	for _, dir := range r.searchPaths {
		path := filepath.Join(dir, name+".yaml")
		if _, err := os.Stat(path); err == nil {
			t, err := r.loadTemplate(path)
			if err != nil {
				return nil, err
			}
			// Cache it
			r.mu.Lock()
			r.cache[name] = t
			r.mu.Unlock()
			return t, nil
		}
	}

	return nil, &TemplateNotFoundError{TemplateName: name}
}

// ListTemplates returns all available template names.
func (r *YAMLTemplateRepository) ListTemplates(_ context.Context) ([]string, error) {
	var names []string
	seen := make(map[string]bool)

	for _, dir := range r.searchPaths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // skip non-existent directories
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".yaml")
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names, nil
}

// Exists checks if a template exists.
func (r *YAMLTemplateRepository) Exists(_ context.Context, name string) bool {
	for _, dir := range r.searchPaths {
		path := filepath.Join(dir, name+".yaml")
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func (r *YAMLTemplateRepository) loadTemplate(path string) (*workflow.Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, WrapParseError(path, err).ToStructuredError()
	}

	var yt yamlTemplate
	if err := yaml.Unmarshal(data, &yt); err != nil {
		return nil, WrapParseError(path, err).ToStructuredError()
	}

	// Parse states section (same as workflow, has initial + inline steps)
	if err := r.parseStates(data, &yt); err != nil {
		return nil, WrapParseError(path, err).ToStructuredError()
	}

	return mapTemplate(path, &yt)
}

// parseStates parses the states section with inline step definitions.
// YAML structure:
//
//	states:
//	  initial: echo
//	  echo:
//	    type: command
//	    command: echo hello
func (r *YAMLTemplateRepository) parseStates(data []byte, t *yamlTemplate) error {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling YAML: %w", err)
	}

	statesRaw, ok := raw["states"].(map[string]any)
	if !ok {
		return nil // no states section
	}

	t.States.Steps = make(map[string]yamlStep)
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

		t.States.Steps[key] = step
	}

	if len(parseErrors) > 0 {
		return errors.Join(parseErrors...)
	}

	return nil
}
