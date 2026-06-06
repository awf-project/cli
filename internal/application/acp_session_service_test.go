package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// MockWorkflowRepository implements ports.WorkflowRepository for testing.
type MockWorkflowRepository struct {
	mock.Mock
}

func (m *MockWorkflowRepository) Load(ctx context.Context, name string) (*workflow.Workflow, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workflow.Workflow), args.Error(1)
}

func (m *MockWorkflowRepository) List(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockWorkflowRepository) ListWithSource(ctx context.Context) ([]ports.WorkflowInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ports.WorkflowInfo), args.Error(1)
}

func (m *MockWorkflowRepository) Exists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

// MockWorkflowProvider implements application.WorkflowProvider (the pack-aware lister/loader)
// for testing the ACP available-commands discovery path.
type MockWorkflowProvider struct {
	mock.Mock
}

func (m *MockWorkflowProvider) ListAllWorkflows(ctx context.Context) ([]workflow.WorkflowEntry, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]workflow.WorkflowEntry), args.Error(1)
}

func (m *MockWorkflowProvider) GetWorkflow(ctx context.Context, name string) (*workflow.Workflow, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workflow.Workflow), args.Error(1)
}

// fakeRunner is a WorkflowRunner test double that records the dispatched workflow and
// inputs and optionally blocks until its run context is cancelled (to exercise cancel).
type fakeRunner struct {
	mu      sync.Mutex
	calls   int
	name    string
	inputs  map[string]any
	block   bool
	execCtx *workflow.ExecutionContext
	err     error
}

func (f *fakeRunner) Run(ctx context.Context, name string, inputs map[string]any) (*workflow.ExecutionContext, error) {
	f.mu.Lock()
	f.calls++
	f.name = name
	f.inputs = inputs
	block := f.block
	f.mu.Unlock()
	if block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.execCtx, f.err
}

func (f *fakeRunner) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fakeInputResponder implements ACPInputResponder, recording routed continuation turns.
type fakeInputResponder struct {
	mu        sync.Mutex
	responses []string
	onPark    func()
	onUnpark  func()
}

func (f *fakeInputResponder) ReadInput(context.Context) (string, error) { return "", nil }

func (f *fakeInputResponder) Respond(text string) {
	f.mu.Lock()
	f.responses = append(f.responses, text)
	f.mu.Unlock()
}

// SetParkHooks records the park hooks so tests can drive park/unpark accounting and verify
// the CRITIQUE-3 wiring bumps the session's ParkedTurnCount.
func (f *fakeInputResponder) SetParkHooks(onPark, onUnpark func()) {
	f.mu.Lock()
	f.onPark = onPark
	f.onUnpark = onUnpark
	f.mu.Unlock()
}

// parkHooks returns the recorded hooks (nil until SetParkHooks ran).
func (f *fakeInputResponder) parkHooks() (onPark, onUnpark func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.onPark, f.onUnpark
}

func (f *fakeInputResponder) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.responses...)
}

// fakeEmitter captures session/update notifications emitted by the service so tests can
// assert on the agent text streamed back to the editor.
type fakeEmitter struct {
	mu      sync.Mutex
	updates []fakeUpdate
}

type fakeUpdate struct {
	sessionID string
	kind      string
	fields    map[string]any
}

func (e *fakeEmitter) EmitSessionUpdate(_ context.Context, sessionID, kind string, fields map[string]any) error {
	e.mu.Lock()
	e.updates = append(e.updates, fakeUpdate{sessionID: sessionID, kind: kind, fields: fields})
	e.mu.Unlock()
	return nil
}

// agentText concatenates the text of every agent_message_chunk update.
func (e *fakeEmitter) agentText() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var b strings.Builder
	for _, u := range e.updates {
		if u.kind != "agent_message_chunk" {
			continue
		}
		if content, ok := u.fields["content"].(map[string]any); ok {
			if txt, ok := content["text"].(string); ok {
				b.WriteString(txt)
			}
		}
	}
	return b.String()
}

