//go:build integration

package agents_test

import (
	"context"
	"io"
	"os"

	"github.com/awf-project/cli/internal/domain/ports"
)

// streamingMockExecutor writes NDJSON lines to stdout to exercise live stream
// filtering in provider integration tests.
type streamingMockExecutor struct {
	lines  []string
	stdout []byte
}

func (m *streamingMockExecutor) Run(_ context.Context, _ string, stdoutW, _ io.Writer, _ ...string) ([]byte, []byte, error) {
	if stdoutW != nil {
		for _, line := range m.lines {
			_, _ = stdoutW.Write([]byte(line + "\n"))
		}
	}
	return m.stdout, nil, nil
}

func (m *streamingMockExecutor) Start(_ context.Context, _ string, _ ...string) (ports.CLIProcess, error) {
	return &streamFakeProc{done: make(chan struct{})}, nil
}

type streamFakeProc struct {
	done chan struct{}
}

func (p *streamFakeProc) Signal(_ os.Signal) error { return nil }
func (p *streamFakeProc) Wait() error              { close(p.done); return nil }
func (p *streamFakeProc) Done() <-chan struct{}    { return p.done }

var _ ports.CLIExecutor = (*streamingMockExecutor)(nil)
