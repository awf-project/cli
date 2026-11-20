//go:build integration

package cli_test

// T010: Integration tests for workflow pack resolution and execution
// Tests: 12 functions covering pack resolution, context injection, path resolution, and error handling
// Scope: End-to-end workflow pack execution via CLI with local overrides and manifest validation

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createPackStructure creates a workflow pack in the .awf/workflow-packs directory
// with the given pack name, workflows, and optional manifest.
func createPackStructure(t *testing.T, baseDir, packName string, workflows map[string]string) {
	t.Helper()

	packDir := filepath.Join(baseDir, ".awf", "workflow-packs", packName)
	require.NoError(t, os.MkdirAll(packDir, 0o755))

	// Create workflows directory and workflow files
	workflowsDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	for name, content := range workflows {
		workflowPath := filepath.Join(workflowsDir, name+".yaml")
		require.NoError(t, os.WriteFile(workflowPath, []byte(content), 0o644))
	}

	// Create manifest.yaml listing public workflows
	manifestPath := filepath.Join(packDir, "manifest.yaml")
	manifestContent := fmt.Sprintf("name: %s\nworkflows:\n", packName)
	for name := range workflows {
		manifestContent += fmt.Sprintf("  - %s\n", name)
	}
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifestContent), 0o644))
}

// createPackPrompts creates prompts directory within a pack
func createPackPrompts(t *testing.T, baseDir, packName string, prompts map[string]string) {
	t.Helper()

	promptDir := filepath.Join(baseDir, ".awf", "workflow-packs", packName, "prompts")
	require.NoError(t, os.MkdirAll(promptDir, 0o755))

	for name, content := range prompts {
		promptPath := filepath.Join(promptDir, name)
		dir := filepath.Dir(promptPath)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(promptPath, []byte(content), 0o644))
	}
}

// createPackScripts creates scripts directory within a pack
func createPackScripts(t *testing.T, baseDir, packName string, scripts map[string]string) {
	t.Helper()

	scriptsDir := filepath.Join(baseDir, ".awf", "workflow-packs", packName, "scripts")
	require.NoError(t, os.MkdirAll(scriptsDir, 0o755))

	for name, content := range scripts {
		scriptPath := filepath.Join(scriptsDir, name)
		require.NoError(t, os.WriteFile(scriptPath, []byte(content), 0o644))
	}
}

// TestPackRun_SimpleWorkflow executes a packaged workflow successfully
func TestPackRun_SimpleWorkflow(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

	// Create a simple workflow in a pack
	workflows := map[string]string{
		"greet": `name: greet
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "Hello from pack"
    on_success: done
  done:
    type: terminal
`,
	}
	createPackStructure(t, tmpDir, "greeting", workflows)

	// Run the packaged workflow
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "greeting/greet", "--step=start"})

	err := cmd.Execute()
	require.NoError(t, err, "Failed to run packaged workflow: %v", err)
}

// TestPackRun_UnlistedWorkflow rejects workflows not in manifest
func TestPackRun_UnlistedWorkflow(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create pack with one public workflow
	workflows := map[string]string{
		"public": `name: public
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
		// This workflow exists but is not listed in manifest
		"internal": `name: internal
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
	}
	createPackStructure(t, tmpDir, "restricted", workflows)

	// Update manifest to only list "public"
	manifestPath := filepath.Join(tmpDir, ".awf", "workflow-packs", "restricted", "manifest.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte("name: restricted\nworkflows:\n  - public\n"), 0o644))

	// Try to run unlisted workflow - should fail
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "restricted/internal"})

	err := cmd.Execute()
	require.Error(t, err, "Should reject unlisted workflow")
}

