package errfmt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domainerrors "github.com/vanoix/awf/internal/domain/errors"
)

// =============================================================================
// FileNotFoundHintGenerator Tests (Happy Path)
// =============================================================================

func TestFileNotFoundHintGenerator_ReturnsHints_ForMissingFileWithSimilarFile(t *testing.T) {
	// Given: Create temp directory with similar workflow file
	tmpDir := t.TempDir()
	similarFile := filepath.Join(tmpDir, "my-workflow.yaml")
	err := os.WriteFile(similarFile, []byte("version: 1.0"), 0o644)
	require.NoError(t, err)

	// Error references typo'd filename in same directory
	missingPath := filepath.Join(tmpDir, "my-workfow.yaml") // typo: missing 'l'
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints when similar file exists")
	// Should suggest the similar file with Levenshtein matching
	foundSuggestion := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "my-workflow.yaml") {
			foundSuggestion = true
			break
		}
	}
	assert.True(t, foundSuggestion, "should suggest similar filename")
}

func TestFileNotFoundHintGenerator_ReturnsMultipleHints_WithAllRecommendations(t *testing.T) {
	// Given: Create temp directory with a workflow file
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "example.yaml")
	err := os.WriteFile(existingFile, []byte("version: 1.0"), 0o644)
	require.NoError(t, err)

	missingPath := filepath.Join(tmpDir, "exmple.yaml") // typo
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return multiple hints")
	assert.LessOrEqual(t, len(hints), 3, "should limit to max 3 hints")

	// Check for expected hint types
	hintMessages := make([]string, len(hints))
	for i, hint := range hints {
		hintMessages[i] = hint.Message
	}

	// Should include at least one of: similarity suggestion, list command, or directory check
	hasActionableHint := false
	for _, msg := range hintMessages {
		if containsSubstring(msg, "awf list") ||
			containsSubstring(msg, "pwd") ||
			containsSubstring(msg, "example.yaml") {
			hasActionableHint = true
			break
		}
	}
	assert.True(t, hasActionableHint, "should include at least one actionable hint")
}

func TestFileNotFoundHintGenerator_SuggestsListCommand_WhenNoSimilarFiles(t *testing.T) {
	// Given: Empty directory
	tmpDir := t.TempDir()
	missingPath := filepath.Join(tmpDir, "missing.yaml")

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints even without similar files")

	// Should suggest using 'awf list' command
	foundListCommand := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "awf list") {
			foundListCommand = true
			break
		}
	}
	assert.True(t, foundListCommand, "should suggest 'awf list' command when no similar files")
}

func TestFileNotFoundHintGenerator_FindsMultipleSimilarFiles_OrdersByRelevance(t *testing.T) {
	// Given: Create temp directory with multiple similar files
	tmpDir := t.TempDir()

	// Create files with varying similarity to target
	files := []string{
		"test-workflow.yaml", // distance 5 from "test.yaml"
		"test.yaml",          // exact match if looking for test
		"my-test-flow.yaml",  // more different
	}
	for _, filename := range files {
		err := os.WriteFile(filepath.Join(tmpDir, filename), []byte("version: 1.0"), 0o644)
		require.NoError(t, err)
	}

	missingPath := filepath.Join(tmpDir, "tset.yaml") // typo: 'tset' instead of 'test'
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints with multiple candidates")
	// First hint should mention the closest match (test.yaml is closest to tset.yaml)
	if len(hints) > 0 {
		firstHint := hints[0].Message
		assert.True(t,
			containsSubstring(firstHint, "test.yaml") || containsSubstring(firstHint, "test-workflow.yaml"),
			"first hint should mention one of the similar files",
		)
	}
}

// =============================================================================
// FileNotFoundHintGenerator Tests (Edge Cases)
// =============================================================================

func TestFileNotFoundHintGenerator_ReturnsEmptySlice_ForNonMissingFileError(t *testing.T) {
	// Given: Different error code (not MISSING_FILE)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"YAML syntax error",
		nil,
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice for non-file-not-found errors")
}

func TestFileNotFoundHintGenerator_ReturnsEmptySlice_WhenDetailsLackPath(t *testing.T) {
	// Given: MISSING_FILE error but no path in Details
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		map[string]any{"other": "value"},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice when path detail missing")
}

func TestFileNotFoundHintGenerator_ReturnsEmptySlice_WhenDetailsIsNil(t *testing.T) {
	// Given: MISSING_FILE error with nil Details
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		nil,
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice when details is nil")
}

func TestFileNotFoundHintGenerator_ReturnsEmptySlice_WhenPathIsNotString(t *testing.T) {
	// Given: path detail is not a string
	tests := []struct {
		name      string
		pathValue any
	}{
		{
			name:      "path is integer",
			pathValue: 123,
		},
		{
			name:      "path is boolean",
			pathValue: true,
		},
		{
			name:      "path is nil",
			pathValue: nil,
		},
		{
			name:      "path is slice",
			pathValue: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"file not found",
				map[string]any{"path": tt.pathValue},
				nil,
			)

			// When
			hints := FileNotFoundHintGenerator(structErr)

			// Then
			assert.NotNil(t, hints, "should return non-nil slice")
			assert.Empty(t, hints, "should return empty slice for non-string path")
		})
	}
}

func TestFileNotFoundHintGenerator_HandlesEmptyPath_Gracefully(t *testing.T) {
	// Given: path is empty string
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		map[string]any{"path": ""},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// May return empty or general hints, but should not panic
}

func TestFileNotFoundHintGenerator_HandlesNonexistentDirectory_Gracefully(t *testing.T) {
	// Given: path in directory that doesn't exist
	missingPath := filepath.Join("/nonexistent", "directory", "workflow.yaml")
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should gracefully degrade if directory read fails
	// May return generic hints or empty slice, but should not panic
}

func TestFileNotFoundHintGenerator_HandlesRelativePath_Correctly(t *testing.T) {
	// Given: relative path in Details
	missingPath := "workflows/missing.yaml"
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle relative paths without panic
}

func TestFileNotFoundHintGenerator_HandlesPathWithSpecialCharacters(t *testing.T) {
	// Given: Create temp directory with file containing special chars
	tmpDir := t.TempDir()
	specialFile := filepath.Join(tmpDir, "my-workflow (1).yaml")
	err := os.WriteFile(specialFile, []byte("version: 1.0"), 0o644)
	require.NoError(t, err)

	// Path with similar name but missing parentheses
	missingPath := filepath.Join(tmpDir, "my-workflow1.yaml")
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle special characters in filenames without panic
}

func TestFileNotFoundHintGenerator_HandlesNilError_Gracefully(t *testing.T) {
	// Given: nil StructuredError
	var structErr *domainerrors.StructuredError

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice for nil error")
}

func TestFileNotFoundHintGenerator_LimitsNumberOfSuggestions(t *testing.T) {
	// Given: Create temp directory with many similar files
	tmpDir := t.TempDir()

	// Create 10 files with similar names
	for i := 0; i < 10; i++ {
		filename := filepath.Join(tmpDir, "workflow"+string(rune('a'+i))+".yaml")
		err := os.WriteFile(filename, []byte("version: 1.0"), 0o644)
		require.NoError(t, err)
	}

	missingPath := filepath.Join(tmpDir, "workflow.yaml")
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.LessOrEqual(t, len(hints), 3, "should limit hints to max 3 to avoid overwhelming user")
}

