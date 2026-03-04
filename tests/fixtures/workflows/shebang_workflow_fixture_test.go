package workflows_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func fixtureDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller should succeed")
	return filepath.Dir(file)
}

func loadShebangFixture(t *testing.T) map[string]interface{} {
	t.Helper()
	path := filepath.Join(fixtureDir(t), "script-file-shebang.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var parsed map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &parsed))
	return parsed
}

func TestWorkflowFixture_ScriptFileShebang_Exists(t *testing.T) {
	path := filepath.Join(fixtureDir(t), "script-file-shebang.yaml")
	_, err := os.Stat(path)
	assert.NoError(t, err)
}

func TestWorkflowFixture_ScriptFileShebang_ValidYAML(t *testing.T) {
	loadShebangFixture(t)
}

func TestWorkflowFixture_ScriptFileShebang_HasExpectedStates(t *testing.T) {
	parsed := loadShebangFixture(t)

	states, ok := parsed["states"].(map[string]interface{})
	require.True(t, ok, "states must be a map")

	assert.Equal(t, "python_step", states["initial"])

	for _, name := range []string{"python_step", "bash_step", "no_shebang_step", "done"} {
		assert.Contains(t, states, name)
	}
}

func TestWorkflowFixture_ScriptFileShebang_ScriptFileReferences(t *testing.T) {
	parsed := loadShebangFixture(t)

	states, ok := parsed["states"].(map[string]interface{})
	require.True(t, ok, "states must be a map")

	expectations := map[string]string{
		"python_step":     "scripts/shebang_python.py",
		"bash_step":       "scripts/shebang_bash.sh",
		"no_shebang_step": "scripts/no_shebang.sh",
	}

	for stepName, expectedScript := range expectations {
		step, ok := states[stepName].(map[string]interface{})
		require.True(t, ok, "%s must be a map", stepName)
		assert.Equal(t, expectedScript, step["script_file"], "%s script_file mismatch", stepName)
	}
}

func TestWorkflowFixture_ScriptFileShebang_TransitionsCorrect(t *testing.T) {
	parsed := loadShebangFixture(t)

	states, ok := parsed["states"].(map[string]interface{})
	require.True(t, ok, "states must be a map")

	chain := []struct {
		from      string
		onSuccess string
	}{
		{"python_step", "bash_step"},
		{"bash_step", "no_shebang_step"},
		{"no_shebang_step", "done"},
	}

	for _, link := range chain {
		step, ok := states[link.from].(map[string]interface{})
		require.True(t, ok, "%s must be a map", link.from)
		assert.Equal(t, link.onSuccess, step["on_success"], "%s on_success mismatch", link.from)
	}

	done, ok := states["done"].(map[string]interface{})
	require.True(t, ok, "done must be a map")
	assert.Equal(t, "terminal", done["type"])
}