// testWorkflow creates a simple test workflow with required and optional inputs.
func testWorkflow(name string) *workflow.Workflow {
	return &workflow.Workflow{
		Name:        name,
		Description: "Test workflow for " + name,
		Version:     "1.0.0",
		Initial:     "start",
		Inputs: []workflow.Input{
			{Name: "required_input", Type: "string", Description: "A required input", Required: true},
			{Name: "optional_input", Type: "string", Description: "An optional input", Required: false, Default: "default_value"},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}
}

func resultMap(t *testing.T, result any) map[string]any {
	t.Helper()
	m, ok := result.(map[string]any)
	require.True(t, ok, "result must be a map[string]any, got %T", result)
	return m
}

// stopReasonOf extracts the stopReason from a PromptResult value returned by HandleSessionPrompt
// or HandleSessionCancel. Using the typed struct avoids stringly-typed map access.
func stopReasonOf(t *testing.T, result any) string {
	t.Helper()
	pr, ok := result.(PromptResult)
	require.True(t, ok, "result must be a PromptResult, got %T", result)
	return pr.StopReason
}

// TestACPSessionService_HandleSessionNew_AdvertisesAllWorkflows verifies session/new echoes
// the sessionId and advertises every discovered workflow as a slash command.
func TestACPSessionService_HandleSessionNew_AdvertisesAllWorkflows(t *testing.T) {
	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()

	infos := []ports.WorkflowInfo{
		{Name: "workflow-1", Source: ports.SourceLocal, Path: "/path/to/workflow-1.yaml"},
		{Name: "workflow-2", Source: ports.SourceGlobal, Path: "/path/to/workflow-2.yaml"},
	}
	mockRepo.On("ListWithSource", ctx).Return(infos, nil)
	mockRepo.On("Load", ctx, "workflow-1").Return(testWorkflow("workflow-1"), nil)
	mockRepo.On("Load", ctx, "workflow-2").Return(testWorkflow("workflow-2"), nil)

	svc := &ACPSessionService{workflowRepo: mockRepo, logger: ports.NopLogger{}}

	result, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/home/user","mcpServers":[]}`))
	require.Nil(t, acpErr)

	m := resultMap(t, result)
	sessionID, _ := m["sessionId"].(string)
	assert.True(t, strings.HasPrefix(sessionID, "sess_"), "agent must mint a sessionId (got %q)", sessionID)

	commands, ok := m["commands"].([]WorkflowSlashCommand)
	require.True(t, ok, "commands must be []WorkflowSlashCommand, got %T", m["commands"])
	names := make([]string, 0, len(commands))
	for _, c := range commands {
		names = append(names, c.Name)
	}
	assert.ElementsMatch(t, []string{"workflow-1", "workflow-2"}, names)
	mockRepo.AssertExpectations(t)
}

// availableCommandNames extracts the advertised slash-command names from the last
// available_commands_update notification captured by the fake emitter.
func (e *fakeEmitter) availableCommandNames() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var names []string
	for _, u := range e.updates {
		if u.kind != "available_commands_update" {
			continue
		}
		cmds, ok := u.fields["availableCommands"].([]WorkflowSlashCommand)
		if !ok {
			continue
		}
		names = names[:0]
		for _, c := range cmds {
			names = append(names, c.Name)
		}
	}
	return names
}

// TestACPSessionService_HandleSessionNew_AdvertisesPackWorkflows verifies that when a pack-aware
// WorkflowProvider is wired, session/new advertises pack workflows ("packName/workflowName")
// alongside local ones — both in the result and in the available_commands_update notification.
// This is the F102 pack-discovery gap: the ACP server consumed the pack-blind WorkflowRepository
// directly, so installed pack workflows were never surfaced as slash commands.
func TestACPSessionService_HandleSessionNew_AdvertisesPackWorkflows(t *testing.T) {
	ctx := context.Background()
	provider := new(MockWorkflowProvider)

	entries := []workflow.WorkflowEntry{
		{Name: "local-wf", Source: "local", Scope: "local", Workflow: "local-wf"},
		{Name: "acme/deploy", Source: "pack", Scope: "acme", Workflow: "deploy", Description: "Deploy via acme"},
	}
	provider.On("ListAllWorkflows", ctx).Return(entries, nil)
	provider.On("GetWorkflow", ctx, "local-wf").Return(testWorkflow("local-wf"), nil)
	provider.On("GetWorkflow", ctx, "acme/deploy").Return(testWorkflow("acme/deploy"), nil)

	emitter := &fakeEmitter{}
	svc := &ACPSessionService{logger: ports.NopLogger{}, emitter: emitter}
	svc.SetWorkflowProvider(provider)

	result, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/home/user","mcpServers":[]}`))
	require.Nil(t, acpErr)

	commands, ok := resultMap(t, result)["commands"].([]WorkflowSlashCommand)
	require.True(t, ok, "commands must be []WorkflowSlashCommand, got %T", resultMap(t, result)["commands"])
	names := make([]string, 0, len(commands))
	for _, c := range commands {
		names = append(names, c.Name)
	}
	// Pack workflows are advertised with a ':' namespace separator (slash-safe for the editor's
	// command menu), not the internal "pack/workflow" form whose '/' breaks the slash palette.
	assert.ElementsMatch(t, []string{"local-wf", "acme:deploy"}, names,
		"pack workflow must be advertised with a ':' namespace separator alongside local workflows")

	assert.Contains(t, emitter.availableCommandNames(), "acme:deploy",
		"pack workflow must appear in the available_commands_update notification with the ':' separator")
	provider.AssertExpectations(t)
}

