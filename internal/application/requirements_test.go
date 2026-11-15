package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePluginVersions map[string]string

func (f fakePluginVersions) EnabledPluginVersion(name string) (string, bool) {
	version, ok := f[name]
	return version, ok
}

type fakeAgentPlugins struct {
	installed map[string]bool
	err       error
}

func (f fakeAgentPlugins) Supports(provider string) bool { return provider == "claude" }

func (f fakeAgentPlugins) Installed(_ context.Context, _, plugin string) (bool, error) {
	return f.installed[plugin], f.err
}

func requiringWorkflow(requires workflow.Requirements) *workflow.Workflow {
	return &workflow.Workflow{
		Name:     "needs",
		Initial:  "run",
		Requires: requires,
		Steps: map[string]*workflow.Step{
			"run":  {Name: "run", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
}

func requirementService(versions ports.PluginVersionLookup, agentPlugins ports.AgentPluginRegistry) *application.WorkflowService {
	svc := application.NewWorkflowService(nil, nil, nil, nil, nil)
	svc.SetPluginVersionLookup(versions)
	svc.SetAgentPluginRegistry(agentPlugins)
	return svc
}

func TestValidateRequirements(t *testing.T) {
	tests := []struct {
		name     string
		requires workflow.Requirements
		opts     ports.ValidateOptions
		agentErr error
		wantErr  []string
	}{
		{name: "no requirement"},
		{name: "plugin installed, any version", requires: workflow.Requirements{Plugins: map[string]string{"nexus": ""}}},
		{name: "plugin installed in a matching version", requires: workflow.Requirements{Plugins: map[string]string{"nexus": ">=0.1.0"}}},
		{
			name:     "plugin missing",
			requires: workflow.Requirements{Plugins: map[string]string{"jira": ">=1.0.0"}},
			wantErr:  []string{"requires.plugins.jira", `awf plugin "jira", which is not installed or not enabled`, "awf plugin enable jira"},
		},
		{
			name:     "plugin too old",
			requires: workflow.Requirements{Plugins: map[string]string{"nexus": ">=0.2.0"}},
			wantErr:  []string{`"nexus" >=0.2.0, and version 0.1.3 is installed`, "awf plugin update nexus"},
		},
		{
			name:     "invalid constraint",
			requires: workflow.Requirements{Plugins: map[string]string{"nexus": "soon"}},
			wantErr:  []string{`version constraint "soon"`},
		},
		{name: "plugin without a semantic version", requires: workflow.Requirements{Plugins: map[string]string{"github": ">=9.0.0"}}},
		{name: "plugin missing, with SkipPlugins", requires: workflow.Requirements{Plugins: map[string]string{"jira": ""}}, opts: ports.ValidateOptions{SkipPlugins: true}},
		{name: "agent plugin installed", requires: workflow.Requirements{AgentPlugins: map[string][]string{"claude": {"egm@egm"}}}},
		{
			name:     "agent plugin missing, reported with SkipPlugins too",
			requires: workflow.Requirements{AgentPlugins: map[string][]string{"claude": {"egm@egm", "other@market"}}},
			opts:     ports.ValidateOptions{SkipPlugins: true},
			wantErr:  []string{`claude plugin "other@market", which is not installed`, "claude plugin install other@market"},
		},
		{
			name:     "unsupported agent provider",
			requires: workflow.Requirements{AgentPlugins: map[string][]string{"codex": {"x"}}},
			wantErr:  []string{`agent provider "codex"`},
		},
		{
			name:     "unreadable agent registry",
			requires: workflow.Requirements{AgentPlugins: map[string][]string{"claude": {"egm@egm"}}},
			agentErr: errors.New("permission denied"),
			wantErr:  []string{"cannot be read: permission denied"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := requirementService(
				fakePluginVersions{"nexus": "0.1.3", "github": "dev"},
				fakeAgentPlugins{installed: map[string]bool{"egm@egm": true}, err: tt.agentErr},
			)

			err := svc.ValidateLoadedWorkflow(context.Background(), requiringWorkflow(tt.requires), "needs", tt.opts)

			if len(tt.wantErr) == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, part := range tt.wantErr {
				assert.Contains(t, err.Error(), part)
			}
		})
	}
}

func TestValidateRequirements_NotCheckedWithoutLookups(t *testing.T) {
	svc := application.NewWorkflowService(nil, nil, nil, nil, nil)
	wf := requiringWorkflow(workflow.Requirements{Plugins: map[string]string{"jira": ""}, AgentPlugins: map[string][]string{"claude": {"egm@egm"}}})

	require.NoError(t, svc.ValidateLoadedWorkflow(context.Background(), wf, "needs", ports.ValidateOptions{}))
}
