package ui

import (
	"bufio"
	"context"
	"strings"
)

// readLineWithContext reads a line from reader, returning early if ctx is cancelled.
// Uses goroutine+channel+select so blocking I/O does not outlive context cancellation.
// One goroutine may leak per cancelled read (~2KB); acceptable per ADR-0013.
func readLineWithContext(ctx context.Context, reader *bufio.Reader) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	type result struct {
		line string
		err  error
	}

	ch := make(chan result, 1)

	go func() {
		line, err := reader.ReadString('\n')
		ch <- result{strings.TrimRight(line, "\r\n"), err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		return r.line, r.err
	}
}
