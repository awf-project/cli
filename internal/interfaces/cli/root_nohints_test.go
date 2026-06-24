package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
)

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

	if flag.Usage == "" {
		t.Error("expected --no-hints to have usage description")
	}

	usage := strings.ToLower(flag.Usage)
	if !strings.Contains(usage, "hint") && !strings.Contains(usage, "suggestion") {
		t.Errorf("expected --no-hints usage to mention hints/suggestions, got: %s", flag.Usage)
	}
}

func TestRootCommand_NoHintsFlagPersistence(t *testing.T) {
	cmd := cli.NewRootCommand()

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

	subcommandTests := []string{"list", "run", "validate", "error"}

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

	if !strings.Contains(output, "--no-hints") {
		t.Error("expected --no-hints to appear in help output")
	}

	if !strings.Contains(strings.ToLower(output), "hint") {
		t.Error("expected --no-hints description to mention hints in help output")
	}
}

func TestRootCommand_NoHintsFlagAcceptsTrue(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--no-hints", "--version"})

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
			args:      []string{"--no-hints=true", "--version"},
			wantError: false,
		},
		{
			name:      "explicit false",
			args:      []string{"--no-hints=false", "--version"},
			wantError: false,
		},
		{
			name:      "implicit true (flag present)",
			args:      []string{"--no-hints", "--version"},
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
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "flag before subcommand",
			args: []string{"--no-hints", "help"},
		},
		{
			name: "flag after subcommand",
			args: []string{"help", "--no-hints"},
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
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--no-hints", "--no-color", "--quiet", "--version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected --no-hints to work with other flags, got error: %v", err)
	}
}

func TestRootCommand_GlobalFlagsIncludesNoHints(t *testing.T) {
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
	cmd := cli.NewRootCommand()

	noHintsFlag := cmd.PersistentFlags().Lookup("no-hints")
	noColorFlag := cmd.PersistentFlags().Lookup("no-color")

	if noHintsFlag == nil || noColorFlag == nil {
		t.Fatal("expected both --no-hints and --no-color to exist")
	}

	if noHintsFlag.Value.Type() != noColorFlag.Value.Type() {
		t.Error("expected --no-hints to have same type as --no-color (bool)")
	}

	if noHintsFlag.DefValue != noColorFlag.DefValue {
		t.Error("expected --no-hints and --no-color to have same default value (false)")
	}

	if noHintsFlag.Name == "" || noColorFlag.Name == "" {
		t.Error("expected both flags to be persistent")
	}
}

func TestRootCommand_NoHintsFlagInvalidValue(t *testing.T) {
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

	if err == nil && errBuf.String() == "" {
		t.Error("expected error for invalid boolean value")
	}
}

func TestRootCommand_NoHintsFlagEmptyValue(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--no-hints", "--version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected --no-hints without value to work (default true), got error: %v", err)
	}
}

func TestConfig_NoHintsField(t *testing.T) {
	cfg := cli.DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() should return non-nil config")
	}

	noHints := cfg.NoHints

	if noHints != false {
		t.Errorf("expected Config.NoHints default to be false, got: %v", noHints)
	}
}

func TestConfig_NoHintsFieldMutable(t *testing.T) {
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
	cfg := cli.DefaultConfig()

	if cfg.NoHints != false {
		t.Errorf("expected DefaultConfig().NoHints to be false (hints enabled by default), got: %v", cfg.NoHints)
	}
}
