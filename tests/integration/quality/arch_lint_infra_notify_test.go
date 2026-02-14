package quality_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestArchLintInfraNotify_ComponentRegistration(t *testing.T) {
	projectRoot := filepath.Join("..", "..", "..")
	configPath := filepath.Join(projectRoot, ".go-arch-lint.yml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err, "Failed to read .go-arch-lint.yml")

	var config struct {
		Components map[string]struct {
			In string `yaml:"in"`
		} `yaml:"components"`
		Deps map[string]struct {
			MayDependOn []string `yaml:"mayDependOn"`
			CanUse      []string `yaml:"canUse"`
		} `yaml:"deps"`
	}

	err = yaml.Unmarshal(data, &config)
	require.NoError(t, err, "Failed to parse .go-arch-lint.yml")

	t.Run("component_registered", func(t *testing.T) {
		component, exists := config.Components["infra-notify"]
		assert.True(t, exists, "infra-notify component should be registered in components section")
		assert.Equal(t, "infrastructure/notify", component.In,
			"infra-notify should point to infrastructure/notify directory")
	})

	t.Run("dependencies_configured", func(t *testing.T) {
		deps, exists := config.Deps["infra-notify"]
		require.True(t, exists, "infra-notify dependencies should be configured in deps section")

		expectedMayDependOn := []string{
			"domain-workflow",
			"domain-ports",
			"domain-errors",
			"domain-plugin",
			"infra-logger",
		}

		expectedCanUse := []string{
			"go-stdlib",
			"go-sync",
		}

		assert.ElementsMatch(t, expectedMayDependOn, deps.MayDependOn,
			"infra-notify should depend on domain layers, domain-plugin, and infra-logger")

		assert.ElementsMatch(t, expectedCanUse, deps.CanUse,
			"infra-notify should only use go-stdlib and go-sync (no external deps)")
	})

	t.Run("interfaces_cli_can_wire_infra_notify", func(t *testing.T) {
		cliDeps, exists := config.Deps["interfaces-cli"]
		require.True(t, exists, "interfaces-cli dependencies should be configured")

		assert.Contains(t, cliDeps.MayDependOn, "infra-notify",
			"interfaces-cli should be able to depend on infra-notify for wiring")
	})
}

func TestArchLintInfraNotify_ArchitectureValidation(t *testing.T) {
	if _, err := exec.LookPath("go-arch-lint"); err != nil {
		t.Skip("go-arch-lint not installed, skipping validation test")
	}

	projectRoot := filepath.Join("..", "..", "..")
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "go-arch-lint", "check")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "go-arch-lint check failed:\n%s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "OK - No warnings found",
		"Expected no architecture warnings for infra-notify component, got:\n%s", outputStr)

	assert.NotContains(t, outputStr, "infra-notify",
		"Should have no warnings mentioning infra-notify")
}

func TestArchLintInfraNotify_DependencyIsolation(t *testing.T) {
	projectRoot := filepath.Join("..", "..", "..")
	configPath := filepath.Join(projectRoot, ".go-arch-lint.yml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config struct {
		Deps map[string]struct {
			MayDependOn []string `yaml:"mayDependOn"`
		} `yaml:"deps"`
	}
	err = yaml.Unmarshal(data, &config)
	require.NoError(t, err)

	allowedDeps := config.Deps["infra-notify"].MayDependOn

	tests := []struct {
		name              string
		forbiddenDep      string
		reason            string
		shouldBeForbidden bool
	}{
		{
			name:              "cannot_depend_on_application_layer",
			forbiddenDep:      "application",
			reason:            "Infrastructure should not depend on application layer",
			shouldBeForbidden: true,
		},
		{
			name:              "cannot_depend_on_interfaces",
			forbiddenDep:      "interfaces-cli",
			reason:            "Infrastructure should not depend on interfaces layer",
			shouldBeForbidden: true,
		},
		{
			name:              "cannot_depend_on_other_infra",
			forbiddenDep:      "infra-repository",
			reason:            "Should not depend on unrelated infrastructure components",
			shouldBeForbidden: true,
		},
		{
			name:              "cannot_depend_on_infra_executor",
			forbiddenDep:      "infra-executor",
			reason:            "Should not depend on executor (architectural separation)",
			shouldBeForbidden: true,
		},
		{
			name:              "can_depend_on_domain_workflow",
			forbiddenDep:      "domain-workflow",
			reason:            "Should depend on domain-workflow for workflow context",
			shouldBeForbidden: false,
		},
		{
			name:              "can_depend_on_infra_logger",
			forbiddenDep:      "infra-logger",
			reason:            "Should depend on infra-logger for logging",
			shouldBeForbidden: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contains := slices.Contains(allowedDeps, tt.forbiddenDep)

			if tt.shouldBeForbidden {
				assert.False(t, contains,
					"%s: infra-notify should NOT depend on %s", tt.reason, tt.forbiddenDep)
			} else {
				assert.True(t, contains,
					"%s: infra-notify should depend on %s", tt.reason, tt.forbiddenDep)
			}
		})
	}
}

