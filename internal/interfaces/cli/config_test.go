package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type routingTestFacade struct{}

type recordedHistoryStore struct {
	events *[]string
}

func (*recordedHistoryStore) Record(context.Context, *workflow.ExecutionRecord) error { return nil }

func (*recordedHistoryStore) List(context.Context, *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	return nil, nil
}

func (*recordedHistoryStore) GetStats(context.Context, *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	return nil, nil
}

func (*recordedHistoryStore) Cleanup(context.Context, time.Duration) (int, error) { return 0, nil }

func (s *recordedHistoryStore) Close() error {
	*s.events = append(*s.events, "history-close")
	return nil
}

func (routingTestFacade) List(context.Context) ([]ports.WorkflowSummary, error) { return nil, nil }

func (routingTestFacade) Validate(context.Context, ports.RunRequest) (ports.ValidationReport, error) {
	return ports.ValidationReport{}, nil
}

func (routingTestFacade) Status(context.Context, string) (ports.RunStatus, error) {
	return ports.RunStatus{RunID: "run-1"}, nil
}

func (routingTestFacade) History(context.Context, ports.HistoryFilter) ([]ports.RunRecord, error) {
	return nil, nil
}

func (routingTestFacade) Run(context.Context, ports.RunRequest) (ports.RunSession, error) {
	return nil, nil
}

func (routingTestFacade) Resume(context.Context, ports.ResumeRequest) (ports.RunSession, error) {
	return nil, nil
}

func TestValidateCommand_IsOnlyPluginAwareFacadeRoute(t *testing.T) {
	cmd := NewRootCommand()
	var marked []string
	var visit func(*cobra.Command)
	visit = func(current *cobra.Command) {
		if current.Annotations[annotationPluginAwareValidation] == "true" {
			marked = append(marked, current.CommandPath())
		}
		for _, child := range current.Commands() {
			visit(child)
		}
	}
	visit(cmd)

	assert.Equal(t, []string{"awf validate"}, marked)
}

func TestAutoFacade_ValidationSetupErrorPreventsRunE(t *testing.T) {
	t.Run("setup error", func(t *testing.T) {
		setupErr := errors.New("validation setup failed")
		cleanupCalls := 0
		runCalls := 0
		cfg := DefaultConfig()
		cfg.StoragePath = t.TempDir()
		cmd, cleanup := newRootCommandWithBuilders(
			cfg,
			true,
			func(*Config) (ports.WorkflowFacade, func()) { return routingTestFacade{}, func() {} },
			func(*Config) (ports.WorkflowFacade, func(), error) {
				return nil, func() { cleanupCalls++ }, setupErr
			},
		)
		validateCmd, _, findErr := cmd.Find([]string{"validate"})
		require.NoError(t, findErr)
		validateCmd.RunE = func(*cobra.Command, []string) error {
			runCalls++
			return errors.New("validate RunE invoked")
		}
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"validate", "workflow"})

		err := cmd.Execute()

		require.ErrorIs(t, err, setupErr)
		assert.ErrorContains(t, err, "initialize validation facade")
		assert.Zero(t, runCalls)
		assert.Equal(t, 1, cleanupCalls)
		cleanup()
		cleanup()
		assert.Equal(t, 1, cleanupCalls)
	})

	t.Run("successful setup cleanup", func(t *testing.T) {
		cleanupCalls := 0
		runCalls := 0
		home := t.TempDir()
		t.Setenv("HOME", home)
		require.NoError(t, os.Mkdir(filepath.Join(home, ".awf"), 0o700))
		migrationNoticeMu.Lock()
		migrationNoticeShown = false
		migrationNoticeMu.Unlock()
		t.Cleanup(func() {
			migrationNoticeMu.Lock()
			migrationNoticeShown = false
			migrationNoticeMu.Unlock()
		})
		cfg := DefaultConfig()
		cfg.StoragePath = t.TempDir()
		cmd, cleanup := newRootCommandWithBuilders(
			cfg,
			true,
			func(*Config) (ports.WorkflowFacade, func()) { return routingTestFacade{}, func() {} },
			func(*Config) (ports.WorkflowFacade, func(), error) {
				return routingTestFacade{}, func() { cleanupCalls++ }, nil
			},
		)
		validateCmd, _, findErr := cmd.Find([]string{"validate"})
		require.NoError(t, findErr)
		validateCmd.RunE = func(*cobra.Command, []string) error {
			runCalls++
			return nil
		}
		output := new(bytes.Buffer)
		cmd.SetOut(output)
		cmd.SetErr(output)
		cmd.SetArgs([]string{"--format", "json", "validate", "workflow"})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, 1, runCalls)
		assert.Equal(t, ui.FormatJSON, cfg.OutputFormat)
		assert.Equal(t, 1, strings.Count(output.String(), "NOTICE: Legacy ~/.awf directory detected"))
		assert.Zero(t, cleanupCalls)
		cleanup()
		cleanup()
		assert.Equal(t, 1, cleanupCalls)
	})
}

