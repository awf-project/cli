package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type resumeContextKey struct{}

type resumeContextObservation struct {
	value       any
	deadline    time.Time
	hasDeadline bool
}

type contextObservingResumeFacade struct {
	*mockFacadeForResumeList
	received chan<- resumeContextObservation
	release  <-chan struct{}
}

type historyRecordingResumeFacade struct {
	*mockFacadeForResumeList
	records []ports.RunRecord
	filter  ports.HistoryFilter
	ctx     context.Context
}

func (f *historyRecordingResumeFacade) History(ctx context.Context, filter ports.HistoryFilter) ([]ports.RunRecord, error) {
	f.ctx = ctx
	f.filter = filter
	return f.records, nil
}

func (f *contextObservingResumeFacade) Resume(ctx context.Context, _ ports.ResumeRequest) (ports.RunSession, error) {
	deadline, hasDeadline := ctx.Deadline()
	f.received <- resumeContextObservation{
		value:       ctx.Value(resumeContextKey{}),
		deadline:    deadline,
		hasDeadline: hasDeadline,
	}
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("resume cancelled: %w", ctx.Err())
	case <-f.release:
		return nil, errors.New("test released facade after cancellation was not propagated")
	}
}

func TestResumeViaFacade_PropagatesCommandContext(t *testing.T) {
	t.Run("cancellation after facade receives context", func(t *testing.T) {
		parent, cancel := context.WithTimeout(
			context.WithValue(context.Background(), resumeContextKey{}, "canary"),
			5*time.Second,
		)
		defer cancel()
		deadline, ok := parent.Deadline()
		require.True(t, ok)

		received := make(chan resumeContextObservation, 1)
		release := make(chan struct{})
		cfg := DefaultConfig()
		cfg.Facade = &contextObservingResumeFacade{
			mockFacadeForResumeList: &mockFacadeForResumeList{},
			received:                received,
			release:                 release,
		}
		cmd, writer, formatter := newResumeContextTestDependencies(parent, cfg)
		errCh := make(chan error, 1)
		go func() {
			errCh <- runResumeViaFacade(cmd, cfg, writer, formatter, "workflow-id", nil, "")
		}()

		observation := <-received
		assert.Equal(t, "canary", observation.value)
		assert.True(t, observation.hasDeadline)
		assert.Equal(t, deadline, observation.deadline)
		cancel()

		select {
		case err := <-errCh:
			assertResumeCancellationExit(t, err)
		case <-time.After(200 * time.Millisecond):
			close(release)
			<-errCh
			assert.Fail(t, "resume helper did not terminate after command context cancellation")
		}
	})

	t.Run("already cancelled parent", func(t *testing.T) {
		parent, cancel := context.WithCancel(
			context.WithValue(context.Background(), resumeContextKey{}, "already-cancelled"),
		)
		cancel()

		received := make(chan resumeContextObservation, 1)
		release := make(chan struct{})
		cfg := DefaultConfig()
		cfg.Facade = &contextObservingResumeFacade{
			mockFacadeForResumeList: &mockFacadeForResumeList{},
			received:                received,
			release:                 release,
		}
		cmd, writer, formatter := newResumeContextTestDependencies(parent, cfg)

		errCh := make(chan error, 1)
		go func() {
			errCh <- runResumeViaFacade(cmd, cfg, writer, formatter, "workflow-id", nil, "")
		}()
		assert.Equal(t, "already-cancelled", (<-received).value)
		select {
		case err := <-errCh:
			assertResumeCancellationExit(t, err)
		case <-time.After(200 * time.Millisecond):
			close(release)
			<-errCh
			assert.Fail(t, "resume helper did not honor an already-cancelled command context")
		}
	})

	t.Run("facade setup and execution use the command context", func(t *testing.T) {
		parent, cancel := context.WithTimeout(
			context.WithValue(context.Background(), resumeContextKey{}, "setup-canary"),
			5*time.Second,
		)
		defer cancel()
		cancel()

		storagePath := t.TempDir()
		t.Setenv("XDG_DATA_HOME", storagePath)
		t.Setenv("XDG_CONFIG_HOME", storagePath)
		cfg := DefaultConfig()
		cfg.StoragePath = storagePath
		cmd, _, _ := newResumeContextTestDependencies(parent, cfg)

		assertResumeCancellationExit(t, runResume(cmd, cfg, "workflow-id", nil))
	})
}