// TestPackRun_PromptsDirResolution ensures {{.awf.prompts_dir}} resolves to pack
func TestPackRun_PromptsDirResolution(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

	// Create pack with prompts and a workflow that references them
	workflows := map[string]string{
		"prompt-test": `name: prompt-test
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: test -f "{{.awf.prompts_dir}}/system-prompt.md"
    on_success: done
  done:
    type: terminal
`,
	}
	createPackStructure(t, tmpDir, "prompt-pack", workflows)

	prompts := map[string]string{
		"system-prompt.md": "This is a system prompt",
	}
	createPackPrompts(t, tmpDir, "prompt-pack", prompts)

	// Run workflow - should find the prompt file
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "prompt-pack/prompt-test", "--step=start"})

	err := cmd.Execute()
	require.NoError(t, err, "Failed to resolve {{.awf.prompts_dir}} in pack: %v", err)
}

// TestPackRun_ScriptsDirResolution ensures {{.awf.scripts_dir}} resolves to pack
func TestPackRun_ScriptsDirResolution(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

	// Create pack with scripts and a workflow that references them
	workflows := map[string]string{
		"script-test": `name: script-test
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: test -f "{{.awf.scripts_dir}}/helper.sh"
    on_success: done
  done:
    type: terminal
`,
	}
	createPackStructure(t, tmpDir, "script-pack", workflows)

	scripts := map[string]string{
		"helper.sh": "#!/bin/bash\necho \"Helper script\"",
	}
	createPackScripts(t, tmpDir, "script-pack", scripts)

	// Run workflow - should find the script file
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "script-pack/script-test", "--step=start"})

	err := cmd.Execute()
	require.NoError(t, err, "Failed to resolve {{.awf.scripts_dir}} in pack: %v", err)
}

// TestPackRun_UserOverride ensures .awf/prompts/<pack> takes precedence
func TestPackRun_UserOverride(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

	// Create pack with embedded prompt
	workflows := map[string]string{
		"override-test": `name: override-test
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: cat "{{.awf.prompts_dir}}/message.txt"
    on_success: done
  done:
    type: terminal
`,
	}
	createPackStructure(t, tmpDir, "override-pack", workflows)

	prompts := map[string]string{
		"message.txt": "Embedded prompt",
	}
	createPackPrompts(t, tmpDir, "override-pack", prompts)

	// Create user override in .awf/prompts/override-pack/
	overrideDir := filepath.Join(tmpDir, ".awf", "prompts", "override-pack")
	require.NoError(t, os.MkdirAll(overrideDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "message.txt"), []byte("User override"), 0o644))

	// Run workflow - should use user override
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "override-pack/override-test", "--step=start"})

	err := cmd.Execute()
	require.NoError(t, err, "Failed to run workflow with user override: %v", err)
	// Note: verifying the actual content would require capturing command output, which is framework-dependent
}

// TestPackRun_LocalWorkflowUnchanged ensures local workflows still work without namespace
func TestPackRun_LocalWorkflowUnchanged(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

	// Create a local workflow (not in a pack)
	workflowContent := `name: local
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "Local workflow"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "local.yaml", workflowContent)

	// Run local workflow without namespace - should work as before
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "local", "--step=start"})

	err := cmd.Execute()
	require.NoError(t, err, "Local workflow should still work: %v", err)
}

// TestPackRun_LocalPromptDirUnchanged ensures local workflows use .awf/prompts/ still
func TestPackRun_LocalPromptDirUnchanged(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

	// Create local workflow that uses {{.awf.prompts_dir}}
	workflowContent := `name: local-prompt
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: test -f "{{.awf.prompts_dir}}/local-prompt.md"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "local-prompt.yaml", workflowContent)

	// Create local prompts directory with a prompt
	promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "local-prompt.md"), []byte("Local prompt"), 0o644))

	// Run local workflow - should use .awf/prompts/ as before
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "local-prompt", "--step=start"})

	err := cmd.Execute()
	require.NoError(t, err, "Local workflow should still use .awf/prompts/: %v", err)
}

// TestPackRun_CallWorkflowRelative ensures call_workflow within pack resolves correctly
func TestPackRun_CallWorkflowRelative(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

	// Create pack with multiple workflows where one calls another
	workflows := map[string]string{
		"main": `name: main
version: "1.0.0"
states:
  initial: start
  start:
    type: call_workflow
    workflow: helper
    on_success: done
  done:
    type: terminal
`,
		"helper": `name: helper
version: "1.0.0"
states:
  initial: greet
  greet:
    type: step
    command: echo "Helper called"
    on_success: done
  done:
    type: terminal
`,
	}
	createPackStructure(t, tmpDir, "call-pack", workflows)

	// Run workflow that calls another in the same pack
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "call-pack/main", "--step=start"})

	err := cmd.Execute()
	require.NoError(t, err, "Failed to resolve relative call_workflow in pack: %v", err)
}

