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

// TestResolveStepCommand_LocalOverGlobal_ScriptsDir tests that {{.awf.scripts_dir}} in command
// resolves to local .awf/scripts/ when the referenced file exists locally.
func TestResolveStepCommand_LocalOverGlobal_ScriptsDir(t *testing.T) {
	tmpDir := t.TempDir()

	localScriptsDir := filepath.Join(tmpDir, ".awf", "scripts")
	globalScriptsDir := filepath.Join(tmpDir, "global-scripts")

	require.NoError(t, os.MkdirAll(localScriptsDir, 0o755))
	require.NoError(t, os.MkdirAll(globalScriptsDir, 0o755))

	localScriptPath := filepath.Join(localScriptsDir, "helpers.sh")
	require.NoError(t, os.WriteFile(localScriptPath, []byte("local helpers"), 0o755))

	globalScriptPath := filepath.Join(globalScriptsDir, "helpers.sh")
	require.NoError(t, os.WriteFile(globalScriptPath, []byte("global helpers"), 0o755))

	step := &workflow.Step{
		Name:    "deploy",
		Type:    workflow.StepTypeCommand,
		Command: "source {{.awf.scripts_dir}}/helpers.sh && deploy",
	}

	intCtx := &interpolation.Context{
		AWF: map[string]string{
			"scripts_dir": globalScriptsDir,
		},
	}

	wf := &workflow.Workflow{
		SourceDir: filepath.Join(tmpDir, ".awf", "workflows"),
	}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
		awfPaths: map[string]string{
			"scripts_dir": globalScriptsDir,
		},
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.NoError(t, err)
	require.NotNil(t, cmd)
	assert.Contains(t, cmd.Program, localScriptsDir, "should resolve to local .awf/scripts directory")
	assert.NotContains(t, cmd.Program, globalScriptsDir, "should not use global directory when local file exists")
}

// TestResolveStepCommand_LocalOverGlobal_PromptsDir tests that {{.awf.prompts_dir}} in command
// resolves to local .awf/prompts/ when the referenced file exists locally.
func TestResolveStepCommand_LocalOverGlobal_PromptsDir(t *testing.T) {
	tmpDir := t.TempDir()

	localPromptsDir := filepath.Join(tmpDir, ".awf", "prompts")
	globalPromptsDir := filepath.Join(tmpDir, "global-prompts")

	require.NoError(t, os.MkdirAll(localPromptsDir, 0o755))
	require.NoError(t, os.MkdirAll(globalPromptsDir, 0o755))

	localPromptPath := filepath.Join(localPromptsDir, "template.md")
	require.NoError(t, os.WriteFile(localPromptPath, []byte("local template"), 0o644))

	globalPromptPath := filepath.Join(globalPromptsDir, "template.md")
	require.NoError(t, os.WriteFile(globalPromptPath, []byte("global template"), 0o644))

	step := &workflow.Step{
		Name:    "analyze",
		Type:    workflow.StepTypeCommand,
		Command: "cat {{.awf.prompts_dir}}/template.md | process",
	}

	intCtx := &interpolation.Context{
		AWF: map[string]string{
			"prompts_dir": globalPromptsDir,
		},
	}

	wf := &workflow.Workflow{
		SourceDir: filepath.Join(tmpDir, ".awf", "workflows"),
	}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
		awfPaths: map[string]string{
			"prompts_dir": globalPromptsDir,
		},
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.NoError(t, err)
	require.NotNil(t, cmd)
	assert.Contains(t, cmd.Program, localPromptsDir, "should resolve to local .awf/prompts directory")
	assert.NotContains(t, cmd.Program, globalPromptsDir, "should not use global directory when local file exists")
}

