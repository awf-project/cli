package application

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/operation"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type preflightTestProvider struct {
	name           string
	validateErr    error
	preflightErr   error
	validateCalls  atomic.Int32
	preflightCalls atomic.Int32
	executeCalls   atomic.Int32
	options        map[string]any
}

func (p *preflightTestProvider) Execute(context.Context, string, map[string]any, io.Writer, io.Writer) (*workflow.AgentResult, error) {
	p.executeCalls.Add(1)
	return &workflow.AgentResult{Provider: p.name}, nil
}

func (p *preflightTestProvider) ExecuteConversation(context.Context, *workflow.ConversationState, string, map[string]any, io.Writer, io.Writer) (*workflow.ConversationResult, error) {
	p.executeCalls.Add(1)
	return &workflow.ConversationResult{Provider: p.name}, nil
}

func (p *preflightTestProvider) Name() string { return p.name }

func (p *preflightTestProvider) Validate() error {
	p.validateCalls.Add(1)
	return p.validateErr
}

func (p *preflightTestProvider) Preflight(_ context.Context, options map[string]any) error {
	p.preflightCalls.Add(1)
	p.options = options
	return p.preflightErr
}

type availabilityOnlyTestProvider struct {
	provider *preflightTestProvider
}

func (p *availabilityOnlyTestProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	return p.provider.Execute(ctx, prompt, options, stdout, stderr)
}

func (p *availabilityOnlyTestProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	return p.provider.ExecuteConversation(ctx, state, prompt, options, stdout, stderr)
}

func (p *availabilityOnlyTestProvider) Name() string    { return p.provider.Name() }
func (p *availabilityOnlyTestProvider) Validate() error { return p.provider.Validate() }

type preflightTestRegistry struct {
	provider ports.AgentProvider
}

func (r *preflightTestRegistry) Register(provider ports.AgentProvider) error {
	r.provider = provider
	return nil
}

func (r *preflightTestRegistry) Get(name string) (ports.AgentProvider, error) {
	if r.provider == nil || r.provider.Name() != name {
		return nil, errors.New("provider not found")
	}
	return r.provider, nil
}

func (r *preflightTestRegistry) List() []string {
	if r.provider == nil {
		return nil
	}
	return []string{r.provider.Name()}
}

func (r *preflightTestRegistry) Has(name string) bool {
	return r.provider != nil && r.provider.Name() == name
}

type workflowServicePreflightWiring interface {
	SetAgentRegistry(ports.AgentRegistry)
	SetUserInputReader(ports.UserInputReader)
}

type unavailableConversationReader struct {
	reads int
}

func (r *unavailableConversationReader) ReadInput(context.Context) (string, error) {
	r.reads++
	return "", io.EOF
}

func (*unavailableConversationReader) PreflightInput(context.Context) error { return io.EOF }

func TestWorkflowPreflightRejectsNonNilUnavailableInput(t *testing.T) {
	provider := &preflightTestProvider{name: "test"}
	reader := &unavailableConversationReader{}
	service := NewWorkflowService(nil, nil, nil, nil, nil)
	service.SetAgentRegistry(&preflightTestRegistry{provider: provider})
	service.SetUserInputReader(reader)
	err := service.ValidateLoadedWorkflowWithInputs(context.Background(), preflightWorkflow("test", "conversation"), "preflight", ports.ValidateOptions{}, nil)
	var structured *domainerrors.StructuredError
	require.ErrorAs(t, err, &structured)
	assert.Equal(t, "interactive_input_required", structured.Details["reason"])
	assert.ErrorIs(t, err, io.EOF)
	assert.Zero(t, reader.reads)
	assert.Zero(t, provider.executeCalls.Load())
}

