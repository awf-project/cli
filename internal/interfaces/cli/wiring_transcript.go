package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/transcript"
)

// WireTranscript builds a transcript Recorder at storage/transcripts/<runID>.jsonl
// and returns the recorder along with a cleanup closure that calls Close().
// The storageRoot is the parent directory for transcripts (typically cfg.StoragePath).
// Returns the recorder, a cleanup function, and any error.
func WireTranscript(runID, storageRoot string) (ports.Recorder, func() error, error) {
	transcriptDir := filepath.Join(storageRoot, "transcripts")
	if err := os.MkdirAll(transcriptDir, 0o750); err != nil {
		return nil, nil, fmt.Errorf("creating transcripts directory: %w", err)
	}

	transcriptPath := filepath.Join(transcriptDir, runID+".jsonl")

	rec, err := transcript.NewRecorder(transcriptPath)
	if err != nil {
		return nil, nil, fmt.Errorf("creating transcript recorder: %w", err)
	}

	cleanup := rec.Close

	return rec, cleanup, nil
}

// NewRecorderFactory returns a ports.RecorderFactory backed by the transcript
// infrastructure Recorder. It is used by ExecutionService to create one child
// recorder per sub-run for F106 sub-workflow transcript linkage. The parent
// directory of the path passed to the factory must already exist.
func NewRecorderFactory() ports.RecorderFactory {
	return func(path string) (ports.Recorder, error) {
		return transcript.NewRecorder(path)
	}
}

// AttachMirrorSubscriber attaches a debug mirror subscriber to the recorder.
// When mirrorPath is non-empty, it subscribes to recorder events and writes them to mirrorPath.
// Returns a cancel function that should be called on shutdown.
// When mirrorPath is empty, returns a no-op cancel function.
func AttachMirrorSubscriber(rec ports.Recorder, mirrorPath string) func() {
	if mirrorPath == "" || rec == nil {
		return func() {}
	}

	ch, cancel := rec.Subscribe()

	go func() {
		f, err := os.OpenFile(mirrorPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // caller-controlled debug path
		if err != nil {
			// Unsubscribe so the fanout stops buffering (and logging drops) for a
			// subscriber that will never drain its channel, and drain any already-queued
			// events to let the buffered channel be garbage-collected.
			cancel()
			for range ch { //nolint:revive // intentional drain of the closed channel
			}
			return
		}
		defer f.Close() //nolint:errcheck // best-effort debug mirror

		enc := json.NewEncoder(f)
		for event := range ch {
			_ = enc.Encode(event) //nolint:errcheck // best-effort debug mirror
		}
	}()

	return cancel
}
