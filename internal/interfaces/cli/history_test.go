package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "history" {
			found = true
			break
		}
	}

	assert.True(t, found, "expected root command to have 'history' subcommand")
}

func TestHistoryCommand_Flags(t *testing.T) {
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	tests := []struct {
		name      string
		flagName  string
		shorthand string
		defValue  string
	}{
		{"workflow flag", "workflow", "w", ""},
		{"status flag", "status", "s", ""},
		{"since flag", "since", "", ""},
		{"limit flag", "limit", "n", "20"},
		{"stats flag", "stats", "", "false"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flag := historyCmd.Flags().Lookup(tc.flagName)
			require.NotNil(t, flag, "flag %s should exist", tc.flagName)
			assert.Equal(t, tc.defValue, flag.DefValue, "default value mismatch for %s", tc.flagName)
			if tc.shorthand != "" {
				assert.Equal(t, tc.shorthand, flag.Shorthand, "shorthand mismatch for %s", tc.flagName)
			}
		})
	}
}

func TestHistoryCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"history", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "history")
	assert.Contains(t, output, "--workflow")
	assert.Contains(t, output, "--status")
	assert.Contains(t, output, "--since")
	assert.Contains(t, output, "--limit")
	assert.Contains(t, output, "--stats")
}

func TestHistoryCommand_Description(t *testing.T) {
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	assert.NotEmpty(t, historyCmd.Short, "short description should be set")
	assert.NotEmpty(t, historyCmd.Long, "long description should be set")
	assert.Contains(t, historyCmd.Long, "awf history", "long description should contain usage examples")
}

func TestHistoryCommand_FlagTypes(t *testing.T) {
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	// Test that flags have correct types
	t.Run("workflow flag is string", func(t *testing.T) {
		flag := historyCmd.Flags().Lookup("workflow")
		require.NotNil(t, flag)
		assert.Equal(t, "string", flag.Value.Type())
	})

	t.Run("status flag is string", func(t *testing.T) {
		flag := historyCmd.Flags().Lookup("status")
		require.NotNil(t, flag)
		assert.Equal(t, "string", flag.Value.Type())
	})

	t.Run("since flag is string", func(t *testing.T) {
		flag := historyCmd.Flags().Lookup("since")
		require.NotNil(t, flag)
		assert.Equal(t, "string", flag.Value.Type())
	})

	t.Run("limit flag is int", func(t *testing.T) {
		flag := historyCmd.Flags().Lookup("limit")
		require.NotNil(t, flag)
		assert.Equal(t, "int", flag.Value.Type())
	})

	t.Run("stats flag is bool", func(t *testing.T) {
		flag := historyCmd.Flags().Lookup("stats")
		require.NotNil(t, flag)
		assert.Equal(t, "bool", flag.Value.Type())
	})
}

func TestHistoryCommand_DefaultLimit(t *testing.T) {
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	flag := historyCmd.Flags().Lookup("limit")
	require.NotNil(t, flag)
	assert.Equal(t, "20", flag.DefValue, "default limit should be 20")
}

func TestHistoryCommand_StatusFlagUsage(t *testing.T) {
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	flag := historyCmd.Flags().Lookup("status")
	require.NotNil(t, flag)

	// Usage should document valid values
	assert.Contains(t, flag.Usage, "success", "usage should mention 'success' status")
	assert.Contains(t, flag.Usage, "failed", "usage should mention 'failed' status")
	assert.Contains(t, flag.Usage, "cancelled", "usage should mention 'cancelled' status")
}

func TestHistoryCommand_SinceFlagFormat(t *testing.T) {
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	flag := historyCmd.Flags().Lookup("since")
	require.NotNil(t, flag)

	// Usage should document date format
	assert.Contains(t, flag.Usage, "YYYY-MM-DD", "usage should document date format")
}

func TestHistoryCommand_ExamplesInLongDescription(t *testing.T) {
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	longDesc := historyCmd.Long

	// Should contain various example usages
	examples := []string{
		"awf history",
		"--workflow",
		"--status",
		"--since",
		"--stats",
	}

	for _, example := range examples {
		assert.Contains(t, longDesc, example, "long description should contain example: %s", example)
	}
}

