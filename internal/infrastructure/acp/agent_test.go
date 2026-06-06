package acp

import (
	"context"
	"encoding/json"
	"testing"

	sdk "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
)

// fakeACPSessionService is a configurable test double for the sessionService interface.
// Func fields allow per-call behavior; Calls record all invocations.
type fakeACPSessionService struct {
	HandleSessionNewFunc    func(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError)
	HandleSessionPromptFunc func(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError)
	HandleSessionCancelFunc func(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError)

	Calls struct {
		HandleSessionNew    []json.RawMessage
		HandleSessionPrompt []json.RawMessage
		HandleSessionCancel []json.RawMessage
	}
}

func (f *fakeACPSessionService) HandleSessionNew(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError) {
	f.Calls.HandleSessionNew = append(f.Calls.HandleSessionNew, params)
	if f.HandleSessionNewFunc != nil {
		return f.HandleSessionNewFunc(ctx, params)
	}
	return nil, nil
}

func (f *fakeACPSessionService) HandleSessionPrompt(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError) {
	f.Calls.HandleSessionPrompt = append(f.Calls.HandleSessionPrompt, params)
	if f.HandleSessionPromptFunc != nil {
		return f.HandleSessionPromptFunc(ctx, params)
	}
	return nil, nil
}

func (f *fakeACPSessionService) HandleSessionCancel(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError) {
	f.Calls.HandleSessionCancel = append(f.Calls.HandleSessionCancel, params)
	if f.HandleSessionCancelFunc != nil {
		return f.HandleSessionCancelFunc(ctx, params)
	}
	return nil, nil
}

// assertRequestErrorCode asserts that err is (or wraps) an SDK *RequestError
// carrying wantCode. This is the contract agent.go must honor: every handler
// failure is translated through the errors.go helpers into a typed SDK error so
// the transport emits the correct JSON-RPC code (-32602/-32601/-32603).
func assertRequestErrorCode(t *testing.T, err error, wantCode int) {
	t.Helper()
	var reqErr *sdk.RequestError
	require.ErrorAs(t, err, &reqErr)
	assert.Equal(t, wantCode, reqErr.Code)
}

// TestAgent_Initialize verifies Initialize responses and error handling.
func TestAgent_Initialize(t *testing.T) {
	tests := []struct {
		name      string
		svc       *application.ACPSessionService
		wantErr   bool
		checkResp func(t *testing.T, resp sdk.InitializeResponse)
	}{
		{
			name: "success with valid service",
			svc:  application.NewACPSessionService(nil, nil, nil, nil),
			checkResp: func(t *testing.T, resp sdk.InitializeResponse) {
				assert.NotEmpty(t, resp.ProtocolVersion)
				assert.NotNil(t, resp.AgentCapabilities)
				assert.Empty(t, resp.AuthMethods)
			},
		},
		{
			name:    "service nil returns internal error",
			svc:     nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := NewAgent(tt.svc)
			ctx := context.Background()

			resp, err := agent.Initialize(ctx, sdk.InitializeRequest{})

			if tt.wantErr {
				assert.Error(t, err)
				assertRequestErrorCode(t, err, -32603)
			} else {
				assert.NoError(t, err)
				if tt.checkResp != nil {
					tt.checkResp(t, resp)
				}
			}
		})
	}
}

// TestAgent_NewSession verifies NewSession validation, request marshaling, and error handling.
func TestAgent_NewSession(t *testing.T) {
	tests := []struct {
		name       string
		cwd        string
		mcpServers []sdk.McpServer
		svcResult  any
		svcErr     *application.ACPHandlerError
		wantErr    bool
		wantCode   int
	}{
		{
			name:       "success with cwd and mcp servers",
			cwd:        "/home/user",
			mcpServers: []sdk.McpServer{},
			svcResult:  map[string]any{"sessionId": "sess_123"},
			wantErr:    false,
		},
		{
			name:       "success with cwd only",
			cwd:        "/home/user",
			mcpServers: nil,
			svcResult:  map[string]any{"sessionId": "sess_456"},
			wantErr:    false,
		},
		{
			name:       "empty cwd rejected with invalidParamsErr",
			cwd:        "",
			mcpServers: nil,
			wantErr:    true,
			wantCode:   -32602,
		},
		{
			name:       "service returns internal error",
			cwd:        "/home/user",
			mcpServers: nil,
			svcErr:     &application.ACPHandlerError{Kind: application.ACPErrInternal, Message: "workflow runner not configured"},
			wantErr:    true,
			wantCode:   -32603,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeACPSessionService{
				HandleSessionNewFunc: func(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError) {
					return tt.svcResult, tt.svcErr
				},
			}
			agent := &Agent{svc: fake}
			ctx := context.Background()
			req := sdk.NewSessionRequest{
				Cwd:        tt.cwd,
				McpServers: tt.mcpServers,
			}

			resp, err := agent.NewSession(ctx, req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantCode != 0 {
					assertRequestErrorCode(t, err, tt.wantCode)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}

			if !tt.wantErr {
				require.Len(t, fake.Calls.HandleSessionNew, 1)
				var params map[string]any
				err := json.Unmarshal(fake.Calls.HandleSessionNew[0], &params)
				require.NoError(t, err)
				assert.Equal(t, tt.cwd, params["cwd"])
			}
		})
	}
}

