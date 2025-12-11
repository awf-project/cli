package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
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
