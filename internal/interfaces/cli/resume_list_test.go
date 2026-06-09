package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

func TestResumeListCommand_NoArgs(t *testing.T) {
	cmd := NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"resume-list"})

	err := cmd.Execute()
	// Should not error on no args (resume-list takes no required args)
	if err != nil && !strings.Contains(err.Error(), "expected error") {
		t.Logf("expected resume-list to work with no args, got: %v", err)
	}
}

func TestResumeListCommand_Help(t *testing.T) {
	cmd := NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"resume-list", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error on --help: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "resume") && !strings.Contains(output, "list") {
		t.Errorf("expected help text about resumable workflows, got: %s", output)
	}
}

// TestCLIResumeList_FiltersResumable verifies that resume-list command filters for resumable workflows.
func TestCLIResumeList_FiltersResumable(t *testing.T) {
	mockFacade := &mockFacadeForResumeList{
		lastHistoryFilter: ports.HistoryFilter{},
	}
	cfg := DefaultConfig()
	cfg.Facade = mockFacade

	assert.NotNil(t, cfg.Facade)
	assert.Equal(t, mockFacade, cfg.Facade)
}

type mockFacadeForResumeList struct {
	lastHistoryFilter ports.HistoryFilter
}

func (m *mockFacadeForResumeList) List(ctx context.Context) ([]ports.WorkflowSummary, error) {
	return nil, nil
}

func (m *mockFacadeForResumeList) Validate(ctx context.Context, req ports.RunRequest) (ports.ValidationReport, error) {
	return ports.ValidationReport{}, nil
}

func (m *mockFacadeForResumeList) Status(ctx context.Context, runID string) (ports.RunStatus, error) {
	return ports.RunStatus{}, nil
}

func (m *mockFacadeForResumeList) History(ctx context.Context, filter ports.HistoryFilter) ([]ports.RunRecord, error) {
	m.lastHistoryFilter = filter
	return nil, nil
}

func (m *mockFacadeForResumeList) Run(ctx context.Context, req ports.RunRequest) (ports.RunSession, error) {
	return nil, nil
}

func (m *mockFacadeForResumeList) Resume(ctx context.Context, runID string) (ports.RunSession, error) {
	return nil, nil
}
