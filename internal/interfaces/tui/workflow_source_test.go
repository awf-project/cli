package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

func itemNames(items []ConfigItem) []string {
	out := make([]string, len(items))
	for i := range items {
		out[i] = items[i].Name
	}
	return out
}

func TestParseWorkflowConfig_SplitsGeneralSectionAndStates(t *testing.T) {
	items, err := parseWorkflowConfig([]byte(sampleYAML))
	require.NoError(t, err)

	assert.Equal(t, []string{labelConfigGeneral, "build", "test", "done"}, itemNames(items), "general first, then states in file order")
	general := strings.Join(items[0].Lines, "\n")
	for _, want := range []string{"name: deploy", `version: "1.2.0"`, "inputs:", "env:", "initial: build"} {
		assert.Contains(t, general, want)
	}
	assert.NotContains(t, general, "command:", "state bodies are not part of the general section")
	assert.NotContains(t, general, "states:")
}

func TestParseWorkflowConfig_StateBlocksKeepTheirContentAndKind(t *testing.T) {
	items, err := parseWorkflowConfig([]byte(sampleYAML))
	require.NoError(t, err)

	build := items[1]
	assert.Equal(t, "step", build.Kind)
	body := strings.Join(build.Lines, "\n")
	assert.Contains(t, body, "command: make build")
	assert.Contains(t, body, "# compile", "comments survive the split")
	assert.Contains(t, body, "on_success: test")

	assert.Equal(t, "terminal", items[3].Kind)
	assert.Empty(t, items[0].Kind, "the general section has no kind")
}

func TestParseWorkflowConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
		want    []string
	}{
		{"empty file", "", labelConfigEmpty, nil},
		{"only comments", "# nothing here\n", labelConfigEmpty, nil},
		{"root is a list", "- a\n- b\n", labelConfigNotAWorkflow, nil},
		{"root is a scalar", "just text\n", labelConfigNotAWorkflow, nil},
		{"broken yaml", "name: [unclosed\n", "invalid YAML", nil},
		{"no states", "name: x\nversion: \"1\"\n", "", []string{labelConfigGeneral}},
		{"states is not a mapping", "name: x\nstates: nope\n", "", []string{labelConfigGeneral}},
		{"state without a type", "states:\n  initial: a\n  a:\n    command: x\n", "", []string{labelConfigGeneral, "a"}},
		{"initial only", "states:\n  initial: a\n", "", []string{labelConfigGeneral}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := parseWorkflowConfig([]byte(tt.src))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, itemNames(items))
		})
	}
}

func TestParseWorkflowConfig_StateWithoutATypeHasNoKind(t *testing.T) {
	items, err := parseWorkflowConfig([]byte("states:\n  initial: a\n  a:\n    command: x\n"))
	require.NoError(t, err)
	assert.Empty(t, items[1].Kind)
}

// stubRepo is a ports.WorkflowRepository that only knows where workflow files are.
type stubRepo struct {
	infos []ports.WorkflowInfo
	err   error
}

func (r stubRepo) Load(context.Context, string) (*workflow.Workflow, error) { return nil, nil }
func (r stubRepo) List(context.Context) ([]string, error)                   { return nil, nil }
func (r stubRepo) Exists(context.Context, string) (bool, error)             { return false, nil }
func (r stubRepo) ListWithSource(context.Context) ([]ports.WorkflowInfo, error) {
	return r.infos, r.err
}

func TestRepositorySourceReader(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "deploy.yaml")
	require.NoError(t, os.WriteFile(file, []byte(sampleYAML), 0o600))
	big := filepath.Join(dir, "big.yaml")
	require.NoError(t, os.WriteFile(big, []byte(strings.Repeat("x", maxSourceBytes+10)), 0o600))

	repo := stubRepo{infos: []ports.WorkflowInfo{
		{Name: "deploy", Path: file},
		{Name: "big", Path: big},
		{Name: "gone", Path: filepath.Join(dir, "missing.yaml")},
	}}
	r := NewRepositorySourceReader(repo)

	t.Run("reads the file the repository points to", func(t *testing.T) {
		data, path, err := r.WorkflowSource(context.Background(), "deploy")
		require.NoError(t, err)
		assert.Equal(t, sampleYAML, string(data))
		assert.Equal(t, file, path)
	})
	t.Run("unknown workflow", func(t *testing.T) {
		_, _, err := r.WorkflowSource(context.Background(), "my-pack/release")
		assert.ErrorIs(t, err, ErrSourceNotFound)
	})
	t.Run("file listed but missing", func(t *testing.T) {
		_, path, err := r.WorkflowSource(context.Background(), "gone")
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrSourceNotFound, "a missing file is an error, not 'a pack workflow'")
		assert.Contains(t, path, "missing.yaml")
	})
	t.Run("file larger than the cap", func(t *testing.T) {
		_, _, err := r.WorkflowSource(context.Background(), "big")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "larger than")
	})
	t.Run("repository failure", func(t *testing.T) {
		_, _, err := NewRepositorySourceReader(stubRepo{err: errors.New("boom")}).WorkflowSource(context.Background(), "deploy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})
}

func TestBridge_LoadWorkflowSource(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name      string
		reader    SourceReader
		ctx       context.Context
		wantErr   string
		wantItems int
		wantPath  string
	}{
		{"no reader", nil, context.Background(), labelConfigNoReader, 0, ""},
		{"workflow from a pack", &fakeSources{err: ErrSourceNotFound}, context.Background(), labelConfigNoSource, 0, ""},
		{"read failure keeps the path", &fakeSources{err: errors.New("denied"), path: "/x/w.yaml"}, context.Background(), "denied", 0, "/x/w.yaml"},
		{"invalid yaml", &fakeSources{data: []byte("a: [oops"), path: "/x/w.yaml"}, context.Background(), "invalid YAML", 0, "/x/w.yaml"},
		{"cancelled context", &fakeSources{data: []byte(sampleYAML)}, cancelled, context.Canceled.Error(), 0, ""},
		{"success", &fakeSources{data: []byte(sampleYAML), path: "/x/w.yaml"}, context.Background(), "", 4, "/x/w.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBridge(nil, nil)
			if tt.reader != nil {
				b.SetSourceReader(tt.reader)
			}

			msg := b.LoadWorkflowSource(tt.ctx, "deploy")()

			got, ok := msg.(WorkflowSourceLoadedMsg)
			require.True(t, ok)
			assert.Equal(t, "deploy", got.Name, "the request is echoed so late replies can be matched")
			if tt.wantErr == "" {
				assert.Empty(t, got.Err)
			} else {
				assert.Contains(t, got.Err, tt.wantErr)
			}
			assert.Len(t, got.Items, tt.wantItems)
			assert.Equal(t, tt.wantPath, got.Path)
		})
	}
}

func TestStyleYAMLLine_OnlyDecorates(t *testing.T) {
	for _, line := range []string{
		"name: deploy", "  command: make build   # compile", "- item", "  - key: value", "# a comment",
		"    nested:", "", "plain text without a key", "url: http://example.com:8080/x",
	} {
		assert.Equal(t, line, stripANSI(styleYAMLLine(line)), "styling must never change the text of %q", line)
	}
}