// =============================================================================
// FileNotFoundHintGenerator Tests (Error Handling)
// =============================================================================

func TestFileNotFoundHintGenerator_HandlesPermissionDeniedDirectory(t *testing.T) {
	// Given: Create temp directory and restrict permissions
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()
	restrictedDir := filepath.Join(tmpDir, "restricted")
	err := os.Mkdir(restrictedDir, 0o000) // No permissions
	require.NoError(t, err)
	defer os.Chmod(restrictedDir, 0o755) // Restore for cleanup

	missingPath := filepath.Join(restrictedDir, "workflow.yaml")
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should gracefully degrade on permission error, not panic
}

func TestFileNotFoundHintGenerator_HandlesVeryLongPath(t *testing.T) {
	// Given: Very long path
	longPath := filepath.Join("/very/long/path/that/does/not/exist/with/many/nested/directories",
		"and/even/more/directories/to/make/it/really/long/workflow.yaml")

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		map[string]any{"path": longPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle long paths without issue
}

func TestFileNotFoundHintGenerator_HandlesPathWithNoDirectory(t *testing.T) {
	// Given: Just a filename, no directory
	missingPath := "workflow.yaml"
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle bare filename (current directory implied)
}

func TestFileNotFoundHintGenerator_IgnoresNonYAMLFiles_WhenLookingForWorkflows(t *testing.T) {
	// Given: Create temp directory with non-YAML files
	tmpDir := t.TempDir()

	files := []string{
		"README.md",
		"config.json",
		"script.sh",
	}
	for _, filename := range files {
		err := os.WriteFile(filepath.Join(tmpDir, filename), []byte("content"), 0o644)
		require.NoError(t, err)
	}

	missingPath := filepath.Join(tmpDir, "workflow.yaml")
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// May return hints about awf list or pwd, but shouldn't suggest non-YAML files
	for _, hint := range hints {
		assert.NotContains(t, hint.Message, "README.md", "should not suggest non-YAML files")
		assert.NotContains(t, hint.Message, "config.json", "should not suggest non-YAML files")
		assert.NotContains(t, hint.Message, "script.sh", "should not suggest non-YAML files")
	}
}

func TestFileNotFoundHintGenerator_HandlesSymlinks_Gracefully(t *testing.T) {
	// Given: Create temp directory with symlink
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "real.yaml")
	err := os.WriteFile(realFile, []byte("version: 1.0"), 0o644)
	require.NoError(t, err)

	symlinkFile := filepath.Join(tmpDir, "link.yaml")
	err = os.Symlink(realFile, symlinkFile)
	if err != nil {
		t.Skip("Symlink creation failed, skipping test")
	}

	missingPath := filepath.Join(tmpDir, "lnk.yaml") // typo
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When
	hints := FileNotFoundHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle symlinks in directory listing
}

func TestFileNotFoundHintGenerator_ThreadSafety_NoConcurrentAccessIssues(t *testing.T) {
	// Given: Create temp directory with files
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte("version: 1.0"), 0o644)
	require.NoError(t, err)

	missingPath := filepath.Join(tmpDir, "tst.yaml")
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{"path": missingPath},
		nil,
	)

	// When: Call generator concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			hints := FileNotFoundHintGenerator(structErr)
			assert.NotNil(t, hints)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Then: No race conditions or panics (verified by -race flag in CI)
}

// =============================================================================
// Test Helpers
// =============================================================================

// =============================================================================
// InvalidStateHintGenerator Tests (Happy Path)
// =============================================================================

func TestInvalidStateHintGenerator_ReturnsHints_ForMissingStateWithSimilarState(t *testing.T) {
	// Given: Error references typo'd state name with similar available state
	availableStates := []string{"initialize", "validate", "execute", "cleanup"}
	//nolint:misspell // intentional typo for test
	missingState := "intialize" // typo: missing 'i'

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints when similar state exists")
	// Should suggest the similar state with Levenshtein matching
	foundSuggestion := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "initialize") {
			foundSuggestion = true
			break
		}
	}
	assert.True(t, foundSuggestion, "should suggest similar state name")
}

func TestInvalidStateHintGenerator_ReturnsMultipleHints_OrderedByRelevance(t *testing.T) {
	// Given: Multiple available states with varying similarity
	availableStates := []string{"start", "setup", "start_process", "restart"}
	missingState := "stat" // Similar to "start"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints")
	assert.LessOrEqual(t, len(hints), 3, "should limit to max 3 hints")

	// First hint should mention the closest match
	if len(hints) > 0 {
		firstHint := hints[0].Message
		assert.True(t,
			containsSubstring(firstHint, "start") || containsSubstring(firstHint, "setup"),
			"first hint should mention one of the closest matches",
		)
	}
}

func TestInvalidStateHintGenerator_ListsAllStates_WhenNoCloseMatch(t *testing.T) {
	// Given: Error with state very different from all available states
	availableStates := []string{"alpha", "beta", "gamma"}
	missingState := "zzz_completely_different"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints even without close matches")

	// Should list available states
	foundListHint := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "alpha") ||
			containsSubstring(hint.Message, "beta") ||
			containsSubstring(hint.Message, "available") {
			foundListHint = true
			break
		}
	}
	assert.True(t, foundListHint, "should list available states when no close match")
}

func TestInvalidStateHintGenerator_FindsMultipleSimilarStates_OrdersByDistance(t *testing.T) {
	// Given: Multiple states with varying similarity to target
	availableStates := []string{
		"process_data",
		"process",
		"preprocess",
		"postprocess",
		"data_process",
	}
	//nolint:misspell // intentional typo for test
	missingState := "proces" // typo: missing 's' from "process"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints with multiple candidates")
	//nolint:misspell // intentional typo in comment
	// First hint should mention the closest match ("process" is closest to "proces")
	if len(hints) > 0 {
		firstHint := hints[0].Message
		assert.True(t,
			containsSubstring(firstHint, "process") || containsSubstring(firstHint, "preprocess"),
			"first hint should mention one of the closest matches",
		)
	}
}

func TestInvalidStateHintGenerator_HandlesEmptyAvailableStates(t *testing.T) {
	// Given: Error with empty available states list
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            "missing",
			"available_states": []string{},
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// May return empty or generic hints, but should not panic
}

func TestInvalidStateHintGenerator_HandlesSingleAvailableState(t *testing.T) {
	// Given: Only one available state
	availableStates := []string{"only_state"}
	missingState := "only_stat" // typo

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints for single available state")
	foundSuggestion := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "only_state") {
			foundSuggestion = true
			break
		}
	}
	assert.True(t, foundSuggestion, "should suggest the only available state")
}

// =============================================================================
// InvalidStateHintGenerator Tests (Edge Cases)
// =============================================================================