func TestHistoryCommand_FlagShorthands(t *testing.T) {
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	// Verify important flags have shorthands
	tests := []struct {
		longFlag  string
		shorthand string
	}{
		{"workflow", "w"},
		{"status", "s"},
		{"limit", "n"},
	}

	for _, tc := range tests {
		t.Run(tc.longFlag, func(t *testing.T) {
			flag := historyCmd.Flags().Lookup(tc.longFlag)
			require.NotNil(t, flag)
			assert.Equal(t, tc.shorthand, flag.Shorthand)
		})
	}
}

func TestHistoryCommand_NoAliases(t *testing.T) {
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	// History command should not have aliases (it's clear enough)
	assert.Empty(t, historyCmd.Aliases)
}

func TestHistoryCommand_RunE(t *testing.T) {
	// Test that RunE is set (command is runnable)
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	assert.NotNil(t, historyCmd.RunE, "history command should have RunE set")
}

func TestHistoryCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no flags",
			args:    []string{"history"},
			wantErr: false,
		},
		{
			name:    "workflow flag",
			args:    []string{"history", "--workflow", "deploy"},
			wantErr: false,
		},
		{
			name:    "workflow short flag",
			args:    []string{"history", "-w", "deploy"},
			wantErr: false,
		},
		{
			name:    "status flag",
			args:    []string{"history", "--status", "failed"},
			wantErr: false,
		},
		{
			name:    "status short flag",
			args:    []string{"history", "-s", "success"},
			wantErr: false,
		},
		{
			name:    "since flag",
			args:    []string{"history", "--since", "2025-12-01"},
			wantErr: false,
		},
		{
			name:    "limit flag",
			args:    []string{"history", "--limit", "50"},
			wantErr: false,
		},
		{
			name:    "limit short flag",
			args:    []string{"history", "-n", "50"},
			wantErr: false,
		},
		{
			name:    "stats flag",
			args:    []string{"history", "--stats"},
			wantErr: false,
		},
		{
			name:    "combined flags",
			args:    []string{"history", "-w", "deploy", "-s", "success", "--since", "2025-12-01", "-n", "100"},
			wantErr: false,
		},
		{
			name:    "invalid limit (non-numeric)",
			args:    []string{"history", "--limit", "abc"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tc.args)

			// Find and manually parse flags to check parsing without full execution
			historyCmd, _, err := cmd.Find([]string{"history"})
			require.NoError(t, err)

			// Reset flags for clean parsing
			_ = historyCmd.Flags().Parse(tc.args[1:])

			if tc.wantErr {
				err := historyCmd.Flags().Parse(tc.args[1:])
				assert.Error(t, err)
			}
		})
	}
}

func TestHistoryCommand_InheritsGlobalFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	// Should inherit global flags from root
	globalFlags := []string{"verbose", "quiet", "no-color", "format"}

	for _, flagName := range globalFlags {
		flag := historyCmd.InheritedFlags().Lookup(flagName)
		assert.NotNil(t, flag, "should inherit global flag: %s", flagName)
	}
}

func TestHistoryCommand_AcceptsNoArguments(t *testing.T) {
	cmd := cli.NewRootCommand()
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	// History command should not require positional arguments
	// The Args field should be nil (accepts any) or NoArgs
	assert.Nil(t, historyCmd.Args, "history command should not require positional args")
}

func TestRunHistory_NoHistory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty history directory
	historyDir := filepath.Join(tmpDir, "history")
	require.NoError(t, os.MkdirAll(historyDir, 0o755))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "history"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "No execution history found")
}

func TestRunHistory_InvalidSinceFormat(t *testing.T) {
	tests := []struct {
		name        string
		sinceValue  string
		errContains string
	}{
		{
			name:        "invalid date format",
			sinceValue:  "not-a-date",
			errContains: "invalid --since format",
		},
		{
			name:        "wrong date format DD-MM-YYYY",
			sinceValue:  "13-12-2025",
			errContains: "invalid --since format",
		},
		{
			name:        "partial date",
			sinceValue:  "2025-12",
			errContains: "invalid --since format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create history directory
			historyDir := filepath.Join(tmpDir, "history")
			require.NoError(t, os.MkdirAll(historyDir, 0o755))

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			var errOut bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)
			cmd.SetArgs([]string{"--storage=" + tmpDir, "history", "--since=" + tt.sinceValue})

			err := cmd.Execute()
			// Error should be in stderr or returned from Execute
			if err != nil {
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				// Check stderr for error message
				errOutput := errOut.String()
				assert.Contains(t, errOutput, tt.errContains)
			}
		})
	}
}

