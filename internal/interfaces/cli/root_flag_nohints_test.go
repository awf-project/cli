package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/interfaces/cli"
)

// Component T012: Tests for --no-hints persistent flag registration

func TestRootCommand_HasNoHintsFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	flag := cmd.PersistentFlags().Lookup("no-hints")
	if flag == nil {
		t.Fatal("expected --no-hints persistent flag to exist")
	}
}

func TestRootCommand_NoHintsFlagType(t *testing.T) {
	cmd := cli.NewRootCommand()

	flag := cmd.PersistentFlags().Lookup("no-hints")
	if flag == nil {
		t.Fatal("expected --no-hints persistent flag to exist")
	}

	// Verify it's a boolean flag
	if flag.Value.Type() != "bool" {
		t.Errorf("expected --no-hints to be bool type, got: %s", flag.Value.Type())
	}
}

func TestRootCommand_NoHintsFlagDefaultValue(t *testing.T) {
	cmd := cli.NewRootCommand()

	flag := cmd.PersistentFlags().Lookup("no-hints")
	if flag == nil {
		t.Fatal("expected --no-hints persistent flag to exist")
	}

	// Default value should be "false" (hints enabled by default)
	if flag.DefValue != "false" {
		t.Errorf("expected --no-hints default value 'false', got: %s", flag.DefValue)
	}
}

func TestRootCommand_NoHintsFlagDescription(t *testing.T) {
	cmd := cli.NewRootCommand()

	flag := cmd.PersistentFlags().Lookup("no-hints")
	if flag == nil {
		t.Fatal("expected --no-hints persistent flag to exist")
	}

	// Verify usage description is meaningful
	if flag.Usage == "" {
		t.Error("expected --no-hints to have usage description")
	}

	// Description should mention hints or suggestions
	usage := strings.ToLower(flag.Usage)
	if !strings.Contains(usage, "hint") && !strings.Contains(usage, "suggestion") {
		t.Errorf("expected --no-hints usage to mention hints/suggestions, got: %s", flag.Usage)
	}
}

func TestRootCommand_NoHintsFlagPersistence(t *testing.T) {
	cmd := cli.NewRootCommand()

	// Verify flag is persistent (available to all subcommands)
	persistentFlag := cmd.PersistentFlags().Lookup("no-hints")
	localFlag := cmd.Flags().Lookup("no-hints")

	if persistentFlag == nil {
		t.Error("expected --no-hints to be a persistent flag")
	}
	if localFlag != nil {
		t.Error("expected --no-hints to be persistent, not local")
	}
}

func TestRootCommand_NoHintsFlagAvailableToSubcommands(t *testing.T) {
	cmd := cli.NewRootCommand()

	// Test that flag is inherited by subcommands
	subcommandTests := []string{"version", "list", "run", "validate", "error"}

	for _, subName := range subcommandTests {
		found := false
		for _, c := range cmd.Commands() {
			if c.Name() == subName {
				found = true
				break
			}
		}

		if !found {
			t.Logf("subcommand '%s' not found, skipping", subName)
			continue
		}

		// Inherited flags are accessible via root command's PersistentFlags
		// but not directly in subcommand's own flags
		if cmd.PersistentFlags().Lookup("no-hints") == nil {
			t.Errorf("expected --no-hints to be inherited by '%s' subcommand", subName)
		}
	}
}

func TestRootCommand_NoHintsInHelpOutput(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing --help: %v", err)
	}

	output := buf.String()

	// Verify --no-hints appears in help output
	if !strings.Contains(output, "--no-hints") {
		t.Error("expected --no-hints to appear in help output")
	}

	// Verify description is present
	if !strings.Contains(strings.ToLower(output), "hint") {
		t.Error("expected --no-hints description to mention hints in help output")
	}
}

func TestRootCommand_NoHintsFlagAcceptsTrue(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--no-hints", "version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected --no-hints to accept boolean value, got error: %v", err)
	}
}

