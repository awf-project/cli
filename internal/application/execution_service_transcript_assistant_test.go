package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// fakeNormalizer is a test double for ports.AgentOutputNormalizer.
type fakeNormalizer struct {
	blocks      []transcript.ContentBlock
	gotProvider string
	gotRaw      string
	calls       int
}

func (f *fakeNormalizer) Normalize(provider string, rawOutput []byte) []transcript.ContentBlock {
	f.calls++
	f.gotProvider = provider
	f.gotRaw = string(rawOutput)
	return f.blocks
}

// TestEmitTranscriptAgentResponse_EmitsAssistantMessage verifies F106 US2: provider raw
// NDJSON output is normalized and emitted as a message.assistant event carrying the blocks.
func TestEmitTranscriptAgentResponse_EmitsAssistantMessage(t *testing.T) {
	svc := newTestExecutionService()
	rec := &fakeRecorder{}
	svc.SetRecorder(rec)
	norm := &fakeNormalizer{blocks: []transcript.ContentBlock{
		{Type: transcript.BlockTypeText, Fidelity: transcript.FidelityAgentEmitted, Text: "hi"},
	}}
	svc.SetAgentOutputNormalizer(norm)

	ec := workflow.NewExecutionContext("wf-assistant", "test-workflow")
	// rawOutput is the NDJSON stream; text is the extracted fallback (ignored when blocks exist).
	svc.emitTranscriptAgentResponse(context.Background(), ec, "claude", "raw-ndjson-output", "extracted text")

	require.Len(t, rec.events, 1)
	ev := rec.events[0]
	assert.Equal(t, transcript.EventTypeMessageAssistant, ev.Type)
	assert.Equal(t, "wf-assistant", ev.RunID)

	payload, ok := ev.Payload.(*transcript.MessagePayload)
	require.True(t, ok)
	assert.Equal(t, "assistant", payload.Role)
	require.Len(t, payload.Blocks, 1)
	assert.Equal(t, "hi", payload.Blocks[0].Text)
	assert.Equal(t, transcript.FidelityAgentEmitted, payload.Blocks[0].Fidelity)

	// Normalizer received the resolved provider name and the RAW NDJSON output (not the
	// extracted text) — this is the P-1 regression guard: feeding extracted text here
	// would yield zero blocks in production.
	assert.Equal(t, "claude", norm.gotProvider)
	assert.Equal(t, "raw-ndjson-output", norm.gotRaw)
}

// TestEmitTranscriptAgentResponse_FallsBackToTextBlock verifies that when the normalizer
// yields no blocks (e.g. openai_compatible HTTP, which has no raw NDJSON stream, or a
// non-streaming provider) but an extracted assistant text is available, a single
// agent-emitted text block is captured so the assistant turn is never lost.
func TestEmitTranscriptAgentResponse_FallsBackToTextBlock(t *testing.T) {
	svc := newTestExecutionService()
	rec := &fakeRecorder{}
	svc.SetRecorder(rec)
	// Normalizer present but produces nothing from the (empty) raw stream.
	svc.SetAgentOutputNormalizer(&fakeNormalizer{blocks: nil})

	ec := workflow.NewExecutionContext("wf-fallback", "test-workflow")
	svc.emitTranscriptAgentResponse(context.Background(), ec, "openai_compatible", "", "final answer")

	require.Len(t, rec.events, 1)
	payload, ok := rec.events[0].Payload.(*transcript.MessagePayload)
	require.True(t, ok)
	assert.Equal(t, "assistant", payload.Role)
	require.Len(t, payload.Blocks, 1)
	assert.Equal(t, "final answer", payload.Blocks[0].Text)
	assert.Equal(t, transcript.BlockTypeText, payload.Blocks[0].Type)
	assert.Equal(t, transcript.FidelityAgentEmitted, payload.Blocks[0].Fidelity)
}

// TestEmitTranscriptAgentResponse_NoBlocksNoTextNoEvent verifies that output yielding no
// blocks and no fallback text does not produce an empty assistant message.
func TestEmitTranscriptAgentResponse_NoBlocksNoTextNoEvent(t *testing.T) {
	svc := newTestExecutionService()
	rec := &fakeRecorder{}
	svc.SetRecorder(rec)
	svc.SetAgentOutputNormalizer(&fakeNormalizer{blocks: nil})

	ec := workflow.NewExecutionContext("wf-empty", "test-workflow")
	svc.emitTranscriptAgentResponse(context.Background(), ec, "claude", "raw", "")

	assert.Empty(t, rec.events, "no blocks and no text must produce no assistant event")
}

// TestEmitTranscriptAgentResponse_NilNormalizerStillFallsBack verifies that with no
// normalizer wired, the extracted assistant text is still captured as a text block.
func TestEmitTranscriptAgentResponse_NilNormalizerStillFallsBack(t *testing.T) {
	svc := newTestExecutionService()
	rec := &fakeRecorder{}
	svc.SetRecorder(rec)
	// no normalizer set

	ec := workflow.NewExecutionContext("wf-nonorm", "test-workflow")
	svc.emitTranscriptAgentResponse(context.Background(), ec, "claude", "raw", "answer text")

	require.Len(t, rec.events, 1)
	payload, ok := rec.events[0].Payload.(*transcript.MessagePayload)
	require.True(t, ok)
	require.Len(t, payload.Blocks, 1)
	assert.Equal(t, "answer text", payload.Blocks[0].Text)
}

// TestEmitTranscriptAgentResponse_NoRecorderIsNoOp verifies graceful degradation when no
// recorder is wired.
func TestEmitTranscriptAgentResponse_NoRecorderIsNoOp(t *testing.T) {
	svc := newTestExecutionService()
	svc.SetAgentOutputNormalizer(&fakeNormalizer{blocks: []transcript.ContentBlock{{Type: transcript.BlockTypeText}}})

	ec := workflow.NewExecutionContext("wf-norec", "test-workflow")
	// Must not panic and must be a no-op (no recorder).
	svc.emitTranscriptAgentResponse(context.Background(), ec, "claude", "raw", "text")
}

// TestEmitTranscriptAgentResponse_EmptyRawSkipsNormalizer verifies an empty raw stream
// short-circuits the normalizer (the text fallback path is exercised separately).
func TestEmitTranscriptAgentResponse_EmptyRawSkipsNormalizer(t *testing.T) {
	svc := newTestExecutionService()
	rec := &fakeRecorder{}
	svc.SetRecorder(rec)
	norm := &fakeNormalizer{blocks: []transcript.ContentBlock{{Type: transcript.BlockTypeText}}}
	svc.SetAgentOutputNormalizer(norm)

	ec := workflow.NewExecutionContext("wf-emptyraw", "test-workflow")
	svc.emitTranscriptAgentResponse(context.Background(), ec, "claude", "", "")

	assert.Empty(t, rec.events)
	assert.Equal(t, 0, norm.calls, "empty raw output must short-circuit before the normalizer")
}
