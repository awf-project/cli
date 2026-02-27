package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockResolverForScriptFile is a real resolver (not a mock) for script file tests.
type mockResolverForScriptFile struct {
	resolver interpolation.Resolver
}

func newRealResolver() *mockResolverForScriptFile {
	return &mockResolverForScriptFile{
		resolver: interpolation.NewTemplateResolver(),
	}
}

func (m *mockResolverForScriptFile) Resolve(template string, ctx *interpolation.Context) (string, error) {
	return m.resolver.Resolve(template, ctx)
}

func TestResolveStepCommand_ScriptFile_HappyPath_LoadsAndInterpolatesContent(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "deploy.sh")
	scriptContent := "echo 'Deploying to production'"
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	step := &workflow.Step{
		Name:       "deploy",
		Type:       workflow.StepTypeCommand,
		ScriptFile: "deploy.sh",
		Command:    "",
	}

	intCtx := &interpolation.Context{
		Inputs: map[string]any{},
	}

	wf := &workflow.Workflow{SourceDir: tmpDir}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.NoError(t, err, "should load script file successfully")
	require.NotNil(t, cmd, "should return command")
	assert.Equal(t, scriptContent, cmd.Program, "command should contain loaded script file content")
}

func TestResolveStepCommand_ScriptFile_HappyPath_InterpolatesPathBeforeLoading(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	require.NoError(t, os.MkdirAll(scriptsDir, 0o755))

	scriptPath := filepath.Join(scriptsDir, "build.sh")
	scriptContent := "make build"
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	step := &workflow.Step{
		Name:       "build",
		Type:       workflow.StepTypeCommand,
		ScriptFile: "{{.awf.scripts_dir}}/build.sh",
	}

	intCtx := &interpolation.Context{
		AWF: map[string]string{
			"scripts_dir": scriptsDir,
		},
	}

	wf := &workflow.Workflow{SourceDir: tmpDir}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
		awfPaths: map[string]string{
			"scripts_dir": scriptsDir,
		},
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.NoError(t, err, "should interpolate path and load script file")
	require.NotNil(t, cmd)
	assert.Equal(t, scriptContent, cmd.Program, "should load content from interpolated path")
}

func TestResolveStepCommand_ScriptFile_HappyPath_ContentWithTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "notify.sh")
	scriptContent := "curl -X POST {{.inputs.webhook_url}} -d 'Build completed'"
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	step := &workflow.Step{
		Name:       "notify",
		Type:       workflow.StepTypeCommand,
		ScriptFile: "notify.sh",
	}

	intCtx := &interpolation.Context{
		Inputs: map[string]any{
			"webhook_url": "https://api.example.com/notify",
		},
	}

	wf := &workflow.Workflow{SourceDir: tmpDir}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.NoError(t, err, "should load and interpolate script content")
	require.NotNil(t, cmd)
	assert.Equal(t, "curl -X POST https://api.example.com/notify -d 'Build completed'", cmd.Program,
		"script content should be interpolated")
}

func TestResolveStepCommand_ScriptFile_Error_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	step := &workflow.Step{
		Name:       "missing",
		Type:       workflow.StepTypeCommand,
		ScriptFile: "nonexistent.sh",
	}

	intCtx := &interpolation.Context{}

	wf := &workflow.Workflow{SourceDir: tmpDir}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.Error(t, err, "should return error when script file not found")
	assert.Nil(t, cmd, "should not return command on error")
	assert.Contains(t, err.Error(), "nonexistent.sh", "error should include file path")
}

func TestResolveStepCommand_ScriptFile_Error_SizeExceeded(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "oversized.sh")
	oversizedContent := strings.Repeat("echo 'line'\n", 100000)
	require.NoError(t, os.WriteFile(scriptPath, []byte(oversizedContent), 0o755))

	step := &workflow.Step{
		Name:       "oversized",
		Type:       workflow.StepTypeCommand,
		ScriptFile: "oversized.sh",
	}

	intCtx := &interpolation.Context{}

	wf := &workflow.Workflow{SourceDir: tmpDir}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.Error(t, err, "should return error when file exceeds size limit")
	assert.Nil(t, cmd)
	assert.Contains(t, err.Error(), "exceeds 1MB limit", "error should indicate size limit exceeded")
}