// TestACPSessionService_HandleSessionNew_AlwaysAdvertisesNonEmptyDescription verifies that every
// advertised command carries a non-empty description. The ACP AvailableCommand schema makes
// `description` a REQUIRED field; emitting a command without it (omitempty) makes strict clients
// (e.g. Zed's serde-based parser) reject the entire availableCommands array, so the slash-command
// suggestion menu shows nothing. A workflow with no description must fall back to a non-empty value.
func TestACPSessionService_HandleSessionNew_AlwaysAdvertisesNonEmptyDescription(t *testing.T) {
	ctx := context.Background()
	provider := new(MockWorkflowProvider)

	entries := []workflow.WorkflowEntry{{Name: "no-desc", Source: "local", Scope: "local", Workflow: "no-desc"}}
	provider.On("ListAllWorkflows", ctx).Return(entries, nil)
	// Workflow definition deliberately has an empty Description.
	wf := &workflow.Workflow{
		Name:    "no-desc",
		Initial: "start",
		Steps:   map[string]*workflow.Step{"start": {Name: "start", Type: workflow.StepTypeTerminal}},
	}
	provider.On("GetWorkflow", ctx, "no-desc").Return(wf, nil)

	emitter := &fakeEmitter{}
	svc := &ACPSessionService{logger: ports.NopLogger{}, emitter: emitter}
	svc.SetWorkflowProvider(provider)

	result, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr)

	commands, ok := resultMap(t, result)["commands"].([]WorkflowSlashCommand)
	require.True(t, ok)
	require.Len(t, commands, 1)
	assert.NotEmpty(t, commands[0].Description,
		"ACP requires a non-empty description per command; a description-less workflow must fall back to a non-empty value")

	// The serialized JSON must include the description field (no omitempty drop) so a strict
	// client that requires the field can deserialize the command.
	raw, err := json.Marshal(commands[0])
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"description"`,
		"the description field must always be serialized (ACP requires it)")
}

// TestACPSessionService_HandleSessionNew_StoresEditorMcpServers verifies editor-provided MCP
// servers are decoded (camelCase) and stored on the session.
func TestACPSessionService_HandleSessionNew_StoresEditorMcpServers(t *testing.T) {
	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{}, nil)

	svc := &ACPSessionService{workflowRepo: mockRepo, logger: ports.NopLogger{}}

	// env is the ACP wire array form ([{name,value}]) — matching the SDK's McpServerStdio
	// marshaller. The object form ({"K":"V"}) is NOT valid ACP and would fail to decode.
	params := json.RawMessage(`{
		"cwd": "/home/user",
		"mcpServers": [{"name": "editor-server", "command": "python", "args": ["-m", "srv"], "env": [{"name": "K", "value": "V"}]}]
	}`)
	result, acpErr := svc.HandleSessionNew(ctx, params)
	require.Nil(t, acpErr)

	sessionID, _ := resultMap(t, result)["sessionId"].(string)
	val, ok := svc.sessions.Load(sessionID)
	require.True(t, ok, "session must be stored under the minted sessionId")
	session := val.(*ACPSession)
	spec, ok := session.MCPServers["editor-server"]
	require.True(t, ok, "editor MCP server must be stored")
	assert.Equal(t, "python", spec.Command)
	assert.Equal(t, []string{"-m", "srv"}, spec.Args)
	assert.Equal(t, []MCPEnvVariable{{Name: "K", Value: "V"}}, spec.Env)
}

// TestACPSessionService_HandleSessionPrompt_DispatchesToRunner verifies the slash command and
// --input flags are parsed and dispatched to the workflow runner, returning stopReason=end_turn.
func TestACPSessionService_HandleSessionPrompt_DispatchesToRunner(t *testing.T) {
	exec := workflow.NewExecutionContext("workflow-1", "Test Workflow")
	exec.SetStepState("run", workflow.StepState{Output: "Hello World\n"})
	runner := &fakeRunner{execCtx: exec}
	emitter := &fakeEmitter{}
	svc := &ACPSessionService{logger: ports.NopLogger{}, runner: runner, emitter: emitter}
	svc.sessions.Store("sess-run", &ACPSession{ID: "sess-run"})

	params := json.RawMessage(`{
		"sessionId": "sess-run",
		"prompt": [{"type": "text", "text": "/workflow-1 --input=required_input=test_value --input=optional_input=custom"}]
	}`)
	result, acpErr := svc.HandleSessionPrompt(context.Background(), params)
	require.Nil(t, acpErr)

	assert.Equal(t, 1, runner.callCount(), "runner must be dispatched exactly once")
	assert.Equal(t, "workflow-1", runner.name)
	assert.Equal(t, map[string]any{"required_input": "test_value", "optional_input": "custom"}, runner.inputs)
	assert.Equal(t, "end_turn", stopReasonOf(t, result))
	assert.Contains(t, emitter.agentText(), "Hello World",
		"workflow output must be streamed back to the editor as an agent message")
}

// TestACPSessionService_HandleSessionPrompt_RejectsUnsupportedBlocks verifies image/audio/resource
// blocks end the turn with a USER.ACP.UNSUPPORTED_BLOCK stopReason (not a JSON-RPC error),
// while text blocks dispatch normally.
func TestACPSessionService_HandleSessionPrompt_RejectsUnsupportedBlocks(t *testing.T) {
	tests := []struct {
		name         string
		block        string
		wantDispatch bool
	}{
		{name: "text dispatches", block: `{"type":"text","text":"/workflow-1"}`, wantDispatch: true},
		{name: "image rejected", block: `{"type":"image"}`, wantDispatch: false},
		{name: "audio rejected", block: `{"type":"audio"}`, wantDispatch: false},
		{name: "resource rejected", block: `{"type":"resource"}`, wantDispatch: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{}
			emitter := &fakeEmitter{}
			svc := &ACPSessionService{logger: ports.NopLogger{}, runner: runner, emitter: emitter}
			svc.sessions.Store("sess-blk", &ACPSession{ID: "sess-blk"})

			params := json.RawMessage(`{"sessionId":"sess-blk","prompt":[` + tt.block + `]}`)
			result, acpErr := svc.HandleSessionPrompt(context.Background(), params)
			require.Nil(t, acpErr, "unsupported blocks must not be a JSON-RPC error")

			// The turn always ends with a valid ACP stop reason; the reason for a rejection
			// is conveyed to the user as an agent message, not in the stopReason.
			assert.Equal(t, "end_turn", stopReasonOf(t, result))
			if tt.wantDispatch {
				assert.Equal(t, 1, runner.callCount())
			} else {
				// m-2 fix: agent text must be human-readable, not contain the machine error code.
				// The message now reads "Unsupported content: ..." rather than prefixing the code.
				assert.Contains(t, emitter.agentText(), "Unsupported content",
					"rejected block must explain why via a human-readable agent message")
				assert.NotContains(t, emitter.agentText(), string(domainerrors.ErrorCodeUserACPUnsupportedBlock),
					"machine error code must not appear in the user-visible agent message (m-2 fix)")
				assert.Equal(t, 0, runner.callCount(), "rejected block must not dispatch a workflow")
			}
		})
	}
}

// TestACPSessionService_HandleSessionPrompt_RejectsConcurrentPrompts verifies a second prompt
// on a session with an in-flight turn returns USER.ACP.PROMPT_IN_FLIGHT.
// C-3: the machine code must be in Data; Message must be human-readable (not a raw code).
func TestACPSessionService_HandleSessionPrompt_RejectsConcurrentPrompts(t *testing.T) {
	svc := &ACPSessionService{logger: ports.NopLogger{}, runner: &fakeRunner{}}
	session := &ACPSession{ID: "sess-busy"}
	session.InFlight.Store(true)
	svc.sessions.Store("sess-busy", session)

	params := json.RawMessage(`{"sessionId":"sess-busy","prompt":[{"type":"text","text":"/workflow-1"}]}`)
	_, acpErr := svc.HandleSessionPrompt(context.Background(), params)

	require.NotNil(t, acpErr)
	assert.Equal(t, ACPErrInvalidParams, acpErr.Kind)
	// C-3: Data carries the machine-readable code; Message is human-readable.
	assert.Equal(t, string(domainerrors.ErrorCodeUserACPPromptInFlight), acpErr.Data,
		"error code must be in Data, not Message")
	assert.NotEqual(t, string(domainerrors.ErrorCodeUserACPPromptInFlight), acpErr.Message,
		"Message must be human-readable, not the raw error code")
	assert.NotEmpty(t, acpErr.Message, "Message must not be empty")
}

// TestACPSessionService_HandleSessionPrompt_MissingSlashCommand_ReturnsInvalidPrompt verifies a
// prompt without a leading /<workflow> ends the turn with USER.ACP.INVALID_PROMPT.
func TestACPSessionService_HandleSessionPrompt_MissingSlashCommand_ReturnsInvalidPrompt(t *testing.T) {
	runner := &fakeRunner{}
	emitter := &fakeEmitter{}
	svc := &ACPSessionService{logger: ports.NopLogger{}, runner: runner, emitter: emitter}
	svc.sessions.Store("sess-bad", &ACPSession{ID: "sess-bad"})

	params := json.RawMessage(`{"sessionId":"sess-bad","prompt":[{"type":"text","text":"just some text"}]}`)
	result, acpErr := svc.HandleSessionPrompt(context.Background(), params)
	require.Nil(t, acpErr)

	assert.Equal(t, "end_turn", stopReasonOf(t, result))
	// m-2 fix: agent text must be human-readable, not contain the machine error code.
	// The message now reads "Invalid prompt: ..." rather than prefixing the code.
	assert.Contains(t, emitter.agentText(), "Invalid prompt",
		"missing slash command must be explained to the user via a human-readable agent message")
	assert.NotContains(t, emitter.agentText(), string(domainerrors.ErrorCodeUserACPInvalidPrompt),
		"machine error code must not appear in the user-visible agent message (m-2 fix)")
	assert.Equal(t, 0, runner.callCount())
}

// TestACPSessionService_HandleSessionPrompt_UnknownSession returns USER.ACP.UNKNOWN_SESSION.
// C-3: the machine code must be in Data; Message must be human-readable (not a raw code).
func TestACPSessionService_HandleSessionPrompt_UnknownSession(t *testing.T) {
	svc := &ACPSessionService{logger: ports.NopLogger{}, runner: &fakeRunner{}}
	_, acpErr := svc.HandleSessionPrompt(context.Background(),
		json.RawMessage(`{"sessionId":"nope","prompt":[{"type":"text","text":"/x"}]}`))
	require.NotNil(t, acpErr)
	// C-3: Data carries the machine-readable code; Message is human-readable.
	assert.Equal(t, string(domainerrors.ErrorCodeUserACPUnknownSession), acpErr.Data,
		"error code must be in Data, not Message")
	assert.NotEqual(t, string(domainerrors.ErrorCodeUserACPUnknownSession), acpErr.Message,
		"Message must be human-readable, not the raw error code")
	assert.NotEmpty(t, acpErr.Message, "Message must not be empty")
}

// TestACPSessionService_HandleSessionCancel_InvokesCancel verifies session/cancel calls the
// recorded cancel function and reports stopReason=cancelled.
func TestACPSessionService_HandleSessionCancel_InvokesCancel(t *testing.T) {
	svc := &ACPSessionService{logger: ports.NopLogger{}}
	cancelled := make(chan struct{})
	session := &ACPSession{ID: "sess-cancel"}
	session.setCancel(func() { close(cancelled) })
	svc.sessions.Store("sess-cancel", session)

	result, acpErr := svc.HandleSessionCancel(context.Background(), json.RawMessage(`{"sessionId":"sess-cancel"}`))
	require.Nil(t, acpErr)
	assert.Equal(t, "cancelled", stopReasonOf(t, result))

	select {
	case <-cancelled:
	default:
		t.Fatal("session/cancel must invoke the recorded cancel function")
	}
}

// TestACPSessionService_HandleSessionCancel_KeepsSessionReusable is the regression test for the
// bug where session/cancel cancelled the session-lifetime context, so every later prompt started
// with an already-cancelled runCtx (a child of sessionCtx) and resolved instantly as "cancelled".
// Per ACP, session/cancel interrupts only the ongoing turn; the session must stay usable.
func TestACPSessionService_HandleSessionCancel_KeepsSessionReusable(t *testing.T) {
	exec := workflow.NewExecutionContext("trivial", "Trivial Workflow")
	exec.SetStepState("run", workflow.StepState{Output: "ok\n"})

	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{{Name: "trivial", Source: ports.SourceLocal, Path: "/p/trivial.yaml"}}, nil)
	mockRepo.On("Load", ctx, "trivial").Return(testWorkflow("trivial"), nil)

	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, runner: &fakeRunner{execCtx: exec}, emitter: &fakeEmitter{}}
	svc.SetServerContext(context.Background())

	newResult, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/home/user","mcpServers":[]}`))
	require.Nil(t, acpErr)
	sessionID, _ := resultMap(t, newResult)["sessionId"].(string)
	require.NotEmpty(t, sessionID)

	// Cancel the (idle) session — must NOT poison the session-lifetime context.
	_, acpErr = svc.HandleSessionCancel(ctx, json.RawMessage(fmt.Sprintf(`{"sessionId":%q}`, sessionID)))
	require.Nil(t, acpErr)

	val, ok := svc.sessions.Load(sessionID)
	require.True(t, ok)
	session := val.(*ACPSession)
	require.NoError(t, session.getSessionCtx().Err(),
		"session/cancel must not cancel the session-lifetime context; the session stays reusable")

	// A subsequent prompt must execute normally (end_turn). Under the bug, runCtx derived from the
	// cancelled sessionCtx would make run.cancelled true and resolve as stopReason=cancelled.
	promptParams, _ := json.Marshal(map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/trivial"}},
	})
	result, acpErr := svc.HandleSessionPrompt(ctx, promptParams)
	require.Nil(t, acpErr)
	assert.Equal(t, "end_turn", stopReasonOf(t, result),
		"a prompt after session/cancel must execute, not start on a cancelled context")
}