func TestResumeListUsesLegacyRunIdentity(t *testing.T) {
	facade := &historyRecordingResumeFacade{
		mockFacadeForResumeList: &mockFacadeForResumeList{},
		records: []ports.RunRecord{{
			RunID:        "legacy-run-id",
			WorkflowName: "legacy-workflow",
			Status:       ports.RunStateFailed,
			CompletedAt:  time.Date(2026, time.October, 20, 12, 0, 0, 0, time.UTC),
		}},
	}
	cfg := DefaultConfig()
	cfg.Facade = facade
	cfg.OutputFormat = ui.FormatJSON
	commandCtx, cancel := context.WithTimeout(
		context.WithValue(context.Background(), resumeContextKey{}, "list-canary"),
		5*time.Second,
	)
	defer cancel()
	deadline, hasDeadline := commandCtx.Deadline()
	require.True(t, hasDeadline)

	cmd, cleanup := newRootCommandWithConfig(cfg, false)
	t.Cleanup(cleanup)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"--storage", t.TempDir(), "--format=json", "resume", "--list"})
	require.NoError(t, cmd.ExecuteContext(commandCtx))
	assert.Contains(t, buf.String(), "legacy-run-id")
	assert.Equal(t, math.MaxInt, facade.filter.Limit)
	assert.Empty(t, facade.filter.Status)
	assert.Equal(t, commandCtx, facade.ctx)
	assert.Equal(t, "list-canary", facade.ctx.Value(resumeContextKey{}))
	actualDeadline, actualHasDeadline := facade.ctx.Deadline()
	assert.True(t, actualHasDeadline)
	assert.Equal(t, deadline, actualDeadline)
}

// Exercise real root-command dispatch and the run-capable facade, which resume
// constructs itself rather than using the read-only facade injected for lists.
func TestResumeCommand_PropagatesCommandContext(t *testing.T) {
	for _, scenario := range []string{"cancel after command starts", "deadline", "already cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
			t.Setenv("AWF_WORKFLOWS_PATH", root)
			t.Setenv("AWF_AUDIT_LOG", "off")
			ready := filepath.Join(root, "command ready")
			source := fmt.Sprintf(`name: context-resume
version: "1.0.0"
states:
  initial: wait
  wait:
    type: step
    command: "touch %s; while :; do sleep 1; done"
    on_success: done
  done:
    type: terminal
`, interpolation.ShellEscape(ready))
			require.NoError(t, os.WriteFile(filepath.Join(root, "context-resume.yaml"), []byte(source), 0o600))
			stateStore := store.NewJSONStore(filepath.Join(root, "storage", "states"))
			state := workflow.NewExecutionContext("context-workflow", "context-resume")
			state.Status = workflow.StatusFailed
			state.CurrentStep = "wait"
			require.NoError(t, stateStore.Save(context.Background(), state))
			cmd, cleanup := NewRootCommandAutoFacade()
			t.Cleanup(cleanup)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{"--storage", filepath.Join(root, "storage"), "--no-color", "resume", state.WorkflowID})
			timeout := 5 * time.Second
			if scenario == "deadline" {
				timeout = 2 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if scenario == "already cancelled" {
				cancel()
				assertResumeCancellationExit(t, cmd.ExecuteContext(ctx))
				assert.NoFileExists(t, ready)
				return
			}
			result := make(chan error, 1)
			done := make(chan struct{})
			go func() {
				defer close(done)
				result <- cmd.ExecuteContext(ctx)
			}()
			t.Cleanup(func() {
				cancel()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("root resume did not terminate")
				}
			})
			require.Eventually(t, func() bool {
				_, err := os.Stat(ready)
				return err == nil
			}, time.Second, 10*time.Millisecond, "resumed command must start before cancellation")
			if scenario != "deadline" {
				cancel()
			}
			select {
			case err := <-result:
				if scenario == "deadline" {
					assert.ErrorIs(t, err, context.DeadlineExceeded)
					var exitErr *exitError
					require.ErrorAs(t, err, &exitErr)
					assert.Equal(t, ExitExecution, exitErr.ExitCode())
				} else {
					assertResumeCancellationExit(t, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("root resume did not honor its context")
			}
			persisted, err := stateStore.Load(context.Background(), state.WorkflowID)
			require.NoError(t, err)
			require.NotNil(t, persisted)
			assert.Equal(t, workflow.StatusCancelled, persisted.Status)
		})
	}
}