// TestResolveStepCommand_LocalOverGlobal_GlobalFallback tests that {{.awf.scripts_dir}} falls back
// to global path when no local file exists.
func TestResolveStepCommand_LocalOverGlobal_GlobalFallback(t *testing.T) {
	tmpDir := t.TempDir()

	localScriptsDir := filepath.Join(tmpDir, ".awf", "scripts")
	globalScriptsDir := filepath.Join(tmpDir, "global-scripts")

	require.NoError(t, os.MkdirAll(localScriptsDir, 0o755))
	require.NoError(t, os.MkdirAll(globalScriptsDir, 0o755))

	// No local file — only global exists, verifying fallback behavior
	globalScriptPath := filepath.Join(globalScriptsDir, "setup.sh")
	require.NoError(t, os.WriteFile(globalScriptPath, []byte("setup script"), 0o755))

	step := &workflow.Step{
		Name:    "setup",
		Type:    workflow.StepTypeCommand,
		Command: "bash {{.awf.scripts_dir}}/setup.sh",
	}

	intCtx := &interpolation.Context{
		AWF: map[string]string{
			"scripts_dir": globalScriptsDir,
		},
	}

	wf := &workflow.Workflow{
		SourceDir: filepath.Join(tmpDir, ".awf", "workflows"),
	}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
		awfPaths: map[string]string{
			"scripts_dir": globalScriptsDir,
		},
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.NoError(t, err)
	require.NotNil(t, cmd)
	assert.Contains(t, cmd.Program, globalScriptsDir, "should fall back to global directory when local file doesn't exist")
}

// TestResolveStepCommand_LocalOverGlobal_NoAWFVars tests that commands without AWF variables
// pass through unchanged.
func TestResolveStepCommand_LocalOverGlobal_NoAWFVars(t *testing.T) {
	tmpDir := t.TempDir()

	step := &workflow.Step{
		Name:    "echo",
		Type:    workflow.StepTypeCommand,
		Command: "echo 'Hello, World!'",
	}

	intCtx := &interpolation.Context{
		AWF: map[string]string{},
	}

	wf := &workflow.Workflow{
		SourceDir: tmpDir,
	}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.NoError(t, err)
	require.NotNil(t, cmd)
	assert.Equal(t, "echo 'Hello, World!'", cmd.Program, "command without AWF variables should pass through unchanged")
}

// TestResolveStepCommand_LocalOverGlobal_DirField tests that {{.awf.scripts_dir}} in dir field
// resolves to local directory when it exists locally.
func TestResolveStepCommand_LocalOverGlobal_DirField(t *testing.T) {
	tmpDir := t.TempDir()

	localScriptsDir := filepath.Join(tmpDir, ".awf", "scripts")
	globalScriptsDir := filepath.Join(tmpDir, "global-scripts")

	require.NoError(t, os.MkdirAll(localScriptsDir, 0o755))
	require.NoError(t, os.MkdirAll(globalScriptsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(localScriptsDir, "marker.txt"), []byte("local"), 0o644))

	step := &workflow.Step{
		Name:    "run-local",
		Type:    workflow.StepTypeCommand,
		Command: "ls",
		Dir:     "{{.awf.scripts_dir}}",
	}

	intCtx := &interpolation.Context{
		AWF: map[string]string{
			"scripts_dir": globalScriptsDir,
		},
	}

	wf := &workflow.Workflow{
		SourceDir: filepath.Join(tmpDir, ".awf", "workflows"),
	}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
		awfPaths: map[string]string{
			"scripts_dir": globalScriptsDir,
		},
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.NoError(t, err)
	require.NotNil(t, cmd)
	assert.Equal(t, localScriptsDir, cmd.Dir, "dir field should resolve to local directory when it exists")
}