// TestACPSessionService_HandleSessionNew_AcceptsMCPServersWithEnv is the regression test for the
// MCPServerSpec.Env type mismatch: the ACP wire format (and the SDK's McpServerStdio marshaller)
// emit env as a JSON ARRAY of {name,value}. Decoding that into a map[string]string failed the whole
// session/new json.Unmarshal, rejecting every session that declared an MCP stdio server with env.
func TestACPSessionService_HandleSessionNew_AcceptsMCPServersWithEnv(t *testing.T) {
	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{}, nil)
	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo}

	// Exact wire shape the SDK's McpServerStdio marshaller produces: env is an array of {name,value}.
	params := json.RawMessage(`{"cwd":"/w","mcpServers":[{"name":"fs","command":"srv","args":["--x"],"env":[{"name":"TOKEN","value":"abc"},{"name":"DEBUG","value":"1"}]}]}`)
	result, acpErr := svc.HandleSessionNew(ctx, params)
	require.Nil(t, acpErr, "session/new must accept mcpServers whose env is the wire array form")

	sessionID, _ := resultMap(t, result)["sessionId"].(string)
	require.NotEmpty(t, sessionID)
	val, ok := svc.sessions.Load(sessionID)
	require.True(t, ok)
	session := val.(*ACPSession)
	require.Len(t, session.MCPServers, 1)
	assert.Equal(t, []MCPEnvVariable{{Name: "TOKEN", Value: "abc"}, {Name: "DEBUG", Value: "1"}}, session.MCPServers["fs"].Env,
		"env vars must survive decoding, not be silently dropped")
}

