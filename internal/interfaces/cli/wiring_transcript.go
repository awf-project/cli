package cli

import (
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
// When mirrorPath is non-empty, recorder events are written to mirrorPath; when empty,
// returns a no-op cancel. The actual subscription lives in the transcript infrastructure
// (transcript.MirrorToFile) so the interface layer holds no direct recorder.Subscribe()
// call, preserving the SC-001 sole-subscriber invariant.
func AttachMirrorSubscriber(rec ports.Recorder, mirrorPath string) func() {
	return transcript.MirrorToFile(rec, mirrorPath)
}
