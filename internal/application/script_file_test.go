package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadScriptFile_DelegatesToLoadExternalFile(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "deploy.sh")
	scriptContent := "#!/bin/bash\necho 'Deploy script'"
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
	}

	intCtx := &interpolation.Context{}

	result, err := loadScriptFile(context.Background(), "deploy.sh", wf, intCtx)

	require.NoError(t, err)
	assert.Equal(t, scriptContent, result)
}
