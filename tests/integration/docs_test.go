//go:build integration

package integration_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ╔══════════════════════════════════════════════════════════════════════════╗
// ║ T018: Documentation Tests for Project Configuration                       ║
// ╠══════════════════════════════════════════════════════════════════════════╣
// ║ Verifies documentation completeness for F036 feature:                     ║
// ║   - configuration.md exists and contains all required sections           ║
// ║   - commands.md includes awf config show documentation                   ║
// ║   - No NOT IMPLEMENTED markers remain in final documentation             ║
// ╚══════════════════════════════════════════════════════════════════════════╝

// getProjectRoot returns the project root directory.
func getProjectRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "failed to get caller info")

	// Navigate from tests/integration/ to project root
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

// TestConfigurationDocumentation_Integration tests that configuration.md is complete.
func TestConfigurationDocumentation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	projectRoot := getProjectRoot(t)
	configDocPath := filepath.Join(projectRoot, "docs", "user-guide", "configuration.md")

	t.Run("configuration.md exists", func(t *testing.T) {
		_, err := os.Stat(configDocPath)
		require.NoError(t, err, "docs/user-guide/configuration.md should exist")
	})

	t.Run("contains required sections", func(t *testing.T) {
		content, err := os.ReadFile(configDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		requiredSections := []struct {
			heading     string
			description string
		}{
			{"# Project Configuration", "main title"},
			{"## Overview", "overview section"},
			{"## Configuration File Location", "file location documentation"},
			{"## Configuration Format", "YAML format documentation"},
			{"## Input Pre-population", "input pre-population explanation"},
			{"### Priority Order", "merge priority documentation"},
			{"## Initialization", "awf init documentation"},
			{"## Viewing Configuration", "awf config show documentation"},
			{"## Validation and Errors", "validation behavior"},
			{"### Invalid YAML", "error handling documentation"},
			{"### Unknown Keys", "warning behavior documentation"},
			{"## Best Practices", "best practices/security guidance"},
		}

		for _, section := range requiredSections {
			assert.Contains(t, contentStr, section.heading,
				"missing section: %s (%s)", section.heading, section.description)
		}
	})

	t.Run("contains config file path", func(t *testing.T) {
		content, err := os.ReadFile(configDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Must document the config file path
		assert.Contains(t, contentStr, ".awf/config.yaml",
			"should document the config file path")
	})

	t.Run("contains YAML example", func(t *testing.T) {
		content, err := os.ReadFile(configDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Must have at least one proper YAML code block with inputs:
		yamlBlockPattern := regexp.MustCompile("```yaml[\\s\\S]*?inputs:[\\s\\S]*?```")
		assert.Regexp(t, yamlBlockPattern, contentStr,
			"should contain a YAML code block with inputs: example")
	})

	t.Run("documents CLI override behavior", func(t *testing.T) {
		content, err := os.ReadFile(configDocPath)
		require.NoError(t, err)
		contentStr := strings.ToLower(string(content))

		// Must explain that CLI flags override config
		hasOverrideDoc := strings.Contains(contentStr, "override") ||
			strings.Contains(contentStr, "priority") ||
			strings.Contains(contentStr, "precedence")
		assert.True(t, hasOverrideDoc,
			"should document CLI flag override behavior")
	})

	t.Run("documents secrets guidance", func(t *testing.T) {
		content, err := os.ReadFile(configDocPath)
		require.NoError(t, err)
		contentStr := strings.ToLower(string(content))

		// NFR-002: Must document not to store secrets
		hasSecretsGuidance := strings.Contains(contentStr, "secret") ||
			strings.Contains(contentStr, "password") ||
			strings.Contains(contentStr, "credential") ||
			strings.Contains(contentStr, "sensitive")
		assert.True(t, hasSecretsGuidance,
			"should document guidance on not storing secrets")
	})

	t.Run("no NOT IMPLEMENTED markers remain", func(t *testing.T) {
		content, err := os.ReadFile(configDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Stub markers should be replaced with real content
		assert.NotContains(t, contentStr, "NOT IMPLEMENTED",
			"documentation should not contain NOT IMPLEMENTED markers")
		assert.NotContains(t, contentStr, "TODO:",
			"documentation should not contain TODO markers")
	})

	t.Run("references commands.md", func(t *testing.T) {
		content, err := os.ReadFile(configDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Should cross-reference the commands documentation
		assert.Contains(t, contentStr, "commands.md",
			"should reference commands.md for CLI documentation")
	})
}

// TestCommandsDocumentation_ConfigSection_Integration tests that commands.md includes config show.
func TestCommandsDocumentation_ConfigSection_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	projectRoot := getProjectRoot(t)
	commandsDocPath := filepath.Join(projectRoot, "docs", "user-guide", "commands.md")

	t.Run("commands.md exists", func(t *testing.T) {
		_, err := os.Stat(commandsDocPath)
		require.NoError(t, err, "docs/user-guide/commands.md should exist")
	})

	t.Run("contains awf config show in overview table", func(t *testing.T) {
		content, err := os.ReadFile(commandsDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		assert.Contains(t, contentStr, "awf config show",
			"overview table should include 'awf config show' command")
	})

	t.Run("contains awf config show section", func(t *testing.T) {
		content, err := os.ReadFile(commandsDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		assert.Contains(t, contentStr, "## awf config show",
			"should have dedicated section for 'awf config show'")
	})

	t.Run("documents config show flags", func(t *testing.T) {
		content, err := os.ReadFile(commandsDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Should document the --format flag
		assert.Contains(t, contentStr, "--format",
			"should document --format flag for config show")
	})

	t.Run("documents config show examples", func(t *testing.T) {
		content, err := os.ReadFile(commandsDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Find the config show section and verify it has examples
		configShowIdx := strings.Index(contentStr, "## awf config show")
		require.NotEqual(t, -1, configShowIdx, "config show section should exist")

		// Get content from config show section to next section
		afterConfigShow := contentStr[configShowIdx:]
		nextSectionIdx := strings.Index(afterConfigShow[3:], "\n## ")
		if nextSectionIdx == -1 {
			nextSectionIdx = len(afterConfigShow)
		} else {
			nextSectionIdx += 3
		}
		configShowSection := afterConfigShow[:nextSectionIdx]

		assert.Contains(t, configShowSection, "```bash",
			"config show section should have bash examples")
	})

	t.Run("config show section links to configuration.md", func(t *testing.T) {
		content, err := os.ReadFile(commandsDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Find config show section
		configShowIdx := strings.Index(contentStr, "## awf config show")
		require.NotEqual(t, -1, configShowIdx)

		afterConfigShow := contentStr[configShowIdx:]
		nextSectionIdx := strings.Index(afterConfigShow[3:], "\n## ")
		if nextSectionIdx == -1 {
			nextSectionIdx = len(afterConfigShow)
		} else {
			nextSectionIdx += 3
		}
		configShowSection := afterConfigShow[:nextSectionIdx]

		assert.Contains(t, configShowSection, "configuration.md",
			"config show section should link to configuration.md")
	})

	t.Run("config show section has no NOT IMPLEMENTED markers", func(t *testing.T) {
		content, err := os.ReadFile(commandsDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Find config show section
		configShowIdx := strings.Index(contentStr, "## awf config show")
		require.NotEqual(t, -1, configShowIdx)

		afterConfigShow := contentStr[configShowIdx:]
		nextSectionIdx := strings.Index(afterConfigShow[3:], "\n## ")
		if nextSectionIdx == -1 {
			nextSectionIdx = len(afterConfigShow)
		} else {
			nextSectionIdx += 3
		}
		configShowSection := afterConfigShow[:nextSectionIdx]

		assert.NotContains(t, configShowSection, "NOT IMPLEMENTED",
			"config show section should not contain NOT IMPLEMENTED markers")
	})

	t.Run("awf init documents config.yaml creation", func(t *testing.T) {
		content, err := os.ReadFile(commandsDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Find the init section
		initIdx := strings.Index(contentStr, "## awf init")
		require.NotEqual(t, -1, initIdx, "awf init section should exist")

		// Get content from init section to next major section
		afterInit := contentStr[initIdx:]
		nextSectionIdx := strings.Index(afterInit[3:], "\n## ")
		if nextSectionIdx == -1 {
			nextSectionIdx = len(afterInit)
		} else {
			nextSectionIdx += 3
		}
		initSection := afterInit[:nextSectionIdx]

		// Should mention config.yaml in the created structure
		assert.Contains(t, initSection, "config.yaml",
			"awf init section should document config.yaml creation")
	})
}

// TestDocumentationConsistency_Integration tests cross-references between docs.
func TestDocumentationConsistency_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	projectRoot := getProjectRoot(t)

	t.Run("all referenced docs exist", func(t *testing.T) {
		configDocPath := filepath.Join(projectRoot, "docs", "user-guide", "configuration.md")
		content, err := os.ReadFile(configDocPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Extract markdown links like [text](file.md)
		linkPattern := regexp.MustCompile(`\]\(([^)]+\.md)\)`)
		matches := linkPattern.FindAllStringSubmatch(contentStr, -1)

		for _, match := range matches {
			linkedFile := match[1]
			// Handle relative paths from docs/user-guide/
			linkedPath := filepath.Join(projectRoot, "docs", "user-guide", linkedFile)
			_, err := os.Stat(linkedPath)
			assert.NoError(t, err, "linked file should exist: %s", linkedFile)
		}
	})
}
