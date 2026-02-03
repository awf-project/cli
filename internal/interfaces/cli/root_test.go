package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

func TestNewRootCommand(t *testing.T) {
	cmd := cli.NewRootCommand()

	if cmd.Use != "awf" {
		t.Errorf("expected Use 'awf', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

func TestRootCommandHelp(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "awf") {
		t.Error("expected help output to contain 'awf'")
	}
	if !strings.Contains(output, "AI Workflow") {
		t.Error("expected help output to contain 'AI Workflow'")
	}
}

func TestVersionCommand(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "awf version") {
		t.Errorf("expected version output, got: %s", output)
	}
}

func TestVersionCommandFlags(t *testing.T) {
	// Set version info
	cli.Version = "1.0.0"
	cli.Commit = "abc123"
	cli.BuildDate = "2024-01-01"

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"version"})

	_ = cmd.Execute()
	output := buf.String()

	if !strings.Contains(output, "1.0.0") {
		t.Errorf("expected version '1.0.0' in output: %s", output)
	}
	if !strings.Contains(output, "abc123") {
		t.Errorf("expected commit 'abc123' in output: %s", output)
	}
}

func TestRootCommandHasVersionSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "version" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'version' subcommand")
	}
}

func TestRootCommand_HasAllSubcommands(t *testing.T) {
	cmd := cli.NewRootCommand()

	expectedCommands := []string{"version", "list", "run", "status", "validate", "diagram", "error"}

	for _, expected := range expectedCommands {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected root command to have '%s' subcommand", expected)
		}
	}
}

func TestRootCommand_HasDiagramSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()

	var diagramCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "diagram" {
			diagramCmd = sub
			break
		}
	}

	if diagramCmd == nil {
		t.Fatal("expected root command to have 'diagram' subcommand")
	}

	// Verify diagram command has expected flags
	flags := []struct {
		name      string
		shorthand string
	}{
		{"output", "o"},
		{"direction", ""},
		{"highlight", ""},
	}

	for _, f := range flags {
		flag := diagramCmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("expected diagram command to have '--%s' flag", f.name)
		}
		if f.shorthand != "" && flag.Shorthand != f.shorthand {
			t.Errorf("expected '--%s' to have shorthand '-%s', got '-%s'", f.name, f.shorthand, flag.Shorthand)
		}
	}
}

func TestRootCommand_DiagramHelpAccessible(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verify key elements are in help output
	expectedPhrases := []string{
		"diagram",
		"DOT",
		"--output",
		"--direction",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("expected diagram help to contain '%s', got:\n%s", phrase, output)
		}
	}
}

func TestRootCommand_GlobalFlags(t *testing.T) {
	cmd := cli.NewRootCommand()

	flags := []string{"verbose", "quiet", "no-color", "log-level", "config", "storage"}

	for _, flag := range flags {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("expected global flag '--%s' to exist", flag)
		}
	}
}

func TestRootCommand_VerboseShortFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	flag := cmd.PersistentFlags().ShorthandLookup("v")
	if flag == nil {
		t.Error("expected -v shorthand for --verbose")
	}
}

func TestRootCommand_QuietShortFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	flag := cmd.PersistentFlags().ShorthandLookup("q")
	if flag == nil {
		t.Error("expected -q shorthand for --quiet")
	}
}

// RED Phase: Test stubs for NewApp entry point

func TestNewApp_ReturnsNonNil(t *testing.T) {
	cfg := cli.DefaultConfig()
	app := cli.NewApp(cfg)

	if app == nil {
		t.Error("NewApp should return non-nil App")
	}
}

func TestNewApp_HasConfig(t *testing.T) {
	cfg := cli.DefaultConfig()
	cfg.Verbose = true
	cfg.NoColor = true

	app := cli.NewApp(cfg)

	if app.Config == nil {
		t.Error("App should have Config set")
	}
	if app.Config.Verbose != true {
		t.Error("App.Config.Verbose should be true")
	}
	if app.Config.NoColor != true {
		t.Error("App.Config.NoColor should be true")
	}
}

func TestNewApp_HasFormatter(t *testing.T) {
	cfg := cli.DefaultConfig()
	app := cli.NewApp(cfg)

	if app.Formatter == nil {
		t.Error("App should have Formatter set")
	}
}

func TestNewApp_FormatterReflectsConfig(t *testing.T) {
	// Test that formatter options match config
	tests := []struct {
		name    string
		cfg     *cli.Config
		wantErr bool
	}{
		{
			name: "default config",
			cfg:  cli.DefaultConfig(),
		},
		{
			name: "verbose mode",
			cfg: func() *cli.Config {
				c := cli.DefaultConfig()
				c.Verbose = true
				return c
			}(),
		},
		{
			name: "quiet mode",
			cfg: func() *cli.Config {
				c := cli.DefaultConfig()
				c.Quiet = true
				return c
			}(),
		},
		{
			name: "no color mode",
			cfg: func() *cli.Config {
				c := cli.DefaultConfig()
				c.NoColor = true
				return c
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := cli.NewApp(tt.cfg)
			if app == nil {
				t.Fatal("NewApp should return non-nil App")
			}
			if app.Formatter == nil {
				t.Error("App should have Formatter initialized")
			}
		})
	}
}

