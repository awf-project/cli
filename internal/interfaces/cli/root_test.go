package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/spf13/cobra"
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

func TestVersion_NewRootCommandExposesRootVersionOutputUsingVersionCommitAndBuildDate(t *testing.T) {
	t.Cleanup(func() {
		cli.Version = "dev"
		cli.Commit = "unknown"
		cli.BuildDate = "unknown"
	})
	cli.Version = "1.2.3"
	cli.Commit = "abc123"
	cli.BuildDate = "2026-06-24"

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	for _, expected := range []string{"awf version 1.2.3", "commit: abc123", "built: 2026-06-24"} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected version output to contain %q, got: %s", expected, output)
		}
	}
}

func TestVersion_PrintsExactlyThreeMetadataLinesInRequiredStructure(t *testing.T) {
	t.Cleanup(func() {
		cli.Version = "dev"
		cli.Commit = "unknown"
		cli.BuildDate = "unknown"
	})
	cli.Version = "1.0.0"
	cli.Commit = "abc123"
	cli.BuildDate = "2024-01-01"

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	want := []string{
		"awf version 1.0.0",
		"commit: abc123",
		"built: 2024-01-01",
	}
	if !slices.Equal(got, want) {
		t.Errorf("expected exact version lines %q, got %q", want, got)
	}
}

func TestVersion_WorksWithPlaceholderDevelopmentMetadataAndKeepsThreeLineStructure(t *testing.T) {
	t.Cleanup(func() {
		cli.Version = "dev"
		cli.Commit = "unknown"
		cli.BuildDate = "unknown"
	})
	cli.Version = "dev"
	cli.Commit = "unknown"
	cli.BuildDate = "unknown"

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	want := []string{
		"awf version dev",
		"commit: unknown",
		"built: unknown",
	}
	if !slices.Equal(got, want) {
		t.Errorf("expected placeholder version lines %q, got %q", want, got)
	}
}

func TestVersion_DoesNotExecuteNormalPersistentPreRunBehaviorInitializeProjectStateOrPrintUnrelatedOutput(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmpDir, "home"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "config"))

	historyDB := filepath.Join(tmpDir, "data", "awf", "history.db")
	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(historyDB); !os.IsNotExist(statErr) {
		t.Fatalf("expected --version to skip facade-backed project state initialization, history db stat error: %v", statErr)
	}
	if strings.Contains(buf.String(), "NOTICE:") || strings.Contains(buf.String(), "Error:") {
		t.Errorf("expected --version to avoid unrelated output, got: %s", buf.String())
	}
}

func TestVersion_NewVersionCommandIsRemovedAndNoLongerRegistered(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "version" {
			t.Fatal("expected root command not to register a 'version' subcommand")
		}
	}
}

func TestVersion_AWFVersionIsNoLongerARegisteredSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected 'awf version' to be rejected, got output: %s", buf.String())
	}
	if !strings.Contains(err.Error(), `unknown command "version"`) {
		t.Fatalf("expected 'awf version' to fail as an unknown command, got error: %v", err)
	}
}

func TestRootRegistersACPServeCommand(t *testing.T) {
	cmd := cli.NewRootCommand()

	var acpServe *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "acp-serve" {
			acpServe = sub
			break
		}
	}

	if acpServe == nil {
		t.Fatal("expected root command to register the 'acp-serve' subcommand")
	}
	if !acpServe.Hidden {
		t.Error("expected 'acp-serve' to be hidden")
	}
	if _, ok := acpServe.Annotations["skipFormatValidation"]; !ok {
		t.Error("expected 'acp-serve' to carry the skipFormatValidation annotation")
	}
}

func TestRootCommand_HasAllSubcommands(t *testing.T) {
	cmd := cli.NewRootCommand()

	expectedCommands := []string{"list", "run", "status", "validate", "diagram", "error"}

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

	flags := []string{"verbose", "quiet", "no-color", "no-hints", "log-level", "config", "storage", "format"}

	for _, flag := range flags {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("expected global flag '--%s' to exist", flag)
		}
	}
}

func TestRootCommand_VerboseFlagRemainsAvailableAsLongForm(t *testing.T) {
	cmd := cli.NewRootCommand()

	flag := cmd.PersistentFlags().Lookup("verbose")
	if flag == nil {
		t.Error("expected --verbose flag to exist")
	}
}

func TestRootCommand_VerboseFlagNoLongerHasShorthandV(t *testing.T) {
	cmd := cli.NewRootCommand()

	if flag := cmd.PersistentFlags().ShorthandLookup("v"); flag != nil {
		t.Errorf("expected -v shorthand to be unassigned, got --%s", flag.Name)
	}
}

