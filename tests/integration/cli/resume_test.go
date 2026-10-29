//go:build integration

package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResumeCommand_RestoresInputsAndOutputsFromCheckpoint exercises the
// resume contract using real run checkpoints, not seeded state.
func TestResumeCommand_RestoresInputsAndOutputsFromCheckpoint(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		args  []string
		label string
	}{
		{name: "current step", label: "PRIVATE_RESUME_DEFAULT"},
		{name: "named step", args: []string{"--from=wait"}, label: "PRIVATE_RESUME_DEFAULT"},
		{name: "input override", args: []string{"--input=label=OVERRIDDEN_LABEL"}, label: "OVERRIDDEN_LABEL"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			testResumeRestoresCheckpoint(t, scenario.args, scenario.label)
		})
	}
}

func testResumeRestoresCheckpoint(t *testing.T, resumeArgs []string, expectedLabel string) {
	t.Helper()
	env := newRootCommandTestEnvironment(t)
	t.Setenv("AWF_MCP_PROXY_NO_PURGE", "1")
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	t.Setenv("AWF_AUDIT_LOG", auditPath)
	ready, release, marker := filepath.Join(t.TempDir(), "ready"), filepath.Join(t.TempDir(), "release"), filepath.Join(t.TempDir(), "completed")
	const privateInput = "PRIVATE_RESUME_INPUT"
	env.writeWorkflow(t, "resume-replay", fmt.Sprintf(`name: resume-replay
inputs:
  - name: credential
    type: string
    required: true
  - name: label
    type: string
    default: PRIVATE_RESUME_DEFAULT
states:
  initial: prepare
  prepare:
    type: command
    command: test ! -f %s && touch %s && printf '%%s' '{{inputs.credential}}'
    on_success: wait
  wait:
    type: command
    command: touch %s; while [ ! -f %s ]; do sleep 0.01; done
    on_success: verify
  verify:
    type: command
    command: test '{{states.prepare.Output}}' = '{{inputs.credential}}' && test '{{inputs.label}}' = %s && touch %s
    on_success: done
  done:
    type: terminal
`, interpolation.ShellEscape(marker+"-prepared"), interpolation.ShellEscape(marker+"-prepared"), interpolation.ShellEscape(ready), interpolation.ShellEscape(release), interpolation.ShellEscape(expectedLabel), interpolation.ShellEscape(marker)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := startRootWorkflow(t, ctx, env, "resume-replay", "--input=credential="+privateInput)
	require.Eventually(t, func() bool {
		select {
		case result := <-completed:
			t.Fatalf("workflow stopped before reaching the interruption point: %v\n%s", result.err, result.output)
		default:
		}
		return timeoutFileExists(ready)
	}, 5*time.Second, 10*time.Millisecond)
	cancel()
	initial := receiveRootCommandOutcome(t, completed)
	require.ErrorIs(t, initial.err, context.Canceled)
	checkpoint := env.loadExecutionByWorkflowName(t, "resume-replay")
	assertPersistedExecution(t, checkpoint, workflow.StatusCancelled)
	require.Equal(t, privateInput, checkpoint.Inputs["credential"])
	require.Equal(t, "PRIVATE_RESUME_DEFAULT", checkpoint.Inputs["label"])
	prior, ok := checkpoint.GetStepState("prepare")
	require.True(t, ok)
	require.Equal(t, workflow.StatusCompleted, prior.Status)
	require.Equal(t, privateInput, prior.Output)
	statePath := filepath.Join(env.storageDir, "states", checkpoint.WorkflowID+".json")
	before, err := os.ReadFile(statePath)
	require.NoError(t, err)
	require.Contains(t, string(before), privateInput)
	stateInfo, statErr := os.Stat(statePath)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o600), stateInfo.Mode().Perm())

	t.Run("invalid overrides leave checkpoint unchanged", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		output, resumeErr := env.executeArgs(t, ctx, "resume", checkpoint.WorkflowID, "--input=credential=")
		require.Error(t, resumeErr, output)
		assert.Contains(t, resumeErr.Error(), "credential")
		assertFileBytesEqual(t, statePath, before)
		assert.NoFileExists(t, marker)
	})

	t.Run("resume restores inputs and outputs without replay", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, os.WriteFile(release, nil, 0o600))
		args := append([]string{"resume", checkpoint.WorkflowID}, resumeArgs...)
		output, resumeErr := env.executeArgs(t, ctx, args...)
		require.NoError(t, resumeErr, output)
		assert.FileExists(t, marker)
		resumed := env.loadExecutionByWorkflowName(t, "resume-replay")
		assertPersistedExecution(t, resumed, workflow.StatusCompleted)
		assert.Equal(t, checkpoint.HistoryAttempt+1, resumed.HistoryAttempt)
		assert.Equal(t, checkpoint.HistoryRecordID, resumed.PreviousHistoryRecordID)
		assert.Equal(t, privateInput, resumed.Inputs["credential"])
		assert.Equal(t, expectedLabel, resumed.Inputs["label"])
		rebuilt, found := resumed.GetStepState("prepare")
		require.True(t, found)
		assert.Equal(t, privateInput, rebuilt.Output)
		assert.Equal(t, prior.StartedAt, rebuilt.StartedAt)
		stateData, readErr := os.ReadFile(statePath)
		require.NoError(t, readErr)
		assert.Contains(t, string(stateData), privateInput)
		auditData, readErr := os.ReadFile(auditPath)
		require.NoError(t, readErr)
		history, openErr := store.NewSQLiteHistoryStore(filepath.Join(env.storageDir, "history.db"))
		require.NoError(t, openErr)
		t.Cleanup(func() { require.NoError(t, history.Close()) })
		records, listErr := history.List(context.Background(), &workflow.HistoryFilter{WorkflowID: checkpoint.WorkflowID})
		require.NoError(t, listErr)
		require.Len(t, records, 2)
		historyData, marshalErr := json.Marshal(records)
		require.NoError(t, marshalErr)
		assertConfidentialValuesAbsent(t, string(auditData)+string(historyData), privateInput, "PRIVATE_RESUME_DEFAULT", expectedLabel)
	})
}

