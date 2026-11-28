//go:build integration

// Feature: 27
package cli_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/awf-project/cli/pkg/plugin/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const trackerSecretMarker = "TRACKER_SECRET_MARKER_27"

type trackerScenario struct {
	name, operation string
	inputs, result  map[string]any
}

type trackerCallRecord struct {
	Backend          string   `json:"backend"`
	Operation        string   `json:"operation"`
	InputKeys        []string `json:"input_keys"`
	CanonicalPayload bool     `json:"canonical_payload"`
	SecretSeen       bool     `json:"secret_seen"`
}

type trackerTransportPlugin struct {
	sdk.BasePlugin
	operations []string
}

func (p *trackerTransportPlugin) Operations() []string { return p.operations }

func (p *trackerTransportPlugin) HandleOperation(_ context.Context, operation string, inputs map[string]any) (*sdk.OperationResult, error) {
	keys := make([]string, 0, len(inputs))
	secretSeen := false
	for key, value := range inputs {
		keys = append(keys, key)
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		secretSeen = secretSeen || bytes.Contains(encoded, []byte(trackerSecretMarker))
	}
	sort.Strings(keys)
	record := trackerCallRecord{
		Backend:          strings.TrimPrefix(filepath.Base(os.Args[0]), "awf-plugin-"),
		Operation:        operation,
		InputKeys:        keys,
		CanonicalPayload: isCanonicalTrackerPayload(operation, inputs),
		SecretSeen:       secretSeen,
	}
	if err := appendTrackerCallRecord(os.Getenv("AWF_TRACKER_CALLS"), record); err != nil {
		return nil, err
	}
	result, ok := trackerAdapterResults()[operation]
	if !ok {
		return sdk.NewErrorResult("unsupported tracker fixture operation"), nil
	}
	return sdk.NewSuccessResult(operation, result), nil
}

func serveTrackerTransportPlugin() {
	operations := strings.Split(os.Getenv("AWF_TRACKER_OPERATIONS"), ",")
	if len(operations) == 1 && operations[0] == "" {
		operations = nil
	}
	sdk.Serve(&trackerTransportPlugin{
		BasePlugin: sdk.BasePlugin{
			PluginName:    strings.TrimPrefix(filepath.Base(os.Args[0]), "awf-plugin-"),
			PluginVersion: "1.0.0",
		},
		operations: operations,
	})
}

func trackerCapabilityScenarios() []trackerScenario {
	issue := map[string]any{
		"id": "issue-27", "key": "TEAM-27", "title": "Canonical title", "status": "Open",
		"description": "Canonical description", "assignee": "canonical-user", "url": "https://tracker.invalid/TEAM-27",
	}
	return []trackerScenario{
		{
			name: "CreateIssueReturnsCanonicalIdentityAndURL", operation: "create_issue",
			inputs: map[string]any{"title": trackerSecretMarker, "description": trackerSecretMarker},
			result: map[string]any{"id": "issue-27", "key": "TEAM-27", "url": "https://tracker.invalid/TEAM-27"},
		},
		{
			name: "UpdateIssueReturnsCanonicalIdentity", operation: "update_issue",
			inputs: map[string]any{"key": trackerSecretMarker, "title": trackerSecretMarker},
			result: map[string]any{"id": "issue-27", "key": "TEAM-27"},
		},
		{
			name: "GetIssueReturnsCanonicalDetails", operation: "get_issue",
			inputs: map[string]any{"key": trackerSecretMarker}, result: issue,
		},
		{
			name: "SearchReturnsCanonicalIssuesAndTotal", operation: "search",
			inputs: map[string]any{"query": trackerSecretMarker},
			result: map[string]any{"issues": []any{issue}, "total": float64(1)},
		},
		{
			name: "TransitionReturnsCanonicalStatus", operation: "transition",
			inputs: map[string]any{"key": trackerSecretMarker, "transition": "Done"},
			result: map[string]any{"id": "issue-27", "key": "TEAM-27", "status": "Done"},
		},
		{
			name: "AddCommentReturnsCanonicalCommentIdentity", operation: "add_comment",
			inputs: map[string]any{"key": trackerSecretMarker, "body": trackerSecretMarker},
			result: map[string]any{"comment_id": "comment-27"},
		},
	}
}

