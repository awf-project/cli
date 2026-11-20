package notifierwiring

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/infrastructure/notifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingOperationProvider struct {
	operations map[string]*pluginmodel.OperationSchema
	calls      []recordedOperationCall
}

type recordedOperationCall struct {
	name   string
	inputs map[string]any
}

func (p *recordingOperationProvider) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	operation, found := p.operations[name]
	return operation, found
}

func (p *recordingOperationProvider) ListOperations() []*pluginmodel.OperationSchema {
	operations := make([]*pluginmodel.OperationSchema, 0, len(p.operations))
	for _, operation := range p.operations {
		operations = append(operations, operation)
	}
	return operations
}

func (p *recordingOperationProvider) Execute(
	_ context.Context,
	name string,
	inputs map[string]any,
) (*pluginmodel.OperationResult, error) {
	p.calls = append(p.calls, recordedOperationCall{name: name, inputs: inputs})
	return &pluginmodel.OperationResult{Success: true, Outputs: map[string]any{"operation": name}}, nil
}

func TestNotifierProviderCompositionPreservesExternalOperations(t *testing.T) {
	external := &recordingOperationProvider{operations: map[string]*pluginmodel.OperationSchema{
		"canary.send":        {Name: "canary.send", PluginName: "canary"},
		"external.operation": {Name: "external.operation", PluginName: "external"},
	}}

	disabled := ComposeOperationProvider(Settings{Backend: "canary", DefaultChannel: "alerts"}, false, external)
	assert.Same(t, external, disabled)

	var enabled any
	stdout, stderr := captureOutput(t, func() {
		enabled = ComposeOperationProvider(
			Settings{Backend: "backend-output-canary", DefaultChannel: "channel-output-canary"},
			true,
			external,
		)
	})
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)

	provider, ok := enabled.(interface {
		ListOperations() []*pluginmodel.OperationSchema
		GetOperation(string) (*pluginmodel.OperationSchema, bool)
	})
	require.True(t, ok)
	names := operationNames(provider.ListOperations())
	assert.Len(t, names, 6, "enabled composition must expose exactly the external and four notifier operations")
	assert.ElementsMatch(t, []string{
		"canary.send",
		"external.operation",
		notifier.OperationSend,
		notifier.OperationUpdate,
		notifier.OperationReact,
		notifier.OperationUpload,
	}, names)
	for _, notifierOperation := range []string{
		notifier.OperationSend,
		notifier.OperationUpdate,
		notifier.OperationReact,
		notifier.OperationUpload,
	} {
		assert.Equal(t, 1, operationNameCount(names, notifierOperation), "notifier operation %q must be exposed exactly once", notifierOperation)
	}
	_, found := provider.GetOperation("external.operation")
	assert.True(t, found)

	withoutExternal := ComposeOperationProvider(Settings{}, true, nil)
	require.NotNil(t, withoutExternal)
	nilNames := operationNames(withoutExternal.ListOperations())
	assert.Len(t, nilNames, 4, "enabled nil-external composition must expose exactly the four notifier operations")
	assert.ElementsMatch(t, []string{
		notifier.OperationSend,
		notifier.OperationUpdate,
		notifier.OperationReact,
		notifier.OperationUpload,
	}, nilNames)
	_, err := withoutExternal.Execute(context.Background(), notifier.OperationSend, map[string]any{
		"backend": "missing",
		"channel": "alerts",
		"text":    "payload-canary",
	})
	assert.ErrorIs(t, err, notifier.ErrAdapterUnavailable)
}

func TestNotifierProviderCompositionAppliesConfiguredDefaults(t *testing.T) {
	external := &recordingOperationProvider{operations: map[string]*pluginmodel.OperationSchema{
		"configured.send": {Name: "configured.send", PluginName: "configured"},
		"explicit.send":   {Name: "explicit.send", PluginName: "explicit"},
	}}
	provider := ComposeOperationProvider(
		Settings{Backend: "configured", DefaultChannel: "configured-channel-canary"},
		true,
		external,
	)
	rich := map[string]any{"token": "payload-canary"}

	stdout, stderr := captureOutput(t, func() {
		result, err := provider.Execute(context.Background(), notifier.OperationSend, map[string]any{
			"text": "default-payload-canary",
			"rich": rich,
		})
		require.NoError(t, err)
		assert.True(t, result.Success)

		result, err = provider.Execute(context.Background(), notifier.OperationSend, map[string]any{
			"backend": "explicit",
			"channel": "explicit-channel-canary",
			"text":    "explicit-payload-canary",
		})
		require.NoError(t, err)
		assert.True(t, result.Success)
	})
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)

	require.Len(t, external.calls, 2)
	assert.Equal(t, "configured.send", external.calls[0].name)
	assert.Equal(t, "configured-channel-canary", external.calls[0].inputs["channel"])
	assert.Equal(t, rich, external.calls[0].inputs["rich"])
	assert.NotContains(t, external.calls[0].inputs, "backend")
	assert.Equal(t, "explicit.send", external.calls[1].name)
	assert.Equal(t, "explicit-channel-canary", external.calls[1].inputs["channel"])
	assert.NotContains(t, external.calls[1].inputs, "backend")
}

func operationNames(operations []*pluginmodel.OperationSchema) []string {
	names := make([]string, 0, len(operations))
	for _, operation := range operations {
		if operation != nil {
			names = append(names, operation.Name)
		}
	}
	return names
}

func operationNameCount(names []string, target string) int {
	count := 0
	for _, name := range names {
		if name == target {
			count++
		}
	}
	return count
}

func captureOutput(t *testing.T, run func()) (string, string) {
	t.Helper()
	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)
	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)
	originalStdout, originalStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	t.Cleanup(func() {
		os.Stdout, os.Stderr = originalStdout, originalStderr
	})

	run()
	require.NoError(t, stdoutWriter.Close())
	require.NoError(t, stderrWriter.Close())
	os.Stdout, os.Stderr = originalStdout, originalStderr
	stdout, err := io.ReadAll(stdoutReader)
	require.NoError(t, err)
	stderr, err := io.ReadAll(stderrReader)
	require.NoError(t, err)
	require.NoError(t, stdoutReader.Close())
	require.NoError(t, stderrReader.Close())
	return string(stdout), string(stderr)
}