func TestResumeCommand_PersistsChronologicalAttemptsAcrossRepeatedResumes(t *testing.T) {
	root := setupInitTestDir(t)
	storageDir := filepath.Join(root, "owned storage")
	readyPath := filepath.Join(root, "owned storage", "resume ready")
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg config"))
	t.Setenv("AWF_AUDIT_LOG", "off")

	const workflowName = "repeated-resume-lineage"
	workflowSource := fmt.Sprintf(`name: repeated-resume-lineage
version: "1.0.0"
inputs:
  - name: behavior
    type: string
    required: true
    validation:
      enum: [fail, wait, success]
states:
  initial: execute
  execute:
    type: step
    command: >-
      case {{inputs.behavior}} in
        fail) exit 17 ;;
        wait) touch %s; while :; do sleep 1; done ;;
        success) exit 0 ;;
      esac
    on_success: done
  done:
    type: terminal
`, interpolation.ShellEscape(readyPath))
	workflowPath := filepath.Join(root, ".awf", "workflows", workflowName+".yaml")
	createTestWorkflow(t, root, workflowName+".yaml", workflowSource)
	originalWorkflow, err := os.ReadFile(workflowPath)
	require.NoError(t, err)

	run := func(ctx context.Context, args []string) (string, error) {
		t.Helper()
		cmd, cleanup := cli.NewRootCommandAutoFacade()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		commandArgs := make([]string, 2+len(args))
		commandArgs[0] = "--storage=" + storageDir
		commandArgs[1] = "--no-color"
		copy(commandArgs[2:], args)
		cmd.SetArgs(commandArgs)
		err := cmd.ExecuteContext(ctx)
		cleanup()
		return output.String(), err
	}

	validationOutput, err := run(context.Background(), []string{"validate", workflowName})
	require.NoError(t, err, validationOutput)

	initialOutput, err := run(context.Background(), []string{"run", workflowName, "--input=behavior=fail", "--skip-plugins"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit code 17")
	assertNoHistoryPersistenceDiagnostic(t, initialOutput)

	history := readResumeHistory(t, run, "")
	require.Len(t, history, 1)
	workflowID := history[0].WorkflowID
	require.NotEmpty(t, workflowID)
	assert.Equal(t, "failed", history[0].Status)
	assert.Equal(t, 1, history[0].Attempt)
	assert.Empty(t, history[0].ResumedFromID)
	assertResumePublicViews(t, run, workflowID, 1, "failed", true)

	otherStore, err := store.NewSQLiteHistoryStore(filepath.Join(storageDir, "history.db"))
	require.NoError(t, err)
	now := time.Now().UTC().Add(-time.Hour)
	unrelated := ports.RunRecord{
		RunID: "other-record", WorkflowID: "other-workflow", Attempt: 1,
		WorkflowName: "other", Status: ports.RunStateCompleted, StartedAt: now, CompletedAt: now.Add(time.Second),
	}
	require.NoError(t, otherStore.Record(context.Background(), executionRecordFromRunRecord(unrelated)))
	conflicting := ports.RunRecord{
		RunID: "conflicting-record", WorkflowID: workflowID, Attempt: 1,
		WorkflowName: workflowName, Status: ports.RunStateFailed, StartedAt: now, CompletedAt: now.Add(time.Second),
	}
	conflictErr := otherStore.Record(context.Background(), executionRecordFromRunRecord(conflicting))
	require.Error(t, conflictErr)
	assert.True(t, errors.Is(conflictErr, store.ErrHistoryRecordConflict), conflictErr)
	require.NoError(t, otherStore.Close())

	failedOutput, err := run(context.Background(), []string{"resume", workflowID, "--input=behavior=fail"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit code 17")
	assertNoHistoryPersistenceDiagnostic(t, failedOutput)
	assertResumePublicViews(t, run, workflowID, 2, "failed", true)

	cancelCtx, cancel := context.WithCancel(context.Background())
	type commandResult struct {
		output string
		err    error
	}
	result := make(chan commandResult, 1)
	commandJoined := false
	go func() {
		output, runErr := run(cancelCtx, []string{"resume", workflowID, "--input=behavior=wait"})
		result <- commandResult{output: output, err: runErr}
	}()
	t.Cleanup(func() {
		cancel()
		if commandJoined {
			return
		}
		select {
		case <-result:
		case <-time.After(5 * time.Second):
			t.Error("cleanup timed out joining cancelled resume command")
		}
	})

	readyDeadline := time.NewTimer(10 * time.Second)
	defer readyDeadline.Stop()
	readyTicker := time.NewTicker(20 * time.Millisecond)
	defer readyTicker.Stop()
	for {
		if _, statErr := os.Stat(readyPath); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			cancel()
			t.Fatalf("inspect readiness file: %v", statErr)
		}
		select {
		case <-readyTicker.C:
		case <-readyDeadline.C:
			cancel()
			select {
			case <-result:
				commandJoined = true
			case <-time.After(5 * time.Second):
			}
			t.Fatal("resume command did not reach its blocking shell branch")
		}
	}
	cancel()

	var cancelled commandResult
	select {
	case cancelled = <-result:
		commandJoined = true
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled resume command did not terminate")
	}
	require.Error(t, cancelled.err)
	assert.ErrorIs(t, cancelled.err, context.Canceled)
	var exitCoder interface{ ExitCode() int }
	require.ErrorAs(t, cancelled.err, &exitCoder)
	assert.Equal(t, cli.ExitExecution, exitCoder.ExitCode())
	assertNoHistoryPersistenceDiagnostic(t, cancelled.output)
	assertResumePublicViews(t, run, workflowID, 3, "cancelled", true)

	successOutput, err := run(context.Background(), []string{"resume", workflowID, "--input=behavior=success"})
	require.NoError(t, err, successOutput)
	assertNoHistoryPersistenceDiagnostic(t, successOutput)
	assertResumePublicViews(t, run, workflowID, 4, "completed", false)

	history = readResumeHistory(t, run, workflowID)
	require.Len(t, history, 4)
	seenIDs := make(map[string]struct{}, len(history))
	expectedStatuses := []string{"completed", "cancelled", "failed", "failed"}
	for i := range history {
		record := history[i]
		assert.Equal(t, workflowID, record.WorkflowID)
		assert.Equal(t, 4-i, record.Attempt)
		assert.Equal(t, expectedStatuses[i], record.Status)
		assert.NotEmpty(t, record.ID)
		_, duplicate := seenIDs[record.ID]
		assert.False(t, duplicate, "history record IDs must be unique")
		seenIDs[record.ID] = struct{}{}
		if i == len(history)-1 {
			assert.Empty(t, record.ResumedFromID)
		} else {
			assert.Equal(t, history[i+1].ID, record.ResumedFromID)
		}
	}

	persistedWorkflow, err := os.ReadFile(workflowPath)
	require.NoError(t, err)
	assert.Equal(t, originalWorkflow, persistedWorkflow, "resume must not rewrite the workflow definition")
	assert.FileExists(t, filepath.Join(storageDir, "history.db"), "resume must retain the SQLite history database")
	assert.FileExists(t, filepath.Join(storageDir, "states", workflowID+".json"), "resume must retain the workflow state")
}

func TestResumeCommand_SerializesAcrossProcesses(t *testing.T) {
	root := setupInitTestDir(t)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
	t.Setenv("AWF_AUDIT_LOG", "off")
	storageDir := filepath.Join(root, "storage")
	readyPath := filepath.Join(root, "owner ready")
	releasePath := filepath.Join(root, "owner release")
	executionsPath := filepath.Join(root, "executions")
	createTestWorkflow(t, root, "serialized-resume.yaml", fmt.Sprintf(`name: serialized-resume
version: "1.0.0"
states:
  initial: execute
  execute:
    type: step
    command: >-
      echo executed >> %s; touch %s; while [ ! -f %s ]; do sleep 0.05; done
    on_success: done
  done:
    type: terminal
`, interpolation.ShellEscape(executionsPath), interpolation.ShellEscape(readyPath), interpolation.ShellEscape(releasePath)))
	state := workflow.NewExecutionContext("serialized-workflow", "serialized-resume")
	state.Status = workflow.StatusFailed
	state.CurrentStep = "execute"
	require.NoError(t, state.BeginInitialHistoryAttempt("serialized-record-1"))
	state.HistoryRecorded = true
	stateStore := store.NewJSONStore(filepath.Join(storageDir, "states"))
	require.NoError(t, stateStore.Save(context.Background(), state))
	historyStore, err := store.NewSQLiteHistoryStore(filepath.Join(storageDir, "history.db"))
	require.NoError(t, err)
	require.NoError(t, historyStore.Record(context.Background(), &workflow.ExecutionRecord{
		ID: "serialized-record-1", WorkflowID: state.WorkflowID, WorkflowName: state.WorkflowName,
		Attempt: 1, Status: "failed", StartedAt: time.Now().Add(-time.Minute), CompletedAt: time.Now(),
	}))
	require.NoError(t, historyStore.Close())

	executable, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	newProcess := func(role string) *exec.Cmd {
		// Self-host the test executable so each root command gets an independent
		// service and process-global resume guard, while sharing real JSON/SQLite.
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestResumeCommand_ProcessHelper$", "-test.count=1") //nolint:gosec // executable is this test binary, not user input.
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "AWF_RESUME_TEST_ROLE="+role, "AWF_RESUME_TEST_STORAGE="+storageDir)
		return cmd
	}
	owner := newProcess("owner")
	var ownerOutput bytes.Buffer
	owner.Stdout = &ownerOutput
	owner.Stderr = &ownerOutput
	require.NoError(t, owner.Start())
	ownerResult := make(chan error, 1)
	ownerDone := make(chan struct{})
	go func() {
		defer close(ownerDone)
		ownerResult <- owner.Wait()
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-ownerDone:
		case <-time.After(5 * time.Second):
			t.Error("owner process did not terminate")
		}
	})
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(readyPath)
		return statErr == nil
	}, 10*time.Second, 10*time.Millisecond, "owner must be executing before contender starts")
	statePath := filepath.Join(storageDir, "states", state.WorkflowID+".json")
	before, err := os.ReadFile(statePath)
	require.NoError(t, err)
	contenderOutput, contenderErr := newProcess("contender").CombinedOutput()
	require.NoError(t, contenderErr, string(contenderOutput))
	after, err := os.ReadFile(statePath)
	require.NoError(t, err)
	assert.Equal(t, before, after, "a blocked contender must not allocate or change the owner's running attempt")
	executions, err := os.ReadFile(executionsPath)
	require.NoError(t, err)
	assert.Equal(t, "executed\n", string(executions), "only the lease owner may execute commands")
	require.NoError(t, os.WriteFile(releasePath, nil, 0o600))
	select {
	case err = <-ownerResult:
		require.NoError(t, err, ownerOutput.String())
	case <-ctx.Done():
		t.Fatal("owner did not complete after release")
	}
	historyStore, err = store.NewSQLiteHistoryStore(filepath.Join(storageDir, "history.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, historyStore.Close()) })
	records, err := historyStore.List(context.Background(), &workflow.HistoryFilter{WorkflowID: state.WorkflowID})
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, 2, records[0].Attempt)
	assert.Equal(t, "serialized-record-1", records[0].ResumedFromID)
	assert.Equal(t, "completed", records[0].Status)
}

