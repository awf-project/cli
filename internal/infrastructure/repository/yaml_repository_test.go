package repository

import (
	"context"
	"os"
	"testing"

	"github.com/vanoix/awf/internal/domain/workflow"
)

const fixturesPath = "../../../tests/fixtures/workflows"

func TestYAMLRepository_Load_ValidSimple(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "valid-simple")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if wf == nil {
		t.Fatal("Load() returned nil workflow")
	}

	if wf.Name != "simple-workflow" {
		t.Errorf("Name = %q, want %q", wf.Name, "simple-workflow")
	}
	if wf.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", wf.Version, "1.0.0")
	}
	if wf.Initial != "start" {
		t.Errorf("Initial = %q, want %q", wf.Initial, "start")
	}
	if len(wf.Steps) != 3 {
		t.Errorf("Steps count = %d, want 3", len(wf.Steps))
	}

	// Check inputs (added for interpolation testing)
	if len(wf.Inputs) != 2 {
		t.Errorf("Inputs count = %d, want 2", len(wf.Inputs))
	}

	// Check start step
	start, ok := wf.GetStep("start")
	if !ok {
		t.Fatal("start step not found")
	}
	if start.Type != workflow.StepTypeCommand {
		t.Errorf("start.Type = %v, want %v", start.Type, workflow.StepTypeCommand)
	}
	expectedCmd := `echo "hello world"`
	if start.Command != expectedCmd {
		t.Errorf("start.Command = %q, want %q", start.Command, expectedCmd)
	}
	if start.OnSuccess != "done" {
		t.Errorf("start.OnSuccess = %q, want %q", start.OnSuccess, "done")
	}
}

func TestYAMLRepository_Load_ValidFull(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "valid-full")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if wf == nil {
		t.Fatal("Load() returned nil workflow")
	}

	// Check metadata
	if wf.Name != "full-workflow" {
		t.Errorf("Name = %q, want %q", wf.Name, "full-workflow")
	}
	if len(wf.Tags) != 2 {
		t.Errorf("Tags count = %d, want 2", len(wf.Tags))
	}

	// Check inputs
	if len(wf.Inputs) != 2 {
		t.Errorf("Inputs count = %d, want 2", len(wf.Inputs))
	}
	if wf.Inputs[0].Name != "file_path" {
		t.Errorf("Inputs[0].Name = %q, want %q", wf.Inputs[0].Name, "file_path")
	}
	if wf.Inputs[0].Validation == nil {
		t.Error("Inputs[0].Validation is nil")
	} else if !wf.Inputs[0].Validation.FileExists {
		t.Error("Inputs[0].Validation.FileExists = false, want true")
	}

	// Check env
	if len(wf.Env) != 2 {
		t.Errorf("Env count = %d, want 2", len(wf.Env))
	}

	// Check workflow hooks
	if len(wf.Hooks.WorkflowStart) != 1 {
		t.Errorf("Hooks.WorkflowStart count = %d, want 1", len(wf.Hooks.WorkflowStart))
	}

	// Check extract step with retry and capture
	extract, ok := wf.GetStep("extract")
	if !ok {
		t.Fatal("extract step not found")
	}
	if extract.Capture == nil {
		t.Error("extract.Capture is nil")
	} else if extract.Capture.Stdout != "file_content" {
		t.Errorf("extract.Capture.Stdout = %q, want %q", extract.Capture.Stdout, "file_content")
	}
	if extract.Retry == nil {
		t.Error("extract.Retry is nil")
	} else {
		if extract.Retry.MaxAttempts != 3 {
			t.Errorf("extract.Retry.MaxAttempts = %d, want 3", extract.Retry.MaxAttempts)
		}
		if extract.Retry.Backoff != "exponential" {
			t.Errorf("extract.Retry.Backoff = %q, want %q", extract.Retry.Backoff, "exponential")
		}
	}

	// Check validate step with hooks
	validate, ok := wf.GetStep("validate")
	if !ok {
		t.Fatal("validate step not found")
	}
	if len(validate.Hooks.Pre) != 1 {
		t.Errorf("validate.Hooks.Pre count = %d, want 1", len(validate.Hooks.Pre))
	}

	// Check parallel step
	process, ok := wf.GetStep("process")
	if !ok {
		t.Fatal("process step not found")
	}
	if process.Type != workflow.StepTypeParallel {
		t.Errorf("process.Type = %v, want %v", process.Type, workflow.StepTypeParallel)
	}
	if len(process.Branches) != 2 {
		t.Errorf("process.Branches count = %d, want 2", len(process.Branches))
	}
	if process.Strategy != "all_succeed" {
		t.Errorf("process.Strategy = %q, want %q", process.Strategy, "all_succeed")
	}
}

func TestYAMLRepository_Load_NonExistent(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "nonexistent")
	if err != nil {
		t.Errorf("Load() error = %v, want nil", err)
	}
	if wf != nil {
		t.Errorf("Load() = %v, want nil", wf)
	}
}

