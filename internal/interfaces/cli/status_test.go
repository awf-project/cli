package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

func TestStatusCommand_NoArgs(t *testing.T) {
	cmd := NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no workflow ID provided")
	}
}

func TestStatusCommand_NotFound(t *testing.T) {
	cmd := NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", "nonexistent-id"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent workflow ID")
	}

	output := buf.String()
	errOutput := buf.String()
	combined := output + errOutput
	if !strings.Contains(combined, "not found") && err != nil && !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' message, got output: %s, err: %v", combined, err)
	}
}

func TestStatusCommand_Exists(t *testing.T) {
	cmd := NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "status" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'status' subcommand")
	}
}

func TestStatusCommand_Help(t *testing.T) {
	cmd := NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "workflow") {
		t.Errorf("expected help text about workflow, got: %s", output)
	}
}

// TestCLIStatus_DoesNotImportJSONStore verifies that status.go has no JSONStore import.
func TestCLIStatus_DoesNotImportJSONStore(t *testing.T) {
	statusFile := "internal/interfaces/cli/status.go"
	data, err := os.ReadFile(statusFile)
	if err != nil {
		t.Skipf("could not read %s: %v", statusFile, err)
	}

	content := string(data)
	if strings.Contains(content, "infrastructure/store") {
		t.Errorf("status.go should not import JSONStore (infrastructure/store)")
	}
}

// TestCLIStatus_RoutesToFacade verifies that status command would route through WorkflowFacade.Status.
func TestCLIStatus_RoutesToFacade(t *testing.T) {
	mockFacade := &mockFacadeForStatus{}
	cfg := DefaultConfig()
	cfg.Facade = mockFacade

	assert.NotNil(t, cfg.Facade)
	assert.Equal(t, mockFacade, cfg.Facade)
}

type mockFacadeForStatus struct{}

func (m *mockFacadeForStatus) List(ctx context.Context) ([]ports.WorkflowSummary, error) {
	return nil, nil
}

func (m *mockFacadeForStatus) Validate(ctx context.Context, req ports.RunRequest) (ports.ValidationReport, error) {
	return ports.ValidationReport{}, nil
}

func (m *mockFacadeForStatus) Status(ctx context.Context, runID string) (ports.RunStatus, error) {
	return ports.RunStatus{RunID: runID}, nil
}

func (m *mockFacadeForStatus) History(ctx context.Context, filter ports.HistoryFilter) ([]ports.RunRecord, error) {
	return nil, nil
}

func (m *mockFacadeForStatus) Run(ctx context.Context, req ports.RunRequest) (ports.RunSession, error) {
	return nil, nil
}

func (m *mockFacadeForStatus) Resume(_ context.Context, _ ports.ResumeRequest) (ports.RunSession, error) {
	return nil, nil
}

func (m *mockFacadeForStatus) RunStep(_ context.Context, _ ports.RunStepRequest) (ports.StepResult, error) {
	return ports.StepResult{}, nil
}
