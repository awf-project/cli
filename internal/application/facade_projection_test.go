package application_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/application"
	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allEventTypeMappings = []struct {
	src  transcript.EventType
	want ports.EventKind
}{
	{transcript.EventTypeRunStarted, ports.EventRunStarted},
	{transcript.EventTypeRunCompleted, ports.EventRunCompleted},
	{transcript.EventTypeStepStarted, ports.EventStepStarted},
	{transcript.EventTypeStepCompleted, ports.EventStepCompleted},
	{transcript.EventTypeStepCallWorkflowStarted, ports.EventStepCallWorkflowStarted},
	{transcript.EventTypeStepCallWorkflowCompleted, ports.EventStepCallWorkflowCompleted},
	{transcript.EventTypeMessageUser, ports.EventMessageUser},
	{transcript.EventTypeMessageAssistant, ports.EventMessageAssistant},
	{transcript.EventTypeToolCall, ports.EventToolCall},
	{transcript.EventTypeToolResult, ports.EventToolResult},
}

func TestProjectEvent_ExhaustiveMapping(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	eventGoPath := filepath.Join(filepath.Dir(testFile), "..", "domain", "transcript", "event.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, eventGoPath, nil, 0)
	require.NoError(t, err, "failed to parse transcript/event.go")

	var constCount int
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			valSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			ident, ok := valSpec.Type.(*ast.Ident)
			if !ok || ident.Name != "EventType" {
				continue
			}
			constCount += len(valSpec.Names)
		}
	}
	require.Equalf(t, constCount, len(allEventTypeMappings),
		"allEventTypeMappings must enumerate all %d transcript.EventType constants; got %d — add new ones with their expected EventKind",
		constCount, len(allEventTypeMappings))

	for _, m := range allEventTypeMappings {
		t.Run(string(m.src), func(t *testing.T) {
			ev := transcript.ExchangeEvent{Type: m.src, Seq: 1, RunID: "r1"}
			got, err := application.ProjectEvent(ev)
			assert.NoError(t, err)
			assert.Equal(t, m.want, got.Kind)
		})
	}
}

func TestProjectEvent_UnknownEventTypeFailsClosed(t *testing.T) {
	ev := transcript.ExchangeEvent{
		Type:  transcript.EventType("totally.unknown.event"),
		Seq:   7,
		RunID: "r1",
	}
	got, err := application.ProjectEvent(ev)
	assert.Equal(t, ports.EventKindUnknown, got.Kind)
	var structuredErr *domainerrors.StructuredError
	require.ErrorAs(t, err, &structuredErr, "projection error must be a *StructuredError (non-fatal system error per NFR-007)")
	assert.Equal(t, "SYSTEM", structuredErr.Code.Category(), "projection error must be in SYSTEM category")
}

func TestProjectEvent_UnknownBlockTypeFailsClosed(t *testing.T) {
	ev := transcript.ExchangeEvent{
		Type:    transcript.EventTypeMessageAssistant,
		Seq:     3,
		RunID:   "r2",
		Payload: []transcript.ContentBlock{{Type: transcript.BlockType("future_unknown_block_xyz")}},
	}
	got, err := application.ProjectEvent(ev)
	assert.Equal(t, ports.EventKindUnknown, got.Kind)
	var structuredErr *domainerrors.StructuredError
	require.ErrorAs(t, err, &structuredErr, "projection error must be a *StructuredError (non-fatal system error per NFR-007)")
	assert.Equal(t, "SYSTEM", structuredErr.Code.Category(), "projection error must be in SYSTEM category")
}

func TestProjectEvent_NoProviderBranching(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	sourceFile := filepath.Join(filepath.Dir(testFile), "facade_projection.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, sourceFile, nil, 0)
	require.NoError(t, err, "failed to parse facade_projection.go")

	providerNames := []string{"claude", "gemini", "codex", "copilot", "openai"}

	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		lower := strings.ToLower(lit.Value)
		for _, provider := range providerNames {
			assert.NotContains(t, lower, provider,
				"facade_projection.go must not contain provider string literal %q (D10, NFR-006)", provider)
		}
		return true
	})
}

func TestProjectEvent_SeqPreserved(t *testing.T) {
	samplePayload := []transcript.ContentBlock{{Type: transcript.BlockTypeText}}
	inputs := []transcript.ExchangeEvent{
		{Type: transcript.EventTypeRunStarted, Seq: 1, RunID: "run-abc", ParentRunID: "parent-xyz", Payload: samplePayload},
		{Type: transcript.EventTypeRunStarted, Seq: 42, RunID: "run-abc", ParentRunID: "parent-xyz", Payload: samplePayload},
		{Type: transcript.EventTypeRunStarted, Seq: 999, RunID: "run-abc", ParentRunID: "parent-xyz", Payload: samplePayload},
		{Type: transcript.EventTypeRunStarted, Seq: 1_000_000, RunID: "run-abc", ParentRunID: "parent-xyz", Payload: samplePayload},
	}
	for _, ev := range inputs {
		got, err := application.ProjectEvent(ev)
		assert.NoError(t, err)
		assert.Equal(t, ev.Seq, got.Seq, "Seq must be preserved verbatim for Seq=%d", ev.Seq)
		assert.Equal(t, ev.RunID, got.RunID, "RunID must be preserved verbatim")
		assert.Equal(t, ev.ParentRunID, got.ParentRunID, "ParentRunID must be preserved verbatim")
		assert.Equal(t, ev.Payload, got.Payload, "Payload must be preserved verbatim")
	}
}

func TestProjectEvent_NeverPanics(t *testing.T) {
	inputs := []transcript.ExchangeEvent{
		{},
		{Type: ""},
		{Type: "totally.unknown"},
		{Type: transcript.EventTypeMessageAssistant, Payload: nil},
		{Type: transcript.EventTypeMessageAssistant, Payload: "malformed payload string"},
		{
			Type:    transcript.EventTypeMessageAssistant,
			Payload: []transcript.ContentBlock{{Type: transcript.BlockType("unknown_block_xyz")}},
		},
		{
			Seq:         ^uint64(0),
			Type:        transcript.EventTypeToolCall,
			RunID:       strings.Repeat("x", 4096),
			ParentRunID: strings.Repeat("y", 4096),
		},
	}

	for i, ev := range inputs {
		assert.NotPanics(t, func() {
			application.ProjectEvent(ev) //nolint:errcheck // panic-safety test; return values are intentionally ignored
		}, "input[%d] must not panic", i)
	}
}