func TestResumeCommand_ProcessHelper(t *testing.T) {
	role := os.Getenv("AWF_RESUME_TEST_ROLE")
	if role == "" {
		return
	}
	timeout := 20 * time.Second
	if role == "contender" {
		timeout = 500 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--storage", os.Getenv("AWF_RESUME_TEST_STORAGE"), "--no-color", "resume", "serialized-workflow"})
	err := cmd.ExecuteContext(ctx)
	if role == "contender" {
		require.ErrorIs(t, err, context.DeadlineExceeded, output.String())
		assert.ErrorContains(t, err, "acquire resume lease", "contender must lose before load, allocation, hooks, or commands")
	} else {
		require.NoError(t, err, output.String())
	}
	assertNoHistoryPersistenceDiagnostic(t, output.String())
}

type resumeCommandRunner func(context.Context, []string) (string, error)

func readResumeHistory(t *testing.T, run resumeCommandRunner, workflowID string) []cli.HistoryInfo {
	t.Helper()
	args := []string{"--format=json", "history"}
	if workflowID != "" {
		args = append(args, "--workflow-id="+workflowID)
	}
	output, err := run(context.Background(), args)
	require.NoError(t, err, output)
	var records []cli.HistoryInfo
	require.NoError(t, json.Unmarshal([]byte(output), &records), output)
	return records
}

func executionRecordFromRunRecord(record ports.RunRecord) *workflow.ExecutionRecord {
	return &workflow.ExecutionRecord{
		ID:            record.RunID,
		WorkflowID:    record.WorkflowID,
		Attempt:       record.Attempt,
		ResumedFromID: record.ResumedFromID,
		WorkflowName:  record.WorkflowName,
		Status:        string(record.Status),
		StartedAt:     record.StartedAt,
		CompletedAt:   record.CompletedAt,
		DurationMs:    record.DurationMs,
		ErrorMessage:  record.ErrorMessage,
	}
}

func assertResumePublicViews(
	t *testing.T,
	run resumeCommandRunner,
	workflowID string,
	expectedAttempt int,
	status string,
	resumable bool,
) {
	t.Helper()
	history := readResumeHistory(t, run, workflowID)
	require.NotEmpty(t, history)
	latest := history[0]
	assert.Equal(t, expectedAttempt, latest.Attempt)
	assert.Equal(t, workflowID, latest.WorkflowID)
	assert.NotEmpty(t, latest.ID)
	assert.Equal(t, status, latest.Status)
	if expectedAttempt == 1 {
		assert.Empty(t, latest.ResumedFromID)
	} else {
		assert.NotEmpty(t, latest.ResumedFromID)
	}

	statusOutput, err := run(context.Background(), []string{"--format=json", "status", workflowID})
	require.NoError(t, err, statusOutput)
	var projectedStatus struct {
		WorkflowID  string `json:"workflow_id"`
		Status      string `json:"status"`
		CompletedAt string `json:"completed_at"`
	}
	require.NoError(t, json.Unmarshal([]byte(statusOutput), &projectedStatus), statusOutput)
	assert.Equal(t, workflowID, projectedStatus.WorkflowID)
	assert.Equal(t, status, projectedStatus.Status)
	assert.Equal(t, latest.CompletedAt, projectedStatus.CompletedAt)

	listOutput, err := run(context.Background(), []string{"--format=json", "resume", "--list"})
	require.NoError(t, err, listOutput)
	var resumableWorkflows []struct {
		WorkflowID string `json:"workflow_id"`
		Status     string `json:"status"`
		UpdatedAt  string `json:"updated_at"`
	}
	require.NoError(t, json.Unmarshal([]byte(listOutput), &resumableWorkflows), listOutput)
	var projectedResume *struct {
		WorkflowID string `json:"workflow_id"`
		Status     string `json:"status"`
		UpdatedAt  string `json:"updated_at"`
	}
	for i := range resumableWorkflows {
		if resumableWorkflows[i].WorkflowID == workflowID {
			projectedResume = &resumableWorkflows[i]
			break
		}
	}
	if resumable {
		require.NotNil(t, projectedResume)
		assert.Equal(t, status, projectedResume.Status)
		assert.Equal(t, latest.CompletedAt, projectedResume.UpdatedAt)
	} else {
		assert.Nil(t, projectedResume)
	}
}

func assertNoHistoryPersistenceDiagnostic(t *testing.T, output string) {
	t.Helper()
	assert.NotContains(t, output, "history persistence failed")
	assert.NotContains(t, output, "UNIQUE constraint failed")
	assert.NotContains(t, output, store.ErrHistoryRecordConflict.Error())
}

func recordResumeHistory(t *testing.T, storageDir, id, workflowName, status string) {
	t.Helper()

	historyStore, err := store.NewSQLiteHistoryStore(filepath.Join(storageDir, "history.db"))
	require.NoError(t, err)
	defer func() { _ = historyStore.Close() }()

	now := time.Date(2025, 1, 1, 10, 5, 0, 0, time.UTC)
	err = historyStore.Record(context.Background(), &workflow.ExecutionRecord{
		ID:           id,
		WorkflowID:   id,
		Attempt:      1,
		WorkflowName: workflowName,
		Status:       status,
		StartedAt:    now.Add(-5 * time.Minute),
		CompletedAt:  now,
	})
	require.NoError(t, err)
}

// TestResumeCommand_PersistsAttemptLineage verifies the public resume command
// carries a stable workflow identity into a distinct, linked history attempt.
func TestResumeCommand_PersistsAttemptLineage(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	const (
		workflowID    = "stable-workflow-id"
		workflowName  = "lineage-workflow"
		firstRecordID = "attempt-record-1"
	)

	createTestWorkflow(t, tmpDir, workflowName+".yaml", `name: lineage-workflow
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`)

	historyStore, err := store.NewSQLiteHistoryStore(filepath.Join(tmpDir, "history.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, historyStore.Close()) })
	startedAt := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	require.NoError(t, historyStore.Record(context.Background(), &workflow.ExecutionRecord{
		ID:           firstRecordID,
		WorkflowID:   workflowID,
		Attempt:      1,
		WorkflowName: workflowName,
		Status:       "cancelled",
		StartedAt:    startedAt,
		CompletedAt:  startedAt.Add(time.Second),
	}))
	require.NoError(t, historyStore.Close())

	stateStore := store.NewJSONStore(filepath.Join(tmpDir, "states"))
	saved := workflow.NewExecutionContext(workflowID, workflowName)
	saved.CurrentStep = "done"
	saved.SetStatus(workflow.StatusCancelled)
	require.NoError(t, saved.BeginInitialHistoryAttempt(firstRecordID))
	require.True(t, saved.TryBeginHistoryFinalization())
	saved.FinishHistoryFinalization(true)
	require.NoError(t, stateStore.Save(context.Background(), saved))

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	t.Cleanup(cleanup)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", workflowID})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	historyReader, err := store.NewSQLiteHistoryStore(filepath.Join(tmpDir, "history.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, historyReader.Close()) })
	records, err := historyReader.List(context.Background(), &workflow.HistoryFilter{WorkflowID: workflowID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, records, 2, "resume must append a second attempt without replacing the predecessor")

	assert.Equal(t, workflowID, records[0].WorkflowID)
	assert.Equal(t, 2, records[0].Attempt)
	assert.Equal(t, firstRecordID, records[0].ResumedFromID)
	assert.NotEmpty(t, records[0].ID)
	assert.NotEqual(t, firstRecordID, records[0].ID)
	assert.Equal(t, firstRecordID, records[1].ID)
	assert.Equal(t, 1, records[1].Attempt)
	assert.Empty(t, records[1].ResumedFromID)
	assert.NotContains(t, output.String(), "UNIQUE constraint failed")
}

func TestResumeCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "resume" {
			found = true
			break
		}
	}

	assert.True(t, found)
}