func TestValidationFacade_ReleasesPluginResources(t *testing.T) {
	t.Run("plugin initialization error", func(t *testing.T) {
		events := []string{}
		initErr := errors.New("plugin initialization failed")
		facade, cleanup, err := buildValidationFacadeWithDependencies(DefaultConfig(), validationFacadeDependencies{
			initPlugins: func(_ context.Context, _ *Config, logger ports.Logger) (*PluginSystemResult, error) {
				require.NotPanics(t, func() { logger.Warn("synthetic plugin initialization warning") })
				events = append(events, "plugin-init", "plugin-partial-release")
				return nil, initErr
			},
			openHistory: func(string) (ports.HistoryStore, error) {
				events = append(events, "history-open")
				return nil, nil
			},
		})

		require.ErrorIs(t, err, initErr)
		assert.Nil(t, facade)
		cleanup()
		cleanup()
		assert.Equal(t, []string{"plugin-init", "plugin-partial-release"}, events)
	})

	t.Run("history acquisition error", func(t *testing.T) {
		events := []string{}
		historyErr := errors.New("history initialization failed")
		facade, cleanup, err := buildValidationFacadeWithDependencies(DefaultConfig(), validationFacadeDependencies{
			initPlugins: func(context.Context, *Config, ports.Logger) (*PluginSystemResult, error) {
				events = append(events, "plugin-init")
				return &PluginSystemResult{
					Cleanup: func() { events = append(events, "plugin-cleanup") },
				}, nil
			},
			openHistory: func(string) (ports.HistoryStore, error) {
				events = append(events, "history-open")
				return nil, historyErr
			},
		})

		require.ErrorIs(t, err, historyErr)
		assert.Nil(t, facade)
		cleanup()
		cleanup()
		assert.Equal(t, []string{"plugin-init", "history-open", "plugin-cleanup"}, events)
	})

	t.Run("successful cleanup is idempotent", func(t *testing.T) {
		events := []string{}
		facade, cleanup, err := buildValidationFacadeWithDependencies(DefaultConfig(), validationFacadeDependencies{
			initPlugins: func(context.Context, *Config, ports.Logger) (*PluginSystemResult, error) {
				events = append(events, "plugin-init")
				return &PluginSystemResult{
					Service: application.NewPluginService(nil, nil, ports.NopLogger{}),
					Cleanup: func() { events = append(events, "plugin-cleanup") },
				}, nil
			},
			openHistory: func(string) (ports.HistoryStore, error) {
				events = append(events, "history-open")
				return &recordedHistoryStore{events: &events}, nil
			},
		})

		require.NoError(t, err)
		require.NotNil(t, facade)
		cleanup()
		cleanup()
		assert.Equal(t, []string{"plugin-init", "history-open", "plugin-cleanup", "history-close"}, events)
	})
}

