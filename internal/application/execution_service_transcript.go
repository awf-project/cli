package application

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// recorderCtxKey scopes a per-run Recorder onto a context. Sub-workflow runs carry
// their own child Recorder (one file per sub-run, F106 US5) without mutating shared
// ExecutionService state, which keeps emission goroutine-safe under parallel steps.
type recorderCtxKey struct{}

// withRecorder returns a context that routes transcript emission to rec. A nil rec is
// returned unchanged so callers can wrap unconditionally.
func withRecorder(ctx context.Context, rec ports.Recorder) context.Context {
	if rec == nil {
		return ctx
	}
	return context.WithValue(ctx, recorderCtxKey{}, rec)
}

// recorderFor resolves the Recorder for ctx: a context-scoped child Recorder takes
// precedence, falling back to the service-level recorder for the top-level run.
func (s *ExecutionService) recorderFor(ctx context.Context) ports.Recorder {
	if rec, ok := ctx.Value(recorderCtxKey{}).(ports.Recorder); ok && rec != nil {
		return rec
	}
	return s.recorder
}

func (s *ExecutionService) emitTranscriptEvent(ctx context.Context, event transcript.ExchangeEvent) { //nolint:gocritic // hugeParam: callers construct event inline; pointer indirection adds no benefit here
	rec := s.recorderFor(ctx)
	if rec == nil {
		return
	}
	if err := rec.Record(ctx, event); err != nil && s.logger != nil {
		s.logger.Warn("transcript record warning", "error", err, "event", event.Type)
	}
}

func (s *ExecutionService) emitTranscriptStep(ctx context.Context, ec *workflow.ExecutionContext, step *workflow.Step, eventType transcript.EventType) {
	if s.recorderFor(ctx) == nil {
		return
	}
	var iteration int
	if ec.CurrentLoop != nil {
		iteration = ec.CurrentLoop.Index
	}
	s.emitTranscriptEvent(ctx, transcript.ExchangeEvent{
		Type:        eventType,
		RunID:       ec.WorkflowID,
		ParentRunID: ec.ParentRunID,
		Path:        buildTranscriptPath(ec, step),
		Iteration:   iteration,
		Timestamp:   time.Now(),
		Payload:     &transcript.StepPayload{Name: step.Name, Kind: string(step.Type)},
	})
}

func (s *ExecutionService) emitTranscriptAgentMessage(ctx context.Context, ec *workflow.ExecutionContext, prompt, systemPrompt string) {
	if s.recorderFor(ctx) == nil {
		return
	}
	// User/system input blocks are composed by AWF (the orchestrator), not emitted by the
	// agent — fidelity:"router" marks them accordingly (FR-002 requires the marker).
	blocks := []transcript.ContentBlock{{Type: transcript.BlockTypeText, Fidelity: transcript.FidelityRouter, Text: prompt}}
	if systemPrompt != "" {
		blocks = append(blocks, transcript.ContentBlock{Type: transcript.BlockTypeText, Fidelity: transcript.FidelityRouter, Text: systemPrompt})
	}
	s.emitTranscriptEvent(ctx, transcript.ExchangeEvent{
		Type:        transcript.EventTypeMessageUser,
		RunID:       ec.WorkflowID,
		ParentRunID: ec.ParentRunID,
		Path:        ec.WorkflowName,
		Timestamp:   time.Now(),
		Payload:     &transcript.MessagePayload{Role: "user", Blocks: blocks},
	})
}