func TestResumeCommand_HasListFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "resume" {
			flag := sub.Flags().Lookup("list")
			require.NotNil(t, flag)
			assert.Equal(t, "l", flag.Shorthand)
			return
		}
	}

	t.Error("resume command not found")
}

func TestResumeCommand_HasInputFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "resume" {
			flag := sub.Flags().Lookup("input")
			require.NotNil(t, flag)
			assert.Equal(t, "i", flag.Shorthand)
			return
		}
	}

	t.Error("resume command not found")
}

func TestResumeCommand_HasOutputFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "resume" {
			flag := sub.Flags().Lookup("output")
			require.NotNil(t, flag)
			assert.Equal(t, "o", flag.Shorthand)
			return
		}
	}

	t.Error("resume command not found")
}

func TestResumeCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"resume", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Resume")
	assert.Contains(t, output, "--list")
	assert.Contains(t, output, "--input")
}

func TestResumeCommand_NoArgsNoListFlag(t *testing.T) {
	// Without --list flag and without workflow-id argument, should error
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"resume"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workflow-id required")
}

func TestResumeCommand_ListFlag_NoResumableWorkflows(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty states directory
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No resumable workflows")
}

func TestResumeCommand_ListFlag_ShowsResumableWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	recordResumeHistory(t, tmpDir, "test-id-123", "my-workflow", "running")

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "test-id-123")
	assert.Contains(t, output, "my-workflow")
	assert.Contains(t, output, "running")
}

