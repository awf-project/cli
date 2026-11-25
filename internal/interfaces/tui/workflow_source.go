package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"gopkg.in/yaml.v3"

	"github.com/awf-project/cli/internal/domain/ports"
)

// maxSourceBytes caps how much of a workflow file is read.
const maxSourceBytes = 1 << 20

// ErrSourceNotFound is returned when no workflow file is known for a name, which is the
// case for workflows that come from packs.
var ErrSourceNotFound = errors.New("workflow source not found")

// SourceReader is the driven port for reading a workflow's raw YAML. The parsed
// workflow.Workflow cannot stand in for it: it has no YAML tags, so marshaling it would
// not give back what the author wrote (comments, key order, field names).
type SourceReader interface {
	// WorkflowSource returns the file content and its path.
	WorkflowSource(ctx context.Context, name string) (data []byte, path string, err error)
}

// RepositorySourceReader reads workflow files located through a ports.WorkflowRepository,
// which reports the path of every workflow it lists.
type RepositorySourceReader struct {
	repo ports.WorkflowRepository
}

// NewRepositorySourceReader returns a SourceReader backed by repo.
func NewRepositorySourceReader(repo ports.WorkflowRepository) *RepositorySourceReader {
	return &RepositorySourceReader{repo: repo}
}

// WorkflowSource implements SourceReader.
func (r *RepositorySourceReader) WorkflowSource(ctx context.Context, name string) (data []byte, path string, err error) {
	infos, err := r.repo.ListWithSource(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("listing workflows: %w", err)
	}
	for i := range infos {
		if infos[i].Name != name {
			continue
		}
		return readSourceFile(filepath.Clean(infos[i].Path))
	}
	return nil, "", ErrSourceNotFound
}

// readSourceFile reads a workflow file, refusing anything larger than maxSourceBytes.
func readSourceFile(path string) (data []byte, p string, err error) {
	f, err := os.Open(path) //nolint:gosec // path comes from the workflow repository, cleaned by the caller
	if err != nil {
		return nil, path, fmt.Errorf("opening workflow file: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only file
	data, err = io.ReadAll(io.LimitReader(f, maxSourceBytes+1))
	if err != nil {
		return nil, path, fmt.Errorf("reading workflow file: %w", err)
	}
	if len(data) > maxSourceBytes {
		return nil, path, fmt.Errorf("workflow file is larger than %d bytes", maxSourceBytes)
	}
	return data, path, nil
}

// ConfigItem is one block of a workflow's configuration: the general section, or a state.
type ConfigItem struct {
	// Name is the state name, or labelConfigGeneral for the general section.
	Name string
	// Kind is the state's `type` ("" for the general section or when it declares none).
	Kind string
	// Lines is the block as YAML.
	Lines []string
}

// WorkflowSourceLoadedMsg carries a workflow's configuration split into blocks, or the
// reason it could not be read. Name echoes the request so late replies can be ignored.
type WorkflowSourceLoadedMsg struct {
	Name  string
	Path  string
	Items []ConfigItem
	Err   string
}

// loadWorkflowSourceMsg asks the Model to load a workflow's configuration.
type loadWorkflowSourceMsg struct{ Name string }

// SetSourceReader wires the reader used by LoadWorkflowSource. A nil reader is accepted.
func (b *Bridge) SetSourceReader(r SourceReader) {
	b.sources = r
}

// LoadWorkflowSource returns a tea.Cmd that reads a workflow's YAML, splits it into blocks
// and emits WorkflowSourceLoadedMsg.
func (b *Bridge) LoadWorkflowSource(ctx context.Context, name string) tea.Cmd {
	return func() tea.Msg {
		if b.sources == nil {
			return WorkflowSourceLoadedMsg{Name: name, Err: labelConfigNoReader}
		}
		if err := ctx.Err(); err != nil {
			return WorkflowSourceLoadedMsg{Name: name, Err: err.Error()}
		}
		data, path, err := b.sources.WorkflowSource(ctx, name)
		if errors.Is(err, ErrSourceNotFound) {
			return WorkflowSourceLoadedMsg{Name: name, Err: labelConfigNoSource}
		}
		if err != nil {
			return WorkflowSourceLoadedMsg{Name: name, Path: path, Err: err.Error()}
		}
		items, err := parseWorkflowConfig(data)
		if err != nil {
			return WorkflowSourceLoadedMsg{Name: name, Path: path, Err: err.Error()}
		}
		return WorkflowSourceLoadedMsg{Name: name, Path: path, Items: items}
	}
}

// parseWorkflowConfig splits a workflow file into its general section (every top-level
// key except `states`, plus the `initial` state) followed by one block per state, in file
// order. Comments are kept.
func parseWorkflowConfig(src []byte) ([]ConfigItem, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, errors.New(labelConfigEmpty)
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New(labelConfigNotAWorkflow)
	}

	general := &yaml.Node{Kind: yaml.MappingNode}
	var states *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		if key.Value == "states" && value.Kind == yaml.MappingNode {
			states = value
			continue
		}
		general.Content = append(general.Content, key, value)
	}

	var stateItems []ConfigItem
	if states != nil {
		for i := 0; i+1 < len(states.Content); i += 2 {
			key, value := states.Content[i], states.Content[i+1]
			if key.Value == "initial" && value.Kind == yaml.ScalarNode {
				// The entry point belongs with the workflow's general settings.
				general.Content = append(general.Content, key, value)
				continue
			}
			stateItems = append(stateItems, ConfigItem{Name: key.Value, Kind: stateKind(value), Lines: marshalYAML(value)})
		}
	}

	items := make([]ConfigItem, 0, 1+len(stateItems))
	items = append(items, ConfigItem{Name: labelConfigGeneral, Lines: marshalYAML(general)})
	return append(items, stateItems...), nil
}

// stateKind returns the `type` declared by a state mapping, or "".
func stateKind(n *yaml.Node) string {
	if n.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "type" {
			return n.Content[i+1].Value
		}
	}
	return ""
}

// marshalYAML renders a node as indented YAML lines.
func marshalYAML(n *yaml.Node) []string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return []string{"# " + err.Error()}
	}
	_ = enc.Close() //nolint:errcheck // writing to a bytes.Buffer cannot fail
	return strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
}