func TestAutoFacade_ReadCommandsDoNotInitializePlugins(t *testing.T) {
	pluginsDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(pluginsDir, "invalid-plugin.yaml"), []byte("not: [valid"), 0o600))
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	tests := []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"list"}},
		{name: "history", args: []string{"history"}},
		{name: "status", args: []string{"status", "run-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationBuilds := 0
			ordinaryBuilds := 0
			cfg := DefaultConfig()
			cfg.StoragePath = t.TempDir()
			cmd, cleanup := newRootCommandWithBuilders(
				cfg,
				true,
				func(*Config) (ports.WorkflowFacade, func()) {
					ordinaryBuilds++
					return routingTestFacade{}, func() {}
				},
				func(*Config) (ports.WorkflowFacade, func(), error) {
					validationBuilds++
					return nil, func() {}, errors.New("plugin setup must not run")
				},
			)
			defer cleanup()
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			require.NoError(t, err)
			assert.Equal(t, 1, ordinaryBuilds)
			assert.Zero(t, validationBuilds)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Verbose {
		t.Error("expected Verbose to be false by default")
	}
	if cfg.Quiet {
		t.Error("expected Quiet to be false by default")
	}
	if cfg.NoColor {
		t.Error("expected NoColor to be false by default")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel 'info', got '%s'", cfg.LogLevel)
	}
	if !strings.Contains(cfg.StoragePath, "awf") {
		t.Errorf("expected StoragePath to contain 'awf', got '%s'", cfg.StoragePath)
	}
	if cfg.OutputMode != OutputSilent {
		t.Errorf("expected OutputMode to be silent by default, got %v", cfg.OutputMode)
	}
}

func TestOutputMode_String(t *testing.T) {
	tests := []struct {
		mode OutputMode
		want string
	}{
		{OutputSilent, "silent"},
		{OutputStreaming, "streaming"},
		{OutputBuffered, "buffered"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.mode.String())
	}
}

func TestParseOutputMode(t *testing.T) {
	tests := []struct {
		input   string
		want    OutputMode
		wantErr bool
	}{
		{"silent", OutputSilent, false},
		{"streaming", OutputStreaming, false},
		{"buffered", OutputBuffered, false},
		{"invalid", OutputSilent, true},
		{"", OutputSilent, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseOutputMode(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestOutputFormat_String(t *testing.T) {
	tests := []struct {
		format ui.OutputFormat
		want   string
	}{
		{ui.FormatText, "text"},
		{ui.FormatJSON, "json"},
		{ui.FormatTable, "table"},
		{ui.FormatQuiet, "quiet"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.format.String())
	}
}

func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		input   string
		want    ui.OutputFormat
		wantErr bool
	}{
		{"text", ui.FormatText, false},
		{"json", ui.FormatJSON, false},
		{"table", ui.FormatTable, false},
		{"quiet", ui.FormatQuiet, false},
		{"", ui.FormatText, false}, // default to text
		{"yaml", ui.FormatText, true},
		{"XML", ui.FormatText, true},
		{"invalid", ui.FormatText, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ui.ParseOutputFormat(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid output format")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestBuildPromptPaths_ReturnsCorrectNumberOfPaths(t *testing.T) {
	// BuildPromptPaths should return exactly 2 paths: local and global
	// (no AWF_PROMPTS_PATH env var support per ADR-002)
	paths := BuildPromptPaths()

	require.Len(t, paths, 2, "BuildPromptPaths should return exactly 2 paths (local + global)")
}

func TestBuildPromptPaths_LocalPathFirst(t *testing.T) {
	// FR-001: Local prompts have highest priority
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	assert.Equal(t, repository.SourceLocal, paths[0].Source, "First path should be local source")
	assert.Equal(t, xdg.LocalPromptsDir(), paths[0].Path, "First path should be local prompts directory")
}

func TestBuildPromptPaths_GlobalPathSecond(t *testing.T) {
	// FR-001: Global prompts are searched after local
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	assert.Equal(t, repository.SourceGlobal, paths[1].Source, "Second path should be global source")
	assert.Equal(t, xdg.AWFPromptsDir(), paths[1].Path, "Second path should be XDG prompts directory")
}

func TestBuildPromptPaths_PriorityOrder(t *testing.T) {
	// Comprehensive test: verify priority order matches spec FR-001
	// Order: 1. .awf/prompts/ (local), 2. $XDG_CONFIG_HOME/awf/prompts/ (global)
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)

	// Verify complete ordering
	expectedOrder := []struct {
		source repository.Source
		path   string
	}{
		{repository.SourceLocal, xdg.LocalPromptsDir()},
		{repository.SourceGlobal, xdg.AWFPromptsDir()},
	}

	for i, expected := range expectedOrder {
		assert.Equal(t, expected.source, paths[i].Source,
			"Path %d should have source %v", i, expected.source)
		assert.Equal(t, expected.path, paths[i].Path,
			"Path %d should have path %s", i, expected.path)
	}
}

func TestBuildPromptPaths_LocalPathIsRelative(t *testing.T) {
	// Local prompts dir should be relative (project-specific)
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	localPath := paths[0].Path

	assert.False(t, filepath.IsAbs(localPath),
		"Local prompts path should be relative, got: %s", localPath)
	assert.Equal(t, ".awf/prompts", localPath,
		"Local prompts path should be .awf/prompts")
}

func TestBuildPromptPaths_GlobalPathIsAbsolute(t *testing.T) {
	// Global prompts dir should be absolute (user-level)
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	globalPath := paths[1].Path

	assert.True(t, filepath.IsAbs(globalPath),
		"Global prompts path should be absolute, got: %s", globalPath)
}

func TestBuildPromptPaths_GlobalPathContainsAwfPrompts(t *testing.T) {
	// Verify global path structure contains awf/prompts
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	globalPath := paths[1].Path

	assert.Contains(t, globalPath, "awf",
		"Global path should contain 'awf'")
	assert.True(t, strings.HasSuffix(globalPath, filepath.Join("awf", "prompts")),
		"Global path should end with awf/prompts, got: %s", globalPath)
}

func TestBuildPromptPaths_RespectsXDGConfigHome(t *testing.T) {
	// FR-002: XDG_CONFIG_HOME environment variable is respected
	// Set custom XDG_CONFIG_HOME
	customConfig := "/custom/config/path"
	t.Setenv("XDG_CONFIG_HOME", customConfig)

	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	expectedGlobalPath := filepath.Join(customConfig, "awf", "prompts")
	assert.Equal(t, expectedGlobalPath, paths[1].Path,
		"Global path should respect XDG_CONFIG_HOME")
}

func TestBuildPromptPaths_DefaultsToHomeConfig(t *testing.T) {
	// FR-002: Defaults to ~/.config when XDG_CONFIG_HOME is not set
	// Unset XDG_CONFIG_HOME
	t.Setenv("XDG_CONFIG_HOME", "")

	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	homeDir, _ := os.UserHomeDir()
	expectedGlobalPath := filepath.Join(homeDir, ".config", "awf", "prompts")
	assert.Equal(t, expectedGlobalPath, paths[1].Path,
		"Global path should default to ~/.config/awf/prompts")
}

func TestBuildPromptPaths_ConsistentResults(t *testing.T) {
	// Multiple calls should return consistent results
	paths1 := BuildPromptPaths()
	paths2 := BuildPromptPaths()

	require.Len(t, paths1, 2)
	require.Len(t, paths2, 2)

	for i := range paths1 {
		assert.Equal(t, paths1[i].Path, paths2[i].Path,
			"Path %d should be consistent across calls", i)
		assert.Equal(t, paths1[i].Source, paths2[i].Source,
			"Source %d should be consistent across calls", i)
	}
}

func TestBuildPromptPaths_SourcedPathStructure(t *testing.T) {
	// Verify returned slice contains properly structured SourcedPath values
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)

	for i, path := range paths {
		// Each path should have non-empty Path field
		assert.NotEmpty(t, path.Path,
			"SourcedPath[%d] should have non-empty Path", i)

		// Source should be a valid Source type
		assert.True(t, path.Source == repository.SourceLocal || path.Source == repository.SourceGlobal,
			"SourcedPath[%d] should have valid Source (Local or Global)", i)
	}
}

func TestBuildPromptPaths_MirrorsWorkflowPathsPattern(t *testing.T) {
	// ADR-003: BuildPromptPaths follows same pattern as BuildWorkflowPaths
	// Both should return paths with same source types in same order (excluding env var)

	promptPaths := BuildPromptPaths()
	workflowPaths := BuildWorkflowPaths()

	// Prompt paths: local, global (2 paths)
	// Workflow paths: env (if set), local, global (2-3 paths)
	require.Len(t, promptPaths, 2)

	// Find matching sources in workflow paths
	workflowLocalIdx, workflowGlobalIdx := -1, -1
	for i, wp := range workflowPaths {
		switch wp.Source {
		case repository.SourceLocal:
			workflowLocalIdx = i
		case repository.SourceGlobal:
			workflowGlobalIdx = i
		case repository.SourceEnv:
			// SourceEnv not used in workflow paths in this test
		}
	}

	// Verify local comes before global in both
	if workflowLocalIdx != -1 && workflowGlobalIdx != -1 {
		assert.Less(t, workflowLocalIdx, workflowGlobalIdx,
			"In BuildWorkflowPaths, local should come before global")
	}

	// Prompt paths: local at 0, global at 1
	assert.Equal(t, repository.SourceLocal, promptPaths[0].Source)
	assert.Equal(t, repository.SourceGlobal, promptPaths[1].Source)
}

func TestBuildPromptPaths_TableDriven(t *testing.T) {
	// Table-driven test for various XDG_CONFIG_HOME scenarios
	tests := []struct {
		name           string
		xdgConfigHome  string
		expectLocalDir string
		expectGlobal   func() string // function to compute expected global path
	}{
		{
			name:           "default_xdg_unset",
			xdgConfigHome:  "",
			expectLocalDir: ".awf/prompts",
			expectGlobal: func() string {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, ".config", "awf", "prompts")
			},
		},
		{
			name:           "custom_xdg_config_home",
			xdgConfigHome:  "/tmp/custom-xdg",
			expectLocalDir: ".awf/prompts",
			expectGlobal: func() string {
				return filepath.Join("/tmp/custom-xdg", "awf", "prompts")
			},
		},
		{
			name:           "xdg_with_trailing_slash",
			xdgConfigHome:  "/opt/config/",
			expectLocalDir: ".awf/prompts",
			expectGlobal: func() string {
				return filepath.Join("/opt/config/", "awf", "prompts")
			},
		},
		{
			name:           "xdg_with_spaces_in_path",
			xdgConfigHome:  "/home/user/My Config",
			expectLocalDir: ".awf/prompts",
			expectGlobal: func() string {
				return filepath.Join("/home/user/My Config", "awf", "prompts")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore XDG_CONFIG_HOME
			t.Setenv("XDG_CONFIG_HOME", tt.xdgConfigHome)

			paths := BuildPromptPaths()

			require.Len(t, paths, 2)
			assert.Equal(t, tt.expectLocalDir, paths[0].Path)
			assert.Equal(t, tt.expectGlobal(), paths[1].Path)
		})
	}
}