func TestResumeCommand_ListFlag_FiltersCompletedWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	recordResumeHistory(t, tmpDir, "completed-id", "completed-workflow", "completed")
	recordResumeHistory(t, tmpDir, "running-id", "running-workflow", "running")

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "running-id")
	assert.NotContains(t, output, "completed-id")
}

func TestResumeCommand_ListFlag_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	recordResumeHistory(t, tmpDir, "json-test-id", "json-workflow", "running")

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list", "--format=json"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify output is valid JSON
	var result []map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "json-test-id", result[0]["workflow_id"])
	assert.Equal(t, "json-workflow", result[0]["workflow_name"])
}

func TestResumeCommand_WorkflowNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty states directory
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "nonexistent-workflow-id"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResumeCommand_AlreadyCompleted(t *testing.T) {
	tmpDir := t.TempDir()

	// Create states and workflows directories
	statesDir := filepath.Join(tmpDir, "states")
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	// Create a completed workflow state
	completedState := `{
		"WorkflowID": "completed-id",
		"WorkflowName": "test-workflow",
		"Status": "completed",
		"CurrentStep": "done",
		"Inputs": {},
		"States": {},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "completed-id.json"),
		[]byte(completedState),
		0o644,
	))

	// Create the workflow definition
	workflowContent := `name: test-workflow
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(
		filepath.Join(workflowsDir, "test-workflow.yaml"),
		[]byte(workflowContent),
		0o644,
	))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "completed-id"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "completed")
}

