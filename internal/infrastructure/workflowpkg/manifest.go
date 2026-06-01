package workflowpkg

import (
	"fmt"
	"os"
	"path/filepath"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/pkg/registry"
	"github.com/awf-project/cli/pkg/validation"
	"gopkg.in/yaml.v3"
)

// Manifest is the parsed content of a workflow pack's manifest.yaml file.
type Manifest struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description"`
	Author      string            `yaml:"author"`
	License     string            `yaml:"license"`
	AWFVersion  string            `yaml:"awf_version"`
	Workflows   []string          `yaml:"workflows"`
	Plugins     map[string]string `yaml:"plugins"`
}

// ParseManifest parses manifest.yaml bytes into a Manifest.
func ParseManifest(data []byte) (*Manifest, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("manifest: empty data")
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}

	return &m, nil
}

// Validate checks manifest fields against spec rules:
//   - name matches ^[a-z][a-z0-9-]*$
//   - version is valid semver
//   - awf_version is a valid semver constraint
//   - every entry in workflows has a corresponding .yaml file in packDir/workflows/
func (m *Manifest) Validate(packDir string) error {
	if err := validation.ValidateName(m.Name); err != nil {
		return fmt.Errorf("manifest: invalid pack name: %w", err)
	}

	if m.Name == "local" || m.Name == "global" || m.Name == "env" {
		return domerrors.NewUserError(
			domerrors.ErrorCodeUserInputValidationFailed,
			fmt.Sprintf("pack name %q is reserved as a scope sentinel; rename the pack", m.Name),
			map[string]any{"pack_name": m.Name, "reserved_tokens": []string{"local", "global", "env"}},
			nil,
		)
	}

	if _, err := registry.ParseVersion(m.Version); err != nil {
		return fmt.Errorf("manifest: invalid version %q: %w", m.Version, err)
	}

	if _, err := registry.ParseConstraints(m.AWFVersion); err != nil {
		return fmt.Errorf("manifest: invalid awf_version constraint %q: %w", m.AWFVersion, err)
	}

	if len(m.Workflows) == 0 {
		return fmt.Errorf("manifest: workflows list is empty")
	}

	workflowsDir := filepath.Join(packDir, "workflows")
	if _, err := os.Stat(workflowsDir); err != nil {
		return fmt.Errorf("manifest: workflows directory does not exist: %w", err)
	}

	for _, workflow := range m.Workflows {
		if err := validation.ValidateName(workflow); err != nil {
			return fmt.Errorf("manifest: invalid workflow name: %w", err)
		}
		workflowFile := filepath.Join(workflowsDir, workflow+".yaml")
		if _, err := os.Stat(workflowFile); err != nil {
			return fmt.Errorf("manifest: workflow file %q not found", workflow+".yaml")
		}
	}

	return nil
}