// TestACPSessionService_HandleSessionPrompt_FactoryBuildsPerSessionRunnerAndSendsAggregateWhenNothingStreamed
// verifies that when a runnerFactory is set, each session builds its own runner (exactly once)
// and that the aggregate text IS sent when nothing was streamed live (streamed flag stays false
// because the fakeRunner never writes through output writers).
func TestACPSessionService_HandleSessionPrompt_FactoryBuildsPerSessionRunnerAndSendsAggregateWhenNothingStreamed(t *testing.T) {
	// Build an ExecutionContext with a step that has non-empty output — mirrors DispatchesToRunner.
	exec := workflow.NewExecutionContext("trivial", "Trivial Workflow")
	exec.SetStepState("run", workflow.StepState{Output: "live output\n"})

	var factoryCalls int
	factory := func(sessionID string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		factoryCalls++
		// The fakeRunner does not write through output writers, so the returned streamed
		// flag stays false — the service must fall back to sending the aggregate.
		return &fakeRunner{execCtx: exec}, &fakeInputResponder{}, &atomic.Bool{}, func() {}, nil
	}

	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{{Name: "trivial", Source: ports.SourceLocal, Path: "/p/trivial.yaml"}}, nil)
	mockRepo.On("Load", ctx, "trivial").Return(testWorkflow("trivial"), nil)

	emitter := &fakeEmitter{}
	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, emitter: emitter}
	svc.SetRunnerFactory(factory)

	// Establish a session via HandleSessionNew.
	newParams := json.RawMessage(`{"cwd":"/home/user","mcpServers":[]}`)
	newResult, acpErr := svc.HandleSessionNew(ctx, newParams)
	require.Nil(t, acpErr)
	sessionID, _ := resultMap(t, newResult)["sessionId"].(string)
	require.NotEmpty(t, sessionID)

	// Dispatch a prompt naming the "trivial" workflow.
	promptParams, _ := json.Marshal(map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/trivial"}},
	})
	result, acpErr := svc.HandleSessionPrompt(ctx, promptParams)
	require.Nil(t, acpErr)
	assert.Equal(t, "end_turn", stopReasonOf(t, result))

	// Factory must have been called exactly once (lazy construction on first prompt).
	assert.Equal(t, 1, factoryCalls, "factory must be invoked exactly once per session")

	// streamed=false → aggregate is sent so the editor sees the workflow output.
	assert.NotEmpty(t, emitter.agentText(),
		"aggregate output must be sent when nothing was streamed live (streamed flag is false)")
	assert.Contains(t, emitter.agentText(), "live output",
		"aggregate must contain the workflow's step output")
}