// TestAgent_Prompt verifies Prompt request marshaling, payload validation, and error handling.
func TestAgent_Prompt(t *testing.T) {
	tests := []struct {
		name           string
		sessionID      string
		promptText     string
		svcResult      any
		svcErr         *application.ACPHandlerError
		wantErr        bool
		wantCode       int
		wantStopReason string
	}{
		{
			name:           "success with valid prompt extracts stop reason from PromptResult",
			sessionID:      "sess_123",
			promptText:     "test prompt",
			svcResult:      application.PromptResult{StopReason: "end_turn"},
			wantErr:        false,
			wantStopReason: "end_turn",
		},
		{
			name:       "service returns invalid params error",
			sessionID:  "sess_123",
			promptText: "test prompt",
			svcErr:     &application.ACPHandlerError{Kind: application.ACPErrInvalidParams, Message: "invalid session"},
			wantErr:    true,
			wantCode:   -32602,
		},
		{
			name:       "service returns internal error",
			sessionID:  "sess_123",
			promptText: "test prompt",
			svcErr:     &application.ACPHandlerError{Kind: application.ACPErrInternal, Message: "server error"},
			wantErr:    true,
			wantCode:   -32603,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeACPSessionService{
				HandleSessionPromptFunc: func(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError) {
					return tt.svcResult, tt.svcErr
				},
			}
			agent := &Agent{svc: fake}
			ctx := context.Background()
			req := sdk.PromptRequest{
				SessionId: sdk.SessionId(tt.sessionID),
				Prompt: []sdk.ContentBlock{
					sdk.TextBlock(tt.promptText),
				},
			}

			resp, err := agent.Prompt(ctx, req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantCode != 0 {
					assertRequestErrorCode(t, err, tt.wantCode)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, sdk.StopReason(tt.wantStopReason), resp.StopReason)
			}

			if !tt.wantErr {
				require.Len(t, fake.Calls.HandleSessionPrompt, 1)
			}
		})
	}
}

// TestAgent_PromptRejectsOversizePayload verifies that prompts exceeding 1 MiB are rejected.
func TestAgent_PromptRejectsOversizePayload(t *testing.T) {
	oversize := make([]byte, (1<<20)+1)
	for i := range oversize {
		oversize[i] = 'a'
	}
	prompt := string(oversize)

	fake := &fakeACPSessionService{}
	agent := &Agent{svc: fake}
	ctx := context.Background()
	req := sdk.PromptRequest{
		SessionId: sdk.SessionId("sess_123"),
		Prompt: []sdk.ContentBlock{
			sdk.TextBlock(prompt),
		},
	}

	resp, err := agent.Prompt(ctx, req)

	assert.Error(t, err)
	assertRequestErrorCode(t, err, -32602)
	assert.Empty(t, resp.StopReason)
	assert.Len(t, fake.Calls.HandleSessionPrompt, 0)
}