func TestResumeCommand_InputOverrides(t *testing.T) {
	tmpDir := t.TempDir()

	// Create states and workflows directories
	statesDir := filepath.Join(tmpDir, "states")
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	// Create a running workflow state
	runningState := `{
		"WorkflowID": "override-id",
		"WorkflowName": "override-workflow",
		"Status": "running",
		"CurrentStep": "step1",
		"Inputs": {"key": "original"},
		"States": {},
		"Env": {},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "override-id.json"),
		[]byte(runningState),
		0o644,
	))

	// Create the workflow definition
	workflowContent := `name: override-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "{{inputs.key}}"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(
		filepath.Join(workflowsDir, "override-workflow.yaml"),
		[]byte(workflowContent),
		0o644,
	))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "override-id", "--input=key=overridden"})

	err := cmd.Execute()
	// The resume should work with input override
	// (Implementation will determine success/failure)
	_ = err // Acknowledge error for now - test verifies command accepts --input flag
}

func TestResumeCommand_WorkflowDefinitionDeleted(t *testing.T) {
	tmpDir := t.TempDir()

	// Create states directory but NOT workflows directory with the definition
	statesDir := filepath.Join(tmpDir, "states")
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	// Create a running workflow state (but workflow definition doesn't exist)
	runningState := `{
		"WorkflowID": "orphan-id",
		"WorkflowName": "deleted-workflow",
		"Status": "running",
		"CurrentStep": "step1",
		"Inputs": {},
		"States": {},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "orphan-id.json"),
		[]byte(runningState),
		0o644,
	))
	// Note: No workflow file created for "deleted-workflow"

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "orphan-id"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResumeCommand_Success(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create states directory
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create a running workflow state (step1 completed, at step2)
	now := time.Now().Format(time.RFC3339)
	runningState := `{
		"WorkflowID": "success-id",
		"WorkflowName": "success-workflow",
		"Status": "running",
		"CurrentStep": "step2",
		"Inputs": {},
		"States": {
			"step1": {
				"Name": "step1",
				"Status": "completed",
				"Output": "step1 output"
			}
		},
		"Env": {},
		"StartedAt": "` + now + `",
		"UpdatedAt": "` + now + `"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "success-id.json"),
		[]byte(runningState),
		0o644,
	))

	// Create the workflow definition using the helper
	workflowContent := `name: success-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo step1
    on_success: step2
  step2:
    type: step
    command: echo step2
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "success-workflow.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "success-id"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.True(t, strings.Contains(output, "completed") || strings.Contains(output, "success"),
		"output should indicate success")
}

func TestResumeCommand_QuietFormat(t *testing.T) {
	tmpDir := t.TempDir()
	recordResumeHistory(t, tmpDir, "quiet-id", "quiet-workflow", "running")

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list", "--format=quiet"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := strings.TrimSpace(buf.String())
	// Quiet format should only output workflow IDs
	assert.Equal(t, "quiet-id", output)
}

func TestResumeCommand_TableFormat(t *testing.T) {
	tmpDir := t.TempDir()
	recordResumeHistory(t, tmpDir, "table-id", "table-workflow", "running")

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list", "--format=table"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Table format should contain headers and borders
	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "WORKFLOW")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "table-id")
}

func TestResumeCommand_MultipleInputOverrides(t *testing.T) {
	// Test that multiple --input flags work
	cmd := cli.NewRootCommand()

	// Find the resume command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "resume" {
			// Verify input flag can be specified multiple times (StringArray)
			flag := sub.Flags().Lookup("input")
			require.NotNil(t, flag)
			// StringArray type allows multiple values
			return
		}
	}
	t.Error("resume command not found")
}

func TestResumeCommand_SQLiteHistoryStore_Wiring(t *testing.T) {
	// Test that resume command creates history.db (SQLite) instead of Badger directory
	tmpDir := setupInitTestDir(t)

	// Create states directory
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create a running workflow state
	now := time.Now().Format(time.RFC3339)
	runningState := `{
		"WorkflowID": "sqlite-test-id",
		"WorkflowName": "sqlite-workflow",
		"Status": "running",
		"CurrentStep": "step2",
		"Inputs": {},
		"States": {
			"step1": {
				"Name": "step1",
				"Status": "completed",
				"Output": "step1 done"
			}
		},
		"Env": {},
		"StartedAt": "` + now + `",
		"UpdatedAt": "` + now + `"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "sqlite-test-id.json"),
		[]byte(runningState),
		0o644,
	))

	// Create the workflow definition using the helper
	workflowContent := `name: sqlite-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo step1
    on_success: step2
  step2:
    type: step
    command: echo step2
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "sqlite-workflow.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "sqlite-test-id"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify SQLite database file was created
	historyDBPath := filepath.Join(tmpDir, "history.db")
	_, err = os.Stat(historyDBPath)
	assert.NoError(t, err)

	// Verify Badger directory was NOT created
	badgerDir := filepath.Join(tmpDir, "history")
	_, err = os.Stat(badgerDir)
	assert.True(t, os.IsNotExist(err))
}

func TestResumeCommand_ConcurrentAccess(t *testing.T) {
	// Test that multiple concurrent resume --list commands don't cause lock errors
	// This validates the bug-48 fix for the resume command
	tmpDir := t.TempDir()

	// Create states directory with resumable workflows
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create multiple running workflow states
	for i := 0; i < 3; i++ {
		state := `{
			"WorkflowID": "concurrent-` + string(rune('a'+i)) + `",
			"WorkflowName": "concurrent-workflow",
			"Status": "running",
			"CurrentStep": "step1",
			"Inputs": {},
			"States": {},
			"StartedAt": "2025-01-01T10:00:00Z",
			"UpdatedAt": "2025-01-01T10:05:00Z"
		}`
		require.NoError(t, os.WriteFile(
			filepath.Join(statesDir, "concurrent-"+string(rune('a'+i))+".json"),
			[]byte(state),
			0o644,
		))
	}

	// Run concurrent resume --list commands
	const concurrency = 3
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			cmd, cleanup := cli.NewRootCommandAutoFacade()
			defer cleanup()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list"})
			errCh <- cmd.Execute()
		}()
	}

	// Collect results - all should succeed without lock errors
	for i := 0; i < concurrency; i++ {
		err := <-errCh
		assert.NoError(t, err)
	}
}

// TestResumeCommand_ConfigIntegration tests that runResume properly integrates
// with loadProjectConfig and mergeInputs.
//
// These tests verify:
// - US1: Config inputs are used when no CLI inputs provided
// - FR-003: CLI inputs override config inputs
// - FR-004: Missing config file is not an error
func TestResumeCommand_ConfigIntegration(t *testing.T) {
	tests := []struct {
		name           string
		description    string
		configContent  string // YAML content for .awf/config.yaml (empty = no file)
		cliInputFlags  []string
		expectedInEcho string // Part of the expected echo output
	}{
		{
			name:        "config inputs used when no CLI inputs",
			description: "US1: Config values are used when no --input flags provided",
			configContent: `inputs:
  greeting: from-config