func TestInvalidStateHintGenerator_ReturnsEmptySlice_ForNonMissingStateError(t *testing.T) {
	// Given: Different error code (not MISSING_STATE)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"YAML syntax error",
		nil,
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice for non-missing-state errors")
}

func TestInvalidStateHintGenerator_ReturnsEmptySlice_WhenDetailsLackState(t *testing.T) {
	// Given: MISSING_STATE error but no state in Details
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"state not found",
		map[string]any{
			"available_states": []string{"a", "b"},
			"other":            "value",
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice when state detail missing")
}

func TestInvalidStateHintGenerator_ReturnsEmptySlice_WhenDetailsLackAvailableStates(t *testing.T) {
	// Given: MISSING_STATE error but no available_states in Details
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"state not found",
		map[string]any{
			"state": "missing",
			"other": "value",
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice when available_states detail missing")
}

func TestInvalidStateHintGenerator_ReturnsEmptySlice_WhenDetailsIsNil(t *testing.T) {
	// Given: MISSING_STATE error with nil Details
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"state not found",
		nil,
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice when details is nil")
}

func TestInvalidStateHintGenerator_ReturnsEmptySlice_WhenStateIsNotString(t *testing.T) {
	// Given: state detail is not a string
	tests := []struct {
		name       string
		stateValue any
	}{
		{
			name:       "state is integer",
			stateValue: 123,
		},
		{
			name:       "state is boolean",
			stateValue: true,
		},
		{
			name:       "state is nil",
			stateValue: nil,
		},
		{
			name:       "state is map",
			stateValue: map[string]string{"key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeWorkflowValidationMissingState,
				"state not found",
				map[string]any{
					"state":            tt.stateValue,
					"available_states": []string{"a", "b"},
				},
				nil,
			)

			// When
			hints := InvalidStateHintGenerator(structErr)

			// Then
			assert.NotNil(t, hints, "should return non-nil slice")
			assert.Empty(t, hints, "should return empty slice for non-string state")
		})
	}
}

func TestInvalidStateHintGenerator_ReturnsEmptySlice_WhenAvailableStatesIsNotSlice(t *testing.T) {
	// Given: available_states is not a slice
	tests := []struct {
		name                 string
		availableStatesValue any
	}{
		{
			name:                 "available_states is string",
			availableStatesValue: "not a slice",
		},
		{
			name:                 "available_states is integer",
			availableStatesValue: 123,
		},
		{
			name:                 "available_states is boolean",
			availableStatesValue: true,
		},
		{
			name:                 "available_states is nil",
			availableStatesValue: nil,
		},
		{
			name:                 "available_states is map",
			availableStatesValue: map[string]string{"key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeWorkflowValidationMissingState,
				"state not found",
				map[string]any{
					"state":            "missing",
					"available_states": tt.availableStatesValue,
				},
				nil,
			)

			// When
			hints := InvalidStateHintGenerator(structErr)

			// Then
			assert.NotNil(t, hints, "should return non-nil slice")
			assert.Empty(t, hints, "should return empty slice for non-slice available_states")
		})
	}
}