func TestYAMLRepository_Load_InvalidSyntax(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	_, err := repo.Load(context.Background(), "invalid-syntax")
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}

	// Check it's a ParseError
	parseErr, ok := err.(*ParseError)
	if !ok {
		t.Errorf("error type = %T, want *ParseError", err)
	} else if parseErr.File == "" {
		t.Error("ParseError.File is empty")
	}
}

func TestYAMLRepository_Load_MissingName(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	_, err := repo.Load(context.Background(), "invalid-missing-name")
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}

	parseErr, ok := err.(*ParseError)
	if !ok {
		t.Errorf("error type = %T, want *ParseError", err)
	} else if parseErr.Field != "name" {
		t.Errorf("ParseError.Field = %q, want %q", parseErr.Field, "name")
	}
}

func TestYAMLRepository_Load_BadStateRef(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	_, err := repo.Load(context.Background(), "invalid-bad-state-ref")
	if err == nil {
		t.Fatal("Load() error = nil, want error for bad state reference")
	}
	// This should fail domain validation for unreachable terminal
}

func TestYAMLRepository_List(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	names, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should find at least the valid workflows
	if len(names) < 2 {
		t.Errorf("List() returned %d names, want at least 2", len(names))
	}

	// Check valid-simple is in the list
	found := false
	for _, name := range names {
		if name == "valid-simple" {
			found = true
			break
		}
	}
	if !found {
		t.Error("List() did not include valid-simple")
	}
}

func TestYAMLRepository_Exists(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	tests := []struct {
		name   string
		exists bool
	}{
		{"valid-simple", true},
		{"valid-full", true},
		{"nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := repo.Exists(context.Background(), tt.name)
			if err != nil {
				t.Fatalf("Exists() error = %v", err)
			}
			if exists != tt.exists {
				t.Errorf("Exists() = %v, want %v", exists, tt.exists)
			}
		})
	}
}

func TestYAMLRepository_Load_WithYamlExtension(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	// Should work with .yaml extension
	wf, err := repo.Load(context.Background(), "valid-simple.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if wf == nil {
		t.Fatal("Load() returned nil workflow")
	}
	if wf.Name != "simple-workflow" {
		t.Errorf("Name = %q, want %q", wf.Name, "simple-workflow")
	}
}

func TestYAMLRepository_Load_WithDir(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "valid-with-dir")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if wf == nil {
		t.Fatal("Load() returned nil workflow")
	}

	// Check build step has absolute dir
	build, ok := wf.GetStep("build")
	if !ok {
		t.Fatal("build step not found")
	}
	if build.Dir != "/tmp/project" {
		t.Errorf("build.Dir = %q, want %q", build.Dir, "/tmp/project")
	}

	// Check test step has interpolated dir
	test, ok := wf.GetStep("test")
	if !ok {
		t.Fatal("test step not found")
	}
	if test.Dir != "{{inputs.project_path}}" {
		t.Errorf("test.Dir = %q, want %q", test.Dir, "{{inputs.project_path}}")
	}

	// Check done step has no dir (terminal)
	done, ok := wf.GetStep("done")
	if !ok {
		t.Fatal("done step not found")
	}
	if done.Dir != "" {
		t.Errorf("done.Dir = %q, want empty", done.Dir)
	}
}

func TestYAMLRepository_Timeout(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "valid-full")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	validate, ok := wf.GetStep("validate")
	if !ok {
		t.Fatal("validate step not found")
	}

	// 5s should be parsed as 5 seconds
	if validate.Timeout != 5 {
		t.Errorf("validate.Timeout = %d, want 5", validate.Timeout)
	}

	extract, ok := wf.GetStep("extract")
	if !ok {
		t.Fatal("extract step not found")
	}

	// 30s should be parsed as 30 seconds
	if extract.Timeout != 30 {
		t.Errorf("extract.Timeout = %d, want 30", extract.Timeout)
	}
}

// Integration test to ensure repository implements the port interface
func TestYAMLRepository_ImplementsInterface(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	// This compilation check ensures YAMLRepository implements WorkflowRepository
	var _ interface {
		Load(context.Context, string) (*workflow.Workflow, error)
		List(context.Context) ([]string, error)
		Exists(context.Context, string) (bool, error)
	} = repo
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		wantSec int
		wantErr bool
	}{
		{"5s", 5, false},
		{"30s", 30, false},
		{"2m", 120, false},
		{"1h", 3600, false},
		{"1m30s", 90, false},
		{"60", 60, false}, // plain integer as seconds
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			d, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && int(d.Seconds()) != tt.wantSec {
				t.Errorf("parseDuration(%q) = %v, want %d seconds", tt.input, d, tt.wantSec)
			}
		})
	}
}

// =============================================================================
// F037: Dynamic MaxIterations YAML Parsing Tests
// =============================================================================

