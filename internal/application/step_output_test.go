package application

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
)

type outputChunk struct{ step, stream, text string }

type chunkCollector struct {
	mu     sync.Mutex
	chunks []outputChunk
}

func (c *chunkCollector) sink(step, stream, text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.chunks = append(c.chunks, outputChunk{step, stream, text})
}

func (c *chunkCollector) all() []outputChunk {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]outputChunk(nil), c.chunks...)
}

type failingWriter struct{ n int }

func (w failingWriter) Write(p []byte) (int, error) { return w.n, errors.New("disk full") }

func TestStepOutputWriter_Write(t *testing.T) {
	tests := []struct {
		name      string
		dst       func() (writerUnderTest, *bytes.Buffer)
		input     string
		wantN     int
		wantErr   bool
		wantDst   string
		wantChunk []string
	}{
		{
			name: "forwards to destination and reports chunk",
			dst: func() (writerUnderTest, *bytes.Buffer) {
				b := &bytes.Buffer{}
				return b, b
			},
			input: "hello\nwor", wantN: 9, wantDst: "hello\nwor", wantChunk: []string{"hello\nwor"},
		},
		{
			name:  "nil destination still reports chunk",
			dst:   func() (writerUnderTest, *bytes.Buffer) { return nil, nil },
			input: "only sink", wantN: 9, wantChunk: []string{"only sink"},
		},
		{
			name:      "empty write reports nothing",
			dst:       func() (writerUnderTest, *bytes.Buffer) { return &bytes.Buffer{}, nil },
			input:     "",
			wantN:     0,
			wantChunk: nil,
		},
		{
			name:      "destination error is returned and unwritten bytes are not reported",
			dst:       func() (writerUnderTest, *bytes.Buffer) { return failingWriter{n: 0}, nil },
			input:     "lost",
			wantN:     0,
			wantErr:   true,
			wantChunk: nil,
		},
		{
			name:      "short write reports only the written prefix",
			dst:       func() (writerUnderTest, *bytes.Buffer) { return failingWriter{n: 2}, nil },
			input:     "abcd",
			wantN:     2,
			wantErr:   true,
			wantChunk: []string{"ab"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst, buf := tt.dst()
			col := &chunkCollector{}
			w := &stepOutputWriter{dst: dst, sink: col.sink, stepName: "build", stream: ports.StepOutputStdout}

			n, err := w.Write([]byte(tt.input))

			assert.Equal(t, tt.wantN, n)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if buf != nil {
				assert.Equal(t, tt.wantDst, buf.String())
			}
			var got []string
			for _, c := range col.all() {
				assert.Equal(t, "build", c.step)
				assert.Equal(t, ports.StepOutputStdout, c.stream)
				got = append(got, c.text)
			}
			assert.Equal(t, tt.wantChunk, got)
		})
	}
}

// writerUnderTest keeps the table above readable: a nil io.Writer is a valid destination.
type writerUnderTest interface{ Write(p []byte) (int, error) }

func TestStepWriters_WithoutSinkReturnsServiceWritersUntouched(t *testing.T) {
	out, errw := &bytes.Buffer{}, &bytes.Buffer{}
	s := &ExecutionService{stdoutWriter: out, stderrWriter: errw}

	gotOut, gotErr := s.stepWriters(context.Background(), "step")

	assert.Same(t, out, gotOut, "no sink: stdout must be the exact service writer (zero overhead)")
	assert.Same(t, errw, gotErr, "no sink: stderr must be the exact service writer (zero overhead)")
}

func TestStepWriters_WithSinkTeesBothStreams(t *testing.T) {
	out, errw := &bytes.Buffer{}, &bytes.Buffer{}
	s := &ExecutionService{stdoutWriter: out, stderrWriter: errw}
	col := &chunkCollector{}
	ctx := withStepOutputSink(context.Background(), col.sink)

	stdout, stderr := s.stepWriters(ctx, "deploy")
	_, err := stdout.Write([]byte("to stdout"))
	require.NoError(t, err)
	_, err = stderr.Write([]byte("to stderr"))
	require.NoError(t, err)

	assert.Equal(t, "to stdout", out.String())
	assert.Equal(t, "to stderr", errw.String())
	assert.Equal(t, []outputChunk{
		{"deploy", ports.StepOutputStdout, "to stdout"},
		{"deploy", ports.StepOutputStderr, "to stderr"},
	}, col.all())
}

func TestWithStepOutputSink_NilSinkLeavesContextUnchanged(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, ctx, withStepOutputSink(ctx, nil))
	assert.Nil(t, stepOutputSinkFrom(ctx))
}

func TestRunSession_AppendStepOutput_PublishesWithoutReplayBuffering(t *testing.T) {
	s := newRunSession(context.Background(), "run-1", 16)

	s.appendStepOutput("build", ports.StepOutputStderr, "warning: x\n")

	select {
	case ev := <-s.Events():
		assert.Equal(t, ports.EventStepOutput, ev.Kind)
		assert.Equal(t, "run-1", ev.RunID)
		assert.Zero(t, ev.Seq)
		assert.False(t, ev.Timestamp.IsZero())
		payload, ok := ev.Payload.(*ports.EnrichedStepOutput)
		require.True(t, ok)
		assert.Equal(t, &ports.EnrichedStepOutput{StepName: "build", Stream: ports.StepOutputStderr, Text: "warning: x\n"}, payload)
	case <-time.After(time.Second):
		t.Fatal("no event published")
	}
	assert.Empty(t, s.ReplayFromSeq(0), "live output must never enter the replay buffer")
}