func TestArchLintInfraNotify_EdgeCases(t *testing.T) {
	projectRoot := filepath.Join("..", "..", "..")
	configPath := filepath.Join(projectRoot, ".go-arch-lint.yml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	t.Run("no_duplicate_component_definitions", func(t *testing.T) {
		count := strings.Count(string(data), "infra-notify:")
		assert.Equal(t, 2, count,
			"infra-notify should appear exactly twice (components + deps), found %d", count)
	})

	t.Run("no_typos_in_dependency_names", func(t *testing.T) {
		var config struct {
			Deps map[string]struct {
				MayDependOn []string `yaml:"mayDependOn"`
			} `yaml:"deps"`
		}
		err := yaml.Unmarshal(data, &config)
		require.NoError(t, err)

		deps := config.Deps["infra-notify"]

		validDeps := map[string]bool{
			"domain-workflow": true,
			"domain-ports":    true,
			"domain-errors":   true,
			"domain-plugin":   true,
			"infra-logger":    true,
		}

		for _, dep := range deps.MayDependOn {
			assert.True(t, validDeps[dep],
				"Unexpected/misspelled dependency: %s", dep)
		}
	})

	t.Run("yaml_properly_formatted", func(t *testing.T) {
		assert.NotContains(t, string(data), "\t",
			"YAML should use spaces for indentation, not tabs")

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(line, "infra-notify:") {
				trimmed := strings.TrimLeft(line, " ")
				indent := len(line) - len(trimmed)
				assert.Equal(t, 2, indent,
					"Line %d: infra-notify should be indented with 2 spaces, got %d", i+1, indent)
			}
		}
	})

	t.Run("component_path_exists", func(t *testing.T) {
		notifyPath := filepath.Join(projectRoot, "internal", "infrastructure", "notify")
		info, err := os.Stat(notifyPath)
		require.NoError(t, err, "infrastructure/notify directory should exist")
		assert.True(t, info.IsDir(), "infrastructure/notify should be a directory")
	})
}

func TestArchLintInfraNotify_ErrorHandling(t *testing.T) {
	t.Run("missing_config_file", func(t *testing.T) {
		_, err := os.ReadFile("nonexistent-config.yml")

		assert.Error(t, err, "Reading non-existent config should return error")
		assert.True(t, os.IsNotExist(err), "Error should be ErrNotExist")
	})

	t.Run("malformed_yaml_parsing", func(t *testing.T) {
		malformed := `
components:
  infra-notify:
    in: infrastructure/notify
  this is invalid yaml syntax [[[
`
		var config map[string]any
		err := yaml.Unmarshal([]byte(malformed), &config)

		assert.Error(t, err, "Malformed YAML should fail to parse")
	})

	t.Run("empty_dependency_list_is_valid", func(t *testing.T) {
		emptyDeps := `
deps:
  infra-notify:
    mayDependOn: []
    canUse:
      - go-stdlib
`
		var config struct {
			Deps map[string]struct {
				MayDependOn []string `yaml:"mayDependOn"`
				CanUse      []string `yaml:"canUse"`
			} `yaml:"deps"`
		}
		err := yaml.Unmarshal([]byte(emptyDeps), &config)
		require.NoError(t, err, "Empty dependency list should be valid YAML")

		assert.NotNil(t, config.Deps["infra-notify"].MayDependOn,
			"Empty dependency list should be non-nil slice")
		assert.Empty(t, config.Deps["infra-notify"].MayDependOn,
			"Should have zero dependencies")
		assert.Len(t, config.Deps["infra-notify"].CanUse, 1,
			"Should have one vendor dependency (go-stdlib)")
	})

	t.Run("missing_component_in_deps", func(t *testing.T) {
		missingDeps := `
components:
  infra-notify:
    in: infrastructure/notify
deps:
  other-component:
    mayDependOn: []
`
		var config struct {
			Components map[string]struct {
				In string `yaml:"in"`
			} `yaml:"components"`
			Deps map[string]struct {
				MayDependOn []string `yaml:"mayDependOn"`
			} `yaml:"deps"`
		}
		err := yaml.Unmarshal([]byte(missingDeps), &config)
		require.NoError(t, err)

		_, componentExists := config.Components["infra-notify"]
		assert.True(t, componentExists, "Component should be defined")

		_, depsExist := config.Deps["infra-notify"]
		assert.False(t, depsExist, "Dependencies should be missing (edge case)")
	})
}