func TestWorkflowPreflightRejectsUnavailableUnauthenticatedOrInteractiveProviders(t *testing.T) {
	providerFailure := errors.New("provider readiness canary-secret")
	cancelled := context.Canceled
	deadline := context.DeadlineExceeded

	tests := []struct {
		name       string
		provider   ports.AgentProvider
		providerID string
		mode       string
		reader     ports.UserInputReader
		wantReason string
		wantCause  error
	}{
		{name: "registry miss", providerID: "missing", wantReason: "unavailable"},
		{name: "unavailable binary", provider: &preflightTestProvider{name: "test", validateErr: providerFailure}, providerID: "test", wantReason: "unavailable", wantCause: providerFailure},
		{name: "missing preflight capability", provider: &availabilityOnlyTestProvider{provider: &preflightTestProvider{name: "test"}}, providerID: "test", wantReason: "preflight_unsupported"},
		{name: "unauthenticated", provider: &preflightTestProvider{name: "test", preflightErr: providerFailure}, providerID: "test", wantReason: "not_ready", wantCause: providerFailure},
		{name: "cancelled preflight", provider: &preflightTestProvider{name: "test", preflightErr: cancelled}, providerID: "test", wantReason: "not_ready", wantCause: cancelled},
		{name: "timed out preflight", provider: &preflightTestProvider{name: "test", preflightErr: deadline}, providerID: "test", wantReason: "not_ready", wantCause: deadline},
		{name: "conversation without reader", provider: &preflightTestProvider{name: "test"}, providerID: "test", mode: "conversation", wantReason: "interactive_input_required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewWorkflowService(nil, nil, nil, nil, nil)
			wiring, ok := any(service).(workflowServicePreflightWiring)
			require.True(t, ok, "WorkflowService must expose provider-preflight dependency setters")
			wiring.SetAgentRegistry(&preflightTestRegistry{provider: tt.provider})
			wiring.SetUserInputReader(tt.reader)

			err := service.ValidateLoadedWorkflowWithInputs(context.Background(), preflightWorkflow(tt.providerID, tt.mode), "preflight", ports.ValidateOptions{}, nil)

			var structured *domainerrors.StructuredError
			require.ErrorAs(t, err, &structured)
			assert.Equal(t, domainerrors.ErrorCodeUserInputValidationFailed, structured.Code)
			assert.Equal(t, map[string]any{"provider": tt.providerID, "step": "agent", "reason": tt.wantReason}, structured.Details)
			assert.NotContains(t, err.Error(), "canary-secret")
			if tt.wantCause != nil {
				assert.ErrorIs(t, err, tt.wantCause)
			}
			if provider, ok := tt.provider.(*preflightTestProvider); ok {
				assert.Zero(t, provider.executeCalls.Load())
			}
		})
	}

	t.Run("resolves supplied provider and options before successful readiness", func(t *testing.T) {
		provider := &preflightTestProvider{name: "test"}
		service := NewWorkflowService(nil, nil, nil, nil, nil)
		wiring, ok := any(service).(workflowServicePreflightWiring)
		require.True(t, ok, "WorkflowService must expose provider-preflight dependency setters")
		wiring.SetAgentRegistry(&preflightTestRegistry{provider: provider})
		wiring.SetUserInputReader(&mockUserInputReader{})
		wf := preflightWorkflow("{{inputs.provider}}", "single")
		wf.Inputs = []workflow.Input{{Name: "provider", Required: true}, {Name: "token", Required: true}}
		wf.Steps["agent"].Agent.Options = map[string]any{"api_token": "{{inputs.token}}"} //nolint:gosec // Template placeholder verifies resolution; it is not a credential.

		err := service.ValidateLoadedWorkflowWithInputs(context.Background(), wf, "preflight", ports.ValidateOptions{}, map[string]any{"provider": "test", "token": "resolved-token"})

		require.NoError(t, err)
		assert.Equal(t, int32(1), provider.validateCalls.Load())
		assert.Equal(t, int32(1), provider.preflightCalls.Load())
		assert.Equal(t, "resolved-token", provider.options["api_token"])
		assert.Zero(t, provider.executeCalls.Load())
	})

	// These cases deliberately validate without executing a workflow. They establish
	// the static-resolution boundary: unknown runtime values defer provider work,
	// while defaults and subsequently supplied values make the same checks decidable.
	t.Run("defers and later resolves provider readiness", func(t *testing.T) {
		tests := []struct {
			name              string
			providerName      string
			inputs            []workflow.Input
			options           map[string]any
			runInputs         map[string]any
			wantValidateCalls int32
			wantPreflight     int32
			wantOption        any
		}{
			{
				name:         "unresolved provider is deferred",
				providerName: "{{inputs.provider}}",
				inputs:       []workflow.Input{{Name: "provider", Required: true}},
			},
			{
				name:              "runtime derived readiness option defers preflight but checks availability",
				providerName:      "test",
				options:           map[string]any{"api_token": "{{states.login.output}}"}, //nolint:gosec // Template placeholder verifies deferred resolution; it is not a credential.
				wantValidateCalls: 1,
			},
			{
				name:              "declared default makes readiness option decidable",
				providerName:      "test",
				inputs:            []workflow.Input{{Name: "token", Default: "default-token"}},
				options:           map[string]any{"api_token": "{{inputs.token}}"}, //nolint:gosec // Template placeholder verifies default resolution; it is not a credential.
				wantValidateCalls: 1,
				wantPreflight:     1,
				wantOption:        "default-token",
			},
			{
				name:              "supplied value runs previously deferred readiness check",
				providerName:      "test",
				inputs:            []workflow.Input{{Name: "token", Required: true}},
				options:           map[string]any{"api_token": "{{inputs.token}}"}, //nolint:gosec // Template placeholder verifies supplied-input resolution; it is not a credential.
				runInputs:         map[string]any{"token": "runtime-token"},
				wantValidateCalls: 1,
				wantPreflight:     1,
				wantOption:        "runtime-token",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				provider := &preflightTestProvider{name: "test"}
				service := NewWorkflowService(nil, nil, nil, nil, nil)
				wiring, ok := any(service).(workflowServicePreflightWiring)
				require.True(t, ok, "WorkflowService must expose provider-preflight dependency setters")
				wiring.SetAgentRegistry(&preflightTestRegistry{provider: provider})

				wf := preflightWorkflow(tt.providerName, "single")
				wf.Inputs = tt.inputs
				wf.Steps["agent"].Agent.Options = tt.options
				var err error
				if tt.runInputs == nil {
					err = service.ValidateLoadedWorkflow(context.Background(), wf, wf.Name, ports.ValidateOptions{})
				} else {
					err = service.ValidateLoadedWorkflowWithInputs(context.Background(), wf, wf.Name, ports.ValidateOptions{}, tt.runInputs)
				}

				require.NoError(t, err)
				assert.Equal(t, tt.wantValidateCalls, provider.validateCalls.Load())
				assert.Equal(t, tt.wantPreflight, provider.preflightCalls.Load())
				if tt.wantPreflight > 0 {
					assert.Equal(t, tt.wantOption, provider.options["api_token"])
				}
				assert.Zero(t, provider.executeCalls.Load(), "validation must not execute the provider")
			})
		}
	})

	t.Run("ExecutionSetup wires template and operation validation", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name: "setup-validation", Initial: "command",
			Steps: map[string]*workflow.Step{
				"command": {Name: "command", Type: workflow.StepTypeCommand, Command: "echo {{states.missing.output}}", OnSuccess: "done"},
				"done":    {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			},
		}
		service := buildPreflightWorkflowService(t, wf)
		err := service.ValidateLoadedWorkflow(context.Background(), wf, wf.Name, ports.ValidateOptions{})
		var templateErr workflow.ValidationError
		require.ErrorAs(t, err, &templateErr)
		assert.Equal(t, workflow.ErrUndefinedStep, templateErr.Code)
		assert.Equal(t, "steps.command.command", templateErr.Path)

		operationWorkflow := &workflow.Workflow{
			Name: "setup-operation", Initial: "request",
			Steps: map[string]*workflow.Step{
				"request": {Name: "request", Type: workflow.StepTypeOperation, Operation: "http.request", OnSuccess: "done"},
				"done":    {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			},
		}
		err = service.ValidateLoadedWorkflow(context.Background(), operationWorkflow, operationWorkflow.Name, ports.ValidateOptions{})
		var operationErr *domainerrors.StructuredError
		require.ErrorAs(t, err, &operationErr)
		assert.Equal(t, domainerrors.ErrorCodeUserInputValidationFailed, operationErr.Code)
		assert.Equal(t, map[string]any{
			"operation":   "http.request",
			"step":        "request",
			"field_paths": []string{"steps.request.inputs.method", "steps.request.inputs.url"},
		}, operationErr.Details)
		assert.ErrorIs(t, operationErr, operation.ErrInvalidInputs)
	})

	t.Run("ExecutionSetup wires registry preflight and conversation reader", func(t *testing.T) {
		// A built service must use the registry created by ExecutionSetup, rather than
		// silently treating a statically selected provider as valid.
		noReaderService := buildPreflightWorkflowService(t, preflightWorkflow("missing-provider", "single"))
		err := noReaderService.ValidateLoadedWorkflowWithInputs(context.Background(), preflightWorkflow("missing-provider", "single"), "preflight", ports.ValidateOptions{}, nil)
		var unavailable *domainerrors.StructuredError
		require.ErrorAs(t, err, &unavailable)
		assert.Equal(t, domainerrors.ErrorCodeUserInputValidationFailed, unavailable.Code)
		assert.Equal(t, map[string]any{"provider": "missing-provider", "step": "agent", "reason": "unavailable"}, unavailable.Details)

		conversation := preflightWorkflow("{{inputs.provider}}", "conversation")
		conversation.Inputs = []workflow.Input{{Name: "provider", Required: true}}
		withoutReader := buildPreflightWorkflowService(t, conversation)
		err = withoutReader.ValidateLoadedWorkflow(context.Background(), conversation, "preflight", ports.ValidateOptions{})
		var missingReader *domainerrors.StructuredError
		require.ErrorAs(t, err, &missingReader)
		assert.Equal(t, "interactive_input_required", missingReader.Details["reason"])

		withReader := buildPreflightWorkflowService(t, conversation, WithUserInputReader(&mockUserInputReader{}))
		require.NoError(t, withReader.ValidateLoadedWorkflow(context.Background(), conversation, "preflight", ports.ValidateOptions{}))
	})
}

func preflightWorkflow(provider, mode string) *workflow.Workflow {
	return &workflow.Workflow{
		Name: "preflight", Initial: "agent",
		Steps: map[string]*workflow.Step{
			"agent": {Name: "agent", Type: workflow.StepTypeAgent, OnSuccess: "done", Agent: &workflow.AgentConfig{Provider: provider, Prompt: "hello", Mode: mode}},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
}

func buildPreflightWorkflowService(t *testing.T, wf *workflow.Workflow, opts ...SetupOption) *WorkflowService {
	t.Helper()
	repo := testmocks.NewMockWorkflowRepository()
	repo.AddWorkflow(wf.Name, wf)
	setup := NewExecutionSetup(repo, testmocks.NewMockStateStore(), testmocks.NewMockCommandExecutor(), testmocks.NewMockLogger(), opts...)
	result, err := setup.Build(context.Background())
	require.NoError(t, err)
	t.Cleanup(result.Cleanup)
	return result.WorkflowSvc
}
