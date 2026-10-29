package application

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	httpoperations "github.com/awf-project/cli/internal/infrastructure/http"
	"github.com/awf-project/cli/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type deadlineObservingExecutor struct {
	contextReceived chan context.Context
	finished        chan struct{}
	waitForDone     bool
}

func (e *deadlineObservingExecutor) Execute(ctx context.Context, _ *ports.Command) (*ports.CommandResult, error) {
	e.contextReceived <- ctx
	if !e.waitForDone {
		close(e.finished)
		return &ports.CommandResult{}, nil
	}

	<-ctx.Done()
	close(e.finished)
	return nil, ctx.Err()
}

type deadlineObservingStepTypeProvider struct {
	contextReceived chan context.Context
	finished        chan struct{}
	waitForDone     bool
}

func (*deadlineObservingStepTypeProvider) HasStepType(string) bool { return true }

func (p *deadlineObservingStepTypeProvider) ExecuteStep(
	ctx context.Context,
	_ ports.StepExecuteRequest,
) (ports.StepExecuteResult, error) {
	p.contextReceived <- ctx
	if !p.waitForDone {
		close(p.finished)
		return ports.StepExecuteResult{}, nil
	}

	<-ctx.Done()
	close(p.finished)
	return ports.StepExecuteResult{}, ctx.Err()
}

func TestExecutionService_NormalStep_WithoutConfiguredTimeout(t *testing.T) {
	executor := newDeadlineObservingExecutor(false)
	svc := newDeadlinePolicyExecutionService(executor)
	parent := context.Background()
	step := deadlinePolicyCommandStep(0)

	next, err := svc.executeStep(parent, &workflow.Workflow{}, step, deadlinePolicyExecutionContext())

	require.NoError(t, err)
	assert.Empty(t, next)
	observed := receiveDeadlinePolicyContext(t, executor.contextReceived)
	assert.Equal(t, parent, observed)
	_, hasDeadline := observed.Deadline()
	assert.False(t, hasDeadline)
	requireDeadlinePolicyCleanup(t, executor.finished)
}

func TestExecutionService_NormalStep_ExplicitTimeout(t *testing.T) {
	const timeoutSeconds = 1
	executor := newDeadlineObservingExecutor(true)
	svc := newDeadlinePolicyExecutionService(executor)
	startedAt := time.Now()

	_, err := svc.executeStep(
		context.Background(),
		&workflow.Workflow{},
		deadlinePolicyCommandStep(timeoutSeconds),
		deadlinePolicyExecutionContext(),
	)

	assertLocalTimeoutContract(t, err, timeoutSeconds)
	assertConfiguredDeadline(t, receiveDeadlinePolicyContext(t, executor.contextReceived), startedAt, timeoutSeconds)
	requireDeadlinePolicyCleanup(t, executor.finished)
}

func TestExecutionService_NormalStep_ParentCancellation(t *testing.T) {
	executor := newDeadlineObservingExecutor(true)
	svc := newDeadlinePolicyExecutionService(executor)
	parent, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan error, 1)

	go func() {
		_, err := svc.executeStep(parent, &workflow.Workflow{}, deadlinePolicyCommandStep(0), deadlinePolicyExecutionContext())
		result <- err
	}()

	observed := receiveDeadlinePolicyContext(t, executor.contextReceived)
	assert.Equal(t, parent, observed)
	cancel()
	err := receiveDeadlinePolicyError(t, result)
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, errors.Is(err, context.DeadlineExceeded))
	assertDeadlineHasNoLocalMetadata(t, err)
	requireDeadlinePolicyCleanup(t, executor.finished)
}