func trackerAdapterResults() map[string]map[string]any {
	results := make(map[string]map[string]any)
	for _, scenario := range trackerCapabilityScenarios() {
		result := make(map[string]any, len(scenario.result)+1)
		for key, value := range scenario.result {
			result[key] = value
		}
		results[scenario.operation] = result
	}
	// The public tracker facade must expose only canonical result fields. This
	// adapter-only value exercises result confidentiality without changing the
	// required canonical business result.
	results["create_issue"]["adapter_secret"] = trackerSecretMarker
	return results
}

func TestTrackerWorkflowsDelegateEveryOperationThroughInstalledAdapter(t *testing.T) {
	for _, scenario := range trackerCapabilityScenarios() {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			fixture := newTrackerTransportFixture(t, "jira", trackerOperationNames())
			fixture.writeConfig(t, "jira")
			workflowName := "tracker-" + strings.ReplaceAll(scenario.operation, "_", "-")
			fixture.writeWorkflow(t, workflowName, scenario.operation, scenario.inputs)

			output, err := fixture.execute(t, []string{"run", workflowName, "--output=buffered"})
			require.NoError(t, err, output)
			execution := fixture.execution(t, "tracker-transport")
			assert.Equal(t, workflow.StatusCompleted, execution.Status)
			assert.Equal(t, stringifyTrackerResult(scenario.result), parseTrackerOutput(t, execution.States["operation"].Output))
			calls := fixture.calls(t)
			require.Len(t, calls, 1)
			assert.Equal(t, scenario.operation, calls[0].Operation)
			assert.Equal(t, sortedPayloadKeys(scenario.inputs), calls[0].InputKeys)
			assert.True(t, calls[0].CanonicalPayload)
			assert.True(t, calls[0].SecretSeen)
			fixture.assertConfidential(t, output)
			fixture.closeAndRemoveBinaries(t)
		})
	}
}

func TestTrackerBackendSelectionUsesConfigurationAndInputOverride(t *testing.T) {
	tests := []struct{ name, configured, inputBackend, expectedBackend string }{
		{name: "ConfiguredDefaultSelectsInstalledAdapter", configured: "jira", expectedBackend: "jira"},
		{name: "InputBackendOverridesConfiguredDefault", configured: "jira", inputBackend: "linear", expectedBackend: "linear"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			fixture := newTrackerTransportFixture(t, "jira", trackerOperationNames())
			fixture.installAdapter(t, "linear")
			fixture.writeConfig(t, test.configured)
			inputs := map[string]any{"title": trackerSecretMarker}
			if test.inputBackend != "" {
				inputs["backend"] = test.inputBackend
			}
			fixture.writeWorkflow(t, "tracker-selection", "create_issue", inputs)

			output, err := fixture.execute(t, []string{"run", "tracker-selection", "--output=buffered"})
			require.NoError(t, err, output)
			calls := fixture.calls(t)
			require.Len(t, calls, 1)
			assert.Equal(t, "tracker-"+test.expectedBackend, calls[0].Backend)
			assert.True(t, calls[0].CanonicalPayload)
			fixture.assertConfidential(t, output)
			fixture.closeAndRemoveBinaries(t)
		})
	}
}

func TestTrackerWorkflowReportsUnavailableAdapter(t *testing.T) {
	t.Run("MissingAdapterFailsBeforeExecution", func(t *testing.T) {
		fixture := newTrackerTransportFixture(t, "linear", trackerOperationNames())
		fixture.writeConfig(t, "jira")
		fixture.writeWorkflow(t, "tracker-missing", "create_issue", map[string]any{"title": trackerSecretMarker})

		output, err := fixture.execute(t, []string{"run", "tracker-missing", "--output=buffered"})
		require.Error(t, err)
		diagnostic := output + err.Error()
		assert.Contains(t, diagnostic, "backend jira operation create_issue unavailable")
		assert.Empty(t, fixture.calls(t))
		fixture.assertConfidential(t, diagnostic)
		fixture.closeAndRemoveBinaries(t)
	})
	t.Run("DisabledAdapterFailsBeforeExecution", func(t *testing.T) {
		fixture := newTrackerTransportFixture(t, "jira", trackerOperationNames())
		fixture.writeConfig(t, "jira")
		fixture.writeWorkflow(t, "tracker-disabled", "create_issue", map[string]any{"title": trackerSecretMarker})

		output, err := fixture.execute(t, []string{"plugin", "disable", "tracker-jira"})
		require.NoError(t, err, "disabling the installed tracker adapter must succeed")
		assert.Contains(t, output, "disabled")

		output, err = fixture.execute(t, []string{"run", "tracker-disabled", "--output=buffered"})
		require.Error(t, err)
		diagnostic := output + err.Error()
		fixture.assertConfidential(t, diagnostic)
		assert.Contains(t, diagnostic, "backend jira operation create_issue unavailable")
		assert.Empty(t, fixture.calls(t))
		fixture.closeAndRemoveBinaries(t)
	})
	t.Run("IncompleteAdapterFailsBeforeExecution", func(t *testing.T) {
		fixture := newTrackerTransportFixture(t, "jira", []string{"get_issue"})
		fixture.writeConfig(t, "jira")
		fixture.writeWorkflow(t, "tracker-incomplete", "create_issue", map[string]any{"title": trackerSecretMarker})

		output, err := fixture.execute(t, []string{"run", "tracker-incomplete", "--output=buffered"})
		require.Error(t, err)
		diagnostic := output + err.Error()
		assert.Contains(t, diagnostic, "backend jira operation create_issue unavailable")
		assert.Empty(t, fixture.calls(t))
		fixture.assertConfidential(t, diagnostic)
		fixture.closeAndRemoveBinaries(t)
	})
}