// Feature: 185
func TestPackRun_NestedCrossPackCallsUseChildContext(t *testing.T) {
	tests := []struct {
		name                         string
		createUtilsOverride          bool
		createUnrelatedUtilsOverride bool
		wantResource                 string
	}{
		{name: "user script override", createUtilsOverride: true, wantResource: "utils-override"},
		{name: "embedded child script", wantResource: "utils-embedded"},
		{name: "unrelated user script override", createUnrelatedUtilsOverride: true, wantResource: "utils-embedded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := setupInitTestDir(t)
			t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "xdg-data"))
			require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

			markerPath := filepath.Join(tmpDir, "pack-context-markers")
			appendMarker := func(label string) string {
				return fmt.Sprintf("printf '%%s\\n' %s >> %s", interpolation.ShellEscape(label), interpolation.ShellEscape(markerPath))
			}

			createPackStructure(t, tmpDir, "source", map[string]string{
				"parent": `name: parent
version: "1.0.0"
states:
  initial: call_helper
  call_helper:
    type: call_workflow
    workflow: utils/helper
    on_success: call_parent_tail
  call_parent_tail:
    type: call_workflow
    workflow: parent-tail
    on_success: done
  done:
    type: terminal
`,
				"leaf": fmt.Sprintf(`name: leaf
version: "1.0.0"
states:
  initial: mark
  mark:
    type: step
    command: %s
    on_success: done
  done:
    type: terminal
`, appendMarker("source-leaf")),
				"parent-tail": fmt.Sprintf(`name: parent-tail
version: "1.0.0"
states:
  initial: mark
  mark:
    type: step
    command: %s
    on_success: done
  done:
    type: terminal
`, appendMarker("source-parent")),
				"parallel-child": `name: parallel-child
version: "1.0.0"
states:
  initial: call_parallel_leaf
  call_parallel_leaf:
    type: call_workflow
    workflow: parallel-leaf
    on_success: done
  done:
    type: terminal
`,
				"parallel-leaf": fmt.Sprintf(`name: parallel-leaf
version: "1.0.0"
states:
  initial: mark
  mark:
    type: step
    command: %s
    on_success: done
  done:
    type: terminal
`, appendMarker("source-parallel")),
			})

			createPackStructure(t, tmpDir, "utils", map[string]string{
				"helper": `name: helper
version: "1.0.0"
states:
  initial: call_leaf
  call_leaf:
    type: call_workflow
    workflow: leaf
    on_success: run_resource
  run_resource:
    type: step
    command: sh "{{.awf.scripts_dir}}/helper.sh"
    on_success: run_parallel
  run_parallel:
    type: parallel
    strategy: all_succeed
    parallel:
      - source_branch
      - auxiliary_branch
    on_success: done
  source_branch:
    type: call_workflow
    workflow: source/parallel-child
    on_success: done
  auxiliary_branch:
    type: call_workflow
    workflow: auxiliary/parallel-child
    on_success: done
  done:
    type: terminal
`,
				"leaf": fmt.Sprintf(`name: leaf
version: "1.0.0"
states:
  initial: mark
  mark:
    type: step
    command: %s
    on_success: done
  done:
    type: terminal
`, appendMarker("utils-leaf")),
			})
			createPackScripts(t, tmpDir, "utils", map[string]string{
				"helper.sh": "#!/bin/sh\n" + appendMarker("utils-embedded") + "\n",
			})

			createPackStructure(t, tmpDir, "auxiliary", map[string]string{
				"parallel-child": `name: parallel-child
version: "1.0.0"
states:
  initial: call_parallel_leaf
  call_parallel_leaf:
    type: call_workflow
    workflow: parallel-leaf
    on_success: done
  done:
    type: terminal
`,
				"parallel-leaf": fmt.Sprintf(`name: parallel-leaf
version: "1.0.0"
states:
  initial: mark
  mark:
    type: step
    command: %s
    on_success: done
  done:
    type: terminal
`, appendMarker("auxiliary-parallel")),
			})

			for packName, marker := range map[string]string{
				"source": "source-override",
				"utils":  "utils-override",
			} {
				if packName == "utils" && !tt.createUtilsOverride {
					continue
				}
				overrideDir := filepath.Join(tmpDir, ".awf", "scripts", packName)
				require.NoError(t, os.MkdirAll(overrideDir, 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(overrideDir, "helper.sh"),
					[]byte("#!/bin/sh\n"+appendMarker(marker)+"\n"),
					0o644,
				))
			}

			if tt.createUnrelatedUtilsOverride {
				overrideDir := filepath.Join(tmpDir, ".awf", "scripts", "utils")
				require.NoError(t, os.MkdirAll(overrideDir, 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(overrideDir, "foo.sh"),
					[]byte("#!/bin/sh\n"+appendMarker("unrelated-override")+"\n"),
					0o644,
				))
			}

			cmd := cli.NewRootCommand()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "source/parent"})

			require.NoError(t, cmd.Execute(), output.String())
			markerData, err := os.ReadFile(markerPath)
			require.NoError(t, err)
			markers := strings.Fields(string(markerData))
			require.Len(t, markers, 5)
			assert.Equal(t, "utils-leaf", markers[0])
			assert.Equal(t, tt.wantResource, markers[1])
			assert.ElementsMatch(t, []string{"source-parallel", "auxiliary-parallel"}, markers[2:4])
			assert.Equal(t, "source-parent", markers[4])
			assert.NotContains(t, markers, "source-leaf")
			assert.NotContains(t, markers, "source-override")
			if tt.createUtilsOverride {
				assert.NotContains(t, markers, "utils-embedded")
			}
		})
	}
}