func TestExecutionService_CustomStep_WithoutConfiguredTimeout(t *testing.T) {
	provider := newDeadlineObservingStepTypeProvider(false)
	wf := deadlinePolicyCustomWorkflow(0)
	svc := newDeadlinePolicyPublicExecutionService()
	svc.SetStepTypeProvider(provider)

	execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "custom-without-timeout")

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	observed := receiveDeadlinePolicyContext(t, provider.contextReceived)
	_, hasDeadline := observed.Deadline()
	assert.False(t, hasDeadline)
	requireDeadlinePolicyCleanup(t, provider.finished)
}

func TestExecutionService_CustomStep_ExplicitTimeout(t *testing.T) {
	const timeoutSeconds = 1
	provider := newDeadlineObservingStepTypeProvider(true)
	wf := deadlinePolicyCustomWorkflow(timeoutSeconds)
	svc := newDeadlinePolicyPublicExecutionService()
	svc.SetStepTypeProvider(provider)
	startedAt := time.Now()

	execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "custom-explicit-timeout")

	assertLocalTimeoutContract(t, err, timeoutSeconds)
	assert.Equal(t, workflow.StatusCancelled, execCtx.Status)
	assertConfiguredDeadline(t, receiveDeadlinePolicyContext(t, provider.contextReceived), startedAt, timeoutSeconds)
	requireDeadlinePolicyCleanup(t, provider.finished)
}

func TestExecutionService_CustomStep_ParentCancellation(t *testing.T) {
	provider := newDeadlineObservingStepTypeProvider(true)
	wf := deadlinePolicyCustomWorkflow(0)
	svc := newDeadlinePolicyPublicExecutionService()
	svc.SetStepTypeProvider(provider)
	parent, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan error, 1)

	go func() {
		_, err := svc.RunWithWorkflowAndRunID(parent, wf, nil, "custom-parent-cancellation")
		result <- err
	}()

	observed := receiveDeadlinePolicyContext(t, provider.contextReceived)
	_, hasDeadline := observed.Deadline()
	assert.False(t, hasDeadline)
	cancel()
	err := receiveDeadlinePolicyError(t, result)
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, errors.Is(err, context.DeadlineExceeded))
	assertDeadlineHasNoLocalMetadata(t, err)
	requireDeadlinePolicyCleanup(t, provider.finished)
}

func TestExecutionService_HTTPOperation_DeadlineErrorIsClassified(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		requestStarted <- struct{}{}
		<-request.Context().Done()
	}))
	defer server.Close()

	svc := newDeadlinePolicyPublicExecutionService()
	svc.SetOperationProvider(httpoperations.NewHTTPOperationProvider(httpx.NewClient(httpx.WithTimeout(0)), newMockLogger()))
	wf := &workflow.Workflow{
		Name:    "http-operation-timeout",
		Initial: "request",
		Steps: map[string]*workflow.Step{
			"request": {
				Name:      "request",
				Type:      workflow.StepTypeOperation,
				Operation: "http.request",
				OperationInputs: map[string]any{
					"url":     server.URL,
					"method":  http.MethodGet,
					"timeout": 1,
				},
			},
		},
	}

	type outcome struct {
		execution *workflow.ExecutionContext
		err       error
	}
	completed := make(chan outcome, 1)
	go func() {
		execution, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "http-operation-timeout")
		completed <- outcome{execution: execution, err: err}
	}()

	requireDeadlinePolicyCleanup(t, requestStarted)
	result := receiveDeadlinePolicyOutcome(t, completed)
	require.NotNil(t, result.execution)
	require.Error(t, result.err)
	assert.ErrorIs(t, result.err, context.DeadlineExceeded)
	assert.Equal(t, "execution", classifyErrorType(result.err))
	assert.Equal(t, workflow.StatusCancelled, result.execution.Status)
}

