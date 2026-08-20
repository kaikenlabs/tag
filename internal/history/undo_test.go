package history

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupGenerationWithFile creates a file, adds a backup (for inject/append entries),
// writes a manifest entry, and returns the tag dir and file path.
func setupUndoEnv(t *testing.T) (tagDir, projectDir string) {
	t.Helper()
	dir := t.TempDir()
	tagDir = filepath.Join(dir, ".tag")
	require.NoError(t, os.MkdirAll(tagDir, 0o755))
	return tagDir, dir
}

func TestUT_UndoEngine_LastGeneration_DeletesCreatedFiles(t *testing.T) {
	tagDir, projectDir := setupUndoEnv(t)
	target := filepath.Join(projectDir, "handler.go")
	require.NoError(t, os.WriteFile(target, []byte("package main"), 0o644))
	hash, err := HashFile(target)
	require.NoError(t, err)

	gen := Generation{
		ID: "gen_1_aaa", Timestamp: time.Now(), Template: "model", Command: "generate",
		Files: []FileEntry{{Path: target, Action: ActionCreate, HashAfter: hash}},
	}
	require.NoError(t, Append(tagDir, gen))

	var out bytes.Buffer
	result, undoErr := Undo(tagDir, UndoOptions{Out: &out})
	require.NoError(t, undoErr)
	assert.Equal(t, "gen_1_aaa", result.GenID)
	assert.Equal(t, 1, result.Reverted)

	_, statErr := os.Stat(target)
	assert.True(t, os.IsNotExist(statErr), "created file should be deleted")
	assert.Contains(t, out.String(), "gen_1_aaa")

	m, err := Load(tagDir)
	require.NoError(t, err)
	assert.Empty(t, m.Generations)
}

func TestUT_UndoEngine_LastGeneration_RestoresModifiedFiles(t *testing.T) {
	tagDir, projectDir := setupUndoEnv(t)
	target := filepath.Join(projectDir, "router.go")
	original := []byte("original content")
	modified := []byte("original content\nappended")

	// Write the "after" state to the target.
	require.NoError(t, os.WriteFile(target, modified, 0o644))
	hashAfter, err := HashFile(target)
	require.NoError(t, err)

	// Set up backup (pre-generation content).
	backupDir := filepath.Join(tagDir, "history", "backups", "gen_1_bbb")
	backupPath := filepath.Join(backupDir, target)
	require.NoError(t, os.MkdirAll(filepath.Dir(backupPath), 0o755))
	require.NoError(t, os.WriteFile(backupPath, original, 0o644))

	hashBefore := HashBytes(original)
	gen := Generation{
		ID: "gen_1_bbb", Timestamp: time.Now(), Template: "routes", Command: "generate",
		Files: []FileEntry{{Path: target, Action: ActionAppend, HashBefore: &hashBefore, HashAfter: hashAfter}},
	}
	require.NoError(t, Append(tagDir, gen))

	_, undoErr := Undo(tagDir, UndoOptions{Out: &bytes.Buffer{}})
	require.NoError(t, undoErr)

	restored, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, original, restored)
}

// TestUT_RevertFile_RestoresOverwriteAndOpenAPIMerge is the regression test
// for the user-approved revertFile fix: before the fix, revertFile's second
// case only matched ActionInject and ActionAppend, so undoing a generation
// recorded as "overwrite" or "openapi-merge" fell through to the trailing
// `return nil` — Undo silently did nothing to the file while still counting
// it as reverted and reporting success. Both actions have a backup (see
// RecordingFileWriter.snapshotBefore, which backs up any pre-existing file
// before WriteFile and before MergeOpenAPIFile), so restore-from-backup is
// the correct behaviour for both, exactly as it already is for inject/append.
func TestUT_RevertFile_RestoresOverwriteAndOpenAPIMerge(t *testing.T) {
	tests := []struct {
		name        string
		action      Action
		wantRestore bool
	}{
		{name: "overwrite restores from backup", action: ActionOverwrite, wantRestore: true},
		{name: "openapi-merge restores from backup", action: ActionOpenAPIMerge, wantRestore: true},
		{name: "unknown action is a silent no-op, not an error", action: Action("future-op"), wantRestore: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tagDir, projectDir := setupUndoEnv(t)
			target := filepath.Join(projectDir, "config.go")
			original := []byte("original content")
			current := []byte("current content")

			require.NoError(t, os.WriteFile(target, current, 0o644))
			hashAfter, err := HashFile(target)
			require.NoError(t, err)

			backupDir := filepath.Join(tagDir, "history", "backups", "gen_1_ccc")
			backupPath := filepath.Join(backupDir, target)
			require.NoError(t, os.MkdirAll(filepath.Dir(backupPath), 0o755))
			require.NoError(t, os.WriteFile(backupPath, original, 0o644))

			hashBefore := HashBytes(original)
			gen := Generation{
				ID: "gen_1_ccc", Timestamp: time.Now(), Template: "config", Command: "generate",
				Files: []FileEntry{{Path: target, Action: tt.action, HashBefore: &hashBefore, HashAfter: hashAfter}},
			}
			require.NoError(t, Append(tagDir, gen))

			_, undoErr := Undo(tagDir, UndoOptions{Out: &bytes.Buffer{}})
			require.NoError(t, undoErr)

			after, err := os.ReadFile(target)
			require.NoError(t, err)
			if tt.wantRestore {
				assert.Equal(t, original, after, "%s should restore from backup", tt.action)
			} else {
				assert.Equal(t, current, after, "unknown action must leave the file untouched")
			}
		})
	}
}