func TestRunHistory_ValidSinceFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create history directory
	historyDir := filepath.Join(tmpDir, "history")
	require.NoError(t, os.MkdirAll(historyDir, 0o755))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "history", "--since=2025-12-01"})

	err := cmd.Execute()
	// Should not error on date parsing
	require.NoError(t, err)
}

func TestRunHistory_Stats(t *testing.T) {
	tmpDir := t.TempDir()

	// Create history directory
	historyDir := filepath.Join(tmpDir, "history")
	require.NoError(t, os.MkdirAll(historyDir, 0o755))

	t.Run("text format shows statistics", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--storage=" + tmpDir, "history", "--stats"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "Execution Statistics")
		assert.Contains(t, output, "Total Executions:")
		assert.Contains(t, output, "Success:")
		assert.Contains(t, output, "Failed:")
		assert.Contains(t, output, "Cancelled:")
	})

	t.Run("json format shows statistics", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--storage=" + tmpDir, "--format=json", "history", "--stats"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		var stats map[string]interface{}
		err = json.Unmarshal([]byte(output), &stats)
		require.NoError(t, err, "output should be valid JSON")

		assert.Contains(t, stats, "total_executions")
		assert.Contains(t, stats, "success_count")
		assert.Contains(t, stats, "failed_count")
		assert.Contains(t, stats, "cancelled_count")
		assert.Contains(t, stats, "avg_duration_ms")
	})
}

func TestRunHistory_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create history directory
	historyDir := filepath.Join(tmpDir, "history")
	require.NoError(t, os.MkdirAll(historyDir, 0o755))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "--format=json", "history"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// Empty history should return valid JSON array
	var records []interface{}
	err = json.Unmarshal([]byte(output), &records)
	require.NoError(t, err, "output should be valid JSON array")
}

func TestRunHistory_Filters(t *testing.T) {
	tmpDir := t.TempDir()

	// Create history directory
	historyDir := filepath.Join(tmpDir, "history")
	require.NoError(t, os.MkdirAll(historyDir, 0o755))

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "filter by workflow name",
			args: []string{"--storage=" + tmpDir, "history", "--workflow=deploy"},
		},
		{
			name: "filter by status success",
			args: []string{"--storage=" + tmpDir, "history", "--status=success"},
		},
		{
			name: "filter by status failed",
			args: []string{"--storage=" + tmpDir, "history", "--status=failed"},
		},
		{
			name: "filter by status cancelled",
			args: []string{"--storage=" + tmpDir, "history", "--status=cancelled"},
		},
		{
			name: "limit results",
			args: []string{"--storage=" + tmpDir, "history", "--limit=10"},
		},
		{
			name: "combined filters",
			args: []string{"--storage=" + tmpDir, "history", "--workflow=test", "--status=success", "--limit=5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			// Should not error (even if no results found)
			require.NoError(t, err)
		})
	}
}

func TestRunHistory_TextOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create history directory
	historyDir := filepath.Join(tmpDir, "history")
	require.NoError(t, os.MkdirAll(historyDir, 0o755))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "history"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// For empty history, should show "No execution history found"
	if !strings.Contains(output, "No execution history found") {
		// If there are records, should have table headers
		assert.Contains(t, output, "ID")
		assert.Contains(t, output, "WORKFLOW")
		assert.Contains(t, output, "STATUS")
		assert.Contains(t, output, "DURATION")
		assert.Contains(t, output, "COMPLETED")
	}
}

// TestHistoryCommand_SQLiteHistoryStore_Wiring validates that the history command
// uses SQLiteHistoryStore (T005 component validation for bug-48 fix)
func TestHistoryCommand_SQLiteHistoryStore_Wiring(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "history"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify SQLite database file was created (not Badger directory)
	historyDBPath := filepath.Join(tmpDir, "history.db")
	_, statErr := os.Stat(historyDBPath)
	assert.NoError(t, statErr, "SQLite history.db should exist after history command execution")

	// Verify no Badger directory was created
	badgerPath := filepath.Join(tmpDir, "history")
	info, badgerErr := os.Stat(badgerPath)
	if badgerErr == nil && info.IsDir() {
		// If directory exists, it should be empty (leftover from old tests)
		entries, _ := os.ReadDir(badgerPath)
		assert.Empty(t, entries, "Badger history directory should be empty (no MANIFEST/VLOG files)")
	}
}