func TestExecutionDeadlinePolicy_LocalExpiryReportsConfiguredDuration(t *testing.T) {
	const timeoutSeconds = 1

	parent := context.Background()
	executionCtx, cancel, derived := deriveExecutionContext(parent, timeoutSeconds)
	require.True(t, derived)
	t.Cleanup(cancel)

	<-executionCtx.Done()
	err := classifyExecutionContextError(parent, executionCtx, timeoutSeconds, executionCtx.Err())

	var structuredErr *domainerrors.StructuredError
	require.ErrorAs(t, err, &structuredErr)
	assert.Equal(t, domainerrors.ErrorCodeExecutionCommandTimeout, structuredErr.Code)
	assert.Equal(t, 3, structuredErr.ExitCode())
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, map[string]any{
		"timeout_owner":   "step",
		"timeout_seconds": timeoutSeconds,
	}, structuredErr.Details)

	cancel()
	assert.ErrorIs(t, executionCtx.Err(), context.DeadlineExceeded)
}

func TestExecutionDeadlinePolicy_ParentExpiryRemainsInherited(t *testing.T) {
	parent, cancelParent := context.WithTimeout(context.Background(), 10*time.Millisecond)
	t.Cleanup(cancelParent)
	executionCtx, cancelExecution, derived := deriveExecutionContext(parent, 1)
	require.True(t, derived)
	t.Cleanup(cancelExecution)

	<-executionCtx.Done()
	err := classifyExecutionContextError(parent, executionCtx, 1, executionCtx.Err())

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assertDeadlineHasNoLocalMetadata(t, err)

	cancelExecution()
	assert.ErrorIs(t, executionCtx.Err(), context.DeadlineExceeded)
}

func TestExecutionDeadlinePolicy_UserCancellationRemainsCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	executionCtx, cancelExecution, derived := deriveExecutionContext(parent, 0)
	require.False(t, derived)
	assert.Same(t, parent, executionCtx)

	cancelParent()
	<-executionCtx.Done()
	err := classifyExecutionContextError(parent, executionCtx, 0, executionCtx.Err())

	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, errors.Is(err, context.DeadlineExceeded))
	assertDeadlineHasNoLocalMetadata(t, err)

	cancelExecution()
	assert.ErrorIs(t, executionCtx.Err(), context.Canceled)
}

func TestExecutionDeadlinePolicy_HugePositiveTimeoutDoesNotExpireImmediately(t *testing.T) {
	if ^uint(0)>>63 == 0 {
		t.Skip("timeout duration overflow requires a 64-bit int")
	}

	firstOverflowingTimeoutSeconds64 := int64(1<<63-1)/int64(time.Second) + 1
	firstOverflowingTimeoutSeconds := int(firstOverflowingTimeoutSeconds64)
	executionCtx, cancel, derived := deriveExecutionContext(context.Background(), firstOverflowingTimeoutSeconds)
	require.True(t, derived)
	t.Cleanup(cancel)

	select {
	case <-executionCtx.Done():
		t.Fatalf("huge positive timeout expired immediately: %v", executionCtx.Err())
	default:
	}
}

func newDeadlineObservingExecutor(waitForDone bool) *deadlineObservingExecutor {
	return &deadlineObservingExecutor{
		contextReceived: make(chan context.Context, 1),
		finished:        make(chan struct{}),
		waitForDone:     waitForDone,
	}
}

func newDeadlineObservingStepTypeProvider(waitForDone bool) *deadlineObservingStepTypeProvider {
	return &deadlineObservingStepTypeProvider{
		contextReceived: make(chan context.Context, 1),
		finished:        make(chan struct{}),
		waitForDone:     waitForDone,
	}
}

func newDeadlinePolicyExecutionService(executor ports.CommandExecutor) *ExecutionService {
	logger := &mockLogger{}
	resolver := newMockResolver()
	return &ExecutionService{
		executor:      executor,
		logger:        logger,
		resolver:      resolver,
		hookExecutor:  NewHookExecutor(executor, logger, resolver),
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
	}
}

func deadlinePolicyExecutionContext() *workflow.ExecutionContext {
	return workflow.NewExecutionContext("deadline-policy", "deadline-policy")
}

