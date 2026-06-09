// Package facadetest provides a scriptable test double (fake) for the
// ports.WorkflowFacade and ports.RunSession interfaces.
//
// # Overview
//
// facadetest is the third member of the AWF testutil family alongside
// internal/testutil/mocks (thread-safe port mocks) and
// internal/testutil/builders (fluent object builders).  Its purpose is
// different: instead of recording or stubbing individual method calls, it
// lets a test script a complete event sequence that a FakeSession will emit in
// order, including synchronization points where the session pauses until the
// consumer calls Respond.
//
// # Architecture
//
// The Fake struct implements ports.WorkflowFacade.
// FakeSession implements ports.RunSession.
//
// Design decisions:
//   - D37: New testutil sub-packages get doc.go per CLAUDE.md convention.
//   - D38: Builder-style API allows all consumer packages (CLI, API, ACP, conformance
//     suite) to share one fake rather than defining per-package inline doubles.
//
// The package depends only on:
//   - internal/domain/ports  — the interface contract Fake and FakeSession satisfy.
//   - internal/application   — MapError for ErrorCode mapping in WithTerminalFailed.
//
// It does NOT depend on the production FacadeAdapter (internal/interfaces/facade).
// Depending on the adapter would create a compilation cycle and block the
// parallel build of T064 and T058.
//
// # Event Channel Lifecycle
//
// Each call to Fake.Run creates a fresh FakeSession with its own buffered event
// channel, sized to len(script)+1 to avoid blocking the pump goroutine.  The
// pump goroutine sends events in order and exits (closing the channel) after:
//   - a terminal event (EventWorkflowCompleted or EventWorkflowFailed) is sent, or
//   - the context is cancelled, or
//   - FakeSession.Close is called.
//
// The events channel is always closed before Close returns, so reading from
// sess.Events() after sess.Close() immediately yields the zero value and false.
//
// # Builder API
//
// Construct a Fake with the New() constructor and chain builder methods:
//
//	f := facadetest.New().
//	    Script(
//	        ports.Event{Kind: ports.EventRunStarted, Seq: 1},
//	        ports.Event{Kind: ports.EventStepStarted, Seq: 2},
//	    ).
//	    WithTerminalCompleted()
//
// Builder methods:
//
//	Script(events ...ports.Event) — append raw events to the script.
//
//	WithTerminalCompleted() — append EventWorkflowCompleted (terminal, closes channel).
//
//	WithTerminalFailed(err error) — append EventWorkflowFailed with the
//	  application.MapError(err) ErrorCode as the event Payload.
//
//	WithInputRequired(req ports.InputRequest) — append EventInputRequired with req
//	  as Payload.  The pump goroutine pauses after emitting this event and
//	  resumes only when FakeSession.Respond is called.
//
//	WithHistory(records ...ports.RunRecord) — seed records returned by
//	  Fake.History; useful for testing history-display code paths.
//
// # Respond Synchronization
//
// When the scripted sequence contains an EventInputRequired event, the ordering
// contract is:
//
//  1. Consumer reads EventInputRequired from sess.Events().
//  2. No further event is available on the channel (pump is blocked).
//  3. Consumer calls sess.Respond(ports.InputResponse{...}).
//  4. Pump unblocks and emits the next scripted event.
//
// This models the real FacadeAdapter behavior where the workflow is paused
// awaiting user input.
//
// # Integration with the Conformance Suite
//
// T065 builds a cross-interface conformance suite that exercises all consumer
// packages (CLI, TUI, HTTP API, ACP) against a single shared Fake.
// Each consumer test creates a Fake with the scenario under test, calls the
// consumer entry-point, and asserts the resulting output or state.
//
// Import path:
//
//	import "github.com/awf-project/cli/internal/testutil/facadetest"
//
// # Contract Compliance
//
// Fake satisfies the five-point port contract verified by
// internal/domain/ports/facade_contract_test.go:
//
//  1. Close is idempotent (multiple calls return nil).
//  2. Events channel is closed after Close.
//  3. Run with empty Identifier returns ports.ErrInvalidRequest.
//  4. Run with a cancelled context propagates the context error.
//  5. Respond after Close returns ports.ErrSessionClosed.
//
// TestFakeFacade_SatisfiesPortContract in facadetest_test.go re-runs these
// five assertions against Fake and FakeSession directly.
//
// # Thread Safety
//
// Fake is safe for concurrent calls to Run, History, and the builder methods.
// Each FakeSession is an independent instance; multiple sessions created from
// the same Fake do not share state.
//
// FakeSession.Respond and FakeSession.Close are thread-safe and may be called
// concurrently with Events().
package facadetest