// TestACPSessionService_ConversationParking_RoutesToInputReader verifies that once a workflow is
// parked (ParkedTurnCount > 0), subsequent prompts are routed to the InputReader rather than
// starting a new workflow run.
func TestACPSessionService_ConversationParking_RoutesToInputReader(t *testing.T) {
	runner := &fakeRunner{}
	reader := &fakeInputResponder{}
	session := &ACPSession{ID: "sess-park"}
	// Wire the reader via the atomic.Pointer[inputReaderHolder] accessor. The holder
	// wrapper avoids the pointer-on-interface anti-pattern: the concrete struct gives
	// atomic.Pointer a stable pointer rather than an interface-value address (C-2 fix).
	session.inputReader.Store(&inputReaderHolder{r: reader})
	session.ParkedTurnCount.Store(1)
	// A parked turn always has a published run (created on first dispatch, before the park
	// hook can fire). Inject one whose parkedCh already carries a token to model the workflow
	// re-parking after it consumes the continuation input — so waitTurn ends the turn with
	// end_turn without starting a new run.
	run := &acpRun{parkedCh: make(chan struct{}, 1), doneCh: make(chan struct{}), workflowName: "trivial"}
	run.parkedCh <- struct{}{}
	session.run.Store(run)

	svc := &ACPSessionService{logger: ports.NopLogger{}, runner: runner}
	svc.sessions.Store("sess-park", session)

	params := json.RawMessage(`{"sessionId":"sess-park","prompt":[{"type":"text","text":"continue please"}]}`)
	result, acpErr := svc.HandleSessionPrompt(context.Background(), params)
	require.Nil(t, acpErr)

	assert.Equal(t, "end_turn", stopReasonOf(t, result))
	assert.Equal(t, []string{"continue please"}, reader.recorded(), "continuation turn must be routed to the InputReader")
	assert.Equal(t, 0, runner.callCount(), "a parked session must not start a new workflow run")
}

// TestACPSessionService_ParkHooksRouteContinuationToInputReader is the CRITIQUE-3 production
// seam test: when a factory-built runner's park hooks are wired (which ensureRunner does),
// firing OnPark bumps the session's ParkedTurnCount so that a second prompt routes to
// InputReader.Respond and returns end_turn — instead of falling into parseSlashCommand and
// returning ErrorCodeUserACPInvalidPrompt. Pre-fix the hooks were never wired, so the branch was
// dead and the second prompt failed parsing.
func TestACPSessionService_ParkHooksRouteContinuationToInputReader(t *testing.T) {
	exec := workflow.NewExecutionContext("trivial", "Trivial Workflow")
	exec.SetStepState("run", workflow.StepState{Output: "done\n"})
	reader := &fakeInputResponder{}

	factory := func(string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		return &fakeRunner{execCtx: exec}, reader, &atomic.Bool{}, func() {}, nil
	}

	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{{Name: "trivial", Source: ports.SourceLocal, Path: "/p/trivial.yaml"}}, nil)
	mockRepo.On("Load", ctx, "trivial").Return(testWorkflow("trivial"), nil)

	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, emitter: &fakeEmitter{}}
	svc.SetRunnerFactory(factory)

	newResult, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr)
	sessionID, _ := resultMap(t, newResult)["sessionId"].(string)
	require.NotEmpty(t, sessionID)

	// First prompt builds the runner (and wires the park hooks).
	firstParams, _ := json.Marshal(map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/trivial"}},
	})
	_, acpErr = svc.HandleSessionPrompt(ctx, firstParams)
	require.Nil(t, acpErr)

	// The reader's park hooks must have been wired by ensureRunner (CRITIQUE-3 seam).
	onPark, onUnpark := reader.parkHooks()
	require.NotNil(t, onPark, "ensureRunner must wire the reader's OnPark hook")
	require.NotNil(t, onUnpark, "ensureRunner must wire the reader's OnUnpark hook")

	val, ok := svc.sessions.Load(sessionID)
	require.True(t, ok)
	session := val.(*ACPSession)

	// Simulate a workflow goroutine parking on the reader: OnPark bumps the counter.
	onPark()
	require.Equal(t, int32(1), session.ParkedTurnCount.Load(),
		"OnPark must increment the session's ParkedTurnCount")

	// Second prompt: because ParkedTurnCount > 0, it must route to Respond and end_turn,
	// NOT re-parse as a slash command.
	secondParams := json.RawMessage(`{"sessionId":"` + sessionID + `","prompt":[{"type":"text","text":"continue now"}]}`)
	result, acpErr := svc.HandleSessionPrompt(ctx, secondParams)
	require.Nil(t, acpErr)
	assert.Equal(t, "end_turn", stopReasonOf(t, result))
	assert.Equal(t, []string{"continue now"}, reader.recorded(),
		"continuation prompt must be routed to InputReader.Respond")

	// OnUnpark releases the parked turn (balanced accounting).
	onUnpark()
	assert.Equal(t, int32(0), session.ParkedTurnCount.Load(),
		"OnUnpark must decrement back to zero (one OnUnpark per OnPark)")
}

