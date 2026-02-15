package repository

import (
	"context"
	"path/filepath"
	"testing"
)

func TestYAMLRepository_Load_PopulatesSourceDir_AbsolutePath(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "valid-simple")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	absFixturePath, err := filepath.Abs(fixturesPath)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	if wf.SourceDir != absFixturePath {
		t.Errorf("SourceDir = %q, want %q", wf.SourceDir, absFixturePath)
	}
}

func TestYAMLRepository_Load_PopulatesSourceDir_WithYamlExtension(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "valid-full.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	absFixturePath, err := filepath.Abs(fixturesPath)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	if wf.SourceDir != absFixturePath {
		t.Errorf("SourceDir = %q, want %q", wf.SourceDir, absFixturePath)
	}
}

func TestYAMLRepository_Load_PopulatesSourceDir_MultipleWorkflows(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
	}{
		{"valid-simple", "valid-simple"},
		{"valid-full", "valid-full"},
		{"valid-with-dir", "valid-with-dir"},
		{"loop-foreach", "loop-foreach"},
	}

	repo := NewYAMLRepository(fixturesPath)
	absFixturePath, err := filepath.Abs(fixturesPath)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf, err := repo.Load(context.Background(), tt.workflowName)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if wf.SourceDir != absFixturePath {
				t.Errorf("SourceDir = %q, want %q", wf.SourceDir, absFixturePath)
			}
		})
	}
}

func TestYAMLRepository_Load_SourceDir_UsableForRelativePathResolution(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "valid-simple")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if wf.SourceDir == "" {
		t.Fatal("SourceDir should not be empty after Load()")
	}

	relativePromptPath := "prompts/analyze.md"
	resolvedPath := filepath.Join(wf.SourceDir, relativePromptPath)

	absFixturePath, err := filepath.Abs(fixturesPath)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	expectedPath := filepath.Join(absFixturePath, relativePromptPath)

	if resolvedPath != expectedPath {
		t.Errorf("resolved path = %q, want %q", resolvedPath, expectedPath)
	}
}

func TestYAMLRepository_Load_SourceDir_ExtractedFromFilePath(t *testing.T) {
	tests := []struct {
		name           string
		basePath       string
		workflowName   string
		expectedResult string
	}{
		{
			name:           "simple base path",
			basePath:       "/workflows",
			workflowName:   "test",
			expectedResult: "/workflows",
		},
		{
			name:           "nested base path",
			basePath:       "/home/user/.config/awf/workflows",
			workflowName:   "analyze",
			expectedResult: "/home/user/.config/awf/workflows",
		},
		{
			name:           "relative base path",
			basePath:       "configs/workflows",
			workflowName:   "deploy",
			expectedResult: "configs/workflows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tt.basePath, tt.workflowName+".yaml")
			actualSourceDir := filepath.Dir(filePath)

			if actualSourceDir != tt.expectedResult {
				t.Errorf("filepath.Dir(%q) = %q, want %q", filePath, actualSourceDir, tt.expectedResult)
			}
		})
	}
}
