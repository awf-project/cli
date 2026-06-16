package application_test

// service_validate_options_test.go proves that ValidateLoadedWorkflow honors
// ports.ValidateOptions by observing BEHAVIORAL effects on a
// capturingValidatorProvider, not merely parsing flags.
//
// Three cases:
//   A – SkipPlugins:true  ⇒ provider NOT called
//   B – zero opts         ⇒ provider IS called
//   C – ValidatorTimeout:1ns ⇒ provider receives a ctx with a deadline (or
//       the call returns a deadline error when the timeout fires first)

import (
	"context"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingValidatorProvider records whether the ctx passed to ValidateWorkflow
// carries a deadline and blocks until the ctx is cancelled. Used for Case C.
type blockingValidatorProvider struct {
	called           bool
	receivedDeadline bool
}

func (b *blockingValidatorProvider) ValidateWorkflow(ctx context.Context, _ []byte) ([]ports.ValidationResult, error) {
	b.called = true
	_, b.receivedDeadline = ctx.Deadline()
	// Block until ctx is cancelled so the caller can assert on the deadline.
	<-ctx.Done()
	return nil, ctx.Err()
}

func (b *blockingValidatorProvider) ValidateStep(_ context.Context, _ []byte, _ string) ([]ports.ValidationResult, error) {
	return nil, nil
}

var _ ports.WorkflowValidatorProvider = (*blockingValidatorProvider)(nil)

// buildSvcWithProvider is a test-local helper to avoid repetition.
func buildSvcWithProvider(t *testing.T, provider ports.WorkflowValidatorProvider) *application.WorkflowService {
	t.Helper()
	repo := newMockRepository()
	repo.workflows["test"] = validWorkflow()
	svc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, newMockExpressionValidator())
	svc.SetValidatorProvider(provider)
	return svc
}

// TestValidateLoadedWorkflow_SkipPlugins_ProviderNotCalled (Case A):
// when SkipPlugins is true the provider must NOT be invoked.
func TestValidateLoadedWorkflow_SkipPlugins_ProviderNotCalled(t *testing.T) {
	provider := &capturingValidatorProvider{}
	svc := buildSvcWithProvider(t, provider)
	wf := validWorkflow()

	err := svc.ValidateLoadedWorkflow(context.Background(), wf, "test", ports.ValidateOptions{SkipPlugins: true})

	require.NoError(t, err)
	assert.False(t, provider.validateWorkflowCalled,
		"provider must NOT be called when SkipPlugins=true")
}

// TestValidateLoadedWorkflow_ZeroOpts_ProviderCalled (Case B):
// zero-value opts must invoke the provider (full validation, no skip).
func TestValidateLoadedWorkflow_ZeroOpts_ProviderCalled(t *testing.T) {
	provider := &capturingValidatorProvider{}
	svc := buildSvcWithProvider(t, provider)
	wf := validWorkflow()

	err := svc.ValidateLoadedWorkflow(context.Background(), wf, "test", ports.ValidateOptions{})

	require.NoError(t, err)
	assert.True(t, provider.validateWorkflowCalled,
		"provider must be called when opts are zero (full validation)")
}

// TestValidateLoadedWorkflow_ValidatorTimeout_CtxHasDeadline (Case C):
// ValidatorTimeout > 0 must cause the ctx passed to the provider to carry a
// deadline. The blocking provider observes the deadline and blocks until it
// fires; the call returns a deadline error.
func TestValidateLoadedWorkflow_ValidatorTimeout_CtxHasDeadline(t *testing.T) {
	provider := &blockingValidatorProvider{}
	svc := buildSvcWithProvider(t, provider)
	wf := validWorkflow()

	err := svc.ValidateLoadedWorkflow(context.Background(), wf, "test", ports.ValidateOptions{
		ValidatorTimeout: 1 * time.Nanosecond,
	})

	// The provider was reached (called=true) and received a deadline context.
	assert.True(t, provider.called, "provider must be called when ValidatorTimeout > 0")
	assert.True(t, provider.receivedDeadline,
		"provider must receive a ctx with a deadline when ValidatorTimeout > 0")
	// The call must return a deadline/timeout error (context.DeadlineExceeded).
	require.Error(t, err, "ValidateLoadedWorkflow must return an error when the validator timeout fires")
}