func TestUT_UndoEngine_SpecificID_ByID(t *testing.T) {
	tagDir, projectDir := setupUndoEnv(t)
	target := filepath.Join(projectDir, "service.go")
	require.NoError(t, os.WriteFile(target, []byte("pkg"), 0o644))
	hash, err := HashFile(target)
	require.NoError(t, err)

	gen1 := Generation{
		ID: "gen_1_aaa", Command: "generate",
		Files: []FileEntry{{Path: "other.go", Action: ActionCreate, HashAfter: "sha256:xxx"}},
	}
	gen2 := Generation{
		ID: "gen_2_bbb", Command: "generate",
		Files: []FileEntry{{Path: target, Action: ActionCreate, HashAfter: hash}},
	}
	require.NoError(t, Append(tagDir, gen1))
	require.NoError(t, Append(tagDir, gen2))

	_, undoErr := Undo(tagDir, UndoOptions{GenID: "gen_2_bbb", Out: &bytes.Buffer{}})
	require.NoError(t, undoErr)

	_, statErr := os.Stat(target)
	assert.True(t, os.IsNotExist(statErr))

	m, err := Load(tagDir)
	require.NoError(t, err)
	require.Len(t, m.Generations, 1)
	assert.Equal(t, "gen_1_aaa", m.Generations[0].ID)
}

func TestUT_UndoEngine_UnknownID_ReturnsErrNotFound(t *testing.T) {
	tagDir, _ := setupUndoEnv(t)
	_, err := Undo(tagDir, UndoOptions{GenID: "gen_unknown", Out: &bytes.Buffer{}})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUT_UndoEngine_NoGenerations_ReturnsError(t *testing.T) {
	tagDir, _ := setupUndoEnv(t)
	_, err := Undo(tagDir, UndoOptions{Out: &bytes.Buffer{}})
	assert.Error(t, err)
}

func TestUT_UndoEngine_ConflictDetection_Blocks_ByDefault(t *testing.T) {
	tagDir, projectDir := setupUndoEnv(t)
	target := filepath.Join(projectDir, "handler.go")
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))

	// Record hash of original.
	hashAfter := HashBytes([]byte("original"))
	gen := Generation{
		ID: "gen_1_aaa", Command: "generate",
		Files: []FileEntry{{Path: target, Action: ActionCreate, HashAfter: hashAfter}},
	}
	require.NoError(t, Append(tagDir, gen))

	// Simulate user modification after generation.
	require.NoError(t, os.WriteFile(target, []byte("user modified"), 0o644))

	_, err := Undo(tagDir, UndoOptions{Out: &bytes.Buffer{}})
	assert.ErrorIs(t, err, ErrConflict)
}

func TestUT_UndoEngine_ConflictDetection_Force_Proceeds(t *testing.T) {
	tagDir, projectDir := setupUndoEnv(t)
	target := filepath.Join(projectDir, "handler.go")
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))

	hashAfter := HashBytes([]byte("original"))
	gen := Generation{
		ID: "gen_1_aaa", Command: "generate",
		Files: []FileEntry{{Path: target, Action: ActionCreate, HashAfter: hashAfter}},
	}
	require.NoError(t, Append(tagDir, gen))

	require.NoError(t, os.WriteFile(target, []byte("user modified"), 0o644))

	_, err := Undo(tagDir, UndoOptions{Force: true, Out: &bytes.Buffer{}})
	assert.NoError(t, err)
	_, statErr := os.Stat(target)
	assert.True(t, os.IsNotExist(statErr))
}