func TestYAMLRepository_Load_LoopWithIntegerMaxIterations(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "loop-foreach")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if wf == nil {
		t.Fatal("Load() returned nil workflow")
	}

	// Get the for_each step
	processFiles, ok := wf.GetStep("process_files")
	if !ok {
		t.Fatal("process_files step not found")
	}

	if processFiles.Type != workflow.StepTypeForEach {
		t.Errorf("process_files.Type = %v, want %v", processFiles.Type, workflow.StepTypeForEach)
	}
	if processFiles.Loop == nil {
		t.Fatal("process_files.Loop is nil")
	}

	// Verify integer max_iterations parsing
	if processFiles.Loop.MaxIterations != 10 {
		t.Errorf("Loop.MaxIterations = %d, want 10", processFiles.Loop.MaxIterations)
	}
	if processFiles.Loop.MaxIterationsExpr != "" {
		t.Errorf("Loop.MaxIterationsExpr = %q, want empty", processFiles.Loop.MaxIterationsExpr)
	}
	if processFiles.Loop.IsMaxIterationsDynamic() {
		t.Error("Loop.IsMaxIterationsDynamic() = true, want false")
	}
}

func TestYAMLRepository_Load_LoopWithDynamicMaxIterations(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "loop-dynamic")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if wf == nil {
		t.Fatal("Load() returned nil workflow")
	}

	// Get the for_each step with dynamic max_iterations
	countLoop, ok := wf.GetStep("count_loop")
	if !ok {
		t.Fatal("count_loop step not found")
	}

	if countLoop.Type != workflow.StepTypeForEach {
		t.Errorf("count_loop.Type = %v, want %v", countLoop.Type, workflow.StepTypeForEach)
	}
	if countLoop.Loop == nil {
		t.Fatal("count_loop.Loop is nil")
	}

	// Verify dynamic max_iterations parsing
	if countLoop.Loop.MaxIterations != 0 {
		t.Errorf("Loop.MaxIterations = %d, want 0 (expression mode)", countLoop.Loop.MaxIterations)
	}
	// Go template syntax requires dot prefix for accessing map values
	expectedExpr := "{{.inputs.limit}}"
	if countLoop.Loop.MaxIterationsExpr != expectedExpr {
		t.Errorf("Loop.MaxIterationsExpr = %q, want %q", countLoop.Loop.MaxIterationsExpr, expectedExpr)
	}
	if !countLoop.Loop.IsMaxIterationsDynamic() {
		t.Error("Loop.IsMaxIterationsDynamic() = false, want true")
	}
}

func TestYAMLRepository_Load_LoopWithDynamicMaxIterationsFromEnv(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "loop-dynamic-env")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if wf == nil {
		t.Fatal("Load() returned nil workflow")
	}

	// Get the for_each step with env-based max_iterations
	processLoop, ok := wf.GetStep("process_loop")
	if !ok {
		t.Fatal("process_loop step not found")
	}

	if processLoop.Loop == nil {
		t.Fatal("process_loop.Loop is nil")
	}

	// Verify env-based expression parsing
	// Go template syntax requires dot prefix for accessing map values
	expectedExpr := "{{.env.LOOP_LIMIT}}"
	if processLoop.Loop.MaxIterationsExpr != expectedExpr {
		t.Errorf("Loop.MaxIterationsExpr = %q, want %q", processLoop.Loop.MaxIterationsExpr, expectedExpr)
	}
	if !processLoop.Loop.IsMaxIterationsDynamic() {
		t.Error("Loop.IsMaxIterationsDynamic() = false, want true")
	}
}

func TestYAMLRepository_Load_LoopWithArithmeticMaxIterations(t *testing.T) {
	repo := NewYAMLRepository(fixturesPath)

	wf, err := repo.Load(context.Background(), "loop-dynamic-arithmetic")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if wf == nil {
		t.Fatal("Load() returned nil workflow")
	}

	// Get the for_each step with arithmetic expression
	retryLoop, ok := wf.GetStep("retry_loop")
	if !ok {
		t.Fatal("retry_loop step not found")
	}

	if retryLoop.Loop == nil {
		t.Fatal("retry_loop.Loop is nil")
	}

	// Verify arithmetic expression parsing
	// Arithmetic uses separate templates: {{.inputs.pages}} * {{.inputs.retries_per_page}}
	expectedExpr := "{{.inputs.pages}} * {{.inputs.retries_per_page}}"
	if retryLoop.Loop.MaxIterationsExpr != expectedExpr {
		t.Errorf("Loop.MaxIterationsExpr = %q, want %q", retryLoop.Loop.MaxIterationsExpr, expectedExpr)
	}
	if !retryLoop.Loop.IsMaxIterationsDynamic() {
		t.Error("Loop.IsMaxIterationsDynamic() = false, want true")
	}
	if retryLoop.Loop.MaxIterations != 0 {
		t.Errorf("Loop.MaxIterations = %d, want 0 (expression mode)", retryLoop.Loop.MaxIterations)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