func TestValidateCommandAcceptsCanonicalTrackerOperation(t *testing.T) {
	fixture := newTrackerTransportFixture(t, "jira", trackerOperationNames())
	fixture.writeConfig(t, "jira")
	source := fixture.workflowSource("create_issue", map[string]any{"title": "canonical-title"})
	fixture.writeRawWorkflow(t, "tracker-valid", source)
	fixture.writeRawWorkflow(t, "tracker-unknown", strings.Replace(source, "tracker.create_issue", "tracker.unknown", 1))

	output, err := fixture.execute(t, []string{"validate", "tracker-valid"})
	require.NoError(t, err, output)
	assert.Contains(t, output, "is valid")
	assert.Empty(t, fixture.calls(t))

	output, err = fixture.execute(t, []string{"validate", "tracker-valid", "--skip-plugins"})
	require.NoError(t, err, output)
	assert.Contains(t, output, "is valid")
	assert.Empty(t, fixture.calls(t))

	output, err = fixture.execute(t, []string{"validate", "tracker-unknown"})
	require.Error(t, err)
	diagnostic := strings.ToLower(output + err.Error())
	assert.Contains(t, diagnostic, "tracker.unknown")
	assert.Contains(t, diagnostic, "not found")
	assert.Empty(t, fixture.calls(t))
	fixture.closeAndRemoveBinaries(t)
}

func TestTrackerSingleStepDelegatesThroughInstalledAdapter(t *testing.T) {
	fixture := newTrackerTransportFixture(t, "jira", trackerOperationNames())
	fixture.writeConfig(t, "jira")
	fixture.writeWorkflow(t, "tracker-single-step", "create_issue", map[string]any{"title": trackerSecretMarker})

	output, err := fixture.execute(t, []string{"run", "tracker-single-step", "--step=operation", "--output=buffered"})
	require.NoError(t, err, output)
	assert.Equal(t, map[string]string{
		"id":  "issue-27",
		"key": "TEAM-27",
		"url": "https://tracker.invalid/TEAM-27",
	}, parseTrackerOutput(t, singleStepStdout(t, output)))
	calls := fixture.calls(t)
	require.Len(t, calls, 1)
	assert.Equal(t, "create_issue", calls[0].Operation)
	assert.True(t, calls[0].CanonicalPayload)
	fixture.assertConfidential(t, output)
	fixture.closeAndRemoveBinaries(t)
}

type trackerTransportFixture struct {
	root, pluginsDir, storageDir, callsPath string
	cleanups                                []func()
	closed                                  bool
}

func newTrackerTransportFixture(t *testing.T, backend string, operations []string) *trackerTransportFixture {
	t.Helper()
	root := t.TempDir()
	fixture := &trackerTransportFixture{
		root: root, pluginsDir: filepath.Join(root, "plugins"),
		storageDir: filepath.Join(root, "storage"), callsPath: filepath.Join(root, "calls.jsonl"),
	}
	t.Setenv("AWF_TEST_MODE", "1")
	t.Setenv("AWF_PLUGINS_PATH", fixture.pluginsDir)
	t.Setenv("AWF_TRACKER_CALLS", fixture.callsPath)
	t.Setenv("AWF_TRACKER_OPERATIONS", strings.Join(operations, ","))
	t.Setenv("AWF_WORKFLOWS_PATH", filepath.Join(root, "workflows"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "xdg-state"))
	t.Setenv("AWF_AUDIT_LOG", "off")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "workflows"), 0o700))
	fixture.installAdapter(t, backend)
	return fixture
}

