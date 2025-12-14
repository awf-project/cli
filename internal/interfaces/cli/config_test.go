package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/infrastructure/xdg"
	"github.com/vanoix/awf/internal/interfaces/cli"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

func TestDefaultConfig(t *testing.T) {
	cfg := cli.DefaultConfig()

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
	if cfg.OutputMode != cli.OutputSilent {
		t.Errorf("expected OutputMode to be silent by default, got %v", cfg.OutputMode)
	}
}

func TestOutputMode_String(t *testing.T) {
	tests := []struct {
		mode cli.OutputMode
		want string
	}{
		{cli.OutputSilent, "silent"},
		{cli.OutputStreaming, "streaming"},
		{cli.OutputBuffered, "buffered"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.mode.String())
	}
}

func TestParseOutputMode(t *testing.T) {
	tests := []struct {
		input   string
		want    cli.OutputMode
		wantErr bool
	}{
		{"silent", cli.OutputSilent, false},
		{"streaming", cli.OutputStreaming, false},
		{"buffered", cli.OutputBuffered, false},
		{"invalid", cli.OutputSilent, true},
		{"", cli.OutputSilent, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := cli.ParseOutputMode(tt.input)
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

// =============================================================================
// BuildPromptPaths Tests (T004)
// =============================================================================

func TestBuildPromptPaths_ReturnsCorrectNumberOfPaths(t *testing.T) {
	// BuildPromptPaths should return exactly 2 paths: local and global
	// (no AWF_PROMPTS_PATH env var support per ADR-002)
	paths := cli.BuildPromptPaths()

	require.Len(t, paths, 2, "BuildPromptPaths should return exactly 2 paths (local + global)")
}

func TestBuildPromptPaths_LocalPathFirst(t *testing.T) {
	// FR-001: Local prompts have highest priority
	paths := cli.BuildPromptPaths()

	require.Len(t, paths, 2)
	assert.Equal(t, repository.SourceLocal, paths[0].Source, "First path should be local source")
	assert.Equal(t, xdg.LocalPromptsDir(), paths[0].Path, "First path should be local prompts directory")
}

func TestBuildPromptPaths_GlobalPathSecond(t *testing.T) {
	// FR-001: Global prompts are searched after local
	paths := cli.BuildPromptPaths()

	require.Len(t, paths, 2)
	assert.Equal(t, repository.SourceGlobal, paths[1].Source, "Second path should be global source")
	assert.Equal(t, xdg.AWFPromptsDir(), paths[1].Path, "Second path should be XDG prompts directory")
}

func TestBuildPromptPaths_PriorityOrder(t *testing.T) {
	// Comprehensive test: verify priority order matches spec FR-001
	// Order: 1. .awf/prompts/ (local), 2. $XDG_CONFIG_HOME/awf/prompts/ (global)
	paths := cli.BuildPromptPaths()

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
	paths := cli.BuildPromptPaths()

	require.Len(t, paths, 2)
	localPath := paths[0].Path

	assert.False(t, filepath.IsAbs(localPath),
		"Local prompts path should be relative, got: %s", localPath)
	assert.Equal(t, ".awf/prompts", localPath,
		"Local prompts path should be .awf/prompts")
}

func TestBuildPromptPaths_GlobalPathIsAbsolute(t *testing.T) {
	// Global prompts dir should be absolute (user-level)
	paths := cli.BuildPromptPaths()

	require.Len(t, paths, 2)
	globalPath := paths[1].Path

	assert.True(t, filepath.IsAbs(globalPath),
		"Global prompts path should be absolute, got: %s", globalPath)
}

func TestBuildPromptPaths_GlobalPathContainsAwfPrompts(t *testing.T) {
	// Verify global path structure contains awf/prompts
	paths := cli.BuildPromptPaths()

	require.Len(t, paths, 2)
	globalPath := paths[1].Path

	assert.Contains(t, globalPath, "awf",
		"Global path should contain 'awf'")
	assert.True(t, strings.HasSuffix(globalPath, filepath.Join("awf", "prompts")),
		"Global path should end with awf/prompts, got: %s", globalPath)
}

func TestBuildPromptPaths_RespectsXDGConfigHome(t *testing.T) {
	// FR-002: XDG_CONFIG_HOME environment variable is respected
	// Save and restore original env
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if originalXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	// Set custom XDG_CONFIG_HOME
	customConfig := "/custom/config/path"
	os.Setenv("XDG_CONFIG_HOME", customConfig)

	paths := cli.BuildPromptPaths()

	require.Len(t, paths, 2)
	expectedGlobalPath := filepath.Join(customConfig, "awf", "prompts")
	assert.Equal(t, expectedGlobalPath, paths[1].Path,
		"Global path should respect XDG_CONFIG_HOME")
}

func TestBuildPromptPaths_DefaultsToHomeConfig(t *testing.T) {
	// FR-002: Defaults to ~/.config when XDG_CONFIG_HOME is not set
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if originalXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	// Unset XDG_CONFIG_HOME
	os.Unsetenv("XDG_CONFIG_HOME")

	paths := cli.BuildPromptPaths()

	require.Len(t, paths, 2)
	homeDir, _ := os.UserHomeDir()
	expectedGlobalPath := filepath.Join(homeDir, ".config", "awf", "prompts")
	assert.Equal(t, expectedGlobalPath, paths[1].Path,
		"Global path should default to ~/.config/awf/prompts")
}

func TestBuildPromptPaths_ConsistentResults(t *testing.T) {
	// Multiple calls should return consistent results
	paths1 := cli.BuildPromptPaths()
	paths2 := cli.BuildPromptPaths()

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
	paths := cli.BuildPromptPaths()

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

	promptPaths := cli.BuildPromptPaths()
	workflowPaths := cli.BuildWorkflowPaths()

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
			originalXDG := os.Getenv("XDG_CONFIG_HOME")
			defer func() {
				if originalXDG != "" {
					os.Setenv("XDG_CONFIG_HOME", originalXDG)
				} else {
					os.Unsetenv("XDG_CONFIG_HOME")
				}
			}()

			if tt.xdgConfigHome != "" {
				os.Setenv("XDG_CONFIG_HOME", tt.xdgConfigHome)
			} else {
				os.Unsetenv("XDG_CONFIG_HOME")
			}

			paths := cli.BuildPromptPaths()

			require.Len(t, paths, 2)
			assert.Equal(t, tt.expectLocalDir, paths[0].Path)
			assert.Equal(t, tt.expectGlobal(), paths[1].Path)
		})
	}
}