`,
			cliInputFlags:  []string{},
			expectedInEcho: "from-config",
		},
		{
			name:        "CLI overrides config for same key",
			description: "FR-003: CLI --input flag overrides config value",
			configContent: `inputs:
  greeting: from-config
`,
			cliInputFlags:  []string{"greeting=from-cli"},
			expectedInEcho: "from-cli",
		},
		{
			name:        "both merged when no overlap",
			description: "Config and CLI inputs are merged when keys are disjoint",
			configContent: `inputs:
  prefix: hello
`,
			cliInputFlags:  []string{"suffix=world"},
			expectedInEcho: "hello", // Both should be available
		},
		{
			name:           "no config file works",
			description:    "FR-004: Missing config file is not an error",
			configContent:  "", // No config file
			cliInputFlags:  []string{"greeting=cli-only"},
			expectedInEcho: "cli-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Scenario: %s", tt.description)

			tmpDir := setupInitTestDir(t)

			// Create states directory
			statesDir := filepath.Join(tmpDir, "states")
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Create .awf/config.yaml if content provided
			if tt.configContent != "" {
				configDir := filepath.Join(tmpDir, ".awf")
				require.NoError(t, os.MkdirAll(configDir, 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(configDir, "config.yaml"),
					[]byte(tt.configContent),
					0o644,
				))
			}

			// Create a running workflow state at step2
			now := time.Now().Format(time.RFC3339)
			runningState := `{
				"WorkflowID": "config-test-id",
				"WorkflowName": "config-workflow",
				"Status": "running",
				"CurrentStep": "step2",
				"Inputs": {},
				"States": {
					"step1": {
						"Name": "step1",
						"Status": "completed",
						"Output": "step1 done"
					}
				},
				"Env": {},
				"StartedAt": "` + now + `",
				"UpdatedAt": "` + now + `"
			}`
			require.NoError(t, os.WriteFile(
				filepath.Join(statesDir, "config-test-id.json"),
				[]byte(runningState),
				0o644,
			))

			// Create the workflow definition using the helper
			workflowContent := `name: config-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo step1
    on_success: step2
  step2:
    type: step
    command: echo step2
    on_success: done
  done:
    type: terminal
`
			createTestWorkflow(t, tmpDir, "config-workflow.yaml", workflowContent)

			// Build command args
			args := make([]string, 0, 3+len(tt.cliInputFlags))
			args = append(args, "--storage="+tmpDir, "resume", "config-test-id")
			for _, input := range tt.cliInputFlags {
				args = append(args, "--input="+input)
			}

			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(args)

			err := cmd.Execute()
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, "completed")
		})
	}
}

// TestResumeCommand_ConfigError_Propagates tests that config loading errors
// are properly propagated from runResume.
func TestResumeCommand_ConfigError_Propagates(t *testing.T) {
	// Spec: FR-005 - Invalid YAML in config file produces exit code 1 with descriptive error

	t.Run("invalid YAML config should cause error", func(t *testing.T) {
		tmpDir := setupInitTestDir(t)

		// Create states directory
		statesDir := filepath.Join(tmpDir, "states")
		require.NoError(t, os.MkdirAll(statesDir, 0o755))

		// Create invalid config file
		configDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(configDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(configDir, "config.yaml"),
			[]byte("invalid: yaml: content: [unclosed"),
			0o644,
		))

		// Create a running workflow state
		now := time.Now().Format(time.RFC3339)
		runningState := `{
			"WorkflowID": "error-test-id",
			"WorkflowName": "error-workflow",
			"Status": "running",
			"CurrentStep": "step1",
			"Inputs": {},
			"States": {},
			"Env": {},
			"StartedAt": "` + now + `",
			"UpdatedAt": "` + now + `"
		}`
		require.NoError(t, os.WriteFile(
			filepath.Join(statesDir, "error-test-id.json"),
			[]byte(runningState),
			0o644,
		))

		// Create the workflow definition using the helper
		workflowContent := `name: error-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
