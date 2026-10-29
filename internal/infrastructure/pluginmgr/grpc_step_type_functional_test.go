package pluginmgr

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	"github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/pkg/interpolation"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// TestExecutionService_CustomStep_PropagatesCancellationThroughGRPCAdapter exercises
// a workflow through the public application entry point with the concrete gRPC adapter.
// The server blocks until the caller cancels, making cancellation observable at the
// workflow boundary rather than only through an adapter mock.
func TestExecutionService_CustomStep_PropagatesCancellationThroughGRPCAdapter(t *testing.T) {
	service := &cancellationBlockingStepTypeService{
		executeStarted:    make(chan struct{}, 1),
		executionDeadline: make(chan bool, 1),
	}
	client := newFunctionalStepTypeClient(t, service)

	adapter := newGRPCStepTypeAdapter(client, "functional-plugin", time.Second, logger.NopLogger{})
	require.NoError(t, adapter.listAndCache(context.Background(), map[string]bool{}))

	stateStore := store.NewJSONStore(t.TempDir())
	commandExecutor := executor.NewShellExecutor()
	workflowService := application.NewWorkflowService(
		repository.NewYAMLRepository(t.TempDir()),
		stateStore,
		commandExecutor,
		logger.NopLogger{},
		nil,
	)
	executionService := application.NewExecutionService(
		workflowService,
		commandExecutor,
		nil,
		stateStore,
		logger.NopLogger{},
		interpolation.NewTemplateResolver(),
		nil,
	)
	executionService.SetStepTypeProvider(&compositeStepTypeProvider{adapters: []*grpcStepTypeAdapter{adapter}})

	callerCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan error, 1)
	var executionContext *workflow.ExecutionContext
	go func() {
		var err error
		executionContext, err = executionService.RunWithWorkflowAndRunID(
			callerCtx,
			functionalCancellationWorkflow(),
			nil,
			"grpc-custom-step-cancellation",
		)
		result <- err
	}()

	select {
	case <-service.executeStarted:
	case <-time.After(time.Second):
		t.Fatal("custom step did not reach the gRPC service")
	}
	select {
	case hasDeadline := <-service.executionDeadline:
		assert.False(t, hasDeadline, "an unlimited workflow step must not receive an adapter deadline")
	case <-time.After(time.Second):
		t.Fatal("gRPC service did not report its execution context deadline")
	}
	cancel()

	select {
	case err := <-result:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("workflow did not return after caller cancellation")
	}
	require.NotNil(t, executionContext)
	stepState, found := executionContext.GetStepState("custom")
	require.True(t, found)
	assert.Equal(t, workflow.StatusFailed, stepState.Status)
}

func functionalCancellationWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "grpc-custom-step-cancellation",
		Initial: "custom",
		Steps: map[string]*workflow.Step{
			"custom": {
				Name:      "custom",
				Type:      "functional-plugin.wait-for-cancellation",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
}

type cancellationBlockingStepTypeService struct {
	pluginv1.UnimplementedStepTypeServiceServer
	executeStarted    chan struct{}
	executionDeadline chan bool
}

func (*cancellationBlockingStepTypeService) ListStepTypes(
	context.Context,
	*pluginv1.ListStepTypesRequest,
) (*pluginv1.ListStepTypesResponse, error) {
	return &pluginv1.ListStepTypesResponse{StepTypes: []*pluginv1.StepTypeInfo{{Name: "wait-for-cancellation"}}}, nil
}

func (s *cancellationBlockingStepTypeService) ExecuteStep(
	ctx context.Context,
	_ *pluginv1.ExecuteStepRequest,
) (*pluginv1.ExecuteStepResponse, error) {
	_, hasDeadline := ctx.Deadline()
	s.executionDeadline <- hasDeadline
	s.executeStarted <- struct{}{}
	<-ctx.Done()
	return nil, ctx.Err()
}

func newFunctionalStepTypeClient(t *testing.T, service pluginv1.StepTypeServiceServer) pluginv1.StepTypeServiceClient {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pluginv1.RegisterStepTypeServiceServer(server, service)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	connection, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	return pluginv1.NewStepTypeServiceClient(connection)
}