func (f *trackerTransportFixture) installAdapter(t *testing.T, backend string) {
	t.Helper()
	pluginName := "tracker-" + backend
	pluginDir := filepath.Join(f.pluginsDir, pluginName)
	require.NoError(t, os.MkdirAll(pluginDir, 0o700))
	manifest := fmt.Sprintf("name: %s\nversion: 1.0.0\nawf_version: \">=0.1.0\"\ncapabilities:\n  - operations\n", pluginName)
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(manifest), 0o600))
	executable, err := os.Executable()
	require.NoError(t, err)
	target := filepath.Join(pluginDir, "awf-plugin-"+pluginName)
	copyExecutableAtomically(t, executable, target)
	f.cleanups = append(f.cleanups, func() { require.NoError(t, os.Remove(target)) })
}

func (f *trackerTransportFixture) writeConfig(t *testing.T, backend string) {
	t.Helper()
	path := filepath.Join(f.root, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("tracker:\n  default_backend: "+backend+"\n"), 0o600))
	t.Setenv("AWF_CONFIG_PATH", path)
}

func (f *trackerTransportFixture) workflowSource(operation string, inputs map[string]any) string {
	var inputLines strings.Builder
	for _, key := range sortedKeys(inputs) {
		encoded, _ := json.Marshal(inputs[key])
		fmt.Fprintf(&inputLines, "      %s: %s\n", key, encoded)
	}
	return fmt.Sprintf("name: tracker-transport\nversion: \"1.0.0\"\nstates:\n  initial: operation\n  operation:\n    type: operation\n    operation: tracker.%s\n    inputs:\n%s    on_success: done\n  done:\n    type: terminal\n    status: success\n", operation, inputLines.String())
}

func (f *trackerTransportFixture) writeWorkflow(t *testing.T, name, operation string, inputs map[string]any) {
	t.Helper()
	f.writeRawWorkflow(t, name, f.workflowSource(operation, inputs))
}

func (f *trackerTransportFixture) writeRawWorkflow(t *testing.T, name, source string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "workflows", name+".yaml"), []byte(source), 0o600))
}

func (f *trackerTransportFixture) execute(t *testing.T, args []string) (string, error) {
	t.Helper()
	command, cleanup := cli.NewRootCommandAutoFacade()
	var once sync.Once
	closeCommand := func() { once.Do(cleanup) }
	f.cleanups = append(f.cleanups, closeCommand)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	commandArgs := make([]string, len(args)+2)
	commandArgs[0] = "--storage=" + f.storageDir
	commandArgs[1] = "--no-color"
	copy(commandArgs[2:], args)
	command.SetArgs(commandArgs)
	err := command.Execute()
	closeCommand()
	return output.String(), err
}

func (f *trackerTransportFixture) execution(t *testing.T, workflowName string) *workflow.ExecutionContext {
	t.Helper()
	stateStore := store.NewJSONStore(filepath.Join(f.storageDir, "states"))
	ids, err := stateStore.List(context.Background())
	require.NoError(t, err)
	for _, id := range ids {
		execution, loadErr := stateStore.Load(context.Background(), id)
		require.NoError(t, loadErr)
		if execution.WorkflowName == workflowName {
			return execution
		}
	}
	t.Fatalf("execution for %q not found", workflowName)
	return nil
}

func (f *trackerTransportFixture) calls(t *testing.T) []trackerCallRecord {
	t.Helper()
	file, err := os.Open(f.callsPath) //nolint:gosec // test-owned path.
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()
	var calls []trackerCallRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var call trackerCallRecord
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &call))
		calls = append(calls, call)
	}
	require.NoError(t, scanner.Err())
	return calls
}