func (f *contextObservingResumeFacade) History(ctx context.Context, _ ports.HistoryFilter) ([]ports.RunRecord, error) {
	deadline, hasDeadline := ctx.Deadline()
	f.received <- resumeContextObservation{value: ctx.Value(resumeContextKey{}), deadline: deadline, hasDeadline: hasDeadline}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.release:
		return nil, errors.New("test released history facade without cancellation")
	}
}

func TestResumeList_PropagatesRootCommandCancellation(t *testing.T) {
	for _, alreadyCancelled := range []bool{false, true} {
		t.Run(fmt.Sprintf("already_cancelled=%t", alreadyCancelled), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), resumeContextKey{}, "list-context"), 5*time.Second)
			defer cancel()
			received := make(chan resumeContextObservation, 1)
			release := make(chan struct{})
			cfg := DefaultConfig()
			cfg.Facade = &contextObservingResumeFacade{
				mockFacadeForResumeList: &mockFacadeForResumeList{}, received: received, release: release,
			}
			cmd, cleanup := newRootCommandWithConfig(cfg, false)
			t.Cleanup(cleanup)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{"--storage", t.TempDir(), "--no-color", "resume", "--list"})
			if alreadyCancelled {
				cancel()
			}
			result := make(chan error, 1)
			go func() { result <- cmd.ExecuteContext(ctx) }()
			select {
			case observation := <-received:
				assert.Equal(t, "list-context", observation.value)
				deadline, _ := ctx.Deadline()
				assert.True(t, observation.hasDeadline)
				assert.Equal(t, deadline, observation.deadline)
			case <-time.After(time.Second):
				close(release)
				<-result
				t.Fatal("root command did not invoke history facade")
			}
			cancel()
			select {
			case err := <-result:
				assert.ErrorIs(t, err, context.Canceled)
				var exitErr *exitError
				require.ErrorAs(t, err, &exitErr)
				assert.Equal(t, ExitSystem, exitErr.ExitCode(), "history-query failures retain the existing system exit category")
			case <-time.After(time.Second):
				close(release)
				<-result
				t.Fatal("root resume list did not honor cancellation")
			}
		})
	}
}

func newResumeContextTestDependencies(ctx context.Context, cfg *Config) (*cobra.Command, *ui.OutputWriter, *ui.Formatter) {
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, true, true)
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{NoColor: true})
	return cmd, writer, formatter
}

func assertResumeCancellationExit(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	exitCoder, ok := err.(interface{ ExitCode() int })
	require.True(t, ok, "resume error must preserve the executable exit-code contract")
	assert.Equal(t, ExitExecution, exitCoder.ExitCode())
	handled, ok := err.(interface{ Handled() bool })
	require.True(t, ok, "resume error must preserve the executable handled-error contract")
	assert.True(t, handled.Handled())
	var exitErr *exitError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, ExitExecution, exitErr.code)
}