// emitTranscriptAgentResponse normalizes a provider's raw NDJSON stream into ContentBlocks
// and emits a message.assistant event (F106 US2).
//
// rawOutput is the provider's RAW agent stream (NDJSON for CLI providers), NOT the
// extracted text response: the per-provider normalizers parse the NDJSON envelope, so
// feeding them the extracted text yields zero blocks (the P-1 production gap). text is the
// extracted assistant response used as a fallback for providers that expose no raw stream
// (e.g. openai_compatible over HTTP, whose tool calls are already captured at the Router
// seam — emitting only a text block here avoids double-counting per FR-009).
//
// It is a no-op when there is no recorder or when neither path yields any block, so a
// provider with no output never produces an empty assistant message.
func (s *ExecutionService) emitTranscriptAgentResponse(ctx context.Context, ec *workflow.ExecutionContext, provider, rawOutput, text string) {
	if s.recorderFor(ctx) == nil {
		return
	}

	var blocks []transcript.ContentBlock
	if s.agentOutputNormalizer != nil && rawOutput != "" {
		blocks = s.agentOutputNormalizer.Normalize(provider, []byte(rawOutput))
	}
	if len(blocks) == 0 && text != "" {
		// No raw stream (or it normalized to nothing): capture the extracted assistant
		// text as a single agent-emitted block rather than losing the turn entirely.
		blocks = []transcript.ContentBlock{{Type: transcript.BlockTypeText, Fidelity: transcript.FidelityAgentEmitted, Text: text}}
	}
	if len(blocks) == 0 {
		return
	}

	s.emitTranscriptEvent(ctx, transcript.ExchangeEvent{
		Type:        transcript.EventTypeMessageAssistant,
		RunID:       ec.WorkflowID,
		ParentRunID: ec.ParentRunID,
		Path:        ec.WorkflowName,
		Timestamp:   time.Now(),
		Payload:     &transcript.MessagePayload{Role: "assistant", Blocks: blocks},
	})
}

func (s *ExecutionService) emitTranscriptCallWorkflowStarted(ctx context.Context, ec *workflow.ExecutionContext, step *workflow.Step, childRunID string) {
	if s.recorderFor(ctx) == nil {
		return
	}
	s.emitTranscriptEvent(ctx, transcript.ExchangeEvent{
		Type:        transcript.EventTypeStepCallWorkflowStarted,
		RunID:       ec.WorkflowID,
		ParentRunID: ec.ParentRunID,
		ChildRunID:  childRunID,
		Path:        step.Name,
		Timestamp:   time.Now(),
		Payload:     &transcript.StepPayload{Name: step.Name, Kind: string(step.Type)},
	})
}

func (s *ExecutionService) emitTranscriptCallWorkflowCompleted(ctx context.Context, ec *workflow.ExecutionContext, step *workflow.Step, childRunID string, execErr error) {
	if s.recorderFor(ctx) == nil {
		return
	}
	payload := &transcript.StepPayload{Name: step.Name, Kind: string(step.Type)}
	if execErr != nil {
		payload.Error = execErr.Error()
	}
	s.emitTranscriptEvent(ctx, transcript.ExchangeEvent{
		Type:        transcript.EventTypeStepCallWorkflowCompleted,
		RunID:       ec.WorkflowID,
		ParentRunID: ec.ParentRunID,
		ChildRunID:  childRunID,
		Path:        step.Name,
		Timestamp:   time.Now(),
		Payload:     payload,
	})
}

// maxTranscriptLoopDepth bounds how many nested loop levels are encoded into a transcript
// path. Nesting deeper than this is truncated at the outermost levels — the path stays a
// human-readable label, not a lossless address (the run_id + step name remain exact). Eight
// levels is far beyond any realistic workflow nesting.
const maxTranscriptLoopDepth = 8

func buildTranscriptPath(ec *workflow.ExecutionContext, step *workflow.Step) string {
	if ec.CurrentLoop == nil {
		return step.Name
	}

	var stack [maxTranscriptLoopDepth]*workflow.LoopContext
	depth := 0
	for loop := ec.CurrentLoop; loop != nil && depth < len(stack); loop = loop.Parent {
		stack[depth] = loop
		depth++
	}

	var b strings.Builder
	for i := depth - 1; i >= 0; i-- {
		if i < depth-1 {
			b.WriteByte('/')
		}
		b.WriteString("loop:")
		b.WriteString(strconv.Itoa(stack[i].Index))
	}
	b.WriteByte('/')
	b.WriteString(step.Name)
	return b.String()
}