func TestRunSession_AppendStepOutput_IgnoresEmptyText(t *testing.T) {
	s := newRunSession(context.Background(), "run-1", 16)

	s.appendStepOutput("build", ports.StepOutputStdout, "")

	assert.Empty(t, s.Events())
}

func TestRunSession_AppendStepOutput_DropsAboveHighWaterLeavingRoomForLifecycleEvents(t *testing.T) {
	const size = 16
	s := newRunSession(context.Background(), "run-1", size)

	for i := 0; i < size*2; i++ {
		s.appendStepOutput("noisy", ports.StepOutputStdout, "x")
	}

	assert.Equal(t, size*3/4, len(s.Events()), "output must stop at the high-water mark")

	// A lifecycle event must still fit after the output flood.
	s.appendEvent(ports.Event{Kind: ports.EventStepCompleted, Seq: 1})
	assert.Equal(t, size*3/4+1, len(s.Events()), "lifecycle events must not be dropped by output back-pressure")
	assert.Len(t, s.ReplayFromSeq(0), 1)
}

func TestRunSession_AppendStepOutput_AfterCloseIsSafe(t *testing.T) {
	s := newRunSession(context.Background(), "run-1", 16)
	require.NoError(t, s.Close())

	assert.NotPanics(t, func() { s.appendStepOutput("late", ports.StepOutputStdout, "text") })
}

func TestRunSession_AppendStepOutput_ConcurrentWithCloseIsRaceFree(t *testing.T) {
	s := newRunSession(context.Background(), "run-1", 64)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				s.appendStepOutput("s", ports.StepOutputStdout, "chunk")
			}
		}()
	}
	go func() { _ = s.Close() }() //nolint:errcheck // Close always returns nil
	wg.Wait()
}

func TestResolveStepCommand_TeesLiveOutputWhenContextCarriesSink(t *testing.T) {
	out := &bytes.Buffer{}
	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
		stdoutWriter:  out,
		stderrWriter:  &bytes.Buffer{},
	}
	step := &workflow.Step{Name: "greet", Type: workflow.StepTypeCommand, Command: "echo hi"}
	col := &chunkCollector{}
	ctx := withStepOutputSink(context.Background(), col.sink)

	cmd, err := svc.resolveStepCommand(ctx, &workflow.Workflow{}, step, &interpolation.Context{})
	require.NoError(t, err)

	_, err = cmd.Stdout.Write([]byte("hi\n"))
	require.NoError(t, err)
	_, err = cmd.Stderr.Write([]byte("oops\n"))
	require.NoError(t, err)

	assert.Equal(t, "hi\n", out.String(), "the configured writer must still receive the output")
	assert.Equal(t, []outputChunk{
		{"greet", ports.StepOutputStdout, "hi\n"},
		{"greet", ports.StepOutputStderr, "oops\n"},
	}, col.all())
}

func TestResolveStepCommand_WithoutSinkKeepsServiceWriters(t *testing.T) {
	out := &bytes.Buffer{}
	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		resolver:      newRealResolver(),
		stdoutWriter:  out,
	}
	step := &workflow.Step{Name: "greet", Type: workflow.StepTypeCommand, Command: "echo hi"}

	cmd, err := svc.resolveStepCommand(context.Background(), &workflow.Workflow{}, step, &interpolation.Context{})

	require.NoError(t, err)
	assert.Same(t, out, cmd.Stdout)
}

func TestAdapter_NewSession_StepOutputEventsAreOptIn(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		wantEv  bool
	}{
		{"disabled by default behavior", false, false},
		{"enabled publishes EventStepOutput", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewAdapter(nil, nil, nil, nil, nil, NewSessionRegistry())
			adapter.SetStepOutputEvents(tt.enabled)
			emitted := make(chan struct{})
			release := make(chan struct{})

			session, err := adapter.newSession(context.Background(), "run-out", nil, func(ctx context.Context) error {
				if sink := stepOutputSinkFrom(ctx); sink != nil {
					sink("build", ports.StepOutputStdout, "compiling\n")
				}
				close(emitted)
				<-release
				return nil
			})
			require.NoError(t, err)
			t.Cleanup(func() { _ = session.Close() }) //nolint:errcheck // Close always returns nil

			<-emitted
			var got *ports.EnrichedStepOutput
			deadline := time.After(time.Second)
		collect:
			for {
				select {
				case ev := <-session.Events():
					if p, ok := ev.Payload.(*ports.EnrichedStepOutput); ok && ev.Kind == ports.EventStepOutput {
						got = p
						break collect
					}
				case <-time.After(100 * time.Millisecond):
					break collect
				case <-deadline:
					break collect
				}
			}
			close(release)

			if tt.wantEv {
				require.NotNil(t, got, "enabled adapter must publish the output chunk")
				assert.Equal(t, "build", got.StepName)
				assert.Equal(t, "compiling\n", got.Text)
			} else {
				assert.Nil(t, got, "output events must stay off unless explicitly enabled")
			}
		})
	}
}
