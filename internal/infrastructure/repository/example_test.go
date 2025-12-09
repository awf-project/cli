package repository

import (
	"context"
	"testing"
)

func TestLoadExampleWorkflow(t *testing.T) {
	repo := NewYAMLRepository("../../../configs/workflows")

	wf, err := repo.Load(context.Background(), "example-simple")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if wf == nil {
		t.Fatal("Load() returned nil workflow")
	}

	if wf.Name != "analyze-file" {
		t.Errorf("Name = %q, want %q", wf.Name, "analyze-file")
	}
	if wf.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", wf.Version, "1.0.0")
	}
	if len(wf.Inputs) != 2 {
		t.Errorf("Inputs count = %d, want 2", len(wf.Inputs))
	}
	if len(wf.Steps) != 5 {
		t.Errorf("Steps count = %d, want 5", len(wf.Steps))
	}

	// Check that hooks are loaded
	if len(wf.Hooks.WorkflowStart) != 1 {
		t.Errorf("WorkflowStart hooks = %d, want 1", len(wf.Hooks.WorkflowStart))
	}

	// Check capture config
	extract, ok := wf.GetStep("extract")
	if !ok {
		t.Fatal("extract step not found")
	}
	if extract.Capture == nil {
		t.Error("extract.Capture is nil")
	} else if extract.Capture.Stdout != "file_content" {
		t.Errorf("extract.Capture.Stdout = %q, want %q", extract.Capture.Stdout, "file_content")
	}
}
