//go:build integration

package cli_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/infrastructure/store"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/awf-project/cli/pkg/httpx"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

type telemetryCollector struct {
	collectorpb.UnimplementedTraceServiceServer
	exports chan *collectorpb.ExportTraceServiceRequest
}

func (c *telemetryCollector) Export(ctx context.Context, request *collectorpb.ExportTraceServiceRequest) (*collectorpb.ExportTraceServiceResponse, error) {
	select {
	case c.exports <- request:
		return &collectorpb.ExportTraceServiceResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func newTelemetryCollector(t *testing.T) (*telemetryCollector, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	collector := &telemetryCollector{exports: make(chan *collectorpb.ExportTraceServiceRequest, 1)}
	server := grpc.NewServer()
	collectorpb.RegisterTraceServiceServer(server, collector)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return collector, listener.Addr().String()
}

func TestRunCommand_CompletesWorkWithoutConfiguredTimeout(t *testing.T) {
	env := newRootCommandTestEnvironment(t)
	observed := observeRootCommandExecutor(t, &env)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := filepath.Join(t.TempDir(), "ready")
	release := filepath.Join(t.TempDir(), "release")
	marker := filepath.Join(t.TempDir(), "completed")
	env.writeWorkflow(t, "unlimited-work", fmt.Sprintf(`name: unlimited-work
states:
  initial: work
  work:
    type: command
    command: touch %s; while [ ! -f %s ]; do sleep 0.01; done; touch %s
    on_success: done
  done:
    type: terminal
`, interpolation.ShellEscape(ready), interpolation.ShellEscape(release), interpolation.ShellEscape(marker)))

	completed := make(chan rootCommandOutcome, 1)
	outcomeDrained := false
	t.Cleanup(func() {
		// Always unblock the shell and cancel the command if an assertion aborts
		// the test before the normal release-and-receive path.
		_ = os.WriteFile(release, nil, 0o600)
		cancel()
		if !outcomeDrained {
			select {
			case <-completed:
			case <-time.After(5 * time.Second):
				t.Error("root command did not stop during cleanup")
			}
		}
	})
	go func() {
		output, err := env.execute(t, ctx, "unlimited-work")
		completed <- rootCommandOutcome{output: output, err: err}
	}()

	require.Eventually(t, func() bool { return timeoutFileExists(ready) }, 2*time.Second, 10*time.Millisecond)
	_, hasDeadline := (<-observed).Deadline()
	assert.False(t, hasDeadline, "omission must not install a hidden execution deadline")
	select {
	case result := <-completed:
		t.Fatalf("deadline-free work stopped during canary interval: %v\n%s", result.err, result.output)
	case <-time.After(150 * time.Millisecond):
	}
	require.NoError(t, os.WriteFile(release, nil, 0o600))
	result := receiveRootCommandOutcome(t, completed)
	outcomeDrained = true
	require.NoError(t, result.err, result.output)
	assert.FileExists(t, marker)
	assertPersistedExecution(t, env.loadExecutionByWorkflowName(t, "unlimited-work"), workflow.StatusCompleted)
}

func TestRunCommand_ExplicitZeroDisablesInheritedTemplateTimeout(t *testing.T) {
	env := newRootCommandTestEnvironment(t)
	observed := observeRootCommandExecutor(t, &env)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	templateDir := filepath.Join(env.storageDir, "templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o700))
	release := filepath.Join(t.TempDir(), "release")
	marker := filepath.Join(t.TempDir(), "completed")
	templatePath := filepath.Join(templateDir, "bounded-command.yaml")
	templateSource := fmt.Sprintf(`name: bounded-command
states:
  initial: bounded-command
  bounded-command:
    type: command
    command: while [ ! -f %s ]; do sleep 0.01; done; touch %s
    timeout: 1s
`, interpolation.ShellEscape(release), interpolation.ShellEscape(marker))
	workflowSource := `name: explicit-zero
states:
  initial: work
  work:
    type: command
    use_template: bounded-command
    timeout: 0
    command: echo consumer-placeholder
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(templatePath, []byte(templateSource), 0o600))
	workflowPath := env.writeWorkflow(t, "explicit-zero", workflowSource)
	templateBefore, err := os.ReadFile(templatePath)
	require.NoError(t, err)
	workflowBefore, err := os.ReadFile(workflowPath)
	require.NoError(t, err)

	completed := make(chan rootCommandOutcome, 1)
	go func() {
		output, runErr := env.execute(t, ctx, "explicit-zero")
		completed <- rootCommandOutcome{output: output, err: runErr}
	}()
	select {
	case result := <-completed:
		t.Fatalf("explicit zero inherited the template deadline: %v\n%s", result.err, result.output)
	case <-time.After(1200 * time.Millisecond):
	}
	_, hasDeadline := (<-observed).Deadline()
	assert.False(t, hasDeadline, "explicit zero suppresses the inherited template deadline")
	require.NoError(t, os.WriteFile(release, nil, 0o600))
	result := receiveRootCommandOutcome(t, completed)
	require.NoError(t, result.err, result.output)
	assert.FileExists(t, marker)
	assertFileBytesEqual(t, templatePath, templateBefore)
	assertFileBytesEqual(t, workflowPath, workflowBefore)
	assertPersistedExecution(t, env.loadExecutionByWorkflowName(t, "explicit-zero"), workflow.StatusCompleted)
}

func TestRunCommand_StopsWorkAtExplicitTimeout(t *testing.T) {
	env := newRootCommandTestEnvironment(t)
	collector, telemetryEndpoint := newTelemetryCollector(t)
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	t.Setenv("AWF_AUDIT_LOG", auditPath)
	const (
		workflowInputSecret = "workflow-input-secret"
		promptSecret        = "prompt-secret"
		commandSecret       = "command-output-secret"
		environmentSecret   = "environment-value-secret"
		authorizationSecret = "authorization-header-secret"
		tokenSecret         = "token-secret"
		panicSecret         = "panic-payload-secret"
		requestSecret       = "request-body-secret"
		pluginConfigSecret  = "plugin-config-secret"
	)
	env.writeWorkflow(t, "local-timeout", fmt.Sprintf(`name: local-timeout
inputs:
  - name: credential
    type: string
    default: %s
states:
  initial: work
  work:
    type: command
    command: printf '%%s%%s%%s%%s%%s' '%s' "$PRIVATE_VALUE" "$AUTHORIZATION" "$API_TOKEN" "$PANIC_PAYLOAD"; while :; do sleep 0.01; done
    prompt: %s
    timeout: 1
    on_success: done
  done:
    type: terminal
`, workflowInputSecret, commandSecret, promptSecret))
	t.Setenv("PRIVATE_VALUE", environmentSecret)
	t.Setenv("AUTHORIZATION", authorizationSecret)
	t.Setenv("API_TOKEN", tokenSecret)
	t.Setenv("PANIC_PAYLOAD", panicSecret)

	startedAt := time.Now()
	output, err := env.executeWithRunFlags(t, context.Background(), "local-timeout",
		"--otel-exporter", telemetryEndpoint,
		"--otel-service-name", "timeout-lifecycle-test",
	)
	elapsed := time.Since(startedAt)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	var structuredErr *domainerrors.StructuredError
	require.ErrorAs(t, err, &structuredErr)
	assert.Equal(t, domainerrors.ErrorCodeExecutionCommandTimeout, structuredErr.Code)
	assert.Equal(t, 3, structuredErr.ExitCode())
	assert.Equal(t, map[string]any{"timeout_owner": "step", "timeout_seconds": 1}, structuredErr.Details)
	assert.Less(t, elapsed, 5*time.Second)
	execution := env.loadExecutionByWorkflowName(t, "local-timeout")
	assertPersistedExecution(t, execution, workflow.StatusCancelled)
	auditEvents, auditErr := readSignalAuditEvents(auditPath)
	require.NoError(t, auditErr)
	require.Len(t, auditEvents, 2)
	var completion *workflow.AuditEvent
	for i := range auditEvents {
		if auditEvents[i].Event == workflow.EventWorkflowCompleted {
			completion = &auditEvents[i]
			break
		}
	}
	require.NotNil(t, completion)
	assert.Equal(t, execution.HistoryRecordID, completion.ExecutionID, "audit must correlate with the durable attempt")
	assert.Equal(t, "failure", completion.Status)
	require.NotNil(t, completion.ExitCode)
	assert.Equal(t, 3, *completion.ExitCode)
	assert.Contains(t, completion.Error, "timeout_owner: step")
	assert.Contains(t, completion.Error, "timeout_seconds: 1")
	auditBytes, readErr := os.ReadFile(auditPath)
	require.NoError(t, readErr)
	var telemetryRequest *collectorpb.ExportTraceServiceRequest
	select {
	case telemetryRequest = <-collector.exports:
	case <-time.After(time.Second):
		t.Fatal("public CLI lifecycle did not flush telemetry before shutdown returned")
	}
	require.NotEmpty(t, telemetryRequest.ResourceSpans,
		"telemetry shutdown must flush the public workflow lifecycle spans")
	assert.Less(t, elapsed, 5*time.Second,
		"public CLI lifecycle must complete telemetry shutdown within its bounded cleanup window")
	telemetryEvidence := fmt.Sprintf("%+v", telemetryRequest)
	assert.Contains(t, telemetryEvidence, "shell.execute",
		"telemetry may identify the operation that timed out")
	assertConfidentialValuesAbsent(t, output+"\n"+fmt.Sprintf("%+v", err)+"\n"+string(auditBytes)+"\n"+telemetryEvidence,
		workflowInputSecret, promptSecret, commandSecret, environmentSecret,
		authorizationSecret, tokenSecret, panicSecret, requestSecret, pluginConfigSecret)
}

func TestRunCommand_NestedWorkflowUsesShortestConfiguredDeadline(t *testing.T) {
	env := newRootCommandTestEnvironment(t)
	marker := filepath.Join(t.TempDir(), "child-completed")
	env.writeWorkflow(t, "slow-child", fmt.Sprintf(`name: slow-child
states:
  initial: work
  work:
    type: command
    timeout: 300
    command: while :; do sleep 0.01; done; touch %s
    on_success: done
  done:
    type: terminal
`, interpolation.ShellEscape(marker)))
	env.writeWorkflow(t, "short-parent", `name: short-parent
states:
  initial: call-child
  call-child:
    type: call_workflow
    workflow: slow-child
    timeout: 1
    on_success: done
  done:
    type: terminal
`)

	startedAt := time.Now()
	output, err := env.execute(t, context.Background(), "short-parent")
	elapsed := time.Since(startedAt)
	require.Error(t, err, output)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 5*time.Second, "nested execution must use the one-second parent deadline")
	assert.NoFileExists(t, marker)
	parent := env.loadExecutionByWorkflowName(t, "short-parent")
	assertPersistedExecution(t, parent, workflow.StatusCancelled)
	callState, ok := parent.GetStepState("call-child")
	require.True(t, ok, "nested routing must persist the call_workflow state")
	assert.Equal(t, workflow.StatusFailed, callState.Status)
	child := env.loadExecutionByWorkflowName(t, "slow-child")
	assertPersistedExecution(t, child, workflow.StatusCancelled)
}

type rootCommandOutcome struct {
	output string
	err    error
}

func receiveRootCommandOutcome(
	t *testing.T,
	completed <-chan rootCommandOutcome,
) rootCommandOutcome {
	t.Helper()
	select {
	case result := <-completed:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("root command did not finish")
		return rootCommandOutcome{}
	}
}

func assertPersistedExecution(t *testing.T, execution *workflow.ExecutionContext, status workflow.ExecutionStatus) {
	t.Helper()
	require.NotNil(t, execution)
	assert.Equal(t, status, execution.Status)
	assert.NotEmpty(t, execution.WorkflowID)
	_, err := uuid.Parse(execution.WorkflowID)
	assert.NoError(t, err, "persisted workflow ID must remain stable and machine-readable")
}

func assertFileBytesEqual(t *testing.T, path string, before []byte) {
	t.Helper()
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func assertConfidentialValuesAbsent(t *testing.T, captured string, values ...string) {
	t.Helper()
	for _, value := range values {
		assert.False(t, strings.Contains(captured, value), "captured diagnostics disclosed confidential value %q", value)
	}
}

func timeoutFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// observedHTTPDoer decorates the production client, preserving its transport and
// timeout configuration while observing the context passed to the client.
type observedHTTPDoer struct {
	next     httpx.HTTPDoer
	observed chan<- context.Context
}

func (d observedHTTPDoer) Do(request *http.Request) (*http.Response, error) {
	d.observed <- request.Context()
	return d.next.Do(request)
}

func TestRunCommand_HTTPRequestCompletesWithoutConfiguredTimeout(t *testing.T) {
	testRootHTTPDeadline(t, rootHTTPOperation)
	t.Run("response body remains capped", func(t *testing.T) {
		env := newRootCommandTestEnvironment(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, strings.Repeat("x", (1<<20)+1))
		}))
		defer server.Close()
		env.writeWorkflow(t, "capped-http", fmt.Sprintf("name: capped-http\nstates:\n  initial: request\n  request:\n    type: operation\n    operation: http.request\n    inputs:\n      method: GET\n      url: %q\n    on_success: verify\n  verify:\n    type: command\n    command: test \"{{states.request.Response.body_truncated}}\" = true\n    on_success: done\n  done:\n    type: terminal\n", server.URL))
		output, err := env.execute(t, context.Background(), "capped-http")
		require.NoError(t, err, output)
		state := env.loadExecutionByWorkflowName(t, "capped-http")
		step, ok := state.GetStepState("request")
		require.True(t, ok)
		require.NotEmpty(t, step.Response, "response contents must remain available after resume")
		assert.Equal(t, true, step.Response["body_truncated"])
		verify, ok := state.GetStepState("verify")
		require.True(t, ok)
		assert.Equal(t, workflow.StatusCompleted, verify.Status)
	})
}

func TestRunCommand_OpenAICompatibleAgentCompletesWithoutConfiguredTimeout(t *testing.T) {
	// The local endpoint models Ollama's non-streaming, delayed first response.
	testRootHTTPDeadline(t, rootOpenAIAgent)
}

type rootHTTPAdapter uint8

const (
	rootHTTPOperation rootHTTPAdapter = iota
	rootOpenAIAgent
)

type rootDeadlineScenario struct {
	name           string
	timeoutYAML    string
	parentDeadline bool
	wantErr        error
}

func rootDeadlineScenarios() []rootDeadlineScenario {
	return []rootDeadlineScenario{
		{name: "unlimited"},
		{name: "parent cancellation", parentDeadline: true, wantErr: context.Canceled},
		{name: "explicit timeout", timeoutYAML: "    timeout: 1\n", wantErr: context.DeadlineExceeded},
	}
}

func (s rootDeadlineScenario) context() (context.Context, context.CancelFunc) {
	if s.parentDeadline {
		return context.WithTimeout(context.Background(), time.Minute)
	}
	return context.WithCancel(context.Background())
}

func (s rootDeadlineScenario) assertContext(t *testing.T, actual, parent context.Context, tolerance time.Duration) {
	t.Helper()
	deadline, bounded := actual.Deadline()
	s.assertDeadline(t, bounded, deadline, parent, tolerance)
}

func (s rootDeadlineScenario) assertDeadline(t *testing.T, bounded bool, deadline time.Time, parent context.Context, tolerance time.Duration) {
	t.Helper()
	assert.Equal(t, s.parentDeadline || s.timeoutYAML != "", bounded)
	if s.parentDeadline {
		parentDeadline, ok := parent.Deadline()
		require.True(t, ok)
		assert.WithinDuration(t, parentDeadline, deadline, tolerance)
	}
}

func (s rootDeadlineScenario) assertResult(t *testing.T, result rootCommandOutcome) workflow.ExecutionStatus {
	t.Helper()
	if s.wantErr == nil {
		require.NoError(t, result.err, result.output)
		return workflow.StatusCompleted
	}
	require.ErrorIs(t, result.err, s.wantErr, result.output)
	return workflow.StatusCancelled
}

func testRootHTTPDeadline(t *testing.T, adapter rootHTTPAdapter) {
	t.Helper()
	for _, scenario := range rootDeadlineScenarios() {
		t.Run(scenario.name, func(t *testing.T) { testRootHTTPDeadlineMode(t, adapter, scenario) })
	}
}

func observeRootHTTPClient(t *testing.T, env *rootCommandTestEnvironment) <-chan context.Context {
	t.Helper()
	observed := make(chan context.Context, 1)
	env.options = []cli.RootOption{cli.WithExecutionSetupOptions(application.WithExecutionHTTPClientOptions(httpx.WithDoerDecorator(func(doer httpx.HTTPDoer) httpx.HTTPDoer {
		client, ok := doer.(*http.Client)
		require.True(t, ok, "instrument the actual production client")
		assert.Zero(t, client.Timeout)
		transport, ok := client.Transport.(*http.Transport)
		require.True(t, ok)
		assert.Equal(t, 10*time.Second, transport.TLSHandshakeTimeout, "TLS setup stays bounded without limiting execution")
		assert.Zero(t, transport.ResponseHeaderTimeout)
		return observedHTTPDoer{next: doer, observed: observed}
	})))}
	return observed
}

func (a rootHTTPAdapter) writeWorkflow(t *testing.T, env rootCommandTestEnvironment, endpoint, timeout string) {
	t.Helper()
	step := fmt.Sprintf("    type: operation\n    operation: http.request\n    inputs:\n      method: GET\n      url: %q\n", endpoint)
	next := "    on_success: done\n  done:\n    type: terminal\n"
	if a == rootOpenAIAgent {
		step = fmt.Sprintf("    type: agent\n    provider: openai_compatible\n    prompt: a private prompt\n    options:\n      model: local-test\n      base_url: %q\n      api_key: private-api-key\n", endpoint)
		next = "    on_success: verify\n  verify:\n    type: command\n    command: test \"{{states.work.Output}}\" = agent-completed\n    on_success: done\n  done:\n    type: terminal\n"
	}
	env.writeWorkflow(t, "adapter-deadline", "name: adapter-deadline\nstates:\n  initial: work\n  work:\n"+step+timeout+next)
}

func startRootWorkflow(t *testing.T, ctx context.Context, env rootCommandTestEnvironment, name string, flags ...string) <-chan rootCommandOutcome {
	t.Helper()
	completed := make(chan rootCommandOutcome, 1)
	go func() {
		out, err := env.executeWithRunFlags(t, ctx, name, flags...)
		completed <- rootCommandOutcome{out, err}
	}()
	return completed
}

func waitRootWorkStarted(t *testing.T, started <-chan struct{}, completed <-chan rootCommandOutcome) {
	t.Helper()
	select {
	case <-started:
	case result := <-completed:
		t.Fatalf("adapter did not start: %v %s", result.err, result.output)
	case <-time.After(5 * time.Second):
		t.Fatal("adapter did not start")
	}
}

func assertRootWorkStillRunning(t *testing.T, completed <-chan rootCommandOutcome) {
	t.Helper()
	select {
	case result := <-completed:
		t.Fatalf("adapter stopped before release: %v %s", result.err, result.output)
	case <-time.After(30 * time.Millisecond):
	}
}

func testRootHTTPDeadlineMode(t *testing.T, adapter rootHTTPAdapter, scenario rootDeadlineScenario) {
	t.Helper()
	env := newRootCommandTestEnvironment(t)
	t.Setenv("AWF_MCP_PROXY_NO_PURGE", "1")
	started, stopped, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	releaseWork := func() { releaseOnce.Do(func() { close(release) }) }
	server := httptest.NewServer(rootHTTPDeadlineHandler(adapter, started, stopped, release))
	t.Cleanup(func() { releaseWork(); server.CloseClientConnections(); server.Close() })
	observed := observeRootHTTPClient(t, &env)
	adapter.writeWorkflow(t, env, server.URL, scenario.timeoutYAML)
	ctx, cancel := scenario.context()
	defer cancel()
	completed := startRootWorkflow(t, ctx, env, "adapter-deadline")
	waitRootWorkStarted(t, started, completed)
	select {
	case actual := <-observed:
		scenario.assertContext(t, actual, ctx, 0)
	case <-time.After(time.Second):
		t.Fatal("client context not observed")
	}
	if scenario.wantErr == nil {
		assertRootWorkStillRunning(t, completed)
		releaseWork()
	}
	if scenario.parentDeadline {
		cancel()
	}
	result := receiveRootCommandOutcome(t, completed)
	expected := scenario.assertResult(t, result)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("request outlived root command")
	}
	state := env.loadExecutionByWorkflowName(t, "adapter-deadline")
	assertPersistedExecution(t, state, expected)
	if adapter == rootOpenAIAgent && scenario.wantErr == nil {
		stepState, ok := state.GetStepState("work")
		require.True(t, ok)
		assert.Equal(t, "agent-completed", stepState.Output, "agent output must remain available after resume")
	}
	assertConfidentialValuesAbsent(t, result.output+fmt.Sprint(result.err), "private-api-key", "a private prompt")
}

func rootHTTPDeadlineHandler(adapter rootHTTPAdapter, started, stopped chan struct{}, release <-chan struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		close(started)
		defer close(stopped)
		select {
		case <-request.Context().Done():
			return
		case <-release:
		}
		w.Header().Set("Content-Type", "application/json")
		body := "http-completed"
		if adapter == rootOpenAIAgent {
			body = "{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"agent-completed\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}"
		}
		_, _ = io.WriteString(w, body)
	})
}

type rootPluginFixture struct {
	env rootCommandTestEnvironment
	dir string
}

type rootPluginDeadline struct {
	Bounded  bool
	Deadline time.Time
}

func newRootPluginFixture(t *testing.T) rootPluginFixture {
	t.Helper()
	fixture := rootPluginFixture{env: newRootCommandTestEnvironment(t), dir: t.TempDir()}
	t.Setenv("AWF_MCP_PROXY_NO_PURGE", "1")
	t.Setenv("AWF_TIMEOUT_FIXTURE_DIR", fixture.dir)
	pluginDir := filepath.Join(fixture.dir, "plugins", "deadline-fixture")
	require.NoError(t, os.MkdirAll(pluginDir, 0o700))
	binary, err := os.Executable()
	require.NoError(t, err)
	require.NoError(t, os.Symlink(binary, filepath.Join(pluginDir, "awf-plugin-deadline-fixture")))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte("name: deadline-fixture\nversion: 1.0.0\nawf_version: \">=0.1.0\"\ncapabilities:\n  - step_types\n"), 0o600))
	fixture.env.options = []cli.RootOption{func(cfg *cli.Config) { cfg.PluginsDir = filepath.Dir(pluginDir) }}
	fixture.env.writeWorkflow(t, "plugin-deadline", "name: plugin-deadline\nstates:\n  initial: work\n  work:\n    type: deadline-fixture.wait\n    on_success: verify\n  verify:\n    type: command\n    command: test \"{{states.work.Output}}\" = plugin-completed\n    on_success: done\n  done:\n    type: terminal\n")
	return fixture
}

func (f rootPluginFixture) waitForExecution(t *testing.T, completed <-chan rootCommandOutcome) rootPluginDeadline {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case result := <-completed:
			t.Fatalf("plugin did not start: %v %s", result.err, result.output)
		default:
		}
		return timeoutFileExists(filepath.Join(f.dir, "execution"))
	}, 5*time.Second, 10*time.Millisecond)
	evidence, err := os.ReadFile(filepath.Join(f.dir, "execution"))
	require.NoError(t, err)
	var observed rootPluginDeadline
	require.NoError(t, json.Unmarshal(evidence, &observed))
	discovery, err := os.ReadFile(filepath.Join(f.dir, "discovery"))
	require.NoError(t, err)
	assert.Equal(t, "bounded", string(discovery), "discovery remains independently bounded")
	return observed
}

func TestRunCommand_CustomPluginStepCompletesWithoutDiscoveryTimeout(t *testing.T) {
	t.Run("unregistered type is rejected before execution", func(t *testing.T) {
		env := newRootCommandTestEnvironment(t)
		env.writeWorkflow(t, "unknown-plugin", "name: unknown-plugin\nstates:\n  initial: work\n  work:\n    type: missing.wait\n    on_success: done\n  done:\n    type: terminal\n")
		_, err := env.execute(t, context.Background(), "unknown-plugin")
		require.ErrorContains(t, err, "unknown step type")
	})
	for _, scenario := range rootDeadlineScenarios()[:2] {
		t.Run(scenario.name, func(t *testing.T) { testRootPluginDeadline(t, scenario) })
	}
}

func testRootPluginDeadline(t *testing.T, scenario rootDeadlineScenario) {
	t.Helper()
	fixture := newRootPluginFixture(t)
	ctx, cancel := scenario.context()
	defer cancel()
	completed := startRootWorkflow(t, ctx, fixture.env, "plugin-deadline", "--skip-plugins=false")
	observed := fixture.waitForExecution(t, completed)
	scenario.assertDeadline(t, observed.Bounded, observed.Deadline, ctx, 20*time.Millisecond)
	if scenario.wantErr == nil {
		assertRootWorkStillRunning(t, completed)
		require.NoError(t, os.WriteFile(filepath.Join(fixture.dir, "release"), nil, 0o600))
	} else {
		cancel()
	}
	result := receiveRootCommandOutcome(t, completed)
	expected := scenario.assertResult(t, result)
	require.Eventually(t, func() bool { return timeoutFileExists(filepath.Join(fixture.dir, "stopped")) }, time.Second, 5*time.Millisecond)
	state := fixture.env.loadExecutionByWorkflowName(t, "plugin-deadline")
	assertPersistedExecution(t, state, expected)
	if scenario.wantErr == nil {
		step, ok := state.GetStepState("work")
		require.True(t, ok)
		assert.Empty(t, step.Output, "plugin output must not be persisted")
	}
}

type rootObservingExecutor struct {
	next     ports.CommandExecutor
	contexts chan<- context.Context
	finished chan struct{}
}

func (e *rootObservingExecutor) Execute(ctx context.Context, command *ports.Command) (*ports.CommandResult, error) {
	e.contexts <- ctx
	defer close(e.finished)
	return e.next.Execute(ctx, command)
}

func TestRunCommand_ParentCancellationStopsActiveWork(t *testing.T) {
	env := newRootCommandTestEnvironment(t)
	t.Setenv("AWF_MCP_PROXY_NO_PURGE", "1")
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	t.Setenv("AWF_AUDIT_LOG", auditPath)
	observed := make(chan context.Context, 1)
	finished := make(chan struct{})
	env.options = []cli.RootOption{cli.WithCommandExecutor(&rootObservingExecutor{next: executor.NewShellExecutor(), contexts: observed, finished: finished})}
	pidPath := filepath.Join(t.TempDir(), "pid")
	t.Setenv("PRIVATE_PARENT_ENV", "PRIVATE_PARENT_ENV_VALUE")
	env.writeWorkflow(t, "parent-cancel", fmt.Sprintf("name: parent-cancel\ninputs:\n  - name: credential\n    type: string\n    default: PRIVATE_PARENT_INPUT\nstates:\n  initial: work\n  work:\n    type: command\n    command: printf PRIVATE_PARENT_OUTPUT; printenv PRIVATE_PARENT_ENV; echo $$ > %s; while :; do sleep 0.01; done\n    prompt: PRIVATE_PARENT_PROMPT\n    on_success: done\n  done:\n    type: terminal\n", interpolation.ShellEscape(pidPath)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := make(chan rootCommandOutcome, 1)
	go func() { out, err := env.execute(t, ctx, "parent-cancel"); completed <- rootCommandOutcome{out, err} }()
	require.Eventually(t, func() bool { return timeoutFileExists(pidPath) }, 5*time.Second, 10*time.Millisecond)
	actual := <-observed
	_, bounded := actual.Deadline()
	assert.False(t, bounded)
	pidBytes, err := os.ReadFile(pidPath)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	require.NoError(t, err)
	cancel()
	result := receiveRootCommandOutcome(t, completed)
	require.ErrorIs(t, result.err, context.Canceled)
	assert.NotErrorIs(t, result.err, context.DeadlineExceeded)
	var structured *domainerrors.StructuredError
	if errors.As(result.err, &structured) {
		assert.NotContains(t, structured.Details, "timeout_owner")
		assert.NotContains(t, structured.Details, "timeout_seconds")
	}
	select {
	case <-finished:
	default:
		t.Fatal("executor outlived the root command")
	}
	require.Eventually(t, func() bool { return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }, time.Second, 5*time.Millisecond, "child process was not reaped")
	state := env.loadExecutionByWorkflowName(t, "parent-cancel")
	assertPersistedExecution(t, state, workflow.StatusCancelled)
	assert.Equal(t, 3, state.ExitCode)
	events, err := readSignalAuditEvents(auditPath)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, workflow.EventWorkflowStarted, events[0].Event)
	assert.Equal(t, workflow.EventWorkflowCompleted, events[1].Event)
	assert.Equal(t, state.HistoryRecordID, events[1].ExecutionID)
	assert.NotContains(t, events[1].Error, "timeout_owner")
	history, err := store.NewSQLiteHistoryStore(filepath.Join(env.storageDir, "history.db"))
	require.NoError(t, err)
	defer history.Close()
	records, err := history.List(context.Background(), &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, state.HistoryRecordID, records[0].ID)
	assert.Equal(t, state.WorkflowID, records[0].WorkflowID)
	assert.Equal(t, 3, records[0].ExitCode)
	assert.Contains(t, records[0].ErrorMessage, context.Canceled.Error())
	stateBytes, err := json.Marshal(state)
	require.NoError(t, err)
	auditBytes, err := os.ReadFile(auditPath)
	require.NoError(t, err)
	assertConfidentialValuesAbsent(t, string(stateBytes)+string(auditBytes)+fmt.Sprint(records)+result.output+fmt.Sprint(result.err), "PRIVATE_PARENT_INPUT", "PRIVATE_PARENT_OUTPUT", "PRIVATE_PARENT_ENV_VALUE", "PRIVATE_PARENT_PROMPT")
}

func observeRootCommandExecutor(t *testing.T, env *rootCommandTestEnvironment) <-chan context.Context {
	t.Helper()
	t.Setenv("AWF_MCP_PROXY_NO_PURGE", "1")
	observed := make(chan context.Context, 1)
	env.options = append(env.options, cli.WithCommandExecutor(&rootObservingExecutor{
		next: executor.NewShellExecutor(), contexts: observed, finished: make(chan struct{}),
	}))
	return observed
}
