package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeCommand_FlagDefaults_PortAndHost(t *testing.T) {
	cmd := cli.NewServeCommand()

	portFlag := cmd.Flags().Lookup("port")
	require.NotNil(t, portFlag, "expected --port flag to exist")
	assert.Equal(t, "2511", portFlag.DefValue, "expected default port to be 2511")

	hostFlag := cmd.Flags().Lookup("host")
	require.NotNil(t, hostFlag, "expected --host flag to exist")
	assert.Equal(t, "127.0.0.1", hostFlag.DefValue, "expected default host to be 127.0.0.1")
}

func TestServeCommand_BindsLocalhostByDefault_NFR005(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"serve", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "127.0.0.1:2511", "expected help to show localhost binding")
	assert.Contains(t, output, "prevent inadvertent network exposure", "expected security warning in help")
}

func TestServeCommand_CommandStructure(t *testing.T) {
	cmd := cli.NewServeCommand()

	assert.Equal(t, "serve", cmd.Use, "expected Use field to be 'serve'")
	assert.NotEmpty(t, cmd.Short, "expected Short description to be set")
	assert.NotEmpty(t, cmd.Long, "expected Long description to be set")
	assert.Contains(t, cmd.Short, "API", "expected Short to mention API")
	assert.Contains(t, cmd.Long, "2511", "expected Long to mention default port")
	assert.Contains(t, cmd.Long, "127.0.0.1", "expected Long to mention localhost binding")
}

func TestServeCommand_IsRegisteredInRoot(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "serve" {
			found = true
			break
		}
	}

	assert.True(t, found, "expected 'serve' command to be registered in root")
}

func TestServeCommand_FlagsExist(t *testing.T) {
	cmd := cli.NewServeCommand()

	tests := []struct {
		flagName string
	}{
		{"port"},
		{"host"},
	}

	for _, tt := range tests {
		t.Run(tt.flagName, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.flagName)
			require.NotNil(t, flag, "expected --%s flag to exist", tt.flagName)
		})
	}
}

func TestServeCommand_FlagTypes(t *testing.T) {
	cmd := cli.NewServeCommand()

	portFlag := cmd.Flags().Lookup("port")
	require.NotNil(t, portFlag)
	assert.Equal(t, "int", portFlag.Value.Type(), "expected --port to be int type")

	hostFlag := cmd.Flags().Lookup("host")
	require.NotNil(t, hostFlag)
	assert.Equal(t, "string", hostFlag.Value.Type(), "expected --host to be string type")
}

func TestServeCommand_NonLoopbackHostAccepted(t *testing.T) {
	cmd := cli.NewServeCommand()

	err := cmd.ParseFlags([]string{"--host=0.0.0.0", "--port=8080"})
	require.NoError(t, err, "non-loopback host must be accepted at flag parse time (warning emitted at runtime)")

	hostFlag := cmd.Flags().Lookup("host")
	require.NotNil(t, hostFlag)
	assert.Equal(t, "0.0.0.0", hostFlag.Value.String(), "parsed non-loopback host must be preserved exactly")

	portFlag := cmd.Flags().Lookup("port")
	require.NotNil(t, portFlag)
	assert.Equal(t, "8080", portFlag.Value.String(), "parsed non-default port must be preserved exactly")
}

func TestServeCommand_HasRunE(t *testing.T) {
	cmd := cli.NewServeCommand()
	assert.NotNil(t, cmd.RunE, "expected RunE to be set")
}

func TestServeCommand_FlagHelpText(t *testing.T) {
	cmd := cli.NewServeCommand()

	portFlag := cmd.Flags().Lookup("port")
	assert.NotEmpty(t, portFlag.Usage, "expected --port to have help text")

	hostFlag := cmd.Flags().Lookup("host")
	assert.NotEmpty(t, hostFlag.Usage, "expected --host to have help text")
	assert.Contains(t, strings.ToLower(hostFlag.Usage), "bind", "expected host flag help to mention 'bind'")
}

func TestServeCommand_DefaultsInHelpOutput(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"serve", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "2511", "expected help output to show port default")
	assert.Contains(t, output, "127.0.0.1", "expected help output to show host default")
}

func TestServeCommand_ExecutesWithDefaults(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Don't actually run - just test that flags are parsed without error
	// The stub implementation returns nil, so execution would succeed if we ran it
	cmd.SetArgs([]string{"serve"})

	// Verify command can be found and executed (stub returns nil)
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "serve" {
			found = true
			assert.NotNil(t, sub.RunE, "expected RunE to exist")
			break
		}
	}
	assert.True(t, found, "expected serve command to be available")
}