func deadlinePolicyCommandStep(timeout int) *workflow.Step {
	return &workflow.Step{
		Name:    "command",
		Type:    workflow.StepTypeCommand,
		Command: "true",
		Timeout: timeout,
	}
}

func deadlinePolicyCustomStep(timeout int) *workflow.Step {
	return &workflow.Step{
		Name:      "custom",
		Type:      workflow.StepType("custom.deadline-policy"),
		Timeout:   timeout,
		OnSuccess: "done",
	}
}

func deadlinePolicyCustomWorkflow(timeout int) *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "deadline-policy-custom",
		Initial: "custom",
		Steps: map[string]*workflow.Step{
			"custom": deadlinePolicyCustomStep(timeout),
			"done":   {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}
}

type deadlinePolicyStateStore struct {
	states map[string]*workflow.ExecutionContext
}

func (s *deadlinePolicyStateStore) Save(_ context.Context, state *workflow.ExecutionContext) error {
	s.states[state.WorkflowID] = state
	return nil
}

func (s *deadlinePolicyStateStore) Load(_ context.Context, id string) (*workflow.ExecutionContext, error) {
	return s.states[id], nil
}

func (s *deadlinePolicyStateStore) Delete(_ context.Context, id string) error {
	delete(s.states, id)
	return nil
}

func (s *deadlinePolicyStateStore) List(context.Context) ([]string, error) {
	return nil, nil
}

func newDeadlinePolicyPublicExecutionService() *ExecutionService {
	store := &deadlinePolicyStateStore{states: make(map[string]*workflow.ExecutionContext)}
	logger := &mockLogger{}
	wfSvc := NewWorkflowService(&MockWorkflowRepository{}, store, nil, logger, nil)
	return NewExecutionService(wfSvc, nil, nil, store, logger, newMockResolver(), nil)
}

func receiveDeadlinePolicyContext(t *testing.T, contexts <-chan context.Context) context.Context {
	t.Helper()
	select {
	case ctx := <-contexts:
		return ctx
	case <-time.After(2 * time.Second):
		t.Fatal("execution dependency did not receive a context")
		return nil
	}
}

func receiveDeadlinePolicyError(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("execution did not return after parent cancellation")
		return nil
	}
}

func receiveDeadlinePolicyOutcome[T any](t *testing.T, outcomes <-chan T) T {
	t.Helper()
	select {
	case outcome := <-outcomes:
		return outcome
	case <-time.After(2 * time.Second):
		t.Fatal("execution did not return after HTTP operation timeout")
		var zero T
		return zero
	}
}

func requireDeadlinePolicyCleanup(t *testing.T, finished <-chan struct{}) {
	t.Helper()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("execution dependency leaked work")
	}
}

func assertConfiguredDeadline(t *testing.T, ctx context.Context, startedAt time.Time, timeoutSeconds int) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	assert.WithinDuration(t, startedAt.Add(time.Duration(timeoutSeconds)*time.Second), deadline, 250*time.Millisecond)
}

func assertLocalTimeoutContract(t *testing.T, err error, timeoutSeconds int) {
	t.Helper()
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.False(t, errors.Is(err, context.Canceled))

	var structuredErr *domainerrors.StructuredError
	require.ErrorAs(t, err, &structuredErr)
	assert.Equal(t, domainerrors.ErrorCodeExecutionCommandTimeout, structuredErr.Code)
	assert.Equal(t, 3, structuredErr.ExitCode())
	assert.Equal(t, map[string]any{
		"timeout_owner":   "step",
		"timeout_seconds": timeoutSeconds,
	}, structuredErr.Details)
}

func assertDeadlineHasNoLocalMetadata(t *testing.T, err error) {
	t.Helper()

	var structuredErr *domainerrors.StructuredError
	if errors.As(err, &structuredErr) {
		assert.NotContains(t, structuredErr.Details, "timeout_owner")
		assert.NotContains(t, structuredErr.Details, "timeout_seconds")
	}
}