func TestUT_UndoEngine_ConflictDetection_Partial_SkipsConflicted(t *testing.T) {
	tagDir, projectDir := setupUndoEnv(t)

	cleanFile := filepath.Join(projectDir, "clean.go")
	dirtyFile := filepath.Join(projectDir, "dirty.go")
	require.NoError(t, os.WriteFile(cleanFile, []byte("clean"), 0o644))
	require.NoError(t, os.WriteFile(dirtyFile, []byte("dirty_original"), 0o644))

	cleanHash := HashBytes([]byte("clean"))
	dirtyHash := HashBytes([]byte("dirty_original"))

	gen := Generation{
		ID: "gen_1_aaa", Command: "generate",
		Files: []FileEntry{
			{Path: cleanFile, Action: ActionCreate, HashAfter: cleanHash},
			{Path: dirtyFile, Action: ActionCreate, HashAfter: dirtyHash},
		},
	}
	require.NoError(t, Append(tagDir, gen))

	// Modify only dirty file.
	require.NoError(t, os.WriteFile(dirtyFile, []byte("user modified"), 0o644))

	var out bytes.Buffer
	result, err := Undo(tagDir, UndoOptions{Partial: true, Out: &out})
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Reverted)
	assert.Equal(t, 1, result.Skipped)
	assert.Equal(t, []string{dirtyFile}, result.Conflicts)

	_, cleanStatErr := os.Stat(cleanFile)
	assert.True(t, os.IsNotExist(cleanStatErr), "clean file should be deleted")

	_, dirtyStatErr := os.Stat(dirtyFile)
	assert.False(t, os.IsNotExist(dirtyStatErr), "dirty file should still exist (skipped)")

	assert.Contains(t, out.String(), "skipped (conflict)")
}

func TestUT_UndoEngine_DeletesEmptyDirectories(t *testing.T) {
	tagDir, projectDir := setupUndoEnv(t)

	subDir := filepath.Join(projectDir, "internal", "auth")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	target := filepath.Join(subDir, "handler.go")
	require.NoError(t, os.WriteFile(target, []byte("pkg"), 0o644))
	hash, err := HashFile(target)
	require.NoError(t, err)

	gen := Generation{
		ID: "gen_1_aaa", Command: "generate",
		Files: []FileEntry{{Path: target, Action: ActionCreate, HashAfter: hash}},
	}
	require.NoError(t, Append(tagDir, gen))

	_, undoErr := Undo(tagDir, UndoOptions{Out: &bytes.Buffer{}})
	require.NoError(t, undoErr)

	_, dirStatErr := os.Stat(subDir)
	assert.True(t, os.IsNotExist(dirStatErr), "empty directory should be removed")
}

func TestUT_UndoEngine_ListGenerations_NewestFirst(t *testing.T) {
	tagDir, _ := setupUndoEnv(t)
	g1 := Generation{ID: "gen_1_aaa", Command: "generate"}
	g2 := Generation{ID: "gen_2_bbb", Command: "generate"}
	require.NoError(t, Append(tagDir, g1))
	require.NoError(t, Append(tagDir, g2))

	gens, err := ListGenerations(tagDir)
	require.NoError(t, err)
	require.Len(t, gens, 2)
	assert.Equal(t, "gen_2_bbb", gens[0].ID)
	assert.Equal(t, "gen_1_aaa", gens[1].ID)
}

func TestUT_UndoEngine_UndoTwice_SecondReturnsNotFound(t *testing.T) {
	tagDir, projectDir := setupUndoEnv(t)
	target := filepath.Join(projectDir, "handler.go")
	require.NoError(t, os.WriteFile(target, []byte("pkg"), 0o644))
	hash, err := HashFile(target)
	require.NoError(t, err)

	gen := Generation{
		ID: "gen_1_aaa", Command: "generate",
		Files: []FileEntry{{Path: target, Action: ActionCreate, HashAfter: hash}},
	}
	require.NoError(t, Append(tagDir, gen))

	_, undoErr := Undo(tagDir, UndoOptions{Out: &bytes.Buffer{}})
	require.NoError(t, undoErr)
	_, err = Undo(tagDir, UndoOptions{Out: &bytes.Buffer{}})
	assert.Error(t, err, "second undo should fail because no generations remain")
}
