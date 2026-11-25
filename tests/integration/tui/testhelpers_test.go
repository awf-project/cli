//go:build integration && !windows

package tui_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
)

const maxTUICaptureBytes = 1 << 20

type boundedTerminalBuffer struct {
	mu  sync.RWMutex
	buf bytes.Buffer
}

func (b *boundedTerminalBuffer) append(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := maxTUICaptureBytes - b.buf.Len()
	if remaining <= 0 {
		return
	}
	if len(data) > remaining {
		data = data[:remaining]
	}
	_, _ = b.buf.Write(data)
}

func (b *boundedTerminalBuffer) contains(value string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return bytes.Contains(b.buf.Bytes(), []byte(value))
}

func (b *boundedTerminalBuffer) text() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.buf.String()
}

type tuiPTYProcess struct {
	command   *exec.Cmd
	terminal  *os.File
	done      chan error
	output    boundedTerminalBuffer
	closeOnce sync.Once
}

func startTUIUnderPTY(t *testing.T, binary string, args, environment []string) *tuiPTYProcess {
	t.Helper()
	command := exec.Command(binary) //nolint:gosec // Binary and arguments are test-owned.
	for _, arg := range args {
		command.Args = append(command.Args, arg)
	}
	command.Env = os.Environ()
	for _, entry := range environment {
		command.Env = append(command.Env, entry)
	}
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 30, Cols: 120})
	require.NoError(t, err)

	process := &tuiPTYProcess{command: command, terminal: terminal, done: make(chan error, 1)}
	go func() {
		data := make([]byte, 4096)
		for {
			n, readErr := terminal.Read(data)
			if n > 0 {
				process.output.append(data[:n])
			}
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) && !errors.Is(readErr, os.ErrClosed) && !strings.Contains(readErr.Error(), "input/output error") {
					process.output.append([]byte("terminal read failed"))
				}
				return
			}
		}
	}()
	go func() { process.done <- command.Wait() }()
	t.Cleanup(func() { _ = process.close() })
	return process
}

func (p *tuiPTYProcess) waitFor(timeout time.Duration, value string) bool {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if p.output.contains(value) {
			return true
		}
		select {
		case <-deadline.C:
			return false
		case <-ticker.C:
		}
	}
}

func (p *tuiPTYProcess) send(value string) error {
	_, err := io.WriteString(p.terminal, value)
	return err
}

func (p *tuiPTYProcess) wait(timeout time.Duration) error {
	select {
	case err := <-p.done:
		return err
	case <-time.After(timeout):
		return errors.New("TUI process did not stop before timeout")
	}
}

func (p *tuiPTYProcess) close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		closeErr = p.terminal.Close()
		if p.command.Process != nil {
			_ = p.command.Process.Kill()
		}
	})
	return closeErr
}