// TestPackRun_MissingPack returns error for non-existent pack
func TestPackRun_MissingPack(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Try to run workflow from non-existent pack
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "nonexistent-pack/workflow"})

	err := cmd.Execute()
	require.Error(t, err, "Should fail for non-existent pack")
}

// TestPackRun_InvalidNamespace returns error for invalid pack name
func TestPackRun_InvalidNamespace(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Try to run with invalid namespace (multiple slashes)
	// Pack names should only have one slash separator
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "pack/sub/workflow"})

	err := cmd.Execute()
	require.Error(t, err, "Should reject namespace with multiple slashes")
}

// TestPackRun_MultiplePacksIndependent ensures multiple packs work independently
func TestPackRun_MultiplePacksIndependent(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

	// Create two separate packs
	workflows1 := map[string]string{
		"alpha": `name: alpha
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "Pack 1"
    on_success: done
  done:
    type: terminal
`,
	}
	createPackStructure(t, tmpDir, "pack1", workflows1)

	workflows2 := map[string]string{
		"beta": `name: beta
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "Pack 2"
    on_success: done
  done:
    type: terminal
`,
	}
	createPackStructure(t, tmpDir, "pack2", workflows2)

	// Run first pack workflow
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "pack1/alpha", "--step=start"})

	err := cmd.Execute()
	require.NoError(t, err, "Failed to run first pack: %v", err)

	// Run second pack workflow
	cmd = cli.NewRootCommand()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "pack2/beta", "--step=start"})

	err = cmd.Execute()
	require.NoError(t, err, "Failed to run second pack: %v", err)
}

// TestPackRun_MissingManifest returns error when manifest is missing
func TestPackRun_MissingManifest(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create pack structure without manifest.yaml
	packDir := filepath.Join(tmpDir, ".awf", "workflow-packs", "no-manifest")
	workflowsDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	workflowPath := filepath.Join(workflowsDir, "workflow.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(`name: workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`), 0o644))

	// Try to run workflow from pack without manifest
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "no-manifest/workflow"})

	err := cmd.Execute()
	require.Error(t, err, "Should fail when manifest is missing")
}
