package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/transcript"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
)

type confidentialitySlowCleanupExecutor struct{ started, release, finished chan struct{} }

func (e *confidentialitySlowCleanupExecutor) Execute(ctx context.Context, _ *ports.Command) (*ports.CommandResult, error) {
	close(e.started)
	<-ctx.Done()
	<-e.release
	close(e.finished)
	return nil, ctx.Err()
}

func TestAPITerminalWaitsForCleanup(t *testing.T) {
	exec := &confidentialitySlowCleanupExecutor{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	var once sync.Once
	release := func() { once.Do(func() { close(exec.release) }) }
	defer release()
	repo := testmocks.NewMockWorkflowRepository()
	repo.AddWorkflow("test-workflow", &workflow.Workflow{Name: "test-workflow", Initial: "work", Steps: map[string]*workflow.Step{
		"work": {Name: "work", Type: workflow.StepTypeCommand, Command: "controlled cleanup", OnSuccess: "done"},
		"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
	}})
	ws := application.NewWorkflowService(repo, nil, nil, nil, nil)
	es := application.NewExecutionService(ws, exec, nil, testmocks.NewMockStateStore(), testmocks.NewMockLogger(), interpolation.NewTemplateResolver(), nil)
	adapter := application.NewAdapter(ws, es, &application.HistoryService{}, application.NewResolver(nil, repo), transcript.NewNopRecorder(), application.NewSessionRegistry())
	api, _, sessions := newFacadeExecutionHandlerAPI(t, adapter)
	response := api.Post("/api/workflows/local/test-workflow/run", map[string]any{"inputs": map[string]any{}})
	var run struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
		} `json:"body"`
	}
	if err := json.NewDecoder(response.Body).Decode(&run); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exec.started:
	case <-time.After(2 * time.Second):
		t.Fatal("work never started")
	}
	session, ok := sessions.Get(run.Body.ExecutionID)
	if !ok {
		t.Fatal("session missing")
	}
	api.Delete("/api/executions/"+run.Body.ExecutionID, struct {
		ID string `path:"id"`
	}{ID: run.Body.ExecutionID})
	if !errors.Is(session.Err(), context.Canceled) {
		t.Errorf("T034: closed cancellation session lost context.Canceled sentinel: %v", session.Err())
	}
	closed := false
	terminalCount := 0
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for !closed {
		select {
		case event, open := <-session.Events():
			if open && (event.Kind == ports.EventWorkflowFailed || event.Kind == ports.EventWorkflowCompleted) {
				terminalCount++
				select {
				case <-exec.finished:
				default:
					t.Error("terminal event preceded executor cleanup")
				}
			}
			if !open {
				closed = true
			}
		case <-time.After(50 * time.Millisecond):
			release()
		case <-deadline.C:
			t.Fatal("session did not close after cleanup")
		}
	}
	if terminalCount != 1 {
		t.Errorf("expected exactly one terminal event, got %d", terminalCount)
	}
	if !errors.Is(session.Err(), context.Canceled) {
		t.Errorf("terminal error lost cancellation: %v", session.Err())
	}
	select {
	case <-exec.finished:
	default:
		t.Error("T034: terminal event stream closed while real executor cleanup is still blocked")
	}
	release()
	select {
	case <-exec.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup never finished")
	}
}

func TestSSETimeoutDoesNotLeaveActiveStream(t *testing.T) {
	session := newMockRunSession("blocked-sse")
	session.events <- ports.Event{Kind: ports.EventInputRequired, RunID: session.ID(), Payload: &ports.EnrichedInputRequest{Prompt: strings.Repeat("x", 16<<20)}}
	lookup := newMockSessionLookup()
	lookup.add(session)
	server := NewServer(NewBridge(), ":0", WithSessionRegistry(lookup), WithShutdownTimeout(25*time.Millisecond))
	httpTest := httptest.NewUnstartedServer(server.Handler())
	server.httpSrv = httpTest.Config
	httpTest.Start()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, httpTest.URL+"/api/executions/blocked-sse/events", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpTest.Client().Do(req)
	if err != nil {
		httpTest.Close()
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close(); httpTest.CloseClientConnections(); httpTest.Close() }()
	err = server.Shutdown(context.Background())
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
	drained := make(chan struct{})
	go func() { server.sseWG.Wait(); close(drained) }()
	select {
	case <-drained:
	case <-time.After(50 * time.Millisecond):
		t.Errorf("T038: Shutdown returned %v while the SSE writer/connection remained active", err)
	}
}
