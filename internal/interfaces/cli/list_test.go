package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// mockWorkflowRepository implements ports.WorkflowRepository for testing.
type mockWorkflowRepository struct {
	workflows map[string]*workflow.Workflow
	listErr   error
}

func (m *mockWorkflowRepository) Load(_ context.Context, name string) (*workflow.Workflow, error) {
	wf, ok := m.workflows[name]
	if !ok {
		return nil, nil
	}
	return wf, nil
}

func (m *mockWorkflowRepository) List(_ context.Context) ([]string, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	names := make([]string, 0, len(m.workflows))
	for name := range m.workflows {
		names = append(names, name)
	}
	return names, nil
}

func (m *mockWorkflowRepository) Exists(_ context.Context, name string) (bool, error) {
	_, ok := m.workflows[name]
	return ok, nil
}

func TestListCommand_NoWorkflows(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No workflows found") {
		t.Errorf("expected 'No workflows found' message, got: %s", output)
	}
}

func TestListCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "list" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'list' subcommand")
	}
}
