package workflow

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPersistenceSnapshotRetainsResumeDataWithoutMutatingLiveContext(t *testing.T) {
	const canary = "PRIVATE_BUSINESS_DATA"
	live := NewExecutionContext("run-id", "workflow-name")
	live.Inputs["ordinary-key"] = canary
	live.Env["ordinary-key"] = canary
	live.CurrentLoop = &LoopContext{Item: canary, Parent: &LoopContext{Item: canary}}
	live.CallStack = []string{"parent", "workflow-name"}
	live.SetStepState("work", StepState{Name: "work", Status: StatusCompleted, Error: canary, Output: canary, Stderr: canary, Response: map[string]any{"value": canary}, JSON: canary, Data: map[string]any{"value": canary}, Conversation: &ConversationState{SessionID: canary}, DisplayOutput: canary, OutputPath: canary, StderrPath: canary})
	require.NoError(t, live.BeginInitialHistoryAttempt("attempt-id"))
	snapshot := live.PersistenceSnapshot()
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.Contains(t, string(data), canary)
	var restored ExecutionContext
	require.NoError(t, json.Unmarshal(data, &restored))
	require.Equal(t, live.Inputs, restored.Inputs)
	require.Equal(t, live.Env, restored.Env)
	require.Equal(t, live.CurrentLoop, restored.CurrentLoop)
	require.Equal(t, live.CallStack, restored.CallStack)
	state := restored.States["work"]
	require.Equal(t, canary, state.Output)
	require.Equal(t, canary, state.Stderr)
	require.Equal(t, live.States["work"].Response, state.Response)
	require.Equal(t, live.States["work"].JSON, state.JSON)
	require.Equal(t, live.States["work"].Data, state.Data)
	require.Equal(t, live.States["work"].Conversation, state.Conversation)
	require.Equal(t, canary, state.OutputPath)
	require.Equal(t, canary, state.StderrPath)
	require.Empty(t, state.DisplayOutput)
	require.Equal(t, "step execution failed", state.Error)
	require.Equal(t, canary, live.States["work"].Error)
	require.Equal(t, "attempt-id", snapshot.HistoryRecordID)
	require.Equal(t, StatusCompleted, snapshot.States["work"].Status)
	require.Equal(t, canary, live.Inputs["ordinary-key"])
	require.Equal(t, canary, live.States["work"].Output)
	snapshot.Inputs["ordinary-key"] = "changed"
	snapshot.Env["ordinary-key"] = "changed"
	snapshot.CallStack[0] = "changed"
	require.Equal(t, canary, live.Inputs["ordinary-key"])
	require.Equal(t, canary, live.Env["ordinary-key"])
	require.Equal(t, "parent", live.CallStack[0])
	snapshot.States["other"] = StepState{}
	require.NotContains(t, live.States, "other")
}