// TestResolveStepCommand_LocalOverGlobal_MultipleVars tests that commands with multiple AWF variables
// have all variables resolved with local-over-global precedence.
func TestResolveStepCommand_LocalOverGlobal_MultipleVars(t *testing.T) {
	tmpDir := t.TempDir()

	localScriptsDir := filepath.Join(tmpDir, ".awf", "scripts")
	localPromptsDir := filepath.Join(tmpDir, ".awf", "prompts")
	globalScriptsDir := filepath.Join(tmpDir, "global-scripts")
	globalPromptsDir := filepath.Join(tmpDir, "global-prompts")

	require.NoError(t, os.MkdirAll(localScriptsDir, 0o755))
	require.NoError(t, os.MkdirAll(localPromptsDir, 0o755))
	require.NoError(t, os.MkdirAll(globalScriptsDir, 0o755))
	require.NoError(t, os.MkdirAll(globalPromptsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(localScriptsDir, "helpers.sh"), []byte("local"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(localPromptsDir, "template.md"), []byte("local"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(globalScriptsDir, "helpers.sh"), []byte("global"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(globalPromptsDir, "template.md"), []byte("global"), 0o644))

	step := &workflow.Step{
		Name:    "process",
		Type:    workflow.StepTypeCommand,
		Command: "source {{.awf.scripts_dir}}/helpers.sh && cat {{.awf.prompts_dir}}/template.md",
	}

	intCtx := &interpolation.Context{
		AWF: map[string]string{
			"scripts_dir": globalScriptsDir,
			"prompts_dir": globalPromptsDir,
		},
	}

	wf := &workflow.Workflow{
		SourceDir: filepath.Join(tmpDir, ".awf", "workflows"),
	}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
		awfPaths: map[string]string{
			"scripts_dir": globalScriptsDir,
			"prompts_dir": globalPromptsDir,
		},
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.NoError(t, err)
	require.NotNil(t, cmd)
	assert.Contains(t, cmd.Program, localScriptsDir, "should resolve scripts_dir to local")
	assert.Contains(t, cmd.Program, localPromptsDir, "should resolve prompts_dir to local")
	assert.NotContains(t, cmd.Program, globalScriptsDir, "should not contain global scripts_dir")
	assert.NotContains(t, cmd.Program, globalPromptsDir, "should not contain global prompts_dir")
}

// TestResolveStepCommand_LocalOverGlobal_MultipleOccurrencesSameKey tests that a command containing
// three references to the same scripts_dir prefix resolves all resolvable occurrences independently.
// The middle reference (b.sh) does NOT exist locally, so it must stay global while the first (a.sh)
// and third (c.sh) references, which do exist locally, are resolved to their local paths.
func TestResolveStepCommand_LocalOverGlobal_MultipleOccurrencesSameKey(t *testing.T) {
	tmpDir := t.TempDir()

	localScriptsDir := filepath.Join(tmpDir, ".awf", "scripts")
	globalScriptsDir := filepath.Join(tmpDir, "global-scripts")

	require.NoError(t, os.MkdirAll(localScriptsDir, 0o755))
	require.NoError(t, os.MkdirAll(globalScriptsDir, 0o755))

	// a.sh and c.sh exist locally; b.sh only exists globally.
	require.NoError(t, os.WriteFile(filepath.Join(localScriptsDir, "a.sh"), []byte("local a"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(globalScriptsDir, "a.sh"), []byte("global a"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(globalScriptsDir, "b.sh"), []byte("global b only"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(localScriptsDir, "c.sh"), []byte("local c"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(globalScriptsDir, "c.sh"), []byte("global c"), 0o755))

	step := &workflow.Step{
		Name:    "multi-source",
		Type:    workflow.StepTypeCommand,
		Command: "source {{.awf.scripts_dir}}/a.sh && source {{.awf.scripts_dir}}/b.sh && source {{.awf.scripts_dir}}/c.sh",
	}

	intCtx := &interpolation.Context{
		AWF: map[string]string{
			"scripts_dir": globalScriptsDir,
		},
	}

	wf := &workflow.Workflow{
		SourceDir: filepath.Join(tmpDir, ".awf", "workflows"),
	}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
		awfPaths: map[string]string{
			"scripts_dir": globalScriptsDir,
		},
	}

	cmd, err := svc.resolveStepCommand(context.Background(), wf, step, intCtx)

	require.NoError(t, err)
	require.NotNil(t, cmd)

	// a.sh and c.sh must resolve to local paths.
	assert.Contains(t, cmd.Program, filepath.Join(localScriptsDir, "a.sh"), "a.sh should resolve to local")
	assert.Contains(t, cmd.Program, filepath.Join(localScriptsDir, "c.sh"), "c.sh should resolve to local")

	// b.sh has no local copy — must remain at its global path.
	assert.Contains(t, cmd.Program, filepath.Join(globalScriptsDir, "b.sh"), "b.sh should stay at global path when no local copy exists")
}