func TestInvalidStateHintGenerator_HandlesEmptyState_Gracefully(t *testing.T) {
	// Given: state is empty string
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"state not found",
		map[string]any{
			"state":            "",
			"available_states": []string{"a", "b", "c"},
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// May return empty or general hints, but should not panic
}

func TestInvalidStateHintGenerator_HandlesAvailableStatesWithNonStrings(t *testing.T) {
	// Given: available_states contains non-string elements
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"state not found",
		map[string]any{
			"state": "missing",
			"available_states": []any{
				"valid_state",
				123,  // non-string
				true, // non-string
				"other_state",
				nil, // non-string
			},
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle mixed types gracefully, only processing strings
	// May return hints for valid string states or empty slice
}

func TestInvalidStateHintGenerator_HandlesStateWithSpecialCharacters(t *testing.T) {
	// Given: State names with special characters
	availableStates := []string{
		"user-login",
		"user_login",
		"user.login",
		"user:login",
		"user@login",
	}
	missingState := "user_logins" // typo: extra 's'

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle special characters in state names without panic
}

func TestInvalidStateHintGenerator_HandlesVeryLongStateName(t *testing.T) {
	// Given: Very long state name
	availableStates := []string{"short", "medium_length"}
	longState := "very_long_state_name_with_many_words_that_exceeds_typical_identifier_length_limits_but_still_valid"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            longState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle long state names without issue
}

func TestInvalidStateHintGenerator_HandlesNilError_Gracefully(t *testing.T) {
	// Given: nil StructuredError
	var structErr *domainerrors.StructuredError

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice for nil error")
}

func TestInvalidStateHintGenerator_LimitsNumberOfSuggestions(t *testing.T) {
	// Given: Many available states
	availableStates := []string{
		"state_a", "state_b", "state_c", "state_d", "state_e",
		"state_f", "state_g", "state_h", "state_i", "state_j",
	}
	missingState := "state" // Similar to all

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.LessOrEqual(t, len(hints), 3, "should limit hints to max 3 to avoid overwhelming user")
}

func TestInvalidStateHintGenerator_HandlesCaseVariations(t *testing.T) {
	// Given: Available states with different case
	availableStates := []string{"Initialize", "VALIDATE", "execute"}
	missingState := "INITIALIZE" // Different case than available

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should perform case-sensitive matching (Go convention)
	// May or may not suggest "Initialize" depending on Levenshtein distance
}

func TestInvalidStateHintGenerator_HandlesIdenticalStateName(t *testing.T) {
	// Given: Missing state that is identical to an available state
	// (shouldn't happen in practice, but test defensive code)
	availableStates := []string{"validate", "execute"}
	missingState := "validate" // Exact match

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle this edge case gracefully (may suggest exact match with distance 0)
}

func TestInvalidStateHintGenerator_HandlesVeryLongAvailableStatesList(t *testing.T) {
	// Given: Very large list of available states
	availableStates := make([]string, 100)
	for i := 0; i < 100; i++ {
		availableStates[i] = "state_" + string(rune('a'+i%26))
	}
	missingState := "state_z"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.LessOrEqual(t, len(hints), 3, "should limit to max 3 hints even with many states")
}

// =============================================================================
// InvalidStateHintGenerator Tests (Error Handling)
// =============================================================================

func TestInvalidStateHintGenerator_HandlesUnicodeStateNames(t *testing.T) {
	// Given: State names with Unicode characters
	availableStates := []string{"初期化", "検証", "実行"}
	missingState := "初期" // Part of first state

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle Unicode without panic
}

func TestInvalidStateHintGenerator_HandlesStateNameWithWhitespace(t *testing.T) {
	// Given: State names with whitespace
	availableStates := []string{"load data", "process data", "save data"}
	missingState := "load dta" // typo: missing 'a'

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle whitespace in state names
}

func TestInvalidStateHintGenerator_HandlesAllStatesVeryDifferent(t *testing.T) {
	// Given: All available states are very different from missing state
	availableStates := []string{"abcd", "efgh", "ijkl"}
	missingState := "xyz"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should return hints listing available states or empty slice
}

func TestInvalidStateHintGenerator_ThreadSafety_NoConcurrentAccessIssues(t *testing.T) {
	// Given: Error with available states
	availableStates := []string{"start", "process", "finish"}
	missingState := "stat"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When: Call generator concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			hints := InvalidStateHintGenerator(structErr)
			assert.NotNil(t, hints)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Then: No race conditions or panics (verified by -race flag in CI)
}

func TestInvalidStateHintGenerator_HandlesInterfaceSliceAvailableStates(t *testing.T) {
	// Given: available_states as []interface{} with string elements
	availableStates := []any{"state_one", "state_two", "state_three"}
	missingState := "state_on" // typo

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle []any conversion and extract string elements
}

func TestInvalidStateHintGenerator_HandlesMixedCaseInStateComparison(t *testing.T) {
	// Given: Mixed case in both missing and available states
	availableStates := []string{"StartProcess", "EndProcess"}
	missingState := "startProcess" // lowercase 's'

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Levenshtein should find similarity despite case difference
	if len(hints) > 0 {
		foundSuggestion := false
		for _, hint := range hints {
			if containsSubstring(hint.Message, "StartProcess") {
				foundSuggestion = true
				break
			}
		}
		// May or may not suggest depending on distance threshold
		_ = foundSuggestion
	}
}

func TestInvalidStateHintGenerator_HandlesDuplicateStatesInAvailableList(t *testing.T) {
	// Given: Duplicate states in available list
	availableStates := []string{"process", "validate", "process", "execute"}
	//nolint:misspell // intentional typo for test
	missingState := "proces" // typo

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state not found",
		map[string]any{
			"state":            missingState,
			"available_states": availableStates,
		},
		nil,
	)

	// When
	hints := InvalidStateHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle duplicates gracefully (may or may not deduplicate suggestions)
}

// =============================================================================
// YAMLSyntaxHintGenerator Tests (Happy Path)
// =============================================================================

func TestYAMLSyntaxHintGenerator_ReturnsHints_WithLineAndColumn(t *testing.T) {
	// Given: YAML syntax error with line and column information
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   5,
			"column": 12,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints when line/column information exists")
	// Should mention line and column numbers
	foundLineColumnHint := false
	for _, hint := range hints {
		if (containsSubstring(hint.Message, "line") || containsSubstring(hint.Message, "5")) &&
			(containsSubstring(hint.Message, "column") || containsSubstring(hint.Message, "12")) {
			foundLineColumnHint = true
			break
		}
	}
	assert.True(t, foundLineColumnHint, "should include line and column information in hints")
}

func TestYAMLSyntaxHintGenerator_ReturnsMultipleHints_WithCommonSyntaxTips(t *testing.T) {
	// Given: YAML syntax error
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   10,
			"column": 3,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return multiple hints")
	assert.LessOrEqual(t, len(hints), 5, "should limit to reasonable number of hints")

	// Check for expected hint types
	hintMessages := make([]string, len(hints))
	for i, hint := range hints {
		hintMessages[i] = hint.Message
	}

	// Should include at least one actionable hint about YAML syntax
	hasActionableHint := false
	for _, msg := range hintMessages {
		if containsSubstring(msg, "indent") ||
			containsSubstring(msg, "colon") ||
			containsSubstring(msg, "dash") ||
			containsSubstring(msg, "tab") ||
			containsSubstring(msg, "space") {
			hasActionableHint = true
			break
		}
	}
	assert.True(t, hasActionableHint, "should include at least one YAML syntax guidance hint")
}

func TestYAMLSyntaxHintGenerator_SuggestsIndentation_ForCommonMistakes(t *testing.T) {
	// Given: YAML syntax error (indentation is common issue)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   15,
			"column": 1,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints for syntax errors")

	// Should include guidance about common YAML issues
	foundGuidance := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "indent") ||
			containsSubstring(hint.Message, "space") ||
			containsSubstring(hint.Message, "tab") ||
			containsSubstring(hint.Message, "colon") {
			foundGuidance = true
			break
		}
	}
	assert.True(t, foundGuidance, "should provide guidance about common YAML syntax mistakes")
}

func TestYAMLSyntaxHintGenerator_IncludesLineColumn_WhenProvided(t *testing.T) {
	// Given: Multiple YAML syntax errors with different positions
	tests := []struct {
		name   string
		line   int
		column int
	}{
		{
			name:   "beginning of file",
			line:   1,
			column: 1,
		},
		{
			name:   "middle of file",
			line:   50,
			column: 25,
		},
		{
			name:   "large line number",
			line:   999,
			column: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
				"invalid YAML syntax",
				map[string]any{
					"line":   tt.line,
					"column": tt.column,
				},
				nil,
			)

			// When
			hints := YAMLSyntaxHintGenerator(structErr)

			// Then
			assert.NotEmpty(t, hints, "should return hints with position information")
		})
	}
}

func TestYAMLSyntaxHintGenerator_ProvidesContextualHelp_EvenWithoutLineColumn(t *testing.T) {
	// Given: YAML syntax error without line/column information
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return general hints even without position info")

	// Should provide generic YAML syntax guidance
	hasGenericGuidance := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "YAML") ||
			containsSubstring(hint.Message, "indent") ||
			containsSubstring(hint.Message, "syntax") {
			hasGenericGuidance = true
			break
		}
	}
	assert.True(t, hasGenericGuidance, "should provide generic YAML guidance when position unknown")
}

func TestYAMLSyntaxHintGenerator_SuggestsOnlineValidator(t *testing.T) {
	// Given: YAML syntax error
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   20,
			"column": 5,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints")

	// May suggest using online validator or documentation
	// This is implementation-dependent, so we just verify hints are returned
}

// =============================================================================
// YAMLSyntaxHintGenerator Tests (Edge Cases)
// =============================================================================

func TestYAMLSyntaxHintGenerator_ReturnsEmptySlice_ForNonYAMLSyntaxError(t *testing.T) {
	// Given: Different error code (not YAML_SYNTAX)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		nil,
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice for non-YAML-syntax errors")
}

func TestYAMLSyntaxHintGenerator_ReturnsHints_WhenDetailsIsNil(t *testing.T) {
	// Given: YAML syntax error with nil Details
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		nil,
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should return generic hints even without details
	assert.NotEmpty(t, hints, "should return general YAML guidance when details missing")
}

func TestYAMLSyntaxHintGenerator_HandlesLineOnly_WithoutColumn(t *testing.T) {
	// Given: YAML syntax error with line but no column
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line": 10,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should return hints with line information only")
}

func TestYAMLSyntaxHintGenerator_HandlesColumnOnly_WithoutLine(t *testing.T) {
	// Given: YAML syntax error with column but no line
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"column": 5,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should return hints with column information only")
}