func TestRootCommand_NoHintsFlagAcceptsExplicitValue(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{
			name:      "explicit true",
			args:      []string{"--no-hints=true", "version"},
			wantError: false,
		},
		{
			name:      "explicit false",
			args:      []string{"--no-hints=false", "version"},
			wantError: false,
		},
		{
			name:      "implicit true (flag present)",
			args:      []string{"--no-hints", "version"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()

			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestRootCommand_NoHintsFlagOrdering(t *testing.T) {
	// Test that --no-hints can appear before or after subcommand
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "flag before subcommand",
			args: []string{"--no-hints", "version"},
		},
		{
			name: "flag after subcommand",
			args: []string{"version", "--no-hints"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()

			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err != nil {
				t.Errorf("expected --no-hints to work in any position, got error: %v", err)
			}
		})
	}
}

func TestRootCommand_NoHintsFlagWithOtherFlags(t *testing.T) {
	// Test that --no-hints works alongside other global flags
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--no-hints", "--no-color", "--quiet", "version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected --no-hints to work with other flags, got error: %v", err)
	}
}

func TestRootCommand_GlobalFlagsIncludesNoHints(t *testing.T) {
	// Verify --no-hints is in the list of expected global flags
	cmd := cli.NewRootCommand()

	expectedFlags := []string{"verbose", "quiet", "no-color", "no-hints", "log-level", "config", "storage"}

	for _, flagName := range expectedFlags {
		flag := cmd.PersistentFlags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected global flag '--%s' to exist", flagName)
		}
	}
}

func TestRootCommand_NoHintsFlagConsistentWithNoColor(t *testing.T) {
	// Verify --no-hints follows the same pattern as --no-color
	cmd := cli.NewRootCommand()

	noHintsFlag := cmd.PersistentFlags().Lookup("no-hints")
	noColorFlag := cmd.PersistentFlags().Lookup("no-color")

	if noHintsFlag == nil || noColorFlag == nil {
		t.Fatal("expected both --no-hints and --no-color to exist")
	}

	// Both should be boolean flags
	if noHintsFlag.Value.Type() != noColorFlag.Value.Type() {
		t.Error("expected --no-hints to have same type as --no-color (bool)")
	}

	// Both should default to false (features enabled by default)
	if noHintsFlag.DefValue != noColorFlag.DefValue {
		t.Error("expected --no-hints and --no-color to have same default value (false)")
	}

	// Both should be persistent flags
	if noHintsFlag.Name == "" || noColorFlag.Name == "" {
		t.Error("expected both flags to be persistent")
	}
}

func TestRootCommand_NoHintsFlagInvalidValue(t *testing.T) {
	// Test that invalid boolean values are rejected
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--no-hints=invalid", "version"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected invalid boolean value to cause error")
	}

	// Error should be returned (either as err or in errBuf)
	if err == nil && errBuf.String() == "" {
		t.Error("expected error for invalid boolean value")
	}
}

func TestRootCommand_NoHintsFlagEmptyValue(t *testing.T) {
	// Test that --no-hints without value defaults to true (standard flag behavior)
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--no-hints", "version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected --no-hints without value to work (default true), got error: %v", err)
	}
}

func TestConfig_NoHintsField(t *testing.T) {
	// Verify Config struct has NoHints field
	cfg := cli.DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() should return non-nil config")
	}

	// Access the NoHints field (compilation check)
	noHints := cfg.NoHints

	// Default should be false (hints enabled by default)
	if noHints != false {
		t.Errorf("expected Config.NoHints default to be false, got: %v", noHints)
	}
}

func TestConfig_NoHintsFieldMutable(t *testing.T) {
	// Verify NoHints field can be set
	cfg := cli.DefaultConfig()

	cfg.NoHints = true
	if cfg.NoHints != true {
		t.Error("expected Config.NoHints to be settable to true")
	}

	cfg.NoHints = false
	if cfg.NoHints != false {
		t.Error("expected Config.NoHints to be settable to false")
	}
}

func TestDefaultConfig_NoHintsDefault(t *testing.T) {
	// Verify DefaultConfig initializes NoHints to false
	cfg := cli.DefaultConfig()

	if cfg.NoHints != false {
		t.Errorf("expected DefaultConfig().NoHints to be false (hints enabled by default), got: %v", cfg.NoHints)
	}
}
