package tui

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
)

// initialTailLines is the number of recent entries loaded on first open.
const initialTailLines = 200

// LogEntry represents a single parsed line from the AWF audit JSONL log.
type LogEntry struct {
	Timestamp    string         `json:"timestamp"`
	Event        string         `json:"event"`
	WorkflowName string         `json:"workflow_name"`
	ExecutionID  string         `json:"execution_id"`
	Status       string         `json:"status"`
	DurationMs   float64        `json:"duration_ms"`
	User         string         `json:"user"`
	Error        string         `json:"error"`
	Fields       map[string]any `json:"-"`
}

// LogLineMsg carries a newly-tailed log entry to the Update loop.
type LogLineMsg struct {
	Entry LogEntry
}

// LogBatchMsg carries multiple entries from a batch read (initial tail or follow).
type LogBatchMsg struct {
	Entries []LogEntry
}

// logRotationMsg is an internal message emitted when the log file is deleted
// or becomes inaccessible, so the tab can surface a notice to the user.
type logRotationMsg struct {
	path string
}

// Tailer implements a tail-follow pattern on a JSONL file:
//   - First call (Tail): reads the last N lines from end-of-file
//   - Subsequent calls (Follow): reads all new lines since last offset
type Tailer struct {
	path   string
	offset int64
	seeded bool
}

// NewTailer creates a Tailer for the given JSONL file path.
func NewTailer(path string) *Tailer {
	return &Tailer{path: path}
}

// parseLine parses a single AWF audit JSONL line into a LogEntry.
func parseLine(data []byte) (LogEntry, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return LogEntry{}, fmt.Errorf("invalid JSON: %w", err)
	}

	entry := LogEntry{Fields: raw}

	if v, ok := raw["timestamp"].(string); ok {
		entry.Timestamp = v
	}
	if v, ok := raw["event"].(string); ok {
		entry.Event = v
	}
	if v, ok := raw["workflow_name"].(string); ok {
		entry.WorkflowName = v
	}
	if v, ok := raw["execution_id"].(string); ok {
		entry.ExecutionID = v
	}
	if v, ok := raw["status"].(string); ok {
		entry.Status = v
	}
	if v, ok := raw["duration_ms"].(float64); ok {
		entry.DurationMs = v
	}
	if v, ok := raw["user"].(string); ok {
		entry.User = v
	}
	if v, ok := raw["error"].(string); ok {
		entry.Error = v
	}

	return entry, nil
}

// Tail returns a tea.Cmd that reads the last N lines from the file.
// Sets the offset to EOF so subsequent Follow calls only read new content.
func (t *Tailer) Tail() tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(t.path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return logRotationMsg{path: t.path}
		}
		defer f.Close()

		entries := tailLastLines(f, initialTailLines)

		end, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			return nil
		}
		t.offset = end
		t.seeded = true

		if len(entries) == 0 {
			return nil
		}
		return LogBatchMsg{Entries: entries}
	}
}

// Follow returns a tea.Cmd that reads all new lines appended since last read.
// Returns nil when no new data is available.
func (t *Tailer) Follow() tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(t.path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				t.offset = 0
				t.seeded = false
				return logRotationMsg{path: t.path}
			}
			t.offset = 0
			t.seeded = false
			return logRotationMsg{path: t.path}
		}
		defer f.Close()

		if _, seekErr := f.Seek(t.offset, io.SeekStart); seekErr != nil {
			return nil
		}

		var entries []LogEntry
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			t.offset += int64(len(line)) + 1
			entry, parseErr := parseLine(line)
			if parseErr != nil {
				continue
			}
			entries = append(entries, entry)
		}

		if len(entries) == 0 {
			return nil
		}
		return LogBatchMsg{Entries: entries}
	}
}

// Next returns the appropriate command: Tail on first call, Follow afterwards.
func (t *Tailer) Next() tea.Cmd {
	if !t.seeded {
		return t.Tail()
	}
	return t.Follow()
}

// tailLastLines reads the last n lines from a file using backward scanning.
func tailLastLines(f *os.File, n int) []LogEntry {
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return nil
	}

	size := info.Size()
	buf := make([]byte, 0, min(size, 256*1024))
	readSize := int64(min(size, 256*1024))

	// Read the last chunk of the file.
	offset := size - readSize
	chunk := make([]byte, readSize)
	if _, err := f.ReadAt(chunk, offset); err != nil && !errors.Is(err, io.EOF) {
		return nil
	}
	buf = append(buf, chunk...)

	// If we didn't get enough lines, read a bigger chunk.
	lines := splitLines(buf)
	for len(lines) < n && offset > 0 {
		extra := int64(min(offset, 256*1024))
		offset -= extra
		prev := make([]byte, extra) //nolint:prealloc // chunk size varies per iteration
		if _, err := f.ReadAt(prev, offset); err != nil && !errors.Is(err, io.EOF) {
			break
		}
		buf = append(prev, buf...)
		lines = splitLines(buf)
	}

	// Take the last n lines.
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	entries := make([]LogEntry, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		entry, parseErr := parseLine(line)
		if parseErr != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// splitLines splits a byte slice into non-empty lines.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	scanner := bufio.NewScanner(bytesReader(data))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 {
			cp := make([]byte, len(line))
			copy(cp, line)
			lines = append(lines, cp)
		}
	}
	return lines
}

// bytesReader wraps []byte as an io.Reader.
func bytesReader(data []byte) io.Reader {
	return &byteSliceReader{data: data}
}

type byteSliceReader struct {
	data []byte
	pos  int
}

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