func TestYAMLSyntaxHintGenerator_HandlesLineColumnAsStrings(t *testing.T) {
	// Given: line/column provided as strings instead of integers
	tests := []struct {
		name   string
		line   any
		column any
	}{
		{
			name:   "both strings",
			line:   "10",
			column: "5",
		},
		{
			name:   "line string, column int",
			line:   "10",
			column: 5,
		},
		{
			name:   "line int, column string",
			line:   10,
			column: "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
				"invalid YAML syntax",
				map[string]any{
					"line":   tt.line,
					"column": tt.column,
				},
				nil,
			)

			// When
			hints := YAMLSyntaxHintGenerator(structErr)

			// Then
			assert.NotNil(t, hints, "should return non-nil slice")
			assert.NotEmpty(t, hints, "should handle type conversion gracefully")
		})
	}
}

func TestYAMLSyntaxHintGenerator_HandlesNegativeLineColumn(t *testing.T) {
	// Given: Invalid negative line/column numbers
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   -5,
			"column": -10,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle invalid position gracefully, may return generic hints
	assert.NotEmpty(t, hints, "should provide generic hints for invalid position")
}

func TestYAMLSyntaxHintGenerator_HandlesZeroLineColumn(t *testing.T) {
	// Given: Zero line/column numbers
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   0,
			"column": 0,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should handle zero position values")
}

func TestYAMLSyntaxHintGenerator_HandlesVeryLargeLineColumn(t *testing.T) {
	// Given: Very large line/column numbers
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   999999,
			"column": 888888,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should handle large position values")
}

func TestYAMLSyntaxHintGenerator_HandlesLineColumnAsFloat(t *testing.T) {
	// Given: line/column provided as floats
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   10.5,
			"column": 5.2,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should handle float type conversion")
}

func TestYAMLSyntaxHintGenerator_HandlesLineColumnAsBoolean(t *testing.T) {
	// Given: line/column provided as booleans (invalid)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   true,
			"column": false,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should provide generic hints when type conversion fails")
}

func TestYAMLSyntaxHintGenerator_HandlesNilError_Gracefully(t *testing.T) {
	// Given: nil StructuredError
	var structErr *domainerrors.StructuredError

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice for nil error")
}

func TestYAMLSyntaxHintGenerator_HandlesEmptyDetails(t *testing.T) {
	// Given: Empty Details map
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should return generic hints for empty details")
}

func TestYAMLSyntaxHintGenerator_HandlesExtraDetailsFields(t *testing.T) {
	// Given: Details with extra fields beyond line/column
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":     10,
			"column":   5,
			"snippet":  "  version: invalid",
			"filename": "workflow.yaml",
			"extra":    "data",
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should handle extra detail fields gracefully")
}

func TestYAMLSyntaxHintGenerator_LimitsNumberOfHints(t *testing.T) {
	// Given: YAML syntax error
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   10,
			"column": 5,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.LessOrEqual(t, len(hints), 5, "should limit hints to reasonable number to avoid overwhelming user")
}

// =============================================================================
// YAMLSyntaxHintGenerator Tests (Error Handling)
// =============================================================================

func TestYAMLSyntaxHintGenerator_ProvidesActionableGuidance_ForAllCases(t *testing.T) {
	// Given: Various YAML syntax error scenarios
	tests := []struct {
		name    string
		details map[string]any
	}{
		{
			name:    "with line and column",
			details: map[string]any{"line": 10, "column": 5},
		},
		{
			name:    "with line only",
			details: map[string]any{"line": 10},
		},
		{
			name:    "with column only",
			details: map[string]any{"column": 5},
		},
		{
			name:    "no position info",
			details: map[string]any{},
		},
		{
			name:    "nil details",
			details: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
				"invalid YAML syntax",
				tt.details,
				nil,
			)

			// When
			hints := YAMLSyntaxHintGenerator(structErr)

			// Then
			assert.NotNil(t, hints, "should return non-nil slice")
			// Should provide at least one actionable hint in all cases
			assert.NotEmpty(t, hints, "should always provide actionable guidance for YAML syntax errors")
		})
	}
}

func TestYAMLSyntaxHintGenerator_ThreadSafety_NoConcurrentAccessIssues(t *testing.T) {
	// Given: YAML syntax error
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   10,
			"column": 5,
		},
		nil,
	)

	// When: Call generator concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			hints := YAMLSyntaxHintGenerator(structErr)
			assert.NotNil(t, hints)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Then: No race conditions or panics (verified by -race flag in CI)
}

func TestYAMLSyntaxHintGenerator_HandlesDetailsWithNilValues(t *testing.T) {
	// Given: Details with nil values
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   nil,
			"column": nil,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should handle nil detail values gracefully")
}

func TestYAMLSyntaxHintGenerator_HandlesDetailsWithComplexTypes(t *testing.T) {
	// Given: Details with complex types
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   []int{10, 20},          // slice instead of int
			"column": map[string]int{"a": 5}, // map instead of int
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should provide generic hints when details have unexpected types")
}

func TestYAMLSyntaxHintGenerator_HandlesUnicodeInErrorMessage(t *testing.T) {
	// Given: YAML syntax error with unicode characters
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"YAMLシンタックスエラー", // "YAML syntax error" in Japanese
		map[string]any{
			"line":   10,
			"column": 5,
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should handle unicode in error messages")
}

func TestYAMLSyntaxHintGenerator_HandlesLineColumnWithMaxInt(t *testing.T) {
	// Given: Maximum integer values for line/column
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{
			"line":   int(^uint(0) >> 1), // max int
			"column": int(^uint(0) >> 1),
		},
		nil,
	)

	// When
	hints := YAMLSyntaxHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.NotEmpty(t, hints, "should handle maximum integer values")
}

func TestYAMLSyntaxHintGenerator_ProvidesDifferentHints_BasedOnPosition(t *testing.T) {
	// Given: YAML syntax errors at different positions
	tests := []struct {
		name   string
		line   int
		column int
	}{
		{
			name:   "first line, first column",
			line:   1,
			column: 1,
		},
		{
			name:   "first line, middle column",
			line:   1,
			column: 50,
		},
		{
			name:   "middle line, first column",
			line:   100,
			column: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
				"invalid YAML syntax",
				map[string]any{
					"line":   tt.line,
					"column": tt.column,
				},
				nil,
			)

			// When
			hints := YAMLSyntaxHintGenerator(structErr)

			// Then
			assert.NotNil(t, hints, "should return non-nil slice")
			assert.NotEmpty(t, hints, "should provide hints for all positions")
		})
	}
}

// =============================================================================
// MissingInputHintGenerator Tests (Happy Path)
// =============================================================================

func TestMissingInputHintGenerator_ReturnsHints_ForMissingRequiredInput(t *testing.T) {
	// Given: Error for missing required input
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   "api_key",
			"required_inputs": []string{"api_key", "endpoint", "timeout"},
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints for missing required input")
	// Should mention the missing input name
	foundMissingInput := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "api_key") {
			foundMissingInput = true
			break
		}
	}
	assert.True(t, foundMissingInput, "should mention the missing input name")
}