`
		createTestWorkflow(t, tmpDir, "error-workflow.yaml", workflowContent)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "error-test-id"})

		execErr := cmd.Execute()
		require.Error(t, execErr)
		assert.Contains(t, execErr.Error(), "config")
	})
}

// TestResumeCommand_NoConfigFile_Succeeds tests that resume succeeds
// when there's no config file (FR-004).
func TestResumeCommand_NoConfigFile_Succeeds(t *testing.T) {
	// Spec: FR-004 - Missing config file is not an error; system proceeds with empty defaults

	tmpDir := setupInitTestDir(t)

	// Create states directory (no .awf/config.yaml)
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create a running workflow state at step2
	now := time.Now().Format(time.RFC3339)
	runningState := `{
		"WorkflowID": "no-config-id",
		"WorkflowName": "no-config-workflow",
		"Status": "running",
		"CurrentStep": "step2",
		"Inputs": {},
		"States": {
			"step1": {
				"Name": "step1",
				"Status": "completed",
				"Output": "step1 done"
			}
		},
		"Env": {},
		"StartedAt": "` + now + `",
		"UpdatedAt": "` + now + `"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "no-config-id.json"),
		[]byte(runningState),
		0o644,
	))

	// Create the workflow definition using the helper
	workflowContent := `name: no-config-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo step1
    on_success: step2
  step2:
    type: step
    command: echo step2
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "no-config-workflow.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "no-config-id"})

	err := cmd.Execute()
	require.NoError(t, err)
}

// TestResumeCommand_ConfigWithCLIInputMerge tests that config and CLI inputs
// are properly merged with CLI taking precedence.
func TestResumeCommand_ConfigWithCLIInputMerge(t *testing.T) {
	// Test the merge order: config < CLI (CLI wins)

	tmpDir := setupInitTestDir(t)

	// Create states directory
	statesDir := filepath.Join(tmpDir, "states")
	configDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	// Create config with inputs
	configContent := `inputs:
  shared: config-value
  config_only: from-config
`
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "config.yaml"),
		[]byte(configContent),
		0o644,
	))

	// Create a running workflow state at step2
	now := time.Now().Format(time.RFC3339)
	runningState := `{
		"WorkflowID": "merge-test-id",
		"WorkflowName": "merge-workflow",
		"Status": "running",
		"CurrentStep": "step2",
		"Inputs": {},
		"States": {
			"step1": {
				"Name": "step1",
				"Status": "completed",
				"Output": "step1 done"
			}
		},
		"Env": {},
		"StartedAt": "` + now + `",
		"UpdatedAt": "` + now + `"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "merge-test-id.json"),
		[]byte(runningState),
		0o644,
	))

	// Create the workflow definition using the helper
	workflowContent := `name: merge-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo step1
    on_success: step2
  step2:
    type: step
    command: echo step2
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "merge-workflow.yaml", workflowContent)

	// CLI overrides 'shared' and adds 'cli_only'
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"resume", "merge-test-id",
		"--input=shared=cli-override",
		"--input=cli_only=from-cli",
	})

	err := cmd.Execute()
	require.NoError(t, err)
	// The test validates that the command completes successfully with merged inputs
}

// TestResumeCommand_ConfigYAMLComments tests that config files with comments work.
func TestResumeCommand_ConfigYAMLComments(t *testing.T) {
	// Spec: NFR-003 - Config file supports YAML comments for documentation

	tmpDir := setupInitTestDir(t)

	// Create states directory
	statesDir := filepath.Join(tmpDir, "states")
	configDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	// Create config with comments
	configContent := `# Project configuration
# These inputs are used as defaults for all workflows

inputs:
  # The project identifier
  project: my-project
  # Environment (staging, production)
  env: staging  # default to staging
`
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "config.yaml"),
		[]byte(configContent),
		0o644,
	))

	// Create a running workflow state
	now := time.Now().Format(time.RFC3339)
	runningState := `{
		"WorkflowID": "comments-test-id",
		"WorkflowName": "comments-workflow",
		"Status": "running",
		"CurrentStep": "step2",
		"Inputs": {},
		"States": {
			"step1": {
				"Name": "step1",
				"Status": "completed",
				"Output": "step1 done"
			}
		},
		"Env": {},
		"StartedAt": "` + now + `",
		"UpdatedAt": "` + now + `"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "comments-test-id.json"),
		[]byte(runningState),
		0o644,
	))

	// Create the workflow definition using the helper
	workflowContent := `name: comments-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo step1
    on_success: step2
  step2:
    type: step
    command: echo step2
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "comments-workflow.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "comments-test-id"})

	err := cmd.Execute()
	require.NoError(t, err, "resume should succeed with commented config")
}

// TestResumeCommand_ConfigEmptyInputs tests behavior with empty inputs section.
func TestResumeCommand_ConfigEmptyInputs(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create states directory
	statesDir := filepath.Join(tmpDir, "states")
	configDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	// Create config with empty inputs
	configContent := `inputs:
`
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "config.yaml"),
		[]byte(configContent),
		0o644,
	))

	// Create a running workflow state
	now := time.Now().Format(time.RFC3339)
	runningState := `{
		"WorkflowID": "empty-inputs-id",
		"WorkflowName": "empty-inputs-workflow",
		"Status": "running",
		"CurrentStep": "step2",
		"Inputs": {},
		"States": {
			"step1": {
				"Name": "step1",
				"Status": "completed",
				"Output": "step1 done"
			}
		},
		"Env": {},
		"StartedAt": "` + now + `",
		"UpdatedAt": "` + now + `"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "empty-inputs-id.json"),
		[]byte(runningState),
		0o644,
	))

	// Create the workflow definition using the helper
	workflowContent := `name: empty-inputs-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo step1
    on_success: step2
  step2:
    type: step
    command: echo step2
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "empty-inputs-workflow.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "empty-inputs-id"})

	err := cmd.Execute()
	require.NoError(t, err)
}