// TestACPSessionService_EnsureRunner_RetriesAfterFactoryFailure is the MAJEUR-4 test: a
// session whose first factory call fails must NOT be permanently bricked — the next prompt
// retries the factory and, on success, dispatches normally.
func TestACPSessionService_EnsureRunner_RetriesAfterFactoryFailure(t *testing.T) {
	exec := workflow.NewExecutionContext("trivial", "Trivial Workflow")
	exec.SetStepState("run", workflow.StepState{Output: "recovered\n"})

	var calls int
	factory := func(string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		calls++
		if calls == 1 {
			return nil, nil, nil, nil, errors.New("transient factory failure")
		}
		return &fakeRunner{execCtx: exec}, &fakeInputResponder{}, &atomic.Bool{}, func() {}, nil
	}

	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{{Name: "trivial", Source: ports.SourceLocal, Path: "/p/trivial.yaml"}}, nil)
	mockRepo.On("Load", ctx, "trivial").Return(testWorkflow("trivial"), nil)

	emitter := &fakeEmitter{}
	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, emitter: emitter}
	svc.SetRunnerFactory(factory)

	newResult, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr)
	sessionID, _ := resultMap(t, newResult)["sessionId"].(string)

	promptParams, _ := json.Marshal(map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/trivial"}},
	})

	// First prompt: factory fails → structured internal error, session NOT bricked.
	_, acpErr = svc.HandleSessionPrompt(ctx, promptParams)
	require.NotNil(t, acpErr, "first prompt must surface the factory failure")
	assert.Equal(t, ACPErrInternal, acpErr.Kind)

	// Second prompt: factory is retried and succeeds → dispatch end_turn.
	result, acpErr := svc.HandleSessionPrompt(ctx, promptParams)
	require.Nil(t, acpErr, "second prompt must retry the factory and succeed")
	assert.Equal(t, "end_turn", stopReasonOf(t, result))
	assert.Equal(t, 2, calls, "factory must be retried after the first failure")
	assert.Contains(t, emitter.agentText(), "recovered")
}

// TestACPSessionService_Shutdown_WaitsForInFlightWorkflowBeforeCleanup is the CRITIQUE-1
// race test: Shutdown must cancel the in-flight run, wait for it to return, and only then
// invoke the per-session cleanup. A cleanup running before the workflow finishes would
// release resources still in use.
func TestACPSessionService_Shutdown_WaitsForInFlightWorkflowBeforeCleanup(t *testing.T) {
	runStarted := make(chan struct{})
	cleanupCalled := make(chan struct{})
	var workflowDone atomic.Bool
	var cleanupAfterDone atomic.Bool

	// A runner that blocks until its context is cancelled, then marks workflowDone.
	blockingRunner := &blockingRunner{started: runStarted, done: &workflowDone}

	factory := func(string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		cleanup := func() {
			// Record whether the workflow had already finished when cleanup ran.
			cleanupAfterDone.Store(workflowDone.Load())
			close(cleanupCalled)
		}
		return blockingRunner, &fakeInputResponder{}, &atomic.Bool{}, cleanup, nil
	}

	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{{Name: "trivial", Source: ports.SourceLocal, Path: "/p/trivial.yaml"}}, nil)
	mockRepo.On("Load", ctx, "trivial").Return(testWorkflow("trivial"), nil)

	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, emitter: &fakeEmitter{}}
	svc.SetRunnerFactory(factory)

	newResult, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr)
	sessionID, _ := resultMap(t, newResult)["sessionId"].(string)

	promptParams, _ := json.Marshal(map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/trivial"}},
	})

	// Dispatch the prompt on its own goroutine (it blocks until Shutdown cancels it).
	promptReturned := make(chan struct{})
	go func() {
		_, _ = svc.HandleSessionPrompt(ctx, promptParams)
		close(promptReturned)
	}()

	// Wait until the workflow run is actually in flight before shutting down.
	<-runStarted

	// Shutdown must cancel, wait for the workflow to finish, then run cleanup.
	svc.Shutdown()

	<-cleanupCalled
	<-promptReturned

	assert.True(t, cleanupAfterDone.Load(),
		"cleanup must run only after the in-flight workflow has finished")
}

// blockingRunner blocks in Run until its context is cancelled, then records completion. It
// lets the Shutdown ordering test observe whether cleanup raced ahead of the workflow.
type blockingRunner struct {
	started   chan struct{}
	startOnce sync.Once
	done      *atomic.Bool
}

func (b *blockingRunner) Run(ctx context.Context, _ string, _ map[string]any) (*workflow.ExecutionContext, error) {
	b.startOnce.Do(func() { close(b.started) })
	<-ctx.Done()
	// Simulate the tail of a workflow still touching session resources after cancel.
	b.done.Store(true)
	return nil, ctx.Err()
}

// TestParseSlashCommand_Issue11_ErrorMessageNamesComponent verifies that parseSlashCommand
// error messages name the specific failing component (pack vs workflow) rather than showing
// only the full "pack/workflow" string. This lets the editor give precise feedback.
func TestParseSlashCommand_Issue11_ErrorMessageNamesComponent(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantErrContains string // substring that must appear in the error message
	}{
		{
			name:            "plain invalid workflow name",
			input:           "/Bad_Name",
			wantErrContains: "workflow",
		},
		{
			name:            "pack component invalid — error names pack",
			input:           "/Bad_Pack/good-workflow",
			wantErrContains: "pack",
		},
		{
			name:            "workflow component invalid — error names workflow",
			input:           "/good-pack/Bad_Workflow",
			wantErrContains: "workflow",
		},
		{
			name:            "both components invalid — first (pack) is reported",
			input:           "/Bad_Pack/Bad_Workflow",
			wantErrContains: "pack",
		},
		{
			name:            "plain invalid name error includes the bad name",
			input:           "/Bad_Name",
			wantErrContains: "Bad_Name",
		},
		{
			name:            "pack error includes the bad pack name",
			input:           "/Bad_Pack/good-workflow",
			wantErrContains: "Bad_Pack",
		},
		{
			name:            "workflow error includes the bad workflow name",
			input:           "/good-pack/Bad_Workflow",
			wantErrContains: "Bad_Workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseSlashCommand(tt.input)
			require.Error(t, err, "invalid slash command must produce an error")
			assert.Contains(t, err.Error(), tt.wantErrContains,
				"issue #11: error message must name the failing component")
		})
	}
}

