//go:build integration

package docs_test

// Feature: F072

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

func TestHugoSiteConfiguration(t *testing.T) {
	root := projectRoot(t)

	t.Run("ignoreFiles excludes CLAUDE.md and template files", func(t *testing.T) {
		hugoToml, err := os.ReadFile(filepath.Join(root, "site", "config", "_default", "hugo.toml"))
		require.NoError(t, err)
		content := string(hugoToml)

		assert.Contains(t, content, `CLAUDE\.md$`, "ignoreFiles must exclude CLAUDE.md (FR-005)")
		assert.Contains(t, content, `\.template\.md$`, "ignoreFiles must exclude .template.md")
	})

	t.Run("module mounts map all docs sections", func(t *testing.T) {
		moduleToml, err := os.ReadFile(filepath.Join(root, "site", "config", "_default", "module.toml"))
		require.NoError(t, err)
		content := string(moduleToml)

		expectedMounts := []struct {
			source string
			target string
		}{
			{"../docs/getting-started", "content/docs/getting-started"},
			{"../docs/user-guide", "content/docs/user-guide"},
			{"../docs/reference", "content/docs/reference"},
			{"../docs/development", "content/docs/development"},
			{"../docs/ADR", "content/docs/adr"},
		}

		for _, mount := range expectedMounts {
			assert.Contains(t, content, mount.source,
				"module.toml must mount %s (FR-003, FR-013)", mount.source)
			assert.Contains(t, content, mount.target,
				"module.toml must target %s", mount.target)
		}
	})

	t.Run("navigation menu has required entries", func(t *testing.T) {
		menuToml, err := os.ReadFile(filepath.Join(root, "site", "config", "_default", "menus", "menus.en.toml"))
		require.NoError(t, err)
		content := string(menuToml)

		assert.Contains(t, content, `"/docs/"`, "navigation must include Docs link (FR-008)")
		assert.Contains(t, content, `"/blog/"`, "navigation must include Blog link (FR-008)")
		assert.Contains(t, content, "github.com/awf-project/cli", "navigation must include GitHub link (FR-008)")
	})
}

func TestDocsSectionStructure(t *testing.T) {
	root := projectRoot(t)

	t.Run("section index files exist with front matter", func(t *testing.T) {
		sections := []struct {
			path          string
			expectedTitle string
		}{
			{"site/content/docs/_index.md", "Documentation"},
			{"site/content/docs/getting-started/_index.md", "Getting Started"},
			{"site/content/docs/user-guide/_index.md", "User Guide"},
			{"site/content/docs/reference/_index.md", "Reference"},
			{"site/content/docs/development/_index.md", "Development"},
			{"site/content/docs/adr/_index.md", "Architecture Decision Records"},
		}

		for _, section := range sections {
			data, err := os.ReadFile(filepath.Join(root, section.path))
			require.NoError(t, err, "section index must exist: %s", section.path)

			content := string(data)
			assert.True(t, strings.HasPrefix(content, "---"),
				"%s must have YAML front matter", section.path)
			assert.Contains(t, content, section.expectedTitle,
				"%s must have correct title", section.path)
		}
	})

	t.Run("original docs files are not moved or deleted", func(t *testing.T) {
		// FR-011: docs/ content served via mounts, originals untouched
		expectedDirs := []string{
			"docs/getting-started",
			"docs/user-guide",
			"docs/reference",
			"docs/development",
			"docs/ADR",
		}

		for _, dir := range expectedDirs {
			info, err := os.Stat(filepath.Join(root, dir))
			require.NoError(t, err, "original docs directory must exist: %s", dir)
			assert.True(t, info.IsDir(), "%s must be a directory", dir)

			entries, err := os.ReadDir(filepath.Join(root, dir))
			require.NoError(t, err)
			assert.NotEmpty(t, entries, "%s must contain files", dir)
		}
	})

	t.Run("landing page has install snippet and description", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "site", "content", "_index.md"))
		require.NoError(t, err)
		content := string(data)

		assert.Contains(t, content, "go install", "landing page must have install snippet (US2)")
		assert.Contains(t, content, "AWF", "landing page must reference AWF")
	})

	t.Run("blog section has placeholder post", func(t *testing.T) {
		blogIndex := filepath.Join(root, "site", "content", "blog", "_index.md")
		_, err := os.Stat(blogIndex)
		require.NoError(t, err, "blog section index must exist (FR-006)")

		postIndex := filepath.Join(root, "site", "content", "blog", "introducing-awf", "index.md")
		data, err := os.ReadFile(postIndex)
		require.NoError(t, err, "blog placeholder post must exist (FR-006)")
		assert.Contains(t, string(data), "AWF")
	})
}

func TestGitHubPagesWorkflow(t *testing.T) {
	root := projectRoot(t)

	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "hugo.yml"))
	require.NoError(t, err, "hugo.yml workflow must exist (FR-007)")

	var workflow map[string]any
	require.NoError(t, yaml.Unmarshal(data, &workflow), "hugo.yml must be valid YAML")

	t.Run("has build and deploy jobs", func(t *testing.T) {
		jobs, ok := workflow["jobs"].(map[string]any)
		require.True(t, ok, "workflow must have jobs")

		assert.Contains(t, jobs, "build", "workflow must have build job")
		assert.Contains(t, jobs, "deploy", "workflow must have deploy job")
	})

	t.Run("deploy is conditional on main branch push", func(t *testing.T) {
		jobs := workflow["jobs"].(map[string]any)
		deploy := jobs["deploy"].(map[string]any)

		condition, ok := deploy["if"].(string)
		require.True(t, ok, "deploy job must have if condition")
		assert.Contains(t, condition, "refs/heads/main", "deploy must be conditional on main branch")
	})

	t.Run("has required permissions for GitHub Pages", func(t *testing.T) {
		perms, ok := workflow["permissions"].(map[string]any)
		require.True(t, ok, "workflow must declare permissions")

		assert.Equal(t, "read", perms["contents"], "contents permission must be read")
		assert.Equal(t, "write", perms["pages"], "pages permission must be write")
		assert.Equal(t, "write", perms["id-token"], "id-token permission must be write")
	})

	t.Run("Hugo version is pinned", func(t *testing.T) {
		jobs := workflow["jobs"].(map[string]any)
		build := jobs["build"].(map[string]any)
		env := build["env"].(map[string]any)

		hugoVersion, ok := env["HUGO_VERSION"].(string)
		require.True(t, ok, "build job must set HUGO_VERSION (NFR-003)")
		assert.NotEmpty(t, hugoVersion, "HUGO_VERSION must not be empty")
		assert.Regexp(t, `^\d+\.\d+\.\d+$`, hugoVersion, "HUGO_VERSION must be a semver string")
	})
}
