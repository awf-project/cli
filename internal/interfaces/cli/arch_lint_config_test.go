package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func loadArchLintConfig(t *testing.T) map[string]interface{} {
	t.Helper()
	configPath := filepath.Join("..", "..", "..", ".go-arch-lint.yml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	require.NoError(t, err)
	return config
}

func TestGoArchLintConfig_ValidYAML(t *testing.T) {
	config := loadArchLintConfig(t)
	assert.NotEmpty(t, config)
}

func TestGoArchLintConfig_HasRequiredTopLevelKeys(t *testing.T) {
	config := loadArchLintConfig(t)

	requiredKeys := []string{"version", "workdir", "components", "deps"}
	for _, key := range requiredKeys {
		assert.Contains(t, config, key, "missing required key: %s", key)
	}
}

func TestGoArchLintConfig_RequiredComponentsExist(t *testing.T) {
	config := loadArchLintConfig(t)

	components := config["components"].(map[string]interface{})

	requiredComponents := []string{
		"domain-workflow",
		"domain-ports",
		"domain-errors",
		"domain-plugin",
		"domain-operation",
		"application",
		"infra-plugin",
		"proto-plugin",
		"interfaces-cli",
	}

	for _, comp := range requiredComponents {
		assert.Contains(t, components, comp, "missing component: %s", comp)
	}
}

func TestGoArchLintConfig_InfraPluginDependencies(t *testing.T) {
	config := loadArchLintConfig(t)

	deps := config["deps"].(map[string]interface{})
	infraPluginDeps := deps["infra-plugin"].(map[string]interface{})

	mayDependOn := infraPluginDeps["mayDependOn"].([]interface{})
	mayDependOnStrs := make([]string, len(mayDependOn))
	for i, v := range mayDependOn {
		mayDependOnStrs[i] = v.(string)
	}

	requiredDeps := []string{"domain-plugin", "domain-operation", "domain-ports", "proto-plugin"}
	for _, dep := range requiredDeps {
		assert.Contains(t, mayDependOnStrs, dep,
			"infra-plugin should depend on %s for new validator/step_type imports", dep)
	}
}

func TestGoArchLintConfig_ApplicationDependencies(t *testing.T) {
	config := loadArchLintConfig(t)

	deps := config["deps"].(map[string]interface{})
	appDeps := deps["application"].(map[string]interface{})

	mayDependOn := appDeps["mayDependOn"].([]interface{})
	mayDependOnStrs := make([]string, len(mayDependOn))
	for i, v := range mayDependOn {
		mayDependOnStrs[i] = v.(string)
	}

	requiredDeps := []string{"domain-ports", "domain-workflow"}
	for _, dep := range requiredDeps {
		assert.Contains(t, mayDependOnStrs, dep,
			"application should depend on %s for SetValidatorProvider and SetStepTypeProvider", dep)
	}
}

func TestGoArchLintConfig_DomainPortsDependencies(t *testing.T) {
	config := loadArchLintConfig(t)

	deps := config["deps"].(map[string]interface{})
	portsDeps := deps["domain-ports"].(map[string]interface{})

	mayDependOn := portsDeps["mayDependOn"].([]interface{})
	mayDependOnStrs := make([]string, len(mayDependOn))
	for i, v := range mayDependOn {
		mayDependOnStrs[i] = v.(string)
	}

	requiredDeps := []string{"domain-workflow", "domain-errors", "domain-plugin", "domain-operation"}
	for _, dep := range requiredDeps {
		assert.Contains(t, mayDependOnStrs, dep,
			"domain-ports should depend on %s for new port definitions", dep)
	}
}

func TestGoArchLintConfig_ProtoPluginDependencies(t *testing.T) {
	config := loadArchLintConfig(t)

	deps := config["deps"].(map[string]interface{})
	protoDeps := deps["proto-plugin"].(map[string]interface{})

	canUse := protoDeps["canUse"].([]interface{})
	canUseStrs := make([]string, len(canUse))
	for i, v := range canUse {
		canUseStrs[i] = v.(string)
	}

	requiredVendors := []string{"go-stdlib", "go-protobuf"}
	for _, vendor := range requiredVendors {
		assert.Contains(t, canUseStrs, vendor,
			"proto-plugin should use %s for ValidatorService and StepTypeService definitions", vendor)
	}
}

func TestGoArchLintConfig_InterfacesCliCanAccessInfraPlugin(t *testing.T) {
	config := loadArchLintConfig(t)

	deps := config["deps"].(map[string]interface{})
	cliDeps := deps["interfaces-cli"].(map[string]interface{})

	mayDependOn := cliDeps["mayDependOn"].([]interface{})
	mayDependOnStrs := make([]string, len(mayDependOn))
	for i, v := range mayDependOn {
		mayDependOnStrs[i] = v.(string)
	}

	assert.Contains(t, mayDependOnStrs, "infra-plugin",
		"interfaces-cli must depend on infra-plugin to wire validator/step_type providers")
}

func TestGoArchLintConfig_NoCircularDependencies(t *testing.T) {
	config := loadArchLintConfig(t)

	domainLayers := []string{"domain-workflow", "domain-ports", "domain-errors", "domain-plugin", "domain-operation"}
	infraLayers := []string{"infra-plugin", "infra-repository", "infra-executor", "infra-logger"}

	deps := config["deps"].(map[string]interface{})

	for _, domainComponent := range domainLayers {
		domainDeps := deps[domainComponent].(map[string]interface{})
		mayDependOnRaw := domainDeps["mayDependOn"]

		if mayDependOnRaw != nil {
			mayDependOn := mayDependOnRaw.([]interface{})
			mayDependOnStrs := make([]string, len(mayDependOn))
			for i, v := range mayDependOn {
				mayDependOnStrs[i] = v.(string)
			}

			for _, infraComponent := range infraLayers {
				assert.NotContains(t, mayDependOnStrs, infraComponent,
					"domain layer component %s should not depend on infrastructure %s", domainComponent, infraComponent)
			}
		}
	}
}