// TestSendAgentText_HumanReadableMessage_NoMachineCodePrefix is the m-2 non-regression test:
// the text delivered to the editor via sendAgentText must NOT contain machine error-code
// prefixes like "USER.ACP.*". Both the unsupported-content path and the invalid-prompt path
// are covered. The machine codes must remain absent from the visible message.
func TestSendAgentText_HumanReadableMessage_NoMachineCodePrefix(t *testing.T) {
	tests := []struct {
		name           string
		promptJSON     string
		wantContains   string // expected human-readable substring
		wantNoContains string // machine code that must NOT appear
	}{
		{
			name:           "unsupported image block — human message, no machine code",
			promptJSON:     `{"sessionId":"sess-m2","prompt":[{"type":"image"}]}`,
			wantContains:   "Unsupported content",
			wantNoContains: "USER.ACP.UNSUPPORTED_BLOCK",
		},
		{
			name:           "missing slash command — human message, no machine code",
			promptJSON:     `{"sessionId":"sess-m2","prompt":[{"type":"text","text":"just prose"}]}`,
			wantContains:   "Invalid prompt",
			wantNoContains: "USER.ACP.INVALID_PROMPT",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &fakeEmitter{}
			svc := &ACPSessionService{logger: ports.NopLogger{}, runner: &fakeRunner{}, emitter: emitter}
			svc.sessions.Store("sess-m2", &ACPSession{ID: "sess-m2"})

			result, acpErr := svc.HandleSessionPrompt(context.Background(), json.RawMessage(tt.promptJSON))
			require.Nil(t, acpErr, "unsupported/invalid prompt must not be a JSON-RPC error")
			assert.Equal(t, "end_turn", stopReasonOf(t, result))

			txt := emitter.agentText()
			assert.Contains(t, txt, tt.wantContains,
				"agent message must contain a human-readable explanation")
			assert.NotContains(t, txt, tt.wantNoContains,
				"machine error code must not appear in the user-visible agent message (m-2 fix)")
		})
	}
}

// TestParseSlashCommand_PromptTooLarge is the m-4 non-regression test: a prompt that exceeds
// MaxPromptBytes must be rejected before tokenization, returning an error that mentions both
// the actual size and the limit. This prevents unbounded memory allocation in tokenizePrompt.
func TestParseSlashCommand_PromptTooLarge(t *testing.T) {
	// One byte over the 1 MiB limit.
	oversized := "/" + strings.Repeat("a", MaxPromptBytes)
	_, _, err := parseSlashCommand(oversized)
	require.Error(t, err, "prompt exceeding MaxPromptBytes must be rejected")
	assert.Contains(t, err.Error(), "prompt too large",
		"error must clearly state the prompt is too large")
	assert.Contains(t, err.Error(), fmt.Sprintf("%d", MaxPromptBytes),
		"error must include the max allowed size")
}

// TestParseSlashCommand_PromptAtLimit verifies that a prompt of exactly MaxPromptBytes is
// accepted (boundary: limit is exclusive, i.e. len > max triggers the guard).
func TestParseSlashCommand_PromptAtLimit(t *testing.T) {
	// Exactly MaxPromptBytes — should NOT trigger the guard.
	// Build "/A" + padding to reach exactly MaxPromptBytes bytes.
	// Uppercase "A" is rejected by ValidateName (^[a-z][a-z0-9-]*$), so parseSlashCommand
	// must return an error from name validation — not from the size guard.
	padding := strings.Repeat("x", MaxPromptBytes-len("/A"))
	atLimit := "/A" + padding
	require.Equal(t, MaxPromptBytes, len(atLimit), "test setup: prompt must be exactly MaxPromptBytes")
	// parseSlashCommand must fail with a name-validation error, not "prompt too large".
	_, _, err := parseSlashCommand(atLimit)
	require.Error(t, err, "name validation must reject the uppercase name")
	assert.NotContains(t, err.Error(), "prompt too large",
		"a prompt of exactly MaxPromptBytes must not be rejected by the size guard")
}

// TestParseSlashCommand_PackNamespaceColonMapsToSlash verifies that a pack slash command using
// the ':' namespace separator advertised over ACP is mapped back to the internal "pack/workflow"
// form for dispatch. The legacy '/' form is still accepted so a hand-typed "/pack/workflow" works.
func TestParseSlashCommand_PackNamespaceColonMapsToSlash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{name: "colon separator maps to slash", input: "/speckit:specify", wantName: "speckit/specify"},
		{name: "slash form still accepted", input: "/speckit/specify", wantName: "speckit/specify"},
		{name: "plain local name unchanged", input: "/commit", wantName: "commit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := parseSlashCommand(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, got, "advertised ':' separator must resolve to the internal '/' workflow name")
		})
	}
}

// TestParseSlashCommand_ValidNames verifies valid single and compound workflow names are accepted.
func TestParseSlashCommand_ValidNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{name: "plain workflow name", input: "/my-workflow", wantName: "my-workflow"},
		{name: "pack/workflow name", input: "/my-pack/my-workflow", wantName: "my-pack/my-workflow"},
		{name: "name with digits", input: "/wf-123", wantName: "wf-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, _, err := parseSlashCommand(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, gotName)
		})
	}
}
