package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationFacadeUsesBuiltinSchemasAndConfiguredDefaults(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("AWF_WORKFLOWS_PATH", root)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".awf"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte("notify:\n  default_backend: desktop\n"), 0o600))
	facade, cleanup := buildFacade(&Config{StoragePath: root, Quiet: true})
	require.NotNil(t, facade)
	t.Cleanup(cleanup)
	for _, tc := range []struct {
		name, state string
		valid       bool
	}{
		{"http", "type: operation\n    operation: http.request\n    inputs:\n      method: GET\n      url: https://example.invalid", true},
		{"github", "type: operation\n    operation: github.create_issue\n    inputs:\n      title: Valid title", true},
		{"notify-default", "type: operation\n    operation: notify.send\n    inputs:\n      message: Valid message", true},
		{"legacy-shell", "type: step\n    operation: shell\n    command: echo valid", true},
		{"missing-url", "type: operation\n    operation: http.request\n    inputs:\n      method: GET", false},
		{"unknown-operation", "type: operation\n    operation: missing.operation\n    inputs: {}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := "name: " + tc.name + "\nversion: '1.0.0'\nstates:\n  initial: action\n  action:\n    " + tc.state + "\n    on_success: done\n  done:\n    type: terminal\n"
			require.NoError(t, os.WriteFile(filepath.Join(root, tc.name+".yaml"), []byte(content), 0o600))
			report, err := facade.Validate(context.Background(), ports.RunRequest{Identifier: tc.name})
			require.NoError(t, err)
			assert.Equal(t, tc.valid, report.Valid, "%+v", report.Errors)
		})
	}
	_, err := os.Stat(filepath.Join(root, "data", "awf", "audit.jsonl"))
	assert.True(t, os.IsNotExist(err), "read-only validation must not create an audit trail")
}