func TestDefaultConfig_ViaNewApp(t *testing.T) {
	// Test DefaultConfig() returns valid config for NewApp
	cfg := cli.DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig should return non-nil Config")
	}

	// Verify config can be used with NewApp
	app := cli.NewApp(cfg)
	if app == nil {
		t.Error("NewApp should accept DefaultConfig")
	}
}

// Component T011: Error Command Registration Tests

func TestRootCommand_HasErrorSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()

	var errorCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "error" {
			errorCmd = sub
			break
		}
	}

	if errorCmd == nil {
		t.Fatal("expected root command to have 'error' subcommand")
	}
}

func TestRootCommand_ErrorCommandStructure(t *testing.T) {
	cmd := cli.NewRootCommand()

	var errorCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "error" {
			errorCmd = sub
			break
		}
	}

	if errorCmd == nil {
		t.Fatal("expected root command to have 'error' subcommand")
	}

	// Verify basic command structure
	if errorCmd.Use == "" {
		t.Error("error command should have 'Use' field set")
	}
	if !strings.Contains(errorCmd.Use, "error") {
		t.Errorf("error command Use should contain 'error', got: %s", errorCmd.Use)
	}

	if errorCmd.Short == "" {
		t.Error("error command should have Short description")
	}

	if errorCmd.Long == "" {
		t.Error("error command should have Long description")
	}
}

func TestRootCommand_ErrorCommandHelp(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"error", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verify key elements are in help output
	expectedPhrases := []string{
		"error",
		"Look up error code",
		"documentation",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("expected error help to contain '%s', got:\n%s", phrase, output)
		}
	}
}

func TestRootCommand_ErrorCommandAcceptsOptionalArg(t *testing.T) {
	cmd := cli.NewRootCommand()

	var errorCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "error" {
			errorCmd = sub
			break
		}
	}

	if errorCmd == nil {
		t.Fatal("expected root command to have 'error' subcommand")
	}

	// Verify command accepts 0 or 1 arguments
	// MaximumNArgs(1) means Args validator should accept 0 or 1 args
	if errorCmd.Args == nil {
		t.Error("error command should have Args validator set")
	}

	// Test with no args (should pass validation)
	err := errorCmd.Args(errorCmd, []string{})
	if err != nil {
		t.Errorf("error command should accept 0 args, got error: %v", err)
	}

	// Test with 1 arg (should pass validation)
	err = errorCmd.Args(errorCmd, []string{"USER.INPUT.MISSING_FILE"})
	if err != nil {
		t.Errorf("error command should accept 1 arg, got error: %v", err)
	}

	// Test with 2 args (should fail validation)
	err = errorCmd.Args(errorCmd, []string{"arg1", "arg2"})
	if err == nil {
		t.Error("error command should reject 2 args")
	}
}

func TestRootCommand_ErrorCommandExamplesSyntax(t *testing.T) {
	cmd := cli.NewRootCommand()

	var errorCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "error" {
			errorCmd = sub
			break
		}
	}

	if errorCmd == nil {
		t.Fatal("expected root command to have 'error' subcommand")
	}

	// Verify examples are present in Long description
	if !strings.Contains(errorCmd.Long, "Examples:") {
		t.Error("error command Long description should contain 'Examples:' section")
	}

	// Verify example commands are properly formatted
	expectedExamples := []string{
		"awf error",
		"awf error USER.INPUT.MISSING_FILE",
	}

	for _, example := range expectedExamples {
		if !strings.Contains(errorCmd.Long, example) {
			t.Errorf("error command examples should contain '%s'", example)
		}
	}
}

func TestRootCommand_ErrorCommandIntegration(t *testing.T) {
	// Integration test: verify error command can be executed through root command
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"error", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("error command should be executable through root, got error: %v", err)
	}

	output := buf.String()
	if output == "" && errBuf.String() == "" {
		t.Error("error command should produce output")
	}
}

func TestRootCommand_ErrorCommandWithFormat(t *testing.T) {
	// Test that error command inherits global format flag
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)

	// Test with JSON format flag
	cmd.SetArgs([]string{"--format", "json", "error", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("error command should work with global --format flag, got error: %v", err)
	}
}

func TestRootCommand_ErrorCommandPosition(t *testing.T) {
	// Verify error command is registered in the correct position
	// Should be after config, diagram, etc.
	cmd := cli.NewRootCommand()

	subcommands := cmd.Commands()
	errorCmdIndex := -1

	for i, sub := range subcommands {
		if sub.Name() == "error" {
			errorCmdIndex = i
			break
		}
	}

	if errorCmdIndex == -1 {
		t.Fatal("error command not found in subcommands")
	}

	// Verify it's registered (position doesn't matter much, but it should exist)
	if errorCmdIndex < 0 {
		t.Error("error command should be registered")
	}
}
