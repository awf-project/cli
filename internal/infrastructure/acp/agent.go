package acp

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/coder/acp-go-sdk"

	"github.com/awf-project/cli/internal/application"
)

var _ sdk.Agent = (*Agent)(nil)

// sessionService is the subset of *application.ACPSessionService consumed by Agent.
// Declaring it as an interface keeps the agent unit-testable with a fake.
type sessionService interface {
	HandleSessionNew(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError)
	HandleSessionPrompt(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError)
	HandleSessionCancel(ctx context.Context, params json.RawMessage) (any, *application.ACPHandlerError)
}

// Agent implements sdk.Agent delegating to the application-layer ACPSessionService.
type Agent struct {
	svc sessionService
}

// NewAgent constructs an Agent backed by svc. The live SDK connection is owned and
// wired by the interfaces/cli ACP server, not by the agent itself.
func NewAgent(svc *application.ACPSessionService) *Agent {
	// Explicit nil check prevents a typed nil pointer from masking as a non-nil interface.
	if svc == nil {
		return &Agent{}
	}
	return &Agent{svc: svc}
}

// Initialize responds to ACP initialize handshakes.
func (a *Agent) Initialize(_ context.Context, _ sdk.InitializeRequest) (resp sdk.InitializeResponse, err error) { //nolint:gocritic // hugeParam: signature fixed by sdk.Agent interface
	defer func() {
		if r := recover(); r != nil {
			err = internalErr(fmt.Sprintf("panic recovered: %v", r))
		}
	}()
	if a.svc == nil {
		return sdk.InitializeResponse{}, internalErr("session service not configured")
	}
	return sdk.InitializeResponse{
		ProtocolVersion: sdk.ProtocolVersionNumber,
	}, nil
}

// NewSession creates a new ACP session via the application service.
func (a *Agent) NewSession(ctx context.Context, req sdk.NewSessionRequest) (resp sdk.NewSessionResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = internalErr(fmt.Sprintf("panic recovered: %v", r))
		}
	}()
	if req.Cwd == "" {
		return sdk.NewSessionResponse{}, invalidParamsErr("cwd is required")
	}
	params, jerr := json.Marshal(req)
	if jerr != nil {
		return sdk.NewSessionResponse{}, internalErr(jerr.Error())
	}
	result, svcErr := a.svc.HandleSessionNew(ctx, params)
	if svcErr != nil {
		return sdk.NewSessionResponse{}, toACPError(svcErr)
	}
	// HandleSessionNew returns a map carrying the minted session id. A missing or empty
	// id is a contract violation (the editor would receive an empty SessionId and bind
	// every subsequent request to ""), so surface it as an internal error instead of
	// silently returning a blank session.
	m, ok := result.(map[string]any)
	if !ok {
		return sdk.NewSessionResponse{}, internalErr(fmt.Sprintf("session/new returned unexpected result type %T", result))
	}
	id, ok := m["sessionId"].(string)
	if !ok || id == "" {
		return sdk.NewSessionResponse{}, internalErr("session/new returned a missing or empty session id")
	}
	return sdk.NewSessionResponse{SessionId: sdk.SessionId(id)}, nil
}

// Prompt dispatches a user turn to the application service.
func (a *Agent) Prompt(ctx context.Context, req sdk.PromptRequest) (resp sdk.PromptResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = internalErr(fmt.Sprintf("panic recovered: %v", r))
		}
	}()
	var promptBytes int
	for _, block := range req.Prompt {
		if block.Text != nil {
			promptBytes += len(block.Text.Text)
		}
	}
	if promptBytes > application.MaxPromptBytes {
		return sdk.PromptResponse{}, invalidParamsErr(fmt.Sprintf("prompt body exceeds %d bytes", application.MaxPromptBytes))
	}
	params, jerr := json.Marshal(req)
	if jerr != nil {
		return sdk.PromptResponse{}, internalErr(jerr.Error())
	}
	result, svcErr := a.svc.HandleSessionPrompt(ctx, params)
	if svcErr != nil {
		return sdk.PromptResponse{}, toACPError(svcErr)
	}
	var stopReason string
	if pr, ok := result.(application.PromptResult); ok {
		stopReason = pr.StopReason
	}
	return sdk.PromptResponse{StopReason: sdk.StopReason(stopReason)}, nil
}

// Cancel signals the application service to cancel ongoing work for a session.
func (a *Agent) Cancel(ctx context.Context, notif sdk.CancelNotification) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = internalErr(fmt.Sprintf("panic recovered: %v", r))
		}
	}()
	params, jerr := json.Marshal(notif)
	if jerr != nil {
		return internalErr(jerr.Error())
	}
	_, svcErr := a.svc.HandleSessionCancel(ctx, params)
	if svcErr != nil {
		return toACPError(svcErr)
	}
	return nil
}

func (a *Agent) Authenticate(_ context.Context, _ sdk.AuthenticateRequest) (sdk.AuthenticateResponse, error) {
	return sdk.AuthenticateResponse{}, methodNotFoundErr(string(sdk.AgentMethodAuthenticate))
}

func (a *Agent) CloseSession(_ context.Context, _ sdk.CloseSessionRequest) (sdk.CloseSessionResponse, error) {
	return sdk.CloseSessionResponse{}, methodNotFoundErr(string(sdk.AgentMethodSessionClose))
}

func (a *Agent) ListSessions(_ context.Context, _ sdk.ListSessionsRequest) (sdk.ListSessionsResponse, error) {
	return sdk.ListSessionsResponse{}, methodNotFoundErr(string(sdk.AgentMethodSessionList))
}

func (a *Agent) ResumeSession(_ context.Context, _ sdk.ResumeSessionRequest) (sdk.ResumeSessionResponse, error) { //nolint:gocritic // hugeParam: signature fixed by sdk.Agent interface
	return sdk.ResumeSessionResponse{}, methodNotFoundErr(string(sdk.AgentMethodSessionResume))
}

func (a *Agent) SetSessionConfigOption(_ context.Context, _ sdk.SetSessionConfigOptionRequest) (sdk.SetSessionConfigOptionResponse, error) {
	return sdk.SetSessionConfigOptionResponse{}, methodNotFoundErr(string(sdk.AgentMethodSessionSetConfigOption))
}

func (a *Agent) SetSessionMode(_ context.Context, _ sdk.SetSessionModeRequest) (sdk.SetSessionModeResponse, error) {
	return sdk.SetSessionModeResponse{}, methodNotFoundErr(string(sdk.AgentMethodSessionSetMode))
}