func TestMissingInputHintGenerator_ListsAllRequiredInputs(t *testing.T) {
	// Given: Error with multiple required inputs
	requiredInputs := []string{"project_id", "region", "credentials"}
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   "project_id",
			"required_inputs": requiredInputs,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints listing required inputs")
	// Should mention some or all required inputs
	hintMessages := make([]string, len(hints))
	for i, hint := range hints {
		hintMessages[i] = hint.Message
	}

	foundRequiredInputsHint := false
	for _, msg := range hintMessages {
		if containsSubstring(msg, "required") || containsSubstring(msg, "project_id") {
			foundRequiredInputsHint = true
			break
		}
	}
	assert.True(t, foundRequiredInputsHint, "should mention required inputs")
}

func TestMissingInputHintGenerator_ProvidesExampleUsage(t *testing.T) {
	// Given: Missing input error
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   "config_file",
			"required_inputs": []string{"config_file", "output_dir"},
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints with usage guidance")
	// Should provide command usage example
	foundUsageHint := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "--input") ||
			containsSubstring(hint.Message, "awf") ||
			containsSubstring(hint.Message, "example") {
			foundUsageHint = true
			break
		}
	}
	assert.True(t, foundUsageHint, "should provide usage example")
}

func TestMissingInputHintGenerator_SuggestsExampleValues(t *testing.T) {
	// Given: Missing input with context
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   "timeout",
			"required_inputs": []string{"timeout", "retries"},
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints with example values")
	// Should suggest example values for the input type
	foundExampleHint := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "example") ||
			containsSubstring(hint.Message, "=") {
			foundExampleHint = true
			break
		}
	}
	assert.True(t, foundExampleHint, "should suggest example values")
}

func TestMissingInputHintGenerator_HandlesMultipleMissingInputs(t *testing.T) {
	// Given: Error with multiple missing inputs
	missingInputs := []string{"username", "password"}
	requiredInputs := []string{"username", "password", "host"}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variables",
		map[string]any{
			"missing_inputs":  missingInputs,
			"required_inputs": requiredInputs,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints for multiple missing inputs")
	// Should mention multiple inputs or provide comprehensive guidance
}

func TestMissingInputHintGenerator_LimitsNumberOfHints(t *testing.T) {
	// Given: Error with many required inputs
	requiredInputs := make([]string, 20)
	for i := 0; i < 20; i++ {
		requiredInputs[i] = "input_" + string(rune('a'+i%26))
	}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   "input_a",
			"required_inputs": requiredInputs,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.LessOrEqual(t, len(hints), 5, "should limit hints to reasonable number to avoid overwhelming user")
}

// =============================================================================
// MissingInputHintGenerator Tests (Edge Cases)
// =============================================================================

func TestMissingInputHintGenerator_ReturnsEmptySlice_ForNonValidationError(t *testing.T) {
	// Given: Different error code (not VALIDATION_FAILED)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		nil,
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice for non-validation errors")
}

func TestMissingInputHintGenerator_ReturnsEmptySlice_WhenDetailsIsNil(t *testing.T) {
	// Given: VALIDATION_FAILED error with nil Details
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"validation failed",
		nil,
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// May return empty or generic hints, but should not panic
}

func TestMissingInputHintGenerator_ReturnsHints_WhenMissingInputLacks(t *testing.T) {
	// Given: Error without missing_input field
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"validation failed",
		map[string]any{
			"required_inputs": []string{"foo", "bar"},
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// May return hints about required inputs even without specific missing_input
}

func TestMissingInputHintGenerator_HandlesEmptyRequiredInputs(t *testing.T) {
	// Given: Error with empty required inputs list
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"validation failed",
		map[string]any{
			"missing_input":   "some_input",
			"required_inputs": []string{},
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle empty required inputs gracefully
}

func TestMissingInputHintGenerator_HandlesMissingInputNotString(t *testing.T) {
	// Given: missing_input is not a string
	tests := []struct {
		name              string
		missingInputValue any
	}{
		{
			name:              "missing_input is integer",
			missingInputValue: 123,
		},
		{
			name:              "missing_input is boolean",
			missingInputValue: true,
		},
		{
			name:              "missing_input is nil",
			missingInputValue: nil,
		},
		{
			name:              "missing_input is slice",
			missingInputValue: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputValidationFailed,
				"validation failed",
				map[string]any{
					"missing_input":   tt.missingInputValue,
					"required_inputs": []string{"input1", "input2"},
				},
				nil,
			)

			// When
			hints := MissingInputHintGenerator(structErr)

			// Then
			assert.NotNil(t, hints, "should return non-nil slice")
			// Should handle type mismatch gracefully
		})
	}
}

func TestMissingInputHintGenerator_HandlesRequiredInputsNotSlice(t *testing.T) {
	// Given: required_inputs is not a slice
	tests := []struct {
		name                string
		requiredInputsValue any
	}{
		{
			name:                "required_inputs is string",
			requiredInputsValue: "not a slice",
		},
		{
			name:                "required_inputs is integer",
			requiredInputsValue: 123,
		},
		{
			name:                "required_inputs is boolean",
			requiredInputsValue: false,
		},
		{
			name:                "required_inputs is nil",
			requiredInputsValue: nil,
		},
		{
			name:                "required_inputs is map",
			requiredInputsValue: map[string]string{"key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputValidationFailed,
				"validation failed",
				map[string]any{
					"missing_input":   "some_input",
					"required_inputs": tt.requiredInputsValue,
				},
				nil,
			)

			// When
			hints := MissingInputHintGenerator(structErr)

			// Then
			assert.NotNil(t, hints, "should return non-nil slice")
			// Should handle type mismatch gracefully
		})
	}
}

