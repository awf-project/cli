package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domerrors "github.com/vanoix/awf/internal/domain/errors"
)

func TestCompositeRepository_Load(t *testing.T) {
	// Setup temp directories
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, ".awf", "workflows")
	globalDir := filepath.Join(tmpDir, "global", "workflows")

	require.NoError(t, os.MkdirAll(localDir, 0o755))
	require.NoError(t, os.MkdirAll(globalDir, 0o755))

	// Create same workflow in multiple locations
	localWorkflow := `name: test-workflow
version: "1.0.0"
description: Local version
states:
  initial: start
  start:
    type: terminal
`
	globalWorkflow := `name: test-workflow
version: "2.0.0"
description: Global version
states:
  initial: start
  start:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "test-workflow.yaml"), []byte(localWorkflow), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "test-workflow.yaml"), []byte(globalWorkflow), 0o644))

	// Only global workflow
	globalOnlyWorkflow := `name: global-only
version: "1.0.0"
states:
  initial: start
  start:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "global-only.yaml"), []byte(globalOnlyWorkflow), 0o644))

	// Create composite repository
	repo := NewCompositeRepository([]SourcedPath{
		{Path: localDir, Source: SourceLocal},
		{Path: globalDir, Source: SourceGlobal},
	})

	ctx := context.Background()

	t.Run("local takes precedence over global", func(t *testing.T) {
		wf, err := repo.Load(ctx, "test-workflow")
		require.NoError(t, err)
		require.NotNil(t, wf)
		assert.Equal(t, "Local version", wf.Description)
		assert.Equal(t, "1.0.0", wf.Version)
	})

	t.Run("loads global-only workflow", func(t *testing.T) {
		wf, err := repo.Load(ctx, "global-only")
		require.NoError(t, err)
		require.NotNil(t, wf)
		assert.Equal(t, "global-only", wf.Name)
	})

	t.Run("returns error for non-existent workflow", func(t *testing.T) {
		wf, err := repo.Load(ctx, "non-existent")
		require.Error(t, err)
		assert.Nil(t, wf)

		// Should be a StructuredError with USER.INPUT.MISSING_FILE code
		var se *domerrors.StructuredError
		require.ErrorAs(t, err, &se)
		assert.Equal(t, domerrors.ErrorCodeUserInputMissingFile, se.Code)
	})
}

func TestCompositeRepository_List(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, ".awf", "workflows")
	globalDir := filepath.Join(tmpDir, "global", "workflows")

	require.NoError(t, os.MkdirAll(localDir, 0o755))
	require.NoError(t, os.MkdirAll(globalDir, 0o755))

	// Local workflows
	localWf := `name: local-wf
version: "1.0.0"
states:
  initial: start
  start:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "local-wf.yaml"), []byte(localWf), 0o644))

	// Global workflows
	globalWf := `name: global-wf
version: "1.0.0"
states:
  initial: start
  start:
    type: terminal
`
	sharedWf := `name: shared-wf
version: "1.0.0"
states:
  initial: start
  start:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "global-wf.yaml"), []byte(globalWf), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "shared-wf.yaml"), []byte(sharedWf), 0o644))
	// Same workflow also in local
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "shared-wf.yaml"), []byte(sharedWf), 0o644))

	repo := NewCompositeRepository([]SourcedPath{
		{Path: localDir, Source: SourceLocal},
		{Path: globalDir, Source: SourceGlobal},
	})

	ctx := context.Background()

	t.Run("returns unique workflow names", func(t *testing.T) {
		names, err := repo.List(ctx)
		require.NoError(t, err)

		// Should have 3 unique workflows: local-wf, global-wf, shared-wf
		assert.Len(t, names, 3)
		assert.Contains(t, names, "local-wf")
		assert.Contains(t, names, "global-wf")
		assert.Contains(t, names, "shared-wf")
	})
}

func TestCompositeRepository_ListWithSource(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, ".awf", "workflows")
	globalDir := filepath.Join(tmpDir, "global", "workflows")

	require.NoError(t, os.MkdirAll(localDir, 0o755))
	require.NoError(t, os.MkdirAll(globalDir, 0o755))

	localWf := `name: local-wf
version: "1.0.0"
states:
  initial: start
  start:
    type: terminal
`
	globalWf := `name: global-wf
version: "1.0.0"
states:
  initial: start
  start:
    type: terminal
`
	sharedWf := `name: shared-wf
version: "1.0.0"
states:
  initial: start
  start:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "local-wf.yaml"), []byte(localWf), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "shared-wf.yaml"), []byte(sharedWf), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "global-wf.yaml"), []byte(globalWf), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "shared-wf.yaml"), []byte(sharedWf), 0o644))

	repo := NewCompositeRepository([]SourcedPath{
		{Path: localDir, Source: SourceLocal},
		{Path: globalDir, Source: SourceGlobal},
	})

	ctx := context.Background()

	t.Run("returns workflow info with correct source", func(t *testing.T) {
		infos, err := repo.ListWithSource(ctx)
		require.NoError(t, err)

		// Build map for easy lookup
		infoMap := make(map[string]WorkflowInfo)
		for _, info := range infos {
			infoMap[info.Name] = info
		}

		assert.Equal(t, SourceLocal, infoMap["local-wf"].Source)
		assert.Equal(t, SourceGlobal, infoMap["global-wf"].Source)
		// shared-wf should show as local (higher priority)
		assert.Equal(t, SourceLocal, infoMap["shared-wf"].Source)
	})
}

func TestCompositeRepository_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, ".awf", "workflows")

	require.NoError(t, os.MkdirAll(localDir, 0o755))

	localWf := `name: exists-wf
version: "1.0.0"
states:
  initial: start
  start:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "exists-wf.yaml"), []byte(localWf), 0o644))

	repo := NewCompositeRepository([]SourcedPath{
		{Path: localDir, Source: SourceLocal},
	})

	ctx := context.Background()

	t.Run("returns true for existing workflow", func(t *testing.T) {
		exists, err := repo.Exists(ctx, "exists-wf")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("returns false for non-existent workflow", func(t *testing.T) {
		exists, err := repo.Exists(ctx, "non-existent")
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestCompositeRepository_EmptyPaths(t *testing.T) {
	repo := NewCompositeRepository(nil)
	ctx := context.Background()

	t.Run("List returns empty for no paths", func(t *testing.T) {
		names, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, names)
	})

	t.Run("Load returns error for no paths", func(t *testing.T) {
		wf, err := repo.Load(ctx, "anything")
		require.Error(t, err)
		assert.Nil(t, wf)

		// Should be a StructuredError with USER.INPUT.MISSING_FILE code
		var se *domerrors.StructuredError
		require.ErrorAs(t, err, &se)
		assert.Equal(t, domerrors.ErrorCodeUserInputMissingFile, se.Code)
	})
}

func TestCompositeRepository_SkipsMissingDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")
	missingDir := filepath.Join(tmpDir, "missing")

	require.NoError(t, os.MkdirAll(existingDir, 0o755))

	wf := `name: test-wf
version: "1.0.0"
states:
  initial: start
  start:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(existingDir, "test-wf.yaml"), []byte(wf), 0o644))

	repo := NewCompositeRepository([]SourcedPath{
		{Path: missingDir, Source: SourceLocal},
		{Path: existingDir, Source: SourceGlobal},
	})

	ctx := context.Background()

	t.Run("skips missing directory and finds workflow in existing", func(t *testing.T) {
		names, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Contains(t, names, "test-wf")
	})
}