// TestHistoryCommand_ConcurrentAccess validates that multiple history commands
// can run concurrently without lock errors (bug-48 fix validation)
func TestHistoryCommand_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()

	const numConcurrent = 3
	errChan := make(chan error, numConcurrent)
	doneChan := make(chan struct{}, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(workerID int) {
			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--storage=" + tmpDir, "history"})

			err := cmd.Execute()
			if err != nil {
				errChan <- err
			}
			doneChan <- struct{}{}
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < numConcurrent; i++ {
		<-doneChan
	}
	close(errChan)

	// Check if any worker failed
	// Preallocate for potential errors
	errors := make([]error, 0, numConcurrent)
	for err := range errChan {
		errors = append(errors, err)
	}

	// All concurrent executions should succeed (no lock errors)
	assert.Empty(t, errors, "concurrent history command executions should not fail with lock errors")

	// Verify history.db exists and is a valid SQLite file
	historyDBPath := filepath.Join(tmpDir, "history.db")
	info, err := os.Stat(historyDBPath)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0, "history.db should have content")
}

// TestHistoryCommand_Stats_SQLiteIntegration validates stats with SQLite store
func TestHistoryCommand_Stats_SQLiteIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Execute history stats command
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "history", "--stats"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify SQLite database was created
	historyDBPath := filepath.Join(tmpDir, "history.db")
	_, statErr := os.Stat(historyDBPath)
	assert.NoError(t, statErr, "SQLite history.db should exist after stats query")

	// Output should contain stats
	output := out.String()
	assert.Contains(t, output, "Execution Statistics")
}

// TestHistoryCommand_FilterWithSQLite validates filtering with SQLite store
func TestHistoryCommand_FilterWithSQLite(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "filter by workflow",
			args: []string{"history", "--workflow=test"},
		},
		{
			name: "filter by status",
			args: []string{"history", "--status=success"},
		},
		{
			name: "filter with limit",
			args: []string{"history", "--limit=10"},
		},
		{
			name: "filter with since date",
			args: []string{"history", "--since=2025-01-01"},
		},
		{
			name: "combined filters",
			args: []string{"history", "--workflow=deploy", "--status=failed", "--limit=5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(append([]string{"--storage=" + tmpDir}, tt.args...))

			err := cmd.Execute()
			require.NoError(t, err)

			// Verify SQLite database was created
			historyDBPath := filepath.Join(tmpDir, "history.db")
			_, statErr := os.Stat(historyDBPath)
			assert.NoError(t, statErr, "SQLite history.db should exist after filtered query")
		})
	}
}

// TestHistoryCommand_JSONOutput_SQLite validates JSON output with SQLite store
func TestHistoryCommand_JSONOutput_SQLite(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "--format=json", "history"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify SQLite database was created
	historyDBPath := filepath.Join(tmpDir, "history.db")
	_, statErr := os.Stat(historyDBPath)
	assert.NoError(t, statErr, "SQLite history.db should exist for JSON output")

	// Output should be valid JSON
	output := out.String()
	var records []interface{}
	jsonErr := json.Unmarshal([]byte(output), &records)
	require.NoError(t, jsonErr, "output should be valid JSON array")
}

// TestHistoryCommand_StatsJSON_SQLite validates JSON stats output with SQLite
func TestHistoryCommand_StatsJSON_SQLite(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "--format=json", "history", "--stats"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify SQLite database was created
	historyDBPath := filepath.Join(tmpDir, "history.db")
	_, statErr := os.Stat(historyDBPath)
	assert.NoError(t, statErr, "SQLite history.db should exist for JSON stats")

	// Output should be valid JSON with stats fields
	output := out.String()
	var stats map[string]interface{}
	jsonErr := json.Unmarshal([]byte(output), &stats)
	require.NoError(t, jsonErr, "output should be valid JSON object")
	assert.Contains(t, stats, "total_executions")
}