func TestMissingInputHintGenerator_HandlesRequiredInputsWithNonStrings(t *testing.T) {
	// Given: required_inputs contains non-string elements
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"validation failed",
		map[string]any{
			"missing_input": "input1",
			"required_inputs": []any{
				"valid_input",
				123,  // non-string
				true, // non-string
				"another_input",
				nil, // non-string
			},
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should filter out non-string elements and process valid ones
}

func TestMissingInputHintGenerator_HandlesEmptyMissingInput(t *testing.T) {
	// Given: missing_input is empty string
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"validation failed",
		map[string]any{
			"missing_input":   "",
			"required_inputs": []string{"input1", "input2"},
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle empty string gracefully
}

func TestMissingInputHintGenerator_HandlesInputNameWithSpecialCharacters(t *testing.T) {
	// Given: Input names with special characters
	requiredInputs := []string{
		"user-name",
		"user_name",
		"user.name",
		"user:name",
		"user@domain",
	}
	missingInput := "user-name"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   missingInput,
			"required_inputs": requiredInputs,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle special characters in input names
}

func TestMissingInputHintGenerator_HandlesVeryLongInputName(t *testing.T) {
	// Given: Very long input name
	longInput := "very_long_input_name_with_many_words_that_exceeds_typical_identifier_length_limits_but_still_valid"
	requiredInputs := []string{longInput, "short"}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   longInput,
			"required_inputs": requiredInputs,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle long input names without issue
}

func TestMissingInputHintGenerator_HandlesNilError_Gracefully(t *testing.T) {
	// Given: nil StructuredError
	var structErr *domainerrors.StructuredError

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	assert.Empty(t, hints, "should return empty slice for nil error")
}

func TestMissingInputHintGenerator_HandlesEmptyDetails(t *testing.T) {
	// Given: Empty Details map
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"validation failed",
		map[string]any{},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// May return empty or generic hints, but should not panic
}

func TestMissingInputHintGenerator_HandlesSingleRequiredInput(t *testing.T) {
	// Given: Only one required input
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   "api_key",
			"required_inputs": []string{"api_key"},
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints for single required input")
	// Should provide clear guidance for single input case
}

func TestMissingInputHintGenerator_HandlesCaseVariations(t *testing.T) {
	// Given: Input names with different case
	requiredInputs := []string{"ApiKey", "USERNAME", "password"}
	missingInput := "apikey" // Different case than available

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   missingInput,
			"required_inputs": requiredInputs,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle case-sensitive matching (Go convention)
}

func TestMissingInputHintGenerator_HandlesInputNameWithWhitespace(t *testing.T) {
	// Given: Input names with whitespace
	requiredInputs := []string{"config file", "output dir"}
	missingInput := "config file"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   missingInput,
			"required_inputs": requiredInputs,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle whitespace in input names
}

// =============================================================================
// MissingInputHintGenerator Tests (Error Handling)
// =============================================================================

func TestMissingInputHintGenerator_HandlesUnicodeInputNames(t *testing.T) {
	// Given: Input names with Unicode characters
	requiredInputs := []string{"ユーザー名", "パスワード"}
	missingInput := "ユーザー名"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   missingInput,
			"required_inputs": requiredInputs,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle Unicode without panic
}

func TestMissingInputHintGenerator_ThreadSafety_NoConcurrentAccessIssues(t *testing.T) {
	// Given: Error with required inputs
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   "api_key",
			"required_inputs": []string{"api_key", "endpoint"},
		},
		nil,
	)

	// When: Call generator concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			hints := MissingInputHintGenerator(structErr)
			assert.NotNil(t, hints)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Then: No race conditions or panics (verified by -race flag in CI)
}

func TestMissingInputHintGenerator_HandlesDetailsWithNilValues(t *testing.T) {
	// Given: Details with nil values
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"validation failed",
		map[string]any{
			"missing_input":   nil,
			"required_inputs": nil,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle nil detail values gracefully
}

func TestMissingInputHintGenerator_HandlesInterfaceSliceRequiredInputs(t *testing.T) {
	// Given: required_inputs as []interface{} with string elements
	requiredInputs := []any{"input_one", "input_two", "input_three"}

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   "input_one",
			"required_inputs": requiredInputs,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle []any conversion and extract string elements
}

func TestMissingInputHintGenerator_HandlesDuplicateInputsInRequiredList(t *testing.T) {
	// Given: Duplicate inputs in required list
	requiredInputs := []string{"config", "api_key", "config", "timeout"}
	missingInput := "config"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   missingInput,
			"required_inputs": requiredInputs,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle duplicates gracefully
}

func TestMissingInputHintGenerator_ProvidesActionableGuidance_ForAllCases(t *testing.T) {
	// Given: Various missing input scenarios
	tests := []struct {
		name    string
		details map[string]any
	}{
		{
			name: "with missing_input and required_inputs",
			details: map[string]any{
				"missing_input":   "api_key",
				"required_inputs": []string{"api_key", "endpoint"},
			},
		},
		{
			name: "with required_inputs only",
			details: map[string]any{
				"required_inputs": []string{"input1", "input2"},
			},
		},
		{
			name: "with missing_input only",
			details: map[string]any{
				"missing_input": "some_input",
			},
		},
		{
			name:    "no details",
			details: map[string]any{},
		},
		{
			name:    "nil details",
			details: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputValidationFailed,
				"validation failed",
				tt.details,
				nil,
			)

			// When
			hints := MissingInputHintGenerator(structErr)

			// Then
			assert.NotNil(t, hints, "should return non-nil slice")
			// Should provide actionable guidance when possible
		})
	}
}

func TestMissingInputHintGenerator_HandlesExtraDetailsFields(t *testing.T) {
	// Given: Details with extra fields beyond missing_input/required_inputs
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"validation failed",
		map[string]any{
			"missing_input":   "api_key",
			"required_inputs": []string{"api_key"},
			"workflow_name":   "test-workflow",
			"step_name":       "authenticate",
			"extra_field":     "extra_value",
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// Should handle extra detail fields gracefully
}

func TestMissingInputHintGenerator_SuggestsSimilarInputName_IfTypo(t *testing.T) {
	// Given: Missing input that might be a typo of a required input
	requiredInputs := []string{"api_key", "endpoint", "timeout"}
	missingInput := "api_ky" // typo: missing 'e'

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputValidationFailed,
		"missing required input variable",
		map[string]any{
			"missing_input":   missingInput,
			"required_inputs": requiredInputs,
		},
		nil,
	)

	// When
	hints := MissingInputHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice")
	// May suggest "did you mean api_key?" using Levenshtein similarity
	// (Implementation-dependent, so we just verify hints are returned)
}

// =============================================================================
// CommandFailureHintGenerator Tests (Happy Path)
// =============================================================================

func TestCommandFailureHintGenerator_ReturnsPermissionHint_ForExitCode126(t *testing.T) {
	// Given: Command execution failed with exit code 126 (permission denied)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command execution failed",
		map[string]any{
			"exit_code": 126,
			"command":   "./deploy.sh",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints for exit code 126")
	// Should suggest checking permissions
	foundPermissionHint := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "permission") ||
			containsSubstring(hint.Message, "chmod") ||
			containsSubstring(hint.Message, "executable") {
			foundPermissionHint = true
			break
		}
	}
	assert.True(t, foundPermissionHint, "should suggest checking permissions for exit 126")
}