// TestAgent_Cancel verifies Cancel request marshaling and error handling.
func TestAgent_Cancel(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		svcErr    *application.ACPHandlerError
		wantErr   bool
		wantCode  int
	}{
		{
			name:      "success with valid session id",
			sessionID: "sess_123",
			wantErr:   false,
		},
		{
			name:      "service returns invalid params error",
			sessionID: "sess_123",
			svcErr:    &application.ACPHandlerError{Kind: application.ACPErrInvalidParams, Message: "unknown session"},
			wantErr:   true,
			wantCode:  -32602,
		},
		{
			name:     "service returns internal error",
			svcErr:   &application.ACPHandlerError{Kind: application.ACPErrInternal, Message: "server error"},
			wantErr:  true,
			wantCode: -32603,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeACPSessionService{
				HandleSessionCancelFunc: func(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError) {
					return nil, tt.svcErr
				},
			}
			agent := &Agent{svc: fake}
			ctx := context.Background()
			notif := sdk.CancelNotification{SessionId: sdk.SessionId(tt.sessionID)}

			err := agent.Cancel(ctx, notif)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantCode != 0 {
					assertRequestErrorCode(t, err, tt.wantCode)
				}
			} else {
				assert.NoError(t, err)
			}

			require.Len(t, fake.Calls.HandleSessionCancel, 1)
		})
	}
}

// TestAgent_UnsupportedMethods verifies that all 7 unsupported methods return MethodNotFound errors.
func TestAgent_UnsupportedMethods(t *testing.T) {
	tests := []struct {
		name     string
		methodFn func(*Agent) error
		wantCode int
	}{
		{
			name: "Authenticate returns methodNotFoundErr",
			methodFn: func(a *Agent) error {
				_, err := a.Authenticate(context.Background(), sdk.AuthenticateRequest{})
				return err
			},
			wantCode: -32601,
		},
		{
			name: "CloseSession returns methodNotFoundErr",
			methodFn: func(a *Agent) error {
				_, err := a.CloseSession(context.Background(), sdk.CloseSessionRequest{})
				return err
			},
			wantCode: -32601,
		},
		{
			name: "ListSessions returns methodNotFoundErr",
			methodFn: func(a *Agent) error {
				_, err := a.ListSessions(context.Background(), sdk.ListSessionsRequest{})
				return err
			},
			wantCode: -32601,
		},
		{
			name: "ResumeSession returns methodNotFoundErr",
			methodFn: func(a *Agent) error {
				_, err := a.ResumeSession(context.Background(), sdk.ResumeSessionRequest{})
				return err
			},
			wantCode: -32601,
		},
		{
			name: "SetSessionConfigOption returns methodNotFoundErr",
			methodFn: func(a *Agent) error {
				_, err := a.SetSessionConfigOption(context.Background(), sdk.SetSessionConfigOptionRequest{})
				return err
			},
			wantCode: -32601,
		},
		{
			name: "SetSessionMode returns methodNotFoundErr",
			methodFn: func(a *Agent) error {
				_, err := a.SetSessionMode(context.Background(), sdk.SetSessionModeRequest{})
				return err
			},
			wantCode: -32601,
		},
	}

	agent := NewAgent(application.NewACPSessionService(nil, nil, nil, nil))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.methodFn(agent)

			assert.Error(t, err)
			assertRequestErrorCode(t, err, tt.wantCode)
		})
	}
}

// TestAgent_PanicRecoveryDoesNotRepanic verifies that panic recovery via defer works
// and subsequent calls do not re-panic or leave the agent in a poisoned state.
func TestAgent_PanicRecoveryDoesNotRepanic(t *testing.T) {
	callCount := 0

	fake := &fakeACPSessionService{
		HandleSessionPromptFunc: func(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError) {
			callCount++
			if callCount == 1 {
				panic("test panic in handler")
			}
			return application.PromptResult{StopReason: "end_turn"}, nil
		},
	}

	agent := &Agent{svc: fake}
	ctx := context.Background()
	req := sdk.PromptRequest{
		SessionId: sdk.SessionId("sess_123"),
		Prompt: []sdk.ContentBlock{
			sdk.TextBlock("test"),
		},
	}

	resp1, err1 := agent.Prompt(ctx, req)
	assert.Error(t, err1, "first call should recover from panic and return error")
	// The recovered panic is translated to a typed SDK internal error (-32603).
	// The panic detail is intentionally NOT surfaced to the client (no internal
	// state leak); it is conveyed only as the generic JSON-RPC internal error.
	assertRequestErrorCode(t, err1, -32603)
	assert.Empty(t, resp1.StopReason)

	resp2, err2 := agent.Prompt(ctx, req)
	assert.NoError(t, err2, "second call must succeed without re-panicking")
	assert.NotNil(t, resp2)
	assert.Equal(t, 2, callCount, "handler should be called twice")
}