func (f *trackerTransportFixture) assertConfidential(t *testing.T, output string) {
	t.Helper()
	assert.NotContains(t, output, trackerSecretMarker)
	contents, err := os.ReadFile(f.callsPath) //nolint:gosec // test-owned path.
	if err == nil {
		assert.NotContains(t, string(contents), trackerSecretMarker)
	} else {
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
}

func (f *trackerTransportFixture) closeAndRemoveBinaries(t *testing.T) {
	t.Helper()
	if f.closed {
		return
	}
	for index := len(f.cleanups) - 1; index >= 0; index-- {
		f.cleanups[index]()
	}
	f.closed = true
}

func appendTrackerCallRecord(path string, record trackerCallRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // test-owned path.
	if err != nil {
		return err
	}
	_, writeErr := file.Write(payload)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func copyExecutableAtomically(t *testing.T, source, target string) {
	t.Helper()
	sourceFile, err := os.Open(source) //nolint:gosec // os.Executable supplies the source.
	require.NoError(t, err)
	defer func() { require.NoError(t, sourceFile.Close()) }()
	temporary := target + ".tmp"
	targetFile, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700) //nolint:gosec // test-owned path.
	require.NoError(t, err)
	_, err = io.Copy(targetFile, sourceFile)
	require.NoError(t, err)
	require.NoError(t, targetFile.Close())
	require.NoError(t, os.Rename(temporary, target))
}

func trackerOperationNames() []string {
	return []string{"create_issue", "update_issue", "get_issue", "search", "transition", "add_comment"}
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedPayloadKeys(values map[string]any) []string {
	payload := make(map[string]any, len(values))
	for key, value := range values {
		if key != "backend" {
			payload[key] = value
		}
	}
	return sortedKeys(payload)
}

// isCanonicalTrackerPayload validates values in the adapter process but records
// only a match flag, so test call records prove the complete payload without
// persisting configured secrets.
func isCanonicalTrackerPayload(operation string, inputs map[string]any) bool {
	for _, scenario := range trackerCapabilityScenarios() {
		if scenario.operation == operation && mapsEqual(inputs, withoutBackend(scenario.inputs)) {
			return true
		}
	}
	// Backend-selection and single-step scenarios intentionally provide only a
	// title to create_issue; it remains a canonical payload for those workflows.
	return operation == "create_issue" && mapsEqual(inputs, map[string]any{"title": trackerSecretMarker})
}

func withoutBackend(inputs map[string]any) map[string]any {
	payload := make(map[string]any, len(inputs))
	for key, value := range inputs {
		if key != "backend" {
			payload[key] = value
		}
	}
	return payload
}

func mapsEqual(actual, expected map[string]any) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, expectedValue := range expected {
		actualValue, found := actual[key]
		if !found || fmt.Sprint(actualValue) != fmt.Sprint(expectedValue) {
			return false
		}
	}
	return true
}

func stringifyTrackerResult(result map[string]any) map[string]string {
	values := make(map[string]string, len(result))
	for key, value := range result {
		values[key] = fmt.Sprint(value)
	}
	return values
}

func parseTrackerOutput(t *testing.T, output string) map[string]string {
	t.Helper()
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		require.True(t, found, "operation output line must be key=value: %q", line)
		values[key] = value
	}
	return values
}

func singleStepStdout(t *testing.T, output string) string {
	t.Helper()
	const header = "--- [operation] stdout ---\n"
	_, stdout, found := strings.Cut(output, header)
	require.True(t, found, "single-step output must contain the operation stdout header: %q", output)
	stdout, _, found = strings.Cut(stdout, "Step 'operation' completed successfully")
	require.True(t, found, "single-step output must contain the operation success message: %q", output)
	return strings.TrimSpace(stdout)
}

func TestTrackerResumeUsesConfiguredDefaultBackend(t *testing.T) {
	const workflowName = "tracker-transport"
	const workflowID = "tracker-resume-checkpoint"

	fixture := newTrackerTransportFixture(t, "jira", trackerOperationNames())
	fixture.writeConfig(t, "jira")
	fixture.writeWorkflow(t, workflowName, "create_issue", map[string]any{"title": "Resume tracker issue"})

	saved := workflow.NewExecutionContext(workflowID, workflowName)
	saved.CurrentStep = "operation"
	saved.SetStatus(workflow.StatusFailed)
	stateStore := store.NewJSONStore(filepath.Join(fixture.storageDir, "states"))
	require.NoError(t, stateStore.Save(context.Background(), saved))
	require.Empty(t, fixture.calls(t))

	output, err := fixture.execute(t, []string{"resume", workflowID})
	require.NoError(t, err, output)
	require.Len(t, fixture.calls(t), 1)
	assert.Equal(t, workflow.StatusCompleted, fixture.execution(t, workflowName).Status)
	fixture.closeAndRemoveBinaries(t)
}