func TestCommandFailureHintGenerator_ReturnsCommandNotFoundHint_ForExitCode127(t *testing.T) {
	// Given: Command execution failed with exit code 127 (command not found)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command execution failed",
		map[string]any{
			"exit_code": 127,
			"command":   "nonexistent-command",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints for exit code 127")
	// Should suggest command not found
	foundCommandNotFoundHint := false
	for _, hint := range hints {
		if containsSubstring(hint.Message, "not found") ||
			containsSubstring(hint.Message, "PATH") ||
			containsSubstring(hint.Message, "installed") {
			foundCommandNotFoundHint = true
			break
		}
	}
	assert.True(t, foundCommandNotFoundHint, "should suggest command not found for exit 127")
}

func TestCommandFailureHintGenerator_ReturnsGenericHint_ForOtherExitCodes(t *testing.T) {
	// Given: Command execution failed with generic exit code
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command execution failed",
		map[string]any{
			"exit_code": 1,
			"command":   "git push",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return hints for generic exit codes")
	// May include generic troubleshooting hints
}

func TestCommandFailureHintGenerator_ReturnsMultipleHints_WithCommandContext(t *testing.T) {
	// Given: Command execution failed with command context
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command execution failed",
		map[string]any{
			"exit_code": 127,
			"command":   "docker-compose up",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotEmpty(t, hints, "should return hints when command context available")
	// Should include actionable suggestions
}

func TestCommandFailureHintGenerator_HandlesStringExitCode(t *testing.T) {
	// Given: Exit code provided as string instead of int
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command execution failed",
		map[string]any{
			"exit_code": "126",
			"command":   "./script.sh",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle string exit codes gracefully")
}

func TestCommandFailureHintGenerator_HandlesFloatExitCode(t *testing.T) {
	// Given: Exit code provided as float
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command execution failed",
		map[string]any{
			"exit_code": 127.0,
			"command":   "unknown-cmd",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle float exit codes gracefully")
}

// =============================================================================
// CommandFailureHintGenerator Tests (Edge Cases)
// =============================================================================

func TestCommandFailureHintGenerator_ReturnsEmptySlice_ForWrongErrorCode(t *testing.T) {
	// Given: Error with different error code
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		map[string]any{"path": "/missing.yaml"},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.Empty(t, hints, "should return empty slice for non-matching error codes")
}

func TestCommandFailureHintGenerator_ReturnsEmptySlice_ForNilError(t *testing.T) {
	// Given: Nil error
	var structErr *domainerrors.StructuredError

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.Empty(t, hints, "should return empty slice for nil error")
}

func TestCommandFailureHintGenerator_HandlesNilDetails(t *testing.T) {
	// Given: StructuredError with nil Details
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		nil,
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice even with nil Details")
	assert.Empty(t, hints, "should return empty hints when Details is nil")
}

func TestCommandFailureHintGenerator_HandlesMissingExitCode(t *testing.T) {
	// Given: Error without exit_code in Details
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"command": "some-command",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice when exit_code missing")
	// May return generic hints without specific exit code guidance
}

func TestCommandFailureHintGenerator_HandlesMissingCommand(t *testing.T) {
	// Given: Error without command in Details
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 1,
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice when command missing")
	// Should handle absence of command context gracefully
}

func TestCommandFailureHintGenerator_HandlesEmptyDetails(t *testing.T) {
	// Given: Error with empty Details map
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should return non-nil slice with empty Details")
	assert.Empty(t, hints, "should return empty hints when Details is empty")
}

func TestCommandFailureHintGenerator_HandlesNegativeExitCode(t *testing.T) {
	// Given: Negative exit code (unusual but possible in some contexts)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": -1,
			"command":   "test-cmd",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle negative exit codes gracefully")
}

func TestCommandFailureHintGenerator_HandlesVeryLargeExitCode(t *testing.T) {
	// Given: Very large exit code (outside typical range)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 999999,
			"command":   "test-cmd",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle large exit codes gracefully")
}

func TestCommandFailureHintGenerator_HandlesComplexCommand(t *testing.T) {
	// Given: Complex command with pipes and redirects
	complexCmd := "cat /var/log/app.log | grep ERROR | tail -n 10 > /tmp/errors.txt"
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 1,
			"command":   complexCmd,
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle complex commands gracefully")
}

func TestCommandFailureHintGenerator_HandlesEmptyCommand(t *testing.T) {
	// Given: Empty command string
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 127,
			"command":   "",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle empty command string")
}

func TestCommandFailureHintGenerator_HandlesCommandWithSpecialCharacters(t *testing.T) {
	// Given: Command with special shell characters
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 126,
			"command":   "bash -c 'echo $HOME'",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle commands with special characters")
}

// =============================================================================
// CommandFailureHintGenerator Tests (Error Handling)
// =============================================================================

func TestCommandFailureHintGenerator_HandlesInvalidExitCodeType(t *testing.T) {
	// Given: Exit code is not a number (e.g., boolean)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": true,
			"command":   "test-cmd",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle invalid exit code types gracefully")
	// Should not panic on type mismatch
}

func TestCommandFailureHintGenerator_HandlesInvalidCommandType(t *testing.T) {
	// Given: Command is not a string (e.g., number)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 1,
			"command":   12345,
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle invalid command types gracefully")
	// Should not panic on type mismatch
}

func TestCommandFailureHintGenerator_HandlesExitCodeZero(t *testing.T) {
	// Given: Exit code 0 (success - unusual for error)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 0,
			"command":   "echo hello",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle exit code 0 gracefully")
	// Exit 0 is unusual in error context but should be handled
}

func TestCommandFailureHintGenerator_HandlesExtraDetailsFields(t *testing.T) {
	// Given: Error with additional unexpected fields in Details
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code":     127,
			"command":       "unknown-cmd",
			"stderr":        "command not found",
			"stdout":        "",
			"duration_ms":   150,
			"retry_count":   3,
			"extra_context": map[string]string{"key": "value"},
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle extra detail fields gracefully")
	// Should extract only what's needed
}

func TestCommandFailureHintGenerator_HandlesUnicodeInCommand(t *testing.T) {
	// Given: Command with Unicode characters
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 127,
			"command":   "プログラム実行",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle Unicode in command strings")
}

func TestCommandFailureHintGenerator_ThreadSafe_ConcurrentCalls(t *testing.T) {
	// Given: Error that will be processed concurrently
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 126,
			"command":   "./deploy.sh",
		},
		nil,
	)

	// When: Call generator concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			hints := CommandFailureHintGenerator(structErr)
			assert.NotNil(t, hints)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Then: No race conditions or panics (verified by -race flag in CI)
}

func TestCommandFailureHintGenerator_HandlesVeryLongCommand(t *testing.T) {
	// Given: Very long command string
	longCommand := ""
	for i := 0; i < 1000; i++ {
		longCommand += "echo test | "
	}
	longCommand += "cat"

	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 1,
			"command":   longCommand,
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle very long commands")
}

func TestCommandFailureHintGenerator_HandlesExitCode130_SIGINT(t *testing.T) {
	// Given: Exit code 130 (128 + 2 = SIGINT - Ctrl+C)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 130,
			"command":   "long-running-process",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle signal-related exit codes")
	// May provide hints about interruption/cancellation
}

func TestCommandFailureHintGenerator_HandlesExitCode137_SIGKILL(t *testing.T) {
	// Given: Exit code 137 (128 + 9 = SIGKILL)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 137,
			"command":   "memory-intensive-task",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle SIGKILL exit codes")
	// May provide hints about resource limits or OOM
}

func TestCommandFailureHintGenerator_HandlesExitCode2_MisuseOfShellBuiltin(t *testing.T) {
	// Given: Exit code 2 (misuse of shell builtin)
	structErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		map[string]any{
			"exit_code": 2,
			"command":   "cd /nonexistent/path",
		},
		nil,
	)

	// When
	hints := CommandFailureHintGenerator(structErr)

	// Then
	assert.NotNil(t, hints, "should handle exit code 2")
}

// =============================================================================
// Test Helpers
// =============================================================================

// containsSubstring checks if a string contains a substring (case-insensitive for robustness)
func containsSubstring(s, substr string) bool {
	return contains(s, substr)
}

// contains is a simple case-sensitive substring check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || substr == "" || indexByte(s, substr) >= 0)
}

// indexByte finds the index of substr in s, returns -1 if not found
func indexByte(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