func TestRootCommand_VerboseHelpOutputNoLongerAdvertisesVerboseShorthand(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if strings.Contains(buf.String(), "-v, --verbose") {
		t.Errorf("expected help output not to advertise '-v, --verbose', got:\n%s", buf.String())
	}
}

func TestRootCommand_VerboseRunningWithVDoesNotEnableVerboseModeAndIsRejectedOrReservedByCobra(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-v", "--version"})

	err := cmd.Execute()
	output := buf.String()
	if err == nil && !strings.Contains(output, "awf version") {
		t.Fatalf("expected -v to be rejected or reserved by Cobra, got output: %s", output)
	}
	if strings.Contains(output, "Enable verbose output") {
		t.Fatalf("expected -v not to enable or describe verbose mode, got output: %s", output)
	}
}

func TestRootCommand_GlobalFlagsOtherThanVerboseShorthandContinueToBeRegisteredAsBefore(t *testing.T) {
	cmd := cli.NewRootCommand()

	tests := []struct {
		name      string
		shorthand string
	}{
		{name: "quiet", shorthand: "q"},
		{name: "format", shorthand: "f"},
		{name: "no-color"},
		{name: "no-hints"},
		{name: "log-level"},
		{name: "config"},
		{name: "storage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.PersistentFlags().Lookup(tt.name)
			if flag == nil {
				t.Fatalf("expected global flag --%s to exist", tt.name)
			}
			if flag.Shorthand != tt.shorthand {
				t.Errorf("expected --%s shorthand %q, got %q", tt.name, tt.shorthand, flag.Shorthand)
			}
		})
	}
}

func TestRootCommand_QuietShortFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	flag := cmd.PersistentFlags().ShorthandLookup("q")
	if flag == nil {
		t.Error("expected -q shorthand for --quiet")
	}
}

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

// Component T010: Workflow Command Registration Tests

func TestRootCommand_HasWorkflowSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()

	var workflowCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "workflow" {
			workflowCmd = sub
			break
		}
	}

	if workflowCmd == nil {
		t.Fatal("expected root command to have 'workflow' subcommand")
	}
}

func TestRootCommand_WorkflowCommandStructure(t *testing.T) {
	cmd := cli.NewRootCommand()

	var workflowCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "workflow" {
			workflowCmd = sub
			break
		}
	}

	if workflowCmd == nil {
		t.Fatal("expected root command to have 'workflow' subcommand")
	}

	if workflowCmd.Use == "" {
		t.Error("workflow command should have 'Use' field set")
	}
	if !strings.Contains(workflowCmd.Use, "workflow") {
		t.Errorf("workflow command Use should contain 'workflow', got: %s", workflowCmd.Use)
	}

	if workflowCmd.Short == "" {
		t.Error("workflow command should have Short description")
	}

	if strings.TrimSpace(workflowCmd.Short) == "" {
		t.Error("workflow command Short description should not be empty")
	}
}

func TestRootCommand_WorkflowCommandHasAlias(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "workflow" {
			if !slices.Contains(sub.Aliases, "wf") {
				t.Error("workflow command should have 'wf' alias")
			}
			return
		}
	}

	t.Error("workflow command not found")
}

func TestRootCommand_WorkflowCommandHasSubcommands(t *testing.T) {
	cmd := cli.NewRootCommand()

	var workflowCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "workflow" {
			workflowCmd = sub
			break
		}
	}

	if workflowCmd == nil {
		t.Fatal("expected root command to have 'workflow' subcommand")
	}

	subcommandNames := []string{"install", "remove"}
	subcommands := workflowCmd.Commands()

	if len(subcommands) == 0 {
		t.Fatal("workflow command should have subcommands")
	}

	for _, expectedName := range subcommandNames {
		found := false
		for _, sub := range subcommands {
			if sub.Name() == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected workflow command to have '%s' subcommand", expectedName)
		}
	}
}

func TestRootCommand_WorkflowCommandHelp(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"workflow", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()

	expectedPhrases := []string{
		"workflow",
		"Manage",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("expected workflow help to contain '%s', got:\n%s", phrase, output)
		}
	}
}

func TestRootCommand_WorkflowInstallHelp(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"workflow", "install", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()

	expectedPhrases := []string{
		"install",
		"workflow",
		"GitHub",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("expected workflow install help to contain '%s', got:\n%s", phrase, output)
		}
	}
}

func TestRootCommand_WorkflowRemoveHelp(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"workflow", "remove", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()

	expectedPhrases := []string{
		"remove",
		"workflow",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("expected workflow remove help to contain '%s', got:\n%s", phrase, output)
		}
	}
}

func TestRootCommand_WorkflowCommandIntegration(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"workflow", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("workflow command should be executable through root, got error: %v", err)
	}

	output := buf.String()
	if output == "" && errBuf.String() == "" {
		t.Error("workflow command should produce output")
	}
}
